---
phase: 13-auto-start-platform
verified: 2026-03-13T07:57:06Z
status: passed
score: 10/10 must-haves verified
re_verification: false
---

# Phase 13: Auto-Start & Platform Verification Report

**Phase Goal:** Users on WSL/Linux can run agent-deck session start from non-interactive contexts and tool processes receive a working PTY
**Verified:** 2026-03-13T07:57:06Z
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

Two success criteria were extracted from the ROADMAP:

1. Running `agent-deck session start` from a non-interactive shell on WSL/Linux starts the session without tool processes rejecting input due to a missing PTY.
2. After auto-starting and stopping a session on WSL/Linux, resuming it attaches to the correct tool conversation (identified by the tool conversation ID, not the agent-deck internal UUID).

Both are addressed by plans 13-01 (PLAT-01) and 13-02 (PLAT-02). Plan 13-03 was retired after its scope was fully absorbed into 13-01.

---

## Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | `tmux Start()` waits for the pane shell to be ready before calling `SendKeysAndEnter` | VERIFIED | `tmux.go:1179-1189`: `waitForPaneReady(s, paneReadyTimeout)` called inside `if command != "" && !startWithInitialProcess` block before send-keys |
| 2 | `isPaneShellReady` correctly identifies bash ($), zsh (%), fish (>), root (#) prompt endings and rejects non-prompt output | VERIFIED | `pane_ready.go:18-33`: implementation confirmed; 12 table-driven tests all pass |
| 3 | Pane-ready wait uses platform-aware timeouts (5s on WSL, 2s elsewhere) and is non-fatal on timeout | VERIFIED | `tmux.go:1180-1183`: `platform.IsWSL()` branch; timeout leads to `statusLog.Warn` and continues |
| 4 | Claude command strings contain no `uuidgen` shell invocation or `$(` subshell for UUID generation | VERIFIED | `grep uuidgen instance.go` returns only comments; `generateUUID()` called at line 488 and 4109 |
| 5 | `generateUUID()` produces valid lowercase UUID v4 using `crypto/rand` with no external binary dependency | VERIFIED | `instance.go:5001-5012`: implementation present; `TestGenerateUUID` and `TestGenerateUUID_Uniqueness` pass |
| 6 | Fork path also uses Go-generated UUID (no `uuidgen` in fork command) | VERIFIED | `instance.go:4109`: `forkUUID := generateUUID()`; `TestForkCommandNoUuidgen` passes |
| 7 | `SyncSessionIDsFromTmux` reads all four tool session IDs from tmux env into Instance fields | VERIFIED | `instance.go:2885-2907`: reads CLAUDE/GEMINI/OPENCODE/CODEX SESSION_IDs; 7 tests all pass |
| 8 | `SyncSessionIDsFromTmux` does not blank existing IDs when tmux env var is missing or empty | VERIFIED | Conditional: only updates `if id != ""`; `TestSyncSessionIDsFromTmux_NoOverwriteWithEmpty` passes |
| 9 | `handleSessionStop` calls `SyncSessionIDsFromTmux` before `Kill`, capturing IDs before tmux session is destroyed | VERIFIED | `session_cmd.go:272-279`: `inst.SyncSessionIDsFromTmux()` at line 276, `inst.Kill()` at line 279 |
| 10 | `saveSessionData` is called after kill, persisting the captured IDs to SQLite | VERIFIED | `session_cmd.go:285`: `saveSessionData(storage, instances)` follows the kill block |

**Score: 10/10 truths verified**

---

## Required Artifacts

### Plan 13-01 Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/tmux/pane_ready.go` | `waitForPaneReady` and `isPaneShellReady` functions | VERIFIED | 49 lines; both functions fully implemented |
| `internal/tmux/pane_ready_test.go` | Unit tests for prompt detection and real-tmux integration test | VERIFIED | 125 lines; `TestIsPaneShellReady` (12 cases), `TestWaitForPaneReady_Timeout`, `TestWaitForPaneReady_RealTmux` |
| `internal/session/instance.go` | `generateUUID()` function; command builders use Go-side UUID | VERIFIED | `generateUUID` at line 5001; called at lines 488 and 4109 |

### Plan 13-02 Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/session/instance.go` | `SyncSessionIDsFromTmux` method | VERIFIED | Defined at line 2885; reads all four tool env vars |
| `cmd/agent-deck/session_cmd.go` | `handleSessionStop` calls `SyncSessionIDsFromTmux` before `Kill` | VERIFIED | Line 276: `inst.SyncSessionIDsFromTmux()` before line 279: `inst.Kill()` |
| `internal/session/instance_platform_test.go` | 7 tests for `SyncSessionIDsFromTmux` and stop-saves-ID flow | VERIFIED | 235 lines; 7 test functions: NilTmuxSession, NonExistentSession, Claude, AllTools, NoOverwriteWithEmpty, OverwriteWithNew, StopSavesSessionID — all pass |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/tmux/tmux.go` | `internal/tmux/pane_ready.go` | `waitForPaneReady()` call before `SendKeysAndEnter` in `Start()` | WIRED | `tmux.go:1184`: `if err := waitForPaneReady(s, paneReadyTimeout)` |
| `internal/tmux/pane_ready.go` | `internal/tmux/tmux.go` | `s.CapturePaneFresh()` for polling pane output | WIRED | `pane_ready.go:41`: `output, err := s.CapturePaneFresh()` |
| `internal/session/instance.go` | `crypto/rand` | `generateUUID()` uses `rand.Read` | WIRED | `instance.go:5002-5003`: `b := make([]byte, 16); rand.Read(b)` |
| `internal/session/instance.go:buildClaudeCommand` | `internal/session/instance.go:generateUUID` | `sessionUUID := generateUUID()` before command string construction | WIRED | `instance.go:488`: `sessionUUID := generateUUID()` |
| `internal/session/instance.go:BuildForkCommand` | `internal/session/instance.go:generateUUID` | `forkUUID := generateUUID()` | WIRED | `instance.go:4109`: `forkUUID := generateUUID()` |
| `cmd/agent-deck/session_cmd.go` | `internal/session/instance.go` | `inst.SyncSessionIDsFromTmux()` before `inst.Kill()` in `handleSessionStop` | WIRED | `session_cmd.go:276`: `inst.SyncSessionIDsFromTmux()` |
| `internal/session/instance.go` | `internal/tmux/tmux.go` | `i.tmuxSession.GetEnvironment()` reads tool session IDs from tmux env | WIRED | `instance.go:2890,2897,2901,2905`: `i.tmuxSession.GetEnvironment(...)` for all four tool vars |
| `cmd/agent-deck/session_cmd.go` | `internal/session/storage.go` | `saveSessionData()` called after `SyncSessionIDsFromTmux()` and `Kill()` | WIRED | `session_cmd.go:285`: `saveSessionData(storage, instances)` |

---

## Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| PLAT-01 | 13-01-PLAN.md, 13-03-PLAN.md | Auto-start from non-interactive contexts on WSL/Linux; tool processes receive a PTY | SATISFIED | Pane-ready detection in `pane_ready.go`; `waitForPaneReady` wired into `Start()`; `generateUUID()` eliminates `uuidgen` dependency; fish-compat double-wrap no longer triggered for non-message Claude commands |
| PLAT-02 | 13-02-PLAN.md | Resume after auto-start uses correct tool conversation ID | SATISFIED | `SyncSessionIDsFromTmux` wired into stop path; all four tool IDs captured before `Kill`; `TestStopSavesSessionID` confirms data flow end-to-end |

**Note on Plan 13-03:** `13-03-PLAN.md` was deleted after checkers determined its scope (Go-side UUID generation) was fully absorbed into 13-01. No orphaned requirements: PLAT-01 is covered by 13-01 which successfully delivers all UUID-related must-haves that 13-03 would have addressed. The REQUIREMENTS.md marks both PLAT-01 and PLAT-02 as complete.

---

## Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| None found | — | — | — |

Scanned: `internal/tmux/pane_ready.go`, `internal/tmux/pane_ready_test.go`, `internal/session/instance_platform_test.go`, relevant sections of `internal/session/instance.go`, `cmd/agent-deck/session_cmd.go`. No TODO/FIXME/placeholder/stub patterns found in phase deliverables.

**Note:** The `return nil` in `pane_ready.go:43` is a valid early success return (prompt detected), not a stub.

---

## Test Results

All targeted tests pass under the race detector:

```
# internal/tmux: pane-ready detection
TestIsPaneShellReady          PASS (12 sub-cases)
TestWaitForPaneReady_Timeout  SKIP (pane ready in 1ms — correct skip behavior)
TestWaitForPaneReady_RealTmux PASS

# internal/session: UUID generation and command validation
TestGenerateUUID              PASS
TestGenerateUUID_Uniqueness   PASS
TestBuildClaudeCommandNoUuidgen PASS
TestBuildClaudeCommandHasSessionID PASS
TestForkCommandNoUuidgen      PASS

# internal/session: SyncSessionIDsFromTmux
TestSyncSessionIDsFromTmux_Claude             PASS
TestSyncSessionIDsFromTmux_AllTools           PASS
TestSyncSessionIDsFromTmux_NoOverwriteWithEmpty PASS
TestSyncSessionIDsFromTmux_OverwriteWithNew   PASS
TestSyncSessionIDsFromTmux_NilTmuxSession     PASS
TestSyncSessionIDsFromTmux_NonExistentSession PASS
TestStopSavesSessionID                        PASS

# go vet: all modified packages
go vet ./internal/tmux/... ./internal/session/... ./cmd/agent-deck/...
(no output — clean)
```

---

## Human Verification Required

### 1. End-to-end non-interactive WSL/Linux auto-start

**Test:** On a WSL2 instance, run `agent-deck session start --title "test" --tool claude` from a non-interactive shell (e.g., via `bash -c "agent-deck session start ..."` or a cron-style invocation). Observe whether the Claude process starts without "PTY not allocated" errors.
**Expected:** Session starts cleanly; Claude command is sent to the pane after the shell prompt is detected.
**Why human:** Cannot simulate a non-interactive WSL2 context in automated tests. The pane-ready polling is the fix mechanism and passes in CI, but end-to-end PTY behavior on a real WSL2 host requires manual verification.

### 2. Resume uses correct conversation ID after slow-start stop

**Test:** On WSL2, start a session, let PostStartSync time out (or manually kill the sync window), then stop the session and resume it. Verify the resume command references the correct `CLAUDE_SESSION_ID` (not a blank or internal UUID).
**Expected:** After stop, `agent-deck session resume` or the equivalent attaches to the same Claude conversation.
**Why human:** The `TestStopSavesSessionID` test confirms the sync logic at unit level, but the full resume flow involves storage reads and the Claude `--resume` flag which cannot be verified without a real running Claude instance.

---

## Summary

Phase 13 goal is achieved. All code artifacts are present, substantive, and correctly wired:

- **PLAT-01** (non-interactive WSL/Linux auto-start): `waitForPaneReady` gives the pane shell time to initialize before commands are sent (2s normal, 5s on WSL). `generateUUID()` eliminates the `uuidgen` binary dependency that fails silently on minimal WSL/Linux installs. The fish-compat double `bash -c` wrap is no longer triggered for non-message Claude sessions.

- **PLAT-02** (resume uses correct conversation ID): `SyncSessionIDsFromTmux` snapshots all four tool session IDs from the tmux environment immediately before `Kill`, ensuring IDs are persisted to SQLite even when `PostStartSync` timed out during start. The stop path ordering (sync → kill → save) is correctly implemented and tested end-to-end.

Two items are flagged for human verification because they require a real WSL2 host and running Claude process. All automated checks pass.

---

_Verified: 2026-03-13T07:57:06Z_
_Verifier: Claude (gsd-verifier)_
