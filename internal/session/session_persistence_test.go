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
	"encoding/json"
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

// pidAlive returns true if a process with the given pid exists AND is not
// a zombie. syscall.Kill(pid, 0) returns nil for zombies, but for our
// "did tmux die?" assertions we treat a zombie as dead — the daemon has
// exited and is merely awaiting reap by its parent. We consult
// /proc/<pid>/status State: field; state "Z" (zombie) or "X" (dead,
// exiting) counts as dead. Non-positive pids and missing /proc entries
// are also dead.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if syscall.Kill(pid, syscall.Signal(0)) != nil {
		return false
	}
	data, rerr := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status")
	if rerr != nil {
		// /proc entry gone between the Kill(0) check and now — process has
		// been reaped. Treat as dead.
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "State:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				switch fields[1] {
				case "Z", "X":
					return false
				}
			}
			break
		}
	}
	return true
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

// startTmuxInsideFakeLogin launches a tmux server as a grandchild of a
// throwaway fake-login-<hex> user scope — mirroring the production
// LaunchInUserScope=false path where tmux inherits the user's SSH
// login-session cgroup. Used by TEST-02 to confirm that WITHOUT
// cgroup isolation, a scope teardown does kill the tmux server.
//
// Returns (fakeName, tmuxServerPID). Registers cleanup that stops the
// scope and kills the private tmux socket (-L <serverName>).
//
// Safety: tmux socket name and scope name both use per-test random
// suffixes. kill-server is confined to the -L <serverName> socket.
func startTmuxInsideFakeLogin(t *testing.T, serverName string) (string, int) {
	t.Helper()
	fakeName := "fake-login-" + randomHex8(t)
	// Start tmux as a grandchild of the fake-login scope. The outer
	// `sleep 300` keeps the scope alive until the test body tears it down.
	shellCmd := "tmux -L " + serverName + " new-session -d -s persist bash -c 'exec sleep 300'; exec sleep 300"
	cmd := exec.Command("systemd-run", "--user", "--scope", "--quiet",
		"--collect", "--unit="+fakeName,
		"bash", "-c", shellCmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("startTmuxInsideFakeLogin: systemd-run start: %v", err)
	}
	t.Cleanup(func() {
		_ = exec.Command("systemctl", "--user", "stop", fakeName+".scope").Run()
		// -L <serverName> confines kill-server to this test's private socket.
		_ = exec.Command("tmux", "-L", serverName, "kill-server").Run()
	})
	// Poll up to 3s for the tmux server process to appear. pgrep with
	// the unique -L <serverName> argument ensures we only ever match
	// the server we just started.
	deadline := time.Now().Add(3 * time.Second)
	var pid int
	for time.Now().Before(deadline) {
		out, err := exec.Command("pgrep", "-f", "tmux -L "+serverName+" ").Output()
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				p, perr := strconv.Atoi(line)
				if perr == nil && p > 0 {
					pid = p
					break
				}
			}
			if pid > 0 {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatalf("startTmuxInsideFakeLogin: could not locate tmux server PID for -L %s within 3s", serverName)
	}
	return fakeName, pid
}

