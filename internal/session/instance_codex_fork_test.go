package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"al.essio.dev/pkg/shellescape"

	"github.com/asheshgoplani/agent-deck/internal/statedb"
)

func seedCodexRollout(t *testing.T, codexHome, sid string) {
	t.Helper()
	dir := filepath.Join(codexHome, "sessions", "2026", "06", "06")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir rollout dir: %v", err)
	}
	p := filepath.Join(dir, "rollout-20260606T000000-"+sid+".jsonl")
	if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}
}

func TestCanForkCodex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	sid := "11111111-2222-3333-4444-555555555555"
	seedCodexRollout(t, home, sid)

	inst := NewInstanceWithTool("cx", "/tmp/p", "codex")
	inst.CodexSessionID = sid
	inst.CodexBindingConfirmed = true
	inst.CodexDetectedAt = time.Now()
	if !inst.CanForkCodex() {
		t.Fatal("codex session with an on-disk rollout must be forkable")
	}

	inst.CodexSessionID = "no-rollout-uuid"
	if inst.CanForkCodex() {
		t.Fatal("codex session without a rollout must NOT be forkable")
	}
}

func TestCreateForkedCodexInstance_UsesWorktreeAndForkCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	sid := "11111111-2222-3333-4444-555555555555"
	seedCodexRollout(t, home, sid)

	parent := NewInstanceWithTool("cx parent", "/tmp/original", "codex")
	parent.CodexSessionID = sid
	parent.CodexBindingConfirmed = true
	parent.CodexDetectedAt = time.Now()

	opts := &ClaudeOptions{
		WorkDir:          "/tmp/original-wt",
		WorktreePath:     "/tmp/original-wt",
		WorktreeRepoRoot: "/tmp/original",
		WorktreeBranch:   "fork/cx-parent",
	}
	forked, cmd, err := parent.CreateForkedCodexInstanceWithOptions("cx parent (fork)", "", opts)
	if err != nil {
		t.Fatalf("CreateForkedCodexInstanceWithOptions: %v", err)
	}
	if forked.ProjectPath != "/tmp/original-wt" {
		t.Fatalf("forked ProjectPath = %q, want worktree dir", forked.ProjectPath)
	}
	if forked.WorktreePath != "/tmp/original-wt" || forked.WorktreeBranch != "fork/cx-parent" {
		t.Fatalf("worktree metadata not copied: %+v", forked)
	}
	if !forked.IsForkAwaitingStart || forked.ForkStartCommand == "" {
		t.Fatal("codex fork must defer launch via ForkStartCommand/IsForkAwaitingStart (Pi pattern)")
	}
	if !strings.Contains(cmd, "fork "+sid) {
		t.Fatalf("fork command must run `codex fork <parent-sid>`; got: %s", cmd)
	}
}

func TestCreateForkedCodexInstance_NativeIsolationArgv(t *testing.T) {
	for _, client := range []struct {
		name string
		help string
		flag string
	}{
		{"supported", "Options:\n  --no-daemon  Bypass an existing daemon\n", "--no-daemon\n"},
		{"older", "Options:\n  --model <MODEL>\n", ""},
	} {
		t.Run(client.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			isolateConfigHomeXDG(t)
			t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
			binDir, logPath, _ := codexIsolationClient(t, client.help)
			command := "CODEX_TEST_SCOPE=fork " + shellescape.Quote(filepath.Join(binDir, "codex"))
			t.Cleanup(resetUserConfigCache(t, &UserConfig{Codex: CodexSettings{Command: command, YoloMode: true}}))
			const parentID = "11111111-2222-4333-8444-555555555555"
			seedCodexRollout(t, os.Getenv("CODEX_HOME"), parentID)
			parent := NewInstanceWithTool("native parent", home, "codex")
			parent.Command = "codex"
			parent.CodexSessionID = parentID
			parent.CodexBindingConfirmed = true
			if err := parent.ApplyLaunchReasoningEffort("high"); err != nil {
				t.Fatal(err)
			}
			forked, built, err := parent.CreateForkedCodexInstanceWithOptions("native fork", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			want := client.flag + "--yolo\n--config\nmodel_reasoning_effort=high\nfork\n" + parentID
			if got := runBuiltCommandCapturingArgv(t, built, binDir, logPath); got != want {
				t.Fatalf("native fork argv = %q, want %q; command: %s", got, want, built)
			}

			// A restart goes through the shared normal builder and must resume
			// the child's current native conversation, not replay the fork.
			const childID = "66666666-7777-4888-8999-aaaaaaaaaaaa"
			seedCodexRollout(t, os.Getenv("CODEX_HOME"), childID)
			forked.CodexSessionID = childID
			forked.CodexBindingConfirmed = true
			built = forked.buildCodexCommand(forked.Command)
			want = client.flag + "--yolo\n--config\nmodel_reasoning_effort=high\nresume\n" + childID
			if got := runBuiltCommandCapturingArgv(t, built, binDir, logPath); got != want {
				t.Fatalf("native fork restart argv = %q, want %q; command: %s", got, want, built)
			}
		})
	}
}

