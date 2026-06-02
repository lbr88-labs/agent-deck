// Package main — `agent-deck creds-refresh` subcommand.
//
// The keep-warm OAuth refresh daemon (Part B of the recurring "/login" outage
// fix). It keeps each profile's canonical `.credentials.json` token fresh so
// the N symlinked worker sessions never hit an expired access token and never
// race Anthropic's single-use rotating refresh token. See the package doc in
// internal/credrefresh and /tmp/oauth-fix/SUBSCRIPTION-FIX.md.
//
// Subscription-safe: it uses the profile's existing OAuth refresh token and
// clientId — NO API key, no per-token billing. It exchanges the refresh token
// at the OAuth /token endpoint and atomically rewrites canonical under the same
// proper-lockfile Claude uses, so a running session and the daemon never
// refresh at the same instant.
//
// Usage:
//
//	agent-deck creds-refresh --once                 # single pass, exit (cron-friendly)
//	agent-deck creds-refresh                         # run forever (systemd --user)
//	agent-deck creds-refresh --config-dir ~/.claude --config-dir ~/.claude-work
//	agent-deck creds-refresh --interval 25m --threshold 20m
//
// Default config dirs (when none given): ~/.claude and ~/.claude-work, whichever
// has a .credentials.json present.
//
// NOTE: not started automatically anywhere. Enable it deliberately via the
// systemd --user unit in scripts/systemd/ (see that file's header).

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/credrefresh"
	"github.com/asheshgoplani/agent-deck/internal/session"
)

// stringSliceFlag collects a repeatable string flag (--config-dir a --config-dir b).
type stringSliceFlag []string

func (s *stringSliceFlag) String() string { return strings.Join(*s, ",") }
func (s *stringSliceFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func handleCredsRefresh(args []string) {
	fs := flag.NewFlagSet("creds-refresh", flag.ExitOnError)
	once := fs.Bool("once", false, "run a single refresh pass and exit (cron-friendly)")
	interval := fs.Duration("interval", credrefresh.DefaultInterval, "refresh cadence when running as a daemon")
	threshold := fs.Duration("threshold", credrefresh.DefaultThreshold, "refresh when the access token expires within this window")
	endpoint := fs.String("endpoint", credrefresh.DefaultTokenEndpoint, "OAuth token endpoint (override for testing only)")
	var configDirs stringSliceFlag
	fs.Var(&configDirs, "config-dir", "profile config dir to keep warm (repeatable); defaults to ~/.claude and ~/.claude-work")
	if err := fs.Parse(normalizeArgs(fs, args)); err != nil {
		os.Exit(1)
	}

	dirs := resolveCredConfigDirs(configDirs)
	if len(dirs) == 0 {
		fmt.Fprintln(os.Stderr, "creds-refresh: no profile config dir with a .credentials.json found; pass --config-dir explicitly")
		os.Exit(1)
	}

	cfg := credrefresh.DaemonConfig{
		ConfigDirs: dirs,
		Interval:   *interval,
		Refresh: credrefresh.RefreshConfig{
			TokenEndpoint: *endpoint,
			Threshold:     *threshold,
		},
		OnResult: logCredsRefreshResult,
	}

	if *once {
		// Single pass: a tick with an already-cancelled context returns after
		// the immediate refresh.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		credrefresh.Run(ctx, cfg)
		return
	}

	fmt.Fprintf(os.Stderr, "creds-refresh: keeping %d profile(s) warm every %s (threshold %s)\n", len(dirs), *interval, *threshold)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	credrefresh.Run(ctx, cfg)
}

// resolveCredConfigDirs expands the explicit --config-dir flags, or falls back
// to the standard dual-profile pair, keeping only dirs that actually hold a
// .credentials.json (so a single-profile host doesn't error on the other).
func resolveCredConfigDirs(explicit stringSliceFlag) []string {
	candidates := explicit
	if len(candidates) == 0 {
		home, err := os.UserHomeDir()
		if err == nil {
			candidates = stringSliceFlag{
				filepath.Join(home, ".claude"),
				filepath.Join(home, ".claude-work"),
			}
		}
	}
	out := make([]string, 0, len(candidates))
	for _, d := range candidates {
		dir := session.ExpandPath(d)
		if _, err := os.Stat(credrefresh.CanonicalCredPath(dir)); err == nil {
			out = append(out, dir)
		}
	}
	return out
}

func logCredsRefreshResult(credPath string, res credrefresh.RefreshResult, err error) {
	ts := time.Now().Format(time.RFC3339)
	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "%s creds-refresh ERROR %s: %v\n", ts, credPath, err)
	case res.Refreshed:
		fmt.Fprintf(os.Stderr, "%s creds-refresh OK %s rotated; next expiry %s\n", ts, credPath, res.ExpiresAt.Format(time.RFC3339))
	default:
		fmt.Fprintf(os.Stderr, "%s creds-refresh skip %s (%s)\n", ts, credPath, res.Reason)
	}
}
