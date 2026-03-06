---
phase: 06-conductor-pipeline-edge-cases
plan: 02
subsystem: testing
tags: [integration-tests, skills, concurrent-polling, storage-watcher, tmux, sqlite, race-detection]

# Dependency graph
requires:
  - phase: 04-integration-test-framework
    provides: TmuxHarness, WaitForCondition, NewTestDB fixtures
provides:
  - Edge case integration tests for skills discovery/attach, concurrent tmux polling, cross-instance storage watcher
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "HOME/CLAUDE_CONFIG_DIR environment override for skills isolation in integration tests"
    - "errgroup.Group for concurrent goroutine coordination with error propagation"
    - "Dual StateDB instances on same file for cross-process simulation"

key-files:
  created:
    - internal/integration/edge_cases_test.go
  modified: []

key-decisions:
  - "Replicated setupSkillTestEnv pattern (unexported) into integration package for HOME/config isolation"
  - "12 concurrent sessions chosen to exceed typical developer workload while staying within tmux limits"
  - "2s grace period after session creation covers the 1.5s status detection startup window"

patterns-established:
  - "Skills integration test pattern: override HOME, register custom source, discover, attach, verify materialized content"
  - "Concurrent polling pattern: create N sessions, wait for existence, grace period, errgroup blast with race detector"

requirements-completed: [EDGE-01, EDGE-02, EDGE-03]

# Metrics
duration: 22min
completed: 2026-03-06
---

# Phase 06 Plan 02: Edge Cases Summary

**Skills discover-attach pipeline, 12-session concurrent polling with race detection, and cross-instance StorageWatcher verification**

## Performance

- **Duration:** 22 min
- **Started:** 2026-03-06T17:13:43Z
- **Completed:** 2026-03-06T17:36:10Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments
- TestEdge_SkillsDiscoverAttach: end-to-end skills pipeline (register source, discover, attach, read materialized SKILL.md)
- TestEdge_ConcurrentPolling: 12 real tmux sessions polled concurrently with -race flag, zero data races detected
- TestEdge_StorageWatcherCrossInstance: two separate StateDB instances on same file, watcher detects external Touch()

## Task Commits

Each task was committed atomically:

1. **Task 1: Skills discover-attach integration test (EDGE-01)** - `fe806e3` (test)
2. **Task 2: Concurrent polling and storage watcher tests (EDGE-02, EDGE-03)** - included in `fe806e3` (all 3 tests written as single cohesive file)

## Files Created/Modified
- `internal/integration/edge_cases_test.go` - Three edge case integration tests: skills pipeline, concurrent polling, cross-instance storage watcher

## Decisions Made
- Wrote all three test functions in a single file creation (Task 1 commit) since they share imports and helper functions. Task 2 verification confirms both additional tests pass.
- Replicated `setupSkillTestEnv` from `session/skills_catalog_test.go` because it is unexported and cannot be imported cross-package.
- Used `errgroup.Group` (already in go.mod as `golang.org/x/sync`) for concurrent goroutine coordination with error propagation, matching the plan's specification.

## Deviations from Plan

None - plan executed exactly as written. The only structural difference is that all three tests were created in a single commit rather than split across two, since they share the same file and import block.

## Issues Encountered
- First run of TestEdge_ConcurrentPolling failed with all sessions in error state due to leftover tmux sessions from a prior test run. The testmain cleanup function resolved this. Subsequent runs pass consistently.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- All edge case integration tests complete and passing
- Full project test suite green (`go test -race ./...`)
- Phase 06 plan 02 complete

## Self-Check: PASSED

- FOUND: internal/integration/edge_cases_test.go
- FOUND: fe806e3 (Task 1 commit)
- FOUND: 06-02-SUMMARY.md

---
*Phase: 06-conductor-pipeline-edge-cases*
*Completed: 2026-03-06*
