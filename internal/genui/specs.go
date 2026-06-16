package genui

import (
	"encoding/json"
	"fmt"
)

// This file holds the 2-3 HAND-AUTHORED whole-UI view specs that prove the
// engine (v-genui-0, NO LLM). All three bind the SAME fleet snapshot data by
// reference; switching specs reshapes the WHOLE UI live. They are derived from
// Ashesh's real default views (AGENT-DECK-USAGE-ANALYSIS.md §6 / research plan
// Part 2 §2.4):
//
//   1. status-board   — the always-on "how's it going": fleet + active sessions
//   2. blocked-first  — decisions-waiting + stuck/error surfaced on top
//   3. by-project     — grouped per conductor/project
//
// These are stored as JSON strings (not Go structs) so they are exactly the
// inert DATA a future LLM would emit, and they round-trip through the SAME
// validator the renderer trusts.

// ViewIDs is the ordered list of the hand-authored views (drives the UI switch).
var ViewIDs = []string{"status-board", "blocked-first", "by-project"}

// ViewTitle is a friendly label per view id.
var ViewTitle = map[string]string{
	"status-board":  "Status board",
	"blocked-first": "Blocked first",
	"by-project":    "By project",
}

// SpecJSON returns the raw JSON for a hand-authored view by id.
func SpecJSON(id string) ([]byte, bool) {
	s, ok := handAuthored[id]
	if !ok {
		return nil, false
	}
	return []byte(s), true
}

// LoadValidatedSpec returns a hand-authored view, validated. This guarantees
// the shipped defaults pass the same gate as any generated spec — if one of
// these ever drifts out of the schema, the server fails loudly rather than
// shipping an invalid default.
func LoadValidatedSpec(id string) (*Spec, error) {
	raw, ok := SpecJSON(id)
	if !ok {
		return nil, fmt.Errorf("unknown view %q", id)
	}
	return ValidateBytes(raw)
}

// MustValidateAll validates every hand-authored spec; used by a startup/test
// assertion so a malformed default is caught immediately.
func MustValidateAll() error {
	for _, id := range ViewIDs {
		if _, err := LoadValidatedSpec(id); err != nil {
			return fmt.Errorf("hand-authored spec %q invalid: %w", id, err)
		}
	}
	return nil
}

// compact re-marshals a JSON literal so the embedded specs are stored compactly
// and we get a parse-time check that the literal is well-formed JSON.
func compact(s string) string {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		panic("genui: malformed hand-authored spec literal: " + err.Error())
	}
	out, _ := json.Marshal(v)
	return string(out)
}

// handAuthored maps view id -> compacted spec JSON.
var handAuthored = map[string]string{
	"status-board":  compact(specStatusBoard),
	"blocked-first": compact(specBlockedFirst),
	"by-project":    compact(specByProject),
}

// --- 1. STATUS BOARD ------------------------------------------------------
// The always-on "how's it going" view: a totals strip, then every conductor
// with its live counts, then the active sessions. The single most-wanted view.
const specStatusBoard = `{
  "schema": 1,
  "specId": "status-board",
  "title": "Status board — how's the fleet going",
  "version": 1,
  "root": {
    "type": "col", "gap": "lg",
    "children": [
      { "type": "heading", "level": 1, "text": "Status board" },
      { "type": "row", "gap": "md", "children": [
        { "type": "stat", "label": "Running", "tone": "ok",      "bind": "totals.running" },
        { "type": "stat", "label": "Waiting", "tone": "warn",    "bind": "totals.waiting" },
        { "type": "stat", "label": "Idle",    "tone": "neutral", "bind": "totals.idle" }
      ]},
      { "type": "section", "text": "The fleet — what each is doing", "gap": "md", "children": [
        { "type": "col", "gap": "sm", "children": [
          { "type": "status-list", "bind": "conductors" }
        ]}
      ]},
      { "type": "section", "text": "Active sessions across everything", "gap": "md", "children": [
        { "type": "session-list", "bind": "sessions" }
      ]}
    ]
  }
}`

// --- 2. BLOCKED FIRST -----------------------------------------------------
// Decisions waiting on a human, then anything stuck/error, surfaced on top.
// The fleet status drops below the fold — same data, reordered.
const specBlockedFirst = `{
  "schema": 1,
  "specId": "blocked-first",
  "title": "Blocked first — what needs you",
  "version": 1,
  "root": {
    "type": "col", "gap": "lg",
    "children": [
      { "type": "heading", "level": 1, "text": "Blocked first" },
      { "type": "section", "text": "👉 Needs you — decisions waiting", "gap": "md", "children": [
        { "type": "decision-list", "bind": "decisionsWaiting" }
      ]},
      { "type": "section", "text": "⚠️ Stuck or errored sessions", "gap": "md", "children": [
        { "type": "session-list", "bind": "stuckSessions" }
      ]},
      { "type": "section", "text": "The rest of the fleet", "gap": "md", "children": [
        { "type": "status-list", "bind": "conductors" }
      ]}
    ]
  }
}`

// --- 3. BY PROJECT --------------------------------------------------------
// Grouped per conductor/project: one card per conductor, each repeating its
// own sessions underneath. Demonstrates repeat over a list ref + per-item bind.
const specByProject = `{
  "schema": 1,
  "specId": "by-project",
  "title": "By project — grouped per conductor",
  "version": 1,
  "root": {
    "type": "col", "gap": "lg",
    "children": [
      { "type": "heading", "level": 1, "text": "By project" },
      { "type": "grid", "cols": 2, "gap": "md",
        "repeat": {
          "over": "conductors",
          "as": "item",
          "template": {
            "type": "section", "gap": "sm", "children": [
              { "type": "conductor-card", "bind": "item" },
              { "type": "session-list", "bind": "item.sessions" }
            ]
          }
        }
      }
    ]
  }
}`
