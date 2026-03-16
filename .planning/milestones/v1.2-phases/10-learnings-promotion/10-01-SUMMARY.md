---
phase: 10-learnings-promotion
plan: 01
subsystem: documentation
tags: [conductor, learnings, knowledge-base, skill-promotion, orchestration-patterns]

# Dependency graph
requires:
  - phase: 09-process-stability
    provides: "Exit 137 mitigations already in conductor CLAUDE.md and gsd-conductor SKILL.md"
provides:
  - "Orchestration Best Practices section in shared conductor CLAUDE.md with 10 universal patterns"
  - "6 GSD-specific learnings integrated into gsd-conductor SKILL.md"
  - "Operational Patterns section in agent-deck-workflow SKILL.md with 11 operational learnings"
affects: [conductor-sessions, gsd-conductor-sessions, agent-deck-workflow]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Knowledge promotion pattern: curate scattered learnings into shared skill/knowledge files"
    - "Classification pattern: universal vs GSD-specific vs operational vs project-specific"

key-files:
  created: []
  modified:
    - ~/.agent-deck/conductor/CLAUDE.md
    - ~/.agent-deck/skills/pool/gsd-conductor/SKILL.md
    - ~/.agent-deck/skills/pool/agent-deck-workflow/SKILL.md

key-decisions:
  - "Placed Orchestration Best Practices between Exit 137 and State Management in conductor CLAUDE.md"
  - "Grouped universal patterns into Monitoring, Sending, Session Management, and Task Design subsections"
  - "Added GSD Claude-only constraint as prominent callout at top of gsd-conductor SKILL.md"
  - "Added Stage 0 (codebase mapping) to GSD lifecycle for brownfield projects"
  - "Added Codex-specific troubleshooting items directly into Troubleshooting section of agent-deck-workflow SKILL.md"
  - "Placed Operational Patterns section before Quick Reference Card in agent-deck-workflow SKILL.md"

patterns-established:
  - "Learnings promotion: classify by scope (universal/tool-specific/project-specific), then integrate surgically into existing document structure"

requirements-completed: [LEARN-01, LEARN-02, LEARN-03]

# Metrics
duration: 3min
completed: 2026-03-07
---

# Phase 10 Plan 1: Learnings Promotion Summary

**Promoted 27 validated conductor learnings from 7 LEARNINGS.md files into 3 shared destinations: 10 universal orchestration patterns, 6 GSD-specific learnings, and 11 agent-deck operational patterns**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-06T21:28:11Z
- **Completed:** 2026-03-06T21:31:11Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- Added "Orchestration Best Practices" section to conductor CLAUDE.md with 10 universal patterns organized into Monitoring, Sending, Session Management, and Task Design subsections
- Integrated 6 GSD-specific learnings into gsd-conductor SKILL.md: Claude-only constraint callout, Stage 0 codebase mapping, auto_advance guidance, comprehensive spec feeding, parallel wave execution, and tmux prompt reliability validation
- Added "Operational Patterns" section to agent-deck-workflow SKILL.md with 11 learnings covering session identification, deletion, Codex patterns, Gemini video, --wait behavior, worktree limitations, and heartbeat verbosity
- Added 3 Codex-specific troubleshooting items to existing Troubleshooting section of agent-deck-workflow SKILL.md

## Task Commits

Modified files are outside the project git repository (~/.agent-deck/), so per-task git commits for the actual edits are not applicable. The files were modified in place.

1. **Task 1: Promote universal patterns to shared conductor CLAUDE.md** - N/A (file outside repo)
2. **Task 2: Promote GSD-specific learnings to gsd-conductor SKILL.md** - N/A (file outside repo)
3. **Task 3: Promote agent-deck operational learnings to agent-deck-workflow SKILL.md** - N/A (file outside repo)

## Files Created/Modified
- `~/.agent-deck/conductor/CLAUDE.md` - Added "Orchestration Best Practices" section with 10 universal conductor patterns between Exit 137 and State Management
- `~/.agent-deck/skills/pool/gsd-conductor/SKILL.md` - Added Claude-only constraint callout, Stage 0 codebase mapping, auto_advance guidance, spec feeding guidance, parallel execution guidance, tmux validation note
- `~/.agent-deck/skills/pool/agent-deck-workflow/SKILL.md` - Added "Operational Patterns" section with 11 patterns plus 3 Codex troubleshooting items

## Decisions Made
- Placed Orchestration Best Practices between Exit 137 and State Management in conductor CLAUDE.md for logical flow (monitoring and sending patterns follow the exit 137 guidance)
- Grouped 10 universal patterns into 4 subsections (Monitoring, Sending, Session Management, Task Design) for scannability
- Added Claude-only constraint as a blockquote callout at the very top of gsd-conductor SKILL.md (after frontmatter) for maximum visibility
- Created "Stage 0: Codebase Mapping" as a formal lifecycle stage rather than a footnote, with code examples
- Added Codex troubleshooting items directly into the existing Troubleshooting section (not the new Operational Patterns section) since they match the existing Q&A format
- Included cross-conductor validation notes (e.g., "Validated across agent-deck, ard, ryan, opengraphdb conductors") per plan instructions

## Deviations from Plan

None. Plan executed exactly as written.

## Issues Encountered
- Modified files are outside the project git repository (~/.agent-deck/), so per-task git commits for the actual edits were not possible. Planning artifacts are committed to the project repo.

## User Setup Required
None. No external service configuration required.

## Next Phase Readiness
- Plan 01 (promotion) is complete. All 27 learnings are now in their destination files.
- Plan 02 (LEARNINGS.md cleanup: marking promoted entries, removing retired entries) can proceed.

## Self-Check: PASSED

- FOUND: ~/.agent-deck/conductor/CLAUDE.md
- FOUND: ~/.agent-deck/skills/pool/gsd-conductor/SKILL.md
- FOUND: ~/.agent-deck/skills/pool/agent-deck-workflow/SKILL.md
- FOUND: 10-01-SUMMARY.md
- VERIFIED: Orchestration Best Practices section in conductor CLAUDE.md
- VERIFIED: Claude-only constraint in gsd-conductor SKILL.md
- VERIFIED: Operational Patterns section in agent-deck-workflow SKILL.md

---
*Phase: 10-learnings-promotion*
*Completed: 2026-03-07*
