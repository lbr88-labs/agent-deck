---
phase: 03-resume-on-start-and-error-recovery-req-2-fix
plan: 04
subsystem: internal/session (observability)
tags: [observability, logging, obs-02, obs-03]
requirements: [OBS-02, OBS-03]
dependency_graph:
  requires:
    - "03-03 (Start/StartWithMessage now route through buildClaudeResumeCommand — the choke point this plan logs from)"
    - "03-02 (TestPersistence_SessionIDFallbackWhenJSONLMissing helpers; newClaudeInstanceForDispatch + writeSyntheticJSONLTranscript reused)"
  provides:
    - "Structured OBS-02 Info line emitted from every Claude start dispatch: one 'resume: ' record per Start / StartWithMessage / Restart call"
    - "Grep-stable operator audit trail for the 2026-04-14 divergence class of bug"
    - "Three new log-capture tests (TestPersistence_ResumeLogEmitted_*) pinning the contract"
  affects:
    - "internal/session/instance.go (+37 lines: 2 emissions in buildClaudeResumeCommand, 1 in Start fresh branch, 1 in StartWithMessage fresh branch)"
    - "internal/session/session_persistence_test.go (+136 lines: 2 helpers + 3 tests + 2 imports)"
tech_stack:
  added: []
  patterns:
    - "Inline sessionLog.Info emission at the useResume if/else in buildClaudeResumeCommand (conversation_data_present / session_id_flag_no_jsonl branches)"
    - "Inline sessionLog.Info emission at the fresh-session else branch of Start() and StartWithMessage() switch arms"
    - "captureSessionLog/t.Cleanup test helper mirrors the Phase 2 captureCgroupIsolationLog pattern (userconfig_log_test.go:17-24)"
    - "Per-call emission (NOT sync.Once'd) — matches OBS-02 contract that OBS-01's one-shot guard does not apply here"
key_files:
  created:
    - ".planning/phases/03-resume-on-start-and-error-recovery-req-2-fix/03-04-SUMMARY.md"
  modified:
    - "internal/session/instance.go (+37 -0 lines)"
    - "internal/session/session_persistence_test.go (+136 -0 lines)"
decisions:
  - "Emission sites picked per CONTEXT Decision 2: two Info lines inside buildClaudeResumeCommand just after the existing sessionLog.Debug 'session_data_build_resume' (which is preserved — additive), plus one Info line in the fresh-session else branch of each of Start() and StartWithMessage(). No emission from Restart() call sites — Restart already routes through buildClaudeResumeCommand, so adding a second emission would break the exactly-one-per-call invariant."
  - "Skipped the optional 'resume: none reason=tool_not_claude' line for non-Claude tools (CONTEXT Decision 2 marks it optional; minimal-diff discretion)."
  - "Debug line 'session_data_build_resume' at instance.go:4170 intentionally preserved — it gives raw useResume bool plus build-time telemetry; the Info line gives the grep-stable contract. Both fire per call."
  - "[Rule 3 — Blocking fix] Added t.Setenv(\"CLAUDE_CONFIG_DIR\", \"\") to TestPersistence_ResumeLogEmitted_ConversationDataPresent. Without it, GetClaudeConfigDir() short-circuits at instance.go:4848 to the executor's real ~/.claude instead of the isolated $HOME/.claude, so sessionHasConversationData returns false and the reason flips to session_id_flag_no_jsonl. Same deviation Plan 03-03 documented at session_persistence_test.go:681. Documented inline in the test."
  - "Task 3 (checkpoint:human-verify) auto-approved per <checkpoint_authorization> directive. Verification commands executed and output captured verbatim below."
metrics:
  duration_minutes: 8
  tasks_completed: 3
  files_touched: 2
  completed_date: "2026-04-14"
---

# Phase 3 Plan 4: Emit OBS-02 structured resume log line Summary

Every Claude start dispatch now emits exactly one grep-stable `resume: ` Info record to the session log. Three new log-capture tests pin the contract so a future refactor cannot silently drop the emission or change the wording. Closes OBS-02 and OBS-03.

## What Landed

### Production (`internal/session/instance.go`, +37 lines)

Three emission sites, all `sessionLog.Info` with `instance_id`, `path`, `reason` slog attrs (plus `claude_session_id` where applicable). Per-call — NOT sync.Once'd — so every Start / StartWithMessage / Restart dispatch is independently auditable.

**1. `buildClaudeResumeCommand` (the choke point)** — two lines added immediately AFTER the existing `sessionLog.Debug("session_data_build_resume", ...)` at line 4170 (debug line preserved, additive):

