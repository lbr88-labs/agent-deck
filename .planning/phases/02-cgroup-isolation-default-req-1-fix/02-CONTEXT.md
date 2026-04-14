# Phase 2: Cgroup isolation default (REQ-1 fix) - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning
**Source:** Inline conductor instructions + docs/SESSION-PERSISTENCE-SPEC.md REQ-1 + Phase 1 RED tests

<domain>
## Phase Boundary

This phase delivers a single, narrow behavior change: on Linux+systemd hosts, the agent-deck tmux server is launched under `user@UID.service` BY DEFAULT — without any user editing `~/.agent-deck/config.toml` — so an SSH logout no longer destroys every managed session. The change is a runtime default flip plus a graceful fallback path plus one structured startup log line. Nothing else.

**In scope:**
- `TmuxSettings.GetLaunchInUserScope()` returns `true` on Linux+systemd hosts when no explicit config override is set.
- Same call returns `false` on macOS / BSD / Linux-without-`systemd-run`, with no error logged.
- Explicit `launch_in_user_scope = false` in `config.toml` is always honored (the opt-out path remains intact).
- Exactly one structured log line at startup describing the decision.
- Graceful fallback in `internal/tmux/` when `systemd-run` is detected but the spawn invocation fails (e.g. no user manager) — fall back to direct tmux, log warning, never block session creation.
- Phase 1 tests TEST-01 and TEST-03 must turn GREEN; TEST-02 and TEST-04 must STAY GREEN.

**Out of scope (deferred to later phases):**
- TEST-05 through TEST-08 (resume-dispatch path) — those belong to Phases that fix REQ-2 (Claude resume).
- Adding a new TUI surface for the cgroup-isolation status (logs are the only required surface).
- Restructuring `userconfig.go` beyond what the bool→pointer change requires.
- Touching `internal/session/instance.go` or `cmd/session_cmd.go` (those are Phase 3 territory).
- macOS launchd equivalent (explicitly out per spec line 47–48).

</domain>

<decisions>
## Implementation Decisions (LOCKED)

### Detection function (where Linux+systemd capability is probed)

- **Location:** `internal/session/userconfig.go`, alongside `TmuxSettings`.
- **Name:** `isSystemdUserScopeAvailable() bool` (or equivalent unexported helper). Implementer may pick the exact name; the contract is what matters.
- **Semantics:** Returns `true` IFF `exec.LookPath("systemd-run")` succeeds AND `systemd-run --user --version` exits zero. This mirrors the `requireSystemdRun` test helper at `internal/session/session_persistence_test.go:89-95`, so the production code and the test gate agree on what "Linux+systemd available" means.
- **Caching:** Cache the result in a package-level `sync.Once`-guarded variable. Detection runs at most once per process. Tests must be able to reset this cache via an unexported helper (e.g. `resetSystemdDetectionCache()`) called from the test package.
- **Side effects:** None beyond the `exec.Command` probe. Must NOT print to stderr/stdout. Must NOT panic. Errors from `exec.Command` are swallowed and treated as `false`.

### Default behavior change

- **TOML field type change:** `LaunchInUserScope bool` → `LaunchInUserScope *bool` in `internal/session/userconfig.go:879`. This is mandatory: a plain `bool` cannot distinguish "field absent" from "explicit `false`". Without the pointer change, the spec's "explicit `launch_in_user_scope = false` is always honored" rule cannot be implemented.
- **Getter logic:** `GetLaunchInUserScope()` returns:
  - `*t.LaunchInUserScope` if non-nil (explicit override — both `true` and `false` honored)
  - `isSystemdUserScopeAvailable()` otherwise (default — `true` on Linux+systemd, `false` elsewhere)
- **Backwards compatibility:** Existing configs with `launch_in_user_scope = true` or `launch_in_user_scope = false` continue to work because TOML decodes scalar booleans into `*bool` correctly.

### Startup log line (OBS-01)

