# Codebase Structure

**Analysis Date:** 2026-03-11

## Directory Layout

```
/Users/ashesh/claude-deck/
├── cmd/agent-deck/              # CLI entry point and subcommand handlers
│   ├── main.go                  # Global CLI dispatch, TUI launch, version
│   ├── session_cmd.go           # session start/stop/attach/send/output/fork
│   ├── mcp_cmd.go               # mcp attach/detach/list/reload
│   ├── group_cmd.go             # group create/delete/move/reorder
│   ├── conductor_cmd.go         # Multi-agent orchestration commands
│   ├── worktree_cmd.go          # Git worktree integration
│   ├── launch_cmd.go            # Session creation via UI
│   ├── hook_handler.go          # Claude Code hook event handler
│   ├── gemini_hooks_cmd.go      # Gemini CLI status updates
│   ├── codex_hooks_cmd.go       # Codex CLI integration
│   ├── openclaw_cmd.go          # OpenCode CLI integration
│   ├── remote_cmd.go            # SSH remote session support
│   ├── skill_cmd.go             # Skill management
│   ├── web_cmd.go               # REST API server
│   ├── mcp_proxy_cmd.go         # MCP socket proxy
│   ├── notify_daemon_cmd.go     # System notifications daemon
│   ├── try_cmd.go               # Experimental features
│   ├── gemini_yolo.go           # Gemini YOLO mode settings
│   ├── cli_utils.go             # Shared CLI helpers (table formatting, JSON output)
│   ├── testmain_test.go         # Test profile isolation (CRITICAL: never delete)
│   └── *_test.go                # Unit tests for each handler
│
├── internal/
│
│   ├── ui/                      # Bubble Tea TUI implementation
│   │   ├── home.go              # Main model (8500+ lines): all UI logic, message handling, rendering
│   │   ├── styles.go            # Tokyo Night theme, responsive layout constants
│   │   ├── newdialog.go         # Session creation form
│   │   ├── forkdialog.go        # Claude session forking dialog
│   │   ├── mcp_dialog.go        # MCP attach/detach LOCAL vs GLOBAL scope
│   │   ├── settings_panel.go    # User preferences (theme, hotkeys, MCP)
│   │   ├── global_search.go     # Cross-conversation search via ~/.claude/projects/
│   │   ├── confirm_dialog.go    # Yes/No confirmations
│   │   ├── gemini_model_dialog.go # Gemini model selection
│   │   ├── wizard.go            # Setup wizard for first run
│   │   ├── testmain_test.go     # Test profile isolation
│   │   └── *_test.go            # Tests for UI components
│   │   └── tmp/                 # Temporary files (generated fixtures)
│   │
│   ├── session/                 # Session model and business logic
│   │   ├── instance.go          # Instance struct (240+ fields), thread-safe accessors
│   │   ├── storage.go           # Load/save sessions from SQLite
│   │   ├── groups.go            # Group hierarchy (path-based tree)
│   │   ├── conductor.go         # Multi-agent orchestration logic
│   │   ├── config.go            # Per-profile configuration loading
│   │   ├── theme.go             # Theme resolution (system/dark/light)
│   │   ├── update_settings.go   # Update checking preferences
│   │   ├── claude.go            # Claude Code integration (session ID detection)
│   │   ├── gemini.go            # Gemini CLI integration (YOLO mode, analytics)
│   │   ├── codex.go             # Codex CLI integration (rotation, probe scanning)
│   │   ├── global_search.go     # Search index across ~/.claude/projects/
│   │   ├── testmain_test.go     # Test profile isolation
│   │   └── *_test.go            # Tests
│   │
│   ├── statedb/                 # SQLite persistence layer
│   │   ├── statedb.go           # StateDB wrapper, schema, CRUD ops
│   │   ├── migrations.go        # Auto-migration system (SchemaVersion = 2)
│   │   ├── testmain_test.go     # Test profile isolation
│   │   └── *_test.go            # Tests
│   │
│   ├── tmux/                    # tmux abstraction and session cache
│   │   ├── tmux.go              # RefreshSessionCache(), session existence checks, subprocess batching
│   │   ├── pipemanager.go       # Control-mode pipe for zero-subprocess polling
│   │   ├── controlpipe.go       # Low-level pipe protocol
│   │   ├── detector.go          # Claude/Gemini/Codex session ID detection from hook files
│   │   ├── patterns.go          # Regex patterns for status detection
│   │   ├── title_detection.go   # tmux window title parsing
│   │   ├── pty.go               # PTY utilities
│   │   ├── status_fixes.go      # Status transition corrections
│   │   ├── testmain_test.go     # Test profile isolation
│   │   └── *_test.go            # Tests
│   │
│   ├── logging/                 # Structured slog with ring buffer
│   │   ├── logger.go            # Global slog.Logger, lumberjack rotation
│   │   ├── aggregator.go        # Log aggregation (30s flush)
│   │   ├── ringbuffer.go        # In-memory circular buffer (10MB default)
│   │   ├── compat.go            # Backward compatibility (old JSON format)
│   │   ├── pprof.go             # pprof server (localhost:6060)
│   │   └── *_test.go            # Tests
│   │
│   ├── profile/                 # Profile detection and resolution
│   │   └── profile.go           # Profile lookup: CLI -p > env AGENTDECK_PROFILE > config default > "default"
│   │
│   ├── platform/                # OS detection
│   │   └── platform.go          # macOS/Linux/WSL detection
│   │
│   ├── git/                     # Git integration
│   │   ├── git.go               # Git worktree detection, branch info
│   │   └── *_test.go            # Tests
│   │
│   ├── clipboard/               # Cross-platform clipboard
│   │   ├── clipboard.go         # Copy to clipboard (macOS: pbcopy, Linux: xclip)
│   │   └── *_test.go            # Tests
│   │
│   ├── send/                    # Message sending to Claude sessions
│   │   └── send.go              # Send message via Claude Code hook
│   │
│   ├── docker/                  # Docker sandbox support
│   │   └── docker.go            # Container creation/cleanup
│   │
│   ├── mcppool/                 # MCP connection pooling
│   │   ├── pool.go              # Connection pool (HTTP + socket proxy)
│   │   └── *_test.go            # Tests
│   │
│   ├── update/                  # Version checking and auto-update
│   │   ├── update.go            # Check for updates, download, perform update
│   │   └── *_test.go            # Tests
│   │
│   ├── web/                     # REST API server
│   │   ├── server.go            # HTTP server setup
│   │   ├── handlers.go          # API endpoints
│   │   └── static/              # Static assets (UI)
│   │
│   ├── openclaw/                # OpenCode CLI integration (experimental)
│   │   └── openclaw.go          # OpenCode session coordination
│   │
│   ├── integration/             # External integrations
│   │   └── integration.go       # Integration utilities
│   │
│   ├── testutil/                # Test utilities
│   │   ├── testutil.go          # Test helpers (temp dirs, mocks)
│   │   └── *_test.go            # Tests
│   │
│   └── experiments/             # Experimental features
│       └── *_test.go            # Experimental tests
│
├── tests/                       # End-to-end tests
│   └── e2e/                     # E2E test scenarios
│
├── docs/                        # Documentation (git-ignored)
│   └── plans/                   # Planning docs
│
├── conductor/                   # Multi-agent conductor service
│   ├── bridge.py                # Python bridge for external communication
│   ├── setup.sh / teardown.sh   # Service lifecycle
│   ├── conductor-claude.md      # Conductor Claude config
│   └── HEARTBEAT_RULES.md       # Session heartbeat protocol
│
├── .planning/                   # GSD planning (generated, not committed)
│   └── codebase/               # Architecture/structure analysis
│
├── go.mod / go.sum             # Go module dependencies
├── Makefile                     # Build targets (build, run, test, lint, ci)
├── Dockerfile                   # Container image
├── .goreleaser.yaml            # GoReleaser config (GitHub releases, Homebrew)
├── lefthook.yaml               # Git hooks (gofmt, go vet, golangci-lint)
├── .air.toml                   # Air config (hot-reload for development)
└── CLAUDE.md                   # Project-specific Claude instructions
```

