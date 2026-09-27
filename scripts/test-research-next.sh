#!/bin/bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "$TEST_DIR"' EXIT
export RESEARCH_TEST_DIR="$TEST_DIR"
export RESEARCH_LOG_DIR="$TEST_DIR/logs"
export RESEARCH_ENV_FILE="$TEST_DIR/research.env"
export RESEARCH_DRY_RUN=1
mkdir -p "$TEST_DIR/bin" "$TEST_DIR/repo/scripts" "$TEST_DIR/repo/config"
: > "$TEST_DIR/worktrees"
: > "$TEST_DIR/branches"
cp "$ROOT/scripts/research-next.sh" "$TEST_DIR/repo/scripts/"
cp "$ROOT/config/research.json" "$TEST_DIR/repo/config/"

cat > "$TEST_DIR/bin/ps" <<'EOF'
#!/bin/bash
printf '%s\n' 'research-test-process-start'
EOF
cat > "$TEST_DIR/bin/git" <<'EOF'
#!/bin/bash
[[ "${PAGER:-}" == cat && "${GIT_PAGER:-}" == cat && "${GH_PAGER:-}" == cat ]] || exit 99
if [[ "$1" == -C && "$3" == pull ]]; then
  [[ "$2" == */repo ]] || exit 98
  touch "$RESEARCH_TEST_DIR/pulled-main"
  exit 0
fi
git_dir="$PWD"
if [[ "$1" == -C ]]; then
  git_dir="$2"
  shift 2
fi
case "$1" in
  branch)
    if [[ "$2" == --show-current ]]; then
      [[ "$git_dir" != */repo ]] || { printf 'main\n'; exit 0; }
      while IFS=$'\t' read -r path branch; do
        [[ "$path" != "$git_dir" ]] || { printf '%s\n' "$branch"; exit 0; }
      done < "$RESEARCH_TEST_DIR/worktrees"
      exit 1
    fi
    printf 'main\n'
    ;;
  status)
    if [[ "${MOCK_DIRTY:-0}" == 1 ]]; then
      printf '?? notes.md\n'
    elif [[ "${MOCK_FULL_RESEARCH:-0}" == 1 && -f "$PWD/knowledge/testing/go-test.md" && ! -f "$RESEARCH_TEST_DIR/committed" ]]; then
      case "$*" in
        *'-z'*) printf '?? knowledge/testing/go-test.md\0?? evals/knowledge/quality-operations.json\0' ;;
        *'-- evals/knowledge/'*) printf '?? evals/knowledge/quality-operations.json\n' ;;
        *'-- knowledge/'*) printf '?? knowledge/testing/go-test.md\n' ;;
        *) printf '?? knowledge/testing/go-test.md\n?? evals/knowledge/quality-operations.json\n' ;;
      esac
    fi
    ;;
  diff)
    if [[ "$*" == *'--cached --name-only'* ]]; then
      printf 'knowledge/testing/go-test.md\nevals/knowledge/quality-operations.json\n'
    elif [[ "$*" == *'--cached --stat'* ]]; then
      printf ' 2 files changed\n'
    elif [[ "$*" == *'--cached --quiet'* ]]; then
      exit 1
    fi
    ;;
  add) ;;
  commit) touch "$RESEARCH_TEST_DIR/committed" ;;
  rev-parse)
    if [[ "$2" == --show-toplevel ]]; then
      printf '%s\n' "$git_dir"
    else
      printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n'
    fi
    ;;
  show-ref)
    ref="${@: -1}"
    grep -Fqx "$ref" "$RESEARCH_TEST_DIR/branches"
    ;;
  update-ref)
    ref="$3"
    grep -Fvx "$ref" "$RESEARCH_TEST_DIR/branches" > "$RESEARCH_TEST_DIR/branches.tmp" || true
    mv "$RESEARCH_TEST_DIR/branches.tmp" "$RESEARCH_TEST_DIR/branches"
    ;;
  fetch|push) ;;
  worktree)
    [[ "${MOCK_NO_WORKTREE:-0}" != 1 ]] || exit 1
    case "$2" in
      list)
        printf 'worktree %s\n' "$RESEARCH_TEST_DIR/repo"
        while IFS=$'\t' read -r path branch; do
          [[ -n "$path" ]] && printf 'worktree %s\n' "$path"
        done < "$RESEARCH_TEST_DIR/worktrees"
        ;;
      add)
        if [[ "$3" == --detach ]]; then
          worktree_path="$4"
          branch=''
        else
          branch="$4"
          worktree_path="$5"
          printf 'refs/heads/%s\n' "$branch" >> "$RESEARCH_TEST_DIR/branches"
        fi
        mkdir -p "$worktree_path"
        printf '%s\t%s\n' "$worktree_path" "$branch" >> "$RESEARCH_TEST_DIR/worktrees"
        ;;
      remove)
        [[ "$3" == --force ]] && worktree_path="$4" || worktree_path="$3"
        rm -rf "$worktree_path"
        while IFS=$'\t' read -r path branch; do
          [[ "$path" == "$worktree_path" ]] || printf '%s\t%s\n' "$path" "$branch"
        done < "$RESEARCH_TEST_DIR/worktrees" > "$RESEARCH_TEST_DIR/worktrees.tmp"
        mv "$RESEARCH_TEST_DIR/worktrees.tmp" "$RESEARCH_TEST_DIR/worktrees"
        ;;
      *) exit 1 ;;
    esac
    ;;
  *) exit 1 ;;
