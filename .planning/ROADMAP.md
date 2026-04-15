# Agent Deck v1.5.3 Roadmap

**Milestone:** v1.5.3 — Feedback Closeout
**Starting point:** v1.5.2 (2026-04-14) — REQ-7 custom-command JSONL resume fix landed (top commit `9c0295d` on `fix/session-persistence`)
**Created:** 2026-04-15
**Granularity:** Minimal (3 phases, 30–45 min total)
**Parallelization:** None — strict TDD sequence: RED → GREEN → docs/mandate
**Branch:** `fix/feedback-closeout`
**Source spec:** `docs/FEEDBACK-CLOSEOUT-SPEC.md`

---

## Executive Summary

v1.5.3 is a closeout hotfix for the in-product feedback feature that landed on `main` across commits `e8ba9c6..775ec29`. The feature is functionally complete with 22 green tests, but `internal/feedback/sender.go:18` still holds `const DiscussionNodeID = "D_PLACEHOLDER"`. The first-tier GraphQL submit path fails silently against GitHub; fallback tiers work, so no user-visible regression — but the intended path is dead.

Three phases, executed sequentially for clean TDD lineage:
1. Land the regression test in RED state (fails against current placeholder).
2. Resolve the real GitHub Discussion node ID, replace the const, flip the test GREEN.
3. Add README entry + CLAUDE.md mandate locking in mandatory test coverage.

**Release-safety anchors carry forward from v1.5.0 and v1.5.2:**
- Go 1.24.0 toolchain pinned. Go 1.25 silently breaks macOS TUI. Non-negotiable.
- No SQLite schema changes.

**Hard rules (from spec):**
- TDD non-negotiable: `TestSender_DiscussionNodeID_IsReal` lands RED first, GREEN only after const update.
- No `git push`, `git tag`, `gh pr create`, `gh pr merge`.
- No `rm` — use `trash`.
- No `--no-verify`.
- Sign commits "Committed by Ashesh Goplani". No Claude attribution.
- No scope creep beyond files listed in REQ-FB-4's mandate block.

---

## Phases

- [x] **Phase 1: RED Format Regression Test** — Add `TestSender_DiscussionNodeID_IsReal` in `internal/feedback/sender_test.go` asserting the const is not `D_PLACEHOLDER` and matches `^D_[A-Za-z0-9_-]{10,}$`. Test MUST be RED against current code. ✓ 2026-04-15 (commit `23e49d2`)
- [ ] **Phase 2: Real Discussion Node ID** — Resolve the real GitHub Discussion node ID via `gh api graphql`, update `internal/feedback/sender.go:18`, flip RED test to GREEN. Full feedback suite remains green.
- [ ] **Phase 3: Docs and Mandate** — Add README "Feedback" section documenting `Ctrl+E` / `agent-deck feedback` with Discussion URL. Add "Feedback feature: mandatory test coverage" section to CLAUDE.md locking 22 existing tests + new format check.

---

## Phase Overview

| # | Phase | Requirements | Plans | Status | Depends on |
|---|-------|--------------|-------|--------|------------|
| 1 | RED Format Regression Test | REQ-FB-2 | 1 | Complete ✓ | — |
| 2 | Real Discussion Node ID | REQ-FB-1 | 1 | Pending | Phase 1 |
| 3 | Docs and Mandate | REQ-FB-3, REQ-FB-4 | 1 | Pending | Phase 2 |

**Total requirements mapped:** 4 / 4 (100%)

---

## Phase Details

### Phase 1: RED Format Regression Test

**Status:** Complete ✓ (2026-04-15, commit `23e49d2`)
**Goal:** Land a new test `TestSender_DiscussionNodeID_IsReal` that fails against the current `D_PLACEHOLDER` constant and locks the node ID format via regex. Commit message: `test(fb-01): TestSender_DiscussionNodeID_IsReal (RED)`.
**Depends on:** —
**Requirements:** REQ-FB-2
**Canonical refs:** `internal/feedback/sender.go`, `internal/feedback/sender_test.go`

