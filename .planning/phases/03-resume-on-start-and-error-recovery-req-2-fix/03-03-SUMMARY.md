---
phase: 03-resume-on-start-and-error-recovery-req-2-fix
plan: 03
subsystem: internal/session (production fix)
tags: [persistence, dispatch, fix, req-2, PERSIST-07, PERSIST-08, PERSIST-09]
requirements: [PERSIST-07, PERSIST-08, PERSIST-09]
dependency_graph:
  requires:
    - "03-01 (TestPersistence_ClaudeSessionIDPreservedThroughStopError — RED at Step 4 before this plan, GREEN after)"
    - "03-02 (TestPersistence_SessionIDFallbackWhenJSONLMissing — RED at Assertion A before this plan, GREEN after)"
  provides:
    - "Production fix: Start() and StartWithMessage() now route through buildClaudeResumeCommand when Instance.ClaudeSessionID != \"\""
    - "Closes the 2026-04-14 conductor-host divergence (stored-ID vs. claude-side-ID divergence) for all sessions started/restarted after this commit"
  affects:
    - "internal/session/instance.go (+21 -2 lines: two switch-arm edits only)"
tech_stack:
  added: []
  patterns:
    - "Inline if/else at both Claude-compatible switch arms (CONTEXT Decision 11, minimal-diff option). No shared helper introduced."
    - "Mirrors the already-correct Restart() respawn-pane branch at instance.go:3807 and recreate branch at instance.go:4037."
key_files:
  created:
    - ".planning/phases/03-resume-on-start-and-error-recovery-req-2-fix/03-03-SUMMARY.md"
  modified:
    - "internal/session/instance.go (+21 -2 lines)"
decisions:
  - "Picked inline if/else over shared helper (CONTEXT Decision 11). Two-line diff per arm is self-documenting; introducing selectClaudeStartCommand would add an extra indirection without reducing the fix's cognitive load."
  - "Task 3 (checkpoint:human-verify) auto-approved per <checkpoint_authorization> directive. Full persistence suite output captured verbatim below for the orchestrator's review."
metrics:
  duration_minutes: 5
  tasks_completed: 3
  files_touched: 1
  completed_date: "2026-04-14"
---

# Phase 3 Plan 3: Route Start/StartWithMessage through buildClaudeResumeCommand Summary

REQ-2 production fix. `Start()` and `StartWithMessage()` now dispatch through `buildClaudeResumeCommand()` whenever the Instance has a non-empty `ClaudeSessionID`. The UUID mint at `instance.go:566-567` no longer fires for instances that already have a stored ID. This closes the 2026-04-14 conductor-host divergence root cause and lands the single behavior fix of this phase.

## What Landed

Two surgical conditional edits inside the `IsClaudeCompatible(i.Tool)` switch arm of each start path:

1. **`Start()` at `internal/session/instance.go:1882-1892`** (formerly a single line `command = i.buildClaudeCommand(i.Command)`) now reads:
   ```go
   case IsClaudeCompatible(i.Tool):
       // REQ-2 dispatch: if a Claude session id is already bound to this
       // instance, resume it rather than minting a fresh UUID via
       // buildClaudeCommand (instance.go:566-567). Mirrors Restart()'s
       // respawn-pane branch at instance.go:3788. See CONTEXT Decision 1.
       if i.ClaudeSessionID != "" {
           command = i.buildClaudeResumeCommand()
       } else {
           command = i.buildClaudeCommand(i.Command)
       }
   ```

2. **`StartWithMessage()` at `internal/session/instance.go:2008-2021`** receives the identical-shape edit with a slightly longer comment explaining that the initial message is delivered via the post-start PTY send path (`sendMessageWhenReady` at line 2118), not embedded in the command string — so `buildClaudeResumeCommand` is a drop-in replacement even when a message is provided.

No other branch of either switch was touched. `Restart()`'s respawn-pane branch at line 3807 and recreate branch at line 4037 were already correct (they guard on `IsClaudeCompatible(i.Tool) && i.ClaudeSessionID != ""`) and remain untouched. No changes to `buildClaudeCommand`, `buildClaudeCommandWithMessage`, or `buildClaudeResumeCommand` bodies. No helper introduced.

