package main

import (
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

func TestStandaloneWaitFresh(t *testing.T) {
	dispatch := time.Now().UTC()
	for _, tc := range []struct {
		name, role, stamp, content string
		fresh                      bool
	}{
		{"older reply", "assistant", dispatch.Add(-time.Nanosecond).Format(time.RFC3339Nano), "answer", false},
		{"same time", "assistant", dispatch.Format(time.RFC3339Nano), "answer", false},
		{"terminal repaint", "assistant", "", "busy", false},
		{"echo", "user", dispatch.Add(time.Second).Format(time.RFC3339Nano), "question", false},
		{"already completed", "assistant", dispatch.Add(time.Millisecond).Format(time.RFC3339Nano), "answer", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &session.ResponseOutput{Role: tc.role, Timestamp: tc.stamp, Content: tc.content}
			if waitFresh(dispatch, r) != tc.fresh {
				t.Fatalf("freshness mismatch for %+v", r)
			}
		})
	}
}

func TestWaitForSavedReplyOutcomes(t *testing.T) {
	since := time.Now()
	for _, tc := range []struct {
		name, state string
		fresh       bool
		want        string
		code        int
	}{
		{"completed before polling", "waiting", true, "completed", 0},
		{"completed before stop", "inactive", true, "completed", 0},
		{"stopped without reply", "inactive", false, "stopped", 3},
		{"stale idle reply", "waiting", false, "timeout", 4},
		{"active prior text", "active", true, "timeout", 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stamp := since
			if tc.fresh {
				stamp = stamp.Add(time.Millisecond)
			}
			got, _, code := waitForSavedReply(since, 5*time.Millisecond, time.Millisecond, func() (string, error) { return tc.state, nil }, func() (*session.ResponseOutput, error) {
				return &session.ResponseOutput{Role: "assistant", Content: "answer", Timestamp: stamp.Format(time.RFC3339Nano)}, nil
			})
			if got != tc.want || code != tc.code {
				t.Fatalf("got %s (%d), want %s (%d)", got, code, tc.want, tc.code)
			}
		})
	}
}

type waitCaptureTarget struct {
	mockSendRetryTarget
	lastCapture time.Time
}

func (t *waitCaptureTarget) CapturePaneFresh() (string, error) {
	if t.sendKeysCalls == 0 {
		t.lastCapture = time.Now()
	}
	return t.mockSendRetryTarget.CapturePaneFresh()
}

func TestWaitCursorAfterComposerPreflight(t *testing.T) {
	target := &waitCaptureTarget{mockSendRetryTarget: mockSendRetryTarget{panes: []string{claudeComposer("")}, statuses: []string{"active"}}}
	tun := testGuardTuning(sendRetryOptions{maxRetries: 1, verifyDelivery: true})
	res, err := executeSend(target, "claude", "test", false, tun)
	if err != nil {
		t.Fatal(err)
	}
	if target.lastCapture.IsZero() || res.submittedAt.Before(target.lastCapture) {
		t.Fatalf("cursor %s predates preflight %s", res.submittedAt, target.lastCapture)
	}
}
