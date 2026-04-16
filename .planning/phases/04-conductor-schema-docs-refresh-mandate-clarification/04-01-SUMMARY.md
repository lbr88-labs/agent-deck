---
phase: 04-conductor-schema-docs-refresh-mandate-clarification
plan: 01
subsystem: config
tags: [conductor, claude, per-group-config, toml-schema, tdd, cfg-08, cfg-11]

requires:
  - phase: 01-custom-command-injection-core-regression-tests
    provides: per-group Claude config_dir injection on custom-command spawn path (CFG-02)
  - phase: 02-env-file-source-semantics-observability-conductor-e2e
    provides: env_file sourcing on normal+custom spawn paths (CFG-03) and CFG-07 source-label helper

provides:
  - "Conductors map on UserConfig with ConductorOverrides / ConductorClaudeSettings TOML schema"
  - "GetClaudeConfigDirForInstance + GetClaudeConfigDirSourceForInstance + IsClaudeConfigDirExplicitForInstance loader triplet"
  - "Four instance.go callsite swaps including the resume path (buildClaudeResumeCommand L4172)"
  - "Eight locked CFG-11 TestConductorConfig_* regression tests in conductorconfig_test.go"
  - "conductor env_file branch in env.go getToolEnvFile"

affects:
  - 04-02 (docs + mandate — must document nested [conductors.<name>.claude] form)
  - Future milestones where [conductors.<name>] grows beyond the Claude sub-block

tech-stack:
  added: []
  patterns:
    - "Instance-aware config resolution: callers pass *Instance, loader derives conductor-name via TrimPrefix"
    - "Nested [<group>.<name>.claude] sub-table mirrors [groups.\"<g>\".claude] pattern"
    - "RED-gate stubs in production files (Task 1) that Task 2/3 replace with real bodies"

key-files:
  created:
    - internal/session/conductorconfig_test.go
    - .planning/phases/04-conductor-schema-docs-refresh-mandate-clarification/deferred-items.md
  modified:
    - internal/session/userconfig.go
    - internal/session/claude.go
    - internal/session/instance.go
    - internal/session/env.go

key-decisions:
  - "Nested [conductors.<name>.claude] TOML shape (NOT flat [conductors.<name>]) — mirrors [groups.\"<g>\".claude] for internal consistency. Plan 04-02 docs MUST use this nested form."
  - "Renamed per-conductor type from ConductorSettings to ConductorOverrides — ConductorSettings already exists in conductor.go:49 for the global [conductor] bot block. Go does not allow duplicate type declarations."
  - "conductorNameFromInstance helper in claude.go mirrors env.go:267 getConductorEnv TrimPrefix pattern — single source of truth for conductor-name derivation."
  - "GetClaudeConfigDirForGroup / IsClaudeConfigDirExplicitForGroup / GetClaudeConfigDirSourceForGroup preserved unchanged for callers without an *Instance. Four instance.go callsites all swapped to Instance-aware variants."
  - "Resume path (buildClaudeResumeCommand at instance.go:4172) was the fourth critical callsite — without it, restart of gsd-v154 conductor falls through to group-only resolution and milestone success criterion #8 silently breaks."

patterns-established:
  - "Per-conductor TOML: [conductors.<name>.claude] with config_dir and env_file mirroring [groups.\"<g>\".claude]"
  - "Loader pair pattern: GetClaudeConfigDirForGroup (legacy) + GetClaudeConfigDirForInstance (conductor-aware) — kept in sync, same priority chain plus one extra branch"
  - "Observability source-label extension: env|conductor|group|profile|global|default"

requirements-completed: [CFG-08, CFG-11]

duration: 8min
completed: 2026-04-15
---

# Phase 04 Plan 01: Conductor Schema + Loader + Tests Summary

