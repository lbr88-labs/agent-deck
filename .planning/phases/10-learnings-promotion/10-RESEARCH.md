# Phase 10: Learnings Promotion - Research

**Researched:** 2026-03-07
**Domain:** Documentation curation, knowledge management, conductor skill maintenance
**Confidence:** HIGH

## Summary

Phase 10 is a documentation-only phase with zero code changes. Seven LEARNINGS.md files exist across the conductor directory tree (one shared root file and six per-conductor files). The content must be audited, categorized by destination, and promoted into three shared locations: the conductor CLAUDE.md, the GSD conductor SKILL.md, and the agent-deck skill. After promotion, each LEARNINGS.md entry must be marked as `promoted`, and entries already marked `retired` should be removed.

The core challenge is classification: each learning must be categorized as (a) universal conductor pattern, (b) GSD-specific, (c) agent-deck operational, or (d) project-specific (stays in its conductor LEARNINGS.md). The requirements explicitly exclude project-specific learnings (ARD deploy, Ryan ElevenLabs, opengraphdb Rust/Codex sandbox) from promotion. The shared conductor CLAUDE.md already has some content added during Phase 9 (exit 137 mitigations), so promotions must merge without duplicating that work.

**Primary recommendation:** Split into two plans: (1) audit all learnings, classify each, and promote to the three destinations; (2) clean up all LEARNINGS.md files by marking promoted entries and removing retired ones.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| LEARN-01 | Universal conductor patterns promoted to shared conductor CLAUDE.md | Inventory below identifies 7 entries across 5 files that qualify as universal patterns |
| LEARN-02 | GSD-specific learnings promoted to GSD conductor skill | Inventory identifies 6 entries from agent-deck and root LEARNINGS.md |
| LEARN-03 | Agent-deck operational learnings promoted to agent-deck skill | Inventory identifies 5 entries from ryan, ard, and agent-deck LEARNINGS.md |
| LEARN-04 | All LEARNINGS.md files cleaned up (promoted marked, retired removed, duplicates consolidated) | 7 files totaling 712 lines; 2 files empty (si, work); 2 entries already retired (ard 004, 006) |
</phase_requirements>

## Inventory of All Learnings

### File 1: Root `~/.agent-deck/conductor/LEARNINGS.md` (53 lines, 4 entries)

| Entry ID | Description | Category | Destination | Notes |
|----------|-------------|----------|-------------|-------|
| 20260224-001 | Open-ended prompts beat directed ones | Universal | CLAUDE.md | Recurrence 1, but corroborated by ryan 014 and 017 |
| 20260226-002 | Event routing is parent-only; keep parent linkage | Universal | CLAUDE.md | Recurrence 2, also in ard 008, 011, 012, 013 |
| 20260301-003 | GSD: one session per stage, never auto-advance | GSD | gsd-conductor SKILL.md | Recurrence 3, also in agent-deck 001 |
| 20260301-004 | Enter key submission failure is top pain point | Universal | CLAUDE.md | Recurrence 15+, already partially in CLAUDE.md (exit 137 section). This is about verification after send, not exit 137 |

### File 2: `agent-deck/LEARNINGS.md` (101 lines, 10 entries)

| Entry ID | Description | Category | Destination | Notes |
|----------|-------------|----------|-------------|-------|
| 20260301-001 | GSD auto-advance exhausts context | GSD | gsd-conductor SKILL.md | Duplicate of root 003 |
| 20260301-002 | Enter bug is #1 operational pain point | Universal | CLAUDE.md | Duplicate of root 004 |
| 20260301-003 | Driving GSD interactive prompts via tmux | GSD | gsd-conductor SKILL.md | Already partially covered in SKILL.md |
| 20260301-004 | Feed comprehensive specs to GSD --auto | GSD | gsd-conductor SKILL.md | New content for SKILL.md |
| 20260304-001 | Stay focused on agent-deck only | Project-specific | Stays | Agent-deck conductor identity rule |
| 20260301-005 | Parallel GSD executors work for independent plans | GSD | gsd-conductor SKILL.md | Validates wave model |
| 20260304-002 | Run /gsd:map-codebase BEFORE /gsd:new-project | GSD | gsd-conductor SKILL.md | New lifecycle recommendation |
| 20260304-004 | Codex cannot run GSD commands (Claude only) | GSD | gsd-conductor SKILL.md | Critical constraint |
| 20260306-001 | Never do releases from the conductor | Agent-deck op | agent-deck skill | Launch dedicated session for releases |
| 20260304-003 | Heartbeat messages too verbose | Agent-deck op | agent-deck skill | Bridge improvement note |

