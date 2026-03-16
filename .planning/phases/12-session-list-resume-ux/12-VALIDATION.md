---
phase: 12
slug: session-list-resume-ux
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-13
---

# Phase 12 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (stdlib testing + `stretchr/testify` v1.11.1) |
| **Config file** | none — standard Go test runner |
| **Quick run command** | `go test -race -v ./internal/session/... ./internal/ui/...` |
| **Full suite command** | `make test` (runs `go test -race -v ./...`) |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -v ./internal/session/... ./internal/ui/...`
- **After every plan wave:** Run `make test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 12-01-01 | 01 | 1 | VIS-01 | unit | `go test -race -v ./internal/ui/... -run TestRebuildFlatItems_StoppedSessionVisible` | ❌ W0 | ⬜ pending |
| 12-01-02 | 01 | 1 | VIS-02 | unit | `go test -race -v ./internal/ui/... -run TestPreviewPane_StoppedVsError` | ❌ W0 | ⬜ pending |
| 12-01-03 | 01 | 1 | VIS-03 | unit | `go test -race -v ./internal/ui/... -run TestSessionPickerDialog_FiltersStopped` | ✅ | ⬜ pending |
| 12-02-01 | 02 | 1 | DEDUP-01 | unit | `go test -race -v ./internal/session/... -run TestRestart_ReusesExistingRecord` | ❌ W0 | ⬜ pending |
| 12-02-02 | 02 | 1 | DEDUP-02 | unit | `go test -race -v ./internal/session/... -run TestUpdateClaudeSessionsWithDedup_OnRestart` | ❌ W0 | ⬜ pending |
| 12-02-03 | 02 | 1 | DEDUP-03 | integration | `go test -race -v ./internal/session/... -run TestConcurrentStorageWrites` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/ui/home_flatitems_test.go` — stubs for VIS-01 (TestRebuildFlatItems_StoppedSessionVisible)
- [ ] `internal/ui/home_preview_test.go` — stubs for VIS-02 (TestPreviewPane_StoppedVsError)
- [ ] `internal/session/storage_concurrent_test.go` — stubs for DEDUP-03 (TestConcurrentStorageWrites)
- [ ] `internal/session/lifecycle_test.go` extension — stubs for DEDUP-01 (TestRestart_ReusesExistingRecord)

*Existing infrastructure covers VIS-03 (session_picker_dialog_test.go already exists).*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Stopped session visual styling (dim gray vs red) | VIS-01/VIS-02 | Lipgloss rendering is terminal-dependent | Run `agent-deck`, stop a session, verify dim gray icon vs red error icon |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