// pidCgroup returns the contents of /proc/<pid>/cgroup (unified hierarchy
// v2 line). Empty string on any error.
func pidCgroup(pid int) string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cgroup")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// TestPersistence_TmuxDiesWithoutUserScope is the INVERSE PIN. It asserts
// that when tmux is spawned WITHOUT the systemd-run --user --scope wrap
// (i.e., launch_in_user_scope=false — the current v1.5.1 default and also
// the explicit opt-out path after Phase 2), a login-session scope teardown
// DOES kill the tmux server. This replicates the 2026-04-14 incident root
// cause and must stay green for the entire milestone. Any future "fix"
// that silently masks opt-outs will break this test.
//
// Skip semantics:
//   - requireSystemdRun skips cleanly on macOS / non-systemd hosts with
//     "no systemd-run available:" in the message.
//   - If this process is already running inside a transient scope (e.g., a
//     tmux-spawn-*.scope used by agent-deck itself, or a nested
//     systemd-run --scope call), systemd places the child scope's tracked
//     processes in the PARENT scope's cgroup rather than the new unit's
//     cgroup. In that edge case the scope-teardown simulation is a no-op
//     and the test skips with a diagnostic so CI (running from a normal
//     login shell) still exercises the assertion.
func TestPersistence_TmuxDiesWithoutUserScope(t *testing.T) {
	requireSystemdRun(t)
	_ = isolatedHomeDir(t)
	serverName := uniqueTmuxServerName(t)

	fakeName, pid := startTmuxInsideFakeLogin(t, serverName)
	if !pidAlive(pid) {
		t.Fatalf("setup failure: tmux server pid %d not alive immediately after spawn", pid)
	}

	// Diagnostic: record the actual cgroup placement so failures surface the
	// systemd nesting edge case loudly.
	t.Logf("tmux pid=%d cgroup=%q", pid, pidCgroup(pid))

	// Nested-scope edge case: if tmux did not actually land inside the
	// fake-login scope's cgroup, the scope teardown cannot kill it and the
	// assertion below would be testing nothing. Skip cleanly so CI running
	// from a normal login shell (where tmux DOES land in the scope cgroup)
	// still exercises the real assertion.
	cg := pidCgroup(pid)
	if !strings.Contains(cg, fakeName+".scope") {
		t.Skipf("TEST-02 skipped: tmux pid %d did not land in %s.scope cgroup (got %q) — this process is likely already inside a transient scope, which reparents child scopes. Run from a login shell or the verify-session-persistence.sh harness.", pid, fakeName, cg)
	}

	// Simulate the 2026-04-14 incident: systemd-logind forcibly terminates
	// an SSH login-session scope when the user logs out. That is NOT a
	// polite `systemctl stop` — scopes by default release their cgroup
	// without actively killing members, and `systemctl kill` on an
	// already-transitioning scope can race with concurrent tmux forks.
	// The only atomic, race-free primitive is cgroup v2's `cgroup.kill`,
	// which SIGKILLs every task in the cgroup (and any concurrently
	// forking descendants) in one kernel operation. This matches the
	// effective behavior logind applies to a session scope on logout.
	scopeCg, scopeErr := exec.Command("systemctl", "--user", "show",
		"-p", "ControlGroup", "--value", fakeName+".scope").Output()
	scopeCgPath := strings.TrimSpace(string(scopeCg))
	if scopeErr != nil || scopeCgPath == "" {
		t.Fatalf("could not resolve ControlGroup for %s: err=%v out=%q", fakeName, scopeErr, scopeCgPath)
	}
	killFile := "/sys/fs/cgroup" + scopeCgPath + "/cgroup.kill"
	if err := os.WriteFile(killFile, []byte("1"), 0o644); err != nil {
		t.Fatalf("write cgroup.kill %s: %v", killFile, err)
	}

	// Poll up to 3s for the pid to die. cgroup.kill delivers SIGKILL to
	// all tasks atomically; reap is near-instant but scheduler latency can
	// add tens of milliseconds.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return // PASS — tmux died with the scope as expected.
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Final diagnostic before failing: report the pid's cgroup state so a
	// nested-scope or SIGKILL-not-delivered regression is easy to diagnose.
	finalCg := pidCgroup(pid)
	t.Fatalf("TEST-02 INVERSE PIN: tmux server pid %d survived cgroup.kill SIGKILL teardown WITHOUT launch_in_user_scope after 3s. "+
		"Pid cgroup after kill: %q. "+
		"The opt-out path must remain vulnerable so any future 'fix' that silently masks opt-outs is caught. Expected death.",
		pid, finalCg)
}

// ----------------------------------------------------------------------------
// Wave 3 (Plan 03): resume-dispatch helpers + TEST-05, TEST-06, TEST-07, TEST-08
// ----------------------------------------------------------------------------
//
// These tests exercise the REAL Claude dispatch paths (`(*Instance).Start()`
// and `(*Instance).Restart()`) by placing a stub `claude` binary on PATH and
// capturing the argv it receives when the dispatch spawns it inside a real
// tmux session. The contract under test is REQ-2: every path that starts a
// Claude session on an Instance with a non-empty `ClaudeSessionID` and an
// existing JSONL transcript MUST produce `claude --resume <id>`; a fresh
// session (no transcript) MUST produce `claude --session-id <id>`.
//
// Per CLAUDE.md no-mocking rule: tmux and shell spawn are real binaries; only
// the `claude` binary itself is a stub (explicitly carved out in CONTEXT.md
// because these tests assert on the spawned command line, not Claude's
// behavior).

