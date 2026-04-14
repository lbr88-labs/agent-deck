# Phase 3: Resume-on-start and error-recovery (REQ-2 fix) - Context

**Gathered:** 2026-04-14
**Status:** Ready for planning
**Source:** Inline conductor instructions + docs/SESSION-PERSISTENCE-SPEC.md REQ-2 + Phase 1 RED tests TEST-05..08 + dispatch-path analysis of internal/session/instance.go

<domain>
## Phase Boundary

This phase delivers a single, narrow behavior fix: every code path that starts a Claude session for an `Instance` with a non-empty `ClaudeSessionID` routes through `buildClaudeResumeCommand()` so the spawned command line is:

- `claude --resume <id>` when `sessionHasConversationData(i.ClaudeSessionID, i.ProjectPath)` returns true, OR
- `claude --session-id <id>` (same stored id, NO mint) when the JSONL is missing or empty.

A fresh session (empty `ClaudeSessionID`) continues to use the existing capture-resume pattern in `buildClaudeCommand()` which generates a new UUID.

This routing rule MUST hold for:
1. `session start` (`cmd/agent-deck/session_cmd.go:188` → `inst.Start()`).
2. `session start --message` (`cmd/agent-deck/session_cmd.go:183` → `inst.StartWithMessage(...)`).
3. `session restart` (`cmd/agent-deck/session_cmd.go:342` → `inst.Restart()`). Restart's respawn-pane branch at `instance.go:3788` and recreate branch at `instance.go:4018` already behave correctly — Phase 3 adds regression guards.
4. Automatic error-recovery from TUI / daemon / programmatic callers that invoke `inst.Start()` on an instance with `Status == StatusError`.
5. Conductor-driven restart after tmux teardown — any programmatic call site that rebuilds the Claude command for a resumable instance.

Phase 1 landed RED tests TEST-05/06/07/08. Phase 3 MUST turn them GREEN. TEST-01..04 (Phase 2) MUST stay GREEN. The bugfix lives primarily in one branch of a switch statement; most of the diff is surface — tests, a single conditional, and one structured log line.

**In scope:**
- Modify `Start()` (`instance.go:1873-1986`) and `StartWithMessage()` (`instance.go:1992-...`) so the Claude-compatible switch arm (currently `command = i.buildClaudeCommand(i.Command)` at lines 1883 and 2002) branches on `i.ClaudeSessionID != ""`.
- Add a structured log line from every Claude start path (Start, StartWithMessage, Restart respawn branch, Restart recreate branch) emitting exactly one of:
  - `resume: id=<ClaudeSessionID> reason=conversation_data_present`
  - `resume: id=<ClaudeSessionID> reason=session_id_flag_no_jsonl`
  - `resume: none reason=fresh_session`
  - `resume: none reason=tool_not_claude`
- Add regression test `TestPersistence_ClaudeSessionIDPreservedThroughStopError` pinning PERSIST-08 invariant.
- Add regression test `TestPersistence_SessionIDFallbackWhenJSONLMissing` pinning divergence behavior: stored ID non-empty, JSONL absent → spawned argv contains `--session-id <stored-id>`, NEVER `--resume`, and `ClaudeSessionID` value is unchanged afterwards.
- Update `docs/session-id-lifecycle.md` with a "Start / Restart dispatch" subsection stating the new routing rule and reaffirming instance JSON as authoritative (mirrors PERSIST-10).

**Out of scope (deferred or out of milestone):**
- Fork behavior: `Fork()` at `instance.go:4274` mints a new UUID for the target — unchanged.
- Explicit delete flow: clearing `ClaudeSessionID` on delete is the existing contract — unchanged.
- `/clear` / tool-driven session rotation: surfaced via hook events, handled in `updateSessionIDFromHook` — unchanged.
- UI affordance showing "resumable" status (spec REQ-2 non-goals).
- Migration of the 33 pre-existing error / 39 pre-existing stopped sessions on the user's host (spec "Out of scope").
- Any behavior change to Gemini / OpenCode / Codex / custom tool branches — Phase 3 touches only the `IsClaudeCompatible(i.Tool)` arm.
- Changes to `cgroupIsolationLog` / OBS-01 (Phase 2 territory).

