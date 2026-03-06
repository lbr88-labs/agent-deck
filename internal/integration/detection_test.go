package integration

import (
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/session"
	"github.com/asheshgoplani/agent-deck/internal/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// DETECT-01: Pattern detection tests per tool
// =============================================================================

// TestDetection_ClaudeBusy verifies that PromptDetector("claude").HasPrompt returns
// false when Claude is busy (spinner, ctrl+c, whimsical words with timing).
func TestDetection_ClaudeBusy(t *testing.T) {
	detector := tmux.NewPromptDetector("claude")

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "spinner with whimsical word",
			content: "Some output\n\u2722 Clauding\u2026 (25s \u00b7 \u2193 749 tokens)\n",
		},
		{
			name:    "ctrl+c to interrupt",
			content: "Working on request\nctrl+c to interrupt\n",
		},
		{
			name:    "whimsical ellipsis and tokens",
			content: "Output\n\u2026 tokens in progress\n\u2722 Pondering\u2026 (10s \u00b7 \u2193 200 tokens)\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, detector.HasPrompt(tc.content),
				"HasPrompt should return false for busy content: %s", tc.name)
		})
	}
}

// TestDetection_ClaudeWaiting verifies that PromptDetector("claude").HasPrompt returns
// true when Claude is waiting for user input (prompt, permission dialog, trust prompt).
func TestDetection_ClaudeWaiting(t *testing.T) {
	detector := tmux.NewPromptDetector("claude")

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "input prompt skip-permissions mode",
			content: "Task completed.\n\u276f \n",
		},
		{
			name:    "permission dialog",
			content: "\u2502 Do you want to run this command?\n\u276f Yes, allow once\n",
		},
		{
			name:    "trust prompt",
			content: "Do you trust the files in this folder?\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, detector.HasPrompt(tc.content),
				"HasPrompt should return true for waiting content: %s", tc.name)
		})
	}
}

// TestDetection_GeminiBusy verifies that PromptDetector("gemini").HasPrompt returns
// false when Gemini shows busy indicators.
func TestDetection_GeminiBusy(t *testing.T) {
	detector := tmux.NewPromptDetector("gemini")

	// Note: Gemini's HasPrompt checks for prompt patterns but doesn't have
	// explicit busy indicators in the current detector implementation.
	// The "esc to cancel" text is a busy indicator from DefaultRawPatterns,
	// but the PromptDetector's hasGeminiPrompt checks last 10 non-blank lines
	// for prompt patterns. If none match, HasPrompt returns false.
	content := "Processing your request\nesc to cancel\n"
	assert.False(t, detector.HasPrompt(content),
		"HasPrompt should return false for Gemini busy content")
}

// TestDetection_GeminiWaiting verifies that PromptDetector("gemini").HasPrompt returns
// true when Gemini shows prompt indicators.
func TestDetection_GeminiWaiting(t *testing.T) {
	detector := tmux.NewPromptDetector("gemini")

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "gemini prompt",
			content: "Previous output\ngemini>\n",
		},
		{
			name:    "type your message",
			content: "Welcome to Gemini CLI\nType your message\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, detector.HasPrompt(tc.content),
				"HasPrompt should return true for Gemini waiting content: %s", tc.name)
		})
	}
}

// TestDetection_OpenCodeBusy verifies that PromptDetector("opencode").HasPrompt returns
// false when OpenCode shows busy indicators.
func TestDetection_OpenCodeBusy(t *testing.T) {
	detector := tmux.NewPromptDetector("opencode")

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "pulse spinner char",
			content: "Processing\n\u2588 Working on your request\n",
		},
		{
			name:    "esc interrupt",
			content: "Running task\nesc interrupt\n",
		},
		{
			name:    "thinking text",
			content: "OpenCode\nThinking...\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, detector.HasPrompt(tc.content),
				"HasPrompt should return false for OpenCode busy content: %s", tc.name)
		})
	}
}

