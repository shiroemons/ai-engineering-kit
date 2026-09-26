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
cp "$ROOT/scripts/research-next.sh" "$TEST_DIR/repo/scripts/"
cp "$ROOT/config/research.json" "$TEST_DIR/repo/config/"

cat > "$TEST_DIR/bin/git" <<'EOF'
#!/bin/bash
case "$1" in
  branch) printf 'main\n' ;;
  status) ;;
  worktree)
    if [[ "$2" == add ]]; then mkdir -p "$4"; fi
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
((count != 0)) || exit 1
printf '%s\n' opencode/muse-spark-1.3-contributor-free opencode/mimo-v2.6-flash-free opencode/paid-cache opencode/decisions-only
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
  printf '%s\n' '{"opencode":{"models":{"muse-spark-1.3-contributor-free":{"tool_call":true,"cost":{"input":0,"output":0,"cache_read":0}},"mimo-v2.6-flash-free":{"tool_call":true,"cost":{"input":0,"output":0}},"paid-cache":{"tool_call":true,"cost":{"input":0,"output":0,"cache_read":1}},"decisions-only":{"tool_call":false,"cost":{"input":0,"output":0}}}}}'
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
printf 'BATCH: complete\nTOPIC: test research\n'
EOF
printf '#!/bin/bash\nexit 0\n' > "$TEST_DIR/bin/sleep"
printf '#!/bin/bash\nexit 0\n' > "$TEST_DIR/bin/just"
chmod +x "$TEST_DIR/bin/"*
export PATH="$TEST_DIR/bin:$PATH"
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

# Scheduled and continuous executions have independent locks.
mkdir "$RESEARCH_LOG_DIR/research.lock"
RESEARCH_CONTINUOUS=1 bash "$RUNNER"
[[ "$(tail -n 1 "$TEST_DIR/batch-args")" == opencode/muse-spark-1.3-contributor-free ]]
! grep -Fq opencode/mimo-v2.6-flash-free "$TEST_DIR/batch-args"
rmdir "$RESEARCH_LOG_DIR/research.lock"
export RESEARCH_CONTINUOUS=1 MOCK_NO_MUSE=1
expect_failure 'no currently available OpenCode model'
unset RESEARCH_CONTINUOUS MOCK_NO_MUSE

export MOCK_BATCH_FAIL=1
expect_failure 'research batch failed'
grep -Fq 'worker failed: validation error' "$RESEARCH_LOG_DIR/research-error.log"
unset MOCK_BATCH_FAIL
printf '%s\n' "$(($(date +%s) + 3600))" > "$RESEARCH_LOG_DIR/cooldown-until"
status=0
bash "$RUNNER" || status=$?
[[ "$status" == 4 ]]
printf 'research discovery, Muse-only mode, independent locks, and cooldown: passed\n'
