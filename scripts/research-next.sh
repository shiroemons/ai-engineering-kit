#!/bin/bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LOG_DIR="$HOME/Library/Logs/ai-engineering-kit"
CONFIG_FILE="$HOME/Library/Application Support/ai-engineering-kit/research.env"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/research.log"
ERROR_FILE="$LOG_DIR/research-error.log"
LOCK_DIR="$LOG_DIR/research.lock"
RESEARCH_BRANCH=''
OUTPUT_FILE=''
PR_BODY_FILE=''
timestamp() { date '+%Y-%m-%dT%H:%M:%S%z'; }
log() { printf '%s %s\n' "$(timestamp)" "$*" >> "$LOG_FILE"; }
fail() {
  local message="$*"
  log "failure: $message"
  printf '%s %s\n' "$(timestamp)" "$message" >> "$ERROR_FILE"
  printf 'research: %s (details: %s)\n' "$message" "$ERROR_FILE" >&2
  exit 1
}
cleanup() {
  local exit_code="$1"
  local current_branch=''

  [[ -z "$OUTPUT_FILE" ]] || rm -f "$OUTPUT_FILE"
  [[ -z "$PR_BODY_FILE" ]] || rm -f "$PR_BODY_FILE"

  if [[ -n "$RESEARCH_BRANCH" ]]; then
    current_branch="$(git branch --show-current 2>/dev/null || true)"
    if [[ "$current_branch" == "$RESEARCH_BRANCH" ]]; then
      if [[ -z "$(git status --porcelain --untracked-files=all 2>/dev/null)" ]]; then
        if git switch main >/dev/null 2>&1; then
          log 'returned to main'
        else
          log 'warning: could not return to main after research run'
          if ((exit_code == 0)); then
            exit_code=1
          fi
        fi
      else
        log "warning: left $RESEARCH_BRANCH checked out because the working tree contains changes"
      fi
    fi
  fi

  rm -f "$LOCK_DIR/pid" 2>/dev/null || true
  rmdir "$LOCK_DIR" 2>/dev/null || true
  return "$exit_code"
}

if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  log 'skipped: another run holds the lock (remove stale lock only after confirming no run exists)'
  exit 0
fi
trap 'cleanup "$?"' EXIT
printf '%s\n' "$$" > "$LOCK_DIR/pid"

cd "$ROOT"
log 'start'
[[ "$(git branch --show-current)" == main ]] || fail 'branch is not main'
[[ -z "$(git status --porcelain --untracked-files=all)" ]] || fail 'working tree is dirty'
[[ -f "$CONFIG_FILE" ]] || fail 'research.env is missing'

# This local file contains one non-secret assignment, never shell-evaluate it.
MODEL="$(sed -nE 's/^OPENCODE_RESEARCH_MODEL=([A-Za-z0-9._\/-]+)$/\1/p' "$CONFIG_FILE")"
[[ -n "$MODEL" && "$(wc -l < "$CONFIG_FILE" | tr -d ' ')" == 1 ]] || fail 'invalid research.env'
[[ "$MODEL" =~ ^opencode/[A-Za-z0-9._/-]+$ ]] || fail 'configured model must use the opencode provider'

for command in git opencode curl jq just mise; do
  command -v "$command" >/dev/null || fail "missing command: $command"
done
if [[ "${RESEARCH_DRY_RUN:-0}" != 1 ]]; then
  command -v gh >/dev/null || fail 'missing command: gh'
  GH_PROMPT_DISABLED=1 gh auth status >/dev/null 2>&1 || fail 'GitHub CLI is not authenticated'
fi

