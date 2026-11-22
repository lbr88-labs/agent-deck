package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
	"github.com/asheshgoplani/agent-deck/internal/session"
	"github.com/google/uuid"
)

// A managed initial task is persisted before spawning so even a fast startup
// hook can discover it. A start retry can use the receipt or omit the body.
type peerBootstrap struct {
	service   *peerService
	requestID string
	recipient string
}

func preparePeerBootstrap(profile, body, requestID string) (*peerBootstrap, error) {
	if body == "" || !sessionSendFromAgent() {
		if requestID != "" {
			return nil, errors.New("--message-id requires an initial message from a managed agent")
		}
		return nil, nil
	}
	service, err := openPeerService(profile)
	if err != nil {
		return nil, err
	}
	if requestID == "" {
		requestID = uuid.NewString()
	}
	return &peerBootstrap{service: service, requestID: requestID}, nil
}

// terminalBody is empty after a successful inbox handoff. One-shot tools
// which require the task as an argv operand have no receiver to poll an inbox.
func (b *peerBootstrap) terminalBody(target *session.Instance, body string) (string, error) {
	if b == nil || target.PromptRidesCommandLine() {
		return body, nil
	}
	if err := target.PromptDeliveryError(); err != nil {
		return "", err
	}
	if err := b.service.validateRecipient(target.ID); err != nil {
		return "", err
	}
	_, err := b.service.store.Send(context.Background(), peermsg.SendRequest{
		ID: b.requestID, SenderID: b.service.self, RecipientID: target.ID, Body: body,
	})
	if err != nil {
		return "", fmt.Errorf("enqueue initial task %s for session %s: %w", b.requestID, target.ID, err)
	}
	b.recipient = target.ID
	return "", nil
}

func (b *peerBootstrap) addReceipt(data map[string]interface{}) {
	if b != nil && b.recipient != "" {
		data["accepted"], data["message_id"], data["transport"] = true, b.requestID, "inbox"
		data["message_pending"] = true
	}
}

func (b *peerBootstrap) reportError(out *CLIOutput, operation string, err error) {
	if b == nil || b.recipient == "" {
		out.Error(fmt.Sprintf("%s: %v", operation, err), ErrCodeInvalidOperation)
		return
	}
	data := map[string]interface{}{"session_id": b.recipient}
	b.addReceipt(data)
	out.ErrorWithData(fmt.Sprintf("%s: %v; task %s is queued for session %s. If stopped, restart it without a message", operation, err, b.requestID, b.recipient),
		ErrCodeInvalidOperation, data)
}
