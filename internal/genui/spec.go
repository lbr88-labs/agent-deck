// Package genui is the trusted engine for the generative command center
// (v-genui-0). It defines the DATA-ONLY view-spec substrate and a strict
// validator. The spec is pure inert data: a recursive tree of layout
// primitives and registered leaf widgets, with data bound BY REFERENCE.
//
// The security thesis (load-bearing): the spec is DATA; a fixed, validated
// renderer interprets it; the browser NEVER runs spec-supplied code. This
// package owns the validation half of that thesis — it rejects any spec that
// carries code, an unknown widget/primitive, an unresolvable reference, or
// that exceeds the structural caps. No field in the schema can carry
// executable code, HTML, event-handler strings, or URLs-to-fetch.
package genui

// SchemaVersion is the version of the spec schema this engine understands.
// A spec must declare a matching version (fallback/requires targeting, per the
// Adaptive-Cards lesson in the research plan).
const SchemaVersion = 1

// Structural caps — the DoS / context-blowup / runaway-recursion guards.
const (
	MaxDepth     = 12   // max nesting depth of the Node tree
	MaxNodes     = 400  // max total nodes in a spec
	MaxChildren  = 64   // max direct children of any container node
	MaxStringLen = 4000 // max length of any literal text/string field
	MaxColumns   = 16   // max columns in a table widget
	MaxRepeatFan = 200  // max fan-out of a repeat over a list ref
)

// PrimitiveTypes is the CLOSED set of composable layout primitives. These give
// an OPEN layout space (kanban, tree, master-detail, dashboards) out of a
// CLOSED vocabulary via recursion — novel layouts are deeper trees of known
// nodes, not new types (the Notion lesson).
var PrimitiveTypes = map[string]bool{
	"col":     true, // vertical stack
	"row":     true, // horizontal stack
	"grid":    true, // wrapping grid of children
	"stack":   true, // alias for col (overlap-free vertical)
	"section": true, // titled region (carries an optional heading)
	"text":    true, // escaped plain text / heading leaf
	"heading": true, // a heading leaf (level 1-3)
}

// WidgetTypes is the CLOSED registry of rich leaf widgets. Each is backed by a
// real, audited renderer component. Leaves bind to data BY REFERENCE; they
// never carry literal data the model could hallucinate. New widgets are added
// by a DEVELOPER shipping a vetted component — never by the spec author.
var WidgetTypes = map[string]bool{
	"status-list":    true, // a fleet/session status list bound to a ref
	"session-list":   true, // a list of sessions bound to a ref
	"conductor-card": true, // a single conductor card bound to a ref
	"decision-list":  true, // decisions-waiting list bound to a ref
	"stat":           true, // a single labelled stat bound to a ref or scalar
}

// containerTypes is the subset of primitives that may carry children.
var containerTypes = map[string]bool{
	"col":     true,
	"row":     true,
	"grid":    true,
	"stack":   true,
	"section": true,
}

// Tone is a CLOSED enum of leaf tones (no free-form styling / no CSS).
var toneEnum = map[string]bool{
	"":        true,
	"neutral": true,
	"ok":      true,
	"warn":    true,
	"danger":  true,
	"info":    true,
}

// Spec is a whole-UI view spec — the top-level DATA document the renderer
// draws. It is pure JSON; every field below is data, never code.
type Spec struct {
	Schema  int    `json:"schema"`            // must equal SchemaVersion
	SpecID  string `json:"specId"`            // stable id for this view
	Title   string `json:"title"`             // plain text, escaped on render
	Version int    `json:"version,omitempty"` // monotonic per-spec version
	Root    *Node  `json:"root"`              // the recursive view tree
}

// Node is one element of the recursive spec tree. A node is EITHER a layout
// primitive (with children) OR a registered leaf widget (with a bound ref) —
// the validator enforces which fields are legal per type. There is no field
// that accepts code, HTML, a handler string, or a URL.
type Node struct {
	Type string `json:"type"` // primitive or widget type (closed enums)

	// Primitive fields.
	Children []*Node `json:"children,omitempty"` // container children
	Text     string  `json:"text,omitempty"`     // literal text (escaped on render)
	Level    int     `json:"level,omitempty"`    // heading level 1-3
	Gap      string  `json:"gap,omitempty"`       // closed spacing token: sm|md|lg
	Cols     int     `json:"cols,omitempty"`      // grid column count (1-6)

	// Widget fields.
	Label string `json:"label,omitempty"` // a leaf label (escaped)
	Tone  string `json:"tone,omitempty"`  // closed tone enum
	Bind  string `json:"bind,omitempty"`  // a DATA reference (resolved by renderer)

	// Binding / dynamic fields.
	When   string  `json:"when,omitempty"`   // a boolean-flag ref (pre-computed); render only if true
	Repeat *Repeat `json:"repeat,omitempty"` // render `template` once per item of a list ref
}

// Repeat renders a template node once per element of a list reference. The
// `over` ref is server/data-resolved; fan-out is capped. `as` names the
// per-item scope a child's `bind`/`when`/`text` may reference (e.g. "item").
type Repeat struct {
	Over     string `json:"over"`     // list ref, e.g. "conductors"
	As       string `json:"as"`       // per-item scope name, e.g. "item"
	Template *Node  `json:"template"` // the node rendered per item
}
