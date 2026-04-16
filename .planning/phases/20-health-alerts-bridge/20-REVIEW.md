---
phase: 20-health-alerts-bridge
reviewed: 2026-04-16T00:00:00Z
depth: standard
files_reviewed: 5
files_reviewed_list:
  - internal/watcher/health_bridge.go
  - internal/watcher/health_bridge_test.go
  - internal/session/userconfig.go
  - internal/session/userconfig_test.go
  - CHANGELOG.md
findings:
  critical: 0
  warning: 4
  info: 5
  total: 9
status: issues_found
---

# Phase 20: Code Review Report

**Reviewed:** 2026-04-16
**Depth:** standard
**Files Reviewed:** 5
**Status:** issues_found

## Summary

Phase 20 adds the opt-in watcher health alerts bridge (`HealthBridge`) that consumes `HealthState` values from the engine and fans alerts through a `Notifier` with per-(watcher x trigger) debounce. The implementation is small, cohesive, and well-tested. Panic/error recovery around the notifier is solid, the debounce logic is correct, and the `Config` zero-value defaulting works.

Scope-limited review of `userconfig.go` focused on the new `WatcherAlertsSettings` struct (~line 2189). The struct, TOML tags, and `GetDebounceMinutes()` default behavior match the pattern used by neighboring `WatcherSettings`.

No critical security or correctness bugs were found. Four warnings concern goroutine lifecycle, unbounded state growth, and test-goroutine data races. Five info items concern maintainability (brittle string matching, unused field, asymmetric cleanup, test flakiness, unprotected test hook).

## Warnings

### WR-01: NotifyTeardown uses context.Background() and can block engine teardown

**File:** `internal/watcher/health_bridge.go:111-117`
**Issue:** `NotifyTeardown` is a synchronous method called (per design intent) from adapter teardown code paths. It invokes `b.maybeEmit(context.Background(), ...)`, which then calls `b.notifier.Notify(ctx, alert)` on the caller's goroutine. Because the context can never be cancelled, a slow or hanging `Notifier` (e.g., a Telegram/Slack HTTP round-trip that stalls) will block the caller indefinitely. During engine shutdown this can prevent graceful teardown and hold up process exit. The engine's own ctx is not threaded in here, so there is no external way to cancel.
**Fix:**
```go
// Option A: accept a context from the caller
func (b *HealthBridge) NotifyTeardown(ctx context.Context, watcherName string, unexpected bool) {
    if !b.cfg.Enabled || !unexpected {
        return
    }
    b.maybeEmit(ctx, watcherName, TriggerAdapterTeardownUnexpected,
        "adapter teardown was unexpected")
}

// Option B: bound the notifier call with a short timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
b.maybeEmit(ctx, watcherName, TriggerAdapterTeardownUnexpected, "...")
```
Option A is preferred so the engine's shutdown ctx propagates. The `Notifier` contract already accepts a context, so honoring cancellation is expected.

### WR-02: lastSent map grows unbounded across unique (watcher, trigger) keys

**File:** `internal/watcher/health_bridge.go:50, 138-149`
**Issue:** The `lastSent` map keys by `watcherName + "|" + trigger` and is only reset on `ctx.Done` (line 93). In deployments where watcher names churn (per-session adapters with session-id-suffixed names, or dynamic per-project watchers), this map grows for the lifetime of the bridge. Each entry is small (string + time.Time), so the leak is slow, but it is unbounded by design. There is no mechanism to forget entries for watchers that have been torn down.
**Fix:** Evict stale entries opportunistically during `maybeEmit`. Any entry older than `2*DebounceWindow` cannot suppress a future alert and is safe to drop:
```go
b.mu.Lock()
last, seen := b.lastSent[key]
if seen && now.Sub(last) < b.cfg.DebounceWindow {
    b.mu.Unlock()
    return
}
b.lastSent[key] = now
// Best-effort GC: drop entries too old to matter for debounce.
cutoff := now.Add(-2 * b.cfg.DebounceWindow)
for k, t := range b.lastSent {
    if t.Before(cutoff) {
        delete(b.lastSent, k)
    }
}
b.mu.Unlock()
```
Alternative: expose a `ForgetWatcher(name string)` call that the engine invokes on adapter removal.

### WR-03: Disabled bridge still holds channel, can block upstream senders

**File:** `internal/watcher/health_bridge.go:79-84`
**Issue:** When `cfg.Enabled == false`, `Run` parks on `<-ctx.Done()` and never drains `b.source`. If the engine publishes `HealthState` values into that channel without a writer-side select on ctx.Done, the buffered channel fills and upstream `send` operations block. This is a latent coupling bug: future engine code may assume the channel is always drained while the bridge is "running".
**Fix:** Either drain-and-discard when disabled, or document the contract clearly in `Run`'s doc comment and ensure callers select on ctx:
```go
if !b.cfg.Enabled {
    b.log.Info("health_bridge_disabled")
    for {
        select {
        case <-ctx.Done():
            return nil
        case _, ok := <-b.source:
            if !ok {
                return nil
            }
            // discard
        }
    }
}
```
This keeps the bridge's side-effect surface consistent regardless of `Enabled`.

### WR-04: Data race on `now` variable between test goroutine and bridge goroutine

