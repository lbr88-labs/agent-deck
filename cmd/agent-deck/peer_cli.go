package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
	"github.com/asheshgoplani/agent-deck/internal/session"
)

type peerService struct {
	store    *peermsg.Store
	sessions *session.Storage
	self     string
}

func openPeerService(profile string) (*peerService, error) {
	self := strings.TrimSpace(os.Getenv("AGENTDECK_INSTANCE_ID"))
	if strings.HasPrefix(self, "system:") {
		return nil, errors.New("managed session identity cannot use the system namespace")
	}
	if self == "" {
		return nil, errors.New("peer messaging requires a managed session with AGENTDECK_INSTANCE_ID")
	}
	service, err := openPeerMailbox(profile, self)
	if err != nil {
		return nil, err
	}
	if err := service.validateRecipient(self); err != nil {
		_ = service.Close()
		return nil, fmt.Errorf("caller identity: %w", err)
	}
	return service, nil
}

func openProducerMailbox(profile, producer string) (*peerService, error) {
	if strings.TrimSpace(os.Getenv("AGENTDECK_INSTANCE_ID")) != "" || strings.TrimSpace(os.Getenv("AGENT_DECK_SESSION_ID")) != "" || GetCurrentSessionID() != "" {
		return nil, errors.New("system notifications require a caller outside a managed session")
	}
	if !validProducer(producer) {
		return nil, errors.New("--producer must be operator, heartbeat, watchdog, or completion-wake")
	}
	return openPeerMailbox(profile, "system:"+producer)
}

func validProducer(name string) bool {
	switch name {
	case "operator", "heartbeat", "watchdog", "completion-wake":
		return true
	}
	return false
}

func openPeerMailbox(profile, self string) (*peerService, error) {
	sessions, err := session.NewStorageWithProfile(profile)
	if err != nil {
		return nil, err
	}
	service := &peerService{sessions: sessions, self: self}
	dir, err := session.GetProfileDir(sessions.Profile())
	if err == nil {
		service.store, err = peermsg.Open(dir)
	}
	if err != nil {
		_ = sessions.Close()
		return nil, err
	}
	return service, nil
}

func (s *peerService) validateRecipient(id string) error {
	exists, err := s.sessions.InstanceExists(id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("unknown session ID %q in profile %q", id, s.sessions.Profile())
	}
	return nil
}

// Store.Send verifies reply authorship and destination against the parent in its insert transaction.
func (s *peerService) validatePeerDestination(id, replyTo string) error {
	if name, reserved := strings.CutPrefix(id, "system:"); reserved {
		if !validProducer(name) || replyTo == "" {
			return errors.New("a system destination requires a known producer and a matching reply parent")
		}
		return nil
	}
	return s.validateRecipient(id)
}

func (s *peerService) resolveNotificationRecipient(ref string) (string, error) {
	// Exact durable IDs also work for short or imported legacy identifiers.
	exists, err := s.sessions.InstanceExists(ref)
	if err != nil || exists {
		return ref, err
	}
	instances, err := s.sessions.Load()
	if err != nil {
		return "", err
	}
	recipient, reason, _ := ResolveSession(ref, instances)
	if recipient == nil {
		return "", errors.New(reason)
	}
	return recipient.ID, nil
}

func (s *peerService) Close() error { return errors.Join(s.store.Close(), s.sessions.Close()) }

const peerUsage = `Usage: agent-deck [-p PROFILE] message <command> [flags]

  send [--id ID] [--reply-to ID] RECIPIENT_ID BODY   Enqueue a message
  notify --producer NAME [--id ID] RECIPIENT BODY   Enqueue a system notification
  read [--after SEQUENCE] [--limit N]               List your pending messages
  ack MESSAGE_ID                                    Acknowledge receipt
  wait [--timeout 10s] [--after SEQUENCE] [--limit N]  Wait for incoming messages
  wait --response-to ID [--after SEQUENCE] [--limit N] [--reply-only] [--producer NAME] [--timeout 10s]              Wait for an ack or reply
  mcp                                               Serve MCP over stdio

System producers (operator, heartbeat, watchdog, completion-wake) run outside managed sessions.
Use a full recipient ID or an unambiguous session reference with notify.
Use --json with notify, send, read, ack, or wait. Read is repeatable until ack.
Use the full recipient session ID. A stable --id makes a send retry idempotent.
Run inside the sending or receiving Agent Deck session. Examples:

  agent-deck message send --json "$RECIPIENT_ID" 'Please review the patch'
  agent-deck message notify --producer heartbeat conductor 'Heartbeat due'
  agent-deck message wait --producer heartbeat --response-to "$MESSAGE_ID" --reply-only
  agent-deck message read --json
  agent-deck message send --reply-to "$MESSAGE_ID" "$SENDER_ID" 'Reviewed'
  agent-deck message ack "$MESSAGE_ID"
  agent-deck message wait --response-to "$MESSAGE_ID" --timeout 20s --json
`