func TestCreateForkedCodexInstance_ExactResumeParentRestartsChildNativeArgv(t *testing.T) {
	for _, source := range []string{"explicit", "configured"} {
		t.Run(source, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			isolateConfigHomeXDG(t)
			t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
			binDir, logPath, _ := codexIsolationClient(t, "Options:\n  --no-daemon\n")
			const parentID = "11111111-2222-4333-8444-555555555555"
			const childID = "66666666-7777-4888-8999-aaaaaaaaaaaa"
			command := "CODEX_TEST_SCOPE=fork " + shellescape.Quote(filepath.Join(binDir, "codex")) + " resume " + parentID
			cfg := &UserConfig{}
			t.Cleanup(resetUserConfigCache(t, cfg))
			seedCodexRollout(t, os.Getenv("CODEX_HOME"), parentID)
			parent := NewInstanceWithTool("exact parent", home, "codex")
			parent.Command = command
			if source == "configured" {
				cfg.Codex.Command = command
				parent.Command = "codex"
			}
			parent.CodexSessionID = parentID
			parent.CodexBindingConfirmed = true
			forked, built, err := parent.CreateForkedCodexInstanceWithOptions("exact fork", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := runBuiltCommandCapturingArgv(t, built, binDir, logPath); got != "--no-daemon\nfork\n"+parentID {
				t.Fatalf("exact-parent fork argv = %q, want isolated fork of parent UUID", got)
			}
			seedCodexRollout(t, os.Getenv("CODEX_HOME"), childID)
			forked.CodexSessionID = childID
			forked.CodexBindingConfirmed = true
			built = forked.buildCodexCommand(forked.Command)
			if got := runBuiltCommandCapturingArgv(t, built, binDir, logPath); got != "--no-daemon\nresume\n"+childID {
				t.Fatalf("fork restart argv = %q, want isolated resume of child UUID", got)
			}
		})
	}
}

// TestCreateForkedCodexInstance_RejectsParentBindingDuringStartup reproduces
// the live rename collision from pr-review-and-merge 3. While `codex fork`
// initializes, the child process temporarily has the parent's rollout open.
// Process-FD discovery must not bind that parent ID to the new Agent Deck row,
// because an immediate explicit rename would then rename both the parent and
// the fork through the shared native thread ID.
func TestCreateForkedCodexInstance_RejectsParentBindingDuringStartup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	parentID := "12121212-2222-4333-8444-555555555555"
	seedCodexRollout(t, home, parentID)

	parent := NewInstanceWithTool("parent", t.TempDir(), "codex")
	parent.CodexSessionID = parentID
	parent.CodexBindingConfirmed = true
	parent.CodexDetectedAt = time.Now()
	forked, _, err := parent.CreateForkedCodexInstanceWithOptions("fork", "", nil)
	if err != nil {
		t.Fatalf("CreateForkedCodexInstanceWithOptions: %v", err)
	}

	if changed := forked.acceptCodexSessionID(parentID, false, codexBindingProcess); changed {
		t.Fatal("fork startup accepted the parent's Codex session ID")
	}
	if forked.CodexSessionID != "" {
		t.Fatalf("fork CodexSessionID = %q, want empty until the new child thread exists", forked.CodexSessionID)
	}
}

