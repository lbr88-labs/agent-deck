---
phase: 04-verification-harness-docs-and-ci-wiring
plan: 03
subsystem: ci
tags: [ci, github-actions, gating, session-persistence, v1.5.2]

# Dependency graph
requires:
  - phase: 04-verification-harness-docs-and-ci-wiring
    plan: 01
    provides: scripts/verify-session-persistence.sh + fake-claude stub
  - phase: 03-resume-on-start-and-error-recovery-req-2-fix
    provides: internal/session/session_persistence_test.go with TestPersistence_* suite
provides:
  - .github/workflows/session-persistence.yml (per-PR CI gate on mandated paths)
affects: [04-04-final-signoff]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "GitHub Actions paths-filter gating tied 1:1 to the CLAUDE.md v1.5.2 mandated-paths list"
    - "Two-job CI workflow: TestPersistence_* suite (-race -count=1) + verify-session-persistence.sh"
    - "AGENT_DECK_VERIFY_USE_STUB=1 wired via env: on the harness step so CI never needs real claude"
    - "loginctl enable-linger '$USER' || true as an idempotent, non-fatal setup step"
    - "permissions: contents: read — read-only workflow, no secrets, no write scope"

key-files:
  created:
    - .github/workflows/session-persistence.yml
  modified: []

key-decisions:
  - "Additive workflow only — .github/workflows/release.yml is explicitly NOT modified (CLAUDE.md + CONTEXT.md hard rule)."
  - "Paths filter mirrors the eight mandated paths from CLAUDE.md plus the script, stub dir, test file, CLAUDE.md itself, and this workflow file — so any change that could regress v1.5.2 behavior triggers the gate."
  - "Both jobs pinned to ubuntu-latest: systemd user bus + loginctl are available there; the harness's Linux-only scenarios actually run (not SKIP)."
  - "go-version-file: go.mod used in both jobs (matching release.yml house style) so the Go version floats with the project's actual go.mod."
  - "No actions/cache declared — each run is a pure build-and-test, no cache-poisoning threat surface."

patterns-established:
  - "CI workflows that gate the v1.5.2 invariants use paths: filters matching the CLAUDE.md mandated paths verbatim."
  - "The verify-session-persistence.sh step sets AGENT_DECK_VERIFY_USE_STUB=1 via env: (not inline) so the contract is self-documenting."
  - "enable-linger runs with || true because it is idempotent and harmless when already enabled."

requirements-completed: [SCRIPT-07]

# Metrics
duration: 3min
completed: 2026-04-14
---

# Phase 04 Plan 03: CI Workflow Wiring (SCRIPT-07) Summary

**Shipped `.github/workflows/session-persistence.yml` — an additive per-PR GitHub Actions gate that runs the eight `TestPersistence_*` tests and the verify-session-persistence.sh harness on every PR touching the v1.5.2 mandated paths.**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-04-14
- **Completed:** 2026-04-14
- **Tasks:** 1 completed
- **Files created:** 1

## Accomplishments

- Added `.github/workflows/session-persistence.yml` with two jobs, both `runs-on: ubuntu-latest`:
  1. **tests** — checkout → setup-go (go.mod) → install tmux → `loginctl enable-linger "$USER" || true` → `go test -run TestPersistence_ ./internal/session/... -race -count=1`.
  2. **verify-script** — checkout → setup-go (go.mod) → install tmux → `loginctl enable-linger` → `go build -o /tmp/agent-deck ./cmd/agent-deck` (with `/tmp` on `$GITHUB_PATH`) → `AGENT_DECK_VERIFY_USE_STUB=1 bash scripts/verify-session-persistence.sh`.
- Triggered by `pull_request` touching any of the eight CLAUDE.md mandated paths plus the script, stub dir, test file, CLAUDE.md itself, and this workflow file. Also reachable via `workflow_dispatch` for ad hoc runs.
- `permissions: contents: read` — no write scope, no secrets used.
- `.github/workflows/release.yml` is untouched (verified by `git log -1 --name-only`).
- SCRIPT-07 closed: the verification harness from Plan 04-01 is now gated in CI.

## Task Commits

1. **Task 1: Create .github/workflows/session-persistence.yml** — `eda4728` (ci)

## Files Created/Modified

- `.github/workflows/session-persistence.yml` — 86 lines, two jobs, paths-filtered to the CLAUDE.md mandated set. Only file changed by the commit.

## Grep-Verifiable Suite (pasted output)

All acceptance counts captured immediately after commit `eda4728`:

```
YAML parse (python3 yaml.safe_load):            exit 0 (YAML_VALID=yes)
verify-session-persistence.sh count:            4   (>=1 required)
'go test -run TestPersistence_' count:          3   (>=1 required)
'go-version-file: go.mod' count:                2   (>=2 required, both jobs)
'runs-on: ubuntu-latest' count:                 2   (>=2 required, both jobs)
'enable-linger' count:                          2   (>=2 required)
'AGENT_DECK_VERIFY_USE_STUB' count:             1   (>=1 required)
'pull_request' count:                           1   (>=1 required)
Mandated-paths pattern count (7 patterns):      7   (>=7 required)
Tab characters in file:                         0   (must be 0)
```

## Mandated-Path Audit

`git log -1 --name-only` lists exactly:

- `.github/workflows/session-persistence.yml`

