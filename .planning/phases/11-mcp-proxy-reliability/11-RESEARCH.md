# Phase 11: MCP Proxy Reliability - Research

**Researched:** 2026-03-13
**Domain:** JSON-RPC multiplexing, Unix socket proxy, Go concurrency
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MCP-01 | MCP socket proxy assigns unique request IDs per proxy instance to prevent collisions when multiple sessions share the same proxy (#324) | Root cause confirmed in code: `requestMap[req.ID] = sessionID` overwrites on duplicate IDs. Fix: atomic counter rewrites ID before forwarding to MCP stdin. |
| MCP-02 | Request/response correlation uses session-scoped ID mapping so responses route to the correct caller (#324) | Current `requestMap map[interface{}]string` is keyed by the raw client-supplied ID. Fix: keyed by proxy-assigned ID, stores both original ID and session ID for response restoration. |
| MCP-03 | Integration test verifies two concurrent sessions issuing tool calls through a shared proxy receive correct responses without cross-talk | No such test exists yet. A new test using `net.Pipe()` pairs (no real MCP process needed) can simulate two clients concurrently, under `-race`. |
</phase_requirements>

---

## Summary

The `SocketProxy` in `internal/mcppool/socket_proxy.go` multiplexes multiple client connections onto a single stdio MCP process. The bug is well-understood and confirmed in the code: `handleClient` records `requestMap[req.ID] = sessionID` using the raw JSON-RPC ID supplied by the client. Since Claude Code generates sequential integer IDs starting from 1, two sessions both send `id: 1`, `id: 2`, etc. The second session's write silently overwrites the first session's entry, causing the first session's response to be routed to the second session or dropped entirely.

The fix pattern is a standard JSON-RPC proxy technique: rewrite every incoming request ID with a proxy-global atomic counter before forwarding to the MCP process, store the mapping `{proxyID -> (sessionID, originalID)}`, and on response receipt restore the original ID before forwarding back to the correct session. The `requestMap` must be redesigned to hold this two-field struct instead of just a session ID string.

The issue author's suggested fix (atomic counter + `sync.Map`) is the correct approach. The existing `requestMu sync.Mutex` + `map[interface{}]string` pattern can be replaced with an `atomic.Int64` counter and a new `idMapping` struct stored in a `sync.Map` (or the existing locked map with a new value type). The `broadcastToAll` path for notifications (responses with `id: null`) is correct and should remain unchanged.

**Primary recommendation:** Replace `requestMap map[interface{}]string` with `idMap sync.Map` keyed by `int64` proxy IDs; add `atomic.Int64 nextID`; rewrite IDs in `handleClient`; restore original IDs in `routeToClient`.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `sync/atomic` (stdlib) | Go 1.24 | Atomic int64 counter for proxy ID generation | Zero contention, no lock needed for ID increment |
| `sync.Map` (stdlib) | Go 1.24 | Concurrent map for `proxyID -> idMapping`; replaces `requestMap` + mutex | Optimized for write-once/read-once access patterns, which matches request/response lifecycle |
| `encoding/json` (stdlib) | Go 1.24 | Already used for JSON-RPC marshal/unmarshal | No change needed |
| `bufio.Scanner` (stdlib) | Go 1.24 | Already used for line-by-line reading | No change needed |

### Supporting (no new deps required)

All required primitives are in the Go standard library. No new external dependencies needed.

**Installation:** No new packages to install. This is a pure refactor of `internal/mcppool/socket_proxy.go`.

---

## Architecture Patterns

### Current Architecture (BROKEN)

```
Client A (id:1) ──┐
                   ├──► requestMap[1] = "sessionA"  ← Client B overwrites this
Client B (id:1) ──┘     requestMap[1] = "sessionB"  ← COLLISION

MCP stdin ← both requests forwarded verbatim (two id:1 in flight)
MCP stdout → response id:1 → routeToClient(1) → only "sessionB" gets it
                                                    "sessionA" hangs forever
```

### Fixed Architecture

```
Client A (id:1) ─► rewrite id=101 ─► idMap[101] = {sessionA, id:1}
Client B (id:1) ─► rewrite id=102 ─► idMap[102] = {sessionB, id:1}

MCP stdin ← {id:101, ...}, {id:102, ...}  (globally unique IDs)
MCP stdout → {id:101, result:...} → restore id:1 → route to sessionA
             {id:102, result:...} → restore id:1 → route to sessionB
```

### Pattern 1: Atomic ID Rewriting

**What:** Replace incoming JSON-RPC ID with a proxy-scoped monotonically increasing int64 before forwarding to the MCP process. Store the reverse mapping so the original ID is restored before sending the response back to the client.

**When to use:** Any time multiple clients share one JSON-RPC server connection.

**Example (the exact change in `handleClient`):**

```go
// Source: Issue #324 suggested fix + Go stdlib docs

type idMapping struct {
    sessionID  string
    originalID interface{}
}

// In SocketProxy struct:
//   nextID atomic.Int64
//   idMap  sync.Map  // int64 -> idMapping

func (p *SocketProxy) handleClient(sessionID string, conn net.Conn) {
    defer func() {
        // Clean up orphaned idMap entries for this client
        p.idMap.Range(func(k, v interface{}) bool {
            if v.(idMapping).sessionID == sessionID {
                p.idMap.Delete(k)
            }
            return true
        })
        // ... rest of cleanup unchanged
    }()

    scanner := bufio.NewScanner(conn)
    scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
    for scanner.Scan() {
        line := scanner.Bytes()

        var req JSONRPCRequest
        if err := json.Unmarshal(line, &req); err != nil {
            continue
        }

        if req.ID != nil {
            proxyID := p.nextID.Add(1)
            p.idMap.Store(proxyID, idMapping{
                sessionID:  sessionID,
                originalID: req.ID,
            })
            // Rewrite ID before forwarding
            req.ID = proxyID
            rewritten, err := json.Marshal(req)
            if err == nil {
                line = rewritten
            }
        }

        _, _ = p.mcpStdin.Write(line)
        _, _ = p.mcpStdin.Write([]byte("\n"))
    }
}
```

**Example (the exact change in `routeToClient`):**

```go
func (p *SocketProxy) routeToClient(responseID interface{}, line []byte) {
    // responseID from MCP is now a proxy int64; look it up
    // JSON numbers unmarshal as float64 when type is interface{}
    var proxyKey int64
    switch v := responseID.(type) {
    case float64:
        proxyKey = int64(v)
    case int64:
        proxyKey = v
    case json.Number:
        n, _ := v.Int64()
        proxyKey = n
    default:
        // Non-integer IDs: fall through to broadcastToAll
        p.broadcastToAll(line)
        return
    }

    val, ok := p.idMap.LoadAndDelete(proxyKey)
    if !ok {
        p.broadcastToAll(line)
        return
    }

    mapping := val.(idMapping)

    // Restore original ID in response before sending to client
    var resp JSONRPCResponse
    if err := json.Unmarshal(line, &resp); err == nil {
        resp.ID = mapping.originalID
        if restored, err := json.Marshal(resp); err == nil {
            line = restored
        }
    }

    p.clientsMu.RLock()
    conn, exists := p.clients[mapping.sessionID]
    p.clientsMu.RUnlock()

    if exists {
        _, _ = conn.Write(line)
        _, _ = conn.Write([]byte("\n"))
    }
}
```

### Pattern 2: Type-Safe ID Extraction

**What:** When the MCP process responds, JSON numbers decode as `float64` when the target type is `interface{}`. The proxy must handle this type coercion consistently when looking up the proxyID in the map.

**When to use:** Any code path that receives a response ID from JSON unmarshaling into `interface{}`.

**Key insight:** Use `json.Decoder` with `UseNumber()` option to avoid `float64` precision loss for large int64 IDs, or constrain proxy IDs to the safe integer range for `float64` (< 2^53). Since `atomic.Int64` starts at 1 and a typical session won't exceed millions of calls, `float64` precision is not a practical concern.

### Pattern 3: `sync.Map` vs Locked Map Tradeoff

**What:** `sync.Map` is optimized for the write-once/delete-once access pattern (one `Store` per request, one `LoadAndDelete` per response). The existing `requestMu sync.Mutex` + `map[interface{}]string` could also be adapted, but `sync.Map` eliminates the need for the mutex entirely.

**When to use `sync.Map`:** When entries are inserted once and deleted once with high read concurrency. This matches the request/response lifecycle exactly.

**Alternative (also acceptable):** Keep the existing `requestMu` mutex but change the map value type to `idMapping`. The `sync.Map` approach is marginally cleaner but both are correct.

### Recommended Project Structure (no changes needed)

The fix is entirely within `internal/mcppool/socket_proxy.go`. No new files required. The test belongs in `internal/mcppool/socket_proxy_test.go`.

### Anti-Patterns to Avoid

- **Do not use `requestMap[req.ID] = sessionID` with client-supplied IDs.** This is the root cause. Any solution that keeps the original ID as the map key will still collide.
- **Do not assume JSON-RPC IDs are integers.** The spec allows strings too. The fix must handle string IDs correctly. For string IDs, the proxy-assigned key is always int64, so there is no collision even if clients send string IDs like `"req-1"`.
- **Do not remove the `broadcastToAll` path.** JSON-RPC notifications (method calls with `id: null` or no ID) are legitimate and must still be broadcast to all clients.
- **Do not skip the ID cleanup in `closeAllClientsOnFailure`.** The existing cleanup of `requestMap` must also clear `idMap` when the MCP process dies.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Thread-safe counter | Custom mutex-wrapped counter | `atomic.Int64.Add(1)` | Already in stdlib, zero contention |
| Concurrent map for in-flight requests | New mutex + map | `sync.Map` | Optimized for write-once/delete-once; stdlib |
| JSON-RPC framing/parsing | Custom parser | `encoding/json` (already used) | Already handles all edge cases |

**Key insight:** The entire fix requires zero new dependencies. Every primitive needed is in the Go standard library and already imported in the file.

---

## Common Pitfalls

### Pitfall 1: float64 Type Coercion on Response ID Lookup

**What goes wrong:** When `broadcastResponses` unmarshals a JSON-RPC response into `JSONRPCResponse` (which has `ID interface{}`), a JSON integer like `101` comes back as `float64(101)`, not `int64(101)`. Looking up `float64(101)` in an `sync.Map` keyed by `int64(101)` will fail to find the entry.

**Why it happens:** Go's `encoding/json` unmarshals JSON numbers into `float64` when the target type is `interface{}`.

**How to avoid:** Normalize the response ID to `int64` before the lookup using a type switch (see Pattern 2 above). Alternatively, use `json.Decoder.UseNumber()` and call `.Int64()`.

**Warning signs:** Test passes when request IDs are sequential small integers but responses are silently broadcast to all instead of routed to the correct client.

### Pitfall 2: String Request IDs

**What goes wrong:** Some MCP clients send string IDs (e.g., `"id": "req-abc-123"`). The proxy must still rewrite these to its own int64 counter. The response routing must use the proxy's int64 key, not the client's original string.

**Why it happens:** JSON-RPC 2.0 spec (RFC 7049) allows IDs to be strings, numbers, or null.

**How to avoid:** Rewrite ALL non-null IDs regardless of type. The `if req.ID != nil` guard covers this correctly.

**Warning signs:** Tool calls hang only for sessions using string-format request IDs.

### Pitfall 3: Cleanup Incomplete for Disconnected Clients

**What goes wrong:** When a client disconnects mid-flight (e.g., session stopped while a tool call was pending), the `handleClient` goroutine exits. If it only cleans up the `clients` map but leaves orphaned entries in `idMap` for that session, the orphaned entries accumulate.

**Why it happens:** The current `requestMap` cleanup in `handleClient`'s defer block iterates the map looking for entries matching `sessionID`. The `sync.Map` equivalent must do the same with `Range`.

**How to avoid:** The defer in `handleClient` must range over `idMap` and delete all entries where `mapping.sessionID == sessionID`. See the cleanup code in Pattern 1 above.

**Warning signs:** `idMap` grows indefinitely in long-running sessions; responses for dead sessions attempt to write to closed connections.

### Pitfall 4: closeAllClientsOnFailure Must Clear idMap Too

**What goes wrong:** When `broadcastResponses` exits (MCP died), `closeAllClientsOnFailure` clears `clients` and the old `requestMap`. If `idMap` (the new sync.Map) is not also cleared, stale entries accumulate across restarts.

**Why it happens:** `sync.Map` has no `Clear()` method pre-Go 1.23. In Go 1.24 (used here), `sync.Map.Clear()` is available.

**How to avoid:** Call `p.idMap.Clear()` in `closeAllClientsOnFailure` and in `Stop()`. For clarity, also clear in `RestartProxyWithRateLimit`.

**Warning signs:** After a proxy restart, responses for new requests are silently dropped because old stale entries exist in `idMap`.

### Pitfall 5: Race Between routeToClient and Client Disconnect

**What goes wrong:** `routeToClient` calls `p.idMap.LoadAndDelete` (atomically removes), then looks up `p.clients` under `clientsMu.RLock`. If the client disconnected between these two steps, the write to `conn.Write` returns an error. This is benign (the error is discarded), but the log should reflect it.

**Why it happens:** Inherent TOCTOU between map lookup and connection write.

**How to avoid:** The existing `_, _ = conn.Write(...)` pattern is acceptable. Optionally add a debug log for the write error to aid future debugging.

---

## Code Examples

Verified patterns from official sources and project codebase:

### atomic.Int64 Usage (Go stdlib)

```go
// Source: https://pkg.go.dev/sync/atomic#Int64
var counter atomic.Int64
id := counter.Add(1) // returns new value; thread-safe, no mutex needed
```

### sync.Map Usage Pattern (write-once/delete-once)

```go
// Source: https://pkg.go.dev/sync#Map
var m sync.Map

// Store (in handleClient)
m.Store(proxyID, idMapping{sessionID: "A", originalID: 1})

// LoadAndDelete (in routeToClient)
val, ok := m.LoadAndDelete(proxyID)
if ok {
    mapping := val.(idMapping)
    // use mapping
}

// Bulk cleanup (in handleClient defer / closeAllClientsOnFailure)
m.Range(func(k, v interface{}) bool {
    if v.(idMapping).sessionID == targetSessionID {
        m.Delete(k)
    }
    return true
})

// Full clear (Go 1.23+, available in Go 1.24)
m.Clear()
```

### Existing SocketProxy Struct Fields to Replace

```go
// REMOVE:
requestMap map[interface{}]string
requestMu  sync.Mutex

// ADD:
nextID atomic.Int64       // proxy-scoped request ID counter
idMap  sync.Map           // int64 -> idMapping; no separate mutex needed
```

### JSON Number Handling in routeToClient

```go
// Source: Go encoding/json docs — interface{} receives float64 for JSON numbers
switch v := responseID.(type) {
case float64:
    proxyKey = int64(v)
case json.Number:
    proxyKey, _ = strconv.ParseInt(v.String(), 10, 64)
case int64:
    proxyKey = v
default:
    p.broadcastToAll(line)
    return
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Client-supplied IDs as routing keys | Proxy-assigned IDs as routing keys | This phase | Eliminates collisions between sessions |
| `map[interface{}]string` + mutex | `sync.Map` with `idMapping` struct | This phase | Single map, no separate mutex, correct cleanup |
| No integration test for concurrency | Concurrent tool call test under `-race` | This phase | Regression protection for MCP-01/MCP-02 |

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib), `go test -race` |
| Config file | none (no external config; standard `go test` flags) |
| Quick run command | `go test -race -v ./internal/mcppool/...` |
| Full suite command | `go test -race -v ./...` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MCP-01 | Two clients send `id: 1` concurrently; proxy assigns unique IDs so both responses arrive | integration | `go test -race -v -run TestConcurrentToolCalls ./internal/mcppool/` | Wave 0 (new test needed) |
| MCP-02 | Response with proxy ID `101` is routed to session A with original `id: 1`; session B unaffected | integration | `go test -race -v -run TestResponseRouting ./internal/mcppool/` | Wave 0 (new test needed) |
| MCP-03 | Two concurrent sessions, 10 tool calls each, zero cross-talk under `-race` | integration | `go test -race -v -run TestConcurrentToolCalls ./internal/mcppool/` | Wave 0 (new test needed) |

### Sampling Rate

- **Per task commit:** `go test -race -v ./internal/mcppool/...`
- **Per wave merge:** `go test -race -v ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- `internal/mcppool/socket_proxy_test.go` needs three new test functions: `TestConcurrentToolCalls`, `TestResponseRoutingNoXTalk`, `TestIDRewriteAndRestore`. The file already exists; only additions needed.
- No `testmain_test.go` exists in `internal/mcppool/`. The existing tests (`TestScannerHandlesLargeMessages`, `TestBroadcastResponsesClosesClientsOnFailure`) don't require `AGENTDECK_PROFILE` isolation since `mcppool` has no SQLite state. A `testmain_test.go` is NOT required here, consistent with the pattern in `internal/git/`.

---

## Open Questions

1. **Should `idMap` use `sync.Map` or the existing mutex pattern?**
   - What we know: Both are correct. `sync.Map` eliminates the need for a separate mutex.
   - What's unclear: `sync.Map.Range` for cleanup is O(n entries) but the total in-flight count is bounded by `maxClientsPerProxy * max_concurrent_requests_per_client`, which is small.
   - Recommendation: Use `sync.Map` for clarity. The mutex alternative is also acceptable if preferred for consistency with the rest of the file's locking style.

2. **Are string request IDs possible from Claude Code?**
   - What we know: Claude Code historically uses small sequential integers. The issue author confirms integer IDs.
   - What's unclear: Future versions of Claude Code or other MCP clients might use string IDs.
   - Recommendation: Handle both types defensively with the `req.ID != nil` guard and the type switch in `routeToClient`. This costs nothing.

3. **Does the test require a real MCP subprocess?**
   - What we know: Existing tests use `net.Pipe()` and directly manipulate `SocketProxy` struct fields, bypassing `Start()`.
   - Recommendation: Use the same approach for the new concurrent test. Two `net.Pipe()` pairs simulate two clients. A goroutine simulating the MCP server reads requests and echoes responses. No real MCP process needed.

---

## Sources

### Primary (HIGH confidence)

- **Codebase direct read:** `internal/mcppool/socket_proxy.go` lines 40, 288-296, 354-375 — root cause confirmed
- **GitHub issue #324** (project issue tracker) — detailed root cause analysis and suggested fix by bug reporter, independently verified against code
- **Go stdlib docs:** `sync/atomic.Int64`, `sync.Map` — authoritative API reference

### Secondary (MEDIUM confidence)

- **JSON-RPC 2.0 spec behavior:** ID types (string/number/null) — well-established protocol behavior, consistent with Go `encoding/json` behavior

### Tertiary (LOW confidence — no validation needed, all facts confirmed in code)

None.

---

## Metadata

**Confidence breakdown:**
- Root cause: HIGH — confirmed by direct code inspection and issue #324
- Fix approach: HIGH — standard proxy pattern, suggested by issue author, verified against Go stdlib
- Test approach: HIGH — follows existing test patterns in `socket_proxy_test.go`
- Type coercion pitfall: HIGH — confirmed by Go `encoding/json` behavior docs
- `sync.Map.Clear()` availability: HIGH — available since Go 1.23; project uses Go 1.24

**Research date:** 2026-03-13
**Valid until:** 2026-06-13 (stable domain; Go stdlib APIs are stable)
