package watcher

import (
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/asheshgoplani/agent-deck/internal/statedb"
)

// newTestDB creates a temporary StateDB for engine tests.
func newTestDB(t *testing.T) *statedb.StateDB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := statedb.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// newTestEngine creates an Engine with a fresh DB and the given client routing rules.
// HealthCheckInterval is 0 to disable the health loop in tests.
// Uses a fakeSpawner to avoid spawning real agent-deck subprocesses (goroutine-leak-safe).
func newTestEngine(t *testing.T, clients map[string]ClientEntry) (*Engine, *statedb.StateDB) {
	t.Helper()
	db := newTestDB(t)
	router := NewRouter(clients)
	cfg := EngineConfig{
		DB:                  db,
		Router:              router,
		MaxEventsPerWatcher: 500,
		HealthCheckInterval: 0,
		// Use fakeSpawner so tests never exec real agent-deck subprocesses.
		// This prevents os/exec.watchCtx goroutine leaks across tests.
		TriageSpawner: &fakeSpawner{},
		TriageDir:     t.TempDir(),
		ClientsPath:   filepath.Join(t.TempDir(), "clients.json"),
	}
	engine := NewEngine(cfg)
	return engine, db
}

// saveTestWatcher inserts a watcher row into the database for testing.
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

// countWatcherEvents queries the watcher_events table and returns the row count for a given watcher.
func countWatcherEvents(t *testing.T, db *statedb.StateDB, watcherID string) int {
	t.Helper()
	var count int
	err := db.DB().QueryRow(
		`SELECT COUNT(*) FROM watcher_events WHERE watcher_id = ?`, watcherID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count watcher_events: %v", err)
	}
	return count
}

// queryWatcherEventRoutedTo returns the routed_to value for the first event matching the given watcher.
func queryWatcherEventRoutedTo(t *testing.T, db *statedb.StateDB, watcherID string) string {
	t.Helper()
	var routedTo string
	err := db.DB().QueryRow(
		`SELECT routed_to FROM watcher_events WHERE watcher_id = ? ORDER BY id LIMIT 1`, watcherID,
	).Scan(&routedTo)
	if err != nil {
		t.Fatalf("query routed_to: %v", err)
	}
	return routedTo
}

// drainEvents reads all available events from the channel within a timeout.
func drainEvents(ch <-chan Event, timeout time.Duration) []Event {
	var events []Event
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, evt)
		case <-deadline:
			return events
		}
	}
}

