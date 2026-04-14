# Agent Deck v1.5.1 Roadmap

**Milestone:** v1.5.1 — Bug Fix Patch
**Starting point:** v1.5.0 (2026-04-10)
**Created:** 2026-04-12
**Granularity:** Standard (5 phases, 4 bug fixes + 1 housekeeping)
**Parallelization:** None (each bug fix is independent but sequential for clean git history)

---

## Executive Summary

v1.5.1 is a patch release fixing 4 user-reported bugs that cause downgrades from v1.5.0. Each bug gets TDD treatment (failing test first, then fix). Also includes reviewing 6 community PRs and closing stale issues.

Two release-safety anchors carry forward from v1.5.0:
- **Go 1.24.0 toolchain pinned** at every layer. Go 1.25 silently breaks macOS TUI.
- **No SQLite schema changes this milestone.** Any new persistence uses localStorage or config files.

TDD is non-negotiable: every bug must have a failing test BEFORE the fix is implemented.

---

## Phases

- [x] **Phase 1: Web Terminal Resize** — Web terminal fills browser viewport on resize with PTY + tmux window size updates (COMPLETE)
- [x] **Phase 2: Underscore Key Input** — Allow underscore '_' character in TUI dialog text input fields (COMPLETE)
- [x] **Phase 3: Ctrl+C Session Detach Regression** — Ctrl+C inside attached Codex/Claude session forwards SIGINT to agent instead of exiting to TUI. Restore v0.27.5 behavior. (completed 2026-04-12)
- [ ] **Phase 4: Scrollback Buffer Contamination** — Clear scrollback buffer on session switch so host terminal shows only current session content
- [ ] **Phase 5: Community PRs and Housekeeping** — Review/merge 6 community PRs, close 3 stale PRs and 3 spam/fixed issues

---

## Phase Overview

| # | Phase | Requirements | Plans | Status | Blocks |
|---|-------|-------------|-------|--------|--------|
| 1 | Web Terminal Resize | 1 (BUG-01) | 1 | COMPLETE | — |
| 2 | Underscore Key Input | 1 (BUG-02) | 1 | COMPLETE | — |
| 3 | Ctrl+C Session Detach Regression | 1 (BUG-03) | 1 | Planned | — |
| 4 | Scrollback Buffer Contamination | 1 (BUG-04) | 1 | Planned | — |
| 5 | Community PRs and Housekeeping | 8 (PR-01..06, CLEAN-01..02) | TBD | Pending | — |

**Total requirements mapped:** 12 / 12 (100%)

---

## Phase Details

### Phase 1: Web Terminal Resize

**Status:** COMPLETE
**Goal:** Web terminal fills browser viewport on resize. Both PTY size and tmux window size update when browser resizes.
**Depends on:** —
**Requirements:** BUG-01
**Canonical refs:** `internal/web/terminal_bridge.go`, `internal/tmux/tmux.go`

**Success Criteria:**
1. Browser resize triggers PTY resize via `pty.Setsize()` on the bridge's ptmx
2. tmux window also resized via `tmux resize-window`
3. Terminal content reflows correctly after resize
4. Integration test covers resize path

**Commits:**
- `4073efa` test(01-01): add failing integration test for web terminal resize + TestMain isolation (RED)
- `7f0b887` fix(01-01): implement web terminal resize with pty.Setsize + tmux resize-window (GREEN)

---

### Phase 2: Underscore Key Input

**Status:** COMPLETE
**Goal:** Underscore '_' character can be typed in session name and path input fields in TUI dialogs.
**Depends on:** —
**Requirements:** BUG-02
**Canonical refs:** `internal/ui/newdialog.go`, `internal/ui/home.go`

**Success Criteria:**
1. Underscore character reaches the text input buffer
2. Character persists after session creation
3. All TUI dialog text inputs accept underscore

