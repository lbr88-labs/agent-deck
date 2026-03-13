# Phase 12: Session List & Resume UX - Research

**Researched:** 2026-03-13
**Domain:** Go TUI (Bubble Tea), session lifecycle, SQLite deduplication
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| VIS-01 | Stopped sessions appear in main TUI session list with distinct styling from error sessions | `rebuildFlatItems()` includes all sessions when `statusFilter == ""`; stopped sessions already have `SessionStatusStopped` lipgloss style (dim gray `■`); error sessions have `SessionStatusError` (red `✕`). The visual split already exists. |
| VIS-02 | Preview pane differentiates stopped (user-intentional) from error (crash) with distinct action guidance and resume affordance | `home.go:9871` conflates both statuses into one "Session Inactive" block with identical text. Stopped needs its own path: different header, intentional-stop messaging, resume keybinding hint. |
| VIS-03 | Session picker dialog correctly filters stopped sessions for conductor flows | `session_picker_dialog.go:41` already excludes `StatusStopped` from conductor picker. Preserve this. `getOtherActiveSessions()` at `home.go:10840` also correctly excludes stopped. Both must stay unchanged. |
| DEDUP-01 | Resuming a stopped session reuses the existing session record instead of creating a new duplicate entry | `Restart()` mutates the existing `Instance` in place. No new row is created by `Restart()`. The dedup gap is that `UpdateClaudeSessionsWithDedup` is not called immediately when the resumed session reacquires its `ClaudeSessionID` from tmux env (it only runs at next `SaveWithGroups`). |
| DEDUP-02 | `UpdateClaudeSessionsWithDedup` runs in-memory immediately at resume site, not only at persist time | Currently called only in `SaveWithGroups` (deferred to next persist). Must also be called in `sessionRestartedMsg` handler and/or at the point where `ClaudeSessionID` is re-acquired. |
| DEDUP-03 | Concurrent-write integration test covers two Storage instances against the same SQLite file | No such test exists. SQLite WAL mode is already configured in `statedb.Open()`; need a test that opens two `Storage` objects against the same path and verifies dedup survives a race. |
</phase_requirements>

---

## Summary

Phase 12 touches three tightly coupled concerns: (1) making stopped sessions visible with the right affordance in the preview pane, (2) ensuring deduplication runs in-memory at resume time, and (3) proving concurrent storage writes are safe.

The good news: most infrastructure is already in place. `StatusStopped` is defined, persisted, styled, and correctly filtered from the conductor picker. The list view already shows stopped sessions when no status filter is active. `UpdateClaudeSessionsWithDedup` already exists and runs correctly. The real work is three targeted fixes plus one new integration test.

The bad news: the preview pane at `home.go:9871` conflates stopped and error into one identical block. The dedup function is called in `SaveWithGroups` but not at the resume call site. And the concurrent-write test does not exist.

**Primary recommendation:** Three code changes plus one test. No new packages. No schema changes. Touches only `internal/ui/home.go` (two hunks), `internal/session/instance.go` (verify dedup call in status update path), and a new `internal/session/storage_concurrent_test.go`.

---

## Standard Stack

### Core (no changes needed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `charmbracelet/bubbletea` | v1.3.10 | TUI framework, message dispatch | Already in use; all UI changes live in `Update()` and `View()` |
| `charmbracelet/lipgloss` | v1.1.0 | Terminal styling | Already in use; stopped/error styles already defined |
| `modernc.org/sqlite` | v1.44.3 | SQLite storage (no CGO) | Already in use; WAL mode already configured |
| `stretchr/testify` | v1.11.1 | Test assertions | Already in use across all test packages |

### No New Packages Required

All phase 12 work uses existing dependencies. `go mod verify` before and after to confirm no drift.

---

## Architecture Patterns

### Recommended Project Structure (no changes)

```
internal/ui/
├── home.go               # Two-hunk change: preview pane differentiation
├── session_picker_dialog.go  # No change needed (already correct)
└── styles.go             # No change (stopped/error styles already distinct)

internal/session/
├── instance.go           # Possible: call UpdateClaudeSessionsWithDedup at status update time
└── storage_concurrent_test.go  # NEW: DEDUP-03 test
```

### Pattern 1: Preview Pane Status Differentiation

**What:** The preview pane at `home.go:9871` needs two distinct code paths: one for `StatusStopped` and one for `StatusError`.

