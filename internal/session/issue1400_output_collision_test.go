package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Issue #1400 (first half): `session output <id> -q` returned byte-identical
// output for distinct sessions because multiple live instance rows resolved to
// one claude_session_id → one transcript. #1352 wired the collision-checked
// resolver (GetJSONLPathChecked) into `session output --stream` only; the
// -q/last-response path still read the colliding transcript silently.
//
// These tests pin the fix: GetLastResponseBestEffortChecked must refuse the
// read with #1352's collision semantics, while non-colliding reads behave
// exactly like GetLastResponseBestEffort.
//
// Written test-first: TestOutputCollision_CheckedRefusesSharedTranscript fails
// on pre-#1400 main (the checked variant did not exist; the -q path returned
// the same bytes for every collided instance, as documented by the unchecked
// assertion inside the test).

// isolateHome1400 sandboxes HOME + XDG so nothing touches a real profile.
func isolateHome1400(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("AGENT_DECK_HOME", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	ClearUserConfigCache()
	t.Cleanup(func() { ClearUserConfigCache() })
	return home
}

// writeAssistantTranscript1400 materializes a Claude transcript jsonl whose
// last assistant message is `reply`, so GetLastResponse parses real content.
func writeAssistantTranscript1400(t *testing.T, projectPath, sessionID, reply string) {
	t.Helper()
	configDir := GetClaudeConfigDir()
	projDir := filepath.Join(configDir, "projects", ConvertToClaudeDirName(projectPath))
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir transcript dir: %v", err)
	}
	records := []map[string]any{
		{
			"sessionId": sessionID,
			"type":      "user",
			"message":   map[string]any{"role": "user", "content": "hello"},
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		},
		{
			"sessionId": sessionID,
			"type":      "assistant",
			"message":   map[string]any{"role": "assistant", "content": reply},
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
	f, err := os.Create(filepath.Join(projDir, sessionID+".jsonl"))
	if err != nil {
		t.Fatalf("create transcript: %v", err)
	}
	defer f.Close()
	for _, rec := range records {
		b, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("marshal record: %v", err)
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			t.Fatalf("write record: %v", err)
		}
	}
}

func mkInstance1400(id, projectPath, claudeSessionID string) *Instance {
	return &Instance{
		ID:              id,
		Title:           id,
		ProjectPath:     projectPath,
		GroupPath:       DefaultGroupPath,
		Tool:            "claude",
		Status:          StatusRunning,
		ClaudeSessionID: claudeSessionID,
		CreatedAt:       time.Now(),
	}
}

// TestOutputCollision_CheckedRefusesSharedTranscript is the PRIMARY #1400
// regression test: two live instances forced onto one claude_session_id (and
// one project path) must NOT both read the same "last response". First the
// test documents the bug shape (the unchecked read returns byte-identical
// content for both), then asserts the checked variant refuses both reads with
// the #1352 collision error.
func TestOutputCollision_CheckedRefusesSharedTranscript(t *testing.T) {
	home := isolateHome1400(t)

	projectPath := filepath.Join(home, "proj1400")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	sharedID := "shared-session-1400"
	writeAssistantTranscript1400(t, projectPath, sharedID, "one transcript to rule them all")

	a := mkInstance1400("inst-a-1400", projectPath, sharedID)
	b := mkInstance1400("inst-b-1400", projectPath, sharedID)
	peers := []*Instance{a, b}

	// Bug shape (pre-#1400): the unchecked path returns the SAME bytes for two
	// distinct sessions. This is the silent corruption the guard must surface.
	respA, errA := a.GetLastResponseBestEffort()
	respB, errB := b.GetLastResponseBestEffort()
	if errA != nil || errB != nil {
		t.Fatalf("unchecked reads should succeed (bug shape), got errA=%v errB=%v", errA, errB)
	}
	if respA.Content != respB.Content {
		t.Fatalf("test setup broken: expected byte-identical unchecked output, got %q vs %q", respA.Content, respB.Content)
	}

	// The fix: the checked variant refuses both reads.
	if resp, err := a.GetLastResponseBestEffortChecked(peers); err == nil {
		t.Fatalf("expected collision error for instance a, got content %q", resp.Content)
	} else if !strings.Contains(err.Error(), "colliding transcript") {
		t.Fatalf("expected colliding-transcript error for instance a, got: %v", err)
	}
	if resp, err := b.GetLastResponseBestEffortChecked(peers); err == nil {
		t.Fatalf("expected collision error for instance b, got content %q", resp.Content)
	} else if !strings.Contains(err.Error(), "colliding transcript") {
		t.Fatalf("expected colliding-transcript error for instance b, got: %v", err)
	}
}

// TestOutputCollision_UniqueIDsReadNormally guards against over-blocking:
// instances with distinct claude_session_ids (even in the same project) must
// read their own transcripts through the checked variant.
func TestOutputCollision_UniqueIDsReadNormally(t *testing.T) {
	home := isolateHome1400(t)

	projectPath := filepath.Join(home, "proj1400-unique")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	writeAssistantTranscript1400(t, projectPath, "session-a-1400", "reply from a")
	writeAssistantTranscript1400(t, projectPath, "session-b-1400", "reply from b")

	a := mkInstance1400("inst-a-1400u", projectPath, "session-a-1400")
	b := mkInstance1400("inst-b-1400u", projectPath, "session-b-1400")
	peers := []*Instance{a, b}

	respA, err := a.GetLastResponseBestEffortChecked(peers)
	if err != nil {
		t.Fatalf("unique id read for a must not error: %v", err)
	}
	if respA.Content != "reply from a" {
		t.Fatalf("instance a read wrong transcript: %q", respA.Content)
	}
	respB, err := b.GetLastResponseBestEffortChecked(peers)
	if err != nil {
		t.Fatalf("unique id read for b must not error: %v", err)
	}
	if respB.Content != "reply from b" {
		t.Fatalf("instance b read wrong transcript: %q", respB.Content)
	}
}

// TestOutputCollision_StoppedPeerDoesNotBlock mirrors #1352's live-only
// semantics: a STOPPED peer sharing the session id is not a routing hazard and
// must not block the live instance's read.
func TestOutputCollision_StoppedPeerDoesNotBlock(t *testing.T) {
	home := isolateHome1400(t)

	projectPath := filepath.Join(home, "proj1400-stopped")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	sharedID := "shared-session-1400s"
	writeAssistantTranscript1400(t, projectPath, sharedID, "live owner reply")

	live := mkInstance1400("inst-live-1400", projectPath, sharedID)
	stopped := mkInstance1400("inst-stopped-1400", projectPath, sharedID)
	stopped.Status = StatusStopped

	resp, err := live.GetLastResponseBestEffortChecked([]*Instance{live, stopped})
	if err != nil {
		t.Fatalf("stopped peer must not block the live read: %v", err)
	}
	if resp.Content != "live owner reply" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
}
