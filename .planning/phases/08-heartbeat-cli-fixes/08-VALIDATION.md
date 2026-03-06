---
phase: 8
slug: heartbeat-cli-fixes
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-07
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + `go test -race` |
| **Config file** | Per-package TestMain files (4 locations) |
| **Quick run command** | `go test -race -v -run TestName ./cmd/agent-deck/... ./internal/session/...` |
| **Full suite command** | `go test -race -v ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -v ./cmd/agent-deck/... ./internal/session/...`
- **After every plan wave:** Run `go test -race -v ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | HB-01 | unit | `go test -race -v -run TestConductorHeartbeatScript ./internal/session/...` | Partially (needs group-scoping assertion) | ⬜ pending |
| 08-01-02 | 01 | 1 | HB-02 | unit | `go test -race -v -run TestHeartbeat ./internal/session/...` | ❌ W0 | ⬜ pending |
| 08-02-01 | 02 | 1 | CLI-01 | unit | `go test -race -v -run TestWaitForCompletion ./cmd/agent-deck/...` | Partially (needs session-death test) | ⬜ pending |
| 08-02-02 | 02 | 1 | CLI-02 | unit | `go test -race -v -run TestReorderArgs ./cmd/agent-deck/...` | ❌ W0 | ⬜ pending |
| 08-02-03 | 02 | 1 | CLI-03 | unit+manual | `go test -race -v -run TestNoParent ./cmd/agent-deck/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/session/conductor_test.go` — Test `conductorHeartbeatScript` contains group-scoped message (not "all sessions in the profile")
- [ ] `internal/session/conductor_test.go` — Test `GetHeartbeatInterval` returns 0 when input is 0, and heartbeat script contains config-enabled check
- [ ] `cmd/agent-deck/session_send_test.go` — Test `waitForCompletion` handles consecutive GetStatus errors (session death scenario)
- [ ] `cmd/agent-deck/main_test.go` or `cli_utils_test.go` — Test `reorderArgsForFlagParsing` with `-c claude -g mygroup .` produces correct output
- [ ] `cmd/agent-deck/main_test.go` — Test `--no-parent` help text content (may be manual verification)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| CLI-03 help text clarity | CLI-03 | Natural language content | Run `agent-deck add --help` and verify `--no-parent` description mentions `set-parent` recovery |
| Heartbeat script installed correctly after migration | HB-01 | Requires launchd/installed scripts | Run `conductor setup`, then inspect `~/.agent-deck/conductor/*/heartbeat.sh` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
