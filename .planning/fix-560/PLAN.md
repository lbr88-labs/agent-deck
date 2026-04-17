# PLAN — fix #560: Having issues detaching within Tmux

## Problem summary

Issue reporter (shorea) ran `agent-deck` inside an existing tmux session and
got surprising detach behavior. Community contributor DieracDelta correctly
answered that agent-deck is designed to be run OUTSIDE tmux. The TUI already
blocks launch inside an *agent-deck-managed* tmux session (`isNestedSession()`
check in `main.go:313`), but there is NO guidance for users who launch
agent-deck inside a *generic* (non-agentdeck) tmux session — it just boots the
TUI, and then Ctrl+Q detach lands them back in the outer tmux instead of
returning them to a clean shell, which is confusing.

## Reproducer

```bash
tmux new -s myouter
# inside tmux:
agent-deck
# → TUI boots with no warning, no indication that detach semantics will be
#   surprising. Ctrl+Q from an attached session returns to TUI; quitting the
#   TUI returns to the outer tmux, not a clean shell.
```

## Data-flow trace (parallel paths audit)

Startup code path for `agent-deck` (no args, TUI launch):

1. `cmd/agent-deck/main.go:main()` — line 186
2. → `extractProfileFlag(os.Args[1:])` — splits profile flag from args
3. → subcommand dispatch (if args[0] matches a known command, handled and returns)
4. → `reviveOnStartup(profile)` — background reviver
5. → **`isNestedSession()` guard** — blocks launch inside agentdeck-managed tmux
   - In-scope: this guard handles `agentdeck_*` tmux sessions ONLY
   - **MISSING**: guard for *generic* tmux sessions (TMUX env set, but not agentdeck-managed) ← THIS IS THE FIX
6. → `ui.SetVersion`, theme init, `promptForUpdate()`, `ensureTmuxInPath()`, TUI launch

**Parallel paths considered (and why we only touch the TUI path):**
- CLI subcommands (`agent-deck add`, `session start`, `mcp attach`, …) — MUST keep working inside tmux. The reporter's confusion is ONLY about the interactive TUI. CLI already works fine in nested contexts and we must not regress that.
- `agent-deck session current` (session_cmd.go:2069) — REQUIRES TMUX to be set. Keep as-is.
- `ResolveSessionOrCurrent` fallback (session_cmd.go:686) — USES TMUX for auto-detect. Keep as-is.
- `GetCurrentSessionID` (cli_utils.go:280) — USES TMUX to detect agentdeck session. Keep as-is.

Only ONE code path changes: the TUI launch path in `main.go`, immediately after
the existing `isNestedSession()` guard. Adding a second guard for the
generic-tmux case preserves all CLI behaviour.

## Fix design

Add a function `isOuterTmuxWithoutOptIn() bool` to `main.go`:

```go
// isOuterTmuxWithoutOptIn reports true when the user is launching the
// interactive TUI from inside a NON-agentdeck tmux session without the
// opt-in env var set. See issue #560.
func isOuterTmuxWithoutOptIn() bool {
    if os.Getenv("TMUX") == "" {
        return false // not in tmux, OK
    }
    if isNestedSession() {
        return false // agentdeck-managed, handled by the nested guard above
    }
    if os.Getenv("AGENT_DECK_ALLOW_OUTER_TMUX") == "1" {
        return false // user opted in
    }
    return true
}
```

Call it in `main()` immediately after the `isNestedSession()` block. On hit,
print an explanatory message (same style as the existing nested guard) and
`os.Exit(1)`. The message must:
1. State clearly that agent-deck manages its own tmux sessions
2. Explain that nesting causes detach confusion
3. Offer the opt-in env var for users who insist
4. Point at the CLI subcommand pattern for users who just want to run one command

## Failing tests (RED first)

File: `cmd/agent-deck/main_test.go` — append the following three tests:

1. `TestOuterTmuxGuard_OuterTmuxNoOptIn` — `TMUX` set to a non-agentdeck value,
   `AGENT_DECK_ALLOW_OUTER_TMUX` unset → expect `isOuterTmuxWithoutOptIn()` to
   return `true`.
2. `TestOuterTmuxGuard_OuterTmuxWithOptIn` — `TMUX` set, opt-in env set to
   `"1"` → expect `false`.
3. `TestOuterTmuxGuard_NoTmux` — `TMUX` unset → expect `false`.

All three will fail to COMPILE on current main because `isOuterTmuxWithoutOptIn`
does not exist. That's the RED signal.

## Scope boundaries

**May change:**
- `cmd/agent-deck/main.go` — add `isOuterTmuxWithoutOptIn()` helper + call site after the nested guard
- `cmd/agent-deck/main_test.go` — add the three tests above
- `.claude/release-tests.yaml` — append new regression entries in phase 8a

**MUST NOT change:**
- `internal/tmux/**` — unrelated
- `internal/session/**` — unrelated
- `cmd/agent-deck/session_cmd.go` — CLI subcommands must stay working inside tmux
- `cmd/agent-deck/cli_utils.go` — `GetCurrentSessionID` logic must stay
- Any watcher / feedback / persistence / per-group paths

## Live-boundary verification (phase 7)

1. Start a generic tmux session (not agentdeck): `tmux new-session -d -s outer560 'sleep 600'`
2. Exec agent-deck binary inside that tmux pane: expect startup exit with
   non-zero status and the explanatory message on stderr.
3. Set `AGENT_DECK_ALLOW_OUTER_TMUX=1` and rerun: expect TUI boots normally
   (user opted in).
4. Exit tmux; run `agent-deck --version`: expect normal version output
   (guard only fires on TUI launch, not on CLI subcommands).
5. Inside the same outer tmux, run `agent-deck list`: expect normal list
   output (CLI subcommand unaffected by the guard).

All five boundary checks must pass.
