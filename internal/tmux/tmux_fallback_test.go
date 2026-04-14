// Tests for the systemd-run -> direct-tmux fallback path in Start().
//
// Plan 02-04 of the session-persistence milestone. These tests pin the
// contract that a non-zero exit from the systemd-run --user --scope wrap
// retries once with the direct tmux launcher using the bare args, emits
// a structured tmux_systemd_run_fallback warning, and never blocks session
// creation unless both paths fail. If both paths fail the returned error
// must wrap both diagnostics so operators can see why isolation was
// attempted AND how the retry broke.
//
// Safety: the integration arms use the execCommand swappable seam to
// inject failure at the launcher level. Real tmux servers that do get
// created are scoped under a unique agentdeck-test-fallback-<hex> name
// with a targeted `tmux kill-server -t <name>` cleanup — never a bare
// kill-server (see CLAUDE.md 2025-12-10 incident notes).
package tmux

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// randomServerSuffix returns 8 hex chars (4 random bytes) for use in
// unique test server / unit names. Mirrors the helper in
// internal/session/session_persistence_test.go:230-237.
func randomServerSuffix(t *testing.T) string {
	t.Helper()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("randomServerSuffix: rand.Read: %v", err)
	}
	return hex.EncodeToString(b[:])
}

// captureStatusLog swaps the package-level statusLog with a JSON-handler
// writing to a buffer for the test duration; restores via t.Cleanup.
func captureStatusLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	original := statusLog
	statusLog = slog.New(slog.NewJSONHandler(buf, nil))
	t.Cleanup(func() { statusLog = original })
	return buf
}

// failOnLauncher returns an exec wrapper that returns a guaranteed-fail
// command (`false`, exit 1, no side effects) for any invocation whose
// argv[0] equals failBinary, and otherwise delegates to exec.Command.
// Used to simulate "systemd-run is present but invocation fails"
// without mutating host PATH or systemd state.
func failOnLauncher(failBinary string) func(name string, arg ...string) *exec.Cmd {
	return func(name string, arg ...string) *exec.Cmd {
		if name == failBinary {
			return exec.Command("false")
		}
		return exec.Command(name, arg...)
	}
}

func TestStripSystemdRunPrefix_RecoversTmuxArgs(t *testing.T) {
	in := []string{
		"--user", "--scope", "--quiet", "--collect", "--unit",
		"agentdeck-tmux-foo", "tmux",
		"new-session", "-d", "-s", "name",
	}
	want := []string{"new-session", "-d", "-s", "name"}
	got := stripSystemdRunPrefix(in)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStripSystemdRunPrefix_PassesThroughUnexpectedShape(t *testing.T) {
	// Too-short arg list — shape does not match, pass through unchanged.
	in := []string{"--user", "--scope"}
	got := stripSystemdRunPrefix(in)
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("got %v, want unchanged %v", got, in)
	}
	// Length OK but args[6] is not "tmux" — pass through unchanged.
	in2 := []string{
		"--user", "--scope", "--quiet", "--collect", "--unit",
		"name", "not-tmux", "x",
	}
	got2 := stripSystemdRunPrefix(in2)
	if !reflect.DeepEqual(got2, in2) {
		t.Fatalf("got %v, want unchanged %v (args[6] != %q)", got2, in2, "tmux")
	}
}

// TestStartCommandSpec_FallsBackToDirect drives (*Session).Start through
// the real entry point with the execCommand seam overridden so the
// systemd-run path exits 1. The direct retry delegates to the real
// exec.Command (so a real tmux server is created) and the test asserts:
//   - Start() returns nil (fallback recovered)
//   - statusLog recorded a tmux_systemd_run_fallback event
//   - cleanup kills the specific server we created (targeted -t filter)
func TestStartCommandSpec_FallsBackToDirect(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skipf("no tmux binary available: %v", err)
	}

	original := execCommand
	execCommand = failOnLauncher("systemd-run")
	t.Cleanup(func() { execCommand = original })

	buf := captureStatusLog(t)

	// Build a properly-initialized Session via the public constructor,
	// then flip LaunchInUserScope to force the systemd-run path that
	// failOnLauncher("systemd-run") will reject.
	displayName := "test-fallback-" + randomServerSuffix(t)
	s := NewSession(displayName, "/tmp")
	s.LaunchInUserScope = true
	// Targeted cleanup: only our session. -t filter is mandatory
	// per CLAUDE.md tmux safety mandate (2025-12-10 incident).
	t.Cleanup(func() { _ = exec.Command("tmux", "kill-session", "-t", s.Name).Run() })

	if err := s.Start(""); err != nil {
		t.Fatalf("expected fallback to recover, got error: %v\nlog: %s", err, buf.String())
	}

	if !strings.Contains(buf.String(), "tmux_systemd_run_fallback") {
		t.Fatalf("expected tmux_systemd_run_fallback warning in log, got: %s", buf.String())
	}
}

// TestStartCommandSpec_BothFailWrapsError poisons both systemd-run AND
// tmux so the fallback also fails. Asserts the returned error carries
// both "systemd-run path:" and "direct retry:" substrings so operators
// can grep logs and diagnose.
func TestStartCommandSpec_BothFailWrapsError(t *testing.T) {
	original := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "systemd-run" || name == "tmux" {
			return exec.Command("false")
		}
		return exec.Command(name, arg...)
	}
	t.Cleanup(func() { execCommand = original })

	displayName := "test-bothfail-" + randomServerSuffix(t)
	s := NewSession(displayName, "/tmp")
	s.LaunchInUserScope = true
	// Defensive cleanup (no real server should be created, but cheap insurance).
	t.Cleanup(func() { _ = exec.Command("tmux", "kill-session", "-t", s.Name).Run() })

	err := s.Start("")
	if err == nil {
		t.Fatalf("expected error when both paths fail, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "systemd-run path:") {
		t.Fatalf("error %q must contain 'systemd-run path:' substring", msg)
	}
	if !strings.Contains(msg, "direct retry:") {
		t.Fatalf("error %q must contain 'direct retry:' substring", msg)
	}
}
