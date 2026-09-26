#!/bin/bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
BASE_ROOT="$ROOT"
LOG_DIR="${RESEARCH_LOG_DIR:-$HOME/Library/Logs/ai-engineering-kit}"
CONFIG_FILE="${RESEARCH_ENV_FILE:-$HOME/Library/Application Support/ai-engineering-kit/research.env}"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/research.log"
ERROR_FILE="$LOG_DIR/research-error.log"
LOCK_DIR="$LOG_DIR/research.lock"
[[ "${RESEARCH_CONTINUOUS:-0}" != 1 ]] || LOCK_DIR="$LOG_DIR/research-continuous.lock"
PROGRESS_FILE="${RESEARCH_PROGRESS_FILE:-}"
RESEARCH_BRANCH=''
RUN_WORKTREE=''
OUTPUT_FILE=''
PR_BODY_FILE=''
BATCH_PID=''
PROGRESS_PID=''
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
monitor_progress() {
  local last='' current percent domain_name domain topic phase monitor_sleep=''
  trap '[[ -z "$monitor_sleep" ]] || kill "$monitor_sleep" 2>/dev/null || true; exit 0' INT TERM
  while [[ -n "$BATCH_PID" ]] && kill -0 "$BATCH_PID" 2>/dev/null; do
    current=''
    if [[ -f "$PROGRESS_FILE" ]]; then
      current="$(jq -r '[(.percent | tostring), (.domain_name // "-"), (.domain // "-"), (.topic // "テーマ選定中"), (.phase // "-")] | @tsv' "$PROGRESS_FILE" 2>/dev/null)" || current=''
    fi
    if [[ -n "$current" && "$current" != "$last" ]]; then
      IFS=$'\t' read -r percent domain_name domain topic phase <<< "$current"
      printf '調査進捗（推定）: %s%% | 領域=%s (%s) | テーマ=%s | 段階=%s\n' \
        "$percent" "$domain_name" "$domain" "$topic" "$phase"
      last="$current"
    fi
    sleep 2 &
    monitor_sleep=$!
    wait "$monitor_sleep" || true
    monitor_sleep=''
  done
}
stop_progress_monitor() {
  if [[ -n "$PROGRESS_PID" ]] && kill -0 "$PROGRESS_PID" 2>/dev/null; then
    kill "$PROGRESS_PID" 2>/dev/null || true
    wait "$PROGRESS_PID" || true
  fi
  PROGRESS_PID=''
}
cleanup() {
  local exit_code="$1"

  if [[ -n "$BATCH_PID" ]] && kill -0 "$BATCH_PID" 2>/dev/null; then
    kill -TERM "$BATCH_PID"
    wait "$BATCH_PID" || true
  fi
  stop_progress_monitor

  [[ -z "$OUTPUT_FILE" ]] || rm -f "$OUTPUT_FILE"
  [[ -z "$PR_BODY_FILE" ]] || rm -f "$PR_BODY_FILE"

  if [[ -n "$RUN_WORKTREE" ]]; then
    cd "$BASE_ROOT"
    if git worktree remove "$RUN_WORKTREE" >/dev/null 2>&1; then
      log "removed clean integration worktree: $RUN_WORKTREE"
    else
      log "recovery: retained integration worktree: $RUN_WORKTREE branch=$RESEARCH_BRANCH"
    fi
  fi

  rm -f "$LOCK_DIR/pid" 2>/dev/null || true
  rmdir "$LOCK_DIR" 2>/dev/null || true
  return "$exit_code"
}

if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  log 'skipped: another run holds the lock (remove stale lock only after confirming no run exists)'
  exit 3
fi
trap 'cleanup "$?"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
printf '%s\n' "$$" > "$LOCK_DIR/pid"

cd "$ROOT"
log 'start'
[[ "$(git branch --show-current)" == main ]] || fail 'branch is not main' 2
WORKTREE_STATUS="$(git status --porcelain --untracked-files=all)" || fail 'could not inspect working tree' 2
if [[ -n "$WORKTREE_STATUS" ]]; then
  log 'working tree changes (first 20 entries):'
  printf '%s\n' "$WORKTREE_STATUS" | sed -n '1,20p' >> "$LOG_FILE"
  fail 'working tree is dirty; see changed paths in research.log' 2
