package session

import "strings"

// Telegram-topology validator.
//
// v1.7.23 semantic reversal (#661 scope): v1.7.22 warned when
// enabledPlugins.telegram@...=true on the theory that every claude session
// would load the plugin and start a redundant bun poller. Production
// verification disproved this. --channels only SUBSCRIBES a session to a
// channel; it does not force-load the plugin. If enabledPlugins is false,
// the plugin is never loaded, --channels is a silent no-op, and the
// conductor has no telegram bridge at all.
//
// The supported conductor topology is therefore:
//
//   1. enabledPlugins."telegram@claude-plugins-official"=true globally —
//      this is how the plugin actually loads into the claude process.
//   2. --channels plugin:telegram@... on each conductor session — this
//      subscribes the conductor to the bot's updates.
//   3. [conductors.<name>.claude].env_file per conductor — deterministic
//      TELEGRAM_STATE_DIR injection on both fresh-start and resume paths.
//
// The only current topology antipatterns are:
//
//	CHANNELS_WITHOUT_GLOBAL_PLUGIN — conductor subscribes via --channels
//	                                 but enabledPlugins is false → plugin
//	                                 never loads → silent bridge outage.
//	WRAPPER_DEPRECATED             — TELEGRAM_STATE_DIR injected via
//	                                 session wrapper; unreliable on
//	                                 fresh-start due to bash -c argv
//	                                 splitting. Use env_file instead.
//
// Pure and side-effect-free. CLI layer owns I/O (reading settings.json) and
// presentation (formatting warnings on stderr).

// telegramChannelPrefix matches channel ids like
// "plugin:telegram@claude-plugins-official" or any other
// "plugin:telegram@<owner>/<repo>" variant. We match by prefix so forks and
// repo renames still trigger the guard.
const telegramChannelPrefix = "plugin:telegram@"

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
	Code    string // CHANNELS_WITHOUT_GLOBAL_PLUGIN | WRAPPER_DEPRECATED
	Message string
}

// ValidateTelegramTopology returns zero or more warnings for the given
// session configuration. Ordering is stable:
// CHANNELS_WITHOUT_GLOBAL_PLUGIN, WRAPPER_DEPRECATED.
func ValidateTelegramTopology(in TelegramValidatorInput) []TelegramWarning {
	hasTelegramChannel := false
	for _, ch := range in.SessionChannels {
		if strings.HasPrefix(ch, telegramChannelPrefix) {
			hasTelegramChannel = true
			break
		}
	}

	var out []TelegramWarning

	if hasTelegramChannel && !in.GlobalEnabled {
		out = append(out, TelegramWarning{
			Code:    "CHANNELS_WITHOUT_GLOBAL_PLUGIN",
			Message: `Session subscribes via --channels plugin:telegram@... but enabledPlugins."telegram@claude-plugins-official"=false in the profile settings.json. --channels only SUBSCRIBES a session to a channel — it does not force-load the plugin. With global disabled the plugin never loads, the bun poller never starts, and this conductor has no telegram bridge. Fix: set enabledPlugins."telegram@claude-plugins-official"=true in the profile settings.json so the plugin loads; --channels then subscribes each conductor session. Canonical topology is global=true + per-conductor --channels + per-conductor [conductors.<name>.claude].env_file for TELEGRAM_STATE_DIR.`,
		})
	}

	if hasTelegramChannel && strings.Contains(in.SessionWrapper, "TELEGRAM_STATE_DIR=") {
		out = append(out, TelegramWarning{
			Code:    "WRAPPER_DEPRECATED",
			Message: `The session wrapper injects TELEGRAM_STATE_DIR. This path is deprecated because bash -c argv splitting makes it unreliable on the fresh-start path (claude without --resume). Inject the variable via [conductors.<name>.claude].env_file in ~/.agent-deck/config.toml — it is sourced deterministically on both fresh and resume spawns.`,
		})
	}

	return out
}
