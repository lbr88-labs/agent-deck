---
phase: 14
slug: detection-sandbox
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-13
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify/assert, `go test -race` |
| **Config file** | none (`TestMain` enforces `AGENTDECK_PROFILE=_test`) |
| **Quick run command** | `go test -race -v ./internal/session/... -run TestCommandBuilders_NoTmuxSetEnv && go test -race -v ./internal/tmux/... -run TestOpencode` |
| **Full suite command** | `go test -race -v ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race ./internal/tmux/... -run TestOpencode` and `go test -race ./internal/session/... -run TestCommandBuilders`
- **After every plan wave:** Run `go test -race -v ./internal/tmux/... ./internal/session/...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | DET-01 | unit (table-driven) | `go test ./internal/session/... -run TestCommandBuilders_NoTmuxSetEnv -v` | TDD (plan creates in RED phase) | ⬜ pending |
| 14-02-01 | 02 | 1 | DET-02 | unit (extend existing) | `go test ./internal/tmux/... -run TestOpencodeBusyGuard -v` | ✅ (extend) | ⬜ pending |
| 14-02-02 | 02 | 1 | DET-02 | unit | `go test ./internal/tmux/... -run TestDefaultRawPatterns_OpenCode -v` | ✅ (extend) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

### Coverage Notes

**14-01-01 (TestCommandBuilders_NoTmuxSetEnv):** Single table-driven test in `internal/session/sandbox_env_test.go` with subtests covering all command builders: Claude new session, Claude resume, Claude resume with session-id, Gemini resume, Gemini fresh, OpenCode resume, Codex resume, Claude resume command, and UUID pre-generation. This replaces the previously listed 5 separate test functions (TestBuildClaudeCommand_NoTmuxSetEnv, etc.) which were never in the plan. The table-driven approach provides equivalent or better coverage.

**14-02-01 (TestOpencodeBusyGuard):** Extended with 4+ new test cases covering question tool help bar, permission approval dialog, question tool with TUI chrome, and pulse char false-positive prevention. No separate integration test is needed because detection is purely string-matching on pane content (no tmux interaction required).

---

## Wave 0 Requirements

All test creation is handled inline by TDD plans (RED phase creates tests before GREEN phase implements). No separate Wave 0 scaffolding needed.

- Plan 14-01 Task 1 (tdd="true"): Creates `internal/session/sandbox_env_test.go` with `TestCommandBuilders_NoTmuxSetEnv` in RED phase
- Plan 14-02 Task 1 (tdd="true"): Extends `internal/tmux/status_fixes_test.go` with question tool test cases in RED phase

*Existing infrastructure: `internal/session/testmain_test.go` and `internal/tmux/testmain_test.go` already provide profile isolation.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Docker sandbox session env propagation end-to-end | DET-01 | Requires running Docker sandbox with tmux | 1. Start sandbox session 2. Run `tmux show-environment -t <session> CLAUDE_SESSION_ID` 3. Verify UUID returned |
| OpenCode question tool visual transition | DET-02 | Requires running OpenCode with question tool active | 1. Start OpenCode session 2. Trigger question tool 3. Verify session transitions from green (running) to orange (waiting) |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved (aligned with plan structure)
