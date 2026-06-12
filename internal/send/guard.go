package send

import (
	"strings"
	"time"
)

// IsComposerPlaceholder reports whether the visible composer text is Claude's
// idle-suggestion placeholder rather than operator input. Claude renders hint
// suggestions in the empty composer, e.g.:
//
//	❯ Try "write a test for <filepath>"
//
// Treating these as operator drafts would make every automated send hold and
// Ctrl+C an actually-empty composer (issue #1409).
func IsComposerPlaceholder(text string) bool {
	t := strings.TrimSpace(text)
	return strings.HasPrefix(t, `Try "`) && strings.HasSuffix(t, `"`)
}

// ComposerDraft returns the normalized text currently sitting in the visible
// composer and whether a composer is visible at all. Claude's idle-suggestion
// placeholder is reported as an empty draft. Callers must pass ANSI-stripped
// pane content (same contract as CurrentComposerPrompt).
func ComposerDraft(content string) (draft string, composerVisible bool) {
	body, ok := CurrentComposerPrompt(content)
	if !ok {
		return "", false
	}
	body = NormalizePromptText(body)
	if IsComposerPlaceholder(body) {
		return "", true
	}
	return body, true
}

// ComposerHasDraft reports whether the visible composer holds operator input.
// This is the shared "is the composer busy?" check automated senders must run
// before injecting keystrokes into the pane (issue #1409).
func ComposerHasDraft(content string) bool {
	draft, visible := ComposerDraft(content)
	return visible && draft != ""
}

// ComposerGuardTarget is the minimal pane surface GuardComposerDraft needs to
// hold an automated send while an operator draft occupies the composer.
// *tmux.Session satisfies it.
type ComposerGuardTarget interface {
	CapturePaneFresh() (string, error)
	SendCtrlC() error
}

// ComposerGuardOptions tunes GuardComposerDraft. All bounds are mandatory so
// the guard can never hold a delivery indefinitely.
type ComposerGuardOptions struct {
	// HoldWait is the maximum time to wait for an operator draft to clear on
	// its own (operator submits or erases it) before falling back to
	// save-clear-restore.
	HoldWait time.Duration
	// PollInterval is the capture cadence during the hold phase.
	// Defaults to 250ms when <= 0.
	PollInterval time.Duration
	// ClearWait is the maximum time to wait, per Ctrl+C attempt, for the
	// composer to actually clear.
	ClearWait time.Duration
	// Strip is applied to raw captured pane content before composer
	// introspection (pass tmux.StripANSI). nil means identity.
	Strip func(string) string
}

// ComposerGuardResult reports what the guard did.
type ComposerGuardResult struct {
	// Held is the total wall-clock time the guard spent before returning.
	Held time.Duration
	// SavedDraft is the operator draft that was cleared to make way for the
	// automated send. Empty when the composer was empty or cleared on its
	// own. Callers must restore it (type it back, without Enter) after the
	// automated delivery is confirmed.
	SavedDraft string
	// DraftCleared is true when the guard issued Ctrl+C and confirmed the
	// composer emptied.
	DraftCleared bool
	// ClearFailed is true when Ctrl+C attempts were exhausted and the
	// composer still held the draft. The caller proceeds with the send
	// regardless (delivery must not be dropped), accepting the residual
	// merge risk for this pathological case.
	ClearFailed bool
}

// maxComposerClearAttempts bounds Ctrl+C attempts during save-clear.
const maxComposerClearAttempts = 2

// GuardComposerDraft implements the composer-collision guard for automated
// sends (issue #1409): an automated SendKeysAndEnter against a composer that
// already holds half-typed operator input would merge with it and submit the
// merged prompt. The guard:
//
//  1. Holds (bounded by HoldWait) while the composer shows a non-empty
//     operator draft, polling for it to clear on its own.
//  2. If the draft is still present at the bound, saves it, clears the
//     composer with Ctrl+C (Claude clears the current input on a single
//     Ctrl+C; same primitive the full-resend recovery path already uses)
//     and confirms the clear, bounded by ClearWait per attempt.
//
// The guard never blocks delivery indefinitely and never errors: on capture
// failures or a composer that refuses to clear it returns and lets the caller
// proceed, because watchers/conductors depend on the send going through.
func GuardComposerDraft(t ComposerGuardTarget, opts ComposerGuardOptions) ComposerGuardResult {
	strip := opts.Strip
	if strip == nil {
		strip = func(s string) string { return s }
	}
	poll := opts.PollInterval
	if poll <= 0 {
		poll = 250 * time.Millisecond
	}

	start := time.Now()
	deadline := start.Add(opts.HoldWait)
	lastDraft := ""

	for {
		raw, err := t.CapturePaneFresh()
		if err != nil {
			// Pane not introspectable: never block delivery on it.
			return ComposerGuardResult{Held: time.Since(start)}
		}
		draft, visible := ComposerDraft(strip(raw))
		if !visible || draft == "" {
			return ComposerGuardResult{Held: time.Since(start)}
		}
		lastDraft = draft
		if !time.Now().Before(deadline) {
			break
		}
		sleepFor := poll
		if remaining := time.Until(deadline); remaining < sleepFor {
			sleepFor = remaining
		}
		if sleepFor > 0 {
			time.Sleep(sleepFor)
		}
	}

	// Hold bound reached with the operator draft still present: save it and
	// clear the composer so the automated message cannot merge with it.
	res := ComposerGuardResult{SavedDraft: lastDraft}
	clearPoll := poll
	if clearPoll > 100*time.Millisecond {
		clearPoll = 100 * time.Millisecond
	}
	for attempt := 0; attempt < maxComposerClearAttempts; attempt++ {
		if err := t.SendCtrlC(); err != nil {
			break
		}
		clearDeadline := time.Now().Add(opts.ClearWait)
		for {
			raw, err := t.CapturePaneFresh()
			if err == nil && !ComposerHasDraft(strip(raw)) {
				res.DraftCleared = true
				res.Held = time.Since(start)
				return res
			}
			if !time.Now().Before(clearDeadline) {
				break
			}
			sleepFor := clearPoll
			if remaining := time.Until(clearDeadline); remaining < sleepFor {
				sleepFor = remaining
			}
			if sleepFor > 0 {
				time.Sleep(sleepFor)
			}
		}
	}
	res.ClearFailed = true
	res.Held = time.Since(start)
	return res
}
