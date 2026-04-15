---
phase: 04-conductor-schema-docs-refresh-mandate-clarification
plan: 02
subsystem: docs
tags: [docs, skill-md, claude-md, readme, mandate, no-verify, cfg-09, cfg-10]

requires:
  - phase: 04-conductor-schema-docs-refresh-mandate-clarification
    plan: 01
    provides: nested [conductors.<name>.claude] TOML schema + Instance-aware loader + 8 CFG-11 tests

provides:
  - "README.md `#### Per-conductor Claude config (v1.5.4)` subsection (sibling of CFG-06's `### Per-group Claude config`)"
  - "Canonical agent-deck SKILL.md at plugin-cache path documents [conductors.<name>.claude] schema + precedence + canonical-vs-pool distinction"
  - "SKILL_MD_DIFF.md repo-visible audit artifact (canonical diff + pool skip string)"
  - "Repo-root CLAUDE.md `**No `--no-verify`**` bullet in ## General rules (NEW — base ban did not exist before)"
  - "Repo-root CLAUDE.md `### --no-verify scope clarification (v1.5.4+)` sub-section with metadata-paths list + positive + negative examples + rationale"

affects:
  - Future metadata-only commits across the repo (CFG-10 exemption now documented)
  - Future Claude Code sessions that load the agent-deck skill (SKILL.md discoverability for [conductors.<name>.claude])

tech-stack:
  added: []
  patterns:
    - "Repo-visible audit artifact pattern for external-file edits (SKILL_MD_DIFF.md records diffs applied to files outside the repo)"
    - "Rule-then-scope layout in CLAUDE.md: base bullet + sub-section clarification"

key-files:
  created:
    - .planning/phases/04-conductor-schema-docs-refresh-mandate-clarification/SKILL_MD_DIFF.md
    - .planning/phases/04-conductor-schema-docs-refresh-mandate-clarification/04-02-SUMMARY.md
  modified:
    - README.md
    - CLAUDE.md
    - "~/.claude/plugins/cache/agent-deck/agent-deck/12c0a65dfb13/skills/agent-deck/SKILL.md (external)"
    - .planning/STATE.md
    - .planning/REQUIREMENTS.md

key-decisions:
  - "Nested [conductors.<name>.claude] TOML shape used in ALL docs — matches 04-01's Go struct path (ConductorOverrides.Claude.ConfigDir) and mirrors [groups.\"<g>\".claude]. The spec's flat-form example is explicitly NOT the shipped form."
  - "Pool SKILL.md at ~/.agent-deck/skills/pool/agent-deck/SKILL.md is ABSENT on this execution host. Recorded with the literal skip string per plan template — this is not a gap per REQ CFG-09's 'pool path if present' language."
  - "Final metadata commit uses plain `git commit` (hooks enabled) — CFG-10 clarification permits --no-verify on `.planning/**`-only staging, but this plan defaults to hooks-enabled to avoid normalizing the exemption on the very commit that establishes the rule."
  - "Repo-root CLAUDE.md is classified as source-modifying (per its own CFG-10 clarification) — the commit that shipped the clarification ran with hooks enabled."

requirements-completed: [CFG-09, CFG-10]

duration: 6min
completed: 2026-04-16
---

# Phase 04 Plan 02: Docs Refresh + --no-verify Mandate Summary

**README.md extended with `[conductors.<name>.claude]` subsection, canonical SKILL.md updated with the same paragraph, repo-visible `SKILL_MD_DIFF.md` audit artifact shipped, and repo-root CLAUDE.md gains both a NEW `--no-verify` ban bullet and a scope-clarification sub-section with metadata-paths list + positive/negative examples — all four Phase 4 hard-rule audit gates pass with zero violations across seven Phase 4 commits.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-15T22:09:00Z
- **Completed:** 2026-04-15T22:15:00Z
- **Tasks:** 4 (README → SKILL audit artifact → CLAUDE.md mandate → audit + metadata close-out)
- **Files modified:** 2 (README.md, CLAUDE.md) + 1 external SKILL.md + 1 new repo artifact (SKILL_MD_DIFF.md)

## Accomplishments

