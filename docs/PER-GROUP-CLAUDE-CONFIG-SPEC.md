# Per-group Claude config — v1.5.4

## Project overview

Agent-deck currently supports a single `CLAUDE_CONFIG_DIR` per agent-deck **profile** (`~/.claude` for `personal`, `~/.claude-work` for `work`). Sessions inside a profile all inherit the same config dir. This is not enough: a single profile often hosts groups that should use different Claude authentications — e.g. a `conductor` group that should use the work Claude account, a `side-projects` group that should use a separate account, all inside the `personal` profile.

## Problem statement

User has two Claude auth contexts (`~/.claude` personal, `~/.claude-work` work) and a dual-profile alias setup (`cdp` / `cdw`). Agent-deck exposes this at the profile level. **But agent-deck's conductor (in group `conductor`), the user's `agent-deck/*` work sessions, and their personal `trip`/`mails` sessions all run under a single `personal` profile** — they can't be split across Claude config dirs without creating more profiles, which fragments session visibility.

External contributor PR #578 (`feat/per-group-config` by @alec-pinson, 2026-04-12) solves this at the config schema + lookup level:

```toml
[groups."conductor".claude]
config_dir = "~/.claude-work"
env_file = "~/git/work/.envrc"
```

The PR's code-level integration points:
- `GetClaudeConfigDirForGroup(groupPath)` with priority env > group > profile > global
- `IsClaudeConfigDirExplicitForGroup(groupPath)` matching
- `GetGroupClaudeConfigDir` / `GetGroupClaudeEnvFile` helpers on UserConfig
- `instance.go` 3 call-sites rewired: `buildClaudeCommandWithMessage`, `buildBashExportPrefix`, `buildClaudeResumeCommand`

PR #578 is a clean base. This milestone (v1.5.4) **accepts** PR #578 and closes the gaps that block adoption for the user's actual use cases.

## Goals

1. Adopt PR #578's config schema and lookup priority, exactly as designed.
2. Prove per-group config dir works end-to-end for **custom-command sessions** (conductors, `add --command <script>`). This is the highest-risk path because existing code skips `CLAUDE_CONFIG_DIR` prefix for custom commands in `buildClaudeCommandWithMessage` (comment: "alias handles config dir").
3. Prove `env_file` is sourced before `claude` exec, not just exported into the tmux env.
4. Ship named regression tests so this never regresses on future refactors.
5. Ship a visual harness that prints the resolved `CLAUDE_CONFIG_DIR` per session and per group — human-watchable.

## Version

**v1.5.4** — small feature release on top of v1.5.3. Accepts external PR #578's implementation as base. No breaking changes. Added tests are additive.

## Open GitHub items relevant

- **PR #578** (`feat/per-group-config`) — @alec-pinson — OPEN, MERGEABLE, mergeStateStatus=UNSTABLE (no CI checks configured on branch). This milestone's branch `fix/per-group-claude-config-v154` is forked from PR #578's HEAD `fa9971e`, so a future merge strategy is either (a) merge PR #578 then this milestone's additions as a follow-up PR, or (b) land everything as one PR that supersedes #578, with attribution to @alec-pinson in the commit message. User decides at milestone end.

## Requirements

### REQ-1: PR #578 config schema and lookup priority (P0)

**Rule:** `[groups."<name>".claude] { config_dir, env_file }` is a valid TOML section. `GetClaudeConfigDirForGroup(groupPath)` resolves with priority: env var `CLAUDE_CONFIG_DIR` > group override > profile override > global `[claude] config_dir` > default `~/.claude`. Empty or missing group name falls through to profile.

**Acceptance:**
- Unit tests from PR #578 (`TestGetClaudeConfigDirForGroup_GroupWins`, `TestIsClaudeConfigDirExplicitForGroup`) remain GREEN — no modification to their assertions.
- `config_dir` accepts `~` expansion, absolute paths, and environment variable expansion (`$HOME`).
- Adding or removing a group's `config_dir` at runtime is picked up after `ClearUserConfigCache()` (agent-deck's existing cache invalidation path on config reload).

### REQ-2: Custom-command (conductor) sessions honor per-group config_dir (P0)

