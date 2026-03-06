# Phase 4: Framework Foundation - Research

**Researched:** 2026-03-06
**Domain:** Go integration test infrastructure for tmux-based session management
**Confidence:** HIGH

## Summary

Phase 4 builds the shared test infrastructure (TmuxHarness, polling helpers, SQLite fixture helpers) that all subsequent integration test phases depend on, plus proves it works by testing session lifecycle (start, stop, fork, restart). The codebase already has substantial unit test coverage (70+ test files) and established patterns: `TestMain` with `AGENTDECK_PROFILE=_test`, `skipIfNoTmuxServer(t)`, `t.Cleanup` for teardown, and `newTestStorage(t)` for temp-dir SQLite. The new integration package will formalize these ad-hoc patterns into reusable helpers.

The codebase uses Go 1.24, testify v1.11, modernc.org/sqlite (pure Go, no CGO), and tmux as its session backend. All existing test packages that touch tmux already create sessions with `NewInstance()`, start them with `inst.Start()`, and clean up with `defer inst.Kill()`. The integration framework must wrap this existing API rather than replace it, adding lifecycle management (automatic cleanup), polling (replace `time.Sleep`), and SQLite fixture seeding.

**Primary recommendation:** Create a new `internal/integration` package with `TmuxHarness`, `WaitForCondition`, and `TestStorageFactory` types, using only existing dependencies (Go stdlib + testify). Lifecycle tests go in the same package as consumers of the harness.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `testing` | 1.24 | Test framework | Built-in, no dependency |
| testify | v1.11.1 | Assertions (require/assert) | Already used in 50+ test files |
| modernc.org/sqlite | v1.44.3 | Pure-Go SQLite | Already the production database driver |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/statedb` | project | SQLite wrapper with Migrate() | All fixture helpers |
| `internal/tmux` | project | Session create/kill/capture | TmuxHarness wraps this |
| `internal/session` | project | Instance lifecycle (Start/Kill/Fork) | Lifecycle tests |
| `internal/testutil` | project | Git env cleanup | TestMain setup |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom polling | `github.com/cenkalti/backoff` | Adds dependency for simple retry; stdlib `time.Ticker` + `context` sufficient |
| Test containers | `testcontainers-go` | tmux-in-Docker is fragile; out of scope per REQUIREMENTS.md |
| `go test -tags integration` | Build tags | Adds build complexity; `skipIfNoTmuxServer(t)` pattern already works and is established |

**Installation:**
No new dependencies needed. The existing `go.mod` has everything required.

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/
    harness.go          # TmuxHarness: session create/cleanup/naming
    poll.go             # WaitForCondition, WaitForPaneContent, WaitForStatus
    fixtures.go         # TestStorageFactory, InstanceBuilder, conductor fixtures
    testmain_test.go    # TestMain with AGENTDECK_PROFILE=_test + orphan cleanup
    lifecycle_test.go   # LIFE-01..04: start, stop, fork, restart tests
```

### Pattern 1: TmuxHarness (INFRA-01)
**What:** Wraps tmux session creation with automatic `t.Cleanup` teardown and unique naming.
**When to use:** Any integration test that needs real tmux sessions.
**Example:**
```go
// Source: Derived from existing patterns in lifecycle_test.go and tmux_test.go
type TmuxHarness struct {
    t        *testing.T
    sessions []*session.Instance // Track for cleanup
    prefix   string              // Test-unique prefix for session names
}

func NewTmuxHarness(t *testing.T) *TmuxHarness {
    t.Helper()
    skipIfNoTmuxServer(t)

    h := &TmuxHarness{
        t:      t,
        prefix: fmt.Sprintf("inttest_%s_", sanitize(t.Name())),
    }
    t.Cleanup(h.cleanup)
    return h
}

// CreateSession creates a session with auto-cleanup.
// Uses real session.NewInstance under the hood.
func (h *TmuxHarness) CreateSession(title, projectPath string) *session.Instance {
    h.t.Helper()
    inst := session.NewInstance(h.prefix+title, projectPath)
    h.sessions = append(h.sessions, inst)
    return inst
}

func (h *TmuxHarness) cleanup() {
    for _, inst := range h.sessions {
        if inst.Exists() {
            _ = inst.Kill()
        }
    }
}
```

