# Project Research Summary

**Project:** agent-deck v1.3 — Session Reliability & Resume
**Domain:** Go + Bubble Tea TUI terminal session manager (subsequent milestone research)
**Researched:** 2026-03-12
**Confidence:** HIGH

## Executive Summary

Agent-deck v1.3 is a focused reliability and UX milestone on an established Go/Bubble Tea/tmux/SQLite stack. No new dependencies are required: every one of the seven issues in scope is solvable with existing packages, existing schema, and existing infrastructure. The foundational architecture is sound. The bugs in this milestone are primarily about data flow gaps (sandbox config not reaching the restart path), missing event handlers (mouse events received but not dispatched), and UX omissions (stopped sessions visible but not clearly resumable). The research confirms these are all bounded, localized fixes rather than architectural changes.

The recommended implementation order follows a dependency chain: fix sandbox persistence and stopped-session visibility first, which establishes the correct baseline state for the session list. Session deduplication on resume comes next and can now be visually confirmed. The auto-start TTY fix follows since it relies on correct session ID propagation from the dedup fix. Mouse support and settings tool icons are fully independent and can be parallelized with any phase. The auto_cleanup documentation change carries zero implementation risk and can be merged at any time.

The primary risk area is the `toolDataBlob` serialization boundary in `statedb/migrate.go`: it is a wide-signature function with no compile-time enforcement of field coverage. Any incomplete update silently drops data. The recommended mitigation is to refactor `MarshalToolData` to accept a struct parameter before adding any new fields, and to write a round-trip integration test against a fresh SQLite `Storage` instance as an acceptance criterion for the sandbox persistence fix. A secondary risk is mouse support inside tmux: the interaction between Bubble Tea's `WithMouseCellMotion` and tmux's own mouse mode must be tested in context, not just in a standalone terminal.

---

## Key Findings

### Recommended Stack

The stack requires no changes for this milestone. Go 1.24, Bubble Tea v1.3.10, bubbles v0.21.0, lipgloss v1.1.0, modernc.org/sqlite v1.44.3, and golang.org/x/term v0.37.0 are all already present and validated. Mouse support uses `tea.MouseButtonWheelUp` / `tea.MouseButtonWheelDown` (not the deprecated `tea.MouseWheelUp` / `tea.MouseWheelDown` constants, deprecated as of v1.3.x). TTY detection uses `term.IsTerminal(int(os.Stdin.Fd()))` from the already-imported `golang.org/x/term`. Sandbox persistence uses the existing `toolDataBlob` JSON-blob pattern rather than new SQLite columns.

**Core technologies:**
- **Go 1.24:** Runtime — no change
- **charmbracelet/bubbletea v1.3.10:** TUI event loop — `tea.MouseMsg` handler to be added; `WithMouseCellMotion` already active in `main.go:468`
- **modernc.org/sqlite v1.44.3:** No-CGO persistence — existing WAL + `toolDataBlob` pattern covers all v1.3 needs; no new schema columns
- **golang.org/x/term v0.37.0:** TTY detection for WSL/Linux auto-start fix — already a direct dependency, used in `drainStdin()`

See [STACK.md](./STACK.md) for the full feature-by-feature stack analysis and alternatives considered.

### Expected Features

All seven issues are classified as table stakes (regressions or missing basics). None should be deferred.

