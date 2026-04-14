---
phase: 02-cgroup-isolation-default-req-1-fix
plan: 04
subsystem: session-persistence
tags: [go, tmux, fallback, error-recovery, tdd, execCommand-seam]
requirements_completed: [PERSIST-04, PERSIST-05]
requirements_partial: [PERSIST-06]
dependency_graph:
  requires:
    - "startCommandSpec (internal/tmux/tmux.go:814-837) — unchanged, provides the systemd-run --user --scope wrap this plan falls back FROM"
    - "GetLaunchInUserScope() default=true on Linux+systemd (Plan 02-02) — the reason the fallback is reachable at all on fresh-install hosts"
    - "TmuxSettings.LaunchInUserScope *bool + opt-out (Plan 02-02) — the fallback must NOT re-enable systemd-run for explicit opt-outs (TEST-02 regression gate)"
    - "logging.ForComponent(logging.CompStatus) — existing statusLog handle"
  provides:
    - "tmux.stripSystemdRunPrefix([]string) []string — pure helper, strips the systemd-run prefix to recover bare tmux args"
    - "tmux.execCommand = exec.Command — swappable seam, test-only override target"
    - "Single-retry fallback block in (*Session).Start() failure handler — tmux_systemd_run_fallback warning + direct retry + both-fail error wrapping"
  affects:
    - "internal/tmux/tmux.go (Start failure handler; two existing exec.Command sites re-routed through the seam)"
tech_stack:
  added: []
  patterns:
    - "Swappable exec.Command seam (var fn = exec.Command) for deterministic unit tests without mutating host PATH/systemd"
    - "Single-retry fallback with structured warning (never loop, never silent)"
    - "Defensive prefix-stripping helper that passes-through unexpected shapes rather than panicking"
    - "Merged RED+GREEN commit (same lefthook-go-vet precedent as Plans 02-01/02/03)"
key_files:
  created:
    - "internal/tmux/tmux_fallback_test.go (173 LOC, 4 tests + 3 helpers)"
    - ".planning/phases/02-cgroup-isolation-default-req-1-fix/02-04-SUMMARY.md (this file)"
  modified:
    - "internal/tmux/tmux.go (+44 net lines: +6 execCommand seam + comment, +20 stripSystemdRunPrefix helper + comment, +22 fallback block including warning+retry+both-fail wrap, −7 collapsed final-err branch; 2 exec.Command→execCommand substitutions)"
decisions:
  - "Single commit, not two (Task 1 RED + Task 2 GREEN merged). Reason: lefthook pre-commit runs `go vet ./...`, which rejects any RED-only state where the test references undefined symbols (`execCommand`, `stripSystemdRunPrefix`). Same precedent set by Plans 02-01 / 02-02 / 02-03. Bypassing with --no-verify is forbidden by user-global CLAUDE.md. TDD discipline preserved at design level — tests were authored first and dictate the exact pinned fallback contract."
  - "Used the execCommand-seam approach (plan's option A), not the broken-unit-name approach (plan's option B). Seam is hermetic (no host-state mutation), finishes in ~110ms, and the same seam pattern is already established in internal/session/userconfig.go for systemdAvailableForLog — keeps the codebase consistent."
  - "Cleanup uses `tmux kill-session -t <name>` (not `kill-server`). In this test the seam delegates tmux to the real exec.Command, which talks to the DEFAULT tmux socket — so `kill-server` without `-t` would kill all user tmux servers on the host (the 2025-12-10 incident pattern). `kill-session -t <s.Name>` only destroys the specific session we created. Names include the `agentdeck_` prefix + sanitize + unique 6-hex suffix from NewSession, so collisions are effectively zero."
  - "TEST-01 did NOT flip GREEN. The production fallback works correctly (Tests 3+4 prove it), but TEST-01's RED-gate has moved downstream: the test helper `startAgentDeckTmuxInUserScope` (internal/session/session_persistence_test.go:283-318) queries `systemctl --user show -p MainPID --value <unit>.scope` and gets an empty string back on this host because tmux daemonizes — the scope's initial process exits before systemd records a MainPID. The fix is in the TEST HELPER (pgrep-based PID discovery), not in production. The plan's `files_modified` frontmatter is strict (internal/tmux/* only); touching `session_persistence_test.go` is out of scope for Plan 04 and should land in a follow-up plan. Plan 04 explicitly allows this outcome per its output spec: `TEST-01 GREEN confirmation (or RED diagnosis with next-steps if it stays red)`."
