---
phase: 17-release-pipeline-slack-bridge
plan: 02
subsystem: conductor
tags: [slack, python, bridge, markdown, mrkdwn, regex]

# Dependency graph
requires:
  - phase: 17-01
    provides: Release pipeline and CI workflow already in place
provides:
  - _markdown_to_slack() converter in Python bridge template
  - Automatic GFM-to-mrkdwn conversion in _safe_say() for all outbound Slack messages
affects: [conductor, slack-bridge]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Sentinel placeholder substitution for backtick content in Python regex embedded in Go raw strings"
    - "Code-block and inline-code protection before text transformation, restore after"

key-files:
  created: []
  modified:
    - internal/session/conductor_templates.go
    - internal/session/conductor_test.go

key-decisions:
  - "Used sentinel strings (__CODE_BLOCK_N__ / __INLINE_CODE_N__) instead of \\x00 bytes because Go raw string literals cannot contain escape sequences"
  - "_markdown_to_slack applied in _safe_say (single choke point) so all outbound Slack text is converted automatically without modifying each call site"
  - "Conversion is conditional: only applied when 'text' kwarg is present, leaving attachments/blocks-only calls untouched"

patterns-established:
  - "Embedding regex patterns with backticks in Go raw strings: use + \"`\" + concatenation pattern"

requirements-completed: [SLACK-01, SLACK-02]

# Metrics
duration: 20min
completed: 2026-03-16
---

# Phase 17 Plan 02: Slack Bridge Markdown Converter Summary

**GFM-to-Slack-mrkdwn converter added to conductor bridge: headers, bold, strikethrough, links, and bullets convert automatically; code blocks and inline code pass through unchanged**

## Performance

- **Duration:** 20 min
- **Started:** 2026-03-16T13:00:00Z
- **Completed:** 2026-03-16T13:20:51Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments

- Added `_markdown_to_slack()` nested function to the Slack app closure in the Python bridge template, converting GFM headers (`## H2` to `*H2*`), bold (`**text**` to `*text*`), strikethrough (`~~text~~` to `~text~`), hyperlinks (`[text](url)` to `<url|text>`), and bullet lists (`- item` to `• item`)
- Code blocks (triple-backtick fenced) and inline code (single-backtick) are extracted to sentinel placeholders before conversion and restored after, preserving Slack's native code rendering
- Modified `_safe_say()` to apply the converter to any `text` kwarg before calling `say()`, making conversion automatic for all outbound conductor messages without touching individual call sites
- Go tests in `conductor_test.go` verify the template contains the function definition, all regex patterns, code protection lists, and that `_safe_say` applies the converter conditionally

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Failing tests for _markdown_to_slack** - `358c31f` (test) — part of 17-01 commit
2. **Task 1 (GREEN): Implement _markdown_to_slack and update _safe_say** - `d59c036` (feat)

## Files Created/Modified

- `internal/session/conductor_templates.go` - Added `_markdown_to_slack()` function (44 lines) and updated `_safe_say()` to apply it
- `internal/session/conductor_test.go` - Added `TestBridgeTemplate_ContainsMarkdownToSlackConverter` and `TestBridgeTemplate_SafeSayConvertsMarkdown` (already committed in 358c31f)

## Decisions Made

- Used sentinel strings (`__CODE_BLOCK_N__` and `__INLINE_CODE_N__`) rather than `\x00` null bytes as placeholders, because Go raw string literals cannot contain `\x00` escape sequences and the sentinel strings are unlikely to appear in real conductor responses.
- Applied conversion inside `_safe_say` rather than at each individual call site: `_safe_say` is the single choke point for all outbound Slack text, so modifying it converts all messages without touching `_handle_slack_text`, heartbeat notifications, or error messages individually.
- Conversion is conditional (`if "text" in kwargs`) to avoid interfering with Block Kit payloads that may have no `text` field.

## Deviations from Plan

None — plan executed exactly as written. The only minor adaptation was the sentinel placeholder approach (explicitly described in the plan's NOTE section as the recommended alternative to `\x00`).

## Issues Encountered

The pre-commit hook runs `go vet ./...` on all Go files on disk, including the untracked `internal/ui/keyboard_compat_test.go` that was present from prior Phase 18 planning work. Investigation showed `keyboard_compat.go` already existed (committed in `6d83c71`), so vet passed cleanly once that was confirmed. No action was required.

## Next Phase Readiness

- Phase 17 is now complete (both plans done).
- Phase 18 (Wayland key input) is also complete per git log.
- Milestone v0.26.2 Stability Fixes appears fully implemented.

---
*Phase: 17-release-pipeline-slack-bridge*
*Completed: 2026-03-16*
