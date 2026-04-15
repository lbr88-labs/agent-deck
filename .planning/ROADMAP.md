# Roadmap: Agent-Deck v1.5.2 — Session Persistence Hotfix

## Overview

This is a brownfield hotfix off v1.5.1 on branch `fix/session-persistence`. It closes two recurring production failures (SSH logout destroying every managed tmux server; error-path recovery failing to resume the prior Claude conversation) and permanently test-gates both so they cannot regress a fourth time. The authoritative spec is `docs/SESSION-PERSISTENCE-SPEC.md`. No new features, no breaking changes, no push/tag/PR.

The work is organized TDD-first: **Phase 1** lands all eight `TestPersistence_*` regression tests in RED state (plus one green pin for the opt-out inverse), **Phase 2** flips the cgroup isolation default to fix REQ-1 and turns the Linux-default / macOS-default / login-session-survival tests green, **Phase 3** audits the resume code paths to fix REQ-2 and turns the restart / SIGKILL / sidecar-deletion / fresh-session tests green, **Phase 4** ships the visual verification harness, CI wiring, and docs/CHANGELOG touches.

- **Milestone:** v1.5.2 "session-persistence"
- **Branch:** fix/session-persistence (off local `main`, not pushed)
- **Granularity:** standard
- **Total v1 requirements:** 37 (PERSIST × 13, TEST × 9, DOC × 5, SCRIPT × 7, OBS × 3)
- **Phases:** 5
- **Spec:** `docs/SESSION-PERSISTENCE-SPEC.md`
- **Repo mandate:** `CLAUDE.md` "Session persistence: mandatory test coverage" section

## Phases

- [x] **Phase 1: Persistence test scaffolding (RED)** - Land all eight `TestPersistence_*` regression tests to fail against current code, pinning both failure modes
- [x] **Phase 2: Cgroup isolation default (REQ-1 fix)** - Flip `launch_in_user_scope` default to true on Linux+systemd with detection and graceful fallback
- [x] **Phase 3: Resume-on-start and error-recovery (REQ-2 fix)** - Route every Claude start path through `ClaudeSessionID` resume logic with authoritative instance storage
- [x] **Phase 4: Verification harness, docs, and CI wiring** - Ship `scripts/verify-session-persistence.sh`, verify CLAUDE.md mandate completeness, and wire CI gates
- [ ] **Phase 5: Custom-command JSONL resume (REQ-7 fix)** - Resume from latest JSONL when `ClaudeSessionID` is empty, then write-through persist the discovered UUID so future restarts take the Phase 3 fast path

## Phase Details

### Phase 1: Persistence test scaffolding (RED)

**Goal**: All eight `TestPersistence_*` regression tests exist in `internal/session/session_persistence_test.go`, run independently, skip cleanly on non-systemd hosts, and fail against current v1.5.1 code for the six that will be fixed in Phases 2–3. The two default-detection tests (TEST-03, TEST-04) and the inverse pin (TEST-02) may behave differently per host — see criteria below.

**Depends on**: Nothing (first phase)

**Requirements**: TEST-01, TEST-02, TEST-03, TEST-04, TEST-05, TEST-06, TEST-07, TEST-08

**Success Criteria** (what must be TRUE):
  1. `go test -run TestPersistence_ ./internal/session/... -race -count=1` executes all eight tests on a Linux+systemd host with no compile errors.
  2. On Linux+systemd, TEST-02 (`TmuxDiesWithoutUserScope`) passes immediately — it pins the opt-out failure mode and must stay green from the moment the suite lands through the rest of the milestone.
  3. On Linux+systemd, TEST-01, TEST-03, TEST-05, TEST-06, TEST-07, TEST-08 fail against current code (RED) with failure messages that unambiguously reference the cgroup default or the resume path — not compile errors, not vacuous passes.
  4. TEST-04 (`MacOSDefaultIsDirect`) passes on any host lacking `systemd-run` and skips with a clear reason on Linux+systemd hosts (or passes there only once Phase 2 explicitly adds the branch — whichever the implementer chooses, the behavior is documented in the test file header).
  5. On macOS / BSD / any host without `systemd-run`, every test that requires real systemd skips with `t.Skipf("no systemd-run available: <reason>")` — no vacuous passes, no errors.
  6. Each test cleans up every tmux server, JSONL transcript, and hook sidecar it creates; `tmux list-sessions` shows no stray `agentdeck-test-*` servers after the suite runs.

