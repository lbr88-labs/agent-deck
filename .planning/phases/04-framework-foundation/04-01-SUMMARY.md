---
phase: 04-framework-foundation
plan: 01
subsystem: testing
tags: [integration-testing, tmux, sqlite, polling, test-harness, go]

# Dependency graph
requires: []
provides:
  - TmuxHarness with auto-cleanup via t.Cleanup for integration tests
  - Polling helpers (WaitForCondition, WaitForPaneContent, WaitForStatus) replacing time.Sleep
  - SQLite fixture helpers (NewTestDB, InstanceBuilder) for isolated test databases
  - TestMain with AGENTDECK_PROFILE=_test enforcement and orphan cleanup
affects: [04-02, 05-session-lifecycle, 06-conductor-orchestration]

# Tech tracking
tech-stack:
  added: []
  patterns: [TmuxHarness pattern, polling-based assertions, InstanceBuilder fluent API]

key-files:
  created:
    - internal/integration/harness.go
    - internal/integration/poll.go
    - internal/integration/fixtures.go
    - internal/integration/testmain_test.go
    - internal/integration/harness_test.go
    - internal/integration/poll_test.go
    - internal/integration/fixtures_test.go
  modified: []

key-decisions:
  - "Used dashes in inttest- prefix to survive tmux sanitizeName (converts underscores to dashes)"
  - "Defined TestingT interface for polling helpers to enable timeout testing without killing test runner"
  - "Used statedb.StateDB directly for fixtures instead of session.Storage (decoupled, all fields exported)"

patterns-established:
  - "TmuxHarness: NewTmuxHarness(t) creates harness, CreateSession() tracks sessions, t.Cleanup auto-kills"
  - "Polling: WaitForCondition with timeout/poll/desc replaces time.Sleep; WaitForPaneContent and WaitForStatus wrap it"
  - "Fixtures: NewTestDB(t) creates isolated SQLite, InstanceBuilder fluent API for test data"
  - "Integration TestMain: AGENTDECK_PROFILE=_test + cleanupIntegrationSessions targeting agentdeck_inttest- prefix"

requirements-completed: [INFRA-01, INFRA-02, INFRA-03, INFRA-04]

# Metrics
duration: 7min
completed: 2026-03-06
---

# Phase 4 Plan 01: Integration Test Infrastructure Summary

**TmuxHarness with auto-cleanup, polling helpers replacing time.Sleep, and SQLite fixture builders with InstanceBuilder for isolated test databases**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-06T11:21:23Z
- **Completed:** 2026-03-06T11:29:14Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- TmuxHarness creates real tmux sessions and automatically cleans them up via t.Cleanup (no orphans)
- WaitForCondition/WaitForPaneContent/WaitForStatus replace flaky time.Sleep assertions with deterministic polling
- NewTestDB creates isolated SQLite databases in temp directories with automatic Close on test end
- InstanceBuilder produces statedb.InstanceRow values with sensible defaults and fluent setter API
- All 13 tests pass with -race flag, zero regressions across full test suite (17 packages)

## Task Commits

Each task was committed atomically:

1. **Task 1: TmuxHarness, polling helpers, and TestMain** - `fc59882` (feat)
2. **Task 2: SQLite fixture helpers with InstanceBuilder** - `f4e519f` (feat)

## Files Created/Modified
- `internal/integration/harness.go` - TmuxHarness struct with auto-cleanup via t.Cleanup and inttest- prefix naming
- `internal/integration/poll.go` - WaitForCondition, WaitForPaneContent, WaitForStatus polling helpers
- `internal/integration/fixtures.go` - NewTestDB for isolated SQLite, InstanceBuilder fluent API
- `internal/integration/testmain_test.go` - TestMain with AGENTDECK_PROFILE=_test and orphan cleanup
- `internal/integration/harness_test.go` - Tests for create, multi-cleanup, prefix naming
- `internal/integration/poll_test.go` - Tests for condition success/timeout, pane content, status transitions
- `internal/integration/fixtures_test.go` - Tests for DB creation, isolation, builder defaults, save/load round-trip

## Decisions Made
- Used dashes in inttest- prefix because tmux sanitizeName converts underscores to dashes; using dashes ensures the prefix survives sanitization and the cleanup function matches correctly
- Defined a TestingT interface (Helper + Fatalf) so WaitForCondition can accept a mock for testing the timeout path without killing the real test
- Used statedb.StateDB directly for fixture helpers instead of session.Storage, as recommended by research (04-RESEARCH.md), keeping the integration package decoupled from session.Storage internals

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed inttest_ prefix to inttest- for tmux compatibility**
- **Found during:** Task 1 (TestHarness_PrefixNaming)
- **Issue:** Plan specified "inttest_" prefix, but tmux's sanitizeName replaces underscores with dashes, so the actual session name contained "inttest-" not "inttest_"
- **Fix:** Changed prefix to use dashes ("inttest-") so it survives tmux sanitization. Updated cleanup function to match "agentdeck_inttest-" and test assertion accordingly.
- **Files modified:** internal/integration/harness.go, internal/integration/testmain_test.go, internal/integration/harness_test.go
- **Verification:** TestHarness_PrefixNaming passes, cleanup targets correct prefix
- **Committed in:** fc59882 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Necessary correction for tmux session naming compatibility. No scope creep.

## Issues Encountered
- Pre-commit hook (`go vet`) prevents committing test files without their implementations since vet checks compilation. Resolved by combining RED and GREEN phases into a single commit per task.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Integration test infrastructure is complete and ready for Phase 4 Plan 02 (session lifecycle tests)
- All helpers (TmuxHarness, polling, fixtures) are exported and documented for use by subsequent test phases
- No blockers or concerns

## Self-Check: PASSED

- All 7 created files verified on disk
- Both task commits (fc59882, f4e519f) verified in git log

---
*Phase: 04-framework-foundation*
*Completed: 2026-03-06*
