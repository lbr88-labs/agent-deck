---
phase: 06-conductor-pipeline-edge-cases
plan: 01
subsystem: testing
tags: [tmux, conductor, heartbeat, chunked-send, integration-tests]

# Dependency graph
requires:
  - phase: 05-status-detection-events
    provides: "Conductor send and event tests (COND-01, COND-02)"
provides:
  - "Heartbeat round-trip integration test (COND-03)"
  - "Chunked send delivery test for >4KB payloads (COND-04)"
  - "Small send delivery test for <4KB payloads (COND-04)"
affects: [06-conductor-pipeline-edge-cases]

# Tech tracking
tech-stack:
  added: []
  patterns: [sentinel-based delivery verification, multi-line chunked payloads]

key-files:
  created: []
  modified:
    - internal/integration/conductor_test.go

key-decisions:
  - "Multi-line chunked payload avoids terminal line buffer overflow (canonical mode ~4096 byte limit per line)"
  - "Sentinel message after chunked send proves sequential delivery without relying on scrollback capture"

patterns-established:
  - "Sentinel verification: send a small marker after large payload to prove complete delivery"
  - "Multi-line payloads for chunk tests: use embedded newlines to flush through terminal line discipline"

requirements-completed: [COND-03, COND-04]

# Metrics
duration: 5min
completed: 2026-03-07
---

# Phase 6 Plan 1: Conductor Pipeline Tests Summary

**Heartbeat round-trip and chunked send delivery tests proving end-to-end conductor pipeline with real tmux sessions**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-06T17:12:55Z
- **Completed:** 2026-03-06T17:17:28Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments
- Heartbeat round-trip test verifies child existence check then message delivery (COND-03)
- Chunked send test proves >4KB multi-line payload delivered intact via 4096-byte chunk splitting (COND-04)
- Small send test confirms <4KB messages use direct SendKeys path without chunking (COND-04)
- All 7 conductor integration tests pass with zero data races under -race flag

## Task Commits

Each task was committed atomically:

1. **Task 1: Heartbeat round-trip test (COND-03)** - `d88815c` (test)
2. **Task 2: Chunked send delivery test (COND-04)** - `d37400f` (test)

## Files Created/Modified
- `internal/integration/conductor_test.go` - Added 3 new test functions: TestConductor_HeartbeatRoundTrip, TestConductor_ChunkedSendDelivery, TestConductor_SmallSendDelivery

## Decisions Made
- Used multi-line payloads with embedded newlines for chunk testing to avoid terminal canonical-mode line buffer overflow (single lines >4096 bytes get truncated by tty driver)
- Used sentinel message technique for chunked delivery verification: a small known string sent after the large payload proves all chunks were delivered sequentially, avoiding reliance on tmux scrollback capture

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Chunked send test payload redesign for terminal line buffer compatibility**
- **Found during:** Task 2 (Chunked send delivery test)
- **Issue:** Original plan specified a single 4100+ byte line (strings.Repeat("X", 4100)). Terminal canonical mode buffers only ~4096 bytes per line, causing cat to drop the tail of the message. Pane capture (visible area only, no scrollback) also couldn't display the full wrapped output.
- **Fix:** Redesigned payload to use 55 short lines with embedded newlines (total >4096 bytes), so each line flushes through the terminal independently. Used CHUNK-END marker on the last line to verify full delivery.
- **Files modified:** internal/integration/conductor_test.go
- **Verification:** Test passes consistently, CHUNK-END marker appears in pane
- **Committed in:** d37400f (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug in test design)
**Impact on plan:** Auto-fix necessary for correct test behavior given terminal line buffer constraints. No scope creep.

## Issues Encountered
- Pre-existing flaky test TestLifecycleStop_TerminatesSession in lifecycle_test.go (unrelated to our changes, tmux kill-session race condition)

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All COND-03 and COND-04 requirements covered
- Ready for 06-02-PLAN.md (remaining conductor edge case tests)

---
*Phase: 06-conductor-pipeline-edge-cases*
*Completed: 2026-03-07*

## Self-Check: PASSED
- conductor_test.go: FOUND
- 06-01-SUMMARY.md: FOUND
- Commit d88815c (Task 1): FOUND
- Commit d37400f (Task 2): FOUND
