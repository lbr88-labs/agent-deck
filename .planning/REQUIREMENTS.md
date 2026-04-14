# Requirements: Agent-Deck v1.5.2 Session-Persistence Hotfix

**Defined:** 2026-04-14
**Core Value:** After v1.5.2, SSH logout on a Linux+systemd host must not kill any agent-deck tmux server, and restarting any dead session must resume the prior Claude conversation — both behaviors permanently test-gated.

## v1 Requirements

All requirements below are in scope for the v1.5.2 hotfix milestone. Each is atomic, testable, and maps to spec REQ-1..REQ-6 in `docs/SESSION-PERSISTENCE-SPEC.md`.

### Runtime Persistence (PERSIST) — spec REQ-1, REQ-2

- [ ] **PERSIST-01**: On Linux hosts where `systemd-run --user --version` succeeds, `launch_in_user_scope` defaults to `true` without user configuration.
- [ ] **PERSIST-02**: On hosts without `systemd-run` (macOS, BSD, Linux lacking user manager), `launch_in_user_scope` silently defaults to `false` and emits no error.
- [ ] **PERSIST-03**: An explicit `launch_in_user_scope = false` in `~/.agent-deck/config.toml` is always honored, overriding the Linux+systemd default.
- [ ] **PERSIST-04**: On a Linux+systemd host, a fresh install spawns tmux under `user@UID.service`, verifiable via `systemctl status user@UID.service` showing an `agentdeck-tmux-*.scope` child.
- [ ] **PERSIST-05**: An SSH logout that spawned the agent-deck command does not kill the tmux server; `tmux list-sessions` from a new SSH login returns the same server.
- [ ] **PERSIST-06**: If `systemd-run` exists but its invocation fails (e.g., no user manager), agent-deck falls back to direct tmux spawn, logs a warning, and never blocks session creation.
- [ ] **PERSIST-07**: Any code path that starts a Claude session for an Instance with non-empty `ClaudeSessionID` launches `claude --resume <id>` when `sessionHasConversationData()` returns true, else `claude --session-id <id>`. Applies to `session start`, `session restart`, automatic error-recovery, and conductor-driven restart after tmux teardown.
- [ ] **PERSIST-08**: `Instance.ClaudeSessionID` is preserved through any transition to `stopped` or `error` state, cleared only on explicit delete or user-initiated `fork`.
- [ ] **PERSIST-09**: Resume works even when the hook sidecar at `~/.agent-deck/hooks/<instance>.sid` has been deleted — `ClaudeSessionID` is read from instance JSON storage as the authoritative source.
- [ ] **PERSIST-10**: The `docs/session-id-lifecycle.md` invariants (no disk-scan authoritative binding) remain honored.

### Regression Tests (TEST) — spec REQ-3

All eight tests live in `internal/session/session_persistence_test.go`. Each is independently runnable via `go test -run TestPersistence_<name> ./internal/session/...`, requires no external network, cleans up all tmux servers and transcripts it creates, and skips cleanly (not vacuously) on hosts lacking systemd-run.

- [ ] **TEST-01**: `TestPersistence_TmuxSurvivesLoginSessionRemoval` — with `LaunchInUserScope=true`, simulate a login-session teardown and verify agent-deck tmux server PID is still alive. Skips on non-systemd hosts with a clear reason.
- [ ] **TEST-02**: `TestPersistence_TmuxDiesWithoutUserScope` — inverse: with `LaunchInUserScope=false` and simulated teardown, the tmux server does die. Pins the failure mode so we don't "fix" by changing scope and leave opt-outs vulnerable.
- [ ] **TEST-03**: `TestPersistence_LinuxDefaultIsUserScope` — on a Linux host with `systemd-run` available, `TmuxSettings{}.GetLaunchInUserScope()` returns `true` with no config file.
- [ ] **TEST-04**: `TestPersistence_MacOSDefaultIsDirect` — on a host without `systemd-run`, `TmuxSettings{}.GetLaunchInUserScope()` returns `false` and no error is logged.
- [ ] **TEST-05**: `TestPersistence_RestartResumesConversation` — start a session, write a synthetic JSONL transcript to `~/.claude/projects/<hash>/`, stop, restart, and verify the spawned command line contains `--resume <claudeSessionID>` and the JSONL path exists.
- [ ] **TEST-06**: `TestPersistence_StartAfterSIGKILLResumesConversation` — same as TEST-05 but the session is marked `error` after a simulated SIGKILL of its tmux server, and recovery is via `session start` (not `session restart`).
- [ ] **TEST-07**: `TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion` — delete `~/.agent-deck/hooks/<instance>.sid`, start the session, assert `ClaudeSessionID` is still read from instance JSON storage and applied.
- [ ] **TEST-08**: `TestPersistence_FreshSessionUsesSessionIDNotResume` — first start (no prior conversation) uses `--session-id <uuid>` only, not `--resume`. Guards against accidentally passing `--resume` with a non-existent ID.

### Documentation (DOC) — spec REQ-4

- [ ] **DOC-01**: Repo `CLAUDE.md` contains a section titled "Session persistence: mandatory test coverage" listing the eight TEST-01..TEST-08 names verbatim.
- [ ] **DOC-02**: That CLAUDE.md section declares that any PR modifying `internal/tmux/**`, `internal/session/instance.go`, `internal/session/userconfig.go`, or the session start/restart command handlers MUST run the full `TestPersistence_*` suite and include the output (or a CI run link) in the PR description.
- [ ] **DOC-03**: That CLAUDE.md section names the 2026-04-14 incident as the reason.
- [ ] **DOC-04**: That CLAUDE.md section states that the `launch_in_user_scope` default may not be flipped back to `false` on Linux without an RFC.
- [ ] **DOC-05**: The top-level `README.md` or `CHANGELOG.md` mentions the v1.5.2 hotfix in one line.

