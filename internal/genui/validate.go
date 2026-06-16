package genui

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidationError is a structured rejection. The renderer turns it into an
// inert error card; nothing partially-valid is ever rendered.
type ValidationError struct {
	Path   string `json:"path"`   // where in the spec the problem is
	Reason string `json:"reason"` // why it was rejected
}

func (e *ValidationError) Error() string {
	if e.Path == "" {
		return e.Reason
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Reason)
}

// knownNodeFields is the CLOSED set of permitted JSON keys on a Node. ANY other
// key (e.g. an injected raw-HTML field, an "onClick", an "src", a "code", or a
// "script" key) is rejected — this is what makes a code-bearing spec fail.
var knownNodeFields = map[string]bool{
	"type": true, "children": true, "text": true, "level": true,
	"gap": true, "cols": true, "label": true, "tone": true,
	"bind": true, "when": true, "repeat": true,
}

var knownSpecFields = map[string]bool{
	"schema": true, "specId": true, "title": true, "version": true, "root": true,
}

var knownRepeatFields = map[string]bool{
	"over": true, "as": true, "template": true,
}

var gapEnum = map[string]bool{"": true, "sm": true, "md": true, "lg": true}

// ValidateBytes is the entry point: it parses raw spec JSON and validates it.
// It validates at the RAW JSON level first (to catch unknown/code-bearing
// fields that a typed unmarshal would silently drop), then validates the typed
// structure (enums, refs, caps, depth). Returns the parsed Spec only if valid.
func ValidateBytes(raw []byte) (*Spec, error) {
	// 1. Raw-level: reject unknown fields anywhere in the tree. A typed
	//    json.Unmarshal silently ignores unknown keys, so an injected
	//    raw-HTML / onClick / src field would vanish instead of being
	//    rejected. We must REJECT it loudly — that is the security keystone.
	var rawAny interface{}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&rawAny); err != nil {
		return nil, &ValidationError{Path: "", Reason: "invalid JSON: " + err.Error()}
	}
	if err := checkRawUnknownFields(rawAny, "$"); err != nil {
		return nil, err
	}

	// 2. Typed parse + structural validation.
	var spec Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, &ValidationError{Path: "", Reason: "decode: " + err.Error()}
	}
	if err := Validate(&spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// checkRawUnknownFields walks the parsed-but-untyped JSON and rejects any object
// key that is not in the closed allow-list for its position. This is the layer
// that catches a code-bearing field regardless of name, because ANYTHING
// outside the closed vocabulary is rejected.
func checkRawUnknownFields(v interface{}, path string) error {
	switch t := v.(type) {
	case map[string]interface{}:
		// Decide which allow-list applies by shape. The top-level object (has
		// "root" or "schema") uses the spec allow-list; a "repeat" sub-object
		// uses the repeat list; everything else is a node.
		allow := knownNodeFields
		if path == "$" {
			allow = knownSpecFields
		} else if strings.HasSuffix(path, ".repeat") {
			allow = knownRepeatFields
		}
		for k, child := range t {
			if !allow[k] {
				return &ValidationError{Path: path, Reason: "unknown field " + jsonKey(k)}
			}
			if err := checkRawUnknownFields(child, path+"."+k); err != nil {
				return err
			}
		}
	case []interface{}:
		for i, child := range t {
			if err := checkRawUnknownFields(child, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func jsonKey(k string) string { return "\"" + k + "\"" }

// Validate checks a typed Spec against the schema: version, closed type enums,
// per-type field legality, reference syntax, tone/gap enums, and the structural
// caps (depth, node count, children, repeat fan-out, string length).
func Validate(spec *Spec) error {
	if spec == nil {
		return &ValidationError{Reason: "nil spec"}
	}
	if spec.Schema != SchemaVersion {
		return &ValidationError{Path: "$.schema", Reason: fmt.Sprintf("unsupported schema %d (want %d)", spec.Schema, SchemaVersion)}
	}
	if strings.TrimSpace(spec.SpecID) == "" {
		return &ValidationError{Path: "$.specId", Reason: "specId is required"}
	}
	if len(spec.Title) > MaxStringLen {
		return &ValidationError{Path: "$.title", Reason: "title exceeds max length"}
	}
	if spec.Root == nil {
		return &ValidationError{Path: "$.root", Reason: "root node is required"}
	}
	count := 0
	return validateNode(spec.Root, "$.root", 1, &count)
}

func validateNode(n *Node, path string, depth int, count *int) error {
	if n == nil {
		return &ValidationError{Path: path, Reason: "nil node"}
	}
	if depth > MaxDepth {
		return &ValidationError{Path: path, Reason: fmt.Sprintf("nesting exceeds max depth %d", MaxDepth)}
	}
	*count++
	if *count > MaxNodes {
		return &ValidationError{Path: path, Reason: fmt.Sprintf("spec exceeds max node count %d", MaxNodes)}
	}

	isPrimitive := PrimitiveTypes[n.Type]
	isWidget := WidgetTypes[n.Type]
	if !isPrimitive && !isWidget {
		return &ValidationError{Path: path + ".type", Reason: "unknown type " + jsonKey(n.Type)}
	}

	// Enums.
	if !toneEnum[n.Tone] {
		return &ValidationError{Path: path + ".tone", Reason: "unknown tone " + jsonKey(n.Tone)}
	}
	if !gapEnum[n.Gap] {
		return &ValidationError{Path: path + ".gap", Reason: "unknown gap " + jsonKey(n.Gap)}
	}
	if len(n.Text) > MaxStringLen {
		return &ValidationError{Path: path + ".text", Reason: "text exceeds max length"}
	}
	if len(n.Label) > MaxStringLen {
		return &ValidationError{Path: path + ".label", Reason: "label exceeds max length"}
	}
	if n.Level < 0 || n.Level > 3 {
		return &ValidationError{Path: path + ".level", Reason: "level must be 0-3"}
	}
	if n.Cols < 0 || n.Cols > 6 {
		return &ValidationError{Path: path + ".cols", Reason: "cols must be 0-6"}
	}

	// References must be syntactically safe identifiers (the renderer resolves
	// them against the secret-free bound data). Reject anything that isn't a
	// dotted identifier — closes off injection through a ref string.
	if n.Bind != "" && !isSafeRef(n.Bind) {
		return &ValidationError{Path: path + ".bind", Reason: "invalid ref syntax " + jsonKey(n.Bind)}
	}
	if n.When != "" && !isSafeRef(n.When) {
		return &ValidationError{Path: path + ".when", Reason: "invalid ref syntax " + jsonKey(n.When)}
	}

	// Children only on container primitives.
	if len(n.Children) > 0 && !containerTypes[n.Type] {
		return &ValidationError{Path: path + ".children", Reason: jsonKey(n.Type) + " cannot have children"}
	}
	if len(n.Children) > MaxChildren {
		return &ValidationError{Path: path + ".children", Reason: fmt.Sprintf("exceeds max children %d", MaxChildren)}
	}
	for i, c := range n.Children {
		if err := validateNode(c, fmt.Sprintf("%s.children[%d]", path, i), depth+1, count); err != nil {
			return err
		}
	}

	// Repeat: over must be a safe ref; template recurses.
	if n.Repeat != nil {
		rp := path + ".repeat"
		if !isSafeRef(n.Repeat.Over) {
			return &ValidationError{Path: rp + ".over", Reason: "invalid over ref " + jsonKey(n.Repeat.Over)}
		}
		if n.Repeat.As != "" && !isSafeIdent(n.Repeat.As) {
			return &ValidationError{Path: rp + ".as", Reason: "invalid as name " + jsonKey(n.Repeat.As)}
		}
		if n.Repeat.Template == nil {
			return &ValidationError{Path: rp + ".template", Reason: "repeat requires a template"}
		}
		if err := validateNode(n.Repeat.Template, rp+".template", depth+1, count); err != nil {
			return err
		}
	}
	return nil
}

// isSafeRef accepts dotted identifiers like "conductors", "item.status",
// "totals.running". It rejects anything with spaces, quotes, brackets,
// operators, or other characters that could carry an expression/injection.
func isSafeRef(s string) bool {
	if s == "" || len(s) > 200 {
		return false
	}
	for _, part := range strings.Split(s, ".") {
		if !isSafeIdent(part) {
			return false
		}
	}
	return true
}

func isSafeIdent(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for i, r := range s {
		ok := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if i > 0 {
			ok = ok || (r >= '0' && r <= '9') || r == '-'
		}
		if !ok {
			return false
		}
	}
	return true
}