Mandated-path check (must be empty):

```
git log -1 --name-only --pretty=format: | grep -E '^(internal/tmux|internal/session/instance.go|internal/session/userconfig.go|internal/session/storage|cmd/session_cmd|cmd/start_cmd|cmd/restart_cmd)'
```

Returned NONE — no mandated read-only file was modified by this commit.

`.github/workflows/release.yml` untouched:

```
git log -1 --name-only --pretty=format: | grep -Fx '.github/workflows/release.yml'
```

Returned NONE — release.yml is NOT in the commit. Confirmed.

## Decisions Made

- **Additive-only**: created a NEW workflow file rather than extending `release.yml`. `release.yml` is tag-triggered (releases only) and CONTEXT.md/CLAUDE.md both forbid modifying it. A separate `pull_request`-triggered workflow is the right fit.
- **Both jobs on ubuntu-latest**: macOS runners lack systemd-run + systemd user bus. Scenario 2 of the harness would SKIP there, defeating the purpose of the CI gate. Linux-only is intentional and matches the CONTEXT's locked recipe.
- **`loginctl enable-linger "$USER" || true`**: trailing `|| true` guards against runners where linger is already enabled (idempotent no-op) or where the call fails harmlessly; either way the subsequent steps still run.
- **`permissions: contents: read`**: minimum viable scope. No writes, no releases, no PR comments — just read the repo, run the tests, report green or red.
- **Paths filter**: mirrors the eight mandated paths from CLAUDE.md's "Session persistence: mandatory test coverage" section verbatim, plus the harness script/stub dir/test file/CLAUDE.md/this workflow itself so every regression vector triggers the gate.

## Deviations from Plan

None — plan executed exactly as written. The workflow file content matches the plan's `<action>` block byte-for-byte.

## Issues Encountered

- The PreToolUse security-reminder hook flagged the Write tool on the workflow file as a generic GitHub-Actions-injection warning. No untrusted input is used in the workflow (no `github.event.*` expressions anywhere; only static shell commands and `$USER` from the runner env). The warning was a reminder, not a block — second attempt wrote successfully. No code change required.

## Threat Surface

Per the plan's `<threat_model>`:

- **T-04-03-01 (Elevation, workflow permissions)** — mitigated. `permissions: contents: read` only. No secrets referenced. No write scope anywhere in the file.
- **T-04-03-02 (Tampering, cache poisoning)** — accepted. Zero `actions/cache` declarations; every run is a pure build-and-test, nothing persisted between runs.
- **T-04-03-03 (DoS, long-running verify script)** — accepted. The harness has its own `sleep 2` per-scenario bounds; GitHub Actions' default 360-minute job timeout is the outer cap. No custom timeout override needed.

No new threat surface introduced beyond what was declared in the plan.

## CI Execution Note

This commit places the workflow file on branch `fix/session-persistence`. Per CLAUDE.md, **no `git push`, `git tag`, or `gh pr create` was run** — the workflow will only be evaluated by GitHub Actions when the user manually pushes the branch. Live CI execution is outside the scope of this phase.

Until the branch is pushed, the workflow file is dormant-but-correct: the YAML parses, all grep-verifiable counts pass, and the mandated paths filter is complete. Plan 04-04 (final sign-off) runs the harness locally on the conductor host; this workflow runs it on every future PR.

## User Setup Required

None — the workflow file is declarative and takes effect automatically once the branch is pushed. Branch protection (requiring both jobs green for PR merges) is a GitHub web-UI setting and is explicitly user-managed per CLAUDE.md / CONTEXT.md.

## Next Phase Readiness

- CI gate is in place for every future PR touching the v1.5.2 mandated paths.
- Plan 04-04 can now proceed: run the harness on the conductor host, capture output to `04-VERIFY.md`, and produce the phase sign-off commit.
- All three v1.5.2 deliverables (script + docs + CI) are now committed on the `fix/session-persistence` branch.

## Self-Check: PASSED

Verified via filesystem and git log (all items FOUND):

- `.github/workflows/session-persistence.yml` exists, parses as valid YAML (`python3 -c "import yaml; yaml.safe_load(...)"` exit 0).
- Commit `eda4728` present: `ci(04-03): add session-persistence workflow (SCRIPT-07)`.
- Commit message contains `Committed by Ashesh Goplani` and does NOT contain `Co-Authored-By: Claude` or `Generated with Claude Code`.
- `git log -1 --name-only` lists exactly `.github/workflows/session-persistence.yml` — no other files.
- No mandated-path file modified (grep over `internal/tmux|internal/session/instance.go|internal/session/userconfig.go|internal/session/storage|cmd/session_cmd|cmd/start_cmd|cmd/restart_cmd` returns empty).
- `.github/workflows/release.yml` NOT in commit (verified via `git log -1 --name-only --pretty=format: | grep -Fx '.github/workflows/release.yml'` returning empty).
- All grep-verifiable acceptance counts pass: verify-script (4), go test -run TestPersistence_ (3), go-version-file (2), runs-on ubuntu-latest (2), enable-linger (2), AGENT_DECK_VERIFY_USE_STUB (1), pull_request (1), mandated paths (7), tab chars (0).

---
*Phase: 04-verification-harness-docs-and-ci-wiring*
*Completed: 2026-04-14*
