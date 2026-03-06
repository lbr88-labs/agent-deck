# Exit 137 (SIGKILL) Root Cause Investigation

## Summary

Exit 137 (SIGKILL) occurs when new user input arrives at a Claude Code session while a Bash tool child process is still executing. The SIGKILL does **not** originate from tmux or agent-deck. It originates from Claude Code's own process management: when new text is injected into the composer (via tmux send-keys or manual typing), Claude Code interprets this as user intent to interrupt the current operation and kills the running Bash tool's child process with SIGKILL (signal 9). Agent-deck cannot prevent this because tmux send-keys is functionally identical to a human typing, and Claude Code's interrupt-on-input behavior is by design. The primary mitigation is to never send messages to a session that has status "active" (i.e., a tool is running).

## Reproduction Steps

1. Start a Claude Code session (via agent-deck or directly in tmux).
2. Ask Claude to run a long-lived Bash command, e.g., `sleep 300` or a script that takes minutes.
3. While the Bash tool is executing (spinner visible, status = "active"), inject text into the tmux pane:
   ```bash
   tmux send-keys -l -t <session-name> -- "hello"
   tmux send-keys -t <session-name> Enter
   ```
4. Observe: the running Bash tool process receives SIGKILL and exits with code 137 (128 + 9).
5. Claude Code shows the tool output was interrupted and begins processing the new input.

**Control test (raw shell, no Claude Code):**
1. In a plain tmux session running `sleep 300`.
2. `tmux send-keys -l -t <session-name> -- "hello"` followed by `tmux send-keys -t <session-name> Enter`.
3. The `sleep` process is **not** killed. The text appears as input after sleep finishes (or is buffered by the shell).
4. This confirms: tmux send-keys does not send signals. The SIGKILL comes from the application layer (Claude Code), not the terminal layer (tmux).

## Signal Chain Analysis

The full path from agent-deck to the killed process:

```
agent-deck session send <id> <message>
  |
  v
handleSessionSend() [cmd/agent-deck/session_cmd.go:1299]
  |-- waitForAgentReady()  (waits for "waiting" status)
  |-- sendWithRetry()
  |     |-- target.SendKeysAndEnter(message)  [session_cmd.go:1475]
  |           |
  |           v
  |     Session.SendKeysAndEnter() [internal/tmux/tmux.go:3039]
  |       |-- SendKeysChunked(keys)  (tmux send-keys -l -t <name> -- <text>)
  |       |-- time.Sleep(100ms)      (bracketed paste delay)
  |       |-- SendEnter()            (tmux send-keys -t <name> Enter)
  |           |
  |           v
  |     tmux server receives send-keys command
  |       |-- Writes bytes into the PTY master fd for the target pane
  |       |-- For -l flag: wraps in bracketed paste (\e[200~...\e[201~) on tmux 3.2+
  |       |-- For Enter: writes \r (carriage return) byte
  |       |-- NO SIGNALS ARE SENT. This is pure PTY I/O.
  |           |
  |           v
  |     PTY slave fd (Claude Code's stdin)
  |       |-- Node.js process reads bytes from its PTY
  |       |-- Claude Code's Ink TUI framework processes the input
  |       |-- Input arrives at the composer/input area
  |           |
  |           v
  |     Claude Code internal message handling
  |       |-- Detects new user input while a Bash tool is running
  |       |-- Decides to interrupt the current tool execution
  |       |-- Sends SIGKILL (signal 9) to the Bash tool child process
  |       |-- Child process exits with code 137 (128 + 9)
  |       |-- Claude Code reports the tool was interrupted
  |       |-- Begins processing the new user input
```

### Evidence

**1. tmux send-keys does not send signals (code analysis + tmux documentation):**

`tmux send-keys` operates purely at the PTY I/O level. It writes bytes into the PTY master file descriptor. The `-l` (literal) flag causes tmux to wrap the content in bracketed paste escape sequences on tmux 3.2+ (`\e[200~...\e[201~`). The `Enter` key name is translated to a carriage return (`\r`) byte. At no point does tmux send any Unix signal (SIGINT, SIGKILL, SIGTERM, etc.) to any process. This is confirmed by:

- Agent-deck's `SendKeys()` implementation at `internal/tmux/tmux.go:3018-3024`: it calls `exec.Command("tmux", "send-keys", "-l", "-t", s.Name, "--", keys)`. This is a pure data-plane operation.
- Agent-deck's `SendEnter()` at `tmux.go:3028-3031`: writes the Enter key name, which tmux translates to `\r`.
- The control test above: a raw shell does not kill `sleep` when send-keys injects text.

**2. Claude Code kills Bash tool children on new input (production evidence):**

From conductor LEARNINGS data across multiple conductors:

- `ryan/LEARNINGS.md [20260224-012]`: "Every agent-deck CLI command got killed with exit 137 (SIGKILL). Root cause: new messages arriving while a Bash tool runs cause Claude Code to interrupt and kill the child process." Recurrence: 10+.
- `ryan/LEARNINGS.md [earlier entry]`: "commands got killed (137) by incoming messages during sleep" when using `sleep 60 && agent-deck session output` polling loops.
- The behavior is consistent: any text arriving at the composer while a Bash tool runs triggers Claude Code to kill the child process.

**3. Agent-deck's send path is equivalent to human typing:**

