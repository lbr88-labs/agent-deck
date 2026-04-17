package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

// homeWithRunningPreview builds a Home with one running session whose
// preview cache is primed. Used to exercise the terminal-output render
// branch of renderPreviewPane without spawning real tmux.
func homeWithRunningPreview(t *testing.T, previewContent string, width, height int) *Home {
	t.Helper()

	h := NewHome()
	h.width = width
	h.height = height
	h.initialLoading = false

	inst := session.NewInstance("clip-test", t.TempDir())
	inst.Status = session.StatusRunning
	inst.Tool = "bash" // avoid Claude-specific header branches
	inst.CreatedAt = inst.CreatedAt.Add(-1)

	h.instancesMu.Lock()
	h.instances = []*session.Instance{inst}
	h.instanceByID[inst.ID] = inst
	h.instancesMu.Unlock()

	h.flatItems = []session.Item{{Type: session.ItemTypeSession, Session: inst}}
	h.cursor = 0
	h.setHotkeys(resolveHotkeys(nil))

	h.previewCacheMu.Lock()
	h.previewCache[inst.ID] = previewContent
	h.previewCacheTime[inst.ID] = inst.CreatedAt
	h.previewCacheMu.Unlock()

	return h
}

// Test 1: A Neovim-style statusline captured from a wider tmux session
// ends with CSI K (Erase in Line). When rendered inside the preview pane,
// CSI K causes the outer terminal to fill the rest of the OUTER row with
// the active SGR background — bleeding past the preview boundary.
//
// Regression test for issue #579: "Preview pane doesn't clip terminal
// content — Neovim statusline renders outside boundary".
//
// Expected behavior after fix: renderPreviewPane must NOT emit CSI K
// (ESC [ K) in its output. That erase-in-line escape must be stripped
// from captured content so the outer terminal never receives it.
func TestRenderPreviewPane_StripsEraseInLine_Issue579(t *testing.T) {
	// Simulate what `tmux capture-pane -e` emits for a Neovim statusline
	// on a wider original pane: a bg-color SGR, some text, then CSI K
	// to fill the rest of the LINE with the active bg in the outer terminal.
	nvimStatusline := "\x1b[42;30m NORMAL \x1b[0m\x1b[42m " +
		strings.Repeat(" ", 20) + " main.go \x1b[K"

	h := homeWithRunningPreview(t, nvimStatusline+"\n", 40, 20)
	rendered := h.renderPreviewPane(40, 20)

	if strings.Contains(rendered, "\x1b[K") {
		t.Fatalf("preview must not emit CSI K (Erase in Line); it causes the outer terminal to paint the active bg past the preview boundary.\nrendered=%q", rendered)
	}
}

// Test 2: CSI J (Erase in Display) has the same bleed problem as CSI K —
// it tells the outer terminal to erase to end of screen with the active
// bg color. Must also be stripped.
func TestRenderPreviewPane_StripsEraseInDisplay_Issue579(t *testing.T) {
	content := "\x1b[41m bleed? \x1b[J\n"
	h := homeWithRunningPreview(t, content, 40, 20)
	rendered := h.renderPreviewPane(40, 20)

	if strings.Contains(rendered, "\x1b[J") {
		t.Fatalf("preview must not emit CSI J (Erase in Display).\nrendered=%q", rendered)
	}
}

// Test 3: A line of bg-colored content wider than the preview must be
// truncated AND its trailing SGR state must be reset so it cannot leak
// color into adjacent joined panels or subsequent rows.
//
// Concretely: after ansi.Truncate, there must exist an SGR reset
// (ESC [ 0 m OR ESC [ m) before the final newline of every rendered
// content line that opened with an SGR. Without a reset, lipgloss
// horizontal-join paints the active bg into the gap between panels.
func TestRenderPreviewPane_ResetsSGRAtEndOfLines_Issue579(t *testing.T) {
	// Bg-colored line wider than preview — will be truncated.
	// Note: contains NO CSI K; the only way for bg to leak is an
	// unreset SGR at the end of the line.
	longBgLine := "\x1b[42m" + strings.Repeat(" ", 200) + "\n"

	width := 40
	h := homeWithRunningPreview(t, longBgLine, width, 20)
	rendered := h.renderPreviewPane(width, 20)

	lines := strings.Split(rendered, "\n")
	var offending []string
	resetRE := "\x1b[0m"
	altResetRE := "\x1b[m"
	for _, line := range lines {
		// Only check lines that actually turned on a bg color.
		if !strings.Contains(line, "\x1b[42m") {
			continue
		}
		// Find the last SGR reset in the line.
		idxReset := strings.LastIndex(line, resetRE)
		idxAltReset := strings.LastIndex(line, altResetRE)
		idxBg := strings.LastIndex(line, "\x1b[42m")
		lastReset := idxReset
		if idxAltReset > lastReset {
			lastReset = idxAltReset
		}
		if lastReset < idxBg {
			offending = append(offending, line)
		}
	}
	if len(offending) > 0 {
		t.Fatalf("each bg-colored preview line must end with an SGR reset to prevent color leakage into adjacent panels.\nrendered=%q\noffending=%v", rendered, offending)
	}
}

// Test 4: Every rendered line's visible width must fit within the
// preview pane's budget (width - margin). This locks down the core
// clip guarantee that issue #579 reports violated — without it,
// Neovim's full-width statusline extends past the right edge.
func TestRenderPreviewPane_EveryLineFitsWidth_Issue579(t *testing.T) {
	// A line much wider than the pane, with ANSI BG so we exercise
	// the ansi-aware width path.
	wideLine := "\x1b[42m" + strings.Repeat("x", 300) + "\x1b[0m\n"

	width := 60
	h := homeWithRunningPreview(t, wideLine, width, 20)
	rendered := h.renderPreviewPane(width, 20)

	for i, line := range strings.Split(rendered, "\n") {
		w := ansi.StringWidth(line)
		if w > width {
			t.Fatalf("line %d exceeds preview width %d (visible=%d): %q\nfull rendered=%q", i, width, w, line, rendered)
		}
	}
}
