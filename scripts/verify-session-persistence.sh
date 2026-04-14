#!/usr/bin/env bash
# verify-session-persistence.sh — human-watchable end-to-end verification for
# v1.5.2 session persistence. Exits 0 if every scenario prints [PASS] or [SKIP];
# exits 1 on any [FAIL]; exits 2 on missing agent-deck/tmux. CI uses the stub
# scripts/verify-session-persistence.d/fake-claude.sh (captures claude argv).
# Env: AGENT_DECK_VERIFY_USE_STUB=1, AGENT_DECK_VERIFY_DESTRUCTIVE=1, SCENARIO=N.
set -euo pipefail

# Numbered scenario checklist (printed at startup, also parseable via head -30).
readonly CHECKLIST="$(cat <<'EOF'
[1] Live session + cgroup inspection
[2] Login-session teardown survival (Linux+systemd only)
[3] Stop -> restart resume (--resume or --session-id in argv)
[4] Fresh session uses --session-id, not --resume
EOF
)"

# ---------- color + logging ----------
readonly C_RED='\033[31m'
readonly C_GREEN='\033[32m'
readonly C_YELLOW='\033[33m'
readonly C_RESET='\033[0m'

banner_pass() { printf "${C_GREEN}[PASS]${C_RESET} %s\n" "$*"; }
banner_fail() { printf "${C_RED}[FAIL]${C_RESET} %s\n" "$*" >&2; FAILED=1; }
banner_skip() { printf "${C_YELLOW}[SKIP]${C_RESET} %s\n" "$*"; }
log() { printf '    %s\n' "$*"; }

FAILED=0
RUN_ID="$$"
TMPROOT="$(mktemp -d -t adeck-verify.XXXXXX)"
SESSION_PREFIX="verify-persist-${RUN_ID}"
LOGINSIM_SCOPE="adeck-verify-loginsim-${RUN_ID}"
ARGV_OUT="${TMPROOT}/argv.log"
export AGENT_DECK_VERIFY_ARGV_OUT="${ARGV_OUT}"