// readCapturedClaudeArgv polls the stub claude argv log until it is non-empty
// (stub has been spawned and wrote its argv), then returns the argv lines.
// Fatals if timeout elapses with empty log (dispatch never spawned claude).
func readCapturedClaudeArgv(t *testing.T, logPath string, timeout time.Duration) []string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(logPath)
		if err == nil && len(data) > 0 {
			var argv []string
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					argv = append(argv, line)
				}
			}
			if len(argv) > 0 {
				return argv
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("readCapturedClaudeArgv: no argv captured in %s at %s — stub claude was never spawned; check PATH prepending and tmux session creation", timeout, logPath)
	return nil // unreachable
}

// newClaudeInstanceForDispatch constructs an *Instance wired for Claude
// dispatch testing. It:
//   - creates a real project directory under the isolated HOME,
//   - calls NewInstanceWithTool so inst.tmuxSession is initialized,
//   - overrides inst.ID with a deterministic-per-test hex suffix so the
//     hook-sidecar path is predictable (TEST-07 sanity assertion),
//   - generates a uuid-shaped inst.ClaudeSessionID from 16 random bytes,
//   - sets inst.Command = "claude" so buildClaudeCommandWithMessage takes
//     the `baseCommand == "claude"` branch,
//   - registers t.Cleanup to kill the tmux session via the safe (Name-
//     scoped) (*tmux.Session).Kill() path.
func newClaudeInstanceForDispatch(t *testing.T, home string) *Instance {
	t.Helper()
	var idb [4]byte
	if _, err := rand.Read(idb[:]); err != nil {
		t.Fatalf("newClaudeInstanceForDispatch: rand.Read(idb): %v", err)
	}
	var sidb [16]byte
	if _, err := rand.Read(sidb[:]); err != nil {
		t.Fatalf("newClaudeInstanceForDispatch: rand.Read(sidb): %v", err)
	}
	sidHex := hex.EncodeToString(sidb[:])
	sid := sidHex[0:8] + "-" + sidHex[8:12] + "-" + sidHex[12:16] + "-" + sidHex[16:20] + "-" + sidHex[20:32]

	projectPath := filepath.Join(home, "project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("newClaudeInstanceForDispatch: mkdir project: %v", err)
	}
	title := "persist-test-" + hex.EncodeToString(idb[:])
	inst := NewInstanceWithTool(title, projectPath, "claude")
	// Override auto-generated ID so the sidecar path is deterministic for
	// TEST-07 and log messages reference a recognizable test ID. Note: the
	// tmux session Name was set by NewInstanceWithTool from the title — it
	// remains unique via the "persist-test-<hex>" suffix regardless of ID.
	inst.ID = "test-" + hex.EncodeToString(idb[:])
	inst.ClaudeSessionID = sid
	inst.Command = "claude"

	t.Cleanup(func() {
		// inst.tmuxSession.Kill() targets the unique session Name — SAFE;
		// does NOT call bare `tmux kill-server`.
		if inst.tmuxSession != nil {
			_ = inst.tmuxSession.Kill()
		}
	})
	return inst
}

