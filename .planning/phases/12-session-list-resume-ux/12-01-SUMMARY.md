---
phase: 12-session-list-resume-ux
plan: 01
status: complete
completed: "2026-03-13"
---

## Summary

Split the TUI preview pane into distinct code paths for stopped vs error sessions, replacing the combined "Session Inactive" block with differentiated headers, messaging, and action guidance.

## What Changed

- **Stopped sessions** now show "Session Stopped" header with user-intentional messaging ("You stopped this session intentionally"), resume-focused actions, and dim gray styling
- **Error sessions** now show "Session Error" header with crash-diagnostic messaging (tmux restart, terminal closed, etc.), start-focused actions, and red styling
- VIS-01 preserved: stopped sessions remain visible in main session list (verified via `TestFlatItems_IncludesStoppedSessions`)
- VIS-03 preserved: session picker dialog still excludes stopped sessions from conductor flows (unchanged `session_picker_dialog.go`)

## Key Files

### Created
- `internal/ui/preview_pane_test.go` — Tests for stopped/error header differentiation, resume-oriented text, crash-diagnostic text

### Modified
- `internal/ui/home.go` — Split combined `StatusError || StatusStopped` preview block (~line 9871) into two separate blocks with distinct headers, icons, messaging, and action labels

## Commits
- `c3b3b0e test(12-01): add failing tests for stopped vs error preview pane differentiation`
- `df9c1b6 feat(12-01): split preview pane into distinct stopped vs error code paths`

## Deviations

None.
