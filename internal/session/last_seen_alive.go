// Durable "this pane was alive recently" evidence (tool_data.last_seen_alive_at).
//
// The status column cannot answer "was this session open when the machine
// died": it carries no timestamp, deliberate pane teardowns can classify as
// error, and old error rows accumulate forever — so a restore proposal built
// on status alone proposes August corpses alongside real crash victims.
//
// last_seen_alive_at is the missing signal, recorded the same way as
// #1846's last_activity_at (targeted tool_data extras write, no schema
// migration): every time an observer (TUI status sweep, CLI status refresh)
// confirms the tmux pane EXISTS, the instance stamps now. After a power
// loss the stamp simply stops advancing — so "stamp is recent at next
// launch, pane gone" is exactly "open when the machine went down".
//
// Operator stops clear the stamp (see clearSeenAlive, called from
// killInternal), so a session you deliberately stopped is never proposed,
// no matter what its status column says afterwards.
package session

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/statedb"
)

const toolDataLastSeenAliveAtKey = "last_seen_alive_at"

// lastSeenAlivePersistInterval throttles the targeted DB write: the TUI
// status sweep confirms liveness for every alive session each tick, and
// each confirmation would otherwise cost an UPDATE. One interval of
// precision is the most a hard kill can lose.
const lastSeenAlivePersistInterval = 60 * time.Second

// WriteLastSeenAliveAtToToolData merges last_seen_alive_at into the given
// tool_data JSON blob as a Unix-seconds integer. A zero time removes the
// key, keeping never-observed sessions indistinguishable from rows saved
// by an older binary.
func WriteLastSeenAliveAtToToolData(td json.RawMessage, t time.Time) json.RawMessage {
	if len(td) == 0 {
		td = json.RawMessage("{}")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(td, &m); err != nil {
		m = make(map[string]json.RawMessage)
	}
	if t.IsZero() {
		delete(m, toolDataLastSeenAliveAtKey)
	} else {
		b, err := json.Marshal(t.Unix())
		if err != nil {
			return td
		}
		m[toolDataLastSeenAliveAtKey] = b
	}
	out, err := json.Marshal(m)
	if err != nil {
		return td
	}
	return out
}

// ReadLastSeenAliveAtFromToolData extracts last_seen_alive_at from the
// blob. Returns the zero time for missing/malformed/legacy rows — callers
// must treat that as "never observed alive", never as "alive at epoch".
func ReadLastSeenAliveAtFromToolData(td json.RawMessage) time.Time {
	if len(td) == 0 {
		return time.Time{}
	}
	var m struct {
		At int64 `json:"last_seen_alive_at"`
	}
	if err := json.Unmarshal(td, &m); err != nil || m.At == 0 {
		return time.Time{}
	}
	return time.Unix(m.At, 0)
}

// LastSeenAliveAt returns the most recent time any observer in this or a
// previous process confirmed the session's tmux pane existed. Zero means
// never observed (legacy row, never started, or deliberately stopped and
// cleared). Thread-safe.
func (i *Instance) LastSeenAliveAt() time.Time {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.lastSeenAliveAt
}

// MarkSeenAlive records liveness evidence at t (monotonic: an older
// timestamp never rewinds it). Memory-only; the SQLite flush happens via
// persistLastSeenAlive so i.mu is never held across the DB write. Safe for
// any observer and for tests.
func (i *Instance) MarkSeenAlive(t time.Time) {
	i.mu.Lock()
	i.noteSeenAliveLocked(t)
	i.mu.Unlock()
}

// noteSeenAliveLocked folds t into the in-memory liveness record.
// Caller must hold i.mu.
func (i *Instance) noteSeenAliveLocked(t time.Time) {
	if !t.IsZero() && t.After(i.lastSeenAliveAt) {
		i.lastSeenAliveAt = t
	}
}

// persistLastSeenAlive flushes the in-memory liveness stamp to SQLite.
// Caller must NOT hold i.mu. Skips zero values: only positive liveness
// evidence flows through here — clearing goes through clearSeenAlive.
func (i *Instance) persistLastSeenAlive(force bool) {
	i.lastSeenAlivePersistMu.Lock()
	defer i.lastSeenAlivePersistMu.Unlock()

	i.mu.RLock()
	to := i.lastSeenAliveAt
	skip := to.IsZero() || !to.After(i.lastSeenAlivePersisted) ||
		(!force && to.Sub(i.lastSeenAlivePersisted) < lastSeenAlivePersistInterval)
	i.mu.RUnlock()
	if skip {
		return
	}

	db := statedb.GetGlobal()
	if db == nil {
		return
	}
	if err := db.WriteLastSeenAliveAt(i.ID, to); err != nil {
		sessionLog.Debug("last_seen_alive_persist_failed",
			slog.String("instance", i.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	i.mu.Lock()
	if to.After(i.lastSeenAlivePersisted) {
		i.lastSeenAlivePersisted = to
	}
	i.mu.Unlock()
}

// ClearSeenAlive erases the liveness stamp in memory and on disk. Called
// when the operator deliberately stops a session: from that moment the
// pane's existence is operator intent, not crash evidence, and the session
// must never be proposed for crash recovery — whatever its status column
// ends up saying. Best-effort; caller must NOT hold i.mu.
func (i *Instance) ClearSeenAlive() {
	i.mu.Lock()
	i.lastSeenAliveAt = time.Time{}
	i.lastSeenAlivePersisted = time.Time{}
	i.mu.Unlock()

	db := statedb.GetGlobal()
	if db == nil {
		return
	}
	if err := db.WriteLastSeenAliveAt(i.ID, time.Time{}); err != nil {
		sessionLog.Debug("last_seen_alive_clear_failed",
			slog.String("instance", i.ID),
			slog.String("error", err.Error()),
		)
	}
}
