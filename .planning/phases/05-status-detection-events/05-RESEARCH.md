# Phase 5: Status Detection & Events - Research

**Researched:** 2026-03-06
**Domain:** Multi-tool status detection patterns, conductor cross-session event propagation
**Confidence:** HIGH

## Summary

Phase 5 tests two interconnected subsystems: (1) the status detection engine that identifies sleep/wait/busy patterns for Claude, Gemini, OpenCode, and Codex through simulated terminal output, and (2) the conductor's cross-session event notification pipeline where a parent session sends commands to child sessions and receives status transition events back.

The codebase has a well-structured, layered detection system. At the lowest level, `internal/tmux/patterns.go` defines `RawPatterns` (busy strings, prompt strings, spinner chars, whimsical words) per tool and compiles them to `ResolvedPatterns` with regex support. The `PromptDetector` in `internal/tmux/detector.go` provides tool-specific `HasPrompt()` checks with busy-indicator priority (busy always trumps prompt). At the `Session.GetStatus()` level (tmux.go:1895), these are combined with activity timestamp tracking, spinner grace periods, and acknowledgment state to produce final statuses: "active", "waiting", "idle", "starting", or "inactive". The `Instance.UpdateStatus()` (instance.go:2251) maps these to `StatusRunning`, `StatusWaiting`, `StatusIdle`, `StatusStarting`, or `StatusError`, with an additional hook-based fast path for Claude and Codex.

For conductor testing, the event pipeline flows: child status changes -> `WriteStatusEvent()` writes JSON atomically to `~/.agent-deck/events/` -> `StatusEventWatcher` (fsnotify) detects the file -> event delivered via channel. The `TransitionDaemon` and `TransitionNotifier` handle the parent notification side, using `SendSessionMessageReliable()` which invokes the agent-deck CLI's `session send` command.

**Primary recommendation:** Test status detection at two levels: (a) unit-style tests using `PromptDetector.HasPrompt()` and `Session.hasBusyIndicator()`/`hasPromptIndicator()` directly with simulated pane content strings for all four tools (DETECT-01), (b) integration tests that start real tmux sessions with echo commands that produce prompt-like or busy-like output and verify `GetStatus()` returns expected values (DETECT-03). Conductor tests should use real tmux `SendKeysAndEnter()` for COND-01 and file-based event writing + fsnotify watching for COND-02.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| DETECT-01 | Sleep/wait detection correctly identifies patterns for Claude, Gemini, OpenCode, and Codex via simulated output | `PromptDetector.HasPrompt()` and `Session.hasBusyIndicator()` can be tested with synthetic content strings. `DefaultRawPatterns()` provides canonical patterns per tool. |
| DETECT-02 | Multi-tool session creation produces correct commands and detection config per tool type | `NewInstanceWithTool()` + `Start()` builds tool-specific commands via `buildClaudeCommand()`, `buildGeminiCommand()`, `buildOpenCodeCommand()`, `buildCodexCommand()`. `loadCustomPatternsFromConfig()` loads detection config. |
| DETECT-03 | Status transition cycle (starting -> running -> waiting -> idle) verified with real tmux pane content | `Instance.UpdateStatus()` maps `Session.GetStatus()` results. Real tmux sessions with `echo` commands can produce content that triggers prompt detection, driving status transitions. |
| COND-01 | Conductor sends command to child session via real tmux and child receives it | `Session.SendKeysAndEnter()` sends text + Enter to tmux. `WaitForPaneContent()` verifies receipt. Can also test `SendSessionMessageReliable()` which wraps the CLI. |
| COND-02 | Cross-session event notification cycle works (event written, watcher detects, parent notified) | `WriteStatusEvent()` writes JSON to events dir. `StatusEventWatcher` (fsnotify) detects and delivers via channel. Test by writing an event and asserting the watcher delivers it. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `testing` | 1.24 | Test framework | Built-in, no dependency |
| testify | v1.11.1 | Assertions (require/assert) | Already used in 50+ test files |
| fsnotify | v1.9.0 | File system event watching | Already used by `StatusEventWatcher` and `StorageWatcher` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/integration` | project | TmuxHarness, WaitForCondition, fixtures | All integration tests in this phase |
| `internal/tmux` | project | PromptDetector, patterns, Session, StripANSI | DETECT-01, DETECT-03 status detection tests |
| `internal/session` | project | Instance, StatusEvent, EventWatcher | All requirements |
| `internal/statedb` | project | StateDB for fixture creation | COND-02 event tests needing DB state |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Direct `PromptDetector.HasPrompt()` tests | Full `Session.GetStatus()` end-to-end | GetStatus has many layers (timestamps, acknowledgment, spike detection); unit testing HasPrompt is simpler and more targeted for DETECT-01 |
| Real AI tool binaries | Simulated pane output via echo | Real tools require API keys, cost money, are flaky; out of scope per REQUIREMENTS.md |
| Custom event bus | File-based events via fsnotify | File-based is the existing architecture; no reason to change |

**Installation:**
No new dependencies needed. All libraries are already in `go.mod`.

## Architecture Patterns

### Recommended Test Structure
```
internal/integration/
    harness.go              # [exists] TmuxHarness
    poll.go                 # [exists] WaitForCondition, WaitForPaneContent, WaitForStatus
    fixtures.go             # [exists] NewTestDB, InstanceBuilder
    testmain_test.go        # [exists] TestMain + orphan cleanup
    lifecycle_test.go       # [exists] LIFE-01..04 tests
    detection_test.go       # [NEW] DETECT-01, DETECT-02, DETECT-03
    conductor_test.go       # [NEW] COND-01, COND-02
