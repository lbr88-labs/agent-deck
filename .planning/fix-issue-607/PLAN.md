# fix-issue-607 — TUI row offset drift when scrolling

## Problem summary

Since v1.5.1 the TUI accumulates vertical rendering drift while the user scrolls the session list on every terminal tested (Ghostty 1.3.1, Terminal.app, Warp). This is a regression from v0.27.5. Setting `full_repaint = true` in `[display]` reduces severity because it fires `tea.ClearScreen` on the 2-second tick, but drift still accumulates between ticks.

Reporter: Max (`@maxfi`), issue #607.

## Reproducer

1. `agent-deck` v1.7.10 with ≥1 screen of sessions.
2. Hold `j` or spin the mouse wheel. Observe rows duplicate / stack with increasing vertical offset.
3. With `full_repaint = true` the drift is smaller but still visible for up to 2 seconds between clears.

## Data-flow trace

Every navigation event that shifts the viewport converges on `h.syncViewport()`. Incremental-redraw drift occurs whenever Bubble Tea's diff renderer loses cursor tracking — possible when `lipgloss.Width` / `go-runewidth` widths disagree, or when the `CSIuReader` stdin wrapper (added at `cmd/agent-deck/main.go:639` in v1.5.x) batches reads in a way that delays `tea.WindowSizeMsg`.

The ONLY existing escape hatch is the tick-based `tea.ClearScreen` emitted under `full_repaint = true`:

| Hop | File : line | What happens |
|---|---|---|
| 1. stdin wrap | `cmd/agent-deck/main.go:639` | `tea.WithInput(ui.NewCSIuReader(os.Stdin))` |
| 2. translate | `internal/ui/keyboard_compat.go:294-335` | `csiuReader.Read` batches + translates CSI-u |
| 3. input dispatch | bubbletea runtime | emits `tea.KeyMsg` / `tea.MouseMsg` |
| 4. cursor move | `internal/ui/home.go:3106-3121` (mouse wheel) and `5151-5230` (j/k, ctrl+u/d, ctrl+b/f) | `h.cursor±±`, `h.syncViewport()` |
| 5. viewport sync | `internal/ui/home.go:1420+` | adjusts `h.viewOffset` |
| 6. return cmd | `return h, h.fetchSelectedPreview()` or `return h, nil` | NO `tea.ClearScreen` |
| 7. tick (every 2s) | `internal/ui/home.go:4346-4349` | `if h.fullRepaint { cmds = append(cmds, tea.ClearScreen) }` — the only clear |
| 8. render | bubbletea `renderer.flush` | diff-based paint; drift sticks until next tick |

The drift window is hops 4→7: every input-driven scroll between ticks paints incrementally with whatever cached width/cursor state the renderer is holding. Under `full_repaint = true` users already opt in to full clears; they just never get one on the event that causes drift (scroll/key).

## Fix hypothesis

**When `fullRepaint` is enabled, append `tea.ClearScreen` on every `tea.KeyMsg` and on mouse wheel `tea.MouseMsg` inside `Update` — not only on the 2-second tick.**

