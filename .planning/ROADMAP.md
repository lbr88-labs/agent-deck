# Roadmap: Agent Deck

## Milestones

- ~~**v1.0 Skills Reorganization & Stabilization**~~ -- Phases 1-3 (shipped)
- **v1.1 Integration Testing** -- Phases 4-6 (in progress)

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

<details>
<summary>v1.0 Skills Reorganization & Stabilization (Phases 1-3) -- SHIPPED</summary>

- [x] **Phase 1: Skills Reorganization** - Reformat all skills to official Anthropic skill-creator structure and verify path resolution
- [x] **Phase 2: Testing & Bug Fixes** - Verify session lifecycle, sleep/wake detection, and skills triggering; fix discovered bugs
- [x] **Phase 3: Stabilization & Release Readiness** - Clean up codebase, pass all quality gates, prepare for release

</details>

### v1.1 Integration Testing

- [ ] **Phase 4: Framework Foundation** - Build shared test infrastructure and verify session lifecycle with real tmux sessions
- [ ] **Phase 5: Status Detection & Events** - Validate sleep/wait detection accuracy, multi-tool behavior, and cross-session event delivery
- [ ] **Phase 6: Conductor Pipeline & Edge Cases** - Test full orchestration round-trips and production-grade edge case scenarios

## Phase Details

<details>
<summary>v1.0 Phases (shipped)</summary>

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
- [x] 01-01: Fix frontmatter for agent-deck and session-share, add $SKILL_DIR path resolution, register session-share in marketplace.json
- [x] 01-02: Audit and update GSD conductor skill content, validate path resolution across all three skills

### Phase 2: Testing & Bug Fixes
**Goal**: Session lifecycle, sleep/wake detection, and skills triggering are verified through tests, and all bugs found during testing are fixed
**Depends on**: Phase 1
**Requirements**: TEST-01, TEST-02, TEST-03, TEST-04, TEST-05, TEST-06, TEST-07, STAB-01
**Success Criteria** (what must be TRUE):
  1. Sleep/wake detection transitions session status correctly (running to idle on inactivity, back to running on activity)
  2. Session start, stop, fork, and attach operations complete successfully and update status accurately in both SQLite and tmux
  3. Skills referenced in session context trigger correctly, and pool skills loaded on demand are functional
  4. All bugs discovered during this testing phase are identified, fixed, and regression-tested
**Plans:** 3 plans

Plans:
- [x] 02-01: Sleep/wake status transition cycle tests and SQLite persistence verification
- [x] 02-02: Session lifecycle tests (start, stop, fork, attach) with tmux verification
- [x] 02-03: Skills runtime triggering tests and bug fixes from Plans 01-02

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
**Plans:** 2 plans

Plans:
- [x] 03-01: Quality gates verification, dead code scan, and stale artifact removal
- [x] 03-02: Changelog entry for milestone and final make ci release gate

</details>

### Phase 4: Framework Foundation
**Goal**: Developers can write integration tests using shared helpers that manage real tmux sessions, poll for conditions, and seed SQLite fixtures, with session lifecycle tests proving the foundation works
**Depends on**: Phase 3 (stable codebase from v1.0)
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04, LIFE-01, LIFE-02, LIFE-03, LIFE-04
**Success Criteria** (what must be TRUE):
  1. A new integration test can create a real tmux session, run a command in it, and have the session automatically cleaned up after the test completes, without orphaned sessions remaining
  2. Tests that need to wait for asynchronous state (pane content appearing, status changing) use polling helpers instead of time.Sleep, and fail with clear timeout messages when conditions are not met
  3. Integration tests run in complete isolation from production data: profile is forced to `_test`, SQLite databases are created in temp directories, and no user sessions are affected
  4. Session start creates a real tmux session that transitions to running, session stop terminates it, session fork produces an independent copy with parent-child linkage, and session restart with flags recreates correctly
**Plans**: TBD

### Phase 5: Status Detection & Events
**Goal**: Sleep/wait detection correctly identifies tool-specific patterns across all supported tools, and cross-session event notifications reliably propagate between conductor parent and child sessions
**Depends on**: Phase 4
**Requirements**: DETECT-01, DETECT-02, DETECT-03, COND-01, COND-02
**Success Criteria** (what must be TRUE):
  1. Simulated pane output containing Claude, Gemini, OpenCode, and Codex sleep/wait patterns is correctly detected by the status engine, producing accurate status transitions
  2. Creating sessions with different tool types (Claude, Gemini, OpenCode, Codex) produces the correct launch commands and detection configuration for each tool
  3. A session driven through real tmux pane content transitions correctly through the full status cycle: starting to running to waiting to idle
  4. A conductor parent can send a command to a child session via real tmux and the child receives it; the cross-session event notification cycle completes (event written, watcher detects, parent notified)
**Plans**: TBD

### Phase 6: Conductor Pipeline & Edge Cases
**Goal**: The full conductor orchestration pipeline is tested end-to-end, and production-grade edge cases (concurrent polling, external storage changes, skills integration) are verified
**Depends on**: Phase 5
**Requirements**: COND-03, COND-04, EDGE-01, EDGE-02, EDGE-03
**Success Criteria** (what must be TRUE):
  1. A conductor heartbeat round-trip completes: parent sends heartbeat, child responds, parent verifies receipt
  2. Send-with-retry delivers content to a real tmux session using chunked sending and paste-marker detection
  3. Skills are discovered from directory, attached to a session, and trigger conditions are evaluated correctly
  4. Concurrent polling of 10+ sessions returns correct status for each session without data races (verified by -race flag)
  5. A second Storage instance writing to the same SQLite database triggers the storage watcher in the first instance to detect the external change
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 4 -> 5 -> 6

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Skills Reorganization | v1.0 | 2/2 | Complete | 2026-03-06 |
| 2. Testing & Bug Fixes | v1.0 | 3/3 | Complete | 2026-03-06 |
| 3. Stabilization & Release Readiness | v1.0 | 2/2 | Complete | 2026-03-06 |
| 4. Framework Foundation | v1.1 | 0/? | Not started | - |
| 5. Status Detection & Events | v1.1 | 0/? | Not started | - |
| 6. Conductor Pipeline & Edge Cases | v1.1 | 0/? | Not started | - |
