---
phase: 08-heartbeat-cli-fixes
verified: 2026-03-07T12:00:00Z
status: passed
score: 9/9 must-haves verified
---

# Phase 08: Heartbeat CLI Fixes Verification Report

**Phase Goal:** Fix heartbeat group scoping, interval=0 semantics, CLI --wait death detection, flag co-parsing, and help text improvements.
**Verified:** 2026-03-07
**Status:** passed
**Re-verification:** No (initial verification)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Heartbeat script message references the conductor's own group name, not "all sessions in the profile" | VERIFIED | `conductor.go:625`: message says `Check sessions in your group ({NAME})`, no mention of "all sessions in the {PROFILE} profile" |
| 2 | GetHeartbeatInterval returns 0 when heartbeat_interval is set to 0, signaling disabled | VERIFIED | `conductor.go:138-146`: `HeartbeatInterval == 0` returns 0; negative returns 15; positive returns configured value |
| 3 | Conductor setup skips heartbeat daemon installation when interval is 0 | VERIFIED | `conductor_cmd.go:441-455`: `if interval <= 0` block prints skip message and bypasses InstallHeartbeatScript/InstallHeartbeatDaemon |
| 4 | Heartbeat script checks conductor enabled status before sending | VERIFIED | `conductor.go:615-619`: ENABLED guard queries `conductor status --json`, exits if not "true" |
| 5 | MigrateConductorHeartbeatScripts auto-refreshes installed scripts to the new template | VERIFIED | `conductor.go:1013-1058`: generates expected from `conductorHeartbeatScript` constant, compares to disk, overwrites if different |
| 6 | waitForCompletion detects session death (persistent GetStatus errors) and returns "error" status instead of hanging | VERIFIED | `session_cmd.go:1632-1646`: `consecutiveErrors` counter with threshold of 5, returns `("error", nil)` on breach |
| 7 | Using -c and -g flags together in agent-deck add parses both correctly | VERIFIED | `cli_utils_test.go:214-250`: `TestReorderArgsForFlagParsing_CmdAndGroup` with 4 table-driven cases covering all orderings |
| 8 | --no-parent help text explains that set-parent can restore parent linking after creation | VERIFIED | `main.go:695`: help string includes `(use 'session set-parent' later to link manually)` |
| 9 | --wait returns exit code 0 for success, 1 for error/inactive/session-death | VERIFIED | `session_cmd.go:1439-1441`: `if finalStatus == "inactive" \|\| finalStatus == "error" { os.Exit(1) }` |

**Score:** 9/9 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/session/conductor.go` | Group-scoped heartbeat script template, interval=0 means disabled | VERIFIED | Contains `Check sessions in` (line 625), `ENABLED` guard (line 616), `HeartbeatInterval == 0` returns 0 (line 139) |
| `internal/session/conductor_test.go` | Tests for heartbeat group scoping and interval=0 semantics | VERIFIED | `TestConductorHeartbeatScript_GroupScoped` (line 1882), `TestGetHeartbeatInterval_ZeroMeansDisabled` (line 1907) |
| `cmd/agent-deck/conductor_cmd.go` | Skip heartbeat installation when interval is 0 | VERIFIED | `interval > 0` guard at line 442 |
| `cmd/agent-deck/session_cmd.go` | Session death detection in waitForCompletion | VERIFIED | `consecutiveErrors` counter at line 1632, threshold check at line 1645 |
| `cmd/agent-deck/session_send_test.go` | Test for session death scenario | VERIFIED | `TestWaitForCompletion_SessionDeath` (line 101), `TestWaitForCompletion_TransientRecovery` (line 125) |
| `cmd/agent-deck/main.go` | Improved help text for --no-parent and -c flags | VERIFIED | `set-parent` in --no-parent help (line 695), `-c is shorthand for --cmd` example (line 751) |
| `cmd/agent-deck/cli_utils_test.go` | Test for -c and -g flag co-parsing | VERIFIED | `TestReorderArgsForFlagParsing_CmdAndGroup` (line 214) with 4 table-driven cases |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `conductor.go:conductorHeartbeatScript` | `conductor.go:InstallHeartbeatScript` | template string replacement | WIRED | `strings.ReplaceAll(conductorHeartbeatScript, "{NAME}", name)` at line 422 |
| `conductor.go:GetHeartbeatInterval` | `conductor_cmd.go:handleConductorSetup` | interval check before daemon install | WIRED | `settings.GetHeartbeatInterval()` at line 441, guard `if interval <= 0` at line 442 |
| `session_cmd.go:waitForCompletion` | `session_cmd.go:handleSessionSend` | called when --wait flag is set | WIRED | `waitForCompletion(tmuxSess, *timeout)` at line 1405 |
| `main.go:reorderArgsForFlagParsing` | `main.go:handleAdd` | reorders args before flag.Parse | WIRED | `reorderArgsForFlagParsing(args)` at line 768 |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| HB-01 | 08-01 | Heartbeat scripts filter sessions by the conductor's own group | SATISFIED | Script template uses `{NAME}` group scoping, TUI heartbeat also updated (home.go:2178) |
| HB-02 | 08-01 | Heartbeat respects `conductor.enabled = false` and `heartbeat_interval = 0` | SATISFIED | ENABLED guard in script template, GetHeartbeatInterval(0) returns 0, setup skips daemon when interval=0 |
| CLI-01 | 08-02 | `session send --wait` exits cleanly with correct status codes and does not hang on edge cases | SATISFIED | consecutiveErrors death detection returns "error" status, which maps to exit code 1 |
| CLI-02 | 08-02 | Using `-cmd` flag does not break `-group` flag parsing; `-c` shorthand is documented | SATISFIED | 4 table-driven test cases verify -c/-g co-parsing; -c shorthand documented in usage examples |
| CLI-03 | 08-02 | `--no-parent` followed by `set-parent` correctly restores parent routing | SATISFIED | --no-parent help mentions set-parent recovery; set-parent help mentions --no-parent compatibility |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/ui/home.go` | 754 | TODO comment about watched dirs | Info | Pre-existing, unrelated to phase 08 |
| `internal/ui/home.go` | 2174 | `_ = session.DefaultProfile` (unused variable suppression) | Info | Pre-existing vet fix from plan 02 deviation; variable is suppressed, not a stub |

No blocker or warning-level anti-patterns found in phase 08 modified files.

### Human Verification Required

### 1. Heartbeat Script Execution

**Test:** Set up a conductor with `heartbeat_interval = 0`, run `agent-deck conductor setup`, observe output.
**Expected:** Should print `[skip] Heartbeat disabled (interval = 0)` and not install a heartbeat daemon.
**Why human:** Requires live tmux environment and real conductor setup flow.

### 2. Heartbeat Script Enabled Guard

**Test:** Disable conductor via config, then manually run the installed heartbeat.sh script.
**Expected:** Script should exit silently without sending any message to the conductor session.
**Why human:** Requires real conductor status JSON endpoint and running tmux session.

### 3. Session Death Detection End-to-End

**Test:** Start a session with `--wait`, then kill the tmux session externally during processing.
**Expected:** CLI should exit with code 1 within approximately 10 seconds (5 polls x 2s), not hang indefinitely.
**Why human:** Requires real tmux session lifecycle and timing verification.

### Gaps Summary

No gaps found. All 9 observable truths verified, all 7 artifacts pass three-level verification (exists, substantive, wired), all 4 key links confirmed, all 5 requirements satisfied. Build succeeds. Tests pass (`go test -race` for both `internal/session/...` and `cmd/agent-deck/...`).

---

_Verified: 2026-03-07_
_Verifier: Claude (gsd-verifier)_