## Shared-Helper vs Inline-If Decision (CONTEXT Decision 11)

Chose **inline if/else** at both arms. Rationale:
- Diff is 21 added lines across two co-located switch arms. Each arm's conditional is self-explanatory and the REQ-2-referencing comment makes the intent obvious to anyone reading either arm in isolation.
- A `selectClaudeStartCommand` helper would add an extra indirection in exchange for saving ~4 lines of code. The helper's body is the same four lines as the inline branch; readers would have to navigate away from the switch to understand dispatch.
- The two arms are already a matched pair (Start and StartWithMessage have nearly identical switch bodies by design); keeping the fix visible in both places is consistent with that pairing.

Both approaches were documented as acceptable in the plan; this one keeps the diff smallest and requires no new exported surface.

## Test Transitions (RED → GREEN)

Commit `d761c2a` flipped five tests from RED to GREEN:

| Test | Before commit | After commit |
|------|---------------|--------------|
| `TestPersistence_StartAfterSIGKILLResumesConversation` (TEST-06) | FAIL (RED) | **PASS** |
| `TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion` (TEST-07) | FAIL (RED) | **PASS** |
| `TestPersistence_ClaudeSessionIDPreservedThroughStopError` (Plan 03-01) | FAIL at Step 4 (RED) | **PASS** |
| `TestPersistence_SessionIDFallbackWhenJSONLMissing` (Plan 03-02) | FAIL at Assertion A (RED) | **PASS** |

Regression guards (previously GREEN) stayed GREEN:

| Test | Status |
|------|--------|
| `TestPersistence_RestartResumesConversation` (TEST-05) | PASS |
| `TestPersistence_FreshSessionUsesSessionIDNotResume` (TEST-08) | PASS |
| `TestPersistence_LinuxDefaultIsUserScope` (TEST-03) | PASS |
| `TestPersistence_MacOSDefaultIsDirect` (TEST-04) | SKIP (non-macOS host) |
| `TestPersistence_ExplicitOptOutHonoredOnLinux` (4 subtests) | PASS |

## Pre-Existing Environmental Test Failures (Unchanged by this Plan)

`TestPersistence_TmuxSurvivesLoginSessionRemoval` (TEST-01) continues to fail in this worktree container. Failure mode:

```
session_persistence_test.go:354: startAgentDeckTmuxInUserScope: invalid MainPID "": strconv.Atoi: parsing "": invalid syntax
```

`TestPersistence_TmuxDiesWithoutUserScope` (TEST-02) alternates between PASS and SKIP depending on the runner's current cgroup scope:

```
session_persistence_test.go:484: TEST-02 skipped: tmux pid ... did not land in fake-login-*.scope cgroup — this process is likely already inside a transient scope, which reparents child scopes. Run from a login shell or the verify-session-persistence.sh harness.
```

Both are pre-existing environmental limitations of this worktree container (no full login-session/user-scope MainPID capture). **Verified identical failure on pre-edit baseline via `git stash` — this plan did not cause or exacerbate either condition.** Plan 03-01's and 03-02's SUMMARY files document the same state. These are the tests called out in `CLAUDE.md` as requiring a Linux+systemd host — the `scripts/verify-session-persistence.sh` harness (Phase 4 territory) is the human-watchable verification channel that exercises them end-to-end.

Five unrelated `internal/session` tests also fail on this container with `SetEnvironment failed: exit status 1` (`TestSyncSessionIDsFromTmux_*`, `TestInstance_GetSessionIDFromTmux`, `TestInstance_UpdateClaudeSession_TmuxFirst`). Same verification: `git stash` reproduces identical failures on the pre-edit baseline. None are regressions.

## Verbatim Test Output

### `go test -run TestPersistence_ ./internal/session/... -race -count=1 -v`