```

### Pattern 1: Simulated Pane Content Detection (DETECT-01)
**What:** Create test content strings that simulate real tool terminal output, then assert detection functions identify the correct state.
**When to use:** Testing that tool-specific patterns (busy indicators, prompt strings, spinner chars) are correctly identified without needing real tool binaries.
**Example:**
```go
// Source: Derived from existing patterns_test.go and status_fixes_test.go
func TestDetection_ClaudeBusy(t *testing.T) {
    detector := tmux.NewPromptDetector("claude")

    busyContent := "Working on your request...\n✢ Clauding… (25s · ↓ 749 tokens)\n"
    assert.False(t, detector.HasPrompt(busyContent),
        "should NOT detect prompt when Claude shows busy spinner")

    waitingContent := "Task completed.\n❯ \n"
    assert.True(t, detector.HasPrompt(waitingContent),
        "should detect prompt when Claude shows input prompt")
}
```

### Pattern 2: Tool Launch Command Verification (DETECT-02)
**What:** Create `Instance` with each tool type, inspect the built command to verify tool-specific flags and detection config.
**When to use:** Verifying that `NewInstanceWithTool()` + tool-specific build functions produce correct launch commands.
**Example:**
```go
func TestDetection_ToolCommands(t *testing.T) {
    inst := session.NewInstanceWithTool("test", "/tmp", "claude")
    // Verify tool is set correctly
    assert.Equal(t, "claude", inst.Tool)

    instCodex := session.NewInstanceWithTool("test", "/tmp", "codex")
    assert.Equal(t, "codex", instCodex.Tool)

    // Verify detection patterns exist for each tool
    for _, tool := range []string{"claude", "gemini", "opencode", "codex"} {
        raw := tmux.DefaultRawPatterns(tool)
        assert.NotNil(t, raw, "tool %s should have default patterns", tool)
    }
}
```

### Pattern 3: Real Tmux Status Transitions (DETECT-03)
**What:** Start a real tmux session with commands that simulate tool states, then call `UpdateStatus()` and verify the status transitions.
**When to use:** End-to-end verification that the full status detection pipeline works through real tmux.
**Key insight:** Shell sessions (tool="shell") don't have busy/prompt patterns. Instead, use `echo` commands with sleep to create a running process, then let it exit to a shell prompt for "waiting"/"idle" transitions.
**Example:**
```go
func TestDetection_StatusCycle(t *testing.T) {
    h := NewTmuxHarness(t)
    inst := h.CreateSession("status-cycle", "/tmp")
    inst.Command = "echo 'processing...' && sleep 2 && echo 'done'"
    require.NoError(t, inst.Start())

    // Should start as StatusStarting
    assert.Equal(t, session.StatusStarting, inst.Status)

    // Wait for tmux to be up
    WaitForCondition(t, 5*time.Second, 200*time.Millisecond,
        "session exists", func() bool { return inst.Exists() })

    // After sleep completes, command exits, shell prompt appears
    WaitForPaneContent(t, inst, "done", 10*time.Second)

    // UpdateStatus should reflect the shell prompt
    time.Sleep(500*time.Millisecond) // Let tmux activity timestamp settle
    require.NoError(t, inst.UpdateStatus())
    // Shell tool shows idle when at prompt
}
```

### Pattern 4: Conductor Send via Real Tmux (COND-01)
**What:** Create parent and child tmux sessions, send a command from parent to child using `SendKeysAndEnter()`, verify the child received it.
**When to use:** Testing that the conductor can deliver instructions to child sessions.
**Example:**
```go
func TestConductor_SendToChild(t *testing.T) {
    h := NewTmuxHarness(t)

    // Child runs cat (waits for stdin, echoes back)
    child := h.CreateSession("cond-child", "/tmp")
    child.Command = "cat"
    require.NoError(t, child.Start())

    WaitForCondition(t, 5*time.Second, 200*time.Millisecond,
        "child exists", func() bool { return child.Exists() })

    // Send message via tmux
    tmuxSess := child.GetTmuxSession()
    require.NotNil(t, tmuxSess)
    require.NoError(t, tmuxSess.SendKeysAndEnter("hello from conductor"))

    // Verify child received the message
    WaitForPaneContent(t, child, "hello from conductor", 5*time.Second)
}
```

### Pattern 5: Event Write-Watch Cycle (COND-02)
**What:** Write a `StatusEvent` to the events directory, verify the `StatusEventWatcher` picks it up and delivers it.
**When to use:** Testing the event notification pipeline.
**Example:**
```go
func TestConductor_EventNotification(t *testing.T) {
    instanceID := "test-" + generateID()

    // Create watcher filtering for our instance
    watcher, err := session.NewStatusEventWatcher(instanceID)
    require.NoError(t, err)
    defer watcher.Stop()
    go watcher.Start()

    // Short delay for fsnotify to register
    time.Sleep(200 * time.Millisecond)

    // Write a status event
    event := session.StatusEvent{
        InstanceID: instanceID,
        Title:      "test-child",
        Tool:       "claude",
        Status:     "waiting",
        PrevStatus: "running",
        Timestamp:  time.Now().Unix(),
    }
    require.NoError(t, session.WriteStatusEvent(event))

    // Watcher should deliver the event
    received, err := watcher.WaitForStatus([]string{"waiting"}, 5*time.Second)
    require.NoError(t, err)
    assert.Equal(t, instanceID, received.InstanceID)
    assert.Equal(t, "waiting", received.Status)
}
```

### Anti-Patterns to Avoid
- **Testing with real AI tool binaries:** Requires API keys, costs money, is flaky. Use simulated pane content instead.
- **Using `time.Sleep` for synchronization:** Always use `WaitForCondition` or `WaitForPaneContent` polling helpers.
- **Testing `GetStatus()` directly without understanding the layers:** `GetStatus()` has title-based fast path, activity timestamp tracking, spike detection, and spinner grace periods. For DETECT-01, test the detection functions directly. Only use `GetStatus()` for DETECT-03 end-to-end tests.
- **Creating sessions without `TmuxHarness`:** All integration test sessions MUST go through the harness for automatic cleanup.
- **Forgetting event file cleanup:** The events directory (`~/.agent-deck/events/`) is shared. Tests that write events must clean up after themselves. Use `t.Cleanup` with `os.Remove`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tmux session lifecycle | Manual `tmux new-session` / `kill-session` | `TmuxHarness.CreateSession()` | Handles naming, cleanup, error cases |
| Polling for state changes | `time.Sleep` loops | `WaitForCondition()`, `WaitForPaneContent()` | Proper timeout handling, descriptive failure messages |
| Event file watching | Custom inotify/polling | `StatusEventWatcher` (already uses fsnotify) | Handles debouncing, JSON parsing, channel delivery |
| Pattern compilation | Manual regex construction | `DefaultRawPatterns()` + `CompilePatterns()` | Handles regex/string split, invalid pattern recovery |
| ANSI stripping | Regex-based ANSI removal | `tmux.StripANSI()` | O(n) single-pass, handles all ANSI sequence types |

**Key insight:** The detection system is already well-factored. Tests should exercise the existing APIs at the appropriate level rather than reimplementing any detection logic.

## Common Pitfalls

### Pitfall 1: Busy Indicator Priority Over Prompt Detection
**What goes wrong:** Tests may expect a "waiting" status when busy indicators are present because the prompt character `❯` is always visible from the previous input.
**Why it happens:** The status engine gives busy indicators absolute priority. If both a spinner and a prompt are in the pane content, the result is "active" (busy), not "waiting".
**How to avoid:** In simulated content for "waiting" tests, ensure there are NO busy indicators (no spinners, no "ctrl+c to interrupt", no "esc to interrupt", no whimsical word + ellipsis patterns).
**Warning signs:** Tests passing locally but failing under race detector; status returning "active" when "waiting" was expected.

### Pitfall 2: Shell Tool Has No Busy/Prompt Detection
**What goes wrong:** Tests create sessions with `tool="shell"` and expect busy/waiting status transitions driven by content detection.
**Why it happens:** Shell sessions have no `DefaultRawPatterns` (returns nil). The `PromptDetector` has basic shell prompt detection (`$ `, `# `, `% `), but `Instance.UpdateStatus()` maps shell's "waiting" status to `StatusIdle` (line 2377-2378).
**How to avoid:** For DETECT-01 pattern tests, use the specific tool name ("claude", "gemini", "opencode", "codex"). For DETECT-03 transition tests with real tmux, understand that shell sessions will show `StatusIdle` at shell prompt, not `StatusWaiting`.
**Warning signs:** Tests expecting `StatusWaiting` from shell sessions but getting `StatusIdle`.