esac
EOF
cat > "$TEST_DIR/bin/opencode" <<'EOF'
#!/bin/bash
[[ "$1" == models ]] || exit 1
count=0
[[ ! -f "$RESEARCH_TEST_DIR/list-count" ]] || read -r count < "$RESEARCH_TEST_DIR/list-count"
printf '%s\n' "$((count + 1))" > "$RESEARCH_TEST_DIR/list-count"
[[ "${MOCK_EMPTY_MODELS:-0}" != 1 ]] || exit 0
if [[ "${MOCK_COLD_MODELS:-0}" == 1 && "$count" == 0 ]]; then exit 0; fi
((count != 0)) || exit 1
printf '%s\n' opencode/muse-spark-1.3-contributor-free opencode/mimo-v2.6-flash-free opencode/third-free opencode/paid-cache opencode/decisions-only
EOF
cat > "$TEST_DIR/bin/curl" <<'EOF'
#!/bin/bash
if [[ ! -f "$RESEARCH_TEST_DIR/pricing-ready" ]]; then
  touch "$RESEARCH_TEST_DIR/pricing-ready"
  exit 22
fi
if [[ "${MOCK_NO_MUSE:-0}" == 1 ]]; then
  printf '%s\n' '{"opencode":{"models":{"mimo-v2.6-flash-free":{"tool_call":true,"cost":{"input":0,"output":0}}}}}'
else
  printf '%s\n' '{"opencode":{"models":{"muse-spark-1.3-contributor-free":{"tool_call":true,"cost":{"input":0,"output":0,"cache_read":0}},"mimo-v2.6-flash-free":{"tool_call":true,"cost":{"input":0,"output":0}},"third-free":{"tool_call":true,"cost":{"input":0,"output":0}},"paid-cache":{"tool_call":true,"cost":{"input":0,"output":0,"cache_read":1}},"decisions-only":{"tool_call":false,"cost":{"input":0,"output":0}}}}}'
fi
EOF
cat > "$TEST_DIR/bin/mise" <<'EOF'
#!/bin/bash
if [[ "$*" == 'exec -- go build '* ]]; then
  mkdir -p bin
  cp "$RESEARCH_TEST_DIR/coordinator" bin/research-batch
  chmod +x bin/research-batch
fi
EOF
cat > "$TEST_DIR/coordinator" <<'EOF'
#!/bin/bash
printf '%s\n' "$@" > "$RESEARCH_TEST_DIR/batch-args"
if [[ "${MOCK_BATCH_FAIL:-0}" == 1 ]]; then
  printf 'worker failed: validation error; recovery=worker-worktree\n'
  exit 1
fi
if [[ "${MOCK_BATCH_RATE_LIMIT:-0}" == 1 ]]; then
  printf 'RATE_LIMIT: opencode/muse-spark-1.3-contributor-free; all workers stopped\n'
  printf '%s\n' "$(($(date +%s) + 3600))" > "$RESEARCH_LOG_DIR/cooldown-until"
  exit 6
fi
if [[ "${MOCK_FULL_RESEARCH:-0}" == 1 ]]; then
  mkdir -p knowledge/testing evals/knowledge
  : > knowledge/testing/go-test.md
  : > evals/knowledge/quality-operations.json
fi
printf 'BATCH: complete\nTOPIC: test research\n'
EOF
cat > "$TEST_DIR/bin/gh" <<'EOF'
#!/bin/bash
case "$1:$2" in
  auth:status) ;;
  pr:create) printf 'https://example.test/owner/repo/pull/1\n' ;;
  pr:view)
    case "$*" in
      *'--json mergeable,mergeStateStatus'*) printf 'MERGEABLE\tCLEAN\n' ;;
      *'--json state'*) printf 'MERGED\n' ;;
      *'--json url'*) printf 'https://example.test/owner/repo/pull/1\n' ;;
      *) exit 1 ;;
    esac
    ;;
  pr:merge) ;;
  *) exit 1 ;;