metrics:
  duration_sec: 1200
  completed: "2026-04-14T14:10:00Z"
  commits: 1
  files_changed: 2
  tests_added: 4
  lines_added_production: 44
  lines_added_test: 173
---

# Phase 2 Plan 04: Graceful systemd-run Failure Fallback Summary

**One-liner:** Extended `(*Session).Start` in `internal/tmux/tmux.go` so a non-zero exit from the `systemd-run --user --scope` wrap now retries ONCE with the direct `tmux` launcher using the bare args, emits a `tmux_systemd_run_fallback` structured warning, and wraps both diagnostics if both paths fail — never blocking session creation on the primary path. Added a pure `stripSystemdRunPrefix` helper and a swappable `execCommand` seam, with four new tests (2 unit + 2 integration via the seam) pinning the contract.

## What Landed

### Production code

#### `internal/tmux/tmux.go` (+44 net LOC)

Three surgical additions + one inline edit:

**1. New swappable seam** (just below the existing `statusLog`/`respawnLog` var block):

```go
// execCommand is a swappable seam that defaults to exec.Command. Tests
// override it to inject failure into specific launcher names without
// mutating host PATH or systemd state. Production callers always read
// the default.
var execCommand = exec.Command
```

**2. New `stripSystemdRunPrefix` helper** (immediately after `startCommandSpec`):

```go
func stripSystemdRunPrefix(args []string) []string {
    if len(args) >= 7 && args[6] == "tmux" {
        return args[7:]
    }
    return args
}
```

Doc block pins the exact expected shape produced by `startCommandSpec` so a future refactor of the wrap can't silently desync.

**3. Extended failure handler in `(*Session).Start`** — the two `exec.Command(launcher, args...)` calls in the spawn region (one at line ~1351, one in the stale-socket-recovery retry at line ~1363) were re-routed through `execCommand(...)`. The `if err != nil { if launcher == "systemd-run" { return ... } }` final-return block was replaced with:

```go
if err != nil && launcher == "systemd-run" {
    statusLog.Warn("tmux_systemd_run_fallback",
        slog.String("session", s.Name),
        slog.String("error", err.Error()),
        slog.String("output", string(output)))
    directArgs := stripSystemdRunPrefix(args)
    retryOutput, retryErr := execCommand("tmux", directArgs...).CombinedOutput()
    if retryErr == nil {
        output = retryOutput
        err = nil
    } else {
        return fmt.Errorf("failed to create tmux session: systemd-run path: %w (output: %s); direct retry: %v (output: %s)",
            err, string(output), retryErr, string(retryOutput))
    }
}
if err != nil {
    return fmt.Errorf("failed to create tmux session: %w (output: %s)", err, string(output))
}
```

Semantics:
- **Primary path OK** → warning not emitted, retry not attempted (zero-cost on the success path).
- **systemd-run fails, direct tmux OK** → warning emitted, session created, `nil` returned to caller.
- **Both paths fail** → wrapped error containing `"systemd-run path:"` + `"direct retry:"` substrings so operators can grep logs and diagnose.
- **Explicit opt-out** → `launcher` is `"tmux"` (from `startCommandSpec` when `LaunchInUserScope=false`), the `launcher == "systemd-run"` guard short-circuits, fallback does NOT fire — TEST-02 regression gate stays GREEN.

### Test code

#### `internal/tmux/tmux_fallback_test.go` (new file, 173 LOC)

**Helpers (3):**

- `randomServerSuffix(t)` — 8 hex chars via crypto/rand (mirrors session_persistence_test.go:230-237).
- `captureStatusLog(t)` — swaps package-level `statusLog` with a JSON-handler buffer for the test duration; restores via `t.Cleanup`.
- `failOnLauncher(failBinary)` — returns a function matching `exec.Command`'s signature that returns `exec.Command("false")` (guaranteed-fail, no side effects) when argv[0] equals `failBinary`, and otherwise delegates to the real `exec.Command`.

