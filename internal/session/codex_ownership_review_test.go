package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeCodexReviewRolloutAt(t *testing.T, home, sessionID, cwd string, createdAt time.Time) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"timestamp": createdAt.UTC().Format(time.RFC3339Nano),
		"type":      "session_meta",
		"payload": map[string]any{
			"id": sessionID, "session_id": sessionID, "cwd": cwd, "source": "cli",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeCodexRolloutBody(t, home, sessionID, string(data)+"\n")
	return filepath.Join(home, "sessions", "2026", "06", "30", "rollout-2026-06-30T10-00-00-"+sessionID+".jsonl")
}

// The emitter is the pane's initial process, not a reader in the test process or
// a second Codex child. Wait for its writable descriptors before using evidence.
func startCodexReviewPane(t *testing.T, inst *Instance, paths ...string) *HookStatus {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux /proc writable-descriptor inspection")
	}
	skipIfNoTmuxBinary(t)
	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	for index, path := range paths {
		fmt.Fprintf(&script, "exec %d>> %s\n", index+3, shellQuote(path))
	}
	script.WriteString("while :; do sleep 1; done\n")
	binary := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(binary, []byte(script.String()), 0o700); err != nil {
		t.Fatal(err)
	}
	inst.tmuxSession.RunCommandAsInitialProcess = true
	if err := inst.tmuxSession.Start("exec " + shellQuote(binary)); err != nil {
		t.Fatalf("start writable Codex emitter: %v", err)
	}
	t.Cleanup(func() { _ = inst.tmuxSession.Kill() })
	status := ownedCodexHook(t, inst, &HookStatus{
		Status: "running", Event: "thread/started", UpdatedAt: time.Now(),
	})
	deadline := time.Now().Add(2 * time.Second)
	for {
		open := make(map[string]bool)
		for _, candidate := range extractCodexRolloutCandidatesFromProcFD(status.CodexEmitterPID) {
			open[candidate.Path] = true
		}
		ready := true
		for _, path := range paths {
			ready = ready && open[path]
		}
		if ready {
			return status
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("Codex emitter did not open writable rollouts: got %v, want %q", open, paths)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func writeCodexReviewHook(t *testing.T, inst *Instance, status *HookStatus) {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"status": status.Status, "session_id": status.SessionID, "event": status.Event,
		"ts": status.UpdatedAt.Unix(), "codex_emitter_pid": status.CodexEmitterPID,
		"codex_emitter_identity": status.CodexEmitterIdentity,
	})
	if err != nil {
		t.Fatal(err)
	}
	path := hookStatusFilePath(inst.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readHookStatusFile(inst.ID); got == nil || got.SessionID != status.SessionID || got.CodexEmitterIdentity != status.CodexEmitterIdentity {
		t.Fatalf("pane-owned hook fixture was not readable: %+v", got)
	}
}

func TestCodexRestartRefusesUnadoptedPaneConversation(t *testing.T) {
	for _, evidence := range []string{"process", "hook"} {
		for _, rejection := range []string{"older_binding_floor", "another_owner"} {
			t.Run(evidence+"/"+rejection, func(t *testing.T) {
				inst, home := newCodexGateInstance(t)
				db := withTempGlobalStateDB(t)
				originalID, actualID := uniqueSID(t), uniqueSID(t)
				detectedAt := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
				writeCodexReviewRolloutAt(t, home, originalID, inst.ProjectPath, detectedAt.Add(-time.Hour))
				actualCreatedAt := detectedAt.Add(-2 * time.Hour)
				if rejection == "another_owner" {
					actualCreatedAt = detectedAt.Add(time.Hour)
					owner := NewInstanceWithTool("actual-chat-owner", inst.ProjectPath, "codex")
					seedPersistedCodexBinding(t, db, owner, actualID)
				}
				actualPath := writeCodexReviewRolloutAt(t, home, actualID, inst.ProjectPath, actualCreatedAt)
				inst.Command, inst.CodexSessionID = "codex", originalID
				seedPersistedCodexBinding(t, db, inst, originalID)
				revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, originalID, detectedAt)
				if err != nil || !matched {
					t.Fatalf("seed recorded binding floor: matched=%v err=%v", matched, err)
				}
				inst.CodexDetectedAt, inst.CodexBindingRevision = detectedAt, revision
				WriteHookSessionAnchor(inst.ID, originalID)
				var status *HookStatus
				if evidence == "process" {
					status = startCodexReviewPane(t, inst, actualPath)
					if probe := inst.queryCodexSessionFromProcessFiles(); probe.sessionID != actualID || probe.ambiguous {
						t.Fatalf("live pane did not prove its single actual conversation: %+v", probe)
					}
				} else {
					status = startCodexReviewPane(t, inst)
					status.SessionID = actualID
					if !inst.codexHookEmitterOwns(status) {
						t.Fatal("hook fixture does not identify the pane's live emitter")
					}
					if probe := inst.queryCodexSessionFromProcessFiles(); probe.sessionID != "" || probe.ambiguous {
						t.Fatalf("hook-only fixture unexpectedly has process-file evidence: %+v", probe)
					}
					writeCodexReviewHook(t, inst, status)
				}

				if err := inst.Restart(); err == nil || !strings.Contains(strings.ToLower(err.Error()), "codex") {
					t.Fatalf("restart must visibly refuse an unadopted %s conversation, not resume recorded A: %v", evidence, err)
				}
				if !inst.tmuxSession.Exists() || !inst.codexHookEmitterOwns(status) {
					t.Fatal("refused restart killed or replaced the pane containing the actual conversation")
				}
				gotID, gotDetectedAt, gotRevision, err := db.ReadCodexSessionBinding(inst.ID)
				if err != nil || inst.CodexSessionID != originalID || !inst.CodexDetectedAt.Equal(detectedAt) ||
					inst.CodexBindingRevision != revision || gotID != originalID || !gotDetectedAt.Equal(detectedAt) || gotRevision != revision {
					t.Fatalf("refusal changed recorded binding: memory=%q/%v/%d db=%q/%v/%d err=%v",
						inst.CodexSessionID, inst.CodexDetectedAt, inst.CodexBindingRevision, gotID, gotDetectedAt, gotRevision, err)
				}
			})
		}
	}
}

