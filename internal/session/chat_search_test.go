package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
)

const chatSearchTestClaudeID = "11111111-1111-4111-8111-111111111111"
const chatSearchTestCodexID = "22222222-2222-4222-8222-222222222222"
const chatSearchTestForeignID = "33333333-3333-4333-8333-333333333333"

func chatSearchTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("AGENTDECK_PROFILE", "")
	ClearUserConfigCache()
	t.Cleanup(ClearUserConfigCache)
	return home
}

func chatSearchTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func chatSearchTestRollout(t *testing.T, home, id, content string) string {
	t.Helper()
	path := filepath.Join(home, "sessions", "2026", "10", "19", "rollout-2026-10-19T09-00-00-"+id+".jsonl")
	chatSearchTestFile(t, path, fmt.Sprintf("{\"type\":\"session_meta\",\"payload\":{\"id\":%q,\"cwd\":\"/same/project\",\"source\":\"cli\"}}\n", id)+content)
	return path
}

func chatSearchTestMessage(t *testing.T, role, text string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"type": "message", "message": map[string]any{
			"role": role, "content": []map[string]string{{"type": "text", "text": text}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(body) + "\n"
}

func chatSearchTestWantMatch(t *testing.T, report ChatSearchReport, id, text string) {
	t.Helper()
	if len(report.Matches) != 1 || report.Matches[0].InstanceID != id || !strings.Contains(report.Matches[0].Snippet, text) {
		t.Fatalf("expected one %q match containing %q, got %+v", id, text, report)
	}
}

// A newest-by-project lookup would match the foreign files in all three cases.
func TestChatSearchExactSavedBindings(t *testing.T) {
	for _, tool := range []string{"omp", "codex", "claude"} {
		t.Run(tool, func(t *testing.T) {
			home := chatSearchTestHome(t)
			inst := &Instance{ID: "row-exact", Tool: tool, ProjectPath: "/same/project", Command: tool}
			own := "bound conversation violet lighthouse"
			foreign := "foreign conversation secret needle"
			switch tool {
			case "omp":
				dir := filepath.Join(home, ".omp", "agent-deck", inst.ID)
				path := filepath.Join(dir, "bound.jsonl")
				chatSearchTestFile(t, path, "{\"type\":\"title\",\"title\":\"title\"}\n{\"type\":\"session\",\"id\":\"omp-bound\"}\n"+chatSearchTestMessage(t, "user", own))
				chatSearchTestFile(t, filepath.Join(dir, "newer.jsonl"), "{\"type\":\"session\",\"id\":\"omp-foreign\"}\n"+chatSearchTestMessage(t, "assistant", foreign))
				chatSearchTestFile(t, filepath.Join(dir, "bound", "task.jsonl"), chatSearchTestMessage(t, "assistant", foreign))
				writeOmpValidationBindingAt(t, dir, ompActiveBindingName+".launch-1", path, "omp-bound", "saved", "launch-1")
				chatSearchTestFile(t, filepath.Join(dir, ".agent-deck-launch-generation"), "launch-1\n")
				chatSearchTestFile(t, filepath.Join(dir, ".agent-deck-omp-status.launch-1.json"), fmt.Sprintf(`{"instance_id":%q,"launch_id":"launch-1","identity_ready":true,"session_id":"omp-bound","session_file":%q,"error":""}`, inst.ID, path))
			case "codex":
				inst.CodexSessionID = chatSearchTestCodexID
				chatSearchTestRollout(t, filepath.Join(home, ".codex"), chatSearchTestCodexID, `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"`+own+`"}]}}`+"\n")
				chatSearchTestRollout(t, filepath.Join(home, ".codex"), chatSearchTestForeignID, `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"`+foreign+`"}]}}`+"\n")
			case "claude":
				inst.ClaudeSessionID = chatSearchTestClaudeID
				dir := filepath.Join(home, ".claude", "projects", "-same-project")
				chatSearchTestFile(t, filepath.Join(dir, inst.ClaudeSessionID+".jsonl"), `{"type":"user","sessionId":"`+inst.ClaudeSessionID+`","message":{"role":"user","content":"`+own+`"}}`+"\n")
				chatSearchTestFile(t, filepath.Join(dir, chatSearchTestForeignID+".jsonl"), `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"`+foreign+`"}]}}`+"\n")
			}
			binding := NewChatSearchBinding(inst)
			report := SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "VIOLET LIGHTHOUSE")
			chatSearchTestWantMatch(t, report, "row-exact", "violet lighthouse")
			if report.Partial || len(report.Unavailable) != 0 {
				t.Fatalf("complete fixture was reported incomplete: %+v", report)
			}
			report = SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "secret needle")
			if len(report.Matches) != 0 || len(report.Unavailable) != 0 || report.Partial {
				t.Fatalf("foreign conversation leaked or own transcript became unavailable: %+v", report)
			}
		})
	}
}

