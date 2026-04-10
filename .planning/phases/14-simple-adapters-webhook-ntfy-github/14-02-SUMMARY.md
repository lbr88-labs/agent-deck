---
phase: 14-simple-adapters-webhook-ntfy-github
plan: 02
subsystem: watcher
tags: [github-webhook, hmac-sha256, adapter-pattern, integration-test, event-normalization]

# Dependency graph
requires:
  - phase: 14-simple-adapters-webhook-ntfy-github
    plan: 01
    provides: WebhookAdapter, NtfyAdapter, testmain_test.go
  - phase: 13-watcher-engine-core
    provides: WatcherAdapter interface, Event struct, Engine lifecycle, Router
provides:
  - GitHubAdapter implementing WatcherAdapter with HMAC-SHA256 signature verification
  - Integration test proving all three adapters (webhook, ntfy, github) flow through engine pipeline
affects: [future adapter implementations, watcher system integration, conductor routing]

# Tech tracking
tech-stack:
  added: []
  patterns: [HMAC-SHA256 constant-time verification via hmac.Equal, GitHub event normalization (issues/pull_request/push/unknown), HTTP server adapter with signature gating]

key-files:
  created:
    - internal/watcher/github.go
    - internal/watcher/github_test.go
    - internal/watcher/adapters_integration_test.go
  modified: []

key-decisions:
  - "HMAC-SHA256 verification uses hmac.Equal for constant-time comparison (prevents timing attacks, T-14-02)"
  - "Default bind 127.0.0.1:18461 (port 18461 distinct from webhook's 18460) per T-14-04"
  - "Pusher email preferred over sender login for push events when email contains @"
  - "Unknown event types produce generic event with truncated body (1000 chars) per D-14"
  - "Integration test uses wildcard routing (*@github.com) and exact routing for webhook/ntfy senders"

patterns-established:
  - "GitHub HMAC gating pattern: read full body, verify signature, respond 202, then normalize and emit"
  - "Event type normalization: type-specific payload structs with json.Unmarshal into typed fields"
  - "Integration test pattern: mock ntfy server, port-0 for HTTP adapters, routing verification via DB queries"

requirements-completed: [ADAPT-03]

# Metrics
duration: 4min
completed: 2026-04-10
---

# Phase 14 Plan 02: GitHub Adapter + Integration Test Summary

**GitHubAdapter with HMAC-SHA256 signature verification (hmac.Equal constant-time) normalizing issues/PR/push/unknown events, plus integration test validating all three adapters through engine dedup and routing pipeline**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-10T14:10:31Z
- **Completed:** 2026-04-10T14:14:53Z
- **Tasks:** 2
- **Files created:** 3

## Accomplishments
- GitHubAdapter: standalone HTTP server on 127.0.0.1:18461 (configurable), POST /github with HMAC-SHA256 signature verification, responds 202 before processing, 10MB body limit, ReadHeaderTimeout 5s
- HMAC verification: constant-time comparison via hmac.Equal, rejects missing signatures (401), invalid signatures (401), malformed prefixes, bad hex
- Event normalization: issues ("[opened] #42: Bug title"), pull_request ("[PR opened] #17: Feature title"), push ("[push] main: 1 commit(s)"), unknown ("[check_run] event from owner/repo")
- Push events prefer pusher email over sender login when email contains "@"
- Integration test wires all three adapters (WebhookAdapter, NtfyAdapter with mock server, GitHubAdapter) through a real engine with temp statedb
- Integration test verifies: event persistence (3 rows), routing (each to correct conductor), EventCh delivery (3 events with correct sources)
- Dedup test confirms events from different sources are NOT deduped (DedupKey includes Source)
- 19 new tests (17 GitHub + 2 integration) all passing with -race and goleak verification
- All 62 watcher package tests pass (Phase 13 + Plan 01 + Plan 02)

## Task Commits

Each task was committed atomically (TDD: test then feat):

1. **Task 1: GitHubAdapter with HMAC verification and event normalization**
   - `d8512cc` (test) - failing GitHub adapter tests (17 tests)
   - `137821f` (feat) - implement GitHubAdapter with HMAC-SHA256 verification

2. **Task 2: Integration test wiring all three adapters through engine**
   - `4184258` (test) - integration test with all three adapters

## Files Created/Modified
- `internal/watcher/github.go` (359 lines) - GitHubAdapter: HTTP server, HMAC-SHA256 gating, event normalization for 4 event types, health check
- `internal/watcher/github_test.go` (681 lines) - 17 tests: setup, HMAC verification (4 edge cases), listen with valid/invalid/missing signatures, body size limit, normalization (issues/PR/push/unknown), health check, goleak
- `internal/watcher/adapters_integration_test.go` (290 lines) - 2 tests: full pipeline with all 3 adapters + dedup across adapters

## Decisions Made
- HMAC-SHA256 verification uses `hmac.Equal` for constant-time comparison, preventing timing-based signature forgery (T-14-02)
- Default bind 127.0.0.1:18461 (port 18461, distinct from webhook's 18460) to avoid accidental public exposure (T-14-04)
- Pusher email preferred over sender login for push events when the email field contains "@", matching GitHub's documented behavior
- Unknown event types produce a generic event with body truncated to 1000 characters (D-14)
- Integration test uses wildcard domain routing (`*@github.com`) for GitHub events and exact routing for webhook/ntfy senders

## Deviations from Plan

None - plan executed exactly as written.

## Threat Surface Scan

No new threat surface beyond what is documented in the plan's threat_model. All five threats (T-14-02 through T-14-09) are mitigated:
- T-14-02 (Spoofing): hmac.Equal constant-time comparison
- T-14-03 (Tampering): full body read before verification, 10MB limit
- T-14-04 (Information Disclosure): default bind 127.0.0.1
- T-14-08 (DoS): ReadHeaderTimeout 5s, MaxHeaderBytes 1<<20, body limit 10MB, non-blocking channel send
- T-14-09 (Repudiation): accepted (X-GitHub-Delivery header available for logging)

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All three Phase 14 adapters (webhook, ntfy, github) are implemented and tested
- Integration test confirms the full adapter -> engine -> dedup -> routing pipeline works
- 62 watcher package tests pass with -race flag
- Phase 14 is complete; the watcher subsystem has a solid adapter foundation for future extensions (Slack, Gmail, etc.)

## Self-Check: PASSED
