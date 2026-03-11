# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.2 -- Conductor Reliability & Learnings Cleanup

**Shipped:** 2026-03-07
**Phases:** 4 | **Plans:** 8

### What Was Built
- Consolidated `internal/send` package replacing 7 duplicated prompt detection functions
- Hardened Enter retry loop (aggressive early-window nudging) and Codex readiness gating
- Group-scoped heartbeat with interval=0 disable and enabled-config guard
- Session death detection in waitForCompletion (5-error threshold)
- Exit 137 root cause analysis and 6 documented mitigation strategies
- 27 learnings promoted to conductor CLAUDE.md, GSD conductor skill, and agent-deck skill

### What Worked
- Ordering phases by impact (send reliability first, learnings last) kept fixes isolated and learnings comprehensive
- Reusing the v1.1 integration test framework made verification fast
- Investigation-first approach for exit 137 saved time: confirmed it was not fixable before attempting fixes
- Bulk documentation phase (10) was efficient: all context fresh from phases 7-9

### What Was Inefficient
- Some ROADMAP.md plan checkboxes were not updated during execution (showed `[ ]` for completed plans)
- Phase 10 SUMMARY.md files lacked one_liner fields, making automated extraction fail

### Patterns Established
- `internal/send` package as the single owner of prompt detection and send verification logic
- Consecutive-error threshold pattern for detecting dead sessions
- Investigation plan (root cause) before fix plan for mysterious failures
- Blockquote format for LEARNINGS.md entries from non-standard sources

### Key Lessons
1. When 7+ functions do the same thing across packages, consolidate into a shared package immediately
2. Exit 137 type issues (external tool limitations) should be investigated before attempting fixes
3. SUMMARY.md one_liner fields must be populated for milestone tooling to work

### Cost Observations
- Model mix: Primarily sonnet for execution, opus for investigation (phase 9)
- Notable: All 4 phases completed in a single day (2026-03-07)

---

## Milestone: v1.1 -- Integration Testing

**Shipped:** 2026-03-07
**Phases:** 3 | **Plans:** 6

### What Was Built
- Integration test framework (TmuxHarness, polling helpers, SQLite fixtures)
- Session lifecycle integration tests (start, stop, fork, restart)
- Status detection tests for Claude, Gemini, OpenCode, Codex
- Conductor send-to-child and event notification cycle tests
- Edge case tests (concurrent polling, external storage changes, skills integration)

### What Worked
- Architecture-first approach (design framework, then write tests) produced consistent patterns
- Real tmux sessions with simple commands (echo, sleep, cat) gave confidence without needing real AI tools

### Patterns Established
- TmuxHarness for test session lifecycle management
- Polling helpers instead of time.Sleep for async assertions
- TestMain with AGENTDECK_PROFILE=_test as mandatory isolation

### Key Lessons
1. Integration tests using real infrastructure (tmux) catch bugs that unit tests miss
2. Test fixture helpers pay for themselves after 3+ uses

---

## Milestone: v1.0 -- Skills Reorganization & Stabilization

**Shipped:** 2026-03-06
**Phases:** 3 | **Plans:** 7

### What Was Built
- Skills reformatted to official Anthropic skill-creator structure
- Session lifecycle and sleep/wake detection verified
- Codebase stabilized: lint clean, all tests passing, dead code removed

### What Worked
- Following the official skill-creator format exactly eliminated path resolution issues
- Quality gates (lint, test, build) as formal phase ensured nothing slipped through

### Patterns Established
- TestMain files in every test package for profile isolation
- Skills use $SKILL_DIR for path resolution

### Key Lessons
1. Invest in reformatting to official standards early: it prevents a class of integration bugs
2. Quality gates should be a dedicated phase, not an afterthought

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Key Change |
|-----------|--------|-------|------------|
| v1.0 | 3 | 7 | Established test isolation pattern |
| v1.1 | 3 | 6 | Added integration test framework |
| v1.2 | 4 | 8 | Investigation-first for unknowns |

### Top Lessons (Verified Across Milestones)

1. TestMain profile isolation is non-negotiable (validated v1.0, v1.1, v1.2)
2. Architecture/investigation first, then implementation (validated v1.1, v1.2)
3. Consolidate duplicated logic into shared packages as soon as pattern emerges (validated v1.0, v1.2)
