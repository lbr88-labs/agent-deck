# Deferred Items — Phase 02

Issues discovered during Phase 02 execution that are out of scope for this
phase and logged for follow-up.

## 1. Pre-existing tmux-environment test failures (carried from Phase 01)

Same six `internal/session` failures documented in Phase 01's
`deferred-items.md`. Re-confirmed in Phase 02 against HEAD 38a2af3 and
against the parent commit — failures are identical with or without
Phase 02 changes applied. Not a Phase 02 regression.

- `TestSyncSessionIDsFromTmux_Claude`
- `TestSyncSessionIDsFromTmux_AllTools`
- `TestSyncSessionIDsFromTmux_OverwriteWithNew`
- `TestInstance_GetSessionIDFromTmux`
- `TestInstance_UpdateClaudeSession_TmuxFirst`
- `TestInstance_UpdateClaudeSession_RejectZombie`

## 2. New deferred: StatusEventWatcher fsnotify timeout failures

**Scope:** NOT caused by Phase 02 changes. Verified by `git stash` of
Phase 02 working changes on top of HEAD 38a2af3: the identical three
tests still fail with the same `timeout waiting for event delivery`
error.

**Failing tests (all in `package session`):**
- `TestStatusEventWatcher_DetectsNewFile` (`event_watcher_test.go:61`)
- `TestStatusEventWatcher_FilterByInstance` (`event_watcher_test.go:117`)
- `TestStatusEventWatcher_WaitForStatus` (`event_watcher_test.go:161`)

**Symptom:** fsnotify event delivery timeout. The harness writes a
status-event file and waits on a channel; no event arrives within the
2–5 second timeout.

**Hypothesis:** fsnotify behavior is sensitive to the filesystem driver
and container/worktree filesystem quirks. Possible causes include the
test TempDir being on an overlayfs that does not deliver inotify events
reliably, or lefthook-scrubbed env vars affecting a path that feeds
into fsnotify. The sibling test
`TestStatusEventWatcher_WaitForStatus_Timeout` (which exercises the
timeout path rather than the delivery path) passes, supporting the
"delivery-side only" hypothesis.

**Impact on Phase 02:**
- All 5 `TestPerGroupConfig_*` tests PASS under
  `go test -run TestPerGroupConfig_ ./internal/session/... -race -count=1`.
- PR #578 regression subset PASSES.
- `make ci` returns non-zero solely from these pre-existing failures
  (6 from Phase 01 + 3 new watcher tests documented here).

**Deferred to:** a separate investigation. Diagnosing the exact
fsnotify environment mismatch is out of scope for CFG-03 / CFG-04
test 4.

**Not a Phase 02 regression.** Recorded here per executor deviation
rules (SCOPE BOUNDARY — "Pre-existing failures in unrelated files are
out of scope. Log to deferred-items.md.").
