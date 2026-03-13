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
| **Framework** | Go testing + testify/assert, `go test -race` |
| **Config file** | none (`TestMain` enforces `AGENTDECK_PROFILE=_test`) |
| **Quick run command** | `go test -race -v ./internal/session/... ./internal/tmux/... -run 'TestOpencode\|TestBuildClaude\|TestBuildOpenCode\|TestBuildGemini\|TestBuildCodex'` |
| **Full suite command** | `go test -race -v ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race ./internal/tmux/... -run TestOpencode` and `go test -race ./internal/session/... -run TestBuild`
- **After every plan wave:** Run `go test -race -v ./internal/tmux/... ./internal/session/...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | DET-01 | unit | `go test ./internal/session/... -run TestBuildClaudeCommand_NoTmuxSetEnv -v` | ❌ W0 | ⬜ pending |
| 14-01-02 | 01 | 1 | DET-01 | unit | `go test ./internal/session/... -run TestBuildOpenCodeCommand_NoTmuxSetEnv -v` | ❌ W0 | ⬜ pending |
| 14-01-03 | 01 | 1 | DET-01 | unit | `go test ./internal/session/... -run TestBuildGeminiCommand_NoTmuxSetEnv -v` | ❌ W0 | ⬜ pending |
| 14-01-04 | 01 | 1 | DET-01 | unit | `go test ./internal/session/... -run TestBuildCodexCommand_NoTmuxSetEnv -v` | ❌ W0 | ⬜ pending |
| 14-01-05 | 01 | 1 | DET-01 | unit | `go test ./internal/session/... -run TestBuildClaudeResumeCommand_NoTmuxSetEnv -v` | ❌ W0 | ⬜ pending |
| 14-02-01 | 02 | 1 | DET-02 | unit | `go test ./internal/tmux/... -run TestOpencodeBusyGuard -v` | ✅ (extend) | ⬜ pending |
| 14-02-02 | 02 | 1 | DET-02 | unit | `go test ./internal/tmux/... -run TestDefaultRawPatterns_OpenCode -v` | ✅ (extend) | ⬜ pending |
| 14-02-03 | 02 | 1 | DET-02 | integration | `go test ./internal/tmux/... -run TestDetection_OpenCodeQuestionTool -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/session/instance_test.go` — add `TestBuildClaudeCommand_NoTmuxSetEnv`, `TestBuildOpenCodeCommand_NoTmuxSetEnv`, `TestBuildGeminiCommand_NoTmuxSetEnv`, `TestBuildCodexCommand_NoTmuxSetEnv`, `TestBuildClaudeResumeCommand_NoTmuxSetEnv`
- [ ] `internal/tmux/status_fixes_test.go` — extend with VALIDATION 8.0 section covering question-tool prompt cases
- [ ] `internal/tmux/status_fixes_test.go` — add `TestDetection_OpenCodeQuestionTool` integration test

*Existing infrastructure: `internal/session/testmain_test.go` and `internal/tmux/testmain_test.go` already provide profile isolation.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Docker sandbox session env propagation end-to-end | DET-01 | Requires running Docker sandbox with tmux | 1. Start sandbox session 2. Run `tmux show-environment -t <session> CLAUDE_SESSION_ID` 3. Verify UUID returned |
| OpenCode question tool visual transition | DET-02 | Requires running OpenCode with question tool active | 1. Start OpenCode session 2. Trigger question tool 3. Verify session transitions from green (running) to orange (waiting) |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