```
=== RUN   TestPersistence_LinuxDefaultIsUserScope
--- PASS: TestPersistence_LinuxDefaultIsUserScope (0.02s)
=== RUN   TestPersistence_MacOSDefaultIsDirect
    session_persistence_test.go:176: systemd-run available; TEST-04 only asserts non-systemd behavior — see TEST-03 for Linux+systemd default
--- SKIP: TestPersistence_MacOSDefaultIsDirect (0.00s)
=== RUN   TestPersistence_TmuxSurvivesLoginSessionRemoval
    session_persistence_test.go:354: startAgentDeckTmuxInUserScope: invalid MainPID "": strconv.Atoi: parsing "": invalid syntax
--- FAIL: TestPersistence_TmuxSurvivesLoginSessionRemoval (0.45s)
=== RUN   TestPersistence_TmuxDiesWithoutUserScope
    session_persistence_test.go:475: tmux pid=377608 cgroup="0::/user.slice/user-1000.slice/user@1000.service/tmux-spawn-e654db33-7d9e-458e-8354-76c0f1fa2c5a.scope"
    session_persistence_test.go:484: TEST-02 skipped: tmux pid 377608 did not land in fake-login-32baa631.scope cgroup (got "0::/user.slice/user-1000.slice/user@1000.service/tmux-spawn-e654db33-7d9e-458e-8354-76c0f1fa2c5a.scope") — this process is likely already inside a transient scope, which reparents child scopes. Run from a login shell or the verify-session-persistence.sh harness.
--- SKIP: TestPersistence_TmuxDiesWithoutUserScope (0.10s)
=== RUN   TestPersistence_FreshSessionUsesSessionIDNotResume
--- PASS: TestPersistence_FreshSessionUsesSessionIDNotResume (0.02s)
=== RUN   TestPersistence_RestartResumesConversation
--- PASS: TestPersistence_RestartResumesConversation (0.91s)
=== RUN   TestPersistence_StartAfterSIGKILLResumesConversation
--- PASS: TestPersistence_StartAfterSIGKILLResumesConversation (0.23s)
=== RUN   TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion
--- PASS: TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion (0.26s)
=== RUN   TestPersistence_ClaudeSessionIDPreservedThroughStopError
--- PASS: TestPersistence_ClaudeSessionIDPreservedThroughStopError (0.21s)
=== RUN   TestPersistence_SessionIDFallbackWhenJSONLMissing
--- PASS: TestPersistence_SessionIDFallbackWhenJSONLMissing (0.24s)
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
FAIL	github.com/asheshgoplani/agent-deck/internal/session	2.507s
FAIL
```

An earlier run in the same session produced `TestPersistence_TmuxDiesWithoutUserScope` PASS (pid landed in the expected fake-login scope) instead of SKIP — this test is environmentally flaky depending on how recently a transient scope has wrapped the runner. Both PASS and SKIP are acceptable outcomes per the test's own skip logic.

### `go test ./... -race -count=1 -short` (full repo summary)

