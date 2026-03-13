---
phase: 14-detection-sandbox
plan: 02
subsystem: tmux
tags: [opencode, prompt-detection, status-detection, tdd]

# Dependency graph
requires: []
provides:
  - OpenCode question tool waiting status detection via "enter submit" / "esc dismiss" help bar strings
  - Refined pulse-char busy detection preventing false positives in static UI contexts
  - VALIDATION 8.0 test suite (7 cases) for question tool and false-positive coverage
affects: [phase-16-comprehensive-testing]

# Tech tracking
tech-stack:
  added: []
  patterns: [TDD red-green cycle, authoritative-busy-first detection ordering, prompt-indicator guard for pulse chars]

key-files:
  created: []
  modified:
    - internal/tmux/patterns.go
    - internal/tmux/detector.go
    - internal/tmux/status_fixes_test.go

key-decisions:
  - "Pulse chars now only indicate busy when no prompt-indicating strings (enter submit, esc dismiss, press enter to send, Ask anything) are present; authoritative busy strings (esc interrupt, task text) always take priority"
  - "New patterns added to both PromptPatterns (for DefaultRawPatterns consumers) and HasPrompt direct checks (for detector consumers) to keep both code paths consistent"

requirements-completed: [DET-02]

# Metrics
duration: 4min
completed: 2026-03-13
---

# Phase 14 Plan 02: OpenCode Question Tool Detection Summary

**OpenCode question tool help bar ("enter submit"/"esc dismiss") now triggers waiting status detection; pulse char false-positive prevention added via prompt-indicator guard in hasOpencodeBusyIndicator**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-13T07:15:37Z
- **Completed:** 2026-03-13T07:19:00Z
- **Tasks:** 1 (TDD)
- **Files modified:** 3

## Accomplishments

- Detected OpenCode question tool selection UI as "waiting" status by adding "enter submit" and "esc dismiss" to PromptPatterns and HasPrompt opencode case
- Prevented false-positive busy detection when pulse chars appear in static UI elements (progress bars, decorative borders) alongside question tool prompts
- Added VALIDATION 8.0 test suite with 7 targeted cases covering idle/busy edge cases (TestOpencodeBusyGuard_QuestionTool + TestDefaultRawPatterns_OpenCodeQuestionTool)
- All pre-existing TestOpencodeBusyGuard (19 cases) continue to pass with no regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Add question tool detection tests and update patterns and detector** - `62f3c81` (feat)

**Plan metadata:** committed inline with SUMMARY.md

## Files Created/Modified

- `/Users/ashesh/claude-deck/internal/tmux/patterns.go` - Added "enter submit" and "esc dismiss" to opencode PromptPatterns
- `/Users/ashesh/claude-deck/internal/tmux/detector.go` - Added "enter submit"/"esc dismiss" checks to HasPrompt opencode case; refined hasOpencodeBusyIndicator with prompt-indicator guard for pulse chars
- `/Users/ashesh/claude-deck/internal/tmux/status_fixes_test.go` - Added VALIDATION 8.0 section with TestDefaultRawPatterns_OpenCodeQuestionTool and TestOpencodeBusyGuard_QuestionTool

## Decisions Made

- Pulse chars now only indicate busy when no prompt-indicating strings are present. This prevents false positives from decorative chars (progress bars, static UI borders) appearing alongside question tool help text.
- Authoritative busy strings ("esc interrupt", "esc to exit", "Thinking...", etc.) always take priority and are checked before the pulse char guard, so the guard does not weaken actual busy detection.
- Both `DefaultRawPatterns` PromptPatterns and `HasPrompt` direct checks were updated in tandem to keep both code paths consistent for all consumers of the detector.

## Deviations from Plan

None - plan executed exactly as written. The TDD flow (RED: tests fail, GREEN: implementation passes all tests) was followed precisely as specified.

## Issues Encountered

- `TestWaitForPaneReady_RealTmux` fails when run alongside other tests in the full suite due to tmux resource contention (pre-existing issue unrelated to this plan; passes in isolation).

## Self-Check: PASSED

- FOUND: internal/tmux/patterns.go
- FOUND: internal/tmux/detector.go
- FOUND: internal/tmux/status_fixes_test.go
- FOUND: commit 62f3c81

## Next Phase Readiness

- OpenCode detection improvements complete; ready for Phase 16 comprehensive testing
- No blockers introduced

---
*Phase: 14-detection-sandbox*
*Completed: 2026-03-13*
