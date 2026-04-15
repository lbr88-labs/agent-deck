# Phase 2: env_file source semantics + observability + conductor E2E — Context

**Gathered:** 2026-04-15
**Status:** Ready for planning
**Source:** PRD Express Path — `docs/PER-GROUP-CLAUDE-CONFIG-SPEC.md` (commit `4ade7f8`), scoped to CFG-03, CFG-04 tests 4/5, CFG-07. Additive vs PR #578 (`fa9971e`).

<domain>
## Phase Boundary

Prove the per-group `env_file` is `source`d in the tmux spawn pipeline before `claude` (or the custom command) exec's, lock that behavior with two TDD regression tests (CFG-04 tests 4 and 5), and emit the CFG-07 observability log line at spawn. Additive on top of Phase 1 (CFG-01/02, tests 1/2/3/6 already GREEN). No changes to PR #578's existing assertions. No SQLite schema changes. No `git push`/tag/PR/merge.

**In scope (files touched):**
- `internal/session/pergroupconfig_test.go` (extend — add tests 4 and 5)
- `internal/session/env.go` (only if CFG-03 wiring gap exists — `getToolEnvFile()` already reads `GetGroupClaudeEnvFile(i.GroupPath)` at `env.go:248`, so this may be verification-only)
- `internal/session/claude.go` and/or `internal/session/instance.go` (CFG-07 log line at spawn — one location, once per session)

**Out of scope (this phase):**
- CFG-05 visual harness, CFG-06 docs/README/CHANGELOG/attribution commit (deferred to Phase 3)
- Any refactor or revert of PR #578 code
- direnv integration layer, profile-level `[profiles.<x>.claude]` semantics, per-group `mcp_servers`

</domain>

<decisions>
## Implementation Decisions

### CFG-03 — env_file sourced before claude exec (P0, locked)

- `[groups."<name>".claude] env_file = "/path/to/.envrc"` MUST cause the tmux pane to `source "/path/to/.envrc"` before `exec claude` (or the custom command).
- Path expansion mirrors `config_dir`: `~` expansion, absolute paths, env-var expansion (`$HOME`).
- Supports both shell-style `.envrc` (exports) and flat `KEY=VALUE` `.env` — bash `source` handles both identically.
- Missing file: log a warning, do NOT block session start. Route via `buildSourceCmd(resolved, ignoreMissing=true)` semantics already established in `env.go`.
- Priority: group `env_file` override > global `[claude].env_file` — already enforced at `env.go:248` in `getToolEnvFile()`.
- **Wiring check:** `buildEnvSourceCommand()` (at `env.go:20`) sources tool-specific env_file via `getToolEnvFile()` (at `env.go:240`) which already reads `config.GetGroupClaudeEnvFile(i.GroupPath)`. Phase 2 tests MUST prove this path is taken for both normal-claude and custom-command sessions; only add code if a gap is found. If a gap exists, fix minimally in `env.go`.
- **Non-goal:** No direnv layer, no hashing, no auto-reload. Just the `source` line.

### CFG-04 test 4 — `TestPerGroupConfig_EnvFileSourcedInSpawn` (P0, locked)

- New test in `internal/session/pergroupconfig_test.go`.
- Write a throwaway file under `t.TempDir()` (NOT `/tmp` — the spec says `/tmp/envrc-*` but Go test hygiene prefers `t.TempDir()` which auto-cleans; acceptable deviation, justified).
- File content: `export TEST_ENVFILE_VAR=hello`.
- Configure a test group with `env_file = "<tempdir>/envrc-test"`.
- Build the spawn prefix via the instance's existing prefix builder (whichever method assembles `buildEnvSourceCommand() + buildBashExportPrefix()` for the spawn pipeline).
- Assert: the built shell string contains `source "<resolved-path>"` (or equivalent `.` builtin), AND if feasible via `exec.Command("bash", "-c", prefix+"echo $TEST_ENVFILE_VAR")`, the output is `hello`.
- Self-cleaning via `t.TempDir()` + config reset. No network.
- Independently runnable: `go test -run TestPerGroupConfig_EnvFileSourcedInSpawn ./internal/session/...`.

### CFG-04 test 5 — `TestPerGroupConfig_ConductorRestartPreservesConfigDir` (P0, locked)

