package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/atomicfile"
	"github.com/asheshgoplani/agent-deck/internal/peermsg"
	"github.com/asheshgoplani/agent-deck/internal/session"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

// Each native hook is tied to the Claude process that started it. A new Stop
// replaces the previous watcher to renew Claude's own background-hook timeout.
type peerWakeState struct {
	Owner     string
	Notified  int64
	Harness   peerProcess
	Watcher   peerProcess
	SessionID string
}

type peerWake struct {
	dir, instance, path, owner string
	sessionID                  string
	verify                     func(context.Context) peerProcess
	harness                    peerProcess
}

func newPeerWake(dir, instance string) peerWake {
	return peerWake{dir: dir, instance: instance, path: filepath.Join(dir, ".peer-wake-"+instance+".json"), owner: uuid.NewString()}
}

func (w peerWake) state() (peerWakeState, error) {
	var state peerWakeState
	b, err := os.ReadFile(w.path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(b, &state)
	return state, err
}

func (w peerWake) save(state peerWakeState) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(w.path, b, 0600)
}

// Flock serializes the short cursor update across hook processes. Use a
// dedicated, retained lock file and bound acquisition even if a holder stalls.
func (w peerWake) lock(ctx context.Context) (func(), error) {
	file, err := os.OpenFile(w.path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { _ = file.Close() }, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			_ = file.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w peerWake) arm() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	release, err := w.lock(ctx)
	if err != nil {
		return err
	}
	defer release()
	state, err := w.state()
	if err != nil {
		return err
	}
	if w.verify == nil || !w.harness.live() || w.verify(ctx) != w.harness {
		return errors.New("Claude hook no longer owns recipient pane")
	}
	// A new process (including --resume) must rediscover unacknowledged
	// mail if its predecessor crashed between notice and acknowledgement.
	if state.Harness != w.harness || state.SessionID != w.sessionID {
		state.Notified = 0
	}
	state.Owner = w.owner
	state.Harness = w.harness
	state.SessionID = w.sessionID
	state.Watcher = currentPeerProcess(os.Getpid())
	return w.save(state)
}

// check returns done when replaced or when this native hook has a notice to
// deliver. Errors leave the watcher armed for a later filesystem event.
func (w peerWake) check(ctx context.Context) (done, notify bool) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	release, err := w.lock(ctx)
	if err != nil {
		return false, false
	}
	defer release()
	state, err := w.state()
	if err != nil {
		return false, false
	}
	if state.Owner != w.owner || !w.harness.live() {
		return true, false
	}
	seq, err := peermsg.PendingSequence(ctx, w.dir, w.instance)
	if err != nil || seq <= state.Notified {
		return false, false
	}
	if w.verify == nil || w.verify(ctx) != w.harness {
		return false, false
	}
	state.Notified = seq
	if w.save(state) != nil {
		return false, false
	}
	return true, true
}

func (w peerWake) wait(ctx context.Context) bool {
	if err := os.MkdirAll(w.dir, 0700); err != nil {
		return false
	}
	var events <-chan fsnotify.Event
	var failures <-chan error
	if watcher, err := fsnotify.NewWatcher(); err == nil {
		defer watcher.Close()
		if watcher.Add(w.dir) == nil {
			events = watcher.Events
			failures = watcher.Errors
		}
	}
	if w.arm() != nil {
		return false
	}
	// Cover storage without working fsnotify and lost filesystem events.
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	parent := os.Getppid()
	if done, notify := w.check(ctx); done {
		return notify
	}
	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if os.Getppid() != parent {
				return false
			}
		case _, ok := <-failures:
			if !ok {
				failures = nil
			}
		case e, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			base := filepath.Base(e.Name)
			if e.Name != w.path && base != peermsg.DatabaseName && base != peermsg.DatabaseName+"-wal" && base != peermsg.DatabaseName+"-journal" {
				continue
			}
		}
		if done, notify := w.check(ctx); done {
			return notify
		}
	}
}

func handlePeerWakeHook(input io.Reader, output io.Writer) int {
	instance := os.Getenv("AGENTDECK_INSTANCE_ID")
	// Older managed Claude launches exported the instance and profile only.
	// The stored row and native pane/UUID/process proof below remain mandatory.
	tool := os.Getenv("AGENTDECK_TOOL")
	if !validInstanceID.MatchString(instance) || strings.Contains(instance, "..") || (tool != "" && !session.IsClaudeCompatible(tool)) {
		return 0
	}
	var p hookPayload
	if json.NewDecoder(io.LimitReader(input, maxHookPayloadSize)).Decode(&p) != nil || p.SessionID == "" || (p.HookEventName != "SessionStart" && p.HookEventName != "Stop") {
		return 0
	}
	profile, err := session.ResolveProfileForStorage("")
	if err != nil {
		return 0
	}
	dir, err := session.GetProfileDir(profile)
	if err != nil {
		return 0
	}

	w := newPeerWake(dir, instance)
	// On a cold start Agent Deck can still be saving the canonical pane name
	// while Claude publishes its per-PID registration. Wait for both pieces of
	// evidence in the background without accepting the inherited ID alone.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	for w.harness.PID == 0 {
		source, err := peerWakeSource(profile, instance, p.SessionID)
		if err == nil {
			w.harness = source.harness(ctx)
			w.verify = source.harness
			w.sessionID = p.SessionID
		}
		if w.harness.PID != 0 {
			break
		}
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(400 * time.Millisecond):
		}
	}
	if w.wait(context.Background()) {
		fmt.Fprintln(output, pendingPeerNotice)
		return 2
	}
	return 0
}
