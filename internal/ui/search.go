package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/asheshgoplani/agent-deck/internal/session"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

var (
	overlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(1, 2)
)

// SearchScope controls which known Agent Deck sessions appear in the search
// overlay. Global is the fleet snapshot (local + SSH remotes + hub nodes),
// while Local contains only sessions owned by this machine.
type SearchScope string

const (
	SearchScopeGlobal SearchScope = "global"
	SearchScopeLocal  SearchScope = "local"
)

// SessionSearchSource identifies the activation path for a search result.
type SessionSearchSource string

const (
	SearchSourceLocal  SessionSearchSource = "local"
	SearchSourceRemote SessionSearchSource = "remote"
	SearchSourceHub    SessionSearchSource = "hub"
)

// SessionSearchResult is a searchable, fleet-aware session identity. It keeps
// stable IDs instead of retaining pointers into reloadable session snapshots.
type SessionSearchResult struct {
	Source       SessionSearchSource
	SessionID    string
	Title        string
	ProjectPath  string
	GroupPath    string
	Tool         string
	Status       session.Status
	Host         string
	LastOpened   time.Time
	LastActivity time.Time
	Chat         session.ChatSearchBinding
	Snippet      string

	RemoteName string
	HubNodeID  string
}

func localSessionSearchResult(inst *session.Instance, host string) *SessionSearchResult {
	if inst == nil {
		return nil
	}
	activity, _ := inst.LastObservedActivity()
	if persisted := inst.LastActivityAt(); persisted.After(activity) {
		activity = persisted
	}
	return &SessionSearchResult{
		Source:       SearchSourceLocal,
		SessionID:    inst.ID,
		Title:        inst.GetTitleThreadSafe(),
		ProjectPath:  inst.ProjectPath,
		GroupPath:    inst.GroupPath,
		Tool:         inst.GetToolThreadSafe(),
		Status:       inst.GetStatusThreadSafe(),
		Host:         host,
		LastOpened:   inst.LastAccessedAt,
		LastActivity: activity,
		Chat:         session.NewChatSearchBinding(inst),
	}
}

// Search represents the search overlay component
type Search struct {
	input          textinput.Model
	results        []*SessionSearchResult
	cursor         int
	width          int
	height         int
	visible        bool
	scope          SearchScope
	allItems       []*SessionSearchResult
	localItems     []*SessionSearchResult
	fleetItems     []*SessionSearchResult
	scopedGroup    string // Non-empty => filter items to this exact GroupPath (v1.7.60)
	chatEnabled    bool
	chatGeneration uint64
	chatContext    context.Context
	chatCancel     context.CancelFunc
	chatPending    bool
	chatLoading    bool
	chatReport     session.ChatSearchReport
}

// NewSearch creates a new search overlay
func NewSearch() *Search {
	ti := textinput.New()
	ti.Placeholder = "Search sessions..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 50

	return &Search{
		input:   ti,
		results: []*SessionSearchResult{},
		cursor:  0,
		visible: false,
		scope:   SearchScopeLocal,
	}
}

