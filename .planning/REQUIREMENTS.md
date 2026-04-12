# Requirements: Agent Deck v1.6.0 — Watcher Framework

**Defined:** 2026-04-10
**Core Value:** Reliable session management for AI coding agents: users can create, monitor, and control many concurrent agent sessions from anywhere without losing work or context.
**Milestone target:** v1.6.0
**Starting point:** v1.5.0
**Source spec:** `docs/superpowers/specs/2026-04-10-watcher-framework-design.md`
**Research:** `.planning/research/SUMMARY.md` (synthesized from Stack, Features, Architecture, Pitfalls)

## v1.6.0 Requirements

Requirements for the watcher framework milestone. Each maps to exactly one phase.

### Schema & Config

- [ ] **SCHEMA-01**: Watchers table exists in statedb with full ALTER TABLE migration path (SchemaVersion bumped to 5)
- [ ] **SCHEMA-02**: Watcher events table exists with UNIQUE(watcher_id, dedup_key) constraint and session_id column for thread reply routing
- [ ] **SCHEMA-03**: Existing databases upgrade cleanly (TestMigrate_OldSchema_WatcherTablesUpgrade in same PR)
- [ ] **SCHEMA-04**: Watcher events pruned to 500 rows per watcher on insert with (watcher_id, created_at DESC) index
- [ ] **SCHEMA-05**: WatcherSettings added to UserConfig with defaults applied in LoadConfig()
- [ ] **SCHEMA-06**: WatcherMeta struct persisted as meta.json in ~/.agent-deck/watchers/<name>/

### Engine Core

- [x] **ENGINE-01**: WatcherAdapter interface defined (Setup/Listen/Teardown/HealthCheck) with AdapterConfig
- [x] **ENGINE-02**: Event struct with DedupKey(), JSON serialization, source/sender/subject normalization
- [x] **ENGINE-03**: Router loads clients.json, matches exact email and wildcard *@domain, exact takes priority
- [x] **ENGINE-04**: Engine event loop deduplicates via INSERT OR IGNORE + rows-affected (no check-then-insert TOCTOU)
- [x] **ENGINE-05**: Single-writer goroutine serializes all watcher DB writes (buffered channel pattern)
- [x] **ENGINE-06**: Health tracker with rolling event rate, silence detection (max_silence_minutes), consecutive error counting
- [x] **ENGINE-07**: Engine Stop() cancels all adapter contexts without goroutine leaks (goleak test in same PR)

### Adapters

- [ ] **ADAPT-01**: Webhook adapter receives HTTP POST on configurable port, normalizes to Event, responds 202 immediately
- [ ] **ADAPT-02**: ntfy adapter subscribes to topic via SSE stream (bufio.Scanner), auto-reconnects on disconnect
- [ ] **ADAPT-03**: GitHub adapter verifies X-Hub-Signature-256 HMAC-SHA256, rejects invalid signatures with 401
- [x] **ADAPT-04**: Slack adapter routes via ntfy bridge with thread reply routing (session_id lookup by parent dedup_key)
- [ ] **ADAPT-05**: Gmail adapter handles OAuth2 token refresh via ReuseTokenSource, Pub/Sub watch registration via users.Watch()
- [ ] **ADAPT-06**: Gmail watch_expiry persisted in meta.json, renewal scheduled 1hr before expiry, immediate renewal on startup if within 2hr

### CLI

- [ ] **CLI-01**: `agent-deck watcher create` registers watcher in statedb + creates filesystem directory with meta.json
- [ ] **CLI-02**: `agent-deck watcher start/stop` manages watcher lifecycle (starts adapter goroutine or cancels context)
- [ ] **CLI-03**: `agent-deck watcher list` shows all watchers with name, type, status, event rate, health
- [ ] **CLI-04**: `agent-deck watcher status <name>` shows detailed info including recent events and config
- [ ] **CLI-05**: `agent-deck watcher test <name>` sends synthetic event through full pipeline, reports routing decision
- [ ] **CLI-06**: `agent-deck watcher routes` displays all clients.json routing rules with sender patterns and conductors
- [x] **CLI-07**: `agent-deck watcher import <path>` migrates existing bash issue-watcher to Go watcher (reads channels.json, generates watcher.toml + clients.json entries)

### TUI

- [ ] **TUI-01**: Watcher panel toggled with W key showing name, type, status indicator, event rate per hour
- [ ] **TUI-02**: Selecting a watcher shows recent events (last 10), routing decisions, and quick actions (start/stop/test/edit/logs)
- [ ] **TUI-03**: Health alerts sent via conductor notification bridge (Telegram/Slack/Discord) when watcher enters warning/error state
- [ ] **TUI-04**: W key binding audited against all existing single-key bindings in home.go, no conflicts, help overlay updated

