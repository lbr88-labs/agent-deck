package peermsg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ClaimNativeWake records permission for one non-idempotent native Codex queue
// invocation. A process crash after this commit leaves its delivery uncertain.
func (s *Store) ClaimNativeWake(ctx context.Context, message Message) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS codex_wakes (
 message_id TEXT PRIMARY KEY, native_receipt TEXT)`); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO codex_wakes(message_id)
 SELECT id FROM messages WHERE id=? AND recipient=? AND acknowledged IS NULL
 ON CONFLICT(message_id) DO NOTHING`, message.ID, message.RecipientID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		var receipt sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT w.native_receipt FROM codex_wakes w
 JOIN messages m ON m.id=w.message_id
 WHERE m.id=? AND m.recipient=? AND m.acknowledged IS NULL`, message.ID, message.RecipientID).Scan(&receipt)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}
		if err == nil && !receipt.Valid {
			return false, fmt.Errorf("Codex wake for accepted message %q is unconfirmed", message.ID)
		}
	}
	return n == 1, tx.Commit()
}

func (s *Store) RecordNativeWake(ctx context.Context, id, receipt string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE codex_wakes SET native_receipt=? WHERE message_id=?", receipt, id)
	return err
}
