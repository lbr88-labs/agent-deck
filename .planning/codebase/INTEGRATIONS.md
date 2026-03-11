# External Integrations

**Analysis Date:** 2026-03-11

## APIs & External Services

**GitHub API:**
- Update checking: `https://api.github.com/repos/asheshgoplani/agent-deck/releases/latest`
  - SDK/Client: Go stdlib `net/http`
  - Purpose: Check for new releases
  - Auth: None (unauthenticated, rate-limited)
  - Location: `internal/update/update.go:CheckForUpdate()`

**GitHub Raw Content:**
- Changelog fetch: `https://raw.githubusercontent.com/asheshgoplani/agent-deck/main/CHANGELOG.md`
  - SDK/Client: Go stdlib `net/http`
  - Purpose: Display changelog to user during update flow
  - Auth: None
  - Location: `internal/update/update.go:GetChangelog()`

**Web Push Service (Browser vendor-specific):**
- Sends push notifications to registered browser endpoints
  - SDK/Client: `github.com/SherClockHolmes/webpush-go`
  - Auth: VAPID keypair (auto-generated, stored locally in `~/.agent-deck/profiles/{profile}/web_push_vapid_keys.json`)
  - Purpose: Real-time notifications from session status changes
  - Location: `internal/web/push_service.go`

## Data Storage

**Databases:**
- SQLite (local file-based)
  - Connection: `sqlite:///Users/{user}/.agent-deck/profiles/{profile}/state.db`
  - Client: `modernc.org/sqlite` (pure Go, no CGO)
  - Schema version: 2 (managed via migrations in `internal/statedb/migrate.go`)
  - Concurrency: WAL mode enabled, supports multiple readers/writers via busy timeout
  - Tables: `instances` (sessions), `groups` (hierarchy), `recent_sessions` (deleted session recovery), `update_cache`
  - Location: `internal/statedb/statedb.go`, `internal/statedb/migrate.go`

**File Storage:**
- Local filesystem only
  - Session logs: `~/.agent-deck/profiles/{profile}/session_data.jsonl`
  - Web push subscriptions: `~/.agent-deck/profiles/{profile}/web_push_subscriptions.json`
  - VAPID keys: `~/.agent-deck/profiles/{profile}/web_push_vapid_keys.json`
  - Global config: `~/.agent-deck/config.json`
  - Update cache: `~/.agent-deck/profiles/{profile}/update-cache.json`

**Caching:**
- In-memory: Menu snapshots cached during web server lifetime (invalidated on session changes)
- File-based: Update check cache (1-hour expiry via config, stored in `update-cache.json`)

## Authentication & Identity

**Auth Provider:**
- None for primary app (CLI TUI is local-only)
- Web mode: Token-based auth (API token required for HTTP endpoints)
  - Implementation: Bearer token in `Authorization: Bearer {token}` header
  - Token source: Passed via `-t` flag to `agent-deck web` command
  - Validation: `internal/web/auth.go` checks token on protected endpoints
  - Scope: Protects `/api/*` and `/s/` routes from unauthenticated access

**Claude Code Integration:**
- No direct API communication
- Integration via: Project configuration in `~/.claude.json`, MCP server definitions
- Session tracking: Stores `lastSessionId` in Claude's project metadata
- Location: `internal/session/claude.go`

## Monitoring & Observability

**Error Tracking:**
- None detected (no Sentry, Rollbar, or similar)

**Logs:**
- Structured logging via Go stdlib `log/slog`
- Component-based: `CompUI`, `CompSession`, `CompWeb`, `CompTmux`, etc.
- Rotation: `gopkg.in/natefinch/lumberjack.v2` (file size-based)
- Location: `~/.agent-deck/logs/`
- Debug mode: `AGENTDECK_DEBUG=1` environment variable
- Output: File + console (depending on command)

## CI/CD & Deployment

