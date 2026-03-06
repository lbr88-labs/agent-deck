---
phase: 09-process-stability
verified: 2026-03-06T21:09:32Z
status: passed
score: 7/7 must-haves verified
re_verification: false
---

# Phase 9: Process Stability Verification Report

**Phase Goal:** The cause of exit 137 (SIGKILL) on incoming messages is identified, and either fixed in agent-deck or documented as a Claude Code limitation with a practical mitigation strategy
**Verified:** 2026-03-06T21:09:32Z
**Status:** passed
**Re-verification:** No (initial verification)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | The root cause of exit 137 is identified with evidence, not speculation | VERIFIED | 09-INVESTIGATION.md attributes root cause to Claude Code with 4 evidence categories: tmux send-keys code analysis, control test methodology, production LEARNINGS data (10+ recurrences), Claude Code interrupt-on-input design analysis. 30 mentions of "Claude Code" in the document. |
| 2 | Reproduction steps are documented so anyone can observe the behavior | VERIFIED | 09-INVESTIGATION.md "Reproduction Steps" section has 5 numbered steps plus a control test (4 steps) showing the behavior with Claude Code vs. raw shell. |
| 3 | The responsible component is identified: agent-deck, tmux, or Claude Code | VERIFIED | Root Cause section explicitly states "Component responsible: Claude Code (Anthropic's CLI application)". Control test proves tmux send-keys does not send signals. |
| 4 | A clear fixable-or-not determination is made with justification | VERIFIED | "Fixability Determination" section: "Can agent-deck fix the root cause? NO." with 3 specific reasons. Also covers tmux-level changes (NO) and timing mitigations (YES, partial). |
| 5 | Conductor operators know why exit 137 happens and how to avoid triggering it | VERIFIED | `~/.agent-deck/conductor/CLAUDE.md` contains "Exit 137: Tool Interruption on Incoming Messages" section (line 74) with cause explanation, 5 numbered mitigations, and "What Does NOT Help" subsection. |
| 6 | GSD conductor sessions have specific guidance on sending to sessions with running tools | VERIFIED | `~/.agent-deck/skills/pool/gsd-conductor/SKILL.md` contains "Exit 137: Protecting GSD Sessions from Tool Interruption" section (line 76) with 5 GSD-specific mitigations and a "Recovery from Exit 137" subsection. |
| 7 | The mitigation strategy is actionable, not just explanatory | VERIFIED | Both documents use numbered action items with specific commands (e.g., `session output <id> -q`, `session send ... --wait`, `agent-deck launch ... -m`). Conductor CLAUDE.md explicitly states what does NOT help (nohup, background processes, signal trapping). |

**Score:** 7/7 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.planning/phases/09-process-stability/09-INVESTIGATION.md` | Root cause analysis with reproduction steps, signal trace, and fixability determination | VERIFIED | 173 lines. Contains all 6 required sections: Summary, Reproduction Steps, Signal Chain Analysis, Root Cause, Fixability Determination, Mitigation Strategies. No TODOs or placeholders. Commit 7b7f622 verified. |
| `~/.agent-deck/conductor/CLAUDE.md` | Exit 137 mitigation section in shared conductor knowledge base | VERIFIED | Exit 137 section at line 74, placed between Heartbeat Protocol (line 49) and State Management (line 95) as planned. 5 mitigations + what-doesn't-help. No placeholders. |
| `~/.agent-deck/skills/pool/gsd-conductor/SKILL.md` | GSD-specific guidance on avoiding tool interruption during send | VERIFIED | Exit 137 section at line 76, placed before GSD Lifecycle (line 99). 5 GSD-specific mitigations + recovery procedure. No placeholders. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `~/.agent-deck/conductor/CLAUDE.md` | conductor sessions | loaded at startup per CLAUDE.md instructions | VERIFIED | Section "Exit 137: Tool Interruption on Incoming Messages" present with keyword "Exit 137" at line 74. Any conductor session loading this file will see the guidance. |
| `~/.agent-deck/skills/pool/gsd-conductor/SKILL.md` | GSD conductor sessions | loaded on demand from pool | VERIFIED | Section "Exit 137: Protecting GSD Sessions from Tool Interruption" present with keyword "exit 137" at lines 76, 86, 92, 94. Pool skill loads when a conductor reads the SKILL.md. |
| Both documents | Consistent recommendations | shared patterns | VERIFIED | Both recommend: (1) check status before sending, (2) use --wait flag, (3) use session output for read-only checks. GSD adds GSD-specific patterns (launch over send, read STATE.md directly). |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| PROC-01 | 09-01, 09-02 | Incoming messages do not kill running Bash tool child processes with SIGKILL (exit 137), or mitigation is documented if this is a Claude Code limitation | SATISFIED | The "or" clause is satisfied: root cause identified as Claude Code limitation (not fixable in agent-deck), mitigation documented in conductor CLAUDE.md and GSD conductor SKILL.md. REQUIREMENTS.md shows PROC-01 as `[x]` Complete. |

No orphaned requirements found. Phase 9 maps to PROC-01 only, and PROC-01 appears in both plans (09-01 and 09-02).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None | - | - | - | No anti-patterns detected in any phase artifacts |

All three artifacts scanned for TODO, FIXME, XXX, HACK, PLACEHOLDER, "coming soon", "will be here" patterns. None found.

### Human Verification Required

### 1. Exit 137 Reproduction

**Test:** Follow the reproduction steps in 09-INVESTIGATION.md with an actual Claude Code session to confirm the SIGKILL behavior matches the documented analysis.
**Expected:** Running `tmux send-keys` to a session with an active Bash tool should result in the tool being killed with exit 137, while the same test against a raw shell should NOT kill the running process.
**Why human:** The investigation was conducted through code analysis and production evidence rather than a live reproduction test. A manual run would provide additional confidence.

### 2. Conductor Document Discovery

**Test:** Start a new conductor session and verify it reads the exit 137 section from `~/.agent-deck/conductor/CLAUDE.md` at startup.
**Expected:** The conductor should reference exit 137 mitigations when deciding how to interact with running sessions.
**Why human:** Cannot programmatically verify that a conductor session actually reads and applies the guidance during real orchestration.

### Gaps Summary

No gaps found. All 7 observable truths are verified. All 3 required artifacts exist, are substantive, and are wired to their consumers. The single requirement (PROC-01) is satisfied through the documented mitigation path (Claude Code limitation with practical mitigations). Both mitigation documents are consistent in their recommendations and placed in the correct locations for their respective audiences.

The phase achieved its goal: the root cause of exit 137 is identified (Claude Code's interrupt-on-input behavior), the fixability determination is definitive (not fixable in agent-deck, with clear justification), and practical mitigation strategies are documented where conductor sessions will find them.

---

_Verified: 2026-03-06T21:09:32Z_
_Verifier: Claude (gsd-verifier)_
