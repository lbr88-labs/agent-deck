# Session ID Lifecycle (Authoritative and Race-Safe)

This document defines the authoritative lifecycle for tool session IDs (Claude, Codex, Gemini) in agent-deck.

## Invariants

1. Disk scans are non-authoritative for identity binding.
2. Session ID binding/rebinding happens only from:
   - tmux environment (`*_SESSION_ID`)
   - hook payload `session_id`
   - hook sidecar anchor (`~/.agent-deck/session-hooks/<instance>.sid`) when payload omits ID
3. Every bind/rebind/reject decision is appended to:
   - `~/.agent-deck/logs/session-id-lifecycle.jsonl`
4. Reject decisions must preserve the currently bound ID.

## Creation and Persistence

1. Session starts with a generated/preselected ID in agent-deck for capture-resume flows.
2. The ID is mirrored into tmux env (`*_SESSION_ID`).
3. Hook anchor sidecar is written so hook updates can be correlated after restart.

## Reconnect / Restart

1. On reconnect/restart, agent-deck reads tmux env and hook updates.
2. If tmux is gone and no hook evidence exists, the last persisted ID remains unchanged.
3. No disk-based reassignment occurs during reconnect/restart/fork/output.

## Fork / Clear / ID Changes

1. `fork` creates a new target ID and binds it through start/resume paths.
2. Tool-driven ID rotation (`/clear` or equivalent) is accepted only when surfaced by tmux/hook evidence.
3. Unknown or invalid candidates are rejected and logged.

## Event Log Schema

Each JSONL entry contains:

- `instance_id`
- `tool`
- `action` (`bind`, `rebind`, `reject`, `scan_disabled`)
- `source` (`tmux_env`, `hook_payload`, `hook_anchor`, `disk_scan`)
- `old_id`, `new_id`, `candidate`
- `reason`
- `hook_event`
- `ts`

## Start / Restart Dispatch

This section was added in v1.5.2 (Phase 3 of the session-persistence hotfix).
It pins the routing rule for Claude session starts introduced by the
2026-04-14 incident fix. Any refactor that violates these invariants
regresses REQ-2 and must include a matching amendment here.

1. `Instance.ClaudeSessionID` in instance JSON storage is the sole authoritative source for the Claude session UUID bound to an agent-deck instance.
2. `Start()`, `StartWithMessage()`, and `Restart()` route through `buildClaudeResumeCommand()` whenever `ClaudeSessionID != ""`. They never mint a new UUID for an instance that already has one.
3. `buildClaudeResumeCommand()` uses `claude --resume <id>` when JSONL evidence exists and `claude --session-id <id>` otherwise. In both cases `<id>` is the stored `ClaudeSessionID` — never a newly minted UUID.
4. Disk scans of `~/.claude/projects/<hash>/*.jsonl` remain non-authoritative. `sessionHasConversationData()` is a presence check, NOT an identity probe.

### Enforcement

The following tests in `internal/session/session_persistence_test.go` pin these invariants and are part of the CLAUDE.md mandate gate:

- `TestPersistence_RestartResumesConversation` — invariant 2 + 3 (Restart branch).
- `TestPersistence_StartAfterSIGKILLResumesConversation` — invariant 2 + 3 (Start branch).
- `TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion` — invariant 1.
- `TestPersistence_FreshSessionUsesSessionIDNotResume` — invariant 3 (no-JSONL fallback).
- `TestPersistence_ClaudeSessionIDPreservedThroughStopError` — invariant 1 + 2 (no implicit clear on status transitions).
- `TestPersistence_SessionIDFallbackWhenJSONLMissing` — invariant 3 + 4 (no disk-scan rebind when stored ID's JSONL is absent).
- `TestPersistence_ResumeLogEmitted_*` (three variants) — OBS-02 audit line records every dispatch decision.

A PR that removes any of the eight CLAUDE.md-mandated tests above, or that adds a code path which assigns `i.ClaudeSessionID = ""` outside `Fork()` (target-side) or `Delete()`, violates this section and requires an RFC.
