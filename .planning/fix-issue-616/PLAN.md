# Fix issue #616 — `session send --no-wait` can leave prompts typed-but-not-submitted on freshly-launched sessions

**Target release:** v1.7.10
**Branch:** `fix/616-send-no-wait-reliability`
**Discipline:** claude-conductor TDD (RED → GREEN → verify → ship)

---

## 1. Problem summary

`agent-deck session send --no-wait <session> "message"` (and `launch --no-wait -m ...`) can paste the message into Claude's composer but leave the trailing Enter unsubmitted. The message is visible at the `❯` prompt, `status=waiting`, and the agent is idle. Reported in issue #616; user observed ~30–50% failure rate on fresh launches with long prompts.

Root cause (from data-flow trace below): the `--no-wait` path bypasses `waitForAgentReady` entirely, then runs a **1.2-second verification loop** (`maxRetries=8, checkDelay=150ms`) with `maxFullResends=-1` (full-resend disabled to protect fix #479). On a freshly-launched Claude session with MCPs, the TUI may spend 5–40s loading. During the 1.2s window:

- `GetStatus()` typically returns `"active"` (Claude's loading animations register as pane activity).
- The loop counts `activeChecks` and returns success at `activeSuccessThreshold=2` (≈300ms).
- But at this moment the composer input-handler isn't mounted yet. The `Enter` from `SendKeysAndEnter` was dropped during bracketed-paste processing or TUI init.
- The message stays typed in the composer. No re-Enter is fired because the loop already returned.

The existing `HasUnsentComposerPrompt` detector would catch this — but only if the composer has rendered before the loop exits on "active" status.

## 2. Data-flow trace

CLI → TMux for `session send --no-wait`:

| Hop | File:line | What it does |
|-----|-----------|--------------|
| 1 | `cmd/agent-deck/session_cmd.go:1349 handleSessionSend` | Parses flags (`--no-wait`, `--wait`, etc.) |
| 2 | `:1422` | If `--no-wait` → **skip** `waitForAgentReady` |
| 3 | `:1437` | Call `sendWithRetryTarget(... maxRetries:8, checkDelay:150ms, maxFullResends:-1)` |
| 4 | `:1536` | `target.SendKeysAndEnter(message)` |
| 5 | `internal/tmux/tmux.go:3462 SendKeysAndEnter` | `SendKeysChunked` → 100ms sleep → `SendEnter` |
| 6 | `:3478 SendKeysChunked` | 4KB-chunked `tmux send-keys -l -t <sess> -- <payload>` (bracketed-paste wrapped by tmux 3.2+) |
| 7 | `:3451 SendEnter` | `tmux send-keys -t <sess> Enter` |
| 8 | `cmd/agent-deck/session_cmd.go:1572` | Verification loop: check `unsentPromptDetected` + `GetStatus()` for up to `maxRetries × checkDelay = 1.2s` |

**Failure mode:** hops 5–8 race against Claude TUI mount. Hop 7's `Enter` can be consumed by the bracketed-paste-end handler (`\e[201~`) in a not-yet-interactive Ink TUI, leaving the composer populated with the message. Hop 8's 1.2-second window is too short to span the Claude + MCP startup window (5–40s).

## 3. Failing test cases (Phase 3 RED)

Tests land in `cmd/agent-deck/session_send_test.go` (mock-level) and `cmd/agent-deck/session_send_integration_test.go` (real-tmux integration).

### Test 1: `TestSendWithRetryTarget_NoWait_ReEntersWhenComposerStillHasMessageAfterInitialSuccess`
**(mock-level, deterministic, must fail on main)**

Simulates the 616 race:
- `GetStatus()` returns `active, active, active, active, active, active, waiting` (Claude booting, then ready).
- `CapturePaneFresh()` returns `"❯ TEST_MSG_616"` for all iterations (message typed but not submitted).
- Main's `sendWithRetryTarget` with `--no-wait` options bails at iteration 1 on `activeChecks>=2` → SendEnter never fires to re-submit.
- After fix: loop must detect `HasUnsentComposerPrompt` and fire `SendEnter()` even when status is "active".

### Test 2: `TestSendWithRetryTarget_NoWait_ExtendedBudget_CatchesLateUnsentPrompt`
**(mock-level, deterministic)**

Simulates the scenario where the composer renders late (after iteration 2):
- Statuses: `active, active, active, waiting, waiting, waiting, …` (Claude loading, then idle but with pasted-but-unsent prompt).
- Panes: `"", "", "", "❯ TEST_MSG", "❯ TEST_MSG", …` (composer renders at retry 3).
- On main: `active` wins at retry 1 → loop exits before composer renders.
- After fix: extended budget must let retry 3+ fire → SendEnter() nudges.

### Test 3: `TestSendNoWaitPreflightBarrier_WaitsForComposer`
**(mock-level, deterministic)**

New preflight helper `awaitComposerReadyBestEffort(target, maxWait)`:
- Given a pane that shows no `❯` for 800ms then shows `❯` → returns `true` at ~800ms.
- Given a pane that never shows `❯` within `maxWait=2s` → returns `false` at ≥2s (doesn't block longer).
- Verifies the latency cap, ensuring `--no-wait` spirit is preserved.

### Test 4: `TestSendWithRetry_NoWaitIntegration_FreshSession`
**(integration, real tmux, deterministic race reproducer)**

Reuses the pattern from `TestSendWithRetry_DelayedInputHandler_Integration`:
- Script prints `❯ ` immediately, sleeps 2s draining input (simulates Claude TUI mount window), then accepts a non-empty line.
- Test calls `handleSessionSendNoWait`-equivalent (direct invocation of CLI `--no-wait` path via `sendWithRetryTarget` with the v1.7.10 options).
- Asserts `GOT: <message>` appears in pane content within 6s.
- Must run 10 times without failure (`-count=10`) as a stability gate.

Guarded by `AGENT_DECK_INTEGRATION_TESTS=1` env var (consistent with existing integration gating).

## 4. Implementation sketch

### Approach: hybrid A+B
**Why:** Option A (readiness barrier) alone adds latency; Option C (double-Enter) risks a stray blank line. Combining a **cheap, capped preflight barrier** with an **extended verification budget** eliminates the race without ever sending payload twice (preserves #479 fix).

### Change set (minimal)

1. **`cmd/agent-deck/session_cmd.go`**
   - Extract helper `awaitComposerReadyBestEffort(target sendRetryTarget, tool string, maxWait time.Duration) bool`:
     - Poll `CapturePaneFresh` every 100ms.
     - Return true on first appearance of `send.HasCurrentComposerPrompt(content)`.
     - Return false (don't block) after `maxWait` elapses.
   - In `handleSessionSend` `--no-wait` branch (line ~1437):
     - Call `awaitComposerReadyBestEffort(tmuxSess, inst.Tool, 2*time.Second)` for Claude-compatible tools only.
     - Bump retry budget: `maxRetries: 30, checkDelay: 200ms` → total window ≈ 6s. Still fast; still bounded.
     - Keep `maxFullResends: -1` (preserve #479 regression test).

2. **`cmd/agent-deck/session_send_test.go`**
   - Add Tests 1–3 (mock-level).

3. **`cmd/agent-deck/session_send_integration_test.go`** (new file)
   - Add Test 4 (real-tmux integration).
   - Uses `testutil.IsolateTmuxSocket()` via package-level `TestMain` (already present in `testmain_test.go`).

4. **`CHANGELOG.md`** — entry under `## [1.7.10]`.
5. **`cmd/agent-deck/main.go`** — `Version = "1.7.10"`.
6. **`.claude/release-tests.yaml`** — append the 4 new test names.

### Why NOT alternatives

- **Double-Enter (Option C)** — cheap but a stray empty Enter arriving after a different input prompt (e.g. a permission dialog) could dismiss the wrong thing. Rejected.
- **Full-resend in `--no-wait`** — would regress #479 (double-send). Rejected.
- **Always call `waitForAgentReady` in `--no-wait`** — contradicts the user's intent (opt-in fast path). Rejected.

## 5. Scope boundaries

**In scope:**
- The `--no-wait` path in `handleSessionSend`.
- The `launch --no-wait -m` path (flows through the same `sendWithRetryTarget` call via `handleLaunch` — verify no secondary site needs the same fix).

**Out of scope:**
- Default `session send` behavior (already handles this via `waitForAgentReady`).
- MCP attach path.
- TUI interactive send path.
- Codex/Gemini tool paths (their prompts are already gated; issue is Claude-specific per the reporter).

**Forbidden:**
- Reducing `maxFullResends: -1` for `--no-wait` (would regress #479).
- Any change to `SendKeysAndEnter`'s 100ms inter-keystroke delay (preserves tmux 3.2 bracketed-paste fix).
- Removing `waitForAgentReady` from the default send path.

## 6. Verification plan

**Phase 4 (RED confirmation):**
```bash
export GOTOOLCHAIN=go1.24.0
go test ./cmd/agent-deck/... -run "TestSendWithRetryTarget_NoWait_ReEnters|TestSendWithRetryTarget_NoWait_ExtendedBudget|TestSendNoWaitPreflightBarrier" -count=1 -v
# EXPECT: all FAIL on main
```

**Phase 6 (full suite):**
```bash
export GOTOOLCHAIN=go1.24.0 TMUX_TMPDIR=$(mktemp -d)
unset TMUX TMUX_PANE
go test ./... -race -count=1 -timeout 600s
# EXPECT: all pass, no new failures beyond documented flakes
#   (statedb TestWatcherEventDedup, TestMarshalUnmarshalToolData_MultiRepo)
```

**Phase 7 (live boundary, 10x):**
```bash
# In the conductor host — for i in 1..10:
agent-deck launch /tmp/test-616-$i -t "test616-$i" -c claude --no-wait -m "Say LIVE_OK_$i"
sleep 2
agent-deck session output "test616-$i" -q | grep -q "LIVE_OK_$i" && echo PASS_$i || echo FAIL_$i
agent-deck remove "test616-$i" -y
```
Must print PASS for all 10 iterations.

**Phase 8a:** append the 4 test names + the integration invocation to `.claude/release-tests.yaml`.

---

## 7. Open questions (escalate if hit)

- If preflight barrier fires on a session that's already past composer-ready (i.e. a warm session used repeatedly), does it add perceptible latency? Expected: no — `HasCurrentComposerPrompt` on first poll returns true immediately, so barrier returns at ~0ms.
- Does `launch --no-wait -m` reach the same code path? Trace: `handleLaunch` → creates instance → `StartWithMessage(msg)` → `sendMessageWhenReady` in `internal/session/instance.go:2313`. **Separate code path** — but it already has `waitForAgentReady`-equivalent polling (state-machine: `starting → active → waiting`). Issue #616's reproducer used `launch --no-wait -m`, implying the StartWithMessage path is ALSO vulnerable OR the CLI falls through to `session send --no-wait` internally. **To investigate in Phase 2 — confirm which code path the reproducer exercises.**

(Resolved in Phase 2 implementation: the CLI `launch --no-wait` uses `StartWithMessage`, but `session send --no-wait` is the more frequently hit path. Fix in `handleSessionSend`; if `StartWithMessage` also shows the race in Phase 7 live testing, apply the same preflight barrier to `sendMessageWhenReady`.)
