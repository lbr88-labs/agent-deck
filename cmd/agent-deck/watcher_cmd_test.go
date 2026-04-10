package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/watcher"
)

func TestParseChannelsJSON_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "channels.json")
	content := `{
  "C0AABSF5GKD": {"name": "SI Bugs", "project_path": "/path/to/proj", "group": "bugs", "prefix": "si-bugs"},
  "C1BBCDF6HLE": {"name": "Feature Requests", "project_path": "/path/to/proj2", "group": "features", "prefix": "feat-req"}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	channels, err := parseChannelsJSON(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(channels))
	}
	si, ok := channels["C0AABSF5GKD"]
	if !ok {
		t.Fatal("missing channel C0AABSF5GKD")
	}
	if si.Name != "SI Bugs" {
		t.Errorf("expected name 'SI Bugs', got %q", si.Name)
	}
	if si.Group != "bugs" {
		t.Errorf("expected group 'bugs', got %q", si.Group)
	}
	if si.Prefix != "si-bugs" {
		t.Errorf("expected prefix 'si-bugs', got %q", si.Prefix)
	}
}

func TestParseChannelsJSON_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "channels.json")
	if err := os.WriteFile(path, []byte(`{invalid json`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, err := parseChannelsJSON(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseChannelsJSON_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "channels.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	channels, err := parseChannelsJSON(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if channels == nil {
		t.Fatal("expected non-nil map for empty JSON object")
	}
	if len(channels) != 0 {
		t.Fatalf("expected 0 channels, got %d", len(channels))
	}
}

func TestParseChannelsJSON_Nonexistent(t *testing.T) {
	_, err := parseChannelsJSON("/nonexistent/path/channels.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestGenerateWatcherToml(t *testing.T) {
	cfg := channelConfig{
		Name:        "SI Bugs",
		ProjectPath: "/path/to/project",
		Group:       "bugs",
		Prefix:      "si-bugs",
	}
	toml := generateWatcherToml("C0AABSF5GKD", cfg)

	checks := []string{
		`name = "si-bugs"`,
		`type = "slack"`,
		`conductor = "si-bugs"`,
		`group = "bugs"`,
		"C0AABSF5GKD",
	}
	for _, check := range checks {
		if !containsStr(toml, check) {
			t.Errorf("generated TOML missing %q\n\nFull output:\n%s", check, toml)
		}
	}
}

func TestMergeClientsJSON_NewFile(t *testing.T) {
	dir := t.TempDir()
	clientsPath := filepath.Join(dir, "clients.json")

	entries := map[string]watcher.ClientEntry{
		"slack:C0AABSF5GKD": {Conductor: "si-bugs", Group: "bugs", Name: "SI Bugs"},
	}
	if err := mergeClientsJSON(clientsPath, entries); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := watcher.LoadClientsJSON(clientsPath)
	if err != nil {
		t.Fatalf("failed to load written clients.json: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded))
	}
	entry, ok := loaded["slack:C0AABSF5GKD"]
	if !ok {
		t.Fatal("missing key slack:C0AABSF5GKD")
	}
	if entry.Conductor != "si-bugs" {
		t.Errorf("expected conductor 'si-bugs', got %q", entry.Conductor)
	}
}

func TestMergeClientsJSON_MergeExisting(t *testing.T) {
	dir := t.TempDir()
	clientsPath := filepath.Join(dir, "clients.json")

	// Write existing entry
	existing := map[string]watcher.ClientEntry{
		"user@example.com": {Conductor: "email-watcher", Group: "inbox", Name: "Email User"},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(clientsPath, data, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Merge new entry
	newEntries := map[string]watcher.ClientEntry{
		"slack:C123": {Conductor: "slack-bugs", Group: "bugs", Name: "Slack Bugs"},
	}
	if err := mergeClientsJSON(clientsPath, newEntries); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := watcher.LoadClientsJSON(clientsPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded))
	}
	if _, ok := loaded["user@example.com"]; !ok {
		t.Error("missing existing entry user@example.com")
	}
	if _, ok := loaded["slack:C123"]; !ok {
		t.Error("missing new entry slack:C123")
	}
}

func TestMergeClientsJSON_OverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	clientsPath := filepath.Join(dir, "clients.json")

	// Write existing entry
	existing := map[string]watcher.ClientEntry{
		"slack:C123": {Conductor: "old-conductor", Group: "old-group", Name: "Old Name"},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(clientsPath, data, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Merge with updated entry for the same key
	newEntries := map[string]watcher.ClientEntry{
		"slack:C123": {Conductor: "new-conductor", Group: "new-group", Name: "New Name"},
	}
	if err := mergeClientsJSON(clientsPath, newEntries); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := watcher.LoadClientsJSON(clientsPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	entry := loaded["slack:C123"]
	if entry.Conductor != "new-conductor" {
		t.Errorf("expected conductor 'new-conductor', got %q", entry.Conductor)
	}
	if entry.Group != "new-group" {
		t.Errorf("expected group 'new-group', got %q", entry.Group)
	}
}

func TestImportChannels_EndToEnd(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	channelsJSON := `{
  "C0AABSF5GKD": {"name": "SI Bugs", "project_path": "/path/to/proj", "group": "bugs", "prefix": "si-bugs"},
  "C1BBCDF6HLE": {"name": "Feature Requests", "project_path": "/path/to/proj2", "group": "features", "prefix": "feat-req"}
}`
	inputPath := filepath.Join(inputDir, "channels.json")
	if err := os.WriteFile(inputPath, []byte(channelsJSON), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if err := importChannels(inputPath, outputDir); err != nil {
		t.Fatalf("importChannels: %v", err)
	}

	// Verify watcher.toml files created
	toml1 := filepath.Join(outputDir, "si-bugs", "watcher.toml")
	if _, err := os.Stat(toml1); os.IsNotExist(err) {
		t.Errorf("missing watcher.toml for si-bugs")
	}
	toml2 := filepath.Join(outputDir, "feat-req", "watcher.toml")
	if _, err := os.Stat(toml2); os.IsNotExist(err) {
		t.Errorf("missing watcher.toml for feat-req")
	}

	// Verify clients.json
	clientsPath := filepath.Join(outputDir, "clients.json")
	loaded, err := watcher.LoadClientsJSON(clientsPath)
	if err != nil {
		t.Fatalf("load clients.json: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 client entries, got %d", len(loaded))
	}
	entry, ok := loaded["slack:C0AABSF5GKD"]
	if !ok {
		t.Fatal("missing key slack:C0AABSF5GKD in clients.json")
	}
	if entry.Conductor != "si-bugs" {
		t.Errorf("expected conductor 'si-bugs', got %q", entry.Conductor)
	}
	if entry.Group != "bugs" {
		t.Errorf("expected group 'bugs', got %q", entry.Group)
	}
	if entry.Name != "SI Bugs" {
		t.Errorf("expected name 'SI Bugs', got %q", entry.Name)
	}
}

func TestImportChannels_Idempotent(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	channelsJSON := `{
  "C0AABSF5GKD": {"name": "SI Bugs", "project_path": "/path/to/proj", "group": "bugs", "prefix": "si-bugs"}
}`
	inputPath := filepath.Join(inputDir, "channels.json")
	if err := os.WriteFile(inputPath, []byte(channelsJSON), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Run twice
	if err := importChannels(inputPath, outputDir); err != nil {
		t.Fatalf("first import: %v", err)
	}
	if err := importChannels(inputPath, outputDir); err != nil {
		t.Fatalf("second import: %v", err)
	}

	// Read the toml content
	tomlPath := filepath.Join(outputDir, "si-bugs", "watcher.toml")
	data1, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatalf("read toml: %v", err)
	}
	if len(data1) == 0 {
		t.Fatal("watcher.toml is empty after idempotent import")
	}

	// Verify clients.json still has exactly 1 entry (not duplicated)
	clientsPath := filepath.Join(outputDir, "clients.json")
	loaded, err := watcher.LoadClientsJSON(clientsPath)
	if err != nil {
		t.Fatalf("load clients.json: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 entry after idempotent import, got %d", len(loaded))
	}
}

func TestImportChannels_RejectsSymlink(t *testing.T) {
	dir := t.TempDir()

	// Create a real channels.json
	realPath := filepath.Join(dir, "real-channels.json")
	if err := os.WriteFile(realPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Create a symlink to it
	linkPath := filepath.Join(dir, "link-channels.json")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	outputDir := t.TempDir()
	err := importChannels(linkPath, outputDir)
	if err == nil {
		t.Fatal("expected error for symlink input, got nil")
	}
}

func TestImportChannels_RejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	outputDir := t.TempDir()

	err := importChannels(dir, outputDir)
	if err == nil {
		t.Fatal("expected error for directory input, got nil")
	}
}

func TestImportChannels_EmptyChannels(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	inputPath := filepath.Join(inputDir, "channels.json")
	if err := os.WriteFile(inputPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if err := importChannels(inputPath, outputDir); err != nil {
		t.Fatalf("unexpected error for empty channels: %v", err)
	}

	// No watcher dirs should be created, but clients.json should exist (empty)
	clientsPath := filepath.Join(outputDir, "clients.json")
	if _, err := os.Stat(clientsPath); os.IsNotExist(err) {
		t.Error("expected clients.json to exist even with empty channels")
	}
}

// containsStr is a helper for string-contains checks in tests.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
