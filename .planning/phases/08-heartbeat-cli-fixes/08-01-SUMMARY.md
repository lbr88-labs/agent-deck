---
phase: 08-heartbeat-cli-fixes
plan: 01
subsystem: conductor
tags: [heartbeat, shell-script, conductor, launchd, group-scoping]

# Dependency graph
requires:
  - phase: 06-conductor-pipeline-edge-cases
    provides: conductor heartbeat script template and daemon infrastructure
provides:
  - Group-scoped heartbeat messages that reference conductor's own group, not entire profile
  - Interval=0 semantics for disabling heartbeat
  - Enabled-config guard in heartbeat script
  - Setup guard that skips daemon installation when interval=0
affects: [08-heartbeat-cli-fixes, conductor]

# Tech tracking
tech-stack:
  added: []
  patterns: [heartbeat-enabled-guard, interval-zero-disabled-semantics]

key-files:
  created: []
  modified:
    - internal/session/conductor.go
    - internal/session/conductor_test.go
    - cmd/agent-deck/conductor_cmd.go
    - internal/ui/home.go

key-decisions:
  - "interval=0 means disabled (returns 0), negative means use default 15"
  - "Heartbeat script checks conductor enabled status via JSON before sending"
  - "TUI clear-on-compact heartbeat also updated to group-scoped message"

patterns-established:
  - "Heartbeat group scoping: messages reference {NAME} conductor group, not entire profile"
  - "Interval=0 disabled convention: 0=disabled, negative=default, positive=custom"

requirements-completed: [HB-01, HB-02]

# Metrics
duration: 4min
completed: 2026-03-07
---

# Phase 08 Plan 01: Heartbeat Fixes Summary

**Group-scoped heartbeat messages with enabled-config guard and interval=0 disabled semantics**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-06T20:21:51Z
- **Completed:** 2026-03-06T20:26:26Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Heartbeat script now messages about conductor's own group ({NAME}) instead of all sessions in the profile
- Added enabled-config guard: heartbeat script queries `conductor status --json` and exits if conductor is disabled
- `GetHeartbeatInterval` returns 0 when interval is set to 0 (disabled), 15 for negative (default)
- `handleConductorSetup` skips heartbeat daemon installation when interval is 0
- `MigrateConductorHeartbeatScripts` will auto-refresh existing scripts (uses the updated template constant)

## Task Commits

Each task was committed atomically:

1. **Task 1: Write tests for heartbeat group scoping and interval semantics** - `4f961c2` (test)
2. **Task 2: Fix heartbeat template, interval semantics, and setup guard** - `9b6d96a` (feat)

## Files Created/Modified
- `internal/session/conductor.go` - Fixed heartbeat script template (group-scoped message, enabled guard), updated GetHeartbeatInterval semantics
- `internal/session/conductor_test.go` - Added TestConductorHeartbeatScript_GroupScoped, TestGetHeartbeatInterval_ZeroMeansDisabled, updated TestGetHeartbeatInterval
- `cmd/agent-deck/conductor_cmd.go` - Added interval=0 guard in handleConductorSetup to skip daemon installation
- `internal/ui/home.go` - Updated TUI clear-on-compact heartbeat message to group-scoped format

## Decisions Made
- interval=0 means disabled (returns 0), negative means use default 15: This preserves backward compatibility for users who set positive values while adding a clear "off switch"
- Heartbeat script checks conductor enabled status via JSON API before sending: Prevents orphaned heartbeat daemons from sending messages after conductor is disabled
- TUI clear-on-compact heartbeat also updated to group-scoped message: Consistency between shell script and TUI heartbeat paths

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated existing TestGetHeartbeatInterval test case**
- **Found during:** Task 2
- **Issue:** Existing test expected GetHeartbeatInterval(0) to return 15 (old buggy behavior)
- **Fix:** Changed test expectation from {0, 15} to {0, 0} to match corrected semantics
- **Files modified:** internal/session/conductor_test.go
- **Verification:** All tests pass
- **Committed in:** 9b6d96a (part of Task 2 commit)

**2. [Rule 1 - Bug] Fixed TUI clear-on-compact heartbeat message**
- **Found during:** Task 2
- **Issue:** internal/ui/home.go:2178 had the same profile-scoped heartbeat message ("Check all sessions in the %s profile")
- **Fix:** Changed to group-scoped message format using conductorName instead of profile
- **Files modified:** internal/ui/home.go
- **Verification:** Build passes, message format consistent with script template
- **Committed in:** 9b6d96a (part of Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both fixes necessary for consistency. The existing test reflected the old buggy behavior, and the TUI had a copy of the same unscoped message. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Heartbeat fixes complete, ready for Plan 02 (CLI fixes)
- Existing heartbeat scripts will be auto-refreshed by MigrateConductorHeartbeatScripts on next setup/status call

## Self-Check: PASSED

All files verified present. Both task commits verified in git log.

---
*Phase: 08-heartbeat-cli-fixes*
*Completed: 2026-03-07*
