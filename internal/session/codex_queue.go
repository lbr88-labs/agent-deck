package session

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/asheshgoplani/agent-deck/internal/childenv"
)

// QueueCodexMessage submits an operator command through the native durable queue.
// Native queue acceptance is distinct from starting or completing a model turn.
func (i *Instance) QueueCodexMessage(ctx context.Context, body string) (string, error) {
	i.mu.RLock()
	id, confirmed, command := i.CodexSessionID, i.CodexBindingConfirmed, i.Command
	i.mu.RUnlock()
	if !confirmed || !codexSessionIDPattern.MatchString(id) {
		return "", errors.New("Codex session binding must be confirmed before queueing")
	}
	probe := i.queryCodexSessionFromProcessFiles()
	if probe.ambiguous || probe.sessionID != id || probe.queueHome == "" {
		return "", errors.New("cannot verify the live Codex thread for this pane")
	}
	executable, ok := codexExecutableFromCommand(i.resolveCodexCommand(command))
	if !ok {
		return "", errors.New("cannot resolve native Codex queue executable")
	}
	// #nosec G204 -- verified native executable; body is one argument, never shell input.
	cmd := exec.CommandContext(ctx, executable, "queue", "--thread", id, "--message="+body)
	cmd.Env = replaceEnv(childenv.ForLaunch(""), "CODEX_HOME", probe.queueHome)
	cmd.Env = replaceEnv(cmd.Env, "CODEX_SQLITE_HOME", probe.queueHome)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Codex queue acceptance unconfirmed (%w); check the recipient before retrying", err)
	}
	return parseQueueReceipt(string(out), id)
}

func parseQueueReceipt(output, thread string) (string, error) {
	words := strings.Fields(output)
	if len(words) == 6 && words[0] == "Queued" && words[1] == "message" && codexSessionIDPattern.MatchString(words[2]) && words[3] == "for" && words[4] == "thread" && words[5] == thread+"." {
		return words[2], nil
	}
	return "", errors.New("Codex queue receipt did not identify a message for the requested thread; check the recipient before retrying")
}
