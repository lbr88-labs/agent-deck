---
phase: 14-simple-adapters-webhook-ntfy-github
plan: 01
subsystem: watcher
tags: [webhook, ntfy, http-server, ndjson, exponential-backoff, adapter-pattern]

# Dependency graph
requires:
  - phase: 13-watcher-engine-core
    provides: WatcherAdapter interface, Event struct, AdapterConfig, Engine lifecycle, goleak patterns
provides:
  - WebhookAdapter implementing WatcherAdapter (HTTP POST receiver on configurable port)
  - NtfyAdapter implementing WatcherAdapter (NDJSON stream subscriber with reconnect)
  - testmain_test.go with AGENTDECK_PROFILE=_test isolation for watcher package
affects: [14-02 (GitHub adapter), future adapter implementations, engine integration tests]

# Tech tracking
tech-stack:
  added: []
  patterns: [HTTP server adapter lifecycle (Setup creates/Listen starts/Teardown no-op), NDJSON stream client with exponential backoff reconnect, RWMutex for concurrent addr/eventsCh access, port-0 listener for test isolation]

key-files:
  created:
    - internal/watcher/testmain_test.go
    - internal/watcher/webhook.go
    - internal/watcher/webhook_test.go
    - internal/watcher/ntfy.go
    - internal/watcher/ntfy_test.go
  modified: []

key-decisions:
  - "Used net.Listen with port 0 in tests to avoid port conflicts between parallel test runs"
  - "RWMutex on WebhookAdapter addr field to prevent data races when Listen resolves port 0"
  - "NtfyAdapter exposes initialBackoff/maxBackoff fields for test-time override instead of injecting a clock"
  - "Non-blocking channel send in both adapters (select with default) to prevent slow consumers from blocking the adapter"

patterns-established:
  - "HTTP server adapter pattern: Setup creates mux+server, Listen starts net.Listen+Serve, Teardown no-op"
  - "NDJSON client adapter pattern: streamOnce reads lines, Listen wraps with backoff reconnect loop"
  - "Test backoff override: expose initialBackoff/maxBackoff fields, set to small values in tests"
  - "Port 0 test isolation: use net.Listen port 0, store actual addr via mutex"

requirements-completed: [ADAPT-01, ADAPT-02]

# Metrics
duration: 6min
completed: 2026-04-10
---

# Phase 14 Plan 01: Webhook + ntfy Adapters Summary

**WebhookAdapter (HTTP POST receiver with 202 response, 1MB body limit) and NtfyAdapter (NDJSON stream subscriber with 2s/2x/30s exponential backoff reconnect) implementing WatcherAdapter interface**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-10T13:59:55Z
- **Completed:** 2026-04-10T14:05:50Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- WebhookAdapter: standalone HTTP server on 127.0.0.1:18460 (configurable), POST /webhook normalizes body to Event, responds 202 before processing, 1MB body limit, ReadHeaderTimeout 5s, TCP dial health check
- NtfyAdapter: subscribes to /topic/json NDJSON stream, skips open/keepalive events, normalizes message events (title or first-line subject, ntfy:topic@host sender), auto-reconnects with exponential backoff (2s initial, 2x factor, 30s cap), resumes from last message ID
- testmain_test.go with AGENTDECK_PROFILE=_test isolation per CLAUDE.md mandate
- 24 new tests (12 webhook + 12 ntfy) all passing with -race and goleak verification
- All 40 watcher package tests pass (Phase 13 + Phase 14)

## Task Commits

Each task was committed atomically (TDD: test then feat):

1. **Task 1: testmain_test.go + WebhookAdapter with tests**
   - `e86e439` (test) - failing webhook adapter tests and testmain_test.go
   - `8ec2f81` (feat) - implement WebhookAdapter with race-safe HTTP server lifecycle

2. **Task 2: NtfyAdapter with reconnect and tests**
   - `e79583f` (test) - failing ntfy adapter tests
   - `937a54e` (feat) - implement NtfyAdapter with NDJSON stream and exponential backoff

## Files Created/Modified
- `internal/watcher/testmain_test.go` - AGENTDECK_PROFILE=_test isolation for watcher test package
- `internal/watcher/webhook.go` - WebhookAdapter: HTTP server, POST handler, 202 response, 1MB limit, health check
- `internal/watcher/webhook_test.go` - 12 tests: setup, POST/GET, body limit, normalization, health, goleak
- `internal/watcher/ntfy.go` - NtfyAdapter: NDJSON stream, backoff reconnect, message ID resumption, health check
- `internal/watcher/ntfy_test.go` - 12 tests: setup, messages, skip/keepalive, normalization, reconnect, backoff, goleak

## Decisions Made
- Used `net.Listen` with port 0 in tests to avoid port conflicts (tests bind to random available ports)
- Added `sync.RWMutex` on WebhookAdapter to protect `addr` and `eventsCh` from data races (discovered in TDD GREEN phase when -race detected concurrent write from Listen and read from HealthCheck)
- NtfyAdapter exposes `initialBackoff`/`maxBackoff` fields for test-time override rather than clock injection (simpler, matches pipemanager.go pattern)
- Both adapters use non-blocking channel send (select with default) per plan spec, preventing slow consumers from blocking the adapter

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed data race on WebhookAdapter.addr field**
- **Found during:** Task 1 (WebhookAdapter implementation, GREEN phase)
- **Issue:** When using port 0, Listen writes `a.addr` with the resolved address while HealthCheck (via waitForServer) reads it concurrently. Race detector caught this.
- **Fix:** Changed mutex from sync.Mutex to sync.RWMutex, protected addr writes in Listen with Lock and reads in HealthCheck with RLock
- **Files modified:** internal/watcher/webhook.go
- **Verification:** All 12 webhook tests pass with -race flag
- **Committed in:** 8ec2f81 (part of Task 1 feat commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Race fix was necessary for correctness under -race testing. No scope creep.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Both adapters validate the WatcherAdapter lifecycle (Setup/Listen/Teardown/HealthCheck) against real HTTP protocols
- Server-role (webhook) and client-role (ntfy) patterns established for future adapters
- Plan 14-02 (GitHub adapter with HMAC-SHA256 verification) can proceed; it follows the webhook server-role pattern
- All Phase 13 engine tests continue to pass alongside the new adapter tests

## Self-Check: PASSED

All 6 files verified present. All 4 commits verified in git log.

---
*Phase: 14-simple-adapters-webhook-ntfy-github*
*Completed: 2026-04-10*
