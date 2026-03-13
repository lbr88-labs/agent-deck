---
phase: 15-mouse-theme-polish
plan: "02"
subsystem: ui
tags: [docker, sandbox, conductor, heartbeat, worktree, git]

# Dependency graph
requires:
  - phase: 15-00
    provides: Research and validation strategy for Phase 15 polish items
  - phase: 15-01
    provides: Mouse scroll and light theme fixes
provides:
  - README auto_cleanup documentation in Docker sandbox section
  - OS daemon detection guard in bridge.py heartbeat_loop
  - Identical daemon detection in conductor_templates.go Go template
  - Worktree reuse at all 5 CreateWorktree call sites (CLI + TUI)
affects:
  - Phase 16 testing (worktree reuse and heartbeat behaviors need regression coverage)
  - conductor setup users who deploy via conductor_templates.go template

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "GetWorktreeForBranch pre-check before CreateWorktree: check-then-create with graceful reuse"
    - "OS daemon existence check via filesystem glob in Python (launchd plist / systemd timer)"

key-files:
  created: []
  modified:
    - README.md
    - conductor/bridge.py
    - internal/session/conductor_templates.go
    - cmd/agent-deck/main.go
    - cmd/agent-deck/session_cmd.go
    - cmd/agent-deck/launch_cmd.go
    - internal/ui/home.go

key-decisions:
  - "Worktree reuse silently updates worktreePath to existing path rather than erroring, preserving downstream session WorktreePath accuracy"
  - "Heartbeat guard uses filesystem glob (not config lookup) to detect OS daemon, making it robust to profile/name mismatches"
  - "Template and live bridge.py must contain identical _os_heartbeat_daemon_installed function; drift is a must-have violation"

patterns-established:
  - "Worktree reuse pattern: GetWorktreeForBranch -> reuse if found, CreateWorktree if not"
  - "OS daemon guard pattern: check plist/timer existence at function entry before starting loop"

requirements-completed: [UX-03, UX-04, UX-05]

# Metrics
duration: 15min
completed: "2026-03-13"
---

# Phase 15 Plan 02: Documentation, Heartbeat Consolidation, and Worktree Reuse Summary

**README auto_cleanup doc, OS daemon heartbeat guard in bridge.py and Go template, and GetWorktreeForBranch pre-check at all 5 CreateWorktree call sites**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-13T07:07:09Z
- **Completed:** 2026-03-13T07:14:24Z
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments

- Added `auto_cleanup` to README Docker sandbox config example with one-line explanation of when to disable it
- Added `_os_heartbeat_daemon_installed()` helper to `conductor/bridge.py` that detects launchd plist (macOS) or systemd timer (Linux) and skips the bridge heartbeat loop when an OS daemon is already installed, preventing double-trigger
- Applied identical daemon detection to the Go template in `conductor_templates.go` so new conductor deployments get the same behavior
- Added `GetWorktreeForBranch` pre-check before all 5 `CreateWorktree` call sites: 3 CLI sites (main.go, session_cmd.go, launch_cmd.go) and 2 TUI sites (home.go session creation and fork paths)

## Task Commits

Each task was committed atomically:

1. **Task 1: Document auto_cleanup in README sandbox section** - `95db794` (docs)
2. **Task 2: Consolidate heartbeat by making bridge loop detect OS daemon** - `2c4b5b1` (feat)
3. **Task 3: Add worktree reuse detection at all CreateWorktree call sites** - `5d637c9` (feat)

## Files Created/Modified

- `README.md` - Added auto_cleanup to Docker sandbox TOML config example with explanatory sentence
- `conductor/bridge.py` - Added _os_heartbeat_daemon_installed() function and guard at heartbeat_loop entry
- `internal/session/conductor_templates.go` - Added identical _os_heartbeat_daemon_installed() function and guard in bridge template string
- `cmd/agent-deck/main.go` - GetWorktreeForBranch pre-check at CLI launch worktree creation (line 956)
- `cmd/agent-deck/session_cmd.go` - GetWorktreeForBranch pre-check at CLI session fork worktree creation (line 498)
- `cmd/agent-deck/launch_cmd.go` - GetWorktreeForBranch pre-check at CLI launch_cmd worktree creation (line 183)
- `internal/ui/home.go` - GetWorktreeForBranch pre-check at TUI session creation (line 5992) and TUI fork path (line 6254)

## Decisions Made

- Worktree reuse silently updates the `worktreePath` (or `opts.WorktreePath`) variable to the existing path rather than returning an error. This means the session's `WorktreePath` field reflects the actual on-disk location, preventing "MISSING" status in `agent-deck worktree info`.
- The heartbeat OS daemon guard uses filesystem glob (checking directory listings) rather than config lookup, making it robust to profile/name variations in daemon file names.
- Both `conductor/bridge.py` and the Go string template in `conductor_templates.go` must stay in sync. The plan's must-have artifacts enforce this as a drift check requirement.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- `go build ./...` failed with a pre-existing error in `internal/integration/harness.go:27` where `skipIfNoTmuxServer` is defined in a `_test.go` file but referenced in a non-test file. This is unrelated to Plan 02 changes and was pre-existing before any edits. All packages modified in this plan build and vet cleanly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 15 is now complete (plans 00, 01, 02 all done)
- Phase 16 (Comprehensive Testing) can begin; worktree reuse and heartbeat consolidation behaviors are candidates for regression test coverage
- The pre-existing `internal/integration/harness.go` build error should be fixed before Phase 16 integration tests are added

---
*Phase: 15-mouse-theme-polish*
*Completed: 2026-03-13*

## Self-Check: PASSED

- FOUND: README.md
- FOUND: conductor/bridge.py
- FOUND: internal/session/conductor_templates.go
- FOUND: internal/ui/home.go
- FOUND: .planning/phases/15-mouse-theme-polish/15-02-SUMMARY.md
- FOUND commit: 95db794 (Task 1)
- FOUND commit: 2c4b5b1 (Task 2)
- FOUND commit: 5d637c9 (Task 3)