// SetItems sets the full list of items to search through. When a group scope
// has been set via SetScopedGroup, items are filtered to that group before
// storage so background reloads do not leak out-of-group sessions into a
// scoped in-group search session.
func (s *Search) SetItems(items []*session.Instance) {
	selected := s.Selected()
	hosts := make(map[string]string, len(s.fleetItems))
	defaultHost := ""
	remoteCount := 0
	for _, item := range s.fleetItems {
		if item != nil && item.Source == SearchSourceLocal {
			hosts[item.SessionID] = item.Host
			if item.Host != "" {
				defaultHost = item.Host
			}
		} else if item != nil {
			remoteCount++
		}
	}
	localItems := make([]*SessionSearchResult, 0, len(items))
	for _, inst := range items {
		if result := localSessionSearchResult(inst, ""); result != nil {
			result.Host = hosts[result.SessionID]
			if result.Host == "" {
				result.Host = defaultHost
			}
			localItems = append(localItems, result)
		}
	}
	if s.scopedGroup != "" {
		filtered := make([]*SessionSearchResult, 0, len(localItems))
		for _, it := range localItems {
			if it.GroupPath == s.scopedGroup {
				filtered = append(filtered, it)
			}
		}
		localItems = filtered
	}
	s.localItems = localItems
	// Refresh local identities even while the fleet scope is visible. A /new
	// binding or deleted row must invalidate an in-flight chat scan; keep the
	// remote snapshots and the user's selected stable identity in place.
	fleetItems := make([]*SessionSearchResult, 0, len(localItems)+remoteCount)
	fleetItems = append(fleetItems, localItems...)
	for _, item := range s.fleetItems {
		if item != nil && item.Source != SearchSourceLocal {
			fleetItems = append(fleetItems, item)
		}
	}
	s.fleetItems = fleetItems
	if s.scope == SearchScopeGlobal {
		s.allItems = s.fleetItems
	} else {
		s.allItems = s.localItems
	}
	s.updateResults()
	s.restoreSelection(selected)
}

// SetFleetItems replaces the in-memory fleet snapshot. It also derives the
// Local pool from the same rows so Tab never needs a disk or network refresh.
func (s *Search) SetFleetItems(items []*SessionSearchResult) {
	s.fleetItems = append(s.fleetItems[:0], items...)
	s.localItems = s.localItems[:0]
	for _, item := range s.fleetItems {
		if item != nil && item.Source == SearchSourceLocal {
			s.localItems = append(s.localItems, item)
		}
	}
	if s.scope == SearchScopeGlobal {
		s.allItems = s.fleetItems
	} else {
		s.allItems = s.localItems
	}
	s.updateResults()
}

// SetScopedGroup restricts SetItems to a single group path. Pass "" to clear.
func (s *Search) SetScopedGroup(groupPath string) {
	s.scopedGroup = groupPath
}

// SetSize sets the dimensions of the search overlay
func (s *Search) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.input.Width = max(1, s.contentWidth()-6)
}

// Show preserves the historical local-search behavior for callers such as
// Alt+/. Ordinary / uses ShowGlobal explicitly.
func (s *Search) Show() {
	s.ShowLocal()
}

// ShowGlobal opens search over the complete fleet snapshot.
func (s *Search) ShowGlobal() {
	s.show(SearchScopeGlobal)
}

// ShowLocal opens search over sessions owned by this machine.
func (s *Search) ShowLocal() {
	s.show(SearchScopeLocal)
}

func (s *Search) show(scope SearchScope) {
	s.chatEnabled = false
	s.scope = scope
	if scope == SearchScopeGlobal {
		s.allItems = s.fleetItems
	} else {
		s.allItems = s.localItems
	}
	s.visible = true
	s.input.Focus()
	s.input.SetValue("")
	s.updateResults()
}

// Scope returns the active search scope.
func (s *Search) Scope() SearchScope {
	return s.scope
}

// Hide hides the search overlay and clears any group scope.
func (s *Search) Hide() {
	s.visible = false
	s.input.Blur()
	s.input.SetValue("")
	s.updateResults()
	s.scopedGroup = ""
}

// IsVisible returns whether the search overlay is visible
func (s *Search) IsVisible() bool {
	return s.visible
}

// Selected returns the currently selected item
func (s *Search) Selected() *SessionSearchResult {
	if len(s.results) == 0 {
		return nil
	}
	if s.cursor >= len(s.results) {
		s.cursor = len(s.results) - 1
	}
	return s.results[s.cursor]
}

func (s *Search) restoreSelection(selected *SessionSearchResult) {
	if selected == nil {
		return
	}
	for i, item := range s.results {
		if item.Source == selected.Source && item.SessionID == selected.SessionID && item.RemoteName == selected.RemoteName && item.HubNodeID == selected.HubNodeID {
			s.cursor = i
			return
		}
	}
}

