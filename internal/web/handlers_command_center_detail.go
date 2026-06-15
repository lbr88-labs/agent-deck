package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/logging"
)

// Command Center v2 (Phase 1) — per-project DETAIL pages, correlated reply
// read-back, and context-aware comment routing. See
// conductor/agent-deck/COMMAND-CENTER-DESIGN.md (Addenda 1-4).
//
// A detail page is a LIVE AGGREGATION of four feeds for one conductor/project
// (Addendum 1), never hand-written:
//   (a) latest status/activity   — conductor session status + status.json headline
//   (b) produced docs/summaries  — markdown in conductor/<name>/outputs/, rendered inline
//   (c) live session list        — that conductor's active children (noise filtered)
//   (d) decisions/questions       — its decisions-waiting (scoped subset of OPEN-ITEMS §D)
//
// New endpoints (all behind the existing authorize/CSRF/mutation/rate gates):
//   GET /api/command-center/detail/{name}   — the detail aggregation for one conductor
//   GET /api/command-center/reply           — correlated reply read-back via `session output -q`
// and an extension of POST /api/command-center/ask to honour the `context`
// object so a detail-page input / 💬 comment routes to the OWNING conductor
// (Addendum 2 context-aware routing) with the commented entity referenced.

// CommandCenterDetail is the live aggregation rendered by a project detail page.
type CommandCenterDetail struct {
	Name        string `json:"name"`   // conductor short name, e.g. "agent-deck"
	Target      string `json:"target"` // routing target, e.g. "conductor-agent-deck"
	Status      string `json:"status"` // conductor session status (or "absent")
	Substate    string `json:"substate,omitempty"`
	GeneratedAt string `json:"generatedAt"`

	// Headline is the plain-language "currently working on" (status.json
	// headline if present, else the conductor's latest prompt).
	Headline string `json:"headline,omitempty"`
	// InProgress / RecentlyDone come from the structured status.json contract
	// when the conductor has adopted it; empty otherwise.
	InProgress   []string `json:"inProgress,omitempty"`
	RecentlyDone []string `json:"recentlyDone,omitempty"`

	// Docs are the conductor's produced docs/summaries from its outputs/ dir,
	// rendered markdown→HTML inline, newest first. Self-maintaining (Addendum 1).
	Docs []CommandCenterDoc `json:"docs"`
	// Sessions are the conductor's active children (error/stopped filtered out).
	Sessions []CommandCenterSession `json:"sessions"`
	Counts   CommandCenterCounts    `json:"counts"`
	// Decisions are the decisions-waiting surfaced for THIS conductor.
	Decisions []CommandCenterDecision `json:"decisions"`
}

// CommandCenterDoc is one rendered output doc on a detail page.
type CommandCenterDoc struct {
	Name      string `json:"name"`      // file name, e.g. "research-summary.md"
	Title     string `json:"title"`     // first heading or the file name
	HTML      string `json:"html"`      // rendered markdown
	UpdatedAt string `json:"updatedAt"` // file mtime, RFC3339
}

// conductorOutputsDirName is the fleet-wide convention: each conductor drops its
// key docs/updates into conductor/<name>/outputs/ and the Command Center renders
// whatever's there, live (Addendum 1).
const conductorOutputsDirName = "outputs"

// maxDocBytes bounds a single rendered doc so a runaway file can't bloat the
// snapshot. maxDocsPerConductor bounds how many are surfaced (newest first).
const (
	maxDocBytes         = 256 * 1024
	maxDocsPerConductor = 25
)

// handleCommandCenterDetail serves the live aggregation for one conductor.
// Path: /api/command-center/detail/{name}.
func (s *Server) handleCommandCenterDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	if !s.authorizeRequest(r) {
		writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/command-center/detail/")
	name = strings.Trim(name, "/")
	// Reject path-traversal / multi-segment names — a conductor name is a single
	// directory component by construction.
	if name == "" || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		writeAPIError(w, http.StatusBadRequest, "INVALID_NAME", "invalid conductor name")
		return
	}

	snapshot, err := s.loadCommandCenterSnapshot(nil)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to build snapshot")
		return
	}
	detail := buildCommandCenterDetail(snapshot, name, conductorArtifactDir())
	if detail == nil {
		writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "unknown conductor")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(detail)
}