**Per-conductor `[conductors.<name>.claude]` TOML schema with Instance-aware loader, four callsite swaps (including buildClaudeResumeCommand for resume-path safety), and eight CFG-11 regression tests — all 8 GREEN, all 8 Phase 1-2 TestPerGroupConfig_* still GREEN.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-15T21:34:12Z
- **Completed:** 2026-04-15T21:42:46Z
- **Tasks:** 4 (RED → GREEN step 1 → GREEN step 2 → regression sweep)
- **Files modified:** 4 (+ 1 created + 1 deferred-items doc)

## Accomplishments

- CFG-08 closed: conductors are first-class entities that can carry their own Claude `config_dir` and `env_file` via `[conductors.<name>.claude]`.
- CFG-11 closed: eight `TestConductorConfig_*` regression tests land in a dedicated file and pass.
- Milestone success criterion #8 protected: `buildClaudeResumeCommand` now consults the Instance-aware loader, so restart of the `gsd-v154` conductor reports `~/.claude-work`.
- Phase 1-2 regression gate preserved: all 8 `TestPerGroupConfig_*` tests still GREEN, confirming non-conductor sessions resolve identically to PR #578.

## Task Commits

1. **Task 1: RED — eight CFG-11 tests + compile stubs** — `41c9b8e` (test)
   - `internal/session/conductorconfig_test.go` (8 TestConductorConfig_* with nested `[conductors.<name>.claude]` fixtures)
   - `internal/session/userconfig.go` (ConductorOverrides + ConductorClaudeSettings + Conductors map; GetConductor* helpers STUB returning "")
   - `internal/session/claude.go` (GetClaudeConfigDirForInstance + GetClaudeConfigDirSourceForInstance STUBS returning "")
   - Verified: 8/8 FAIL with assertion errors; package compiles clean.

2. **Task 2: GREEN step 1 — schema helpers** — `f0cf791` (feat)
   - `internal/session/userconfig.go` GetConductorClaudeConfigDir + GetConductorClaudeEnvFile bodies filled in.
   - Verified: `TestConductorConfig_SchemaParses` GREEN; tests 2-8 still FAIL (loader not yet wired).

3. **Task 3: GREEN step 2 — loader + four callsite swaps + env.go** — `6fdac26` (feat)
   - `internal/session/claude.go` — real bodies for GetClaudeConfigDirForInstance, GetClaudeConfigDirSourceForInstance, plus new IsClaudeConfigDirExplicitForInstance + conductorNameFromInstance helper.
   - `internal/session/instance.go` — four callsites swapped (L501 buildClaudeCommandWithMessage, L606 buildBashExportPrefix, L624 logClaudeConfigResolution, L4172 buildClaudeResumeCommand).
   - `internal/session/env.go` — getToolEnvFile `case "claude":` now consults `[conductors.<name>.claude].env_file` before the group block.
   - Verified: 8/8 CFG-11 GREEN, 8/8 TestPerGroupConfig_* GREEN.

4. **Task 4: Regression sweep + SUMMARY** — pending metadata commit.

## Four-callsite Swap Record

All four callsites in `internal/session/instance.go` now use the Instance-aware loader. Zero `GetClaudeConfigDirForGroup` / `IsClaudeConfigDirExplicitForGroup` / `GetClaudeConfigDirSourceForGroup` references remain in `instance.go` (verified via `grep`).

| Line | Function | Path purpose | Protects |
| --- | --- | --- | --- |
| L501 | `buildClaudeCommandWithMessage` | Normal-claude spawn | CFG-08 spawn env |
| L606 | `buildBashExportPrefix` | Custom-command (conductor wrapper) spawn | CFG-02 + CFG-08 combined |
| L624 | `logClaudeConfigResolution` | CFG-07 observability | Correct source label emission |
| **L4172** | **`buildClaudeResumeCommand`** | **RESUME / RESTART (called by `Restart()` L4057 + container spawn L3827)** | **Milestone success criterion #8 — restart `gsd-v154` conductor → reports `~/.claude-work`** |

The L4172 swap is the critical fourth callsite. Without it, `Restart()` of a conductor session would fall through to group-only resolution, silently losing the `[conductors.<name>]` override. CFG-11 test 6 sub-assertion 6c exercises this path explicitly.

## Files Created/Modified

