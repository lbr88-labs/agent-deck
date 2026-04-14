---
phase: 02-cgroup-isolation-default-req-1-fix
plan: 01
subsystem: session-persistence
tags: [go, systemd, detection, tdd, sync-once]
requirements_completed: [PERSIST-01, PERSIST-02]
dependency_graph:
  requires: []
  provides:
    - "isSystemdUserScopeAvailable() bool — cached Linux+systemd detection helper"
    - "resetSystemdDetectionCacheForTest() — test-only cache reset"
    - "systemdUserScopeProbeCount int64 — probe counter (atomic)"
  affects:
    - "Plan 02 (default getter will call isSystemdUserScopeAvailable)"
    - "Plan 03 (startup log line will call isSystemdUserScopeAvailable)"
    - "Plan 04 (fallback path decision surface)"
tech_stack:
  added: []
  patterns:
    - "sync.Once-cached host capability probe"
    - "sync/atomic counter for probe invocations"
    - "Swallow exec errors as false (no I/O, no panic)"
key_files:
  created:
    - "internal/session/userconfig_systemd_test.go (55 lines)"
  modified:
    - "internal/session/userconfig.go (+48 lines)"
decisions:
  - "Tests and implementation landed in a single commit because the repo's pre-commit hook runs `go vet`; a RED-only commit that references undefined production names is rejected. TDD discipline preserved by authoring tests first and verifying compilation failed RED before implementing."
metrics:
  duration_sec: 253
  completed: "2026-04-14T11:31:45Z"
---

# Phase 2 Plan 01: isSystemdUserScopeAvailable Detection Helper Summary

**One-liner:** Landed the sync.Once-cached `isSystemdUserScopeAvailable()` helper in `internal/session/userconfig.go` with three unit tests that pin its contract, agreeing byte-for-byte with the `requireSystemdRun` test gate.

## What Landed

### Production code (`internal/session/userconfig.go`)

Added three symbols directly after `GetLaunchInUserScope()`:

- **Package vars**
  - `systemdUserScopeOnce sync.Once` — guards the probe.
  - `systemdUserScopeAvailable bool` — cached result.
  - `systemdUserScopeProbeCount int64` — probe invocation counter (atomic).
- **`func isSystemdUserScopeAvailable() bool`** — returns true iff `exec.LookPath("systemd-run")` succeeds AND `systemd-run --user --version` exits zero. Result cached for the process lifetime. No stdout/stderr writes, no panic on missing/broken systemd-run, errors swallowed as false. Increments the probe counter atomically inside the `sync.Once.Do` callback.
- **`func resetSystemdDetectionCacheForTest()`** — rewinds the `sync.Once` and the cached bool so the next call re-probes. Package-private helper used only by tests in package `session`.

Imports added: `os/exec`, `sync/atomic`. `sync` was already present.

### Tests (`internal/session/userconfig_systemd_test.go`)

Three tests, all passing:

1. **`TestIsSystemdUserScopeAvailable_MatchesHostCapability`** — recomputes the host probe (same LookPath + `--user --version` pair as `requireSystemdRun`) and asserts the helper agrees. On this host: both returned `true`.
2. **`TestIsSystemdUserScopeAvailable_CachesResult`** — resets the cache, calls twice, asserts results match AND `systemdUserScopeProbeCount == 1` (sync.Once held).
3. **`TestIsSystemdUserScopeAvailable_ResetForTestRePrubes`** — after reset, calls again, asserts `systemdUserScopeProbeCount == 2` (re-probe occurred).

## Commits

| Commit  | Message                                                                           | Files                                                                    |
| ------- | --------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| cdab8ef | feat(02-01): add isSystemdUserScopeAvailable detection helper (TDD RED+GREEN)     | internal/session/userconfig.go, internal/session/userconfig_systemd_test.go |

Both files landed in a single commit — see deviations.

## Deviations from Plan

### [Rule 3 — Blocking issue] TDD RED and GREEN commits merged into one

- **Found during:** Task 1 commit attempt.
- **Issue:** The repo's lefthook `pre-commit` hook runs `go vet ./...`, which rejects any commit where Go source references undefined symbols. The planned RED-only commit (test file that references `isSystemdUserScopeAvailable`, `resetSystemdDetectionCacheForTest`, and `systemdUserScopeProbeCount` before they exist) cannot pass the hook. Skipping the hook with `--no-verify` is forbidden by user global CLAUDE.md ("Never skip hooks unless user explicitly requests it").
- **Fix:** Authored the tests first, confirmed compilation was RED via `go test -c ./internal/session/ -o /dev/null` (11 undefined-symbol errors captured below), then added the production helper and landed both files in a single `feat(02-01):` commit. TDD discipline is preserved — the test was written first against a codebase where the symbol did not exist, and its RED state was verified before the implementation was typed. The commit message documents this sequence.
- **Files modified:** internal/session/userconfig.go, internal/session/userconfig_systemd_test.go.
- **Commit:** cdab8ef.

#### Captured RED output (before implementation landed)

