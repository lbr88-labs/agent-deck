package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
	"github.com/asheshgoplani/agent-deck/internal/session"
)

func TestStreamOperatorReplyCorrelatesBodyAndPreservesUnicode(t *testing.T) {
	t.Setenv("AGENT_DECK_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	inst := session.Instance{ID: "target", Tool: "codex"}
	receipt, err := inst.QueueOperatorRequest(ctx, "test", "what happened?")
	if err != nil {
		t.Fatal(err)
	}
	id := receipt.ID
	dir, err := session.GetProfileDir("test")
	if err != nil {
		t.Fatal(err)
	}
	store, err := peermsg.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	reply, err := store.Send(ctx, peermsg.SendRequest{SenderID: inst.ID, RecipientID: "system:operator", ReplyTo: id, Body: "æøå"})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := streamOperatorReply(ctx, &buf, inst.ID, "test", id, 2); err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(&buf)
	for _, want := range []session.StreamEvent{{Type: "start", MessageID: id}, {Type: "text", MessageID: reply.ID, Delta: "æø"}, {Type: "text", MessageID: reply.ID, Delta: "å"}, {Type: "stop", MessageID: reply.ID}} {
		var got session.StreamEvent
		if err := d.Decode(&got); err != nil || got.Type != want.Type || got.MessageID != want.MessageID || got.Delta != want.Delta {
			t.Fatalf("event = %+v, error %v; want %+v", got, err, want)
		}
	}
}
