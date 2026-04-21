# GitHub Actions workflows

This directory holds the CI/CD gates and automation for agent-deck. After the
v1.7.42 audit (#682), every workflow here is either (a) green on main, (b)
green when its trigger path fires, or (c) explicitly alert-only off the PR
path. No PR should merge with a red check unless the failure is a real,
actionable regression.

## Active PR gates (block merge)

These run on pull requests and **must go green** before merge.

| Workflow | Trigger | What it gates |
|---|---|---|
| `session-persistence.yml` | PR touching tmux/session lifecycle paths, or `workflow_dispatch` | The eight `TestPersistence_*` tests (race-detector on) plus `scripts/verify-session-persistence.sh` end-to-end. Covers the class of bug where a single SSH logout destroys every managed tmux session on Linux+systemd. See the "Session persistence: mandatory test coverage" section in the root `CLAUDE.md`. |

Any other red on a PR is either a pre-release workflow (see below) or a bug —
file an issue and fix it, don't merge through it.

## Release automation (tag-triggered)

| Workflow | Trigger | What it does |
|---|---|---|
| `release.yml` | push tag `v*` | Validates the tag matches `cmd/agent-deck/main.go`'s `Version`, runs `go test -race ./...`, runs `goreleaser --clean` to build Darwin/Linux × amd64/arm64 tarballs, publishes the GitHub Release, and asserts the expected five assets + `checksums.txt` landed. Replaces the pre-#332 manual `make release-local` step. |
| `pages.yml` | push to `main` touching `site/**`, or `workflow_dispatch` | Deploys the static landing site under `site/` to GitHub Pages. |

## Notification-only (no gate, no build)

| Workflow | Trigger | What it does |
|---|---|---|
| `issue-notify.yml` | issue opened | Posts issue context (title, body, labels, related issues, recent commits) to the configured ntfy topic so the conductor picks it up. |
| `pr-notify.yml` | PR opened or marked ready-for-review | Posts PR context (files, commits, reviews, comments) to the same ntfy topic. |

Both expect `secrets.NTFY_TOPIC` to be set on the repo. Neither blocks
anything — they can fail silently without affecting merges.

## Schedule-only (alert-only, not a PR gate)

| Workflow | Trigger | What it does |
|---|---|---|
| `weekly-regression.yml` | Sunday 00:00 UTC cron, or `workflow_dispatch` | Runs the Playwright visual-regression suite (`tests/e2e/pw-visual-regression.config.ts`) and Lighthouse CI (`.lighthouserc.json`) against a freshly built `agent-deck web` server. On failure, opens or appends to a single `Weekly regression check: … [date]` issue labelled `regression,automated` (idempotent — no duplicate issues on back-to-back failures). **Alert-only** — does not block any PR. |

> **Note on weekly-regression reliability (as of v1.7.42):** the underlying
> `agent-deck web` test server currently fails to start in fully headless CI
> because a transitive bubbletea import tries to open a cancel-reader on a
> non-existent TTY (`error creating cancelreader: bubbletea: error creating
> cancel reader: add reader to epoll interest list`). Both the visual and
> Lighthouse steps therefore fail with `ERR_CONNECTION_REFUSED`. Because
> `weekly-regression.yml` is alert-only and idempotent, this produces at most
> one open issue per week, not a flood. Fixing the server-start path (PTY
> wrapper or a `--no-tui` startup flag) is tracked as a stability-ledger
> follow-up; until then, the weekly issue is a known false positive.

## Deliberately removed in v1.7.42 (#682)

These PR gates were deleted in v1.7.42 because they were red on every run and
were teaching the team to ignore red checks (the worst possible CI
behaviour). Reach the repo at a commit before v1.7.42 if you want the
original files:

| Workflow | Why it was removed |
|---|---|
| `visual-regression.yml` | Ran on every PR. Broken since the bubbletea/TTY regression — `agent-deck web` never binds, every Playwright spec fails with `ERR_CONNECTION_REFUSED`. Same test matrix still runs weekly via `weekly-regression.yml`. Developers can re-run locally with `cd tests/e2e && npx playwright test --config=pw-visual-regression.config.ts` against a local `agent-deck web`. |
| `lighthouse-ci.yml` | Ran on PRs touching `internal/web/**` or `.lighthouserc.json`. Has never passed since 2026-04-10 — same bubbletea server-start failure as above, plus the performance budget in `.lighthouserc.json` was never re-baselined against current webui bundle size. Same Lighthouse suite still runs weekly via `weekly-regression.yml`. Local run: `./tests/lighthouse/calibrate.sh` to re-baseline, then `npx lhci autorun --config=.lighthouserc.json`. |

If you want to reinstate either one as a PR gate, first fix the underlying
`agent-deck web` headless-startup issue, re-baseline the relevant budget
file, and land the change in the same PR so it stays green from run #1. Red
gates worse than no gates.

## Adding a new workflow

1. The gate must be green on the first merged run. If you can't guarantee
   that, make it `workflow_dispatch`-only or `continue-on-error: true` until
   it is.
2. Add a row in this README.
3. Reference the CLAUDE.md section in the project root if the gate is
   mandatory (see `session-persistence.yml` for the pattern).