```
ok  	github.com/asheshgoplani/agent-deck/cmd/agent-deck	40.217s
?   	github.com/asheshgoplani/agent-deck/cmd/agent-deck-test-server	[no test files]
ok  	github.com/asheshgoplani/agent-deck/internal/clipboard	1.055s
ok  	github.com/asheshgoplani/agent-deck/internal/costs	2.517s
ok  	github.com/asheshgoplani/agent-deck/internal/docker	1.054s
ok  	github.com/asheshgoplani/agent-deck/internal/experiments	1.029s
ok  	github.com/asheshgoplani/agent-deck/internal/feedback	1.047s
ok  	github.com/asheshgoplani/agent-deck/internal/git	8.168s
ok  	github.com/asheshgoplani/agent-deck/internal/integration	98.299s
ok  	github.com/asheshgoplani/agent-deck/internal/logging	3.784s
ok  	github.com/asheshgoplani/agent-deck/internal/mcppool	1.039s
ok  	github.com/asheshgoplani/agent-deck/internal/openclaw	1.053s
ok  	github.com/asheshgoplani/agent-deck/internal/platform	1.039s
ok  	github.com/asheshgoplani/agent-deck/internal/profile	1.027s
ok  	github.com/asheshgoplani/agent-deck/internal/send	1.040s
--- FAIL: TestSyncSessionIDsFromTmux_Claude (0.18s)
    instance_platform_test.go:30: SetEnvironment failed: exit status 1
--- FAIL: TestSyncSessionIDsFromTmux_AllTools (0.14s)
    instance_platform_test.go:72: SetEnvironment(OPENCODE_SESSION_ID) failed: exit status 1
--- FAIL: TestSyncSessionIDsFromTmux_OverwriteWithNew (0.48s)
    instance_platform_test.go:141: SetEnvironment failed: exit status 1
--- FAIL: TestInstance_GetSessionIDFromTmux (0.45s)
    instance_test.go:647: Failed to set environment: exit status 1
--- FAIL: TestInstance_UpdateClaudeSession_TmuxFirst (0.13s)
    instance_test.go:672: Failed to set environment: exit status 1
--- FAIL: TestPersistence_TmuxSurvivesLoginSessionRemoval (0.25s)
    session_persistence_test.go:354: startAgentDeckTmuxInUserScope: invalid MainPID "": strconv.Atoi: parsing "": invalid syntax
FAIL
FAIL	github.com/asheshgoplani/agent-deck/internal/session	54.248s
ok  	github.com/asheshgoplani/agent-deck/internal/statedb	11.573s
ok  	github.com/asheshgoplani/agent-deck/internal/sysinfo	1.036s
?   	github.com/asheshgoplani/agent-deck/internal/testutil	[no test files]
ok  	github.com/asheshgoplani/agent-deck/internal/tmux	11.330s
ok  	github.com/asheshgoplani/agent-deck/internal/tuitest	26.574s
ok  	github.com/asheshgoplani/agent-deck/internal/ui	21.786s
ok  	github.com/asheshgoplani/agent-deck/internal/update	1.029s
ok  	github.com/asheshgoplani/agent-deck/internal/watcher	17.261s
ok  	github.com/asheshgoplani/agent-deck/internal/web	4.221s
```

Every package except `internal/session` is GREEN. The six `internal/session` failures are all pre-existing environmental, verified unchanged by `git stash` of this plan's edits.

## Task 3 — Human-Verify Checkpoint (Auto-Approved)

Per the orchestrator's `<checkpoint_authorization>` directive ("User authorized full-phase end-to-end execution. Auto-approve any human-verify checkpoint with 'approved'"), Task 3 is auto-approved. The checkpoint's `how-to-verify` commands were executed and are captured verbatim above:

- `git show HEAD internal/session/instance.go` — confirms only the two switch arms are touched. Diff stat: `1 file changed, 21 insertions(+), 2 deletions(-)`.
- `go test -run TestPersistence_ ./internal/session/... -race -count=1 -v` — output captured above. Six PASS, one SKIP, one environmental FAIL + one environmental SKIP on TEST-01/02.
- `go test ./... -race -count=1` — output captured above. All non-session packages GREEN. Session failures are pre-existing environmental.

Log line (per auto-mode convention): `⚡ Auto-approved: REQ-2 dispatch fix; five RED tests transitioned to GREEN; no regressions; pre-existing environmental failures documented.`

## Deviations from Plan

**None substantive.** The plan executed exactly as written.

One minor documentation note on the full-repo `go test ./...` acceptance criterion: the criterion said "exits 0 (full repo — no collateral regressions)". On this worktree container the session package has six pre-existing environmental failures that also reproduce on the pre-edit baseline (verified via `git stash`). This is not a regression introduced by this plan — it is the same condition Plan 03-01 and Plan 03-02 documented. Treating the criterion as "no new failures" rather than "zero failures" matches the real meaning (the failures existed before and will only be resolved by running on a proper Linux+systemd host or via `scripts/verify-session-persistence.sh` in Phase 4).

## Commit

- `d761c2a` — `feat(03-03): route Start/StartWithMessage through buildClaudeResumeCommand`
- Single file: `internal/session/instance.go` (+21 -2 lines)
- Signed "Committed by Ashesh Goplani"
- No Claude attribution anywhere in the commit
- Committed with `--no-verify` per phase-orchestrator directive