- `internal/session/conductorconfig_test.go` — created; 8 TestConductorConfig_* tests, all fixtures use nested `[conductors.<name>.claude]` form.
- `internal/session/userconfig.go` — added `Conductors map[string]ConductorOverrides` + `ConductorOverrides` + `ConductorClaudeSettings` types + 2 Get helpers.
- `internal/session/claude.go` — added `conductorNameFromInstance`, `GetClaudeConfigDirForInstance`, `GetClaudeConfigDirSourceForInstance`, `IsClaudeConfigDirExplicitForInstance`. Legacy `...ForGroup` functions preserved unchanged.
- `internal/session/instance.go` — four callsite swaps documented above.
- `internal/session/env.go` — conductor branch added to `getToolEnvFile` `case "claude":`.
- `.planning/phases/04-conductor-schema-docs-refresh-mandate-clarification/deferred-items.md` — logs 6 pre-existing tmux-dependent test failures (not Phase 04 regressions).

## Decisions Made

See frontmatter `key-decisions`. The two highest-impact:

1. **Nested TOML shape:** `[conductors.<name>.claude]` (sub-table), not flat `[conductors.<name>]`. Spec was ambiguous; plan resolved in favor of mirroring `[groups."<g>".claude]` for internal consistency. Plan 04-02's README/CHANGELOG/SKILL docs MUST use this nested form.
2. **Type rename:** `ConductorSettings` already exists in `conductor.go:49` for the global `[conductor]` bot block (heartbeat, telegram, slack, discord). Go cannot redeclare; per-conductor struct renamed to `ConductorOverrides` — see deviations below.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] Per-conductor struct renamed from `ConductorSettings` → `ConductorOverrides`**
- **Found during:** Task 1 (compile-stub pass)
- **Issue:** Plan Task 2 text says `type ConductorSettings struct` in userconfig.go. But `ConductorSettings` is already declared in `internal/session/conductor.go:49` (global conductor bot orchestration block — heartbeat, telegram, slack, discord). Go does not permit duplicate type declarations in the same package. Plan acceptance criterion `grep -q "type ConductorSettings struct" internal/session/userconfig.go` is self-contradicting.
- **Fix:** Used `ConductorOverrides` as the per-conductor struct name and `Conductors map[string]ConductorOverrides` as the map type. Behavior, TOML shape, and helper signatures all match the plan; only the type name changes.
- **Files modified:** `internal/session/userconfig.go`, `internal/session/conductorconfig_test.go` (not affected — tests never reference the type by name, only by field access `cfg.Conductors["foo"].Claude.ConfigDir`).
- **Verification:** `go build ./internal/session/...` exits 0; `grep -q "type ConductorOverrides struct" internal/session/userconfig.go`; `grep -q "Conductors map\[string\]ConductorOverrides" internal/session/userconfig.go`.
- **Committed in:** `41c9b8e` (Task 1 stub), `f0cf791` (Task 2 real bodies), both with explanatory notes in the commit body.

**2. [Rule 3 — Blocking] Test 1 (SchemaParses) reached RED via stubbed helpers, not stubbed struct**
- **Found during:** Task 1 RED run
- **Issue:** Plan expected 8/8 FAIL in Task 1. But the schema field (`Conductors map[...]`) must be present for the test to compile (it accesses `cfg.Conductors["foo"].Claude.ConfigDir` directly). If the TOML parses and the helper works, test 1 is GREEN before Task 2 even runs.
- **Fix:** Added the `Conductors` field + types in Task 1 (needed for compile), but stubbed `GetConductorClaudeConfigDir` / `GetConductorClaudeEnvFile` to return `""`. Test 1's final assertion `cfg.GetConductorClaudeConfigDir("foo") == "/tmp/x"` fails against the stub → RED. Task 2 fills in the helper bodies → GREEN.
- **Verification:** `grep -c "^--- FAIL: TestConductorConfig_" /tmp/conductorconfig-red.log` returned `8` after Task 1.
- **Committed in:** `41c9b8e` (stubs), `f0cf791` (real helper bodies).

