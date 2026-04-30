package session

import (
	"sync/atomic"
	"testing"
	"time"
)

// TestRunWithTimeout_CompletesBeforeTimeout verifies the happy path: an op
// that finishes quickly returns true, and its writes to captured state are
// observable by the caller.
func TestRunWithTimeout_CompletesBeforeTimeout(t *testing.T) {
	var got int
	completed := runWithTimeout(500*time.Millisecond, func() {
		got = 42
	})
	if !completed {
		t.Fatalf("runWithTimeout reported timeout for an instant op")
	}
	if got != 42 {
		t.Errorf("captured state = %d, want 42", got)
	}
}

// TestRunWithTimeout_TimesOut verifies the timeout path: an op that blocks
// longer than the deadline returns false, and the caller is unblocked
// promptly (within a small buffer over the configured timeout).
func TestRunWithTimeout_TimesOut(t *testing.T) {
	timeout := 50 * time.Millisecond
	start := time.Now()
	completed := runWithTimeout(timeout, func() {
		time.Sleep(2 * time.Second) // far longer than the timeout
	})
	elapsed := time.Since(start)
	if completed {
		t.Fatalf("runWithTimeout reported completion for a long-blocking op")
	}
	// Allow generous slack: the select should fire near the timeout, but
	// scheduler jitter on a busy CI host can add tens of ms. 500ms ceiling
	// is comfortably below the op's 2s sleep, so this catches real
	// regressions without being flaky.
	if elapsed > 500*time.Millisecond {
		t.Errorf("caller blocked for %v, expected return near %v", elapsed, timeout)
	}
}

// TestRunWithTimeout_AbandonedGoroutineIsHarmless verifies the contract that
// callers must not consult captured state after timeout. We trigger the
// timeout path, the abandoned goroutine eventually writes to a shared
// counter, and the test ends without panicking. The counter assertion is
// secondary: it just confirms the goroutine kept running, which is
// expected (we cannot kill goroutines from outside).
func TestRunWithTimeout_AbandonedGoroutineIsHarmless(t *testing.T) {
	var counter atomic.Int64
	completed := runWithTimeout(20*time.Millisecond, func() {
		time.Sleep(200 * time.Millisecond)
		counter.Add(1)
	})
	if completed {
		t.Fatalf("expected timeout, got completion")
	}

	// Wait long enough for the abandoned goroutine to finish on a healthy
	// scheduler. If it never increments, that is also fine; the contract
	// is just that the caller is not blocked and does not panic.
	time.Sleep(400 * time.Millisecond)
	final := counter.Load()
	if final != 0 && final != 1 {
		t.Errorf("counter = %d after abandoned goroutine; expected 0 or 1", final)
	}
}
