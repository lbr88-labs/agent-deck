---
phase: 4
slug: framework-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-06
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + testify v1.11.1 |
| **Config file** | None (go test discovers `*_test.go` automatically) |
| **Quick run command** | `go test -race -v ./internal/integration/...` |
| **Full suite command** | `go test -race -v ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -v ./internal/integration/...`
- **After every plan wave:** Run `go test -race -v ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 04-01-01 | 01 | 1 | INFRA-01 | integration | `go test -race -v -run TestHarness ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 04-01-02 | 01 | 1 | INFRA-02 | unit | `go test -race -v -run TestWaitFor ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 04-01-03 | 01 | 1 | INFRA-03 | unit | `go test -race -v -run TestFixture ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 04-01-04 | 01 | 1 | INFRA-04 | integration | `go test -race -v -run TestIsolation ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 04-02-01 | 02 | 2 | LIFE-01 | integration | `go test -race -v -run TestLifecycleStart ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 04-02-02 | 02 | 2 | LIFE-02 | integration | `go test -race -v -run TestLifecycleStop ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 04-02-03 | 02 | 2 | LIFE-03 | integration | `go test -race -v -run TestLifecycleFork ./internal/integration/...` | ❌ W0 | ⬜ pending |
| 04-02-04 | 02 | 2 | LIFE-04 | integration | `go test -race -v -run TestLifecycleRestart ./internal/integration/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/integration/harness.go` — TmuxHarness with auto-cleanup
- [ ] `internal/integration/poll.go` — WaitForCondition, WaitForPaneContent, WaitForStatus
- [ ] `internal/integration/fixtures.go` — TestStorageFactory, InstanceBuilder
- [ ] `internal/integration/testmain_test.go` — TestMain with AGENTDECK_PROFILE=_test
- [ ] `internal/integration/lifecycle_test.go` — LIFE-01 through LIFE-04 test stubs

*All files are new; no existing infrastructure covers these.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Orphaned session cleanup after crash | INFRA-01 | Requires simulating process crash mid-test | 1. Run test, kill process mid-execution. 2. Check `tmux list-sessions \| grep inttest`. 3. Verify TestMain cleanup catches orphans on next run. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
