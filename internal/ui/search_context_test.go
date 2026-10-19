package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/hub"
	"github.com/asheshgoplani/agent-deck/internal/session"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// A newer session must not outrank a better metadata match; only equal
// relevance uses last opened, never last activity or creation time.
func TestSearchRelevanceThenLastOpened(t *testing.T) {
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := old.Add(time.Hour)
	items := []*SessionSearchResult{
		{SessionID: "path", Title: "unrelated", ProjectPath: "/work/api", LastOpened: newer},
		{SessionID: "older-title", Title: "api", LastOpened: old, LastActivity: newer.Add(time.Hour)},
		{SessionID: "newer-title", Title: "api", LastOpened: newer},
		{SessionID: "unknown-title", Title: "api"},
	}
	got := filterSessionSearchResults(items, "api")
	want := []string{"newer-title", "older-title", "unknown-title", "path"}
	for i, id := range want {
		if len(got) <= i || got[i].SessionID != id {
			t.Fatalf("result %d = %+v, want %q (all: %+v)", i, got, id, got)
		}
	}
	got = filterSessionSearchResults(items, "")
	if got[0].SessionID != "path" || got[1].SessionID != "newer-title" || got[3].SessionID != "unknown-title" {
		t.Fatalf("empty query did not use stable last-opened ordering: %+v", got)
	}
}

func TestSearchFuzzyRelevanceBeforeLastOpened(t *testing.T) {
	items := []*SessionSearchResult{
		{SessionID: "scattered", Title: "axxxxxpxxxxxi", LastOpened: time.Now()},
		{SessionID: "compact", Title: "apix"},
	}
	got := filterSessionSearchResults(items, "apz")
	if len(got) != 0 {
		t.Fatalf("unexpected nonmatching fuzzy results: %+v", got)
	}
	items[1].Title = "aPi"
	got = filterSessionSearchResults(items, "ai")
	if len(got) != 2 || got[0].SessionID != "compact" {
		t.Fatalf("fuzzy relevance lost to recency: %+v", got)
	}
}

func TestSearchMetadataKeepsUnknownActivityUnknown(t *testing.T) {
	created := time.Date(2025, 2, 3, 4, 5, 0, 0, time.UTC)
	opened := created.Add(time.Hour)
	item := localSessionSearchResult(&session.Instance{ID: "one", Title: "same", Tool: "omp", ProjectPath: "/work/alpha", GroupPath: "team/backend", Status: session.StatusWaiting, CreatedAt: created, LastAccessedAt: opened}, "laptop")
	if !item.LastOpened.Equal(opened) || !item.LastActivity.IsZero() {
		t.Fatalf("timestamps conflated: opened=%v activity=%v", item.LastOpened, item.LastActivity)
	}
	s := NewSearch()
	s.SetFleetItems([]*SessionSearchResult{item})
	s.SetSize(140, 40)
	s.ShowGlobal()
	view := s.View()
	for _, text := range []string{"/work/alpha", "team/backend", "waiting", "Opened:", "Activity: unknown", "laptop"} {
		if !strings.Contains(view, text) {
			t.Errorf("search view missing %q:\n%s", text, view)
		}
	}
}

