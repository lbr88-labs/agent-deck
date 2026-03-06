# Phase 6: Conductor Pipeline & Edge Cases - Research

**Researched:** 2026-03-06
**Domain:** Conductor heartbeat pipeline, send-with-retry mechanics, skills integration, concurrent tmux polling, cross-process SQLite change detection
**Confidence:** HIGH

## Summary

Phase 6 covers the final five requirements for the integration testing milestone. These span three distinct domains: (1) conductor pipeline completion (COND-03 heartbeat round-trip, COND-04 send-with-retry), (2) skills system integration (EDGE-01), and (3) production-grade concurrency and storage edge cases (EDGE-02 concurrent polling, EDGE-03 external storage changes).

The codebase provides all the building blocks. The heartbeat system (`conductor.go`) uses a shell script that checks session status and sends a message via `session send`. The send-with-retry logic (`session_cmd.go:sendWithRetryTarget`) handles chunked sending, paste-marker detection, and composer prompt re-submission. The skills catalog (`skills_catalog.go`) has full discover/resolve/attach/detach/materialize logic with existing unit tests. The `StorageWatcher` (`storage_watcher.go`) polls SQLite `metadata.last_modified` timestamps, and the existing `storage_watcher_test.go` already tests single-process change detection (but not cross-process with separate `Storage` instances). The concurrent polling test requires spinning up 10+ real tmux sessions and calling `UpdateStatus()` in parallel goroutines under `-race`.

**Primary recommendation:** Split into three plans: (1) COND-03 + COND-04 (conductor pipeline tests), (2) EDGE-01 (skills integration), (3) EDGE-02 + EDGE-03 (concurrency and storage). Each plan adds tests to `internal/integration/` following the established TmuxHarness + polling helpers pattern. For COND-03, simulate the heartbeat shell script logic in Go (check status, send message, verify receipt). For COND-04, test `sendWithRetryTarget` with a real tmux session rather than mocks. For EDGE-02, use `errgroup` to poll 10+ sessions concurrently under `-race`.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| COND-03 | Conductor heartbeat round-trip completes (send heartbeat, child responds, verify receipt) | Heartbeat is a shell script that checks `session show --json` status, then calls `session send` if idle/waiting. Test by creating a tmux session running `cat`, simulating heartbeat logic (check status, send message, verify pane content). |
| COND-04 | Send-with-retry delivers to real tmux session with chunked sending and paste-marker detection | `sendWithRetryTarget` interface has `SendKeysAndEnter`, `GetStatus`, `SendEnter`, `CapturePaneFresh`. `tmux.Session` implements all four. Test with real tmux session running `cat`, send large messages (>4KB triggers chunking), verify delivery. |
| EDGE-01 | Skills discovered from directory, attached to session, trigger conditions evaluated correctly | `ListAvailableSkills()` discovers from sources.toml paths. `AttachSkillToProject()` resolves + materializes. Existing unit tests in `skills_catalog_test.go` and `skills_runtime_test.go` cover most logic. Integration test should verify end-to-end: create temp source dir with SKILL.md, register source, discover, attach, verify materialized symlink is readable. |
| EDGE-02 | Concurrent polling of 10+ sessions returns correct status for each without races | `Instance.UpdateStatus()` acquires `mu sync.RWMutex` per instance. Create 10+ real tmux sessions via TmuxHarness, spawn goroutines calling `UpdateStatus()` concurrently, assert each converges to expected status. The `-race` flag (standard in `make test`) detects data races. |
| EDGE-03 | Storage watcher detects external SQLite changes from a second Storage instance | `StorageWatcher` polls `statedb.LastModified()` every 2s. Two `statedb.StateDB` instances on the same file (SQLite WAL mode allows this). Instance B calls `db.Touch()`, instance A's watcher fires on `ReloadChannel()`. Similar to existing `storage_watcher_test.go` but with TWO statedb instances sharing ONE file path. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `testing` | 1.24 | Test framework | Built-in, no dependency |
| testify | v1.11.1 | Assertions (require/assert) | Already used in 50+ test files |
| errgroup | stdlib | Concurrent goroutine management | For EDGE-02 parallel polling test |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `internal/integration` | project | TmuxHarness, WaitForCondition, WaitForPaneContent, NewTestDB, InstanceBuilder | All tests in this phase |
| `internal/tmux` | project | Session (SendKeysAndEnter, GetStatus, CapturePaneFresh, SendKeysChunked) | COND-03, COND-04 |
| `internal/session` | project | Instance, UpdateStatus, StatusEvent, EventWatcher, skills catalog | All requirements |
| `internal/statedb` | project | StateDB (Open, Touch, LastModified, SaveInstances) | EDGE-03 |
| `internal/ui` | project | StorageWatcher (NewStorageWatcher, Start, ReloadChannel) | EDGE-03 |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Real tmux `sendWithRetryTarget` test | Only mock-based tests (existing in `session_send_test.go`) | Mock tests already exist and pass; COND-04 specifically requires real tmux to verify chunked delivery and paste-marker detection against actual terminal behavior |
| `session.Storage` for EDGE-03 | Direct `statedb.StateDB` | Storage adds migration logic, profile resolution, and mutex overhead. statedb.StateDB is the underlying engine and is what StorageWatcher actually watches. Using StateDB directly avoids test environment complexity. |
| Skills end-to-end with TUI | Skills catalog API only | TUI tests are out of scope (REQUIREMENTS.md). The catalog API (`ListAvailableSkills`, `AttachSkillToProject`) covers the requirement. |

