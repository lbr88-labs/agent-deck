# Phase 2: Testing & Bug Fixes - Research

**Researched:** 2026-03-06
**Domain:** Go testing for tmux session management, status lifecycle, and skills system
**Confidence:** HIGH

## Summary

Phase 2 focuses on verifying the agent-deck session lifecycle (start, stop, fork, attach), sleep/wake detection (status transitions between running, waiting, idle, and error), and skills triggering functionality through comprehensive tests, then fixing any bugs discovered during testing.

The codebase already has a mature test infrastructure with 1222 tests across all packages (509 in `internal/session`, 140 in `internal/tmux`). The testing framework uses Go's stdlib `testing` package with `github.com/stretchr/testify v1.11.1` for assertions. All test packages enforce profile isolation via `TestMain` functions that set `AGENTDECK_PROFILE=_test`, and integration tests requiring tmux use `skipIfNoTmuxServer(t)` to skip gracefully in CI.

The primary research finding is that most of the required test areas already have partial coverage. The gaps are: (1) no dedicated test for the complete status lifecycle flow (running to idle on inactivity, back to running on activity), (2) session start/stop tests exist at the unit level but lack end-to-end verification that SQLite persistence reflects tmux state accurately, (3) skills triggering tests exist for catalog operations (attach/detach/discover) but not for runtime triggering behavior (loading a pool skill into a running session context and verifying it functions), and (4) no systematic bug tracking during test execution.

**Primary recommendation:** Write targeted tests for the identified gaps rather than rewriting existing tests. Use table-driven test patterns consistent with the codebase. Integration tests requiring tmux must call `skipIfNoTmuxServer(t)` and use `defer inst.Kill()` for cleanup.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TEST-01 | Sleep/wake detection correctly transitions session status (running -> idle -> running on activity) | `UpdateStatus()` in instance.go (line 2251) handles all transitions via tmux activity timestamps and hook fast path. Existing `TestNewSessionStatusFlicker` tests partial flow. Gap: need full cycle test with simulated activity changes. |
| TEST-02 | Skills trigger correctly when referenced in session context or loaded on demand | `skills_catalog.go` provides `AttachSkillToProject`, `DetachSkillFromProject`, `ListAvailableSkills`, `ResolveSkillCandidate`, `ApplyProjectSkills`. Existing tests cover catalog ops but not runtime loading verification. |
| TEST-03 | Session start creates tmux session and transitions to running status | `Instance.Start()` (line 1745) creates tmux session, sets `StatusStarting`. `UpdateStatus()` transitions to running once tmux reports "active". `TestNewSessionStatusFlicker` partially covers this. |
| TEST-04 | Session stop cleanly terminates tmux session and updates status | `Instance.Kill()` (line 3560) kills tmux and sets `StatusError`. Need test verifying tmux session is gone and SQLite reflects the status. |
| TEST-05 | Session fork creates independent copy with correct instance ID propagation | `fork_integration_test.go` covers fork command structure. `instance_test.go` covers `CreateForkedInstance`. Gap: verify forked instance has independent tmux session. |
| TEST-06 | Session attach connects to existing tmux session without errors | `tmux.Session.Attach()` in `pty.go` (line 24) does `tmux attach-session`. This is interactive (requires PTY) so must be tested with care or verified at unit level. |
| TEST-07 | Session status tracking reflects actual tmux session state accurately | `UpdateStatus()` maps tmux states (active, waiting, idle, starting, inactive) to instance statuses. Existing tests in `tmux_test.go` cover `GetStatus` flow. Gap: end-to-end with SQLite persistence check. |
| STAB-01 | All bugs discovered during testing are fixed | Bug discovery happens during test execution. Research provides patterns for regression tests. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| testing (stdlib) | Go 1.24 | Test framework | Project standard, all 1222 existing tests use it |
| testify | v1.11.1 | Assertions (require, assert) | Already in go.mod, used in tmux and session tests |
| os/exec | stdlib | Running tmux subprocess commands in tests | Used by all integration tests for tmux verification |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| statedb (internal) | N/A | SQLite test database creation via `newTestDB(t)` | When verifying persistence (TEST-07) |
| testutil (internal) | N/A | Git env cleanup via `UnsetGitRepoEnv()` | Used in TestMain for session package |
| fsnotify | v1.9.0 | File system watching (used by StatusFileWatcher) | Relevant for hook_watcher tests |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| testify/assert | stdlib t.Errorf | testify already in deps, more expressive |
| Real tmux integration | Mock tmux.Session | Mocking loses confidence in real behavior; use skipIfNoTmuxServer |