func TestChatSearchSnapshotRetainsAccountAndIdentity(t *testing.T) {
	home := chatSearchTestHome(t)
	chatSearchTestFile(t, filepath.Join(home, ".agent-deck", "config.toml"), "[profiles.work.claude]\nconfig_dir = \"~/.claude-work\"\n[profiles.work.codex]\nconfig_dir = \"~/.codex-work\"\n")
	ClearUserConfigCache()
	for _, tool := range []string{"claude", "codex"} {
		inst := &Instance{ID: "original-" + tool, Tool: tool, Account: "work", ProjectPath: "/same/project", Command: tool, ClaudeSessionID: chatSearchTestClaudeID, CodexSessionID: chatSearchTestCodexID}
		binding := NewChatSearchBinding(inst)
		// Create the corpus after snapshotting: binding construction needs no file.
		if tool == "claude" {
			chatSearchTestFile(t, filepath.Join(home, ".claude-work", "projects", "-same-project", chatSearchTestClaudeID+".jsonl"), `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"account-bound answer"}]}}`+"\n")
		} else {
			chatSearchTestRollout(t, filepath.Join(home, ".codex-work"), chatSearchTestCodexID, `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"account-bound answer"}]}}`+"\n")
		}
		inst.mu.Lock()
		inst.ID, inst.Tool, inst.Account, inst.ProjectPath = "changed", "shell", "personal", "/foreign"
		inst.ClaudeSessionID, inst.CodexSessionID = chatSearchTestForeignID, chatSearchTestForeignID
		inst.mu.Unlock()
		chatSearchTestWantMatch(t, SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "account-bound"), "original-"+tool, "account-bound answer")
	}
}

func TestChatSearchCodexCommandHomeAndRolloutIdentity(t *testing.T) {
	for _, archived := range []bool{false, true} {
		t.Run(fmt.Sprintf("archived=%t", archived), func(t *testing.T) {
			home := chatSearchTestHome(t)
			explicit := filepath.Join(home, "explicit codex home")
			inst := &Instance{ID: "codex-row", Tool: "codex", Command: "CODEX_HOME='" + explicit + "' codex", CodexSessionID: chatSearchTestCodexID}
			path := chatSearchTestRollout(t, explicit, chatSearchTestCodexID, `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"explicit-home answer"}]}}`+"\n")
			if archived {
				dir := filepath.Join(explicit, "archived_sessions")
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				destination := filepath.Join(dir, filepath.Base(path))
				if err := os.Rename(path, destination); err != nil {
					t.Fatal(err)
				}
				path = destination
			}
			binding := NewChatSearchBinding(inst)
			chatSearchTestWantMatch(t, SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "explicit-home"), inst.ID, "explicit-home answer")
			chatSearchTestFile(t, path, `{"type":"session_meta","payload":{"id":"`+chatSearchTestForeignID+`"}}`+"\n"+`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"explicit-home foreign answer"}]}}`+"\n")
			report := SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "explicit-home")
			if len(report.Matches) != 0 || len(report.Unavailable) != 1 {
				t.Fatalf("filename alone must not authorize a mismatched rollout: %+v", report)
			}
		})
	}
}

func TestChatSearchCodexArchivedExactBinding(t *testing.T) {
	for _, liveSessions := range []bool{false, true} {
		t.Run(fmt.Sprintf("live-sessions=%t", liveSessions), func(t *testing.T) {
			home := chatSearchTestHome(t)
			codexHome := filepath.Join(home, ".codex")
			inst := &Instance{ID: "archived-row", Tool: "codex", ProjectPath: "/same/project", CodexSessionID: chatSearchTestCodexID}
			archive := filepath.Join(codexHome, "archived_sessions")
			chatSearchTestFile(t, filepath.Join(archive, "rollout-2026-10-18T09-00-00-"+chatSearchTestCodexID+".jsonl"),
				`{"type":"session_meta","payload":{"id":"`+chatSearchTestCodexID+`","cwd":"/same/project","source":"cli"}}`+"\n"+
					`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"archived violet lighthouse"}]}}`+"\n")
			foreign := `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"foreign secret needle"}]}}` + "\n"
			chatSearchTestFile(t, filepath.Join(archive, "rollout-2026-10-19T09-00-00-"+chatSearchTestForeignID+".jsonl"),
				`{"type":"session_meta","payload":{"id":"`+chatSearchTestForeignID+`","cwd":"/same/project","source":"cli"}}`+"\n"+foreign)
			if liveSessions {
				chatSearchTestRollout(t, codexHome, chatSearchTestForeignID, foreign)
			}
			binding := NewChatSearchBinding(inst)
			report := SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "violet lighthouse")
			chatSearchTestWantMatch(t, report, inst.ID, "archived violet lighthouse")
			if report.Partial || len(report.Unavailable) != 0 {
				t.Fatalf("exact archived conversation was reported incomplete: %+v", report)
			}
			report = SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "secret needle")
			if len(report.Matches) != 0 || len(report.Unavailable) != 0 || report.Partial {
				t.Fatalf("foreign conversation leaked or exact archive became unavailable: %+v", report)
			}
		})
	}
}