</domain>

<decisions>
## Implementation Decisions (LOCKED)

### 1. Dispatch rule in Start() and StartWithMessage()

- **Location:** `internal/session/instance.go:1881-1901` (Start) and `instance.go:1999-2018` (StartWithMessage).
- **Current code (bug):**
  ```go
  case IsClaudeCompatible(i.Tool):
      command = i.buildClaudeCommand(i.Command)
  ```
- **Required code (fix):**
  ```go
  case IsClaudeCompatible(i.Tool):
      if i.ClaudeSessionID != "" {
          command = i.buildClaudeResumeCommand()
      } else {
          command = i.buildClaudeCommand(i.Command)
      }
  ```
  This mirrors the resume check at `instance.go:3788` for Restart's respawn-pane branch and at `instance.go:4018` for Restart's recreate branch.
- **StartWithMessage extra constraint:** when a resume path is taken AND an initial message is provided, the initial-message send logic (currently wired through `buildClaudeCommandWithMessage`) must still deliver the message. The planner may either (a) extend `buildClaudeResumeCommand()` to accept an optional initial message, OR (b) send the message via the existing post-start PTY send path (`inst.SendMessage()` equivalent after grace period). Decision deferred to planner; pick whichever keeps the diff smaller. Both paths must be exercised by a unit test.
- **Rationale:** This is the 2026-04-14 incident root cause. `buildClaudeCommand()` at `instance.go:566-567` mints a fresh UUID via `generateUUID()` and overwrites `i.ClaudeSessionID`. When Start() is called on a `StatusError` instance with a populated ID, the overwrite produces the `f1e103df → b9403638` divergence the user observed on the conductor session.

### 2. Structured resume log line (OBS-02)

- **Emit from:** `buildClaudeResumeCommand()` at `instance.go:4114` (the natural choke point — every resume-or-fallback goes through here) AND from the fresh-session branch of `buildClaudeCommand()` / Start() for the `resume: none` lines.
- **Logger:** Use the existing `sessionLog` handle (`logging.ForComponent(logging.CompSession)` — already in use at `instance.go:4151-4156` for `session_data_build_resume` debug line). Do NOT introduce a new log component.
- **Level:** `Info` (OBS-02 is a production-visible observability line; debug would be filtered out).
- **Exact grep substrings — pick exactly one per Start() / Restart() / StartWithMessage call:**
  - `resume: id=<uuid> reason=conversation_data_present` — emitted when `useResume == true` in `buildClaudeResumeCommand()`.
  - `resume: id=<uuid> reason=session_id_flag_no_jsonl` — emitted when `useResume == false` in `buildClaudeResumeCommand()` (stored ID exists but no JSONL found).
  - `resume: none reason=fresh_session` — emitted by the fresh-Claude branch (empty ClaudeSessionID, Start routes to `buildClaudeCommand`).
  - `resume: none reason=tool_not_claude` — emitted by non-Claude tool branches (gemini/opencode/codex/custom/raw) if they reach a start dispatch with non-empty ID (optional — implementer may skip; OBS-02 text only requires "on every Claude start").
- **slog attributes (required for structured parse):**
  ```go
  sessionLog.Info("resume: id="+i.ClaudeSessionID+" reason=conversation_data_present",
      slog.String("instance_id", i.ID),
      slog.String("claude_session_id", i.ClaudeSessionID),
      slog.String("path", i.ProjectPath),
      slog.String("reason", "conversation_data_present"))
  ```
  The message prefix `resume: ` is the grep-stable contract; the slog attrs are ergonomic extras.
- **Acceptance:** After any `agent-deck session start` or `agent-deck session restart` against a Claude instance, `grep 'resume:' ~/.agent-deck/debug.log*` returns at least one row. Use `debug.log*` (glob) to cover rotation.
  - **File path caveat:** The spec text says `~/.agent-deck/logs/*.log` but the actual sink wired up by `logging.Init` at `cmd/agent-deck/main.go:394-437` is `~/.agent-deck/debug.log` (single file + lumberjack rotation). Phase 2 honored this in its own tests (`cgroupIsolationLog` writes to `debug.log`). Phase 3 inherits the same convention. Acceptance criteria in this phase use `debug.log*`; the spec-text grep target is treated as a symbolic synonym.
