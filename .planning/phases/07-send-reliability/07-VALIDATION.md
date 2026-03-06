---
phase: 7
slug: send-reliability
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-07
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify + integration package |
| **Config file** | `internal/integration/testmain_test.go` (profile isolation) |
| **Quick run command** | `go test -race -v -run "TestSend\|TestConductor_Send" ./cmd/agent-deck/... ./internal/integration/...` |
| **Full suite command** | `go test -race -v ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -v -run "TestSend\|TestConductor_Send" ./cmd/agent-deck/... ./internal/integration/...`
- **After every plan wave:** Run `go test -race -v ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 07-01-01 | 01 | 1 | SEND-01 | unit | `go test -race -v -run TestSendWithRetry ./cmd/agent-deck/... -x` | Exists | pending |
| 07-01-02 | 01 | 1 | SEND-01 | integration | `go test -race -v -run TestConductor_SendMultiple ./internal/integration/... -x` | Exists | pending |
| 07-02-01 | 02 | 1 | SEND-01 | integration | `go test -race -v -run TestSend_EnterRetry ./internal/integration/... -x` | W0 | pending |
| 07-02-02 | 02 | 1 | SEND-02 | unit | `go test -race -v -run TestWaitForAgentReady_Codex ./cmd/agent-deck/... -x` | W0 | pending |
| 07-02-03 | 02 | 1 | SEND-02 | integration | `go test -race -v -run TestSend_CodexReadiness ./internal/integration/... -x` | W0 | pending |
| 07-03-01 | 03 | 2 | SEND-01/02 | integration | `go test -race -v -run TestConductor ./internal/integration/... -x` | Exists | pending |

*Status: pending / green / red / flaky*

---

## Wave 0 Requirements

- [ ] `internal/integration/send_reliability_test.go` -- integration tests for Enter retry on real tmux and Codex readiness simulation (SEND-01, SEND-02)
- [ ] `cmd/agent-deck/session_send_test.go` -- unit tests for Codex-specific `waitForAgentReady` behavior (extend existing file)

*Existing infrastructure covers remaining requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Rapid successive sends under real TUI load | SEND-01 | Requires real Claude Code or Codex TUI running with production latency | 1. Start a Claude session. 2. Send 3 messages rapidly via `session send`. 3. Verify all 3 are submitted (not stuck in composer). |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
