# Architecture

**Analysis Date:** 2026-03-11

## Pattern Overview

**Overall:** Layered TUI application with event-driven message passing

Agent-deck is a Go terminal session manager built on Bubble Tea (TUI framework). It follows a classic three-layer architecture:
1. **CLI entry point** (`cmd/agent-deck`) dispatches to subcommands or launches the TUI
2. **TUI layer** (`internal/ui/home.go`) implements the main Bubble Tea model with keyboard handling and rendering
3. **Data layer** (`internal/session`, `internal/statedb`, `internal/tmux`) manages sessions, persistence, and terminal abstractions

**Key Characteristics:**
- Message-driven updates through Bubble Tea's `Update()` method
- Thread-safe data access with `sync.RWMutex` protecting shared mutable state
- Zero-subprocess polling via tmux control-mode pipe (background activity tracking)
- SQLite WAL-mode persistence for concurrent multi-instance access
- Structured slog logging with component tags and ring buffer

## Layers

**CLI Layer:**
- Location: `cmd/agent-deck/`
- Contains: Subcommand handlers (session_cmd.go, mcp_cmd.go, group_cmd.go, etc.), argument parsing, profile resolution
- Depends on: All internal packages (session, tmux, ui, statedb, logging)
- Used by: System shell; invokes TUI or executes CLI subcommands
- Examples: `agent-deck add /path -t "Title"`, `agent-deck session start id`, `agent-deck mcp attach id mcp-name`

**TUI Layer:**
- Purpose: Interactive session management with keyboard controls and live status updates
- Location: `internal/ui/`
- Contains: `home.go` (8500+ lines, all UI logic), dialogs (newdialog.go, forkdialog.go, mcp_dialog.go), settings panel, styles
- Depends on: session, tmux, statedb, logging, update checking
- Used by: main.go when no CLI subcommand is given
- Responsibilities: Render session list, handle keypresses (add/remove/search/mcp), show live status updates

**Session Data Layer:**
- Purpose: Session model, status tracking, tool integration (Claude/Gemini/Codex/OpenCode)
- Location: `internal/session/`
- Contains: `instance.go` (session model with thread-safe accessors), `storage.go` (load/save from SQLite), `groups.go` (group hierarchy)
- Depends on: statedb, tmux, logging, docker, git
- Used by: UI, CLI, status update background worker
- Key types: `Instance` (single session), `Storage` (load/save logic), `GroupTree` (hierarchical groups)

**Persistence Layer:**
- Purpose: SQLite WAL-mode storage with auto-migration and concurrent access control
- Location: `internal/statedb/`
- Contains: SQLite schema, row structs, global DB singleton accessor
- Depends on: logging
- Used by: session.Storage for CRUD operations; UI/CLI for status acknowledgments
- Features: Primary/secondary instance election via distributed SQLite triggers, zero-CGO via modernc.org/sqlite

**tmux Abstraction Layer:**
- Purpose: Zero-subprocess terminal activity tracking via control-mode pipe
- Location: `internal/tmux/`
- Contains: `tmux.go` (session cache, subprocess batching), `pipemanager.go` (control-mode pipe), `detector.go` (Claude/Gemini/Codex session detection)
- Depends on: logging, platform detection
- Used by: Session status tracking, session creation/destruction
- Key: RefreshSessionCache() called once per 2s tick instead of O(n) lookups

**Supporting Layers:**
- `internal/logging`: Structured slog with component tags, ring buffer, file rotation
- `internal/platform`: Platform detection (macOS, Linux, WSL)
- `internal/profile`: Profile resolution (CLI -p flag, env, config default)
- `internal/clipboard`: Cross-platform clipboard access
- `internal/git`: Git worktree detection and integration
- `internal/send`: Message sending to Claude Code sessions
- `internal/docker`: Sandbox container support
- `internal/mcppool`: MCP connection pooling (HTTP + socket proxy)
- `internal/update`: Version checking and auto-update

## Data Flow

**Session Creation:**

