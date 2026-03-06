---
gsd_state_version: 1.0
milestone: v1.2
milestone_name: Conductor Reliability & Learnings Cleanup
status: completed
stopped_at: Milestone v1.2 complete (all 10 phases, 21 plans)
last_updated: "2026-03-06T22:44:44.432Z"
last_activity: 2026-03-07 -- Completed 10-02 LEARNINGS.md cleanup; all 10 phases and 21 plans complete
progress:
  total_phases: 10
  completed_phases: 10
  total_plans: 21
  completed_plans: 21
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-07)

**Core value:** Conductor orchestration and cross-session coordination must work reliably in production
**Current focus:** Milestone v1.2 COMPLETE -- all 10 phases finished

## Current Position

Phase: 10 of 10 (Learnings Promotion) -- COMPLETE
Plan: 2 of 2 in current phase (10-01 complete, 10-02 complete)
Status: All phases complete
Last activity: 2026-03-07 -- Completed 10-02 LEARNINGS.md cleanup; all 10 phases and 21 plans complete

Progress: [##########] 100% (all 10 phases, 21 plans complete)

## Accumulated Context

### Decisions

- [v1.0]: 3 phases (skills reorg, testing, stabilization), all completed
- [v1.0]: TestMain files in all test packages force AGENTDECK_PROFILE=_test
- [v1.1]: Architecture first approach for test framework
- [v1.1]: Integration tests use real tmux but simple commands (echo, sleep, cat), not real AI tools
- [v1.2 init]: Skip codebase mapping, CLAUDE.md already has comprehensive architecture docs
- [v1.2 init]: GSD conductor goes to pool, not built-in (only needed in conductor contexts)
- [v1.2 roadmap]: Send reliability (Phase 7) before heartbeat/CLI (Phase 8) to fix highest-impact bugs first
- [v1.2 roadmap]: Process stability (Phase 9) after send fixes to isolate exit 137 root cause
- [v1.2 roadmap]: Learnings promotion (Phase 10) last so docs capture findings from all code phases
- [v1.2 07-01]: Consolidated 7 duplicated prompt detection functions into internal/send package
- [v1.2 07-01]: Codex readiness uses existing PromptDetector for consistency with detector.go patterns
- [v1.2 07-01]: Enter retry hardened to every-iteration for first 5, then every-2nd (was every-3rd)
- [Phase 07-02]: Integration tests verify tmux primitives, not cmd-level wrappers (not importable)
- [Phase 07-02]: Shell script fixtures in t.TempDir simulate tool startup delay for integration tests
- [Phase 08-01]: interval=0 means disabled (returns 0), negative means use default 15
- [Phase 08-01]: Heartbeat script checks conductor enabled status via JSON before sending
- [Phase 08-01]: TUI clear-on-compact heartbeat also updated to group-scoped message
- [Phase 08-02]: 5 consecutive GetStatus errors threshold for session death detection
- [Phase 08-02]: Return ("error", nil) on session death so exit code 1 via existing logic
- [Phase 09-01]: Exit 137 root cause: Claude Code kills Bash tool children on new PTY input, not tmux or agent-deck
- [Phase 09-01]: Not fixable in agent-deck: tmux send-keys (only channel) is indistinguishable from human typing
- [Phase 09-01]: Primary mitigation (waitForAgentReady status gating) already implemented in Phase 7
- [Phase 09-02]: Exit 137 mitigation documented in conductor CLAUDE.md (between Heartbeat Protocol and State Management)
- [Phase 09-02]: GSD-specific exit 137 guidance added to gsd-conductor SKILL.md (before GSD Lifecycle)
- [Phase 09-02]: Emphasized session output as safe read-only alternative to sending messages to running sessions
- [Phase 10-01]: Orchestration Best Practices placed between Exit 137 and State Management in conductor CLAUDE.md
- [Phase 10-01]: Universal patterns grouped into Monitoring, Sending, Session Management, and Task Design subsections
- [Phase 10-01]: GSD Claude-only constraint added as prominent callout at top of gsd-conductor SKILL.md
- [Phase 10-01]: Stage 0 (codebase mapping) added to GSD lifecycle for brownfield projects
- [Phase 10-01]: Codex troubleshooting items added to existing Troubleshooting section of agent-deck-workflow SKILL.md
- [Phase 10]: Used blockquote format for opengraphdb entries (section-header format vs standard entry IDs)
- [Phase 10]: Consolidated duplicates with promoted (consolidated) status AND See: cross-reference for traceability
- [Phase 10]: Project-specific entries left untouched with no annotation (per plan instructions)

### Pending Todos

None yet.

### Blockers/Concerns

- PROC-01 (exit 137) confirmed as Claude Code limitation, not fixable in agent-deck. Mitigations documented in 09-INVESTIGATION.md.

## Session Continuity

Last session: 2026-03-06T22:39:28.922Z
Stopped at: Milestone v1.2 complete (all 10 phases, 21 plans)
Resume file: None
