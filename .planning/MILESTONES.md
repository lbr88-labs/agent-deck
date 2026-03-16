# Milestones: Agent Deck

## Shipped

### v1.2 Conductor Reliability & Learnings Cleanup

**Shipped:** 2026-03-07
**Phases:** 7-10 (8 plans)
**Summary:** Fixed the top conductor reliability issues: hardened Enter key retry, added Codex readiness detection, scoped heartbeat to groups, added session death detection. Investigated exit 137 (Claude Code limitation) and documented mitigations. Promoted 27 validated learnings to 3 shared destinations.

**Requirements completed:** 12
- Send reliability (SEND-01, SEND-02)
- Heartbeat (HB-01, HB-02)
- CLI reliability (CLI-01, CLI-02, CLI-03)
- Process stability (PROC-01)
- Learnings promotion (LEARN-01 through LEARN-04)

**Key accomplishments:**
- Consolidated send verification into `internal/send` package with hardened Enter retry and Codex readiness detection
- Group-scoped heartbeat with interval=0 disable semantics and enabled-config guard
- Session death detection in waitForCompletion, verified -c/-g flag co-parsing
- Root cause analysis: exit 137 is Claude Code killing Bash tool children (documented with 6 mitigations)
- 27 learnings promoted to shared conductor CLAUDE.md, GSD conductor skill, and agent-deck skill
- All 5 LEARNINGS.md files annotated with promotion status and cross-references

### v1.0 Skills Reorganization & Stabilization

**Shipped:** 2026-03-06
**Phases:** 1-3 (7 plans)
**Summary:** Reformatted all skills to official Anthropic skill-creator structure. Verified session lifecycle, sleep/wake detection, and skills triggering. Stabilized codebase, passed all quality gates, prepared for release.

**Requirements completed:** 18
- Skills reformatted (SKILL-01 through SKILL-05)
- Testing verified (TEST-01 through TEST-07, STAB-01)
- Stabilization complete (STAB-02 through STAB-06)

### v1.1 Integration Testing

**Shipped:** 2026-03-07
**Phases:** 4-6 (6 plans)
**Summary:** Built comprehensive integration testing framework for conductor orchestration pipeline. Tested session lifecycle, status detection across all tools, cross-session events, conductor send/heartbeat, and edge cases (concurrent polling, external storage changes, skills integration).

**Requirements completed:** 13
- Framework infrastructure (INFRA-01 through INFRA-04)
- Session lifecycle integration (LIFE-01 through LIFE-04)
- Status detection (DETECT-01 through DETECT-03)
- Conductor operations (COND-01 through COND-04)
- Edge cases (EDGE-01 through EDGE-03)

---
*Last phase shipped: Phase 10*
*Next phase number: 11*
