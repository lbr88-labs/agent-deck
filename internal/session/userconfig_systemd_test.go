package session

import (
	"os/exec"
	"sync/atomic"
	"testing"
)

// TestIsSystemdUserScopeAvailable_MatchesHostCapability asserts that the
// production detection helper agrees byte-for-byte with the requireSystemdRun
// test gate in session_persistence_test.go: true iff exec.LookPath("systemd-run")
// succeeds AND `systemd-run --user --version` exits zero.
func TestIsSystemdUserScopeAvailable_MatchesHostCapability(t *testing.T) {
	expected := false
	if _, err := exec.LookPath("systemd-run"); err == nil {
		if err := exec.Command("systemd-run", "--user", "--version").Run(); err == nil {
			expected = true
		}
	}
	resetSystemdDetectionCacheForTest()
	got := isSystemdUserScopeAvailable()
	if got != expected {
		t.Fatalf("isSystemdUserScopeAvailable() = %v, want %v (host probe agreement)", got, expected)
	}
}

// TestIsSystemdUserScopeAvailable_CachesResult asserts that sync.Once holds:
// two consecutive calls return the same value and the underlying probe body
// only runs once.
func TestIsSystemdUserScopeAvailable_CachesResult(t *testing.T) {
	resetSystemdDetectionCacheForTest()
	atomic.StoreInt64(&systemdUserScopeProbeCount, 0)
	a := isSystemdUserScopeAvailable()
	b := isSystemdUserScopeAvailable()
	if a != b {
		t.Fatalf("cache mismatch: %v vs %v", a, b)
	}
	if n := atomic.LoadInt64(&systemdUserScopeProbeCount); n != 1 {
		t.Fatalf("probe ran %d times, want exactly 1 (sync.Once cache must hold)", n)
	}
}

// TestIsSystemdUserScopeAvailable_ResetForTestRePrubes asserts that after
// resetSystemdDetectionCacheForTest() the next call re-probes the host
// (probe counter increments a second time).
func TestIsSystemdUserScopeAvailable_ResetForTestRePrubes(t *testing.T) {
	resetSystemdDetectionCacheForTest()
	atomic.StoreInt64(&systemdUserScopeProbeCount, 0)
	_ = isSystemdUserScopeAvailable()
	resetSystemdDetectionCacheForTest()
	_ = isSystemdUserScopeAvailable()
	if n := atomic.LoadInt64(&systemdUserScopeProbeCount); n != 2 {
		t.Fatalf("probe ran %d times after reset, want exactly 2", n)
	}
}
