---
phase: 20-health-alerts-bridge
plan: "01"
subsystem: watcher
tags: [health-alerts, tdd, bridge, notifier, debounce, REQ-WF-3]
dependency_graph:
  requires: []
  provides: [watcher.HealthBridge, watcher.Notifier, watcher.Alert, watcher.Config, session.WatcherAlertsSettings]
  affects: [internal/watcher, internal/session]
tech_stack:
  added: []
  patterns: [tdd-red-green-docs, clock-injection, panic-recovery, per-key-debounce]
key_files:
  created:
    - internal/watcher/health_bridge.go
    - internal/watcher/health_bridge_test.go
  modified:
    - internal/session/userconfig.go
    - internal/session/userconfig_test.go
    - CHANGELOG.md
decisions:
  - "Clock injected via unexported setClockForTest for deterministic debounce tests, keeping public surface minimal"
  - "Single Notifier interface (not []Notifier): fan-out is caller's responsibility"
  - "maybeEmit is synchronous (inline, no goroutine) to simplify race detection"
  - "Run returns nil on ctx cancel, notifier errors, and panics — never propagates errors"
  - "Debounce map cleared on ctx cancel so bridge restarts begin fresh"
metrics:
  duration: "~15 minutes"
  completed: "2026-04-16T00:38:49Z"
  tasks_completed: 3
  files_changed: 5
---

# Phase 20 Plan 01: Health Alerts Bridge Summary

**One-liner:** HealthBridge subscribing to `<-chan HealthState` with per-(watcher x trigger) 15-min debounce, panic/error resilience, and opt-in `[watcher.alerts]` config — closes REQ-WF-3.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | RED: 7 failing tests + WatcherAlertsSettingsDefaults | `8c45428` | health_bridge_test.go (new), userconfig_test.go (modified) |
| 2 | GREEN: Implement HealthBridge + WatcherAlertsSettings | `ab139b3` | health_bridge.go (new), userconfig.go (modified) |
| 3 | DOCS: CHANGELOG entry for watcher.alerts | `f096986` | CHANGELOG.md |

## Commit Details

| Hash | Full SHA | Message |
|------|----------|---------|
| `8c45428` | `8c454288275dbd529e8217aaae39b5c3b05ed054` | test(20-01): health_bridge RED |
| `ab139b3` | `ab139b302f8289ddd7f8f29e1683f90b0f123fba` | feat(20-01): health_bridge GREEN |
| `f096986` | `f096986f4c94f0dea1e224129e0d7761620f1bb3` | docs(20-01): CHANGELOG note for watcher.alerts |

All three commits signed `Committed by Ashesh Goplani`. Zero Claude attribution. `--no-verify` used per worktree parallel-mode instructions only.

## Test Pass Banners

### go test ./internal/watcher/... -run TestHealthBridge -race -count=1 -timeout 120s -v

```
=== RUN   TestHealthBridge_SilenceTriggersOneAlert
--- PASS: TestHealthBridge_SilenceTriggersOneAlert (0.01s)
=== RUN   TestHealthBridge_ErrorThresholdTriggersOneAlert
--- PASS: TestHealthBridge_ErrorThresholdTriggersOneAlert (0.01s)
=== RUN   TestHealthBridge_DebounceWithin15Min
--- PASS: TestHealthBridge_DebounceWithin15Min (0.21s)
=== RUN   TestHealthBridge_DisabledConfigZeroAlerts
--- PASS: TestHealthBridge_DisabledConfigZeroAlerts (0.00s)
=== RUN   TestHealthBridge_DownstreamFailureNoncrash
--- PASS: TestHealthBridge_DownstreamFailureNoncrash (0.01s)
=== RUN   TestHealthBridge_TeardownCancelsPending
--- PASS: TestHealthBridge_TeardownCancelsPending (0.20s)
=== RUN   TestHealthBridge_Integration_MockAdapterForcedSilence
--- PASS: TestHealthBridge_Integration_MockAdapterForcedSilence (0.01s)
PASS
ok  	github.com/asheshgoplani/agent-deck/internal/watcher	1.506s
```

### go test ./internal/session/... -run WatcherAlertsSettings -race -count=1 -v

```
=== RUN   TestWatcherAlertsSettingsDefaults
--- PASS: TestWatcherAlertsSettingsDefaults (0.00s)
PASS
ok  	github.com/asheshgoplani/agent-deck/internal/session	1.030s
```

## Scope Verification

### git diff --name-only HEAD~3 HEAD | sort -u (exactly 5 files, no engine.go)

```
CHANGELOG.md
internal/session/userconfig.go
internal/session/userconfig_test.go
internal/watcher/health_bridge.go
internal/watcher/health_bridge_test.go
```

### grep -c "watcher.alerts" CHANGELOG.md

```
1
```

### Commit hygiene

- `git log -3 --format=%B | grep -c "Committed by Ashesh Goplani"` = **3**
- `git log -3 --format=%B | grep -ciE "claude|co-authored-by"` = **0**
- `engine.go` untouched: `git diff --name-only HEAD~3 HEAD | grep -c "internal/watcher/engine.go"` = **0**

## TDD Gate Compliance

Gate sequence followed strictly:
1. RED commit `8c45428` — `test(20-01): health_bridge RED` (compile-fails, 7+1 failing tests)
2. GREEN commit `ab139b3` — `feat(20-01): health_bridge GREEN` (all 8 tests pass)
3. DOCS commit `f096986` — `docs(20-01): CHANGELOG note for watcher.alerts`

## Decisions Made

1. **Clock injection via `setClockForTest`** — unexported test-only setter keeps the public surface (`Config`, `NewHealthBridge`) clean. The debounce test controls virtual time by swapping the clock function before Run.

2. **Single `Notifier` interface** — one method, one concrete notifier per bridge instance. Fan-out to multiple channels is the concrete notifier's responsibility, not the bridge's. Matches CONTEXT.md locked surface.

3. **Synchronous `maybeEmit`** — notifier calls happen inline in the Run select loop (not in a goroutine). This simplifies race detection and keeps the debounce map consistent without additional coordination. The engine's channel buffer (16) is large enough that the synchronous approach does not create backpressure for normal workloads.

4. **`Run` always returns `nil`** — notifier errors are logged via `slog.Warn`; panics are recovered and logged. The engine's health signal is never affected by notifier behavior.

5. **Debounce map cleared on ctx cancel** — ensures a bridge that is stopped and restarted begins with a fresh debounce state, rather than suppressing alerts due to stale timestamps from a previous run.

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None. `HealthBridge` is fully wired. Production wiring (`engine.HealthCh()` passed as `source`) happens at the call site when a conductor instantiates the bridge.

## Threat Flags

None. `health_bridge.go` introduces no new network endpoints, auth paths, file access patterns, or schema changes. The `Notifier` interface is caller-supplied and out of scope for this phase.

## Self-Check

### Files exist
- `internal/watcher/health_bridge.go`: EXISTS
- `internal/watcher/health_bridge_test.go`: EXISTS
- `internal/session/userconfig.go`: modified (EXISTS)
- `internal/session/userconfig_test.go`: modified (EXISTS)
- `CHANGELOG.md`: modified (EXISTS)

### Commits exist
- `8c45428` (RED): EXISTS
- `ab139b3` (GREEN): EXISTS
- `f096986` (DOCS): EXISTS

## Self-Check: PASSED
