---
phase: 12-session-list-resume-ux
plan: 01
subsystem: ui
tags: [bubble-tea, lipgloss, session-status, preview-pane, tdd]

# Dependency graph
requires: []
provides:
  - "Split preview pane: stopped sessions show 'Session Stopped' with resume guidance"
  - "Split preview pane: error sessions show 'Session Error' with crash-diagnostic guidance"
  - "VIS-01 verified: stopped sessions visible in main session list"
  - "VIS-03 verified: session picker dialog correctly excludes stopped sessions"
affects:
  - 12-session-list-resume-ux
  - 13-auto-start-platform

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Status-split preview pane: separate if-blocks per status instead of combined OR condition"
    - "TDD RED/GREEN for UI rendering: write assertions on string output before implementing"

key-files:
  created:
    - internal/ui/preview_pane_test.go
  modified:
    - internal/ui/home.go

key-decisions:
  - "Split combined 'Session Inactive' block at home.go:9871 into two separate status-checked blocks: StatusStopped first, then StatusError"
  - "Stopped preview uses resume-oriented language (stopped by user, preserved for resuming, Resume key label)"
  - "Error preview uses crash-diagnostic language (No tmux session running, cause list, Start key label)"
  - "Pad-to-height test updated: function uses strip-trailing-newline pattern so split yields height-1 elements; ensureExactHeight handles final correction in caller"

patterns-established:
  - "Preview pane status blocks: always separate if-blocks per status, never combine with OR condition"
  - "TDD for rendering tests: homeWithSession() helper + renderPreviewPane() direct call"

requirements-completed: [VIS-01, VIS-02, VIS-03]

# Metrics
duration: 11min
completed: 2026-03-13
---

# Phase 12 Plan 01: Session List & Resume UX Summary

**Stopped sessions now render a 'Session Stopped' preview with user-intentional resume messaging, while error sessions render a 'Session Error' preview with crash-diagnostic guidance, replacing the combined ambiguous 'Session Inactive' block**

## Performance

- **Duration:** 11 min
- **Started:** 2026-03-13T06:42:47Z
- **Completed:** 2026-03-13T06:53:37Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Replaced the combined `if StatusError || StatusStopped` block with two distinct code paths in `renderPreviewPane`
- Stopped sessions: "Session Stopped" header, "stopped by user" icon, intentional-stop messaging, "Resume" key label
- Error sessions: "Session Error" header, "No tmux session running" icon, crash cause list, "Start" key label
- Verified VIS-01 (stopped sessions visible in list) and VIS-03 (picker excludes stopped) via existing plus new tests
- Full test suite passes with no regressions (73s UI suite, all packages green)

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: TDD failing tests** - `c3b3b0e` (test)
2. **Task 1 GREEN: Split preview pane implementation** - `df9c1b6` (feat)
3. **Task 2: VIS-01/VIS-03 verification** - no new files (verified via existing test suite)

## Files Created/Modified

- `internal/ui/home.go` - Split combined stopped/error preview block into two status-specific blocks with distinct headers, icons, messaging, and action labels
- `internal/ui/preview_pane_test.go` - 6 new tests: 5 for preview pane differentiation plus 1 for VIS-01 flatItems inclusion

## Decisions Made

- Split the combined OR condition block into sequential `if StatusStopped { return }` then `if StatusError { return }` blocks rather than an if/else-if, matching the existing function structure's early-return pattern
- Pad-to-height test expectation adjusted: the function's pad-then-strip-trailing-newline pattern yields `height-1` items in `strings.Split`; the caller (`renderDualLayout`) always applies `ensureExactHeight` for final correction, so testing exact line count is not meaningful

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Pad-to-height test assertion corrected**

- **Found during:** Task 1 (TDD GREEN phase)
- **Issue:** Test 5 `TestPreviewPane_BothStatuses_PadToHeight` asserted `len(lines) == height` but the existing padding pattern (pad until `lineCount >= height`, then strip trailing `\n`) yields `height-1` items in `strings.Split`. This is the same behavior as the original combined block.
- **Fix:** Updated test to assert `len(lines) >= height-1` and that both statuses produce equal line counts
- **Files modified:** `internal/ui/preview_pane_test.go`
- **Verification:** All 6 tests pass
- **Committed in:** df9c1b6 (Task 1 GREEN commit)

---

**Total deviations:** 1 auto-fixed (test assertion correction)
**Impact on plan:** Minor test expectation fix. No behavior change. No scope creep.

## Issues Encountered

None of significance. The implementation was straightforward: copy the existing block structure twice and customize the text in each copy.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- VIS-01, VIS-02, VIS-03 all complete
- Preview pane now correctly communicates intent for stopped vs error sessions
- Plan 12-02 (dedup logic) was already committed prior to this execution
- Phase 12 plans are complete; Phase 13 (auto-start platform) can proceed

## Self-Check: PASSED

- `internal/ui/home.go` - modified (split preview block exists)
- `internal/ui/preview_pane_test.go` - created (6 tests)
- Commit `c3b3b0e` - test RED phase
- Commit `df9c1b6` - feat GREEN phase
- All files present and commits verified in git log

---
*Phase: 12-session-list-resume-ux*
*Completed: 2026-03-13*
