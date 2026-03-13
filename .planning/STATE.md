---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: planning
stopped_at: Milestone rescoped; Phase 11 (MCP Proxy Reliability) ready for plan-phase
last_updated: "2026-03-13T06:54:35.144Z"
last_activity: "2026-03-12 — Milestone rescoped: removed completed #320/#318, added #324/#322/#266/#255/#225/#216"
progress:
  total_phases: 6
  completed_phases: 1
  total_plans: 3
  completed_plans: 2
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-12)

**Core value:** Reliable terminal session management for AI coding agents with conductor orchestration
**Current focus:** v1.3 Session Reliability & Resume (rescoped 2026-03-12)

## Current Position

```
Phase:    11 — MCP Proxy Reliability
Plan:     —
Status:   Milestone rescoped, ready for Phase 11 planning
Progress: [----------] 0% (0/6 phases)
```

Last activity: 2026-03-12 — Milestone rescoped: removed completed #320/#318, added #324/#322/#266/#255/#225/#216

## Rescoping Summary (2026-03-12)

**Removed (already shipped):**
- #320 Sandbox config persistence — CLOSED 2026-03-12 (was Phase 11 Storage Foundation)
- #318 Settings default tool icons — CLOSED 2026-03-11 (was in Phase 15)

**Added (new/promoted):**
- #324 MCP socket proxy request ID collision — CRITICAL, new Phase 11
- #322 Light theme dark background bleed — moved to Phase 15
- #266 tmux set-environment in Docker — unblocked by #320 fix, Phase 14
- #255 OpenCode waiting status detection — Phase 14
- #225 Redundant heartbeat cleanup — Phase 15
- #216 Support existing worktree — Phase 15

**New phase:** Phase 16 (Comprehensive Testing) added for full regression coverage

## Accumulated Context

### Decisions

Full decision log in PROJECT.md Key Decisions table.

### v1.3 Phase Notes

**Phase 11 (MCP Proxy Reliability):**
- #324 is critical for production multi-session workflows; proxy request IDs collide when multiple sessions share a proxy
- Investigate `internal/mcppool/` for the ID generation and routing logic
- Must test under race detector with concurrent tool calls

**Phase 12 (Session List & Resume UX):**
- Combines original Phase 12 (visibility) and Phase 13 (dedup) since they're closely coupled
- Audit all StatusStopped exclusion sites before writing new render code
- session_picker_dialog.go:41 already correctly excludes stopped sessions for conductor picker; preserve this behavior
- Preview pane differentiation: stopped = user intent + resume affordance; error = crash + distinct guidance
- UpdateClaudeSessionsWithDedup must run in-memory immediately at resume site

**Phase 13 (Auto-Start & Platform):**
- Root cause on WSL/Linux NOT confirmed without reproduction; three candidate failure modes identified
- Flag for hands-on debugging session on WSL/Linux before writing implementation tasks
- Correct session ID propagation depends on Phase 12 dedup being stable

**Phase 14 (Detection & Sandbox):**
- #266 was blocked by #320, now unblocked since sandbox persistence is fixed
- #255 requires understanding OpenCode's question tool prompt format for status detection
- Both are relatively contained changes

**Phase 15 (Mouse, Theme & Polish):**
- Fully independent of Phases 12-14; can be parallelized with them
- Use tea.MouseButtonWheelUp / tea.MouseButtonWheelDown (NOT deprecated tea.MouseWheelUp/Down)
- WithMouseCellMotion already active in main.go:468
- Mouse handler must be O(1), no blocking I/O (Bubble Tea issue #1047 risk)
- Light theme (#322): audit hardcoded color values in preview and session rendering
- Heartbeat (#225): consolidate systemd timer and bridge.py heartbeat_loop
- Worktree (#216): check for existing worktree before creating new one

**Phase 16 (Comprehensive Testing):**
- Runs after all implementation phases
- Must cover every fix with at least one integration test
- Race detector mandatory for MCP proxy tests
- Goal: robust regression suite that prevents future breakage

### Pending Todos

None.

### Blockers/Concerns

- Exit 137 is a known Claude Code limitation. Mitigated via status gating, documented in conductor CLAUDE.md.
- Phase 13 (auto-start TTY fix) root cause on WSL/Linux not confirmed; requires reproduction before planning.
- #324 (MCP proxy) is the most critical issue: can cause tool call hangs in production multi-session workflows.

## Session Continuity

Last session: 2026-03-12
Stopped at: Milestone rescoped; Phase 11 (MCP Proxy Reliability) ready for plan-phase
Resume file: None
