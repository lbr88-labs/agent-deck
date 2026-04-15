---
phase: 02-env-file-source-semantics-observability-conductor-e2e
verified: 2026-04-15T00:00:00Z
status: passed
score: 8/8 must-haves verified
overrides_applied: 0
---

# Phase 2: env_file source semantics + observability + conductor E2E — Verification Report

**Phase Goal:** Close CFG-03 (env_file sourced before claude exec for BOTH normal-claude and custom-command/conductor-wrapped spawn paths) + CFG-04 tests 4/5 + CFG-07 observability (single-line `claude config resolution` slog emitted on spawn success/restart success, gated on `IsClaudeCompatible(tool)`, with source label `path|env|group|profile|global|default`).

**Verified:** 2026-04-15
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | env_file sourced before `claude` exec for normal-claude path | VERIFIED | `internal/session/instance.go:477-480` prepends `envPrefix := i.buildEnvSourceCommand()` unconditionally. Test assertion A in `TestPerGroupConfig_EnvFileSourcedInSpawn` (pergroupconfig_test.go:305) matches `source "<path>"` in normal-claude cmd. PASS. |
| 2  | env_file sourced before custom-command / conductor-wrapped exec | VERIFIED | `internal/session/instance.go:599` returns `i.buildEnvSourceCommand() + i.buildBashExportPrefix() + baseCommand` — L599 hardening shipped in commit e608480 (CFG-03 wiring fix, defense-in-depth). Test assertion B confirms `source "<path>"` present in custom-command cmd (pergroupconfig_test.go:318). PASS. |
| 3  | Missing env_file path logs warning and does NOT block spawn | VERIFIED | `internal/session/env.go:152` `buildSourceCmd(path, ignoreMissing=true)` wraps with `[ -f "<path>" ] && source "<path>"` guard. Test at pergroupconfig_test.go:360-374 points env_file at non-existent path, asserts cmd still references path with no fatal. PASS. |
| 4  | Conductor-restart preserves CLAUDE_CONFIG_DIR export | VERIFIED | `TestPerGroupConfig_ConductorRestartPreservesConfigDir` (pergroupconfig_test.go:385) builds the spawn cmd, `ClearUserConfigCache()`, rebuilds; asserts both contain `CLAUDE_CONFIG_DIR=<override>`. PASS. |
| 5  | `GetClaudeConfigDirSourceForGroup(groupPath)` returns `(path, source)` matching priority chain exactly | VERIFIED | `internal/session/claude.go:305-326`. Priority walk (env → group → profile → global → default) mirrors `GetClaudeConfigDirForGroup` at L246. `TestPerGroupConfig_ClaudeConfigDirSourceLabel` (pergroupconfig_test.go:459) runs 5 subtests covering every priority level — all PASS. |
| 6  | Private helper `(i *Instance) logClaudeConfigResolution()` owns the single `"claude config resolution"` slog literal | VERIFIED | Declared at `internal/session/instance.go:623-631`. Audit: `grep -c 'func (i \*Instance) logClaudeConfigResolution' instance.go` = 1; `grep -c '"claude config resolution"' instance.go` = 1. Emits `session`, `group`, `resolved`, `source` via `slog.String`. |
| 7  | CFG-07 emission from EXACTLY three sites (Start, StartWithMessage, Restart), each gated on `IsClaudeCompatible(i.Tool)` — NOT in buildClaudeCommand, buildBashExportPrefix, buildClaudeForkCommandForTarget, Fork | VERIFIED | `grep -cE 'i\.logClaudeConfigResolution\(\)' instance.go` = 3. Call sites: L1955 (Start), L2079 (StartWithMessage), L4118 (Restart). Each wrapped in `if IsClaudeCompatible(i.Tool) { ... }` (L1954, L2078, L4117). No emission from builder or Fork paths (grep confirms). |
| 8  | Rendered slog line matches spec format `claude config resolution: session=<id> group=<g> resolved=<path> source=<env\|group\|profile\|global\|default>` | VERIFIED | `TestPerGroupConfig_ClaudeConfigResolutionLogFormat` (pergroupconfig_test.go:597) swaps `sessionLog` with a `bytes.Buffer`-backed `slog.NewTextHandler` and regex-matches the rendered line against `claude config resolution.*session=\S+\s+group=\S*\s+resolved=\S+\s+source=(env\|group\|profile\|global\|default)`. PASS. |

