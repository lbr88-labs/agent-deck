---
phase: 15-slack-adapter-and-import
plan: 01
subsystem: watcher
tags: [slack, adapter, ntfy-bridge, channel-routing, event-normalization, backfill]
backfill: true

# Dependency graph
requires:
  - phase: 13-watcher-engine-core
    provides: "WatcherAdapter interface, Event struct, AdapterConfig"
  - phase: 14-simple-adapters-webhook-ntfy-github
    plan: 01
    provides: "NtfyAdapter streaming and exponential-backoff reconnect pattern (SlackAdapter reuses)"
provides:
  - "SlackAdapter implementing WatcherAdapter via the ntfy bridge"
  - "slack:{CHANNEL_ID} Sender format for routing via clients.json"
  - "v1/v2 Slack payload normalization with deterministic dedup keys"
  - "Thread reply detection (ParentDedupKey) for session lookup"
affects: [15-02 (watcher import reuses slack:{CHANNEL_ID} key format), 16-watcher-cli]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "NDJSON stream over ntfy bridge (independent copy of NtfyAdapter's streaming core per D-02)"
    - "v1/v2 payload dispatcher keyed on JSON-parse success + version field"
    - "slack:{CHANNEL_ID} Sender format for router compatibility"
    - "slack-{CHANNEL}-{TS} CustomDedupKey for deterministic dedup"

key-files:
  created:
    - internal/watcher/slack.go
    - internal/watcher/slack_test.go
  modified: []

key-decisions:
  - "Route via the existing ntfy bridge rather than native Slack Socket Mode — the Cloudflare Worker ntfy bridge already forwards Slack Events API events, so the adapter re-uses the NtfyAdapter NDJSON streaming contract (ADAPT-04)"
  - "Sender format `slack:{CHANNEL_ID}` so clients.json can route by channel ID without additional metadata; the Wave 2 `watcher import` command writes the matching key format"
  - "Independent copy of ntfy streaming core rather than embedding NtfyAdapter (D-02) to keep the Slack-specific v2 payload dispatch and thread detection localized"
  - "v1/v2 payload dispatcher keeps legacy Slack Events API v1 format working while new installations emit v2 structured envelopes via the Cloudflare Worker"
  - "Thread replies carry ParentDedupKey `slack-{CHANNEL}-{THREAD_TS}` so the engine writerLoop can look up the parent's session_id and reuse the session"

requirements-completed: [ADAPT-04]

# Metrics
duration: ~2min (between the two TDD commits in git log)
completed: 2026-04-10
---

# Phase 15 Plan 01: Slack Adapter Summary (backfill)

**SlackAdapter implementing WatcherAdapter via the ntfy bridge: v1/v2 Slack payload normalization with `slack:{CHANNEL_ID}` Sender format, `slack-{CHANNEL}-{TS}` deterministic dedup keys, and thread reply detection for session routing (ADAPT-04).**

## Performance

- **Duration:** ~2 min (delta between commits `feb50f8` at 2026-04-10T14:53:25Z and `be1d1f3` at 2026-04-10T14:55:02Z)
- **Started:** 2026-04-10T14:53:25Z (RED commit `feb50f8`)
- **Completed:** 2026-04-10T14:55:02Z (GREEN commit `be1d1f3`)
- **Tasks:** 2 (TDD RED+GREEN pairs, compressed into two atomic commits)
- **Files created:** 2 (`internal/watcher/slack.go`, `internal/watcher/slack_test.go`)

## Accomplishments

- SlackAdapter struct declared at `internal/watcher/slack.go:22-34` holding ntfy server URL, topic, HTTP client, last-seen message ID cursor, and configurable exponential-backoff durations.
- `Setup` validates `Settings["topic"]`, defaults server to `https://ntfy.sh`, and trims trailing slash at `internal/watcher/slack.go:57-81`.
- `Listen` wraps `streamOnce` in an exponential-backoff reconnect loop (2s initial, 2x factor, 30s cap) at `internal/watcher/slack.go:86-105`; returns only when context is cancelled.
- `streamOnce` opens one NDJSON stream against `{server}/{topic}/json[?since=lastID]`, skips non-"message" ntfy frames ("open"/"keepalive"), tracks `lastID` for resumption, and dispatches each frame through `normalizeSlackEvent` at `internal/watcher/slack.go:109-159`.
- `normalizeSlackEvent` dispatches to `normalizeV2` when the message body parses as JSON with `V == 2`, otherwise `normalizeV1`, at `internal/watcher/slack.go:164-170`.
- v2 payload handler emits `Sender: fmt.Sprintf("slack:%s", payload.Channel)` at `internal/watcher/slack.go:181` — this is the central ADAPT-04 routing claim that pairs with the `watcher import` writer key format.
- v2 handler emits deterministic `CustomDedupKey` of `slack-{CHANNEL}-{TS}` at `internal/watcher/slack.go:186`, so re-delivery of the same Slack message produces the same dedup key.
- v2 handler detects thread replies (`thread_ts != ts`) and emits `ParentDedupKey` of `slack-{CHANNEL}-{THREAD_TS}` at `internal/watcher/slack.go:190-192`, so the engine can look up the parent's session_id for thread routing (ADAPT-04 rule).
- v1 legacy handler falls back to `Sender: "slack:unknown"` at `internal/watcher/slack.go:206` when the payload is plain text rather than structured v2 JSON.
- `Teardown()` is a no-op at `internal/watcher/slack.go:216-218` because the streaming HTTP body is closed by context cancellation in `Listen`.
- `HealthCheck()` issues a 5s-timeout HEAD request against the ntfy server at `internal/watcher/slack.go:222-242`, returning non-nil on transport error or non-2xx status.
- 13 tests in `internal/watcher/slack_test.go` cover every branch above and all pass with `-race`: `TestSlack_Setup_DefaultServer` at `internal/watcher/slack_test.go:16`, `TestSlack_Setup_CustomServer` at `internal/watcher/slack_test.go:31`, `TestSlack_Setup_MissingTopic` at `internal/watcher/slack_test.go:46`, `TestSlack_Listen_V2Payload` at `internal/watcher/slack_test.go:72`, `TestSlack_Listen_SenderFormat` at `internal/watcher/slack_test.go:128` (asserts Sender equals `slack:C0AABSF5GKD`), `TestSlack_Listen_DedupKeyFormat` at `internal/watcher/slack_test.go:178`, `TestSlack_Listen_ThreadReply` at `internal/watcher/slack_test.go:230`, `TestSlack_Listen_V1Fallback` at `internal/watcher/slack_test.go:285` (asserts Sender equals `slack:unknown`), `TestSlack_Listen_SkipsOpenKeepalive` at `internal/watcher/slack_test.go:343`, `TestSlack_Listen_ReconnectsOnDisconnect` at `internal/watcher/slack_test.go:407`, `TestSlack_HealthCheck_Reachable` at `internal/watcher/slack_test.go:467`, `TestSlack_HealthCheck_Unreachable` at `internal/watcher/slack_test.go:485`, and `TestSlack_Listen_StopNoLeaks` at `internal/watcher/slack_test.go:498` (goleak-bounded).

