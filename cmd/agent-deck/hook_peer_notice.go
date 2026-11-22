package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
	"github.com/asheshgoplani/agent-deck/internal/session"
)

const pendingPeerNotice = "You have pending Agent Deck peer messages. Read your inbox with MCP peer_read or agent-deck message read --json. Acknowledge receipt of each message by ID with MCP peer_ack or agent-deck message ack <id>."

func peerInboxHookNotice(instanceID string) string {
	// On SessionStart, the native hook must be able to start a new turn for
	// mail that was queued while the harness was stopped.
	if session.IsClaudeCompatible(os.Getenv("AGENTDECK_TOOL")) && session.CheckClaudeHooksInstalled(getClaudeConfigDirForHooks()) {
		return ""
	}
	return pendingPeerHookNotice(instanceID)
}

func pendingPeerHookNotice(instanceID string) string {
	if !validInstanceID.MatchString(instanceID) || strings.Contains(instanceID, "..") {
		return ""
	}
	profile, err := session.ResolveProfileForStorage("")
	if err != nil {
		return ""
	}
	dir, err := session.GetProfileDir(profile)
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if pending, err := peermsg.HasPending(ctx, dir, instanceID); err == nil && pending {
		return pendingPeerNotice
	}
	return ""
}

// The existing completion hook can already be blocking. Combine both reasons
// into its single native response. Only an explicitly fresh Claude Stop may
// wake the agent for mail, even if messages remain unacknowledged.
func withPeerStopNotice(dec session.StopHookDecision, payload hookPayload, instanceID string) session.StopHookDecision {
	if payload.HookEventName != "Stop" || resolveStopHookActive(payload) || !session.IsClaudeCompatible(os.Getenv("AGENTDECK_TOOL")) {
		return dec
	}
	var notice string
	if session.CheckClaudeHooksInstalled(getClaudeConfigDirForHooks()) {
		if !validInstanceID.MatchString(instanceID) || strings.Contains(instanceID, "..") || payload.SessionID == "" {
			return dec
		}
		notice = peerLegacyStopNotice(instanceID, payload.SessionID)
	} else {
		notice = pendingPeerHookNotice(instanceID)
	}
	if notice != "" {
		dec.Decision = "block"
		dec.Reason = strings.TrimSpace(dec.Reason + "\n" + notice)
	}
	return dec
}

// Settings can change before a running Claude reloads its hook snapshot. On a
// fresh Stop, share the sequence cursor with the native watcher so that an old
// snapshot can still deliver mail without racing a newly armed background hook.
func peerLegacyStopNotice(instance, sid string) string {
	profile, err := session.ResolveProfileForStorage("")
	if err != nil {
		return ""
	}
	dir, err := session.GetProfileDir(profile)
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	seq, err := peermsg.PendingSequence(ctx, dir, instance)
	if err != nil || seq == 0 {
		return ""
	}
	source, err := peerWakeSource(profile, instance, sid)
	if err != nil {
		return ""
	}
	w := newPeerWake(dir, instance)
	release, err := w.lock(ctx)
	if err != nil {
		return ""
	}
	defer release()
	harness := source.harness(ctx)
	if harness.PID == 0 {
		return ""
	}
	state, err := w.state()
	if err != nil {
		return ""
	}
	if state.Harness == harness && state.SessionID == sid && (state.Notified >= seq || state.Watcher.live()) {
		return ""
	}
	state = peerWakeState{Harness: harness, SessionID: sid, Notified: seq}
	if w.save(state) != nil {
		return ""
	}
	return pendingPeerNotice
}
