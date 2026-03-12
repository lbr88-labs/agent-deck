# Pitfalls Research

**Domain:** Session reliability, resume UX, and mouse support for Go/BubbleTea/tmux TUI (v1.3 milestone)
**Researched:** 2026-03-12
**Confidence:** HIGH (based on direct codebase analysis of instance.go, storage.go, statedb/migrate.go, home.go, and verified against official Bubble Tea, Claude Code hooks, and SQLite documentation)

## Critical Pitfalls

### Pitfall 1: Sandbox Config Lost on Save-Load Round-Trip

**What goes wrong:**
`SandboxConfig` is stored as a JSON blob inside the `tool_data` column via `MarshalToolData`/`UnmarshalToolData`. If the `Sandbox` field is non-nil but the marshal step silently encodes it as `null` (e.g., due to a pointer dereference mistake or a nil-check short-circuit), every subsequent load returns `nil` Sandbox. The session restarts without its container, appearing to work but silently dropping the sandbox constraint.

**Why it happens:**
The `SaveWithGroups` path does:
```go
var sandboxJSON json.RawMessage
if inst.Sandbox != nil {
    data, _ := json.Marshal(inst.Sandbox)
    sandboxJSON = data
}
```
If `inst.Sandbox` is non-nil but `inst.Sandbox.Enabled == false`, the JSON is encoded correctly, but callers may forget to check `Enabled` on load and skip container setup. Conversely, an `Enabled: true` sandbox with a zero-value `Image` field will produce valid JSON that is stored and reloaded, but `docker run` will fail at startup with a cryptic error rather than at save time with a validation error. The save layer accepts any `SandboxConfig` without validation.

**How to avoid:**
1. Add a `Validate() error` method to `SandboxConfig` called at session start, not just at session creation.
2. Write a round-trip test: create an `Instance` with a fully-populated `SandboxConfig`, save to a temp SQLite via `SaveWithGroups`, load it back, and assert deep-equal on the `SandboxConfig`. This test currently does not exist.
3. The `json.Marshal` error is silently swallowed (`data, _ = json.Marshal(...)`) in `MarshalToolData`. Promote it to a fatal log to surface encoding failures immediately.

**Warning signs:**
- Sandboxed session restarts but no container name appears in `SandboxContainer` field after startup
- `docker ps` shows no container after a sandboxed session resumes
- `Sandbox.Image` is empty string in a running session

**Phase to address:**
Phase 1 (Sandbox Config Persistence). The round-trip test must be written and passing before the feature is considered complete.

---

### Pitfall 2: toolDataBlob Field Addition Breaks Existing Stored Rows

**What goes wrong:**
When a new field is added to `toolDataBlob` in `statedb/migrate.go`, existing rows in `tool_data` do not contain that key. `json.Unmarshal` silently leaves the new field at its zero value. For `string` fields this is `""`, for `*bool` this is `nil`, and for `json.RawMessage` this is `nil`. The behavior is correct by Go JSON rules, but causes invisible data loss when code later checks `if field != ""` and branches incorrectly.

**Why it happens:**
The `toolDataBlob` struct is the sole serialization boundary between `Instance` fields and SQLite. Any new field added to `Instance` must also be added to `toolDataBlob`, to `MarshalToolData`'s parameter list, to `UnmarshalToolData`'s return list, to every callsite of both functions, and to the migration path in `MigrateFromJSON`. This is a 6-location change. Missing any one location produces a field that exists in Go structs but is never persisted or is persisted but never read back.

**How to avoid:**
1. The `MarshalToolData` function signature is already 18 parameters wide (as of current codebase). Adding more makes it brittle. Refactor to accept a `toolDataBlob` struct directly so the compiler catches missing fields: `MarshalToolData(td toolDataBlob) json.RawMessage`.
2. Add a property-based round-trip test that verifies every exported field of `toolDataBlob` survives marshal/unmarshal. Use `reflect` to enumerate fields and assert none are zero after a round-trip of a fully-populated struct.
3. After adding any field to `toolDataBlob`, run `grep -n 'MarshalToolData\|UnmarshalToolData' ./...` and verify all callsites were updated.

