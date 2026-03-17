package vcs

import (
	"os"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/testutil"
)

func TestMain(m *testing.M) {
	// Git hooks export GIT_DIR/GIT_WORK_TREE; clear them so test subprocess git
	// commands operate on their temp repos instead of the real repository.
	testutil.UnsetGitRepoEnv()

	// Force test profile to prevent production data corruption
	os.Setenv("AGENTDECK_PROFILE", "_test")

	os.Exit(m.Run())
}
