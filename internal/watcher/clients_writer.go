package watcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// clientsWriteMu serializes concurrent AppendClientEntry calls process-wide.
// Held across the full load-merge-write-rename cycle to prevent TOCTOU races
// between concurrent triage sessions (D-11, T-18-03).
var clientsWriteMu sync.Mutex

// AppendClientEntry atomically appends a sender→entry mapping to clients.json
// at the given path. It is safe to call concurrently from multiple goroutines.
//
// Behavior:
//   - Wildcard senders ("*@domain") are rejected immediately — these must only
//     be added by explicit human action (D-14, T-18-02).
//   - If the sender is already present, returns nil without modifying the file
//     (idempotent, D-11 step 3).
//   - If clients.json does not exist, it is created with mode 0o600.
//   - The parent directory is created with mode 0o700 if it does not exist.
//   - The write is atomic via write-to-temp + os.Rename (POSIX, D-11, T-18-01).
//   - A stale <path>.tmp from a prior crash is silently overwritten by
//     os.WriteFile (truncates) and then renamed away (T-18-04).
func AppendClientEntry(path, sender string, entry ClientEntry) error {
	// Reject wildcard entries before acquiring the lock (D-14).
	if strings.HasPrefix(sender, "*@") {
		return errors.New("clients_writer: wildcard entries must not be auto-written")
	}

	clientsWriteMu.Lock()
	defer clientsWriteMu.Unlock()

	// Load current state, or start empty if file does not exist (D-11 step 2).
	var current map[string]ClientEntry
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if jsonErr := json.Unmarshal(data, &current); jsonErr != nil {
			return fmt.Errorf("clients_writer: parse %q: %w", path, jsonErr)
		}
	case os.IsNotExist(err):
		current = make(map[string]ClientEntry)
	default:
		return fmt.Errorf("clients_writer: read %q: %w", path, err)
	}
	// Guard against a valid but null JSON file ("null" unmarshals to nil map).
	if current == nil {
		current = make(map[string]ClientEntry)
	}

	// Idempotent: if sender already exists, no-op (D-11 step 3).
	if _, exists := current[sender]; exists {
		return nil
	}
	current[sender] = entry

	// Marshal with indentation for human readability (D-11 step 5).
	out, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("clients_writer: marshal: %w", err)
	}

	// Ensure parent directory exists with restricted perms (D-11 step 6).
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("clients_writer: mkdir: %w", err)
	}

	// Atomic write-temp-rename (D-11 steps 6–7, T-18-01/T-18-04).
	// os.WriteFile truncates any stale .tmp from a prior crash before writing.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return fmt.Errorf("clients_writer: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("clients_writer: rename tmp: %w", err)
	}
	return nil
}
