#!/bin/bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEST_HOME="$(mktemp -d)"
trap 'rm -rf "$TEST_HOME"' EXIT
mkdir -p "$TEST_HOME/bin"

cat > "$TEST_HOME/bin/git" <<'EOF'
#!/bin/bash
case "$1" in
  branch) printf 'main\n' ;;
  status) ;;
  *) exit 1 ;;
esac
EOF
cat > "$TEST_HOME/bin/opencode" <<'EOF'
#!/bin/bash
[[ "$1" == models ]] || exit 1
printf 'opencode/other-free\n'
EOF
chmod +x "$TEST_HOME/bin/git" "$TEST_HOME/bin/opencode"

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
run_expect_failure 'configured model is unavailable: opencode/mimo-v2.6-flash-free'

printf 'research error reporting: passed\n'
