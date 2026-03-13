# Requirements: Agent Deck

**Defined:** 2026-03-12
**Core Value:** Reliable terminal session management for AI coding agents with conductor orchestration

## v1.3 Requirements

Requirements for v1.3 Session Reliability & Resume. Each maps to roadmap phases.

Rescoped 2026-03-12: removed completed items (#320, #318), added critical new issues (#324, #322), promoted items from future (#225, #266, #255, #216).

### MCP Proxy Reliability

- [x] **MCP-01**: MCP socket proxy assigns unique request IDs per proxy instance to prevent collisions when multiple sessions share the same proxy (#324)
- [x] **MCP-02**: Request/response correlation uses session-scoped ID mapping so responses route to the correct caller (#324)
- [x] **MCP-03**: Integration test verifies two concurrent sessions issuing tool calls through a shared proxy receive correct responses without cross-talk

### Session Visibility

- [x] **VIS-01**: Stopped sessions appear in main TUI session list with distinct styling from error sessions (#307)
- [x] **VIS-02**: Preview pane differentiates stopped (user-intentional) from error (crash) with distinct action guidance and resume affordance (#307)
- [x] **VIS-03**: Session picker dialog correctly filters stopped sessions for conductor flows (stopped excluded from conductor picker, visible in main list)

### Resume Deduplication

- [x] **DEDUP-01**: Resuming a stopped session reuses the existing session record instead of creating a new duplicate entry (#224)
- [x] **DEDUP-02**: UpdateClaudeSessionsWithDedup runs in-memory immediately at resume site, not only at persist time (#224)
- [x] **DEDUP-03**: Concurrent-write integration test covers two Storage instances against the same SQLite file

### Platform Reliability

- [ ] **PLAT-01**: Auto-start (agent-deck session start) works from non-interactive contexts on WSL/Linux; tool processes receive a PTY (#311)
- [ ] **PLAT-02**: Resume after auto-start uses correct tool conversation ID (not agent-deck internal UUID) (#311)

### Detection & Sandbox

- [ ] **DET-01**: tmux set-environment works correctly inside Docker sandbox sessions now that sandbox config persistence is fixed (#266)
- [ ] **DET-02**: OpenCode waiting status detection triggers correctly when OpenCode presents the question tool prompt (#255)

### UX Polish

- [x] **UX-01**: Mouse wheel scroll works in session list and other scrollable areas (settings, search, dialogs) (#262, #254)
- [x] **UX-02**: Light theme renders correctly in Codex preview and live session views; no dark background bleed-through (#322)
- [ ] **UX-03**: auto_cleanup option documented in README sandbox section with explanation of what gets cleaned and when (#228)
- [ ] **UX-04**: Redundant heartbeat mechanisms consolidated into a single mechanism (systemd timer vs bridge.py heartbeat_loop) (#225)
- [ ] **UX-05**: Existing git worktrees are detected and reused instead of creating new ones when a worktree for the target branch already exists (#216)

### Comprehensive Testing

- [ ] **TEST-01**: Integration tests cover all v1.3 fixes with regression assertions
- [ ] **TEST-02**: MCP proxy concurrent session test passes under race detector
- [ ] **TEST-03**: Session lifecycle test covers stopped/resumed/error transitions end-to-end
- [ ] **TEST-04**: Light theme rendering test validates no hardcoded dark-only color values in preview/session views
- [ ] **TEST-05**: Memory leak detection: run extended sessions (10+ min) and monitor RSS growth; flag if memory grows beyond 2x initial after GC
- [ ] **TEST-06**: CPU usage monitoring: profile hot paths during idle polling and active sessions; CPU should be <5% when idle with 10 sessions
- [ ] **TEST-07**: Resource cleanup verification: after session stop/delete, no orphaned tmux sessions, no leaked goroutines, no stale file handles
- [ ] **TEST-08**: Regression test framework: every bug fix in Phases 11-15 gets a dedicated regression test that reproduces the original bug and verifies the fix
- [ ] **TEST-09**: Cross-tool matrix: run the full test suite against Claude, Codex, Gemini, and OpenCode session types; each tool must pass status detection, send, and lifecycle tests
- [ ] **TEST-10**: Concurrent stress test: 20+ simultaneous sessions with random operations (start, stop, send, fork); no races (go test -race), no deadlocks, no data corruption

## Completed (v1.3 scope, already shipped)

- [x] **~~STORE-01~~**: Sandbox config persistence (#320) — closed 2026-03-12
- [x] **~~STORE-02~~**: MarshalToolData struct refactor (#320) — closed 2026-03-12
- [x] **~~STORE-03~~**: Round-trip integration test for sandbox config (#320) — closed 2026-03-12
- [x] **~~SET-01~~**: Settings panel custom tool icons (#318) — closed 2026-03-11

## Future Requirements

Deferred to v1.4+. Tracked but not in current roadmap.

### Mouse Interaction

- **MOUSE-01**: Mouse click-to-select session in list (requires coordinate hit-testing against custom list renderer)
- **MOUSE-02**: Double-click or click-then-Enter to attach (requires click-select first + stateful double-click detection)

### Infrastructure

- **INFRA-02**: Custom env variables for conductor sessions (#256)
- **INFRA-03**: Native session notification bridge without conductor (#211)

### Platform Expansion

- **PLAT-03**: Native Windows support via psmux (#277)
- **PLAT-04**: Remote session management improvements (#297)
- **PLAT-05**: OpenCode fork support (#317)

## Out of Scope

| Feature | Reason |
|---------|--------|
| `bubbles/list` migration | Full rewrite of home.go (~8500 lines); regression risk across every existing feature |
| Global tmux mouse config | `set -g mouse on` affects all tmux sessions; violates user sovereignty |
| Performance testing at 50+ sessions | Per PROJECT.md out-of-scope; defer to v2 |
| Auto-hide stopped sessions by default | Anti-feature: hides the sessions users want to resume |
| Merge stopped+error status | Different semantics (user intent vs crash); conductor templates depend on distinction |
| Bidi file sync for remote sessions (#272) | Advanced remote feature, low priority |
| Phone-optimized web view (#313) | Low priority |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| MCP-01 | Phase 11 | Complete |
| MCP-02 | Phase 11 | Complete |
| MCP-03 | Phase 11 | Complete |
| VIS-01 | Phase 12 | Complete |
| VIS-02 | Phase 12 | Complete |
| VIS-03 | Phase 12 | Complete |
| DEDUP-01 | Phase 12 | Complete |
| DEDUP-02 | Phase 12 | Complete |
| DEDUP-03 | Phase 12 | Complete |
| PLAT-01 | Phase 13 | Pending |
| PLAT-02 | Phase 13 | Pending |
| DET-01 | Phase 14 | Pending |
| DET-02 | Phase 14 | Pending |
| UX-01 | Phase 15 | Complete |
| UX-02 | Phase 15 | Complete |
| UX-03 | Phase 15 | Pending |
| UX-04 | Phase 15 | Pending |
| UX-05 | Phase 15 | Pending |
| TEST-01 | Phase 16 | Pending |
| TEST-02 | Phase 16 | Pending |
| TEST-03 | Phase 16 | Pending |
| TEST-04 | Phase 16 | Pending |
| TEST-05 | Phase 16 | Pending |
| TEST-06 | Phase 16 | Pending |
| TEST-07 | Phase 16 | Pending |
| TEST-08 | Phase 16 | Pending |
| TEST-09 | Phase 16 | Pending |
| TEST-10 | Phase 16 | Pending |

**Coverage:**
- v1.3 requirements: 28 total (4 completed, 24 pending)
- Mapped to phases: 28
- Unmapped: 0

**Phase distribution:**
- Phase 11 (MCP Proxy Reliability): MCP-01, MCP-02, MCP-03
- Phase 12 (Session List & Resume UX): VIS-01, VIS-02, VIS-03, DEDUP-01, DEDUP-02, DEDUP-03
- Phase 13 (Auto-Start & Platform): PLAT-01, PLAT-02
- Phase 14 (Detection & Sandbox): DET-01, DET-02
- Phase 15 (Mouse, Theme & Polish): UX-01, UX-02, UX-03, UX-04, UX-05
- Phase 16 (Comprehensive Testing): TEST-01, TEST-02, TEST-03, TEST-04, TEST-05, TEST-06, TEST-07, TEST-08, TEST-09, TEST-10

---
*Requirements defined: 2026-03-12*
*Last updated: 2026-03-12 — Rescoped: removed completed #320/#318, added #324/#322/#266/#255/#225/#216, added Phase 16 testing*
