# Requirements: Agent Deck

**Defined:** 2026-03-12
**Core Value:** Reliable terminal session management for AI coding agents with conductor orchestration

## v1.3 Requirements

Requirements for v1.3 Session Reliability & Resume. Each maps to roadmap phases.

### Storage Persistence

- [ ] **STORE-01**: Sandbox config (Docker image, limits, auto_cleanup) survives save/reload/restart cycle without data loss (#320)
- [ ] **STORE-02**: MarshalToolData refactored to accept struct parameter so compiler catches missing fields on future additions
- [ ] **STORE-03**: Round-trip integration test verifies sandbox config persists through a fresh Storage instance (not same in-memory instance)

### Session Visibility

- [ ] **VIS-01**: Stopped sessions appear in main TUI session list with distinct styling from error sessions (#307)
- [ ] **VIS-02**: Preview pane differentiates stopped (user-intentional) from error (crash) with distinct action guidance and resume affordance (#307)
- [ ] **VIS-03**: Session picker dialog correctly filters stopped sessions for conductor flows (stopped excluded from conductor picker, visible in main list)

### Resume Deduplication

- [ ] **DEDUP-01**: Resuming a stopped session reuses the existing session record instead of creating a new duplicate entry (#224)
- [ ] **DEDUP-02**: UpdateClaudeSessionsWithDedup runs in-memory immediately at resume site, not only at persist time (#224)
- [ ] **DEDUP-03**: Concurrent-write integration test covers two Storage instances against the same SQLite file

### Platform Reliability

- [ ] **PLAT-01**: Auto-start (agent-deck session start) works from non-interactive contexts on WSL/Linux; tool processes receive a PTY (#311)
- [ ] **PLAT-02**: Resume after auto-start uses correct tool conversation ID (not agent-deck internal UUID) (#311)

### UX Polish

- [ ] **UX-01**: Mouse wheel scroll works in session list and other scrollable areas (settings, search, dialogs) (#262, #254)
- [ ] **UX-02**: Settings panel shows custom tool icons from ToolDef in the default tool radio group (#318)
- [ ] **UX-03**: auto_cleanup option documented in README sandbox section with explanation of what gets cleaned and when (#228)

## Future Requirements

Deferred to v1.4+. Tracked but not in current roadmap.

### Mouse Interaction

- **MOUSE-01**: Mouse click-to-select session in list (requires coordinate hit-testing against custom list renderer)
- **MOUSE-02**: Double-click or click-then-Enter to attach (requires click-select first + stateful double-click detection)

### Infrastructure

- **INFRA-01**: Redundant heartbeat mechanism cleanup (systemd timer vs bridge.py heartbeat_loop, #225)
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
| STORE-01 | Phase 11 | Pending |
| STORE-02 | Phase 11 | Pending |
| STORE-03 | Phase 11 | Pending |
| UX-03 | Phase 11 | Pending |
| VIS-01 | Phase 12 | Pending |
| VIS-02 | Phase 12 | Pending |
| VIS-03 | Phase 12 | Pending |
| DEDUP-01 | Phase 13 | Pending |
| DEDUP-02 | Phase 13 | Pending |
| DEDUP-03 | Phase 13 | Pending |
| PLAT-01 | Phase 14 | Pending |
| PLAT-02 | Phase 14 | Pending |
| UX-01 | Phase 15 | Pending |
| UX-02 | Phase 15 | Pending |

**Coverage:**
- v1.3 requirements: 14 total
- Mapped to phases: 14
- Unmapped: 0

**Phase distribution:**
- Phase 11 (Storage Foundation): STORE-01, STORE-02, STORE-03, UX-03
- Phase 12 (Session List Correctness): VIS-01, VIS-02, VIS-03
- Phase 13 (Resume Deduplication): DEDUP-01, DEDUP-02, DEDUP-03
- Phase 14 (Auto-Start TTY Fix): PLAT-01, PLAT-02
- Phase 15 (Mouse Support & Settings Polish): UX-01, UX-02

---
*Requirements defined: 2026-03-12*
*Last updated: 2026-03-12 — UX-03 moved to Phase 11 (bundled with storage, zero risk, prevents deferral)*