```
$ go test -c ./internal/session/ -o /dev/null
# github.com/asheshgoplani/agent-deck/internal/session [github.com/asheshgoplani/agent-deck/internal/session.test]
internal/session/userconfig_systemd_test.go:20:2: undefined: resetSystemdDetectionCacheForTest
internal/session/userconfig_systemd_test.go:21:9: undefined: isSystemdUserScopeAvailable
internal/session/userconfig_systemd_test.go:31:2: undefined: resetSystemdDetectionCacheForTest
internal/session/userconfig_systemd_test.go:32:21: undefined: systemdUserScopeProbeCount
internal/session/userconfig_systemd_test.go:33:7: undefined: isSystemdUserScopeAvailable
internal/session/userconfig_systemd_test.go:34:7: undefined: isSystemdUserScopeAvailable
internal/session/userconfig_systemd_test.go:38:28: undefined: systemdUserScopeProbeCount
internal/session/userconfig_systemd_test.go:47:2: undefined: resetSystemdDetectionCacheForTest
internal/session/userconfig_systemd_test.go:48:21: undefined: systemdUserScopeProbeCount
internal/session/userconfig_systemd_test.go:49:6: undefined: isSystemdUserScopeAvailable
internal/session/userconfig_systemd_test.go:49:6: too many errors
```

All three target symbols (`isSystemdUserScopeAvailable`, `resetSystemdDetectionCacheForTest`, `systemdUserScopeProbeCount`) were undefined before the implementation landed, satisfying the plan's RED contract.

## Verification

### New unit tests

```
$ go test -run TestIsSystemdUserScopeAvailable_ ./internal/session/... -race -count=1 -v
=== RUN   TestIsSystemdUserScopeAvailable_MatchesHostCapability
--- PASS: TestIsSystemdUserScopeAvailable_MatchesHostCapability (0.01s)
=== RUN   TestIsSystemdUserScopeAvailable_CachesResult
--- PASS: TestIsSystemdUserScopeAvailable_CachesResult (0.01s)
=== RUN   TestIsSystemdUserScopeAvailable_ResetForTestRePrubes
--- PASS: TestIsSystemdUserScopeAvailable_ResetForTestRePrubes (0.01s)
PASS
ok  	github.com/asheshgoplani/agent-deck/internal/session	1.076s
```

### Full persistence suite (no regression vs Phase 1 baseline)

```
$ go test -run TestPersistence_ ./internal/session/... -race -count=1
--- FAIL: TestPersistence_LinuxDefaultIsUserScope (TEST-03 RED — Plan 02 fixes)
--- SKIP: TestPersistence_MacOSDefaultIsDirect (systemd-run available; inverse test skips)
--- FAIL: TestPersistence_TmuxSurvivesLoginSessionRemoval (TEST-01 RED — Plan 02 fixes)
--- SKIP: TestPersistence_TmuxDiesWithoutUserScope (host already inside a transient scope — matches Phase 1 baseline)
--- PASS: TestPersistence_FreshSessionUsesSessionIDNotResume (TEST-08)
--- PASS: TestPersistence_RestartResumesConversation (TEST-05)
--- FAIL: TestPersistence_StartAfterSIGKILLResumesConversation (TEST-06 RED — Phase 3 fixes)
--- FAIL: TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion (TEST-07 RED — Phase 3 fixes)
```

Baseline status confirmed unchanged:
- TEST-01: RED (Phase 2 Plan 02 target)
- TEST-02: SKIP on this host (transient-scope reparenting — Phase 1 baseline)
- TEST-03: RED (Phase 2 Plan 02 target)
- TEST-04: SKIP (inverse of TEST-03 — systemd-run present, so this variant skips)
- TEST-05: PASS
- TEST-06: RED (Phase 3)
- TEST-07: RED (Phase 3)
- TEST-08: PASS

No test that was previously passing/skipping regressed.

### Other checks

```
$ go vet ./internal/session/...   # exit 0
$ go build ./...                  # exit 0
$ grep -c '^func TestIsSystemdUserScopeAvailable_' internal/session/userconfig_systemd_test.go  # 3
$ grep -c '^func isSystemdUserScopeAvailable() bool' internal/session/userconfig.go              # 1
$ grep -c '^func resetSystemdDetectionCacheForTest()' internal/session/userconfig.go             # 1
$ grep -c 'systemdUserScopeOnce' internal/session/userconfig.go                                  # 2
$ grep -c 'atomic.AddInt64' internal/session/userconfig.go                                       # 1
$ git log --format=%B HEAD~0..HEAD | grep -cE '(Co-Authored-By: Claude|Generated with Claude)'   # 0
```

## Requirements Closed

- **PERSIST-01** — foundation laid (detection helper exists and returns `true` on Linux+systemd hosts). Final requirement closure will be staged when Plan 02 wires the getter to read this helper.
- **PERSIST-02** — foundation laid (detection helper returns `false` on hosts without `systemd-run` with no error logged). Same note as PERSIST-01.

Note: These two requirements remain formally "Pending" in REQUIREMENTS.md — the orchestrator owns that file and will mark them complete after Plan 02 lands the default getter that surfaces the behavior.

## Known Stubs

None. The helper is fully wired and tested; Plan 02 will consume it without modification.

## Commit command for this SUMMARY

```bash
git add -f .planning/phases/02-cgroup-isolation-default-req-1-fix/02-01-SUMMARY.md
git commit -m "docs(02-01): add SUMMARY for isSystemdUserScopeAvailable plan

Committed by Ashesh Goplani"
```

## Self-Check: PASSED

- FOUND: internal/session/userconfig_systemd_test.go
- FOUND: internal/session/userconfig.go
- FOUND: .planning/phases/02-cgroup-isolation-default-req-1-fix/02-01-SUMMARY.md
- FOUND commit: cdab8ef
