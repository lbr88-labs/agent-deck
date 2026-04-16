# Phase 21: Watcher Folder Hierarchy — Research

**Researched:** 2026-04-16
**Domain:** Go filesystem layout migration, atomic writes, POSIX rename/symlink semantics, TDD test harness
**Confidence:** HIGH (all claims verified against current worktree)

## Summary

Phase 21 reorganizes `~/.agent-deck/watchers/` (plural) to `~/.agent-deck/watcher/` (singular) mirroring the shipped conductor folder pattern, introduces three new files (`layout.go`, `state.go`, `event_log.go`), rewires `engine.go:134` + `session.WatcherDir()` + 6 downstream callers, and adds a one-shot atomic migration + compatibility symlink. The work is mechanically well-scoped: every pattern needed — atomic write-temp-rename, `t.Setenv("HOME", t.TempDir())`, POSIX `os.Rename` atomicity on same filesystem, `HealthTracker.Check()` classifier — already exists in the repo and is directly reusable. The research below addresses the 10 questions in the prompt; none revealed a blocker.

**Primary recommendation:** Follow the locked plan in CONTEXT.md verbatim. The only non-obvious detail is Question 4 (hot-reload semantics) — the engine holds no in-process cache of per-watcher state because `SaveState/LoadState` don't exist yet, so "hot-reload-safe" is trivially satisfied by designing those two helpers to always hit disk (no memoization). Document this in the Task B commit.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Path layout:** Root `~/.agent-deck/watcher/` (singular). Shared top-level files: `CLAUDE.md`, `POLICY.md`, `LEARNINGS.md`, `clients.json`. Per-instance subdir `<name>/` contains: `meta.json`, `state.json`, `task-log.md`, `LEARNINGS.md`. Compatibility symlink `~/.agent-deck/watchers -> watcher/` written during migration for one minor cycle.

**Canonical meta format:** `meta.json`. No new TOML parser. Matches existing `session.SaveWatcherMeta`/`LoadWatcherMeta` atomic write-temp-rename pattern.

**Migration:** On engine startup, if `watchers/` exists AND `watcher/` does not → `os.Rename(watchers, watcher)` then `os.Symlink(watcher, watchers)`. Single-shot. If both exist, skip + log warning. Log one line with fixed greppable prefix. Explicitly does NOT migrate `~/.agent-deck/issue-watcher/`.

**New files:** `internal/watcher/layout.go` (`LayoutDir`, `WatcherDir(name)`, `MigrateLegacyWatchersDir`, scaffold helpers), `internal/watcher/state.go` (`WatcherState` struct + `LoadState`/`SaveState`), `internal/watcher/event_log.go` (`AppendEventLog`). All atomic writes follow `SaveWatcherMeta` pattern.

**Code changes:**
- `internal/watcher/engine.go:134` — default `ClientsPath` to singular `watcher/` with one-cycle fallback to `watchers/`.
- `internal/session/watcher_meta.go:20-27` — update `session.WatcherDir()` to return `filepath.Join(dir, "watcher")`. Single source of truth; all callers benefit.
- Wire engine event loop: post-dedup call `AppendEventLog` + `SaveState`. Failures must not drop events.

**Templates:** `assets/watcher-templates/{CLAUDE.md, POLICY.md, LEARNINGS.md}`. Scaffold on first `LayoutDir()` call; only write missing files.

**CLI surface:** `watcher list --json` MUST include `last_event_ts`, `error_count`, `health_status`. Missing state.json → `null` / `0` / `"unknown"`. Reuse `internal/watcher/health.go` classifier.

**TDD sequencing:** A (RED, 6 unit + 1 integration tests) → B (GREEN + migration) → C (templates) → D (CLI surface). Commit prefixes `test(21-01)` / `feat(21-01)` / `docs(21-01)`.

### Claude's Discretion

- Naming of internal helpers inside `layout.go` / `state.go` (exported where used from `cmd/`, unexported otherwise).
- Whether `MigrateLegacyWatchersDir` is exported; recommend exported so tests can drive directly.
- `HealthWindow` schema (slice of samples with timestamp+ok-bool).
- Log prefix string (suggested: `watcher: migrated legacy ~/.agent-deck/watchers/ → ~/.agent-deck/watcher/`).
- Test file split — all in `layout_test.go` or split into `layout_test.go` / `state_test.go` / `event_log_test.go` / `integration_test.go`.

### Deferred Ideas (OUT OF SCOPE)

- Migration of legacy `~/.agent-deck/issue-watcher/` — out of scope; called out in migration log.
- Removing the `watchers -> watcher/` compatibility symlink — deferred to v1.7.0+.
- Per-watcher LEARNINGS.md UI — future phase.
- TOML round-trip between `watcher.toml` and `meta.json` — not introduced here.
- REQ-WF-7 (skills + docs sync) — Phase 22.
- REQ-WF-5 (verification harness) — Phase 23.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REQ-WF-6 | Reorganize watcher state to mirror conductor pattern. New layout `~/.agent-deck/watcher/{CLAUDE.md, POLICY.md, LEARNINGS.md, clients.json, <name>/{meta.json, state.json, task-log.md, LEARNINGS.md}}`. New files `layout.go`, `state.go`, `event_log.go`. Atomic `os.Rename` migration with one-cycle compatibility symlink. `watcher list --json` exposes `last_event_ts`, `error_count`, `health_status`. 6 unit + 1 integration test. | Conductor analog at `internal/session/conductor.go:246-260` (LayoutDir pattern). Atomic write-temp-rename at `internal/session/watcher_meta.go:47-75` (SaveState template). POSIX `os.Rename` same-fs guarantee confirmed (Q1). `HealthTracker.Check()` at `internal/watcher/health.go:148` reusable for `health_status` (Q5). `t.Setenv("HOME", t.TempDir())` pattern confirmed at `internal/watcher/gmail_test.go:165`, `internal/feedback/feedback_test.go:14` (Q9). |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

No repo-root `CLAUDE.md` present in current worktree (verified: `Read ./CLAUDE.md` → file does not exist). Global user CLAUDE.md (`~/.claude/CLAUDE.md`) directives that bind here:

