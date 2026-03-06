# Phase 7: Send Reliability - Research

**Researched:** 2026-03-07
**Domain:** tmux send-keys reliability, Codex stdin readiness, race condition mitigation
**Confidence:** HIGH

## Summary

This phase addresses the two highest-impact send failures in agent-deck: (1) Enter key submission being dropped after pasting text into tmux (SEND-01, 15+ recurrences across all conductors), and (2) messages sent to Codex sessions being consumed by the underlying shell because Codex hasn't attached to stdin yet (SEND-02, 7+ recurrences).

The Enter key problem is caused by tmux 3.2+ bracketed paste behavior. When `send-keys -l` sends text, tmux wraps it in paste sequences (`\e[200~...\e[201~`). When Enter immediately follows, it can arrive in the same PTY buffer as the paste-end marker and get swallowed by async TUI frameworks (Ink/Node.js used by Claude Code, curses). The project has already tried two approaches: atomic command chaining (`;` in a single subprocess, reverted because it still gets swallowed) and a 100ms delay between paste and Enter (current approach, still fails under load). The solution requires making the retry/verification loop more robust, not changing the underlying 2-step send mechanism.

The Codex readiness problem is a timing issue: when a Codex session starts, the tmux pane initially shows a shell prompt. If `session send` fires before Codex has launched and attached to stdin, the text goes to the shell (zsh), not to Codex. The `waitForAgentReady` function checks for `GetStatus()` transitions but has no Codex-specific readiness detection. It needs to wait for the `codex>` prompt to appear in the pane content before sending.

**Primary recommendation:** Fix SEND-01 by improving the retry verification in `sendWithRetryTarget` to be more aggressive with Enter retries and more reliable at detecting unsent state. Fix SEND-02 by adding Codex-specific readiness polling in `waitForAgentReady` that checks for the `codex>` prompt in pane content.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SEND-01 | Session send reliably submits Enter key after pasting text into tmux, eliminating the race condition between paste and keypress | Verified: root cause is bracketed paste timing in tmux 3.2+. Current 100ms delay and retry loop exist but need hardening. Code locations identified in `internal/tmux/tmux.go:SendKeysAndEnter`, `cmd/agent-deck/session_cmd.go:sendWithRetryTarget`, and `internal/session/instance.go:sendMessageWhenReady`. |
| SEND-02 | Messages sent to Codex sessions wait for Codex to attach to stdin before delivery, preventing text from going to the underlying shell | Verified: `waitForAgentReady` has no Codex-specific prompt check. `PromptDetector("codex").HasPrompt()` can detect `codex>` prompt. Need to add Codex prompt polling in `waitForAgentReady`. Production workaround documented: `add + start + sleep 10 + send --no-wait`. |
</phase_requirements>

## Architecture Patterns

### Relevant Code Locations

The send pipeline has three entry points that all converge on the same tmux primitives:

```
Entry Points:
1. CLI: `agent-deck session send` -> handleSessionSend() -> sendWithRetry() -> sendWithRetryTarget()
2. TUI: home.go -> SendKeysAndEnter() / SendKeysChunked() (direct tmux calls)
3. Instance: sendMessageWhenReady() (used by launch -m and session start with message)

All converge on:
  internal/tmux/tmux.go:
    SendKeys()           -> tmux send-keys -l -t <name> -- <text>
    SendEnter()          -> tmux send-keys -t <name> Enter
    SendKeysAndEnter()   -> SendKeysChunked() + 100ms delay + SendEnter()
    SendKeysChunked()    -> splits at 4KB boundaries, 50ms inter-chunk delay
```

### Code Duplication (must be addressed)

These functions are **duplicated** between `cmd/agent-deck/session_cmd.go` and `internal/session/instance.go`:

| Function | session_cmd.go | instance.go |
|----------|---------------|-------------|
| `hasUnsentPastedPrompt` | line 1467 | line 2058 |
| `hasUnsentComposerPrompt` | line 1595 | duplicated logic |
| `normalizePromptText` | line 1471 | line 2062 |
| `isComposerDividerLine` | line 1480 | line 2071 |
| `parsePromptFromComposerBlock` | line 1496 | line 2087 |
| `currentComposerPrompt` | line 1537 | duplicated |
| `hasCurrentComposerPrompt` | line 1587 | line 2178 |

The planner should consolidate these into a single package during this phase to prevent divergence bugs.

### Pattern: sendWithRetryTarget (the retry verification loop)

The core retry loop in `sendWithRetryTarget` works as follows:

