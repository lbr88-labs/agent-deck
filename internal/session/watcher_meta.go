package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WatcherMeta holds metadata for a named watcher instance.
// Persisted as meta.json in ~/.agent-deck/watchers/<name>/.
type WatcherMeta struct {
	Name      string `json:"name"`
	Type      string `json:"type"`       // adapter type: "webhook", "ntfy", "github", "slack", "gmail"
	CreatedAt string `json:"created_at"` // RFC3339 timestamp
}

// WatcherDir returns the base directory for all watchers (~/.agent-deck/watchers).
func WatcherDir() (string, error) {
	dir, err := GetAgentDeckDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "watchers"), nil
}

// WatcherNameDir returns the directory for a named watcher (~/.agent-deck/watchers/<name>).
func WatcherNameDir(name string) (string, error) {
	base, err := WatcherDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, name), nil
}

// SaveWatcherMeta writes meta.json for a watcher.
// Creates the watcher directory if it does not exist.
func SaveWatcherMeta(meta *WatcherMeta) error {
	if meta == nil {
		return fmt.Errorf("watcher metadata cannot be nil")
	}
	if meta.Name == "" {
		return fmt.Errorf("watcher name cannot be empty")
	}
	dir, err := WatcherNameDir(meta.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create watcher dir: %w", err)
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal watcher meta.json: %w", err)
	}
	metaPath := filepath.Join(dir, "meta.json")
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write watcher meta.json: %w", err)
	}
	return nil
}

// LoadWatcherMeta reads meta.json for a named watcher.
func LoadWatcherMeta(name string) (*WatcherMeta, error) {
	dir, err := WatcherNameDir(name)
	if err != nil {
		return nil, err
	}
	metaPath := filepath.Join(dir, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read meta.json for watcher %q: %w", name, err)
	}
	var meta WatcherMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse meta.json for watcher %q: %w", name, err)
	}
	if meta.Name == "" {
		meta.Name = name
	}
	return &meta, nil
}
