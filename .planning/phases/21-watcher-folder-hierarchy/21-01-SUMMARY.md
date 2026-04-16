---
phase: 21-watcher-folder-hierarchy
plan: 01
subsystem: watcher
tags: [tdd, filesystem, migration, security, cli]
dependency_graph:
  requires: [phase-20-health-bridge]
  provides: [watcher-folder-hierarchy, singular-watcher-dir, per-watcher-state, event-log]
  affects: [internal/watcher, internal/session, cmd/agent-deck]
tech_stack:
  added: []
  patterns: [write-temp-rename, O_APPEND-atomic, go-embed, os.Lstat-symlink-guard, O_CREATE-O_EXCL-idempotent]
key_files:
  created:
    - internal/watcher/layout.go
    - internal/watcher/state.go
    - internal/watcher/event_log.go
    - internal/watcher/layout_test.go
    - internal/watcher/assets/watcher-templates/CLAUDE.md
    - internal/watcher/assets/watcher-templates/POLICY.md
    - internal/watcher/assets/watcher-templates/LEARNINGS.md
    - assets/watcher-templates/CLAUDE.md
    - assets/watcher-templates/POLICY.md
    - assets/watcher-templates/LEARNINGS.md
  modified:
    - internal/watcher/engine.go
    - internal/session/watcher_meta.go
    - cmd/agent-deck/watcher_cmd.go
    - cmd/agent-deck/watcher_cmd_test.go
decisions:
  - "hs.WatcherName from env.tracker.Check() used in writerLoop (watcherName not directly on eventEnvelope)"
  - "classifyFromState kept in-sync with health.go thresholds via code comment (no circular import)"
  - "Placeholder embed files created in Task B to satisfy //go:embed at compile time; replaced with full content in Task C"
metrics:
  duration: ~35 min
  completed: 2026-04-16
  tasks_completed: 4
  files_modified: 13
---

# Phase 21 Plan 01: Watcher Folder Hierarchy Summary

**One-liner:** Atomic `~/.agent-deck/watchers/ -> watcher/` migration with per-watcher `state.json`/`task-log.md` persistence and `list --json` health fields via `os.Lstat`-guarded T-21-SL symlink refusal and T-21-PI name regex.

## Task Commits

| Task | Commit | Type | Description |
|------|--------|------|-------------|
| A (RED) | `e578bfc` | test | 8 failing tests + sub-tests; compile stubs |
| B (GREEN) | `e329636` | feat | Full implementation: layout.go, state.go, event_log.go, engine.go, session/watcher_meta.go, watcher_cmd.go |
| C (Templates) | `8ee7080` | docs | 3 template files (canonical + embedded), byte-identical pairs |
| D (CLI) | `dcbc6eb` | feat | watcherListEntry extended; populateStateFields; TestWatcherList_JSON_ExposesStateFields |

## What Was Built

### New Source Files

**`internal/watcher/layout.go`** — The layout anchor:
- `LayoutDir()` delegates to `session.WatcherDir()` (single source of truth)
- `WatcherDir(name)` validates via `watcherNameRegex` (`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`) before composing path (T-21-PI)
- `ScaffoldWatcherLayout()` uses `O_CREATE|O_EXCL` to idempotently write 4 shared files
- `MigrateLegacyWatchersDir()` uses `os.Lstat` on both `legacy` and `current` paths, refuses if `current` is a symlink targeting outside `~/.agent-deck/` (T-21-SL), then atomically renames and creates relative `watchers -> watcher` symlink

**`internal/watcher/state.go`** — Per-watcher state persistence:
- `WatcherState` struct: `LastEventTS`, `ErrorCount`, `AdapterHealthy`, `HealthWindow`, `DedupCursor`
- `SaveState(name, s)`: atomic write-temp-rename (copied from `internal/session SaveWatcherMeta` pattern)
- `LoadState(name)`: returns `(nil, nil)` on missing file (hot-reload safe: no in-process cache)

**`internal/watcher/event_log.go`** — Append-only event log:
- `AppendEventLog(name, entry)`: `O_APPEND|O_CREATE` writer; entries <= 512 bytes for POSIX atomicity

### Modified Files

**`internal/session/watcher_meta.go`**: `WatcherDir()` return value flipped from `"watchers"` to `"watcher"`. All callers flip automatically.

