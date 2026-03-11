# Roadmap: Agent Deck

## Milestones

- ~~**v1.0 Skills Reorganization & Stabilization**~~ -- Phases 1-3 (shipped)
- ~~**v1.1 Integration Testing**~~ -- Phases 4-6 (shipped)
- **v1.2 Conductor Reliability & Learnings Cleanup** -- Phases 7-10 (in progress)

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

<details>
<summary>v1.1 Integration Testing (Phases 4-6) -- SHIPPED</summary>

- [x] **Phase 4: Framework Foundation** - Build shared test infrastructure and verify session lifecycle with real tmux sessions
- [x] **Phase 5: Status Detection & Events** - Validate sleep/wait detection accuracy, multi-tool behavior, and cross-session event delivery
- [x] **Phase 6: Conductor Pipeline & Edge Cases** - Test full orchestration round-trips and production-grade edge case scenarios

</details>

### v1.2 Conductor Reliability & Learnings Cleanup

- [ ] **Phase 7: Send Reliability** - Fix Enter key race condition and Codex readiness detection, the two highest-impact send failures
- [ ] **Phase 8: Heartbeat & CLI Fixes** - Scope heartbeat to conductor groups, honor enabled=false config, and fix CLI flag parsing issues
- [x] **Phase 9: Process Stability** - Investigate exit 137 from incoming messages and document mitigation or fix
- [x] **Phase 10: Learnings Promotion** - Promote validated conductor learnings to shared locations and clean up LEARNINGS.md files (completed 2026-03-06)

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

<details>
<summary>v1.1 Phases (shipped)</summary>

### Phase 4: Framework Foundation
**Goal**: Developers can write integration tests using shared helpers that manage real tmux sessions, poll for conditions, and seed SQLite fixtures, with session lifecycle tests proving the foundation works
**Depends on**: Phase 3 (stable codebase from v1.0)
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04, LIFE-01, LIFE-02, LIFE-03, LIFE-04
**Success Criteria** (what must be TRUE):
  1. A new integration test can create a real tmux session, run a command in it, and have the session automatically cleaned up after the test completes, without orphaned sessions remaining
  2. Tests that need to wait for asynchronous state (pane content appearing, status changing) use polling helpers instead of time.Sleep, and fail with clear timeout messages when conditions are not met
  3. Integration tests run in complete isolation from production data: profile is forced to `_test`, SQLite databases are created in temp directories, and no user sessions are affected
  4. Session start creates a real tmux session that transitions to running, session stop terminates it, session fork produces an independent copy with parent-child linkage, and session restart with flags recreates correctly
**Plans:** 2 plans

Plans:
- [x] 04-01: Integration test infrastructure: TmuxHarness, polling helpers, SQLite fixtures, TestMain isolation
- [x] 04-02: Session lifecycle integration tests: start, stop, fork, restart using the new framework

### Phase 5: Status Detection & Events
**Goal**: Sleep/wait detection correctly identifies tool-specific patterns across all supported tools, and cross-session event notifications reliably propagate between conductor parent and child sessions
**Depends on**: Phase 4
**Requirements**: DETECT-01, DETECT-02, DETECT-03, COND-01, COND-02
**Success Criteria** (what must be TRUE):
  1. Simulated pane output containing Claude, Gemini, OpenCode, and Codex sleep/wait patterns is correctly detected by the status engine, producing accurate status transitions
  2. Creating sessions with different tool types (Claude, Gemini, OpenCode, Codex) produces the correct launch commands and detection configuration for each tool
  3. A session driven through real tmux pane content transitions correctly through the full status cycle: starting to running to waiting to idle
  4. A conductor parent can send a command to a child session via real tmux and the child receives it; the cross-session event notification cycle completes (event written, watcher detects, parent notified)
**Plans:** 2 plans

Plans:
- [x] 05-01: Status detection pattern tests for all four tools and real tmux status transition cycle
- [x] 05-02: Conductor send-to-child via tmux and cross-session event write-watch cycle

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
**Plans:** 2 plans

Plans:
- [x] 06-01: Conductor heartbeat round-trip and chunked send delivery tests (COND-03, COND-04)
- [x] 06-02: Edge case tests: skills discover/attach, concurrent polling, cross-instance storage watcher (EDGE-01, EDGE-02, EDGE-03)

</details>

### Phase 7: Send Reliability
**Goal**: Messages sent to sessions are reliably delivered, with Enter key submission working consistently and Codex sessions receiving text only after they are ready to accept input
**Depends on**: Phase 6 (integration test infrastructure available for verification)
**Requirements**: SEND-01, SEND-02
**Success Criteria** (what must be TRUE):
  1. Pasting text into a tmux session followed by Enter key submission delivers the complete message without dropped Enter keys, even under rapid successive sends
  2. Messages sent to a Codex session are held until Codex has attached to stdin, preventing text from being consumed by the underlying shell
  3. Existing send-related integration tests (COND-04 chunked send, COND-01 send-to-child) continue passing after the fixes
**Plans:** 2 plans