# Retry transient catalog and pricing failures. Then keep the configured model
# first and add at most two currently available OpenCode models whose live
# catalog price is exactly zero for both input and output.
AVAILABLE=''
for attempt in 1 2 3; do
  if AVAILABLE="$(opencode models --print-logs --log-level debug 2>/dev/null)" && [[ -n "$AVAILABLE" ]]; then
    log "OpenCode model listing: passed (attempt $attempt/3)"
    break
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
  '.opencode.models | to_entries[] | select(.value.cost.input == 0 and .value.cost.output == 0) | .key')" \
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
  local candidate fallback_count
  MODEL_CANDIDATES=()
  MODEL_CANDIDATE_FOUND=0
  if is_verified_free_model "$MODEL"; then
    MODEL_CANDIDATES+=("$MODEL")
    MODEL_CANDIDATE_FOUND=1
  else
    log "configured model is unavailable or not currently free: $MODEL"
  fi
  fallback_count=0
  while IFS= read -r candidate; do
    [[ "$candidate" == "$MODEL" ]] && continue
    [[ "$candidate" == opencode/* ]] || continue
    if is_verified_free_model "$candidate"; then
      MODEL_CANDIDATES+=("$candidate")
      MODEL_CANDIDATE_FOUND=1
      fallback_count=$((fallback_count + 1))
      log "verified free fallback available: $candidate"
      [[ "$fallback_count" -ge 2 ]] && break
    fi
  done <<< "$AVAILABLE"
  return 0
}

find_model_candidates
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

if [[ "${RESEARCH_DRY_RUN:-0}" != 1 ]]; then
  # Keep the daily research moving if remote synchronization is unavailable.
  if GIT_TERMINAL_PROMPT=0 git pull --ff-only >/dev/null 2>&1; then
    log 'git pull --ff-only: passed'
  else
    log 'warning: git pull --ff-only failed; continuing from current local main'
  fi
  [[ -z "$(git status --porcelain --untracked-files=all)" ]] || fail 'working tree became dirty after pull'

  RESEARCH_BRANCH="research/$(date '+%Y-%m-%d-%H%M%S')"
  git switch -c "$RESEARCH_BRANCH" >/dev/null 2>&1 || fail 'could not create research branch'
  log "research branch: $RESEARCH_BRANCH"
fi

OUTPUT_FILE="$(mktemp "$LOG_DIR/opencode.XXXXXXXX")"
chmod 600 "$OUTPUT_FILE"

if [[ "${RESEARCH_DRY_RUN:-0}" == 1 ]]; then
  PROMPT='Inspect the repository and choose the next research topic. This is a dry run: do not edit any file or run modifying commands. Explain your choice and finish with TOPIC: <technology and topic>.'
else
  PROMPT='/research-next'
fi

TOPIC=''
for candidate in "${MODEL_CANDIDATES[@]}"; do
  for attempt in 1 2; do
    log "research attempt: model=$candidate attempt=$attempt/2"
    if (cd "$ROOT" && opencode run --standalone --agent knowledge-researcher --model "$candidate" "$PROMPT") > "$OUTPUT_FILE" 2>&1; then
      log "OpenCode exit status: 0 (model=$candidate attempt=$attempt/2)"
      TOPIC="$(sed -nE 's/.*TOPIC:[[:space:]]*//p' "$OUTPUT_FILE" | tail -n 1 | sed -E 's/\*\*$//' | tr -cd '[:print:]' | cut -c 1-120)"
      if [[ -n "$TOPIC" ]]; then
        log "selected topic: $TOPIC"
        break 2
      fi
      log "OpenCode did not report a selected topic (model=$candidate attempt=$attempt/2)"
    else
      status=$?
      log "OpenCode exit status: $status (model=$candidate attempt=$attempt/2)"
    fi

    if [[ -n "$(git status --porcelain --untracked-files=all)" ]]; then
      fail 'OpenCode left file changes after a failed attempt; automatic retry stopped to avoid duplicate edits'
    fi
    if [[ "$attempt" -lt 2 ]]; then
      log "retrying research with the same model in 10s: $candidate"
      sleep 10
    fi
  done
done
[[ -n "$TOPIC" ]] || fail 'research failed with all currently available, verified-free models'

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
mise exec -- just check >/dev/null 2>&1 || fail 'just check failed'
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
git commit -m "docs: 自動リサーチ結果を更新" >/dev/null 2>&1 || fail 'research commit failed'
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
[[ -n "$PR_URL" ]] || fail 'research pull request creation failed after 3 attempts'
log "pull request: $PR_URL"

git switch main >/dev/null 2>&1 || fail 'could not return to main after creating pull request'
log 'returned to main after creating pull request'

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
    log "warning: research PR has conflicts; left open for resolution: $PR_URL"
    ;;
  UPDATE_FAILED)
    log "warning: research PR branch is behind main and could not be updated; left open: $PR_URL"
    ;;
  *)
    AUTO_MERGE_REQUESTED=0
    for attempt in 1 2 3; do
      if GH_PROMPT_DISABLED=1 gh pr merge "$PR_URL" --squash --auto --delete-branch >/dev/null 2>&1; then
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
      log "warning: GitHub did not accept auto-merge; PR remains open: $PR_URL"
    fi
    ;;
esac

if GIT_TERMINAL_PROMPT=0 git pull --ff-only >/dev/null 2>&1; then
  log 'local main refresh after PR: passed'
else
  log 'warning: local main refresh after PR failed; next run will retry'
fi
