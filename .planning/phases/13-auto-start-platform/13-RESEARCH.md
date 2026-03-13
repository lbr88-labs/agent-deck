# Phase 13: Auto-Start & Platform - Research

**Researched:** 2026-03-13
**Domain:** tmux session launch, PTY allocation, WSL/Linux non-interactive contexts, tool conversation ID propagation
**Confidence:** MEDIUM (root cause on WSL/Linux not confirmed without live reproduction; analysis from issue #311 evidence + codebase reading)

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PLAT-01 | Auto-start works from non-interactive contexts on WSL/Linux; tool processes receive a PTY (#311) | Root-cause analysis identifies three candidate failure modes in `tmux.go:Start()` and the `send-keys` dispatch path; fix strategies documented below |
| PLAT-02 | Resume after auto-start uses correct tool conversation ID (not agent-deck internal UUID) (#311) | The `PostStartSync` + `WaitForClaudeSession` flow correctly captures the ID for Claude, but `handleSessionStop` does not save the tmux-env snapshot; the resume path in `buildClaudeCommandWithMessage` reads from `ClaudeSessionID` which may be stale or empty |
</phase_requirements>

---

## Summary

Phase 13 addresses two linked bugs reported in GitHub issue #311 affecting WSL/Linux users who invoke `agent-deck session start` from non-interactive contexts (scripts, systemd units, CI pipelines, or any shell without a controlling TTY). Both bugs share a common theme: the code path that launches tool processes inside tmux was designed for the interactive TUI case and has subtle assumptions that break in non-interactive scenarios.

**Bug 1 (PLAT-01):** Tool processes (Codex, Claude, Gemini) exit immediately after being launched from a non-interactive agent-deck invocation. The reporter confirmed `codex 2>&1 | head -40` produces `Error: stdout is not a terminal` when piped, but the same `codex` command typed manually in the same tmux pane works fine. This is strong evidence that stdout is being redirected or piped somewhere in the launch chain specifically in non-interactive mode. Three candidate failure modes are identified below, ranging from a timing race in the `send-keys` → `bash -c` layer to the tmux server requiring a running X display on certain Linux configurations.

**Bug 2 (PLAT-02):** Resume after an auto-started session attaches to the wrong conversation. The reporter saw `No conversation found with session ID: <id>` where the ID was agent-deck's internal UUID, not the tool's own conversation ID. The `PostStartSync` call in `handleSessionStart` does wait for the tmux environment variable, but `handleSessionStop` does not capture or persist the tool session ID before killing the tmux session; if `PostStartSync` times out (e.g., because the tool never started due to Bug 1), `ClaudeSessionID` is never saved to storage.

These two bugs are causally linked: PLAT-01 causes the tool to exit immediately without generating a conversation ID, which means PLAT-02 has nothing to resume. Fixing PLAT-01 is prerequisite to verifying PLAT-02 in practice.

**Primary recommendation:** Add a `--no-attach` / `tmux new-session -d` path that explicitly verifies the pane shell is ready before issuing `send-keys`, and ensure `handleSessionStop` captures the tool conversation ID to storage before killing the session.

---

## Standard Stack

### Core
| Component | Version | Purpose | Notes |
|-----------|---------|---------|-------|
| `tmux` | 3.x | Session + PTY management | `new-session -d` always allocates a PTY for the pane |
| `internal/tmux/tmux.go` | project | Session lifecycle | `Start()` is the single creation point; `SendKeysAndEnter` dispatches the command |
| `internal/session/instance.go` | project | Tool command building | `buildClaudeCommand`, `buildCodexCommand`, `prepareCommand`, `PostStartSync` |
| `internal/platform/platform.go` | project | WSL/Linux detection | `IsWSL()`, `IsWSL1()`, `IsWSL2()` already implemented |
| `cmd/agent-deck/session_cmd.go` | project | CLI entry point | `handleSessionStart`, `handleSessionStop` |

### Supporting
| Component | Version | Purpose | When to Use |
|-----------|---------|---------|-------------|
| `golang.org/x/term` | current | `term.IsTerminal(fd)` | Detecting non-interactive invocation at CLI entry point |
| `os.Stdin` / `os.Getenv("TERM")` | stdlib | Heuristics for interactivity | Fallback TTY detection when `term.IsTerminal` is ambiguous |

---

## Architecture Patterns

### Existing Launch Flow

```
handleSessionStart (session_cmd.go)
  └── inst.Start() or inst.StartWithMessage()  (instance.go)
        └── buildClaudeCommand / buildCodexCommand / ...  (instance.go)
              → prepareCommand: applyWrapper → wrapForSSH → wrapForSandbox → wrapIgnoreSuspend
              → tmuxSession.Start(command)  (tmux.go)
                    → tmux new-session -d -s <name> -c <workdir>
                    → batch set-option calls
                    → ConfigureStatusBar
                    → SendKeysAndEnter(wrappedCommand)   ← pane shell must be ready here
  └── inst.PostStartSync(3s)   ← polls tmux env for CLAUDE_SESSION_ID
  └── saveSessionData           ← saves ClaudeSessionID to SQLite
```

### Candidate Failure Modes for PLAT-01

**Mode A (most likely on WSL): Timing race between `new-session` and `send-keys`**

When `tmux new-session -d` starts and no tmux server is running, the server boots as a daemon subprocess. On WSL2 under Windows Terminal, the server startup includes acquiring a pseudo-TTY from the Windows conhost layer. This can take 100-500ms on slow Windows machines or when running from a non-interactive context (no attached terminal). The current code issues `ConfigureStatusBar` (which spawns multiple subprocess calls) before `SendKeysAndEnter`, and the pane shell may not be ready for input yet. On macOS and interactive Linux, the pane shell is ready instantly; on WSL2 without an attached terminal, it may not be.

Evidence: Issue says "commands are printed but not executed" — this matches a race where `send-keys` fires before the shell prompt is ready. The keystrokes arrive in the pane but are swallowed by the initial tmux pane setup rather than being processed by the shell.

**Mode B: stdout redirect from bash -c wrapping**

`wrapIgnoreSuspend` wraps every command in `bash -c 'stty susp undef; ...'`. In `tmux.go:Start()` at line 1177, if the command contains `$(` or `session_id=`, it is ADDITIONALLY wrapped in `bash -c '...'`. For Claude's command (`session_id=$(uuidgen ...); ...`), this produces double-nested `bash -c` calls. In certain WSL configurations, the outer `bash -c` launched from `send-keys` may inherit a redirected stdout from the calling shell environment — specifically if `agent-deck` was itself invoked with stdout redirected (e.g., `agent-deck session start myproj >> log.txt 2>&1`). Any child shell of the tmux pane that inherits stdout from the calling process rather than the pane PTY will see a non-TTY stdout.

Evidence: `codex 2>&1 | head` shows the error. The `|` redirect causes the non-TTY stdout. If `send-keys` somehow triggers a similar redirect, the behaviour would match.

**Mode C: tmux server not running, WSL2 requires display**

On some WSL2 configurations without `DISPLAY` set (headless or running from systemd), `tmux new-session -d` may fail silently or produce a server that does not properly initialize the pane PTY. This is unlikely with modern WSL2 + tmux 3.x but possible with mismatched tmux builds (e.g., a distro tmux compiled with X11 support).

Evidence: Lower confidence, no direct evidence in the issue.

**Mode D: Codex / Claude CLI checks stdout of the WRAPPER, not the pane**

The command sent to the pane shell is `bash -c 'stty susp undef; AGENTDECK_INSTANCE_ID=... codex'`. If `bash -c` is invoked in a non-interactive context where `bash` itself has a non-TTY stdin/stdout (e.g., called from a send-keys that forks a non-interactive shell), the tool process's stdout IS the pane PTY... unless bash -c in non-interactive mode doesn't set up the child process's stdio correctly. This is related to Mode B.

### Recommended Fix Strategy

**For PLAT-01:** Add an explicit pane-ready wait before `SendKeysAndEnter`. Use `tmux wait-for-pane` or a polling loop that calls `tmux display-message -p '#{pane_start_command}'` or checks `tmux list-panes` to confirm the pane shell is in a ready state before dispatching keys.

The most robust approach used by other tools (e.g., tmuxinator, tmux-sessionizer) is to wait until `capture-pane` shows a shell prompt before sending the command. The codebase already has prompt detection patterns in `internal/tmux/patterns.go`; these can be reused.

```go
// In tmux.go Start(), replace direct SendKeysAndEnter with:
if err := s.waitForPaneReady(5 * time.Second); err != nil {
    // Log but don't fail — attempt send-keys anyway
    statusLog.Warn("pane_ready_timeout", slog.String("session", s.Name))
}
if err := s.SendKeysAndEnter(cmdToSend); err != nil {
    return fmt.Errorf("failed to send command: %w", err)
}
```

`waitForPaneReady` polls `capture-pane -p` and checks for a shell prompt pattern (e.g., `$`, `%`, `#`, `>`) at the end of output.

**For PLAT-02:** Ensure `handleSessionStop` (and any path that kills a tmux session) snapshots the tool conversation ID from the tmux environment into the `Instance` struct and persists it to storage before killing. Currently, `handleSessionStop` calls `inst.Kill()` then `saveSessionData`, but `inst.ClaudeSessionID` may be empty if `PostStartSync` timed out. The fix: call `inst.PostStartSync(1 * time.Second)` or a lighter `inst.SyncSessionIDsFromTmux()` before kill, so the stored ID reflects the tool's actual conversation.

Additionally, the `buildClaudeCommandWithMessage` resume path checks `sessionHasConversationData(opts.ResumeSessionID, ...)` — if the Claude session was created via `--session-id` but never had a message sent, it falls back to `--session-id` mode (line 464-469). This is correct but depends on `opts.ResumeSessionID` being set, which requires `ClaudeSessionID` to have been saved. Making the stop path reliable (above) resolves this chain.

### Don't Hand-Roll

| Problem | Don't Build | Use Instead |
|---------|-------------|-------------|
| Detecting non-interactive shell | Custom env var checks | `term.IsTerminal(int(os.Stdin.Fd()))` from `golang.org/x/term` |
| Waiting for pane shell prompt | Custom sleep loop | Poll `tmux capture-pane -p` and check tail of output for prompt chars |
| WSL detection | Custom `/proc/version` reading | `platform.IsWSL()` already in `internal/platform/platform.go` |

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Detecting interactive terminal | Custom heuristics | `golang.org/x/term.IsTerminal` | Handles all Unix PTY cases correctly including WSL |
| Pane shell readiness | Fixed sleep delays | `capture-pane` polling with prompt detection | Timing varies across hardware; polling is reliable |
| WSL2 vs WSL1 distinction | New detection code | Existing `platform.Detect()` | Already handles all cases including `/run/WSL` check |

---

## Common Pitfalls

### Pitfall 1: Fixed Sleep Before send-keys
**What goes wrong:** Adding `time.Sleep(500ms)` before `SendKeysAndEnter` "fixes" slow machines but regresses fast machines and doesn't fix very slow WSL2 cold-start scenarios.
**Why it happens:** Developers test on fast hardware where the shell is ready in <100ms.
**How to avoid:** Use polling with a timeout: call `capture-pane` in a loop until output ends with a prompt or until a 5s deadline.
**Warning signs:** Tests pass on CI (macOS/Linux) but fail on WSL2 users' machines.

### Pitfall 2: Saving ClaudeSessionID After Kill
**What goes wrong:** Calling `inst.Kill()` before reading `CLAUDE_SESSION_ID` from the tmux environment means the ID is lost — `tmux show-environment` on a dead session returns an error.
**Why it happens:** The current `handleSessionStop` calls `Kill()` then `saveSessionData`. If `ClaudeSessionID` was never set (because `PostStartSync` timed out), it stays empty in storage.
**How to avoid:** Read session IDs from tmux env before calling Kill(). Add a `SyncSessionIDsFromTmux()` call in the stop path.
**Warning signs:** `agent-deck session stop` followed by `agent-deck session start` launches a fresh conversation instead of resuming.

### Pitfall 3: Double bash -c Wrapping Breaks TTY Detection
**What goes wrong:** `wrapIgnoreSuspend` wraps the command in `bash -c 'stty susp undef; ...'`. Then `tmux.go:Start()` additionally wraps bash-syntax commands in `bash -c '...'` for fish compatibility. Tools like Codex that call `isatty(STDOUT_FILENO)` check the real file descriptor of the running process, not their parent shell.
**Why it happens:** The double wrapping is an artifact of two independent fixes — fish compatibility (#47) and the suspend-ignore wrapper.
**How to avoid:** The wrapping is correct; the PTY inheritance should be preserved through `bash -c` invocations. Verify by running `bash -c 'tty; ls -la /proc/self/fd/1'` inside a tmux pane started from a non-interactive context.
**Warning signs:** `bash -c 'ls -la /proc/self/fd/1'` shows a PTY in interactive mode but not in non-interactive mode.

### Pitfall 4: `PostStartSync` Timeout Silently Discards the Session ID
**What goes wrong:** `PostStartSync(3 * time.Second)` polls for `CLAUDE_SESSION_ID`. If the tool never starts (Bug 1), this polls for 3s then returns empty string. `saveSessionData` then saves an empty `ClaudeSessionID`, and subsequent resume attempts fail.
**Why it happens:** The function has no error return — callers don't know if it timed out.
**How to avoid:** Log a warning when `WaitForClaudeSession` returns empty, and flag the instance for ID-less resume. The resume path in `buildClaudeCommandWithMessage` already handles this (falls back to `--session-id` with a newly generated UUID), so this is a degraded-but-functional path.
**Warning signs:** `agent-deck session show <id>` shows empty `claude_session_id` after start.

---

## Code Examples

### Pane Ready Detection (new pattern to add)

```go
// waitForPaneReady polls capture-pane until the pane shell shows a prompt.
// Returns nil once ready, or an error if the deadline passes.
// Callers should proceed with SendKeysAndEnter even if this returns an error,
// as the command may still succeed on a slow-starting pane.
func (s *Session) waitForPaneReady(timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    interval := 100 * time.Millisecond
    for time.Now().Before(deadline) {
        output, err := s.CapturePane()
        if err == nil && isPaneShellReady(output) {
            return nil
        }
        time.Sleep(interval)
    }
    return fmt.Errorf("pane not ready after %s", timeout)
}

// isPaneShellReady returns true when the last non-empty line looks like a shell prompt.
// Supports bash ($), zsh (%), fish (>), root (#), and common prompt endings.
func isPaneShellReady(output string) bool {
    lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
    for i := len(lines) - 1; i >= 0; i-- {
        line := strings.TrimSpace(lines[i])
        if line == "" {
            continue
        }
        // Last non-empty line ends with a typical shell prompt character
        return strings.HasSuffix(line, "$") ||
            strings.HasSuffix(line, "%") ||
            strings.HasSuffix(line, "#") ||
            strings.HasSuffix(line, ">")
    }
    return false
}
```

### Sync IDs Before Kill (fix for PLAT-02)

```go
// In handleSessionStop (session_cmd.go), before inst.Kill():
// Snapshot tool conversation IDs from tmux env into instance struct.
// If tmux env has IDs that were not yet saved (e.g., PostStartSync timed out),
// they will now be included in saveSessionData.
inst.SyncSessionIDsFromTmux()  // reads tmux show-environment into inst fields

if err := inst.Kill(); err != nil {
    // ... existing error handling
}
if err := saveSessionData(storage, instances); err != nil {
    // ... existing error handling
}
```

`SyncSessionIDsFromTmux` already exists in `instance.go` as `SyncSessionIDsToTmux` (which pushes TO tmux); we need the reverse: `SyncSessionIDsFromTmux` that reads FROM tmux and updates the instance struct. This is a small addition mirroring `WaitForClaudeSession` but without polling.

### Platform-Aware Wait Strategy

```go
// In tmux.go Start(), use platform-aware timeout for pane readiness:
import "github.com/asheshgoplani/agent-deck/internal/platform"

paneReadyTimeout := 2 * time.Second
if platform.IsWSL() {
    // WSL2 cold-start (no prior tmux server) can take up to 3-4s
    paneReadyTimeout = 5 * time.Second
}
_ = s.waitForPaneReady(paneReadyTimeout) // non-fatal
```

---

## State of the Art

| Old Approach | Current Approach | Change | Impact |
|--------------|-----------------|--------|--------|
| Sleep before send-keys | None (fire immediately after ConfigureStatusBar) | N/A | Timing-dependent; works on fast machines, fails on WSL2 cold-start |
| Capture-resume (API call to get session ID) | Pre-generate UUID via `uuidgen`, pass `--session-id` | Claude CLI 2.1.x | Instant, no API call at start |
| Double bash -c for fish compat + ignore-suspend | `prepareCommand` chain | Current | Two layers of shell wrapping; verify PTY propagation |

---

## Open Questions

1. **Is Mode A (timing race) the actual root cause?**
   - What we know: Reporter sees "command printed but not executed" which matches send-keys fired before shell ready
   - What's unclear: Whether this is timing or the shell silently discarding keys due to a pane initialization issue
   - Recommendation: Add debug logging around `SendKeysAndEnter` on WSL, and add `waitForPaneReady` defensively

2. **Does `tmux new-session -d` on WSL2 without a running server behave differently than on macOS?**
   - What we know: tmux allocates a PTY regardless of the calling context
   - What's unclear: Whether WSL2's conhost-backed PTY has longer initialization time vs native Linux
   - Recommendation: Test empirically on WSL2 by running `time tmux new-session -d -s test && time tmux send-keys -t test 'echo hello' Enter` from a non-interactive context

3. **Is PLAT-02 fully resolved once PLAT-01 is fixed?**
   - What we know: If the tool starts successfully, `PostStartSync` captures the ID. The resume path in `buildClaudeCommandWithMessage` correctly uses `--resume` for sessions with conversation data
   - What's unclear: Whether there are edge cases where the tool starts but CLAUDE_SESSION_ID is not yet set in the tmux env within the 3s window (e.g., slow WSL2 startup)
   - Recommendation: Add `SyncSessionIDsFromTmux` before kill as a belt-and-suspenders fix regardless

4. **Does the fix require platform-specific code or is a universal pane-ready wait sufficient?**
   - What we know: The platform package already detects WSL1/WSL2; a universal wait would be safe
   - What's unclear: Whether the wait hurts macOS/Linux performance measurably
   - Recommendation: Universal wait (100ms polling, 2-5s timeout) that's a no-op when the pane is already ready; make the timeout platform-aware for WSL

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (existing) |
| Config file | None (go test flags) |
| Quick run command | `go test -race -v ./internal/tmux/... ./internal/session/... -run TestPaneReady` |
| Full suite command | `make test` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PLAT-01 | Pane shell is ready before send-keys fires | unit | `go test -race -v ./internal/tmux/... -run TestWaitForPaneReady` | ❌ Wave 0 |
| PLAT-01 | `isPaneShellReady` correctly identifies prompt patterns | unit | `go test -race -v ./internal/tmux/... -run TestIsPaneShellReady` | ❌ Wave 0 |
| PLAT-01 | `Start()` with platform-aware wait integrates correctly | integration (needs tmux) | `go test -race -v ./internal/tmux/... -run TestStartWithPaneReady` | ❌ Wave 0 |
| PLAT-02 | `SyncSessionIDsFromTmux` reads correct IDs from tmux env | unit | `go test -race -v ./internal/session/... -run TestSyncSessionIDsFromTmux` | ❌ Wave 0 |
| PLAT-02 | Stop path saves tool conversation ID before kill | unit/integration | `go test -race -v ./internal/session/... -run TestStopSavesSessionID` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race -v ./internal/tmux/... ./internal/session/... -run TestPaneReady\|TestSyncSessionIDs\|TestStopSavesSessionID`
- **Per wave merge:** `make test`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/tmux/pane_ready_test.go` — covers PLAT-01 (isPaneShellReady, waitForPaneReady)
- [ ] `internal/session/instance_test.go` additions — covers PLAT-02 (SyncSessionIDsFromTmux, stop-saves-id)
- Note: Tests that require a running tmux server must call `skipIfNoTmuxServer(t)` per project convention

---

## Sources

### Primary (HIGH confidence)
- `/Users/ashesh/claude-deck/internal/tmux/tmux.go` — `Start()` function at line 1073; `SendKeysAndEnter` at line 3039; `ConfigureStatusBar` invocation at line 1171
- `/Users/ashesh/claude-deck/internal/session/instance.go` — `buildClaudeCommand` at line 402; `buildCodexCommand` at line 719; `prepareCommand` at line 4729; `wrapIgnoreSuspend` at line 4990; `PostStartSync` at line 2753; `SyncSessionIDsToTmux` at line 2819
- `/Users/ashesh/claude-deck/cmd/agent-deck/session_cmd.go` — `handleSessionStart` at line 117; `handleSessionStop` at line 224
- `/Users/ashesh/claude-deck/internal/platform/platform.go` — `IsWSL()`, `IsWSL2()` implementations

### Secondary (MEDIUM confidence)
- GitHub issue #311 — reporter's evidence: `codex 2>&1 | head` shows TTY error; command works manually in same pane; "command printed but Enter not sent" for resume
- `golang.org/x/term` package — `IsTerminal` API is stable and well-documented

### Tertiary (LOW confidence — requires live WSL2 reproduction to confirm)
- Mode A (timing race) is the most probable root cause based on issue symptom matching, but has not been confirmed without reproduction on WSL2
- The double `bash -c` wrapping as a contributing factor has not been directly tested

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all relevant code is in-codebase and directly read
- Architecture: HIGH — the launch flow is well-understood from source
- Root cause: MEDIUM — three candidate modes identified from evidence; Mode A (timing race) most likely; reproduction on WSL2 required to confirm
- Fix strategy: MEDIUM-HIGH — `waitForPaneReady` pattern is standard practice; `SyncSessionIDsFromTmux` is a small, targeted addition

**Research date:** 2026-03-13
**Valid until:** 2026-04-13 (stable domain; Go + tmux APIs change slowly)

**Critical note from STATE.md:**
> Root cause on WSL/Linux NOT confirmed without reproduction; three candidate failure modes identified. Flag for hands-on debugging session on WSL/Linux before writing implementation tasks.

This research documents all three candidates and provides a fix strategy for the most probable one (Mode A). The planner should scope the first plan as a hypothesis-driven investigation: add the `waitForPaneReady` + `SyncSessionIDsFromTmux` fixes with logging, then test on a real WSL2 system to confirm the root cause before finalising.
