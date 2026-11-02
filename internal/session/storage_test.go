package session

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/statedb"
)

// newTestStorage creates a Storage backed by an in-memory-like temp dir SQLite database.
func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	db, err := statedb.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &Storage{db: db, dbPath: dbPath, profile: "_test"}
}

func TestStorageCodexBindingConfirmationRoundTrip(t *testing.T) {
	const sessionID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	for _, tc := range []struct {
		name      string
		id        string
		revision  int64
		confirmed bool
		wantProof bool
	}{
		{"confirmed-zero", sessionID, 0, true, true},
		{"confirmed-one", sessionID, 1, true, true},
		{"unconfirmed-zero", sessionID, 0, false, false},
		{"unconfirmed-one", sessionID, 1, false, false},
		{"empty", "", 1, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStorage(t)
			inst := &Instance{
				ID: "proof-roundtrip", Title: "test", ProjectPath: t.TempDir(),
				GroupPath: DefaultGroupPath, Tool: "codex", Command: "codex",
				Status: StatusIdle, CreatedAt: time.Now(), stateDB: s.db,
				CodexSessionID: tc.id, CodexDetectedAt: time.Unix(100, 0),
				CodexBindingRevision: tc.revision, CodexBindingConfirmed: tc.confirmed,
			}
			if err := s.SaveWithGroups([]*Instance{inst}, nil); err != nil {
				t.Fatalf("SaveWithGroups: %v", err)
			}
			if inst.CodexBindingConfirmed != tc.wantProof {
				t.Fatalf("post-commit confirmation=%v, want %v", inst.CodexBindingConfirmed, tc.wantProof)
			}
			lite, _, err := s.LoadLite()
			if err != nil || len(lite) != 1 {
				t.Fatalf("LoadLite: count=%d err=%v", len(lite), err)
			}
			if lite[0].CodexSessionID != tc.id || lite[0].CodexBindingRevision != tc.revision || lite[0].CodexBindingConfirmed != tc.wantProof {
				t.Fatalf("lite tuple = (%q, %d, %v), want (%q, %d, %v)", lite[0].CodexSessionID, lite[0].CodexBindingRevision, lite[0].CodexBindingConfirmed, tc.id, tc.revision, tc.wantProof)
			}
			serialized, err := json.Marshal(lite[0])
			if err != nil {
				t.Fatal(err)
			}
			var values map[string]any
			if err := json.Unmarshal(serialized, &values); err != nil {
				t.Fatal(err)
			}
			if marker, _ := values["codex_binding_confirmed"].(bool); marker != tc.wantProof {
				t.Fatalf("InstanceData JSON lost confirmation: %s", serialized)
			}
			loaded, err := s.Load()
			if err != nil || len(loaded) != 1 {
				t.Fatalf("Load: count=%d err=%v", len(loaded), err)
			}
			if loaded[0].CodexSessionID != tc.id || loaded[0].CodexBindingRevision != tc.revision || loaded[0].CodexBindingConfirmed != tc.wantProof {
				t.Fatalf("loaded tuple = (%q, %d, %v), want (%q, %d, %v)", loaded[0].CodexSessionID, loaded[0].CodexBindingRevision, loaded[0].CodexBindingConfirmed, tc.id, tc.revision, tc.wantProof)
			}
		})
	}
}