**Installation:**
```bash
# No new dependencies needed - everything is already in go.mod
```

## Architecture Patterns

### Recommended Test Structure
```
internal/session/
├── instance_test.go          # Existing: unit + integration tests for Instance
├── session_test.go           # Existing: basic Instance creation tests
├── storage_test.go           # Existing: SQLite persistence tests
├── skills_catalog_test.go    # Existing: skills attach/detach/discover tests
├── hook_watcher_test.go      # Existing: hook status file processing tests
├── fork_integration_test.go  # Existing: fork flow integration tests
├── transition_notifier_test.go  # Existing: transition notification tests
├── transition_daemon.go      # Source: the daemon that drives status polling
└── testmain_test.go          # CRITICAL: profile isolation, cleanup

internal/tmux/
├── tmux_test.go              # Existing: session creation, status detection
├── status_fixes_test.go      # Existing: whimsical word detection, progress bar
├── patterns_test.go          # Existing: content pattern matching
└── testmain_test.go          # CRITICAL: profile isolation, cleanup
```

### Pattern 1: Table-Driven Tests with Tmux Skip
**What:** Table-driven tests with integration guard
**When to use:** All session lifecycle and status transition tests
**Example:**
```go
// Source: existing pattern from instance_test.go and hook_watcher_test.go
func TestStatusTransitionCycle(t *testing.T) {
    skipIfNoTmuxServer(t)

    tests := []struct {
        name       string
        command    string
        wantStart  Status
        wantAfter  Status
    }{
        {"shell session starts idle", "", StatusIdle, StatusIdle},
        {"command session starts starting", "echo hello", StatusStarting, StatusIdle},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            inst := NewInstance("test-"+tt.name, "/tmp")
            inst.Command = tt.command

            err := inst.Start()
            require.NoError(t, err)
            defer func() { _ = inst.Kill() }()

            assert.Equal(t, tt.wantStart, inst.Status)

            // Wait for status to settle
            time.Sleep(2 * time.Second)
            require.NoError(t, inst.UpdateStatus())
            assert.Equal(t, tt.wantAfter, inst.GetStatusThreadSafe())
        })
    }
}
```

### Pattern 2: Storage Round-Trip Verification
**What:** Verify that status changes persist to SQLite correctly
**When to use:** TEST-07 (status tracking reflects actual state)
**Example:**
```go
// Source: existing pattern from storage_test.go
func TestStatusPersistence(t *testing.T) {
    storage := newTestStorage(t)

    inst := &Instance{
        ID:          "test-persist",
        Title:       "Test Persistence",
        ProjectPath: "/tmp",
        Tool:        "shell",
        Status:      StatusRunning,
        CreatedAt:   time.Now(),
    }

    err := storage.Save([]*Instance{inst})
    require.NoError(t, err)

    loaded, err := storage.Load()
    require.NoError(t, err)
    require.Len(t, loaded, 1)
    assert.Equal(t, StatusRunning, loaded[0].Status)
}
```

### Pattern 3: Hook Status Simulation
**What:** Simulate hook status file writes to test status fast-path
**When to use:** TEST-01 (sleep/wake detection) with hook-based transitions
**Example:**
```go
// Source: existing pattern from hook_watcher_test.go
func TestHookDrivenStatusTransition(t *testing.T) {
    tmpDir := t.TempDir()
    hooksDir := filepath.Join(tmpDir, "hooks")
    _ = os.MkdirAll(hooksDir, 0755)

    w := &StatusFileWatcher{
        hooksDir: hooksDir,
        statuses: make(map[string]*HookStatus),
    }

    // Simulate running -> waiting transition via hook
    writeHookStatus(t, hooksDir, "inst-1", "running", "UserPromptSubmit")
    w.processFile(filepath.Join(hooksDir, "inst-1.json"))
    require.Equal(t, "running", w.GetHookStatus("inst-1").Status)

    writeHookStatus(t, hooksDir, "inst-1", "waiting", "Stop")
    w.processFile(filepath.Join(hooksDir, "inst-1.json"))
    require.Equal(t, "waiting", w.GetHookStatus("inst-1").Status)
}
```

