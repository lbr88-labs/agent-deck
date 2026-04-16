# Phase 21: Watcher Folder Hierarchy — Context

**Gathered:** 2026-04-16
**Status:** Ready for planning
**Source:** Deterministic spec (docs/WATCHER-COMPLETION-SPEC.md REQ-WF-6) — discuss-phase skipped
**Requirements addressed:** REQ-WF-6
**Parent commit:** bc368f2
**TDD:** Yes (RED → GREEN → migration task sequence mandated by phase goal)

<domain>
## Phase Boundary

**In scope:** Reorganize on-disk watcher state to mirror the conductor folder pattern. New singular `~/.agent-deck/watcher/` root with shared `CLAUDE.md`/`POLICY.md`/`LEARNINGS.md`/`clients.json` at the top level and per-watcher `meta.json`/`state.json`/`task-log.md`/`LEARNINGS.md` inside `<name>/`. Atomic one-time migration of legacy `~/.agent-deck/watchers/` on first run with a compatibility symlink `watchers -> watcher/` kept for one cycle.

**Out of scope (explicit):**
- Legacy `~/.agent-deck/issue-watcher/` (pre-framework bash directory) — NOT auto-migrated. Migration log line must call this out.
- REQ-WF-7 (skills + docs sync) — handled in a later phase.
- Any refactor of adapter code (github.go, slack.go, gmail.go, webhook.go, ntfy.go) beyond what layout/state wiring requires.
- New adapter types.

</domain>

<decisions>
## Implementation Decisions

### Path layout (LOCKED)
- Root: `~/.agent-deck/watcher/` (singular).
- Shared top-level files: `CLAUDE.md`, `POLICY.md`, `LEARNINGS.md`, `clients.json`.
- Per-instance subdir `<name>/` contains: `meta.json`, `state.json`, `task-log.md`, `LEARNINGS.md`.
- Compatibility: symlink `~/.agent-deck/watchers -> watcher/` written during migration; downstream scripts that hardcoded `watchers/` keep working for one minor cycle.

### Canonical meta format (LOCKED)
- `meta.json` is canonical. Matches existing `session.SaveWatcherMeta`/`LoadWatcherMeta` in `internal/session/watcher_meta.go` (which already writes `meta.json`). No new TOML is introduced. If any legacy `watcher.toml` exists next to an existing meta.json, it is left untouched (human-readable form); it is not read by the engine.
- **Why this over the spec's "implementer's call":** the current codebase already ships a `meta.json` writer with an atomic write-temp-rename pattern. Picking JSON avoids adding a second parser path and keeps "one format, one writer" simple.

### Migration (LOCKED)
- On engine startup: if `~/.agent-deck/watchers/` exists AND `~/.agent-deck/watcher/` does not exist → `os.Rename(watchers, watcher)` then `os.Symlink(watcher, watchers)`.
- Atomicity: `os.Rename` on same filesystem is POSIX-atomic. Symlink creation follows immediately. Migration is single-shot (never re-runs once `watcher/` exists).
- One log line emitted via the stdlib `log` package (or whatever the engine currently uses — follow engine convention) with a fixed prefix so tests can grep for it. Include an explicit mention that `~/.agent-deck/issue-watcher/` is NOT migrated.
- If BOTH `watchers/` and `watcher/` exist (operator intervention or prior failed migration), skip migration silently and log a warning — do NOT clobber either directory.

### New files (LOCKED)
- `internal/watcher/layout.go` — `LayoutDir() (string, error)`, `WatcherDir(name string) (string, error)`, `MigrateLegacyWatchersDir() error`, scaffold helpers for top-level shared files (idempotent).
- `internal/watcher/state.go` — `WatcherState` struct (`LastEventTS time.Time`, `ErrorCount int`, `HealthWindow []HealthSample`, `DedupCursor string`), `LoadState(name) (*WatcherState, error)`, `SaveState(name string, s *WatcherState) error`. Atomic write-temp-rename pattern (same as existing `SaveWatcherMeta`).
- `internal/watcher/event_log.go` — `AppendEventLog(name, entry string) error` writing one Markdown line per inbound event in the form `## <RFC3339-ts> - <event_type>: <summary>`. Append-only, O_APPEND|O_CREATE, 0o644.