### Pitfall 3: Tmux Activity Timestamps and Caching
**What goes wrong:** `UpdateStatus()` returns stale results because tmux's `window_activity` timestamp hasn't changed.
**Why it happens:** `GetStatus()` optimizes by skipping `CapturePane()` when no activity change is detected (line 1968). The tmux session cache refreshes every 2 seconds.
**How to avoid:** After modifying pane content (e.g., sending keys), wait at least 500ms before calling `UpdateStatus()`. Use `WaitForCondition` with repeated `UpdateStatus()` calls rather than a single check.
**Warning signs:** Status not updating after content changes; intermittent test failures.

### Pitfall 4: StatusEventWatcher Debouncing Delay
**What goes wrong:** Tests write an event and immediately check the channel, missing the event.
**Why it happens:** The `StatusEventWatcher` has a 100ms debounce timer (event_watcher.go:106). After fsnotify delivers the file event, the watcher waits 100ms before processing.
**How to avoid:** Always use `WaitForStatus()` with a reasonable timeout (5s) instead of directly reading from `EventCh()`. The 200ms startup delay after `go watcher.Start()` allows fsnotify to register.
**Warning signs:** Intermittent test timeouts; events delivered but not seen.

### Pitfall 5: Event File Cleanup in Shared Events Directory
**What goes wrong:** Events from previous test runs are present in `~/.agent-deck/events/`, causing watchers to deliver stale events.
**Why it happens:** The events directory is a global path. Tests that write events must clean up.
**How to avoid:** Use unique instance IDs for each test (e.g., `"inttest-" + t.Name() + "-" + uuid`). Use `filterInstanceID` parameter in `NewStatusEventWatcher()` to filter events. Add `t.Cleanup` to remove event files.
**Warning signs:** Tests receiving unexpected events; non-deterministic test behavior.

