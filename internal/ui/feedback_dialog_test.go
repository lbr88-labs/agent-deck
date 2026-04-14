package ui

import (
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/feedback"
	tea "github.com/charmbracelet/bubbletea"
)

// TestFeedbackDialog_InitiallyHidden verifies NewFeedbackDialog() returns a dialog that is not visible.
func TestFeedbackDialog_InitiallyHidden(t *testing.T) {
	d := NewFeedbackDialog()
	if d == nil {
		t.Fatal("NewFeedbackDialog() returned nil")
	}
	if d.IsVisible() {
		t.Error("expected dialog to be hidden initially, but IsVisible() returned true")
	}
}

// TestFeedbackDialog_ShowMakesVisible verifies Show() makes the dialog visible.
func TestFeedbackDialog_ShowMakesVisible(t *testing.T) {
	d := NewFeedbackDialog()
	st := &feedback.State{FeedbackEnabled: true, MaxShows: 3, ShownCount: 1}
	sender := feedback.NewSender()
	d.Show("1.5.1", st, sender)
	if !d.IsVisible() {
		t.Error("expected dialog to be visible after Show(), but IsVisible() returned false")
	}
}

// TestFeedbackDialog_RatingKey_AdvancesToComment verifies pressing '3' at stepRating transitions to stepComment.
func TestFeedbackDialog_RatingKey_AdvancesToComment(t *testing.T) {
	d := NewFeedbackDialog()
	st := &feedback.State{FeedbackEnabled: true, MaxShows: 3, ShownCount: 1}
	sender := feedback.NewSender()
	d.Show("1.5.1", st, sender)

	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if d.step != stepComment {
		t.Errorf("expected step to be stepComment (%d), got %d", stepComment, d.step)
	}
	if d.rating != 3 {
		t.Errorf("expected rating to be 3, got %d", d.rating)
	}
}

// TestFeedbackDialog_AllRatingKeys verifies keys '1'-'5' store the correct integer rating.
func TestFeedbackDialog_AllRatingKeys(t *testing.T) {
	cases := []struct {
		key    rune
		rating int
	}{
		{'1', 1},
		{'2', 2},
		{'3', 3},
		{'4', 4},
		{'5', 5},
	}
	for _, tc := range cases {
		d := NewFeedbackDialog()
		st := &feedback.State{FeedbackEnabled: true, MaxShows: 3, ShownCount: 1}
		sender := feedback.NewSender()
		d.Show("1.5.1", st, sender)

		d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
		if d.rating != tc.rating {
			t.Errorf("key '%c': expected rating %d, got %d", tc.key, tc.rating, d.rating)
		}
		if d.step != stepComment {
			t.Errorf("key '%c': expected stepComment (%d), got %d", tc.key, stepComment, d.step)
		}
	}
}

// TestFeedbackDialog_OptOutKey verifies pressing 'n' at stepRating hides dialog and records opt-out.
func TestFeedbackDialog_OptOutKey(t *testing.T) {
	d := NewFeedbackDialog()
	st := &feedback.State{FeedbackEnabled: true, MaxShows: 3, ShownCount: 1}
	sender := feedback.NewSender()
	d.Show("1.5.1", st, sender)

	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if d.IsVisible() {
		t.Error("expected dialog to be hidden after opt-out, but IsVisible() returned true")
	}
	if st.FeedbackEnabled {
		t.Error("expected FeedbackEnabled to be false after opt-out, but it is still true")
	}
}

// TestFeedbackDialog_EscAtRating_HidesWithoutOptOut verifies Esc at stepRating hides without opt-out.
func TestFeedbackDialog_EscAtRating_HidesWithoutOptOut(t *testing.T) {
	d := NewFeedbackDialog()
	st := &feedback.State{FeedbackEnabled: true, MaxShows: 3, ShownCount: 1}
	sender := feedback.NewSender()
	d.Show("1.5.1", st, sender)

	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if d.IsVisible() {
		t.Error("expected dialog to be hidden after Esc, but IsVisible() returned true")
	}
	if !st.FeedbackEnabled {
		t.Error("expected FeedbackEnabled to remain true after Esc, but it was set to false")
	}
}

// TestFeedbackDialog_EnterAtComment_ReturnsSendCmd verifies Enter at stepComment returns a non-nil cmd.
func TestFeedbackDialog_EnterAtComment_ReturnsSendCmd(t *testing.T) {
	d := NewFeedbackDialog()
	st := &feedback.State{FeedbackEnabled: true, MaxShows: 3, ShownCount: 1}
	sender := feedback.NewSender()
	d.Show("1.5.1", st, sender)

	// Advance to stepComment by pressing '3'
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if d.step != stepComment {
		t.Fatalf("expected stepComment after rating key, got %d", d.step)
	}

	// Press Enter at stepComment
	d, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("expected non-nil tea.Cmd after Enter at stepComment, got nil")
	}
	if d.step != stepSent {
		t.Errorf("expected stepSent (%d) after Enter at stepComment, got %d", stepSent, d.step)
	}
}

// TestFeedbackDialog_ViewNonEmpty verifies View() returns non-empty string when visible, empty when hidden.
func TestFeedbackDialog_ViewNonEmpty(t *testing.T) {
	d := NewFeedbackDialog()
	st := &feedback.State{FeedbackEnabled: true, MaxShows: 3, ShownCount: 1}
	sender := feedback.NewSender()

	// Hidden: should return empty string
	if v := d.View(); v != "" {
		t.Errorf("expected empty View() when hidden, got %q", v)
	}

	// Visible: should return non-empty string
	d.Show("1.5.1", st, sender)
	if v := d.View(); v == "" {
		t.Error("expected non-empty View() when visible, got empty string")
	}
}

// TestFeedbackDialog_OnDemandShortcut verifies that Show() makes the dialog visible regardless
// of opt-out state or prior rating -- i.e. on-demand bypasses ShouldShow() entirely.
// This mirrors what the ctrl+e handler does: call Show() unconditionally.
func TestFeedbackDialog_OnDemandShortcut(t *testing.T) {
	sender := feedback.NewSender()

	// Case 1: LastRatedVersion matches current version (auto-popup would block this).
	d1 := NewFeedbackDialog()
	st1 := &feedback.State{
		FeedbackEnabled:  true,
		LastRatedVersion: "1.5.1", // already rated this version
		MaxShows:         3,
		ShownCount:       1,
	}
	d1.Show("1.5.1", st1, sender)
	if !d1.IsVisible() {
		t.Error("case 1: expected dialog to be visible after on-demand Show() even though LastRatedVersion matches, but IsVisible() returned false")
	}

	// Case 2: FeedbackEnabled=false (auto-popup would block this).
	d2 := NewFeedbackDialog()
	d2.Show("1.5.1", &feedback.State{FeedbackEnabled: true, MaxShows: 3}, sender)
	d2.Hide()
	if d2.IsVisible() {
		t.Fatal("expected dialog hidden after Hide(), but IsVisible() returned true")
	}

	st2 := &feedback.State{
		FeedbackEnabled: false, // user opted out -- auto-popup would skip entirely
		MaxShows:        3,
	}
	d2.Show("1.5.1", st2, sender)
	if !d2.IsVisible() {
		t.Error("case 2: expected dialog to be visible after on-demand Show() even with FeedbackEnabled=false, but IsVisible() returned false")
	}
}
