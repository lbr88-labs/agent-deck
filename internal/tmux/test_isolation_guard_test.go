package tmux

import (
	"os"
	"strings"
	"testing"
)

// TestAssertTestTmuxIsolation_NoPanic_WhenMarkerSetAndTmuxUnset verifies the
// common happy path: a well-behaved test calls testutil.IsolateTmuxSocket,
// which sets the marker and clears TMUX. The guard must not panic.
func TestAssertTestTmuxIsolation_NoPanic_WhenMarkerSetAndTmuxUnset(t *testing.T) {
	origMarker, hadMarker := os.LookupEnv(testIsolationMarkerEnv)
	origTmux, hadTmux := os.LookupEnv("TMUX")
	defer func() {
		restoreForGuard(testIsolationMarkerEnv, origMarker, hadMarker)
		restoreForGuard("TMUX", origTmux, hadTmux)
	}()

	os.Setenv(testIsolationMarkerEnv, "1")
	os.Unsetenv("TMUX")

	// Must not panic.
	assertTestTmuxIsolation()
}

// TestAssertTestTmuxIsolation_Panics_WhenNoMarkerAndTmuxLeaks is the
// regression test for the 2026-04-17 three-cascade bug: a test binary
// whose TestMain forgot testutil.IsolateTmuxSocket and therefore inherits
// the user's real TMUX must NOT be allowed to spawn tmux.
func TestAssertTestTmuxIsolation_Panics_WhenNoMarkerAndTmuxLeaks(t *testing.T) {
	origMarker, hadMarker := os.LookupEnv(testIsolationMarkerEnv)
	origTmux, hadTmux := os.LookupEnv("TMUX")
	defer func() {
		restoreForGuard(testIsolationMarkerEnv, origMarker, hadMarker)
		restoreForGuard("TMUX", origTmux, hadTmux)
	}()

	os.Unsetenv(testIsolationMarkerEnv)
	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when guard runs under test binary with no marker and TMUX pointing at user socket; none occurred — the 2026-04-17 bug could recur")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, "tmux isolation guard") {
			t.Errorf("panic message does not mention the guard: %q", msg)
		}
		if !strings.Contains(msg, "/tmp/tmux-1000/default") {
			t.Errorf("panic message should include the leaking TMUX path: %q", msg)
		}
	}()

	assertTestTmuxIsolation()
}

// TestAssertTestTmuxIsolation_Panics_WhenMarkerSetButTmuxReintroduced
// covers the belt-and-suspenders branch: someone re-set TMUX after
// IsolateTmuxSocket cleared it (rare, but possible with careless
// t.Setenv before test body). The guard should still catch this.
func TestAssertTestTmuxIsolation_Panics_WhenMarkerSetButTmuxReintroduced(t *testing.T) {
	origMarker, hadMarker := os.LookupEnv(testIsolationMarkerEnv)
	origTmux, hadTmux := os.LookupEnv("TMUX")
	defer func() {
		restoreForGuard(testIsolationMarkerEnv, origMarker, hadMarker)
		restoreForGuard("TMUX", origTmux, hadTmux)
	}()

	os.Setenv(testIsolationMarkerEnv, "1")
	os.Setenv("TMUX", "/tmp/tmux-1000/default,77777,0")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic: marker is set yet TMUX points at user socket — someone re-set TMUX after isolation")
		}
	}()

	assertTestTmuxIsolation()
}

// TestAssertTestTmuxIsolation_NoPanic_WhenNoMarkerAndTmuxUnset covers the
// "orphan code path" case: TestMain didn't call IsolateTmuxSocket, but
// TMUX happens to be unset too (e.g. CI environment without a parent
// tmux). No leak risk, so no panic — but we do emit a warn log.
func TestAssertTestTmuxIsolation_NoPanic_WhenNoMarkerAndTmuxUnset(t *testing.T) {
	origMarker, hadMarker := os.LookupEnv(testIsolationMarkerEnv)
	origTmux, hadTmux := os.LookupEnv("TMUX")
	defer func() {
		restoreForGuard(testIsolationMarkerEnv, origMarker, hadMarker)
		restoreForGuard("TMUX", origTmux, hadTmux)
	}()

	os.Unsetenv(testIsolationMarkerEnv)
	os.Unsetenv("TMUX")

	// Must not panic.
	assertTestTmuxIsolation()
}

func TestLooksLikeUserDefaultSocket(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"default socket uid 1000", "/tmp/tmux-1000/default,12345,0", true},
		{"default socket uid 0", "/tmp/tmux-0/default,1,0", true},
		{"default socket no comma", "/tmp/tmux-1000/default", true},
		{"isolated dir", "/tmp/agent-deck-test-tmux-abc/tmux-1000/default,5555,0", false},
		{"fallback dir", "/tmp/agent-deck-test-tmux-fallback-42/tmux-1000/default", false},
		{"empty", "", false},
		{"random path", "/home/user/.something", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeUserDefaultSocket(tc.input); got != tc.want {
				t.Errorf("looksLikeUserDefaultSocket(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestLooksLikeGoTestBinary(t *testing.T) {
	// We are currently running inside a go-test binary, so the real
	// os.Args[0] should satisfy looksLikeGoTestBinary.
	if !looksLikeGoTestBinary() {
		t.Errorf("looksLikeGoTestBinary returned false inside a go test run (os.Args[0]=%q)", os.Args[0])
	}
}

func restoreForGuard(key, orig string, had bool) {
	if had {
		_ = os.Setenv(key, orig)
	} else {
		_ = os.Unsetenv(key)
	}
}
