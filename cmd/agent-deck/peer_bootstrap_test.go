package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

func TestPeerBootstrapRetryKeepsOneInboxTaskAndLeavesNoTerminalBody(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("AGENT_DECK_HOME", filepath.Join(home, "deck"))
	t.Setenv("AGENTDECK_INSTANCE_ID", "manager")
	session.ClearUserConfigCache()
	t.Cleanup(session.ClearUserConfigCache)
	const profile = "bootstrap"
	manager := &session.Instance{ID: "manager", Title: "manager"}
	target := &session.Instance{ID: "worker", Title: "worker", Tool: "claude"}
	reg, err := session.NewStorageWithProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	if err := reg.Save([]*session.Instance{manager, target}); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		task, err := preparePeerBootstrap(profile, "check", "initial-1")
		if err != nil {
			t.Fatal(err)
		}
		body, err := task.terminalBody(target, "check")
		if err != nil || body != "" {
			t.Fatalf("terminal body %q, error %v", body, err)
		}
		receipt := map[string]interface{}{}
		task.addReceipt(receipt)
		if receipt["message_id"] != "initial-1" || receipt["message_pending"] != true {
			t.Fatalf("receipt %+v", receipt)
		}
		messages, err := task.service.store.Pending(context.Background(), target.ID, 0, 10)
		if err != nil || len(messages) != 1 || messages[0].SenderID != manager.ID {
			t.Fatalf("inbox %+v, error %v", messages, err)
		}
		if _, err := task.terminalBody(target, "different task"); err == nil {
			t.Fatal("reused ID accepted with a different body")
		}
		_ = task.service.Close()
	}
}
