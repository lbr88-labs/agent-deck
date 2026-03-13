---
phase: 14
slug: detection-sandbox
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-13
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + `go test -race` |
| **Config file** | none (uses TestMain profile isolation) |
| **Quick run command** | `go test -race -v ./internal/session/... ./internal/tmux/...` |
| **Full suite command** | `go test -race -v ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -v ./internal/session/... ./internal/tmux/...`
- **After every plan wave:** Run `go test -race -v ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | DET-01 | unit | `go test -race -v -run TestSandbox ./internal/session/...` | ❌ W0 | ⬜ pending |
| 14-01-02 | 01 | 1 | DET-01 | unit | `go test -race -v -run TestSandbox ./internal/session/...` | ❌ W0 | ⬜ pending |
| 14-02-01 | 02 | 1 | DET-02 | unit | `go test -race -v -run TestOpenCode ./internal/tmux/...` | ❌ W0 | ⬜ pending |
| 14-02-02 | 02 | 1 | DET-02 | unit | `go test -race -v -run TestOpenCode ./internal/tmux/...` | ❌ W0 | ⬜ pending |
| 14-02-03 | 02 | 1 | DET-02 | unit | `go test -race -v -run TestOpenCode ./internal/tmux/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/session/sandbox_env_test.go` — tests that command builders do NOT embed `tmux set-environment` for sandbox sessions, and that `SetEnvironment` is called from host side (DET-01)
- [ ] `internal/tmux/opencode_detection_test.go` — tests that `HasPrompt("opencode", ...)` returns true for question-tool help bar content (DET-02)

*Existing infrastructure: `internal/session/testmain_test.go` and `internal/tmux/testmain_test.go` already provide profile isolation.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Docker sandbox session picks up env vars set by host tmux | DET-01 | Requires running Docker sandbox environment | 1. Start a sandbox session 2. Check `tmux show-environment` shows session ID 3. Verify spawned process can read it |
| OpenCode question tool shows "waiting" in agent-deck | DET-02 | Requires running OpenCode with question tool trigger | 1. Start OpenCode session 2. Trigger question tool 3. Verify agent-deck shows "waiting" status |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
