package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/statedb"
)

// These tests deliberately leave the competing chat out of Agent Deck's DB.
// A native conversation does not become ours just because no registry row owns it.
func TestCodexAutomaticDiscoveryNeverClaimsSameCWD(t *testing.T) {
	for _, bound := range []bool{false, true} {
		name := "unbound"
		if bound {
			name = "bound"
		}
		t.Run(name, func(t *testing.T) {
			inst, codexHome := newCodexGateInstance(t)
			db := withTempGlobalStateDB(t)
			originalID, foreignID := uniqueSID(t), uniqueSID(t)
			writeCodexRolloutWithSource(t, codexHome, originalID, inst.ProjectPath, "cli")
			foreignPath := writeCodexRolloutWithSource(t, codexHome, foreignID, inst.ProjectPath, "cli")
			// The other conversation remains active after this pane's launch.
			inst.CodexStartedAt = time.Now().Add(-time.Second).UnixMilli()
			updated := time.Now().Add(time.Minute)
			if err := os.Chtimes(foreignPath, updated, updated); err != nil {
				t.Fatal(err)
			}
			wantID := ""
			if bound {
				wantID = originalID
			}
			inst.CodexSessionID = wantID
			seedPersistedCodexBinding(t, db, inst, wantID)
			// The existing ownership binding predates both newly-written fixtures;
			// a creation-time floor must not accidentally hide the scan hijack.
			detectedAt := time.Now().Add(-time.Hour)
			revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, wantID, detectedAt, wantID != "")
			if err != nil || !matched {
				t.Fatalf("persist original binding: matched=%v err=%v", matched, err)
			}
			inst.CodexDetectedAt, inst.CodexBindingRevision = detectedAt, revision
			beforeRevision := revision
			rows, err := db.LoadInstances()
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				id, _, _, _, err := db.ReadCodexSessionBinding(row.ID)
				if err != nil {
					t.Fatal(err)
				}
				if id == foreignID {
					t.Fatalf("foreign native chat unexpectedly has a registry owner: %q", row.ID)
				}
			}

			inst.detectCodexSessionAsync()
			inst.UpdateCodexSession(nil)

			gotID, _, revision, _, err := db.ReadCodexSessionBinding(inst.ID)
			if err != nil {
				t.Fatal(err)
			}
			if inst.CodexSessionID != wantID || gotID != wantID || revision != beforeRevision {
				t.Fatalf("automatic discovery stole a same-cwd chat: memory=%q db=%q revision=%d; want %q revision=%d",
					inst.CodexSessionID, gotID, revision, wantID, beforeRevision)
			}
		})
	}
}

func TestCodexStartRetainsArchivedAndHubOnlyBinding(t *testing.T) {
	for _, storage := range []string{"archived", "json_index", "state_db"} {
		t.Run(storage, func(t *testing.T) {
			inst, codexHome := newCodexGateInstance(t)
			id := uniqueSID(t)
			switch storage {
			case "archived":
				path := writeCodexRolloutWithSource(t, codexHome, id, inst.ProjectPath, "cli")
				dir := filepath.Join(codexHome, "archived_sessions")
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(path, filepath.Join(dir, filepath.Base(path))); err != nil {
					t.Fatal(err)
				}
			case "json_index":
				if err := AppendCodexSessionIndexName(codexHome, id, "original conversation", time.Now()); err != nil {
					t.Fatal(err)
				}
			case "state_db":
				writeCodexStateThread(t, codexHome, id, "original conversation", "original prompt", inst.ProjectPath, time.Now().UnixMilli())
			}
			inst.CodexSessionID = id
			inst.CodexDetectedAt = time.Now().Add(-time.Hour)
			detectedAt := inst.CodexDetectedAt
			WriteHookSessionAnchor(inst.ID, id)
			command := inst.buildCodexCommand("codex")
			if !strings.Contains(command, "resume "+id) {
				t.Fatalf("%s-only original conversation was not resumed: %q", storage, command)
			}
			if inst.CodexSessionID != id || !inst.CodexDetectedAt.Equal(detectedAt) || ReadHookSessionAnchor(inst.ID) != id {
				t.Fatalf("resume mutated %s-only binding: id=%q detected=%v anchor=%q", storage,
					inst.CodexSessionID, inst.CodexDetectedAt, ReadHookSessionAnchor(inst.ID))
			}
		})
	}
}

