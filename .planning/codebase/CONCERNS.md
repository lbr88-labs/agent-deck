# Codebase Concerns

**Analysis Date:** 2026-03-11

## Tech Debt

**Global Search Memory Leak (High Impact, Partially Fixed):**
- Issue: Global search feature opens 884+ directory watchers and loads 4.4 GB of JSONL content into memory, causing agent-deck to balloon to 6+ GB and get OOM-killed
- Files: `internal/ui/home.go:752-766`, `internal/session/global_search.go:589-599`
- Impact: System crashes under heavy conversation load; feature is currently disabled (init code commented out)
- Fix approach: Limit watched directories to project-level only (depth 1, not all subdirectories). Already partially implemented at `global_search.go:589-599` but feature remains disabled due to overall memory footprint. Consider re-enabling with stricter tier enforcement or lazy-loading of JSONL data.

**Monolithic home.go File (Code Complexity):**
- Issue: `internal/ui/home.go` is 10,851 lines in a single file with 80+ methods and significant intertwined state
- Files: `internal/ui/home.go`
- Impact: Makes refactoring difficult; increases cognitive load for understanding control flow; harder to test individual UI behaviors
- Fix approach: Extract discrete concerns into separate modules (e.g., `preview_manager.go`, `analytics_handler.go`, `notification_manager.go`). Keep `home.go` as the message dispatcher. Requires careful handling of shared state like `instancesMu`, `previewCacheMu`, `analyticsCacheMu`.

**Monolithic main.go File (Go Complexity):**
- Issue: `cmd/agent-deck/main.go` is 2,711 lines with subcommand implementations mixed with CLI utilities
- Files: `cmd/agent-deck/main.go`
- Impact: Hard to navigate; session command logic is interleaved with MCP, conductor, worktree logic
- Fix approach: Extract subcommands into separate files more thoroughly (partial refactor already exists with `session_cmd.go`, `mcp_cmd.go`, etc.). Move utility functions to `cli_utils.go`. Consolidate init/setup logic.

## Known Issues

**Race Condition in Session Deletion (Medium Impact):**
- Symptoms: TUI force-saves can re-insert a deleted session after CLI deletion
- Files: `cmd/agent-deck/main.go:1486-1507`
- Trigger: Run `agent-deck session delete <id>` while TUI is running and persisting session state
- Workaround: Direct SQL DELETE happens before SaveWithGroups, reducing but not eliminating the race window. Currently mitigated by deleting from instances list and re-saving with groups.
- Root cause: TUI's `forceSaveInstances()` and CLI's `SaveWithGroups()` can race without distributed locks

**Timing-Sensitive Send Operations (Medium Impact):**
- Symptoms: Messages "pasted but not submitted" silently; timing-dependent test failures in CI
- Files: `cmd/agent-deck/session_cmd.go:1370-1394`, `cmd/agent-deck/session_cmd.go:1496`
- Trigger: Rapid sends when agent is slow to initialize; ptty buffer races
- Mitigation: Uses `waitForAgentReady()` grace period (1.5s), followed by retry loops (up to 8 retries with 150ms delays). Logic documented in comments at lines 1378-1379.
- Risk: Timing assumptions may break on slow systems or under high tmux load

**Session ID Format Assumptions (Low Impact):**
- Symptoms: None observed; defensive code in place
- Files: `internal/session/instance.go:633`, `633`, `745`
- Issue: Code assumes session IDs are in `ses_XXXXX` format (5+ hex chars). This is internal implementation detail but not enforced at ID generation time.
- Fix approach: Add explicit validation in `OpenCode.GenerateSessionID()` to assert format or use a strongly-typed SessionID struct

**Preview Cache Invalidation Edge Case (Low Impact):**
- Symptoms: Stale previews shown after rapid session state changes
- Files: `internal/ui/home.go:1533-1540`, `internal/ui/home.go:1731-1760`
- Issue: Preview cache uses `previewCacheMu RWMutex` but invalidation is not synchronized with fetch operations. If a fetch is in-flight when invalidation occurs, old data might be written to cache after invalidation.
- Fix approach: Use a revision number or timestamp in cache key; invalidation increments the revision. This ensures stale fetches can't overwrite newer data.