- **Exactly-once per call:** Each Start / Restart / StartWithMessage invocation MUST produce exactly one `resume: ` line. Do NOT add `sync.Once` — that would suppress subsequent calls. Just emit inline. Tests will count lines.

### 3. Preserve ClaudeSessionID through stop / error (PERSIST-08)

- **Current state:** `ClaudeSessionID` is already preserved through `Stop` / `Kill` / status transitions to `StatusStopped` or `StatusError`. There is no existing code path that clears it on those transitions. The bug that caused the 2026-04-14 divergence was NOT an explicit clear — it was the implicit overwrite at `instance.go:567` in `buildClaudeCommand`. Once Decision 1 lands, that overwrite stops firing for instances with stored IDs.
- **Required:** Add an explicit regression test (`TestPersistence_ClaudeSessionIDPreservedThroughStopError` — new test, not one of the eight mandated) that:
  1. Sets `inst.ClaudeSessionID = "<uuid>"` and `inst.Status = StatusRunning`.
  2. Transitions to `StatusStopped` (via `Kill` / synthetic status set).
  3. Asserts `inst.ClaudeSessionID` still equals `<uuid>`.
  4. Transitions to `StatusError` (synthetic SIGKILL + status tick).
  5. Asserts `inst.ClaudeSessionID` still equals `<uuid>`.
  6. Calls `inst.Start()` with a JSONL transcript written.
  7. Asserts `inst.ClaudeSessionID` still equals `<uuid>` AFTER Start returns.
- **Rationale:** Prevent a future refactor from accidentally clearing the ID on error-recovery paths. This test is strictly additive — it is NOT one of the eight mandated tests (those are in `CLAUDE.md`), but it lives in the same file `internal/session/session_persistence_test.go` to stay discoverable.
- **Forbidden:** Do NOT add any `i.ClaudeSessionID = ""` in Start / Restart / error-recovery paths. The only legitimate clears are in `Fork` (target instance only, not source) and `Delete` (removes the whole instance).

### 4. Read authoritatively from instance JSON (PERSIST-09)

- **Current state:** `i.ClaudeSessionID` is the Go struct field; it is populated from storage JSON by `storage.Load()` at `storage.go:405+` and persisted back by `storage.Save()` at `storage.go:247+`. The hook sidecar at `~/.agent-deck/hooks/<instance>.sid` is a READ-ONLY fallback used only inside `updateSessionIDFromHook` at `instance.go:2626+` when hook payloads omit a session ID. Start() / Restart() DO NOT read the sidecar — they read `i.ClaudeSessionID` directly. TEST-07 in Phase 1 asserts this.
- **Required:** No code change beyond Decision 1. The fix in Decision 1 already satisfies PERSIST-09 because `buildClaudeResumeCommand()` reads `i.ClaudeSessionID` from the struct. TEST-07 goes GREEN as a byproduct of TEST-06 going GREEN.
- **Rationale:** The spec's PERSIST-09 exists to prevent a future refactor from making the sidecar authoritative. This decision locks the current pattern.

### 5. Divergence behavior when JSONL is missing

