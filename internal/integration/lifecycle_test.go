package integration

import (
	"os/exec"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLifecycleStart_CreatesRealSession verifies that starting a session through
// TmuxHarness creates a real tmux session with observable pane content. (LIFE-01)
func TestLifecycleStart_CreatesRealSession(t *testing.T) {
	h := NewTmuxHarness(t)

	inst := h.CreateSession("start-real", "/tmp")
	inst.Command = "echo hello && sleep 60"
	require.NoError(t, inst.Start())

	assert.True(t, inst.Exists(), "session should exist after Start()")
	WaitForPaneContent(t, inst, "hello", 5*time.Second)
	assert.NotEmpty(t, inst.GetTmuxSession().Name, "tmux session name should not be empty")
}

// TestLifecycleStart_StatusTransition verifies that Start() sets StatusStarting
// immediately and the tmux session becomes reachable shortly after. (LIFE-01)
func TestLifecycleStart_StatusTransition(t *testing.T) {
	h := NewTmuxHarness(t)

	inst := h.CreateSession("start-status", "/tmp")
	inst.Command = "sleep 60"
	require.NoError(t, inst.Start())

	assert.Equal(t, session.StatusStarting, inst.Status, "status should be starting immediately after Start()")

	WaitForCondition(t, 5*time.Second, 200*time.Millisecond,
		"tmux session to exist",
		func() bool { return inst.Exists() })
}

// TestLifecycleStop_TerminatesSession verifies that Kill() terminates the tmux
// session and sets StatusError. (LIFE-02)
func TestLifecycleStop_TerminatesSession(t *testing.T) {
	h := NewTmuxHarness(t)

	inst := h.CreateSession("stop-term", "/tmp")
	inst.Command = "sleep 60"
	require.NoError(t, inst.Start())

	WaitForCondition(t, 5*time.Second, 200*time.Millisecond,
		"session to exist after start",
		func() bool { return inst.Exists() })

	tmuxName := inst.GetTmuxSession().Name
	require.NotEmpty(t, tmuxName, "tmux session name must be set before Kill()")

	require.NoError(t, inst.Kill())
	assert.Equal(t, session.StatusError, inst.Status, "status should be error after Kill()")

	WaitForCondition(t, 3*time.Second, 200*time.Millisecond,
		"session to not exist",
		func() bool { return !inst.Exists() })

	// Verify at the tmux level that the session is gone.
	err := exec.Command("tmux", "has-session", "-t", tmuxName).Run()
	assert.Error(t, err, "tmux has-session should fail for killed session")
}

// TestLifecycleStop_PaneContentGoneAfterKill verifies that pane content is no
// longer accessible after the session is killed. (LIFE-02)
func TestLifecycleStop_PaneContentGoneAfterKill(t *testing.T) {
	h := NewTmuxHarness(t)

	inst := h.CreateSession("stop-pane", "/tmp")
	inst.Command = "echo marker_abc && sleep 60"
	require.NoError(t, inst.Start())

	WaitForPaneContent(t, inst, "marker_abc", 5*time.Second)

	tmuxName := inst.GetTmuxSession().Name
	require.NotEmpty(t, tmuxName)

	require.NoError(t, inst.Kill())

	// Verify the tmux session is gone.
	WaitForCondition(t, 3*time.Second, 200*time.Millisecond,
		"tmux session to be gone",
		func() bool {
			return exec.Command("tmux", "has-session", "-t", tmuxName).Run() != nil
		})
}
