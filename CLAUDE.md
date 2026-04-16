# Agent Deck — Project Instructions

## Go Toolchain

Pin to Go 1.24.0 for all builds and tests. Go 1.25 silently breaks macOS TUI rendering.

```bash
export GOTOOLCHAIN=go1.24.0
```

## Watcher framework: mandatory test coverage

Any commit touching watcher source code MUST pass the following test commands before merge:

```bash
# Watcher engine, adapters, health bridge, layout, state, event log
GOTOOLCHAIN=go1.24.0 go test ./internal/watcher/... -race -count=1 -timeout 120s

# CLI: watcher commands, drift-check, health fields, import
GOTOOLCHAIN=go1.24.0 go test ./cmd/agent-deck/... -run "Watcher" -race -count=1
```

### Paths that trigger this mandate

Changes to any of the following paths require the above test commands to pass:

- `internal/watcher/**` (engine, adapters, health bridge, layout, state, event log, router)
- `cmd/agent-deck/watcher_cmd*.go` (CLI surface)
- `internal/ui/watcher_panel.go` (TUI watcher panel)
- `internal/statedb/statedb.go` (watcher rows in SQLite)
- `cmd/agent-deck/assets/skills/watcher-creator/` (embedded skill)
- `internal/session/watcher_meta.go` (watcher directory helpers)

### Structural changes requiring RFC

The following changes require discussion before implementation:

- Removing or weakening the health bridge (`internal/watcher/health_bridge.go`)
- Disabling SQLite dedup (INSERT OR IGNORE on `watcher_events`)
- Weakening or removing HMAC-SHA256 verification on the GitHub adapter
- Changing the `~/.agent-deck/watcher/` folder layout (REQ-WF-6)

### Skills + docs sync (REQ-WF-7)

Any commit modifying `internal/watcher/layout.go` or path-resolution code in `internal/session/watcher_meta.go` MUST also update:

- `cmd/agent-deck/assets/skills/watcher-creator/SKILL.md` (embedded skill paths)
- `README.md` if it references watcher data paths
- `CHANGELOG.md` if the change affects user-visible paths

The `TestSkillDriftCheck_WatcherCreator` test enforces this at build time.

### Integration harness

Run the full verification harness to validate the watcher framework end-to-end:

```bash
bash scripts/verify-watcher-framework.sh
```

This exercises: build, unit tests, integration tests, artifact checks, drift detection, security mitigations (T-21-SL, T-21-PI), and CHANGELOG validation.
