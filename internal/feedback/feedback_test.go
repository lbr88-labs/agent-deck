package feedback_test

import (
	"os"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/feedback"
	"github.com/stretchr/testify/require"
)

// TEST-01: ShouldShow returns true when this is a new version
func TestShouldShow_NewVersion(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	st := &feedback.State{
		LastRatedVersion: "1.0.0",
		FeedbackEnabled:  true,
		ShownCount:       0,
		MaxShows:         3,
	}
	require.NoError(t, feedback.SaveState(st))

	loaded, err := feedback.LoadState()
	require.NoError(t, err)
	require.True(t, feedback.ShouldShow(loaded, "1.5.1"))
}

// TEST-02: ShouldShow returns false when already rated this version
func TestShouldShow_AlreadyRated(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	st := &feedback.State{
		LastRatedVersion: "1.5.1",
		FeedbackEnabled:  true,
		ShownCount:       0,
		MaxShows:         3,
	}
	require.NoError(t, feedback.SaveState(st))

	loaded, err := feedback.LoadState()
	require.NoError(t, err)
	require.False(t, feedback.ShouldShow(loaded, "1.5.1"))
}

// TEST-03: ShouldShow returns false when user opted out
func TestShouldShow_OptedOut(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	st := &feedback.State{
		LastRatedVersion: "1.0.0",
		FeedbackEnabled:  false,
		ShownCount:       0,
		MaxShows:         3,
	}
	require.NoError(t, feedback.SaveState(st))

	loaded, err := feedback.LoadState()
	require.NoError(t, err)
	require.False(t, feedback.ShouldShow(loaded, "1.5.1"))
}

// TEST-04: ShouldShow returns false when shown_count >= max_shows
func TestShouldShow_MaxShows(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	st := &feedback.State{
		LastRatedVersion: "1.0.0",
		FeedbackEnabled:  true,
		ShownCount:       3,
		MaxShows:         3,
	}
	require.NoError(t, feedback.SaveState(st))

	loaded, err := feedback.LoadState()
	require.NoError(t, err)
	require.False(t, feedback.ShouldShow(loaded, "1.5.1"))
}

// TEST-05: RecordRating sets last_rated_version and resets shown_count
func TestRecordRating_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	st := &feedback.State{
		LastRatedVersion: "1.0.0",
		FeedbackEnabled:  true,
		ShownCount:       2,
		MaxShows:         3,
	}
	feedback.RecordRating(st, "1.5.1", 4)
	require.NoError(t, feedback.SaveState(st))

	loaded, err := feedback.LoadState()
	require.NoError(t, err)
	require.Equal(t, "1.5.1", loaded.LastRatedVersion)
	require.Equal(t, 0, loaded.ShownCount)
}

// TEST-06: RecordOptOut sets feedback_enabled to false (persisted)
func TestRecordOptOut(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	st := &feedback.State{
		LastRatedVersion: "1.0.0",
		FeedbackEnabled:  true,
		ShownCount:       0,
		MaxShows:         3,
	}
	feedback.RecordOptOut(st)
	require.NoError(t, feedback.SaveState(st))

	loaded, err := feedback.LoadState()
	require.NoError(t, err)
	require.False(t, loaded.FeedbackEnabled)
}

// TEST-07: RecordShown increments shown_count (persisted)
func TestRecordShown(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	st := &feedback.State{
		LastRatedVersion: "1.0.0",
		FeedbackEnabled:  true,
		ShownCount:       0,
		MaxShows:         3,
	}
	feedback.RecordShown(st)
	require.NoError(t, feedback.SaveState(st))

	loaded, err := feedback.LoadState()
	require.NoError(t, err)
	require.Equal(t, 1, loaded.ShownCount)
}

// TEST-08: FormatComment returns exact formatted string
func TestFormatComment(t *testing.T) {
	result := feedback.FormatComment("1.5.1", 4, "darwin", "arm64", "scrollback fix")
	require.Equal(t, "**v1.5.1** | **4/5** 😀 | darwin arm64\nscrollback fix", result)
}

// TEST-09: RatingEmoji maps 1-5 to correct emojis
func TestRatingEmoji(t *testing.T) {
	require.Equal(t, "😞", feedback.RatingEmoji(1))
	require.Equal(t, "😐", feedback.RatingEmoji(2))
	require.Equal(t, "🙂", feedback.RatingEmoji(3))
	require.Equal(t, "😀", feedback.RatingEmoji(4))
	require.Equal(t, "🤩", feedback.RatingEmoji(5))
}