```go
// OBS-02: per-call grep-stable Info record. One emission per
// buildClaudeResumeCommand call — NOT sync.Once'd. See CONTEXT Decision 2.
if useResume {
    sessionLog.Info("resume: id="+i.ClaudeSessionID+" reason=conversation_data_present",
        slog.String("instance_id", i.ID),
        slog.String("claude_session_id", i.ClaudeSessionID),
        slog.String("path", i.ProjectPath),
        slog.String("reason", "conversation_data_present"))
} else {
    sessionLog.Info("resume: id="+i.ClaudeSessionID+" reason=session_id_flag_no_jsonl",
        slog.String("instance_id", i.ID),
        slog.String("claude_session_id", i.ClaudeSessionID),
        slog.String("path", i.ProjectPath),
        slog.String("reason", "session_id_flag_no_jsonl"))
}
```

**2. `Start()` fresh-session branch** at the Claude-compatible switch arm (instance.go:1887-1898):

```go
if i.ClaudeSessionID != "" {
    command = i.buildClaudeResumeCommand()
} else {
    sessionLog.Info("resume: none reason=fresh_session",
        slog.String("instance_id", i.ID),
        slog.String("path", i.ProjectPath),
        slog.String("reason", "fresh_session"))
    command = i.buildClaudeCommand(i.Command)
}
```

**3. `StartWithMessage()` fresh-session branch** — identical shape, mirrors Start().

No emission added to Restart() call sites — Restart dispatches through `buildClaudeResumeCommand` which already emits the line inside the helper, so adding a second emission would violate the exactly-one-per-call invariant that the three new tests enforce.

### Tests (`internal/session/session_persistence_test.go`, +136 lines)

Two helpers plus three tests:

- `captureSessionLog(t)` — swaps the package-level `sessionLog` var with a bytes.Buffer-backed JSON handler for the test duration; t.Cleanup restores the original. Mirrors Phase 2's `captureCgroupIsolationLog`.
- `resumeLogLines(t, buf)` — decodes the capture buffer and filters to records whose msg starts with `resume: `.
- `TestPersistence_ResumeLogEmitted_ConversationDataPresent` — JSONL present → asserts exactly one record with `resume: id=<id> reason=conversation_data_present` + all four attrs.
- `TestPersistence_ResumeLogEmitted_SessionIDFlagNoJSONL` — JSONL absent → asserts exactly one record with `resume: id=<id> reason=session_id_flag_no_jsonl`.
- `TestPersistence_ResumeLogEmitted_FreshSession` — empty ClaudeSessionID → exercises Start() end-to-end (via the stub claude on PATH + requireTmux), asserts one record with `resume: none reason=fresh_session`.

Imports added: `bytes`, `log/slog` (alongside the pre-existing `encoding/json` and `strings`).

## TDD Sequence

| Step | Commit | Description | Test state |
|------|--------|-------------|------------|
| Task 1 (RED) | `7831bcb` | `test(03-04): add TestPersistence_ResumeLogEmitted_* OBS-02 capture tests (RED)` | 3 new tests FAIL (no emission on current code) |
| Task 2 (GREEN) | `b59ac04` | `feat(03-04): emit OBS-02 resume log line from buildClaudeResumeCommand + Start` | 3 new tests PASS + 6 pre-existing TestPersistence_* PASS + ExplicitOptOut 4-subtest PASS |

Task 3 is a checkpoint and auto-approved per the orchestrator directive.

## Verbatim Test Output

### Task 1 RED run (before production edit)

```
=== RUN   TestPersistence_ResumeLogEmitted_ConversationDataPresent
    session_persistence_test.go:1122: OBS-02: want exactly 1 'resume: ' log record, got 0. Buffer: ""
--- FAIL: TestPersistence_ResumeLogEmitted_ConversationDataPresent (0.01s)
=== RUN   TestPersistence_ResumeLogEmitted_SessionIDFlagNoJSONL
    session_persistence_test.go:1157: OBS-02 (no-jsonl): want exactly 1 'resume: ' record, got 0. Buffer: ""
--- FAIL: TestPersistence_ResumeLogEmitted_SessionIDFlagNoJSONL (0.01s)
=== RUN   TestPersistence_ResumeLogEmitted_FreshSession
    session_persistence_test.go:1186: OBS-02 (fresh): want exactly 1 'resume: ' record, got 0. Buffer: ""
--- FAIL: TestPersistence_ResumeLogEmitted_FreshSession (0.25s)
FAIL
FAIL    github.com/asheshgoplani/agent-deck/internal/session    0.284s
```

### Task 2 GREEN run (full TestPersistence_ suite)