fi
if [[ -f "$LOG_DIR/cooldown-until" ]]; then
  read -r COOLDOWN_UNTIL < "$LOG_DIR/cooldown-until" || fail 'invalid cooldown state' 2
  [[ "$COOLDOWN_UNTIL" =~ ^[0-9]+$ ]] || fail 'invalid cooldown state' 2
  if [[ "$(date +%s)" -lt "$COOLDOWN_UNTIL" ]]; then
    log "skipped: provider cooldown until $COOLDOWN_UNTIL"
    exit 4
  fi
fi
[[ -f "$CONFIG_FILE" ]] || fail 'research.env is missing' 2

# This local file contains one non-secret assignment, never shell-evaluate it.
MODEL="$(sed -nE 's/^OPENCODE_RESEARCH_MODEL=([A-Za-z0-9._\/-]+)$/\1/p' "$CONFIG_FILE")"
[[ -n "$MODEL" && "$(wc -l < "$CONFIG_FILE" | tr -d ' ')" == 1 ]] || fail 'invalid research.env' 2
[[ "$MODEL" =~ ^opencode/[A-Za-z0-9._/-]+$ ]] || fail 'configured model must use the opencode provider' 2

for command in git opencode curl jq just mise; do
  command -v "$command" >/dev/null || fail "missing command: $command" 2
done
if [[ "${RESEARCH_DRY_RUN:-0}" != 1 ]]; then
  command -v gh >/dev/null || fail 'missing command: gh' 2
  GH_PROMPT_DISABLED=1 gh auth status >/dev/null 2>&1 || fail 'GitHub CLI is not authenticated' 2
fi

# Retry transient discovery failures, then prefer the configured model pair.
# Continuous execution accepts only its exact configured model.
AVAILABLE=''
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
  local candidate preferred
  MODEL_CANDIDATES=()
  MODEL_CANDIDATE_FOUND=0
  if [[ "${RESEARCH_CONTINUOUS:-0}" == 1 ]]; then
    preferred="$(jq -er '.continuous.model' "$BASE_ROOT/config/research.json")" || fail 'invalid continuous model' 2
    if is_verified_free_model "$preferred"; then
      MODEL_CANDIDATES+=("$preferred")
      MODEL_CANDIDATE_FOUND=1
    fi
    return 0
  fi
  preferred="$(jq -er '.parallel.preferred_models[]' "$BASE_ROOT/config/research.json")" || fail 'invalid preferred models' 2
  if ! is_verified_free_model "$MODEL"; then
    log "configured model is unavailable or not currently free: $MODEL"
  fi
  while IFS= read -r candidate; do
    if is_verified_free_model "$candidate" && ! printf '%s\n' "${MODEL_CANDIDATES[@]:-}" | grep -Fxq "$candidate"; then
      MODEL_CANDIDATES+=("$candidate")
      MODEL_CANDIDATE_FOUND=1
      log "verified free model selected: $candidate"
      [[ "${#MODEL_CANDIDATES[@]}" -ge 2 ]] && break
    fi
  done <<< "$(printf '%s\n%s\n%s\n' "$preferred" "$MODEL" "$AVAILABLE")"
  return 0
}

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

if [[ "${RESEARCH_PREFLIGHT:-0}" == 1 ]]; then
  log "preflight: passed; selected models: ${MODEL_CANDIDATES[*]}"
  printf 'Preflight passed; selected models: %s\n' "${MODEL_CANDIDATES[*]}"
  exit 0
fi

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
else
  git worktree add -b "$RESEARCH_BRANCH" "$RUN_WORKTREE" "$BASE_REF" >/dev/null 2>&1 || fail 'could not create research worktree'
fi
ROOT="$RUN_WORKTREE"
cd "$ROOT"
log "research worktree: $RUN_WORKTREE branch=$RESEARCH_BRANCH"

OUTPUT_FILE="$(mktemp "$LOG_DIR/opencode.XXXXXXXX")"
chmod 600 "$OUTPUT_FILE"