func TestSearchChatToggleAsyncLocalSnippetsAndReopenDefault(t *testing.T) {
	home := setXDGTestHome(t)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	const sessionID = "11111111-1111-4111-8111-111111111111"
	path := filepath.Join(home, ".claude", "projects", "-work-local", sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"type":"user","sessionId":"`+sessionID+`","message":{"role":"user","content":"Discuss the orchard migration with Celeste"}}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	local := &session.Instance{ID: "local", Title: "local title", Tool: "claude", ProjectPath: "/work/local", ClaudeSessionID: sessionID}
	s := NewSearch()
	s.SetFleetItems([]*SessionSearchResult{
		localSessionSearchResult(local, "laptop"),
		{Source: SearchSourceRemote, SessionID: "remote", Title: "remote title", Tool: "claude", ProjectPath: "/work/local", RemoteName: "remote", Chat: session.NewChatSearchBinding(local)},
	})
	s.SetSize(140, 40)
	s.ShowGlobal()
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("orchard")})
	if cmd != nil {
		// Text-input cursor commands are unrelated to transcript I/O.
		if _, ok := cmd().(searchChatDebounceMsg); ok {
			t.Fatal("chat search ran before explicit opt-in")
		}
	}
	if len(s.results) != 0 || s.chatEnabled {
		t.Fatalf("chat search should default off: %+v", s.results)
	}
	_, cmd = s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil || !s.chatEnabled {
		t.Fatal("Search chats toggle did not schedule asynchronous work")
	}
	_, scan := s.Update(cmd())
	if scan == nil {
		t.Fatal("debounce did not schedule disk search")
	}
	_, _ = s.Update(scan())
	if len(s.results) != 1 || s.results[0].SessionID != "local" || !strings.Contains(s.results[0].Snippet, "orchard") {
		t.Fatalf("local transcript hit/snippet missing or remote included: results=%+v report=%+v", s.results, s.chatReport)
	}
	if !strings.Contains(s.View(), "orchard") {
		t.Fatal("matching chat snippet is not visible")
	}
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if s.Scope() != SearchScopeLocal || !s.chatEnabled {
		t.Fatal("chat toggle was conflated with existing scope toggle")
	}
	s.Hide()
	s.ShowGlobal()
	if s.chatEnabled {
		t.Fatal("Search chats must default off on reopen")
	}
}

func TestSearchChatCancelsAndRejectsStaleResponses(t *testing.T) {
	s := NewSearch()
	s.ShowGlobal()
	s.input.SetValue("old")
	s.updateResults()
	_, debounce := s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	_, scan := s.Update(debounce())
	if scan == nil {
		t.Fatal("missing chat search command")
	}
	oldGeneration := s.chatGeneration
	oldContext := s.chatContext
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("new")})
	if oldContext.Err() != context.Canceled {
		t.Fatal("editing query did not cancel the previous disk scan")
	}
	_, _ = s.Update(searchChatResultsMsg{generation: oldGeneration, report: session.ChatSearchReport{Matches: []session.ChatSearchMatch{{InstanceID: "stale", Snippet: "old result"}}}})
	if len(s.results) != 0 {
		t.Fatalf("stale search polluted current results: %+v", s.results)
	}
	currentContext := s.chatContext
	s.Hide()
	if currentContext != nil && currentContext.Err() != context.Canceled {
		t.Fatal("hiding overlay did not cancel pending search")
	}
	s.ShowGlobal()
	_, _ = s.Update(scan())
	if len(s.results) != 0 || s.chatEnabled {
		t.Fatal("late completion from previous opening was applied")
	}
}

func TestSearchViewFitsNarrowAndShortTerminals(t *testing.T) {
	for _, size := range [][2]int{{140, 45}, {80, 24}, {48, 16}, {30, 10}, {18, 6}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			s := NewSearch()
			items := make([]*SessionSearchResult, 20)
			for i := range items {
				items[i] = &SessionSearchResult{Title: strings.Repeat("title", 35), Tool: "omp", ProjectPath: "/very/long/" + strings.Repeat("path/", 40), GroupPath: "team/backend", Status: session.StatusWaiting}
			}
			s.SetFleetItems(items)
			s.SetSize(size[0], size[1])
			s.ShowGlobal()
			view := s.View()
			if w, h := lipgloss.Width(view), lipgloss.Height(view); w > size[0] || h > size[1] {
				t.Fatalf("view = %dx%d exceeds terminal %dx%d", w, h, size[0], size[1])
			}
		})
	}
}

