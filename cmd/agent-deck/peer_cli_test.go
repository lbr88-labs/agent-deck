package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
	"github.com/asheshgoplani/agent-deck/internal/session"
)

func TestPeerCLIProfileRoundTrip(t *testing.T) {
	home := t.TempDir()
	for k, v := range map[string]string{"HOME": home, "AGENT_DECK_HOME": filepath.Join(home, "deck"), "XDG_DATA_HOME": filepath.Join(home, "data"), "XDG_CONFIG_HOME": filepath.Join(home, "config"), "XDG_CACHE_HOME": filepath.Join(home, "cache")} {
		t.Setenv(k, v)
	}
	session.ClearUserConfigCache()
	t.Cleanup(session.ClearUserConfigCache)
	const profile = "peers"
	sessions, err := session.NewStorageWithProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	err = sessions.Save([]*session.Instance{{ID: "first", Title: "writer"}, {ID: "second", Title: "reader"}, {ID: "duplicate-one", Title: "duplicate"}, {ID: "duplicate-two", Title: "duplicate"}})
	if closeErr := sessions.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	run := func(self string, args ...string) error {
		t.Helper()
		t.Setenv("AGENTDECK_INSTANCE_ID", self)
		out.Reset()
		return runPeerCommand(context.Background(), profile, args, io.NopCloser(strings.NewReader("")), &out)
	}
	must := func(self string, args ...string) {
		t.Helper()
		if err := run(self, args...); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("AGENT_DECK_SESSION_ID", "")
	t.Setenv("TMUX", "")
	if err := run("first", "notify", "--producer", "watchdog", "reader", "event"); err == nil {
		t.Fatal("managed caller accepted as producer")
	}
	if err := run("", "notify", "--producer", "arbitrary", "reader", "event"); err == nil {
		t.Fatal("unknown producer accepted")
	}
	if err := run("", "notify", "--producer", "watchdog", "duplicate", "event"); err == nil {
		t.Fatal("ambiguous notification recipient accepted")
	}
	must("", "notify", "--json", "--id", "tick", "--producer", "heartbeat", "reader", "event")
	var notice peermsg.Message
	if err := json.Unmarshal(out.Bytes(), &notice); err != nil || notice.SenderID != "system:heartbeat" || notice.RecipientID != "second" {
		t.Fatalf("notification %s, err %v", out.Bytes(), err)
	}
	must("second", "ack", "tick")
	must("", "wait", "--producer", "heartbeat", "--response-to", "tick", "--timeout=0s", "--reply-only", "--json")
	var hb peermsg.Response
	if err := json.Unmarshal(out.Bytes(), &hb); err != nil || !hb.TimedOut {
		t.Fatalf("expected reply-only timeout: %s", out.Bytes())
	}
	if err := run("", "wait", "--producer", "operator", "--response-to", "tick", "--timeout=0s"); err == nil {
		t.Fatal("wrong producer observed heartbeat")
	}
	if err := run("first", "send", "--reply-to", "tick", "system:heartbeat", "spoof"); err == nil {
		t.Fatal("wrong session replied to producer")
	}
	if err := run("second", "send", "system:heartbeat", "stray"); err == nil {
		t.Fatal("uncorrelated system message accepted")
	}
	must("second", "send", "--reply-to", "tick", "system:heartbeat", "ready")
	must("", "wait", "--producer", "heartbeat", "--response-to", "tick", "--timeout=0s", "--reply-only", "--json")
	if err := json.Unmarshal(out.Bytes(), &hb); err != nil || hb.TimedOut || len(hb.Replies) != 1 {
		t.Fatalf("missing heartbeat reply: %s", out.Bytes())
	}
	must("", "notify", "--id", "op", "--producer", "operator", "first", "hello")
	must("first", "ack", "op")
	if err := run("first", "send", "reader", "mail"); err == nil {
		t.Fatal("ambiguous title accepted")
	}
	if err := run("absent", "read"); err == nil {
		t.Fatal("unknown sender accepted")
	}
	must("first", "send", "--json", "--id", "stable", "second", "--", "--literal")
	must("second", "read", "--json")
	var pending []peermsg.Message
	if err := json.Unmarshal(out.Bytes(), &pending); err != nil || len(pending) != 1 || pending[0].Body != "--literal" {
		t.Fatalf("pending %s, err %v", out.Bytes(), err)
	}
	must("first", "wait", "--response-to", "stable", "--timeout", "0s", "--json")
	var response peermsg.Response
	if err := json.Unmarshal(out.Bytes(), &response); err != nil || !response.TimedOut {
		t.Fatalf("wait %s, err %v", out.Bytes(), err)
	}
	must("second", "send", "--reply-to", "stable", "first", "done")
	must("second", "ack", "stable")
	must("first", "wait", "--response-to", "stable", "--timeout", "0s", "--json")
	if err := json.Unmarshal(out.Bytes(), &response); err != nil || response.TimedOut || len(response.Replies) != 1 || response.Message.AcknowledgedAt == nil {
		t.Fatalf("response %s, err %v", out.Bytes(), err)
	}
	must("second", "read", "--json")
	if strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("pending after ack %s", out.Bytes())
	}
}
