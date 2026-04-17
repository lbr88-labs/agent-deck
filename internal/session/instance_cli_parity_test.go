package session

// Failing tests for issue #610: `agent-deck list --json` and
// `agent-deck session show <id> --json` report wrong status (idle/waiting)
// for sessions that the TUI and web API /api/menu correctly report as
// running.
//
// Root cause trace (see .planning/fix-issue-610/PLAN.md for the full
// data-flow analysis):
//
//   TUI   backgroundStatusUpdate → tmux.RefreshPaneInfoCache()
//                                → hookWatcher.GetHookStatus + UpdateHookStatus
//                                → inst.UpdateStatus()           ← title fast-path hits
//   Web   SessionDataService.refreshStatuses → tmux.RefreshPaneInfoCache()
//                                → defaultLoadHookStatuses + UpdateHookStatus
//                                → inst.UpdateStatus()           ← title fast-path hits
//   CLI   handleList / handleShow → inst.UpdateStatus()          ← cache cold, hook stale
//
// When Claude is mid-tool-execution the only reliable running-state signal
// is the braille spinner embedded in the pane title (set by Claude Code via
// OSC sequences). The TUI/web populate the pane-title cache before each
// UpdateStatus tick; the CLI never does. As a result the CLI's GetStatus
// falls through the title fast path, the hook fast path is stale (>2min,
// because Claude only emits "running" on UserPromptSubmit), and content-scan
// on the bottom of the pane misses the busy indicator for long tool calls.
//
// Fix surface: introduce session.RefreshInstancesForCLIStatus(instances)
// — the CLI analogue of SessionDataService.refreshStatuses — and call it
// from handleList and the session-show JSON emitter before the UpdateStatus
// loop. Until that helper exists this file fails to compile, which is the
// intended red state for TDD.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/tmux"
)

// writeHookFile writes a hook status file under the test-scoped HOME so
// readHookStatusFile picks it up via GetHooksDir().
func writeHookFile(t *testing.T, instanceID, status string, tsSecondsAgo int) {
	t.Helper()
	hooksDir := GetHooksDir()
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	ts := time.Now().Add(-time.Duration(tsSecondsAgo) * time.Second).Unix()
	body := fmt.Sprintf(
		`{"status":%q,"session_id":"sess-610","event":"UserPromptSubmit","ts":%d}`,
		status, ts,
	)
	path := filepath.Join(hooksDir, instanceID+".json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write hook: %v", err)
	}
}

// setPaneTitle injects a pane title via `tmux select-pane -T`. Claude Code
// normally does this with OSC escape sequences while it is actively working;
// the tests fake the same state.
func setPaneTitle(t *testing.T, tmuxSession, title string) {
	t.Helper()
	cmd := exec.Command("tmux", "select-pane", "-t", tmuxSession, "-T", title)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("select-pane -T %q: %v\n%s", title, err, out)
	}
}

// TestUpdateStatus_CLIParity_SpinnerTitle_StaleHook reproduces the core
// symptom of issue #610: when the hook fast path is stale and the pane
// title carries a braille spinner (Claude "working" signal), the CLI cold
// path must still report StatusRunning.
//
// Required behavior after fix:
//
//	RefreshInstancesForCLIStatus(instances) warms the title cache (and loads
//	hook files from disk) so the subsequent UpdateStatus sees the spinner
//	via the title fast-path — identical to what the TUI and web already do.
func TestUpdateStatus_CLIParity_SpinnerTitle_StaleHook(t *testing.T) {
	// Requires only a live tmux server; TestMain bootstraps one, so skip
	// only when tmux is entirely missing. This was the F3 silent-skip trap.
	skipIfNoTmuxBinary(t)

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	inst := NewInstanceWithTool("issue610-spinner", tmpHome, "claude")
	if err := inst.tmuxSession.Start("sleep 3600"); err != nil {
		t.Fatalf("tmux start: %v", err)
	}
	defer func() { _ = inst.tmuxSession.Kill() }()

	// Stale running hook (3 minutes old, past the 2-minute fast-path window).
	// Matches real-world behavior: Claude only emits "running" on
	// UserPromptSubmit, so a long-running tool call leaves the hook stale.
	writeHookFile(t, inst.ID, "running", 180)

	// Simulate Claude's OSC title sequence while working.
	setPaneTitle(t, inst.tmuxSession.Name, "⠋ Working on refactor")

	// Past the 1.5-second grace period inside UpdateStatus.
	time.Sleep(2 * time.Second)

	// CLI entry point: the fix must expose a helper that parallels
	// SessionDataService.refreshStatuses and Home.backgroundStatusUpdate,
	// and handleList / session-show must call it before the UpdateStatus
	// loop. Until then this symbol does not exist and the file will not
	// compile — the intended TDD red.
	RefreshInstancesForCLIStatus([]*Instance{inst})

	if err := inst.UpdateStatus(); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if got := inst.GetStatusThreadSafe(); got != StatusRunning {
		t.Errorf(
			"issue #610: CLI cold path reported %q, want %q. "+
				"Pane title carries a braille spinner; TUI and web API report "+
				"\"running\" for this exact state.",
			got, StatusRunning,
		)
	}
}

