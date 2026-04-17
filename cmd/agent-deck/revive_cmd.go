package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

// ReviveSummary tallies outcomes from a reviver sweep. Format() produces the
// single-line human-readable summary emitted by the CLI.
type ReviveSummary struct {
	Revived int
	Errored int
	Alive   int
	Dead    int
}

// Format returns the single-line summary. Stable keys — tests assert on
// substring presence, so "revived=N errored=N alive=N dead=N" is a contract.
func (s ReviveSummary) Format() string {
	return fmt.Sprintf("revived=%d errored=%d alive=%d dead=%d",
		s.Revived, s.Errored, s.Alive, s.Dead)
}

// runReviveAll is the testable core: classify all instances, trigger revives,
// aggregate the summary. Separate from handleSessionRevive so tests can stub
// the reviver and storage without exec'ing the binary.
func runReviveAll(instances []*session.Instance, rev *session.Reviver) ReviveSummary {
	outcomes := rev.ReviveAll(instances)
	summary := ReviveSummary{}
	for _, o := range outcomes {
		switch o.Class {
		case session.ClassAlive:
			summary.Alive++
		case session.ClassDead:
			summary.Dead++
		case session.ClassErrored:
			summary.Errored++
			if o.Revived {
				summary.Revived++
			}
		}
	}
	return summary
}

// handleSessionRevive dispatches `agent-deck session revive [--all|--name <title>]`.
// Rebuilds dead control pipes for sessions whose tmux server is still alive
// (see REPORT-D). Exits 0 on success, 1 on usage/load errors, 2 if --name not found.
func handleSessionRevive(profile string, args []string) {
	fs := flag.NewFlagSet("session revive", flag.ExitOnError)
	all := fs.Bool("all", false, "Revive all errored sessions with alive tmux servers")
	name := fs.String("name", "", "Revive a single session by title or id")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	quiet := fs.Bool("quiet", false, "Minimal output")
	quietShort := fs.Bool("q", false, "Minimal output (short)")

	fs.Usage = func() {
		fmt.Println("Usage: agent-deck session revive [--all | --name <title>]")
		fmt.Println()
		fmt.Println("Re-establish control pipes for sessions whose tmux server survived")
		fmt.Println("but whose pipe was killed (e.g., SSH logout on Linux+systemd hosts).")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(normalizeArgs(fs, args)); err != nil {
		os.Exit(1)
	}

	if !*all && *name == "" {
		fs.Usage()
		os.Exit(1)
	}

	quietMode := *quiet || *quietShort
	out := NewCLIOutput(*jsonOutput, quietMode)

	storage, instances, groups, err := loadSessionData(profile)
	if err != nil {
		out.Error(err.Error(), ErrCodeNotFound)
		os.Exit(1)
	}

	rev := session.NewReviver()

	var summary ReviveSummary
	if *all {
		summary = runReviveAll(instances, rev)
	} else {
		inst, errMsg, errCode := ResolveSession(*name, instances)
		if inst == nil {
			out.Error(errMsg, errCode)
			if errCode == ErrCodeNotFound {
				os.Exit(2)
			}
			os.Exit(1)
			return
		}
		summary = runReviveAll([]*session.Instance{inst}, rev)
	}

	// Persist any status mutations (e.g., StatusError → StatusRunning on revive).
	if err := saveSessionData(storage, instances, groups); err != nil {
		out.Error(fmt.Sprintf("failed to save session state: %v", err), ErrCodeInvalidOperation)
		os.Exit(1)
	}

	jsonData := map[string]interface{}{
		"success": true,
		"revived": summary.Revived,
		"errored": summary.Errored,
		"alive":   summary.Alive,
		"dead":    summary.Dead,
	}
	out.Success(summary.Format(), jsonData)
}

// reviveOnStartup is the non-blocking startup hook. Called once from main()
// before TUI boot. Silently logs failures; never surfaces errors to the user
// — this is a best-effort recovery, not a gate.
func reviveOnStartup(profile string) {
	_, instances, _, err := loadSessionData(profile)
	if err != nil {
		return
	}
	rev := session.NewReviver()
	_ = rev.ReviveAll(instances)
	// Note: we intentionally do NOT save storage here — the reviver is fire-
	// and-forget on startup. The next TUI tick or CLI command will persist
	// status mutations through the normal save path.
}
