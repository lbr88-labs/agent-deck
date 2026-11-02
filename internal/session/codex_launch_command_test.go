package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"al.essio.dev/pkg/shellescape"
)

func TestIsSupportedCodexLaunchCommandAllowsOnlyBareExecutable(t *testing.T) {
	valid := []string{
		"codex",
		"codex-nightly",
		"codex_custom",
		"CODEX_HOME=/tmp/codex codex",
		`CODEX_HOME="/tmp/codex home" EXTRA_FLAG=1 codex-nightly`,
	}
	for _, command := range valid {
		t.Run("valid_"+command, func(t *testing.T) {
			if !IsSupportedCodexLaunchCommand(command) {
				t.Fatalf("IsSupportedCodexLaunchCommand(%q) = false, want true", command)
			}
		})
	}

	invalid := []string{
		"codex exec",
		"codex && echo",
		"codex extra",
		"codex-nightly resume",
		"CODEX_HOME=/tmp/codex codex --model gpt-5",
	}
	for _, command := range invalid {
		t.Run("invalid_"+command, func(t *testing.T) {
			if IsSupportedCodexLaunchCommand(command) {
				t.Fatalf("IsSupportedCodexLaunchCommand(%q) = true, want false", command)
			}
		})
	}
}

// Help never enters the interactive fixture or writes an argv capture. The
// probe log also exposes whether inline launch environment reached --help.
func codexIsolationClient(t *testing.T, help string) (binDir, logPath, probePath string) {
	t.Helper()
	binDir = filepath.Join(t.TempDir(), "native client")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	logPath = filepath.Join(t.TempDir(), "argv")
	probePath = filepath.Join(t.TempDir(), "help-probes")
	rejectUnsupportedFlag := ""
	if !strings.Contains(help, "Options:\n  --no-daemon") {
		rejectUnsupportedFlag = "if [ \"${1-}\" = --no-daemon ]; then exit 64; fi\n"
	}
	script := "#!/bin/sh\n" +
		"if [ \"${1-}\" = --help ]; then\n" +
		"  printf '%s\\n' \"${CODEX_TEST_SCOPE-}\" >> " + shellescape.Quote(probePath) + "\n" +
		"  printf '%s\\n' " + shellescape.Quote(help) + "\n" +
		"  exit 0\n" +
		"fi\n" +
		rejectUnsupportedFlag +
		"printf '%s\\n' \"$@\" > \"$CLAUDE_SHIM_LOG\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return binDir, logPath, probePath
}

func TestBuildCodexCommand_NativeIsolationArgv(t *testing.T) {
	const sid = "11111111-2222-4333-8444-555555555555"
	for _, client := range []struct {
		name string
		help string
		flag string
	}{
		{"supported", "Options:\n  --no-daemon  Bypass an existing daemon\n", "--no-daemon"},
		{"older", "Options:\n  --model <MODEL>\n", ""},
		{"prose_is_not_an_option", "This client does not support --no-daemon.\n", ""},
	} {
		for _, mode := range []string{
			"fresh", "fresh_prompt", "configured_absolute", "configured_environment",
			"explicit_absolute", "resume", "explicit_resume", "configured_resume",
		} {
			t.Run(client.name+"/"+mode, func(t *testing.T) {
				home := t.TempDir()
				t.Setenv("HOME", home)
				isolateConfigHomeXDG(t)
				t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
				binDir, logPath, probePath := codexIsolationClient(t, client.help)
				t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
				binary := shellescape.Quote(filepath.Join(binDir, "codex"))
				cfg := &UserConfig{Codex: CodexSettings{YoloMode: true}}
				inst := &Instance{ID: "codex-isolation", Title: "native", Tool: "codex", Command: "codex", ProjectPath: home}
				if err := inst.SetCodexOptions(&CodexOptions{Model: "gpt-5", ReasoningEffort: "high"}); err != nil {
					t.Fatal(err)
				}
				want := []string{"--yolo", "--model", "gpt-5", "--config", "model_reasoning_effort=high"}
				probeScope := ""
				switch mode {
				case "configured_absolute":
					cfg.Codex.Command = binary
				case "configured_environment":
					cfg.Codex.Command = "CODEX_TEST_SCOPE='work profile' CODEX_HOME=" +
						shellescape.Quote(filepath.Join(home, "codex work")) + " " + binary
					probeScope = "work profile"
				case "explicit_absolute":
					inst.Command = binary
					want = nil // Explicit commands retain their existing option passthrough.
				case "resume", "explicit_resume", "configured_resume":
					inst.CodexSessionID = sid
					inst.CodexBindingConfirmed = true
					seedCodexRollout(t, os.Getenv("CODEX_HOME"), sid)
					switch mode {
					case "resume":
						cfg.Codex.Command = binary
					case "explicit_resume":
						inst.Command = binary + " resume " + sid
						want = nil
					case "configured_resume":
						cfg.Codex.Command = binary + " resume " + sid
						want = nil
					}
					want = append(want, "resume", sid)
				}
				if client.flag != "" {
					want = append([]string{client.flag}, want...)
				}
				t.Cleanup(resetUserConfigCache(t, cfg))
				command := ""
				if mode == "fresh_prompt" {
					var embedded bool
					command, embedded = inst.buildCodexCommandWithPrompt(inst.Command, "carry the context")
					if !embedded {
						t.Fatal("fresh prompt was not embedded")
					}
					want = append(want, "carry the context")
				} else {
					command = inst.buildCodexCommand(inst.Command)
				}
				got := runBuiltCommandCapturingArgv(t, command, binDir, logPath)
				if got != strings.Join(want, "\n") {
					t.Fatalf("native argv = %q, want %q; command: %s", got, strings.Join(want, "\n"), command)
				}
				probes, err := os.ReadFile(probePath)
				if err != nil || string(probes) != probeScope+"\n" {
					t.Fatalf("help probe environment = %q, err %v; want one probe with scope %q", probes, err, probeScope)
				}
			})
		}
	}
}

