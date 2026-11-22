package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
	"github.com/asheshgoplani/agent-deck/internal/session"
)

const pendingPeerNotice = "You have pending Agent Deck peer messages. Read your inbox with MCP peer_read or agent-deck message read --json."

func peerInboxHookNotice(instanceID string) string {
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
	if notice := peerInboxHookNotice(instanceID); notice != "" {
		dec.Decision = "block"
		dec.Reason = strings.TrimSpace(dec.Reason + "\n" + notice)
	}
	return dec
}
