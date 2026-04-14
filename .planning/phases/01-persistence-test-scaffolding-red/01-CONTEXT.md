# Phase 1: Persistence test scaffolding (RED) - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning
**Source:** Conductor directive (acts as PRD)

<domain>
## Phase Boundary

This phase produces the regression test suite that pins both failure modes of the 2026-04-14 incident. **No production code is fixed in Phase 1.** Phase 2 fixes REQ-1 (cgroup default), Phase 3 fixes REQ-2 (resume routing). All eight `TestPersistence_*` tests must exist, compile, and behave as specified before any fix lands.

This is strict TDD RED: six of the eight tests MUST FAIL against current `main` (v1.5.1) code. One test (TEST-02, the inverse pin) MUST PASS immediately because it pins the unfixed opt-out failure mode and must stay green for the rest of the milestone. One (TEST-04) is host-conditional.

</domain>

<decisions>
## Implementation Decisions (all locked from spec REQ-3 + conductor directive)

### Test file location and packaging
- Single new file: `internal/session/session_persistence_test.go`
- Package: same `session` package as `instance_test.go` and `userconfig_test.go` (not `session_test`)
- Build tag: none — must run by default in `go test ./internal/session/...`
- Uses standard `testing` package only (no testify, no ginkgo) to match repo convention

### Test names (verbatim, no abbreviation)
1. `TestPersistence_TmuxSurvivesLoginSessionRemoval`
2. `TestPersistence_TmuxDiesWithoutUserScope`
3. `TestPersistence_LinuxDefaultIsUserScope`
4. `TestPersistence_MacOSDefaultIsDirect`
5. `TestPersistence_RestartResumesConversation`
6. `TestPersistence_StartAfterSIGKILLResumesConversation`
7. `TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion`
8. `TestPersistence_FreshSessionUsesSessionIDNotResume`

### RED state expectations against current v1.5.1 (Linux+systemd)
- TEST-01: FAIL (cgroup default is false → tmux dies on simulated teardown)
- TEST-02: PASS (inverse pin — opt-out behavior already broken, this test confirms it)
- TEST-03: FAIL (`GetLaunchInUserScope()` returns false on Linux today)
- TEST-04: PASS on macOS, conditional on Linux (test author chooses; document choice in test header)
- TEST-05: FAIL (resume routing missing in current code path)
- TEST-06: FAIL (error → start path bypasses resume)
- TEST-07: FAIL (sidecar removal currently breaks resume because authoritative source is wrong)
- TEST-08: FAIL or PASS depending on current accidental-resume behavior — failure messaging must be unambiguous

### Skip semantics
- Every test that requires `systemd-run --user` MUST call `t.Skipf("no systemd-run available: %v", err)` on hosts where the binary is missing or returns non-zero
- Skips are NOT vacuous passes; they print a clear reason
- TEST-03 and TEST-04 detect host capability inside the test body and route to the appropriate assertion
- macOS CI path: every systemd-dependent test skips, suite exits 0

### No-mocking rule (repo CLAUDE.md, hard)
- No mocking of tmux, systemd, or claude binaries inside the persistence tests
- Use real `tmux` and real `systemd-run` binaries; skip if absent
- `claude` itself can be a stub binary in the test's `PATH` because the test is asserting on the spawned command line, not on Claude's behavior — the stub just needs to exit cleanly so tmux pane creation succeeds
- Synthetic JSONL transcripts are written to a temp `~/.claude/projects/<hash>/` directory the test creates and cleans up

### Cleanup invariants
- Each test cleans up every tmux server it spawns (`tmux kill-server -t agentdeck-test-<uniq>`)
- Each test cleans up every JSONL transcript and hook sidecar it writes
- After the suite runs, `tmux list-sessions` must show no stray `agentdeck-test-*` servers
- Cleanup uses `t.Cleanup()` so it runs even on test failure
- Test names embed a `t.TempDir()`-scoped unique suffix to allow `-parallel` execution without collisions

### tmux server safety (CRITICAL — repo CLAUDE.md mandate)
- **NEVER** call `tmux kill-server` without a `-t <name>` filter
- **NEVER** target tmux server names matching `agentdeck-*` outside the test's own scope
- Tests use a unique server name per test invocation (e.g. `agentdeck-test-persist-<random>`) and only kill that specific server
- 2025-12-10 incident: a `tmux ls | grep agentdeck | xargs tmux kill-session` killed all 40 user sessions. Tests must not introduce a similar pattern.

### Independent runnability
- Each test runs standalone via `go test -run TestPersistence_<name> ./internal/session/...`
- No shared global state between tests; each constructs its own `Instance`, temp dirs, and tmux server
- No external network dependencies

### Failure message quality
- RED-state failures must reference the exact missing behavior (e.g. `"expected --resume in claude command, got: %s"`) — not compile errors, not nil-pointer panics
- The diagnostic messaging is what tells Phase 2 / Phase 3 implementers what to fix

### Production code: read-only this phase
- Phase 1 may inspect `internal/tmux/tmux.go`, `internal/session/instance.go`, `internal/session/userconfig.go`, `internal/session/storage.go`, and `cmd/agent-deck/session_cmd.go` to write tests against the real APIs
- Phase 1 MUST NOT modify any production code; if a test cannot be written without an exported helper, document the gap in the test file as a `// TODO(phase-2):` comment and write the test against the closest existing API

