package session

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Task 1: Status Transition Cycle Tests (TEST-01)
// =============================================================================

// TestStatusCycle_ShellSessionWithCommand verifies the full lifecycle:
// StatusStarting (after Start with command) -> running/idle (after grace period + UpdateStatus) -> StatusError (after Kill)
func TestStatusCycle_ShellSessionWithCommand(t *testing.T) {
	skipIfNoTmuxServer(t)

	inst := NewInstance("test-lifecycle-cmd", "/tmp")
	inst.Tool = "shell"
	inst.Command = "sleep 30"

	// Before Start: should be idle (default)
	require.Equal(t, StatusIdle, inst.Status, "before Start() status should be idle")

	err := inst.Start()
	require.NoError(t, err, "Start() should succeed")
	defer func() { _ = inst.Kill() }()

	// After Start with command: should be StatusStarting
	assert.Equal(t, StatusStarting, inst.Status, "after Start() with command, status should be starting")

	// Wait past the 1.5s grace period
	time.Sleep(2 * time.Second)

	err = inst.UpdateStatus()
	require.NoError(t, err, "UpdateStatus() should succeed")

	// After grace period, status should NOT be starting
	assert.NotEqual(t, StatusStarting, inst.Status, "after grace period, status should not be starting")
	// For a shell session running sleep, it should be idle (shell sessions map "waiting" to idle)
	assert.Equal(t, StatusIdle, inst.Status, "shell session running sleep should show as idle")

	// Kill the session
	err = inst.Kill()
	require.NoError(t, err, "Kill() should succeed")

	// After Kill: should be StatusError
	assert.Equal(t, StatusError, inst.Status, "after Kill() status should be error")

	// Tmux session should no longer exist
	assert.False(t, inst.Exists(), "tmux session should not exist after Kill()")
}

// TestStatusCycle_ShellSessionNoCommand verifies that a shell session without
// a command does NOT get StatusStarting from Start() (unlike sessions with commands).
// After Start(), the status remains StatusIdle. UpdateStatus() then reflects the
// actual tmux state which may be "starting" during the 2-minute startup window
// or "waiting"/"idle" depending on prompt detection.
func TestStatusCycle_ShellSessionNoCommand(t *testing.T) {
	skipIfNoTmuxServer(t)

	inst := NewInstance("test-lifecycle-nocmd", "/tmp")
	inst.Tool = "shell"
	// No command set

	err := inst.Start()
	require.NoError(t, err, "Start() should succeed")
	defer func() { _ = inst.Kill() }()

	// Key contract: Without a command, Start() does NOT set StatusStarting.
	// Status should remain StatusIdle (the default from NewInstance).
	assert.Equal(t, StatusIdle, inst.Status, "shell session without command should stay idle after Start()")

	// Wait past the 1.5s instance-level grace period
	time.Sleep(2 * time.Second)

	err = inst.UpdateStatus()
	require.NoError(t, err, "UpdateStatus() should succeed")

	// After UpdateStatus, the tmux layer takes over status detection.
	// The session exists and is valid, so it should NOT be StatusError.
	assert.NotEqual(t, StatusError, inst.Status,
		"shell session without command should not be in error state")
	// The status will be determined by tmux content analysis (starting, idle, or waiting).
	// During the 2-minute tmux startup window, "starting" is expected for new sessions
	// without detectable prompt patterns.
	t.Logf("Status after UpdateStatus(): %s (tmux-determined)", inst.Status)
}

// TestStatusCycle_KilledExternally verifies that when a tmux session is killed
// externally (not via inst.Kill()), UpdateStatus detects this and sets StatusError.
func TestStatusCycle_KilledExternally(t *testing.T) {
	skipIfNoTmuxServer(t)

	inst := NewInstance("test-lifecycle-ext-kill", "/tmp")
	inst.Command = "sleep 30"

	err := inst.Start()
	require.NoError(t, err, "Start() should succeed")
	defer func() { _ = inst.Kill() }() // Safety cleanup

	// Wait for grace period
	time.Sleep(2 * time.Second)

	// Verify session exists before external kill
	require.True(t, inst.Exists(), "session should exist before external kill")

	// Kill the tmux session externally (simulates user/system killing it)
	tmuxSess := inst.GetTmuxSession()
	require.NotNil(t, tmuxSess, "tmux session should not be nil")
	killCmd := exec.Command("tmux", "kill-session", "-t", tmuxSess.Name)
	err = killCmd.Run()
	require.NoError(t, err, "external tmux kill should succeed")

	// UpdateStatus should detect the missing session
	err = inst.UpdateStatus()
	require.NoError(t, err, "UpdateStatus() should succeed even when session is gone")

	// Should be StatusError after external kill
	assert.Equal(t, StatusError, inst.Status, "status should be error after external kill")
}