// buildCommandCenterDetail is the pure synthesis function for a detail page,
// reusing the already-built snapshot (live sessions/status/decisions) and
// folding in the on-disk outputs/ docs + structured status.json. Returns nil if
// the conductor is not a known project in the snapshot.
func buildCommandCenterDetail(snap *CommandCenterSnapshot, name, artifactDir string) *CommandCenterDetail {
	if snap == nil {
		return nil
	}
	var row *CommandCenterConductor
	for i := range snap.Conductors {
		if snap.Conductors[i].Name == name {
			row = &snap.Conductors[i]
			break
		}
	}
	if row == nil {
		return nil
	}

	detail := &CommandCenterDetail{
		Name:        row.Name,
		Target:      row.Target,
		Status:      row.Status,
		Substate:    row.Substate,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Headline:    row.CurrentlyWorkingOn,
		Sessions:    row.Sessions,
		Counts:      row.Counts,
		Docs:        []CommandCenterDoc{},
		Decisions:   []CommandCenterDecision{},
	}
	if detail.Sessions == nil {
		detail.Sessions = []CommandCenterSession{}
	}

	// (d) decisions scoped to this conductor: the subset of the parsed
	// OPEN-ITEMS §D set whose owner route matches this conductor's target.
	for _, d := range snap.DecisionsWaiting {
		if d.Route == row.Target {
			detail.Decisions = append(detail.Decisions, d)
		}
	}

	// (a) structured status.json (inProgress / recentlyDone) when present.
	if artifactDir != "" {
		if feed := loadConductorStatusStruct(filepath.Join(artifactDir, name)); feed != nil {
			if detail.Headline == "" {
				detail.Headline = firstLine(feed.Headline)
			}
			detail.InProgress = feed.InProgress
			detail.RecentlyDone = feed.RecentlyDone
		}
		// (b) produced docs from outputs/, rendered inline newest-first.
		detail.Docs = loadConductorDocs(filepath.Join(artifactDir, name, conductorOutputsDirName))
	}
	return detail
}

// conductorStatusStruct is the full structured status.json contract (the v1.x
// hardening, read-side). All fields optional; a conductor adopts as much as it
// wants and the detail page renders whatever is present.
type conductorStatusStruct struct {
	Headline     string   `json:"headline"`
	InProgress   []string `json:"inProgress"`
	RecentlyDone []string `json:"recentlyDone"`
}

func loadConductorStatusStruct(dir string) *conductorStatusStruct {
	raw, err := os.ReadFile(filepath.Join(dir, "status.json"))
	if err != nil {
		return nil
	}
	var parsed conductorStatusStruct
	if json.Unmarshal(raw, &parsed) != nil {
		return nil
	}
	return &parsed
}

// loadConductorDocs reads, sorts (newest mtime first), and renders the markdown
// docs in a conductor's outputs/ dir. Best-effort: returns empty on any failure.
func loadConductorDocs(outputsDir string) []CommandCenterDoc {
	docs := []CommandCenterDoc{}
	entries, err := os.ReadDir(outputsDir)
	if err != nil {
		return docs
	}
	type docFile struct {
		name string
		mod  time.Time
	}
	var files []docFile
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		lower := strings.ToLower(ent.Name())
		if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".markdown") {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		files = append(files, docFile{name: ent.Name(), mod: info.ModTime()})
	}
	// Newest first so the latest update leads the page.
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	if len(files) > maxDocsPerConductor {
		files = files[:maxDocsPerConductor]
	}
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(outputsDir, f.name))
		if err != nil {
			continue
		}
		if len(raw) > maxDocBytes {
			raw = raw[:maxDocBytes]
		}
		docs = append(docs, CommandCenterDoc{
			Name:      f.name,
			Title:     docTitle(string(raw), f.name),
			HTML:      renderMarkdown(string(raw)),
			UpdatedAt: f.mod.UTC().Format(time.RFC3339),
		})
	}
	return docs
}

