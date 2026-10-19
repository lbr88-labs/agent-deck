package session

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// ChatSearchSupportedTools is the UI's saved-chat support hint. Pi requires one
// unambiguous saved conversation in the instance-owned session directory.
const ChatSearchSupportedTools = "OMP, Codex, Claude, Gemini, OpenCode, Pi, Kiro, Copilot"

const (
	chatSearchRecordLimit  = 1024 * 1024
	chatSearchFileLimit    = 128 * 1024 * 1024
	chatSearchSnippetRunes = 220
)

var (
	errChatSearchLimit     = errors.New("saved chat scan limit reached")
	errChatSearchIdentity  = errors.New("saved conversation identity unavailable")
	errChatSearchFormat    = errors.New("saved chat format unsupported")
	errChatSearchAmbiguous = errors.New("saved conversation binding ambiguous")
)

// ChatSearchBinding owns a private, immutable snapshot. Constructing it performs
// no filesystem/network I/O; account roots and saved paths are resolved by the
// search worker, never by the UI or through a live Instance pointer.
// InstanceID and Tool identify the supplied row in results and status messages.
// Obtain bindings with NewChatSearchBinding rather than constructing literals.
type ChatSearchBinding struct {
	InstanceID string
	Tool       string
	instance   *Instance
	sandboxed  bool
}

// ChatSearchMatch contains only the matched row identity and a terminal-safe,
// bounded excerpt, not the saved message or an in-memory transcript corpus.
type ChatSearchMatch struct {
	InstanceID string
	Snippet    string
}

// ChatSearchUnavailable is a short, non-sensitive status for one supplied row.
// Reason never includes filesystem paths, provider errors, or saved content.
type ChatSearchUnavailable struct {
	InstanceID string
	Tool       string
	Reason     string
}

// ChatSearchReport distinguishes an ordinary no-match from unreadable or
// unsupported bindings. Partial means cancellation, a scan limit, or skipped
// oversized/malformed records; unsupported/missing chats are listed separately.
type ChatSearchReport struct {
	Matches     []ChatSearchMatch
	Unavailable []ChatSearchUnavailable
	Partial     bool
}

// NewChatSearchBinding copies the fields needed by existing account resolvers
// under the instance mutex. In particular, those resolvers may read config on a
// cache miss, so they must NOT be called while constructing this snapshot.
func NewChatSearchBinding(inst *Instance) ChatSearchBinding {
	if inst == nil {
		return ChatSearchBinding{}
	}
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	snapshot := &Instance{
		ID: inst.ID, Tool: inst.Tool, Title: inst.Title,
		ProjectPath: inst.ProjectPath, GroupPath: inst.GroupPath,
		Account: inst.Account, Command: inst.Command,
		MultiRepoEnabled: inst.MultiRepoEnabled, MultiRepoTempDir: inst.MultiRepoTempDir,
		ClaudeSessionID: inst.recordedClaudeSessionID(), CodexSessionID: inst.CodexSessionID,
		GeminiSessionID: inst.GeminiSessionID, OpenCodeSessionID: inst.OpenCodeSessionID,
		KiroSessionID: inst.KiroSessionID, CopilotSessionID: inst.CopilotSessionID,
		SSHHost: inst.SSHHost, SSHRemotePath: inst.SSHRemotePath,
		ToolOptionsJSON: append(json.RawMessage(nil), inst.ToolOptionsJSON...),
	}
	return ChatSearchBinding{
		InstanceID: inst.ID, Tool: inst.Tool, instance: snapshot,
		sandboxed: inst.IsSandboxed() || inst.SandboxContainer != "",
	}
}

