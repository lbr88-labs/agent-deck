package ui

import "testing"

func TestResolveHotkeysOverridesAndUnbinds(t *testing.T) {
	bindings := resolveHotkeys(map[string]string{
		"delete":        "backspace",
		"close_session": "",
		"unknown":       "x",
	})

	if got := bindings[hotkeyDelete]; got != "backspace" {
		t.Fatalf("delete binding = %q, want backspace", got)
	}

	if _, ok := bindings[hotkeyCloseSession]; ok {
		t.Fatalf("close_session should be unbound")
	}

	if got := bindings[hotkeyRestart]; got != defaultHotkeyBindings[hotkeyRestart] {
		t.Fatalf("restart binding = %q, want %q", got, defaultHotkeyBindings[hotkeyRestart])
	}
}

func TestResolveHotkeysPrefersCanonicalNameOverLegacyRename(t *testing.T) {
	bindings := resolveHotkeys(map[string]string{
		"toggle_gemini_yolo": "g",
		"toggle_yolo":        "y",
	})

	if got := bindings[hotkeyToggleYolo]; got != "y" {
		t.Fatalf("toggle_yolo binding = %q, want %q", got, "y")
	}
}

func TestResolveHotkeysMapsLegacyRenameWhenCanonicalAbsent(t *testing.T) {
	bindings := resolveHotkeys(map[string]string{
		"toggle_gemini_yolo": "g",
	})

	if got := bindings[hotkeyToggleYolo]; got != "g" {
		t.Fatalf("toggle_yolo binding = %q, want %q", got, "g")
	}
}

func TestBuildHotkeyLookupRemapAndUnbind(t *testing.T) {
	bindings := resolveHotkeys(map[string]string{
		"delete": "backspace",
		"quit":   "",
	})
	lookup, blocked := buildHotkeyLookup(bindings)

	if got := lookup["backspace"]; got != defaultHotkeyBindings[hotkeyDelete] {
		t.Fatalf("backspace maps to %q, want %q", got, defaultHotkeyBindings[hotkeyDelete])
	}

	if !blocked[defaultHotkeyBindings[hotkeyDelete]] {
		t.Fatalf("default delete key should be blocked when remapped")
	}

	if !blocked["q"] {
		t.Fatalf("q should be blocked when quit is unbound")
	}

	if !blocked["ctrl+c"] {
		t.Fatalf("ctrl+c should be blocked when quit is unbound")
	}
}

func TestHotkeyAliasesShiftAndSymbols(t *testing.T) {
	aliases := hotkeyAliases("shift+f")
	hasUpper := false
	for _, alias := range aliases {
		if alias == "F" {
			hasUpper = true
			break
		}
	}
	if !hasUpper {
		t.Fatalf("shift+f aliases should include F")
	}

	symbolAliases := hotkeyAliases("!")
	hasShiftNum := false
	for _, alias := range symbolAliases {
		if alias == "shift+1" {
			hasShiftNum = true
			break
		}
	}
	if !hasShiftNum {
		t.Fatalf("! aliases should include shift+1")
	}
}

func TestNormalizeMainKeyWithConfiguredHotkeys(t *testing.T) {
	h := NewHome()
	h.setHotkeys(resolveHotkeys(map[string]string{
		"delete": "backspace",
		"quit":   "",
	}))

	if got := h.normalizeMainKey("backspace"); got != defaultHotkeyBindings[hotkeyDelete] {
		t.Fatalf("backspace normalized to %q, want %q", got, defaultHotkeyBindings[hotkeyDelete])
	}

	if got := h.normalizeMainKey(defaultHotkeyBindings[hotkeyDelete]); got != "" {
		t.Fatalf("default delete key should be blocked after remap, got %q", got)
	}

	if got := h.normalizeMainKey("ctrl+c"); got != "" {
		t.Fatalf("ctrl+c should be blocked when quit is unbound, got %q", got)
	}
}
