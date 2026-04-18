package session

// Telegram-topology validator.
//
// Surfaces three anti-patterns that cause telegram poller leaks and 409
// Conflict lockouts across conductor hosts (see fix v1.7.22, issue #658):
//   GLOBAL_ANTIPATTERN — enabledPlugins."telegram@claude-plugins-official"=true
//                        in a profile settings.json makes every claude
//                        session load the plugin.
//   DOUBLE_LOAD        — global + --channels on the same session makes the
//                        plugin load twice in one process → 409.
//   WRAPPER_DEPRECATED — TELEGRAM_STATE_DIR injected via session wrapper is
//                        unreliable on the fresh-start path. Use
//                        [conductors.<name>.claude].env_file instead.
//
// Pure and side-effect-free. CLI layer owns I/O (reading settings.json) and
// presentation (formatting warnings on stderr).

// TelegramValidatorInput captures the three signals the validator inspects.
type TelegramValidatorInput struct {
	// GlobalEnabled is the value of
	// enabledPlugins."telegram@claude-plugins-official" in the relevant
	// profile's settings.json, or false if that file or key is absent.
	GlobalEnabled bool

	// SessionChannels is the list of channels the session is launched with
	// (Instance.Channels). Empty for ordinary child sessions.
	SessionChannels []string

	// SessionWrapper is the wrapper template for the session (may be empty).
	SessionWrapper string
}

// TelegramWarning is one emission from the validator.
type TelegramWarning struct {
	Code    string // GLOBAL_ANTIPATTERN | DOUBLE_LOAD | WRAPPER_DEPRECATED
	Message string
}

// ValidateTelegramTopology returns zero or more warnings for the given
// session configuration. Stub during RED phase (Phase 3 TDD) — real logic
// lands in Phase 5.
func ValidateTelegramTopology(in TelegramValidatorInput) []TelegramWarning {
	return nil
}