### File 3: `ard/LEARNINGS.md` (170 lines, 17 entries)

| Entry ID | Description | Category | Destination | Notes |
|----------|-------------|----------|-------------|-------|
| 20260224-001 | -cmd flag breaks -group parsing | Agent-deck op | agent-deck skill | Recurrence 7 |
| 20260224-002 | Codex launch times out | Agent-deck op | agent-deck skill | Use add+start+send pattern |
| 20260224-003 | Always verify session group after launch | Universal | CLAUDE.md | Recurrence 3 |
| 20260224-004 | Sessions from conductor dir get conductor group | Retired | Remove | Already marked retired |
| 20260224-005 | Codex sessions unreliable with agent-deck | Agent-deck op | agent-deck skill | Recurrence 7, root cause documented |
| 20260224-006 | Working pattern for Codex with permissions | Retired | Remove | Already marked retired |
| 20260224-007 | Keep messages simple for launch -m | Universal | CLAUDE.md | Recurrence 4 |
| 20260224-008 | CRITICAL: unset-parent breaks notifications | Universal | CLAUDE.md | Duplicate of root 002 |
| 20260224-009 | Use agent-deck rm to delete sessions | Agent-deck op | agent-deck skill | CLI reference |
| 20260224-010 | Never sleep-poll, use heartbeats | Universal | CLAUDE.md | Recurrence 8+, also in ryan 011 |
| 20260226-011 | Verified parent+group routing works | Universal | CLAUDE.md | Consolidate with root 002 |
| 20260226-012 | --no-parent + set-parent does NOT work | Universal | CLAUDE.md | Recurrence 6+ |
| 20260227-013 | Pass full issue context to fix sessions | Project-specific | Stays | ARD-specific workflow |
| 20260302-014 | Verification must cover ALL symptoms | Universal | CLAUDE.md | General orchestration principle |
| 20260306-015 | Always bump ALL package versions | Project-specific | Stays | ARD deploy pattern |
| 20260306-016 | Deploy must rebuild ALL containers | Project-specific | Stays | ARD deploy pattern |
| 20260306-017 | Use Gemini sessions for video analysis | Agent-deck op | agent-deck skill | Multi-tool pattern |

### File 4: `opengraphdb/LEARNINGS.md` (172 lines, ~20 entries)

| Entry ID | Description | Category | Destination | Notes |
|----------|-------------|----------|-------------|-------|
| Session management (group launch) | Always launch in opengraphdb group | Project-specific | Stays | Opengraphdb-specific |
| Session identification | Exact title, full ID, 8-char prefix | Agent-deck op | agent-deck skill | General CLI knowledge |
| Duplicate titles | Avoid duplicate titles | Agent-deck op | agent-deck skill | General agent-deck behavior |
| Cleanup commands | Use rm to remove, stop sets error | Agent-deck op | agent-deck skill | Duplicate of ard 009 |
| Resolution priority | Exact title > Full ID > 8-char prefix | Agent-deck op | agent-deck skill | General CLI knowledge |
| Key commands reference | launch/send/output/show/remove/stop | Agent-deck op | agent-deck skill | Already in CLAUDE.md |
| Status meanings | running/waiting/idle/error | Universal | Already in CLAUDE.md | Duplicate |
| Stuck sessions: interactive prompts | tmux capture-pane workaround | GSD + Universal | gsd-conductor SKILL.md | Already there |
| Sending: tmux vs agent-deck send | Prefer session send over tmux send-keys | Universal | CLAUDE.md | Important distinction |
| API ConnectionRefused recovery | Stall detection via heartbeat polling | Universal | CLAUDE.md | General resilience pattern |
| Codex readiness detection | launch -c codex fails, use add+start+send | Agent-deck op | agent-deck skill | Codex pattern |
| Parallel Codex target/ conflicts | Use worktrees for parallel Codex | Project-specific | Stays | Rust-specific |
| Codex sandbox blocks socket tests | Use 'p' for permanent approve | Project-specific | Stays | Codex sandbox specifics |
| Pipeline health validation | Events work, sequential reliable | Universal | CLAUDE.md | Operational confirmation |
| Codex analysis paralysis | "START WRITING CODE IMMEDIATELY" | Project-specific | Stays | Codex-specific |
| Codex sandbox blocks npm install | Pre-install packages | Project-specific | Stays | Codex sandbox |
| Codex sandbox blocks writes outside root | Approve with tmux send-keys | Project-specific | Stays | Codex sandbox |
| React 19 + neo4j peer dep conflict | --legacy-peer-deps | Project-specific | Stays | opengraphdb-specific |
| Codex launch timeout pattern | add+start+sleep+send pattern | Agent-deck op | agent-deck skill | Confirmed pattern |
| Worktrees only contain git-tracked files | Copy source into worktrees | Agent-deck op | agent-deck skill | Worktree limitation |