**File:** `internal/watcher/health_bridge_test.go:159-160, 188, 324-325`
**Issue:** In `TestHealthBridge_DebounceWithin15Min` and `TestHealthBridge_TeardownCancelsPending`, `now` is a local `time.Time` captured by the injected clock closure. The test goroutine writes `now = now.Add(5 * time.Minute)` on line 188 while the `Run` goroutine reads `now` via `b.clock()` from `maybeEmit` without synchronization. Running the test suite under `-race` (which CI enables per 1.5.0 changelog PERF notes) should flag this. It has likely been masked by scheduling luck because the first event is processed before the clock is advanced, but the race is genuine.
**Fix:** Protect the clock with a mutex or use `sync/atomic.Value`:
```go
var clockMu sync.Mutex
now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
bridge.setClockForTest(func() time.Time {
    clockMu.Lock()
    defer clockMu.Unlock()
    return now
})
// ...
clockMu.Lock()
now = now.Add(5 * time.Minute)
clockMu.Unlock()
```
Or use an `atomic.Pointer[time.Time]` if mutex feels heavy.

## Info

### IN-01: triggerFor uses strings.Contains — brittle coupling to HealthTracker message format

**File:** `internal/watcher/health_bridge.go:120-135`
**Issue:** Mapping from `HealthState` to trigger relies on substring matches against `state.Message` (`"no events"`, `"consecutive errors"`). If `HealthTracker` reworks its message strings for UX, the bridge silently stops classifying alerts and emits nothing. There is no compile-time or test-time coupling to catch this.
**Fix:** Extend `HealthState` with a structured `Kind` enum (e.g., `HealthKindSilence`, `HealthKindErrorThreshold`) populated by the tracker, and switch on that in `triggerFor`. Reserve the message for human display. If changing the struct is out of scope, add a unit test that calls `NewHealthTracker(...).Check()` on a forced-silence fixture and asserts `triggerFor` returns `TriggerSilenceDetected`. (The existing integration test `TestHealthBridge_Integration_MockAdapterForcedSilence` partially guards this for silence, but error-threshold has no equivalent.)

### IN-02: Config.Channels is declared but never used by the bridge

**File:** `internal/watcher/health_bridge.go:35-39, 85-88`
**Issue:** `Config.Channels` is logged on startup (count only) but never passed to `Notifier.Notify` — the `Alert` struct has no `Channels` field. Whether the notifier fans to Telegram/Slack/Discord is entirely determined by the notifier's own configuration. This makes `cfg.Channels` a documentation field at best and confusing at worst: operators may set it in `[watcher.alerts].channels` and expect routing to take effect.
**Fix:** Either (a) thread `cfg.Channels` into the `Alert` struct so the notifier can fan per-alert, or (b) remove the field from `Config` and `WatcherAlertsSettings` if the notifier config is authoritative. If (a) is chosen now, sufficient to add one field:
```go
type Alert struct {
    WatcherName string
    Trigger     string
    Message     string
    Timestamp   time.Time
    Channels    []string // copied from Config.Channels at construction
}
```

### IN-03: Asymmetric cleanup between ctx.Done and channel-close paths in Run

**File:** `internal/watcher/health_bridge.go:89-99`
**Issue:** On `ctx.Done` the `lastSent` map is reset under lock; on source channel close it is not. Both paths terminate the bridge, so the behavioral difference is subtle (the bridge is dead either way), but the inconsistency is confusing and a future caller re-running the bridge against a new source would see stale debounce state after channel close.
**Fix:** Extract cleanup into a helper and call it on both exit paths, or simply document that the bridge is single-shot and not reusable. A one-line comment on `Run` stating "bridge is not reusable after Run returns" would remove ambiguity.

### IN-04: setClockForTest writes b.clock without a mutex

**File:** `internal/watcher/health_bridge.go:73-75`
**Issue:** `setClockForTest` writes `b.clock`; `maybeEmit` reads `b.clock` without a lock. Because tests always invoke `setClockForTest` before starting `Run`, no race is exercised in the current suite, but the API is a footgun. A future test that swaps clocks mid-run would see a data race.
**Fix:** Either guard the clock behind `b.mu`, or restrict the hook to construction only (move to an option passed to `NewHealthBridge`). The latter is cleanest:
```go
type Option func(*HealthBridge)
func WithClock(fn func() time.Time) Option { return func(b *HealthBridge) { b.clock = fn } }
func NewHealthBridge(cfg Config, n Notifier, src <-chan HealthState, opts ...Option) *HealthBridge { ... }
```

### IN-05: Test relies on wall-clock sleeps that can flake under load

**File:** `internal/watcher/health_bridge_test.go:72-78, 124-129, 179-185, 198, 286-292, 344, 359, 406-412`
**Issue:** Several tests use `time.Sleep` or polling deadlines of 200-500ms to wait for goroutine progress. On a busy CI node these can flake, and on a slow race-detector run they are the first tests to time out. The pattern also slows the test suite.
**Fix:** Replace polling with a synchronization primitive in the test double. For example, have `testMockNotifier.Notify` signal a channel so the test can `select`-wait without sleeps:
```go
type testMockNotifier struct {
    // ...
    gotAlert chan struct{} // send non-blocking on each recorded alert
}
func (m *testMockNotifier) Notify(_ context.Context, a Alert) error {
    // ...existing code...
    select {
    case m.gotAlert <- struct{}{}:
    default:
    }
    return nil
}
// In test:
select {
case <-notifier.gotAlert:
case <-time.After(2 * time.Second):
    t.Fatal("no alert within 2s")
}
```
Also consider resetting `ClearUserConfigCache()` in `t.Cleanup` in the new `TestWatcherAlertsSettingsDefaults` test so it does not leak config-cache state to sibling tests (the test calls `ClearUserConfigCache()` at entry but not at exit).

---

_Reviewed: 2026-04-16_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
