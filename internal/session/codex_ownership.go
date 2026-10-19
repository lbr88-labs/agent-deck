package session

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/docker"
)

// Binding provenance is separate from conflict detection: an unregistered
// native thread is not available for another pane to claim.
type codexBindingSource string

const (
	codexBindingProcess codexBindingSource = "process_probe"
	codexBindingHook    codexBindingSource = "pane_hook"
	codexBindingTmux    codexBindingSource = "tmux_env"
)

// RefreshAgentConfigs(..., "") mounts the sandbox backing store here.
const codexSandboxContainerHome = "/root/.codex"

func (source codexBindingSource) provesOwnership() bool {
	return source == codexBindingProcess || source == codexBindingHook
}

// prepareCodexIdentity runs before any launch can tear down the current pane.
// An ordinary restart must either resume its recorded identity or refuse; only
// RestartFresh (or a newly-created instance's first Start) may start unbound.
func (i *Instance) prepareCodexIdentity(restarting bool) error {
	if !IsCodexCompatible(i.Tool) {
		return nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	command := i.resolveCodexCommand(i.Command)
	if err := i.validateCodexSandboxHome(command); err != nil {
		return i.refuseCodexLaunch(err.Error())
	}
	if !i.adoptPersistedCodexPromotion(true) {
		i.adoptCompletedCodexMigrationTarget(true)
	}
	if restarting || (i.CodexSessionID == "" && !i.codexFreshStart && !i.IsForkAwaitingStart) {
		i.lastCodexProbeAt = time.Now()
		probe := i.queryCodexSessionFromProcessFiles()
		if probe.ambiguous {
			return i.refuseCodexLaunch("the pane has conflicting conversation identities; its process and binding are preserved. Resolve the active conversation before restarting, or explicitly use session restart-fresh")
		}
		candidate := probe.sessionID
		if candidate != "" {
			if candidate != i.CodexSessionID && !codexVerifiedTopLevelSession(i.getCodexHomeDir(), candidate) {
				return i.refuseCodexLaunch("the pane's conversation metadata cannot be verified; its binding is preserved. Restore the exact conversation or explicitly use session restart-fresh")
			}
			i.acceptCodexSessionID(candidate, true, codexBindingProcess)
			if !strings.EqualFold(candidate, i.CodexSessionID) {
				return i.refuseCodexLaunch("the pane's actual conversation could not be accepted; its process and binding are preserved. Repair the exact binding before restarting, or explicitly use session restart-fresh")
			}
		}
		if restarting {
			i.pendingCodexRestartWarning = codexProbeMissingWarning(probe.missingDependency)
		}
		if status := readHookStatusFile(i.ID); status != nil {
			candidate := strings.ToLower(strings.TrimSpace(status.SessionID))
			if i.codexHookOwnsCandidate(status) {
				i.acceptCodexSessionID(candidate, true, codexBindingHook)
				if !strings.EqualFold(candidate, i.CodexSessionID) {
					return i.refuseCodexLaunch("the pane-reported conversation could not be accepted; its process and binding are preserved. Repair the exact binding before restarting, or explicitly use session restart-fresh")
				}
			} else if candidate != "" && !strings.EqualFold(candidate, i.CodexSessionID) &&
				!IsCodexSubagentSession(i.getCodexHomeDir(), candidate) && i.codexHookEmitterOwns(status) {
				return i.refuseCodexLaunch("the pane-reported conversation conflicts with live evidence or cannot be verified; its process and binding are preserved. Restore the exact conversation or explicitly use session restart-fresh")
			}
		}
	}
	resumeID, _ := codexExactResumeCommand(command)
	if i.CodexSessionID == "" {
		if resumeID != "" {
			return i.refuseCodexLaunch("the command explicitly resumes a conversation but the instance has no binding; import or bind that exact UUID before starting, or explicitly use session restart-fresh")
		}
		if !restarting && (i.codexFreshStart || i.IsForkAwaitingStart ||
			(i.LastStartedAt.IsZero() && i.lastStartTime.IsZero() && i.CodexStartedAt == 0 &&
				i.CodexBindingRevision == 0 && i.CodexDetectedAt.IsZero())) {
			return nil
		}
		return i.refuseCodexLaunch("the conversation identity is unknown; bind the exact Codex conversation or explicitly use session restart-fresh")
	}
	if !isCodexSessionUUID(i.CodexSessionID) {
		return i.refuseCodexLaunch("the recorded conversation identity is invalid; bind the exact Codex UUID or explicitly use session restart-fresh")
	}
	if i.Tool == "codex" && !IsSupportedCodexLaunchCommand(command) && !strings.EqualFold(resumeID, i.CodexSessionID) {
		return i.refuseCodexLaunch("the resolved command cannot be verified to resume the recorded UUID; use a supported Codex command or an exact `codex resume <UUID>` command. The binding is preserved")
	}
	owner, err := i.codexSessionOwner(i.CodexSessionID)
	if err != nil {
		return i.refuseCodexLaunch(fmt.Sprintf("conversation ownership cannot be verified: %v; its binding is preserved", err))
	}
	if owner != "" {
		return i.refuseCodexLaunch(fmt.Sprintf("conversation %s is also bound to instance %s; repair the explicit bindings before restarting, or use session restart-fresh", i.CodexSessionID, owner))
	}
	if !codexResumableInHome(i.CodexSessionID, i.getCodexHomeDir()) {
		return i.refuseCodexLaunch(fmt.Sprintf("conversation %s cannot be verified in this Codex home; its binding is preserved. Restore the conversation/account or explicitly use session restart-fresh", i.CodexSessionID))
	}
	return nil
}

func (i *Instance) refuseCodexLaunch(reason string) error {
	err := fmt.Errorf("refusing Codex launch: %s", reason)
	i.recordPrepareFailure(i.Command, err)
	return err
}

// A sandbox has one known Codex mount. Never silently substitute it for an
// explicit account/home or forward a host-only path into the container.
func (i *Instance) validateCodexSandboxHome(command string) error {
	if !i.IsSandboxed() {
		return nil
	}
	hostHome, err := os.UserHomeDir()
	if err != nil || hostHome == "" {
		return fmt.Errorf("sandbox Codex home cannot be verified; its process and binding are preserved")
	}
	inlineHome, inlineHomeSet := "", false
	for rest := strings.TrimSpace(command); rest != ""; {
		token, remainder, ok := nextShellWord(rest)
		if !ok || !isShellEnvAssignment(token) {
			break
		}
		key, value, _ := strings.Cut(token, "=")
		if key == "CODEX_HOME" {
			// Shells honor the last assignment; never validate only the first.
			if inlineHomeSet {
				return fmt.Errorf("sandbox command repeats CODEX_HOME; its process and binding are preserved")
			}
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("sandbox command explicitly empties CODEX_HOME; its process and binding are preserved")
			}
			inlineHome, inlineHomeSet = ExpandPath(strings.TrimSpace(value)), true
		}
		rest = strings.TrimLeft(remainder, " \t\r\n")
	}
	if inlineHomeSet {
		if filepath.Clean(inlineHome) != codexSandboxContainerHome {
			return fmt.Errorf("sandbox CODEX_HOME %q is not its mounted container home; its process and binding are preserved", inlineHome)
		}
	} else {
		selectedHome := strings.TrimSpace(i.accountCodexHomeDir())
		if selectedHome == "" {
			selectedHome = getCodexHomeDir()
		}
		selectedHome = filepath.Clean(selectedHome)
		if selectedHome != filepath.Join(hostHome, ".codex") && selectedHome != docker.SandboxDir(hostHome, ".codex") {
			return fmt.Errorf("sandbox Codex account/home %q has no verified container mapping; its process and binding are preserved", selectedHome)
		}
	}
	config, err := LoadUserConfig()
	if err != nil {
		return fmt.Errorf("sandbox Codex configuration cannot be verified: %v; its process and binding are preserved", err)
	}
	if config == nil {
		return nil
	}
	if value, set := config.Docker.EnvironmentValues["CODEX_HOME"]; set && filepath.Clean(value) != codexSandboxContainerHome {
		return fmt.Errorf("sandbox static CODEX_HOME %q is not its mounted container home; its process and binding are preserved", value)
	}
	for _, name := range config.Docker.Environment {
		if name == "CODEX_HOME" {
			if value, set := os.LookupEnv(name); set && filepath.Clean(value) != codexSandboxContainerHome {
				return fmt.Errorf("sandbox forwarded CODEX_HOME %q is not its mounted container home; its process and binding are preserved", value)
			}
		}
	}
	for _, mount := range config.Docker.ExtraVolumes {
		mount, _, _ = strings.Cut(mount, ":")
		mount = filepath.Clean(mount)
		if mount == "/" || mount == codexSandboxContainerHome ||
			strings.HasPrefix(mount, codexSandboxContainerHome+"/") ||
			strings.HasPrefix(codexSandboxContainerHome, mount+"/") {
			return fmt.Errorf("sandbox volume %q shadows the verified Codex mount; its process and binding are preserved", mount)
		}
	}
	return nil
}

