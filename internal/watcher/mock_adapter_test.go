package watcher_test

import (
	"context"
	"sync"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/watcher"
)

// MockAdapter is a test-only WatcherAdapter that emits a pre-configured list
// of events and then blocks until the context is cancelled.
type MockAdapter struct {
	mu sync.Mutex

	// events is the list of events to emit on Listen
	events []watcher.Event

	// setupErr is returned from Setup if non-nil
	setupErr error

	// listenDelay is the pause between emitting each event
	listenDelay time.Duration

	// healthCheckErr is returned from HealthCheck if non-nil
	healthCheckErr error

	// setupCalled is set to true when Setup is called
	setupCalled bool

	// teardownCalled is set to true when Teardown is called
	teardownCalled bool
}

// Setup implements WatcherAdapter.
func (m *MockAdapter) Setup(_ context.Context, _ watcher.AdapterConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setupCalled = true
	return m.setupErr
}

// Listen implements WatcherAdapter.
// Emits all pre-configured events (with optional delay between each),
// then blocks until ctx is cancelled.
func (m *MockAdapter) Listen(ctx context.Context, events chan<- watcher.Event) error {
	m.mu.Lock()
	evts := make([]watcher.Event, len(m.events))
	copy(evts, m.events)
	delay := m.listenDelay
	m.mu.Unlock()

	for _, evt := range evts {
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil
			}
		}

		select {
		case events <- evt:
		case <-ctx.Done():
			return nil
		}
	}

	// Block until context is cancelled (simulates a long-running listener)
	<-ctx.Done()
	return nil
}

// Teardown implements WatcherAdapter.
func (m *MockAdapter) Teardown() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.teardownCalled = true
	return nil
}

// HealthCheck implements WatcherAdapter.
func (m *MockAdapter) HealthCheck() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.healthCheckErr
}

// SetupCalled returns true if Setup has been called.
func (m *MockAdapter) SetupCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.setupCalled
}

// TeardownCalled returns true if Teardown has been called.
func (m *MockAdapter) TeardownCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.teardownCalled
}