**Hosting:**
- GitHub (source code: `https://github.com/asheshgoplani/agent-deck`)
- GitHub Releases (binary distribution)
- Homebrew tap: `asheshgoplani/homebrew-tap` (formula in tap repository)

**CI Pipeline:**
- None (no GitHub Actions or external CI)
- Local release: GoReleaser CLI (`make release-local`)
- Pre-push hooks: Lefthook (lint + test + build in parallel)
- Validation: Tag must match code `Version` constant

**Binary Distribution:**
- Platforms: Linux (amd64, arm64), Darwin/macOS (amd64, arm64)
- Format: tar.gz with checksums
- Artifact location: GitHub Releases page
- Installation: Homebrew or direct download

## Environment Configuration

**Required env vars (runtime):**
- `AGENTDECK_PROFILE` - Profile name (optional, falls back to config default or "default")
- `CLAUDE_CONFIG_DIR` - Claude Code config directory (optional, for project lookup)

**Required env vars (web mode):**
- `-t TOKEN` or implicit: Token required for `/api/*` endpoints in web mode

**Optional env vars:**
- `AGENTDECK_DEBUG` - Debug logging
- `AGENTDECK_COLOR` - Color mode (truecolor/256/16/none)
- `AGENTDECK_SANDBOX` - Enable Docker sandbox mode

**Secrets location:**
- VAPID private key: `~/.agent-deck/profiles/{profile}/web_push_vapid_keys.json` (600 perms)
- Web push subscriptions: `~/.agent-deck/profiles/{profile}/web_push_subscriptions.json` (600 perms)
- SQLite database: `~/.agent-deck/profiles/{profile}/state.db` (user-readable)
- No API keys or credentials stored (GitHub API is unauthenticated)

## Webhooks & Callbacks

**Incoming:**
- Web Push Protocol: Browser sends subscription endpoints to `/api/push/subscribe`
  - Payload: `PushSubscription` with endpoint, P256DH key, auth key
  - Location: `internal/web/handlers_push.go:handlePushSubscribe()`

- Session Status Updates: Clients POST to `/api/push/presence` with focus state
  - Payload: `{ clientFocused: bool }`
  - Location: `internal/web/handlers_push.go:handlePushPresence()`

- WebSocket connections: `/ws/session/{sessionID}` for real-time output streaming
  - Protocol: WebSocket with JSON messages
  - Location: `internal/web/handlers_ws.go:handleSessionWS()`

**Outgoing:**
- Web Push notifications: Sent to browser push service endpoints (vendor-specific)
  - Trigger: Session status changes (idle, running, error)
  - Payload: Session title, status, menu snapshot
  - Location: `internal/web/push_service.go:Push()`

- GitHub Release Check: Poll `https://api.github.com/repos/asheshgoplani/agent-deck/releases/latest`
  - Frequency: Configurable, default 1 hour (via `check_interval_hours` in config)
  - Cache: Stored locally to avoid repeated API calls
  - Location: `internal/update/update.go:CheckForUpdate()`

## MCP Server Integration

**MCP Definition Sources:**
- Global MCPs: `~/.claude.json` (mcpServers section)
- Project MCPs: `~/.claude.json` (projects[path].mcpServers)
- Local MCPs: `.mcp.json` files (walks up directory tree)

**MCP Detection & Tracking:**
- Auto-detection from Claude Code config on session start
- Stored in database for session history
- File watching: `fsnotify` monitors `.mcp.json` changes in real-time
- Location: `internal/session/claude.go`, `internal/session/mcp_*.go`

## Docker Integration

**Container Management:**
- Sandbox mode: Sessions can run inside Docker containers (optional)
- Execution: Via `docker run` with mounted volumes, user ID matching
- Image: Configurable (default: official Go image)
- Security: Docker socket NOT mounted into container (sandboxed)
- Location: `internal/docker/` package

**Configuration:**
- Per-session sandbox opt-in
- Image selection: Template-based or explicit image URI
- Environment passthrough: Selected env vars passed to container

---

*Integration audit: 2026-03-11*
