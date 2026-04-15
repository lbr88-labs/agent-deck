---
phase: 15-slack-adapter-and-import
verified: 2026-04-16T00:35:00Z
status: passed
score: 7/7
overrides_applied: 0
---

# Phase 15: Slack Adapter + watcher import Verification Report

**Phase Goal:** Ship the Slack adapter (ADAPT-04) that normalizes Slack events arriving via the ntfy bridge into the engine's Event stream with `slack:{CHANNEL_ID}` routing keys, and the `agent-deck watcher import` command (CLI-07) that migrates the legacy bash issue-watcher `channels.json` into per-channel `watcher.toml` files plus a merged `clients.json` with matching `slack:{CHANNEL_ID}` keys. Every claim in this report cites a `path:line` anchor that resolves against the current worktree tree.
**Verified:** 2026-04-16T00:35:00Z
**Status:** passed
**Re-verification:** No, backfill verification closing REQ-WF-2.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `SlackAdapter` implements the `WatcherAdapter` interface declared in `adapter.go`: Setup, Listen, Teardown, HealthCheck all present with matching signatures | VERIFIED | `internal/watcher/adapter.go:26-38` declares `type WatcherAdapter interface` with `Setup(ctx, config) error`, `Listen(ctx, events) error`, `Teardown() error`, `HealthCheck() error`. `SlackAdapter` struct at `internal/watcher/slack.go:22-34`. `Setup` at `internal/watcher/slack.go:57`, `Listen` at `internal/watcher/slack.go:86`, `Teardown` at `internal/watcher/slack.go:216`, `HealthCheck` at `internal/watcher/slack.go:222`. Setup validation test at `internal/watcher/slack_test.go:16` (default server) and `internal/watcher/slack_test.go:46` (missing-topic rejection). |
| 2 | Slack events stream via the ntfy bridge (ADAPT-04 rule): `streamOnce` opens a NDJSON connection against `{server}/{topic}/json`, skipping non-"message" frames, tracking `lastID` for resumption — shared contract with `NtfyAdapter` | VERIFIED | `internal/watcher/slack.go:109` declares `streamOnce(ctx, events)`; stream URL assembly at `internal/watcher/slack.go:114-117`; `bufio.Scanner` NDJSON reader at `internal/watcher/slack.go:134-156` skips frames where `msg.Event != "message"` at `internal/watcher/slack.go:140-142`; `lastID` updated under mutex at `internal/watcher/slack.go:145-147`. Test `TestSlack_Listen_SkipsOpenKeepalive` at `internal/watcher/slack_test.go:343` proves "open" and "keepalive" frames are filtered. |
| 3 | `slack:{CHANNEL_ID}` routing key format — THE CENTRAL ADAPT-04 + REQ-WF-2 routing CLAIM. `normalizeV2` assigns `Sender: fmt.Sprintf("slack:%s", payload.Channel)` so Router.Match pairs it with a `clients.json` entry keyed `slack:{CHANNEL_ID}` | VERIFIED | `internal/watcher/slack.go:181` emits `Sender: fmt.Sprintf("slack:%s", payload.Channel)` inside `normalizeV2` (signature at `internal/watcher/slack.go:173`). Test `TestSlack_Listen_SenderFormat` at `internal/watcher/slack_test.go:128` posts a v2 payload with `Channel: "C0AABSF5GKD"` and asserts the emitted Event's `Sender` equals `"slack:C0AABSF5GKD"` at `internal/watcher/slack_test.go:167-170`. |
| 4 | v1/v2 Slack payload dispatch: `normalizeSlackEvent` tries v2 JSON parse first; v2 success produces a structured Event with deterministic dedup key, v1 fallback produces `Sender: "slack:unknown"` for legacy plain-text bridge payloads | VERIFIED | Dispatcher at `internal/watcher/slack.go:164-170` branches on `json.Unmarshal(...) == nil && payload.V == 2`. v2 path at `internal/watcher/slack.go:173-195` builds `CustomDedupKey: fmt.Sprintf("slack-%s-%s", payload.Channel, payload.TS)` at `internal/watcher/slack.go:186` and `ParentDedupKey: fmt.Sprintf("slack-%s-%s", payload.Channel, payload.ThreadTS)` at `internal/watcher/slack.go:191` when `thread_ts != ts`. v1 path at `internal/watcher/slack.go:198-212` emits `Sender: "slack:unknown"` at `internal/watcher/slack.go:206`. Tests: `TestSlack_Listen_DedupKeyFormat` at `internal/watcher/slack_test.go:178` (asserts `slack-C0AABSF5GKD-1712345678.123456`), `TestSlack_Listen_ThreadReply` at `internal/watcher/slack_test.go:230` (asserts ParentDedupKey), and `TestSlack_Listen_V1Fallback` at `internal/watcher/slack_test.go:285` (asserts `slack:unknown`). |
| 5 | `watcher import` performs an atomic merge of `clients.json` via `os.Rename` of a temp file (CLI-07 rule, threat-model T-15-09): no partial write is ever visible | VERIFIED | `mergeClientsJSON` at `cmd/agent-deck/watcher_cmd.go:666` writes to `CreateTemp` + `Close` then calls `os.Rename(tmpName, clientsPath)` at `cmd/agent-deck/watcher_cmd.go:708`, cleaning up the temp on rename failure at `cmd/agent-deck/watcher_cmd.go:709`. End-to-end behavior covered by `TestImportChannels_EndToEnd` at `cmd/agent-deck/watcher_cmd_test.go:208` (2 channels → 2 watcher.toml files + merged clients.json), `TestMergeClientsJSON_NewFile` at `cmd/agent-deck/watcher_cmd_test.go:111`, `TestMergeClientsJSON_MergeExisting` at `cmd/agent-deck/watcher_cmd_test.go:138`, and `TestMergeClientsJSON_OverwriteExisting` at `cmd/agent-deck/watcher_cmd_test.go:174`. |
| 6 | `watcher import` rejects symlink inputs via `os.Lstat` (threat-model T-15-07) — THE CENTRAL REQ-WF-2 symlink-rejection CLAIM. `os.Lstat` (not `os.Stat`) inspects the symlink itself rather than following it, so a symlinked channels.json cannot bypass path validation | VERIFIED | `importChannels` at `cmd/agent-deck/watcher_cmd.go:719` calls `info, err := os.Lstat(cleanPath)` at `cmd/agent-deck/watcher_cmd.go:723` (`cleanPath` is `filepath.Clean(inputPath)` at `cmd/agent-deck/watcher_cmd.go:721`). The symlink mode check at `cmd/agent-deck/watcher_cmd.go:727` returns `fmt.Errorf("symlink not allowed: %q", cleanPath)` at `cmd/agent-deck/watcher_cmd.go:728`. The directory/irregular-file rejection branch at `cmd/agent-deck/watcher_cmd.go:730-732` returns `"not a regular file"`. Tests: `TestImportChannels_RejectsSymlink` at `cmd/agent-deck/watcher_cmd_test.go:300` creates a real file plus symlink and asserts `importChannels(linkPath, ...)` returns an error; `TestImportChannels_RejectsDirectory` at `cmd/agent-deck/watcher_cmd_test.go:322` asserts a directory path is rejected. |
| 7 | `watcher import` writes `slack:{CHANNEL_ID}` keys into `clients.json` that are byte-identical to the `Sender` format emitted by `SlackAdapter` — closing the router loop between the import writer and the adapter emitter | VERIFIED | `importChannels` builds `clientKey := fmt.Sprintf("slack:%s", channelID)` at `cmd/agent-deck/watcher_cmd.go:772` and writes it into `clients.json` via `mergeClientsJSON` at `cmd/agent-deck/watcher_cmd.go:789`. This string is produced by the same format verb as the `Sender` field at `internal/watcher/slack.go:181` (`fmt.Sprintf("slack:%s", payload.Channel)`) — identical key format on both sides of the router. `TestImportChannels_EndToEnd` at `cmd/agent-deck/watcher_cmd_test.go:208` asserts the loaded `clients.json` contains key `"slack:C0AABSF5GKD"` at `cmd/agent-deck/watcher_cmd_test.go:244`; `TestMergeClientsJSON_NewFile` at `cmd/agent-deck/watcher_cmd_test.go:111` asserts the same key via `loaded["slack:C0AABSF5GKD"]`. `TestImportChannels_Idempotent` at `cmd/agent-deck/watcher_cmd_test.go:259` further asserts that re-running the import produces the same `clients.json` with exactly one entry. |