- **Scenario (from user-reported conductor incident):** Stored `ClaudeSessionID = f1e103df` but `~/.claude/projects/<hash>/f1e103df-...-.jsonl` does not exist. Claude was writing to a DIFFERENT UUID (`b9403638`) that was never captured in agent-deck storage.
- **Current `buildClaudeResumeCommand` behavior (correct):** Line 4150 calls `sessionHasConversationData(i.ClaudeSessionID, i.ProjectPath)`. If false, line 4176 produces `claude --session-id f1e103df` (stored id, NO mint). Claude then either uses that id as its next session or — if `f1e103df` already exists as a (never-used) registered session — picks it up. Either way, the stored id stays authoritative and no overwrite occurs.
- **Phase 3 contract:** Do NOT attempt to "detect" the real running UUID by scanning `~/.claude/projects/<hash>/*.jsonl` for recency. That would violate PERSIST-10 / `docs/session-id-lifecycle.md` ("disk scans are non-authoritative for identity binding"). The trade-off is: if the user's conductor session already diverged BEFORE Phase 3 lands, Phase 3 cannot retroactively recover the `b9403638` conversation. It CAN guarantee this never happens again for sessions started/restarted after the fix.
- **Required test (`TestPersistence_SessionIDFallbackWhenJSONLMissing` — new):**
  1. Set `inst.ClaudeSessionID = "deadbeef-fake-uuid"`.
  2. Do NOT write any JSONL under `~/.claude/projects/<hash>/`.
  3. Call `inst.Start()`.
  4. Assert captured argv contains `--session-id deadbeef-fake-uuid` AND does NOT contain `--resume`.
  5. Assert `inst.ClaudeSessionID == "deadbeef-fake-uuid"` after Start returns (no overwrite).
  6. Assert `resume: id=deadbeef-fake-uuid reason=session_id_flag_no_jsonl` appears in captured logs.

### 6. Update docs/session-id-lifecycle.md (PERSIST-10)

- **Add subsection** titled `## Start / Restart Dispatch` with exactly these invariants (copy verbatim into the doc):
  1. `Instance.ClaudeSessionID` in instance JSON storage is the sole authoritative source for the Claude session UUID bound to an agent-deck instance.
  2. `Start()`, `StartWithMessage()`, and `Restart()` route through `buildClaudeResumeCommand()` whenever `ClaudeSessionID != ""`. They never mint a new UUID for an instance that already has one.
  3. `buildClaudeResumeCommand()` uses `claude --resume <id>` when JSONL evidence exists and `claude --session-id <id>` otherwise. In both cases `<id>` is the stored `ClaudeSessionID` — never a newly minted UUID.
  4. Disk scans of `~/.claude/projects/<hash>/*.jsonl` remain non-authoritative. `sessionHasConversationData()` is a presence check, NOT an identity probe.
- **Keep existing invariants section intact** — this is additive.

### 7. Files to modify (authoritative list)

- `internal/session/instance.go` — two switch-arm edits (Start, StartWithMessage) + log-line emissions in `buildClaudeResumeCommand` and adjacent fresh-session path. Bulk of the production diff.
- `internal/session/session_persistence_test.go` — two new tests (`ClaudeSessionIDPreservedThroughStopError`, `SessionIDFallbackWhenJSONLMissing`). The existing four RED tests are NOT modified — their assertions are the contract.
- `docs/session-id-lifecycle.md` — additive subsection per Decision 6.

### 8. Files NOT to modify

- `internal/session/storage.go` — ClaudeSessionID persistence is already correct.
- `internal/session/userconfig.go` — Phase 2 territory.
- `internal/tmux/tmux.go` — Phase 2 territory.
- `cmd/agent-deck/session_cmd.go` — Start / Restart handlers already call the right instance methods. No changes needed unless the planner discovers a command-level clear that needs removal.
- Any of the eight mandated test names or bodies.

### 9. Test-driven discipline

- Per `CLAUDE.md` mandate and global rules: each new test lands in its own commit BEFORE the production code that turns it green. Suggested commit sequence:
  1. `test(03): add TestPersistence_ClaudeSessionIDPreservedThroughStopError (RED on current code? likely PASSing — regression guard)`
  2. `test(03): add TestPersistence_SessionIDFallbackWhenJSONLMissing (RED — Start bypass triggers overwrite)`
  3. `feat(03): route Start/StartWithMessage through buildClaudeResumeCommand when ClaudeSessionID non-empty`
  4. `feat(03): emit OBS-02 resume log line from buildClaudeResumeCommand and fresh-session path`
  5. `docs(03): add Start/Restart Dispatch subsection to session-id-lifecycle.md`
- Run the full `go test -run TestPersistence_ ./internal/session/... -race -count=1` suite after EACH commit. The transition TEST-05/06/07/08 RED→GREEN MUST happen in commit 3. TEST-01..04 MUST stay GREEN across all five commits.

### 10. Commit hygiene

