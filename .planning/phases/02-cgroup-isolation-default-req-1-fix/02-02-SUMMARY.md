---
phase: 02-cgroup-isolation-default-req-1-fix
plan: 02
subsystem: session-persistence
tags: [go, config, pointer-migration, default-flip, tdd]
requirements_completed: [PERSIST-01, PERSIST-02, PERSIST-03]
dependency_graph:
  requires:
    - "isSystemdUserScopeAvailable() (Plan 01)"
    - "resetSystemdDetectionCacheForTest() (Plan 01)"
  provides:
    - "TmuxSettings.LaunchInUserScope *bool — pointer field that distinguishes 'absent' from 'explicit false'"
    - "GetLaunchInUserScope() three-way logic (explicit override → host-aware default)"
    - "TestPersistence_ExplicitOptOutHonoredOnLinux — four-arm pin on PERSIST-03"
  affects:
    - "Plan 03 (startup log line consumes the same getter to decide which message to emit)"
    - "Plan 04 (fallback path triggers off the same default)"
    - "internal/session/instance.go — three existing call sites already route through GetLaunchInUserScope(); no change required"
tech_stack:
  added: []
  patterns:
    - "*bool TOML field for tri-state: nil (absent), false (explicit opt-out), true (explicit opt-in)"
    - "Host-aware default in getter (delegate to cached probe when nil)"
key_files:
  created: []
  modified:
    - "internal/session/userconfig.go (+15 lines / −5 lines: struct field bool→*bool with updated comment, getter rewritten)"
    - "internal/session/session_persistence_test.go (+122 lines: TestPersistence_ExplicitOptOutHonoredOnLinux with four sub-arms)"
    - "internal/session/userconfig_test.go (+9 lines / −5 lines: TestGetTmuxSettings_LaunchInUserScope_Default updated to assert nil pointer rather than GetLaunchInUserScope()==false; the latter is now host-dependent)"
decisions:
  - "RED test and GREEN production change merged into a single commit, same deviation pattern as Plan 02-01. Rationale: the lefthook pre-commit hook runs go vet, which rejects any RED-only state where Arm 4 references settings.LaunchInUserScope != nil against the current bool field. TDD discipline preserved by writing tests first and capturing 8 compile errors as RED evidence before adding production code."
  - "userconfig_test.go: TestGetTmuxSettings_LaunchInUserScope_Default rewritten to assert pointer-state (nil) instead of GetLaunchInUserScope()==false. The old assertion is incompatible with the host-aware default flip on Linux+systemd; the host-aware behavior is fully covered by TestPersistence_LinuxDefaultIsUserScope (TEST-03) and TestPersistence_MacOSDefaultIsDirect (TEST-04)."
  - "TEST-02 (TestPersistence_TmuxDiesWithoutUserScope) is now PASS on this host where the Phase 1 baseline reported SKIP. No regression — the test now genuinely exercises the failure mode it was designed for. Treating this as a free improvement."
metrics:
  duration_sec: 720
  completed: "2026-04-14T11:50:00Z"
---

# Phase 2 Plan 02: Migrate LaunchInUserScope to *bool and flip Linux+systemd default Summary

**One-liner:** Migrated `TmuxSettings.LaunchInUserScope` from `bool` to `*bool` and rewrote `GetLaunchInUserScope()` so Linux+systemd hosts default to user-scope-launched tmux servers (surviving SSH logout) while explicit `launch_in_user_scope = false` in `config.toml` is always honored.

## What Landed

### Production code (`internal/session/userconfig.go`)

- **Struct field type change** (line 881): `LaunchInUserScope bool` → `LaunchInUserScope *bool`. Comment expanded to document the new tri-state semantics (nil = host-aware default; non-nil = explicit override).
- **Getter rewrite** (lines 912–921): `GetLaunchInUserScope()` now returns `*t.LaunchInUserScope` when non-nil, otherwise falls back to `isSystemdUserScopeAvailable()` (the cached probe added in Plan 01).