### Pattern 2: Polling Helpers (INFRA-02)
**What:** Replace `time.Sleep(2 * time.Second)` with condition-based polling.
**When to use:** Any assertion that depends on async tmux state.
**Example:**
```go
// Source: Inspired by existing waitForAgentReady in session_cmd.go (line 1721)
// and StatusEventWatcher.WaitForStatus in event_watcher.go (line 141)
func WaitForCondition(t *testing.T, timeout time.Duration, poll time.Duration, desc string, condition func() bool) {
    t.Helper()
    deadline := time.After(timeout)
    ticker := time.NewTicker(poll)
    defer ticker.Stop()

    for {
        if condition() {
            return
        }
        select {
        case <-deadline:
            t.Fatalf("timed out after %v waiting for: %s", timeout, desc)
        case <-ticker.C:
            // continue polling
        }
    }
}

func WaitForPaneContent(t *testing.T, inst *session.Instance, contains string, timeout time.Duration) {
    t.Helper()
    WaitForCondition(t, timeout, 200*time.Millisecond,
        fmt.Sprintf("pane to contain %q", contains),
        func() bool {
            tmuxSess := inst.GetTmuxSession()
            if tmuxSess == nil {
                return false
            }
            content, err := tmuxSess.CapturePaneFresh()
            return err == nil && strings.Contains(content, contains)
        },
    )
}

func WaitForStatus(t *testing.T, inst *session.Instance, status session.Status, timeout time.Duration) {
    t.Helper()
    WaitForCondition(t, timeout, 200*time.Millisecond,
        fmt.Sprintf("status to become %s", status),
        func() bool {
            return inst.GetStatusThreadSafe() == status
        },
    )
}
```

### Pattern 3: SQLite Fixture Factory (INFRA-03)
**What:** Creates isolated Storage instances backed by temp-dir SQLite for test data seeding.
**When to use:** Tests that need pre-populated session data without real tmux sessions.
**Example:**
```go
// Source: Derived from existing newTestStorage() in storage_test.go (line 12)
// and newTestDB() in statedb_test.go (line 11)
type TestStorageFactory struct {
    t *testing.T
}

func NewTestStorageFactory(t *testing.T) *TestStorageFactory {
    return &TestStorageFactory{t: t}
}

func (f *TestStorageFactory) Create() *session.Storage {
    f.t.Helper()
    // Reuses the exact pattern from storage_test.go
    tmpDir := f.t.TempDir()
    dbPath := filepath.Join(tmpDir, "state.db")
    db, err := statedb.Open(dbPath)
    require.NoError(f.t, err, "failed to open test db")
    require.NoError(f.t, db.Migrate(), "failed to migrate test db")
    f.t.Cleanup(func() { db.Close() })
    // Return Storage struct directly (unexported fields accessible within same module)
    return &session.Storage{... } // See note below on access pattern
}

// InstanceBuilder provides a fluent API for creating test Instance records
type InstanceBuilder struct {
    data *statedb.InstanceRow
}

func NewInstanceBuilder(id, title string) *InstanceBuilder {
    return &InstanceBuilder{
        data: &statedb.InstanceRow{
            ID:          id,
            Title:       title,
            ProjectPath: "/tmp/test",
            GroupPath:   "test-group",
            Tool:        "shell",
            Status:      "idle",
            CreatedAt:   time.Now(),
            ToolData:    json.RawMessage("{}"),
        },
    }
}

func (b *InstanceBuilder) WithTool(tool string) *InstanceBuilder { b.data.Tool = tool; return b }
func (b *InstanceBuilder) WithStatus(s string) *InstanceBuilder { b.data.Status = s; return b }
func (b *InstanceBuilder) WithParent(id string) *InstanceBuilder { b.data.ParentSessionID = id; return b }
func (b *InstanceBuilder) Build() *statedb.InstanceRow { return b.data }
```

**IMPORTANT access pattern note:** The existing `newTestStorage()` in `internal/session/storage_test.go` constructs `Storage` directly since it's in the same package. The new `internal/integration` package cannot access unexported fields of `session.Storage`. Two approaches:
1. **Preferred:** Add a `session.NewStorageWithDB(db *statedb.StateDB, profile string) *Storage` constructor (minimal API surface addition).
2. **Alternative:** Put fixture helpers in a sub-package of session (e.g., `internal/session/testhelpers`), but this creates circular dependency risk.

