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

The in-product feedback feature (CLI `agent-deck feedback` + TUI `Ctrl+E` + `FeedbackDialog` + `Sender.Send()` three-tier submit) is covered by 23 tests across three packages. These tests are LOAD-BEARING — they gate the GraphQL primary tier, the clipboard/browser fallback tiers, the headless detection path, and the dialog UX. All 23 must pass before any PR that touches the feedback surface is merged.

### Test inventory (23 total)

| Package / Location | Count | Notes |
|--------------------|-------|-------|
| `internal/feedback` | 11 | Pre-existing suite: `ShouldShow_*` (4), `RecordRating_*` / `RecordOptOut` / `RecordShown` (3), `FormatComment` (1), `RatingEmoji` (1), `Send_GhAuthFailure` (1), `Send_Headless` (1). |
| `internal/ui` FeedbackDialog | 9 | All `FeedbackDialog_*` tests in `internal/ui/feedback_dialog_test.go`. |
| `cmd/agent-deck` feedback handler | 2 | `HandleFeedback_ValidRating`, `HandleFeedback_OptOut`. |
| `TestSender_DiscussionNodeID_IsReal` | 1 | Locks the shape of `feedback.DiscussionNodeID` and blocks `D_PLACEHOLDER` regressions. |

Total: **23 tests.** No test may be deleted, skipped, or renamed without updating this inventory in the same PR.

### Mandatory PR command

Any PR whose diff touches any of the following paths MUST include the full stdout of the command below in the PR description:

- `internal/feedback/**`
- `internal/ui/feedback_dialog.go`
- `cmd/agent-deck/feedback_cmd.go`
- `internal/platform/headless.go`

```
go test ./internal/feedback/... ./internal/ui/... ./cmd/agent-deck/... -run "Feedback|Sender_" -race -count=1
```

### Placeholder-reintroduction rule: BLOCKER, not warning

Reintroducing `D_PLACEHOLDER` as the value of `feedback.DiscussionNodeID` in `internal/feedback/sender.go` is a **blocker**. The format regression test `TestSender_DiscussionNodeID_IsReal` catches this automatically.

## --no-verify mandate

**`git commit --no-verify` is FORBIDDEN on this repository.** The rule applies to every commit on every branch.

### Why the hooks are load-bearing

The pre-commit hook chain runs `gofmt`, `go vet`, and conventional-commit message lint. Every check is cheap (sub-second) and catches real defects.

### Incident evidence

Two commits demonstrate exactly what goes wrong when hooks are skipped:

1. **`6785da6`** — bypassed pre-commit hooks, required follow-up work that hooks would have caught.
2. **`0d4f5b1`** — landed with `gofmt` debt, requiring a separate cleanup commit (`a2b2f27`).

### The remedy when a hook fails

1. Read the hook output.
2. Fix the root cause (`gofmt -w`, fix `vet` diagnostic, correct commit subject).
3. Re-stage: `git add <fixed-files>`.
4. Create a NEW commit. **Never `git commit --amend` past a failed hook. Never `git commit --no-verify`.**

## General rules

- **Never `rm`** — use `trash`.
- **Never commit with Claude attribution** — no "Generated with Claude Code" or "Co-Authored-By: Claude" lines.
- **Never `git push`, `git tag`, `gh release`, `gh pr create/merge`** without explicit user approval.
- **TDD always** — the regression test for a bug lands BEFORE the fix.
- **Simplicity first** — every change minimal, targeted, no speculative refactoring.
