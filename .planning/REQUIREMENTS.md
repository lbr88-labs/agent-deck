# Requirements: Agent Deck Integration Testing Framework

**Defined:** 2026-03-06
**Core Value:** Conductor orchestration and cross-session coordination must be reliably tested end-to-end

## v1.1 Requirements

Requirements for integration testing milestone. Each maps to roadmap phases.

### Test Framework Infrastructure

- [ ] **INFRA-01**: Shared TmuxHarness helper provides session create/cleanup/naming with t.Cleanup teardown
- [ ] **INFRA-02**: Polling helpers (WaitForCondition, WaitForPaneContent, WaitForStatus) replace flaky time.Sleep assertions
- [ ] **INFRA-03**: SQLite fixture helpers provide test storage factory, instance builders, and conductor fixtures
- [ ] **INFRA-04**: Integration package has TestMain with AGENTDECK_PROFILE=_test isolation and orphan session cleanup

### Session Lifecycle

- [ ] **LIFE-01**: Session start creates real tmux session and transitions status (starting -> running)
- [ ] **LIFE-02**: Session stop terminates tmux session and updates status correctly
- [ ] **LIFE-03**: Session fork creates independent copy with env var propagation and parent-child linkage in SQLite
- [ ] **LIFE-04**: Session restart with flags (yolo, etc.) recreates session correctly

### Status Detection

- [ ] **DETECT-01**: Sleep/wait detection correctly identifies patterns for Claude, Gemini, OpenCode, and Codex via simulated output
- [ ] **DETECT-02**: Multi-tool session creation produces correct commands and detection config per tool type
- [ ] **DETECT-03**: Status transition cycle (starting -> running -> waiting -> idle) verified with real tmux pane content

### Conductor Orchestration

- [ ] **COND-01**: Conductor sends command to child session via real tmux and child receives it
- [ ] **COND-02**: Cross-session event notification cycle works (event written, watcher detects, parent notified)
- [ ] **COND-03**: Conductor heartbeat round-trip completes (send heartbeat, child responds, verify receipt)
- [ ] **COND-04**: Send-with-retry delivers to real tmux session with chunked sending and paste-marker detection

### Edge Cases

- [ ] **EDGE-01**: Skills discovered from directory, attached to session, trigger conditions evaluated correctly
- [ ] **EDGE-02**: Concurrent polling of 10+ sessions returns correct status for each without races
- [ ] **EDGE-03**: Storage watcher detects external SQLite changes from a second Storage instance

## v2 Requirements

### Test Ecosystem

- **TEST-EXT-01**: TUI (Bubble Tea) integration tests via tea.Test or VHS
- **TEST-EXT-02**: CI/CD pipeline integration with tmux server in GitHub Actions
- **TEST-EXT-03**: Performance/load benchmarks for session polling hot paths
- **TEST-EXT-04**: MCP attach/detach scoped config integration tests
- **TEST-EXT-05**: Event watcher recovery after directory recreation

## Out of Scope

| Feature | Reason |
|---------|--------|
| Full end-to-end tests with real AI tools | Requires API keys, costs money, flaky, violates public repo constraint |
| TUI (Bubble Tea) integration tests | Separate approach needed (tea.Test/VHS), home.go is 8500 lines |
| Docker-based test isolation | tmux in Docker is fragile, adds CGO dependency |
| CI/CD pipeline integration | Tests run locally, CI is a separate effort |
| Performance/load testing | Different infrastructure needed, separate concern |
| Parallel integration test execution | tmux global namespace causes race conditions |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFRA-01 | Phase 4 | Pending |
| INFRA-02 | Phase 4 | Pending |
| INFRA-03 | Phase 4 | Pending |
| INFRA-04 | Phase 4 | Pending |
| LIFE-01 | Phase 4 | Pending |
| LIFE-02 | Phase 4 | Pending |
| LIFE-03 | Phase 4 | Pending |
| LIFE-04 | Phase 4 | Pending |
| DETECT-01 | Phase 5 | Pending |
| DETECT-02 | Phase 5 | Pending |
| DETECT-03 | Phase 5 | Pending |
| COND-01 | Phase 5 | Pending |
| COND-02 | Phase 5 | Pending |
| COND-03 | Phase 6 | Pending |
| COND-04 | Phase 6 | Pending |
| EDGE-01 | Phase 6 | Pending |
| EDGE-02 | Phase 6 | Pending |
| EDGE-03 | Phase 6 | Pending |

**Coverage:**
- v1.1 requirements: 18 total
- Mapped to phases: 18
- Unmapped: 0

---
*Requirements defined: 2026-03-06*
*Last updated: 2026-03-06 after roadmap creation*