**Installation:**
No new dependencies needed. All libraries are already in `go.mod`.

## Architecture Patterns

### Recommended Test Structure
```
internal/integration/
    harness.go              # [exists] TmuxHarness
    poll.go                 # [exists] WaitForCondition, WaitForPaneContent, WaitForStatus
    fixtures.go             # [exists] NewTestDB, InstanceBuilder
    testmain_test.go        # [exists] TestMain + orphan cleanup
    lifecycle_test.go       # [exists] LIFE-01..04 tests
    detection_test.go       # [exists] DETECT-01..03 tests
    conductor_test.go       # [exists] COND-01, COND-02 tests
                            # [ADD] COND-03, COND-04 tests
    edge_cases_test.go      # [NEW] EDGE-01, EDGE-02, EDGE-03
```

### Pattern 1: Heartbeat Round-Trip (COND-03)
**What:** Simulate the heartbeat shell script logic in Go: create a child session running `cat`, check its tmux existence, send a heartbeat message, and verify it appears in the pane.
**When to use:** Testing the conductor heartbeat pipeline without requiring launchd/systemd.
**Example:**
```go
// Heartbeat round-trip: parent sends heartbeat, child "responds" by echoing it
func TestConductor_HeartbeatRoundTrip(t *testing.T) {
    h := NewTmuxHarness(t)

    // Child session running cat (reads stdin, echoes to stdout)
    child := h.CreateSession("hb-child", "/tmp")
    child.Command = "cat"
    require.NoError(t, child.Start())

    WaitForCondition(t, 5*time.Second, 200*time.Millisecond,
        "child to exist", func() bool { return child.Exists() })

    // Simulate heartbeat: check session exists, then send message
    tmuxSess := child.GetTmuxSession()
    require.NotNil(t, tmuxSess)

    heartbeatMsg := "Heartbeat: Check all sessions"
    require.NoError(t, tmuxSess.SendKeysAndEnter(heartbeatMsg))

    // Verify receipt (child echoes it back via cat)
    WaitForPaneContent(t, child, "Heartbeat:", 5*time.Second)
}
```

### Pattern 2: Send-With-Retry via Real tmux (COND-04)
**What:** Exercise `sendWithRetryTarget` against a real `tmux.Session` (which implements the interface) rather than mocks. Test both small and large (chunked) messages.
**When to use:** Verifying that chunked sending and paste-marker detection work against real terminal behavior.
**Key insight:** The `sendRetryTarget` interface is defined in `session_cmd.go` (package main), not exported. For integration testing, use `tmux.Session` directly, which implements all four methods: `SendKeysAndEnter`, `GetStatus`, `SendEnter`, `CapturePaneFresh`.
**Example:**
```go
func TestConductor_ChunkedSendDelivery(t *testing.T) {
    h := NewTmuxHarness(t)

    inst := h.CreateSession("chunked-send", "/tmp")
    inst.Command = "cat"
    require.NoError(t, inst.Start())

    WaitForCondition(t, 5*time.Second, 200*time.Millisecond,
        "session to exist", func() bool { return inst.Exists() })

    // Build a message > 4096 bytes to trigger chunked sending
    bigMsg := strings.Repeat("ABCDEFGH", 600) // 4800 bytes
    tmuxSess := inst.GetTmuxSession()
    require.NotNil(t, tmuxSess)

    require.NoError(t, tmuxSess.SendKeysAndEnter(bigMsg))

    // Verify a unique substring appears in pane
    WaitForPaneContent(t, inst, "ABCDEFGH", 5*time.Second)
}
```