```
1. Initial send: SendKeysAndEnter(message)
2. Verify loop (up to maxRetries iterations):
   a. Sleep checkDelay
   b. CapturePaneFresh() -> check for unsent markers
   c. GetStatus() -> check if agent became "active"
   d. If unsent prompt detected: SendEnter() and continue
   e. If status == "active" for 2+ consecutive checks: SUCCESS
   f. If status == "waiting" after seeing "active": SUCCESS
   g. If status == "waiting" but never saw "active": periodic Enter nudge every 3rd iteration
   h. Ambiguous state: limited fallback Enter retries (first 2 iterations only)
3. Best effort: return nil even if verification inconclusive
```

### Pattern: waitForAgentReady (the readiness gate)

Used by `handleSessionSend` (CLI) and `sendMessageWhenReady` (Instance):

```
1. Poll loop (up to 400 attempts * 200ms = 80 seconds):
   a. GetStatus() -> wait for active -> waiting/idle transition
   b. OR 10+ consecutive waiting/idle checks after 3s elapsed (already ready)
   c. Claude-specific: also verify composer prompt visible via CapturePaneFresh()
2. No Codex-specific check exists (the gap for SEND-02)
```

### Anti-Patterns to Avoid

- **Increasing the 100ms delay arbitrarily:** The delay between paste and Enter is a heuristic. Making it 200ms or 300ms would slow ALL sends. The correct fix is making the retry loop detect failure and recover, not making the initial send slower.
- **Re-attempting atomic command chaining:** Already tried in commit 058a176 and reverted in 8832903. Atomic `;` chaining was swallowed by Ink/Node.js TUI frameworks that process bracketed paste asynchronously.
- **Sending Enter without checking unsent state:** Blindly retrying Enter can submit the same prompt twice if the agent already accepted it. The retry loop must detect unsent state before sending Enter.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Codex prompt detection | Custom regex for codex prompt | `PromptDetector("codex").HasPrompt()` from `internal/tmux/detector.go` | Already handles busy vs. waiting states, tested in detection_test.go |
| Pane content capture | Manual tmux subprocess | `Session.CapturePaneFresh()` | Handles cache invalidation, ANSI stripping |
| Status polling | Custom tmux poll loop | `Session.GetStatus()` | Combines pane title fast-path, prompt detection, activity timestamps |
| Integration test harness | Custom tmux setup/teardown | `integration.TmuxHarness` + `WaitForCondition`/`WaitForPaneContent` | Profile isolation, automatic cleanup |

## Common Pitfalls

### Pitfall 1: Bracketed Paste Timing
**What goes wrong:** tmux 3.2+ wraps `send-keys -l` in bracketed paste sequences. When Enter arrives in the same PTY buffer as paste-end (`\e[201~`), the TUI framework discards it.
**Why it happens:** Two separate tmux subprocesses (text + Enter) can be scheduled back-to-back before the PTY consumer processes the first.
**How to avoid:** Keep the 2-step approach (SendKeysChunked + delay + SendEnter) and rely on the retry verification loop to detect and recover from dropped Enter keys.
**Warning signs:** Message appears pasted in the composer but never submitted (the `[Pasted text #1 +N lines]` marker is visible).

### Pitfall 2: Codex Shell vs. Codex Prompt
**What goes wrong:** Text sent before Codex attaches to stdin is consumed by the underlying shell (zsh), producing garbled output or shell errors.
**Why it happens:** Codex takes 2-10 seconds to initialize (sometimes longer if auto-updating). During this time, the tmux pane shows the shell prompt, not `codex>`.
**How to avoid:** Poll for `codex>` in pane content before sending. Do not rely solely on status transitions, because the shell itself can appear "waiting" while Codex is still loading.
**Warning signs:** Pane shows shell errors like `command not found`, or text appears at the shell prompt instead of the Codex input.

### Pitfall 3: Code Duplication Drift
**What goes wrong:** Fix applied in `session_cmd.go` but not in `instance.go` (or vice versa).
**Why it happens:** Send verification logic is duplicated across CLI and Instance paths.
**How to avoid:** Consolidate shared prompt detection functions into a single package (e.g., `internal/session/` or a new `internal/send/` package). Both callers import from one source.
**Warning signs:** Different retry behavior between `agent-deck session send` and `launch -m` paths.

### Pitfall 4: Rapid Successive Sends
**What goes wrong:** When a conductor sends multiple messages in quick succession, the retry loop from the first send can still be running when the second send starts.
**Why it happens:** Each `session send` call has its own retry loop. No inter-send coordination exists.
**How to avoid:** Focus on making each individual send reliable with its own retry loop. The existing architecture handles successive sends sequentially at the tmux level (each command blocks until complete).
**Warning signs:** Double-submission of the same message.

