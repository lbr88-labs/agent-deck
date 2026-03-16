---
phase: 11
slug: mcp-proxy-reliability
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-13
---

# Phase 11 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none |
| **Quick run command** | `go test -race -v ./internal/mcppool/...` |
| **Full suite command** | `go test -race -v ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -v ./internal/mcppool/...`
- **After every plan wave:** Run `go test -race -v ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 11-01-01 | 01 | 1 | MCP-01 | unit | `go test -race -v -run TestIDRewriteAndRestore ./internal/mcppool/` | ❌ W0 | ⬜ pending |
| 11-01-02 | 01 | 1 | MCP-02 | unit | `go test -race -v -run TestResponseRoutingNoXTalk ./internal/mcppool/` | ❌ W0 | ⬜ pending |
| 11-01-03 | 01 | 1 | MCP-03 | integration | `go test -race -v -run TestConcurrentToolCalls ./internal/mcppool/` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/mcppool/socket_proxy_test.go` — add `TestIDRewriteAndRestore` stub for MCP-01
- [ ] `internal/mcppool/socket_proxy_test.go` — add `TestResponseRoutingNoXTalk` stub for MCP-02
- [ ] `internal/mcppool/socket_proxy_test.go` — add `TestConcurrentToolCalls` stub for MCP-03

*No new framework install needed. File already exists with existing tests.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
