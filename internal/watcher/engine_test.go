package watcher_test

import (
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/asheshgoplani/agent-deck/internal/statedb"
	"github.com/asheshgoplani/agent-deck/internal/watcher"
)

// newTestDB creates a temporary StateDB for use in tests.
func newTestDB(t *testing.T) *statedb.StateDB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := statedb.Open(dbPath)
	if err != nil {
		t.Fatalf("statedb.Open: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// saveTestWatcher inserts a watcher row so that engine tests can reference it.
func saveTestWatcher(t *testing.T, db *statedb.StateDB, id, name, typ string) {
	t.Helper()
	now := time.Now()
	err := db.SaveWatcher(&statedb.WatcherRow{
		ID:        id,
		Name:      name,
		Type:      typ,
		Status:    "running",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("SaveWatcher: %v", err)
	}
}

// countWatcherEvents queries the watcher_events table directly using the
// underlying sql.DB exposed by StateDB.DB().
func countWatcherEvents(t *testing.T, db *statedb.StateDB, watcherID string) int {
	t.Helper()
	var count int
	row := db.DB().QueryRow(
		`SELECT COUNT(*) FROM watcher_events WHERE watcher_id = ?`,
		watcherID,
	)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("countWatcherEvents: %v", err)
	}
	return count
}

// routedToForWatcher returns the routed_to value for the first event of a watcher.
func routedToForWatcher(t *testing.T, db *statedb.StateDB, watcherID string) string {
	t.Helper()
	var routedTo string
	row := db.DB().QueryRow(
		`SELECT routed_to FROM watcher_events WHERE watcher_id = ? LIMIT 1`,
		watcherID,
	)
	if err := row.Scan(&routedTo); err != nil {
		t.Fatalf("routedToForWatcher: %v", err)
	}
	return routedTo
}

// newTestEngine constructs an Engine with a test DB and the given client map.
func newTestEngine(t *testing.T, clients map[string]watcher.ClientEntry) (*watcher.Engine, *statedb.StateDB) {
	t.Helper()
	db := newTestDB(t)
	router := watcher.NewRouter(clients)
	cfg := watcher.EngineConfig{
		DB:                  db,
		Router:              router,
		MaxEventsPerWatcher: 500,
		// HealthCheckInterval=0 disables health loop to avoid timing flakiness in tests
		HealthCheckInterval: 0,
	}
	engine := watcher.NewEngine(cfg)
	return engine, db
}

// makeEvent constructs an Event with a unique timestamp to avoid unintentional dedup.
func makeEvent(source, sender, subject string) watcher.Event {
	return watcher.Event{
		Source:    source,
		Sender:    sender,
		Subject:   subject,
		Timestamp: time.Now(),
	}
}

// TestWatcherEngine_Dedup verifies that two events with identical DedupKey()
// result in only one row in the database and only one event on EventCh().
func TestWatcherEngine_Dedup(t *testing.T) {
	engine, db := newTestEngine(t, nil)
	saveTestWatcher(t, db, "w1", "test-watcher", "mock")

	// Create a fixed timestamp so both events produce the same DedupKey
	fixedTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	evt := watcher.Event{
		Source:    "mock",
		Sender:    "test@example.com",
		Subject:   "same subject",
		Timestamp: fixedTime,
	}

	adapter := &MockAdapter{
		events:      []watcher.Event{evt, evt}, // identical events
		listenDelay: 10 * time.Millisecond,
	}

	engine.RegisterAdapter("w1", adapter, watcher.AdapterConfig{
		Type: "mock",
		Name: "test-watcher",
	}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Collect events from routedEventCh
	received := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		timer := time.NewTimer(300 * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-engine.EventCh():
				received++
			case <-timer.C:
				return
			}
		}
	}()

	<-done
	engine.Stop()

	// Only 1 event should reach EventCh (dedup prevents the second)
	if received != 1 {
		t.Errorf("expected 1 event on EventCh, got %d", received)
	}

	// Only 1 row should exist in DB
	count := countWatcherEvents(t, db, "w1")
	if count != 1 {
		t.Errorf("expected 1 row in watcher_events, got %d", count)
	}
}

// TestWatcherEngine_Stop_NoLeaks verifies that Engine.Stop() cancels all goroutines
// and produces no goroutine leaks (enforced by goleak).
func TestWatcherEngine_Stop_NoLeaks(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionResetter"),
		goleak.IgnoreAnyFunction("modernc.org"),
	)

	engine, db := newTestEngine(t, nil)

	saveTestWatcher(t, db, "w1", "adapter-1", "mock")
	saveTestWatcher(t, db, "w2", "adapter-2", "mock")
	saveTestWatcher(t, db, "w3", "adapter-3", "mock")

	for i, id := range []string{"w1", "w2", "w3"} {
		_ = i
		adapter := &MockAdapter{
			events:      []watcher.Event{makeEvent("mock", "sender@example.com", "subject")},
			listenDelay: 5 * time.Millisecond,
		}
		engine.RegisterAdapter(id, adapter, watcher.AdapterConfig{
			Type: "mock",
			Name: id,
		}, 60)
	}

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Allow adapters to process events
	time.Sleep(200 * time.Millisecond)

	engine.Stop()
	// goleak.VerifyNone runs via defer after Stop() completes
}

