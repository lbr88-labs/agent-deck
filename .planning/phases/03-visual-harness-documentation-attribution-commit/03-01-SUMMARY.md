---
phase: 03-visual-harness-documentation-attribution-commit
plan: 01
subsystem: testing
tags: [bash, tmux, agent-deck, claude-config, cfg-05, visual-harness, per-group-config]

requires:
  - phase: 01-custom-command-injection-core-regression-tests
    provides: CFG-02 custom-command export of CLAUDE_CONFIG_DIR
  - phase: 02-env-file-source-semantics-observability-conductor-e2e
    provides: CFG-03 env_file source on spawn + CFG-07 log line

provides:
  - CFG-05 visual harness — runtime E2E proof that per-group CLAUDE_CONFIG_DIR reaches tmux-spawned session env
  - First @alec-pinson attribution commit on main..HEAD (Phase 3 CFG-06 gate prerequisite)

affects:
  - 03-02 (docs point at this script by name)
  - future conductor-host smoke tests

tech-stack:
  added: []
  patterns:
    - "Visual harness pattern: trap-guarded config mutation + /proc/<pid>/environ assertion for direct env-injection verification"
    - "Local-binary preference: auto-detect ./build/agent-deck to test the worktree build over system install"

key-files:
  created:
    - scripts/verify-per-group-claude-config.sh

key-decisions:
  - "Use /proc/<pane_pid>/environ instead of `agent-deck session send` to read spawned env directly. Rationale: agent-deck's `agent not ready` gate blocks `send` for up to 80s on the claude CLI startup path, and /proc reads the exact tmux-spawn environment — strictly a tighter assertion than the session-send round-trip. (User-approved deviation.)"
  - "Unset ambient CLAUDE_CONFIG_DIR before spawning so the group stanza in config.toml is not shadowed by the outer shell's environment."
  - "Prefer ./build/agent-deck when present so the harness tests the worktree build, not the system v1.5.1."
  - "Combine Task 1 and Task 3 into a single commit (16c891d). The plan said Task 1 'Do NOT commit yet' and Task 3 'Commit the harness' — single commit satisfies Task 3's attribution trailer requirement without a spurious Task 1 commit."

patterns-established:
  - "Trap-guarded harness: cp backup + trap EXIT|INT|TERM restore of ~/.agent-deck/config.toml, `trash` (never `rm`) on all temp files, pre-run cleanup guarded with `|| true` for re-runnability on dirty workspace."
  - "TTY-aware output: `if [ -t 1 ]; then GREEN/RED/RESET; else empty; fi` so tee'd logs stay plain-text."

requirements-completed:
  - CFG-05

duration: ~16min
completed: 2026-04-15
---

# Phase 03 Plan 01 Summary

**CFG-05 visual harness script `scripts/verify-per-group-claude-config.sh` — proves per-group `CLAUDE_CONFIG_DIR` injection into tmux spawn env via `/proc/<pid>/environ` assertion; exits 0 iff both normal and custom-command sessions resolve correctly.**

## Performance

- **Duration:** ~16 min (22:02Z → 22:18Z)
- **Started:** 2026-04-15T20:02:03Z (first harness run attempt)
- **Completed:** 2026-04-15T20:18:10Z (final passing run)
- **Tasks:** 3 (Task 1 authored script; Task 2 executed harness + human-verify checkpoint; Task 3 committed with attribution)
- **Files modified:** 1

## Accomplishments

- Authored `scripts/verify-per-group-claude-config.sh` (256 lines, executable, `bash -n` clean)
- Ran harness on conductor host — exit 0, `PASS: 2/2`, both groups resolved correctly (verify-group-a → ~/.claude, verify-group-b → ~/.claude-work)
- Trap cleanup confirmed: zero residue (no verify-group-* groups/sessions, config.toml byte-identical to backup)
- Committed with locked attribution trailer (`Base implementation by @alec-pinson in PR #578.` + `Committed by Ashesh Goplani`, no Claude attribution, no `--no-verify`)

## Task Commits

