---
status: complete
phase: 03-resume-on-start-and-error-recovery-req-2-fix
source:
  - 03-01-SUMMARY.md
  - 03-02-SUMMARY.md
  - 03-03-SUMMARY.md
  - 03-04-SUMMARY.md
  - 03-05-SUMMARY.md
started: 2026-04-14T00:00:00Z
updated: 2026-04-14T00:00:00Z
verification_mode: automated
verification_command: "go test -run TestPersistence_ ./internal/session/... -race -count=1"
---

## Current Test

[testing complete]

## Tests

### 1. REQ-2 Production Fix — Start() routes through buildClaudeResumeCommand
expected: |
  internal/session/instance.go contains `if i.ClaudeSessionID != "" { command = i.buildClaudeResumeCommand() } else { ... }`
  inside both Start() and StartWithMessage() Claude-compatible switch arms, mirroring Restart()'s respawn branch.
result: pass
evidence: "instance.go:1893 and instance.go:2032 both gate on i.ClaudeSessionID != \"\""

### 2. TEST-05 RestartResumesConversation — Phase 2 baseline, must stay GREEN
expected: go test output shows PASS (flipped from RED by Plan 03-03 dispatch fix).
result: pass
evidence: "--- PASS: TestPersistence_RestartResumesConversation (1.01s)"

### 3. TEST-06 StartAfterSIGKILLResumesConversation — must flip RED→GREEN
expected: Start() on an instance with populated ClaudeSessionID after SIGKILL resumes rather than minting a new UUID.
result: pass
evidence: "--- PASS: TestPersistence_StartAfterSIGKILLResumesConversation (0.23s)"

### 4. TEST-07 ClaudeSessionIDSurvivesHookSidecarDeletion — invariant 1
expected: Deleting the hook sidecar file does not clear Instance.ClaudeSessionID (disk scans non-authoritative).
result: pass
evidence: "--- PASS: TestPersistence_ClaudeSessionIDSurvivesHookSidecarDeletion (0.24s)"

### 5. TEST-08 FreshSessionUsesSessionIDNotResume — must flip RED→GREEN
expected: First-ever start of a Claude session emits `--session-id <uuid>`, never `--resume`.
result: pass
evidence: "--- PASS: TestPersistence_FreshSessionUsesSessionIDNotResume (0.02s)"

### 6. TEST-03 LinuxDefaultIsUserScope — Phase 2 no-regression
expected: Stays GREEN post-Phase 3 (config default unchanged).
result: pass
evidence: "--- PASS: TestPersistence_LinuxDefaultIsUserScope (0.01s)"

### 7. TEST-04 MacOSDefaultIsDirect — Phase 2 no-regression
expected: Not applicable under systemd; TEST-03 covers the Linux default path.
result: skipped
reason: "Test self-skips when systemd-run is available; TEST-03 covers the Linux+systemd default. Documented behavior, not a regression."

### 8. Phase 3 Additive — ClaudeSessionIDPreservedThroughStopError (PERSIST-08 guard)
expected: |
  ClaudeSessionID survives StatusRunning → StatusStopped → StatusError and is not overwritten
  when Start() runs on an instance with a populated ID. Previously RED at Step 4.
result: pass
evidence: "--- PASS: TestPersistence_ClaudeSessionIDPreservedThroughStopError (0.24s)"

### 9. Phase 3 Additive — SessionIDFallbackWhenJSONLMissing (CONTEXT Decision 5)
expected: |
  When ClaudeSessionID is populated but no JSONL transcript exists, Start() emits
  `--session-id <stored-id>` (NOT --resume, NOT a fresh UUID). Previously RED at Assertion A.
result: pass
evidence: "--- PASS: TestPersistence_SessionIDFallbackWhenJSONLMissing (0.22s)"

### 10. OBS-02 — ResumeLogEmitted_ConversationDataPresent
expected: sessionLog.Info "resume: id=<id> reason=conversation_data_present" emitted when JSONL transcript present.
result: pass
evidence: "--- PASS: TestPersistence_ResumeLogEmitted_ConversationDataPresent (0.01s); emission at instance.go:4201"

### 11. OBS-02 — ResumeLogEmitted_SessionIDFlagNoJSONL
expected: sessionLog.Info "resume: id=<id> reason=session_id_flag_no_jsonl" emitted on the fallback path.
result: pass
evidence: "--- PASS: TestPersistence_ResumeLogEmitted_SessionIDFlagNoJSONL (0.02s); emission at instance.go:4207"

### 12. OBS-02 — ResumeLogEmitted_FreshSession
expected: sessionLog.Info "resume: none reason=fresh_session" emitted for brand-new sessions.
result: pass
evidence: "--- PASS: TestPersistence_ResumeLogEmitted_FreshSession (0.21s); emissions at instance.go:1896 and 2035"

### 13. PERSIST-10 — docs/session-id-lifecycle.md Start / Restart Dispatch subsection
expected: Additive H2 section appended after Event Log Schema, enumerating the four CONTEXT Decision 6 invariants plus Enforcement paragraph naming the six tests that pin them.
result: pass
evidence: "docs/session-id-lifecycle.md:47 — ## Start / Restart Dispatch"

### 14. STATE.md rolled forward — Phase 03 sign-off
expected: completed_phases = 3, completed_plans = 13, Current focus points at Phase 04, Last activity reports Phase 03 complete.
result: pass
evidence: ".planning/STATE.md — completed_phases: 3, completed_plans: 13, stopped_at 'Phase 03 fully landed. Next step: /gsd-plan-phase 4'"

### 15. TEST-01 TmuxSurvivesLoginSessionRemoval — host-harness gate
expected: PASS under the `scripts/verify-session-persistence.sh` harness on a clean Linux+systemd login shell (the only environment that can produce a valid fake-login scope MainPID).
result: blocked
blocked_by: harness
reason: |
  Run context here is a nested/transient scope (tmux spawn), so startAgentDeckTmuxInUserScope
  cannot read a valid MainPID for the fake-login scope — this is the pre-existing
  environmental failure documented in Plan 03-05 SUMMARY, NOT a Phase 3 regression.
  Requires `bash scripts/verify-session-persistence.sh` on a Linux+systemd login shell.
  Deferred to Phase 04 (verification-harness-docs-and-ci-wiring).

### 16. TEST-02 TmuxDiesWithoutUserScope — host-harness gate
expected: PASS under the `scripts/verify-session-persistence.sh` harness on a clean login shell.
result: blocked
blocked_by: harness
reason: |
  Test self-skips with an explicit message when the parent process is already inside
  a transient systemd scope: "tmux pid ... did not land in fake-login-... scope cgroup
  (...) — Run from a login shell or the verify-session-persistence.sh harness."
  This is documented, expected behavior — not a regression. Deferred to Phase 04.

## Summary

total: 16
passed: 13
issues: 0
pending: 0
skipped: 1
blocked: 2

## Gaps

[none — all Phase 3 verifier truths confirmed; TEST-01/TEST-02 are pre-existing harness-dependent gates whose closure is Phase 04's explicit scope per STATE.md "verification-harness-docs-and-ci-wiring"]
