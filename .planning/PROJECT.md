# Agent Deck

## What This Is

Agent-deck is a terminal session manager for AI coding agents (Go + Bubble Tea TUI managing tmux sessions). It manages tmux sessions with status tracking, supports multiple AI tools (Claude Code, Gemini CLI, OpenCode, Codex), and provides conductor orchestration for multi-agent workflows. Currently at v0.23.0.

## Core Value

Conductor orchestration and cross-session coordination must work reliably in production, enabling multi-agent workflows without manual intervention.

## Current Milestone: v1.2 Conductor Reliability & Learnings Cleanup

**Goal:** Fix the top operational issues discovered across 6 conductors, promote validated learnings to appropriate locations, and improve send/heartbeat reliability.

**Target features:**
- Heartbeat scope filtering (group-aware, not global)
- Heartbeat respects enabled=false configuration
- Enter key submission reliability (race condition fix)
- Codex session send timing (readiness detection)
- Exit 137 investigation and mitigation
- --wait flag reliability
- -cmd flag group parsing fix
- --no-parent + set-parent routing restoration
- Learnings promotion to shared CLAUDE.md, GSD conductor skill, and agent-deck skill

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
- Skills reformatted to official Anthropic skill-creator structure (v1.0)
- Sleep/wake detection and status transitions tested (v1.0)
- Session lifecycle unit tests passing (v1.0)
- Codebase stabilized, lint clean, all tests passing (v1.0)
- Integration test framework with shared helpers (v1.1)
- Conductor orchestration pipeline tested end-to-end (v1.1)
- Cross-session event notification tested (v1.1)
- Multi-tool session behavior verified (Claude, Gemini, OpenCode, Codex) (v1.1)
- Sleep/wait detection reliability tests across all tools (v1.1)
- Edge cases tested: concurrent polling, external storage changes, skills integration (v1.1)

### Active

- [ ] Heartbeat scope filtering per conductor group
- [ ] Heartbeat respects enabled=false configuration
- [ ] Enter key submission reliability
- [ ] Codex session send timing (readiness detection)
- [ ] Exit 137 from incoming messages (investigation + mitigation)
- [ ] --wait flag reliability
- [ ] -cmd flag group parsing
- [ ] --no-parent + set-parent routing restoration
- [ ] Learnings promoted to shared locations

### Out of Scope

- Project-specific learnings (ARD deploy, Ryan ElevenLabs, etc.) stay in their conductors
- Personal preferences (voice-to-text parsing) stay in user CLAUDE.md
- New features unrelated to conductor reliability
- UI/TUI testing (Bubble Tea testing requires separate approach)
- Performance/load testing

## Context

- **Previous milestones:** v1.0 Skills Reorg (3 phases), v1.1 Integration Testing (3 phases), both complete
- **Conductor operations:** 6 conductors in daily use, surfacing reliability issues documented in spec
- **Issue recurrence:** Enter key failure (15+), exit 137 (10+), Codex timing (7+), -cmd parsing (7+), set-parent (6+)
- **Existing test infrastructure:** TestMain files force `AGENTDECK_PROFILE=_test`, integration test framework from v1.1
- **Key files:** `internal/session/conductor.go`, `conductor_templates.go` (heartbeat), `internal/tmux/tmux.go` (SendKeys), `internal/session/instance.go` (Send, WaitForReady)
- **tmux layer:** `internal/tmux/` with session cache, pipe manager, pane capture
- **Learnings scattered:** 6 conductor LEARNINGS.md files need consolidation and promotion

## Constraints

- **Tech stack:** Go 1.24+, tmux, Bubble Tea, SQLite (modernc.org/sqlite)
- **Test isolation:** All tests must use `AGENTDECK_PROFILE=_test` via TestMain
- **tmux dependency:** Integration tests require running tmux server; must skip gracefully without one
- **No production side effects:** Tests must not affect real user sessions or state
- **Public repo:** No API keys, tokens, or personal data in test fixtures

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Skills stay in repo `skills/` directory | Plugin system copies them to cache on install | Good |
| GSD conductor goes to pool, not built-in | Only needed in conductor contexts, not every session | Good |
| Skip codebase mapping | CLAUDE.md already has comprehensive architecture docs | Good |
| Architecture first approach for test framework | Need consistent patterns before writing many tests | Good |

---
*Last updated: 2026-03-07 after milestone v1.2 initialization*
