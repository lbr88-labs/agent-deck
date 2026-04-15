# Plan 05-02 Summary — GREEN PERSIST-11..13 + TEST-09 fix

**Phase:** 05-custom-command-jsonl-resume
**Plan:** 02
**TDD stage:** GREEN
**Commit:** `d1590d6` on `fix/session-persistence`
**Date:** 2026-04-15

## New symbols

```go
// internal/session/claude.go
func discoverLatestClaudeJSONL(projectPath string) (string, bool)

// internal/session/instance.go
func (i *Instance) ensureClaudeSessionIDFromDisk()
```

## Insertion points

- `claude.go` — `discoverLatestClaudeJSONL` added immediately after `findActiveSessionIDExcluding` (after line 401 of pre-change file; lives between `findActiveSessionIDExcluding` and `getProjectSettingsPath`).
- `instance.go` — `ensureClaudeSessionIDFromDisk` added immediately before `Start()` (inserted at the pre-change line 1872, just before the `// Start starts the session in tmux` comment).
- `instance.go:1882 (Start)` — prelude `i.ensureClaudeSessionIDFromDisk()` added as the first line of the `case IsClaudeCompatible(i.Tool):` arm, BEFORE the existing `if i.ClaudeSessionID != ""` / `else` branches.
- `instance.go:2019 (StartWithMessage)` — identical prelude at the matching dispatch site.

Nothing else changed. Verified via `grep -c`:

- `findActiveSessionID` / `findActiveSessionIDExcluding` unchanged (count = 2, both still present).
- `buildClaudeResumeCommand` body unchanged.
- `Restart()` body unchanged.
- `buildClaudeCommand` body unchanged.
- `i.ensureClaudeSessionIDFromDisk()` called exactly twice in `instance.go` (Start + StartWithMessage).
- Neither `discoverLatestClaudeJSONL` nor `ensureClaudeSessionIDFromDisk` references `i.Command` (PERSIST-11 no-branch guarantee).

## Test results

```
go test -run TestPersistence_ ./internal/session/... -race -count=1 -timeout=300s -v

--- PASS: TestPersistence_ClaudeSessionIDPreservedThroughStopError (0.24s)
--- PASS: TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion (0.57s)
--- PASS: TestPersistence_CustomCommandResumesFromLatestJSONL (0.85s)
--- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux (0.02s)
--- PASS: TestPersistence_FreshSessionUsesSessionIDNotResume (0.01s)
--- PASS: TestPersistence_LinuxDefaultIsUserScope (0.01s)
--- PASS: TestPersistence_RestartResumesConversation (1.16s)
--- PASS: TestPersistence_ResumeLogEmitted_ConversationDataPresent (0.02s)
--- PASS: TestPersistence_ResumeLogEmitted_FreshSession (0.23s)
--- PASS: TestPersistence_ResumeLogEmitted_SessionIDFlagNoJSONL (0.02s)
--- PASS: TestPersistence_SessionIDFallbackWhenJSONLMissing (0.23s)
--- PASS: TestPersistence_StartAfterSIGKILLResumesConversation (0.21s)
--- PASS: TestPersistence_TmuxDiesWithoutUserScope (0.21s)
--- SKIP: TestPersistence_MacOSDefaultIsDirect (0.00s)
--- FAIL: TestPersistence_TmuxSurvivesLoginSessionRemoval (0.28s)
```

TEST-09 GREEN. 13 PASS, 1 SKIP (macOS-only), 1 FAIL (**pre-existing environmental flake**, signed off in Phase 2 UAT: `invalid MainPID ""` from `startAgentDeckTmuxInUserScope` helper, NOT a REQ-1 regression). Verified this failure exists on HEAD pre-change via `git stash`.

Adjacent package tests (`TestSyncSessionIDsFromTmux_*`, `TestInstance_GetSessionIDFromTmux`, `TestInstance_UpdateClaudeSession_TmuxFirst`) also fail identically on HEAD pre-change — same root cause (tmux `SetEnvironment` returning exit status 1 in this worktree environment). Not introduced by Phase 5.

## verify-session-persistence.sh

```
OVERALL: PASS
  [1] pre-existing shared tmux daemon in login scope — SKIP (environmental, unchanged)
  [2] tmux pid 1752166 survived login-session teardown (cgroup isolation works) — PASS
  [3] restart spawned claude with --resume or --session-id — PASS
  [4] fresh session uses --session-id without --resume — PASS
```

Not regressed by Phase 5. The script exercises populated-ID paths; Phase 5 only changes empty-ID dispatch.

## Commit

```
d1590d6 feat(05-02): discover + resume latest JSONL for empty-ID Claude starts (PERSIST-11..13, TEST-09 GREEN)
```

Trailer `Committed by Ashesh Goplani`. No push.

## Deviations

None from `<action>`. Implementation followed the exact code blocks specified in the plan (helper body, method body, prelude insertion at both dispatch sites). Single atomic commit covering both Task 1 + Task 2 (executor's choice per plan's `<action>` section).
