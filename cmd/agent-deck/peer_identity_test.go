package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

func TestPeerCodexThreadIdentity(t *testing.T) {
	t.Setenv("AGENT_DECK_HOME", t.TempDir())
	t.Setenv("AGENTDECK_INSTANCE_ID", "")
	const thread = "01a1d513-38b8-7a20-9422-7bc04409c700"
	t.Setenv("CODEX_THREAD_ID", thread)
	const profile = "thread-identity"
	reg, err := session.NewStorageWithProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	instance := func(id string) *session.Instance {
		return &session.Instance{ID: id, Tool: "codex", CodexSessionID: thread, CodexBindingConfirmed: true}
	}
	row := instance("owner")
	if err := reg.Save([]*session.Instance{row}); err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"read", "mcp"} {
		if err := runPeerCommand(context.Background(), profile, []string{op}, io.NopCloser(strings.NewReader("")), io.Discard); err != nil {
			t.Fatalf("%s: %v", op, err)
		}
	}
	svc, err := openPeerService(profile)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if svc.self != row.ID {
		t.Fatalf("resolved %q, want %q", svc.self, row.ID)
	}
	reject := func(profile, want string) {
		t.Helper()
		service, err := openPeerService(profile)
		if err == nil {
			service.Close()
			t.Fatalf("unexpected identity for %s", profile)
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("want %q, got %v", want, err)
		}
	}
	t.Run("explicit ID and native thread", func(t *testing.T) {
		t.Setenv("AGENTDECK_INSTANCE_ID", row.ID)
		for _, native := range []string{thread, ""} {
			t.Setenv("CODEX_THREAD_ID", native)
			if err := runPeerCommand(context.Background(), profile, []string{"read"}, io.NopCloser(strings.NewReader("")), io.Discard); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("CODEX_THREAD_ID", "different-native-thread")
		reject(profile, "conflicts with the confirmed binding")
	})
	reject("different-profile", "no confirmed session")
	unconfirmed := instance("unconfirmed")
	unconfirmed.CodexBindingConfirmed = false
	otherTool := instance("other-tool")
	otherTool.Tool = "claude"
	untrusted, err := session.NewStorageWithProfile("untrusted-bindings")
	if err != nil {
		t.Fatal(err)
	}
	defer untrusted.Close()
	if err := untrusted.Save([]*session.Instance{unconfirmed, otherTool}); err != nil {
		t.Fatal(err)
	}
	reject("untrusted-bindings", "no confirmed session")
	t.Run("other harness with inherited Codex environment", func(t *testing.T) {
		t.Setenv("AGENTDECK_INSTANCE_ID", otherTool.ID)
		t.Setenv("CODEX_THREAD_ID", "different-native-thread")
		if err := runPeerCommand(context.Background(), "untrusted-bindings", []string{"read"}, io.NopCloser(strings.NewReader("")), io.Discard); err != nil {
			t.Fatal(err)
		}
	})
	t.Setenv("CODEX_THREAD_ID", thread[:12])
	reject(profile, "no confirmed session")
	t.Setenv("CODEX_THREAD_ID", thread)
	if err := reg.Save([]*session.Instance{row, instance("duplicate")}); err != nil {
		t.Fatal(err)
	}
	reject(profile, "multiple sessions")
}
