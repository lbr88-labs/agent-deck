package session

import "testing"

// #1238: the post-send delivery verification (issue #876) keys off
// Claude-specific TUI signals — an "active" status transition, the composer
// glyph, and unsent-paste markers. Non-Claude tools (codex #1205, codewhale,
// gemini #876) never surface those, so the verify false-negatives and reports a
// delivered message as "dropped silently". UsesClaudeDeliveryVerify is the
// single tool-aware predicate that decides whether the Claude-tuned verify
// applies. It must be true ONLY for Claude-compatible tools — the general
// superset of #1228's codex-only skip.
func TestUsesClaudeDeliveryVerify(t *testing.T) {
	cases := []struct {
		tool string
		want bool
	}{
		{"claude", true},
		{"codex", false},     // #1205
		{"codewhale", false}, // #1238 (the reporter's tool)
		{"gemini", false},    // #876
		{"opencode", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := UsesClaudeDeliveryVerify(tc.tool); got != tc.want {
			t.Errorf("UsesClaudeDeliveryVerify(%q) = %v, want %v", tc.tool, got, tc.want)
		}
	}
}

// TestUsesClaudeDeliveryVerify_SupersetOfCodex pins the relationship to #1228:
// every tool #1228 skips (codex-compatible) is also skipped here, and the new
// behavior additionally skips the rest of the non-Claude tools.
func TestUsesClaudeDeliveryVerify_SupersetOfCodex(t *testing.T) {
	// codex is a strict subset of the tools that skip the Claude-tuned verify.
	if IsCodexCompatible("codex") && UsesClaudeDeliveryVerify("codex") {
		t.Fatal("codex must skip the Claude-tuned verify (subsumes #1228)")
	}
	// A non-codex, non-Claude tool must also skip — that is the superset.
	if UsesClaudeDeliveryVerify("codewhale") {
		t.Fatal("non-Claude tools beyond codex must also skip the Claude-tuned verify")
	}
}
