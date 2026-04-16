# Phase 1: Custom-command injection + core regression tests — Research

**Researched:** 2026-04-15
**Domain:** Go / `internal/session` — TDD tests and (likely) a minimal surgical fix for `CLAUDE_CONFIG_DIR` export on custom-command sessions.
**Confidence:** HIGH — every load-bearing claim below is a direct file:line reference into this worktree at base `fa9971e`.

---

## User Constraints (from CONTEXT.md / ROADMAP.md / REQUIREMENTS.md)

### Locked Decisions

- TDD is non-negotiable. Tests go in `internal/session/pergroupconfig_test.go` (new file) and MUST fail before any production change.
- Additive-only vs PR #578 (`fa9971e` by @alec-pinson). Do NOT revert or refactor `claude.go`/`userconfig.go`/`instance.go`/`env.go` already-merged logic. Only surgical fixes forced by a failing test.
- Phase 1 covers CFG-01 (verify only), CFG-02, CFG-04 tests 1 / 2 / 3 / 6.
- Commit trailer MUST sign "Committed by Ashesh Goplani". No Claude attribution strings anywhere (`🤖 Generated with Claude Code`, `Co-Authored-By: Claude`, etc.).
- NEVER pass `--no-verify` to `git commit`. Pre-commit hook is wired via lefthook (`.git/hooks/pre-commit` → `lefthook run pre-commit` — see `lefthook.yml:25-33`: runs `gofmt -l` check + `go vet ./...`).
- Use `trash` instead of `rm` for any cleanup.
- No `git push`, `git tag`, `gh pr create`, `gh pr merge`, `gh release`.
- If a surgical fix is needed, the fix-commit body MUST carry: `Builds on PR #578 by @alec-pinson.` (dedicated attribution commit is Phase 3; this is a per-commit reference if Phase 1 touches PR #578's files).

### Claude's Discretion

- Exact Go test body style (matching existing house style — see §2 below).
- Exact fix shape IF the custom-command export is confirmed missing — two options sketched in §1.5.
- Helper seam to assert on: the shell command string returned by `buildClaudeCommand(i.Command)`, using `strings.Contains` (that is the dominant assertion in `instance_test.go`).

### Deferred Ideas (OUT OF SCOPE — ignore)

- env_file sourcing semantics (CFG-03 / CFG-04 tests 4 & 5) — Phase 2.
- Observability log line (CFG-07) — Phase 2.
- Visual harness, README/CLAUDE.md/CHANGELOG docs, attribution commit — Phase 3.
- Any refactor of PR #578's `GetClaudeConfigDirForGroup` / `IsClaudeConfigDirExplicitForGroup` / `GetGroupClaudeConfigDir` / `GetGroupClaudeEnvFile`.
- Any `statedb` / SQLite change.
- Go toolchain work: pinned at 1.24.0 via `Makefile:13` (`export GOTOOLCHAIN=go1.24.0`). Don't touch.

---

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CFG-01 | PR #578 schema + lookup stays green | §3 confirms both existing tests exist and pass shape is unchanged. Phase 1 adds no assertion changes to them. |
| CFG-02 | Custom-command sessions get `CLAUDE_CONFIG_DIR` override | §1 shows the export path is **NOT live today** for `Instance.Command != ""` — a minimal fix is required. |
| CFG-04 test 1 (`CustomCommandGetsGroupConfigDir`) | The spawn command for a custom-command claude session contains `CLAUDE_CONFIG_DIR=<group's dir>` | §2.1 — test against `buildClaudeCommand(i.Command)` return string. |
| CFG-04 test 2 (`GroupOverrideBeatsProfile`) | Group override wins over profile override in the resolver | §2.2 — unit test on `GetClaudeConfigDirForGroup`. Likely green immediately (PR #578's `TestGetClaudeConfigDirForGroup_GroupWins` already proves this; we add a test asserting the spawn command uses the group value). |
| CFG-04 test 3 (`UnknownGroupFallsThroughToProfile`) | Unknown group → profile value | §2.3 — unit test on `GetClaudeConfigDirForGroup`. Green immediately via existing logic at `claude.go:253-258`. |
| CFG-04 test 6 (`CacheInvalidation`) | Change config on disk, call `ClearUserConfigCache()`, resolver returns new value | §2.4 — unit test. Green immediately via existing `ClearUserConfigCache` at `userconfig.go:1222`. |

---

## Summary

**Primary recommendation:** Write all four tests RED-first. Expect tests **3** and **6** (resolver-level) to go GREEN on first run against `fa9971e` without any code change. Expect tests **1** (`CustomCommandGetsGroupConfigDir`) and likely **2** (spawn-level group-beats-profile) to stay RED. The root cause is localized to `internal/session/instance.go:485-597` (`buildClaudeCommandWithMessage`): when `baseCommand != "claude"` (i.e. custom wrapper script), the function returns `baseCommand` unchanged at **line 596** without prepending `buildBashExportPrefix()`. The minimal surgical fix is to prepend `buildBashExportPrefix()` (which already handles `CLAUDE_CONFIG_DIR` unconditionally — `instance.go:601-607`) to the `baseCommand` return path.

Bottom line for the planner: **Phase 1 is test-first + a ~3-line surgical patch in one function**, not a pure test-authoring phase.

---

## 1. Custom-command export path — live or gap?

### 1.1 Where `buildBashExportPrefix` is defined

```
internal/session/instance.go:599-607
```

```go
// buildBashExportPrefix builds the export prefix used in bash -c commands.
// It always exports AGENTDECK_INSTANCE_ID, and conditionally adds CLAUDE_CONFIG_DIR.
func (i *Instance) buildBashExportPrefix() string {
	prefix := fmt.Sprintf("export AGENTDECK_INSTANCE_ID=%s; ", i.ID)
	if IsClaudeConfigDirExplicitForGroup(i.GroupPath) {
		prefix += fmt.Sprintf("export CLAUDE_CONFIG_DIR=%s; ", GetClaudeConfigDirForGroup(i.GroupPath))
	}
	return prefix
}
```

It unconditionally exports `AGENTDECK_INSTANCE_ID`. It conditionally exports `CLAUDE_CONFIG_DIR` whenever the group or any higher-priority source has it set. This is the helper PR #578 wired with group-awareness via `IsClaudeConfigDirExplicitForGroup(i.GroupPath)` and `GetClaudeConfigDirForGroup(i.GroupPath)`.

### 1.2 Every call site of `buildBashExportPrefix`

```
internal/session/instance.go:541   — claude --session-id resume branch (baseCommand == "claude")
internal/session/instance.go:559   — default new-session capture-resume (baseCommand == "claude")
internal/session/instance.go:4312  — buildClaudeForkCommandForTarget (fork path, uses "claude" binary literally)
```

**All three call sites are gated on `baseCommand == "claude"`** — i.e. non-custom-command paths. None of them fire when `Instance.Command` is a wrapper script.

### 1.3 Custom-command spawn path

`Start()` (`instance.go:1873-1909`) dispatches based on tool:

```
instance.go:1881-1883
case IsClaudeCompatible(i.Tool):
    command = i.buildClaudeCommand(i.Command)
```

For a conductor wrapper session (`i.Tool = "claude"`, `i.Command = "/home/.../start-conductor.sh"`), this calls:

```
instance.go:477-481
func (i *Instance) buildClaudeCommand(baseCommand string) string {
	envPrefix := i.buildEnvSourceCommand()
	cmd := i.buildClaudeCommandWithMessage(baseCommand, "")
	return envPrefix + cmd
}
```

Then `buildClaudeCommandWithMessage`:

```
instance.go:483-597
func (i *Instance) buildClaudeCommandWithMessage(baseCommand, message string) string {
	if !IsClaudeCompatible(i.Tool) {
		return baseCommand                               // (not taken; tool is "claude")
	}
	...
	configDirPrefix := ""
	if !hasCustomCommand && IsClaudeConfigDirExplicitForGroup(i.GroupPath) { // line 501
		configDir := GetClaudeConfigDirForGroup(i.GroupPath)
		configDirPrefix = fmt.Sprintf("CLAUDE_CONFIG_DIR=%s ", configDir)
	}
	instanceIDPrefix := fmt.Sprintf("AGENTDECK_INSTANCE_ID=%s ", i.ID)
	configDirPrefix = instanceIDPrefix + configDirPrefix
	...
	if baseCommand == "claude" {                          // line 520
		... all export/prefix work happens here ...
		bashExportPrefix := i.buildBashExportPrefix()     // lines 541, 559
		return fmt.Sprintf(`%sexec %s --session-id "%s"%s`, bashExportPrefix, claudeCmd, sessionUUID, extraFlags)
	}

	// For custom commands (e.g., fork commands), return as-is
	return baseCommand                                    // line 596
}
```

Note: `hasCustomCommand` at line 493 refers to the *user-config claude alias* (`[claude].command`, e.g. `"cdw"`), NOT to `Instance.Command`. This is a misleading name. For `Instance.Command = "./wrapper.sh"` with no `[claude].command` set, `hasCustomCommand == false`, so the `configDirPrefix` IS built at line 501 — but it is never concatenated anywhere because execution jumps past the `if baseCommand == "claude"` branch and returns `baseCommand` unchanged.

**`envPrefix` from `buildClaudeCommand`** (`env.go:21-87`) sources `.env` files but does NOT export `CLAUDE_CONFIG_DIR`. It is strictly `source "<path>"` lines joined with `&&`.

### 1.4 Does `CLAUDE_CONFIG_DIR` reach the bash exec env for custom-command sessions today?

**Answer: NO.** Evidence:

- `buildClaudeCommandWithMessage` returns `baseCommand` unchanged at `instance.go:596` when the base command is not literally `"claude"`.
- `buildBashExportPrefix` is invoked only at `instance.go:541`, `559`, `4312` — all inside `if baseCommand == "claude"` branches or fork-specific paths. Never for `Instance.Command` wrapper scripts.
- `buildEnvSourceCommand` (`env.go:21-87`) does not export `CLAUDE_CONFIG_DIR`.
- `tmuxSession.SetEnvironment` is called for `AGENTDECK_INSTANCE_ID`, `CLAUDE_SESSION_ID`, `CODEX_SESSION_ID`, `GEMINI_SESSION_ID`, `GEMINI_YOLO_MODE`, `COLORFGBG`, `OPENCODE_SESSION_ID` (see `instance.go:926, 1068, 1638, 1677, 1929, 1938, 1941, 1948, 1958, 2046, 2053, 2056, 2063, 2070, 2660, 2683, 2702, 2757, 2857, 3032, 3037, 3042, 3047, 4079`) but **never for `CLAUDE_CONFIG_DIR`**. Confirmed by `grep -n 'SetEnvironment' internal/session/` having zero `CLAUDE_CONFIG_DIR` hits.

The only way `CLAUDE_CONFIG_DIR` could land in a conductor pane today is if the user's group `env_file` happens to `export CLAUDE_CONFIG_DIR=...` inside the sourced file. That is a side-effect of the env_file support (CFG-03, Phase 2), not a designed code path for CFG-02.

### 1.5 Minimal surgical fix sketch

Two equivalent options. The planner should pick Option A unless it surfaces a regression against PR #578's existing tests.

**Option A (preferred — additive, minimal):** At `instance.go:596`, prepend `buildBashExportPrefix()` for claude-compatible custom commands. The prefix already conditionally emits `export CLAUDE_CONFIG_DIR=...`. It also unconditionally emits `export AGENTDECK_INSTANCE_ID=...`, which is already inline-set elsewhere (`instance.go:508-509`) — so re-exporting it inside the bash shell is redundant but harmless.

Patch (lines near `instance.go:593-597`):

```go
		return baseCmd
	}

	// For custom commands (e.g., fork commands or conductor wrappers), prepend the
	// bash export prefix so CLAUDE_CONFIG_DIR from a group override lands in the
	// spawn env before exec'ing the wrapper. (REQ CFG-02)
	return i.buildBashExportPrefix() + baseCommand
}
```

**Option B (alternative — use the already-built `configDirPrefix`):** The function already builds `configDirPrefix` at lines 500-509 (as an inline `CLAUDE_CONFIG_DIR=... AGENTDECK_INSTANCE_ID=...` prefix — note: inline form, no `export`, not a full bash export). Prepend that at line 596. The inline form is sufficient for a single exec'd command but NOT for multi-command wrappers that spawn sub-processes; for a conductor wrapper that later does `exec claude`, the `export` form (Option A) is strictly safer.

**Recommendation:** Option A. Three lines of real change. No modification to PR #578's files beyond this single return statement.

The additional `hasCustomCommand` concern (lines 491-503): this refers to `GetClaudeCommand() != "claude"`, i.e. user sets `[claude].command = "cdw"`. It is a *separate* branch from `Instance.Command`. PR #578 kept `!hasCustomCommand && IsClaudeConfigDirExplicitForGroup(...)` guarding the inline-prefix path for the `baseCommand == "claude"` branch — don't touch that guard; it's covered by `TestBuildClaudeCommand_CustomAlias` at `instance_test.go:462-505`.

---

## 2. Four test shapes

House style (verified against `instance_test.go` and `claude_test.go`):

- `package session` (not `_test`); helpers in-package.
- Plain `testing.T`, no `stretchr/testify`, no `assert.*` (grep confirms).
- Isolation recipe: save `HOME`, `AGENTDECK_PROFILE`, `CLAUDE_CONFIG_DIR` → `t.TempDir()` → write `~/.agent-deck/config.toml` → `ClearUserConfigCache()` → assertion → `defer` restores and re-clears cache. Exact pattern used at `claude_test.go:693-763` and `instance_test.go:424-460`.
- Assertion against spawn commands: `strings.Contains(cmd, "CLAUDE_CONFIG_DIR=...")` — `instance_test.go:445`, `:494`, `:137`, `:194`.
- Test names already namespaced `TestPerGroupConfig_<Name>` (required by ROADMAP and so `-run TestPerGroupConfig_` matches all six across both phases).

### 2.1 `TestPerGroupConfig_CustomCommandGetsGroupConfigDir`

**Helper seam:** `(*Instance).buildClaudeCommand(baseCommand string) string` — `instance.go:477`.
**Seam exists today?** Yes (PR #578 didn't change its signature).
**Expected RED/GREEN:** RED against `fa9971e`. GREEN after Option-A fix.

```go
func TestPerGroupConfig_CustomCommandGetsGroupConfigDir(t *testing.T) {
    tmpHome := t.TempDir()
    origHome := os.Getenv("HOME")
    origClaudeDir := os.Getenv("CLAUDE_CONFIG_DIR")
    t.Cleanup(func() {
        _ = os.Setenv("HOME", origHome)
        if origClaudeDir != "" {
            _ = os.Setenv("CLAUDE_CONFIG_DIR", origClaudeDir)
        } else {
            _ = os.Unsetenv("CLAUDE_CONFIG_DIR")
        }
        ClearUserConfigCache()
    })
    _ = os.Setenv("HOME", tmpHome)
    _ = os.Unsetenv("CLAUDE_CONFIG_DIR")

    agentDeckDir := filepath.Join(tmpHome, ".agent-deck")
    if err := os.MkdirAll(agentDeckDir, 0o700); err != nil {
        t.Fatalf("mkdir: %v", err)
    }
    cfg := `
[groups."conductor".claude]
config_dir = "~/.claude-work"
`
    if err := os.WriteFile(filepath.Join(agentDeckDir, "config.toml"), []byte(cfg), 0o600); err != nil {
        t.Fatalf("write config: %v", err)
    }
    ClearUserConfigCache()

    inst := NewInstanceWithGroupAndTool("conductor-x", "/tmp/p", "conductor", "claude")
    wrapper := "/tmp/start-conductor.sh"
    cmd := inst.buildClaudeCommand(wrapper)

    wantDir := filepath.Join(tmpHome, ".claude-work")
    if !strings.Contains(cmd, "CLAUDE_CONFIG_DIR="+wantDir) {
        t.Errorf("custom-command spawn missing CLAUDE_CONFIG_DIR=%s\ngot: %s", wantDir, cmd)
    }
    if !strings.HasSuffix(cmd, wrapper) {
        t.Errorf("spawn must end with wrapper path %q, got: %s", wrapper, cmd)
    }
}
```

### 2.2 `TestPerGroupConfig_GroupOverrideBeatsProfile`

**Helper seam:** Primary seam is `(*Instance).buildClaudeCommand` — we want the spawn command for a group+profile both-set case to contain the group's value, not the profile's. (Secondary: `GetClaudeConfigDirForGroup` is already proven by `TestGetClaudeConfigDirForGroup_GroupWins` at `claude_test.go:693-763`; no point duplicating.)
**Seam exists today?** Yes.
**Expected RED/GREEN:** RED against `fa9971e` (same root cause as test 2.1 — no export at all for custom commands). GREEN after Option-A fix.

```go
func TestPerGroupConfig_GroupOverrideBeatsProfile(t *testing.T) {
    tmpHome := t.TempDir()
    origHome, origProfile, origEnvDir := os.Getenv("HOME"), os.Getenv("AGENTDECK_PROFILE"), os.Getenv("CLAUDE_CONFIG_DIR")
    t.Cleanup(func() {
        _ = os.Setenv("HOME", origHome)
        if origProfile != "" { _ = os.Setenv("AGENTDECK_PROFILE", origProfile) } else { _ = os.Unsetenv("AGENTDECK_PROFILE") }
        if origEnvDir != "" { _ = os.Setenv("CLAUDE_CONFIG_DIR", origEnvDir) } else { _ = os.Unsetenv("CLAUDE_CONFIG_DIR") }
        ClearUserConfigCache()
    })
    _ = os.Setenv("HOME", tmpHome)
    _ = os.Unsetenv("CLAUDE_CONFIG_DIR")
    _ = os.Setenv("AGENTDECK_PROFILE", "work")

    agentDeckDir := filepath.Join(tmpHome, ".agent-deck")
    _ = os.MkdirAll(agentDeckDir, 0o700)
    cfg := `
[profiles.work.claude]
config_dir = "~/.claude-work"

[groups."conductor".claude]
config_dir = "~/.claude-group"
`
    _ = os.WriteFile(filepath.Join(agentDeckDir, "config.toml"), []byte(cfg), 0o600)
    ClearUserConfigCache()

    inst := NewInstanceWithGroupAndTool("c", "/tmp/p", "conductor", "claude")
    cmd := inst.buildClaudeCommand("/tmp/wrapper.sh")

    wantGroup := filepath.Join(tmpHome, ".claude-group")
    if !strings.Contains(cmd, "CLAUDE_CONFIG_DIR="+wantGroup) {
        t.Errorf("group override must beat profile; got: %s", cmd)
    }
    if strings.Contains(cmd, filepath.Join(tmpHome, ".claude-work")) {
        t.Errorf("profile path leaked into spawn despite group override; got: %s", cmd)
    }
}
```

### 2.3 `TestPerGroupConfig_UnknownGroupFallsThroughToProfile`

**Helper seam:** `GetClaudeConfigDirForGroup(groupPath string) string` — `claude.go:246`.
**Seam exists today?** Yes, and already covered by `TestGetClaudeConfigDirForGroup_GroupWins` at `claude_test.go:744-748` (unknown-group branch). We add a dedicated, single-assertion test for explicitness.
**Expected RED/GREEN:** GREEN immediately against `fa9971e`. No fix required for this one.

```go
func TestPerGroupConfig_UnknownGroupFallsThroughToProfile(t *testing.T) {
    tmpHome := t.TempDir()
    origHome, origProfile, origEnvDir := os.Getenv("HOME"), os.Getenv("AGENTDECK_PROFILE"), os.Getenv("CLAUDE_CONFIG_DIR")
    t.Cleanup(func() {
        _ = os.Setenv("HOME", origHome)
        if origProfile != "" { _ = os.Setenv("AGENTDECK_PROFILE", origProfile) } else { _ = os.Unsetenv("AGENTDECK_PROFILE") }
        if origEnvDir != "" { _ = os.Setenv("CLAUDE_CONFIG_DIR", origEnvDir) } else { _ = os.Unsetenv("CLAUDE_CONFIG_DIR") }
        ClearUserConfigCache()
    })
    _ = os.Setenv("HOME", tmpHome)
    _ = os.Unsetenv("CLAUDE_CONFIG_DIR")
    _ = os.Setenv("AGENTDECK_PROFILE", "work")

    agentDeckDir := filepath.Join(tmpHome, ".agent-deck")
    _ = os.MkdirAll(agentDeckDir, 0o700)
    cfg := `
[profiles.work.claude]
config_dir = "~/.claude-work"

[groups."real-group".claude]
config_dir = "~/.claude-real-group"
`
    _ = os.WriteFile(filepath.Join(agentDeckDir, "config.toml"), []byte(cfg), 0o600)
    ClearUserConfigCache()

    got := GetClaudeConfigDirForGroup("does-not-exist")
    want := filepath.Join(tmpHome, ".claude-work")
    if got != want {
        t.Errorf("unknown group should fall through to profile: got=%s want=%s", got, want)
    }
}
```

### 2.4 `TestPerGroupConfig_CacheInvalidation`

**Helper seam:** `ClearUserConfigCache()` — `userconfig.go:1222`; `GetClaudeConfigDirForGroup` — `claude.go:246`.
**Seam exists today?** Yes. Pattern mirrored from `claude_test.go:710-716`.
**Expected RED/GREEN:** GREEN immediately against `fa9971e`.

```go
func TestPerGroupConfig_CacheInvalidation(t *testing.T) {
    tmpHome := t.TempDir()
    origHome, origEnvDir := os.Getenv("HOME"), os.Getenv("CLAUDE_CONFIG_DIR")
    t.Cleanup(func() {
        _ = os.Setenv("HOME", origHome)
        if origEnvDir != "" { _ = os.Setenv("CLAUDE_CONFIG_DIR", origEnvDir) } else { _ = os.Unsetenv("CLAUDE_CONFIG_DIR") }
        ClearUserConfigCache()
    })
    _ = os.Setenv("HOME", tmpHome)
    _ = os.Unsetenv("CLAUDE_CONFIG_DIR")

    agentDeckDir := filepath.Join(tmpHome, ".agent-deck")
    configPath := filepath.Join(agentDeckDir, "config.toml")
    _ = os.MkdirAll(agentDeckDir, 0o700)

    // v1: group override present
    _ = os.WriteFile(configPath, []byte(`[groups."g".claude]` + "\nconfig_dir = \"~/.claude-g\"\n"), 0o600)
    ClearUserConfigCache()
    if got, want := GetClaudeConfigDirForGroup("g"), filepath.Join(tmpHome, ".claude-g"); got != want {
        t.Fatalf("v1: got %s want %s", got, want)
    }

    // v2: group override removed; cache must be cleared to pick up the change
    _ = os.WriteFile(configPath, []byte("# empty config\n"), 0o600)
    ClearUserConfigCache()
    got := GetClaudeConfigDirForGroup("g")
    want := filepath.Join(tmpHome, ".claude") // default
    if got != want {
        t.Errorf("after cache invalidation, got=%s want=%s", got, want)
    }
}
```

---

## 3. PR #578 test inventory (regression-check)

Two tests in PR #578 must remain GREEN with zero assertion changes:

| Test name | File | Line |
|-----------|------|------|
| `TestGetClaudeConfigDirForGroup_GroupWins` | `internal/session/claude_test.go` | 693 |
| `TestIsClaudeConfigDirExplicitForGroup` | `internal/session/claude_test.go` | 765 |

Additional PR #578 tests in `userconfig_test.go` that the planner should also track (no assertion changes):

| Test name | File | Line |
|-----------|------|------|
| `TestUserConfig_GroupClaudeConfigDir_*` (config parsing: normal, empty, nested path) | `internal/session/userconfig_test.go` | 1140, 1167, 1177 |
| `TestUserConfig_GroupClaudeEnvFile` | `internal/session/userconfig_test.go` | 1198 |

Verification command to run after each change:

```bash
go test ./internal/session/... \
  -run 'TestGetClaudeConfigDirForGroup_GroupWins|TestIsClaudeConfigDirExplicitForGroup|TestUserConfig_GroupClaudeConfigDir|TestUserConfig_GroupClaudeEnvFile' \
  -race -count=1 -v
```

---

## 4. Test file naming

**Target:** `internal/session/pergroupconfig_test.go` — NEW file.

**Note:** There is **no** `internal/session/pergroupconfig.go` source file. PR #578 wired its logic into existing source files (`claude.go`, `userconfig.go`, `instance.go`, `env.go`). That is fine for Go — `_test.go` files do not need a paired source file. The name `pergroupconfig_test.go` is chosen for discoverability (it is the name committed in REQUIREMENTS.md § CFG-04 and ROADMAP.md §Phase 1 / success criterion 1).

Verified source-file inventory at base `fa9971e`:

```
internal/session/claude.go          (GetClaudeConfigDirForGroup, IsClaudeConfigDirExplicitForGroup, GetClaudeCommand, ...)
internal/session/userconfig.go      (GetGroupClaudeConfigDir, GetGroupClaudeEnvFile, ClearUserConfigCache, GroupClaudeConfig struct, ...)
internal/session/instance.go        (buildClaudeCommand, buildClaudeCommandWithMessage, buildBashExportPrefix, buildClaudeResumeCommand, NewInstance*, ...)
internal/session/env.go             (buildEnvSourceCommand, getToolEnvFile — reads GetGroupClaudeEnvFile at env.go:248)
internal/session/groups.go          (group persistence — not touched by PR #578 for claude logic)
```

No rename needed.

---

## 5. `make ci` invocation

`Makefile:142-145`:

```make
ci:
    @which lefthook > /dev/null || (echo "ERROR: lefthook not found. Run: brew install lefthook" && exit 1)
    lefthook run pre-push --force --no-auto-install
```

`lefthook.yml:8-22` defines `pre-push`:

| Step | Command | Tag |
|------|---------|-----|
| css-verify | `make css-verify` | quality,css |
| lint       | `golangci-lint run` | quality |
| test       | `go test -race -count=1 ./...` | test |
| build      | `go build -o /dev/null ./cmd/agent-deck/` | build |

Each is wrapped with `env -u GIT_DIR -u GIT_WORK_TREE -u GIT_COMMON_DIR -u GIT_INDEX_FILE -u GIT_OBJECT_DIRECTORY -u GIT_ALTERNATE_OBJECT_DIRECTORIES -u GIT_PREFIX` — this detaches the process from the git-hook environment so Go's module cache / CSS pipeline doesn't fight the worktree's `$GIT_DIR`.

**Phase-1-scoped verification command** (narrower than `make ci`, faster feedback during TDD):

```bash
go test ./internal/session/... -run TestPerGroupConfig_ -race -count=1 -v
```

**Full-suite verification** (mirrors lefthook `test`, use before final commit):

```bash
go test -race -count=1 ./...
```

**Full `make ci` parity** is the final gate before the phase's last commit.

**Pitfalls the planner must honor:**
- `make css` is a prerequisite of `make build` (`Makefile:16`), but `make ci` does `css-verify`, not `css`. If `internal/web/static/styles.css` is untouched in this phase (which it is — scope is Go-only), `make css-verify` should pass as-is. If the pre-push hook complains about CSS drift, it means something outside Phase 1 scope was touched — escalate, don't mask.
- `go vet ./...` and `gofmt -l` run on every `git commit` via the pre-commit hook (`lefthook.yml:25-33`). Any unformatted import or missing error check will block the commit with a real error, not a cosmetic one. Do NOT use `--no-verify`.

---

## 6. Commit signing infrastructure

**Active hooks on this worktree** (`ls -la .git/hooks/`):

```
pre-commit         → lefthook run pre-commit  (gofmt check + go vet)
prepare-commit-msg → lefthook run prepare-commit-msg
pre-push           → lefthook run pre-push    (css-verify + lint + test + build)
```

No `commit-msg` hook present — the planner is free to include the "Committed by Ashesh Goplani" trailer manually via the `git commit -m "..."` HEREDOC pattern. No hook validates signature format, so discipline is on the author.

**What `--no-verify` would do:** skip `pre-commit` (gofmt + vet) and `pre-push` (lint/test/build). Forbidden by v1.5.3 mandate at repo-root `CLAUDE.md` (ROADMAP.md:20-22). If `gofmt` or `go vet` complains, fix the underlying issue — don't bypass.

**What the pre-commit check will block in Phase 1:** any unformatted Go in `pergroupconfig_test.go` or in `instance.go` (if Option A is applied). Solution: run `go fmt ./internal/session/...` before `git add`.

**Commit trailer template (verified style):**

```
<subject line>

<body paragraph>

Builds on PR #578 by @alec-pinson.

Committed by Ashesh Goplani
```

The trailing "Committed by Ashesh Goplani" is NOT a standard git trailer (no `key:` prefix) — it is a plain signature line. `git commit --trailer` would not help; use the HEREDOC pattern from CLAUDE.md step 3.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `go test -run 'TestPerGroupConfig_CustomCommandGetsGroupConfigDir\|TestPerGroupConfig_GroupOverrideBeatsProfile' ./internal/session/...` will be RED against `fa9971e` (tests 1 and 2 fail) | §1.4 — VERIFIED by static read of `instance.go:485-597`, but NOT yet executed. | LOW — if they are already GREEN, Phase 1 collapses to pure test authoring and the Option-A patch is dropped. Planner must still author tests and run them first. |
| A2 | `go test -run 'TestPerGroupConfig_UnknownGroupFallsThroughToProfile\|TestPerGroupConfig_CacheInvalidation' ./internal/session/...` will be GREEN on first run | §1.4 + §2.3 + §2.4 — VERIFIED by reading `claude.go:246-267` and `userconfig.go:1222`. | LOW — they test the resolver and cache paths PR #578 already exercises. |
| A3 | Option A (prepend `buildBashExportPrefix()` at `instance.go:596`) will not regress `TestBuildClaudeCommand_CustomAlias` (`instance_test.go:462`) or the Fork tests | §1.5 | MEDIUM — if regression appears, Option B (inline prefix using already-built `configDirPrefix`) is the fallback. The PLAN must include a step "re-run `go test ./internal/session/... -race -count=1` before committing the fix." |
| A4 | Go 1.24.0 toolchain (`Makefile:13`) will not interfere with `t.Cleanup` / `strings.Contains` / standard-library TOML (`github.com/BurntSushi/toml` — confirmed via `userconfig_test.go:1151`) | §2 | NONE — these APIs are stable since Go 1.14. |

---

## Open Questions

None. All six research questions from the research scope are answered with file:line evidence.

---

## RESEARCH COMPLETE

**Planner's bottom line:** Phase 1 is **test-first, then one surgical patch**. Author `internal/session/pergroupconfig_test.go` with the four tests in §2 (commit #1 — RED for tests 1 & 2, GREEN for tests 3 & 6). Run the suite; confirm the two RED tests fail for the reason predicted in §1.4 (spawn string has no `CLAUDE_CONFIG_DIR=`). Apply Option A at `internal/session/instance.go:596` — prepend `i.buildBashExportPrefix()` to the custom-command return (commit #2, body carries `Builds on PR #578 by @alec-pinson.`). Re-run the four tests → all GREEN. Re-run PR #578's two existing tests (§3) + `TestBuildClaudeCommand_CustomAlias` → all GREEN. Final: `go test -race -count=1 ./internal/session/...` then `make ci`. Three expected commits total; all signed "Committed by Ashesh Goplani"; never `--no-verify`.
