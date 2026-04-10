package watcher

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// signGitHubPayload computes the HMAC-SHA256 signature for integration tests.
func signGitHubPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// TestEngine_Integration_AllAdapters registers WebhookAdapter, NtfyAdapter (with mock server),
// and GitHubAdapter with a real engine backed by a temp statedb. It sends one event through
// each adapter and verifies all 3 events are persisted and appear on EventCh.
func TestEngine_Integration_AllAdapters(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionResetter"),
		goleak.IgnoreAnyFunction("modernc.org"),
		goleak.IgnoreAnyFunction("poll.runtime_pollWait"),
	)

	const githubSecret = "integration-test-secret"

	// Routing table: one conductor per adapter source
	clients := map[string]ClientEntry{
		"sender@webhook.com": {
			Conductor: "webhook-conductor",
			Group:     "webhook/inbox",
			Name:      "Webhook Client",
		},
		"*@github.com": {
			Conductor: "github-conductor",
			Group:     "github/inbox",
			Name:      "GitHub Wildcard",
		},
	}
	// ntfy sender format is "ntfy:topic@host", which won't match the above,
	// so we add a wildcard for the mock server host later or use exact match.
	// We'll add the ntfy sender as exact match after we know the mock server host.

	// Create a mock ntfy server that serves one message event
	ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		flusher, _ := w.(http.Flusher)

		msg := ntfyMessage{
			ID:      "integ-msg-1",
			Time:    time.Now().Unix(),
			Event:   "message",
			Topic:   "testtopic",
			Message: "ntfy integration event",
		}
		data, _ := json.Marshal(msg)
		fmt.Fprintf(w, "%s\n", data)
		if flusher != nil {
			flusher.Flush()
		}
		// Hold connection until client disconnects
		<-r.Context().Done()
	}))
	defer ntfyServer.Close()

	// Now we know the ntfy sender format and can add routing
	ntfyHost := hostFromURL(ntfyServer.URL)
	ntfySender := fmt.Sprintf("ntfy:testtopic@%s", ntfyHost)
	clients[ntfySender] = ClientEntry{
		Conductor: "ntfy-conductor",
		Group:     "ntfy/inbox",
		Name:      "Ntfy Client",
	}

	engine, db := newTestEngine(t, clients)

	// Create watcher rows in DB
	saveTestWatcher(t, db, "w-webhook", "webhook-watcher", "webhook")
	saveTestWatcher(t, db, "w-ntfy", "ntfy-watcher", "ntfy")
	saveTestWatcher(t, db, "w-github", "github-watcher", "github")

	// Create and register adapters
	webhookAdapter := &WebhookAdapter{}
	ntfyAdapter := &NtfyAdapter{}
	githubAdapter := &GitHubAdapter{}

	engine.RegisterAdapter("w-webhook", webhookAdapter, AdapterConfig{
		Type:     "webhook",
		Name:     "webhook-watcher",
		Settings: map[string]string{"port": "0"},
	}, 60)

	engine.RegisterAdapter("w-ntfy", ntfyAdapter, AdapterConfig{
		Type:     "ntfy",
		Name:     "ntfy-watcher",
		Settings: map[string]string{"topic": "testtopic", "server": ntfyServer.URL},
	}, 60)

	engine.RegisterAdapter("w-github", githubAdapter, AdapterConfig{
		Type:     "github",
		Name:     "github-watcher",
		Settings: map[string]string{"port": "0", "secret": githubSecret},
	}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("engine.Start: %v", err)
	}

	// Wait for webhook and github servers to be ready
	if !waitForServer(t, webhookAdapter, 2*time.Second) {
		t.Fatal("webhook server did not start in time")
	}
	if !waitForGitHubServer(t, githubAdapter, 2*time.Second) {
		t.Fatal("github server did not start in time")
	}

	// Give ntfy adapter time to connect and receive its message
	time.Sleep(500 * time.Millisecond)

	// Send webhook event
	webhookBody := `{"message": "webhook integration event"}`
	webhookReq, _ := http.NewRequest("POST", "http://"+webhookAdapter.addr+"/webhook", bytes.NewReader([]byte(webhookBody)))
	webhookReq.Header.Set("X-Webhook-Sender", "sender@webhook.com")
	webhookReq.Header.Set("X-Webhook-Subject", "webhook test")
	webhookResp, err := http.DefaultClient.Do(webhookReq)
	if err != nil {
		t.Fatalf("webhook POST: %v", err)
	}
	webhookResp.Body.Close()
	if webhookResp.StatusCode != http.StatusAccepted {
		t.Errorf("webhook: expected 202, got %d", webhookResp.StatusCode)
	}

	// Send GitHub event
	ghPayload := map[string]interface{}{
		"action": "opened",
		"issue": map[string]interface{}{
			"number": 99,
			"title":  "Integration test issue",
			"body":   "github integration event",
		},
		"sender":     map[string]interface{}{"login": "testbot"},
		"repository": map[string]interface{}{"full_name": "test/integration"},
	}
	ghBody, _ := json.Marshal(ghPayload)
	ghSig := signGitHubPayload(githubSecret, ghBody)

	ghReq, _ := http.NewRequest("POST", "http://"+githubAdapter.Addr()+"/github", bytes.NewReader(ghBody))
	ghReq.Header.Set("X-Hub-Signature-256", ghSig)
	ghReq.Header.Set("X-GitHub-Event", "issues")
	ghReq.Header.Set("X-GitHub-Delivery", "integration-delivery-1")
	ghResp, err := http.DefaultClient.Do(ghReq)
	if err != nil {
		t.Fatalf("github POST: %v", err)
	}
	ghResp.Body.Close()
	if ghResp.StatusCode != http.StatusAccepted {
		t.Errorf("github: expected 202, got %d", ghResp.StatusCode)
	}

	// Wait for events to flow through the engine pipeline
	time.Sleep(500 * time.Millisecond)

	engine.Stop()

	// Verify events persisted in DB
	webhookCount := countWatcherEvents(t, db, "w-webhook")
	if webhookCount != 1 {
		t.Errorf("expected 1 webhook event in DB, got %d", webhookCount)
	}

	ntfyCount := countWatcherEvents(t, db, "w-ntfy")
	if ntfyCount != 1 {
		t.Errorf("expected 1 ntfy event in DB, got %d", ntfyCount)
	}

	githubCount := countWatcherEvents(t, db, "w-github")
	if githubCount != 1 {
		t.Errorf("expected 1 github event in DB, got %d", githubCount)
	}

	// Verify routing
	webhookRoute := queryWatcherEventRoutedTo(t, db, "w-webhook")
	if webhookRoute != "webhook-conductor" {
		t.Errorf("webhook: expected routed_to=webhook-conductor, got %q", webhookRoute)
	}

	ntfyRoute := queryWatcherEventRoutedTo(t, db, "w-ntfy")
	if ntfyRoute != "ntfy-conductor" {
		t.Errorf("ntfy: expected routed_to=ntfy-conductor, got %q", ntfyRoute)
	}

	githubRoute := queryWatcherEventRoutedTo(t, db, "w-github")
	if githubRoute != "github-conductor" {
		t.Errorf("github: expected routed_to=github-conductor, got %q", githubRoute)
	}

	// Verify all 3 events appeared on EventCh
	events := drainEvents(engine.EventCh(), 100*time.Millisecond)
	if len(events) != 3 {
		t.Errorf("expected 3 events on EventCh, got %d", len(events))
		for i, e := range events {
			t.Logf("  event[%d]: source=%s sender=%s subject=%s", i, e.Source, e.Sender, e.Subject)
		}
	}

	// Verify event sources
	sources := make(map[string]bool)
	for _, e := range events {
		sources[e.Source] = true
	}
	for _, expected := range []string{"webhook", "ntfy", "github"} {
		if !sources[expected] {
			t.Errorf("missing event from source %q", expected)
		}
	}
}