**Tests (4):**

| # | Name | Kind | Assertion |
|---|------|------|-----------|
| 1 | `TestStripSystemdRunPrefix_RecoversTmuxArgs` | pure unit | Given the exact args shape produced by `startCommandSpec` (11 tokens), returns the last 4 bare tmux args. |
| 2 | `TestStripSystemdRunPrefix_PassesThroughUnexpectedShape` | pure unit | Too-short and `args[6] != "tmux"` cases both pass through unchanged (defensive). |
| 3 | `TestStartCommandSpec_FallsBackToDirect` | integration via seam | Overrides `execCommand` with `failOnLauncher("systemd-run")` → systemd-run path exits 1; direct retry delegates to real `exec.Command("tmux", …)` so a real tmux server is created. Asserts `s.Start("")` returns nil AND the buffer-captured statusLog contains `tmux_systemd_run_fallback`. Cleanup: `tmux kill-session -t s.Name`. |
| 4 | `TestStartCommandSpec_BothFailWrapsError` | integration via seam | Fails both `"systemd-run"` AND `"tmux"` via the seam. Asserts the returned error contains BOTH `"systemd-run path:"` AND `"direct retry:"` substrings. |

## Commits

| # | Hash | Message |
|---|------|---------|
| 1 | `3e62021` | `feat(02-04): fall back to direct tmux when systemd-run fails` |

Commit body ends with `Committed by Ashesh Goplani`. Zero Claude-attribution lines (verified: `git log --format=%B -1 | grep -ciE "co-authored-by:.*claude\|generated with claude code\|🤖"` → 0).

## Deviations from Plan

### [Rule 3 / Plan-precedent] RED+GREEN merged into a single commit

- **Why:** lefthook pre-commit hook runs `go vet ./...`, which rejects any RED-only state where `internal/tmux/tmux_fallback_test.go` references the undefined `execCommand` and `stripSystemdRunPrefix` symbols. Same wall hit by Plans 02-01, 02-02, 02-03. Bypassing with `--no-verify` is forbidden by user-global CLAUDE.md.
- **What was kept:** TDD discipline at design level. The four test cases were authored before the production function bodies and dictate their exact pinned contract (the warning event name, the two error-substring tokens, the expected args shape). The merged commit body explicitly documents the RED state that would have existed.
- **Impact:** None on coverage or contract pinning.

### [Plan-anticipated outcome] TEST-01 stays RED on THIS host with a NEW failure mode

The plan's output spec explicitly allowed this (`TEST-01 GREEN confirmation (or RED diagnosis with next-steps if it stays red)`), so this is a planned contingency rather than a deviation. Full diagnostic below under **Test Results**.

## Test Results

### go vet / go build

```
$ go vet ./...
(clean)

$ go build ./...
(clean)
```

### New fallback tests (4 cases, Plan 04 primary gate)

```
$ go test -run "TestStripSystemdRunPrefix_|TestStartCommandSpec_" ./internal/tmux/... -race -count=1 -v
=== RUN   TestStripSystemdRunPrefix_RecoversTmuxArgs
--- PASS: TestStripSystemdRunPrefix_RecoversTmuxArgs (0.00s)
=== RUN   TestStripSystemdRunPrefix_PassesThroughUnexpectedShape
--- PASS: TestStripSystemdRunPrefix_PassesThroughUnexpectedShape (0.00s)
=== RUN   TestStartCommandSpec_FallsBackToDirect
--- PASS: TestStartCommandSpec_FallsBackToDirect (0.10s)
=== RUN   TestStartCommandSpec_BothFailWrapsError
--- PASS: TestStartCommandSpec_BothFailWrapsError (0.01s)
PASS
ok  github.com/asheshgoplani/agent-deck/internal/tmux  1.174s
```

All four Plan 04 target tests GREEN.

### Full internal/tmux/... package (no regressions)