This extends the existing opt-in behaviour to the exact events that cause drift. It does NOT touch the CSIuReader (ripping that out would regress #535's CSI-u fix). It does NOT change default behaviour (fullRepaint defaults to false; existing users unaffected).

Implementation shape: rename current `Update` → `updateInner`, add a thin outer `Update` that post-processes the returned `tea.Cmd` under `fullRepaint` + `KeyMsg|WheelUp|WheelDown`. Single intervention point automatically covers all 27 `syncViewport()` call sites inside Update — no per-handler sprinkle needed.

## Failing tests (TDD RED)

File: `internal/ui/home_repaint_test.go` (new).

1. `TestFullRepaint_ClearsOnMouseWheelDown_Issue607` — fullRepaint=true + wheel-down → cmd yields `tea.clearScreenMsg`.
2. `TestFullRepaint_ClearsOnMouseWheelUp_Issue607` — fullRepaint=true + wheel-up → cmd yields `tea.clearScreenMsg`.
3. `TestFullRepaint_ClearsOnKeyNavigation_Issue607` — fullRepaint=true + `j` → cmd yields `tea.clearScreenMsg`.
4. `TestFullRepaint_Disabled_NoClearOnScroll` — regression guard: fullRepaint=false + wheel-down → cmd does NOT include `tea.clearScreenMsg`.
5. `TestFullRepaint_Disabled_NoClearOnKey` — regression guard: fullRepaint=false + `j` → cmd does NOT include `tea.clearScreenMsg`.
6. `TestFullRepaint_NonNavKeyStillClears` — fullRepaint=true + any KeyMsg → clears (covers ctrl+u, ctrl+d, ctrl+b, ctrl+f, g, G, etc. via a single rule rather than per-key enumeration).

All six assert against the composed `tea.Cmd` by executing it and walking `tea.BatchMsg` for a message whose reflect type name is `tea.clearScreenMsg`.

## Implementation sketch (TDD GREEN)

```go
// internal/ui/home.go

// Update implements tea.Model. It delegates to updateInner and, when
// fullRepaint is enabled, appends tea.ClearScreen on KeyMsg and mouse
// wheel events to prevent incremental-redraw drift between tick-based
// clears (issue #607).
func (h *Home) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    model, cmd := h.updateInner(msg)
    if !h.fullRepaint {
        return model, cmd
    }
    switch m := msg.(type) {
    case tea.KeyMsg:
        _ = m
        return model, appendClearScreen(cmd)
    case tea.MouseMsg:
        if m.Button == tea.MouseButtonWheelUp || m.Button == tea.MouseButtonWheelDown {
            return model, appendClearScreen(cmd)
        }
    }
    return model, cmd
}

func appendClearScreen(cmd tea.Cmd) tea.Cmd {
    if cmd == nil {
        return tea.ClearScreen
    }
    return tea.Batch(cmd, tea.ClearScreen)
}

// Rename the existing big Update to updateInner. Signature unchanged.
func (h *Home) updateInner(msg tea.Msg) (tea.Model, tea.Cmd) {
    // ... existing body ...
}
```

## Scope boundaries

**MAY change:**
- `internal/ui/home.go` — rename `Update` → `updateInner`, add new `Update` + `appendClearScreen`.
- `internal/ui/home_repaint_test.go` — new test file.
- `.claude/release-tests.yaml` — append 6 regression entries (phase 8).

**MUST NOT change:**
- `internal/ui/keyboard_compat.go` — do NOT touch CSIuReader (guarded by PR #619's mechanism tests).
- `cmd/agent-deck/main.go` — do NOT remove `tea.WithInput(...)` (regresses #535).
- Any other file in the repo.

## Parallel-paths audit checklist

- [ ] `tea.MouseMsg` wheel-up path triggers ClearScreen under fullRepaint.
- [ ] `tea.MouseMsg` wheel-down path triggers ClearScreen under fullRepaint.
- [ ] `tea.KeyMsg` path triggers ClearScreen under fullRepaint (catches j, k, ctrl+u/d/b/f, g, G, arrow keys — everything routed through the same case).
- [ ] `fullRepaint == false` → no ClearScreen on either message type (default-user regression guard).
- [ ] Tick-based ClearScreen at `home.go:4348` still fires (pre-existing behaviour unchanged).
- [ ] Theme-switch ClearScreen at `home.go:3809, 4431, 4433` still fires (pre-existing behaviour unchanged).
- [ ] Non-wheel MouseMsg (clicks, drags) under fullRepaint does NOT trigger extra ClearScreen (clicks don't scroll; over-clearing them would flicker).

## Release gate

v1.7.11. Branch `fix/607-full-repaint-on-nav`. PR body closes #607.