- **CFG-09 closed:** README.md's `### Per-group Claude config` subsection now has a sibling `#### Per-conductor Claude config (v1.5.4)` block documenting the nested `[conductors.<name>.claude]` schema, the six-step precedence chain (env > conductor > group > profile > global > default), backward compat with PR #578, and a cross-link to issue #602. Canonical SKILL.md at `~/.claude/plugins/cache/agent-deck/agent-deck/12c0a65dfb13/skills/agent-deck/SKILL.md` received the same paragraph plus a canonical-vs-pool note. Pool SKILL.md is absent on this host; documented in the audit artifact.
- **CFG-10 closed:** Repo-root `CLAUDE.md` `## General rules` gains a NEW `**No `--no-verify`**` bullet (`grep -n "no-verify" CLAUDE.md` returned zero matches pre-edit; verified). A new `### --no-verify scope clarification (v1.5.4+)` sub-section lists metadata-only paths exempt from the ban, source-modifying paths where it still applies, and ships both a negative example (`.planning/STATE.md` + `internal/session/instance.go`) and a positive example (`.planning/**` only).
- **Audit artifact shipped:** `.planning/phases/04-*/SKILL_MD_DIFF.md` is the repo-visible record of external-file edits (SKILL.md lives outside the repo working tree). Includes the exact unified diff applied to the canonical SKILL.md and the canonical skip string for the absent pool SKILL.md.
- **Hard-rule audit passed (all four gates):**
  - `@alec-pinson` count in Phase 4 commits: **0** (MUST be 0) ✓
  - `Co-Authored-By: Claude` / `Generated with Claude` count: **0** (MUST be 0) ✓
  - Signed / total: **7 / 7** ✓
  - `#602` / `issues/602` references: **8** (MUST be ≥ 1) ✓
- **Test regression gate preserved:** `go test ./internal/session/... -run "TestConductorConfig_|TestPerGroupConfig_" -race -count=1` → 16/16 PASS (8 TestConductorConfig_ + 8 TestPerGroupConfig_ including the `ClaudeConfigDirSourceLabel` subtests). Zero regressions from CFG-09/CFG-10 docs changes (as expected — no Go source touched).

## Task Commits

1. **Task 1: README.md extension — `c230c77` (docs)**
   - Extended `### Per-group Claude config` with sibling `#### Per-conductor Claude config (v1.5.4)` block.
   - 27 insertions, 0 deletions.
   - Hooks ran (lefthook fmt-check + vet, 0.76s). Commit signed "Committed by Ashesh Goplani".

2. **Task 2: SKILL_MD_DIFF audit artifact — `0ac0efe` (docs)**
   - Canonical SKILL.md at `~/.claude/plugins/cache/agent-deck/agent-deck/12c0a65dfb13/skills/agent-deck/SKILL.md` edited directly (external file; no repo commit for it).
   - Pool SKILL.md absent; skip string recorded.
   - `.planning/phases/04-*/SKILL_MD_DIFF.md` (new file, 80 lines) committed with `-f` (local exclude blocks `.planning/` by default; consistent with 04-01 precedent).
   - Hooks ran (0.78s). Commit signed "Committed by Ashesh Goplani".

3. **Task 3: CLAUDE.md `--no-verify` ban + scope clarification — `95f382d` (docs)**
   - Added bullet to `## General rules` and a new `### --no-verify scope clarification (v1.5.4+)` sub-section immediately after.
   - 24 insertions, 0 deletions.
   - Hooks ran (0.78s). Commit signed "Committed by Ashesh Goplani".
   - Note: CLAUDE.md is classified as source-modifying by its own new clarification — running hooks on this commit is consistent with the rule it ships.

4. **Task 4: Metadata close-out — PENDING this commit**
   - `.planning/phases/04-*/04-02-SUMMARY.md` (this file)
   - `.planning/STATE.md` — Phase 04 commits table extended; Current Position flipped to Complete
   - `.planning/REQUIREMENTS.md` — CFG-09 and CFG-10 checkboxes flipped to `[x]`; Traceability table updated
   - `.planning/ROADMAP.md` — plan-progress sync via gsd-tools
   - Plain `git commit` (hooks enabled) per plan directive — hooks no-op on metadata-only staging.

## Files Created / Modified

**Repo files:**