func TestChatSearchCodexArchivePreservesAmbiguityRefusal(t *testing.T) {
	for _, ambiguousStore := range []string{"sessions", "archived_sessions"} {
		t.Run(ambiguousStore, func(t *testing.T) {
			home := chatSearchTestHome(t)
			codexHome := filepath.Join(home, ".codex")
			inst := &Instance{ID: "ambiguous-row", Tool: "codex", CodexSessionID: chatSearchTestCodexID}
			body := `{"type":"session_meta","payload":{"id":"` + chatSearchTestCodexID + `"}}` + "\n" +
				`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ambiguous archive needle"}]}}` + "\n"
			chatSearchTestFile(t, filepath.Join(codexHome, "archived_sessions", "rollout-2026-10-18T09-00-00-"+chatSearchTestCodexID+".jsonl"), body)
			if ambiguousStore == "sessions" {
				chatSearchTestRollout(t, codexHome, chatSearchTestCodexID, "")
			}
			dir := filepath.Join(codexHome, ambiguousStore)
			if ambiguousStore == "sessions" {
				dir = filepath.Join(dir, "2026", "10", "19")
			}
			chatSearchTestFile(t, filepath.Join(dir, "rollout-2026-10-19T10-00-00-"+chatSearchTestCodexID+".jsonl"), body)
			report := SearchLocalChats(context.Background(), []ChatSearchBinding{NewChatSearchBinding(inst)}, "needle")
			if len(report.Matches) != 0 || len(report.Unavailable) != 1 || report.Unavailable[0].Reason != "saved conversation binding ambiguous" || report.Partial {
				t.Fatalf("ambiguous exact identity must not select an archive: %+v", report)
			}
		})
	}
}

func TestChatSearchRejectsNonlocalAndUnsupportedBindings(t *testing.T) {
	home := chatSearchTestHome(t)
	chatSearchTestFile(t, filepath.Join(home, ".claude", "projects", "-same-project", chatSearchTestClaudeID+".jsonl"), `{"type":"assistant","message":{"role":"assistant","content":"local private needle"}}`+"\n")
	chatSearchTestRollout(t, filepath.Join(home, ".codex"), chatSearchTestCodexID, `{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"local private needle"}]}}`+"\n")
	instances := []*Instance{
		{ID: "ssh", Tool: "claude", SSHHost: "remote", ClaudeSessionID: chatSearchTestClaudeID, ProjectPath: "/same/project"},
		{ID: "docker", Tool: "codex", CodexSessionID: chatSearchTestCodexID, Sandbox: &SandboxConfig{Enabled: true}},
		{ID: "unknown", Tool: "unknown-private-tool"},
		{ID: "no-id", Tool: "claude", ProjectPath: "/same/project"},
		{ID: "unsafe-id", Tool: "claude", ClaudeSessionID: "../../secret"},
	}
	bindings := make([]ChatSearchBinding, 0, len(instances))
	for _, inst := range instances {
		bindings = append(bindings, NewChatSearchBinding(inst))
	}
	report := SearchLocalChats(context.Background(), bindings, "needle")
	if len(report.Matches) != 0 || len(report.Unavailable) != len(instances) {
		t.Fatalf("nonlocal or unsupported binding silently treated as searchable: %+v", report)
	}
	for idx, unavailable := range report.Unavailable {
		if unavailable.InstanceID != instances[idx].ID || unavailable.Tool != instances[idx].Tool || unavailable.Reason == "" || strings.Contains(unavailable.Reason, "secret") {
			t.Fatalf("unsafe or misattributed status: %+v", unavailable)
		}
	}
}

func TestChatSearchOMPRequiresAcknowledgedCurrentBinding(t *testing.T) {
	for _, state := range []string{"unbound", "unacknowledged", "wrong-header", "pending", "fresh-boundary"} {
		t.Run(state, func(t *testing.T) {
			home := chatSearchTestHome(t)
			inst := &Instance{ID: "omp-row", Tool: "omp"}
			dir := filepath.Join(home, ".omp", "agent-deck", inst.ID)
			path := filepath.Join(dir, "conversation.jsonl")
			chatSearchTestFile(t, path, "{\"type\":\"session\",\"id\":\"bound\"}\n"+chatSearchTestMessage(t, "assistant", "never borrow this needle"))
			if state != "unbound" {
				id := "bound"
				if state == "wrong-header" {
					id = "another-conversation"
				}
				if state == "pending" {
					path = filepath.Join(dir, "not-saved-yet.jsonl")
				}
				writeOmpValidationBinding(t, dir, path, id, map[bool]string{true: "pending", false: "saved"}[state == "pending"], "legacy")
			}
			if state == "unacknowledged" {
				chatSearchTestFile(t, filepath.Join(dir, ".agent-deck-launch-generation"), "new-launch\n")
			}
			if state == "fresh-boundary" {
				chatSearchTestFile(t, filepath.Join(dir, ".agent-deck-fresh-pending"), "new-launch\n")
			}
			report := SearchLocalChats(context.Background(), []ChatSearchBinding{NewChatSearchBinding(inst)}, "needle")
			if len(report.Matches) != 0 || len(report.Unavailable) != 1 {
				t.Fatalf("unbound conversation must not be borrowed: %+v", report)
			}
		})
	}
}

