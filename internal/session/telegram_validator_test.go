package session

import (
	"strings"
	"testing"
)

// Telegram-topology validator tests — v1.7.23 semantic reversal (#661 scope).
//
// v1.7.22 warned when enabledPlugins.telegram@...=true in the profile's
// settings.json, on the theory that every claude session loaded the plugin
// and started a redundant bun poller. Production verification disproved
// that: --channels only SUBSCRIBES a session to a channel; it does not
// force-load the plugin. If enabledPlugins.telegram@... is false, the
// plugin is never loaded, --channels is a no-op, and the conductor has no
// telegram bridge at all.
//
// v1.7.23 therefore reverses the warning direction:
//
//   CHANNELS_WITHOUT_GLOBAL_PLUGIN — conductor subscribes via --channels
//       but enabledPlugins is false → plugin never loads → silent bridge
//       outage. The correct topology is global=true + --channels per
//       conductor + env_file per conductor for TELEGRAM_STATE_DIR.
//
//   WRAPPER_DEPRECATED — unchanged. Wrapper-based TELEGRAM_STATE_DIR is
//       still unreliable on the fresh-start path; env_file is canonical.
//
// The v1.7.22 GLOBAL_ANTIPATTERN and DOUBLE_LOAD codes are removed —
// both rested on a mis-model of claude's plugin loader.

func TestTelegramValidator_GlobalEnabledWithChannels_NoWarning(t *testing.T) {
	// Canonical supported topology: plugin loaded globally, one conductor
	// session subscribes to the channel via --channels. Must be silent.
	in := TelegramValidatorInput{
		GlobalEnabled: true,
		SessionChannels: []string{
			"plugin:telegram@claude-plugins-official",
		},
		SessionWrapper: "",
	}
	got := ValidateTelegramTopology(in)
	if len(got) != 0 {
		t.Fatalf("canonical topology must produce zero warnings, got %d: %+v", len(got), got)
	}
}

func TestTelegramValidator_GlobalEnabled_NoChannels_NoWarning(t *testing.T) {
	// Plugin globally loaded, ordinary session that doesn't own a channel.
	// This is fine — non-subscribing sessions coexist with a single
	// channel-owning session. Must be silent.
	in := TelegramValidatorInput{
		GlobalEnabled:   true,
		SessionChannels: nil,
		SessionWrapper:  "",
	}
	got := ValidateTelegramTopology(in)
	if len(got) != 0 {
		t.Fatalf("global-enabled + no channels must be silent, got %d: %+v", len(got), got)
	}
}

func TestTelegramValidator_GlobalDisabled_NoChannels_NoWarning(t *testing.T) {
	// Plugin disabled globally, session doesn't use telegram — nothing to
	// warn about. Must be silent.
	in := TelegramValidatorInput{
		GlobalEnabled:   false,
		SessionChannels: nil,
		SessionWrapper:  "",
	}
	got := ValidateTelegramTopology(in)
	if len(got) != 0 {
		t.Fatalf("global-disabled + no channels must be silent, got %d: %+v", len(got), got)
	}
}

func TestTelegramValidator_GlobalDisabled_WithChannels_WarnChannelsWithoutPlugin(t *testing.T) {
	// The real v1.7.23-corrected misconfiguration: conductor asks for a
	// telegram channel subscription but the plugin isn't enabled globally,
	// so the plugin never loads and --channels silently no-ops.
	in := TelegramValidatorInput{
		GlobalEnabled: false,
		SessionChannels: []string{
			"plugin:telegram@claude-plugins-official",
		},
		SessionWrapper: "",
	}
	got := ValidateTelegramTopology(in)

	var warn *TelegramWarning
	for i := range got {
		if got[i].Code == "CHANNELS_WITHOUT_GLOBAL_PLUGIN" {
			warn = &got[i]
			break
		}
	}
	if warn == nil {
		t.Fatalf("expected CHANNELS_WITHOUT_GLOBAL_PLUGIN warning, got %+v", got)
	}
	msg := strings.ToLower(warn.Message)
	if !strings.Contains(msg, "enabledplugins") {
		t.Errorf("message must reference enabledPlugins so users can find the setting, got: %s", warn.Message)
	}
	if !strings.Contains(msg, "--channels") {
		t.Errorf("message must reference --channels, got: %s", warn.Message)
	}
	if !strings.Contains(msg, "force-load") && !strings.Contains(msg, "never load") {
		t.Errorf("message should explain that --channels does not force-load, got: %s", warn.Message)
	}
}

