# Phase 8: Heartbeat & CLI Fixes - Research

**Researched:** 2026-03-07
**Domain:** Conductor heartbeat system, CLI flag parsing, session parent linkage
**Confidence:** HIGH

## Summary

Phase 8 addresses five concrete bugs/gaps across two domains: conductor heartbeat scoping (HB-01, HB-02) and CLI command reliability (CLI-01, CLI-02, CLI-03). All five issues are well-understood from codebase inspection. No new libraries are needed; all fixes are modifications to existing code in `internal/session/conductor.go` and `cmd/agent-deck/session_cmd.go` / `main.go`.

The heartbeat issues stem from the `conductorHeartbeatScript` constant (conductor.go:602-615) which sends a natural-language message instructing the conductor to "Check all sessions in the {PROFILE} profile" without group scoping, and from `GetHeartbeatInterval()` treating `heartbeat_interval = 0` as "use default 15" rather than "disabled." The CLI issues are in flag parsing (`reorderArgsForFlagParsing`), `waitForCompletion` exit-code handling, and the `--no-parent` / `set-parent` lifecycle.

**Primary recommendation:** Fix each bug at its source location with focused, isolated changes. No architectural refactoring needed.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| HB-01 | Heartbeat scripts filter sessions by the conductor's own group instead of reporting all sessions across all groups | Heartbeat script template at conductor.go:602-615 sends unscoped message. Fix: change message text to reference conductor's group name `{NAME}`, and optionally add `--group` flag to `session list` for machine-readable filtering |
| HB-02 | Heartbeat respects `conductor.enabled = false` and `heartbeat_interval = 0` by stopping launchd services or checking config before sending | `GetHeartbeatInterval()` at conductor.go:135 returns 15 when 0. Heartbeat.sh does not check `conductor.enabled`. Fix: add config check in heartbeat.sh and handle interval=0 as disabled |
| CLI-01 | `session send --wait` exits cleanly with correct status codes and does not hang on edge cases | `waitForCompletion` at session_cmd.go:1621 polls correctly but edge cases exist: session exits during processing (tmux session dies), infinite polling if status never leaves "active" due to tmux errors |
| CLI-02 | `-cmd` flag does not break `-group` flag parsing; `-c` shorthand is documented | `reorderArgsForFlagParsing` at main.go:519 handles `-c` in valueFlags map. The `-c` shorthand is defined at main.go:687. Potential collision: `-c` is shorthand for `--cmd` only, needs documentation clarity |
| CLI-03 | `--no-parent` followed by `set-parent` correctly restores parent routing, or `--no-parent` emits a clear warning | `--no-parent` is a creation-time flag (main.go:695). `set-parent` works independently (session_cmd.go:1117). No conflict exists because `--no-parent` only suppresses auto-parent detection during `add`. `set-parent` can always be called afterwards. Documentation gap: this behavior is not explained in help text |
</phase_requirements>

## Standard Stack

### Core (No New Dependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `flag` | 1.24+ | CLI flag parsing | Already used throughout `cmd/agent-deck/` |
| Go stdlib `os/exec` | 1.24+ | launchd/systemd commands | Already used in conductor.go |
| `internal/session` | local | Conductor settings, heartbeat scripts | Existing package |
| `internal/tmux` | local | Status polling for `--wait` | Existing package |
| `internal/send` | local | Send verification (Phase 7 output) | New package from Phase 7 |

### No New Packages Needed

All fixes are modifications to existing Go code and shell script templates. No `go get` or dependency changes required.

## Architecture Patterns

### Heartbeat Script Template Pattern

The heartbeat system uses a **template-and-install pattern**: a Go constant string (`conductorHeartbeatScript`) contains shell script with `{NAME}` and `{PROFILE}` placeholders. `InstallHeartbeatScript()` performs string replacement and writes the result to `~/.agent-deck/conductor/{NAME}/heartbeat.sh`. A launchd plist (or systemd timer) triggers the script periodically.

