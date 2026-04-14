package web

import (
	"os"
	"testing"
)

// TestMain forces AGENTDECK_PROFILE=_test for all internal/web tests.
// This prevents integration tests that create real tmux sessions from
// running under the active production profile and corrupting session data.
// CRITICAL: Do not remove — see CLAUDE.md test isolation rules.
func TestMain(m *testing.M) {
	os.Setenv("AGENTDECK_PROFILE", "_test")
	os.Exit(m.Run())
}
