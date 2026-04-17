# Fix issue #610 — CLI status parity with TUI / web API

## (a) Problem summary

`agent-deck list --json` and `agent-deck session show <id> --json` report
incorrect status (typically `idle` or `waiting`) for sessions that the TUI
and the web API at `/api/menu` correctly report as `running`.

Scripts that automate agent-deck via CLI JSON output make wrong decisions
because their view of the world disagrees with the user's view in the TUI.

## (b) Exact reproducer

Requires a live tmux server and a Claude session doing a long tool call
(e.g. a multi-minute `bash -c` or a file read that spans >2 minutes).

```bash
# 1. Start a long-running session via agent-deck
agent-deck add -t repro610 -c claude ~/some/project
agent-deck session attach repro610

# 2. Inside Claude, kick off a long tool call (>2 min). Example:
#    "Read this 500MB log file and summarize everything."
#    The pane title acquires a braille spinner (⠋/⠙/⠹/⠸/⠼/⠴/⠦/⠧/⠇/⠏).

# 3. Detach from the session (C-b d) or leave it running in another pane.

# 4. In a second shell, wait >2 minutes so the stored "running" hook event
#    ages out of the 2-minute fast-path window (hookFastPathWindow).
sleep 130

# 5. Poll each surface. All three SHOULD say "running".
agent-deck list --json                     | jq '.[] | select(.id=="<ID>") | .status'
agent-deck session show <ID> --json         | jq .status
curl -s http://localhost:8080/api/menu     | jq '.. | .status? // empty' | head

# Observed on main:
#   list --json     → "idle" or "waiting"
#   session show    → "idle" or "waiting"
#   /api/menu       → "running"      ← parity break
```

The same reproducer works without an actual Claude process by setting a
braille character on the pane title with:
`tmux select-pane -t <tmux-session> -T "⠋ Working"` — used by the unit
tests in `internal/session/instance_cli_parity_test.go`.

## (c) Data-flow trace

### TUI — `internal/ui/home.go :: Home.backgroundStatusUpdate` (line ~2497)
```
tick →
  tmux.RefreshExistingSessions()               # tmux.go
  tmux.RefreshPaneInfoCache()                  # title_detection.go:110  ★ warms pane-title cache
  for each inst:
    hookWatcher.GetHookStatus(inst.ID)         # hook_watcher.go
    inst.UpdateHookStatus(hs)                  # instance.go:2798
  parallel for each inst:
    inst.UpdateStatus()                        # instance.go:2476
      → hookFastPathWindow check (2m)
      → tmuxSession.GetStatus()                # tmux.go:2296
          → GetCachedPaneInfo(name)            # title_detection.go:191   ★ cache HIT
          → AnalyzePaneTitle(title,_)          # title_detection.go:212
            → braille → TitleStateWorking
          → return "active"
      → inst.Status = StatusRunning
```

### Web API — `GET /api/menu` → `internal/web/handlers_menu.go::handleMenu` (line 26)
```
GET /api/menu →
  s.menuData.LoadMenuSnapshot()                # either MemoryMenuData or SessionDataService

SessionDataService.LoadMenuSnapshot (session_data_service.go:126)
  → s.refreshStatuses(instances)                # line 210
      → tmux.RefreshExistingSessions()
      → tmux.RefreshPaneInfoCache()            ★ warms pane-title cache (same as TUI)
      → defaultLoadHookStatuses()               # reads hook JSON from disk
      → inst.UpdateHookStatus(hs)
      → inst.ForceNextStatusCheck() (for Claude without hook)
      → inst.UpdateStatus()                     # title fast-path HIT
```

MemoryMenuData (web-mode) reads the last snapshot published by the web
server's background refresher — which in turn runs through
`refreshStatuses` on every tick. So the pane-title cache is always warm
inside the web server's process memory.

### CLI — `agent-deck list --json` — `cmd/agent-deck/main.go :: handleList` (line 1438)
```
main → handleList →
  storage.LoadInstances()                       # cold load from sessions.json
  for each inst: inst.UpdateStatus()            # line 1501  ← NO pre-warm
      → UpdateStatus reads hook file cold (instance.go:2545-2558)
      → hookFastPathWindow check → STALE (>2m) → fall through
      → tmuxSession.GetStatus() (tmux.go:2296)
          → GetCachedPaneInfo(name)             # title_detection.go:191
            → cache was NEVER populated by this process → MISS
          → fall through to window-activity + content scan
          → first call, stateTracker nil → CapturePane → hasBusyIndicator
            - "sleep 3600" pane has no "esc to interrupt" → not busy
            - no prompt indicator → not waiting
          → falls to "Restored session" block (tmux.go:2514)
            - previousStatus "idle" → StatusIdle
            - else                    → StatusWaiting
      → inst.Status = StatusIdle / StatusWaiting   ★ wrong
```