### New test (`internal/session/session_persistence_test.go`)

- **`TestPersistence_ExplicitOptOutHonoredOnLinux`** with four sub-arms:
  - `empty_config_defaults_true` — pins the default flip on Linux+systemd.
  - `explicit_false_overrides_default` — pins PERSIST-03's "always honored" rule for the opt-out path.
  - `explicit_true_overrides` — symmetric pin for explicit opt-in.
  - `pointer_state_locked` — three direct `*bool` field-level assertions (4a nil for absent, 4b non-nil pointing to false for explicit false, 4c non-nil pointing to true for explicit true). This is the W2 checker fix that locks the decoder contract at the field level.

Failure messages all contain the literal substring `EXPLICIT-OPT-OUT-RED:` for CI scrapers (8 occurrences in the file).

### Updated test (`internal/session/userconfig_test.go`)

- **`TestGetTmuxSettings_LaunchInUserScope_Default`** rewritten: previously asserted `GetLaunchInUserScope() == false` on empty config, which is now host-dependent and would fail on Linux+systemd post-flip. Now asserts the pointer is `nil` when the field is absent — verifying the decoder contract without colliding with the host-aware default.

## Commits

| Commit  | Message                                                             | Files                                                                                                          |
| ------- | ------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| 61a2f86 | feat(02-02): GREEN — flip launch_in_user_scope default on Linux+systemd | internal/session/userconfig.go, internal/session/session_persistence_test.go, internal/session/userconfig_test.go |

One commit total — see deviations.

## Deviations from Plan

### [Rule 3 — Blocking issue] TDD RED and GREEN commits merged into one

