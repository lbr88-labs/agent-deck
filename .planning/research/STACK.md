# Stack Research: v1.3 Session Reliability & Resume

**Domain:** Terminal session manager TUI (Go + Bubble Tea + tmux), subsequent milestone
**Researched:** 2026-03-12
**Confidence:** HIGH — verified against module cache, live source files, and existing code

---

## Context: Milestone-Scoped Research

This is a SUBSEQUENT MILESTONE research pass for v1.3. The foundational stack (Go 1.24, Bubble Tea, tmux, modernc.org/sqlite WAL) is validated and unchanged. This document covers ONLY the stack additions or changes needed for five v1.3 features:

1. Sandbox config persistence in SQLite (#320)
2. TTY handling for auto-start on WSL/Linux (#311)
3. Session deduplication by conversation ID on resume (#224)
4. Mouse/trackpad support in Bubble Tea TUI (#262, #254)
5. Resume UX: stopped sessions visible and resumable (#307)

**Bottom line: No new dependencies required. All five features are implementable with existing packages.**

---

## What Already Exists (Validated, No Changes Needed)

| Area | Current State | Gap for v1.3 |
|------|--------------|--------------|
| `SandboxConfig` in `InstanceData` | Already marshaled to JSON in `tool_data` column; `TestStorageSaveWithGroups_PersistsSandboxConfig` passes | Bug is in load-to-restart data flow, not schema |
| `UpdateClaudeSessionsWithDedup()` | Runs at save, load, and session creation | Not called in the resume code path |
| `tea.WithMouseCellMotion()` | Already passed to `tea.NewProgram` in `main.go:468` | Zero `case tea.MouseMsg` handlers exist |
| `StatusStopped` | Defined, persisted, has its own lipgloss style | Excluded from session picker; help text unclear |
| `golang.org/x/term v0.37.0` | Already imported in `main.go`; `term.IsTerminal()` used in `drainStdin()` | Available for TTY detection in CLI paths |
| `mattn/go-isatty v0.0.20` | Transitive dependency via bubbletea/termenv | No direct use needed; `x/term` suffices |
| `creack/pty v1.1.24` | Used in `web/terminal_bridge.go` and `tmux/pty.go` | Not relevant to v1.3 features |

---

## Recommended Stack

### Core Technologies (No Changes)

| Technology | Version | Purpose | Status |
|------------|---------|---------|--------|
| Go | 1.24+ | Runtime | Validated, no change |
| `charmbracelet/bubbletea` | v1.3.10 | TUI framework | Validated, no change |
| `charmbracelet/bubbles` | v0.21.0 | TUI input components | Validated, no change |
| `charmbracelet/lipgloss` | v1.1.0 | Styling | Validated, no change |
| `modernc.org/sqlite` | v1.44.3 | SQLite, no CGO | Validated, no change |
| `golang.org/x/term` | v0.37.0 | Terminal detection (`IsTerminal`) | Already direct dep, used in v1.3 |

### Supporting Libraries (No New Additions)

No new `go get` commands. All five features use existing dependencies.

---

## Feature-by-Feature Stack Analysis

### Feature 1: Sandbox Config Persistence (#320)

**Problem:** When a sandboxed session is stopped and restarted, the session may lose its `SandboxConfig`. The schema already supports persistence — the bug is in the data flow from SQLite load to session restart.

**Root cause to investigate:** `prepareCommand()` in `instance.go:4725` calls `wrapForSandbox()` which reads `inst.Sandbox`. If `inst.Sandbox` is nil at restart time (because it was not loaded from storage, or was cleared during a stop operation), the restart runs without sandbox wrapping. Verify the `Load()` → `convertToInstances()` → `inst.Sandbox` chain survives a stop/restart cycle.

**Stack requirement:** None. The `Sandbox *SandboxConfig` field in `Instance` and the `decodeSandboxConfig()` function in `storage.go:786` already do the right thing. Fix is in ensuring `inst.Sandbox` is populated before `prepareCommand()` is called on restart.

**Integration point:** `cmd/agent-deck/session_cmd.go` restart handler and/or `internal/ui/home.go` session restart message handler — verify `inst.Sandbox` is not nil.

**Confidence:** HIGH — schema confirmed correct via `TestStorageSaveWithGroups_PersistsSandboxConfig`; bug is behavioral.

---

### Feature 2: TTY Auto-Start Fix for WSL/Linux (#311)

**Problem:** `agent-deck session start` invoked from a non-interactive context (script, conductor) fails on WSL/Linux when the AI tool (claude, gemini) receives a broken TTY environment.

**Mechanism understood:** `tmux new-session -d` creates a detached session with its own PTY. The tool inside has a real PTY from tmux. The issue is likely one of:
- Agent-deck's own stdio (not a TTY when called from a script) causing agent-deck itself to error before launching tmux
- Environment variable leakage (`NO_COLOR`, missing `TERM`) from the non-TTY parent context contaminating the tmux session environment
- `wrapIgnoreSuspend()` adding `bash -c 'stty susp undef; ...'` which fails in some Linux configurations

**Stack requirement:** `golang.org/x/term` is already imported. `term.IsTerminal(int(os.Stdin.Fd()))` is the correct check. No new packages needed.

**Integration pattern:**
```go
// In CLI path before launching tmux:
isInteractive := term.IsTerminal(int(os.Stdin.Fd()))
// If !isInteractive, suppress agent-deck's own interactive prompts
// The tool inside tmux always has its own PTY regardless
```

**Confidence:** MEDIUM — exact failure mode of #311 is not documented in the available issue context files. The TTY detection approach is correct; the specific fix requires reproducing on WSL/Linux. Flag this for hands-on debugging during implementation.

---

### Feature 3: Session Deduplication by Conversation ID (#224)

**Problem:** When a user resumes a Claude conversation via `--resume session_id`, agent-deck may create a new `Instance` while the old one (with the same `ClaudeSessionID`) still exists, resulting in two visible entries sharing one Claude conversation.

**Existing mechanism:** `UpdateClaudeSessionsWithDedup()` in `instance.go:4952` already handles dedup by `ClaudeSessionID`. It runs at save, load, and session creation (`sessionCreatedMsg`, `sessionForkedMsg`, `sessionImportedMsg` handlers in `home.go`). It is NOT called in the resume code path.

**Stack requirement:** None. Add a `UpdateClaudeSessionsWithDedup()` call in the resume handler, or perform a targeted pre-check: before persisting a resumed session, verify no existing session already owns the target `ClaudeSessionID`. If one does, reuse or replace it instead of creating a new entry.

**Integration point:** The `--resume <session_id>` flag is parsed in `internal/ui/claudeoptions.go` and `cmd/agent-deck/session_cmd.go`. The fix should run in the code path that creates the new `Instance` for a resumed session.

**Confidence:** HIGH — mechanism exists and is documented; scope extension is straightforward.

---

### Feature 4: Mouse/Trackpad Support (#262, #254)

**Problem:** `tea.WithMouseCellMotion()` is already enabled, so Bubble Tea receives mouse events. There are zero `case tea.MouseMsg` handlers in the codebase, so all scroll and click events are silently dropped.

**Verified API (from module cache `/Users/ashesh/go/pkg/mod/github.com/charmbracelet/bubbletea@v1.3.10/mouse.go`):**

```go
// In Update() switch:
case tea.MouseMsg:
    switch msg.Button {
    case tea.MouseButtonWheelUp:
        // dispatch same logic as 'k' key
    case tea.MouseButtonWheelDown:
        // dispatch same logic as 'j' key
    case tea.MouseButtonLeft:
        if msg.Action == tea.MouseActionPress {
            // click-to-select: map msg.Y to session list index
        }
    }
```

**Deprecated constants to avoid:**
The old `tea.MouseWheelUp` / `tea.MouseWheelDown` constants are deprecated as of v1.3.x. Use `msg.Button == tea.MouseButtonWheelUp` and `msg.Button == tea.MouseButtonWheelDown` instead.

**Scrollable areas needing handlers (from issue #254):**
1. Session list panel (`home.go`) — `viewOffset` + cursor system
2. Help overlay — scroll offset
3. Settings panel (`settings_panel.go`) — scroll offset
4. Global search results/preview (`global_search.go`) — `previewScroll`
5. Dialogs (NewDialog, ForkDialog, MCPDialog) — various scroll implementations

**Click-to-select:** Issue #262 requests clicking on sessions to navigate. `msg.Y` is the terminal row; mapping Y to session index requires knowing each item's rendered Y position. The simplest approach: track `viewOffset` and item height, compute `targetIndex = viewOffset + (msg.Y - listStartRow)`.

**Stack requirement:** None. Pure code addition to `internal/ui/home.go` (and potentially helper files for each scrollable area). No new packages.

**Performance note:** `tea.WithMouseCellMotion()` (already in use) sends motion events only on cell boundary crossings, not on every pixel. Guard click handlers with `msg.Action == tea.MouseActionPress` to avoid processing drag/release events.

**Confidence:** HIGH — fully verified against Bubble Tea v1.3.10 module cache.

---

### Feature 5: Resume UX — Stopped Sessions in TUI (#307)

**Problem:** Stopped sessions exist in SQLite and display in the list with `SessionStatusStopped` styling, but the help text and UX flow do not make resume obvious. The session picker dialog (`session_picker_dialog.go:41`) explicitly excludes `StatusStopped` sessions from sub-session selection.

**Existing state:**
- `StatusStopped` is defined, persisted, and has a visual style
- Help text in `home.go:9907` says "attach (will auto-start)" for Enter key on a session
- The auto-start-on-attach path already exists in the session lifecycle

**Fix scope:**
- Ensure the main session list shows stopped sessions (verify no filtering at render time)
- Clarify help text specifically for stopped sessions (e.g., "Enter — resume stopped session")
- Consider allowing stopped sessions in the session picker for conductor scenarios (issue #307 asks for this)
- The `agent-deck session start <title>` CLI command should already work for stopped sessions; verify and document

**Stack requirement:** None. UI text and flow changes only.

**Confidence:** HIGH — the infrastructure exists; the fix is UX and filtering changes.

---

## Installation

No new packages required. All v1.3 features use existing dependencies.

```bash
# Verify no drift from expected deps:
go mod verify

# No new go get commands needed
```

---

## Alternatives Considered

| Feature | Alternative | Why Not |
|---------|-------------|---------|
| Mouse support | Upgrade to Bubble Tea v2 for new input API | v2 is alpha as of March 2026; v1.3.10 API is stable and sufficient |
| Mouse support | Migrate to `charmbracelet/bubbles/list` for built-in mouse | Home model uses a custom viewport; migration is a large rewrite, not a v1.3 fix |
| Session dedup | Add a new `conversation_id` SQLite column | Overcomplicated; `ClaudeSessionID` IS the conversation ID; dedup function already exists |
| TTY fix | Add `github.com/mattn/go-isatty` as a direct dep | Already a transitive dep; `golang.org/x/term` (already a direct dep) covers `IsTerminal()` |
| Sandbox persistence | New SQLite column for sandbox fields | Schema already supports it via `tool_data` JSON blob; adding columns requires a migration for no benefit |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `tea.WithMouseAllMotion()` | Sends a motion event for every mouse position change; floods the Update loop and interferes with text selection during tmux session attachment | Keep `tea.WithMouseCellMotion()` — fires only on cell boundary crossings |
| Deprecated `tea.MouseWheelUp` / `tea.MouseWheelDown` constants | Deprecated in Bubble Tea v1.3.x; linter will flag them | Use `tea.MouseButtonWheelUp` / `tea.MouseButtonWheelDown` with `msg.Button ==` check |
| New `conversation_id` SQLite schema column | Requires a migration, adds complexity, and duplicates `ClaudeSessionID` | Use existing `ClaudeSessionID` field and `UpdateClaudeSessionsWithDedup()` |

---

## Version Compatibility

| Package | Version | Mouse API Notes |
|---------|---------|-----------------|
| `charmbracelet/bubbletea` | v1.3.10 | `tea.MouseMsg`, `tea.MouseButtonWheelUp/Down`, `tea.MouseButtonLeft`, `msg.Action`, `msg.X`, `msg.Y` are all current and not deprecated |
| `charmbracelet/bubbletea` | v1.3.10 | Old `tea.MouseEventType` constants (`tea.MouseWheelUp`, etc.) are deprecated — do not use in new code |

---

## Sources

- `go.mod` — confirmed all current deps and versions (HIGH confidence)
- `/Users/ashesh/go/pkg/mod/github.com/charmbracelet/bubbletea@v1.3.10/mouse.go` — verified `MouseMsg`, `MouseButton`, `MouseButtonWheelUp/Down`, `MouseActionPress`, deprecation notice on `MouseEventType` (HIGH confidence)
- `internal/session/storage.go`, `internal/statedb/migrate.go` — confirmed sandbox already in `tool_data` schema (HIGH confidence)
- `internal/session/storage_test.go:260` — `TestStorageSaveWithGroups_PersistsSandboxConfig` confirms schema correct (HIGH confidence)
- `internal/session/instance.go:UpdateClaudeSessionsWithDedup()` — confirmed dedup mechanism and call sites (HIGH confidence)
- `cmd/agent-deck/main.go:468` — confirmed `tea.WithMouseCellMotion()` already enabled (HIGH confidence)
- `.github-issue-context-254.json` — confirmed zero `MouseMsg` handlers in codebase (HIGH confidence)
- `internal/ui/session_picker_dialog.go:41` — confirmed `StatusStopped` excluded from picker (HIGH confidence)
- `.planning/PROJECT.md` — confirmed v1.3 feature scope (HIGH confidence)

---

*Stack research for: agent-deck v1.3 Session Reliability & Resume*
*Researched: 2026-03-12*
