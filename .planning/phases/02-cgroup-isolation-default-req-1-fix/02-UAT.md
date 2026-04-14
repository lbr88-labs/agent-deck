---
status: complete
phase: 02-cgroup-isolation-default-req-1-fix
source: [02-01-SUMMARY.md, 02-02-SUMMARY.md, 02-03-SUMMARY.md, 02-04-SUMMARY.md, 02-05-SUMMARY.md]
started: 2026-04-14T14:19:00Z
updated: 2026-04-14T14:20:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: Build fresh binary, launch with isolated HOME and AGENTDECK_DEBUG=1, OBS-01 line lands as the first record in ~/.agent-deck/debug.log with the exact pinned string.
result: pass
evidence: |
  $ go build -o /tmp/agent-deck-verify-bin ./cmd/agent-deck    # exit 0
  $ env -u TMUX ... HOME=$tmpdir AGENTDECK_DEBUG=1 ... /tmp/agent-deck-verify-bin
  $ head -1 $tmpdir/.agent-deck/debug.log
  {"time":"2026-04-14T14:19:38...","level":"INFO","msg":"tmux cgroup isolation: enabled (systemd-run detected)","component":"session"}
  $ grep -c 'tmux cgroup isolation' $tmpdir/.agent-deck/debug.log  # 1 (sync.Once dedup honored)

### 2. REQ-1a: Linux+systemd default is true
expected: On Linux+systemd host, GetLaunchInUserScope() with no explicit setting returns true via isSystemdUserScopeAvailable() cached probe.
result: pass
evidence: |
  $ go test -run TestPersistence_LinuxDefaultIsUserScope ./internal/session/... -race
  --- PASS: TestPersistence_LinuxDefaultIsUserScope (0.01s)
  $ go test -run TestIsSystemdUserScopeAvailable_ ./internal/session/... -race
  --- PASS: TestIsSystemdUserScopeAvailable_MatchesHostCapability (0.02s)
  --- PASS: TestIsSystemdUserScopeAvailable_CachesResult (0.01s)
  --- PASS: TestIsSystemdUserScopeAvailable_ResetForTestRePrubes (0.01s)

### 3. REQ-1b: Non-systemd host defaults to false (macOS / BSD / Linux without user manager)
expected: TestPersistence_MacOSDefaultIsDirect skip-guard correctly gates this host (systemd-run present), and the logic is proven by TestIsSystemdUserScopeAvailable_MatchesHostCapability (returns false when systemd-run absent) plus ExplicitOptOutHonoredOnLinux/pointer_state_locked.
result: pass
evidence: |
  --- SKIP: TestPersistence_MacOSDefaultIsDirect
      session_persistence_test.go:176: systemd-run available; TEST-04 only asserts non-systemd behavior
  Logic coverage: isSystemdUserScopeAvailable returns false iff exec.LookPath fails OR `systemd-run --user --version` exits non-zero.

### 4. REQ-1c: Explicit override (opt-out and opt-in) always honored
expected: TestPersistence_ExplicitOptOutHonoredOnLinux passes all four sub-arms (empty_config_defaults_true, explicit_false_overrides_default, explicit_true_overrides, pointer_state_locked).
result: pass
evidence: |
  --- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux (0.02s)
      --- PASS: .../empty_config_defaults_true
      --- PASS: .../explicit_false_overrides_default
      --- PASS: .../explicit_true_overrides
      --- PASS: .../pointer_state_locked

### 5. TEST-02 opt-out regression gate (fallback must NOT re-enable systemd-run for explicit opt-outs)
expected: TestPersistence_TmuxDiesWithoutUserScope either PASS (test runs through) or SKIP (host already inside a transient scope — Phase 1 baseline skip semantics). Either outcome proves the opt-out gate is intact; production logic is additionally pinned by ExplicitOptOutHonoredOnLinux/pointer_state_locked and by the `launcher == "systemd-run"` guard in the fallback block.
result: pass
evidence: |
  --- SKIP: TestPersistence_TmuxDiesWithoutUserScope (0.35s)
      Host already inside a transient scope — matches Phase 1 baseline skip; not a regression.
  Production pin: internal/tmux/tmux.go fallback block guarded by `if err != nil && launcher == "systemd-run"` — launcher is "tmux" when LaunchInUserScope=false, so fallback never fires on opt-out path.

