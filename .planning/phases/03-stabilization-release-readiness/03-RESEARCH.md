# Phase 3: Stabilization & Release Readiness - Research

**Researched:** 2026-03-06
**Domain:** Go codebase quality gates, dead code removal, cross-platform builds, changelog management
**Confidence:** HIGH

## Summary

Phase 3 is purely a stabilization phase: the codebase must pass all quality gates (`golangci-lint`, `go test -race`, cross-platform `go build`), have dead code removed, and get a CHANGELOG.md entry documenting all milestone work. This phase does NOT involve feature development or architectural changes.

The current state is excellent. All four target platforms build cleanly (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64). The default `golangci-lint run` (no config file, default linters only) completes with zero warnings. All tests pass with race detector enabled. The primary work is therefore: (1) confirming the clean state, (2) scanning for dead code and stale artifacts that default linters may miss, (3) writing a CHANGELOG.md entry for the skills reorganization and testing milestone, and (4) running the full `make ci` pipeline as a final gate.

**Primary recommendation:** Since the codebase is already clean under default golangci-lint, focus the lint plan on explicitly confirming zero warnings (not fixing issues that do not exist), running a targeted dead code scan with `go vet` and unused import checks, removing any stale artifacts (old JSON configs, unused test helpers, orphaned files), and then writing the changelog. Do NOT attempt to fix high-complexity functions flagged by `gocyclo` since these are pre-existing in the TUI layer (home.go has 243-complexity `Update()`) and are out of scope for this stabilization milestone.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| STAB-02 | `golangci-lint run` passes with zero warnings | Already passing under default config. Plan should verify this as a gate, not fix non-existent issues. |
| STAB-03 | `go test -race ./...` passes with zero failures | Already passing (17 packages, all OK). Plan should run the full suite as verification. |
| STAB-04 | `go build` succeeds for all 4 target platforms | Already verified: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 all build to /dev/null successfully. |
| STAB-05 | Dead code and stale artifacts removed from codebase | Requires targeted scan. `gofmt -l` shows no formatting issues. `go vet` clean. Need to check for unused exports, stale files, orphaned test helpers. |
| STAB-06 | CHANGELOG.md updated with all changes | Changelog follows Keep a Changelog format. Current latest entry is [0.21.1]. Milestone changes are: skills reformatting (SKILL.md frontmatter, $SKILL_DIR path resolution, marketplace.json registration) and new test files (3 test files, 950 lines). |
</phase_requirements>

## Standard Stack

### Core (Already in Project)
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| golangci-lint | v1.64.8 | Multi-linter runner | Standard Go linting tool, already in Makefile/lefthook |
| go test -race | Go 1.24+ | Race-detected test runner | Built-in Go toolchain |
| go build | Go 1.24+ | Cross-compilation | Built-in Go cross-compilation via GOOS/GOARCH |
| gofmt | Go 1.24+ | Code formatting | Built-in, enforced by pre-commit hook |
| go vet | Go 1.24+ | Static analysis | Built-in, enforced by pre-commit hook |
| lefthook | latest | Git hooks runner | Already configured in project |

### No Additional Dependencies Required

This phase requires zero new dependencies. All tools are already installed and configured.

## Architecture Patterns

### Existing Quality Gate Pipeline

The project has a well-defined quality pipeline via `make ci` (lefthook pre-push):

```
Pre-commit (fast):
  gofmt check + go vet (parallel)

Pre-push (full, parallel):
  golangci-lint run
  go test -race -count=1 ./...
  go build -o /dev/null ./cmd/agent-deck/
```

### Cross-Platform Build Pattern

GoReleaser config (`.goreleaser.yml`) defines the target matrix:
```yaml
goos: [linux, darwin]
goarch: [amd64, arm64]
env: [CGO_ENABLED=0]
```

Manual verification command pattern:
```bash
GOOS=$os GOARCH=$arch go build -o /dev/null ./cmd/agent-deck
```

### CHANGELOG Format

The project uses [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) with Semantic Versioning. Entry structure:

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added
- Feature descriptions (start with verb)

### Changed
- Modification descriptions