### Verification Script (SCRIPT) — spec REQ-5

- [ ] **SCRIPT-01**: `scripts/verify-session-persistence.sh` exists, is executable, and prints a numbered checklist of scenarios it will test.
- [ ] **SCRIPT-02**: The script launches a real agent-deck session (real Claude, or a stub claude binary on CI) in a real tmux server — no mocking.
- [ ] **SCRIPT-03**: The script prints the tmux server PID and its cgroup path from `/proc/<pid>/cgroup`, so a human can confirm it is under `user@UID.service` and not a login-session scope.
- [ ] **SCRIPT-04**: On Linux+systemd, the script forces a simulated SSH-session teardown (throwaway `systemd-run --user --scope` unit terminated) and verifies the agent-deck tmux PID is still alive. On macOS/non-systemd, it skips with a clear "skipped: no systemd-run" message.
- [ ] **SCRIPT-05**: The script stops the session, restarts it, prints the exact claude command line spawned, and highlights whether it contains `--resume` or `--session-id`.
- [ ] **SCRIPT-06**: Each scenario ends with a green `[PASS]` or red `[FAIL]` banner; the script exits non-zero on any failure.
- [ ] **SCRIPT-07**: The script is invoked in CI and referenced from the CLAUDE.md DOC section. CI failure on this script blocks the PR.

### Observability (OBS) — spec REQ-6

- [ ] **OBS-01**: On startup, agent-deck emits one structured log line describing the cgroup isolation decision: `tmux cgroup isolation: enabled (systemd-run detected)` OR `tmux cgroup isolation: disabled (systemd-run not available)` OR `tmux cgroup isolation: disabled (config override)`.
- [ ] **OBS-02**: On every `session start` / `restart`, agent-deck emits one structured log line stating whether the resume path was taken: `resume: id=<x> reason=conversation_data_present` or `resume: none reason=fresh_session`.
- [ ] **OBS-03**: `grep 'tmux cgroup isolation' ~/.agent-deck/logs/*.log` and `grep 'resume:' ~/.agent-deck/logs/*.log` each return rows after normal operation. (Surface in logs only, not TUI.)

## v2 Requirements

<!-- Deferred. Not in this milestone. Re-evaluate after v1.5.2 ships. -->

### UI Affordances

- **UI-01**: Show a `↻` glyph in the session list indicating sessions with resumable conversation data. (P2 per spec; only if phase bandwidth allows — otherwise v2.)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Migrating the 33 error / 39 stopped sessions on the conductor host | Recovery of existing dead sessions is a separate manual operator task, not a code change. |
| Modifying `KillUserProcesses` in `/etc/systemd/logind.conf` | Host-level systemd config is outside agent-deck's scope; a runtime cgroup fix is sufficient. |
| Setup wizard prompt for new `launch_in_user_scope` default | Silent default change is the intended UX; a prompt would add friction for zero benefit. |
| Config auto-upgrade that rewrites `~/.agent-deck/config.toml` | Too invasive; risks breaking user customizations. Runtime-only default is sufficient. |
| Changes to MCP attach/detach flow | Unrelated to the incident class; would expand scope. |
| Changes to session-sharing export/import | Unrelated to the incident class; would expand scope. |
| Changes to `fork` semantics | Unrelated to the incident class; `fork` intentionally clears `ClaudeSessionID`. |
| Resuming legacy v15 roadmap (stalled at phase 11 release-plan) | Hotfix is scoped small and standalone per spec; legacy roadmap archived in `.planning.legacy-v15/`. |

## Traceability

Populated during roadmap creation. Every v1 requirement must map to exactly one phase.

| Requirement | Phase | Status |
|-------------|-------|--------|
| PERSIST-01 | TBD | Pending |
| PERSIST-02 | TBD | Pending |
| PERSIST-03 | TBD | Pending |
| PERSIST-04 | TBD | Pending |
| PERSIST-05 | TBD | Pending |
| PERSIST-06 | TBD | Pending |
| PERSIST-07 | TBD | Pending |
| PERSIST-08 | TBD | Pending |
| PERSIST-09 | TBD | Pending |
| PERSIST-10 | TBD | Pending |
| TEST-01 | TBD | Pending |
| TEST-02 | TBD | Pending |
| TEST-03 | TBD | Pending |
| TEST-04 | TBD | Pending |
| TEST-05 | TBD | Pending |
| TEST-06 | TBD | Pending |
| TEST-07 | TBD | Pending |
| TEST-08 | TBD | Pending |
| DOC-01 | TBD | Pending |
| DOC-02 | TBD | Pending |
| DOC-03 | TBD | Pending |
| DOC-04 | TBD | Pending |
| DOC-05 | TBD | Pending |
| SCRIPT-01 | TBD | Pending |
| SCRIPT-02 | TBD | Pending |
| SCRIPT-03 | TBD | Pending |
| SCRIPT-04 | TBD | Pending |
| SCRIPT-05 | TBD | Pending |
| SCRIPT-06 | TBD | Pending |
| SCRIPT-07 | TBD | Pending |
| OBS-01 | TBD | Pending |
| OBS-02 | TBD | Pending |
| OBS-03 | TBD | Pending |

**Coverage:**
- v1 requirements: 33 total
- Mapped to phases: 0 (populated by roadmapper)
- Unmapped: 33 (to be resolved)

---
*Requirements defined: 2026-04-14*
*Last updated: 2026-04-14 after initial definition from SESSION-PERSISTENCE-SPEC.md*