// A hook's AGENTDECK_INSTANCE_ID can be inherited by a shared Codex daemon.
// Only an emitter in this pane's live process tree may introduce a new ID.
// Existing-ID status updates need no new identity authority.
func (i *Instance) codexHookOwnsCandidate(status *HookStatus) bool {
	if status == nil || strings.TrimSpace(status.SessionID) == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(status.SessionID), strings.TrimSpace(i.CodexSessionID)) {
		return true
	}
	if !codexVerifiedTopLevelSession(i.getCodexHomeDir(), status.SessionID) || !i.codexHookEmitterOwns(status) {
		return false
	}
	probe := i.queryCodexSessionFromProcessFiles()
	return !probe.ambiguous && (probe.sessionID == "" || strings.EqualFold(probe.sessionID, strings.TrimSpace(status.SessionID)))
}

func (i *Instance) codexHookEmitterOwns(status *HookStatus) bool {
	if status == nil {
		return false
	}
	if status.CodexEmitterPID <= 0 || status.CodexEmitterPID == os.Getpid() ||
		status.CodexEmitterIdentity == "" || status.CodexEmitterIdentity != CodexProcessIdentity(status.CodexEmitterPID) {
		return false
	}
	// Npm/shell launchers can be ancestors of the native TUI. Prefer the
	// unique native process; a second interactive TUI in the same tree makes
	// foreground ownership ambiguous, not permission to take either chat.
	nativePID, launcherPID, launchers := 0, 0, 0
	for _, pid := range i.collectTmuxPaneProcessTreePIDs() {
		args := codexProcessArguments(pid)
		if !isInteractiveCodexArgs(args) {
			continue
		}
		switch filepath.Base(args[0]) {
		case "sh", "bash", "dash", "zsh", "node", "bun":
			launcherPID, launchers = pid, launchers+1
		default:
			if nativePID != 0 {
				return false
			}
			nativePID = pid
		}
	}
	if nativePID != 0 {
		return nativePID == status.CodexEmitterPID
	}
	return launchers == 1 && launcherPID == status.CodexEmitterPID
}