### CLI — `agent-deck session show <id> --json` — `cmd/agent-deck/session_cmd.go :: handleShow` (line 711)

Same shape as `handleList`: a single `inst.UpdateStatus()` with no
pre-warm of `tmux.RefreshPaneInfoCache()` and no explicit hook-file load
beyond what `UpdateStatus` does cold internally.

### Divergence point

| Step                         | TUI | Web | CLI list | CLI show |
|------------------------------|-----|-----|----------|----------|
| `tmux.RefreshPaneInfoCache`  | ✔   | ✔   | **✘**    | **✘**    |
| Fresh hook statuses loaded   | ✔   | ✔   | cold (UpdateStatus only) | cold |
| `ForceNextStatusCheck`       | n/a | ✔   | ✘        | ✘        |
| `inst.UpdateStatus()`        | ✔   | ✔   | ✔        | ✔        |

The CLI skips the only step that reliably surfaces "working" state during
long tool calls — the pane-title cache refresh.

Authoritative sources examined:
- `internal/tmux/tmux.go:2296` `GetStatus`
- `internal/tmux/tmux.go:2326` title fast-path reads `GetCachedPaneInfo`
- `internal/tmux/title_detection.go:110` `RefreshPaneInfoCache` (populates cache)
- `internal/tmux/title_detection.go:191` `GetCachedPaneInfo` (4 s staleness)
- `internal/tmux/title_detection.go:212` `AnalyzePaneTitle` (braille → working)
- `internal/session/instance.go:2474` `UpdateStatus` (hook fast-path gate)
- `internal/session/instance.go:2545` cold-load hook read
- `internal/session/instance.go:56` `hookFastPathWindow = 2 * time.Minute`
- `internal/ui/home.go:2526` TUI calls `RefreshPaneInfoCache`
- `internal/web/session_data_service.go:213` web calls `RefreshPaneInfoCache`
- `cmd/agent-deck/main.go:1501` CLI list path — no pre-warm
- `cmd/agent-deck/session_cmd.go:711` CLI show path — no pre-warm

## (d) Failing test cases (committed)

File: `internal/session/instance_cli_parity_test.go`

