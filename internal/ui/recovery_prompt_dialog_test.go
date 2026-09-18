package ui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/asheshgoplani/agent-deck/internal/fleet"
	"github.com/asheshgoplani/agent-deck/internal/session"
)

// key builds a rune KeyMsg for dialog navigation/selection keys.
func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func makeTestCandidates() fleet.Assessment {
	insts := []*session.Instance{
		{ID: "id-1", Title: "frontend-agent", Tool: "claude", Status: session.StatusRunning},
		{ID: "id-2", Title: "backend-agent", Tool: "codex", Status: session.StatusWaiting},
		{ID: "id-3", Title: "data-pipeline", Tool: "gemini", Status: session.StatusError},
	}
	as := fleet.Assessment{Total: 3, Down: 3, MassDeath: true}
	for _, inst := range insts {
		as.Candidates = append(as.Candidates, fleet.Candidate{Instance: inst, Health: fleet.HealthDown, Status: string(inst.Status)})
	}
	return as
}

func TestNewRecoveryPromptDialog_Hidden(t *testing.T) {
	d := NewRecoveryPromptDialog()
	if d.IsVisible() {
		t.Error("new dialog should not be visible")
	}
	if d.View() != "" {
		t.Error("hidden dialog should render empty")
	}
}

func TestShow_AllCheckedByDefault(t *testing.T) {
	d := NewRecoveryPromptDialog()
	d.Show(makeTestCandidates())
	defer d.Hide()

	if !d.IsVisible() {
		t.Error("dialog should be visible after Show")
	}
	got := d.SelectedAssessment()
	if got.Down != 3 || len(got.Candidates) != 3 {
		t.Errorf("all candidates should be checked by default, got down=%d candidates=%d", got.Down, len(got.Candidates))
	}
	if got.Candidates[0].ID() != "id-1" {
		t.Errorf("candidate order should be preserved, first id=%q", got.Candidates[0].ID())
	}
	// Non-candidate fields ride along so the Recoverer sees the real shape.
	if !got.MassDeath || got.Total != 3 {
		t.Errorf("assessment fields should be preserved, got massDeath=%v total=%d", got.MassDeath, got.Total)
	}
}

func TestSpace_TogglesOnlyCursorRow(t *testing.T) {
	d := NewRecoveryPromptDialog()
	d.Show(makeTestCandidates())
	defer d.Hide()

	d.Update(tea.KeyMsg{Type: tea.KeySpace})
	got := d.SelectedAssessment()
	if got.Down != 2 || len(got.Candidates) != 2 {
		t.Fatalf("expected 2 checked after toggling first, got down=%d", got.Down)
	}
	for _, c := range got.Candidates {
		if c.ID() == "id-1" {
			t.Error("toggled-off candidate must be excluded from selection")
		}
	}

	// Toggle it back on.
	d.Update(tea.KeyMsg{Type: tea.KeySpace})
	if got := d.SelectedAssessment(); got.Down != 3 {
		t.Errorf("expected 3 checked after re-toggle, got %d", got.Down)
	}
}

func TestSelectNone_SelectAll(t *testing.T) {
	d := NewRecoveryPromptDialog()
	d.Show(makeTestCandidates())

	d.Update(key("n"))
	if got := d.SelectedAssessment(); got.Down != 0 {
		t.Errorf("'n' should uncheck everything, got down=%d", got.Down)
	}

	d.Update(key("a"))
	if got := d.SelectedAssessment(); got.Down != 3 {
		t.Errorf("'a' should check everything, got down=%d", got.Down)
	}
}