var codexBootIdentity struct {
	sync.Once
	value string
}

// CodexProcessIdentity identifies a hook emitter's lifetime, so a delayed hook
// cannot gain authority merely because the OS reused a pane process's PID.
func CodexProcessIdentity(pid int) string {
	if pid <= 0 {
		return ""
	}
	if runtime.GOOS == "linux" {
		raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
		if err != nil {
			return ""
		}
		end := bytes.LastIndexByte(raw, ')')
		if end < 0 {
			return ""
		}
		fields := strings.Fields(string(raw[end+1:]))
		if len(fields) <= 19 {
			return ""
		}
		codexBootIdentity.Do(func() {
			data, _ := os.ReadFile("/proc/sys/kernel/random/boot_id")
			codexBootIdentity.value = strings.TrimSpace(string(data))
		})
		if codexBootIdentity.value == "" {
			return ""
		}
		return codexBootIdentity.value + ":" + fields[19]
	}
	// #nosec G204 -- "ps" and flags are fixed; the only varying argument is
	// strconv.Itoa(int), passed as one argv element without shell evaluation.
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "lstart=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func codexVerifiedTopLevelSession(codexHome, sessionID string) bool {
	path := codexRolloutPathInHome(sessionID, codexHome)
	if path == "" {
		return false
	}
	meta := readCodexRolloutThreadMetadata(path)
	return meta.kind == codexThreadTopLevel && strings.EqualFold(meta.id, strings.TrimSpace(sessionID))
}

// Codex's own hub can resolve a UUID after moving its rollout out of the dated
// tree. These exact-ID lookups verify resumability, never discover ownership.
func codexResumableInHome(sessionID, codexHome string) bool {
	sessionID = strings.ToLower(strings.TrimSpace(sessionID))
	if !isCodexSessionUUID(sessionID) {
		return false
	}
	if path := codexRolloutPathInHome(sessionID, codexHome); path != "" && readCodexRolloutThreadMetadata(path).id == sessionID {
		return true
	}
	latest := make(map[string]CodexIndexEntry)
	if err := readCodexJSONIndex(codexHome, latest); err == nil {
		if _, known := latest[sessionID]; known {
			return true
		}
	}
	path := filepath.Join(codexHome, "state_5.sqlite")
	if _, err := os.Stat(path); err != nil {
		return false
	}
	db, err := sql.Open("sqlite", codexSQLiteReadOnlyDSN(path))
	if err != nil {
		return false
	}
	defer db.Close()
	// Archived threads are still resumable; the import picker's active-only
	// query deliberately has a different contract.
	var known int
	return db.QueryRow("SELECT 1 FROM threads WHERE id = ? LIMIT 1", sessionID).Scan(&known) == nil
}

