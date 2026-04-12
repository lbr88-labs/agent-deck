---
phase: 13-watcher-engine-core
plan: 01
subsystem: watcher
tags: [watcher, adapter, router, health, tdd, new-package]
dependency_graph:
  requires: [internal/session/watcher_meta.go, internal/session/userconfig.go, internal/statedb/statedb.go]
  provides: [internal/watcher/adapter.go, internal/watcher/router.go, internal/watcher/health.go]
  affects: [internal/logging/logger.go]
tech_stack:
  added: [internal/watcher package, crypto/sha256 for DedupKey]
  patterns: [TDD red-green-commit, passive health tracker, exact-over-wildcard routing]
key_files:
  created:
    - internal/watcher/adapter.go
    - internal/watcher/adapter_test.go
    - internal/watcher/router.go
    - internal/watcher/router_test.go
    - internal/watcher/health.go
    - internal/watcher/health_test.go
  modified:
    - internal/logging/logger.go
key_decisions:
  - "SetLastEventTimeForTest exported on HealthTracker for deterministic silence detection tests"
  - "Router uses LastIndex for @ split to handle edge cases in email addresses"
  - "External test package (watcher_test) used to enforce public API boundary testing"
metrics:
  duration: "~20 minutes"
  completed: "2026-04-10T13:12:19Z"
  tasks_completed: 3
  files_changed: 7
  tests_added: 14
---

# Phase 13 Plan 01: Watcher Foundation Types Summary

Foundational interfaces, routing logic, and health tracking for the watcher engine: WatcherAdapter interface, Event struct with DedupKey(), config-driven Router with clients.json exact/wildcard matching, and HealthTracker with rolling rate and silence detection.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | WatcherAdapter interface, Event struct, AdapterConfig, CompWatcher | 142aed8 | adapter.go, adapter_test.go, logger.go |
| 2 | Router with clients.json loading and exact/wildcard matching | a181368 | router.go, router_test.go |
| 3 | HealthTracker with silence detection, error counting, rolling rate | 7364402 | health.go, health_test.go |

## Deliverables

**internal/watcher/adapter.go**
- `WatcherAdapter` interface with 4 methods: `Setup`, `Listen`, `Teardown`, `HealthCheck`
- `AdapterConfig` struct with `Type`, `Name`, `Settings` fields
- `Event` struct with all 6 fields (source, sender, subject, body, timestamp, raw_payload) with JSON tags
- `DedupKey()` method returning `sha256(source|sender|subject|timestamp_rfc3339nano)` as hex

**internal/watcher/router.go**
- `ClientEntry` struct with conductor/group/name JSON fields
- `RouteResult` struct with match type indicator
- `NewRouter()` splitting `*@domain` wildcards from exact email keys
- `LoadClientsJSON()` reading and parsing clients.json from disk
- `LoadFromWatcherDir()` using `session.WatcherDir()` path resolver
- `Match()` with exact-over-wildcard priority per D-08

**internal/watcher/health.go**
- `HealthStatus` type with `healthy`, `warning`, `error` constants
- `HealthState` struct for TUI and health alert consumption (Phase 16)
- `HealthTracker` passive struct (no goroutine) with sliding window
- `RecordEvent()` resets errors and prunes old timestamps (T-13-03 mitigated)
- `Check()` evaluates all conditions: adapter health, consecutive errors, silence

**internal/logging/logger.go**
- Added `CompWatcher = "watcher"` constant

## Test Coverage

14 tests across 3 files, all passing with `-race`:

| File | Tests | Coverage |
|------|-------|---------|
| adapter_test.go | 3 | DedupKey determinism, different sender produces different key, JSON round-trip |
| router_test.go | 5 | Exact over wildcard, unrouted nil, wildcard match, empty map, LoadClientsJSON |
| health_test.go | 6 | Fresh tracker healthy, silence detection, 3 errors = warning, 10 errors = error, RecordEvent resets errors, adapter unhealthy = error |

## Deviations from Plan

### Auto-added for testability

**SetLastEventTimeForTest method added to HealthTracker**
- Found during: Task 3
- Issue: Tests need to simulate silence without sleeping; lastEventTime is unexported
- Fix: Added exported `SetLastEventTimeForTest(t time.Time)` method to enable deterministic silence detection tests
- Files modified: internal/watcher/health.go, internal/watcher/health_test.go
- This is test-only infrastructure, clearly named with `ForTest` suffix to indicate purpose

### External test package

Tests use `package watcher_test` (external) rather than `package watcher` (internal). This enforces testing against the public API boundary, which is appropriate for a new package establishing its contracts.

## Threat Model Compliance

| Threat | Disposition | Status |
|--------|-------------|--------|
| T-13-01 (tampering: LoadClientsJSON) | mitigate | LoadClientsJSON validates JSON parse; comment documents engine-startup validation of empty conductor fields as Phase 13 engine responsibility |
| T-13-02 (info disclosure: Event.Body) | accept | Accepted by design; not mitigated in this plan |
| T-13-03 (DoS: eventTimestamps growth) | mitigate | Pruning implemented in RecordEvent(); lazy cleanup on EventsInLastHour() read |
| T-13-04 (spoofing: Event.Sender) | accept | Adapter responsibility (Phase 14+) |
| T-13-05 (tampering: clients.json path) | mitigate | LoadFromWatcherDir uses session.WatcherDir() hardcoded path |

## Known Stubs

None. All implementations are complete and functional. No placeholder values or TODO stubs in production code paths.

## Self-Check: PASSED

Files exist:
- internal/watcher/adapter.go: FOUND
- internal/watcher/router.go: FOUND
- internal/watcher/health.go: FOUND
- internal/logging/logger.go (CompWatcher): FOUND

Commits exist:
- 142aed8 (Task 1: adapter): FOUND
- a181368 (Task 2: router): FOUND
- 7364402 (Task 3: health): FOUND

All 14 tests pass: `go test -race -v ./internal/watcher/...` exits 0
Build clean: `go build ./internal/watcher/...` exits 0
Vet clean: `go vet ./internal/watcher/...` exits 0