**Rule:** When an `Instance.Command` is non-empty (e.g. `/home/user/.agent-deck/conductor/agent-deck/start-conductor.sh`), agent-deck MUST still inject `CLAUDE_CONFIG_DIR=<resolved>` into the spawn environment for that session if the group or profile has an override. This closes the gap in PR #578's `buildClaudeCommandWithMessage` which skips the prefix for custom commands — the gap is acceptable for shell aliases that set the env themselves, but NOT acceptable for conductor-style wrapper scripts that have no such alias.

**Resolution approach:** `buildBashExportPrefix` already exports `CLAUDE_CONFIG_DIR` unconditionally (even for custom commands). Verify by test that this path is actually taken for custom-command sessions, OR move the export into the tmux pane env injection if not.

**Acceptance:**
- A session created with `agent-deck -p personal add ./some-wrapper.sh -t "test-conductor" -g "conductor"` where `~/.agent-deck/config.toml` has `[groups."conductor".claude] config_dir = "~/.claude-work"` launches with `CLAUDE_CONFIG_DIR=~/.claude-work` visible inside the tmux pane's environment (verified by `agent-deck session send <id> "echo CLAUDE_CONFIG_DIR=\$CLAUDE_CONFIG_DIR"`).
- After restart, the env var persists — the wrapper script sees it.
- Conductor restart (via `start-conductor.sh`) preserves the env var — the `exec claude ...` inside the wrapper uses `~/.claude-work` for its Claude auth.
- A session in a group with NO override falls through to the profile's config dir.

### REQ-3: env_file is sourced before claude exec (P0)

**Rule:** `[groups."<name>".claude] env_file = "/path/to/.envrc"` causes the tmux pane to `source "/path/to/.envrc"` before exec'ing claude (or the custom command). This enables per-group secrets, PATH adjustments, and tool versions (e.g. `direnv`-style workflows). Path expansion mirrors `config_dir` (`~`, env vars).

**Acceptance:**
- Write a throwaway `/tmp/envrc-test` that `export TEST_ENVFILE_VAR=hello`. Configure a group to use it. Launch a session. `echo $TEST_ENVFILE_VAR` inside the session returns `hello`.
- If `env_file` does not exist, log a warning and continue; do not block session start.
- `env_file` supports both shell-style `.envrc` (sourced) and flat KEY=VALUE `.env` format (also sourced — bash can handle both).

**Non-goals:**
- Not implementing a direnv integration layer. Just a source line.

### REQ-4: Named regression tests (P0)

A new test file `internal/session/pergroupconfig_test.go` MUST contain:

1. `TestPerGroupConfig_CustomCommandGetsGroupConfigDir` — instance with non-empty `Command`, group `foo` has config_dir override. The built env/exports include `CLAUDE_CONFIG_DIR=<foo's dir>`.
2. `TestPerGroupConfig_GroupOverrideBeatsProfile` — group and profile both set, group wins.
3. `TestPerGroupConfig_UnknownGroupFallsThroughToProfile` — instance in group `nonexistent`, falls through to profile override.
4. `TestPerGroupConfig_EnvFileSourcedInSpawn` — env_file set, its exported vars are visible in the spawn env (via `buildBashExportPrefix` or equivalent).
5. `TestPerGroupConfig_ConductorRestartPreservesConfigDir` — end-to-end: create custom-command session, stop, restart, assert `CLAUDE_CONFIG_DIR` in new spawn matches group's override. Connects REQ-2 to REQ-7 from v1.5.2 (custom-command resume path).
6. `TestPerGroupConfig_CacheInvalidation` — add/remove group override, `ClearUserConfigCache()`, resolver returns the new value.

Each test independently runnable (`go test -run TestPerGroupConfig_<name> ./internal/session/...`), self-cleaning, no network.

### REQ-5: Visual harness (P1)

`scripts/verify-per-group-claude-config.sh` — a human-watchable script that:

1. Creates two throwaway groups (`verify-group-a`, `verify-group-b`) with different `config_dir` values in a temp config.
2. Launches one session per group (one normal, one custom-command).
3. Sends `echo CLAUDE_CONFIG_DIR=$CLAUDE_CONFIG_DIR` to each. Captures output.
4. Prints a pass/fail table. Exit 0 iff both sessions show the expected per-group value.
5. Cleans up — stops sessions, restores config.

### REQ-6: Documentation (P0)