// Update handles messages for the search overlay
// Returns the updated Search and any command to execute
func (s *Search) Update(msg tea.Msg) (*Search, tea.Cmd) {
	if !s.visible {
		return s, nil
	}

	switch msg := msg.(type) {
	case searchChatDebounceMsg:
		return s, s.runChatSearch(msg.generation)
	case searchChatResultsMsg:
		s.applyChatSearch(msg)
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			s.Hide()
			return s, nil

		case "enter":
			if len(s.results) > 0 {
				s.Hide()
				// Parent should handle the selection
			}
			return s, nil

		case "up", "ctrl+k":
			if s.cursor > 0 {
				s.cursor--
			}
			return s, nil

		case "down", "ctrl+j":
			if s.cursor < len(s.results)-1 {
				s.cursor++
			}
			return s, nil

		case "ctrl+s":
			s.chatEnabled = !s.chatEnabled
			s.updateResults()
			return s, s.takeChatSearchCmd()

		case "tab":
			// Alt+/ is intentionally group-local; ordinary fleet search toggles
			// between the complete snapshot and this machine's rows.
			if s.scopedGroup != "" {
				return s, nil
			}
			if s.scope == SearchScopeGlobal {
				s.scope = SearchScopeLocal
				s.allItems = s.localItems
			} else {
				s.scope = SearchScopeGlobal
				s.allItems = s.fleetItems
			}
			s.updateResults()
			return s, s.takeChatSearchCmd()

		default:
			before := s.input.Value()
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			if before != s.input.Value() {
				s.updateResults()
			}
			return s, tea.Batch(cmd, s.takeChatSearchCmd())
		}
	}

	return s, nil
}

// updateResults filters the items based on the current input
func (s *Search) updateResults() {
	s.cancelChatSearch()
	s.chatReport = session.ChatSearchReport{}
	s.results = filterSessionSearchResults(s.allItems, s.input.Value())
	s.cursor = 0
	s.chatPending = s.visible && s.chatEnabled && strings.TrimSpace(s.input.Value()) != ""
}

func filterSessionSearchResults(items []*SessionSearchResult, query string) []*SessionSearchResult {
	if strings.TrimSpace(query) == "" {
		results := make([]*SessionSearchResult, 0, len(items))
		for _, item := range items {
			if item != nil {
				results = append(results, item)
			}
		}
		sort.SliceStable(results, func(i, j int) bool { return results[i].LastOpened.After(results[j].LastOpened) })
		return results
	}

	// Reuse the established local title/path/tool/status and fuzzy semantics by
	// adapting each fleet result to the same search surface. Host and group are
	// appended to ProjectPath so they participate without changing session's
	// generic query API.
	proxies := make([]*session.Instance, 0, len(items))
	byProxy := make(map[*session.Instance]*SessionSearchResult, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		proxy := &session.Instance{
			Title:       item.Title,
			ProjectPath: strings.Join([]string{item.ProjectPath, item.GroupPath, item.Host}, " "),
			Tool:        item.Tool,
			Status:      item.Status,
		}
		proxies = append(proxies, proxy)
		byProxy[proxy] = item
	}

	matches := session.FilterByQuery(proxies, query)
	results := make([]*SessionSearchResult, 0, len(matches))
	for _, match := range matches {
		if item := byProxy[match]; item != nil {
			results = append(results, item)
		}
	}
	sortSearchResults(results, query)
	return results
}

