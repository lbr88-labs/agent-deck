---
phase: 14-detection-sandbox
plan: 01
subsystem: session
tags: [tmux, sandbox, docker, session-id, claude, gemini, opencode, codex, tdd]

# Dependency graph
requires: []
provides:
  - "All command builders in instance.go emit shell strings with zero tmux set-environment calls"
  - "New Claude sessions use Go-pre-generated UUIDs via generateUUID() instead of shell $(uuidgen)"
  - "Host-side SetEnvironment() calls in Start() and StartWithMessage() propagate CLAUDE_SESSION_ID, GEMINI_SESSION_ID, GEMINI_YOLO_MODE"
  - "SyncSessionIDsToTmux() called in Restart() fallback path after session start"
  - "sandbox_env_test.go verifies all command builders are clean"
affects: [testing, sandbox, docker-session-reliability, phase-16-testing]

# Tech tracking
tech-stack:
  added: [generateUUID() helper using crypto/rand for UUID v4 (no external lib added)]
  patterns:
    - "Host-side SetEnvironment pattern: all tool session IDs propagated via tmuxSession.SetEnvironment() after Start(), not in shell strings"
    - "Go-side UUID generation: generateUUID() using crypto/rand for UUID v4, replacing shell $(uuidgen | tr '[:upper:]' '[:lower:]')"
    - "TDD RED/GREEN: sandbox_env_test.go written first to fail, then command builders fixed"

key-files:
  created:
    - internal/session/sandbox_env_test.go
  modified:
    - internal/session/instance.go
    - internal/session/instance_test.go
    - internal/session/opencode_test.go
    - internal/session/fork_integration_test.go

key-decisions:
  - "Apply universally (not conditionally on IsSandboxed()) — host-side SetEnvironment is idempotent for non-sandbox sessions and eliminates divergent code paths"
  - "Use crypto/rand directly for UUID v4 generation rather than exec.Command(uuidgen) to avoid subprocess overhead and sandbox failures"
  - "Store pre-generated forkUUID on forked Instance.ClaudeSessionID immediately so it is available for host-side SetEnvironment on Start()"

patterns-established:
  - "Command builder pattern: shell strings never call tmux set-environment; session IDs propagated post-start via Go SetEnvironment() calls"
  - "UUID generation pattern: generateUUID() for Claude session IDs, literal values embedded in --session-id flag"

requirements-completed: [DET-01]

# Metrics
duration: 26min
completed: 2026-03-13
---

# Phase 14 Plan 01: Remove tmux set-environment from Command Builders Summary

**Host-side SetEnvironment replaces all 15 embedded tmux set-environment shell calls, with Go-side UUID generation for new Claude and fork sessions, fixing Docker sandbox environment propagation (#266)**

## Performance

- **Duration:** ~26 min
- **Started:** 2026-03-13T07:10:00Z
- **Completed:** 2026-03-13T07:36:28Z
- **Tasks:** 1 (TDD RED/GREEN cycle)
- **Files modified:** 5

## Accomplishments
- Removed all 15 embedded `tmux set-environment` calls from shell command strings across buildClaudeCommandWithMessage, buildGeminiCommand, buildOpenCodeCommand, buildCodexCommand, buildGenericCommand, buildClaudeResumeCommand, BuildForkCommand, BuildOpenCodeForkCommand, and all Restart() paths
- Added `generateUUID()` helper using `crypto/rand` for UUID v4; new Claude sessions and fork sessions now embed literal UUIDs in `--session-id` flags instead of `$session_id` shell variables
- Added host-side `SetEnvironment` calls in both `Start()` and `StartWithMessage()` for CLAUDE_SESSION_ID, GEMINI_SESSION_ID, GEMINI_YOLO_MODE; `SyncSessionIDsToTmux()` added to Restart() fallback path
- Created `sandbox_env_test.go` with 11 tests verifying all builders produce clean output (9 no-tmux-set-environment, 1 no-2>/dev/null suppressor, 1 literal UUID)
- Updated 4 existing tests that relied on old inline tmux set-environment behavior

## Task Commits

1. **Task 1: Remove tmux set-environment from all command builders** - `2e7f1d7` (feat)

## Files Created/Modified
- `internal/session/sandbox_env_test.go` - New test file: 11 tests verifying no command builder embeds tmux set-environment in shell output
- `internal/session/instance.go` - All 15 call sites removed; generateUUID() added; host-side SetEnvironment added to Start(), StartWithMessage(), Restart()
- `internal/session/instance_test.go` - Updated TestBuildGeminiCommand to expect host-side propagation instead of inline tmux set-environment
- `internal/session/opencode_test.go` - Updated "Resume with existing session ID" test case to not expect tmux set-environment
- `internal/session/fork_integration_test.go` - Updated TestForkFlow_Integration to expect Go-generated UUID and no tmux set-environment in fork command

## Decisions Made
- Apply universally (not conditionally on IsSandboxed()) because host-side SetEnvironment is idempotent for non-sandbox sessions and divergent code paths would be a maintenance burden
- Use `crypto/rand` directly for UUID v4 generation rather than `exec.Command("uuidgen")` to avoid subprocess overhead and Docker sandbox failures
- Store pre-generated `forkUUID` on `target.ClaudeSessionID` immediately so the value is available when `Start()` calls host-side SetEnvironment

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated 4 pre-existing tests that asserted old tmux set-environment behavior**
- **Found during:** Task 1 (GREEN phase)
- **Issue:** instance_test.go (TestBuildGeminiCommand), opencode_test.go (TestOpenCodeBuildCommand resume case), fork_integration_test.go (TestForkFlow_Integration) all had assertions requiring tmux set-environment in command output, which would fail after the fix
- **Fix:** Updated assertions to verify the absence of tmux set-environment (the correct behavior after fix)
- **Files modified:** internal/session/instance_test.go, internal/session/opencode_test.go, internal/session/fork_integration_test.go
- **Verification:** All previously failing tests now pass; full suite passes (52s, race detector)
- **Committed in:** 2e7f1d7 (part of task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1: existing tests asserted old incorrect behavior)
**Impact on plan:** Required for test suite to pass. The tests themselves were incorrect under the new behavior, not bugs in the implementation.

## Issues Encountered
- `sandbox_env_test.go` was deleted between initial Write and first test run (macOS filesystem timing); recreated via Write tool and persisted correctly on second attempt
- A prior agent session had partially modified the file (added generateUUID, updated fork_integration_test.go); these were incorporated without conflict

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 14-01 complete: all command builders are sandbox-safe (no tmux set-environment in shell strings)
- Plan 14-02 (OpenCode question tool detection) is independent and can proceed
- Phase 16 (Comprehensive Testing) should include regression tests for this fix in sandbox context

---
*Phase: 14-detection-sandbox*
*Completed: 2026-03-13*

## Self-Check: PASSED

- sandbox_env_test.go: FOUND
- 14-01-SUMMARY.md: FOUND
- Commit 2e7f1d7: FOUND
- tmux set-environment in shell strings: 0 (4 in comments only)