### Pattern 3: Skills Discovery + Attach (EDGE-01)
**What:** Create a temp directory with SKILL.md files, register it as a skill source, discover skills, attach to a temp project, and verify the materialized symlink is readable.
**When to use:** End-to-end skills integration without TUI.
**Key insight:** The existing `skills_catalog_test.go` uses `setupSkillTestEnv()` which overrides `HOME` and `CLAUDE_CONFIG_DIR`. Integration tests can use the same pattern.
**Example:**
```go
func TestEdge_SkillsDiscoverAttach(t *testing.T) {
    _, cleanup := session.SetupSkillTestEnvPublic(t) // needs export
    defer cleanup()

    sourcePath := t.TempDir()
    // Write a skill directory with SKILL.md
    skillDir := filepath.Join(sourcePath, "my-test-skill")
    os.MkdirAll(skillDir, 0o755)
    os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
        []byte("---\nname: my-test-skill\ndescription: Test\n---\n"), 0o644)

    // Register source
    require.NoError(t, session.SaveSkillSources(map[string]session.SkillSourceDef{
        "test-source": {Path: sourcePath, Enabled: session.BoolPtr(true)},
    }))

    // Discover
    skills, err := session.ListAvailableSkills()
    require.NoError(t, err)
    require.NotEmpty(t, skills)

    // Attach to project
    projectPath := t.TempDir()
    attachment, err := session.AttachSkillToProject(projectPath, "my-test-skill", "test-source")
    require.NoError(t, err)
    require.NotNil(t, attachment)

    // Verify materialized
    targetDir := filepath.Join(projectPath, attachment.TargetPath)
    content, err := os.ReadFile(filepath.Join(targetDir, "SKILL.md"))
    require.NoError(t, err)
    assert.Contains(t, string(content), "my-test-skill")
}
```

### Pattern 4: Concurrent Polling (EDGE-02)
**What:** Create 10+ real tmux sessions, launch goroutines calling `UpdateStatus()` simultaneously, verify all converge to expected status under `-race`.
**When to use:** Proving thread safety of the status polling pipeline.
**Example:**
```go
func TestEdge_ConcurrentPolling(t *testing.T) {
    h := NewTmuxHarness(t)
    const N = 12

    instances := make([]*session.Instance, N)
    for i := 0; i < N; i++ {
        inst := h.CreateSession(fmt.Sprintf("poll-%02d", i), "/tmp")
        inst.Command = "sleep 60"
        require.NoError(t, inst.Start())
        instances[i] = inst
    }

    // Wait for all sessions to exist
    for i, inst := range instances {
        WaitForCondition(t, 5*time.Second, 200*time.Millisecond,
            fmt.Sprintf("session %d exists", i),
            func() bool { return inst.Exists() })
    }

    time.Sleep(2 * time.Second) // Grace period

    // Concurrent UpdateStatus calls
    g, _ := errgroup.WithContext(context.Background())
    for _, inst := range instances {
        inst := inst
        g.Go(func() error {
            for j := 0; j < 5; j++ {
                if err := inst.UpdateStatus(); err != nil {
                    return err
                }
                time.Sleep(100 * time.Millisecond)
            }
            return nil
        })
    }
    require.NoError(t, g.Wait())

    // Verify each session has a valid status
    for i, inst := range instances {
        s := inst.GetStatusThreadSafe()
        assert.NotEqual(t, session.StatusError, s,
            "session %d should not be in error state", i)
    }
}
```