### File 5: `ryan/LEARNINGS.md` (174 lines, 19 entries)

| Entry ID | Description | Category | Destination | Notes |
|----------|-------------|----------|-------------|-------|
| 20260215-001 | Never declare fix without visual verification | Project-specific | Stays | Ryan/Playwright pattern |
| 20260215-002 | Identify correct event for UI state changes | Project-specific | Stays | Ryan UI pattern |
| 20260215-003 | Sessions run out of context on complex tasks | Universal | CLAUDE.md | Recurrence 4, break into smaller sessions |
| 20260215-004 | User works iteratively with fast pivots | Project-specific | Stays | User behavior pattern |
| 20260215-005 | Voice-to-text messages need intent parsing | Project-specific | Stays | User-specific |
| 20260215-006 | --wait flag on session send | Agent-deck op | agent-deck skill | Recurrence 2 |
| 20260215-007 | Don't use sub-agents for ryan work | Project-specific | Stays | Ryan-specific |
| 20260215-008 | ElevenLabs ConvAI split: code vs dashboard | Project-specific | Stays | Ryan/ElevenLabs |
| 20260215-009 | Clean up stale error sessions proactively | Universal | CLAUDE.md | General hygiene pattern |
| 20260215-010 | Fuzzy search for voice-first apps | Project-specific | Stays | Ryan-specific |
| 20260224-011 | Use events instead of sleep+poll | Universal | CLAUDE.md | Duplicate of ard 010 |
| 20260224-013 | launch -m may not reliably deliver | Agent-deck op | agent-deck skill | Historical reliability note |
| 20260224-014 | Don't give sessions direct file paths | Universal | CLAUDE.md | Duplicate of root 001 |
| 20260303-015 | Codex launch syntax for bypass flag | Agent-deck op | agent-deck skill | Recurrence 3, correct flag name |
| 20260303-016 | Codex found real bugs unit tests missed | Project-specific | Stays | Verification insight |
| 20260303-017 | Don't hand-hold Codex, let it explore | Universal | CLAUDE.md | Consolidate with root 001, ryan 014 |
| 20260303-018 | ElevenLabs tools path | Project-specific | Stays | Ryan/ElevenLabs |
| 20260303-019 | Verify production agent ID matches local | Project-specific | Stays | Ryan/ElevenLabs |
| 20260224-012 | Exit 137 caused by incoming messages | Universal | Already in CLAUDE.md | Added in Phase 9 |

### File 6: `si/LEARNINGS.md` (21 lines, 0 entries)
Template only. No content to promote.

### File 7: `work/LEARNINGS.md` (21 lines, 0 entries)
Template only. No content to promote.

## Classification Summary

### Destination: Shared Conductor CLAUDE.md (`~/.agent-deck/conductor/CLAUDE.md`)

Universal patterns to add (not already present):

