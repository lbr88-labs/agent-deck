package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseWaitTranscript(t *testing.T) {
	const claudeID = "65b49aa9-f2b5-4024-ac27-f10bfc1b22fe"
	for _, tc := range []struct{ tool, line, want string }{
		{"codex", `{"timestamp":"stamp","type":"event_msg","payload":{"type":"task_complete","last_agent_message":"done"}}`, "done"},
		{"codex", `{"timestamp":"stamp","type":"event_msg","payload":{"type":"user_message","last_agent_message":"echo"}}`, ""},
		{"omp", `{"type":"message","timestamp":"stamp","message":{"role":"assistant","stopReason":"stop","content":[{"type":"text","text":"done"}]}}`, "done"},
		{"omp", `{"type":"message","timestamp":"stamp","message":{"role":"assistant","stopReason":"toolUse","content":[{"type":"text","text":"working"}]}}`, ""},
		{"claude", `{"type":"assistant","sessionId":"` + claudeID + `","timestamp":"stamp","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"done"}]}}`, "done"},
		{"claude", `{"type":"assistant","sessionId":"` + claudeID + `","isSidechain":true,"timestamp":"stamp","message":{"role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"child"}]}}`, ""},
	} {
		r := parseWaitTranscript(&Instance{Tool: tc.tool, ClaudeSessionID: claudeID}, []string{tc.line})
		if tc.want == "" {
			if r != nil {
				t.Errorf("%s accepted %v", tc.tool, r)
			}
		} else if r == nil || r.Content != tc.want || r.Timestamp != "stamp" {
			t.Errorf("%s got %v", tc.tool, r)
		}
	}
}

func TestCursorWaitResponseRejectsOldGeneration(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("AGENT_DECK_HOME", "")
	t.Setenv("AGENTDECK_HOOK_GENERATION", "")
	const id = "wait-cursor-fixture"
	if err := ObserveCursorWaitTurn(id, "conversation", "old", ""); err != nil {
		t.Fatal(err)
	}
	if err := ObserveCursorWaitTurn(id, "conversation", "new", ""); err != nil {
		t.Fatal(err)
	}
	if err := ObserveCursorWaitTurn(id, "conversation", "old", "late"); !errors.Is(err, errChatSearchIdentity) {
		t.Fatalf("accepted previous turn: %v", err)
	}
	if err := ObserveCursorWaitTurn(id, "other", "new", "foreign"); !errors.Is(err, errChatSearchIdentity) {
		t.Fatalf("accepted another conversation: %v", err)
	}
	if err := ObserveCursorWaitTurn(id, "conversation", "new", "finished"); err != nil {
		t.Fatal(err)
	}
	r, err := readCursorWaitResponse(id)
	if err != nil || r.Content != "finished" || r.Role != "assistant" || r.Timestamp == "" {
		t.Fatalf("reply=%+v err=%v", r, err)
	}
	if err := os.WriteFile(filepath.Join(GetHooksDir(), id+".generation.json"), []byte(`{"generation":"restarted"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readCursorWaitResponse(id); !os.IsNotExist(err) {
		t.Fatalf("stale generation accepted: %v", err)
	}
}

func TestWaitTranscriptHeader(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		codex      bool
		want       error
	}{
		{"codex match", `{"type":"session_meta","payload":{"id":"mine"}}` + "\n", true, nil},
		{"codex replaced", `{"type":"session_meta","payload":{"id":"other"}}` + "\n", true, errChatSearchIdentity},
		{"omp match", `{"type":"session","id":"mine"}` + "\n", false, nil},
		{"omp replaced", `{"type":"session","id":"other"}` + "\n", false, errChatSearchIdentity},
		{"new empty", "", true, os.ErrNotExist},
		{"incomplete header", `{"type":"ses`, true, os.ErrNotExist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.CreateTemp(t.TempDir(), "transcript")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if _, err = f.WriteString(tc.body); err != nil {
				t.Fatal(err)
			}
			if _, err = f.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			if err = verifyWaitTranscriptHeader(f, "mine", tc.codex); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}