func TestCodexAmbiguousPaneFilesCannotAuthorizeHookRebindOrRestart(t *testing.T) {
	inst, home := newCodexGateInstance(t)
	db := withTempGlobalStateDB(t)
	originalID, switchedID := uniqueSID(t), uniqueSID(t)
	detectedAt := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	originalPath := writeCodexReviewRolloutAt(t, home, originalID, inst.ProjectPath, detectedAt.Add(-time.Hour))
	// B is eligible by creation time; only competing writable pane evidence
	// prevents its otherwise valid hook from replacing A.
	switchedPath := writeCodexReviewRolloutAt(t, home, switchedID, inst.ProjectPath, detectedAt.Add(time.Hour))
	inst.Command, inst.CodexSessionID = "codex", originalID
	seedPersistedCodexBinding(t, db, inst, originalID)
	revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, originalID, detectedAt)
	if err != nil || !matched {
		t.Fatalf("seed recorded binding floor: matched=%v err=%v", matched, err)
	}
	inst.CodexDetectedAt, inst.CodexBindingRevision = detectedAt, revision
	WriteHookSessionAnchor(inst.ID, originalID)
	status := startCodexReviewPane(t, inst, originalPath, switchedPath)
	status.SessionID = switchedID
	if !inst.codexHookEmitterOwns(status) {
		t.Fatal("two descriptors must belong to one verified pane emitter")
	}
	if probe := inst.queryCodexSessionFromProcessFiles(); probe.sessionID != "" || !probe.ambiguous {
		t.Fatalf("two writable top-level conversations must remain ambiguous: %+v", probe)
	}
	writeCodexReviewHook(t, inst, status)

	inst.UpdateHookStatus(status)
	gotID, _, gotRevision, err := db.ReadCodexSessionBinding(inst.ID)
	if err != nil || inst.CodexSessionID != originalID || gotID != originalID || gotRevision != revision {
		t.Fatalf("ambiguous process evidence let a hook rebind: memory=%q db=%q revision=%d want=%d err=%v",
			inst.CodexSessionID, gotID, gotRevision, revision, err)
	}
	if err := inst.prepareCodexIdentity(true); err == nil || !strings.Contains(strings.ToLower(err.Error()), "codex") {
		t.Fatalf("ambiguous live pane silently authorized restart: %v", err)
	}
	gotID, _, gotRevision, err = db.ReadCodexSessionBinding(inst.ID)
	if err != nil || inst.CodexSessionID != originalID || !inst.CodexDetectedAt.Equal(detectedAt) ||
		inst.CodexBindingRevision != revision || gotID != originalID || gotRevision != revision {
		t.Fatalf("ambiguous restart changed binding: memory=%q/%v/%d db=%q/%d err=%v",
			inst.CodexSessionID, inst.CodexDetectedAt, inst.CodexBindingRevision, gotID, gotRevision, err)
	}
	if !inst.tmuxSession.Exists() || !inst.codexHookEmitterOwns(status) {
		t.Fatal("ambiguous launch preflight disturbed the original pane emitter")
	}
}

