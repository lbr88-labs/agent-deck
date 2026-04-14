// Package session: Session persistence regression test suite.
//
// Purpose
// -------
// This file holds the regression tests for the 2026-04-14 session-persistence
// incident. At 09:08:01 local time on the conductor host, a single SSH logout
// caused systemd-logind to tear down every agent-deck-managed tmux server,
// destroying 33 live Claude conversations (plus another 39 that ended up in
// "stopped" status). This was the third recurrence of the same class of bug.
//
// Mandate
// -------
// The repo-root CLAUDE.md file contains a "Session persistence: mandatory
// test coverage" section that makes this suite P0 forever. Any PR touching
// internal/tmux/**, internal/session/instance.go, internal/session/userconfig.go,
// internal/session/storage*.go, or cmd/agent-deck/session_cmd.go MUST run
// `go test -run TestPersistence_ ./internal/session/... -race -count=1` and
// include the output in the PR description. The following eight tests are
// permanently required — removing any of them without an RFC is forbidden:
//
//  1. TestPersistence_TmuxSurvivesLoginSessionRemoval
//  2. TestPersistence_TmuxDiesWithoutUserScope
//  3. TestPersistence_LinuxDefaultIsUserScope
//  4. TestPersistence_MacOSDefaultIsDirect
//  5. TestPersistence_RestartResumesConversation
//  6. TestPersistence_StartAfterSIGKILLResumesConversation
//  7. TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion
//  8. TestPersistence_FreshSessionUsesSessionIDNotResume
//
// Phase 1 of v1.5.2 (this file) lands the shared helpers plus TEST-03 and
// TEST-04; Plans 02 and 03 of the phase append the remaining six tests.
//
// Safety note (tmux)
// ------------------
// On 2025-12-10, an earlier incident killed 40 user tmux sessions because a
// blanket `tmux kill-server` was run against all servers matching "agentdeck".
// Tests in this file MUST:
//   - use the `agentdeck-test-persist-<hex>` prefix for every server they create;
//   - only call `tmux kill-server -t <name>` with the exact server name they
//     own; and
//   - NEVER call `tmux kill-server` without a `-t <name>` filter.
//
// The helper uniqueTmuxServerName enforces this by registering a targeted
// t.Cleanup that kills only the server it allocated.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// uniqueTmuxServerName returns a tmux server name with the mandatory
// "agentdeck-test-persist-" prefix plus an 8-hex-character random suffix,
// and registers a t.Cleanup that runs `tmux kill-server -t <name>` on teardown.
//
// Safety: this helper NEVER runs a bare `tmux kill-server`. The -t filter is
// required by the repo CLAUDE.md tmux safety mandate (see the 2025-12-10
// incident notes in the package-level comment above).
func uniqueTmuxServerName(t *testing.T) string {
	t.Helper()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("uniqueTmuxServerName: rand.Read: %v", err)
	}
	name := "agentdeck-test-persist-" + hex.EncodeToString(b[:])
	t.Cleanup(func() {
		// Safety: ONLY kill the server we created. Never run bare
		// `tmux kill-server` — that would destroy every user session on
		// the host. The -t <name> filter is mandatory.
		_ = exec.Command("tmux", "kill-server", "-t", name).Run()
	})
	return name
}

// requireSystemdRun skips the current test if systemd-run is unavailable.
//
// The skip message contains the literal substring "no systemd-run available:"
// so CI log scrapers and the grep-based acceptance criteria in the plan can
// detect a vacuous-skip regression.
func requireSystemdRun(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("systemd-run"); err != nil {
		t.Skipf("no systemd-run available: %v", err)
		return
	}
	if err := exec.Command("systemd-run", "--user", "--version").Run(); err != nil {
		t.Skipf("no systemd-run available: %v", err)
	}
}