## Security Considerations

**Subprocess Command Injection (Low Risk):**
- Risk: `exec.Command()` calls with user-provided paths could be exploited if path validation is insufficient
- Files: `cmd/agent-deck/worktree_cmd.go:618`, `cmd/agent-deck/session_cmd.go:920-928`, `internal/session/conductor.go` (pip install)
- Current mitigation: Uses direct command args (not shell), which prevents injection via shell metacharacters. Paths are validated before execution in most cases.
- Recommendations: Audit path validation in worktree operations; consider using a whitelist of allowed git commands; add tests for malicious path input

**Session ID Leakage (Low Risk):**
- Risk: Session IDs (Claude, Gemini, OpenCode, Codex) are stored in SQLite and tmux environment variables
- Files: `internal/session/storage.go`, `cmd/agent-deck/session_cmd.go:920`
- Current mitigation: Files are user-owned (no world-readable permissions on `~/.agent-deck/`); SQLite is protected by filesystem permissions
- Recommendations: Ensure state.db file mode is 0600; document that session IDs should be treated as short-lived authentication tokens

**No Input Validation on Group Paths (Low Risk):**
- Risk: Group paths use user input without strict validation; could create deeply nested or special-character paths
- Files: `internal/session/groups.go`
- Current mitigation: Paths use forward slash separators; basic string operations only
- Recommendations: Add regex validation to enforce safe group path format; limit nesting depth to prevent DoS

## Performance Bottlenecks

**tmux Session Caching Polling (Moderate Impact):**
- Problem: Status updates poll tmux window_activity every 2 seconds via `RefreshSessionCache()`. On systems with 50+ sessions, this can become CPU-intensive
- Files: `internal/tmux/tmux.go:57-74`, `internal/ui/home.go:1525-1532` (tick loop)
- Cause: Uses `tmux list-windows -a` subprocess as fallback (when control-mode pipe fails)
- Improvement path: Prioritize `PipeManager.RefreshAllActivities()` (zero subprocess); add batching of status updates; consider increasing tickInterval from 2s to 3-4s for large session counts

**Global Search Index Loading (High Impact):**
- Problem: Loading all JSONL files into memory at startup takes 15+ seconds on large conversations
- Files: `internal/session/global_search.go:614-644`
- Cause: Synchronous loading of all files during `NewGlobalSearchIndex()` initialization
- Improvement path: Implement lazy-loading (load on-demand per project); use sqlite FTS (full-text search) instead of in-memory indexing; add progress reporting

**Preview Rendering on Large Conversations (Moderate Impact):**
- Problem: Fetching and rendering previews for 100+ sessions can cause frame drops
- Files: `internal/ui/home.go:1731-1760`, `internal/ui/home.go:1792-1802`
- Cause: `fetchPreviewDebounced` can queue multiple async fetch operations that render synchronously
- Improvement path: Implement view-port-based fetching (only fetch visible previews); add LRU cache limit; consider lower resolution previews for off-screen sessions

**Log Maintenance Interval Too Aggressive (Low-Medium Impact):**
- Problem: Log maintenance runs every 5 minutes (line 75), checking all session logs for oversized files
- Files: `internal/ui/home.go:69-75`, `internal/ui/home.go:836-853`
- Cause: Background worker iterates all sessions to check log file sizes
- Improvement path: Increase interval to 15-30 minutes; add file size stats caching; only check logs that recently had activity

## Fragile Areas

**Session Lifecycle State Machine (High Fragility):**
- Files: `internal/session/instance.go` (status field), `internal/ui/home.go:statusWorker()` (lines 2033-2120)
- Why fragile: Status transitions (`starting` -> `running` <-> `waiting` -> `idle` -> `error`) are driven by background polling. No explicit state machine enforces valid transitions; multiple goroutines update status. Grace periods (1.5s for startup) are timing-sensitive.
- Safe modification: Add explicit `Status.IsValidTransition(from, to) bool` check before any status update. Document grace period assumptions clearly. Add comprehensive tests for all transition paths.
- Test coverage: `internal/session/lifecycle_test.go` covers basic Start/Stop/DoubleKill but not stress-testing concurrent status updates. Missing tests for grace period race conditions.