// TestHookFastPath_RunningStatus verifies that when a Claude-compatible instance
// has hookStatus="running" set via UpdateHookStatus, the next UpdateStatus()
// returns StatusRunning (via the hook fast path).
func TestHookFastPath_RunningStatus(t *testing.T) {
	skipIfNoTmuxServer(t)

	inst := NewInstanceWithTool("test-hook-running", "/tmp", "claude")
	inst.Command = "sleep 30" // Need a command so the session actually does something

	err := inst.Start()
	require.NoError(t, err, "Start() should succeed")
	defer func() { _ = inst.Kill() }()

	// Wait past grace period
	time.Sleep(2 * time.Second)

	// Set hook status to "running"
	inst.UpdateHookStatus(&HookStatus{
		Status:    "running",
		SessionID: "test-session-123",
		Event:     "UserPromptSubmit",
		UpdatedAt: time.Now(),
	})

	// UpdateStatus should use hook fast path and return Running
	err = inst.UpdateStatus()
	require.NoError(t, err, "UpdateStatus() should succeed")
	assert.Equal(t, StatusRunning, inst.Status, "hook fast path should set status to running")
}

// TestHookFastPath_WaitingAcknowledged is a table-driven test verifying the
// hook fast path behavior for waiting status. When acknowledged=true, status
// should be StatusIdle. When acknowledged=false, status should be StatusWaiting.
func TestHookFastPath_WaitingAcknowledged(t *testing.T) {
	skipIfNoTmuxServer(t)

	tests := []struct {
		name         string
		acknowledged bool
		wantStatus   Status
	}{
		{
			name:         "waiting not acknowledged shows as waiting",
			acknowledged: false,
			wantStatus:   StatusWaiting,
		},
		{
			name:         "waiting acknowledged shows as idle",
			acknowledged: true,
			wantStatus:   StatusIdle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := NewInstanceWithTool("test-hook-ack-"+tt.name, "/tmp", "claude")
			inst.Command = "sleep 30"

			err := inst.Start()
			require.NoError(t, err, "Start() should succeed")
			defer func() { _ = inst.Kill() }()

			// Wait past grace period
			time.Sleep(2 * time.Second)

			// Set acknowledged state on tmux session
			tmuxSess := inst.GetTmuxSession()
			require.NotNil(t, tmuxSess, "tmux session should not be nil")

			if tt.acknowledged {
				tmuxSess.Acknowledge()
			} else {
				tmuxSess.ResetAcknowledged()
			}

			// Set hook status to "waiting"
			inst.UpdateHookStatus(&HookStatus{
				Status:    "waiting",
				SessionID: "test-session-456",
				Event:     "Stop",
				UpdatedAt: time.Now(),
			})

			// UpdateStatus should use hook fast path
			err = inst.UpdateStatus()
			require.NoError(t, err, "UpdateStatus() should succeed")
			assert.Equal(t, tt.wantStatus, inst.Status,
				"hook fast path waiting with acknowledged=%v should be %s", tt.acknowledged, tt.wantStatus)
		})
	}
}

// TestHookFastPath_ShellIgnoresHooks verifies that shell tool sessions do NOT
// use the hook fast path. Shell sessions should always use tmux polling.
// The hook fast path condition requires IsClaudeCompatible(tool) || tool == "codex",
// and "shell" matches neither, so hook data should be ignored entirely.
func TestHookFastPath_ShellIgnoresHooks(t *testing.T) {
	skipIfNoTmuxServer(t)

	inst := NewInstance("test-shell-no-hooks", "/tmp")
	inst.Tool = "shell"
	inst.Command = "sleep 30"

	err := inst.Start()
	require.NoError(t, err, "Start() should succeed")
	defer func() { _ = inst.Kill() }()

	// Wait past instance-level grace period
	time.Sleep(2 * time.Second)

	// Set hook status to "running" (this should be ignored for shell tool)
	inst.UpdateHookStatus(&HookStatus{
		Status:    "running",
		SessionID: "test-session-789",
		Event:     "UserPromptSubmit",
		UpdatedAt: time.Now(),
	})

	// UpdateStatus should use tmux polling, not hook fast path
	err = inst.UpdateStatus()
	require.NoError(t, err, "UpdateStatus() should succeed")

	// The critical assertion: shell sessions must NOT get StatusRunning from hooks.
	// If the hook fast path were used, the status would be Running.
	// Since shell bypasses hooks, the status comes from tmux polling.
	assert.NotEqual(t, StatusRunning, inst.Status,
		"shell session should NOT use hook fast path (status should not be running from hook data)")
	t.Logf("Shell session status from tmux polling: %s", inst.Status)
}