### Pitfall 5: Test Interference with Production
**What goes wrong:** Tests that create tmux sessions with send operations can interfere with production sessions or other tests.
**Why it happens:** Missing profile isolation or test session cleanup.
**How to avoid:** Always use `TmuxHarness` from `internal/integration/harness.go` which auto-cleans sessions and uses test-prefixed names. All test packages must have `TestMain` with `AGENTDECK_PROFILE=_test`.

## Code Examples

### Current SendKeysAndEnter (the 2-step approach)

```go
// Source: internal/tmux/tmux.go:3039
func (s *Session) SendKeysAndEnter(keys string) error {
    s.invalidateCache()
    if err := s.SendKeysChunked(keys); err != nil {
        return err
    }
    // 100ms delay for bracketed paste processing
    time.Sleep(100 * time.Millisecond)
    return s.SendEnter()
}
```

### Current sendWithRetryTarget (the retry verification loop)

```go
// Source: cmd/agent-deck/session_cmd.go:1637
func sendWithRetryTarget(target sendRetryTarget, message string, skipVerify bool, opts sendRetryOptions) error {
    if err := target.SendKeysAndEnter(message); err != nil {
        return fmt.Errorf("failed to send message: %w", err)
    }
    if skipVerify {
        return nil
    }
    // Verify loop checks for unsent markers, status transitions, and retries Enter
    for retry := 0; retry < opts.maxRetries; retry++ {
        time.Sleep(opts.checkDelay)
        // ... detection and retry logic ...
    }
    return nil // best effort
}
```

### Current waitForAgentReady (no Codex-specific check)

```go
// Source: cmd/agent-deck/session_cmd.go:1721
func waitForAgentReady(tmuxSess *tmux.Session, tool string) error {
    // Polls GetStatus() for active -> waiting transition
    // Claude-specific: also checks for composer prompt
    // NO Codex-specific check exists
    if tool == "claude" {
        // check for composer prompt in pane content
    }
    // Codex falls through to generic status transition check
}
```

### Codex PromptDetector (already exists, usable for readiness check)

```go
// Source: internal/tmux/detector.go:63
case "codex":
    lower := strings.ToLower(content)
    if strings.Contains(lower, "esc to interrupt") ||
        strings.Contains(lower, "ctrl+c to interrupt") {
        return false // busy
    }
    return strings.Contains(content, "codex>") ||
        strings.Contains(content, "Continue?")
```

### Integration test pattern (from existing COND-01 test)

```go
// Source: internal/integration/conductor_test.go:18
func TestConductor_SendToChild(t *testing.T) {
    h := NewTmuxHarness(t)
    inst := h.CreateSession("cond-child", "/tmp")
    inst.Command = "cat"
    require.NoError(t, inst.Start())
    WaitForCondition(t, 5*time.Second, 200*time.Millisecond,
        "session to exist", func() bool { return inst.Exists() })
    tmuxSess := inst.GetTmuxSession()
    msg := "hello-from-conductor-" + t.Name()
    require.NoError(t, tmuxSess.SendKeysAndEnter(msg))
    WaitForPaneContent(t, inst, "hello-from-conductor-", 5*time.Second)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Single `send-keys text Enter` | Two calls: `send-keys -l text` then `send-keys Enter` | v0.12.2 (2026-02-10) | Fixed most Enter drops but introduced 100ms delay |
| Atomic `;` chaining | 2-step with 100ms delay | v0.14.1 (2026-02-12) | Reverted: atomic still swallowed by Ink/Node.js bracketed paste |
| No send verification | `sendWithRetryTarget` with paste marker detection | v0.12.2 | Catches most unsent prompts, but still misses cases under load |
| No Codex readiness gate | Generic status transition check | v0.19.11 | Treats stable idle/waiting as ready, but doesn't check for Codex-specific prompt |

**What's still broken (15+ recurrences):**
- Enter still dropped under rapid successive sends or when the TUI is slow to process bracketed paste end marker
- Codex sends go to shell instead of Codex when timing is unlucky (7+ recurrences)

## Open Questions

1. **Optimal Enter retry delay**
   - What we know: 100ms delay works most of the time, but fails under load. The retry loop catches many failures.
   - What's unclear: Whether increasing to 150-200ms would eliminate enough failures to be worth the latency cost for ALL sends.
   - Recommendation: Keep 100ms for initial send, make retry loop more aggressive (shorter checkDelay, more retries when unsent state detected).

2. **Codex auto-update on first start**
   - What we know: Codex sometimes auto-updates and exits on first start. This causes the shell to reappear after Codex exits.
   - What's unclear: Whether there's a reliable way to distinguish "Codex updating" from "Codex starting normally".
   - Recommendation: After detecting `codex>` prompt, verify it persists for at least 2 consecutive checks. If the session goes back to a shell prompt, the send should fail with a clear error rather than sending to the shell.

3. **Should code deduplication be in scope?**
   - What we know: 7+ functions are duplicated between `session_cmd.go` and `instance.go`.
   - What's unclear: Whether consolidation could introduce regressions if the two copies have subtly diverged.
   - Recommendation: Include consolidation as part of this phase. The functions should be compared side-by-side and merged into `internal/session/` with a single source of truth.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify + integration package |
| Config file | `internal/integration/testmain_test.go` (profile isolation) |
| Quick run command | `go test -race -v -run TestSend ./cmd/agent-deck/... ./internal/integration/...` |
| Full suite command | `go test -race -v ./...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SEND-01 | Enter key retry catches unsent state and resubmits | unit | `go test -race -v -run TestSendWithRetry ./cmd/agent-deck/... -x` | Exists: `cmd/agent-deck/session_send_test.go` |
| SEND-01 | Rapid successive sends both deliver via real tmux | integration | `go test -race -v -run TestConductor_SendMultiple ./internal/integration/... -x` | Exists: `internal/integration/conductor_test.go` |
| SEND-01 | Enter retry on real tmux with delayed processing | integration | `go test -race -v -run TestSend_EnterRetry ./internal/integration/... -x` | Wave 0: needs creation |
| SEND-02 | Codex readiness gate waits for codex> prompt | unit | `go test -race -v -run TestWaitForAgentReady_Codex ./cmd/agent-deck/... -x` | Wave 0: needs creation |
| SEND-02 | Send to non-ready Codex session is held until ready | integration | `go test -race -v -run TestSend_CodexReadiness ./internal/integration/... -x` | Wave 0: needs creation (simulated, not real Codex) |
| SEND-01/02 | Existing COND-01 and COND-04 tests still pass | integration | `go test -race -v -run TestConductor ./internal/integration/... -x` | Exists |