**`internal/watcher/engine.go`**:
- `cfg.ClientsPath` default changed to `~/.agent-deck/watcher/clients.json`
- `Start()` now calls `MigrateLegacyWatchersDir()` + `ScaffoldWatcherLayout()` before adapter setup (non-fatal: log and continue)
- `writerLoop` `if inserted` branch now calls `AppendEventLog` + `SaveState` after `RecordEvent()`. Watcher name obtained via `env.tracker.Check().WatcherName`. Failures log-and-continue (never drop events). `health.go` is untouched.

**`cmd/agent-deck/watcher_cmd.go`**: `watcherListEntry` promoted to package-level type with 3 new fields (`LastEventTS *time.Time`, `ErrorCount int`, `HealthStatus string`). `populateStateFields()` and `classifyFromState()` helpers added. Error text at line 524 flipped to singular `watcher/clients.json`.

**`cmd/agent-deck/watcher_cmd_test.go`**: `TestWatcherList_JSON_ExposesStateFields` with 4 sub-tests (healthy/warning/error/fresh).

## Tests

| Suite | Count | Status |
|-------|-------|--------|
| `./internal/watcher/` `-run TestLayout` | 8 top-level + sub-tests | GREEN (race) |
| `./internal/watcher/...` (full) | 127+ | GREEN (race) |
| `./cmd/agent-deck/...` (full) | all | GREEN (race) |

## Security Mitigations Shipped

| Threat | ID | Mitigation | Test |
|--------|-----|-----------|------|
| Symlink traversal on `~/.agent-deck/watcher` pre-created by adversary | T-21-SL | `os.Lstat` + `os.Readlink` + `filepath.Abs` prefix check; refuses with `"refusing migration: ..."` error | `TestLayout_LegacyMigrationAtomic/symlink_attack` |
| Path traversal via watcher name containing `..` or `/` | T-21-PI | `watcherNameRegex` `^[a-zA-Z0-9][a-zA-Z0-9._-]*$` in `validateWatcherName` called from `WatcherDir(name)` | `TestLayout_WatcherDir_RejectsMaliciousNames` |
| TOCTOU on `ScaffoldWatcherLayout` template writes | T-21-RC | `O_CREATE|O_EXCL` in `writeIfAbsent` | `TestLayout_FreshInstallCreatesLayout` (correctness) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `env.config.Name` does not exist on `eventEnvelope`**
- **Found during:** Task B sub-step 5d
- **Issue:** Plan's code snippet referenced `env.config.Name` but `eventEnvelope` struct only has `event`, `watcherID`, `tracker` — no `config` field.
- **Fix:** Called `env.tracker.Check()` first (already needed for health snapshot) and used `hs.WatcherName` throughout the persistence block.
- **Files modified:** `internal/watcher/engine.go`
- **Commit:** `e329636`

**2. [Rule 1 - Bug] gofmt formatting issues on first commit attempt**
- **Found during:** Task B commit (pre-commit hook)
- **Issue:** `layout.go` had minor formatting difference flagged by `gofmt`.
- **Fix:** `gofmt -w internal/watcher/layout.go` then re-staged.
- **Commit:** `e329636` (after reformat)

**3. [Rule 1 - Bug] gofmt formatting issues on Task D commit attempt**
- **Found during:** Task D commit (pre-commit hook)
- **Issue:** `watcher_cmd.go` struct field alignment difference.
- **Fix:** `gofmt -w cmd/agent-deck/watcher_cmd.go` then re-staged.
- **Commit:** `dcbc6eb` (after reformat)

## TDD Gate Compliance

- RED gate: `test(21-01)` commit `e578bfc` — 8 test functions, all failing with "not implemented (RED)"
- GREEN gate: `feat(21-01)` commit `e329636` — all 8 tests pass under `-race`
- REFACTOR gate: not needed (code was clean after GREEN)

## Known Stubs

None. All exported functions are fully implemented. Template files contain substantive content (no placeholders remain after Task C).

## Threat Flags

None. All new filesystem paths are within `~/.agent-deck/` and gated by the existing `GetAgentDeckDir()` primitive. The two explicit threat mitigations (T-21-SL, T-21-PI) are implemented and tested.

## Self-Check: PASSED

| Check | Result |
|-------|--------|
| `internal/watcher/layout.go` exists | FOUND |
| `internal/watcher/state.go` exists | FOUND |
| `internal/watcher/event_log.go` exists | FOUND |
| `internal/watcher/layout_test.go` exists | FOUND |
| `assets/watcher-templates/CLAUDE.md` exists | FOUND |
| commit `e578bfc` (RED) exists | FOUND |
| commit `e329636` (GREEN) exists | FOUND |
| commit `8ee7080` (templates) exists | FOUND |
| commit `dcbc6eb` (CLI) exists | FOUND |
