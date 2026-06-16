package genui

import (
	"strings"
	"testing"
)

// TestHandAuthoredSpecsValidate is the keystone-positive: the three shipped
// default views must all pass the same validator the renderer trusts.
func TestHandAuthoredSpecsValidate(t *testing.T) {
	if err := MustValidateAll(); err != nil {
		t.Fatalf("hand-authored specs must validate: %v", err)
	}
	for _, id := range ViewIDs {
		spec, err := LoadValidatedSpec(id)
		if err != nil {
			t.Fatalf("view %q: %v", id, err)
		}
		if spec.SpecID != id {
			t.Errorf("view %q: specId = %q", id, spec.SpecID)
		}
	}
}

func TestValidate_AcceptsValidSpec(t *testing.T) {
	raw := `{
		"schema": 1, "specId": "v", "title": "ok",
		"root": { "type": "col", "gap": "md", "children": [
			{ "type": "heading", "level": 1, "text": "hi" },
			{ "type": "stat", "label": "Running", "tone": "ok", "bind": "totals.running" },
			{ "type": "grid", "cols": 2, "repeat": {
				"over": "conductors", "as": "item",
				"template": { "type": "conductor-card", "bind": "item" }
			}}
		]}
	}`
	if _, err := ValidateBytes([]byte(raw)); err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
}

// TestValidate_RejectsCodeBearingField is THE security keystone: a spec that
// carries a code/HTML/handler field in ANY position must be rejected, not
// silently dropped.
func TestValidate_RejectsCodeBearingField(t *testing.T) {
	cases := map[string]string{
		"onClick handler": `{"schema":1,"specId":"x","root":{"type":"text","text":"hi","onClick":"alert(1)"}}`,
		"raw html field":   `{"schema":1,"specId":"x","root":{"type":"text","html":"<img src=x onerror=alert(1)>"}}`,
		"src url":          `{"schema":1,"specId":"x","root":{"type":"text","src":"https://evil/x.js"}}`,
		"script field":     `{"schema":1,"specId":"x","root":{"type":"text","script":"fetch('/steal')"}}`,
		"nested code":      `{"schema":1,"specId":"x","root":{"type":"col","children":[{"type":"text","code":"x()"}]}}`,
		"code in repeat":   `{"schema":1,"specId":"x","root":{"type":"col","repeat":{"over":"a","template":{"type":"text","eval":"1"}}}}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateBytes([]byte(raw)); err == nil {
				t.Fatalf("code-bearing spec accepted (must be rejected): %s", raw)
			}
		})
	}
}

func TestValidate_RejectsUnknownWidget(t *testing.T) {
	raw := `{"schema":1,"specId":"x","root":{"type":"col","children":[{"type":"iframe"}]}}`
	_, err := ValidateBytes([]byte(raw))
	if err == nil {
		t.Fatal("unknown widget type accepted")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("want unknown-type error, got: %v", err)
	}
}

func TestValidate_RejectsBadRef(t *testing.T) {
	cases := map[string]string{
		"space":      `{"schema":1,"specId":"x","root":{"type":"stat","bind":"totals running"}}`,
		"expression": `{"schema":1,"specId":"x","root":{"type":"stat","bind":"a + b"}}`,
		"bracket":    `{"schema":1,"specId":"x","root":{"type":"stat","bind":"items[0]"}}`,
		"quote":      `{"schema":1,"specId":"x","root":{"type":"stat","bind":"x'y"}}`,
		"bad when":   `{"schema":1,"specId":"x","root":{"type":"stat","when":"1==1"}}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateBytes([]byte(raw)); err == nil {
				t.Fatalf("bad ref accepted: %s", raw)
			}
		})
	}
}

func TestValidate_RejectsUnknownTopLevelField(t *testing.T) {
	raw := `{"schema":1,"specId":"x","root":{"type":"col"},"behavior":"onload"}`
	if _, err := ValidateBytes([]byte(raw)); err == nil {
		t.Fatal("unknown top-level field accepted")
	}
}

func TestValidate_RejectsBadSchemaVersion(t *testing.T) {
	raw := `{"schema":99,"specId":"x","root":{"type":"col"}}`
	if _, err := ValidateBytes([]byte(raw)); err == nil {
		t.Fatal("bad schema version accepted")
	}
}

func TestValidate_RejectsChildrenOnLeaf(t *testing.T) {
	raw := `{"schema":1,"specId":"x","root":{"type":"stat","children":[{"type":"text"}]}}`
	if _, err := ValidateBytes([]byte(raw)); err == nil {
		t.Fatal("children on a leaf widget accepted")
	}
}

func TestValidate_RejectsOverDepth(t *testing.T) {
	// Build a chain deeper than MaxDepth.
	inner := `{"type":"col"}`
	for i := 0; i < MaxDepth+2; i++ {
		inner = `{"type":"col","children":[` + inner + `]}`
	}
	raw := `{"schema":1,"specId":"x","root":` + inner + `}`
	_, err := ValidateBytes([]byte(raw))
	if err == nil {
		t.Fatal("over-depth spec accepted")
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Errorf("want depth error, got: %v", err)
	}
}

func TestValidate_RejectsOverNodeCount(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"schema":1,"specId":"x","root":{"type":"col","children":[`)
	for i := 0; i < MaxNodes+5; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"type":"text","text":"x"}`)
	}
	b.WriteString(`]}}`)
	if _, err := ValidateBytes([]byte(b.String())); err == nil {
		t.Fatal("over-node-count spec accepted (also exceeds max children)")
	}
}

func TestValidate_RejectsBadTone(t *testing.T) {
	raw := `{"schema":1,"specId":"x","root":{"type":"stat","tone":"explode"}}`
	if _, err := ValidateBytes([]byte(raw)); err == nil {
		t.Fatal("bad tone accepted")
	}
}

func TestValidate_RejectsInvalidJSON(t *testing.T) {
	if _, err := ValidateBytes([]byte(`{not json`)); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}
