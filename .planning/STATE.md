---
gsd_state_version: 1.0
milestone: v1.5.4
milestone_name: milestone
status: executing
last_updated: "2026-04-15T13:38:00.000Z"
last_activity: 2026-04-15 -- Phase 02 plan 01 (CFG-03 + CFG-04 test 4) COMPLETE
progress:
  total_phases: 3
  completed_phases: 1
  total_plans: 4
  completed_plans: 2
  percent: 50
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

Phase: 02 (env-file-source-semantics-observability-conductor-e2e) — EXECUTING
Plan: 1 of 2 — **COMPLETE**; next: 02-02 (CFG-07 observability + conductor E2E + CFG-04 test 5)
Status: Executing Phase 02
Last activity: 2026-04-15 -- Plan 02-01 complete (CFG-03 locked + CFG-04 test 4 GREEN)

## Phase Progress

| # | Phase | Status | Requirements | Plans |
|---|-------|--------|--------------|-------|
| 1 | Custom-command injection + core regression tests | Complete | CFG-01, CFG-02, CFG-04 (tests 1, 2, 3, 6) | 1/1 (01-01) |
| 2 | env_file source semantics + observability + conductor E2E | In progress | CFG-03, CFG-04 (tests 4, 5), CFG-07 | 1/2 (02-01 complete) |
| 3 | Visual harness + documentation + attribution commit | Pending | CFG-05, CFG-06 | — |

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

## Next action (from conductor)

The user instructed: **stop after bootstrapping the roadmap. Do NOT auto-plan.** The conductor will spawn `gsd-v154-plan-1` to plan Phase 1.

When that happens, the phase-1 planner should:

1. Read `.planning/PROJECT.md`, `.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md`, `docs/PER-GROUP-CLAUDE-CONFIG-SPEC.md`.
2. Run `/gsd-plan-phase 1` to produce `.planning/phases/01-custom-command-injection/PLAN.md`.
3. Honor the scope list in REQUIREMENTS.md — any touch outside is escalation.

## Accumulated Context

Prior milestones on main (not relevant to this branch's scope but preserved for context): v1.5.0 premium web app polish, v1.5.1/1.5.2/1.5.3 patch work, v1.6.0 Watcher Framework in progress on main.

v1.6.0 phase directories (`.planning/phases/13-*`, `14-*`, `15-*`) are leakage from main's `.planning/` into this worktree. They are left untouched. This milestone's phase dirs will be `01-*`, `02-*`, `03-*`.