### Pattern 4: TestMain Isolation (INFRA-04)
**What:** Forces `AGENTDECK_PROFILE=_test` and cleans up orphaned integration test sessions.
**When to use:** Required in every package that creates tmux sessions.
**Example:**
```go
// Source: Modeled on existing testmain_test.go files
// (internal/session/testmain_test.go, internal/tmux/testmain_test.go)
func TestMain(m *testing.M) {
    testutil.UnsetGitRepoEnv()
    os.Setenv("AGENTDECK_PROFILE", "_test")

    code := m.Run()

    // Kill integration test sessions (prefix-based, safe pattern)
    cleanupIntegrationSessions()
    os.Exit(code)
}

func cleanupIntegrationSessions() {
    out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
    if err != nil {
        return
    }
    for _, sess := range strings.Split(strings.TrimSpace(string(out)), "\n") {
        if strings.HasPrefix(sess, "agentdeck_inttest_") {
            _ = exec.Command("tmux", "kill-session", "-t", sess).Run()
        }
    }
}
```

### Anti-Patterns to Avoid
- **`time.Sleep` for assertions:** Every existing lifecycle test uses `time.Sleep(2 * time.Second)` before `UpdateStatus()`. New tests must use `WaitForCondition` instead.
- **Broad tmux cleanup patterns:** The 2026-01-20 incident (3GB RAM leak) was caused by broad `Contains("test")` cleanup. Use only the `agentdeck_inttest_` prefix for integration test cleanup.
- **Testing in the session package:** Existing lifecycle tests live in `internal/session/lifecycle_test.go`. The new integration tests should be in `internal/integration/` to prove the helpers work as external consumers. But session-package tests that use tmux directly should stay where they are.
- **Parallel tmux tests:** Out of scope per REQUIREMENTS.md ("tmux global namespace causes race conditions"). All integration tests MUST run serially.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Test assertions | Custom if/t.Fatal chains | `testify/assert` + `testify/require` | Already used everywhere; consistent error messages |
| SQLite test databases | Manual file management | `t.TempDir()` + `statedb.Open()` | Auto-cleaned by Go test framework |
| Tmux session uniqueness | Random names | `tmux.NewSession()` (adds 8-hex suffix) | Already handles uniqueness via crypto/rand |
| Session cleanup | Manual defer chains | `t.Cleanup()` via TmuxHarness | Runs even on t.Fatal, panic; composable |
| Polling with backoff | Custom retry loop | `WaitForCondition` helper | Consistent timeout messaging across all tests |

**Key insight:** The codebase already solves these problems in production code. The framework formalizes existing patterns (e.g., `waitForAgentReady` becomes `WaitForCondition`) rather than inventing new ones.

## Common Pitfalls

### Pitfall 1: Orphaned tmux Sessions
**What goes wrong:** Tests crash or `t.Fatal` before cleanup runs, leaving tmux sessions consuming RAM.
**Why it happens:** `defer inst.Kill()` only runs on function return, not on `t.Fatal` in subtests. Historical incident: 20+ orphaned sessions leaked 3GB RAM.
**How to avoid:** Use `t.Cleanup()` instead of `defer` for session teardown. `t.Cleanup` is called even when `t.Fatal` fires. The TmuxHarness pattern handles this automatically.
**Warning signs:** `tmux list-sessions | grep agentdeck_inttest` shows sessions after test suite completes.

### Pitfall 2: Profile Isolation Failure
**What goes wrong:** Tests write to production `~/.agent-deck/profiles/default/state.db`.
**Why it happens:** `AGENTDECK_PROFILE` not set early enough, or `NewStorageWithProfile("")` falls back to default.
**How to avoid:** TestMain sets `AGENTDECK_PROFILE=_test` before any test runs. Integration test storage uses `t.TempDir()` for databases, never the profile directory. Double-check by asserting `os.Getenv("AGENTDECK_PROFILE") == "_test"` in test setup.
**Warning signs:** Production session data changes after running tests (2025-12-11 incident: 36 sessions overwritten).

