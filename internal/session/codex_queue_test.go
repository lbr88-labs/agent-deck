package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeQueueReceipt(t *testing.T) {
	const id = "01986884-2ff9-7163-bb30-dd6f1404c2ae"
	const msg = "01986884-2ff9-7163-bb30-dd6f1404c2af"
	receipt := "Queued message " + msg + " for thread " + id + "."
	if got, err := parseQueueReceipt(receipt, id); err != nil || got != msg {
		t.Fatal(got, err)
	}
	for _, out := range []string{"", "message pasted", receipt + " extra", "Queued message " + msg + " for thread " + msg + "."} {
		if _, err := parseQueueReceipt(out, id); err == nil {
			t.Fatal("accepted", out)
		}
	}
}

func TestCodexQueueRequiresMatchingLiveWriterBeforeCallingNativeCLI(t *testing.T) {
	inst, home := newCodexGateInstance(t)
	actual, other, receipt := uniqueSID(t), uniqueSID(t), uniqueSID(t)
	path := writeCodexReviewRolloutAt(t, home, actual, inst.ProjectPath, time.Now())
	startCodexReviewPaneWithQueue(t, inst, filepath.Join(home, "queue_1.sqlite"), path)
	dir := t.TempDir()
	log := filepath.Join(dir, "body")
	exe := filepath.Join(dir, "codex")
	script := "#!/bin/sh\nset -eu\n" +
		"[ \"$CODEX_HOME\" = " + shellQuote(home) + " ] && [ \"$CODEX_SQLITE_HOME\" = \"$CODEX_HOME\" ] || exit 3\n" +
		"[ \"$1\" = queue ] && [ \"$2\" = --thread ] && [ \"$3\" = " + shellQuote(actual) + " ] || exit 2\n" +
		"printf %s \"$4\" > " + shellQuote(log) + "\n" +
		"printf 'Queued message %s for thread %s.\\n' " + shellQuote(receipt) + " \"$3\"\n"
	if err := os.WriteFile(exe, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	inst.Command = exe
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("CODEX_SQLITE_HOME", t.TempDir())
	for _, binding := range []struct {
		id        string
		confirmed bool
	}{{actual, false}, {other, true}} {
		inst.CodexSessionID, inst.CodexBindingConfirmed = binding.id, binding.confirmed
		if _, err := inst.QueueCodexMessage(context.Background(), "must fail"); err == nil {
			t.Fatal("accepted unverified binding")
		}
		if _, err := os.Stat(log); !os.IsNotExist(err) {
			t.Fatalf("native CLI ran for unverified binding: %v", err)
		}
	}
	inst.CodexSessionID, inst.CodexBindingConfirmed = actual, true
	body := "-option-like first line\n" + strings.Repeat("a long line\n", 500)
	got, err := inst.QueueCodexMessage(context.Background(), body)
	if err != nil || got.ID != receipt || got.SubmittedAt.IsZero() {
		t.Fatalf("got receipt %+v, error %v", got, err)
	}
	arg, err := os.ReadFile(log)
	if err != nil || string(arg) != "--message="+body {
		t.Fatalf("message argument changed: %v", err)
	}
}

func TestNativeQueueRejectsSeparateRolloutAndQueueRoots(t *testing.T) {
	inst, home := newCodexGateInstance(t)
	id := uniqueSID(t)
	path := writeCodexReviewRolloutAt(t, home, id, inst.ProjectPath, time.Now())
	startCodexReviewPaneWithQueue(t, inst, filepath.Join(t.TempDir(), "queue_1.sqlite"), path)
	inst.CodexSessionID, inst.CodexBindingConfirmed = id, true
	if _, err := inst.QueueCodexMessage(context.Background(), "foreign home"); err == nil {
		t.Fatal("accepted rollout outside the live queue home")
	}
}
