#!/bin/bash
set -Eeuo pipefail

# Continuous research must not suspend for an interactive pager inherited from
# the user's shell; keep git and gh output in the loop's normal streams.
export PAGER=cat GIT_PAGER=cat GH_PAGER=cat

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
source "$ROOT/scripts/research-platform.sh"
research_platform_init
LOG_DIR="${RESEARCH_LOG_DIR:-$RESEARCH_DEFAULT_LOG_DIR}"
mkdir -p "$LOG_DIR"
LOG_DIR="$(cd "$LOG_DIR" && pwd -P)"
PROGRESS_FILE="$LOG_DIR/research-loop-$$.json"
rm -f "$PROGRESS_FILE"
LOCK_DIR="$LOG_DIR/research-loop.lock"
LOOP_PID_START="$(ps -p "$$" -o lstart= 2>/dev/null | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
LOOP_LOCK_HELD=0
LOOP_GUARD_LOCK_HELD=0
LOCK_ACQUIRE_IN_PROGRESS=0
CHILD_START_IN_PROGRESS=0
PENDING_SIGNAL_STATUS=''
child=''
child_gate=''
child_gate_open=0
child_gate_created=0
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
    if ((percent >= 100)); then stage='調査完了'
    elif ((percent <= 3)); then stage='テーマ確認'
    elif ((percent <= 15)); then stage='資料確認'
    elif ((percent <= 40)); then stage='要点整理'
    elif ((percent <= 55)); then stage='文書作成'
    elif ((percent <= 60)); then stage='eval作成'
    elif ((percent <= 92)); then stage='eval検証'
    elif ((percent <= 95)); then stage='成果確認'
    else stage='統合中'
    fi
    printf '\033[u\033[Jテーマ: %s\n[%s] %3d%% | %s | %s' \
      "$topic" "$bar" "$percent" "$domain" "$stage"
  else
    printf '[%s] %3d%% | %s | %s | %s' "$bar" "$percent" "$domain" "$topic" "$phase"
  fi
}

save_progress_cursor() {
  if ((PROGRESS_TTY)); then printf '\033[s'; fi
}

clear_progress_block() {
  if ((PROGRESS_TTY)); then printf '\033[u\033[J'; fi
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
  elif ((PROGRESS_TTY)); then
    clear_progress_block
  fi
}

cleanup() {
  if ((child_gate_open)); then
    exec 8>&-
    child_gate_open=0
  fi
  if [[ -n "$child_gate" ]]; then
    if ((child_gate_created)) && [[ -p "$child_gate" && ! -L "$child_gate" ]]; then
      rm -f "$child_gate"
    fi
    child_gate=''
    child_gate_created=0
  fi
  if [[ -n "$child" ]] && kill -0 "$child" 2>/dev/null; then
    kill -TERM "$child" 2>/dev/null || true
    wait "$child" || true
  fi
  child=''
  if [[ -n "$progress_monitor" ]]; then
    wait "$progress_monitor" || true
  fi
  if ((PROGRESS_TTY && PROGRESS_RUNNING)); then
    clear_progress_block
    PROGRESS_RUNNING=0
  fi
  if ((LOOP_LOCK_HELD)); then
    rm -f "$LOCK_DIR/pid" 2>/dev/null || true
    rm -f "$LOCK_DIR/pid_start" "$LOCK_DIR/child_pid" "$LOCK_DIR/child_pid_start" "$LOCK_DIR/monitor_pid" "$LOCK_DIR/monitor_pid_start" 2>/dev/null || true
    rmdir "$LOCK_DIR" 2>/dev/null || true
    LOOP_LOCK_HELD=0
  fi
  rm -f "$PROGRESS_FILE"
  if ((LOOP_GUARD_LOCK_HELD)); then
    exec 8>&-
    LOOP_GUARD_LOCK_HELD=0
  fi
}

loop_pid_is_alive() {
  local pid="$1" expected_start="${2:-}" actual_start
  [[ "$pid" =~ ^[0-9]+$ && "$pid" -gt 1 ]] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  if [[ -n "$expected_start" ]]; then
    actual_start="$(ps -p "$pid" -o lstart= 2>/dev/null | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    [[ -n "$actual_start" && "$actual_start" == "$expected_start" ]] || return 1
  fi
  return 0
}