## Directory Purposes

**`cmd/agent-deck/`:**
- Purpose: CLI entry point and subcommand routing
- Contains: main.go dispatches to handlers, each handler implements one command (add, session, mcp, etc.)
- Key files: `main.go` (183+ lines), `session_cmd.go`, `mcp_cmd.go`, `group_cmd.go`
- Pattern: `handleXXX(profile string, args []string)` functions parse args and delegate to internal packages

**`internal/ui/`:**
- Purpose: Interactive TUI using Bubble Tea framework
- Contains: home.go is the main Bubble Tea model (8500+ lines); dialogs, settings panel, search
- Key invariant: Single-threaded Bubble Tea event loop with background worker for status updates
- Protected by: `Home.instancesMu` for concurrent background access to session list

**`internal/session/`:**
- Purpose: Session model and business logic (not persistence)
- Contains: Instance struct (session definition), Storage (load/save), GroupTree (hierarchy)
- Key types: Instance (single session), Storage (SQLite bridge), GroupTree (path-based)
- Pattern: Instance has thread-safe Get/Set methods (GetStatus, SetStatus, GetTool, SetTool)

**`internal/statedb/`:**
- Purpose: SQLite WAL-mode persistence with zero CGO via modernc.org/sqlite
- Contains: StateDB wrapper, schema definition, migrations, CRUD operations
- Key feature: Global singleton accessor (`SetGlobal`, `GetGlobal`) for cross-package access
- Thread safety: WAL mode allows concurrent read/write from multiple processes

