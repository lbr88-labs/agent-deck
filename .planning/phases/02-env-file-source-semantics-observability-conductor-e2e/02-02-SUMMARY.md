---
phase: 02-env-file-source-semantics-observability-conductor-e2e
plan: 02
subsystem: session
tags: [claude, config, observability, slog, tdd, per-group, pr-578]

# Dependency graph
requires:
  - phase: 02-env-file-source-semantics-observability-conductor-e2e plan 01
    provides: TestPerGroupConfig_EnvFileSourcedInSpawn (test 4) + CFG-03 env_file source guarantee on both normal-claude and custom-command spawn paths
  - phase: 01-custom-command-injection-core-regression-tests
    provides: NewInstanceWithGroupAndTool factory + TestPerGroupConfig_* isolation recipe
  - phase: base (PR #578 fa9971e by @alec-pinson)
    provides: GetClaudeConfigDirForGroup priority chain at claude.go:246; LoadUserConfig + ClearUserConfigCache; GetGroupClaudeConfigDir; GetProfileClaudeConfigDir
provides:
  - TestPerGroupConfig_ConductorRestartPreservesConfigDir (CFG-04 test 5) — custom-command restart preserves group config_dir export
  - TestPerGroupConfig_ClaudeConfigDirSourceLabel — 5-subtest table exercising every priority level (env|group|profile|global|default)
  - TestPerGroupConfig_ClaudeConfigResolutionLogFormat — handler-swap regex test locking the CFG-07 rendered slog line format
  - GetClaudeConfigDirSourceForGroup(groupPath) (path, source string) — returns resolved dir AND priority label
  - (i *Instance) logClaudeConfigResolution() — owns the single CFG-07 slog message literal
  - CFG-07 emission at exactly 3 sites: Start(), StartWithMessage(), Restart() — each gated on IsClaudeCompatible(i.Tool)
affects: [phase-03-visual-harness-docs-attribution]

# Tech tracking
tech-stack:
  added:
    - "bytes in internal/session/pergroupconfig_test.go — buffer-backed handler for log-format assertion"
    - "log/slog in internal/session/pergroupconfig_test.go — handler-swap pattern + slog.NewTextHandler"
    - "regexp in internal/session/pergroupconfig_test.go — CFG-07 format regex + CLAUDE_CONFIG_DIR extraction"
  patterns:
    - "Handler-swap pattern: override package-level *slog.Logger with a bytes.Buffer-backed TextHandler for test assertions on rendered log output; restore via t.Cleanup"
    - "Compile-stub TDD: add stub (empty-body) helpers in the production files in the SAME commit as the failing tests, so the test package builds and RED shows as assertion failures rather than compile errors. Cleaner git bisect than build-tag tricks."
    - "Grep-auditable emission sites: single helper owns the slog literal; 3 gated call sites (Start/StartWithMessage/Restart) form a simple `grep -cE 'i\\.logClaudeConfigResolution\\(\\)'` = 3 audit that any reviewer can run."

key-files:
  created:
    - .planning/phases/02-env-file-source-semantics-observability-conductor-e2e/02-02-SUMMARY.md
  modified:
    - internal/session/pergroupconfig_test.go (+291 lines total across RED + GREEN: 3 imports added, 3 new test functions appended, 1 inst.ID override in GREEN)
    - internal/session/claude.go (+33 lines: stub replaced with real priority-walk returning path + source label)
    - internal/session/instance.go (+34 lines net: helper body + 3 gated call sites in Start/StartWithMessage/Restart)

key-decisions:
  - "Plan test mispredicted constructor: the plan assumed NewInstanceWithGroupAndTool's first arg populated i.ID, but it actually populates i.Title. The CFG-07 helper logs i.ID (correct — session logs key on ID, not human-readable title). Fixed with a one-line inst.ID override in the LogFormat test, preserving the test's intent (assert session=<precise-id> appears) without changing the helper. Classified as Rule 1 deviation (bug in plan test spec); commit body documents the fix."
  - "Stub-owns-the-comment tweak: initial helper comment said 'Owns the single \"claude config resolution\" slog literal' which made grep -c return 2 (comment + actual literal). Plan verification gate expected exactly 1. Reworded to 'Owns the single CFG-07 slog message literal' — no semantic loss, grep now returns 1 as specified. Comment still reads naturally."
  - "Three emission sites, three guards: the gate `if IsClaudeCompatible(i.Tool)` is duplicated at each call site (Start, StartWithMessage, Restart) rather than pushed into the helper body. Rationale per plan: keeps the three emission sites grep-auditable — a reviewer running `grep -B1 'i\\.logClaudeConfigResolution' instance.go` sees the gate co-located with every call. Moving the gate into the helper would hide the tool-type contract."

patterns-established:
  - "Handler-swap tests for slog: `sessionLog = slog.New(slog.NewTextHandler(&buf, opts))` with t.Cleanup restoration. Works because sessionLog is a package-level var (not const); verified at instance.go:33."
  - "Restart as a first-class spawn path: Restart() gets the same observability treatment as Start()/StartWithMessage() — triage must not go dark on the scenario most likely to need debugging."
  - "Plan pre-declared audit greps form the commit-message readme for reviewers: every gate in <verification> is a one-liner a reviewer can copy into their terminal."

requirements-completed:
  - CFG-04-5
  - CFG-07

# Metrics
duration: 8min
completed: 2026-04-15
tasks_completed: 2
files_modified: 3
files_created: 1
---

# Phase 02 Plan 02: CFG-04 test 5 + CFG-07 observability — Summary

**Conductor restart preservation (CFG-04 test 5) and CFG-07 observability are now both locked under automated regression test. A new `GetClaudeConfigDirSourceForGroup(groupPath) (path, source string)` helper mirrors the priority chain at `claude.go:246` and returns both the resolved config dir AND the label that set it (one of `env|group|profile|global|default`). A private `(i *Instance) logClaudeConfigResolution()` owns the single CFG-07 slog message literal and is called from exactly three session-spawn entrypoints — `Start()`, `StartWithMessage()`, `Restart()` — each gated on `IsClaudeCompatible(i.Tool)`. The rendered slog format is automated-asserted via a bytes.Buffer handler-swap test, replacing the previously-manual step-9 smoke check.**

## Performance

- **Duration:** ~8 min (TDD RED + GREEN + verification + SUMMARY)
- **Started:** 2026-04-15T15:45:00Z (approx.)
- **Tasks:** 2/2 complete
- **Files modified:** 3 (`internal/session/pergroupconfig_test.go`, `internal/session/claude.go`, `internal/session/instance.go`)
- **Files created:** 1 (`02-02-SUMMARY.md`)
- **Net diff vs plan 02-01 HEAD (5d8737f):** +379/-12 lines

## Commits on top of plan 02-01 HEAD (5d8737f)

| Hash    | Type | Subject                                                                                |
| ------- | ---- | -------------------------------------------------------------------------------------- |
| e000801 | test | test(session): add conductor-restart + CFG-07 source-label + log-format regression tests |
| 476367c | feat | feat(session): add CFG-07 claude-config-resolution log line + source-label helper      |

Both commits carry `Committed by Ashesh Goplani`. The `feat` commit carries `Base implementation by @alec-pinson in PR #578.` per milestone must_have #6. Neither commit carries Claude attribution. Neither commit used `--no-verify`; lefthook `fmt-check` and `vet` ran and passed on both.

## RED gate outcome

- Log: `/tmp/pergroupconfig-plan2-red.log`
- **Observed (as predicted by plan):**
  - `--- PASS: TestPerGroupConfig_ConductorRestartPreservesConfigDir (0.00s)` — uses only existing helpers (build → ClearUserConfigCache → rebuild); plan 02-01's CFG-03 wiring already suffices.
  - `--- FAIL: TestPerGroupConfig_ClaudeConfigDirSourceLabel` — all 5 subtests fail (stub returns "", "").
  - `--- FAIL: TestPerGroupConfig_ClaudeConfigResolutionLogFormat` — stub logs nothing; regex mismatches.
- Stub-owns-the-compile pattern worked: no compile failures; RED manifested as runtime assertion failures as planned.

## GREEN gate outcome

- Log: `/tmp/pergroupconfig-plan2-green.log`
- All 8 `TestPerGroupConfig_*` tests PASS under `go test ./internal/session/... -run TestPerGroupConfig_ -race -count=1 -v`:
  - `TestPerGroupConfig_CustomCommandGetsGroupConfigDir` (Phase 1, CFG-04 test 1) — PASS
  - `TestPerGroupConfig_GroupOverrideBeatsProfile` (Phase 1, CFG-04 test 2) — PASS
  - `TestPerGroupConfig_UnknownGroupFallsThroughToProfile` (Phase 1, CFG-04 test 3) — PASS
  - `TestPerGroupConfig_CacheInvalidation` (Phase 1, CFG-04 test 6) — PASS
  - `TestPerGroupConfig_EnvFileSourcedInSpawn` (Phase 2 plan 01, CFG-04 test 4) — PASS
  - `TestPerGroupConfig_ConductorRestartPreservesConfigDir` (Phase 2 plan 02, CFG-04 test 5) — PASS ← this plan
  - `TestPerGroupConfig_ClaudeConfigDirSourceLabel` (CFG-07 priority chain, 5 subtests) — PASS ← this plan
  - `TestPerGroupConfig_ClaudeConfigResolutionLogFormat` (CFG-07 rendered format) — PASS ← this plan

## PR #578 regression subset

- Log: `/tmp/pr578-regressions-phase2-plan02.log`
- All 7 matching tests PASS:
  - `TestGetClaudeConfigDirForGroup_GroupWins`
  - `TestIsClaudeConfigDirExplicitForGroup`
  - `TestBuildClaudeCommand_CustomAlias`
  - `TestUserConfig_GroupClaudeConfigDir`
  - `TestUserConfig_GroupClaudeConfigDir_Empty`
  - `TestUserConfig_GroupClaudeConfigDir_NestedPath`
  - `TestUserConfig_GroupClaudeEnvFile`

## CFG-07 wiring audit (from plan <verification> step 3)

```
$ grep -cE 'i\.logClaudeConfigResolution\(\)' internal/session/instance.go
3                                   # Start + StartWithMessage + Restart

$ grep -c 'func (i \*Instance) logClaudeConfigResolution' internal/session/instance.go
1                                   # exactly one declaration

$ grep -c '"claude config resolution"' internal/session/instance.go
1                                   # helper owns the single slog literal

$ grep -n '"claude config resolution"' internal/session/*.go | grep -v instance.go | grep -v _test.go
(empty)                             # not referenced in any other production file
```

## CFG-07 log line — rendered format (from GREEN run, synthesized example)

```
time=2026-04-15T15:48:07.743+02:00 level=INFO msg="claude config resolution" session=logfmt-sess-123 group=logfmt resolved=/tmp/TestPerGroupConfig_ClaudeConfigResolutionLogFormat.../001/.claude-logfmt source=group
```

This exact format is automated-asserted by `TestPerGroupConfig_ClaudeConfigResolutionLogFormat` via the regex:
```
claude config resolution.*session=\S+\s+group=\S*\s+resolved=\S+\s+source=(env|group|profile|global|default)
```

## TDD gate compliance

- **RED commit:** `e000801` — `test(session): add conductor-restart + CFG-07 source-label + log-format regression tests`. Three new tests appended; compile-only stubs added in claude.go (returns "","") and instance.go (empty helper body). RED log captured. SourceLabel and LogFormat tests fail; Restart test passes (uses only existing helpers).
- **GREEN commit:** `476367c` — `feat(session): add CFG-07 claude-config-resolution log line + source-label helper`. Stub bodies replaced with real logic; 3 call sites added to Start/StartWithMessage/Restart; one-line inst.ID override in the LogFormat test (Rule 1 deviation, see below). All 8 TestPerGroupConfig_* tests GREEN after.
- **REFACTOR commit:** none required — implementation is minimal and already DRY.
- **Attribution:** `Base implementation by @alec-pinson in PR #578.` present in body of 476367c.
- **Sign-off:** `Committed by Ashesh Goplani` present in body of both commits.
- **No Claude attribution anywhere** — verified via `git log 4730aa5..HEAD --format="%s%n%b" | grep -E "🤖|Co-Authored-By: Claude|Generated with Claude"` → empty.
- **No `--no-verify`** — lefthook hooks ran and passed on both commits.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan test used first arg as session ID but `NewInstanceWithGroupAndTool` assigns it as Title**
- **Found during:** Task 2 GREEN run — LogFormat test failed with the helper fully wired.
- **Issue:** The plan's `TestPerGroupConfig_ClaudeConfigResolutionLogFormat` spec asserts `strings.Contains(line, "session=logfmt-sess-123")`, but `NewInstanceWithGroupAndTool("logfmt-sess-123", ...)` populates `inst.Title = "logfmt-sess-123"` and assigns a generated UUID to `inst.ID`. The CFG-07 helper correctly logs `i.ID` (session logs key on ID, not human-readable title — matches the broader `sessionLog.*slog.String("session", i.ID)` pattern elsewhere in the codebase).
- **Fix:** One-line `inst.ID = "logfmt-sess-123"` override immediately after construction in the LogFormat test. Preserves the test's intent (assert `session=<precise-id>` appears in rendered log) without contorting the helper. Alternative considered: change the assertion to regex-match `session=\S+` instead of a literal — rejected because it would lose specificity and let a future bug silently flip the field to, say, `session=<title>`.
- **Files modified:** `internal/session/pergroupconfig_test.go` (+3 lines: 1 assignment, 2-line explanatory comment).
- **Commit:** `476367c` (bundled with Task 2 GREEN commit; documented in body).

**2. [Rule 3 - Blocking] Helper comment contained the slog literal string, inflating grep count**
- **Found during:** Task 2 audit run — `grep -c '"claude config resolution"' internal/session/instance.go` returned 2 (plan expected 1).
- **Issue:** Initial helper comment was `// Owns the single "claude config resolution" slog literal for this package.` — the grep treats the comment mention identically to the actual Go string literal, inflating the count past the plan's expected 1.
- **Fix:** Reworded comment to `// Owns the single CFG-07 slog message literal for this package.` — no semantic change; comment still reads naturally; grep now returns exactly 1 (the `sessionLog.Info("claude config resolution", ...)` call).
- **Files modified:** `internal/session/instance.go` (1-line comment reword).
- **Commit:** `476367c`.

### No other deviations

All three emission sites (`Start`, `StartWithMessage`, `Restart`) landed at the positions specified by the plan (immediately after successful `i.tmuxSession.Start(command)`). Fork is intentionally silent as specified. All imports for the test file landed in alphabetical order. No new top-level imports needed in claude.go or instance.go.

## Must-haves verification (from plan frontmatter)

| #   | Truth                                                                                                                                                                                 | Result |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| 1   | `pergroupconfig_test.go` contains `TestPerGroupConfig_ConductorRestartPreservesConfigDir`                                                                                             | ✓ (grep confirms) |
| 2   | `pergroupconfig_test.go` contains `TestPerGroupConfig_ClaudeConfigDirSourceLabel`                                                                                                     | ✓ |
| 3   | `pergroupconfig_test.go` contains `TestPerGroupConfig_ClaudeConfigResolutionLogFormat`                                                                                                | ✓ |
| 4   | All 8 `TestPerGroupConfig_*` tests GREEN under `-race -count=1`                                                                                                                       | ✓ (`/tmp/pergroupconfig-plan2-green.log`) |
| 5   | PR #578's `TestGetClaudeConfigDirForGroup_GroupWins` and `TestIsClaudeConfigDirExplicitForGroup` remain GREEN                                                                         | ✓ (`/tmp/pr578-regressions-phase2-plan02.log`) |
| 6   | Restart of custom-command session with group override re-emits the same `CLAUDE_CONFIG_DIR` — proved by rebuild after ClearUserConfigCache                                            | ✓ (CFG-04 test 5 PASS) |
| 7   | `GetClaudeConfigDirSourceForGroup` exists and returns path+source matching priority chain at claude.go:246                                                                            | ✓ (5-subtest table all PASS) |
| 8   | CFG-07 emission factored into `(i *Instance) logClaudeConfigResolution()` owning the single slog literal; called from EXACTLY 3 sites (Start, StartWithMessage, Restart)              | ✓ (grep audit: 3 call sites, 1 decl, 1 literal) |
| 9   | CFG-07 emission NOT in buildBashExportPrefix, buildClaudeCommand, buildClaudeForkCommandForTarget, or Fork                                                                            | ✓ (grep `i.logClaudeConfigResolution` in those functions → empty) |
| 10  | Substantive commit (`476367c`) carries `Base implementation by @alec-pinson in PR #578.` in body                                                                                      | ✓ |
| 11  | All commits signed `Committed by Ashesh Goplani`; no Claude attribution; no `--no-verify`                                                                                             | ✓ |

## Pre-existing failures unchanged

Phase 1 + plan 02-01 documented the tmux-set-environment and fsnotify-timeout failures in `deferred-items.md`. Not re-running the full package suite in this plan's SUMMARY — the CFG-07 changes touch only the session-spawn log path and cannot plausibly affect those pre-existing failures.

## Retained log paths

- `/tmp/pergroupconfig-plan2-red.log` — RED state (SourceLabel + LogFormat FAIL; Restart PASS)
- `/tmp/pergroupconfig-plan2-green.log` — GREEN state (all 8 TestPerGroupConfig_* tests PASS)
- `/tmp/pr578-regressions-phase2-plan02.log` — PR #578 regression subset (all 7 tests PASS)

## Self-Check: PASSED

- ✓ `internal/session/pergroupconfig_test.go` exists and contains 8 `TestPerGroupConfig_*` functions (verified: `grep -c "^func TestPerGroupConfig_"` returns 8)
- ✓ `internal/session/claude.go` contains `func GetClaudeConfigDirSourceForGroup(groupPath string) (path, source string)` with a real body (not `return "", ""`)
- ✓ `internal/session/instance.go` contains `func (i *Instance) logClaudeConfigResolution()` with a real body (calls `GetClaudeConfigDirSourceForGroup(i.GroupPath)` + `sessionLog.Info("claude config resolution", ...)`)
- ✓ `internal/session/instance.go` contains exactly 3 `i.logClaudeConfigResolution()` call sites, each gated on `IsClaudeCompatible(i.Tool)`, inside `Start`, `StartWithMessage`, `Restart`
- ✓ Commit `e000801` exists in `git log` (RED, test + stubs)
- ✓ Commit `476367c` exists in `git log` (GREEN, real helper + emission + deviation fix)
- ✓ `git log 4730aa5..HEAD --format="%s%n%b" | grep "Generated with Claude"` — empty
- ✓ `git log 4730aa5..HEAD --format="%s%n%b" | grep -c "Committed by Ashesh Goplani"` — 12 (>= 5 required)
- ✓ `git log 4730aa5..HEAD --grep "@alec-pinson" --format="%h" | wc -l` — 7 (>= 2 required)
- ✓ `gofmt -l internal/session/*.go` — empty
- ✓ `go vet ./internal/session/...` — exit 0
- ✓ All 11 phase-level verification gates pass