### Fixed
- Bug fix descriptions
```

The milestone's changes are documentation and test infrastructure, not user-facing features. The appropriate changelog categories are:
- **Added**: New test files for session lifecycle and skills runtime
- **Changed**: Skills SKILL.md reformatted to official Anthropic skill-creator format, $SKILL_DIR path resolution added

### Dead Code Scan Pattern

For a Go project without golangci-lint config, dead code detection uses:
1. `go vet ./...` (already clean)
2. Default golangci-lint linters include `unused` (already clean)
3. Manual scan for: orphaned files not referenced by any import, stale migration artifacts, unused exported functions

### Anti-Patterns to Avoid
- **Do NOT add a .golangci.yml config file:** The project intentionally uses defaults. Adding config to enable extra linters (gocyclo, dupl, etc.) would create noise from pre-existing code and is out of scope.
- **Do NOT refactor high-complexity functions:** The `home.go:Update()` function (complexity 243) is the Bubble Tea message handler. Refactoring it is a feature change, not stabilization.
- **Do NOT bump the version:** REQUIREMENTS.md explicitly states "Version bump: Deferred until work is assessed". The changelog documents what changed, but version bump is out of scope.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Lint checking | Custom script parsing go files | `golangci-lint run` | Already configured, catches all standard issues |
| Dead code detection | Manual grep for unused functions | `go vet ./...` + `golangci-lint` default `unused` linter | Toolchain handles this correctly |
| Cross-platform build verification | CI/CD pipeline | `GOOS=x GOARCH=y go build` loop | Simple, already proven working |
| Changelog generation | Automated git log parsing | Manual Keep a Changelog entry | Project convention, provides curated human-readable entries |

## Common Pitfalls

### Pitfall 1: Creating Work Where None Exists
**What goes wrong:** Enabling extra linters or running non-default analysis and then "fixing" hundreds of pre-existing issues that were never in scope.
**Why it happens:** Over-enthusiasm for code quality in a stabilization phase.
**How to avoid:** STAB-02 says "`golangci-lint run` passes with zero warnings". The default run already passes. Verify and move on.
**Warning signs:** Spending more than 5 minutes on lint fixes.

### Pitfall 2: Confusing "Dead Code" With "Code I Don't Understand"
**What goes wrong:** Removing functions that appear unused but are called via reflection, interface satisfaction, or build tags.
**Why it happens:** Static analysis cannot see all call paths.
**How to avoid:** Only remove code that `unused` linter flags. For manual removals, verify with `grep -r` across the entire codebase.
**Warning signs:** Removing exported functions from packages with many callers.

### Pitfall 3: Changelog Scope Creep
**What goes wrong:** Documenting every planning doc and internal change instead of user-visible changes.
**Why it happens:** Milestone includes planning docs, but changelog is for users.
**How to avoid:** Only document changes that affect the user or developer experience: skills format changes, new tests, build fixes.
**Warning signs:** Changelog entry mentions ".planning/" files.

### Pitfall 4: Accidentally Breaking Builds With Dead Code Removal
**What goes wrong:** Removing a file that is imported by another package, causing build failure.
**Why it happens:** Go import paths are string-based and easy to miss.
**How to avoid:** After every removal, run `go build ./...` immediately. Never batch removals without intermediate builds.
**Warning signs:** Removing files without checking imports.

## Code Examples

### Cross-Platform Build Verification Loop
```bash
# Verify all 4 target platforms
for os in darwin linux; do
  for arch in amd64 arm64; do
    GOOS=$os GOARCH=$arch go build -o /dev/null ./cmd/agent-deck && echo "$os/$arch: OK" || echo "$os/$arch: FAILED"
  done
done
```

### Dead Code Scan Commands
```bash
# Default lint (includes unused linter)
golangci-lint run

# Go vet for static analysis
go vet ./...

# Check formatting
test -z "$(gofmt -l .)"
```

### Full CI Pipeline
```bash
# Equivalent to make ci / lefthook pre-push
make ci
```

### CHANGELOG Entry Template for This Milestone
```markdown
## [Unreleased]

### Added
- Add session lifecycle tests covering start, stop, fork, and attach operations with tmux verification.
- Add status lifecycle tests for sleep/wake detection and SQLite persistence round-trips.
- Add skills runtime tests verifying on-demand skill loading, pool skill discovery, and project skill application.

