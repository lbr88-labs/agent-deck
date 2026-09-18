package session

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLastSeenAliveToolDataRoundTrip(t *testing.T) {
	at := time.Now().Add(-5 * time.Minute).Truncate(time.Second)

	td := WriteLastSeenAliveAtToToolData(nil, at)
	if got := ReadLastSeenAliveAtFromToolData(td); !got.Equal(at) {
		t.Errorf("round trip = %v, want %v", got, at)
	}

	// Merging must preserve unrelated keys (the extras-zone contract).
	base := json.RawMessage(`{"claude_session_id":"abc","last_started_at":1700000000}`)
	td = WriteLastSeenAliveAtToToolData(base, at)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(td, &m); err != nil {
		t.Fatalf("merged tool_data invalid: %v", err)
	}
	if string(m["claude_session_id"]) != `"abc"` {
		t.Errorf("unrelated key lost: %s", td)
	}
	if got := ReadLastSeenAliveAtFromToolData(td); !got.Equal(at) {
		t.Errorf("merge round trip = %v, want %v", got, at)
	}
}

// TestLastSeenAliveZeroRemovesKey pins the clear semantics: a zero time must
// REMOVE the key so a deliberately stopped session is indistinguishable from
// a legacy row — never readable as "alive at epoch".
func TestLastSeenAliveZeroRemovesKey(t *testing.T) {
	at := time.Now().Add(-time.Minute)
	td := WriteLastSeenAliveAtToToolData(nil, at)
	td = WriteLastSeenAliveAtToToolData(td, time.Time{})
	if got := ReadLastSeenAliveAtFromToolData(td); !got.IsZero() {
		t.Errorf("cleared stamp read back as %v, want zero", got)
	}
}

func TestReadLastSeenAlive_MissingAndMalformed(t *testing.T) {
	for name, td := range map[string]json.RawMessage{
		"nil":       nil,
		"empty":     {},
		"no key":    json.RawMessage(`{"other":1}`),
		"zero":      json.RawMessage(`{"last_seen_alive_at":0}`),
		"malformed": json.RawMessage(`not json`),
	} {
		if got := ReadLastSeenAliveAtFromToolData(td); !got.IsZero() {
			t.Errorf("%s: got %v, want zero", name, got)
		}
	}
}

// TestMarkSeenAlive_Monotonic pins the fold: an older observation must never
// rewind the stamp (out-of-order observers: daemon vs TUI sweep).
func TestMarkSeenAlive_Monotonic(t *testing.T) {
	i := &Instance{}
	newer := time.Now().Add(-time.Minute)
	older := newer.Add(-time.Hour)

	i.MarkSeenAlive(newer)
	i.MarkSeenAlive(older)
	if got := i.LastSeenAliveAt(); !got.Equal(newer) {
		t.Errorf("older observation rewound stamp: got %v, want %v", got, newer)
	}
}
