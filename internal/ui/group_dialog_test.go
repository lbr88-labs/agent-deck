package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestGroupDialog_NameInput_AcceptsUnderscore verifies that typing '_' into the
// group name input reaches the textinput buffer (regression test for BUG-02).
func TestGroupDialog_NameInput_AcceptsUnderscore(t *testing.T) {
	g := NewGroupDialog()
	g.Show()

	underscoreKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'_'}}
	updated, _ := g.Update(underscoreKey)

	if updated.nameInput.Value() != "_" {
		t.Errorf("nameInput.Value() = %q after typing '_', want %q", updated.nameInput.Value(), "_")
	}
}