1. `TestUpdateStatus_CLIParity_SpinnerTitle_StaleHook`
   - Creates a real tmux session (`sleep 3600` as the pane process).
   - Writes a 3-minute-old `running` hook file under HOME/.agent-deck/hooks
     so the hook fast-path is stale (mirrors Claude's long tool calls).
   - Sets pane title to `"⠋ Working on refactor"` via `tmux select-pane -T`.
   - Sleeps past the 1.5 s grace window.
   - Calls `RefreshInstancesForCLIStatus([]*Instance{inst})` — the helper
     the fix must introduce, analogous to `refreshStatuses` in the web
     package and `backgroundStatusUpdate` in the UI package.
   - Asserts `inst.GetStatusThreadSafe() == StatusRunning`.

2. `TestUpdateStatus_CLIvsTUIParity_SameTmuxState`
   - Builds one tmux session and two `*Instance` wrappers pointing at it
     (mirrors the real split: TUI and CLI are different OS processes
     loading the same `sessions.json`).
   - Runs the TUI path (`tmux.RefreshPaneInfoCache` + `UpdateHookStatus` +
     `UpdateStatus`) and the CLI path (`RefreshInstancesForCLIStatus` +
     `UpdateStatus`) against the same tmux state.
   - Asserts both report the same `Status`, and that it is `StatusRunning`
     (sanity oracle).

Current failure mode (`go test ./internal/session/ -run
TestUpdateStatus_CLIParity`):

```
internal/session/instance_cli_parity_test.go:111:2:
    undefined: RefreshInstancesForCLIStatus
internal/session/instance_cli_parity_test.go:171:2:
    undefined: RefreshInstancesForCLIStatus
FAIL	github.com/asheshgoplani/agent-deck/internal/session [build failed]
```

i.e. compilation fails because the fix's public helper does not exist yet
— the mandated TDD red.

## (e) Implementation sketch

### Option 1 (recommended) — introduce `session.RefreshInstancesForCLIStatus(instances []*Instance)`

New file `internal/session/cli_status_refresh.go`:

```go
package session

import "github.com/asheshgoplani/agent-deck/internal/tmux"

// RefreshInstancesForCLIStatus warms the shared tmux caches and loads
// on-disk hook statuses into the given instances. It is the CLI analogue
// of SessionDataService.refreshStatuses (internal/web) and
// Home.backgroundStatusUpdate (internal/ui). Call this before looping
// inst.UpdateStatus() in any CLI emitter — without it the title fast-path
// in tmux.GetStatus cannot fire because the pane-info cache is process-
// local and the CLI never populates it otherwise. Regression: issue #610.
func RefreshInstancesForCLIStatus(instances []*Instance) {
	if len(instances) == 0 {
		return
	}
	tmux.RefreshExistingSessions()
	tmux.RefreshPaneInfoCache()
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		if IsClaudeCompatible(inst.Tool) || inst.Tool == "codex" || inst.Tool == "gemini" {
			if hs := readHookStatusFile(inst.ID); hs != nil {
				inst.UpdateHookStatus(hs)
			}
		}
	}
}
```

CLI call sites (single-line insertions):

- `cmd/agent-deck/main.go` `handleList`, immediately above the
  `for i, inst := range instances { _ = inst.UpdateStatus() ...` loop
  (line ~1499).
- `cmd/agent-deck/session_cmd.go` `handleShow`, immediately above
  `_ = inst.UpdateStatus()` (line ~710).
- Anywhere else in `cmd/agent-deck` that emits JSON and loops
  `UpdateStatus` without pre-warm — audit other JSON emitters in
  `session_cmd.go` (`handleStatus`, etc.) and apply the same pattern.

### Option 2 — widen `hookFastPathWindow` for `running`

Set a separate `hookRunningFastPathWindow = 10 * time.Minute` so long
tool calls stay "running" via the hook fast-path. Downsides:
- Still wrong when the hook was never written (new session, CLI-only
  interaction, hooks disabled).
- Hides a stale dead session as `running` for 10 minutes.
- Does not address the parity gap structurally — TUI and web already
  warm the cache, CLI still would not.

### Option 3 — have `UpdateStatus` auto-refresh the cache when it is stale

Cheap per-instance but `tmux.RefreshPaneInfoCache` is a
`list-panes -a` subprocess — running it once per Instance per tick is
wasteful and undoes the "once per tick" contract documented at
`title_detection.go:108`.

**Chosen:** Option 1. Smallest surface, reuses existing web and TUI
patterns, testable at package boundary.

## (f) Scope boundaries

**MAY edit (within fix/issue-610-cli-status-parity):**
- `internal/session/cli_status_refresh.go` (new file) — helper landing spot.
- `internal/session/instance_cli_parity_test.go` (new) — failing tests.
- `cmd/agent-deck/main.go` — call `RefreshInstancesForCLIStatus` in
  `handleList` and `handleListAllProfiles` JSON branches.
- `cmd/agent-deck/session_cmd.go` — call the helper in `handleShow` (and
  any sibling JSON emitters touching `UpdateStatus`).
- `CHANGELOG.md` — one-line entry under Unreleased.
- `.planning/fix-issue-610/**` — planning artifacts (this file).

**MUST NOT edit without an RFC:**
- `internal/tmux/**` — adding refresh calls, changing cache TTL, adding
  new exported state. Regression risk on the eight persistence tests
  plus the watcher suite per `CLAUDE.md`.
- `internal/session/instance.go` — the hot `UpdateStatus` path. No widened
  hookFastPathWindow, no per-call `RefreshPaneInfoCache`, no new branches
  that could destabilise the `TestPersistence_*` suite.
- `internal/ui/**`, `internal/web/**` — already correct; do not touch.
- `internal/watcher/**` — unrelated, protected by mandate.
- Any `--no-verify` commits on source-modifying changes (CLAUDE.md).

**Verification before marking done:**
- `GOTOOLCHAIN=go1.24.0 go test ./internal/session/... -race -count=1`
- `GOTOOLCHAIN=go1.24.0 go test ./cmd/agent-deck/... -race -count=1`
- `GOTOOLCHAIN=go1.24.0 go test -run TestPersistence_ ./internal/session/... -race -count=1`
- Manual spot-check: set up a live Claude session; confirm
  `list --json | jq '.[].status'`, `session show <id> --json | jq .status`,
  and `/api/menu` all report `running` during a >2-minute tool call.
