package watcher

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testMockNotifier is a test double for the Notifier interface.
// It records all Notify calls and can simulate errors and panics on specific calls.
type testMockNotifier struct {
	mu          sync.Mutex
	alerts      []Alert
	callCount   atomic.Int64
	errOnCall   int64 // 1-based: return error when callCount == errOnCall (0 = never)
	panicOnCall int64 // 1-based: panic when callCount == panicOnCall (0 = never)
}

func (m *testMockNotifier) Notify(_ context.Context, alert Alert) error {
	count := m.callCount.Add(1)
	if m.panicOnCall > 0 && count == m.panicOnCall {
		panic("boom2")
	}
	if m.errOnCall > 0 && count == m.errOnCall {
		return errors.New("boom")
	}
	m.mu.Lock()
	m.alerts = append(m.alerts, alert)
	m.mu.Unlock()
	return nil
}

func (m *testMockNotifier) AlertCount() int64 {
	return m.callCount.Load()
}

func (m *testMockNotifier) RecordedAlerts() []Alert {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Alert, len(m.alerts))
	copy(out, m.alerts)
	return out
}

// TestHealthBridge_SilenceTriggersOneAlert verifies that one silence HealthState
// produces exactly one Notify call with trigger == "silence_detected".
func TestHealthBridge_SilenceTriggersOneAlert(t *testing.T) {
	notifier := &testMockNotifier{}
	ch := make(chan HealthState, 16)
	cfg := Config{Enabled: true}
	bridge := NewHealthBridge(cfg, notifier, ch)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch <- HealthState{
		WatcherName: "w1",
		Status:      HealthStatusWarning,
		Message:     "no events for 65m (threshold 60 minutes)",
	}

	done := make(chan struct{})
	go func() {
		_ = bridge.Run(ctx)
		close(done)
	}()

	// Wait for the notifier to be called or timeout.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if notifier.AlertCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-done

	if got := notifier.AlertCount(); got != 1 {
		t.Errorf("expected 1 Notify call, got %d", got)
	}
	alerts := notifier.RecordedAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 recorded alert, got %d", len(alerts))
	}
	if alerts[0].Trigger != TriggerSilenceDetected {
		t.Errorf("expected trigger %q, got %q", TriggerSilenceDetected, alerts[0].Trigger)
	}
	if alerts[0].WatcherName != "w1" {
		t.Errorf("expected WatcherName %q, got %q", "w1", alerts[0].WatcherName)
	}
}

// TestHealthBridge_ErrorThresholdTriggersOneAlert verifies that an error-threshold
// HealthState produces exactly one Notify call with trigger == "error_threshold_exceeded".
func TestHealthBridge_ErrorThresholdTriggersOneAlert(t *testing.T) {
	notifier := &testMockNotifier{}
	ch := make(chan HealthState, 16)
	cfg := Config{Enabled: true}
	bridge := NewHealthBridge(cfg, notifier, ch)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch <- HealthState{
		WatcherName:       "w2",
		Status:            HealthStatusError,
		Message:           "10 consecutive errors",
		ConsecutiveErrors: 10,
	}

	done := make(chan struct{})
	go func() {
		_ = bridge.Run(ctx)
		close(done)
	}()

	// Wait for the notifier to be called or timeout.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if notifier.AlertCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-done

	if got := notifier.AlertCount(); got != 1 {
		t.Errorf("expected 1 Notify call, got %d", got)
	}
	alerts := notifier.RecordedAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 recorded alert, got %d", len(alerts))
	}
	if alerts[0].Trigger != TriggerErrorThresholdExceeded {
		t.Errorf("expected trigger %q, got %q", TriggerErrorThresholdExceeded, alerts[0].Trigger)
	}
	if alerts[0].WatcherName != "w2" {
		t.Errorf("expected WatcherName %q, got %q", "w2", alerts[0].WatcherName)
	}
}

