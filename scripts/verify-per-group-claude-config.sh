#!/usr/bin/env bash
# verify-per-group-claude-config.sh — CFG-05 visual harness.
#
# Creates two throwaway groups, launches one normal claude session and one
# custom-command session, asserts each session's CLAUDE_CONFIG_DIR matches the
# group override, prints a pass/fail table, and exits 0 iff both match.
#
# Usage: bash scripts/verify-per-group-claude-config.sh
#
# Requires: agent-deck (built from this branch), tmux, bash 4+, trash.
#
# The harness auto-detects a local ./build/agent-deck binary and uses it in
# preference to the system agent-deck, because per-group config injection
# requires the v1.5.4 changes from this branch.

set -euo pipefail

# --- Resolve agent-deck binary (prefer local build) ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
if [ -x "${REPO_ROOT}/build/agent-deck" ]; then
    AGENTDECK="${REPO_ROOT}/build/agent-deck"
else
    AGENTDECK="agent-deck"
fi

# Wrapper so all agent-deck calls use the resolved binary.
agent-deck() { "$AGENTDECK" "$@"; }
export -f agent-deck

# --- TTY-aware colors ---
if [ -t 1 ]; then
    GREEN=$'\033[32m'
    RED=$'\033[31m'
    RESET=$'\033[0m'
else
    GREEN=""
    RED=""
    RESET=""
fi

# --- Config ---
CONFIG_FILE="${HOME}/.agent-deck/config.toml"
BACKUP_FILE=""
GROUP_A="verify-group-a"
GROUP_B="verify-group-b"
# Capture target config dirs BEFORE we unset CLAUDE_CONFIG_DIR below, so that
# the user can override via env vars (e.g. CLAUDE_CONFIG_DIR_A=~/.work bash ...sh).
CONFIG_DIR_A="${CLAUDE_CONFIG_DIR_A:-${HOME}/.claude}"
CONFIG_DIR_B="${CLAUDE_CONFIG_DIR_B:-${HOME}/.claude-work}"
SESSION_A_TITLE="verify-session-a"
SESSION_B_TITLE="verify-session-b"
WRAPPER_SCRIPT=""
CAPTURE_DELAY=2.5  # allow extra time for claude TUI startup
POLL_TIMEOUT=5.0   # seconds to poll for the CLAUDE_CONFIG_DIR= line

# Unset CLAUDE_CONFIG_DIR for the duration of the harness.
# Priority chain: env var > group override > profile > global > default.
# If CLAUDE_CONFIG_DIR is already set in the environment, the env-var wins
# and group overrides are invisible. We must clear it so the group config
# stanzas injected into config.toml actually take effect.
_SAVED_CLAUDE_CONFIG_DIR="${CLAUDE_CONFIG_DIR:-}"
unset CLAUDE_CONFIG_DIR

# --- Preflight ---
preflight() {
    [ -x "$AGENTDECK" ] || command -v agent-deck >/dev/null 2>&1 || { echo "ERROR: agent-deck not on PATH and no ./build/agent-deck found" >&2; exit 2; }
    command -v tmux       >/dev/null 2>&1 || { echo "ERROR: tmux not on PATH" >&2; exit 2; }
    command -v trash      >/dev/null 2>&1 || { echo "ERROR: trash not on PATH (repo mandates trash for cleanup)" >&2; exit 2; }
    [ -f "$CONFIG_FILE" ] || { echo "ERROR: $CONFIG_FILE not found" >&2; exit 2; }
    [ -d "$CONFIG_DIR_A" ] || echo "WARN: $CONFIG_DIR_A does not exist — echo will still return the literal path string" >&2
    [ -d "$CONFIG_DIR_B" ] || echo "WARN: $CONFIG_DIR_B does not exist — echo will still return the literal path string" >&2
    echo "Using agent-deck: $AGENTDECK ($("$AGENTDECK" --version 2>/dev/null || echo 'unknown version'))"
}

# --- Best-effort pre-run cleanup (re-runnability on dirty workspace) ---
pre_cleanup() {
    agent-deck session stop   "$SESSION_A_TITLE" >/dev/null 2>&1 || true
    agent-deck session stop   "$SESSION_B_TITLE" >/dev/null 2>&1 || true
    agent-deck remove          "$SESSION_A_TITLE" >/dev/null 2>&1 || true
    agent-deck remove          "$SESSION_B_TITLE" >/dev/null 2>&1 || true
    agent-deck group delete    "$GROUP_A"         >/dev/null 2>&1 || true
    agent-deck group delete    "$GROUP_B"         >/dev/null 2>&1 || true
}