### Code changes to existing files (LOCKED)
- `internal/watcher/engine.go:134` — default `ClientsPath` becomes `filepath.Join(home, ".agent-deck", "watcher", "clients.json")` with a fallback read of `watchers/clients.json` for one cycle (only if the singular path does not resolve).
- `cmd/agent-deck/watcher_cmd.go:562, 614` — `session.WatcherDir()` callers must point at the singular path. Either (a) update `session.WatcherDir()` itself in `internal/session/watcher_meta.go:20-27` to return `filepath.Join(dir, "watcher")` or (b) leave `session.WatcherDir()` unchanged and introduce a new resolver in `internal/watcher/layout.go` that those call sites switch to. **Decision: update `session.WatcherDir()` to singular.** Rationale: the function is the single source of truth for the path; two resolvers drift. Any existing callers under `internal/session/` that walked the plural dir (e.g. `watcher_meta.go` itself) are updated in the same commit.
- Wire the engine's event-handling loop: after dedup + routing, call `AppendEventLog(name, entry)` then `SaveState(name, s)` with the updated cursor. Failures in these calls must not drop the event — log and continue.

### Templates (LOCKED)
- `assets/watcher-templates/CLAUDE.md` — default guidance for any agent inspecting `~/.agent-deck/watcher/`.
- `assets/watcher-templates/POLICY.md` — default escalation / dedup / retry rules.
- `assets/watcher-templates/LEARNINGS.md` — empty-with-heading scaffold.
- Scaffolding runs once on first `LayoutDir()` call; only writes files that don't already exist.

### CLI surface (LOCKED)
- `agent-deck watcher list --json` output MUST include `last_event_ts`, `error_count`, `health_status` per watcher. These values come from the per-watcher `state.json`. If `state.json` is missing (fresh watcher, never received an event), emit `null` / `0` / `"unknown"` respectively.
- `health_status` is derived from `HealthWindow`: see existing `internal/watcher/health.go` for the classification rule; mirror it (or call it) — do NOT reinvent a parallel classifier.

### TDD sequencing (LOCKED by phase goal)
Single plan with tasks A → B → C → D, committed in sequence:
- **Task A (RED):** `internal/watcher/layout_test.go` with 6 unit tests (fresh-install-creates-layout, legacy-migration-atomic, symlink-resolves, state-roundtrip, event-log-append-atomic, hot-reload-safe) + 1 integration test (three events → three task-log lines + three state.json updates). All MUST fail initially. Commit: `test(21-01): watcher folder hierarchy RED`.
- **Task B (GREEN):** Create `layout.go`/`state.go`/`event_log.go`, update `engine.go:134`, update `session.WatcherDir()` + its callers, wire engine loop. Tests go green. Commit: `feat(21-01): watcher folder hierarchy GREEN + migration`.
- **Task C (templates):** Create `assets/watcher-templates/{CLAUDE.md, POLICY.md, LEARNINGS.md}`. Commit: `docs(21-01): scaffold watcher CLAUDE.md/POLICY.md/LEARNINGS.md templates`.
- **Task D (CLI surface):** Extend `agent-deck watcher list --json` to include `last_event_ts`, `error_count`, `health_status`. Add unit test in `cmd/agent-deck/watcher_cmd_test.go` asserting the JSON schema. Commit: `feat(21-01): expose watcher health fields in list --json`.