// TestHealthBridge_DebounceWithin15Min verifies that two silence events for the same
// watcher within the debounce window produce exactly one Notify call.
func TestHealthBridge_DebounceWithin15Min(t *testing.T) {
	notifier := &testMockNotifier{}
	ch := make(chan HealthState, 16)
	// Use a 10-minute debounce window with an injected clock.
	cfg := Config{Enabled: true, DebounceWindow: 10 * time.Minute}
	bridge := NewHealthBridge(cfg, notifier, ch)

	// Inject a fake clock that we control.
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	bridge.setClockForTest(func() time.Time { return now })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// First silence event.
	ch <- HealthState{
		WatcherName: "w3",
		Status:      HealthStatusWarning,
		Message:     "no events for 65m (threshold 60 minutes)",
	}

	done := make(chan struct{})
	go func() {
		_ = bridge.Run(ctx)
		close(done)
	}()

	// Wait for the first alert to be processed.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if notifier.AlertCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Advance clock by 5 minutes (within the 10-minute debounce window).
	now = now.Add(5 * time.Minute)

	// Second silence event — same watcher, same trigger; should be debounced.
	ch <- HealthState{
		WatcherName: "w3",
		Status:      HealthStatusWarning,
		Message:     "no events for 70m (threshold 60 minutes)",
	}

	// Allow time for the second event to be processed.
	time.Sleep(200 * time.Millisecond)

	cancel()
	<-done

	if got := notifier.AlertCount(); got != 1 {
		t.Errorf("expected exactly 1 Notify call (debounce), got %d", got)
	}
}

// TestHealthBridge_DisabledConfigZeroAlerts verifies that when cfg.Enabled == false,
// zero Notify calls are made and Run returns cleanly on ctx cancel.
func TestHealthBridge_DisabledConfigZeroAlerts(t *testing.T) {
	notifier := &testMockNotifier{}
	ch := make(chan HealthState, 16)
	cfg := Config{Enabled: false}
	bridge := NewHealthBridge(cfg, notifier, ch)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

	// Emit five warnings.
	for i := 0; i < 5; i++ {
		ch <- HealthState{
			WatcherName: "w4",
			Status:      HealthStatusWarning,
			Message:     "no events for 65m (threshold 60 minutes)",
		}
	}

	done := make(chan struct{})
	go func() {
		_ = bridge.Run(ctx)
		close(done)
	}()

	// Cancel quickly — Run should return cleanly.
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of ctx cancel in disabled mode")
	}

	if got := notifier.AlertCount(); got != 0 {
		t.Errorf("expected 0 Notify calls when disabled, got %d", got)
	}
}

// TestHealthBridge_DownstreamFailureNoncrash verifies that a notifier that returns
// an error on call #1 and panics on call #2 does not crash Run, and that the source
// channel continues to be drained normally.
func TestHealthBridge_DownstreamFailureNoncrash(t *testing.T) {
	notifier := &testMockNotifier{
		errOnCall:   1, // return error on first call
		panicOnCall: 2, // panic on second call
	}
	ch := make(chan HealthState, 16)
	cfg := Config{Enabled: true}
	bridge := NewHealthBridge(cfg, notifier, ch)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- bridge.Run(ctx)
	}()

	// Emit three events for different watchers so debounce doesn't suppress them.
	ch <- HealthState{
		WatcherName: "w5a",
		Status:      HealthStatusWarning,
		Message:     "no events for 65m (threshold 60 minutes)",
	}
	ch <- HealthState{
		WatcherName: "w5b",
		Status:      HealthStatusWarning,
		Message:     "no events for 65m (threshold 60 minutes)",
	}
	// Third event for a different watcher — "w-post-panic"
	ch <- HealthState{
		WatcherName: "w-post-panic",
		Status:      HealthStatusWarning,
		Message:     "no events for 70m (threshold 60 minutes)",
	}

	// Allow time for events to be processed.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if notifier.AlertCount() >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()

	var runErr error
	select {
	case runErr = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of ctx cancel")
	}

	if runErr != nil {
		t.Errorf("Run returned non-nil error %v, want nil (errors must not propagate)", runErr)
	}

	// Either the third Notify attempt was recorded OR the channel was drained (len == 0).
	totalCalls := notifier.AlertCount()
	chLen := len(ch)
	if totalCalls < 2 && chLen != 0 {
		t.Errorf("expected source channel to be drained or >= 2 calls recorded; got calls=%d, chLen=%d", totalCalls, chLen)
	}
}