- **Emitted from:** `internal/session/userconfig.go` is OK, but the natural location is wherever the application bootstraps user config. Implementer picks the call site; the requirement is that it fires exactly once per process at startup.
- **Logger:** Use the existing structured logger (`slog`) used elsewhere in the codebase. Do NOT introduce a new logging dependency.
- **Exact strings — pick exactly one per startup:**
  - `tmux cgroup isolation: enabled (systemd-run detected)` — when default kicks in AND `isSystemdUserScopeAvailable()` returned true AND no explicit config override.
  - `tmux cgroup isolation: disabled (systemd-run not available)` — when default kicks in AND `isSystemdUserScopeAvailable()` returned false AND no explicit config override.
  - `tmux cgroup isolation: disabled (config override)` — when explicit `launch_in_user_scope = false` in config.
  - (Implementer may add a fourth `tmux cgroup isolation: enabled (config override)` for explicit `true`; spec doesn't require it but it's the natural completion of the matrix. Decision deferred to planner — fine either way.)
- **Acceptance:** `grep 'tmux cgroup isolation' ~/.agent-deck/logs/*.log` must return exactly one row per process startup.

### Graceful systemd-run failure fallback

- **Location:** `internal/tmux/tmux.go` around the existing failure-handling block at lines 1366–1372.
- **Trigger:** `launcher == "systemd-run"` AND the invocation returns a non-zero exit. This means detection said "yes" but the actual spawn failed (e.g. no user manager, dbus down, disabled lingering).
- **Behavior:** Log a structured warning (e.g. `tmux_systemd_run_fallback`), then retry once with the direct tmux launcher (the same `args[1:]` list with `tmux` as launcher). If THAT also fails, return the original error wrapped — session creation only fails when both paths fail.
- **Logging:** Warning only — never an error. Session creation is the priority.
- **Test gating:** TEST-02 (`TestPersistence_TmuxDiesWithoutUserScope`) MUST stay green. Confirm the fallback path does not accidentally re-enable `--user --scope` for explicit opt-outs.

### Files to modify (authoritative list)

- `internal/session/userconfig.go` — pointer field, getter logic, detection helper, log emission. Bulk of the change lands here.
- `internal/tmux/tmux.go` — only the failure handler block at ~1366. A surgical addition, not a rewrite.
- `internal/session/session_persistence_test.go` — add a test that pins explicit-override behavior (`launch_in_user_scope = false` on a Linux+systemd host returns `false`). This closes the "explicit opt-out at the config layer" hole that TEST-02 alone doesn't cover (TEST-02 spawns tmux directly with the field, never exercising config-load).
- The two example-comment blocks in `userconfig.go` (around lines 875–878 and 1825–1828) need updating to reflect the new default. Line 878 also has a typo `/ scope is torn down` (single slash) that should be left alone unless the planner notices it — out of phase scope.

### Forbidden changes (per CLAUDE.md mandate)

- Do NOT remove or rename any of the eight required `TestPersistence_*` tests.
- Do NOT flip `launch_in_user_scope` default back to `false` on Linux. (Not the risk here, but the planner must be aware.)
- Do NOT skip the `verify-session-persistence.sh` script in CI configuration.
- Do NOT touch `internal/session/instance.go`, `internal/session/storage*.go`, or `cmd/session_cmd.go` — those are out of phase scope.

### Test-driven discipline (per CLAUDE.md)

- Phase 1 already landed RED tests (TEST-03 fails, TEST-01 fails with diagnostic). The implementation MUST turn them green WITHOUT modifying their assertions.
- Any new test (e.g. explicit-override pin) lands BEFORE its corresponding production code change, in its own commit.
- TEST-02 must stay green after the fallback patch — verify by running the full suite after each commit, not just at the end.

### Commit hygiene

- Each commit signed with `Committed by Ashesh Goplani` (per global rule). NO `Co-Authored-By: Claude` or `🤖 Generated with Claude Code` lines.
- Atomic commits per task: detection helper → field type change + getter → log line → fallback patch → new explicit-override test.
- `.planning/` is in `.git/info/exclude`; commit planning artifacts with `git add -f .planning/<file>`.
- No `git push`, no `git tag`, no `gh pr create` without explicit user approval (this phase ends at planning — no commits land yet).

### Claude's discretion (planner picks reasonable defaults)

