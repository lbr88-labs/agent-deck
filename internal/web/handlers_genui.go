package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/asheshgoplani/agent-deck/internal/genui"
)

// The generative command center (v-genui-0) read endpoints. These serve the
// hand-authored, server-VALIDATED whole-UI view specs. The client renderer
// draws a spec; switching specs reshapes the whole UI live. NO LLM in v0 — the
// specs are inert DATA proving the safe substrate.
//
// Every spec served here passes genui.ValidateBytes server-side before it
// crosses to the client, so the browser only ever receives a vetted spec —
// the security control is on the server, not the browser.

// genuiViewMeta is the lightweight list entry (id + title) for the view switch.
type genuiViewMeta struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// handleGenuiViews lists the available hand-authored views.
func (s *Server) handleGenuiViews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	if !s.authorizeRequest(r) {
		writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	views := make([]genuiViewMeta, 0, len(genui.ViewIDs))
	for _, id := range genui.ViewIDs {
		views = append(views, genuiViewMeta{ID: id, Title: genui.ViewTitle[id]})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"views": views})
}

// handleGenuiSpec serves one validated whole-UI view spec by id. The spec is
// re-validated on every request — a default that ever drifted out of schema
// returns an error card rather than an unvalidated payload.
func (s *Server) handleGenuiSpec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	if !s.authorizeRequest(r) {
		writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/command-center/genui/spec/")
	id = strings.Trim(id, "/")
	raw, ok := genui.SpecJSON(id)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "unknown view")
		return
	}
	// Validate before serving — the browser must only ever see a vetted spec.
	if _, err := genui.ValidateBytes(raw); err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INVALID_SPEC", "spec failed validation")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}