func codexProcessArguments(pid int) []string {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
		if err != nil {
			return nil
		}
		return strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	}
	// #nosec G204 -- "ps" and flags are fixed; the only varying argument is
	// strconv.Itoa(int), passed as one argv element without shell evaluation.
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "args=").Output()
	if err != nil {
		return nil
	}
	var args []string
	rest := strings.TrimSpace(string(out))
	for rest != "" {
		word, remainder, ok := nextShellWord(rest)
		if !ok {
			return nil
		}
		args = append(args, word)
		rest = strings.TrimSpace(remainder)
	}
	return args
}

func isInteractiveCodexArgs(args []string) bool {
	if len(args) == 0 {
		return false
	}
	executable := strings.ToLower(filepath.Base(args[0]))
	switch executable {
	case "sh", "bash", "dash", "zsh", "node", "bun":
		// A directly-executed wrapper script is fine; a shell's -c payload
		// merely mentioning Codex is not process identity evidence.
		if len(args) < 2 || strings.HasPrefix(args[1], "-") {
			return false
		}
		args = args[1:]
		executable = strings.ToLower(filepath.Base(args[0]))
	}
	executable = strings.TrimSuffix(executable, ".exe")
	if executable != "codex" && executable != "codex.js" &&
		!strings.HasPrefix(executable, "codex-") && !strings.HasPrefix(executable, "codex_") {
		return false
	}
	for index := 1; index < len(args); index++ {
		arg := args[index]
		option, _, inlineValue := strings.Cut(arg, "=")
		switch option {
		case "--model", "-m", "--config", "-c", "--profile", "-p", "--cd", "-C", "--add-dir",
			"--sandbox", "-s", "--ask-for-approval", "-a", "--enable", "--disable", "--image", "-i", "--local-provider":
			if !inlineValue {
				index++
				if index >= len(args) {
					return false
				}
			}
			continue
		case "--oss", "--full-auto", "--dangerously-bypass-approvals-and-sandbox", "--yolo", "--search", "--no-alt-screen":
			if inlineValue {
				return false
			}
			continue
		case "--":
			return true
		}
		if strings.HasPrefix(arg, "-") {
			return false
		}
		switch arg {
		case "app-server", "exec", "e", "review", "mcp", "mcp-server", "cloud",
			"login", "logout", "features", "debug", "apply", "completion", "sandbox", "proto":
			return false
		default:
			return true
		}
	}
	return true
}

func codexProcFDWritable(path string) bool {
	info, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	start := bytes.Index(info, []byte("flags:"))
	if start < 0 {
		return false
	}
	flagsText := info[start+len("flags:"):]
	if end := bytes.IndexByte(flagsText, '\n'); end >= 0 {
		flagsText = flagsText[:end]
	}
	flags, err := strconv.ParseUint(string(bytes.TrimSpace(flagsText)), 8, 64)
	return err == nil && flags&uint64(os.O_WRONLY|os.O_RDWR) != 0
}

func codexExactResumeCommand(command string) (sessionID, freshCommand string) {
	command = strings.TrimSpace(command)
	rest := command
	for rest != "" {
		word, remainder, ok := nextShellWord(rest)
		if !ok {
			return "", ""
		}
		if isShellEnvAssignment(word) {
			rest = strings.TrimSpace(remainder)
			continue
		}
		if !IsSupportedCodexLaunchCommand(strconv.Quote(word)) {
			return "", ""
		}
		freshCommand = strings.TrimSpace(command[:len(command)-len(strings.TrimSpace(remainder))])
		if freshCommand == "codex" {
			// Keep this an explicit per-instance command after restart-fresh;
			// resolving a literal bare codex would restore a global resume command.
			freshCommand = `"codex"`
		}
		subcommand, remainder, ok := nextShellWord(strings.TrimSpace(remainder))
		if !ok || subcommand != "resume" {
			return "", ""
		}
		id, remainder, ok := nextShellWord(strings.TrimSpace(remainder))
		if !ok || !isCodexSessionUUID(id) || strings.TrimSpace(remainder) != "" {
			return "", ""
		}
		return strings.ToLower(id), freshCommand
	}
	return "", ""
}