- `README.md` — modified; added 27-line `#### Per-conductor Claude config (v1.5.4)` block with TOML example, 6-step precedence chain, backward-compat note, issue #602 link.
- `CLAUDE.md` — modified; added 1-bullet `No `--no-verify`` + 22-line `### --no-verify scope clarification (v1.5.4+)` sub-section.
- `.planning/phases/04-conductor-schema-docs-refresh-mandate-clarification/SKILL_MD_DIFF.md` — created; 80 lines; repo-visible audit trail for external SKILL.md edits.
- `.planning/phases/04-conductor-schema-docs-refresh-mandate-clarification/04-02-SUMMARY.md` — created (this file).
- `.planning/STATE.md` — updated Phase 04 commits table + Current Position.
- `.planning/REQUIREMENTS.md` — CFG-09 + CFG-10 checkboxes flipped; Traceability rows updated.

**External files (outside repo, tracked via SKILL_MD_DIFF.md):**

- `~/.claude/plugins/cache/agent-deck/agent-deck/12c0a65dfb13/skills/agent-deck/SKILL.md` — appended 17 lines under the Configuration section.
- `~/.agent-deck/skills/pool/agent-deck/SKILL.md` — ABSENT on this host; skip string recorded.

## Decisions Made

See frontmatter `key-decisions`. The three highest-impact:

1. **Nested TOML shape locked into all docs.** README, SKILL.md, and all examples use `[conductors.<name>.claude]` (sub-table). The flat-form `[conductors.<name>]` mentioned in the spec (REQUIREMENTS.md CFG-08 line 61) is explicitly NOT the shipped form — Plan 04-01 resolved this by mirroring `[groups."<g>".claude]` for internal consistency; Plan 04-02 carries that forward.
2. **Pool SKILL.md absent — documented, not patched.** REQ CFG-09 uses "pool path if present" language; absence is acceptable. The audit artifact (SKILL_MD_DIFF.md) records the absence with the canonical skip string so that future executors don't re-chase it.
3. **Final metadata commit defaults to hooks-enabled.** The CFG-10 clarification shipped in Task 3 PERMITS `--no-verify` on `.planning/**`-only staging, but this plan deliberately uses plain `git commit` on the metadata close-out to avoid normalizing the exemption on the very commit that establishes the rule. Hooks no-op on `.planning/**` anyway — no latency cost.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] `.planning/` path blocked by local exclude; used `git add -f`**
- **Found during:** Task 2 (SKILL_MD_DIFF.md commit).
- **Issue:** `git add .planning/phases/04-*/SKILL_MD_DIFF.md` failed with "The following paths are ignored by one of your .gitignore files". Investigation showed `.git/info/exclude` line 58 contains `.planning/` — this is a worktree-level local exclude.
- **Fix:** Used `git add -f` to force-add (the same mechanism used by commit `917c111` in Plan 04-01 which successfully committed `.planning/` files). Preserves the local exclude default while allowing phase-scoped metadata artifacts through.
- **Files modified:** none (fix was command-level).
- **Commit:** `0ac0efe`.
- **Verification:** `git log --oneline --all -- .planning/phases/04-*/SKILL_MD_DIFF.md` → `0ac0efe`.

---

**Total deviations:** 1 auto-fixed (Rule 3 — blocking). No Rule 4 architectural stops.

**Impact on plan:** Mechanical (tooling), zero semantic divergence.

## Acceptance Criteria Verification

| Plan criterion | Status |
| --- | --- |
| `grep -c "Per-conductor Claude config" README.md` ≥ 1 | PASS (1 match) |
| `grep -q "\[conductors\." README.md` | PASS |
| `grep -q "conductors.gsd-v154.claude" README.md` (nested form) | PASS |
| `grep -q "issues/602" README.md` | PASS |
| `grep -q "Precedence chain" README.md` | PASS |
| `grep -q "Backward compat" README.md` | PASS |
| README Per-group subsection preserved (not clobbered) | PASS (line 98 `### Per-group Claude config` intact) |
| SKILL_MD_DIFF.md exists, non-empty, BOTH sections populated | PASS |
| Canonical SKILL.md contains `[conductors.<name>.claude]` + `Per-conductor` + `issues/602` | PASS |
| Pool SKILL.md status recorded as absent (per plan template skip string) | PASS |
| `grep -q "No \`--no-verify\`" CLAUDE.md` | PASS |
| `grep -c "no-verify" CLAUDE.md` ≥ 2 | PASS (4 matches) |
| `grep -q "no-verify scope clarification" CLAUDE.md` | PASS |
| `grep -q "Metadata-only paths" CLAUDE.md` | PASS |
| `grep -q "Negative example" CLAUDE.md` + `grep -q "Positive example" CLAUDE.md` | PASS |
| New `--no-verify` bullet inside `## General rules` section | PASS (line 57, within General rules block) |
| Audit 1: `@alec-pinson` in Phase 4 bodies = 0 | PASS (0) |
| Audit 2: Claude attribution in Phase 4 bodies = 0 | PASS (0) |
| Audit 3: all Phase 4 commits signed | PASS (7/7) |
| Audit 4: ≥ 1 Phase 4 commit references #602 | PASS (8 refs) |
| Audit 5: `go test ...TestConductorConfig_|TestPerGroupConfig_...` → all PASS | PASS (16/16) |
| All Phase 4 plan-02 commits signed "Committed by Ashesh Goplani" | PASS |
| No `@alec-pinson` in Phase 4 plan-02 commits | PASS |
| No `Co-Authored-By: Claude` in Phase 4 plan-02 commits | PASS |

