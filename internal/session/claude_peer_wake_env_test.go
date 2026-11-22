package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeLaunchExportsPeerHookIdentity(t *testing.T) {
	bin := t.TempDir()
	for _, launcher := range []string{"claude", "work-claude"} {
		err := os.WriteFile(filepath.Join(bin, launcher), []byte("#!/bin/sh\nprintf '%s\\n' \"$AGENTDECK_TOOL\" \"$AGENTDECK_INSTANCE_ID\" \"$@\"\n"), 0700)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, launcher := range []string{"claude", "work-claude"} {
		t.Run(launcher, func(t *testing.T) {
			cfg := &UserConfig{Claude: ClaudeSettings{Command: launcher}}
			defer resetUserConfigCache(t, cfg)()
			root := t.TempDir()
			t.Setenv("CLAUDE_CONFIG_DIR", root)
			inst := &Instance{ID: "peer-env-recipient", Tool: "claude", ProjectPath: t.TempDir(), ClaudeSessionID: "0d9cfc43-bc70-44ef-9d1e-be18cbb8fe49"}
			sid := inst.ClaudeSessionID
			writeClaudeImportTranscript(t, root, inst.ProjectPath, sid, `{"type":"user","message":{"role":"user","content":"hello"}}`)
			for _, kind := range []string{"fresh", "resume", "custom command"} {
				var command string
				switch kind {
				case "fresh":
					command = inst.buildClaudeCommandWithMessage("claude", "")
				case "resume":
					inst.ClaudeSessionID = sid
					command = inst.buildClaudeResumeCommand()
				case "custom command":
					command = inst.buildClaudeCommandWithMessage("work-claude --version", "")
				}
				cmd := exec.Command("bash", "-c", command)
				cmd.Env = []string{"PATH=" + bin + ":" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("%s: %v: %s", kind, err, out)
				}
				if !strings.HasPrefix(string(out), "claude\n"+inst.ID+"\n") {
					t.Fatalf("%s: missing exported identity in child: %q", kind, out)
				}
				if kind == "resume" && !strings.Contains(string(out), "--resume\n"+sid+"\n") {
					t.Fatalf("expected native resume, got %q", out)
				}
			}
		})
	}
}
