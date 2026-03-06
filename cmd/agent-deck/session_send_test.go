package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

// mockStatusChecker implements statusChecker for testing waitForCompletion.
type mockStatusChecker struct {
	statuses []string // statuses returned in order
	errors   []error  // errors returned in order (nil = no error)
	idx      atomic.Int32
}

func (m *mockStatusChecker) GetStatus() (string, error) {
	i := int(m.idx.Add(1) - 1)
	if i >= len(m.statuses) {
		// Stay on last status if we exceed the list
		i = len(m.statuses) - 1
	}
	var err error
	if i < len(m.errors) {
		err = m.errors[i]
	}
	return m.statuses[i], err
}

func TestWaitForCompletion_ImmediateWaiting(t *testing.T) {
	mock := &mockStatusChecker{
		statuses: []string{"waiting"},
	}
	status, err := waitForCompletion(mock, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "waiting" {
		t.Errorf("expected status 'waiting', got %q", status)
	}
}

func TestWaitForCompletion_ActiveThenWaiting(t *testing.T) {
	mock := &mockStatusChecker{
		statuses: []string{"active", "active", "waiting"},
	}
	status, err := waitForCompletion(mock, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "waiting" {
		t.Errorf("expected status 'waiting', got %q", status)
	}
}

func TestWaitForCompletion_ActiveThenIdle(t *testing.T) {
	mock := &mockStatusChecker{
		statuses: []string{"active", "idle"},
	}
	status, err := waitForCompletion(mock, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "idle" {
		t.Errorf("expected status 'idle', got %q", status)
	}
}

func TestWaitForCompletion_ActiveThenInactive(t *testing.T) {
	mock := &mockStatusChecker{
		statuses: []string{"active", "inactive"},
	}
	status, err := waitForCompletion(mock, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "inactive" {
		t.Errorf("expected status 'inactive', got %q", status)
	}
}

func TestWaitForCompletion_TransientErrors(t *testing.T) {
	mock := &mockStatusChecker{
		statuses: []string{"", "", "waiting"},
		errors:   []error{fmt.Errorf("tmux error"), fmt.Errorf("tmux error"), nil},
	}
	status, err := waitForCompletion(mock, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "waiting" {
		t.Errorf("expected status 'waiting', got %q", status)
	}
}

func TestWaitForCompletion_SessionDeath(t *testing.T) {
	// When GetStatus returns 5+ consecutive errors, the session is dead.
	// waitForCompletion should return ("error", nil) instead of hanging.
	mock := &mockStatusChecker{
		statuses: []string{"", "", "", "", "", "", ""},
		errors: []error{
			fmt.Errorf("tmux session not found"),
			fmt.Errorf("tmux session not found"),
			fmt.Errorf("tmux session not found"),
			fmt.Errorf("tmux session not found"),
			fmt.Errorf("tmux session not found"),
			fmt.Errorf("tmux session not found"),
			fmt.Errorf("tmux session not found"),
		},
	}
	status, err := waitForCompletion(mock, 10*time.Second)
	if err != nil {
		t.Fatalf("expected nil error (session death detection), got: %v", err)
	}
	if status != "error" {
		t.Errorf("expected status 'error' for session death, got %q", status)
	}
}

func TestWaitForCompletion_TransientRecovery(t *testing.T) {
	// Fewer than 5 consecutive errors should recover when a valid status follows.
	mock := &mockStatusChecker{
		statuses: []string{"", "", "", "waiting"},
		errors:   []error{fmt.Errorf("tmux error"), fmt.Errorf("tmux error"), fmt.Errorf("tmux error"), nil},
	}
	status, err := waitForCompletion(mock, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "waiting" {
		t.Errorf("expected status 'waiting' after transient recovery, got %q", status)
	}
}

func TestWaitForCompletion_Timeout(t *testing.T) {
	mock := &mockStatusChecker{
		statuses: []string{"active"}, // Stays active forever
	}
	// Use a very short timeout so the test doesn't block
	_, err := waitForCompletion(mock, 2*time.Second)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

type mockSendRetryTarget struct {
	sendKeysErr error
	statuses    []string
	statusErrs  []error
	panes       []string
	paneErrs    []error

	statusIdx atomic.Int32
	paneIdx   atomic.Int32

	sendKeysCalls  int32
	sendEnterCalls int32
}

func (m *mockSendRetryTarget) SendKeysAndEnter(_ string) error {
	atomic.AddInt32(&m.sendKeysCalls, 1)
	return m.sendKeysErr
}

func (m *mockSendRetryTarget) GetStatus() (string, error) {
	i := int(m.statusIdx.Add(1) - 1)
	if len(m.statuses) == 0 {
		return "", nil
	}
	if i >= len(m.statuses) {
		i = len(m.statuses) - 1
	}
	var err error
	if i < len(m.statusErrs) {
		err = m.statusErrs[i]
	}
	return m.statuses[i], err
}

func (m *mockSendRetryTarget) SendEnter() error {
	atomic.AddInt32(&m.sendEnterCalls, 1)
	return nil
}

func (m *mockSendRetryTarget) CapturePaneFresh() (string, error) {
	i := int(m.paneIdx.Add(1) - 1)
	if len(m.panes) == 0 {
		return "", nil
	}
	if i >= len(m.panes) {
		i = len(m.panes) - 1
	}
	var err error
	if i < len(m.paneErrs) {
		err = m.paneErrs[i]
	}
	return m.panes[i], err
}

func TestSendWithRetryTarget_SkipVerify(t *testing.T) {
	mock := &mockSendRetryTarget{
		statuses: []string{"waiting"},
		panes:    []string{""},
	}
	err := sendWithRetryTarget(mock, "hello", true, sendRetryOptions{maxRetries: 4, checkDelay: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&mock.sendEnterCalls) != 0 {
		t.Fatalf("expected 0 SendEnter calls, got %d", mock.sendEnterCalls)
	}
}

func TestSendWithRetryTarget_StopsWhenActive(t *testing.T) {
	mock := &mockSendRetryTarget{
		statuses: []string{"active"},
		panes:    []string{""},
	}
	err := sendWithRetryTarget(mock, "hello", false, sendRetryOptions{maxRetries: 4, checkDelay: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&mock.sendEnterCalls) != 0 {
		t.Fatalf("expected 0 SendEnter calls, got %d", mock.sendEnterCalls)
	}
}

func TestSendWithRetryTarget_WaitingWithoutPasteMarkerReturnsSuccess(t *testing.T) {
	mock := &mockSendRetryTarget{
		statuses: []string{"waiting", "waiting", "waiting", "waiting"},
		panes:    []string{"", "", "", ""},
	}
	err := sendWithRetryTarget(mock, "hello", false, sendRetryOptions{maxRetries: 4, checkDelay: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With aggressive early retry (retry < 5), all 4 iterations nudge Enter.
	if got := atomic.LoadInt32(&mock.sendEnterCalls); got != 4 {
		t.Fatalf("expected 4 aggressive early SendEnter calls for waiting-without-active state, got %d", got)
	}
}

func TestSendWithRetryTarget_RetriesOnUnsentPasteMarker(t *testing.T) {
	mock := &mockSendRetryTarget{
		statuses: []string{"waiting", "waiting", "waiting", "waiting", "waiting"},
		panes: []string{
			"[Pasted text #1 +89 lines]",
			"[Pasted text #1 +89 lines]",
			"[Pasted text #1 +89 lines]",
			"[Pasted text #1 +89 lines]",
			"[Pasted text #1 +89 lines]",
		},
	}
	err := sendWithRetryTarget(mock, "hello", false, sendRetryOptions{maxRetries: 5, checkDelay: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&mock.sendEnterCalls); got != 5 {
		t.Fatalf("expected 5 SendEnter calls when unsent marker persists, got %d", got)
	}
}

func TestSendWithRetryTarget_DetectsPasteMarkerAfterInitialWaiting(t *testing.T) {
	mock := &mockSendRetryTarget{
		statuses: []string{"waiting", "waiting", "active"},
		panes: []string{
			"",
			"[Pasted text #1 +18 lines]",
			"",
		},
	}
	err := sendWithRetryTarget(mock, "hello", false, sendRetryOptions{maxRetries: 5, checkDelay: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 2 calls: retry 0 fires early aggressive nudge (waiting, no active seen),
	// retry 1 fires from paste marker detection.
	if got := atomic.LoadInt32(&mock.sendEnterCalls); got != 2 {
		t.Fatalf("expected 2 SendEnter calls (1 early nudge + 1 paste marker), got %d", got)
	}
}

func TestSendWithRetryTarget_RetriesWhenComposerPromptStillHasMessage(t *testing.T) {
	mock := &mockSendRetryTarget{
		statuses: []string{"waiting", "active"},
		panes: []string{
			"❯ Write one line: LAUNCH_OK",
			"",
		},
	}
	err := sendWithRetryTarget(mock, "Write one line: LAUNCH_OK", false, sendRetryOptions{maxRetries: 4, checkDelay: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&mock.sendEnterCalls); got != 1 {
		t.Fatalf("expected 1 SendEnter call when composer still has unsent message, got %d", got)
	}
}

func TestSendWithRetryTarget_RetriesWhenWrappedComposerPromptStillHasMessage(t *testing.T) {
	mock := &mockSendRetryTarget{
		statuses: []string{"waiting", "active"},
		panes: []string{
			"────────────────\n❯ Read these 3 files and produce a summary for DIAGTOKEN_123. Keep\n  under 80 lines.\n────────────────",
			"",
		},
	}
	message := "Read these 3 files and produce a summary for DIAGTOKEN_123. Keep under 80 lines."
	err := sendWithRetryTarget(mock, message, false, sendRetryOptions{maxRetries: 4, checkDelay: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&mock.sendEnterCalls); got != 1 {
		t.Fatalf("expected 1 SendEnter call when wrapped composer prompt still has unsent message, got %d", got)
	}
}

func TestSendWithRetryTarget_AmbiguousStateUsesLimitedFallbackRetries(t *testing.T) {
	mock := &mockSendRetryTarget{
		statuses: []string{"error", "error", "error", "error"},
		panes:    []string{"", "", "", ""},
	}
	err := sendWithRetryTarget(mock, "hello", false, sendRetryOptions{maxRetries: 4, checkDelay: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Ambiguous-state Enter budget increased from 2 to 4; all 4 retries send Enter.
	if got := atomic.LoadInt32(&mock.sendEnterCalls); got != 4 {
		t.Fatalf("expected 4 fallback SendEnter calls (increased budget), got %d", got)
	}
}

func TestSendWithRetryTarget_ReturnsErrorWhenInitialSendFails(t *testing.T) {
	mock := &mockSendRetryTarget{
		sendKeysErr: fmt.Errorf("tmux send failed"),
	}
	err := sendWithRetryTarget(mock, "hello", false, sendRetryOptions{maxRetries: 3, checkDelay: 0})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to send message") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSendWithRetryTarget_AggressiveEarlyEnterNudge(t *testing.T) {
	// Verify that SendEnter is called on every iteration for the first 5
	// retries when in waiting-without-active state, then every 2nd iteration.
	mock := &mockSendRetryTarget{
		statuses: []string{
			"waiting", "waiting", "waiting", "waiting", "waiting", // retries 0-4: all nudge
			"waiting", "waiting", "waiting", "waiting", "waiting", // retries 5-9: even nudge
		},
		panes: []string{"", "", "", "", "", "", "", "", "", ""},
	}
	err := sendWithRetryTarget(mock, "hello", false, sendRetryOptions{maxRetries: 10, checkDelay: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First 5 retries (0-4): all nudge = 5 calls
	// Retries 5-9: retry%2==0 means retries 6, 8 nudge = 2 calls
	// Total: 5 + 2 = 7
	// But wait: retry 5 is not < 5 and 5%2 != 0, so no nudge.
	// retry 6: 6%2 == 0, nudge. retry 7: no. retry 8: nudge. retry 9: no.
	// Total: 5 (early) + 2 (even from 5-9) = 7
	if got := atomic.LoadInt32(&mock.sendEnterCalls); got != 7 {
		t.Fatalf("expected 7 SendEnter calls (5 early + 2 even), got %d", got)
	}
}

func TestSendWithRetryTarget_IncreasedAmbiguousBudget(t *testing.T) {
	// Verify that ambiguous-state Enter budget is 4 (up from 2).
	mock := &mockSendRetryTarget{
		statuses: []string{"error", "error", "error", "error", "error"},
		panes:    []string{"", "", "", "", ""},
	}
	err := sendWithRetryTarget(mock, "hello", false, sendRetryOptions{maxRetries: 5, checkDelay: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Retries 0, 1, 2, 3 are < 4 so SendEnter is called 4 times; retry 4 is not.
	if got := atomic.LoadInt32(&mock.sendEnterCalls); got != 4 {
		t.Fatalf("expected 4 SendEnter calls for increased ambiguous budget, got %d", got)
	}
}

// Integration test coverage for Codex readiness: waitForAgentReady uses a
// concrete *tmux.Session so it cannot be unit tested with mocks here.
// See TestSend_CodexReadiness in internal/integration/send_reliability_test.go
// (Plan 02) for integration test coverage of Codex prompt gating.

// TestWaitOutputRetrieval_StaleSessionID verifies that --wait correctly
// retrieves output even when the initially-loaded ClaudeSessionID is stale.
// This simulates the bug where inst.GetLastResponse() fails because the
// session ID stored in the DB doesn't match the actual JSONL file on disk.
func TestWaitOutputRetrieval_StaleSessionID(t *testing.T) {
	// Set up a temp Claude config dir with a JSONL file
	tmpDir := t.TempDir()
	projectPath := "/test/wait-project"
	encodedPath := session.ConvertToClaudeDirName(projectPath)

	projectsDir := filepath.Join(tmpDir, "projects", encodedPath)
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		t.Fatalf("failed to create projects dir: %v", err)
	}

	// Override config dir for test
	origConfigDir := os.Getenv("CLAUDE_CONFIG_DIR")
	os.Setenv("CLAUDE_CONFIG_DIR", tmpDir)
	defer os.Setenv("CLAUDE_CONFIG_DIR", origConfigDir)
	session.ClearUserConfigCache()
	defer session.ClearUserConfigCache()

	// Create the "real" session JSONL file (what Claude actually wrote to)
	realSessionID := "real-session-id-after-start"
	realJSONL := filepath.Join(projectsDir, realSessionID+".jsonl")
	jsonlContent := `{"type":"summary","sessionId":"` + realSessionID + `"}
{"message":{"role":"user","content":"hello"},"sessionId":"` + realSessionID + `","type":"user","timestamp":"2026-01-01T00:00:00Z"}
{"message":{"role":"assistant","content":[{"type":"text","text":"Hello! How can I help?"}]},"sessionId":"` + realSessionID + `","type":"assistant","timestamp":"2026-01-01T00:00:01Z"}`
	if err := os.WriteFile(realJSONL, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("failed to write JSONL: %v", err)
	}

	t.Run("stale session ID fails to find file", func(t *testing.T) {
		// Instance with stale session ID (doesn't match any JSONL file)
		inst := session.NewInstance("wait-test", projectPath)
		inst.Tool = "claude"
		inst.ClaudeSessionID = "stale-old-session-id"

		_, err := inst.GetLastResponse()
		if err == nil {
			t.Fatal("expected error with stale session ID, got nil")
		}
	})

	t.Run("correct session ID finds file", func(t *testing.T) {
		// Instance with correct session ID
		inst := session.NewInstance("wait-test", projectPath)
		inst.Tool = "claude"
		inst.ClaudeSessionID = realSessionID

		resp, err := inst.GetLastResponse()
		if err != nil {
			t.Fatalf("unexpected error with correct session ID: %v", err)
		}
		if resp.Content != "Hello! How can I help?" {
			t.Errorf("expected 'Hello! How can I help?', got %q", resp.Content)
		}
	})

	t.Run("refreshing session ID fixes retrieval", func(t *testing.T) {
		// Simulates the --wait fix: start with stale ID, then refresh
		inst := session.NewInstance("wait-test", projectPath)
		inst.Tool = "claude"
		inst.ClaudeSessionID = "stale-old-session-id"

		// First attempt fails (stale ID)
		_, err := inst.GetLastResponse()
		if err == nil {
			t.Fatal("expected error with stale session ID")
		}

		// Simulate refreshing session ID (as the fix does from tmux env)
		inst.ClaudeSessionID = realSessionID
		inst.ClaudeDetectedAt = time.Now()

		// Second attempt succeeds with refreshed ID
		resp, err := inst.GetLastResponse()
		if err != nil {
			t.Fatalf("unexpected error after refresh: %v", err)
		}
		if resp.Content != "Hello! How can I help?" {
			t.Errorf("expected 'Hello! How can I help?', got %q", resp.Content)
		}
	})
}
