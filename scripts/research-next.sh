#!/bin/bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LOG_DIR="$HOME/Library/Logs/ai-engineering-kit"
CONFIG_FILE="$HOME/Library/Application Support/ai-engineering-kit/research.env"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/research.log"
ERROR_FILE="$LOG_DIR/research-error.log"
LOCK_DIR="$LOG_DIR/research.lock"
timestamp() { date '+%Y-%m-%dT%H:%M:%S%z'; }
log() { printf '%s %s\n' "$(timestamp)" "$*" >> "$LOG_FILE"; }
fail() {
  local message="$*"
  log "failure: $message"
  printf '%s %s\n' "$(timestamp)" "$message" >> "$ERROR_FILE"
  printf 'research: %s (details: %s)\n' "$message" "$ERROR_FILE" >&2
  exit 1
}

if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  log 'skipped: another run holds the lock (remove stale lock only after confirming no run exists)'
  exit 0
fi
trap 'rm -f "$LOCK_DIR/pid"; rmdir "$LOCK_DIR"' EXIT
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
fi

OUTPUT_FILE="$(mktemp "$LOG_DIR/opencode.XXXXXXXX")"
chmod 600 "$OUTPUT_FILE"
trap 'rm -f "$OUTPUT_FILE" "$LOCK_DIR/pid"; rmdir "$LOCK_DIR"' EXIT

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
git commit -m "knowledge: automated research update" >/dev/null 2>&1 || fail 'research commit failed'
SHA="$(git rev-parse HEAD)"
log "commit SHA: $SHA"
[[ -z "$(git status --porcelain --untracked-files=all)" ]] || fail 'working tree is dirty after commit'