// TestCodexBootstrapExcludesPersistedBindings reproduces the ordinary-session
// form of the live collision: a new Codex process starts in the same cwd as an
// already-running session whose rollout is still being written. The older
// session's durable binding must be excluded even when its tmux environment is
// missing CODEX_SESSION_ID, otherwise the new row temporarily adopts the old
// thread and title synchronization renames the wrong session.
func TestCodexBootstrapExcludesPersistedBindings(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	db := withTempGlobalStateDB(t)

	oldID := "13131313-2222-4333-8444-555555555555"
	newID := "14141414-2222-4333-8444-555555555555"
	projectPath := t.TempDir()
	oldPath := writeCodexRolloutWithSource(t, codexHome, oldID, projectPath, "cli")
	newPath := writeCodexRolloutWithSource(t, codexHome, newID, projectPath, "cli")
	now := time.Now()
	if err := os.Chtimes(newPath, now.Add(-time.Second), now.Add(-time.Second)); err != nil {
		t.Fatalf("age new rollout: %v", err)
	}
	if err := os.Chtimes(oldPath, now, now); err != nil {
		t.Fatalf("refresh old rollout: %v", err)
	}

	if err := db.SaveInstance(&statedb.InstanceRow{
		ID:          "existing-agent-deck-row",
		Title:       "existing",
		ProjectPath: projectPath,
		GroupPath:   "test",
		Tool:        "codex",
		Status:      "running",
		CreatedAt:   now.Add(-time.Hour),
		ToolData:    json.RawMessage(`{"codex_session_id":"` + oldID + `"}`),
	}); err != nil {
		t.Fatalf("persist existing Codex binding: %v", err)
	}

	created := NewInstanceWithTool("new", projectPath, "codex")
	created.CodexStartedAt = now.Add(-time.Minute).UnixMilli()
	created.UpdateHookStatus(ownedCodexHook(t, created, &HookStatus{
		Status:    "running",
		SessionID: oldID,
		Event:     "SessionStart",
		UpdatedAt: time.Now(),
	}))
	if created.CodexSessionID != "" {
		t.Fatalf("CodexSessionID = %q after owned candidate, want empty", created.CodexSessionID)
	}
	created.UpdateCodexSession(nil)
	if created.CodexSessionID != "" {
		t.Fatalf("bootstrap claimed an unproven same-cwd thread: %q", created.CodexSessionID)
	}
}

func TestCreateForkedCodexInstance_PreservesReasoningEffort(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	sid := "11111111-2222-3333-4444-555555555555"
	seedCodexRollout(t, home, sid)

	parent := NewInstanceWithTool("cx parent", "/tmp/original", "codex")
	parent.CodexSessionID = sid
	parent.CodexBindingConfirmed = true
	parent.CodexDetectedAt = time.Now()
	if err := parent.ApplyLaunchReasoningEffort("high"); err != nil {
		t.Fatalf("ApplyLaunchReasoningEffort: %v", err)
	}

	forked, cmd, err := parent.CreateForkedCodexInstanceWithOptions("cx fork", "", nil)
	if err != nil {
		t.Fatalf("CreateForkedCodexInstanceWithOptions: %v", err)
	}
	if got := forked.LaunchReasoningEffort(); got != "high" {
		t.Fatalf("forked LaunchReasoningEffort() = %q, want high", got)
	}
	if !strings.Contains(cmd, "--config model_reasoning_effort=high") {
		t.Fatalf("fork command missing reasoning effort: %s", cmd)
	}
}

func TestCreateForkedCodexInstance_UsesConfiguredCodexHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("CODEX_HOME", "")
	ClearUserConfigCache()
	t.Cleanup(ClearUserConfigCache)

	codexHome := filepath.Join(home, "codex work")
	cfg := &UserConfig{Codex: CodexSettings{Command: `CODEX_HOME="` + codexHome + `" codex`}}
	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}
	ClearUserConfigCache()

	sid := "aaaaaaaa-2222-3333-4444-555555555555"
	seedCodexRollout(t, codexHome, sid)

	parent := NewInstanceWithTool("cx parent", "/tmp/original", "codex")
	parent.CodexSessionID = sid
	parent.CodexBindingConfirmed = true
	parent.CodexDetectedAt = time.Now()

	_, cmd, err := parent.CreateForkedCodexInstanceWithOptions("cx parent (fork)", "", nil)
	if err != nil {
		t.Fatalf("CreateForkedCodexInstanceWithOptions: %v", err)
	}
	want := `CODEX_HOME="` + codexHome + `" codex `
	if !strings.Contains(cmd, want) || !strings.HasSuffix(cmd, " fork "+sid) {
		t.Fatalf("configured CODEX_HOME command must be preserved for fork; want %q in %q", want, cmd)
	}
}

