package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPerGroupConfig_CustomCommandGetsGroupConfigDir locks CFG-02: a
// custom-command (wrapper-script) claude session spawn command must export
// CLAUDE_CONFIG_DIR from the group's Claude config_dir override.
//
// Expected RED against base fa9971e: buildClaudeCommandWithMessage returns
// baseCommand unchanged at instance.go:596 when baseCommand != "claude",
// so CLAUDE_CONFIG_DIR never reaches the wrapper's exec env.
func TestPerGroupConfig_CustomCommandGetsGroupConfigDir(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	origProfile := os.Getenv("AGENTDECK_PROFILE")
	origClaudeDir := os.Getenv("CLAUDE_CONFIG_DIR")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
		if origProfile != "" {
			_ = os.Setenv("AGENTDECK_PROFILE", origProfile)
		} else {
			_ = os.Unsetenv("AGENTDECK_PROFILE")
		}
		if origClaudeDir != "" {
			_ = os.Setenv("CLAUDE_CONFIG_DIR", origClaudeDir)
		} else {
			_ = os.Unsetenv("CLAUDE_CONFIG_DIR")
		}
		ClearUserConfigCache()
	})

	_ = os.Setenv("HOME", tmpHome)
	_ = os.Unsetenv("CLAUDE_CONFIG_DIR")
	_ = os.Unsetenv("AGENTDECK_PROFILE")

	agentDeckDir := filepath.Join(tmpHome, ".agent-deck")
	if err := os.MkdirAll(agentDeckDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := `
[groups."conductor".claude]
config_dir = "~/.claude-work"
`
	if err := os.WriteFile(filepath.Join(agentDeckDir, "config.toml"), []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	ClearUserConfigCache()

	inst := NewInstanceWithGroupAndTool("conductor-x", "/tmp/p", "conductor", "claude")
	wrapper := "/tmp/start-conductor.sh"
	cmd := inst.buildClaudeCommand(wrapper)

	wantDir := filepath.Join(tmpHome, ".claude-work")
	if !strings.Contains(cmd, "CLAUDE_CONFIG_DIR="+wantDir) {
		t.Errorf("custom-command spawn missing CLAUDE_CONFIG_DIR=%s\ngot: %s", wantDir, cmd)
	}
	if !strings.HasSuffix(cmd, wrapper) {
		t.Errorf("spawn must end with wrapper path %q, got: %s", wrapper, cmd)
	}
}

// TestPerGroupConfig_GroupOverrideBeatsProfile locks CFG-04 test 2: when
// both a profile-level and group-level Claude config_dir are set, the group
// value wins in the spawn command for a custom-command session.
//
// Expected RED against base fa9971e (same root cause as test 1).
func TestPerGroupConfig_GroupOverrideBeatsProfile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	origProfile := os.Getenv("AGENTDECK_PROFILE")
	origEnvDir := os.Getenv("CLAUDE_CONFIG_DIR")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
		if origProfile != "" {
			_ = os.Setenv("AGENTDECK_PROFILE", origProfile)
		} else {
			_ = os.Unsetenv("AGENTDECK_PROFILE")
		}
		if origEnvDir != "" {
			_ = os.Setenv("CLAUDE_CONFIG_DIR", origEnvDir)
		} else {
			_ = os.Unsetenv("CLAUDE_CONFIG_DIR")
		}
		ClearUserConfigCache()
	})

	_ = os.Setenv("HOME", tmpHome)
	_ = os.Unsetenv("CLAUDE_CONFIG_DIR")
	_ = os.Setenv("AGENTDECK_PROFILE", "work")

	agentDeckDir := filepath.Join(tmpHome, ".agent-deck")
	if err := os.MkdirAll(agentDeckDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := `
[profiles.work.claude]
config_dir = "~/.claude-work"

[groups."conductor".claude]
config_dir = "~/.claude-group"
`
	if err := os.WriteFile(filepath.Join(agentDeckDir, "config.toml"), []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	ClearUserConfigCache()

	inst := NewInstanceWithGroupAndTool("c", "/tmp/p", "conductor", "claude")
	cmd := inst.buildClaudeCommand("/tmp/wrapper.sh")

	wantGroup := filepath.Join(tmpHome, ".claude-group")
	if !strings.Contains(cmd, "CLAUDE_CONFIG_DIR="+wantGroup) {
		t.Errorf("group override must beat profile; want CLAUDE_CONFIG_DIR=%s, got: %s", wantGroup, cmd)
	}
	profilePath := filepath.Join(tmpHome, ".claude-work")
	if strings.Contains(cmd, "CLAUDE_CONFIG_DIR="+profilePath) {
		t.Errorf("profile path leaked into spawn despite group override; got: %s", cmd)
	}
}

// TestPerGroupConfig_UnknownGroupFallsThroughToProfile locks CFG-04 test 3:
// an unknown group name resolves to the profile-level Claude config_dir.
//
// Expected GREEN immediately against base fa9971e (resolver already correct
// per PR #578).
func TestPerGroupConfig_UnknownGroupFallsThroughToProfile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	origProfile := os.Getenv("AGENTDECK_PROFILE")
	origEnvDir := os.Getenv("CLAUDE_CONFIG_DIR")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
		if origProfile != "" {
			_ = os.Setenv("AGENTDECK_PROFILE", origProfile)
		} else {
			_ = os.Unsetenv("AGENTDECK_PROFILE")
		}
		if origEnvDir != "" {
			_ = os.Setenv("CLAUDE_CONFIG_DIR", origEnvDir)
		} else {
			_ = os.Unsetenv("CLAUDE_CONFIG_DIR")
		}
		ClearUserConfigCache()
	})

	_ = os.Setenv("HOME", tmpHome)
	_ = os.Unsetenv("CLAUDE_CONFIG_DIR")
	_ = os.Setenv("AGENTDECK_PROFILE", "work")

	agentDeckDir := filepath.Join(tmpHome, ".agent-deck")
	if err := os.MkdirAll(agentDeckDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := `
[profiles.work.claude]
config_dir = "~/.claude-work"

[groups."real-group".claude]
config_dir = "~/.claude-real-group"
`
	if err := os.WriteFile(filepath.Join(agentDeckDir, "config.toml"), []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	ClearUserConfigCache()

	got := GetClaudeConfigDirForGroup("does-not-exist")
	want := filepath.Join(tmpHome, ".claude-work")
	if got != want {
		t.Errorf("unknown group should fall through to profile: got=%s want=%s", got, want)
	}
}

// TestPerGroupConfig_CacheInvalidation locks CFG-04 test 6: rewriting the
// on-disk config.toml followed by ClearUserConfigCache() causes the resolver
// to return the new value (or the default when the override is removed).
//
// Expected GREEN immediately against base fa9971e.
func TestPerGroupConfig_CacheInvalidation(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	origProfile := os.Getenv("AGENTDECK_PROFILE")
	origEnvDir := os.Getenv("CLAUDE_CONFIG_DIR")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
		if origProfile != "" {
			_ = os.Setenv("AGENTDECK_PROFILE", origProfile)
		} else {
			_ = os.Unsetenv("AGENTDECK_PROFILE")
		}
		if origEnvDir != "" {
			_ = os.Setenv("CLAUDE_CONFIG_DIR", origEnvDir)
		} else {
			_ = os.Unsetenv("CLAUDE_CONFIG_DIR")
		}
		ClearUserConfigCache()
	})

	_ = os.Setenv("HOME", tmpHome)
	_ = os.Unsetenv("CLAUDE_CONFIG_DIR")
	_ = os.Unsetenv("AGENTDECK_PROFILE")

	agentDeckDir := filepath.Join(tmpHome, ".agent-deck")
	configPath := filepath.Join(agentDeckDir, "config.toml")
	if err := os.MkdirAll(agentDeckDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// v1: group override present
	v1 := `[groups."g".claude]
config_dir = "~/.claude-g"
`
	if err := os.WriteFile(configPath, []byte(v1), 0o600); err != nil {
		t.Fatalf("write v1 config: %v", err)
	}
	ClearUserConfigCache()
	if got, want := GetClaudeConfigDirForGroup("g"), filepath.Join(tmpHome, ".claude-g"); got != want {
		t.Fatalf("v1: got %s want %s", got, want)
	}

	// v2: group override removed; cache must be cleared to pick up the change
	v2 := "# empty config\n"
	if err := os.WriteFile(configPath, []byte(v2), 0o600); err != nil {
		t.Fatalf("write v2 config: %v", err)
	}
	ClearUserConfigCache()
	got := GetClaudeConfigDirForGroup("g")
	want := filepath.Join(tmpHome, ".claude") // default when no override
	if got != want {
		t.Errorf("after cache invalidation, got=%s want=%s", got, want)
	}
}
