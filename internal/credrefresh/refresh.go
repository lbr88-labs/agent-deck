// Package credrefresh is the subscription-safe keep-warm OAuth refresh daemon
// (Part B of the recurring "/login" outage fix). It reads the single canonical
// `.credentials.json` for a profile, and when the access token is within a
// short window of expiry, exchanges the rotating refresh token for a fresh one
// at Anthropic's OAuth token endpoint and atomically writes the result back.
//
// Why a daemon at all. Claude Code stores OAuth credentials in
// `$CLAUDE_CONFIG_DIR/.credentials.json` and re-reads that file from disk on
// access-token expiry (confirmed by disassembling v2.1.159 — the Linux
// plaintext store has no in-memory cache and watches the file mtime). When N
// agent-deck worker sessions all symlink their scratch `.credentials.json` to
// ONE canonical file, Claude's cross-process lock — keyed on
// `realpath(...)` — collapses to a single lock, so the workers serialize their
// refreshes instead of racing. A refresh daemon that keeps the canonical token
// warm removes even the brief lock-contention storm and the cold-start case
// where the whole host was asleep past expiry. No API key, no per-worker
// restart: workers pick up the new token on their next disk read. See
// /tmp/oauth-fix/SUBSCRIPTION-FIX.md and the bug_oauth_multisession_rotation
// root-cause memo.
//
// This package NEVER hits the real endpoint in tests and NEVER hardcodes a
// token: the endpoint and HTTP client are injectable, and tests pass a mock.
package credrefresh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultTokenEndpoint is Claude Code's OAuth token endpoint (the
	// `claudeAiOauth` subscription flow), read from the shipped binary.
	// #nosec G101 -- this is a public OAuth endpoint URL, not a credential.
	DefaultTokenEndpoint = "https://platform.claude.com/v1/oauth/token"

	// ClaudeCodeClientID is Claude Code's well-known public OAuth client_id,
	// used as a fallback when the stored credentials omit `clientId`. The
	// token endpoint rejects refresh requests without a client_id.
	// #nosec G101 -- public OAuth client identifier, not a secret.
	ClaudeCodeClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"

	// DefaultThreshold is the early-refresh window: refresh when the access
	// token expires within this much time. Access tokens live ~1h.
	DefaultThreshold = 20 * time.Minute

	// DefaultInterval is the recommended daemon cadence. Shorter than the
	// ~1h token lifetime so a single missed tick never lets the token lapse.
	DefaultInterval = 25 * time.Minute

	// oauthKey is the top-level object holding the subscription credentials.
	oauthKey = "claudeAiOauth"

	// httpTimeout bounds a single token request (Claude uses 10s).
	httpTimeout = 10 * time.Second

	// lockStaleThreshold matches proper-lockfile semantics: a lock whose
	// directory mtime is older than this is treated as abandoned and stolen.
	// Generous relative to our sub-second-to-10s hold so we never steal a
	// lock another writer is actively holding.
	lockStaleThreshold = 30 * time.Second

	// lockRetryInterval is how often acquireLock retries a held lock.
	lockRetryInterval = 50 * time.Millisecond
)

// RefreshConfig configures a single refresh attempt. All fields are optional;
// zero values fall back to the package defaults.
type RefreshConfig struct {
	// TokenEndpoint is the OAuth /token URL. Defaults to DefaultTokenEndpoint.
	TokenEndpoint string
	// HTTPClient performs the token request. Defaults to a client with httpTimeout.
	HTTPClient *http.Client
	// Threshold is the early-refresh window. Defaults to DefaultThreshold.
	Threshold time.Duration
	// Now is an injectable clock for the expiry decision. Defaults to time.Now.
	Now func() time.Time
}

// RefreshResult reports the outcome of a single RefreshIfNeeded call.
type RefreshResult struct {
	// Refreshed is true iff a rotation happened and canonical was rewritten.
	Refreshed bool
	// Reason explains a no-op (e.g. "token not near expiry").
	Reason string
	// ExpiresAt is the access-token expiry after the call (rotated or current).
	ExpiresAt time.Time
}

func (c RefreshConfig) endpoint() string {
	if c.TokenEndpoint != "" {
		return c.TokenEndpoint
	}
	return DefaultTokenEndpoint
}

func (c RefreshConfig) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: httpTimeout}
}

func (c RefreshConfig) threshold() time.Duration {
	if c.Threshold > 0 {
		return c.Threshold
	}
	return DefaultThreshold
}

func (c RefreshConfig) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// tokenResponse is the subset of the OAuth /token response we consume.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // seconds
}