1. User: `agent-deck add /path -t "Title"`
2. `cmd/agent-deck/main.go:handleAdd()` parses args
3. Calls `session.NewInstance()` which creates Instance struct with auto-generated ID
4. `session.Storage.Save()` inserts into `statedb.InstanceRow` (SQLite)
5. Instance appears in TUI list on next reload

**Session Startup and Status Detection:**

1. User selects session in TUI, presses Enter/Space
2. `home.go:Update(quitMsg)` calls `session.Instance.Start()`
3. `instance.Start()` creates tmux session via `tmux new-session -d`
4. Calls `tmux.Session.Start(command)` which begins the terminal process
5. Background worker runs `statusUpdateWorker()` at 2s intervals:
   - Calls `tmux.RefreshSessionCache()` (one subprocess: `tmux list-windows -a`)
   - Caches max activity timestamp per session to detect running/idle states
   - Calls `instance.UpdateStatus()` which probes actual Claude/Gemini session IDs from hook files
   - Reads hook-installed status files (`~/.codex/sessions/`, `~/.claude.json`, etc.)
6. UI re-renders with updated status (running/waiting/idle)

**MCP Attach/Detach:**

1. User selects MCP in dialog, presses Space
2. UI shows LOCAL vs GLOBAL scope prompt
3. `mcp_cmd.go:handleMCPAttach()` updates `.mcp.json` (LOCAL) or `config.toml` (GLOBAL)
4. User restarts session manually (`session restart`)
5. New tmux process loads fresh MCP config

**Multi-Instance Coordination (SQLite-based):**

1. First TUI instance calls `statedb.ElectPrimary()` which uses INSERT OR IGNORE on instance_election table
2. Only first instance succeeds (inserts its PID); others fail and see "already running"
3. All instances can read/write session data via SQLite (WAL mode allows concurrent access)
4. `StorageWatcher` (fsnotify) detects external state.db changes and reloads

**State Management:**

- Session list: `Home.instances` (protected by `Home.instancesMu`)
- Flat view for cursor: `Home.flatItems` (session.Item unions)
- Group tree: `Home.groupTree` (session.GroupTree, path-based hierarchy)
- UI cursor position: `Home.cursor` (index into flatItems)
- Viewport scroll: `Home.viewport` (Bubble Tea component)
- Dialogs/panels: visible/hidden flags on dialog structs

## Key Abstractions

**Session (Instance):**
- Purpose: Represents one agent/shell session in agent-deck
- Examples: `internal/session/instance.go`, `internal/tmux/tmux.go:Session`
- Pattern: Each session has unique ID (hex string), title, working directory, command template
- Thread-safe via `Instance.mu sync.RWMutex` protecting mutable status/tool fields
- Can be in sub-session relationship (ParentSessionID field)

**Status Enum:**
- Values: `running`, `waiting`, `idle`, `starting`, `stopped`, `error`
- Transition: `starting` -> `running` <-> `waiting` -> `idle`, or `error` if tmux session lost
- Detection: Hook files written by Claude Code contain status; background worker parses them

**Tool Integration:**
- Pattern: Each session tracks which tool (Claude, Gemini, Codex, OpenCode) it contains
- Detection: Hook files or JSONL prompt files indicate active tool
- Per-tool fields: `ClaudeSessionID`, `GeminiSessionID`, `CodexSessionID`, `OpenCodeSessionID`
- Per-tool data: `GeminiAnalytics`, `GeminiModel`, `GeminiYoloMode` (tool-specific options)

**GroupTree (Hierarchy):**
- Path-based: "projects/devops/backend" is a path, not nested structs
- Collapsed/expanded: Stored per group, affects flatItems rendering
- Order: Sessions within group maintain insertion order via `Order` field
- Default paths: Groups can suggest default working directory for new sessions

**Message Types (Bubble Tea):**
- All inherit from `tea.Msg` interface
- Custom types: `tickMsg`, `statusUpdateMsg`, `loadSessionsMsg`, `storageChangedMsg`, `quitMsg`
- Pattern: Each message type carries data needed for one update cycle
- Command functions return `tea.Cmd` which async-produce messages

