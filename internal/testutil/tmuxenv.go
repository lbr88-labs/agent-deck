package testutil

import (
	"fmt"
	"os"
)

// IsolateTmuxSocket points TMUX_TMPDIR at a unique temp directory for this
// test process, so that `tmux new-session`, `tmux list-sessions`, and
// `tmux kill-session` commands spawned by the test suite all operate on an
// isolated tmux socket — NOT the user's default `/tmp/tmux-<uid>/default`.
//
// Why this matters:
//
// Before this helper existed, `go test ./...` on a host with active
// agent-deck sessions (a conductor running, user sessions in flight)
// caused cascading session death. The test suite creates and kills tmux
// sessions as part of normal test fixture setup/teardown. Without socket
// isolation, those operations hit the user's real tmux server:
//
//   - New sessions pile up in the user's server, polluting `tmux ls`
//     output for the duration of the test run.
//   - The cleanup paths in various tests (kill-session patterns,
//     post-TestMain cleanup) can kill or destabilize the user's tmux
//     server, taking down every user session with it.
//   - Incident on 2026-04-17: a maintainer ran `go test ./...` during
//     PR review; every user session in the personal profile went to
//     error state within 24 minutes as the shared tmux server died.
//
// Call this from every package-level `TestMain` that touches tmux:
//
//	func TestMain(m *testing.M) {
//	    cleanup := testutil.IsolateTmuxSocket()
//	    defer cleanup()
//	    os.Exit(m.Run())
//	}
//
// Returns a cleanup function that removes the temp directory.
func IsolateTmuxSocket() func() {
	dir, err := os.MkdirTemp("", "agent-deck-test-tmux-")
	if err != nil {
		// If we can't isolate, we still want tests to run — but we
		// REALLY don't want them on the default socket.
		// Fall back to a well-known test path that won't collide.
		dir = fmt.Sprintf("/tmp/agent-deck-test-tmux-fallback-%d", os.Getpid())
		_ = os.MkdirAll(dir, 0o700)
	}
	_ = os.Setenv("TMUX_TMPDIR", dir)
	return func() {
		// Best-effort cleanup. Stale tmux sockets in the temp dir are
		// harmless — kernel removes them when the binding tmux server
		// exits, and `go test` process exit will release any we spawned.
		_ = os.RemoveAll(dir)
	}
}
