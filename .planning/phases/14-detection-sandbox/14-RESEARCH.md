# Phase 14: Detection & Sandbox - Research

**Researched:** 2026-03-13
**Domain:** Docker sandbox tmux environment propagation, OpenCode status detection
**Confidence:** HIGH

## Summary

Phase 14 addresses two independent but contained bugs. DET-01 is a structural flaw: commands built by `buildClaudeCommand`, `buildOpenCodeCommand`, `buildCodexCommand`, and `buildGenericCommand` embed `tmux set-environment` calls that run **inside the Docker container** (via `docker exec`), but the tmux server lives on the **host**. The container's `/tmp` is a tmpfs — the host's Unix domain socket at `/tmp/tmux-<uid>/default` is unreachable from inside the container. The fix is to move environment variable stores to the host side before the `docker exec` wrapper, or call `tmux set-environment` from Go immediately after session start (which already partially happens in `Instance.Start()`). The `#320` sandbox config persistence fix is already merged, removing one blocker.

DET-02 is a pattern-coverage gap: OpenCode's `question` tool renders a UI with `↑↓ select`, `enter submit`, `esc dismiss` in the help bar, but none of these strings are in OpenCode's `PromptPatterns`. The existing detector checks `"press enter to send"` and `"Ask anything"` (normal idle state) but misses the question-tool waiting state. Additionally, users report false positives — the tool sometimes shows busy when it is actually idle — suggesting that some busy-indicator patterns fire on static content.