1. **Event-driven monitoring, not sleep+poll** (root 001-poll from ard 010, ryan 011). After sending work, wait for heartbeat or events. Never do `sleep && capture-pane` loops.
2. **Parent linkage is mandatory for event routing** (root 002, ard 008, 011, 012). Never use `--no-parent`. Use `-g <group>` while keeping parent. `--no-parent + set-parent` does NOT work.
3. **Enter key verification after send** (root 004, agent-deck 002). After any send, verify session transitions from waiting to running within 15s. Nudge with tmux send-keys Enter if needed.
4. **Verify session group after launch** (ard 003). Run `session show --json` to confirm group and parent are correct.
5. **Open-ended goals beat directed prompts** (root 001, ryan 014, 017). Give sessions high-level goals, not specific file paths.
6. **Break complex tasks into smaller sessions** (ryan 003). Keep prompts focused: one clear objective per session.
7. **Clean up stale error sessions proactively** (ryan 009). On heartbeat, clean if error sessions > threshold.
8. **Sending: prefer session send over tmux send-keys** (opengraphdb sending section). Use `session send` for messages, `tmux send-keys` only for interactive prompts. Use `tmux capture-pane` for reading.
9. **Verification must cover ALL symptoms** (ard 014). Check every symptom from the bug report, not just the headline.
10. **API ConnectionRefused recovery** (opengraphdb). Poll on heartbeats, don't rely solely on events for stalled session detection.

Already present (skip): Exit 137 mitigations (added in Phase 9), status values table, heartbeat protocol.

### Destination: GSD Conductor SKILL.md (`~/.agent-deck/skills/pool/gsd-conductor/SKILL.md`)

GSD-specific learnings to incorporate:

1. **One session per GSD stage, never auto-advance** (root 003, agent-deck 001). Set `auto_advance: false`. Each stage gets clean 200k context. Already partially present in "Recommended: One session per stage" section.
2. **GSD is Claude-only, Codex cannot run GSD commands** (agent-deck 004). GSD slash commands only work in Claude Code.
3. **Run /gsd:map-codebase before /gsd:new-project for brownfield** (agent-deck 002). Order: map-codebase, new-project, discuss, plan, execute, verify.
4. **Feed comprehensive specs to --auto** (agent-deck 004). Include project overview, open issues, changelog, architecture, operational pain points.
5. **Parallel executors work for independent (wave 1) plans** (agent-deck 005). Wave model validation: same-wave plans can run in parallel if they touch different files.
6. **Driving interactive prompts via tmux is reliable** (agent-deck 003). Already covered in "Critical: Interactive Prompt Handling" section.

### Destination: Agent-Deck Skill

The ROADMAP says "agent-deck operational learnings ... are incorporated into the agent-deck skill." There are two candidate skills:
- `agent-deck-workflow` at `~/.agent-deck/skills/pool/agent-deck-workflow/SKILL.md` (session creation workflow)
- `agent-deck-cli.skill` at `~/.agent-deck/skills/pool/agent-deck-cli.skill` (binary archive, not directly editable)

The agent-deck-workflow SKILL.md is the appropriate destination since it covers operational patterns. Agent-deck operational learnings to add:

1. **Codex launch syntax: --dangerously-bypass-approvals-and-sandbox** (ryan 015). Not `--dangerously-skip-permissions`.
2. **Codex launch: use add+start+sleep+send, not launch** (ard 002, opengraphdb codex timeout). `launch -c codex` frequently times out.
3. **Never do releases from conductor, launch dedicated session** (agent-deck 006). Conductor should never do git push, tag, or release directly.
4. **Gemini sessions for video analysis** (ard 017). Use `-c gemini` for video review workflows.
5. **--wait flag behavior and reliability** (ryan 006). Available but sometimes exits code 2. For critical flows, fall back to polling.
6. **-cmd flag breaks -group parsing** (ard 001). Always use `-c claude` or `-c codex` short form.
7. **Use agent-deck rm to delete sessions** (ard 009). Not `session remove`.
8. **Session identification: exact title > full ID > 8-char prefix** (opengraphdb). Nothing else works (no fuzzy, no partial).
9. **Worktrees only contain git-tracked files** (opengraphdb). Must copy untracked source files manually.
10. **launch -m may not reliably deliver initial message** (ryan 013). Verify first transition or confirm output changed.
11. **Heartbeat messages too verbose for multi-conductor** (agent-deck 003). Bridge improvement note.

## Architecture Patterns

### Promotion Workflow

```
For each LEARNINGS.md file:
  1. Read all entries
  2. Skip entries with Status: retired (will be removed)
  3. Skip entries with Status: promoted (already done)
  4. Classify each active entry:
     a. Universal conductor pattern -> CLAUDE.md
     b. GSD-specific -> gsd-conductor SKILL.md
     c. Agent-deck operational -> agent-deck-workflow SKILL.md
     d. Project-specific -> stays, no promotion needed
  5. For classified entries (a, b, c):
     - Check if content already exists at destination
     - If not, add it in the appropriate section
     - Mark source entry Status: promoted
  6. For retired entries: remove entirely
  7. For duplicates across files: keep the most detailed version, mark others as promoted with a cross-reference
```