**Key file locations:**
```
internal/session/conductor.go:602    # conductorHeartbeatScript constant
internal/session/conductor.go:407    # InstallHeartbeatScript()
internal/session/conductor.go:431    # GenerateHeartbeatPlist()
internal/session/conductor.go:1806   # InstallHeartbeatDaemon()
internal/session/conductor.go:999    # MigrateConductorHeartbeatScripts()
```

**Migration concern:** When the script template changes, `MigrateConductorHeartbeatScripts()` auto-refreshes installed scripts on next `conductor setup` or `conductor status`. This is the existing migration mechanism. Changing the template constant automatically triggers re-deployment of all managed heartbeat scripts.

### CLI Flag Parsing Pattern

The `add` command uses a two-stage reorder:
1. `reorderArgsForFlagParsing()` moves path to end so flags parse correctly
2. `normalizeArgs()` moves all flags before positional args for Go's `flag` package

Both functions maintain a `valueFlags` map of flags that consume the next argument. The `-c`/`--cmd` flag is correctly listed in both maps.

### Wait-for-Completion Pattern

`waitForCompletion()` at session_cmd.go:1621 implements polling:
```
1. Initial 1s grace period
2. Poll GetStatus() every 2s
3. "active" = keep waiting
4. Any non-active status = return that status
5. Timeout after --timeout duration (default 10min)
```

Exit codes: `os.Exit(1)` for `finalStatus == "inactive" || "error"`, no explicit `os.Exit(0)` for success (implicit via normal return).

### Parent Linkage Pattern

`--no-parent` is a **creation-time-only** flag on the `add` command. It suppresses `resolveAutoParentInstance()` which auto-detects the parent from environment variables (`AGENT_DECK_SESSION_ID`, `AGENTDECK_INSTANCE_ID`, or tmux session). After creation, `set-parent` and `unset-parent` work independently regardless of how the session was created.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Heartbeat config checking in shell | Parsing TOML in bash | `agent-deck conductor status --json` output piped through `jq` | TOML parsing in bash is fragile; existing JSON output provides all needed fields |
| Custom group filtering | New filtering logic | Existing `session list --group` flag (if it exists) or heartbeat message text change | The heartbeat sends a natural-language instruction to Claude; the fix is the message text, not programmatic filtering |

## Common Pitfalls

### Pitfall 1: Heartbeat Migration Not Triggered
**What goes wrong:** Template change in Go constant doesn't update already-installed heartbeat.sh files
**Why it happens:** The script is installed once during `conductor setup`. Old installations keep the old text.
**How to avoid:** `MigrateConductorHeartbeatScripts()` already handles this. It compares installed script against expected template and refreshes if different. Runs on every `conductor setup` and `conductor status` call.
**Warning signs:** Old heartbeat messages still reporting all sessions after code change

### Pitfall 2: GetHeartbeatInterval Returning 15 for "Disabled"
**What goes wrong:** Setting `heartbeat_interval = 0` to disable heartbeat still results in 15-minute interval
**Why it happens:** `GetHeartbeatInterval()` treats `<= 0` as "use default 15"
**How to avoid:** Add a separate method like `IsHeartbeatDisabledByInterval()` that checks for 0 specifically, or change the semantics so callers check before calling `GetHeartbeatInterval()`
**Warning signs:** Heartbeat still fires after user sets interval to 0

### Pitfall 3: waitForCompletion Hanging When Session Dies
**What goes wrong:** `--wait` hangs indefinitely if the tmux session is killed during processing
**Why it happens:** `GetStatus()` may return error when tmux session doesn't exist, but the code just keeps polling on transient errors
**How to avoid:** Detect persistent GetStatus errors (e.g., 3+ consecutive errors) and treat as session death, returning "error" status
**Warning signs:** CLI process stuck after target session has exited

### Pitfall 4: Flag -c Collision Potential
**What goes wrong:** Users might expect `-c` to mean something else (e.g., `-c` for `--count` or `-c` for config)
**Why it happens:** Single-letter flag aliases are scarce; `-c` is claimed by `--cmd`
**How to avoid:** Document clearly in help text. The alias is already correctly defined in code, so the fix is documentation only.
**Warning signs:** User reports of unexpected behavior when using `-c`