**Score:** 7/7 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/watcher/slack.go` | SlackAdapter struct, Setup, Listen, streamOnce, normalizeSlackEvent, normalizeV2, normalizeV1, Teardown, HealthCheck | VERIFIED | 243 lines; all exports present; `SlackAdapter` at L22-34, `Setup` L57, `Listen` L86, `streamOnce` L109, `normalizeSlackEvent` L164, `normalizeV2` L173, `normalizeV1` L198, `Teardown` L216, `HealthCheck` L222. |
| `internal/watcher/slack_test.go` | 13 tests covering Setup / v2 / v1 / reconnect / HealthCheck / goleak | VERIFIED | 559 lines; 13 tests enumerated in Observable Truths 1–4 and Behavioral Spot-Checks. |
| `cmd/agent-deck/watcher_cmd.go` | handleWatcher dispatch, handleWatcherImport, parseChannelsJSON, generateWatcherToml, mergeClientsJSON, importChannels | VERIFIED | 816 lines; `handleWatcher` dispatch at L29 (watcher subcommand switch), `handleWatcherImport` entry at L605, `parseChannelsJSON` at L628, `generateWatcherToml` at L644, `mergeClientsJSON` at L666 (atomic rename at L708), `importChannels` at L719 (Lstat at L723, symlink rejection at L728, directory rejection at L730-732, slack:{CHANNEL_ID} key at L772). |
| `cmd/agent-deck/watcher_cmd_test.go` | 13 watcher-import tests: parse valid/invalid/empty/nonexistent, generate TOML, merge new/existing/overwrite, end-to-end, idempotent, symlink rejection, directory rejection, empty channels | VERIFIED | 544 lines; tests at L16 (Valid), L49 (Invalid), L62 (Empty), L81 (Nonexistent), L88 (GenerateToml), L111 (MergeNew), L138 (MergeExisting), L174 (OverwriteExisting), L208 (EndToEnd), L259 (Idempotent), L300 (RejectsSymlink), L322 (RejectsDirectory), L332 (EmptyChannels). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/watcher/slack.go:22-34` SlackAdapter | `internal/watcher/adapter.go:26-38` WatcherAdapter | Interface satisfaction: Setup/Listen/Teardown/HealthCheck all present with matching signatures | WIRED | Setup L57, Listen L86, Teardown L216, HealthCheck L222 all match the interface declarations at adapter.go:28/31/34/37. |
| `cmd/agent-deck/watcher_cmd.go:772` clientKey writer | `internal/watcher/slack.go:181` Sender emitter | Both use `fmt.Sprintf("slack:%s", ...)` producing byte-identical routing keys | WIRED | Identical format verb `"slack:%s"` used at both locations; closes the router loop so `Router.Match(event.Sender)` against a clients.json written by `watcher import` hits the same entry. |
| `cmd/agent-deck/watcher_cmd.go:723` os.Lstat | stdlib `os.Lstat` | Symlink detection primitive (T-15-07 mitigation): Lstat inspects the link itself rather than following it, enabling the mode-bit check at L727 | WIRED | `os.Lstat(cleanPath)` followed by `info.Mode()&os.ModeSymlink != 0` check at L727 and explicit `symlink not allowed` error return at L728. |
| `cmd/agent-deck/watcher_cmd.go:708` os.Rename | stdlib `os.Rename` | Atomic merge primitive (T-15-09 mitigation): temp-file + rename guarantees clients.json never observes a partial write | WIRED | `CreateTemp` then `os.Rename(tmpName, clientsPath)` at `cmd/agent-deck/watcher_cmd.go:708`, with temp cleanup on rename failure. |
| `internal/watcher/slack.go:134-156` NDJSON reader | bridge (Cloudflare Worker → ntfy.sh) | Shared transport path with `NtfyAdapter` per design decision D-02 (independent copy to keep v2 dispatch localized) | WIRED | `bufio.Scanner` over response body, parsing each line as `ntfyMessage`. Tests prove framing via `TestSlack_Listen_SkipsOpenKeepalive` at `internal/watcher/slack_test.go:343`. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Slack adapter tests pass with race detector | `go test ./internal/watcher/... -run TestSlack -race -count=1 -timeout 120s` | `ok  	github.com/asheshgoplani/agent-deck/internal/watcher	1.357s` | PASS |
| Watcher import tests pass with race detector | `go test ./cmd/agent-deck/... -run Watcher -race -count=1 -timeout 120s` | `ok  	github.com/asheshgoplani/agent-deck/cmd/agent-deck	1.041s` | PASS |
| Both affected packages build cleanly | `go build ./internal/watcher/... ./cmd/agent-deck/...` | Exit 0 (no stdout/stderr) | PASS |
| Slack adapter TDD commits exist in git log | `git show --no-patch --format=%H feb50f8 be1d1f3` | `feb50f81309e285d9799839bcf56f960012369dd` (test) followed by `be1d1f330588bf22c8d74ec1ff737677113615a1` (feat) | PASS |
| goleak filter covers go.opencensus (Plan 17-01 carryforward) | `grep go.opencensus internal/watcher/slack_test.go` | `internal/watcher/slack_test.go:506` lists `goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start")` | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| ADAPT-04 | 15-01 | Slack adapter routes via ntfy bridge with thread reply routing (session_id lookup by parent dedup_key) | SATISFIED | Observable Truths 1–4 back this: SlackAdapter implements WatcherAdapter, streams via ntfy, emits `slack:{CHANNEL_ID}` Sender, and sets `ParentDedupKey` for thread replies at `internal/watcher/slack.go:190-192`. |
| CLI-07 | 15-02 | `agent-deck watcher import <path>` migrates existing bash issue-watcher to Go watcher (reads channels.json, generates watcher.toml + clients.json entries) | SATISFIED | Observable Truths 5–7 back this: atomic `os.Rename` merge at `cmd/agent-deck/watcher_cmd.go:708`, `os.Lstat` symlink rejection at `cmd/agent-deck/watcher_cmd.go:723-728`, and matching `slack:{CHANNEL_ID}` key format at `cmd/agent-deck/watcher_cmd.go:772`. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none found) | - | No TODOs, FIXMEs, placeholders, empty implementations, or hardcoded stubs in the shipped Phase 15 source files (`slack.go`, `watcher_cmd.go`) | - | - |