reclaim_stale_loop_lock() {
  local name pid start found=0 loop_pid='' gate
  [[ -d "$LOCK_DIR" ]] || return 0
  for name in pid child_pid monitor_pid; do
    [[ -e "$LOCK_DIR/$name" ]] || continue
    found=1
    [[ -f "$LOCK_DIR/$name" ]] || return 1
    read -r pid < "$LOCK_DIR/$name" || return 1
    [[ "$pid" =~ ^[0-9]+$ && "$pid" -gt 1 ]] || return 1
    [[ "$name" != pid ]] || loop_pid="$pid"
    start=''
    [[ ! -f "$LOCK_DIR/${name}_start" ]] || read -r start < "$LOCK_DIR/${name}_start" || return 1
    if loop_pid_is_alive "$pid" "$start"; then return 1; fi
  done
  if ((!found)); then
    sleep 1
    [[ -d "$LOCK_DIR" && -z "$(find "$LOCK_DIR" -mindepth 1 -maxdepth 1 -print -quit)" ]] || return 1
    rmdir "$LOCK_DIR" 2>/dev/null
    return
  fi
  [[ -z "$(find "$LOCK_DIR" -mindepth 1 -maxdepth 1 \
    ! -name pid ! -name pid_start ! -name child_pid ! -name child_pid_start \
    ! -name monitor_pid ! -name monitor_pid_start -print -quit)" ]] || return 1
  rm -f "$LOCK_DIR/pid" "$LOCK_DIR/pid_start" "$LOCK_DIR/child_pid" "$LOCK_DIR/child_pid_start" \
    "$LOCK_DIR/monitor_pid" "$LOCK_DIR/monitor_pid_start" || return 1
  rmdir "$LOCK_DIR" 2>/dev/null || return 1
  [[ -z "$loop_pid" ]] || rm -f "$LOG_DIR/research-loop-$loop_pid.json"
  if [[ -n "$loop_pid" ]]; then
    for gate in "$LOG_DIR"/.research-loop-start-"$loop_pid"-*.fifo; do
      [[ -e "$gate" || -L "$gate" ]] || continue
      [[ -p "$gate" && ! -L "$gate" ]] || return 1
      rm -f "$gate" || return 1
    done
  fi
}

handle_signal() {
  local status="$1"
  if ((LOCK_ACQUIRE_IN_PROGRESS || CHILD_START_IN_PROGRESS)); then
    PENDING_SIGNAL_STATUS="$status"
  else
    exit "$status"
  fi
}

trap cleanup EXIT
trap 'handle_signal 130' INT
trap 'handle_signal 143' TERM
command -v "$RESEARCH_LOCK_COMMAND" >/dev/null 2>&1 || { printf 'research-loop: missing command: %s\n' "$RESEARCH_LOCK_COMMAND" >&2; exit 2; }
exec 8<>"$LOG_DIR/research-loop.guard" || { printf 'research-loop: could not open loop guard file\n' >&2; exit 2; }
LOCK_ACQUIRE_IN_PROGRESS=1
if research_lock_fd 8; then
  LOOP_GUARD_LOCK_HELD=1
else
  status=$?
  exec 8>&-
  if [[ "$status" == 75 ]]; then
    printf 'research-loop: another continuous loop is active\n' >&2
    exit 1
  fi
  printf 'research-loop: could not acquire loop guard (%s exit %s)\n' "$RESEARCH_LOCK_COMMAND" "$status" >&2
  exit 2
fi
LOCK_ACQUIRE_IN_PROGRESS=0
[[ -z "$PENDING_SIGNAL_STATUS" ]] || exit "$PENDING_SIGNAL_STATUS"
LOCK_ACQUIRE_IN_PROGRESS=1
if mkdir "$LOCK_DIR" 2>/dev/null; then
  LOOP_LOCK_HELD=1
  printf '%s\n' "$$" > "$LOCK_DIR/pid"
  printf '%s\n' "$LOOP_PID_START" > "$LOCK_DIR/pid_start"
  LOCK_ACQUIRE_IN_PROGRESS=0
  [[ -z "$PENDING_SIGNAL_STATUS" ]] || exit "$PENDING_SIGNAL_STATUS"
else
  LOCK_ACQUIRE_IN_PROGRESS=0
  [[ -z "$PENDING_SIGNAL_STATUS" ]] || exit "$PENDING_SIGNAL_STATUS"
  if reclaim_stale_loop_lock; then
    LOCK_ACQUIRE_IN_PROGRESS=1
    mkdir "$LOCK_DIR" 2>/dev/null || { printf 'research-loop: another loop acquired the reclaimed lock\n' >&2; exit 1; }
    LOOP_LOCK_HELD=1
    printf '%s\n' "$$" > "$LOCK_DIR/pid"
    printf '%s\n' "$LOOP_PID_START" > "$LOCK_DIR/pid_start"
    LOCK_ACQUIRE_IN_PROGRESS=0
    [[ -z "$PENDING_SIGNAL_STATUS" ]] || exit "$PENDING_SIGNAL_STATUS"
  else
    printf 'research-loop: another continuous loop holds the lock\n' >&2
    exit 1
  fi
