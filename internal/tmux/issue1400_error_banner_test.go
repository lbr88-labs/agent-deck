package tmux

import "testing"

// Issue #1400 (second half): the registry reported "waiting" while a 401
// error banner was visible in the pane. Error banners Claude Code renders
// (auth failure: "API Error: 401" / "Please run /login"; connection failure:
// "socket connection closed") must classify as error status.
//
// These table-driven tests pin PromptDetector.HasErrorBanner — the pure
// pane-content heuristic — including the over-match guards: a user merely
// DISCUSSING a 401 (typed at the prompt, inside the input box, or quoted in a
// tool result) must NOT flip the session to error.

func TestClaudeErrorBanner_DetectsRenderedBanners(t *testing.T) {
	detector := NewPromptDetector("claude")

	cases := []struct {
		name    string
		content string
	}{
		{
			// Field evidence shape: auth failure rendered as the turn's reply.
			name: "401 auth failure with login instruction (assistant-level line)",
			content: "⏺ Please run /login · API Error: 401 {\"type\":\"error\",\"error\":{\"type\":\"authentication_error\",\"message\":\"Invalid authentication credentials\"},\"request_id\":\"req_011CaU1BfZ8vvqHH3qFzEEeX\"}\n" +
				"\n" +
				"❯ \n" +
				"  ? for shortcuts",
		},
		{
			name: "standalone API Error 401 line",
			content: "some earlier output\n" +
				"API Error: 401 {\"type\":\"error\",\"error\":{\"type\":\"authentication_error\",\"message\":\"OAuth token has expired\"}}\n" +
				"\n" +
				"❯ ",
		},
		{
			name: "parenthesized API Error 401 variant",
			content: "API Error (401 {\"type\":\"error\",\"error\":{\"type\":\"authentication_error\"}}) · Please run /login\n" +
				"\n" +
				"❯ ",
		},
		{
			name: "bare login instruction banner",
			content: "Please run /login\n" +
				"\n" +
				"❯ ",
		},
		{
			name: "socket connection closed banner",
			content: "API Error (Connection error.) · socket connection closed\n" +
				"\n" +
				"❯ ",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !detector.HasErrorBanner(tc.content) {
				t.Errorf("should detect error banner: %s", tc.name)
			}
		})
	}
}

func TestClaudeErrorBanner_DoesNotOverMatch(t *testing.T) {
	detector := NewPromptDetector("claude")

	cases := []struct {
		name    string
		content string
	}{
		{
			// User typing ABOUT a 401 at the prompt.
			name:    "user discussing a 401 at the input prompt",
			content: "⏺ Done with the refactor.\n\n❯ why did the worker show API Error: 401 and Please run /login yesterday?",
		},
		{
			// Input-box (bordered) variant of the same.
			name: "user discussing a 401 inside the bordered input box",
			content: "⏺ Done with the refactor.\n" +
				"╭──────────────────────────────────────────╮\n" +
				"│ > the child hit API Error: 401, fix it?  │\n" +
				"╰──────────────────────────────────────────╯",
		},
		{
			// A conductor reading an errored child's pane: the child's banner
			// is QUOTED inside a tool result (⎿ connector) — the conductor
			// itself is fine.
			name: "tool result quoting another session's 401 banner",
			content: "⏺ Bash(agent-deck session output worker-3 -q)\n" +
				"  ⎿  Please run /login · API Error: 401 {\"type\":\"error\",\"error\":{\"type\":\"authentication_error\"}}\n" +
				"\n" +
				"❯ ",
		},
		{
			// Retry in progress renders behind the tool-result connector and a
			// busy spinner; the banner heuristic must not fire on the quoted
			// retry line (busy detection owns this state).
			name: "API error retry notice behind tool-result connector",
			content: "✻ Reticulating… (3s · ↑ 50 tokens · ctrl+c to interrupt)\n" +
				"  ⎿  API Error: 401 · Retrying in 4 seconds… (attempt 3/10)",
		},
		{
			name:    "ordinary conversation about errors",
			content: "⏺ The API returned an authentication error earlier; I retried and it succeeded.\n\n❯ ",
		},
		{
			// Banner scrolled out of the recent-tail window (>15 non-empty
			// lines above the bottom) is stale scrollback, not current state.
			name: "stale banner outside the 15-line window",
			content: "API Error: 401 {\"type\":\"error\"}\n" +
				"line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\n" +
				"line9\nline10\nline11\nline12\nline13\nline14\nline15\nline16\n" +
				"❯ ",
		},
		{
			name:    "healthy waiting pane",
			content: "⏺ All tests pass.\n\n❯ \n  ? for shortcuts",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if detector.HasErrorBanner(tc.content) {
				t.Errorf("should NOT detect error banner: %s", tc.name)
			}
		})
	}
}

// TestErrorBanner_NonClaudeToolsReturnFalse pins the tool gate: the banner
// shapes are Claude Code renderings; other tools must not match.
func TestErrorBanner_NonClaudeToolsReturnFalse(t *testing.T) {
	content := "API Error: 401 · Please run /login\n❯ "
	for _, tool := range []string{"codex", "gemini", "opencode", "shell"} {
		if NewPromptDetector(tool).HasErrorBanner(content) {
			t.Errorf("tool %q must not match Claude error banners", tool)
		}
	}
}

// TestSessionErrorBannerIndicator pins the Session-level wrapper used by
// GetStatus: tool inference from the session's command, claude-only.
func TestSessionErrorBannerIndicator(t *testing.T) {
	banner := "Please run /login · API Error: 401 {\"type\":\"error\"}\n❯ "

	claudeSess := &Session{Command: "claude"}
	if !claudeSess.hasErrorBannerIndicator(banner) {
		t.Error("claude session should detect the error banner")
	}

	shellSess := &Session{Command: "bash"}
	if shellSess.hasErrorBannerIndicator(banner) {
		t.Error("non-claude session must not detect Claude error banners")
	}
}
