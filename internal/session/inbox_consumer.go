package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Issue #1225: the consumer side of the durable per-parent outbox. The parent
// drains its inbox at its own turn boundary (Stop hook) and on heartbeat. The
// drain:
//   - reads + truncates the inbox atomically (one winner under inboxWriteMu, so
//     concurrent drains partition the records — no double-ack, no loss),
//   - collapses to last-wins per child (one deliverable per child),
//   - skips turn_fingerprints already consumed (exactly-once EFFECTS across a
//     re-delivery, e.g. after a daemon restart re-stamps timestamps),
//   - records the rest as consumed.
//
// Truncation IS the ack for the durable record; the consumed-fingerprint set is
// the second-line guard that converts at-least-once delivery into exactly-once
// effects (the outbox+inbox dedup-table pattern).

// consumedTurnsTTL bounds the consumed-fingerprint ledger so it can't grow
// without limit. A turn older than this can never be re-delivered (the inbox
// TTL is shorter), so forgetting it is safe.
const consumedTurnsTTL = 14 * 24 * time.Hour

var consumedTurnsMu sync.Mutex

// ConsumedTurnsDir holds per-parent consumed-fingerprint ledgers.
func ConsumedTurnsDir() string {
	dir, err := GetAgentDeckDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".agent-deck", "runtime", "consumed-turns")
	}
	return filepath.Join(dir, "runtime", "consumed-turns")
}

func consumedTurnsPathFor(parentID string) string {
	return filepath.Join(ConsumedTurnsDir(), sanitizeInboxName(parentID)+".json")
}

func loadConsumedTurnsLocked(parentID string) map[string]int64 {
	out := map[string]int64{}
	raw, err := os.ReadFile(consumedTurnsPathFor(parentID))
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func saveConsumedTurnsLocked(parentID string, m map[string]int64) error {
	// Prune expired entries on every save to bound growth.
	cutoff := time.Now().Add(-consumedTurnsTTL).Unix()
	for fp, ts := range m {
		if ts < cutoff {
			delete(m, fp)
		}
	}
	path := consumedTurnsPathFor(parentID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// DrainInboxForParent drains the parent's durable outbox and returns the
// deliverables (last-wins per child, exactly-once per turn). See file comment.
func DrainInboxForParent(parentID string) ([]TransitionNotificationEvent, error) {
	if strings.TrimSpace(parentID) == "" {
		return nil, errors.New("inbox drain: empty parent session id")
	}

	// Atomic gate: exactly one concurrent caller gets the records and truncates.
	raw, err := ReadAndTruncateInbox(parentID)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}

	collapsed := collapseLastWins(raw)

	consumedTurnsMu.Lock()
	defer consumedTurnsMu.Unlock()
	consumed := loadConsumedTurnsLocked(parentID)

	now := time.Now().Unix()
	var out []TransitionNotificationEvent
	dirty := false
	for _, ev := range collapsed {
		fp := ev.TurnFingerprint
		if fp == "" {
			fp = TurnFingerprint(ev)
		}
		if _, seen := consumed[fp]; seen {
			continue // exactly-once effects: this turn was already acted on
		}
		consumed[fp] = now
		dirty = true
		out = append(out, ev)
	}
	if dirty {
		if err := saveConsumedTurnsLocked(parentID, consumed); err != nil {
			return out, err
		}
	}
	return out, nil
}

// collapseLastWins reduces multiple records for one child to the single latest
// (by Timestamp), preserving first-seen order of children for stable output.
func collapseLastWins(events []TransitionNotificationEvent) []TransitionNotificationEvent {
	latest := map[string]TransitionNotificationEvent{}
	order := []string{}
	for _, ev := range events {
		cur, seen := latest[ev.ChildSessionID]
		if !seen {
			order = append(order, ev.ChildSessionID)
			latest[ev.ChildSessionID] = ev
			continue
		}
		if !ev.Timestamp.Before(cur.Timestamp) {
			latest[ev.ChildSessionID] = ev
		}
	}
	out := make([]TransitionNotificationEvent, 0, len(order))
	for _, id := range order {
		out = append(out, latest[id])
	}
	return out
}

// ForgetConsumedTurnsForChild removes any consumed-turn ledger entries for a
// child across all parents — used by rm_sweep on child removal so the ledger
// doesn't leak. Best-effort.
func ForgetConsumedTurnsForChild(childSessionID string) {
	child := strings.TrimSpace(childSessionID)
	if child == "" {
		return
	}
	prefix := child + "@"
	consumedTurnsMu.Lock()
	defer consumedTurnsMu.Unlock()
	entries, err := os.ReadDir(ConsumedTurnsDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		parentID := strings.TrimSuffix(e.Name(), ".json")
		m := loadConsumedTurnsLocked(parentID)
		changed := false
		for fp := range m {
			if strings.HasPrefix(fp, prefix) {
				delete(m, fp)
				changed = true
			}
		}
		if changed {
			_ = saveConsumedTurnsLocked(parentID, m)
		}
	}
}
