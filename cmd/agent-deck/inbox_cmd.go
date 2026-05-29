package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

// handleInbox is the dispatch entry for `agent-deck inbox <session-id>`. It
// drains the per-conductor inbox file populated by the transition notifier
// when in-process retries exhaust against a busy parent. Reading truncates;
// callers should expect at-most-once delivery (consumer-side dedup, not
// producer-side, intentional — see internal/session/inbox.go).
func handleInbox(args []string) {
	if err := runInbox(os.Stdout, args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runInbox is the testable seam — handleInbox wires it to os.Stdout/Stderr;
// tests pass a buffer.
//
// Forms:
//
//	agent-deck inbox <session-id>          legacy raw drain (read + truncate)
//	agent-deck inbox drain [--json] <id>   issue #1225 consumer drain — collapses
//	                                       last-wins per child and dedups
//	                                       re-delivery via turn_fingerprint. This
//	                                       is the conductor's heartbeat step.
func runInbox(stdout io.Writer, args []string) error {
	if len(args) > 0 && args[0] == "drain" {
		return runInboxDrain(stdout, args[1:])
	}

	fs := flag.NewFlagSet("inbox", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(stdout, "Usage: agent-deck inbox <session-id>")
		fmt.Fprintln(stdout, "       agent-deck inbox drain [--json] <session-id>")
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Drain pending completion events from the parent's durable outbox.")
		fmt.Fprintln(stdout, "The `drain` form (issue #1225) collapses last-wins per child and")
		fmt.Fprintln(stdout, "dedups re-delivery via turn_fingerprint; run it first on every")
		fmt.Fprintln(stdout, "heartbeat. Reading clears the inbox.")
	}
	if err := fs.Parse(normalizeArgs(fs, args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected exactly one session id argument")
	}
	sessionID := fs.Arg(0)

	events, err := session.ReadAndTruncateInbox(sessionID)
	if err != nil {
		return fmt.Errorf("read inbox: %w", err)
	}
	printInboxEvents(stdout, events)
	return nil
}

// runInboxDrain is the issue #1225 consumer path: exactly-once-per-turn,
// last-wins-per-child. Used by the conductor heartbeat and any machine consumer.
func runInboxDrain(stdout io.Writer, args []string) error {
	fs := flag.NewFlagSet("inbox drain", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "emit the drained events as a JSON array")
	fs.Usage = func() {
		fmt.Fprintln(stdout, "Usage: agent-deck inbox drain [--json] <session-id>")
	}
	if err := fs.Parse(normalizeArgs(fs, args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected exactly one session id argument")
	}
	sessionID := fs.Arg(0)

	events, err := session.DrainInboxForParent(sessionID)
	if err != nil {
		return fmt.Errorf("drain inbox: %w", err)
	}

	if *asJSON {
		if events == nil {
			events = []session.TransitionNotificationEvent{}
		}
		enc := json.NewEncoder(stdout)
		return enc.Encode(events)
	}

	printInboxEvents(stdout, events)
	return nil
}

func printInboxEvents(stdout io.Writer, events []session.TransitionNotificationEvent) {
	if len(events) == 0 {
		fmt.Fprintln(stdout, "No pending events.")
		return
	}
	for _, ev := range events {
		fmt.Fprintf(stdout, "%s  child=%s title=%q profile=%s %s→%s\n",
			ev.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			ev.ChildSessionID,
			ev.ChildTitle,
			ev.Profile,
			ev.FromStatus,
			ev.ToStatus,
		)
	}
	fmt.Fprintf(stdout, "\nDrained %d event(s).\n", len(events))
}