**`internal/tmux/`:**
- Purpose: Minimal tmux abstraction, zero-subprocess polling via control-mode pipe
- Contains: Session cache (one per 2s tick), pipe manager, status detection, patterns
- Key optimization: RefreshSessionCache() calls `tmux list-windows -a` once, not per-session
- Fallback: If pipe fails, uses subprocess; logs the fallback for verification

**`internal/logging/`:**
- Purpose: Structured slog with component tags, ring buffer (10MB in-memory), file rotation
- Contains: Logger setup, aggregation (30s flush), ring buffer, pprof server
- Location: `~/.agent-deck/logs/debug.log` (only when AGENTDECK_DEBUG=1)
- Format: JSON with lumberjack rotation (10MB files, 5 backups)

**`internal/profile/`:**
- Purpose: Profile resolution (per-user isolated session storage)
- Priority: CLI -p flag > env AGENTDECK_PROFILE > config default > "default"
- Isolation: Each profile has separate state.db file in `~/.agent-deck/profiles/{profile}/`

**`internal/platform/`:**
- Purpose: OS detection for platform-specific behavior
- Detects: macOS, Linux, WSL (via /proc/version)
- Used by: Clipboard, shell detection, path handling

**`tests/e2e/`:**
- Purpose: End-to-end integration tests
- Requires: Running tmux server, must set AGENTDECK_PROFILE=_test

## Key File Locations

**Entry Points:**
- `cmd/agent-deck/main.go`: CLI dispatch, TUI launch, nested-session prevention
- `internal/ui/home.go`: Bubble Tea model, all TUI logic
- `cmd/agent-deck/session_cmd.go`: session subcommand routing
- `cmd/agent-deck/mcp_cmd.go`: mcp subcommand routing

**Configuration:**
- `~/.agent-deck/config.toml`: Global agent-deck config (MCPs, theme, hotkeys)
- `.mcp.json`: Per-directory local MCP overrides
- `~/.claude/claude.json`: Claude Code's session/API config (read-only for session ID detection)
- Per-profile state: `~/.agent-deck/profiles/{profile}/state.db`

**Core Logic:**
- `internal/session/instance.go`: Session model with 240+ fields, status lifecycle
- `internal/session/storage.go`: Load/save sessions from SQLite
- `internal/ui/home.go`: All UI rendering, keyboard handling, message dispatch
- `internal/tmux/tmux.go`: Session cache, activity tracking
- `internal/statedb/statedb.go`: SQLite schema, CRUD, migrations

**Testing:**
- `cmd/agent-deck/testmain_test.go`: Profile isolation to AGENTDECK_PROFILE=_test (CRITICAL)
- `internal/session/testmain_test.go`: Session package test profile isolation
- `internal/ui/testmain_test.go`: UI tests
- `internal/tmux/testmain_test.go`: tmux tests
- `tests/e2e/`: End-to-end scenarios
- Pattern: All test packages use TestMain to prevent session data corruption

## Naming Conventions

**Files:**
- `*_cmd.go`: CLI subcommand handler (session_cmd.go, mcp_cmd.go, group_cmd.go)
- `*_hooks_cmd.go`: Hook event handler (gemini_hooks_cmd.go, codex_hooks_cmd.go)
- `*_test.go`: Unit tests for same-named file
- `testmain_test.go`: Profile isolation for all tests in package
- `*_dialog.go`: Bubble Tea dialog component
- No acronyms in filenames (except _test, _cmd, _hooks)