- **Found during:** Task 1 commit attempt (anticipated from Plan 02-01's deviation note in the prior-wave block of the prompt).
- **Issue:** The repo's lefthook `pre-commit` hook runs `go vet ./...`, which rejects any commit where Go source is not vet-clean. Arm 4 of `TestPersistence_ExplicitOptOutHonoredOnLinux` references `settings.LaunchInUserScope != nil` and `*settings.LaunchInUserScope`, which are type errors against the original `bool` field. A RED-only commit cannot pass the hook. Skipping the hook with `--no-verify` is forbidden by user global CLAUDE.md.
- **Fix:** Authored the test first, captured the compile-failure RED evidence (`go test -c ./internal/session/ -o /dev/null` produced 8 errors — see "Captured RED output" below), then applied the production migration in `userconfig.go` and the caller-update in `userconfig_test.go` and committed everything as one atomic `feat(02-02):` commit. TDD discipline preserved — the test was written first against a codebase where the type contract did not match, and its RED state was verified before the field type was changed. The commit body documents this sequence.
- **Files modified:** internal/session/userconfig.go, internal/session/session_persistence_test.go, internal/session/userconfig_test.go.
- **Commit:** 61a2f86.

#### Captured RED output (test added before production change landed)

```
$ go test -c ./internal/session/ -o /dev/null
# github.com/asheshgoplani/agent-deck/internal/session [github.com/asheshgoplani/agent-deck/internal/session.test]
internal/session/session_persistence_test.go:1023:36: invalid operation: settings.LaunchInUserScope != nil (mismatched types bool and untyped nil)
internal/session/session_persistence_test.go:1024:111: invalid operation: cannot indirect settings.LaunchInUserScope (variable of type bool)
internal/session/session_persistence_test.go:1035:36: invalid operation: settings.LaunchInUserScope == nil (mismatched types bool and untyped nil)
internal/session/session_persistence_test.go:1038:7: invalid operation: cannot indirect settings.LaunchInUserScope (variable of type bool)
internal/session/session_persistence_test.go:1039:70: invalid operation: cannot indirect settings.LaunchInUserScope (variable of type bool)
internal/session/session_persistence_test.go:1050:36: invalid operation: settings.LaunchInUserScope == nil (mismatched types bool and untyped nil)
internal/session/session_persistence_test.go:1053:7: invalid operation: cannot indirect settings.LaunchInUserScope (variable of type bool)
internal/session/session_persistence_test.go:1054:69: invalid operation: cannot indirect settings.LaunchInUserScope (variable of type bool)
```

Eight compile errors confirm the test was written against the *bool contract that did not yet exist — the strongest possible RED.

### [Plan-update] `userconfig_test.go` updated as a mechanical consequence

The plan's Task 2 read_first block flagged this as in-scope ("If `internal/session/userconfig_test.go` has a test asserting `LaunchInUserScope == false` after empty config, it WILL break compile … Update it"). `TestGetTmuxSettings_LaunchInUserScope_Default` was indeed asserting `GetLaunchInUserScope() == false` on empty config, which is now host-dependent. Rewrote it to assert pointer-state (`settings.LaunchInUserScope != nil` is the failure condition), so the decoder contract is still pinned without colliding with the new host-aware default. This is documented in the commit body.

## Verification

### Targeted persistence suite (the four tests the plan calls out)

```
$ go test -run "TestPersistence_LinuxDefaultIsUserScope|TestPersistence_ExplicitOptOutHonoredOnLinux|TestPersistence_TmuxDiesWithoutUserScope|TestPersistence_MacOSDefaultIsDirect" ./internal/session/... -race -count=1 -v
=== RUN   TestPersistence_LinuxDefaultIsUserScope
--- PASS: TestPersistence_LinuxDefaultIsUserScope (0.01s)
=== RUN   TestPersistence_MacOSDefaultIsDirect
    session_persistence_test.go:176: systemd-run available; TEST-04 only asserts non-systemd behavior — see TEST-03 for Linux+systemd default
--- SKIP: TestPersistence_MacOSDefaultIsDirect (0.00s)
=== RUN   TestPersistence_TmuxDiesWithoutUserScope
    session_persistence_test.go:475: tmux pid=3742570 cgroup="0::/user.slice/user-1000.slice/user@1000.service/app.slice/fake-login-dff96d1a.scope"
--- PASS: TestPersistence_TmuxDiesWithoutUserScope (0.20s)
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
PASS
ok  	github.com/asheshgoplani/agent-deck/internal/session	1.390s
```

### Full eight-test persistence suite (delta vs Phase 1 baseline)

```
$ go test -run TestPersistence_ ./internal/session/... -race -count=1 -v | tail -50
=== RUN   TestPersistence_LinuxDefaultIsUserScope
--- PASS: TestPersistence_LinuxDefaultIsUserScope (0.01s)
=== RUN   TestPersistence_MacOSDefaultIsDirect
--- SKIP: TestPersistence_MacOSDefaultIsDirect (0.00s)
=== RUN   TestPersistence_TmuxSurvivesLoginSessionRemoval
    session_persistence_test.go:354: startAgentDeckTmuxInUserScope: invalid MainPID "": strconv.Atoi: parsing "": invalid syntax
--- FAIL: TestPersistence_TmuxSurvivesLoginSessionRemoval
=== RUN   TestPersistence_TmuxDiesWithoutUserScope
--- PASS: TestPersistence_TmuxDiesWithoutUserScope (0.18s)
=== RUN   TestPersistence_FreshSessionUsesSessionIDNotResume
--- PASS: TestPersistence_FreshSessionUsesSessionIDNotResume (0.02s)
=== RUN   TestPersistence_RestartResumesConversation
--- PASS: TestPersistence_RestartResumesConversation (1.21s)
=== RUN   TestPersistence_StartAfterSIGKILLResumesConversation
--- FAIL: TestPersistence_StartAfterSIGKILLResumesConversation (Phase 3 RED)
=== RUN   TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion
--- FAIL: TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion (Phase 3 RED)
=== RUN   TestPersistence_ExplicitOptOutHonoredOnLinux
--- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux (0.02s)
    --- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux/empty_config_defaults_true
    --- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux/explicit_false_overrides_default
    --- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux/explicit_true_overrides
    --- PASS: TestPersistence_ExplicitOptOutHonoredOnLinux/pointer_state_locked
```

#### Status delta table

| Test                                                            | Before this plan | After this plan | Notes                                                                                        |
| --------------------------------------------------------------- | ---------------- | --------------- | -------------------------------------------------------------------------------------------- |
| TEST-01 TmuxSurvivesLoginSessionRemoval                         | RED              | RED (different) | Failure changed from "default false" to `invalid MainPID ""` in `startAgentDeckTmuxInUserScope`. The default-flip side-effect this plan was meant to deliver is in (TEST-03 + Arm 1 prove it); TEST-01's remaining failure is the spawn-helper bug Plan 04 will fix. |
| TEST-02 TmuxDiesWithoutUserScope                                | SKIP (Phase 1)   | PASS            | Free improvement: the host now actually exercises the test path. No regression.              |
| TEST-03 LinuxDefaultIsUserScope                                 | RED              | **GREEN**       | Direct target of this plan. Default flipped.                                                 |
| TEST-04 MacOSDefaultIsDirect                                    | SKIP             | SKIP            | Correct — systemd-run is available so the inverse test skips.                                |
| TEST-05 RestartResumesConversation                              | PASS             | PASS            | Untouched.                                                                                   |
| TEST-06 StartAfterSIGKILLResumesConversation                    | RED              | RED             | Phase 3 territory.                                                                           |
| TEST-07 ClaudeSessionIDSurvivesHookSidecarDeletion              | RED              | RED             | Phase 3 territory.                                                                           |
| TEST-08 FreshSessionUsesSessionIDNotResume                      | PASS             | PASS            | Untouched.                                                                                   |
| TestPersistence_ExplicitOptOutHonoredOnLinux (this plan, 4 arms) | n/a              | **GREEN** (4/4) | New regression pin for PERSIST-03.                                                           |

### Build and vet

```
$ go vet ./...        # exit 0
$ go build ./...      # exit 0
```

### Acceptance grep checks

```
$ grep -c "LaunchInUserScope \*bool" internal/session/userconfig.go     # 1 ✓
$ grep -c "LaunchInUserScope bool " internal/session/userconfig.go      # 0 ✓ (old type removed)
$ grep -c "if t.LaunchInUserScope != nil" internal/session/userconfig.go # 1 ✓
$ grep -c "return isSystemdUserScopeAvailable()" internal/session/userconfig.go # 1 ✓
$ grep -c "^func TestPersistence_ExplicitOptOutHonoredOnLinux" internal/session/session_persistence_test.go  # 1 ✓
$ grep -c "EXPLICIT-OPT-OUT-RED" internal/session/session_persistence_test.go  # 8 (≥6 required) ✓
$ git log --format=%B HEAD~0..HEAD | grep -cE "(Co-Authored-By: Claude|Generated with Claude)"  # 0 ✓
```

### Out-of-scope file mandate

```
$ git diff --stat HEAD~1 HEAD -- internal/tmux/ internal/session/instance.go internal/session/storage.go cmd/agent-deck/session_cmd.go
(empty — no out-of-scope file touched) ✓
```

## Caller Audit

`grep -rn '\.LaunchInUserScope' --include='*.go' .` resolved to:

| File                                          | Line  | Type                       | Action                                          |
| --------------------------------------------- | ----- | -------------------------- | ----------------------------------------------- |
| internal/session/userconfig.go               | 881   | TmuxSettings field (`*bool`) | This plan migrated bool→*bool                   |
| internal/session/userconfig.go               | 919   | Getter (deref non-nil)      | This plan rewrote                               |
| internal/session/instance.go                 | 1920, 2037, 4065 | `i.tmuxSession.LaunchInUserScope = GetTmuxSettings().GetLaunchInUserScope()` | No change — already routes through the getter, returns `bool`, target is the Session struct's `bool` field |
| internal/tmux/tmux.go                        | 724, 829 | Session.LaunchInUserScope (bool) | No change — separate `bool` field on Session, fed by the getter upstream |
| internal/tmux/tmux_test.go                   | 2704  | `Session{LaunchInUserScope: true}` | No change — Session.LaunchInUserScope is still bool |
| internal/session/userconfig_test.go          | (rewritten) | TmuxSettings field         | Test updated to assert `*bool` nil-state         |
| internal/session/session_persistence_test.go | (new test) | TmuxSettings field & getter | Four-arm test added by this plan                 |

No caller reads `TmuxSettings.LaunchInUserScope` as `bool` anymore — all consumers go through `GetLaunchInUserScope()`.

## Requirements Closed

- **PERSIST-01** — Linux+systemd default-on cgroup isolation: ✓ delivered. `TestPersistence_LinuxDefaultIsUserScope` is GREEN.
- **PERSIST-02** — non-systemd hosts default-off: ✓ delivered. `TestPersistence_MacOSDefaultIsDirect` skips on this host but its assertion body would pass; the contract is enforced by `isSystemdUserScopeAvailable()` returning false when `systemd-run` is absent (covered by Plan 01's `TestIsSystemdUserScopeAvailable_*` suite).
- **PERSIST-03** — explicit override always honored: ✓ delivered. `TestPersistence_ExplicitOptOutHonoredOnLinux` Arm 2 pins explicit-false, Arm 3 pins explicit-true, Arm 4 pins the underlying `*bool` field state.

Note: REQUIREMENTS.md is owned by the orchestrator. Per the prompt directive ("Do NOT touch STATE.md or ROADMAP.md"), this plan does not toggle requirement status; the orchestrator will mark them complete after the phase rollup.

## Known Stubs

None. All four arms of the new test pass against the migrated production code; the getter is wired to the Plan 01 helper without TODOs.

## Deferred Issues (out of scope per scope-boundary rule)

These were observed during full-package test runs and pre-date this plan (verified by `git stash` test of baseline). They are NOT in the paths under the CLAUDE.md mandate that this plan modifies, and are out of phase scope:

- `TestSyncSessionIDsFromTmux_AllTools` / `TestSyncSessionIDsFromTmux_OverwriteWithNew` / `TestInstance_GetSessionIDFromTmux` / `TestInstance_UpdateClaudeSession_TmuxFirst` / `TestInstance_UpdateClaudeSession_RejectZombie` — all fail with `SetEnvironment failed: exit status 1` (a tmux env-set issue). Confirmed pre-existing on the parent commit (`5e41916`).
- `TestPersistence_TmuxSurvivesLoginSessionRemoval` (TEST-01) — failure mode shifted from "default false" to a spawn-helper edge case (`invalid MainPID ""`). Plan 04's fallback-path patch is the planned fix; documented in the status delta table above so Plan 04 knows what to address.

## Threat Flags

None. The change is a default-flip and a pointer-type refactor; no new network surface, no new auth path, no new file I/O. The trust boundary listed in the plan's threat model (`disk → process` for the TOML decoder) is the existing decoder, and its `nil`/`false`/`true` decoding is now regression-locked by Arm 4 of the new test (mitigates T-02-02-01).

## Self-Check: PASSED

- FOUND: internal/session/userconfig.go (modified, *bool field at line 881)
- FOUND: internal/session/session_persistence_test.go (modified, new test at end)
- FOUND: internal/session/userconfig_test.go (modified, default test rewritten)
- FOUND: .planning/phases/02-cgroup-isolation-default-req-1-fix/02-02-SUMMARY.md (this file)
- FOUND commit: 61a2f86

## Commit command for this SUMMARY

```bash
git add -f .planning/phases/02-cgroup-isolation-default-req-1-fix/02-02-SUMMARY.md
git commit -m "docs(02-02): add SUMMARY for launch_in_user_scope default flip plan

Committed by Ashesh Goplani"
```