// SearchLocalChats streams only the exact saved conversations bound to the
// supplied rows. It never launches a provider, fetches remote content, creates a
// database, discovers a latest-by-project conversation, or retains a corpus.
// Call it outside the UI goroutine, with a context canceled when the query or
// visible rows change. Results preserve the caller's InstanceID verbatim.
func SearchLocalChats(ctx context.Context, bindings []ChatSearchBinding, query string) ChatSearchReport {
	var report ChatSearchReport
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return report
	}
	for _, binding := range bindings {
		if ctx.Err() != nil {
			report.Partial = true
			break
		}
		snippet, partial, err := searchLocalChat(ctx, binding, needle)
		report.Partial = report.Partial || partial
		if ctx.Err() != nil {
			report.Partial = true
			break
		}
		if err != nil {
			report.Unavailable = append(report.Unavailable, ChatSearchUnavailable{
				InstanceID: binding.InstanceID, Tool: binding.Tool, Reason: chatSearchReason(err),
			})
			continue
		}
		if snippet != "" {
			report.Matches = append(report.Matches, ChatSearchMatch{InstanceID: binding.InstanceID, Snippet: snippet})
		}
	}
	return report
}

var (
	errChatSearchRemote        = errors.New("remote transcript is not local")
	errChatSearchSandbox       = errors.New("container transcript is not locally resolvable")
	errChatSearchNotSaved      = errors.New("conversation is not saved")
	errChatSearchKiroStore     = errors.New("Kiro CLI transcript missing; other stores unsupported")
	errChatSearchOpenCodeStore = errors.New("OpenCode SQLite missing; legacy JSON unsupported")
)

func chatSearchReason(err error) string {
	switch {
	case errors.Is(err, errChatSearchRemote):
		return "remote transcript is not local"
	case errors.Is(err, errChatSearchSandbox):
		return "container transcript is not locally resolvable"
	case errors.Is(err, errChatSearchNotSaved):
		return "conversation is not saved"
	case errors.Is(err, errChatSearchKiroStore):
		return "Kiro CLI transcript missing; other stores unsupported"
	case errors.Is(err, errChatSearchOpenCodeStore):
		return "OpenCode SQLite missing; legacy JSON unsupported"
	case errors.Is(err, errChatSearchIdentity):
		return "saved conversation identity unavailable"
	case errors.Is(err, errChatSearchAmbiguous):
		return "saved conversation binding ambiguous"
	case errors.Is(err, errChatSearchFormat):
		return "saved chat format unsupported"
	case errors.Is(err, errChatSearchLimit):
		return "saved chat scan limit reached"
	case errors.Is(err, os.ErrNotExist):
		return "saved transcript not found"
	case errors.Is(err, os.ErrPermission):
		return "saved transcript is not readable"
	default:
		return "saved transcript is unavailable"
	}
}

