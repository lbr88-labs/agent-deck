package session

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

const (
	// DefaultProfile is the name of the default profile
	DefaultProfile = "default"

	// ProfilesDirName is the directory containing all profiles
	ProfilesDirName = "profiles"

	// ConfigFileName is the global config file name
	ConfigFileName = "config.json"
)

// Config represents the global agent-deck configuration
type Config struct {
	// DefaultProfile is the profile to use when none is specified
	DefaultProfile string `json:"default_profile"`

	// LastUsed is the most recently used profile (for future use)
	LastUsed string `json:"last_used,omitempty"`

	// Version tracks config format for future migrations
	Version int `json:"version"`
}

// --- S4 data-loss safeguard (2026-06-04 incident, chain-link #2) -------------
//
// GetAgentDeckDir() is the single chokepoint every other agent-deck path
// function (config.json, profiles/<p>/state.db, worker-scratch, logs) builds
// on. It resolves via os.UserHomeDir(), which reads $HOME. When $HOME points at
// the developer's real home (e.g. a test that forgot testutil.IsolateHome(), or
// any caller running without an XDG/sandbox dir present) the resolver used to
// hand back the live ~/.agent-deck with no signal at all — so an un-isolated
// test silently touched real user data.
//
// S4 makes that chain-link fail closed *in the production resolver itself*,
// complementing S5's opt-in IsolateHome() + pathsafety guard test:
//
//   - When running under test (testing.Testing()==true) AND resolution lands
//     under the real OS-user home, GetAgentDeckDir() REFUSES (returns an error)
//     instead of returning the live path, and emits a loud one-time warning.
//   - The real binary (testing.Testing()==false) is UNCHANGED: it still
//     resolves ~/.agent-deck normally and silently.
//
// The real home is read from the OS user database (os/user), NOT $HOME, so the
// guard can tell a sandboxed HOME from the real home even after $HOME is
// overridden — mirroring internal/pathsafety's realHome().

var (
	legacyFallbackWarnOnce sync.Once
	legacyFallbackWarnMu   sync.Mutex
	legacyFallbackWarnSink io.Writer = os.Stderr
)

// setLegacyFallbackWarnSink redirects the S4 warning (test hook). Returns a
// restore func.
func setLegacyFallbackWarnSink(w io.Writer) func() {
	legacyFallbackWarnMu.Lock()
	prev := legacyFallbackWarnSink
	legacyFallbackWarnSink = w
	legacyFallbackWarnMu.Unlock()
	return func() {
		legacyFallbackWarnMu.Lock()
		legacyFallbackWarnSink = prev
		legacyFallbackWarnMu.Unlock()
	}
}

// resetLegacyFallbackWarnOnce re-arms the debounce (test hook).
func resetLegacyFallbackWarnOnce() {
	legacyFallbackWarnMu.Lock()
	legacyFallbackWarnOnce = sync.Once{}
	legacyFallbackWarnMu.Unlock()
}

// osUserRealHome returns the developer's actual home from the OS user database,
// independent of $HOME. Empty string if it cannot be determined (in which case
// the guard cannot prove a real-home hit and stays out of the way).
func osUserRealHome() string {
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		return filepath.Clean(u.HomeDir)
	}
	return ""
}

// pathUnderReal reports whether p is the real home (or its ~/.agent-deck) or
// lives beneath it.
func pathUnderReal(p, realHome string) bool {
	if realHome == "" {
		return false
	}
	clean := filepath.Clean(p)
	if clean == realHome || strings.HasPrefix(clean, realHome+string(os.PathSeparator)) {
		return true
	}
	realAgentDeck := filepath.Join(realHome, ".agent-deck")
	return clean == realAgentDeck || strings.HasPrefix(clean, realAgentDeck+string(os.PathSeparator))
}

