package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

func responseCursor(id string, dispatch time.Time) string {
	return id + "." + dispatch.UTC().Format(time.RFC3339Nano)
}

func waitFresh(since time.Time, r *session.ResponseOutput) bool {
	if r == nil || r.Role != "assistant" || strings.TrimSpace(r.Content) == "" {
		return false
	}
	stamp, err := time.Parse(time.RFC3339Nano, r.Timestamp)
	return err == nil && stamp.After(since)
}

func runSessionWait(profile string, args []string) int {
	fs := flag.NewFlagSet("session wait", flag.ContinueOnError)
	timeout := fs.Duration("timeout", 2*time.Minute, "Maximum time to wait")
	cursor := fs.String("cursor", "", "wait_cursor from session send --json (also detects a reply completed before wait starts)")
	sinceText := fs.String("since", "", "Require response timestamp after this RFC3339 time")
	machine := fs.Bool("json", false, "Print one JSON result")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: agent-deck session wait <id|title> [--cursor TOKEN | --since RFC3339] [--timeout 2m] [--json]\nExample: agent-deck session wait my-project --cursor TOKEN --json\nWithout a cursor, use the last recorded dispatch, or wait for the next reply if none exists.\nExit codes: 0 completed, 1 error, 2 missing, 3 stopped, 4 timeout.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(normalizeArgs(fs, args)); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	emit := func(status, msg string, code int) int {
		if *machine {
			data := map[string]any{"status": status, "success": code == 0}
			if code == 0 {
				data["output"] = msg
			} else {
				data["error"] = msg
			}
			b, _ := json.Marshal(data)
			fmt.Println(string(b))
		} else if code == 0 {
			fmt.Println(msg)
		} else {
			fmt.Fprintln(os.Stderr, msg)
		}
		return code
	}
	if fs.NArg() != 1 || *timeout <= 0 || (*sinceText != "" && *cursor != "") {
		return emit("invalid", "Specify one session, a positive timeout, and at most one cursor.", 1)
	}
	storage, peers, _, err := loadSessionData(profile)
	if err != nil {
		return emit("error", err.Error(), 1)
	}
	defer storage.Close()
	inst, msg, code := ResolveSession(fs.Arg(0), peers)
	if inst == nil {
		if code == ErrCodeNotFound {
			return emit("missing", msg, 2)
		}
		return emit("error", msg, 1)
	}
	since := time.Now()
	if *cursor != "" {
		value, ok := strings.CutPrefix(*cursor, inst.ID+".")
		if !ok {
			return emit("invalid", "Cursor belongs to a different session", 1)
		}
		since, err = time.Parse(time.RFC3339Nano, value)
	} else if *sinceText != "" {
		since, err = time.Parse(time.RFC3339Nano, *sinceText)
	} else if db := storage.GetDB(); db != nil {
		if value, e := db.GetMeta("session-wait-dispatch:" + inst.ID); e == nil && value != "" {
			since, err = time.Parse(time.RFC3339Nano, value)
		} else if ts, e := db.ReadLastSentAt(inst.ID); e == nil && ts > 0 {
			since = time.Unix(ts+1, 0).Add(-time.Nanosecond)
		}
	}
	if err != nil {
		return emit("invalid", err.Error(), 1)
	}
	id := inst.ID
	bound := waitBindingID(inst)
	// data_version must be sampled on a single SQLite connection. Skip full
	// fleet loads while the profile has not changed.
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	conn, err := storage.GetDB().DB().Conn(ctx)
	if err != nil {
		return emit("error", err.Error(), 1)
	}
	defer conn.Close()
	var lastVersion int64 = -1
	status := func() (string, error) {
		var version int64
		if err := conn.QueryRowContext(ctx, "PRAGMA data_version").Scan(&version); err != nil {
			return "", err
		}
		if version != lastVersion {
			peers, err = storage.Load()
			if err != nil {
				return "", err
			}
			inst = nil
			for _, candidate := range peers {
				if candidate.ID == id {
					inst = candidate
					break
				}
			}
			if inst == nil {
				return "missing", nil
			}
			current := waitBindingID(inst)
			if bound != "" && bound != current {
				return "", errors.New("session conversation changed while waiting")
			}
			bound = current
			lastVersion = version
		}
		pane := inst.GetTmuxSession()
		if pane == nil {
			return "inactive", nil
		}
		return pane.GetStatus()
	}
	outcome, msg, exitCode := waitForSavedReply(since, *timeout, 500*time.Millisecond, status, func() (*session.ResponseOutput, error) {
		return inst.LastSavedWaitResponse(peers)
	})
	return emit(outcome, msg, exitCode)
}

func waitBindingID(inst *session.Instance) string {
	switch {
	case session.IsCodexCompatible(inst.Tool) && inst.CodexBindingConfirmed:
		return inst.CodexSessionID
	case session.IsClaudeCompatible(inst.Tool):
		return inst.ClaudeSessionID
	case inst.Tool == "gemini":
		return inst.GeminiSessionID
	default:
		return ""
	}
}

func waitForSavedReply(since time.Time, timeout, interval time.Duration, getStatus func() (string, error), read func() (*session.ResponseOutput, error)) (string, string, int) {
	deadline := time.Now().Add(timeout)
	for {
		status, err := getStatus()
		if errors.Is(err, context.DeadlineExceeded) {
			return "timeout", "Timed out waiting for fresh output", 4
		}
		if err != nil {
			return "error", err.Error(), 1
		}
		if status == "missing" {
			return "missing", "Session no longer exists", 2
		}
		if status != "active" {
			r, err := read()
			if err == nil && waitFresh(since, r) {
				return "completed", r.Content, 0
			}
			if status == "inactive" || status == "error" {
				return "stopped", "Session stopped before a reply was available", 3
			}
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return "error", err.Error(), 1
			}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return "timeout", "Timed out waiting for fresh output", 4
		}
		time.Sleep(min(remaining, interval))
	}
}
