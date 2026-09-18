package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/asheshgoplani/agent-deck/internal/fleet"
)

// recoveryPromptMaxVisible caps how many candidate rows are rendered at once.
// A fleet-wide death (the scenario this dialog exists for) can down dozens of
// sessions; scrolling keeps the dialog on screen instead of overflowing the
// terminal. `agent-deck fleet recover` remains the tool for large sweeps.
const recoveryPromptMaxVisible = 12

// RecoveryPromptDialog proposes restarting sessions the registry still
// believes are alive (status running/waiting/starting/error) but whose tmux
// session is gone — the signature an ungraceful shutdown (power loss, OOM, a
// killed tmux server) leaves behind. Shown at most once per launch, right
// after the first session load, when fleet.Detector finds a candidate. See
// internal/fleet for the detection/recovery mechanics this dialog wraps.
type RecoveryPromptDialog struct {
	visible       bool
	width, height int

	// assessment is the full scan result. Only assessment.Candidates is
	// walked for display/selection; other fields (MassDeath, Total, ...) ride
	// along so SelectedAssessment can hand the Recoverer a faithful subset.
	assessment fleet.Assessment
	checked    []bool // parallel to assessment.Candidates; true = restore it
	cursor     int
}

// NewRecoveryPromptDialog creates a new (hidden) recovery prompt dialog.
func NewRecoveryPromptDialog() *RecoveryPromptDialog {
	return &RecoveryPromptDialog{}
}

// Show opens the dialog over the given assessment. Every candidate starts
// checked: the common case is "yes, bring all of them back".
func (d *RecoveryPromptDialog) Show(as fleet.Assessment) {
	d.visible = true
	d.assessment = as
	d.checked = make([]bool, len(as.Candidates))
	for i := range d.checked {
		d.checked[i] = true
	}
	d.cursor = 0
}

// Hide closes the dialog and releases its candidate list.
func (d *RecoveryPromptDialog) Hide() {
	d.visible = false
	d.assessment = fleet.Assessment{}
	d.checked = nil
	d.cursor = 0
}

// IsVisible reports whether the dialog is currently shown.
func (d *RecoveryPromptDialog) IsVisible() bool {
	return d.visible
}

// SetSize updates the dialog dimensions for centering.
func (d *RecoveryPromptDialog) SetSize(w, h int) {
	d.width = w
	d.height = h
}

// SelectedAssessment returns a copy of the shown assessment whose Candidates
// (and Down) are narrowed to the checked rows, in original order. Other
// fields (Total, Alive, Skipped, MassDeath, Probes) are preserved so a
// Recoverer sees the real fleet shape even when the operator restores only a
// subset.
func (d *RecoveryPromptDialog) SelectedAssessment() fleet.Assessment {
	out := d.assessment
	out.Candidates = nil
	for i, c := range d.assessment.Candidates {
		if i < len(d.checked) && d.checked[i] {
			out.Candidates = append(out.Candidates, c)
		}
	}
	out.Down = len(out.Candidates)
	return out
}

// Update handles key events for the dialog. Enter/Esc are intentionally left
// for the caller (Home.handleRecoveryPromptKey) to act on: this method only
// owns navigation and selection state.
func (d *RecoveryPromptDialog) Update(msg tea.KeyMsg) (*RecoveryPromptDialog, tea.Cmd) {
	if !d.visible {
		return d, nil
	}

	n := len(d.assessment.Candidates)
	switch msg.String() {
	case "j", "down":
		if n > 0 {
			d.cursor = (d.cursor + 1) % n
		}
	case "k", "up":
		if n > 0 {
			d.cursor = (d.cursor - 1 + n) % n
		}
	case " ", "space":
		if d.cursor >= 0 && d.cursor < len(d.checked) {
			d.checked[d.cursor] = !d.checked[d.cursor]
		}
	case "a":
		for i := range d.checked {
			d.checked[i] = true
		}
	case "n":
		for i := range d.checked {
			d.checked[i] = false
		}
	case "esc":
		d.Hide()
	}

	return d, nil
}

// View renders the recovery prompt dialog.
func (d *RecoveryPromptDialog) View() string {
	if !d.visible {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorYellow)
	noteStyle := lipgloss.NewStyle().Foreground(ColorTextDim)
	dimStyle := lipgloss.NewStyle().Foreground(ColorComment)
	footerStyle := lipgloss.NewStyle().Foreground(ColorComment).Italic(true)

	candidates := d.assessment.Candidates
	var lines []string

	sessionWord := "sessions look"
	if len(candidates) == 1 {
		sessionWord = "session looks"
	}
	lines = append(lines, titleStyle.Render(fmt.Sprintf("⚠ %d %s crashed", len(candidates), sessionWord)))
	if d.assessment.MassDeath {
		lines = append(lines, noteStyle.Render("This looks like a full crash (tmux server killed, or the host rebooted),"))
		lines = append(lines, noteStyle.Render("not one bad session."))
	} else {
		lines = append(lines, noteStyle.Render("Their tmux session is gone, but agent-deck still expected them running."))
	}
	lines = append(lines, "")

	start := 0
	if d.cursor >= recoveryPromptMaxVisible {
		start = d.cursor - recoveryPromptMaxVisible + 1
	}
	end := start + recoveryPromptMaxVisible
	if end > len(candidates) {
		end = len(candidates)
	}

	for i := start; i < end; i++ {
		c := candidates[i]
		checked := i < len(d.checked) && d.checked[i]
		label := fmt.Sprintf("%s (was %s)", c.Title(), c.Status)
		lines = append(lines, renderCheckboxLine(label, checked, i == d.cursor))
	}
	if len(candidates) > recoveryPromptMaxVisible {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  (showing %d-%d of %d — ↑↓ to scroll)", start+1, end, len(candidates))))
	}

	lines = append(lines, "")
	lines = append(lines, footerStyle.Render("↑↓ move · space toggle · a all · n none · enter restore checked · esc not now"))

	content := strings.Join(lines, "\n")
	dialogWidth := fitDialogWidth(64, 40, d.width)
	box := DialogBoxStyle.Width(dialogWidth).Render(content)
	return centerInScreen(box, d.width, d.height)
}
