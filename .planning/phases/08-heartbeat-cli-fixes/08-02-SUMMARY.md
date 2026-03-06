---
phase: 08-heartbeat-cli-fixes
plan: 02
subsystem: cli
tags: [tmux, session-death, flag-parsing, help-text, waitForCompletion]

# Dependency graph
requires:
  - phase: 07-send-reliability
    provides: sendWithRetry and waitForCompletion infrastructure
provides:
  - Session death detection in waitForCompletion (consecutiveErrors counter)
  - Verified -c and -g flag co-parsing with tests
  - Improved --no-parent and set-parent help text
affects: [conductor, cli]

# Tech tracking
tech-stack:
  added: []
  patterns: [consecutive-error-threshold for session death detection]

key-files:
  created: []
  modified:
    - cmd/agent-deck/session_cmd.go
    - cmd/agent-deck/session_send_test.go
    - cmd/agent-deck/cli_utils_test.go
    - cmd/agent-deck/main.go

key-decisions:
  - "5 consecutive GetStatus errors threshold for session death detection (balances transient recovery vs fast failure)"
  - "Return ('error', nil) on session death so handleSessionSend exits with code 1 via existing exit logic"

patterns-established:
  - "Consecutive error counter pattern: track sequential failures, reset on success, trigger action at threshold"

requirements-completed: [CLI-01, CLI-02, CLI-03]

# Metrics
duration: 7min
completed: 2026-03-07
---

# Phase 8 Plan 2: CLI Fixes Summary

**Session death detection in waitForCompletion with 5-error threshold, verified -c/-g flag co-parsing, and improved --no-parent/set-parent help text**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-06T20:21:51Z
- **Completed:** 2026-03-06T20:28:52Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- waitForCompletion now detects session death after 5 consecutive GetStatus errors and returns "error" status instead of hanging indefinitely
- Added test coverage proving -c and -g flags parse correctly together in all argument orderings (4 table-driven test cases)
- Improved help text: --no-parent mentions set-parent recovery, set-parent mentions --no-parent compatibility, -c shorthand documented in examples

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Add failing tests** - `ccf45d9` (test)
2. **Task 1 (GREEN): Implement session death detection** - `e248698` (feat)
3. **Task 2: Improve help text** - `3bada75` (feat)

_Note: Task 1 used TDD flow (RED then GREEN commits)_

## Files Created/Modified
- `cmd/agent-deck/session_cmd.go` - Added consecutiveErrors counter to waitForCompletion, expanded set-parent help text
- `cmd/agent-deck/session_send_test.go` - Added TestWaitForCompletion_SessionDeath and TestWaitForCompletion_TransientRecovery
- `cmd/agent-deck/cli_utils_test.go` - Added TestReorderArgsForFlagParsing_CmdAndGroup with 4 table-driven cases
- `cmd/agent-deck/main.go` - Expanded --no-parent help text, added -c shorthand example
- `internal/ui/home.go` - Fixed pre-existing unused variable (vet blocker)

## Decisions Made
- Used 5 as the consecutive error threshold: high enough to tolerate brief tmux hiccups, low enough to detect actual session death within ~10 seconds of polling
- Return ("error", nil) rather than a new error type, leveraging the existing handleSessionSend exit code logic that maps "error" status to exit code 1

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed pre-existing unused variable in internal/ui/home.go**
- **Found during:** Task 1 (GREEN commit)
- **Issue:** `profile` variable declared but never used at home.go:2174, causing `go vet` failure on pre-commit hook
- **Fix:** Changed `profile :=` assignment to `_ =` to suppress vet error
- **Files modified:** internal/ui/home.go
- **Verification:** Pre-commit hooks pass, all tests pass
- **Committed in:** e248698 (part of Task 1 GREEN commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Pre-existing vet error blocked commits. Fix was minimal (unused variable suppression). No scope creep.

## Issues Encountered
None beyond the pre-existing vet error documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 8 (heartbeat-cli-fixes) is now complete with both plans done
- Ready for Phase 9 (process stability) to investigate exit 137 issues

---
*Phase: 08-heartbeat-cli-fixes*
*Completed: 2026-03-07*

## Self-Check: PASSED
- All 5 files verified present
- All 3 commits verified in git history
- Full test suite passes (0 failures across all packages)
