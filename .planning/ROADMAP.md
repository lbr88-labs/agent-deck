# Agent Deck v1.5.4 Roadmap

**Milestone:** v1.5.4 — Per-group Claude Config
**Starting point:** v1.5.3 (`ee7f29e` on `fix/feedback-closeout`)
**Branch:** `fix/per-group-claude-config-v154` (worktree-isolated, forked from `fa9971e` — PR #578 HEAD by @alec-pinson)
**Created:** 2026-04-15
**Granularity:** Small patch (4 phases — Phase 4 added 2026-04-15 post-Phase-3, user-authorized)
**Estimated duration:** 60–90 minutes for Phases 1–3 (actuals on track); Phase 4 adds ~30–45 minutes
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

- [x] **Phase 4: Conductor schema + docs refresh + mandate clarification** (added 2026-04-15 post-Phase-3, user-authorized) — DONE. `[conductors.<name>.claude]` config block (CFG-08, closes issue #602), eight-test regression suite (CFG-11), README + agent-deck SKILL.md docs refresh (CFG-09), and repo-root `CLAUDE.md` `--no-verify` ban + scope clarification (CFG-10). Plans 04-01 + 04-02 complete. Hard-rule audit: 0 @alec-pinson, 0 Claude attribution, 7/7 signed, 8 #602 refs. Tests: 8/8 TestConductorConfig_ + 8/8 TestPerGroupConfig_ GREEN. [REQ mapping: CFG-08, CFG-09, CFG-10, CFG-11]

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
- [x] 02-02-PLAN.md — CFG-04 test 5 + CFG-07: conductor-restart regression test + source-label helper (`GetClaudeConfigDirSourceForGroup`) in claude.go + private `logClaudeConfigResolution` helper in instance.go emitted from THREE sites (Start, StartWithMessage, Restart). Two CFG-07 unit tests (`ClaudeConfigDirSourceLabel` priority-chain, `ClaudeConfigResolutionLogFormat` slog text-handler format lock).

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

**Plans:** 2 plans

Plans:
- [x] 03-01-PLAN.md — CFG-05 visual harness: ship `scripts/verify-per-group-claude-config.sh` with preflight + config isolation + trap cleanup + TTY-aware pass/fail table; run on conductor host; commit with @alec-pinson attribution trailer.
- [x] 03-02-PLAN.md — CFG-06 docs: README `### Per-group Claude config` subsection under §Features, repo-root CLAUDE.md resurrection (Path A from `5013940^`) + TestPerGroupConfig_* one-liner, CHANGELOG `[Unreleased] > Added` bullet with PR #578 reference, phase-wide hard-rule audit (attribution ≥ 2, no Claude attribution, all commits signed).

---

### Phase 4: Conductor schema + docs refresh + mandate clarification

**Goal:** Add a top-level `[conductors.<name>]` config block (with `config_dir` + `env_file`) and a loader seam in `GetClaudeConfigDirForGroup` so conductor-tagged sessions (`groupPath == "conductor/<name>"`) inherit Claude config from the conductor block. Refresh README and the agent-deck skill SKILL.md to document the new schema. Clarify the repo-root `CLAUDE.md` `--no-verify` mandate so metadata-only commits don't pay hook latency for zero verification value.

**Requirements covered:**
- CFG-08 — `[conductors.<name>]` schema + loader, precedence env > conductor > group > profile > global > default, propagation to conductor-group sessions (closes issue [#602](https://github.com/asheshgoplani/agent-deck/issues/602))
- CFG-09 — README "Per-group Claude config" subsection extension + agent-deck SKILL.md update at canonical plugin-cache path AND pool path (if present)
- CFG-10 — repo-root `CLAUDE.md` `--no-verify` mandate scope clarification (metadata-only commits exemption with negative example)
- CFG-11 — eight regression tests in new file `internal/session/conductorconfig_test.go`

**Approach (TDD, in order):**
1. Create `internal/session/conductorconfig_test.go` with the eight tests from CFG-11 — RED first. Tests 1 (`SchemaParses`) and 4 (`FallsThroughToGroupOverride`) probably fail at compile because `UserConfig.Conductors` doesn't exist yet; tests 2/3/5/6/7/8 fail at assertion because the loader doesn't consult the conductor block.
2. Run `go test ./internal/session/... -run TestConductorConfig_ -race -count=1` — confirm RED across the suite (some compile errors, some assertion failures).
3. **Schema first.** In `internal/session/userconfig.go`, add `ConductorClaudeConfig` struct (mirroring `GroupClaudeConfig`), top-level `Conductors map[string]ConductorClaudeConfig` on `UserConfig`, and `GetConductorClaudeConfigDir(name) string` / `GetConductorClaudeEnvFile(name) string` helpers with the same `~`/env-var expansion as the group helpers. Re-run tests — `SchemaParses` and `FallsThroughToGroupOverride` go GREEN; the loader-dependent tests still RED.
4. **Loader seam.** In `internal/session/claude.go`, extend `GetClaudeConfigDirForGroup(groupPath)` to detect the `conductor/<name>` prefix (use the canonical prefix from `cmd/agent-deck/conductor_cmd.go` — confirm via Read before editing), look up `cfg.Conductors[name]`, and slot the conductor-block check between the env-var check and the existing group check. Mirror the change in `GetClaudeConfigDirSourceForGroup` so the source label `conductor` is returned for the new branch.
5. **env_file wiring.** Confirm via Read that `buildEnvSourceCommand()` (touched in Phase 2 plan 02-01) reads `env_file` via the same group-aware helper chain. If not, add a `GetEnvFileForGroup(groupPath)` shim that walks conductor → group → (no profile/global env_file today) and have `buildEnvSourceCommand` call it. Test 7 (`EnvFileSourced`) gates this.
6. Re-run the full `TestConductorConfig_*` suite — all 8 GREEN under `-race -count=1`.
7. Re-run `TestPerGroupConfig_*` (Phases 1–2) — confirm no regression. The conductor branch is upstream of the group branch in the precedence chain, so existing group-only configs MUST still resolve to their group value.
8. **Docs.** Update README `### Per-group Claude config` subsection (added in Phase 3 plan 03-02) with a sibling `[conductors.<name>]` example and a one-line precedence note linking to issue #602. Update agent-deck SKILL.md at canonical plugin-cache path (`~/.claude/plugins/cache/agent-deck/agent-deck/<hash>/skills/agent-deck/SKILL.md` — find the actual hash via `ls`) AND the pool copy at `~/.agent-deck/skills/pool/agent-deck/SKILL.md` if present.
9. **Mandate clarification.** Update repo-root `CLAUDE.md` "General rules" block: clarify that the `--no-verify` ban applies to source-modifying commits only, define metadata-only paths (`.planning/**`, `docs/**`, `*.md` outside source dirs, `CHANGELOG.md` during milestone-prep), include the negative example (mixed metadata+source = source-modifying = hooks required).
10. **Atomic commits.** TDD test commit (RED), schema commit, loader commit, docs commit, CLAUDE.md mandate commit — each signed "Committed by Ashesh Goplani", no Claude attribution, no @alec-pinson attribution. Body of one commit references issue #602.

**Scope (files touched):**
- `internal/session/conductorconfig_test.go` (NEW — kept separate from `pergroupconfig_test.go` for clean revert)
- `internal/session/userconfig.go` (additive — new struct, new field, new helpers)
- `internal/session/claude.go` (additive — new branch in `GetClaudeConfigDirForGroup` + `GetClaudeConfigDirSourceForGroup`)
- `internal/session/env.go` (only if step 5 finds a gap)
- `README.md` (extend Phase 3's "Per-group Claude config" subsection)
- `~/.claude/plugins/cache/agent-deck/agent-deck/<hash>/skills/agent-deck/SKILL.md` (canonical) + `~/.agent-deck/skills/pool/agent-deck/SKILL.md` (pool, if present) — outside repo, but in the milestone scope per spec amendment
- Repo-root `CLAUDE.md` (mandate clarification)
- `docs/PER-GROUP-CLAUDE-CONFIG-SPEC.md` (already amended — this commit)
- `.planning/ROADMAP.md`, `.planning/STATE.md`, `.planning/REQUIREMENTS.md` (already amended — this commit)

**Success criteria:**
1. `go test ./internal/session/... -run TestConductorConfig_ -race -count=1` — all 8 GREEN.
2. `go test ./internal/session/... -run TestPerGroupConfig_ -race -count=1` — all 8 still GREEN (no regression on Phases 1–2 test suite).
3. README documents `[conductors.<name>]` schema with example and precedence note, cross-linked to issue #602.
4. Both agent-deck SKILL.md surfaces (canonical + pool, if present) document the new block.
5. Repo-root `CLAUDE.md` carries the `--no-verify` mandate clarification with metadata-paths list and negative example.
6. All Phase 4 commits sign "Committed by Ashesh Goplani"; no Claude attribution; no @alec-pinson attribution. Issue #602 referenced in commit body.
7. No `git push`, `git tag`, `gh release`, `gh pr create`, `gh pr merge` executed.

**Dependencies:** Phases 1–3 complete. Phase 4 reads the existing `GetClaudeConfigDirForGroup` (Phase 1), `GetClaudeConfigDirSourceForGroup` (Phase 2 plan 02-02), `buildEnvSourceCommand` (Phase 2 plan 02-01), and the README `### Per-group Claude config` subsection (Phase 3 plan 03-02) and extends them additively.

**Plans:** TBD by `/gsd-plan-phase 4` — likely 2 plans (test+schema+loader as plan 04-01; docs+mandate as plan 04-02), but the planner agent decides.

---

## Milestone Verification (runs at `/gsd-complete-milestone`)

Recap of the spec success criteria — the audit step will confirm all of them:

1. PR #578 unit tests remain GREEN.
2. `go test ./internal/session/... -run TestPerGroupConfig_ -race -count=1` — all 8 GREEN (six ROADMAP-numbered tests 1/2/3/4/5/6 + two CFG-07 helper tests).
3. `bash scripts/verify-per-group-claude-config.sh` exits 0 on conductor host.
4. Manual conductor proof: `ps -p <pane_pid>` env shows the overridden `CLAUDE_CONFIG_DIR` after restart.
5. Commit log includes README + CHANGELOG + CLAUDE.md commits and at least one `@alec-pinson` attribution commit (Phases 1–3 only; Phase 4 commits MUST NOT carry @alec-pinson attribution).
6. No push / tag / PR / merge performed.

Phase 4 additions (gated alongside #1–#6):

7. `go test ./internal/session/... -run TestConductorConfig_ -race -count=1` — all 8 GREEN.
8. README + agent-deck SKILL.md (canonical + pool) document `[conductors.<name>]`. Repo-root `CLAUDE.md` carries the `--no-verify` mandate clarification.
9. Phase 4 commits sign "Committed by Ashesh Goplani"; no Claude attribution; no @alec-pinson attribution; commit body references issue #602.

---

## Carry-forward notes

- **v1.5.3 mandate (repo-root `CLAUDE.md`):** No `--no-verify`. Every commit goes through pre-commit hooks.
- **Commit signature:** "Committed by Ashesh Goplani". No Claude attribution.
- **Scope discipline:** Any change outside the spec's scope list is escalation-worthy, not drift-worthy.
- **Rebase posture:** `fa9971e` is behind current `main`. Rebase is a merge-time concern — NOT this milestone's scope.

---

*Roadmap created: 2026-04-15*
*Last updated: 2026-04-16 — Phase 4 complete (plans 04-01 + 04-02); CFG-08/09/10/11 all closed; milestone ready for /gsd-complete-milestone.*
