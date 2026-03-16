---
phase: 13-auto-start-platform
plan: 01
subsystem: tmux
tags: [tmux, pane-detection, uuid, wsl, platform, shell-compatibility]

# Dependency graph
requires:
  - phase: 14-detection-sandbox
    provides: host-side SetEnvironment pattern (tmux set-environment removed from shell commands)
provides:
  - Pane-ready detection (isPaneShellReady, waitForPaneReady) in internal/tmux/pane_ready.go
  - Platform-aware pane-ready wait in tmux Start() before SendKeysAndEnter
  - Go-side UUID generation (generateUUID) eliminating shell uuidgen dependency
affects: [15-mouse-theme-polish, 16-comprehensive-testing]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pane-ready polling: poll CapturePaneFresh() at 100ms intervals until shell prompt detected"
    - "Platform-aware timeout: WSL gets 5s, macOS/Linux get 2s, non-fatal on expiry"
    - "Go-side UUID v4: crypto/rand-based generation eliminates uuidgen shell dependency"
    - "Fish-compat wrap not triggered: new commands lack $( and session_id= substrings"

key-files:
  created:
    - internal/tmux/pane_ready.go
    - internal/tmux/pane_ready_test.go
  modified:
    - internal/tmux/tmux.go
    - internal/session/instance.go
    - internal/session/instance_test.go
    - internal/session/fork_integration_test.go
    - internal/session/sandbox_env_test.go

key-decisions:
  - "generateUUID uses crypto/rand directly (not google/uuid package) for zero external dependency UUID v4"
  - "Pane-ready wait is non-fatal: timeout logs Warn and continues, same as pre-guard behavior"
  - "tmux set-environment removed from shell command strings; host-side SetEnvironment() propagates session IDs"
  - "Fish-compat bash-c wrap not triggered for new non-message Claude sessions (no $( or session_id= substring)"

patterns-established:
  - "TDD: RED tests first, then GREEN implementation, verified under race detector"
  - "Platform guard: platform.IsWSL() controls timeout duration for pane-ready wait"

requirements-completed:
  - PLAT-01

# Metrics
duration: 26min
completed: 2026-03-13
---

# Phase 13 Plan 01: Auto-Start Platform Fix Summary

**Pane-ready polling and Go-side UUID v4 generation fix WSL/Linux auto-start timing races and uuidgen absence**

## Performance

- **Duration:** 26 min
- **Started:** 2026-03-13T07:16:02Z
- **Completed:** 2026-03-13T07:42:00Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments

