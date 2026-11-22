package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/peermsg"
	"github.com/asheshgoplani/agent-deck/internal/session"
	"github.com/stretchr/testify/require"
)

func TestPeerMCPWire(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AGENT_DECK_HOME", "")
	session.ClearUserConfigCache()
	t.Cleanup(session.ClearUserConfigCache)
	sessions, err := session.NewStorageWithProfile("test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sessions.Close() })
	require.NoError(t, sessions.Save([]*session.Instance{{ID: "alice", Title: "Alice"}, {ID: "bob", Title: "Bob"}}))

	dir := t.TempDir()
	store, err := peermsg.Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	// An independent connection represents another client of the shared mailbox.
	other, err := peermsg.Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = other.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	server, client := net.Pipe()
	defer client.Close()
	deadline, _ := ctx.Deadline()
	require.NoError(t, client.SetDeadline(deadline))
	done := make(chan error, 1)
	go func() {
		done <- servePeerMCP(ctx, &peerService{self: "alice", store: store, sessions: sessions}, server, server)
	}()
	reader := bufio.NewReader(client)
	id := 0
	type response struct {
		ID    int `json:"id"`
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
		Result struct {
			IsError    bool            `json:"isError"`
			Structured json.RawMessage `json:"structuredContent"`
			Tools      []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	call := func(method, params string) response {
		t.Helper()
		id++
		_, err := fmt.Fprintf(client, "{\"jsonrpc\":\"2.0\",\"id\":%d,\"method\":%q,\"params\":%s}\n", id, method, params)
		require.NoError(t, err)
		line, err := reader.ReadBytes('\n')
		require.NoError(t, err)
		var reply response
		require.NoError(t, json.Unmarshal(line, &reply))
		require.Equal(t, id, reply.ID)
		return reply
	}
	require.Nil(t, call("initialize", `{"protocolVersion":"2025-06-18","clientInfo":{"name":"test","version":"1"},"capabilities":{}}`).Error)
	_, err = io.WriteString(client, "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n")
	require.NoError(t, err)
	list := call("tools/list", "{}")
	var names []string
	for _, tool := range list.Result.Tools {
		names = append(names, tool.Name)
	}
	require.ElementsMatch(t, []string{"peer_send", "peer_read", "peer_ack", "peer_wait", "peer_wait_response"}, names)

	tool := func(name, args string) response {
		t.Helper()
		return call("tools/call", fmt.Sprintf(`{"name":%q,"arguments":%s}`, name, args))
	}
	require.NotNil(t, call("unknown", "{}").Error)
	for _, bad := range []string{`{"recipient_id":"bob","body":"spoof","sender_id":"eve"}`, `{"body":"missing recipient"}`} {
		reply := tool("peer_send", bad)
		require.True(t, reply.Error != nil || reply.Result.IsError)
	}
	require.True(t, tool("peer_send", `{"recipient_id":"missing","body":"hello"}`).Result.IsError)
	sent := tool("peer_send", `{"recipient_id":"bob","body":"hello\nworld","id":"first"}`)
	require.Nil(t, sent.Error)
	require.False(t, sent.Result.IsError)
	messages, err := other.Pending(ctx, "bob", 0, 0)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, "alice", messages[0].SenderID)
	require.Equal(t, "hello\nworld", messages[0].Body)
	require.True(t, tool("peer_ack", `{"id":"first"}`).Result.IsError)
	_, err = other.Ack(ctx, "bob", "first")
	require.NoError(t, err)
	var awaiting peermsg.Response
	poll := tool("peer_wait_response", `{"id":"first","timeout_ms":0,"reply_only":true}`)
	require.NoError(t, json.Unmarshal(poll.Result.Structured, &awaiting))
	require.True(t, awaiting.TimedOut)
	_, err = other.Send(ctx, peermsg.SendRequest{SenderID: "bob", RecipientID: "alice", ID: "answer", ReplyTo: "first", Body: "done"})
	require.NoError(t, err)
	for _, op := range []string{"peer_read", "peer_wait"} {
		reply := tool(op, "{}")
		var page peerPage
		require.NoError(t, json.Unmarshal(reply.Result.Structured, &page))
		require.Len(t, page.Messages, 1)
		require.Equal(t, "answer", page.Messages[0].ID)
	}
	reply := tool("peer_wait_response", `{"id":"first","timeout_ms":0}`)
	var observed peermsg.Response
	require.NoError(t, json.Unmarshal(reply.Result.Structured, &observed))
	require.Len(t, observed.Replies, 1)
	require.Equal(t, "answer", observed.Replies[0].ID)
	next := tool("peer_wait_response", fmt.Sprintf(`{"id":"first","reply_only":true,"after":%d,"limit":1,"timeout_ms":0}`, observed.Replies[0].Sequence))
	require.NoError(t, json.Unmarshal(next.Result.Structured, &observed))
	require.True(t, observed.TimedOut)
	require.Empty(t, observed.Replies)
	require.False(t, tool("peer_ack", `{"id":"answer"}`).Result.IsError)
	messages, err = other.Pending(ctx, "alice", 0, 0)
	require.NoError(t, err)
	require.Empty(t, messages)
	_, err = other.Send(ctx, peermsg.SendRequest{ID: "operator-parent", SenderID: "system:operator", RecipientID: "alice", Body: "question"})
	require.NoError(t, err)
	require.True(t, tool("peer_send", `{"recipient_id":"system:operator","body":"unrelated"}`).Result.IsError)
	require.False(t, tool("peer_send", `{"recipient_id":"system:operator","reply_to":"operator-parent","body":"answer"}`).Result.IsError)
	require.NoError(t, client.Close())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