esac
EOF
cat > "$TEST_DIR/bin/sleep" <<'EOF'
#!/bin/bash
if [[ "${MOCK_RELEASE_PIPELINE_LOCK:-0}" == 1 && ! -f "$RESEARCH_TEST_DIR/pipeline-lock-released" ]]; then
  touch "$RESEARCH_TEST_DIR/pipeline-lock-released"
  rm -f "$RESEARCH_LOG_DIR/research-pipeline.lock/pid" "$RESEARCH_LOG_DIR/research-pipeline.lock/pid_start"
  rmdir "$RESEARCH_LOG_DIR/research-pipeline.lock"
fi
exit 0
EOF
printf '#!/bin/bash\nexit 0\n' > "$TEST_DIR/bin/just"
chmod +x "$TEST_DIR/bin/"*
export PATH="$TEST_DIR/bin:$PATH"
export PAGER=less GIT_PAGER=less GH_PAGER=less
RUNNER="$TEST_DIR/repo/scripts/research-next.sh"

expect_failure() {
  local expected="$1" output
  if output="$(bash "$RUNNER" 2>&1)"; then
    printf 'expected failure: %s\n' "$expected" >&2; exit 1
  fi
  [[ "$output" == *"$expected"* ]] || { printf '%s\n' "$output" >&2; exit 1; }
}
expect_failure 'research.env is missing'
printf 'OPENCODE_RESEARCH_MODEL=opencode/mimo-v2.6-flash-free\n' > "$RESEARCH_ENV_FILE"
bash "$RUNNER"
grep -Fq 'OpenCode model listing failed; retrying' "$RESEARCH_LOG_DIR/research.log"
grep -Fq 'current model pricing unavailable; retrying' "$RESEARCH_LOG_DIR/research.log"
[[ "$(tail -n 2 "$TEST_DIR/batch-args")" == $'opencode/muse-spark-1.3-contributor-free\nopencode/mimo-v2.6-flash-free' ]]
! grep -Eq 'paid-cache|decisions-only' "$TEST_DIR/batch-args"
! grep -Fq 'opencode/third-free' "$TEST_DIR/batch-args"

# A successful CLI exit can precede initial model catalog settlement.
rm "$TEST_DIR/list-count"
MOCK_COLD_MODELS=1 bash "$RUNNER"
grep -Fq 'catalog may still be initializing' "$RESEARCH_LOG_DIR/research.log"
[[ "$(< "$TEST_DIR/list-count")" == 2 ]]

# Preflight never starts a research coordinator or creates a worktree.
rm "$TEST_DIR/batch-args"
RESEARCH_PREFLIGHT=1 MOCK_NO_WORKTREE=1 bash "$RUNNER"
[[ ! -f "$TEST_DIR/batch-args" ]]
grep -Fq 'preflight: passed' "$RESEARCH_LOG_DIR/research.log"
export MOCK_DIRTY=1
expect_failure 'working tree is dirty'
grep -Fq '?? notes.md' "$RESEARCH_LOG_DIR/research.log"
unset MOCK_DIRTY
export MOCK_EMPTY_MODELS=1
expect_failure 'OpenCode model listing failed after 3 attempts'
[[ ! -f "$TEST_DIR/batch-args" ]]
unset MOCK_EMPTY_MODELS

# Continuous execution starts with its configured model and fills the 3-worker limit.
rm -f "$TEST_DIR/batch-args"
RESEARCH_CONTINUOUS=1 bash "$RUNNER"
[[ "$(tail -n 3 "$TEST_DIR/batch-args")" == $'opencode/muse-spark-1.3-contributor-free\nopencode/mimo-v2.6-flash-free\nopencode/third-free' ]]
[[ ! -d "$RESEARCH_LOG_DIR/research-pipeline.lock" ]]

# Scheduled and continuous pipelines share a lock through post-processing.
rm "$TEST_DIR/batch-args"
mkdir "$RESEARCH_LOG_DIR/research-pipeline.lock"
printf '%s\n' "$$" > "$RESEARCH_LOG_DIR/research-pipeline.lock/pid"
printf '%s\n' 'research-test-process-start' > "$RESEARCH_LOG_DIR/research-pipeline.lock/pid_start"
status=0
RESEARCH_CONTINUOUS=1 bash "$RUNNER" > "$TEST_DIR/pipeline-lock-output" 2>&1 || status=$?
[[ "$status" == 3 ]]
grep -Fq 'another research pipeline holds the shared lock' "$TEST_DIR/pipeline-lock-output"
[[ ! -f "$TEST_DIR/batch-args" ]]
[[ ! -d "$RESEARCH_LOG_DIR/research-continuous.lock" ]]
rm -f "$RESEARCH_LOG_DIR/research-pipeline.lock/pid" "$RESEARCH_LOG_DIR/research-pipeline.lock/pid_start"
rmdir "$RESEARCH_LOG_DIR/research-pipeline.lock"

