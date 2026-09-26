#!/bin/bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LOG_DIR="${RESEARCH_LOG_DIR:-$HOME/Library/Logs/ai-engineering-kit}"
mkdir -p "$LOG_DIR"
PROGRESS_FILE="$LOG_DIR/research-loop-$$.json"
rm -f "$PROGRESS_FILE"
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
  rm -f "$PROGRESS_FILE"
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
completed=0
failures=0
failure_delay="$PAUSE"
while [[ "$(date +%s)" -lt "$DEADLINE" ]]; do
  status=0
  retry_delay="$PAUSE"
  rm -f "$PROGRESS_FILE"
  RESEARCH_CONTINUOUS=1 RESEARCH_DEADLINE="$DEADLINE" RESEARCH_PROGRESS_FILE="$PROGRESS_FILE" bash "$ROOT/scripts/research-next.sh" &
  child=$!
  wait "$child" || status=$?
  child=''
  case "$status" in
    0)
      completed=$((completed + 1))
      failures=0
      failure_delay="$PAUSE"
      printf '%s completed one research topic (loop total: %d)\n' "$(date '+%Y-%m-%dT%H:%M:%S%z')" "$completed"
      ;;
    3) printf 'continuous research already active; waiting\n' ;;
    4) printf 'provider cooldown active; waiting to retry\n' ;;
    2) printf 'research-loop: stopped after a non-retryable research failure; details: %s\n' "$LOG_DIR/research-error.log" >&2; exit "$status" ;;
    *)
      failures=$((failures + 1))
      retry_delay="$failure_delay"
      printf 'research-loop: research failed (exit code %d); retrying in %ss (%d consecutive failures); details: %s\n' \
        "$status" "$retry_delay" "$failures" "$LOG_DIR/research-error.log" >&2
      if ((failure_delay < 300)); then
        failure_delay=$((failure_delay * 2))
        ((failure_delay <= 300)) || failure_delay=300
      fi
      ;;
  esac
  remaining=$((DEADLINE - $(date +%s)))
  ((remaining > 0)) || break
  ((remaining >= retry_delay)) || retry_delay="$remaining"
  sleep "$retry_delay" &
  child=$!
  wait "$child"
  child=''
done
printf 'Continuous research time limit reached; completed %d topics\n' "$completed"
