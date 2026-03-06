# Phase 1: Skills Reorganization - Research

**Researched:** 2026-03-06
**Domain:** Anthropic Agent Skills format, skill packaging, path resolution
**Confidence:** HIGH

## Summary

Phase 1 requires reformatting three skills (agent-deck, session-share, gsd-conductor) to conform to the official Anthropic skill-creator format, then verifying that script path resolution works from both plugin cache paths (`~/.claude/plugins/cache/agent-deck/agent-deck/<hash>/skills/`) and local development paths (`<repo>/skills/`). The current skills already partially conform: both agent-deck and session-share have proper YAML frontmatter with `name` and `description` fields, and use `scripts/` and `references/` directories. The gsd-conductor skill in the pool already has valid structure.

The primary work is: (1) audit frontmatter against the official spec, (2) ensure `compatibility` is moved to `metadata` if kept (it is not a standard field), (3) verify the session-share skill is registered in `marketplace.json` (currently missing), (4) ensure gsd-conductor content is current and complete, and (5) validate that all script path references in SKILL.md files use the `$SKILL_DIR` pattern for portable resolution.

**Primary recommendation:** Audit each skill against the official `agent_skills_spec.md`, fix frontmatter field placement, add session-share to marketplace.json, and verify path resolution by running `quick_validate.py` on each skill.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SKILL-01 | Agent-deck skill uses official skill-creator format with proper SKILL.md frontmatter, scripts/, and references/ directories | Frontmatter spec documented below; current skill already has scripts/ and references/ but `compatibility` is non-standard |
| SKILL-02 | Session-share skill uses official skill-creator format with proper SKILL.md frontmatter and scripts/ | Same frontmatter fix needed; session-share is NOT registered in marketplace.json (critical gap) |
| SKILL-03 | GSD conductor skill is properly packaged in ~/.agent-deck/skills/pool/gsd-conductor/ with up-to-date content | Skill exists at correct path with SKILL.md + references/; need to verify content currency |
| SKILL-04 | All skill SKILL.md files have correct frontmatter (name, description, compatibility fields) | Official spec defines name + description (required), license + allowed-tools + metadata (optional); `compatibility` is NOT a standard field |
| SKILL-05 | Skill script path resolution works correctly from both plugin cache and local development paths | Agent-deck skill already documents $SKILL_DIR pattern; session-share skill uses relative `scripts/` paths (broken from plugin cache) |
</phase_requirements>

## Standard Stack

### Core
| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| Agent Skills Spec | 1.0 (2025-10-16) | Defines SKILL.md format and directory layout | Official Anthropic specification |
| skill-creator | 1.0 | Skill scaffolding, validation, and packaging tools | Official Anthropic tooling from anthropic-agent-skills repo |
| quick_validate.py | - | Validates frontmatter format and naming conventions | Bundled with skill-creator; checks name, description, hyphen-case naming |
| package_skill.py | - | Validates + packages skills into distributable zip | Bundled with skill-creator |

### Supporting
| Tool | Purpose | When to Use |
|------|---------|-------------|
| marketplace.json | Plugin manifest listing skills for Claude Code plugin system | Required for skills to be discovered when loaded as a plugin |
| skills.toml | Agent-deck skill attachment tracking | Used by conductor/agent-deck to track which pool skills are symlinked |

## Architecture Patterns

### Official Skill Directory Structure
```
skill-name/
├── SKILL.md              # Required: YAML frontmatter + markdown instructions
├── scripts/              # Optional: Executable code (bash/python)
├── references/           # Optional: Documentation loaded on demand
└── assets/               # Optional: Files used in output (templates, images)
```

**Note:** Some Anthropic examples use `reference/` (singular, e.g., mcp-builder) while others have no reference dir at all. The spec does not mandate singular vs plural. The agent-deck project currently uses `references/` (plural). Keep this consistent within the project.

### YAML Frontmatter Format (from official spec)

**Required fields:**
```yaml
---
name: skill-name          # hyphen-case, lowercase, must match directory name
description: Description   # What the skill does and when to use it
---
```

**Optional fields:**
```yaml
---
name: skill-name
description: Description
license: MIT              # Or reference to bundled LICENSE.txt
allowed-tools:            # Pre-approved tools (Claude Code only)
  - Bash(git *)
  - Read
metadata:                 # Custom key-value pairs
  compatibility: claude, opencode
  version: "1.0"
---
```