### Intelligence

- [x] **INTEL-01**: Triage session spawned (via agent-deck launch) for unknown senders, classifies with structured output: ROUTE_TO, SUMMARY, CONFIDENCE
- [ ] **INTEL-02**: Confirmed triage decisions auto-added to clients.json via atomic write-temp-rename (self-improving routing)
- [x] **INTEL-03**: Triage rate limited to max 5 sessions per hour to prevent subscription usage spikes
- [ ] **INTEL-04**: Watcher-creator skill in agent-deck pool enables conversational watcher setup (creates watcher.toml + clients.json entries + conductor if needed)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Web Integration

- **WEB-01**: Watcher management panel in web app (start/stop/status/events)
- **WEB-02**: Real-time event stream in web UI via SSE

### Advanced Adapters

- **ADV-01**: Fathom meeting transcript adapter (webhook-based with participant extraction)
- **ADV-02**: Fireflies meeting transcript adapter
- **ADV-03**: IMAP IDLE adapter for non-Gmail providers
- **ADV-04**: Microsoft Graph webhook adapter for Outlook

### Community

- **COMM-01**: Community adapter SDK for third-party adapters
- **COMM-02**: Adapter marketplace or registry

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Always-on LLM router | Config-driven routing handles 95%+ at zero cost; triage fallback for rest |
| IMAP IDLE adapter | Requires persistent TCP connection; Gmail Pub/Sub is recommended for Google |
| Web UI watcher panel | TUI + CLI sufficient for v1.6.0; web integration deferred to v1.7+ |
| Community adapter marketplace | Future possibility after adapter interface stabilizes |
| Windows native support | Tailscale from Mac/iPhone covers remote access; no validated demand |
| Managed Agents / Agent SDK | Require API key billing; incompatible with subscription-based Claude Code |
| Meeting-specific adapters | Generic webhook adapter covers Fathom/Fireflies; specific adapters v1.7+ |
| Storing full payloads in SQLite | Unbounded growth; events store metadata only, raw payloads in filesystem |
| Silence windows config | Field reserved in schema; logic deferred to v1.6.1 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SCHEMA-01 | Phase 12 | Pending |
| SCHEMA-02 | Phase 12 | Pending |
| SCHEMA-03 | Phase 12 | Pending |
| SCHEMA-04 | Phase 12 | Pending |
| SCHEMA-05 | Phase 12 | Pending |
| SCHEMA-06 | Phase 12 | Pending |
| ENGINE-01 | Phase 13 | Complete |
| ENGINE-02 | Phase 13 | Complete |
| ENGINE-03 | Phase 13 | Complete |
| ENGINE-04 | Phase 13 | Complete |
| ENGINE-05 | Phase 13 | Complete |
| ENGINE-06 | Phase 13 | Complete |
| ENGINE-07 | Phase 13 | Complete |
| ADAPT-01 | Phase 14 | Pending |
| ADAPT-02 | Phase 14 | Pending |
| ADAPT-03 | Phase 14 | Pending |
| ADAPT-04 | Phase 15 | Complete |
| ADAPT-05 | Phase 17 | Pending |
| ADAPT-06 | Phase 17 | Pending |
| CLI-01 | Phase 16 | Pending |
| CLI-02 | Phase 16 | Pending |
| CLI-03 | Phase 16 | Pending |
| CLI-04 | Phase 16 | Pending |
| CLI-05 | Phase 16 | Pending |
| CLI-06 | Phase 16 | Pending |
| CLI-07 | Phase 15 | Complete |
| TUI-01 | Phase 16 | Pending |
| TUI-02 | Phase 16 | Pending |
| TUI-03 | Phase 16 | Pending |
| TUI-04 | Phase 16 | Pending |
| INTEL-01 | Phase 18 | Complete |
| INTEL-02 | Phase 18 | Pending |
| INTEL-03 | Phase 18 | Complete |
| INTEL-04 | Phase 18 | Pending |

**Coverage:**
- v1.6.0 requirements: 34 total
- Mapped to phases: 34 (roadmap created 2026-04-10)
- Unmapped: 0

---
*Requirements defined: 2026-04-10 from design spec with research from `.planning/research/SUMMARY.md`*
*Last updated: 2026-04-10 after roadmap creation — all 34 requirements mapped to phases 12-18*