- New test in `internal/session/pergroupconfig_test.go`.
- Simulates the custom-command (conductor) restart path: create an `Instance` with non-empty `Command` (e.g., `bash -c 'exec claude'`) and a group override setting `config_dir = "~/.claude-work"`.
- Build the spawn command via `buildBashExportPrefix()` (or the instance helper used in `instance.go:541/559/598/4314`) → assert contains `export CLAUDE_CONFIG_DIR=~/.claude-work` (or resolved absolute path).
- Simulate restart: call `ClearUserConfigCache()` (or equivalent) then rebuild the prefix → assert `CLAUDE_CONFIG_DIR` is present and identical.
- No live tmux, no real process spawn — pure build-and-assert on the command strings. End-to-end in the sense that it exercises the full build→stop→restart code path, not the user-facing process.
- Self-cleaning; no network.

### CFG-07 — observability log line (P2, locked)

- On session spawn, emit exactly one log line at `slog` INFO level:
  `claude config resolution: session=<id> group=<g> resolved=<path> source=<env|group|profile|global|default>`
- **Attribution rule:** the `source=` field MUST reflect which priority level actually set the dir. Mapping to the priority chain in `GetClaudeConfigDirForGroup` (at `claude.go:246`):
  - `env` — `CLAUDE_CONFIG_DIR` env var was set at process start
  - `group` — group override hit (`config.GetGroupClaudeConfigDir(groupPath)` returned non-empty)
  - `profile` — profile-level override hit
  - `global` — `[claude].config_dir` from top-level config
  - `default` — fell through to `~/.claude`
