# Agent Deck v1.5.4 Roadmap

**Milestone:** v1.5.4 — Per-group Claude Config
**Starting point:** v1.5.3 (`ee7f29e` on `fix/feedback-closeout`)
**Branch:** `fix/per-group-claude-config-v154` (worktree-isolated, forked from `fa9971e` — PR #578 HEAD by @alec-pinson)
**Created:** 2026-04-15
**Granularity:** Small patch (3 phases)
**Estimated duration:** 60–90 minutes
**Parallelization:** None — phases are sequential along TDD seams and dependency order

---

## Executive Summary

v1.5.4 accepts external PR #578 (`feat/per-group-config` by @alec-pinson) as the base and closes the gaps that block adoption for the user's conductor use case. Core value: one agent-deck profile can host groups that authenticate against different Claude config dirs without splitting into more profiles.

Three release-safety anchors apply:

- **Go 1.24.0 toolchain pinned.** Go 1.25 silently breaks macOS TUI (carried from v1.5.0).
- **No `--no-verify`.** Repo-root `CLAUDE.md` mandate from v1.5.3 (commit `ee7f29e`) forbids bypassing pre-commit hooks.
- **No SQLite schema changes.** This milestone touches `internal/session/*` only — no `statedb` migrations.

TDD is non-negotiable: every test in `pergroupconfig_test.go` must be written and must FAIL before the implementation or verification change that makes it pass.

Attribution: at least one commit must carry `Base implementation by @alec-pinson in PR #578.` in the body. No Claude attribution. Sign as "Committed by Ashesh Goplani".

No `git push`, no tags, no PR create, no merge — this is local-only work for review at milestone end.

---

## Phases

- [x] **Phase 1: Custom-command injection + core regression tests** (~13 min actual) — DONE. Four TDD regression tests (CFG-04 tests 1, 2, 3, 6) added in `internal/session/pergroupconfig_test.go`. `buildBashExportPrefix()` now prepended to the custom-command return path at `instance.go:596` (+4/-2 lines). All 4 tests GREEN under `-race -count=1`; PR #578's `TestGetClaudeConfigDirForGroup_GroupWins`, `TestIsClaudeConfigDirExplicitForGroup`, `TestBuildClaudeCommand_CustomAlias`, and all `TestUserConfig_GroupClaude*` tests remain GREEN. Commits: `40f4f04` (RED test) + `b39bbf3` (GREEN fix, carries `Builds on PR #578 by @alec-pinson.`). [REQ mapping: CFG-01, CFG-02, CFG-04 (subset)]

- [ ] **Phase 2: env_file source semantics + observability + conductor E2E** (~25–30 min) — Prove `env_file` is `source`d before `claude` exec in the spawn pipeline for BOTH normal-claude and custom-command paths. Write three TDD regression tests (CFG-04 tests 4, 5 plus the CFG-07 log-format lock). Add the observability log line (CFG-07) via a `logClaudeConfigResolution` helper called from Start/StartWithMessage/Restart. All Go tests green under `-race -count=1`. [REQ mapping: CFG-03, CFG-04 (remainder), CFG-07] — **Plan 02-01 DONE (CFG-03 + CFG-04 test 4 locked in 38a2af3 + e608480); plan 02-02 pending (CFG-04 test 5 + CFG-07 observability).**

- [ ] **Phase 3: Visual harness + documentation + attribution commit** (~15–25 min) — Ship `scripts/verify-per-group-claude-config.sh` (CFG-05), the README / CLAUDE.md / CHANGELOG updates (CFG-06), and an attribution commit referencing @alec-pinson. Run the harness on the conductor host and capture its output. [REQ mapping: CFG-05, CFG-06]

---

## Phase Details

### Phase 1: Custom-command injection + core regression tests

**Goal:** Prove per-group `CLAUDE_CONFIG_DIR` is injected into the tmux spawn env for custom-command (conductor) sessions, and lock that behavior with four regression tests.

**Requirements covered:**
- CFG-01 — PR #578 schema + lookup (verify existing tests stay green; no code changes required here unless Phase 1 uncovers a gap)
- CFG-02 — custom-command sessions receive the override
- CFG-04 (tests 1, 2, 3, 6) — `CustomCommandGetsGroupConfigDir`, `GroupOverrideBeatsProfile`, `UnknownGroupFallsThroughToProfile`, `CacheInvalidation`

**Approach (TDD, in order):**
1. Create `internal/session/pergroupconfig_test.go` with tests 1, 2, 3, 6 — red first (tests compile but fail because either assertions don't hold or helper seams don't exist yet).
2. Run `go test ./internal/session/... -run TestPerGroupConfig_ -race -count=1` — confirm RED.
3. Investigate whether `buildBashExportPrefix` actually exports `CLAUDE_CONFIG_DIR` for custom-command sessions today (spec hints it does, but no test proves it). If the path is live, the tests go green immediately and the phase becomes pure test-authoring. If there's a genuine gap — the prefix isn't applied to custom commands — the minimal fix is to route the export through the tmux pane env injection so it lands before `exec` regardless of `Instance.Command`.
4. Re-run tests — confirm GREEN.
5. Run the full PR #578 test suite (`TestGetClaudeConfigDirForGroup_GroupWins`, `TestIsClaudeConfigDirExplicitForGroup`) — confirm no regressions.

**Scope (files touched):** `internal/session/pergroupconfig_test.go` (new), potentially `internal/session/env.go` and/or `internal/session/instance.go` (minimal injection fix if gap found). No changes to PR #578's existing code unless a test requires it.

**Success criteria:**
1. `internal/session/pergroupconfig_test.go` exists and contains the four named tests listed above.
2. `go test ./internal/session/... -run TestPerGroupConfig_ -race -count=1` — all 4 GREEN.
3. PR #578's existing unit tests (`TestGetClaudeConfigDirForGroup_GroupWins`, `TestIsClaudeConfigDirExplicitForGroup`) remain GREEN.
4. At least one atomic commit per logical change (test addition commit; fix commit if needed); all commits signed "Committed by Ashesh Goplani".
5. `make ci` (or equivalent) passes.

**Dependencies:** None (phase entry point). The branch is already at `fa9971e` which contains PR #578's implementation.

**Plans:** 1 plan

Plans:
- [x] 01-01-PLAN.md — DONE (see `01-01-SUMMARY.md`). Four regression tests (CFG-04 tests 1/2/3/6) added in `40f4f04`, surgical `buildBashExportPrefix()` patch at `instance.go:596` shipped in `b39bbf3`. RED split confirmed (`/tmp/pergroupconfig-red.log`: 2 FAIL / 2 PASS), GREEN gate clean (`/tmp/pergroupconfig-green.log`: 4/4 PASS). PR #578 regression tests all GREEN. `make ci` returns non-zero from six pre-existing tmux-env failures in `internal/session` (verified pre-existing at parent `4730aa5`; logged in `deferred-items.md` — not a Phase-01 regression).

---

### Phase 2: env_file source semantics + observability + conductor E2E

**Goal:** Prove `env_file` is sourced in the tmux spawn pipeline before `claude` exec (for BOTH the normal-claude path and the custom-command/conductor path), add the observability log line emitted from every session-spawn entrypoint, and close the custom-command restart loop with an end-to-end test.

**Requirements covered:**
- CFG-03 — `env_file` sourced before `claude` exec (on both normal-claude and custom-command paths)
- CFG-04 (tests 4, 5) — `EnvFileSourcedInSpawn`, `ConductorRestartPreservesConfigDir`
- CFG-07 — observability log line (emitted from Start, StartWithMessage, and Restart)

**Approach (TDD, in order):**
1. Add test 4 (`TestPerGroupConfig_EnvFileSourcedInSpawn`) — write a throwaway envrc file under `t.TempDir()` that exports a sentinel var; assert the production spawn-command builder (`Instance.buildClaudeCommand`) emits a `source "<path>"` line for BOTH the normal-claude branch (instance.go:478) AND the custom-command branch (instance.go:598). Assertion C runs the built command under `bash -c` and proves the sentinel var is set.
2. Add test 5 (`TestPerGroupConfig_ConductorRestartPreservesConfigDir`) — create a custom-command instance with a group override, build the spawn command, stop, rebuild the spawn command (simulated restart via `ClearUserConfigCache`), assert the override is present in both.
3. Run tests — confirm RED (expect a CFG-03 wiring gap at `instance.go:598` where the custom-command return does not prepend `buildEnvSourceCommand()`; assertion B will fail).
4. Fix the CFG-03 gap with a minimal one-line change at `instance.go:598`: prepend `i.buildEnvSourceCommand()` to the custom-command return so env_file is sourced before the wrapper exec's. Missing file → warning log, not a spawn failure.
5. Add the CFG-07 observability log line. Factor the emission into a private helper `(i *Instance) logClaudeConfigResolution()` that owns the single `"claude config resolution"` slog literal. Call the helper from THREE session-spawn entrypoints — `Start()`, `StartWithMessage()`, `Restart()` — each gated on `IsClaudeCompatible(i.Tool)`. Fork path intentionally silent. Back the helper with a new `GetClaudeConfigDirSourceForGroup(groupPath) (path, source string)` in `claude.go` that returns the resolved path AND the priority-level label (`env|group|profile|global|default`).
6. Add two CFG-07 unit tests alongside test 5: `TestPerGroupConfig_ClaudeConfigDirSourceLabel` (priority-chain label mapping, all 5 levels) and `TestPerGroupConfig_ClaudeConfigResolutionLogFormat` (swaps `sessionLog`'s handler for a `bytes.Buffer`-backed `slog.NewTextHandler` and regex-matches the rendered line against the spec format).
7. Re-run the full `TestPerGroupConfig_*` suite — all 8 GREEN under `go test ./internal/session/... -run TestPerGroupConfig_ -race -count=1` (tests 1/2/3/4/5/6 from the ROADMAP numbering, plus `ClaudeConfigDirSourceLabel` + `ClaudeConfigResolutionLogFormat`).

**Scope (files touched):** `internal/session/pergroupconfig_test.go` (extend), `internal/session/instance.go` (CFG-03 one-line fix at L598 if gap confirmed + new `logClaudeConfigResolution` helper + 3 call sites in Start/StartWithMessage/Restart), `internal/session/claude.go` (new `GetClaudeConfigDirSourceForGroup` helper). `internal/session/env.go` touched only if CFG-03 diagnosis reveals a deeper gap than the L598 wiring.

**Success criteria:**
1. All 8 `TestPerGroupConfig_*` tests GREEN under `-race -count=1` (six ROADMAP-numbered tests 1/2/3/4/5/6 + two CFG-07 helper tests: `ClaudeConfigDirSourceLabel` + `ClaudeConfigResolutionLogFormat`).
2. `env_file` with `.envrc` or flat `KEY=VALUE` format has its exports visible in the spawn env on BOTH the normal-claude path and the custom-command (conductor) path. Missing file logs a warning and does not block.
3. Observability log line is emitted on every session spawn (Start, StartWithMessage, AND Restart) with the correct `source=` attribution, owned by a single private helper so the `"claude config resolution"` literal appears exactly once in the package.
4. Atomic commits per logical change, signed "Committed by Ashesh Goplani". Fix commits carry `Base implementation by @alec-pinson in PR #578.`
5. `make ci` passes.

**Dependencies:** Phase 1 complete (shared test file; Phase 2 extends it).

**Plans:** 2 plans

Plans:
- [x] 02-01-PLAN.md — DONE (see `02-01-SUMMARY.md`). TestPerGroupConfig_EnvFileSourcedInSpawn added in `38a2af3` (138 lines); pre-authorized L599 hardening shipped in `e608480` (+4/-3 lines, carries `Base implementation by @alec-pinson in PR #578.`). All 5 TestPerGroupConfig_* tests GREEN under `-race -count=1`; PR #578 regression subset (7 tests) GREEN. Diagnosis: outer `buildClaudeCommand` wrapper at L477-480 already prepends envPrefix unconditionally, so the L599 change is defense-in-depth against future callsites that bypass `buildClaudeCommand`. Three new pre-existing StatusEventWatcher fsnotify-timeout failures logged to Phase 02 deferred-items.md (not a regression).
- [ ] 02-02-PLAN.md — CFG-04 test 5 + CFG-07: conductor-restart regression test + source-label helper (`GetClaudeConfigDirSourceForGroup`) in claude.go + private `logClaudeConfigResolution` helper in instance.go emitted from THREE sites (Start, StartWithMessage, Restart). Two CFG-07 unit tests (`ClaudeConfigDirSourceLabel` priority-chain, `ClaudeConfigResolutionLogFormat` slog text-handler format lock).

---

### Phase 3: Visual harness + documentation + attribution commit

**Goal:** Ship the human-watchable verification script, update all three doc surfaces (README, CLAUDE.md, CHANGELOG), and record attribution to @alec-pinson in at least one commit.

**Requirements covered:**
- CFG-05 — visual harness `scripts/verify-per-group-claude-config.sh`
- CFG-06 — README subsection, CLAUDE.md one-liner, CHANGELOG bullet, attribution commit

**Approach (ordered):**
1. Write `scripts/verify-per-group-claude-config.sh`. Structure:
   - `set -euo pipefail`; capture original `~/.agent-deck/config.toml` to a temp backup (or use a dedicated test config via `AGENT_DECK_CONFIG` if supported).
   - Create two throwaway groups `verify-group-a` (config_dir `~/.claude`) and `verify-group-b` (config_dir `~/.claude-work`).
   - Launch one session per group — one normal `claude`, one custom-command (e.g. `bash -c 'exec claude'` wrapper).
   - `agent-deck session send <id> "echo CLAUDE_CONFIG_DIR=\$CLAUDE_CONFIG_DIR"`; capture output via `agent-deck session output`.
   - Print a pass/fail table (aligned columns, color for TTY, plain for redirect).
   - Exit 0 iff both sessions show expected values; exit 1 otherwise.
   - `trap` cleanup: stop both sessions, restore config backup. Use `trash` not `rm`.
2. Run the harness once on the conductor host; capture stdout into the phase artifact (not the commit).
3. Update `README.md` — add subsection "Per-group Claude config" under Configuration with the example from PR #578 and a pointer to `scripts/verify-per-group-claude-config.sh`.
4. Update repo-root `CLAUDE.md` — one-line entry under the session-persistence mandate block: "Per-group config dir applies to custom-command sessions too; `TestPerGroupConfig_*` suite enforces this."
5. Update `CHANGELOG.md` — `[Unreleased] > Added` bullet: `Per-group Claude config overrides ([groups."<name>".claude]).`
6. Finalize with an attribution commit — either a dedicated commit or inserted in the body of the CHANGELOG commit — carrying: `Base implementation by @alec-pinson in PR #578.` Sign "Committed by Ashesh Goplani".

**Scope (files touched):** `scripts/verify-per-group-claude-config.sh` (new, `chmod +x`), `README.md`, `CLAUDE.md` (repo root), `CHANGELOG.md`.

**Success criteria:**
1. `bash scripts/verify-per-group-claude-config.sh` exits 0 on conductor host with a visible pass/fail table for both sessions.
2. `README.md` has the new "Per-group Claude config" subsection with the `[groups."conductor".claude]` TOML example.
3. Repo-root `CLAUDE.md` has the one-line `TestPerGroupConfig_*` enforcement entry under the session-persistence mandate block.
4. `CHANGELOG.md` has the `[Unreleased] > Added` bullet for per-group Claude config overrides.
5. `git log main..HEAD --grep "@alec-pinson"` returns at least one commit. Sign "Committed by Ashesh Goplani"; no Claude attribution.
6. No `git push`, `git tag`, `gh release`, `gh pr create`, `gh pr merge` executed during this milestone.

**Dependencies:** Phases 1 and 2 complete (tests and implementation must exist before the harness can prove end-to-end behavior and before CLAUDE.md can claim `TestPerGroupConfig_*` enforcement).

---

## Milestone Verification (runs at `/gsd-complete-milestone`)

Recap of the six success criteria from the spec — the audit step will confirm all six:

1. PR #578 unit tests remain GREEN.
2. `go test ./internal/session/... -run TestPerGroupConfig_ -race -count=1` — all 8 GREEN (six ROADMAP-numbered tests 1/2/3/4/5/6 + two CFG-07 helper tests).
3. `bash scripts/verify-per-group-claude-config.sh` exits 0 on conductor host.
4. Manual conductor proof: `ps -p <pane_pid>` env shows the overridden `CLAUDE_CONFIG_DIR` after restart.
5. Commit log includes README + CHANGELOG + CLAUDE.md commits and at least one `@alec-pinson` attribution commit.
6. No push / tag / PR / merge performed.

---

## Carry-forward notes

- **v1.5.3 mandate (repo-root `CLAUDE.md`):** No `--no-verify`. Every commit goes through pre-commit hooks.
- **Commit signature:** "Committed by Ashesh Goplani". No Claude attribution.
- **Scope discipline:** Any change outside the spec's scope list is escalation-worthy, not drift-worthy.
- **Rebase posture:** `fa9971e` is behind current `main`. Rebase is a merge-time concern — NOT this milestone's scope.

---

*Roadmap created: 2026-04-15*
*Last updated: 2026-04-15 — Phase 2 revision (iteration 1/3): env_file wiring fix for custom-command path at instance.go:598; CFG-07 factored into logClaudeConfigResolution helper called from Start/StartWithMessage/Restart; added TestPerGroupConfig_ClaudeConfigResolutionLogFormat automated format lock*