### Pitfall 6: Hook Fast Path Overrides Content Detection
**What goes wrong:** Tests set up content detection scenarios but the hook fast path (instance.go:2311) returns a status before content detection runs.
**Why it happens:** For Claude and Codex tools, if `hookStatus` is set and fresh (within `hookFastPathWindow`), `UpdateStatus()` returns the hook-based status immediately.
**How to avoid:** In integration tests, don't set `hookStatus` on instances. The integration test instances are created fresh with empty hook fields. For explicit hook testing, set `hookStatus` and `hookLastUpdate` to control the fast path.
**Warning signs:** Content detection logic not being exercised; status always returning hook-based value.

### Pitfall 7: Grace Period During Startup
**What goes wrong:** Tests call `UpdateStatus()` immediately after `Start()` and get `StatusStarting` instead of expected status.
**Why it happens:** `UpdateStatus()` has a 1.5 second grace period (instance.go:2263) that returns `StatusStarting` if the tmux session doesn't exist yet.
**How to avoid:** After `Start()`, use `WaitForCondition` to ensure the session exists before testing status detection.
**Warning signs:** Tests consistently seeing `StatusStarting` when expecting other statuses.

## Code Examples

Verified patterns from the codebase.

### Simulated Claude Output Patterns
```go
// Source: internal/tmux/detector.go and patterns.go

// Claude BUSY (spinner + whimsical word + timing)
claudeBusy := "✢ Clauding… (25s · ↓ 749 tokens)"
// Claude BUSY (explicit interrupt text)
claudeBusyAlt := "Working on request\nctrl+c to interrupt"
// Claude WAITING (permission dialog)
claudeWaiting := "│ Do you want to run this command?\n❯ Yes, allow once"
// Claude WAITING (input prompt, dangerously-skip-permissions mode)
claudePrompt := "Task completed.\n❯ "
// Claude WAITING (trust prompt on startup)
claudeTrust := "Do you trust the files in this folder?"
```