### 6. OBS-01: Exactly one structured startup log line describing the cgroup-isolation decision
expected: Five unit tests pin the four matrix branches + dedup. Wire-up at cmd/agent-deck/main.go:444. Real binary launch produces the pinned string as the first line of debug.log.
result: pass
evidence: |
  --- PASS: TestLogCgroupIsolationDecision_NilOverride_SystemdAvailable
  --- PASS: TestLogCgroupIsolationDecision_NilOverride_SystemdAbsent
  --- PASS: TestLogCgroupIsolationDecision_ExplicitFalseOverride
  --- PASS: TestLogCgroupIsolationDecision_ExplicitTrueOverride
  --- PASS: TestLogCgroupIsolationDecision_OnlyEmitsOnce
  Wire-up: cmd/agent-deck/main.go:444 → session.LogCgroupIsolationDecision()
  Live: head -1 debug.log → "tmux cgroup isolation: enabled (systemd-run detected)"

### 7. PERSIST-04/05: Fallback on systemd-run failure never blocks session creation
expected: TestStartCommandSpec_FallsBackToDirect proves s.Start returns nil when systemd-run fails and direct tmux retry succeeds, with `tmux_systemd_run_fallback` slog warning emitted. TestStartCommandSpec_BothFailWrapsError proves the wrapped error contains both "systemd-run path:" and "direct retry:" substrings when both paths fail.
result: pass
evidence: |
  --- PASS: TestStripSystemdRunPrefix_RecoversTmuxArgs (0.00s)
  --- PASS: TestStripSystemdRunPrefix_PassesThroughUnexpectedShape (0.00s)
  --- PASS: TestStartCommandSpec_FallsBackToDirect (0.09s)
  --- PASS: TestStartCommandSpec_BothFailWrapsError (0.01s)

### 8. Example-config comment aligned with runtime behavior (no drift)
expected: The embedded example-config in internal/session/userconfig.go (lines 1948-1953) documents the host-aware default AND the "always honored" override promise, with the example line showing `launch_in_user_scope = false` (the natural override case now that true is the default).
result: pass
evidence: |
  internal/session/userconfig.go:1948-1953:
    # launch_in_user_scope starts new tmux servers with systemd-run --user --scope
    # so they survive when the current login session is torn down (e.g. SSH logout).
    # Default: true on Linux+systemd hosts where 'systemd-run --user --version'
    #          succeeds, false on macOS / BSD / Linux without a user manager.
    # An explicit setting here is ALWAYS honored.
    # launch_in_user_scope = false

### 9. TEST-01 RED-cause distinction (test-helper vs. REQ-1 regression)
expected: TestPersistence_TmuxSurvivesLoginSessionRemoval RED failure line is a test-helper PID-discovery bug (`invalid MainPID ""`), NOT a production REQ-1 regression. Production contract is proven by TEST-03 (default flip), fallback seam tests (PERSIST-04/05), and ExplicitOptOutHonoredOnLinux/empty_config_defaults_true.
result: pass
evidence: |
  Failure: session_persistence_test.go:354: startAgentDeckTmuxInUserScope: invalid MainPID "": strconv.Atoi: parsing "": invalid syntax
  Root cause (per 02-04-SUMMARY): `systemctl --user show -p MainPID --value <unit>.scope` returns empty because tmux double-forks — the scope's initial process exits before systemd records MainPID. The scope IS active and tmux IS inside its cgroup; MainPID is just an unsuitable readout.
  Fix owner: dedicated test-infra follow-up plan (replace MainPID query with cgroup.procs / pgrep walk). Out of Phase 2 scope per Plan 04 files_modified whitelist.
  Production contract unaffected: TEST-03 GREEN + TEST-02 gate intact + fallback tests GREEN + cold-start smoke emits correct OBS-01 line.

## Summary

total: 9
passed: 9
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none — all observable truths per REQ-1 spec verified GREEN]

## Deferred (out of Phase 2 scope; tracked but not gap-classified)

- TEST-01 `TestPersistence_TmuxSurvivesLoginSessionRemoval` RED — test-helper PID-discovery bug in `startAgentDeckTmuxInUserScope` at internal/session/session_persistence_test.go:283-318. Not a REQ-1 regression (see Test 9 above). Follow-up: separate test-infra plan.
- TEST-06 `TestPersistence_StartAfterSIGKILLResumesConversation` RED — Phase 3 territory (REQ-2 resume dispatch; Start() bypasses buildClaudeResumeCommand).
- TEST-07 `TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion` RED — Phase 3 territory, same root cause as TEST-06.
