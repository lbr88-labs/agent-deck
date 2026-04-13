package feedback

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

// State holds the persisted feedback preferences for a user.
// File: ~/.agent-deck/feedback-state.json. Always serializes all fields (D-05).
type State struct {
	LastRatedVersion string `json:"last_rated_version"`
	FeedbackEnabled  bool   `json:"feedback_enabled"`
	ShownCount       int    `json:"shown_count"`
	MaxShows         int    `json:"max_shows"`
}

// defaultState returns an initialized State with safe defaults.
func defaultState() *State {
	return &State{
		FeedbackEnabled: true,
		MaxShows:        3,
	}
}

// statePath returns the absolute path to the feedback state file.
func statePath() (string, error) {
	dir, err := session.GetAgentDeckDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "feedback-state.json"), nil
}

// LoadState reads ~/.agent-deck/feedback-state.json and returns the state.
// If the file does not exist, it returns a default State (FeedbackEnabled=true, MaxShows=3).
// A missing file is NOT an error. A malformed file returns a default state to prevent crashes.
func LoadState() (*State, error) {
	path, err := statePath()
	if err != nil {
		return defaultState(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultState(), nil
		}
		return defaultState(), nil
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		// Malformed file — return default to prevent crash (T-01-03)
		return defaultState(), nil
	}
	return &s, nil
}

// SaveState atomically writes the state to ~/.agent-deck/feedback-state.json.
// Uses tmp+rename to prevent partial writes (T-01-01).
func SaveState(s *State) error {
	dir, err := session.GetAgentDeckDir()
	if err != nil {
		return fmt.Errorf("feedback: get agent-deck dir: %w", err)
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("feedback: create dir: %w", err)
	}

	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("feedback: marshal state: %w", err)
	}

	path := filepath.Join(dir, "feedback-state.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("feedback: write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("feedback: rename tmp: %w", err)
	}
	return nil
}

// ShouldShow returns true only when all three conditions are met:
// 1. feedback_enabled is true
// 2. last_rated_version does not match currentVersion
// 3. shown_count < max_shows
func ShouldShow(s *State, currentVersion string) bool {
	return s.FeedbackEnabled &&
		s.LastRatedVersion != currentVersion &&
		s.ShownCount < s.MaxShows
}

// RecordShown increments shown_count by 1. Does NOT save — caller must call SaveState.
func RecordShown(s *State) {
	s.ShownCount++
}

// RecordRating sets last_rated_version to currentVersion and resets shown_count to 0.
// Does NOT save — caller must call SaveState.
func RecordRating(s *State, currentVersion string, rating int) {
	s.LastRatedVersion = currentVersion
	s.ShownCount = 0
	_ = rating // rating is used by the caller for display/formatting; stored externally
}

// RecordOptOut sets feedback_enabled to false (permanent opt-out).
// Does NOT save — caller must call SaveState.
func RecordOptOut(s *State) {
	s.FeedbackEnabled = false
}

// RatingEmoji maps a numeric rating (1-5) to an emoji.
// Returns "" for out-of-range values.
func RatingEmoji(rating int) string {
	switch rating {
	case 1:
		return "😞"
	case 2:
		return "😐"
	case 3:
		return "🙂"
	case 4:
		return "😀"
	case 5:
		return "🤩"
	default:
		return ""
	}
}

// FormatComment formats a feedback submission for posting to GitHub Discussions.
// Format: "**vVER** | **N/5** EMOJI | GOOS GOARCH\nCOMMENT"
// When comment is empty, the trailing newline and comment are omitted.
func FormatComment(version string, rating int, goos, goarch, comment string) string {
	header := fmt.Sprintf("**v%s** | **%d/5** %s | %s %s", version, rating, RatingEmoji(rating), goos, goarch)
	if comment == "" {
		return header
	}
	return header + "\n" + comment
}
