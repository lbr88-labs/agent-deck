# Requirements: Agent Deck Skills Reorganization & Stabilization

**Defined:** 2026-03-06
**Core Value:** Skills must load correctly and trigger reliably when sessions start or on demand

## v1 Requirements

### Skills Reformatting

- [x] **SKILL-01**: Agent-deck skill uses official skill-creator format with proper SKILL.md frontmatter, scripts/, and references/ directories
- [x] **SKILL-02**: Session-share skill uses official skill-creator format with proper SKILL.md frontmatter and scripts/
- [x] **SKILL-03**: GSD conductor skill is properly packaged in ~/.agent-deck/skills/pool/gsd-conductor/ with up-to-date content
- [x] **SKILL-04**: All skill SKILL.md files have correct frontmatter (name, description, compatibility fields)
- [x] **SKILL-05**: Skill script path resolution works correctly from both plugin cache and local development paths

### Testing

- [x] **TEST-01**: Sleep/wake detection correctly transitions session status (running -> idle -> running on activity)
- [x] **TEST-02**: Skills trigger correctly when referenced in session context or loaded on demand
- [x] **TEST-03**: Session start creates tmux session and transitions to running status
- [x] **TEST-04**: Session stop cleanly terminates tmux session and updates status
- [x] **TEST-05**: Session fork creates independent copy with correct instance ID propagation
- [x] **TEST-06**: Session attach connects to existing tmux session without errors
- [x] **TEST-07**: Session status tracking reflects actual tmux session state accurately

### Stabilization

- [x] **STAB-01**: All bugs discovered during testing are fixed
- [x] **STAB-02**: `golangci-lint run` passes with zero warnings
- [x] **STAB-03**: `go test -race ./...` passes with zero failures
- [x] **STAB-04**: `go build` succeeds for all target platforms (darwin/linux, amd64/arm64)
- [x] **STAB-05**: Dead code and stale artifacts removed from codebase
- [ ] **STAB-06**: CHANGELOG.md updated with all changes

## v2 Requirements

### Skills Ecosystem

- **SKILL-06**: Pool skill auto-discovery (list available pool skills from TUI)
- **SKILL-07**: Skill versioning and update mechanism
- **SKILL-08**: Skill dependency resolution

## Out of Scope

| Feature | Reason |
|---------|--------|
| New agent-deck features | This milestone is reorganization and stabilization only |
| CI/CD pipeline changes | Local release process is sufficient |
| Version bump | Deferred until work is assessed |
| Skills for other tools (Gemini, etc.) | Focus on Claude Code skills only |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| SKILL-01 | Phase 1 | Complete |
| SKILL-02 | Phase 1 | Complete |
| SKILL-03 | Phase 1 | Complete |
| SKILL-04 | Phase 1 | Complete |
| SKILL-05 | Phase 1 | Complete |
| TEST-01 | Phase 2 | Complete |
| TEST-02 | Phase 2 | Complete |
| TEST-03 | Phase 2 | Complete |
| TEST-04 | Phase 2 | Complete |
| TEST-05 | Phase 2 | Complete |
| TEST-06 | Phase 2 | Complete |
| TEST-07 | Phase 2 | Complete |
| STAB-01 | Phase 2 | Complete |
| STAB-02 | Phase 3 | Complete |
| STAB-03 | Phase 3 | Complete |
| STAB-04 | Phase 3 | Complete |
| STAB-05 | Phase 3 | Complete |
| STAB-06 | Phase 3 | Pending |

**Coverage:**
- v1 requirements: 18 total
- Mapped to phases: 18
- Unmapped: 0

---
*Requirements defined: 2026-03-06*
*Last updated: 2026-03-06 after roadmap creation*