// warnLegacyFallbackOnce emits the S4 legacy-fallback warning exactly once.
func warnLegacyFallbackOnce(resolved string) {
	legacyFallbackWarnMu.Lock()
	once := &legacyFallbackWarnOnce
	sink := legacyFallbackWarnSink
	legacyFallbackWarnMu.Unlock()
	once.Do(func() {
		fmt.Fprintf(sink,
			"agentpaths: resolved to legacy ~/.agent-deck (%s) (no XDG dir present); "+
				"this touches REAL user data. If this is a test, it is NOT sandboxed — "+
				"call testutil.IsolateHome() in TestMain and run with a temp HOME+XDG "+
				"(2026-06-04 data-loss incident, S4).\n",
			resolved)
	})
}

// GetAgentDeckDir returns the base agent-deck directory (~/.agent-deck).
//
// See the S4 block above: under test it refuses (and warns) when resolution
// lands under the real OS-user home; the real binary is unchanged.
func GetAgentDeckDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	resolved := filepath.Join(homeDir, ".agent-deck")

	// S4 fail-closed guard: only active under `go test`. The real binary keeps
	// the original behavior exactly.
	if testing.Testing() {
		if realHome := osUserRealHome(); pathUnderReal(resolved, realHome) {
			warnLegacyFallbackOnce(resolved)
			return "", fmt.Errorf(
				"agentpaths: refusing to resolve agent-deck dir under the real home %q "+
					"(resolved %q) while running under test — the suite is NOT sandboxed; "+
					"call testutil.IsolateHome() in TestMain (2026-06-04 data-loss incident, S4)",
				realHome, resolved)
		}
	}

	return resolved, nil
}

// GetConfigPath returns the path to the global config file
func GetConfigPath() (string, error) {
	dir, err := GetAgentDeckDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
}

// GetProfilesDir returns the path to the profiles directory
func GetProfilesDir() (string, error) {
	dir, err := GetAgentDeckDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ProfilesDirName), nil
}

// GetProfileDir returns the path to a specific profile's directory
func GetProfileDir(profile string) (string, error) {
	if profile == "" {
		profile = DefaultProfile
	}

	// Sanitize profile name (prevent path traversal)
	profile = filepath.Base(profile)
	if profile == "." || profile == ".." {
		return "", fmt.Errorf("invalid profile name: %s", profile)
	}

	profilesDir, err := GetProfilesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(profilesDir, profile), nil
}

// LoadConfig loads the global configuration
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	// Check if config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return default config
		return &Config{
			DefaultProfile: DefaultProfile,
			Version:        1,
		}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Ensure default profile is set
	if config.DefaultProfile == "" {
		config.DefaultProfile = DefaultProfile
	}

	return &config, nil
}

// SaveConfig saves the global configuration
func SaveConfig(config *Config) error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// ListProfiles returns all available profile names
func ListProfiles() ([]string, error) {
	profilesDir, err := GetProfilesDir()
	if err != nil {
		return nil, err
	}

	// Check if profiles directory exists
	if _, err := os.Stat(profilesDir); os.IsNotExist(err) {
		// No profiles yet - check if we need migration
		return []string{}, nil
	}

	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles directory: %w", err)
	}

	var profiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Check for state.db (SQLite, v0.11.0+) or sessions.json (legacy, auto-migrates on open)
			dbPath := filepath.Join(profilesDir, entry.Name(), "state.db")
			jsonPath := filepath.Join(profilesDir, entry.Name(), "sessions.json")
			if _, err := os.Stat(dbPath); err == nil {
				profiles = append(profiles, entry.Name())
			} else if _, err := os.Stat(jsonPath); err == nil {
				profiles = append(profiles, entry.Name())
			}
		}
	}

	sort.Strings(profiles)
	return profiles, nil
}

// ProfileExists checks if a profile exists
func ProfileExists(profile string) (bool, error) {
	profileDir, err := GetProfileDir(profile)
	if err != nil {
		return false, err
	}

	// Check for state.db (SQLite, v0.11.0+) or sessions.json (legacy)
	dbPath := filepath.Join(profileDir, "state.db")
	if _, err = os.Stat(dbPath); err == nil {
		return true, nil
	}
	jsonPath := filepath.Join(profileDir, "sessions.json")
	if _, err = os.Stat(jsonPath); err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// CreateProfile creates a new empty profile
func CreateProfile(profile string) error {
	// Validate profile name
	if profile == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	// Check if already exists
	exists, err := ProfileExists(profile)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("profile '%s' already exists", profile)
	}

	profileDir, err := GetProfileDir(profile)
	if err != nil {
		return err
	}

	// Create profile directory
	if err := os.MkdirAll(profileDir, 0700); err != nil {
		return fmt.Errorf("failed to create profile directory: %w", err)
	}

	// Initialize SQLite database for the new profile.
	// NewStorageWithProfile auto-creates tables, so just opening it is sufficient.
	_, err = NewStorageWithProfile(profile)
	if err != nil {
		return fmt.Errorf("failed to initialize profile storage: %w", err)
	}

	return nil
}

