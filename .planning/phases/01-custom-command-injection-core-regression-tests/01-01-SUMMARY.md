---
phase: 01-custom-command-injection-core-regression-tests
plan: 01
subsystem: session
tags: [claude, config, tmux, tdd, per-group, pr-578]

# Dependency graph
requires:
  - phase: base (PR #578 fa9971e by @alec-pinson)
    provides: GetClaudeConfigDirForGroup, IsClaudeConfigDirExplicitForGroup, buildBashExportPrefix
provides:
  - Four regression tests under TestPerGroupConfig_* locking per-group CLAUDE_CONFIG_DIR behavior
  - Export of CLAUDE_CONFIG_DIR in custom-command (wrapper-script) spawn env via buildBashExportPrefix() prefix
  - TDD RED/GREEN gate visible in git history (40f4f04 RED test, b39bbf3 GREEN fix)
affects: [phase-02-env-file-source-semantics, phase-03-visual-harness-docs-attribution]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "In-package Go test style (package session, no testify, t.Errorf/t.Fatalf)"
    - "Test isolation recipe: save HOME/AGENTDECK_PROFILE/CLAUDE_CONFIG_DIR, set HOME=t.TempDir(), write config.toml, ClearUserConfigCache(), t.Cleanup restores + re-clears"
    - "Spawn-command assertion via strings.Contains(cmd, \"CLAUDE_CONFIG_DIR=\"+wantDir)"

key-files:
  created:
    - internal/session/pergroupconfig_test.go
    - .planning/phases/01-custom-command-injection-core-regression-tests/deferred-items.md
  modified:
    - internal/session/instance.go

key-decisions:
  - "Option A (prepend buildBashExportPrefix()) chosen over Option B (inline configDirPrefix) — export form is strictly safer for wrappers that later exec their own children; Option B's inline form only persists to a single exec"
  - "Preserved RED state in git history via separate test commit (40f4f04) before fix commit (b39bbf3), so TDD discipline is auditable post hoc"
  - "Six pre-existing tmux-environment test failures in internal/session (TestSyncSessionIDsFromTmux_*, TestInstance_GetSessionIDFromTmux, TestInstance_UpdateClaudeSession_*) deferred — verified they reproduce at parent commit 4730aa5 without the Phase-01 changes, so not a regression"

patterns-established:
  - "Regression test naming: TestPerGroupConfig_<BehaviorName> — shared namespace across phases so `-run TestPerGroupConfig_` matches all six tests once Phase 2 adds tests 4 and 5"
  - "Minimal surgical patch policy: single-line concatenation change at instance.go:596 with REQ CFG-02 comment citation; no helpers introduced, no refactor of PR #578's code"
  - "Commit trailer discipline: body ends with 'Committed by Ashesh Goplani'; fix commits that build on external work carry 'Builds on PR #578 by @alec-pinson.' as a separate attribution line"

requirements-completed:
  - CFG-01
  - CFG-02
  - CFG-04

# Metrics
duration: 13min
completed: 2026-04-15
---

# Phase 01 Plan 01: Custom-command injection + core regression tests — Summary

**Per-group CLAUDE_CONFIG_DIR now exports into the tmux spawn env for conductor / wrapper-script sessions via a single-line buildBashExportPrefix() prefix at instance.go:596, locked under four TestPerGroupConfig_* regression tests that ran RED before the fix and GREEN after.**

## Performance

- **Duration:** ~13 min (plan commit 4730aa5 at 14:22:35 CET → GREEN fix b39bbf3 at 14:30:47 CET, plus SUMMARY write)
- **Started:** 2026-04-15T12:22:35Z (plan committed)
- **Completed:** 2026-04-15T12:35:43Z (after GREEN gate + deferred-items log)
- **Tasks:** 7 completed (1 test-write, 2 verify runs, 2 commits, 1 patch, 1 make-ci gate)
- **Files modified:** 2 source files (1 new test, 1 surgical patch) + 1 deferred-items log + SUMMARY/STATE

## Accomplishments

- Added `internal/session/pergroupconfig_test.go` with all four CFG-04 Phase-1 tests: `TestPerGroupConfig_CustomCommandGetsGroupConfigDir`, `TestPerGroupConfig_GroupOverrideBeatsProfile`, `TestPerGroupConfig_UnknownGroupFallsThroughToProfile`, `TestPerGroupConfig_CacheInvalidation`.
- Verified the predicted RED/GREEN split against base `4730aa5` via `/tmp/pergroupconfig-red.log` — tests 1 and 2 FAILED (no `CLAUDE_CONFIG_DIR=` in spawn string), tests 3 and 6 PASSED. Exactly matched RESEARCH §1.4 assumption A1/A2.
- Applied the minimal surgical patch at `internal/session/instance.go:596`: replaced `return baseCommand` with `return i.buildBashExportPrefix() + baseCommand`, with a comment citing `REQ CFG-02`. Net diff: +4/-2 lines in the one permitted production file.
- Flipped tests 1 and 2 to GREEN (`/tmp/pergroupconfig-green.log`), with all four `TestPerGroupConfig_*` passing under `-race -count=1`.
- Verified zero regressions in PR #578's existing tests (`TestGetClaudeConfigDirForGroup_GroupWins`, `TestIsClaudeConfigDirExplicitForGroup`, `TestBuildClaudeCommand_CustomAlias`, three `TestUserConfig_GroupClaudeConfigDir*`, `TestUserConfig_GroupClaudeEnvFile`) via `/tmp/pr578-regressions.log`.
- Produced two atomic commits with full attribution compliance: RED test commit + GREEN fix commit carrying `Builds on PR #578 by @alec-pinson.`.

## Task Commits

Each task was committed atomically on a mandatory RED-before-GREEN schedule:

1. **Task 1: Write `internal/session/pergroupconfig_test.go`** — part of `40f4f04` (see Task 3)
2. **Task 2: Run RED gate** — no commit; captured `/tmp/pergroupconfig-red.log` (EXIT=1, 2 FAIL / 2 PASS as predicted)
3. **Task 3: RED test commit** — `40f4f04` `test(session): add per-group Claude config regression tests (CFG-04 tests 1/2/3/6)`
4. **Task 4: Apply surgical patch at `instance.go:596`** — part of `b39bbf3` (see Task 6)
5. **Task 5: GREEN gate** — no commit; captured `/tmp/pergroupconfig-green.log` (EXIT=0, all 4 PASS) and `/tmp/pr578-regressions.log` (EXIT=0, all 7 PR #578 tests PASS)
6. **Task 6: GREEN fix commit** — `b39bbf3` `fix(session): export CLAUDE_CONFIG_DIR for custom-command sessions (CFG-02)` — body contains both `Builds on PR #578 by @alec-pinson.` and `Committed by Ashesh Goplani`
7. **Task 7: make ci gate** — see "Issues Encountered" and "Deferred Issues" for the pre-existing tmux-env failures

**Plan metadata:** will be recorded in the final docs commit together with STATE.md and SUMMARY.md.

## Files Created/Modified

- `internal/session/pergroupconfig_test.go` (NEW, 235 lines) — Four regression tests covering custom-command export, group-beats-profile spawn behavior, unknown-group fallthrough, and cache invalidation. Uses the house-style in-package isolation recipe from `claude_test.go:693-763`.
- `internal/session/instance.go` (+4/-2) — Custom-command return path at line 596 now prepends `i.buildBashExportPrefix()` before `baseCommand`. Comment cites `REQ CFG-02`.
- `.planning/phases/01-custom-command-injection-core-regression-tests/deferred-items.md` (NEW) — Logs the six pre-existing tmux-environment test failures in `internal/session` (all unrelated to Phase-01 changes) per executor scope-boundary rules.

## Decisions Made

- **Option A over Option B (RESEARCH §1.5):** Prepending `buildBashExportPrefix()` (which emits `export CLAUDE_CONFIG_DIR=...;`) is strictly safer than Option B's inline-prefix form because the conductor wrapper script later exec's its own children (`exec claude`) that must inherit the env var. The `export` form persists to child processes; the inline form does not.
- **Preserve RED in git history:** The test commit (`40f4f04`) lands before the fix commit (`b39bbf3`) so the TDD RED-before-GREEN transition is auditable post hoc via `git show 40f4f04` + `git test` replay.
- **Scope discipline:** Declined to investigate or fix the six pre-existing `TestSyncSessionIDsFromTmux_*` / `TestInstance_GetSessionIDFromTmux` / `TestInstance_UpdateClaudeSession_*` failures even though they surfaced in `make ci`. Logged to `deferred-items.md` with reproduction proof at parent commit `4730aa5`.

## Deviations from Plan

None of the Rule 1-4 deviation rules triggered during Task 1 through Task 6. The plan's Task 4 action block matched the codebase byte-for-byte (the BEFORE snippet was exactly what `instance.go:592-597` contained at `4730aa5`). The `gofmt` and `go vet` pre-empt steps in Tasks 1 and 4 both succeeded on the first run.

**Task 7 scope-boundary decision (logged as deferred, not as a Rule 1 bug fix):** `make ci` returned non-zero with six tmux-environment test failures in `internal/session`. Per the executor's SCOPE BOUNDARY rule ("Only auto-fix issues DIRECTLY caused by the current task's changes. Pre-existing warnings, linting errors, or failures in unrelated files are out of scope. Log to `deferred-items.md`."), I verified the failures reproduce identically at parent commit `4730aa5` without the Phase-01 changes. They are not a Phase-01 regression and are logged in `deferred-items.md` for future investigation. Phase-01's own test suite is GREEN; PR #578's test suite remains GREEN.

## Issues Encountered

- **During Task 7 investigation:** Briefly reverted `internal/session/instance.go` and removed `pergroupconfig_test.go` locally to reproduce the `make ci` failures at parent commit `4730aa5`. Restored both files via `git checkout HEAD --` immediately after confirming the failures are pre-existing. No commit, no push, no loss of work. Should have used `git worktree` for this rather than in-place revert; noted for future root-cause reproduction work.

## Deferred Issues

See `.planning/phases/01-custom-command-injection-core-regression-tests/deferred-items.md` for full details.

Six pre-existing tmux-environment test failures in `internal/session`:
- `TestSyncSessionIDsFromTmux_Claude`
- `TestSyncSessionIDsFromTmux_AllTools`
- `TestSyncSessionIDsFromTmux_OverwriteWithNew`
- `TestInstance_GetSessionIDFromTmux`
- `TestInstance_UpdateClaudeSession_TmuxFirst`
- `TestInstance_UpdateClaudeSession_RejectZombie`

All fail with `SetEnvironment failed: exit status 1` from `tmux set-environment` after `inst.Start()`. `skipIfNoTmuxServer` does not skip them because the binary and some server are available, but the new session created by the test is unable to have env vars written to it. Likely interaction between running inside an outer tmux pane (`TMUX` set) and lefthook's env-scrubbed sub-shell. Out of scope for CFG-02; tracked for a follow-up plan.

## TDD Gate Compliance

Plan-level TDD (`type: tdd`) gate sequence validated in git log since base `3e402e2`:

1. **RED commit:** `40f4f04` `test(session): add per-group Claude config regression tests ...` (1 file, test-only). Subject prefix `test(`.
2. **GREEN commit:** `b39bbf3` `fix(session): export CLAUDE_CONFIG_DIR for custom-command sessions ...` (1 file, implementation). Subject prefix `fix(`, lands after RED.
3. **REFACTOR commit:** none required; the patch was minimal (single concatenation) and needed no clean-up.

RED and GREEN gates both present in the correct order. No `--no-verify` used; lefthook pre-commit (gofmt + go vet) passed cleanly on both commits.

## Self-Check

Verifying all declared artifacts and commits exist:

- `internal/session/pergroupconfig_test.go` — FOUND
- `internal/session/instance.go` (patched) — FOUND with `return i.buildBashExportPrefix() + baseCommand` present and 1 occurrence of `REQ CFG-02`
- `.planning/phases/01-custom-command-injection-core-regression-tests/deferred-items.md` — FOUND
- Commit `40f4f04` (RED test) — FOUND in `git log`
- Commit `b39bbf3` (GREEN fix) — FOUND in `git log`

Phase-level invariants (from plan `must_haves.truths`):

- `>=2 "Committed by Ashesh Goplani"` trailers since `3e402e2` — **3** (pass)
- `==0` Claude-attribution strings (`🤖|Co-Authored-By: Claude|Generated with Claude`) since `3e402e2` — **0** (pass)
- `>=1 "Builds on PR #578 by @alec-pinson."` line — **1** (pass)
- No `push`/`tag`/`gh pr`/`merge` in reflog — **0** (pass)

## Self-Check: PASSED

## Next Phase Readiness

- Test file `internal/session/pergroupconfig_test.go` is ready for Phase 2 to extend with tests 4 (`EnvFileSourcedInSpawn`) and 5 (`ConductorRestartPreservesConfigDir`) — the in-package isolation recipe, imports (`os`, `path/filepath`, `strings`, `testing`), and naming convention are already established.
- `buildBashExportPrefix()` is now the single chokepoint for per-group export in both the `baseCommand == "claude"` branch and the custom-command return path. Phase 2's observability log line (CFG-07) can hook here if desired.
- No blockers for Phase 2. The deferred tmux-env failures do not affect Phase-2 test authoring because they are in unrelated test files (`instance_platform_test.go`, `instance_test.go` subset).

---
*Phase: 01-custom-command-injection-core-regression-tests*
*Completed: 2026-04-15*
