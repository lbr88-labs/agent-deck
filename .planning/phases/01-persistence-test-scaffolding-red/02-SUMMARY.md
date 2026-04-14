---
phase: 01-persistence-test-scaffolding-red
plan: 02
subsystem: testing
tags: [go, tmux, systemd, persistence, tdd-red, cgroup-v2, regression-tests]

# Dependency graph
requires:
  - phase: 01-persistence-test-scaffolding-red
    plan: 01
    provides: session_persistence_test.go skeleton + helpers (uniqueTmuxServerName, requireSystemdRun, isolatedHomeDir, writeStubClaudeBinary) + TEST-03/TEST-04
provides:
  - internal/session/session_persistence_test.go appended with TEST-01 + TEST-02 (4 of the 8 mandated tests now landed)
  - pidAlive helper (zombie-aware via /proc/<pid>/status State: Z/X)
  - randomHex8 helper (shared unique-suffix generator)
  - startFakeLoginScope helper (throwaway systemd user scope simulating an SSH login session)
  - startAgentDeckTmuxInUserScope helper (tmux under its own agentdeck-tmux-<name>.scope, mirroring the REQ-1 fix)
  - startTmuxInsideFakeLogin helper (tmux as a grandchild of a fake-login scope, mirroring the pre-fix LaunchInUserScope=false path)
  - pidCgroup helper (reads /proc/<pid>/cgroup, used to detect nested-scope edge case)
  - TEST-01 TestPersistence_TmuxSurvivesLoginSessionRemoval — FAILS RED on current v1.5.1 with "TEST-01 RED: GetLaunchInUserScope() default is false"
  - TEST-02 TestPersistence_TmuxDiesWithoutUserScope — PASSES on Linux+systemd (inverse pin), SKIPS on nested-scope executors and non-systemd hosts
affects: 03-persistence-test-scaffolding (appends TEST-06, TEST-07, TEST-08), 02-cgroup-default-req1-fix (TEST-01 turns green when default flips), 04-verification-and-ci (wires full suite into CI + verify-session-persistence.sh)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "cgroup v2 atomic kill via /sys/fs/cgroup/.../cgroup.kill for race-free scope teardown simulation (systemctl stop releases the cgroup without killing members; systemctl kill races with concurrent forks)"
    - "Zombie-aware pidAlive (syscall.Kill(pid, 0) returns nil for zombies — we filter via /proc/<pid>/status State: Z/X to treat reaped-but-unreaped processes as dead)"
    - "Nested-scope detection: if this test process is itself inside a transient scope (e.g., an agent-deck tmux-spawn-*.scope), systemd reparents child scopes' tracked processes into the parent cgroup — breaks the scope-teardown simulation. Detect via cgroup path match and skip with diagnostic pointing at verify-session-persistence.sh."
    - "Real-binary tests continue (no mocking of tmux/systemd/claude per CLAUDE.md mandate)"

key-files:
  created: []
  modified:
    - internal/session/session_persistence_test.go  # +343 lines across 2 commits (imports added: strconv, strings, syscall, time)

key-decisions:
  - "TEST-01 uses a two-stage design: first a cheap RED-state gate (GetLaunchInUserScope() default check) that fails immediately on v1.5.1 with a precise diagnostic telling Phase 2 what to fix, THEN a full simulated-teardown path that only runs once the default is true. This means the RED path creates zero tmux servers and zero systemd scopes on v1.5.1 — no cleanup burden for the current failing run."
  - "TEST-02 uses cgroup v2's cgroup.kill primitive rather than systemctl stop or systemctl kill. Rationale: (1) scope `stop` releases the cgroup without killing tasks (systemd semantics for scopes — they track, they don't lifecycle); (2) systemctl kill races with concurrent tmux forks; (3) cgroup.kill atomically SIGKILLs every current and future task in the cgroup in one kernel operation, matching logind's effective behavior on SSH logout. Discovered empirically after 5/10 flaky runs with systemctl-based approaches (see Deviations)."
  - "pidAlive must filter zombies. syscall.Kill(pid, 0) returns nil for zombie processes, so naive 'is this pid alive?' checks falsely report a dead-but-unreaped tmux daemon as alive. We read /proc/<pid>/status and treat State: Z or X as dead. Caused 3 additional spurious TEST-02 failures before the filter was added."
  - "Nested-scope edge case (this test process running inside a tmux-spawn-*.scope) is handled by a runtime cgroup check. If the spawned tmux lands in the parent scope's cgroup instead of the fake-login scope's cgroup, we skip with a diagnostic pointing at verify-session-persistence.sh. This keeps the assertion meaningful on CI (login-shell environments) while not failing noisily on executor environments."
  - "TEST-02 uses `tmux -L <serverName>` (private tmux socket per test) so `kill-server` in cleanup is confined to that socket only. Never touches user sessions. Enforces the 2025-12-10 repo safety mandate."

