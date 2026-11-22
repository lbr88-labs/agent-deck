package session

import (
	"context"
	"fmt"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
)

// DispatchReceipt identifies an accepted instruction and the boundary for fresh output.
type DispatchReceipt struct {
	ID          string
	SubmittedAt time.Time
}

// QueueOperatorMessage routes explicit human input without touching a terminal.
// Codex provides its own live-thread queue. Other agents read the profile inbox.
func (i *Instance) QueueOperatorMessage(ctx context.Context, profile, body string) (DispatchReceipt, error) {
	if IsCodexCompatible(i.Tool) {
		return i.QueueCodexMessage(ctx, body)
	}
	return i.QueueOperatorRequest(ctx, profile, body)
}

// QueueOperatorRequest requests an explicitly correlated inbox reply from any harness.
func (i *Instance) QueueOperatorRequest(ctx context.Context, profile, body string) (DispatchReceipt, error) {
	return queueInboxMessage(ctx, profile, "system:operator", i.ID, body)
}

func queueInboxMessage(ctx context.Context, profile, sender, recipient, body string) (DispatchReceipt, error) {
	dir, err := GetProfileDir(profile)
	if err != nil {
		return DispatchReceipt{}, err
	}
	store, err := peermsg.Open(dir)
	if err != nil {
		return DispatchReceipt{}, err
	}
	defer func() { _ = store.Close() }()
	m, err := store.Send(ctx, peermsg.SendRequest{SenderID: sender, RecipientID: recipient, Body: body})
	if err != nil {
		return DispatchReceipt{}, err
	}
	return DispatchReceipt{ID: m.ID, SubmittedAt: m.AcceptedAt}, nil
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
		res, err := store.WaitResponse(ctx, "system:operator", messageID, peermsg.ResponseOptions{Timeout: peermsg.MaxWait, ReplyOnly: true})
		if err != nil {
			return peermsg.Message{}, err
		}
		if len(res.Replies) > 0 {
			return res.Replies[0], nil
		}
	}
	return peermsg.Message{}, fmt.Errorf("waiting for reply to %s: %w", messageID, ctx.Err())
}