### Simulated Gemini Output Patterns
```go
// Source: internal/tmux/detector.go and patterns.go

// Gemini BUSY (no explicit busy indicator in detector, uses "esc to cancel" from patterns)
geminiBusy := "Processing request...\nesc to cancel"
// Gemini WAITING (prompt)
geminiWaiting := "gemini>"
// Gemini WAITING (permission)
geminiPermission := "Yes, allow once"
// Gemini WAITING (type prompt)
geminiType := "Type your message"
```

### Simulated OpenCode Output Patterns
```go
// Source: internal/tmux/detector.go and patterns.go

// OpenCode BUSY (spinner characters)
opencodeBusy := "█ Processing request\nesc interrupt"
// OpenCode BUSY (task text)
opencodeBusyAlt := "Thinking...\nSome output here"
// OpenCode WAITING (input placeholder)
opencodeWaiting := "Ask anything\npress enter to send"
// OpenCode WAITING (prompt character)
opencodeWaitingAlt := "open code\n>"
```

### Simulated Codex Output Patterns
```go
// Source: internal/tmux/detector.go and patterns.go

// Codex BUSY (interrupt text)
codexBusy := "Running code...\nesc to interrupt"
// Codex WAITING (prompt)
codexWaiting := "codex>"
// Codex WAITING (continue prompt)
codexContinue := "Continue?"
// Codex WAITING (how can I help)
codexHelp := "How can I help"
```