patterns-established:
  - "Every systemd user unit created by the suite uses a per-test random suffix (`fake-login-<hex>` or `agentdeck-tmux-<hex>`). t.Cleanup registers idempotent stop calls scoped to that exact unit name."
  - "Every tmux server uses `tmux -L <unique-socket>`. kill-server is ALWAYS scoped by `-L <socket>` or `-t <server-name>` — grep-verified."
  - "Test diagnostics on FAIL include PID, scope name, final cgroup path, and the explicit systemd state — so a reviewer can tell at a glance whether the failure is real (tmux survived teardown) or environmental (nested scope)."

requirements-completed:
  - TEST-01
  - TEST-02

# Metrics
duration: ~16m
completed: 2026-04-14
---

# Phase 1 Plan 02: Persistence test scaffolding (RED) — TEST-01 and TEST-02 Summary

**Appended TEST-01 (TmuxSurvivesLoginSessionRemoval, RED on v1.5.1) and TEST-02 (TmuxDiesWithoutUserScope, inverse pin green) to `internal/session/session_persistence_test.go`, landing 4 of the 8 mandated persistence tests with a cgroup v2 `cgroup.kill`-based teardown simulation and a zombie-aware `pidAlive` helper.**

## Performance

- **Duration:** ~16 min (deviation-driven; see Deviations — 3 root-cause iterations on TEST-02's teardown mechanism)
- **Started:** 2026-04-14T09:02:23Z
- **Completed:** 2026-04-14T09:18:42Z
- **Tasks:** 2
- **Files modified:** 1 (internal/session/session_persistence_test.go) — 0 production files touched (CLAUDE.md mandate preserved)

## Accomplishments

- Added 5 new unexported helpers: `pidAlive` (zombie-aware), `randomHex8`, `startFakeLoginScope`, `startAgentDeckTmuxInUserScope`, `startTmuxInsideFakeLogin`, `pidCgroup`.
- Added TEST-01 `TestPersistence_TmuxSurvivesLoginSessionRemoval` with the RED-state gate pattern — on current v1.5.1 it fails at the `GetLaunchInUserScope()` default check with the precise Phase-2 diagnostic, creating no tmux/scope artifacts. On post-Phase-2 code it proceeds to the full simulated-teardown path and asserts the tmux server (launched under its OWN `agentdeck-tmux-<name>.scope`) survives a fake-login scope kill.
- Added TEST-02 `TestPersistence_TmuxDiesWithoutUserScope` with three-way behavior: PASSES on Linux+systemd (normal login shell, asserts tmux IS killed by cgroup.kill when inside a fake-login scope), SKIPS on nested-scope executors (this process itself running inside a tmux-spawn-*.scope), SKIPS via `requireSystemdRun` on macOS/non-systemd.
- On this executor host the combined suite reports: TEST-01 FAIL RED, TEST-02 PASS, TEST-03 FAIL RED, TEST-04 SKIP — the intended Phase 1 state.
- Verified no stray tmux servers (`agentdeck-test-persist-*`) and no stray systemd scopes (`fake-login-*`, `agentdeck-tmux-*`) remain after the suite.

## Task Commits

Each task was committed atomically with the `Committed by Ashesh Goplani` sign-off (no Claude attribution):

1. **Task 1: TEST-01 TmuxSurvivesLoginSessionRemoval (RED)** — `bf06f53` (test)
2. **Task 2: TEST-02 TmuxDiesWithoutUserScope (inverse pin)** — `5e42a24` (test)

Plan metadata commit: pending (orchestrator will make the final doc commit with SUMMARY.md, STATE.md, ROADMAP.md).

## Files Created/Modified

- `internal/session/session_persistence_test.go` — MODIFIED. Grew from 185 → 524 lines (+339). New imports: `strconv`, `strings`, `syscall`, `time`. New sections:
  - Helpers (lines 195–310): `pidAlive` (zombie-aware), `randomHex8`, `startFakeLoginScope`, `startAgentDeckTmuxInUserScope`.
  - TEST-01 (lines 312–348): `TestPersistence_TmuxSurvivesLoginSessionRemoval`.
  - Helpers (lines 353–447): `startTmuxInsideFakeLogin`, `pidCgroup`.
  - TEST-02 (lines 450–500): `TestPersistence_TmuxDiesWithoutUserScope`.

## Decisions Made

- **TEST-01 two-stage design** — RED-state gate first (no spawning), then full simulation post-fix. Keeps RED failures cleanup-free.
- **cgroup.kill over systemctl stop/kill** — empirically discovered that scopes don't kill on `stop` (cgroup is released but tasks survive) and `systemctl kill` races with concurrent tmux forks. cgroup v2's `cgroup.kill` is atomic and matches logind's effective SSH-logout behavior.
- **Zombie filtering in pidAlive** — `syscall.Kill(pid, 0)` succeeds for zombies; we supplement with `/proc/<pid>/status` State check so reaped-but-unreaped tmux daemons correctly count as dead.
- **Nested-scope skip** — if the test process itself is inside a transient scope, systemd reparents child scope members to the parent cgroup, breaking the simulation. Detect and skip cleanly with a pointer to the human-watchable verification script (REQ-5, Phase 4).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] Flaky TEST-02: `systemctl stop` does not kill scope members**
- **Found during:** Task 2 initial verification (TEST-02 failed with tmux alive after 3s)
- **Issue:** Plan prescribed `systemctl --user stop <fake-login>.scope` to simulate login-session teardown. Systemd scope semantics are "track, don't lifecycle" — stopping a scope releases its cgroup but does not kill the tasks in it. In the failing run the `(deleted)` cgroup string appeared in `/proc/pid/cgroup` but the tmux daemon was still running.
- **Fix:** Switched to cgroup v2's `/sys/fs/cgroup/<scope-path>/cgroup.kill` write — atomic SIGKILL of every task in the cgroup. This matches logind's effective behavior on SSH logout (`KillMode=control-group`).
- **Files modified:** `internal/session/session_persistence_test.go` (TEST-02 body + comment block explaining the choice)
- **Commit:** `5e42a24`

