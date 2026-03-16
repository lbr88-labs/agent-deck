---
phase: 18-wayland-key-input
verified: 2026-03-16T00:00:00Z
status: human_needed
score: 5/5 must-haves verified
human_verification:
  - test: "Shift+M triggers Move to Group on a Wayland compositor (Ghostty or Foot)"
    expected: "The Move to Group dialog opens — same as pressing M on macOS"
    why_human: "Cannot automate a Wayland compositor environment from a macOS CI context"
  - test: "Shift+R triggers Restart on a Wayland compositor"
    expected: "The Restart confirmation or action fires — same as pressing R on macOS"
    why_human: "Requires live Wayland session; not programmatically verifiable"
  - test: "Typing uppercase letters in the session name field on Wayland produces uppercase characters"
    expected: "Typing Shift+A produces 'A' in the input field, not a dropped keystroke"
    why_human: "Text-input rendering under Wayland requires a real terminal running the TUI"
  - test: "macOS regression check — all uppercase shortcuts still work after the change"
    expected: "No behavior change on macOS: Shift+M still opens Move to Group, etc."
    why_human: "Requires manual TUI exercise on macOS with the new binary"
---

# Phase 18: Wayland Key Input Verification Report

**Phase Goal:** Users running agent-deck on Wayland compositors can use all uppercase/shifted key shortcuts and type uppercase characters in text input fields
**Verified:** 2026-03-16
**Status:** human_needed (all automated checks passed; Wayland behavior requires live compositor)
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Pressing Shift+M/R/F on Wayland triggers the expected TUI action | ? HUMAN | DisableKittyKeyboard integration in place; confirmed by tests + wiring, but runtime Wayland behavior requires human |
| 2 | Typing uppercase characters in TUI text fields on Wayland produces uppercase | ? HUMAN | CSIuReader + ParseCSIu verified for uppercase translation; live terminal needed to confirm |
| 3 | CSI u sequences are correctly parsed into legacy tea.KeyMsg | VERIFIED | 14/14 unit tests pass (TestParseCSIu, TestCSIuReaderMixedInput, etc.) |
| 4 | DisableKittyKeyboard/RestoreKittyKeyboard write correct escape sequences | VERIFIED | TestDisableKittyKeyboard and TestRestoreKittyKeyboard pass; sequences `\x1b[>0u` and `\x1b[<u` confirmed |
| 5 | TUI startup sends disable sequence before tea.NewProgram | VERIFIED | main.go lines 474-475: `ui.DisableKittyKeyboard(os.Stdout)` + `defer ui.RestoreKittyKeyboard(os.Stdout)` immediately before `tea.NewProgram` |

**Score:** 3/3 automated truths verified. Truths 1 and 2 require human verification on a Wayland compositor.

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/ui/keyboard_compat.go` | CSI u parser + Disable/Restore functions | VERIFIED | 299 lines (min: 80). Contains `DisableKittyKeyboard`, `RestoreKittyKeyboard`, `ParseCSIu`, `NewCSIuReader`, `csiuReader.translate` — all substantive. |
| `internal/ui/keyboard_compat_test.go` | Unit tests for CSI u parsing and input filter | VERIFIED | 182 lines (min: 100). 14 test functions covering shift, ctrl, special codepoints, pass-through, mixed input. |
| `cmd/agent-deck/main.go` | TUI startup integration: disable before tea.NewProgram | VERIFIED | Lines 474-475 call `ui.DisableKittyKeyboard(os.Stdout)` and `defer ui.RestoreKittyKeyboard(os.Stdout)` with explanatory comment referencing Wayland/Bubble Tea limitation. |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/agent-deck/main.go` | `internal/ui/keyboard_compat.go` | `ui.DisableKittyKeyboard` call before `tea.NewProgram` | VERIFIED | Lines 474-475 confirmed; `defer ui.RestoreKittyKeyboard` on line 475 ensures restore on all exit paths. |
| `internal/ui/keyboard_compat.go` | `os.Stdout` (via caller) | `io.WriteString(w, "\x1b[>0u")` in `DisableKittyKeyboard` | VERIFIED | Line 33 writes the exact sequence. `w` parameter receives `os.Stdout` from main.go. |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| KEY-01 | 18-01-PLAN.md | Uppercase/shifted key shortcuts (M, R, F, etc.) trigger on Wayland compositors | SATISFIED (automated); needs human for runtime confirmation | `DisableKittyKeyboard` + `ParseCSIu` + `CSIuReader` all implemented and tested. `\x1b[>0u` suppress sequence wired into TUI startup path. |
| KEY-02 | 18-01-PLAN.md | Uppercase characters can be typed in TUI text input fields on Wayland | SATISFIED (automated); needs human for runtime confirmation | `ParseCSIu` correctly maps `\x1b[<letter>;2u` (shift modifier) to uppercase rune. `CSIuReader.translate` tested with `TestCSIuReaderMixedInput` producing `aRb` from `a\x1b[114;2ub`. |

