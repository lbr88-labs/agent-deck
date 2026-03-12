---
gsd_state_version: 1.0
milestone: v1.3
milestone_name: Session Reliability & Resume
status: active
stopped_at: null
last_updated: "2026-03-12"
last_activity: 2026-03-12 -- Roadmap created for v1.3
progress:
  total_phases: 5
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-12)

**Core value:** Reliable terminal session management for AI coding agents with conductor orchestration
**Current focus:** v1.3 Session Reliability & Resume

## Current Position

```
Phase:    11 — Storage Foundation
Plan:     —
Status:   Roadmap defined, ready for planning
Progress: [----------] 0% (0/5 phases)
```

Last activity: 2026-03-12 — Roadmap created (Phases 11-15 defined)

## Accumulated Context

### Decisions

Full decision log in PROJECT.md Key Decisions table.

### v1.3 Phase Notes

**Phase 11 (Storage Foundation):**
- Verify `PRAGMA busy_timeout` in `statedb.Open()` before touching storage code (10-minute check)
- toolDataBlob field drift requires 6-location update; refactor to struct parameter first to make compiler catch misses
- Round-trip test must use a fresh Storage instance (not same in-memory instance that saved)
- UX-03 (auto_cleanup docs) bundled here, zero risk, prevents indefinite deferral

**Phase 12 (Session List Correctness):**
- Audit all StatusStopped exclusion sites before writing new render code
- session_picker_dialog.go:41 already correctly excludes stopped sessions for conductor picker; preserve this behavior
- Preview pane differentiation: stopped = user intent + resume affordance; error = crash + distinct guidance

**Phase 13 (Resume Deduplication):**
- UpdateClaudeSessionsWithDedup must run in-memory immediately at resume site (not only at persist time)
- Write concurrent-write test: two Storage instances against the same SQLite file
- Audit hook status ordering while touching dedup logic (Pitfall 5)

**Phase 14 (Auto-Start TTY Fix):**
- Root cause on WSL/Linux NOT confirmed without reproduction; three candidate failure modes identified
- Flag for hands-on debugging session on WSL/Linux before writing implementation tasks
- Correct session ID propagation depends on Phase 13 dedup being stable

**Phase 15 (Mouse Support):**
- Fully independent of Phases 12-14; can be parallelized
- Use tea.MouseButtonWheelUp / tea.MouseButtonWheelDown (NOT deprecated tea.MouseWheelUp/Down)
- WithMouseCellMotion already active in main.go:468; handler is a new case tea.MouseMsg in home.go:Update()
- Mouse handler must be O(1), no blocking I/O (Bubble Tea issue #1047 risk)
- Always test scroll inside tmux, not just standalone terminal (tmux mouse mode conflict risk)

### Pending Todos

None.

### Blockers/Concerns

- Exit 137 is a known Claude Code limitation. Mitigated via status gating, documented in conductor CLAUDE.md.
- #266 (tmux set-environment in Docker) blocked by #320 (sandbox persistence). Will be unblocked by Phase 11.
- Phase 14 (auto-start TTY fix) root cause on WSL/Linux not confirmed; requires reproduction before planning.

## Session Continuity

Last session: 2026-03-12
Stopped at: Roadmap created for v1.3; Phase 11 ready for plan-phase
Resume file: None