// TestEngine_Integration_DedupAcrossAdapters verifies that events from different
// sources are NOT deduped (since DedupKey includes Source field).
func TestEngine_Integration_DedupAcrossAdapters(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionOpener"),
		goleak.IgnoreTopFunction("database/sql.(*DB).connectionResetter"),
		goleak.IgnoreAnyFunction("modernc.org"),
		goleak.IgnoreAnyFunction("poll.runtime_pollWait"),
	)

	// Use two mock adapters with identical events but different sources
	engine, db := newTestEngine(t, nil)
	saveTestWatcher(t, db, "w1", "watcher-1", "mock-a")
	saveTestWatcher(t, db, "w2", "watcher-2", "mock-b")

	now := time.Now()

	// Same sender, subject, timestamp but different Source
	adapter1 := &MockAdapter{
		events: []Event{
			{Source: "mock-a", Sender: "same@test.com", Subject: "same subject", Timestamp: now},
		},
		listenDelay: 10 * time.Millisecond,
	}
	adapter2 := &MockAdapter{
		events: []Event{
			{Source: "mock-b", Sender: "same@test.com", Subject: "same subject", Timestamp: now},
		},
		listenDelay: 10 * time.Millisecond,
	}

	engine.RegisterAdapter("w1", adapter1, AdapterConfig{Type: "mock-a", Name: "watcher-1"}, 60)
	engine.RegisterAdapter("w2", adapter2, AdapterConfig{Type: "mock-b", Name: "watcher-2"}, 60)

	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(300 * time.Millisecond)
	engine.Stop()

	// Both should be persisted because DedupKey includes Source
	count1 := countWatcherEvents(t, db, "w1")
	count2 := countWatcherEvents(t, db, "w2")

	if count1 != 1 {
		t.Errorf("expected 1 event from watcher-1, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("expected 1 event from watcher-2, got %d", count2)
	}

	events := drainEvents(engine.EventCh(), 100*time.Millisecond)
	if len(events) != 2 {
		t.Errorf("expected 2 events on EventCh (different sources don't dedup), got %d", len(events))
	}
}
