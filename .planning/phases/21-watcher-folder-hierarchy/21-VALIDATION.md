---
phase: 21
slug: watcher-folder-hierarchy
status: approved
nyquist_compliant: true
wave_0_complete: false
created: 2026-04-16
approved: 2026-04-16
---

# Phase 21 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Populated from `21-RESEARCH.md` §Validation Architecture. Finalized during planning.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` stdlib (no third-party runner needed) |
| **Config file** | None — `go test` conventions |
| **Quick run command** | `GOTOOLCHAIN=go1.24.0 go test ./internal/watcher/ -run TestLayout -race -count=1 -timeout 30s` |
| **Full suite command** | `GOTOOLCHAIN=go1.24.0 go test ./internal/watcher/... -race -count=1 -timeout 120s` |
| **CLI suite command** | `GOTOOLCHAIN=go1.24.0 go test ./cmd/agent-deck/... -run "Watcher" -race -count=1 -timeout 120s` |
| **Estimated runtime** | Quick ~2s · Full ~60–90s · CLI ~30s |

---

## Sampling Rate

- **After every task commit:** Run the quick command (`-run TestLayout`).
- **After every plan wave:** Run the full watcher suite + CLI suite.
- **Before `/gsd-verify-work`:** Full suite green under `-race`.
- **Max feedback latency:** 90 seconds (full suite).

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 21-01-A1 | 01 | 1 | REQ-WF-6 | — | N/A (test file only) | unit | `go test ./internal/watcher/ -run TestLayout_FreshInstallCreatesLayout -race` | ❌ Task A | ⬜ pending |
| 21-01-A2 | 01 | 1 | REQ-WF-6 | T-21-SL (symlink) | Use `os.Lstat` to detect pre-existing symlink; refuse traversal | unit | `go test ./internal/watcher/ -run TestLayout_LegacyMigrationAtomic -race` | ❌ Task A | ⬜ pending |
| 21-01-A3 | 01 | 1 | REQ-WF-6 | — | Relative symlink target — survives `$HOME` moves | unit | `go test ./internal/watcher/ -run TestLayout_SymlinkResolves -race` | ❌ Task A | ⬜ pending |
| 21-01-A4 | 01 | 1 | REQ-WF-6 | — | Atomic write-temp-rename (`state.json.tmp → state.json`) | unit | `go test ./internal/watcher/ -run TestLayout_StateRoundtrip -race` | ❌ Task A | ⬜ pending |
| 21-01-A5 | 01 | 1 | REQ-WF-6 | — | `O_APPEND` < PIPE_BUF; summary bounded to 400B | unit | `go test ./internal/watcher/ -run TestLayout_EventLogAppendAtomic -race` | ❌ Task A | ⬜ pending |
| 21-01-A6 | 01 | 1 | REQ-WF-6 | — | No in-process cache of state — always disk-read | unit | `go test ./internal/watcher/ -run TestLayout_HotReloadSafe -race` | ❌ Task A | ⬜ pending |
| 21-01-A7 | 01 | 1 | REQ-WF-6 | — | Engine wiring: 3 events → 3 log lines + 3 state updates | integration | `go test ./internal/watcher/ -run TestLayout_Integration_ThreeEvents -race` | ❌ Task A | ⬜ pending |
| 21-01-B | 01 | 1 | REQ-WF-6 | T-21-SL, T-21-PI (path injection) | `name` validated `^[a-zA-Z0-9][a-zA-Z0-9._-]*$` | unit | `go test ./internal/watcher/ -run TestLayout -race` (all green) | ❌ Task B | ⬜ pending |
| 21-01-C | 01 | 1 | REQ-WF-6 | — | Templates read-only scaffolds | manual | `ls assets/watcher-templates/` (3 files present) | ❌ Task C | ⬜ pending |
| 21-01-D | 01 | 2 | REQ-WF-6 | — | JSON schema stable; `health_status` from existing classifier | unit | `go test ./cmd/agent-deck/ -run TestWatcherList_JSON_ExposesStateFields -race` | ❌ Task D | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/watcher/layout_test.go` — new test file (7 tests); created in Task A.
- [ ] `internal/watcher/layout.go` — stub with exported symbols only, so test file compiles RED in Task A; filled in Task B.
- [ ] `internal/watcher/state.go` — struct + stub helpers in Task A, implementation in Task B.
- [ ] `internal/watcher/event_log.go` — stub in Task A, implementation in Task B.
- [ ] `cmd/agent-deck/watcher_cmd_test.go` — append `TestWatcherList_JSON_ExposesStateFields` in Task D.

*Framework install: none — Go stdlib `testing` already used everywhere.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Migration on a real populated `~/.agent-deck/watchers/` | REQ-WF-6 acceptance #2 | `os.Rename` cross-FS edge cases vary per user; atomic behavior only fully verifiable on the real box | Pre-merge dry-run on dev machine: build binary, back up `~/.agent-deck/`, start engine, verify `watcher/` exists + `watchers` is a relative symlink + all meta.json/state.json preserved + one migration log line emitted. |
| Template scaffolding tone/content | REQ-WF-6 | Templates are prose; quality judged by reviewer | `cat assets/watcher-templates/*.md` — reviewer confirms tone matches conductor templates and guidance is useful. |

---

## Validation Gaps & Risks

- **Migration collision path** (both `watchers/` and `watcher/` exist as real dirs): covered via sub-test of `TestLayout_LegacyMigrationAtomic`. Mandated behavior is "skip + log warning" — the sub-test asserts the warning log line is emitted and neither directory is modified.
- **Concurrent `AppendEventLog` writers:** engine is single-writer today, so POSIX atomic append holds. Research recommends a `-race`-clean test spawning two goroutines at `AppendEventLog(name, ...)` to catch any future refactor that breaks the single-writer assumption. Planner decides whether to include as an 8th unit test or a sub-test of A5.
- **Legacy `ClientsPath` fallback:** no code-level fallback implemented — the migration symlink handles it transparently. Covered by `TestLayout_SymlinkResolves`.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter (flip after checker passes)

**Approval:** approved 2026-04-16 (plan-checker VERIFICATION PASSED on iteration 2/3)
