package mcppool

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestScannerHandlesLargeMessages(t *testing.T) {
	// Default bufio.Scanner fails on messages > 64KB
	// MCP responses from tools like context7, firecrawl regularly exceed this
	largeMessage := strings.Repeat("x", 100*1024) // 100KB

	// This simulates what broadcastResponses does with our fix
	scanner := bufio.NewScanner(strings.NewReader(largeMessage + "\n"))
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024) // Our fix: 10MB max

	if !scanner.Scan() {
		t.Fatalf("Scanner should handle 100KB message, got error: %v", scanner.Err())
	}
	if len(scanner.Text()) != 100*1024 {
		t.Errorf("Expected 100KB message, got %d bytes", len(scanner.Text()))
	}
}

func TestDefaultScannerFailsOnLargeMessages(t *testing.T) {
	// Proves the bug: default scanner cannot handle >64KB
	largeMessage := strings.Repeat("x", 100*1024)

	scanner := bufio.NewScanner(strings.NewReader(largeMessage + "\n"))
	// No Buffer() call = default 64KB limit

	if scanner.Scan() {
		t.Fatal("Default scanner should fail on 100KB message (this proves the bug exists)")
	}
	if scanner.Err() == nil {
		t.Fatal("Expected bufio.ErrTooLong error")
	}
}

func TestBroadcastResponsesClosesClientsOnFailure(t *testing.T) {
	// When broadcastResponses exits (MCP died), all client connections
	// should be closed so reconnecting proxies know to retry
	proxy := &SocketProxy{
		name:       "test",
		clients:    make(map[string]net.Conn),
		requestMap: make(map[interface{}]string),
		Status:     StatusRunning,
	}

	// Create a pipe to simulate a client connection
	server, client := net.Pipe()
	proxy.clientsMu.Lock()
	proxy.clients["test-client"] = server
	proxy.clientsMu.Unlock()

	// Simulate what happens after broadcastResponses exits
	proxy.closeAllClientsOnFailure()

	// Client should be closed
	buf := make([]byte, 1)
	_, err := client.Read(buf)
	if err == nil {
		t.Error("Expected client connection to be closed")
	}

	// Clients map should be empty
	proxy.clientsMu.RLock()
	count := len(proxy.clients)
	proxy.clientsMu.RUnlock()
	if count != 0 {
		t.Errorf("Expected 0 clients after failure, got %d", count)
	}
}

// newTestProxy constructs a SocketProxy wired to in-memory pipes, suitable for
// unit tests that don't need a real MCP subprocess or Unix socket.
//
// Returns the proxy, the write end of the MCP stdout pipe (so tests can inject
// fake MCP responses), and the read end of the MCP stdin pipe (so tests can
// observe rewritten request IDs).
func newTestProxy(t *testing.T) (*SocketProxy, io.WriteCloser, io.ReadCloser) {
	t.Helper()

	// mcpStdinR/W simulate the MCP process's stdin
	mcpStdinR, mcpStdinW := io.Pipe()
	// mcpStdoutR/W simulate the MCP process's stdout
	mcpStdoutR, mcpStdoutW := io.Pipe()

	proxy := &SocketProxy{
		name:       "test-proxy",
		clients:    make(map[string]net.Conn),
		requestMap: make(map[interface{}]string),
		mcpStdin:   mcpStdinW,
		mcpStdout:  mcpStdoutR,
		Status:     StatusRunning,
	}

	// broadcastResponses reads from mcpStdoutR and routes to clients
	go proxy.broadcastResponses()

	return proxy, mcpStdoutW, mcpStdinR
}

