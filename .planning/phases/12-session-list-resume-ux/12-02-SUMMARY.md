---
phase: 12-session-list-resume-ux
plan: 02
status: complete
completed: "2026-03-13"
---

## Summary

Added in-memory dedup call at the `sessionRestartedMsg` handler and created a concurrent storage write integration test proving SQLite WAL-mode safety.

## What Changed

- **DEDUP-01 (verified):** `Restart()` already reuses the existing `*Instance` in place; no new row is ever created
- **DEDUP-02:** `UpdateClaudeSessionsWithDedup(h.instances)` now runs under `instancesMu` lock in the `sessionRestartedMsg` handler, before `saveInstances()`, mirroring the existing pattern in `sessionCreatedMsg` (~line 2864)
- **DEDUP-03:** New integration test proves two `Storage` instances writing the same SQLite file concurrently produce consistent dedup results under the race detector

## Key Files

### Created
- `internal/session/storage_concurrent_test.go` — `TestConcurrentStorageWrites`: concurrent WAL-mode write safety with dedup semantics

### Modified
- `internal/ui/home.go` — Added `session.UpdateClaudeSessionsWithDedup(h.instances)` in `sessionRestartedMsg` success path before `saveInstances()`

## Commits
- `31b5029 feat(12-02): add in-memory dedup at sessionRestartedMsg handler`
- `2e4be3c test(12-02): add concurrent storage write integration test (DEDUP-03)`

## Deviations

None.
