package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
	"github.com/asheshgoplani/agent-deck/internal/session"
)

func TestPeerWakeReplacementAndRecovery(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	newWatcher := func() peerWake {
		w := newPeerWake(dir, "recipient")
		w.sessionID = "current"
		w.harness = currentPeerProcess(os.Getpid())
		w.verify = func(context.Context) peerProcess { return w.harness }
		return w
	}
	first := newWatcher()
	if err := first.arm(); err != nil {
		t.Fatal(err)
	}
	// Starting a newer Stop supersedes the old native hook. The new hook
	// observes a later database creation and addressed commit while idle.
	next := newWatcher()
	result := make(chan bool, 1)
	go func() { result <- next.wait(ctx) }()
	for {
		if done, notify := first.check(ctx); done && !notify {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("Stop failed to replace watcher")
		case <-time.After(10 * time.Millisecond):
		}
	}
	store, err := peermsg.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.Send(ctx, peermsg.SendRequest{SenderID: "sender", RecipientID: "other", Body: "other"}); err != nil {
		t.Fatal(err)
	}
	if done, notify := next.check(ctx); done || notify {
		t.Fatal("matched other recipient")
	}
	if _, err = store.Send(ctx, peermsg.SendRequest{SenderID: "sender", RecipientID: "recipient", Body: "private"}); err != nil {
		t.Fatal(err)
	}
	select {
	case notified := <-result:
		if !notified {
			t.Fatal("missing native wake")
		}
	case <-ctx.Done():
		t.Fatal("unattended wake timed out")
	}
	last := newWatcher()
	if err := last.arm(); err != nil {
		t.Fatal(err)
	}
	if done, notify := last.check(ctx); done || notify {
		t.Fatal("unacknowledged mail repeated at Stop")
	}
	last.sessionID = "restarted"
	if err := last.arm(); err != nil {
		t.Fatal(err)
	}
	if done, notify := last.check(ctx); !done || !notify {
		t.Fatal("new harness missed unacknowledged mail")
	}
}

