package session

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Task 1: Session start and stop tests (TEST-03, TEST-04) ---

// TestSessionStart_CreatesTmuxSession verifies that Start() creates a real tmux
// session that is detectable both via Exists() and raw `tmux has-session`.
func TestSessionStart_CreatesTmuxSession(t *testing.T) {
	skipIfNoTmuxServer(t)

	inst := NewInstance("test-start-creates", "/tmp")
	inst.Command = "sleep 60"

	err := inst.Start()
	require.NoError(t, err, "Start() should succeed")
	defer func() { _ = inst.Kill() }()

	// Verify via Instance.Exists()
	assert.True(t, inst.Exists(), "Exists() should return true after Start()")

	// Verify tmux session object is available
	tmuxSess := inst.GetTmuxSession()
	require.NotNil(t, tmuxSess, "GetTmuxSession() should not be nil after Start()")
	assert.NotEmpty(t, tmuxSess.Name, "tmux session name should be non-empty")

	// Verify via raw tmux has-session command (independent verification)
	err = exec.Command("tmux", "has-session", "-t", tmuxSess.Name).Run()
	assert.NoError(t, err, "tmux has-session should succeed for the started session")
}

// TestSessionStart_SetsStartingStatus verifies that Start() sets the status to
// StatusStarting when a command is provided.
func TestSessionStart_SetsStartingStatus(t *testing.T) {
	skipIfNoTmuxServer(t)

	inst := NewInstance("test-start-status", "/tmp")
	inst.Command = "sleep 60"

	err := inst.Start()
	require.NoError(t, err, "Start() should succeed")
	defer func() { _ = inst.Kill() }()

	// Immediately after Start(), status should be StatusStarting (before grace period)
	assert.Equal(t, StatusStarting, inst.Status,
		"Status should be StatusStarting immediately after Start() with a command")
}

// TestSessionStop_KillsAndSetsError verifies that Kill() terminates the tmux
// session and sets Status to StatusError.
func TestSessionStop_KillsAndSetsError(t *testing.T) {
	skipIfNoTmuxServer(t)

	inst := NewInstance("test-stop-kills", "/tmp")
	inst.Command = "sleep 60"

	err := inst.Start()
	require.NoError(t, err, "Start() should succeed")

	// Verify session exists before kill
	tmuxName := inst.GetTmuxSession().Name
	require.True(t, inst.Exists(), "session should exist before Kill()")

	err = inst.Kill()
	require.NoError(t, err, "Kill() should succeed")

	// Verify status is error
	assert.Equal(t, StatusError, inst.Status,
		"Status should be StatusError after Kill()")

	// Verify Exists() returns false
	assert.False(t, inst.Exists(), "Exists() should return false after Kill()")

	// Verify via raw tmux has-session (session should be gone)
	err = exec.Command("tmux", "has-session", "-t", tmuxName).Run()
	assert.Error(t, err, "tmux has-session should fail after Kill()")
}

// TestSessionStop_DoubleKill verifies that calling Kill() twice does not panic.
// The second call may return an error (tmux session already gone), which is acceptable.
func TestSessionStop_DoubleKill(t *testing.T) {
	skipIfNoTmuxServer(t)

	inst := NewInstance("test-stop-double", "/tmp")
	inst.Command = "sleep 60"

	err := inst.Start()
	require.NoError(t, err, "Start() should succeed")

	// First kill
	err = inst.Kill()
	require.NoError(t, err, "First Kill() should succeed")

	// Second kill should not panic (error is acceptable)
	assert.NotPanics(t, func() {
		_ = inst.Kill()
	}, "Second Kill() should not panic")
}

// TestSessionStop_UpdateStatusAfterKill verifies that UpdateStatus() reports
// StatusError after the session has been killed.
func TestSessionStop_UpdateStatusAfterKill(t *testing.T) {
	skipIfNoTmuxServer(t)

	inst := NewInstance("test-stop-update", "/tmp")
	inst.Command = "sleep 60"

	err := inst.Start()
	require.NoError(t, err, "Start() should succeed")

	err = inst.Kill()
	require.NoError(t, err, "Kill() should succeed")

	// Wait past any grace period (1.5s) so UpdateStatus does a real check
	time.Sleep(2 * time.Second)

	err = inst.UpdateStatus()
	require.NoError(t, err, "UpdateStatus() should not error")

	assert.Equal(t, StatusError, inst.Status,
		"UpdateStatus() should report StatusError after Kill()")
}

// TestSessionStart_NilTmuxSession verifies that Start() on a bare Instance
// without tmux initialization returns an appropriate error.
func TestSessionStart_NilTmuxSession(t *testing.T) {
	// Create a bare instance without tmux session (no NewInstance)
	inst := &Instance{}

	err := inst.Start()
	require.Error(t, err, "Start() should fail without tmux session")
	assert.Contains(t, err.Error(), "tmux session not initialized",
		"error should mention tmux session not initialized")
}