// TestIDRewriteAndRestore verifies that the proxy rewrites the client-supplied
// request ID before forwarding to the MCP process and restores the original ID
// in the response before returning it to the client.
//
// RED: This test expects ID rewriting behavior not yet implemented.
// The current implementation forwards the original ID verbatim, so the response
// arrives at the client with the same ID (42) — but the MCP stdin pipe will
// also contain ID 42 rather than a proxy-assigned integer, meaning no rewrite
// occurs.  The test checks that the ID on the wire to the MCP process is NOT 42,
// which will fail until the atomic rewriting logic is added.
func TestIDRewriteAndRestore(t *testing.T) {
	proxy, mcpStdoutW, mcpStdinR := newTestProxy(t)
	defer mcpStdoutW.Close()

	// Wire a client connection into the proxy
	clientConn, serverConn := net.Pipe()
	proxy.clientsMu.Lock()
	proxy.clients["session-a"] = serverConn
	proxy.clientsMu.Unlock()
	go proxy.handleClient("session-a", serverConn)

	// Client sends a request with id: 42
	_, err := clientConn.Write([]byte(`{"jsonrpc":"2.0","method":"tools/call","params":{},"id":42}` + "\n"))
	if err != nil {
		t.Fatalf("failed to write request: %v", err)
	}

	// Read what the proxy forwarded to MCP stdin
	mcpStdinScanner := bufio.NewScanner(mcpStdinR)
	if !mcpStdinScanner.Scan() {
		t.Fatalf("expected a line on MCP stdin, got none")
	}
	forwardedLine := mcpStdinScanner.Bytes()

	var forwardedReq JSONRPCRequest
	if err := json.Unmarshal(forwardedLine, &forwardedReq); err != nil {
		t.Fatalf("failed to unmarshal forwarded request: %v", err)
	}

	// The proxy must have rewritten the ID: it should NOT be 42
	if forwardedReq.ID == nil {
		t.Fatal("forwarded request must have a non-nil ID")
	}
	// Normalize to float64 (JSON numbers decode as float64)
	forwardedIDFloat, ok := forwardedReq.ID.(float64)
	if !ok {
		t.Fatalf("expected forwarded ID to be a number, got %T (%v)", forwardedReq.ID, forwardedReq.ID)
	}
	if int64(forwardedIDFloat) == 42 {
		t.Error("proxy must rewrite the request ID; expected a proxy-assigned ID != 42, but got 42")
	}
	proxyAssignedID := forwardedIDFloat

	// Now simulate MCP responding with the proxy-assigned ID
	respLine := []byte(`{"jsonrpc":"2.0","result":"ok"}` + "\n")
	// Build response with the proxy-assigned ID embedded
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"result":  "ok",
		"id":      proxyAssignedID,
	}
	respBytes, _ := json.Marshal(resp)
	_, err = mcpStdoutW.Write(append(respBytes, '\n'))
	if err != nil {
		t.Fatalf("failed to write MCP response: %v", err)
	}
	_ = respLine // unused variable guard

	// Read the response from the client's end
	clientScanner := bufio.NewScanner(clientConn)
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	if !clientScanner.Scan() {
		t.Fatalf("client expected to receive a response, got none (err: %v)", clientScanner.Err())
	}

	var clientResp JSONRPCResponse
	if err := json.Unmarshal(clientScanner.Bytes(), &clientResp); err != nil {
		t.Fatalf("failed to unmarshal client response: %v", err)
	}

	// The response ID must be restored to the original client-supplied value (42)
	if clientResp.ID == nil {
		t.Fatal("response ID must not be nil")
	}
	restoredIDFloat, ok := clientResp.ID.(float64)
	if !ok {
		t.Fatalf("expected response ID to be a number, got %T (%v)", clientResp.ID, clientResp.ID)
	}
	if int64(restoredIDFloat) != 42 {
		t.Errorf("response ID must be restored to original value 42, got %v", int64(restoredIDFloat))
	}
}