**Plans**: TBD

### Phase 2: Cgroup isolation default (REQ-1 fix)

**Goal**: On Linux+systemd hosts, `launch_in_user_scope` defaults to `true` without any user configuration, spawning tmux under `user@UID.service` so SSH logout no longer kills the server. On macOS / BSD / Linux-without-user-manager hosts, the default silently stays `false`. Explicit `launch_in_user_scope = false` in `config.toml` is always honored. One structured startup log line records the decision.

**Depends on**: Phase 1 (all eight tests must exist before the fix lands)

**Requirements**: PERSIST-01, PERSIST-02, PERSIST-03, PERSIST-04, PERSIST-05, PERSIST-06, OBS-01

**Success Criteria** (what must be TRUE):
  1. `TmuxSettings{}.GetLaunchInUserScope()` returns `true` on a Linux host where `systemd-run --user --version` succeeds, with no `~/.agent-deck/config.toml` present — verified by TEST-03 turning green.
  2. The same call returns `false` on a host where `systemd-run` is absent, and emits no error — verified by TEST-04 turning green.
  3. `TestPersistence_TmuxSurvivesLoginSessionRemoval` (TEST-01) passes: a simulated login-session teardown leaves the agent-deck tmux server PID alive.
  4. `TestPersistence_TmuxDiesWithoutUserScope` (TEST-02) still passes: explicit opt-out with `launch_in_user_scope=false` continues to die under the simulated teardown, proving the fix is the cgroup default and not something that masks opt-outs.
  5. On a Linux+systemd host after a fresh install, `systemctl --user status` shows an `agentdeck-tmux-*.scope` unit for each live agent-deck session.
  6. On startup, `~/.agent-deck/logs/*.log` contains exactly one of: `tmux cgroup isolation: enabled (systemd-run detected)`, `tmux cgroup isolation: disabled (systemd-run not available)`, or `tmux cgroup isolation: disabled (config override)` — verified by `grep 'tmux cgroup isolation' ~/.agent-deck/logs/*.log`.
  7. If `systemd-run` is present but invocation fails (e.g. no user manager), the spawn falls back to direct tmux and logs a warning — session creation is never blocked.

**Plans:** 5 plans

Plans:
- [ ] 02-01-PLAN.md — Detection helper isSystemdUserScopeAvailable() with sync.Once cache + unit test (PERSIST-01, PERSIST-02)
- [ ] 02-02-PLAN.md — Migrate LaunchInUserScope to *bool, rewrite GetLaunchInUserScope, add ExplicitOptOut pin (TEST-03 GREEN; PERSIST-01..03)
- [ ] 02-03-PLAN.md — Emit OBS-01 startup log line + wire into bootstrap (OBS-01)
- [ ] 02-04-PLAN.md — Graceful systemd-run failure fallback in internal/tmux/tmux.go (TEST-01 GREEN; PERSIST-04..06)
- [ ] 02-05-PLAN.md — Update example-config comments + final full-suite confirmation + Phase 2 sign-off (PERSIST-01..03)

### Phase 3: Resume-on-start and error-recovery (REQ-2 fix)

**Goal**: Every code path that starts a Claude session for an Instance with non-empty `ClaudeSessionID` routes through the resume logic: `claude --resume <id>` when `sessionHasConversationData()` is true, else `claude --session-id <id>`. This applies to `session start`, `session restart`, automatic error-recovery, and conductor-driven restart after tmux teardown. `ClaudeSessionID` is read authoritatively from instance JSON storage (not the hook sidecar) and is preserved through any transition to `stopped` or `error`.

