# Roadmap: Agent Deck Skills Reorganization & Stabilization

## Overview

This milestone transforms the agent-deck skills system to use the official Anthropic skill-creator format, verifies session lifecycle and skills loading through comprehensive testing, and stabilizes the codebase for release readiness. The work flows naturally from reformatting skills, to verifying everything works, to ensuring the codebase is clean and shippable.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Skills Reorganization** - Reformat all skills to official Anthropic skill-creator structure and verify path resolution
- [ ] **Phase 2: Testing & Bug Fixes** - Verify session lifecycle, sleep/wake detection, and skills triggering; fix discovered bugs
- [ ] **Phase 3: Stabilization & Release Readiness** - Clean up codebase, pass all quality gates, prepare for release

## Phase Details

### Phase 1: Skills Reorganization
**Goal**: All agent-deck skills use the official Anthropic skill-creator format and load correctly from both plugin cache and local development paths
**Depends on**: Nothing (first phase)
**Requirements**: SKILL-01, SKILL-02, SKILL-03, SKILL-04, SKILL-05
**Success Criteria** (what must be TRUE):
  1. Agent-deck skill has proper SKILL.md frontmatter (name, description, compatibility) and organized scripts/ and references/ directories
  2. Session-share skill has proper SKILL.md frontmatter and scripts/ directory following the official format
  3. GSD conductor skill exists in ~/.agent-deck/skills/pool/gsd-conductor/ with current, complete content
  4. Loading a skill via `Read ~/.agent-deck/skills/pool/<name>/SKILL.md` resolves script paths correctly regardless of whether it runs from plugin cache or local checkout
**Plans:** 2 plans

Plans:
- [ ] 01-01-PLAN.md -- Fix frontmatter for agent-deck and session-share, add $SKILL_DIR path resolution, register session-share in marketplace.json
- [ ] 01-02-PLAN.md -- Audit and update GSD conductor skill content, validate path resolution across all three skills

### Phase 2: Testing & Bug Fixes
**Goal**: Session lifecycle, sleep/wake detection, and skills triggering are verified through tests, and all bugs found during testing are fixed
**Depends on**: Phase 1
**Requirements**: TEST-01, TEST-02, TEST-03, TEST-04, TEST-05, TEST-06, TEST-07, STAB-01
**Success Criteria** (what must be TRUE):
  1. Sleep/wake detection transitions session status correctly (running to idle on inactivity, back to running on activity)
  2. Session start, stop, fork, and attach operations complete successfully and update status accurately in both SQLite and tmux
  3. Skills referenced in session context trigger correctly, and pool skills loaded on demand are functional
  4. All bugs discovered during this testing phase are identified, fixed, and regression-tested
**Plans**: TBD

Plans:
- [ ] 02-01: Sleep/wake and status transition tests
- [ ] 02-02: Session lifecycle end-to-end tests (start, stop, fork, attach)
- [ ] 02-03: Skills triggering tests and bug fixes

### Phase 3: Stabilization & Release Readiness
**Goal**: Codebase passes all quality gates, is free of dead code, and is ready to tag a release
**Depends on**: Phase 2
**Requirements**: STAB-02, STAB-03, STAB-04, STAB-05, STAB-06
**Success Criteria** (what must be TRUE):
  1. `golangci-lint run` completes with zero warnings
  2. `go test -race ./...` passes with zero failures across the entire codebase
  3. `go build` succeeds for darwin/amd64, darwin/arm64, linux/amd64, and linux/arm64
  4. No dead code, unused imports, or stale artifacts remain in the repository
  5. CHANGELOG.md documents all changes made during this milestone
**Plans**: TBD

Plans:
- [ ] 03-01: Lint fixes, dead code removal, and cross-platform build verification
- [ ] 03-02: Changelog and final release preparation

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Skills Reorganization | 0/2 | Not started | - |
| 2. Testing & Bug Fixes | 0/3 | Not started | - |
| 3. Stabilization & Release Readiness | 0/2 | Not started | - |
