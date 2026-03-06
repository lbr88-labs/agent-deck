---
phase: 2
slug: testing-bug-fixes
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-06
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | None (Go convention: `_test.go` files) |
| **Quick run command** | `go test -race -v -run TestSpecificName ./internal/session/...` |
| **Full suite command** | `go test -race -v ./... -timeout 120s` |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -v -run TestNewTest ./internal/session/... -timeout 30s`
- **After every plan wave:** Run `go test -race -v ./... -timeout 120s`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | TEST-01 | integration | `go test -race -v -run TestSleepWake ./internal/session/... -timeout 30s` | Partial | pending |
| 02-01-02 | 01 | 1 | TEST-07 | integration | `go test -race -v -run TestStatusTrack ./internal/session/... -timeout 30s` | Partial | pending |
| 02-02-01 | 02 | 1 | TEST-03 | integration | `go test -race -v -run TestSessionStart ./internal/session/... -timeout 30s` | Partial | pending |
| 02-02-02 | 02 | 1 | TEST-04 | integration | `go test -race -v -run TestSessionStop ./internal/session/... -timeout 30s` | No | pending |
| 02-02-03 | 02 | 1 | TEST-05 | integration | `go test -race -v -run TestFork ./internal/session/... -timeout 30s` | Yes | pending |
| 02-02-04 | 02 | 1 | TEST-06 | unit | `go test -race -v -run TestAttach ./internal/session/... -timeout 30s` | No | pending |
| 02-03-01 | 03 | 2 | TEST-02 | unit | `go test -race -v -run TestSkill ./internal/session/... -timeout 30s` | Partial | pending |
| 02-03-02 | 03 | 2 | STAB-01 | regression | `go test -race -v ./... -timeout 120s` | N/A | pending |

*Status: pending · green · red · flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements:
- `testmain_test.go` files provide profile isolation (`AGENTDECK_PROFILE=_test`)
- `skipIfNoTmuxServer(t)` helper handles CI environments
- `newTestStorage(t)` helper provides SQLite test databases

*No new framework or config needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Session attach connects to PTY | TEST-06 | Requires interactive terminal | 1. Start session 2. Run `agent-deck session attach <name>` 3. Verify tmux pane displays |

*Unit-level verification of attach parameters is automated. Full PTY attach is manual.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