### Creating Instances with Different Tools
```go
// Source: internal/session/instance.go:339
// Each tool type gets appropriate defaults
inst := session.NewInstanceWithTool("my-session", "/tmp/project", "claude")
assert.Equal(t, "claude", inst.Tool)

inst2 := session.NewInstanceWithTool("my-session", "/tmp/project", "gemini")
assert.Equal(t, "gemini", inst2.Tool)

// Default patterns exist for each supported tool
for _, tool := range []string{"claude", "gemini", "opencode", "codex"} {
    raw := tmux.DefaultRawPatterns(tool)
    assert.NotNil(t, raw)
}
```

### Event Write and Watch
```go
// Source: internal/session/event_writer.go and event_watcher.go
event := session.StatusEvent{
    InstanceID: "instance-123",
    Title:      "test-session",
    Tool:       "claude",
    Status:     "waiting",
    PrevStatus: "running",
    Timestamp:  time.Now().Unix(),
}
err := session.WriteStatusEvent(event)  // Atomic write (tmp + rename)

watcher, err := session.NewStatusEventWatcher("instance-123")  // Filter by instance
go watcher.Start()
defer watcher.Stop()

received, err := watcher.WaitForStatus([]string{"waiting"}, 5*time.Second)
```

### Conductor Send via Tmux
```go
// Source: internal/tmux/tmux.go:3039
tmuxSess := child.GetTmuxSession()
err := tmuxSess.SendKeysAndEnter("hello from conductor")
// SendKeysAndEnter: sends text via SendKeysChunked, waits 100ms, then sends Enter
// This 2-step approach prevents Enter from being swallowed by bracketed paste in tmux 3.2+
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Only "Thinking" and "Connecting" detected | All 90+ whimsical words via regex `[✳✽✶✻✢·]\s*.+…` | 2025-11+ (status_fixes_test.go) | Eliminates false "waiting" when Claude shows whimsical busy words |
| String-only patterns | Hybrid `re:` prefix for regex patterns in RawPatterns | 2025-12+ | Enables anchored regex for precise matching (e.g., line-start anchor for spinner) |
| Single-pass CapturePane | Title-based fast path + activity timestamp gating | 2026-01+ | Reduces subprocess spawning from 74/sec to ~5/sec for idle sessions |
| Direct `string.Contains` | `ResolvedPatterns` with compiled regex + string split | 2025-12+ | Centralizes pattern management, enables user-configurable overrides |
| Echo-based send | `SendKeysAndEnter` with chunked sending + 100ms delay | 2026-01+ | Prevents Enter swallowing in tmux 3.2+ bracketed paste mode |

**Deprecated/outdated:**
- `detector.go`'s `PromptDetector.hasClaudePrompt()`: Still works but the `hasBusyIndicatorResolved()` + `hasPromptIndicator()` path through `ResolvedPatterns` is the authoritative detection path used by `GetStatus()`. `PromptDetector` remains as a fallback layer.

## Open Questions

1. **Testing `UpdateStatus()` with non-shell tools**
   - What we know: `UpdateStatus()` uses both hook fast path and content-based detection. For integration tests, hook fields are empty by default, so content detection will be used.
   - What's unclear: Whether we should also test the hook fast path in this phase, or defer to Phase 6.
   - Recommendation: Focus on content-based detection for DETECT-01/02/03. Hook-based detection is covered by existing unit tests in `status_fixes_test.go`. Defer hook integration testing to Phase 6 if needed.

2. **Event directory isolation for COND-02**
   - What we know: `GetEventsDir()` returns `~/.agent-deck/events/`, which is a global path.
   - What's unclear: Whether tests should use a different events dir to avoid collisions with running agent-deck instances.
   - Recommendation: Use unique instance IDs with `filterInstanceID` parameter in `NewStatusEventWatcher()`. Add `t.Cleanup` to remove test event files. This is sufficient because the watcher filters by filename.

3. **Testing `SendSessionMessageReliable()` for COND-01**
   - What we know: `SendSessionMessageReliable()` invokes the `agent-deck` CLI binary, which must be built and in PATH.
   - What's unclear: Whether tests should test this or use lower-level `tmux.SendKeysAndEnter()` directly.
   - Recommendation: Use `tmux.SendKeysAndEnter()` directly for COND-01. It's the actual transport layer and doesn't require a built binary. `SendSessionMessageReliable` is a convenience wrapper.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | `internal/integration/testmain_test.go` |
| Quick run command | `go test -race -v -run TestDetection ./internal/integration/...` |
| Full suite command | `go test -race -v ./internal/integration/...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DETECT-01 | Pattern detection for Claude, Gemini, OpenCode, Codex with simulated output | unit (in integration pkg) | `go test -race -v -run TestDetection_Patterns ./internal/integration/... -x` | Wave 0 |
| DETECT-02 | Multi-tool session creation produces correct commands and detection config | unit (in integration pkg) | `go test -race -v -run TestDetection_ToolConfig ./internal/integration/... -x` | Wave 0 |
| DETECT-03 | Status transition cycle with real tmux pane content | integration | `go test -race -v -run TestDetection_StatusCycle ./internal/integration/... -x` | Wave 0 |
| COND-01 | Conductor sends command to child via real tmux | integration | `go test -race -v -run TestConductor_Send ./internal/integration/... -x` | Wave 0 |
| COND-02 | Cross-session event notification cycle | integration | `go test -race -v -run TestConductor_Event ./internal/integration/... -x` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race -v -run "TestDetection|TestConductor" ./internal/integration/... -count=1`
- **Per wave merge:** `go test -race -v ./internal/integration/... -count=1`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/integration/detection_test.go` -- covers DETECT-01, DETECT-02, DETECT-03
- [ ] `internal/integration/conductor_test.go` -- covers COND-01, COND-02

