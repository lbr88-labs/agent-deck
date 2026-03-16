---
phase: 18-wayland-key-input
plan: "01"
subsystem: ui
tags: [wayland, keyboard, input, tui, compatibility]
dependency_graph:
  requires: []
  provides: [keyboard-compat-layer, kitty-protocol-disable]
  affects: [internal/ui, cmd/agent-deck]
tech_stack:
  added: []
  patterns: [CSI-u-parsing, escape-sequence-injection, io.Reader-wrapping]
key_files:
  created:
    - internal/ui/keyboard_compat.go
    - internal/ui/keyboard_compat_test.go
  modified:
    - cmd/agent-deck/main.go
decisions:
  - "Use escape-sequence disable (\\x1b[>0u) rather than input filter as primary fix: cleaner, works before Bubble Tea starts reading, no raw-mode side effects"
  - "Keep CSIuReader as public belt-and-suspenders fallback API without wiring it into main.go startup"
  - "modifier=5 means ctrl-only (1+4), not shift+ctrl; Kitty modifier encoding is 1+bitmask"
metrics:
  duration_minutes: 4
  completed_date: "2026-03-16"
  tasks_completed: 2
  files_created: 2
  files_modified: 1
---

# Phase 18 Plan 01: Wayland Key Input Compatibility Summary

**One-liner:** Kitty keyboard protocol disable via `\x1b[>0u` escape sequence at TUI startup, plus CSI u parser and passthrough reader for belt-and-suspenders coverage.

## What Was Built

### `internal/ui/keyboard_compat.go`

Keyboard compatibility layer for Wayland/Kitty keyboard protocol:

- `DisableKittyKeyboard(w io.Writer)` writes `\x1b[>0u` to push keyboard mode 0 (legacy) on the Kitty protocol stack. Tells Ghostty, Foot, Alacritty to stop sending CSI u sequences and revert to standard key reporting. Safe to call on non-Kitty terminals (they ignore unknown sequences).
- `RestoreKittyKeyboard(w io.Writer)` writes `\x1b[<u` to pop the keyboard mode stack on TUI exit.
- `ParseCSIu(data []byte) *tea.KeyMsg` parses `ESC [ codepoint ; modifier u` sequences. Handles shift (modifier bit 1), ctrl (modifier bit 4), and maps codepoints 13/9/27/127/32 to their tea key types.
- `NewCSIuReader(r io.Reader) io.Reader` wraps any reader to translate CSI u sequences to legacy bytes in-flight. Belt-and-suspenders fallback for terminals that partially honor the disable.

### `cmd/agent-deck/main.go`

Added before `tea.NewProgram`:

```go
ui.DisableKittyKeyboard(os.Stdout)
defer ui.RestoreKittyKeyboard(os.Stdout)
```

This executes after any pre-TUI stdout writes (web server address, etc.) and before Bubble Tea starts consuming stdin, so the terminal receives the mode-change before any key events are generated.

## Tasks Completed

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Create keyboard_compat.go + tests (TDD) | 6d83c71 |
| 2 | Integrate DisableKittyKeyboard into TUI startup | c96312c |

## Test Coverage

All tests pass:
- `TestParseCSIu/*` — 10 subtests covering shift, ctrl, special codepoints, non-CSI-u
- `TestParseCSIuCtrlA` — modifier=5 decoding
- `TestDisableKittyKeyboard` — escape sequence content
- `TestRestoreKittyKeyboard` — escape sequence content
- `TestCSIuReaderPassesCSIuShiftM` — CSI u translation to "M"
- `TestCSIuReaderPassesNormalASCII` — plain byte pass-through
- `TestCSIuReaderPassesStandardEscapeSequences` — arrow key pass-through
- `TestCSIuReaderMixedInput` — "a + shift+R CSI + b" -> "aRb"

## Deviations from Plan

None — plan executed exactly as written.

The plan's `<action>` block for Task 2 concluded with the simpler approach (escape sequence only, no `tea.WithInput` wrapping), and that is what was implemented. The `CSIuReader` remains as a public API per the plan's intent.

## Self-Check: PASSED

### Files Created/Modified
- `internal/ui/keyboard_compat.go` — 175 lines (min_lines: 80 per plan)
- `internal/ui/keyboard_compat_test.go` — 183 lines (min_lines: 100 per plan)
- `cmd/agent-deck/main.go` contains `DisableKittyKeyboard` per plan artifact spec

### Commits Verified
- `6d83c71` feat(18-01): add Kitty keyboard protocol compatibility layer
- `c96312c` feat(18-01): disable Kitty keyboard protocol at TUI startup
