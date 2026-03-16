---
phase: 15-mouse-theme-polish
plan: 01
subsystem: ui
tags: [bubbletea, mouse, theme, ansi, lipgloss, tui]

requires: []
provides:
  - Mouse wheel scroll routing in Home.Update() with overlay priority
  - ScrollUp/ScrollDown methods on SettingsPanel and MCPDialog
  - tea.MouseMsg handler in GlobalSearch.Update()
  - stripANSIBackground() helper for light theme preview rendering
affects: []

tech-stack:
  added: [regexp (stdlib, used for ANSI background stripping)]
  patterns:
    - "Mouse wheel routing: overlay priority guard in Home.Update() before delegating to ScrollUp/ScrollDown helpers"
    - "ANSI stripping: compile regex once as package var, apply per-line when ThemeLight active"

key-files:
  created: []
  modified:
    - internal/ui/home.go
    - internal/ui/settings_panel.go
    - internal/ui/global_search.go
    - internal/ui/mcp_dialog.go

key-decisions:
  - "Used tea.MouseButtonWheelUp/Down (not deprecated tea.MouseWheelUp/Down) and matched on msg.Button"
  - "Mouse handler is O(1) with no blocking I/O; preview fetch routes through existing debounce via fetchSelectedPreview()"
  - "Background ANSI regex covers standard (40-47), bright (100-107), 256-color/truecolor (48;...) and reset (49) — foreground and formatting preserved"

patterns-established:
  - "Overlay priority for mouse: setupWizard > settings > help > globalSearch > mcpDialog > newDialog/forkDialog > main list"

requirements-completed: [UX-01, UX-02]

duration: 7min
completed: 2026-03-13
---

# Phase 15 Plan 01: Mouse Scroll and Light Theme Fix Summary

**Mouse wheel scroll routing across all TUI overlays plus ANSI background stripping in preview pane for light theme users**

## Performance

- **Duration:** ~7 min
- **Started:** 2026-03-13T07:00:00Z
- **Completed:** 2026-03-13T07:05:26Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Mouse wheel events now scroll the session list, settings panel, global search results, and MCP dialog cursor
- Light theme no longer shows dark background bands from captured tmux pane output
- Overlay priority routing ensures wheel events reach the correct active area; non-wheel events are silently dropped

## Task Commits

Each task was committed atomically:

1. **Task 1 + Task 2: Mouse scroll and light theme fix** - `2dd3f79` (feat/fix)

Note: Both task sets of changes were staged together before the first commit (home.go edits for both tasks were made before the git add). All changes landed in a single atomic commit.

## Files Created/Modified

- `/Users/ashesh/claude-deck/internal/ui/home.go` - Added tea.MouseMsg case in Update(), stripANSIBackground() helper, regexp import, and light-theme application in preview loop
- `/Users/ashesh/claude-deck/internal/ui/settings_panel.go` - Added ScrollUp() and ScrollDown() methods
- `/Users/ashesh/claude-deck/internal/ui/mcp_dialog.go` - Added ScrollUp() and ScrollDown() methods
- `/Users/ashesh/claude-deck/internal/ui/global_search.go` - Added tea.MouseMsg case in Update() for wheel events

## Decisions Made

- Used `msg.Button` matching with `tea.MouseButtonWheelUp/Down` (not deprecated `tea.MouseWheelUp/Down` constants)
- Background ANSI regex targets: standard colors (ESC[40-47m), bright colors (ESC[100-107m), 256/truecolor (ESC[48;...m), and default reset (ESC[49m). Foreground sequences untouched.
- Compiled the regex as a package-level `var` (not inside the function) to avoid per-call allocation

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None. Both tasks compiled clean on first attempt.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Mouse scroll and light theme fix are self-contained; no follow-up work required
- Phase 15 plan 01 complete; next plan can proceed
- No blockers

---
*Phase: 15-mouse-theme-polish*
*Completed: 2026-03-13*
