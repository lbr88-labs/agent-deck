// Regression tests for issue #1264: `session send` silently fails to submit
// when Claude Code's prompt is in vim NORMAL mode.
//
// Failure mechanism (confirmed from the issue + the send path): a bracketed
// paste lands in the input widget regardless of vim mode, but the trailing
// `Enter` in normal mode is a navigation keystroke, not submit — so the
// message is typed but never sent, and the send-verify retry loop's bare
// `SendEnter()` nudges all no-op for the same reason.
//
// The fix gates an Escape + `i` insert-mode guarantee at the keysender layer
// (Session.VimMode), so every SendEnter / SendKeysAndEnter issued against a
// vim-mode target is preceded by a guarantee-insert sequence. These tests
// record the exact key sequence emitted via the keySenderExec seam, so they
// run deterministically without standing up a real tmux server.
package tmux

import (
	"os/exec"
	"strings"
	"sync"
	"testing"
)

// recordKeySender swaps keySenderExec for a recorder that captures each tmux
// send-keys invocation's argv and returns a no-op command that exits 0, so the
// production .Run() succeeds without a real tmux server. Returns a pointer to
// the recorded calls slice (each entry is the full argv after the socket name)
// and registers cleanup to restore the original seam.
func recordKeySender(t *testing.T) *[]string {
	t.Helper()
	original := keySenderExec
	var mu sync.Mutex
	var calls []string
	keySenderExec = func(socketName string, args ...string) *exec.Cmd {
		mu.Lock()
		calls = append(calls, strings.Join(args, " "))
		mu.Unlock()
		// `true` is a tiny binary that exits 0 — keeps .Run() happy without
		// touching tmux. It exists on every POSIX host the suite runs on.
		return exec.Command("true")
	}
	t.Cleanup(func() { keySenderExec = original })
	return &calls
}

// sentKey extracts the trailing key token from a recorded `send-keys ... <key>`
// argv string.
func sentKey(call string) string {
	fields := strings.Fields(call)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

// TestSendEnter_VimMode_PrependsInsertGuard is the core #1264 regression: a
// bare SendEnter against a vim-mode target must first force insert mode
// (Escape then `i`) so the Enter actually submits instead of being consumed as
// a normal-mode motion. This is the exact path the retry loop's nudges take.
//
// Pre-fix: SendEnter emits only `Enter` → test FAILS (no Escape/`i` guard).
func TestSendEnter_VimMode_PrependsInsertGuard(t *testing.T) {
	calls := recordKeySender(t)

	s := &Session{Name: "vim-enter", VimMode: true}
	if err := s.SendEnter(); err != nil {
		t.Fatalf("SendEnter returned error: %v", err)
	}

	keys := []string{}
	for _, c := range *calls {
		keys = append(keys, sentKey(c))
	}
	want := []string{"Escape", "i", "Enter"}
	if len(keys) != len(want) {
		t.Fatalf("vim-mode SendEnter emitted %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("vim-mode SendEnter emitted %v, want %v", keys, want)
		}
	}
}

// TestSendEnter_NonVim_Unchanged guards against regressing the default path:
// when VimMode is false the bare Enter must be emitted alone, with no Escape/i
// prefix that would corrupt a normal (non-vim) Claude composer or other tools.
func TestSendEnter_NonVim_Unchanged(t *testing.T) {
	calls := recordKeySender(t)

	s := &Session{Name: "plain-enter"} // VimMode defaults to false
	if err := s.SendEnter(); err != nil {
		t.Fatalf("SendEnter returned error: %v", err)
	}

	if len(*calls) != 1 || sentKey((*calls)[0]) != "Enter" {
		t.Fatalf("non-vim SendEnter must emit exactly [Enter], got %v", *calls)
	}
}

// TestSendKeysAndEnter_VimMode_InsertThenPasteThenEnter verifies the full send
// path: insert guard fires BEFORE the paste (so the message body isn't eaten
// as vim commands), and the trailing Enter is NOT re-escaped (re-escaping
// would drop back to normal mode and swallow the submit).
//
// Pre-fix: emits only `<paste> Enter` with no leading Escape/`i` → test FAILS.
func TestSendKeysAndEnter_VimMode_InsertThenPasteThenEnter(t *testing.T) {
	calls := recordKeySender(t)

	s := &Session{Name: "vim-send", VimMode: true}
	if err := s.SendKeysAndEnter("hello world"); err != nil {
		t.Fatalf("SendKeysAndEnter returned error: %v", err)
	}

	c := *calls
	// Expected: Escape, i (insert guard), literal paste, Enter — 4 calls.
	if len(c) != 4 {
		t.Fatalf("vim-mode SendKeysAndEnter expected 4 tmux calls (Escape, i, paste, Enter)\n got %d: %v", len(c), c)
	}

	if sentKey(c[0]) != "Escape" {
		t.Fatalf("first key must be Escape, got %q (%v)", sentKey(c[0]), c)
	}
	if sentKey(c[1]) != "i" {
		t.Fatalf("second key must be i, got %q (%v)", sentKey(c[1]), c)
	}
	// Remaining calls must contain the literal paste and a trailing Enter, with
	// NO additional Escape after the paste (would un-insert before submit).
	rest := c[2:]
	var sawPaste, sawEnter bool
	for _, call := range rest {
		if strings.Contains(call, "-l") && strings.Contains(call, "hello world") {
			sawPaste = true
		}
		if sentKey(call) == "Enter" {
			sawEnter = true
		}
		if strings.Contains(call, " Escape") || call == "Escape" {
			t.Fatalf("no Escape allowed after the paste (would swallow submit): %v", c)
		}
	}
	if !sawPaste {
		t.Fatalf("literal paste of message body not emitted: %v", c)
	}
	if !sawEnter {
		t.Fatalf("trailing Enter not emitted: %v", c)
	}
}

// TestSendKeysAndEnter_NonVim_NoInsertGuard guards the default path: a non-vim
// send must NOT inject Escape/`i` (which would be typed as literal characters
// into a non-vim composer or break other tools like codex/gemini/opencode).
func TestSendKeysAndEnter_NonVim_NoInsertGuard(t *testing.T) {
	calls := recordKeySender(t)

	s := &Session{Name: "plain-send"} // VimMode defaults to false
	if err := s.SendKeysAndEnter("hello"); err != nil {
		t.Fatalf("SendKeysAndEnter returned error: %v", err)
	}

	for _, call := range *calls {
		if sentKey(call) == "Escape" || sentKey(call) == "i" {
			t.Fatalf("non-vim SendKeysAndEnter must not emit Escape/i guard: %v", *calls)
		}
	}
}
