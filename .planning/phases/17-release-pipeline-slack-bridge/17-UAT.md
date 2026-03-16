---
status: complete
phase: 17-release-pipeline-slack-bridge
source: 17-01-SUMMARY.md, 17-02-SUMMARY.md
started: 2026-03-16T14:00:00Z
updated: 2026-03-16T14:08:00Z
---

## Current Test

[testing complete]

## Tests

### 1. CI Release Asset Validation Step
expected: `.github/workflows/release.yml` contains a "Validate release assets" step after GoReleaser that checks for 4 platform tarballs (darwin/linux x amd64/arm64) plus checksums.txt. Uses `gh` CLI. Fails with clear error on missing assets.
result: pass

### 2. install.sh No-Assets Error Message
expected: When a GitHub release exists but has no assets (e.g., CI failed), install.sh displays a message about the CI workflow and provides a link to the GitHub Actions page so the user knows where to look.
result: pass

### 3. install.sh Platform Mismatch Error
expected: When a release has assets but none match the current OS/architecture, install.sh lists the available asset names so the user can see what platforms are supported.
result: pass

### 4. install.sh jq-with-grep Fallback
expected: install.sh uses `jq` for JSON parsing when available. When jq is not installed, it falls back to grep/sed-based parsing to extract asset information from the GitHub API response.
result: pass

### 5. GFM-to-Slack Markdown Converter
expected: `_markdown_to_slack()` function exists in `internal/session/conductor_templates.go` within the Python bridge template. Converts: `## Header` to `*Header*`, `**bold**` to `*bold*`, `~~strike~~` to `~strike~`, `[text](url)` to `<url|text>`, `- item` to `• item`.
result: pass

### 6. Code Block Preservation
expected: The markdown converter protects fenced code blocks (triple-backtick) and inline code (single-backtick) by extracting them to sentinel placeholders (`__CODE_BLOCK_N__` / `__INLINE_CODE_N__`) before conversion and restoring them after. Code content passes through to Slack unchanged.
result: pass

### 7. Auto-Conversion via _safe_say
expected: `_safe_say()` in the bridge template applies `_markdown_to_slack` to the `text` kwarg before calling `say()`. Conversion is conditional: only applied when `text` is present, leaving blocks-only or attachment-only calls untouched.
result: pass

### 8. Go Tests Pass
expected: Running `go test -run "TestBridgeTemplate_Contains|TestBridgeTemplate_SafeSay" ./internal/session/...` passes with all assertions green. Tests verify the template contains the function definition, regex patterns, code protection, and that _safe_say applies the converter conditionally.
result: pass

## Summary

total: 8
passed: 8
issues: 0
pending: 0
skipped: 0

## Gaps

[none yet]
