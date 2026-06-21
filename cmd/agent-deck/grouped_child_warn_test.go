package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

// captureStderr runs fn with os.Stderr redirected to a pipe and returns what
// was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			b.WriteString(sc.Text())
			b.WriteByte('\n')
		}
		done <- b.String()
	}()
	fn()
	_ = w.Close()
	os.Stderr = orig
	return <-done
}

// TestWarnGroupAccountMismatch covers the launch-time warn-guard: when a
// grouped child will resolve to a config_dir that differs from its group's
// configured one (e.g. an explicit account override diverts it), the operator
// is warned before the wrong account's quota is burned. The healthy case
// (child follows its group) stays silent.
func TestWarnGroupAccountMismatch(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("AGENTDECK_PROFILE", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Cleanup(session.ClearUserConfigCache)

	agentDeckDir := filepath.Join(tmpHome, ".agent-deck")
	if err := os.MkdirAll(agentDeckDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := `
[profiles.work.claude]
config_dir = "~/.claude-work"

[profiles.buddii.claude]
config_dir = "~/.claude-buddii"

[groups.ryan.claude]
config_dir = "~/.claude-buddii"
`
	if err := os.WriteFile(filepath.Join(agentDeckDir, "config.toml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	session.ClearUserConfigCache()

	// Healthy: grouped child, no override -> resolves to group's buddii -> silent.
	healthy := session.NewInstanceWithGroupAndTool("ok-child", filepath.Join(tmpHome, "p"), "ryan", "claude")
	if out := captureStderr(t, func() { warnGroupAccountMismatch(healthy) }); strings.TrimSpace(out) != "" {
		t.Errorf("expected no warning for a child that follows its group, got: %q", out)
	}

	// Mismatch: explicit account "work" diverts the ryan child off buddii -> warn.
	mismatch := session.NewInstanceWithGroupAndTool("bad-child", filepath.Join(tmpHome, "p"), "ryan", "claude")
	mismatch.Account = "work"
	out := captureStderr(t, func() { warnGroupAccountMismatch(mismatch) })
	if !strings.Contains(out, "wrong account") || !strings.Contains(out, "ryan") {
		t.Errorf("expected wrong-account warning naming the group, got: %q", out)
	}
	if !strings.Contains(out, filepath.Join(tmpHome, ".claude-work")) {
		t.Errorf("warning should report the resolved (work) dir, got: %q", out)
	}
}
