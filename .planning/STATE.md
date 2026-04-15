---
gsd_state_version: 1.0
milestone: v1.5.2
milestone_name: milestone
status: executing
stopped_at: "Phase 05 fully landed (REQ-7 custom-command JSONL resume). TEST-09 GREEN + new portable unit test. Visual verify-session-persistence.sh still OVERALL PASS. v1.5.2 milestone complete. Next step: user sign-off + manual SSH-logout verification per milestone criterion #2."
last_updated: "2026-04-15T20:00:00.000Z"
last_activity: 2026-04-15 -- Phase 5 executed (waves 1-3) on post-update GSD v1.36.0
progress:
  total_phases: 5
  completed_phases: 5
  total_plans: 23
  completed_plans: 23
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-14)

**Core value:** After v1.5.2, SSH logout on Linux+systemd must not kill any agent-deck tmux server, and restarting any dead session must resume the prior Claude conversation — both permanently test-gated.
**Current focus:** v1.5.2 milestone complete on branch fix/session-persistence — awaiting user sign-off (no push/tag/PR allowed).

## Current Position

Phase: 05 (custom-command-jsonl-resume) — COMPLETE
Plan: 3 of 3
Status: Milestone v1.5.2 complete; awaiting user sign-off
Last activity: 2026-04-15 -- Phase 5 waves 1-3 executed, TEST-09 GREEN, portable unit test added

Progress: [██████████] 100%

## Performance Metrics

**Velocity:**

- Total plans completed: 0
- Average duration: — min
- Total execution time: 0.0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 1. Persistence test scaffolding (RED) | 2/2 | — | — |
| 2. Cgroup isolation default (REQ-1 fix) | 6/6 | — | — |
| 3. Resume-on-start and error-recovery (REQ-2 fix) | 5/5 | — | — |
| 4. Verification harness, docs, and CI wiring | 4/4 | — | — |
| 5. Custom-command JSONL resume (REQ-7 fix) | 3/3 | — | — |

**Recent Trend:**

- Last 5 plans: —
- Trend: —

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Default `launch_in_user_scope=true` on Linux+systemd without a wizard prompt — silent runtime default, explicit opt-out still honored.
- No config auto-upgrade rewriting `~/.agent-deck/config.toml` — runtime-only default is sufficient.
- Gate every PR on the eight `TestPersistence_*` tests + `scripts/verify-session-persistence.sh` via the CLAUDE.md mandate — third recurrence of the same incident class, per-PR hard gate is the only prevention.
- Do not migrate the 33 error / 39 stopped sessions on the conductor host — separate manual operator task.
- Do not resume the legacy v15 roadmap in `.planning.legacy-v15/` — out of scope per PROJECT.md.
- Phase 03: routed Start() and StartWithMessage() through buildClaudeResumeCommand when ClaudeSessionID != "" — closed the 2026-04-14 f1e103df/b9403638 divergence. OBS-02 per-call audit line landed. docs/session-id-lifecycle.md gained a Start / Restart Dispatch subsection (PERSIST-10).
- Phase 05: added `discoverLatestClaudeJSONL` pure helper in claude.go + `ensureClaudeSessionIDFromDisk` prelude on Instance at two dispatch sites (Start / StartWithMessage). Empty-ID Claude-compatible starts now write-through the newest UUID JSONL before spawn — retires the 2026-04-15 conductor `start-conductor.sh` wrapper hack. TEST-09 GREEN + host-portable unit test covers PERSIST-11..13 / D-04 no-branch-on-Command / D-05 no-5-minute-cap.

### Pending Todos

None yet.

### Blockers/Concerns

None yet. Spec is authoritative; requirements are atomic and testable; CLAUDE.md mandate section already exists at commit a262c6d and will be audited in Phase 4.

## Session Continuity

Last session: 2026-04-15 — Phase 05 execution complete (waves 1-3)
Stopped at: Phase 05 fully landed (REQ-7 custom-command JSONL resume). TEST-09 GREEN + portable unit test added. scripts/verify-session-persistence.sh still OVERALL PASS. v1.5.2 milestone complete end-to-end. Next step: user sign-off + manual SSH-logout verification per milestone criterion #2.
Resume file: None
Commits (Phase 05): 7f2ff35 (05-01 RED), d1590d6 (05-02 GREEN), 108132a (05-03 portable unit test)