## Task Commits

Each TDD phase was committed atomically in git log order:

1. **Task 1 (RED): Add failing tests for SlackAdapter** — `feb50f8` (`test(15-01): add failing tests for SlackAdapter`, Ashesh Goplani, 2026-04-10 16:53:25 +0200)
2. **Task 1+2 (GREEN): Implement SlackAdapter with v2 payload parsing and thread routing** — `be1d1f3` (`feat(15-01): implement SlackAdapter with v2 payload parsing and thread routing`, Ashesh Goplani, 2026-04-10 16:55:02 +0200)

_Note: The commit history shows a single combined GREEN commit for both TDD tasks in this PLAN rather than a Task 1 GREEN / Task 2 RED+GREEN split. This is recorded as-is for backfill accuracy; shipped behavior is the source of truth._

## Files Created/Modified

- `internal/watcher/slack.go` (243 lines) — SlackAdapter with exports: `SlackAdapter` struct, `Setup`, `Listen`, `Teardown`, `HealthCheck`. Internal: `slackV2Payload` struct, `streamOnce`, `normalizeSlackEvent`, `normalizeV2`, `normalizeV1`.
- `internal/watcher/slack_test.go` (559 lines) — 13 tests covering Setup validation (3), v2 payload handling (4: base parse, Sender format, CustomDedupKey format, thread reply), v1 fallback (1), stream frame filtering (1), reconnect (1), HealthCheck (2), goleak-bounded stop (1).

## Decisions Made

- Route via the existing ntfy bridge (Cloudflare Worker forwards Slack Events API to an ntfy topic) rather than native Slack Socket Mode. The bridge already handled bash issue-watcher traffic in production, so reusing it keeps the event path singular.
- Independent copy of the NDJSON streaming core rather than embedding NtfyAdapter (design decision D-02). The duplication is small (~50 lines of `streamOnce`) and keeps the Slack-specific v2 dispatch + thread detection co-located with the type it belongs to.
- `Sender: slack:{CHANNEL_ID}` format chosen so `clients.json` can route by channel ID alone, with no additional metadata. The Wave 2 `watcher import` command writes the matching key format at `cmd/agent-deck/watcher_cmd.go:772`.
- Deterministic `CustomDedupKey: slack-{CHANNEL}-{TS}` so retried ntfy deliveries of the same Slack message produce the same dedup key and are rejected by the engine's `INSERT OR IGNORE` path in `statedb.SaveWatcherEvent`.
- Thread replies emit `ParentDedupKey` (matching the parent message's dedup key) so the engine's `writerLoop` can look up the parent's session_id and reuse the session (D-07).

## Deviations from Plan

Retrospective backfill; the PLAN (`15-01-PLAN.md`) was reconstructed after
the code shipped as part of REQ-WF-2 closure, so deviations cannot be
meaningfully recorded. Shipped behavior is the source of truth.

## Issues Encountered

None known at backfill time.

## User Setup Required

None. The ntfy bridge reuses the existing Cloudflare Worker topic, so no
new external account is required. The operator sets the topic in the
watcher's `watcher.toml` under `[source]`.

## Next Phase Readiness

- Plan 15-02 (`watcher import`) uses the same `slack:{CHANNEL_ID}` key
  format in its `clients.json` merge path at
  `cmd/agent-deck/watcher_cmd.go:772`. The loop closes:
  `watcher import` writes the key, SlackAdapter emits events with the
  matching `Sender` format, and Router matches them against the entry.
- Phase 16 (Watcher CLI + TUI) inherits the `SlackAdapter` type and can
  register it alongside webhook/ntfy/github/gmail adapters via the engine's
  `RegisterAdapter` path.

---
*Phase: 15-slack-adapter-and-import*
*Completed: 2026-04-10*
*Reconstructed: 2026-04-16T00:35:00Z as part of REQ-WF-2 backfill*