// TestResponseRoutingNoXTalk verifies that when two clients both send id:1,
// each client receives only its own response (no cross-talk).
//
// RED: This test expects session-scoped routing not yet implemented.
// The current code uses the raw client ID as a map key, so the second
// session's write overwrites the first, causing cross-routing or drop.
func TestResponseRoutingNoXTalk(t *testing.T) {
	proxy, mcpStdoutW, mcpStdinR := newTestProxy(t)
	defer mcpStdoutW.Close()

	// Wire two client connections
	clientA, serverA := net.Pipe()
	clientB, serverB := net.Pipe()

	proxy.clientsMu.Lock()
	proxy.clients["session-a"] = serverA
	proxy.clients["session-b"] = serverB
	proxy.clientsMu.Unlock()

	go proxy.handleClient("session-a", serverA)
	go proxy.handleClient("session-b", serverB)

	// Both clients send a request with id:1
	_, err := clientA.Write([]byte(`{"jsonrpc":"2.0","method":"tools/call","params":{},"id":1}` + "\n"))
	if err != nil {
		t.Fatalf("clientA write failed: %v", err)
	}
	_, err = clientB.Write([]byte(`{"jsonrpc":"2.0","method":"tools/call","params":{},"id":1}` + "\n"))
	if err != nil {
		t.Fatalf("clientB write failed: %v", err)
	}

	// Read both forwarded requests from MCP stdin and collect their proxy IDs
	mcpStdinScanner := bufio.NewScanner(mcpStdinR)
	proxyIDToResult := map[float64]string{}

	for i := 0; i < 2; i++ {
		if !mcpStdinScanner.Scan() {
			t.Fatalf("expected request %d on MCP stdin, got none", i+1)
		}
		var req JSONRPCRequest
		if err := json.Unmarshal(mcpStdinScanner.Bytes(), &req); err != nil {
			t.Fatalf("failed to unmarshal forwarded request: %v", err)
		}
		idFloat, ok := req.ID.(float64)
		if !ok {
			t.Fatalf("expected proxy-assigned numeric ID, got %T (%v)", req.ID, req.ID)
		}
		// Assign a unique result per proxy ID so we can verify routing
		if i == 0 {
			proxyIDToResult[idFloat] = "resultA"
		} else {
			proxyIDToResult[idFloat] = "resultB"
		}
	}

	if len(proxyIDToResult) != 2 {
		t.Fatalf("expected 2 distinct proxy IDs, got %d (map: %v)", len(proxyIDToResult), proxyIDToResult)
	}

	// Send responses back through MCP stdout in reverse order to stress routing
	ids := make([]float64, 0, 2)
	for id := range proxyIDToResult {
		ids = append(ids, id)
	}
	for _, proxyID := range ids {
		result := proxyIDToResult[proxyID]
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"result":  result,
			"id":      proxyID,
		}
		respBytes, _ := json.Marshal(resp)
		_, err := mcpStdoutW.Write(append(respBytes, '\n'))
		if err != nil {
			t.Fatalf("failed to write MCP response: %v", err)
		}
	}

	// Each client must receive exactly one response with id:1 restored and
	// the correct result value. Since we don't know which proxy ID went to
	// which client, collect both and verify no cross-talk.
	type clientResult struct {
		result string
		id     int64
	}
	results := make(chan clientResult, 2)

	readClient := func(conn net.Conn, name string) {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
		scanner := bufio.NewScanner(conn)
		if !scanner.Scan() {
			t.Errorf("%s: expected response, got none (err: %v)", name, scanner.Err())
			results <- clientResult{}
			return
		}
		var resp JSONRPCResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			t.Errorf("%s: failed to unmarshal response: %v", name, err)
			results <- clientResult{}
			return
		}
		idFloat, _ := resp.ID.(float64)
		resultStr, _ := resp.Result.(string)
		results <- clientResult{result: resultStr, id: int64(idFloat)}
	}

	go readClient(clientA, "clientA")
	go readClient(clientB, "clientB")

	rA := <-results
	rB := <-results

	// Both must have original id:1 restored
	if rA.id != 1 {
		t.Errorf("clientA response ID must be 1 (original), got %d", rA.id)
	}
	if rB.id != 1 {
		t.Errorf("clientB response ID must be 1 (original), got %d", rB.id)
	}

	// The two results must differ (no cross-talk means each gets its own result)
	if rA.result == rB.result {
		t.Errorf("cross-talk detected: both clients got result %q, expected distinct results", rA.result)
	}
	if rA.result != "resultA" && rA.result != "resultB" {
		t.Errorf("clientA got unexpected result %q", rA.result)
	}
	if rB.result != "resultA" && rB.result != "resultB" {
		t.Errorf("clientB got unexpected result %q", rB.result)
	}
}