// conductorDocsMeta returns a cheap (name+mtime only) count of markdown docs in
// a conductor's outputs/ dir plus the newest mtime (RFC3339). Used in the
// snapshot/list row so it stays light; the detail page does the full render.
func conductorDocsMeta(outputsDir string) (int, string) {
	entries, err := os.ReadDir(outputsDir)
	if err != nil {
		return 0, ""
	}
	count := 0
	var newest time.Time
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		lower := strings.ToLower(ent.Name())
		if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".markdown") {
			continue
		}
		count++
		if info, err := ent.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	if count == 0 {
		return 0, ""
	}
	return count, newest.UTC().Format(time.RFC3339)
}

// docTitle returns the first Markdown heading text, or the file name if none.
func docTitle(src, fallback string) string {
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if m := mdHeading.FindStringSubmatch(t); m != nil {
			return strings.TrimSpace(m[2])
		}
		if t != "" {
			break // first non-empty line isn't a heading; use the fallback
		}
	}
	return fallback
}

// handleCommandCenterReply serves the correlated reply read-back (design §5.3
// tier 2). It shells the supported `session output -q` path for the target's
// latest response — the same read primitive the CLI uses — and returns it so the
// panel can show "you asked X, conductor replied Y" inline. Read-only.
func (s *Server) handleCommandCenterReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	if !s.authorizeRequest(r) {
		writeAPIError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_TARGET", "target is required")
		return
	}
	// Re-validate target against the live allowlist — never read an arbitrary
	// session's output from an unauthenticated-shaped param.
	snapshot, err := s.loadCommandCenterSnapshot(nil)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to resolve targets")
		return
	}
	resolved := resolveAskTarget(target, snapshot.AskTargets)
	if resolved == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_TARGET", "unknown target")
		return
	}

	exe, err := os.Executable()
	if err != nil || exe == "" {
		exe = "agent-deck"
	}
	cmdCtx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, exe, "-p", s.cfg.Profile, "session", "output", resolved, "-q")
	cmd.Env = os.Environ()
	out, runErr := cmd.CombinedOutput()
	reply := strings.TrimSpace(string(out))
	corr := strings.TrimSpace(r.URL.Query().Get("correlationId"))
	if runErr != nil {
		logging.ForComponent(logging.CompWeb).Warn("command_center_reply_read_failed",
			slog.String("target", resolved),
			slog.String("correlation", correlationFingerprint(corr)),
			slog.String("error", runErr.Error()))
		// Not fatal: an empty reply is a valid "nothing back yet" state.
		reply = ""
	}
	const maxReply = 8 * 1024
	if len(reply) > maxReply {
		reply = reply[len(reply)-maxReply:]
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"target":        resolved,
		"correlationId": corr,
		"reply":         reply,
		"readAt":        time.Now().UTC().Format(time.RFC3339),
	})
}

// composeAskMessage builds the tagged, context-scoped message routed to a
// conductor. The [command-center] tag mirrors every other inbound channel; when
// a comment carries entity context (a decision id, a session, or a project) it
// is referenced inline so the receiving conductor knows exactly what is being
// commented on (Addendum 2 context-carrying comment).
func composeAskMessage(text string, ctx askContext) string {
	ref := ""
	switch {
	case ctx.DecisionID != "":
		ref = " re " + ctx.DecisionID
	case ctx.SessionTitle != "":
		ref = " re session " + ctx.SessionTitle
	case ctx.Project != "":
		ref = " re " + ctx.Project
	}
	return "[command-center" + ref + "] " + text
}

// askContext is the optional comment-on-anything context carried by /ask.
type askContext struct {
	DecisionID   string `json:"decisionId,omitempty"`
	SessionTitle string `json:"sessionTitle,omitempty"`
	Project      string `json:"project,omitempty"`
}

// correlationFingerprint is a short stable id for the audit log of a reply read.
func correlationFingerprint(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:6])
}