- **Emission site:** the canonical spawn path that runs once per session start. Candidate: the code path that builds and invokes the tmux command in `instance.go` (around `buildBashExportPrefix` call-sites at `541`, `559`, `598`). Pick a single site on the normal-start path; do NOT emit from `buildBashExportPrefix` itself (it's called from multiple places including Fork, which would log duplicates).
- Fields use `slog.String`. Use the existing `sessionLog` logger (see `env.go:273`).
- Helper: introduce `GetClaudeConfigDirSourceForGroup(groupPath string) (path, source string)` in `claude.go` that returns both the resolved path AND the source label — avoids a second priority walk and keeps the log line honest. Internal; additive.

### Test file hygiene (locked)

- Extend existing `internal/session/pergroupconfig_test.go`. Keep tests 1/2/3/6 untouched.
- Each new test independently runnable, self-cleaning, no network.
- Use `t.Cleanup(func() { ClearUserConfigCache() })` where the test mutates config, so state leakage across `-count=1` runs cannot happen.
- TDD order: RED commit first (test compiles + fails), GREEN commit next (minimal code that makes it pass). Two commits per test is ideal; one combined RED+GREEN commit is acceptable only if the wiring turns out to exist and the test goes green on first run (explicit note in commit body).

### TDD and commit discipline (locked — carried from Phase 1)

- No `--no-verify`. All commits go through pre-commit hooks (repo-root `CLAUDE.md` mandate from v1.5.3 `ee7f29e`).
- Sign: `Committed by Ashesh Goplani`. No Claude attribution.
- Substantive commits (non-trivial code or test additions) carry `Base implementation by @alec-pinson in PR #578.` in the body where PR #578's schema/lookup is the foundation. At minimum: the CFG-07 helper addition commit and the CFG-03 wiring-fix commit (if any) should carry attribution. The pure-test commits may omit it.
- No `git push`, no `git tag`, no `gh release`, no `gh pr create`, no `gh pr merge` during this phase.
- No `rm` — use `trash` if cleanup needed.
- Go 1.24.0 toolchain pinned (Go 1.25 breaks macOS TUI). Run tests with `-race -count=1`.

### Claude's Discretion

- Exact name and signature of the `GetClaudeConfigDirSourceForGroup` helper (internal; can be refactored later).
- Choice of emission site for the CFG-07 log line (must be single-shot per session; planner picks the cleanest seam in `instance.go` among the three existing `buildBashExportPrefix` call-sites — prefer the one on the normal-start path, not Fork).
- Test harness mechanics for CFG-04 test 4: asserting on built command string vs. actually executing `bash -c` with the prefix. Either is acceptable if the assertion is deterministic and self-cleaning.
- Commit granularity within the phase (RED/GREEN split per test vs. combined) as long as TDD ordering is preserved and attribution rules are honored.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Source spec
- `docs/PER-GROUP-CLAUDE-CONFIG-SPEC.md` — full v1.5.4 spec (commit `4ade7f8`); REQ-3 (CFG-03), REQ-4 items 4 and 5 (CFG-04 tests 4/5), REQ-7 (CFG-07) are this phase's contract.

### Project state
- `.planning/ROADMAP.md` — Phase 2 block lists success criteria and scope.
- `.planning/REQUIREMENTS.md` — CFG-03, CFG-04 (tests 4/5), CFG-07 definitions and traceability.
- `.planning/STATE.md` — current milestone position.
- `.planning/phases/01-custom-command-injection-core-regression-tests/01-01-SUMMARY.md` — Phase 1 outcome (CFG-02 closed, `buildBashExportPrefix()` now on the custom-command return path at `instance.go:596`).
- `.planning/phases/01-custom-command-injection-core-regression-tests/01-VERIFICATION.md` — Phase 1 verification (8/8 must-haves), informs Phase 2 regression baseline.
- `.planning/phases/01-custom-command-injection-core-regression-tests/deferred-items.md` — 6 pre-existing tmux-env failures in `internal/session` at parent `4730aa5` (NOT Phase 2's problem; log if re-surfaced).

### Code seams (read before touching)
- `internal/session/pergroupconfig_test.go` — existing tests 1/2/3/6 (Phase 1); extend additively.
- `internal/session/env.go` — `buildEnvSourceCommand()` at L20, `getToolEnvFile()` at L240; env_file plumbing.
- `internal/session/claude.go` — `GetClaudeConfigDirForGroup()` at L246; priority chain is the source of truth for CFG-07's `source=` label.
- `internal/session/instance.go` — `buildBashExportPrefix()` at L603; call-sites at L541, L559, L598, L4314 (Fork). Phase 2 emits CFG-07 from the normal-start path, NOT from Fork.
- `internal/session/claude_test.go` — PR #578 tests `TestGetClaudeConfigDirForGroup_GroupWins` (L693) and `TestIsClaudeConfigDirExplicitForGroup`; MUST stay GREEN.

### Hard rules (repo-level)
- `CLAUDE.md` (repo root) — session-persistence mandate + no `--no-verify` rule (from v1.5.3 `ee7f29e`).

</canonical_refs>

<specifics>
## Specific Ideas

- `sessionLog` logger is already wired (see `env.go:273` `sessionLog.Warn(...)`). Reuse it for CFG-07.
- CFG-07 log line format is locked verbatim from the spec: `claude config resolution: session=<id> group=<g> resolved=<path> source=<env|group|profile|global|default>`. Implement with `slog.Info("claude config resolution", slog.String("session", id), slog.String("group", g), slog.String("resolved", path), slog.String("source", src))` — slog key=value rendering matches the contract.
- CFG-04 test 4 sentinel var: `TEST_ENVFILE_VAR=hello` (spec-locked). Use `t.TempDir()` for the envrc file location (Go idiom; `/tmp/envrc-*` in the spec was illustrative).
- CFG-04 test 5 can reuse the `Instance` builder pattern from Phase 1's test 1 (custom command) — same Command field, same GroupPath seeding; the delta is the restart simulation (`ClearUserConfigCache()` + rebuild).
- Expected verification command: `go test ./internal/session/... -run TestPerGroupConfig_ -race -count=1` — all 6 tests GREEN (the 4 from Phase 1 plus the 2 added here).
- Expected CI gate: `make ci` non-zero return from six pre-existing tmux-env failures in `internal/session` is acceptable per Phase 1's `deferred-items.md`; Phase 2 must not add new failures.

</specifics>

<deferred>
## Deferred Ideas

- CFG-05 visual harness `scripts/verify-per-group-claude-config.sh` → Phase 3.
- CFG-06 README / CLAUDE.md / CHANGELOG updates + attribution commit → Phase 3.
- direnv integration layer (hashing, auto-reload) → future milestone.
- Per-group `mcp_servers` overrides → future milestone (may reuse this phase's lookup helpers).
- Rebase of `fa9971e` onto current `main` → merge-time concern, not this milestone.
- Manual conductor-host proof (`ps -p <pane_pid>` env check) → milestone verification step at `/gsd-complete-milestone`.

</deferred>

---

*Phase: 02-env-file-source-semantics-observability-conductor-e2e*
*Context gathered: 2026-04-15 via PRD Express Path (spec: docs/PER-GROUP-CLAUDE-CONFIG-SPEC.md @ 4ade7f8)*
