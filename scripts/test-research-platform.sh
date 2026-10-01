#!/bin/bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
TEST_DIR="$(mktemp -d)"
holder=''
cleanup() {
  if [[ -n "$holder" ]]; then kill -TERM "$holder" 2>/dev/null || true; wait "$holder" || true; fi
  rm -rf "$TEST_DIR"
}
trap cleanup EXIT
source "$ROOT/scripts/research-platform.sh"

# Platform defaults are deterministic; the caller's explicit overrides win.
(
  HOME="$TEST_DIR/home"
  unset XDG_STATE_HOME XDG_CONFIG_HOME
  uname() { printf 'Linux\n'; }
  research_platform_init
  [[ "$RESEARCH_LOCK_COMMAND" == flock ]]
  [[ "$RESEARCH_DEFAULT_LOG_DIR" == "$HOME/.local/state/ai-engineering-kit" ]]
  [[ "$RESEARCH_DEFAULT_ENV_FILE" == "$HOME/.config/ai-engineering-kit/research.env" ]]
  XDG_STATE_HOME="$TEST_DIR/state space" XDG_CONFIG_HOME="$TEST_DIR/config space"
  research_platform_init
  [[ "$RESEARCH_DEFAULT_LOG_DIR" == "$XDG_STATE_HOME/ai-engineering-kit" ]]
  [[ "$RESEARCH_DEFAULT_ENV_FILE" == "$XDG_CONFIG_HOME/ai-engineering-kit/research.env" ]]
  uname() { printf 'Darwin\n'; }
  research_platform_init
  [[ "$RESEARCH_LOCK_COMMAND" == lockf ]]
  [[ "$RESEARCH_DEFAULT_LOG_DIR" == "$HOME/Library/Logs/ai-engineering-kit" ]]
  [[ "$RESEARCH_DEFAULT_ENV_FILE" == "$HOME/Library/Application Support/ai-engineering-kit/research.env" ]]
  uname() { printf 'Unsupported\n'; }
  status=0
  research_platform_init > "$TEST_DIR/unsupported" 2>&1 || status=$?
  [[ "$status" == 2 ]]
  grep -Fq 'unsupported platform: Unsupported' "$TEST_DIR/unsupported"
)

# Verify both backend argument contracts and keep I/O failures distinct from busy.
(
  lockf() { printf '%s\n' "$*" > "$TEST_DIR/args"; return "$mock_status"; }
  flock() { printf '%s\n' "$*" > "$TEST_DIR/args"; return "$mock_status"; }
  for RESEARCH_SYSTEM in Darwin Linux; do
    for mock_status in 0 75 70; do
      status=0
      research_lock_fd 8 || status=$?
      [[ "$status" == "$mock_status" ]]
      if [[ "$RESEARCH_SYSTEM" == Darwin ]]; then
        [[ "$(cat "$TEST_DIR/args")" == '-s -t 0 8' ]]
      else
        [[ "$(cat "$TEST_DIR/args")" == '-x -n -E 75 8' ]]
      fi
    done
  done
)

# Use real OS locks, not stubs: contention, repeated attempts, close and exit.
research_platform_init
command -v "$RESEARCH_LOCK_COMMAND" >/dev/null
try_lock() ( exec 8<>"$TEST_DIR/guard"; research_lock_fd 8; )
exec 8<>"$TEST_DIR/guard"
research_lock_fd 8
for attempt in 1 2; do
  status=0
  ( exec 8>&-; try_lock ) || status=$?
  [[ "$status" == 75 ]]
done
exec 8>&-
try_lock
try_lock
[[ -f "$TEST_DIR/guard" ]]
status=0
( exec 8>&-; research_lock_fd 8 ) 2> "$TEST_DIR/invalid-fd" || status=$?
[[ "$status" != 0 && "$status" != 75 ]]

# An interrupted owner releases its descriptor; no child should retain the guard.
bash -c '
  set -Eeuo pipefail
  source "$1"
  research_platform_init
  trap "exec 8>&-" EXIT
  trap "exit 143" TERM
  exec 8<>"$2/guard"
  research_lock_fd 8
  touch "$2/ready"
  while :; do sleep 0.05 8>&-; done
' _ "$ROOT/scripts/research-platform.sh" "$TEST_DIR" &
holder=$!
for attempt in {1..100}; do
  [[ ! -f "$TEST_DIR/ready" ]] || break
  sleep 0.05
done
[[ -f "$TEST_DIR/ready" ]]
status=0
try_lock || status=$?
[[ "$status" == 75 ]]
kill -TERM "$holder"
status=0
wait "$holder" || status=$?
holder=''
[[ "$status" == 143 ]]
try_lock
printf 'platform paths, backend contracts, native lock contention, release, and signal cleanup: passed\n'
