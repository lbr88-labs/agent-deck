package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/asheshgoplani/agent-deck/internal/costs"
	"github.com/asheshgoplani/agent-deck/internal/feedback"
	"github.com/asheshgoplani/agent-deck/internal/git"
	"github.com/asheshgoplani/agent-deck/internal/logging"
	"github.com/asheshgoplani/agent-deck/internal/session"
	"github.com/asheshgoplani/agent-deck/internal/statedb"
	"github.com/asheshgoplani/agent-deck/internal/tmux"
	"github.com/asheshgoplani/agent-deck/internal/ui"
	"github.com/asheshgoplani/agent-deck/internal/update"
	"github.com/asheshgoplani/agent-deck/internal/vcs"
	"github.com/asheshgoplani/agent-deck/internal/web"
)

var Version = "1.9.66" // overridden at build time via -ldflags "-X main.Version=..."

// Table column widths for list command output
const (
	tableColTitle     = 20
	tableColGroup     = 15
	tableColPath      = 40
	tableColIDDisplay = 12
)

// init sets up color profile for consistent terminal colors across environments
func init() {
	initColorProfile()
	initUpdateSettings()
}

// initUpdateSettings configures update checking from user config
func initUpdateSettings() {
	settings := session.GetUpdateSettings()
	update.SetCheckInterval(settings.CheckIntervalHours)
	update.SetBridgeScriptInstaller(session.InstallBridgeScript)
	update.SetConductorDirResolver(session.ConductorDir)
}

// writeVersionOutput prints `Agent Deck vX.Y.Z` to `w`, appending
// ` (update available: vA.B.C)` when the on-disk cache says the user
// is behind. Offline — never touches the network. Conductor task #45.
func writeVersionOutput(w io.Writer, currentVersion string) {
	fmt.Fprintf(w, "Agent Deck v%s", currentVersion)
	info, err := update.CachedUpdateInfo(currentVersion)
	if err == nil && info != nil && info.Available {
		fmt.Fprintf(w, " (update available: v%s)", info.LatestVersion)
	}
	fmt.Fprintln(w)
}

// printUpdateNotice checks for updates and prints a one-liner if available
// Uses cache to avoid API calls - only prints if update was already detected
func printUpdateNotice() {
	settings := session.GetUpdateSettings()
	if !settings.GetCheckEnabled() || !settings.GetNotifyInCLI() {
		return
	}

	info, err := update.CheckForUpdate(Version, false)
	if err != nil || info == nil || !info.Available {
		return
	}

	// Print update notice to stderr so it doesn't interfere with JSON output
	fmt.Fprintf(os.Stderr, "\n💡 Update available: v%s → v%s (run: agent-deck update)\n",
		info.CurrentVersion, info.LatestVersion)
}

// promptForUpdate checks for updates and prompts user if auto_update is enabled
func promptForUpdate() bool {
	settings := session.GetUpdateSettings()
	if !settings.GetCheckEnabled() {
		return false
	}

	info, err := update.CheckForUpdate(Version, false)
	if err != nil || info == nil || !info.Available {
		return false
	}

	// If auto_update is disabled, just show notification (don't prompt)
	if !settings.AutoUpdate {
		fmt.Fprintf(os.Stderr, "\n💡 Update available: v%s → v%s (run: agent-deck update)\n",
			info.CurrentVersion, info.LatestVersion)
		return false
	}

	// auto_update is enabled - prompt user
	fmt.Printf("\n⬆ Update available: v%s → v%s\n", info.CurrentVersion, info.LatestVersion)
	fmt.Print("Update now? [Y/n]: ")

	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))

	// Default to yes (empty or "y" or "yes")
	if response != "" && response != "y" && response != "yes" {
		fmt.Println("Skipped. Run 'agent-deck update' later.")
		return false
	}

	fmt.Println()
	release, err := update.FetchReleaseByTag(info.LatestVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: failed to fetch release info: %v\n", err)
		return false
	}
	if err := update.PerformVerifiedUpdate(release, runtime.GOOS, runtime.GOARCH); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		return false
	}

	fmt.Println("Restart agent-deck to use the new version.")
	return true
}

// initColorProfile configures lipgloss color profile based on terminal capabilities.
// Prefers TrueColor for best visuals, falls back to ANSI256 for compatibility.
func initColorProfile() {
	// Allow user override via environment variable
	// AGENTDECK_COLOR: truecolor, 256, 16, none
	if colorEnv := os.Getenv("AGENTDECK_COLOR"); colorEnv != "" {
		switch strings.ToLower(colorEnv) {
		case "truecolor", "true", "24bit":
			lipgloss.SetColorProfile(termenv.TrueColor)
			return
		case "256", "ansi256":
			lipgloss.SetColorProfile(termenv.ANSI256)
			return
		case "16", "ansi", "basic":
			lipgloss.SetColorProfile(termenv.ANSI)
			return
		case "none", "off", "ascii":
			lipgloss.SetColorProfile(termenv.Ascii)
			return
		}
	}

	// Auto-detect with TrueColor preference
	// Most modern terminals support TrueColor even if not advertised

	// Explicit TrueColor support
	colorTerm := os.Getenv("COLORTERM")
	if colorTerm == "truecolor" || colorTerm == "24bit" {
		lipgloss.SetColorProfile(termenv.TrueColor)
		return
	}

	// Check TERM for capability hints
	term := os.Getenv("TERM")

	// Known TrueColor-capable terminals
	trueColorTerms := []string{
		"xterm-256color",
		"screen-256color",
		"tmux-256color",
		"xterm-direct",
		"alacritty",
		"kitty",
		"wezterm",
	}
	for _, t := range trueColorTerms {
		if strings.Contains(term, t) || term == t {
			// These terminals typically support TrueColor
			lipgloss.SetColorProfile(termenv.TrueColor)
			return
		}
	}

	// Check for common terminal emulators via env vars
	// Windows Terminal, iTerm2, etc. set these
	if os.Getenv("WT_SESSION") != "" || // Windows Terminal
		os.Getenv("ITERM_SESSION_ID") != "" || // iTerm2
		os.Getenv("TERMINAL_EMULATOR") != "" || // JetBrains terminals
		os.Getenv("KONSOLE_VERSION") != "" { // Konsole
		lipgloss.SetColorProfile(termenv.TrueColor)
		return
	}

	// Fallback: Use ANSI256 for maximum compatibility
	// Works in SSH, basic terminals, and older emulators
	lipgloss.SetColorProfile(termenv.ANSI256)
}