**Primary recommendation:** For DET-01 use host-side `tmux set-environment` via the Go API (already called in `Start()`) instead of embedding `tmux set-environment` inside the shell command string that docker exec runs. For DET-02 add `"enter submit"` and `"esc dismiss"` to OpenCode's `PromptPatterns` in `patterns.go`.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DET-01 | tmux set-environment works correctly inside Docker sandbox sessions now that sandbox config persistence is fixed (#266) | Root cause confirmed: all `tmux set-environment` calls embedded in command strings are executed inside `docker exec` and cannot reach the host tmux socket. Fix: remove these calls from the shell command strings and rely on the Go-side `SetEnvironment` calls that happen after `tmuxSession.Start()` returns. |
| DET-02 | OpenCode waiting status detection triggers correctly when OpenCode presents the question tool prompt (#255) | Root cause confirmed: `PromptPatterns` for opencode miss the question tool help-bar strings (`"enter submit"`, `"esc dismiss"`). False-positive busy also reported; spinner chars `█ ▓ ▒ ░` may appear in static UI elements. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go standard library | 1.24+ | Command execution, env manipulation | Already used throughout |
| `internal/tmux` | local | tmux session and environment abstraction | All tmux operations go through this package |
| `internal/session` | local | Session lifecycle, command building | All tool-specific command construction is here |

### No New Dependencies

Both fixes are pure logic changes within existing packages. No new libraries are required.

## Architecture Patterns

### DET-01: tmux set-environment in Docker Sandbox

#### Root Cause — Confirmed

`buildClaudeCommand` generates a shell string like:

```go
`session_id=$(uuidgen | tr '[:upper:]' '[:lower:]'); `+
    `tmux set-environment CLAUDE_SESSION_ID "$session_id"; `+
    `AGENTDECK_INSTANCE_ID=... claude --session-id "$session_id"`
```

This string becomes the `toolCommand` argument to `buildExecCommand`, which wraps it as:

```
docker exec -it -e TERM=... <container> bash -c '<above string>'
```

The `tmux set-environment` subprocess inside the container cannot connect to `/tmp/tmux-<uid>/default` on the host. The `;` separator ensures Claude still launches, but `CLAUDE_SESSION_ID` is never stored — so `GetSessionIDFromTmux()` finds nothing.

**Affected command builders (all have the same pattern):**

| Function | Env var set in shell string | File |
|---|---|---|
| `buildClaudeCommandWithMessage` | `CLAUDE_SESSION_ID` | instance.go:491 |
| `buildClaudeResumeCommand` | `CLAUDE_SESSION_ID` | instance.go:3887 |
| `buildOpenCodeCommand` | `OPENCODE_SESSION_ID` | instance.go:655 |
| `Restart()` — opencode respawn path | `OPENCODE_SESSION_ID` | instance.go:3604 |
| `Restart()` — fallback opencode | `OPENCODE_SESSION_ID` | instance.go:3748 |
| `Restart()` — generic tool | tool-specific env var | instance.go:3693 |
| `buildGenericCommand` | tool-specific env var | instance.go:1652 |

#### Why the Current `Start()` SetEnvironment Calls Do NOT Fully Solve It

After `tmuxSession.Start()` returns, `Instance.Start()` already calls:

```go
i.tmuxSession.SetEnvironment("AGENTDECK_INSTANCE_ID", i.ID)
i.tmuxSession.SetEnvironment("COLORFGBG", colorfgbg)
```

These calls run on the **host** and work correctly. However, `CLAUDE_SESSION_ID` / `OPENCODE_SESSION_ID` etc. are generated **inside** the shell string, so the Go side never knows the value to store. The fix must either:

1. **Generate session IDs on the host** (before `docker exec`) and pass them in via `-e`, then call `tmux set-environment` from Go after `Start()` returns.
2. **Strip the `tmux set-environment` calls from all sandbox command strings** and replace them with host-side Go calls.

Option 1 is the recommended approach from issue #266 and is architecturally cleaner.

#### Recommended Fix Pattern

For Claude new sessions (the most common case):

```go
// BEFORE (current): generates UUID inside container, calls tmux set-environment inside container
`session_id=$(uuidgen | tr '[:upper:]' '[:lower:]'); `+
    `tmux set-environment CLAUDE_SESSION_ID "$session_id"; `+
    `claude --session-id "$session_id"`

// AFTER (fix): UUID generated in Go, passed via -e to docker exec, tmux set-environment called from Go
// Step 1 (in buildClaudeCommandWithMessage when IsSandboxed):
sessionID := newUUID()
i.pendingSessionID = sessionID   // stored on Instance before Start() is called
// or: return command without tmux set-environment, with --session-id hardcoded

// Step 2 (in buildExecCommand or via -e flag):
docker exec -e CLAUDE_SESSION_ID=<id> ... bash -c 'claude --session-id "$CLAUDE_SESSION_ID"'

// Step 3 (in Start() after tmuxSession.Start() returns):
i.tmuxSession.SetEnvironment("CLAUDE_SESSION_ID", sessionID)
i.ClaudeSessionID = sessionID
```

The simpler approach (option 2, avoid embedding in docker): for sandbox sessions, strip `tmux set-environment ...` from the shell string and instead generate the ID on the host, pass it via an inline env var, and call `SetEnvironment` from Go. This requires `buildClaudeCommandWithMessage` to be sandbox-aware (or use a pre-generated ID argument).

For resume commands (`buildClaudeResumeCommand`), the session ID is already known on the host (`i.ClaudeSessionID`), so simply removing the `tmux set-environment` from the shell string and keeping the Go-side `SetEnvironment` is sufficient.

**Key code path to follow:**

```
Instance.Start()
  → buildClaudeCommand()           // builds shell string with embedded tmux set-env
  → prepareCommand()               // applies wrapper, SSH, sandbox wrapping
      → wrapForSandbox()           // wraps in docker exec
  → tmuxSession.Start(command)     // runs docker exec
  → tmuxSession.SetEnvironment()   // host-side: correct, but missing CLAUDE_SESSION_ID
```

### DET-02: OpenCode Question Tool Detection

#### Root Cause — Confirmed

When OpenCode's `question` tool is active, the terminal shows a selection UI with this help bar:

```
↑↓ select     enter submit     esc dismiss
```

The current OpenCode `PromptPatterns` in `DefaultRawPatterns("opencode")` are:

```go
PromptPatterns: []string{"Ask anything", "press enter to send"},
```

The `HasPrompt` method for opencode also checks:
- `"open code"` (inline in `detector.go:57`)
- Lines ending with `>`

None of these match the question tool UI.

The `PromptDetector.HasPrompt` for opencode also checks `hasOpencodeBusyIndicator` first. If that returns true, it returns `false` regardless of prompt patterns. The busy indicator checks for pulse spinner chars `█ ▓ ▒ ░` — these are part of OpenCode's Bubble Tea UI and may appear in static UI elements (not just active processing), causing false-positive busy detection.

#### Recommended Fix Pattern

Add the question tool help-bar strings to `DefaultRawPatterns("opencode")`:

```go
// Source: internal/tmux/patterns.go DefaultRawPatterns("opencode")
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
        PromptPatterns: []string{
            "Ask anything",
            "press enter to send",
            "enter submit",     // ADD: question tool help bar
            "esc dismiss",      // ADD: question tool help bar
        },
        SpinnerChars: []string{"█", "▓", "▒", "░"},
    }
```

The two strings `"enter submit"` and `"esc dismiss"` are unique to the question-tool waiting state and will not appear during busy processing. The existing `hasOpencodeBusyIndicator` check in `HasPrompt` ensures that if the pulse spinner is running, busy takes priority over this new prompt detection.

For the permission approval case (second screenshot in issue #255), the help bar shows similar navigation patterns. Investigate whether `"↑↓ select"` and `"enter submit"` also appear on the permission dialog and add them together.

#### False Positive Investigation

The false positive case (issue reports session appears busy when idle) may be caused by:
1. The `░` or `▓` characters appearing in OpenCode's static UI elements (e.g., progress bars, decorative elements).
2. The `"Waiting for tool response..."` busy string matching stale terminal content after the tool completes.

The false-positive fix approach: check that pulse spinner chars only match when accompanied by the spinner being recently active (already partially implemented via `SpinnerActivityTracker`), or add a grace period check similar to how Claude's spinner tracking works.

### Recommended Project Structure (unchanged)

```
internal/session/
  instance.go           # DET-01: modify buildClaudeCommandWithMessage, buildOpenCodeCommand,
                        #   buildClaudeResumeCommand (strip tmux set-env from sandbox commands)
internal/tmux/
  patterns.go           # DET-02: add question-tool prompt patterns to opencode case
  detector.go           # DET-02: potentially adjust hasOpencodeBusyIndicator for false positives
```

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| UUID generation for session IDs | Custom UUID generator | `uuidgen` on host via `exec.Command` or `crypto/rand` hex string | Already used in `generateID()` and `randomString()` |
| Docker env var injection | Manual string building | `docker exec -e KEY=VALUE` (already done in `collectDockerEnvVars`) | `buildExecCommand` already handles `-e` flags via `collectDockerEnvVars` |
| tmux env store | Subprocess call inside container | `Session.SetEnvironment()` Go API from host | Works correctly on host, confirmed by existing tests |

## Common Pitfalls

### Pitfall 1: Generating Session IDs Before Sandbox Wrapping
**What goes wrong:** If session ID is generated in `buildClaudeCommand` but the sandbox wrapping happens in `prepareCommand`, there is a sequencing problem — the ID needs to be known before `prepareCommand` but `buildClaudeCommand` is called before `prepareCommand`.
**Why it happens:** The current architecture generates the ID inside the shell string, which avoids this timing issue.
**How to avoid:** Pre-generate the ID in `Instance.Start()` before calling `buildClaudeCommand`, store it on the instance, then have `buildClaudeCommandWithMessage` use `i.pendingClaudeSessionID` when non-empty. After `tmuxSession.Start()` returns, call `SetEnvironment`.
**Alternative:** For sandbox sessions only, replace `$(uuidgen ...)` with a hardcoded ID by detecting `i.IsSandboxed()` in `buildClaudeCommandWithMessage`.

### Pitfall 2: Non-Sandbox Sessions Must Not Change
**What goes wrong:** Modifying the `tmux set-environment` call for all sessions (not just sandbox) would break session tracking for normal sessions, where the command runs in the host's tmux context and `tmux set-environment` works fine.
**Why it happens:** Overly broad refactoring.
**How to avoid:** The fix must be conditional on `i.IsSandboxed()`. Non-sandbox sessions keep the existing `tmux set-environment` in-shell behavior.

### Pitfall 3: OpenCode False Positive Busy State
**What goes wrong:** Adding `"enter submit"` to PromptPatterns while keeping `░` in SpinnerChars means the busy check still runs first. If `░` appears in a static progress bar, the busy check fires and the prompt check is skipped.
**Why it happens:** OpenCode renders block characters as UI decoration, not only as spinners.
**How to avoid:** If false positives persist after adding prompt patterns, narrow the busy check to require BOTH a spinner char AND a busy text string (e.g., only fire when `░` appears on the SAME line as a busy task string). This is a secondary concern; adding prompt patterns may be sufficient.

### Pitfall 4: Resume Paths in Restart()
**What goes wrong:** `Restart()` has multiple code paths that call `tmux set-environment` inside shell strings (the respawn-pane paths at lines 3604 and 3748). If only `buildClaudeCommand` is fixed but not the `Restart()` paths, the bug persists after the first restart.
**Why it happens:** Session ID propagation is duplicated across multiple code paths.
**How to avoid:** Audit all `tmux set-environment` occurrences in `instance.go` and apply the same fix. There are at least 7 call sites identified above.

### Pitfall 5: Sandbox OpenCode/Codex Session Detection
**What goes wrong:** For OpenCode and Codex, the session ID is detected asynchronously after startup (`detectOpenCodeSessionAsync`, `detectCodexSessionAsync`). The async detection uses `tmux GetEnvironment` to retrieve the ID set by the shell string. If the shell string no longer calls `tmux set-environment`, the async detection will find nothing.
**Why it happens:** The async detection reads from tmux env, which previously was set inside the container (failing) but with the fix becomes set from Go after `Start()`.
**How to avoid:** After generating the session ID on the host and calling `SetEnvironment("OPENCODE_SESSION_ID", id)` from Go, the async detection via `GetEnvironment("OPENCODE_SESSION_ID")` will work correctly.

## Code Examples

### Existing SetEnvironment Pattern (HOST-SIDE, correct)
```go
// Source: internal/session/instance.go lines 1806-1818
// Set AGENTDECK_INSTANCE_ID for Claude hooks to identify this session
if err := i.tmuxSession.SetEnvironment("AGENTDECK_INSTANCE_ID", i.ID); err != nil {
    sessionLog.Warn("set_instance_id_failed", slog.String("error", err.Error()))
}
// Propagate COLORFGBG into the tmux session environment
if colorfgbg := ThemeColorFGBG(); colorfgbg != "" {
    _ = i.tmuxSession.SetEnvironment("COLORFGBG", colorfgbg)
}
```

### Existing docker exec -e Pattern (for passing env vars to container)
```go
// Source: internal/session/instance.go buildExecCommand()
func buildExecCommand(ctr *docker.Container, userCfg *UserConfig, toolCommand string) string {
    var userNames []string
    if userCfg != nil {
        userNames = userCfg.Docker.Environment
    }
    runtimeEnv := collectDockerEnvVars(userNames)
    var prefix []string
    if len(runtimeEnv) > 0 {
        prefix = ctr.ExecPrefixWithEnv(runtimeEnv)
    } else {
        prefix = ctr.ExecPrefix()
    }
    return docker.ShellJoinArgs(append(prefix, "bash", "-c", toolCommand))
}
```

### Existing buildClaudeResumeCommand Comment (already handles sandbox awareness)
```go
// Source: internal/session/instance.go lines 3884-3892
// Use ";" (not "&&") so the tool command runs even if tmux set-environment
// fails — inside a Docker sandbox there is no tmux server.
if useResume {
    return fmt.Sprintf("tmux set-environment CLAUDE_SESSION_ID %s 2>/dev/null; %s%s --resume %s%s",
        i.ClaudeSessionID, configDirPrefix, claudeCmd, i.ClaudeSessionID, dangerousFlag)
}
```

This comment shows the existing code is already aware of the problem but only silences the error. The fix should eliminate the call entirely for sandbox sessions.

### OpenCode Prompt Patterns to Add
```go
// Source: internal/tmux/patterns.go DefaultRawPatterns()
// Question tool help bar: "↑↓ select   enter submit   esc dismiss"
PromptPatterns: []string{
    "Ask anything",
    "press enter to send",
    "enter submit",   // question tool navigation help
    "esc dismiss",    // question tool cancel affordance
},
```

### Test for tmux set-environment (existing, reference)
```go
// Source: internal/tmux/tmux_test.go lines 2247-2262
err = sess.SetEnvironment("TEST_VAR", "test_value_123")
// ... then:
value, err := sess.GetEnvironment("TEST_VAR")
// value == "test_value_123"
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Sandbox config not persisted | Sandbox config persisted (PR #320) | 2026-03-12 | DET-01 is now unblocked; sandbox sessions can be restored with correct config |
| `tmux set-environment` embedded in shell string (works for host sessions) | Same — has never worked inside Docker | Still broken | Root cause of DET-01 |

**Deprecated/outdated:**
- Using `$(uuidgen)` inside `docker exec` shell strings: works on host but not in sandbox; must be replaced with host-side generation for sandbox sessions.

## Open Questions

1. **Which of the two fix strategies for DET-01 is simpler?**
   - What we know: Strategy A (pre-generate ID on host, pass via `-e`) requires threading the ID through `buildClaudeCommandWithMessage` → `buildExecCommand`, and calling `SetEnvironment` from Go. Strategy B (strip `tmux set-environment` from shell strings and rely entirely on Go-side `SetEnvironment`) requires knowing the session ID before the shell string is built.
   - What's unclear: For Claude new sessions, the UUID is currently generated inside the shell string with `$(uuidgen ...)`. Moving this to Go side requires generating a UUID in Go code and threading it into the command builder. The simplest path: add a `preGeneratedSessionID` field to `Instance` (or take it as a function parameter), set it before calling `buildClaudeCommand`, pass it into the shell string as a literal (not generated at runtime), and call `SetEnvironment` from Go.
   - Recommendation: Generate UUID in Go, pass into command builders as a literal, call `SetEnvironment` from host. This is the cleanest separation.

2. **Does the OpenCode question tool also use `"esc interrupt"` in the help bar?**
   - What we know: From the issue screenshot, the help bar shows `↑↓ select`, `enter submit`, `esc dismiss`. The `hasOpencodeBusyIndicator` checks for `"esc interrupt"` and `"esc to exit"`, which should NOT match `"esc dismiss"`.
   - What's unclear: Whether permission approval dialogs also use `"enter submit"` or different text.
   - Recommendation: Add both strings to `PromptPatterns`. Validate against the screenshots in the issue.

3. **Are there other call sites for `tmux set-environment` inside docker exec that are missing from the audit?**
   - What we know: 7 call sites identified. Grep confirms these are all in `instance.go`.
   - Recommendation: After applying the fix, run `grep -n "tmux set-environment" internal/session/instance.go` and verify all remaining occurrences are either host-side Go calls or are guarded by `!i.IsSandboxed()`.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + `go test -race` |
| Config file | none (uses TestMain profile isolation) |
| Quick run command | `go test -race -v ./internal/session/... ./internal/tmux/...` |
| Full suite command | `go test -race -v ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DET-01 | tmux set-environment called from host (not inside container) for sandbox sessions | unit | `go test -race -v -run TestSandbox ./internal/session/...` | ❌ Wave 0 |
| DET-01 | Session ID is accessible via GetEnvironment after Start() for sandbox sessions | unit | `go test -race -v -run TestSandbox ./internal/session/...` | ❌ Wave 0 |
| DET-02 | OpenCode question tool prompt detected as waiting status | unit | `go test -race -v -run TestOpenCode ./internal/tmux/...` | ❌ Wave 0 |
| DET-02 | OpenCode permission approval prompt detected as waiting status | unit | `go test -race -v -run TestOpenCode ./internal/tmux/...` | ❌ Wave 0 |
| DET-02 | OpenCode busy state not false-positived on static UI | unit | `go test -race -v -run TestOpenCode ./internal/tmux/...` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race -v ./internal/session/... ./internal/tmux/...`
- **Per wave merge:** `go test -race -v ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/session/sandbox_env_test.go` — tests that `buildClaudeCommandWithMessage` does NOT contain `tmux set-environment` for sandbox sessions, and that `SetEnvironment` is called from Go side (REQ: DET-01)
- [ ] `internal/tmux/opencode_detection_test.go` — tests that `HasPrompt("opencode", ...)` returns true for question-tool help bar content (REQ: DET-02)
- [ ] `internal/session/testmain_test.go` — already exists, ensures `AGENTDECK_PROFILE=_test`

## Sources

### Primary (HIGH confidence)
- Direct code reading: `internal/session/instance.go` — all `buildClaudeCommand*`, `buildOpenCodeCommand`, `Restart()`, `wrapForSandbox`, `buildExecCommand` functions
- Direct code reading: `internal/tmux/patterns.go` — `DefaultRawPatterns("opencode")` and existing PromptPatterns
- Direct code reading: `internal/tmux/detector.go` — `hasOpencodeBusyIndicator`, `HasPrompt` for opencode
- GitHub issue #266 — detailed root cause and suggested fix from reporter
- GitHub issue #255 — screenshots showing question tool and permission approval waiting states

### Secondary (MEDIUM confidence)
- GitHub issue #255 comments — additional cases (permission approval, false positive busy)
- Existing test `TestOpenCodeBuildCommand` in `internal/session/opencode_test.go` — confirms `tmux set-environment OPENCODE_SESSION_ID` is embedded in the command string for resume

### Tertiary (LOW confidence)
- OpenCode source code not directly inspected — question tool UI strings inferred from issue screenshots

## Metadata

**Confidence breakdown:**
- Root cause DET-01: HIGH — confirmed by reading all relevant code paths and issue report
- Root cause DET-02: HIGH — confirmed by reading patterns.go, detector.go, and issue screenshots
- Fix approach DET-01: MEDIUM — strategy is clear but precise implementation requires choosing between two valid approaches (pre-generate vs strip-and-store)
- Fix approach DET-02: HIGH — simple string addition to existing patterns array

**Research date:** 2026-03-13
**Valid until:** 2026-04-13 (stable domain — OpenCode TUI changes may affect prompt strings)