**Score:** 8/8 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/session/pergroupconfig_test.go` | Contains TestPerGroupConfig_EnvFileSourcedInSpawn, TestPerGroupConfig_ConductorRestartPreservesConfigDir, TestPerGroupConfig_ClaudeConfigDirSourceLabel, TestPerGroupConfig_ClaudeConfigResolutionLogFormat | VERIFIED | All four tests present (grep confirms 8 total `^func TestPerGroupConfig_` entries at lines 22, 77, 135, 187, 254, 385, 459, 597). |
| `internal/session/claude.go` | Contains `GetClaudeConfigDirSourceForGroup(groupPath) (path, source string)` with real priority-walk body (not stub) | VERIFIED | L305-326. Real priority chain: env → group → profile → global → default. Returns `(path, source)` with literal source labels. |
| `internal/session/instance.go` | Contains `logClaudeConfigResolution` helper + CFG-03 L599 fix + 3 gated call sites | VERIFIED | L599 custom-command return prepends `buildEnvSourceCommand()` (CFG-03). L623-631 helper body. L1955/L2079/L4118 call sites, each gated on `IsClaudeCompatible(i.Tool)`. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `instance.go:Start` (L1955) | `(i *Instance) logClaudeConfigResolution` | direct call, gated on `IsClaudeCompatible(i.Tool)` at L1954 | WIRED | Emission site #1, placed immediately after `i.tmuxSession.Start(command)` success. |
| `instance.go:StartWithMessage` (L2079) | `(i *Instance) logClaudeConfigResolution` | direct call, gated on `IsClaudeCompatible(i.Tool)` at L2078 | WIRED | Emission site #2, sister path to Start. |
| `instance.go:Restart` (L4118) | `(i *Instance) logClaudeConfigResolution` | direct call, gated on `IsClaudeCompatible(i.Tool)` at L4117 | WIRED | Emission site #3, after successful restart tmux `Start(command)`. |
| `logClaudeConfigResolution` (L624) | `GetClaudeConfigDirSourceForGroup(i.GroupPath)` | direct function call | WIRED | Helper body calls the priority-chain resolver and passes result to `sessionLog.Info`. |
| `instance.go:599` (custom-command return) | `env.go:buildEnvSourceCommand` | prepended via `i.buildEnvSourceCommand()` | WIRED | CFG-03 fix (commit e608480). Defense-in-depth; outer `buildClaudeCommand` wrapper at L477-480 also prepends envPrefix. |
| `env.go:getToolEnvFile` | `config.GetGroupClaudeEnvFile(i.GroupPath)` | existing at env.go:248 (PR #578 base) | WIRED | Group-level env_file override takes precedence over global. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| All `TestPerGroupConfig_*` tests GREEN under -race -count=1 | `go test -count=1 -race -run "TestPerGroupConfig_" ./internal/session/` | `ok  github.com/asheshgoplani/agent-deck/internal/session	1.063s` (and `-v` run shows all 8 funcs + 5 SourceLabel subtests PASS) | PASS |
| PR #578 regression tests remain GREEN | `go test -count=1 -race -run "TestGetClaudeConfigDirForGroup_GroupWins\|TestIsClaudeConfigDirExplicitForGroup" ./internal/session/` | `ok  github.com/asheshgoplani/agent-deck/internal/session	1.135s` | PASS |
| CFG-07 call-site audit: exactly 3 | `grep -cE 'i\.logClaudeConfigResolution\(\)' internal/session/instance.go` | `3` | PASS |
| CFG-07 declaration count: exactly 1 | `grep -c 'func (i \*Instance) logClaudeConfigResolution' internal/session/instance.go` | `1` | PASS |
| CFG-07 literal count: exactly 1 | `grep -c '"claude config resolution"' internal/session/instance.go` | `1` | PASS |
| Attribution: at least one @alec-pinson trailer in phase 02 commits | `git log 6a0205d..HEAD --format="%s%n%b" \| grep -c "Base implementation by @alec-pinson in PR #578"` | `3` | PASS |
| No Claude attribution in phase 02 commits | `git log 6a0205d..HEAD --format="%s%n%b" \| grep -cE "🤖\|Co-Authored-By: Claude\|Generated with Claude"` | `0` | PASS |

### `go test -v` Output (excerpt)

```
=== RUN   TestPerGroupConfig_CustomCommandGetsGroupConfigDir       --- PASS (0.00s)
=== RUN   TestPerGroupConfig_GroupOverrideBeatsProfile             --- PASS (0.00s)
=== RUN   TestPerGroupConfig_UnknownGroupFallsThroughToProfile     --- PASS (0.00s)
=== RUN   TestPerGroupConfig_CacheInvalidation                     --- PASS (0.00s)
=== RUN   TestPerGroupConfig_EnvFileSourcedInSpawn                 --- PASS (0.00s)
=== RUN   TestPerGroupConfig_ConductorRestartPreservesConfigDir    --- PASS (0.00s)
=== RUN   TestPerGroupConfig_ClaudeConfigDirSourceLabel            --- PASS (0.00s)
    --- PASS: env_var_wins                    (0.00s)
    --- PASS: group_wins_over_profile_global  (0.00s)
    --- PASS: profile_wins_over_global        (0.00s)
    --- PASS: global_fallback                 (0.00s)
    --- PASS: default_fallback                (0.00s)