### Anti-Patterns to Avoid
- **Creating tmux sessions without defer Kill():** Orphaned sessions waste RAM. Historical incident: 20+ orphaned sessions leaked 3GB.
- **Using broad session name patterns in cleanup:** Must NOT match user sessions. Only match specific test artifact names.
- **Running tests without AGENTDECK_PROFILE=_test:** Overwrote 36 production sessions in 2025-12-11 incident.
- **Skipping t.Helper() in test helpers:** Makes error messages point to wrong line.
- **Testing Attach() in automated tests:** It is interactive (requires PTY). Verify at unit level that the correct tmux command is constructed, or use a smoke test that attaches and immediately detaches.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Test SQLite databases | Manual file management | `newTestStorage(t)` from storage_test.go | Handles temp dir, migration, cleanup automatically |
| Test profile isolation | Ad-hoc env var setting | Existing TestMain pattern with `AGENTDECK_PROFILE=_test` | Prevents production data corruption (documented incidents) |
| Session cleanup | Custom goroutine cleanup | `defer func() { _ = inst.Kill() }()` | Runs on panic, Fatal, etc. Reliable. |
| Hook status files | Manual JSON construction | `json.Marshal` with struct like existing hook_watcher_test.go | Consistent format, avoids typos |
| Tmux availability check | Custom binary check | `skipIfNoTmuxServer(t)` | Checks both binary AND running server |

**Key insight:** The test infrastructure is already well-established. The primary work is writing new test cases using existing patterns, not building new test infrastructure.

## Common Pitfalls

### Pitfall 1: Tmux Session Naming Collisions
**What goes wrong:** Two tests create sessions with the same name, causing one to fail or interfere.
**Why it happens:** `NewSession` generates unique suffixes, but `NewInstance` without Start() does not create a tmux session. If test code manually creates tmux sessions, collisions can occur.
**How to avoid:** Always use `NewInstance("unique-name", "/tmp")` followed by `inst.Start()` and `defer inst.Kill()`. The tmux session gets a random 8-hex-char suffix.
**Warning signs:** Flaky tests that pass individually but fail when run together.

### Pitfall 2: Status Detection Timing
**What goes wrong:** Tests check status immediately after Start() and get StatusStarting instead of expected status.
**Why it happens:** There is a 1.5-second grace period for tmux initialization (line 2263 in instance.go). Status detection requires at least one `UpdateStatus()` call after the grace period.
**How to avoid:** After `inst.Start()`, wait at least 2 seconds, then call `inst.UpdateStatus()` before checking status. For shell sessions, expect `StatusIdle` (not StatusRunning) since shell without active process is idle.
**Warning signs:** Tests that use `time.Sleep(100*time.Millisecond)` before status checks.

### Pitfall 3: Hook Fast Path vs Tmux Polling
**What goes wrong:** Tests assume status comes from tmux polling, but hook fast path takes precedence for Claude-compatible tools.
**Why it happens:** The `UpdateStatus()` method has a "HOOK FAST PATH" (line 2309) that uses hook status files when they are fresh (within `hookFastPathWindow` of 2 minutes). For shell/generic sessions, tmux polling is used instead.
**How to avoid:** When testing hook-based status, explicitly set up hook status files. When testing tmux-based status, use `Tool: "shell"` to avoid the hook fast path.
**Warning signs:** Status tests that pass for "shell" tool but fail for "claude" tool.

### Pitfall 4: Forgetting to Clear UserConfig Cache
**What goes wrong:** Tests that modify HOME or CLAUDE_CONFIG_DIR get stale config data.
**Why it happens:** `UserConfig` is cached globally. Tests that change env vars must call `ClearUserConfigCache()`.
**How to avoid:** Always call `ClearUserConfigCache()` after setting HOME or CLAUDE_CONFIG_DIR. Follow the established pattern from `instance_test.go`.
**Warning signs:** Tests that pass alone but fail when run after other tests.

### Pitfall 5: Skills Test Environment Isolation
**What goes wrong:** Skills tests read from user's actual `~/.agent-deck/skills/` directory.
**Why it happens:** Skills catalog functions resolve paths relative to HOME.
**How to avoid:** Use `setupSkillTestEnv(t)` from skills_catalog_test.go which creates temp HOME and CLAUDE_CONFIG_DIR.
**Warning signs:** Tests that pass on one machine but fail on another.

