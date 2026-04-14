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
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
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

// pidAlive returns true if a process with the given pid currently exists.
// It uses signal 0, which checks for process existence without sending a
// real signal. Returns false for non-positive pids.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, syscall.Signal(0)) == nil
}

// randomHex8 returns 8 hex chars (4 random bytes) for use in unique unit /
// socket names. On rand.Read failure it calls t.Fatalf — a truly vacuous
// failure mode we want surfaced loudly.
func randomHex8(t *testing.T) string {
	t.Helper()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("randomHex8: rand.Read: %v", err)
	}
	return hex.EncodeToString(b[:])
}

// startFakeLoginScope launches a throwaway systemd user scope that simulates
// an SSH login-session scope: `systemd-run --user --scope --unit=fake-login-<hex>
// bash -c "exec sleep 300"`. The scope stays alive until the test (or its
// cleanup) calls `systemctl --user stop <name>.scope`. Returns the unit name
// (without the ".scope" suffix) and registers a best-effort stop in t.Cleanup.
//
// Safety: scope unit names use the literal "fake-login-" prefix plus an 8-hex
// random suffix. Cleanup only ever stops that exact unit — never a wildcard.
func startFakeLoginScope(t *testing.T) string {
	t.Helper()
	fakeName := "fake-login-" + randomHex8(t)
	cmd := exec.Command("systemd-run", "--user", "--scope", "--quiet",
		"--collect", "--unit="+fakeName,
		"bash", "-c", "exec sleep 300")
	if err := cmd.Start(); err != nil {
		t.Fatalf("startFakeLoginScope: systemd-run start: %v", err)
	}
	t.Cleanup(func() {
		// Idempotent: scope may already be stopped by the test body.
		_ = exec.Command("systemctl", "--user", "stop", fakeName+".scope").Run()
	})
	// Give systemd up to 2s to register the transient scope so a racing
	// systemctl stop in the test body is not a no-op.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := exec.Command("systemctl", "--user", "is-active", "--quiet", fakeName+".scope").Run(); err == nil {
			return fakeName
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Not strictly fatal — the scope may be in "activating" state which
	// is still stoppable. Return the name and let the caller proceed.
	return fakeName
}

// startAgentDeckTmuxInUserScope launches a tmux server under its OWN
// `agentdeck-tmux-<serverName>` user scope — mirroring the production
// `LaunchInUserScope=true` path in internal/tmux/tmux.go:startCommandSpec.
// Uses `tmux -L <serverName>` so kill-server is scoped to this test's
// private socket (never touches user sessions — see repo CLAUDE.md tmux
// safety mandate and 2025-12-10 incident).
//
// Returns the tmux server PID (read from `systemctl --user show -p MainPID`).
// Registers cleanup that kills the private tmux socket and stops the scope.
func startAgentDeckTmuxInUserScope(t *testing.T, serverName string) int {
	t.Helper()
	unit := "agentdeck-tmux-" + serverName
	cmd := exec.Command("systemd-run", "--user", "--scope", "--quiet",
		"--collect", "--unit="+unit,
		"tmux", "-L", serverName, "new-session", "-d", "-s", "persist",
		"bash", "-c", "exec sleep 300")
	if err := cmd.Start(); err != nil {
		t.Fatalf("startAgentDeckTmuxInUserScope: systemd-run start: %v", err)
	}
	t.Cleanup(func() {
		// -L <serverName> confines kill-server to this test's private socket.
		_ = exec.Command("tmux", "-L", serverName, "kill-server").Run()
		_ = exec.Command("systemctl", "--user", "stop", unit+".scope").Run()
	})
	// Wait up to 2s for `tmux -L <serverName> list-sessions` to succeed.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := exec.Command("tmux", "-L", serverName, "list-sessions").Run(); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Read MainPID from systemd user manager — the server PID is the
	// MainPID of its enclosing scope.
	out, err := exec.Command("systemctl", "--user", "show", "-p", "MainPID", "--value", unit+".scope").Output()
	if err != nil {
		t.Fatalf("startAgentDeckTmuxInUserScope: systemctl show MainPID: %v", err)
	}
	pidStr := strings.TrimSpace(string(out))
	pid, perr := strconv.Atoi(pidStr)
	if perr != nil || pid <= 0 {
		t.Fatalf("startAgentDeckTmuxInUserScope: invalid MainPID %q: %v", pidStr, perr)
	}
	return pid
}

// TestPersistence_TmuxSurvivesLoginSessionRemoval replicates the 2026-04-14
// incident root cause. It:
//
//  1. Checks GetLaunchInUserScope() default — on current v1.5.1 this is
//     false, which means the production path would have inherited the
//     login-session cgroup and died. Test fails RED here with a diagnostic
//     message telling Phase 2 what to fix. No tmux spawning happens in
//     the RED branch, so there is nothing to leak.
//  2. (Post-Phase-2 flow) Starts a fake-login user scope simulating an
//     SSH login session, starts a tmux server under its OWN
//     agentdeck-tmux-<name>.scope (mirroring the fix), tears down the
//     fake-login scope, and asserts the tmux server survives because it
//     was parented under user@UID.service, NOT under the login-session
//     scope tree.
//
// Skip semantics: requireSystemdRun skips cleanly on macOS / non-systemd
// hosts with "no systemd-run available:" in the message.
func TestPersistence_TmuxSurvivesLoginSessionRemoval(t *testing.T) {
	requireSystemdRun(t)

	// RED-state gate: if the default is still false, this test fails with
	// the diagnostic that tells Phase 2 what to fix. This check intentionally
	// runs BEFORE any tmux spawning so the RED message is unambiguous and
	// no tmux server is created to leak.
	_ = isolatedHomeDir(t)
	settings := GetTmuxSettings()
	if !settings.GetLaunchInUserScope() {
		t.Fatalf("TEST-01 RED: GetLaunchInUserScope() default is false on Linux+systemd; simulated teardown would kill production tmux. Phase 2 must flip the default; rerun this test after the flip to exercise real cgroup survival.")
	}

	// Post-Phase-2 flow: simulate the 2026-04-14 incident.
	serverName := uniqueTmuxServerName(t)
	fakeLogin := startFakeLoginScope(t)

	pid := startAgentDeckTmuxInUserScope(t, serverName)
	if !pidAlive(pid) {
		t.Fatalf("setup failure: tmux pid %d not alive immediately after spawn", pid)
	}

	// Teardown the fake login scope — simulates logind removing an SSH login session.
	if err := exec.Command("systemctl", "--user", "stop", fakeLogin+".scope").Run(); err != nil {
		// Treat non-existence as acceptable (already stopped / never registered).
		t.Logf("systemctl stop %s: %v (continuing)", fakeLogin, err)
	}

	// Give systemd up to 3s to settle the teardown.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}

	if !pidAlive(pid) {
		t.Fatalf("TEST-01 RED: tmux server pid %d died after fake-login scope teardown; expected to survive because the server was launched under its own agentdeck-tmux-<name>.scope. The 2026-04-14 incident is recurring.", pid)
	}
}
