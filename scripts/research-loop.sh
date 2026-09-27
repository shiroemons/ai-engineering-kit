#!/bin/bash
set -Eeuo pipefail

# Continuous research must not suspend for an interactive pager inherited from
# the user's shell; keep git and gh output in the loop's normal streams.
export PAGER=cat GIT_PAGER=cat GH_PAGER=cat

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
progress_monitor=''
PROGRESS_RUNNING=0
PROGRESS_TTY=0
if [[ -t 1 && "${TERM:-dumb}" != dumb ]]; then PROGRESS_TTY=1; fi

progress_fields() {
  jq -r '[(.percent // 0 | tostring), (.domain // .domain_name // "-"), (.topic // "テーマ選定中"), (.phase // "-")] | @tsv' "$1" 2>/dev/null
}

render_progress() {
  local percent="$1" domain="$2" topic="$3" phase="$4"
  local filled bar='' index stage
  ((percent >= 0 && percent <= 100)) || percent=0
  filled=$((percent / 10))
  for ((index = 0; index < 10; index++)); do
    if ((index < filled)); then bar+='█'; else bar+='░'; fi
  done
  if ((PROGRESS_TTY)); then
    if ((${#domain} > 8)); then domain="${domain:0:7}…"; fi
    if ((percent >= 100)); then
      if ((${#topic} > 10)); then topic="${topic:0:9}…"; fi
      printf '\r\033[2K[%s] %3d%% | %s | %s' "$bar" "$percent" "$domain" "$topic"
    else
      if ((percent <= 3)); then stage='テーマ確認'
      elif ((percent <= 15)); then stage='資料確認'
      elif ((percent <= 40)); then stage='要点整理'
      elif ((percent <= 55)); then stage='文書作成'
      elif ((percent <= 60)); then stage='eval作成'
      elif ((percent <= 92)); then stage='eval検証'
      elif ((percent <= 95)); then stage='成果確認'
      else stage='統合中'
      fi
      printf '\r\033[2K[%s] %3d%% | %s | %s' "$bar" "$percent" "$domain" "$stage"
    fi
  else
    if ((${#topic} > 10)); then topic="${topic:0:9}…"; fi
    if ((${#domain} > 10)); then domain="${domain:0:9}…"; fi
    printf '[%s] %3d%% | %s | %s | %s' "$bar" "$percent" "$domain" "$topic" "$phase"
  fi
}

monitor_progress() {
  local progress_file="$1" monitored_child="$2" last='' current=''
  local percent domain topic phase
  while :; do
    current=''
    if [[ -f "$progress_file" ]]; then
      current="$(progress_fields "$progress_file")" || current=''
    fi
    if ((PROGRESS_TTY)) && [[ -n "$current" && "$current" != "$last" ]]; then
      IFS=$'\t' read -r percent domain topic phase <<< "$current"
      render_progress "$percent" "$domain" "$topic" "$phase"
    fi
    last="$current"
    kill -0 "$monitored_child" 2>/dev/null || break
    sleep 0.5
  done
}

finish_progress() {
  local status="$1" current='' percent domain topic
  if [[ -f "$PROGRESS_FILE" ]]; then
    current="$(progress_fields "$PROGRESS_FILE")" || current=''
  fi
  if ((status == 0)); then
    if [[ -n "$current" ]]; then
      IFS=$'\t' read -r percent domain topic _ <<< "$current"
    else
      domain='-'
      topic='調査テーマ'
    fi
    render_progress 100 "$domain" "$topic" '調査完了'
    printf '\n'
  elif ((PROGRESS_TTY)) && [[ -n "$current" ]]; then
    printf '\r\033[2K'
  fi
}

cleanup() {
  if [[ -n "$child" ]] && kill -0 "$child" 2>/dev/null; then
    kill -TERM "$child"
    wait "$child" || true
  fi
  if [[ -n "$progress_monitor" ]]; then
    wait "$progress_monitor" || true
  fi
  if ((PROGRESS_TTY && PROGRESS_RUNNING)); then
    printf '\r\033[2K'
    PROGRESS_RUNNING=0
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
  PROGRESS_RUNNING=1
  monitor_progress "$PROGRESS_FILE" "$child" &
  progress_monitor=$!
  wait "$child" || status=$?
  child=''
  wait "$progress_monitor" || true
  progress_monitor=''
  finish_progress "$status"
  PROGRESS_RUNNING=0
  case "$status" in
    0)
      completed=$((completed + 1))
      failures=0
      failure_delay="$PAUSE"
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