## Code Examples

Verified patterns from existing test files in the codebase:

### Creating a Test Instance with Tmux (Integration)
```go
// Source: internal/session/instance_test.go:TestNewSessionStatusFlicker
func TestSessionLifecycle(t *testing.T) {
    skipIfNoTmuxServer(t)

    inst := NewInstance("lifecycle-test", "/tmp")
    inst.Command = "echo hello"

    // Start
    err := inst.Start()
    require.NoError(t, err)
    defer func() { _ = inst.Kill() }()

    // Verify tmux session exists
    assert.True(t, inst.Exists())

    // Wait for status to settle
    time.Sleep(2 * time.Second)
    require.NoError(t, inst.UpdateStatus())

    // Kill and verify
    require.NoError(t, inst.Kill())
    assert.Equal(t, StatusError, inst.Status) // Kill sets StatusError
}
```

### Testing Skills Catalog Operations
```go
// Source: internal/session/skills_catalog_test.go:TestAttachDetachSkillProject
func TestSkillAttachAndDetach(t *testing.T) {
    _, cleanup := setupSkillTestEnv(t)
    defer cleanup()

    sourcePath, _ := os.MkdirTemp("", "test-source-*")
    defer os.RemoveAll(sourcePath)

    writeSkillDir(t, sourcePath, "my-skill", "my-skill", "Test skill")
    SaveSkillSources(map[string]SkillSourceDef{
        "local": {Path: sourcePath, Enabled: boolPtr(true)},
    })

    projectPath, _ := os.MkdirTemp("", "test-project-*")
    defer os.RemoveAll(projectPath)

    attached, err := AttachSkillToProject(projectPath, "my-skill", "local")
    require.NoError(t, err)
    assert.Equal(t, "my-skill", attached.Name)
}
```

### Testing Hook-Based Status Updates
```go
// Source: internal/session/hook_watcher_test.go:TestStatusFileWatcher_UpdatesExisting
func TestHookStatusUpdate(t *testing.T) {
    tmpDir := t.TempDir()
    hooksDir := filepath.Join(tmpDir, "hooks")
    os.MkdirAll(hooksDir, 0755)

    w := &StatusFileWatcher{
        hooksDir: hooksDir,
        statuses: make(map[string]*HookStatus),
    }

    data, _ := json.Marshal(map[string]any{
        "status": "running", "session_id": "s1",
        "event": "UserPromptSubmit", "ts": time.Now().Unix(),
    })
    filePath := filepath.Join(hooksDir, "inst-x.json")
    os.WriteFile(filePath, data, 0644)
    w.processFile(filePath)

    assert.Equal(t, "running", w.GetHookStatus("inst-x").Status)
}
```

