# Phase 15: Mouse, Theme & Polish - Research

**Researched:** 2026-03-13
**Domain:** Bubble Tea TUI mouse events, lipgloss theme correctness, conductor heartbeat architecture, git worktree reuse, README documentation
**Confidence:** HIGH

## Summary

Phase 15 addresses five independent polish items that are all self-contained within the existing codebase. No new packages or external dependencies are required. The work divides into two categories: UI correctness (mouse scroll everywhere, light theme bleed-through) and internal hygiene (heartbeat consolidation, worktree reuse, auto_cleanup docs).

The mouse scroll gap (UX-01) is the most impactful: `tea.WithMouseCellMotion()` is already enabled in `main.go:468`, so Bubble Tea already delivers `tea.MouseMsg` events to `Update()`, but there is no `case tea.MouseMsg` handler anywhere in the codebase. All wheel events are silently dropped. Fixing this is adding one switch case to `Update()` plus corresponding handlers in `settings_panel.go` and `global_search.go`.

The light theme bleed-through (UX-02) is caused by ANSI terminal output in the preview pane carrying dark-background ANSI escape codes that were written when the session ran under dark theme. The color palette itself (`styles.go`) is correctly split between dark/light. The fix is either stripping background ANSI from terminal preview content, or ensuring the preview pane background is forced to the theme background color.

**Primary recommendation:** Implement all five items in one wave, in dependency order: docs first (zero risk), then mouse, then light theme, then heartbeat consolidation, then worktree reuse.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| UX-01 | Mouse wheel scroll works in session list and other scrollable areas (settings, search, dialogs) | `tea.MouseButtonWheelUp/Down` verified in bubbletea v1.3.10; all scrollable areas identified |
| UX-02 | Light theme renders correctly in Codex preview and live session views; no dark background bleed-through | Hardcoded ANSI colors from tmux pane capture are the root cause; lipgloss palette is correct |
| UX-03 | auto_cleanup option documented in README sandbox section with explanation of what gets cleaned and when | Option exists in `DockerSettings`, already documented in `sandbox.md` but missing from README |
| UX-04 | Redundant heartbeat mechanisms consolidated into a single mechanism (systemd timer vs bridge.py heartbeat_loop) | Two distinct mechanisms identified: OS daemon (launchd/systemd) vs bridge.py async loop |
| UX-05 | Existing git worktrees are detected and reused instead of creating new ones when a worktree for the target branch already exists | `GetWorktreeForBranch()` exists in `internal/git/git.go:253`; `CreateWorktree()` does not check first |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/charmbracelet/bubbletea | v1.3.10 | TUI framework, mouse events | Already used; v1.3.10 confirmed in go.mod |
| github.com/charmbracelet/lipgloss | v1.1.0 | Styling, color palette | Already used; theme system built on it |
| github.com/asheshgoplani/agent-deck/internal/git | (internal) | Worktree operations | ListWorktrees, GetWorktreeForBranch already exist |

### No New Dependencies Needed
All five UX items are pure code changes within the existing codebase. No `go get` commands required.

## Architecture Patterns

### Recommended Project Structure (no changes)
All changes fit inside existing files:
```
internal/ui/home.go            Add tea.MouseMsg case to Update()
internal/ui/settings_panel.go  Add MouseMsg to Update() signature; handle wheel
internal/ui/global_search.go   Add MouseMsg case to Update(); handle wheel
internal/git/git.go             Check existing worktree before CreateWorktree calls
cmd/agent-deck/main.go          CreateWorktree call sites
cmd/agent-deck/session_cmd.go   CreateWorktree call site
cmd/agent-deck/launch_cmd.go    CreateWorktree call site
conductor/bridge.py             heartbeat_loop consolidation
README.md                       Add auto_cleanup explanation
```

### Pattern 1: Bubble Tea Mouse Event Handler

