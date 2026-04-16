---
phase: 02-env-file-source-semantics-observability-conductor-e2e
plan: 01
subsystem: session
tags: [claude, config, env_file, tmux, tdd, per-group, pr-578]

# Dependency graph
requires:
  - phase: 01-custom-command-injection-core-regression-tests
    provides: TestPerGroupConfig_* isolation recipe; buildBashExportPrefix integrated into custom-command spawn path (CFG-02)
  - phase: base (PR #578 fa9971e by @alec-pinson)
    provides: GetGroupClaudeEnvFile, getToolEnvFile, buildEnvSourceCommand, buildSourceCmd ignore-missing guard
provides:
  - TestPerGroupConfig_EnvFileSourcedInSpawn — 5th regression test locking env_file sourcing on BOTH normal-claude (instance.go:478) and custom-command (instance.go:598/599) spawn paths
  - Defensive hardening at instance.go:599 — custom-command return now prepends buildEnvSourceCommand() so direct callers of buildClaudeCommandWithMessage (bypassing buildClaudeCommand) still source env_file
  - TDD RED/GREEN gate visible in git history (38a2af3 RED test, e608480 GREEN fix)
affects: [phase-02-02-observability-conductor-e2e, phase-03-visual-harness-docs-attribution]

# Tech tracking
tech-stack:
  added:
    - "os/exec in internal/session/pergroupconfig_test.go — for assertion C runtime proof via bash -c harness"
    - "fmt in internal/session/pergroupconfig_test.go — for config.toml template formatting"
  patterns:
    - "Runtime-proof assertion: build the full spawn command, strip the trailing wrapper payload, re-append an echo of a sentinel env var, exec under bash, verify stdout — proves the env_file is actually sourced at runtime, not merely named in the string"
    - "Three-layer assertion: (A) string-match on normal path, (B) string-match on custom-command path, (C) runtime bash exec — catches both wiring gaps AND path-resolution bugs"
    - "Missing-file and negative-override cases in the same test function — locks non-blocking semantics and cache-invalidation semantics alongside the positive case"

key-files:
  created:
    - .planning/phases/02-env-file-source-semantics-observability-conductor-e2e/deferred-items.md
    - .planning/phases/02-env-file-source-semantics-observability-conductor-e2e/02-01-SUMMARY.md
  modified:
    - internal/session/pergroupconfig_test.go (+138 lines, TestPerGroupConfig_EnvFileSourcedInSpawn appended; imports expanded by fmt + os/exec)
    - internal/session/instance.go (+4/-3 lines at the custom-command return inside buildClaudeCommandWithMessage; comment block updated to cite both CFG-02 and CFG-03)

key-decisions:
  - "Pre-authorized L598 fix applied per plan instruction despite first-run GREEN on assertion B. Diagnosis: buildClaudeCommand at instance.go:477-480 already prepends envPrefix unconditionally around buildClaudeCommandWithMessage, so production callers (L1885/L2004/L4032) were already sourcing env_file on the custom-command path via the outer wrapper. The plan explicitly pre-authorized this L598 hardening as defense-in-depth against any future callsite that invokes buildClaudeCommandWithMessage directly. Applied accordingly."
  - "Regression test continues to pass both before and after the L598 fix — the test locks the CFG-03 guarantee regardless of which layer delivers it. If the outer envPrefix wrapping is ever removed, the L598 fix still keeps the custom-command path compliant; if L598 regresses, the outer wrapper still keeps it compliant. Belt-and-suspenders."
  - "Three StatusEventWatcher tests (TestStatusEventWatcher_DetectsNewFile/FilterByInstance/WaitForStatus) fail pre-existingly due to fsnotify timeout in this worktree's filesystem environment. Confirmed by git stash of Phase 02 changes — identical failures without the patch. Logged to Phase 02 deferred-items.md as new-since-Phase-01. Not a Phase 02 regression."

patterns-established:
  - "Plan `<pre_authorized>` directive honored even when RED diagnosis is different from expected — the plan author's defense-in-depth intent takes precedence over naive 'test already passes, skip the fix' reasoning. The commit body documents both the original plan hypothesis AND the observed diagnosis."
  - "Runtime proof pattern for shell-prefix builders: execute the built command under bash with the terminal payload swapped for an echo of a sentinel env var. Catches path-resolution bugs that string-contains assertions cannot."
  - "Attribution trailer 'Base implementation by @alec-pinson in PR #578.' attached to any commit that touches code paths introduced by PR #578, per milestone must_have #6."

requirements-completed:
  - CFG-03
  - CFG-04-4

# Metrics
duration: 5min
completed: 2026-04-15
tasks_completed: 2
files_modified: 2
files_created: 2
---

# Phase 02 Plan 01: env_file source semantics — Summary

**Group-level `[groups."<name>".claude].env_file` is now locked under automated regression test (`TestPerGroupConfig_EnvFileSourcedInSpawn`) for BOTH the normal-claude path (instance.go:478) and the custom-command/conductor path (instance.go:599), with defense-in-depth hardening at the inner custom-command return so env_file sourcing survives any future refactor that bypasses the outer `buildClaudeCommand` wrapper.**

## Performance

- **Duration:** ~5 min (start 13:32:11 UTC → GREEN fix 13:37:38 UTC, plus SUMMARY write)
- **Started:** 2026-04-15T13:32:11Z
- **Tasks:** 2/2 complete
- **Files modified:** 2 (`internal/session/pergroupconfig_test.go`, `internal/session/instance.go`)
- **Files created:** 2 (`deferred-items.md`, `02-01-SUMMARY.md`)
- **Net diff vs base 6a0205d:** +142/-3 lines across the two session files

## Commits on top of phase 02 base (6a0205d)

| Hash | Type | Subject |
|------|------|---------|
| 38a2af3 | test | test(session): add env_file spawn-source regression test (CFG-04 test 4) |
| e608480 | fix  | fix(session): source group env_file on custom-command spawn path (CFG-03) |

Both commits carry `Committed by Ashesh Goplani`. The fix commit carries `Base implementation by @alec-pinson in PR #578.` per milestone must_have #6. Neither commit carries Claude attribution. Neither commit used `--no-verify`; lefthook `fmt-check` and `vet` ran and passed on both.

## RED gate outcome

- Log: `/tmp/pergroupconfig-envfile-red.log`
- **Observed:** `--- PASS: TestPerGroupConfig_EnvFileSourcedInSpawn (0.00s)` on first run, before the L599 fix was applied.
- **Plan hypothesis:** assertion B (custom-command branch) would RED against HEAD 6a0205d because instance.go:598 returned `i.buildBashExportPrefix() + baseCommand` without prepending `buildEnvSourceCommand()`.
- **Actual diagnosis:** the outer wrapper `buildClaudeCommand` at instance.go:477-480 already prepends `envPrefix := i.buildEnvSourceCommand()` unconditionally before returning the output of `buildClaudeCommandWithMessage`. All three production callsites (L1885, L2004, L4032) call `buildClaudeCommand`, not `buildClaudeCommandWithMessage` directly, so env_file was already being sourced for custom-command sessions in practice.
- **Response (per plan):** the plan explicitly pre-authorized the L598 one-line fix and documented it as defense-in-depth via the `<pre_authorized>` directive in `<project_rules>`. Applied the fix as specified. The commit body documents both the plan hypothesis and the observed diagnosis.

## GREEN gate outcome

- Log: `/tmp/pergroupconfig-envfile-green.log`
- All 5 `TestPerGroupConfig_*` tests PASS under `go test ./internal/session/... -run TestPerGroupConfig_ -race -count=1 -v`:
  - `TestPerGroupConfig_CustomCommandGetsGroupConfigDir` (Phase 1, CFG-04 test 1) — PASS
  - `TestPerGroupConfig_GroupOverrideBeatsProfile` (Phase 1, CFG-04 test 2) — PASS
  - `TestPerGroupConfig_UnknownGroupFallsThroughToProfile` (Phase 1, CFG-04 test 3) — PASS
  - `TestPerGroupConfig_CacheInvalidation` (Phase 1, CFG-04 test 6) — PASS
  - `TestPerGroupConfig_EnvFileSourcedInSpawn` (Phase 2, CFG-04 test 4) — PASS ← this plan

## PR #578 regression subset

- Log: `/tmp/pr578-regressions-phase2.log`
- All 7 matching tests PASS:
  - `TestGetClaudeConfigDirForGroup_GroupWins`
  - `TestIsClaudeConfigDirExplicitForGroup`
  - `TestBuildClaudeCommand_CustomAlias`
  - `TestUserConfig_GroupClaudeConfigDir`
  - `TestUserConfig_GroupClaudeConfigDir_Empty`
  - `TestUserConfig_GroupClaudeConfigDir_NestedPath`
  - `TestUserConfig_GroupClaudeEnvFile`

## TDD gate compliance

- **RED commit:** `38a2af3` — `test(session): add env_file spawn-source regression test (CFG-04 test 4)`. Test-only change; production code untouched. RED log captured.
- **GREEN commit:** `e608480` — `fix(session): source group env_file on custom-command spawn path (CFG-03)`. One-line production fix. All 5 TestPerGroupConfig_ tests GREEN after.
- **REFACTOR commit:** none required — single-line change, no cleanup needed.
- **Attribution:** `Base implementation by @alec-pinson in PR #578.` present in body of e608480.
- **Sign-off:** `Committed by Ashesh Goplani` present in body of both commits.
- **No Claude attribution anywhere** — verified via `git log 4730aa5..HEAD --format="%s%n%b" | grep -E "🤖|Co-Authored-By: Claude|Generated with Claude"` → empty.
- **No `--no-verify`** — lefthook hooks ran on both commits.

## Must-haves verification (from plan frontmatter)

| # | Truth | Result |
|---|-------|--------|
| 1 | `pergroupconfig_test.go` contains `TestPerGroupConfig_EnvFileSourcedInSpawn` | ✅ (grep confirms 5 total) |
| 2 | env_file sourced by tmux spawn for BOTH normal-claude AND custom-command paths | ✅ (assertion A + B both GREEN; L599 hardens the inner return) |
| 3 | Missing env_file logs warning, does NOT block session start | ✅ (missing-file case in test asserts path still present via ignore-missing guard) |
| 4 | All 5 `TestPerGroupConfig_*` tests GREEN under `-race -count=1` | ✅ (`/tmp/pergroupconfig-envfile-green.log`) |
| 5 | PR #578's `TestGetClaudeConfigDirForGroup_GroupWins` and `TestIsClaudeConfigDirExplicitForGroup` remain GREEN | ✅ (`/tmp/pr578-regressions-phase2.log`) |
| 6 | Wiring-fix commit carries `Base implementation by @alec-pinson in PR #578.` | ✅ (e608480 body) |
| 7 | All commits signed `Committed by Ashesh Goplani`; no Claude attribution; no `--no-verify` | ✅ |

## Deferred issues (see `deferred-items.md`)

- **Phase 01 carry-over:** 6 `TestSyncSessionIDsFromTmux_*` / `TestInstance_UpdateClaudeSession_*` / `TestInstance_GetSessionIDFromTmux` failures due to tmux set-environment in the worktree environment. Re-confirmed not a Phase 02 regression.
- **New in Phase 02:** 3 `TestStatusEventWatcher_*` failures due to fsnotify event-delivery timeout in this filesystem environment. Verified via `git stash` that failures reproduce without Phase 02 changes; not a Phase 02 regression.

## Retained log paths

- `/tmp/pergroupconfig-envfile-red.log` — RED state capture (assertion B passed unexpectedly due to outer-wrapper envPrefix — documented in commit body)
- `/tmp/pergroupconfig-envfile-green.log` — GREEN state capture, all 5 `TestPerGroupConfig_*` tests PASS
- `/tmp/pr578-regressions-phase2.log` — PR #578 regression subset, all 7 tests PASS

## Self-Check: PASSED

- ✅ `internal/session/pergroupconfig_test.go` exists and contains `TestPerGroupConfig_EnvFileSourcedInSpawn`
- ✅ `internal/session/instance.go` contains `return i.buildEnvSourceCommand() + i.buildBashExportPrefix() + baseCommand` at line 599
- ✅ Commit `38a2af3` exists in `git log`
- ✅ Commit `e608480` exists in `git log`
- ✅ `.planning/phases/02-env-file-source-semantics-observability-conductor-e2e/deferred-items.md` exists
- ✅ All 10 phase-level verification gates pass (see plan `<verification>` block)