func TestSearchChatMergeRanksMetadataThenOpenedAndKeepsSnapshotsClean(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewSearch()
	s.SetFleetItems([]*SessionSearchResult{
		{Source: SearchSourceLocal, SessionID: "chat-old", Title: "first", LastOpened: now},
		{Source: SearchSourceLocal, SessionID: "metadata", Title: "orchard"},
		{Source: SearchSourceLocal, SessionID: "chat-new", Title: "second", LastOpened: now.Add(time.Hour)},
		{Source: SearchSourceHub, SessionID: "foreign", Title: "hub", HubNodeID: "node"},
	})
	s.ShowGlobal()
	s.input.SetValue("orchard")
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	s.applyChatSearch(searchChatResultsMsg{
		generation: s.chatGeneration,
		report: session.ChatSearchReport{Matches: []session.ChatSearchMatch{
			{InstanceID: "chat-old", Snippet: "old orchard discussion"},
			{InstanceID: "chat-new", Snippet: "new orchard discussion"},
			{InstanceID: "foreign", Snippet: "foreign orchard discussion"},
		}},
	})
	for i, id := range []string{"metadata", "chat-new", "chat-old"} {
		if len(s.results) != 3 || s.results[i].SessionID != id {
			t.Fatalf("result %d should be %q: %+v", i, id, s.results)
		}
	}
	for _, item := range s.fleetItems {
		if item.Snippet != "" {
			t.Fatalf("async snippet mutated reusable metadata snapshot: %+v", item)
		}
	}
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if len(s.results) != 1 || s.results[0].SessionID != "metadata" || s.results[0].Snippet != "" {
		t.Fatalf("disabling chats retained chat-only results/snippets: %+v", s.results)
	}
}

func TestSearchChatScopeAndToggleCancelPendingWork(t *testing.T) {
	for _, key := range []tea.KeyMsg{{Type: tea.KeyTab}, {Type: tea.KeyCtrlS}} {
		t.Run(key.String(), func(t *testing.T) {
			s := NewSearch()
			s.ShowGlobal()
			s.input.SetValue("query")
			_, old := s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
			ctx, generation := s.chatContext, s.chatGeneration
			_, _ = s.Update(key)
			if ctx.Err() != context.Canceled {
				t.Fatal("scope/toggle change did not cancel search")
			}
			if _, cmd := s.Update(old()); cmd != nil {
				t.Fatal("stale debounce started a disk scan")
			}
			s.applyChatSearch(searchChatResultsMsg{generation: generation})
			if s.chatGeneration == generation {
				t.Fatal("scope/toggle change did not advance search generation")
			}
			s.Hide()
		})
	}
}

func TestSearchChatGroupScopeAndStatusFilterRemainLocal(t *testing.T) {
	s := NewSearch()
	s.SetScopedGroup("team")
	s.SetItems([]*session.Instance{
		{ID: "inside", Title: "first", GroupPath: "team", Tool: "omp", Status: session.StatusRunning},
		{ID: "outside", Title: "second", GroupPath: "other", Tool: "omp", Status: session.StatusRunning},
		{ID: "waiting", Title: "third", GroupPath: "team", Tool: "omp", Status: session.StatusWaiting},
	})
	s.ShowLocal()
	s.input.SetValue("running")
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	s.applyChatSearch(searchChatResultsMsg{generation: s.chatGeneration, report: session.ChatSearchReport{Matches: []session.ChatSearchMatch{
		{InstanceID: "outside", Snippet: "running outside"},
		{InstanceID: "waiting", Snippet: "running but still waiting"},
	}}})
	if len(s.results) != 1 || s.results[0].SessionID != "inside" {
		t.Fatalf("chats widened group/status filter: %+v", s.results)
	}
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
	if s.Scope() != SearchScopeLocal || s.scopedGroup != "team" {
		t.Fatal("group-local Tab behavior changed")
	}
}