- Exact name of the unexported detection helper.
- Whether to emit the optional fourth log line variant (`enabled (config override)`).
- Whether the structured log uses `slog.Info` with key/value pairs vs a single message string — the spec only cares about the grep substring.
- Order of plans within the phase (e.g. detection helper + new test first, then field type change, then log line, then fallback). Suggested wave structure is sequential since each step depends on the previous.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Specification
- `docs/SESSION-PERSISTENCE-SPEC.md` — full v1.5.2 spec. Phase 2 implements REQ-1 (lines 39–48), REQ-3 tests TEST-03 + TEST-04 (lines 75–76), TEST-01 + TEST-02 (lines 73 + the inverse test below it), and OBS-01 first sentence (line 111–113).

### Roadmap & requirements
- `.planning/ROADMAP.md` — Phase 2 section is the single source of truth for goal + success criteria.
- `.planning/REQUIREMENTS.md` — PERSIST-01 through PERSIST-06 and OBS-01 acceptance text. Every plan in this phase MUST list its REQ-IDs in frontmatter.

### Mandate
- `CLAUDE.md` (project root) — "Session persistence: mandatory test coverage" section. Lists the eight forbidden-to-remove tests and the paths under the mandate.

### Phase 1 artifacts (the RED tests this phase must turn GREEN)
- `internal/session/session_persistence_test.go` — contains all eight `TestPersistence_*` functions. Phase 2 must read this file before planning to understand the exact assertions and skip semantics. Lines 137–158 (TEST-03), 174–199 (TEST-04), 337+ (TEST-01), 463+ (TEST-02).
- `.planning/phases/01-persistence-test-scaffolding-red/01-SUMMARY.md` — documents what already exists and the RED→GREEN expectations for Phase 2.

### Production code touched
- `internal/session/userconfig.go` — `TmuxSettings` struct (lines 866–898), `GetLaunchInUserScope()` (lines 908–912), example config block in user-facing comment (lines 1822–1830).
- `internal/tmux/tmux.go` — `startCommandSpec` (lines 814–837 has the systemd-run wrap), failure handler (lines 1366–1372).

### Verification harness
- `scripts/verify-session-persistence.sh` — the human-watchable script per CLAUDE.md. Phase 2 does NOT need to modify it; the script must continue to exit zero on a Linux+systemd host after Phase 2 lands.

</canonical_refs>

<specifics>
## Specific Ideas

- The detection helper should be testable in isolation. Consider exposing a package-level function variable (`var systemdRunProbe = func() bool { ... }`) so a test can swap it for a stub without invoking real `systemd-run`. This is OPTIONAL — `requireSystemdRun` already gates the host matrix correctly, but a stubbable hook makes the cache-reset path cleaner.
- `slog` is already used throughout the codebase (search for `slog.Info` / `slog.Warn`). Reuse the same logger handle the rest of `internal/session/` uses; do not initialize a new one.
- The existing systemd-run wrap at `internal/tmux/tmux.go:833-836` already handles the `--collect` / `--unit` semantics correctly. Phase 2 does NOT modify that block — it modifies only the failure-handling branch a few hundred lines below.
- The structured log line at the user-facing path goes to `~/.agent-deck/logs/*.log`. Confirm the existing log writer flushes synchronously enough that a fresh-install grep within ~1s of startup will find the line. If not, add a sync — but the existing logger likely already handles this.

</specifics>

<deferred>
## Deferred Ideas

- TEST-05 / TEST-06 / TEST-07 / TEST-08 (resume-dispatch path) — already RED in Phase 1 commits but belong to a later phase that fixes REQ-2 (Claude `--resume`/`--session-id` discipline). Do NOT attempt to turn them green in this phase.
- A TUI affordance to show "cgroup isolation: enabled" in the session list — explicitly deferred per spec ("No goal is to surface this in the TUI; log-level is enough.").
- Restructuring `userconfig.go` to split `TmuxSettings` into its own file — out of scope, would balloon the diff.
- macOS launchd-equivalent isolation (LaunchAgent plist) — explicitly out of scope per spec line 47–48 ("No behavior change on macOS/BSD hosts").
- Telemetry on how many users hit the `enabled` vs `disabled` path — no telemetry infra exists yet; not this phase.

</deferred>

---

*Phase: 02-cgroup-isolation-default-req-1-fix*
*Context gathered: 2026-04-14 from inline conductor instructions + spec REQ-1 + Phase 1 RED tests*
