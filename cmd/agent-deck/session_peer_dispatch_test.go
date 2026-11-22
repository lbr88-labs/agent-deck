package main

import "testing"

func TestSessionSendFromAgent(t *testing.T) {
	keys := []string{"TMUX", "AGENT_DECK_SESSION_ID", "AGENTDECK_INSTANCE_ID", "CODEX_THREAD_ID", "CLAUDECODE", "CURSOR_AGENT", "OMP_SESSION_ID"}
	for _, k := range keys {
		t.Setenv(k, "")
	}
	if sessionSendFromAgent() {
		t.Fatal("plain operator shell incorrectly classified as an agent")
	}
	for _, k := range keys[1:] {
		t.Run(k, func(t *testing.T) {
			t.Setenv(k, "managed")
			if !sessionSendFromAgent() {
				t.Fatal("agent tool shell allowed to use terminal delivery")
			}
		})
	}
}