### Pattern 5: Cross-Process Storage Change Detection (EDGE-03)
**What:** Open TWO `statedb.StateDB` instances against the SAME SQLite file (WAL mode allows concurrent readers/writers). Create a `StorageWatcher` on db1, call `db2.Touch()`, assert watcher fires.
**When to use:** Verifying that the StorageWatcher detects changes from a separate process (simulated by a second StateDB instance).
**Key insight:** The existing `storage_watcher_test.go` uses a single db instance for both watching and touching. EDGE-03 specifically requires TWO instances to simulate cross-process behavior. SQLite WAL mode makes this safe.
**Example:**
```go
func TestEdge_StorageWatcherCrossInstance(t *testing.T) {
    tmpDir := t.TempDir()
    dbPath := filepath.Join(tmpDir, "state.db")

    // Instance A: watcher
    dbA, err := statedb.Open(dbPath)
    require.NoError(t, err)
    require.NoError(t, dbA.Migrate())
    t.Cleanup(func() { dbA.Close() })

    watcher, err := ui.NewStorageWatcher(dbA)
    require.NoError(t, err)
    defer watcher.Close()
    watcher.Start()

    // Instance B: writer (simulates another process)
    dbB, err := statedb.Open(dbPath)
    require.NoError(t, err)
    require.NoError(t, dbB.Migrate())
    t.Cleanup(func() { dbB.Close() })

    // Wait for watcher to start polling
    time.Sleep(100 * time.Millisecond)

    // External write triggers watcher
    require.NoError(t, dbB.Touch())

    select {
    case <-watcher.ReloadChannel():
        // Success: cross-instance change detected
    case <-time.After(5 * time.Second):
        t.Fatal("StorageWatcher should detect Touch() from second instance")
    }
}
```

### Anti-Patterns to Avoid
- **Testing heartbeat via launchd/systemd:** Tests should simulate the heartbeat logic in Go code, not install actual system daemons. The shell script is just `check status + session send`; replicate that logic directly.
- **Testing `sendWithRetryTarget` from package main:** The interface and function are in `cmd/agent-deck` (package main, unexported). Don't try to import them. Instead, test the underlying `tmux.Session` methods directly, which is what `sendWithRetryTarget` delegates to.
- **Using the real skills pool (`~/.agent-deck/skills/pool/`):** Always use temp directories and override HOME/CLAUDE_CONFIG_DIR. The existing `setupSkillTestEnv()` pattern handles this.
- **Creating 10+ sessions without cleanup:** Always use `TmuxHarness` which auto-cleans via `t.Cleanup()`. The `TestMain` also runs orphan cleanup as a safety net.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tmux session lifecycle | Custom tmux subprocess calls | `TmuxHarness.CreateSession()` + `Instance.Start()` | Handles naming, cleanup, prefix isolation |
| Polling for conditions | `time.Sleep` + assert | `WaitForCondition()` with timeout | Deterministic, informative timeout messages |
| Test SQLite databases | Manual temp file management | `NewTestDB(t)` from fixtures.go | Auto-cleanup, migration, isolation |
| Skills test environment | Custom HOME override | `setupSkillTestEnv(t)` from skills_catalog_test.go | Handles HOME, CLAUDE_CONFIG_DIR, cache clearing |
| Concurrent goroutine management | Raw `go func()` + WaitGroup | `errgroup.WithContext()` | Error propagation, context cancellation |

**Key insight:** All infrastructure from Phases 4-5 is reusable. Phase 6 adds no new helper code; it only adds test functions using the established patterns.

## Common Pitfalls

### Pitfall 1: sendRetryTarget Not Exportable
**What goes wrong:** Attempting to import `sendRetryTarget` or `sendWithRetryTarget` from `cmd/agent-deck` fails because they're in package `main`.
**Why it happens:** The retry logic was designed as CLI implementation, not library code.
**How to avoid:** Test the underlying `tmux.Session` methods directly (`SendKeysAndEnter`, `SendKeysChunked`, `CapturePaneFresh`). These are the actual delivery mechanism; the retry wrapper adds status-checking logic that's already tested via mocks in `session_send_test.go`.
**Warning signs:** Import errors mentioning "package main".

### Pitfall 2: Skills Tests Modifying Real Config
**What goes wrong:** Skills tests that don't override HOME/CLAUDE_CONFIG_DIR modify the real `~/.agent-deck/skills/sources.toml`.
**Why it happens:** `GetSkillsRootPath()` and `LoadSkillSources()` use `GetAgentDeckDir()` which reads from HOME.
**How to avoid:** Always use `setupSkillTestEnv(t)` which sets HOME and CLAUDE_CONFIG_DIR to temp directories. Note: this function is currently unexported (lowercase). Either export it or replicate the pattern in the integration package.
**Warning signs:** Skills tests passing locally but leaving artifacts in `~/.agent-deck/`.

### Pitfall 3: 10+ tmux Sessions Exhausting Resources
**What goes wrong:** Creating many tmux sessions in rapid succession can fail if tmux hits file descriptor limits or session naming collisions.
**Why it happens:** Each tmux session spawns a shell process; 10+ concurrent processes can be resource-intensive on CI.
**How to avoid:** Use simple commands (`sleep 60`) that consume minimal resources. The TmuxHarness prefix ensures unique naming. The orphan cleanup in TestMain catches leaks. Add `skipIfNoTmuxServer(t)` to gracefully skip in CI without tmux.
**Warning signs:** Flaky "session already exists" or "too many open files" errors.