### Pattern 1: Script Path Resolution via $SKILL_DIR

**What:** Scripts live inside the skill's directory and must be resolved relative to the skill's actual location, not the project root.

**When to use:** Always, when SKILL.md references scripts.

**Current (correct) pattern in agent-deck skill:**
```markdown
## Script Path Resolution (IMPORTANT)

This skill includes helper scripts in its `scripts/` subdirectory. When Claude Code loads this skill, it shows a line like:

```
Base directory for this skill: /path/to/.../skills/agent-deck
```

**You MUST use that base directory path to resolve all script references.** Store it as `SKILL_DIR`.
```

**Session-share skill currently uses broken relative paths:**
```markdown
# Currently in session-share SKILL.md:
scripts/export.sh              # BROKEN: no base path, fails from plugin cache
scripts/import.sh              # BROKEN: same issue
```

**Fix needed:** session-share must adopt the same `$SKILL_DIR` pattern, or always be invoked via the agent-deck skill's cross-reference (`$SKILL_DIR/../session-share/scripts/export.sh`).

### Pattern 2: Plugin Manifest Registration

**What:** The `.claude-plugin/marketplace.json` file controls which skills are discoverable when the project is installed as a Claude Code plugin.

**Current state:**
```json
{
  "plugins": [
    {
      "name": "agent-deck",
      "skills": ["./skills/agent-deck"]
      // session-share is NOT listed!
    }
  ]
}
```

**Gap:** session-share skill is not registered. Users installing agent-deck as a plugin won't get session-share auto-loaded. This may be intentional (session-share is referenced from agent-deck SKILL.md as a sibling), but it means session-share isn't independently discoverable.

### Pattern 3: Pool Skill Symlink Loading

**What:** Pool skills at `~/.agent-deck/skills/pool/<name>/` are loaded on demand via `Read ~/.agent-deck/skills/pool/<name>/SKILL.md`. For conductor sessions, they can be symlinked into `.claude/skills/` via agent-deck's skills.toml.

**Current gsd-conductor setup:**
```
~/.agent-deck/conductor/agent-deck/.claude/skills/gsd-conductor -> ../../../../skills/pool/gsd-conductor
```

**This symlink is relative (4 levels up)**. It resolves correctly because the conductor working directory is always at `~/.agent-deck/conductor/agent-deck/`. This pattern works.

### Anti-Patterns to Avoid
- **Hardcoded absolute paths in SKILL.md:** Never embed `/Users/ashesh/...` paths. Use `$SKILL_DIR` or relative references.
- **Assuming project root == skill root:** Scripts may execute from plugin cache, not the project checkout.
- **Using `compatibility` as a top-level frontmatter field:** Not in the official spec; must be under `metadata:` if needed.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Skill validation | Manual frontmatter checking | `quick_validate.py` from skill-creator | Validates name format, description, frontmatter structure automatically |
| Skill scaffolding | Manual directory creation | `init_skill.py` from skill-creator | Generates correct structure with proper frontmatter template |
| Skill packaging | Manual zip creation | `package_skill.py` from skill-creator | Validates before packaging, ensures correct structure |

**Key insight:** The skill-creator tools are already available at `/Users/ashesh/.agent-deck/skills/pool/skill-creator/scripts/` and can validate all three skills automatically.

## Common Pitfalls

### Pitfall 1: `compatibility` is Not a Standard Frontmatter Field
**What goes wrong:** Using `compatibility: claude, opencode` as a top-level YAML field. The official spec only defines `name`, `description`, `license`, `allowed-tools`, and `metadata` as valid frontmatter properties.
**Why it happens:** The field was added before the spec was finalized or by convention.
**How to avoid:** Move `compatibility` under `metadata:` map, or remove it if not needed. Check with `quick_validate.py` which currently only validates `name` and `description`.
**Warning signs:** `quick_validate.py` won't catch this since it only checks required fields, but other validators or future spec updates might reject unknown top-level fields.
**Decision point:** The roadmap success criteria mentions "compatibility" explicitly ("proper SKILL.md frontmatter (name, description, compatibility)"). This suggests the user wants `compatibility` preserved. Move it to `metadata.compatibility` to satisfy both the spec and the user's intent.

