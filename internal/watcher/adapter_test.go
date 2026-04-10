package watcher_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/watcher"
)

// TestEventDedupKey_SameInputsSameKey verifies that identical inputs produce the same DedupKey.
func TestEventDedupKey_SameInputsSameKey(t *testing.T) {
	ts := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	e1 := watcher.Event{
		Source:    "webhook",
		Sender:    "user@example.com",
		Subject:   "New issue",
		Timestamp: ts,
	}
	e2 := watcher.Event{
		Source:    "webhook",
		Sender:    "user@example.com",
		Subject:   "New issue",
		Timestamp: ts,
	}
	if e1.DedupKey() != e2.DedupKey() {
		t.Errorf("expected same DedupKey for identical events, got %q and %q", e1.DedupKey(), e2.DedupKey())
	}
}

// TestEventDedupKey_DifferentSenderDifferentKey verifies that different senders produce different keys.
func TestEventDedupKey_DifferentSenderDifferentKey(t *testing.T) {
	ts := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	e1 := watcher.Event{
		Source:    "webhook",
		Sender:    "user1@example.com",
		Subject:   "New issue",
		Timestamp: ts,
	}
	e2 := watcher.Event{
		Source:    "webhook",
		Sender:    "user2@example.com",
		Subject:   "New issue",
		Timestamp: ts,
	}
	if e1.DedupKey() == e2.DedupKey() {
		t.Errorf("expected different DedupKeys for different senders, but got same key %q", e1.DedupKey())
	}
}

// TestEventJSONRoundTrip verifies that Event marshals and unmarshals without data loss.
func TestEventJSONRoundTrip(t *testing.T) {
	ts := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	rawPayload := json.RawMessage(`{"key":"value","num":42}`)
	orig := watcher.Event{
		Source:     "ntfy",
		Sender:     "bot@example.com",
		Subject:    "Alert",
		Body:       "Something happened",
		Timestamp:  ts,
		RawPayload: rawPayload,
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("failed to marshal Event: %v", err)
	}
	var decoded watcher.Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal Event: %v", err)
	}
	if decoded.Source != orig.Source {
		t.Errorf("Source mismatch: got %q, want %q", decoded.Source, orig.Source)
	}
	if decoded.Sender != orig.Sender {
		t.Errorf("Sender mismatch: got %q, want %q", decoded.Sender, orig.Sender)
	}
	if decoded.Subject != orig.Subject {
		t.Errorf("Subject mismatch: got %q, want %q", decoded.Subject, orig.Subject)
	}
	if decoded.Body != orig.Body {
		t.Errorf("Body mismatch: got %q, want %q", decoded.Body, orig.Body)
	}
	if !decoded.Timestamp.Equal(orig.Timestamp) {
		t.Errorf("Timestamp mismatch: got %v, want %v", decoded.Timestamp, orig.Timestamp)
	}
	if string(decoded.RawPayload) != string(orig.RawPayload) {
		t.Errorf("RawPayload mismatch: got %q, want %q", string(decoded.RawPayload), string(orig.RawPayload))
	}
}
