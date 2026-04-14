# Agent-Deck — Session Persistence Hotfix (v1.5.2)

## What This Is

Agent-deck is a terminal session manager for AI coding agents (Claude, Codex, Gemini) that creates and manages tmux sessions hosting long-running agent conversations. This milestone is **v1.5.2 "session-persistence"** — a hotfix off v1.5.1 targeting two recurring production failures where a single SSH logout on a Linux+systemd host destroys every managed tmux session and every live Claude conversation with it.

The audience for this hotfix is any developer running agent-deck on a Linux host with multiple SSH sessions, where logout currently cascades into total session loss.

## Core Value

After installing v1.5.2, an SSH logout on a Linux+systemd host must not kill any agent-deck-managed tmux server, and restarting any dead session must resume the prior Claude conversation using the persisted `ClaudeSessionID`. Both behaviors must be permanently test-gated so this class of bug cannot regress.

## Requirements

### Validated

<!-- Capabilities already present in the v1.5.1 codebase that this milestone depends on and must not break. -->

- ✓ `LaunchInUserScope` wrap exists at `internal/tmux/tmux.go:724` — wraps tmux spawn in `systemd-run --user --scope --quiet --collect --unit agentdeck-tmux-<name>`. Currently defaults to `false` (`internal/session/userconfig_test.go:1102`).
- ✓ `Instance.ClaudeSessionID` is persisted through clean stop (`cmd/session_cmd.go:286`) and survives in JSON storage across restarts.
- ✓ Resume logic exists: `Restart()` at `internal/session/instance.go:3763` → `buildClaudeResumeCommand()` at `:4114`.
- ✓ Hook sidecar at `~/.agent-deck/hooks/<instance>.sid` is populated for Claude Code integration, but instance storage is the authoritative source of `ClaudeSessionID` per `docs/session-id-lifecycle.md`.
- ✓ Linger is enabled on the conductor host via `loginctl enable-linger` (not sufficient alone — see incident below).

### Active

<!-- This milestone's hypotheses. Move to Validated when shipped and proven. -->

- [ ] **REQ-1** Default-on cgroup isolation: on Linux+systemd hosts, `launch_in_user_scope` defaults to `true` without user configuration; on non-systemd hosts it silently defaults to `false`; explicit `false` in `config.toml` is always honored.
- [ ] **REQ-2** Resume-on-start and resume-on-error-recovery: any code path that starts a Claude session for an Instance with non-empty `ClaudeSessionID` launches `claude --resume <id>` if `sessionHasConversationData()` returns true, else `claude --session-id <id>`. Applies to `session start`, `session restart`, automatic error-recovery, and conductor-driven restart after tmux teardown.
- [ ] **REQ-3** Regression test suite `internal/session/session_persistence_test.go` with the eight named `TestPersistence_*` tests from the spec, CI-gated on every PR touching the mandated paths, with no host mocking of tmux/systemd.
- [ ] **REQ-4** `CLAUDE.md` at repo root contains a "Session persistence: mandatory test coverage" section enumerating the eight tests, the paths under mandate, forbidden changes without an RFC, and the 2026-04-14 incident as the reason. Top-level README/CHANGELOG mentions v1.5.2 in one line.
- [ ] **REQ-5** Visual end-to-end harness at `scripts/verify-session-persistence.sh` — a runnable, human-watchable script that prints PIDs, cgroup paths, and the exact resume command line; exits non-zero on any scenario failure; is invoked in CI.
- [ ] **REQ-6** Observability: one structured startup log line describing the cgroup isolation decision (`enabled`/`disabled`/`unavailable`), and one structured log line per `session start`/`restart` stating whether resume was taken.

### Out of Scope

- Migrating the 33 error / 39 stopped sessions currently on the conductor host — separate manual recovery task.
- Touching `KillUserProcesses` in `/etc/systemd/logind.conf` — we do not modify host-level systemd config.
- A setup wizard prompt for the new default — the change is silent.
- Changing MCP attach/detach flow.
- Changing the session-sharing export/import mechanism.
- Changing `fork` semantics.
- UI indicators for "resumable" status (a `↻` glyph is P2 and deferred unless bandwidth allows).
- A config auto-upgrade path that rewrites `~/.agent-deck/config.toml` — too invasive; defaults are runtime-only.
- Picking up the legacy v15 roadmap stalled at phase 11 in `.planning.legacy-v15/` — this hotfix is intentionally standalone.