**What:** Add a `case tea.MouseMsg` to `Home.Update()` that dispatches wheel events to the active scrollable area.

**When to use:** Any time mouse events need to affect UI state. Fires on every scroll tick.

**Key rules:**
- Match on `msg.Button`, NOT the deprecated `msg.Type`
- Guard non-wheel button events with `msg.Action == tea.MouseActionPress`
- Handler must be O(1) with no blocking I/O (Bubble Tea issue #1047: slow Update() blocks rendering)
- Route to the active overlay first, then fall through to the main list

**Example:**
```go
// Source: bubbletea v1.3.10 mouse.go (verified from module cache)
case tea.MouseMsg:
    switch msg.Button {
    case tea.MouseButtonWheelUp:
        if h.settingsPanel.IsVisible() {
            h.settingsPanel.ScrollUp()
            return h, nil
        }
        if h.globalSearch.IsVisible() {
            h.globalSearch.ScrollUp()
            return h, nil
        }
        // Main session list
        if h.cursor > 0 {
            h.cursor--
            h.syncViewport()
            h.markNavigationActivity()
            return h, h.fetchSelectedPreview()
        }
    case tea.MouseButtonWheelDown:
        if h.settingsPanel.IsVisible() {
            h.settingsPanel.ScrollDown()
            return h, nil
        }
        if h.globalSearch.IsVisible() {
            h.globalSearch.ScrollDown()
            return h, nil
        }
        // Main session list
        if h.cursor < len(h.flatItems)-1 {
            h.cursor++
            h.syncViewport()
            h.markNavigationActivity()
            return h, h.fetchSelectedPreview()
        }
    }
    return h, nil
```

**Insertion point:** Inside `func (h *Home) Update(msg tea.Msg)`, after `case tea.WindowSizeMsg` and before `case tea.KeyMsg`. Add as a peer switch case.

**Scrollable areas and their scroll state:**

| Area | File | Scroll field | Notes |
|------|------|--------------|-------|
| Session list | `home.go` | `h.cursor` + `h.viewOffset` via `syncViewport()` | Primary target |
| Settings panel | `settings_panel.go` | `s.cursor` + `s.scrollOffset` | Update() only takes `tea.KeyMsg`; add `ScrollUp()/ScrollDown()` helpers or change signature |
| Global search results | `global_search.go` | `gs.cursor` | Update() already takes `tea.Msg`; add `tea.MouseMsg` case there |
| Global search preview | `global_search.go` | `gs.previewScroll` | Scroll by 3 lines per tick |
| MCP dialog | `mcp_dialog.go` | Per-scope index fields | Update() only takes `tea.KeyMsg`; add helpers or route from home.go |

**Settings panel approach:** `SettingsPanel.Update()` currently takes `tea.KeyMsg`. Either:
- Option A: Add `ScrollUp() / ScrollDown()` methods to `SettingsPanel`, call them from `home.go`'s `tea.MouseMsg` case
- Option B: Change `SettingsPanel.Update()` to accept `tea.Msg` (broader change)

Option A is preferred: minimal impact, no interface changes.

### Pattern 2: Light Theme ANSI Bleed-Through Fix

**What:** Terminal preview content captured via `tmux capture-pane` contains raw ANSI codes. These codes include background color sequences using dark theme hex values that were written by the tool (e.g., Claude Code renders its own UI with dark backgrounds). When agent-deck switches to light theme, these ANSI codes are still rendered as-is.

**Root cause (verified by code inspection):**
- `styles.go` correctly defines separate `darkColors` and `lightColors` palettes
- `InitTheme()` correctly switches all `Color*` variables
- `renderPreviewPane()` uses `ColorText`, `ColorAccent`, etc. (theme-aware)
- BUT: the raw pane capture at `home.go:10092-10095` is passed through unchanged
- Tool output (Claude Code, Codex) writes its own ANSI styling with specific colors
- These embedded ANSI codes hardcode dark terminal colors and survive theme switching

**Approaches (in order of recommendation):**

Option A (recommended): Wrap the preview content block with a lipgloss style that forces the background to `ColorBg`. Lipgloss overrides will apply to content that lacks explicit ANSI, but embedded ANSI colors will still show through because lipgloss renders around existing ANSI sequences.

Option B (more complete): Strip background ANSI sequences from captured pane content before rendering. Use `github.com/charmbracelet/x/ansi` (already imported at `home.go:10051`) to strip or transform sequences. Specifically, strip `ESC[4xm` (background color) sequences.

Option C: Only apply background stripping in light theme mode (check `GetCurrentTheme() == ThemeLight`).

**Code location for the fix:**
```go
// home.go ~line 10122, where preview content is rendered:
// Currently: lines are output directly
// Fix: when light theme, strip ANSI background from each line
```

**Note on scope:** The issue title says "Codex preview and live session views." The preview pane covers all tools. The fix should apply to any captured pane content, not Codex-specifically. Light theme is the condition, not the tool.

**Confidence:** MEDIUM — the mechanism is understood, but the exact stripping approach (lipgloss wrapping vs ANSI strip) needs validation. Recommend trying lipgloss `Background(ColorBg)` wrapper on the preview content block first; if bleed persists, use ANSI stripping.

### Pattern 3: auto_cleanup Documentation

**What:** Add a brief explanation of `auto_cleanup` to the README.md Docker Sandbox section.

**Current state:** `auto_cleanup` is documented in `skills/agent-deck/references/sandbox.md` (line 69) with a one-line description: "Remove containers when sessions are killed." The README sandbox section at lines 153-159 only shows `default_enabled = true` and `mount_ssh = true` in its config example.

**Fix:** Add `auto_cleanup = true` to the README's `[docker]` TOML example block, plus one sentence explaining what it controls and why you'd set it to `false`.

**What auto_cleanup actually does (from code):**
- `DockerSettings.GetAutoCleanup()` in `userconfig.go:901` — defaults to `true` if nil
- Used in `instance.go:3474` — removes the container when the session is killed
- Used in `maintenance.go:303` — skips cleanup during maintenance if disabled
- Setting `auto_cleanup = false` keeps the container alive after session termination (useful for debugging, inspecting container state)

### Pattern 4: Heartbeat Consolidation (UX-04)

**What:** Two mechanisms trigger periodic conductor heartbeats. They serve the same purpose but operate independently.

**Mechanism 1 (OS daemon):** `InstallHeartbeatDaemon()` in `internal/session/conductor.go:1820`
- macOS: launchd plist (`~/Library/LaunchAgents/com.agentdeck.conductor-heartbeat.{name}.plist`)
- Linux/WSL: systemd user timer (`~/.config/systemd/user/agent-deck-conductor-heartbeat-{name}.timer`)
- Executes `heartbeat.sh` which runs `agent-deck conductor send ... "Check sessions"`
- Interval: configurable via `ConductorSettings.GetHeartbeatInterval()`
- This is the PRIMARY mechanism installed by `agent-deck conductor setup`

**Mechanism 2 (bridge.py):** `heartbeat_loop()` in `conductor/bridge.py:592`
- Telegram/Slack bridge only — runs when the Python bridge is active
- Checks all profiles for waiting/error sessions and sends conductor messages
- Same interval source: `config["heartbeat_interval"]`

**Issue #225 description:** "Redundant heartbeat mechanisms." The bridge's `heartbeat_loop` fires heartbeats even when the OS daemon is also running, causing double-triggers.

**Consolidation approach (from docs/GSD-SPEC-v021.md:121):** The spec says to consolidate, not to eliminate. Options:

Option A: Bridge skips heartbeat if OS daemon is detected (check for plist/timer file existence). If daemon found, `heartbeat_loop` becomes a no-op.

Option B: Bridge heartbeat runs only if no OS daemon is installed (i.e., when `heartbeat_interval > 0` but daemon install failed on non-launchd/systemd systems).

Option C: Remove `heartbeat_loop` from bridge.py entirely; rely solely on OS daemon. The bridge already sends heartbeats on user Telegram/Slack commands; the periodic loop is additive.

**Recommendation:** Option A is safest. The OS daemon is the canonical mechanism. The bridge loop adds value only in environments without launchd/systemd (rare). Detection: check whether `~/.config/systemd/user/agent-deck-conductor-heartbeat-{name}.timer` or `~/Library/LaunchAgents/com.agentdeck.conductor-heartbeat.{name}.plist` exists.

**Alternative scoping (LOW confidence):** The issue may simply want a config flag `use_bridge_heartbeat = false` to let users disable the bridge loop when the OS daemon is running. This is the minimal change.

### Pattern 5: Git Worktree Reuse (UX-05)

**What:** When a user creates a new session with a branch that already has a worktree, `CreateWorktree()` fails with "fatal: already checked out in another worktree." Instead, detect the existing worktree and reuse its path.

**Detection function already exists:**
```go
// internal/git/git.go:252
func GetWorktreeForBranch(repoDir, branchName string) (string, error)
// Returns worktree path if branch is checked out in a worktree, or "" if not
```

**Call sites that need the pre-check:**
1. `internal/ui/home.go:5936` — TUI session creation async command
2. `internal/ui/home.go:6191` — TUI fork/resume async command
3. `cmd/agent-deck/main.go:963` — CLI `agent-deck launch`
4. `cmd/agent-deck/session_cmd.go:507` — CLI `agent-deck session start`
5. `cmd/agent-deck/launch_cmd.go:192` — CLI `agent-deck launch` (separate path)

**Fix pattern for each call site:**
```go
// Before:
if err := git.CreateWorktree(repoRoot, worktreePath, branchName); err != nil {
    return ..., err
}
path = worktreePath

// After:
existingPath, err := git.GetWorktreeForBranch(repoRoot, branchName)
if err == nil && existingPath != "" {
    // Reuse existing worktree — update path to existing location
    path = existingPath
    worktreePath = existingPath
} else {
    if err := git.CreateWorktree(repoRoot, worktreePath, branchName); err != nil {
        return ..., err
    }
    path = worktreePath
}
```

**Session metadata:** When reusing an existing worktree, `inst.WorktreePath` should be set to `existingPath`, not the originally computed `worktreePath`. The session record will then correctly reflect the actual worktree location.

### Anti-Patterns to Avoid

- **Using deprecated mouse constants:** `tea.MouseWheelUp` and `tea.MouseWheelDown` are deprecated since bubbletea v1.3.x. Use `msg.Button == tea.MouseButtonWheelUp` and `msg.Button == tea.MouseButtonWheelDown`.
- **Blocking I/O in mouse handler:** Mouse events fire on every wheel tick. The handler must be O(1). No subprocess spawning, no file reads. Delegate preview fetching through the debounce system just like keyboard navigation does.
- **Routing all mouse events to dialogs before checking which is active:** Only route to the active overlay; fall through to the session list otherwise.
- **Full ANSI strip for light theme:** Stripping ALL ANSI in light theme would destroy intentional formatting (bold, italic, syntax highlighting). Strip only background color sequences (`ESC[4xm` family) or use a lipgloss background wrapper.
- **Creating a new worktree without checking:** If `GetWorktreeForBranch` returns a path, do not call `CreateWorktree` — it will fail with a git error.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Mouse wheel events | Custom terminal escape sequence parser | `tea.MouseMsg` with `msg.Button == tea.MouseButtonWheelUp/Down` | bubbletea already parses, delivers, and handles edge cases |
| ANSI sequence manipulation | Custom regex/string parser | `github.com/charmbracelet/x/ansi` (already imported) | Handles edge cases in multi-byte ANSI sequences |
| Worktree existence check | Custom `git worktree list` parser | `git.GetWorktreeForBranch()` (already exists in internal/git) | Already handles porcelain format parsing, branch matching |
| Scroll bounds checking | Custom min/max clamping | Mirror the existing `cursor--/cursor++` guards already in key handlers | Consistency; same bounds logic already proven correct |

**Key insight:** Every problem in Phase 15 already has infrastructure in the codebase. The work is wiring, not building.

## Common Pitfalls

### Pitfall 1: Mouse Events Routing to Wrong Active Area
**What goes wrong:** When settings panel is open, wheel events scroll the session list behind it instead of the panel.
**Why it happens:** The `tea.MouseMsg` case falls through to the main list without checking active overlays first.
**How to avoid:** Always check active overlays in priority order before acting on the main list. Order: setup wizard > settings panel > help overlay > global search > mcp dialog > main list.
**Warning signs:** Scrolling settings panel moves session list cursor.

### Pitfall 2: Deprecated Mouse Constants
**What goes wrong:** Using `tea.MouseWheelUp` causes a compile-time deprecation warning, or worse, silent mismatch if the constant values changed.
**Why it happens:** bubbletea v1.3.x moved from `MouseEventType` (deprecated `Type` field) to `MouseButton` and `MouseAction` fields on `MouseEvent`.
**How to avoid:** Only use `msg.Button == tea.MouseButtonWheelUp` and `msg.Button == tea.MouseButtonWheelDown`. Never reference `msg.Type`.
**Warning signs:** `golangci-lint` deprecation warnings.

### Pitfall 3: Light Theme Preview Line-by-Line Background Inconsistency
**What goes wrong:** Some lines in the preview pane show dark backgrounds, others show correct light backgrounds, creating a striped appearance.
**Why it happens:** Some terminal lines have explicit background ANSI codes; lines without them inherit the terminal background color.
**How to avoid:** Apply the background fix uniformly — either to all preview lines or to the container block, not line-by-line conditionally.
**Warning signs:** Alternating dark/light background bands in preview pane under light theme.

### Pitfall 4: Worktree Path Mismatch in Session Record
**What goes wrong:** Session's `WorktreePath` contains the originally computed path (from branch name slug) but the actual worktree is at a different path (the existing one).
**Why it happens:** When reusing an existing worktree, the code sets `path = existingPath` for session creation but forgets to also update `inst.WorktreePath = existingPath`.
**How to avoid:** After reuse detection, update ALL worktree-related fields on the instance: `WorktreePath`, `WorktreeRepoRoot`, `WorktreeBranch`.
**Warning signs:** `agent-deck worktree info` shows wrong path; worktree status shows "MISSING."

### Pitfall 5: Double Heartbeat After Consolidation
**What goes wrong:** The bridge.py still runs `heartbeat_loop` after the consolidation change, sending two heartbeat triggers.
**Why it happens:** The OS daemon detection check in bridge.py is wrong (path check fails on different OS).
**How to avoid:** Log the detection result at startup; verify in tests.
**Warning signs:** Conductor receives two "Check sessions" messages within seconds of each other.

## Code Examples

Verified patterns from official sources and codebase inspection:

### Mouse Wheel in Bubble Tea v1.3.10
```go
// Source: bubbletea v1.3.10 mouse.go (verified from module cache)
// MouseButton constants:
//   MouseButtonWheelUp   (value: 4)
//   MouseButtonWheelDown (value: 5)
// MouseAction constants:
//   MouseActionPress (value: 0) — scroll wheel always fires as Press
case tea.MouseMsg:
    if msg.Button == tea.MouseButtonWheelUp {
        // same logic as "k" / "up"
    }
    if msg.Button == tea.MouseButtonWheelDown {
        // same logic as "j" / "down"
    }
```

### GetWorktreeForBranch Usage
```go
// Source: internal/git/git.go:252 (verified)
existingPath, err := git.GetWorktreeForBranch(repoRoot, branchName)
if err == nil && existingPath != "" {
    // Branch already has a worktree at existingPath
    worktreePath = existingPath
} else {
    // Create new worktree
    if err := git.CreateWorktree(repoRoot, worktreePath, branchName); err != nil {
        return sessionCreatedMsg{err: fmt.Errorf("failed to create worktree: %w", err)}
    }
}
```

### SettingsPanel Scroll Helpers (to add)
```go
// Add to settings_panel.go
func (s *SettingsPanel) ScrollUp() {
    if s.cursor > 0 {
        s.cursor--
    }
}

func (s *SettingsPanel) ScrollDown() {
    if s.cursor < settingsCount-1 {
        s.cursor++
    }
}
```

### GlobalSearch Mouse Handler (to add inside Update)
```go
// Source: global_search.go Update() already accepts tea.Msg; add case
case tea.MouseMsg:
    switch msg.Button {
    case tea.MouseButtonWheelUp:
        if gs.cursor > 0 {
            gs.cursor--
            gs.previewScroll = 0
        }
    case tea.MouseButtonWheelDown:
        if gs.cursor < len(gs.results)-1 {
            gs.cursor++
            gs.previewScroll = 0
        }
    }
    return gs, nil
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `msg.Type == tea.MouseWheelUp` | `msg.Button == tea.MouseButtonWheelUp` | bubbletea v1.3.x | Old constants deprecated; still compile but `Type` field deprecated |
| Manual worktree check via `os.Stat` | `git.GetWorktreeForBranch()` (internal) | Existed since worktree feature | Unified lookup, already handles porcelain parse |

**Deprecated/outdated:**
- `tea.MouseEventType` (the `Type` field on `MouseEvent`): deprecated; use `MouseButton` + `MouseAction` fields
- `tea.MouseWheelUp`, `tea.MouseWheelDown`, etc.: deprecated constants of the deprecated type

## Open Questions

1. **Light theme ANSI fix approach**
   - What we know: Tool output (Claude Code, Codex) uses its own ANSI background styling; lipgloss color variables are correct
   - What's unclear: Whether a lipgloss background wrapper is sufficient or whether individual background ANSI sequences need stripping
   - Recommendation: Try lipgloss `Background(ColorBg)` on the preview content block first. If bleed persists (because embedded ANSI overrides lipgloss), switch to stripping `ESC[4xm` background sequences using `ansi` package

2. **Heartbeat consolidation scope**
   - What we know: Two mechanisms (OS daemon + bridge.py loop) can double-trigger
   - What's unclear: Whether issue #225 wants the bridge loop removed, or just made conditional
   - Recommendation: Make bridge loop conditional: check for installed daemon file, skip if found. Safer than removing — preserves bridge heartbeat for cron-only environments

3. **MCP dialog mouse scroll**
   - What we know: `MCPDialog.Update()` takes `tea.KeyMsg`, scroll state is per-scope cursor indices
   - What's unclear: Whether mouse scroll in the MCP dialog is in scope for UX-01 (requirement says "session list and other scrollable areas including settings, search, dialogs")
   - Recommendation: Include it. Add `ScrollUp()/ScrollDown()` helpers to `MCPDialog`, call from `home.go` mouse handler when `mcpDialog.IsVisible()`

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (go test -race -v) |
| Config file | none — test packages use TestMain with AGENTDECK_PROFILE=_test |
| Quick run command | `go test -race -v ./internal/ui/...` |
| Full suite command | `go test -race -v ./...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| UX-01 | Mouse wheel scrolls session list cursor | unit | `go test -race -v -run TestMouseScroll ./internal/ui/...` | ❌ Wave 0 |
| UX-01 | Mouse wheel scrolls settings panel cursor | unit | `go test -race -v -run TestSettingsPanelScroll ./internal/ui/...` | ❌ Wave 0 |
| UX-01 | Mouse wheel scrolls global search results | unit | `go test -race -v -run TestGlobalSearchMouseScroll ./internal/ui/...` | ❌ Wave 0 |
| UX-02 | Light theme: no hardcoded dark hex in preview rendering functions | unit | `go test -race -v -run TestLightThemePreview ./internal/ui/...` | ❌ Wave 0 |
| UX-03 | auto_cleanup in README sandbox section | manual | — (doc review) | n/a |
| UX-04 | Bridge skips heartbeat when OS daemon installed | unit | `go test -race -v -run TestBridgeHeartbeat ./...` | ❌ Wave 0 |
| UX-05 | Existing worktree detected and reused | unit | `go test -race -v -run TestWorktreeReuse ./internal/git/...` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race -v ./internal/ui/... ./internal/git/...`
- **Per wave merge:** `go test -race -v ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/ui/mouse_scroll_test.go` — covers UX-01 (mouse wheel on session list, settings, search)
- [ ] `internal/ui/light_theme_test.go` or extend `styles_test.go` — covers UX-02 (light theme preview rendering)
- [ ] `internal/git/worktree_reuse_test.go` or extend `git_test.go` — covers UX-05 (GetWorktreeForBranch → reuse path)

*(UX-03 is documentation only; UX-04 is Python — consider a shell integration test or manual verification)*

## Sources

### Primary (HIGH confidence)
- `github.com/charmbracelet/bubbletea@v1.3.10/mouse.go` — mouse event types, `MouseButtonWheelUp/Down` constants, deprecation of `Type` field
- `internal/ui/home.go` (local codebase) — confirmed zero existing `tea.MouseMsg` handlers; `tea.WithMouseCellMotion()` at line 468; `cursor`/`viewOffset` scroll system
- `internal/ui/styles.go` (local codebase) — confirmed correct dual-palette theme system; `InitTheme()` correctly switches all Color* variables
- `internal/git/git.go` (local codebase) — confirmed `GetWorktreeForBranch()` at line 252; `CreateWorktree()` at line 141 does not pre-check
- `internal/session/conductor.go` (local codebase) — confirmed two heartbeat paths: `installHeartbeatDaemonLaunchd/Systemd` (OS daemon) vs `conductor/bridge.py:heartbeat_loop`
- `internal/session/userconfig.go` (local codebase) — confirmed `DockerSettings.AutoCleanup`, `GetAutoCleanup()` defaults to true, semantic meaning

### Secondary (MEDIUM confidence)
- `skills/agent-deck/references/sandbox.md` — auto_cleanup already documented there; README.md gap confirmed
- `.planning/research/STACK.md` — prior research notes on mouse support (consistent with findings)
- `cmd/agent-deck/conductor_cmd.go` — confirmed all heartbeat installation call sites

### Tertiary (LOW confidence)
- GitHub issue descriptions (referenced in STATE.md and docs/GSD-SPEC.md) — issue intent inferred from in-codebase references, not read directly

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all libraries already in go.mod
- Architecture (mouse): HIGH — bubbletea API verified from module cache; zero existing handlers confirmed
- Architecture (light theme): MEDIUM — root cause understood but fix approach needs validation (lipgloss wrap vs ANSI strip)
- Architecture (worktree): HIGH — `GetWorktreeForBranch` exists and tested; all CreateWorktree call sites identified
- Architecture (heartbeat): MEDIUM — two mechanisms confirmed; consolidation strategy is a judgment call
- Architecture (auto_cleanup docs): HIGH — zero-risk doc-only change; semantic meaning verified from code
- Pitfalls: HIGH — all from direct code inspection

**Research date:** 2026-03-13
**Valid until:** 2026-04-13 (bubbletea API stable; internal code stable unless concurrent phase work touches same files)