func searchLocalChat(ctx context.Context, binding ChatSearchBinding, needle string) (string, bool, error) {
	inst := binding.instance
	if inst == nil {
		return "", false, errChatSearchIdentity
	}
	// Defense in depth: the UI's local-only filtering is not an authority.
	if inst.SSHHost != "" {
		return "", false, errChatSearchRemote
	}
	if binding.sandboxed {
		return "", false, errChatSearchSandbox
	}
	switch {
	case inst.Tool == "omp":
		if inst.resolvedOmpOptions().NoSession {
			return "", false, errChatSearchNotSaved
		}
		if !validHookSessionAnchorInstanceID(inst.ID) {
			return "", false, errChatSearchIdentity
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, err
		}
		dir := filepath.Join(home, ".omp", "agent-deck", inst.ID)
		active, generation, tracked, err := readOmpActiveBindingSelectionContext(ctx, dir)
		if err != nil {
			return "", false, errChatSearchIdentity
		}
		if active == nil {
			return "", false, errChatSearchIdentity
		}
		if tracked {
			if err := ctx.Err(); err != nil {
				return "", true, err
			}
			// Reconciliation failures retain the previous active binding. Only
			// the provider's matching positive ACK authorizes reading it.
			data, statusErr := readOmpHealthFile(filepath.Join(dir, ".agent-deck-omp-status."+generation+".json"))
			if err := ctx.Err(); err != nil {
				return "", true, err
			}
			var status ompIdentityStatus
			if statusErr != nil || json.Unmarshal(data, &status) != nil || !status.IdentityReady ||
				status.InstanceID != inst.ID || status.LaunchID != generation ||
				status.SessionID != active.SessionID || status.SessionFile != active.File {
				return "", false, errChatSearchIdentity
			}
			// A title-only Error does not revoke a valid identity ACK. As in
			// binding selection, reject a launch that changed during the read.
			_, currentGeneration, currentTracked, generationErr := ompCurrentBindingPath(dir)
			if err := ctx.Err(); err != nil {
				return "", true, err
			}
			if generationErr != nil || !currentTracked || currentGeneration != generation {
				return "", false, errChatSearchIdentity
			}
		}
		// A failed fresh start deliberately supersedes the previous binding.
		marker := filepath.Join(dir, ".agent-deck-fresh-pending")
		if tracked {
			marker += "." + generation
		}
		if pending, exists, markerErr := readOmpControlLine(marker, "fresh-conversation boundary"); markerErr != nil || (exists && (!ompSafeGeneration(pending) || pending != active.Generation)) {
			return "", false, errChatSearchIdentity
		}
		if active.State == "pending" {
			if _, err := os.Lstat(active.File); os.IsNotExist(err) {
				return "", false, errChatSearchNotSaved
			}
		}
		return chatSearchJSONL(ctx, active.File, "omp", active.SessionID, needle)
	case IsCodexCompatible(inst.Tool):
		id := strings.ToLower(strings.TrimSpace(inst.CodexSessionID))
		if !isCodexSessionUUID(id) {
			return "", false, errChatSearchIdentity
		}
		// This is the launch resolver, including command CODEX_HOME and account.
		codexHome := inst.getCodexHomeDir()
		matches := func(path string) bool {
			base := filepath.Base(path)
			return strings.HasPrefix(base, "rollout-") && strings.HasSuffix(base, "-"+id+".jsonl")
		}
		// Follow codexRolloutPathInHome's sessions-then-archive lookup, while
		// retaining cancellable enumeration and rejecting ambiguous exact IDs.
		path, err := chatSearchUniqueFile(ctx, filepath.Join(codexHome, "sessions"), 3, false, matches)
		if errors.Is(err, os.ErrNotExist) {
			path, err = chatSearchUniqueFile(ctx, filepath.Join(codexHome, "archived_sessions"), 0, false, matches)
		}
		if err != nil {
			return "", errors.Is(err, errChatSearchLimit), err
		}
		return chatSearchJSONL(ctx, path, "codex", id, needle)
	case IsClaudeCompatible(inst.Tool):
		if !IsBareClaudeSessionUUID(inst.ClaudeSessionID) {
			return "", false, errChatSearchIdentity
		}
		path, err := chatSearchClaudePath(ctx, GetClaudeConfigDirForInstance(inst), inst.EffectiveWorkingDir(), inst.ClaudeSessionID)
		if err != nil {
			return "", errors.Is(err, errChatSearchLimit), err
		}
		return chatSearchJSONL(ctx, path, "claude", inst.ClaudeSessionID, needle)
	case inst.Tool == "gemini":
		return chatSearchGemini(ctx, inst, needle)
	case inst.Tool == "opencode":
		return chatSearchOpenCode(ctx, inst, needle)
	case inst.Tool == "kiro":
		return chatSearchKiro(ctx, inst, needle)
	case inst.Tool == "copilot":
		if !IsBareClaudeSessionUUID(inst.CopilotSessionID) {
			return "", false, errChatSearchIdentity
		}
		return chatSearchJSONL(ctx, filepath.Join(getCopilotSessionStateDir(), inst.CopilotSessionID, "events.jsonl"), "copilot", inst.CopilotSessionID, needle)
	case inst.Tool == "pi":
		if !validHookSessionAnchorInstanceID(inst.ID) {
			return "", false, errChatSearchIdentity
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, err
		}
		root := filepath.Join(home, ".pi", "agent-deck", inst.ID)
		if info, err := os.Lstat(root); err != nil {
			return "", false, err
		} else if info.Mode()&os.ModeSymlink != 0 {
			return "", false, errChatSearchIdentity
		}
		path, err := chatSearchUniqueFile(ctx, root, 32, true, func(path string) bool {
			return filepath.Ext(path) == ".jsonl"
		})
		if err != nil {
			return "", errors.Is(err, errChatSearchLimit), err
		}
		return chatSearchJSONL(ctx, path, "pi", "", needle)
	default:
		return "", false, errChatSearchFormat
	}
}

