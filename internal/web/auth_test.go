package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// streamRecorder is an http.ResponseWriter+Flusher for testing SSE handlers.
// The handler writes its status + initial snapshot synchronously, then blocks
// in a select loop; this recorder lets the test read the status as soon as the
// first write lands without waiting for the (never-ending) stream to finish.
type streamRecorder struct {
	mu         sync.Mutex
	code       int
	wroteOnce  bool
	firstWrite chan struct{}
}

func newHeaderOnlyRecorder() *streamRecorder {
	return &streamRecorder{code: http.StatusOK, firstWrite: make(chan struct{})}
}

func (r *streamRecorder) Header() http.Header { return http.Header{} }

func (r *streamRecorder) WriteHeader(code int) {
	r.mu.Lock()
	r.code = code
	r.signalLocked()
	r.mu.Unlock()
}

func (r *streamRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	r.signalLocked()
	r.mu.Unlock()
	return len(p), nil
}

func (r *streamRecorder) Flush() {}

func (r *streamRecorder) signalLocked() {
	if !r.wroteOnce {
		r.wroteOnce = true
		close(r.firstWrite)
	}
}

func (r *streamRecorder) waitFirstWrite() {
	select {
	case <-r.firstWrite:
	case <-time.After(3 * time.Second):
	}
}

func (r *streamRecorder) status() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.code
}

// Regression for the headless+token Tailscale serve bug: the page served 200
// with ?token=, but the menu and command-center SSE feeds 401'd because the
// stream handlers used the header-only authorizer. The browser EventSource API
// (like the WS handshake) cannot set an Authorization header, so the client
// passes the token on the query string and the server must accept it there for
// streaming endpoints. Without this, no fleet snapshot ever arrives and the
// Command Center is stuck on "Waiting for the first fleet snapshot…".
//
// streamEndpoints are the endpoints opened by the browser via EventSource (or
// any push channel without explicit fetch options) — they must accept the
// query-string token. Regular JSON API endpoints stay header-only.
var streamEndpoints = []string{
	"/events/menu",
	"/events/command-center",
}

func newTokenStreamServer(token string) *Server {
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0", Token: token})
	srv.menuData = &fakeMenuDataLoader{snapshot: ccTestMenu()}
	return srv
}

// statusForStream issues a GET against path and returns the HTTP status. SSE
// handlers, once authorized, block streaming; so we send a request whose
// context is already cancelled to force the handler to return promptly after
// the auth gate (a 401 short-circuits before any streaming anyway).
func statusForStream(t *testing.T, srv *Server, path string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	// Recorder so an authorized stream that starts flushing doesn't hang the
	// test: we only assert on the status line, which is written before any
	// streaming loop. For the authorized case we read just the header via a
	// goroutine-free recorder and rely on the handler writing 200 + first
	// snapshot synchronously (it does: writeSSEEvent for the initial snapshot
	// happens before entering the select loop).
	rr := newHeaderOnlyRecorder()
	done := make(chan struct{})
	go func() {
		srv.Handler().ServeHTTP(rr, req)
		close(done)
	}()
	// The handler writes the status + initial SSE event synchronously, then
	// blocks in its select loop. Wait for the first write, then read status.
	rr.waitFirstWrite()
	return rr.status()
}

func TestStreamEndpointsAcceptQueryToken(t *testing.T) {
	const token = "secret-token"
	for _, ep := range streamEndpoints {
		t.Run(ep+"/query-token-ok", func(t *testing.T) {
			srv := newTokenStreamServer(token)
			if got := statusForStream(t, srv, ep+"?token="+token); got != http.StatusOK {
				t.Fatalf("%s with valid query token: expected 200, got %d", ep, got)
			}
		})

		t.Run(ep+"/bad-query-token-rejected", func(t *testing.T) {
			srv := newTokenStreamServer(token)
			req := httptest.NewRequest(http.MethodGet, ep+"?token=wrong", nil)
			rr := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("%s with bad query token: expected 401, got %d", ep, rr.Code)
			}
		})

		t.Run(ep+"/missing-token-rejected", func(t *testing.T) {
			srv := newTokenStreamServer(token)
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			rr := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("%s with no token: expected 401, got %d", ep, rr.Code)
			}
		})

		t.Run(ep+"/header-token-still-works", func(t *testing.T) {
			srv := newTokenStreamServer(token)
			rr := newHeaderOnlyRecorder()
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			done := make(chan struct{})
			go func() {
				srv.Handler().ServeHTTP(rr, req)
				close(done)
			}()
			rr.waitFirstWrite()
			if got := rr.status(); got != http.StatusOK {
				t.Fatalf("%s with valid bearer header: expected 200, got %d", ep, got)
			}
		})
	}
}

// TestJSONAPIRemainsHeaderOnly guards the security property that regular JSON
// API endpoints (called via fetch, which CAN set headers) still refuse a
// query-string token — keeping the report #5 mitigation intact for everything
// except the two browser-push channels that genuinely cannot use a header.
func TestJSONAPIRemainsHeaderOnly(t *testing.T) {
	const token = "secret-token"
	srv := newTokenStreamServer(token)

	req := httptest.NewRequest(http.MethodGet, "/api/command-center/status?token="+token, nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("JSON API with query token: expected 401 (header-only), got %d body=%s", rr.Code, rr.Body.String())
	}

	// Same endpoint with the bearer header is authorized.
	req2 := httptest.NewRequest(http.MethodGet, "/api/command-center/status", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("JSON API with bearer header: expected 200, got %d body=%s", rr2.Code, rr2.Body.String())
	}
	if strings.Contains(rr.Body.String(), `"profile"`) {
		t.Fatalf("unauthorized response leaked snapshot data: %s", rr.Body.String())
	}
}
