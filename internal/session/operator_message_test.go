package session

import (
	"context"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
)

func TestOperatorInboxCorrelatesRepliesAcrossHarnesses(t *testing.T) {
	t.Setenv("AGENT_DECK_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dir, err := GetProfileDir("test")
	if err != nil {
		t.Fatal(err)
	}
	store, err := peermsg.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, tool := range []string{"claude", "cursor", "omp"} {
		t.Run(tool, func(t *testing.T) {
			inst := Instance{ID: tool, Tool: tool}
			first, err := inst.QueueOperatorMessage(ctx, "test", "first")
			if err != nil {
				t.Fatal(err)
			}
			second, err := inst.QueueOperatorMessage(ctx, "test", "second")
			if err != nil || first.ID == second.ID || first.SubmittedAt.IsZero() {
				t.Fatalf("second receipt %+v: %v", second, err)
			}
			for _, request := range []struct{ id, body string }{{second.ID, "unrelated"}, {first.ID, "matching"}} {
				_, err := store.Send(ctx, peermsg.SendRequest{SenderID: inst.ID, RecipientID: "system:operator", ReplyTo: request.id, Body: request.body})
				if err != nil {
					t.Fatal(err)
				}
			}
			reply, err := WaitOperatorReply(ctx, "test", first.ID)
			if err != nil || reply.Body != "matching" {
				t.Fatalf("reply %+v: %v", reply, err)
			}
		})
	}
}
