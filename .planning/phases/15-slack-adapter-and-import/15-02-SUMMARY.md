---
phase: 15-slack-adapter-and-import
plan: 02
subsystem: cli
tags: [watcher, import, channels-json, slack, toml, cli]

# Dependency graph
requires:
  - phase: 13-watcher-engine-core
    provides: "watcher.ClientEntry struct, LoadClientsJSON, Router for clients.json format"
  - phase: 14-simple-adapters-webhook-ntfy-github
    provides: "Adapter pattern and WatcherMeta/WatcherDir from session package"
provides:
  - "'agent-deck watcher import <path>' CLI command"
  - "channels.json parser for bash issue-watcher migration"
  - "watcher.toml generation per channel"
  - "clients.json merge with slack:{CHANNEL_ID} keys"
  - "handleWatcher dispatch skeleton for future watcher subcommands"
affects: [16-watcher-cli-commands, slack-adapter]

# Tech tracking
tech-stack:
  added: []
  patterns: ["watcher subcommand dispatch (handleWatcher + switch)", "atomic JSON write via temp+rename", "Lstat symlink rejection for user-provided paths"]

key-files:
  created:
    - cmd/agent-deck/watcher_cmd.go
    - cmd/agent-deck/watcher_cmd_test.go
  modified:
    - cmd/agent-deck/main.go

key-decisions:
  - "Used Lstat for symlink detection on input path per threat model T-15-07"
  - "Atomic write (temp file + rename) for clients.json per T-15-09"
  - "Empty ntfy topic with TODO comment since all Slack channels share one Cloudflare Worker topic"
  - "Sorted channel IDs for deterministic output across runs"

patterns-established:
  - "Watcher subcommand dispatch: handleWatcher(profile, args) with switch on args[0]"
  - "Import command extracts testable pure functions: parseChannelsJSON, generateWatcherToml, mergeClientsJSON, importChannels"
  - "clients.json key format for Slack: slack:{CHANNEL_ID}"

requirements-completed: [CLI-07]

# Metrics
duration: 4min
completed: 2026-04-10
---

# Phase 15 Plan 02: Watcher Import Command Summary

**CLI command `agent-deck watcher import` that migrates bash issue-watcher channels.json to Go watcher config (watcher.toml per channel + clients.json with slack:{CHANNEL_ID} routing keys)**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-10T14:48:55Z
- **Completed:** 2026-04-10T14:52:32Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Watcher subcommand dispatch with handleWatcher following conductor_cmd.go pattern
- Import command reads bash issue-watcher channels.json and generates watcher.toml per channel under ~/.agent-deck/watchers/{prefix}/
- clients.json merge with slack:{CHANNEL_ID} keys using atomic write (temp+rename)
- Security: Lstat-based symlink and directory rejection on user-provided input path
- 13 tests covering parsing, generation, merging, end-to-end, idempotency, and security

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Add failing tests for watcher import** - `a101660` (test)
2. **Task 1 (GREEN): Implement watcher import command** - `de88658` (feat)

_Note: TDD tasks combined since implementation and tests were developed in RED-GREEN cycle across the two plan tasks._

## Files Created/Modified
- `cmd/agent-deck/watcher_cmd.go` - Watcher subcommand dispatch + import implementation (handleWatcher, handleWatcherImport, parseChannelsJSON, generateWatcherToml, mergeClientsJSON, importChannels)
- `cmd/agent-deck/watcher_cmd_test.go` - 13 tests: parse valid/invalid/empty/nonexistent, generate TOML, merge new/existing/overwrite, end-to-end, idempotent, symlink rejection, directory rejection, empty channels
- `cmd/agent-deck/main.go` - Added `case "watcher"` routing to handleWatcher

## Decisions Made
- Used Lstat (not Stat) for input path validation to detect symlinks before following them, per threat model T-15-07
- Atomic write pattern (CreateTemp + Write + Close + Rename) for clients.json to prevent partial writes on crash, per T-15-09
- Set ntfy topic to empty string with TODO comment since all Slack channels share one Cloudflare Worker topic; user must set manually
- Sorted channel IDs alphabetically for deterministic output across runs (idempotency)
- Extracted core logic into pure functions (parseChannelsJSON, generateWatcherToml, mergeClientsJSON, importChannels) for testability

## Deviations from Plan

None. Plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None. No external service configuration required.

## Next Phase Readiness
- Watcher CLI foundation is ready for Phase 16 (remaining subcommands: create, start, stop, list, status, test, routes)
- The handleWatcher switch statement is pre-wired with help/default cases; adding new subcommands is a single case addition
- clients.json format established with slack:{CHANNEL_ID} keys, compatible with existing Router.Match()

---
*Phase: 15-slack-adapter-and-import*
*Completed: 2026-04-10*
