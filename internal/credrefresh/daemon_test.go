package credrefresh

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Run must perform an immediate refresh on startup (not wait a full interval),
// then stop cleanly when the context is cancelled.
func TestRun_RefreshesImmediatelyThenStopsOnCancel(t *testing.T) {
	now := fixedNow()
	configDir := t.TempDir()
	credPath := filepath.Join(configDir, ".credentials.json")
	writeCreds(t, credPath, map[string]any{
		"accessToken": "old", "refreshToken": "rt", "clientId": "c",
		"expiresAt": float64(now().Add(time.Minute).UnixMilli()), // near expiry
	}, nil)

	srv, calls := mockTokenServer(t, "new", "new-rt", 3600)

	var mu sync.Mutex
	var results []RefreshResult
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		Run(ctx, DaemonConfig{
			ConfigDirs: []string{configDir},
			Interval:   time.Hour, // long — only the immediate tick should fire
			Refresh:    RefreshConfig{TokenEndpoint: srv.URL, HTTPClient: srv.Client(), Threshold: 20 * time.Minute, Now: now},
			OnResult: func(_ string, r RefreshResult, _ error) {
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
			},
		})
		close(done)
	}()

	// Wait for the immediate refresh, then cancel.
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		n := len(results)
		mu.Unlock()
		if n >= 1 {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("Run did not perform an immediate refresh within 2s")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}

	if calls.len() < 1 {
		t.Fatalf("expected at least one token request from the immediate tick; got %d", calls.len())
	}
	if !results[0].Refreshed {
		t.Fatalf("immediate tick should have refreshed the near-expiry token")
	}
}

// CanonicalCredPath joins a config dir with the credentials file name.
func TestCanonicalCredPath(t *testing.T) {
	got := CanonicalCredPath("/home/x/.claude")
	want := filepath.Join("/home/x/.claude", ".credentials.json")
	if got != want {
		t.Fatalf("CanonicalCredPath = %q; want %q", got, want)
	}
}