// TestDetection_OpenCodeWaiting verifies that PromptDetector("opencode").HasPrompt returns
// true when OpenCode shows prompt indicators.
func TestDetection_OpenCodeWaiting(t *testing.T) {
	detector := tmux.NewPromptDetector("opencode")

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "ask anything",
			content: "Welcome to OpenCode\nAsk anything\n",
		},
		{
			name:    "press enter to send",
			content: "Output here\npress enter to send\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, detector.HasPrompt(tc.content),
				"HasPrompt should return true for OpenCode waiting content: %s", tc.name)
		})
	}
}

// TestDetection_CodexBusy verifies that PromptDetector("codex").HasPrompt returns
// false when Codex shows busy indicators.
func TestDetection_CodexBusy(t *testing.T) {
	detector := tmux.NewPromptDetector("codex")

	content := "Running your task\nesc to interrupt\n"
	assert.False(t, detector.HasPrompt(content),
		"HasPrompt should return false for Codex busy content")
}

// TestDetection_CodexWaiting verifies that PromptDetector("codex").HasPrompt returns
// true when Codex shows prompt indicators.
func TestDetection_CodexWaiting(t *testing.T) {
	detector := tmux.NewPromptDetector("codex")

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "codex prompt",
			content: "Ready\ncodex>\n",
		},
		{
			name:    "continue prompt",
			content: "Task paused\nContinue?\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, detector.HasPrompt(tc.content),
				"HasPrompt should return true for Codex waiting content: %s", tc.name)
		})
	}
}

// =============================================================================
// DETECT-02: DefaultRawPatterns, CompilePatterns, ToolConfig
// =============================================================================

// TestDetection_DefaultPatternsExist verifies that DefaultRawPatterns returns non-nil
// for all four supported tools and nil for unknown tools.
func TestDetection_DefaultPatternsExist(t *testing.T) {
	knownTools := []string{"claude", "gemini", "opencode", "codex"}
	for _, tool := range knownTools {
		t.Run(tool, func(t *testing.T) {
			raw := tmux.DefaultRawPatterns(tool)
			assert.NotNil(t, raw, "DefaultRawPatterns(%q) should return non-nil", tool)
		})
	}

	t.Run("unknown tool returns nil", func(t *testing.T) {
		raw := tmux.DefaultRawPatterns("unknown-tool-xyz")
		assert.Nil(t, raw, "DefaultRawPatterns for unknown tool should return nil")
	})
}

// TestDetection_CompilePatterns verifies that CompilePatterns on Claude's
// DefaultRawPatterns produces valid ResolvedPatterns with populated fields.
func TestDetection_CompilePatterns(t *testing.T) {
	raw := tmux.DefaultRawPatterns("claude")
	require.NotNil(t, raw, "DefaultRawPatterns for claude must exist")

	resolved, err := tmux.CompilePatterns(raw)
	require.NoError(t, err, "CompilePatterns should not error")
	require.NotNil(t, resolved, "ResolvedPatterns should not be nil")

	// Claude patterns include "re:" prefixed regex patterns, so BusyRegexps should be populated
	assert.NotEmpty(t, resolved.BusyRegexps, "BusyRegexps should contain compiled regex patterns")

	// SpinnerChars should be copied from raw
	assert.NotEmpty(t, resolved.SpinnerChars, "SpinnerChars should be populated")
	assert.Equal(t, len(raw.SpinnerChars), len(resolved.SpinnerChars),
		"SpinnerChars count should match raw patterns")

	// Claude has WhimsicalWords + SpinnerChars, so combo patterns should be built
	assert.NotNil(t, resolved.ThinkingPattern, "ThinkingPattern should be compiled")
	assert.NotNil(t, resolved.ThinkingPatternEllipsis, "ThinkingPatternEllipsis should be compiled")
	assert.NotNil(t, resolved.SpinnerActivePattern, "SpinnerActivePattern should be compiled")
}

// TestDetection_ToolConfig verifies that NewInstanceWithTool correctly sets the
// Tool field for each supported tool type.
func TestDetection_ToolConfig(t *testing.T) {
	tools := []string{"claude", "gemini", "opencode", "codex", "shell"}

	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			inst := session.NewInstanceWithTool("test-"+tool, "/tmp", tool)
			assert.Equal(t, tool, inst.Tool,
				"NewInstanceWithTool(%q) should set Tool field correctly", tool)
		})
	}
}
