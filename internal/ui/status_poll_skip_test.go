package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/session"
	"github.com/asheshgoplani/agent-deck/internal/tmux"
)

// Perf: the per-tick background status loop walks every loaded instance and
// runs UpdateStatus() (a tmux subprocess). Archived sessions have had their
// tmux pane torn down and their row status is display-frozen (rowStatusGlyph
// forces the stopped glyph regardless of Status), so polling them can never
// change anything the UI shows — it only burns a serialized tmux call. With a
// large archive backlog (observed: 723 archived of 742 total) this dominated
// the loop and pushed it to multi-second spikes. shouldPollStatusInLoop pins
// the contract that archived sessions are skipped and active ones are not.
func TestShouldPollStatusInLoop_SkipsArchived(t *testing.T) {
	active := &session.Instance{ID: "a", Title: "active"}
	archived := &session.Instance{ID: "b", Title: "archived", ArchivedAt: time.Now().UTC()}

	if !shouldPollStatusInLoop(active) {
		t.Fatalf("active session must be polled")
	}
	if shouldPollStatusInLoop(archived) {
		t.Fatalf("archived session must be skipped (no live pane, frozen status)")
	}
	if shouldPollStatusInLoop(nil) {
		t.Fatalf("nil instance must not be polled")
	}
}

// A quiet control pipe is not proof of idle: the last working sample still
// needs the busy grace and prompt/Instance confirmation samples to expire.
// Drive the real sweep, pipe, pane classifier, and render snapshot. TestMain
// isolates HOME and tmux; the fixture never launches a real agent or sends keys.
func TestBackgroundStatusUpdate_SilentCodexCompletionConverges(t *testing.T) {
	skipIfNoTmuxBinaryUI(t)
	for _, completion := range []string{"idle", "approval"} {
		t.Run(completion, func(t *testing.T) {
			dir := t.TempDir()
			modePath := filepath.Join(dir, "mode")
			scriptPath := filepath.Join(dir, "codex")
			const script = `last=
while :; do
  IFS= read -r mode < "$1" || mode=
  if [ -n "$mode" ] && [ "$mode" != "$last" ]; then
    printf '\033[2J\033[H\033]0;⠏ project | Starting | Task\007'
    case "$mode" in
      working)
        printf '%s\n' '• Working (10s • esc to interrupt)' '  └ Tip: Paste an image with Ctrl+V to attach it to your next message.' '' '' '› Ask Codex to do anything' '' '  model · ~/project · Task' '  ← for agents · ? for shortcuts'
        ;;
      approval)
        printf '%s\n' '• Running curl https://example.com' '' '  Would you like to run the following command?' '' '  $ curl https://example.com' '' '› 1. Yes, proceed (y)' "  2. Yes, and don't ask again for commands that start with curl (p)" '  3. No, and tell Codex what to do differently (esc)'
        ;;
      idle)
        printf '%s\n' '• Finished the requested work.' '' '› Ask Codex to do anything' '' '  model · ~/project · Task' '  ← for agents · ? for shortcuts'
        ;;
    esac
    last="$mode"
  fi
  sleep 0.05
done
`
			if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
				t.Fatal(err)
			}
			setMode := func(mode string) {
				t.Helper()
				if err := os.WriteFile(modePath, []byte(mode+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			await := func(description string, condition func() bool) {
				t.Helper()
				deadline := time.Now().Add(15 * time.Second)
				for time.Now().Before(deadline) {
					if condition() {
						return
					}
					time.Sleep(25 * time.Millisecond)
				}
				t.Fatalf("timed out waiting for %s", description)
			}
			setMode("idle")
			sess := tmux.NewSession("test-silent-codex", dir)
			sess.SocketName = tmux.DefaultSocketName()
			sess.RunCommandAsInitialProcess = true
			t.Cleanup(func() { _ = sess.Kill() })
			sess.SetInjectStatusLine(false)
			if err := sess.Start(fmt.Sprintf("bash %q %q", scriptPath, modePath)); err != nil {
				t.Fatal(err)
			}
			tmux.ExpireStartupWindowForTest(t, sess)
			sess.EnsureConfigured()

			pm := tmux.NewPipeManager(context.Background(), nil)
			previousPM := tmux.GetPipeManager()
			tmux.SetPipeManager(pm)
			t.Cleanup(func() {
				pm.Close()
				tmux.SetPipeManager(previousPM)
			})
			if err := pm.Connect(sess.Name, sess.SocketName); err != nil {
				t.Fatal(err)
			}
			await("connected private tmux server", func() bool {
				return pm.IsConnected(sess.Name) && tmux.IsServerAlive()
			})

			inst := session.NewInstance("silent-codex", dir)
			inst.Tool = "codex"
			inst.CreatedAt = time.Now().Add(-time.Hour)
			inst.SetTmuxSessionForTest(sess)
			h := &Home{
				instances:                 []*session.Instance{inst},
				lastPersistedStatus:       make(map[string]string),
				lastPersistedAutoNameDesc: make(map[string]string),
			}
			setMode("working")
			await("working pane frame", func() bool {
				content, err := sess.CapturePaneFresh()
				return err == nil && strings.Contains(content, "• Working")
			})
			await("working status from native pane", func() bool {
				h.backgroundStatusUpdate()
				return inst.GetStatusThreadSafe() == session.StatusRunning
			})
			setMode(completion)
			await("completed pane frame", func() bool {
				content, err := sess.CapturePaneFresh()
				return err == nil && !strings.Contains(content, "• Working") &&
					strings.Contains(content, "› ")
			})
			// Wait on actual output silence, not a guessed worker delay. Six
			// seconds is the existing spinner grace; no status consumer runs
			// during this wait to finish the transition on the worker's behalf.
			await("existing busy grace after final output", func() bool {
				lastOutput := pm.LastOutputTime(sess.Name)
				return !lastOutput.IsZero() && time.Since(lastOutput) > 6*time.Second
			})
			for range 4 {
				h.backgroundStatusUpdate()
			}
			if got := inst.GetStatusThreadSafe(); got != session.StatusWaiting {
				t.Fatalf("silent %s remained %q after confirmation sweeps, want waiting", completion, got)
			}
			if state, ok := h.getSessionRenderSnapshot()[inst.ID]; !ok || state.status != session.StatusWaiting {
				t.Fatalf("render snapshot present=%v status=%q, want waiting", ok, state.status)
			}
			if !h.acknowledgeViewedSession(inst) {
				t.Fatal("completed session was not acknowledged")
			}
			h.backgroundStatusUpdate()
			if state, ok := h.getSessionRenderSnapshot()[inst.ID]; !ok || state.status != session.StatusIdle {
				t.Fatalf("acknowledged render snapshot present=%v status=%q, want idle", ok, state.status)
			}
		})
	}
}