func TestNavigation_Wraps(t *testing.T) {
	d := NewRecoveryPromptDialog()
	d.Show(makeTestCandidates())
	defer d.Hide()

	// A single up from the top wraps to the last row; toggling there must
	// flip only the last candidate (id-3).
	d.Update(key("up"))
	d.Update(tea.KeyMsg{Type: tea.KeySpace})
	got := d.SelectedAssessment()
	if got.Down != 2 {
		t.Fatalf("expected 2 checked after toggling wrapped cursor, got %d", got.Down)
	}
	for _, c := range got.Candidates {
		if c.ID() == "id-3" {
			t.Error("wrong row toggled after wrap: id-3 should be off")
		}
	}

	// From the bottom, a single down wraps back to the first row; toggling
	// there must re-check id-1 (cursor is on it) — flip it off instead.
	d.Update(key("down"))
	d.Update(tea.KeyMsg{Type: tea.KeySpace})
	got = d.SelectedAssessment()
	if got.Down != 1 {
		t.Fatalf("expected 1 checked after toggling row 0, got %d", got.Down)
	}
	for _, c := range got.Candidates {
		if c.ID() == "id-1" {
			t.Error("wrong row toggled after wrap: id-1 should be off")
		}
	}
}

func TestEsc_HidesAndResets(t *testing.T) {
	d := NewRecoveryPromptDialog()
	d.Show(makeTestCandidates())

	d.Update(key("esc"))
	if d.IsVisible() {
		t.Error("esc should hide the dialog")
	}
	if got := d.SelectedAssessment(); got.Down != 0 || len(got.Candidates) != 0 {
		t.Errorf("Hide should reset selection state, got down=%d candidates=%d", got.Down, len(got.Candidates))
	}
}

func TestUpdate_IgnoresWhenHidden(t *testing.T) {
	d := NewRecoveryPromptDialog()
	d.Update(key("j"))
	d.Update(tea.KeyMsg{Type: tea.KeySpace})
	if d.IsVisible() {
		t.Error("hidden dialog must stay hidden")
	}
}

func TestView_ShowsCountTitleAndCandidates(t *testing.T) {
	d := NewRecoveryPromptDialog()
	d.SetSize(120, 40)
	d.Show(makeTestCandidates())
	defer d.Hide()

	view := d.View()
	for _, want := range []string{"3 sessions look crashed", "frontend-agent", "backend-agent", "data-pipeline", "enter restore"} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain %q", want)
		}
	}
}

func TestView_SingularTitle(t *testing.T) {
	d := NewRecoveryPromptDialog()
	d.SetSize(120, 40)
	as := fleet.Assessment{Total: 1, Down: 1}
	as.Candidates = append(as.Candidates, fleet.Candidate{
		Instance: &session.Instance{ID: "id-1", Title: "solo", Tool: "claude", Status: session.StatusRunning},
		Health:   fleet.HealthDown, Status: "running",
	})
	d.Show(as)
	defer d.Hide()

	if !strings.Contains(d.View(), "1 session looks crashed") {
		t.Error("singular count should read '1 session looks crashed'")
	}
}

func TestView_ScrollWindowForLargeFleets(t *testing.T) {
	d := NewRecoveryPromptDialog()
	d.SetSize(120, 40)
	as := fleet.Assessment{Total: 20, Down: 20}
	for i := range 20 {
		as.Candidates = append(as.Candidates, fleet.Candidate{
			Instance: &session.Instance{
				ID:     "id-" + strconv.Itoa(i),
				Title:  fmt.Sprintf("sess-%02d", i),
				Tool:   "claude",
				Status: session.StatusRunning,
			},
			Health: fleet.HealthDown, Status: "running",
		})
	}
	d.Show(as)
	defer d.Hide()

	// Bottom of the list must not be rendered initially: the dialog caps
	// rendered rows instead of overflowing the screen.
	if strings.Contains(d.View(), "sess-19") {
		t.Error("dialog should cap rendered rows instead of overflowing the screen")
	}
	// Cursor to the end; the window must follow.
	for range 19 {
		d.Update(key("j"))
	}
	if !strings.Contains(d.View(), "sess-19") {
		t.Error("cursor row must stay visible while scrolling")
	}
}