```
$ go test ./internal/tmux/... -race -count=1
ok  github.com/asheshgoplani/agent-deck/internal/tmux  12.389s
```

### Persistence suite (the CLAUDE.md mandate)

```
$ go test -run TestPersistence_ ./internal/session/... -race -count=1 -v
=== RUN   TestPersistence_LinuxDefaultIsUserScope
--- PASS: TestPersistence_LinuxDefaultIsUserScope (0.01s)
=== RUN   TestPersistence_MacOSDefaultIsDirect
    session_persistence_test.go:176: systemd-run available; TEST-04 only asserts non-systemd behavior — see TEST-03 for Linux+systemd default
--- SKIP: TestPersistence_MacOSDefaultIsDirect (0.00s)
=== RUN   TestPersistence_TmuxSurvivesLoginSessionRemoval
    session_persistence_test.go:354: startAgentDeckTmuxInUserScope: invalid MainPID "": strconv.Atoi: parsing "": invalid syntax
--- FAIL: TestPersistence_TmuxSurvivesLoginSessionRemoval (0.24s)
=== RUN   TestPersistence_TmuxDiesWithoutUserScope
    session_persistence_test.go:475: tmux pid=3966017 cgroup="0::/user.slice/user-1000.slice/user@1000.service/app.slice/fake-login-20655455.scope"
--- PASS: TestPersistence_TmuxDiesWithoutUserScope (0.36s)
=== RUN   TestPersistence_FreshSessionUsesSessionIDNotResume
--- PASS: TestPersistence_FreshSessionUsesSessionIDNotResume (0.02s)
=== RUN   TestPersistence_RestartResumesConversation
--- PASS: TestPersistence_RestartResumesConversation (0.94s)
=== RUN   TestPersistence_StartAfterSIGKILLResumesConversation
    TEST-06 RED (Phase 3 territory — REQ-2 resume dispatch)
--- FAIL: TestPersistence_StartAfterSIGKILLResumesConversation (0.45s)
=== RUN   TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion
    TEST-07 RED (Phase 3 territory — REQ-2 resume dispatch)
--- FAIL: TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion (0.22s)
=== RUN   TestPersistence_ExplicitOptOutHonoredOnLinux
    === RUN   .../empty_config_defaults_true
    === RUN   .../explicit_false_overrides_default
    === RUN   .../explicit_true_overrides
    === RUN   .../pointer_state_locked
--- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux (0.02s)
FAIL
FAIL  github.com/asheshgoplani/agent-deck/internal/session  2.310s
```

Status table — Plan 04 vs the post-Plan-02-03 baseline:

| Test | Before this plan | After this plan | Notes |
|------|------------------|-----------------|-------|
| TEST-01 TmuxSurvivesLoginSessionRemoval | RED (default-flip gate failing — but that was already green after Plan 02-02; RED reason on this host was the downstream helper crash) | RED (test-helper issue — see diagnostic below) | Plan 04 production contract met; remaining RED is test-helper robustness. |
| TEST-02 TmuxDiesWithoutUserScope | PASS (or SKIP depending on outer scope context) | PASS | Regression gate GREEN — fallback did not re-enable systemd-run for opt-outs. |
| TEST-03 LinuxDefaultIsUserScope | PASS (Plan 02-02) | PASS | Untouched. |
| TEST-04 MacOSDefaultIsDirect | SKIP | SKIP | systemd present → inverse always skips. |
| TEST-05 RestartResumesConversation | PASS | PASS | Untouched. |
| TEST-06 StartAfterSIGKILLResumesConversation | RED | RED | Phase 3 territory (REQ-2). |
| TEST-07 ClaudeSessionIDSurvivesHookSidecarDeletion | RED | RED | Phase 3 territory (REQ-2). |
| TEST-08 FreshSessionUsesSessionIDNotResume | PASS | PASS | Untouched. |
| TestPersistence_ExplicitOptOutHonoredOnLinux | PASS (4/4) | PASS (4/4) | Untouched. |

No test changed status from PASS → FAIL because of this plan.

### TEST-01 RED diagnostic (expected-to-follow-up)