// TestUpdateStatus_CLIvsTUIParity_SameTmuxState verifies that the CLI and
// TUI paths produce the same Status for a session in the same underlying
// tmux state. Direct parity assertion: scripts that consume `list --json`
// must see the same answer the TUI shows on screen.
//
// Both entry points run against a single tmux session:
//   - TUI path:   tmux.RefreshPaneInfoCache + UpdateHookStatus + UpdateStatus
//   - CLI path:   RefreshInstancesForCLIStatus + UpdateStatus
//
// On main the CLI path has no equivalent to RefreshPaneInfoCache, so even
// once the helper exists the two must produce identical output for the
// same pane state. The fix lands green only when CLI output == TUI output.
func TestUpdateStatus_CLIvsTUIParity_SameTmuxState(t *testing.T) {
	// See TestUpdateStatus_CLIParity_SpinnerTitle_StaleHook for rationale.
	skipIfNoTmuxBinary(t)

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Shared tmux state: create the session once. Two Instance wrappers
	// point at the same tmux session name, mirroring the real world where
	// the TUI and CLI are separate processes loading the same sessions.json.
	base := NewInstanceWithTool("issue610-parity", tmpHome, "claude")
	if err := base.tmuxSession.Start("sleep 3600"); err != nil {
		t.Fatalf("tmux start: %v", err)
	}
	defer func() { _ = base.tmuxSession.Kill() }()

	writeHookFile(t, base.ID, "running", 180)
	setPaneTitle(t, base.tmuxSession.Name, "⠴ Running tool")
	time.Sleep(2 * time.Second)

	tuiInst := reloadInstanceForParityTest(base)
	cliInst := reloadInstanceForParityTest(base)

	// Run CLI path FIRST so the tmux pane-info cache is cold — mirrors a
	// real agent-deck list invocation where the CLI is a fresh OS process
	// with its own (empty) tmux package globals. If the TUI path ran first
	// in this binary it would warm the package-level cache and mask the
	// parity gap.
	//
	// --- CLI path (handleList / session show --json) ---
	RefreshInstancesForCLIStatus([]*Instance{cliInst})
	if err := cliInst.UpdateStatus(); err != nil {
		t.Fatalf("CLI UpdateStatus: %v", err)
	}
	cliStatus := cliInst.GetStatusThreadSafe()

	// --- TUI path (internal/ui/home.go:backgroundStatusUpdate) ---
	tmux.RefreshPaneInfoCache()
	if hs := readHookStatusFile(tuiInst.ID); hs != nil {
		tuiInst.UpdateHookStatus(hs)
	}
	if err := tuiInst.UpdateStatus(); err != nil {
		t.Fatalf("TUI UpdateStatus: %v", err)
	}
	tuiStatus := tuiInst.GetStatusThreadSafe()

	if tuiStatus != StatusRunning {
		// Sanity: if the TUI path itself does not report Running on this
		// setup, the test oracle is wrong — bail before trusting the parity
		// check.
		t.Fatalf(
			"test oracle broken: TUI path did not report running for a "+
				"spinner-title session; got %q. Check tmux.RefreshPaneInfoCache "+
				"and the AnalyzePaneTitle contract before blaming CLI parity.",
			tuiStatus,
		)
	}

	if cliStatus != tuiStatus {
		t.Errorf(
			"issue #610 parity break: TUI path=%q, CLI path=%q for the same "+
				"tmux state. list --json must match /api/menu.",
			tuiStatus, cliStatus,
		)
	}
}

// reloadInstanceForParityTest constructs a second Instance wrapper pointing
// at the same underlying tmux session as base — simulates what
// ReconnectSessionLazy does across process boundaries (TUI vs CLI as
// separate OS processes reading the same sessions.json).
func reloadInstanceForParityTest(base *Instance) *Instance {
	reloaded := &Instance{
		ID:          base.ID,
		Title:       base.Title,
		ProjectPath: base.ProjectPath,
		GroupPath:   base.GroupPath,
		Tool:        base.Tool,
		Status:      StatusIdle,
		CreatedAt:   time.Now().Add(-10 * time.Second), // past grace window
	}
	reloaded.tmuxSession = tmux.ReconnectSessionLazy(
		base.tmuxSession.Name,
		base.Title,
		base.ProjectPath,
		"claude",
		"idle",
	)
	reloaded.tmuxSession.InstanceID = base.ID
	return reloaded
}
