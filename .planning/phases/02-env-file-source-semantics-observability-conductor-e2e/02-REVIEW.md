---
phase: 02-env-file-source-semantics-observability-conductor-e2e
reviewed: 2026-04-15T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - internal/session/claude.go
  - internal/session/instance.go
  - internal/session/pergroupconfig_test.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 02: Code Review Report

**Reviewed:** 2026-04-15
**Depth:** standard
**Files Reviewed:** 3
**Status:** issues_found

## Summary

Phase 02 lands three small, well-scoped changes on top of phase 01:

1. CFG-03 fix: custom-command spawn path now prepends `buildEnvSourceCommand()` + `buildBashExportPrefix()` before the wrapper payload (instance.go:599).
2. CFG-07 observability: `GetClaudeConfigDirSourceForGroup` (claude.go:305) plus `logClaudeConfigResolution` helper (instance.go:623) gated on three Start/Restart call sites (instance.go:1955, :2079, :4118).
3. Regression tests covering the env_file spawn-source path, conductor restart, source-label priority, and slog rendered format.

Overall assessment: the production-path changes are minimal and surgically placed. The new helper correctly mirrors `GetClaudeConfigDirForGroup`'s priority chain, and the test suite exercises all five priority levels including the missing-file guard. The shared-source `buildSourceCmd` (env.go:155) pre-existed and is reused unchanged; it is the right call site for the fix.

The issues below are mostly pre-existing shell-quoting concerns amplified by the new custom-command branch and one maintenance liability from the intentional priority-chain duplication. No correctness or security regressions introduced by this phase, but the quoting concerns warrant a follow-up.

## Warnings

### WR-01: `buildSourceCmd` embeds user-controlled path inside double-quoted shell string without escaping

**File:** `internal/session/env.go:152-158`
**Issue:** The helper wraps the resolved env_file path in double quotes: `[ -f "%s" ] && source "%s"` / `source "%s"`. Inside a double-quoted shell context, `$`, `` ` ``, `\`, and `"` retain their special meaning. A config.toml entry like

```toml
[groups."g".claude]
env_file = "/tmp/x`id`"
```

would execute the backtick expression at spawn time. Even without malice, legitimate paths containing `$` (e.g., deliberate `$HOME`-style templating users may not realize has already been expanded, or Windows-style paths via WSL) can silently expand to something unexpected. Phase 02 does not introduce this code, but commit e608480 routes a NEW code path (the custom-command branch) through it, widening the blast radius.

The comparable inline-env-var escaping at env.go:231 already single-quote-escapes values; the file-path variant should be brought to parity.

**Fix:**
```go
// env.go:152 - escape embedded double quotes and literal dollar/backtick,
// or (better) single-quote the path so no expansion occurs:
func buildSourceCmd(path string, ignoreMissing bool) string {
    // Single-quote is the safest shell quoting: no expansion, no escapes.
    // Escape embedded single quotes via the '\'' dance.
    q := "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
    if ignoreMissing {
        return fmt.Sprintf(`[ -f %s ] && source %s`, q, q)
    }
    return fmt.Sprintf(`source %s`, q)
}
```

Note this is a trust-boundary question: config.toml is written by the user, so the threat model is mostly "typos and weird paths" rather than RCE. Still worth fixing while the change is fresh.

---

### WR-02: `CLAUDE_CONFIG_DIR=%s` export is unquoted; breaks on paths containing spaces

**File:** `internal/session/instance.go:503` and `internal/session/instance.go:607`
**Issue:** Both emission sites build the export as `fmt.Sprintf("CLAUDE_CONFIG_DIR=%s ", configDir)` / `fmt.Sprintf("export CLAUDE_CONFIG_DIR=%s; ", ...)`. If `configDir` contains a space (legitimate on macOS where home paths like `/Users/Joe Smith/.claude-work` exist), the shell will split the assignment at the first space, assigning only the prefix to `CLAUDE_CONFIG_DIR` and executing the remainder as a command. For the inline `VAR=x command` form at line 503, this silently degrades (the env var is set for the prefix only, rest becomes argv); for the `export …;` form at line 607, it yields a syntax error and the spawn fails.

Phase 02 did not introduce this call pattern, but commit e608480 added a new call site (line 599 chains `buildBashExportPrefix()` which calls L607) on the custom-command path, so any wrapper-script user with a spaced home dir is newly exposed.

**Fix:**
```go
// instance.go:503
configDirPrefix = fmt.Sprintf("CLAUDE_CONFIG_DIR=%q ", configDir)

// instance.go:607
prefix += fmt.Sprintf("export CLAUDE_CONFIG_DIR=%q; ", GetClaudeConfigDirForGroup(i.GroupPath))
```

