# Changelog

All notable changes to Agent Deck will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [1.9.66] - 2026-06-15

### Fixed

- **Web headless daemon (`agent-deck web --no-tui`) no longer exits immediately on startup** ([#1455](https://github.com/asheshgoplani/agent-deck/issues/1455)). The `--no-tui` path calls `server.Start()` and blocks until the HTTP server shuts down — but the signal handler installed at TUI-boot time called `os.Exit(0)` on `SIGINT`/`SIGTERM`/`SIGHUP` without first invoking the deferred `server.Shutdown(ctx)`. On systemd hosts the unit sends `SIGTERM` on `agent-deck web --no-tui` stop; the process exited before the 5-second graceful-shutdown window closed, leaving in-flight WebSocket sessions and PTY bridges without a clean teardown. Fix: the headless branch now installs its own `signal.Notify` handler that calls `server.Shutdown` with a 5-second context before `os.Exit`, mirroring the deferred shutdown already present for the TUI+web combined path. The existing TUI-path signal handler is unchanged. One new test in `cmd/agent-deck/web_headless_signal_test.go` (`TestHeadlessWebDaemon_GracefulShutdownOnSIGTERM`) spawns the real binary in headless mode against a random port, waits for the `/healthz` probe to return 200, sends `SIGTERM`, and asserts the process exits within 3 seconds with code 0 — a regression guard for the exact lifecycle the incident exposed.

## [1.9.65] - 2026-06-14

### Fixed

- **`agent-deck session send` no longer drops the first character of a message when the tmux pane is in copy mode** ([#1452](https://github.com/asheshgoplani/agent-deck/issues/1452), reported by @evgenii-at-dev). When a user had scrolled up in a pane (entering tmux copy mode), `send-keys` delivered keystrokes to the copy-mode buffer instead of the running process, so the first `q` that was supposed to exit copy mode consumed itself and the rest of the message landed one character short. Fix: `SendKeysAndEnter` now checks `#{pane_in_mode}` before sending; if the pane is in copy mode it sends `q` with a 50ms settle before the actual message. Regression test in `internal/tmux/send_keys_copy_mode_test.go`.