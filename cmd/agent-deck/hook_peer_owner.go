package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/session"
	"github.com/asheshgoplani/agent-deck/internal/tmux"
)

// PID alone can be reused. Birth identifies the kernel process lifetime.
type peerProcess struct {
	PID   int
	Birth string
}

func currentPeerProcess(pid int) peerProcess {
	return peerProcess{pid, session.CodexProcessIdentity(pid)}
}

func (p peerProcess) live() bool {
	return p.PID > 1 && p.Birth != "" && currentPeerProcess(p.PID) == p
}

type peerClaudeSource struct {
	socket, target, registrationDir, sessionID string
}

func peerWakeSource(profile, instance, sid string) (peerClaudeSource, error) {
	store, err := session.NewStorageWithProfile(profile)
	if err != nil {
		return peerClaudeSource{}, err
	}
	defer store.Close()
	row, err := store.GetDB().LoadInstanceByID(instance)
	if err != nil {
		return peerClaudeSource{}, err
	}
	if row == nil || !session.IsClaudeCompatible(row.Tool) || row.TmuxSession == "" {
		return peerClaudeSource{}, errors.New("no managed Claude pane")
	}
	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if configDir == "" {
		configDir = getClaudeConfigDirForHooks()
	}
	return peerClaudeSource{row.TmuxSocketName, "=" + row.TmuxSession + ":0.0", filepath.Join(configDir, "sessions"), sid}, nil
}

func (s peerClaudeSource) harness(ctx context.Context) peerProcess {
	// Read the current root of this specific row's primary pane. A respawn
	// changes it even when the new Claude resumes the same session UUID.
	cmd := tmux.ExecContext(ctx, s.socket, "display-message", "-p", "-t", s.target, "#{pane_pid}")
	cmd.WaitDelay = 100 * time.Millisecond
	out, err := cmd.Output()
	if err != nil {
		return peerProcess{}
	}
	root, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || root <= 1 {
		return peerProcess{}
	}
	return s.harnessAncestor(ctx, os.Getppid(), root)
}

func peerParent(ctx context.Context, pid int) (int, string) {
	if runtime.GOOS == "linux" {
		b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
		if err != nil {
			return 0, ""
		}
		line := string(b)
		left, right := strings.IndexByte(line, '('), strings.LastIndexByte(line, ')')
		if left < 0 || right <= left {
			return 0, ""
		}
		fields := strings.Fields(line[right+1:])
		if len(fields) < 2 {
			return 0, ""
		}
		parent, _ := strconv.Atoi(fields[1])
		return parent, line[left+1 : right]
	}
	// #nosec G204 -- Fixed ps executable and options; the PID is a decimal integer argument.
	out, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "ppid=,comm=").Output()
	fields := strings.Fields(string(out))
	if err != nil || len(fields) < 2 {
		return 0, ""
	}
	parent, _ := strconv.Atoi(fields[0])
	return parent, filepath.Base(strings.Join(fields[1:], " "))
}

// The session row can legitimately lack a Claude UUID at startup. Claude's
// own per-PID interactive registration binds the event UUID to the emitter;
// the ancestry walk binds that emitter to the live recipient pane. Only a
// shell trampoline can sit between the hook command and its native parent.
func (s peerClaudeSource) harnessAncestor(ctx context.Context, pid, root int) peerProcess {
	var found peerProcess
	for range 30 {
		if pid <= 1 || ctx.Err() != nil {
			break
		}
		parent, comm := peerParent(ctx, pid)
		if parent <= 0 {
			break
		}
		data, err := os.ReadFile(filepath.Join(s.registrationDir, strconv.Itoa(pid)+".json"))
		if err == nil {
			var reg struct {
				PID                        int
				SessionID, Kind, ProcStart string
			}
			if found.PID != 0 || json.Unmarshal(data, &reg) != nil || reg.PID != pid || reg.SessionID != s.sessionID || reg.Kind != "interactive" {
				return peerProcess{}
			}
			found = currentPeerProcess(pid)
			if found.Birth == "" || (runtime.GOOS == "linux" && reg.ProcStart != "" && !strings.HasSuffix(found.Birth, ":"+reg.ProcStart)) {
				return peerProcess{}
			}
		} else if found.PID == 0 {
			switch comm {
			case "sh", "bash", "dash", "zsh", "fish":
			default:
				return peerProcess{}
			}
		} else if comm == "claude" {
			return peerProcess{}
		}
		if pid == root {
			return found
		}
		pid = parent
	}
	return peerProcess{}
}
