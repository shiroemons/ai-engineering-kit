#!/bin/bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "$TEST_DIR"' EXIT
export RESEARCH_TEST_DIR="$TEST_DIR"
export RESEARCH_TEST_TIME_STEP=1000
export RESEARCH_TEST_TOPIC='testing — go testのキャッシュ判定条件とt.Runのシャッフル、並列実行'
export RESEARCH_LOG_DIR="$TEST_DIR/logs"
mkdir -p "$TEST_DIR/bin" "$TEST_DIR/repo/scripts" "$TEST_DIR/repo/config"
cp "$ROOT/scripts/research-loop.sh" "$TEST_DIR/repo/scripts/"
jq '.continuous.hours = 1 | .continuous.pause_seconds = 1' "$ROOT/config/research.json" > "$TEST_DIR/repo/config/research.json"
cat > "$TEST_DIR/bin/date" <<'EOF'
#!/bin/bash
if [[ "$1" == +%s ]]; then
  count=0
  [[ ! -f "$RESEARCH_TEST_DIR/time" ]] || read -r count < "$RESEARCH_TEST_DIR/time"
  count=$((count + RESEARCH_TEST_TIME_STEP))
  printf '%s\n' "$count" > "$RESEARCH_TEST_DIR/time"
  printf '%s\n' "$count"
else
  printf 'test-time\n'
fi
EOF
printf '#!/bin/bash\nexit 0\n' > "$TEST_DIR/bin/sleep"
cat > "$TEST_DIR/repo/scripts/research-next.sh" <<'EOF'
#!/bin/bash
[[ "${PAGER:-}" == cat && "${GIT_PAGER:-}" == cat && "${GH_PAGER:-}" == cat ]] || exit 10
expected_deadline=4600
[[ "$RESEARCH_TEST_TIME_STEP" != 300 ]] || expected_deadline=3900
[[ "$RESEARCH_CONTINUOUS" == 1 && "$RESEARCH_DEADLINE" == "$expected_deadline" ]] || exit 8
if [[ -e "$RESEARCH_PROGRESS_FILE" ]]; then
  touch "$RESEARCH_TEST_DIR/stale-progress"
  exit 9
fi
printf '{"percent":92,"topic":"%s"}\n' "$RESEARCH_TEST_TOPIC" > "$RESEARCH_PROGRESS_FILE"
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
export PAGER=less GIT_PAGER=less GH_PAGER=less
LOOP="$TEST_DIR/repo/scripts/research-loop.sh"
! grep -Fq 'monitor_progress' "$ROOT/scripts/research-next.sh"
bash "$LOOP" > "$TEST_DIR/output"
[[ "$(wc -l < "$TEST_DIR/runs" | tr -d ' ')" == 2 ]]
[[ ! -e "$TEST_DIR/stale-progress" ]]
[[ "$(grep -c '100%' "$TEST_DIR/output")" == 2 ]]
grep -Fq "$RESEARCH_TEST_TOPIC" "$TEST_DIR/output"
! grep -Fq '92%' "$TEST_DIR/output"
grep -Fq 'time limit reached' "$TEST_DIR/output"
[[ ! -d "$RESEARCH_LOG_DIR/research-loop.lock" ]]

if [[ "$(uname -s)" == Darwin ]] && command -v script >/dev/null; then
  rm "$TEST_DIR/time" "$TEST_DIR/runs"
  TERM=xterm-256color script -q /dev/null bash "$LOOP" > "$TEST_DIR/tty-output"
  grep -Fq "$RESEARCH_TEST_TOPIC" "$TEST_DIR/tty-output"
  grep -Fq $'\033[u\033[J' "$TEST_DIR/tty-output"
fi

rm "$TEST_DIR/time" "$TEST_DIR/runs"
RESEARCH_TEST_TIME_STEP=300 MOCK_LOOP_STATUS=1 bash "$LOOP" > "$TEST_DIR/output" 2>&1
[[ "$(wc -l < "$TEST_DIR/runs" | tr -d ' ')" -ge 4 ]]
grep -Fq 'retrying in 1s' "$TEST_DIR/output"
! grep -Fq 'stopped after 3 consecutive' "$TEST_DIR/output"
grep -Fq 'time limit reached' "$TEST_DIR/output"

rm "$TEST_DIR/time" "$TEST_DIR/runs"
MOCK_LOOP_STATUS=4 bash "$LOOP" > "$TEST_DIR/output"
[[ "$(wc -l < "$TEST_DIR/runs" | tr -d ' ')" == 1 ]]
grep -Fq 'provider cooldown active; stopped without counting a failure' "$TEST_DIR/output"

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
printf 'continuous loop deadline, failure, cooldown stop, and child stop: passed\n'