**3. [Rule 3 — Scope boundary] Pre-existing tmux-dependent test failures logged to deferred-items.md**
- **Found during:** Task 4 full-suite regression sweep
- **Issue:** 6 tests in `instance_test.go` / `instance_platform_test.go` FAIL with `SetEnvironment failed: exit status 1` — they require a live tmux server.
- **Verification:** `git stash` + re-run on unchanged tree at `6fdac26`: same 6 tests still FAIL. Confirmed pre-existing, not a Phase 04 regression.
- **Fix:** Logged to `.planning/phases/04-conductor-schema-docs-refresh-mandate-clarification/deferred-items.md`. Not auto-fixed (out of Phase 04 scope per scope-boundary rule).

---

**Total deviations:** 3 auto-fixed (3 × Rule 3 — blocking/scope-boundary). No Rule 4 architectural stops.
**Impact on plan:** All deviations are mechanical (rename, sequencing) or out-of-scope-tmux-failures. No semantic divergence — the shipped schema, loader, and test behaviors match the plan intent exactly.

## Issues Encountered

None beyond the three deviations documented above.

## Acceptance Criteria Verification

| Plan criterion | Status |
| --- | --- |
| `grep -c "^func TestConductorConfig_" conductorconfig_test.go == 8` | PASS |
| `grep -q buildClaudeResumeCommand conductorconfig_test.go` | PASS |
| `go test -run TestConductorConfig_ -race -count=1` → 8/8 GREEN | PASS |
| `go test -run TestPerGroupConfig_ -race -count=1` → 8/8 GREEN | PASS |
| `go build ./...` | PASS |
| `go vet ./internal/session/...` | PASS |
| Four callsite swap in instance.go, zero Group-only refs remaining | PASS |
| New source label `"conductor"` returned by SourceForInstance | PASS |
| Env_file sourced on both spawn paths + runtime proof | PASS |
| Every Phase 04 commit signed `Committed by Ashesh Goplani` | PASS |
| Zero Claude attribution / zero @alec-pinson in Phase 04 commits | PASS |
| Issue #602 referenced in at least one Phase 04 commit body | PASS (all three) |
| `grep "type ConductorSettings struct" userconfig.go` | **N/A — deviated to ConductorOverrides (see Deviation 1)** |

## Schema Shape Lock-in for Plan 04-02

Plan 04-02 docs (README, CHANGELOG, SKILL.md) MUST use the nested form:

```toml
[conductors.<name>.claude]
config_dir = "~/.claude-work"
env_file   = "~/git/work/.envrc"
```

NOT the flat form `[conductors.<name>]` with direct `config_dir` / `env_file` at the top level. The nested shape mirrors `[groups."<g>".claude]` and matches the `ConductorOverrides.Claude.ConfigDir` Go struct path.

## Next Phase Readiness

Plan 04-02 (docs + mandate clarification) is unblocked. It needs to:

- Update `README.md` / `CHANGELOG.md` / agent-deck SKILL.md examples using the nested `[conductors.<name>.claude]` shape.
- Reference the struct name `ConductorOverrides` (not `ConductorSettings`) if any docs cite Go type names.
- Clarify the `--no-verify` mandate wording in `CLAUDE.md` to permit metadata-only commits.

## Self-Check: PASSED

- All created files exist on disk (conductorconfig_test.go, 04-01-SUMMARY.md, deferred-items.md).
- All three commit hashes (`41c9b8e`, `f0cf791`, `6fdac26`) are reachable from HEAD.
- Zero `alec-pinson` and zero `Co-Authored-By: Claude` in Phase 04 commit bodies.
- Three Phase 04 commits, all signed `Committed by Ashesh Goplani <ashesh.goplani96@gmail.com>`.
- Three Phase 04 commits, all reference `issues/602`.
- Test pass counts: CFG-11 = 8/8, TestPerGroupConfig_* = 8/8, FAIL = 0.

---

*Phase: 04-conductor-schema-docs-refresh-mandate-clarification*
*Completed: 2026-04-15*
