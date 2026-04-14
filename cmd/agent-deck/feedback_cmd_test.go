package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/feedback"
)

// mockSender creates a *feedback.Sender with injectable GhCmd for testing.
// sendCalled is set to true if GhCmd is invoked (i.e. Send() reaches the gh step).
func mockSender(sendCalled *bool) *feedback.Sender {
	return &feedback.Sender{
		GhCmd: func(args ...string) error {
			*sendCalled = true
			return nil
		},
		BrowserCmd:     func(url string) error { return nil },
		ClipboardCmd:   func(text string) error { return nil },
		IsHeadlessFunc: func() bool { return true },
	}
}

// pipeInput creates an os.Pipe, writes lines to it, and returns the read end.
// The caller is responsible for restoring os.Stdin after the test.
func pipeInput(t *testing.T, lines ...string) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	go func() {
		defer w.Close()
		for _, line := range lines {
			fmt.Fprintln(w, line)
		}
	}()
	return r
}

// TestHandleFeedback_ValidRating verifies that piping "4\n\n" causes handleFeedbackWithSender
// to call sender.Send() (i.e. sendCalled becomes true) and exits cleanly.
func TestHandleFeedback_ValidRating(t *testing.T) {
	// Save original stdin and restore after test
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	var sendCalled bool
	sender := mockSender(&sendCalled)

	// Pipe: rating="4", comment="" (empty line to skip)
	os.Stdin = pipeInput(t, "4", "")

	var stdout strings.Builder
	err := handleFeedbackWithSender([]string{}, "1.5.1", sender, &stdout)
	if err != nil {
		t.Fatalf("handleFeedbackWithSender returned unexpected error: %v", err)
	}
	if !sendCalled {
		t.Error("expected sender.Send() to be called after valid rating, but it was not")
	}
}

// TestHandleFeedback_OptOut verifies that piping "n\n" causes handleFeedbackWithSender
// to call RecordOptOut and save state (FeedbackEnabled=false), and NOT call sender.Send().
func TestHandleFeedback_OptOut(t *testing.T) {
	// Save original stdin and restore after test
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	var sendCalled bool
	sender := mockSender(&sendCalled)

	// Pipe: opt-out with "n"
	os.Stdin = pipeInput(t, "n")

	var stdout strings.Builder
	err := handleFeedbackWithSender([]string{}, "1.5.1", sender, &stdout)
	if err != nil {
		t.Fatalf("handleFeedbackWithSender returned unexpected error: %v", err)
	}
	if sendCalled {
		t.Error("expected sender.Send() NOT to be called on opt-out, but it was")
	}

	// Verify FeedbackEnabled is false in saved state
	st, loadErr := feedback.LoadState()
	if loadErr != nil {
		t.Fatalf("feedback.LoadState() failed after opt-out: %v", loadErr)
	}
	if st.FeedbackEnabled {
		t.Error("expected FeedbackEnabled=false after opt-out, but it is still true")
	}
}
