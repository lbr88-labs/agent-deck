package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
	"github.com/asheshgoplani/agent-deck/internal/session"
)

func TestPeerHookNoticeIsAddressedAndStopContinuationIsBounded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("AGENTDECK_PROFILE", "test-hooks")
	t.Setenv("AGENTDECK_TOOL", "claude")
	dir, err := session.GetProfileDir("test-hooks")
	if err != nil {
		t.Fatal(err)
	}
	store, err := peermsg.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	msg, err := store.Send(ctx, peermsg.SendRequest{SenderID: "a", RecipientID: "b", Body: "private message body"})
	if err != nil {
		t.Fatal(err)
	}
	if peerInboxHookNotice("a") != "" || peerInboxHookNotice("b") != pendingPeerNotice {
		t.Fatal("hook should notify only the addressed session")
	}
	dec := session.StopHookDecision{Decision: "block", Reason: "child finished"}
	active := false
	payload := hookPayload{HookEventName: "Stop", StopHookActive: &active}
	got := withPeerStopNotice(dec, payload, "b")
	if got != dec {
		t.Fatalf("unverified Stop received a peer notice: %#v", got)
	}
	active = true
	if got := withPeerStopNotice(dec, payload, "b"); got != dec {
		t.Fatalf("unexpected peer notice on induced stop: %#v", got)
	}
	payload.StopHookActive = nil
	if got := withPeerStopNotice(session.StopHookDecision{}, payload, "b"); got.Decision != "" {
		t.Fatalf("blocked with missing stop guard: %#v", got)
	}
	if _, err := store.Ack(ctx, "b", msg.ID); err != nil {
		t.Fatal(err)
	}
	if peerInboxHookNotice("b") != "" {
		t.Fatal("acknowledged message still generating notice")
	}
}