**Warning signs:**
- A new field always shows its zero value even after saving a session that had it set
- `UnmarshalToolData` returns more values than `MarshalToolData` accepts parameters (signature drift)
- `MigrateFromJSON` does not reference a field that appears in `toolDataBlob`

**Phase to address:**
Phase 1 (Sandbox Config Persistence). Refactoring `MarshalToolData` to accept a struct is a prerequisite, not an optimization.

---

### Pitfall 3: Duplicate Claude Session IDs After Resume if Dedup Runs Before ID Detection

**What goes wrong:**
`UpdateClaudeSessionsWithDedup` is called both on load in `home.go` and in `SaveWithGroups`. If a resumed session doesn't yet have a `ClaudeSessionID` (the tmux session is starting and the hook hasn't fired yet), dedup correctly does nothing. But if the user opens agent-deck in two windows simultaneously, both call `UpdateClaudeSessionsWithDedup` on the same set of loaded instances. Whichever window processes the hook first assigns the Claude session ID to "its" instance; the other window loads from SQLite without the ID, runs dedup (no-op since no ID yet), then the hook fires in the second window too. Both instances now claim the same Claude session ID from different processes, and the dedup only clears it within a single in-memory pass, not across processes.

**Why it happens:**
`UpdateClaudeSessionsWithDedup` is a pure in-memory operation. It has no SQLite-level locking or cross-process coordination. The `StorageWatcher` (fsnotify) will eventually reload both windows, but there is a window between hook delivery and storage reload where both in-memory models have the same ID.

**How to avoid:**
1. The dedup pass must run on every `SaveWithGroups` call (it already does), which writes the canonical state to SQLite. A second process reloading from SQLite after the first write will see the dedup result.
2. The key invariant is: dedup must win over stale in-memory state. The `StorageWatcher` reload interval (currently driven by fsnotify) must fire within a bounded time after a save. Verify this in a test: write duplicate IDs to SQLite from two goroutines, assert dedup resolves within 3 seconds.
3. Never derive the "canonical" Claude session ID from in-memory state alone. Always treat the SQLite-persisted value as authoritative after a reload.

**Warning signs:**
- Two sessions in the list both show the same Claude conversation thread
- `UpdateClaudeSessionsWithDedup` clears an ID that was just set, causing repeated re-detection cycles
- Logs show "claude_session_detected" for the same session ID more than once across different instances

**Phase to address:**
Phase 2 (Session Deduplication). Write a concurrent-access test that starts two `Storage` objects against the same SQLite file, performs concurrent saves with duplicate IDs, and asserts the final state is deduplicated.

---

### Pitfall 4: Auto-Start TTY Detection Breaks Tool Interactivity on WSL/Linux

**What goes wrong:**
Claude Code, Gemini CLI, and other AI tools check `os.Stdout.Fd()` with `isatty` to determine whether they are running interactively. When agent-deck's auto-start feature redirects stdout to a log file or pipe for capture, the tool detects "not a TTY" and switches to non-interactive mode: disabling color output, streaming JSON instead of human-readable responses, or refusing to run entirely (Claude Code requires a TTY for its input).

**Why it happens:**
On macOS, tmux creates a new PTY for each session automatically. The shell inside the tmux session always has a PTY. On WSL/Linux, the auto-start path may spawn the process differently (e.g., via `exec.Command` with stdout redirected to capture session output for monitoring), which bypasses the tmux PTY and runs the process without a TTY attached. The issue is that TTY detection is done at process startup, before the tool connects to its UI layer.

**How to avoid:**
1. Never redirect stdout of a tool process directly. All tool processes must start inside a tmux session, which guarantees PTY. The auto-start path must be `tmux new-session -d -s {name} {command}`, not `exec.Command(command)`.
2. Test auto-start specifically on a system where tmux is running but the invoking process has no TTY (simulate with `script -q /dev/null agent-deck --auto-start` or by running from a cron-style context).
3. Add an integration test that starts a session via the auto-start code path, captures the pane, and asserts the tool started in interactive mode (e.g., Gemini shows its interactive prompt, Claude shows its TUI).

