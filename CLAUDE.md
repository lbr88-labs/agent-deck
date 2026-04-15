# CLAUDE.md — Agent Deck (fix/feedback-closeout)

This file is repo-local guidance for agents (human or AI) working on the
`fix/feedback-closeout` worktree during milestone v1.5.3. It scopes two
non-negotiable rules:

1. A mandatory test-coverage discipline for the in-product feedback
   feature (REQ-FB-4), grounded in the 23 tests that shipped with the
   feature across earlier phases.
2. A repository-wide ban on `git commit --no-verify`, backed by two real
   incident commits discovered during the v1.5.3 closeout review.

Both rules apply to every commit on this branch and to every PR that
touches the feedback feature surface.

## Feedback feature: mandatory test coverage

The in-product feedback feature (CLI `agent-deck feedback` + TUI `Ctrl+E`
+ `FeedbackDialog` + `Sender.Send()` three-tier submit) is covered by 23
tests across three packages. These tests are LOAD-BEARING — they gate
the GraphQL primary tier, the clipboard/browser fallback tiers, the
headless detection path, and the dialog UX. All 23 must pass before any
PR that touches the feedback surface is merged.

### Test inventory (23 total)

| Package / Location | Count | Notes |
|--------------------|-------|-------|
| `internal/feedback` | 11 | Pre-existing suite: `ShouldShow_*` (4), `RecordRating_*` / `RecordOptOut` / `RecordShown` (3), `FormatComment` (1), `RatingEmoji` (1), `Send_GhAuthFailure` (1), `Send_Headless` (1). |
| `internal/ui` FeedbackDialog | 9 | All `FeedbackDialog_*` tests in `internal/ui/feedback_dialog_test.go`. |
| `cmd/agent-deck` feedback handler | 2 | `HandleFeedback_ValidRating`, `HandleFeedback_OptOut`. |
| `TestSender_DiscussionNodeID_IsReal` | 1 | Added in Phase 1 (RED, commit `23e49d2`), flipped GREEN in Phase 2 (commit `ae89731`). Locks the shape of `feedback.DiscussionNodeID` and blocks `D_PLACEHOLDER` regressions. |

Total: **23 tests.** No test may be deleted, skipped, or renamed without
updating this inventory in the same PR.

### Mandatory PR command

Any PR whose diff touches any of the following paths MUST include the
full stdout of the command below in the PR description (or as a
committed test-output artefact):

- `internal/feedback/**`
- `internal/ui/feedback_dialog.go`
- `cmd/agent-deck/feedback_cmd.go`
- `internal/platform/headless.go`

```
go test ./internal/feedback/... ./internal/ui/... ./cmd/agent-deck/... -run "Feedback|Sender_" -race -count=1
```

Expected output: 3 `ok` lines (one per package path), 0 `FAIL` lines, no
`SKIP` lines. If the command shows anything other than all green, the
PR must not merge.

### Placeholder-reintroduction rule: BLOCKER, not warning

Reintroducing the literal string `D_PLACEHOLDER` (or any other sentinel
that replaces the real Discussion node ID) as the value of
`feedback.DiscussionNodeID` in `internal/feedback/sender.go` is a
**blocker**, not a warning. PRs that do this must be rejected, not
merged with a TODO or "we'll fix it next release" comment.

The format regression test `TestSender_DiscussionNodeID_IsReal` exists
to catch this automatically (it asserts `DiscussionNodeID !=
"D_PLACEHOLDER"` AND matches `^D_[A-Za-z0-9_-]{10,}$`). If that test is
removed, weakened, or skipped, the PR must be rejected on that basis
alone.

## --no-verify mandate

**`git commit --no-verify` is FORBIDDEN on this repository.** The rule
applies to every commit on every branch, not just `fix/feedback-closeout`.
It is a repo-wide mandate.

### Why the hooks are load-bearing

The pre-commit hook chain on this repository runs:

- `gofmt` (fails the commit on any whitespace / formatting drift)
- `go vet` (fails on common Go mistakes: unreachable code, Printf
  format mismatches, etc.)
- Conventional-commit message lint (fails on subjects that don't match
  `^(feat|fix|docs|chore|test|refactor)\\([a-z0-9-]+\\): .+$` or
  similar — see `lefthook.yml`)

Every one of those checks is cheap (sub-second) and catches real
defects that only surface retroactively when someone else pulls the
branch. Bypassing them trades a few seconds of local friction for hours
of downstream cleanup.

### Incident evidence

Two commits in recent history demonstrate exactly what goes wrong when
hooks are skipped:

1. **`6785da6`** — `docs(05): scaffold Phase 5 — REQ-7 custom-command
   JSONL resume` (v1.5.2). This commit bypassed the pre-commit hooks
   and was later found to require follow-up work that the hooks would
   have flagged at commit time. Fixing it after the fact cost more
   effort than fixing it in place would have.

2. **`0d4f5b1`** — `feat(02-01): implement FeedbackDialog (GREEN)`
   (pre-v1.5.3). This commit landed with pre-existing `gofmt` debt in
   `internal/ui/feedback_dialog.go`. The `gofmt` pre-commit check
   would have caught it. Instead, the debt lingered and had to be
   cleaned up retroactively in a separate commit:
   **`a2b2f27`** — `chore: gofmt internal/ui/feedback_dialog.go
   (cleanup pre-existing fmt debt from 0d4f5b1)`. Net cost: two
   commits, a separate review cycle, and a dirty bisect history
   instead of one clean commit.

Both incidents are cheap to prevent and expensive to fix later. This
is the empirical basis for the ban.

### The remedy when a hook fails

When a pre-commit hook fails:

1. **Read the hook output.** The failure message tells you exactly
   which file and which check failed (e.g. `gofmt -l` listing drifted
   files; `go vet` printing the offending function).
2. **Fix the root cause.** Run `gofmt -w` on the listed files, fix the
   `vet` diagnostic, or correct the commit subject.
3. **Re-stage.** `git add <fixed-files>`.
4. **Create a NEW commit.** Run `git commit` again with the same
   message (or an updated one, if the fix changed the scope). The
   hook will re-run on the new commit.

**Never `git commit --amend` past a failed hook.** Amending past a
failure produces a commit with the same SHA pattern as one that was
never hook-checked, hiding the bypass from `git log`. If the original
commit that failed its hook never made it to HEAD (which is the usual
case — failed hooks abort the commit), there is nothing to amend. If
it did land somehow, fix forward with a new commit, never by
rewriting history.

**Never `git commit --no-verify`.** There is no scenario where this is
correct on this repository. If a hook is genuinely broken (e.g. a
flaky network check), fix the hook — don't bypass it.

---

*File scope: this CLAUDE.md applies to the `fix/feedback-closeout`
worktree as of milestone v1.5.3. It is not upstreamed to the parent
`/home/ashesh-goplani/agent-deck/CLAUDE.md` by this plan — that is a
separate decision for a future phase.*