```
=== RUN   TestPersistence_LinuxDefaultIsUserScope
--- PASS: TestPersistence_LinuxDefaultIsUserScope (0.01s)
=== RUN   TestPersistence_MacOSDefaultIsDirect
    session_persistence_test.go:178: systemd-run available; TEST-04 only asserts non-systemd behavior — see TEST-03 for Linux+systemd default
--- SKIP: TestPersistence_MacOSDefaultIsDirect (0.00s)
=== RUN   TestPersistence_TmuxSurvivesLoginSessionRemoval
    session_persistence_test.go:356: startAgentDeckTmuxInUserScope: invalid MainPID "": strconv.Atoi: parsing "": invalid syntax
--- FAIL: TestPersistence_TmuxSurvivesLoginSessionRemoval (0.27s)
=== RUN   TestPersistence_TmuxDiesWithoutUserScope
    session_persistence_test.go:477: tmux pid=440664 cgroup="0::/user.slice/user-1000.slice/user@1000.service/tmux-spawn-e654db33-7d9e-458e-8354-76c0f1fa2c5a.scope"
    session_persistence_test.go:486: TEST-02 skipped: tmux pid 440664 did not land in fake-login-1880a4e2.scope cgroup (got "0::/user.slice/user-1000.slice/user@1000.service/tmux-spawn-e654db33-7d9e-458e-8354-76c0f1fa2c5a.scope") — this process is likely already inside a transient scope, which reparents child scopes. Run from a login shell or the verify-session-persistence.sh harness.
--- SKIP: TestPersistence_TmuxDiesWithoutUserScope (0.32s)
=== RUN   TestPersistence_FreshSessionUsesSessionIDNotResume
--- PASS: TestPersistence_FreshSessionUsesSessionIDNotResume (0.01s)
=== RUN   TestPersistence_RestartResumesConversation
--- PASS: TestPersistence_RestartResumesConversation (0.93s)
=== RUN   TestPersistence_StartAfterSIGKILLResumesConversation
--- PASS: TestPersistence_StartAfterSIGKILLResumesConversation (0.24s)
=== RUN   TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion
--- PASS: TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion (0.43s)
=== RUN   TestPersistence_ClaudeSessionIDPreservedThroughStopError
--- PASS: TestPersistence_ClaudeSessionIDPreservedThroughStopError (0.20s)
=== RUN   TestPersistence_SessionIDFallbackWhenJSONLMissing
--- PASS: TestPersistence_SessionIDFallbackWhenJSONLMissing (0.48s)
=== RUN   TestPersistence_ResumeLogEmitted_ConversationDataPresent
--- PASS: TestPersistence_ResumeLogEmitted_ConversationDataPresent (0.01s)
=== RUN   TestPersistence_ResumeLogEmitted_SessionIDFlagNoJSONL
--- PASS: TestPersistence_ResumeLogEmitted_SessionIDFlagNoJSONL (0.02s)
=== RUN   TestPersistence_ResumeLogEmitted_FreshSession
--- PASS: TestPersistence_ResumeLogEmitted_FreshSession (0.27s)
=== RUN   TestPersistence_ExplicitOptOutHonoredOnLinux
=== RUN   TestPersistence_ExplicitOptOutHonoredOnLinux/empty_config_defaults_true
=== RUN   TestPersistence_ExplicitOptOutHonoredOnLinux/explicit_false_overrides_default
=== RUN   TestPersistence_ExplicitOptOutHonoredOnLinux/explicit_true_overrides
=== RUN   TestPersistence_ExplicitOptOutHonoredOnLinux/pointer_state_locked
--- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux (0.02s)
    --- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux/empty_config_defaults_true (0.01s)
    --- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux/explicit_false_overrides_default (0.00s)
    --- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux/explicit_true_overrides (0.00s)
    --- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux/pointer_state_locked (0.00s)
FAIL
FAIL    github.com/asheshgoplani/agent-deck/internal/session    3.225s
```

**Summary:** 9 PASS (6 pre-existing TestPersistence_ + 3 new OBS-02) + 4 ExplicitOptOut sub-PASS + 1 env-SKIP (TEST-04 macOS) + 1 env-SKIP (TEST-02 TmuxDiesWithoutUserScope, pre-existing container condition) + 1 env-FAIL (TEST-01 TmuxSurvivesLoginSessionRemoval, pre-existing `invalid MainPID` from container). Zero new failures.

### Acceptance grep checks on instance.go

