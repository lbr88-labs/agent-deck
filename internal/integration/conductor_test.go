package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestConductor_SendToChild verifies that a child session running `cat` receives
// text sent via SendKeysAndEnter and the text appears in the child's pane content. (COND-01)
func TestConductor_SendToChild(t *testing.T) {
	h := NewTmuxHarness(t)

	inst := h.CreateSession("cond-child", "/tmp")
	inst.Command = "cat"
	require.NoError(t, inst.Start())

	WaitForCondition(t, 5*time.Second, 200*time.Millisecond,
		"session to exist",
		func() bool { return inst.Exists() })

	tmuxSess := inst.GetTmuxSession()
	require.NotNil(t, tmuxSess, "tmux session should not be nil")

	msg := "hello-from-conductor-" + t.Name()
	require.NoError(t, tmuxSess.SendKeysAndEnter(msg))

	WaitForPaneContent(t, inst, "hello-from-conductor-", 5*time.Second)
}

// TestConductor_SendMultipleMessages verifies that two sequential messages sent
// via SendKeysAndEnter both appear in the child's pane content, proving reliable
// sequential delivery. (COND-01)
func TestConductor_SendMultipleMessages(t *testing.T) {
	h := NewTmuxHarness(t)

	inst := h.CreateSession("cond-multi", "/tmp")
	inst.Command = "cat"
	require.NoError(t, inst.Start())

	WaitForCondition(t, 5*time.Second, 200*time.Millisecond,
		"session to exist",
		func() bool { return inst.Exists() })

	tmuxSess := inst.GetTmuxSession()
	require.NotNil(t, tmuxSess, "tmux session should not be nil")

	require.NoError(t, tmuxSess.SendKeysAndEnter("msg-one"))
	WaitForPaneContent(t, inst, "msg-one", 5*time.Second)

	require.NoError(t, tmuxSess.SendKeysAndEnter("msg-two"))
	WaitForPaneContent(t, inst, "msg-two", 5*time.Second)
}
