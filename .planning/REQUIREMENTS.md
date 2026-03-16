# Requirements: Agent Deck

**Defined:** 2026-03-16
**Core Value:** Reliable terminal session management for AI coding agents with conductor orchestration

## v0.26.2 Requirements

Requirements for patch release v0.26.2 Stability Fixes. Each maps to roadmap phases.

### Release Pipeline

- [x] **REL-01**: Install script successfully downloads and installs the current release
- [x] **REL-02**: CI workflow validates that all platform assets exist before publishing a release
- [x] **REL-03**: v0.26.2 release includes all 4 platform binaries (darwin_amd64, darwin_arm64, linux_amd64, linux_arm64)

### Slack Bridge

- [ ] **SLACK-01**: Outbound Slack messages convert GFM headers, bold, strikethrough, links, and bullets to mrkdwn
- [ ] **SLACK-02**: Code blocks and inline code pass through to Slack unchanged

### Key Input

- [x] **KEY-01**: Uppercase/shifted key shortcuts (M, R, F, etc.) trigger on Wayland compositors
- [x] **KEY-02**: Uppercase characters can be typed in TUI text input fields on Wayland

## Future Requirements

Deferred. v1.3 remaining phases (11, 15, 16) resume after v0.26.2 ships.

### v1.3 Remaining (paused)
- **MCP-01**: MCP proxy request ID collision fix (#324)
- **TEST-01 through TEST-10**: Comprehensive testing suite (Phase 16)

### Mouse Interaction
- **MOUSE-01**: Mouse click-to-select session in list
- **MOUSE-02**: Double-click or click-then-Enter to attach

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
| New features (#317, #313, #298, etc.) | Patch release: bug fixes only |
| Refactoring | Stability focus, minimal code changes |
| v1.3 remaining phases (11, 15, 16) | Paused, resume after v0.26.2 ships |
| `bubbles/list` migration | Full rewrite risk, not a bug fix |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| REL-01 | Phase 17 | Complete |
| REL-02 | Phase 17 | Complete |
| REL-03 | Phase 17 | Complete |
| SLACK-01 | Phase 17 | Pending |
| SLACK-02 | Phase 17 | Pending |
| KEY-01 | Phase 18 | Complete |
| KEY-02 | Phase 18 | Complete |

**Coverage:**
- v0.26.2 requirements: 7 total
- Mapped to phases: 7
- Unmapped: 0

---
*Requirements defined: 2026-03-16*
*Last updated: 2026-03-16 after roadmap creation (Phases 17-18)*