func TestBuildCodexCommand_NativeIsolationPreservesOpaqueCommands(t *testing.T) {
	for _, mode := range []string{"options", "already_isolated", "exec", "shell_wrapper", "ssh", "sandbox"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			isolateConfigHomeXDG(t)
			t.Setenv("CODEX_HOME", t.TempDir())
			t.Cleanup(resetUserConfigCache(t, &UserConfig{Codex: CodexSettings{YoloMode: true}}))
			binDir, logPath, probePath := codexIsolationClient(t, "Options:\n  --no-daemon\n")
			command := shellescape.Quote(filepath.Join(binDir, "codex"))
			var want []string
			switch mode {
			case "options":
				command += ` --profile work --config 'notify=["user-hook","--keep"]'`
				want = []string{"--profile", "work", "--config", `notify=["user-hook","--keep"]`}
			case "already_isolated":
				command += " --no-daemon"
				want = []string{"--no-daemon"}
			case "exec":
				command += " exec 'one task'"
				want = []string{"exec", "one task"}
			case "shell_wrapper":
				command = "/bin/sh -c " + shellescape.Quote("exec "+command)
			case "ssh":
				command = "CODEX_TEST_SCOPE=remote codex"
				t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			inst := &Instance{ID: "codex-opaque", Title: "opaque", Tool: "codex", Command: command}
			switch mode {
			case "ssh":
				inst.SSHHost = "unprobed-remote"
			case "sandbox":
				inst.Sandbox = NewSandboxConfig("")
			}
			built := inst.buildCodexCommand(command)
			if mode == "ssh" && (!strings.HasSuffix(built, " "+command) ||
				strings.Contains(built, binDir) || strings.Contains(built, "cd ") || strings.Contains(built, "--no-daemon")) {
				t.Fatalf("remote native command gained a local executable/cwd/flag: %s", built)
			}
			got := runBuiltCommandCapturingArgv(t, built, binDir, logPath)
			if got != strings.Join(want, "\n") {
				t.Fatalf("opaque argv = %q, want %q; command: %s", got, strings.Join(want, "\n"), built)
			}
			if _, err := os.Stat(probePath); !os.IsNotExist(err) {
				t.Fatalf("opaque command must not be capability-probed, stat error: %v", err)
			}
		})
	}
}