### Destination File Structure

**CLAUDE.md** already has these sections: CLI Reference, Session Status Values, Heartbeat Protocol, Exit 137, State Management, Task Log, Self-Improvement, Quick Commands, Important Notes. New universal patterns should be added as a new section (e.g., "Operational Patterns" or "Orchestration Best Practices") placed between "Exit 137" and "State Management".

**gsd-conductor SKILL.md** already has: Installation, Interactive Prompt Handling, Exit 137, GSD Lifecycle, Driving GSD from Conductor, GSD File Structure, Reference. New GSD learnings should be incorporated into existing sections where they fit (e.g., "Claude-only" goes near the top, "map-codebase" goes into the Lifecycle section, "comprehensive specs" goes into the Driving GSD section).

**agent-deck-workflow SKILL.md** already has: Common Mistakes, Complete Session Creation Workflow, Session Status Reference, Group Management, Creating Multiple Sessions, Troubleshooting, Quick Reference Card. Operational learnings should be added to relevant existing sections or as a new "Operational Patterns" section.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Duplication detection | Manual diffing across 7 files | Pre-built inventory in this research | Already classified above |
| Section placement | Ad-hoc insertion | Follow existing document structure | Maintains readability |

**Key insight:** This is a curation task, not a creation task. The learnings already exist and are well-written. The work is classification, deduplication, and integration into existing document structures.

## Common Pitfalls

### Pitfall 1: Duplicating Content Already Added in Phase 9
**What goes wrong:** Exit 137 mitigations were already added to CLAUDE.md and gsd-conductor SKILL.md during Phase 9 (09-02-PLAN.md). Promoting the same content again creates redundancy.
**How to avoid:** Before writing any content to a destination file, read the current version and check for existing coverage. The exit 137 section in CLAUDE.md and gsd-conductor SKILL.md should NOT be touched.

### Pitfall 2: Promoting Project-Specific Learnings
**What goes wrong:** Entries like "React 19 peer dep conflict" (opengraphdb), "ElevenLabs tools path" (ryan), or "Deploy must rebuild ALL containers" (ard) get promoted to shared files, cluttering them with irrelevant context.
**How to avoid:** Requirements explicitly state: "Project-specific learnings (ARD deploy, Ryan ElevenLabs) stay in their respective conductor LEARNINGS.md files." Only promote entries classified as Universal, GSD, or Agent-deck operational.

### Pitfall 3: Breaking LEARNINGS.md Template Structure
**What goes wrong:** Removing all entries from a LEARNINGS.md file (e.g., si, work) or altering the "How to Use This File" header and "Entry Format" template.
**How to avoid:** Keep the template header and format section intact in every LEARNINGS.md file. Only modify entries below the `---` separator. Empty files (si, work) stay as-is since they have no entries.

### Pitfall 4: Overwriting vs Merging in Destination Files
**What goes wrong:** Rewriting entire sections of CLAUDE.md or SKILL.md, potentially losing existing content or changing well-established formatting.
**How to avoid:** Add new content surgically. For CLAUDE.md, add a new section rather than modifying existing ones. For SKILL.md, add to existing sections where appropriate, or add new subsections within them.

### Pitfall 5: Losing Cross-References
**What goes wrong:** When consolidating duplicates, losing the knowledge that a pattern was observed across multiple conductors and sessions.
**How to avoid:** When promoting, include a note about cross-conductor validation (e.g., "Validated across agent-deck, opengraphdb, and ryan conductors").

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Learnings scattered across 7 files | Centralized by category into 3 shared files | This phase | Conductors start with curated knowledge instead of raw experience logs |
| Active/retired/promoted status tracking | Same, but now enforced with cleanup | This phase | LEARNINGS.md files become living journals, not accumulating archives |

## File Locations (Quick Reference)

