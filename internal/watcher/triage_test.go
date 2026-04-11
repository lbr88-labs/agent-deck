package watcher

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeClock is a controllable clock for test determinism.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	// In tests we use real time.After since we don't rely on After for rate-limiter tests.
	// Tests control the window via Now() + Advance().
	go func() {
		<-time.After(d)
		ch <- c.Now()
	}()
	return ch
}

func (c *fakeClock) NewTicker(d time.Duration) *time.Ticker {
	// Delegates to real ticker. Tests control timing via Now()/Advance() for rate limits,
	// and for the reaper tests call scanOnce() directly.
	return time.NewTicker(d)
}

// fakeSpawner records Spawn calls for test assertions.
type fakeSpawner struct {
	mu           sync.Mutex
	calls        []TriageRequest
	resultWriter func(req TriageRequest) // optional: simulates session writing result.json
	err          error                   // optional: return this error from Spawn
}

func (f *fakeSpawner) Spawn(_ context.Context, req TriageRequest) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.mu.Lock()
	f.calls = append(f.calls, req)
	n := len(f.calls)
	f.mu.Unlock()
	if f.resultWriter != nil {
		go f.resultWriter(req)
	}
	return fmt.Sprintf("fake-session-%d", n), nil
}

func (f *fakeSpawner) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// TestRateLimiter_PrunesStaleOnEveryCheck seeds 5 stale spawns, then calls
// tryAcquire — pruning must clear the window and return true, leaving 1 entry.
func TestRateLimiter_PrunesStaleOnEveryCheck(t *testing.T) {
	fc := newFakeClock()
	rl := &rateLimiter{}

	staleTime := fc.Now().Add(-70 * time.Minute) // 70 min ago → outside the 60-min window
	for i := 0; i < 5; i++ {
		rl.spawns = append(rl.spawns, staleTime)
	}

	// tryAcquire must prune all 5 stale entries and allow a new spawn.
	got := rl.tryAcquire(fc.Now())
	if !got {
		t.Fatal("expected tryAcquire to return true after pruning stale entries, got false")
	}
	// After the successful acquire, there should be exactly 1 entry (the one just added).
	if len(rl.spawns) != 1 {
		t.Fatalf("expected 1 entry after pruning and acquire, got %d", len(rl.spawns))
	}
}

// TestTriageLoop_RateLimitSixthQueued_Unit unit-tests just the rateLimiter:
// 6 calls within 1ms must yield 5 true and 1 false.
func TestTriageLoop_RateLimitSixthQueued_Unit(t *testing.T) {
	fc := newFakeClock()
	rl := &rateLimiter{}

	for i := 0; i < 5; i++ {
		got := rl.tryAcquire(fc.Now())
		if !got {
			t.Fatalf("call %d: expected true, got false", i+1)
		}
	}
	// 6th call must be denied.
	got := rl.tryAcquire(fc.Now())
	if got {
		t.Fatal("6th call: expected false, got true")
	}
}

// TestTriageSpawner_BinaryNotFound verifies that AgentDeckLaunchSpawner returns
// a non-nil error when the binary path does not exist.
func TestTriageSpawner_BinaryNotFound(t *testing.T) {
	dir := t.TempDir()
	req := TriageRequest{
		Event:      Event{Source: "mock", Sender: "test@example.com", Subject: "test", Timestamp: time.Now()},
		WatcherID:  "w1",
		Profile:    "test",
		TriageDir:  dir,
		ResultPath: dir + "/result.json",
		SpawnedAt:  time.Now(),
	}

	spawner := AgentDeckLaunchSpawner{BinaryPath: "/definitely/not/a/real/path/agent-deck"}
	_, err := spawner.Spawn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent binary, got nil")
	}
	// The error should mention something about the path being invalid.
	t.Logf("got expected error: %v", err)
}