func TestPeerWakeHookRejectsUnregisteredEmitter(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("AGENTDECK_INSTANCE_ID", "recipient")
	t.Setenv("AGENTDECK_TOOL", "claude")
	t.Setenv("AGENTDECK_PROFILE", "test")
	profile, err := session.GetProfileDir("test")
	if err != nil {
		t.Fatal(err)
	}
	store, err := peermsg.Open(profile)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.Send(context.Background(), peermsg.SendRequest{SenderID: "sender", RecipientID: "recipient", Body: "sensitive body"}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := handlePeerWakeHook(strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"current"}`), &out)
	if code != 0 || out.Len() != 0 {
		t.Fatalf("unexpected native hook result %d %q", code, out.String())
	}
}

func TestPeerWakeNativeRegistrationAndAncestry(t *testing.T) {
	dir := t.TempDir()
	pid := os.Getpid()
	parent := os.Getppid()
	source := peerClaudeSource{registrationDir: dir, sessionID: "root"}
	register := func(pid int, kind, id string) {
		t.Helper()
		b, err := json.Marshal(map[string]any{"pid": pid, "kind": kind, "sessionId": id})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, strconv.Itoa(pid)+".json"), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	register(pid, "interactive", "root")
	ctx := context.Background()
	if owner := source.harnessAncestor(ctx, pid, pid); owner != currentPeerProcess(pid) {
		t.Fatalf("missing live registered pane owner: %+v", owner)
	}
	if owner := source.harnessAncestor(ctx, pid, 1); owner.PID != 0 {
		t.Fatal("stale pane process gained ownership")
	}
	register(parent, "interactive", "outer")
	if owner := source.harnessAncestor(ctx, pid, parent); owner.PID != 0 {
		t.Fatal("nested interactive process gained ownership")
	}
	register(pid, "headless", "root")
	if owner := source.harnessAncestor(ctx, pid, pid); owner.PID != 0 {
		t.Fatal("headless process gained ownership")
	}
	register(pid, "interactive", "different-session")
	if owner := source.harnessAncestor(ctx, pid, pid); owner.PID != 0 {
		t.Fatal("late UUID gained ownership")
	}
}

func TestPeerWakeLockWaitHonorsDeadline(t *testing.T) {
	w := newPeerWake(t.TempDir(), "recipient")
	release, err := w.lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if release, err := w.lock(ctx); err == nil {
		release()
		t.Fatal("second lock acquired")
	}
}

func TestPeerWakeWithoutToolHintRequiresRegisteredClaudePane(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENT_DECK_HOME", root)
	t.Setenv("AGENTDECK_PROFILE", "_test_legacy_wake")
	t.Setenv("AGENTDECK_TOOL", "")
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, "claude"))
	store, err := session.NewStorageWithProfile("_test_legacy_wake")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner := session.NewInstance("legacy", root)
	owner.Tool = "claude"
	other := session.NewInstance("other", root)
	other.Tool = "codex"
	if err := store.Save([]*session.Instance{owner, other}); err != nil {
		t.Fatal(err)
	}
	if !hasClaudePeerWake(owner.ID) || hasClaudePeerWake(other.ID) || hasClaudePeerWake("missing") {
		t.Fatal("tool hint fallback must require a managed Claude row")
	}

	t.Setenv("AGENTDECK_INSTANCE_ID", owner.ID)
	sid := "e5e12033-1edc-44b9-905c-28af73165037"
	pid := os.Getppid()
	regDir := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "sessions")
	if err := os.MkdirAll(regDir, 0700); err != nil {
		t.Fatal(err)
	}
	reg, err := json.Marshal(map[string]any{"pid": pid, "sessionId": sid, "kind": "interactive"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(regDir, strconv.Itoa(pid)+".json"), reg, 0600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	// Report the fixture's native parent only for the expected pane target.
	target := "=" + owner.GetTmuxSession().Name + ":0.0"
	if err := os.WriteFile(filepath.Join(bin, "tmux"), []byte("#!/bin/sh\nfor a do [ \"$a\" != "+strconv.Quote(target)+" ] || { echo "+strconv.Itoa(pid)+"; exit 0; }; done\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	dir, err := session.GetProfileDir("_test_legacy_wake")
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := peermsg.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Close()
	if _, err := inbox.Send(context.Background(), peermsg.SendRequest{SenderID: "sender", RecipientID: owner.ID, Body: "private"}); err != nil {
		t.Fatal(err)
	}
	// Synchronous Stop hooks can survive an old or partial installation. A
	// nested Claude inherits the hint; its UUID must not wake the outer row.
	t.Setenv("AGENTDECK_TOOL", "claude")
	active := false
	stop := hookPayload{HookEventName: "Stop", StopHookActive: &active, SessionID: "other-native-session"}
	dec := session.StopHookDecision{Decision: "block", Reason: "child finished"}
	if got := withPeerStopNotice(dec, stop, owner.ID); got != dec {
		t.Fatalf("foreign Stop exposed inbox: %+v", got)
	}
	stop.SessionID = sid
	if got := withPeerStopNotice(dec, stop, owner.ID); got.Decision != "block" || got.Reason != dec.Reason+"\n"+pendingPeerNotice {
		t.Fatalf("verified Stop lost notice: %+v", got)
	}
	if got := withPeerStopNotice(dec, stop, owner.ID); got != dec {
		t.Fatalf("repeated Stop notice: %+v", got)
	}
	t.Setenv("AGENTDECK_TOOL", "")
	if _, err := inbox.Send(context.Background(), peermsg.SendRequest{SenderID: "sender", RecipientID: owner.ID, Body: "next"}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	code := handlePeerWakeHook(strings.NewReader(`{"hook_event_name":"SessionStart","session_id":"`+sid+`"}`), &output)
	if code != 2 || strings.TrimSpace(output.String()) != pendingPeerNotice {
		t.Fatalf("verified no-hint hook = %d %q", code, output.String())
	}
}