**Success Criteria:**
1. `internal/feedback/sender_test.go` contains `TestSender_DiscussionNodeID_IsReal`.
2. Test asserts `DiscussionNodeID != "D_PLACEHOLDER"` AND `DiscussionNodeID` matches `^D_[A-Za-z0-9_-]{10,}$`.
3. Running `go test -run TestSender_DiscussionNodeID_IsReal ./internal/feedback/` FAILS (RED state — the current placeholder doesn't match the regex).
4. Commit lands with RED test before any production code change.
5. All other feedback tests remain green (`go test ./internal/feedback/... ./internal/ui/... ./cmd/agent-deck/... -run "Feedback|Sender_" -race -count=1` shows 22 pass, 1 fail).

**Out of Scope:**
- Any change to `sender.go` itself (that's Phase 2).
- Any docs or CLAUDE.md edits (Phase 3).

---

### Phase 2: Real Discussion Node ID

**Status:** Pending
**Goal:** Replace `const DiscussionNodeID = "D_PLACEHOLDER"` at `internal/feedback/sender.go:18` with the real node ID for the agent-deck GitHub Discussions "Feedback" category. Flip Phase 1's RED test to GREEN. Commit message: `fix(fb-01): replace D_PLACEHOLDER with real Discussion node ID (GREEN)`.
**Depends on:** Phase 1 (RED test must be in place first).
**Requirements:** REQ-FB-1
**Canonical refs:** `internal/feedback/sender.go:18`

**Success Criteria:**
1. `gh api graphql -f query='{ repository(owner: "asheshgoplani", name: "agent-deck") { discussions(first: 10) { nodes { id title } } } }'` (or category-scoped variant) resolves the real Discussion node ID.
2. `internal/feedback/sender.go:18` updated with real `D_kw...` node ID; constant appears in exactly one place (no duplicate literal).
3. `go test -run TestSender_DiscussionNodeID_IsReal ./internal/feedback/` PASSES (RED → GREEN transition).
4. Full feedback suite green: `go test ./internal/feedback/... ./internal/ui/... ./cmd/agent-deck/... -run "Feedback|Sender_" -race -count=1` shows 23 pass, 0 fail.
5. Manual smoke test from a non-headless host: `agent-deck feedback 4 "closeout smoke"` successfully creates a comment in the target Discussion (verified via `gh api` or browser).

**Out of Scope:**
- Any change to Sender three-tier fallback logic.
- Any change to FeedbackDialog UI.
- Any `internal/platform/headless.go` changes beyond maintaining `IsHeadless()`.

---

### Phase 3: Docs and Mandate

**Status:** Pending
**Goal:** Document the feedback feature in README and lock mandatory test coverage via CLAUDE.md. Commit message: `docs(v1.5.3): feedback README and CLAUDE.md mandate`.
**Depends on:** Phase 2 (feature must be fully functional before documenting).
**Requirements:** REQ-FB-3, REQ-FB-4
**Canonical refs:** `README.md`, `CLAUDE.md`

**Success Criteria:**
1. README has a "Feedback" section (top-level or inside Features) that mentions:
   - Press `Ctrl+E` in the TUI, or run `agent-deck feedback`, to send feedback.
   - Feedback posts to a public GitHub Discussion (link the URL).
2. `grep -i "ctrl+e" README.md` matches. `grep -i "agent-deck feedback" README.md` matches.
3. `agent-deck --help` still shows the `feedback` subcommand (no regression).
4. CLAUDE.md gains a "Feedback feature: mandatory test coverage" section that:
   - Names the 22 existing tests at suite granularity (`internal/feedback` 11, `internal/ui` FeedbackDialog 9, `cmd/agent-deck` 2) plus `TestSender_DiscussionNodeID_IsReal`.
   - Declares mandatory `go test ./internal/feedback/... ./internal/ui/... ./cmd/agent-deck/... -run "Feedback|Sender_" -race -count=1` output in PR description for any PR touching `internal/feedback/**`, `internal/ui/feedback_dialog.go`, `cmd/agent-deck/feedback_cmd.go`, or `internal/platform/headless.go`.
   - States that reintroducing a placeholder node ID is a blocker, not a warning.
5. `grep "Feedback feature: mandatory test coverage" CLAUDE.md` matches.

**Out of Scope:**
- Adding new feedback categories.
- Changing feature UX.

---

## Milestone Success Criteria

1. `go test ./internal/feedback/... ./internal/ui/... ./cmd/agent-deck/... -run "Feedback|Sender_" -race -count=1` returns 23 passing tests (22 existing + 1 new).
2. Manual end-to-end feedback submit creates a Discussion comment in the right repo.
3. `grep -i "feedback" README.md` AND `grep "Feedback feature: mandatory test coverage" CLAUDE.md` both match.
4. No `git push`, `git tag`, `gh pr create`, `gh pr merge` on this branch.
5. Every phase committed with "Committed by Ashesh Goplani" sign-off, no Claude attribution.

---

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| REQ-FB-1 | Phase 2 | Pending |
| REQ-FB-2 | Phase 1 | Pending |
| REQ-FB-3 | Phase 3 | Pending |
| REQ-FB-4 | Phase 3 | Pending |

**Coverage:** 4 / 4 requirements mapped (100%). No unmapped requirements.

---

*Roadmap created: 2026-04-15*
*Last updated: 2026-04-15 after milestone v1.5.3 initialization*