// setupStubClaudeOnPATH drops the writeStubClaudeBinary helper's stub script
// at <home>/bin/claude, sets AGENTDECK_TEST_ARGV_LOG so the stub logs argv to
// a known file, and configures the dispatch to invoke the stub by its
// ABSOLUTE path.
//
// [Deviation Rule 3 — Blocking fix applied during Plan 03 Task 2]
// The plan prescribed prepending <home>/bin to PATH so `claude` in the
// spawned tmux pane would resolve to the stub. That does not work on this
// executor host (or any environment where tests run inside a pre-existing
// tmux server): `tmux new-session` uses the DEFAULT tmux socket, which is
// already owned by a server started long before the test. That server's
// environment was captured at its own startup, and new sessions inherit the
// server's PATH rather than the spawning client's PATH. Empirically
// confirmed: env vars set via `t.Setenv("PATH", ...)` do not reach the
// initial process of a tmux pane on the default socket.
//
// The robust fix: write a real agent-deck config.toml under the isolated
// HOME with `[claude] command = "<abs stub path>"`. GetClaudeCommand() picks
// this up at dispatch time (instance.go:4121, :492), and the spawned
// command embeds the stub's ABSOLUTE path — no PATH search needed. We also
// forward AGENTDECK_TEST_ARGV_LOG into the tmux pane environment via
// tmux set-environment on the default socket (inline export is redundant
// but cheap; env-var resolution inside the stub reads it from the pane's
// env, which tmux set-environment on the default socket injects correctly
// for ALL subsequent new-sessions in that server).
//
// Returns the argv log path.
func setupStubClaudeOnPATH(t *testing.T, home string) string {
	t.Helper()
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("setupStubClaudeOnPATH: mkdir binDir: %v", err)
	}
	writeStubClaudeBinary(t, binDir)
	stubAbs := filepath.Join(binDir, "claude")
	argvLog := filepath.Join(home, "claude-argv.log")

	// Write an empty argv log file so readers see a sentinel rather than
	// ENOENT before the stub first runs.
	if err := os.WriteFile(argvLog, nil, 0o644); err != nil {
		t.Fatalf("setupStubClaudeOnPATH: init argvLog: %v", err)
	}

	// Configure [claude] command = <abs stub> via the user config under the
	// isolated HOME. This is read by GetClaudeCommand() at dispatch time.
	cfgPath := filepath.Join(home, ".agent-deck", "config.toml")
	cfgBody := "[claude]\ncommand = \"" + stubAbs + "\"\n"
	if err := os.WriteFile(cfgPath, []byte(cfgBody), 0o644); err != nil {
		t.Fatalf("setupStubClaudeOnPATH: write config.toml: %v", err)
	}
	ClearUserConfigCache()
	t.Cleanup(func() { ClearUserConfigCache() })

	// [Deviation Rule 3 — Blocking fix] GetClaudeConfigDir() at claude.go:234
	// short-circuits to the CLAUDE_CONFIG_DIR env var when set. On this
	// executor host (and on any real user's machine) that env var is
	// pre-set to the user's real ~/.claude — which poisons
	// sessionHasConversationData() by pointing it at the real home's
	// projects/ dir instead of the isolated HOME. We unset it for the
	// duration of the test so GetClaudeConfigDir() falls through to the
	// os.UserHomeDir() default under our isolated HOME. The Plan 01
	// isolatedHomeDir helper only sets HOME; this helper layers the
	// CLAUDE_CONFIG_DIR unset that the dispatch tests require.
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	// Set env var on THIS test process. Go's exec inherits; tmux binary
	// sees it. But tmux new-session does not propagate to the server's
	// pane env on the default socket, so also inject via tmux
	// set-environment -g (global) on the default socket as a belt-and-
	// suspenders path — the stub resolves AGENTDECK_TEST_ARGV_LOG inside
	// the pane's env.
	t.Setenv("AGENTDECK_TEST_ARGV_LOG", argvLog)
	// Best-effort: set it globally on the default tmux socket so new
	// sessions inherit. Errors ignored (no-op if tmux is absent or server
	// is unreachable — tests that need it call requireTmux() first).
	_ = exec.Command("tmux", "set-environment", "-g", "AGENTDECK_TEST_ARGV_LOG", argvLog).Run()
	// Register a cleanup to unset the global tmux env var so it does not
	// leak across tests or into the user's interactive sessions.
	t.Cleanup(func() {
		_ = exec.Command("tmux", "set-environment", "-g", "-u", "AGENTDECK_TEST_ARGV_LOG").Run()
	})
	return argvLog
}

