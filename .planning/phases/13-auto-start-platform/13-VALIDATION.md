---
phase: 13
slug: auto-start-platform
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-13
---

# Phase 13 -- Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify (existing) |
| **Config file** | None (go test flags) |
| **Quick run command** | `go test -race -v ./internal/tmux/... ./internal/session/... -run TestPaneReady\|TestSyncSessionIDs\|TestStopSavesSessionID\|TestGenerateUUID\|TestBuildClaudeCommandNoUuidgen\|TestBuildForkCommandNoUuidgen` |
| **Full suite command** | `make test` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -v ./internal/tmux/... ./internal/session/... -run TestPaneReady\|TestSyncSessionIDs\|TestStopSavesSessionID\|TestGenerateUUID\|TestBuildClaudeCommandNoUuidgen\|TestBuildForkCommandNoUuidgen`
- **After every plan wave:** Run `make test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 13-01-01 | 01 | 1 | PLAT-01 | unit | `go test -race -v ./internal/tmux/... -run TestIsPaneShellReady` | W0 (TDD) | pending |
| 13-01-02 | 01 | 1 | PLAT-01 | unit+integration | `go test -race -v ./internal/tmux/... -run TestWaitForPaneReady` | W0 (TDD) | pending |
| 13-01-03 | 01 | 1 | PLAT-01 | unit | `go test -race -v ./internal/session/... -run TestGenerateUUID` | W0 (TDD) | pending |
| 13-01-04 | 01 | 1 | PLAT-01 | unit | `go test -race -v ./internal/session/... -run TestBuildClaudeCommandNoUuidgen` | W0 (TDD) | pending |
| 13-01-05 | 01 | 1 | PLAT-01 | unit | `go test -race -v ./internal/session/... -run TestBuildForkCommandNoUuidgen` | W0 (TDD) | pending |
| 13-02-01 | 02 | 2 | PLAT-02 | unit | `go test -race -v ./internal/session/... -run TestSyncSessionIDsFromTmux` | W0 (TDD) | pending |
| 13-02-02 | 02 | 2 | PLAT-02 | integration | `go test -race -v ./internal/session/... -run TestStopSavesSessionID` | W0 (TDD) | pending |

*Status: pending / green / red / flaky*

---

## Wave 0 Strategy

All plans use `tdd="true"` on code-producing tasks. This means each task writes failing tests FIRST (RED), then implements to pass them (GREEN). This is equivalent to Wave 0 test scaffolding but integrated into the task execution flow rather than as a separate pre-step. The TDD discipline ensures:

1. Tests exist BEFORE implementation code
2. Tests fail initially (proving they test real behavior, not tautologies)
3. Implementation is driven by test expectations

This approach is validated as nyquist-compliant because every code-producing task has explicit `<behavior>` blocks defining test expectations, and every `<verify>` block contains an automated test command.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| WSL2 cold-start launch works | PLAT-01 | Requires WSL2 host with tmux | SSH into WSL2 machine, run `agent-deck session start test-wsl --tool claude /tmp/test` from a non-interactive context (e.g., `bash -c 'agent-deck session start ...'`), verify tool process starts |
| Resume attaches correct conversation on WSL | PLAT-02 | Requires WSL2 + prior session | After starting and stopping a session on WSL2, run `agent-deck session start test-wsl --resume`, verify conversation continuity |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify with behavioral test commands
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covered via TDD task structure (tests written before implementation)
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
