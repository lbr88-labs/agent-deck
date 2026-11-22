package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
	"github.com/asheshgoplani/agent-deck/internal/statedb"
)

// WakeCodexPeer rings the native doorbell, without copying the durable message into the notice.
func (s *Storage) WakeCodexPeer(ctx context.Context, store *peermsg.Store, message peermsg.Message) error {
	if message.AcknowledgedAt != nil || strings.HasPrefix(message.RecipientID, "system:") {
		return nil
	}
	inst, err := s.codexWakeTarget(message.RecipientID)
	if err == nil && inst != nil && inst.Exists() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		const notice = "Agent Deck inbox notification. Read pending messages using MCP peer_read or agent-deck message read --json. Acknowledge each message you received using peer_ack or agent-deck message ack with its ID."
		var receipt DispatchReceipt
		receipt, err = inst.QueueCodexMessageIf(ctx, notice, func() (bool, error) {
			return store.ClaimNativeWake(ctx, message)
		})
		if err == nil && receipt.ID != "" {
			err = store.RecordNativeWake(ctx, message.ID, receipt.ID)
		}
	}
	if err != nil {
		return fmt.Errorf("message %q accepted, Codex inbox wake: %w", message.ID, err)
	}
	return nil
}

func (s *Storage) codexWakeTarget(id string) (*Instance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil, nil
	}
	row, err := s.db.LoadInstanceByID(id)
	if err != nil || row == nil || !IsCodexCompatible(row.Tool) || row.TmuxSession == "" || !row.ArchivedAt.IsZero() {
		return nil, err
	}
	instances, _, err := s.convertStoredRows([]*statedb.InstanceRow{row}, nil)
	if err != nil {
		return nil, err
	}
	return instances[0], nil
}