### Pitfall 4: StorageWatcher Poll Timing
**What goes wrong:** Cross-instance test fails intermittently because `Touch()` happens before the watcher's first poll cycle.
**Why it happens:** `pollInterval` is 2s, so the watcher might not poll for up to 2s after start.
**How to avoid:** Wait at least 100ms after `watcher.Start()` before calling `Touch()`, and use a 5s timeout on `ReloadChannel()`. The watcher starts polling immediately via `time.NewTicker`.
**Warning signs:** Test passes locally but flakes with "Expected reload signal but got timeout".

### Pitfall 5: Skills setupSkillTestEnv Not Exported
**What goes wrong:** Can't call `setupSkillTestEnv(t)` from `internal/integration` package because it's lowercase in `internal/session`.
**Why it happens:** Test helpers in Go are package-private by convention.
**How to avoid:** Either (a) replicate the HOME/CLAUDE_CONFIG_DIR override pattern in the integration test file, or (b) export the function. Option (a) is simpler and avoids changing production package exports for test purposes.
**Warning signs:** Compilation error "unexported function".

### Pitfall 6: Grace Period for Status Detection
**What goes wrong:** `UpdateStatus()` returns `StatusStarting` even though the session is clearly running.
**Why it happens:** The status detection system has a 1.5s grace period after session start where it always returns "starting" regardless of pane content.
**How to avoid:** Wait at least 2 seconds after `Start()` before calling `UpdateStatus()` in tests. This is a deliberate design choice to avoid false "waiting" detection during tmux startup.
**Warning signs:** Status assertions failing with "got starting, want idle".

## Code Examples

### COND-03: Heartbeat Script Logic (from conductor.go)
The production heartbeat script does exactly this:
```bash
# From conductorHeartbeatScript in conductor.go
STATUS=$(agent-deck -p "$PROFILE" session show "$SESSION" --json 2>/dev/null | ...)
if [ "$STATUS" = "idle" ] || [ "$STATUS" = "waiting" ]; then
    agent-deck -p "$PROFILE" session send "$SESSION" "Heartbeat: ..." --no-wait -q
fi
```
In Go test code, this translates to: check `inst.Exists()`, send via `tmuxSess.SendKeysAndEnter()`, verify via `WaitForPaneContent()`.

### COND-04: Chunked Sending (from tmux.go:3052)
```go
// Source: internal/tmux/tmux.go
func (s *Session) SendKeysChunked(content string) error {
    const chunkSize = 4096
    const chunkDelay = 50 * time.Millisecond

    if len(content) <= chunkSize {
        return s.SendKeys(content)
    }

    chunks := splitIntoChunks(content, chunkSize)
    for i, chunk := range chunks {
        if err := s.SendKeys(chunk); err != nil {
            return fmt.Errorf("failed to send chunk %d/%d: %w", i+1, len(chunks), err)
        }
        if i < len(chunks)-1 {
            time.Sleep(chunkDelay)
        }
    }
    return nil
}
```
Test should send >4096 bytes and verify delivery.

### EDGE-01: Skills Discovery (from skills_catalog.go)
```go
// Source: internal/session/skills_catalog.go
func ListAvailableSkills() ([]SkillCandidate, error)
func AttachSkillToProject(projectPath, skillRef, sourceName string) (*ProjectSkillAttachment, error)
func DetachSkillFromProject(projectPath, skillRef, sourceName string) (*ProjectSkillAttachment, error)
```