=== RUN   TestPerGroupConfig_ClaudeConfigResolutionLogFormat       --- PASS (0.00s)
PASS
ok  github.com/asheshgoplani/agent-deck/internal/session  1.047s
```

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| CFG-03 | 02-01-PLAN.md | `env_file` sourced before claude exec for both normal-claude and custom-command paths; missing file warns + continues | SATISFIED | `instance.go:599` prepends `buildEnvSourceCommand()`. `env.go:152` `buildSourceCmd(..., ignoreMissing=true)` emits `[ -f ... ] && source ...` guard. TestPerGroupConfig_EnvFileSourcedInSpawn locks both paths + missing-file case (pergroupconfig_test.go:254-380). |
| CFG-04-4 | 02-01-PLAN.md | `TestPerGroupConfig_EnvFileSourcedInSpawn` with 3-layer assertion (normal + custom + runtime bash exec) | SATISFIED | pergroupconfig_test.go:254-383. Assertions A (normal-claude string match), B (custom-command string match), C (runtime bash -c + echo `$TEST_ENVFILE_VAR`). |
| CFG-04-5 | 02-02-PLAN.md | `TestPerGroupConfig_ConductorRestartPreservesConfigDir` — build → ClearUserConfigCache → rebuild; CLAUDE_CONFIG_DIR preserved | SATISFIED | pergroupconfig_test.go:385-457. Uses `NewInstanceWithGroupAndTool` with non-empty `Command` to exercise custom-command path; asserts both spawn cmds contain `CLAUDE_CONFIG_DIR=<override>`. |
| CFG-07 | 02-02-PLAN.md | Single `claude config resolution` slog line emitted on spawn (Start/StartWithMessage/Restart) gated on `IsClaudeCompatible(tool)` with priority-level source label | SATISFIED | `GetClaudeConfigDirSourceForGroup` at claude.go:305. `logClaudeConfigResolution` at instance.go:623. 3 gated call sites (L1955/L2079/L4118). `TestPerGroupConfig_ClaudeConfigDirSourceLabel` (5 subtests) + `TestPerGroupConfig_ClaudeConfigResolutionLogFormat` (regex format lock). |

**Orphaned requirements check:** REQUIREMENTS.md maps CFG-03, CFG-04 (tests 4/5), and CFG-07 to Phase 2. All four IDs are claimed by 02-01 or 02-02 PLAN.md frontmatter. No orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/session/env.go | 152-158 | `buildSourceCmd` embeds user-controlled path in double-quoted shell string without escaping | Warning (WR-01, advisory) | Pre-existing from PR #578; phase 02 widened blast radius via new custom-command call site. Paths with `` ` ``, `$`, `\` could expand. Threat model: typos/weird paths, not RCE. NOT a phase-blocking gap — the warning pre-exists and REQUIREMENTS.md does not list shell-quoting hardening as a CFG-03 acceptance criterion. Advisory only. |
| internal/session/instance.go | 503, 607 | `CLAUDE_CONFIG_DIR=%s` export is unquoted; breaks on paths with spaces | Warning (WR-02, advisory) | Pre-existing pattern (L607 inside `buildBashExportPrefix` pre-dates phase 02). L599 hardening chains through `buildBashExportPrefix` on the custom-command path — widens blast radius but does not introduce the bug. Not listed as a phase-02 acceptance criterion. Advisory only. |
| internal/session/claude.go | 305-326 | `GetClaudeConfigDirSourceForGroup` duplicates the resolver chain from `GetClaudeConfigDirForGroup` (L246-267) and `IsClaudeConfigDirExplicitForGroup` (L271-291) | Warning (WR-03, advisory) | Acknowledged in code comment at L302-304. Duplication is a future-drift risk (a new priority level could be added to only one of the three). Plan explicitly called for this as a new helper; refactor to delegate via `path, _ := GetClaudeConfigDirSourceForGroup(...)` is a follow-up, not a phase-02 gap. Advisory only. |
| internal/session/instance.go | 1954-1955, 2078-2079, 4117-4118 | `logClaudeConfigResolution` emits only after successful `tmuxSession.Start(command)`; failed-start paths never emit the resolution line | Info (IN-01, advisory) | Matches ROADMAP SC #3 wording ("emitted on every session spawn") — a failed spawn arguably is not a completed spawn. Plan 02-02 intentionally placed the call site post-Start; no acceptance criterion required the line to emit on failure. Advisory; future enhancement could move the call to pre-Start + Warn on `source=default` for Claude-compatible tools. |
| internal/session/pergroupconfig_test.go | 324-340 | Runtime-proof harness (assertion C) locates payload via `strings.LastIndex(cmdCustom, "bash -c 'exec claude'")` — fragile to builder quoting changes | Info (IN-03, advisory) | Test fatals with a clear message ("could not locate custom-command payload") if the LastIndex lookup returns -1, so the failure mode is diagnosable. Future builder changes that alter quoting would require updating the test; not a correctness bug, just brittle coupling. Advisory only. |
| internal/session/instance.go | 625-629 | Log injection theoretically possible if `i.GroupPath` contains control chars and a future handler swap occurs | Info (IN-02, advisory) | Default `slog.NewTextHandler` quotes safely. Future JSON-handler or custom-handler swap could expose injection via malicious `GroupPath` (sourced from config.toml, user-controlled). Advisory; defensive fix is input validation at `NewInstanceWithGroupAndTool` ingest. |
| internal/session/claude.go | 271-291 | `IsClaudeConfigDirExplicitForGroup` open-codes the priority traversal a third time | Info (IN-04, advisory) | Related to WR-03; folds into the same WR-03 refactor. Pre-existing from PR #578. Advisory only. |

All 7 findings from 02-REVIEW.md are **advisory** (3 Warning + 4 Info, 0 Critical). None gate phase 02 goal achievement; REVIEW.md itself classifies them as follow-up hardening rather than phase-02 blockers. All are documented here for traceability.

### Human Verification Required

_None required._ All Phase 2 must-haves are automated-asserted by the test suite; the rendered slog line is regex-locked by `TestPerGroupConfig_ClaudeConfigResolutionLogFormat`; the runtime-proof harness (assertion C in `TestPerGroupConfig_EnvFileSourcedInSpawn`) executes the built command under `bash -c` and verifies the sentinel env var, providing end-to-end proof without human intervention. Conductor-host manual proof (spec item 4: `ps -p <pane_pid>` env check on live conductor) is explicitly deferred to `/gsd-complete-milestone` per 02-CONTEXT.md deferred block and is a milestone-level check, not a phase-02 verification.

### Gaps Summary

_No gaps found._ All 8 must-haves VERIFIED. All 4 requirement IDs (CFG-03, CFG-04-4, CFG-04-5, CFG-07) SATISFIED. The 7 findings from 02-REVIEW.md are advisory (pre-existing shell-quoting hygiene and future-drift risks around a documented intentional duplication) and do not block the phase goal. Test suite (`go test -count=1 -race -run "TestPerGroupConfig_" ./internal/session/`) returns `ok ... 1.063s` with all 8 test functions + 5 `ClaudeConfigDirSourceLabel` subtests PASS. PR #578 regression subset (`TestGetClaudeConfigDirForGroup_GroupWins`, `TestIsClaudeConfigDirExplicitForGroup`) remains GREEN. Attribution and sign-off rules observed on all 7 phase-02 commits; 3 commits carry `Base implementation by @alec-pinson in PR #578.`; 0 commits carry Claude attribution.

---

_Verified: 2026-04-15_
_Verifier: Claude (gsd-verifier)_
