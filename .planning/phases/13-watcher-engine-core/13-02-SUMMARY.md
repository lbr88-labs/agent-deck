---
phase: 13-watcher-engine-core
plan: 02
subsystem: watcher
tags: [engine, event-loop, dedup, goleak, goroutine-lifecycle, adapter-management]
dependency_graph:
  requires: [13-01]
  provides: [Engine, EngineConfig, NewEngine, eventEnvelope, MockAdapter]
  affects: [internal/watcher/engine.go, internal/watcher/engine_test.go, internal/watcher/mock_adapter_test.go]
tech_stack:
  added: [go.uber.org/goleak v1.3.0]
  patterns: [single-writer-goroutine, eventEnvelope-wrapper, context-cancellation-cascade, non-blocking-channel-send]
key_files:
  created:
    - internal/watcher/engine.go
    - internal/watcher/engine_test.go
    - internal/watcher/mock_adapter_test.go
  modified:
    - go.mod
    - go.sum
decisions:
  - Used eventEnvelope wrapper to avoid modifying the public Event struct with internal routing fields
  - Intermediary adapter channel with forwarding goroutine for per-adapter envelope wrapping
  - goleak with IgnoreAnyFunction for modernc.org sqlite driver goroutines
  - DB assertions via statedb.DB() accessor for direct SQL queries in tests
metrics:
  duration: 3m44s
  completed: 2026-04-10T13:20:37Z
  tasks: 2/2
  files_created: 3
  files_modified: 2
---

# Phase 13 Plan 02: Watcher Engine Core Summary

Engine event loop with single-writer goroutine serializing all DB writes, INSERT OR IGNORE dedup, adapter lifecycle via derived contexts, and goleak-verified zero goroutine leaks after Stop.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | faaf023 | Engine struct with event loop, single-writer goroutine, and MockAdapter |
| 2 | bfb9bc1 | Engine tests for dedup, goleak, routing, and adapter teardown |

## What Was Built

### Engine (internal/watcher/engine.go)

- **EngineConfig**: holds DB, Router, MaxEventsPerWatcher, HealthCheckInterval, Logger
- **Engine**: orchestrates adapter goroutines via derived contexts with a single-writer goroutine
- **eventEnvelope**: internal wrapper that pairs an Event with its watcherID and HealthTracker (avoids modifying the public Event struct)
- **NewEngine(cfg)**: creates engine with buffered channels (eventCh: 64, routedEventCh: 64, healthCh: 16)
- **RegisterAdapter(watcherID, adapter, config, maxSilenceMinutes)**: registers adapters before Start
- **Start()**: calls Setup on each adapter, launches per-adapter goroutines + single-writer + optional health loop
- **runAdapter()**: creates intermediary channel, wraps events in envelopes with non-blocking sends (T-13-06)
- **writerLoop()**: single-writer goroutine that routes via Router.Match, persists via SaveWatcherEvent with dedup, forwards inserted events to routedEventCh (D-13, D-14)
- **healthLoop()**: periodic adapter HealthCheck with non-blocking HealthState emission
- **Stop()**: cancels root context, calls Teardown on all adapters, waits for all goroutines via sync.WaitGroup
- **EventCh() / HealthCh()**: read-only channels for TUI consumption (D-20)

### MockAdapter (internal/watcher/mock_adapter_test.go)

- Implements WatcherAdapter with configurable events, delays, and error injection
- Emits all configured events then blocks until context cancelled
- Tracks setupCalled and teardownCalled for lifecycle verification

### Engine Tests (internal/watcher/engine_test.go)

- **TestWatcherEngine_Dedup**: sends 2 identical events, verifies only 1 persisted and 1 routed (D-23)
- **TestWatcherEngine_Stop_NoLeaks**: 3 adapters each sending 1 event, goleak.VerifyNone after Stop (D-22)
- **TestWatcherEngine_KnownSenderRouting**: event from known sender routed to correct conductor
- **TestWatcherEngine_UnknownSenderRouting**: event from unknown sender saved with empty routed_to
- **TestWatcherEngine_StopCancelsAdapters**: verifies Teardown called on all adapters after Stop

### Test Infrastructure

- newTestDB/newTestEngine helpers for isolated test databases
- countWatcherEvents/queryWatcherEventRoutedTo for direct DB assertions via statedb.DB()
- drainEvents helper for channel-based assertions

## Deviations from Plan

None. Plan executed exactly as written.

## Test Results

All 20 watcher tests pass (5 new engine tests + 15 existing tests from plan 13-01):

```
TestWatcherEngine_Dedup                    PASS (0.39s)
TestWatcherEngine_Stop_NoLeaks             PASS (0.33s)
TestWatcherEngine_KnownSenderRouting       PASS (0.32s)
TestWatcherEngine_UnknownSenderRouting     PASS (0.30s)
TestWatcherEngine_StopCancelsAdapters      PASS (0.16s)
```

`go build`, `go test -race -count=1`, and `go vet` all exit 0.

## Self-Check: PASSED

- All 3 created files exist on disk
- Both commits (faaf023, bfb9bc1) verified in git log
- SUMMARY.md exists at expected path