### Claude's Discretion (no spec mandate)
- Helper file structure inside the test file (single file vs. helpers in a `_helpers.go` file) — keep all 8 tests in `session_persistence_test.go` per spec REQ-3, but extract repeated setup into unexported test helpers within the same file
- Naming of unique tmux server suffixes (random hex, atomic counter, t.Name() hash — implementer chooses)
- Stub claude binary mechanism: a test-local script written to `t.TempDir()` and prepended to `PATH`, vs. building a tiny Go binary in `TestMain`. Prefer the simpler PATH-prepend approach
- Whether to combine TEST-05 and TEST-06 setup into a shared `setupResumableInstance(t)` helper

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Spec and requirements
- `docs/SESSION-PERSISTENCE-SPEC.md` — REQ-3 enumerates the eight tests verbatim with semantics and skip rules
- `.planning/REQUIREMENTS.md` — TEST-01 through TEST-08 acceptance criteria
- `.planning/ROADMAP.md` — Phase 1 Success Criteria (six items, including the RED state expectations)

### Repo mandates and safety
- `CLAUDE.md` (project root) — "Session persistence: mandatory test coverage" section names the eight tests as P0 forever; also contains the no-`rm`, no-Claude-attribution, no-`tmux kill-server` rules
- `~/.claude/CLAUDE.md` (user global) — tmux session protection rules

### Production code under test (read for API surface; do not modify in Phase 1)
- `internal/tmux/tmux.go` — `LaunchInUserScope` field, `systemd-run` wrap (around line 724 and 814–837 per spec)
- `internal/session/userconfig.go` — `TmuxSettings` struct, `GetLaunchInUserScope()` accessor
- `internal/session/userconfig_test.go` — current default behavior pinned at line ~1102 (currently `false`)
- `internal/session/instance.go` — `Instance` struct, `ClaudeSessionID` field, `Restart()` (~line 3763), `buildClaudeResumeCommand()` (~line 4114), `sessionHasConversationData()`
- `internal/session/storage.go` — instance JSON persistence (`~/.agent-deck/<profile>/`)
- `cmd/agent-deck/session_cmd.go` — `session start` / `session stop` / `session restart` handlers; `ClaudeSessionID` preservation on stop (~line 286)
- `docs/session-id-lifecycle.md` — invariant: instance JSON is the authoritative source, no disk-scan binding

### Existing test patterns to match
- `internal/session/instance_test.go` — table-driven test style, helper conventions
- `internal/session/userconfig_test.go` — config-default test pattern (relevant for TEST-03 / TEST-04)
- `internal/session/storage_concurrent_test.go` — temp-dir + cleanup pattern
- `internal/tmux/tmux_test.go` — real-tmux test pattern, cleanup, skip-if-no-binary

### Hard rules (project + user CLAUDE.md, both apply)
- No `git push`, `git tag`, `gh release`, `gh pr create/merge` without explicit user approval
- No `rm` — use `trash` (`/usr/bin/trash`) for any cleanup outside test process
- No Claude attribution in commits ("Co-Authored-By: Claude" forbidden)
- TDD always: tests land before fixes (Phase 1's entire purpose)
- `.git/info/exclude` ignores `.planning/` — commits to `.planning/` MUST use `git add -f` then `git commit` directly (do not rely on gsd-tools commit helper for `.planning/` files)
- No `tmux kill-server` without a `-t <specific-name>` filter

</canonical_refs>

<specifics>
## Specific Ideas

### Verification command (must work after Phase 1)
```
go test -run TestPersistence_ ./internal/session/... -race -count=1
```
On Linux+systemd: 1 PASS (TEST-02), 6 FAIL with diagnostic messages (TEST-01, 03, 05, 06, 07, 08), 1 conditional (TEST-04).
On macOS: tests requiring systemd skip cleanly; remaining tests behave as authored.

### Stub claude binary pattern
A minimal shell script written to `t.TempDir()/claude` that:
- Echoes its argv to a known file in temp dir (so the test can grep for `--resume` / `--session-id`)
- Sleeps briefly so the tmux pane stays alive long enough to be inspected
- Test prepends temp dir to `PATH` for the duration of the test

### Login-session teardown simulation
Per spec REQ-3 TEST-01:
1. `systemd-run --user --scope --unit=fake-login-<rand> bash -c "exec sleep 60"` — establishes a throwaway scope
2. Spawn agent-deck tmux from a child of that scope (or capture the agent-deck tmux PID before triggering teardown)
3. `systemctl --user stop fake-login-<rand>.scope` — terminates the throwaway scope tree
4. Assert agent-deck tmux PID still alive (TEST-01) or dead (TEST-02 when launch_in_user_scope=false)

### TEST-05 / TEST-06 transcript path
`~/.claude/projects/<hash>/` where `<hash>` is the cwd hash claude uses. Tests write a synthetic JSONL with at least one user/assistant exchange so `sessionHasConversationData()` returns true.

</specifics>

<deferred>
## Deferred Ideas

- The actual code fixes for REQ-1 (Phase 2) and REQ-2 (Phase 3)
- The verification harness `scripts/verify-session-persistence.sh` (Phase 4)
- CI wiring of the test suite (Phase 4)
- Updating CHANGELOG.md / README.md (Phase 4)
- TUI `↻` glyph for resumable sessions (deferred to v2 per requirements)

</deferred>

---

*Phase: 01-persistence-test-scaffolding-red*
*Context gathered: 2026-04-14 from conductor directive + spec REQ-3*
