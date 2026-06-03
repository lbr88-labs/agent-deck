// Package credrefresh tests — the subscription-safe keep-warm OAuth refresh
// daemon (Part B of the OAuth /login outage fix).
//
// These tests NEVER hit the real Anthropic endpoint and NEVER use a real
// refresh token — every case injects an httptest mock and synthetic tokens.
// Burning the real single-use rotating refresh token in a test would log the
// host out, which is the exact outage we are preventing.
package credrefresh

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// callRecorder collects token-endpoint requests under a mutex. The httptest
// server invokes its handler in a separate goroutine, so the slice append must
// be synchronized against the test goroutine's reads (else -race flags it).
type callRecorder struct {
	mu    sync.Mutex
	calls []map[string]string
}

func (r *callRecorder) add(c map[string]string) {
	r.mu.Lock()
	r.calls = append(r.calls, c)
	r.mu.Unlock()
}

func (r *callRecorder) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *callRecorder) at(i int) map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[i]
}

// writeCreds writes a credentials document with a claudeAiOauth block plus an
// arbitrary extra top-level key, to prove unknown fields survive a round-trip.
func writeCreds(t *testing.T, path string, oauth map[string]any, extraTopLevel map[string]any) {
	t.Helper()
	doc := map[string]any{"claudeAiOauth": oauth}
	for k, v := range extraTopLevel {
		doc[k] = v
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal creds: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write creds: %v", err)
	}
}

// readOAuth reads the claudeAiOauth block back out of a credentials file.
func readOAuth(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read creds: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal creds: %v", err)
	}
	oauth, ok := doc["claudeAiOauth"].(map[string]any)
	if !ok {
		t.Fatalf("claudeAiOauth missing or wrong shape in %s", path)
	}
	return oauth
}

