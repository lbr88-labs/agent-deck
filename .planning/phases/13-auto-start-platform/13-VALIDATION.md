---
phase: 13
slug: auto-start-platform
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-13
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify (existing) |
| **Config file** | None (go test flags) |
| **Quick run command** | `go test -race -v ./internal/tmux/... ./internal/session/... -run TestPaneReady\|TestSyncSessionIDs\|TestStopSavesSessionID` |
| **Full suite command** | `make test` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -v ./internal/tmux/... ./internal/session/... -run TestPaneReady\|TestSyncSessionIDs\|TestStopSavesSessionID`
- **After every plan wave:** Run `make test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 13-01-01 | 01 | 1 | PLAT-01 | unit | `go test -race -v ./internal/tmux/... -run TestIsPaneShellReady` | ❌ W0 | ⬜ pending |
| 13-01-02 | 01 | 1 | PLAT-01 | unit | `go test -race -v ./internal/tmux/... -run TestWaitForPaneReady` | ❌ W0 | ⬜ pending |
| 13-01-03 | 01 | 1 | PLAT-01 | integration | `go test -race -v ./internal/tmux/... -run TestStartWithPaneReady` | ❌ W0 | ⬜ pending |
| 13-02-01 | 02 | 1 | PLAT-02 | unit | `go test -race -v ./internal/session/... -run TestSyncSessionIDsFromTmux` | ❌ W0 | ⬜ pending |
| 13-02-02 | 02 | 1 | PLAT-02 | unit/integration | `go test -race -v ./internal/session/... -run TestStopSavesSessionID` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/tmux/pane_ready_test.go` — stubs for PLAT-01 (isPaneShellReady, waitForPaneReady)
- [ ] `internal/session/instance_test.go` additions — stubs for PLAT-02 (SyncSessionIDsFromTmux, stop-saves-id)
- Note: Tests requiring a running tmux server must call `skipIfNoTmuxServer(t)` per project convention

*Existing infrastructure covers framework and test runner; only new test files/additions needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| WSL2 cold-start launch works | PLAT-01 | Requires WSL2 host with tmux | SSH into WSL2 machine, run `agent-deck session start test-wsl --tool claude /tmp/test` from a non-interactive context (e.g., `bash -c 'agent-deck session start ...'`), verify tool process starts |
| Resume attaches correct conversation on WSL | PLAT-02 | Requires WSL2 + prior session | After starting and stopping a session on WSL2, run `agent-deck session start test-wsl --resume`, verify conversation continuity |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
