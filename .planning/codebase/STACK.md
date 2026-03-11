# Technology Stack

**Analysis Date:** 2026-03-11

## Languages

**Primary:**
- Go 1.24.0 - Terminal application, CLI, web backend, all core logic

**Secondary:**
- JavaScript/TypeScript - Web frontend in `internal/web/static/` (PWA + service worker)
- Shell/Bash - Install scripts, tmux integration, shell command execution

## Runtime

**Environment:**
- Go 1.24.0 (specified in `go.mod`)
- Linux/Darwin (macOS) supported architectures: amd64, arm64 (via GoReleaser)

**Package Manager:**
- Go modules (go.mod, go.sum)
- Lockfile: `go.sum` present

## Frameworks

**Core UI/TUI:**
- Charmbracelet Bubble Tea 1.3.10 - Terminal UI framework
- Charmbracelet Bubbles 0.21.0 - Pre-built TUI components (textarea, textinput, viewport)
- Charmbracelet Lipgloss 1.1.0 - Styling, layout for TUI

**Web/HTTP:**
- Go stdlib `net/http` - HTTP server, handlers, middleware
- Gorilla WebSocket 1.5.3 - Real-time session output via WebSocket (handlers in `internal/web/handlers_ws.go`)

**Database:**
- SQLite via `modernc.org/sqlite` 1.44.3 - Pure Go SQLite driver (CGO-free, WAL mode enabled)
  - Query builder: None (direct SQL in `internal/statedb/statedb.go`)
  - ORM: None (manual schema management in `internal/statedb/migrate.go`)

**Testing:**
- Testify 1.11.1 - Assertions and test utilities
- Go stdlib `testing` - Test framework

**Build/Dev:**
- GoReleaser 2 (`.goreleaser.yml`) - Multi-platform binary releases (darwin/linux, amd64/arm64)
- Lefthook - Git pre-commit/pre-push hooks
- Air - Auto-reload dev mode (`make dev`)
- Golangci-lint - Linting (parallel checks for performance)

## Key Dependencies

**Critical (external integrations):**
- `github.com/gorilla/websocket` 1.5.3 - WebSocket for real-time session data to web UI
- `github.com/SherClockHolmes/webpush-go` 1.4.0 - Web Push Protocol (VAPID keypair generation, push notifications)
- `github.com/creack/pty` 1.1.24 - PTY allocation for terminal emulation in containers
- `modernc.org/sqlite` 1.44.3 - Session persistence in SQLite (no CGO dependency)
- `github.com/BurntSushi/toml` 1.5.0 - Configuration file parsing (`.toml` format support)
- `github.com/google/uuid` 1.6.0 - UUID generation for session IDs

**Terminal/Platform:**
- `golang.org/x/term` 0.37.0 - Terminal feature detection (raw mode, window size)
- `golang.org/x/time/rate` 0.14.0 - Rate limiting for API calls
- `github.com/thiagokokada/dark-mode-go` 0.0.2 - Detect macOS dark mode for TUI theming
- `github.com/muesli/termenv` 0.16.0 - Terminal capability detection and color profiles
- `github.com/mattn/go-runewidth` 0.0.16 - Unicode rune width calculation for TUI layout
- `github.com/charmbracelet/x/ansi` 0.10.1 - ANSI code parsing and manipulation

**Utilities:**
- `github.com/fsnotify/fsnotify` 1.9.0 - File system notifications (watch `.mcp.json` changes)
- `github.com/sahilm/fuzzy` 0.1.1 - Fuzzy search in session list
- `golang.org/x/sync/errgroup` 0.19.0 - Concurrent error handling
- `golang.org/x/sync/singleflight` - Request deduplication
- `gopkg.in/natefinch/lumberjack.v2` 2.2.1 - Log file rotation

**Transitive (included via Bubble Tea):**
- `github.com/atotto/clipboard` 0.1.4 - Clipboard integration (optional, for copy/paste in TUI)

## Configuration

**Environment:**
- `AGENTDECK_PROFILE` - Profile selection (overridable via `-p` flag)
- `CLAUDE_CONFIG_DIR` - Claude Code configuration directory location
- `AGENTDECK_DEBUG` - Enable debug logging (`AGENTDECK_DEBUG=1`)
- `AGENTDECK_COLOR` - Terminal color mode (options: `truecolor`, `256`, `16`, `none`)
- `AGENTDECK_SANDBOX` - Enable Docker sandbox mode (experimental)

**Key configs required:**
- `~/.agent-deck/config.json` - Global config (default profile, version tracking)
- `~/.agent-deck/profiles/{profile}/state.db` - SQLite database per profile
- `~/.agent-deck/profiles/{profile}/.mcp.json` - Local MCP definitions (optional)
- `~/.agent-deck/profiles/{profile}/web_push_vapid_keys.json` - VAPID keypair for web push (auto-generated)
- `~/.agent-deck/logs/` - Logs directory

**Build:**
- `.goreleaser.yml` - Release configuration (multi-platform builds, GitHub releases, Homebrew tap)
- `Makefile` - Build targets, version injection via ldflags
- `lefthook.yml` - Pre-push hook configuration (lint + test in parallel)
- `.air.toml` - Auto-reload dev mode (if present)

## Platform Requirements

**Development:**
- Go 1.24+
- tmux (terminal session manager, required for all sessions)
- jq (optional, for JSON filtering in scripts)
- Make (for build targets)
- Optional: golangci-lint, lefthook, goreleaser, air

**Production:**
- Linux (x86_64, ARM64) or macOS (x86_64, ARM64)
- tmux (hard dependency)
- Docker (optional, for sandbox mode via `docker run`)
- Web browser supporting Service Workers and Web Push (for web UI)

**Deployment:**
- Binary distribution: GitHub Releases (tarball + checksums)
- Installation: Homebrew tap (`asheshgoplani/homebrew-tap`) or direct binary
- No system dependencies beyond tmux (SQLite is bundled via modernc.org)

---

*Stack analysis: 2026-03-11*