// mockTokenServer returns an httptest server that rotates the token, recording
// every request it received so tests can assert the request shape (and that it
// was or was not called).
//
// It mirrors Anthropic's real token endpoint: the body MUST be JSON. A
// form-urlencoded body (the shipped-but-broken format) is rejected with a 400
// "Invalid request format", so a regression to form encoding fails loudly
// instead of silently succeeding (the original ParseForm mock accepted both).
func mockTokenServer(t *testing.T, newAccess, newRefresh string, expiresIn int) (*httptest.Server, *callRecorder) {
	t.Helper()
	rec := &callRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":             "invalid_request",
				"error_description": "Invalid request format",
			})
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		rec.add(map[string]string{
			"grant_type":    body["grant_type"],
			"refresh_token": body["refresh_token"],
			"client_id":     body["client_id"],
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  newAccess,
			"refresh_token": newRefresh,
			"expires_in":    expiresIn,
		})
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func fixedNow() func() time.Time {
	t := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

// (1) Near expiry: the daemon POSTs the refresh token and atomically rewrites
// canonical with the rotated tokens and a recomputed expiresAt.
func TestRefreshIfNeeded_RefreshesWhenNearExpiry(t *testing.T) {
	now := fixedNow()
	credPath := filepath.Join(t.TempDir(), ".credentials.json")
	writeCreds(t, credPath, map[string]any{
		"accessToken":  "old-access",
		"refreshToken": "old-refresh",
		"clientId":     "client-xyz",
		"scopes":       []any{"user:inference", "user:profile"},
		"expiresAt":    float64(now().Add(5 * time.Minute).UnixMilli()), // within window
	}, nil)

	srv, calls := mockTokenServer(t, "new-access", "new-refresh", 3600)

	res, err := RefreshIfNeeded(credPath, RefreshConfig{
		TokenEndpoint: srv.URL,
		HTTPClient:    srv.Client(),
		Threshold:     20 * time.Minute,
		Now:           now,
	})
	if err != nil {
		t.Fatalf("RefreshIfNeeded: %v", err)
	}
	if !res.Refreshed {
		t.Fatal("expected Refreshed=true when within threshold")
	}

	// Endpoint was called exactly once with the correct grant + rotating token.
	if calls.len() != 1 {
		t.Fatalf("expected exactly 1 token request; got %d", calls.len())
	}
	c := calls.at(0)
	if c["grant_type"] != "refresh_token" {
		t.Fatalf("grant_type = %q; want refresh_token", c["grant_type"])
	}
	if c["refresh_token"] != "old-refresh" {
		t.Fatalf("refresh_token = %q; want old-refresh", c["refresh_token"])
	}
	if c["client_id"] != "client-xyz" {
		t.Fatalf("client_id = %q; want client-xyz", c["client_id"])
	}

	// Canonical now holds the rotated tokens + recomputed expiry.
	oauth := readOAuth(t, credPath)
	if oauth["accessToken"] != "new-access" {
		t.Fatalf("accessToken = %v; want new-access", oauth["accessToken"])
	}
	if oauth["refreshToken"] != "new-refresh" {
		t.Fatalf("refreshToken = %v; want new-refresh", oauth["refreshToken"])
	}
	wantExpiry := float64(now().Add(3600 * time.Second).UnixMilli())
	if oauth["expiresAt"] != wantExpiry {
		t.Fatalf("expiresAt = %v; want %v", oauth["expiresAt"], wantExpiry)
	}
}

// (1b) Contract regression: requestRefresh MUST send a JSON body with
// Content-Type: application/json and the three required fields. Anthropic's
// token endpoint rejects a form-urlencoded body with 400 "Invalid request
// format" (the shipped-but-inert v1.9.46 bug). This test captures the raw
// outgoing request and asserts the wire format directly, so a regression to
// form encoding fails here regardless of how a mock parses the body.
func TestRequestRefresh_SendsJSONBody(t *testing.T) {
	var (
		gotContentType string
		gotRawBody     []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotRawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "a",
			"refresh_token": "r",
			"expires_in":    3600,
		})
	}))
	t.Cleanup(srv.Close)

	tok, err := requestRefresh(
		RefreshConfig{TokenEndpoint: srv.URL, HTTPClient: srv.Client()},
		"my-refresh-token", "client-abc", nil,
	)
	if err != nil {
		t.Fatalf("requestRefresh: %v", err)
	}
	if tok.AccessToken != "a" || tok.RefreshToken != "r" {
		t.Fatalf("unexpected token response: %+v", tok)
	}

	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q; want application/json", gotContentType)
	}

	// Body must be parseable JSON (a form-urlencoded body would not be) with
	// all three required fields populated.
	var body map[string]string
	if err := json.Unmarshal(gotRawBody, &body); err != nil {
		t.Fatalf("request body is not JSON (%q): %v", string(gotRawBody), err)
	}
	if body["grant_type"] != "refresh_token" {
		t.Fatalf("grant_type = %q; want refresh_token", body["grant_type"])
	}
	if body["refresh_token"] != "my-refresh-token" {
		t.Fatalf("refresh_token = %q; want my-refresh-token", body["refresh_token"])
	}
	if body["client_id"] != "client-abc" {
		t.Fatalf("client_id = %q; want client-abc", body["client_id"])
	}
}

// (1c) When the stored credentials omit clientId, requestRefresh falls back to
// the well-known Claude Code public client_id — the endpoint requires one.
func TestRequestRefresh_FallsBackToDefaultClientID(t *testing.T) {
	var gotClientID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotClientID = body["client_id"]
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "a", "refresh_token": "r", "expires_in": 3600,
		})
	}))
	t.Cleanup(srv.Close)

	if _, err := requestRefresh(
		RefreshConfig{TokenEndpoint: srv.URL, HTTPClient: srv.Client()},
		"rt", "", nil,
	); err != nil {
		t.Fatalf("requestRefresh: %v", err)
	}
	if gotClientID != ClaudeCodeClientID {
		t.Fatalf("client_id = %q; want fallback %q", gotClientID, ClaudeCodeClientID)
	}
}

