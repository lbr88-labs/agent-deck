# Phase 13: Auto-Start & Platform - Research

**Researched:** 2026-03-13
**Domain:** tmux session launch, PTY allocation, WSL/Linux non-interactive contexts, tool conversation ID propagation
**Confidence:** MEDIUM (root cause on WSL/Linux not confirmed without live reproduction; analysis from issue #311 evidence + full codebase read)

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PLAT-01 | Auto-start works from non-interactive contexts on WSL/Linux; tool processes receive a PTY (#311) | Root-cause analysis identifies four candidate failure modes in `tmux.go:Start()` and the `send-keys` dispatch path; fix strategies documented below |
| PLAT-02 | Resume after auto-start uses correct tool conversation ID (not agent-deck internal UUID) (#311) | `PostStartSync` + `WaitForClaudeSession` correctly captures the ID for Claude, but `handleSessionStop` does not snapshot tmux-env before kill; if `PostStartSync` timed out because the tool never started (PLAT-01 failure), `ClaudeSessionID` is never persisted |
</phase_requirements>

---

## Summary

Phase 13 addresses two linked bugs reported in GitHub issue #311, confirmed on WSL2 for all three major tools: Codex, Claude, and Gemini CLI. Both bugs share a common theme: the launch path was designed for the interactive TUI case and has assumptions that break in non-interactive scenarios (scripts, systemd units, CI pipelines, or any shell without a controlling TTY).

**Bug 1 (PLAT-01):** Tool processes exit immediately after being launched from a non-interactive agent-deck invocation. The reporter confirmed `codex 2>&1 | head -40` produces `Error: stdout is not a terminal` when piped, but the same `codex` command typed manually in the same tmux pane works correctly. This is strong evidence that stdout is being redirected or piped in the launch chain when running from a non-interactive context. Four candidate failure modes are identified, of which Mode A (timing race between `new-session` and `send-keys`) and Mode E (`uuidgen` not installed) are most actionable without live reproduction.

**Bug 2 (PLAT-02):** Resume after an auto-started session uses the wrong ID. The reporter saw `No conversation found with session ID: <id>` where the ID was agent-deck's internal UUID, not the tool's own conversation ID. The `PostStartSync` call in `handleSessionStart` polls the tmux environment variable, but `handleSessionStop` does not snapshot that variable before killing the session; if `PostStartSync` timed out (because Bug 1 prevented the tool from starting), `ClaudeSessionID` is never saved to storage.

These two bugs are causally linked: PLAT-01 causes the tool to exit without generating a conversation ID, which means PLAT-02 has nothing to resume. Fixing PLAT-01 is prerequisite to verifying PLAT-02 in practice.

**Primary recommendation:** Add a `waitForPaneReady` poll before `SendKeysAndEnter`, replace shell-level `uuidgen` with a Go-generated UUID embedded in the command string, and ensure `handleSessionStop` calls `SyncSessionIDsFromTmux` before killing the session.

---

## Standard Stack

### Core
| Component | Version | Purpose | Notes |
|-----------|---------|---------|-------|
| `tmux` | 3.x | Session + PTY management | `new-session -d` always allocates a PTY for the pane; send-keys is the delivery mechanism |
| `internal/tmux/tmux.go` | project | Session lifecycle | `Start()` is the single creation point (line 1073); `SendKeysAndEnter` at line 3039 |
| `internal/session/instance.go` | project | Tool command building | `buildClaudeCommand` (line 402), `buildCodexCommand` (line 719), `prepareCommand` (line 4729), `PostStartSync` (line 2753) |
| `internal/platform/platform.go` | project | WSL/Linux detection | `IsWSL()`, `IsWSL1()`, `IsWSL2()` already implemented |
| `cmd/agent-deck/session_cmd.go` | project | CLI entry point | `handleSessionStart` (line 117), `handleSessionStop` (line 224) |

### Supporting
| Component | Version | Purpose | When to Use |
|-----------|---------|---------|-------------|
| `golang.org/x/term` | current | `term.IsTerminal(fd)` | Detecting non-interactive invocation at CLI entry point |
| `crypto/rand` (stdlib) | Go 1.24 | UUID generation in Go | Replace shell `uuidgen` dependency; already used by `randomString()` in instance.go |

### Key Existing Functions

| Function | File | Purpose |
|----------|------|---------|
| `SyncSessionIDsToTmux()` | instance.go:2819 | Pushes Instance IDs to tmux env vars |
| `WaitForClaudeSession()` | instance.go:2716 | Polls tmux env for CLAUDE_SESSION_ID (up to maxWait) |
| `PostStartSync()` | instance.go:2753 | CLI-only: calls WaitForClaudeSession then saves |
| `GetSessionIDFromTmux()` | instance.go:4361 | Reads CLAUDE_SESSION_ID from tmux show-environment |
| `generateID()` | instance.go:4934 | Uses crypto/rand for agent-deck internal IDs |
| `randomString()` | instance.go:4939 | crypto/rand hex; model for Go-side UUID generation |

---

## Architecture Patterns

### Existing Launch Flow

```
handleSessionStart (session_cmd.go)
  └── inst.Start() or inst.StartWithMessage()  (instance.go)
        └── buildClaudeCommand / buildCodexCommand / buildGeminiCommand / ...
              → prepareCommand: applyWrapper → wrapForSSH → wrapForSandbox → wrapIgnoreSuspend
              → tmuxSession.Start(command)  (tmux.go)
                    → tmux new-session -d -s <name> -c <workdir>
                    → batch set-option calls (7 options, one subprocess)
                    → ConfigureStatusBar (1 subprocess)
                    → SendKeysAndEnter(wrappedCommand)   ← pane shell must be ready here
  └── inst.PostStartSync(3s)   ← polls tmux env for CLAUDE_SESSION_ID
  └── saveSessionData           ← saves ClaudeSessionID to SQLite
```

### Claude Command Shape (current)

```bash
# The command sent to the pane shell via send-keys:
bash -c 'stty susp undef; bash -c '\''session_id=$(uuidgen | tr '\''[:upper:]'\'' '\''[:lower:]'\''); tmux set-environment CLAUDE_SESSION_ID "$session_id"; export AGENTDECK_INSTANCE_ID=<id>; claude --session-id "$session_id"'\'''
```

Key observations:
- Double `bash -c` wrapping: outer from `wrapIgnoreSuspend`, inner from fish-compat wrap in `tmux.go:Start()` at line 1177
- `uuidgen` is a shell command (requires `uuid-runtime` package on Ubuntu/Debian)
- `CLAUDE_SESSION_ID` is set in tmux env before `claude` exec, so `WaitForClaudeSession` can read it

### Codex Command Shape (current)

```bash
# Fresh Codex session:
bash -c 'stty susp undef; AGENTDECK_INSTANCE_ID=<id> AGENTDECK_TITLE="..." AGENTDECK_TOOL=codex codex'
```

Codex does NOT use `uuidgen`. Its session ID is detected asynchronously after startup via `detectCodexSessionAsync()`.

### Candidate Failure Modes for PLAT-01

**Mode A (most likely): Timing race between `new-session` and `send-keys`**

When `tmux new-session -d` starts and no tmux server is running, the server boots as a daemon subprocess. On WSL2 under Windows Terminal, server startup includes acquiring a pseudo-TTY from the Windows conhost layer. This can take 100-500ms on slow hardware or in a non-interactive context. The current code issues `ConfigureStatusBar` (multiple subprocess calls) before `SendKeysAndEnter`, and the pane shell may not be ready for input yet.

Evidence: The reporter says "commands are printed but not executed" — this matches a race where `send-keys` fires before the shell prompt is ready. Keystrokes arrive but are swallowed by the pane initialization rather than processed by the shell.

**Mode B: stdout redirect from bash -c wrapping**

`wrapIgnoreSuspend` wraps in `bash -c 'stty susp undef; ...'`. `tmux.go:Start()` at line 1177 additionally wraps commands containing `$(` or `session_id=` in another `bash -c`. Tools like Codex call `isatty(STDOUT_FILENO)` on their own process's fd. If `bash -c` forks with a non-PTY stdout (e.g., if agent-deck itself was invoked with stdout redirected), the child process sees a non-TTY.

Evidence: `codex 2>&1 | head` reproduces the error. `|` redirects stdout to a pipe (non-TTY). If the launch context passes a similar pipe, behaviour matches.

**Mode C: tmux server not running, WSL2 display issues**

On some WSL2 configurations without `DISPLAY` set (headless or systemd-activated), `tmux new-session -d` may fail to properly initialize the pane PTY. This is unlikely with modern WSL2 + tmux 3.x but possible with distro tmux built with X11 support.

Evidence: Lower confidence, no direct evidence in issue.

**Mode D: Codex/Claude check stdout of the wrapper process**

`bash -c 'stty susp undef; ... codex'` — if `bash -c` itself runs in a non-interactive context where bash has non-TTY stdio (possible if the calling shell that invoked `send-keys` had redirected stdio), the child process's stdout IS the pane PTY but may be overridden.

**Mode E: `uuidgen` not available on the WSL/Linux system**

`uuidgen` is part of the `util-linux` package on most Linux distributions, but on minimal Ubuntu/Debian WSL installations it may require the `uuid-runtime` package explicitly. If `uuidgen` is not found, the shell command:

```bash
session_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
```

silently fails (exits 127 with empty output), so `$session_id` is empty. `tmux set-environment CLAUDE_SESSION_ID ""` is then called with an empty string. Claude then starts without `--session-id`, which may cause it to start in an unexpected mode or use its own generated ID. This does NOT cause an immediate exit, but means `WaitForClaudeSession` times out (no ID in tmux env), and PLAT-02 fails.

Evidence: On Ubuntu 22.04 minimal (common WSL2 base), `uuidgen` is present via `util-linux` (pre-installed). However on custom Docker-derived or minimal WSL images, it may not be. The issue reporter uses NVM-installed Codex/Claude, suggesting a non-standard setup.

**Definitive fix for Mode E:** Replace shell `uuidgen` with a pre-generated Go UUID embedded in the command string. The Go process already has `crypto/rand` via `randomString()` at line 4939. This eliminates the external dependency entirely.

### Recommended Fix Strategy

**For PLAT-01 (Mode A — timing race):** Add an explicit pane-ready wait before `SendKeysAndEnter`. Poll `capture-pane -p` in a loop until the tail of output shows a shell prompt.

```go
// In tmux.go Start(), before SendKeysAndEnter:
if err := s.waitForPaneReady(5 * time.Second); err != nil {
    // Log but do not fail — attempt send-keys anyway
    statusLog.Warn("pane_ready_timeout", slog.String("session", s.Name))
}
if err := s.SendKeysAndEnter(cmdToSend); err != nil {
    return fmt.Errorf("failed to send command: %w", err)
}
```

`waitForPaneReady` polls `CapturePane()` and checks for a shell prompt at the end of output (see Code Examples below). The polling interval is 100ms; timeout is platform-aware (2s on macOS/Linux, 5s on WSL).

**For PLAT-01 (Mode E — uuidgen dependency):** Replace shell-level UUID generation with Go-side generation:

```go
// In instance.go, buildClaudeCommand():
// Before (current):
//   session_id=$(uuidgen | tr '[:upper:]' '[:lower:]'); tmux set-environment CLAUDE_SESSION_ID "$session_id"; ...
//
// After:
sessionUUID := generateUUID()  // Go-side, no external binary
cmd := fmt.Sprintf(
    `tmux set-environment CLAUDE_SESSION_ID "%s"; %s%s --session-id "%s"%s`,
    sessionUUID, bashExportPrefix, claudeCmd, sessionUUID, extraFlags)
```

`generateUUID()` uses `crypto/rand` (already available), formats as lowercase UUID. This also eliminates the `$(` substring that triggers the double-`bash -c` wrap in `tmux.go:Start()` at line 1177, potentially resolving Mode B as a side effect.

**For PLAT-02 (stop without saving ID):** Snapshot session IDs from tmux env before killing the session.

```go
// In handleSessionStop (session_cmd.go), before inst.Kill():
inst.SyncSessionIDsFromTmux()  // read tmux env into instance struct fields

if err := inst.Kill(); err != nil { ... }
if err := saveSessionData(storage, instances); err != nil { ... }
```

`SyncSessionIDsFromTmux` is the reverse of `SyncSessionIDsToTmux` (which already exists at instance.go:2819). It reads `CLAUDE_SESSION_ID`, `GEMINI_SESSION_ID`, `CODEX_SESSION_ID`, `OPENCODE_SESSION_ID` from tmux env and writes them to the Instance struct fields.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| UUID generation in shell | `$(uuidgen ...)` in bash command strings | `crypto/rand` in Go, embed result in command string | `uuidgen` may not be installed on minimal Linux/WSL; Go crypto/rand works everywhere |
| Detecting interactive terminal | Custom env var checks | `golang.org/x/term.IsTerminal` | Handles all Unix PTY cases including WSL |
| Pane shell readiness | Fixed `time.Sleep` | `CapturePane()` polling with prompt detection | Timing varies across hardware; polling is reliable |
| WSL2 vs WSL1 distinction | New `/proc/version` parsing | Existing `platform.Detect()` | Already handles all cases including `/run/WSL` check |

---

## Common Pitfalls

### Pitfall 1: Fixed Sleep Before send-keys
**What goes wrong:** Adding `time.Sleep(500ms)` before `SendKeysAndEnter` "fixes" slow WSL machines but regresses fast machines and does not fix very slow WSL2 cold-start scenarios.
**Why it happens:** Tests run on fast hardware where the shell is ready in <100ms.
**How to avoid:** Poll `CapturePane()` in a loop with a 5s timeout. If the pane shows a prompt, proceed. If the deadline passes, log a warning and proceed anyway (non-fatal).
**Warning signs:** Tests pass on CI (macOS/Linux) but fail on WSL2 users' machines.

### Pitfall 2: Saving ClaudeSessionID After Kill
**What goes wrong:** Calling `inst.Kill()` before reading `CLAUDE_SESSION_ID` from the tmux environment means the ID is lost. `tmux show-environment` on a dead session returns an error.
**Why it happens:** `handleSessionStop` calls `Kill()` then `saveSessionData`. If `ClaudeSessionID` was never set (PostStartSync timed out), it stays empty in storage.
**How to avoid:** Call `SyncSessionIDsFromTmux()` before `Kill()`. This is a one-liner addition.
**Warning signs:** `agent-deck session stop` followed by `agent-deck session start` launches a fresh conversation instead of resuming.

### Pitfall 3: Double bash -c Breaks When uuidgen Is Missing
**What goes wrong:** The command `session_id=$(uuidgen ...)` contains `$(` which triggers the fish-compat wrap in `tmux.go:Start()` at line 1177. If `uuidgen` is missing (exit 127), `$session_id` is empty string, `tmux set-environment CLAUDE_SESSION_ID ""` is called, and `WaitForClaudeSession` times out.
**Why it happens:** `uuidgen` is assumed to be universally available but is not on minimal Linux.
**How to avoid:** Pre-generate the UUID in Go before building the command string. Eliminates the `$(` expression entirely, which also removes the trigger for the double-wrap.
**Warning signs:** `agent-deck session show <id>` shows empty `claude_session_id` after start.

### Pitfall 4: PostStartSync Timeout Silently Discards Session ID
**What goes wrong:** `PostStartSync(3 * time.Second)` polls for `CLAUDE_SESSION_ID`. If the tool never starts (PLAT-01 failure), this polls for 3s then returns empty. `saveSessionData` saves an empty `ClaudeSessionID`, and subsequent resume attempts fail.
**Why it happens:** `WaitForClaudeSession` has no error return — callers do not know if it timed out.
**How to avoid:** Log a warning when `WaitForClaudeSession` returns empty. The resume path in `buildClaudeCommand` already handles empty `ResumeSessionID` gracefully (falls back to a new `--session-id`), so this is degraded-but-functional.
**Warning signs:** `agent-deck session show <id>` shows empty `claude_session_id` after start.

### Pitfall 5: Affecting Only Claude While Codex and Gemini Have Separate ID Flows
**What goes wrong:** Focusing PLAT-02 fix only on Claude's `CLAUDE_SESSION_ID` while ignoring `GEMINI_SESSION_ID` and `CODEX_SESSION_ID`.
**Why it happens:** Claude's capture-resume is the most visible flow, but all three tools store IDs in tmux env.
**How to avoid:** `SyncSessionIDsFromTmux` must read all four env vars: `CLAUDE_SESSION_ID`, `GEMINI_SESSION_ID`, `CODEX_SESSION_ID`, `OPENCODE_SESSION_ID`. The existing `SyncSessionIDsToTmux` at line 2819 handles all four; model the reverse on it.

---

## Code Examples

### waitForPaneReady (new function in tmux.go)

```go
// Source: pattern derived from existing CapturePane() in tmux.go
// waitForPaneReady polls capture-pane until the pane shell shows a prompt.
// Returns nil once ready, or an error if the deadline passes.
// Callers should proceed with SendKeysAndEnter even on error (non-fatal).
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
func isPaneShellReady(output string) bool {
    lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
    for i := len(lines) - 1; i >= 0; i-- {
        line := strings.TrimSpace(lines[i])
        if line == "" {
            continue
        }
        return strings.HasSuffix(line, "$") ||
            strings.HasSuffix(line, "%") ||
            strings.HasSuffix(line, "#") ||
            strings.HasSuffix(line, ">")
    }
    return false
}
```

### Platform-Aware Pane-Ready Timeout (in tmux.go Start())

```go
// Source: internal/platform/platform.go IsWSL()
import "github.com/asheshgoplani/agent-deck/internal/platform"

paneReadyTimeout := 2 * time.Second
if platform.IsWSL() {
    // WSL2 cold-start (no prior server) takes up to 3-4s
    paneReadyTimeout = 5 * time.Second
}
_ = s.waitForPaneReady(paneReadyTimeout) // non-fatal
if err := s.SendKeysAndEnter(cmdToSend); err != nil {
    return fmt.Errorf("failed to send command: %w", err)
}
```

### Go-Side UUID Generation (replacing shell uuidgen)

```go
// Source: pattern from randomString() at instance.go:4939
// generateUUID returns a lowercase UUID v4 using crypto/rand.
// Replaces the shell command: session_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
func generateUUID() string {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        // Fallback to timestamp-based pseudo-UUID (rare)
        return fmt.Sprintf("00000000-0000-0000-%016x", time.Now().UnixNano())
    }
    b[6] = (b[6] & 0x0f) | 0x40 // version 4
    b[8] = (b[8] & 0x3f) | 0x80 // variant bits
    return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
        b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
```

### SyncSessionIDsFromTmux (new method on Instance)

```go
// Source: model from SyncSessionIDsToTmux at instance.go:2819
// SyncSessionIDsFromTmux reads tool conversation IDs from tmux environment
// and updates the Instance struct fields. Call before Kill() to persist IDs.
func (i *Instance) SyncSessionIDsFromTmux() {
    if i.tmuxSession == nil || !i.tmuxSession.Exists() {
        return
    }
    if id, err := i.tmuxSession.GetEnvironment("CLAUDE_SESSION_ID"); err == nil && id != "" {
        i.ClaudeSessionID = id
    }
    if id, err := i.tmuxSession.GetEnvironment("GEMINI_SESSION_ID"); err == nil && id != "" {
        i.GeminiSessionID = id
    }
    if id, err := i.tmuxSession.GetEnvironment("OPENCODE_SESSION_ID"); err == nil && id != "" {
        i.OpenCodeSessionID = id
    }
    if id, err := i.tmuxSession.GetEnvironment("CODEX_SESSION_ID"); err == nil && id != "" {
        i.CodexSessionID = id
    }
}
```

### Stop Path Update (session_cmd.go handleSessionStop)

```go
// Before inst.Kill(), add:
inst.SyncSessionIDsFromTmux()

if err := inst.Kill(); err != nil {
    out.Error(fmt.Sprintf("failed to stop session: %v", err), ErrCodeInvalidOperation)
    os.Exit(1)
}
if err := saveSessionData(storage, instances); err != nil {
    out.Error(fmt.Sprintf("failed to save session state: %v", err), ErrCodeInvalidOperation)
    os.Exit(1)
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|-----------------|--------------|--------|
| Capture-resume (API call to get ID) | Pre-generate UUID via `uuidgen`, pass `--session-id` | Claude CLI 2.1.x | Instant start; but `uuidgen` is a shell dependency |
| Sleep before send-keys | None (fire immediately after ConfigureStatusBar) | N/A | Timing-dependent; works on fast hardware, fails on WSL2 cold-start |
| Double bash -c for fish compat | `tmux.go:Start()` at line 1177 wraps commands with `$(` | Current | Two bash -c layers; removing `uuidgen` eliminates the `$(` trigger |

**Deprecated/outdated:**
- Shell-level `uuidgen`: Replace with Go-side `crypto/rand` UUID. Eliminates external dependency and the `$(` substring that triggers double-wrapping.

---

## Open Questions

1. **Is Mode A (timing race) the actual root cause on WSL?**
   - What we know: "command printed but not executed" matches send-keys before shell ready
   - What's unclear: Whether the keystrokes are swallowed by pane initialization vs the shell silently ignoring them
   - Recommendation: Add `waitForPaneReady` + debug logging around `SendKeysAndEnter` on WSL; test on a real WSL2 system

2. **Does `tmux new-session -d` on WSL2 without a running server behave differently than on macOS?**
   - What we know: tmux allocates a PTY regardless of calling context
   - What's unclear: Whether WSL2's conhost-backed PTY has longer initialization time
   - Recommendation: Empirical test: `time tmux new-session -d -s test && time tmux send-keys -t test 'echo hello' Enter` from a non-interactive shell on WSL2

3. **Is PLAT-02 fully resolved once PLAT-01 is fixed?**
   - What we know: If the tool starts, `PostStartSync` captures the ID; resume path correctly uses `--resume` when `ClaudeSessionID` is non-empty
   - What's unclear: Edge case where tool starts but `CLAUDE_SESSION_ID` is not yet in tmux env within the 3s PostStartSync window
   - Recommendation: Add `SyncSessionIDsFromTmux` before kill as belt-and-suspenders regardless

4. **Does the `uuidgen` → Go-side UUID change affect the fish-compat double-wrap?**
   - What we know: The double-wrap triggers when command contains `$(` or `session_id=`. The new command template `tmux set-environment CLAUDE_SESSION_ID "abc-def-..."` contains neither, so the double-wrap is NOT triggered.
   - What's unclear: Whether removing the double-wrap has any side effects on other shells
   - Recommendation: Remove the outer bash -c wrap as a side effect of the UUID change, and verify on bash + zsh + fish

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (existing) |
| Config file | None (go test flags) |
| Quick run command | `go test -race -v ./internal/tmux/... ./internal/session/... -run TestPaneReady\|TestSync\|TestStop` |
| Full suite command | `make test` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PLAT-01 | `isPaneShellReady` correctly identifies prompt patterns (bash $, zsh %, fish >, root #) | unit | `go test -race -v ./internal/tmux/... -run TestIsPaneShellReady` | ❌ Wave 0 |
| PLAT-01 | `waitForPaneReady` returns nil when pane shows prompt, error on timeout | unit (mock CapturePane) | `go test -race -v ./internal/tmux/... -run TestWaitForPaneReady` | ❌ Wave 0 |
| PLAT-01 | `generateUUID()` returns valid lowercase UUID format | unit | `go test -race -v ./internal/session/... -run TestGenerateUUID` | ❌ Wave 0 |
| PLAT-01 | Claude command string no longer contains `uuidgen` or `$(` substring | unit | `go test -race -v ./internal/session/... -run TestBuildClaudeCommandNoUuidgen` | ❌ Wave 0 |
| PLAT-02 | `SyncSessionIDsFromTmux` reads all four IDs from tmux env into Instance fields | unit | `go test -race -v ./internal/session/... -run TestSyncSessionIDsFromTmux` | ❌ Wave 0 |
| PLAT-02 | Stop path calls `SyncSessionIDsFromTmux` before Kill, ID persisted to storage | integration | `go test -race -v ./internal/session/... -run TestStopSavesSessionID` | ❌ Wave 0 |
| PLAT-01 | `Start()` with `waitForPaneReady` integrates correctly (requires tmux server) | integration | `go test -race -v ./internal/tmux/... -run TestStartWithPaneReady` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race -v ./internal/tmux/... ./internal/session/... -run TestIsPaneShellReady\|TestWaitForPane\|TestGenerateUUID\|TestBuildClaudeCommand\|TestSyncSessionIDs\|TestStopSaves`
- **Per wave merge:** `make test`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/tmux/pane_ready_test.go` — covers `isPaneShellReady` and `waitForPaneReady` (PLAT-01)
- [ ] `internal/session/uuid_test.go` — covers `generateUUID` format validation (PLAT-01)
- [ ] `internal/session/instance_platform_test.go` — covers `SyncSessionIDsFromTmux`, command string checks, stop-saves-ID (PLAT-01 + PLAT-02)
- Note: Tests requiring a running tmux server must call `skipIfNoTmuxServer(t)` per project convention (CLAUDE.md)

---

## Sources

### Primary (HIGH confidence)
- `/Users/ashesh/claude-deck/internal/tmux/tmux.go` — `Start()` at line 1073; `SendKeysAndEnter` at line 3039; ConfigureStatusBar at line 1171; fish-compat wrap at line 1177
- `/Users/ashesh/claude-deck/internal/session/instance.go` — `buildClaudeCommand` at line 402; `buildCodexCommand` at line 719; `prepareCommand` at line 4729; `wrapIgnoreSuspend` at line 4990; `PostStartSync` at line 2753; `SyncSessionIDsToTmux` at line 2819; `randomString` at line 4939
- `/Users/ashesh/claude-deck/cmd/agent-deck/session_cmd.go` — `handleSessionStart` at line 117; `handleSessionStop` at line 224
- `/Users/ashesh/claude-deck/internal/platform/platform.go` — `IsWSL()`, `IsWSL2()` implementations

### Secondary (MEDIUM confidence)
- GitHub issue #311 — reporter evidence: `codex 2>&1 | head` shows TTY error; command works manually in same pane; all three tools affected (Codex, Claude, Gemini) confirmed in follow-up comment
- `golang.org/x/term` package — `IsTerminal` API is stable and well-documented

### Tertiary (LOW confidence — requires WSL2 live reproduction to confirm)
- Mode A (timing race) is the most probable root cause based on symptom matching, but has not been confirmed without live reproduction
- Mode E (`uuidgen` missing) is independently verifiable but unconfirmed in the reporter's specific environment

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all relevant code directly read from source
- Architecture: HIGH — launch flow is well-understood from source
- Root cause: MEDIUM — four candidate modes identified from evidence; Mode A (timing race) and Mode E (uuidgen) most actionable; live WSL2 reproduction needed to confirm
- Fix strategy: MEDIUM-HIGH — `waitForPaneReady` is standard practice; Go-side UUID eliminates `uuidgen` dependency; `SyncSessionIDsFromTmux` is a small targeted addition

**Research date:** 2026-03-13
**Valid until:** 2026-04-13 (stable domain; Go + tmux APIs change slowly)

**Critical note from STATE.md:**
> Root cause on WSL/Linux NOT confirmed without reproduction; three candidate failure modes identified. Flag for hands-on debugging session on WSL/Linux before writing implementation tasks.

This research documents all four candidate modes (A, B, C, E) and provides targeted fix strategies for the two most actionable ones (Mode A: timing race, Mode E: uuidgen). The planner should scope the first task as a hypothesis-driven fix: apply `waitForPaneReady` + Go-side UUID generation + `SyncSessionIDsFromTmux`, then test on a real WSL2 system to confirm the root cause before finalizing.