### Changed
- Reformat agent-deck and session-share SKILL.md files to official Anthropic skill-creator format with proper frontmatter.
- Add $SKILL_DIR path resolution to session-share skill for plugin cache compatibility.
- Register session-share skill in marketplace.json for independent discoverability.
- Update GSD conductor skill content in pool directory with current lifecycle documentation.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| deadcode/varcheck/structcheck linters | `unused` linter (built-in to golangci-lint) | golangci-lint v1.49.0 | Old linters fully deactivated, `unused` covers all cases |
| Manual cross-compile testing | GoReleaser with GOOS/GOARCH matrix | Already in project | Automated in release process |

## Current Codebase State (Research Findings)

| Check | Status | Details |
|-------|--------|---------|
| `golangci-lint run` (default) | CLEAN | Zero warnings, zero errors |
| `go test -race ./...` | CLEAN | 17 packages pass (1 no test files: testutil) |
| `go vet ./...` | CLEAN | Zero issues |
| `gofmt -l .` | CLEAN | Zero files need formatting |
| darwin/amd64 build | PASS | |
| darwin/arm64 build | PASS | |
| linux/amd64 build | PASS | |
| linux/arm64 build | PASS | |

**This means STAB-02, STAB-03, and STAB-04 are already satisfied.** The plan should verify (not fix) these gates and focus effort on STAB-05 (dead code scan) and STAB-06 (changelog).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + race detector (Go 1.24+) |
| Config file | No config file; uses `go test` defaults with `-race` flag |
| Quick run command | `go test -race -v ./internal/session/...` |
| Full suite command | `go test -race -count=1 ./...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| STAB-02 | golangci-lint zero warnings | smoke | `golangci-lint run` (exit code 0 = pass) | N/A (tool output) |
| STAB-03 | Full test suite passes | integration | `go test -race -count=1 ./...` | N/A (runs existing tests) |
| STAB-04 | Cross-platform builds | smoke | `GOOS=x GOARCH=y go build -o /dev/null ./cmd/agent-deck` for 4 combos | N/A (build output) |
| STAB-05 | No dead code or stale artifacts | manual-only | Manual scan + `golangci-lint run` (unused linter) | N/A |
| STAB-06 | CHANGELOG.md updated | manual-only | Visual inspection of CHANGELOG.md content | CHANGELOG.md exists |

### Sampling Rate
- **Per task commit:** `golangci-lint run && go test -race -count=1 ./...`
- **Per wave merge:** Full 4-platform build verification + `make ci`
- **Phase gate:** `make ci` green + all 4 cross-platform builds pass

### Wave 0 Gaps
None. Existing test infrastructure covers all phase requirements. No new test files needed for this phase.

## Open Questions

1. **Should the milestone get a version bump?**
   - What we know: REQUIREMENTS.md says "Version bump: Deferred until work is assessed". Current version is 0.21.1.
   - What's unclear: Whether the user wants to bump to 0.22.0 after this milestone.
   - Recommendation: The changelog should use `[Unreleased]` header. Version bump is explicitly out of scope per requirements.

2. **How thorough should dead code scanning be?**
   - What we know: Default linters already pass clean. No obvious dead code flagged.
   - What's unclear: Whether pre-existing unused code (before this milestone) should be removed.
   - Recommendation: Focus on artifacts related to the milestone's changes (phases 1-2). Do a quick scan for obviously unused exports, but do not undertake a codebase-wide dead code audit. The requirement says "No dead code, unused imports, or stale artifacts remain" but in context of this milestone's changes.

## Sources

### Primary (HIGH confidence)
- Local tooling output: `golangci-lint run`, `go test -race ./...`, `go build`, `go vet ./...`, `gofmt -l .` (all executed directly against the codebase)
- Project files: Makefile, lefthook.yml, .goreleaser.yml, CHANGELOG.md, CLAUDE.md

### Secondary (MEDIUM confidence)
- golangci-lint v1.64.8 deprecation warnings for deadcode/varcheck/structcheck (confirmed via direct execution)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all tools already installed and running in the project
- Architecture: HIGH - quality pipeline is well-defined via Makefile and lefthook
- Pitfalls: HIGH - based on direct observation of codebase state (already clean)
- Current state: HIGH - all commands executed directly, results observed

**Research date:** 2026-03-06
**Valid until:** 2026-04-06 (stable tooling, no fast-moving dependencies)
