// Package peermsg stores durable peer mail for one Agent Deck profile.
package peermsg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const (
	DatabaseName = "peer-messages.db"
	MaxWait      = 30 * time.Second
	MaxLimit     = 200
	DefaultLimit = 50
	MaxBodyBytes = 64 * 1024
)

type SendRequest struct {
	ID          string `json:"id"`
	SenderID    string `json:"sender_id"`
	RecipientID string `json:"recipient_id"`
	Body        string `json:"body"`
	ReplyTo     string `json:"reply_to,omitempty"`
}

type Message struct {
	SendRequest
	Sequence       int64      `json:"sequence"`
	AcceptedAt     time.Time  `json:"accepted_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
}

type Response struct {
	Message  Message   `json:"message"`
	Replies  []Message `json:"replies"`
	TimedOut bool      `json:"timed_out"`
}

var ErrConflict = errors.New("message ID already exists for a different request")

type Store struct{ db *sql.DB }

func Open(profileDir string) (*Store, error) {
	if profileDir == "" {
		return nil, errors.New("missing profile directory")
	}
	dir, err := filepath.Abs(profileDir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, DatabaseName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	addr := url.URL{Scheme: "file", Path: path, RawQuery: "_pragma=busy_timeout(5000)&_txlock=immediate"}
	db, err := sql.Open("sqlite", addr.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS messages(
 seq INTEGER PRIMARY KEY, id TEXT NOT NULL UNIQUE, sender TEXT NOT NULL, recipient TEXT NOT NULL, body TEXT NOT NULL,
 reply_to TEXT NOT NULL, accepted INTEGER NOT NULL, acknowledged INTEGER
 );
 CREATE INDEX IF NOT EXISTS pending_recipient ON messages(recipient,seq) WHERE acknowledged IS NULL;
 CREATE INDEX IF NOT EXISTS correlated_replies ON messages(reply_to,seq) WHERE reply_to <> '';`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.db.Close() }

const columns = "seq,id,sender,recipient,body,reply_to,accepted,acknowledged"

type scanner interface{ Scan(...any) error }