// View renders the search overlay, with row density determined by the actual
// terminal height. No metadata or transcript text is allowed to wrap the box.
func (s *Search) View() string {
	if !s.visible {
		return ""
	}
	width := s.contentWidth()
	header := "Global Search (all Agent Deck sessions)"
	if s.scope == SearchScopeLocal {
		header = "Local Search (this machine)"
	}
	if s.scopedGroup != "" {
		header = "Group Search: " + s.scopedGroup
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render(searchLine(header, width)),
		// textinput sanitizes its value before rendering. Preserve its trusted
		// cursor SGR here; searchLine strips styling from untrusted metadata.
		ansi.Truncate(s.input.View(), width, "…"),
		searchLine(s.chatStatus(), width),
		searchLine("Formats: "+session.ChatSearchSupportedTools, width),
	}
	if unavailable := s.unavailableChatStatus(); unavailable != "" {
		lines = append(lines, searchLine(unavailable, width))
	}

	// Reserve header, footer and border space; compact terminals show the
	// selected row at reduced density rather than overflowing.
	available := 24
	if s.height > 0 {
		available = max(0, s.height-len(lines)-4)
	}
	rowHeight := 5
	if width >= 88 {
		rowHeight = 4
	}
	if s.chatEnabled {
		rowHeight++
	}
	maxResults := min(10, available/rowHeight)
	if maxResults == 0 && available > 0 {
		maxResults = 1
	}
	start := max(0, s.cursor-maxResults+1)
	end := min(start+maxResults, len(s.results))
	for index := start; index < end; index++ {
		item := s.results[index]
		label := item.Title + " (" + item.Tool + ")"
		if item.Host != "" && s.scope == SearchScopeGlobal {
			label += " · " + item.Host
		}
		prefix := "  "
		if index == s.cursor {
			prefix = "› "
		}
		status := string(item.Status)
		if status == "" {
			status = "unknown"
		}
		suffix := searchLine(" ["+status+"]", width)
		row := []string{
			prefix + searchLine(label, max(0, width-lipgloss.Width(prefix+suffix))) + suffix,
			"  Path: " + searchValue(item.ProjectPath),
			"  Group: " + searchValue(item.GroupPath),
		}
		opened := "Opened: " + searchTimestamp(item.LastOpened)
		activity := "Activity: " + searchTimestamp(item.LastActivity)
		if width >= 88 {
			row = append(row, "  "+opened+" · "+activity)
		} else {
			row = append(row, "  "+opened, "  "+activity)
		}
		if s.chatEnabled {
			snippet := "  Chat: " + s.resultChatStatus(item)
			if item.Snippet != "" {
				snippet = "  Chat: " + searchSnippet(item.Snippet, s.input.Value(), max(1, width-8))
			}
			row = append(row, snippet)
		}
		for _, line := range row[:min(len(row), available)] {
			line = searchLine(line, width)
			if index == s.cursor {
				line = lipgloss.NewStyle().Background(ColorAccent).Foreground(ColorBg).Render(line)
			}
			lines = append(lines, line)
		}
		available -= min(len(row), available)
	}
	lines = append(lines, searchLine(formatCount(len(s.results)), width))
	keys := "[Enter] Open  [↑↓] Navigate  [Esc] Cancel"
	if s.scopedGroup == "" {
		nextScope := "Local"
		if s.scope == SearchScopeLocal {
			nextScope = "Global"
		}
		keys = "[Enter] Open  [↑↓] Navigate  [Tab] " + nextScope + "  [Esc] Cancel"
	}
	lines = append(lines, searchLine("[Ctrl+S] Search chats  "+keys, width))

	style := overlayStyle.Padding(0, 1).Width(width + 2)
	// The smallest terminals cannot fit all chrome. Clip complete, already
	// cell-truncated lines, preserving the header, input and current status.
	if s.height > 0 {
		lines = lines[:min(len(lines), max(0, s.height-2))]
	}
	if s.width > 0 && s.width < 6 {
		style = lipgloss.NewStyle().Width(max(1, s.width))
	}
	overlay := style.Render(strings.Join(lines, "\n"))
	return strings.TrimSuffix(centerInScreen(overlay, s.width, s.height), "\n")
}

func (s *Search) contentWidth() int {
	if s.width <= 0 {
		return 94
	}
	return max(1, min(110, s.width-6))
}

func searchLine(value string, width int) string {
	value = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, ansi.Strip(value))
	return ansi.Truncate(value, width, "…")
}

func searchValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func searchTimestamp(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.Local().Format("2006-01-02 15:04")
}