| File | Path | Role |
|------|------|------|
| Shared CLAUDE.md | `~/.agent-deck/conductor/CLAUDE.md` | LEARN-01 destination |
| Root LEARNINGS.md | `~/.agent-deck/conductor/LEARNINGS.md` | Source (4 entries) |
| agent-deck LEARNINGS.md | `~/.agent-deck/conductor/agent-deck/LEARNINGS.md` | Source (10 entries) |
| ard LEARNINGS.md | `~/.agent-deck/conductor/ard/LEARNINGS.md` | Source (17 entries) |
| opengraphdb LEARNINGS.md | `~/.agent-deck/conductor/opengraphdb/LEARNINGS.md` | Source (20 entries) |
| ryan LEARNINGS.md | `~/.agent-deck/conductor/ryan/LEARNINGS.md` | Source (19 entries) |
| si LEARNINGS.md | `~/.agent-deck/conductor/si/LEARNINGS.md` | Source (0 entries) |
| work LEARNINGS.md | `~/.agent-deck/conductor/work/LEARNINGS.md` | Source (0 entries) |
| GSD conductor SKILL.md | `~/.agent-deck/skills/pool/gsd-conductor/SKILL.md` | LEARN-02 destination |
| Agent-deck workflow SKILL.md | `~/.agent-deck/skills/pool/agent-deck-workflow/SKILL.md` | LEARN-03 destination |

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Manual verification (documentation phase, no code) |
| Config file | N/A |
| Quick run command | N/A |
| Full suite command | N/A |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LEARN-01 | Universal patterns in shared CLAUDE.md | manual-only | Grep for key phrases in CLAUDE.md | N/A |
| LEARN-02 | GSD learnings in gsd-conductor SKILL.md | manual-only | Grep for key phrases in SKILL.md | N/A |
| LEARN-03 | Agent-deck op learnings in agent-deck skill | manual-only | Grep for key phrases in SKILL.md | N/A |
| LEARN-04 | LEARNINGS.md cleanup complete | manual-only | Check no active entries that should be promoted remain | N/A |

**Justification for manual-only:** This is a documentation curation phase with no code changes. Verification means reading the destination files and confirming content was integrated correctly. No automated tests are meaningful here.

### Sampling Rate
- **Per task commit:** Visual diff review of changed files
- **Per wave merge:** N/A (no code, no merge)
- **Phase gate:** Read all 3 destination files and all 7 source files to confirm completeness

### Wave 0 Gaps
None. No test infrastructure needed for documentation curation.

## Open Questions

1. **Agent-deck skill location ambiguity**
   - What we know: The ROADMAP says "incorporated into the agent-deck skill." Two candidates exist: `agent-deck-workflow/SKILL.md` (plain text, editable) and `agent-deck-cli.skill` (binary archive). The `agent-deck-workflow` skill is the appropriate target since it covers operational patterns and is editable.
   - What's unclear: Whether the user intended a different location.
   - Recommendation: Use `agent-deck-workflow/SKILL.md` as the destination. If the user objects, it can be moved.

2. **Opengraphdb LEARNINGS.md format differs**
   - What we know: The opengraphdb file uses a different format (section headers instead of `[YYYYMMDD-NNN]` entry IDs). It has no explicit Status fields.
   - What's unclear: Whether to normalize the format during cleanup.
   - Recommendation: After promotion, add `Status: promoted` notes to promoted entries using inline comments, and leave the format otherwise unchanged. This conductor may have its own conventions.

## Sources

### Primary (HIGH confidence)
- All 7 LEARNINGS.md files: read in full
- `~/.agent-deck/conductor/CLAUDE.md`: read in full (188 lines)
- `~/.agent-deck/skills/pool/gsd-conductor/SKILL.md`: read in full (226 lines)
- `~/.agent-deck/skills/pool/agent-deck-workflow/SKILL.md`: read in full (316 lines)
- `.planning/REQUIREMENTS.md`: LEARN-01 through LEARN-04 definitions
- `.planning/ROADMAP.md`: Phase 10 success criteria
- `.planning/STATE.md`: Phase 9 decisions about exit 137 documentation

## Metadata

**Confidence breakdown:**
- Inventory completeness: HIGH. All 7 files read in full, every entry catalogued.
- Classification accuracy: HIGH. Based on explicit requirement descriptions and Out of Scope section in REQUIREMENTS.md.
- Destination file structure: HIGH. All 3 destination files read and analyzed.
- Agent-deck skill location: MEDIUM. Best guess based on available skills; could be wrong.

**Research date:** 2026-03-07
**Valid until:** No expiration (documentation curation, not library versions)