The `sendWithRetry` function (`session_cmd.go:1448`) calls `SendKeysAndEnter`, which calls `tmux send-keys -l` followed by `tmux send-keys Enter`. This is byte-for-byte identical to a human typing in the tmux pane. The `waitForAgentReady` function (`session_cmd.go:1557`) waits for status "waiting" or "idle" before sending, but this only ensures Claude Code is at the composer prompt. It does NOT protect against races where another tool starts running between the readiness check and the actual send.

**4. Claude Code's interrupt-on-input is by design:**

This behavior mirrors what happens in the interactive Claude Code TUI when a user types while a tool is running: Claude Code interrupts the tool and processes the new input. This is a deliberate UX choice, not a bug. It allows users to cancel long-running operations by typing. The interrupt mechanism uses SIGKILL (not SIGTERM or SIGINT), which means the child process has no opportunity to handle or ignore the signal.

## Root Cause

**Component responsible: Claude Code (Anthropic's CLI application)**

Claude Code's Node.js process monitors its PTY input while Bash tool children are executing. When new characters arrive at the composer (whether from a human typing or from tmux send-keys injecting bytes), Claude Code interprets this as a user interrupt request and sends SIGKILL to the running Bash tool's child process group.

This is not a bug in agent-deck or tmux. It is Claude Code's designed behavior for handling concurrent user input during tool execution.

**Why SIGKILL (signal 9) specifically:**
- SIGKILL cannot be caught, blocked, or ignored by the child process
- This ensures immediate termination regardless of what the child process is doing
- The exit code 137 = 128 + 9 is the kernel's standard encoding for "killed by signal 9"

## Fixability Determination

**Can agent-deck fix the root cause? NO.**

The SIGKILL originates inside Claude Code's process management layer. Agent-deck has no ability to:
1. Modify Claude Code's signal handling behavior
2. Prevent Claude Code from monitoring PTY input during tool execution
3. Queue messages outside the PTY (there is no Claude Code API for message injection)

The only communication channel between agent-deck and Claude Code is the tmux PTY, which is indistinguishable from human keyboard input.

**Could tmux-level changes help? NO.**

tmux send-keys already operates at the lowest possible level (PTY byte injection). There is no "gentler" way to deliver text to the PTY. The problem is not how the text arrives but how Claude Code reacts to it.

**Could agent-deck mitigate by timing sends differently? YES (partial).**

Agent-deck already implements `waitForAgentReady()` which waits for status "waiting" before sending. This is the correct approach. However, there is a residual race window:

1. Agent-deck checks status: "waiting" (Claude Code is at the composer)
2. User or conductor sends a message via agent-deck
3. Claude Code accepts the message and starts a Bash tool (status transitions to "active")
4. Before the tool completes, ANOTHER message arrives via agent-deck
5. Claude Code sees new PTY input during tool execution and SIGKILLs the child

The current `waitForAgentReady` correctly prevents step 2 from happening during an active tool. The problem occurs at step 4: a second message sent while the first message's tool execution is still in progress. The default `sendWithRetry` path already waits for readiness, so this scenario primarily affects `--no-wait` sends or conductor systems that send rapid messages.

## Mitigation Strategies

Since the root cause cannot be fixed in agent-deck, these operational mitigations reduce the impact:

### 1. Never send to sessions with status "active" (ALREADY IMPLEMENTED)

`waitForAgentReady()` at `session_cmd.go:1557` already waits for "waiting" or "idle" status before injecting text. This prevents the most common failure scenario (sending while a previous message is being processed). Conductors should always use the default send path (without `--no-wait`) to benefit from this protection.

### 2. Use `--wait` flag for sequential message delivery

The `--wait` flag (`session_cmd.go:1404`) blocks until Claude Code finishes processing and returns to "waiting" status. For conductor workflows that send multiple sequential messages, using `--wait` ensures each message completes before the next is sent:

```bash
agent-deck session send <id> "first task" --wait
agent-deck session send <id> "second task" --wait
```

This eliminates the race between consecutive sends.

### 3. Keep Bash tool commands short-lived

From `ryan/LEARNINGS.md [20260224-012]`: "Keep CLI commands short. Don't rely on long-running commands. Use events instead of sleep+poll."

Short commands reduce the time window during which an incoming message can trigger SIGKILL. Instead of `sleep 60 && check_status`, use event-driven patterns or quick one-shot commands.

### 4. Avoid polling loops inside Claude Code sessions

Long-running `sleep + poll` patterns in Bash tools are especially vulnerable because they occupy the Bash tool slot for extended periods. Any incoming message during the sleep will kill the entire pipeline. Move polling logic to external systems (conductor heartbeat, file watchers) rather than running them as Bash tools inside Claude Code.

### 5. Conductor design: single-message-at-a-time discipline

Conductor orchestration should enforce that only one message is in-flight to a session at any time. The `--wait` flag naturally provides this. Rapid-fire sends (even with `waitForAgentReady`) can still race if the check-to-send window overlaps with tool startup.

### 6. Accept and handle exit 137 gracefully

Since SIGKILL is unavoidable in some edge cases, conductor systems should treat exit 137 as a retriable condition rather than a fatal error. If a command exits 137, the conductor can re-send the message after the session returns to "waiting" status.

---

**Investigation completed:** 2026-03-07
**Evidence sources:** Code analysis of internal/tmux/tmux.go, cmd/agent-deck/session_cmd.go, internal/send/send.go; production LEARNINGS from ryan conductor (10+ recurrences); tmux 3.6a send-keys documentation; control test methodology.