### Pitfall 5: --no-parent Creates False Expectations
**What goes wrong:** User thinks `--no-parent` permanently prevents parent linkage
**Why it happens:** The flag name implies permanence, but it only affects the `add` command's auto-detection. `set-parent` can always link later.
**How to avoid:** Clarify in help text that `--no-parent` only suppresses auto-linking during creation, and `set-parent` can be used later
**Warning signs:** User confusion in issues/support

## Code Examples

### Current Heartbeat Script (Bug Source for HB-01)

```bash
# Source: internal/session/conductor.go:602-615
#!/bin/bash
# Heartbeat for conductor: {NAME} (profile: {PROFILE})
SESSION="conductor-{NAME}"
PROFILE="{PROFILE}"

STATUS=$(agent-deck -p "$PROFILE" session show "$SESSION" --json 2>/dev/null | tr -d '\n' | sed -n 's/.*"status"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')

if [ "$STATUS" = "idle" ] || [ "$STATUS" = "waiting" ]; then
    agent-deck -p "$PROFILE" session send "$SESSION" "Heartbeat: Check all sessions in the {PROFILE} profile. List any waiting sessions, auto-respond where safe, and report what needs my attention." --no-wait -q
fi
```

**Problem:** Message says "Check all sessions in the {PROFILE} profile" with no group scoping.

### Fix Pattern for HB-01: Group-Scoped Heartbeat

```bash
# Fixed version: scope to conductor's own group
agent-deck -p "$PROFILE" session send "$SESSION" "Heartbeat: Check sessions in the {NAME} group. List any that are waiting, auto-respond where safe, and report what needs my attention." --no-wait -q
```

### Fix Pattern for HB-02: Config Guard in Heartbeat Script

```bash
# Add config check before sending
ENABLED=$(agent-deck -p "$PROFILE" conductor status --json 2>/dev/null | tr -d '\n' | sed -n 's/.*"enabled"[[:space:]]*:[[:space:]]*\(true\|false\).*/\1/p')

if [ "$ENABLED" != "true" ]; then
    exit 0
fi
```

Or better: make `GetHeartbeatInterval()` return 0 when disabled, and check interval > 0 during `InstallHeartbeatDaemon()`.

### Fix Pattern for CLI-01: Session Death Detection in waitForCompletion

```go
// Source: cmd/agent-deck/session_cmd.go:1621
func waitForCompletion(checker statusChecker, timeout time.Duration) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    const pollInterval = 2 * time.Second
    consecutiveErrors := 0
    const maxConsecutiveErrors = 5

    time.Sleep(1 * time.Second)

    for {
        select {
        case <-ctx.Done():
            return "", fmt.Errorf("agent still running after %s", timeout)
        default:
        }

        status, err := checker.GetStatus()
        if err != nil {
            consecutiveErrors++
            if consecutiveErrors >= maxConsecutiveErrors {
                return "error", nil // Session likely died
            }
            time.Sleep(pollInterval)
            continue
        }
        consecutiveErrors = 0

        if status == "active" {
            time.Sleep(pollInterval)
            continue
        }

        return status, nil
    }
}
```

### Current --no-parent + set-parent Interaction

```go
// Source: cmd/agent-deck/main.go:838
} else if !*noParent {
    parentInstance = resolveAutoParentInstance(instances)
    // ...
}

// set-parent is completely independent: session_cmd.go:1117
// Works regardless of how session was created
func handleSessionSetParent(profile string, args []string) {
    // Resolves session, validates, sets ParentSessionID
    inst.SetParentWithPath(parentInst.ID, parentInst.ProjectPath)
}
```

**Current behavior is actually correct.** `--no-parent` suppresses auto-linking at creation. `set-parent` works anytime after. The issue is documentation, not functionality.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Heartbeat sends to all sessions | Bug: still sends to all sessions | Pre-v1.2 | HB-01 fix needed |
| interval=0 means disabled | Bug: interval=0 means "use default 15" | Pre-v1.2 | HB-02 fix needed |
| `--wait` always trusts GetStatus | Bug: hangs if session dies | Pre-v1.2 | CLI-01 fix needed |
| Single-letter `-c` for `--cmd` | Works correctly but underdocumented | v0.20+ | CLI-02 docs needed |
| `--no-parent` creation-time flag | Works correctly but underdocumented | v0.19+ | CLI-03 docs needed |