## Context

**The 2026-04-14 incident (today).** At 09:08:01 local time, `systemd-logind` removed three SSH login sessions on the conductor host. At the exact same second, every `tmux-spawn-*.scope` belonging to agent-deck-managed sessions stopped. 33 sessions landed in `error` status, 39 in `stopped`, all live Claude conversations were lost. Uptime was 25 days — no reboot, no OOM, no user action on agent-deck. **Third recurrence on the same host.**

**Root cause REQ-1 — cgroup inheritance.** agent-deck spawns tmux as a child of the invoking shell. The tmux server lands in that shell's login-session scope. When logind tears down the SSH session, it tears down the scope tree and tmux with it. Linger only protects `user@UID.service` and its direct children — not tmux servers parented under a login-session scope. The escape mechanism (`LaunchInUserScope` → `systemd-run --user --scope`) already exists in the codebase but defaults to `false` and the incident host's config.toml does not override it. The feature exists and is dormant.

**Root cause REQ-2 — restart does not resume on error path.** Clean stop → `Restart()` flow does honor `ClaudeSessionID`. But on the 2026-04-14 incident, tmux panes were SIGKILLed by logind, so sessions went `error` (not `stopped`). Error-recovery goes through `session start`, which may or may not honor `ClaudeSessionID` — needs audit and regression coverage.

**Runtime environment.** Go 1.22+ CLI + Bubble Tea TUI. Relevant packages: `internal/tmux/` (tmux spawn, `tmux.go:814-837` holds the systemd-run wrap), `internal/session/` (Instance lifecycle, user config, JSON storage under `~/.agent-deck/<profile>/`), `cmd/` (session subcommands). Hooks live at `~/.agent-deck/hooks/<instance>.json`.

**Versioning.** Currently on branch `fix/session-persistence` off local `main` (v1.5.1, >10 commits ahead of `origin/main`). No breaking changes, no new user-facing features. No push, no tag, no PR — user merges manually.

## Constraints

- **Tech stack**: Go 1.22+, Bubble Tea TUI, tmux, systemd on Linux — existing stack, no new dependencies.
- **Portability**: macOS/BSD hosts must not regress — detection of `systemd-run` must silently fall back, not error.
- **Invariants**: `docs/session-id-lifecycle.md` contract (no disk-scan authoritative binding) must not be violated.
- **Test realism**: No mocking of tmux or systemd in persistence tests — use real binaries; skip cleanly on hosts that lack them.
- **Process rules**: TDD — regression test lands BEFORE fix. No `git push`, no tags, no `gh pr create`. No `rm` — use `trash`. No Claude attribution in commits.
- **Host sensitivity**: On macOS CI, `systemd-run` is absent. Tests must skip with clear reasons, not error or pass vacuously.
- **Scope discipline**: If a plan wants to refactor code outside `internal/tmux/`, `internal/session/instance.go`, `internal/session/userconfig.go`, or `cmd/session_cmd.go` (+ the new test file and script), stop and escalate.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Default `launch_in_user_scope=true` on Linux+systemd without a wizard prompt | Silent runtime default keeps user config.toml untouched; explicit opt-out still honored. Spec REQ-1 non-goal explicitly excludes a prompt. | — Pending |
| No config auto-upgrade rewriting `~/.agent-deck/config.toml` | Too invasive; risks breaking user-customized configs. Runtime-only default change is sufficient. | — Pending |
| Gate every PR on the eight `TestPersistence_*` tests + `scripts/verify-session-persistence.sh` via CLAUDE.md mandate | Third recurrence of the same incident class — per-PR hard gate is the only way to prevent a fourth. | — Pending |
| Do not migrate the 33 error / 39 stopped sessions as part of this milestone | Recovery is a manual operator task; bundling it would bloat scope and delay the fix. | — Pending |
| Do not pick up legacy v15 roadmap (stalled at phase 11) | Milestone is scoped to the hotfix; legacy roadmap is archived in `.planning.legacy-v15/`. | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-14 after v1.5.2 hotfix milestone initialization*