**Depends on**: Phase 2 (cgroup default is in place so error-recovery is exercised via intentional SIGKILL, not via production-like logout teardown)

**Requirements**: PERSIST-07, PERSIST-08, PERSIST-09, PERSIST-10, OBS-02, OBS-03

**Success Criteria** (what must be TRUE):
  1. `TestPersistence_RestartResumesConversation` (TEST-05) passes: stop → restart on a session with a JSONL transcript produces a claude command line containing `--resume <ClaudeSessionID>`.
  2. `TestPersistence_StartAfterSIGKILLResumesConversation` (TEST-06) passes: after SIGKILL of the tmux server and state transition to `error`, `agent-deck session start` (not `restart`) still resumes.
  3. `TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion` (TEST-07) passes: with `~/.agent-deck/hooks/<instance>.sid` deleted, `ClaudeSessionID` is still read from instance JSON storage and applied.
  4. `TestPersistence_FreshSessionUsesSessionIDNotResume` (TEST-08) passes: a first start with no prior conversation spawns `claude --session-id <uuid>`, never `--resume` with a non-existent ID.
  5. `ClaudeSessionID` is preserved through any `stopped` or `error` transition — cleared only on explicit delete or user-initiated `fork`. Verified by manual inspection of instance JSON after stop / SIGKILL / error-recovery cycles.
  6. On every `session start` / `restart`, `~/.agent-deck/logs/*.log` contains exactly one structured line of the form `resume: id=<x> reason=conversation_data_present` or `resume: none reason=fresh_session` — verified by `grep 'resume:' ~/.agent-deck/logs/*.log` returning rows.
  7. The invariants of `docs/session-id-lifecycle.md` remain honored — no disk-scan authoritative binding, instance JSON is the source of truth.

**Plans:** 5 plans

Plans:
- [x] 03-01-PLAN.md — Regression guard TestPersistence_ClaudeSessionIDPreservedThroughStopError (PERSIST-08)
- [x] 03-02-PLAN.md — RED test TestPersistence_SessionIDFallbackWhenJSONLMissing (PERSIST-07, PERSIST-09)
- [x] 03-03-PLAN.md — Route Start/StartWithMessage through buildClaudeResumeCommand (PERSIST-07..09; TEST-05..08 GREEN)
- [x] 03-04-PLAN.md — Emit OBS-02 resume log line + three log-capture tests (OBS-02, OBS-03)
- [x] 03-05-PLAN.md — PERSIST-10 subsection in docs/session-id-lifecycle.md + Phase 3 sign-off in STATE.md (PERSIST-10)

### Phase 4: Verification harness, docs, and CI wiring

**Goal**: Ship `scripts/verify-session-persistence.sh` — a human-watchable end-to-end script that proves the fix is live on any Linux+systemd host. Verify the existing `CLAUDE.md` mandate section covers DOC-01..DOC-04 verbatim (patching any gaps), and add the one-line v1.5.2 mention to `README.md` or `CHANGELOG.md` (DOC-05). Wire both the full `TestPersistence_*` suite and the verification script into CI so PRs touching the mandated paths are hard-gated.

**Depends on**: Phase 3 (the script exercises the Phase 2 and Phase 3 fixes end-to-end)

**Requirements**: SCRIPT-01, SCRIPT-02, SCRIPT-03, SCRIPT-04, SCRIPT-05, SCRIPT-06, SCRIPT-07, DOC-01, DOC-02, DOC-03, DOC-04, DOC-05