func read(row scanner) (Message, error) {
	var m Message
	var accepted int64
	var ack sql.NullInt64
	err := row.Scan(&m.Sequence, &m.ID, &m.SenderID, &m.RecipientID, &m.Body, &m.ReplyTo, &accepted, &ack)
	if err != nil {
		return Message{}, err
	}
	m.AcceptedAt = time.UnixMicro(accepted).UTC()
	if ack.Valid {
		at := time.UnixMicro(ack.Int64).UTC()
		m.AcknowledgedAt = &at
	}
	return m, nil
}
func readAll(rows *sql.Rows, err error) ([]Message, error) {
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []Message{}
	for rows.Next() {
		m, err := read(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (s *Store) Send(ctx context.Context, r SendRequest) (Message, error) {
	if strings.TrimSpace(r.SenderID) == "" || strings.TrimSpace(r.RecipientID) == "" || strings.TrimSpace(r.Body) == "" {
		return Message{}, errors.New("sender, recipient and body are required")
	}
	if len(r.Body) > MaxBodyBytes {
		return Message{}, fmt.Errorf("body exceeds %d bytes", MaxBodyBytes)
	}
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Message{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if r.ReplyTo != "" {
		var exists int
		err = tx.QueryRowContext(ctx, "SELECT 1 FROM messages WHERE id=? AND sender=? AND recipient=?", r.ReplyTo, r.RecipientID, r.SenderID).Scan(&exists)
		if err != nil {
			return Message{}, fmt.Errorf("matching reply parent: %w", err)
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO messages(id,sender,recipient,body,reply_to,accepted) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, r.ID, r.SenderID, r.RecipientID, r.Body, r.ReplyTo, time.Now().UnixMicro())
	if err != nil {
		return Message{}, err
	}
	m, err := read(tx.QueryRowContext(ctx, "SELECT "+columns+" FROM messages WHERE id=?", r.ID))
	if err != nil {
		return Message{}, err
	}
	if m.SendRequest != r {
		return Message{}, ErrConflict
	}
	return m, tx.Commit()
}
func (s *Store) Get(ctx context.Context, id string) (Message, error) {
	return read(s.db.QueryRowContext(ctx, "SELECT "+columns+" FROM messages WHERE id=?", id))
}
func (s *Store) Ack(ctx context.Context, recipient, id string) (Message, error) {
	return read(s.db.QueryRowContext(ctx, "UPDATE messages SET acknowledged=COALESCE(acknowledged,?) WHERE id=? AND recipient=? RETURNING "+columns, time.Now().UnixMicro(), id, recipient))
}
func (s *Store) Pending(ctx context.Context, recipient string, after int64, limit int) ([]Message, error) {
	if limit == 0 {
		limit = DefaultLimit
	}
	if after < 0 || limit < 1 || limit > MaxLimit || recipient == "" {
		return nil, errors.New("invalid inbox recipient, cursor or limit")
	}
	return readAll(s.db.QueryContext(ctx, "SELECT "+columns+" FROM messages WHERE recipient=? AND seq>? AND acknowledged IS NULL ORDER BY seq LIMIT ?", recipient, after, limit))
}

func poll(ctx context.Context, timeout time.Duration, check func(context.Context) (bool, error)) (bool, error) {
	if timeout < 0 || timeout > MaxWait {
		return false, fmt.Errorf("timeout must be between zero and %s", MaxWait)
	}
	if timeout == 0 {
		_, err := check(ctx)
		return false, err
	}
	wait, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		ready, err := check(wait)
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return false, err
		}
		if ready {
			return false, nil
		}
		select {
		case <-wait.Done():
			return true, ctx.Err()
		case <-ticker.C:
		}
	}
}
func (s *Store) Wait(ctx context.Context, recipient string, after int64, limit int, timeout time.Duration) ([]Message, error) {
	result := []Message{}
	_, err := poll(ctx, timeout, func(check context.Context) (bool, error) {
		messages, err := s.Pending(check, recipient, after, limit)
		if err == nil {
			result = messages
		}
		return len(messages) > 0, err
	})
	return result, err
}
func (s *Store) WaitResponse(ctx context.Context, sender, id string, timeout time.Duration) (Response, error) {
	return s.waitResponse(ctx, sender, id, timeout, false)
}

func (s *Store) WaitReply(ctx context.Context, sender, id string, timeout time.Duration) (Response, error) {
	return s.waitResponse(ctx, sender, id, timeout, true)
}

func (s *Store) waitResponse(ctx context.Context, sender, id string, timeout time.Duration, replyOnly bool) (Response, error) {
	var result Response
	var ready bool
	timedOut, err := poll(ctx, timeout, func(check context.Context) (bool, error) {
		m, err := read(s.db.QueryRowContext(check, "SELECT "+columns+" FROM messages WHERE id=? AND sender=?", id, sender))
		if err != nil {
			return false, err
		}
		replies, err := readAll(s.db.QueryContext(check, "SELECT "+columns+" FROM messages WHERE reply_to=? AND reply_to<>'' ORDER BY seq LIMIT ?", id, MaxLimit))
		if err != nil {
			return false, err
		}
		result = Response{Message: m, Replies: replies}
		ready = (!replyOnly && m.AcknowledgedAt != nil) || len(replies) > 0
		return ready, nil
	})
	result.TimedOut = timedOut || (timeout == 0 && !ready)
	return result, err
}

// HasPending checks for inbox notices without creating or initializing the store.
func HasPending(ctx context.Context, profileDir, recipient string) (bool, error) {
	if profileDir == "" || recipient == "" {
		return false, errors.New("profile directory and recipient are required")
	}
	path, err := filepath.Abs(filepath.Join(profileDir, DatabaseName))
	if err != nil {
		return false, err
	}
	if _, err = os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	addr := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro&_pragma=busy_timeout(100)"}
	db, err := sql.Open("sqlite", addr.String())
	if err != nil {
		return false, err
	}
	defer func() { _ = db.Close() }()
	var pending bool
	err = db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM messages WHERE recipient=? AND acknowledged IS NULL)", recipient).Scan(&pending)
	return pending, err
}