// TestWatcherEngine_KnownSenderRouting verifies that an event from a sender
// in the clients map is persisted with the correct routed_to value.
func TestWatcherEngine_KnownSenderRouting(t *testing.T) {
	clients := map[string]watcher.ClientEntry{
		"user@company.com": {
			Conductor: "client-a",
			Group:     "client-a/inbox",
			Name:      "Client A",
		},
	}

	engine, db := newTestEngine(t, clients)
	saveTestWatcher(t, db, "w1", "routing-test", "mock")

	evt := watcher.Event{
		Source:    "mock",
		Sender:    "user@company.com",
		Subject:   "test routing",
		Timestamp: time.Now(),
	}
	adapter := &MockAdapter{
		events:      []watcher.Event{evt},
		listenDelay: 5 * time.Millisecond,
	}

	engine.RegisterAdapter("w1", adapter, watcher.AdapterConfig{
		Type: "mock",
		Name: "routing-test",
	}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for event to be processed
	select {
	case <-engine.EventCh():
		// Event received
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for event on EventCh")
	}

	engine.Stop()

	routedTo := routedToForWatcher(t, db, "w1")
	if routedTo != "client-a" {
		t.Errorf("expected routed_to=%q, got %q", "client-a", routedTo)
	}
}

// TestWatcherEngine_UnknownSenderRouting verifies that an event from a sender
// not in the clients map is persisted with routed_to="" (unrouted).
func TestWatcherEngine_UnknownSenderRouting(t *testing.T) {
	clients := map[string]watcher.ClientEntry{
		"known@company.com": {
			Conductor: "known-conductor",
			Group:     "known/inbox",
			Name:      "Known Client",
		},
	}

	engine, db := newTestEngine(t, clients)
	saveTestWatcher(t, db, "w1", "unknown-routing-test", "mock")

	evt := watcher.Event{
		Source:    "mock",
		Sender:    "unknown@other.com",
		Subject:   "test unrouted",
		Timestamp: time.Now(),
	}
	adapter := &MockAdapter{
		events:      []watcher.Event{evt},
		listenDelay: 5 * time.Millisecond,
	}

	engine.RegisterAdapter("w1", adapter, watcher.AdapterConfig{
		Type: "mock",
		Name: "unknown-routing-test",
	}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for event to be processed via EventCh
	select {
	case <-engine.EventCh():
		// Event received
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for event on EventCh")
	}

	engine.Stop()

	routedTo := routedToForWatcher(t, db, "w1")
	if routedTo != "" {
		t.Errorf("expected routed_to=\"\" for unknown sender, got %q", routedTo)
	}
}

// TestWatcherEngine_StopCancelsAdapters verifies that Engine.Stop() calls
// Teardown on all registered adapters.
func TestWatcherEngine_StopCancelsAdapters(t *testing.T) {
	engine, db := newTestEngine(t, nil)

	saveTestWatcher(t, db, "w1", "adapter-1", "mock")
	saveTestWatcher(t, db, "w2", "adapter-2", "mock")

	adapter1 := &MockAdapter{}
	adapter2 := &MockAdapter{}

	engine.RegisterAdapter("w1", adapter1, watcher.AdapterConfig{
		Type: "mock",
		Name: "adapter-1",
	}, 60)
	engine.RegisterAdapter("w2", adapter2, watcher.AdapterConfig{
		Type: "mock",
		Name: "adapter-2",
	}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	engine.Stop()

	if !adapter1.TeardownCalled() {
		t.Error("expected Teardown to be called on adapter1")
	}
	if !adapter2.TeardownCalled() {
		t.Error("expected Teardown to be called on adapter2")
	}
}
