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
// as down: status claims liveness (running) but no tmux session is attached
// and the title matches no real tmux session on the host. The pid suffix
// keeps the probe title unique even on a dev machine running agent-deck.
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

func TestStartupRecoveryPrompt_ShowsForDeadRunningSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	session.ClearUserConfigCache()
	t.Cleanup(session.ClearUserConfigCache)

	dead := recoveryProbeInstance("dead-1", "zz-adeck-crash-dead", session.StatusRunning)
	stopped := recoveryProbeInstance("stopped-1", "zz-adeck-crash-stopped", session.StatusStopped)
	instances := []*session.Instance{dead, stopped}
	home := newRecoveryTestHome(t, instances)

	// First session load after launch: the one-shot scan must notice the
	// running session whose tmux session is gone and propose recovery.
	model, _ := home.Update(loadSessionsMsg{instances: instances, loadMtime: time.Now()})
	h := model.(*Home)

	if !h.recoveryPrompt.IsVisible() {
		t.Fatal("startup recovery prompt should show when a should-be-alive session has no tmux session")
	}
	got := h.recoveryPrompt.SelectedAssessment()
	if got.Down != 1 || len(got.Candidates) != 1 || got.Candidates[0].ID() != "dead-1" {
		t.Fatalf("only the dead session should be a candidate, got down=%d candidates=%v", got.Down, got.Candidates)
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
	instances := []*session.Instance{dead}
	home := newRecoveryTestHome(t, instances)

	model, _ := home.Update(loadSessionsMsg{instances: instances, loadMtime: time.Now()})
	h := model.(*Home)
	if h.recoveryPrompt.IsVisible() {
		t.Fatal("propose_on_startup=false must suppress the startup recovery prompt")
	}
}
