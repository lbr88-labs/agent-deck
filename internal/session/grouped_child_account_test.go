package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression tests for the wrong-account / group-config_dir-not-applied bug:
// a child session launched with `-g <group>` from inside a running session
// must pin its GROUP's Claude config_dir, NOT silently inherit the caller's
// ambient (and possibly stale) CLAUDE_CONFIG_DIR.
//
// Root cause: resolveClaudeConfigDir's GROUP chain ranked the ambient env var
// ABOVE the group's config_dir ("Group chain: env wins"), so whenever a
// child's config_dir was resolved without a fully-populated *Instance the
// stale ambient account silently won. The fix makes group beat env in the
// group chain too, mirroring the instance chain (#881).
//
// Each test pairs:
//   - a GROUP-chain assertion (RED before the fix, GREEN after) that proves
//     the precedence change directly, and
//   - an INSTANCE-chain bake assertion that locks the end-to-end spawn command
//     so the contract can never regress.

func writeGroupedChildConfig(t *testing.T, tmpHome, body string) {
	t.Helper()
	agentDeckDir := filepath.Join(tmpHome, ".agent-deck")
	if err := os.MkdirAll(agentDeckDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDeckDir, "config.toml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	ClearUserConfigCache()
}

// TestGroupedChild_ConfigDirBeatsAmbient is the repro test from the bug report:
// a group with config_dir = ~/.claude-buddii, an ambient CLAUDE_CONFIG_DIR
// pointing at ~/.claude-work, and a `-g <group>` child. The child must bake
// the GROUP's dir (buddii), never the ambient one (work).
func TestGroupedChild_ConfigDirBeatsAmbient(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("AGENTDECK_PROFILE", "")
	t.Cleanup(ClearUserConfigCache)

	buddiiDir := filepath.Join(tmpHome, ".claude-buddii")
	workDir := filepath.Join(tmpHome, ".claude-work")
	// Stale ambient = the WRONG account (work), exactly as inherited from a
	// conductor pane whose CLAUDE_CONFIG_DIR points elsewhere.
	t.Setenv("CLAUDE_CONFIG_DIR", workDir)

	writeGroupedChildConfig(t, tmpHome, `
[groups.ryan.claude]
config_dir = "~/.claude-buddii"
`)

	// GROUP-chain assertion: RED before the fix (returned work/"env"),
	// GREEN after (group beats ambient env).
	path, source := GetClaudeConfigDirSourceForGroup("ryan")
	if source != "group" {
		t.Errorf("group chain source = %q, want %q (group config_dir must beat ambient CLAUDE_CONFIG_DIR)", source, "group")
	}
	if path != buddiiDir {
		t.Errorf("group chain path = %q, want %q", path, buddiiDir)
	}

	// INSTANCE-chain bake assertion: the actual spawn command for a `-g ryan`
	// child must export CLAUDE_CONFIG_DIR=<buddii> and never the ambient work.
	inst := NewInstanceWithGroupAndTool("ab-triggers", filepath.Join(tmpHome, "proj"), "ryan", "claude")
	cmd := inst.buildClaudeCommand("claude")
	if !strings.Contains(cmd, "CLAUDE_CONFIG_DIR="+buddiiDir) {
		t.Errorf("grouped child spawn missing CLAUDE_CONFIG_DIR=%s\ngot: %s", buddiiDir, cmd)
	}
	if strings.Contains(cmd, "CLAUDE_CONFIG_DIR="+workDir) {
		t.Errorf("grouped child leaked the ambient (wrong-account) CLAUDE_CONFIG_DIR=%s\ngot: %s", workDir, cmd)
	}
}

// TestGroupedChild_SwitchAccountSpawnUsesGroup proves switch-account keeps the
// spawn-env consistent: after a parent session is switched A->B (so the live
// pane's ambient CLAUDE_CONFIG_DIR points at B's dir), a grouped child it
// spawns must still resolve to the GROUP's config_dir — not the parent's
// switched account, and not the stale ambient.
func TestGroupedChild_SwitchAccountSpawnUsesGroup(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("AGENTDECK_PROFILE", "")
	t.Cleanup(ClearUserConfigCache)

	buddiiDir := filepath.Join(tmpHome, ".claude-buddii")
	workDir := filepath.Join(tmpHome, ".claude-work")

	writeGroupedChildConfig(t, tmpHome, `
[profiles.work.claude]
config_dir = "~/.claude-work"

[profiles.buddii.claude]
config_dir = "~/.claude-buddii"

[groups.ryan.claude]
config_dir = "~/.claude-buddii"
`)

	// Model the parent AFTER `switch-account <parent> work`: the live pane's
	// ambient CLAUDE_CONFIG_DIR now points at work — stale from the child's
	// point of view (the child's group says buddii).
	t.Setenv("CLAUDE_CONFIG_DIR", workDir)

	// The child as launched via `agent-deck launch -g ryan` from inside that
	// parent: GroupPath set, Account empty (launch never sets Account).
	child := NewInstanceWithGroupAndTool("ab-test-fixes", filepath.Join(tmpHome, "proj"), "ryan", "claude")
	if child.Account != "" {
		t.Fatalf("precondition: launched child must have empty Account, got %q", child.Account)
	}

	// GROUP-chain assertion: RED before the fix (ambient work won), GREEN after.
	gpath, gsource := GetClaudeConfigDirSourceForGroup("ryan")
	if gsource != "group" || gpath != buddiiDir {
		t.Errorf("group chain = (%q,%q), want (%q,%q): child must follow its group, not the parent's switched/stale ambient account",
			gpath, gsource, buddiiDir, "group")
	}

	// INSTANCE-chain bake assertion: the child's spawn command pins buddii.
	cmd := child.buildClaudeCommand("claude")
	if !strings.Contains(cmd, "CLAUDE_CONFIG_DIR="+buddiiDir) {
		t.Errorf("post-switch grouped child missing CLAUDE_CONFIG_DIR=%s\ngot: %s", buddiiDir, cmd)
	}
	if strings.Contains(cmd, "CLAUDE_CONFIG_DIR="+workDir) {
		t.Errorf("post-switch grouped child inherited stale ambient CLAUDE_CONFIG_DIR=%s\ngot: %s", workDir, cmd)
	}
}