func TestChatSearchClaudeSkipsSidechainsAndThinking(t *testing.T) {
	home := chatSearchTestHome(t)
	inst := &Instance{ID: "claude", Tool: "claude", ProjectPath: "/same/project", ClaudeSessionID: chatSearchTestClaudeID}
	path := filepath.Join(home, ".claude", "projects", "-same-project", inst.ClaudeSessionID+".jsonl")
	chatSearchTestFile(t, path, `{"type":"assistant","isSidechain":true,"message":{"role":"assistant","content":[{"type":"text","text":"private sidechain needle"}]}}`+"\n"+`{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"private reasoning needle"},{"type":"text","text":"visible main answer"}]}}`+"\n")
	binding := NewChatSearchBinding(inst)
	report := SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "private")
	if len(report.Matches) != 0 || len(report.Unavailable) != 0 {
		t.Fatalf("sidechain or reasoning was exposed: %+v", report)
	}
	chatSearchTestWantMatch(t, SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "visible main"), inst.ID, "visible main answer")
}

func TestChatSearchContinuesAfterOversizedRecord(t *testing.T) {
	home := chatSearchTestHome(t)
	inst := &Instance{ID: "oversized", Tool: "claude", ProjectPath: "/same/project", ClaudeSessionID: chatSearchTestClaudeID}
	path := filepath.Join(home, ".claude", "projects", "-same-project", inst.ClaudeSessionID+".jsonl")
	chatSearchTestFile(t, path, `{"type":"assistant","message":{"role":"assistant","content":"`+strings.Repeat("x", 2*1024*1024)+`"}}`+"\n"+`{"type":"assistant","message":{"role":"assistant","content":"later recoverable needle"}}`+"\n")
	report := SearchLocalChats(context.Background(), []ChatSearchBinding{NewChatSearchBinding(inst)}, "recoverable")
	chatSearchTestWantMatch(t, report, inst.ID, "recoverable needle")
	if !report.Partial {
		t.Fatal("skipped oversized records must be reported as partial")
	}
}

func TestChatSearchSnippetIsBoundedCenteredAndTerminalSafe(t *testing.T) {
	home := chatSearchTestHome(t)
	inst := &Instance{ID: "snippet", Tool: "omp"}
	dir := filepath.Join(home, ".omp", "agent-deck", inst.ID)
	path := filepath.Join(dir, "root.jsonl")
	text := strings.Repeat("前", 400) + "\x1b[31mViolet Lighthouse\x1b[0m\x1b]52;c;secret\a\n\t\x00\u202edone " + strings.Repeat("後", 400)
	chatSearchTestFile(t, path, "{\"type\":\"session\",\"id\":\"snippet-root\"}\n"+chatSearchTestMessage(t, "assistant", text))
	writeOmpValidationBinding(t, dir, path, "snippet-root", "saved", "legacy")
	report := SearchLocalChats(context.Background(), []ChatSearchBinding{NewChatSearchBinding(inst)}, "violet lighthouse")
	chatSearchTestWantMatch(t, report, inst.ID, "Violet Lighthouse")
	snippet := report.Matches[0].Snippet
	if !utf8.ValidString(snippet) || utf8.RuneCountInString(snippet) > 240 {
		t.Fatalf("snippet is not a bounded UTF-8 window: %q", snippet)
	}
	for _, r := range snippet {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			t.Fatalf("terminal or display control remains: %q", snippet)
		}
	}
	if strings.Contains(snippet, "secret") {
		t.Fatalf("OSC payload leaked into snippet: %q", snippet)
	}
}

func TestChatSearchCanceledBeforeFilesystemWork(t *testing.T) {
	chatSearchTestHome(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	report := SearchLocalChats(ctx, []ChatSearchBinding{NewChatSearchBinding(&Instance{ID: "cancel", Tool: "omp"})}, "needle")
	if !report.Partial || len(report.Matches) != 0 || time.Since(start) > time.Second {
		t.Fatalf("canceled search was not prompt and partial: %+v", report)
	}
}

func TestChatSearchGeminiRequiresFullIdentityAndStreamsMessages(t *testing.T) {
	home := chatSearchTestHome(t)
	inst := &Instance{ID: "gemini", Tool: "gemini", ProjectPath: "/same/project", GeminiSessionID: chatSearchTestCodexID}
	dir := filepath.Join(home, ".gemini", "tmp", HashProjectPath(inst.ProjectPath), "chats")
	path := filepath.Join(dir, "session-2026-10-19T09-00-22222222.json")
	chatSearchTestFile(t, path, `{"messages":[{"type":"user","content":"`+strings.Repeat("x", 2*1024*1024)+`"},{"type":"gemini","content":"saved gemini needle"}],"sessionId":"`+inst.GeminiSessionID+`"}`)
	binding := NewChatSearchBinding(inst)
	report := SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "gemini needle")
	chatSearchTestWantMatch(t, report, inst.ID, "saved gemini needle")
	if !report.Partial {
		t.Fatal("oversized JSON messages must be skipped, not allocated or silently ignored")
	}
	chatSearchTestFile(t, path, `{"sessionId":"22222222-9999-4999-8999-999999999999","messages":[{"type":"gemini","content":"foreign gemini needle"}]}`)
	report = SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "gemini needle")
	if len(report.Matches) != 0 || len(report.Unavailable) != 1 {
		t.Fatalf("8-character filename prefix is not conversation identity: %+v", report)
	}
}

