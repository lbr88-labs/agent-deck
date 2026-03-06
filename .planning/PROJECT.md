# Agent Deck: Skills Reorganization & Stabilization

## What This Is

Agent-deck is a terminal session manager for AI coding agents (Go + Bubble Tea TUI managing tmux sessions). This milestone focuses on reorganizing the skills system to use the official Anthropic skill-creator format, packaging the GSD conductor skill for the pool, and stabilizing the codebase through testing and bug fixes. Currently at v0.21.1 on origin/main.

## Core Value

Skills must load correctly and trigger reliably when sessions start or on demand, ensuring agent-deck's skill ecosystem works seamlessly with Claude Code's plugin system.

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

### Active

- [ ] Reformat agent-deck skill to official skill-creator structure (SKILL.md + scripts/ + references/)
- [ ] Reformat session-share skill to official skill-creator structure
- [ ] Package gsd-conductor skill properly for the skills pool (~/.agent-deck/skills/pool/)
- [ ] Test sleep/wake detection and status transitions
- [ ] Test skills triggering (loading correctly when sessions start or on demand)
- [ ] Test session lifecycle end-to-end (start, stop, fork, attach, status tracking)
- [ ] Fix bugs discovered during testing
- [ ] Clean up dead code, improve linting, remove stale artifacts
- [ ] Ensure release readiness (all tests pass, lint clean, build succeeds)

### Out of Scope

- New features or functionality beyond skills reorganization
- Version bump decision (deferred until work is assessed)
- CI/CD pipeline changes
- Documentation beyond what's needed for skill format changes

## Context

- **Current skills location (repo):** `skills/agent-deck/` and `skills/session-share/` with SKILL.md + scripts/ + references/
- **Plugin cache:** Skills get copied to `~/.claude/plugins/cache/agent-deck/agent-deck/<hash>/skills/`
- **GSD conductor skill:** Already exists at `~/.agent-deck/skills/pool/gsd-conductor/SKILL.md` but may need updates
- **Pool skills:** On-demand loading via `Read ~/.agent-deck/skills/pool/<name>/SKILL.md`
- **Anthropic official format:** Uses `init_skill.py` from `example-skills/skill-creator`, produces SKILL.md + optional scripts/, references/, assets/
- **GSD framework:** Installed locally in `.claude/get-shit-done/` with commands, agents, workflows, templates
- **Test infrastructure:** TestMain files force `AGENTDECK_PROFILE=_test` for isolation

## Constraints

- **Tech stack:** Go 1.24+, tmux, Bubble Tea, SQLite (modernc.org/sqlite)
- **Compatibility:** Skills must work with Claude Code's plugin system and the existing `~/.claude/plugins/cache/` resolution
- **Safety:** Never run `tmux kill-server` or broad kill patterns targeting agentdeck sessions
- **Public repo:** No API keys, tokens, or personal data in commits

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Skills stay in repo `skills/` directory | Plugin system copies them to cache on install | -- Pending |
| GSD conductor goes to pool, not built-in | Only needed in conductor contexts, not every session | -- Pending |
| Skip codebase mapping | CLAUDE.md already has comprehensive architecture docs | -- Pending |

---
*Last updated: 2026-03-06 after initialization*
