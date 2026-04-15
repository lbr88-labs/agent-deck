# Phase 04 — Deferred Items

## Pre-existing test failures (not Phase 04 regressions)

Confirmed pre-existing via `git stash` + re-run on unchanged tree at `6fdac26`:

| Test | File | Symptom | Root cause |
| --- | --- | --- | --- |
| `TestSyncSessionIDsFromTmux_Claude` | `internal/session/instance_platform_test.go:30` | `SetEnvironment failed: exit status 1` | Requires live tmux server — not available in CI/sandbox |
| `TestSyncSessionIDsFromTmux_AllTools` | `internal/session/instance_platform_test.go:72` | `SetEnvironment(CODEX_SESSION_ID) failed: exit status 1` | Same — tmux-dependent |
| `TestSyncSessionIDsFromTmux_OverwriteWithNew` | `internal/session/instance_platform_test.go:141` | `SetEnvironment failed: exit status 1` | Same — tmux-dependent |
| `TestInstance_GetSessionIDFromTmux` | `internal/session/instance_test.go:647` | `Failed to set environment: exit status 1` | Same — tmux-dependent |
| `TestInstance_UpdateClaudeSession_TmuxFirst` | `internal/session/instance_test.go:672` | `Failed to set environment: exit status 1` | Same — tmux-dependent |
| `TestInstance_UpdateClaudeSession_RejectZombie` | `internal/session/instance_test.go:767` | `set tmux env: exit status 1` | Same — tmux-dependent |

None of these touch per-group or per-conductor config resolution. Out of scope for Phase 04. Scope boundary rule applied: only auto-fix issues directly caused by current changes.

The CFG-11 suite (`TestConductorConfig_*`) and CFG-04 suite (`TestPerGroupConfig_*`) are 8/8 + 8/8 GREEN, which is the Phase 04 regression gate.
