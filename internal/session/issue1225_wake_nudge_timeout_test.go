package session

import (
	"context"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
)

func TestIssue1225_WakeNudgeEnqueuesForExactParent(t *testing.T) {
	t.Setenv("AGENT_DECK_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	const profile = "work"
	const parent = "parent-y"
	if err := sendWakeNudgeNoWait(profile, parent); err != nil {
		t.Fatal(err)
	}
	dir, err := GetProfileDir(profile)
	if err != nil {
		t.Fatal(err)
	}
	store, err := peermsg.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rows, err := store.Pending(context.Background(), parent, 0, 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("wake not in parent inbox: %v, %v", rows, err)
	}
	if got := rows[0]; got.SenderID != "system:completion-wake" || got.Body != wakeNudgeMessage {
		t.Fatalf("unexpected wake: %+v", got)
	}
}
