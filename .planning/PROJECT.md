# Agent Deck

## What This Is

Agent-deck is a terminal session manager for AI coding agents (Go + Bubble Tea TUI managing tmux sessions). It manages tmux sessions with status tracking, supports multiple AI tools (Claude Code, Gemini CLI, OpenCode, Codex), and provides conductor orchestration for multi-agent workflows. Currently at v0.25.1.

## Core Value

Reliable terminal session management for AI coding agents, with conductor orchestration enabling multi-agent workflows without manual intervention.

## Requirements

### Validated

- Session lifecycle management (start, stop, fork, attach, restart)
- tmux session management with status tracking
- Bubble Tea TUI with responsive layout
- SQLite persistence (WAL mode, no CGO)
- MCP attach/detach with LOCAL/GLOBAL scope
- Profile system with isolated state
- Git worktree integration
- Claude Code and Gemini CLI integration
- Plugin system with skills loading from cache
- Skills reformatted to official Anthropic skill-creator structure -- v1.0
- Sleep/wake detection and status transitions tested -- v1.0
- Session lifecycle unit tests passing -- v1.0
- Codebase stabilized, lint clean, all tests passing -- v1.0
- Integration test framework with shared helpers -- v1.1
- Conductor orchestration pipeline tested end-to-end -- v1.1
- Cross-session event notification tested -- v1.1
- Multi-tool session behavior verified (Claude, Gemini, OpenCode, Codex) -- v1.1
- Sleep/wait detection reliability tests across all tools -- v1.1
- Edge cases tested: concurrent polling, external storage changes, skills integration -- v1.1
- Enter key submission reliability (hardened retry with Codex readiness) -- v1.2
- Heartbeat scoped to conductor groups with interval=0 disable semantics -- v1.2
- Session death detection in waitForCompletion -- v1.2
- CLI flag parsing (-c/-g co-parsing, --no-parent docs) -- v1.2
- Exit 137 root cause documented (Claude Code limitation) with 6 mitigations -- v1.2
- 27 learnings promoted to 3 shared destinations -- v1.2

### Active

<!-- v1.3 Session Reliability & Resume -->
- [ ] Sandbox config persisted through SQLite lifecycle (#320)
- [ ] Auto-start works on WSL/Linux without TTY redirect breaking tools (#311)
- [ ] Resumed sessions deduplicated by conversation ID (#224)
- [ ] Stopped sessions visible and resumable in TUI (#307)
- [ ] Settings panel exposes custom tools (#318)
- [ ] Mouse/trackpad support for list navigation and scrolling (#262, #254)
- [ ] auto_cleanup option documented (#228)

### Out of Scope

- Project-specific learnings (ARD deploy, Ryan ElevenLabs, etc.) stay in their conductors
- Personal preferences (voice-to-text parsing) stay in user CLAUDE.md
- UI/TUI testing (Bubble Tea testing requires separate approach)
- Performance/load testing at scale (50+ sessions)

## Context

- **Shipped milestones:** v1.0 (skills reorg, 3 phases), v1.1 (integration testing, 3 phases), v1.2 (conductor reliability, 4 phases)
- **Total phases completed:** 10 (21 plans)
- **Current milestone:** v1.3 Session Reliability & Resume — fixing critical session lifecycle bugs and UX gaps
- **Codebase:** ~114K LOC Go
- **Tech stack:** Go 1.24+, tmux, Bubble Tea, SQLite (modernc.org/sqlite)
- **Conductor operations:** 6 conductors in daily use, top reliability issues fixed in v1.2
- **Known limitation:** Exit 137 is a Claude Code design choice (kills Bash tool children on new PTY input), mitigated via status gating

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Skills stay in repo `skills/` directory | Plugin system copies them to cache on install | Good |
| GSD conductor goes to pool, not built-in | Only needed in conductor contexts, not every session | Good |
| Skip codebase mapping | CLAUDE.md already has comprehensive architecture docs | Good |
| Architecture first approach for test framework | Need consistent patterns before writing many tests | Good |
| Consolidated send into `internal/send` package | 7 duplicated prompt detection functions across codebase | Good |
| Aggressive early retry for Enter key | Every iteration for first 5, then every 2nd (was every 3rd) | Good |
| Exit 137 documented, not fixed | Claude Code limitation, not fixable in agent-deck | Accepted |
| interval=0 means disabled | Simpler than separate boolean; negative means use default 15 | Good |
| 5 consecutive errors for session death | Threshold balances false positives with responsiveness | Good |

## Constraints

- **Tech stack:** Go 1.24+, tmux, Bubble Tea, SQLite (modernc.org/sqlite)
- **Test isolation:** All tests must use `AGENTDECK_PROFILE=_test` via TestMain
- **tmux dependency:** Integration tests require running tmux server; must skip gracefully without one
- **No production side effects:** Tests must not affect real user sessions or state
- **Public repo:** No API keys, tokens, or personal data in test fixtures

---
## Current Milestone: v1.3 Session Reliability & Resume

**Goal:** Fix critical session lifecycle bugs (sandbox persistence, auto-start, resume dedup) and improve resume UX, mouse support, and settings completeness.

**Target features:**
- Sandbox config persistence through SQLite (#320)
- Auto-start TTY fix for WSL/Linux (#311)
- Session deduplication on resume (#224)
- Resume UX: stopped sessions in TUI (#307)
- Settings custom tools completion (#318)
- Mouse/trackpad support (#262, #254)
- auto_cleanup documentation (#228)

---
*Last updated: 2026-03-12 after v1.3 milestone start*