**Success Criteria** (what must be TRUE):
  1. `bash scripts/verify-session-persistence.sh` runs end-to-end on the conductor host (Linux+systemd) and exits `0` with every scenario printing a green `[PASS]` banner.
  2. The script prints a numbered checklist of scenarios up front; for each live tmux server it prints the PID and the `/proc/<pid>/cgroup` path so a human can visually confirm it's under `user@UID.service` and not a login-session scope.
  3. On macOS / non-systemd hosts, the login-session-teardown scenario prints `skipped: no systemd-run` in yellow and the overall script still exits `0`; no scenario errors vacuously.
  4. The stop/restart scenario prints the exact claude command line spawned and highlights whether it contains `--resume` or `--session-id`; the script exits non-zero if neither appears for a session with a non-empty `ClaudeSessionID`.
  5. The repo `CLAUDE.md` "Session persistence: mandatory test coverage" section lists all eight `TestPersistence_*` tests verbatim (DOC-01), declares the PR mandate over the enumerated paths with test output or CI link required (DOC-02), names the 2026-04-14 incident as the reason (DOC-03), and states the no-flip-back-without-RFC rule (DOC-04).
  6. The top-level `README.md` or `CHANGELOG.md` contains exactly one line mentioning v1.5.2 session-persistence (DOC-05).
  7. CI runs both `go test -run TestPersistence_ ./internal/session/... -race -count=1` and `bash scripts/verify-session-persistence.sh`; a red of either blocks the PR. The CLAUDE.md section references the script path by name (SCRIPT-07).

**Plans:** 4 plans

Plans:
- [x] 04-01-PLAN.md — scripts/verify-session-persistence.sh + fake-claude stub (SCRIPT-01..06)
- [x] 04-02-PLAN.md — CLAUDE.md mandate audit + CHANGELOG.md v1.5.2 line (DOC-01..05)
- [x] 04-03-PLAN.md — .github/workflows/session-persistence.yml CI gate (SCRIPT-07)
- [x] 04-04-PLAN.md — End-to-end verify run + 04-VERIFY.md + STATE.md sign-off (verification-only)

### Phase 5: Custom-command JSONL resume (REQ-7 fix)

**Goal**: Any Claude-compatible Instance with empty `ClaudeSessionID` whose `ProjectPath` contains at least one UUID-named JSONL transcript resolves the latest one by mtime, populates `Instance.ClaudeSessionID` to that UUID, persists it to storage, and then routes through the existing Phase 3 `buildClaudeResumeCommand`. The structural code-layer fix that retires the 2026-04-15 ops-layer `start-conductor.sh` workaround — applies uniformly whether `Command` is empty (default wrapper) or a custom script path.