## Entry Points

**TUI Entry Point:**
- Location: `cmd/agent-deck/main.go:main()`
- Triggers: `agent-deck` (no subcommand), or `agent-deck web`
- Responsibilities:
  - Parse CLI flags (-p/--profile, web args)
  - Check nested session prevention
  - Initialize logging, color profile, theme
  - Register instance in SQLite, elect primary if multi-instance allowed
  - Initialize Bubble Tea TUI and run event loop
  - Cleanup on shutdown (resign primary, unregister instance)

**CLI Entry Points (Subcommands):**
- `cmd/agent-deck/main.go:handleXXX()` functions dispatch to:
  - `handleAdd()`: session creation
  - `handleSession()`: dispatch to session subcommands (start/stop/attach/send/output)
  - `handleMCP()`: mcp attach/detach
  - `handleGroup()`: group operations
  - `handleConductor()`: multi-agent orchestration
  - `handleWorktree()`: git worktree support

**Hook Handlers (Background Services):**
- `cmd/agent-deck/hook_handler.go`: Receives Claude Code hook events (session started, status changed)
- `cmd/agent-deck/gemini_hooks_cmd.go`: Gemini CLI status updates
- `cmd/agent-deck/codex_hooks_cmd.go`: Codex CLI integration
- These update hook files that background worker reads

**Web API Entry Point:**
- Location: `internal/web/`
- Triggered by: `agent-deck web` subcommand
- Provides: REST API for external tools to query/control sessions

## Error Handling

**Strategy:** Graceful degradation; errors show in UI status, don't crash

**Patterns:**

- **Status errors:** If tmux session doesn't exist, mark instance as `error` status
- **Storage errors:** Log and retry; if SQLite is locked, wait with timeout
- **tmux subprocess errors:** Fall back to simpler detection; log timeouts
- **Hook file errors:** Silently skip malformed hook data; continue with previous status
- **Missing executables:** Check at startup (tmux, claude, gemini, codex) and show helpful errors

**Error Display:**

- TUI: Errors appear in status line (red background) and in modal confirmDialog
- CLI: Errors print to stderr with exit code 1
- Logs: All errors logged to `~/.agent-deck/logs/` with component tag and context

**Recovery Mechanisms:**

- Background worker loops retry status updates on error (no exponential backoff, just next tick)
- Storage retry: `statedb` uses `PRAGMA busy_timeout = 5s` for SQLite lock contention
- Instance state: Corrupted sessions are skipped; healthy ones continue operating

## Cross-Cutting Concerns

**Logging:**
- Implementation: `slog.Logger` with component context
- Components: CompStatus, CompUI, CompMCP, CompSession, CompStorage
- Format: JSON with aggregation, ring buffer (in-memory), file rotation
- Location: `~/.agent-deck/logs/debug.log` (only when AGENTDECK_DEBUG=1)

**Validation:**
- Session IDs: Hex string format (16 chars)
- Paths: Tilde expansion, symlink resolution
- Group paths: "/" separator, no empty segments
- Commands: Shell validation only (no pre-exec validation)

**Authentication:**
- No internal auth (relies on Claude Code's .claude.json for API keys)
- MCP configs stored in `config.toml` or `.mcp.json` with secrets in plaintext
- Profile system allows per-profile isolation (separate state.db files)

**Configuration:**
- Location: `~/.agent-deck/config.toml` (global), per-profile overrides in state.db
- Loaded at startup by `session.GetXxxConfig()` functions
- Changes detected via `fsnotify` on config.toml

**Performance:**
- tmux cache: Once per 2s tick instead of O(n) lookups
- Status updates: Debounced 2s per session to avoid parsing loops
- Log maintenance: 10s check interval, 5s minute full maintenance (async)
- Analytics cache: 5s TTL before refresh

---

*Architecture analysis: 2026-03-11*