// TestWatcherEngine_Dedup verifies that two events with identical DedupKey
// result in only one persisted row and one routed event (D-23).
func TestWatcherEngine_Dedup(t *testing.T) {
	engine, db := newTestEngine(t, nil)
	saveTestWatcher(t, db, "w1", "test-watcher", "mock")

	now := time.Now()
	identicalEvent := Event{
		Source:    "mock",
		Sender:    "test@example.com",
		Subject:   "same subject",
		Timestamp: now,
	}

	adapter := &MockAdapter{
		events:      []Event{identicalEvent, identicalEvent},
		listenDelay: 10 * time.Millisecond,
	}

	engine.RegisterAdapter("w1", adapter, AdapterConfig{Type: "mock", Name: "test-watcher"}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for events to be processed by the writer loop.
	time.Sleep(200 * time.Millisecond)
	engine.Stop()

	// Verify: only 1 row in DB despite 2 identical events sent.
	count := countWatcherEvents(t, db, "w1")
	if count != 1 {
		t.Errorf("expected 1 event in DB (dedup), got %d", count)
	}

	// Verify: only 1 event on the routed channel.
	events := drainEvents(engine.EventCh(), 50*time.Millisecond)
	if len(events) != 1 {
		t.Errorf("expected 1 routed event, got %d", len(events))
	}
}

// TestWatcherEngine_Stop_NoLeaks verifies that starting an engine with 3 adapters
// (plus triage goroutines from Phase 18) and stopping it leaves no goroutine leaks.
// Uses a fakeSpawner to avoid real subprocess goroutine leaks (D-22, T-18-16).
func TestWatcherEngine_Stop_NoLeaks(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionResetter"),
		goleak.IgnoreAnyFunction("modernc.org"),
		goleak.IgnoreAnyFunction("poll.runtime_pollWait"),
		// Plan 17-01: adding the Google client pulls in go.opencensus.io, whose
		// stats worker is started from an init() and lives for the test binary.
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
	)

	// Use fakeSpawner + fakeClient so triage goroutines start and stop cleanly (Task 5).
	engine, db, _, _, _ := newTestEngineWithTriage(t, nil)

	for i := 0; i < 3; i++ {
		wID := "w" + string(rune('1'+i))
		name := "watcher-" + string(rune('1'+i))
		saveTestWatcher(t, db, wID, name, "mock")

		adapter := &MockAdapter{
			events: []Event{
				{Source: "mock", Sender: "sender@test.com", Subject: "event", Timestamp: time.Now()},
			},
			listenDelay: 5 * time.Millisecond,
		}
		engine.RegisterAdapter(wID, adapter, AdapterConfig{Type: "mock", Name: name}, 60)
	}

	// Add one unrouted event to exercise triage goroutines (Task 5).
	saveTestWatcher(t, db, "w4", "watcher-unrouted", "mock")
	engine.RegisterAdapter("w4", &MockAdapter{
		events: []Event{
			{Source: "mock", Sender: "unrouted@nomatch.com", Subject: "unrouted", Timestamp: time.Now()},
		},
		listenDelay: 5 * time.Millisecond,
	}, AdapterConfig{Type: "mock", Name: "watcher-unrouted"}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	engine.Stop()

	// goleak.VerifyNone runs via defer and will fail the test if any goroutines leaked.
}

// TestWatcherEngine_KnownSenderRouting verifies that an event from a sender
// in the clients map is saved with the correct routed_to conductor.
func TestWatcherEngine_KnownSenderRouting(t *testing.T) {
	clients := map[string]ClientEntry{
		"user@company.com": {
			Conductor: "client-a",
			Group:     "client-a/inbox",
			Name:      "Client A",
		},
	}

	engine, db := newTestEngine(t, clients)
	saveTestWatcher(t, db, "w1", "test-watcher", "mock")

	adapter := &MockAdapter{
		events: []Event{
			{Source: "mock", Sender: "user@company.com", Subject: "test", Timestamp: time.Now()},
		},
	}

	engine.RegisterAdapter("w1", adapter, AdapterConfig{Type: "mock", Name: "test-watcher"}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	engine.Stop()

	routedTo := queryWatcherEventRoutedTo(t, db, "w1")
	if routedTo != "client-a" {
		t.Errorf("expected routed_to=client-a, got %q", routedTo)
	}
}

// TestWatcherEngine_UnknownSenderRouting verifies that an event from an unknown
// sender is saved with an empty routed_to field.
func TestWatcherEngine_UnknownSenderRouting(t *testing.T) {
	clients := map[string]ClientEntry{
		"known@company.com": {
			Conductor: "client-b",
			Group:     "client-b/inbox",
			Name:      "Client B",
		},
	}

	engine, db := newTestEngine(t, clients)
	saveTestWatcher(t, db, "w1", "test-watcher", "mock")

	adapter := &MockAdapter{
		events: []Event{
			{Source: "mock", Sender: "unknown@other.com", Subject: "test", Timestamp: time.Now()},
		},
	}

	engine.RegisterAdapter("w1", adapter, AdapterConfig{Type: "mock", Name: "test-watcher"}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	engine.Stop()

	routedTo := queryWatcherEventRoutedTo(t, db, "w1")
	// After Phase 18: unrouted events are sent to triage, so routed_to = "triage".
	if routedTo != "triage" {
		t.Errorf("expected routed_to=triage for unknown sender (Phase 18), got %q", routedTo)
	}
}

// TestWatcherEngine_ThreadReplyRouting verifies that an event with ParentDedupKey
// whose parent has a session_id gets ThreadSessionID set on the routed event.
func TestWatcherEngine_ThreadReplyRouting(t *testing.T) {
	engine, db := newTestEngine(t, nil)
	saveTestWatcher(t, db, "w1", "test-watcher", "mock")

	// Pre-insert a parent event with a known session_id
	_, err := db.DB().Exec(
		`INSERT INTO watcher_events (watcher_id, dedup_key, sender, subject, routed_to, session_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"w1", "slack-C123-1712345678.000", "slack:C123", "parent msg", "", "sess-123", time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("insert parent event: %v", err)
	}

	// Thread reply: ParentDedupKey points to the parent
	replyEvent := Event{
		Source:         "slack",
		Sender:         "slack:C123",
		Subject:        "reply msg",
		Timestamp:      time.Now(),
		CustomDedupKey: "slack-C123-1712345679.000",
		ParentDedupKey: "slack-C123-1712345678.000",
	}

	adapter := &MockAdapter{events: []Event{replyEvent}}
	engine.RegisterAdapter("w1", adapter, AdapterConfig{Type: "mock", Name: "test-watcher"}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Drain the routed event channel
	events := drainEvents(engine.EventCh(), 2*time.Second)
	engine.Stop()

	if len(events) != 1 {
		t.Fatalf("expected 1 routed event, got %d", len(events))
	}
	if events[0].ThreadSessionID != "sess-123" {
		t.Errorf("expected ThreadSessionID=sess-123, got %q", events[0].ThreadSessionID)
	}
}

// TestWatcherEngine_ThreadReplyFallback verifies that an event with ParentDedupKey
// whose parent has an empty session_id falls back to normal routing (ThreadSessionID empty).
func TestWatcherEngine_ThreadReplyFallback(t *testing.T) {
	engine, db := newTestEngine(t, nil)
	saveTestWatcher(t, db, "w1", "test-watcher", "mock")

	// Pre-insert a parent event with empty session_id
	_, err := db.DB().Exec(
		`INSERT INTO watcher_events (watcher_id, dedup_key, sender, subject, routed_to, session_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"w1", "slack-C123-1712345680.000", "slack:C123", "parent msg", "", "", time.Now().Unix(),
	)
	if err != nil {
		t.Fatalf("insert parent event: %v", err)
	}

	replyEvent := Event{
		Source:         "slack",
		Sender:         "slack:C123",
		Subject:        "reply msg",
		Timestamp:      time.Now(),
		CustomDedupKey: "slack-C123-1712345681.000",
		ParentDedupKey: "slack-C123-1712345680.000",
	}

	adapter := &MockAdapter{events: []Event{replyEvent}}
	engine.RegisterAdapter("w1", adapter, AdapterConfig{Type: "mock", Name: "test-watcher"}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	events := drainEvents(engine.EventCh(), 2*time.Second)
	engine.Stop()

	if len(events) != 1 {
		t.Fatalf("expected 1 routed event, got %d", len(events))
	}
	if events[0].ThreadSessionID != "" {
		t.Errorf("expected empty ThreadSessionID for fallback, got %q", events[0].ThreadSessionID)
	}
}

// TestWatcherEngine_ThreadReplyNoParent verifies that an event with ParentDedupKey
// but no parent event in DB falls back to normal routing.
func TestWatcherEngine_ThreadReplyNoParent(t *testing.T) {
	engine, db := newTestEngine(t, nil)
	saveTestWatcher(t, db, "w1", "test-watcher", "mock")

	replyEvent := Event{
		Source:         "slack",
		Sender:         "slack:C123",
		Subject:        "orphan reply",
		Timestamp:      time.Now(),
		CustomDedupKey: "slack-C123-1712345682.000",
		ParentDedupKey: "slack-C123-nonexistent.000",
	}

	adapter := &MockAdapter{events: []Event{replyEvent}}
	engine.RegisterAdapter("w1", adapter, AdapterConfig{Type: "mock", Name: "test-watcher"}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	events := drainEvents(engine.EventCh(), 2*time.Second)
	engine.Stop()

	if len(events) != 1 {
		t.Fatalf("expected 1 routed event, got %d", len(events))
	}
	if events[0].ThreadSessionID != "" {
		t.Errorf("expected empty ThreadSessionID when parent not found, got %q", events[0].ThreadSessionID)
	}
}

// TestWatcherEngine_StopCancelsAdapters verifies that Stop() calls Teardown()
// on all registered adapters.
func TestWatcherEngine_StopCancelsAdapters(t *testing.T) {
	engine, db := newTestEngine(t, nil)
	saveTestWatcher(t, db, "w1", "watcher-1", "mock")
	saveTestWatcher(t, db, "w2", "watcher-2", "mock")

	adapter1 := &MockAdapter{} // No events, just blocks on ctx
	adapter2 := &MockAdapter{} // No events, just blocks on ctx

	engine.RegisterAdapter("w1", adapter1, AdapterConfig{Type: "mock", Name: "watcher-1"}, 60)
	engine.RegisterAdapter("w2", adapter2, AdapterConfig{Type: "mock", Name: "watcher-2"}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Give adapters time to start.
	time.Sleep(50 * time.Millisecond)
	engine.Stop()

	if !adapter1.teardownCalled {
		t.Error("adapter1.Teardown() was not called")
	}
	if !adapter2.teardownCalled {
		t.Error("adapter2.Teardown() was not called")
	}
}
