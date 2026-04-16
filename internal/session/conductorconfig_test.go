package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupConductorTest prepares HOME, AGENTDECK_PROFILE, CLAUDE_CONFIG_DIR for
// an isolated per-test config.toml under $HOME/.agent-deck/. Returns tmpHome
// and registers a t.Cleanup that restores env + clears the user-config cache.
//
// Mirrors the pergroupconfig_test.go setup pattern (lines 22-58) but factored
// into a shared helper since CFG-11 has eight tests rather than six.
func setupConductorTest(t *testing.T) string {
	t.Helper()
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
		t.Fatalf("mkdir agent-deck dir: %v", err)
	}
	ClearUserConfigCache()
	return tmpHome
}

// writeConductorConfig writes a config.toml and clears the user-config cache.
func writeConductorConfig(t *testing.T, tmpHome, body string) {
	t.Helper()
	path := filepath.Join(tmpHome, ".agent-deck", "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	ClearUserConfigCache()
}

// TestConductorConfig_SchemaParses locks CFG-11 test 1: the nested
// [conductors.<name>.claude] TOML block parses into
// UserConfig.Conductors[<name>].Claude.{ConfigDir,EnvFile} and the
// Get helpers return the expanded value (or literal when already absolute).
func TestConductorConfig_SchemaParses(t *testing.T) {
	tmpHome := setupConductorTest(t)
	writeConductorConfig(t, tmpHome, `
[conductors.foo.claude]
config_dir = "/tmp/x"
env_file = "/tmp/y"

[conductors.bar.claude]
config_dir = "~/conductor-x"
`)

	cfg, err := LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	if cfg == nil {
		t.Fatalf("LoadUserConfig returned nil cfg")
	}

	fooBlock, ok := cfg.Conductors["foo"]
	if !ok {
		t.Fatalf("Conductors[\"foo\"] missing; got map: %+v", cfg.Conductors)
	}
	if fooBlock.Claude.ConfigDir != "/tmp/x" {
		t.Errorf("foo.Claude.ConfigDir = %q, want %q", fooBlock.Claude.ConfigDir, "/tmp/x")
	}
	if fooBlock.Claude.EnvFile != "/tmp/y" {
		t.Errorf("foo.Claude.EnvFile = %q, want %q", fooBlock.Claude.EnvFile, "/tmp/y")
	}

	if got := cfg.GetConductorClaudeConfigDir("foo"); got != "/tmp/x" {
		t.Errorf("GetConductorClaudeConfigDir(foo) = %q, want %q", got, "/tmp/x")
	}

	// ~-expansion path
	wantBar := filepath.Join(tmpHome, "conductor-x")
	if got := cfg.GetConductorClaudeConfigDir("bar"); got != wantBar {
		t.Errorf("GetConductorClaudeConfigDir(bar) = %q, want %q", got, wantBar)
	}
}

// TestConductorConfig_PrecedenceConductorBeatsGroup locks CFG-11 test 2:
// when both [conductors.<name>.claude] and [groups."conductor".claude] set
// config_dir, the conductor block wins for a conductor-<name> Instance.
func TestConductorConfig_PrecedenceConductorBeatsGroup(t *testing.T) {
	tmpHome := setupConductorTest(t)
	writeConductorConfig(t, tmpHome, `
[conductors.foo.claude]
config_dir = "/tmp/conductor-wins"

[groups."conductor".claude]
config_dir = "/tmp/group-loses"
`)

	inst := NewInstanceWithGroupAndTool("conductor-foo", tmpHome, "conductor", "claude")
	got := GetClaudeConfigDirForInstance(inst)
	if got != "/tmp/conductor-wins" {
		t.Errorf("GetClaudeConfigDirForInstance(conductor-foo) = %q, want %q", got, "/tmp/conductor-wins")
	}
}

// TestConductorConfig_PrecedenceEnvBeatsConductor locks CFG-11 test 3:
// the CLAUDE_CONFIG_DIR env var beats a [conductors.<name>.claude] override.
func TestConductorConfig_PrecedenceEnvBeatsConductor(t *testing.T) {
	tmpHome := setupConductorTest(t)
	_ = os.Setenv("CLAUDE_CONFIG_DIR", "/tmp/env-wins")
	// setupConductorTest's t.Cleanup already restores CLAUDE_CONFIG_DIR.

	writeConductorConfig(t, tmpHome, `
[conductors.foo.claude]
config_dir = "/tmp/conductor-loses"
`)

	inst := NewInstanceWithGroupAndTool("conductor-foo", tmpHome, "conductor", "claude")
	got := GetClaudeConfigDirForInstance(inst)
	if got != "/tmp/env-wins" {
		t.Errorf("GetClaudeConfigDirForInstance with CLAUDE_CONFIG_DIR set = %q, want %q", got, "/tmp/env-wins")
	}
}

// TestConductorConfig_FallsThroughToGroupOverride locks CFG-11 test 4:
// when only [groups."conductor".claude] is set (no [conductors.*] block),
// the loader still resolves via the group chain — backward compat with PR #578.
func TestConductorConfig_FallsThroughToGroupOverride(t *testing.T) {
	tmpHome := setupConductorTest(t)
	writeConductorConfig(t, tmpHome, `
[groups."conductor".claude]
config_dir = "/tmp/group-fallback"
`)

	inst := NewInstanceWithGroupAndTool("conductor-foo", tmpHome, "conductor", "claude")
	got := GetClaudeConfigDirForInstance(inst)
	if got != "/tmp/group-fallback" {
		t.Errorf("GetClaudeConfigDirForInstance (group-only) = %q, want %q", got, "/tmp/group-fallback")
	}
}

// TestConductorConfig_FallsThroughToProfile locks CFG-11 test 5:
// when only [profiles.<p>.claude] is set (no conductor, no group), the
// loader falls through to the profile override.
func TestConductorConfig_FallsThroughToProfile(t *testing.T) {
	tmpHome := setupConductorTest(t)
	_ = os.Setenv("AGENTDECK_PROFILE", "personal")
	writeConductorConfig(t, tmpHome, `
[profiles.personal.claude]
config_dir = "/tmp/profile-fallback"
`)

	inst := NewInstanceWithGroupAndTool("conductor-foo", tmpHome, "conductor", "claude")
	got := GetClaudeConfigDirForInstance(inst)
	if got != "/tmp/profile-fallback" {
		t.Errorf("GetClaudeConfigDirForInstance (profile fallback) = %q, want %q", got, "/tmp/profile-fallback")
	}
}

// TestConductorConfig_PropagatesToConductorGroupSession locks CFG-11 test 6
// end-to-end via THREE sub-assertions: normal-claude spawn, custom-command
// spawn, AND the resume path (buildClaudeResumeCommand at instance.go:4172).
// The resume sub-assertion protects milestone success criterion #8 (restart
// gsd-v154 conductor → reports ~/.claude-work).
func TestConductorConfig_PropagatesToConductorGroupSession(t *testing.T) {
	tmpHome := setupConductorTest(t)
	writeConductorConfig(t, tmpHome, `
[conductors.foo.claude]
config_dir = "/tmp/x"
`)

	// 6a — normal-claude spawn path (instance.go:501)
	inst := NewInstanceWithGroupAndTool("conductor-foo", tmpHome, "conductor", "claude")
	cmdNormal := inst.buildClaudeCommand("claude")
	if !strings.Contains(cmdNormal, "CLAUDE_CONFIG_DIR=/tmp/x") {
		t.Errorf("6a normal-claude spawn missing CLAUDE_CONFIG_DIR=/tmp/x\ngot: %s", cmdNormal)
	}

	// 6b — custom-command spawn path (instance.go:606)
	cmdCustom := inst.buildClaudeCommand("/tmp/wrapper.sh")
	if !strings.Contains(cmdCustom, "export CLAUDE_CONFIG_DIR=/tmp/x;") {
		t.Errorf("6b custom-command spawn missing `export CLAUDE_CONFIG_DIR=/tmp/x;`\ngot: %s", cmdCustom)
	}

	// 6c — resume/restart path (instance.go:4172) — NEW, protects milestone #8.
	cmdResume := inst.buildClaudeResumeCommand()
	if !strings.Contains(cmdResume, "CLAUDE_CONFIG_DIR=/tmp/x") {
		t.Errorf("6c buildClaudeResumeCommand missing CLAUDE_CONFIG_DIR=/tmp/x (milestone #8 broken)\ngot: %s", cmdResume)
	}
}

// TestConductorConfig_EnvFileSourced locks CFG-11 test 7:
// [conductors.<name>.claude].env_file is sourced before claude exec on both
// the normal-claude and custom-command spawn paths. Includes a runtime
// proof (bash -c harness) mirroring TestPerGroupConfig_EnvFileSourcedInSpawn.
func TestConductorConfig_EnvFileSourced(t *testing.T) {
	tmpHome := setupConductorTest(t)
	envrcPath := filepath.Join(tmpHome, "conductor-envrc")
	if err := os.WriteFile(envrcPath, []byte("export CONDUCTOR_ENVFILE_SENTINEL=present\n"), 0o600); err != nil {
		t.Fatalf("write envrc: %v", err)
	}
	writeConductorConfig(t, tmpHome, fmt.Sprintf(`
[conductors.foo.claude]
env_file = "%s"
`, envrcPath))

	wantSource := `source "` + envrcPath + `"`

	// Normal-claude spawn path
	instNormal := NewInstanceWithGroupAndTool("conductor-foo", tmpHome, "conductor", "claude")
	cmdNormal := instNormal.buildClaudeCommand("claude")
	if !strings.Contains(cmdNormal, wantSource) {
		t.Errorf("normal-claude spawn missing env_file source line\nwant substring: %s\ngot: %s", wantSource, cmdNormal)
	}

	// Custom-command spawn path
	instCustom := NewInstanceWithGroupAndTool("conductor-foo", tmpHome, "conductor", "claude")
	instCustom.Command = "bash -c 'exec claude'"
	cmdCustom := instCustom.buildClaudeCommand(instCustom.Command)
	if !strings.Contains(cmdCustom, wantSource) {
		t.Errorf("custom-command spawn missing env_file source line\nwant substring: %s\ngot: %s", wantSource, cmdCustom)
	}

	// Runtime proof on custom-command path — executes the sourced envrc and
	// prints the sentinel. Only runs if the source line appeared above.
	if strings.Contains(cmdCustom, wantSource) {
		idx := strings.LastIndex(cmdCustom, "bash -c 'exec claude'")
		if idx == -1 {
			t.Fatalf("runtime proof: could not locate custom-command payload; got: %s", cmdCustom)
		}
		harness := cmdCustom[:idx] + `echo "$CONDUCTOR_ENVFILE_SENTINEL"`
		out, err := exec.Command("bash", "-c", harness).CombinedOutput()
		if err != nil {
			t.Fatalf("runtime proof bash exec failed: %v\noutput: %s\nharness: %s", err, string(out), harness)
		}
		got := strings.TrimSpace(string(out))
		if got != "present" {
			t.Errorf("runtime proof: env_file not sourced\nwant CONDUCTOR_ENVFILE_SENTINEL=present, got %q\nharness: %s", got, harness)
		}
	}
}

// TestConductorConfig_SourceLabelIsConductor locks CFG-11 test 8:
// GetClaudeConfigDirSourceForInstance returns ("conductor", <path>) when
// the value comes from the conductor block. Also asserts the parallel
// regression — a non-conductor Instance with only a group override gets
// source="group" (conductor branch does NOT shadow group for non-conductor
// sessions).
func TestConductorConfig_SourceLabelIsConductor(t *testing.T) {
	tmpHome := setupConductorTest(t)
	writeConductorConfig(t, tmpHome, `
[conductors.foo.claude]
config_dir = "/tmp/x"

[groups."real-group".claude]
config_dir = "/tmp/group-for-non-conductor"
`)

	// Conductor Instance — source must be "conductor"
	instConductor := NewInstanceWithGroupAndTool("conductor-foo", tmpHome, "conductor", "claude")
	path, source := GetClaudeConfigDirSourceForInstance(instConductor)
	if source != "conductor" {
		t.Errorf("conductor Instance source = %q, want %q", source, "conductor")
	}
	if path != "/tmp/x" {
		t.Errorf("conductor Instance path = %q, want %q", path, "/tmp/x")
	}

	// Non-conductor Instance with only a group override — source must be "group"
	instGroup := NewInstanceWithGroupAndTool("plain-session", tmpHome, "real-group", "claude")
	gpath, gsource := GetClaudeConfigDirSourceForInstance(instGroup)
	if gsource != "group" {
		t.Errorf("non-conductor Instance source = %q, want %q (conductor branch leaking?)", gsource, "group")
	}
	if gpath != "/tmp/group-for-non-conductor" {
		t.Errorf("non-conductor Instance path = %q, want %q", gpath, "/tmp/group-for-non-conductor")
	}
}
