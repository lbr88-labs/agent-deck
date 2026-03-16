# Architecture Research: v1.3 Session Reliability and Resume

**Domain:** Go TUI terminal session manager — adding persistence, TTY, dedup, UX, and input fixes to an existing app
**Researched:** 2026-03-12
**Confidence:** HIGH (all findings from direct codebase reads)

## System Overview

```
┌────────────────────────────────────────────────────────────────┐
│                        CLI Layer                               │
│  cmd/agent-deck/main.go  |  session_cmd.go  |  hook_handler.go│
└───────────────────────────────┬────────────────────────────────┘
                                │
┌───────────────────────────────▼────────────────────────────────┐
│                        TUI Layer                               │
│  internal/ui/home.go (~8500 lines)                             │
│  ├── newdialog.go    (session creation form)                   │
│  ├── settings_panel.go  (user preferences)                     │
│  ├── mcp_dialog.go   (MCP attach/detach)                       │
│  └── styles.go       (theme, layout modes)                     │
└───────────────────────────────┬────────────────────────────────┘
                                │
┌───────────────────────────────▼────────────────────────────────┐
│                      Session Data Layer                        │
│  internal/session/instance.go  (Instance struct + Start/Stop)  │
│  internal/session/storage.go   (SQLite save/load pipeline)     │
│  internal/session/claude.go    (Claude session ID tracking)    │
│  internal/session/config.go    (profile + config access)       │
└─────────┬──────────────────────────────────┬───────────────────┘
          │                                  │
┌─────────▼──────────┐         ┌─────────────▼──────────────────┐
│  tmux Abstraction  │         │    Persistence Layer            │
│  internal/tmux/    │         │    internal/statedb/            │
│  ├── tmux.go       │         │    ├── statedb.go (schema)      │
│  ├── session.go    │         │    └── migrate.go (toolDataBlob)│
│  └── pipe_manager  │         └────────────────────────────────┘
└────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | v1.3 Change |
|-----------|----------------|-------------|
| `internal/statedb/migrate.go` | Defines `toolDataBlob` struct; `MarshalToolData`/`UnmarshalToolData` functions | Add sandbox fields to blob (Issue #320) |
| `internal/session/storage.go` | Converts `Instance` to `statedb.InstanceRow` for save; reverses on load | Verify sandbox round-trip is complete (Issue #320) |
| `internal/session/instance.go` | `Start()` builds and launches tmux session; `SandboxConfig` struct | `Start()` already reads `inst.Sandbox`; storage round-trip fix is in storage.go |
| `internal/session/claude.go` | `UpdateClaudeSessionsWithDedup()` clears duplicate `ClaudeSessionID` fields | Call this at resume-creation time, not only at save time (Issue #224) |
| `internal/ui/home.go` | `rebuildFlatItems()` filters session list; `Update()` handles all keyboard/mouse events | Show `StatusStopped` in list (Issue #307); add `tea.MouseMsg` handler (Issues #262, #254) |
| `internal/ui/settings_panel.go` | `buildToolLists()` builds radio group for default tool | Add `Icon` from `ToolDef` to display names (Issue #318) |
| `cmd/agent-deck/main.go` | Launches TUI with `tea.NewProgram`; platform-specific startup | Investigate TTY fd redirect on WSL/Linux (Issue #311) |
| `internal/tmux/tmux.go` | `Session.Start()` creates the tmux session process | TTY fix may require a startup flag or env var (Issue #311) |

## Recommended Project Structure

No new packages are needed for this milestone. All changes are targeted edits inside existing files.

```
internal/
├── statedb/
│   └── migrate.go          MODIFY: verify toolDataBlob includes sandbox fields
│
├── session/
│   ├── storage.go          MODIFY: verify Save/Load round-trips sandbox correctly
│   └── instance.go         MODIFY: call dedup at resume-creation site (#224)
│
├── ui/
│   ├── home.go             MODIFY: stopped session visibility + mouse events
│   └── settings_panel.go   MODIFY: custom tool icons in radio group

cmd/agent-deck/
└── main.go                 INVESTIGATE/MODIFY: WSL/Linux TTY launch path

docs/ (or skills/agent-deck/references/)
└── config-reference.md     MODIFY: document auto_cleanup field (#228)
```

## Architectural Patterns

### Pattern 1: toolDataBlob as the Single Serialization Contract (Issue #320)

**What:** All per-session extended fields (Claude session IDs, Gemini settings, sandbox config, SSH, etc.) are packed into a single JSON blob stored in the `tool_data` column of the SQLite `instances` table. The blob is defined by `toolDataBlob` in `internal/statedb/migrate.go`. Two functions at the `statedb` package boundary marshal and unmarshal it: `MarshalToolData` and `UnmarshalToolData`.

**Existing flow — confirmed working:**

```
Instance.Sandbox (*SandboxConfig)
    │
    ▼  [storage.go:SaveWithGroups]
json.Marshal(inst.Sandbox) → sandboxJSON (json.RawMessage)
    │
    ▼  [statedb.MarshalToolData]
toolDataBlob{Sandbox: sandboxJSON, SandboxContainer: "..."} → tool_data column in SQLite
    │
    ▼  [statedb.UnmarshalToolData]
sandboxJSON (json.RawMessage), sandboxContainer (string)
    │
    ▼  [storage.go:decodeSandboxConfig]
Instance.Sandbox = *SandboxConfig{Enabled: true, Image: "..."}
```

**The round-trip already exists.** The storage code reads and writes sandbox. Issue #320 is a bug in the dialog submission path: `newdialog.go` collects `sandboxEnabled bool` but the dialog's `Result()` struct (`newDialogResult`) needs to be verified that it carries the sandbox bool through to `createSessionInGroupWithWorktreeAndOptions`, which creates `session.NewSandboxConfig("")` when `sandboxEnabled == true`. The question is whether the `sandboxEnabled` bool reaches the save call when resuming (restarting) an existing session rather than creating a new one. The `Restart()` path must read from the persisted `inst.Sandbox` and must not clear it.

**When to use:** Any new per-session config field should go into `toolDataBlob` (not a new SQLite column). Add to the struct, `MarshalToolData`, `UnmarshalToolData`, and both call sites in `storage.go`.

**Trade-offs:** Single JSON blob means no SQL queries on individual fields, but since these fields are only accessed when loading the full session list, this is acceptable. Adding a field is three-file edit (migrate.go + storage.go twice). No schema migration needed.

### Pattern 2: Bubble Tea Message Flow for New Input Events (Issues #262, #254)

**What:** The TUI runs a single event loop in `home.go:Update(msg tea.Msg)`. All state changes happen as responses to messages. Mouse events arrive as `tea.MouseMsg` values. The program already registers for mouse events via `tea.WithMouseCellMotion()` in `cmd/agent-deck/main.go` at line 468, so events are already being delivered to `Update()` — they just have no handler.

**Adding wheel scroll — minimal change:**

```go
// Inside home.go:Update(), alongside the tea.KeyMsg case:
case tea.MouseMsg:
    switch msg.Button {
    case tea.MouseWheelUp:
        // reuse existing cursor-up logic
        return h, h.moveCursor(-3)
    case tea.MouseWheelDown:
        // reuse existing cursor-down logic
        return h, h.moveCursor(3)
    }
```

**The scroll amount (3 lines) is a UX judgment call.** Existing keyboard navigation moves 1 line per j/k, so 3 per scroll tick is a reasonable default.

**Adding click-to-select — requires coordinate mapping:**

```go
case tea.MouseMsg:
    if msg.Button == tea.MouseLeft && msg.Action == tea.MouseActionRelease {
        // msg.Y is the terminal row relative to the top of the program
        // Subtract the header height, then map to flatItems index + viewOffset
        listRow := msg.Y - h.listTopOffset // listTopOffset must be tracked during View()
        targetIdx := h.viewOffset + listRow
        if targetIdx >= 0 && targetIdx < len(h.flatItems) {
            h.cursor = targetIdx
        }
    }
```

`listTopOffset` must be computed during `View()` and stored as a field on `Home`. This is the only new field needed.

**Trade-offs:** Wheel scroll is trivial and low-risk. Click-to-select requires tracking where the list starts on screen during render, which creates a coupling between `View()` (render) and `Update()` (input handling). This is acceptable given Bubble Tea's architecture, but requires care.

### Pattern 3: rebuildFlatItems Predicate for Stopped Session Visibility (Issue #307)

**What:** `home.go:rebuildFlatItems()` calls `h.groupTree.Flatten()` which returns all sessions, then applies a `statusFilter` if one is active. There is no unconditional filter that hides stopped sessions — the issue is that several utility functions that derive "active" sessions (like `getOtherActiveSessions`) exclude `StatusStopped` alongside `StatusError`. This means stopped sessions may appear in the list but are excluded from conductor-style "send to all" actions. The original visibility bug is in these exclusion functions, not in `rebuildFlatItems` itself.

**Verified:** `rebuildFlatItems` does NOT unconditionally hide stopped sessions. The `StatusStopped` sessions will appear in the list if they exist in the group tree. However, the preview pane shows "Session Inactive" for both `StatusError` and `StatusStopped` rather than offering a "Resume" action. The UX fix is to differentiate the preview pane guidance for stopped vs error sessions:

```go
// home.go: renderPreview (or equivalent)
case selected.Status == session.StatusStopped:
    // Show "Press [r] to resume this session"
case selected.Status == session.StatusError:
    // Show "Session crashed — press [r] to restart"
```

No change to filtering is needed. The work is in the preview pane rendering.

### Pattern 4: Deduplication at Resume-Creation Time (Issue #224)

**What:** `UpdateClaudeSessionsWithDedup()` currently runs inside `SaveWithGroups()` in `storage.go`. This means duplicates are cleared when the full session list is persisted. The problem is the resume flow: when the user resumes a stopped Claude session, a new `Instance` is created with the same `ClaudeSessionID` as the stopped one. The duplicate exists in-memory between creation and the next `SaveWithGroups` call.

**The call chain for resume:**
```
User presses [r] on a stopped session
    → home.go handles 'r' key → calls Restart() on the stopped instance
    → instance.Restart() calls Start() with the existing ClaudeSessionID or -r flag
    → buildClaudeCommand reads inst.ClaudeSessionID
    → if two sessions now have the same ClaudeSessionID, conductor counts break
```

**Fix point:** In `home.go`, immediately before or after calling `inst.Restart()`, run `UpdateClaudeSessionsWithDedup(h.instances)` on the in-memory list. This mirrors what happens at save time, but applies it immediately at resume creation so the UI and any concurrent conductor operations see the deduplicated state.

Alternatively, `Restart()` itself can clear the `ClaudeSessionID` and let the session detection in the background worker pick up the new session ID — which is the more robust fix because it prevents the duplicate from ever forming.

### Pattern 5: TTY Fix for WSL/Linux Auto-Start (Issue #311)

**What:** The auto-start path launches the TUI without an interactive TTY in some WSL/Linux environments. Tools like Claude Code check `isatty(stdout)` and refuse to start, or behave differently, when stdout is not a terminal.

**Current launch path:**
```
tmux new-session -d -s agentdeck_... [optional: command]
```

For non-sandbox sessions, `Start()` creates a detached tmux session with no initial process, then sends the command via `SendKeysAndEnter`. The tmux pane already has a TTY. The issue is not in the tmux session itself but in how the `agent-deck` process is invoked at startup on WSL/Linux.

**Investigation target:** `cmd/agent-deck/main.go` — when launched without a terminal (e.g., as a systemd service or via `nohup`), the program must explicitly allocate a PTY or refuse to start cleanly. The fix likely involves:

1. Checking `isatty(os.Stdout.Fd())` at startup and either erroring out or re-execing with a PTY allocator.
2. Or ensuring the auto-start wrapper (whatever invokes agent-deck on WSL/Linux startup) uses `setsid` / `script -c` to provide a controlling TTY.

The `mattn/go-isatty` package is already in `go.sum` (line 54), so the import is available.

## Data Flow

### Sandbox Config Persistence Round-Trip

```
newdialog.go                 home.go                  storage.go              statedb/migrate.go
────────────                 ───────                  ──────────              ──────────────────
sandboxEnabled: bool  →  createSession(sandboxEnabled: bool)
                              │
                              └→ session.NewSandboxConfig("")
                                    inst.Sandbox = &SandboxConfig{Enabled: true, Image: "..."}
                                        │
                                        └→ SaveWithGroups(instances, groupTree)
                                                │
                                                └→ json.Marshal(inst.Sandbox) → sandboxJSON
                                                      │
                                                      └→ MarshalToolData(..., sandboxJSON, ...)
                                                            │
                                                            └→ tool_data JSON column in SQLite

On reload:
SQLite → UnmarshalToolData → sandboxJSON → decodeSandboxConfig → inst.Sandbox
```

**Gap to verify:** Does `Restart()` preserve `inst.Sandbox`? The `Restart()` function in `instance.go` calls `Start()` which calls `prepareCommand()` which calls `wrapForSandbox()`. `wrapForSandbox()` reads `inst.Sandbox` — so if `inst.Sandbox` is correctly loaded from SQLite, `Restart()` will re-use it. The bug in #320 is likely that `inst.Sandbox` is nil after reload, which means the SQLite round-trip is broken somewhere in the load path.

### Mouse Event Flow

```
Terminal input (scroll/click)
    → tea.Program (mouse events enabled via WithMouseCellMotion)
    → home.go:Update(tea.MouseMsg{Button: MouseWheelDown, Y: 12})
    → [NEW] case tea.MouseMsg: h.moveCursor(+3) or h.cursor = computeTargetIdx(msg.Y)
    → home.go:View() re-renders with new cursor position
```

### Deduplication at Resume

```
User presses [r] on stopped session
    → home.go:handleKeyRestart()
    → inst.Restart()                          [this calls Start() with existing ClaudeSessionID]
    → [FIX] UpdateClaudeSessionsWithDedup(h.instances)   [clear the now-duplicate ID from other sessions]
    → h.saveToStorage()                       [already calls dedup in SaveWithGroups, but in-memory dedup is needed first]
```

### Settings Custom Tools

```
config.toml [tools] section → session.UserConfig.Tools map[string]ToolDef
    → settings_panel.go:buildToolLists(config)
        → for name, def := range config.Tools {
              display := def.Icon + " " + Title(name)  // [FIX] add Icon here
              names = append(names, display)
              values = append(values, name)
          }
    → s.toolNames, s.toolValues
    → renderRadioGroup(s.toolNames, s.selectedTool, ...)
```

## Integration Points

### New vs Modified: Per Feature

| Feature | Touch Points | New vs Modified |
|---------|--------------|-----------------|
| Sandbox persistence (#320) | `statedb/migrate.go`, `session/storage.go`, `session/instance.go` | MODIFIED only — verify round-trip |
| Auto-start TTY (#311) | `cmd/agent-deck/main.go`, potentially `internal/tmux/tmux.go` | MODIFIED — add isatty check or re-exec |
| Session dedup on resume (#224) | `session/instance.go` (`Restart()` or `home.go` restart handler) | MODIFIED — add dedup call at resume site |
| Stopped sessions in TUI (#307) | `internal/ui/home.go` (preview pane rendering) | MODIFIED — differentiate stopped vs error guidance |
| Settings custom tools (#318) | `internal/ui/settings_panel.go` (`buildToolLists`) | MODIFIED — add Icon field to display name |
| Mouse scroll (#262, #254) | `internal/ui/home.go` (`Update()`) | MODIFIED — add `tea.MouseMsg` case |
| auto_cleanup docs (#228) | `docs/` or `skills/agent-deck/references/config-reference.md` | MODIFIED — text-only change |

### Internal Boundaries

| Boundary | Communication | v1.3 Notes |
|----------|---------------|------------|
| `storage.go` → `statedb/migrate.go` | `MarshalToolData`/`UnmarshalToolData` function calls | Sandbox already in signature; verify nil-safety in `decodeSandboxConfig` |
| `home.go` → `session/instance.go` | Direct struct field reads + method calls | `Restart()` must not clear `inst.Sandbox`; `UpdateClaudeSessionsWithDedup` must be callable from `home.go` |
| `cmd/agent-deck/main.go` → `internal/platform` | `platform.IsWSL()`, `platform.IsLinux()` | TTY check should branch on platform for targeted fix |
| `tea.Program` → `home.go:Update()` | `tea.Msg` dispatch | `tea.MouseMsg` already arrives; needs a handler case |

## Build Order

The features are independent except for one dependency chain:

```
Phase A (foundation, no deps):
  1. Sandbox persistence (#320)     — storage layer, self-contained
  2. auto_cleanup docs (#228)       — text-only, zero risk

Phase B (depends on stopped sessions being visible):
  3. Stopped sessions in TUI (#307) — prerequisite for manual resume testing

Phase C (depends on stopped sessions being visible for validation):
  4. Session dedup on resume (#224) — confirmed by seeing both sessions in TUI

Phase D (independent, can run in parallel with Phase B/C):
  5. Settings custom tools (#318)   — settings_panel.go, fully isolated
  6. Mouse scroll (#262)            — home.go Update(), low risk
  7. Mouse click (#254)             — builds on scroll, needs listTopOffset tracking

Phase E (requires investigation before work can start):
  8. Auto-start TTY fix (#311)      — platform-specific, needs root cause confirmation
```

**Rationale:**

- #320 first: If sandbox persistence is confirmed broken at load time, it means `inst.Sandbox` is nil after reload — which also affects the restart path. Fix this before touching restart.
- #307 before #224: You cannot confirm dedup is working if stopped sessions are not visible to compare against.
- #228 any time: Doc change, no coordination needed.
- #318 any time: Settings panel change, no coordination with other features.
- #262 before #254: Scroll is simpler (no hit-testing). Confirm mouse events are handled before adding coordinate-dependent click logic.
- #311 last: Needs investigation on a WSL/Linux environment. The fix location is uncertain until the root cause is confirmed. Doing it last prevents it from blocking other work.

## Anti-Patterns

### Anti-Pattern 1: Adding a New SQLite Column for Sandbox Config

**What people do:** When a field is not persisting correctly, add a new column to the schema.
**Why it's wrong:** The `toolDataBlob` design intentionally packs all extended fields into one JSON blob to avoid schema migrations. The sandbox fields are already in `toolDataBlob`. Adding a column creates redundancy and requires a migration version bump.
**Do this instead:** Verify the round-trip through `toolDataBlob`. The fix is in logic, not schema.

### Anti-Pattern 2: Filtering StatusStopped Out of rebuildFlatItems

**What people do:** "Stopped sessions are like error sessions — filter them from the main list."
**Why it's wrong:** Stopped sessions are intentional stops. Hiding them removes the user's ability to resume them. `StatusError` (crashed) should also arguably remain visible.
**Do this instead:** Keep both statuses in the list. Differentiate them visually and in the preview pane. The existing icons already differ ("■" for stopped, "✕" for error).

### Anti-Pattern 3: Running Dedup Only at Save Time

**What people do:** "SaveWithGroups already calls dedup — we're covered."
**Why it's wrong:** Conductors query `h.instances` in-memory between user action and the next save. If two sessions share a `ClaudeSessionID` between a `Restart()` call and the next `SaveWithGroups`, the conductor sees a duplicate and may route the wrong session.
**Do this instead:** Run `UpdateClaudeSessionsWithDedup` in-memory immediately when a session is resumed, not only at persist time.

### Anti-Pattern 4: Setting tea.WithMouseAllMotion Instead of tea.WithMouseCellMotion

**What people do:** Enable all mouse events to get more coverage.
**Why it's wrong:** `WithMouseAllMotion` sends a mouse event for every cursor position, flooding the event loop with messages on every mouse movement. This degrades TUI performance noticeably.
**Do this instead:** `WithMouseCellMotion()` is already active — it sends events only on cell boundary changes, which is sufficient for scroll and click detection.

### Anti-Pattern 5: Hardcoding listTopOffset

**What people do:** `const listTopOffset = 3` because the header "looks like 3 lines."
**Why it's wrong:** Header height depends on layout mode (Single/Stacked/Dual), update banner visibility, filter bar, and other conditional elements. A hardcoded offset produces wrong click targets.
**Do this instead:** Compute and store `listTopOffset` during `View()` by counting lines as they are appended to the output buffer. Store it as `h.listTopOffset int` so `Update()` can use it for hit-testing.

## Sources

- `internal/session/storage.go` — `SaveWithGroups`, `decodeSandboxConfig`, `convertToInstances` (HIGH confidence, direct read)
- `internal/statedb/migrate.go` — `toolDataBlob`, `MarshalToolData`, `UnmarshalToolData` (HIGH confidence, direct read)
- `internal/session/instance.go` — `Instance`, `SandboxConfig`, `Start()`, `UpdateClaudeSessionsWithDedup()`, `wrapIgnoreSuspend()` (HIGH confidence, direct read)
- `internal/ui/home.go` — `rebuildFlatItems()`, `Update()`, `createSessionInGroupWithWorktreeAndOptions()`, `getOtherActiveSessions()` (HIGH confidence, direct read)
- `internal/ui/settings_panel.go` — `buildToolLists()`, `SettingDefaultTool` (HIGH confidence, direct read)
- `internal/ui/newdialog.go` — `sandboxEnabled` field, `newDialogResult` struct (HIGH confidence, direct read)
- `cmd/agent-deck/main.go` line 468 — `tea.WithMouseCellMotion()` already active (HIGH confidence, direct read)
- `.planning/codebase/ARCHITECTURE.md` — existing system architecture map (HIGH confidence, project documentation)
- `.planning/research/FEATURES.md` — feature complexity and integration notes from prior research (HIGH confidence, already written for this milestone)
- `go.sum` line 54 — `mattn/go-isatty v0.0.20` already in dependency tree (HIGH confidence, direct read)

---
*Architecture research for: Agent-Deck v1.3 Session Reliability and Resume*
*Researched: 2026-03-12*