## SKILL_MD_DIFF.md Audit Artifact Reference

Path: `.planning/phases/04-conductor-schema-docs-refresh-mandate-clarification/SKILL_MD_DIFF.md`

Captures:
- **Canonical section:** Applied — full unified diff of the 17-line append under the Configuration section of `~/.claude/plugins/cache/agent-deck/agent-deck/12c0a65dfb13/skills/agent-deck/SKILL.md`. Post-edit verification via `grep -n` shows all three required substrings (`[conductors.<name>.claude]`, `Per-conductor Claude config`, `issues/602`).
- **Pool section:** Skipped — pool directory `~/.agent-deck/skills/pool/agent-deck/` absent on this execution host. Canonical skip string recorded (`no pool skill found — host lacks pool directory; SKILL.md update skipped`) per plan template.

## Phase 4 Milestone Close-out Status

With Plans 04-01 and 04-02 complete, all four Phase 4 requirements are closed:

| Requirement | Phase | Status | Plan |
| --- | --- | --- | --- |
| CFG-08 | Phase 4 | Complete | 04-01 |
| CFG-09 | Phase 4 | Complete | 04-02 |
| CFG-10 | Phase 4 | Complete | 04-02 |
| CFG-11 | Phase 4 | Complete | 04-01 |

**Milestone success criteria 7, 8, 9, 10, 11, 12** (Phase 4 additions to the v1.5.4 milestone spec) are ready for verification at `/gsd-complete-milestone`:

- #7 `TestConductorConfig_` 8/8 GREEN — DONE (this run)
- #8 Manual conductor-host proof for issue #602 — awaits user action on the conductor host
- #9 README includes `[conductors.<name>]` example + precedence note — DONE (plan 04-02)
- #10 SKILL.md (canonical + pool if present) documents the new block — DONE (canonical applied; pool absent and documented)
- #11 Repo-root CLAUDE.md carries `--no-verify` clarification — DONE (plan 04-02)
- #12 Phase 4 commits signed, no Claude/@alec-pinson attribution, #602 referenced — DONE (audit gate)

## Self-Check: PASSED

- `README.md` contains `Per-conductor Claude config` at line 114. ✓
- `CLAUDE.md` contains `No \`--no-verify\`` at line 57 and `--no-verify scope clarification` at line 61. ✓
- `.planning/phases/04-*/SKILL_MD_DIFF.md` exists (80 lines), contains both `Canonical plugin-cache SKILL.md` and `Pool SKILL.md` headings. ✓
- Canonical SKILL.md on-disk at `~/.claude/plugins/cache/agent-deck/agent-deck/12c0a65dfb13/skills/agent-deck/SKILL.md` contains all three required substrings. ✓
- Commit hashes reachable from HEAD: `c230c77` (README), `0ac0efe` (SKILL audit), `95f382d` (CLAUDE.md). ✓
- Hard-rule audit: 0 @alec-pinson, 0 Claude attribution, 7/7 signed, 8 #602 refs. ✓
- Test sweep: 16/16 PASS (8 TestConductorConfig_ + 8 TestPerGroupConfig_ variants). ✓

---

*Phase: 04-conductor-schema-docs-refresh-mandate-clarification*
*Completed: 2026-04-16*