**Warning signs:**
- Sessions started via auto-start show "stdin is not a terminal" or similar errors
- Claude Code exits immediately after auto-start with exit code 1
- Tool output is raw JSON instead of formatted UI (non-interactive mode indicator)

**Phase to address:**
Phase 3 (Auto-Start TTY Fix). The fix is a one-liner (ensure tool runs inside tmux, not directly), but the test to verify it on WSL is the hard part.

---

### Pitfall 5: Race Condition Between Multiple Concurrent Hook Fires

**What goes wrong:**
Claude Code hooks (`Stop`, `PreToolUse`, `PostToolUse`) can fire multiple times in rapid succession when Claude executes a sequence of tool calls. Each hook invocation writes to the `hookStatus` field on `Instance` and may trigger a status update. If two hook fires arrive concurrently (hook A sets "running", hook B immediately sets "idle"), the final status depends on goroutine scheduling, not hook arrival order.

**Why it happens:**
The `StatusFileWatcher` and the `TransitionDaemon` poll hook status from files on disk at 1-3 second intervals. Claude Code writes hook status files asynchronously. The daemon's `hookStatusForInstance` reads both from the file watcher and from the file directly (`readHookStatusFile`). If both reads happen while hooks are actively writing, the daemon may see partial state from two different hooks within the same poll cycle.

**How to avoid:**
1. Hook status updates must be timestamp-ordered. The `HookStatus` struct should always carry the hook's `session_id` and a monotonic timestamp (Claude Code provides `hook_event_id` in its hook payload). Discard any update older than the most recently applied one.
2. The `hookFreshWindow` (45 seconds, defined in `transition_daemon.go`) prevents stale hook data from influencing status. Verify this constant is honored on every code path that reads hook status.
3. Write a test that fires two hook status files for the same session within 100ms and asserts the final status reflects the later-timestamped one.

**Warning signs:**
- Status bounces between "running" and "idle" rapidly without user action
- Log shows "hook_status_applied" for the same session twice within 1 second with different statuses
- `hookLastUpdate` timestamp decreases between consecutive `GetHookStatus` calls (older hook processed after newer)

**Phase to address:**
Phase 2 (Session Deduplication) overlaps here. The hook ordering logic should be audited whenever dedup logic is touched, since both deal with multi-source status authority.

---

### Pitfall 6: Stopped Sessions Invisible in TUI Due to Filter Logic

**What goes wrong:**
`StatusStopped` sessions are currently filtered out in the session picker dialog (`session_picker_dialog.go:41`). If the same filter is accidentally applied in the main list, stopped sessions become invisible. Users cannot resume them from the TUI, defeating the purpose of `#307` (show stopped sessions as resumable).

**Why it happens:**
The filter pattern `if inst.Status == session.StatusError || inst.Status == session.StatusStopped { continue }` is used in at least two places (`session_picker_dialog.go` and `home.go:getOtherActiveSessions`). It is appropriate in the conductor's "pick a session to send a message to" dialog, but inappropriate in the main session list. When adding stopped-session visibility, developers copy existing rendering logic and may inadvertently carry over the exclusion filter.

**How to avoid:**
1. Before adding stopped-session UI, audit every location that filters by `StatusStopped` and explicitly document which filters are correct (conductor picker, active-sessions-only queries) vs. incorrect (main list, resume flow).
2. The `flatItems` construction in `home.go` must explicitly include `StatusStopped` sessions when `statusFilter` is `""` (all sessions). Write a unit test for the flat-items build that asserts a stopped session appears in the list.
3. The preview pane already handles stopped sessions (`home.go:9871` special case). Ensure this codepath is reached, not short-circuited by a list-level filter.

**Warning signs:**
- A session stopped by the user disappears from the list
- The stopped session reappears only after agent-deck restarts (from SQLite)
- The "resume" keybinding is unreachable because the stopped session isn't selectable

**Phase to address:**
Phase 4 (Resume UX). Requires an explicit list of all `StatusStopped` filter locations before writing any new UI code.