### EDGE-03: StateDB Touch/LastModified (from statedb.go)
```go
// Source: internal/statedb/statedb.go
func (s *StateDB) Touch() error {
    return s.SetMeta("last_modified", fmt.Sprintf("%d", time.Now().UnixNano()))
}

func (s *StateDB) LastModified() (int64, error) {
    val, err := s.GetMeta("last_modified")
    if err != nil || val == "" {
        return 0, err
    }
    var ts int64
    _, err = fmt.Sscanf(val, "%d", &ts)
    return ts, err
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| fsnotify StorageWatcher | SQLite metadata polling StorageWatcher | v0.11.0+ | More reliable on NFS/WSL/9p; no fsnotify dependency for storage |
| sessions.json | state.db (SQLite WAL) | v0.11.0 | Concurrent access via WAL mode enables EDGE-03 test |
| Single-tool detection | Multi-tool PromptDetector | v0.10.0+ | Claude, Gemini, OpenCode, Codex all supported |

**Note:** The `StatusEventWatcher` (for conductor events) still uses fsnotify. Only the `StorageWatcher` (for SQLite database changes) was migrated to polling. COND-02 tests (Phase 5) already validated the fsnotify-based event watcher.

## Open Questions

1. **setupSkillTestEnv exportability**
   - What we know: The function is unexported in `internal/session/skills_catalog_test.go`. It overrides HOME, CLAUDE_CONFIG_DIR, and clears user config cache.
   - What's unclear: Whether exporting it would break any conventions.
   - Recommendation: Replicate the HOME/CLAUDE_CONFIG_DIR override pattern directly in the integration test. This is ~15 lines and avoids touching production code.

2. **boolPtr helper exportability**
   - What we know: `skillBoolPtr()` is unexported. `SaveSkillSources()` needs `*bool` for `Enabled` field.
   - What's unclear: Whether there's a public equivalent.
   - Recommendation: Define a local `boolPtr(v bool) *bool` helper in the test file. Trivial one-liner.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib testing + testify v1.11.1 |
| Config file | Makefile (test target: `go test -race -v ./...`) |
| Quick run command | `go test -race -v ./internal/integration/...` |
| Full suite command | `go test -race -v ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| COND-03 | Heartbeat round-trip (send heartbeat, verify receipt) | integration | `go test -race -v -run TestConductor_Heartbeat ./internal/integration/...` | Wave 0 |
| COND-04 | Send-with-retry: chunked sending + paste-marker detection on real tmux | integration | `go test -race -v -run TestConductor_Chunked ./internal/integration/...` | Wave 0 |
| EDGE-01 | Skills discover from directory, attach to session, verify materialized | integration | `go test -race -v -run TestEdge_Skills ./internal/integration/...` | Wave 0 |
| EDGE-02 | Concurrent polling of 10+ sessions without races | integration | `go test -race -v -run TestEdge_ConcurrentPolling ./internal/integration/...` | Wave 0 |
| EDGE-03 | StorageWatcher detects external SQLite change from second StateDB instance | integration | `go test -race -v -run TestEdge_StorageWatcher ./internal/integration/...` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race -v ./internal/integration/...`
- **Per wave merge:** `go test -race -v ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/integration/edge_cases_test.go` -- covers EDGE-01, EDGE-02, EDGE-03
- [ ] COND-03, COND-04 tests added to existing `internal/integration/conductor_test.go`

No new framework infrastructure, test fixtures, or config files needed. All existing helpers (TmuxHarness, WaitForCondition, NewTestDB, InstanceBuilder) are sufficient.

## Sources

### Primary (HIGH confidence)
- `internal/session/conductor.go` -- heartbeat script template, conductor setup/teardown, meta.json
- `cmd/agent-deck/session_cmd.go:sendWithRetryTarget` (lines 1637-1719) -- retry logic, paste-marker detection, composer prompt detection
- `internal/tmux/tmux.go:SendKeysChunked` (lines 3052-3070) -- 4096-byte chunk size, 50ms inter-chunk delay
- `internal/session/skills_catalog.go` -- full skills discovery/resolve/attach/detach pipeline
- `internal/ui/storage_watcher.go` -- SQLite metadata polling, 2s poll interval, 3s ignore window
- `internal/statedb/statedb.go:Touch/LastModified` -- metadata timestamp mechanism
- `internal/session/event_writer.go` + `event_watcher.go` -- fsnotify-based event pipeline
- `internal/integration/` (all files) -- established test patterns from Phases 4-5

### Secondary (MEDIUM confidence)
- `cmd/agent-deck/session_send_test.go` -- mock-based sendWithRetryTarget tests (validates interface contract)
- `internal/ui/storage_watcher_test.go` -- single-instance StorageWatcher tests (pattern for EDGE-03)
- `internal/session/skills_catalog_test.go` + `skills_runtime_test.go` -- skills unit test patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new dependencies, all patterns established in Phases 4-5
- Architecture: HIGH -- all five requirements have clear code paths in the codebase
- Pitfalls: HIGH -- identified from prior phase experience (grace periods, HOME override, poll timing)

**Research date:** 2026-03-06
**Valid until:** 2026-04-06 (stable project, no external dependency changes expected)
