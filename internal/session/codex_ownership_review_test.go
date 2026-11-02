package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const codexReviewScriptHeader = "#!/bin/sh\nfor arg in \"$@\"; do\nif [ \"$arg\" = --help ]; then printf '%s\\n' '  --no-daemon  Bypass an existing daemon'; exit 0; fi\ndone\n"

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
	script.WriteString(codexReviewScriptHeader)
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
				revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, originalID, detectedAt, originalID != "")
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
				gotID, gotDetectedAt, gotRevision, _, err := db.ReadCodexSessionBinding(inst.ID)
				if err != nil || inst.CodexSessionID != originalID || !inst.CodexDetectedAt.Equal(detectedAt) ||
					inst.CodexBindingRevision != revision || gotID != originalID || !gotDetectedAt.Equal(detectedAt) || gotRevision != revision {
					t.Fatalf("refusal changed recorded binding: memory=%q/%v/%d db=%q/%v/%d err=%v",
						inst.CodexSessionID, inst.CodexDetectedAt, inst.CodexBindingRevision, gotID, gotDetectedAt, gotRevision, err)
				}
			})
		}
	}
}

func TestCodexLegacyBindingRestartPreservesPane(t *testing.T) {
	for _, revision := range []int64{0, 1} {
		for _, evidence := range []string{"none", "same_id_process", "same_id_hook"} {
			t.Run(fmt.Sprintf("revision_%d/%s", revision, evidence), func(t *testing.T) {
				inst, home := newCodexGateInstance(t)
				wrongID := seedLegacyCodexBinding(t, inst, revision)
				path := writeCodexRolloutWithSource(t, home, wrongID, inst.ProjectPath, "cli")
				history, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				detectedAt := inst.CodexDetectedAt
				var status *HookStatus
				if evidence == "same_id_process" {
					status = startCodexReviewPane(t, inst, path)
					if probe := inst.queryCodexSessionFromProcessFiles(); probe.sessionID != wrongID || probe.ambiguous {
						t.Fatalf("wrong-resume fixture did not expose its exact conversation: %+v", probe)
					}
				} else {
					status = startCodexReviewPane(t, inst)
					if evidence == "same_id_hook" {
						status.SessionID = wrongID
						writeCodexReviewHook(t, inst, status)
						inst.UpdateHookStatus(status)
					}
				}

				if err := inst.Restart(); err == nil || !strings.Contains(err.Error(), "refusing Codex launch") {
					t.Errorf("legacy binding must require explicit confirmation, not circular %s evidence: %v", evidence, err)
				}
				if !inst.tmuxSession.Exists() || !inst.codexHookEmitterOwns(status) {
					t.Error("refused restart killed or replaced the existing pane process")
				}
				gotID, gotDetectedAt, gotRevision, confirmed, err := inst.stateDB.ReadCodexSessionBinding(inst.ID)
				if err != nil || gotID != wrongID || inst.CodexSessionID != wrongID ||
					gotRevision != revision || inst.CodexBindingRevision != revision || confirmed || inst.CodexBindingConfirmed ||
					!gotDetectedAt.Equal(detectedAt) || !inst.CodexDetectedAt.Equal(detectedAt) {
					t.Errorf("refusal changed historical binding: memory=%q/%v db=%q/%v err=%v",
						inst.CodexSessionID, inst.CodexDetectedAt, gotID, gotDetectedAt, err)
				}
				if after, err := os.ReadFile(path); err != nil || string(after) != string(history) {
					t.Errorf("refusal changed native history: %v", err)
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
	revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, originalID, detectedAt, originalID != "")
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
	gotID, _, gotRevision, _, err := db.ReadCodexSessionBinding(inst.ID)
	if err != nil || inst.CodexSessionID != originalID || gotID != originalID || gotRevision != revision {
		t.Fatalf("ambiguous process evidence let a hook rebind: memory=%q db=%q revision=%d want=%d err=%v",
			inst.CodexSessionID, gotID, gotRevision, revision, err)
	}
	if err := inst.prepareCodexIdentity(true); err == nil || !strings.Contains(strings.ToLower(err.Error()), "codex") {
		t.Fatalf("ambiguous live pane silently authorized restart: %v", err)
	}
	gotID, _, gotRevision, _, err = db.ReadCodexSessionBinding(inst.ID)
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
				revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, id, detectedAt, id != "")
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
				gotID, gotDetectedAt, gotRevision, _, readErr := db.ReadCodexSessionBinding(inst.ID)
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
	revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, id, detectedAt, id != "")
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
	gotID, gotDetectedAt, gotRevision, _, err := db.ReadCodexSessionBinding(inst.ID)
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
			revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, id, detectedAt, id != "")
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
				gotID, gotDetectedAt, gotRevision, _, readErr := db.ReadCodexSessionBinding(inst.ID)
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

func TestCodexUnconfirmedBindingRefusesEveryLaunchEntryPoint(t *testing.T) {
	for _, launch := range []string{"start", "start_with_message", "restart", "restart_with_env"} {
		t.Run(launch, func(t *testing.T) {
			inst, home := newCodexGateInstance(t)
			id := seedLegacyCodexBinding(t, inst, 1)
			writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
			status := startCodexReviewPane(t, inst)
			var err error
			switch launch {
			case "start":
				err = inst.Start()
			case "start_with_message":
				err = inst.StartWithMessage("do not deliver to the unconfirmed conversation")
			case "restart":
				err = inst.Restart()
			case "restart_with_env":
				err = inst.RestartWithEnv(map[string]string{"EXAMPLE": "value"})
			}
			if err == nil || !strings.Contains(err.Error(), "codex-session-id") {
				t.Fatalf("%s did not refuse with the explicit-confirmation remedy: %v", launch, err)
			}
			if !inst.tmuxSession.Exists() || !inst.codexHookEmitterOwns(status) {
				t.Fatalf("%s replaced the unconfirmed existing pane", launch)
			}
			if inst.CodexSessionID != id || inst.CodexBindingConfirmed {
				t.Fatalf("%s changed the unconfirmed binding: id=%q confirmed=%v", launch, inst.CodexSessionID, inst.CodexBindingConfirmed)
			}
		})
	}
}

func waitCodexReviewPIDFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			return pid
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("fixture did not publish its process ID at %s: %q", path, data)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCodexExitToShellCannotTransferFreshCaptureToLaterResume(t *testing.T) {
	for _, evidence := range []string{"hook", "process"} {
		t.Run(evidence, func(t *testing.T) {
			if runtime.GOOS != "linux" {
				t.Skip("requires Linux process lifetime and writable-descriptor inspection")
			}
			skipIfNoTmuxBinary(t)
			inst, home := newCodexGateInstance(t)
			db := withTempGlobalStateDB(t)
			storage := &Storage{db: db}
			oldID := uniqueSID(t)
			path := writeCodexReviewRolloutAt(t, home, oldID, inst.ProjectPath, time.Now().Add(-time.Hour))
			history, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			binary := filepath.Join(dir, "codex")
			freshPIDPath := filepath.Join(dir, "fresh-pid")
			resumePIDPath := filepath.Join(dir, "resume-pid")
			shellPIDPath := filepath.Join(dir, "shell-pid")
			exitTrigger := filepath.Join(dir, "exit-fresh")
			resumeTrigger := filepath.Join(dir, "resume-old")
			writable := ""
			if evidence == "process" {
				writable = "exec 3>> " + shellQuote(path) + "\n"
			}
			script := codexReviewScriptHeader +
				"if [ \"$1\" = --no-daemon ]; then shift; fi\n" +
				"if [ \"$1\" = resume ]; then\n" +
				"[ \"$2\" = " + shellQuote(oldID) + " ] || exit 90\n" +
				writable +
				"printf '%s\\n' \"$$\" > " + shellQuote(resumePIDPath) + "\n" +
				"while :; do sleep 1; done\n" +
				"fi\n" +
				"[ \"$#\" -eq 0 ] || exit 91\n" +
				"printf '%s\\n' \"$$\" > " + shellQuote(freshPIDPath) + "\n" +
				"while [ ! -f " + shellQuote(exitTrigger) + " ]; do sleep 0.02; done\n"
			if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			fallback := filepath.Join(dir, "fallback-shell")
			script = "#!/bin/sh\n" +
				"printf '%s\\n' \"$$\" > " + shellQuote(shellPIDPath) + "\n" +
				"while [ ! -f " + shellQuote(resumeTrigger) + " ]; do sleep 0.02; done\n" +
				shellQuote(binary) + " resume " + oldID + "\n" +
				"while :; do sleep 1; done\n"
			if err := os.WriteFile(fallback, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(resetUserConfigCache(t, &UserConfig{
				Shell: ShellSettings{InitScript: "export SHELL=" + shellQuote(fallback)},
			}))
			inst.Command = shellQuote(binary)
			inst.ExitToShell = boolPtr(true)
			if err := storage.InsertSessionAndVerify(inst, nil); err != nil {
				t.Fatal(err)
			}
			if err := inst.Start(); err != nil {
				t.Fatalf("start supported exit-to-shell launch: %v", err)
			}
			t.Cleanup(func() { _ = inst.tmuxSession.Kill() })
			freshPID := waitCodexReviewPIDFile(t, freshPIDPath)
			fresh := ownedCodexHook(t, inst, &HookStatus{})
			panePID := inst.readPanePID()
			paneIdentity := CodexProcessIdentity(panePID)
			if fresh.CodexEmitterPID != freshPID || freshPID == panePID || paneIdentity == "" || !inst.codexHookEmitterOwns(fresh) {
				t.Fatalf("fixture must launch a fresh Codex child under a surviving pane leader: pane=%d/%q emitter=%+v", panePID, paneIdentity, fresh)
			}
			if err := storage.SaveWithGroups([]*Instance{inst}, nil); err != nil {
				t.Fatal(err)
			}
			instances, err := storage.Load()
			if err != nil || len(instances) != 1 {
				t.Fatalf("reload fresh exit-to-shell launch: instances=%d err=%v", len(instances), err)
			}
			reloaded := instances[0]
			beforeID, beforeAt, beforeRevision, beforeConfirmed, err := db.ReadCodexSessionBinding(inst.ID)
			if err != nil || beforeID != "" || beforeConfirmed || reloaded.CodexSessionID != "" || reloaded.CodexBindingConfirmed {
				t.Fatalf("fresh process captured an identity before emitting evidence: memory=%q/%v db=%q/%v err=%v",
					reloaded.CodexSessionID, reloaded.CodexBindingConfirmed, beforeID, beforeConfirmed, err)
			}

			if err := os.WriteFile(exitTrigger, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if shellPID := waitCodexReviewPIDFile(t, shellPIDPath); shellPID != panePID ||
				reloaded.readPanePID() != panePID || CodexProcessIdentity(panePID) != paneIdentity ||
				CodexProcessIdentity(freshPID) != "" {
				t.Fatalf("exit-to-shell must preserve the pane leader lifetime while the original Codex exits: pane=%d shell=%d fresh=%d",
					panePID, shellPID, freshPID)
			}
			if err := os.WriteFile(resumeTrigger, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			resumePID := waitCodexReviewPIDFile(t, resumePIDPath)
			status := ownedCodexHook(t, reloaded, &HookStatus{
				Status: "running", SessionID: oldID, Event: "thread/started", UpdatedAt: time.Now(),
			})
			if resumePID == freshPID || resumePID == panePID || status.CodexEmitterPID != resumePID ||
				!reloaded.codexHookEmitterOwns(status) ||
				!strings.HasSuffix(strings.Join(codexProcessArguments(resumePID), " "), "resume "+oldID) {
				t.Fatalf("replacement must be a distinct, pane-owned Codex resume of the old UUID: fresh=%d pane=%d replacement=%+v argv=%q",
					freshPID, panePID, status, codexProcessArguments(resumePID))
			}
			probe := reloaded.queryCodexSessionFromProcessFiles()
			if evidence == "process" {
				if probe.sessionID != oldID || probe.ambiguous {
					t.Fatalf("replacement's writable descriptor did not prove its old conversation: %+v", probe)
				}
				reloaded.updateCodexSession(nil, true)
			} else {
				if probe.sessionID != "" || probe.ambiguous {
					t.Fatalf("hook-only replacement unexpectedly has writable rollout evidence: %+v", probe)
				}
				writeCodexReviewHook(t, reloaded, status)
				reloaded.UpdateHookStatus(status)
			}
			gotID, gotAt, gotRevision, confirmed, err := db.ReadCodexSessionBinding(inst.ID)
			if err != nil || gotID != "" || confirmed || gotRevision != beforeRevision || !gotAt.Equal(beforeAt) ||
				reloaded.CodexSessionID != "" || reloaded.CodexBindingConfirmed ||
				reloaded.CodexBindingRevision != beforeRevision || !reloaded.CodexDetectedAt.Equal(beforeAt) ||
				ReadHookSessionAnchor(inst.ID) != "" {
				t.Errorf("later resume inherited the departed fresh Codex's capture authority: memory=%q/%v/%d db=%q/%v/%d err=%v",
					reloaded.CodexSessionID, reloaded.CodexBindingConfirmed, reloaded.CodexBindingRevision, gotID, confirmed, gotRevision, err)
			}
			if !paneAliveNow(reloaded.tmuxSession) || reloaded.readPanePID() != panePID ||
				CodexProcessIdentity(panePID) != paneIdentity || !reloaded.codexHookEmitterOwns(status) {
				t.Error("rejecting the replacement's identity killed or replaced its live pane")
			}
			if after, err := os.ReadFile(path); err != nil || string(after) != string(history) {
				t.Errorf("rejecting the replacement's identity changed native history: %v", err)
			}
		})
	}
}

func TestCodexEffectiveResumeWrappersRefuseBeforeLaunchMutation(t *testing.T) {
	for _, source := range []string{"instance", "tool_config"} {
		for _, form := range []string{"literal", "command_suffix", "command_substitution", "variable_expansion", "shell_control"} {
			for _, launch := range []string{"start", "restart_fresh"} {
				t.Run(source+"/"+form+"/"+launch, func(t *testing.T) {
					skipIfNoTmuxBinary(t)
					inst, home := newCodexGateInstance(t)
					db := withTempGlobalStateDB(t)
					storage := &Storage{db: db}
					id := uniqueSID(t)
					path := writeCodexReviewRolloutAt(t, home, id, inst.ProjectPath, time.Now().Add(-time.Hour))
					history, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					dir := t.TempDir()
					binary, marker := filepath.Join(dir, "codex"), filepath.Join(dir, "invoked")
					script := codexReviewScriptHeader + "printf '%s\\n' \"$$\" > " + shellQuote(marker) + "\nwhile :; do sleep 1; done\n"
					if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
						t.Fatal(err)
					}
					wrapper := shellQuote(binary) + " resume " + id
					switch form {
					case "command_suffix":
						wrapper = "{command} resume " + id
					case "command_substitution":
						wrapper = "{command} $(printf resume) " + id
					case "variable_expansion":
						wrapper = "{command} --model \"$HOME\""
					case "shell_control":
						wrapper = "{command}; " + shellQuote(binary) + " resume " + id
					}
					cfg := &UserConfig{}
					if source == "tool_config" {
						cfg.Tools = map[string]ToolDef{
							"review-codex": {Command: binary, CompatibleWith: "codex", Wrapper: wrapper},
						}
						inst.Tool = "review-codex"
					} else {
						inst.Wrapper = wrapper
					}
					t.Cleanup(resetUserConfigCache(t, cfg))
					inst.Command = shellQuote(binary)
					if err := storage.InsertSessionAndVerify(inst, nil); err != nil {
						t.Fatal(err)
					}
					var status *HookStatus
					var panePID int
					var paneIdentity string
					if launch == "restart_fresh" {
						if _, _, err := SetField(inst, FieldCodexSessionID, id, nil); err != nil {
							t.Fatal(err)
						}
						if err := storage.SaveWithGroups([]*Instance{inst}, nil); err != nil {
							t.Fatal(err)
						}
						status = startCodexReviewPane(t, inst, path)
						panePID, paneIdentity = inst.readPanePID(), CodexProcessIdentity(status.CodexEmitterPID)
					} else {
						t.Cleanup(func() { _ = inst.tmuxSession.Kill() })
					}
					beforeID, beforeAt, beforeRevision, beforeConfirmed, err := db.ReadCodexSessionBinding(inst.ID)
					if err != nil || (launch == "restart_fresh" && (beforeID != id || !beforeConfirmed)) {
						t.Fatalf("invalid pre-launch binding fixture: id=%q confirmed=%v err=%v", beforeID, beforeConfirmed, err)
					}
					memoryAt := inst.CodexDetectedAt
					if launch == "restart_fresh" {
						err = inst.RestartFresh()
					} else {
						err = inst.Start()
					}
					if err == nil || !strings.Contains(strings.ToLower(err.Error()), "codex") {
						t.Errorf("%s accepted an effective wrapper that changes the verified invocation: %v", launch, err)
					}
					gotID, gotAt, gotRevision, confirmed, readErr := db.ReadCodexSessionBinding(inst.ID)
					if readErr != nil || gotID != beforeID || confirmed != beforeConfirmed || gotRevision != beforeRevision ||
						!gotAt.Equal(beforeAt) || inst.CodexSessionID != beforeID || inst.CodexBindingConfirmed != beforeConfirmed ||
						inst.CodexBindingRevision != beforeRevision || !inst.CodexDetectedAt.Equal(memoryAt) ||
						ReadHookSessionAnchor(inst.ID) != beforeID {
						t.Errorf("%s changed binding before rejecting the effective wrapper: memory=%q/%v/%d db=%q/%v/%d err=%v",
							launch, inst.CodexSessionID, inst.CodexBindingConfirmed, inst.CodexBindingRevision, gotID, confirmed, gotRevision, readErr)
					}
					if launch == "restart_fresh" {
						if !paneAliveNow(inst.tmuxSession) || inst.readPanePID() != panePID ||
							CodexProcessIdentity(status.CodexEmitterPID) != paneIdentity || !inst.codexHookEmitterOwns(status) {
							t.Error("RestartFresh killed or replaced the confirmed pane before rejecting the wrapper")
						}
					} else if inst.readPanePID() != 0 || inst.tmuxSession.Exists() {
						t.Error("refused fresh Start created a pane")
					}
					if _, err := os.Stat(marker); !os.IsNotExist(err) {
						t.Errorf("refused launch executed the wrapper's Codex command: %v", err)
					}
					if after, err := os.ReadFile(path); err != nil || string(after) != string(history) {
						t.Errorf("refused launch changed native history: %v", err)
					}
				})
			}
		}
	}
}

func TestCodexFreshCaptureSupportsIdentityWrapperAndExitToShell(t *testing.T) {
	for _, wrapping := range []string{"instance_identity", "tool_identity", "literal_options", "exit_to_shell"} {
		t.Run(wrapping, func(t *testing.T) {
			skipIfNoTmuxBinary(t)
			inst, home := newCodexGateInstance(t)
			db := withTempGlobalStateDB(t)
			storage := &Storage{db: db}
			dir := t.TempDir()
			binary := filepath.Join(dir, "codex")
			argsPath, readyPath := filepath.Join(dir, "args"), filepath.Join(dir, "ready-pid")
			script := codexReviewScriptHeader +
				"printf '%s\\n' \"$@\" > " + shellQuote(argsPath) + "\n" +
				"printf '%s\\n' \"$$\" > " + shellQuote(readyPath) + "\n" +
				"while :; do sleep 1; done\n"
			if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			cfg := &UserConfig{}
			switch wrapping {
			case "instance_identity":
				inst.Wrapper = "{command}"
			case "tool_identity":
				cfg.Tools = map[string]ToolDef{
					"review-codex": {Command: binary, CompatibleWith: "codex", Wrapper: "{command}"},
				}
				inst.Tool = "review-codex"
			case "literal_options":
				inst.Wrapper = "{command} --model 'resume' --config 'note=\"literal;$value\"'"
			case "exit_to_shell":
				inst.ExitToShell = boolPtr(true)
			}
			t.Cleanup(resetUserConfigCache(t, cfg))
			inst.Command = shellQuote(binary)
			if err := storage.InsertSessionAndVerify(inst, nil); err != nil {
				t.Fatal(err)
			}
			if err := inst.Start(); err != nil {
				t.Fatalf("supported %s launch was refused: %v", wrapping, err)
			}
			t.Cleanup(func() { _ = inst.tmuxSession.Kill() })
			readyPID := waitCodexReviewPIDFile(t, readyPath)
			original := ownedCodexHook(t, inst, &HookStatus{})
			if original.CodexEmitterPID != readyPID {
				t.Fatalf("original Codex process differs from the fixture that received the launch argv: got=%d want=%d", original.CodexEmitterPID, readyPID)
			}
			if wrapping == "literal_options" {
				args, err := os.ReadFile(argsPath)
				if err != nil || !strings.HasSuffix(string(args), "--model\nresume\n--config\nnote=\"literal;$value\"\n") {
					t.Fatalf("literal wrapper arguments were expanded or misparsed: args=%q err=%v", args, err)
				}
			}
			if wrapping == "exit_to_shell" && original.CodexEmitterPID == inst.readPanePID() {
				t.Fatal("exit-to-shell fixture must retain a shell parent around the original Codex")
			}
			if err := storage.SaveWithGroups([]*Instance{inst}, nil); err != nil {
				t.Fatal(err)
			}
			instances, err := storage.Load()
			if err != nil || len(instances) != 1 {
				t.Fatalf("reload supported wrapped launch: instances=%d err=%v", len(instances), err)
			}
			reloaded := instances[0]
			launchRevision := reloaded.CodexBindingRevision
			id := uniqueSID(t)
			writeCodexReviewRolloutAt(t, home, id, inst.ProjectPath, time.Now())
			status := ownedCodexHook(t, reloaded, &HookStatus{
				Status: "running", SessionID: id, Event: "thread/started", UpdatedAt: time.Now(),
			})
			if status.CodexEmitterPID != original.CodexEmitterPID || status.CodexEmitterIdentity != original.CodexEmitterIdentity {
				t.Fatal("capture fixture replaced the original fresh Codex process")
			}
			writeCodexReviewHook(t, reloaded, status)
			reloaded.UpdateHookStatus(status)
			gotID, _, revision, confirmed, err := db.ReadCodexSessionBinding(inst.ID)
			if err != nil || gotID != id || !confirmed || revision <= launchRevision ||
				reloaded.CodexSessionID != id || !reloaded.CodexBindingConfirmed {
				t.Fatalf("supported %s launch did not capture its original fresh conversation: memory=%q/%v db=%q/%v/%d err=%v",
					wrapping, reloaded.CodexSessionID, reloaded.CodexBindingConfirmed, gotID, confirmed, revision, err)
			}
			if !paneAliveNow(reloaded.tmuxSession) || !reloaded.codexHookEmitterOwns(status) {
				t.Fatal("capturing the original fresh conversation replaced its live process")
			}
		})
	}
}

func TestCodexSlowLaunchShellFailsExplicitlyAndPreservesConfirmationRecovery(t *testing.T) {
	skipIfNoTmuxBinary(t)
	inst, home := newCodexGateInstance(t)
	db := withTempGlobalStateDB(t)
	storage := &Storage{db: db}
	oldID := uniqueSID(t)
	historyPath := writeCodexReviewRolloutAt(t, home, oldID, inst.ProjectPath, time.Now().Add(-time.Hour))
	history, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "codex")
	readyPath, shellPIDPath := filepath.Join(dir, "ready-pid"), filepath.Join(dir, "shell-pid")
	trigger := filepath.Join(dir, "finish-shell-startup")
	script := codexReviewScriptHeader +
		"printf '%s\\n' \"$$\" > " + shellQuote(readyPath) + "\n" +
		"while :; do sleep 1; done\n"
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	launchShell := filepath.Join(dir, "launch-shell")
	script = "#!/bin/sh\n" +
		"[ \"$#\" -eq 3 ] && [ \"$1\" = -il ] && [ \"$2\" = -c ] || exit 90\n" +
		"printf '%s\\n' \"$$\" > " + shellQuote(shellPIDPath) + "\n" +
		"while [ ! -f " + shellQuote(trigger) + " ]; do sleep 0.02; done\n" +
		"exec /bin/sh -c \"$3\"\n"
	if err := os.WriteFile(launchShell, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", launchShell)
	t.Cleanup(resetUserConfigCache(t, &UserConfig{}))
	inst.Command = shellQuote(binary)
	inst.LaunchShell = boolPtr(true)
	if err := storage.InsertSessionAndVerify(inst, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inst.tmuxSession.Kill() })
	result := make(chan error, 1)
	go func() { result <- inst.Start() }()
	select {
	case err = <-result:
	case <-time.After(8 * time.Second):
		t.Fatal("Start did not report unavailable native ownership within the bounded launch window")
	}
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "codex") ||
		!strings.Contains(err.Error(), "codex-session-id") {
		t.Errorf("slow shell startup must visibly refuse unregistered ownership with exact-ID recovery guidance: %v", err)
	}
	waitCodexReviewPIDFile(t, shellPIDPath)
	panePID := inst.readPanePID()
	paneIdentity := CodexProcessIdentity(panePID)
	if panePID <= 0 || paneIdentity == "" || !paneAliveNow(inst.tmuxSession) {
		t.Fatal("unregistered native launch did not preserve its live startup shell")
	}
	if _, err := os.Stat(readyPath); !os.IsNotExist(err) {
		t.Fatalf("Codex ran before the launch-shell startup trigger was released: %v", err)
	}
	if err := storage.SaveWithGroups([]*Instance{inst}, nil); err != nil {
		t.Fatal(err)
	}
	instances, err := storage.Load()
	if err != nil || len(instances) != 1 {
		t.Fatalf("reload unregistered launch: instances=%d err=%v", len(instances), err)
	}
	reloaded := instances[0]
	beforeID, beforeAt, beforeRevision, beforeConfirmed, err := db.ReadCodexSessionBinding(inst.ID)
	if err != nil || beforeID != "" || beforeConfirmed || reloaded.CodexSessionID != "" || reloaded.CodexBindingConfirmed {
		t.Fatalf("slow launch fabricated a native binding: memory=%q/%v db=%q/%v err=%v",
			reloaded.CodexSessionID, reloaded.CodexBindingConfirmed, beforeID, beforeConfirmed, err)
	}

	if err := os.WriteFile(trigger, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	readyPID := waitCodexReviewPIDFile(t, readyPath)
	id := uniqueSID(t)
	writeCodexReviewRolloutAt(t, home, id, inst.ProjectPath, time.Now())
	status := ownedCodexHook(t, reloaded, &HookStatus{
		Status: "waiting", SessionID: id, Event: "turn/completed", UpdatedAt: time.Now(),
	})
	if status.CodexEmitterPID != readyPID || !reloaded.codexHookEmitterOwns(status) {
		t.Fatal("released startup shell did not launch the pane's verifiable Codex process")
	}
	writeCodexReviewHook(t, reloaded, status)
	reloaded.UpdateHookStatus(status)
	gotID, gotAt, revision, confirmed, err := db.ReadCodexSessionBinding(inst.ID)
	if err != nil || gotID != "" || confirmed || revision != beforeRevision || !gotAt.Equal(beforeAt) ||
		reloaded.CodexSessionID != "" || reloaded.CodexBindingConfirmed || ReadHookSessionAnchor(inst.ID) != "" {
		t.Errorf("native process appearing after failed registration acquired fresh capture permission: memory=%q/%v db=%q/%v/%d err=%v",
			reloaded.CodexSessionID, reloaded.CodexBindingConfirmed, gotID, confirmed, revision, err)
	}

	_, postCommit, err := SetField(reloaded, FieldCodexSessionID, id, nil)
	if err != nil {
		t.Fatalf("explicit confirmation rejected the verified native conversation: %v", err)
	}
	if err := storage.SaveWithGroups([]*Instance{reloaded}, nil); err != nil {
		t.Fatal(err)
	}
	if postCommit != nil {
		if err := postCommit(); err != nil {
			t.Fatalf("publish explicitly confirmed native binding: %v", err)
		}
	}
	reloaded.UpdateHookStatus(status)
	gotID, _, revision, confirmed, err = db.ReadCodexSessionBinding(inst.ID)
	if err != nil || gotID != id || !confirmed || revision <= beforeRevision ||
		reloaded.CodexSessionID != id || !reloaded.CodexBindingConfirmed || ReadHookSessionAnchor(inst.ID) != id ||
		reloaded.hookStatus != "waiting" {
		t.Fatalf("explicit native confirmation did not recover durable ownership and hook updates: memory=%q/%v/%q db=%q/%v/%d err=%v",
			reloaded.CodexSessionID, reloaded.CodexBindingConfirmed, reloaded.hookStatus, gotID, confirmed, revision, err)
	}
	if !paneAliveNow(reloaded.tmuxSession) || reloaded.readPanePID() != panePID ||
		CodexProcessIdentity(panePID) != paneIdentity || !reloaded.codexHookEmitterOwns(status) {
		t.Error("failed registration or explicit confirmation replaced the original live pane")
	}
	if after, err := os.ReadFile(historyPath); err != nil || string(after) != string(history) {
		t.Errorf("failed registration or explicit confirmation changed unrelated native history: %v", err)
	}
}