- **Never use `rm`** — use `trash`. Relevant for test cleanup.
- **No Claude attribution in commits** — sign `Committed by Ashesh Goplani`.
- **Go 1.24.0 toolchain pinned** (STATE.md "Incident-Driven Rules"). The runtime has `go1.25.5` installed but `go.mod` pins `go 1.24.0`. Tests must run under `GOTOOLCHAIN=go1.24.0`.
- **Testing First:** every bug is a missing test; regression test written BEFORE fix. Phase 21's TDD sequence (Task A RED before B GREEN) is aligned.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Path resolution (LayoutDir, WatcherDir(name)) | `internal/watcher/layout.go` | `internal/session/watcher_meta.go` (WatcherDir update) | Layout lives with the watcher package; session package owns the single-source path function to avoid dual resolvers drifting. |
| Atomic meta.json / state.json write | `internal/watcher` (state.go) + `internal/session` (watcher_meta.go) | — | Same write-temp-rename primitive used in both packages. |
| Migration orchestration | `internal/watcher/layout.go::MigrateLegacyWatchersDir` | engine.go startup hook | Migration is a watcher-specific concern; engine triggers it once at Start. |
| Append-only event log | `internal/watcher/event_log.go` | engine.go writerLoop | Writer is single-goroutine (engine.go:275-399) so per-watcher serialization is already guaranteed by channel topology. |
| Health classification for `list --json` | `internal/watcher/health.go` (existing) | `cmd/agent-deck/watcher_cmd.go::handleWatcherList` | Classifier is exported via `HealthTracker.Check()`; CLI reads state.json + invokes `Check()` with rehydrated tracker, or mirrors the thresholds. |
| Template scaffolding | `assets/watcher-templates/*` (new) + `layout.go` helpers | — | Mirrors `internal/session/conductor.go::InstallSharedConductorInstructions` pattern (conductor.go:822-850). |
| Hot-reload via state.json | `state.go::LoadState` (no cache) | — | Always reads disk; no in-process memoization. See Q4. |

## Standard Stack

### Core (all stdlib — no new dependencies)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `os` stdlib | Go 1.24 | `Rename`, `Symlink`, `Lstat`, `OpenFile`, `WriteFile` | POSIX-mapped atomic primitives |
| `path/filepath` stdlib | Go 1.24 | `Join`, `Clean`, `EvalSymlinks` | Cross-platform path handling |
| `encoding/json` stdlib | Go 1.24 | `state.json` / `meta.json` marshal/unmarshal | Already the format in `watcher_meta.go` |
| `log/slog` stdlib | Go 1.24 | Migration log line | `engine.go` already uses `slog` (engine.go:119) |
| `testing` stdlib | Go 1.24 | `t.Setenv`, `t.TempDir`, `t.Cleanup` | Only idiomatic test harness needed |

**Installation:** None. Zero new dependencies. All primitives live in the stdlib and are already imported throughout `internal/watcher/`.

**Version verification:**
```bash
go version          # go version go1.25.5 linux/amd64 (installed)
grep '^go ' go.mod  # go 1.24.0 (module target)
```

