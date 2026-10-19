package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/session"
	tea "github.com/charmbracelet/bubbletea"
)

// Separate from Tab's fleet/local scope. A generation covers one opening,
// query, scope and snapshot so a completed canceled scan cannot leak back in.
type searchChatDebounceMsg struct {
	generation uint64
}

type searchChatResultsMsg struct {
	generation uint64
	report     session.ChatSearchReport
}

func (s *Search) cancelChatSearch() {
	if s.chatCancel != nil {
		s.chatCancel()
		s.chatCancel = nil
	}
	s.chatGeneration++
	s.chatLoading = false
	s.chatPending = false
}

// takeChatSearchCmd also runs after Home's background snapshot updates. SetItems
// can invalidate a local query without needing an asynchronous setter API.
func (s *Search) takeChatSearchCmd() tea.Cmd {
	if !s.chatPending || !s.visible || !s.chatEnabled {
		return nil
	}
	s.chatPending = false
	s.chatLoading = true
	s.chatContext, s.chatCancel = context.WithCancel(context.Background())
	generation := s.chatGeneration
	return tea.Tick(180*time.Millisecond, func(time.Time) tea.Msg {
		return searchChatDebounceMsg{generation: generation}
	})
}

func (s *Search) runChatSearch(generation uint64) tea.Cmd {
	if generation != s.chatGeneration || !s.visible || !s.chatEnabled || s.chatContext == nil || s.chatContext.Err() != nil {
		return nil
	}
	ctx := s.chatContext
	query := strings.TrimSpace(s.input.Value())
	bindings := make([]session.ChatSearchBinding, 0, len(s.allItems))
	for _, item := range s.allItems {
		// The fleet scope includes remote metadata, never remote transcripts.
		// Even a mistakenly attached binding on a remote row is not consulted.
		if item != nil && item.Source == SearchSourceLocal {
			bindings = append(bindings, item.Chat)
		}
	}
	return func() tea.Msg {
		report := session.SearchLocalChats(ctx, bindings, query)
		return searchChatResultsMsg{generation: generation, report: report}
	}
}

func (s *Search) applyChatSearch(msg searchChatResultsMsg) {
	if msg.generation != s.chatGeneration || !s.visible || !s.chatEnabled {
		return
	}
	s.chatLoading = false
	s.chatReport = msg.report
	matches := make(map[string]string, len(msg.report.Matches))
	for _, match := range msg.report.Matches {
		matches[match.InstanceID] = match.Snippet
	}
	results := filterSessionSearchResults(s.allItems, s.input.Value())
	byID := make(map[string]bool, len(results))
	merged := make([]*SessionSearchResult, 0, len(results)+len(matches))
	for _, item := range results {
		copy := *item
		if item.Source == SearchSourceLocal {
			copy.Snippet = matches[item.SessionID]
			byID[item.SessionID] = true
		}
		merged = append(merged, &copy)
	}
	// Status keywords retain their established filtering semantics, even with
	// Search chats enabled; a chat mentioning "running" is not a running row.
	if !searchStatusQuery(s.input.Value()) {
		for _, item := range s.allItems {
			if item == nil || item.Source != SearchSourceLocal || byID[item.SessionID] {
				continue
			}
			if snippet, found := matches[item.SessionID]; found {
				copy := *item
				copy.Snippet = snippet
				merged = append(merged, &copy)
				byID[item.SessionID] = true
			}
		}
	}
	selected := s.Selected()
	// Metadata is already relevance/last-opened sorted by the shared filter.
	// Chat-only matches follow it, with last-opened breaking their equal rank.
	chatResults := merged[len(results):]
	sort.SliceStable(chatResults, func(i, j int) bool {
		return chatResults[i].LastOpened.After(chatResults[j].LastOpened)
	})
	s.results = merged
	s.cursor = 0
	s.restoreSelection(selected)
}

func (s *Search) chatStatus() string {
	if !s.chatEnabled {
		return "Search chats: OFF · Ctrl+S to include saved local chats"
	}
	if s.chatLoading {
		return "Search chats: ON · searching local files… · no remote content"
	}
	if strings.TrimSpace(s.input.Value()) == "" {
		return "Search chats: ON · type a query · local files only"
	}
	status := "Search chats: ON · local files only"
	if s.chatReport.Partial {
		status += " · partial results (some content skipped)"
	}
	return status
}

func (s *Search) unavailableChatStatus() string {
	if !s.chatEnabled {
		return ""
	}
	if len(s.chatReport.Unavailable) > 0 {
		reasons := make(map[string]bool)
		for _, item := range s.chatReport.Unavailable {
			tool := item.Tool
			if tool == "" {
				tool = "unknown"
			}
			reasons[tool+": "+item.Reason] = true
		}
		labels := make([]string, 0, len(reasons))
		for reason := range reasons {
			labels = append(labels, reason)
		}
		sort.Strings(labels)
		return fmt.Sprintf("Unavailable chats (%d): %s", len(s.chatReport.Unavailable), strings.Join(labels, "; "))
	}
	return ""
}

func (s *Search) resultChatStatus(item *SessionSearchResult) string {
	if item.Source != SearchSourceLocal {
		return "remote content not searched"
	}
	for _, unavailable := range s.chatReport.Unavailable {
		if unavailable.InstanceID == item.SessionID {
			return unavailable.Reason
		}
	}
	if s.chatLoading {
		return "searching…"
	}
	if strings.TrimSpace(s.input.Value()) == "" {
		return "type a query"
	}
	return "no match in scanned content"
}