**2. [Rule 1 — Bug] `pidAlive` returned true for zombie processes**
- **Found during:** Task 2 after fix 1 (still flaky: 2–3 out of 10 runs showed tmux pid "alive" after cgroup.kill)
- **Issue:** `syscall.Kill(pid, 0)` returns `nil` for zombies — processes that have exited but not yet been reaped by their parent. In the TEST-02 teardown path the tmux daemon's parent (the `systemd-run --scope` wrapper) has itself been killed, so tmux is reparented to `systemd --user` which may defer reaping under load. Naive pid-alive check falsely reported these zombies as alive.
- **Fix:** Augmented `pidAlive` to read `/proc/<pid>/status` and treat `State: Z` (zombie) or `X` (dying) as dead.
- **Files modified:** same file
- **Commit:** `5e42a24`

**3. [Rule 3 — Blocking] Nested-scope executor environment breaks the simulation**
- **Found during:** Task 2 early iterations
- **Issue:** This test host's executor is itself running inside an `agent-deck tmux-spawn-*.scope` (the orchestration spawning this plan). When such a process calls `systemd-run --user --scope --unit=X bash -c "..."`, systemd creates unit X.scope in app.slice **but the child processes stay in the parent tmux-spawn-*.scope's cgroup** (known systemd nesting quirk). The plan did not anticipate this.
- **Fix:** After spawning tmux, read `/proc/<pid>/cgroup` and check whether the cgroup path contains the expected fake-login scope name. If not, `t.Skipf` with a diagnostic pointing reviewers at the Phase 4 verification script (which runs from a login shell and does not have this issue).
- **Files modified:** same file
- **Commit:** `5e42a24`

All three were in-scope (Rule 1/3 auto-fixes, test-only file, no architectural change). Each was documented in-code with a comment block explaining the "why" for future maintainers.