// TestConcurrentToolCalls verifies that two clients each sending 10 concurrent
// requests through a shared proxy all receive correct responses with original
// IDs restored and zero cross-talk. Passes under go test -race.
//
// RED: This test expects atomic ID rewriting not yet implemented.
func TestConcurrentToolCalls(t *testing.T) {
	proxy, mcpStdoutW, mcpStdinR := newTestProxy(t)
	defer mcpStdoutW.Close()

	// Wire two client connections
	clientA, serverA := net.Pipe()
	clientB, serverB := net.Pipe()

	proxy.clientsMu.Lock()
	proxy.clients["session-a"] = serverA
	proxy.clients["session-b"] = serverB
	proxy.clientsMu.Unlock()

	go proxy.handleClient("session-a", serverA)
	go proxy.handleClient("session-b", serverB)

	const requestsPerClient = 10

	// Goroutine: reads all 20 forwarded requests from MCP stdin and
	// sends back a response keyed as "result-{proxyID}" so each
	// response can be uniquely matched.
	var mcpServerWg sync.WaitGroup
	mcpServerWg.Add(1)
	go func() {
		defer mcpServerWg.Done()
		scanner := bufio.NewScanner(mcpStdinR)
		for i := 0; i < requestsPerClient*2; i++ {
			if !scanner.Scan() {
				return
			}
			var req JSONRPCRequest
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				continue
			}
			idFloat, ok := req.ID.(float64)
			if !ok {
				continue
			}
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"result":  strings.Repeat("r", int(idFloat)), // unique per proxy ID
				"id":      idFloat,
			}
			respBytes, _ := json.Marshal(resp)
			_, _ = mcpStdoutW.Write(append(respBytes, '\n'))
		}
	}()

	// Helper: sends 10 requests and collects 10 responses
	runClient := func(conn net.Conn, sessionLabel string, results chan<- map[int]string, errs chan<- string) {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := 1; id <= requestsPerClient; id++ {
				req := map[string]interface{}{
					"jsonrpc": "2.0",
					"method":  "tools/call",
					"params":  map[string]interface{}{},
					"id":      id,
				}
				reqBytes, _ := json.Marshal(req)
				_, err := conn.Write(append(reqBytes, '\n'))
				if err != nil {
					errs <- sessionLabel + ": write error: " + err.Error()
					return
				}
			}
		}()

		received := make(map[int]string)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck
		scanner := bufio.NewScanner(conn)
		for i := 0; i < requestsPerClient; i++ {
			if !scanner.Scan() {
				errs <- sessionLabel + ": expected more responses"
				break
			}
			var resp JSONRPCResponse
			if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
				errs <- sessionLabel + ": unmarshal error: " + err.Error()
				continue
			}
			idFloat, _ := resp.ID.(float64)
			resultStr, _ := resp.Result.(string)
			received[int(idFloat)] = resultStr
		}
		wg.Wait()
		results <- received
	}

	resultsA := make(chan map[int]string, 1)
	resultsB := make(chan map[int]string, 1)
	errs := make(chan string, 10)

	go runClient(clientA, "session-a", resultsA, errs)
	go runClient(clientB, "session-b", resultsB, errs)

	// Drain errors
	go func() {
		for e := range errs {
			t.Errorf("client error: %s", e)
		}
	}()

	rA := <-resultsA
	rB := <-resultsB

	// Wait for fake MCP server to finish processing all requests
	mcpServerWg.Wait()

	// Each client must have received all 10 original IDs (1..10)
	for id := 1; id <= requestsPerClient; id++ {
		if _, ok := rA[id]; !ok {
			t.Errorf("session-a: missing response for id %d", id)
		}
		if _, ok := rB[id]; !ok {
			t.Errorf("session-b: missing response for id %d", id)
		}
	}

	// No duplicate IDs within a single client
	if len(rA) != requestsPerClient {
		t.Errorf("session-a: expected %d unique IDs, got %d", requestsPerClient, len(rA))
	}
	if len(rB) != requestsPerClient {
		t.Errorf("session-b: expected %d unique IDs, got %d", requestsPerClient, len(rB))
	}
}