---

### Pitfall 7: Mouse Events Conflict with tmux Mouse Mode and Disable Text Selection

**What goes wrong:**
`tea.WithMouseCellMotion()` is already enabled in `main.go:468`. This activates terminal mouse event capture at the Bubble Tea level. When the user is inside a tmux session (the outer TUI), and tmux also has `set -g mouse on` in `.tmux.conf`, two layers of mouse event capture compete. Scroll events may go to tmux (scrolling tmux history) instead of Bubble Tea (scrolling the session list). Click events on the session list may be intercepted by tmux for pane selection. Additionally, enabling mouse capture in Bubble Tea unconditionally disables native text selection in the terminal; the user must hold Shift to select text.

**Why it happens:**
tmux intercepts mouse events before passing them to the running application. When tmux's mouse mode is on, tmux consumes scroll events for its copy-mode and only passes them through to the application under specific conditions (e.g., the terminal has focus and the pane is not in copy mode). Bubble Tea's `WithMouseCellMotion` uses the SGR 1002 escape sequence to request cell-motion mouse events from the terminal. If tmux also requested mouse events, both layers receive them, but tmux's handler runs first and may swallow the event before Bubble Tea sees it.

**How to avoid:**
1. The Bubble Tea program already has `WithMouseCellMotion()`. The implementation work for mouse support (#262, #254) is primarily about handling `tea.MouseMsg` in `Update()`, not about enabling mouse support (it is already on).
2. Mouse scroll in the session list should map to scroll events (`tea.MouseMsg.Button == tea.MouseButtonWheelUp/Down`). These are distinct from click events. Test both inside and outside tmux.
3. Document the text-selection limitation in the settings panel: "Mouse mode is active. Hold Shift to select text." This is not fixable without disabling mouse support.
4. The mouse event handler in `Update()` must check `h.activeDialog` before dispatching. A click on the session list while a dialog is open should not navigate the list.
5. Do NOT switch to `tea.WithMouseAllMotion()`. Cell motion (1002) is sufficient and better supported across terminals. All-motion (1003) sends a mouse event on every cursor move, flooding the event queue.

**Warning signs:**
- Scroll on the session list scrolls tmux history instead
- Clicking a session in the list selects a tmux pane instead of the Bubble Tea item
- A drag action (mouse button held + move) generates spurious `KeyMsg` events in `Update()` (documented Bubble Tea bug when `Update()` is slow)

**Phase to address:**
Phase 5 (Mouse Support). The `tea.WithMouseCellMotion()` option is already set. The work is handling `tea.MouseMsg` in `Update()` and testing the tmux interaction layer.

---

### Pitfall 8: Slow Update() or View() Causes Spurious KeyMsg Events from Mouse Motion

**What goes wrong:**
When `Update()` or `View()` blocks for more than ~50ms (e.g., while iterating through many sessions, computing flat items, or awaiting a mutex), buffered terminal input accumulates in the event queue. Mouse motion events, when decoded while the queue is backed up, can be misinterpreted as `KeyMsg` events with `Type: -1`. The user sees spurious key presses that trigger unintended navigation or actions.

**Why it happens:**
This is a documented Bubble Tea issue (GitHub issue #1047). The root cause is that slow `Update()` allows more raw bytes to accumulate in the input buffer than the parser expects for a single mouse motion event. The parser then misidentifies the trailing bytes as key input. The `home.go` `Update()` function is already ~8500 lines and handles dozens of message types; any blocking operation in it makes this worse.

**How to avoid:**
1. `Update()` and `View()` must never block. Every slow operation (SQLite read, tmux capture, file I/O) must be in a `tea.Cmd` (background goroutine that returns a `tea.Msg`). This is already the architectural pattern in `home.go` but must be preserved as new mouse handling code is added.
2. The mouse click handler for session list items must be O(1): given mouse Y coordinate, compute the list item index mathematically (item height is fixed), do not iterate the full `flatItems` slice.
3. Adding mouse support must not introduce any synchronous I/O in the mouse event path. If clicking a session needs to load its preview, that load must be dispatched as a `tea.Cmd` and the click must return immediately.

**Warning signs:**
- After adding mouse handlers, pressing arrow keys generates double-navigation (one from the key, one spurious)
- `AGENTDECK_DEBUG=1` logs show `KeyMsg{Type:-1, Runes:[91], Alt:true}` events
- UI lag (visible render delay) after mouse interaction

**Phase to address:**
Phase 5 (Mouse Support). The O(1) item-from-coordinate calculation must be designed before any mouse handler is written.

---

### Pitfall 9: SQLite WAL Mode Write Contention During High-Frequency Status Updates

**What goes wrong:**
The background status worker writes individual session status changes to SQLite via `db.SetStatus()`. If status updates fire rapidly (e.g., multiple sessions transitioning simultaneously, or the control pipe flooding `%output` events), the WAL file grows faster than the WAL checkpoint can flush. Concurrent readers see slightly stale data; concurrent writers get `SQLITE_BUSY` errors. The existing retry logic in `migrateStateDBWithRetry` handles this for migrations, but per-status writes have no retry.

**Why it happens:**
SQLite WAL mode allows one writer at a time. Multiple goroutines calling `db.SetStatus()` concurrently serialize through SQLite's locking, but if a write is in progress when another write arrives, the second write gets `SQLITE_BUSY`. The current code uses `modernc.org/sqlite` (no CGO), which honors SQLite's busy timeout. If the busy timeout is not set, the write fails immediately.

**How to avoid:**
1. Verify `PRAGMA busy_timeout = 5000` (5 seconds) is set on `statedb.Open()`. If it is not set, `SQLITE_BUSY` becomes a silent data loss risk.
2. Batch status writes: instead of one `SetStatus` call per session per tick, accumulate all status changes from a single tick and write them in one transaction.
3. The `StorageWatcher` fires on every `Touch()`, which is called on every `SaveWithGroups`. If status updates also call `Touch()`, the watcher floods the UI with reload messages. Confirm that `db.SetStatus()` does NOT call `Touch()` (verify in `statedb` implementation).

**Warning signs:**
- Logs show `database is locked` or `SQLITE_BUSY` errors in status update paths
- Session status in TUI is 2-3 seconds behind actual session state
- WAL file grows unboundedly (`~/.agent-deck/profiles/default/state.db-wal` exceeds 10MB)

**Phase to address:**
Phase 1 (Sandbox Config Persistence) sets up the schema; Phase 2 onwards writes status updates more frequently. Verify busy_timeout at Phase 1.

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Silently swallowing `json.Marshal` errors in `MarshalToolData` | No error to handle at callsite | Silent data loss when `SandboxConfig` fails to serialize | Never. Log at minimum, fail on critical fields |
| Adding fields to `toolDataBlob` without refactoring the 18-param signature | Smallest diff | Signature grows unbounded, callsites drift | Refactor to struct now, before adding more fields |
| Handling mouse clicks by iterating `flatItems` slice | Simple to understand | O(n) in number of sessions; causes Update() lag at 50+ sessions | Never in event-hot paths |
| Relying on `StorageWatcher` for cross-process session state sync | No explicit coordination needed | Up to 2s lag; can miss rapid back-to-back saves | Acceptable for UI refresh, not for correctness-critical dedup |
| Filtering `StatusStopped` in helper functions without documentation | Avoids showing stopped in conductor flows | Filter silently applied in wrong contexts when code is copied | Always document intent at filter site with a comment |

## Integration Gotchas

Common mistakes when connecting these features to the existing system.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| `SandboxConfig` persistence | Assuming `json.RawMessage` nil-safety matches `*SandboxConfig` nil-safety | Write and run the round-trip test before shipping |
| Claude Code hooks | Treating hook status as the sole source of truth | Hooks are fast-path; tmux `capture-pane` is the fallback. Both must agree before status change |
| Mouse events in tmux nesting | Testing mouse support only outside tmux | Always test inside a tmux session (the real deployment context) |
| Stopped session visibility | Adding to render path without auditing filter locations | Grep all `StatusStopped` exclusions before writing new render code |
| `MarshalToolData` signature extension | Adding a parameter and updating one callsite | All callsites are in `storage.go` (SaveWithGroups), `storage.go` (LoadLite), and `statedb/migrate.go` (MigrateFromJSON). Must update all three |
| Bubble Tea `Update()` mouse handler | Reading from SQLite synchronously in mouse handler | Dispatch as `tea.Cmd`. Mouse handler must return within microseconds |

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Linear scan of `flatItems` to find item at mouse Y coordinate | Works at 10 sessions | Visible lag at 50+ sessions | At 30+ sessions with mouse events firing |
| Writing to SQLite on every status tick for each session | Works with 5 sessions | WAL contention at 20+ rapidly-transitioning sessions | At 15-20 sessions when multiple conductors are active |
| `StorageWatcher` reload triggered by every `SetStatus` call | Convenient sync | UI flickers on every status change | Immediately; `SetStatus` must NOT call `Touch()` |
| Enabling `tea.WithMouseAllMotion()` instead of `WithMouseCellMotion()` | Captures all mouse movement | Floods event queue with motion events between clicks | Immediately; never use AllMotion for this use case |

## Security Mistakes

Domain-specific security issues for this feature set.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Storing sandbox CPU/memory limits in `SandboxConfig` without validation | User can set `CPULimit: "0"` or `MemoryLimit: "-1"`, bypassing limits | Validate limits in `NewSandboxConfig` and on load |
| Resuming a stopped session that was running in a deleted worktree | Session re-runs in a non-existent directory, creating files in wrong location | Check `WorktreePath` exists before resume; show error dialog if path is missing |
| Hook status files written by external processes with no authentication | Any process can write `hookStatus: "dead"` for any session, causing false termination | Hook files are scoped to `~/.agent-deck/hooks/{sessionID}/`. Not a security boundary, but scope limits blast radius |

## UX Pitfalls

Common user experience mistakes when adding these features.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Stopped sessions look identical to error sessions in the list | User cannot distinguish "intentionally stopped" from "crashed" | Use distinct icon/color: `StatusStopped` gets a dim gray square (already in `styles.go`), `StatusError` gets a red X |
| Mouse scroll navigates the list AND scrolls tmux copy-mode simultaneously | Double scroll, disorienting | tmux catches scroll first when not in Bubble Tea focus; document this as expected behavior |
| Clicking a stopped session immediately starts it instead of showing a confirmation | Accidental restart of long-running sessions | Show a resume confirmation or dedicated keybinding before restarting |
| Mouse text selection silently disabled with no indication | User drags to select text, nothing happens | Add a status bar note: "Mouse mode active. Shift+drag to select text." |
| Deduplication silently clears a Claude session ID the user can see | The conversation label disappears from the session row | Log dedup action at INFO level; never silently discard a session ID that was visible |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **Sandbox persistence:** Often missing a round-trip test that actually loads the saved config from a fresh `Storage` instance (not the same in-memory instance that saved it)
- [ ] **Sandbox persistence:** Often missing validation that `SandboxConfig.Image` is non-empty before saving — a blank image string is valid JSON but will crash `docker run`
- [ ] **Session deduplication:** Often missing coverage of the case where both instances are `CreatedAt` the same timestamp (sort stability must be defined)
- [ ] **Session deduplication:** Often missing the concurrent-write test (two processes saving overlapping Claude session IDs to the same SQLite)
- [ ] **Auto-start TTY fix:** Often missing a test that runs the auto-start path and verifies the tool received a PTY (not just that the session was created)
- [ ] **Auto-start TTY fix:** Often missing WSL-specific behavior (Linux `/proc/self/fd/1` TTY check differs from macOS)
- [ ] **Stopped session UI:** Often missing the case where a stopped session is in a collapsed group — it must not be silently hidden
- [ ] **Mouse support:** Often missing testing inside a tmux session (tested in raw terminal only)
- [ ] **Mouse support:** Often missing the case where a dialog is open — clicks behind the dialog must not navigate the list
- [ ] **Mouse support:** Often missing scroll while the preview pane has focus — scroll should scroll the preview, not the session list
- [ ] **Settings custom tools:** Often missing persistence — settings saved to config file, not just updated in-memory for the current session

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| `SandboxConfig` lost from existing sessions | MEDIUM | Re-add sandbox config via `agent-deck session edit` (if it exists) or delete and recreate the session with `--sandbox` flag |
| `toolDataBlob` field added but not marshaled | LOW | Add field to `MarshalToolData`, re-save affected sessions. No data loss if field was optional |
| Duplicate Claude session IDs in production database | LOW | Run `agent-deck session list` to identify duplicates. The next `SaveWithGroups` call deduplicates them |
| Mouse support breaks keyboard navigation | LOW | `tea.WithMouseCellMotion()` can be temporarily removed from `main.go` as a one-line rollback |
| Spurious KeyMsg events from mouse motion | MEDIUM | Reduce `Update()` blocking time. Profile with `pprof`. As a short-term workaround, add a `time.Since(lastMouseEvent) < 100ms` guard |
| Stopped sessions not showing in list | LOW | Identify the filter site via grep, remove the `StatusStopped` exclusion. No data loss |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Sandbox config lost on round-trip | Phase 1 (Sandbox persistence) | Round-trip test passes against a fresh Storage instance |
| toolDataBlob field drift | Phase 1 (Sandbox persistence) | Refactor MarshalToolData to struct parameter; compiler enforces all fields |
| Duplicate Claude session IDs cross-process | Phase 2 (Session dedup) | Concurrent-write test with two Storage instances |
| Hook status race (ordering) | Phase 2 (Session dedup) | Rapid dual-hook test asserts later-timestamped status wins |
| Auto-start TTY break | Phase 3 (Auto-start TTY fix) | Integration test verifying PTY is allocated for tool process |
| Stopped sessions invisible | Phase 4 (Resume UX) | List render test asserts StatusStopped item is present in flatItems |
| Filter applied in wrong context | Phase 4 (Resume UX) | Audit of all StatusStopped exclusion sites before merge |
| Mouse events conflict with tmux | Phase 5 (Mouse support) | Manual test inside tmux session; scroll/click both work as expected |
| Slow Update() causing spurious KeyMsg | Phase 5 (Mouse support) | MouseMsg handler benchmarked at O(1) per item; no blocking I/O |
| WAL contention on status writes | All phases (ongoing) | Verify busy_timeout set in statedb.Open(); monitor WAL file size in CI |

## Sources

- Agent-deck codebase: `internal/session/instance.go` (SandboxConfig, toolDataBlob, UpdateClaudeSessionsWithDedup), `internal/session/storage.go` (SaveWithGroups, LoadLite), `internal/statedb/migrate.go` (MarshalToolData, UnmarshalToolData, MigrateFromJSON), `internal/ui/home.go` (Update, flatItems construction, attachSession), `cmd/agent-deck/main.go` (tea.WithMouseCellMotion)
- [Bubble Tea GitHub Issue #162: Mouse mode disables text selection](https://github.com/charmbracelet/bubbletea/issues/162)
- [Bubble Tea GitHub Issue #1047: KeyMsg emitted during mouse drag when Update() is slow](https://github.com/charmbracelet/bubbletea/issues/1047)
- [Bubble Tea docs: tea package on pkg.go.dev](https://pkg.go.dev/github.com/charmbracelet/bubbletea) — WithMouseCellMotion vs WithMouseAllMotion behavior
- [Claude Code Hooks reference](https://code.claude.com/docs/en/hooks) — hook lifecycle, concurrent firing behavior, session_id in payload
- [SQLite WAL mode documentation](https://sqlite.org/wal.html) — WAL contention and busy_timeout behavior
- [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/) — Keep Update() fast, message ordering not guaranteed
- Agent-deck CLAUDE.md: documented production incidents (2025-12-11 data corruption, 2026-01-20 RAM leak)

---
*Pitfalls research for: Session reliability, resume UX, and mouse support (v1.3 milestone)*
*Researched: 2026-03-12*