## Issues Encountered

- The systemd scope-nesting behavior (fix 3 above) is a real environmental constraint, not a test bug. The nested-scope skip is the correct outcome for executor-hosted runs. CI hosts running from a login shell (the normal case) exercise the full assertion. The Phase 4 `scripts/verify-session-persistence.sh` will give visual confirmation from a login shell where the skip does not trigger.
- Minor: the `.planning/config.json` has a transient unstaged diff (`_auto_chain_active`) introduced by the orchestrator harness; not my change, left for the orchestrator to handle.

## User Setup Required

None. The tests use the host's real `systemd-run`, `tmux`, and cgroup v2 interface. On non-systemd hosts they skip cleanly via `requireSystemdRun`; on nested-scope executors the TEST-02 skip triggers with a clear diagnostic.

## Verification Evidence

All commands run from the executor host on branch `fix/session-persistence` at commit `5e42a24`:

```
go vet ./internal/session/...                                         exit 0
go build ./...                                                        exit 0
go test -run TestPersistence_ ./internal/session/... -race -count=1 -v exit 1 (expected — TEST-01 and TEST-03 are RED)
  --- FAIL: TestPersistence_LinuxDefaultIsUserScope (TEST-03 RED:...)
  --- SKIP: TestPersistence_MacOSDefaultIsDirect (documented rationale)
  --- FAIL: TestPersistence_TmuxSurvivesLoginSessionRemoval (TEST-01 RED:...)
  --- PASS: TestPersistence_TmuxDiesWithoutUserScope           (0.17s)

tmux list-sessions 2>/dev/null | grep -c 'agentdeck-test-persist-'    0
systemctl --user list-units --type=scope --no-legend 2>/dev/null \
  | grep -c 'fake-login-\|agentdeck-tmux-'                            0
git diff --stat internal/tmux/ internal/session/instance.go \
  internal/session/userconfig.go internal/session/storage.go \
  cmd/agent-deck/session_cmd.go                                       (empty)

grep -c "kill-server" internal/session/session_persistence_test.go    3 real calls
  All scoped: 1x "-t <name>" (uniqueTmuxServerName cleanup);
              2x "-L <serverName>" (startAgentDeckTmuxInUserScope,
                                    startTmuxInsideFakeLogin).
```

TEST-02 repeatability: 15 consecutive runs from the executor host produced only PASS or SKIP — no FAIL. Mixed PASS/SKIP distribution reflects systemd's probabilistic nesting behavior for `--scope` calls made from within an existing scope.

## Next Phase Readiness

- Plan 03 of this phase (Wave 3) can now append the remaining four tests (TEST-05, TEST-06, TEST-07, TEST-08) to the same file and reuse: `pidAlive`, `pidCgroup`, `writeStubClaudeBinary`, `isolatedHomeDir`, `uniqueTmuxServerName`, `requireSystemdRun`, `randomHex8`, and the scope-spawning helpers if a Claude-resume test needs cgroup isolation.
- Phase 2 planner has TEST-01 and TEST-03 as concrete RED tests pointing at `GetLaunchInUserScope()` — both will turn green when the default flips on Linux+systemd.
- Phase 4's `scripts/verify-session-persistence.sh` should run from a login shell (not a nested tmux-spawn scope), where TEST-02's full assertion path always runs.

## Self-Check: PASSED

- `internal/session/session_persistence_test.go` — FOUND
- Commit `bf06f53` (TEST-01) — FOUND in `git log --oneline`
- Commit `5e42a24` (TEST-02) — FOUND in `git log --oneline`
- `go vet ./internal/session/...` — exit 0
- `go build ./...` — exit 0
- TEST-01 fails RED on this host with expected diagnostic — confirmed
- TEST-02 passes on this host when the fake-login scope successfully holds tmux; skips with diagnostic when nesting reparents — confirmed across 15 consecutive runs
- No stray tmux servers or scopes after suite — confirmed
- No production-mandate files modified — confirmed via `git diff --stat`
- All `kill-server` invocations scoped by `-L <socket>` or `-t <name>` — grep-verified (3/3)

---
*Phase: 01-persistence-test-scaffolding-red*
*Plan: 02*
*Completed: 2026-04-14*
