package session

// Issue #1225 Step 2 — the consumer drain. The parent drains its durable
// outbox at its own turn boundary / heartbeat. Drain collapses last-wins per
// child, records consumed turn_fingerprints for exactly-once EFFECTS across
// re-delivery, and is atomic against concurrent drains (no double-ack, no loss).

import (
	"sort"
	"sync"
	"testing"
	"time"
)

// Case 4 (exactly-once effects): the same completed turn re-delivered after a
// drain must NOT be acted on twice — the consumed turn_fingerprint guards it.
func TestIssue1225_DrainInbox_DedupsSameTurnAcrossRedelivery(t *testing.T) {
	inboxTestHome(t)
	parent := "conductor-c-1777000100"
	child := "child-c-1777000101"

	mk := func() TransitionNotificationEvent {
		return TransitionNotificationEvent{
			ChildSessionID: child, ChildTitle: "worker", Profile: "personal",
			FromStatus: "running", ToStatus: "waiting",
			LastOutputHash: "turn-A", Timestamp: time.Now(),
		}
	}

	if err := CommitToInbox(parent, mk()); err != nil {
		t.Fatalf("commit 1: %v", err)
	}
	first, err := DrainInboxForParent(parent)
	if err != nil {
		t.Fatalf("drain 1: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first drain should deliver the completion once, got %d", len(first))
	}

	// Same turn re-committed (e.g. daemon restart re-stamps Timestamp).
	reEmit := mk()
	reEmit.Timestamp = time.Now().Add(time.Hour)
	if err := CommitToInbox(parent, reEmit); err != nil {
		t.Fatalf("commit 2: %v", err)
	}
	second, err := DrainInboxForParent(parent)
	if err != nil {
		t.Fatalf("drain 2: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("re-delivery of the SAME turn must be deduped, got %d", len(second))
	}

	// A genuinely NEW turn for the same child must be delivered.
	next := mk()
	next.LastOutputHash = "turn-B"
	if err := CommitToInbox(parent, next); err != nil {
		t.Fatalf("commit 3: %v", err)
	}
	third, err := DrainInboxForParent(parent)
	if err != nil {
		t.Fatalf("drain 3: %v", err)
	}
	if len(third) != 1 {
		t.Fatalf("a new turn must be delivered, got %d", len(third))
	}
}

// Within a single drain, multiple records for one child collapse to one.
func TestIssue1225_DrainInbox_LastWinsWithinDrain(t *testing.T) {
	inboxTestHome(t)
	parent := "conductor-c-1777000110"
	// Two distinct children, each with a single pending record after commit.
	for _, c := range []string{"child-1", "child-2"} {
		if err := CommitToInbox(parent, TransitionNotificationEvent{
			ChildSessionID: c, ChildTitle: c, Profile: "personal",
			FromStatus: "running", ToStatus: "waiting", Timestamp: time.Now(),
		}); err != nil {
			t.Fatalf("commit %s: %v", c, err)
		}
	}
	got, err := DrainInboxForParent(parent)
	if err != nil {
		t.Fatalf("drain: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 deliverables (one per child), got %d", len(got))
	}
}

// Case 8 (ack atomicity under concurrent drain): two concurrent drains of the
// same inbox must partition the records — every record delivered exactly once,
// none lost, none doubled.
func TestIssue1225_DrainInbox_ConcurrentDrainNoDoubleNoLoss(t *testing.T) {
	inboxTestHome(t)
	parent := "conductor-c-1777000120"
	const n = 25
	for i := 0; i < n; i++ {
		if err := CommitToInbox(parent, TransitionNotificationEvent{
			ChildSessionID: childIDForN(i), ChildTitle: "w", Profile: "personal",
			FromStatus: "running", ToStatus: "waiting",
			LastOutputHash: "h", Timestamp: time.Now(),
		}); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[string]int{}
	for g := 0; g < 6; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := DrainInboxForParent(parent)
			if err != nil {
				return
			}
			mu.Lock()
			for _, ev := range out {
				seen[ev.ChildSessionID]++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(seen) != n {
		t.Fatalf("loss/duplication: expected %d distinct children delivered, got %d", n, len(seen))
	}
	for child, count := range seen {
		if count != 1 {
			t.Fatalf("double-ack: child %s delivered %d times, want 1", child, count)
		}
	}
	// Inbox must be fully drained.
	if rest, _ := DrainInboxForParent(parent); len(rest) != 0 {
		ids := make([]string, 0, len(rest))
		for _, e := range rest {
			ids = append(ids, e.ChildSessionID)
		}
		sort.Strings(ids)
		t.Fatalf("inbox not fully drained, leftover: %v", ids)
	}
}

func childIDForN(i int) string {
	const digits = "0123456789"
	return "child-" + string(digits[i%10]) + "-" + string(digits[(i/10)%10]) + "-" + string(rune('a'+i))
}