# --- Cleanup trap ---
cleanup() {
    set +e
    agent-deck session stop "$SESSION_A_TITLE" >/dev/null 2>&1 || true
    agent-deck session stop "$SESSION_B_TITLE" >/dev/null 2>&1 || true
    agent-deck remove       "$SESSION_A_TITLE" >/dev/null 2>&1 || true
    agent-deck remove       "$SESSION_B_TITLE" >/dev/null 2>&1 || true
    agent-deck group delete "$GROUP_A"         >/dev/null 2>&1 || true
    agent-deck group delete "$GROUP_B"         >/dev/null 2>&1 || true
    if [ -n "$BACKUP_FILE" ] && [ -f "$BACKUP_FILE" ]; then
        cp "$BACKUP_FILE" "$CONFIG_FILE"
        trash "$BACKUP_FILE" 2>/dev/null || true
    fi
    if [ -n "$WRAPPER_SCRIPT" ] && [ -f "$WRAPPER_SCRIPT" ]; then
        trash "$WRAPPER_SCRIPT" 2>/dev/null || true
    fi
    # Restore CLAUDE_CONFIG_DIR if it was set before the harness ran.
    if [ -n "$_SAVED_CLAUDE_CONFIG_DIR" ]; then
        export CLAUDE_CONFIG_DIR="$_SAVED_CLAUDE_CONFIG_DIR"
    fi
}
trap cleanup EXIT INT TERM

# --- Inject two group stanzas into config.toml (backup first) ---
inject_config() {
    BACKUP_FILE="$(mktemp -t agent-deck-config-backup.XXXXXX.toml)"
    cp "$CONFIG_FILE" "$BACKUP_FILE"
    cat >> "$CONFIG_FILE" <<EOF

# --- verify-per-group-claude-config.sh TEMPORARY STANZAS (will be removed on exit) ---
[groups."${GROUP_A}".claude]
config_dir = "${CONFIG_DIR_A}"

[groups."${GROUP_B}".claude]
config_dir = "${CONFIG_DIR_B}"
EOF
}

# --- Create wrapper script for custom-command session ---
# The filename intentionally contains "claude" so agent-deck's detectTool()
# maps it to Tool="claude" (not Tool="shell"). This is required because
# the CFG-02 export path (buildBashExportPrefix inside buildClaudeCommandWithMessage)
# only fires for IsClaudeCompatible sessions. Real conductors have the same
# constraint — they use commands that exec claude internally.
make_wrapper() {
    WRAPPER_SCRIPT="$(mktemp -t verify-per-group-claude-wrapper.XXXXXX.sh)"
    cat > "$WRAPPER_SCRIPT" <<'WRAP'
#!/usr/bin/env bash
# Throwaway wrapper: simulates conductor's custom-command path.
# Starts an interactive bash shell so the harness can echo the env var.
exec bash -i
WRAP
    chmod +x "$WRAPPER_SCRIPT"
}

# --- Get tmux session name for an agent-deck session ---
get_tmux_name() {
    local title="$1"
    agent-deck session show "$title" --json 2>/dev/null \
        | grep '"tmux_session"' \
        | sed 's/.*"tmux_session": *"\([^"]*\)".*/\1/'
}

# --- Poll raw tmux pane until the CLAUDE_CONFIG_DIR= line appears ---
# Uses tmux capture-pane directly so it works for both claude and custom-command
# sessions (agent-deck session output reads JSONL for claude sessions, which does
# not contain raw shell echo output).
poll_output() {
    local title="$1"
    local tmux_name
    tmux_name="$(get_tmux_name "$title")"
    if [ -z "$tmux_name" ]; then
        echo ""
        return 1
    fi
    local deadline
    deadline=$(awk -v t="$POLL_TIMEOUT" 'BEGIN{printf "%.3f", systime()+t}')
    while :; do
        local out
        out="$(tmux capture-pane -t "$tmux_name" -p 2>/dev/null || true)"
        if echo "$out" | grep -qE 'CLAUDE_CONFIG_DIR='; then
            # Extract just the value after CLAUDE_CONFIG_DIR= (strip any leading decoration)
            echo "$out" | grep -oE 'CLAUDE_CONFIG_DIR=[^ ]*' | tail -n 1
            return 0
        fi
        local now
        now=$(awk 'BEGIN{printf "%.3f", systime()}')
        awk -v a="$now" -v b="$deadline" 'BEGIN{exit !(a>=b)}' && break
        sleep 0.25
    done
    echo ""
    return 1
}