- `README.md` — add one subsection "Per-group Claude config" under Configuration, with the `[groups."conductor".claude]` example from PR #578.
- `CLAUDE.md` (repo root) — add a one-line entry under the session-persistence mandate block: "Per-group config dir applies to custom-command sessions too; `TestPerGroupConfig_*` suite enforces this."
- `CHANGELOG.md` — `[Unreleased] > Added` bullet: "Per-group Claude config overrides (`[groups."<name>".claude]`)."
- Attribution in at least one commit message: "Base implementation by @alec-pinson in PR #578."

### REQ-7: Observability (P2)

- On session spawn, one log line: `claude config resolution: session=<id> group=<g> resolved=<path> source=<env|group|profile|global|default>`.
- Helps future debugging (which level actually set the dir for a given session).

## Out of scope

- Not touching Claude profile-level config (`[profiles.<x>.claude]`) semantics — keep as today.
- Not building a TUI editor for groups — config.toml is hand-edited.
- Not adding per-group `mcp_servers` overrides (future work; `.mcp.json` attach flow already covers this use case).
- Not implementing full direnv `.envrc` with hashing / auto-reload.

## Architecture notes

Based on PR #578's diff (already in this branch):

- `internal/session/userconfig.go` — adds `GetGroupClaudeConfigDir`, `GetGroupClaudeEnvFile`, `GroupClaudeConfig` struct.
- `internal/session/claude.go` — adds `GetClaudeConfigDirForGroup`, `IsClaudeConfigDirExplicitForGroup`, legacy `GetClaudeConfigDir` now delegates with empty group.
- `internal/session/instance.go` — three call-sites rewired to pass `i.GroupPath`.
- `internal/session/env.go` — 4 added lines for env injection.
- `internal/ui/home.go` — 2 lines touched (group passed in UI-initiated spawns).

Gaps this milestone closes:
- `env_file` source semantics: PR #578 adds the schema but the source-before-exec wiring needs verification. Check `env.go` and `buildBashExportPrefix`.
- Custom-command injection path: PR #578 intentionally skips `CLAUDE_CONFIG_DIR` prefix for custom commands in `buildClaudeCommandWithMessage`. `buildBashExportPrefix` is the fallback but needs test coverage.
- Conductor end-to-end: this milestone's visual harness proves it.

## Known pain points

- `fa9971e` (PR #578 HEAD) is several commits behind current `main` on this repo; the v1.5.4 branch may need a rebase before merge. Rebase is a merge-time concern, not this milestone's scope.
- External contributor PR — keep their commit history intact in the final merge, attribute properly.
- The user is on a `personal` profile but wants `conductor` group to use `~/.claude-work`. Unusual direction: groups overriding to a DIFFERENT profile's config dir. Test explicitly.

## Hard rules for all phases

- No `git push`, `git tag`, `gh release`, `gh pr create`, `gh pr merge`.
- No `rm` — use `trash` if cleanup needed.
- No Claude attribution in own commits. Sign as "Committed by Ashesh Goplani" when signing.
- TDD ordering: test before fix.
- Do NOT revert or refactor PR #578's existing code unless a test requires it. Additive only.
- Scope: `internal/session/claude.go`, `internal/session/userconfig.go`, `internal/session/instance.go`, `internal/session/env.go`, new test file `pergroupconfig_test.go`, `scripts/verify-per-group-claude-config.sh`, README.md, CLAUDE.md, CHANGELOG.md, docs/PER-GROUP-CLAUDE-CONFIG-SPEC.md. Anything else = escalate.

## Success criteria for the milestone

1. PR #578 unit tests remain GREEN.
2. `go test ./internal/session/... -run TestPerGroupConfig_ -race -count=1` — all 6 GREEN.
3. `bash scripts/verify-per-group-claude-config.sh` exits 0 on conductor host with visual table.
4. Manual proof on conductor host: add `[groups."conductor".claude] config_dir = "~/.claude-work"` to `~/.agent-deck/config.toml`, restart conductor, `ps -p <pane_pid>` env shows `CLAUDE_CONFIG_DIR=/home/user/.claude-work`, conductor now uses the work Claude account.
5. `git log main..HEAD --oneline` ends with README+CHANGELOG+CLAUDE.md commits and one attribution commit referencing @alec-pinson.
6. No push / tag / PR / merge performed.
