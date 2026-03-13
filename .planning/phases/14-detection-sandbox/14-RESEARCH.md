# Phase 14: Detection & Sandbox - Research

**Researched:** 2026-03-13
**Domain:** Docker sandbox tmux environment propagation; OpenCode question tool waiting status detection
**Confidence:** HIGH

## Summary

Phase 14 has two tightly scoped, independent fixes. DET-01 resolves a structural flaw: commands built by `buildClaudeCommand`, `buildOpenCodeCommand`, `buildGeminiCommand`, `buildCodexCommand`, and `buildClaudeResumeCommand` embed `tmux set-environment` calls that execute **inside the Docker container** (via `docker exec`), but the tmux server lives on the **host**. The container has no access to the host Unix domain socket at `/tmp/tmux-<uid>/default`, so every `tmux set-environment` silently fails (confirmed by issue #266). Claude still launches because the separator is `;`, not `&&`, but `CLAUDE_SESSION_ID` and its equivalents are never stored. The `#320` sandbox config persistence fix (STORE-01 through STORE-03, Phase 11) is already merged, unblocking DET-01.

DET-02 is a pattern-coverage gap: OpenCode's `question` tool renders a selection UI with unique help-bar text (`"enter submit"`, `"esc dismiss"`, `"↑↓ select"`), none of which are in `DefaultRawPatterns("opencode").PromptPatterns`. The existing detector only covers the normal idle state (`"Ask anything"`, `"press enter to send"`). Additionally, issue #255 reports false-positive busy detection where sessions appear green (running) when they are actually waiting.

**Primary recommendation:** For DET-01, pre-generate all session IDs in Go on the host and call `i.tmuxSession.SetEnvironment(key, id)` after `tmuxSession.Start()` returns — removing `tmux set-environment` from all embedded shell strings. For DET-02, add `"enter submit"`, `"esc dismiss"`, and `"↑↓ select"` to `PromptPatterns` in `DefaultRawPatterns("opencode")` and mirror them in `detector.go`'s `HasPrompt` opencode case.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DET-01 | tmux set-environment works correctly inside Docker sandbox sessions now that sandbox config persistence is fixed (#266) | Root cause confirmed: all 7 `tmux set-environment` calls embedded in command strings execute inside `docker exec` where no tmux socket is reachable. Fix: strip these calls from embedded shell strings; call `i.tmuxSession.SetEnvironment()` from the Go host side after session start. |
| DET-02 | OpenCode waiting status detection triggers correctly when OpenCode presents the question tool prompt (#255) | Root cause confirmed: `PromptPatterns` for opencode do not include question-tool help-bar strings. Current detector only matches normal-idle patterns. Fix: add question-tool patterns to `DefaultRawPatterns("opencode")` and `detector.go`. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `os/exec` | 1.24 | UUID pre-generation on host (`exec.Command("uuidgen")`) | Already used for `generateID()` and `randomString()` in `instance.go` |
| `internal/tmux.Session.SetEnvironment` | local | Host-side tmux env store after session start | Already used for `AGENTDECK_INSTANCE_ID` and `COLORFGBG` at `instance.go:1806` |
| `internal/tmux/patterns.go` | local | `DefaultRawPatterns(toolName)` pattern tables | Canonical source for all tool detection patterns |
| `internal/tmux/detector.go` | local | `PromptDetector.HasPrompt()` logic | Existing `hasOpencodeBusyIndicator` guard must remain; add prompt patterns below it |

### No New Dependencies

Both fixes are pure logic changes within existing packages. No new libraries required.

## Architecture Patterns

### DET-01: tmux set-environment in Docker Sandbox

#### Root Cause Confirmed

`buildClaudeCommandWithMessage` generates a shell string like:

```bash
session_id=$(uuidgen | tr '[:upper:]' '[:lower:]'); \
  tmux set-environment CLAUDE_SESSION_ID "$session_id"; \
  claude --session-id "$session_id"
```

`prepareCommand()` then wraps this entire string in `docker exec`:

```bash
docker exec -it -e TERM=xterm-256color <container> bash -c '<above string>'
```

`tmux set-environment` inside the container cannot connect to the host socket. The `;` separator ensures Claude launches anyway, but `CLAUDE_SESSION_ID` is never written to the tmux environment — `GetSessionIDFromTmux()` finds nothing.

**`buildClaudeResumeCommand` already acknowledges the problem** (line 3884):

```go
// Use ";" (not "&&") so the tool command runs even if tmux set-environment
// fails — inside a Docker sandbox there is no tmux server.
```

This is a workaround, not a fix. The `2>/dev/null;` suppressor silences the error but the env var is still not set.

**All 7 affected call sites in `instance.go`:**

| Function / path | Env var | Approx line |
|---|---|---|
| `buildClaudeCommandWithMessage` — new session | `CLAUDE_SESSION_ID` | 491 |
| `buildClaudeCommandWithMessage` — message with send | `CLAUDE_SESSION_ID` | 503 |
| `buildClaudeCommandWithMessage` — resume | `CLAUDE_SESSION_ID` | 468 |
| `buildClaudeResumeCommand` — resume | `CLAUDE_SESSION_ID` | 3887 |
| `buildClaudeResumeCommand` — session-id | `CLAUDE_SESSION_ID` | 3891 |
| `buildOpenCodeCommand` — resume with ID | `OPENCODE_SESSION_ID` | 655 |
| `buildGeminiCommand` — resume with ID | `GEMINI_SESSION_ID` | 610 |
| `buildGeminiCommand` — fresh start | `GEMINI_YOLO_MODE` | 623 |
| `buildCodexCommand` — resume with ID | `CODEX_SESSION_ID` | 735 |
| Opencode respawn path in `Restart()` | `OPENCODE_SESSION_ID` | 3604, 3748 |

#### Recommended Fix Pattern

**For resume commands (session ID already known in Go):** Remove `tmux set-environment X Y;` from the command string. After `tmuxSession.Start()` returns, call `i.tmuxSession.SetEnvironment("X", Y)` from Go. The session ID is already stored on the `Instance` field before `Start()` is called.

```go
// BEFORE (buildClaudeResumeCommand, current):
return fmt.Sprintf("tmux set-environment CLAUDE_SESSION_ID %s 2>/dev/null; %s%s --resume %s%s",
    i.ClaudeSessionID, configDirPrefix, claudeCmd, i.ClaudeSessionID, dangerousFlag)

// AFTER:
return fmt.Sprintf("%s%s --resume %s%s", configDirPrefix, claudeCmd, i.ClaudeSessionID, dangerousFlag)
// And in Start()/Restart() after tmuxSession.Start():
_ = i.tmuxSession.SetEnvironment("CLAUDE_SESSION_ID", i.ClaudeSessionID)
```

**For new Claude sessions (UUID generated in shell):** The UUID is currently generated with `$(uuidgen | tr '[:upper:]' '[:lower:]')` inside the shell string. Pre-generate it in Go:

```go
// In Instance.Start() or buildClaudeCommandWithMessage, pre-generate the ID:
out, _ := exec.Command("uuidgen").Output()
sessionID := strings.ToLower(strings.TrimSpace(string(out)))

// Pass as literal into command string (no shell expansion needed):
baseCmd = fmt.Sprintf(
    `%s%s --session-id "%s"%s`,
    bashExportPrefix, claudeCmd, sessionID, extraFlags)

// After tmuxSession.Start() succeeds:
i.ClaudeSessionID = sessionID
_ = i.tmuxSession.SetEnvironment("CLAUDE_SESSION_ID", sessionID)
```

**For OpenCode, Gemini, Codex:** Session IDs are already known in Go before the command string is built. Simply remove the `tmux set-environment` prefix from the string and add a `SetEnvironment` call in the post-start path.

**Key code path:**

```
Instance.Start()
  → buildClaudeCommand()           // builds command string WITHOUT tmux set-env
  → prepareCommand()               // wrapper, SSH, sandbox wrapping
      → wrapForSandbox()           // wraps in docker exec
  → tmuxSession.Start(command)     // runs docker exec
  → tmuxSession.SetEnvironment()   // HOST-SIDE: stores CLAUDE_SESSION_ID correctly
```

The existing pattern at `instance.go:1806` (setting `AGENTDECK_INSTANCE_ID` and `COLORFGBG`) already follows this exact pattern — this fix makes session IDs consistent with it.

#### Sandbox-Only vs. Universal Fix

Issue #266 comments suggest applying the fix uniformly to all sessions (not just sandboxed ones) for simplicity. Non-sandbox sessions are not harmed by calling `SetEnvironment` from Go — the host-side tmux call is idempotent. The fix should be universal: clean up all `tmux set-environment` prefixes from command strings and rely solely on host-side Go calls.

### DET-02: OpenCode Question Tool Detection

#### Root Cause Confirmed

When OpenCode's `question` tool is active, the terminal pane shows a selection UI. The help bar at the bottom renders:

```
↑↓ select     enter submit     esc dismiss
```

This text is **exclusive to the question-tool waiting state** and does not appear during normal processing. The existing detector does not check for any of these strings.

Current `DefaultRawPatterns("opencode").PromptPatterns` (line 66):

```go
PromptPatterns: []string{"Ask anything", "press enter to send"},
```

Current `HasPrompt` for opencode in `detector.go` (line 49–58):

```go
if d.hasOpencodeBusyIndicator(content) {
    return false
}
return strings.Contains(content, "press enter to send") ||
    strings.Contains(content, "Ask anything") ||
    strings.Contains(content, "open code") ||
    d.hasLineEndingWith(content, ">")
```

Neither checks for question-tool help-bar strings.

#### Recommended Fix

Add question-tool patterns to `DefaultRawPatterns("opencode")`:

```go
// Source: internal/tmux/patterns.go DefaultRawPatterns() opencode case
PromptPatterns: []string{
    "Ask anything",
    "press enter to send",
    "enter submit",   // question tool help bar (exclusive to waiting state)
    "esc dismiss",    // question tool help bar (exclusive to waiting state)
},
```

Update `detector.go`'s `HasPrompt` opencode case to check the new strings:

```go
if d.hasOpencodeBusyIndicator(content) {
    return false
}
return strings.Contains(content, "press enter to send") ||
    strings.Contains(content, "Ask anything") ||
    strings.Contains(content, "open code") ||
    strings.Contains(content, "enter submit") ||  // question tool
    strings.Contains(content, "esc dismiss") ||   // question tool
    d.hasLineEndingWith(content, ">")
```

The `hasOpencodeBusyIndicator` guard remains unchanged. If the pulse spinner (`█ ▓ ▒ ░`) or `"esc interrupt"` is present, the busy check fires first and `HasPrompt` returns false regardless.

#### False Positive Busy Detection

Issue #255 also mentions sessions appearing busy (green) when actually idle. The likely cause: block characters `░` or `▓` appear in static OpenCode UI decorations (progress bars, borders). The `hasOpencodeBusyIndicator` at `detector.go:421` checks for these chars anywhere in the content.

The conservative fix is to check spinner chars only when accompanied by a busy text string on the same line (rather than anywhere in the full pane content). However, since no regression data exists and the primary ask is the question-tool detection, the false-positive fix is secondary. Flag as a follow-up if the simple prompt-pattern addition does not resolve the symptom.

### Recommended Project Structure (No New Files)

```
internal/session/
└── instance.go          # DET-01: remove tmux set-environment from all 7 call sites,
                         # add host-side SetEnvironment calls in Start() and Restart()
internal/tmux/
├── patterns.go          # DET-02: add question-tool patterns to DefaultRawPatterns("opencode")
└── detector.go          # DET-02: add question-tool pattern checks to HasPrompt opencode case
```

Test files to extend/add:

```
internal/tmux/
└── status_fixes_test.go  # Add: VALIDATION 8.0 for opencode question tool detection
internal/session/
└── instance_test.go      # Add: TestBuildClaudeCommand_NoTmuxSetEnv (sandbox + non-sandbox)
                          #      TestBuildOpenCodeCommand_NoTmuxSetEnv
                          #      TestBuildGeminiCommand_NoTmuxSetEnv
                          #      TestBuildCodexCommand_NoTmuxSetEnv
internal/integration/
└── detection_test.go     # Add: TestDetection_OpenCodeQuestionTool
```

### Anti-Patterns to Avoid

- **Keeping `2>/dev/null;` after removing `tmux set-environment`:** Once the call is removed from the shell string, the suppressor becomes dead code. Remove it.
- **Guarding the fix with `if i.IsSandboxed()`:** Non-sandbox sessions are not harmed by the host-side `SetEnvironment` pattern and benefit from cleaner command strings.
- **Calling `SetEnvironment` before `tmuxSession.Start()`:** The tmux session does not exist yet. Always call after `Start()` returns without error.
- **Broad OpenCode prompt patterns (e.g., "Build", "Plan"):** These strings are visible in the OpenCode TUI header at all times, causing false positives. Only add patterns exclusive to the idle/question-tool state.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| UUID generation for session IDs | Custom UUID generator or `crypto/rand` hex | `exec.Command("uuidgen").Output()` on host | Consistent with platform behavior; already used elsewhere in codebase |
| Docker env var injection | Manual `-e KEY=VALUE` string building | `collectDockerEnvVars` + `ExecPrefixWithEnv` in `buildExecCommand` | Already handles env forwarding correctly |
| tmux environment store | New subprocess call | `Session.SetEnvironment(key, value)` | Already handles `-t` flag, env-cache invalidation, and error reporting |

**Key insight:** `Session.SetEnvironment()` exists precisely for this use case. The original approach of embedding `tmux set-environment` inside command strings was written before sandbox wrapping existed. Now that `wrapForSandbox()` / `prepareCommand()` exist, the call should live outside the container command.

## Common Pitfalls

### Pitfall 1: Resume Paths Are Duplicated Across `Start()` and `Restart()`

**What goes wrong:** Fixing `buildClaudeCommand` but missing the `Restart()` respawn paths (lines 3604, 3748) leaves sandbox sessions broken after the first restart.
**Why it happens:** Session ID propagation is duplicated across multiple code paths in `instance.go`.
**How to avoid:** Grep for all `tmux set-environment` occurrences in `instance.go` and apply the fix to every one. There are at least 7 confirmed call sites.
**Warning signs:** Session tracking works on first start but breaks after restart in sandbox.

### Pitfall 2: Claude New-Session UUID Sequencing

**What goes wrong:** If UUID generation stays in the shell string (`$(uuidgen ...)`), there is nothing to pass to `SetEnvironment` on the Go side. The fix requires pre-generating in Go.
**Why it happens:** The current shell-expansion approach avoids threading the ID through the Go call chain, but it breaks in sandbox.
**How to avoid:** Pre-generate `sessionID` in Go before calling `buildClaudeCommand`. Pass the literal UUID into the command string and call `SetEnvironment` on the host after `Start()` returns.
**Warning signs:** `CLAUDE_SESSION_ID` is empty after sandbox session start.

### Pitfall 3: OpenCode Async Detection Depends on tmux Env

**What goes wrong:** `detectOpenCodeSessionAsync` reads from tmux env via `GetEnvironment("OPENCODE_SESSION_ID")`. If the shell string previously set this env var (even though it failed in sandbox), removing it without adding a Go-side `SetEnvironment` breaks the async detection.
**Why it happens:** The async detection relies on tmux env as a rendezvous point.
**How to avoid:** After `Start()` / `Restart()` succeeds, call `i.tmuxSession.SetEnvironment("OPENCODE_SESSION_ID", i.OpenCodeSessionID)` before returning. The async detection path will then find the value correctly.

### Pitfall 4: OpenCode Question-Tool Patterns Causing False Positives

**What goes wrong:** "enter submit" or "esc dismiss" might appear in non-question-tool context (e.g., some other TUI element).
**Why it happens:** OpenCode renders multiple overlapping panels; text can collide.
**How to avoid:** The `hasOpencodeBusyIndicator` guard takes priority. Test the new patterns against realistic pane captures of both busy and idle states. The existing `TestOpencodeBusyGuard` test suite in `status_fixes_test.go` must still pass with no regressions.
**Warning signs:** `TestOpencodeBusyGuard` fails on "busy" cases after adding new patterns.

## Code Examples

### `Session.SetEnvironment` — already exists, no changes:

```go
// Source: internal/tmux/tmux.go:904-917
func (s *Session) SetEnvironment(key, value string) error {
    cmd := exec.Command("tmux", "set-environment", "-t", s.Name, key, value)
    err := cmd.Run()
    if err == nil {
        s.envCacheMu.Lock()
        if s.envCache != nil {
            delete(s.envCache, key)
        }
        s.envCacheMu.Unlock()
    }
    return err
}
```

### Existing host-side SetEnvironment pattern (reference model for DET-01):

```go
// Source: internal/session/instance.go ~line 1806 (in Start(), after tmuxSession.Start())
// Set AGENTDECK_INSTANCE_ID for Claude hooks to identify this session
if err := i.tmuxSession.SetEnvironment("AGENTDECK_INSTANCE_ID", i.ID); err != nil {
    sessionLog.Warn("set_instance_id_failed", slog.String("error", err.Error()))
}
```

### Current `buildClaudeResumeCommand` (with workaround comment):

```go
// Source: internal/session/instance.go:3884-3892
// Use ";" (not "&&") so the tool command runs even if tmux set-environment
// fails — inside a Docker sandbox there is no tmux server.
if useResume {
    return fmt.Sprintf("tmux set-environment CLAUDE_SESSION_ID %s 2>/dev/null; %s%s --resume %s%s",
        i.ClaudeSessionID, configDirPrefix, claudeCmd, i.ClaudeSessionID, dangerousFlag)
}
```

After DET-01 fix, this becomes:

```go
if useResume {
    return fmt.Sprintf("%s%s --resume %s%s", configDirPrefix, claudeCmd, i.ClaudeSessionID, dangerousFlag)
}
// Caller (Start/Restart) handles: _ = i.tmuxSession.SetEnvironment("CLAUDE_SESSION_ID", i.ClaudeSessionID)
```

### Current OpenCode prompt patterns (before fix):

```go
// Source: internal/tmux/patterns.go:56-68
case "opencode":
    return &RawPatterns{
        BusyPatterns: []string{
            "esc interrupt",
            "esc to exit",
            "thinking...",
            "generating...",
            "building tool call...",
            "waiting for tool response...",
        },
        PromptPatterns: []string{"Ask anything", "press enter to send"},
        SpinnerChars:   []string{"█", "▓", "▒", "░"},
    }
```

After DET-02 fix:

```go
PromptPatterns: []string{
    "Ask anything",
    "press enter to send",
    "enter submit",   // question tool help bar
    "esc dismiss",    // question tool help bar
},
```

### Existing OpenCode HasPrompt (before fix):

```go
// Source: internal/tmux/detector.go:39-58
case "opencode":
    if d.hasOpencodeBusyIndicator(content) {
        return false
    }
    return strings.Contains(content, "press enter to send") ||
        strings.Contains(content, "Ask anything") ||
        strings.Contains(content, "open code") ||
        d.hasLineEndingWith(content, ">")
```

After DET-02 fix:

```go
case "opencode":
    if d.hasOpencodeBusyIndicator(content) {
        return false
    }
    return strings.Contains(content, "press enter to send") ||
        strings.Contains(content, "Ask anything") ||
        strings.Contains(content, "open code") ||
        strings.Contains(content, "enter submit") ||
        strings.Contains(content, "esc dismiss") ||
        d.hasLineEndingWith(content, ">")
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Sandbox config not persisted | Persisted in SQLite (STORE-01 to STORE-03) | 2026-03-12 (#320) | DET-01 now unblocked |
| `tmux set-environment` in container command string | Should be host-side `SetEnvironment` call | This phase (DET-01) | Eliminates silent env-var loss for all sandbox sessions |
| OpenCode detection only covers normal idle state | Should also cover question-tool selection UI | This phase (DET-02) | Sessions transition to "waiting" (orange) when agent asks a question |

**Deprecated/outdated:**
- `2>/dev/null;` suppressor in `buildClaudeResumeCommand` (line 3887): workaround for the socket-absent problem. Remove after DET-01 fix.

## Open Questions

1. **Exact question-tool prompt strings in current OpenCode versions**
   - What we know: Issue #255 confirms `"↑↓ select"`, `"enter submit"`, `"esc dismiss"` in the help bar. The sst/opencode TypeScript rewrite has `question.ts`. The opencode-ai/opencode original (Go, archived Sept 2025) used Bubble Tea components.
   - What is unclear: Whether these strings match exactly in current released versions (agent-deck users may be on either the original or the rewrite).
   - Recommendation: Add `"enter submit"` and `"esc dismiss"` as starting point. After ship, add a regression test using a real pane capture from a live question-tool interaction.

2. **UUID pre-generation in sandboxed Claude new sessions**
   - What we know: `uuidgen` exists on macOS host. The container may not have `uuidgen`.
   - What is unclear: Whether any edge case in session ID format handling breaks if the UUID comes from Go vs. from `uuidgen | tr '[:upper:]' '[:lower:]'`.
   - Recommendation: Use `exec.Command("uuidgen").Output()` with `strings.ToLower(strings.TrimSpace(...))` to exactly replicate the current behavior.

3. **Scope of fix: sandbox-only vs. all sessions**
   - What we know: The bug only manifests in sandbox sessions. Non-sandbox sessions have the host tmux server reachable.
   - What is unclear: Whether applying the fix uniformly (remove `tmux set-environment` from all command strings) might subtly change non-sandbox behavior.
   - Recommendation: Apply universally. `Session.SetEnvironment()` on the host is equivalent to `tmux set-environment` in the shell for non-sandbox sessions. Removes divergent code paths.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify/assert, `go test -race` |
| Config file | none (`TestMain` enforces `AGENTDECK_PROFILE=_test`) |
| Quick run command | `go test -race -v ./internal/session/... ./internal/tmux/... -run 'TestOpencode\|TestBuildClaude\|TestBuildOpenCode\|TestBuildGemini\|TestBuildCodex'` |
| Full suite command | `go test -race -v ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DET-01 | `buildClaudeCommandWithMessage` (new session) command string does NOT contain `tmux set-environment` | unit | `go test ./internal/session/... -run TestBuildClaudeCommand_NoTmuxSetEnv -v` | ❌ Wave 0 |
| DET-01 | `buildOpenCodeCommand` with session ID command string does NOT contain `tmux set-environment` | unit | `go test ./internal/session/... -run TestBuildOpenCodeCommand_NoTmuxSetEnv -v` | ❌ Wave 0 |
| DET-01 | `buildGeminiCommand` with session ID command string does NOT contain `tmux set-environment` | unit | `go test ./internal/session/... -run TestBuildGeminiCommand_NoTmuxSetEnv -v` | ❌ Wave 0 |
| DET-01 | `buildCodexCommand` with session ID command string does NOT contain `tmux set-environment` | unit | `go test ./internal/session/... -run TestBuildCodexCommand_NoTmuxSetEnv -v` | ❌ Wave 0 |
| DET-01 | `buildClaudeResumeCommand` does NOT contain `tmux set-environment` or `2>/dev/null` | unit | `go test ./internal/session/... -run TestBuildClaudeResumeCommand_NoTmuxSetEnv -v` | ❌ Wave 0 |
| DET-02 | `HasPrompt("opencode")` returns true for question-tool content with `"enter submit"` | unit | `go test ./internal/tmux/... -run TestOpencodeBusyGuard -v` | ✅ (extend existing test) |
| DET-02 | `HasPrompt("opencode")` returns true for content with `"esc dismiss"` | unit | `go test ./internal/tmux/... -run TestOpencodeBusyGuard -v` | ✅ (extend existing test) |
| DET-02 | `HasPrompt("opencode")` returns false for busy content that also contains `"enter submit"` | unit | `go test ./internal/tmux/... -run TestOpencodeBusyGuard -v` | ✅ (extend existing test) |
| DET-02 | `DefaultRawPatterns("opencode").PromptPatterns` includes question-tool strings | unit | `go test ./internal/tmux/... -run TestDefaultRawPatterns_OpenCode -v` | ✅ (extend existing test) |
| DET-02 | Integration: `TestDetection_OpenCodeQuestionTool` passes for question-tool pane content | integration | `go test ./internal/integration/... -run TestDetection_OpenCodeQuestionTool -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race ./internal/tmux/... -run TestOpencode` and `go test -race ./internal/session/... -run TestBuild`
- **Per wave merge:** `go test -race ./internal/tmux/... ./internal/session/... ./internal/integration/...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/session/instance_test.go` — add `TestBuildClaudeCommand_NoTmuxSetEnv`, `TestBuildOpenCodeCommand_NoTmuxSetEnv`, `TestBuildGeminiCommand_NoTmuxSetEnv`, `TestBuildCodexCommand_NoTmuxSetEnv`, `TestBuildClaudeResumeCommand_NoTmuxSetEnv`
- [ ] `internal/integration/detection_test.go` — add `TestDetection_OpenCodeQuestionTool`
- [ ] `internal/tmux/status_fixes_test.go` — extend with VALIDATION 8.0 section covering question-tool prompt cases

*(Existing test infrastructure in `testmain_test.go` files already ensures profile isolation — no framework gaps.)*

## Sources

### Primary (HIGH confidence)
- `internal/session/instance.go` (direct source read) — all 7+ `tmux set-environment` call sites; `buildClaudeCommand`, `buildOpenCodeCommand`, `buildGeminiCommand`, `buildCodexCommand`, `buildClaudeResumeCommand`, `prepareCommand`, `wrapForSandbox`, `buildExecCommand`, `SetEnvironment` usage pattern at line 1806
- `internal/tmux/tmux.go` (direct source read) — `SetEnvironment(key, value)` implementation at line 904
- `internal/tmux/detector.go` (direct source read) — `HasPrompt` opencode case, `hasOpencodeBusyIndicator`
- `internal/tmux/patterns.go` (direct source read) — `DefaultRawPatterns("opencode")` at line 56
- `internal/tmux/status_fixes_test.go` (direct source read) — VALIDATION 7.0 opencode busy-guard tests at line 765
- GitHub issue #266 (fetched) — root cause confirmed: tmux socket unreachable from container; UUID pre-generation on host recommended
- GitHub issue #255 (fetched) — confirmed: question tool shows arrow-key selection with `"enter submit"` / `"esc dismiss"` help bar; waiting state not detected

### Secondary (MEDIUM confidence)
- `internal/session/opencode_test.go` (direct source read) — `TestOpenCodeBuildCommand` confirms `tmux set-environment OPENCODE_SESSION_ID` appears in resume command string
- `internal/tmux/tmux_test.go` (direct source read) — existing OpenCode `HasPrompt` test cases at line 224

### Tertiary (LOW confidence)
- sst/opencode repository — `question.ts` confirmed to exist; exact terminal rendering strings not confirmed due to rate limiting. Patterns inferred from issue #255 screenshots and common Bubble Tea list component conventions.

## Metadata

**Confidence breakdown:**
- DET-01 root cause: HIGH — confirmed by reading all affected code paths, `buildExecCommand`, and issue #266
- DET-01 fix architecture: HIGH — host-side `SetEnvironment` pattern already established at line 1806
- DET-02 root cause: HIGH — confirmed by reading `patterns.go`, `detector.go`, and issue #255
- DET-02 fix (patterns): MEDIUM — `"enter submit"` and `"esc dismiss"` from issue report; exact strings not verified against a live terminal capture. Should be treated as a starting point that may need refinement.

**Research date:** 2026-03-13
**Valid until:** 2026-04-13 (OpenCode TUI changes frequently; verify prompt strings against installed version before implementation)
