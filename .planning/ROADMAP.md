# Roadmap: Agent Deck

## Milestones

- ~~**v1.0 Skills Reorganization & Stabilization**~~ -- Phases 1-3 (shipped 2026-03-06)
- ~~**v1.1 Integration Testing**~~ -- Phases 4-6 (shipped 2026-03-07)
- ~~**v1.2 Conductor Reliability & Learnings Cleanup**~~ -- Phases 7-10 (shipped 2026-03-07)
- **v1.3 Session Reliability & Resume** -- Phases 11-15 (active)

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

<details>
<summary>v1.0 Skills Reorganization & Stabilization (Phases 1-3) -- SHIPPED 2026-03-06</summary>

- [x] **Phase 1: Skills Reorganization** (2/2 plans) -- completed 2026-03-06
- [x] **Phase 2: Testing & Bug Fixes** (3/3 plans) -- completed 2026-03-06
- [x] **Phase 3: Stabilization & Release Readiness** (2/2 plans) -- completed 2026-03-06

</details>

<details>
<summary>v1.1 Integration Testing (Phases 4-6) -- SHIPPED 2026-03-07</summary>

- [x] **Phase 4: Framework Foundation** (2/2 plans) -- completed 2026-03-07
- [x] **Phase 5: Status Detection & Events** (2/2 plans) -- completed 2026-03-07
- [x] **Phase 6: Conductor Pipeline & Edge Cases** (2/2 plans) -- completed 2026-03-07

</details>

<details>
<summary>v1.2 Conductor Reliability & Learnings Cleanup (Phases 7-10) -- SHIPPED 2026-03-07</summary>

- [x] **Phase 7: Send Reliability** (2/2 plans) -- completed 2026-03-07
- [x] **Phase 8: Heartbeat & CLI Fixes** (2/2 plans) -- completed 2026-03-07
- [x] **Phase 9: Process Stability** (2/2 plans) -- completed 2026-03-07
- [x] **Phase 10: Learnings Promotion** (2/2 plans) -- completed 2026-03-06

</details>

**v1.3 Session Reliability & Resume (Phases 11-15)**

- [ ] **Phase 11: Storage Foundation** - Sandbox config persists through stop/restart; MarshalToolData refactored; auto_cleanup documented
- [ ] **Phase 12: Session List Correctness** - Stopped sessions visible and resumable in main TUI with distinct styling
- [ ] **Phase 13: Resume Deduplication** - Resuming a stopped session reuses the existing record, no duplicates
- [ ] **Phase 14: Auto-Start TTY Fix** - Auto-start works on WSL/Linux from non-interactive contexts with correct conversation ID
- [ ] **Phase 15: Mouse Support & Settings Polish** - Mouse wheel scroll and custom tool icons in settings panel

## Phase Details

### Phase 11: Storage Foundation
**Goal:** Sandbox config survives the full stop/restart/reload cycle without data loss, and the serialization contract is compiler-enforced
**Depends on:** Nothing (first phase of v1.3)
**Requirements:** STORE-01, STORE-02, STORE-03, UX-03
**Success Criteria** (what must be TRUE):
  1. User configures a Docker sandbox session, stops it, and restarts it; the container image, limits, and auto_cleanup setting are preserved exactly as set
  2. A developer adding a new field to toolDataBlob gets a compile error if MarshalToolData is not updated, rather than silent data loss at runtime
  3. A round-trip integration test against a fresh Storage instance (not the in-memory instance that saved) passes green
  4. The README sandbox section documents the auto_cleanup option with a clear explanation of what gets cleaned and when
**Plans:** TBD

### Phase 12: Session List Correctness
**Goal:** Users can see, identify, and take action on stopped sessions directly from the main TUI session list
**Depends on:** Phase 11
**Requirements:** VIS-01, VIS-02, VIS-03
**Success Criteria** (what must be TRUE):
  1. A stopped session appears in the main TUI session list rather than being hidden or silently excluded
  2. A stopped session and an error session are visually distinguishable in the list (distinct styling)
  3. The preview pane for a stopped session shows user-intentional stop context with a resume affordance; the preview pane for an error session shows crash context with different guidance
  4. The conductor session picker dialog excludes stopped sessions from its list (correct filtering preserved), while the main list continues to show them
**Plans:** TBD

### Phase 13: Resume Deduplication
**Goal:** Resuming a stopped session produces exactly one session record, not two; concurrent writes between Storage instances are safe
**Depends on:** Phase 12
**Requirements:** DEDUP-01, DEDUP-02, DEDUP-03
**Success Criteria** (what must be TRUE):
  1. User resumes a stopped session and sees one entry in the session list, not two; the original record is reused rather than duplicated
  2. Conductor session counts remain accurate after a resume (no phantom duplicate counted as an active session)
  3. UpdateClaudeSessionsWithDedup runs immediately in memory at the resume call site, so the dedup is visible before the next persist cycle
  4. A concurrent-write integration test covering two Storage instances against the same SQLite file passes green
**Plans:** TBD

### Phase 14: Auto-Start TTY Fix
**Goal:** Users on WSL/Linux can run agent-deck session start from non-interactive contexts (scripts, cron, systemd) and tool processes receive a working PTY
**Depends on:** Phase 13
**Requirements:** PLAT-01, PLAT-02
**Success Criteria** (what must be TRUE):
  1. Running agent-deck session start from a non-interactive shell on WSL/Linux starts the session without tool processes rejecting input due to a missing PTY
  2. After auto-starting and stopping a session on WSL/Linux, resuming it attaches to the correct tool conversation (identified by the tool conversation ID, not the agent-deck internal UUID)
**Plans:** TBD

### Phase 15: Mouse Support & Settings Polish
**Goal:** Users can scroll with a mouse or trackpad in all scrollable areas, and custom tool icons appear in the settings panel
**Depends on:** Phase 11 (independent of Phases 12-14; can be parallelized)
**Requirements:** UX-01, UX-02
**Success Criteria** (what must be TRUE):
  1. Scrolling the mouse wheel or trackpad in the session list moves the selection up and down without requiring keyboard input
  2. Mouse wheel scroll works in all other scrollable areas: settings panel, global search results, and dialogs
  3. Custom tools added by the user display their configured icon in the settings panel default tool radio group, making them visually distinguishable from built-in tools
**Plans:** TBD

## Progress

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
| 10. Learnings Promotion | v1.2 | 2/2 | Complete | 2026-03-06 |
| 11. Storage Foundation | v1.3 | 0/TBD | Not started | - |
| 12. Session List Correctness | v1.3 | 0/TBD | Not started | - |
| 13. Resume Deduplication | v1.3 | 0/TBD | Not started | - |
| 14. Auto-Start TTY Fix | v1.3 | 0/TBD | Not started | - |
| 15. Mouse Support & Settings Polish | v1.3 | 0/TBD | Not started | - |
