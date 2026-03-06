# Requirements: Agent Deck

**Defined:** 2026-03-07
**Core Value:** Conductor orchestration and cross-session coordination must work reliably in production

## v1.2 Requirements

Requirements for Conductor Reliability & Learnings Cleanup milestone. Each maps to roadmap phases.

### Heartbeat

- [x] **HB-01**: Heartbeat scripts filter sessions by the conductor's own group instead of reporting all sessions across all groups
- [x] **HB-02**: Heartbeat respects `conductor.enabled = false` and `heartbeat_interval = 0` by stopping launchd services or checking config before sending

### Send Reliability

- [x] **SEND-01**: Session send reliably submits Enter key after pasting text into tmux, eliminating the race condition between paste and keypress
- [x] **SEND-02**: Messages sent to Codex sessions wait for Codex to attach to stdin before delivery, preventing text from going to the underlying shell

### Process Stability

- [x] **PROC-01**: Incoming messages to the conductor do not kill running Bash tool child processes with SIGKILL (exit 137), or mitigation is documented if this is a Claude Code limitation

### CLI Reliability

- [x] **CLI-01**: `session send --wait` exits cleanly with correct status codes and does not hang on edge cases
- [x] **CLI-02**: Using `-cmd` flag does not break `-group` flag parsing; `-c` shorthand is documented as the supported pattern
- [x] **CLI-03**: `--no-parent` followed by `set-parent` correctly restores parent routing, or `--no-parent` emits a clear warning about permanent effects

### Learnings Promotion

- [x] **LEARN-01**: Validated universal conductor patterns (event-driven monitoring, parent linkage, session transition verification, Enter key workaround) are promoted to the shared conductor CLAUDE.md
- [x] **LEARN-02**: GSD-specific learnings (Claude-only, codebase mapping, comprehensive specs, wave model) are promoted to the GSD conductor skill
- [x] **LEARN-03**: Agent-deck operational learnings (Codex launch syntax, release sessions, Gemini video, --wait patterns, project folder launching) are promoted to the agent-deck skill
- [x] **LEARN-04**: All conductor LEARNINGS.md files are cleaned up: promoted entries marked, retired entries removed, duplicates consolidated

## v1.1 Requirements (Complete)

<details>
<summary>18 requirements, all complete</summary>

### Test Framework Infrastructure

- [x] **INFRA-01**: Shared TmuxHarness helper provides session create/cleanup/naming with t.Cleanup teardown
- [x] **INFRA-02**: Polling helpers (WaitForCondition, WaitForPaneContent, WaitForStatus) replace flaky time.Sleep assertions
- [x] **INFRA-03**: SQLite fixture helpers provide test storage factory, instance builders, and conductor fixtures
- [x] **INFRA-04**: Integration package has TestMain with AGENTDECK_PROFILE=_test isolation and orphan session cleanup

### Session Lifecycle

- [x] **LIFE-01**: Session start creates real tmux session and transitions status (starting -> running)
- [x] **LIFE-02**: Session stop terminates tmux session and updates status correctly
- [x] **LIFE-03**: Session fork creates independent copy with env var propagation and parent-child linkage in SQLite
- [x] **LIFE-04**: Session restart with flags (yolo, etc.) recreates session correctly

### Status Detection

- [x] **DETECT-01**: Sleep/wait detection correctly identifies patterns for Claude, Gemini, OpenCode, and Codex via simulated output
- [x] **DETECT-02**: Multi-tool session creation produces correct commands and detection config per tool type
- [x] **DETECT-03**: Status transition cycle (starting -> running -> waiting -> idle) verified with real tmux pane content

### Conductor Orchestration

- [x] **COND-01**: Conductor sends command to child session via real tmux and child receives it
- [x] **COND-02**: Cross-session event notification cycle works (event written, watcher detects, parent notified)
- [x] **COND-03**: Conductor heartbeat round-trip completes (send heartbeat, child responds, verify receipt)
- [x] **COND-04**: Send-with-retry delivers to real tmux session with chunked sending and paste-marker detection

### Edge Cases

- [x] **EDGE-01**: Skills discovered from directory, attached to session, trigger conditions evaluated correctly
- [x] **EDGE-02**: Concurrent polling of 10+ sessions returns correct status for each without races
- [x] **EDGE-03**: Storage watcher detects external SQLite changes from a second Storage instance

</details>

## Future Requirements

Deferred to future milestones. Tracked but not in current roadmap.

### Testing Expansion

- **TEXP-01**: Integration tests for Codex readiness detection with real Codex binary
- **TEXP-02**: Performance benchmarks for concurrent session polling at scale (50+ sessions)
- **TEST-EXT-01**: TUI (Bubble Tea) integration tests via tea.Test or VHS
- **TEST-EXT-02**: CI/CD pipeline integration with tmux server in GitHub Actions

## Out of Scope

| Feature | Reason |
|---------|--------|
| Project-specific learnings (ARD deploy, Ryan ElevenLabs) | Stay in their respective conductor LEARNINGS.md files |
| Personal preferences (voice-to-text parsing) | Stay in user CLAUDE.md, not shared |
| New features unrelated to conductor reliability | This is a bugfix and cleanup milestone |
| UI/TUI testing | Requires separate Bubble Tea testing approach |
| CI/CD pipeline integration | Tests run locally; CI integration is a future milestone |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SEND-01 | Phase 7 | Complete |
| SEND-02 | Phase 7 | Complete |
| HB-01 | Phase 8 | Complete |
| HB-02 | Phase 8 | Complete |
| CLI-01 | Phase 8 | Complete |
| CLI-02 | Phase 8 | Complete |
| CLI-03 | Phase 8 | Complete |
| PROC-01 | Phase 9 | Complete |
| LEARN-01 | Phase 10 | Complete |
| LEARN-02 | Phase 10 | Complete |
| LEARN-03 | Phase 10 | Complete |
| LEARN-04 | Phase 10 | Complete |

**Coverage:**
- v1.2 requirements: 12 total
- Mapped to phases: 12
- Unmapped: 0

---
*Requirements defined: 2026-03-07*
*Last updated: 2026-03-07 after roadmap creation (phases 7-10)*
