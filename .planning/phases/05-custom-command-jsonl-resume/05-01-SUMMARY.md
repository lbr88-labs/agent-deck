# Plan 05-01 Summary — RED TEST-09

**Phase:** 05-custom-command-jsonl-resume
**Plan:** 01
**TDD stage:** RED
**Commit:** `7f2ff35` on `fix/session-persistence`
**Date:** 2026-04-15

## What landed

One new test + one helper appended to `internal/session/session_persistence_test.go`:

- `func writeCustomWrapperScript(t *testing.T, home string) string` — stages a real shell script at `<home>/bin/my-wrapper.sh` that writes a `wrapper_invoked` sentinel + argv to `AGENTDECK_TEST_ARGV_LOG` then `sleep 30`. Required for the RED path where `buildClaudeCommand(i.Command)` returns `i.Command` verbatim when `Command != "claude"`.
- `func TestPersistence_CustomCommandResumesFromLatestJSONL(t *testing.T)` — pins REQ-7 / TEST-09. Top-level asserts `inst.ClaudeSessionID == newerUUID` after Start() AND the spawned claude argv contains `--resume <newerUUID>`. Sub-case `no_jsonl_falls_through_to_fresh` pins PERSIST-13 (no JSONL → no `--resume`, `ClaudeSessionID` stays empty).

## UUIDs used (for grepability)

- `olderUUID = 11111111-1111-1111-1111-111111111111`
- `newerUUID = 22222222-2222-2222-2222-222222222222`

## RED-stage `go test` output

```
--- FAIL: TestPersistence_CustomCommandResumesFromLatestJSONL (0.27s)
    session_persistence_test.go:1418: TEST-09 PERSIST-12 RED: after Start() with Command="/tmp/.../bin/my-wrapper.sh", empty ClaudeSessionID, and TWO JSONLs (11111111-... older, 22222222-... newer) under /tmp/.../.claude/projects/..., inst.ClaudeSessionID="", want "22222222-..." (newer JSONL UUID). The Phase 5 helper must mutate i.ClaudeSessionID before spawn so subsequent Restart() takes the Phase 3 fast path. This is the 2026-04-15 incident REQ-7 root cause: Start()'s empty-ID branch at instance.go:1895-1901 dispatches through buildClaudeCommand (fresh UUID) instead of discovering the newest JSONL on disk.
FAIL    github.com/asheshgoplani/agent-deck/internal/session    0.278s
```

Exit code non-zero as required.

## Deviation from `<action>` and rationale

**Deviation:** The plan prescribed `inst.Command = filepath.Join(home, "bin", "my-wrapper.sh")` with "the file need not exist" and the claim that `setupStubClaudeOnPATH`'s config.toml override would still cause the stub to spawn. This is factually incorrect: `buildClaudeCommand(baseCommand)` at `instance.go:485-597` only consults `GetClaudeCommand()` (which reads the config override) when `baseCommand == "claude"`. When `baseCommand` is a custom wrapper path, the function returns the wrapper path verbatim at line 596. With a non-existent wrapper, the tmux pane would exec a missing file, die immediately, and `readCapturedClaudeArgv` would `t.Fatal` with a generic "stub claude was never spawned" message BEFORE our targeted `TEST-09 PERSIST-12 RED:` diagnostic could fire.

**Fix applied:** Added `writeCustomWrapperScript` that stages a real functional wrapper at `<home>/bin/my-wrapper.sh`. In RED the wrapper runs and emits the `wrapper_invoked` sentinel; in GREEN the wrapper is bypassed (dispatch routes to `buildClaudeResumeCommand` → `GetClaudeCommand()` → stub with `--resume newerUUID`). Assertion order also changed: `ClaudeSessionID` check fires BEFORE `readCapturedClaudeArgv` so the RED diagnostic is guaranteed to surface.

**Also dropped:** ASSERT 4 (storage round-trip via `NewStorage` + `SaveWithGroups`) — plan explicitly marked this as best-effort and permitted demotion. ASSERT 3's in-memory `ClaudeSessionID` check is sufficient RED fidelity per D-02 (write-through onto Instance struct; external storage save cycle is Phase 5's responsibility, not TEST-09's).

## Other tests at end of plan 05-01

- `TestPersistence_CustomCommandResumesFromLatestJSONL` — RED (by design)
- `TestPersistence_TmuxSurvivesLoginSessionRemoval` — RED (pre-existing environmental flake from Phase 2 UAT; `invalid MainPID ""` in `startAgentDeckTmuxInUserScope` test helper, unchanged by this plan)
- All other 7 `TestPersistence_*` tests remain GREEN

Acceptance criteria satisfied:
- [x] `func TestPersistence_CustomCommandResumesFromLatestJSONL` present
- [x] `go vet ./internal/session/...` exits 0
- [x] per-test gate exits non-zero with `TEST-09 PERSIST-12 RED:` in stderr
- [x] `grep -c "^func TestPersistence_" internal/session/session_persistence_test.go` = 12 (11 prior + 1 new)
- [x] Commit trailer `Committed by Ashesh Goplani`; no Claude attribution
- [x] Only `internal/session/session_persistence_test.go` modified
