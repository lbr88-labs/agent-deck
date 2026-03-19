package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

type sessionsListResponse struct {
	Sessions []*MenuSession `json:"sessions"`
	Groups   []*MenuGroup   `json:"groups"`
	Profile  string         `json:"profile"`
}

func (s *Server) handleSessionsCollection(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeRequest(r) {
		writeAPIError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "unauthorized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		snapshot, err := s.menuData.LoadMenuSnapshot()
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, ErrCodeInternalError, "failed to load session data")
			return
		}
		resp := sessionsListResponse{
			Sessions: make([]*MenuSession, 0),
			Groups:   make([]*MenuGroup, 0),
			Profile:  snapshot.Profile,
		}
		for _, item := range snapshot.Items {
			if item.Type == MenuItemTypeSession && item.Session != nil {
				resp.Sessions = append(resp.Sessions, item.Session)
			} else if item.Type == MenuItemTypeGroup && item.Group != nil {
				resp.Groups = append(resp.Groups, item.Group)
			}
		}
		writeJSON(w, http.StatusOK, resp)

	case http.MethodPost:
		if !s.checkMutationsAllowed(w) {
			return
		}
		if !s.checkMutationRateLimit(w) {
			return
		}
		var req CreateSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
			return
		}
		writeAPIError(w, http.StatusNotImplemented, ErrCodeNotImplemented, "session creation not yet implemented")

	default:
		writeAPIError(w, http.StatusMethodNotAllowed, ErrCodeMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleSessionByAction(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeRequest(r) {
		writeAPIError(w, http.StatusUnauthorized, ErrCodeUnauthorized, "unauthorized")
		return
	}

	// Path: /api/sessions/{id} or /api/sessions/{id}/{action}
	const prefix = "/api/sessions/"
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	sessionID := parts[0]
	if sessionID == "" {
		writeAPIError(w, http.StatusBadRequest, ErrCodeBadRequest, "session id is required")
		return
	}

	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}

	// DELETE /api/sessions/{id}
	if r.Method == http.MethodDelete && action == "" {
		if !s.checkMutationsAllowed(w) {
			return
		}
		if !s.checkMutationRateLimit(w) {
			return
		}
		writeAPIError(w, http.StatusNotImplemented, ErrCodeNotImplemented, "session deletion not yet implemented")
		return
	}

	// POST /api/sessions/{id}/{action}
	if r.Method == http.MethodPost {
		if !s.checkMutationsAllowed(w) {
			return
		}
		if !s.checkMutationRateLimit(w) {
			return
		}
		switch action {
		case "stop":
			writeAPIError(w, http.StatusNotImplemented, ErrCodeNotImplemented, "session stop not yet implemented")
		case "start":
			writeAPIError(w, http.StatusNotImplemented, ErrCodeNotImplemented, "session start not yet implemented")
		case "restart":
			writeAPIError(w, http.StatusNotImplemented, ErrCodeNotImplemented, "session restart not yet implemented")
		case "fork":
			writeAPIError(w, http.StatusNotImplemented, ErrCodeNotImplemented, "session fork not yet implemented")
		default:
			writeAPIError(w, http.StatusNotFound, ErrCodeNotFound, "unknown session action")
		}
		return
	}

	writeAPIError(w, http.StatusNotFound, ErrCodeNotFound, "route not found")
}