### Testing Storage Persistence
```go
// Source: internal/session/storage_test.go:TestStorageUpdatedAtTimestamp
func TestStoragePersistsStatus(t *testing.T) {
    s := newTestStorage(t)

    instances := []*Instance{{
        ID: "test-1", Title: "Test", ProjectPath: "/tmp",
        GroupPath: "grp", Tool: "shell", Status: StatusRunning,
        CreatedAt: time.Now(),
    }}

    err := s.SaveWithGroups(instances, nil)
    require.NoError(t, err)

    loaded, _, err := s.LoadWithGroups()
    require.NoError(t, err)
    require.Len(t, loaded, 1)
    assert.Equal(t, StatusRunning, loaded[0].Status)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| JSON sessions.json | SQLite WAL state.db | v0.11.0 (Feb 2026) | All persistence tests use `newTestStorage(t)` with statedb |
| Tmux polling only | Hook fast path + tmux fallback | Pre-v0.11.0 | Status tests must consider both paths |
| Global session namespace | Profile-isolated (\_test) | After Dec 2025 incident | TestMain enforces `AGENTDECK_PROFILE=_test` |
| Broad test cleanup | Specific artifact patterns | After Jan 2026 incident | Only kill sessions matching known test names |

**Deprecated/outdated:**
- `sessions.json` persistence: Replaced by SQLite. Migration code exists but tests should use `newTestStorage(t)`.
- Raw tmux status polling without hooks: Still works for shell sessions, but Claude sessions prioritize hook fast path.

## Open Questions

1. **Attach Testing Approach**
   - What we know: `tmux.Session.Attach()` is interactive (requires a PTY). Cannot be fully tested in automated tests.
   - What is unclear: Whether the project considers a unit test (verifying correct tmux command construction) sufficient, or requires a PTY-based integration test.
   - Recommendation: Test at the unit level by verifying `inst.GetTmuxSession()` returns non-nil for running sessions, and that the session exists in tmux. Skip full attach integration test with a comment explaining the PTY requirement. This matches the existing pattern where `skipIfNoTmuxServer` is used.

2. **Bug Discovery Tracking**
   - What we know: STAB-01 requires all bugs found during testing to be fixed.
   - What is unclear: How to systematically track bugs discovered during test writing.
   - Recommendation: Each plan should have a "Bugs Found" section. Bugs discovered during test writing should be fixed in the same plan, with regression tests added.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None (Go convention: `_test.go` files, `go test` command) |
| Quick run command | `go test -race -v -run TestSpecificName ./internal/session/...` |
| Full suite command | `go test -race -v ./...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TEST-01 | Sleep/wake status transitions | integration | `go test -race -v -run TestSleepWake ./internal/session/... -timeout 30s` | Partial (TestNewSessionStatusFlicker exists) |
| TEST-02 | Skills trigger correctly | unit | `go test -race -v -run TestSkill ./internal/session/... -timeout 30s` | Partial (catalog tests exist, runtime trigger tests needed) |
| TEST-03 | Session start creates tmux | integration | `go test -race -v -run TestSessionStart ./internal/session/... -timeout 30s` | Partial (TestNewSessionStatusFlicker covers start) |
| TEST-04 | Session stop terminates tmux | integration | `go test -race -v -run TestSessionStop ./internal/session/... -timeout 30s` | Not directly (Kill tested implicitly via defer) |
| TEST-05 | Session fork creates copy | unit+integration | `go test -race -v -run TestFork ./internal/session/... -timeout 30s` | Yes (fork_integration_test.go, instance_test.go) |
| TEST-06 | Session attach connects | unit | `go test -race -v -run TestAttach ./internal/session/... -timeout 30s` | No (interactive, needs unit-level test) |
| TEST-07 | Status tracks tmux state | integration | `go test -race -v -run TestStatusTrack ./internal/session/... -timeout 30s` | Partial (storage_test.go, status_fixes_test.go) |
| STAB-01 | All discovered bugs fixed | regression | `go test -race -v ./... -timeout 120s` | N/A (bugs not yet discovered) |

### Sampling Rate
- **Per task commit:** `go test -race -v -run TestNewTest ./internal/session/... -timeout 30s`
- **Per wave merge:** `go test -race -v ./... -timeout 120s`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
None. Existing test infrastructure covers all phase requirements. The `testmain_test.go` files provide profile isolation. The `skipIfNoTmuxServer(t)` helper handles CI environments. The `newTestStorage(t)` helper provides SQLite test databases. No new framework or config needed.

## Sources

### Primary (HIGH confidence)
- Codebase analysis: `internal/session/instance.go` (4340+ lines) for status lifecycle
- Codebase analysis: `internal/session/instance_test.go` (2600+ lines) for test patterns
- Codebase analysis: `internal/session/skills_catalog.go` and `skills_catalog_test.go` for skills system
- Codebase analysis: `internal/tmux/tmux.go` and `tmux_test.go` for tmux integration
- Codebase analysis: `internal/session/transition_daemon.go` for status polling architecture
- Codebase analysis: `internal/session/hook_watcher.go` and `hook_watcher_test.go` for hook-based status
- Codebase analysis: `internal/session/storage_test.go` for SQLite persistence patterns
- Codebase analysis: `internal/session/fork_integration_test.go` for fork test patterns
- `CLAUDE.md`: Testing conventions, profile isolation requirement, historical incidents

### Secondary (MEDIUM confidence)
- Go testing documentation for `-race`, `-timeout`, table-driven test patterns
- `go.mod`: Confirmed testify v1.11.1, Go 1.24

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH. All libraries already in go.mod, all patterns already in codebase.
- Architecture: HIGH. Test structure follows existing codebase conventions with 1222 existing tests.
- Pitfalls: HIGH. Documented from actual incidents (production data corruption, memory leak from orphaned sessions) with code-level evidence.

**Research date:** 2026-03-06
**Valid until:** 2026-04-06 (stable Go test patterns, project-specific)
