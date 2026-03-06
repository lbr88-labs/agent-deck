---
phase: 6
slug: conductor-pipeline-edge-cases
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-06
---

# Phase 6 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib testing + testify v1.11.1 |
| **Config file** | Makefile (test target: `go test -race -v ./...`) |
| **Quick run command** | `go test -race -v ./internal/integration/...` |
| **Full suite command** | `go test -race -v ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -v ./internal/integration/...`
- **After every plan wave:** Run `go test -race -v ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 06-01-01 | 01 | 1 | COND-03 | integration | `go test -race -v -run TestConductor_Heartbeat ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 06-01-02 | 01 | 1 | COND-04 | integration | `go test -race -v -run TestConductor_Chunked ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 06-02-01 | 02 | 1 | EDGE-01 | integration | `go test -race -v -run TestEdge_Skills ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 06-02-02 | 02 | 1 | EDGE-02 | integration | `go test -race -v -run TestEdge_ConcurrentPolling ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 06-02-03 | 02 | 1 | EDGE-03 | integration | `go test -race -v -run TestEdge_StorageWatcher ./internal/integration/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] COND-03, COND-04 tests added to `internal/integration/conductor_test.go`
- [ ] `internal/integration/edge_cases_test.go` created for EDGE-01, EDGE-02, EDGE-03

*Existing infrastructure covers all phase requirements. No new framework, fixtures, or config files needed.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