### Pitfall 3: Flaky Timing-Dependent Tests
**What goes wrong:** Tests pass locally but fail under load or on slower machines.
**Why it happens:** Hard-coded `time.Sleep` values (e.g., the existing 2s sleep before `UpdateStatus()`). tmux session creation is typically <100ms but can spike under load.
**How to avoid:** Use `WaitForCondition` with generous timeouts (5-10s) and short poll intervals (200ms). Never assert state immediately after `Start()` without polling.
**Warning signs:** Tests fail intermittently with "status was starting, expected running" type errors.

### Pitfall 4: StatusStarting Grace Period
**What goes wrong:** Tests check status immediately after `inst.Start()` and see `StatusStarting` instead of `StatusRunning`.
**Why it happens:** `Start()` sets status to `StatusStarting`. The grace period is 1.5s (`instance.go:2263`). `UpdateStatus()` returns early during this window if tmux session doesn't exist yet.
**How to avoid:** Use `WaitForStatus` helper that polls past the grace period. For tests that need `StatusRunning`, wait for the session to exist AND the grace period to expire.
**Warning signs:** Assertions fail with "got starting, want running" in tests that check status immediately.

### Pitfall 5: CapturePane Timing
**What goes wrong:** `CapturePane()` returns empty or stale content right after sending a command.
**Why it happens:** tmux has an internal rendering pipeline. Content appears in the pane asynchronously. The codebase has a `singleflight.Group` cache on CapturePane (500ms TTL).
**How to avoid:** Use `WaitForPaneContent` which polls `CapturePaneFresh()` (bypasses cache). Allow 1-3 seconds for simple echo commands to appear.
**Warning signs:** Tests see empty pane content or content from a previous command.

### Pitfall 6: Package Access to Unexported Fields
**What goes wrong:** `internal/integration` package cannot construct `session.Storage` directly because fields are unexported.
**Why it happens:** The existing `newTestStorage()` works only because it's in the same package (`internal/session`).
**How to avoid:** Either add a minimal exported constructor (`NewStorageWithDB`) or work through `statedb` directly (which has all exported types). The `InstanceBuilder` should produce `statedb.InstanceRow` values that can be saved via `db.SaveInstance()`.
**Warning signs:** Compilation error "cannot refer to unexported field 'db' in struct literal".

## Code Examples

Verified patterns from the existing codebase:

### Creating and Starting a Session (from lifecycle_test.go)
```go
// Source: internal/session/lifecycle_test.go:17-37
inst := session.NewInstance("test-start-creates", "/tmp")
inst.Command = "sleep 60"

err := inst.Start()
require.NoError(t, err, "Start() should succeed")
defer func() { _ = inst.Kill() }()

assert.True(t, inst.Exists(), "Exists() should return true after Start()")
tmuxSess := inst.GetTmuxSession()
require.NotNil(t, tmuxSess)
```

### Creating SQLite Test Fixture (from statedb_test.go)
```go
// Source: internal/statedb/statedb_test.go:11-23
func newTestDB(t *testing.T) *statedb.StateDB {
    t.Helper()
    dbPath := filepath.Join(t.TempDir(), "state.db")
    db, err := statedb.Open(dbPath)
    if err != nil {
        t.Fatalf("Open: %v", err)
    }
    if err := db.Migrate(); err != nil {
        t.Fatalf("Migrate: %v", err)
    }
    t.Cleanup(func() { db.Close() })
    return db
}
```

### Session Fork Command Verification (from fork_integration_test.go)
```go
// Source: internal/session/fork_integration_test.go:21-82
parent := session.NewInstanceWithTool("fork-parent", "/tmp", "claude")
parent.ClaudeSessionID = "abc-123-def"
parent.ClaudeDetectedAt = time.Now()

forked, cmd, err := parent.CreateForkedInstance("fork-child", "")
require.NoError(t, err)
assert.NotEqual(t, parent.ID, forked.ID)
assert.Equal(t, parent.ProjectPath, forked.ProjectPath)
```

### Saving Instance Data to SQLite (from statedb_test.go)
```go
// Source: internal/statedb/statedb_test.go:72-103
db.SaveInstances([]*statedb.InstanceRow{
    {
        ID: "a", Title: "Alpha", ProjectPath: "/a",
        GroupPath: "grp", Order: 0, Tool: "claude",
        Status: "idle", CreatedAt: time.Now(),
        ToolData: json.RawMessage(`{"claude_session_id":"abc"}`),
    },
})
loaded, _ := db.LoadInstances()
assert.Equal(t, "a", loaded[0].ID)
```