**When to use:** Whenever `selected.Status == session.StatusStopped` (user-intentional stop) vs `selected.Status == session.StatusError` (crash/unexpected).

**Current code (to replace):**
```go
// home.go:9871 — BOTH statuses get identical text today
if selected.Status == session.StatusError || selected.Status == session.StatusStopped {
    errorHeader := renderSectionDivider("Session Inactive", width-4)
    // ... same message for both, including "tmux server was restarted" language
```

**Target structure:**
```go
// Stopped: user intentional — show resume affordance
if selected.Status == session.StatusStopped {
    header := renderSectionDivider("Session Stopped", width-4)
    // Message: "You stopped this session intentionally."
    // Action: R key = Resume   Enter = Attach (auto-start)   D = Delete
}

// Error: crash/unexpected — show crash guidance
if selected.Status == session.StatusError {
    header := renderSectionDivider("Session Error", width-4)
    // Message: "This can happen if: tmux server restarted / terminal closed"
    // Action: R key = Restart   D = Delete
}
```

**Key: use `h.actionKey(hotkeyRestart)` for the keybinding** — already used at line 9893. The `hotkeyRestart` constant maps to `R` by default but is user-configurable.

### Pattern 2: In-Memory Dedup at Resume Site

**What:** `UpdateClaudeSessionsWithDedup` must run in `h.instances` immediately when a session restarts, before the next `SaveWithGroups` call.

**Current flow:**
1. User presses `R` on a stopped Claude session.
2. `restartSession()` dispatches an async `tea.Cmd`.
3. `Restart()` calls `RespawnPane()` — the tmux pane is respawned.
4. `Restart()` sets `Status = StatusWaiting` on the existing instance.
5. `sessionRestartedMsg` arrives; `saveInstances()` is called.
6. `SaveWithGroups` calls `UpdateClaudeSessionsWithDedup(instances)` — this is the only current dedup site.

**The gap:** Between step 4 and step 6, the session's `ClaudeSessionID` may be re-detected from tmux env (via `UpdateClaudeSession()` in the background status loop). If another session already holds the same `ClaudeSessionID`, the duplicate isn't cleared until the next save completes.

**Fix location:** In the `sessionRestartedMsg` handler (at `home.go:3146`) where `saveInstances()` is called, add an in-memory dedup call before the save:

```go
case sessionRestartedMsg:
    if msg.err == nil {
        h.instancesMu.Lock()
        session.UpdateClaudeSessionsWithDedup(h.instances)  // <-- ADD THIS
        h.instancesMu.Unlock()
        // ... then saveInstances() as before
    }
```

This mirrors the pattern already used in `sessionCreatedMsg` handler at `home.go:2864` where `UpdateClaudeSessionsWithDedup` is called before the save.

### Pattern 3: Concurrent Storage Integration Test

**What:** Two `Storage` instances opened against the same SQLite file path, writing concurrently, with verification that `UpdateClaudeSessionsWithDedup` semantics survive.

**How:** SQLite WAL mode (already configured in `statedb.Open()`) allows concurrent readers + one writer. The test should:
1. Create a temp dir with a single `state.db`.
2. Open two `Storage` instances (`s1`, `s2`) against it.
3. Have both write sessions with the same `ClaudeSessionID` concurrently using `sync.WaitGroup`.
4. Load from a third `Storage` instance and assert only one session holds the ID.

**File:** `internal/session/storage_concurrent_test.go` — new file following the `newTestStorage(t)` pattern from `storage_test.go`.

**Test package:** `package session` (same as other storage tests). Must call `TestMain` in `testmain_test.go` to enforce `AGENTDECK_PROFILE=_test`.

### Anti-Patterns to Avoid

- **Modifying `session_picker_dialog.go:41`:** The `StatusStopped` exclusion there is CORRECT for conductor flows. VIS-03 is "preserve this behavior," not change it.
- **Calling `UpdateClaudeSessionsWithDedup` inside `Restart()`:** `Restart()` only has access to the single instance, not the full `[]*Instance` slice. The dedup function needs the full slice to detect cross-session conflicts. The call belongs in the UI layer (`home.go`), not in `Restart()`.
- **Adding a new `stopped` icon to `styles.go`:** Already defined (`SessionStatusStopped = dim gray "■"`). Error already uses `SessionStatusError = red "✕"`. No style changes needed.
- **Creating a `StatusResuming` status variant:** Unnecessary. `resumingSessions` map in `home.go` already tracks animation state. Status enum addition would require SQLite migration for zero benefit.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-session dedup logic | Custom "which session owns this ID" map | `session.UpdateClaudeSessionsWithDedup(h.instances)` | Oldest-wins semantics already tested, handles nil IDs, doesn't mutate order |
| Preview pane height padding | New padding helper | Existing pad-to-height pattern at lines 9911-9923 | Copy the exact pattern already used in the stopped/error block |
| SQLite concurrent test | Rolling your own WAL test | `newTestStorage(t)` + same file path for both | Test helper already handles dir setup and cleanup |
| Resume affordance keybinding | Hardcoded key strings | `h.actionKey(hotkeyRestart)` | User-configurable key lookup; already used at line 9893 |

