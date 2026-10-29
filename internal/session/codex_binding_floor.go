package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type codexBindingFloorState struct {
	SessionID  string    `json:"session_id,omitempty"`
	DetectedAt time.Time `json:"detected_at"`
	Revision   int64     `json:"revision,omitempty"`
}

func codexBindingFloorStatePath(instanceID string) string {
	return filepath.Join(GetHooksDir(), strings.TrimSpace(instanceID)+".codex-binding")
}

func writeCodexBindingFloorState(instanceID, sessionID string, detectedAt time.Time, revision int64) {
	instanceID = strings.TrimSpace(instanceID)
	if !validHookSessionAnchorInstanceID(instanceID) || detectedAt.IsZero() {
		return
	}
	state := codexBindingFloorState{
		SessionID:  strings.ToLower(strings.TrimSpace(sessionID)),
		DetectedAt: detectedAt,
		Revision:   max(revision, int64(0)),
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	_ = writeInternalStateFileAtomic(codexBindingFloorStatePath(instanceID), data, 0o600)
}

func ensureCodexBindingFloorState(instanceID, sessionID string, detectedAt time.Time, revision int64) {
	sessionID = strings.ToLower(strings.TrimSpace(sessionID))
	if current, ok := readCodexBindingFloorState(instanceID); ok && current.SessionID == sessionID {
		requestedRevision := max(revision, int64(0))
		if current.Revision > requestedRevision ||
			(current.Revision == requestedRevision && !detectedAt.After(current.DetectedAt)) {
			return
		}
		if current.DetectedAt.After(detectedAt) {
			detectedAt = current.DetectedAt
		}
	}
	writeCodexBindingFloorState(instanceID, sessionID, detectedAt, revision)
}

func readCodexBindingFloorState(instanceID string) (codexBindingFloorState, bool) {
	instanceID = strings.TrimSpace(instanceID)
	if !validHookSessionAnchorInstanceID(instanceID) {
		return codexBindingFloorState{}, false
	}
	data, err := readStatusFileNoFollow(codexBindingFloorStatePath(instanceID))
	if err != nil || len(data) > 16*1024 {
		return codexBindingFloorState{}, false
	}
	var state codexBindingFloorState
	if json.Unmarshal(data, &state) != nil || state.DetectedAt.IsZero() {
		return codexBindingFloorState{}, false
	}
	state.SessionID = strings.ToLower(strings.TrimSpace(state.SessionID))
	return state, true
}

func clearCodexBindingFloorState(instanceID string) {
	instanceID = strings.TrimSpace(instanceID)
	if !validHookSessionAnchorInstanceID(instanceID) {
		return
	}
	_ = os.Remove(codexBindingFloorStatePath(instanceID))
}

// CodexHookCandidateRejectedByBindingFloor lets the notify subprocess apply
// the same immutable creation-time floor as the runtime before it updates the
// sticky anchor/status files. It is called while the shared hook lock is held.
func CodexHookCandidateRejectedByBindingFloor(instanceID, candidateID, codexHome string) bool {
	state, ok := readCodexBindingFloorState(instanceID)
	if !ok {
		return false
	}
	candidateID = strings.ToLower(strings.TrimSpace(candidateID))
	if candidateID == "" || candidateID == state.SessionID {
		return false
	}
	if CodexSessionRolloutExists(codexHome, candidateID) &&
		!IsCodexTopLevelSession(codexHome, candidateID) {
		return true
	}
	createdAt := codexRolloutCreatedAt(codexHome, candidateID)
	return createdAt.IsZero() || createdAt.Before(state.DetectedAt)
}

// Fresh capture authority belongs to one successful unbound launch, not to the
// launcher process. A CLI may exit before Codex emits its first conversation ID.
// Keep this separate from the binding floor: floor reconciliation cannot create
// launch authority, and a failed binding CAS must not erase a still-live launch.
type codexFreshLaunchState struct {
	Revision      int64     `json:"revision"`
	DetectedAt    time.Time `json:"binding_detected_at"`
	CodexHome     string    `json:"codex_home"`
	ForkSourceID  string    `json:"fork_source_id,omitempty"`
	PanePID       int       `json:"pane_pid"`
	PaneIdentity  string    `json:"pane_identity"`
	CodexPID      int       `json:"codex_pid"`
	CodexIdentity string    `json:"codex_identity"`
}

func codexFreshLaunchStatePath(instanceID string) string {
	return filepath.Join(GetHooksDir(), strings.TrimSpace(instanceID)+".codex-launch")
}

// Snapshot before building/spawning the command. A concurrent clear must not
// allow the old launch to mint authority for the new binding generation.
func (i *Instance) prepareCodexFreshLaunch() codexFreshLaunchState {
	forkSourceID := ""
	if i.IsForkAwaitingStart && i.ForkStartCommand != "" {
		forkSourceID = strings.ToLower(strings.TrimSpace(i.CodexForkSourceSessionID))
	}
	if !IsCodexCompatible(i.Tool) || i.CodexSessionID != "" ||
		(i.Tool == "codex" && forkSourceID == "" && !IsSupportedCodexLaunchCommand(i.resolveCodexCommand(i.Command))) {
		return codexFreshLaunchState{Revision: -1}
	}
	if i.validateCodexFreshWrapper() != nil {
		return codexFreshLaunchState{Revision: -1}
	}
	return codexFreshLaunchState{
		Revision: i.CodexBindingRevision, DetectedAt: i.CodexDetectedAt,
		CodexHome:    i.getCodexHomeDir(),
		ForkSourceID: forkSourceID,
	}
}

// Called only after a successful Start/respawn, while its spawn lock is held.
func (i *Instance) recordCodexFreshLaunch(state codexFreshLaunchState) error {
	if state.Revision < 0 {
		return nil
	}
	if !validHookSessionAnchorInstanceID(i.ID) {
		return i.codexFreshLaunchError("the instance identity is invalid")
	}
	// tmux can return before an npm/shell launcher has spawned Codex. Observe
	// that launch synchronously so a short-lived CLI does not leave permission
	// attached only to an outer shell that can outlive the agent.
	deadline := time.Now().Add(2 * time.Second)
	for {
		state.CodexPID = i.codexInteractiveProcessPID()
		if state.CodexPID != 0 {
			if !codexProcessMatchesFreshLaunch(state.CodexPID, state.ForkSourceID) {
				return i.codexFreshLaunchError("the actual Codex process is not the verified fresh invocation")
			}
			state.CodexIdentity = CodexProcessIdentity(state.CodexPID)
			if state.CodexIdentity != "" {
				break
			}
		}
		if !time.Now().Before(deadline) {
			return i.codexFreshLaunchError("no verifiable Codex process appeared during launch registration")
		}
		time.Sleep(25 * time.Millisecond)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	release, err := AcquireHookSessionLock(i.ID)
	if err != nil {
		sessionLog.Warn("codex_fresh_launch_lock_failed", "instance_id", i.ID, "error", err)
		return i.codexFreshLaunchError("the launch ownership lock could not be acquired")
	}
	defer release()
	if i.CodexSessionID != "" || i.CodexBindingRevision != state.Revision ||
		!i.CodexDetectedAt.Equal(state.DetectedAt) || i.getCodexHomeDir() != state.CodexHome {
		return i.codexFreshLaunchError("the binding changed while Codex was starting")
	}
	if db := i.metadataStateDB(); db != nil {
		id, at, revision, _, err := db.ReadCodexSessionBinding(i.ID)
		if err != nil || id != "" || revision != state.Revision || at.Unix() != state.DetectedAt.Unix() {
			return i.codexFreshLaunchError("the authoritative binding changed or could not be read")
		}
	}
	if i.tmuxSession == nil {
		return i.codexFreshLaunchError("the launched pane is unavailable")
	}
	state.PanePID = i.readPanePID()
	state.PaneIdentity = CodexProcessIdentity(state.PanePID)
	if state.PanePID <= 0 || state.PaneIdentity == "" ||
		CodexProcessIdentity(state.CodexPID) != state.CodexIdentity ||
		!codexProcessMatchesFreshLaunch(state.CodexPID, state.ForkSourceID) {
		return i.codexFreshLaunchError("the launched Codex process ended or changed before registration")
	}
	data, err := json.Marshal(state)
	if err == nil {
		err = writeInternalStateFileAtomic(codexFreshLaunchStatePath(i.ID), data, 0o600)
	}
	if err != nil {
		sessionLog.Warn("codex_fresh_launch_persist_failed", "instance_id", i.ID, "error", err)
		return i.codexFreshLaunchError("the verified launch identity could not be persisted")
	}
	return nil
}

func (i *Instance) codexFreshLaunchError(reason string) error {
	return i.refuseCodexLaunch(fmt.Sprintf("%s; fresh ownership was not registered. The pane and native history are preserved. Once Codex is ready, verify its conversation and run `agent-deck session set %s codex-session-id <verified-uuid>`", reason, i.ID))
}

// The hook lock is held by the caller. This is permission to examine a fresh
// candidate, not proof of its UUID; pane ownership and native metadata are still
// required by acceptCodexSessionID. Legacy nonempty roots never use this path.
func (i *Instance) codexFreshLaunchAllowsCandidate(candidateID string) bool {
	if i.CodexSessionID != "" || i.tmuxSession == nil || !validHookSessionAnchorInstanceID(i.ID) {
		return false
	}
	data, err := readStatusFileNoFollow(codexFreshLaunchStatePath(i.ID))
	if err != nil || len(data) > 16*1024 {
		return false
	}
	var state codexFreshLaunchState
	if json.Unmarshal(data, &state) != nil || state.Revision != i.CodexBindingRevision ||
		state.DetectedAt.Unix() != i.CodexDetectedAt.Unix() || state.CodexHome != i.getCodexHomeDir() ||
		state.PanePID <= 0 || state.PaneIdentity == "" ||
		state.CodexPID <= 0 || state.CodexIdentity == "" || candidateID == state.ForkSourceID {
		return false
	}
	if i.readPanePID() != state.PanePID || CodexProcessIdentity(state.PanePID) != state.PaneIdentity ||
		CodexProcessIdentity(state.CodexPID) != state.CodexIdentity ||
		!codexProcessMatchesFreshLaunch(state.CodexPID, state.ForkSourceID) {
		return false
	}
	currentPID := i.codexInteractiveProcessPID()
	if currentPID == 0 || !codexProcessMatchesFreshLaunch(currentPID, state.ForkSourceID) {
		return false
	}
	if currentPID == state.CodexPID {
		return true
	}
	// npm launchers can spawn the native executable after registration. It
	// must still descend from the same live Codex launcher, not another pane
	// process or a later command started in the fallback shell.
	for _, pid := range collectProcessTreePIDsViaPgrep(state.CodexPID) {
		if pid == currentPID {
			return true
		}
	}
	return false
}

func codexProcessMatchesFreshLaunch(pid int, forkSourceID string) bool {
	command, args, ok := codexInteractiveInvocation(codexProcessArguments(pid))
	if !ok {
		return false
	}
	switch command {
	case "resume", "r":
		return false
	case "fork":
		if forkSourceID == "" || len(args) == 0 || !strings.EqualFold(args[0], forkSourceID) {
			return false
		}
		optionCommand, _, ok := codexInteractiveArguments(args[1:])
		return ok && optionCommand == ""
	default:
		optionCommand, _, ok := codexInteractiveArguments(args)
		return forkSourceID == "" && ok && optionCommand == ""
	}
}

func clearCodexFreshLaunchState(instanceID string) {
	if validHookSessionAnchorInstanceID(instanceID) {
		_ = os.Remove(codexFreshLaunchStatePath(instanceID))
	}
}