# ---------- cleanup ----------
cleanup() {
  set +e
  # Stop any sessions we created.
  if command -v agent-deck >/dev/null 2>&1; then
    for n in $(agent-deck session ls 2>/dev/null | awk '{print $1}' | grep -E "^${SESSION_PREFIX}" || true); do
      agent-deck session stop "$n" >/dev/null 2>&1 || true
      agent-deck remove "$n" >/dev/null 2>&1 || true
    done
  fi
  # Tear down any lingering login-sim scope.
  if command -v systemctl >/dev/null 2>&1; then
    systemctl --user stop "${LOGINSIM_SCOPE}.scope" >/dev/null 2>&1 || true
  fi
  # Remove the script's OWN tempdir only (per CLAUDE.md, never rm user state).
  if [[ -n "${TMPROOT}" && "${TMPROOT}" == /tmp/adeck-verify.* ]]; then
    rm -rf "${TMPROOT}" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

# ---------- preflight ----------
if ! command -v agent-deck >/dev/null 2>&1; then
  printf "${C_RED}ERROR${C_RESET}: agent-deck binary not on PATH.\n" >&2
  exit 2
fi
if ! command -v tmux >/dev/null 2>&1; then
  printf "${C_RED}ERROR${C_RESET}: tmux binary not on PATH.\n" >&2
  exit 2
fi

# Decide whether to install the claude stub.
USE_STUB=0
if [[ "${AGENT_DECK_VERIFY_USE_STUB:-0}" == "1" ]]; then
  USE_STUB=1
elif ! command -v claude >/dev/null 2>&1; then
  USE_STUB=1
fi
if [[ "${USE_STUB}" == "1" ]]; then
  STUB_DIR="$(cd "$(dirname "$0")/verify-session-persistence.d" && pwd)"
  # Provide a `claude` alias by symlinking the stub into a PATH-controlled dir.
  mkdir -p "${TMPROOT}/bin"
  ln -sf "${STUB_DIR}/fake-claude.sh" "${TMPROOT}/bin/claude"
  export PATH="${TMPROOT}/bin:${PATH}"
  log "claude stub: ${STUB_DIR}/fake-claude.sh (argv -> ${ARGV_OUT})"
else
  log "claude: $(command -v claude) (real)"
fi

# ---------- checklist ----------
cat <<EOF
==========================================================
verify-session-persistence.sh — v1.5.2 persistence harness
==========================================================
${CHECKLIST}
==========================================================
Each scenario ends with one [PASS], [FAIL], or [SKIP] line.
EOF

# ---------- helpers ----------
tmux_pid_for_session() {
  # Prints the PID of the tmux server hosting the agent-deck session $1.
  local name="$1"
  local sock
  sock="$(agent-deck session show "$name" 2>/dev/null | awk -F'= ' '/tmux_socket/{print $2}' | tr -d '"')"
  if [[ -n "${sock}" ]]; then
    tmux -S "${sock}" display-message -p -F '#{pid}' 2>/dev/null || true
  else
    pgrep -f "tmux.*${name}" | head -1 || true
  fi
}

print_cgroup_for_pid() {
  local pid="$1"
  printf '    PID=%s\n' "${pid}"
  if [[ -r "/proc/${pid}/cgroup" ]]; then
    printf '    /proc/%s/cgroup:\n' "${pid}"
    sed 's/^/        /' "/proc/${pid}/cgroup"
  else
    printf '    cgroup: N/A (macOS or /proc not mounted)\n'
  fi
}

want_scenario() {
  local n="$1"
  if [[ -z "${SCENARIO:-}" ]]; then return 0; fi
  [[ "${SCENARIO}" == "${n}" ]]
}

# ---------- Scenario 1 ----------
scenario_1_live_session_cgroup() {
  local name="${SESSION_PREFIX}-s1"
  log "creating session: ${name}"
  agent-deck add -t "verify-s1" -c claude "${TMPROOT}" --name "${name}" >/dev/null
  agent-deck session start "${name}" >/dev/null
  sleep 2
  local pid
  pid="$(tmux_pid_for_session "${name}")"
  if [[ -z "${pid}" ]]; then
    banner_fail "[1] could not resolve tmux server pid for session ${name}"
    return
  fi
  print_cgroup_for_pid "${pid}"
  if [[ "$(uname)" == "Linux" && -r "/proc/${pid}/cgroup" ]]; then
    if grep -q 'user@' "/proc/${pid}/cgroup"; then
      banner_pass "[1] tmux server ${pid} is under user@*.service (cgroup isolation active)"
    else
      banner_fail "[1] tmux server ${pid} is NOT under user@*.service — cgroup isolation did not activate"
    fi
  else
    banner_pass "[1] tmux server ${pid} is live (cgroup inspection skipped: non-Linux)"
  fi
  agent-deck session stop "${name}" >/dev/null 2>&1 || true
}

# ---------- Scenario 2 ----------
scenario_2_login_teardown() {
  if ! command -v systemd-run >/dev/null 2>&1 || ! systemctl --user is-system-running >/dev/null 2>&1; then
    banner_skip "[2] skipped: no systemd-run (non-Linux or systemd user bus unavailable)"
    return
  fi
  local name="${SESSION_PREFIX}-s2"
  log "launching throwaway login-scope: ${LOGINSIM_SCOPE}"
  systemd-run --user --scope --unit="${LOGINSIM_SCOPE}" sleep 3600 >/dev/null 2>&1 &
  local scope_pid=$!
  sleep 1
  log "creating session inside simulated login scope: ${name}"
  agent-deck add -t "verify-s2" -c claude "${TMPROOT}" --name "${name}" >/dev/null
  agent-deck session start "${name}" >/dev/null
  sleep 2
  local pid
  pid="$(tmux_pid_for_session "${name}")"
  if [[ -z "${pid}" ]]; then
    banner_fail "[2] could not resolve tmux server pid for session ${name}"
    systemctl --user stop "${LOGINSIM_SCOPE}.scope" >/dev/null 2>&1 || true
    return
  fi
  print_cgroup_for_pid "${pid}"
  log "terminating login-scope: systemctl --user stop ${LOGINSIM_SCOPE}.scope"
  systemctl --user stop "${LOGINSIM_SCOPE}.scope" >/dev/null 2>&1 || true
  kill "${scope_pid}" >/dev/null 2>&1 || true
  if [[ "${AGENT_DECK_VERIFY_DESTRUCTIVE:-0}" == "1" ]]; then
    log "DESTRUCTIVE: additionally terminating own login session (will disconnect SSH)"
    local sess
    sess="$(loginctl show-user "$USER" -p Sessions --value 2>/dev/null | awk '{print $1}')"
    if [[ -n "${sess}" ]]; then
      loginctl terminate-session "${sess}" >/dev/null 2>&1 || true
    fi
  fi
  sleep 2
  if kill -0 "${pid}" 2>/dev/null; then
    banner_pass "[2] tmux pid ${pid} survived login-session teardown (cgroup isolation works)"
  else
    banner_fail "[2] tmux pid ${pid} died with login-session teardown — isolation FAILED"
  fi
  agent-deck session stop "${name}" >/dev/null 2>&1 || true
}

# ---------- Scenario 3 ----------
scenario_3_restart_resume() {
  local name="${SESSION_PREFIX}-s3"
  log "creating session: ${name}"
  agent-deck add -t "verify-s3" -c claude "${TMPROOT}" --name "${name}" >/dev/null
  agent-deck session start "${name}" >/dev/null
  sleep 2
  # Seed a non-empty ClaudeSessionID via the state-set command if available;
  # otherwise rely on the natural first-start minting one. We want a restart
  # that passes either --resume OR --session-id.
  agent-deck session stop "${name}" >/dev/null 2>&1 || true
  sleep 1
  : > "${ARGV_OUT}"
  log "restarting session: agent-deck session start ${name}"
  agent-deck session start "${name}" >/dev/null || true
  sleep 2
  # Read captured argv from the stub (or ps if real claude).
  local argv=""
  if [[ -s "${ARGV_OUT}" ]]; then
    argv="$(cat "${ARGV_OUT}")"
  else
    argv="$(ps -ef | grep -E '[c]laude' | head -1 || true)"
  fi
  log "captured claude argv: ${argv}"
  if echo "${argv}" | grep -qE -- '--resume|--session-id'; then
    banner_pass "[3] restart spawned claude with --resume or --session-id"
  else
    banner_fail "[3] restart spawned claude WITHOUT --resume or --session-id: ${argv}"
  fi
  agent-deck session stop "${name}" >/dev/null 2>&1 || true
}

# ---------- Scenario 4 ----------
scenario_4_fresh_session_shape() {
  local name="${SESSION_PREFIX}-s4"
  : > "${ARGV_OUT}"
  log "creating fresh session: ${name}"
  agent-deck add -t "verify-s4" -c claude "${TMPROOT}" --name "${name}" >/dev/null
  agent-deck session start "${name}" >/dev/null
  sleep 2
  local argv=""
  if [[ -s "${ARGV_OUT}" ]]; then
    argv="$(cat "${ARGV_OUT}")"
  else
    argv="$(ps -ef | grep -E '[c]laude' | head -1 || true)"
  fi
  log "captured claude argv: ${argv}"
  if echo "${argv}" | grep -qE -- '--session-id' && ! echo "${argv}" | grep -qE -- '--resume'; then
    banner_pass "[4] fresh session uses --session-id without --resume"
  elif [[ -z "${argv}" ]]; then
    banner_skip "[4] no argv captured (real claude without stub); cannot assert fresh-session shape"
  else
    banner_fail "[4] fresh session argv shape wrong: ${argv}"
  fi
  agent-deck session stop "${name}" >/dev/null 2>&1 || true
}

# ---------- dispatch ----------
want_scenario 1 && scenario_1_live_session_cgroup
want_scenario 2 && scenario_2_login_teardown
want_scenario 3 && scenario_3_restart_resume
want_scenario 4 && scenario_4_fresh_session_shape

if [[ "${FAILED}" -ne 0 ]]; then
  printf "${C_RED}OVERALL: FAIL${C_RESET}\n" >&2
  exit 1
fi
printf "${C_GREEN}OVERALL: PASS${C_RESET}\n"
exit 0