- Each commit signed `Committed by Ashesh Goplani` (per user global rule).
- NO `Co-Authored-By: Claude` / `🤖 Generated with Claude Code` lines.
- Atomic commits per Decision 9 sequence.
- `.planning/` is in `.git/info/exclude` — use `git add -f .planning/<file>` for planning artifacts.
- No `git push`, `git tag`, `gh pr create`, or `gh release` without explicit user approval. Phase 3 ends at verification — the conductor drives `/gsd:execute-phase 3` next.

### 11. Claude's discretion (planner picks reasonable defaults)

- Whether to factor the Claude-compatible dispatch switch arm into a helper like `i.selectClaudeCommand(baseCommand string) string` that both Start and StartWithMessage delegate to. Cleaner but two extra indirection points; fine either way.
- Whether the StartWithMessage initial-message delivery uses the existing `buildClaudeCommandWithMessage(baseCommand, message)` path (current behavior, message embedded in shell command) or a post-start PTY send. Preference: minimal diff. If `buildClaudeResumeCommand` cannot cleanly embed a message today, use post-start send.
- Whether to emit the optional `resume: none reason=tool_not_claude` log line for non-Claude tools. Not required by OBS-02; leave to planner judgment.
- Ordering of plans within the phase. Suggested wave structure: single sequential wave (test → test → fix → log → docs), each plan depending on the previous, since the diff is surgical and sequential commit auditing matters. No parallel waves needed — the phase is small.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Specification
- `docs/SESSION-PERSISTENCE-SPEC.md` — v1.5.2 spec. Phase 3 implements REQ-2 (lines 54-67), REQ-3 tests TEST-05..08 (lines 77-80), and OBS-02 (lines 109-113, second sentence).

### Roadmap & requirements
- `.planning/ROADMAP.md` — Phase 3 section is the single source of truth for goal + success criteria.
- `.planning/REQUIREMENTS.md` — PERSIST-07 through PERSIST-10 and OBS-02 / OBS-03 acceptance text. Every plan MUST list its REQ-IDs in frontmatter.

### Mandate
- `CLAUDE.md` (project root) — "Session persistence: mandatory test coverage" section. Lists the eight forbidden-to-remove tests and the paths under the mandate. Phase 3 modifies paths under the mandate (`internal/session/instance.go`), so the PR description for any downstream integration MUST include the full eight-test output.

### Phase 1 artifacts (the RED tests this phase must turn GREEN)
- `internal/session/session_persistence_test.go` — contains all eight `TestPersistence_*` functions. Phase 3 reads lines 749-762 (TEST-08), 791-832 (TEST-05), 851-873 (TEST-06), 894-935 (TEST-07). Also `newClaudeInstanceForDispatch`, `writeSyntheticJSONLTranscript`, `setupStubClaudeOnPATH`, `readCapturedClaudeArgv` — reuse these helpers; do not re-invent.
- `.planning/phases/01-persistence-test-scaffolding-red/03-PLAN.md` — the dispatch_path_analysis used by Phase 1 to write tests TEST-05..08. Has a diagram of the current call graph that Phase 3's fix mirrors.

### Phase 2 artifacts (to stay GREEN)
- `.planning/phases/02-cgroup-isolation-default-req-1-fix/02-SUMMARY.md` — confirms Phase 2's UAT pattern for log-file verification. Phase 3 reuses the `debug.log*` glob convention.
- `internal/session/userconfig.go:982-1035` — shows the `sessionLog` / `logging.ForComponent` pattern and the `sync.Once` one-shot emission used for OBS-01. Phase 3's OBS-02 emissions are per-call (NOT sync.Once'd), but the log component wiring is the same.

### Production code touched
- `internal/session/instance.go` —
  - `Start()` lines 1873-1986, switch arm at 1881-1901, Claude-compatible branch at 1883.
  - `StartWithMessage()` lines 1992-..., switch arm at 1999-2018, Claude-compatible branch at 2002.
  - `Restart()` respawn-pane branch at lines 3788-3816 (already correct — regression guard only).
  - `Restart()` recreate branch at lines 4018-4019 (already correct — regression guard only).
  - `buildClaudeCommand()` / `buildClaudeCommandWithMessage()` lines 477-597 — the fresh-session path. Read to understand line 566-567 UUID mint.
  - `buildClaudeResumeCommand()` lines 4111-4178 — the helper Phase 3 routes through. Read to understand line 4150 JSONL check and line 4172-4177 argv construction.
  - `sessionHasConversationData()` — JSONL presence helper; do NOT modify its contract.