**Commits:**
- `5505eea` test(02-01): add failing tests for underscore input in dialog text fields (RED)
- `817a616` fix(02-01): allow underscore character in TUI dialog text inputs (GREEN)

---

### Phase 3: Ctrl+C Session Detach Regression

**Status:** Planned
**Goal:** Ctrl+C inside an attached Codex/Claude session forwards SIGINT to the agent process, not exits back to TUI. Restore v0.27.5 behavior where Ctrl+C was forwarded to the attached session.
**Depends on:** —
**Requirements:** BUG-03
**Plans:** 1/1 plans complete
**Canonical refs:** `internal/tmux/pty.go`, `internal/ui/home.go` (attach mode handling)

**Root cause (from research):** The 50ms `controlSeqTimeout` at `internal/tmux/pty.go:194` drops ALL bytes after an escape sequence timeout, including standalone Ctrl+C (0x03) that is not part of any escape sequence. In v0.27.5, Ctrl+C was forwarded through the PTY to the attached tmux session. The regression likely introduced a signal handler or key interception that catches SIGINT/Ctrl+C before it reaches the PTY write path.

**Files likely involved:**
- `internal/tmux/pty.go` — controlSeqTimeout logic that drops bytes
- `internal/ui/home.go` — attach mode keyboard handling
- `internal/session/` — signal forwarding during attach

Plans:
- [x] 03-01-PLAN.md — TDD: failing tests for Ctrl+C forwarding, then fix controlSeqTimeout + SIGINT handling

**Success Criteria:**
1. Ctrl+C in attached Codex session sends SIGINT to the agent process (not TUI exit)
2. Ctrl+C in attached Claude session sends SIGINT to the agent process (not TUI exit)
3. TUI detach still works via the designated detach key (Ctrl+D or Esc)
4. Regression test proves Ctrl+C is forwarded through the PTY

---

### Phase 4: Scrollback Buffer Contamination

**Status:** Planned
**Goal:** Clear scrollback buffer on session switch so host terminal shows only current session content.
**Depends on:** —
**Requirements:** BUG-04
**Plans:** 1 plan
**Canonical refs:** `internal/tmux/pty.go` (cleanupAttach closure), `internal/ui/home.go` (session switch handling)

**Root cause:** `cleanupAttach()` in `internal/tmux/pty.go` resets terminal SGR styles but does NOT emit `\033[3J` (Erase Saved Lines). The host terminal's scrollback buffer retains the previous session's output after detach.

Plans:
- [ ] 04-01-PLAN.md — TDD: failing tests for scrollback clear on detach, then add \033[3J to cleanupAttach()

**Success Criteria:**
1. After switching sessions, scrolling up shows only current session content
2. Works across iTerm2 and WezTerm
3. Uses `\033[3J` (xterm clear-scrollback) emitted in cleanupAttach()
4. Regression test covers session switch scrollback clearing

---

### Phase 5: Community PRs and Housekeeping

**Status:** Pending
**Goal:** Review and merge/close 6 community PRs. Close 3 stale PRs and 3 spam/fixed issues.
**Depends on:** —
**Requirements:** PR-01, PR-02, PR-03, PR-04, PR-05, PR-06, CLEAN-01, CLEAN-02

**Community PRs to review:**
- PR #566: Esc in setup wizard
- PR #565: install script tmux check
- PR #562: worktree branch auto-populate
- PR #557: arrow-key nav for confirm dialogs
- PR #551: stabilize tmux-backed test suites
- PR #550: compatible custom tools

**Housekeeping:**
- Close stale PRs: #540, #545, #521
- Close spam/fixed issues: #552, #553, #544

**Success Criteria:**
1. Each PR reviewed with clear feedback
2. Accepted PRs merged with CI green
3. Rejected PRs closed with constructive explanation
4. Stale PRs and spam issues closed with brief rationale

---

*Roadmap created: 2026-04-12*
*Last updated: 2026-04-12*