// writeStubClaudeBinary writes an executable stub `claude` script into dir and
// returns dir so the caller can prepend it to PATH. The stub appends its argv
// (one arg per line) to the file named by AGENTDECK_TEST_ARGV_LOG (or /dev/null
// if that env var is unset), then sleeps 30 seconds so tmux panes created with
// it stay alive long enough to be inspected. The file is removed on test
// cleanup.
func writeStubClaudeBinary(t *testing.T, dir string) string {
	t.Helper()
	script := "#!/usr/bin/env bash\nprintf '%s\\n' \"$@\" >> \"${AGENTDECK_TEST_ARGV_LOG:-/dev/null}\"\nsleep 30\n"
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writeStubClaudeBinary: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return dir
}

// isolatedHomeDir creates a fresh temp HOME with ~/.agent-deck/,
// ~/.agent-deck/hooks/, and ~/.claude/projects/ pre-created, then sets
// HOME to that path for the duration of the test and clears the
// agent-deck user-config cache so tests exercise the default branch of
// GetTmuxSettings(). A t.Cleanup is registered that clears the cache again
// once HOME is restored, so config state does not leak to adjacent tests.
func isolatedHomeDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	for _, sub := range []string{".agent-deck", ".agent-deck/hooks", ".claude/projects"} {
		if err := os.MkdirAll(filepath.Join(home, sub), 0o755); err != nil {
			t.Fatalf("isolatedHomeDir mkdir %s: %v", sub, err)
		}
	}
	t.Setenv("HOME", home)
	ClearUserConfigCache()
	t.Cleanup(func() { ClearUserConfigCache() })
	return home
}

// TestPersistence_LinuxDefaultIsUserScope pins REQ-1: on a Linux host where
// systemd-run is available and no config.toml overrides it, the default
// MUST be launch_in_user_scope=true. Phase 2 will flip the default; this
// test is RED against current v1.5.1 (userconfig.go pins the default at
// false, userconfig_test.go:~1102 still asserts that pinning).
//
// Skip semantics: on hosts without systemd-run, requireSystemdRun skips
// with "no systemd-run available: <err>" so macOS CI passes cleanly.
func TestPersistence_LinuxDefaultIsUserScope(t *testing.T) {
	requireSystemdRun(t)
	home := isolatedHomeDir(t)
	// Write an empty config so GetTmuxSettings() exercises the default
	// branch (no [tmux] section, no launch_in_user_scope override).
	cfg := filepath.Join(home, ".agent-deck", "config.toml")
	if err := os.WriteFile(cfg, []byte(""), 0o644); err != nil {
		t.Fatalf("write empty config: %v", err)
	}
	ClearUserConfigCache()

	settings := GetTmuxSettings()
	if !settings.GetLaunchInUserScope() {
		t.Fatalf("TEST-03 RED: GetLaunchInUserScope() returned false on a Linux+systemd host with no config; expected true. Phase 2 must flip the default. systemd-run present, no config override.")
	}
}

// TestPersistence_MacOSDefaultIsDirect pins REQ-1: on a host WITHOUT
// systemd-run (macOS, BSD, minimal Linux), the default MUST remain false
// and no error is logged. The test name says "MacOS" but its assertion
// body runs on any host where systemd-run is absent.
//
// Linux+systemd behavior (documented implementer choice, 2026-04-14):
// this test SKIPS on hosts where systemd-run is available. TEST-03
// covers the Linux+systemd default. TEST-04's assertion body only runs
// on hosts where systemd-run is absent. Rationale: GetTmuxSettings() in
// Phase 2 will detect systemd-run at call time; asserting
// "false on Linux+systemd" here would lock in the v1.5.1 bug and
// collide with TEST-03 after Phase 2.
func TestPersistence_MacOSDefaultIsDirect(t *testing.T) {
	if _, err := exec.LookPath("systemd-run"); err == nil {
		t.Skipf("systemd-run available; TEST-04 only asserts non-systemd behavior — see TEST-03 for Linux+systemd default")
		return
	}
	home := isolatedHomeDir(t)
	cfg := filepath.Join(home, ".agent-deck", "config.toml")
	if err := os.WriteFile(cfg, []byte(""), 0o644); err != nil {
		t.Fatalf("write empty config: %v", err)
	}
	ClearUserConfigCache()

	settings := GetTmuxSettings()
	if settings.GetLaunchInUserScope() {
		t.Fatalf("TEST-04: on a host without systemd-run, GetLaunchInUserScope() must return false, got true")
	}
}