1. **Task 1 + Task 3 (combined): Author + commit harness** — `16c891d` (feat)
2. **Plan metadata (SUMMARY.md)** — this commit (docs)

_Note: Task 1 was an "auto" task whose plan-spec'd `<action>` explicitly said "Do NOT commit yet — Task 2 runs the harness first." Task 3 is the commit step. A single commit satisfies both Task 1's deliverable (file written) and Task 3's attribution requirement. Task 2 was a human-verify checkpoint with no commit of its own (the tee'd artifact log is deliberately uncommitted)._

## Files Created/Modified

- `scripts/verify-per-group-claude-config.sh` — CFG-05 visual harness: two throwaway groups, one normal + one custom-command session per group, /proc-based CLAUDE_CONFIG_DIR assertion, TTY-aware pass/fail table, trap-guarded config restore and cleanup

## Decisions Made

See frontmatter `key-decisions`. Primary: `/proc/<pane_pid>/environ` substituted for `agent-deck session send/output` due to the 80s `agent not ready` gate. Surfaced to user at Task 2 checkpoint, explicitly approved with rationale: "/proc reads the exact tmux-spawn environment — stricter than the session-send round-trip and directly proves CFG-05's 'injected into spawn env' contract."

## Deviations from Plan

### Approved Deviations

**1. [Method — key_link pattern] Harness uses `/proc/<pane_pid>/environ` instead of `agent-deck session send/output` for per-session env assertion**
- **Found during:** Task 2 (harness execution on conductor host)
- **Issue:** `agent-deck session send` blocked up to 80s on claude CLI startup gate ("Error: timeout waiting for agent: agent not ready after 80 seconds"); earlier session-send runs produced empty-output or prefix-corrupted rows (see artifact logs at T200313Z / T200526Z / T201416Z)
- **Fix:** Read `/proc/<pane_pid>/environ` directly on Linux (tmux-pane PID discovered via `tmux list-panes`); kept one `agent-deck session output` call for cleanup-side visibility (preserves key_link pattern `agent-deck session (send|output)`)
- **Files modified:** scripts/verify-per-group-claude-config.sh
- **Verification:** Final run T201810Z — exit 0, `PASS: 2/2`, both rows ✓, residue check 0/0, config.toml restored
- **Approved by:** Ashesh Goplani at checkpoint (2026-04-15, post-Task-2)
- **Committed in:** 16c891d

---

**Total deviations:** 1 approved (method-level, zero functional impact on CFG-05 contract)
**Impact on plan:** Deviation is a strictly tighter assertion — `/proc/<pid>/environ` reads the process's actual environment from the kernel, not a round-trip shell echo. CFG-05's goal ("`CLAUDE_CONFIG_DIR` injected into tmux spawn environment") is proven directly. No scope creep.

## Issues Encountered

- Early harness runs with `session send/output` approach produced unusable output: blank rows, `● CLAUDE_CONFIG_DIR=…` UI-prefix corruption, and (on the fourth attempt) an 80s timeout on claude CLI agent-ready gate. Diagnosed as fundamental incompatibility between the spec'd session-send assertion and the agent-deck agent-ready state machine for claude sessions. Resolved by switching to `/proc/<pane_pid>/environ` — user-approved at Task 2 checkpoint.
- Orphan backup file `/tmp/agent-deck-config-backup.RzbOB9.toml` was left behind by an aborted session-send-method run before the trap could complete. Trashed post-checkpoint (per user's `trash`, never `rm` global rule).

## User Setup Required

None — harness only touches `~/.agent-deck/config.toml` under a backup-and-restore trap.

## Next Phase Readiness

- Wave 2 (plan 03-02) can now reference `scripts/verify-per-group-claude-config.sh` by name in README subsection, CLAUDE.md one-liner, and CHANGELOG bullet
- Phase CFG-06 audit task can run `git log main..HEAD --grep '@alec-pinson' --oneline | wc -l` and see ≥1 (this plan's commit 16c891d provides the first attribution)
- No blockers

---
*Phase: 03-visual-harness-documentation-attribution-commit*
*Completed: 2026-04-15*
