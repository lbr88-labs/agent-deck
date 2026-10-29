package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/statedb"
	"github.com/asheshgoplani/agent-deck/internal/tmux"
)

func ownedCodexHook(t *testing.T, inst *Instance, status *HookStatus) *HookStatus {
	t.Helper()
	skipIfNoTmuxBinary(t)
	if !inst.tmuxSession.Exists() {
		if inst.CodexSessionID == "" {
			if db := inst.metadataStateDB(); db != nil {
				id, at, revision, _, err := db.ReadCodexSessionBinding(inst.ID)
				if err != nil {
					t.Fatal(err)
				}
				if id == "" {
					inst.CodexDetectedAt, inst.CodexBindingRevision = at, revision
				}
			}
		}
		binary := filepath.Join(t.TempDir(), "codex")
		if err := os.WriteFile(binary, []byte("#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then exit 0; fi\nwhile :; do sleep 1; done\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		// Run the emitter directly, not through a first-run login-shell wizard.
		inst.tmuxSession.RunCommandAsInitialProcess = true
		if err := inst.tmuxSession.Start("exec " + shellQuote(binary)); err != nil {
			t.Fatalf("start hook-emitter pane: %v", err)
		}
		if err := inst.recordCodexFreshLaunch(inst.prepareCodexFreshLaunch()); err != nil {
			t.Fatalf("register fresh fixture process: %v", err)
		}
		t.Cleanup(func() { _ = inst.tmuxSession.Kill() })
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, pid := range inst.collectTmuxPaneProcessTreePIDs() {
			if isLikelyCodexProcessPID(pid) {
				status.CodexEmitterPID = pid
				status.CodexEmitterIdentity = CodexProcessIdentity(pid)
				if status.CodexEmitterIdentity != "" {
					return status
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	var diagnostics strings.Builder
	for _, pid := range inst.collectTmuxPaneProcessTreePIDs() {
		fmt.Fprintf(&diagnostics, "pid=%d argv=%q lifetime=%q\n", pid, codexProcessArguments(pid), CodexProcessIdentity(pid))
	}
	panes, panesErr := tmux.OutputBounded(inst.tmuxSession.SocketName, "list-panes", "-t", inst.tmuxSession.Name+":", "-F", "#{pane_pid} #{pane_current_command} #{pane_dead}")
	screen, screenErr := tmux.OutputBounded(inst.tmuxSession.SocketName, "capture-pane", "-p", "-t", inst.tmuxSession.Name+":")
	t.Fatalf("hook-emitter pane has no verifiable interactive Codex process: instance_socket=%q session_socket=%q panes=%q (%v) screen=%q (%v)\n%s", inst.TmuxSocketName, inst.tmuxSession.SocketName, panes, panesErr, screen, screenErr, diagnostics.String())
	return status
}

func TestCodexHookRequiresPaneOwnershipForNewIdentity(t *testing.T) {
	for _, bound := range []bool{false, true} {
		for _, emitter := range []string{"legacy", "foreign_process", "reused_pid"} {
			t.Run(emitter+map[bool]string{false: "/unbound", true: "/bound"}[bound], func(t *testing.T) {
				inst, codexHome := newCodexGateInstance(t)
				originalID, foreignID := uniqueSID(t), uniqueSID(t)
				writeCodexRolloutWithSource(t, codexHome, originalID, inst.ProjectPath, "cli")
				writeCodexRolloutWithSource(t, codexHome, foreignID, inst.ProjectPath, "cli")
				if bound {
					inst.CodexSessionID = originalID
				}
				wantID := inst.CodexSessionID
				status := &HookStatus{Status: "waiting", SessionID: foreignID, Event: "turn/completed", UpdatedAt: time.Now()}
				switch emitter {
				case "foreign_process":
					status.CodexEmitterPID = os.Getpid()
					status.CodexEmitterIdentity = CodexProcessIdentity(os.Getpid())
				case "reused_pid":
					status = ownedCodexHook(t, inst, status)
					status.CodexEmitterIdentity += "different-lifetime"
				}
				inst.hookStatus, inst.hookEvent = "running", "turn/started"
				inst.UpdateHookStatus(status)
				if inst.CodexSessionID != wantID || inst.hookStatus != "running" || inst.hookEvent != "turn/started" {
					t.Fatalf("unproven hook changed ownership/status: id=%q status=%q event=%q", inst.CodexSessionID, inst.hookStatus, inst.hookEvent)
				}
			})
		}
	}
}

func TestCodexPaneHookCanSwitchNativeConversation(t *testing.T) {
	inst, codexHome := newCodexGateInstance(t)
	db := withTempGlobalStateDB(t)
	originalID := uniqueSID(t)
	writeCodexRolloutWithSource(t, codexHome, originalID, inst.ProjectPath, "cli")
	inst.CodexSessionID = originalID
	inst.CodexBindingConfirmed = true
	seedPersistedCodexBinding(t, db, inst, originalID)
	// Native switches need both a pane-owned emitter and positive top-level
	// metadata. An unflushed hook must defer, not replace the recorded binding.
	newID := uniqueSID(t)
	status := ownedCodexHook(t, inst, &HookStatus{
		Status: "running", SessionID: newID, Event: "thread/started", UpdatedAt: time.Now(),
	})
	inst.UpdateHookStatus(status)
	if inst.CodexSessionID != originalID {
		t.Fatalf("unverifiable hook changed original binding to %q", inst.CodexSessionID)
	}
	writeCodexRolloutWithSource(t, codexHome, newID, inst.ProjectPath, "cli")
	inst.UpdateHookStatus(status)
	if inst.CodexSessionID != newID || readCodexSessionIDFromDB(t, db, inst.ID) != newID || !inst.CodexBindingConfirmed {
		t.Fatalf("pane-proven native switch was not persisted and confirmed: memory=%q db=%q confirmed=%v", inst.CodexSessionID, readCodexSessionIDFromDB(t, db, inst.ID), inst.CodexBindingConfirmed)
	}
}

func TestCodexCachedEnvironmentCannotIntroduceAnUnownedThread(t *testing.T) {
	for _, bound := range []bool{false, true} {
		t.Run(map[bool]string{false: "unbound", true: "bound"}[bound], func(t *testing.T) {
			inst, codexHome := newCodexGateInstance(t)
			db := withTempGlobalStateDB(t)
			originalID, foreignID := uniqueSID(t), uniqueSID(t)
			writeCodexRolloutWithSource(t, codexHome, originalID, inst.ProjectPath, "cli")
			writeCodexRolloutWithSource(t, codexHome, foreignID, inst.ProjectPath, "cli")
			if bound {
				inst.CodexSessionID = originalID
				inst.CodexBindingConfirmed = true
			}
			wantID := inst.CodexSessionID
			seedPersistedCodexBinding(t, db, inst, wantID)
			ownedCodexHook(t, inst, &HookStatus{})
			if err := inst.tmuxSession.SetEnvironment("CODEX_SESSION_ID", foreignID); err != nil {
				t.Fatal(err)
			}
			inst.SyncSessionIDsFromTmux()
			if id, err := inst.tmuxSession.GetEnvironment("CODEX_SESSION_ID"); err != nil || id != wantID {
				t.Fatalf("rejected tmux cache was not healed: id=%q want=%q err=%v", id, wantID, err)
			}
			inst.UpdateCodexSession(nil)
			if inst.CodexSessionID != wantID || readCodexSessionIDFromDB(t, db, inst.ID) != wantID {
				t.Fatalf("tmux cache introduced a foreign thread: memory=%q db=%q", inst.CodexSessionID, readCodexSessionIDFromDB(t, db, inst.ID))
			}
		})
	}
}

func TestCodexStartAndRestartKeepExactBinding(t *testing.T) {
	skipIfNoTmuxBinary(t)
	inst, codexHome := newCodexGateInstance(t)
	db := withTempGlobalStateDB(t)
	originalID, foreignID := uniqueSID(t), uniqueSID(t)
	writeCodexRolloutWithSource(t, codexHome, originalID, inst.ProjectPath, "cli")
	foreignPath := writeCodexRolloutWithSource(t, codexHome, foreignID, inst.ProjectPath, "cli")
	updated := time.Now().Add(time.Hour)
	if err := os.Chtimes(foreignPath, updated, updated); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "codex-args")
	binary := filepath.Join(t.TempDir(), "codex")
	script := "#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then exit 0; fi\nprintf '%s\\n' \"$*\" >> " + shellQuote(logPath) + "\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	restore := resetUserConfigCache(t, &UserConfig{Codex: CodexSettings{Command: binary}})
	defer restore()
	inst.Command, inst.CodexSessionID = "codex", originalID
	seedPersistedCodexBinding(t, db, inst, originalID)
	detectedAt := time.Now().Add(-time.Hour)
	revision, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, originalID, detectedAt, originalID != "")
	if err != nil || !matched {
		t.Fatalf("seed binding revision: matched=%v err=%v", matched, err)
	}
	inst.CodexDetectedAt, inst.CodexBindingRevision = detectedAt, revision
	if err := inst.Start(); err != nil {
		t.Fatalf("start bound Codex: %v", err)
	}
	t.Cleanup(func() { _ = inst.tmuxSession.Kill() })
	// Exercise the startup worker synchronously, without depending on whether
	// the caller chose to schedule an async goroutine for a bound resume.
	inst.detectCodexSessionAsync()
	if inst.CodexSessionID != originalID || readCodexSessionIDFromDB(t, db, inst.ID) != originalID {
		t.Fatal("start discovery hijacked the newer unregistered same-cwd chat")
	}
	if err := inst.Restart(); err != nil {
		t.Fatalf("restart bound Codex: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	var args string
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(logPath)
		args = string(data)
		if strings.Count(args, "\n") >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if strings.Count(args, "resume "+originalID) != 2 || strings.Contains(args, foreignID) {
		t.Fatalf("start/restart did not execute the exact original resume twice: %q", args)
	}
	gotID, _, gotRevision, _, err := db.ReadCodexSessionBinding(inst.ID)
	if err != nil || gotID != originalID || gotRevision != revision {
		t.Fatalf("resume changed persisted binding: id=%q revision=%d want=%d err=%v", gotID, gotRevision, revision, err)
	}
}

func TestCodexExplicitBindingRejectsAnotherRowsConversation(t *testing.T) {
	inst, codexHome := newCodexGateInstance(t)
	db := withTempGlobalStateDB(t)
	foreignID := uniqueSID(t)
	writeCodexRolloutWithSource(t, codexHome, foreignID, inst.ProjectPath, "cli")
	owner := NewInstanceWithTool("owner", inst.ProjectPath, "codex")
	seedPersistedCodexBinding(t, db, owner, foreignID)
	seedPersistedCodexBinding(t, db, inst, "")
	if _, _, err := SetField(inst, FieldCodexSessionID, foreignID, nil); err == nil {
		t.Fatal("explicit binding silently duplicated another row's conversation")
	}
	if inst.CodexSessionID != "" || readCodexSessionIDFromDB(t, db, inst.ID) != "" {
		t.Fatal("refused explicit binding mutated the recipient")
	}
}

func TestCodexProcessIdentityExcludesSharedDaemonsAndExecHelpers(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"/opt/codex", "resume", "11111111-2222-4333-8444-555555555555"}, true},
		{[]string{"/opt/codex", "fork", "11111111-2222-4333-8444-555555555555"}, true},
		{[]string{"/opt/codex", "--model", "gpt-5", "hello"}, true},
		{[]string{"/bin/sh", "/tmp/codex"}, true},
		{[]string{"/opt/codex", "app-server"}, false},
		{[]string{"/opt/codex", "exec", "check this"}, false},
		{[]string{"/opt/codex", "--model", "gpt-5", "exec", "check this"}, false},
		{[]string{"/opt/codex", "--enable", "hooks", "app-server"}, false},
		{[]string{"/opt/codex", "--model=gpt-5", "exec", "check this"}, false},
		{[]string{"/opt/codex", "--unrecognized-option", "value", "app-server"}, false},
		{[]string{"/opt/codex", "--disable=hooks", "--full-auto", "resume", "11111111-2222-4333-8444-555555555555"}, true},
		{[]string{"/bin/sh", "-c", "codex resume some-other-chat"}, false},
		{[]string{"/usr/bin/python", "read-codex-history.py"}, false},
	} {
		if got := isInteractiveCodexArgs(tc.args); got != tc.want {
			t.Errorf("isInteractiveCodexArgs(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestCodexArchivedAndHubOnlyBindingPassesLaunchPreflight(t *testing.T) {
	for _, source := range []string{"archive", "json_index", "state_db", "archived_state_db"} {
		t.Run(source, func(t *testing.T) {
			inst, home := newCodexGateInstance(t)
			id := uniqueSID(t)
			switch source {
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
				if err := AppendCodexSessionIndexName(home, id, "original", time.Now()); err != nil {
					t.Fatal(err)
				}
			case "state_db", "archived_state_db":
				writeCodexStateThread(t, home, id, "original", "prompt", inst.ProjectPath, time.Now().UnixMilli())
				if source == "archived_state_db" {
					nativeDB, err := sql.Open("sqlite", filepath.Join(home, "state_5.sqlite"))
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = nativeDB.Close() })
					if _, err := nativeDB.Exec("UPDATE threads SET archived = 1 WHERE id = ?", id); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, _, err := SetField(inst, FieldCodexSessionID, id, nil); err != nil {
				t.Fatalf("confirm %s-only native conversation: %v", source, err)
			}
			if err := inst.prepareCodexIdentity(true); err != nil {
				t.Fatalf("%s-only exact binding cannot restart: %v", source, err)
			}
			if inst.CodexSessionID != id {
				t.Fatalf("%s-only launch preflight replaced binding: %q", source, inst.CodexSessionID)
			}
		})
	}
}

func TestCodexReloadedNeverStartedRowCanStartFresh(t *testing.T) {
	inst, _ := newCodexGateInstance(t)
	db := withTempGlobalStateDB(t)
	seedPersistedCodexBinding(t, db, inst, "")
	inst.addedThisProcess = false
	if err := inst.prepareCodexIdentity(false); err != nil {
		t.Fatalf("explicitly created, never-started row cannot make its first launch: %v", err)
	}
	if err := inst.prepareCodexIdentity(true); err == nil {
		t.Fatal("ordinary restart of an unknown identity silently authorized a fresh conversation")
	}
	if !inst.CanRestartFresh() {
		t.Fatal("unknown Codex identity has no explicit fresh-start recovery")
	}
	inst.clearSessionBindingForFreshStart()
	if err := inst.prepareCodexIdentity(false); err != nil {
		t.Fatalf("explicit fresh-start opt-in was not honored: %v", err)
	}
}

func TestCodexHookAnchorCannotIntroduceConversationIdentity(t *testing.T) {
	inst, home := newCodexGateInstance(t)
	foreignID := uniqueSID(t)
	writeCodexRolloutWithSource(t, home, foreignID, inst.ProjectPath, "cli")
	WriteHookSessionAnchor(inst.ID, foreignID)
	inst.UpdateHookStatus(&HookStatus{Status: "running", Event: "turn/started", UpdatedAt: time.Now()})
	if inst.CodexSessionID != "" {
		t.Fatalf("anchor-only hook stole unrelated conversation: %q", inst.CodexSessionID)
	}
}

func TestCodexUnverifiableHookPreservesEvidence(t *testing.T) {
	inst, home := newCodexGateInstance(t)
	id := uniqueSID(t)
	writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
	directory := GetHooksDir()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, inst.ID+".json")
	payload := []byte(`{"status":"running","session_id":"` + id + `","event":"thread/started","ts":1}`)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	inst.UpdateHookStatus(&HookStatus{Status: "running", SessionID: id, Event: "thread/started", UpdatedAt: time.Now()})
	if inst.CodexSessionID != "" {
		t.Fatalf("unverifiable hook changed binding: %q", inst.CodexSessionID)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("unverifiable ownership destroyed hook evidence: %v", err)
	}
}

func TestCodexUnverifiableRolloutCannotAuthorizeResume(t *testing.T) {
	for _, contents := range []string{"", `{"type":"session_meta","payload":{"id":"11111111-2222-4333-8444-555555555555","source":"cli"}}`} {
		t.Run(map[bool]string{false: "mismatched_identity", true: "empty"}[contents == ""], func(t *testing.T) {
			inst, home := newCodexGateInstance(t)
			id := uniqueSID(t)
			path := writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			inst.CodexSessionID = id
			inst.CodexBindingConfirmed = true
			if err := inst.prepareCodexIdentity(true); err == nil {
				t.Fatal("unverifiable rollout silently authorized a resume")
			}
			if inst.CodexSessionID != id {
				t.Fatalf("unverifiable rollout cleared recorded binding: %q", inst.CodexSessionID)
			}
		})
	}
}

func TestCodexProcessFilesRequireWritableConversationDescriptor(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux /proc descriptor access modes")
	}
	inst, home := newCodexGateInstance(t)
	id := uniqueSID(t)
	path := writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
	reader, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, candidate := range extractCodexRolloutCandidatesFromProcFD(os.Getpid()) {
		if candidate.ID == id {
			t.Fatal("a read-only history descriptor became live conversation ownership")
		}
	}
	writer, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if got := selectCodexRolloutCandidate(extractCodexRolloutCandidatesFromProcFD(os.Getpid())); got.sessionID != id || got.ambiguous {
		t.Fatalf("unique writable conversation descriptor was not recognized: %+v", got)
	}
}

func TestCodexLsofOwnershipIgnoresReadOnlyHistory(t *testing.T) {
	home := t.TempDir()
	id, foreignID := uniqueSID(t), uniqueSID(t)
	path := writeCodexRolloutWithSource(t, home, id, t.TempDir(), "cli")
	foreignPath := writeCodexRolloutWithSource(t, home, foreignID, t.TempDir(), "cli")
	output := []byte("codex 123 user 10r REG 1,2 100 3 " + foreignPath + "\n" +
		"codex 123 user 11w REG 1,2 100 4 " + path + "\n")
	if got := selectCodexRolloutCandidate(extractWritableCodexRolloutsFromLsof(output)); got.sessionID != id || got.ambiguous {
		t.Fatalf("read-only lsof history competed with owned writer: %+v", got)
	}
}

func TestCodexRestartFreshLaunchesAnExplicitNewConversation(t *testing.T) {
	skipIfNoTmuxBinary(t)
	inst, home := newCodexGateInstance(t)
	db := withTempGlobalStateDB(t)
	originalID, foreignID := uniqueSID(t), uniqueSID(t)
	writeCodexRolloutWithSource(t, home, originalID, inst.ProjectPath, "cli")
	writeCodexRolloutWithSource(t, home, foreignID, inst.ProjectPath, "cli")
	inst.CodexSessionID = originalID
	inst.LastStartedAt = time.Now().Add(-time.Hour)
	seedPersistedCodexBinding(t, db, inst, originalID)
	logPath := filepath.Join(t.TempDir(), "fresh-args")
	binary := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then exit 0; fi\nprintf '%s\\n' \"$*\" > "+shellQuote(logPath)+"\nwhile :; do sleep 1; done\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	restore := resetUserConfigCache(t, &UserConfig{Codex: CodexSettings{Command: binary}})
	defer restore()
	inst.Command = "codex"
	inst.IsForkAwaitingStart = true
	inst.ForkStartCommand = "codex fork " + originalID
	if err := inst.RestartFresh(); err != nil {
		t.Fatalf("explicit fresh restart failed: %v", err)
	}
	t.Cleanup(func() { _ = inst.tmuxSession.Kill() })
	deadline := time.Now().Add(2 * time.Second)
	var args []byte
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(logPath)
		if err == nil {
			args = data
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if string(args) != "\n" {
		t.Fatalf("fresh restart reused a resume/fork conversation: args=%q", args)
	}
	if inst.CodexSessionID != "" || readCodexSessionIDFromDB(t, db, inst.ID) != "" {
		t.Fatalf("fresh launch stole existing history: memory=%q db=%q", inst.CodexSessionID, readCodexSessionIDFromDB(t, db, inst.ID))
	}
}

func TestCodexOwnershipRegistryFailureDoesNotAllowRebind(t *testing.T) {
	inst, home := newCodexGateInstance(t)
	db := withTempGlobalStateDB(t)
	originalID, otherID := uniqueSID(t), uniqueSID(t)
	writeCodexRolloutWithSource(t, home, originalID, inst.ProjectPath, "cli")
	writeCodexRolloutWithSource(t, home, otherID, inst.ProjectPath, "cli")
	inst.CodexSessionID = originalID
	seedPersistedCodexBinding(t, db, inst, originalID)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if changed := inst.acceptCodexSessionID(otherID, false, codexBindingProcess); changed || inst.CodexSessionID != originalID {
		t.Fatalf("failed registry lookup authorized identity change: changed=%v id=%q", changed, inst.CodexSessionID)
	}
	if err := inst.prepareCodexIdentity(true); err == nil {
		t.Fatal("unverifiable registry ownership silently authorized restart")
	}
}

func TestCodexReconciliationDoesNotClaimAnotherRowsConversation(t *testing.T) {
	for _, source := range []string{"sidecar", "persisted"} {
		t.Run(source, func(t *testing.T) {
			_, cwd, db, inst, rootID, guardianID, targetID := setupCodexPromotionTest(t)
			owner := NewInstanceWithTool("target-owner", cwd, "codex")
			seedPersistedCodexBinding(t, db, owner, targetID)
			inst.CodexSessionID = guardianID
			if source == "sidecar" {
				now := time.Now()
				if err := writeCodexSubagentMigrationState(inst.ID, codexSubagentMigrationState{
					SourceID: guardianID, OldRoot: rootID, TargetID: targetID,
					Started: now.Add(-time.Second), Completed: now,
				}); err != nil {
					t.Fatal(err)
				}
				if inst.adoptCompletedCodexMigrationTarget(false) {
					t.Fatal("completed sidecar adopted another row's conversation")
				}
			} else {
				if _, matched, err := db.WriteCodexSessionBindingOverride(inst.ID, targetID, time.Now(), targetID != ""); err != nil || !matched {
					t.Fatalf("seed conflicting persisted binding: matched=%v err=%v", matched, err)
				}
				if inst.adoptPersistedCodexPromotion(false) {
					t.Fatal("persisted reconciliation adopted another row's conversation")
				}
			}
			if inst.CodexSessionID != guardianID {
				t.Fatalf("%s reconciliation changed identity to %q", source, inst.CodexSessionID)
			}
		})
	}
}

func TestCodexBoundLaunchCannotFallThroughToOpaqueFreshCommand(t *testing.T) {
	for _, command := range []string{"custom-agent-wrapper --flag", "codex --model gpt-5", "codex resume 11111111-2222-4333-8444-555555555555"} {
		t.Run(command, func(t *testing.T) {
			inst, home := newCodexGateInstance(t)
			id := uniqueSID(t)
			writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
			inst.Command, inst.CodexSessionID = command, id
			inst.CodexBindingConfirmed = true
			if err := inst.prepareCodexIdentity(true); err == nil {
				t.Fatalf("bound launch authorized unverifiable passthrough: %q", command)
			}
			if inst.CodexSessionID != id {
				t.Fatalf("unsupported launch discarded exact binding: %q", inst.CodexSessionID)
			}
		})
	}
}

func TestCodexExactExplicitResumeCommandKeepsItsBinding(t *testing.T) {
	inst, home := newCodexGateInstance(t)
	home = filepath.Join(home, "Codex Archive")
	id := uniqueSID(t)
	writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
	inst.CodexSessionID = id
	inst.Command = "CODEX_HOME=" + shellQuote(home) + " codex resume " + id
	if _, _, err := SetField(inst, FieldCodexSessionID, id, nil); err != nil {
		t.Fatalf("confirm exact resume identity: %v", err)
	}
	if err := inst.prepareCodexIdentity(true); err != nil {
		t.Fatalf("explicit exact resume command was refused: %v", err)
	}
	if command := inst.buildCodexCommand(inst.Command); strings.Count(command, "resume "+id) != 1 || !strings.Contains(command, "CODEX_HOME="+shellQuote(home)) {
		t.Fatalf("explicit exact resume command was changed: %q", command)
	}
}

func TestCodexDeduplicatedLaunchCannotHideAnUnverifiedIdentity(t *testing.T) {
	for _, identity := range []string{"missing", "unverifiable"} {
		for _, launch := range []string{"start", "start_with_message", "restart", "restart_with_empty_env"} {
			t.Run(identity+"/"+launch, func(t *testing.T) {
				inst, _ := newCodexGateInstance(t)
				inst.LastStartedAt = time.Now().Add(-time.Hour)
				if identity == "unverifiable" {
					inst.CodexSessionID = uniqueSID(t)
				}
				originalID := inst.CodexSessionID
				recordInstanceSpawn(inst.ID)
				stamp, err := instanceSpawnStampPath(inst.ID)
				if err != nil {
					t.Fatal(err)
				}
				// Exercise the sibling-won-the-spawn-lock branch without racing a
				// goroutine or replacing the process-wide clock test seam.
				afterLock := time.Now().Add(time.Hour)
				if err := os.Chtimes(stamp, afterLock, afterLock); err != nil {
					t.Fatal(err)
				}
				switch launch {
				case "start":
					err = inst.Start()
				case "start_with_message":
					err = inst.StartWithMessage("")
				case "restart":
					err = inst.Restart()
				case "restart_with_empty_env":
					err = inst.RestartWithEnv(nil)
				}
				if err == nil || !strings.Contains(err.Error(), "restart-fresh") {
					t.Fatalf("deduplicated %s silently accepted an %s identity: %v", launch, identity, err)
				}
				if inst.CodexSessionID != originalID {
					t.Fatalf("deduplicated %s changed the recorded identity: %q", launch, inst.CodexSessionID)
				}
			})
		}
	}
}

func TestCodexConfiguredCommandCannotOverrideRecordedBinding(t *testing.T) {
	for _, commandKind := range []string{"foreign_resume", "opaque_flags"} {
		t.Run(commandKind, func(t *testing.T) {
			inst, home := newCodexGateInstance(t)
			originalID, foreignID := uniqueSID(t), uniqueSID(t)
			writeCodexRolloutWithSource(t, home, originalID, inst.ProjectPath, "cli")
			inst.CodexSessionID = originalID
			inst.CodexBindingConfirmed = true
			inst.Command = "codex"
			config := cloneDefaultUserConfig()
			config.Codex.Command = "codex resume " + foreignID
			if commandKind == "opaque_flags" {
				config.Codex.Command = "codex --model example"
			}
			t.Cleanup(resetUserConfigCache(t, &config))
			if err := inst.prepareCodexIdentity(true); err == nil {
				t.Fatalf("configured command %q bypassed the exact binding", config.Codex.Command)
			}
			if inst.CodexSessionID != originalID {
				t.Fatal("refusing a configured command lost the recorded binding")
			}
		})
	}
}

func TestCodexExplicitResumeRequiresAnExplicitBinding(t *testing.T) {
	inst, home := newCodexGateInstance(t)
	id := uniqueSID(t)
	writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
	inst.Command = "codex resume " + id
	if err := inst.prepareCodexIdentity(false); err == nil {
		t.Fatal("an unbound instance resumed a conversation without an explicit import or binding")
	}
	if inst.CodexSessionID != "" {
		t.Fatal("a command string was mistaken for an ownership grant")
	}
}

func TestCodexFreshStartRemovesAnExactResumeCommand(t *testing.T) {
	for _, commandSource := range []string{"instance", "configured"} {
		t.Run(commandSource, func(t *testing.T) {
			inst, home := newCodexGateInstance(t)
			originalID, nextID := uniqueSID(t), uniqueSID(t)
			inst.CodexSessionID = originalID
			inst.Command = "CODEX_HOME=" + shellQuote(home) + " codex resume " + originalID
			if commandSource == "configured" {
				config := cloneDefaultUserConfig()
				config.Codex.Command = "codex resume " + originalID
				t.Cleanup(resetUserConfigCache(t, &config))
				inst.Command = "codex"
			}
			inst.clearSessionBindingForFreshStart()
			command := inst.buildCodexCommand(inst.resolveCodexCommand(inst.Command))
			if strings.Contains(command, "resume ") || strings.Contains(command, originalID) {
				t.Fatalf("explicit fresh start retained the old resume command: %q", command)
			}
			inst.codexFreshStart = false
			inst.CodexSessionID = nextID
			inst.CodexBindingConfirmed = true // Model the proven capture after the fresh launch.
			writeCodexRolloutWithSource(t, home, nextID, inst.ProjectPath, "cli")
			if err := inst.prepareCodexIdentity(true); err != nil {
				t.Fatalf("the new conversation could not be resumed after a fresh start: %v", err)
			}
			command = inst.buildCodexCommand(inst.resolveCodexCommand(inst.Command))
			if !strings.Contains(command, "resume "+nextID) || strings.Contains(command, originalID) {
				t.Fatalf("the old configured resume command returned after the fresh start: %q", command)
			}
		})
	}
}

func TestCodexFreshStartRefusesOpaqueResumeBeforeClearingBinding(t *testing.T) {
	inst, home := newCodexGateInstance(t)
	id := uniqueSID(t)
	writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
	inst.CodexSessionID = id
	inst.Command = "codex resume --last"
	WriteHookSessionAnchor(inst.ID, id)
	if err := inst.RestartFresh(); err == nil {
		t.Fatal("an opaque resume command was allowed to masquerade as a fresh conversation")
	}
	if inst.CodexSessionID != id || ReadHookSessionAnchor(inst.ID) != id {
		t.Fatal("refusing an opaque fresh command destroyed the existing binding")
	}
}

func TestCodexSessionIDThreadSafeDuringBindingUpdates(t *testing.T) {
	ids := []string{
		"019f1111-1111-4111-8111-111111111111",
		"019f2222-2222-4222-8222-222222222222",
	}
	inst := &Instance{CodexSessionID: ids[0]}
	started := make(chan struct{})
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		close(started)
		for n := 0; ; n++ {
			select {
			case <-stop:
				return
			default:
			}
			// Status polling and pane-owned hook updates write the binding
			// while holding this same instance mutex.
			inst.mu.Lock()
			inst.CodexSessionID = ids[n%len(ids)]
			inst.mu.Unlock()
		}
	}()
	defer func() {
		close(stop)
		<-done
	}()
	<-started

	for range 1000 {
		if got := inst.GetCodexSessionIDThreadSafe(); got != ids[0] && got != ids[1] {
			t.Fatalf("read an invalid conversation binding during an update: %q", got)
		}
	}
}

func TestCodexFreshOwnedHookRequiresVerifiedNativeEvidence(t *testing.T) {
	for _, evidence := range []string{"verified", "missing_metadata", "mismatched_metadata", "no_fresh_launch"} {
		t.Run(evidence, func(t *testing.T) {
			inst, home := newCodexGateInstance(t)
			db := withTempGlobalStateDB(t)
			seedPersistedCodexBinding(t, db, inst, "")
			id := uniqueSID(t)
			switch evidence {
			case "verified", "no_fresh_launch":
				writeCodexRolloutWithSource(t, home, id, inst.ProjectPath, "cli")
			case "mismatched_metadata":
				writeCodexRolloutBody(t, home, id, `{"type":"session_meta","payload":{"id":"11111111-2222-4333-8444-555555555555","source":"cli"}}`+"\n")
			}
			status := ownedCodexHook(t, inst, &HookStatus{
				Status: "running", SessionID: id, Event: "thread/started", UpdatedAt: time.Now(),
			})
			if evidence == "no_fresh_launch" {
				// A live process without a recorded fresh launch is not ownership
				// authority: it may have resumed an old, incorrectly selected UUID.
				clearCodexFreshLaunchState(inst.ID)
			}
			inst.UpdateHookStatus(status)
			gotID, _, _, confirmed, err := db.ReadCodexSessionBinding(inst.ID)
			if err != nil {
				t.Fatal(err)
			}
			if evidence == "verified" {
				if gotID != id || inst.CodexSessionID != id || !confirmed || !inst.CodexBindingConfirmed {
					t.Fatalf("fresh owned identity was not confirmed atomically: memory=%q/%v db=%q/%v", inst.CodexSessionID, inst.CodexBindingConfirmed, gotID, confirmed)
				}
			} else if gotID != "" || inst.CodexSessionID != "" || confirmed || inst.CodexBindingConfirmed {
				t.Fatalf("unverified fresh identity was confirmed: memory=%q/%v db=%q/%v", inst.CodexSessionID, inst.CodexBindingConfirmed, gotID, confirmed)
			}
		})
	}
}

func TestCodexFreshLaunchCaptureSurvivesStorageReload(t *testing.T) {
	for _, launch := range []string{"start", "start_after_load", "add_attach", "start_with_message", "restart_fresh", "fork"} {
		t.Run(launch, func(t *testing.T) {
			skipIfNoTmuxBinary(t)
			inst, home := newCodexGateInstance(t)
			db := withTempGlobalStateDB(t)
			statedb.SetGlobal(nil) // One-shot CLI constructors have no global DB.
			storage := &Storage{db: db}
			binary := filepath.Join(t.TempDir(), "codex")
			if err := os.WriteFile(binary, []byte("#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then exit 0; fi\nprintf 'codex>\\n'\nwhile IFS= read -r line; do printf 'codex>\\n'; done\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			inst.Command = shellQuote(binary)
			var sourceID string
			if launch == "fork" || launch == "restart_fresh" {
				sourceID = uniqueSID(t)
				writeCodexRolloutWithSource(t, home, sourceID, inst.ProjectPath, "cli")
				if _, _, err := SetField(inst, FieldCodexSessionID, sourceID, nil); err != nil {
					t.Fatal(err)
				}
				if launch == "fork" {
					child, _, err := inst.CreateForkedCodexInstanceWithOptions("capture-child", "", nil)
					if err != nil {
						t.Fatal(err)
					}
					inst = child
				}
			}
			// CLI launch/add save newly constructed instances before spawning.
			if launch == "add_attach" {
				if err := storage.SaveWithGroups([]*Instance{inst}, nil); err != nil {
					t.Fatal(err)
				}
			} else if err := storage.InsertSessionAndVerify(inst, nil); err != nil {
				t.Fatal(err)
			}
			if launch == "start_after_load" {
				instances, err := storage.Load()
				if err != nil || len(instances) != 1 {
					t.Fatalf("load stopped CLI entry: instances=%d err=%v", len(instances), err)
				}
				inst = instances[0]
			}
			var err error
			switch launch {
			case "start_with_message":
				// The absolute fixture command deliberately uses post-start prompt
				// delivery. Present a real ready prompt and bound fixture failure
				// without changing production's cold-start readiness budget.
				pane := inst.tmuxSession
				deadline := time.AfterFunc(10*time.Second, func() { _ = pane.Kill() })
				err = inst.StartWithMessage("initial prompt")
				deadline.Stop()
			case "restart_fresh":
				err = inst.RestartFresh()
			default:
				err = inst.Start()
			}
			if err != nil {
				t.Fatalf("launch fresh conversation: %v", err)
			}
			t.Cleanup(func() { _ = inst.tmuxSession.Kill() })
			if err := storage.SaveWithGroups([]*Instance{inst}, nil); err != nil {
				t.Fatal(err)
			}
			instances, err := storage.Load()
			if err != nil || len(instances) != 1 {
				t.Fatalf("reload launcher result: instances=%d err=%v", len(instances), err)
			}
			reloaded := instances[0]
			launchRevision := reloaded.CodexBindingRevision
			// No native ID existed before the launcher saved and returned. Only
			// the reloaded daemon/TUI receives this later pane-owned event.
			id := uniqueSID(t)
			writeCodexReviewRolloutAt(t, home, id, inst.ProjectPath, time.Now())
			status := ownedCodexHook(t, reloaded, &HookStatus{
				Status: "running", SessionID: id, Event: "thread/started", UpdatedAt: time.Now(),
			})
			if launch == "fork" {
				status.SessionID = sourceID
				reloaded.UpdateHookStatus(status)
				if reloaded.CodexSessionID != "" {
					t.Fatal("reloaded fork captured its bootstrap source instead of a new child")
				}
				status.SessionID = id
			}
			reloaded.UpdateHookStatus(status)
			gotID, _, revision, confirmed, err := db.ReadCodexSessionBinding(inst.ID)
			if err != nil || gotID != id || !confirmed || !reloaded.CodexBindingConfirmed ||
				reloaded.CodexSessionID != id || revision <= launchRevision {
				t.Fatalf("reloaded owner failed durable fresh capture: memory=%q/%v db=%q/%d/%v err=%v",
					reloaded.CodexSessionID, reloaded.CodexBindingConfirmed, gotID, revision, confirmed, err)
			}
		})
	}
}

func TestCodexFreshLaunchAuthorityCannotSurviveClearOrProcessReplacement(t *testing.T) {
	for _, revoked := range []string{"explicit_clear", "new_process_lifetime", "same_pid_wrong_lifetime"} {
		t.Run(revoked, func(t *testing.T) {
			skipIfNoTmuxBinary(t)
			inst, home := newCodexGateInstance(t)
			db := withTempGlobalStateDB(t)
			statedb.SetGlobal(nil)
			storage := &Storage{db: db}
			binary := filepath.Join(t.TempDir(), "codex")
			if err := os.WriteFile(binary, []byte("#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then exit 0; fi\nwhile :; do sleep 1; done\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			inst.Command = shellQuote(binary)
			if err := storage.InsertSessionAndVerify(inst, nil); err != nil {
				t.Fatal(err)
			}
			if err := inst.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = inst.tmuxSession.Kill() })
			if err := storage.SaveWithGroups([]*Instance{inst}, nil); err != nil {
				t.Fatal(err)
			}
			instances, err := storage.Load()
			if err != nil || len(instances) != 1 {
				t.Fatalf("reload fresh launch: instances=%d err=%v", len(instances), err)
			}
			reloaded := instances[0]
			if revoked == "explicit_clear" {
				staleLaunch := inst.prepareCodexFreshLaunch()
				if _, _, err := SetField(reloaded, FieldCodexSessionID, "", nil); err != nil {
					t.Fatal(err)
				}
				if err := storage.SaveWithGroups([]*Instance{reloaded}, nil); err != nil {
					t.Fatal(err)
				}
				// An old launcher finishing after the clear cannot republish its
				// permission into the new authoritative binding generation.
				inst.recordCodexFreshLaunch(staleLaunch)
				// Saving the launcher's stale snapshot must not resurrect permission.
				if err := storage.SaveWithGroups([]*Instance{inst}, nil); err != nil {
					t.Fatal(err)
				}
			} else if revoked == "new_process_lifetime" {
				if err := reloaded.tmuxSession.RespawnPane("exec " + shellQuote(binary)); err != nil {
					t.Fatal(err)
				}
			} else {
				path := codexFreshLaunchStatePath(inst.ID)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var state codexFreshLaunchState
				if err := json.Unmarshal(data, &state); err != nil {
					t.Fatal(err)
				}
				// Keep the actual PID and valid hook emitter, but model a sidecar
				// from an earlier lifetime of that same operating-system PID.
				state.PaneIdentity += "-previous-lifetime"
				data, err = json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			id := uniqueSID(t)
			writeCodexReviewRolloutAt(t, home, id, inst.ProjectPath, time.Now())
			reloaded.UpdateHookStatus(ownedCodexHook(t, reloaded, &HookStatus{
				Status: "running", SessionID: id, Event: "thread/started", UpdatedAt: time.Now(),
			}))
			gotID, _, _, confirmed, err := db.ReadCodexSessionBinding(inst.ID)
			if err != nil || gotID != "" || confirmed || reloaded.CodexSessionID != "" {
				t.Fatalf("revoked launch confirmed a conversation: memory=%q db=%q/%v err=%v",
					reloaded.CodexSessionID, gotID, confirmed, err)
			}
		})
	}
}

func TestCodexUnconfirmedRootCannotPromoteThroughLiveSwitchOrMigration(t *testing.T) {
	inst, home := newCodexGateInstance(t)
	rootID := seedLegacyCodexBinding(t, inst, 1)
	writeCodexRolloutWithSource(t, home, rootID, inst.ProjectPath, "cli")
	targetID := uniqueSID(t)
	writeCodexRolloutWithSource(t, home, targetID, inst.ProjectPath, "cli")
	status := ownedCodexHook(t, inst, &HookStatus{
		Status: "running", SessionID: targetID, Event: "thread/started", UpdatedAt: time.Now(),
	})
	inst.UpdateHookStatus(status)
	if inst.acceptCodexSessionID(targetID, false, codexBindingProcess) {
		t.Fatal("a live native switch laundered an unconfirmed historical root")
	}
	now := time.Now()
	if err := writeCodexSubagentMigrationState(inst.ID, codexSubagentMigrationState{
		SourceID: rootID, OldRoot: rootID, TargetID: targetID,
		Started: now.Add(-time.Second), Completed: now,
	}); err != nil {
		t.Fatal(err)
	}
	if inst.adoptCompletedCodexMigrationTarget(false) {
		t.Fatal("a completed migration sidecar laundered an unconfirmed root")
	}
	if inst.CanForkCodex() {
		t.Fatal("an unconfirmed root authorized a fresh fork")
	}
	gotID, _, revision, confirmed, err := inst.stateDB.ReadCodexSessionBinding(inst.ID)
	if err != nil || gotID != rootID || inst.CodexSessionID != rootID || revision != 1 || confirmed || inst.CodexBindingConfirmed {
		t.Fatalf("unconfirmed root changed: memory=%q/%v db=%q/%v/%d err=%v", inst.CodexSessionID, inst.CodexBindingConfirmed, gotID, confirmed, revision, err)
	}
}