**Worktree Operations Without Rollback (Medium Fragility):**
- Files: `cmd/agent-deck/worktree_cmd.go:600-650`
- Why fragile: Calls `git worktree add`, then `git checkout`. If checkout fails, worktree is left in inconsistent state. No rollback/cleanup of created worktree.
- Safe modification: Wrap in a transaction-like structure with explicit cleanup on error. Test all failure modes (insufficient disk, permission denied, checkout conflict).
- Test coverage: `worktree_cmd_test.go` only tests success path; no failure scenario tests

**MCP Connection Pool Without Bounds (Medium Fragility):**
- Files: `internal/mcppool/http_server.go`, `internal/mcppool/socket_proxy.go`
- Why fragile: Pool can spawn unlimited HTTP servers and socket proxies. No limit on concurrent connections. If MCP processes hang, connections can leak.
- Safe modification: Add max pool size limits; implement timeout-based connection eviction; track connection lifetime metrics.
- Test coverage: Tests exist but don't cover failure/hang scenarios

**Conductor Template Rendering (Low-Medium Fragility):**
- Files: `internal/session/conductor_templates.go` (2,100 lines of Go template code)
- Why fragile: Large embedded Go templates with minimal test coverage. Template logic is string-based (hard to debug). No validation of template syntax before rendering.
- Safe modification: Add template validation on startup; consider using structured template packages; add round-trip tests (render -> parse -> compare).
- Test coverage: `conductor_test.go` has basic tests but doesn't validate all template branches

## Scaling Limits

**tmux Session Cache (Hard Limit ~300 sessions):**
- Current capacity: ~50-100 sessions (soft limit before CPU spikes); ~300 sessions before tmux itself degrades
- Limit: `tmux list-windows` becomes slow (O(n)); control-mode pipe overhead increases linearly
- Scaling path: Implement session groups / lazy-loading; query only active sessions; use dedicated tmux server instance per profile

**SQLite Database Size (Hard Limit ~1GB):**
- Current capacity: ~50K sessions before state.db reaches 500MB (WAL file + main db)
- Limit: Checkpoint overhead increases; query times degrade after ~1GB
- Scaling path: Archive old sessions to separate database; implement partition-by-date strategy; add data retention policies

**Log File Growth (Hard Limit ~100GB total):**
- Current capacity: ~30 sessions with 2GB logs each before disk fills
- Limit: Log maintenance (5-min interval) becomes O(N); orphaned logs can cause unexpected disk usage
- Scaling path: Implement log rotation by size (100MB per session); add compression (gzip); clean up orphans more aggressively

**Analytics Cache (Memory Limit ~2GB):**
- Current capacity: ~1000 sessions with full analytics cache
- Limit: `analyticsCacheMu RWMutex` protects maps that grow unbounded; cache TTL only removes stale entries
- Scaling path: Add LRU eviction; implement size-based limits; use memory-mapped storage for large datasets

## Dependencies at Risk

**Bubble Tea Framework Stability (Medium Risk):**
- Risk: Large monolithic `home.go` depends heavily on Bubble Tea message dispatch; framework updates could break assumptions
- Impact: If Bubble Tea changes View() semantics or message handling, significant refactoring needed
- Migration plan: Extract UI logic into interfaces (decouple from Bubble Tea). Maintain integration tests against specific Bubble Tea version. Pin major version in go.mod.

**SQLite via modernc.org/sqlite (Low Risk):**
- Risk: Pure-Go SQLite implementation may have performance regressions vs C SQLite; fewer users than cgo version
- Impact: Unusual bugs could surface; schema migrations might behave differently
- Migration plan: Add fallback to system SQLite (via cgo) if performance degrades. Keep schema migration path open.

