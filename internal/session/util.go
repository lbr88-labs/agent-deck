package session

import (
	"os"
	"path/filepath"
	"strings"
)

// JoinWarnings concatenates non-empty, trimmed warning strings with "; ".
func JoinWarnings(parts ...string) string {
	joined := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		joined = append(joined, part)
	}
	return strings.Join(joined, "; ")
}

// ResolveQuickDefaultPath returns the validated absolute path from the
// quick_default_path config setting, or "" if unset / invalid.
func ResolveQuickDefaultPath() string {
	settings := GetInstanceSettings()
	path := strings.TrimSpace(settings.GetQuickDefaultPath())
	if path == "" {
		return ""
	}

	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return ""
		}
		path = absPath
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return ""
	}

	return filepath.Clean(path)
}