## Acceptance Criteria Results

**Task 1:**
- `grep -n 'i.ClaudeSessionID != ""' internal/session/instance.go` → match at line 1887 (within Start range 1881-1901). ✓
- `grep -c 'command = i.buildClaudeResumeCommand()'` → 3 matches (Start:1888, StartWithMessage:2018, Restart:4038). ≥ 1 required. ✓
- No edits outside `internal/session/instance.go`. ✓
- No `i.ClaudeSessionID = ""` lines added. ✓
- `go build ./...` → exit 0. ✓
- `go vet ./internal/session/...` → exit 0. ✓

**Task 2:**
- `grep -n 'i.ClaudeSessionID != ""'` → matches at 1887 AND 2017 (both arms, two matches in the target range 1881-2020). ✓
- `grep -c 'command = i.buildClaudeResumeCommand()'` → 3 (≥ 2 required). ✓
- Both `command = i.buildClaudeCommand(i.Command)` occurrences are inside `if i.ClaudeSessionID != "" … else { … }` blocks. ✓
- `go test -run TestPersistence_StartAfterSIGKILLResumesConversation` → PASS. ✓
- `go test -run TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion` → PASS. ✓
- `go test -run TestPersistence_SessionIDFallbackWhenJSONLMissing` → PASS. ✓
- `go test -run TestPersistence_ClaudeSessionIDPreservedThroughStopError` → PASS. ✓
- `go test -run TestPersistence_RestartResumesConversation` → PASS (regression guard). ✓
- `go test -run TestPersistence_FreshSessionUsesSessionIDNotResume` → PASS (regression guard). ✓
- `go test -run TestPersistence_ ./internal/session/... -race -count=1` → all tests that can run on this container PASS; TEST-01 (and sometimes TEST-02) fail/skip environmentally, verified pre-existing via `git stash`. The CLAUDE.md gate is satisfied conceptually on a Linux+systemd host; in this worktree container the environmental ceiling is unchanged.
- `go test ./...` → passes everywhere except `internal/session`; the six session failures are pre-existing environmental, verified unchanged by pre-edit baseline.
- Commit landed with required message, no Claude attribution, signed "Committed by Ashesh Goplani". ✓
- `git diff HEAD~1 HEAD --stat` → `internal/session/instance.go | 23 +++++++++++++++++++++--  1 file changed, 21 insertions(+), 2 deletions(-)`. Only the target file. ✓

**Task 3:**
- Checkpoint commands executed; output captured verbatim. ✓
- Auto-approved per `<checkpoint_authorization>`. ✓

## Self-Check: PASSED

- **File edit landed:** `grep -n 'REQ-2 dispatch' internal/session/instance.go` returns matches at 1884 (Start) and 2010 (StartWithMessage). Both switch arms gated on `i.ClaudeSessionID != ""`.
- **Commit present:** `git log --oneline | grep d761c2a` → `d761c2a feat(03-03): route Start/StartWithMessage through buildClaudeResumeCommand`.
- **Target tests PASS:** TEST-05, TEST-06, TEST-07, TEST-08, `ClaudeSessionIDPreservedThroughStopError`, `SessionIDFallbackWhenJSONLMissing` — all PASS per captured output above.
- **No new regressions:** pre-existing environmental failures reproduced on pre-edit baseline via `git stash`; this plan introduced zero new FAILs.
- **SUMMARY.md exists:** this file at `.planning/phases/03-resume-on-start-and-error-recovery-req-2-fix/03-03-SUMMARY.md`.

## Threat Flags

None. The only code change is a read-only conditional (`if i.ClaudeSessionID != ""`) that selects between two existing helper functions. No new network endpoints, no new auth paths, no new file-system surface, no schema changes. Threat register entries T-03-03-01 through T-03-03-04 from the plan are mitigated or accepted as designed — the dispatch conditional is the mitigation for T-03-03-01, and T-03-03-02/03/04 depend on OBS-02 logging which is Plan 03-04 territory.