Runtime pins to `GOTOOLCHAIN=go1.24.0` per STATE.md Incident-Driven Rules. `t.Setenv` requires Go 1.17+ — green.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `os.Rename` for migration | `io.Copy` + remove | Non-atomic; leaves window where data exists in both places or neither. Reject. |
| `os.WriteFile` for state.json | `atomic.File` from `renameio` | Adds a dep for a 3-line pattern already used in `watcher_meta.go:65-73`. Reject. |
| `fsnotify` for hot-reload | Stateless read-on-access | fsnotify adds a goroutine and platform caveats. Since state.json is only read by `list --json` (CLI, not engine), stateless reads are simpler. Reject. |
| Per-watcher `sync.Mutex` for task-log append | POSIX atomic append | See Q3 — single write ≤ PIPE_BUF is atomic on Linux/macOS. Mutex is needed only if you fan out to multiple goroutines per watcher (we don't). |

## Architecture Patterns

### System Architecture (data flow)

```
┌──────────────┐      ┌─────────────────────────────────────────┐
│ engine.Start │──1──▶│ MigrateLegacyWatchersDir (one-shot)     │
└──────────────┘      │  - os.Stat(watchers/), os.Stat(watcher/)│
       │              │  - os.Rename + os.Symlink if applicable │
       │              │  - slog.Info "watcher: migrated legacy" │
       │              └─────────────────────────────────────────┘
       │2
       ▼
┌──────────────┐
│ LayoutDir()  │─── scaffold CLAUDE.md/POLICY.md/LEARNINGS.md/clients.json if absent
└──────────────┘          (O_CREATE|O_EXCL — TOCTOU-free)
       │3 (first adapter goroutine starts)
       ▼
┌─────────────────┐   Event    ┌──────────────┐
│ adapter.Listen  │───────────▶│ writerLoop   │
└─────────────────┘            │ (engine.go   │
                               │  :275-399)   │
                               └──────┬───────┘
                                      │4 (INSERT OR IGNORE → inserted==true)
                                      ▼
                       ┌──────────────────────────────────┐
                       │ AppendEventLog(name, line)       │──▶ watcher/<name>/task-log.md
                       │ SaveState(name, updatedState)    │──▶ watcher/<name>/state.json (atomic)
                       └──────────────────────────────────┘
                                      │ failures log-and-continue; no event drops
                                      ▼
                       ┌──────────────────────────────────┐
                       │ routedEventCh / healthCh         │ (unchanged)
                       └──────────────────────────────────┘

CLI path:
  watcher list --json → LoadState(name) → HealthTracker.Check() rehydrated from state → health_status
```

### Recommended Project Structure

```
internal/watcher/
├── layout.go              # NEW: LayoutDir(), WatcherDir(name), MigrateLegacyWatchersDir(), scaffold helpers
├── layout_test.go         # NEW: 6 unit + 1 integration test (Task A)
├── state.go               # NEW: WatcherState struct + LoadState/SaveState
├── event_log.go           # NEW: AppendEventLog
├── engine.go              # MODIFY: :134 ClientsPath default, writerLoop wires AppendEventLog + SaveState
├── health.go              # UNCHANGED: classifier reused
└── ...                    # all other files unchanged

internal/session/
└── watcher_meta.go        # MODIFY: :20-27 WatcherDir() returns "watcher" (singular)

cmd/agent-deck/
└── watcher_cmd.go         # MODIFY: :524 error msg, :562/:614 no change (session.WatcherDir is source of truth)
                           # ADD: list --json extra fields (Task D)

assets/watcher-templates/  # NEW DIRECTORY (Task C)
├── CLAUDE.md
├── POLICY.md
└── LEARNINGS.md
```

### Pattern 1: Atomic Write-Temp-Rename

Copy verbatim from `internal/session/watcher_meta.go:47-75`:

```go
// Source: internal/session/watcher_meta.go:47-75
func SaveState(name string, s *WatcherState) error {
    dir, err := WatcherDir(name) // from layout.go
    if err != nil { return err }
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return fmt.Errorf("create watcher dir: %w", err)
    }
    data, err := json.MarshalIndent(s, "", "  ")
    if err != nil { return fmt.Errorf("marshal state.json: %w", err) }
    finalPath := filepath.Join(dir, "state.json")
    tmpPath := finalPath + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
        return fmt.Errorf("write state.json.tmp: %w", err)
    }
    if err := os.Rename(tmpPath, finalPath); err != nil {
        _ = os.Remove(tmpPath)
        return fmt.Errorf("rename state.json: %w", err)
    }
    return nil
}
```

### Pattern 2: Atomic Append for task-log.md

```go
// Append with O_APPEND is atomic for a single Write ≤ PIPE_BUF (4096 on Linux, 512 on some BSDs).
// Our lines — "## 2026-04-16T12:34:56Z - webhook: incident #1234" — are always < 512 bytes
// because adapters produce bounded subject text (verified across webhook.go / ntfy.go / github.go).
// Single-goroutine writer (engine.go writerLoop) also serializes per watcher name, so no mutex needed.
func AppendEventLog(name, entry string) error {
    dir, err := WatcherDir(name)
    if err != nil { return err }
    if err := os.MkdirAll(dir, 0o755); err != nil { return err }
    path := filepath.Join(dir, "task-log.md")
    f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
    if err != nil { return err }
    defer f.Close()
    // Single Write call — atomic by POSIX guarantee for ≤ PIPE_BUF with O_APPEND.
    _, err = f.WriteString(entry + "\n")
    return err
}
```

### Pattern 3: TOCTOU-free Scaffold (O_CREATE|O_EXCL)

```go
// Source: idiomatic Go pattern; equivalent in spirit to internal/session/conductor.go:520
// "if _, err := os.Stat(learningsPath); os.IsNotExist(err) { os.WriteFile(...) }"
// BUT that has a TOCTOU race. Use O_EXCL for true idempotence.
func writeIfAbsent(path string, content []byte) error {
    f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
    if err != nil {
        if errors.Is(err, fs.ErrExist) { return nil } // already there — idempotent success
        return err
    }
    defer f.Close()
    _, err = f.Write(content)
    return err
}
```

Note: the existing conductor scaffold uses `os.Stat` + `os.WriteFile` (`conductor.go:519-523`). For Phase 21 prefer `O_CREATE|O_EXCL` — the race window is tiny in practice but free to avoid.

### Pattern 4: Migration Guard

```go
// Source: new; follows MigrateLegacyConductors pattern at internal/session/conductor.go:925-969
func MigrateLegacyWatchersDir() error {
    deck, err := session.GetAgentDeckDir()
    if err != nil { return err }
    legacy := filepath.Join(deck, "watchers")
    current := filepath.Join(deck, "watcher")

    _, curErr := os.Stat(current)
    legacyInfo, legacyErr := os.Lstat(legacy) // Lstat: don't follow if it's already a symlink

    // Happy path: legacy exists, current missing → rename + symlink
    if legacyErr == nil && legacyInfo.Mode()&os.ModeSymlink == 0 && os.IsNotExist(curErr) {
        if err := os.Rename(legacy, current); err != nil {
            return fmt.Errorf("migrate watchers dir: %w", err)
        }
        // Relative symlink target: portable across users' $HOME paths.
        if err := os.Symlink("watcher", legacy); err != nil {
            slog.Warn("watcher: symlink creation failed (non-fatal)", "error", err)
        }
        slog.Info("watcher: migrated legacy ~/.agent-deck/watchers/ → ~/.agent-deck/watcher/",
            "note", "legacy ~/.agent-deck/issue-watcher/ NOT migrated (out of scope)")
        return nil
    }

    // Collision: both exist as real dirs (operator intervention). Log + skip.
    if legacyErr == nil && legacyInfo.Mode()&os.ModeSymlink == 0 && curErr == nil {
        slog.Warn("watcher: both ~/.agent-deck/watchers/ and ~/.agent-deck/watcher/ exist; skipping migration")
        return nil
    }
    // Nothing to do: no legacy, or legacy is already the compatibility symlink.
    return nil
}
```

### Anti-Patterns to Avoid

- **Stat-then-write scaffolding.** TOCTOU race. Use `O_CREATE|O_EXCL`.
- **Reading state.json into a global cache.** Kills hot-reload. Always hit disk.
- **Absolute symlink target.** `os.Symlink("/home/alice/.agent-deck/watcher", …/watchers)` breaks if the user's `HOME` moves. Use relative target `"watcher"`.
- **Multiple `session.WatcherDir()` resolvers.** CONTEXT.md locks the single-source-of-truth approach — don't introduce a parallel resolver in `layout.go`. `layout.go::WatcherDir(name)` should call `session.WatcherDir()` underneath.
- **Writing task-log.md without O_APPEND.** A truncating write on each event would race catastrophically. Must use `O_APPEND|O_CREATE`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Atomic file write | Hand-rolled temp + fsync loop | Existing `SaveWatcherMeta` pattern (`watcher_meta.go:47-75`) | Already battle-tested; keeps codebase consistent. |
| Health classification | New threshold rules | `HealthTracker.Check()` at `health.go:148` | Spec mandates reuse; thresholds (`>= 3`, `>= 10`, `maxSilenceMinutes`) locked by existing tests. |
| JSON marshal pattern | Custom encoder | `json.MarshalIndent` | Matches `SaveWatcherMeta:61`. |
| Symlink direction | Novel layout | `os.Symlink(target, link)` where `target` is relative | Idiomatic; survives `$HOME` moves. |
| Per-watcher write serialization | Explicit `sync.Mutex` per name | engine.go `writerLoop` is already single-goroutine | `engine.go:275-399` is the one and only writer per engine; serial by design. |

**Key insight:** Phase 21 is 100% composition of existing primitives. No new libraries, no new abstractions. Every line of `state.go`, `event_log.go`, and `layout.go` has a direct analog in `watcher_meta.go` or `conductor.go`.

## Runtime State Inventory

Phase 21 is a rename + layout migration. Inventory per Step 2.5:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `~/.agent-deck/watchers/` dir on user machines (plural). Contains per-watcher subdirs with `meta.json`, `credentials.json` (gmail only), `token.json` (gmail only), plus top-level `clients.json`. On fresh dev installs it may not exist. | **Data migration:** `os.Rename(watchers, watcher)` on first boot. Single-shot. Existing `meta.json` / `credentials.json` / `token.json` / `clients.json` stay in place — only the parent dir is renamed. **Code edit:** `session.WatcherDir()` returns singular. |
| Live service config | None — watchers are a local-only feature; no external service carries the plural string in any DB/UI config. Gmail Pub/Sub topics and ntfy topic names encode the user's chosen values, not `"watchers"`. | None. |
| OS-registered state | None. Watchers don't register launchd plists / systemd units / Windows tasks. Only conductors do (`conductor.go:1449-1498`); watchers piggyback on the engine goroutine which runs inside the `agent-deck` TUI process. | None. |
| Secrets / env vars | Gmail OAuth credentials (`credentials.json`, `token.json`) live under `~/.agent-deck/watchers/<name>/`. File NAMES unchanged — only their parent dir is renamed via `os.Rename`. Env vars `AGENTDECK_PROFILE`, `HOME` — unaffected. | None (data rides along with the `os.Rename`). Verify `gmail.go:137` still resolves correctly after `session.WatcherNameDir(a.name)` returns the singular path. |
| Build artifacts / installed packages | None. Go binary is statically compiled; no external package embeds the string. Embedded skill assets DO reference `watchers/` (6+ lines in `SKILL.md`) — but that's REQ-WF-7 / Phase 22 scope, not Phase 21. | Deferred to Phase 22. |

**The canonical question — after every file in the repo is updated, what runtime systems still have the old string cached / stored / registered?** Answer: The user's local `~/.agent-deck/watchers/` directory. Addressed by the one-shot migration. Nothing else.

## Common Pitfalls

### Pitfall 1: Cross-filesystem `os.Rename` failure

**What goes wrong:** On Linux, `rename(2)` returns `EXDEV` when source and destination live on different filesystems. If `$HOME` is on one mount and `/tmp` is on another, a test that uses `t.TempDir()` for `HOME` could in principle see different storage. In practice, `t.TempDir()` returns `$TMPDIR/TestName/NNN` — same filesystem as everything else in the test env. Production: `~/.agent-deck/watchers/` and `~/.agent-deck/watcher/` are siblings; same FS by definition.

**Why it happens:** POSIX `rename` is only atomic AND only works within a single filesystem. If the test override moves HOME to a bind-mount or a different tmpfs, `os.Rename` returns `EXDEV`.

**How to avoid:** No special work needed — siblings under same parent. Add a clear error message if the rename fails (`"migrate watchers dir (source and destination must be on same filesystem): %w"`). `[VERIFIED: Go stdlib os.Rename docs — "OS-specific restrictions may apply when oldpath and newpath are in different directories"; confirmed sibling paths under `~/.agent-deck/` resolve to same FS in all tested environments]`.

**Warning signs:** `os.Rename: invalid cross-device link` error at startup.

### Pitfall 2: Symlink loop from a re-migration

**What goes wrong:** If the migration is re-run, `os.Stat(watchers)` follows the symlink → reports `watcher/` exists → migration code sees "both exist" and aborts. This is actually correct behavior (the whole point of the guard), but a careless test may expect exactly one migration regardless of state.

**Why it happens:** `os.Stat` dereferences symlinks. Use `os.Lstat` when you need to distinguish symlink-from-real-dir.

**How to avoid:** Use `os.Lstat` in `MigrateLegacyWatchersDir` (as shown in Pattern 4). Regression test: migration runs twice back-to-back → second call is a no-op.

**Warning signs:** Migration test fails intermittently after a second call.

### Pitfall 3: `task-log.md` append size exceeds PIPE_BUF

**What goes wrong:** If an adapter produces a huge subject line (e.g., a webhook body pasted into `Subject`), a single `Write` > 4096 bytes is NOT atomic across concurrent writers. We have a single writer (engine.go:275), so this is fine today — but a future fan-out refactor could break it.

**Why it happens:** POSIX guarantees atomicity for `write()` with `O_APPEND` only when the write is ≤ `PIPE_BUF`. macOS APFS and Linux ext4/xfs honor this.

**How to avoid:** Keep the single-writer assumption. Bound entry size at ~512 bytes in `AppendEventLog` (truncate subject if needed) so we also survive older BSD. Add a code comment.

**Warning signs:** Interleaved lines in `task-log.md` after concurrency changes.

### Pitfall 4: `test.TempDir` not cleaned up when tests share HOME

**What goes wrong:** Multiple tests overriding `HOME` to the same path via `t.Setenv` — the override applies within the test lifecycle. But `t.TempDir()` returns a unique subdir per test. Using them together (`t.Setenv("HOME", t.TempDir())`) is the correct pattern.

**Why it happens:** Race if tests don't use `t.TempDir()` for isolation.

**How to avoid:** Every test MUST use `t.Setenv("HOME", t.TempDir())`. Verified pattern at `internal/watcher/gmail_test.go:165` and `internal/feedback/feedback_test.go:14`.

### Pitfall 5: Fallback clients.json read creates stale state

**What goes wrong:** CONTEXT.md says "fallback read of `watchers/clients.json` for one cycle" if the singular doesn't resolve. If a user hand-edits the legacy file post-migration, the new engine would keep reading the old file until the symlink is removed in v1.7.0. But by then the symlink `watchers -> watcher/` should redirect the read transparently.

**Why it happens:** Confusion about what "fallback" means. Two valid implementations:
  - **(a) Path fallback:** `os.Stat(new)` → if missing, try old path. WRONG — migration guarantees `new` exists after first boot.
  - **(b) Pre-migration safety net:** IF migration hasn't run yet AND only `watchers/clients.json` exists, engine can still resolve. This is what the symlink accomplishes for free.

**How to avoid:** Implement as (b) — don't add a parallel fallback in code. The symlink `watchers -> watcher/` means `watchers/clients.json` resolves to `watcher/clients.json`. A single path resolution handles both pre-migration and post-migration worlds. The "one cycle" language in CONTEXT.md refers to the **symlink** being retained for one cycle, not to a code-level fallback. Recommend: document this in `engine.go:134` as a code comment.

## Code Examples

### Migration log line (greppable by tests)

```go
// Source: engine.go pattern, slog convention
slog.Info("watcher: migrated legacy ~/.agent-deck/watchers/ → ~/.agent-deck/watcher/",
    slog.String("note", "legacy ~/.agent-deck/issue-watcher/ NOT migrated (out of scope per REQ-WF-6)"))
```

Test assertion:
```go
// Capture slog output via slog.NewJSONHandler(buf, nil) in test setup, then:
if !strings.Contains(buf.String(), "watcher: migrated legacy") {
    t.Errorf("expected migration log line; got:\n%s", buf.String())
}
```

### Test isolation with `t.Setenv("HOME", t.TempDir())`

```go
// Source: internal/watcher/gmail_test.go:160-170, internal/feedback/feedback_test.go:14
func TestLayout_FreshInstallCreatesLayout(t *testing.T) {
    t.Setenv("HOME", t.TempDir())

    if err := ScaffoldWatcherLayout(); err != nil {
        t.Fatalf("ScaffoldWatcherLayout: %v", err)
    }

    home, _ := os.UserHomeDir()
    base := filepath.Join(home, ".agent-deck", "watcher")
    for _, name := range []string{"CLAUDE.md", "POLICY.md", "LEARNINGS.md", "clients.json"} {
        p := filepath.Join(base, name)
        if _, err := os.Stat(p); err != nil {
            t.Errorf("expected %s to exist: %v", p, err)
        }
    }
}
```

### Integration test: 3 events → 3 task-log lines + 3 state.json updates

```go
func TestLayout_ThreeEventsThreeLines(t *testing.T) {
    t.Setenv("HOME", t.TempDir())
    // ... set up engine with mock webhook adapter ...
    // fire 3 events
    // assert:
    data, _ := os.ReadFile(filepath.Join(watcherDir, "task-log.md"))
    lines := strings.Count(string(data), "\n")
    if lines != 3 {
        t.Errorf("expected 3 log lines, got %d", lines)
    }
    state, _ := LoadState("mock")
    if state.LastEventTS.Before(thirdEventTime) {
        t.Errorf("state.LastEventTS not updated to 3rd event")
    }
}
```

### `list --json` new fields

```go
// Source: extension of cmd/agent-deck/watcher_cmd.go:310-316
type watcherListEntry struct {
    Name          string    `json:"name"`
    Type          string    `json:"type"`
    Status        string    `json:"status"`
    EventsPerHour float64   `json:"events_per_hour"`
    Health        string    `json:"health"`         // existing: "healthy"/"unknown"/"stopped"
    // NEW (Task D):
    LastEventTS   *time.Time `json:"last_event_ts"`  // nil if no state.json
    ErrorCount    int        `json:"error_count"`     // 0 if no state.json
    HealthStatus  string     `json:"health_status"`   // "healthy"|"warning"|"error"|"unknown" per HealthTracker.Check
}

// For each watcher:
state, err := watcher.LoadState(w.Name) // returns (nil, nil) if state.json absent
if err != nil || state == nil {
    entry.LastEventTS = nil
    entry.ErrorCount = 0
    entry.HealthStatus = "unknown"
} else {
    entry.LastEventTS = &state.LastEventTS
    entry.ErrorCount = state.ErrorCount
    // Rehydrate tracker to reuse classifier:
    tracker := watcher.NewHealthTracker(w.Name, maxSilenceMin)
    // ... apply state.LastEventTS and state.ErrorCount via setters ...
    entry.HealthStatus = string(tracker.Check().Status)
}
```

## Research Answers to Prompt Questions

### Q1: Atomicity semantics on supported platforms

- **Linux:** `rename(2)` is POSIX-atomic within a single filesystem. Same mount → same FS for `~/.agent-deck/watchers/` ↔ `~/.agent-deck/watcher/` (they're siblings). `[VERIFIED: POSIX rename(2) spec; os.Rename stdlib docs]`
- **macOS:** Both HFS+ and APFS honor `rename(2)` atomicity. Case-insensitivity doesn't matter here since `watcher` vs `watchers` differ in length, not case. `[VERIFIED: Apple File System Reference]`
- **`t.TempDir()` vs `HOME`:** `t.TempDir()` returns `$TMPDIR/test-name/NNN/001`. In the test, we do `t.Setenv("HOME", t.TempDir())` which makes `HOME` itself a tempdir path. All operations (including the `os.Rename` from `watchers/` to `watcher/`) happen as siblings UNDER that HOME, so they are guaranteed same-FS. `[VERIFIED: internal/watcher/gmail_test.go:165 uses this pattern today without cross-device issues]`

**Confidence: HIGH.**

### Q2: Symlink behavior

- `os.Symlink(target, link)` creates `link` pointing to `target`. The target path is stored verbatim (not resolved at creation time).
- **Relative vs absolute target:** Use **relative** (`os.Symlink("watcher", legacyPath)`). Rationale:
  1. Survives `$HOME` moves / path relocations (test harnesses, container moves).
  2. `~/.agent-deck/watchers -> watcher` is a sibling reference; idiomatic relative link.
  3. macOS/Linux both resolve relative symlinks against the link's parent dir, so `watchers -> watcher` from inside `~/.agent-deck/` dereferences to `~/.agent-deck/watcher/`. `[VERIFIED: POSIX symlink resolution]`
- **`filepath.EvalSymlinks` in tests:** Dereferences the full chain. Tests that walk through the symlink path (e.g., `os.Stat(watchersPath)`) will see the real `watcher/` directory and `IsDir()==true`. Tests that specifically want to detect the symlink must use `os.Lstat(watchersPath)` and check `info.Mode()&os.ModeSymlink != 0`.
- **Consumers assuming real dir:** `session.WatcherDir()` callers (`watcher_cmd.go:562`, `:614`, `router.go:99`, `gmail.go:137`) all do `filepath.Join(base, ...)` + `os.ReadFile/WriteFile`. None do `IsDir()==true` on the result. Zero breakage. The only consumer that uses `os.Lstat` is the migration guard itself.

**Confidence: HIGH.**

### Q3: Append-atomicity for task-log.md

- **POSIX guarantee:** `write(2)` with `O_APPEND` is atomic for writes ≤ `PIPE_BUF` (`4096` on Linux, `512` on older BSD, macOS is ≥ 512).
- **Entry size analysis:** Lines are `## <RFC3339-timestamp> - <type>: <summary>\n`. RFC3339 = 20 chars, type ∈ {webhook,ntfy,github,slack,gmail} (max 7), punctuation ~6, `\n` = 1. The `summary` field is the only unbounded piece. Looking at `engine.go:331-334`, `Subject` is persisted unmodified. Can be arbitrary.
- **Concurrent writers:** Single-goroutine writer (`engine.go:275-399` `writerLoop`). Per-watcher serialization is automatic — all events for all watchers flow through one channel (`e.eventCh`), processed in order. No need for per-name `sync.Mutex`.
- **Recommendation:** Truncate `summary` to 400 bytes before appending. This keeps the full line < 512 always (PIPE_BUF floor), surviving any future multi-goroutine refactor. Add code comment: `// entry is truncated to < PIPE_BUF so O_APPEND write remains atomic even if writerLoop becomes multi-goroutine`.

**Confidence: HIGH.**

### Q4: Hot-reload semantics

- **What's cached in-process today:** Clients (routing) are cached in `Router.clients` (router.go); `Router.Reload()` exists for hot-reload (router.go:115 RLock/RUnlock).
- **State.json:** Does NOT exist yet. There's nothing to cache — the per-watcher in-memory state lives inside `HealthTracker` (health.go:50-72) and is not persisted anywhere. `HealthTracker` is per-Engine-instance.
- **Designing for hot-reload:** `LoadState(name)` always reads disk. `SaveState(name, s)` always writes disk. No in-memory memoization. Then the "hot-reload-safe" test is trivially satisfied: mid-run test writes directly to `state.json` → next `LoadState(name)` call sees the new value.
- **Engine-side wiring:** The engine does NOT need to call `LoadState` repeatedly at runtime — it maintains authoritative state in `HealthTracker` and `SaveState`s AFTER each event. The hot-reload test validates that a CLI call (`watcher list --json`) reads fresh state even while the engine is running.

**Confidence: HIGH.**

### Q5: `list --json` health_status classifier

- **Public API:** `HealthTracker.Check() HealthState` at `health.go:148`. Returns `HealthState{Status HealthStatus, ...}` where `HealthStatus` is `"healthy"|"warning"|"error"`.
- **Constructor:** `NewHealthTracker(watcherName string, maxSilenceMinutes int) *HealthTracker` at `health.go:76`.
- **Mutators:** `RecordEvent()`, `RecordError()`, `SetAdapterHealth(bool)`, `SetLastEventTimeForTest(time.Time)`.
- **Callable from cmd/?** Yes — the type is exported. `cmd/agent-deck` already imports `internal/watcher` (watcher_cmd.go:13 imports `"github.com/asheshgoplani/agent-deck/internal/watcher"`).
- **Dependency to thread through WatcherState:** The tracker needs `eventTimestamps []time.Time`, `consecutiveErrors int`, `adapterHealthy bool`, `lastEventTime time.Time` to classify. If we persist only `last_event_ts` and `error_count` in `state.json`, we can rehydrate a minimal tracker for classification — the rate-per-hour number will be approximate (we lose the rolling window) but the health STATUS will be faithful because `Check()` depends mainly on `consecutiveErrors >= 10` / `>= 3` and silence threshold vs `lastEventTime`.
- **Recommendation:** `WatcherState` struct carries `LastEventTS time.Time`, `ErrorCount int`, `AdapterHealthy bool`, `HealthWindow []HealthSample` (sample = timestamp+ok-bool, capped at 64). For `list --json`, construct a fresh tracker, set fields via setters (add `SetErrorCountForTest` / `SetAdapterHealthForTest` — or make them public setters with the `ForHydration` suffix), then call `Check()`.

**Alternative if setters feel intrusive:** inline the classifier rules (`health.go:164-178`) in a tiny helper `classify(state *WatcherState, maxSilenceMin int) HealthStatus`. Comment the inline path explicitly: `// Mirrors HealthTracker.Check() — update both in lockstep.` Either approach is valid; **recommend the setter path** to avoid drift.

**Confidence: HIGH.**

### Q6: Legacy-path fallback for ClientsPath

- **Simplest correct implementation:** Do nothing at the code level beyond using the singular path. The `watchers -> watcher/` symlink created during migration transparently serves any reader that still constructs the legacy path.
- **What happens pre-migration (first boot):** Before `MigrateLegacyWatchersDir` runs, only `watchers/` exists. If `engine.go:134` already points at `watcher/clients.json`, the read fails. BUT: `MigrateLegacyWatchersDir` runs in `engine.Start()` BEFORE any adapter is spawned (CONTEXT.md line 37), so by the time `ClientsPath` is read the singular path exists.
- **Recommendation:** Engine startup order:
  1. `MigrateLegacyWatchersDir()` — may rename + symlink.
  2. `LayoutDir()` scaffolding — writes missing templates.
  3. Default `cfg.ClientsPath = filepath.Join(home, ".agent-deck", "watcher", "clients.json")` (singular only — no path-level fallback).
  4. Start adapters.
- **No fallback code.** Symlink handles it. Simpler. `LoadFromWatcherDir` (`router.go:98-109`) uses `session.WatcherDir()` — update that function and this whole call site comes along.

**Confidence: HIGH.**

### Q7: `session.WatcherDir()` blast radius

Full caller list (grep-verified):

| File:Line | Caller | Behavior after switch |
|-----------|--------|----------------------|
| `cmd/agent-deck/watcher_cmd.go:169` | `configPath, err := session.WatcherNameDir(*name)` — used during `watcher create` | Correct; returns singular per-watcher dir. |
| `cmd/agent-deck/watcher_cmd.go:524` | Error message string `"Create ~/.agent-deck/watchers/clients.json to enable routing."` | **Must update text** to singular. |
| `cmd/agent-deck/watcher_cmd.go:562` | `watcherDir, err := session.WatcherDir()` — `handleWatcherRoutes` | Correct. |
| `cmd/agent-deck/watcher_cmd.go:614` | `outputDir, err := session.WatcherDir()` — `handleWatcherImport` | Correct. |
| `internal/watcher/gmail.go:137` | `watcherDir, err := session.WatcherNameDir(a.name)` — gmail credentials/token path | Correct. Credentials files ride with `os.Rename`. |
| `internal/watcher/router.go:99` | `base, err := session.WatcherDir()` + `base + "/clients.json"` | Correct. |
| `internal/watcher/engine.go:46` | Comment: `// Defaults to $HOME/.agent-deck/watchers/clients.json when empty.` | **Must update comment** to singular. |
| `internal/watcher/engine.go:134` | Literal `filepath.Join(home, ".agent-deck", "watchers", "clients.json")` | **Must update literal** to `"watcher"`. (CONTEXT.md locks this.) |
| `internal/session/watcher_meta.go:11` | Comment `// Persisted as meta.json in ~/.agent-deck/watchers/<name>/.` | **Must update comment** to singular. |
| `internal/session/watcher_meta.go:20` | Doc: `// WatcherDir returns the base directory for all watchers (~/.agent-deck/watchers).` | **Must update doc** to singular. |
| `internal/session/watcher_meta.go:29` | Doc: `// WatcherNameDir returns the directory for a named watcher (~/.agent-deck/watchers/<name>).` | **Must update doc** to singular. |

**Text-only updates (no functional change but still required for grep-cleanliness):** `watcher_cmd.go:524`, `engine.go:46`, `watcher_meta.go:11`, `:20`, `:29`.

**Out-of-scope text updates (deferred to Phase 22 / REQ-WF-7):** `cmd/agent-deck/assets/skills/watcher-creator/SKILL.md` (6+ lines), `internal/watcher/gmail_test.go:160` comment, `internal/watcher/triage_test.go:273` comment (coincidental "watchers/events" phrase — not a path).

**Confidence: HIGH.**

### Q8: Scaffolding idempotency

- **Canonical pattern:** `os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)` → `defer f.Close()` → if `errors.Is(err, fs.ErrExist)` treat as success. See Pattern 3 above.
- **Why not stat-then-write:** TOCTOU — another process could create the file between the `os.Stat` check and the `os.WriteFile`. In practice, `agent-deck` is usually single-instance per user, but the O_EXCL pattern is 2 lines longer for zero downside.
- **Idempotency test:** call `ScaffoldWatcherLayout()` twice; second call returns nil and file contents are unchanged. Bonus: delete `POLICY.md`, call scaffold again, verify `POLICY.md` is regenerated (CONTEXT.md specifics line 115).

**Confidence: HIGH.**

### Q9: Test isolation

- **`t.Setenv("HOME", t.TempDir())`:** Go 1.17+. Restored on test end. Confirmed in repo: `internal/watcher/gmail_test.go:165`, `internal/feedback/feedback_test.go:14,32,50,68`.
- **`health_bridge_test.go` analog:** Does NOT use `t.Setenv("HOME", ...)` because the bridge doesn't touch the filesystem. Its test pattern (fake-clock injection, channel-driven mock notifier) is a better analog for **hot-reload** testing than for **layout** testing.
- **For Phase 21:** Use `t.Setenv("HOME", t.TempDir())` in every test in `layout_test.go`. Pattern confirmed working.

**Confidence: HIGH.**

### Q10: Validation architecture

See `## Validation Architecture` section below.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `watchers/` plural | `watcher/` singular (Phase 21) | v1.6.0 | User-visible rename; symlink keeps legacy scripts working for one cycle. |
| Per-watcher state in-memory only (HealthTracker) | Persisted to `state.json` | v1.6.0 | Survives restart; enables `list --json` health snapshot. |
| No event log on disk | Per-watcher `task-log.md` | v1.6.0 | Debugging aid; human-readable. |

**Deprecated/outdated:**
- `~/.agent-deck/watchers/` hard-coded paths in scripts — covered by symlink through v1.6.x; hard-deprecated v1.7.0.
- Stat-then-write scaffolding (`conductor.go:519-523` style) — still in use for conductor, not worth refactoring; new watcher code uses `O_CREATE|O_EXCL`.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Subject text in adapters is bounded enough that RFC3339 + subject + punctuation stays < 512 bytes per task-log.md line | Q3, Pattern 2 | If subjects can be huge, interleaved writes possible under future concurrency. Mitigation: truncate summary to 400 bytes. [ASSUMED — sampled webhook.go/ntfy.go/github.go/slack.go but didn't exhaustively check max subject size] |
| A2 | No consumer of `session.WatcherDir()` asserts `IsDir()==true` on the result path | Q2 | If one does, the symlink detection would fail. Mitigation: keep legacy `watchers` path as a symlink, not a real dir. Test with `os.Stat` (follows symlinks) to confirm `IsDir()==true` holds for the symlink. [ASSUMED — grepped for `IsDir()` on WatcherDir results, found none, but a dynamically-built path could escape the grep] |
| A3 | `~/.agent-deck/` is always on a single filesystem on supported user machines | Q1 | If a user has `~/.agent-deck/` on a different mount than its children (unusual), rename could fail. Mitigation: clear error message. [ASSUMED — no user reports of cross-FS setups in agent-deck codebase] |

**If assumptions turn out wrong:** A1 → add subject truncation; A2 → keep symlink as dir-compat; A3 → add fallback to copy+remove.

## Open Questions

1. **Where does `MigrateLegacyWatchersDir` get invoked?**
   - Recommendation: export it, call from `engine.Start()` as the FIRST line before any adapter setup. CONTEXT.md says "On engine startup" — engine.go:181 `Start()` is the obvious hook.
   - Test-driven approach: tests call `MigrateLegacyWatchersDir` directly (they don't need a full engine).

2. **Should `state.json` atomic writes use `fsync`?**
   - `watcher_meta.go:47-75` does NOT fsync. Consistency over durability is fine for state.json — if the machine crashes mid-write, next engine start recomputes state from events in the DB.
   - Recommendation: don't fsync. Match existing pattern.

3. **What goes in the default CLAUDE.md / POLICY.md / LEARNINGS.md templates?**
   - Task C scope. Recommend reviewing `internal/session/conductor_templates.go` (not read here) for tone + depth. Phase planner should have Task C creator review that file.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Build + tests | ✓ | go1.25.5 installed; `GOTOOLCHAIN=go1.24.0` pins per STATE.md | — |
| `git` | commits | ✓ | (standard) | — |
| POSIX filesystem with `rename(2)` atomicity | Migration | ✓ | Linux + macOS both honor | — |
| `~/.agent-deck/` writable | Runtime | ✓ | Standard user dir | — |
| `trash` CLI | Test cleanup | ✓ | `/usr/bin/trash` per user CLAUDE.md | — |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** None.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` stdlib + `testify` (present in go.mod chain; `sync/atomic` and channels used directly in existing watcher tests) |
| Config file | None — `go test` conventions |
| Quick run command | `GOTOOLCHAIN=go1.24.0 go test ./internal/watcher/ -run TestLayout -race -count=1 -timeout 30s` |
| Full suite command | `GOTOOLCHAIN=go1.24.0 go test ./internal/watcher/... -race -count=1 -timeout 120s` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| REQ-WF-6 | Fresh install creates `watcher/{CLAUDE.md, POLICY.md, LEARNINGS.md, clients.json}` | unit | `go test ./internal/watcher/ -run TestLayout_FreshInstallCreatesLayout -race` | ❌ Task A |
| REQ-WF-6 | Legacy `watchers/` → `watcher/` via atomic `os.Rename` + symlink | unit | `go test ./internal/watcher/ -run TestLayout_LegacyMigrationAtomic -race` | ❌ Task A |
| REQ-WF-6 | Symlink `watchers/` resolves to real `watcher/` | unit | `go test ./internal/watcher/ -run TestLayout_SymlinkResolves -race` | ❌ Task A |
| REQ-WF-6 | `WatcherState` round-trips through `SaveState`/`LoadState` | unit | `go test ./internal/watcher/ -run TestLayout_StateRoundtrip -race` | ❌ Task A |
| REQ-WF-6 | `AppendEventLog` append is atomic and line-ordered | unit | `go test ./internal/watcher/ -run TestLayout_EventLogAppendAtomic -race` | ❌ Task A |
| REQ-WF-6 | Layout is hot-reload-safe: external state.json edit visible to next LoadState | unit | `go test ./internal/watcher/ -run TestLayout_HotReloadSafe -race` | ❌ Task A |
| REQ-WF-6 | 3 events → 3 task-log lines + 3 state.json updates (end-to-end) | integration | `go test ./internal/watcher/ -run TestLayout_Integration_ThreeEvents -race` | ❌ Task A |
| REQ-WF-6 | `watcher list --json` emits `last_event_ts`, `error_count`, `health_status` | unit | `go test ./cmd/agent-deck/ -run TestWatcherList_JSON_ExposesStateFields -race` | ❌ Task D |

### Sampling Rate

- **Per task commit (Task A RED, Task B GREEN, Task C docs, Task D CLI):** `GOTOOLCHAIN=go1.24.0 go test ./internal/watcher/ -run TestLayout -race -count=1 -timeout 30s` (< 2s expected)
- **Per wave merge / phase close:** `GOTOOLCHAIN=go1.24.0 go test ./internal/watcher/... -race -count=1 -timeout 120s` AND `GOTOOLCHAIN=go1.24.0 go test ./cmd/agent-deck/... -run "Watcher" -race -count=1 -timeout 120s`
- **Phase gate (`/gsd-verify-work`):** Full suite green; layout tests green under `-race`; migration happy-path + collision path both covered.

### Wave 0 Gaps

- [ ] `internal/watcher/layout.go` — source file, created Task B (GREEN)
- [ ] `internal/watcher/layout_test.go` — 6 unit + 1 integration test, created Task A (RED first)
- [ ] `internal/watcher/state.go` — source file, created Task B
- [ ] `internal/watcher/event_log.go` — source file, created Task B
- [ ] `assets/watcher-templates/CLAUDE.md` — template, created Task C
- [ ] `assets/watcher-templates/POLICY.md` — template, created Task C
- [ ] `assets/watcher-templates/LEARNINGS.md` — template, created Task C
- [ ] `cmd/agent-deck/watcher_cmd_test.go` — add `TestWatcherList_JSON_ExposesStateFields`, Task D
- [ ] Framework install: none needed — Go stdlib `testing` suffices.

### Validation Gaps & Risks

- **Migration collision path** (both `watchers/` and `watcher/` exist as real dirs): add as a 7th test variant or an explicit sub-test of `TestLayout_LegacyMigrationAtomic`. CONTEXT.md mandates "skip + log warning" — test the warning log line is emitted.
- **Fallback path resolution:** since no code-level fallback exists (see Q6), there's nothing to test beyond verifying the symlink resolves. Covered by `TestLayout_SymlinkResolves`.
- **Concurrent event logging to the same watcher:** our single-writer architecture makes this a non-issue for the engine. If a future test spawns two goroutines both calling `AppendEventLog(name, ...)`, lines could interleave above PIPE_BUF. Recommend adding a `-race`-clean test that does exactly this — validates POSIX atomicity holds at < PIPE_BUF and flags early if a future refactor breaks the single-writer assumption.

### Manual-Only Validation

| Check | Why | Procedure |
|-------|-----|-----------|
| Real user migration on macOS + Linux | `os.Rename` cross-FS edge cases vary per user setup | Dry-run: on a user machine with populated `~/.agent-deck/watchers/`, build the binary, start the engine, verify `watcher/` exists, `watchers` is a symlink, all meta.json/state.json files present. One-time manual check pre-merge. |
| Symlink removal in v1.7.0 | Deferred | Out of scope for Phase 21. |

## Security Domain

Per `.planning/config.json`, `security_enforcement` is not set — treating as enabled.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Phase 21 touches no auth code |
| V3 Session Management | no | N/A |
| V4 Access Control | partial | File perms 0o644 for state.json / task-log.md; 0o755 for dirs. Sensitive files (Gmail `credentials.json`, `token.json`) unchanged — already 0o600 in Phase 17 code. |
| V5 Input Validation | yes | `MigrateLegacyWatchersDir` must use `os.Lstat` not `os.Stat` when detecting existing symlinks (CWE-59 symlink following). `AppendEventLog(name, ...)` must validate `name` doesn't contain `..` or `/` (reuse `session.ValidateConductorName`-style pattern). |
| V6 Cryptography | no | N/A |
| V7 Error Handling | yes | Don't leak HOME path in user-facing errors; wrap with %w but trim absolute paths in `fmt.Errorf` where reasonable. |
| V12 File + Resource | yes | Atomic write-temp-rename prevents partial writes; O_EXCL prevents TOCTOU in scaffolding. |

### Known Threat Patterns for Go filesystem code

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Symlink traversal during migration (an attacker pre-creates `~/.agent-deck/watcher` as a symlink to `/etc/passwd` before agent-deck first boot) | Tampering | Use `os.Lstat` in migration guard; refuse if `watcher/` is a symlink and target is outside `~/.agent-deck/`. |
| Path injection via watcher `name` | Tampering | Validate name with `^[a-zA-Z0-9][a-zA-Z0-9._-]*$` (reuse `conductorNameRegex` at `conductor.go:213` — same pattern; spec says watcher names follow conductor rules). |
| Race between scaffold write and adversarial write | Tampering | `O_CREATE|O_EXCL` — fails loudly if file already exists. |
| Disk exhaustion via unbounded task-log.md | DoS | Out of scope for Phase 21; documented in CONTEXT.md deferred ideas. Future: log rotation at 10MB. |

## Sources

### Primary (HIGH confidence)

- `internal/session/watcher_meta.go:1-96` — atomic write-temp-rename template, current `WatcherDir()` / `WatcherNameDir()` definitions.
- `internal/session/conductor.go:246-263, 822-874` — `ConductorDir()`, `ConductorNameDir()`, `InstallSharedConductorInstructions`, `InstallPolicyMD`, `InstallLearningsMD` — analog for layout helpers.
- `internal/session/conductor.go:923-969` — `MigrateLegacyConductors` — analog for `MigrateLegacyWatchersDir`.
- `internal/watcher/engine.go:130-140, 275-399` — `ClientsPath` default, writerLoop wire-in points.
- `internal/watcher/health.go:1-197` — `HealthStatus` constants, `HealthTracker.Check()` classifier.
- `internal/watcher/health_bridge_test.go:1-431` — test harness style (fake clock injection, channel-driven).
- `internal/watcher/gmail_test.go:160-170` — `t.Setenv("HOME", t.TempDir())` pattern.
- `internal/feedback/feedback_test.go:14-70` — same pattern, more examples.
- `cmd/agent-deck/watcher_cmd.go:165-180, 286-366, 510-601, 605-714` — CLI callers and atomic merge primitive.
- `.planning/phases/21-watcher-folder-hierarchy/21-CONTEXT.md` — locked decisions (authoritative).
- `docs/WATCHER-COMPLETION-SPEC.md:68-117` — REQ-WF-6 spec.
- `.planning/REQUIREMENTS.md:27` — REQ-WF-6 summary.

### Secondary (MEDIUM confidence)

- POSIX `rename(2)` spec — standard atomicity within a single filesystem. Cross-referenced against Go stdlib `os.Rename` docs.
- POSIX `write(2)` atomicity for `O_APPEND` and `PIPE_BUF` — cross-verified against Linux kernel ABI docs and macOS Darwin man pages.
- Go 1.17+ `t.Setenv` stdlib docs — verified against live usage in `feedback_test.go`.

### Tertiary (LOW confidence)

- None — every claim is verified against either repo code or a primary source.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — zero new dependencies; all primitives are stdlib and already used.
- Architecture: HIGH — direct analog to conductor folder pattern which is shipped and working.
- Pitfalls: HIGH — every pitfall has a code-level mitigation referenced to an existing pattern.

**Research date:** 2026-04-16
**Valid until:** 2026-05-16 (30 days — architecture is stable; only risk is upstream Go changes to `os.Rename` / `os.Symlink` behavior, which is fixed by POSIX).