### Verification harness
- `scripts/verify-session-persistence.sh` — must stay GREEN after Phase 3. Phase 3 does NOT need to modify the script, but its Scenario D (restart + resume) and Scenario E (SIGKILL + start) will now actually exercise the fix.

### Related docs
- `docs/session-id-lifecycle.md` — gets an additive subsection per Decision 6. Do NOT delete or reword the existing invariants.

</canonical_refs>

<specifics>
## Specific Ideas

- `setupStubClaudeOnPATH` in `session_persistence_test.go` writes captured argv to `AGENTDECK_TEST_ARGV_LOG`. Reuse this for the two new tests — do not add a new stub mechanism.
- The `newClaudeInstanceForDispatch` helper sets up the instance with a populated `ClaudeSessionID` and a minimal tmux-capable config. Both new tests should call this.
- `writeSyntheticJSONLTranscript` writes a JSONL under `~/.claude/projects/<hash>/`. For the `SessionIDFallbackWhenJSONLMissing` test, do NOT call this — the absence of the JSONL is the point. Just ensure the `~/.claude/projects/<hash>/` directory either does not exist or is empty for the instance's ClaudeSessionID.
- The `isolatedHomeDir(t)` helper sets `HOME` to a t.TempDir() so `~/.agent-deck/`, `~/.claude/`, and `~/.agent-deck/hooks/` are all under the test tree. Use it.
- To assert log-line presence in a test, redirect `sessionLog` to a bytes.Buffer via a test-only hook (pattern used in `userconfig_log_test.go` — the `cgroupIsolationLog` redirect). Copy that pattern rather than reading the real `debug.log`.
- `buildClaudeResumeCommand` at line 4151-4156 already emits a `session_data_build_resume` debug line. Phase 3 ADDS an Info-level `resume: ` line (different message prefix) — do not replace the debug line. Debug gives raw bool; Info gives grep-stable contract.
- When adding the dispatch conditional in `Start()`, consider the `IsClaudeCompatible` predicate already guards the arm — no need to re-check inside the conditional.
- The `StartWithMessage` code path at line 2002 currently calls `buildClaudeCommand(i.Command)` then later `buildClaudeCommandWithMessage(baseCommand, message)` via a different caller. Trace carefully before flipping — the planner should confirm whether the message is injected via the bash command string or via a post-start PTY send on the current code, and match that convention.
- TEST-05 (`RestartResumesConversation`) may PASS on current code (per comments at line 782-790 of the test file). Still required by the mandate — Phase 3's log-line addition must not break it.

</specifics>

<deferred>
## Deferred Ideas

- Retroactive recovery of the `b9403638` conversation on the user's existing conductor session — not in milestone scope.
- UI / TUI indicator for "resumable" status (`↻` glyph) — spec REQ-2 non-goal (P2 follow-up, not v1.5.2).
- Restructuring `instance.go` Start/StartWithMessage to share a dispatch helper beyond the Claude-compatible branch — out of scope; touches non-Claude tool branches needlessly.
- Adding a `ClaimConductorOwnership` or similar mechanism so the conductor can re-bind a divergent ID — out of scope; violates PERSIST-10 unless driven by tmux/hook evidence.
- Telemetry on `resume:` line counts (how often `session_id_flag_no_jsonl` fires) — no telemetry infra in agent-deck; not this phase.
- Changing `cgroupIsolationLog` to also handle the OBS-02 resume line — they are different observability concerns. Keep them separate.

</deferred>

---

*Phase: 03-resume-on-start-and-error-recovery-req-2-fix*
*Context gathered: 2026-04-14 from inline conductor instructions + spec REQ-2 + dispatch-path analysis of instance.go*