DRY_FLAG=false
[[ "${RESEARCH_DRY_RUN:-0}" != 1 ]] || DRY_FLAG=true
mise exec -- go build -o bin/research-batch ./cmd/research-batch > "$OUTPUT_FILE" 2>&1 || fail 'could not build research coordinator' 2
PROGRESS_ARGS=(--progress-file "$PROGRESS_FILE")
"$ROOT/bin/research-batch" --root "$ROOT" --state-dir "$LOG_DIR" \
  "${PROGRESS_ARGS[@]}" \
  --dry-run="$DRY_FLAG" --deadline="${RESEARCH_DEADLINE:-0}" "${MODEL_CANDIDATES[@]}" > "$OUTPUT_FILE" 2>&1 &
BATCH_PID=$!
if [[ -n "$PROGRESS_FILE" ]]; then
  monitor_progress &
  PROGRESS_PID=$!
fi
if wait "$BATCH_PID"; then
  BATCH_PID=''
  log 'research batch: passed'
else
  BATCH_PID=''
  sed -n '1,80p' "$OUTPUT_FILE" >> "$ERROR_FILE"
  sed -n '1,80p' "$OUTPUT_FILE" >&2
  fail "research batch failed; recovery worktrees: $BASE_ROOT/.workbench/repositories/research"
fi
stop_progress_monitor
sed -n '1,80p' "$OUTPUT_FILE" >> "$LOG_FILE"
TOPIC="$(sed -n 's/^TOPIC: //p' "$OUTPUT_FILE" | tail -n 1)"
[[ -n "$TOPIC" ]] || fail 'research batch returned no topic'
log "selected topic: $TOPIC"
PARTIAL=0
if grep -Fxq 'BATCH: partial' "$OUTPUT_FILE"; then PARTIAL=1; fi

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
git commit -m "docs: ${TOPIC}の根拠と適用条件を記録" -m "一次資料に基づく知識と検索 eval を残し、後続の実装・運用で根拠を再利用できるようにする。" >/dev/null 2>&1 || fail 'research commit failed'
SHA="$(git rev-parse HEAD)"
log "commit SHA: $SHA"
[[ -z "$(git status --porcelain --untracked-files=all)" ]] || fail 'working tree is dirty after commit'

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

PR_TITLE="docs: 自動リサーチ - $TOPIC"
PR_BODY_FILE="$(mktemp "$LOG_DIR/research-pr.XXXXXXXX")"
chmod 600 "$PR_BODY_FILE"
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
log "pull request: $PR_URL"

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

    if [[ "$AUTO_MERGE_REQUESTED" == 1 ]]; then
      PR_STATE="$(GH_PROMPT_DISABLED=1 gh pr view "$PR_URL" --json state --jq '.state' 2>/dev/null || true)"
      if [[ "$PR_STATE" == MERGED ]]; then
        log "GitHub auto-merge completed with squash (merge state: ${PR_MERGE_STATE:-unknown})"
      else
        log "GitHub auto-merge enabled with squash (merge state: ${PR_MERGE_STATE:-unknown})"
      fi
    else
      fail "GitHub did not accept auto-merge; PR remains open: $PR_URL" 2
    fi
    ;;
esac

if [[ "${RESEARCH_CONTINUOUS:-0}" == 1 && "${PR_STATE:-}" != MERGED ]]; then
  # Wait for this result before selecting another topic from remote main.
  for attempt in {1..60}; do
    [[ "$(date +%s)" -lt "${RESEARCH_DEADLINE:-0}" ]] || fail "continuous deadline reached; PR retained: $PR_URL" 2
    sleep 10
    PR_STATE="$(GH_PROMPT_DISABLED=1 gh pr view "$PR_URL" --json state --jq '.state' 2>/dev/null || true)"
    [[ "$PR_STATE" != MERGED ]] || break
    [[ "$PR_STATE" != CLOSED ]] || fail "research PR closed without merge: $PR_URL" 2
  done
  [[ "$PR_STATE" == MERGED ]] || fail "research PR awaiting merge; continuous run stopped: $PR_URL" 2
fi
[[ "$PARTIAL" != 1 ]] || fail 'successful research published; failed worker retained for inspection'
log 'research completed; original checkout unchanged'