func TestCodexSandboxExactResumeUsesOnlySandboxHome(t *testing.T) {
	for _, storage := range []string{"live_rollout", "archive", "json_index"} {
		for _, location := range []string{"sandbox", "host"} {
			t.Run(storage+"/"+location, func(t *testing.T) {
				inst, hostHome := newCodexGateInstance(t)
				db := withTempGlobalStateDB(t)
				inst.Sandbox = NewSandboxConfig("")
				home := filepath.Join(os.Getenv("HOME"), ".codex", "sandbox")
				if location == "host" {
					home = hostHome
				}
				id := uniqueSID(t)
				switch storage {
				case "live_rollout":
					writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
				case "archive":
					path := writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
					directory := filepath.Join(home, "archived_sessions")
					if err := os.MkdirAll(directory, 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(path, filepath.Join(directory, filepath.Base(path))); err != nil {
						t.Fatal(err)
					}
				case "json_index":
					if err := AppendCodexSessionIndexName(home, id, "exact sandbox conversation", time.Now()); err != nil {
						t.Fatal(err)
					}
				}
				inst.Command, inst.CodexSessionID = "codex", id
				seedPersistedCodexBinding(t, db, inst, id)
				detectedAt := time.Now().Add(-time.Hour).Truncate(time.Second)
				revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, id, detectedAt)
				if err != nil || !matched {
					t.Fatalf("seed exact sandbox binding: matched=%v err=%v", matched, err)
				}
				inst.CodexDetectedAt, inst.CodexBindingRevision = detectedAt, revision

				err = inst.prepareCodexIdentity(true)
				if location == "sandbox" && err != nil {
					t.Fatalf("exact %s in sandbox home was refused without Docker: %v", storage, err)
				}
				if location == "sandbox" {
					command := inst.buildCodexCommand(inst.Command)
					if strings.Count(command, "resume "+id) != 1 || strings.Contains(command, hostHome) ||
						!strings.Contains(command, "CODEX_HOME="+codexSandboxContainerHome) {
						t.Fatalf("sandbox resume must pin its mounted container home, not host CODEX_HOME %q: %q", hostHome, command)
					}
				}
				if location == "host" && (err == nil || !strings.Contains(strings.ToLower(err.Error()), "codex")) {
					t.Fatalf("host-only %s authorized a sandbox resume: %v", storage, err)
				}
				gotID, gotDetectedAt, gotRevision, readErr := db.ReadCodexSessionBinding(inst.ID)
				if readErr != nil || inst.CodexSessionID != id || !inst.CodexDetectedAt.Equal(detectedAt) ||
					inst.CodexBindingRevision != revision || gotID != id || !gotDetectedAt.Equal(detectedAt) || gotRevision != revision {
					t.Fatalf("sandbox preflight changed exact binding: memory=%q/%v/%d db=%q/%v/%d err=%v",
						inst.CodexSessionID, inst.CodexDetectedAt, inst.CodexBindingRevision, gotID, gotDetectedAt, gotRevision, readErr)
				}
			})
		}
	}
}