**Go 1.24+ Language Features (Low Risk):**
- Risk: Code uses `maps` package and other 1.24+ stdlib features; users on Go 1.23 can't build
- Impact: Build failures for older Go versions
- Migration plan: Document minimum Go version clearly. Add CI check for version compatibility. Consider conditional compilation for version-specific features.

## Missing Critical Features

**No Session Backup/Recovery (Medium Priority):**
- Problem: If state.db is corrupted, all session metadata is lost
- Blocks: Disaster recovery; long-term session persistence
- Potential solution: Auto-backup state.db before major operations; implement database restore from backup command

**No Distributed Lock for Multi-Instance Concurrency (Medium Priority):**
- Problem: Multiple agent-deck processes writing to same state.db can race (e.g., TUI + CLI)
- Blocks: Reliable concurrent access; safe deletion + recreation
- Potential solution: Implement advisory locks via SQLite; use file-based locking (flock); add lock timeout and expiry

**No Structured Logging for Log Parsing (Low Priority):**
- Problem: Session logs are free-form text; hard to parse programmatically for metrics/errors
- Blocks: Automated error detection; analytics aggregation
- Potential solution: Add optional structured JSON logging format; emit events to stderr in addition to file logs

**No Session Pause/Resume (Low Priority):**
- Problem: Only kill or leave running; can't temporarily suspend a tmux session
- Blocks: Power management; temporary context switches
- Potential solution: Implement `agent-deck session pause` (detach all panes); `agent-deck session resume` (reattach)

## Test Coverage Gaps

**Untested Conductor Bridge Logic (High Priority):**
- What's not tested: Python conductor bridge integration (Telegram, Slack, Discord bot logic)
- Files: `cmd/agent-deck/conductor_cmd.go` (bridge spawn), `internal/session/conductor.py` (if exists) — bridge logic is external/not in Go
- Risk: Bridge failures silently; multi-agent coordination bugs go undetected
- Recommendation: Add integration tests that spawn actual bridge process; mock bot API responses; verify message flow

**Untested Pool Manager (Medium Priority):**
- What's not tested: `internal/session/pool_manager.go` (286 lines, no test file)
- Files: `internal/session/pool_manager.go`
- Risk: Connection pool bugs (leaks, timeouts, concurrent access); no tests for failure scenarios
- Recommendation: Add tests for pool saturation, connection eviction, timeout handling

**Untested Migration Logic (Medium Priority):**
- What's not tested: Schema migrations from v0 -> v1 -> v2
- Files: `internal/session/migration.go` (230 lines, no test file)
- Risk: Data loss on migration; breaking changes not caught until deployment
- Recommendation: Add roundtrip tests (create v0 schema, migrate, verify v2 schema is valid). Test with real data samples.

**Untested Transition Daemon (Medium Priority):**
- What's not tested: Background status transition logic
- Files: `internal/session/transition_daemon.go` (395 lines, no test file)
- Risk: Status stuck in wrong state; daemon crashes silently
- Recommendation: Add tests for all status transitions; mock tmux/Claude APIs; test grace period timing

**Send/Receive Retry Logic Edge Cases (Low Priority):**
- What's not tested: `send_helper.go` retry logic under pathological timing (extreme delays, rapid re-sends)
- Files: `internal/session/send_helper.go`
- Risk: Silent send failures; undeterministic behavior under load
- Recommendation: Add chaos testing (introduce random delays in tmux); test with 100+ rapid sends

## Technical Debt Summary

| Area | Severity | Impact | Fix Effort |
|------|----------|--------|-----------|
| Global search memory leak | High | App crashes under load | Medium (2-3 days) |
| home.go monolith | High | Code maintainability | High (1-2 weeks) |
| main.go monolith | Medium | Navigation difficulty | Medium (3-5 days) |
| Session deletion race | Medium | Data inconsistency | Medium (2-3 days) |
| Worktree rollback | Medium | State inconsistency | Low (1 day) |
| Conductor templates | Medium | Limited test coverage | Medium (3-4 days) |
| Pool bounds | Medium | Resource leak risk | Low (1-2 days) |
| Backup/recovery | Medium | Disaster risk | High (1-2 weeks) |

---

*Concerns audit: 2026-03-11*