// SQLite fixtures below use the persisted schemas, not CLI/export mocks.
func chatSearchTestDB(t *testing.T, path, schema string) *sql.DB {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestChatSearchOpenCodeSQLiteRestrictsSessionAndTextParts(t *testing.T) {
	home := chatSearchTestHome(t)
	db := chatSearchTestDB(t, filepath.Join(home, ".local", "share", "opencode", "opencode.db"), `
		CREATE TABLE session (id TEXT PRIMARY KEY, directory TEXT);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT, time_created INTEGER, data TEXT);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT, session_id TEXT, data TEXT);
		INSERT INTO session VALUES ('ses_bound', '/same/project'), ('ses_foreign', '/same/project');
		INSERT INTO message VALUES ('msg_bound', 'ses_bound', 1, '{"role":"assistant"}'), ('msg_foreign', 'ses_foreign', 2, '{"role":"user"}');
		INSERT INTO part VALUES
			('prt_1', 'msg_bound', 'ses_bound', '{"type":"reasoning","text":"reasoning secret needle"}'),
			('prt_2', 'msg_bound', 'ses_bound', '{"type":"tool","state":{"output":"tool secret needle"}}'),
			('prt_3', 'msg_foreign', 'ses_foreign', '{"type":"text","text":"foreign secret needle"}'),
			('prt_4', 'msg_bound', 'ses_bound', '{"type":"text","text":"saved opencode answer"}');
	`)
	oversized, err := json.Marshal(map[string]string{"type": "text", "text": strings.Repeat("x", 2*1024*1024)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO part VALUES ('prt_0', 'msg_bound', 'ses_bound', ?)`, string(oversized)); err != nil {
		t.Fatal(err)
	}
	inst := &Instance{ID: "opencode-row", Tool: "opencode", OpenCodeSessionID: "ses_bound", ProjectPath: "/same/project"}
	binding := NewChatSearchBinding(inst)
	report := SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "opencode answer")
	chatSearchTestWantMatch(t, report, inst.ID, "saved opencode answer")
	report = SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "secret needle")
	if len(report.Matches) != 0 || len(report.Unavailable) != 0 {
		t.Fatalf("foreign session, reasoning, or tool data leaked: %+v", report)
	}
	if !report.Partial {
		t.Fatal("oversized database records must be bounded and reported")
	}
	inst.OpenCodeSessionID = "ses_missing"
	report = SearchLocalChats(context.Background(), []ChatSearchBinding{NewChatSearchBinding(inst)}, "needle")
	if len(report.Matches) != 0 || len(report.Unavailable) != 1 {
		t.Fatalf("missing bound database session must be unavailable: %+v", report)
	}
}

func TestChatSearchPiRequiresUnambiguousInstanceOwnedSession(t *testing.T) {
	home := chatSearchTestHome(t)
	inst := &Instance{ID: "pi-row", Tool: "pi"}
	dir := filepath.Join(home, ".pi", "agent-deck", inst.ID)
	chatSearchTestFile(t, filepath.Join(dir, "saved.jsonl"), "{\"type\":\"session\",\"version\":3,\"id\":\"pi-bound\",\"cwd\":\"/same/project\"}\n"+chatSearchTestMessage(t, "user", "saved pi question"))
	binding := NewChatSearchBinding(inst)
	chatSearchTestWantMatch(t, SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "pi question"), inst.ID, "saved pi question")
	chatSearchTestFile(t, filepath.Join(dir, "another.jsonl"), "{\"type\":\"session\",\"id\":\"pi-another\"}\n"+chatSearchTestMessage(t, "assistant", "saved pi question in another conversation"))
	report := SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "pi question")
	if len(report.Matches) != 0 || len(report.Unavailable) != 1 {
		t.Fatalf("Pi cannot guess the active conversation by mtime: %+v", report)
	}
}

func TestChatSearchKiroCLIStreamsAssistantPiecesWithExactHeader(t *testing.T) {
	home := chatSearchTestHome(t)
	t.Setenv("KIRO_HOME", filepath.Join(home, ".kiro"))
	inst := &Instance{ID: "kiro-row", Tool: "kiro", KiroSessionID: chatSearchTestClaudeID}
	dir := filepath.Join(home, ".kiro", "sessions", "cli")
	header := filepath.Join(dir, inst.KiroSessionID+".json")
	chatSearchTestFile(t, header, `{"session_id":"`+inst.KiroSessionID+`","cwd":"/same/project","session_state":{}}`)
	chatSearchTestFile(t, filepath.Join(dir, inst.KiroSessionID+".jsonl"),
		`{"kind":"Prompt","data":{"content":[{"kind":"text","data":"saved Kiro question"}]}}`+"\n"+
			`{"kind":"AssistantMessage","data":{"message_id":"reply-1","content":[{"kind":"text","data":"violet light"}]}}`+"\n"+
			`{"kind":"AssistantMessage","data":{"message_id":"reply-1","content":[{"kind":"text","data":"house answer"},{"kind":"toolUse","data":{"name":"shell","input":"private tool needle"}}]}}`+"\n"+
			`{"kind":"ToolResults","data":{"content":[{"kind":"text","data":"private tool needle"}]}}`+"\n")
	binding := NewChatSearchBinding(inst)
	chatSearchTestWantMatch(t, SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "violet lighthouse"), inst.ID, "violet lighthouse answer")
	report := SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "private tool")
	if len(report.Matches) != 0 || len(report.Unavailable) != 0 {
		t.Fatalf("Kiro tool parts are not conversation text: %+v", report)
	}
	chatSearchTestFile(t, header, `{"session_id":"`+chatSearchTestForeignID+`","cwd":"/same/project","session_state":{}}`)
	report = SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "violet")
	if len(report.Matches) != 0 || len(report.Unavailable) != 1 {
		t.Fatalf("Kiro filename must be corroborated by the saved header: %+v", report)
	}
}

func TestChatSearchCopilotUsesSavedContentNotInjectedWrapper(t *testing.T) {
	home := chatSearchTestHome(t)
	t.Setenv("COPILOT_CONFIG_DIR", filepath.Join(home, "copilot-account"))
	inst := &Instance{ID: "copilot-row", Tool: "copilot", CopilotSessionID: chatSearchTestClaudeID}
	path := filepath.Join(home, "copilot-account", "session-state", inst.CopilotSessionID, "events.jsonl")
	chatSearchTestFile(t, path,
		`{"type":"session.start","data":{"sessionId":"`+inst.CopilotSessionID+`","context":{"cwd":"/same/project"}}}`+"\n"+
			`{"type":"user.message","data":{"content":"saved user question","transformedContent":"private injected needle"}}`+"\n"+
			`{"type":"assistant.message","data":{"messageId":"reply-1","content":"saved copilot answer","toolRequests":[{"arguments":"private tool needle"}]}}`+"\n")
	binding := NewChatSearchBinding(inst)
	chatSearchTestWantMatch(t, SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "copilot answer"), inst.ID, "saved copilot answer")
	report := SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "private")
	if len(report.Matches) != 0 || len(report.Unavailable) != 0 {
		t.Fatalf("Copilot transformed context/tool arguments leaked: %+v", report)
	}
	chatSearchTestFile(t, path, `{"type":"session.start","data":{"sessionId":"`+chatSearchTestForeignID+`"}}`+"\n"+`{"type":"assistant.message","data":{"content":"foreign copilot answer"}}`+"\n")
	report = SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "copilot answer")
	if len(report.Matches) != 0 || len(report.Unavailable) != 1 {
		t.Fatalf("Copilot session.start identity must match the binding: %+v", report)
	}
}

func TestChatSearchUnavailableFilesDoNotHideOtherRows(t *testing.T) {
	for _, state := range []string{"missing", "directory", "unknown-format", "empty"} {
		t.Run(state, func(t *testing.T) {
			home := chatSearchTestHome(t)
			bad := &Instance{ID: "bad-" + state, Tool: "claude", ProjectPath: "/same/project", ClaudeSessionID: chatSearchTestClaudeID}
			good := &Instance{ID: "good", Tool: "claude", ProjectPath: "/same/project", ClaudeSessionID: chatSearchTestForeignID}
			dir := filepath.Join(home, ".claude", "projects", "-same-project")
			path := filepath.Join(dir, bad.ClaudeSessionID+".jsonl")
			switch state {
			case "directory":
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "unknown-format":
				chatSearchTestFile(t, path, "{\"not_a_saved_chat\":\"private needle\"}\n")
			case "empty":
				chatSearchTestFile(t, path, "")
			}
			chatSearchTestFile(t, filepath.Join(dir, good.ClaudeSessionID+".jsonl"), `{"type":"assistant","message":{"role":"assistant","content":"good needle answer"}}`+"\n")
			report := SearchLocalChats(context.Background(), []ChatSearchBinding{NewChatSearchBinding(bad), NewChatSearchBinding(good)}, "needle")
			chatSearchTestWantMatch(t, report, good.ID, "good needle answer")
			if len(report.Unavailable) != 1 || report.Unavailable[0].InstanceID != bad.ID || strings.Contains(report.Unavailable[0].Reason, home) || strings.Contains(report.Unavailable[0].Reason, "private") {
				t.Fatalf("per-binding failure must remain isolated and non-sensitive: %+v", report)
			}
		})
	}
}

func TestChatSearchSnapshotCopiesMutableOptions(t *testing.T) {
	home := chatSearchTestHome(t)
	inst := &Instance{ID: "options-row", Tool: "omp"}
	if err := inst.SetOmpOptions(&OmpOptions{NoSession: true}); err != nil {
		t.Fatal(err)
	}
	binding := NewChatSearchBinding(inst)
	inst.mu.Lock()
	// Corrupt the original backing array: an aliased snapshot would fall back
	// to default options and incorrectly search a formerly retained transcript.
	for idx := range inst.ToolOptionsJSON {
		inst.ToolOptionsJSON[idx] = ' '
	}
	inst.mu.Unlock()
	dir := filepath.Join(home, ".omp", "agent-deck", inst.ID)
	path := filepath.Join(dir, "retained.jsonl")
	chatSearchTestFile(t, path, "{\"type\":\"session\",\"id\":\"retained\"}\n"+chatSearchTestMessage(t, "assistant", "retained private needle"))
	writeOmpValidationBinding(t, dir, path, "retained", "saved", "legacy")
	report := SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "needle")
	if len(report.Matches) != 0 || len(report.Unavailable) != 1 {
		t.Fatalf("snapshot options must remain immutable: %+v", report)
	}
}

type chatSearchCancelReader struct {
	io.Reader
	cancel context.CancelFunc
}

func (r chatSearchCancelReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.cancel()
	return n, err
}

func TestChatSearchCancellationInterruptsOversizedRecordDrain(t *testing.T) {
	home := chatSearchTestHome(t)
	path := filepath.Join(home, "oversized.jsonl")
	chatSearchTestFile(t, path, strings.Repeat("x", 2*1024*1024)+"\n")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := chatSearchReader(ctx, chatSearchCancelReader{Reader: file, cancel: cancel})
	_, _, err = chatSearchReadLine(reader, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("draining an oversized line must observe cancellation: %v", err)
	}
}

func TestChatSearchReportsUnsupportedSavedStores(t *testing.T) {
	home := chatSearchTestHome(t)
	t.Setenv("KIRO_HOME", filepath.Join(home, ".kiro"))
	instances := []*Instance{
		{ID: "kiro-headless", Tool: "kiro", KiroSessionID: chatSearchTestClaudeID},
		{ID: "opencode-legacy", Tool: "opencode", OpenCodeSessionID: "ses_legacy"},
	}
	chatSearchTestFile(t, filepath.Join(home, ".local", "share", "opencode", "storage", "message", "ses_legacy", "msg_legacy.json"), `{"role":"user","content":"legacy private needle"}`)
	bindings := []ChatSearchBinding{NewChatSearchBinding(instances[0]), NewChatSearchBinding(instances[1])}
	report := SearchLocalChats(context.Background(), bindings, "needle")
	if len(report.Matches) != 0 || len(report.Unavailable) != 2 {
		t.Fatalf("unsupported saved stores must be explicit, never silent misses: %+v", report)
	}
	for _, unavailable := range report.Unavailable {
		if !strings.Contains(unavailable.Reason, "unsupported") {
			t.Fatalf("status must explain the saved-store limitation: %+v", unavailable)
		}
	}
}

func TestChatSearchKiroDoesNotJoinAcrossSkippedRecords(t *testing.T) {
	home := chatSearchTestHome(t)
	t.Setenv("KIRO_HOME", filepath.Join(home, ".kiro"))
	inst := &Instance{ID: "kiro-skip", Tool: "kiro", KiroSessionID: chatSearchTestClaudeID}
	dir := filepath.Join(home, ".kiro", "sessions", "cli")
	chatSearchTestFile(t, filepath.Join(dir, inst.KiroSessionID+".json"), `{"session_id":"`+inst.KiroSessionID+`"}`)
	chatSearchTestFile(t, filepath.Join(dir, inst.KiroSessionID+".jsonl"),
		`{"kind":"AssistantMessage","data":{"message_id":"reply-1","content":[{"kind":"text","data":"violet "}]}}`+"\n"+
			`{"kind":"AssistantMessage","data":{"message_id":"reply-1","content":[{"kind":"text","data":"`+strings.Repeat("x", 2*1024*1024)+`"}]}}`+"\n"+
			`{"kind":"AssistantMessage","data":{"message_id":"reply-1","content":[{"kind":"text","data":"lighthouse"}]}}`+"\n"+
			`{"kind":"AssistantMessage","data":{"message_id":"reply-2","content":[{"kind":"text","data":"later saved answer"}]}}`+"\n")
	binding := NewChatSearchBinding(inst)
	report := SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "violet lighthouse")
	if len(report.Matches) != 0 || !report.Partial {
		t.Fatalf("skipped assistant pieces must not manufacture adjacent text: %+v", report)
	}
	chatSearchTestWantMatch(t, SearchLocalChats(context.Background(), []ChatSearchBinding{binding}, "later saved"), inst.ID, "later saved answer")
}

func TestChatSearchPiRejectsUnresolvedConversationLocation(t *testing.T) {
	for _, state := range []string{"root-symlink", "depth-limit"} {
		t.Run(state, func(t *testing.T) {
			home := chatSearchTestHome(t)
			inst := &Instance{ID: "pi-location", Tool: "pi"}
			root := filepath.Join(home, ".pi", "agent-deck", inst.ID)
			body := "{\"type\":\"session\",\"id\":\"pi-location-root\"}\n" + chatSearchTestMessage(t, "assistant", "private location needle")
			if state == "root-symlink" {
				foreign := filepath.Join(home, "foreign-conversation")
				chatSearchTestFile(t, filepath.Join(foreign, "root.jsonl"), body)
				if err := os.MkdirAll(filepath.Dir(root), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(foreign, root); err != nil {
					t.Fatal(err)
				}
			} else {
				chatSearchTestFile(t, filepath.Join(root, "root.jsonl"), body)
				deep := root
				for range 33 {
					deep = filepath.Join(deep, "nested")
				}
				chatSearchTestFile(t, filepath.Join(deep, "other.jsonl"), body)
			}
			report := SearchLocalChats(context.Background(), []ChatSearchBinding{NewChatSearchBinding(inst)}, "needle")
			if len(report.Matches) != 0 || len(report.Unavailable) != 1 || report.Partial != (state == "depth-limit") {
				t.Fatalf("an unresolved Pi location must not authorize a conversation: %+v", report)
			}
		})
	}
}

func TestChatSearchOMPRequiresCurrentPositiveIdentityStatus(t *testing.T) {
	for _, state := range []string{
		"missing", "negative-empty", "negative-retained", "wrong-instance",
		"wrong-generation", "wrong-session", "wrong-file", "malformed",
		"oversized", "positive", "title-only-warning", "legacy-binding",
	} {
		t.Run(state, func(t *testing.T) {
			home := chatSearchTestHome(t)
			inst := &Instance{ID: "omp-status-row", Tool: "omp"}
			dir := filepath.Join(home, ".omp", "agent-deck", inst.ID)
			path := filepath.Join(dir, "retained.jsonl")
			chatSearchTestFile(t, path, "{\"type\":\"session\",\"id\":\"retained-root\"}\n"+chatSearchTestMessage(t, "assistant", "retained private needle"))
			generation, bindingName := "launch-1", ompActiveBindingName+".launch-1"
			if state == "legacy-binding" {
				generation, bindingName = "legacy", ompActiveBindingName
			} else {
				chatSearchTestFile(t, filepath.Join(dir, ".agent-deck-launch-generation"), generation+"\n")
			}
			writeOmpValidationBindingAt(t, dir, bindingName, path, "retained-root", "saved", generation)
			status := map[string]any{
				"instance_id": inst.ID, "launch_id": generation, "identity_ready": true,
				"session_id": "retained-root", "session_file": path, "error": "",
			}
			switch state {
			case "negative-empty":
				status["identity_ready"], status["session_id"], status["session_file"] = false, "", ""
				status["error"] = "identity tracking failed: private detail"
			case "negative-retained":
				status["identity_ready"] = false
				status["error"] = "identity tracking failed: private detail"
			case "wrong-instance":
				status["instance_id"] = "another-row"
			case "wrong-generation":
				status["launch_id"] = "another-launch"
			case "wrong-session":
				status["session_id"] = "another-conversation"
			case "wrong-file":
				status["session_file"] = filepath.Join(dir, "another.jsonl")
			case "title-only-warning":
				status["error"] = "title sync failed but identity remains valid"
			case "oversized":
				status["error"] = strings.Repeat("x", ompBindingLimit)
			}
			if state != "missing" && state != "legacy-binding" {
				body, err := json.Marshal(status)
				if err != nil {
					t.Fatal(err)
				}
				if state == "malformed" {
					body = []byte(`{"identity_ready":`)
				}
				chatSearchTestFile(t, filepath.Join(dir, ".agent-deck-omp-status."+generation+".json"), string(body))
			}
			report := SearchLocalChats(context.Background(), []ChatSearchBinding{NewChatSearchBinding(inst)}, "needle")
			if state == "positive" || state == "title-only-warning" || state == "legacy-binding" {
				chatSearchTestWantMatch(t, report, inst.ID, "retained private needle")
				if len(report.Unavailable) != 0 || report.Partial {
					t.Fatalf("valid acknowledged identity must remain searchable: %+v", report)
				}
			} else if len(report.Matches) != 0 || len(report.Unavailable) != 1 || strings.Contains(report.Unavailable[0].Reason, "private detail") {
				t.Fatalf("unacknowledged retained root must not be searched: %+v", report)
			}
		})
	}
}