func TestCodexUnknownRestartFailsWithoutClearingBinding(t *testing.T) {
	for _, idState := range []string{"missing", "unverifiable"} {
		for _, launch := range []string{"start", "start_with_message", "restart", "restart_with_env"} {
			t.Run(idState+"/"+launch, func(t *testing.T) {
				inst, _ := newCodexGateInstance(t)
				inst.LastStartedAt = time.Now().Add(-time.Hour)
				id := ""
				if idState == "unverifiable" {
					id = uniqueSID(t)
				}
				inst.CodexSessionID = id
				inst.CodexDetectedAt = time.Now().Add(-time.Hour)
				detectedAt := inst.CodexDetectedAt
				WriteHookSessionAnchor(inst.ID, id)
				var err error
				switch launch {
				case "start":
					err = inst.Start()
				case "start_with_message":
					err = inst.StartWithMessage("")
				case "restart":
					err = inst.Restart()
				case "restart_with_env":
					err = inst.RestartWithEnv(map[string]string{"EXAMPLE": "value"})
				}
				if err == nil || !strings.Contains(strings.ToLower(err.Error()), "codex") || !strings.Contains(err.Error(), "restart-fresh") {
					t.Fatalf("unknown Codex %s must visibly refuse, with an explicit fresh-start remedy; got %v", launch, err)
				}
				if inst.CodexSessionID != id || !inst.CodexDetectedAt.Equal(detectedAt) || ReadHookSessionAnchor(inst.ID) != id {
					t.Fatal("refused restart discarded the original binding")
				}
			})
		}
	}
}

func TestCodexFreshStartIsExplicitOptIn(t *testing.T) {
	inst, codexHome := newCodexGateInstance(t)
	originalID, foreignID := uniqueSID(t), uniqueSID(t)
	writeCodexRolloutWithSource(t, codexHome, originalID, inst.ProjectPath, "cli")
	writeCodexRolloutWithSource(t, codexHome, foreignID, inst.ProjectPath, "cli")
	inst.CodexSessionID = originalID
	inst.CodexBindingConfirmed = true
	inst.LastStartedAt = time.Now().Add(-time.Hour)
	if command := inst.buildCodexCommand("codex"); !strings.Contains(command, "resume "+originalID) {
		t.Fatalf("ordinary launch did not retain the exact conversation: %q", command)
	}

	inst.IsForkAwaitingStart = true
	inst.ForkStartCommand = "codex fork " + originalID
	inst.clearSessionBindingForFreshStart()
	if inst.IsForkAwaitingStart || inst.ForkStartCommand != "" {
		t.Fatal("fresh-start opt-in retained a pending fork of an existing conversation")
	}
	command := inst.buildCodexCommand("codex")
	if inst.CodexSessionID != "" || inst.CodexBindingConfirmed || strings.Contains(command, " resume ") || strings.Contains(command, originalID) || strings.Contains(command, foreignID) {
		t.Fatalf("explicit fresh start reused an existing conversation: id=%q command=%q", inst.CodexSessionID, command)
	}
	inst.UpdateCodexSession(nil)
	if inst.CodexSessionID != "" {
		t.Fatalf("explicit fresh start was rebound by disk discovery: %q", inst.CodexSessionID)
	}
}

