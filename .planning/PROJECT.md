# Agent Deck: Integration Testing Framework

## What This Is

Agent-deck is a terminal session manager for AI coding agents (Go + Bubble Tea TUI managing tmux sessions). This milestone focuses on building a comprehensive integration testing framework that tests the full conductor orchestration pipeline, multi-tool session behavior, and cross-session event mechanisms. Currently at v0.21.1 on origin/main.

## Core Value

Conductor orchestration and cross-session coordination must be reliably tested end-to-end, catching regressions before they reach users.

## Current Milestone: v1.1 Integration Testing

**Goal:** Build a comprehensive integration testing framework for agent-deck that tests the full conductor orchestration pipeline.

**Target features:**
- Test framework architecture for integration tests
- Conductor orchestration pipeline testing (send to child response)
- Cross-session event notification testing
- Multi-tool session testing (Claude Code, Gemini CLI, OpenCode, Codex)
- Sleep/wait detection reliability testing across all tools
- Skills attachment and triggering tests
- Session lifecycle integration tests (start, stop, fork, restart with flags)

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

### Active

- [ ] Integration test framework architecture
- [ ] Conductor orchestration pipeline tests
- [ ] Cross-session event notification tests
- [ ] Multi-tool session behavior tests
- [ ] Sleep/wait detection reliability tests
- [ ] Skills attachment and triggering integration tests
- [ ] Session lifecycle integration tests with flags

### Out of Scope

- New features or functionality beyond testing infrastructure
- UI/TUI testing (Bubble Tea testing requires separate approach)
- Performance/load testing
- CI/CD pipeline integration (tests run locally)

## Context

- **Previous milestone (v1.0):** Skills Reorganization & Stabilization, 18 requirements completed across 3 phases
- **Existing test infrastructure:** TestMain files force `AGENTDECK_PROFILE=_test` for isolation, `skipIfNoTmuxServer(t)` for CI graceful skip
- **Conductor logic:** `internal/session/conductor.go` handles multi-agent orchestration
- **Multi-tool support:** Claude Code (`claude.go`), Gemini CLI (`gemini.go`), with different sleep/wait patterns
- **Session management:** `internal/session/instance.go` (struct, status enum), `storage.go` (SQLite), `config.go` (profiles)
- **tmux layer:** `internal/tmux/` with session cache, pipe manager, pane capture

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
| Architecture first approach for test framework | Need consistent patterns before writing many tests | -- Pending |

---
*Last updated: 2026-03-06 after milestone v1.1 initialization*
