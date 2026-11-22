package session

import (
	"context"
	"fmt"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
)

// QueueOperatorMessage routes explicit human input without touching a terminal.
// Codex provides its own live-thread queue. Other agents read the profile inbox.
func (i *Instance) QueueOperatorMessage(ctx context.Context, profile, body string) (string, error) {
	if IsCodexCompatible(i.Tool) {
		return i.QueueCodexMessage(ctx, body)
	}
	return i.QueueOperatorRequest(ctx, profile, body)
}

// QueueOperatorRequest requests an explicitly correlated inbox reply from any harness.
func (i *Instance) QueueOperatorRequest(ctx context.Context, profile, body string) (string, error) {
	return queueInboxMessage(ctx, profile, "system:operator", i.ID, body)
}

func queueInboxMessage(ctx context.Context, profile, sender, recipient, body string) (string, error) {
	dir, err := GetProfileDir(profile)
	if err != nil {
		return "", err
	}
	store, err := peermsg.Open(dir)
	if err != nil {
		return "", err
	}
	defer func() { _ = store.Close() }()
	m, err := store.Send(ctx, peermsg.SendRequest{SenderID: sender, RecipientID: recipient, Body: body})
	if err != nil {
		return "", err
	}
	return m.ID, nil
}

// WaitOperatorReply waits for a reply explicitly linked to the operator's request.
func WaitOperatorReply(ctx context.Context, profile, messageID string) (peermsg.Message, error) {
	dir, err := GetProfileDir(profile)
	if err != nil {
		return peermsg.Message{}, err
	}
	store, err := peermsg.Open(dir)
	if err != nil {
		return peermsg.Message{}, err
	}
	defer func() { _ = store.Close() }()
	for ctx.Err() == nil {
		res, err := store.WaitReply(ctx, "system:operator", messageID, peermsg.MaxWait)
		if err != nil {
			return peermsg.Message{}, err
		}
		if len(res.Replies) > 0 {
			return res.Replies[0], nil
		}
	}
	return peermsg.Message{}, fmt.Errorf("waiting for reply to %s: %w", messageID, ctx.Err())
}