// RefreshIfNeeded reads the canonical credentials at credPath, and if the
// access token expires within cfg.Threshold, exchanges the rotating refresh
// token for a fresh one and atomically rewrites canonical (temp+rename, 0600).
//
// It serializes against Claude's own refreshes by holding a
// proper-lockfile-compatible lock at `realpath(credPath)+".lock"` for the
// duration of the read-rotate-write. On any endpoint error the canonical file
// is left byte-for-byte unchanged — a transient 4xx/5xx must never corrupt or
// half-write the only token. Unknown fields (subscriptionType, rateLimitTier,
// and any extra keys) are preserved verbatim across the round-trip.
func RefreshIfNeeded(credPath string, cfg RefreshConfig) (RefreshResult, error) {
	// Resolve symlinks so we read/write the canonical file even when credPath
	// is a symlink to it.
	realPath := credPath
	if rp, err := filepath.EvalSymlinks(credPath); err == nil {
		realPath = rp
	}

	// Lock the SAME path Claude Code locks on its OAuth-refresh path so the
	// daemon and any running session never refresh at the same instant.
	// Verified in the shipped binary: the refresh path calls
	// proper-lockfile's lock() with the CONFIG_DIR (Y7()), NOT the
	// credentials file — so the lock dir is realpath(CONFIG_DIR)+".lock"
	// (e.g. ~/.claude.lock), a sibling of the profile dir. Matching it
	// exactly is what makes the cross-process serialization real.
	lockPath, err := claudeLockPath(credPath)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("resolve credentials lock path: %w", err)
	}
	release, err := acquireLock(lockPath, httpTimeout+5*time.Second)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("acquire credentials lock: %w", err)
	}
	defer release()

	doc, oauth, err := readCredentials(realPath)
	if err != nil {
		return RefreshResult{}, err
	}

	expiresAt := time.UnixMilli(jsonInt64(oauth["expiresAt"]))
	if expiresAt.Sub(cfg.now()) > cfg.threshold() {
		return RefreshResult{Refreshed: false, Reason: "token not near expiry", ExpiresAt: expiresAt}, nil
	}

	refreshToken := jsonString(oauth["refreshToken"])
	if refreshToken == "" {
		return RefreshResult{}, fmt.Errorf("no refresh token in %s", realPath)
	}

	tok, err := requestRefresh(cfg, refreshToken, jsonString(oauth["clientId"]), jsonScopes(oauth["scopes"]))
	if err != nil {
		// Canonical untouched on failure.
		return RefreshResult{Refreshed: false, ExpiresAt: expiresAt}, err
	}

	newExpiry := cfg.now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	oauth["accessToken"] = tok.AccessToken
	oauth["refreshToken"] = tok.RefreshToken
	oauth["expiresAt"] = newExpiry.UnixMilli()
	doc[oauthKey] = oauth

	out, err := json.Marshal(doc)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("marshal refreshed credentials: %w", err)
	}
	if err := atomicWriteFile(realPath, out, 0o600); err != nil {
		return RefreshResult{}, fmt.Errorf("write refreshed credentials: %w", err)
	}
	return RefreshResult{Refreshed: true, ExpiresAt: newExpiry}, nil
}

// readCredentials parses the credentials file, returning the full document
// (for round-trip preservation of unknown keys) and the claudeAiOauth object.
func readCredentials(path string) (map[string]any, map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read credentials %s: %w", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse credentials %s: %w", path, err)
	}
	oauth, ok := doc[oauthKey].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("%s missing %q object", path, oauthKey)
	}
	return doc, oauth, nil
}

// requestRefresh POSTs the rotating refresh token to the OAuth token endpoint
// and returns the rotated tokens. Anthropic's token endpoint requires a JSON
// body (grant_type/refresh_token/client_id); a form-urlencoded body is
// rejected with 400 "Invalid request format". The scopes argument is unused —
// the refresh grant does not re-scope.
func requestRefresh(cfg RefreshConfig, refreshToken, clientID string, _ []string) (tokenResponse, error) {
	if clientID == "" {
		clientID = ClaudeCodeClientID
	}

	payload, err := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     clientID,
	})
	if err != nil {
		return tokenResponse{}, fmt.Errorf("marshal token request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.endpoint(), bytes.NewReader(payload))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := cfg.httpClient().Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return tokenResponse{}, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return tokenResponse{}, fmt.Errorf("parse token response: %w", err)
	}
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		return tokenResponse{}, fmt.Errorf("token response missing access_token or refresh_token")
	}
	return tok, nil
}

// jsonString coerces a decoded JSON value to a string ("" if absent/wrong type).
func jsonString(v any) string {
	s, _ := v.(string)
	return s
}

// jsonInt64 coerces a decoded JSON number (float64 from encoding/json) to int64.
func jsonInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

// jsonScopes coerces a decoded JSON array of strings to []string.
func jsonScopes(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
