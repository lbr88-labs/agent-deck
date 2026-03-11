# Testing Patterns

**Analysis Date:** 2026-03-11

## Test Framework

**Runner:**
- Go's standard `testing` package (t *testing.T)
- Run via `go test -race -v ./...`
- Config: Makefile defines test command with race detector enabled

**Assertion Library:**
- `github.com/stretchr/testify/assert` (soft assertions)
- `github.com/stretchr/testify/require` (hard assertions, fail-fast)
- Used in 27 packages throughout codebase

**Run Commands:**
```bash
make test              # Run all tests with race detector: go test -race -v ./...
make ci                # Full pipeline (lint + test + build): lefthook run pre-push --force
go test -race -v ./...  # Run all tests
go test -race -v ./internal/session/...  # Single package
go test -race -v -run TestFunctionName ./internal/session/...  # Single test
```

## Test File Organization

**Location:**
- Co-located with implementation: `instance.go` and `instance_test.go` in same directory
- Separate test files per domain: `lifecycle_test.go`, `claude_hooks_test.go`, `gemini_test.go` within `internal/session/`
- Test infrastructure in `testmain_test.go` per package

**Naming:**
- Test files: `*_test.go` suffix
- Test functions: `TestXXX` prefix matching exported functions or scenarios
- Helper functions: `createTestSession()`, `drainEvents()`, lowercase prefix

**Structure:**
```
internal/session/
├── instance.go
├── instance_test.go        # Instance lifecycle tests
├── lifecycle_test.go       # Start, stop, fork, attach tests
├── claude_hooks_test.go    # Claude integration tests
├── gemini_test.go          # Gemini CLI integration tests
├── userconfig_test.go      # Configuration tests
└── testmain_test.go        # TestMain + cleanup
```

## Test Structure

**Suite Organization:**
```go
func TestSessionStart_CreatesTmuxSession(t *testing.T) {
    skipIfNoTmuxServer(t)  // Skip if tmux not available

    // Setup
    inst := NewInstance("test-start-creates", "/tmp")
    inst.Command = "sleep 60"

    // Execute
    err := inst.Start()
    require.NoError(t, err, "Start() should succeed")
    defer func() { _ = inst.Kill() }()

    // Verify
    assert.True(t, inst.Exists(), "Exists() should return true after Start()")

    // Independent verification (cross-check)
    tmuxSess := inst.GetTmuxSession()
    require.NotNil(t, tmuxSess, "GetTmuxSession() should not be nil after Start()")
}
```

**Patterns:**

- **Setup, Execute, Verify (AAA):** Arrange inputs → Act on functionality → Assert results
- **Cleanup with defer:** `defer func() { _ = inst.Kill() }()` for resource cleanup
- **Cross-verification:** Multiple approaches to verify same behavior (e.g., `inst.Exists()` + raw tmux command)
- **Descriptive names:** Test names follow pattern `TestSubject_Scenario` (e.g., `TestSessionStart_CreatesTmuxSession`)
- **Helper functions marked with `t.Helper()`:**
```go
func createTestSession(t *testing.T, suffix string) string {
    t.Helper()  // Excludes this function from stack traces
    skipIfNoTmuxServer(t)
    // ...
}
```

## Mocking

**Framework:** No external mocking library; manual mocks and test doubles

**Patterns:**
- Create test instances with known state:
```go
inst := NewInstance("test-id", "/tmp")
inst.Command = "sleep 60"
inst.Tool = "claude"
```

- Use `t.TempDir()` for isolated filesystem:
```go
tmpHome := t.TempDir()
os.Setenv("HOME", tmpHome)
defer os.Setenv("HOME", origHome)
```

- Dependency injection through struct fields (no mocking library needed)

**What to Mock:**
- Filesystem operations (use `t.TempDir()`)
- Environment variables (save/restore pattern)
- External executables (accept command as parameter, test with `sleep` or `echo`)
- tmux sessions (real tmux instances via `createTestSession()`)

**What NOT to Mock:**
- Session state machines (test real behavior)
- Lock acquisition/release (test real concurrency with race detector)
- Git operations (test against real git repos in temp directories)
- tmux session creation/destruction (integration tests need real tmux)

**Example:**
```go
func TestNewHome_DisablesTmuxNotifications(t *testing.T) {
    origHome := os.Getenv("HOME")
    tmpHome := t.TempDir()
    os.Setenv("HOME", tmpHome)
    session.ClearUserConfigCache()
    defer func() {
        os.Setenv("HOME", origHome)
        session.ClearUserConfigCache()
    }()

    configDir := filepath.Join(tmpHome, ".agent-deck")
    os.MkdirAll(configDir, 0o755)
    configPath := filepath.Join(configDir, "config.toml")
    os.WriteFile(configPath, []byte("[tmux]\ninject_status_line = false\n"), 0o644)

    home := NewHome()
    assert.False(t, home.manageTmuxNotifications)
}
```

## Fixtures and Factories

**Test Data:**
- Factory functions for common test objects:
```go
func NewInstance(id, projectPath string) *Instance {
    return &Instance{
        ID: id,
        ProjectPath: projectPath,
        Tool: "claude",
        Status: StatusIdle,
        CreatedAt: time.Now(),
    }
}
```

- Minimal setup: Only set fields required for the test
- Clear dependencies (e.g., `NewInstanceWithTool()` when tool matters)

**Location:**
- Factory functions in main package: `instance.go` exports `NewInstance()`
- Test helpers in `testmain_test.go` and individual test files
- Temporary test sessions created inline with `createTestSession(t, "suffix")`

