package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

func TestSessionsCollectionGET(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr: "127.0.0.1:0",
		Profile:    "test",
	})
	srv.menuData = &fakeMenuDataLoader{
		snapshot: &MenuSnapshot{
			Profile: "test",
			Items: []MenuItem{
				{
					Type: MenuItemTypeGroup,
					Group: &MenuGroup{
						Name: "work",
						Path: "work",
					},
				},
				{
					Type: MenuItemTypeSession,
					Session: &MenuSession{
						ID:     "sess-1",
						Title:  "alpha",
						Status: session.StatusRunning,
					},
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"sessions"`) {
		t.Errorf("expected 'sessions' key in response, got: %s", body)
	}
	if !strings.Contains(body, `"groups"`) {
		t.Errorf("expected 'groups' key in response, got: %s", body)
	}
	if !strings.Contains(body, `"sess-1"`) {
		t.Errorf("expected session id in response, got: %s", body)
	}
}

func TestSessionsCollectionPOSTReturns501(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr:   "127.0.0.1:0",
		WebMutations: true,
	})
	srv.menuData = &fakeMenuDataLoader{snapshot: &MenuSnapshot{}}

	body := strings.NewReader(`{"title":"Test","tool":"claude","projectPath":"/tmp"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotImplemented, rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), ErrCodeNotImplemented) {
		t.Errorf("expected NOT_IMPLEMENTED error, got: %s", rr.Body.String())
	}
}

func TestSessionsCollectionPOSTMutationsDisabled(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr:   "127.0.0.1:0",
		WebMutations: false,
	})
	srv.menuData = &fakeMenuDataLoader{snapshot: &MenuSnapshot{}}

	body := strings.NewReader(`{"title":"Test","tool":"claude","projectPath":"/tmp"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), ErrCodeForbidden) {
		t.Errorf("expected MUTATIONS_DISABLED error, got: %s", rr.Body.String())
	}
}

func TestSessionStopReturns501(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr:   "127.0.0.1:0",
		WebMutations: true,
	})
	srv.menuData = &fakeMenuDataLoader{snapshot: &MenuSnapshot{}}

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/test-id/stop", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotImplemented, rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), ErrCodeNotImplemented) {
		t.Errorf("expected NOT_IMPLEMENTED error, got: %s", rr.Body.String())
	}
}

func TestSessionDeleteReturns501(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr:   "127.0.0.1:0",
		WebMutations: true,
	})
	srv.menuData = &fakeMenuDataLoader{snapshot: &MenuSnapshot{}}

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/test-id", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotImplemented, rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), ErrCodeNotImplemented) {
		t.Errorf("expected NOT_IMPLEMENTED error, got: %s", rr.Body.String())
	}
}

func TestSessionsUnauthorized(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr: "127.0.0.1:0",
		Token:      "secret-token",
	})
	srv.menuData = &fakeMenuDataLoader{snapshot: &MenuSnapshot{}}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), ErrCodeUnauthorized) {
		t.Errorf("expected UNAUTHORIZED error, got: %s", rr.Body.String())
	}
}

func TestSessionStartReturns501(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr:   "127.0.0.1:0",
		WebMutations: true,
	})
	srv.menuData = &fakeMenuDataLoader{snapshot: &MenuSnapshot{}}

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/test-id/start", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotImplemented, rr.Code, rr.Body.String())
	}
}

func TestSessionRestartReturns501(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr:   "127.0.0.1:0",
		WebMutations: true,
	})
	srv.menuData = &fakeMenuDataLoader{snapshot: &MenuSnapshot{}}

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/test-id/restart", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotImplemented, rr.Code, rr.Body.String())
	}
}

func TestSessionForkReturns501(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr:   "127.0.0.1:0",
		WebMutations: true,
	})
	srv.menuData = &fakeMenuDataLoader{snapshot: &MenuSnapshot{}}

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/test-id/fork", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotImplemented, rr.Code, rr.Body.String())
	}
}