// (2) Far from expiry: no network call, canonical byte-for-byte unchanged.
func TestRefreshIfNeeded_SkipsWhenFarFromExpiry(t *testing.T) {
	now := fixedNow()
	credPath := filepath.Join(t.TempDir(), ".credentials.json")
	writeCreds(t, credPath, map[string]any{
		"accessToken":  "still-good",
		"refreshToken": "rt",
		"clientId":     "client-xyz",
		"expiresAt":    float64(now().Add(50 * time.Minute).UnixMilli()), // outside window
	}, nil)

	before, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	srv, calls := mockTokenServer(t, "should-not-be-used", "nope", 3600)

	res, err := RefreshIfNeeded(credPath, RefreshConfig{
		TokenEndpoint: srv.URL,
		HTTPClient:    srv.Client(),
		Threshold:     20 * time.Minute,
		Now:           now,
	})
	if err != nil {
		t.Fatalf("RefreshIfNeeded: %v", err)
	}
	if res.Refreshed {
		t.Fatal("expected Refreshed=false when far from expiry")
	}
	if calls.len() != 0 {
		t.Fatalf("token endpoint must NOT be called when far from expiry; got %d calls", calls.len())
	}
	after, err := os.ReadFile(credPath)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("canonical must be untouched when no refresh is due")
	}
}

// (3) Unknown fields (subscriptionType, rateLimitTier, extra top-level keys)
// survive the rotation round-trip.
func TestRefreshIfNeeded_PreservesUnknownFields(t *testing.T) {
	now := fixedNow()
	credPath := filepath.Join(t.TempDir(), ".credentials.json")
	writeCreds(t, credPath, map[string]any{
		"accessToken":      "old-access",
		"refreshToken":     "old-refresh",
		"clientId":         "client-xyz",
		"expiresAt":        float64(now().Add(time.Minute).UnixMilli()),
		"subscriptionType": "pro",
		"rateLimitTier":    "default",
	}, map[string]any{"someOtherKey": "keep-me"})

	srv, _ := mockTokenServer(t, "new-access", "new-refresh", 3600)
	if _, err := RefreshIfNeeded(credPath, RefreshConfig{
		TokenEndpoint: srv.URL, HTTPClient: srv.Client(), Threshold: 20 * time.Minute, Now: now,
	}); err != nil {
		t.Fatalf("RefreshIfNeeded: %v", err)
	}

	data, _ := os.ReadFile(credPath)
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc["someOtherKey"] != "keep-me" {
		t.Fatalf("top-level unknown field dropped: %v", doc["someOtherKey"])
	}
	oauth := doc["claudeAiOauth"].(map[string]any)
	if oauth["subscriptionType"] != "pro" {
		t.Fatalf("subscriptionType dropped: %v", oauth["subscriptionType"])
	}
	if oauth["rateLimitTier"] != "default" {
		t.Fatalf("rateLimitTier dropped: %v", oauth["rateLimitTier"])
	}
}

// (4) The rewritten canonical keeps 0600 token perms.
func TestRefreshIfNeeded_PreservesSecurePerms(t *testing.T) {
	now := fixedNow()
	credPath := filepath.Join(t.TempDir(), ".credentials.json")
	writeCreds(t, credPath, map[string]any{
		"accessToken": "old", "refreshToken": "rt", "clientId": "c",
		"expiresAt": float64(now().Add(time.Minute).UnixMilli()),
	}, nil)
	srv, _ := mockTokenServer(t, "new", "new-rt", 3600)
	if _, err := RefreshIfNeeded(credPath, RefreshConfig{
		TokenEndpoint: srv.URL, HTTPClient: srv.Client(), Threshold: 20 * time.Minute, Now: now,
	}); err != nil {
		t.Fatalf("RefreshIfNeeded: %v", err)
	}
	fi, err := os.Stat(credPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("refreshed canonical must be 0600; got %o", perm)
	}
}