func TestCodexSandboxContainerDefaultHomeCanResumeExactBinding(t *testing.T) {
	inst, hostHome := newCodexGateInstance(t)
	db := withTempGlobalStateDB(t)
	inst.Sandbox = NewSandboxConfig("")
	id := uniqueSID(t)
	home := filepath.Join(os.Getenv("HOME"), ".codex", "sandbox")
	writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
	inst.Command = "CODEX_HOME=/root/.codex codex resume " + id
	inst.CodexSessionID = id
	seedPersistedCodexBinding(t, db, inst, id)
	detectedAt := time.Now().Add(-time.Hour).Truncate(time.Second)
	revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, id, detectedAt)
	if err != nil || !matched {
		t.Fatalf("seed container-default binding: matched=%v err=%v", matched, err)
	}
	inst.CodexDetectedAt, inst.CodexBindingRevision = detectedAt, revision

	if err := inst.prepareCodexIdentity(true); err != nil {
		t.Fatalf("known container-default Codex home refused exact sandbox resume: %v", err)
	}
	command := inst.buildCodexCommand(inst.Command)
	if strings.Count(command, "resume "+id) != 1 || !strings.Contains(command, "CODEX_HOME=/root/.codex") || strings.Contains(command, hostHome) {
		t.Fatalf("known container-default command lost its exact resume or exported the host home: %q", command)
	}
	gotID, gotDetectedAt, gotRevision, err := db.ReadCodexSessionBinding(inst.ID)
	if err != nil || inst.CodexSessionID != id || !inst.CodexDetectedAt.Equal(detectedAt) ||
		inst.CodexBindingRevision != revision || gotID != id || !gotDetectedAt.Equal(detectedAt) || gotRevision != revision {
		t.Fatalf("container-default resume changed binding: memory=%q/%v/%d db=%q/%v/%d err=%v",
			inst.CodexSessionID, inst.CodexDetectedAt, inst.CodexBindingRevision, gotID, gotDetectedAt, gotRevision, err)
	}
}