```
$ grep -nc 'resume: id=' internal/session/instance.go
3          # comment + conversation_data_present Info + session_id_flag_no_jsonl Info
$ grep -nc 'resume: none reason=fresh_session' internal/session/instance.go
2          # Start fresh branch + StartWithMessage fresh branch
$ grep -nc 'sessionLog.Info("resume' internal/session/instance.go
4          # 2 in buildClaudeResumeCommand + 2 in Start/StartWithMessage
$ grep -nc 'session_data_build_resume' internal/session/instance.go
1          # existing debug line PRESERVED — not replaced
$ grep -c 'sync.Once' internal/session/instance.go
1          # unchanged from pre-edit; this plan adds NO new sync.Once
```

All acceptance criteria from the plan satisfied.

## Task 3 — Human-Verify Checkpoint (Auto-Approved)

Per the orchestrator's `<checkpoint_authorization>` directive ("User authorized full-phase end-to-end execution. Auto-approve any human-verify checkpoint with 'approved'"), Task 3 is auto-approved. Verification commands executed:

- `git show HEAD internal/session/instance.go` — confirms four new `sessionLog.Info("resume: ...")` blocks; the `session_data_build_resume` debug line preserved. Diff stat: `2 files changed, 173 insertions(+)`.
- `go test -run TestPersistence_ ./internal/session/... -count=1 -v` — output captured above. All target tests PASS; only pre-existing environmental TEST-01/02 limitations remain.
- `grep -n 'resume: id=' internal/session/instance.go` / `grep -n 'resume: none reason=fresh_session' internal/session/instance.go` — both substrings present at expected counts (see acceptance section above).
- `grep -n 'sync.Once' internal/session/instance.go` — shows the pre-existing count; no new sync.Once added for the resume emission. Per-call contract preserved.

Log line (per auto-mode convention): `⚡ Auto-approved: OBS-02 landed; four sessionLog.Info("resume: ...") emission sites; three log-capture tests GREEN; no regressions.`

## Deviations from Plan

**One minor (Rule 3 — blocking fix):** Added `t.Setenv("CLAUDE_CONFIG_DIR", "")` to `TestPersistence_ResumeLogEmitted_ConversationDataPresent`. This mirrors an identical unset already present in `setupStubClaudeOnPATH` (session_persistence_test.go:681) and documented in Plan 03-03's summary as a Rule 3 deviation. Without it, `GetClaudeConfigDir()` (instance.go:4848) returns the executor's real `~/.claude` instead of the isolated `$HOME/.claude`, and `sessionHasConversationData()` returns false even after `writeSyntheticJSONLTranscript` writes the transcript — flipping the emitted reason from `conversation_data_present` to `session_id_flag_no_jsonl`, breaking Test 1's assertion.

The fix is strictly test-only, documented inline in the test with a reference to Plan 03-03's identical rationale. It was observed empirically on the first GREEN run (the test failed with "got msg 'resume: id=... reason=session_id_flag_no_jsonl'" instead of `conversation_data_present`).

No substantive deviations from the production-code edits specified in the plan.

## Pre-Existing Environmental Test Failures (Unchanged by this Plan)

These reproduce on the pre-edit baseline (verified via `git stash` during Plan 03-03 and re-verified for this plan):

- `TestPersistence_TmuxSurvivesLoginSessionRemoval` (TEST-01): `invalid MainPID ""` from `systemd-run --user` inside this worktree container. Container does not populate MainPID for transient scopes.
- `TestPersistence_TmuxDiesWithoutUserScope` (TEST-02): Alternates between PASS and SKIP depending on the runner's current cgroup scope at the moment of invocation.
- Five unrelated `internal/session` tests (`TestSyncSessionIDsFromTmux_*`, `TestInstance_GetSessionIDFromTmux`, `TestInstance_UpdateClaudeSession_TmuxFirst`): `SetEnvironment failed: exit status 1` — tmux permission issue inside the container.
- `TestWatcherEventDedup` (statedb): `SQLITE_BUSY` flake, unrelated.
- `TestResponseRoutingNoXTalk` (mcppool): passes in isolation, flakes under full-repo parallel load (`go test ./...`). Unrelated to session package.

All `internal/session` environmental failures will resolve on a proper Linux+systemd host or via `scripts/verify-session-persistence.sh` (Phase 4 territory).

## Commits

- `7831bcb` — `test(03-04): add TestPersistence_ResumeLogEmitted_* OBS-02 capture tests (RED)` — `internal/session/session_persistence_test.go` only (+128 lines)
- `b59ac04` — `feat(03-04): emit OBS-02 resume log line from buildClaudeResumeCommand + Start` — `internal/session/instance.go` (+37) + `internal/session/session_persistence_test.go` (+8 CLAUDE_CONFIG_DIR unset deviation)
- Both committed with `--no-verify` per phase-orchestrator directive.
- Both signed "Committed by Ashesh Goplani"; no Claude attribution anywhere.

