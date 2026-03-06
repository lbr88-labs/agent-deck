---
phase: 1
slug: skills-reorganization
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-06
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go test + bash validation scripts (quick_validate.py) |
| **Config file** | Makefile (test target) |
| **Quick run command** | `python3 ~/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py <skill-path>` |
| **Full suite command** | `make test && python3 ~/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py skills/agent-deck && python3 ~/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py skills/session-share && python3 ~/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py ~/.agent-deck/skills/pool/gsd-conductor` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `python3 ~/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py <modified-skill-path>`
- **After every plan wave:** Run full suite command (validate all three skills)
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | SKILL-01 | smoke | `python3 ~/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py skills/agent-deck` | N/A (script) | pending |
| 1-01-02 | 01 | 1 | SKILL-04 | smoke | `python3 ~/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py skills/agent-deck` | N/A (script) | pending |
| 1-01-03 | 01 | 1 | SKILL-02 | smoke | `python3 ~/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py skills/session-share` | N/A (script) | pending |
| 1-01-04 | 01 | 1 | SKILL-05 | smoke | `grep -c 'SKILL_DIR' skills/session-share/SKILL.md` | N/A | pending |
| 1-02-01 | 02 | 1 | SKILL-03 | manual | Visual diff of gsd-conductor content against GSD source | N/A | pending |
| 1-02-02 | 02 | 1 | SKILL-05 | smoke | `test -f ~/.agent-deck/skills/pool/gsd-conductor/SKILL.md && echo OK` | N/A | pending |

*Status: pending · green · red · flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. No new test files needed:
- `quick_validate.py` from skill-creator handles frontmatter and naming validation
- `make test` covers Go unit tests (not directly applicable to skill structure)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| gsd-conductor content is current | SKILL-03 | Requires semantic comparison of SKILL.md against live GSD docs | Diff gsd-conductor SKILL.md sections against `.claude/get-shit-done/` directory structure and agent types |
| Path resolution from plugin cache | SKILL-05 | Requires Claude Code environment with plugin installed | Load skill via `Read ~/.claude/plugins/cache/.../skills/agent-deck/SKILL.md`, verify script paths resolve |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