// Recenter the already-bounded excerpt when a narrower viewport would cut the
// actual match off its right edge. Lowercasing preserves rune counts, not byte
// offsets, so map the match back by rune index.
func searchSnippet(snippet, query string, width int) string {
	lowered := strings.ToLower(snippet)
	query = strings.ToLower(strings.TrimSpace(query))
	match := strings.Index(lowered, query)
	if match <= 0 || query == "" {
		return snippet
	}
	queryWidth := lipgloss.Width(query)
	if lipgloss.Width(lowered[:match])+queryWidth <= width {
		return snippet
	}
	runes := []rune(snippet)
	start := utf8.RuneCountInString(lowered[:match])
	budget := max(0, (width-queryWidth-1)/3)
	used := 0
	for start > 0 {
		cells := lipgloss.Width(string(runes[start-1]))
		if used+cells > budget {
			break
		}
		start--
		used += cells
	}
	if start == 0 {
		return snippet
	}
	return "…" + string(runes[start:])
}

func sortSearchResults(results []*SessionSearchResult, query string) {
	type ranked struct {
		item  *SessionSearchResult
		score int
	}
	ranks := make([]ranked, len(results))
	for i, item := range results {
		ranks[i] = ranked{item: item, score: searchRelevance(item, query)}
	}
	sort.SliceStable(ranks, func(i, j int) bool {
		if ranks[i].score != ranks[j].score {
			return ranks[i].score > ranks[j].score
		}
		return ranks[i].item.LastOpened.After(ranks[j].item.LastOpened)
	})
	for i, rank := range ranks {
		results[i] = rank.item
	}
}

// Membership still comes from session.FilterByQuery (including its exact
// status filters and fuzzy fallback). Rank direct metadata matches ahead of
// chat-only hits; only equal relevance is broken by last opened.
func searchRelevance(item *SessionSearchResult, query string) int {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" || searchStatusQuery(query) {
		return 0
	}
	fields := []struct {
		value string
		bonus int
	}{
		{item.Title, 5000},
		{item.Tool, 4000},
		{item.ProjectPath, 3000},
		{item.GroupPath, 2000},
		{item.Host, 1000},
	}
	best := 0
	for _, field := range fields {
		value := strings.ToLower(strings.TrimSpace(field.value))
		score := 0
		if value == query {
			score = 30000 + field.bonus
		} else if strings.HasPrefix(value, query) {
			score = 20000 + field.bonus
		} else if strings.Contains(value, query) {
			score = 10000 + field.bonus
		} else if matches := fuzzy.Find(query, []string{value}); len(matches) > 0 {
			score = field.bonus + matches[0].Score
		}
		best = max(best, score)
	}
	return best
}

func searchStatusQuery(query string) bool {
	switch strings.ToLower(strings.TrimSpace(query)) {
	case "waiting", "running", "idle", "error", "stopped":
		return true
	default:
		return false
	}
}

// formatCount formats the result count
func formatCount(count int) string {
	if count == 0 {
		return "No results"
	}
	if count == 1 {
		return "1 result"
	}
	return lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%d", count)) + " results"
}

// centerInScreen centers content in the terminal
func centerInScreen(content string, screenWidth, screenHeight int) string {
	lines := strings.Split(content, "\n")
	contentHeight := len(lines)
	contentWidth := 0
	for _, line := range lines {
		// cellWidth (not len): the box lines carry ANSI styling, so byte length
		// over-counts and the dialog ends up shifted left of true center.
		if w := cellWidth(line); w > contentWidth {
			contentWidth = w
		}
	}

	// Calculate vertical padding
	verticalPad := (screenHeight - contentHeight) / 2
	if verticalPad < 0 {
		verticalPad = 0
	}

	// Calculate horizontal padding
	horizontalPad := (screenWidth - contentWidth) / 2
	if horizontalPad < 0 {
		horizontalPad = 0
	}

	// Add vertical padding
	var result strings.Builder
	for i := 0; i < verticalPad; i++ {
		result.WriteString("\n")
	}

	// Add horizontal padding and content
	padding := strings.Repeat(" ", horizontalPad)
	for _, line := range lines {
		result.WriteString(padding + line + "\n")
	}

	return result.String()
}
