package session

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/childenv"
)

// QueueCodexMessage submits an operator command through the native durable queue.
// Native queue acceptance is distinct from starting or completing a model turn.
func (i *Instance) QueueCodexMessage(ctx context.Context, body string) (DispatchReceipt, error) {
	return i.QueueCodexMessageIf(ctx, body, nil)
}

// QueueCodexMessageIf claims a wake after verifying its live receiver and before
// invoking the native queue. A declined claim returns an empty receipt.
func (i *Instance) QueueCodexMessageIf(ctx context.Context, body string, claim func() (bool, error)) (DispatchReceipt, error) {
	i.mu.RLock()
	id, confirmed, command := i.CodexSessionID, i.CodexBindingConfirmed, i.Command
	i.mu.RUnlock()
	if !confirmed || !codexSessionIDPattern.MatchString(id) {
		return DispatchReceipt{}, errors.New("Codex session binding must be confirmed before queueing")
	}
	probe := i.queryCodexSessionFromProcessFiles()
	if probe.ambiguous || probe.sessionID != id || !probe.queueOwnedBy(i.codexInteractiveProcessPID()) {
		return DispatchReceipt{}, errors.New("cannot verify the live Codex thread for this pane")
	}
	executable, ok := codexExecutableFromCommand(i.resolveCodexCommand(command))
	if !ok {
		return DispatchReceipt{}, errors.New("cannot resolve native Codex queue executable")
	}
	// #nosec G204 -- verified native executable; body is one argument, never shell input.
	cmd := exec.CommandContext(ctx, executable, "queue", "--thread", id, "--message="+body)
	cmd.Env = replaceEnv(childenv.ForLaunch(""), "CODEX_HOME", probe.queueHome)
	cmd.Env = replaceEnv(cmd.Env, "CODEX_SQLITE_HOME", probe.queueHome)
	if claim != nil {
		claimed, err := claim()
		if err != nil || !claimed {
			return DispatchReceipt{}, err
		}
	}
	submitted := time.Now()
	out, err := cmd.Output()
	if err != nil {
		return DispatchReceipt{}, fmt.Errorf("Codex queue acceptance unconfirmed (%w); check the recipient before retrying", err)
	}
	messageID, err := parseQueueReceipt(string(out), id)
	if err != nil {
		return DispatchReceipt{}, err
	}
	return DispatchReceipt{ID: messageID, SubmittedAt: submitted}, nil
}

// Queue ownership belongs to the interactive process, even if another Codex
// launcher in the pane opened a matching rollout and queue.
func (probe codexProcessProbe) queueOwnedBy(pid int) bool {
	if pid <= 0 || probe.queueHome == "" {
		return false
	}
	return slices.ContainsFunc(probe.rollouts, func(candidate codexRolloutCandidate) bool {
		return candidate.PID == pid && candidate.ID == probe.sessionID && candidate.queueHome == probe.queueHome &&
			readCodexRolloutThreadMetadata(candidate.Path).kind != codexThreadSubagent
	})
}

func parseQueueReceipt(output, thread string) (string, error) {
	words := strings.Fields(output)
	if len(words) == 6 && words[0] == "Queued" && words[1] == "message" && codexSessionIDPattern.MatchString(words[2]) && words[3] == "for" && words[4] == "thread" && words[5] == thread+"." {
		return words[2], nil
	}
	return "", errors.New("Codex queue receipt did not identify a message for the requested thread; check the recipient before retrying")
}
