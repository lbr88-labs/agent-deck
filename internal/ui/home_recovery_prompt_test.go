package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

// recoveryProbeInstance builds an instance the fleet detector will classify
// as down: no tmux session is attached and the title matches no real tmux
// session on the host. The pid suffix keeps the probe title unique even on a
// dev machine running agent-deck. Whether it is PROPOSED is decided by the
// liveness stamp the caller adds via MarkSeenAlive.
func recoveryProbeInstance(id, title string, status session.Status) *session.Instance {
	return &session.Instance{
		ID:     id,
		Title:  fmt.Sprintf("%s-%d", title, os.Getpid()),
		Tool:   "claude",
		Status: status,
	}
}

func newRecoveryTestHome(t *testing.T, instances []*session.Instance) *Home {
	t.Helper()
	home := NewHome()
	home.width = 120
	home.height = 40
	home.initialLoading = false
	home.groupTree = session.NewGroupTree(instances)
	return home
}

func TestStartupRecoveryPrompt_ShowsForRecentlyAliveDeadSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	session.ClearUserConfigCache()
	t.Cleanup(session.ClearUserConfigCache)

	dead := recoveryProbeInstance("dead-1", "zz-adeck-crash-dead", session.StatusWaiting)
	dead.MarkSeenAlive(time.Now().Add(-2 * time.Minute))
	stopped := recoveryProbeInstance("stopped-1", "zz-adeck-crash-stopped", session.StatusStopped)
	stopped.MarkSeenAlive(time.Now().Add(-30 * time.Minute))
	stopped.ClearSeenAlive() // operator stop erased the evidence
	instances := []*session.Instance{dead, stopped}
	home := newRecoveryTestHome(t, instances)

	// First session load after launch: the one-shot scan must propose the
	// session whose liveness stamp is still fresh, and exclude the one the
	// operator deliberately stopped (stamp cleared) — whatever its status
	// column says.
	model, _ := home.Update(loadSessionsMsg{instances: instances, loadMtime: time.Now()})
	h := model.(*Home)

	if !h.recoveryPrompt.IsVisible() {
		t.Fatal("startup recovery prompt should show for a recently-alive session whose pane is gone")
	}
	got := h.recoveryPrompt.SelectedAssessment()
	if got.Down != 1 || len(got.Candidates) != 1 || got.Candidates[0].ID() != "dead-1" {
		t.Fatalf("only the stamped dead session should be a candidate, got down=%d candidates=%v", got.Down, got.Candidates)
	}

	// Enter hides the prompt and hands back the sweep command. The command is
	// deliberately NOT executed here: it would boot real sessions.
	model, cmd := h.handleRecoveryPromptKey(tea.KeyMsg{Type: tea.KeyEnter})
	h = model.(*Home)
	if h.recoveryPrompt.IsVisible() {
		t.Fatal("enter should hide the recovery prompt")
	}
	if cmd == nil {
		t.Fatal("enter should return the recovery sweep command")
	}

	// The scan is one-shot per launch: a later reload must not re-show it.
	model, _ = h.Update(loadSessionsMsg{instances: instances, loadMtime: time.Now()})
	h = model.(*Home)
	if h.recoveryPrompt.IsVisible() {
		t.Fatal("recovery prompt must be proposed at most once per launch")
	}
}

func TestStartupRecoveryPrompt_StaleStampExcluded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	session.ClearUserConfigCache()
	t.Cleanup(session.ClearUserConfigCache)

	// The backbone of the v2 fix: a session that died weeks ago still reads
	// as error in the registry, but its liveness stamp stopped advancing at
	// power-off. Outside the window it must NOT be proposed — this is what
	// keeps the 80-row error backlog from firing the prompt on every launch.
	stale := recoveryProbeInstance("stale-1", "zz-adeck-crash-stale", session.StatusError)
	stale.MarkSeenAlive(time.Now().Add(-8 * 24 * time.Hour))
	instances := []*session.Instance{stale}
	home := newRecoveryTestHome(t, instances)

	model, _ := home.Update(loadSessionsMsg{instances: instances, loadMtime: time.Now()})
	h := model.(*Home)
	if h.recoveryPrompt.IsVisible() {
		t.Fatal("a session last seen alive 8 days ago must not be proposed as open before the restart")
	}
}

func TestStartupRecoveryPrompt_FreshIdleShellIncluded(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	session.ClearUserConfigCache()
	t.Cleanup(session.ClearUserConfigCache)

	// A shell the user left open parks at status idle. It is still a session
	// they had open when the machine died, and its pane was stamped alive —
	// so it must be proposed, not silently skipped the way status-only
	// detection skipped it.
	shell := recoveryProbeInstance("shell-1", "zz-adeck-crash-shell", session.StatusIdle)
	shell.MarkSeenAlive(time.Now().Add(-1 * time.Minute))
	instances := []*session.Instance{shell}
	home := newRecoveryTestHome(t, instances)

	model, _ := home.Update(loadSessionsMsg{instances: instances, loadMtime: time.Now()})
	h := model.(*Home)
	if !h.recoveryPrompt.IsVisible() {
		t.Fatal("an idle shell with a fresh liveness stamp was open before the restart and must be proposed")
	}
	if got := h.recoveryPrompt.SelectedAssessment(); got.Down != 1 || got.Candidates[0].ID() != "shell-1" {
		t.Fatalf("expected exactly the idle shell, got down=%d candidates=%v", got.Down, got.Candidates)
	}
}

func TestStartupRecoveryPrompt_NoPromptWhenNothingDead(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	session.ClearUserConfigCache()
	t.Cleanup(session.ClearUserConfigCache)

	// Only operator-stopped sessions: never recovery candidates.
	stopped := recoveryProbeInstance("stopped-1", "zz-adeck-crash-only", session.StatusStopped)
	instances := []*session.Instance{stopped}
	home := newRecoveryTestHome(t, instances)

	model, _ := home.Update(loadSessionsMsg{instances: instances, loadMtime: time.Now()})
	h := model.(*Home)
	if h.recoveryPrompt.IsVisible() {
		t.Fatal("no prompt should appear when every session is intentionally stopped")
	}
}

func TestStartupRecoveryPrompt_OptOutViaConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, ".config"))
	session.ClearUserConfigCache()
	t.Cleanup(session.ClearUserConfigCache)

	cfgDir := filepath.Join(tmpHome, ".config", "agent-deck")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	cfg := "[recovery]\n  propose_on_startup = false\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	dead := recoveryProbeInstance("dead-1", "zz-adeck-crash-optout", session.StatusRunning)
	dead.MarkSeenAlive(time.Now().Add(-1 * time.Minute))
	instances := []*session.Instance{dead}
	home := newRecoveryTestHome(t, instances)

	model, _ := home.Update(loadSessionsMsg{instances: instances, loadMtime: time.Now()})
	h := model.(*Home)
	if h.recoveryPrompt.IsVisible() {
		t.Fatal("propose_on_startup=false must suppress the startup recovery prompt")
	}
}