**Example Fixtures:**
```go
// In lifecycle_test.go
inst := NewInstance("test-id", "/tmp")
inst.Command = "sleep 60"

// In controlpipe_test.go
name := createTestSession(t, "capture")
```

## Coverage

**Requirements:** Not enforced (no coverage threshold configured)

**View Coverage:**
```bash
go test -race -cover ./...              # Show coverage percentage
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out        # View HTML report
```

**Gaps:**
- Error paths tested where critical (e.g., `Kill()` on stopped session)
- Edge cases covered (e.g., double-kill, nil returns)
- Integration tests (tmux operations) require running tmux server

## Test Types

**Unit Tests:**
- Scope: Single function or method
- Approach: Fast, deterministic, no external dependencies
- Examples: `TestGenerateSessionName()`, `TestHashProjectPath()`
- Location: `*_test.go` files in same package

**Integration Tests:**
- Scope: Multiple components or external systems (tmux, filesystem)
- Approach: Real tmux sessions, filesystem operations, git repos
- Examples: `TestSessionStart_CreatesTmuxSession()`, `TestControlPipe_ConnectAndClose()`
- Pattern: `skipIfNoTmuxServer(t)` guards tests requiring tmux
- Cleanup: `defer func() { _ = inst.Kill() }()` or `t.Cleanup()`

**E2E Tests:**
- Not formally separated; integration tests serve this role
- Example: `internal/integration/testmain_test.go` for cross-package scenarios

## Common Patterns

**Async Testing:**
```go
func TestControlPipe_OutputEvents(t *testing.T) {
    name := createTestSession(t, "output")
    pipe, err := NewControlPipe(name)
    require.NoError(t, err)
    defer pipe.Close()

    // Small delay to let pipe connect
    time.Sleep(200 * time.Millisecond)

    // Drain initial events
    drainEvents(pipe.OutputEvents(), 200*time.Millisecond)

    // Send command
    _ = exec.Command("tmux", "send-keys", "-t", name, "echo test", "Enter").Run()

    // Wait for event with timeout
    select {
    case <-time.After(2 * time.Second):
        t.Fatal("timeout waiting for output event")
    case event := <-pipe.OutputEvents():
        assert.Contains(t, event, "test")
    }
}
```

**Error Testing:**
```go
func TestSessionStop_DoubleKill(t *testing.T) {
    skipIfNoTmuxServer(t)

    inst := NewInstance("test-double-kill", "/tmp")
    inst.Command = "sleep 60"

    err := inst.Start()
    require.NoError(t, err)

    // First kill succeeds
    err = inst.Kill()
    require.NoError(t, err)

    // Second kill may return error (session already gone), which is acceptable
    _ = inst.Kill()  // Ignore error; double-kill is safe

    assert.Equal(t, StatusStopped, inst.Status)
    assert.False(t, inst.Exists())
}
```

**Profile Isolation (TestMain):**
```go
// CRITICAL: All test packages have TestMain for profile isolation
func TestMain(m *testing.M) {
    // Force _test profile for all tests
    os.Setenv("AGENTDECK_PROFILE", "_test")

    // Run tests
    code := m.Run()

    // Cleanup orphaned test sessions
    cleanupTestSessions()

    os.Exit(code)
}

// Cleanup only removes known test artifacts
func cleanupTestSessions() {
    out, _ := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
    sessions := strings.Split(strings.TrimSpace(string(out)), "\n")
    for _, sess := range sessions {
        // Only match specific artifacts, not broad patterns
        if strings.Contains(sess, "Test-Skip-Regen") {
            _ = exec.Command("tmux", "kill-session", "-t", sess).Run()
        }
    }
}
```

**Conditional Test Execution:**
```go
func skipIfNoTmuxServer(t *testing.T) {
    t.Helper()
    if _, err := exec.LookPath("tmux"); err != nil {
        t.Skip("tmux not available")
    }
    if err := exec.Command("tmux", "list-sessions").Run(); err != nil {
        t.Skip("tmux server not running")
    }
}
```

## Test Data Organization

**Temporary Directories:**
```go
tmpDir := t.TempDir()  // Auto-cleaned after test
configPath := filepath.Join(tmpDir, "config.toml")
os.WriteFile(configPath, []byte("..."), 0o644)
```

**Git Repositories:**
```go
// In git_test.go
func createTestRepo(t *testing.T, dir string) {
    t.Helper()
    exec.Command("git", "init", dir).Run()
    exec.Command("git", "-C", dir, "config", "user.email", "test@example.com").Run()
    exec.Command("git", "-C", dir, "config", "user.name", "Test User").Run()
}
```

## Critical Testing Rules

**Profile Isolation (CRITICAL):**
- All test packages MUST have `TestMain` setting `AGENTDECK_PROFILE=_test`
- Prevents test data from overwriting production sessions
- Historical incident (2025-12-11): Missing TestMain overwrote 36 production sessions
- Historical incident (2026-01-20): Orphaned tmux sessions wasted 3GB RAM

**Cleanup Pattern:**
- Use `defer` for resource cleanup (survives panics and fatal errors)
- Use `t.Cleanup()` for test-scoped setup/teardown
- Only kill known test artifacts in `cleanupTestSessions()`, not broad patterns

**Race Detector:**
- Always run tests with `-race` flag (`make test` does this automatically)
- Catches concurrent access bugs in production code

---

*Testing analysis: 2026-03-11*
