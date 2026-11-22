package peermsg

import (
	"context"
	"strings"
	"sync"
	"testing"
)

func TestNativeWakeClaimIsDurableAtMostOnce(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	open := func() *Store {
		t.Helper()
		s, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		return s
	}
	writer, other := open(), open()
	first, err := writer.Send(ctx, SendRequest{ID: "one", SenderID: "agent", RecipientID: "codex", Body: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for _, store := range []*Store{writer, other} {
		wg.Go(func() {
			claimed, err := store.ClaimNativeWake(ctx, first)
			if !claimed && (err == nil || !strings.Contains(err.Error(), "unconfirmed")) {
				t.Errorf("concurrent unconfirmed claim: %v", err)
			}
			results <- claimed
		})
	}
	wg.Wait()
	firstClaimed, secondClaimed := <-results, <-results
	if firstClaimed == secondClaimed {
		t.Fatal("expected exactly one native queue invocation")
	}
	// Reopening simulates retry after the first sender crashed inside its native invocation.
	reopened := open()
	if claimed, err := reopened.ClaimNativeWake(ctx, first); claimed || err == nil {
		t.Fatal("retried uncertain wake", claimed, err)
	}
	if err := writer.RecordNativeWake(ctx, first.ID, "native-receipt"); err != nil {
		t.Fatal(err)
	}
	if claimed, err := reopened.ClaimNativeWake(ctx, first); claimed || err != nil {
		t.Fatal("retried accepted wake", claimed, err)
	}
	distinct, err := writer.Send(ctx, SendRequest{ID: "two", SenderID: "agent", RecipientID: "codex", Body: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if claimed, err := other.ClaimNativeWake(ctx, distinct); !claimed || err != nil {
		t.Fatal("lost distinct message", claimed, err)
	}
	if _, err := writer.Ack(ctx, "codex", distinct.ID); err != nil {
		t.Fatal(err)
	}
	if claimed, err := reopened.ClaimNativeWake(ctx, distinct); claimed || err != nil {
		t.Fatal("retried acknowledged message", claimed, err)
	}
}