func handlePeerMessage(profile string, args []string) {
	if err := runPeerCommand(context.Background(), profile, args, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runPeerCommand(ctx context.Context, profile string, args []string, in io.ReadCloser, out io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(out, peerUsage)
		return err
	}
	op := args[0]
	fs := flag.NewFlagSet("message "+op, flag.ContinueOnError)
	fs.SetOutput(out)
	fs.Usage = func() { fmt.Fprint(out, peerUsage) }
	var asJSON, replyOnly bool
	var id, replyTo, responseTo, producer string
	var after int64
	var limit int
	var timeout time.Duration
	if op != "mcp" {
		fs.BoolVar(&asJSON, "json", false, "JSON output")
	}
	switch op {
	case "notify":
		fs.StringVar(&producer, "producer", "", "system producer name")
		fs.StringVar(&id, "id", "", "stable message ID")
	case "send":
		fs.StringVar(&id, "id", "", "stable message ID")
		fs.StringVar(&replyTo, "reply-to", "", "original message ID")
	case "wait", "read":
		fs.Int64Var(&after, "after", 0, "exclusive sequence cursor")
		fs.IntVar(&limit, "limit", 0, "page size")
		if op == "wait" {
			fs.DurationVar(&timeout, "timeout", 10*time.Second, "maximum wait")
			fs.BoolVar(&replyOnly, "reply-only", false, "wait for a reply after acknowledgment")
			fs.StringVar(&producer, "producer", "", "system producer name")
			fs.StringVar(&responseTo, "response-to", "", "sent message ID")
		}
	case "ack", "mcp":
	default:
		return fmt.Errorf("unknown message command %q", op)
	}
	// Keep an explicit -- boundary so literal message bodies can begin with '-'.
	raw := args[1:]
	var literal []string
	if i := slices.Index(raw, "--"); i >= 0 {
		literal = raw[i+1:]
		raw = raw[:i]
	}
	if err := fs.Parse(normalizeArgs(fs, raw)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	positional := append(slices.Clone(fs.Args()), literal...)
	want := map[string]int{"send": 2, "notify": 2, "ack": 1}[op]
	if len(positional) != want {
		return fmt.Errorf("message %s expects %d positional arguments (see message --help)", op, want)
	}
	var svc *peerService
	var err error
	if op == "wait" && responseTo == "" && (replyOnly || producer != "") {
		return errors.New("--reply-only and --producer require --response-to")
	}
	if op == "notify" || producer != "" {
		svc, err = openProducerMailbox(profile, producer)
	} else {
		svc, err = openPeerService(profile)
	}
	if err != nil {
		return err
	}
	defer svc.Close()
	var result any
	switch op {
	case "mcp":
		return servePeerMCP(ctx, svc, in, out)
	case "notify", "send":
		recipient := positional[0]
		if op == "notify" {
			recipient, err = svc.resolveNotificationRecipient(recipient)
		} else {
			err = svc.validatePeerDestination(recipient, replyTo)
		}
		if err != nil {
			return err
		}
		result, err = svc.store.Send(ctx, peermsg.SendRequest{ID: id, SenderID: svc.self, RecipientID: recipient, Body: positional[1], ReplyTo: replyTo})
	case "ack":
		result, err = svc.store.Ack(ctx, svc.self, positional[0])
	case "read":
		result, err = svc.store.Pending(ctx, svc.self, after, limit)
	case "wait":
		if responseTo != "" {
			result, err = svc.store.WaitResponse(ctx, svc.self, responseTo, peermsg.ResponseOptions{
				After: after, Limit: limit, Timeout: timeout, ReplyOnly: replyOnly,
			})
		} else {
			result, err = svc.store.Wait(ctx, svc.self, after, limit, timeout)
		}
	}
	if err != nil {
		return err
	}
	if asJSON {
		return json.NewEncoder(out).Encode(result)
	}
	return printPeerResult(out, result)
}

func printPeerResult(out io.Writer, result any) error {
	switch v := result.(type) {
	case peermsg.Message:
		status := "accepted"
		if v.AcknowledgedAt != nil {
			status = "acknowledged"
		}
		_, err := fmt.Fprintf(out, "%q %s (#%d) %q -> %q  %q\n", v.ID, status, v.Sequence, v.SenderID, v.RecipientID, v.Body)
		return err
	case []peermsg.Message:
		if len(v) == 0 {
			_, err := fmt.Fprintln(out, "No pending messages.")
			return err
		}
		for _, m := range v {
			if err := printPeerResult(out, m); err != nil {
				return err
			}
		}
	case peermsg.Response:
		if v.TimedOut {
			_, err := fmt.Fprintln(out, "Timed out waiting for an acknowledgment or reply.")
			return err
		}
		if err := printPeerResult(out, v.Message); err != nil {
			return err
		}
		for _, m := range v.Replies {
			if err := printPeerResult(out, m); err != nil {
				return err
			}
		}
	}
	return nil
}