fi
if ((LOOP_GUARD_LOCK_HELD)); then
  exec 8>&-
  LOOP_GUARD_LOCK_HELD=0
fi
SETTINGS="$(jq -er '.continuous | select(.hours >= 1 and .hours <= 24 and .hours == (.hours|floor)
  and .pause_seconds >= 1 and .pause_seconds <= 300 and .pause_seconds == (.pause_seconds|floor))
  | [.hours, .pause_seconds, .model] | @tsv' "$ROOT/config/research.json")"
IFS=$'\t' read -r HOURS PAUSE MODEL <<< "$SETTINGS"
DEADLINE=$(($(date +%s) + HOURS * 3600))
printf 'Continuous research: model=%s hours=%s; Ctrl+C to stop\n' "$MODEL" "$HOURS"
completed=0
failures=0
attempt=0
failure_delay="$PAUSE"
while [[ "$(date +%s)" -lt "$DEADLINE" ]]; do
  status=0
  retry_delay="$PAUSE"
  attempt=$((attempt + 1))
  rm -f "$PROGRESS_FILE"
  save_progress_cursor
  CHILD_START_IN_PROGRESS=1
  child_gate="$LOG_DIR/.research-loop-start-$$-$attempt.fifo"
  mkfifo "$child_gate"
  child_gate_created=1
  exec 8<>"$child_gate"
  child_gate_open=1
  (
    exec 8>&-
    IFS= read -r _ <&3 || exit 125
    exec 3<&-
    exec env RESEARCH_CONTINUOUS=1 RESEARCH_DEADLINE="$DEADLINE" RESEARCH_PROGRESS_FILE="$PROGRESS_FILE" \
      bash "$ROOT/scripts/research-next.sh"
  ) 3< "$child_gate" &
  child=$!
  printf '%s\n' "$child" > "$LOCK_DIR/child_pid"
  printf '%s\n' "$(ps -p "$child" -o lstart= 2>/dev/null | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')" > "$LOCK_DIR/child_pid_start"
  if [[ -n "$PENDING_SIGNAL_STATUS" ]]; then
    exec 8>&-
    child_gate_open=0
    if [[ -p "$child_gate" && ! -L "$child_gate" ]]; then rm -f "$child_gate"; fi
    child_gate=''
    child_gate_created=0
    CHILD_START_IN_PROGRESS=0
    exit "$PENDING_SIGNAL_STATUS"
  fi
  CHILD_START_IN_PROGRESS=0
  [[ -z "$PENDING_SIGNAL_STATUS" ]] || exit "$PENDING_SIGNAL_STATUS"
  printf 'start\n' >&8 || exit 1
  exec 8>&-
  child_gate_open=0
  if [[ -p "$child_gate" && ! -L "$child_gate" ]]; then rm -f "$child_gate"; fi
  child_gate=''
  child_gate_created=0
  if ((PROGRESS_TTY)); then
    PROGRESS_RUNNING=1
    monitor_progress "$PROGRESS_FILE" "$child" &
    progress_monitor=$!
    printf '%s\n' "$progress_monitor" > "$LOCK_DIR/monitor_pid"
    printf '%s\n' "$(ps -p "$progress_monitor" -o lstart= 2>/dev/null | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')" > "$LOCK_DIR/monitor_pid_start"
  fi
  wait "$child" || status=$?
  child=''
  rm -f "$LOCK_DIR/child_pid" "$LOCK_DIR/child_pid_start"
  if [[ -n "$progress_monitor" ]]; then
    wait "$progress_monitor" || true
    progress_monitor=''
    rm -f "$LOCK_DIR/monitor_pid" "$LOCK_DIR/monitor_pid_start"
  fi
  finish_progress "$status"
  PROGRESS_RUNNING=0
  case "$status" in
    0)
      completed=$((completed + 1))
      failures=0
      failure_delay="$PAUSE"
      ;;
    3) printf 'another research pipeline is active; waiting\n' ;;
    4) printf 'research-loop: provider cooldown active; stopped without counting a failure\n'; exit 0 ;;
    6) printf 'research-loop: provider rate limit detected; stopped without counting a failure\n'; exit 0 ;;
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
