package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// sshControlDir is the directory for SSH ControlMaster sockets.
const sshControlDir = "/tmp/agent-deck-ssh"

// SSHRunner executes commands on a remote host via SSH.
type SSHRunner struct {
	Host          string // SSH destination (e.g., "user@host")
	AgentDeckPath string // Remote agent-deck binary path
	Profile       string // Remote profile name
}

// NewSSHRunner creates an SSHRunner from a RemoteConfig.
func NewSSHRunner(name string, rc RemoteConfig) *SSHRunner {
	return &SSHRunner{
		Host:          rc.Host,
		AgentDeckPath: rc.GetAgentDeckPath(),
		Profile:       rc.GetProfile(),
	}
}

// Run executes an agent-deck command on the remote host and returns stdout.
func (r *SSHRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return r.run(timeoutCtx, args...)
}

// run executes an agent-deck command on the remote host using the provided context directly.
func (r *SSHRunner) run(ctx context.Context, args ...string) ([]byte, error) {
	_ = os.MkdirAll(sshControlDir, 0700)

	remoteCmd := r.buildRemoteCommand(args...)

	sshArgs := []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + sshControlDir + "/%r@%h:%p",
		"-o", "ControlPersist=600",
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
		r.Host,
		remoteCmd,
	}

	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ssh command failed: %w: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// Attach connects interactively to a remote agent-deck session.
// This connects stdin/stdout/stderr for full terminal interaction.
func (r *SSHRunner) Attach(sessionID string) error {
	_ = os.MkdirAll(sshControlDir, 0700)

	remoteCmd := r.buildRemoteCommand("session", "attach", sessionID)

	sshArgs := []string{
		"-t",
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + sshControlDir + "/%r@%h:%p",
		"-o", "ControlPersist=600",
		r.Host,
		remoteCmd,
	}

	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RunCommand executes an arbitrary agent-deck command on the remote.
func (r *SSHRunner) RunCommand(ctx context.Context, args ...string) ([]byte, error) {
	return r.Run(ctx, args...)
}

// buildRemoteCommand safely quotes each argument for execution through the remote shell.
func (r *SSHRunner) buildRemoteCommand(args ...string) string {
	parts := []string{shellQuote(r.AgentDeckPath)}
	if r.Profile != "" && r.Profile != "default" {
		parts = append(parts, "-p", shellQuote(r.Profile))
	}
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

// FetchSessions retrieves the session list from the remote agent-deck instance.
func (r *SSHRunner) FetchSessions(ctx context.Context) ([]RemoteSessionInfo, error) {
	output, err := r.Run(ctx, "list", "--json")
	if err != nil {
		return nil, err
	}

	// Handle empty/non-JSON output (e.g., "No sessions found" message)
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, nil
	}

	var sessions []RemoteSessionInfo
	if err := json.Unmarshal(trimmed, &sessions); err != nil {
		return nil, fmt.Errorf("failed to parse remote sessions: %w", err)
	}

	return sessions, nil
}

// CreateSession creates and starts a new session on the remote, returning its ID.
// It runs "add --quick --json" to create the session, then "session start" to
// launch the tmux process, so the session is ready to attach.
func (r *SSHRunner) CreateSession(ctx context.Context) (string, error) {
	// Step 1: Create the session
	output, err := r.Run(ctx, "add", "--quick", "--json")
	if err != nil {
		return "", fmt.Errorf("failed to create remote session: %w", err)
	}

	var result struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("failed to parse remote add output: %w", err)
	}
	if result.ID == "" {
		return "", fmt.Errorf("remote add returned empty session ID")
	}

	// Step 2: Start the session so it has a tmux process to attach to.
	// Use title for resolution since it's more reliable across the SSH boundary.
	startCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if _, err := r.run(startCtx, "session", "start", result.Title); err != nil {
		return "", fmt.Errorf("failed to start remote session: %w", err)
	}

	return result.ID, nil
}

// RemoteSessionInfo represents a session from a remote agent-deck instance.
type RemoteSessionInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Path      string `json:"path"`
	Group     string `json:"group"`
	Tool      string `json:"tool"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`

	// Set locally, not from JSON
	RemoteName string `json:"-"`
}