### Pane Capture for Content Verification
```go
// Source: internal/tmux/tmux.go:1698-1702
// CapturePaneFresh bypasses the singleflight cache
content, err := tmuxSess.CapturePaneFresh()
// Use for assertions where cache staleness matters
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `sessions.json` flat file | SQLite WAL mode (`state.db`) | v0.11.0 (2026-02-06) | All fixture helpers must use statedb, not JSON |
| `time.Sleep` in tests | Still used in lifecycle_test.go | Current (to be replaced in Phase 4) | Polling helpers replace these |
| `defer inst.Kill()` | `t.Cleanup()` via harness | Current (to be introduced in Phase 4) | Safer cleanup on Fatal/panic |
| Per-package skipIfNoTmuxServer | Duplicated in session/ and tmux/ | Current | Integration package centralizes this |

**Deprecated/outdated:**
- `sessions.json`: Replaced by SQLite in v0.11.0. Auto-migration still exists but tests should never use JSON.
- `Storage.Save()`: Marked DEPRECATED in storage.go:236. Use `SaveWithGroups()` instead.

## Open Questions

1. **Package placement for integration tests**
   - What we know: `internal/integration/` is the natural location. Tests will import `internal/session`, `internal/tmux`, and `internal/statedb`.
   - What's unclear: Should lifecycle tests (LIFE-01..04) live in `internal/integration/lifecycle_test.go` or remain in `internal/session/lifecycle_test.go`? The existing `lifecycle_test.go` already has Start/Stop/Fork tests.
   - Recommendation: New harness-based lifecycle tests go in `internal/integration/`. The existing `session/lifecycle_test.go` tests remain (they're valuable as package-level unit tests). The integration tests prove the harness works by exercising the same APIs through the harness layer.

2. **Accessing Storage internals from integration package**
   - What we know: `Storage` struct has unexported fields (`db`, `dbPath`, `profile`). Integration package can't construct it directly.
   - What's unclear: Whether to add a new exported constructor or to work through `statedb` directly.
   - Recommendation: Work through `statedb` directly in the fixture factory. Tests that need `Storage` can use `session.NewStorageWithProfile("_test")` which already exists but writes to the real profile directory. For truly isolated tests, create `statedb.StateDB` directly via `statedb.Open(t.TempDir() + "/state.db")` and use `statedb.InstanceRow` for seeding. This avoids adding new API surface.

3. **Restart with flags testing (LIFE-04)**
   - What we know: `Restart()` in instance.go has complex branching: Claude with known session ID uses `RespawnPane`, dead sessions get recreated. The `--yolo` flag is Gemini-specific (sets `GeminiYoloMode`).
   - What's unclear: What "restart with flags" means for non-Gemini tools. The existing `handleSessionRestart` in session_cmd.go only has a basic restart without tool-specific flags.
   - Recommendation: Test restart for shell sessions (simplest case: kill + recreate via Start). Test that the recreated session is functional (pane shows output). Skip Claude/Gemini-specific restart paths (they require real AI tool binaries).

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INFRA-01 | Shared TmuxHarness helper with t.Cleanup teardown | TmuxHarness pattern documented with code example. Uses existing `session.NewInstance` + `t.Cleanup`. Prefix-based naming (`agentdeck_inttest_`) for safe cleanup. |
| INFRA-02 | Polling helpers replace flaky time.Sleep | `WaitForCondition`, `WaitForPaneContent`, `WaitForStatus` documented. Modeled on existing `waitForAgentReady` (session_cmd.go:1721) and `WaitForStatus` (event_watcher.go:141). |
| INFRA-03 | SQLite fixture helpers | `TestStorageFactory` and `InstanceBuilder` documented. Use `statedb.Open(t.TempDir())` pattern from existing `statedb_test.go:11`. Builder produces `statedb.InstanceRow` for direct DB insertion. |
| INFRA-04 | TestMain with AGENTDECK_PROFILE=_test isolation | Modeled on 5 existing TestMain files. Adds `cleanupIntegrationSessions()` with `agentdeck_inttest_` prefix matching. Uses `testutil.UnsetGitRepoEnv()`. |
| LIFE-01 | Session start creates real tmux, transitions to running | Existing `TestSessionStart_CreatesTmuxSession` (lifecycle_test.go:17) proves the API works. Integration test wraps with TmuxHarness + WaitForStatus for robust assertion. |
| LIFE-02 | Session stop terminates tmux, updates status | Existing `TestSessionStop_KillsAndSetsError` (lifecycle_test.go:58) proves API. Integration test uses WaitForCondition to verify `Exists() == false`. |
| LIFE-03 | Session fork with parent-child linkage in SQLite | Existing `TestForkFlow_Integration` (fork_integration_test.go:12) tests command generation. Integration test must also verify `ParentSessionID` is set and both sessions are independent (kill parent, child survives). |
| LIFE-04 | Session restart with flags recreates correctly | `Restart()` (instance.go:3602) recreates dead sessions. For shell tool: Kill then Start. Integration test verifies new tmux session appears and runs command correctly. |
</phase_requirements>

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + testify v1.11.1 |
| Config file | None needed (go test discovers `*_test.go` automatically) |
| Quick run command | `go test -race -v -run TestIntegration ./internal/integration/...` |
| Full suite command | `go test -race -v ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INFRA-01 | TmuxHarness creates/cleans sessions | integration | `go test -race -v -run TestHarness ./internal/integration/... -x` | Wave 0 |
| INFRA-02 | WaitForCondition times out with message | unit | `go test -race -v -run TestWaitFor ./internal/integration/... -x` | Wave 0 |
| INFRA-03 | TestStorageFactory creates isolated DB | unit | `go test -race -v -run TestFixture ./internal/integration/... -x` | Wave 0 |
| INFRA-04 | TestMain sets _test profile | integration | `go test -race -v -run TestIsolation ./internal/integration/... -x` | Wave 0 |
| LIFE-01 | Start creates real tmux session | integration | `go test -race -v -run TestLifecycleStart ./internal/integration/... -x` | Wave 0 |
| LIFE-02 | Stop terminates tmux session | integration | `go test -race -v -run TestLifecycleStop ./internal/integration/... -x` | Wave 0 |
| LIFE-03 | Fork creates independent copy | integration | `go test -race -v -run TestLifecycleFork ./internal/integration/... -x` | Wave 0 |
| LIFE-04 | Restart recreates session | integration | `go test -race -v -run TestLifecycleRestart ./internal/integration/... -x` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race -v ./internal/integration/...`
- **Per wave merge:** `go test -race -v ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/integration/harness.go` -- TmuxHarness implementation
- [ ] `internal/integration/poll.go` -- WaitForCondition, WaitForPaneContent, WaitForStatus
- [ ] `internal/integration/fixtures.go` -- TestStorageFactory, InstanceBuilder
- [ ] `internal/integration/testmain_test.go` -- TestMain with profile isolation
- [ ] `internal/integration/lifecycle_test.go` -- LIFE-01 through LIFE-04

## Sources

### Primary (HIGH confidence)
- Project source code: `internal/session/lifecycle_test.go`, `internal/session/storage_test.go`, `internal/statedb/statedb_test.go` -- existing test patterns
- Project source code: `internal/session/instance.go` -- Start(), Kill(), Restart(), CreateForkedInstance() APIs
- Project source code: `internal/tmux/tmux.go` -- Session creation, CapturePane, cache patterns
- Project source code: `internal/session/testmain_test.go`, `internal/tmux/testmain_test.go` -- TestMain isolation patterns
- Project source code: `cmd/agent-deck/session_cmd.go` -- waitForAgentReady, waitForCompletion polling patterns

### Secondary (MEDIUM confidence)
- Project CLAUDE.md -- Architecture documentation, testing guidelines, data protection rules
- Project go.mod -- Dependency versions (testify v1.11.1, modernc.org/sqlite v1.44.3)

### Tertiary (LOW confidence)
- None. All findings are based on direct source code examination.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- All libraries already in go.mod; no new dependencies needed
- Architecture: HIGH -- Patterns directly derived from existing test code in the same codebase
- Pitfalls: HIGH -- Based on documented historical incidents (2025-12-11, 2026-01-20) in CLAUDE.md and code comments

**Research date:** 2026-03-06
**Valid until:** 2026-04-06 (stable; patterns are internal to this project)
