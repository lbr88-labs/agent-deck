package watcher

import "fmt"

// ScaffoldWatcherLayout creates ~/.agent-deck/watcher/{CLAUDE.md, POLICY.md, LEARNINGS.md, clients.json} if missing. Implemented in Task B.
func ScaffoldWatcherLayout() error { return fmt.Errorf("not implemented (RED)") }

// LayoutDir returns the root singular watcher dir. Implemented in Task B.
func LayoutDir() (string, error) { return "", fmt.Errorf("not implemented (RED)") }

// WatcherDir returns the per-watcher dir (validates name). Implemented in Task B.
func WatcherDir(name string) (string, error) { return "", fmt.Errorf("not implemented (RED)") }

// MigrateLegacyWatchersDir atomically renames legacy watchers/ -> watcher/ once. Implemented in Task B.
func MigrateLegacyWatchersDir() error { return fmt.Errorf("not implemented (RED)") }
