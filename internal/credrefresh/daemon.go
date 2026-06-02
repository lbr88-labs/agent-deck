package credrefresh

import (
	"context"
	"path/filepath"
	"time"
)

// credentialsFileName is the profile's OAuth credentials file.
const credentialsFileName = ".credentials.json"

// CanonicalCredPath returns the canonical credentials path for a profile config
// dir (e.g. ~/.claude → ~/.claude/.credentials.json).
func CanonicalCredPath(configDir string) string {
	return filepath.Join(configDir, credentialsFileName)
}

// DaemonConfig configures the keep-warm refresh loop.
type DaemonConfig struct {
	// ConfigDirs are the profile config dirs to keep warm (one per profile,
	// e.g. ~/.claude and ~/.claude-work). The canonical credentials path is
	// derived from each.
	ConfigDirs []string
	// Interval is the tick cadence. Defaults to DefaultInterval.
	Interval time.Duration
	// Refresh is the per-attempt config (endpoint, client, threshold, clock).
	Refresh RefreshConfig
	// OnResult, if set, is called after every refresh attempt with the
	// canonical path, result, and error. Used for logging.
	OnResult func(credPath string, res RefreshResult, err error)
}

func (c DaemonConfig) interval() time.Duration {
	if c.Interval > 0 {
		return c.Interval
	}
	return DefaultInterval
}

// Run keeps the canonical credentials warm until ctx is cancelled. It refreshes
// immediately on startup (so a host that woke past expiry is healed at once),
// then on every Interval tick. Each profile is refreshed independently; one
// profile's error never blocks another. Run blocks until ctx is done.
func Run(ctx context.Context, cfg DaemonConfig) {
	tick := func() {
		for _, dir := range cfg.ConfigDirs {
			credPath := CanonicalCredPath(dir)
			res, err := RefreshIfNeeded(credPath, cfg.Refresh)
			if cfg.OnResult != nil {
				cfg.OnResult(credPath, res, err)
			}
		}
	}

	tick() // immediate

	ticker := time.NewTicker(cfg.interval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}
