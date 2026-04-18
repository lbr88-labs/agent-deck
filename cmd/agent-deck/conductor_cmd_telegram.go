package main

import (
	"io"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

// Conductor telegram-topology CLI glue (fix v1.7.22, issue #658).
//
// readTelegramGloballyEnabled inspects a Claude Code profile's settings.json
// and reports whether "telegram@claude-plugins-official" is enabled as a
// profile-wide plugin (the root-cause-A anti-pattern). Missing file and
// missing key both map to false.
//
// emitTelegramWarnings runs the validator and writes human-facing warnings
// to w. Silent on a clean configuration.
//
// Stubs during RED phase (Phase 3 TDD) — real logic lands in Phase 5.

func readTelegramGloballyEnabled(configDir string) (bool, error) {
	return false, nil
}

func emitTelegramWarnings(w io.Writer, in session.TelegramValidatorInput) {
	// stub: no output until Phase 5
	_ = w
	_ = in
}