Plans:
- [x] 07-01-PLAN.md -- Consolidate send verification, harden Enter retry, add Codex readiness detection
- [ ] 07-02-PLAN.md -- Integration tests for Enter retry and Codex readiness on real tmux

### Phase 8: Heartbeat & CLI Fixes
**Goal**: Conductor heartbeat only monitors sessions in its own group, respects disabled configuration, and CLI flags for --wait, -cmd, and --no-parent work correctly without interfering with each other
**Depends on**: Phase 7 (send reliability fixes may affect heartbeat send path)
**Requirements**: HB-01, HB-02, CLI-01, CLI-02, CLI-03
**Success Criteria** (what must be TRUE):
  1. A conductor's heartbeat script reports status only for sessions in its configured group, not sessions belonging to other conductor groups
  2. Setting `conductor.enabled = false` or `heartbeat_interval = 0` in config prevents heartbeat from running; no launchd jobs fire and no heartbeat messages are sent
  3. `session send --wait` returns the correct exit code (0 for success, non-zero for failure) and does not hang when the target session completes or errors
  4. Using `-cmd` flag alongside `-group` flag parses both correctly; the `-c` shorthand is documented and does not collide with other flags
  5. Running `--no-parent` followed by `set-parent` restores parent routing, or `--no-parent` emits a clear warning that the detachment is permanent
**Plans:** 2 plans

Plans:
- [ ] 08-01-PLAN.md -- Fix heartbeat group scoping, interval=0 disabled semantics, and config guard
- [ ] 08-02-PLAN.md -- Fix --wait hang on session death, verify -c/-g flag parsing, improve --no-parent docs

### Phase 9: Process Stability
**Goal**: The cause of exit 137 (SIGKILL) on incoming messages is identified, and either fixed in agent-deck or documented as a Claude Code limitation with a practical mitigation strategy
**Depends on**: Phase 7 (send fixes must land first to isolate whether exit 137 is send-related or a separate issue)
**Requirements**: PROC-01
**Success Criteria** (what must be TRUE):
  1. The root cause of exit 137 is documented with reproduction steps, identifying whether it originates from agent-deck's send pipeline, tmux, or Claude Code itself
  2. If fixable in agent-deck, incoming messages no longer kill running Bash tool child processes; if a Claude Code limitation, the mitigation strategy is documented in the conductor CLAUDE.md and the GSD conductor skill
**Plans:** 2 plans

Plans:
- [x] 09-01-PLAN.md -- Investigate exit 137 signal chain, reproduce, determine root cause and fixability
- [x] 09-02-PLAN.md -- Document mitigation in conductor CLAUDE.md and GSD conductor skill

### Phase 10: Learnings Promotion
**Goal**: Validated conductor learnings from six production conductors are promoted to their appropriate shared locations, and the scattered LEARNINGS.md files are cleaned up
**Depends on**: Phase 9 (all code fixes complete, so learnings include any new findings from phases 7-9)
**Requirements**: LEARN-01, LEARN-02, LEARN-03, LEARN-04
**Success Criteria** (what must be TRUE):
  1. Universal conductor patterns (event-driven monitoring, parent linkage, session transition verification, Enter key workaround) appear in the shared conductor CLAUDE.md at `~/.agent-deck/conductor/CLAUDE.md`
  2. GSD-specific learnings (Claude-only, codebase mapping, comprehensive specs, wave model) are incorporated into the GSD conductor skill at `~/.agent-deck/skills/pool/gsd-conductor/`
  3. Agent-deck operational learnings (Codex launch syntax, release sessions, Gemini video, --wait patterns, project folder launching) are incorporated into the agent-deck skill
  4. All six conductor LEARNINGS.md files have promoted entries marked as promoted, retired entries removed, and duplicates consolidated
**Plans:** 2/2 plans complete

Plans:
- [ ] 10-01-PLAN.md -- Promote learnings to 3 destination files (CLAUDE.md, gsd-conductor SKILL.md, agent-deck-workflow SKILL.md)
- [ ] 10-02-PLAN.md -- Clean up all LEARNINGS.md source files (mark promoted, remove retired, consolidate duplicates)

## Progress

**Execution Order:**
Phases execute in numeric order: 7 -> 8 -> 9 -> 10

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Skills Reorganization | v1.0 | 2/2 | Complete | 2026-03-06 |
| 2. Testing & Bug Fixes | v1.0 | 3/3 | Complete | 2026-03-06 |
| 3. Stabilization & Release Readiness | v1.0 | 2/2 | Complete | 2026-03-06 |
| 4. Framework Foundation | v1.1 | 2/2 | Complete | 2026-03-07 |
| 5. Status Detection & Events | v1.1 | 2/2 | Complete | 2026-03-07 |
| 6. Conductor Pipeline & Edge Cases | v1.1 | 2/2 | Complete | 2026-03-07 |
| 7. Send Reliability | v1.2 | 2/2 | Complete | 2026-03-07 |
| 8. Heartbeat & CLI Fixes | v1.2 | 2/2 | Complete | 2026-03-07 |
| 9. Process Stability | v1.2 | 2/2 | Complete | 2026-03-07 |
| 10. Learnings Promotion | 2/2 | Complete    | 2026-03-06 | - |
