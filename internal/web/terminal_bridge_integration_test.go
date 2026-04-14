//go:build !windows

package web

import (
	"fmt"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestTmuxPTYBridgeResize(t *testing.T) {
	requireTmuxForWebIntegration(t)

	sessionName := fmt.Sprintf("agentdeck_web_resize_%d", time.Now().UnixNano())
	if output, err := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-x", "80", "-y", "24").CombinedOutput(); err != nil {
		t.Skipf("tmux new-session unavailable: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	defer func() {
		_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	}()

	srv := NewServer(Config{
		ListenAddr: "127.0.0.1:0",
		Profile:    "work",
	})
	srv.menuData = &fakeMenuDataLoader{
		snapshot: &MenuSnapshot{
			Profile: "work",
			Items: []MenuItem{
				{
					Type: MenuItemTypeSession,
					Session: &MenuSession{
						ID:          "sess-resize",
						TmuxSession: sessionName,
					},
				},
			},
		},
	}

	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(testServer.URL, "/ws/session/sess-resize"), nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("dial failed with status %d: %v", resp.StatusCode, err)
		}
		t.Fatalf("dial failed: %v", err)
	}
	defer func() {
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(200*time.Millisecond),
		)
		_ = conn.Close()
	}()

	waitForStatusOrSkipOnAttachFailure(t, conn, "terminal_attached")

	if err := conn.WriteJSON(wsClientMessage{Type: "resize", Cols: 120, Rows: 40}); err != nil {
		t.Fatalf("failed to send resize message: %v", err)
	}

	// Poll for up to 2 seconds to handle async tmux propagation.
	var got string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		out, err := exec.Command("tmux", "display-message", "-t", sessionName, "-p", "#{window_width}x#{window_height}").Output()
		if err != nil {
			t.Fatalf("tmux display-message failed: %v", err)
		}
		got = strings.TrimSpace(string(out))
		if got == "120x40" {
			return // PASS
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("tmux window size after Resize: got %q, want %q", got, "120x40")
}
