package tmux

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestContainsBrailleChar(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty string", "", false},
		{"plain text", "hello world", false},
		{"done marker only", "✳ Working on tests", false},
		{"braille spinner frame", "⠋ Testing something", true},
		{"braille at end", "Testing ⠙", true},
		{"braille in middle", "task ⠹ running", true},
		{"range start U+2800", string(rune(0x2800)), true},
		{"range end U+28FF", string(rune(0x28FF)), true},
		{"just below range U+27FF", string(rune(0x27FF)), false},
		{"just above range U+2900", string(rune(0x2900)), false},
		{"all 10 spinner frames", "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏", true},
		{"unicode non-braille", "✨ Done", false},
		{"mixed braille and done", "⠋ ✳ mixed", true}, // braille takes priority in detection
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsBrailleChar(tt.input)
			if got != tt.want {
				t.Errorf("containsBrailleChar(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestContainsDoneMarker(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty string", "", false},
		{"plain text", "hello world", false},
		{"eight-spoked asterisk ✳", "✳ Worked for 54s", true},
		{"heavy asterisk ✻", "✻ Done", true},
		{"heavy teardrop-spoked asterisk ✽", "✽ Complete", true},
		{"six-pointed star ✶", "✶ Finished", true},
		{"four teardrop-spoked asterisk ✢", "✢ Ready", true},
		{"regular asterisk", "* not a marker", false},
		{"braille char only", "⠋ Testing", false},
		{"marker at end", "Done ✳", true},
		{"marker in middle", "task ✳ status", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsDoneMarker(tt.input)
			if got != tt.want {
				t.Errorf("containsDoneMarker(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAnalyzePaneTitle(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		command string
		want    TitleState
	}{
		// Working state: Braille spinner present
		{"braille spinner + claude", "⠂ Testing Papaorch", "claude", TitleStateWorking},
		{"braille spinner + bash", "⠋ Running tools", "bash", TitleStateWorking}, // bash during tool use
		{"braille spinner only", "⠹", "claude", TitleStateWorking},

		// Done state: done marker present (regardless of current command)
		{"done marker + claude", "✳ Worked for 54s", "claude", TitleStateDone},
		{"heavy asterisk + claude", "✻ Ready", "claude", TitleStateDone},
		{"done + node", "✳ Complete", "node", TitleStateDone},
		{"done marker + bash", "✳ Worked for 54s", "bash", TitleStateDone},
		{"done marker + zsh", "✻ Done", "zsh", TitleStateDone},
		{"done marker + fish", "✽ Complete", "fish", TitleStateDone},

		// Unknown state: no recognized pattern
		{"plain title", "my-session", "claude", TitleStateUnknown},
		{"empty title", "", "claude", TitleStateUnknown},
		{"empty both", "", "", TitleStateUnknown},
		{"regular asterisk", "* task", "bash", TitleStateUnknown},
		{"gemini title", "Gemini CLI", "gemini", TitleStateUnknown},

		// Edge cases
		{"braille + done marker", "⠋ ✳ mixed signals", "claude", TitleStateWorking}, // braille wins
		{"done + version string", "✳ claude v2.1.25", "claude", TitleStateDone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnalyzePaneTitle(tt.title, tt.command)
			if got != tt.want {
				t.Errorf("AnalyzePaneTitle(%q, %q) = %v, want %v", tt.title, tt.command, got, tt.want)
			}
		})
	}
}

// These footer shapes were captured from live Codex panes: a completed turn
// with a reconnect notice, and active work seven lines above the bottom.
const codexReadyStatusPane = `• Reconnected. No input was resent. Review uncertain submissions before retrying; recovered queues remain paused.


› Ask Codex to do anything

  GPT-6.1-Sol high · ~/project · Task · Main [default]
  ← for agents · ? for shortcuts                            ⚠ 1 warning · f2 to view
`

const codexWorkingStatusPane = `• Working (17m 59s • esc to interrupt) · 1 background terminal running · /ps to view · /stop to close
  └ Tip: Paste an image with Ctrl+V to attach it to your next message.


› Ask Codex to do anything

  GPT-6-Astra xhigh · ~/project · Task · Main [default]
  ← for agents · ? for shortcuts                            ⚠ 1 warning · f2 to view
`

// Same native approval fixture as internal/send/codex_approval_test.go.
const codexApprovalStatusPane = `• Running curl https://example.com

  Would you like to run the following command?

  $ curl https://example.com

› 1. Yes, proceed (y)
  2. Yes, and don't ask again for commands that start with ` + "`curl`" + ` (p)
  3. No, and tell Codex what to do differently (esc)
`

// Exercise GetStatus itself using the same caches populated by tmux polling.
// No test pane is launched, attached, or sent input.
func titleStatusSample(t *testing.T, sess *Session, title, command, content string) string {
	t.Helper()
	sessionCacheMu.Lock()
	sessionCacheData = map[string]int64{sess.Name: 1}
	sessionCacheTime = time.Now()
	sessionCacheMu.Unlock()
	t.Cleanup(func() {
		sessionCacheMu.Lock()
		sessionCacheData = nil
		sessionCacheTime = time.Time{}
		sessionCacheMu.Unlock()
	})
	SeedPaneInfoCacheForTest(t, map[string]PaneInfo{
		sess.Name: {Title: title, CurrentCommand: command},
	})
	sess.cacheMu.Lock()
	sess.cacheContent = content
	sess.cacheTime = time.Now()
	sess.cacheMu.Unlock()
	status, err := sess.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	return status
}

func TestGetStatusCodexSpinnerTitleDoesNotOverrideReadyOrApproval(t *testing.T) {
	for _, label := range []string{"Starting", "Ready", "Working"} {
		for _, tc := range []struct {
			name, command, paneCommand, previous, content, want string
		}{
			{"unseen completion", "codex", "codex", "active", codexReadyStatusPane, "waiting"},
			{"acknowledged completion", "codex", "codex", "idle", codexReadyStatusPane, "idle"},
			{"approval", "codex", "codex", "active", codexApprovalStatusPane, "waiting"},
			{"child shell", "codex", "bash", "idle", codexReadyStatusPane, "idle"},
		} {
			t.Run(label+"/"+tc.name, func(t *testing.T) {
				sess := ReconnectSessionLazy("title-status-codex", "test", "/tmp", tc.command, tc.previous)
				sess.SocketName = DefaultSocketName()
				title := "⠏ project | " + label + " | Task"
				if got := titleStatusSample(t, sess, title, tc.paneCommand, tc.content); got != tc.want {
					t.Fatalf("GetStatus() = %q, want %q from pane, not native title %q", got, tc.want, title)
				}
				if info, _ := GetCachedPaneInfo(sess.Name); info.Title != title {
					t.Fatal("status classification must not rewrite the native pane title")
				}
			})
		}
	}
}

func TestGetStatusCodexSpinnerTitleAllowsWorkingToWaitingTransition(t *testing.T) {
	for _, completion := range []struct{ name, content string }{
		{"completed turn", codexReadyStatusPane},
		{"approval", codexApprovalStatusPane},
	} {
		t.Run(completion.name, func(t *testing.T) {
			sess := ReconnectSessionLazy("title-status-transition", "test", "/tmp", "codex", "idle")
			sess.SocketName = DefaultSocketName()
			const title = "⠏ project | Starting | Task"
			if got := titleStatusSample(t, sess, title, "codex", codexReadyStatusPane); got != "idle" {
				t.Fatalf("initial ready pane = %q, want idle", got)
			}
			if got := titleStatusSample(t, sess, title, "codex", codexWorkingStatusPane); got != "active" {
				t.Fatalf("native busy line above composer = %q, want active", got)
			}
			// Advance only the existing busy grace/cooldown clocks. Keep the
			// real activity and prompt-confirmation state for the transition.
			sess.mu.Lock()
			sess.stateTracker.spinnerTracker.lastBusyTime = time.Now().Add(-time.Minute)
			sess.stateTracker.lastChangeTime = time.Now().Add(-time.Minute)
			sess.mu.Unlock()
			for sample, want := range []string{"active", "active", "waiting"} {
				if got := titleStatusSample(t, sess, title, "codex", completion.content); got != want {
					t.Fatalf("completion sample %d = %q, want %q", sample+1, got, want)
				}
			}
			sess.Acknowledge()
			if got := titleStatusSample(t, sess, title, "codex", completion.content); got != "idle" {
				t.Fatalf("acknowledged completion = %q, want idle", got)
			}
		})
	}
}

func TestGetStatusCodexHistoricalBusyLineDoesNotOverrideCurrentPrompt(t *testing.T) {
	for _, history := range []struct{ name, content string }{
		{"completed busy line", "• Working (17m 59s • esc to interrupt)\n\n• Finished the requested work.\n\n"},
		{"quoted busy line", "  The status used to say: • Working (17m 59s • esc to interrupt)\n\n"},
		{"fenced busy line", "```\n• Working (17m 59s • esc to interrupt)\n```\n\n"},
		{"complete busy pane", codexWorkingStatusPane + "\n"},
		{"fenced complete busy pane", "```\n" + codexWorkingStatusPane + "```\n\n"},
	} {
		for _, current := range []struct{ name, content, previous, want string }{
			{"ready", codexReadyStatusPane, "idle", "idle"},
			{"approval", codexApprovalStatusPane, "active", "waiting"},
		} {
			t.Run(history.name+"/"+current.name, func(t *testing.T) {
				sess := ReconnectSessionLazy("title-status-history", "test", "/tmp", "codex", current.previous)
				sess.SocketName = DefaultSocketName()
				content := history.content + current.content
				if got := titleStatusSample(t, sess, "⠏ project | Working | Task", "codex", content); got != current.want {
					t.Fatalf("historical busy pane made current prompt %q, want %q", got, current.want)
				}
			})
		}
	}
}

func TestGetStatusClaudeSpinnerTitleKeepsFastPathDuringChildCommands(t *testing.T) {
	for _, command := range []string{"claude", "bash", "codex"} {
		t.Run(command, func(t *testing.T) {
			sess := ReconnectSessionLazy("title-status-claude", "test", "/tmp", "claude", "idle")
			sess.SocketName = DefaultSocketName()
			if got := titleStatusSample(t, sess, "⠋ Running tools", command, "❯ \n"); got != "active" {
				t.Fatalf("Claude title with foreground %q = %q, want active", command, got)
			}
		})
	}
}

func TestRefreshPaneInfoCache(t *testing.T) {
	skipIfNoTmuxServer(t)

	// Create a test session to ensure there's at least one pane
	sessName := SessionPrefix + "title_detection_test"
	cmd := exec.Command("tmux", "new-session", "-d", "-s", sessName)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}
	defer func() {
		_ = exec.Command("tmux", "kill-session", "-t", sessName).Run()
	}()

	// Refresh the cache
	RefreshPaneInfoCache()

	// Verify cache was populated
	info, ok := GetCachedPaneInfo(sessName)
	if !ok {
		t.Fatal("Expected to find test session in pane cache")
	}

	// The title might be empty or the session name; command should be a shell
	if info.CurrentCommand == "" {
		t.Error("Expected non-empty CurrentCommand for test session")
	}

	// Verify a non-existent session is not in cache
	_, ok = GetCachedPaneInfo("nonexistent_session_xyz")
	if ok {
		t.Error("Expected nonexistent session to not be in cache")
	}
}

func TestGetCachedPaneInfo_StaleCache(t *testing.T) {
	// Set cache data with a time far in the past (stale)
	paneCacheMu.Lock()
	paneCacheData = map[string]PaneInfo{
		"test_session": {Title: "✳ Done", CurrentCommand: "claude"},
	}
	paneCacheTime = time.Now().Add(-10 * time.Second) // 10 seconds ago (stale, threshold is 4s)
	paneCacheMu.Unlock()

	_, ok := GetCachedPaneInfo("test_session")
	if ok {
		t.Error("Expected stale cache to return false")
	}
}

func TestGetCachedWindows(t *testing.T) {
	// Seed window cache
	windowCacheMu.Lock()
	windowCacheData = map[string][]WindowInfo{
		"agentdeck_test_abc12345": {
			{Index: 0, Name: "main", Activity: 100},
			{Index: 1, Name: "tests", Activity: 200},
		},
	}
	windowCacheTime = time.Now()
	windowCacheMu.Unlock()

	// Seed tool cache — window 1 has claude
	windowToolCacheMu.Lock()
	windowToolCacheData = map[string]map[int]string{
		"agentdeck_test_abc12345": {1: "claude"},
	}
	windowToolCacheMu.Unlock()

	// Hit with tool merge
	wins := GetCachedWindows("agentdeck_test_abc12345")
	assert.Len(t, wins, 2)
	assert.Equal(t, "main", wins[0].Name)
	assert.Equal(t, "", wins[0].Tool)
	assert.Equal(t, 1, wins[1].Index)
	assert.Equal(t, "claude", wins[1].Tool)

	// Miss
	wins = GetCachedWindows("nonexistent")
	assert.Nil(t, wins)

	// Expired cache
	windowCacheMu.Lock()
	windowCacheTime = time.Now().Add(-5 * time.Second)
	windowCacheMu.Unlock()
	wins = GetCachedWindows("agentdeck_test_abc12345")
	assert.Nil(t, wins)

	// Cleanup
	windowToolCacheMu.Lock()
	windowToolCacheData = nil
	windowToolCacheMu.Unlock()
}

func TestGetCachedWindows_ReturnsCopy(t *testing.T) {
	windowCacheMu.Lock()
	windowCacheData = map[string][]WindowInfo{
		"sess": {{Index: 0, Name: "orig", Activity: 100}},
	}
	windowCacheTime = time.Now()
	windowCacheMu.Unlock()

	windowToolCacheMu.Lock()
	windowToolCacheData = nil
	windowToolCacheMu.Unlock()

	wins := GetCachedWindows("sess")
	assert.Len(t, wins, 1)

	// Mutate the returned slice
	wins[0].Name = "mutated"

	// Original cache must be unchanged
	windowCacheMu.RLock()
	assert.Equal(t, "orig", windowCacheData["sess"][0].Name)
	windowCacheMu.RUnlock()
}

func TestGetCachedWindows_ToolMergePartial(t *testing.T) {
	windowCacheMu.Lock()
	windowCacheData = map[string][]WindowInfo{
		"s1": {
			{Index: 0, Name: "a"},
			{Index: 1, Name: "b"},
			{Index: 2, Name: "c"},
		},
	}
	windowCacheTime = time.Now()
	windowCacheMu.Unlock()

	// Only window 2 has a tool
	windowToolCacheMu.Lock()
	windowToolCacheData = map[string]map[int]string{
		"s1": {2: "gemini"},
	}
	windowToolCacheMu.Unlock()

	wins := GetCachedWindows("s1")
	assert.Len(t, wins, 3)
	assert.Equal(t, "", wins[0].Tool)
	assert.Equal(t, "", wins[1].Tool)
	assert.Equal(t, "gemini", wins[2].Tool)

	// Cleanup
	windowToolCacheMu.Lock()
	windowToolCacheData = nil
	windowToolCacheMu.Unlock()
}

func TestUpdateWindowToolCache(t *testing.T) {
	// First write
	data1 := map[string]map[int]string{
		"s1": {0: "claude", 1: "gemini"},
	}
	updateWindowToolCache(data1)

	windowToolCacheMu.RLock()
	assert.Equal(t, "claude", windowToolCacheData["s1"][0])
	assert.Equal(t, "gemini", windowToolCacheData["s1"][1])
	windowToolCacheMu.RUnlock()

	// Full replacement — s1 data is gone
	data2 := map[string]map[int]string{
		"s2": {0: "aider"},
	}
	updateWindowToolCache(data2)

	windowToolCacheMu.RLock()
	assert.Nil(t, windowToolCacheData["s1"])
	assert.Equal(t, "aider", windowToolCacheData["s2"][0])
	windowToolCacheMu.RUnlock()

	// Cleanup
	windowToolCacheMu.Lock()
	windowToolCacheData = nil
	windowToolCacheMu.Unlock()
}
