package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
			revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, wantID, detectedAt)
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
				id, _, _, err := db.ReadCodexSessionBinding(row.ID)
				if err != nil {
					t.Fatal(err)
				}
				if id == foreignID {
					t.Fatalf("foreign native chat unexpectedly has a registry owner: %q", row.ID)
				}
			}

			inst.detectCodexSessionAsync()
			inst.UpdateCodexSession(nil)

			gotID, _, revision, err := db.ReadCodexSessionBinding(inst.ID)
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
	if inst.CodexSessionID != "" || strings.Contains(command, " resume ") || strings.Contains(command, originalID) || strings.Contains(command, foreignID) {
		t.Fatalf("explicit fresh start reused an existing conversation: id=%q command=%q", inst.CodexSessionID, command)
	}
	inst.UpdateCodexSession(nil)
	if inst.CodexSessionID != "" {
		t.Fatalf("explicit fresh start was rebound by disk discovery: %q", inst.CodexSessionID)
	}
}
