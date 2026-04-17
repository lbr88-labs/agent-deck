# Fix #601 — CLI `launch -c` with extra args silently drops flags due to bash -c wrapping order

Task ID: `fix-issue-601`
Target release: **v1.7.11**
Reporter: @christophercolumbusdog (issue #601 — clear reproducer, correct root-cause analysis, correct suggested fix)

---

## 1. Problem summary

`agent-deck launch <path> -c "tool --extra-flag1 --extra-flag2"` loses `--extra-flag1` and `--extra-flag2`. They never reach the child process (Claude, codex, or custom tool). The reporter demonstrated this with `--session-id` — a flag that is silently dropped, giving no user-visible error.

## 2. Exact reproducer (programmatic)

```bash
TEST_UUID=aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
agent-deck launch /tmp -t test-601 \
  -c "codex --dangerously-bypass-approvals-and-sandbox --session-id $TEST_UUID" \
  --json

# Observe the resulting pane_start_command:
tmux_name=$(agent-deck session show test-601 --json | jq -r .tmux_session)
tmux show-options -g -t "$tmux_name" | grep start-command   # or inspect argv directly
ps -o args= $(pgrep -f "agentdeck_test-601")
```

Observable outcome on `main` (v1.7.10):
- `ps` shows `bash -c 'codex' --dangerously-bypass-approvals-and-sandbox --session-id aaaa...`
  → bash swallows the trailing flags as positional parameters.
- `codex` starts with no flags, `--session-id` is lost.

Observable outcome on fix branch:
- `ps` shows `bash -c 'codex --dangerously-bypass-approvals-and-sandbox --session-id aaaa...'`
  → flags are inside the quoted single argv to bash, preserved through exec.

## 3. DATA-FLOW TRACE

```
[USER INPUT]
agent-deck launch /tmp -c "codex --dangerously-bypass-approvals-and-sandbox --session-id UUID"
      │
      ▼
[CLI PARSE: cmd/agent-deck/launch_cmd.go:129]
resolveSessionCommand(rawCommand="codex --dang... --session-id UUID", explicitWrapper="")
      │  (cmd/agent-deck/cli_utils.go:78-111)
      │  - detectTool() → "codex"
      │  - splitFirstWord() → base="codex", extra="--dang... --session-id UUID"
      │  - Since tool != shell + extra != "":
      │      tool     = "codex"
      │      command  = toolDef.Command                          // usually "codex"
      │      wrapper  = "{command} --dang... --session-id UUID"  // extras folded here
      ▼
[INSTANCE CONSTRUCTION: cmd/agent-deck/launch_cmd.go]
inst := NewInstance(title, path)
inst.Tool    = "codex"
inst.Command = "codex"
inst.Wrapper = "{command} --dang... --session-id UUID"
      │
      ▼  (persisted; state.json is source of truth going forward)
[START: internal/session/instance.go ~line 2067]
command = buildCodexCommand(i.Command)          // e.g. "codex --resume <sid>" or just "codex"
command, containerName, _ = i.prepareCommand(command)
      │
      ▼
[PREPARE COMMAND — THE BUG: internal/session/instance.go:5282-5309]
func (i *Instance) prepareCommand(cmd) {
  if i.hasEffectiveWrapper() {                  // TRUE (wrapper set)
    escaped := replace(cmd, "'", "'\"'\"'")
    cmd = "bash -c '" + escaped + "'"           // (A) cmd = "bash -c 'codex'"
  }
  wrapped, _ := i.applyWrapper(cmd)             // (B) substitute {command}
  // Result: "bash -c 'codex' --dang... --session-id UUID"  ← TRAILING ARGS OUTSIDE QUOTES
  ...
}
      │
      ▼
[TMUX START: internal/session/instance.go:2085 → internal/tmux/tmux.go:1400]
s.Start(command)  where command = "bash -c 'codex' --dang... --session-id UUID"
      │
      ▼
[START COMMAND SPEC: internal/tmux/tmux.go:831-844]
startCommandSpec(workDir, command) {
  if s.RunCommandAsInitialProcess {               // TRUE for non-shell tools
    if isBashCWrapped(command) {                  // TRUE (starts with "bash -c '")
      args = append(args, command)                // passed AS-IS to tmux new-session
    }
  }
}
      │
      ▼
[LIVE BOUNDARY: tmux runtime]
tmux new-session -d -s agentdeck_test-601_xxxx -c /tmp \
  "bash -c 'codex' --dang... --session-id UUID"
      │
      ▼
[tmux invokes /bin/sh -c <command>]
/bin/sh tokenizes:  argv to bash = ['codex', '--dang...', '--session-id', 'UUID']
bash -c 'codex': runs literal string "codex" as the script;
                 ['--dang...', '--session-id', 'UUID'] become $0, $1, $2.
                 codex process never sees the flags.   ← FLAGS DROPPED
```

### The fix (matches reporter's suggestion)

Reorder so `applyWrapper` runs FIRST (producing `"codex --dang... --session-id UUID"`), THEN `bash -c '…'` wraps the *entire* resulting string:

```
bash -c 'codex --dang... --session-id UUID'   ← single quoted argv, all flags inside
```

When tmux runs this via `/bin/sh -c`, bash sees `'codex --dang... --session-id UUID'` as its script, interprets the flags correctly, and `codex` receives them.

## 4. Failing tests (committed FIRST, before the fix)

### T1. Unit: `TestPrepareCommand_AppliesWrapperBeforeBashWrap`
**File:** `internal/session/instance_test.go`
**Asserts:** For an instance with `Wrapper = "{command} --extra1 --extra2"` and `cmd = "tool"`, `prepareCommand(cmd)` returns a string where:
- The trailing extra flags are INSIDE the `bash -c '…'` quotes (not outside them).
- Specifically: result matches exactly `bash -c 'tool --extra1 --extra2'`.

### T2. Unit: `TestPrepareCommand_WrapperWithShellMetachars_QuotesSafely`
**File:** `internal/session/instance_test.go`
**Asserts:** For `cmd = "tool"` containing a single quote, the result correctly escapes with the POSIX `'\''` pattern inside the bash -c payload — no broken quoting.

### T3. CLI-level: `TestLaunch_ToolWithFlags_PreservesFlagsInStartCommand`
**File:** `cmd/agent-deck/launch_cmd_test.go` (new file)
**Asserts:** When we invoke the CLI path that builds the Instance + runs prepareCommand for a tool with extra `-c` args, the string ultimately handed to `tmuxSession.Start` contains the trailing flags INSIDE the bash -c quotes. Pins the live-boundary shape.

### T4. Unit: `TestPrepareCommand_NoWrapper_Unchanged`
**File:** `internal/session/instance_test.go`
**Asserts:** Regression guard — when no wrapper is configured, `prepareCommand` returns `cmd` unchanged (no unexpected bash-c wrap).

## 5. Implementation sketch

Single-file change, minimal surface:

**File:** `internal/session/instance.go` function `prepareCommand` (~line 5282).

```go
func (i *Instance) prepareCommand(cmd string) (string, string, error) {
    // Apply the user wrapper FIRST so trailing args in the wrapper template
    // (e.g. "{command} --flag") are part of the command string that will be
    // bash-c wrapped below. Previously the order was reversed and any extra
    // args after {command} became bash positional parameters ($0/$1/...) and
    // were silently dropped. See issue #601.
    wrapped, err := i.applyWrapper(cmd)
    if err != nil {
        return "", "", err
    }

    // Always wrap under bash -c when a wrapper is configured. Wrappers may be
    // delivered through exec paths (docker exec, ssh) where the command text
    // is parsed by a shell; quoting under bash -c keeps shell metacharacters
    // in the base command from leaking out to the outer shell.
    if i.hasEffectiveWrapper() {
        escaped := strings.ReplaceAll(wrapped, "'", "'\"'\"'")
        wrapped = fmt.Sprintf("bash -c '%s'", escaped)
    }

    wrapped = i.wrapForSSH(wrapped)
    wrapped, containerName, err := i.wrapForSandbox(wrapped)
    if err != nil {
        return "", "", err
    }
    if wrapped != "" && i.IsSandboxed() {
        wrapped = wrapIgnoreSuspend(wrapped)
    }
    return wrapped, containerName, nil
}
```

## 6. Scope boundaries

**Files that MAY change:**
- `internal/session/instance.go` — the fix (single function body).
- `internal/session/instance_test.go` — add T1, T2, T4.
- `cmd/agent-deck/launch_cmd_test.go` — new file, T3.
- `.claude/release-tests.yaml` — append regression entries (Phase 8a).
- `cmd/agent-deck/main.go` — version bump to 1.7.11 (Phase 8c, release-only).

**Files that MUST NOT change:**
- `internal/tmux/**` — the bashCWrap/isBashCWrapped primitives stay as-is.
- `cmd/agent-deck/cli_utils.go` — `resolveSessionCommand` already correctly produces the `{command} <extras>` wrapper shape.
- Anything in watcher, session persistence, feedback, or sandbox paths.

## 7. Live-boundary verification (Phase 7)

After impl, run reporter's repro 5 consecutive times:
```bash
for i in 1 2 3 4 5; do
  ./bin/agent-deck launch /tmp -t "test-601-$i" -g test \
    -c "codex --dangerously-bypass-approvals-and-sandbox --session-id $(uuidgen)"
  # inspect the pane_start_command / process argv
  pid=$(pgrep -f "agentdeck_test-601-$i" | head -1)
  ps -o args= "$pid" | grep -q "'--session-id" || echo "FAIL on run $i"
done
```
All 5/5 must have flags present inside bash-c quotes.

## 8. Invariants (Phase 7 checklist)

- [x] Data-flow trace enumerates every hop.
- [x] Every hop has either a test or a boundary inspection.
- [x] No out-of-scope file changes proposed.
- [x] Reporter's suggested fix independently verified against the docker-exec edge case (documented under §9).

## 9. Edge case considered: wrapper + shell metacharacters in base cmd

Old order: `bash -c '<cmd>'` then substitute → `docker exec mycon bash -c '<cmd>'`.
New order: substitute then `bash -c '<full>'` → `bash -c 'docker exec mycon <cmd>'`.

For `cmd = "claude && other"` under wrapper `docker exec mycon {command}`:
- Old: `&&` evaluated INSIDE container (bash inside docker).
- New: `&&` evaluated in the OUTER bash (outside docker).

In practice, the `cmd` fed to `prepareCommand` is constructed by `buildClaudeCommand`/`buildCodexCommand`/`buildGenericCommand` — all produce clean argv-like strings (`"claude --resume <id>"`, `"codex --resume <id>"`, etc.) without shell control operators. No real-world caller passes `&&`/`$()` into `cmd` at this stage. The semantic change is therefore theoretical, and the fix for the #601 class of bugs (real-world, user-reported) outweighs it.

If a future use case surfaces this edge, the correct solution is to escape the substitution point in `applyWrapper`, not to preserve the old ordering.