**Must have (table stakes — all seven in v1.3 scope):**
- **Sandbox config persistence (#320):** Config survives save/reload/restart cycle. Data loss on restart is the highest-frustration regression.
- **Stopped sessions visible in TUI (#307):** Users stop sessions intentionally; hiding them removes resume capability.
- **Session deduplication on resume (#224):** Resuming must reuse the existing record, not create a duplicate that breaks conductor counts.
- **Auto-start TTY fix for WSL/Linux (#311):** Linux/WSL users are fully blocked; tools refuse to start without a PTY.
- **Mouse/trackpad scroll support (#262, #254):** Mouse events are already delivered to `Update()` but have no handler; the gap is approximately 30 lines of code.
- **Settings custom tools completion (#318):** Custom tool icons missing from settings radio group, making custom tools visually indistinguishable from built-ins.
- **auto_cleanup documentation (#228):** Option not mentioned in README or sandbox guide intro despite being default-on behavior.

**Should have (competitive, defer to v1.4+):**
- Mouse click-to-select: requires coordinate hit-testing against a custom list renderer; higher complexity than wheel scroll.
- Click-to-attach: depends on click-select; requires a double-click state machine Bubble Tea does not provide natively.

**Defer (v2+):**
- Performance testing at 50+ sessions (out of scope per PROJECT.md)
- `bubbles/list` migration (full rewrite of home.go custom model; not a v1.3 fix)

See [FEATURES.md](./FEATURES.md) for the full feature landscape, dependency graph, and prioritization matrix.

### Architecture Approach

All v1.3 changes are targeted edits to existing files. No new packages, no new SQLite columns, no new files. The `toolDataBlob` struct in `internal/statedb/migrate.go` is the single serialization contract for per-session extended fields; the sandbox round-trip already exists but has a data-flow gap in the restart path. The Bubble Tea message flow in `home.go:Update()` is the single entry point for all input events; mouse support is a new `case tea.MouseMsg:` in that switch. The deduplication fix is a call-site addition: `UpdateClaudeSessionsWithDedup()` must run in-memory immediately at resume, not only at persist time.

**Major components and v1.3 touch points:**
1. `internal/statedb/migrate.go` — `toolDataBlob` and `MarshalToolData`/`UnmarshalToolData`; refactor to struct parameter before adding fields (#320)
2. `internal/session/storage.go` — sandbox round-trip verification; confirm `inst.Sandbox` survives load to restart cycle (#320)
3. `internal/ui/home.go` — stopped-session preview pane differentiation (#307); `tea.MouseMsg` handler addition (#262, #254); dedup call at resume site (#224)
4. `internal/ui/settings_panel.go` — add `Icon` field from `ToolDef` to custom tool display names in `buildToolLists()` (#318)
5. `cmd/agent-deck/main.go` — WSL/Linux TTY detection and safe launch path investigation (#311)

See [ARCHITECTURE.md](./ARCHITECTURE.md) for the full component diagram, data flow traces, anti-patterns, and build order.

### Critical Pitfalls

See [PITFALLS.md](./PITFALLS.md) for all 9 pitfalls with recovery strategies, the "looks done but isn't" checklist, and performance traps.

1. **Sandbox config silently serialized as null:** If `inst.Sandbox` is nil at `SaveWithGroups` time, the JSON blob stores null and every subsequent load returns nil Sandbox — the session restarts without its container with no error. Prevention: write a round-trip test against a fresh `Storage` instance; never swallow `json.Marshal` errors in `MarshalToolData`.

2. **toolDataBlob field drift (6-location update required):** Adding any field to `toolDataBlob` requires updating the struct, `MarshalToolData`, `UnmarshalToolData`, both call sites in `storage.go`, and `MigrateFromJSON`. Missing any one location produces silent data loss. Prevention: refactor `MarshalToolData` to accept a `toolDataBlob` struct so the compiler catches missing fields.

3. **In-memory dedup window between Restart() and SaveWithGroups:** Conductors read `h.instances` in memory. Two sessions sharing a `ClaudeSessionID` between a `Restart()` call and the next persist give conductors incorrect counts. Prevention: call `UpdateClaudeSessionsWithDedup(h.instances)` immediately after `inst.Restart()` in the TUI handler.

4. **Mouse events conflict with tmux mouse mode:** `WithMouseCellMotion` and `set -g mouse on` in `.tmux.conf` compete; scroll events may go to tmux copy-mode instead of Bubble Tea. Testing outside tmux gives a false pass. Prevention: always test scroll and click inside a tmux session.

5. **Slow Update() causes spurious KeyMsg from mouse motion (Bubble Tea issue #1047):** If any blocking I/O enters the mouse event path, buffered terminal bytes are misinterpreted as key presses. Prevention: the mouse click handler must be O(1) — compute item index from `msg.Y` mathematically — and must not perform any synchronous I/O.

---

## Implications for Roadmap

The dependency chain is clear from combined research. Five phases map directly to the architecture research build order from ARCHITECTURE.md:

### Phase 1: Storage Foundation
**Rationale:** Sandbox persistence (#320) must come first because if `inst.Sandbox` is nil after load, the restart path is also broken — fixing storage first validates the baseline for every subsequent feature that touches session restart. Refactoring `MarshalToolData` to a struct parameter is a prerequisite, not an optimization.
**Delivers:** Sandbox config survives stop/restart cycles; `MarshalToolData` refactored to struct parameter; `busy_timeout` verified in `statedb.Open()`; auto_cleanup documentation added to README/sandbox guide.
**Addresses:** Issue #320 (sandbox persistence); issue #228 (auto_cleanup docs — zero-risk doc change, bundle here to avoid it being forgotten).
**Avoids:** Pitfalls 1 and 2 (sandbox null serialization, toolDataBlob field drift). Acceptance criterion: round-trip test passes against a fresh `Storage` instance, not the same in-memory instance that saved.

### Phase 2: Session List Correctness
**Rationale:** Stopped-session visibility (#307) must be confirmed working before dedup (#224) can be visually validated. If stopped sessions are hidden, you cannot see both the original and any accidental duplicate in the same list view.
**Delivers:** Stopped sessions appear in main TUI list with distinct styling from error sessions; preview pane differentiates stopped (intentional) from error (crash) with distinct action guidance; session picker dialog remains correctly filtered for conductor flows.
**Addresses:** Issue #307 (stopped sessions visible); foundation for issue #224.
**Avoids:** Pitfall 6 (stopped sessions invisible due to filter applied in wrong context). Requires audit of all `StatusStopped` exclusion sites before writing new render code.

### Phase 3: Resume Deduplication
**Rationale:** With stopped sessions now visible, dedup behavior can be confirmed: the user should see one session entry before and after resume, not two. Concurrent-write behavior must also be tested.
**Delivers:** Resuming a stopped session reuses the existing record; conductor session counts remain correct; `UpdateClaudeSessionsWithDedup` runs in-memory at resume site; hook status ordering audited.
**Addresses:** Issue #224 (session deduplication on resume).
**Avoids:** Pitfall 3 (in-memory dedup window); Pitfall 5 from PITFALLS.md (hook status race — audit hook ordering while touching dedup logic). Write a concurrent-write test covering two `Storage` instances against the same SQLite file.

### Phase 4: Auto-Start TTY Fix (WSL/Linux)
**Rationale:** This is the most uncertain fix (root cause requires WSL/Linux reproduction). Dedup should be stable first because correct session ID propagation is required for validating that the resumed session after auto-start is the right record.
**Delivers:** `agent-deck session start` works from non-interactive contexts on WSL/Linux; tool processes always receive a PTY regardless of how agent-deck itself was invoked.
**Addresses:** Issue #311 (auto-start TTY fix for WSL/Linux).
**Avoids:** Pitfall 4 (auto-start TTY breaks tool interactivity). Needs root cause investigation before implementation begins; flag for hands-on debugging session on a WSL/Linux environment.

### Phase 5: Mouse Support and Settings Polish
**Rationale:** Fully independent of the session lifecycle fixes. Wheel scroll is the simpler sub-feature (no hit-testing); confirm it works before implementing click-select. Settings tool icons are similarly isolated and can be done in parallel.
**Delivers:** Mouse wheel scroll in session list and other scrollable areas (settings, search, dialogs); custom tool icons in settings radio group and newdialog tool picker.
**Addresses:** Issues #262 and #254 (mouse/trackpad scroll); issue #318 (settings custom tools completion).
**Avoids:** Pitfalls 7 and 8 from PITFALLS.md (mouse/tmux conflicts; slow Update causing spurious KeyMsg). Click-to-select deferred to v1.4. Mouse handler must be O(1) and contain no blocking I/O.

### Phase Ordering Rationale

- Storage before UI: If the sandbox load path is broken, any restart behavior tested in later phases is against a broken baseline.
- Visibility before dedup: You cannot confirm dedup visually without seeing both sessions simultaneously in the list.
- Dedup before TTY: Session ID correctness after auto-start requires dedup to be stable for validation.
- Mouse is fully independent: No ordering requirement relative to Phases 2-4; can be parallelized if two developers are working.
- Documentation (auto_cleanup #228) has no ordering requirement; bundled into Phase 1 to ensure it is not deferred indefinitely.

### Research Flags

Phases needing deeper investigation during planning:

- **Phase 4 (Auto-Start TTY Fix):** Root cause of #311 on WSL/Linux is not documented in available issue context. Three candidate failure modes are identified (agent-deck's own stdio, env variable leakage, `wrapIgnoreSuspend` incompatibility on Linux), but the exact one is not confirmed without reproduction. Flag for a dedicated hands-on debugging session on WSL before writing implementation tasks.

Phases with standard, well-documented patterns (skip research-phase):

- **Phase 1 (Storage Foundation):** `toolDataBlob` pattern is fully documented in codebase; round-trip is already partially tested; the fix is a code path trace and a structural refactor, not a design decision.
- **Phase 2 (Session List Correctness):** `rebuildFlatItems` predicate and preview pane differentiation are standard Bubble Tea render patterns with no unknowns.
- **Phase 3 (Resume Deduplication):** `UpdateClaudeSessionsWithDedup` exists and is documented; adding a call site is mechanical.
- **Phase 5 (Mouse Support):** Bubble Tea v1.3.10 mouse API is fully verified against module cache; implementation pattern is documented in STACK.md with concrete code examples.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Verified against go.mod, go.sum, and bubbletea v1.3.10 module cache. No new dependencies. Deprecated mouse constants identified and documented. |
| Features | HIGH | All seven issues traced to specific code locations in the codebase. Root causes identified for all but #311 (WSL/Linux reproduction needed). |
| Architecture | HIGH | All touch points identified from direct source reads. Component boundaries and data flow confirmed. No speculative findings. |
| Pitfalls | HIGH | Derived from direct codebase analysis and documented Bubble Tea issues (#162, #1047). SQLite WAL contention from official SQLite docs. |

**Overall confidence:** HIGH

### Gaps to Address

- **Auto-start TTY root cause (#311):** The exact failure path on WSL/Linux is not confirmed without reproduction. Three candidate causes identified. During planning, assign a dedicated investigation session on a Linux/WSL system before writing implementation tasks.
- **`busy_timeout` in `statedb.Open()`:** Research identified SQLite WAL contention as a latent risk at 20+ rapidly-transitioning sessions. Whether `PRAGMA busy_timeout = 5000` is already set was not confirmed in the research pass. Verify at the start of Phase 1 as a 10-minute check before touching any storage code.
- **Concurrent-write dedup behavior:** The cross-process dedup scenario (two agent-deck windows open simultaneously) was analyzed but no existing test covers it. Write a concurrent-write test in Phase 3 before closing #224.

---

## Sources

### Primary (HIGH confidence)

- `internal/session/instance.go` — `SandboxConfig`, `Start()`, `UpdateClaudeSessionsWithDedup()`, `wrapIgnoreSuspend()`
- `internal/session/storage.go` — `SaveWithGroups`, `decodeSandboxConfig`, `convertToInstances`
- `internal/statedb/migrate.go` — `toolDataBlob`, `MarshalToolData`, `UnmarshalToolData`, `MigrateFromJSON`
- `internal/ui/home.go` — `rebuildFlatItems()`, `Update()`, `getOtherActiveSessions()`, `createSessionInGroupWithWorktreeAndOptions()`
- `internal/ui/settings_panel.go` — `buildToolLists()`, `SettingDefaultTool` radio group
- `internal/ui/session_picker_dialog.go:41` — `StatusStopped` exclusion confirmed in conductor picker
- `cmd/agent-deck/main.go:468` — `tea.WithMouseCellMotion()` already active
- `go.mod` and `go.sum` — all current deps and versions verified
- `/Users/ashesh/go/pkg/mod/github.com/charmbracelet/bubbletea@v1.3.10/mouse.go` — verified current mouse API and deprecation notices
- `internal/session/storage_test.go:260` — `TestStorageSaveWithGroups_PersistsSandboxConfig` confirms schema is correct
- `.planning/PROJECT.md` — v1.3 feature scope and constraints

### Secondary (MEDIUM confidence)

- `.planning/codebase/ARCHITECTURE.md` — existing system architecture map (project documentation)
- `skills/agent-deck/references/config-reference.md` and `sandbox.md` — auto_cleanup documentation coverage
- `.github-issue-context-254.json` — confirmed zero `MouseMsg` handlers in codebase

### Tertiary (supporting context)

- [Bubble Tea GitHub Issue #162](https://github.com/charmbracelet/bubbletea/issues/162) — Mouse mode disables text selection
- [Bubble Tea GitHub Issue #1047](https://github.com/charmbracelet/bubbletea/issues/1047) — KeyMsg emitted during mouse drag when Update() is slow
- [SQLite WAL mode documentation](https://sqlite.org/wal.html) — WAL contention and busy_timeout behavior
- [Claude Code Hooks reference](https://code.claude.com/docs/en/hooks) — hook lifecycle and concurrent firing behavior
- [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/) — Keep Update() fast; message ordering not guaranteed

---
*Research completed: 2026-03-12*
*Ready for roadmap: yes*