# A scheduled run waits for an active continuous pipeline instead of being lost.
rm -f "$TEST_DIR/batch-args"
mkdir "$RESEARCH_LOG_DIR/research-pipeline.lock"
printf '%s\n' "$$" > "$RESEARCH_LOG_DIR/research-pipeline.lock/pid"
printf '%s\n' 'research-test-process-start' > "$RESEARCH_LOG_DIR/research-pipeline.lock/pid_start"
RESEARCH_CONTINUOUS=0 MOCK_RELEASE_PIPELINE_LOCK=1 bash "$RUNNER"
[[ -f "$TEST_DIR/pipeline-lock-released" ]]
[[ -f "$TEST_DIR/batch-args" ]]
[[ ! -d "$RESEARCH_LOG_DIR/research-pipeline.lock" ]]

export RESEARCH_CONTINUOUS=1 MOCK_NO_MUSE=1
expect_failure 'no currently available OpenCode model'
unset RESEARCH_CONTINUOUS MOCK_NO_MUSE

export MOCK_BATCH_FAIL=1
expect_failure 'research batch failed'
grep -Fq 'worker failed: validation error' "$RESEARCH_LOG_DIR/research-error.log"
unset MOCK_BATCH_FAIL
printf '%s\n' "$(($(date +%s) + 3600))" > "$RESEARCH_LOG_DIR/cooldown-until"
status=0
bash "$RUNNER" > "$TEST_DIR/cooldown-output" 2>&1 || status=$?
[[ "$status" == 0 ]]
grep -Fq 'provider rate limit cooldown is active; no provider work started' "$TEST_DIR/cooldown-output"
rm "$RESEARCH_LOG_DIR/cooldown-until"

# A completed auto-merge fast-forwards the user's original main checkout.
MOCK_FULL_RESEARCH=1 RESEARCH_DRY_RUN=0 bash "$RUNNER"
[[ -f "$RESEARCH_TEST_DIR/pulled-main" ]]
grep -Fq 'pulled merged origin/main into original checkout' "$RESEARCH_LOG_DIR/research.log"

# A Muse Spark rate limit is recorded without counting as a batch failure.
rm -f "$RESEARCH_TEST_DIR/committed"
error_size_before="$(wc -c < "$RESEARCH_LOG_DIR/research-error.log")"
MOCK_BATCH_RATE_LIMIT=1 RESEARCH_DRY_RUN=0 bash "$RUNNER" > "$TEST_DIR/rate-limit-output" 2>&1
grep -Fq 'rate limit detected; stopped without counting a failure' "$TEST_DIR/rate-limit-output"
error_size_after="$(wc -c < "$RESEARCH_LOG_DIR/research-error.log")"
[[ "$error_size_after" == "$error_size_before" ]]
rate_limited_run_file="$(find "$RESEARCH_LOG_DIR/research-runs" -name run.json -type f -print -quit)"
[[ -n "$rate_limited_run_file" ]]
[[ "$(jq -r '.stop_reason' "$rate_limited_run_file")" == muse-spark-rate-limit ]]
rate_limited_run_id="$(jq -r '.id' "$rate_limited_run_file")"

# The active cooldown exits cleanly, then the next invocation cleans up and starts fresh.
status=0
bash "$RUNNER" > "$TEST_DIR/rate-limit-cooldown-output" 2>&1 || status=$?
[[ "$status" == 0 ]]
grep -Fq 'provider rate limit cooldown is active' "$TEST_DIR/rate-limit-cooldown-output"
printf '0\n' > "$RESEARCH_LOG_DIR/cooldown-until"
unset MOCK_BATCH_RATE_LIMIT
MOCK_FULL_RESEARCH=1 RESEARCH_DRY_RUN=0 bash "$RUNNER" > "$TEST_DIR/rate-limit-recovery-output" 2>&1
grep -Fq 'cleaned the stopped Muse Spark run and starting fresh' "$TEST_DIR/rate-limit-recovery-output"
[[ ! -e "$RESEARCH_LOG_DIR/research-runs/$rate_limited_run_id" ]]
[[ -f "$RESEARCH_TEST_DIR/pulled-main" ]]
printf 'research discovery, pager isolation, auto-merge sync, cooldown, and Muse Spark rate-limit recovery: passed\n'