### Pitfall 2: Session-share Script Paths Break in Plugin Cache
**What goes wrong:** The session-share SKILL.md uses bare relative paths like `scripts/export.sh` which resolve relative to the current working directory, not the skill's actual location.
**Why it happens:** Works fine when loaded from `<repo>/skills/session-share/` but breaks from `~/.claude/plugins/cache/<hash>/skills/session-share/`.
**How to avoid:** Add the same `$SKILL_DIR` path resolution instructions that agent-deck already uses, or ensure session-share is always invoked through agent-deck's cross-reference.
**Warning signs:** "Command not found" errors when running export.sh or import.sh from a plugin-loaded session.

### Pitfall 3: Session-share Not in marketplace.json
**What goes wrong:** Users who install agent-deck as a Claude Code plugin don't get session-share as a discoverable skill.
**Why it happens:** marketplace.json only lists `./skills/agent-deck`.
**How to avoid:** Add `./skills/session-share` to the `skills` array in marketplace.json, or decide this is intentional (session-share accessed only via agent-deck's cross-reference).
**Warning signs:** `Read ~/.claude/plugins/cache/.../skills/session-share/SKILL.md` works, but Claude doesn't auto-discover the skill.

### Pitfall 4: Directory Name Must Match Frontmatter `name`
**What goes wrong:** The spec requires `name` in frontmatter to match the directory name containing SKILL.md.
**Why it happens:** Renaming the directory without updating frontmatter, or vice versa.
**How to avoid:** Always verify: `agent-deck/SKILL.md` must have `name: agent-deck`, `session-share/SKILL.md` must have `name: session-share`, `gsd-conductor/SKILL.md` must have `name: gsd-conductor`.
**Warning signs:** `quick_validate.py` checks the name format but not the directory-name match (it only validates the path passed to it).
**Current state:** All three skills already match correctly.

### Pitfall 5: GSD Conductor Content Staleness
**What goes wrong:** The gsd-conductor skill at `~/.agent-deck/skills/pool/gsd-conductor/` may contain outdated information about GSD commands, agent types, or file structures.
**Why it happens:** GSD evolves frequently and the pool skill isn't auto-updated with the project.
**How to avoid:** Compare current gsd-conductor SKILL.md against the latest GSD version installed in the project, verify all documented commands and agents still exist.
**Warning signs:** Referenced commands that no longer exist, missing new features.

## Code Examples

### Current Agent-deck Frontmatter (already close to spec)
```yaml
---
name: agent-deck
description: Terminal session manager for AI coding agents. Use when user mentions "agent-deck", "session", "sub-agent", "MCP attach", "git worktree", or needs to (1) create/start/stop/restart/fork sessions, (2) attach/detach MCPs, (3) manage groups/profiles, (4) get session output, (5) configure agent-deck, (6) troubleshoot issues, (7) launch sub-agents, or (8) create/manage worktree sessions. Covers CLI commands, TUI shortcuts, config.toml options, and automation.
compatibility: claude, opencode   # <-- Non-standard, move to metadata
---
```

### Corrected Frontmatter (spec-compliant)
```yaml
---
name: agent-deck
description: Terminal session manager for AI coding agents. Use when user mentions "agent-deck", "session", "sub-agent", "MCP attach", "git worktree", or needs to (1) create/start/stop/restart/fork sessions, (2) attach/detach MCPs, (3) manage groups/profiles, (4) get session output, (5) configure agent-deck, (6) troubleshoot issues, (7) launch sub-agents, or (8) create/manage worktree sessions. Covers CLI commands, TUI shortcuts, config.toml options, and automation.
metadata:
  compatibility: "claude, opencode"
---
```

### Session-share Path Resolution Fix
```markdown
## Script Path Resolution (IMPORTANT)

This skill includes helper scripts in its `scripts/` subdirectory. When Claude Code loads this skill, it shows a line like:

```
Base directory for this skill: /path/to/.../skills/session-share
```

**You MUST use that base directory path to resolve all script references.** Store it as `SKILL_DIR`:

```bash
SKILL_DIR="/path/shown/in/base-directory-line"
$SKILL_DIR/scripts/export.sh
$SKILL_DIR/scripts/import.sh ~/Downloads/session-file.json
```
```

### Validation Command
```bash
# Validate all three skills
python3 /Users/ashesh/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py /Users/ashesh/claude-deck/skills/agent-deck
python3 /Users/ashesh/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py /Users/ashesh/claude-deck/skills/session-share
python3 /Users/ashesh/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py /Users/ashesh/.agent-deck/skills/pool/gsd-conductor
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Ad-hoc skill format | Official Agent Skills Spec 1.0 | 2025-10-16 | Standardized frontmatter, naming conventions |
| `.skill` flat files | Directory-based skills with SKILL.md | 2025-10-16 | Enables scripts/, references/, assets/ bundling |
| Custom compatibility field | metadata map for custom fields | 2025-10-16 | Non-standard top-level fields should use metadata |

**Note:** Many older skills in the pool (e.g., `agent-deck-cli.skill`, `codex-support.skill`, `docs.skill`) still use the old flat `.skill` file format. These are NOT in scope for Phase 1.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go test + bash validation scripts |
| Config file | Makefile (test target) |
| Quick run command | `python3 ~/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py <skill-path>` |
| Full suite command | `make test` (Go tests, not directly applicable to skills) |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SKILL-01 | agent-deck skill has correct format | smoke | `python3 ~/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py skills/agent-deck` | N/A (validation script) |
| SKILL-02 | session-share skill has correct format | smoke | `python3 ~/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py skills/session-share` | N/A (validation script) |
| SKILL-03 | gsd-conductor is in pool with current content | manual-only | Visual diff of SKILL.md content against GSD source docs | N/A |
| SKILL-04 | All frontmatter correct | smoke | Run quick_validate.py on all three skills | N/A |
| SKILL-05 | Path resolution works from both locations | smoke | Verify `$SKILL_DIR` pattern present in all SKILL.md files; test script execution from plugin cache path | N/A |

**Note:** Skills reorganization is primarily a documentation/structure task. The validation is done via the skill-creator's `quick_validate.py` script and manual path testing, not via Go unit tests. Phase 2 (TEST-02) covers skills triggering integration tests.

### Sampling Rate
- **Per task commit:** `python3 ~/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py <skill-path>`
- **Per wave merge:** Validate all three skills + test path resolution manually
- **Phase gate:** All three skills pass validation; script paths resolve from both locations

### Wave 0 Gaps
None. Existing validation scripts (`quick_validate.py`) and the `make test` infrastructure cover all phase requirements. No new test files needed for this phase.

## Open Questions

1. **Should session-share be added to marketplace.json as an independent skill?**
   - What we know: Currently session-share is only referenced from agent-deck SKILL.md as a sibling skill (`$SKILL_DIR/../session-share/`). It is not registered in `marketplace.json`.
   - What's unclear: Whether users should be able to discover session-share independently or only through agent-deck.
   - Recommendation: Add it to marketplace.json for discoverability, since it has its own SKILL.md and is a standalone feature.

2. **Should `compatibility` be preserved or removed entirely?**
   - What we know: The roadmap success criteria explicitly mentions "compatibility" as a desired field. The official spec doesn't support it as a top-level field.
   - What's unclear: Whether the user wants strict spec compliance or just functional frontmatter.
   - Recommendation: Move to `metadata.compatibility` to satisfy both goals.

3. **Is the gsd-conductor SKILL.md content current with latest GSD?**
   - What we know: The file was last modified 2026-02-27. GSD is installed locally in this project and may have evolved since.
   - What's unclear: Whether any GSD commands, agent types, or file structures have changed since the SKILL.md was written.
   - Recommendation: During execution, diff gsd-conductor content against the actual GSD installation in `.claude/get-shit-done/` to verify currency.

## Sources

### Primary (HIGH confidence)
- `/Users/ashesh/.agent-deck/skills/pool/anthropic-skills/agent_skills_spec.md` - Official Agent Skills Spec 1.0 (2025-10-16)
- `/Users/ashesh/.agent-deck/skills/pool/skill-creator/SKILL.md` - Official skill-creator guide with format documentation
- `/Users/ashesh/.agent-deck/skills/pool/skill-creator/scripts/quick_validate.py` - Validation script source code
- `/Users/ashesh/claude-deck/.claude-plugin/marketplace.json` - Current plugin manifest

### Secondary (MEDIUM confidence)
- `/Users/ashesh/.agent-deck/skills/pool/anthropic-skills/*/SKILL.md` - Example skills from Anthropic showing conventions (reference vs references, license field usage)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Official Anthropic spec and tooling, directly inspected
- Architecture: HIGH - All skill files, plugin manifest, and cache paths directly inspected
- Pitfalls: HIGH - Derived from direct comparison of current files against spec requirements

**Research date:** 2026-03-06
**Valid until:** 2026-04-06 (stable domain, spec version 1.0)