**Failure line:**

```
session_persistence_test.go:354: startAgentDeckTmuxInUserScope: invalid MainPID "": strconv.Atoi: parsing "": invalid syntax
```

**Root cause (host-specific behavior, not production bug):**

`startAgentDeckTmuxInUserScope` (internal/session/session_persistence_test.go:283-318) launches tmux via `systemd-run --user --scope --quiet --collect --unit=<unit> tmux -L <server> new-session -d ...` and then reads the PID from:

```
systemctl --user show -p MainPID --value <unit>.scope
```

On this Linux+systemd host, that query returns an empty string because:

1. `tmux new-session -d` double-forks (daemonizes) — the tmux daemon reparents itself to the user systemd manager directly.
2. systemd-run's `--scope` transient unit tracks the cgroup membership, but `MainPID` is only populated when the scope's initially-tracked process (the shell that launched tmux) is still running.
3. Because tmux forked and the bash wrapper exited, systemd has no "initial process" to record as MainPID — the scope is active (the daemon is inside its cgroup), but MainPID is empty.

I reproduced this manually to confirm:

```
$ systemd-run --user --scope --quiet --collect --unit=probe \
    tmux -L probe new-session -d -s persist bash -c "exec sleep 30" &
$ sleep 1 && systemctl --user show -p MainPID --value probe.scope
                      # ← empty line, exit 0
$ systemctl --user is-active probe.scope
active                # ← scope is running, MainPID just wasn't recorded
$ systemctl --user status probe.scope | grep CGroup -A2
     CGroup: /user.slice/.../app.slice/probe.scope
             └─3969092 /usr/bin/tmux -L probe new-session -d ...
                      # ← tmux IS in the cgroup, just not tracked as MainPID
```

**Why Plan 04's production fallback does NOT fix this:**

The production fallback operates on the exit code of `systemd-run` itself. In the test helper, `systemd-run` succeeded (exit 0; the scope was created; tmux was spawned into it). The failure is purely in the HELPER's PID-discovery strategy. No production code change can rescue a test helper that's using the wrong strategy.

**Recommended follow-up (out of scope for Plan 04 — not in its `files_modified` whitelist):**

Replace the MainPID query with a `pgrep -f "tmux -L <serverName>"` walk bounded to the scope's cgroup. The cgroup's `cgroup.procs` file can be read directly:

```go
cgroupPath, _ := exec.Command("systemctl", "--user", "show",
    "-p", "ControlGroup", "--value", unit+".scope").Output()
procs, _ := os.ReadFile("/sys/fs/cgroup" + strings.TrimSpace(string(cgroupPath)) + "/cgroup.procs")
// parse PIDs, pick the tmux one via /proc/<pid>/comm
```

This should land in a separate test-infrastructure plan. Plan 04's production deliverable is complete and correct.

## Acceptance grep checks

```
$ grep -c "^func TestStripSystemdRunPrefix_" internal/tmux/tmux_fallback_test.go   # 2 ✓
$ grep -c "^func TestStartCommandSpec_" internal/tmux/tmux_fallback_test.go        # 2 ✓
$ grep -c "tmux_systemd_run_fallback" internal/tmux/tmux_fallback_test.go          # 4 ✓
$ grep -c "s\.Start(" internal/tmux/tmux_fallback_test.go                          # 2 ✓ (B3 pin)
$ grep -c "startUnderlyingProcess" internal/tmux/tmux_fallback_test.go             # 0 ✓ (B3 pin)
$ grep -c "NewSession(" internal/tmux/tmux_fallback_test.go                        # 2 ✓ (B3 pin)
$ grep -c "^var execCommand = exec.Command" internal/tmux/tmux.go                  # 1 ✓
$ grep -c "^func stripSystemdRunPrefix" internal/tmux/tmux.go                      # 1 ✓
$ grep -c "tmux_systemd_run_fallback" internal/tmux/tmux.go                        # 1 ✓
$ grep -c "systemd-run path:" internal/tmux/tmux.go                                # 1 ✓
$ grep -c "direct retry:" internal/tmux/tmux.go                                    # 1 ✓
$ grep -c "execCommand(launcher, args...)" internal/tmux/tmux.go                   # 2 ✓
```