func TestCreateForkedCodexInstance_QuotesConfiguredCodexConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("CODEX_HOME", "")
	ClearUserConfigCache()
	t.Cleanup(ClearUserConfigCache)

	codexHome := filepath.Join(home, "codex config dir")
	cfg := &UserConfig{Codex: CodexSettings{ConfigDir: codexHome}}
	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}
	ClearUserConfigCache()

	sid := "bbbbbbbb-2222-3333-4444-555555555555"
	seedCodexRollout(t, codexHome, sid)

	parent := NewInstanceWithTool("cx parent", "/tmp/original", "codex")
	parent.CodexSessionID = sid
	parent.CodexBindingConfirmed = true
	parent.CodexDetectedAt = time.Now()

	_, cmd, err := parent.CreateForkedCodexInstanceWithOptions("cx parent (fork)", "", nil)
	if err != nil {
		t.Fatalf("CreateForkedCodexInstanceWithOptions: %v", err)
	}
	want := "CODEX_HOME=" + shellescape.Quote(codexHome) + " "
	if !strings.Contains(cmd, want) {
		t.Fatalf("configured [codex].config_dir must be shell-quoted for fork; want %q in %q", want, cmd)
	}
}

func TestCreateForkedCodexInstance_PreservesCompatibleToolIdentity(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("CODEX_HOME", codexHome)
	ClearUserConfigCache()
	t.Cleanup(ClearUserConfigCache)

	cfg := &UserConfig{
		Tools: map[string]ToolDef{
			"my-codex": {
				Command:        "codex-wrapper",
				CompatibleWith: "codex",
			},
		},
	}
	if err := SaveUserConfig(cfg); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}
	ClearUserConfigCache()

	sid := "cccccccc-2222-3333-4444-555555555555"
	seedCodexRollout(t, codexHome, sid)

	parent := NewInstanceWithTool("cx parent", "/tmp/original", "my-codex")
	parent.Command = "codex-wrapper"
	parent.CodexSessionID = sid
	parent.CodexBindingConfirmed = true
	parent.CodexDetectedAt = time.Now()

	forked, cmd, err := parent.CreateForkedCodexInstanceWithOptions("cx parent (fork)", "", nil)
	if err != nil {
		t.Fatalf("CreateForkedCodexInstanceWithOptions: %v", err)
	}
	if forked.Tool != "my-codex" {
		t.Fatalf("forked Tool = %q, want custom Codex-compatible tool identity", forked.Tool)
	}
	if !strings.Contains(cmd, "AGENTDECK_TOOL=my-codex") {
		t.Fatalf("fork command must preserve AGENTDECK_TOOL identity; got %q", cmd)
	}
	if !strings.Contains(cmd, "codex-wrapper ") || !strings.HasSuffix(cmd, " fork "+sid) {
		t.Fatalf("fork command must use the compatible tool command; got %q", cmd)
	}
}

// TestCreateForkedCodexInstance_ShellQuotesEnvPrefix guards the PR #1299 review
// finding: a user-editable session title containing shell metacharacters must be
// shell-quoted in the generated fork launch command, not Go-%q-quoted (which would
// still allow $(...) / backtick expansion under a shell).
func TestCreateForkedCodexInstance_ShellQuotesEnvPrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	sid := "dddddddd-2222-3333-4444-555555555555"
	seedCodexRollout(t, home, sid)

	parent := NewInstanceWithTool("cx parent", "/tmp/original", "codex")
	parent.CodexSessionID = sid
	parent.CodexBindingConfirmed = true
	parent.CodexDetectedAt = time.Now()

	evil := "pwn $(touch /tmp/agentdeck-pwn)"
	_, cmd, err := parent.CreateForkedCodexInstanceWithOptions(evil, "", nil)
	if err != nil {
		t.Fatalf("CreateForkedCodexInstanceWithOptions: %v", err)
	}
	if !strings.Contains(cmd, "AGENTDECK_TITLE="+shellescape.Quote(evil)) {
		t.Fatalf("AGENTDECK_TITLE must be shell-quoted via shellescape.Quote; got: %s", cmd)
	}
}