**Key insight:** Every component needed already exists. The phase is three targeted changes, not new infrastructure.

---

## Common Pitfalls

### Pitfall 1: Conflating Filter Contexts for StatusStopped

**What goes wrong:** Developers see `StatusStopped` exclusion in `session_picker_dialog.go` and add the same filter to the main list or preview pane.

**Why it happens:** The exclusion pattern `if inst.Status == session.StatusError || inst.Status == session.StatusStopped { continue }` appears in TWO correct locations already — `session_picker_dialog.go:41` (conductor picker, correct) and `getOtherActiveSessions()` at `home.go:10840` (send-to-session picker, correct). Neither is the main list.

**How to avoid:** The main list `rebuildFlatItems()` already includes stopped sessions when `statusFilter == ""` (the default). The flat items include all statuses. Do NOT add a stopped-filter to `rebuildFlatItems()`. The preview pane path at line 9871 must be reached — it is only reached if the session is visible in the list.

**Warning signs:** If a stopped session disappears from the list after the change, search for a newly added `StatusStopped` exclusion.

### Pitfall 2: Dedup Called with Wrong Slice Scope

**What goes wrong:** Calling `UpdateClaudeSessionsWithDedup` on a single-element slice (just the restarted session), which defeats the dedup purpose.

**Why it happens:** The restart code path works with `current` (a single `*Instance`). It's tempting to pass `[]*Instance{current}` to the dedup function, but this never finds a duplicate because there's only one element.

**How to avoid:** Always call `UpdateClaudeSessionsWithDedup(h.instances)` — the full slice under `h.instancesMu` lock. This is the same pattern used in `sessionCreatedMsg`.

**Warning signs:** Test for dedup by checking that after resume, the original stopped session AND a new session with the same `ClaudeSessionID` are NOT both present. If both exist, dedup is running on wrong scope.

### Pitfall 3: Preview Pane Height Mismatch

**What goes wrong:** The stopped-session preview pane doesn't pad to the correct height, causing the layout to shift or truncate other panel content.

**Why it happens:** The current error/stopped block at line 9910-9923 manually pads to `height` lines. When splitting into two separate blocks (one for stopped, one for error), each must apply the same padding logic. If one block omits the padding, the preview pane height is inconsistent across states.

**How to avoid:** Copy the pad-to-height logic verbatim from the existing block for each new block. The pattern is:
```go
lines := strings.Split(content, "\n")
for i := len(lines); i < height; i++ {
    content += "\n"
}
if len(content) > 0 && content[len(content)-1] == '\n' {
    content = content[:len(content)-1]
}
```

### Pitfall 4: Concurrent Test Triggering Real Profile State

**What goes wrong:** The concurrent storage test accidentally uses the user's production `default` profile, corrupting real session data.

**Why it happens:** `newTestStorage(t)` creates a storage with `profile: "_test"` and a temp-dir DB path. If any test bypasses this helper and uses `NewStorageWithProfile("")`, it will use the default profile.

**How to avoid:** Always use `newTestStorage(t)` in test files. The `testmain_test.go` in `internal/session/` already sets `AGENTDECK_PROFILE=_test` as a guard.

---

## Code Examples

Verified patterns from direct codebase inspection:

### Dedup Call Pattern (from sessionCreatedMsg handler, home.go:2864)
```go
// home.go ~line 2863 — pattern to replicate in sessionRestartedMsg
h.instancesMu.Lock()
session.UpdateClaudeSessionsWithDedup(h.instances)
h.instancesMu.Unlock()
```

### Action Key Lookup (from existing stopped/error preview block, home.go:9893)
```go
// Already in the error/stopped block — use the same pattern for stopped-specific block
if restartKey := h.actionKey(hotkeyRestart); restartKey != "" {
    b.WriteString("  ")
    b.WriteString(keyStyle.Render(restartKey))
    b.WriteString(dimStyle.Render(" Resume  - restart with session resume"))
    b.WriteString("\n")
}
```

