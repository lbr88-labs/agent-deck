package web

import (
	"net/http"
	"runtime/debug"
)

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, ErrCodeMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorizeRequest(r) {
		writeAPIError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "unauthorized")
		return
	}

	writeJSON(w, http.StatusOK, SettingsResponse{
		Profile:      s.cfg.Profile,
		ReadOnly:     s.cfg.ReadOnly,
		WebMutations: s.cfg.WebMutations,
		Version:      buildVersion(),
	})
}

// buildVersion returns the binary version from embedded build info.
// Falls back to "dev" when build info is unavailable (e.g. during tests).
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}
