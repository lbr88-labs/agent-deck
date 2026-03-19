package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGroupsCollectionGET(t *testing.T) {
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
						ID:    "sess-1",
						Title: "alpha",
					},
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/groups", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"groups"`) {
		t.Errorf("expected 'groups' key in response, got: %s", body)
	}
	if !strings.Contains(body, `"work"`) {
		t.Errorf("expected group name in response, got: %s", body)
	}
}

func TestGroupsCollectionPOSTReturns501(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr:   "127.0.0.1:0",
		WebMutations: true,
	})
	srv.menuData = &fakeMenuDataLoader{snapshot: &MenuSnapshot{}}

	body := strings.NewReader(`{"name":"newgroup"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/groups", body)
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

func TestGroupRenamePATCHReturns501(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr:   "127.0.0.1:0",
		WebMutations: true,
	})
	srv.menuData = &fakeMenuDataLoader{snapshot: &MenuSnapshot{}}

	body := strings.NewReader(`{"name":"renamed"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/groups/mygroup", body)
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

func TestGroupDeleteReturns501(t *testing.T) {
	srv := NewServer(Config{
		ListenAddr:   "127.0.0.1:0",
		WebMutations: true,
	})
	srv.menuData = &fakeMenuDataLoader{snapshot: &MenuSnapshot{}}

	req := httptest.NewRequest(http.MethodDelete, "/api/groups/mygroup", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotImplemented, rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), ErrCodeNotImplemented) {
		t.Errorf("expected NOT_IMPLEMENTED error, got: %s", rr.Body.String())
	}
}