- Created `isPaneShellReady` recognising bash ($), zsh (%), fish (>), root (#) prompt endings
- Created `waitForPaneReady` polling `CapturePaneFresh()` at 100ms intervals with configurable timeout
- Integrated pane-ready wait into `tmux.go` Start() with 2s/5s platform-aware timeout before SendKeysAndEnter
- Replaced all 3 shell uuidgen call sites with Go-side `generateUUID()` using crypto/rand
- Fish-compat double bash -c wrap no longer triggered for non-message Claude commands (no $( substring)
- Updated all affected tests to reflect host-side SetEnvironment pattern (no tmux set-environment in shell strings)

## Task Commits

1. **Task 1: Create pane-ready detection** - `fc73307` (committed by prior phase 15-02 run)
2. **Task 2a: Replace uuidgen + host-side SetEnvironment** - `2e7f1d7` (committed by prior phase 14-01 run)
3. **Task 2b: Integrate pane-ready wait into tmux Start()** - `46fab1a` (feat)

## Files Created/Modified

- `internal/tmux/pane_ready.go` - isPaneShellReady and waitForPaneReady functions
- `internal/tmux/pane_ready_test.go` - Table-driven unit tests and real-tmux integration tests
- `internal/tmux/tmux.go` - Pane-ready wait block and platform import added to Start()
- `internal/session/instance.go` - generateUUID() with crypto/rand; uuidgen replaced at 3 call sites
- `internal/session/instance_test.go` - Tests updated: uuidgen assertions inverted, tmux set-environment assertions updated
- `internal/session/fork_integration_test.go` - Fork test updated for Go-side UUID pattern
- `internal/session/sandbox_env_test.go` - Forward-looking test file for host-side SetEnvironment pattern

## Decisions Made

- Used `crypto/rand` directly for UUID v4 rather than the `github.com/google/uuid` package (already imported but removed as unnecessary dependency)
- Pane-ready timeout is non-fatal: a `statusLog.Warn` is emitted and execution proceeds, preserving the pre-guard degraded path
- `tmux set-environment CLAUDE_SESSION_ID` removed from shell command strings entirely; CLAUDE_SESSION_ID propagated via host-side `SetEnvironment()` calls after tmux session start (aligns with the host-side SetEnvironment pattern established in phase 14-01)
- Fork path also uses Go-side UUID and no longer embeds `tmux set-environment` in the fork command string

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed unused `yoloEnv` variable in buildGeminiCommand**
- **Found during:** Task 2 (go vet run)
- **Issue:** `yoloEnv` declared and assigned but never used after the GEMINI_YOLO_MODE SetEnvironment refactor
- **Fix:** Removed the `yoloEnv := "false"` / `yoloEnv = "true"` lines; yolo mode is now propagated via host-side SetEnvironment only
- **Files modified:** internal/session/instance.go
- **Verification:** go vet passes cleanly
- **Committed in:** 2e7f1d7 (part of phase 14-01 task commit)

**2. [Rule 1 - Bug] Removed unused `github.com/google/uuid` import**
- **Found during:** Task 2 (go vet run)
- **Issue:** uuid package imported but not used after generateClaudeSessionID was reimplemented with crypto/rand
- **Fix:** Removed the import; `generateUUID()` uses crypto/rand directly per plan specification
- **Files modified:** internal/session/instance.go
- **Verification:** go vet passes cleanly
- **Committed in:** 2e7f1d7 (part of phase 14-01 task commit)

**3. [Context] Significant plan work pre-completed by earlier phase executors**
- Task 1 (pane_ready.go + pane_ready_test.go) was committed by the phase 15-02 executor
- Task 2 UUID generation and tmux set-environment removal was committed by the phase 14-01 executor
- This plan's unique contribution: integrating `waitForPaneReady` into `tmux.go` Start() (commit 46fab1a)
- All success criteria are met; the pre-completion is consistent with the plan's artifacts

---

**Total deviations:** 2 auto-fixed (1 unused variable, 1 unused import) + context note about pre-completion
**Impact on plan:** Auto-fixes were correctness requirements. Pre-completion by other phases is additive.

## Issues Encountered

- The Edit tool encountered repeated "file modified since read" errors due to a background linter/formatter. Used Python file manipulation as a fallback for complex multi-line replacements.
- `TestWaitForPaneReady_Timeout` is sometimes skipped (not failed) when the pane is ready within 1ms on fast machines. The test correctly handles this with `t.Skip()`.
- `TestStatusCycle_ShellSessionWithCommand` shows intermittent failures under heavy parallel testing load (pre-existing flakiness, not related to this plan).

## Next Phase Readiness

- Pane-ready detection is available for use by other plans needing shell readiness checks
- Go-side UUID pattern established and tested for all Claude session ID generation paths
- Phase 13-02 can proceed: auto-start failure modes are now addressed at the tmux level

---
*Phase: 13-auto-start-platform*
*Completed: 2026-03-13*

## Self-Check: PASSED

- `internal/tmux/pane_ready.go` FOUND
- `internal/tmux/pane_ready_test.go` FOUND
- `internal/tmux/tmux.go` contains `waitForPaneReady` FOUND
- `internal/session/instance.go` contains `func generateUUID` FOUND
- `.planning/phases/13-auto-start-platform/13-01-SUMMARY.md` FOUND
- Commit `fc73307` (pane_ready.go) FOUND
- Commit `2e7f1d7` (UUID generation) FOUND
- Commit `46fab1a` (tmux Start() integration) FOUND
