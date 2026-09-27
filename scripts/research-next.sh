#!/bin/bash
set -Eeuo pipefail

# Research runs unattended and must not open a pager in the invoking terminal.
export PAGER=cat GIT_PAGER=cat GH_PAGER=cat

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
BASE_ROOT="$ROOT"
LOG_DIR="${RESEARCH_LOG_DIR:-$HOME/Library/Logs/ai-engineering-kit}"
CONFIG_FILE="${RESEARCH_ENV_FILE:-$HOME/Library/Application Support/ai-engineering-kit/research.env}"
mkdir -p "$LOG_DIR"
LOG_DIR="$(cd "$LOG_DIR" && pwd -P)"
LOG_FILE="$LOG_DIR/research.log"
ERROR_FILE="$LOG_DIR/research-error.log"
LOCK_DIR="$LOG_DIR/research.lock"
[[ "${RESEARCH_CONTINUOUS:-0}" != 1 ]] || LOCK_DIR="$LOG_DIR/research-continuous.lock"
PIPELINE_LOCK_DIR="$LOG_DIR/research-pipeline.lock"
RUN_LOCK_HELD=0
PIPELINE_LOCK_HELD=0
GUARD_LOCK_HELD=0
RUN_WORKTREE_CREATED=0
LOCK_ACQUIRE_IN_PROGRESS=0
PENDING_SIGNAL_STATUS=''
PIPELINE_WAIT_LOGGED=0
PIPELINE_LOCK_MISSING_OWNER_ATTEMPTS=0
PROGRESS_FILE="${RESEARCH_PROGRESS_FILE:-}"
REQUESTED_CONTINUOUS="${RESEARCH_CONTINUOUS:-0}"
RESEARCH_BRANCH=''
RUN_WORKTREE=''
OUTPUT_FILE=''
PR_BODY_FILE=''
BATCH_PID=''
BATCH_PID_START=''
BATCH_GATE_FIFO=''
BATCH_GATE_OPEN=0
RUN_ID=''
RUN_DIR=''
RUN_STATE_FILE=''
RUN_PHASE=''
RECOVERY_ACTION=''
RESUMING_COMPLETE_BATCH=0
BASE_CHECKOUT_WAS_DIRTY=0
PID_START="$(ps -p "$$" -o lstart= 2>/dev/null | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
timestamp() { date '+%Y-%m-%dT%H:%M:%S%z'; }
log() { printf '%s mode=%s pid=%s %s\n' "$(timestamp)" "${RESEARCH_CONTINUOUS:-0}" "$$" "$*" >> "$LOG_FILE"; }
fail() {
  local message="$1"
  local status="${2:-1}"
  log "failure (exit $status): $message"
  printf '%s exit=%s %s\n' "$(timestamp)" "$status" "$message" >> "$ERROR_FILE"
  printf 'research: %s (details: %s)\n' "$message" "$ERROR_FILE" >&2
  exit "$status"
}
provider_cooldown_active() {
  local cooldown_until
  [[ -f "$LOG_DIR/cooldown-until" ]] || return 1
  read -r cooldown_until < "$LOG_DIR/cooldown-until" || fail 'invalid cooldown state' 2
  [[ "$cooldown_until" =~ ^[0-9]+$ ]] || fail 'invalid cooldown state' 2
  (( $(date +%s) < cooldown_until ))
}
cleanup() {
  local exit_code="$1"
  local cleanup_status=0

  if ((BATCH_GATE_OPEN)); then
    exec 9>&-
    BATCH_GATE_OPEN=0
  fi
  if [[ -n "$BATCH_GATE_FIFO" ]]; then
    rm -f "$BATCH_GATE_FIFO"
    BATCH_GATE_FIFO=''
  fi

  if [[ -n "$BATCH_PID" ]] && kill -0 "$BATCH_PID" 2>/dev/null; then
    kill -TERM "$BATCH_PID" 2>/dev/null || true
    wait "$BATCH_PID" || true
  fi
  BATCH_PID=''
  BATCH_PID_START=''
  [[ -z "$OUTPUT_FILE" ]] || rm -f "$OUTPUT_FILE"
  [[ -z "$PR_BODY_FILE" ]] || rm -f "$PR_BODY_FILE"

  if [[ -n "$RUN_WORKTREE" ]]; then
    cd "$BASE_ROOT"
    if [[ -n "$RUN_DIR" ]]; then
      log "recovery: retained integration worktree: $RUN_WORKTREE branch=$RESEARCH_BRANCH"
    elif ((RUN_WORKTREE_CREATED)); then
      if git worktree remove "$RUN_WORKTREE" >/dev/null 2>&1; then
        log "removed clean integration worktree: $RUN_WORKTREE"
      else
        log "recovery: retained integration worktree: $RUN_WORKTREE branch=$RESEARCH_BRANCH"
      fi
    fi
  fi

  if [[ -n "$RUN_STATE_FILE" ]]; then
    if [[ "$(jq -r '.status' "$RUN_STATE_FILE" 2>/dev/null || true)" == complete ]]; then
      if ! cleanup_recovery_run "$RUN_STATE_FILE"; then
        log "failure: completed recovery cleanup needs attention: $RUN_DIR"
        cleanup_status=1
      else
        RUN_DIR=''
        RUN_STATE_FILE=''
      fi
    else
      local temporary="$RUN_DIR/.run.tmp.$$"
      if jq --arg updated_at "$(timestamp)" '.status = "pending" | .pid = 0 | .batch_pid = 0 | .batch_pid_start = "" | .updated_at = $updated_at' \
        "$RUN_STATE_FILE" > "$temporary" && chmod 600 "$temporary" && mv -f "$temporary" "$RUN_STATE_FILE"; then
        log "recovery: retained run state: $RUN_DIR phase=$RUN_PHASE"
      else
        log "failure: could not preserve recovery journal: $RUN_DIR"
        cleanup_status=1
      fi
    fi
  fi

  if ((RUN_LOCK_HELD)); then
    rm -f "$LOCK_DIR/pid" 2>/dev/null || true
    rm -f "$LOCK_DIR/pid_start" 2>/dev/null || true
    rm -f "$LOCK_DIR/batch_pid" 2>/dev/null || true
    rm -f "$LOCK_DIR/batch_pid_start" 2>/dev/null || true
    rmdir "$LOCK_DIR" 2>/dev/null || true
    RUN_LOCK_HELD=0
  fi
  if ((PIPELINE_LOCK_HELD)); then
    if ! rm -f "$PIPELINE_LOCK_DIR/pid"; then
      log "failure: could not remove shared pipeline lock owner: $PIPELINE_LOCK_DIR/pid"
      cleanup_status=1
    fi
    rm -f "$PIPELINE_LOCK_DIR/pid_start" 2>/dev/null || true
    rm -f "$PIPELINE_LOCK_DIR/batch_pid" 2>/dev/null || true
    rm -f "$PIPELINE_LOCK_DIR/batch_pid_start" 2>/dev/null || true
    if ! rmdir "$PIPELINE_LOCK_DIR"; then
      log "failure: could not remove shared pipeline lock: $PIPELINE_LOCK_DIR"
      cleanup_status=1
    else
      PIPELINE_LOCK_HELD=0
    fi
  fi
  if ((GUARD_LOCK_HELD)); then
    exec 8>&-
    GUARD_LOCK_HELD=0
  fi
  if ((exit_code == 0 && cleanup_status != 0)); then exit_code=1; fi
  return "$exit_code"
}
handle_signal() {
  local status="$1"
  if ((LOCK_ACQUIRE_IN_PROGRESS)); then
    PENDING_SIGNAL_STATUS="$status"
  else
    exit "$status"
  fi
}

acquire_guard_lock() {
  local status
  ((GUARD_LOCK_HELD)) && return 0
  command -v lockf >/dev/null 2>&1 || fail 'missing command: lockf' 2
  exec 8<>"$LOG_DIR/research-next.guard" || fail 'could not open pipeline guard file' 2
  LOCK_ACQUIRE_IN_PROGRESS=1
  if lockf -s -t 0 8; then
    GUARD_LOCK_HELD=1
    LOCK_ACQUIRE_IN_PROGRESS=0
    [[ -z "$PENDING_SIGNAL_STATUS" ]] || exit "$PENDING_SIGNAL_STATUS"
    return 0
  else
    status=$?
  fi
  exec 8>&-
  LOCK_ACQUIRE_IN_PROGRESS=0
  [[ -z "$PENDING_SIGNAL_STATUS" ]] || exit "$PENDING_SIGNAL_STATUS"
  [[ "$status" == 75 ]] && return 75
  fail "could not acquire pipeline guard (lockf exit $status)" 2
}

release_guard_lock() {
  if ((GUARD_LOCK_HELD)); then
    exec 8>&-
    GUARD_LOCK_HELD=0
  fi
}

pid_is_alive() {
  local pid="$1" expected_start="${2:-}" actual_start
  [[ "$pid" =~ ^[0-9]+$ && "$pid" -gt 1 ]] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  if [[ -n "$expected_start" ]]; then
    actual_start="$(ps -p "$pid" -o lstart= 2>/dev/null | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    [[ -n "$actual_start" && "$actual_start" == "$expected_start" ]] || return 1
  fi
  return 0
}