// writeSyntheticJSONLTranscript writes a 2-line synthetic Claude JSONL
// transcript at ~/.claude/projects/<ConvertToClaudeDirName(ProjectPath)>/<ClaudeSessionID>.jsonl
// under the isolated HOME. Each line embeds a literal "sessionId" field so
// sessionHasConversationData() returns true (it greps for `"sessionId"`).
// Returns the full transcript path. Registers t.Cleanup to remove it.
func writeSyntheticJSONLTranscript(t *testing.T, home string, inst *Instance) string {
	t.Helper()
	projectDirName := ConvertToClaudeDirName(inst.ProjectPath)
	dir := filepath.Join(home, ".claude", "projects", projectDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("writeSyntheticJSONLTranscript: mkdir projects: %v", err)
	}
	path := filepath.Join(dir, inst.ClaudeSessionID+".jsonl")
	// sessionHasConversationData scans for the literal substring `"sessionId"`.
	// Embedding it in each line guarantees a real-conversation signal.
	lines := []map[string]interface{}{
		{"sessionId": inst.ClaudeSessionID, "role": "user", "content": "hello"},
		{"sessionId": inst.ClaudeSessionID, "role": "assistant", "content": "hi back"},
	}
	var buf []byte
	for _, ln := range lines {
		b, err := json.Marshal(ln)
		if err != nil {
			t.Fatalf("writeSyntheticJSONLTranscript: marshal jsonl: %v", err)
		}
		buf = append(buf, b...)
		buf = append(buf, '\n')
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("writeSyntheticJSONLTranscript: write jsonl: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

// TestPersistence_FreshSessionUsesSessionIDNotResume pins REQ-2 fresh-session
// contract: buildClaudeResumeCommand() on an Instance with no JSONL transcript
// MUST produce "claude --session-id <id>", NOT "claude --resume <id>". Passing
// --resume for a non-existent conversation id causes claude to exit with
// "No conversation found".
//
// Per CONTEXT.md FAIL-or-PASS qualifier: current v1.5.1 code at
// internal/session/instance.go:4150 routes this correctly via
// sessionHasConversationData() — so this test PASSES today as a regression
// guard. The unambiguous failure message below protects against future
// regressions that would flip the branch. This test does NOT exercise the
// Start() dispatch path (TEST-06 does); it guards the helper contract only.
func TestPersistence_FreshSessionUsesSessionIDNotResume(t *testing.T) {
	home := isolatedHomeDir(t)
	inst := newClaudeInstanceForDispatch(t, home)
	// NO JSONL transcript — fresh session.

	cmdLine := inst.buildClaudeResumeCommand()

	if !strings.Contains(cmdLine, "--session-id "+inst.ClaudeSessionID) {
		t.Fatalf("TEST-08: buildClaudeResumeCommand() with NO JSONL transcript MUST use '--session-id %s'. This prevents 'No conversation found' errors on first start. Got: %q", inst.ClaudeSessionID, cmdLine)
	}
	if strings.Contains(cmdLine, "--resume") {
		t.Fatalf("TEST-08: buildClaudeResumeCommand() must NOT use --resume for a fresh session (no JSONL transcript exists at ~/.claude/projects/<hash>/<id>.jsonl). Got: %q", cmdLine)
	}
}

// requireTmux skips the current test if the tmux binary is not on PATH. The
// skip message contains "no tmux available:" so CI log scrapers can detect a
// vacuous-skip regression.
func requireTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skipf("no tmux available: %v", err)
	}
}

// TestPersistence_RestartResumesConversation pins REQ-2 Restart() contract:
// when a JSONL transcript exists for the instance's ClaudeSessionID,
// inst.Restart() MUST spawn "claude --resume <id>", NOT "claude --session-id
// <new-uuid>".
//
// Driven via the REAL Restart() dispatch path (internal/session/instance.go
// line 3763). Stub claude on PATH captures argv to AGENTDECK_TEST_ARGV_LOG.
//
// Per CONTEXT.md FAIL-or-PASS qualifier and the dispatch_path_analysis in
// 03-PLAN.md: current v1.5.1 code at instance.go:3789 correctly routes
// Restart() through buildClaudeResumeCommand() — so this test may PASS
// today. Phase 3's REQ-2 fix lives in Start() (TEST-06), not Restart(). This
// test is kept as a REGRESSION GUARD: any future change that breaks
// Restart()'s resume routing (e.g. removing the `i.ClaudeSessionID != ""`
// check at line 3788) will fail this test. Either outcome (PASS now, FAIL
// after regression) is acceptable; the unambiguous failure message below
// prevents silent breakage.
func TestPersistence_RestartResumesConversation(t *testing.T) {
	requireTmux(t)
	home := isolatedHomeDir(t)
	argvLog := setupStubClaudeOnPATH(t, home)
	inst := newClaudeInstanceForDispatch(t, home)

	// First bring the tmux session up so Restart()'s respawn-pane branch
	// (instance.go:3788 — requires tmuxSession.Exists()) is taken.
	//
	// IMPORTANT: Start() at instance.go:566-567 MINTS A NEW UUID via
	// generateUUID() and overwrites inst.ClaudeSessionID with it. Any JSONL
	// transcript written BEFORE Start() points at a stale UUID that is no
	// longer inst.ClaudeSessionID by the time Restart() runs. To pin
	// Restart()'s resume routing, the JSONL must be written AFTER Start()
	// completes, under the post-Start ClaudeSessionID. This mirrors the
	// 2026-04-14 production scenario: a real Claude session ran to the point
	// of writing a transcript, then tmux was SIGKILLed; on restart, Claude
	// finds a JSONL matching its current session UUID.
	if err := inst.Start(); err != nil {
		t.Fatalf("setup: inst.Start: %v", err)
	}
	time.Sleep(500 * time.Millisecond) // let initial argv be written

	// Now write the synthetic JSONL for the POST-Start ClaudeSessionID so
	// sessionHasConversationData() returns true when Restart() consults it.
	jsonlPath := writeSyntheticJSONLTranscript(t, home, inst)

	// Reset the argv log so the subsequent Restart's argv is the only entry.
	if err := os.WriteFile(argvLog, nil, 0o644); err != nil {
		t.Fatalf("truncate argvLog: %v", err)
	}

	if err := inst.Restart(); err != nil {
		t.Fatalf("Restart: %v", err)
	}

	argv := readCapturedClaudeArgv(t, argvLog, 3*time.Second)
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "--resume") || !strings.Contains(joined, inst.ClaudeSessionID) {
		t.Fatalf("TEST-05 RED: after Restart() with JSONL transcript at %s, captured claude argv must contain '--resume %s'. Got argv: %v", jsonlPath, inst.ClaudeSessionID, argv)
	}
}

