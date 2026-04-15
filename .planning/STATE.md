---
gsd_state_version: 1.0
milestone: v1.5.4
milestone_name: milestone
status: complete
last_updated: "2026-04-16T00:16:00.000Z"
last_activity: 2026-04-16
progress:
  total_phases: 4
  completed_phases: 4
  total_plans: 7
  completed_plans: 7
  percent: 100
---

# Project State — v1.5.4

## Project Reference

**Project:** Agent Deck
**Repository:** /home/ashesh-goplani/agent-deck
**Worktree:** `/home/ashesh-goplani/agent-deck/.worktrees/per-group-claude-config`
**Branch:** `fix/per-group-claude-config-v154`
**Starting point:** v1.5.3 (`ee7f29e` on `fix/feedback-closeout`)
**Base:** `fa9971e` (upstream PR #578 by @alec-pinson)
**Target version:** v1.5.4

See `.planning/PROJECT.md` for full project context.
See `.planning/ROADMAP.md` for the v1.5.4 phase plan.
See `.planning/REQUIREMENTS.md` for CFG-01..07 and phase mapping.
See `docs/PER-GROUP-CLAUDE-CONFIG-SPEC.md` for the source spec.

## Milestone: v1.5.4 — Per-group Claude Config

**Goal:** Accept PR #578's config schema + lookup as base, close adoption gaps for the user's conductor use case (custom-command injection, env_file sourcing), ship 6 regression tests + a visual harness + docs, with attribution to @alec-pinson.

**Estimated duration:** 60–90 minutes across 3 phases.

## Current Position

Phase: 04 (conductor-schema-docs-refresh-mandate-clarification) — COMPLETE
Plan: Complete (04-01 + 04-02)
Status: Phase 04 complete; milestone ready for /gsd-complete-milestone
Last activity: 2026-04-16 -- Plan 04-02 complete: CFG-09 + CFG-10 closed, canonical SKILL.md updated, pool SKILL.md absence recorded in SKILL_MD_DIFF.md audit artifact, CLAUDE.md --no-verify bullet + scope clarification shipped, all four Phase 4 hard-rule audit gates pass
Stopped at: Completed 04-02-PLAN.md

## Phase Progress

| # | Phase | Status | Requirements | Plans |
|---|-------|--------|--------------|-------|
| 1 | Custom-command injection + core regression tests | Complete | CFG-01, CFG-02, CFG-04 (tests 1, 2, 3, 6) | 1/1 (01-01) |
| 2 | env_file source semantics + observability + conductor E2E | Plans complete (verification pending) | CFG-03, CFG-04 (tests 4, 5), CFG-07 | 2/2 (02-01 + 02-02 complete) |
| 3 | Visual harness + documentation + attribution commit | Complete | CFG-05, CFG-06 | 2/2 (03-01 + 03-02) |
| 4 | Conductor schema + docs refresh + mandate clarification | Complete | CFG-08, CFG-09, CFG-10, CFG-11 | 2/2 (04-01 + 04-02 complete) |

## Phase 01 commits (since base 3e402e2)

| Hash | Type | Subject |
|------|------|---------|
| 4730aa5 | docs | docs(planning): plan phase 01 — custom-command injection + core regression tests |
| 40f4f04 | test | test(session): add per-group Claude config regression tests (CFG-04 tests 1/2/3/6) |
| b39bbf3 | fix | fix(session): export CLAUDE_CONFIG_DIR for custom-command sessions (CFG-02) |

## Phase 02 commits

| Hash | Type | Subject |
|------|------|---------|
| 6830838 | docs | docs(02): scaffold phase 2 context from spec (CFG-03, CFG-04 tests 4/5, CFG-07) |
| 6a0205d | docs | docs(planning): plan phase 02 — env_file source + observability + conductor E2E |
| 38a2af3 | test | test(session): add env_file spawn-source regression test (CFG-04 test 4) |
| e608480 | fix  | fix(session): source group env_file on custom-command spawn path (CFG-03) |
| 5d8737f | docs | docs(02-01): complete phase 02 plan 01 — CFG-03 closed, CFG-04 test 4 locked |
| e000801 | test | test(session): add conductor-restart + CFG-07 source-label + log-format regression tests |
| 476367c | feat | feat(session): add CFG-07 claude-config-resolution log line + source-label helper |

## Phase 04 commits

| Hash | Type | Subject |
|------|------|---------|
| 13265f4 | docs | docs(planning): plan phase 04 — conductor schema + docs + mandate clarification |
| 41c9b8e | test | test(04): add conductor config regression tests (RED, CFG-11) |
| f0cf791 | feat | feat(04): add [conductors.<name>.claude] schema (CFG-08 partial) |
| 6fdac26 | feat | feat(04): wire conductor-block loader + four callsites (CFG-08) |
| 917c111 | docs | docs(04-01): complete conductor schema + loader + tests plan |
| c230c77 | docs | docs(04): document [conductors.<name>.claude] schema in README (CFG-09) |
| 0ac0efe | docs | docs(04): record SKILL.md external-file updates (CFG-09 audit artifact) |
| 95f382d | docs | docs(04): add --no-verify ban + scope clarification to CLAUDE.md (CFG-10) |

## Decisions — Plan 04-01

- Nested `[conductors.<name>.claude]` TOML shape chosen (NOT flat `[conductors.<name>]`) — mirrors `[groups."<g>".claude]` pattern for internal consistency. Plan 04-02's README/CHANGELOG/SKILL docs MUST use this nested form.
- Per-conductor struct renamed `ConductorSettings` → `ConductorOverrides` (Rule 3 blocking deviation). `ConductorSettings` already exists in `internal/session/conductor.go:49` for the global `[conductor]` bot orchestration block (heartbeat, telegram, slack, discord) — Go cannot declare the same type name twice in a package.
- Fourth callsite swap at `instance.go:4172` (`buildClaudeResumeCommand`) is the critical one. Without it, `Restart()` of a conductor session would silently fall through to group-only resolution on the resume path, breaking milestone success criterion #8 (restart `gsd-v154` conductor → reports `~/.claude-work`). CFG-11 test 6 sub-assertion 6c exercises this path explicitly.
- RED-gate stubs for `GetConductorClaudeConfigDir` / `GetConductorClaudeEnvFile` in userconfig.go allowed CFG-11 test 1 (SchemaParses) to FAIL at the assertion (not compile) in Task 1. Task 2 replaced the stubs with real bodies.

## Decisions — Plan 02-02

- `GetClaudeConfigDirSourceForGroup(groupPath)` in `internal/session/claude.go` mirrors the priority chain at `claude.go:246` and returns both path and source label (env|group|profile|global|default). Keeps a single source-of-truth for observability labels.
- `(i *Instance) logClaudeConfigResolution()` owns the single CFG-07 slog message literal. Called from exactly 3 sites: `Start()`, `StartWithMessage()`, `Restart()` — each gated on `IsClaudeCompatible(i.Tool)`. Fork path intentionally silent (Fork can trigger a subsequent Start() which logs).
- Rule 1 deviation captured in SUMMARY: plan's LogFormat test assumed `NewInstanceWithGroupAndTool`'s first arg populated `i.ID`, but it populates `i.Title`. The CFG-07 helper correctly logs `i.ID` (session logs key on ID, not Title). Fixed with a one-line `inst.ID = "logfmt-sess-123"` override in the test; helper is unchanged.
- Rule 3 deviation captured in SUMMARY: helper comment originally contained the string `"claude config resolution"` which inflated `grep -c` past the plan's expected 1. Reworded comment to `"the single CFG-07 slog message literal"` — no semantic change; grep now returns 1.

## Decisions — Plan 02-01

- Pre-authorized instance.go:598→599 one-line fix applied per plan directive despite first-run GREEN on assertion B. Diagnosis: buildClaudeCommand at L477-480 already prepends envPrefix unconditionally, so the CFG-03 guarantee was being delivered by the outer wrapper for production callers. The L599 hardening is defense-in-depth against any future callsite that invokes buildClaudeCommandWithMessage directly (bypassing buildClaudeCommand).
- Three new pre-existing StatusEventWatcher fsnotify-timeout failures confirmed unrelated to Phase 02 changes via `git stash`. Logged to Phase 02 deferred-items.md. Not a regression.
- Fix commit (e608480) carries `Base implementation by @alec-pinson in PR #578.` per milestone must_have #6.

## Hard rules in force (carried from CLAUDE.md + spec)

- No `git push`, `git tag`, `gh release`, `gh pr create`, `gh pr merge`.
- No `rm` — use `trash`.
- No `--no-verify` (v1.5.3 mandate at repo-root `CLAUDE.md`).
- No Claude attribution in commits. Sign: "Committed by Ashesh Goplani".
- TDD: test before fix; test must fail without the fix.
- Additive only vs PR #578 — do not revert or refactor its existing code.
- At least one commit must carry: "Base implementation by @alec-pinson in PR #578."

## Phase 04 context (added 2026-04-15, user-authorized post-Phase-3)

Phase 4 was added after Phases 1–3 completed. Three drivers:

1. **Issue [#602](https://github.com/asheshgoplani/agent-deck/issues/602)** — conductors are first-class entities but cannot carry their own Claude `config_dir`. Adding `[conductors.<name>]` schema + loader closes this. Precedence sits between env-var and group: `env > conductors.<name> > groups."<g>".claude > profiles.<p>.claude > [claude] > default`.
2. **Docs surface gap** — Phase 3's CFG-06 covered README + repo-root CLAUDE.md + CHANGELOG, but did NOT touch the agent-deck skill `SKILL.md` at the canonical plugin-cache path or the pool path. Other Claude sessions discover agent-deck features via that skill. Phase 4 closes the gap.
3. **Mandate UX issue** — the v1.5.3 `--no-verify` ban (repo-root CLAUDE.md) made no exception for metadata-only commits. `.planning/` and docs commits pay 10–30s of hook latency for zero verification value. Phase 4 clarifies the mandate: `--no-verify` is banned for source-modifying commits; metadata-only commits MAY use it when hooks would no-op.

**Phase 4 hard rules (in addition to milestone-wide rules below):**

- NO @alec-pinson attribution on Phase 4 commits — this is user-driven, not PR #578.
- Issue #602 reference acceptable in commit body (reporter is the milestone user).
- TDD: RED tests first (CFG-11's eight tests in new file `internal/session/conductorconfig_test.go`), then schema, then loader, then docs, then mandate edit.
- Additive only — no refactors of PR #578's code, no refactors of Phases 1–3 code unless a CFG-11 test requires it.

## Next action

User authorized Phase 4 on 2026-04-15. Spec/roadmap/state/requirements amendment commit lands first (this commit), then `/gsd-plan-phase 4` produces `.planning/phases/04-conductor-schema-docs-mandate/PLAN.md`. The Phase 4 planner should:

1. Read `.planning/PROJECT.md`, `.planning/ROADMAP.md` (Phase 4 section), `.planning/REQUIREMENTS.md` (CFG-08/09/10/11), `docs/PER-GROUP-CLAUDE-CONFIG-SPEC.md` (Phase 4 amendment block).
2. Inspect Phase 1–3 outputs in `internal/session/claude.go`, `userconfig.go`, `pergroupconfig_test.go` to find the right additive seams.
3. Confirm the canonical conductor-group prefix in `cmd/agent-deck/conductor_cmd.go` (whether it's `conductor/<name>` literal or composed differently) before writing the loader test assertions.
4. Produce `04-01-PLAN.md` (test+schema+loader) and `04-02-PLAN.md` (docs+mandate) — or fewer plans if the planner judges the work fits in one.
5. Honor the Phase 4 hard rules above plus all milestone-wide hard rules.

## Accumulated Context

Prior milestones on main (not relevant to this branch's scope but preserved for context): v1.5.0 premium web app polish, v1.5.1/1.5.2/1.5.3 patch work, v1.6.0 Watcher Framework in progress on main.

v1.6.0 phase directories (`.planning/phases/13-*`, `14-*`, `15-*`) are leakage from main's `.planning/` into this worktree. They are left untouched. This milestone's phase dirs will be `01-*`, `02-*`, `03-*`.