func TestBuildCodexCommand_NativeIsolationCachesResolvedClient(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	isolateConfigHomeXDG(t)
	t.Setenv("CODEX_HOME", t.TempDir())
	cfg := &UserConfig{}
	t.Cleanup(resetUserConfigCache(t, cfg))
	supportedDir, supportedLog, supportedProbes := codexIsolationClient(t, "Options:\n  --no-daemon\n")
	olderDir, olderLog, olderProbes := codexIsolationClient(t, "Options:\n  --model <MODEL>\n")
	path := os.Getenv("PATH")
	inst := &Instance{ID: "codex-cached", Title: "cache", Tool: "codex", Command: "codex"}
	for _, run := range []struct {
		dir  string
		log  string
		want string
	}{
		{supportedDir, supportedLog, "--no-daemon"},
		{supportedDir, supportedLog, "--no-daemon"},
		{olderDir, olderLog, ""},
	} {
		t.Setenv("PATH", run.dir+string(os.PathListSeparator)+path)
		built := inst.buildCodexCommand(inst.Command)
		if got := runBuiltCommandCapturingArgv(t, built, run.dir, run.log); got != run.want {
			t.Fatalf("resolved client argv = %q, want %q", got, run.want)
		}
	}
	for _, probes := range []string{supportedProbes, olderProbes} {
		if data, err := os.ReadFile(probes); err != nil || string(data) != "\n" {
			t.Fatalf("client should be probed once, log = %q, err %v", data, err)
		}
	}

	// Inline PATH wins over the ambient older client without changing the
	// process-wide environment, and the prefix still reaches the native probe.
	cfg.Codex.Command = "CODEX_TEST_SCOPE=inline PATH=" + shellescape.Quote(supportedDir+string(os.PathListSeparator)+path) + " codex"
	built := inst.buildCodexCommand(inst.Command)
	if got := runBuiltCommandCapturingArgv(t, built, olderDir, olderLog); got != "--no-daemon" {
		t.Fatalf("inline PATH client argv = %q, want --no-daemon", got)
	}
	if data, err := os.ReadFile(supportedProbes); err != nil || string(data) != "\ninline\n" {
		t.Fatalf("inline probe environment = %q, err %v", data, err)
	}
}

func TestBuildCodexCommand_NativeIsolationUsesLaunchPath(t *testing.T) {
	for _, source := range []string{"tool_env", "restart_env", "init_script", "init_with_inline_path", "init_with_absolute"} {
		t.Run(source, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			isolateConfigHomeXDG(t)
			t.Setenv("CODEX_HOME", t.TempDir())
			supportedDir, supportedLog, supportedProbes := codexIsolationClient(t, "Options:\n  --no-daemon\n")
			olderDir, olderLog, _ := codexIsolationClient(t, "Options:\n  --model <MODEL>\n")
			path := os.Getenv("PATH")
			t.Setenv("PATH", supportedDir+string(os.PathListSeparator)+path)
			olderPath := olderDir + string(os.PathListSeparator) + path
			cfg := &UserConfig{}
			inst := &Instance{ID: "codex-launch-path", Title: "path", Tool: "codex", Command: "codex"}
			want, logPath := "", olderLog
			switch source {
			case "tool_env":
				inst.Tool = "work-codex"
				cfg.Tools = map[string]ToolDef{
					"work-codex": {Command: "codex", CompatibleWith: "codex", Env: map[string]string{"PATH": olderPath}},
				}
			case "restart_env":
				inst.restartEnv = map[string]string{"PATH": olderPath}
			case "init_script", "init_with_inline_path", "init_with_absolute":
				cfg.Shell.InitScript = "export PATH=" + shellescape.Quote(olderPath)
				if source == "init_with_inline_path" {
					cfg.Codex.Command = "PATH=" + shellescape.Quote(supportedDir+string(os.PathListSeparator)+path) + " codex"
					want, logPath = "--no-daemon", supportedLog
				}
				if source == "init_with_absolute" {
					cfg.Codex.Command = shellescape.Quote(filepath.Join(supportedDir, "codex"))
					want, logPath = "--no-daemon", supportedLog
				}
			}
			t.Cleanup(resetUserConfigCache(t, cfg))
			built := inst.buildCodexCommand(inst.Command)
			if got := runBuiltCommandCapturingArgv(t, built, supportedDir, logPath); got != want {
				t.Fatalf("launch-selected client argv = %q, want %q; command: %s", got, want, built)
			}
			if want == "" {
				if _, err := os.Stat(supportedProbes); !os.IsNotExist(err) {
					t.Fatalf("unselected ambient client must not be probed, stat error: %v", err)
				}
			}
		})
	}
}

