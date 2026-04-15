---
gsd_state_version: 1.0
milestone: v1.5.2
milestone_name: milestone
status: executing
stopped_at: "Phase 04 fully landed. v1.5.2 milestone complete. Next step: user sign-off + manual SSH-logout verification per milestone criterion #2."
last_updated: "2026-04-15T07:04:58.324Z"
last_activity: 2026-04-15 -- Phase 5 planning complete
progress:
  total_phases: 5
  completed_phases: 3
  total_plans: 20
  completed_plans: 16
  percent: 80
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-14)

**Core value:** After v1.5.2, SSH logout on Linux+systemd must not kill any agent-deck tmux server, and restarting any dead session must resume the prior Claude conversation — both permanently test-gated.
**Current focus:** v1.5.2 milestone complete on branch fix/session-persistence — awaiting user sign-off (no push/tag/PR allowed).

## Current Position

Phase: 04 (verification-harness-docs-and-ci-wiring) — COMPLETE
Plan: 4 of 4
Status: Ready to execute
Last activity: 2026-04-15 -- Phase 5 planning complete

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

### Pending Todos

None yet.

### Blockers/Concerns

None yet. Spec is authoritative; requirements are atomic and testable; CLAUDE.md mandate section already exists at commit a262c6d and will be audited in Phase 4.

## Session Continuity

Last session: 2026-04-15 — Phase 04 execution complete
Stopped at: Phase 04 fully landed. v1.5.2 milestone complete. Next step: user sign-off + manual SSH-logout verification per milestone criterion #2.
Resume file: None
