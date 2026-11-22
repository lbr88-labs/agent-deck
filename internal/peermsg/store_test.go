package peermsg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func openTest(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}
func TestConcurrentSendAndRetry(t *testing.T) {
	dir := t.TempDir()
	a, b := openTest(t, dir), openTest(t, dir)
	ctx := context.Background()
	request := SendRequest{ID: "retry-key", SenderID: "leader", RecipientID: "worker", Body: "one"}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := []*Store{a, b}[i%2]
			if _, err := s.Send(ctx, request); err != nil {
				t.Error(err)
			}
			r := request
			r.ID = fmt.Sprint(i)
			if _, err := s.Send(ctx, r); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	page, err := a.Pending(ctx, "worker", 0, 10)
	if err != nil || len(page) != 10 {
		t.Fatalf("page=%v err=%v", page, err)
	}
	next, err := b.Pending(ctx, "worker", page[9].Sequence, 20)
	if err != nil || len(next) != 11 {
		t.Fatalf("next=%v err=%v", next, err)
	}
	for _, m := range append(page, next...) {
		if m.AcknowledgedAt != nil {
			t.Fatalf("read acknowledged %v", m)
		}
	}
	request.Body = "modified"
	if _, err := a.Send(ctx, request); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict: %v", err)
	}
}
func TestProcessExitAndRecipientAck(t *testing.T) {
	if dir := os.Getenv("PEERMSG_CRASH_TEST_DIR"); dir != "" {
		s, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Send(context.Background(), SendRequest{ID: "saved", SenderID: "one", RecipientID: "two", Body: "persist"}); err != nil {
			t.Fatal(err)
		}
		os.Exit(0) // deliberately skip Close and all cleanup
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessExitAndRecipientAck$")
	cmd.Env = append(os.Environ(), "PEERMSG_CRASH_TEST_DIR="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	info, err := os.Stat(filepath.Join(dir, "peer-messages.db"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("database mode: %v %v", info, err)
	}
	s := openTest(t, dir)
	ctx := context.Background()
	if _, err := s.Ack(ctx, "one", "saved"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("wrong recipient: %v", err)
	}
	m, err := s.Ack(ctx, "two", "saved")
	if err != nil || m.AcknowledgedAt == nil {
		t.Fatalf("%v %v", m, err)
	}
	again, err := s.Ack(ctx, "two", "saved")
	if err != nil || !again.AcknowledgedAt.Equal(*m.AcknowledgedAt) {
		t.Fatalf("repeat ack: %v %v", again, err)
	}
	p, err := s.Pending(ctx, "two", 0, 1)
	if err != nil || len(p) != 0 {
		t.Fatalf("acked pending: %v %v", p, err)
	}
}
func TestWaitAndCorrelation(t *testing.T) {
	dir := t.TempDir()
	a, b := openTest(t, dir), openTest(t, dir)
	ctx := context.Background()
	req := SendRequest{ID: "task", SenderID: "coordinator", RecipientID: "worker", Body: "question"}
	done := make(chan error, 1)
	go func() {
		p, err := b.Wait(ctx, "worker", 0, 1, 2*time.Second)
		if err == nil && (len(p) != 1 || p[0].ID != req.ID) {
			err = fmt.Errorf("wait returned %v", p)
		}
		done <- err
	}()
	if _, err := a.Send(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r, err := b.WaitResponse(ctx, "coordinator", "task", 5*time.Millisecond)
	if err != nil || !r.TimedOut {
		t.Fatalf("empty response: %v %v", r, err)
	}
	reply := SendRequest{SenderID: "other", RecipientID: "coordinator", ReplyTo: "task", Body: "answer"}
	if _, err := a.Send(ctx, reply); err == nil {
		t.Fatal("accepted unrelated reply")
	}
	reply.SenderID = "worker"
	if _, err := a.Send(ctx, reply); err != nil {
		t.Fatal(err)
	}
	r, err = b.WaitResponse(ctx, "coordinator", "task", time.Second)
	if err != nil || r.TimedOut || len(r.Replies) != 1 || r.Message.AcknowledgedAt != nil {
		t.Fatalf("reply response: %v %v", r, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := a.Wait(canceled, "missing", 0, 1, MaxWait); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
}

func TestHasPendingReadOnly(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	check := func(recipient string, want bool) {
		t.Helper()
		got, err := HasPending(ctx, dir, recipient)
		if err != nil || got != want {
			t.Fatalf("HasPending(%q) = %v, %v; want %v", recipient, got, err, want)
		}
	}
	check("worker", false)
	if _, err := os.Stat(filepath.Join(dir, DatabaseName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("absent database stat: %v", err)
	}
	s := openTest(t, dir)
	msg, err := s.Send(ctx, SendRequest{SenderID: "leader", RecipientID: "worker", Body: "pending"})
	if err != nil {
		t.Fatal(err)
	}
	check("worker", true)
	check("other", false)
	if _, err = s.Ack(ctx, "worker", msg.ID); err != nil {
		t.Fatal(err)
	}
	check("worker", false)
}

func TestConcurrentProcesses(t *testing.T) {
	if dir := os.Getenv("PEERMSG_PARALLEL_TEST_DIR"); dir != "" {
		s := openTest(t, dir)
		for i := 0; i < 10; i++ {
			req := SendRequest{ID: fmt.Sprintf("%s-%d", os.Getenv("PEERMSG_WRITER_ID"), i), SenderID: "sender", RecipientID: "target", Body: "work"}
			if _, err := s.Send(context.Background(), req); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestConcurrentProcesses$")
			cmd.Env = append(os.Environ(), "PEERMSG_PARALLEL_TEST_DIR="+dir, fmt.Sprintf("PEERMSG_WRITER_ID=%d", i))
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("writer %d: %v: %s", i, err, output)
			}
		}(i)
	}
	wg.Wait()
	s := openTest(t, dir)
	var after int64
	var count int
	seen := make(map[string]bool)
	for {
		rows, err := s.Pending(context.Background(), "target", after, 7)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if row.Sequence <= after || seen[row.ID] {
				t.Fatalf("repeated or unordered row: %v", row)
			}
			seen[row.ID] = true
			after = row.Sequence
			count++
		}
	}
	if count != 30 {
		t.Fatalf("received %d of 30 process writes", count)
	}
}
