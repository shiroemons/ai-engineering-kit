#!/bin/bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
TEST_HOME="$(mktemp -d)"
trap 'rm -rf "$TEST_HOME"' EXIT
mkdir -p "$TEST_HOME/bin"

cat > "$TEST_HOME/bin/git" <<'EOF'
#!/bin/bash
case "$1" in
  branch) printf 'main\n' ;;
  status)
    if [[ -f "$HOME/partial-change" ]]; then
      printf ' M knowledge/partial.md\n'
    fi
    ;;
  pull) exit 0 ;;
  *) exit 1 ;;
esac
EOF

cat > "$TEST_HOME/bin/opencode" <<'EOF'
#!/bin/bash
case "$1" in
  models)
    count=0
    [[ -f "$HOME/model-list-count" ]] && read -r count < "$HOME/model-list-count"
    count=$((count + 1))
    printf '%s\n' "$count" > "$HOME/model-list-count"
    if [[ "$count" -eq 1 ]]; then
      exit 1
    elif [[ "$count" -eq 2 ]]; then
      printf 'opencode/paid-model\n'
      exit 0
    fi
    printf 'opencode/fallback-free\n'
    ;;
  run)
    count=0
    [[ -f "$HOME/research-run-count" ]] && read -r count < "$HOME/research-run-count"
    count=$((count + 1))
    printf '%s\n' "$count" > "$HOME/research-run-count"
    if [[ "$MOCK_PARTIAL" == 1 ]]; then
      : > "$HOME/partial-change"
      exit 1
    fi
    if [[ "$count" -eq 1 ]]; then
      exit 1
    fi
    printf 'TOPIC: fallback model recovery\n'
    ;;
  *) exit 1 ;;
esac
EOF

cat > "$TEST_HOME/bin/curl" <<'EOF'
#!/bin/bash
count=0
[[ -f "$HOME/pricing-fetch-count" ]] && read -r count < "$HOME/pricing-fetch-count"
count=$((count + 1))
printf '%s\n' "$count" > "$HOME/pricing-fetch-count"
if [[ "$count" -eq 1 ]]; then
  exit 22
fi
printf '%s\n' '{"opencode":{"models":{"fallback-free":{"cost":{"input":0,"output":0}}}}}'
EOF

cat > "$TEST_HOME/bin/jq" <<'EOF'
#!/bin/bash
case "$1" in
  -e) printf 'true\n' ;;
  -r) printf 'fallback-free\n' ;;
  *) exit 1 ;;
esac
EOF

cat > "$TEST_HOME/bin/sleep" <<'EOF'
#!/bin/bash
exit 0
EOF

cat > "$TEST_HOME/bin/just" <<'EOF'
#!/bin/bash
exit 0
EOF

cat > "$TEST_HOME/bin/mise" <<'EOF'
#!/bin/bash
exit 0
EOF

chmod +x "$TEST_HOME/bin/"*

run_expect_failure() {
  local expected="$1" output
  if output="$(HOME="$TEST_HOME" PATH="$TEST_HOME/bin:$PATH" bash "$ROOT/scripts/research-next.sh" 2>&1)"; then
    printf 'expected research to fail: %s\n' "$expected" >&2
    exit 1
  fi
  [[ "$output" == *"$expected"* ]] || {
    printf 'missing error in terminal output: %s\n%s\n' "$expected" "$output" >&2
    exit 1
  }
  [[ "$output" == *"research-error.log"* ]] || {
    printf 'terminal output does not identify the error log\n' >&2
    exit 1
  }
  grep -Fq "$expected" "$TEST_HOME/Library/Logs/ai-engineering-kit/research-error.log"
}

run_expect_failure 'research.env is missing'

mkdir -p "$TEST_HOME/Library/Application Support/ai-engineering-kit"
printf 'OPENCODE_RESEARCH_MODEL=opencode/mimo-v2.6-flash-free\n' \
  > "$TEST_HOME/Library/Application Support/ai-engineering-kit/research.env"

output="$(HOME="$TEST_HOME" RESEARCH_DRY_RUN=1 PATH="$TEST_HOME/bin:$PATH" \
  bash "$ROOT/scripts/research-next.sh" 2>&1)" || {
  printf 'expected transient failures to recover:\n%s\n' "$output" >&2
  exit 1
}
LOG_FILE="$TEST_HOME/Library/Logs/ai-engineering-kit/research.log"
grep -Fq 'OpenCode model listing failed; retrying' "$LOG_FILE"
grep -Fq 'current model pricing unavailable; retrying' "$LOG_FILE"
grep -Fq 'configured model is unavailable or not currently free: opencode/mimo-v2.6-flash-free' "$LOG_FILE"
grep -Fq 'no currently available zero-priced model; retrying model discovery' "$LOG_FILE"
grep -Fq 'research attempt: model=opencode/fallback-free attempt=2/2' "$LOG_FILE"
grep -Fq 'selected topic: fallback model recovery' "$LOG_FILE"

rm -f "$TEST_HOME/model-list-count" "$TEST_HOME/pricing-fetch-count" "$TEST_HOME/research-run-count"
if output="$(HOME="$TEST_HOME" RESEARCH_DRY_RUN=1 MOCK_PARTIAL=1 PATH="$TEST_HOME/bin:$PATH" \
  bash "$ROOT/scripts/research-next.sh" 2>&1)"; then
  printf 'expected partial edits to stop automatic retry\n' >&2
  exit 1
fi
[[ "$output" == *'automatic retry stopped to avoid duplicate edits'* ]]
[[ "$(cat "$TEST_HOME/research-run-count")" == 1 ]]

printf 'research retry, fallback, and partial-edit handling: passed\n'