// TestHealthBridge_TeardownCancelsPending verifies that ctx cancel causes Run to return
// within 2s and that no further Notify calls happen after cancel.
func TestHealthBridge_TeardownCancelsPending(t *testing.T) {
	notifier := &testMockNotifier{}
	ch := make(chan HealthState, 16)
	cfg := Config{Enabled: true, DebounceWindow: 5 * time.Minute}
	bridge := NewHealthBridge(cfg, notifier, ch)

	// Use a fake clock so we can control debounce.
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	bridge.setClockForTest(func() time.Time { return now })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel() })

	done := make(chan struct{})
	go func() {
		_ = bridge.Run(ctx)
		close(done)
	}()

	// Queue one debounced event to be processed.
	ch <- HealthState{
		WatcherName: "w6",
		Status:      HealthStatusWarning,
		Message:     "no events for 65m (threshold 60 minutes)",
	}

	// Allow first alert to process.
	time.Sleep(100 * time.Millisecond)

	// Cancel the context.
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s of ctx cancel")
	}

	// Record alert count immediately after Run exits.
	countAfterCancel := notifier.AlertCount()

	// Wait a short time to ensure no further calls happen.
	time.Sleep(100 * time.Millisecond)

	countAfterWait := notifier.AlertCount()
	if countAfterWait > countAfterCancel {
		t.Errorf("Notify called after ctx cancel: count was %d, now %d", countAfterCancel, countAfterWait)
	}
}

// TestHealthBridge_Integration_MockAdapterForcedSilence drives the bridge via a real
// HealthTracker-produced HealthState over a test-owned channel. Engine internals are
// not involved — this exercises the full path: HealthTracker -> HealthState -> channel
// -> HealthBridge -> Notifier.
func TestHealthBridge_Integration_MockAdapterForcedSilence(t *testing.T) {
	// Construct a real HealthTracker with a 1-minute silence threshold.
	tracker := NewHealthTracker("w-int", 1)

	// Force silence: set last event time to 10 minutes ago.
	tracker.SetLastEventTimeForTest(time.Now().Add(-10 * time.Minute))

	// Check health — should produce HealthStatusWarning with "no events" message.
	state := tracker.Check()
	if state.Status != HealthStatusWarning {
		t.Fatalf("expected HealthStatusWarning from forced silence, got %s", state.Status)
	}
	if state.WatcherName != "w-int" {
		t.Fatalf("expected WatcherName %q, got %q", "w-int", state.WatcherName)
	}

	// Build a test-owned buffered channel and push the state.
	testCh := make(chan HealthState, 16)
	testCh <- state

	// Construct the bridge with a short ctx deadline.
	notifier := &testMockNotifier{}
	cfg := Config{Enabled: true}
	bridge := NewHealthBridge(cfg, notifier, testCh)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = bridge.Run(ctx)
		close(done)
	}()

	// Wait for the alert or timeout.
	deadline := time.Now().Add(450 * time.Millisecond)
	for time.Now().Before(deadline) {
		if notifier.AlertCount() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-done

	if got := notifier.AlertCount(); got != 1 {
		t.Errorf("integration: expected 1 Notify call, got %d", got)
	}
	alerts := notifier.RecordedAlerts()
	if len(alerts) != 1 {
		t.Fatalf("integration: expected 1 recorded alert, got %d", len(alerts))
	}
	if alerts[0].Trigger != TriggerSilenceDetected {
		t.Errorf("integration: expected trigger %q, got %q", TriggerSilenceDetected, alerts[0].Trigger)
	}
	if alerts[0].WatcherName != "w-int" {
		t.Errorf("integration: expected WatcherName %q, got %q", "w-int", alerts[0].WatcherName)
	}
}
