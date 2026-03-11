---
phase: 07-send-reliability
plan: 01
subsystem: send
tags: [tmux, prompt-detection, retry-logic, codex, send-verification]

# Dependency graph
requires: []
provides:
  - "Consolidated send verification package (internal/send) with 7 exported functions"
  - "Hardened Enter retry loop with aggressive early-window nudging"
  - "Codex readiness gating via PromptDetector in both CLI and Instance paths"
affects: [07-02-PLAN]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared package for duplicated logic (internal/send for prompt detection)"
    - "Aggressive early retry with exponential fallback (every iter for 5, then every 2nd)"

key-files:
  created:
    - internal/send/send.go
    - internal/send/send_test.go
  modified:
    - cmd/agent-deck/session_cmd.go
    - cmd/agent-deck/session_send_test.go
    - internal/session/instance.go
    - internal/session/instance_test.go

key-decisions:
  - "Used session_cmd.go as source of truth for consolidated functions (both copies were identical)"
  - "Codex readiness uses existing PromptDetector rather than inline string checks for consistency"
  - "waitForAgentReady Codex test deferred to integration (concrete *tmux.Session not mockable)"

patterns-established:
  - "internal/send package: single source of truth for prompt detection shared between CLI and Instance send paths"

requirements-completed: [SEND-01, SEND-02]

# Metrics
duration: 12min
completed: 2026-03-07
---

# Phase 7 Plan 1: Send Reliability Summary

**Consolidated 7 duplicated prompt detection functions into internal/send package, hardened Enter retry from every-3rd to every-iteration early window, and gated Codex sends on prompt readiness**

## Performance

- **Duration:** 12 min
- **Started:** 2026-03-06T19:12:12Z
- **Completed:** 2026-03-06T19:24:47Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Eliminated code duplication: 7 prompt detection functions now exist in exactly one location (internal/send/send.go)
- Enter retry is more aggressive in the critical early window (every iteration for first 5 retries, then every 2nd)
- Codex sessions are gated on codex> prompt visibility before text delivery in both CLI and Instance paths
- Comprehensive test coverage: 8 new tests in internal/send, 2 new tests for retry behavior, all existing tests updated

## Task Commits

Each task was committed atomically:

1. **Task 1: Consolidate send verification into internal/send package** - `6e1ccef` (feat)
2. **Task 2: Harden Enter retry loop and add Codex readiness detection** - `0b04b82` (fix)

_Note: Task 1 was TDD; RED phase could not be separately committed due to pre-commit go vet hook._

## Files Created/Modified
- `internal/send/send.go` - Consolidated send verification package with 7 exported functions
- `internal/send/send_test.go` - Comprehensive tests for all 7 functions
- `cmd/agent-deck/session_cmd.go` - Removed duplicates, updated to use send.*, hardened retry, added Codex readiness
- `cmd/agent-deck/session_send_test.go` - Removed migrated tests, updated expectations, added 2 new tests
- `internal/session/instance.go` - Removed duplicates, updated to use send.*, hardened retry, added Codex readiness
- `internal/session/instance_test.go` - Removed migrated tests (now in internal/send)

## Decisions Made
- Used session_cmd.go as source of truth for consolidated functions (both copies were byte-identical, no divergence to resolve)
- Codex readiness check uses existing PromptDetector("codex") for consistent detection rather than inline string checks
- waitForAgentReady Codex unit test deferred to integration (Plan 02) because it takes a concrete *tmux.Session that cannot be mocked

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Removed duplicate tests from instance_test.go**
- **Found during:** Task 1 (consolidation)
- **Issue:** instance_test.go had TestHasUnsentComposerPrompt and TestCurrentComposerPrompt_UsesBottomComposerBlock referencing removed private functions, causing go vet failure
- **Fix:** Removed the duplicate tests (equivalent coverage exists in internal/send/send_test.go)
- **Files modified:** internal/session/instance_test.go
- **Verification:** go vet passes, all tests pass
- **Committed in:** 6e1ccef (Task 1 commit)

**2. [Rule 1 - Bug] Updated existing test expectations for new retry cadence**
- **Found during:** Task 2 (retry hardening)
- **Issue:** TestSendWithRetryTarget_DetectsPasteMarkerAfterInitialWaiting expected 1 SendEnter call but aggressive early nudge now fires on retry 0, making it 2
- **Fix:** Updated expected count from 1 to 2 with explanation of the additional early nudge
- **Files modified:** cmd/agent-deck/session_send_test.go
- **Verification:** All tests pass
- **Committed in:** 0b04b82 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both auto-fixes necessary for correctness. No scope creep.

## Issues Encountered
- Pre-commit hook (go vet) prevents committing test files that reference undefined functions, so TDD RED phase could not be committed separately from GREEN. This is expected behavior with strict hooks.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- internal/send package ready for Plan 02 integration test coverage
- Codex readiness gating ready for integration testing with real tmux
- All existing tests pass with -race flag

## Self-Check: PASSED

- FOUND: internal/send/send.go
- FOUND: internal/send/send_test.go
- FOUND: commit 6e1ccef
- FOUND: commit 0b04b82
- FOUND: 07-01-SUMMARY.md

---
*Phase: 07-send-reliability*
*Completed: 2026-03-07*