// DeleteProfile deletes a profile and all its data
func DeleteProfile(profile string) error {
	// Prevent deleting the default profile if it's the only one
	if profile == DefaultProfile {
		profiles, err := ListProfiles()
		if err != nil {
			return err
		}
		if len(profiles) <= 1 {
			return fmt.Errorf("cannot delete the only remaining profile")
		}
	}

	profileDir, err := GetProfileDir(profile)
	if err != nil {
		return err
	}

	// Check if profile exists
	exists, err := ProfileExists(profile)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("profile '%s' does not exist", profile)
	}

	// Remove the profile directory
	if err := os.RemoveAll(profileDir); err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	// Update config if this was the default profile
	config, err := LoadConfig()
	if err != nil {
		return err
	}
	if config.DefaultProfile == profile {
		config.DefaultProfile = DefaultProfile
		if err := SaveConfig(config); err != nil {
			return fmt.Errorf("profile deleted but failed to update config: %w", err)
		}
	}

	return nil
}

// SetDefaultProfile sets the default profile in the config
func SetDefaultProfile(profile string) error {
	// Verify profile exists
	exists, err := ProfileExists(profile)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("profile '%s' does not exist", profile)
	}

	config, err := LoadConfig()
	if err != nil {
		return err
	}

	config.DefaultProfile = profile
	return SaveConfig(config)
}

// GetEffectiveProfile returns the profile to use, considering:
// 1. Explicitly provided profile (from -p flag)
// 2. Environment variable AGENTDECK_PROFILE
// 3. Inferred from CLAUDE_CONFIG_DIR (e.g. ~/.claude-work -> "work")
// 4. Config default profile
// 5. Fallback to "default"
//
// Priority 3 was added to fix issue #881: prior to this, the TUI's
// profile.DetectCurrentProfile honored CLAUDE_CONFIG_DIR while the web /
// storage / push paths did not, so the same user on the same machine could
// see different sessions in TUI vs web. Both call sites now route through
// this function to guarantee a single source of truth.
func GetEffectiveProfile(explicit string) string {
	if explicit != "" {
		return explicit
	}

	if envProfile := os.Getenv("AGENTDECK_PROFILE"); envProfile != "" {
		return envProfile
	}

	if inferred := profileFromClaudeConfigDir(os.Getenv("CLAUDE_CONFIG_DIR")); inferred != "" {
		return inferred
	}

	config, err := LoadConfig()
	if err != nil {
		return DefaultProfile
	}

	if config.DefaultProfile != "" {
		return config.DefaultProfile
	}

	return DefaultProfile
}

// profileFromClaudeConfigDir maps a CLAUDE_CONFIG_DIR path to a profile name.
// The supported shapes mirror the cdw / cdp shell aliases that drive the
// dual-profile setup:
//
//	~/.claude-work        -> "work"
//	~/.claude-personal    -> "personal"
//	~/.claude             -> ""  (no inference; let config default apply)
//	/opt/claude-prod      -> "prod"
//
// Returns "" when no profile can be inferred — the caller then falls back
// to the global config default.
func profileFromClaudeConfigDir(configDir string) string {
	if configDir == "" {
		return ""
	}
	baseName := filepath.Base(configDir)
	if strings.HasPrefix(baseName, ".claude-") {
		if suffix := strings.TrimPrefix(baseName, ".claude-"); suffix != "" {
			return suffix
		}
	}
	if strings.Contains(baseName, "-") {
		parts := strings.Split(baseName, "-")
		if last := parts[len(parts)-1]; last != "" {
			return last
		}
	}
	return ""
}
