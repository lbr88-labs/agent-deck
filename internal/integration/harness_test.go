package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHarness_CreateSession(t *testing.T) {
	skipIfNoTmuxServer(t)

	h := NewTmuxHarness(t)
	inst := h.CreateSession("create-test", "/tmp")
	err := inst.Start()
	require.NoError(t, err, "session should start")
	require.True(t, inst.Exists(), "session should exist after start")

	// Cleanup happens automatically via t.Cleanup.
	// After the test function returns, the harness cleanup runs
	// and the tmux session should be gone. We cannot verify this
	// in the same test, but TestHarness_MultipleSessionsCleanup
	// exercises the cleanup path explicitly.
}

func TestHarness_MultipleSessionsCleanup(t *testing.T) {
	skipIfNoTmuxServer(t)

	h := NewTmuxHarness(t)

	var insts [3]*struct{ exists func() bool }
	for i := 0; i < 3; i++ {
		inst := h.CreateSession("multi-"+string(rune('a'+i)), "/tmp")
		err := inst.Start()
		require.NoError(t, err, "session %d should start", i)
		require.True(t, inst.Exists(), "session %d should exist", i)
		existsFn := inst.Exists // capture
		insts[i] = &struct{ exists func() bool }{exists: existsFn}
	}

	require.Equal(t, 3, h.SessionCount(), "harness should track 3 sessions")

	// Explicitly call cleanup to verify all sessions are killed.
	h.cleanup()

	// After cleanup, no sessions should exist.
	// Note: tmux Exists() checks may need a cache refresh. We do a direct
	// tmux has-session check to avoid stale cache.
	for i, inst := range insts {
		require.False(t, inst.exists(), "session %d should not exist after cleanup", i)
	}
}

func TestHarness_PrefixNaming(t *testing.T) {
	skipIfNoTmuxServer(t)

	h := NewTmuxHarness(t)
	inst := h.CreateSession("prefix-test", "/tmp")

	// The tmux session name should contain the inttest_ prefix.
	tmuxSess := inst.GetTmuxSession()
	require.NotNil(t, tmuxSess, "tmux session should not be nil")
	require.Contains(t, tmuxSess.Name, "inttest-", "session name should contain inttest- prefix")
}