**Depends on**: Phase 3 (reuses `buildClaudeResumeCommand` unchanged; Phase 3's write-through pattern for `ClaudeSessionID` from the hook sidecar is extended to write-through from disk discovery)

**Requirements**: PERSIST-11, PERSIST-12, PERSIST-13, TEST-09

**Success Criteria** (what must be TRUE):
  1. `TestPersistence_CustomCommandResumesFromLatestJSONL` (TEST-09) passes: a Claude-compatible Instance with non-empty `Command`, empty `ClaudeSessionID`, and two UUID-named JSONLs of different mtimes under `~/.claude/projects/<encoded-path>/` produces a spawn command containing `--resume <newer-uuid>`.
  2. After that start, `Instance.ClaudeSessionID` equals the newer JSONL's UUID and the instance has been saved to storage — verified by re-loading the instance from its JSON file and inspecting the field.
  3. Existing tests TEST-05..TEST-08 continue to pass — no regression in Phase 3's resume contract. Verified by `go test -run TestPersistence_ ./internal/session/... -race -count=1` exiting 0 with all nine tests green.
  4. `Instance.Start()` at `instance.go:1893` and the `StartWithMessage` dispatch at `instance.go:2019-2033` both see an `IsClaudeCompatible && ClaudeSessionID == ""` instance handed to a small discovery helper that either populates `ClaudeSessionID` (when a JSONL is found) or leaves it empty (fresh-session path). The two dispatch sites MUST reuse the same helper — no duplicated discovery logic.
  5. The discovery helper is a new exported or internal function (e.g., `discoverLatestClaudeJSONL(projectPath string) (uuid string, found bool)`) in `internal/session/claude.go` that is pure (no side effects on `Instance`, no mutation) — `Instance.Start()` is responsible for the write-through persistence. Logic overlaps with `findActiveSessionID` (`claude.go:332`) but WITHOUT the 5-minute recency cap.
  6. If the encoded project directory is absent, empty, or contains zero UUID-named JSONLs, `discoverLatestClaudeJSONL` returns `("", false)` and `Start()` falls through to the existing fresh-session branch (`buildClaudeCommand` with a newly-minted UUID) — no `--resume`, no error log, no `[FAIL]` banner.
  7. No branch in `Start()` / `StartWithMessage` / `Restart()` reads the `Command` field to decide whether to scan for JSONLs — the discovery runs on every empty-ID Claude-compatible start, custom wrapper or not.
  8. An OBS-02-style log line is emitted when discovery finds a JSONL: `resume: id=<uuid> reason=jsonl_discovery` — so `grep 'resume:' ~/.agent-deck/logs/*.log` shows the discovery path was taken and distinguishes it from `conversation_data_present`. (Observability consistency with Phase 3, not a new OBS requirement.)
  9. `scripts/verify-session-persistence.sh` continues to exit 0 on the conductor host — Phase 5 does not change script scenarios but also does not regress them.

**Plans:** 3 plans

Plans:
- [ ] 05-01-PLAN.md — RED: TestPersistence_CustomCommandResumesFromLatestJSONL against unmodified Start() (TEST-09)
- [ ] 05-02-PLAN.md — GREEN: discoverLatestClaudeJSONL helper + ensureClaudeSessionIDFromDisk wired into Start/StartWithMessage (PERSIST-11, PERSIST-12, PERSIST-13; TEST-09 GREEN)
- [ ] 05-03-PLAN.md — REFACTOR/optional: portable unit tests for discoverLatestClaudeJSONL (newest-wins, agent-* skipped, non-UUID skipped, empty/missing dirs, no recency cap)

## Milestone success criteria

Mirrors the six items in `docs/SESSION-PERSISTENCE-SPEC.md` "Success criteria for the milestone". The milestone is shippable only when all six are observably true:

1. On the conductor host, after installing v1.5.2, `launch_in_user_scope` is effectively `true` without any config edit. Proof: `systemctl --user status` shows `agentdeck-tmux-*.scope` units.
2. An SSH logout cycle on the conductor host does not kill any agent-deck tmux server. Proof: manual test — record PIDs before logout, re-login, confirm same PIDs.
3. `go test ./internal/session/... -run TestPersistence_ -race -count=1` passes locally on Linux+systemd with all eight tests green.
4. `bash scripts/verify-session-persistence.sh` runs end-to-end on the conductor host and exits `0` with every scenario showing `[PASS]`. The user watches this run and confirms visually.
5. `git log main..HEAD --oneline` on the `fix/session-persistence` branch ends with a commit that adds or finalizes the CLAUDE.md mandate section.
6. No commit on this branch pushes, tags, or opens a PR.

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5. Within each phase, TDD ordering is a hard rule — test commits precede fix commits.

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Persistence test scaffolding (RED) | 2/2 | Complete | 2026-04-14 |
| 2. Cgroup isolation default (REQ-1 fix) | 6/6 | Complete | 2026-04-14 |
| 3. Resume-on-start and error-recovery (REQ-2 fix) | 5/5 | Complete | 2026-04-14 |
| 4. Verification harness, docs, and CI wiring | 4/4 | Complete | 2026-04-15 |
| 5. Custom-command JSONL resume (REQ-7 fix) | 0/3 | Planning | — |

---
*Roadmap created: 2026-04-14 from `docs/SESSION-PERSISTENCE-SPEC.md` and `.planning/REQUIREMENTS.md`. Phase 5 appended 2026-04-15 for REQ-7 (spec updated same day). Granularity: standard. Coverage: 37/37 v1 requirements mapped.*