// TestPersistence_StartAfterSIGKILLResumesConversation is the core REQ-2 RED
// test. Models the 2026-04-14 incident: tmux server is SIGKILLed by an SSH
// logout, instance transitions to Status=error, user runs "agent-deck session
// start" — which calls inst.Start() (cmd/agent-deck/session_cmd.go:188).
//
// The CONTRACT: Start() on an Instance with a populated ClaudeSessionID and
// JSONL transcript MUST spawn "claude --resume <id>", NOT a new UUID.
//
// Per dispatch_path_analysis in 03-PLAN.md: current v1.5.1 Start()
// (instance.go:1873) calls buildClaudeCommand() at line 1883, which runs
// through the capture-resume pattern (line 550+) that generates a brand-new
// UUID and spawns "claude --session-id <NEW_UUID>". It does NOT consult
// i.ClaudeSessionID. So this test FAILS RED on current code.
//
// Phase 3's REQ-2 fix: route Start() through buildClaudeResumeCommand() when
// IsClaudeCompatible(i.Tool) && i.ClaudeSessionID != "" — mirroring the
// Restart() code path at line 3789.
func TestPersistence_StartAfterSIGKILLResumesConversation(t *testing.T) {
	requireTmux(t)
	home := isolatedHomeDir(t)
	argvLog := setupStubClaudeOnPATH(t, home)
	inst := newClaudeInstanceForDispatch(t, home)
	// Simulate the post-SIGKILL state transition.
	inst.Status = StatusError
	writeSyntheticJSONLTranscript(t, home, inst)

	if err := inst.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	argv := readCapturedClaudeArgv(t, argvLog, 3*time.Second)
	joined := strings.Join(argv, " ")

	if !strings.Contains(joined, "--resume") || !strings.Contains(joined, inst.ClaudeSessionID) {
		t.Fatalf("TEST-06 RED: after inst.Start() with Status=StatusError, ClaudeSessionID=%s, and JSONL transcript present, captured claude argv must contain '--resume %s'. Got argv: %v. This is the 2026-04-14 incident REQ-2 root cause: Start() dispatches through buildClaudeCommand (instance.go:1883) instead of buildClaudeResumeCommand. Phase 3 must fix this.", inst.ClaudeSessionID, inst.ClaudeSessionID, argv)
	}
	if strings.Contains(joined, "--session-id") && !strings.Contains(joined, inst.ClaudeSessionID) {
		t.Fatalf("TEST-06 RED: Start() minted a NEW session UUID instead of resuming ClaudeSessionID=%s. Argv: %v", inst.ClaudeSessionID, argv)
	}
}

// TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion pins the
// invariant from docs/session-id-lifecycle.md: instance JSON is the
// authoritative ClaudeSessionID source. The hook sidecar at
// ~/.agent-deck/hooks/<id>.sid is a read-only fallback for hook-event
// processing (updateSessionIDFromHook at instance.go:2626) — it is NOT
// consulted by Start() or Restart() dispatch. Deleting the sidecar MUST NOT
// affect the claude --resume command produced by a session start.
//
// Flow:
//  1. Write sidecar at ~/.agent-deck/hooks/<id>.sid with ClaudeSessionID.
//  2. Delete the sidecar (simulates corruption or cleanup incident).
//  3. Call inst.Start() with a JSONL transcript present.
//  4. Assert captured claude argv contains "--resume <ClaudeSessionID>".
//
// Per dispatch_path_analysis in 03-PLAN.md: this test FAILS RED on current
// v1.5.1 for the SAME root cause as TEST-06 (Start() bypasses resume).
// After Phase 3 fixes Start() to route through buildClaudeResumeCommand,
// this test will GREEN because ClaudeSessionID is read from the Instance
// struct (which mirrors instance JSON storage), NOT from the sidecar.
func TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion(t *testing.T) {
	requireTmux(t)
	home := isolatedHomeDir(t)
	argvLog := setupStubClaudeOnPATH(t, home)
	inst := newClaudeInstanceForDispatch(t, home)

	sidecarPath := filepath.Join(home, ".agent-deck", "hooks", inst.ID+".sid")
	if got := HookSessionAnchorPath(inst.ID); got != sidecarPath {
		t.Fatalf("sidecar path mismatch: got %q want %q — isolatedHomeDir HOME override may not have propagated", got, sidecarPath)
	}
	if err := os.MkdirAll(filepath.Dir(sidecarPath), 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	if err := os.WriteFile(sidecarPath, []byte(inst.ClaudeSessionID+"\n"), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}
	if _, err := os.Stat(sidecarPath); err != nil {
		t.Fatalf("setup: sidecar not written: %v", err)
	}

	if err := os.Remove(sidecarPath); err != nil {
		t.Fatalf("delete sidecar: %v", err)
	}
	if _, err := os.Stat(sidecarPath); !os.IsNotExist(err) {
		t.Fatalf("setup: sidecar still present after delete: err=%v", err)
	}
	if inst.ClaudeSessionID == "" {
		t.Fatalf("TEST-07 RED: ClaudeSessionID was cleared when sidecar was deleted; expected instance-JSON to remain authoritative")
	}

	writeSyntheticJSONLTranscript(t, home, inst)

	if err := inst.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	argv := readCapturedClaudeArgv(t, argvLog, 3*time.Second)
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "--resume") || !strings.Contains(joined, inst.ClaudeSessionID) {
		t.Fatalf("TEST-07 RED: after deleting hook sidecar at %s, inst.Start() must still spawn 'claude --resume %s' because ClaudeSessionID lives in instance storage, not the sidecar. Got argv: %v. Root cause: Start() bypasses buildClaudeResumeCommand — same as TEST-06. Phase 3 fix will make both tests GREEN.", sidecarPath, inst.ClaudeSessionID, argv)
	}
}

