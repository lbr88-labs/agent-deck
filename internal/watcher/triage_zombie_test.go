package watcher

// Zombie-reap regression for issue #677 — AgentDeckLaunchSpawner.
//
// Before v1.7.43, Spawn() did `cmd.Start()` + return, leaving the child
// triage `agent-deck launch` process unreaped. Under sustained watcher
// traffic this produced a slow zombie leak. Fix: a reaper goroutine Waits
// on the child after Start returns.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func countZombieChildrenOfPid(t *testing.T, ppid int) int {
	t.Helper()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Skipf("cannot read /proc (non-Linux?): %v", err)
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var pid int
		if _, err := fmt.Sscanf(e.Name(), "%d", &pid); err != nil {
			continue
		}
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
		if err != nil {
			continue
		}
		var (
			parent int
			zombie bool
		)
		for _, line := range bytes.Split(data, []byte{'\n'}) {
			if bytes.HasPrefix(line, []byte("PPid:")) {
				_, _ = fmt.Sscanf(string(line), "PPid:\t%d", &parent)
			} else if bytes.HasPrefix(line, []byte("State:")) && bytes.Contains(line, []byte("zombie")) {
				zombie = true
			}
		}
		if zombie && parent == ppid {
			count++
		}
	}
	return count
}

// TestAgentDeckLaunchSpawner_NoZombie spawns many triage children via a
// stub agent-deck binary that exits immediately. Each Spawn call must be
// followed by automatic reaping; otherwise zombies accumulate one per call.
func TestAgentDeckLaunchSpawner_NoZombie(t *testing.T) {
	if _, err := os.Stat("/proc"); err != nil {
		t.Skip("/proc unavailable — non-Linux")
	}

	tmpDir := t.TempDir()

	// Stub agent-deck that exits 0 immediately — stands in for the real
	// `agent-deck launch` so we can test the reap path without booting a
	// full Claude session.
	stub := filepath.Join(tmpDir, "agent-deck")
	script := "#!/bin/sh\nexit 0\n"
	require.NoError(t, os.WriteFile(stub, []byte(script), 0o755))

	spawner := AgentDeckLaunchSpawner{BinaryPath: stub}

	const count = 25
	baseline := countZombieChildrenOfPid(t, os.Getpid())

	for i := 0; i < count; i++ {
		req := TriageRequest{
			Event:      Event{Sender: "test", Subject: fmt.Sprintf("evt-%d", i)},
			TriageDir:  filepath.Join(tmpDir, fmt.Sprintf("triage-%d", i)),
			ResultPath: filepath.Join(tmpDir, fmt.Sprintf("triage-%d/result.json", i)),
		}
		_, err := spawner.Spawn(context.Background(), req)
		require.NoError(t, err, "spawn %d", i)
	}

	// Reaper goroutines run async; wait for them to drain.
	deadline := time.Now().Add(3 * time.Second)
	var got int
	for time.Now().Before(deadline) {
		got = countZombieChildrenOfPid(t, os.Getpid()) - baseline
		if got <= 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.LessOrEqual(t, got, 0, "zombie children grew by %d after %d spawns (baseline=%d)", got, count, baseline)
}
