package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

func TestSavedSessionImportDialogsDefaultToStartAfterImport(t *testing.T) {
	t.Run("codex", func(t *testing.T) {
		d := NewCodexImportDialog()
		d.Show([]session.CodexIndexEntry{{ID: "44444444-4444-4444-4444-444444444444", ThreadName: "alpha"}})
		if !d.StartAfterImport() {
			t.Fatal("Codex import should default to start after import")
		}
		if rendered := d.View(); !containsAll(rendered, "[x]", "Start after import") {
			t.Fatalf("Codex import dialog missing checked start toggle:\n%s", rendered)
		}
		d.Update(tea.KeyMsg{Type: tea.KeySpace})
		if d.StartAfterImport() {
			t.Fatal("space should disable Codex start after import")
		}
	})

	t.Run("claude", func(t *testing.T) {
		d := NewClaudeImportDialog()
		d.Show([]session.ClaudeImportCandidate{{SessionID: "44444444-4444-4444-4444-444444444444", Name: "alpha"}})
		if !d.StartAfterImport() {
			t.Fatal("Claude import should default to start after import")
		}
		if rendered := d.View(); !containsAll(rendered, "[x]", "Start after import") {
			t.Fatalf("Claude import dialog missing checked start toggle:\n%s", rendered)
		}
		d.Update(tea.KeyMsg{Type: tea.KeySpace})
		if d.StartAfterImport() {
			t.Fatal("space should disable Claude start after import")
		}
	})

	t.Run("opencode", func(t *testing.T) {
		d := NewOpenCodeImportDialog()
		d.Show([]session.OpenCodeImportEntry{{ID: "ses_alpha123", Title: "alpha"}})
		if !d.StartAfterImport() {
			t.Fatal("OpenCode import should default to start after import")
		}
		if rendered := d.View(); !containsAll(rendered, "[x]", "Start after import") {
			t.Fatalf("OpenCode import dialog missing checked start toggle:\n%s", rendered)
		}
		d.Update(tea.KeyMsg{Type: tea.KeySpace})
		if d.StartAfterImport() {
			t.Fatal("space should disable OpenCode start after import")
		}
	})

	t.Run("kiro", func(t *testing.T) {
		d := NewKiroImportDialog()
		d.Show([]session.KiroSavedSession{{ID: "44444444-4444-4444-4444-444444444444", Title: "alpha"}})
		if !d.StartAfterImport() {
			t.Fatal("Kiro import should default to start after import")
		}
		if rendered := d.View(); !containsAll(rendered, "[x]", "Start after import") {
			t.Fatalf("Kiro import dialog missing checked start toggle:\n%s", rendered)
		}
		d.Update(tea.KeyMsg{Type: tea.KeySpace})
		if d.StartAfterImport() {
			t.Fatal("space should disable Kiro start after import")
		}
	})
}

func TestCodexImportConfirmsSelectedConversationBeforeStartAndSave(t *testing.T) {
	home := setXDGTestHome(t)
	codexHome := filepath.Join(home, ".codex")
	t.Setenv("CODEX_HOME", codexHome)
	const id = "019f2222-2222-4222-8222-666666666666"
	projectPath := t.TempDir()
	writeCodexRolloutForHomeImportBody(t, codexHome, id,
		`{"type":"session_meta","payload":{"id":"`+id+`","cwd":"`+projectPath+`","source":"cli"}}`+"\n")
	h := NewHomeWithProfile("codex_import_tui_start")

	started := false
	oldStart := startImportedSession
	startImportedSession = func(inst *session.Instance) error {
		started = true
		if inst.CodexSessionID != id || !inst.CodexBindingConfirmed {
			t.Fatalf("imported conversation was not confirmed before start: id=%q confirmed=%v", inst.CodexSessionID, inst.CodexBindingConfirmed)
		}
		persisted, _, err := h.storage.LoadWithGroups()
		if err != nil {
			t.Fatalf("load before start: %v", err)
		}
		if len(persisted) != 0 {
			t.Fatalf("TUI import was saved before start: %d instances", len(persisted))
		}
		inst.Status = session.StatusRunning
		return nil
	}
	t.Cleanup(func() { startImportedSession = oldStart })

	message := h.createSessionFromCodexImport(session.CodexIndexEntry{
		ID: id, ThreadName: "started Codex import", Path: projectPath, UpdatedAt: time.Now(),
	}, true)().(sessionCreatedMsg)
	if message.err != nil || message.instance == nil {
		t.Fatalf("createSessionFromCodexImport failed: %+v", message)
	}
	if !started {
		t.Fatal("startImportedSession was not called")
	}
	if message.instance.Status != session.StatusRunning {
		t.Fatalf("imported status = %q, want running", message.instance.Status)
	}
	model, _ := h.updateInner(message)
	h = model.(*Home)
	persisted, _, err := h.storage.LoadWithGroups()
	if err != nil {
		t.Fatalf("load after import: %v", err)
	}
	if len(persisted) != 1 || persisted[0].CodexSessionID != id || !persisted[0].CodexBindingConfirmed {
		t.Fatalf("started TUI import did not round-trip its confirmed selection: %#v", persisted)
	}
}

func TestClaudeImportStartsImportedSessionWhenEnabled(t *testing.T) {
	h := newHubProjectionHome(t, nil)
	started := false
	oldStart := startImportedSession
	startImportedSession = func(inst *session.Instance) error {
		started = true
		inst.Status = session.StatusRunning
		return nil
	}
	t.Cleanup(func() { startImportedSession = oldStart })

	cmd := h.createSessionFromClaudeImport(session.ClaudeImportCandidate{
		SessionID: "44444444-4444-4444-4444-444444444444",
		Name:      "Imported Claude",
		CWD:       t.TempDir(),
		UpdatedAt: time.Now(),
	}, true)
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("createSessionFromClaudeImport returned error: %v", msg.err)
	}
	if !started {
		t.Fatal("startImportedSession was not called")
	}
	if msg.instance == nil || msg.instance.Status != session.StatusRunning {
		t.Fatalf("imported instance = %#v, want running", msg.instance)
	}
}

func TestClaudeImportCanRemainStoppedWhenStartDisabled(t *testing.T) {
	h := newHubProjectionHome(t, nil)
	started := false
	oldStart := startImportedSession
	startImportedSession = func(inst *session.Instance) error {
		started = true
		return nil
	}
	t.Cleanup(func() { startImportedSession = oldStart })

	cmd := h.createSessionFromClaudeImport(session.ClaudeImportCandidate{
		SessionID: "44444444-4444-4444-4444-444444444444",
		Name:      "Imported Claude",
		CWD:       t.TempDir(),
		UpdatedAt: time.Now(),
	}, false)
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("createSessionFromClaudeImport returned error: %v", msg.err)
	}
	if started {
		t.Fatal("startImportedSession should not be called when start is disabled")
	}
	if msg.instance == nil || msg.instance.Status != session.StatusStopped {
		t.Fatalf("imported instance = %#v, want stopped", msg.instance)
	}
}

func containsAll(s string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(s, needle) {
			return false
		}
	}
	return true
}
