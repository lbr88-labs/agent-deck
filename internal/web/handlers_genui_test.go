package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/genui"
)

func TestGenuiViews_ListsHandAuthoredViews(t *testing.T) {
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"})
	req := httptest.NewRequest(http.MethodGet, "/api/command-center/genui/views", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("views: status %d", rr.Code)
	}
	var body struct {
		Views []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"views"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Views) != len(genui.ViewIDs) {
		t.Fatalf("expected %d views, got %d", len(genui.ViewIDs), len(body.Views))
	}
}

func TestGenuiSpec_ServesValidatedSpec(t *testing.T) {
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"})
	for _, id := range genui.ViewIDs {
		req := httptest.NewRequest(http.MethodGet, "/api/command-center/genui/spec/"+id, nil)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("spec %q: status %d", id, rr.Code)
		}
		// Re-validate the served bytes — the wire payload must be a valid spec.
		spec, err := genui.ValidateBytes(rr.Body.Bytes())
		if err != nil {
			t.Fatalf("served spec %q failed validation: %v", id, err)
		}
		if spec.SpecID != id {
			t.Errorf("spec %q: specId = %q", id, spec.SpecID)
		}
	}
}

func TestGenuiSpec_UnknownViewIs404(t *testing.T) {
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"})
	req := httptest.NewRequest(http.MethodGet, "/api/command-center/genui/spec/does-not-exist", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown view: status %d (want 404)", rr.Code)
	}
}

func TestGenuiSpec_RequiresAuthWhenTokenSet(t *testing.T) {
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0", Token: "secret-token"})
	// No auth header -> unauthorized (non-loopback gate via token).
	req := httptest.NewRequest(http.MethodGet, "/api/command-center/genui/spec/status-board", nil)
	req.RemoteAddr = "10.0.0.5:1234" // non-loopback
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token from non-loopback, got %d", rr.Code)
	}
	// With the token -> ok.
	req2 := httptest.NewRequest(http.MethodGet, "/api/command-center/genui/spec/status-board", nil)
	req2.RemoteAddr = "10.0.0.5:1234"
	req2.Header.Set("Authorization", "Bearer secret-token")
	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d", rr2.Code)
	}
}
