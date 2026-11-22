package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCodexQueueRejectsSiblingsWritableQueue(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires Linux writable descriptor inspection")
	}
	skipIfNoTmuxBinary(t)
	cat, err := exec.LookPath("gnucat")
	if err != nil {
		cat, err = exec.LookPath("cat")
	}
	if err != nil {
		t.Skip("requires a native process blocked on a FIFO")
	}
	inst, home := newCodexGateInstance(t)
	id := uniqueSID(t)
	inst.CodexSessionID, inst.CodexBindingConfirmed = id, true
	rollout := writeCodexReviewRolloutAt(t, home, id, inst.ProjectPath, time.Now())
	if output, err := exec.Command("mkfifo", filepath.Join(inst.ProjectPath, "resume")).CombinedOutput(); err != nil {
		t.Fatalf("create private FIFO: %s: %v", output, err)
	}
	native := filepath.Join(t.TempDir(), "codex")
	if err := os.Symlink(cat, native); err != nil {
		t.Fatal(err)
	}
	holder := filepath.Join(t.TempDir(), "codex")
	script := "#!/bin/sh\nexec 3>>" + shellQuote(rollout) + "\nexec 4>>" + shellQuote(filepath.Join(home, "queue_1.sqlite")) + "\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(holder, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	inst.tmuxSession.RunCommandAsInitialProcess = true
	command := "cd " + shellQuote(inst.ProjectPath) + " && { " + shellQuote(holder) + " resume " + id + " & " + shellQuote(native) + " resume " + id + " & wait; }"
	if err := inst.tmuxSession.Start(command); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inst.tmuxSession.Kill() })
	deadline := time.Now().Add(3 * time.Second)
	for {
		args := codexProcessArguments(inst.codexInteractiveProcessPID())
		probe := inst.queryCodexSessionFromProcessFiles()
		if len(args) > 0 && args[0] == native && probe.sessionID == id && probe.queueHome == home {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("missing sibling-process fixture: selected argv %q, evidence %+v", args, probe)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// This queue executable records any dispatch, including one whose output
	// the caller subsequently decides to reject.
	marker := filepath.Join(t.TempDir(), "dispatched")
	inst.Command = filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(inst.Command, []byte("#!/bin/sh\ntouch "+shellQuote(marker)+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	_, err = inst.QueueCodexMessage(context.Background(), "must remain undelivered")
	if err == nil || !strings.Contains(err.Error(), "cannot verify") {
		t.Errorf("expected pre-dispatch ownership refusal: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("invoked the native queue for a sibling process: %v", err)
	}
}