// chatSearchWalk reads directory entries in batches instead of Glob/WalkDir's
// all-at-once directory allocation. It never descends through a symlink.
func chatSearchWalk(ctx context.Context, root string, depth int, failOnDepthLimit bool, visit func(string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, err := os.Open(root)
	if err != nil {
		return err
	}
	defer dir.Close()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, readErr := dir.ReadDir(64)
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			path := filepath.Join(root, entry.Name())
			if entry.IsDir() {
				if depth > 0 {
					if err := chatSearchWalk(ctx, path, depth-1, failOnDepthLimit, visit); err != nil {
						return err
					}
				} else if failOnDepthLimit {
					return errChatSearchLimit
				}
			} else if err := visit(path); err != nil {
				return err
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}
	}
}

func chatSearchUniqueFile(ctx context.Context, root string, depth int, failOnDepthLimit bool, matches func(string) bool) (string, error) {
	var found string
	err := chatSearchWalk(ctx, root, depth, failOnDepthLimit, func(path string) error {
		if !matches(path) {
			return nil
		}
		if found != "" {
			return errChatSearchAmbiguous
		}
		found = path
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", os.ErrNotExist
	}
	return found, nil
}

func chatSearchClaudePath(ctx context.Context, configDir, projectPath, id string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Mirror resolveClaudeTranscriptPath's physical-then-lexical cwd encoding,
	// but make its exact-ID fallback cancellable and reject ambiguous matches.
	resolved := projectPath
	if physical, err := filepath.EvalSymlinks(projectPath); err == nil {
		resolved = physical
	}
	for _, candidate := range []string{resolved, projectPath} {
		if candidate == "" {
			continue
		}
		path := filepath.Join(configDir, "projects", ConvertToClaudeDirName(candidate), id+".jsonl")
		if _, err := os.Lstat(path); err == nil {
			return path, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	return chatSearchUniqueFile(ctx, filepath.Join(configDir, "projects"), 1, false, func(path string) bool {
		return filepath.Base(path) == id+".jsonl"
	})
}

// Every read, including draining a single oversized JSON value/line, remains
// cancellable and bounded. The limit applies per saved conversation, not corpus.
type chatSearchInput struct {
	ctx       context.Context
	r         io.Reader
	remaining int64
}

func (r *chatSearchInput) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if r.remaining == 0 {
		return 0, errChatSearchLimit
	}
	if int64(len(p)) > r.remaining {
		p = p[:int(r.remaining)]
	}
	n, err := r.r.Read(p)
	r.remaining -= int64(n)
	return n, err
}

func chatSearchReader(ctx context.Context, r io.Reader) *bufio.Reader {
	return bufio.NewReaderSize(&chatSearchInput{ctx: ctx, r: r, remaining: chatSearchFileLimit}, 32*1024)
}

func chatSearchReadLine(reader *bufio.Reader, line []byte) ([]byte, bool, error) {
	line = line[:0]
	oversized := false
	for {
		fragment, err := reader.ReadSlice('\n')
		if !oversized {
			if len(line)+len(fragment) > chatSearchRecordLimit {
				oversized = true
				line = line[:0]
			} else {
				line = append(line, fragment...)
			}
		}
		if err != bufio.ErrBufferFull {
			return line, oversized, err
		}
	}
}

func chatSearchJSONL(ctx context.Context, path, kind, expectedID, needle string) (string, bool, error) {
	file, err := openOmpRegularFile(path)
	if err != nil {
		return "", false, err
	}
	defer file.Close()
	reader := chatSearchReader(ctx, file)
	var line []byte
	partial, identity, recognized := false, kind == "claude", false
	if kind == "kiro" {
		identity = true // The separate, bounded saved header corroborates its ID.
	}
	var previousMessageID, previousText string
	contentSeen := false
	for count := 0; ; count++ {
		if err := ctx.Err(); err != nil {
			return "", true, err
		}
		var oversized bool
		line, oversized, err = chatSearchReadLine(reader, line)
		partial = partial || oversized
		if err != nil && err != io.EOF {
			return "", partial || errors.Is(err, errChatSearchLimit), err
		}
		if oversized {
			previousMessageID, previousText = "", ""
		}
		trimmed := bytes.TrimSpace(line)
		contentSeen = contentSeen || oversized || len(trimmed) != 0
		if len(trimmed) != 0 {
			var record struct {
				Type      string `json:"type"`
				ID        string `json:"id"`
				SessionID string `json:"sessionId"`
				Kind      string `json:"kind"`
				Data      struct {
					SessionID string          `json:"sessionId"`
					MessageID string          `json:"message_id"`
					Content   json.RawMessage `json:"content"`
				} `json:"data"`
				Message struct {
					Role    string          `json:"role"`
					Content json.RawMessage `json:"content"`
				} `json:"message"`
				Payload struct {
					Type    string `json:"type"`
					ID      string `json:"id"`
					Role    string `json:"role"`
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"payload"`
			}
			if json.Unmarshal(line, &record) != nil {
				partial = true
				previousMessageID, previousText = "", ""
			} else {
				var text string
				switch kind {
				case "omp", "pi":
					if record.Type == "session" {
						if count >= 32 || record.ID == "" || (expectedID != "" && record.ID != expectedID) {
							return "", partial, errChatSearchIdentity
						}
						identity, recognized = true, true
					}
					if record.Type == "message" && (record.Message.Role == "user" || record.Message.Role == "assistant") {
						if !identity {
							return "", partial, errChatSearchIdentity
						}
						text = transcriptText(record.Message.Content)
					}
				case "claude":
					if record.SessionID != "" && !strings.EqualFold(record.SessionID, expectedID) {
						return "", partial, errChatSearchIdentity
					}
					if record.Type == "user" || record.Type == "assistant" || record.Type == "system" {
						recognized = true
						if message, ok := parseClaudeTranscriptLine(line); ok && (message.Role == "user" || message.Role == "assistant") {
							text = message.Content
						}
					}
				case "codex":
					if record.Type == "session_meta" {
						if !strings.EqualFold(record.Payload.ID, expectedID) {
							return "", partial, errChatSearchIdentity
						}
						identity, recognized = true, true
					}
					if record.Type == "response_item" && record.Payload.Type == "message" && (record.Payload.Role == "user" || record.Payload.Role == "assistant") {
						if !identity {
							return "", partial, errChatSearchIdentity
						}
						var content strings.Builder
						for _, block := range record.Payload.Content {
							if block.Type == "input_text" || block.Type == "output_text" {
								content.WriteString(block.Text)
								content.WriteByte('\n')
							}
						}
						text = content.String()
					}
				case "copilot":
					if record.Type == "session.start" {
						if !strings.EqualFold(record.Data.SessionID, expectedID) {
							return "", partial, errChatSearchIdentity
						}
						identity, recognized = true, true
					}
					if record.Type == "user.message" || record.Type == "assistant.message" {
						if !identity {
							return "", partial, errChatSearchIdentity
						}
						if json.Unmarshal(record.Data.Content, &text) != nil {
							partial = true
						}
					}
				case "kiro":
					if record.Kind == "Prompt" || record.Kind == "AssistantMessage" {
						recognized = true
						text = chatSearchKiroContent(record.Data.Content)
						if record.Kind == "AssistantMessage" && record.Data.MessageID != "" {
							if record.Data.MessageID == previousMessageID {
								text = previousText + text
							}
							previousMessageID = record.Data.MessageID
							// Keep only the query overlap and snippet context, not
							// an ever-growing streamed assistant response.
							keep := len(needle) + chatSearchSnippetRunes*utf8.UTFMax
							if keep > chatSearchRecordLimit {
								keep = chatSearchRecordLimit
							}
							start := len(text) - keep
							if start < 0 {
								start = 0
							}
							for start < len(text) && !utf8.RuneStart(text[start]) {
								start++
							}
							previousText = text[start:]
							if start > 0 {
								previousText = strings.Clone(previousText)
							}
						} else {
							previousMessageID, previousText = "", ""
						}
					}
				}
				if text != "" {
					if snippet := chatSearchSnippet(text, needle); snippet != "" {
						return snippet, partial, nil
					}
				}
			}
		}
		if err != nil {
			if !contentSeen {
				return "", partial, errChatSearchNotSaved
			}
			if !identity {
				return "", partial, errChatSearchIdentity
			}
			if !recognized {
				return "", partial, errChatSearchFormat
			}
			return "", partial, nil
		}
	}
}

func chatSearchSnippet(text, needle string) string {
	// Strip complete escape sequences (including OSC clipboard/hyperlinks),
	// then flatten whitespace and discard remaining C0/C1/bidi controls.
	text = ansi.Strip(text)
	var clean strings.Builder
	space := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			space = clean.Len() > 0
			continue
		}
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			continue
		}
		if space {
			clean.WriteByte(' ')
			space = false
		}
		clean.WriteRune(r)
	}
	plain := clean.String()
	lower := strings.ToLower(plain)
	index := strings.Index(lower, needle)
	if index < 0 {
		return ""
	}
	// Unicode lowercasing preserves rune count, not necessarily byte offsets.
	matchRune := utf8.RuneCountInString(lower[:index])
	start := matchRune - 60
	if start < 0 {
		start = 0
	}
	end := start + chatSearchSnippetRunes
	startByte, endByte, position := 0, len(plain), 0
	for offset := range plain {
		if position == start {
			startByte = offset
		}
		if position == end {
			endByte = offset
			break
		}
		position++
	}
	prefix, suffix := "", ""
	if startByte > 0 {
		prefix = "…"
	}
	if endByte < len(plain) {
		suffix = "…"
	}
	return prefix + plain[startByte:endByte] + suffix
}

func chatSearchDataHome() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" && filepath.IsAbs(dir) {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	return filepath.Join(home, ".local", "share"), err
}

func chatSearchOpenCode(ctx context.Context, inst *Instance, needle string) (string, bool, error) {
	id := inst.OpenCodeSessionID
	if !strings.HasPrefix(id, "ses_") || strings.ContainsAny(id, "/\\\x00\r\n") {
		return "", false, errChatSearchIdentity
	}
	root, err := chatSearchDataHome()
	if err != nil {
		return "", false, err
	}
	path := filepath.Join(root, "opencode", "opencode.db")
	if info, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "", false, errChatSearchOpenCodeStore
		}
		return "", false, err
	} else if !info.Mode().IsRegular() {
		return "", false, errChatSearchFormat
	}
	// Reuse the existing SQLite driver/URI convention, but use a short busy
	// timeout for an interactive query. mode=ro keeps live WAL visibility; do
	// not use immutable=1 or create a missing provider database.
	dsn := url.URL{Scheme: "file", Path: path}
	params := dsn.Query()
	params.Set("mode", "ro")
	params.Add("_pragma", "busy_timeout(100)")
	dsn.RawQuery = params.Encode()
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return "", false, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT 1 FROM session WHERE id = ?`, id).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, errChatSearchIdentity
		}
		return "", false, err
	}
	// Bound rows before they cross the driver boundary; reasoning, tool data,
	// and records belonging to another session never enter the text matcher.
	rows, err := db.QueryContext(ctx, `
		SELECT CASE WHEN length(CAST(m.data AS BLOB)) <= ?1 AND json_valid(m.data)
		            THEN json_extract(m.data, '$.role') ELSE NULL END,
		       CASE WHEN length(CAST(p.data AS BLOB)) <= ?1 THEN p.data ELSE NULL END
		FROM part p JOIN message m ON m.id = p.message_id AND m.session_id = p.session_id
		WHERE p.session_id = ?2`, chatSearchRecordLimit, id)
	if err != nil {
		return "", false, err
	}
	defer rows.Close()
	partial := false
	var scanned int64
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return "", true, err
		}
		var role, raw sql.NullString
		if err := rows.Scan(&role, &raw); err != nil {
			return "", partial, err
		}
		if !role.Valid || !raw.Valid {
			partial = true
			scanned += chatSearchRecordLimit
		} else {
			scanned += int64(len(raw.String))
		}
		if scanned > chatSearchFileLimit {
			return "", true, errChatSearchLimit
		}
		if !raw.Valid || !role.Valid || (role.String != "user" && role.String != "assistant") {
			continue
		}
		var part struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal([]byte(raw.String), &part) != nil {
			partial = true
			continue
		}
		if part.Type == "text" {
			if snippet := chatSearchSnippet(part.Text, needle); snippet != "" {
				return snippet, partial, nil
			}
		}
	}
	return "", partial, rows.Err()
}

func chatSearchKiro(ctx context.Context, inst *Instance, needle string) (string, bool, error) {
	id := inst.KiroSessionID
	if !kiroSessionUUIDPattern.MatchString(id) {
		return "", false, errChatSearchFormat
	}
	if err := ctx.Err(); err != nil {
		return "", true, err
	}
	dir := KiroSessionsDir()
	raw, err := readOmpBoundedRegularFile(filepath.Join(dir, id+".json"), chatSearchRecordLimit)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, errChatSearchKiroStore
		}
		return "", false, err
	}
	var header struct {
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal(raw, &header) != nil || !strings.EqualFold(header.SessionID, id) {
		return "", false, errChatSearchIdentity
	}
	return chatSearchJSONL(ctx, filepath.Join(dir, id+".jsonl"), "kiro", id, needle)
}