*(Existing test infrastructure from Phase 4 is sufficient: harness.go, poll.go, fixtures.go, testmain_test.go all exist)*

## Sources

### Primary (HIGH confidence)
- `internal/tmux/detector.go` -- PromptDetector with tool-specific HasPrompt() and busy indicator detection
- `internal/tmux/patterns.go` -- RawPatterns, ResolvedPatterns, DefaultRawPatterns() per tool, CompilePatterns()
- `internal/tmux/tmux.go:1895-2200` -- Session.GetStatus() with layered detection (title fast path, activity timestamps, busy indicators, prompt detection)
- `internal/session/instance.go:2251-2400` -- Instance.UpdateStatus() with hook fast path and tmux status mapping
- `internal/session/event_writer.go` -- StatusEvent struct, WriteStatusEvent() with atomic tmp+rename
- `internal/session/event_watcher.go` -- StatusEventWatcher with fsnotify, debounce, filter, WaitForStatus()
- `internal/session/transition_notifier.go` -- TransitionNotifier with ShouldNotifyTransition(), dispatch to parent
- `internal/session/send_helper.go` -- SendSessionMessageReliable() wrapping CLI send
- `internal/tmux/tmux.go:3018-3073` -- SendKeys, SendKeysAndEnter, SendKeysChunked
- `internal/session/conductor.go` -- ConductorSettings, ConductorMeta, event pipeline
- `internal/session/tooloptions.go` -- ClaudeOptions, CodexOptions, OpenCodeOptions with ToArgs()
- `internal/integration/` -- Existing harness, polling, fixtures from Phase 4

### Secondary (MEDIUM confidence)
- `internal/tmux/status_fixes_test.go` -- Regression tests showing detection patterns and testing methodology
- `internal/tmux/patterns_test.go` -- Unit tests for DefaultRawPatterns per tool

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- using only existing project dependencies, all verified in go.mod
- Architecture: HIGH -- test structure follows established Phase 4 patterns exactly
- Detection patterns: HIGH -- read all source files for all four tools, patterns verified against code
- Conductor patterns: HIGH -- read event_writer, event_watcher, transition_notifier, send_helper source
- Pitfalls: HIGH -- identified from reading the actual GetStatus() implementation and its many edge cases

**Research date:** 2026-03-06
**Valid until:** 2026-04-06 (stable codebase, patterns well-established)