func TestSearchChatLocalReloadInvalidatesAndReschedules(t *testing.T) {
	s := NewSearch()
	s.SetItems([]*session.Instance{{ID: "old", Title: "first", Tool: "omp"}})
	s.ShowLocal()
	s.input.SetValue("query")
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	ctx, generation := s.chatContext, s.chatGeneration
	s.SetItems([]*session.Instance{{ID: "new", Title: "second", Tool: "omp"}})
	if ctx.Err() != context.Canceled || s.chatGeneration == generation {
		t.Fatal("local snapshot replacement retained stale binding scan")
	}
	if s.takeChatSearchCmd() == nil {
		t.Fatal("local snapshot replacement did not reschedule active chat query")
	}
	s.Hide()
}

func TestHomeRoutesAsyncSlashSearchMessages(t *testing.T) {
	h := newHubProjectionHome(t, []*session.Instance{{ID: "one", Title: "plain", Tool: "omp"}})
	h.openFleetSearch()
	h.search.input.SetValue("orchard")
	_, debounce := h.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if debounce == nil {
		t.Fatal("home swallowed chat toggle command")
	}
	_, scan := h.Update(debounce())
	if scan == nil {
		t.Fatal("home did not route asynchronous debounce to slash search")
	}
	_, _ = h.Update(searchChatResultsMsg{generation: h.search.chatGeneration, report: session.ChatSearchReport{Matches: []session.ChatSearchMatch{{InstanceID: "one", Snippet: "orchard planning"}}}})
	if got := h.search.Selected(); got == nil || got.SessionID != "one" || got.Snippet != "orchard planning" {
		t.Fatalf("home did not apply slash chat result: %+v", got)
	}
	h.search.Hide()
}

func TestSearchDisplaysUnavailableToolsAndSanitizesRows(t *testing.T) {
	s := NewSearch()
	s.SetFleetItems([]*SessionSearchResult{{Source: SearchSourceLocal, SessionID: "unsupported", Tool: "mystery", Title: "orchard\x1b[2J", ProjectPath: "/tmp/path\nforged", Status: session.StatusWaiting}})
	s.SetSize(140, 40)
	s.ShowGlobal()
	s.input.SetValue("orchard")
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	s.applyChatSearch(searchChatResultsMsg{generation: s.chatGeneration, report: session.ChatSearchReport{Unavailable: []session.ChatSearchUnavailable{{InstanceID: "unsupported", Tool: "mystery", Reason: "unsupported format"}}}})
	view := s.View()
	if !strings.Contains(view, "mystery: unsupported format") || !strings.Contains(view, "Chat: unsupported format") {
		t.Fatalf("unsupported saved format not identified in UI:\n%s", view)
	}
	if strings.Contains(view, "\x1b[2J") || strings.Contains(view, "path\nforged") {
		t.Fatal("untrusted metadata injected terminal controls or rows")
	}
}

func TestSearchGlobalReloadRefreshesBindingsWithoutMovingRemoteSelection(t *testing.T) {
	s := NewSearch()
	s.SetFleetItems([]*SessionSearchResult{
		localSessionSearchResult(&session.Instance{ID: "local", Title: "orchard", Tool: "claude", ClaudeSessionID: "old"}, "laptop"),
		{Source: SearchSourceHub, SessionID: "hub", Title: "orchard", HubNodeID: "node"},
	})
	s.ShowGlobal()
	s.input.SetValue("orchard")
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyDown})
	ctx, generation := s.chatContext, s.chatGeneration
	s.SetItems([]*session.Instance{{ID: "local", Title: "orchard refreshed", Tool: "claude", ClaudeSessionID: "new"}})
	if ctx.Err() != context.Canceled || s.chatGeneration == generation {
		t.Fatal("global local-binding replacement did not invalidate disk scan")
	}
	if selected := s.Selected(); selected == nil || selected.SessionID != "hub" {
		t.Fatalf("global snapshot refresh moved hub selection: %+v", selected)
	}
	if s.fleetItems[0].Title != "orchard refreshed" || s.fleetItems[0].Host != "laptop" {
		t.Fatalf("global local snapshot/owning host not refreshed: %+v", s.fleetItems[0])
	}
	if s.takeChatSearchCmd() == nil {
		t.Fatal("global binding replacement did not reschedule chat search")
	}
	s.Hide()
}