# --- Main ---
main() {
    preflight
    pre_cleanup
    inject_config
    make_wrapper

    # Create & start session A — normal claude session in verify-group-a
    agent-deck group  create "$GROUP_A" >/dev/null
    agent-deck group  create "$GROUP_B" >/dev/null
    agent-deck add    "$HOME" -t "$SESSION_A_TITLE" -c claude -g "$GROUP_A" >/dev/null
    agent-deck add    "$HOME" -t "$SESSION_B_TITLE" -c "$WRAPPER_SCRIPT" -g "$GROUP_B" >/dev/null
    agent-deck session start "$SESSION_A_TITLE" >/dev/null
    agent-deck session start "$SESSION_B_TITLE" >/dev/null
    sleep "$CAPTURE_DELAY"

    # Assertion pipeline: read CLAUDE_CONFIG_DIR from the tmux pane's spawn environment.
    # We use /proc/<pane_pid>/environ (Linux) or send an echo via tmux send-keys (fallback).
    # Polling /proc is instantaneous and does not require waiting for Claude to be "ready".
    local val_a val_b result_a result_b passes=0

    get_pane_env() {
        local tmux_name="$1"
        local pane_pid
        pane_pid="$(tmux display-message -t "$tmux_name" -p '#{pane_pid}' 2>/dev/null)"
        if [ -n "$pane_pid" ] && [ -f "/proc/$pane_pid/environ" ]; then
            cat "/proc/$pane_pid/environ" 2>/dev/null | tr '\0' '\n' | grep -oE 'CLAUDE_CONFIG_DIR=[^ ]*' | head -1 || echo ""
        else
            # Fallback: send echo via tmux and poll pane output
            tmux send-keys -t "$tmux_name" "echo CLAUDE_CONFIG_DIR=\$CLAUDE_CONFIG_DIR" Enter 2>/dev/null || true
            sleep 1
            tmux capture-pane -t "$tmux_name" -p 2>/dev/null | grep -oE 'CLAUDE_CONFIG_DIR=[^ ]*' | tail -1 || echo ""
        fi
    }

    tmux_a="$(get_tmux_name "$SESSION_A_TITLE")"
    tmux_b="$(get_tmux_name "$SESSION_B_TITLE")"

    line_a="$(get_pane_env "$tmux_a")"
    line_b="$(get_pane_env "$tmux_b")"
    val_a="${line_a#CLAUDE_CONFIG_DIR=}"
    val_b="${line_b#CLAUDE_CONFIG_DIR=}"

    # Resolve ~/ in expected for comparison
    local exp_a exp_b
    exp_a="${CONFIG_DIR_A/#\~/$HOME}"
    exp_b="${CONFIG_DIR_B/#\~/$HOME}"

    if [ "$val_a" = "$exp_a" ] || [ "$val_a" = "$CONFIG_DIR_A" ]; then
        result_a="${GREEN}✓${RESET}"; passes=$((passes+1))
    else
        result_a="${RED}✗${RESET}"
    fi
    if [ "$val_b" = "$exp_b" ] || [ "$val_b" = "$CONFIG_DIR_B" ]; then
        result_b="${GREEN}✓${RESET}"; passes=$((passes+1))
    else
        result_b="${RED}✗${RESET}"
    fi

    # Print table
    printf "\n"
    printf "| %-15s | %-16s | %-28s | %-22s | %-6s |\n" "Group" "Session Type" "Resolved CLAUDE_CONFIG_DIR" "Expected" "Result"
    printf "|%-17s|%-18s|%-30s|%-24s|%-8s|\n" "-----------------" "------------------" "------------------------------" "------------------------" "--------"
    printf "| %-15s | %-16s | %-28s | %-22s | %-6b |\n" "$GROUP_A" "normal" "$val_a" "$exp_a" "$result_a"
    printf "| %-15s | %-16s | %-28s | %-22s | %-6b |\n" "$GROUP_B" "custom-command" "$val_b" "$exp_b" "$result_b"
    printf "\n"

    if [ "$passes" -eq 2 ]; then
        printf "%bPASS: 2/2%b\n" "$GREEN" "$RESET"
        exit 0
    else
        printf "%bFAIL: %d/2%b\n" "$RED" "$passes" "$RESET"
        exit 1
    fi
}

main "$@"
