---
phase: 04-verification-harness-docs-and-ci-wiring
plan: 02
subsystem: docs
tags: [docs, mandate, changelog, audit]
requires:
  - PROJECT.md
  - docs/SESSION-PERSISTENCE-SPEC.md
  - CLAUDE.md (pre-existing mandate section at commit a262c6d)
provides:
  - CHANGELOG.md one-line v1.5.2 session-persistence mention under [Unreleased] > ### Fixed
  - CLAUDE.md mandate audit attestation (DOC-01..04 + SCRIPT-07 thresholds all met on entry; no patch required)
affects:
  - CHANGELOG.md
tech-stack:
  added: []
  patterns:
    - "Strict additive-only docs audit: grep thresholds drive decision to patch or no-op"
    - "Release tagging deferred — hotfix mention lives under [Unreleased] until formal release cut"
key-files:
  created:
    - .planning/phases/04-verification-harness-docs-and-ci-wiring/04-02-SUMMARY.md
  modified:
    - CHANGELOG.md
decisions:
  - "CLAUDE.md mandate audit passed on entry: all DOC-01..04 grep thresholds and SCRIPT-07 inline reference already satisfied at HEAD. No patch applied (per plan decision rule: audit PASS means no commit)."
  - "No [1.5.2] heading created in CHANGELOG.md — release tagging is deferred per project hard rule (no push/tag/PR)."
metrics:
  duration_minutes: 4
  tasks_completed: 2
  completed_date: 2026-04-14
requirements:
  - DOC-01
  - DOC-02
  - DOC-03
  - DOC-04
  - DOC-05
---

# Phase 4 Plan 02: CLAUDE.md mandate audit + DOC-05 CHANGELOG line — Summary

Audited the existing CLAUDE.md "Session persistence: mandatory test coverage" section against DOC-01..04 + SCRIPT-07 and confirmed all thresholds met on entry (no patch needed). Added the one-line v1.5.2 session-persistence bullet to CHANGELOG.md under `[Unreleased] > ### Fixed` (DOC-05).

## Baseline audit (CLAUDE.md on entry, HEAD=e045463)

Run from repo root:

| Check | Threshold | Count | Status |
|-------|-----------|-------|--------|
| DOC-01: `grep -c 'TestPersistence_' CLAUDE.md` | ≥ 8 | **9** | PASS |
| DOC-02: `grep -cE 'internal/tmux\|internal/session/instance\|internal/session/userconfig\|internal/session/storage\|cmd/session_cmd\|cmd/start_cmd\|cmd/restart_cmd' CLAUDE.md` | ≥ 6 | **6** | PASS |
| DOC-03: `grep -c '2026-04-14' CLAUDE.md` | ≥ 1 | **2** | PASS |
| DOC-04 flag: `grep -c 'launch_in_user_scope' CLAUDE.md` | ≥ 1 | **1** | PASS |
| DOC-04 rfc: `grep -cE 'RFC' CLAUDE.md` | ≥ 1 | **1** | PASS |
| SCRIPT-07 inline: `grep -c 'verify-session-persistence.sh' CLAUDE.md` | ≥ 1 | **3** | PASS |

All thresholds met. Per the plan's decision rule, **no CLAUDE.md patch applied**. Final counts = baseline counts.

## Final audit (CLAUDE.md at commit time)

Unchanged from baseline — CLAUDE.md was not modified in this plan:

- DOC-01 = 9 (≥ 8)
- DOC-02 = 6 (≥ 6)
- DOC-03 = 2 (≥ 1)
- DOC-04 flag = 1 (≥ 1), rfc = 1 (≥ 1)
- SCRIPT-07 inline = 3 (≥ 1)

## CHANGELOG.md change (DOC-05)

**File:** `CHANGELOG.md`
**Placement:** Inserted between the existing `## [Unreleased]` line (line 8) and `## [1.5.1] - 2026-04-13` (previously line 10, now line 13). The blank line that separated them was preserved above the new subsection.

**Exact bullet text added (lines 10-12 of CHANGELOG.md at commit time):**

```
### Fixed
- Session persistence: tmux servers now survive SSH logout on Linux+systemd hosts via `launch_in_user_scope` default (v1.5.2 hotfix). ([docs/SESSION-PERSISTENCE-SPEC.md](docs/SESSION-PERSISTENCE-SPEC.md))
```