### Claude's Discretion
- Exact naming of internal helpers inside `layout.go` / `state.go` (must be exported where used from `cmd/`, unexported otherwise).
- Whether `MigrateLegacyWatchersDir` is exported (called from `engine.go` startup) or invoked via an `init` helper — recommend exported so tests can drive it directly.
- Exact schema of `HealthWindow` (slice of samples with timestamp+ok-bool) as long as it supports the classification in `health.go`.
- Log prefix string for the migration line (suggested: `watcher: migrated legacy ~/.agent-deck/watchers/ → ~/.agent-deck/watcher/`).
- Test file structure — one `layout_test.go` can hold all 7 tests or they can be split into `layout_test.go` + `state_test.go` + `event_log_test.go` + `integration_test.go` at the planner's discretion. Phase goal names only `layout_test.go` minimum.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Spec
- `docs/WATCHER-COMPLETION-SPEC.md` — REQ-WF-6 section (lines 68–117). Authoritative for target layout, migration semantics, acceptance.

### Analog: conductor folder pattern
- `internal/session/conductor.go:246-260` — `ConductorDir()` / per-instance dir helpers. Direct analog for `LayoutDir()` / `WatcherDir(name)`.
- `~/.agent-deck/conductor/` on disk — inspect for real-world shape of shared + per-instance layout.

### Existing code to modify
- `internal/session/watcher_meta.go:20-27` — `WatcherDir()`. Currently returns plural `watchers`. Flip to singular `watcher` in Task B.
- `internal/session/watcher_meta.go:47-75` — `SaveWatcherMeta` atomic write-temp-rename pattern. Copy this pattern exactly for `SaveState`.
- `internal/watcher/engine.go:130-140` — `ClientsPath` default assignment. Update to singular.
- `internal/watcher/engine.go` event loop — find the post-dedup call site; wire `AppendEventLog` + `SaveState` there.
- `cmd/agent-deck/watcher_cmd.go:521-525` — current error message references `~/.agent-deck/watchers/clients.json`. Update to singular.
- `cmd/agent-deck/watcher_cmd.go:562, 614` — consumers of `session.WatcherDir()`. No change needed if `session.WatcherDir()` itself is updated.
- `cmd/agent-deck/watcher_cmd.go:666-710` — `mergeClientsJSON` atomic writer. Reference for atomic-write conventions already in use.

### Health classification reuse
- `internal/watcher/health.go` — existing health classifier. Reuse for `health_status` in `list --json`; do not reinvent.

### Test analog
- `internal/session/conductor_test.go` — testing patterns for folder-hierarchy code with temp HOME dirs. Mirror for `layout_test.go`.
- `internal/watcher/health_bridge_test.go` — recent watcher-side test with `t.Setenv("HOME", ...)` setup. Closest analog.

</canonical_refs>

<specifics>
## Specific Ideas

- Migration log line must be greppable by tests — pick a fixed prefix like `watcher: migrated legacy` and assert on it in the migration test.
- `hot-reload-safe` test: start the engine with `watcher/` already in place, write to a per-watcher `state.json` while engine is running, assert engine's next read sees the new value without restart. This validates that state I/O isn't cached in-process inappropriately.
- Integration test: fire three synthetic events through the event-handling path, assert (a) `<name>/task-log.md` has exactly three `##` heading lines, (b) `<name>/state.json.last_event_ts` matches the third event's timestamp, (c) `error_count` is 0.
- Atomic append for `task-log.md`: `O_APPEND|O_CREATE|O_WRONLY` with a single `Write` call per entry — POSIX guarantees atomicity for writes ≤ PIPE_BUF. Include this reasoning in a code comment since the "why" is non-obvious.
- When scaffolding top-level templates on first run, check-and-create each file independently (don't gate on directory existence alone) so a user who deletes `POLICY.md` gets it regenerated on next boot.

</specifics>

<deferred>
## Deferred Ideas

- Migration of `~/.agent-deck/issue-watcher/` — explicitly out of scope. Log line must call this out so future work has a handle.
- Removing the `watchers -> watcher/` compatibility symlink — deferred to v1.7.0 (one minor cycle per spec).
- Any UI for viewing per-watcher LEARNINGS.md — future phase.
- TOML round-trip between `watcher.toml` and `meta.json` — not introduced here; if needed later, add a one-shot importer.

</deferred>

---

*Phase: 21-watcher-folder-hierarchy*
*Context gathered: 2026-04-16 (deterministic spec path, discuss-phase skipped)*
