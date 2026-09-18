package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGetProposeOnStartup_DefaultsTrue pins the default: absent
// [recovery].propose_on_startup must propose crash recovery on launch. A
// user hit by a power loss is exactly the user who will not go edit a
// config.toml to turn a safety feature on.
func TestGetProposeOnStartup_DefaultsTrue(t *testing.T) {
	var c UserConfig
	if !c.Recovery.GetProposeOnStartup() {
		t.Error("propose_on_startup must default to true")
	}
}

func TestGetProposeOnStartup_ExplicitFalse(t *testing.T) {
	off := false
	c := UserConfig{Recovery: RecoverySettings{ProposeOnStartup: &off}}
	if c.Recovery.GetProposeOnStartup() {
		t.Error("propose_on_startup=false not honored")
	}
}

func TestGetProposeOnStartup_ExplicitTrue(t *testing.T) {
	on := true
	c := UserConfig{Recovery: RecoverySettings{ProposeOnStartup: &on}}
	if !c.Recovery.GetProposeOnStartup() {
		t.Error("propose_on_startup=true not honored")
	}
}

// TestRecoveryProposeOnStartup_RoundTrip proves the toml tag wiring: an
// explicit [recovery].propose_on_startup = false survives save → load.
func TestRecoveryProposeOnStartup_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)
	isolateConfigHomeXDG(t)

	initial := "default_tool = \"claude\"\n\n[recovery]\n  propose_on_startup = false\n"
	configPath, err := GetUserConfigPath()
	if err != nil {
		t.Fatalf("GetUserConfigPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	ClearUserConfigCache()

	loaded, err := LoadUserConfig()
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	if loaded.Recovery.GetProposeOnStartup() {
		t.Errorf("propose_on_startup=false lost in round-trip; loaded config: %+v", loaded.Recovery)
	}

	// And saving that loaded config must emit the section back.
	if err := SaveUserConfig(loaded); err != nil {
		t.Fatalf("SaveUserConfig: %v", err)
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	if !strings.Contains(string(raw), "propose_on_startup") {
		t.Errorf("saved config lost [recovery].propose_on_startup:\n%s", string(raw))
	}
}
