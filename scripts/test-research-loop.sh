#!/bin/bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "$TEST_DIR"' EXIT
export RESEARCH_TEST_DIR="$TEST_DIR"
export RESEARCH_LOG_DIR="$TEST_DIR/logs"
mkdir -p "$TEST_DIR/bin" "$TEST_DIR/repo/scripts" "$TEST_DIR/repo/config"
cp "$ROOT/scripts/research-loop.sh" "$TEST_DIR/repo/scripts/"
jq '.continuous.hours = 1 | .continuous.pause_seconds = 1' "$ROOT/config/research.json" > "$TEST_DIR/repo/config/research.json"
cat > "$TEST_DIR/bin/date" <<'EOF'
#!/bin/bash
if [[ "$1" == +%s ]]; then
  count=0
  [[ ! -f "$RESEARCH_TEST_DIR/time" ]] || read -r count < "$RESEARCH_TEST_DIR/time"
  count=$((count + 1000))
  printf '%s\n' "$count" > "$RESEARCH_TEST_DIR/time"
  printf '%s\n' "$count"
else
  printf 'test-time\n'
fi
EOF
printf '#!/bin/bash\nexit 0\n' > "$TEST_DIR/bin/sleep"
cat > "$TEST_DIR/repo/scripts/research-next.sh" <<'EOF'
#!/bin/bash
[[ "$RESEARCH_CONTINUOUS" == 1 && "$RESEARCH_DEADLINE" == 4600 ]] || exit 8
printf 'run\n' >> "$RESEARCH_TEST_DIR/runs"
if [[ "${MOCK_LOOP_STATUS:-0}" == stop ]]; then
  trap 'touch "$RESEARCH_TEST_DIR/stopped"; exit 143' TERM
  touch "$RESEARCH_TEST_DIR/ready"
  while :; do /bin/sleep 0.05; done
fi
exit "${MOCK_LOOP_STATUS:-0}"
EOF
chmod +x "$TEST_DIR/bin/"*
export PATH="$TEST_DIR/bin:$PATH"
LOOP="$TEST_DIR/repo/scripts/research-loop.sh"
bash "$LOOP" > "$TEST_DIR/output"
[[ "$(wc -l < "$TEST_DIR/runs" | tr -d ' ')" == 2 ]]
grep -Fq 'time limit reached' "$TEST_DIR/output"
[[ ! -d "$RESEARCH_LOG_DIR/research-loop.lock" ]]

rm "$TEST_DIR/time" "$TEST_DIR/runs"
MOCK_LOOP_STATUS=1 bash "$LOOP" > "$TEST_DIR/output" 2>&1
[[ "$(wc -l < "$TEST_DIR/runs" | tr -d ' ')" == 2 ]]
grep -Fq 'retrying in 1s' "$TEST_DIR/output"
grep -Fq 'time limit reached' "$TEST_DIR/output"

rm "$TEST_DIR/time" "$TEST_DIR/runs"
MOCK_LOOP_STATUS=4 bash "$LOOP" > "$TEST_DIR/output"
[[ "$(wc -l < "$TEST_DIR/runs" | tr -d ' ')" == 2 ]]
grep -Fq 'provider cooldown active; waiting to retry' "$TEST_DIR/output"

rm "$TEST_DIR/time" "$TEST_DIR/runs"
MOCK_LOOP_STATUS=stop bash "$LOOP" > "$TEST_DIR/output" 2>&1 &
loop_pid=$!
for attempt in {1..100}; do
  [[ ! -f "$TEST_DIR/ready" ]] || break
  /bin/sleep 0.05
done
[[ -f "$TEST_DIR/ready" ]]
kill -TERM "$loop_pid"
wait "$loop_pid" || status=$?
[[ "$status" == 143 && -f "$TEST_DIR/stopped" ]]
[[ ! -d "$RESEARCH_LOG_DIR/research-loop.lock" ]]
printf 'continuous loop deadline, failure, cooldown, and child stop: passed\n'
