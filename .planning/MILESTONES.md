# Milestones

## v1.5.4 per-group Claude config + conductor schema (Shipped: 2026-04-15)

**Phases completed:** 4 phases, 7 plans, 26 tasks

**Key accomplishments:**

- Group-level `[groups."<name>".claude].env_file` is now locked under automated regression test (`TestPerGroupConfig_EnvFileSourcedInSpawn`) for BOTH the normal-claude path (instance.go:478) and the custom-command/conductor path (instance.go:599), with defense-in-depth hardening at the inner custom-command return so env_file sourcing survives any future refactor that bypasses the outer `buildClaudeCommand` wrapper.
- 1. [Rule 1 - Bug] Plan test used first arg as session ID but `NewInstanceWithGroupAndTool` assigns it as Title
- CFG-05 visual harness script `scripts/verify-per-group-claude-config.sh` — proves per-group `CLAUDE_CONFIG_DIR` injection into tmux spawn env via `/proc/<pid>/environ` assertion; exits 0 iff both normal and custom-command sessions resolve correctly.
- Three doc surfaces (README subsection, repo-root CLAUDE.md one-liner, CHANGELOG bullet) land CFG-06 with `@alec-pinson` attribution in every commit body; phase-wide hard-rule audit confirms 0 Claude attribution, 0 unsigned Ashesh commits, 0 forbidden git ops across all of `main..HEAD`.
- 1. [Rule 3 — Blocking] Per-conductor struct renamed from `ConductorSettings` → `ConductorOverrides`
- README.md extended with `[conductors.<name>.claude]` subsection, canonical SKILL.md updated with the same paragraph, repo-visible `SKILL_MD_DIFF.md` audit artifact shipped, and repo-root CLAUDE.md gains both a NEW `--no-verify` ban bullet and a scope-clarification sub-section with metadata-paths list + positive/negative examples — all four Phase 4 hard-rule audit gates pass with zero violations across seven Phase 4 commits.

---