func TestFleetSearchLeavesRemoteActivityAndOpenedUnknown(t *testing.T) {
	h := newHubProjectionHome(t, nil)
	updated := time.Date(2026, 3, 4, 5, 6, 0, 0, time.UTC)
	h.remoteSessions = map[string][]session.RemoteSessionInfo{
		"remote": {{ID: "ssh", Title: "ssh", Tool: "omp", CreatedAt: "2026-03-04T05:06:00Z"}},
	}
	h.hubConfigured = true
	h.hubSessions = map[string]hub.NodeSessions{
		"other-node": {Node: hub.Node{ID: "other-node", Name: "server"}, Sessions: []hub.SessionInfo{{ID: "hub", Title: "hub", Tool: "codex", UpdatedAt: &updated}}},
	}
	for _, item := range h.fleetSearchResults() {
		if !item.LastOpened.IsZero() || !item.LastActivity.IsZero() {
			t.Fatalf("creation/registry update masqueraded as opened/activity: %+v", item)
		}
	}
}

func TestSearchRenderedChatSnippetKeepsMatchInNarrowViewport(t *testing.T) {
	s := NewSearch()
	s.SetFleetItems([]*SessionSearchResult{{Source: SearchSourceLocal, SessionID: "one", Title: "plain", Tool: "omp"}})
	s.SetSize(48, 20)
	s.ShowGlobal()
	s.input.SetValue("orchard")
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	s.applyChatSearch(searchChatResultsMsg{generation: s.chatGeneration, report: session.ChatSearchReport{Matches: []session.ChatSearchMatch{{
		InstanceID: "one", Snippet: strings.Repeat("before ", 12) + "orchard migration after",
	}}}})
	for _, line := range strings.Split(s.View(), "\n") {
		if strings.Contains(line, "Chat:") && strings.Contains(line, "orchard") {
			s.Hide()
			return
		}
	}
	t.Fatal("narrow viewport clipped the matching query out of the chat snippet")
}

func TestSearchViewPreservesVisibleInputCursor(t *testing.T) {
	forceTrueColorProfile()
	for _, width := range []int{80, 48} {
		t.Run(fmt.Sprintf("width-%d", width), func(t *testing.T) {
			s := NewSearch()
			s.SetSize(width, 24)
			s.ShowGlobal()
			s.input.Cursor.SetMode(cursor.CursorStatic)
			s.input.SetValue("query")
			s.input.SetCursor(2)
			for _, char := range []string{"e", "r"} {
				// The cursor is a reverse-video character, not an additional
				// glyph. Plain-text equality cannot detect its disappearance.
				visibleCursor := "\x1b[7m" + char
				if !strings.Contains(s.input.View(), visibleCursor) {
					t.Fatalf("fixture does not expose a visible cursor on %q", char)
				}
				found := false
				for _, line := range strings.Split(s.View(), "\n") {
					if strings.Contains(ansi.Strip(line), "query") {
						found = strings.Contains(line, visibleCursor)
						break
					}
				}
				if !found {
					t.Fatalf("search overlay stripped the visible cursor on %q", char)
				}
				_, _ = s.Update(tea.KeyMsg{Type: tea.KeyRight})
			}
			s.Hide()
		})
	}
}

func TestSearchInputRenderingDoesNotPassThroughPastedControls(t *testing.T) {
	s := NewSearch()
	s.SetSize(80, 24)
	s.ShowGlobal()
	_, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("before\x1b[2Jafter")})
	view := s.View()
	if strings.Contains(view, "\x1b[2J") {
		t.Fatal("preserving rendered cursor styling also passed through a pasted clear-screen sequence")
	}
	if !strings.Contains(ansi.Strip(view), "before") || !strings.Contains(ansi.Strip(view), "after") {
		t.Fatal("sanitized pasted text disappeared with its controls")
	}
	s.Hide()
}
