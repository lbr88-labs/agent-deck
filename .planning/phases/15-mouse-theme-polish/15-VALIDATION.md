---
phase: 15
slug: mouse-theme-polish
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-13
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test -race (testify assertions) |
| **Config file** | TestMain with AGENTDECK_PROFILE=_test (per package) |
| **Quick run command** | `go test -race -v ./internal/ui/... ./internal/git/...` |
| **Full suite command** | `go test -race -v ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -v ./internal/ui/... ./internal/git/...`
- **After every plan wave:** Run `go test -race -v ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 15-01-01 | 01 | 1 | UX-01 | unit | `go test -race -v -run TestMouseScroll ./internal/ui/...` | ❌ W0 | ⬜ pending |
| 15-01-02 | 01 | 1 | UX-01 | unit | `go test -race -v -run TestSettingsPanelScroll ./internal/ui/...` | ❌ W0 | ⬜ pending |
| 15-01-03 | 01 | 1 | UX-01 | unit | `go test -race -v -run TestGlobalSearchMouseScroll ./internal/ui/...` | ❌ W0 | ⬜ pending |
| 15-02-01 | 02 | 1 | UX-02 | unit | `go test -race -v -run TestLightThemePreview ./internal/ui/...` | ❌ W0 | ⬜ pending |
| 15-03-01 | 03 | 1 | UX-03 | manual | — (doc review) | n/a | ⬜ pending |
| 15-04-01 | 04 | 1 | UX-04 | unit | `go test -race -v -run TestBridgeHeartbeat ./...` | ❌ W0 | ⬜ pending |
| 15-05-01 | 05 | 1 | UX-05 | unit | `go test -race -v -run TestWorktreeReuse ./internal/git/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/ui/mouse_scroll_test.go` — stubs for UX-01 (mouse wheel on session list, settings, global search)
- [ ] `internal/ui/light_theme_test.go` — stubs for UX-02 (light theme preview rendering, no dark bleed)
- [ ] `internal/git/worktree_reuse_test.go` — stubs for UX-05 (GetWorktreeForBranch reuse path)

*UX-03 is documentation only. UX-04 is Python (conductor/bridge.py): manual verification or shell integration test.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| auto_cleanup documented in README | UX-03 | Documentation change, no code | Review README.md sandbox section for `auto_cleanup` entry |
| Light theme visual correctness | UX-02 | Visual rendering needs human eye | Run `agent-deck` with light theme, open a session preview, check for dark background bands |
| Bridge heartbeat skip when daemon present | UX-04 | Python code, cross-process behavior | Start bridge with OS daemon installed, verify only one heartbeat fires per interval |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
