---
phase: 17-release-pipeline-slack-bridge
plan: 01
subsystem: infra
tags: [ci, github-actions, goreleaser, install-script, bash, release-pipeline]

requires: []
provides:
  - CI asset validation step that fails the release workflow if any of the 4 platform tarballs are missing
  - Improved install.sh error messaging with CI workflow link and available asset listing
affects: [release-pipeline, install]

tech-stack:
  added: []
  patterns:
    - "Post-GoReleaser validation step using gh CLI to verify release assets exist before marking workflow success"
    - "jq-with-grep-fallback pattern for parsing GitHub API JSON in bash without requiring jq"

key-files:
  created: []
  modified:
    - .github/workflows/release.yml
    - install.sh

key-decisions:
  - "Use gh CLI (pre-installed on ubuntu-latest) for asset validation via GitHub API rather than curl+jq to keep the step simple"
  - "Check for exactly 4 platform tarballs (darwin/linux x amd64/arm64) plus checksums.txt as the validation set"
  - "install.sh falls back from jq to grep-based parsing since jq may not be installed at install time on some systems"

patterns-established:
  - "Validation step placement: always after GoReleaser, uses GH_TOKEN env for gh CLI auth"
  - "jq-with-grep-fallback: prefer jq for JSON parsing in bash when available, fall back to grep/sed"

requirements-completed: [REL-01, REL-02, REL-03]

duration: 3min
completed: 2026-03-16
---

# Phase 17 Plan 01: Release Pipeline & Slack Bridge Summary

**CI release asset validation for all 4 platform tarballs plus improved install.sh error messaging with GitHub Actions link**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-16T13:13:08Z
- **Completed:** 2026-03-16T13:16:00Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Added "Validate release assets" step to release.yml that checks all 4 platform tarballs and checksums.txt exist after GoReleaser runs, failing the workflow with a clear error listing missing assets
- Improved install.sh error handling: when release exists with no assets, shows CI workflow message and links to GitHub Actions; when release has assets but not for the current platform, lists available asset names
- Replaced brittle `grep -c` approach with `jq`-based parsing (with grep fallback for systems without jq)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add post-release asset validation to CI workflow** - `358c31f` (feat)
2. **Task 2: Improve install.sh error handling for empty releases** - `e016e78` (fix)

**Plan metadata:** `08b484a` (included in docs(18-01) commit — summary was staged alongside phase 18 metadata)

## Files Created/Modified
- `.github/workflows/release.yml` - Added "Validate release assets" step after GoReleaser with 4-platform check
- `install.sh` - Improved error handling block with jq parsing, CI workflow link, and available asset listing

## Decisions Made
- Used `gh` CLI for asset validation (pre-installed on ubuntu-latest runners, cleaner API access than raw curl)
- install.sh uses jq when available, falls back to grep+sed since jq may not be installed at install time
- Validation checks exactly: darwin_amd64, darwin_arm64, linux_amd64, linux_arm64 tarballs plus checksums.txt

## Deviations from Plan

None. Plan executed exactly as written.

## Issues Encountered

On first commit attempt, the lefthook pre-commit hook reported a vet error for `internal/ui/keyboard_compat_test.go` (`undefined: ParseCSIu`). On retry, `go vet ./...` passed cleanly (confirmed `internal/ui/keyboard_compat.go` defines all referenced functions). The first failure was a transient hook environment issue.

## User Setup Required

None. No external service configuration required.

## Next Phase Readiness

- Release CI pipeline now validates assets automatically on every tag push
- install.sh provides clearer error guidance when releases are incomplete
- Phase 17 Plan 02 (Slack bridge for release notifications) can proceed independently

## Self-Check: PASSED

- FOUND: `.github/workflows/release.yml` (with Validate release assets step)
- FOUND: `install.sh` (with improved error messaging)
- FOUND: `.planning/phases/17-release-pipeline-slack-bridge/17-01-SUMMARY.md`
- FOUND commit: `358c31f` (Task 1)
- FOUND commit: `e016e78` (Task 2)
- FOUND commit: `08b484a` (metadata, SUMMARY.md included)