`%q` in Go's fmt yields a double-quoted, shell-compatible rendering for typical paths. For paths containing `$` or backticks, prefer explicit single-quoting as in WR-01.

---

### WR-03: `GetClaudeConfigDirSourceForGroup` duplicates the resolver chain; drift risk

**File:** `internal/session/claude.go:305-326`
**Issue:** The new helper copies the exact priority chain from `GetClaudeConfigDirForGroup` at L246-267. The comment at L302-304 explicitly calls this out ("keep the two functions in sync if the chain ever changes"), which is an acknowledgment of the maintenance hazard, not a mitigation. A future contributor who adds a new priority level (e.g., a session-level override) to only one of the two functions will ship a bug where the source label disagrees with the resolved path — exactly the class of bug CFG-07 exists to surface.

**Fix:** Refactor `GetClaudeConfigDirForGroup` to delegate to the source-aware variant:

```go
func GetClaudeConfigDirForGroup(groupPath string) string {
    path, _ := GetClaudeConfigDirSourceForGroup(groupPath)
    return path
}
```

This collapses the two chains to one. Existing callers of `GetClaudeConfigDirForGroup` continue to work. `IsClaudeConfigDirExplicitForGroup` at L271 can similarly be rewritten as `source != "default"`.

---

## Info

### IN-01: `logClaudeConfigResolution` emits only on successful tmux Start

**File:** `internal/session/instance.go:1954, 2079, 4118`
**Issue:** All three gated call sites invoke `logClaudeConfigResolution` AFTER the `tmuxSession.Start(command)` succeeds. If Start errors (e.g., tmux not running, command invalid, wrapper script not executable), the observability line never emits, making failed-start diagnostics harder. For CFG-07's stated goal ("document which priority level resolved CLAUDE_CONFIG_DIR"), users investigating a failed start would benefit from seeing the resolution line even when Start fails.

**Fix:** Move `logClaudeConfigResolution()` to just before `tmuxSession.Start()` so the resolution is logged regardless of Start outcome. Consider whether the helper should also log at a higher level (Warn) when source=="default" but the tool is Claude-compatible — a missed config case.

---

### IN-02: Log injection technically possible if slog handler is swapped to one without quoting

**File:** `internal/session/instance.go:625-629`
**Issue:** `slog.String("group", i.GroupPath)` and `slog.String("session", i.ID)` are safe under the default `slog.NewTextHandler` because TextHandler quotes values containing whitespace, newlines, or `=`. However, CFG-07's contract is the rendered log format (tested at pergroupconfig_test.go:652), and if future code swaps the handler (e.g., to a JSON handler for structured logs, or a custom handler with different quoting rules), logfmt injection via a crafted GroupPath containing `\nsource=env resolved=/evil` becomes possible.

The test at pergroupconfig_test.go:634-637 hard-codes a TextHandler; the production `logging.ForComponent(CompSession)` handler should be audited to confirm it also quotes correctly. `i.ID` is generated internally (safe), `i.GroupPath` and `resolvedPath` derive from config.toml and user input respectively.

**Fix:** Validate `i.GroupPath` at ingestion (reject newlines, control chars) in NewInstanceWithGroupAndTool, so logging can rely on sanitized input regardless of handler. Add a comment at the log call site documenting the handler assumption.

---

### IN-03: Runtime-proof harness in test is fragile to builder changes

**File:** `internal/session/pergroupconfig_test.go:324-340`
**Issue:** Assertion C locates the payload with `strings.LastIndex(cmdCustom, "bash -c 'exec claude'")` and splices in `echo "$TEST_ENVFILE_VAR"`. If `buildClaudeCommandWithMessage` ever changes its quoting (e.g., switches single-to-double quotes, adds escaping, or wraps the custom command differently), the LastIndex lookup returns -1 and the test fatals with a confusing "could not locate custom-command payload" message rather than the real semantic failure.

**Fix:** Assert on `buildEnvSourceCommand()` in isolation for the runtime proof, or construct `inst.Command` as a known-unique sentinel (e.g., `"__AGENTDECK_TEST_PAYLOAD__"`) that can be located unambiguously regardless of quoting. The test would then verify env sourcing executes before payload without coupling to the current quoting strategy.

---

### IN-04: `IsClaudeConfigDirExplicitForGroup` also duplicates the chain

**File:** `internal/session/claude.go:271-291`
**Issue:** Related to WR-03: `IsClaudeConfigDirExplicitForGroup` open-codes the same priority traversal a third time, checking only for presence. After the WR-03 refactor, this becomes:

```go
func IsClaudeConfigDirExplicitForGroup(groupPath string) bool {
    _, source := GetClaudeConfigDirSourceForGroup(groupPath)
    return source != "default"
}
```

**Fix:** Fold into WR-03's consolidation.

---

_Reviewed: 2026-04-15_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
