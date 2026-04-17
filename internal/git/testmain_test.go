package git

import (
	"os"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/testutil"
)

func TestMain(m *testing.M) {
	// Git hooks export GIT_DIR/GIT_WORK_TREE; clear them so test subprocess git
	// commands operate on their temp repos instead of the real repository.
	testutil.UnsetGitRepoEnv()

	// Isolate the tmux socket. Even this package's tests run under `go test ./...`,
	// which means other packages' tmux-spawning code runs in the same shell
	// invocation — we want every package's TestMain to enforce isolation so no
	// ordering surprise can leak onto the user's default socket (2026-04-17 incident).
	// See internal/testutil/tmuxenv.go for the full postmortem.
	cleanupTmux := testutil.IsolateTmuxSocket()
	defer cleanupTmux()

	os.Exit(m.Run())
}