All twelve acceptance-criteria greps return the expected counts.

## Out-of-scope file mandate

```
$ git diff --stat HEAD~1 HEAD -- internal/session/userconfig.go internal/session/instance.go internal/session/storage.go cmd/agent-deck/session_cmd.go
(empty — no out-of-scope mandate file touched)
```

`internal/tmux/tmux.go` is in the CLAUDE.md mandate path list but is the explicit primary subject of this plan; the full persistence suite was re-run after the commit and shows no PASS→FAIL regression caused by this plan.

Leaked test-server check:

```
$ tmux list-sessions 2>&1 | grep -c "agentdeck-test-"
0
```

No leaked test tmux servers after the suite.

## Requirements Closed

- **PERSIST-04** — graceful systemd-run failure fallback with structured warning. ✓ delivered. `tmux_systemd_run_fallback` slog.Warn event fires on the fallback path; both structured fields (`session`, `error`, `output`) are populated; TEST 3 captures and asserts on the event name.
- **PERSIST-05** — never block session creation when fallback is available. ✓ delivered. The `retryErr == nil` branch clears the outer `err`, letting `Start()` continue normally through the rest of the initialization. Tests 3 confirms `s.Start("")` returns nil.
- **PERSIST-06** (TEST-01 GREEN) — **partial**. The production-side contract is fully met: Tests 3+4 prove the fallback recovers correctly when systemd-run fails. TEST-01 remains RED due to a test-helper PID-discovery issue in `startAgentDeckTmuxInUserScope` (see TEST-01 RED diagnostic above). That helper is out of Plan 04's `files_modified` scope; it should land as a follow-up.

Note: REQUIREMENTS.md is owned by the orchestrator. Per the prompt directive (`Do NOT touch STATE.md or ROADMAP.md`), this plan does not toggle requirement status.

## Known Stubs

None. The fallback executes live retries, the warning is a live slog emit, and both helper and seam are production-quality code with no TODOs.

## Threat Flags

No new threat surface introduced beyond the plan's existing threat_model. The three registered mitigations are all in place:

- **T-02-04-01 (execCommand seam tampering):** `grep -c "^var execCommand = exec.Command" internal/tmux/tmux.go` returns exactly 1. No production callsite reassigns the variable.
- **T-02-04-02 (both-paths-fail diagnostics loss):** Error returned from the both-fail branch contains both `"systemd-run path:"` and `"direct retry:"` substrings, verified by `TestStartCommandSpec_BothFailWrapsError`.
- **T-02-04-03 (opt-out leak):** Fallback only fires `if launcher == "systemd-run"`, which only happens when `LaunchInUserScope=true`. TEST-02 stays GREEN.

## Self-Check: PASSED

- FOUND: `internal/tmux/tmux.go` (modified — `execCommand` seam, `stripSystemdRunPrefix`, fallback block, warning, both-fail wrap all present)
- FOUND: `internal/tmux/tmux_fallback_test.go` (new file, 4 tests, 3 helpers)
- FOUND: `.planning/phases/02-cgroup-isolation-default-req-1-fix/02-04-SUMMARY.md` (this file)
- FOUND commit `3e62021` (`feat(02-04): fall back to direct tmux when systemd-run fails`)
- Commit body ends with `Committed by Ashesh Goplani` (1/1)
- Zero `Co-Authored-By: Claude` / `Generated with Claude Code` / `🤖` markers in this plan's single commit
- `go vet ./...` clean, `go build ./...` clean
- Four new tests GREEN; `go test ./internal/tmux/... -race` passes end-to-end
- Persistence suite: TEST-02, TEST-03, TEST-05, TEST-08 GREEN (unchanged); TEST-01 RED with test-helper diagnostic captured; TEST-06/07 RED (Phase 3); no PASS→FAIL regression
- STATE.md and ROADMAP.md unchanged (the `.planning/STATE.md` modification visible in `git status` is pre-existing from before this session, not introduced by this plan)