## Specific Fix Locations

### HB-01: Heartbeat Group Scoping

**File:** `internal/session/conductor.go`
**Line:** 613 (the heartbeat message text)
**Change:** Replace "Check all sessions in the {PROFILE} profile" with "Check sessions in your group ({NAME}). List any..."
**Side effect:** `MigrateConductorHeartbeatScripts()` will auto-refresh installed scripts

### HB-02: Heartbeat Disabled Configuration

**Files:**
1. `internal/session/conductor.go:135` - `GetHeartbeatInterval()`: Allow 0 to mean "disabled" instead of mapping to 15
2. `internal/session/conductor.go:602-615` - `conductorHeartbeatScript`: Add config-enabled check
3. `cmd/agent-deck/conductor_cmd.go:439-449` - `handleConductorSetup`: Skip heartbeat installation when interval=0

**Alternative approach (simpler):** Don't change `GetHeartbeatInterval()` semantics. Instead:
- When `conductor.enabled = false`, `conductor setup` already gates behind `!settings.Enabled` interactive prompt (line 185). The heartbeat plist is only installed after this check.
- When `heartbeat_interval = 0`, add an explicit check in `handleConductorSetup` before calling `InstallHeartbeatDaemon`.
- In the heartbeat script, add a check: query `conductor status --json` and exit if not enabled.

### CLI-01: --wait Exit Code and Hang Prevention

**File:** `cmd/agent-deck/session_cmd.go`
**Lines:** 1621-1654 (waitForCompletion function)
**Change:** Add consecutive error detection to treat persistent GetStatus failures as session death. Current timeout of 10 minutes is already configurable via `--timeout` flag.

### CLI-02: -cmd / -c Documentation

**Files:**
1. `cmd/agent-deck/main.go:686-687` - The `-c` shorthand is already defined correctly
2. `cmd/agent-deck/main.go:729-762` - Usage/help text: add note about `-c` being shorthand for `--cmd`

**Verification needed:** Test that `agent-deck add -g mygroup -c claude .` parses both flags correctly. The `reorderArgsForFlagParsing` at main.go:519 has `-c` and `-g` both in `valueFlags` map, so both flags and their values should be correctly identified and moved before the path argument.

### CLI-03: --no-parent + set-parent Documentation

**Files:**
1. `cmd/agent-deck/main.go:695` - The `--no-parent` flag definition: expand help text
2. `cmd/agent-deck/main.go:751` - Usage examples: add example showing `--no-parent` then `set-parent`
3. `cmd/agent-deck/session_cmd.go:1125-1131` - `set-parent` help text: mention that this works for sessions created with `--no-parent`

**The actual behavior is correct:** `--no-parent` is creation-time only. `set-parent` always works. The fix is documentation/help text, not code logic.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing + `go test -race` |
| Config file | Per-package TestMain files (4 locations) |
| Quick run command | `go test -race -v -run TestName ./package/...` |
| Full suite command | `go test -race -v ./...` |

### Phase Requirements Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| HB-01 | Heartbeat script contains group name, not "all sessions" | unit | `go test -race -v -run TestConductorHeartbeatScript ./internal/session/...` | Partially (TestConductorHeartbeatScript_StatusParsingHandlesWhitespace exists, needs group-scoping assertion) |
| HB-02 | GetHeartbeatInterval returns 0 when disabled; heartbeat script checks config | unit | `go test -race -v -run TestHeartbeat ./internal/session/...` | Wave 0 gap |
| CLI-01 | waitForCompletion exits on session death, returns correct status | unit | `go test -race -v -run TestWaitForCompletion ./cmd/agent-deck/...` | Partially (5 tests exist, need session-death test) |
| CLI-02 | `-cmd` and `-group` parsed together; `-c` documented | unit | `go test -race -v -run TestReorderArgs ./cmd/agent-deck/...` | Wave 0 gap |
| CLI-03 | `--no-parent` help text mentions `set-parent` recovery | unit | `go test -race -v -run TestNoParent ./cmd/agent-deck/...` | Wave 0 gap (documentation, may be manual-only) |

