package testutil

import (
	"os"
	"strings"
	"testing"
)

// TestIsolateTmuxSocket verifies that the helper sets TMUX_TMPDIR to an
// isolated directory that is NOT the user's default /tmp/tmux-<uid>/ path,
// and that cleanup removes the directory.
func TestIsolateTmuxSocket(t *testing.T) {
	// Save and restore the env var around this test.
	orig, had := os.LookupEnv("TMUX_TMPDIR")
	defer func() {
		if had {
			os.Setenv("TMUX_TMPDIR", orig)
		} else {
			os.Unsetenv("TMUX_TMPDIR")
		}
	}()

	cleanup := IsolateTmuxSocket()

	// After call, TMUX_TMPDIR must be set.
	dir := os.Getenv("TMUX_TMPDIR")
	if dir == "" {
		t.Fatal("IsolateTmuxSocket did not set TMUX_TMPDIR")
	}

	// CRITICAL: must NOT be the default /tmp/tmux-<uid> location that the
	// user's real sessions use. That's the whole point of this helper.
	defaultDefault := "/tmp"
	if dir == defaultDefault {
		t.Errorf("IsolateTmuxSocket left TMUX_TMPDIR=%q (the default tmux dir) — this would NOT isolate from user sessions", dir)
	}
	if strings.HasPrefix(dir, "/tmp/tmux-") {
		t.Errorf("IsolateTmuxSocket set TMUX_TMPDIR=%q which is the user's real tmux dir pattern", dir)
	}

	// The directory should exist.
	if stat, err := os.Stat(dir); err != nil || !stat.IsDir() {
		t.Errorf("isolated temp dir %q does not exist or is not a directory: %v", dir, err)
	}

	// Cleanup removes the directory.
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("cleanup did not remove %q (stat err=%v)", dir, err)
	}
}

// TestIsolateTmuxSocket_UniquePerCall ensures repeated calls return
// different directories — so parallel test binaries don't collide.
func TestIsolateTmuxSocket_UniquePerCall(t *testing.T) {
	// Save and restore
	orig, had := os.LookupEnv("TMUX_TMPDIR")
	defer func() {
		if had {
			os.Setenv("TMUX_TMPDIR", orig)
		} else {
			os.Unsetenv("TMUX_TMPDIR")
		}
	}()

	c1 := IsolateTmuxSocket()
	dir1 := os.Getenv("TMUX_TMPDIR")
	c1()

	c2 := IsolateTmuxSocket()
	dir2 := os.Getenv("TMUX_TMPDIR")
	c2()

	if dir1 == dir2 {
		t.Errorf("expected unique dirs per call, both got %q", dir1)
	}
}