func TestStorageCodexLegacyBindingRemainsUnconfirmed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	for _, raw := range []string{
		`{"codex_session_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","codex_detected_at":100}`,
		`{"codex_session_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","codex_detected_at":100,"codex_binding_revision":1}`,
	} {
		s := newTestStorage(t)
		row := &statedb.InstanceRow{
			ID: "legacy-proof", Title: "test", ProjectPath: t.TempDir(), GroupPath: DefaultGroupPath,
			Tool: "codex", Command: "codex", Status: "idle", CreatedAt: time.Now(), ToolData: json.RawMessage(raw),
		}
		if err := s.db.SaveInstance(row); err != nil {
			t.Fatal(err)
		}
		lite, _, err := s.LoadLite()
		if err != nil || len(lite) != 1 {
			t.Fatalf("LoadLite: count=%d err=%v", len(lite), err)
		}
		if lite[0].CodexBindingConfirmed {
			t.Fatalf("LoadLite inferred proof from history: %s", raw)
		}
		loaded, err := s.Load()
		if err != nil || len(loaded) != 1 {
			t.Fatalf("Load: count=%d err=%v", len(loaded), err)
		}
		if loaded[0].CodexBindingConfirmed {
			t.Fatalf("Load inferred proof from history: %s", raw)
		}
		if err := s.SaveWithGroups(loaded, nil); err != nil {
			t.Fatal(err)
		}
		_, _, _, confirmed, err := s.db.ReadCodexSessionBinding(row.ID)
		if err != nil || confirmed {
			t.Fatalf("save inferred proof: confirmed=%v err=%v", confirmed, err)
		}
	}
}

func TestStorageCodexPostCommitSyncReadsCurrentConfirmation(t *testing.T) {
	const sessionID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	for _, confirmed := range []bool{false, true} {
		s := newTestStorage(t)
		inst := &Instance{
			ID: "proof-sync", Title: "test", ProjectPath: t.TempDir(),
			GroupPath: DefaultGroupPath, Tool: "codex", Command: "codex",
			Status: StatusIdle, CreatedAt: time.Now(), stateDB: s.db,
			CodexSessionID: sessionID, CodexDetectedAt: time.Unix(100, 0),
			CodexBindingRevision: 1, CodexBindingConfirmed: !confirmed,
		}
		row, err := instanceToRow(inst)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.db.SaveInstance(row); err != nil {
			t.Fatal(err)
		}
		snapshot := captureCodexBindingSaveSnapshot(inst)
		if revision, matched, err := s.db.WriteCodexSessionBindingOverride(inst.ID, sessionID, time.Unix(200, 0), confirmed); err != nil || !matched || revision != 2 {
			t.Fatalf("peer update: revision=%d matched=%v err=%v", revision, matched, err)
		}
		inst.syncCommittedCodexBinding(s.db, row.ToolData, snapshot)
		if inst.CodexSessionID != sessionID || inst.CodexBindingRevision != 2 || inst.CodexBindingConfirmed != confirmed || !inst.CodexDetectedAt.Equal(time.Unix(200, 0)) {
			t.Fatalf("post-commit sync used stale proof: id=%q revision=%d confirmed=%v at=%v", inst.CodexSessionID, inst.CodexBindingRevision, inst.CodexBindingConfirmed, inst.CodexDetectedAt)
		}
	}
}

