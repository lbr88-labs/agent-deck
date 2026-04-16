package ui

import (
	"bytes"
	"os"
	"testing"
)

// TestCSIuReader_PreservesFdForRawMode is a MECHANISM guard test.
//
// Why this test exists: On 2026-04-08, PR #538 removed
// `tea.WithInput(ui.NewCSIuReader(os.Stdin))` because the wrapper stripped
// the *os.File interface from stdin, preventing Bubble Tea from setting raw
// terminal mode. Arrow keys appeared as literal "^[[A" text (issue #544,
// #539, #607).
//
// On 2026-04-12, commit 817a616 needed the wrapper back to translate CSI u
// sequences for underscore input (issue 02-01). To AVOID re-introducing the
// raw-mode bug, that commit introduced csiuFileReader which embeds *os.File.
// Embedding promotes Fd() / Read / Write / Close to satisfy
// github.com/charmbracelet/x/term.File, which Bubble Tea checks via type
// assertion in tty_unix.go before calling term.MakeRaw.
//
// If a future refactor:
//   - Removes the csiuFileReader struct
//   - Changes NewCSIuReader to return plain *csiuReader even for *os.File input
//   - Removes the *os.File embed from csiuFileReader
//
// then raw terminal mode will silently stop being set, and #544/#607 will
// regress. Every existing integration test (PR #541) may still pass in
// headless-tmux environments but real interactive terminals will break.
//
// This test asserts the MECHANISM (type-assertion chain) directly, not a
// symptom (arrow-key echo), so it catches the reversion regardless of test
// environment.
func TestCSIuReader_PreservesFdForRawMode(t *testing.T) {
	// Use a real *os.File so Fd() returns a meaningful descriptor.
	// os.Stdin works even in test environments because its Fd() is defined
	// (it's the inherited stdin from the go test runner).
	f := os.Stdin
	wantFd := f.Fd()

	got := NewCSIuReader(f)

	// The returned reader MUST satisfy the Fd()-having interface so Bubble
	// Tea can type-assert and call term.MakeRaw on the descriptor.
	fdProvider, ok := got.(interface{ Fd() uintptr })
	if !ok {
		t.Fatalf("NewCSIuReader(*os.File) returned a reader that does NOT implement Fd() uintptr.\n"+
			"Bubble Tea's tty_unix.go checks `p.input.(term.File)` and calls term.MakeRaw on Fd().\n"+
			"Without Fd(), raw mode is never set → arrow keys echo as '^[[A' → TUI unusable.\n"+
			"Concrete type returned: %T. See keyboard_compat.go and issue #544.", got)
	}

	if fdProvider.Fd() != wantFd {
		t.Errorf("Fd() returned %d, want %d (the original os.Stdin fd). "+
			"The returned reader must preserve the original file descriptor so "+
			"Bubble Tea sets raw mode on the correct terminal.", fdProvider.Fd(), wantFd)
	}
}

// TestCSIuReader_NonFileFallsBackToPlainReader asserts the complementary case:
// when NewCSIuReader is given an io.Reader that is NOT a *os.File (e.g., a
// bytes.Buffer in tests), it returns a plain *csiuReader. This keeps the
// translation behavior intact for test harnesses while not synthesizing a
// fake Fd() that would mislead Bubble Tea.
func TestCSIuReader_NonFileFallsBackToPlainReader(t *testing.T) {
	buf := bytes.NewBuffer([]byte{})

	got := NewCSIuReader(buf)

	// Should NOT satisfy Fd() — we don't want to lie about file descriptors.
	if _, ok := got.(interface{ Fd() uintptr }); ok {
		t.Errorf("NewCSIuReader(bytes.Buffer) unexpectedly satisfies Fd() uintptr. "+
			"Only *os.File input should return a reader with Fd() — otherwise "+
			"Bubble Tea would try to call term.MakeRaw on a fake fd. "+
			"Concrete type returned: %T", got)
	}
}

// TestCSIuReader_UnderscorePreserved guards the fix from commit 817a616
// (issue 02-01): Shift+hyphen sends CSI u sequence \x1b[95;2u in terminals
// with extended-keys (Ghostty, Alacritty, tmux). The reader must translate
// this to a literal '_' byte so TUI text inputs receive the underscore.
//
// If this regresses (e.g., by removing csiuReader entirely), dialog text
// inputs will silently drop underscores. This test, together with
// TestCSIuReader_PreservesFdForRawMode, guards the full fix space: the
// wrapper MUST exist AND MUST preserve Fd() for *os.File input.
func TestCSIuReader_UnderscorePreserved(t *testing.T) {
	// CSI u encoding for Shift+hyphen → codepoint 95 ('_') with shift modifier.
	// Sequence: ESC [ 95 ; 2 u
	input := bytes.NewBuffer([]byte("\x1b[95;2u"))
	r := NewCSIuReader(input)

	out := make([]byte, 16)
	n, err := r.Read(out)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("Read failed: %v", err)
	}

	got := string(out[:n])
	if got != "_" {
		t.Errorf("expected CSI u underscore sequence to translate to '_', got %q (%d bytes). "+
			"If this test fails, TUI dialog text inputs will silently drop underscores "+
			"on Ghostty/Alacritty/tmux-extended-keys terminals. See issue 02-01 / commit 817a616.",
			got, n)
	}
}