// Seed historical state directly: using the explicit binding override would
// model an operator confirmation instead of the legacy association under test.
func seedLegacyCodexBinding(t *testing.T, inst *Instance, revision int64) string {
	t.Helper()
	db := withTempGlobalStateDB(t)
	id := uniqueSID(t)
	detectedAt := time.Now().Add(-time.Hour).Truncate(time.Second)
	data, err := json.Marshal(map[string]any{
		"codex_session_id":       id,
		"codex_detected_at":      detectedAt.Unix(),
		"codex_binding_revision": revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveInstance(&statedb.InstanceRow{
		ID: inst.ID, Title: inst.Title, ProjectPath: inst.ProjectPath,
		Command: "codex", Tool: "codex", Status: "idle",
		CreatedAt: detectedAt, ToolData: data,
	}); err != nil {
		t.Fatal(err)
	}
	inst.stateDB = db
	inst.Command, inst.CodexSessionID = "codex", id
	inst.CodexDetectedAt, inst.CodexBindingRevision = detectedAt, revision
	inst.LastStartedAt, inst.addedThisProcess = detectedAt, false
	return id
}

func TestCodexLegacyBindingRefusesLaunchPreflight(t *testing.T) {
	for _, revision := range []int64{0, 1} {
		for _, restarting := range []bool{false, true} {
			name := "legacy_revision_zero"
			if revision == 1 {
				name = "already_migrated_revision_one"
			}
			if restarting {
				name += "/restart"
			} else {
				name += "/start"
			}
			t.Run(name, func(t *testing.T) {
				inst, home := newCodexGateInstance(t)
				wrongID := seedLegacyCodexBinding(t, inst, revision)
				path := writeCodexRolloutWithSource(t, home, wrongID, inst.ProjectPath, "cli")
				history, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				detectedAt := inst.CodexDetectedAt
				inst.tmuxSession = nil
				if inst.codexHookOwnsCandidate(&HookStatus{SessionID: wrongID}) {
					t.Error("same-ID hook shortcut authorized an unconfirmed legacy identity")
				}

				if err := inst.prepareCodexIdentity(restarting); err == nil || !strings.Contains(err.Error(), "refusing Codex launch") {
					t.Errorf("unconfirmed legacy association must refuse before launch, even when its native history exists: %v", err)
				}
				gotID, gotDetectedAt, gotRevision, confirmed, err := inst.stateDB.ReadCodexSessionBinding(inst.ID)
				if err != nil || gotID != wrongID || inst.CodexSessionID != wrongID ||
					gotRevision != revision || inst.CodexBindingRevision != revision || confirmed || inst.CodexBindingConfirmed ||
					!gotDetectedAt.Equal(detectedAt) || !inst.CodexDetectedAt.Equal(detectedAt) {
					t.Errorf("refusal changed the historical binding: memory=%q/%v db=%q/%v err=%v",
						inst.CodexSessionID, inst.CodexDetectedAt, gotID, gotDetectedAt, err)
				}
				if after, err := os.ReadFile(path); err != nil || string(after) != string(history) {
					t.Errorf("refusal changed native history: %v", err)
				}
			})
		}
	}
}

func TestCodexLegacyConfirmationSurvivesStaleSaveAndClear(t *testing.T) {
	for _, revision := range []int64{0, 1} {
		for _, sameID := range []bool{false, true} {
			name := "legacy_revision_zero"
			if revision == 1 {
				name = "already_migrated_revision_one"
			}
			if sameID {
				name += "/same_id"
			} else {
				name += "/corrected_id"
			}
			t.Run(name, func(t *testing.T) {
				inst, home := newCodexGateInstance(t)
				wrongID := seedLegacyCodexBinding(t, inst, revision)
				writeCodexRolloutWithSource(t, home, wrongID, inst.ProjectPath, "cli")
				storage := &Storage{db: inst.stateDB}
				load := func() *Instance {
					t.Helper()
					instances, err := storage.Load()
					if err != nil || len(instances) != 1 {
						t.Fatalf("load binding: instances=%d err=%v", len(instances), err)
					}
					return instances[0]
				}
				inst.Notes = "unrelated metadata edit"
				if err := storage.SaveWithGroups([]*Instance{inst}, nil); err != nil {
					t.Fatal(err)
				}
				legacy := load()
				if legacy.CodexBindingConfirmed || legacy.CodexBindingRevision != revision || legacy.prepareCodexIdentity(false) == nil {
					t.Fatal("metadata save/reload laundered an unconfirmed legacy binding")
				}
				stale := load()
				selectedID := wrongID
				if !sameID {
					selectedID = uniqueSID(t)
					writeCodexRolloutWithSource(t, home, selectedID, inst.ProjectPath, "cli")
				}
				if _, _, err := SetField(legacy, FieldCodexSessionID, selectedID, nil); err != nil {
					t.Fatalf("confirm verified selection: %v", err)
				}
				if err := storage.SaveWithGroups([]*Instance{legacy}, nil); err != nil {
					t.Fatal(err)
				}
				if err := storage.SaveWithGroups([]*Instance{stale}, nil); err != nil {
					t.Fatal(err)
				}
				confirmed := load()
				if confirmed.CodexSessionID != selectedID || !confirmed.CodexBindingConfirmed || confirmed.CodexBindingRevision <= revision {
					t.Fatalf("stale save lost explicit confirmation: id=%q confirmed=%v revision=%d", confirmed.CodexSessionID, confirmed.CodexBindingConfirmed, confirmed.CodexBindingRevision)
				}
				if err := confirmed.prepareCodexIdentity(true); err != nil {
					t.Fatalf("explicitly confirmed conversation cannot resume without live evidence: %v", err)
				}
				if command := confirmed.buildCodexCommand("codex"); !strings.Contains(command, "resume "+selectedID) {
					t.Fatalf("confirmed repair did not resume exact selection: %q", command)
				}
				stale = load()
				if _, _, err := SetField(confirmed, FieldCodexSessionID, "", nil); err != nil {
					t.Fatal(err)
				}
				if err := storage.SaveWithGroups([]*Instance{confirmed}, nil); err != nil {
					t.Fatal(err)
				}
				if err := storage.SaveWithGroups([]*Instance{stale}, nil); err != nil {
					t.Fatal(err)
				}
				cleared := load()
				if cleared.CodexSessionID != "" || cleared.CodexBindingConfirmed {
					t.Fatal("stale confirmed snapshot resurrected an explicitly cleared binding")
				}
			})
		}
	}
}

func TestCodexExplicitConfirmationRequiresNativeEvidence(t *testing.T) {
	inst, home := newCodexGateInstance(t)
	wrongID := seedLegacyCodexBinding(t, inst, 1)
	writeCodexRolloutWithSource(t, home, wrongID, inst.ProjectPath, "cli")
	missingID := uniqueSID(t)
	if _, _, err := SetField(inst, FieldCodexSessionID, missingID, nil); err == nil {
		t.Fatal("explicit set confirmed an ID with no native evidence")
	}
	if inst.CodexSessionID != wrongID || inst.CodexBindingConfirmed || inst.codexSessionBindingOverrideIntent {
		t.Fatal("failed confirmation mutated the legacy binding")
	}
}
