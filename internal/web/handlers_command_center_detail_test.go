package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ccDetailArtifacts writes a conductor artifact tree (status.json + outputs/
// docs + OPEN-ITEMS.md) into a temp dir, returning the artifactDir root that
// buildCommandCenterDetail / buildCommandCenterSnapshot expect.
func ccDetailArtifacts(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	adDir := filepath.Join(dir, "agent-deck")
	outDir := filepath.Join(adDir, "outputs")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// structured status feed
	status := `{"headline":"v1.9.68 shipped","inProgress":["#1431 spawn fix"],"recentlyDone":["v1.9.68 release"]}`
	if err := os.WriteFile(filepath.Join(adDir, "status.json"), []byte(status), 0o644); err != nil {
		t.Fatal(err)
	}
	// two output docs
	if err := os.WriteFile(filepath.Join(outDir, "summary.md"),
		[]byte("# Release Summary\n\nShipped **v1.9.68** today.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "notes.md"),
		[]byte("plain notes file with no heading\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// decisions for §D scoping
	openItems := "## D. DECISIONS — waiting on Ashesh\n#1361 is conductor.enabled necessary?\n\n## E. OTHER\nx\n"
	if err := os.WriteFile(filepath.Join(adDir, "OPEN-ITEMS.md"), []byte(openItems), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBuildCommandCenterDetailAggregates(t *testing.T) {
	artifactDir := ccDetailArtifacts(t)
	snap := buildCommandCenterSnapshot(ccTestMenu(), "personal", artifactDir, nil)

	detail := buildCommandCenterDetail(snap, "agent-deck", artifactDir)
	if detail == nil {
		t.Fatal("expected detail for agent-deck, got nil")
	}
	// (a) status/activity
	if detail.Target != "conductor-agent-deck" {
		t.Errorf("target: got %q", detail.Target)
	}
	if detail.Status != "running" {
		t.Errorf("status: got %q", detail.Status)
	}
	// status.json headline wins over latest prompt
	if detail.Headline != "v1.9.68 shipped" {
		t.Errorf("headline: got %q", detail.Headline)
	}
	if len(detail.InProgress) != 1 || detail.InProgress[0] != "#1431 spawn fix" {
		t.Errorf("inProgress: got %+v", detail.InProgress)
	}
	// (b) docs rendered inline, newest first, with title extraction + fallback
	if len(detail.Docs) != 2 {
		t.Fatalf("expected 2 docs, got %d: %+v", len(detail.Docs), detail.Docs)
	}
	var summary *CommandCenterDoc
	for i := range detail.Docs {
		if detail.Docs[i].Name == "summary.md" {
			summary = &detail.Docs[i]
		}
	}
	if summary == nil {
		t.Fatal("summary.md not found in docs")
	}
	if summary.Title != "Release Summary" {
		t.Errorf("doc title from heading: got %q", summary.Title)
	}
	if !strings.Contains(summary.HTML, "<strong>v1.9.68</strong>") {
		t.Errorf("doc html not rendered: %s", summary.HTML)
	}
	// (c) live sessions filtered (error/stopped excluded — ccTestMenu has those)
	if len(detail.Sessions) != 1 || detail.Sessions[0].ID != "child-1" {
		t.Errorf("sessions: got %+v", detail.Sessions)
	}
	// (d) decisions scoped to this conductor
	if len(detail.Decisions) != 1 || detail.Decisions[0].ID != "#1361" {
		t.Errorf("decisions: got %+v", detail.Decisions)
	}
}

func TestBuildCommandCenterDetailUnknownConductor(t *testing.T) {
	snap := buildCommandCenterSnapshot(ccTestMenu(), "personal", "", nil)
	if d := buildCommandCenterDetail(snap, "does-not-exist", ""); d != nil {
		t.Fatalf("expected nil for unknown conductor, got %+v", d)
	}
}

func TestConductorDocsMeta(t *testing.T) {
	artifactDir := ccDetailArtifacts(t)
	count, last := conductorDocsMeta(filepath.Join(artifactDir, "agent-deck", "outputs"))
	if count != 2 {
		t.Errorf("doc count: got %d", count)
	}
	if last == "" {
		t.Errorf("expected lastDocAt, got empty")
	}
	// snapshot row should carry the meta
	snap := buildCommandCenterSnapshot(ccTestMenu(), "personal", artifactDir, nil)
	if snap.Conductors[0].DocCount != 2 {
		t.Errorf("snapshot DocCount: got %d", snap.Conductors[0].DocCount)
	}
	if snap.Conductors[0].LastDocAt == "" {
		t.Errorf("snapshot LastDocAt empty")
	}
}

func TestComposeAskMessageContext(t *testing.T) {
	cases := []struct {
		ctx  askContext
		want string
	}{
		{askContext{}, "[command-center] hi"},
		{askContext{DecisionID: "#1361"}, "[command-center re #1361] hi"},
		{askContext{SessionTitle: "fix-1431"}, "[command-center re session fix-1431] hi"},
		{askContext{Project: "ryan"}, "[command-center re ryan] hi"},
	}
	for _, c := range cases {
		if got := composeAskMessage("hi", c.ctx); got != c.want {
			t.Errorf("composeAskMessage(%+v) = %q, want %q", c.ctx, got, c.want)
		}
	}
}

func TestCommandCenterDetailEndpointRejectsTraversal(t *testing.T) {
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"})
	srv.menuData = &fakeMenuDataLoader{snapshot: ccTestMenu()}

	// ".." cases are cleaned + redirected (3xx) by net/http.ServeMux before the
	// handler ever runs, which is itself a safe rejection of the traversal. The
	// "a/b" multi-segment case reaches the handler and is rejected with 400.
	for _, bad := range []string{"..", "../etc", "a/b"} {
		req := httptest.NewRequest(http.MethodGet, "/api/command-center/detail/"+bad, nil)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		ok := rr.Code == http.StatusBadRequest || rr.Code == http.StatusNotFound ||
			(rr.Code >= 300 && rr.Code < 400)
		if !ok {
			t.Errorf("path %q: expected 4xx/3xx rejection, got %d", bad, rr.Code)
		}
	}
	// A multi-segment name reaches the handler and must be rejected 400 with the
	// INVALID_NAME error (never served as a detail page).
	req := httptest.NewRequest(http.MethodGet, "/api/command-center/detail/a/b", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "INVALID_NAME") {
		t.Errorf("multi-segment name: expected 400 INVALID_NAME, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommandCenterDetailEndpointJSON(t *testing.T) {
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"})
	srv.menuData = &fakeMenuDataLoader{snapshot: ccTestMenu()}

	req := httptest.NewRequest(http.MethodGet, "/api/command-center/detail/agent-deck", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var detail CommandCenterDetail
	if err := json.Unmarshal(rr.Body.Bytes(), &detail); err != nil {
		t.Fatalf("invalid detail json: %v", err)
	}
	if detail.Name != "agent-deck" {
		t.Errorf("name: got %q", detail.Name)
	}
	// Privacy: no secret-shaped fields leak into a detail page either.
	body := rr.Body.String()
	for _, banned := range []string{"token", "apiKey", "secret", "password", "credential"} {
		if strings.Contains(body, banned) {
			t.Fatalf("detail leaked secret-shaped field %q: %s", banned, body)
		}
	}
}

func TestCommandCenterReplyRejectsUnknownTarget(t *testing.T) {
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"})
	srv.menuData = &fakeMenuDataLoader{snapshot: ccTestMenu()}

	req := httptest.NewRequest(http.MethodGet, "/api/command-center/reply?target=conductor-evil", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown target, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCommandCenterReplyRequiresTarget(t *testing.T) {
	srv := NewServer(Config{ListenAddr: "127.0.0.1:0"})
	srv.menuData = &fakeMenuDataLoader{snapshot: ccTestMenu()}

	req := httptest.NewRequest(http.MethodGet, "/api/command-center/reply", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing target, got %d", rr.Code)
	}
}