### Sampling Rate
- **Per task commit:** `go test -race -v -run "TestSend|TestConductor_Send" ./cmd/agent-deck/... ./internal/integration/...`
- **Per wave merge:** `go test -race -v ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/integration/send_reliability_test.go` -- integration tests for Enter retry on real tmux and Codex readiness simulation (SEND-01, SEND-02)
- [ ] `cmd/agent-deck/session_send_test.go` -- unit tests for Codex-specific waitForAgentReady behavior (extend existing file)

## Sources

### Primary (HIGH confidence)
- **Codebase analysis:** `internal/tmux/tmux.go:3039-3073` (SendKeysAndEnter, SendKeysChunked)
- **Codebase analysis:** `cmd/agent-deck/session_cmd.go:1446-1719` (sendWithRetry, sendWithRetryTarget, waitForAgentReady)
- **Codebase analysis:** `internal/session/instance.go:1928-2054` (sendMessageWhenReady)
- **Codebase analysis:** `internal/tmux/detector.go:63-72` (Codex prompt detection)
- **Codebase analysis:** `internal/integration/conductor_test.go` (existing COND-01, COND-04 tests)
- **Git history:** Commits 058a176 (atomic chaining) and 8832903 (delay restoration)
- **CHANGELOG.md:** v0.12.2, v0.14.1, v0.19.10, v0.19.11, v0.19.17 entries

### Secondary (MEDIUM confidence)
- **Conductor LEARNINGS.md:** `~/.agent-deck/conductor/LEARNINGS.md` entry [20260301-004] (15+ Enter key recurrences)
- **ARD LEARNINGS.md:** `~/.agent-deck/conductor/ard/LEARNINGS.md` entries [20260224-002] and [20260226-005] (Codex timing, 7+ recurrences)
- **OpenGraphDB LEARNINGS.md:** `~/.agent-deck/conductor/opengraphdb/LEARNINGS.md` (Codex launch timeout pattern)
- [tmux issue #1778: send-keys Enter not working as expected](https://github.com/tmux/tmux/issues/1778)
- [tmux issue #1517: send-keys executes commands asynchronously](https://github.com/tmux/tmux/issues/1517)

### Tertiary (LOW confidence)
- [tmux bracketed paste commit discussion](https://www.mail-archive.com/tmux-git@googlegroups.com/msg02626.html)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, all code is in the existing Go codebase, no new libraries needed
- Architecture: HIGH, all three send paths analyzed, code duplication identified, fix approach clear
- Pitfalls: HIGH, based on 15+ Enter key recurrences and 7+ Codex recurrences from production learnings

**Research date:** 2026-03-07
**Valid until:** Indefinite (all findings are from codebase analysis and production learnings, not external API docs)