func TestTelegramValidator_GlobalDisabled_WithNonTelegramChannels_NoWarning(t *testing.T) {
	// Ensure prefix match is correct — a slack channel must not trigger
	// the telegram warning.
	in := TelegramValidatorInput{
		GlobalEnabled: false,
		SessionChannels: []string{
			"plugin:slack@claude-plugins-official",
		},
		SessionWrapper: "",
	}
	got := ValidateTelegramTopology(in)
	if len(got) != 0 {
		t.Fatalf("non-telegram channels must not trigger telegram warnings, got %+v", got)
	}
}

func TestTelegramValidator_WrapperStateDir_AntiPattern(t *testing.T) {
	// WRAPPER_DEPRECATED remains unchanged in v1.7.23: env_file is still
	// the canonical mechanism; wrapper-based injection is still unreliable.
	in := TelegramValidatorInput{
		GlobalEnabled: true,
		SessionChannels: []string{
			"plugin:telegram@claude-plugins-official",
		},
		SessionWrapper: "TELEGRAM_STATE_DIR=/home/me/.claude/channels/telegram {command}",
	}
	got := ValidateTelegramTopology(in)

	var w *TelegramWarning
	for i := range got {
		if got[i].Code == "WRAPPER_DEPRECATED" {
			w = &got[i]
			break
		}
	}
	if w == nil {
		t.Fatalf("expected WRAPPER_DEPRECATED warning, got %+v", got)
	}
	if !strings.Contains(w.Message, "env_file") {
		t.Errorf("WRAPPER_DEPRECATED message must recommend env_file, got: %s", w.Message)
	}
}

func TestTelegramValidator_WrapperStateDir_NoTelegramChannel_NoWarning(t *testing.T) {
	// Wrapper-based TELEGRAM_STATE_DIR on a session without a telegram
	// channel is harmless (nothing to poll). Must not warn.
	in := TelegramValidatorInput{
		GlobalEnabled:   true,
		SessionChannels: nil,
		SessionWrapper:  "TELEGRAM_STATE_DIR=/tmp/x {command}",
	}
	got := ValidateTelegramTopology(in)
	for _, w := range got {
		if w.Code == "WRAPPER_DEPRECATED" {
			t.Errorf("no telegram channel: must not emit WRAPPER_DEPRECATED, got %+v", w)
		}
	}
}

// Guard: the v1.7.22 codes must not re-appear. Removing these is a
// semantic promise of v1.7.23 — any reintroduction should break a test.
func TestTelegramValidator_v1_7_22_CodesRemoved(t *testing.T) {
	cases := []TelegramValidatorInput{
		{GlobalEnabled: true, SessionChannels: nil, SessionWrapper: ""},
		{GlobalEnabled: true, SessionChannels: []string{"plugin:telegram@claude-plugins-official"}, SessionWrapper: ""},
		{GlobalEnabled: false, SessionChannels: []string{"plugin:telegram@claude-plugins-official"}, SessionWrapper: ""},
	}
	for _, in := range cases {
		got := ValidateTelegramTopology(in)
		for _, w := range got {
			if w.Code == "GLOBAL_ANTIPATTERN" {
				t.Errorf("GLOBAL_ANTIPATTERN removed in v1.7.23 — reintroduced for input %+v: %+v", in, w)
			}
			if w.Code == "DOUBLE_LOAD" {
				t.Errorf("DOUBLE_LOAD removed in v1.7.23 — reintroduced for input %+v: %+v", in, w)
			}
		}
	}
}
