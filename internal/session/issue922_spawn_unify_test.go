// Issue #922 — CLI session restart and TUI R diverged at the spawn-env
// construction point: Restart()'s respawn-pane branch did NOT call
// prepareWorkerScratchConfigDirForSpawn, while the recreate fallback DID.
// Combined with the silent worker-scratch override of the resolved
// CLAUDE_CONFIG_DIR, a user who configured per-group config_dir saw
// claude land on the WRONG account with no log line to debug it.
//
// Bug reporter: @bautrey, audit pass 2026-05-10.
//
// Fix shape per the issue:
//  1. All restart sub-paths must funnel the worker-scratch decision through
//     the same call site (prepareSpawnConfigForRestart helper, invoked once
//     at the top of Restart()).
//  2. When worker-scratch overrides the resolved CLAUDE_CONFIG_DIR, emit a
//     single INFO log line with both the resolved dir and the scratch dir
//     so the override is debuggable instead of silent.

package session

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrepareSpawnConfigForRestart_PreparesWorkerScratch locks the
// unification half of the fix: the helper that Restart() calls at its
// top must populate WorkerScratchConfigDir for any non-conductor claude
// worker on a host with a telegram conductor — regardless of whether
// the eventual restart will take the respawn-pane path or the recreate
// fallback.
func TestPrepareSpawnConfigForRestart_PreparesWorkerScratch(t *testing.T) {
	withTelegramConductorPresent(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	profile := filepath.Join(home, ".claude")
	if err := os.MkdirAll(profile, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profile, "settings.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", profile)

	inst := &Instance{
		ID:          "00000000-0000-0000-0000-000000000922",
		Tool:        "claude",
		Title:       "issue-922",
		ProjectPath: filepath.Join(home, "proj"),
	}
	if inst.WorkerScratchConfigDir != "" {
		t.Fatalf("setup: WorkerScratchConfigDir should start empty; got %q", inst.WorkerScratchConfigDir)
	}

	inst.prepareSpawnConfigForRestart()

	if inst.WorkerScratchConfigDir == "" {
		t.Fatal("prepareSpawnConfigForRestart did not populate WorkerScratchConfigDir; both restart sub-paths will diverge again (issue #922)")
	}
	// Defensive: the scratch dir must actually exist on disk — otherwise
	// the spawn would fail with ENOENT.
	if _, err := os.Stat(inst.WorkerScratchConfigDir); err != nil {
		t.Fatalf("scratch dir not on disk: %v", err)
	}
}

// TestBuildClaudeCommand_WorkerScratchOverrideEmitsInfoLog locks the
// debuggability half of the fix: when WorkerScratchConfigDir overrides
// the resolved CLAUDE_CONFIG_DIR, an INFO line must record both the
// original resolved dir and the scratch dir so the override is no
// longer silent.
func TestBuildClaudeCommand_WorkerScratchOverrideEmitsInfoLog(t *testing.T) {
	withTelegramConductorPresent(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	profile := filepath.Join(home, ".claude")
	if err := os.MkdirAll(profile, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profile, "settings.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", profile)

	inst := &Instance{
		ID:          "00000000-0000-0000-0000-000000000922",
		Tool:        "claude",
		Title:       "issue-922",
		ProjectPath: filepath.Join(home, "proj"),
	}
	scratch, err := inst.EnsureWorkerScratchConfigDir(profile)
	if err != nil {
		t.Fatalf("setup scratch: %v", err)
	}
	if scratch == "" {
		t.Fatal("setup: expected non-empty scratch dir")
	}
	inst.WorkerScratchConfigDir = scratch

	// Capture sessionLog at INFO level for this test.
	var buf bytes.Buffer
	origLog := sessionLog
	sessionLog = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	t.Cleanup(func() { sessionLog = origLog })

	_ = inst.buildClaudeCommand("claude")

	output := buf.String()
	if !strings.Contains(output, "worker_scratch_override") {
		t.Errorf("expected `worker_scratch_override` INFO event; got: %s", output)
	}
	if !strings.Contains(output, profile) {
		t.Errorf("override log must include the resolved (overridden) config_dir %q; got: %s", profile, output)
	}
	if !strings.Contains(output, scratch) {
		t.Errorf("override log must include the scratch dir %q; got: %s", scratch, output)
	}
	// `buildClaudeCommand` chains two spawn-env contributors that each
	// emit `CLAUDE_CONFIG_DIR` (the bash-export prefix and the inline
	// prefix). Both legitimately go through applyWorkerScratchOverride
	// and each one should log — anything between 1 and 2 is OK.
	if got := strings.Count(output, "worker_scratch_override"); got < 1 || got > 2 {
		t.Errorf("expected 1 or 2 `worker_scratch_override` logs per build; got %d\noutput: %s", got, output)
	}
}

// TestBuildClaudeCommand_NoOverrideNoLog guards against a noisy fix:
// when WorkerScratchConfigDir is empty (conductor / channel-owner /
// non-claude-worker), no override log must fire.
func TestBuildClaudeCommand_NoOverrideNoLog(t *testing.T) {
	withTelegramConductorPresent(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	profile := filepath.Join(home, ".claude")
	if err := os.MkdirAll(profile, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profile, "settings.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", profile)

	// Conductor — keeps ambient profile, never gets a scratch dir.
	inst := &Instance{
		ID:          "00000000-0000-0000-0000-000000000a00",
		Tool:        "claude",
		Title:       "conductor-x",
		ProjectPath: filepath.Join(home, "proj"),
	}

	var buf bytes.Buffer
	origLog := sessionLog
	sessionLog = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	t.Cleanup(func() { sessionLog = origLog })

	_ = inst.buildClaudeCommand("claude")

	if strings.Contains(buf.String(), "worker_scratch_override") {
		t.Errorf("override log must not fire when WorkerScratchConfigDir is empty; got: %s", buf.String())
	}
}