process_group_is_alive() {
  local pid="$1"
  [[ "$pid" =~ ^[0-9]+$ && "$pid" -gt 1 ]] || return 1
  kill -0 -- "-$pid" 2>/dev/null
}

reclaim_stale_lock() {
  local directory="$1" name pid start found=0
  [[ -d "$directory" ]] || return 0
  for name in pid batch_pid; do
    [[ -e "$directory/$name" ]] || continue
    found=1
    [[ -f "$directory/$name" ]] || return 1
    read -r pid < "$directory/$name" || return 1
    [[ "$pid" =~ ^[0-9]+$ && "$pid" -gt 1 ]] || return 1
    start=''
    [[ ! -f "$directory/${name}_start" ]] || read -r start < "$directory/${name}_start" || return 1
    if pid_is_alive "$pid" "$start"; then return 1; fi
  done
  if ((!found)); then
    sleep 1
    [[ -d "$directory" && -z "$(find "$directory" -mindepth 1 -maxdepth 1 -print -quit)" ]] || return 1
    rmdir "$directory" 2>/dev/null
    return
  fi
  [[ -z "$(find "$directory" -mindepth 1 -maxdepth 1 ! -name pid ! -name pid_start ! -name batch_pid ! -name batch_pid_start -print -quit)" ]] || return 1
  rm -f "$directory/pid" "$directory/pid_start" "$directory/batch_pid" "$directory/batch_pid_start" || return 1
  rmdir "$directory" 2>/dev/null
}

write_run_state() {
  local phase="$1" status="$2" temporary
  [[ -n "$RUN_STATE_FILE" ]] || return 0
  temporary="$RUN_DIR/.run.tmp.$$"
  jq --arg phase "$phase" --arg status "$status" --arg updated_at "$(timestamp)" \
    --argjson pid "$$" --arg pid_start "$PID_START" \
    --argjson batch_pid "${BATCH_PID:-0}" \
    --arg batch_pid_start "$BATCH_PID_START" \
    '.phase = $phase | .status = $status | .pid = $pid | .pid_start = $pid_start | .batch_pid = $batch_pid | .batch_pid_start = $batch_pid_start | .updated_at = $updated_at' \
    "$RUN_STATE_FILE" > "$temporary" || return 1
  chmod 600 "$temporary" || return 1
  mv -f "$temporary" "$RUN_STATE_FILE"
}

write_run_result() {
  local topic="$1" partial="$2" pr_url="${3:-}" temporary
  [[ -n "$RUN_STATE_FILE" ]] || return 0
  temporary="$RUN_DIR/.run.tmp.$$"
  jq --arg topic "$topic" --arg pr_url "$pr_url" --arg updated_at "$(timestamp)" \
    --argjson partial "$partial" \
    '.topic = $topic | .partial = $partial | .pr_url = (if $pr_url == "" then (.pr_url // "") else $pr_url end) | .updated_at = $updated_at' \
    "$RUN_STATE_FILE" > "$temporary" || return 1
  chmod 600 "$temporary" || return 1
  mv -f "$temporary" "$RUN_STATE_FILE"
}

recovery_has_live_owner() {
  local run_file="$1" run_dir pid start run_batch_pid run_batch_start owner_dir
  run_dir="$(dirname "$run_file")"
  pid="$(jq -r '.pid // 0' "$run_file")"
  start="$(jq -r '.pid_start // empty' "$run_file")"
  if pid_is_alive "$pid" "$start"; then return 0; fi
  pid="$(jq -r '.batch_pid // 0' "$run_file")"
  start="$(jq -r '.batch_pid_start // empty' "$run_file")"
  if pid_is_alive "$pid" "$start"; then return 0; fi
  for owner_dir in "$LOG_DIR/research.lock" "$LOG_DIR/research-continuous.lock" "$PIPELINE_LOCK_DIR"; do
    [[ -f "$owner_dir/batch_pid" ]] || continue
    read -r pid < "$owner_dir/batch_pid" || return 0
    start=''
    [[ ! -f "$owner_dir/batch_pid_start" ]] || read -r start < "$owner_dir/batch_pid_start" || return 0
    if pid_is_alive "$pid" "$start"; then return 0; fi
  done
  [[ -f "$run_dir/batch.json" ]] || return 1
  pid="$(jq -r '.pid // 0' "$run_dir/batch.json")"
  start="$(jq -r '.pid_start // empty' "$run_dir/batch.json")"
  run_batch_pid="$(jq -r '.batch_pid // 0' "$run_file")"
  run_batch_start="$(jq -r '.batch_pid_start // empty' "$run_file")"
  if [[ "$pid" == "$run_batch_pid" && -n "$run_batch_start" ]]; then start="$run_batch_start"; fi
  if pid_is_alive "$pid" "$start"; then return 0; fi
  while IFS=$'\t' read -r pid start; do
    if pid_is_alive "$pid" "$start"; then return 0; fi
    if process_group_is_alive "$pid"; then return 0; fi
  done < <(jq -r '.workers[]? | [.opencode_pid // 0, .opencode_pid_start // ""] | @tsv' "$run_dir/batch.json")
  return 1
}

remove_registered_worktree() {
  local path="$1" expected_parent="$2" top name
  [[ "$(dirname "$path")" == "$expected_parent" ]] || return 1
  [[ -d "$path" && ! -L "$path" ]] || return 1
  name="$(basename "$path")"
  case "$name" in run-*|worker-[1-9]*|integration-*) ;; *) return 1 ;; esac
  top="$(git -C "$path" rev-parse --show-toplevel 2>/dev/null)" || return 1
  [[ "$(cd "$top" && pwd -P)" == "$path" ]] || return 1
  git -C "$BASE_ROOT" worktree list --porcelain | grep -Fqx "worktree $path" || return 1
  git -C "$BASE_ROOT" worktree remove --force "$path" >/dev/null
}