### Sampling Rate

- **Per task commit:** `go test -race -v ./cmd/agent-deck/... ./internal/session/...`
- **Per wave merge:** `go test -race -v ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/session/conductor_test.go` - Test that `conductorHeartbeatScript` contains group-scoped message (not "all sessions in the profile")
- [ ] `internal/session/conductor_test.go` - Test `GetHeartbeatInterval` returns 0 when input is 0 (after fix), and test heartbeat script contains config-enabled check
- [ ] `cmd/agent-deck/session_send_test.go` - Test `waitForCompletion` handles consecutive GetStatus errors (session death scenario)
- [ ] `cmd/agent-deck/cli_utils_test.go` or `add_test.go` - Test `reorderArgsForFlagParsing` with `-c claude -g mygroup .` produces correct output
- [ ] `cmd/agent-deck/main_test.go` - Test `--no-parent` help text content (may be manual verification)

## Open Questions

1. **Heartbeat message specificity**
   - What we know: The heartbeat message is natural language sent to a Claude conductor session
   - What's unclear: Should the message reference a group path or just the conductor name? Groups are path-based (e.g., "conductor/ryan")
   - Recommendation: Use the conductor name (`{NAME}`) which maps to the group. The conductor's CLAUDE.md already knows its group.

2. **Should heartbeat_interval=0 disable or use default?**
   - What we know: Currently returns 15 for any value <= 0
   - What's unclear: Is there a semantic difference between "not set" (TOML zero value) and "explicitly set to 0"?
   - Recommendation: Treat 0 as "disabled" since TOML zero value for int is 0 and the default profile (`heartbeat_interval` absent) should fall back to 15 through a different mechanism (e.g., check if the key exists in the raw TOML). Alternatively, keep existing `<= 0 -> 15` behavior and add a separate `heartbeat_disabled` boolean field. Simplest approach: interpret 0 as disabled, negative as "use default 15."

3. **CLI-02: Is there an actual bug or just documentation gap?**
   - What we know: `-c` and `-g` are correctly in `valueFlags` map. `reorderArgsForFlagParsing` handles them.
   - What's unclear: Has an actual parsing failure been observed?
   - Recommendation: Write a test to confirm correct parsing, then focus on documentation. If test reveals a bug, fix it.

## Sources

### Primary (HIGH confidence)

- `internal/session/conductor.go` - Heartbeat script template, interval handling, daemon installation (lines 602-615, 134-140, 407-424, 431-470, 1806-1883)
- `cmd/agent-deck/session_cmd.go` - `--wait` implementation, `set-parent` / `unset-parent` handlers (lines 1296-1443, 1117-1294, 1614-1654)
- `cmd/agent-deck/main.go` - `handleAdd`, `reorderArgsForFlagParsing`, `--no-parent` flag, `resolveAutoParentInstance` (lines 519-566, 680-845, 631-652)
- `cmd/agent-deck/cli_utils.go` - `normalizeArgs` function (lines 18-58)
- `cmd/agent-deck/conductor_cmd.go` - `handleConductorSetup` heartbeat installation (lines 439-449)
- `internal/session/instance.go` - `SetParentWithPath`, `ClearParent`, `IsSubSession` (lines 286-311)

### Secondary (HIGH confidence)

- `cmd/agent-deck/session_send_test.go` - Existing test coverage for waitForCompletion and sendWithRetry (6 waitForCompletion tests, 7 sendWithRetry tests)
- `internal/session/conductor_test.go` - Existing test for heartbeat script content (TestConductorHeartbeatScript_StatusParsingHandlesWhitespace)
- Phase 7 verification report - Confirms send path refactoring completed, heartbeat send path uses `--no-wait -q`

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - No new dependencies, all existing code
- Architecture: HIGH - All fix locations identified with exact line numbers
- Pitfalls: HIGH - All issues traced to specific code patterns with clear fixes

**Research date:** 2026-03-07
**Valid until:** 2026-04-07 (stable codebase, no external dependency changes expected)
