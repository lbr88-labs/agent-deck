# agent-deck — Repo Instructions for Claude Code

This file is read by Claude Code when working inside the `agent-deck` repo. It lists hard rules for any AI or human contributor touching session lifecycle code.

## Session persistence: mandatory test coverage

Agent-deck has a recurring production failure where a single SSH logout on a Linux+systemd host destroys **every** managed tmux session. This has happened at least three times on the conductor host, most recently on 2026-04-14 at 09:08:01 local. Root cause: tmux servers inherit the login-session cgroup and are torn down with it, even when user lingering is enabled.

**As of v1.5.2, this class of bug is permanently test-gated.**

### The eight required tests

Any PR modifying any file in the paths listed below MUST run `go test -run TestPersistence_ ./internal/session/... -race -count=1` and include the output (or a link to the CI run) in the PR description. The following tests must all exist and pass:

1. `TestPersistence_TmuxSurvivesLoginSessionRemoval`
2. `TestPersistence_TmuxDiesWithoutUserScope`
3. `TestPersistence_LinuxDefaultIsUserScope`
4. `TestPersistence_MacOSDefaultIsDirect`
5. `TestPersistence_RestartResumesConversation`
6. `TestPersistence_StartAfterSIGKILLResumesConversation`
7. `TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion`
8. `TestPersistence_FreshSessionUsesSessionIDNotResume`

In addition, `bash scripts/verify-session-persistence.sh` MUST run end-to-end on a Linux+systemd host and exit zero with every scenario reporting `[PASS]`. This script is a human-watchable verification — it prints PIDs, cgroup paths, and the exact resume command lines so a reviewer can see with their own eyes that the fix is live.

### Paths under the mandate

A PR touching any of these requires the test output above:

- `internal/tmux/**`
- `internal/session/instance.go`
- `internal/session/userconfig.go`
- `internal/session/storage*.go`
- `cmd/session_cmd.go`
- `cmd/start_cmd.go`, `cmd/restart_cmd.go` if they exist
- The `scripts/verify-session-persistence.sh` file itself
- This `CLAUDE.md` section

### Forbidden changes without an RFC

- Flipping `launch_in_user_scope` default back to `false` on Linux.
- Removing any of the eight tests above.
- Adding a code path that starts a Claude session and ignores `Instance.ClaudeSessionID`.
- Disabling the `verify-session-persistence.sh` script in CI.

### Why this exists

The 2026-04-14 incident destroyed 33 live Claude conversations across in-flight GSD pipelines and bugfix sessions. The user has declared that this must never recur. The eight tests above replicate the exact failure mode. The visual script gives a human-in-the-loop confirmation. Both are P0 and cannot be skipped.

- Per-group config dir applies to custom-command sessions too; `TestPerGroupConfig_*` suite enforces this.

## General rules (inherited from user global)

- **Never `rm`** — use `trash`.
- **Never commit with Claude attribution** — no "🤖 Generated with Claude Code" or "Co-Authored-By: Claude" lines.
- **Never `git push`, `git tag`, `gh release`, `gh pr create/merge`** without explicit user approval.
- **No `--no-verify`** — every commit goes through pre-commit hooks. EXCEPTION: metadata-only commits — see `### --no-verify scope clarification` below.
- **TDD always** — the regression test for a bug lands BEFORE the fix.
- **Simplicity first** — every change minimal, targeted, no speculative refactoring.

### --no-verify scope clarification (v1.5.4+)

The `--no-verify` ban above applies to **source-modifying commits only**. Commits that touch ONLY metadata paths MAY use `--no-verify` iff the pre-commit hook would no-op. This clarification prevents paying 10–30s of hook latency on commits that provably don't require verification.

**Metadata-only paths (exempt from the ban):**

- `.planning/**` — phase plans, summaries, state, requirements
- `docs/**` — specs, design docs, architecture notes
- `*.md` outside `internal/`, `cmd/`, `pkg/`, and other source dirs
- `CHANGELOG.md` during milestone-prep phases

**Source-modifying commits (ban still applies):**

- Any commit that modifies `internal/**/*.go`, `cmd/**/*.go`, `pkg/**/*.go`, `scripts/**`, or any file under `internal/tmux/**`
- Any commit that modifies `go.mod`, `go.sum`, or CI config
- Repo-root `CLAUDE.md` and `README.md` (these affect development workflow and deployment)

**Negative example.** A commit staging `.planning/STATE.md` AND `internal/session/instance.go` is a SOURCE-modifying commit (source files present in the staged set). Hooks MUST run. You cannot bypass hooks by arguing the metadata file is the "primary" change.

**Positive example.** A commit staging ONLY `.planning/phases/04-*/04-01-SUMMARY.md` and `.planning/STATE.md` is metadata-only. Hooks would no-op (no Go source changed, no lint-relevant files). `--no-verify` is acceptable here.

Rationale: preserves the v1.5.3 test-gate for source changes while freeing metadata commits from hook latency. Phase 4 (v1.5.4) of this milestone established both the `--no-verify` ban and this clarification (see issue #602 for the conductor block that drove the Phase 4 volume of metadata commits).

## Current milestone

`fix/session-persistence` branch, targeting v1.5.2. Spec is at `docs/SESSION-PERSISTENCE-SPEC.md`. Do not work on unrelated changes in this branch.