REQUIREMENTS.md status for both: marked `[x]` Complete. No orphaned requirements.

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | — | — | No anti-patterns detected |

Notes:
- `return nil` at lines 60, 63, 67, 81, 86, 90 in `keyboard_compat.go` are correct early-exit guard clauses in `ParseCSIu` (not stubs).
- No TODO/FIXME/HACK/PLACEHOLDER comments in any modified file.
- No empty implementations. All functions have substantive bodies.

---

### Human Verification Required

#### 1. Wayland Shift+M shortcut

**Test:** On a Wayland compositor (Ghostty, Foot, or Alacritty), launch `agent-deck`, press Shift+M while a session is selected.
**Expected:** The Move to Group dialog opens — identical behavior to macOS.
**Why human:** A live Wayland session is required. The escape-sequence disable (`\x1b[>0u`) cannot be exercised programmatically from a macOS CI environment.

#### 2. Wayland Shift+R shortcut

**Test:** On a Wayland compositor, press Shift+R with a session selected.
**Expected:** The Restart action fires.
**Why human:** Same as above — requires live Wayland compositor.

#### 3. Uppercase text input on Wayland

**Test:** On a Wayland compositor, open the New Session dialog (press N), then type Shift+A, Shift+E, Shift+S, Shift+H in the session name field.
**Expected:** The name field shows "AESH" — uppercase characters appear rather than being silently dropped.
**Why human:** TUI text-field rendering under Wayland requires a real terminal.

#### 4. macOS regression check

**Test:** On macOS (current development machine), rebuild and run `agent-deck`. Verify Shift+M, Shift+R, Shift+F and other uppercase shortcuts still work.
**Expected:** No behavioral regression on macOS. The `\x1b[>0u` sequence is safely ignored by macOS terminals (Terminal.app, iTerm2, Alacritty-macOS) so behavior is unchanged.
**Why human:** Regression check on the non-Wayland host requires manual TUI exercise.

---

### Gaps Summary

No gaps. All automated must-haves are fully verified:

- `keyboard_compat.go` exists, is substantive (299 lines), and is imported/called by `main.go`.
- `keyboard_compat_test.go` exists, is substantive (182 lines), and all 14 tests pass.
- `cmd/agent-deck/main.go` calls `DisableKittyKeyboard` immediately before `tea.NewProgram` with `defer RestoreKittyKeyboard` for safe cleanup.
- Both KEY-01 and KEY-02 requirements are implemented and have unit-test coverage.
- Build succeeds with no errors or vet warnings.
- No regressions in `cmd/agent-deck` or `internal/ui` test suites.

The two success criteria from ROADMAP.md (Shift+M/R/F triggering correct actions, and uppercase text input working) cannot be confirmed without a Wayland compositor. All programmatically verifiable preconditions for those behaviors are satisfied.

One pre-existing issue noted in `deferred-items.md`: `internal/session/conductor_templates.go` has working-tree syntax errors from another session. This is out of scope for Phase 18 and does not affect the keyboard compatibility layer.

---

_Verified: 2026-03-16_
_Verifier: Claude (gsd-verifier)_