**Directories:**
- Lowercase with underscores (session, statedb, ui, mcppool, openclaw)
- Internal packages use `internal/` prefix
- No abbreviations (not `ses`, not `db`, not `tmx`)

**Types:**
- PascalCase: Home, Instance, Storage, StateDB, Session
- Private: lowercase first letter (h, s, st)
- Interface: NamedXxx pattern (e.g., Msg from tea.Msg)

**Functions:**
- PascalCase: NewStorage(), UpdateStatus(), RefreshSessionCache()
- Methods: (r *Receiver) MethodName() pattern
- Private: lowercase (r.privateField)
- Getters: Get{Field}() pattern; no Is/Has prefix

**Variables:**
- camelCase: currentSession, sessionCache, isRunning
- Constants: UPPERCASE_WITH_UNDERSCORES: Version = "0.25.1"
- Package-level: var statusLog, var sessionCacheMu
- Mutex convention: fieldMu for field protection

## Where to Add New Code

**New Feature (e.g., SSH support):**
- Primary code: `cmd/agent-deck/remote_cmd.go` (already exists)
- Model extension: `internal/session/instance.go` (add fields)
- CLI handler: Add case to switch in `cmd/agent-deck/main.go`
- Tests: `cmd/agent-deck/remote_cmd_test.go`
- Documentation: Update CLAUDE.md or --help text

**New Component/Dialog:**
- Implementation: `internal/ui/{component_name}_dialog.go`
- Integration: Import in `internal/ui/home.go`, add field to Home struct
- Styling: Use constants from `internal/ui/styles.go`
- Tests: `internal/ui/{component_name}_dialog_test.go`

**New Session Data Field:**
- Add to: `internal/session/instance.go` (Instance struct)
- Persist: `internal/statedb/statedb.go` (InstanceRow struct, migrations)
- Load: `internal/session/storage.go` (Save/Load methods)
- Migrate: Update SchemaVersion in statedb.go, add migration function

**New Tool Integration (Claude/Gemini/Codex):**
- Detection: `internal/tmux/detector.go` (add regex pattern)
- Status update: `internal/session/instance.go` (UpdateStatus method)
- Per-tool data: `internal/session/{tool_name}.go` (existing: claude.go, gemini.go, codex.go)
- Hook handler: `cmd/agent-deck/{tool_name}_hooks_cmd.go`

**Utilities (Shared Helpers):**
- String/formatting: `cmd/agent-deck/cli_utils.go`
- Session helpers: `internal/session/config.go` or new `internal/session/utils.go`
- UI helpers: `internal/ui/styles.go` or new `internal/ui/utils.go`
- tmux helpers: `internal/tmux/patterns.go` or new `internal/tmux/utils.go`

**Tests:**
- Unit tests: Alongside source file (same package)
- E2E tests: `tests/e2e/` directory
- Fixtures: `internal/testutil/` for shared test utilities
- Mock sessions: Use `statedb.SetProfile("_test")` to isolate from production

## Special Directories

**`~/.agent-deck/`:**
- Purpose: Runtime state and configuration
- Generated: Yes (created on first run)
- Committed: No (gitignored)
- Contents:
  - `config.toml`: Global config, MCPs, hotkeys
  - `profiles/{profile}/state.db`: SQLite session storage
  - `logs/debug.log`: Structured logs (only when AGENTDECK_DEBUG=1)

**`conductor/`:**
- Purpose: Multi-agent orchestration service (experimental)
- Contains: Python bridge, setup/teardown scripts, Claude config
- Committed: Yes (part of codebase for documentation)
- Status: Separate runtime service, not built by agent-deck CLI

**`tests/e2e/`:**
- Purpose: Integration tests requiring running tmux
- Requires: AGENTDECK_PROFILE=_test, tmux server running
- Pattern: Skip with skipIfNoTmuxServer(t) if tmux not available

**`docs/`:**
- Purpose: Project documentation (git-ignored via .git/info/exclude)
- Contains: Planning docs, design notes, handover materials
- Not committed: Personal notes, sensitive data

**`.planning/codebase/`:**
- Purpose: GSD codebase analysis (generated by /gsd:map-codebase)
- Generated: Yes (by agent mapper)
- Committed: No (temporary analysis)
- Contains: ARCHITECTURE.md, STRUCTURE.md, CONVENTIONS.md, TESTING.md, CONCERNS.md

---

*Structure analysis: 2026-03-11*
