# Deferred Items — Phase 01

Issues discovered during Phase 01 execution that are out of scope for this
phase and logged for follow-up.

## 1. Pre-existing tmux-environment test failures in `internal/session`

**Scope:** NOT caused by Phase 01 changes. Verified by reverting
`internal/session/instance.go` to parent commit `4730aa5` and removing
`internal/session/pergroupconfig_test.go` — the identical six tests still
fail with the same `exit status 1` error from `tmux set-environment`.

**Failing tests (all in `package session`):**
- `TestSyncSessionIDsFromTmux_Claude` (`instance_platform_test.go:30`)
- `TestSyncSessionIDsFromTmux_AllTools` (`instance_platform_test.go:72`)
- `TestSyncSessionIDsFromTmux_OverwriteWithNew` (`instance_platform_test.go:141`)
- `TestInstance_GetSessionIDFromTmux` (`instance_test.go:647`)
- `TestInstance_UpdateClaudeSession_TmuxFirst` (`instance_test.go:672`)
- `TestInstance_UpdateClaudeSession_RejectZombie` (`instance_test.go:767`)

**Symptom:** All six fail with `SetEnvironment failed: exit status 1`
from `tmux set-environment` after `inst.Start()` spawns a test tmux session.

**Hypothesis:** These tests use `skipIfNoTmuxServer` but still run when a
tmux binary and some tmux server are available. In this worktree's test
environment (running inside an outer tmux pane with `TMUX` set), the test
sub-process's `tmux set-environment` command appears to target a server
that rejects the write — possibly a permission issue, a stale socket, or
an interaction between the lefthook-scrubbed environment (which removes
`GIT_*` but preserves `TMUX`) and the spawned test session.

**Impact on Phase 01:**
- All four Phase-1 tests (`TestPerGroupConfig_*`) PASS individually and
  under the direct `go test ./internal/session/ -run TestPerGroupConfig_`
  invocation.
- All PR #578 regression tests (`TestGetClaudeConfigDirForGroup_GroupWins`,
  `TestIsClaudeConfigDirExplicitForGroup`, `TestBuildClaudeCommand_CustomAlias`,
  `TestUserConfig_GroupClaudeConfigDir*`, `TestUserConfig_GroupClaudeEnvFile`)
  PASS.
- `make ci` returns non-zero solely from these six pre-existing failures.

**Deferred to:** a separate investigation, likely a follow-up phase or a
v1.5.4 follow-up plan. The `skipIfNoTmuxServer` helper should probably be
expanded to also skip when `tmux set-environment` itself cannot write to
the target server — but diagnosing the exact root cause is out of scope
for CFG-02.

**Not a Phase 01 regression.** Recorded here per executor deviation rules
(SCOPE BOUNDARY — "Pre-existing failures in unrelated files are out of
scope. Log to deferred-items.md.").