func TestStorageCodexNewInstanceUsesOwningDBForTargetedBindingAndClear(t *testing.T) {
	const sessionID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	ClearUserConfigCache()
	t.Cleanup(ClearUserConfigCache)
	previousGlobal := statedb.GetGlobal()
	statedb.SetGlobal(nil)
	t.Cleanup(func() { statedb.SetGlobal(previousGlobal) })

	for _, save := range []struct {
		name string
		run  func(*Storage, *Instance) error
	}{
		{"SaveWithGroups", func(s *Storage, inst *Instance) error { return s.SaveWithGroups([]*Instance{inst}, nil) }},
		{"InsertSessionAndVerify", func(s *Storage, inst *Instance) error { return s.InsertSessionAndVerify(inst, nil) }},
	} {
		t.Run(save.name, func(t *testing.T) {
			s := newTestStorage(t)
			inst := &Instance{
				ID: "owning-db-" + save.name, Title: "fresh", ProjectPath: t.TempDir(),
				GroupPath: DefaultGroupPath, Tool: "codex", Command: "codex",
				Status: StatusStopped, CreatedAt: time.Now(),
			}
			if inst.stateDB != nil || statedb.GetGlobal() != nil {
				t.Fatal("test requires a fresh instance without any database attachment")
			}
			// An explicit clear makes the initial save advance the generation;
			// the following targeted writes must use that committed revision.
			if _, _, err := SetField(inst, FieldCodexSessionID, "", nil); err != nil {
				t.Fatalf("initial explicit clear: %v", err)
			}
			if err := save.run(s, inst); err != nil {
				t.Fatalf("save: %v", err)
			}
			writeCodexRolloutWithSource(t, filepath.Join(home, ".codex"), sessionID, inst.ProjectPath, "cli")
			if _, _, err := SetField(inst, FieldCodexSessionID, sessionID, nil); err != nil {
				t.Fatalf("confirm native conversation: %v", err)
			}
			if !inst.persistCodexSessionBinding("") {
				t.Fatal("targeted explicit binding did not commit")
			}
			id, _, revision, confirmed, err := s.db.ReadCodexSessionBinding(inst.ID)
			if err != nil || id != sessionID || revision != 2 || !confirmed || inst.CodexBindingRevision != revision {
				t.Fatalf("targeted binding missed owning DB: id=%q revision=%d confirmed=%v memory_revision=%d err=%v", id, revision, confirmed, inst.CodexBindingRevision, err)
			}
			if _, _, err := SetField(inst, FieldCodexSessionID, "", nil); err != nil {
				t.Fatalf("explicit clear: %v", err)
			}
			if !inst.persistCodexSessionBinding("") {
				t.Fatal("targeted explicit clear did not commit")
			}
			id, _, revision, confirmed, err = s.db.ReadCodexSessionBinding(inst.ID)
			if err != nil || id != "" || revision != 3 || confirmed || inst.CodexBindingRevision != revision {
				t.Fatalf("targeted clear missed owning DB: id=%q revision=%d confirmed=%v memory_revision=%d err=%v", id, revision, confirmed, inst.CodexBindingRevision, err)
			}
		})
	}
}

func TestStorageCodexTemporarySavePreservesLiveOwnerForTargetedWrites(t *testing.T) {
	const sessionID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	ClearUserConfigCache()
	t.Cleanup(ClearUserConfigCache)
	previousGlobal := statedb.GetGlobal()
	statedb.SetGlobal(nil)
	t.Cleanup(func() { statedb.SetGlobal(previousGlobal) })

	owner := newTestStorage(t)
	created := &Instance{
		ID: "temporary-save-owner", Title: "long-lived", ProjectPath: t.TempDir(),
		GroupPath: DefaultGroupPath, Tool: "codex", Command: "codex",
		Status: StatusStopped, CreatedAt: time.Now(),
	}
	if _, _, err := SetField(created, FieldCodexSessionID, "", nil); err != nil {
		t.Fatalf("initial explicit clear: %v", err)
	}
	if err := owner.SaveWithGroups([]*Instance{created}, nil); err != nil {
		t.Fatalf("initial save: %v", err)
	}
	loaded, err := owner.Load()
	if err != nil || len(loaded) != 1 {
		t.Fatalf("load long-lived instance: count=%d err=%v", len(loaded), err)
	}
	inst := loaded[0]

	temporaryDB, err := statedb.Open(owner.dbPath)
	if err != nil {
		t.Fatalf("open temporary connection: %v", err)
	}
	temporary := &Storage{db: temporaryDB, dbPath: owner.dbPath, profile: owner.profile}
	t.Cleanup(func() { _ = temporary.Close() })
	inst.Title = "saved by temporary web request"
	if err := temporary.SaveWithGroups([]*Instance{inst}, nil); err != nil {
		t.Fatalf("temporary save of shared instance: %v", err)
	}
	if err := temporary.Close(); err != nil {
		t.Fatalf("close temporary storage: %v", err)
	}

	writeCodexRolloutWithSource(t, filepath.Join(home, ".codex"), sessionID, inst.ProjectPath, "cli")
	if _, _, err := SetField(inst, FieldCodexSessionID, sessionID, nil); err != nil {
		t.Fatalf("validate binding after temporary storage closed: %v", err)
	}
	if !inst.persistCodexSessionBinding("") {
		t.Fatal("targeted binding used the closed temporary connection")
	}
	id, _, revision, confirmed, err := owner.db.ReadCodexSessionBinding(inst.ID)
	if err != nil || id != sessionID || revision != 2 || !confirmed || inst.CodexBindingRevision != revision {
		t.Fatalf("binding after temporary close: id=%q revision=%d confirmed=%v memory_revision=%d err=%v", id, revision, confirmed, inst.CodexBindingRevision, err)
	}

	if _, _, err := SetField(inst, FieldCodexSessionID, "", nil); err != nil {
		t.Fatalf("explicit clear after temporary storage closed: %v", err)
	}
	if !inst.persistCodexSessionBinding("") {
		t.Fatal("targeted clear used the closed temporary connection")
	}
	id, _, revision, confirmed, err = owner.db.ReadCodexSessionBinding(inst.ID)
	if err != nil || id != "" || revision != 3 || confirmed || inst.CodexBindingRevision != revision {
		t.Fatalf("clear after temporary close: id=%q revision=%d confirmed=%v memory_revision=%d err=%v", id, revision, confirmed, inst.CodexBindingRevision, err)
	}
}

