#!/bin/bash
# Shared platform choices; callers own descriptors, traps, and lock lifetimes.
research_platform_init() {
  RESEARCH_SYSTEM="$(uname -s)"
  case "$RESEARCH_SYSTEM" in
    Darwin)
      RESEARCH_LOCK_COMMAND=lockf
      RESEARCH_DEFAULT_LOG_DIR="$HOME/Library/Logs/ai-engineering-kit"
      RESEARCH_DEFAULT_ENV_FILE="$HOME/Library/Application Support/ai-engineering-kit/research.env"
      ;;
    Linux)
      RESEARCH_LOCK_COMMAND=flock
      RESEARCH_DEFAULT_LOG_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/ai-engineering-kit"
      RESEARCH_DEFAULT_ENV_FILE="${XDG_CONFIG_HOME:-$HOME/.config}/ai-engineering-kit/research.env"
      ;;
    *)
      printf 'research: unsupported platform: %s (requires macOS or Linux)\n' "$RESEARCH_SYSTEM" >&2
      return 2
      ;;
  esac
}

# Both implementations lock the caller's open file description exclusively.
# Status 75 means contention; other errors must never be treated as contention.
# Closing the descriptor releases the guard. Do not unlink the guard file.
research_lock_fd() {
  case "$RESEARCH_SYSTEM" in
    Darwin) lockf -s -t 0 "$1" ;;
    Linux) flock -x -n -E 75 "$1" ;;
    *) return 2 ;;
  esac
}

research_require_macos() {
  if [[ "$(uname -s)" != Darwin ]]; then
    printf '%s: macOS only (launchd); use research or research-loop on Linux\n' "$1" >&2
    return 2
  fi
}