func TestBuildCodexCommand_NativeIsolationReprobesReplacedExecutable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	isolateConfigHomeXDG(t)
	t.Setenv("CODEX_HOME", t.TempDir())
	binDir, logPath, _ := codexIsolationClient(t, "Options:\n  --no-daemon\n")
	olderDir, _, olderProbes := codexIsolationClient(t, "Options:\n  --model <MODEL>\n")
	binary := filepath.Join(binDir, "codex")
	t.Cleanup(resetUserConfigCache(t, &UserConfig{Codex: CodexSettings{Command: shellescape.Quote(binary)}}))
	inst := &Instance{ID: "codex-replaced", Title: "replacement", Tool: "codex", Command: "codex"}
	built := inst.buildCodexCommand(inst.Command)
	if got := runBuiltCommandCapturingArgv(t, built, binDir, logPath); got != "--no-daemon" {
		t.Fatalf("supported client argv = %q, want --no-daemon", got)
	}
	if err := os.Rename(filepath.Join(olderDir, "codex"), binary); err != nil {
		t.Fatal(err)
	}
	built = inst.buildCodexCommand(inst.Command)
	if got := runBuiltCommandCapturingArgv(t, built, binDir, logPath); got != "" {
		t.Fatalf("replacement older client argv = %q, want no unsupported flag", got)
	}
	if data, err := os.ReadFile(olderProbes); err != nil || string(data) != "\n" {
		t.Fatalf("replacement client was not probed once, log = %q, err %v", data, err)
	}
}

func TestBuildCodexCommand_NativeIsolationUsesIndependentLaunchDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	isolateConfigHomeXDG(t)
	codexHome := filepath.Join(home, ".codex")
	t.Setenv("CODEX_HOME", codexHome)
	supportedDir, logPath, supportedProbes := codexIsolationClient(t, "Options:\n  --no-daemon\n")
	olderDir, _, olderProbes := codexIsolationClient(t, "Options:\n  --model <MODEL>\n")
	supportedProject, olderProject := t.TempDir(), t.TempDir()
	for project, binaryDir := range map[string]string{supportedProject: supportedDir, olderProject: olderDir} {
		vendor := filepath.Join(project, "vendor")
		if err := os.MkdirAll(vendor, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(binaryDir, "codex"), filepath.Join(vendor, "codex")); err != nil {
			t.Fatal(err)
		}
	}
	wrapper := filepath.Join(t.TempDir(), "codex-wrapper")
	launchContext := filepath.Join(t.TempDir(), "launch-context")
	script := "#!/bin/sh\n" +
		"if [ \"${1-}\" != --help ]; then\n" +
		"  printf '%s\\n' \"$PWD\" \"$AGENTDECK_INSTANCE_ID\" \"$CODEX_HOME\" \"$CODEX_TEST_SCOPE\" > " + shellescape.Quote(launchContext) + "\n" +
		"fi\n" +
		"exec ./vendor/codex \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := &UserConfig{Codex: CodexSettings{Command: "CODEX_TEST_SCOPE='kept inline' " + shellescape.Quote(wrapper)}}
	t.Cleanup(resetUserConfigCache(t, cfg))
	inst := &Instance{ID: "codex-cwd", Title: "cwd", Tool: "codex", Command: "codex"}
	for _, run := range []struct {
		project string
		want    string
	}{
		{supportedProject, "--no-daemon"},
		{olderProject, ""},
		{supportedProject, "--no-daemon"},
	} {
		inst.ProjectPath = run.project
		cfg.Shell.InitScript = ""
		if run.want != "" {
			// Setup may change cwd after the help probe. The supported launch
			// must still reach the same project's vendor client with its env.
			cfg.Shell.InitScript = "cd " + shellescape.Quote(olderProject)
		}
		canonicalProject, err := filepath.EvalSymlinks(run.project)
		if err != nil {
			t.Fatal(err)
		}
		built := inst.buildCodexCommand(inst.Command)
		command := "cd " + shellescape.Quote(canonicalProject) + " && " + built
		if got := runBuiltCommandCapturingArgv(t, command, supportedDir, logPath); got != run.want {
			t.Fatalf("project-selected native argv = %q, want %q; command: %s", got, run.want, built)
		}
		wantContext := strings.Join([]string{canonicalProject, inst.ID, codexHome, "kept inline", ""}, "\n")
		if data, err := os.ReadFile(launchContext); err != nil || string(data) != wantContext {
			t.Fatalf("launch cwd/env = %q, err %v; want %q", data, err, wantContext)
		}
	}

	// A fork uses the target worktree's cwd, not the parent's or the setup
	// shell's, and the cd prefix must not consume its hook environment.
	const parentID = "11111111-2222-4333-8444-555555555555"
	seedCodexRollout(t, codexHome, parentID)
	inst.CodexSessionID = parentID
	inst.CodexBindingConfirmed = true
	inst.ProjectPath = olderProject
	forked, built, err := inst.CreateForkedCodexInstanceWithOptions("cwd fork", "", &ClaudeOptions{WorkDir: supportedProject})
	if err != nil {
		t.Fatal(err)
	}
	if got := runBuiltCommandCapturingArgv(t, built, supportedDir, logPath); got != "--no-daemon\nfork\n"+parentID {
		t.Fatalf("cwd-pinned fork argv = %q, want isolated fork with exact parent selector", got)
	}
	canonicalProject, err := filepath.EvalSymlinks(supportedProject)
	if err != nil {
		t.Fatal(err)
	}
	wantContext := strings.Join([]string{canonicalProject, forked.ID, codexHome, "kept inline", ""}, "\n")
	if data, err := os.ReadFile(launchContext); err != nil || string(data) != wantContext {
		t.Fatalf("fork launch cwd/env = %q, err %v; want %q", data, err, wantContext)
	}

	for _, path := range []string{supportedProbes, olderProbes} {
		if data, err := os.ReadFile(path); err != nil || string(data) != "kept inline\n" {
			t.Fatalf("each launch directory must cache its own help result once, log = %q, err %v", data, err)
		}
	}
}