func chatSearchKiroContent(raw json.RawMessage) string {
	var parts []struct {
		Kind string          `json:"kind"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var text strings.Builder
	for _, part := range parts {
		if part.Kind == "text" {
			var content string
			if json.Unmarshal(part.Data, &content) == nil {
				text.WriteString(content)
			}
		}
	}
	return text.String()
}

func chatSearchGemini(ctx context.Context, inst *Instance, needle string) (string, bool, error) {
	id := inst.GeminiSessionID
	if !IsBareClaudeSessionUUID(id) {
		return "", false, errChatSearchIdentity
	}
	// The saved filename has only eight UUID characters. Verify the FULL
	// sessionId inside every candidate before accepting even a matching snippet.
	var found, snippet string
	partial := false
	visit := func(path string) error {
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "session-") || !strings.HasSuffix(base, "-"+id[:8]+".json") {
			return nil
		}
		text, skipped, err := chatSearchJSONDocument(ctx, path, id, "sessionId", "messages", func(raw []byte) string {
			var message struct {
				Type    string          `json:"type"`
				Content json.RawMessage `json:"content"`
			}
			if json.Unmarshal(raw, &message) != nil || (message.Type != "user" && message.Type != "gemini") {
				return ""
			}
			// Current Gemini uses strings; newer content arrays use text parts.
			return transcriptText(message.Content)
		}, needle)
		if errors.Is(err, errChatSearchIdentity) {
			return nil
		}
		partial = partial || skipped
		if err != nil {
			return err
		}
		if found != "" {
			return errChatSearchAmbiguous
		}
		found, snippet = path, text
		return nil
	}
	root := GetGeminiSessionsDir(inst.EffectiveWorkingDir())
	err := chatSearchWalk(ctx, root, 0, false, visit)
	if err != nil && !os.IsNotExist(err) {
		return "", partial, err
	}
	if found == "" {
		// Encoding changes (including WSL) may move a session to a different
		// project directory; identity, never recency or cwd, remains authority.
		err = chatSearchWalk(ctx, filepath.Join(GetGeminiConfigDir(), "tmp"), 2, false, visit)
		if err != nil {
			return "", partial || errors.Is(err, errChatSearchLimit), err
		}
	}
	if found == "" {
		return "", partial, errChatSearchIdentity
	}
	return snippet, partial, nil
}

// chatSearchJSONValue consumes one complete JSON value without allowing a
// decoder to allocate an unbounded string/RawMessage. Oversized values are
// drained, so a later message in a single saved JSON document is still searched.
func chatSearchJSONValue(reader *bufio.Reader, limit int) ([]byte, bool, error) {
	first, err := chatSearchNonspace(reader)
	if err != nil {
		return nil, false, err
	}
	var value []byte
	oversized := false
	appendByte := func(b byte) {
		if len(value) < limit {
			value = append(value, b)
		} else {
			oversized = true
		}
	}
	appendByte(first)
	quoted, escaped := first == '"', false
	depth := 0
	if first == '{' || first == '[' {
		depth = 1
	}
	primitive := depth == 0 && !quoted
	for {
		if !primitive && depth == 0 && !quoted {
			return value, oversized, nil
		}
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF && primitive {
				return value, oversized, nil
			}
			return nil, oversized, err
		}
		if primitive && (b == ',' || b == ']' || b == '}' || b == ' ' || b == '\n' || b == '\r' || b == '\t') {
			if err := reader.UnreadByte(); err != nil {
				return nil, oversized, err
			}
			return value, oversized, nil
		}
		appendByte(b)
		if quoted {
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				quoted = false
			}
			continue
		}
		switch b {
		case '"':
			quoted = true
		case '{', '[':
			depth++
			if depth > 128 {
				return nil, oversized, errChatSearchFormat
			}
		case '}', ']':
			depth--
		}
	}
}

func chatSearchNonspace(reader *bufio.Reader) (byte, error) {
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		if b != ' ' && b != '\n' && b != '\r' && b != '\t' {
			return b, nil
		}
	}
}

func chatSearchJSONDocument(ctx context.Context, path, expectedID, idField, messagesField string, text func([]byte) string, needle string) (string, bool, error) {
	file, err := openOmpRegularFile(path)
	if err != nil {
		return "", false, err
	}
	defer file.Close()
	reader := chatSearchReader(ctx, file)
	b, err := chatSearchNonspace(reader)
	if err != nil || b != '{' {
		return "", false, errChatSearchFormat
	}
	var id, snippet string
	partial, messagesSeen := false, false
	for {
		if err := ctx.Err(); err != nil {
			return "", true, err
		}
		b, err := chatSearchNonspace(reader)
		if err != nil {
			return "", partial || errors.Is(err, errChatSearchLimit), err
		}
		if b == '}' {
			break
		}
		if err := reader.UnreadByte(); err != nil {
			return "", partial, err
		}
		raw, oversized, err := chatSearchJSONValue(reader, 4096)
		var key string
		if err != nil || oversized || json.Unmarshal(raw, &key) != nil {
			if err != nil {
				return "", partial || errors.Is(err, errChatSearchLimit), err
			}
			return "", partial, errChatSearchFormat
		}
		if b, err = chatSearchNonspace(reader); err != nil || b != ':' {
			return "", partial, errChatSearchFormat
		}
		if key == messagesField {
			messagesSeen = true
			if b, err = chatSearchNonspace(reader); err != nil || b != '[' {
				return "", partial, errChatSearchFormat
			}
			for {
				if b, err = chatSearchNonspace(reader); err != nil {
					return "", partial || errors.Is(err, errChatSearchLimit), err
				}
				if b == ']' {
					break
				}
				if err := reader.UnreadByte(); err != nil {
					return "", partial, err
				}
				limit := chatSearchRecordLimit
				if snippet != "" {
					// A session already matches. Only drain its remaining
					// messages to verify an identity field appearing later.
					limit = 0
				}
				raw, oversized, err = chatSearchJSONValue(reader, limit)
				if limit != 0 {
					partial = partial || oversized
				}
				if err != nil {
					return "", partial || errors.Is(err, errChatSearchLimit), err
				}
				if limit != 0 && !oversized {
					if !json.Valid(raw) {
						partial = true
					} else {
						snippet = chatSearchSnippet(text(raw), needle)
					}
				}
				if b, err = chatSearchNonspace(reader); err != nil || (b != ',' && b != ']') {
					return "", partial || errors.Is(err, errChatSearchLimit), errChatSearchFormat
				}
				if b == ']' {
					break
				}
			}
		} else {
			limit := 0
			if key == idField {
				limit = 4096
			}
			raw, oversized, err = chatSearchJSONValue(reader, limit)
			if err != nil {
				return "", partial || errors.Is(err, errChatSearchLimit), err
			}
			if key == idField && (oversized || json.Unmarshal(raw, &id) != nil) {
				return "", partial, errChatSearchIdentity
			}
		}
		if b, err = chatSearchNonspace(reader); err != nil || (b != ',' && b != '}') {
			return "", partial || errors.Is(err, errChatSearchLimit), errChatSearchFormat
		}
		if b == '}' {
			break
		}
	}
	if !strings.EqualFold(id, expectedID) {
		return "", partial, errChatSearchIdentity
	}
	if !messagesSeen {
		return "", partial, errChatSearchFormat
	}
	return snippet, partial, nil
}
