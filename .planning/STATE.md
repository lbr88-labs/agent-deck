---
gsd_state_version: 1.0
milestone: v0.26.2
milestone_name: Stability Fixes
status: planning
stopped_at: ""
last_updated: "2026-03-16"
last_activity: "2026-03-16 — Milestone v0.26.2 started"
progress:
  total_phases: 0
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-16)

**Core value:** Reliable terminal session management for AI coding agents with conductor orchestration
**Current focus:** v0.26.2 Stability Fixes

## Current Position

```
Phase:    Not started (defining requirements)
Plan:     —
Status:   Defining requirements
Progress: [----------] 0%
```

Last activity: 2026-03-16 — Milestone v0.26.2 started

## Accumulated Context

### Decisions

Full decision log in PROJECT.md Key Decisions table.
- [Phase 12-session-list-resume-ux]: Split combined StatusError||StatusStopped preview block into two separate status-checked blocks: stopped gets user-intent messaging, error gets crash-diagnostic messaging
- [Phase 12]: Dedup call placed outside saveInstances() under explicit instancesMu.Lock() to avoid re-entrant lock deadlock
- [Phase 15-mouse-theme-polish]: Mouse wheel routing uses overlay priority guard in Home.Update(); ScrollUp/ScrollDown helpers on SettingsPanel and MCPDialog; tea.MouseButtonWheelUp/Down (not deprecated constants)
- [Phase 15-mouse-theme-polish]: ANSI background stripping in preview pane uses compiled regexp covering standard/bright/256-color/truecolor backgrounds; applied per-line only when ThemeLight active
- [Phase 14-detection-sandbox]: Pulse chars only indicate busy when no prompt-indicating strings present; authoritative busy strings always take priority over pulse char guard
- [Phase 13-auto-start-platform]: generateUUID uses crypto/rand directly (no google/uuid dependency); pane-ready timeout non-fatal with Warn logging

### Pending Todos

None.

### Blockers/Concerns

- Exit 137 is a known Claude Code limitation. Mitigated via status gating, documented in conductor CLAUDE.md.
- #340 (Wayland keys) may require upstream Bubble Tea investigation.

## Session Continuity

Last session: 2026-03-16
Stopped at: —
Resume file: None
