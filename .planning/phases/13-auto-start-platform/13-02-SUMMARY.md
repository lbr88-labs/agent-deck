---
phase: 13-auto-start-platform
plan: 02
subsystem: session
tags: [tmux, session-id, resume, stop, claude, gemini, opencode, codex]

# Dependency graph
requires:
  - phase: 13-01
    provides: pane-ready wait and UUID generation for auto-start reliability
provides:
  - SyncSessionIDsFromTmux method that snapshots all four tool session IDs from tmux env before session destruction
  - handleSessionStop calls SyncSessionIDsFromTmux before Kill so conversation IDs are never lost
  - 7 tests covering nil-safe, no-overwrite, overwrite, all-tools, and real stop-path data flow
affects:
  - 13-03
  - 16-comprehensive-testing

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Reverse sync: read tmux env INTO Instance fields before destroying the tmux session"
    - "No-op on nil or non-existent tmux session — safe to call unconditionally"
    - "Only updates Instance fields when tmux env has a non-empty value (no blanking)"

key-files:
  created:
    - internal/session/instance_platform_test.go
  modified:
    - internal/session/instance.go
    - cmd/agent-deck/session_cmd.go

key-decisions:
  - "Restart path (handleSessionRestart) does not need SyncSessionIDsFromTmux: inst.Restart() uses respawn-pane atomically and never destroys the tmux session before saving IDs"
  - "SyncSessionIDsFromTmux placed immediately after SyncSessionIDsToTmux in instance.go for symmetry"
  - "ClaudeDetectedAt is set only when it was previously zero — updating an already-set timestamp would break fork eligibility calculations"

patterns-established:
  - "Stop path pattern: SyncSessionIDsFromTmux -> Kill -> saveSessionData (always sync before destroy)"

requirements-completed: [PLAT-02]

# Metrics
duration: 12min
completed: 2026-03-13
---

# Phase 13 Plan 02: SyncSessionIDsFromTmux Summary

**SyncSessionIDsFromTmux snapshots all four tool conversation IDs from tmux env before Kill, fixing lost-ID resume failures on slow WSL2/Linux machines where PostStartSync times out**

## Performance

- **Duration:** 12 min
- **Started:** 2026-03-13T07:48:36Z
- **Completed:** 2026-03-13T08:00:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Added `SyncSessionIDsFromTmux()` method to `Instance` as the reverse of `SyncSessionIDsToTmux()`: reads CLAUDE_SESSION_ID, GEMINI_SESSION_ID, OPENCODE_SESSION_ID, CODEX_SESSION_ID from tmux env into Instance fields
- Wired the call into `handleSessionStop` immediately before `inst.Kill()`, ensuring IDs are captured even when `PostStartSync` timed out during session start
- Added 7 targeted tests (nil-safe, no-overwrite, overwrite-with-new, all-tools, real tmux stop-path data flow) all passing under race detector

## Task Commits

Each task was committed atomically:

1. **Task 1: Add SyncSessionIDsFromTmux method with tests** - `7cd993a` (feat + test)
2. **Task 2: Wire SyncSessionIDsFromTmux into stop path** - `809bbd4` (feat)

## Files Created/Modified

- `internal/session/instance.go` - Added `SyncSessionIDsFromTmux()` method after `SyncSessionIDsToTmux()`
- `internal/session/instance_platform_test.go` - 7 new tests covering all behaviors
- `cmd/agent-deck/session_cmd.go` - `handleSessionStop` calls `SyncSessionIDsFromTmux()` before `Kill()`

## Decisions Made

- Restart path does not need the same fix: `inst.Restart()` uses `respawn-pane` atomically and never destroys the tmux session before the ID is accessible, so the race condition does not exist there
- `ClaudeDetectedAt` is only set when zero to preserve existing timestamps from `PostStartSync`, which matters for fork eligibility checks downstream
- Method placed immediately after `SyncSessionIDsToTmux` in `instance.go` to make the symmetric relationship obvious at code-review time

## Deviations from Plan

None — plan executed exactly as written. The restart path inspection confirmed it does not have the kill-before-save pattern, consistent with the plan's "review to determine if this is needed" note.

## Issues Encountered

`TestWaitForPaneReady_RealTmux` in `internal/tmux` showed a flaky failure (5-second timeout under load) during the full test suite run. Confirmed pre-existing: the test passes when run in isolation and is unrelated to changes in this plan.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- PLAT-02 is resolved. Tool conversation IDs are now durably captured at stop time.
- Ready for Phase 13-03 (if present) or Phase 16 comprehensive testing.

---
*Phase: 13-auto-start-platform*
*Completed: 2026-03-13*