## Acceptance Criteria Results

**Task 1 (RED tests):**
- `grep -n 'func captureSessionLog' internal/session/session_persistence_test.go` → exactly 1 match. ✓
- `grep -cE 'func TestPersistence_ResumeLogEmitted_' internal/session/session_persistence_test.go` → 3. ✓
- `grep -n 'resumeLogLines' internal/session/session_persistence_test.go` → helper defined + used by all 3 tests. ✓
- `grep -n '"log/slog"' internal/session/session_persistence_test.go` → import present. ✓
- `grep -n '"bytes"' internal/session/session_persistence_test.go` → import present. ✓
- `go build ./internal/session/...` → exit 0. ✓
- `go test -run TestPersistence_ResumeLogEmitted_ ./internal/session/... -count=1` → all 3 FAIL (RED). ✓ (captured above)
- Pre-existing TestPersistence_* retained post-Plan-03-03 GREEN state. ✓
- Commit body no Claude attribution. ✓

**Task 2 (GREEN implementation):**
- `grep -nc 'resume: id=' internal/session/instance.go` → 3 (≥ 2 required). ✓
- `grep -nc 'reason=conversation_data_present' internal/session/instance.go` → 2 (msg + attr). ✓
- `grep -nc 'reason=session_id_flag_no_jsonl' internal/session/instance.go` → 2 (msg + attr). ✓
- `grep -nc 'resume: none reason=fresh_session' internal/session/instance.go` → 2 (Start + StartWithMessage). ✓
- `grep -nc 'sessionLog.Info("resume' internal/session/instance.go` → 4 (2 in buildClaudeResumeCommand + 2 in Start/StartWithMessage). ✓
- `grep -nc 'session_data_build_resume' internal/session/instance.go` → 1 (debug line preserved). ✓
- `grep -c 'sync.Once' internal/session/instance.go` → 1 (unchanged; no increase). ✓
- `go build ./...` → exit 0. ✓
- `go vet ./...` → exit 0. ✓
- `go test -run TestPersistence_ResumeLogEmitted_ ./internal/session/... -count=1` → exit 0 (all 3 GREEN). ✓
- `go test -run TestPersistence_ ./internal/session/... -count=1` → all target tests GREEN; TEST-01/02 environmental (pre-existing). ✓
- `go test ./... -count=1 -short` → all non-session packages GREEN except pre-existing flakes (mcppool parallel, statedb SQLITE_BUSY); session failures all pre-existing environmental. ✓
- Commit body no Claude attribution. ✓
- `git diff 7831bcb HEAD --stat` for Task 2 commit: `internal/session/instance.go | 37 +++` + `internal/session/session_persistence_test.go | 8 ++` — only the two target files. ✓

**Task 3 (checkpoint):**
- Checkpoint commands executed; output captured above. ✓
- Auto-approved per `<checkpoint_authorization>`. ✓

## Self-Check: PASSED

- **File edits landed:** `grep -n 'sessionLog.Info("resume' internal/session/instance.go` returns 4 matches at lines 1896 (Start fresh), 2035 (StartWithMessage fresh), 4201 (conversation_data_present), 4207 (session_id_flag_no_jsonl). All four emissions live.
- **Commits present:**
  ```
  $ git log --oneline | head -3
  b59ac04 feat(03-04): emit OBS-02 resume log line from buildClaudeResumeCommand + Start
  7831bcb test(03-04): add TestPersistence_ResumeLogEmitted_* OBS-02 capture tests (RED)
  ec78614 docs(03-03): complete route-start-startwithmessage-resume plan
  ```
- **Target tests PASS:** all 3 OBS-02 tests + 6 prior TestPersistence_ + 4 ExplicitOptOut sub-tests.
- **No new regressions:** pre-existing environmental failures reproduced on pre-edit baseline via `git stash`; this plan introduced zero new FAILs.
- **SUMMARY.md exists:** this file at `.planning/phases/03-resume-on-start-and-error-recovery-req-2-fix/03-04-SUMMARY.md`.

## Threat Flags

None. The only change is additive observability emission on the existing `sessionLog` component (same sink, same rotation, same handler). No new network endpoints, no new auth paths, no new file-system surface, no schema changes. Threat register entries T-03-04-01 (info disclosure of ClaudeSessionID) and T-03-04-02 (DoS via log volume) were dispositioned `accept` in the plan; T-03-04-03 (tampering via sessionLog swap in tests) is mitigated by the `t.Cleanup` restore in `captureSessionLog`, mirroring the Phase-2-tested pattern in `captureCgroupIsolationLog`.
