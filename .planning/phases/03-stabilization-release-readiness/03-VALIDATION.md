---
phase: 3
slug: stabilization-release-readiness
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-06
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + race detector (Go 1.24+) |
| **Config file** | none — uses `go test` defaults with `-race` flag |
| **Quick run command** | `golangci-lint run && go test -race -count=1 ./...` |
| **Full suite command** | `make ci` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `golangci-lint run && go test -race -count=1 ./...`
- **After every plan wave:** Run `make ci` + 4-platform build verification
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | STAB-02 | smoke | `golangci-lint run` | N/A (tool) | ⬜ pending |
| 03-01-02 | 01 | 1 | STAB-03 | integration | `go test -race -count=1 ./...` | N/A (runs existing) | ⬜ pending |
| 03-01-03 | 01 | 1 | STAB-04 | smoke | `GOOS=x GOARCH=y go build -o /dev/null ./cmd/agent-deck` x4 | N/A (build) | ⬜ pending |
| 03-01-04 | 01 | 1 | STAB-05 | manual+auto | `golangci-lint run` + manual scan | N/A | ⬜ pending |
| 03-02-01 | 02 | 2 | STAB-06 | manual-only | Visual inspection of CHANGELOG.md | ✅ CHANGELOG.md | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. No new test files or frameworks needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dead code/stale artifact scan | STAB-05 | Requires human judgment on what is "stale" vs. intentionally kept | Scan for unused exports, orphaned files, stale migration artifacts; confirm with `grep -r` before removal |
| CHANGELOG.md content accuracy | STAB-06 | Content must match actual milestone work; no automated way to verify correctness | Review entry against phases 1-2 deliverables; verify Keep a Changelog format |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
