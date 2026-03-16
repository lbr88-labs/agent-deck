# Feature Research

**Domain:** Session reliability and resume UX for Go+BubbleTea TUI session manager (v1.3)
**Researched:** 2026-03-12
**Confidence:** HIGH (all findings from codebase analysis; no speculative external research needed)

## Feature Landscape

### Table Stakes (Users Expect These)

Features that session manager users consider baseline behavior. Missing any of these causes
"is this broken?" frustration, not "that's a nice-to-have" disappointment.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Config survives save/reload cycle | Any persistent UI config that resets on restart is broken. Sandbox Docker image/limits are user-specified; losing them forces re-entry every session. | LOW | Storage layer already round-trips `sandbox` JSON through `tool_data` column. Root cause is likely in the newdialog/settings save path not writing `Sandbox` back to the instance before `SaveWithGroups`. Needs a targeted trace from dialog submission to storage call. |
| Stopped sessions visible in TUI | Users stop sessions intentionally and expect to resume them later. If stopped sessions vanish from the list, users cannot resume or delete them — they must recreate from scratch. | LOW | `StatusStopped` already exists as an enum value. The TUI filtering logic that hides "dead" sessions is conflating `error`/`stopped` with each other. Fix is a one-line predicate change in `home.go` list filtering. |
| Resume does not create duplicates | Starting a stopped session should reuse the existing entry, not add a second row. Duplicate entries break conductor status counts and confuse the user about which session is "real". | MEDIUM | `UpdateClaudeSessionsWithDedup` clears duplicate `ClaudeSessionID` fields at save time, but the bug is upstream: resume via CLI or TUI may call `NewInstance` instead of `Restart` on the existing record. Needs dedup applied at the resume-creation boundary, not just at save. |
| Auto-start works on Linux/WSL | Users running agent-deck on Linux or WSL expect `agent-deck &` (background launch) to work the same as macOS. TTY requirement is a platform surprise, not a design choice users accept. | MEDIUM | Claude Code (and others) call `isatty(stdout)` and refuse to run when stdout is redirected. Auto-start detaches stdout. Fix: use `tmux new-session -d` to launch inside a tmux pane so the tool sees a PTY regardless of the outer process's stdio. Session IDs used for resume after the auto-start also need verification — wrong IDs cause silent resume failure. |
| Mouse scroll works in session list | Terminal UIs with long session lists require scroll. Keyboard-only scroll (j/k) is a fallback, not a complete solution. Users with trackpads expect natural scroll. | LOW | `tea.WithMouseCellMotion()` is already passed to `tea.NewProgram`. The TUI receives mouse events but `home.go:Update()` has no `tea.MouseMsg` handler for the session list. Add `tea.MouseWheelUp`/`tea.MouseWheelDown` cases routing to the existing cursor movement logic. |
| Settings panel shows all configurable tools | If a user adds a custom tool in `config.toml`, they expect it to appear in settings so they can set it as the default. Missing it forces TOML editing for a basic UI preference. | LOW | `buildToolLists()` in `settings_panel.go` already reads `config.Tools` and appends custom entries. The gap (issue #318) is that custom tool entries in the `SettingDefaultTool` radio group do not display their `icon` field from the TOML definition, making them indistinguishable from built-ins. Likely also missing in the `newdialog` tool picker. Needs icon rendering for custom tool rows. |

### Differentiators (Competitive Advantage)

Features beyond baseline that improve reliability for power users running multiple conductors
and AI agent workflows.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Mouse click to select session | Beyond scroll: clicking a session in the list to jump to it directly. Reduces friction when managing 10+ sessions. | MEDIUM | Requires hit-testing the rendered list rows against `tea.MouseMsg.Y` coordinates. The list rendering in `home.go` is custom (not using `bubbles/list`), so hit-test must account for group headers, indent levels, and the current scroll offset. More involved than wheel scroll. |
| Click-to-attach (double-click or Enter after click) | Selecting then attaching in one gesture. Natural for trackpad users. | HIGH | Needs click-select first. Double-click detection requires stateful last-click timestamp tracking. Bubble Tea does not provide double-click natively. |
| auto_cleanup documented in config reference | Users enabling sandbox mode discover `auto_cleanup` only by reading source or config-reference.md. Explicit docs surface a behavior that directly affects disk/container hygiene. | LOW | `auto_cleanup = true` default already in `userconfig.go` template and `config-reference.md`. Issue #228 is documentation completeness — ensure the option appears in the user-facing docs site or README alongside the sandbox section, with an explanation of what gets cleaned and when. |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Auto-hide stopped sessions (filtered out of main list by default) | "Stopped sessions are clutter" | Hides the very sessions users want to resume. The original filter bug is the anti-feature; fixing it means stopped sessions are visible. A separate toggle to hide stopped sessions is fine if the default is visible. | Fix visibility first (table stake). If demand exists for filtering, add a `[f]ilter` toggle in the TUI header — same pattern as existing group expand/collapse. |
| Merge stopped+error into one status | "Simplifies the status model" | `stopped` (user intent) and `error` (crash) have different semantics. Conductor templates already differentiate them: "do not restart stopped sessions." Merging breaks conductor logic. | Keep distinct. Ensure TUI renders them with different colors/icons so users can tell them apart at a glance. |
| Persist mouse mode globally in tmux config | "Enable mouse mode in tmux for all sessions" | `set -g mouse on` in `~/.tmux.conf` affects all tmux sessions outside agent-deck. Agent-deck already enables mouse per-session via `EnableMouseMode` on attach (deferred to `EnsureConfigured()`). Touching global tmux config violates user sovereignty. | Mouse in the BubbleTea TUI (for the session list) is separate from mouse in the attached tmux pane. Fix the TUI mouse handler; leave per-session tmux mouse mode as-is. |
| Full TUI re-architecture to use bubbles/list | "Would fix mouse, scroll, and rendering in one shot" | `home.go` is ~8500 lines built on a custom list model. Migrating to `charmbracelet/bubbles` list would be a full rewrite of the session rendering, group tree, keyboard handling, and status display. Risk of regression across every existing feature. | Add targeted `tea.MouseMsg` handling for wheel events and hit-testing directly in the existing model. Scope is ~30 lines for wheel scroll, ~80 lines for click-select. |

## Feature Dependencies

```
[Stopped sessions visible in TUI]                 -- prerequisite for:
    └──enables──> [Resume does not create duplicates]
                      └──enables──> [Auto-start works on Linux/WSL]
                                        (session IDs must be correct post-dedup)

[Sandbox config persistence]                       -- independent, no dependencies

[Mouse scroll in session list]                     -- independent, no dependencies
    └──enables──> [Mouse click to select session]
                      └──enables──> [Click-to-attach]

[Settings custom tools display]                    -- independent, no dependencies

[auto_cleanup documentation]                       -- independent, no dependencies
```

### Dependency Notes

- **Stopped sessions visible requires no other fix first:** The filter predicate in `home.go` is self-contained. Fix this first — it unblocks manual resume testing which validates the dedup fix.
- **Resume dedup depends on stopped visibility:** You cannot confirm dedup is fixed if you cannot see both the original stopped session and any accidental duplicate in the same list view.
- **Auto-start TTY fix can be developed independently:** The WSL/Linux TTY issue is in the process launch path (`cmd/agent-deck/main.go` or platform-specific launch), not in the session list. However, validating it correctly requires that resume creates the right session record, so dedup should be stable first.
- **Mouse click depends on mouse scroll:** Scroll is the simpler sub-feature (no hit-testing needed). Confirm scroll events are handled before implementing click-select, which requires coordinate mapping.
- **Settings custom tools and auto_cleanup docs are fully independent:** No runtime dependencies on other features. Can be done in any order or in parallel with other fixes.

## MVP Definition

### Ship in v1.3 (All Seven Items)

All seven issues are scoped, bounded, and directly fix reported regressions. None should be deferred.

- [ ] **Sandbox config persistence (#320)** — Data loss on restart. Highest user frustration.
- [ ] **Stopped sessions visible in TUI (#307)** — Prerequisite for manual resume workflows.
- [ ] **Session deduplication on resume (#224)** — Corrupts conductor session counts.
- [ ] **Auto-start TTY fix for WSL/Linux (#311)** — Blocks Linux/WSL users entirely.
- [ ] **Mouse/trackpad scroll support (#262, #254)** — `tea.WithMouseCellMotion()` is already active but unhandled.
- [ ] **Settings custom tools completion (#318)** — Custom tool icons missing in settings radio group.
- [ ] **auto_cleanup documentation (#228)** — Low-effort, high-clarity documentation fix.

### Prioritization Within v1.3

Order matters for implementation because of the dependency chain above:

1. Stopped sessions visible (enables manual resume testing)
2. Sandbox config persistence (independent, quick win)
3. Session deduplication (can now be visually verified)
4. Auto-start TTY fix (session ID correctness relies on dedup being stable)
5. Mouse scroll (independent, isolated change)
6. Settings custom tools (independent, isolated change)
7. auto_cleanup docs (can go in any phase, zero risk)

### Defer to v1.4+

- Mouse click-to-select (requires coordinate hit-testing, higher complexity)
- Click-to-attach (requires double-click state machine, depends on click-select)
- Performance testing at 50+ sessions (out of scope per PROJECT.md)

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Stopped sessions visible (#307) | HIGH | LOW | P1 |
| Sandbox config persistence (#320) | HIGH | LOW | P1 |
| Session deduplication (#224) | HIGH | MEDIUM | P1 |
| Auto-start TTY fix (#311) | HIGH | MEDIUM | P1 |
| Mouse scroll (#262, #254) | MEDIUM | LOW | P1 |
| Settings custom tools (#318) | MEDIUM | LOW | P1 |
| auto_cleanup docs (#228) | LOW | LOW | P1 |
| Mouse click-to-select | MEDIUM | MEDIUM | P2 |
| Click-to-attach | LOW | HIGH | P3 |

**Priority key:**
- P1: In v1.3 scope. All regressions or missing basics.
- P2: Next milestone candidate.
- P3: Future consideration only.

## Existing Infrastructure Analysis

Understanding what already exists prevents reimplementing and confirms where the bugs actually are.

| Feature | Existing Assets | Gap |
|---------|----------------|-----|
| Sandbox persistence | `InstanceData.Sandbox`, `SandboxConfig`, `MarshalToolData`/`UnmarshalToolData`, `decodeSandboxConfig` — full round-trip tested in `TestStorageSaveWithGroups_PersistsSandboxConfig` | Sandbox not written back to instance record at save time in the UI dialog flow |
| Stopped session visibility | `StatusStopped` enum, `statusToString("inactive")`, conductor templates distinguish stopped vs error | TUI list filter hides stopped same as error |
| Deduplication | `UpdateClaudeSessionsWithDedup` (oldest-wins by `CreatedAt`) called in `SaveWithGroups` | Not called at resume-creation path; duplicate row created before save |
| Auto-start TTY | `tmux.ReconnectSessionLazy`, `wrapIgnoreSuspend`, `isatty` pattern documented in issue | Platform-specific launch in main.go does not route through a PTY; session IDs post-launch need verification |
| Mouse in TUI | `tea.WithMouseCellMotion()` already active in `tea.NewProgram` | `home.go:Update()` has no `tea.MouseMsg` handler cases |
| Settings custom tools | `buildToolLists()` reads `config.Tools`, appends custom names and values | Custom tool `icon` field from `ToolDef` not rendered in radio group; `newdialog` tool picker may also miss icons |
| auto_cleanup | `DockerSettings.AutoCleanup`, `GetAutoCleanup()`, default `true`, `config-reference.md` entry | Not mentioned in main README or sandbox guide intro section |

## Sources

- `internal/session/instance.go` — `UpdateClaudeSessionsWithDedup`, `SandboxConfig`, `StatusStopped` (HIGH confidence, direct codebase)
- `internal/session/storage.go` — `SaveWithGroups`, `MarshalToolData`, `decodeSandboxConfig` (HIGH confidence, direct codebase)
- `internal/session/userconfig.go` — `ToolDef`, `DockerSettings.AutoCleanup`, `GetCustomToolNames` (HIGH confidence, direct codebase)
- `internal/ui/settings_panel.go` — `buildToolLists`, `SettingDefaultTool`, tool radio group rendering (HIGH confidence, direct codebase)
- `cmd/agent-deck/main.go` line 468 — `tea.WithMouseCellMotion()` already active (HIGH confidence, direct codebase)
- `.planning/PROJECT.md` — Issue numbers, milestone goal, constraints (HIGH confidence, project documentation)
- `skills/agent-deck/references/config-reference.md` and `sandbox.md` — `auto_cleanup` documentation coverage (HIGH confidence, direct file read)

---
*Feature research for: v1.3 Session Reliability and Resume UX (agent-deck)*
*Researched: 2026-03-12*