func TestCodexSandboxUnmappedHomeRefusesBeforeFreshRestartMutation(t *testing.T) {
	for _, source := range []string{
		"inline_command", "configured_command", "selected_account", "ambient_home", "global_config",
		"repeated_inline_unmapped", "repeated_inline_empty", "inline_empty",
		"docker_environment", "docker_environment_values", "docker_home_volume",
	} {
		t.Run(source, func(t *testing.T) {
			inst, _ := newCodexGateInstance(t)
			db := withTempGlobalStateDB(t)
			home := filepath.Join(os.Getenv("HOME"), ".codex", "sandbox")
			unmappedHome := filepath.Join(t.TempDir(), "unmapped-codex")
			id := uniqueSID(t)
			path := writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
			// Even valid metadata at the unmapped host path cannot make that
			// path the Codex home mounted in this sandbox.
			writeCodexRolloutWithSource(t, unmappedHome, id, inst.ProjectPath, "cli")
			inst.Command, inst.CodexSessionID = "codex", id
			seedPersistedCodexBinding(t, db, inst, id)
			detectedAt := time.Now().Add(-time.Hour).Truncate(time.Second)
			revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, id, detectedAt)
			if err != nil || !matched {
				t.Fatalf("seed sandbox binding: matched=%v err=%v", matched, err)
			}
			inst.CodexDetectedAt, inst.CodexBindingRevision = detectedAt, revision
			WriteHookSessionAnchor(inst.ID, id)
			status := startCodexReviewPane(t, inst, path)
			status.SessionID = id
			inst.Sandbox = NewSandboxConfig("")
			cfg := &UserConfig{}
			switch source {
			case "inline_command":
				inst.Command = "CODEX_HOME=" + shellQuote(unmappedHome) + " codex"
			case "repeated_inline_unmapped":
				inst.Command = "CODEX_HOME=/root/.codex CODEX_HOME=" + shellQuote(unmappedHome) + " codex"
			case "repeated_inline_empty":
				inst.Command = "CODEX_HOME=/root/.codex CODEX_HOME='' codex"
			case "inline_empty":
				inst.Command = "CODEX_HOME='' codex"
			case "configured_command":
				cfg.Codex.Command = "CODEX_HOME=" + shellQuote(unmappedHome) + " codex"
			case "selected_account":
				inst.Account = "unmapped-account"
				cfg.Profiles = map[string]ProfileSettings{
					inst.Account: {Codex: ProfileCodexSettings{ConfigDir: unmappedHome}},
				}
			case "ambient_home":
				t.Setenv("CODEX_HOME", unmappedHome)
			case "global_config":
				t.Setenv("CODEX_HOME", "")
				cfg.Codex.ConfigDir = unmappedHome
			case "docker_environment":
				// The ordinary host source is known on the host, but forwarding
				// that path verbatim does not make it valid inside the container.
				cfg.Docker.Environment = []string{"CODEX_HOME"}
			case "docker_environment_values":
				cfg.Docker.EnvironmentValues = map[string]string{"CODEX_HOME": unmappedHome}
			case "docker_home_volume":
				cfg.Docker.ExtraVolumes = map[string]string{unmappedHome: codexSandboxContainerHome}
			}
			t.Cleanup(resetUserConfigCache(t, cfg))
			// Guard the daemon boundary even when this regression is exercised
			// against an implementation that incorrectly reaches Docker.
			bin := t.TempDir()
			dockerMarker := filepath.Join(bin, "docker-invoked")
			script := "#!/bin/sh\n: > " + shellQuote(dockerMarker) + "\nexit 99\n"
			if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			assertPreserved := func(consumer string) {
				t.Helper()
				gotID, gotDetectedAt, gotRevision, readErr := db.ReadCodexSessionBinding(inst.ID)
				if readErr != nil || inst.CodexSessionID != id || !inst.CodexDetectedAt.Equal(detectedAt) ||
					inst.CodexBindingRevision != revision || gotID != id || !gotDetectedAt.Equal(detectedAt) ||
					gotRevision != revision || ReadHookSessionAnchor(inst.ID) != id {
					t.Fatalf("%s changed the binding before rejecting unmapped home: memory=%q/%v/%d db=%q/%v/%d err=%v",
						consumer, inst.CodexSessionID, inst.CodexDetectedAt, inst.CodexBindingRevision, gotID, gotDetectedAt, gotRevision, readErr)
				}
				if !inst.tmuxSession.Exists() || !inst.codexHookEmitterOwns(status) {
					t.Fatalf("%s killed or replaced the live pane before rejecting unmapped home", consumer)
				}
				if _, err := os.Stat(dockerMarker); !os.IsNotExist(err) {
					t.Fatalf("%s contacted Docker instead of refusing locally: %v", consumer, err)
				}
			}

			if err := inst.prepareCodexIdentity(true); err == nil || !strings.Contains(strings.ToLower(err.Error()), "codex") {
				t.Fatalf("sandbox preflight must visibly refuse unmapped %s home: %v", source, err)
			}
			assertPreserved("prepareCodexIdentity")
			if err := inst.RestartFresh(); err == nil || !strings.Contains(strings.ToLower(err.Error()), "codex") {
				t.Fatalf("RestartFresh must visibly refuse unmapped %s home before mutation: %v", source, err)
			}
			assertPreserved("RestartFresh")
		})
	}
}

func TestCodexProcessFilesRejectCompetingTopLevelChats(t *testing.T) {
	inst, codexHome := newCodexGateInstance(t)
	originalID, foreignID := uniqueSID(t), uniqueSID(t)
	originalPath := writeCodexRolloutWithSource(t, codexHome, originalID, inst.ProjectPath, "cli")
	foreignPath := writeCodexRolloutWithSource(t, codexHome, foreignID, inst.ProjectPath, "cli")
	updated := time.Now().Add(time.Hour)
	if err := os.Chtimes(foreignPath, updated, updated); err != nil {
		t.Fatal(err)
	}
	if probe := selectCodexRolloutCandidate([]codexRolloutCandidate{{ID: originalID, Path: originalPath}, {ID: foreignID, Path: foreignPath}}); probe.sessionID != "" || !probe.ambiguous {
		t.Fatalf("multiple open top-level chats are ambiguous, not newest-wins: %+v", probe)
	}
}
