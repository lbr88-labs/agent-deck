package session

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"al.essio.dev/pkg/shellescape"
)

// Like deepSeekIdentityCache, this caches a bounded --help probe by resolved
// executable. Keep launch directories independent for project-local delegates,
// and refresh when the executable or its explicit environment changes.
var codexNoDaemonCache sync.Map // codexNoDaemonCacheKey -> codexNoDaemonCapability

type codexNoDaemonCacheKey struct {
	path    string
	workDir string
}

type codexNoDaemonCapability struct {
	info      os.FileInfo
	env       string
	supported bool
}

// codexCommandWithNativeIsolation keeps the interactive writer in this launch's
// process tree when the native client advertises --no-daemon. This is not
// conversation ownership evidence and does not expand the supported command
// grammar. Opaque commands, SSH hosts, and container clients cannot be identified
// by a host --help probe, so they are left untouched, including an existing flag.
// The second result pins the launch cwd. Builders apply it after environment
// setup but before inline AGENTDECK_* assignments and native flags/selectors.
func (i *Instance) codexCommandWithNativeIsolation(command string) (string, string) {
	if i.IsSandboxed() || i.IsSSH() {
		return command, ""
	}
	bareCommand := command
	if !IsSupportedCodexLaunchCommand(bareCommand) {
		_, bareCommand = codexExactResumeCommand(command)
		if bareCommand == "" {
			return command, ""
		}
	}
	// nextShellWord handles quoting but deliberately does not evaluate shell
	// expansions or operators. Never run a shell to inspect a custom command.
	if strings.ContainsAny(bareCommand, "$`;&|<>()\r\n") {
		return command, ""
	}

	// Match the static launch environment's precedence. Sourced files/scripts
	// may select another binary; never execute them just to inspect --help.
	pathKnown := !i.launchShellEnabled()
	if cfg, err := LoadUserConfig(); err != nil {
		pathKnown = false
	} else if cfg != nil && (len(cfg.Shell.EnvFiles) != 0 || cfg.Shell.InitScript != "") {
		pathKnown = false
	}
	if i.getToolEnvFile() != "" {
		pathKnown = false
	}
	var env []string
	appendEnv := func(values map[string]string) {
		start := len(env)
		for key, value := range values {
			if !IsValidEnvKey(key) {
				continue
			}
			env = append(env, key+"="+value)
			if key == "PATH" {
				pathKnown = true
			}
		}
		sort.Strings(env[start:])
	}
	if def := GetToolDef(i.Tool); def != nil {
		appendEnv(def.Env)
	}
	if name := strings.TrimPrefix(i.Title, "conductor-"); name != "" && name != i.Title {
		if meta, err := LoadConductorMeta(name); err != nil {
			pathKnown = false
		} else {
			if meta.EnvFile != "" {
				pathKnown = false
			}
			appendEnv(meta.Env)
		}
	}
	appendEnv(i.restartEnv)
	rest := strings.TrimLeft(command, " \t\r\n")
	for rest != "" {
		word, remainder, ok := nextShellWord(rest)
		if !ok {
			return command, ""
		}
		if isShellEnvAssignment(word) {
			env = append(env, word)
			if strings.HasPrefix(word, "PATH=") {
				pathKnown = true
			}
			rest = strings.TrimLeft(remainder, " \t\r\n")
			continue
		}
		if !pathKnown && !strings.ContainsAny(word, `/\`) {
			return command, ""
		}
		workDir := codexCapabilityWorkingDir(i.EffectiveWorkingDir())
		if workDir == "" {
			return command, ""
		}
		pathEnv := os.Getenv("PATH")
		for _, assignment := range env {
			if value, ok := strings.CutPrefix(assignment, "PATH="); ok {
				pathEnv = value
			}
		}
		path := codexCapabilityExecutable(word, pathEnv, workDir)
		if path == "" || !codexSupportsNoDaemon(path, env, workDir) {
			return command, ""
		}
		// The launch may run through bash -lc even without launch_shell.
		// Never attach a verified flag to a later, unresolved PATH lookup.
		start := len(command) - len(rest)
		return command[:start] + shellescape.Quote(path) + " --no-daemon" + remainder, workDir
	}
	return command, ""
}

func codexSupportsNoDaemon(path string, env []string, workDir string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	envKey := strings.Join(env, "\x00")
	key := codexNoDaemonCacheKey{path: path, workDir: workDir}
	if cached, ok := codexNoDaemonCache.Load(key); ok {
		previous := cached.(codexNoDaemonCapability)
		if previous.env == envKey && os.SameFile(previous.info, info) &&
			previous.info.Size() == info.Size() && previous.info.ModTime().Equal(info.ModTime()) {
			return previous.supported
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// #nosec G204 -- the supported native executable is resolved without a shell;
	// --help is the only argument, never a configured command or subcommand.
	cmd := exec.CommandContext(ctx, path, "--help")
	cmd.Dir = workDir
	cmd.Env = append(cmd.Environ(), env...)
	// Match the DeepSeek/Hermes probe bound even if a descendant holds stdout.
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	supported := false
	if err == nil {
		for len(out) > 0 {
			var line []byte
			line, out, _ = bytes.Cut(out, []byte{'\n'})
			line = bytes.TrimSpace(line)
			if bytes.Equal(line, []byte("--no-daemon")) ||
				bytes.HasPrefix(line, []byte("--no-daemon ")) ||
				bytes.HasPrefix(line, []byte("--no-daemon\t")) {
				supported = true
				break
			}
		}
	}
	codexNoDaemonCache.Store(key, codexNoDaemonCapability{info: info, env: envKey, supported: supported})
	return supported
}

// Resolve inline PATH and relative executables against the launch directory,
// without changing Agent Deck's process environment or invoking a user shell.
func codexCapabilityExecutable(executable, pathEnv, workDir string) string {
	if strings.ContainsAny(executable, `/\`) {
		if !filepath.IsAbs(executable) {
			if !filepath.IsAbs(workDir) {
				return ""
			}
			executable = filepath.Join(workDir, executable)
		}
		path, _ := exec.LookPath(executable)
		return path
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		if !filepath.IsAbs(dir) {
			if !filepath.IsAbs(workDir) {
				return ""
			}
			dir = filepath.Join(workDir, dir)
		}
		if path, err := exec.LookPath(filepath.Join(dir, executable)); err == nil {
			return path
		}
	}
	return ""
}

func codexCapabilityWorkingDir(workDir string) string {
	// Match tmux.Start's empty-path fallback and control-whitespace trimming;
	// plain leading/trailing spaces are valid directory-name characters.
	if workDir == "" {
		workDir = os.Getenv("HOME")
	}
	workDir = strings.Trim(workDir, "\n\r\t\v\f")
	if workDir == "" {
		return ""
	}
	absolute, err := filepath.Abs(workDir)
	if err != nil {
		return ""
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return ""
	}
	return canonical
}