// (5) A failing endpoint must leave canonical untouched — never half-write or
// corrupt the only token on a transient 4xx/5xx.
func TestRefreshIfNeeded_EndpointErrorLeavesCanonicalUntouched(t *testing.T) {
	now := fixedNow()
	credPath := filepath.Join(t.TempDir(), ".credentials.json")
	writeCreds(t, credPath, map[string]any{
		"accessToken": "good", "refreshToken": "rt", "clientId": "c",
		"expiresAt": float64(now().Add(time.Minute).UnixMilli()),
	}, nil)
	before, _ := os.ReadFile(credPath)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	res, err := RefreshIfNeeded(credPath, RefreshConfig{
		TokenEndpoint: srv.URL, HTTPClient: srv.Client(), Threshold: 20 * time.Minute, Now: now,
	})
	if err == nil {
		t.Fatal("expected an error on a 400 from the token endpoint")
	}
	if res.Refreshed {
		t.Fatal("Refreshed must be false on endpoint error")
	}
	after, _ := os.ReadFile(credPath)
	if string(before) != string(after) {
		t.Fatalf("canonical must be untouched on endpoint error")
	}
}

// (6) The proper-lockfile-compatible lock is released after a successful
// refresh — no leaked lock dir at Claude's lock path (the CONFIG_DIR sibling).
func TestRefreshIfNeeded_ReleasesLock(t *testing.T) {
	now := fixedNow()
	configDir := t.TempDir()
	credPath := filepath.Join(configDir, ".credentials.json")
	writeCreds(t, credPath, map[string]any{
		"accessToken": "old", "refreshToken": "rt", "clientId": "c",
		"expiresAt": float64(now().Add(time.Minute).UnixMilli()),
	}, nil)
	srv, _ := mockTokenServer(t, "new", "new-rt", 3600)
	if _, err := RefreshIfNeeded(credPath, RefreshConfig{
		TokenEndpoint: srv.URL, HTTPClient: srv.Client(), Threshold: 20 * time.Minute, Now: now,
	}); err != nil {
		t.Fatalf("RefreshIfNeeded: %v", err)
	}
	lockPath, err := claudeLockPath(credPath)
	if err != nil {
		t.Fatalf("claudeLockPath: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock dir %s must be released after refresh; stat err = %v", lockPath, err)
	}
}

// (8) BLOCKER regression: the daemon must lock the SAME path Claude locks on its
// OAuth-refresh path. Verified in the shipped binary: Claude calls
// proper-lockfile lock() with the CONFIG_DIR, so the lock dir is
// realpath(CONFIG_DIR)+".lock" — a SIBLING of the profile dir, NOT
// <profile>/.credentials.json.lock. A mismatch means the daemon and a running
// session could refresh simultaneously and burn the rotating token.
func TestClaudeLockPath_IsConfigDirSibling(t *testing.T) {
	configDir := t.TempDir()
	credPath := filepath.Join(configDir, ".credentials.json")

	got, err := claudeLockPath(credPath)
	if err != nil {
		t.Fatalf("claudeLockPath: %v", err)
	}
	resolvedDir, err := filepath.EvalSymlinks(configDir)
	if err != nil {
		t.Fatalf("evalsymlinks: %v", err)
	}
	want := resolvedDir + ".lock"
	if got != want {
		t.Fatalf("lock path = %q; want CONFIG_DIR sibling %q", got, want)
	}
	// Explicitly NOT the credentials-file lock.
	if got == credPath+".lock" {
		t.Fatalf("lock path must NOT be the credentials-file lock (%s); Claude locks the CONFIG_DIR", got)
	}
}

// (7) acquireLock steals a stale lock (older than the stale threshold) but
// refuses a fresh one within the acquisition timeout.
func TestAcquireLock_StealsStaleLockRefusesFresh(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".credentials.json.lock")

	// Fresh lock held by "another process" → cannot acquire within a short timeout.
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		t.Fatalf("seed fresh lock: %v", err)
	}
	if release, err := acquireLock(lockPath, 200*time.Millisecond); err == nil {
		release()
		t.Fatal("expected acquireLock to time out against a fresh held lock")
	}

	// Age the lock past the stale threshold → it gets stolen.
	stale := time.Now().Add(-2 * lockStaleThreshold)
	if err := os.Chtimes(lockPath, stale, stale); err != nil {
		t.Fatalf("age lock: %v", err)
	}
	release, err := acquireLock(lockPath, 2*time.Second)
	if err != nil {
		t.Fatalf("expected to steal a stale lock; got %v", err)
	}
	release()
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock must be removed after release; stat err = %v", err)
	}
}
