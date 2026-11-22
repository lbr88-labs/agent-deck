package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type peerSendInput struct {
	RecipientID string `json:"recipient_id" jsonschema:"Exact Agent Deck session ID of the recipient"`
	Body        string `json:"body" jsonschema:"Message text"`
	ID          string `json:"id,omitempty" jsonschema:"Stable message ID for retrying the same send"`
	ReplyTo     string `json:"reply_to,omitempty" jsonschema:"ID of the message being answered"`
}

type peerReadInput struct {
	After int64 `json:"after,omitempty" jsonschema:"Return pending messages with sequence greater than this cursor"`
	Limit int   `json:"limit,omitempty" jsonschema:"Maximum page size (default 50; maximum 200)"`
}

type peerPage struct {
	Messages []peermsg.Message `json:"messages"`
}

type peerWaitInput struct {
	peerReadInput
	TimeoutMS *int `json:"timeout_ms,omitempty" jsonschema:"Time to wait in milliseconds (default 30000; zero to poll)"`
}

type peerAckInput struct {
	ID string `json:"id" jsonschema:"Message ID to acknowledge"`
}

type peerWaitResponseInput struct {
	After     int64  `json:"after,omitempty" jsonschema:"Return replies with sequence greater than this cursor"`
	Limit     int    `json:"limit,omitempty" jsonschema:"Maximum reply page size (default and maximum 200)"`
	ReplyOnly bool   `json:"reply_only,omitempty" jsonschema:"Wait for a correlated reply even when receipt is acknowledged"`
	ID        string `json:"id" jsonschema:"ID of a message sent by this session"`
	TimeoutMS *int   `json:"timeout_ms,omitempty" jsonschema:"Time to wait in milliseconds (default 30000; zero to poll)"`
}

func peerTimeout(ms *int) (time.Duration, error) {
	if ms == nil {
		return peermsg.MaxWait, nil
	}
	if *ms < 0 || int64(*ms) > peermsg.MaxWait.Milliseconds() {
		return 0, fmt.Errorf("timeout_ms must be between 0 and %d", peermsg.MaxWait.Milliseconds())
	}
	return time.Duration(*ms) * time.Millisecond, nil
}

func newPeerMCP(service *peerService) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "agent-deck-message", Version: Version}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "peer_send", Description: "Send a durable message as the bound Agent Deck session."}, func(ctx context.Context, _ *mcp.CallToolRequest, in peerSendInput) (*mcp.CallToolResult, peermsg.Message, error) {
		if err := service.validatePeerDestination(in.RecipientID, in.ReplyTo); err != nil {
			return nil, peermsg.Message{}, err
		}
		message, err := service.store.Send(ctx, peermsg.SendRequest{ID: in.ID, SenderID: service.self, RecipientID: in.RecipientID, Body: in.Body, ReplyTo: in.ReplyTo})
		return nil, message, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "peer_read", Description: "Read this session's pending messages in sequence order. Messages stay pending until acknowledged."}, func(ctx context.Context, _ *mcp.CallToolRequest, in peerReadInput) (*mcp.CallToolResult, peerPage, error) {
		messages, err := service.store.Pending(ctx, service.self, in.After, in.Limit)
		return nil, peerPage{messages}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "peer_ack", Description: "Acknowledge receipt of a message addressed to this session."}, func(ctx context.Context, _ *mcp.CallToolRequest, in peerAckInput) (*mcp.CallToolResult, peermsg.Message, error) {
		message, err := service.store.Ack(ctx, service.self, in.ID)
		return nil, message, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "peer_wait", Description: "Wait for pending messages addressed to this session, returning an empty page on timeout."}, func(ctx context.Context, _ *mcp.CallToolRequest, in peerWaitInput) (*mcp.CallToolResult, peerPage, error) {
		timeout, err := peerTimeout(in.TimeoutMS)
		if err != nil {
			return nil, peerPage{}, err
		}
		messages, err := service.store.Wait(ctx, service.self, in.After, in.Limit, timeout)
		return nil, peerPage{messages}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "peer_wait_response", Description: "Wait for acknowledgment or a correlated reply to a message this session sent."}, func(ctx context.Context, _ *mcp.CallToolRequest, in peerWaitResponseInput) (*mcp.CallToolResult, peermsg.Response, error) {
		timeout, err := peerTimeout(in.TimeoutMS)
		if err != nil {
			return nil, peermsg.Response{}, err
		}
		response, err := service.store.WaitResponse(ctx, service.self, in.ID, peermsg.ResponseOptions{
			After: in.After, Limit: in.Limit, Timeout: timeout, ReplyOnly: in.ReplyOnly,
		})
		return nil, response, err
	})
	return server
}

// The SDK closes its transport when the client disconnects. Stdout belongs to the process.
type peerMCPWriter struct{ io.Writer }

func (peerMCPWriter) Close() error { return nil }

func servePeerMCP(ctx context.Context, service *peerService, in io.ReadCloser, out io.Writer) error {
	return newPeerMCP(service).Run(ctx, &mcp.IOTransport{Reader: in, Writer: peerMCPWriter{out}})
}
