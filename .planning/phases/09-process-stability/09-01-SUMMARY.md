---
phase: 09-process-stability
plan: 01
subsystem: investigation
tags: [exit-137, sigkill, tmux, send-keys, claude-code, process-management]

# Dependency graph
requires:
  - phase: 07-send-reliability
    provides: "Consolidated send verification package and hardened retry logic"
provides:
  - "Root cause analysis: exit 137 caused by Claude Code killing Bash tool children on new input"
  - "Fixability determination: NOT fixable in agent-deck, Claude Code design choice"
  - "6 documented mitigation strategies including --wait flag and status gating"
affects: [09-02-PLAN]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Investigation methodology: code trace + production LEARNINGS evidence + control test"

key-files:
  created:
    - .planning/phases/09-process-stability/09-INVESTIGATION.md
  modified: []

key-decisions:
  - "Exit 137 root cause attributed to Claude Code, not tmux or agent-deck, based on signal chain analysis and control test"
  - "Fixability: NOT fixable in agent-deck because tmux send-keys is the only communication channel and is indistinguishable from human typing"
  - "Primary mitigation: always gate sends on status=waiting (already implemented in waitForAgentReady)"

patterns-established:
  - "Conductor discipline: use --wait flag for sequential message delivery to prevent SIGKILL races"

requirements-completed: [PROC-01]

# Metrics
duration: 8min
completed: 2026-03-07
---

# Phase 9 Plan 1: Exit 137 Investigation Summary

**Root cause analysis tracing exit 137 (SIGKILL) through agent-deck -> tmux -> PTY -> Claude Code signal chain, with fixability determination and 6 mitigation strategies**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-06T20:58:08Z
- **Completed:** 2026-03-06T21:06:00Z
- **Tasks:** 1
- **Files created:** 1

## Accomplishments
- Traced the full signal chain from agent-deck CLI through tmux send-keys, PTY I/O, to Claude Code's process management layer
- Identified Claude Code as the component responsible for sending SIGKILL to Bash tool children when new PTY input arrives
- Confirmed via control test methodology that tmux send-keys does NOT send signals (pure PTY byte injection)
- Documented 6 practical mitigation strategies, noting that the most important one (waitForAgentReady) is already implemented
- Corroborated findings with production LEARNINGS data from ryan conductor (10+ recurrences)

## Task Commits

Each task was committed atomically:

1. **Task 1: Trace the signal chain and reproduce exit 137** - `7b7f622` (docs)

## Files Created/Modified
- `.planning/phases/09-process-stability/09-INVESTIGATION.md` - Root cause analysis with reproduction steps, signal chain trace, fixability determination, and 6 mitigation strategies

## Decisions Made
- Attributed root cause to Claude Code (not tmux or agent-deck) based on: (1) tmux send-keys is pure PTY I/O with no signal sending, (2) control test shows raw shell does not kill processes on send-keys, (3) production LEARNINGS consistently report exit 137 only when new messages arrive during tool execution
- Determined NOT fixable in agent-deck because the only communication channel (tmux PTY) is indistinguishable from human typing, and Claude Code's interrupt-on-input behavior is by design
- Identified that primary mitigation (waitForAgentReady status gating) is already implemented, validating Phase 7's send reliability work

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Investigation complete with clear determination: not fixable in agent-deck, mitigations documented
- Plan 02 can proceed with implementing any additional mitigation strategies identified
- PROC-01 requirement satisfied: root cause identified with evidence, fixability determined with justification

## Self-Check: PASSED

- FOUND: .planning/phases/09-process-stability/09-INVESTIGATION.md
- FOUND: commit 7b7f622

---
*Phase: 09-process-stability*
*Completed: 2026-03-07*
