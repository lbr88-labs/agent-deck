package session

// Issue #1225: durable per-parent outbox + drain. Step 1 — the unified
// producer primitives: last-wins-per-child commit, a turn_fingerprint for
// exactly-once consumer effects, and a bounded dead-letter path that logs
// ONCE for an unresolvable target (kills the dropped_no_target ~1/sec runaway).
//
// Regression anchor: a busy conductor (16c012a4) accumulated repeated
// deferred_target_busy lines and a dropped_no_target runaway because push
// could never land. The producer now writes the durable record instead.

import (
	"os"
	"strings"
	"testing"
	"time"
)

func inboxTestHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AGENT_DECK_HOME", "")
	t.Setenv("AGENT_DECK_PROFILE", "")
	ResetInboxFingerprintCacheForTest()
}

func readInboxLines(t *testing.T, parentID string) []TransitionNotificationEvent {
	t.Helper()
	path := InboxPathFor(parentID)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read inbox: %v", err)
	}
	var out []TransitionNotificationEvent
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ev, err := decodeInboxLine([]byte(line))
		if err != nil {
			t.Fatalf("decode inbox line %q: %v", line, err)
		}
		out = append(out, ev)
	}
	return out
}

// Case 5 (no-flood at the source) + case 4 (one pending record per child):
// committing N transitions for the same child leaves exactly ONE record.
func TestIssue1225_CommitToInbox_LastWinsPerChild(t *testing.T) {
	inboxTestHome(t)
	parent := "conductor-p-1777000010"
	child := "child-1777000011"

	for i := 0; i < 7; i++ {
		ev := TransitionNotificationEvent{
			ChildSessionID: child,
			ChildTitle:     "worker",
			Profile:        "personal",
			FromStatus:     "running",
			ToStatus:       "waiting",
			Timestamp:      time.Now().Add(time.Duration(i) * time.Millisecond),
			LastOutputHash: "hash-turn-final", // same turn signal
		}
		if err := CommitToInbox(parent, ev); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}

	got := readInboxLines(t, parent)
	if len(got) != 1 {
		t.Fatalf("last-wins: expected exactly 1 pending record for child, got %d:\n%+v", len(got), got)
	}
	if got[0].ChildSessionID != child {
		t.Fatalf("record child = %q, want %q", got[0].ChildSessionID, child)
	}
	if got[0].TurnFingerprint == "" {
		t.Fatalf("CommitToInbox must stamp a turn_fingerprint; got empty")
	}
}

func TestIssue1225_CommitToInbox_DistinctChildrenCoexist(t *testing.T) {
	inboxTestHome(t)
	parent := "conductor-p-1777000020"
	for _, c := range []string{"child-a", "child-b", "child-c"} {
		ev := TransitionNotificationEvent{
			ChildSessionID: c, ChildTitle: c, Profile: "personal",
			FromStatus: "running", ToStatus: "waiting", Timestamp: time.Now(),
		}
		if err := CommitToInbox(parent, ev); err != nil {
			t.Fatalf("commit %s: %v", c, err)
		}
	}
	if got := readInboxLines(t, parent); len(got) != 3 {
		t.Fatalf("expected 3 distinct-child records, got %d", len(got))
	}
}

// turn_fingerprint is stable across re-emits of the same turn (same child +
// same output hash) but distinct across turns — the basis for exactly-once
// effects (case 4) that survives a daemon restart re-stamping timestamps.
func TestIssue1225_TurnFingerprint_StablePerTurnDistinctAcrossTurns(t *testing.T) {
	base := TransitionNotificationEvent{
		ChildSessionID: "child-x", FromStatus: "running", ToStatus: "waiting",
		LastOutputHash: "turn-1", Timestamp: time.Unix(100, 0),
	}
	reEmit := base
	reEmit.Timestamp = time.Unix(999, 0) // different instant, SAME turn
	if TurnFingerprint(base) != TurnFingerprint(reEmit) {
		t.Fatalf("turn_fingerprint must be stable across re-emit of same turn: %q vs %q",
			TurnFingerprint(base), TurnFingerprint(reEmit))
	}
	nextTurn := base
	nextTurn.LastOutputHash = "turn-2"
	if TurnFingerprint(base) == TurnFingerprint(nextTurn) {
		t.Fatalf("turn_fingerprint must differ across turns")
	}
	otherChild := base
	otherChild.ChildSessionID = "child-y"
	if TurnFingerprint(base) == TurnFingerprint(otherChild) {
		t.Fatalf("turn_fingerprint must differ across children")
	}
}

// Case 6 (TTL/dead-letter, log-once, NO dropped_no_target runaway): a child
// whose parent is unresolvable is dead-lettered after MaxUnresolvedAttempts,
// and the missed-log records exactly ONE line no matter how many times the
// producer re-attempts. This is the regression test for the ~1/sec runaway.
func TestIssue1225_DeadLetter_UnresolvableLoggedOnceNoRunaway(t *testing.T) {
	inboxTestHome(t)
	logDir := t.TempDir()
	missedPath := logDir + "/notifier-missed.log"

	child := "orphan-child-1777000030"
	ev := TransitionNotificationEvent{
		ChildSessionID: child, ChildTitle: "orphan", Profile: "personal",
		FromStatus: "running", ToStatus: "waiting", Timestamp: time.Now(),
	}

	dl := NewDeadLetterSink(missedPath)
	deadLetteredAt := -1
	// Simulate the daemon re-observing the same unresolvable transition 100x
	// (once per ~1s poll). Only ONE missed line may be written, ever.
	for i := 0; i < 100; i++ {
		if dl.RecordUnresolvable(ev) && deadLetteredAt < 0 {
			deadLetteredAt = i
		}
	}

	if deadLetteredAt < 0 {
		t.Fatalf("record should be dead-lettered after MaxUnresolvedAttempts")
	}
	if deadLetteredAt >= 100 {
		t.Fatalf("dead-letter should trigger within the attempt budget, got %d", deadLetteredAt)
	}

	// The missed log must contain exactly ONE line for this child — not 100.
	raw, err := os.ReadFile(missedPath)
	if err != nil {
		t.Fatalf("read missed log: %v", err)
	}
	lines := 0
	for _, ln := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.Contains(ln, child) {
			lines++
		}
		if strings.Contains(ln, "dropped_no_target") {
			t.Fatalf("regression: dropped_no_target must not appear (runaway), got: %s", ln)
		}
	}
	if lines != 1 {
		t.Fatalf("missed log must record the unresolvable child exactly ONCE, got %d lines", lines)
	}

	// The record must be parked in the dead-letter store, not lost.
	dlEvents, err := ReadDeadLetter(child)
	if err != nil {
		t.Fatalf("read dead-letter: %v", err)
	}
	if len(dlEvents) != 1 {
		t.Fatalf("expected 1 dead-lettered record for child, got %d", len(dlEvents))
	}
}
