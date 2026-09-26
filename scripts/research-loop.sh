#!/bin/bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LOG_DIR="${RESEARCH_LOG_DIR:-$HOME/Library/Logs/ai-engineering-kit}"
mkdir -p "$LOG_DIR"
LOCK_DIR="$LOG_DIR/research-loop.lock"
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  printf 'research-loop: another continuous loop holds the lock\n' >&2
  exit 1
fi
child=''
cleanup() {
  if [[ -n "$child" ]] && kill -0 "$child" 2>/dev/null; then
    kill -TERM "$child"
    wait "$child" || true
  fi
  rm -f "$LOCK_DIR/pid"
  rmdir "$LOCK_DIR"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
printf '%s\n' "$$" > "$LOCK_DIR/pid"
SETTINGS="$(jq -er '.continuous | select(.hours >= 1 and .hours <= 24 and .hours == (.hours|floor)
  and .pause_seconds >= 1 and .pause_seconds <= 300 and .pause_seconds == (.pause_seconds|floor))
  | [.hours, .pause_seconds, .model] | @tsv' "$ROOT/config/research.json")"
IFS=$'\t' read -r HOURS PAUSE MODEL <<< "$SETTINGS"
DEADLINE=$(($(date +%s) + HOURS * 3600))
printf 'Continuous research: model=%s hours=%s; Ctrl+C to stop\n' "$MODEL" "$HOURS"
while [[ "$(date +%s)" -lt "$DEADLINE" ]]; do
  status=0
  RESEARCH_CONTINUOUS=1 RESEARCH_DEADLINE="$DEADLINE" bash "$ROOT/scripts/research-next.sh" &
  child=$!
  wait "$child" || status=$?
  child=''
  case "$status" in
    0) printf '%s completed one research topic\n' "$(date '+%Y-%m-%dT%H:%M:%S%z')" ;;
    3) printf 'continuous research already active; waiting\n' ;;
    4) printf 'provider cooldown active; continuous research stopped\n'; exit 0 ;;
    *) printf 'research-loop: stopped after research failure; inspect research-error.log\n' >&2; exit "$status" ;;
  esac
  remaining=$((DEADLINE - $(date +%s)))
  ((remaining > 0)) || break
  ((remaining >= PAUSE)) || PAUSE="$remaining"
  sleep "$PAUSE" &
  child=$!
  wait "$child"
  child=''
done
printf 'Continuous research time limit reached\n'