// TestStorageUpdatedAtTimestamp verifies that SaveWithGroups sets the UpdatedAt timestamp
// and GetUpdatedAt() returns it correctly.
func TestStorageUpdatedAtTimestamp(t *testing.T) {
	s := newTestStorage(t)

	instances := []*Instance{
		{
			ID:          "test-1",
			Title:       "Test Session",
			ProjectPath: "/tmp/test",
			GroupPath:   "test-group",
			Command:     "claude",
			Tool:        "claude",
			Status:      StatusIdle,
			CreatedAt:   time.Now(),
		},
	}

	// Save data
	beforeSave := time.Now()
	time.Sleep(10 * time.Millisecond)

	err := s.SaveWithGroups(instances, nil)
	if err != nil {
		t.Fatalf("SaveWithGroups failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	afterSave := time.Now()

	// Get the updated timestamp
	updatedAt, err := s.GetUpdatedAt()
	if err != nil {
		t.Fatalf("GetUpdatedAt failed: %v", err)
	}

	// Verify timestamp is within expected range
	if updatedAt.Before(beforeSave) {
		t.Errorf("UpdatedAt %v is before save started %v", updatedAt, beforeSave)
	}
	if updatedAt.After(afterSave) {
		t.Errorf("UpdatedAt %v is after save completed %v", updatedAt, afterSave)
	}

	// Verify timestamp is not zero
	if updatedAt.IsZero() {
		t.Error("UpdatedAt is zero, expected a valid timestamp")
	}

	// Save again and verify timestamp updates
	time.Sleep(50 * time.Millisecond)
	firstUpdatedAt := updatedAt

	err = s.SaveWithGroups(instances, nil)
	if err != nil {
		t.Fatalf("Second SaveWithGroups failed: %v", err)
	}

	secondUpdatedAt, err := s.GetUpdatedAt()
	if err != nil {
		t.Fatalf("Second GetUpdatedAt failed: %v", err)
	}

	// Verify second timestamp is after first
	if !secondUpdatedAt.After(firstUpdatedAt) {
		t.Errorf("Second UpdatedAt %v should be after first %v", secondUpdatedAt, firstUpdatedAt)
	}
}

// TestGetUpdatedAtEmpty verifies behavior when no data has been saved
func TestGetUpdatedAtEmpty(t *testing.T) {
	s := newTestStorage(t)

	updatedAt, err := s.GetUpdatedAt()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !updatedAt.IsZero() {
		t.Errorf("Expected zero time for empty db, got %v", updatedAt)
	}
}

// TestLoadLite verifies that LoadLite returns raw InstanceData without tmux initialization
func TestLoadLite(t *testing.T) {
	s := newTestStorage(t)

	instances := []*Instance{
		{
			ID:          "test-1",
			Title:       "Test Session 1",
			ProjectPath: "/tmp/test1",
			GroupPath:   "test-group",
			Command:     "claude",
			Tool:        "claude",
			Status:      StatusWaiting,
			CreatedAt:   time.Now(),
		},
		{
			ID:          "test-2",
			Title:       "Test Session 2",
			ProjectPath: "/tmp/test2",
			GroupPath:   "other-group",
			Command:     "gemini",
			Tool:        "gemini",
			Status:      StatusIdle,
			CreatedAt:   time.Now(),
		},
	}

	err := s.SaveWithGroups(instances, nil)
	if err != nil {
		t.Fatalf("SaveWithGroups failed: %v", err)
	}

	instData, groupData, err := s.LoadLite()
	if err != nil {
		t.Fatalf("LoadLite failed: %v", err)
	}

	if len(instData) != 2 {
		t.Errorf("Expected 2 instances, got %d", len(instData))
	}

	if instData[0].ID != "test-1" {
		t.Errorf("Expected first instance ID 'test-1', got '%s'", instData[0].ID)
	}
	if instData[0].Title != "Test Session 1" {
		t.Errorf("Expected first instance title 'Test Session 1', got '%s'", instData[0].Title)
	}
	if instData[0].Status != StatusWaiting {
		t.Errorf("Expected first instance status 'waiting', got '%s'", instData[0].Status)
	}

	if instData[1].ID != "test-2" {
		t.Errorf("Expected second instance ID 'test-2', got '%s'", instData[1].ID)
	}
	if instData[1].Tool != "gemini" {
		t.Errorf("Expected second instance tool 'gemini', got '%s'", instData[1].Tool)
	}

	if len(groupData) != 0 {
		t.Errorf("Expected 0 groups, got %d", len(groupData))
	}
}

// TestLoadLiteEmptyDB verifies LoadLite returns empty slice when database is empty
func TestLoadLiteEmptyDB(t *testing.T) {
	s := newTestStorage(t)

	instData, groupData, err := s.LoadLite()
	if err != nil {
		t.Errorf("LoadLite should not return error for empty db, got: %v", err)
	}
	if len(instData) != 0 {
		t.Errorf("Expected empty instances, got %d", len(instData))
	}
	if len(groupData) != 0 {
		t.Errorf("Expected empty groups, got %d", len(groupData))
	}
}

func TestStorageSaveWithGroups_DedupsClaudeSessionIDs(t *testing.T) {
	s := newTestStorage(t)
	now := time.Now()

	older := &Instance{
		ID:               "old",
		Title:            "Older",
		ProjectPath:      "/tmp/one",
		GroupPath:        "grp",
		Command:          "claude",
		Tool:             "claude",
		Status:           StatusIdle,
		CreatedAt:        now.Add(-2 * time.Minute),
		ClaudeSessionID:  "shared-session-id",
		ClaudeDetectedAt: now.Add(-2 * time.Minute),
	}
	newer := &Instance{
		ID:               "new",
		Title:            "Newer",
		ProjectPath:      "/tmp/two",
		GroupPath:        "grp",
		Command:          "claude",
		Tool:             "claude",
		Status:           StatusIdle,
		CreatedAt:        now.Add(-1 * time.Minute),
		ClaudeSessionID:  "shared-session-id",
		ClaudeDetectedAt: now.Add(-1 * time.Minute),
	}
	otherTool := &Instance{
		ID:          "gem",
		Title:       "Gemini",
		ProjectPath: "/tmp/gem",
		GroupPath:   "grp",
		Command:     "gemini",
		Tool:        "gemini",
		Status:      StatusIdle,
		CreatedAt:   now,
	}

	// Intentionally unsorted to ensure dedup logic does not rely on caller order.
	instances := []*Instance{newer, otherTool, older}
	if err := s.SaveWithGroups(instances, nil); err != nil {
		t.Fatalf("SaveWithGroups failed: %v", err)
	}

	if older.ClaudeSessionID != "shared-session-id" {
		t.Fatalf("older session should keep shared ID, got %q", older.ClaudeSessionID)
	}
	if newer.ClaudeSessionID != "" {
		t.Fatalf("newer duplicate should be cleared, got %q", newer.ClaudeSessionID)
	}

	loaded, _, err := s.LoadLite()
	if err != nil {
		t.Fatalf("LoadLite failed: %v", err)
	}

	byID := make(map[string]*InstanceData, len(loaded))
	for _, inst := range loaded {
		byID[inst.ID] = inst
	}
	if byID["old"] == nil || byID["new"] == nil {
		t.Fatalf("expected old and new instances in DB, got keys: %#v", byID)
	}
	if byID["old"].ClaudeSessionID != "shared-session-id" {
		t.Fatalf("db old session ID = %q, want shared-session-id", byID["old"].ClaudeSessionID)
	}
	if byID["new"].ClaudeSessionID != "" {
		t.Fatalf("db newer session ID = %q, want empty", byID["new"].ClaudeSessionID)
	}
}

func TestStorageSaveWithGroups_PersistsSandboxConfig(t *testing.T) {
	s := newTestStorage(t)

	cpu := "2.0"
	mem := "4g"
	instances := []*Instance{
		{
			ID:               "sandboxed-1",
			Title:            "Sandboxed Session",
			ProjectPath:      "/tmp/sandboxed",
			GroupPath:        "grp",
			Command:          "claude --dangerously-skip-permissions",
			Tool:             "claude",
			Status:           StatusIdle,
			CreatedAt:        time.Now(),
			Sandbox:          &SandboxConfig{Enabled: true, Image: "ghcr.io/example/sandbox:latest", CPULimit: &cpu, MemoryLimit: &mem},
			SandboxContainer: "agent-deck-sandbox-sandboxed-1",
		},
	}

	if err := s.SaveWithGroups(instances, nil); err != nil {
		t.Fatalf("SaveWithGroups failed: %v", err)
	}

	lite, _, err := s.LoadLite()
	if err != nil {
		t.Fatalf("LoadLite failed: %v", err)
	}
	if len(lite) != 1 {
		t.Fatalf("expected 1 lite instance, got %d", len(lite))
	}
	if lite[0].Sandbox == nil || !lite[0].Sandbox.Enabled {
		t.Fatal("expected sandbox config to be restored in LoadLite")
	}
	if lite[0].Sandbox.Image != "ghcr.io/example/sandbox:latest" {
		t.Fatalf("sandbox image = %q, want ghcr.io/example/sandbox:latest", lite[0].Sandbox.Image)
	}
	if lite[0].SandboxContainer != "agent-deck-sandbox-sandboxed-1" {
		t.Fatalf("sandbox container = %q, want agent-deck-sandbox-sandboxed-1", lite[0].SandboxContainer)
	}

	loaded, _, err := s.LoadWithGroups()
	if err != nil {
		t.Fatalf("LoadWithGroups failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 loaded instance, got %d", len(loaded))
	}
	if !loaded[0].IsSandboxed() {
		t.Fatal("expected loaded instance to remain sandboxed after SQLite round-trip")
	}
	if loaded[0].Sandbox == nil || loaded[0].Sandbox.Image != "ghcr.io/example/sandbox:latest" {
		t.Fatalf("loaded sandbox image = %#v", loaded[0].Sandbox)
	}
	if loaded[0].SandboxContainer != "agent-deck-sandbox-sandboxed-1" {
		t.Fatalf("loaded sandbox container = %q, want agent-deck-sandbox-sandboxed-1", loaded[0].SandboxContainer)
	}
}

// TestStorageSaveWithGroups_PersistsTitleLocked (#697) verifies that
// Instance.TitleLocked round-trips through SQLite so the sync blocker
// survives agent-deck restarts. Without persistence, a conductor-set lock
// would silently evaporate on the first TUI restart and Claude could
// rename the session on the next hook event.
func TestStorageSaveWithGroups_PersistsTitleLocked(t *testing.T) {
	s := newTestStorage(t)

	instances := []*Instance{
		{
			ID:          "locked-1",
			Title:       "SCRUM-351",
			ProjectPath: "/tmp/locked",
			GroupPath:   "grp",
			Command:     "claude",
			Tool:        "claude",
			Status:      StatusIdle,
			CreatedAt:   time.Now(),
			TitleLocked: true,
		},
		{
			ID:          "unlocked-1",
			Title:       "quiet-river",
			ProjectPath: "/tmp/unlocked",
			GroupPath:   "grp",
			Command:     "claude",
			Tool:        "claude",
			Status:      StatusIdle,
			CreatedAt:   time.Now(),
		},
	}

	if err := s.SaveWithGroups(instances, nil); err != nil {
		t.Fatalf("SaveWithGroups failed: %v", err)
	}

	loaded, _, err := s.LoadWithGroups()
	if err != nil {
		t.Fatalf("LoadWithGroups failed: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 loaded instances, got %d", len(loaded))
	}

	byID := map[string]*Instance{}
	for _, inst := range loaded {
		byID[inst.ID] = inst
	}
	if !byID["locked-1"].TitleLocked {
		t.Errorf("locked-1.TitleLocked = false after round-trip, want true (#697)")
	}
	if byID["unlocked-1"].TitleLocked {
		t.Errorf("unlocked-1.TitleLocked = true after round-trip, want false (default must not leak)")
	}

	// Also verify LoadLite preserves it (fast-path used by CLI commands)
	lite, _, err := s.LoadLite()
	if err != nil {
		t.Fatalf("LoadLite failed: %v", err)
	}
	liteByID := map[string]*InstanceData{}
	for _, d := range lite {
		liteByID[d.ID] = d
	}
	if !liteByID["locked-1"].TitleLocked {
		t.Errorf("LoadLite locked-1.TitleLocked = false, want true")
	}
}

// TestStorageSaveWithGroups_PersistsAutoName locks that the AutoName flag and
// its captured description survive Save → Load via the real SQLite path (the
// path the app actually uses on reopen), through both LoadWithGroups (canonical)
// and LoadLite (CLI fast-path). This is the regression that made auto-named
// quick sessions revert to their random handle on reopen: the flag and
// description lived only in memory and were never written to / read from the DB.
func TestStorageSaveWithGroups_PersistsAutoName(t *testing.T) {
	s := newTestStorage(t)

	auto := &Instance{
		ID:          "auto-1",
		Title:       "lively-fjord",
		ProjectPath: "/tmp/auto",
		GroupPath:   "grp",
		Command:     "claude",
		Tool:        "claude",
		Status:      StatusIdle,
		CreatedAt:   time.Now(),
		AutoName:    true,
	}
	auto.SetAutoNameDescription("Review and improve SketchUp house models")

	plain := &Instance{
		ID:          "plain-1",
		Title:       "Auth",
		ProjectPath: "/tmp/plain",
		GroupPath:   "grp",
		Command:     "claude",
		Tool:        "claude",
		Status:      StatusIdle,
		CreatedAt:   time.Now(),
	}

	if err := s.SaveWithGroups([]*Instance{auto, plain}, nil); err != nil {
		t.Fatalf("SaveWithGroups failed: %v", err)
	}

	loaded, _, err := s.LoadWithGroups()
	if err != nil {
		t.Fatalf("LoadWithGroups failed: %v", err)
	}
	byID := map[string]*Instance{}
	for _, inst := range loaded {
		byID[inst.ID] = inst
	}
	if !byID["auto-1"].AutoName {
		t.Errorf("auto-1.AutoName = false after round-trip, want true")
	}
	if got := byID["auto-1"].GetAutoNameDescription(); got != "Review and improve SketchUp house models" {
		t.Errorf("auto-1 description = %q after round-trip, want the saved task title", got)
	}
	if byID["plain-1"].AutoName {
		t.Errorf("plain-1.AutoName = true after round-trip, want false (default must not leak)")
	}
	if got := byID["plain-1"].GetAutoNameDescription(); got != "" {
		t.Errorf("plain-1 description = %q, want empty", got)
	}

	// LoadLite (CLI fast-path) must preserve both fields too.
	lite, _, err := s.LoadLite()
	if err != nil {
		t.Fatalf("LoadLite failed: %v", err)
	}
	liteByID := map[string]*InstanceData{}
	for _, d := range lite {
		liteByID[d.ID] = d
	}
	if !liteByID["auto-1"].AutoName {
		t.Errorf("LoadLite auto-1.AutoName = false, want true")
	}
	if got := liteByID["auto-1"].AutoNameDescription; got != "Review and improve SketchUp house models" {
		t.Errorf("LoadLite auto-1.AutoNameDescription = %q, want the saved task title", got)
	}
	if liteByID["plain-1"].AutoName {
		t.Errorf("LoadLite plain-1.AutoName = true, want false")
	}
	if got := liteByID["plain-1"].AutoNameDescription; got != "" {
		t.Errorf("LoadLite plain-1.AutoNameDescription = %q, want empty", got)
	}
}

// TestSaveSessionData_PreservesGroupSortOrder verifies that saving session data
// with stored groups preserves the sort_order, matching the fix in #465.
func TestSaveSessionData_PreservesGroupSortOrder(t *testing.T) {
	s := newTestStorage(t)
	now := time.Now()

	instances := []*Instance{
		{
			ID:          "s1",
			Title:       "Session Alpha",
			ProjectPath: "/tmp/alpha",
			GroupPath:   "backend",
			Command:     "claude",
			Tool:        "claude",
			Status:      StatusIdle,
			CreatedAt:   now,
		},
		{
			ID:          "s2",
			Title:       "Session Beta",
			ProjectPath: "/tmp/beta",
			GroupPath:   "frontend",
			Command:     "claude",
			Tool:        "claude",
			Status:      StatusIdle,
			CreatedAt:   now,
		},
		{
			ID:          "s3",
			Title:       "Session Gamma",
			ProjectPath: "/tmp/gamma",
			GroupPath:   "infra",
			Command:     "claude",
			Tool:        "claude",
			Status:      StatusIdle,
			CreatedAt:   now,
		},
	}

	// Simulate user-reordered groups: infra=0, frontend=1, backend=2.
	// Alphabetical would be backend=0, frontend=1, infra=2.
	storedGroups := []*GroupData{
		{Name: "infra", Path: "infra", Expanded: true, Order: 0},
		{Name: "frontend", Path: "frontend", Expanded: true, Order: 1},
		{Name: "backend", Path: "backend", Expanded: false, Order: 2},
	}

	// Save using NewGroupTreeWithGroups (the fixed path).
	groupTree := NewGroupTreeWithGroups(instances, storedGroups)
	if err := s.SaveWithGroups(instances, groupTree); err != nil {
		t.Fatalf("SaveWithGroups failed: %v", err)
	}

	// Reload and verify sort_order is preserved.
	_, reloadedGroups, err := s.LoadWithGroups()
	if err != nil {
		t.Fatalf("LoadWithGroups failed: %v", err)
	}

	groupByPath := make(map[string]*GroupData, len(reloadedGroups))
	for _, g := range reloadedGroups {
		groupByPath[g.Path] = g
	}

	for _, want := range storedGroups {
		got, ok := groupByPath[want.Path]
		if !ok {
			t.Fatalf("group %q not found after reload", want.Path)
		}
		if got.Order != want.Order {
			t.Errorf("group %q: Order = %d, want %d", want.Path, got.Order, want.Order)
		}
		if got.Expanded != want.Expanded {
			t.Errorf("group %q: Expanded = %v, want %v", want.Path, got.Expanded, want.Expanded)
		}
	}

	// Verify the bug: NewGroupTree (without stored groups) loses custom order.
	resetTree := NewGroupTree(instances)
	infraGroup := resetTree.Groups["infra"]
	backendGroup := resetTree.Groups["backend"]
	if infraGroup.Order == 0 && backendGroup.Order == 2 {
		t.Error("NewGroupTree unexpectedly preserved custom order; test premise is wrong")
	}
}
