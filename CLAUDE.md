# agent-deck — Repo Instructions for Claude Code

This file is read by Claude Code when working inside the `agent-deck` repo. It lists hard rules for any AI or human contributor.

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

## Feedback feature: mandatory test coverage

The in-product feedback feature is covered by 23 tests across three packages. All 23 must pass before any PR that touches the feedback surface is merged.

### Mandatory PR command for feedback paths

```
go test ./internal/feedback/... ./internal/ui/... ./cmd/agent-deck/... -run "Feedback|Sender_" -race -count=1
```

### Placeholder-reintroduction rule: BLOCKER

Reintroducing `D_PLACEHOLDER` as the value of `feedback.DiscussionNodeID` is a **blocker**. `TestSender_DiscussionNodeID_IsReal` catches this automatically.

## Per-group config: mandatory test coverage

Per-group config dir applies to custom-command sessions too; `TestPerGroupConfig_*` suite enforces this.

## --no-verify mandate

**`git commit --no-verify` is FORBIDDEN on source-modifying commits.** Metadata-only commits (`.planning/**`, `docs/**`, non-source `*.md`) MAY use `--no-verify` when hooks would no-op.

Incident evidence: commits `6785da6` and `0d4f5b1` demonstrate the cost of skipping hooks.

## General rules

- **Never `rm`** — use `trash`.
- **Never commit with Claude attribution** — no "Generated with Claude Code" or "Co-Authored-By: Claude" lines.
- **Never `git push`, `git tag`, `gh release`, `gh pr create/merge`** without explicit user approval.
- **TDD always** — the regression test for a bug lands BEFORE the fix.
- **Simplicity first** — every change minimal, targeted, no speculative refactoring.
