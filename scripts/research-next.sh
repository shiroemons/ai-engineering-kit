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
fail() { log "failure: $*"; printf '%s %s\n' "$(timestamp)" "$*" >> "$ERROR_FILE"; exit 1; }

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
[[ "$MODEL" == opencode/*-free ]] || fail 'configured model is not a recognized free-only OpenCode model'
log "selected model: $MODEL"

for command in git opencode curl jq just mise; do
  command -v "$command" >/dev/null || fail "missing command: $command"
done

# Both current availability and current zero input/output price must be confirmed.
AVAILABLE="$(opencode models --print-logs --log-level debug 2>/dev/null)" || fail 'OpenCode model listing failed'
printf '%s\n' "$AVAILABLE" | grep -Fxq "$MODEL" || fail 'configured model is unavailable'
PRICE_JSON="$(curl -fsSL --max-time 20 https://models.dev/api.json)" || fail 'current model pricing unavailable'
printf '%s\n' "$PRICE_JSON" | jq -e --arg id "${MODEL#opencode/}" \
  '.opencode.models[$id] | .cost.input == 0 and .cost.output == 0' >/dev/null \
  || fail 'configured model is not verified as free'

if [[ "${RESEARCH_DRY_RUN:-0}" != 1 ]]; then
  # A failed fetch or a non-fast-forward update stops before any research edits.
  GIT_TERMINAL_PROMPT=0 git pull --ff-only >/dev/null 2>&1 || fail 'git pull --ff-only failed'
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

if (cd "$ROOT" && opencode run --standalone --agent knowledge-researcher --model "$MODEL" "$PROMPT") > "$OUTPUT_FILE" 2>&1; then
  log 'OpenCode exit status: 0'
else
  status=$?
  log "OpenCode exit status: $status"
  fail 'OpenCode research failed; inspect its local session for details'
fi

TOPIC="$(sed -nE 's/.*TOPIC:[[:space:]]*//p' "$OUTPUT_FILE" | tail -n 1 | sed -E 's/\*\*$//' | tr -cd '[:print:]' | cut -c 1-120)"
[[ -n "$TOPIC" ]] || fail 'OpenCode did not report a selected topic'
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