func TestBuildCodexCommand_NativeIsolationPinsExecutableAgainstLaunchLookup(t *testing.T) {
	for _, mode := range []string{"normal", "session_launch_shell", "global_launch_shell", "session_override_off", "launch_shell_inline_path"} {
		for _, resume := range []bool{false, true} {
			name := mode + "/fresh"
			if resume {
				name = mode + "/resume"
			}
			t.Run(name, func(t *testing.T) {
				home := t.TempDir()
				t.Setenv("HOME", home)
				isolateConfigHomeXDG(t)
				t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
				supportedDir, logPath, supportedProbes := codexIsolationClient(t, "Options:\n  --no-daemon\n")
				olderDir, _, _ := codexIsolationClient(t, "Options:\n  --model <MODEL>\n")
				path := os.Getenv("PATH")
				t.Setenv("PATH", supportedDir+string(os.PathListSeparator)+path)
				cfg := &UserConfig{}
				inst := &Instance{ID: "codex-shell-path", Title: "shell path", Tool: "codex", Command: "codex", ProjectPath: home}
				want := []string{"--no-daemon"}
				switch mode {
				case "session_launch_shell":
					inst.LaunchShell = boolPtr(true)
					want = nil
				case "global_launch_shell":
					cfg.Shell.LaunchShell = boolPtr(true)
					want = nil
				case "session_override_off":
					cfg.Shell.LaunchShell = boolPtr(true)
					inst.LaunchShell = boolPtr(false)
				case "launch_shell_inline_path":
					inst.LaunchShell = boolPtr(true)
					cfg.Codex.Command = "PATH=" + shellescape.Quote(supportedDir+string(os.PathListSeparator)+path) + " codex"
				}
				if resume {
					const sid = "11111111-2222-4333-8444-555555555555"
					inst.CodexSessionID = sid
					inst.CodexBindingConfirmed = true
					seedCodexRollout(t, os.Getenv("CODEX_HOME"), sid)
					want = append(want, "resume", sid)
				}
				t.Cleanup(resetUserConfigCache(t, cfg))
				built := inst.buildCodexCommand(inst.Command)
				// Reproduce the changed lookup environment of a login shell,
				// without sourcing the developer's real shell startup files.
				command := "export PATH=" + shellescape.Quote(olderDir+string(os.PathListSeparator)+path) + " && " + built
				if got := runBuiltCommandCapturingArgv(t, command, olderDir, logPath); got != strings.Join(want, "\n") {
					t.Fatalf("launch-shell-selected argv = %q, want %q; command: %s", got, strings.Join(want, "\n"), built)
				}
				if mode == "session_launch_shell" || mode == "global_launch_shell" {
					if _, err := os.Stat(supportedProbes); !os.IsNotExist(err) {
						t.Fatalf("unresolved launch-shell client must not use the ambient capability, stat error: %v", err)
					}
				}
			})
		}
	}
}
