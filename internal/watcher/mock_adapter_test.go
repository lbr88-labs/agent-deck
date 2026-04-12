package watcher

import (
	"context"
	"time"
)

// MockAdapter implements WatcherAdapter for testing.
// It emits a configurable list of events with optional delays,
// then blocks until the context is cancelled.
type MockAdapter struct {
	// events is the list of events to emit when Listen is called.
	events []Event

	// setupErr is an optional error returned by Setup.
	setupErr error

	// listenDelay is the delay between emitting each event.
	listenDelay time.Duration

	// healthCheckErr is an optional error returned by HealthCheck.
	healthCheckErr error

	// setupCalled is set to true when Setup is called.
	setupCalled bool

	// teardownCalled is set to true when Teardown is called.
	teardownCalled bool
}

// Setup records that it was called and returns the configured error (if any).
func (m *MockAdapter) Setup(_ context.Context, _ AdapterConfig) error {
	m.setupCalled = true
	return m.setupErr
}

// Listen sends all configured events to the channel with optional delays,
// then blocks until the context is cancelled.
func (m *MockAdapter) Listen(ctx context.Context, events chan<- Event) error {
	for _, evt := range m.events {
		if m.listenDelay > 0 {
			select {
			case <-time.After(m.listenDelay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		select {
		case events <- evt:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Block until context is cancelled (simulates a long-running adapter).
	<-ctx.Done()
	return ctx.Err()
}

// Teardown records that it was called.
func (m *MockAdapter) Teardown() error {
	m.teardownCalled = true
	return nil
}

// HealthCheck returns the configured error (if any).
func (m *MockAdapter) HealthCheck() error {
	return m.healthCheckErr
}