// TestPersistence_ExplicitOptOutHonoredOnLinux pins PERSIST-03: an explicit
// `launch_in_user_scope = false` in config.toml MUST always return false,
// even on a Linux+systemd host where the new default (Plan 02) would
// otherwise return true. This closes the gap that TEST-02 alone does not
// cover — TEST-02 spawns tmux directly with the field, never exercising
// the config-load path.
//
// Skip semantics: requireSystemdRun skips cleanly on non-systemd hosts
// with "no systemd-run available:" in the message.
//
// Four-arm structure:
//   - Arm 1: empty config → default fires → MUST be true on Linux+systemd.
//   - Arm 2: explicit false → MUST stay false even though default would be true.
//   - Arm 3: explicit true → MUST be true everywhere.
//   - Arm 4: direct *bool pointer-state assertions (W2 checker fix). Locks
//     the decoder contract at the field level so a future refactor cannot
//     silently erase the nil-vs-zero distinction.
//
// RED note: against current code (LaunchInUserScope is a plain bool), arm 1
// fails AND arm 4 fails to compile (settings.LaunchInUserScope != nil is a
// type error). After Plan 02 Task 2 flips the default and migrates the
// field to *bool, this test goes GREEN across all four arms.
func TestPersistence_ExplicitOptOutHonoredOnLinux(t *testing.T) {
	requireSystemdRun(t)

	// Arm 1: empty config → default fires → MUST be true on Linux+systemd.
	// This is the same assertion as TEST-03 but co-located so a regression
	// is caught here too.
	t.Run("empty_config_defaults_true", func(t *testing.T) {
		home := isolatedHomeDir(t)
		cfg := filepath.Join(home, ".agent-deck", "config.toml")
		if err := os.WriteFile(cfg, []byte(""), 0o644); err != nil {
			t.Fatalf("write empty config: %v", err)
		}
		ClearUserConfigCache()
		resetSystemdDetectionCacheForTest()
		if got := GetTmuxSettings().GetLaunchInUserScope(); !got {
			t.Fatalf("EXPLICIT-OPT-OUT-RED arm1: empty config on Linux+systemd returned false, want true (default flip not in)")
		}
	})

	// Arm 2: explicit false → MUST stay false even though default would be true.
	t.Run("explicit_false_overrides_default", func(t *testing.T) {
		home := isolatedHomeDir(t)
		cfg := filepath.Join(home, ".agent-deck", "config.toml")
		body := "[tmux]\nlaunch_in_user_scope = false\n"
		if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
			t.Fatalf("write config.toml: %v", err)
		}
		ClearUserConfigCache()
		resetSystemdDetectionCacheForTest()
		if got := GetTmuxSettings().GetLaunchInUserScope(); got {
			t.Fatalf("EXPLICIT-OPT-OUT-RED arm2: GetLaunchInUserScope()=true with explicit false override; PERSIST-03 violated")
		}
	})

	// Arm 3: explicit true → MUST be true everywhere (sanity for the symmetry).
	t.Run("explicit_true_overrides", func(t *testing.T) {
		home := isolatedHomeDir(t)
		cfg := filepath.Join(home, ".agent-deck", "config.toml")
		body := "[tmux]\nlaunch_in_user_scope = true\n"
		if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
			t.Fatalf("write config.toml: %v", err)
		}
		ClearUserConfigCache()
		resetSystemdDetectionCacheForTest()
		if got := GetTmuxSettings().GetLaunchInUserScope(); !got {
			t.Fatalf("EXPLICIT-OPT-OUT-RED arm3: explicit true override returned false")
		}
	})

	// Arm 4: direct *bool pointer-state assertions. Locks the decoder
	// contract at the field level so a future refactor cannot silently
	// erase the nil-vs-zero distinction. This is the W2 fix from the
	// checker review — three sub-assertions (4a/4b/4c) prove the decoder
	// honors nil/false/true at the field level, not just via the getter.
	t.Run("pointer_state_locked", func(t *testing.T) {
		// 4a: empty config → field absent → pointer MUST be nil.
		home := isolatedHomeDir(t)
		cfg := filepath.Join(home, ".agent-deck", "config.toml")
		if err := os.WriteFile(cfg, []byte(""), 0o644); err != nil {
			t.Fatalf("write empty config: %v", err)
		}
		ClearUserConfigCache()
		resetSystemdDetectionCacheForTest()
		settings := GetTmuxSettings()
		if settings.LaunchInUserScope != nil {
			t.Fatalf("EXPLICIT-OPT-OUT-RED arm4a: expected nil pointer for absent field, got non-nil pointing to %v", *settings.LaunchInUserScope)
		}

		// 4b: explicit false → pointer MUST be non-nil pointing to false.
		home = isolatedHomeDir(t)
		cfg = filepath.Join(home, ".agent-deck", "config.toml")
		if err := os.WriteFile(cfg, []byte("[tmux]\nlaunch_in_user_scope = false\n"), 0o644); err != nil {
			t.Fatalf("write explicit-false config: %v", err)
		}
		ClearUserConfigCache()
		settings = GetTmuxSettings()
		if settings.LaunchInUserScope == nil {
			t.Fatalf("EXPLICIT-OPT-OUT-RED arm4b: expected non-nil pointer for explicit false, got nil")
		}
		if *settings.LaunchInUserScope != false {
			t.Fatalf("EXPLICIT-OPT-OUT-RED arm4b: expected *false, got *%v", *settings.LaunchInUserScope)
		}

		// 4c: explicit true → pointer MUST be non-nil pointing to true.
		home = isolatedHomeDir(t)
		cfg = filepath.Join(home, ".agent-deck", "config.toml")
		if err := os.WriteFile(cfg, []byte("[tmux]\nlaunch_in_user_scope = true\n"), 0o644); err != nil {
			t.Fatalf("write explicit-true config: %v", err)
		}
		ClearUserConfigCache()
		settings = GetTmuxSettings()
		if settings.LaunchInUserScope == nil {
			t.Fatalf("EXPLICIT-OPT-OUT-RED arm4c: expected non-nil pointer for explicit true, got nil")
		}
		if *settings.LaunchInUserScope != true {
			t.Fatalf("EXPLICIT-OPT-OUT-RED arm4c: expected *true, got *%v", *settings.LaunchInUserScope)
		}
	})
}