**Verifications:**

| Check | Expected | Actual | Status |
|-------|----------|--------|--------|
| `grep -c '1.5.2' CHANGELOG.md` | ≥ 1 | 1 | PASS |
| `grep -ciE 'session.persistence' CHANGELOG.md` | ≥ 1 | 1 | PASS |
| `grep -c '## \[1.5.2\]' CHANGELOG.md` | == 0 | 0 | PASS (no release heading — tagging deferred) |
| `grep -c 'SESSION-PERSISTENCE-SPEC' CHANGELOG.md` | ≥ 1 | 1 | PASS |
| `awk '/## \[Unreleased\]/{flag=1} /## \[1.5.1\]/{flag=0} flag' CHANGELOG.md \| grep -c '### Fixed'` | == 1 | 1 | PASS (subsection sits under [Unreleased], above [1.5.1]) |

## Tasks executed

### Task 1: Audit CLAUDE.md mandate against DOC-01..04

**Outcome:** Audit PASS — all thresholds met on entry.
**Files modified:** none.
**Commit:** none (per plan decision rule: "If EVERY threshold is met: DO NOT modify CLAUDE.md... commit nothing and report the audit in the SUMMARY.").

### Task 2: Add DOC-05 one-liner to CHANGELOG.md under [Unreleased]

**Outcome:** Added `### Fixed` subsection with one bullet under `## [Unreleased]`. No `[1.5.2]` heading created.
**Files modified:** `CHANGELOG.md` (+3 lines, 0 deletions).
**Commit:** `a5d43ec` — `docs(04-02): add v1.5.2 session-persistence mention to CHANGELOG (DOC-05)`.

## Deviations from Plan

None — plan executed exactly as written. CLAUDE.md audit was PASS on entry (baseline counts already satisfied every DOC-01..04 and SCRIPT-07 threshold), so the "no patch, no commit" branch of the decision rule was taken. The only commit for this plan is the CHANGELOG.md DOC-05 bullet.

## Requirements closed

- **DOC-01**: CLAUDE.md names all eight `TestPersistence_*` tests verbatim — already satisfied at HEAD.
- **DOC-02**: CLAUDE.md names all six mandated code-path prefixes — already satisfied at HEAD.
- **DOC-03**: CLAUDE.md contains the `2026-04-14` incident date — already satisfied at HEAD.
- **DOC-04**: CLAUDE.md forbids flipping `launch_in_user_scope` back to `false` without an `RFC` — already satisfied at HEAD.
- **DOC-05**: CHANGELOG.md `[Unreleased] > ### Fixed` now mentions the v1.5.2 session-persistence hotfix with a link to the spec.

## Threat flags

None. Changes are confined to CHANGELOG.md (docs only). No mandated-code-path file modified. No new security-relevant surface introduced.

## Known stubs

None. CHANGELOG.md bullet is fully wired (links to the existing `docs/SESSION-PERSISTENCE-SPEC.md`).

## Commit ledger

| # | Hash      | Subject                                                                        | Files          |
| - | --------- | ------------------------------------------------------------------------------ | -------------- |
| 1 | `a5d43ec` | `docs(04-02): add v1.5.2 session-persistence mention to CHANGELOG (DOC-05)`    | CHANGELOG.md   |

All commits end with `Committed by Ashesh Goplani`. No `Co-Authored-By: Claude` or `Generated with Claude Code` strings anywhere in the commit history for this plan.

## Self-Check

Verified on 2026-04-14:

- `CHANGELOG.md` modified: confirmed via `git log -1 --name-only a5d43ec` → `CHANGELOG.md` only.
- Commit `a5d43ec` exists: confirmed via `git log --oneline | head -1` → `a5d43ec docs(04-02): add v1.5.2 session-persistence mention to CHANGELOG (DOC-05)`.
- CLAUDE.md unmodified since audit: confirmed via `git diff HEAD -- CLAUDE.md` (empty).
- DOC-05 grep acceptance criteria: all four greps returned the expected counts (see table above).
- Commit sign-off: `git log -1 --pretty=%B a5d43ec` contains `Committed by Ashesh Goplani` and no Claude attribution.

## Self-Check: PASSED