### Test Storage Helper (from storage_test.go:12)
```go
// Use this pattern for the concurrent test
func newTestStorage(t *testing.T) *Storage {
    t.Helper()
    tmpDir := t.TempDir()
    dbPath := filepath.Join(tmpDir, "state.db")
    db, err := statedb.Open(dbPath)
    // ...
    return &Storage{db: db, dbPath: dbPath, profile: "_test"}
}

// For the concurrent test: two storages against the SAME path
func TestConcurrentStorageWrites(t *testing.T) {
    tmpDir := t.TempDir()
    dbPath := filepath.Join(tmpDir, "state.db")

    openStorage := func() *Storage {
        db, err := statedb.Open(dbPath)
        require.NoError(t, err)
        require.NoError(t, db.Migrate())
        t.Cleanup(func() { db.Close() })
        return &Storage{db: db, dbPath: dbPath, profile: "_test"}
    }

    s1 := openStorage()
    s2 := openStorage()
    // ... concurrent writes with WaitGroup
}
```

### Session Status Differentiation — Full Switch Pattern (home.go:8650-8661)
```go
// Source: home.go rendering — icons are already defined consistently
case session.StatusError:
    statusIcon = "✕"
    statusStyle = SessionStatusError
case session.StatusStopped:
    statusIcon = "■"
    statusStyle = SessionStatusStopped  // dim gray, distinct from red error
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| sessions.json flat file | SQLite WAL via `modernc.org/sqlite` (no CGO) | v1.1 | Concurrent-write safe; enables StorageWatcher |
| `UpdateClaudeSessionsWithDedup` at persist time only | Already at `sessionCreatedMsg` time; gap is at `sessionRestartedMsg` | This phase | Closes the window where duplicate IDs can exist in memory |
| Stopped/error preview treated identically | Differentiated stopped (intentional) vs error (crash) | This phase | Users understand why session is inactive and what to do |

**Deprecated/outdated:**
- `sessions.json` storage: migrated away; only survives as `.migrated` backup. Do not write to it.
- Old group path `"My Sessions"` → `"my-sessions"` migration is already handled in `convertToInstances`. No action needed.

---

## Open Questions

1. **Does `Restart()` for a stopped Claude session actually reacquire `ClaudeSessionID` from tmux env during the restart call, or only on next status tick?**
   - What we know: `Restart()` calls `syncClaudeSessionFromDisk()` before the respawn, then calls `RespawnPane()`. The tmux env variable `CLAUDE_SESSION_ID` is set inside the spawned command (`--session-id "$session_id"`). The instance's `ClaudeSessionID` is NOT immediately updated by `Restart()` — it's updated later by `UpdateClaudeSession()` in the background status loop.
   - What's unclear: How quickly after `RespawnPane()` does the background loop set `ClaudeSessionID`? Could the dedup window be < 2 seconds (one tick)?
   - Recommendation: Call dedup in `sessionRestartedMsg` handler regardless. It's O(n) and cheap. Belt and suspenders.

2. **Is there a scenario where `Restart()` creates a new `Instance` row?**
   - What we know: `Restart()` never calls `NewInstanceWithTool()` or any `SaveWithGroups()` directly. It mutates the existing `Instance` pointer in place and returns an error or nil. The `sessionRestartedMsg` handler then calls `saveInstances()` which saves the modified existing instance.
   - Conclusion: DEDUP-01 ("reuses the existing session record") is already true at the `Restart()` level. The "duplicate entry" problem described in #224 likely happens when the user triggers session start from the CLI (which DOES create a new row) while a stopped session exists with the same `ClaudeSessionID`.
   - Recommendation: Verify the CLI `session start` path (`cmd/agent-deck/session_cmd.go`) also calls `UpdateClaudeSessionsWithDedup` before saving.

3. **Should the status filter key for stopped sessions be added (e.g., `%` to filter stopped-only)?**
   - What we know: Keys `!@#$` map to running/waiting/idle/error filters (`home.go:5140-5177`). Stopped has no filter key.
   - What's unclear: VIS-01 requires stopped sessions appear in main list — that's satisfied without a filter key. A filter key for stopped would be a bonus UX improvement.
   - Recommendation: Out of scope for this phase. The success criteria does not include a filter key. Skip.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | `go test` (stdlib testing + `stretchr/testify` v1.11.1) |