### Human Verification Required

None. All truths are verifiable through code inspection and automated tests. The phase produces adapter + CLI code paths with no UI components and no visual elements requiring human evaluation.

### Gaps Summary

No gaps found. All 7 observable truths pass with path:line evidence. Both requirements (ADAPT-04 + CLI-07) are satisfied. All artifacts exist and are substantive (no stubs). Slack tests (13) and watcher-import tests (13) both exit 0 under `-race -count=1`. Both TDD commits for the SlackAdapter are present in git history (`feb50f8` test → `be1d1f3` feat).

**Observations (informational, not blocking):**
- `generateWatcherToml` at `cmd/agent-deck/watcher_cmd.go:644` emits an empty `topic = ""` with an operator-action comment on the line above because all Slack channels share one ntfy topic at the Cloudflare Worker edge — the operator sets the topic post-import. This is by design (Plan 15-02 key decision) and not a Phase 15 gap.
- The Slack adapter's `streamOnce` maintains the `lastID` cursor under mutex for reconnect resumption but emits with a non-blocking send (`select { case events <- evt: default: }` at `internal/watcher/slack.go:151-155`), so events can be dropped if the engine channel is full. The engine's buffered `eventCh` (capacity 64) absorbs normal bursts; this is consistent with the NtfyAdapter behavior shipped in Plan 14-01.

---

_Verified: 2026-04-16T00:35:00Z_
_Verifier: Ashesh Goplani (REQ-WF-2 backfill)_
