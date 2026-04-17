package main

import (
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

// helper: create storage, add N root groups, return (storage, instances, groupTree).
// Each call overwrites the _test profile, so tests are independent when run sequentially.
func setupGroupsForReorder(t *testing.T, names ...string) *session.Storage {
	t.Helper()
	storage, err := session.NewStorageWithProfile("_test")
	if err != nil {
		t.Fatalf("NewStorageWithProfile: %v", err)
	}

	instances := []*session.Instance{}
	groupTree := session.NewGroupTreeWithGroups(instances, nil)

	for _, name := range names {
		groupTree.CreateGroup(name)
	}

	if err := storage.SaveWithGroups(instances, groupTree); err != nil {
		t.Fatalf("SaveWithGroups: %v", err)
	}

	return storage
}

// helper: reload groups from storage and return ordered paths (excluding default group)
func reloadGroupPaths(t *testing.T, storage *session.Storage) []string {
	t.Helper()
	_, groups, err := storage.LoadWithGroups()
	if err != nil {
		t.Fatalf("LoadWithGroups: %v", err)
	}

	instances := []*session.Instance{}
	tree := session.NewGroupTreeWithGroups(instances, groups)

	var paths []string
	for _, g := range tree.GroupList {
		if g.Path == session.DefaultGroupPath {
			continue
		}
		paths = append(paths, g.Path)
	}
	return paths
}

func TestGroupReorderUp(t *testing.T) {
	storage := setupGroupsForReorder(t, "Alpha", "Beta", "Gamma")

	// Move Beta up — should swap with Alpha
	handleGroupReorder("_test", []string{"Beta", "--up"})

	paths := reloadGroupPaths(t, storage)
	if len(paths) < 3 {
		t.Fatalf("expected 3 groups, got %d", len(paths))
	}
	if paths[0] != "Beta" || paths[1] != "Alpha" || paths[2] != "Gamma" {
		t.Errorf("expected [Beta Alpha Gamma], got %v", paths)
	}
}

func TestGroupReorderDown(t *testing.T) {
	storage := setupGroupsForReorder(t, "Alpha", "Beta", "Gamma")

	// Move Beta down — should swap with Gamma
	handleGroupReorder("_test", []string{"Beta", "--down"})

	paths := reloadGroupPaths(t, storage)
	if len(paths) < 3 {
		t.Fatalf("expected 3 groups, got %d", len(paths))
	}
	if paths[0] != "Alpha" || paths[1] != "Gamma" || paths[2] != "Beta" {
		t.Errorf("expected [Alpha Gamma Beta], got %v", paths)
	}
}

func TestGroupReorderPosition(t *testing.T) {
	storage := setupGroupsForReorder(t, "Alpha", "Beta", "Gamma")

	// Move Gamma to position 0
	handleGroupReorder("_test", []string{"Gamma", "--position", "0"})

	paths := reloadGroupPaths(t, storage)
	if len(paths) < 3 {
		t.Fatalf("expected 3 groups, got %d", len(paths))
	}
	if paths[0] != "Gamma" || paths[1] != "Alpha" || paths[2] != "Beta" {
		t.Errorf("expected [Gamma Alpha Beta], got %v", paths)
	}
}

func TestGroupReorderAlreadyAtTop(t *testing.T) {
	storage := setupGroupsForReorder(t, "Alpha", "Beta", "Gamma")

	// Move Alpha up — already first, should be no-op
	handleGroupReorder("_test", []string{"Alpha", "--up"})

	paths := reloadGroupPaths(t, storage)
	if len(paths) < 3 {
		t.Fatalf("expected 3 groups, got %d", len(paths))
	}
	if paths[0] != "Alpha" || paths[1] != "Beta" || paths[2] != "Gamma" {
		t.Errorf("expected [Alpha Beta Gamma], got %v", paths)
	}
}

func TestGroupReorderAlreadyAtBottom(t *testing.T) {
	storage := setupGroupsForReorder(t, "Alpha", "Beta", "Gamma")

	// Move Gamma down — already last, should be no-op
	handleGroupReorder("_test", []string{"Gamma", "--down"})

	paths := reloadGroupPaths(t, storage)
	if len(paths) < 3 {
		t.Fatalf("expected 3 groups, got %d", len(paths))
	}
	if paths[0] != "Alpha" || paths[1] != "Beta" || paths[2] != "Gamma" {
		t.Errorf("expected [Alpha Beta Gamma], got %v", paths)
	}
}

func TestGroupReorderPositionClamp(t *testing.T) {
	storage := setupGroupsForReorder(t, "Alpha", "Beta", "Gamma")

	// Move Alpha to position 99 (should clamp to last)
	handleGroupReorder("_test", []string{"Alpha", "--position", "99"})

	paths := reloadGroupPaths(t, storage)
	if len(paths) < 3 {
		t.Fatalf("expected 3 groups, got %d", len(paths))
	}
	if paths[0] != "Beta" || paths[1] != "Gamma" || paths[2] != "Alpha" {
		t.Errorf("expected [Beta Gamma Alpha], got %v", paths)
	}
}