| Config file | none — standard Go test runner |
| Quick run command | `go test -race -v ./internal/session/... -run TestConcurrent` |
| Full suite command | `make test` (runs `go test -race -v ./...`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| VIS-01 | Stopped session appears in flat items list when no status filter | unit | `go test -race -v ./internal/ui/... -run TestRebuildFlatItems_StoppedSessionVisible` | ❌ Wave 0 |
| VIS-02 | Preview pane for stopped session shows distinct "Session Stopped" header with resume hint | unit | `go test -race -v ./internal/ui/... -run TestPreviewPane_StoppedVsError` | ❌ Wave 0 |
| VIS-03 | Session picker dialog excludes stopped sessions (existing behavior preserved) | unit | `go test -race -v ./internal/ui/... -run TestSessionPickerDialog_FiltersStopped` | ✅ `session_picker_dialog_test.go:72` |
| DEDUP-01 | Restart() mutates existing instance, not creates new row | unit | `go test -race -v ./internal/session/... -run TestRestart_ReusesExistingRecord` | ❌ Wave 0 |
| DEDUP-02 | UpdateClaudeSessionsWithDedup runs before saveInstances in sessionRestartedMsg path | unit | `go test -race -v ./internal/session/... -run TestUpdateClaudeSessionsWithDedup_DoesNotReorderInput` | ✅ `instance_test.go:1188` (partial; needs UI-layer test) |
| DEDUP-03 | Two Storage instances writing same SQLite file concurrently pass race detector | integration | `go test -race -v ./internal/session/... -run TestConcurrentStorageWrites` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race -v ./internal/session/... ./internal/ui/...`
- **Per wave merge:** `make test`
- **Phase gate:** `make test` + `make ci` green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/ui/home_flatitems_test.go` — covers VIS-01 (`TestRebuildFlatItems_StoppedSessionVisible`)
- [ ] `internal/ui/home_preview_test.go` or extension of existing UI tests — covers VIS-02 (`TestPreviewPane_StoppedVsError`)
- [ ] `internal/session/storage_concurrent_test.go` — covers DEDUP-03 (`TestConcurrentStorageWrites`)
- [ ] Optional: `internal/session/lifecycle_test.go` extension — covers DEDUP-01 (`TestRestart_ReusesExistingRecord`)

---

## Sources

### Primary (HIGH confidence)

- Direct codebase read: `internal/ui/home.go` — lines 9870-9926 (preview pane), 2836-2898 (sessionCreatedMsg dedup call), 3146-3166 (sessionRestartedMsg handler), 5023-5041 (R key handler), 10832-10844 (getOtherActiveSessions), 1028-1072 (rebuildFlatItems)
- Direct codebase read: `internal/ui/session_picker_dialog.go` — lines 30-46 (Show filter), confirmed `StatusStopped` correctly excluded at line 41
- Direct codebase read: `internal/ui/styles.go` — lines 490-494 (`SessionStatusStopped = dim gray`, `SessionStatusError = red`)
- Direct codebase read: `internal/session/instance.go` — lines 3505-3724 (`Restart()` full implementation), 4948-4980 (`UpdateClaudeSessionsWithDedup`)
- Direct codebase read: `internal/session/storage.go` — lines 246-332 (`SaveWithGroups` calling `UpdateClaudeSessionsWithDedup`)
- Direct codebase read: `internal/session/storage_test.go` — `newTestStorage` helper pattern
- Direct codebase read: `internal/session/lifecycle_test.go` — `TestSessionStop_KillsAndSetsStopped` pattern
- Direct codebase read: `go.mod` — confirmed all deps and versions
- Project planning docs: `.planning/STATE.md`, `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md` — phase notes, requirements
- Prior research: `.planning/research/FEATURES.md`, `PITFALLS.md`, `STACK.md`, `SUMMARY.md` — v1.3 findings

### Secondary (MEDIUM confidence)

- `.planning/research/FEATURES.md:128-129` — "TUI list filter hides stopped same as error" / "Not called at resume-creation path" — prior research confirmed in code

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new packages; all existing
- Architecture: HIGH — all touch points verified directly in source
- Pitfalls: HIGH — confirmed by reading the exact filter locations and the dedup call sites

**Research date:** 2026-03-13
**Valid until:** 2026-04-13 (stable codebase; no fast-moving external deps)