cleanup_orphaned_recovery_files() {
  local run_dir="$1" run_id="$2" temporary
  for temporary in \
    "$run_dir"/.run.tmp.* "$run_dir"/.batch-*.tmp "$run_dir"/.batch-start-*.fifo \
    "$LOG_DIR"/topics/.topic-"$run_id"-*.tmp \
    "$LOG_DIR"/domains/.owner-"$run_id"-*.tmp \
    "$LOG_DIR"/domains/*.lock/.owner-"$run_id"-*.tmp \
    "$LOG_DIR"/.cooldown-"$run_id"-*.tmp; do
    [[ -e "$temporary" || -L "$temporary" ]] || continue
    [[ ! -L "$temporary" ]] || return 1
    case "$temporary" in
      *.fifo) [[ -p "$temporary" ]] || return 1 ;;
      *) [[ -f "$temporary" ]] || return 1 ;;
    esac
    rm -f "$temporary" || return 1
  done
}

cleanup_recovery_run() {
  local run_file="$1" run_dir id root branch batch path lease owner owner_pid owner_start temporary owns_lease base storage root_branch_sha='' cleanup_branch_sha='' branch_ref_sha=''
  run_dir="$(cd "$(dirname "$run_file")" && pwd -P)"
  [[ -f "$run_file" && ! -L "$run_file" ]] || return 1
  id="$(jq -er '.id' "$run_file")" || return 1
  [[ "$(basename "$run_dir")" == "$id" && "$run_dir" == "$LOG_DIR/research-runs/"* ]] || return 1
  root="$(jq -r '.root // empty' "$run_file")"
  branch="$(jq -r '.branch // empty' "$run_file")"
  cleanup_branch_sha="$(jq -r '.cleanup_branch_sha // empty' "$run_file")"
  base="$(jq -er '.base' "$run_file")" || return 1
  [[ "$id" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}-[0-9]{6}-[0-9]+$ ]] || return 1
  storage="$BASE_ROOT/.workbench/repositories/research"
  [[ "$root" == "$storage/run-$id" && "$branch" == "research/$id" ]] || return 1
  if [[ -n "$root" && -e "$root" ]]; then
    [[ "$(dirname "$root")" == "$storage" && "$(basename "$root")" == run-* && ! -L "$root" ]] || return 1
    git -C "$BASE_ROOT" worktree list --porcelain | grep -Fqx "worktree $root" || return 1
    [[ "$(git -C "$root" branch --show-current 2>/dev/null)" == "$branch" ]] || return 1
    root_branch_sha="$(git -C "$BASE_ROOT" rev-parse "refs/heads/$branch" 2>/dev/null)" || return 1
    [[ -z "$cleanup_branch_sha" || "$cleanup_branch_sha" == "$root_branch_sha" ]] || return 1
    if [[ -z "$cleanup_branch_sha" ]]; then
      temporary="$run_dir/.run.tmp.$$"
      jq --arg sha "$root_branch_sha" '.cleanup_branch_sha = $sha' "$run_file" > "$temporary" || return 1
      chmod 600 "$temporary" && mv -f "$temporary" "$run_file" || return 1
      cleanup_branch_sha="$root_branch_sha"
    fi
  fi
  if git -C "$BASE_ROOT" show-ref --verify --quiet "refs/heads/$branch"; then
    [[ "$cleanup_branch_sha" =~ ^[0-9a-f]{40,64}$ ]] || return 1
    branch_ref_sha="$(git -C "$BASE_ROOT" rev-parse "refs/heads/$branch")" || return 1
    [[ "$branch_ref_sha" == "$cleanup_branch_sha" ]] || return 1
  elif [[ -n "$root" && -e "$root" ]]; then
    return 1
  fi
  batch=''
  if [[ -f "$run_dir/batch.json" ]]; then
    [[ ! -L "$run_dir/batch.json" ]] || return 1
    batch="$(jq -r '.batch // empty' "$run_dir/batch.json")"
    [[ "$batch" == "$storage/batch-$id" ]] || return 1
    jq -e --arg id "$id" --arg root "$root" --arg base "$base" --arg batch "$batch" \
      '.run_id == $id and .root == $root and .base == $base and .batch == $batch and
       (.workers | type == "array" and length > 0) and
       ([.workers | to_entries[] | .value.path == ($batch + "/worker-" + ((.key + 1) | tostring))] | all)' \
      "$run_dir/batch.json" >/dev/null || return 1
  fi
  for temporary in "$run_dir"/.batch-start-*.fifo; do
    [[ -e "$temporary" || -L "$temporary" ]] || continue
    [[ -p "$temporary" && ! -L "$temporary" ]] || return 1
    rm -f "$temporary" || return 1
  done
  if [[ -n "$batch" ]]; then
    [[ "$(dirname "$batch")" == "$storage" && ! -L "$batch" ]] || return 1
    if [[ -e "$batch" ]]; then
      [[ -d "$batch" ]] || return 1
      while IFS= read -r path; do
        [[ -n "$path" && -e "$path" ]] || continue
        remove_registered_worktree "$path" "$batch" || return 1
      done < <(jq -r '.workers[]?.path // empty' "$run_dir/batch.json")
      while IFS= read -r path; do
        [[ -n "$path" ]] || continue
        if [[ -d "$path" && ! -L "$path" ]]; then
          remove_registered_worktree "$path" "$batch" || return 1
        fi
      done < <(find "$batch" -mindepth 1 -maxdepth 1 -type d -name 'integration-*' -print)
      rmdir "$batch" 2>/dev/null || return 1
    fi
  fi
  if [[ -n "$root_branch_sha" ]]; then
    remove_registered_worktree "$root" "$storage" || return 1
  fi
  if git -C "$BASE_ROOT" show-ref --verify --quiet "refs/heads/$branch"; then
    git -C "$BASE_ROOT" update-ref -d "refs/heads/$branch" "$cleanup_branch_sha" || return 1
  fi
  for lease in "$LOG_DIR"/domains/*.lock; do
    [[ -e "$lease" || -L "$lease" ]] || continue
    [[ ! -L "$lease" ]] || return 1
    if [[ -d "$lease" ]]; then
      owner=''
      if [[ -e "$lease/owner.json" || -L "$lease/owner.json" ]]; then
        [[ -f "$lease/owner.json" && ! -L "$lease/owner.json" ]] || return 1
        owner="$(jq -er '.run_id | select(type == "string" and length > 0)' "$lease/owner.json" 2>/dev/null)" || return 1
      fi
      owns_lease=0
      if [[ "$owner" == "$id" ]]; then
        owner_pid="$(jq -er '.pid | select(type == "number" and . > 1)' "$lease/owner.json" 2>/dev/null)" || return 1
        owner_start="$(jq -r '.pid_start // empty' "$lease/owner.json" 2>/dev/null)" || return 1
        if pid_is_alive "$owner_pid" "$owner_start"; then return 1; fi
        rm -f "$lease/owner.json" || return 1
        owns_lease=1
      fi
      for temporary in "$lease"/.owner-"$id"-*.tmp; do
        [[ -f "$temporary" && ! -L "$temporary" ]] || continue
        rm -f "$temporary" || return 1
        owns_lease=1
      done
      if [[ -z "$owner" ]]; then
        rmdir "$lease" 2>/dev/null || return 1
      elif ((owns_lease)) && [[ "$owner" == "$id" ]]; then
        rmdir "$lease" 2>/dev/null || return 1
      fi
    elif [[ -f "$lease" ]]; then
      owner="$(jq -er '.run_id | select(type == "string" and length > 0)' "$lease" 2>/dev/null)" || return 1
      if [[ "$owner" == "$id" ]]; then
        owner_pid="$(jq -er '.pid | select(type == "number" and . > 1)' "$lease" 2>/dev/null)" || return 1
        owner_start="$(jq -r '.pid_start // empty' "$lease" 2>/dev/null)" || return 1
        if pid_is_alive "$owner_pid" "$owner_start"; then return 1; fi
        rm -f "$lease" || return 1
      fi
    else
      return 1
    fi
  done
  for temporary in "$LOG_DIR"/domains/.owner-"$id"-*.tmp; do
    [[ -e "$temporary" || -L "$temporary" ]] || continue
    [[ -f "$temporary" && ! -L "$temporary" ]] || return 1
    rm -f "$temporary" || return 1
  done
  for lease in "$LOG_DIR"/topics/*.json; do
    [[ -f "$lease" ]] || continue
    [[ ! -L "$lease" ]] || return 1
    owner="$(jq -r '.run_id // empty' "$lease" 2>/dev/null || true)"
    [[ "$owner" != "$id" ]] || rm -f "$lease" || return 1
  done
  for temporary in "$LOG_DIR"/topics/.topic-"$id"-*.tmp "$LOG_DIR"/.cooldown-"$id"-*.tmp; do
    [[ -f "$temporary" && ! -L "$temporary" ]] || continue
    rm -f "$temporary" || return 1
  done
  rm -f "$run_dir/run.json" "$run_dir/batch.json" "$run_dir/coordinator.log" "$run_dir/research-pr.md" \
    "$run_dir"/.run.tmp.* "$run_dir"/.batch-*.tmp || return 1
  rmdir "$run_dir" 2>/dev/null
}

recover_pending_run() {
  local candidate status phase run_file choice can_resume=1 run_dir run_id batch_complete=0
  local pending=()
  shopt -s nullglob
  [[ ! -L "$LOG_DIR/research-runs" ]] || { printf 'research: recovery root is a symlink; no run was changed\n' >&2; exit 2; }
  for candidate in "$LOG_DIR"/research-runs/*/run.json; do
    [[ -f "$candidate" && ! -L "$candidate" && ! -L "$(dirname "$candidate")" ]] || {
      printf 'research: unexpected recovery record path: %s\n' "$candidate" >&2
      exit 2
    }
    status="$(jq -r '.status // "invalid"' "$candidate" 2>/dev/null || printf invalid)"
    if [[ "$status" == complete ]]; then
      if ! cleanup_recovery_run "$candidate"; then
        printf 'research: completed run cleanup needs attention: %s\n' "$candidate" >&2
        exit 2
      fi
      continue
    fi
    pending+=("$candidate")
  done
  shopt -u nullglob
  ((${#pending[@]} == 0)) && return 0
  if ((${#pending[@]} != 1)); then
    printf 'research: %d incomplete runs need individual recovery; no run was changed\n' "${#pending[@]}" >&2
    exit 2
  fi
  run_file="${pending[0]}"
  run_dir="$(dirname "$run_file")"
  [[ -f "$run_file" && ! -L "$run_file" && -d "$run_dir" && ! -L "$run_dir" ]] || {
    printf 'research: recovery record is not a regular run-owned path\n' >&2
    exit 2
  }
  run_id="$(jq -er '.id' "$run_file")" || { printf 'research: invalid recovery record: %s\n' "$run_file" >&2; exit 2; }
  [[ "$(basename "$run_dir")" == "$run_id" ]] || { printf 'research: recovery record path does not match its run ID\n' >&2; exit 2; }
  [[ ! -L "$run_dir/batch.json" ]] || { printf 'research: recovery journal is a symlink; run retained\n' >&2; exit 2; }
  if recovery_has_live_owner "$run_file"; then
    printf 'research: run %s still has a live owner; leaving it untouched\n' "$run_id" >&2
    exit 3
  fi
  [[ -f "$run_file" && ! -L "$run_file" && -d "$run_dir" && ! -L "$run_dir" ]] || {
    printf 'research: recovery record or directory is not a regular run-owned path\n' >&2
    exit 2
  }
  [[ "$(cd "$run_dir" && pwd -P)" == "$LOG_DIR/research-runs/$run_id" ]] || {
    printf 'research: recovery directory resolves outside its recorded run path\n' >&2
    exit 2
  }
  cleanup_orphaned_recovery_files "$run_dir" "$run_id" || {
    printf 'research: unexpected temporary recovery file; run retained\n' >&2
    exit 2
  }
  jq '.status = "pending" | .pid = 0 | .batch_pid = 0 | .batch_pid_start = ""' "$run_file" > "$run_dir/.run.tmp.$$" || exit 2
  chmod 600 "$run_dir/.run.tmp.$$" && mv -f "$run_dir/.run.tmp.$$" "$run_file" || exit 2
  phase="$(jq -r '.phase // "unknown"' "$run_file")"
  if [[ -f "$run_dir/batch.json" ]] && [[ "$(jq -r '.complete // false' "$run_dir/batch.json")" == true ]]; then
    batch_complete=1
  fi
  case "$phase" in
    committed|pushed|pr-created|merge-requested) batch_complete=1 ;;
  esac
  # Avoid offering a provider-dependent resume while cooldown guarantees it will fail.
  if provider_cooldown_active && ((batch_complete == 0)); then
    printf 'research: provider cooldown active; recovery prompt deferred; run retained\n' >&2
    exit 4
  fi
  printf 'Incomplete research run: %s (phase: %s)\n' "$run_id" "$phase" >&2
  if [[ ! -t 0 || ! -t 1 ]]; then
    printf 'research: recovery decision required; run is retained; start from a TTY to resume or discard\n' >&2
    exit 2
  fi
  if [[ ! -f "$run_dir/batch.json" && "$phase" == batch-running && "$(jq -r '.version // 0' "$run_file")" -lt 2 ]]; then can_resume=0; fi
  if [[ ! "$phase" =~ ^(creating-worktree|building|batch-running|batch-complete|validated|committed|pushed|pr-created|merge-requested)$ ]]; then can_resume=0; fi
  if [[ "$(jq -r '.continuous // false' "$run_file")" == true && "$REQUESTED_CONTINUOUS" != 1 ]] || \
     [[ "$(jq -r '.continuous // false' "$run_file")" != true && "$REQUESTED_CONTINUOUS" == 1 ]]; then
    can_resume=0
  fi
  if ((can_resume)); then
    printf 'Choose: [r]esume from the saved phase, [d]iscard local run state, [q]uit and keep it: ' >&2
  else
    printf 'This phase cannot be resumed automatically. Choose: [d]iscard local run state or [q]uit and keep it: ' >&2
  fi
  IFS= read -r choice || choice=q
  case "$choice" in
    r|R)
      ((can_resume)) || { printf 'research: phase %s cannot be resumed automatically; run retained\n' "$phase" >&2; exit 2; }
      for candidate in "$run_dir"/.batch-start-*.fifo; do
        [[ -e "$candidate" || -L "$candidate" ]] || continue
        [[ -p "$candidate" && ! -L "$candidate" ]] || { printf 'research: unexpected batch start-gate file; run retained\n' >&2; exit 2; }
        rm -f "$candidate" || { printf 'research: could not clean an abandoned batch start gate\n' >&2; exit 2; }
      done
      RECOVERY_ACTION=resume
      RUN_DIR="$run_dir"
      RUN_STATE_FILE="$run_file"
      RUN_ID="$run_id"
      RUN_PHASE="$phase"
      RESEARCH_BRANCH="$(jq -er '.branch' "$run_file")" || exit 2
      RUN_WORKTREE="$(jq -er '.root' "$run_file")" || exit 2
      RESEARCH_CONTINUOUS="$(jq -r '.continuous // false' "$run_file")"
      [[ "$RESEARCH_CONTINUOUS" == true ]] && RESEARCH_CONTINUOUS=1 || RESEARCH_CONTINUOUS=0
      RESUMING_COMPLETE_BATCH="$batch_complete"
      write_run_state "$RUN_PHASE" running || exit 2
      ;;
    d|D)
      cleanup_recovery_run "$run_file" || { printf 'research: could not safely remove run-owned local worktrees; state retained\n' >&2; exit 2; }
      printf 'research: discarded local recovery state for run %s; remote branches and pull requests are unchanged\n' "$run_id" >&2
      ;;
    *)
      printf 'research: recovery run %s retained\n' "$run_id" >&2
      exit 2
      ;;
  esac
}

trap 'cleanup "$?"' EXIT
trap 'handle_signal 130' INT
trap 'handle_signal 143' TERM
while :; do
  if acquire_guard_lock; then break; else status=$?; fi
  [[ "$status" == 75 ]] || exit "$status"
  sleep 1
done
LOCK_ACQUIRE_IN_PROGRESS=1
if mkdir "$LOCK_DIR" 2>/dev/null; then
  RUN_LOCK_HELD=1
  printf '%s\n' "$$" > "$LOCK_DIR/pid"
  printf '%s\n' "$PID_START" > "$LOCK_DIR/pid_start"
  LOCK_ACQUIRE_IN_PROGRESS=0
  [[ -z "$PENDING_SIGNAL_STATUS" ]] || exit "$PENDING_SIGNAL_STATUS"
else
  LOCK_ACQUIRE_IN_PROGRESS=0
  [[ -z "$PENDING_SIGNAL_STATUS" ]] || exit "$PENDING_SIGNAL_STATUS"
  if reclaim_stale_lock "$LOCK_DIR"; then
    LOCK_ACQUIRE_IN_PROGRESS=1
    mkdir "$LOCK_DIR" 2>/dev/null || { log 'skipped: another run acquired the reclaimed lock'; exit 3; }
    RUN_LOCK_HELD=1
    printf '%s\n' "$$" > "$LOCK_DIR/pid"
    printf '%s\n' "$PID_START" > "$LOCK_DIR/pid_start"
    LOCK_ACQUIRE_IN_PROGRESS=0
    [[ -z "$PENDING_SIGNAL_STATUS" ]] || exit "$PENDING_SIGNAL_STATUS"
  else
    log 'skipped: another run holds the lock (stale owner could not be proven dead)'
    exit 3
  fi
fi
while :; do
  if acquire_guard_lock; then
    :
  else
    status=$?
    [[ "$status" == 75 ]] || exit "$status"
    sleep 1
    continue
  fi
  LOCK_ACQUIRE_IN_PROGRESS=1
  if mkdir "$PIPELINE_LOCK_DIR" 2>/dev/null; then
    PIPELINE_LOCK_HELD=1
    printf '%s\n' "$$" > "$PIPELINE_LOCK_DIR/pid"
    printf '%s\n' "$PID_START" > "$PIPELINE_LOCK_DIR/pid_start"
    LOCK_ACQUIRE_IN_PROGRESS=0
    [[ -z "$PENDING_SIGNAL_STATUS" ]] || exit "$PENDING_SIGNAL_STATUS"
    release_guard_lock
    break
  fi
  LOCK_ACQUIRE_IN_PROGRESS=0
  [[ -z "$PENDING_SIGNAL_STATUS" ]] || exit "$PENDING_SIGNAL_STATUS"
  if reclaim_stale_lock "$PIPELINE_LOCK_DIR"; then
    continue
  fi
  if [[ "${RESEARCH_CONTINUOUS:-0}" == 1 ]]; then
    if [[ -d "$LOG_DIR/research.lock" ]]; then
      log 'skipped: scheduled research is waiting for the shared pipeline lock'
      printf 'research: scheduled research is waiting; continuous loop will retry\n' >&2
    else
      log 'skipped: another research pipeline holds the shared lock'
      printf 'research: another research pipeline holds the shared lock; retry later\n' >&2
    fi
    exit 3
  fi

  if ((PIPELINE_WAIT_LOGGED == 0)); then
    log 'waiting: another research pipeline holds the shared lock'
    printf 'research: waiting for the active pipeline to finish\n' >&2
    PIPELINE_WAIT_LOGGED=1
  fi

  PIPELINE_PID=''
  PIPELINE_PID_START=''
  if [[ -f "$PIPELINE_LOCK_DIR/pid" ]]; then
    read -r PIPELINE_PID < "$PIPELINE_LOCK_DIR/pid" || PIPELINE_PID=''
  fi
  if [[ "$PIPELINE_PID" =~ ^[0-9]+$ ]]; then
    PIPELINE_LOCK_MISSING_OWNER_ATTEMPTS=0
    if [[ -f "$PIPELINE_LOCK_DIR/pid_start" ]]; then
      read -r PIPELINE_PID_START < "$PIPELINE_LOCK_DIR/pid_start" || PIPELINE_PID_START=''
    fi
    if [[ -z "$PIPELINE_PID_START" ]]; then
      log 'failure: shared pipeline lock has no process start identity'
      printf 'research: shared pipeline lock owner cannot be identified; leaving it untouched: %s\n' "$PIPELINE_LOCK_DIR" >&2
      exit 3
    fi
    if ! pid_is_alive "$PIPELINE_PID" "$PIPELINE_PID_START"; then
      if reclaim_stale_lock "$PIPELINE_LOCK_DIR"; then continue; fi
      log "failure: shared pipeline lock has no live owner: pid=$PIPELINE_PID start=$PIPELINE_PID_START"
      printf 'research: stale shared pipeline lock could not be safely reclaimed: %s\n' "$PIPELINE_LOCK_DIR" >&2
      exit 3
    fi
  else
    PIPELINE_LOCK_MISSING_OWNER_ATTEMPTS=$((PIPELINE_LOCK_MISSING_OWNER_ATTEMPTS + 1))
    if ((PIPELINE_LOCK_MISSING_OWNER_ATTEMPTS >= 3)); then
      log 'failure: shared pipeline lock has no valid owner'
      printf 'research: shared pipeline lock has no valid owner; confirm no run is active, then remove %s\n' "$PIPELINE_LOCK_DIR" >&2
      exit 3
    fi
  fi
  release_guard_lock
  sleep 1
done

if [[ "${RESEARCH_PREFLIGHT:-0}" != 1 ]]; then
  recover_pending_run
fi

cd "$ROOT"
log 'start'
[[ "$(git branch --show-current)" == main ]] || fail 'branch is not main' 2
WORKTREE_STATUS="$(git status --porcelain --untracked-files=all)" || fail 'could not inspect working tree' 2
if [[ -n "$WORKTREE_STATUS" ]]; then
  log 'working tree changes (first 20 entries):'
  printf '%s\n' "$WORKTREE_STATUS" | sed -n '1,20p' >> "$LOG_FILE"
  if [[ "$RECOVERY_ACTION" == resume && "$RESUMING_COMPLETE_BATCH" == 1 ]]; then
    BASE_CHECKOUT_WAS_DIRTY=1
    log 'recovery: preserving original checkout changes; using the saved completed batch'
  else
    fail 'working tree is dirty; see changed paths in research.log' 2
  fi
fi
if provider_cooldown_active; then
  if [[ "$RECOVERY_ACTION" == resume && "$RESUMING_COMPLETE_BATCH" == 1 ]]; then
    # A completed batch only needs local validation and publishing steps.
    log 'provider cooldown bypassed for saved completed batch'
  else
    log 'skipped: provider cooldown is active'
    exit 4
  fi
fi
NEEDS_RESEARCH_BATCH=1
if [[ "$RECOVERY_ACTION" == resume && "$RESUMING_COMPLETE_BATCH" == 1 ]]; then
  NEEDS_RESEARCH_BATCH=0
fi
MODEL=''
AVAILABLE=''
FREE_MODEL_IDS=''

for command in git jq just; do
  command -v "$command" >/dev/null || fail "missing command: $command" 2
done
if ((NEEDS_RESEARCH_BATCH)); then
  for command in opencode curl mise; do
    command -v "$command" >/dev/null || fail "missing command: $command" 2
  done
  [[ -f "$CONFIG_FILE" ]] || fail 'research.env is missing' 2

  # This local file contains one non-secret assignment, never shell-evaluate it.
  MODEL="$(sed -nE 's/^OPENCODE_RESEARCH_MODEL=([A-Za-z0-9._\/-]+)$/\1/p' "$CONFIG_FILE")"
  [[ -n "$MODEL" && "$(wc -l < "$CONFIG_FILE" | tr -d ' ')" == 1 ]] || fail 'invalid research.env' 2
  [[ "$MODEL" =~ ^opencode/[A-Za-z0-9._/-]+$ ]] || fail 'configured model must use the opencode provider' 2
fi
if [[ "${RESEARCH_DRY_RUN:-0}" != 1 ]]; then
  command -v gh >/dev/null || fail 'missing command: gh' 2
  GH_PROMPT_DISABLED=1 gh auth status >/dev/null 2>&1 || fail 'GitHub CLI is not authenticated' 2
fi

# Continuous execution starts with its configured model, then fills remaining slots.
if ((NEEDS_RESEARCH_BATCH)); then
  for attempt in 1 2 3; do
    if AVAILABLE="$(opencode models --print-logs --log-level debug 2>/dev/null)"; then
      if [[ -n "$AVAILABLE" ]]; then
        log "OpenCode model listing: passed (attempt $attempt/3)"
        break
      fi
      log 'OpenCode model listing returned no models; catalog may still be initializing'
    else
      log 'OpenCode model listing command failed'
    fi
    AVAILABLE=''
    if [[ "$attempt" -lt 3 ]]; then
      delay=$((attempt * 5))
      log "OpenCode model listing failed; retrying in ${delay}s (attempt $attempt/3)"
      sleep "$delay"
    fi
  done
  [[ -n "$AVAILABLE" ]] || fail 'OpenCode model listing failed after 3 attempts'

  PRICE_JSON=''
  for attempt in 1 2 3; do
    if PRICE_JSON="$(curl -fsSL --max-time 20 https://models.dev/api.json)" \
      && printf '%s\n' "$PRICE_JSON" | jq -e 'type == "object" and (.opencode.models | type == "object")' >/dev/null 2>&1; then
      log "current model pricing: passed (attempt $attempt/3)"
      break
    fi
    PRICE_JSON=''
    if [[ "$attempt" -lt 3 ]]; then
      delay=$((attempt * 5))
      log "current model pricing unavailable; retrying in ${delay}s (attempt $attempt/3)"
      sleep "$delay"
    fi
  done
  [[ -n "$PRICE_JSON" ]] || fail 'current model pricing unavailable after 3 attempts'

  FREE_MODEL_IDS="$(printf '%s\n' "$PRICE_JSON" | jq -r \
    '.opencode.models | to_entries[] | select(.value.tool_call == true and .value.cost.input == 0 and .value.cost.output == 0 and ((.value.cost.cache_read // 0) == 0) and ((.value.cost.cache_write // 0) == 0)) | .key')" \
    || fail 'current model pricing could not be parsed'
fi
is_verified_free_model() {
  local candidate="$1" model_id
  [[ "$candidate" =~ ^opencode/[A-Za-z0-9._/-]+$ ]] || return 1
  printf '%s\n' "$AVAILABLE" | grep -Fxq "$candidate" || return 1
  model_id="${candidate#opencode/}"
  printf '%s\n' "$FREE_MODEL_IDS" | grep -Fxq "$model_id"
}

MODEL_CANDIDATES=()
MODEL_CANDIDATE_FOUND=0
find_model_candidates() {
  local candidate preferred topic_limit
  MODEL_CANDIDATES=()
  MODEL_CANDIDATE_FOUND=0
  if [[ "${RESEARCH_CONTINUOUS:-0}" == 1 ]]; then
    topic_limit="$(jq -er '.continuous.topics_per_run | select(. >= 1 and . <= 8 and . == (. | floor))' "$BASE_ROOT/config/research.json")" \
      || fail 'invalid continuous.topics_per_run' 2
    preferred="$(jq -er '.continuous.model' "$BASE_ROOT/config/research.json")" || fail 'invalid continuous model' 2
    if ! is_verified_free_model "$preferred"; then
      log "continuous model is unavailable or not currently free: $preferred"
      return 0
    fi
    MODEL_CANDIDATES+=("$preferred")
    if (( ${#MODEL_CANDIDATES[@]} >= topic_limit )); then
      MODEL_CANDIDATE_FOUND=1
      return 0
    fi
    preferred="$(jq -er '.parallel.preferred_models[]' "$BASE_ROOT/config/research.json")" || fail 'invalid preferred models' 2
  else
    topic_limit="$(jq -er '.topics_per_run | select(. >= 1 and . <= 8 and . == (. | floor))' "$BASE_ROOT/config/research.json")" \
      || fail 'invalid topics_per_run' 2
    preferred="$(jq -er '.parallel.preferred_models[]' "$BASE_ROOT/config/research.json")" || fail 'invalid preferred models' 2
  fi
  if ! is_verified_free_model "$MODEL"; then
    log "configured model is unavailable or not currently free: $MODEL"
  fi
  while IFS= read -r candidate; do
    if is_verified_free_model "$candidate" && ! printf '%s\n' "${MODEL_CANDIDATES[@]:-}" | grep -Fxq "$candidate"; then
      MODEL_CANDIDATES+=("$candidate")
      MODEL_CANDIDATE_FOUND=1
      log "verified free model selected: $candidate"
      [[ "${#MODEL_CANDIDATES[@]}" -ge "$topic_limit" ]] && break
    fi
  done <<< "$(printf '%s\n%s\n%s\n' "$preferred" "$MODEL" "$AVAILABLE")"
  if ((${#MODEL_CANDIDATES[@]} > 0)); then MODEL_CANDIDATE_FOUND=1; fi
  return 0
}

if ((NEEDS_RESEARCH_BATCH == 0)); then
  MODEL_CANDIDATE_FOUND=1
elif [[ "$RECOVERY_ACTION" == resume ]]; then
  while IFS= read -r candidate; do
    [[ -n "$candidate" ]] || continue
    is_verified_free_model "$candidate" || fail "saved model is no longer available and verified free: $candidate" 2
    MODEL_CANDIDATES+=("$candidate")
  done < <(jq -r '.models[]' "$RUN_STATE_FILE")
  ((${#MODEL_CANDIDATES[@]} > 0)) || fail 'recovery record has no verified worker models' 2
  MODEL_CANDIDATE_FOUND=1
else
  find_model_candidates
  attempt=1
  while [[ "$MODEL_CANDIDATE_FOUND" != 1 && "$attempt" -lt 3 ]]; do
    attempt=$((attempt + 1))
    delay=$((attempt * 5))
    log "no currently available zero-priced model; retrying model discovery in ${delay}s (attempt $attempt/3)"
    sleep "$delay"
    AVAILABLE="$(opencode models --print-logs --log-level debug 2>/dev/null)" || AVAILABLE=''
    if [[ -n "$AVAILABLE" ]]; then
      log "OpenCode model listing: passed (attempt $attempt/3)"
      find_model_candidates
    else
      log "OpenCode model listing failed (attempt $attempt/3)"
    fi
  done
  [[ "$MODEL_CANDIDATE_FOUND" == 1 ]] \
    || fail 'no currently available OpenCode model has zero input and output price after 3 attempts'
fi

if [[ "${RESEARCH_PREFLIGHT:-0}" == 1 ]]; then
  log "preflight: passed; selected models: ${MODEL_CANDIDATES[*]}"
  printf 'Preflight passed; selected models: %s\n' "${MODEL_CANDIDATES[*]}"
  exit 0
fi

if [[ "$RECOVERY_ACTION" == resume ]]; then
  BASE_REF="$(jq -er '.base' "$RUN_STATE_FILE")" || fail 'recovery record has no base commit' 2
  [[ "$(dirname "$RUN_WORKTREE")" == "$BASE_ROOT/.workbench/repositories/research" && \
     "$(basename "$RUN_WORKTREE")" == run-* && ! -L "$RUN_WORKTREE" ]] \
    || fail 'recovery integration worktree path is invalid' 2
  if [[ -d "$RUN_WORKTREE" ]]; then
    REGISTERED_ROOT="$(git -C "$RUN_WORKTREE" rev-parse --show-toplevel 2>/dev/null)" \
      || fail 'recovery integration worktree is not registered with Git' 2
    [[ "$(cd "$REGISTERED_ROOT" && pwd -P)" == "$RUN_WORKTREE" ]] || fail 'recovery integration worktree path changed' 2
    [[ "$(git -C "$RUN_WORKTREE" branch --show-current)" == "$RESEARCH_BRANCH" ]] \
      || fail 'recovery branch does not match its saved run' 2
  elif [[ "$RUN_PHASE" == creating-worktree ]]; then
    if git show-ref --verify --quiet "refs/heads/$RESEARCH_BRANCH"; then
      [[ "$(git rev-parse "refs/heads/$RESEARCH_BRANCH")" == "$BASE_REF" ]] \
        || fail 'saved branch does not point to the recorded base commit' 2
      git worktree add "$RUN_WORKTREE" "$RESEARCH_BRANCH" >/dev/null 2>&1 || fail 'could not reattach the saved research branch' 2
    else
      git worktree add -b "$RESEARCH_BRANCH" "$RUN_WORKTREE" "$BASE_REF" >/dev/null 2>&1 || fail 'could not recreate the saved research worktree' 2
    fi
  else
    fail 'saved integration worktree is missing; run retained' 2
  fi
else
  BASE_REF=main
  if [[ "${RESEARCH_DRY_RUN:-0}" != 1 ]]; then
    if GIT_TERMINAL_PROMPT=0 git fetch origin main >/dev/null 2>&1; then
      BASE_REF=refs/remotes/origin/main
      log 'git fetch origin main: passed'
    else
      log 'warning: git fetch failed; continuing from current local main'
    fi
  fi

  RESEARCH_BRANCH="research/$(date '+%Y-%m-%d-%H%M%S')-$$"
  RUN_WORKTREE="$BASE_ROOT/.workbench/repositories/research/run-${RESEARCH_BRANCH#research/}"
  mkdir -p "$(dirname "$RUN_WORKTREE")"
  if [[ "${RESEARCH_DRY_RUN:-0}" == 1 ]]; then
    git worktree add --detach "$RUN_WORKTREE" "$BASE_REF" >/dev/null 2>&1 || fail 'could not create dry-run worktree'
    RUN_WORKTREE_CREATED=1
  else
    RUN_ID="${RESEARCH_BRANCH#research/}"
    [[ ! -e "$RUN_WORKTREE" && ! -L "$RUN_WORKTREE" ]] || fail 'research worktree path already exists' 2
    if git show-ref --verify --quiet "refs/heads/$RESEARCH_BRANCH"; then
      fail 'research branch already exists' 2
    fi
    RUN_DIR="$LOG_DIR/research-runs/$RUN_ID"
    mkdir -p "$(dirname "$RUN_DIR")"
    chmod 700 "$(dirname "$RUN_DIR")" || fail 'could not secure research recovery directory' 2
    mkdir -m 700 "$RUN_DIR" || fail 'could not create unique recovery directory' 2
    RUN_STATE_FILE="$RUN_DIR/run.json"
    RUN_PHASE=creating-worktree
    BASE_SHA="$(git rev-parse "$BASE_REF")" || fail 'could not resolve research base commit'
    MODEL_JSON="$(printf '%s\n' "${MODEL_CANDIDATES[@]}" | jq -R . | jq -s .)" || fail 'could not encode saved model list'
    CONTINUOUS_JSON=false
    [[ "${RESEARCH_CONTINUOUS:-0}" != 1 ]] || CONTINUOUS_JSON=true
    jq -n --arg id "$RUN_ID" --arg branch "$RESEARCH_BRANCH" --arg root "$RUN_WORKTREE" \
      --arg base "$BASE_SHA" --arg phase "$RUN_PHASE" --arg mode "${RESEARCH_CONTINUOUS:-0}" \
      --arg pid_start "$PID_START" --arg updated_at "$(timestamp)" \
      --argjson models "$MODEL_JSON" --argjson continuous "$CONTINUOUS_JSON" --argjson pid "$$" \
      '{version:2,id:$id,status:"running",phase:$phase,pid:$pid,pid_start:$pid_start,batch_pid:0,batch_pid_start:"",branch:$branch,root:$root,base:$base,models:$models,continuous:$continuous,updated_at:$updated_at}' \
      > "$RUN_STATE_FILE" || fail 'could not create research recovery record' 2
    chmod 600 "$RUN_STATE_FILE" || fail 'could not secure research recovery record' 2
    git worktree add -b "$RESEARCH_BRANCH" "$RUN_WORKTREE" "$BASE_REF" >/dev/null 2>&1 || fail 'could not create research worktree'
    RUN_WORKTREE_CREATED=1
  fi
fi
ROOT="$RUN_WORKTREE"
cd "$ROOT"
log "research worktree: $RUN_WORKTREE branch=$RESEARCH_BRANCH"

if [[ -n "$RUN_STATE_FILE" ]]; then
  OUTPUT_FILE="$RUN_DIR/coordinator.log"
  : > "$OUTPUT_FILE"
else
  OUTPUT_FILE="$(mktemp "$LOG_DIR/opencode.XXXXXXXX")"
fi
chmod 600 "$OUTPUT_FILE"

DRY_FLAG=false
[[ "${RESEARCH_DRY_RUN:-0}" != 1 ]] || DRY_FLAG=true
SKIP_RESEARCH_VALIDATION=0
SKIP_PUSH=0
SKIP_PR_CREATE=0
SKIP_MERGE_REQUEST=0
if [[ "$RECOVERY_ACTION" == resume ]]; then
  case "$RUN_PHASE" in
    committed) SKIP_RESEARCH_VALIDATION=1 ;;
    pushed) SKIP_RESEARCH_VALIDATION=1; SKIP_PUSH=1 ;;
    pr-created) SKIP_RESEARCH_VALIDATION=1; SKIP_PUSH=1; SKIP_PR_CREATE=1 ;;
    merge-requested) SKIP_RESEARCH_VALIDATION=1; SKIP_PUSH=1; SKIP_PR_CREATE=1; SKIP_MERGE_REQUEST=1 ;;
  esac
fi

BATCH_ALREADY_COMPLETE=0
if [[ "$RECOVERY_ACTION" == resume && -f "$RUN_DIR/batch.json" ]] && \
   [[ "$(jq -r '.complete // false' "$RUN_DIR/batch.json")" == true ]]; then
  BATCH_ALREADY_COMPLETE=1
fi
COORDINATOR_RESUME=0
if [[ "$RECOVERY_ACTION" == resume && -f "$RUN_DIR/batch.json" && "$BATCH_ALREADY_COMPLETE" == 0 ]]; then
  COORDINATOR_RESUME=1
fi
if [[ "$SKIP_RESEARCH_VALIDATION" == 0 && "$BATCH_ALREADY_COMPLETE" == 0 ]]; then
  if [[ -n "$RUN_STATE_FILE" && "$COORDINATOR_RESUME" == 0 ]]; then
    RUN_PHASE=building
    write_run_state "$RUN_PHASE" running || fail 'could not save research build phase' 2
  fi
  mise exec -- go build -o bin/research-batch ./cmd/research-batch > "$OUTPUT_FILE" 2>&1 || fail 'could not build research coordinator' 2
  PROGRESS_ARGS=(--progress-file "$PROGRESS_FILE")
  BATCH_ARGS=(--root "$ROOT" --state-dir "$LOG_DIR" "${PROGRESS_ARGS[@]}" \
    --dry-run="$DRY_FLAG" --deadline="${RESEARCH_DEADLINE:-0}")
  if [[ -n "$RUN_STATE_FILE" ]]; then
    BATCH_ARGS+=(--run-dir "$RUN_DIR" --run-id "$RUN_ID")
    if [[ "$COORDINATOR_RESUME" == 1 ]]; then BATCH_ARGS+=(--resume); fi
  fi
  if [[ -n "$RUN_STATE_FILE" ]]; then
    RUN_PHASE='batch-running'
    write_run_state "$RUN_PHASE" running || fail 'could not save research batch phase' 2
  fi
  if [[ -n "$RUN_STATE_FILE" ]]; then
    BATCH_GATE_FIFO="$RUN_DIR/.batch-start-$$.fifo"
    mkfifo "$BATCH_GATE_FIFO" || fail 'could not create coordinator start gate' 2
    chmod 600 "$BATCH_GATE_FIFO" || fail 'could not secure coordinator start gate' 2
    exec 9<>"$BATCH_GATE_FIFO"
    BATCH_GATE_OPEN=1
    "$BASH" -c 'exec 9>&-; IFS= read -r _ || exit 125; exec "$@"' \
      research-batch-start-gate "$ROOT/bin/research-batch" "${BATCH_ARGS[@]}" "${MODEL_CANDIDATES[@]}" \
      < "$BATCH_GATE_FIFO" > "$OUTPUT_FILE" 2>&1 &
  else
    "$ROOT/bin/research-batch" "${BATCH_ARGS[@]}" "${MODEL_CANDIDATES[@]}" > "$OUTPUT_FILE" 2>&1 &
  fi
  BATCH_PID=$!
  BATCH_PID_START="$(ps -p "$BATCH_PID" -o lstart= 2>/dev/null | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  printf '%s\n' "$BATCH_PID" > "$LOCK_DIR/batch_pid"
  printf '%s\n' "$BATCH_PID_START" > "$LOCK_DIR/batch_pid_start"
  printf '%s\n' "$BATCH_PID" > "$PIPELINE_LOCK_DIR/batch_pid"
  printf '%s\n' "$BATCH_PID_START" > "$PIPELINE_LOCK_DIR/batch_pid_start"
  if [[ -n "$RUN_STATE_FILE" ]]; then write_run_state "$RUN_PHASE" running || fail 'could not save coordinator PID' 2; fi
  if ((BATCH_GATE_OPEN)); then
    printf 'start\n' >&9 || fail 'could not release coordinator start gate' 2
    exec 9>&-
    BATCH_GATE_OPEN=0
    rm -f "$BATCH_GATE_FIFO" || fail 'could not remove coordinator start gate' 2
    BATCH_GATE_FIFO=''
  fi
  if wait "$BATCH_PID"; then
    BATCH_PID=''
    BATCH_PID_START=''
    rm -f "$LOCK_DIR/batch_pid" "$LOCK_DIR/batch_pid_start" "$PIPELINE_LOCK_DIR/batch_pid" "$PIPELINE_LOCK_DIR/batch_pid_start"
    log 'research batch: passed'
  else
    BATCH_STATUS=$?
    BATCH_PID=''
    BATCH_PID_START=''
    rm -f "$LOCK_DIR/batch_pid" "$LOCK_DIR/batch_pid_start" "$PIPELINE_LOCK_DIR/batch_pid" "$PIPELINE_LOCK_DIR/batch_pid_start"
    sed -n '1,80p' "$OUTPUT_FILE" >> "$ERROR_FILE"
    sed -n '1,80p' "$OUTPUT_FILE" >&2
    if [[ "$BATCH_STATUS" == 2 ]]; then
      fail 'research batch rejected invalid baseline evals; no worker was started' "$BATCH_STATUS"
    fi
    fail "research batch failed; recovery run retained: ${RUN_ID:-$BASE_ROOT/.workbench/repositories/research}" "$BATCH_STATUS"
  fi
  sed -n '1,80p' "$OUTPUT_FILE" >> "$LOG_FILE"
  TOPIC="$(sed -n 's/^TOPIC: //p' "$OUTPUT_FILE" | tail -n 1)"
  [[ -n "$TOPIC" ]] || fail 'research batch returned no topic'
  PARTIAL=0
  if grep -Fxq 'BATCH: partial' "$OUTPUT_FILE"; then PARTIAL=1; fi
  if [[ -n "$RUN_STATE_FILE" ]]; then
    RUN_PHASE='batch-complete'
    write_run_result "$TOPIC" "$PARTIAL" || fail 'could not save completed research result' 2
    write_run_state "$RUN_PHASE" running || fail 'could not save completed batch phase' 2
  fi
else
  if [[ "$BATCH_ALREADY_COMPLETE" == 1 ]]; then
    TOPIC="$(jq -er '.topics | join(" / ")' "$RUN_DIR/batch.json")" || fail 'saved batch result has no topics' 2
    PARTIAL=0
    [[ "$(jq -r '.partial // false' "$RUN_DIR/batch.json")" != true ]] || PARTIAL=1
    write_run_result "$TOPIC" "$PARTIAL" || fail 'could not save recovered batch result' 2
    case "$RUN_PHASE" in
      creating-worktree|building|batch-running)
        RUN_PHASE='batch-complete'
        write_run_state "$RUN_PHASE" running || fail 'could not save recovered batch phase' 2
        ;;
    esac
  else
    TOPIC="$(jq -er '.topic' "$RUN_STATE_FILE")" || fail 'recovery record has no completed research topic' 2
    PARTIAL="$(jq -r '.partial // 0' "$RUN_STATE_FILE")"
  fi
fi
log "selected topic: $TOPIC"

check_paths() {
  local entry path status
  while IFS= read -r -d '' entry; do
    status="${entry:0:2}"
    path="${entry:3}"
    [[ "$status" != *R* && "$status" != *C* ]] || fail "rename/copy is forbidden: $path"
    case "$path" in
      knowledge/*|sources/catalog/*|evals/knowledge/*) ;;
      *) fail "change outside research allowlist: $path" ;;
    esac
  done < <(git status --porcelain -z --untracked-files=all)
}
if [[ "$RECOVERY_ACTION" == resume && "$RUN_PHASE" == validated ]]; then
  CURRENT_HEAD="$(git rev-parse HEAD)"
  EXPECTED_SUBJECT="docs: ${TOPIC}の根拠と適用条件を記録"
  if [[ "$CURRENT_HEAD" != "$(jq -r '.base' "$RUN_STATE_FILE")" && \
        "$(git log -1 --format=%s)" == "$EXPECTED_SUBJECT" && \
        "$(git rev-parse HEAD^)" == "$(jq -r '.base' "$RUN_STATE_FILE")" && \
        -z "$(git status --porcelain --untracked-files=all)" ]]; then
    RUN_PHASE=committed
    write_run_state "$RUN_PHASE" running || fail 'could not save recovered commit phase' 2
    SKIP_RESEARCH_VALIDATION=1
  fi
fi

if [[ "$SKIP_RESEARCH_VALIDATION" == 0 ]]; then
check_paths

if [[ "${RESEARCH_DRY_RUN:-0}" == 1 ]]; then
  [[ -z "$(git status --porcelain --untracked-files=all)" ]] || fail 'dry run modified files'
  mise exec -- just validate >/dev/null 2>&1 || fail 'dry-run validation failed'
  log 'dry-run validation: passed; no commit'
  exit 0
fi

for prefix in knowledge/ evals/knowledge/; do
  git status --porcelain --untracked-files=all -- "$prefix" | grep -q . \
    || fail "required research artifact missing: $prefix"
done
# A verified, already-cataloged primary source may be reused. `just validate`
# checks that the knowledge source ID, URL, and type match that catalog.

mise exec -- just validate >/dev/null 2>&1 || fail 'just validate failed'
log 'just validate: passed'
mise exec -- just index >/dev/null 2>&1 || fail 'just index failed'
log 'just index: passed'
if ! mise exec -- just check > "$OUTPUT_FILE" 2>&1; then
  printf '%s just check output (first 120 lines):\n' "$(timestamp)" >> "$ERROR_FILE"
  sed -n '1,120p' "$OUTPUT_FILE" >> "$ERROR_FILE"
  sed -n '1,120p' "$OUTPUT_FILE" >&2
  fail 'just check failed'
fi
log 'just check: passed'
check_paths

git diff --check || fail 'git diff --check failed'
git add -- knowledge/ sources/catalog/ evals/knowledge/
git diff --cached --check || fail 'staged diff check failed'
git diff --cached --name-only | grep -q '^knowledge/' || fail 'no knowledge change staged'
git diff --cached --name-only | while IFS= read -r path; do
  case "$path" in knowledge/*|sources/catalog/*|evals/knowledge/*) ;; *) exit 1 ;; esac
done || fail 'staged path outside allowlist'
git diff --cached --stat >> "$LOG_FILE"
git diff --cached --quiet && fail 'no staged research change'
if [[ -n "$RUN_STATE_FILE" ]]; then
  RUN_PHASE=validated
  write_run_state "$RUN_PHASE" running || fail 'could not save validation checkpoint' 2
fi
git commit -m "docs: ${TOPIC}の根拠と適用条件を記録" -m "一次資料に基づく知識と検索 eval を残し、後続の実装・運用で根拠を再利用できるようにする。" >/dev/null 2>&1 || fail 'research commit failed'
SHA="$(git rev-parse HEAD)"
log "commit SHA: $SHA"
[[ -z "$(git status --porcelain --untracked-files=all)" ]] || fail 'working tree is dirty after commit'
if [[ -n "$RUN_STATE_FILE" ]]; then
  RUN_PHASE=committed
  write_run_state "$RUN_PHASE" running || fail 'could not save commit checkpoint' 2
fi
fi

if [[ "$SKIP_PUSH" == 0 ]]; then
  PUSH_SUCCEEDED=0
  for attempt in 1 2 3; do
    if git push --set-upstream origin "$RESEARCH_BRANCH" >/dev/null 2>&1; then
      log "research branch push: passed (attempt $attempt/3)"
      PUSH_SUCCEEDED=1
      break
    fi
    if [[ "$attempt" -lt 3 ]]; then
      delay=$((attempt * 5))
      log "research branch push failed; retrying in ${delay}s (attempt $attempt/3)"
      sleep "$delay"
    fi
  done
  [[ "$PUSH_SUCCEEDED" == 1 ]] || fail 'research branch push failed after 3 attempts'
  if [[ -n "$RUN_STATE_FILE" ]]; then
    RUN_PHASE=pushed
    write_run_state "$RUN_PHASE" running || fail 'could not save push checkpoint' 2
  fi
fi

PR_TITLE="docs: 自動リサーチ - $TOPIC"
if [[ "$SKIP_PR_CREATE" == 1 ]]; then
  PR_URL="$(jq -er '.pr_url' "$RUN_STATE_FILE")" || fail 'recovery record has no pull request URL' 2
  GH_PROMPT_DISABLED=1 gh pr view "$PR_URL" --json url >/dev/null 2>&1 || fail 'saved pull request is no longer available; run retained' 2
else
  if [[ -n "$RUN_STATE_FILE" ]]; then
    PR_BODY_FILE="$RUN_DIR/research-pr.md"
  else
    PR_BODY_FILE="$(mktemp "$LOG_DIR/research-pr.XXXXXXXX")"
  fi
  chmod 600 "$PR_BODY_FILE"
  # Literal backticks form Markdown code spans in the PR body.
  # shellcheck disable=SC2016
  printf '自動 Research Pipeline による更新です。\n\n対象テーマ: %s\n\n`just validate`・`just index`・`just check` は成功しました。\n' \
    "$TOPIC" > "$PR_BODY_FILE"

  PR_URL=''
  for attempt in 1 2 3; do
    if PR_URL="$(GH_PROMPT_DISABLED=1 gh pr create \
      --base main \
      --head "$RESEARCH_BRANCH" \
      --title "$PR_TITLE" \
      --body-file "$PR_BODY_FILE" 2>/dev/null)"; then
      break
    fi

    # A request may have created the PR even if the client lost its response.
    PR_URL="$(GH_PROMPT_DISABLED=1 gh pr view "$RESEARCH_BRANCH" --json url --jq '.url' 2>/dev/null || true)"
    [[ -n "$PR_URL" ]] && break
    if [[ "$attempt" -lt 3 ]]; then
      delay=$((attempt * 5))
      log "pull request creation failed; retrying in ${delay}s (attempt $attempt/3)"
      sleep "$delay"
    fi
  done
  [[ -n "$PR_URL" ]] || fail 'research pull request creation failed after 3 attempts' 2
  if [[ -n "$RUN_STATE_FILE" ]]; then
    write_run_result "$TOPIC" "$PARTIAL" "$PR_URL" || fail 'could not save pull request checkpoint' 2
    RUN_PHASE=pr-created
    write_run_state "$RUN_PHASE" running || fail 'could not save pull request phase' 2
  fi
  log "pull request: $PR_URL"
fi

PR_MERGEABLE=''
PR_MERGE_STATE=''
for attempt in 1 2 3 4 5 6; do
  if PR_DETAILS="$(GH_PROMPT_DISABLED=1 gh pr view "$PR_URL" \
    --json mergeable,mergeStateStatus \
    --jq '[.mergeable, .mergeStateStatus] | @tsv' 2>/dev/null)"; then
    IFS=$'\t' read -r PR_MERGEABLE PR_MERGE_STATE <<< "$PR_DETAILS"
    case "$PR_MERGEABLE" in
      MERGEABLE|CONFLICTING) break ;;
    esac
  fi
  if [[ "$attempt" -lt 6 ]]; then
    log "pull request mergeability is not ready; retrying in 5s (attempt $attempt/6)"
    sleep 5
  fi
done

if [[ "$PR_MERGEABLE" == MERGEABLE && "$PR_MERGE_STATE" == BEHIND ]]; then
  if GH_PROMPT_DISABLED=1 gh pr update-branch "$PR_URL" >/dev/null 2>&1; then
    log 'research PR branch updated from main'
    PR_MERGEABLE=''
    PR_MERGE_STATE=''
    for attempt in 1 2 3 4 5 6; do
      if PR_DETAILS="$(GH_PROMPT_DISABLED=1 gh pr view "$PR_URL" \
        --json mergeable,mergeStateStatus \
        --jq '[.mergeable, .mergeStateStatus] | @tsv' 2>/dev/null)"; then
        IFS=$'\t' read -r PR_MERGEABLE PR_MERGE_STATE <<< "$PR_DETAILS"
        case "$PR_MERGEABLE" in
          MERGEABLE|CONFLICTING) break ;;
        esac
      fi
      if [[ "$attempt" -lt 6 ]]; then
        log "updated PR mergeability is not ready; retrying in 5s (attempt $attempt/6)"
        sleep 5
      fi
    done
  else
    log "warning: could not update research PR branch from main: $PR_URL"
    PR_MERGEABLE='UPDATE_FAILED'
  fi
fi

case "$PR_MERGEABLE" in
  CONFLICTING)
    fail "research PR has conflicts; left open for resolution: $PR_URL" 2
    ;;
  UPDATE_FAILED)
    fail "research PR branch is behind main and could not be updated; left open: $PR_URL" 2
    ;;
  *)
    AUTO_MERGE_REQUESTED=0
    PR_STATE="$(GH_PROMPT_DISABLED=1 gh pr view "$PR_URL" --json state --jq '.state' 2>/dev/null || true)"
    if [[ "$PR_STATE" == MERGED ]]; then
      AUTO_MERGE_REQUESTED=1
    elif [[ "$SKIP_MERGE_REQUEST" == 1 ]]; then
      [[ "$PR_STATE" == MERGED || "$PR_STATE" == OPEN ]] || fail "saved research PR is not open: $PR_URL" 2
      AUTO_MERGE_REQUESTED=1
    else
      for attempt in 1 2 3; do
        if GH_PROMPT_DISABLED=1 gh pr merge "$PR_URL" --squash --auto >/dev/null 2>&1; then
          AUTO_MERGE_REQUESTED=1
          break
        fi
        if [[ "$attempt" -lt 3 ]]; then
          delay=$((attempt * 5))
          log "auto-merge request failed; retrying in ${delay}s (attempt $attempt/3)"
          sleep "$delay"
        fi
      done
    fi

    if [[ "$AUTO_MERGE_REQUESTED" == 1 ]]; then
      PR_STATE="$(GH_PROMPT_DISABLED=1 gh pr view "$PR_URL" --json state --jq '.state' 2>/dev/null || true)"
      if [[ "$PR_STATE" == MERGED ]]; then
        log "GitHub auto-merge completed with squash (merge state: ${PR_MERGE_STATE:-unknown})"
      else
        log "GitHub auto-merge enabled with squash (merge state: ${PR_MERGE_STATE:-unknown})"
      fi
      if [[ -n "$RUN_STATE_FILE" && "$SKIP_MERGE_REQUEST" == 0 ]]; then
        RUN_PHASE=merge-requested
        write_run_state "$RUN_PHASE" running || fail 'could not save auto-merge checkpoint' 2
      fi
    else
      fail "GitHub did not accept auto-merge; PR remains open: $PR_URL" 2
    fi
    ;;
esac

if [[ "${PR_STATE:-}" != MERGED ]]; then
  [[ "${PR_STATE:-}" != CLOSED ]] || fail "research PR closed without merge: $PR_URL" 2
  # Wait until the auto-merge finishes so local main can follow the merged result.
  for attempt in {1..60}; do
    if [[ "${RESEARCH_CONTINUOUS:-0}" == 1 ]]; then
      [[ "$(date +%s)" -lt "${RESEARCH_DEADLINE:-0}" ]] || fail "continuous deadline reached; PR retained: $PR_URL" 2
    fi
    sleep 10
    PR_STATE="$(GH_PROMPT_DISABLED=1 gh pr view "$PR_URL" --json state --jq '.state' 2>/dev/null || true)"
    [[ "$PR_STATE" != MERGED ]] || break
    [[ "$PR_STATE" != CLOSED ]] || fail "research PR closed without merge: $PR_URL" 2
  done
  [[ "$PR_STATE" == MERGED ]] || fail "research PR did not merge before timeout; PR remains open: $PR_URL" 2
fi
if [[ "$PR_STATE" == MERGED ]]; then
  if ((BASE_CHECKOUT_WAS_DIRTY)); then
    log 'skipped local main sync to preserve original checkout changes'
  else
    # Keep the refspec implicit so pruning covers every configured origin branch.
    if ! git -C "$BASE_ROOT" pull --ff-only --prune > "$OUTPUT_FILE" 2>&1; then
      sed -n '1,40p' "$OUTPUT_FILE" >> "$ERROR_FILE"
      fail "research PR merged, but local main could not be fast-forwarded; see $ERROR_FILE" 2
    fi
    log 'pulled merged origin/main into original checkout'
  fi
fi
if [[ -n "$RUN_STATE_FILE" ]]; then
  RUN_PHASE=complete
  write_run_result "$TOPIC" "$PARTIAL" "$PR_URL" || fail 'could not save completed run result' 2
  write_run_state "$RUN_PHASE" complete || fail 'could not save completed run state' 2
fi
[[ "$PARTIAL" != 1 ]] || fail 'successful topics were published; failed worker was discarded during run cleanup'
if [[ "$PR_STATE" == MERGED && "$BASE_CHECKOUT_WAS_DIRTY" == 0 ]]; then
  log 'research completed; original checkout updated from origin/main'
elif [[ "$BASE_CHECKOUT_WAS_DIRTY" == 1 ]]; then
  log 'research completed; original checkout changes preserved'
else
  log 'research completed; PR awaits auto-merge; original checkout unchanged'
fi
