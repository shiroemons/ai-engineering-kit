#!/bin/bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd -P)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "$TEST_DIR"' EXIT
PLIST="$TEST_DIR/research agent & schedule.plist"

fail() { printf 'schedule test: %s\n' "$*" >&2; exit 1; }
generate() { bash "$ROOT/scripts/install-research-agent.sh" --output "$PLIST" "$@" >/dev/null; }
assert_plist() {
  plutil -lint "$PLIST" >/dev/null
  plutil -convert json -o - "$PLIST" | jq -e "$1" >/dev/null || fail "$1"
}
reject() {
  local output
  if output="$(generate "$@" 2>&1)"; then
    fail "unexpected success: $*"
  fi
  [[ "$output" == *'research-install:'* ]] || fail "missing diagnostic for $*"
  cmp -s "$PLIST" "$TEST_DIR/original.plist" || fail 'invalid input changed the existing plist'
}

generate
assert_plist '.StartCalendarInterval == [range(0; 24) | {Hour: ., Minute: 0}]'
assert_plist '.RunAtLoad == false and (.ProgramArguments | length) == 1
  and .ProgramArguments[0] == (.WorkingDirectory + "/scripts/research-next.sh")
  and (.EnvironmentVariables.PATH | contains("/.local/share/mise/shims:"))'

generate --interval-hours 2 --minute 05
assert_plist '.StartCalendarInterval == [range(0; 24; 2) | {Hour: ., Minute: 5}]'
generate --interval-hours 1
assert_plist '(.StartCalendarInterval | length) == 24'
generate --interval-hours 24 --minute 59
assert_plist '.StartCalendarInterval == [{Hour: 0, Minute: 59}]'
generate --hour 04 --minute 00
assert_plist '.StartCalendarInterval == {Hour: 4, Minute: 0}'
generate --hour 23 --minute 59
assert_plist '.StartCalendarInterval == {Hour: 23, Minute: 59}'
cp "$PLIST" "$TEST_DIR/original.plist"

reject --interval-hours 0
reject --interval-hours 5
reject --interval-hours 25
reject --interval-hours -3
reject --interval-hours 1.5
reject --interval-hours invalid
reject --hour 24
reject --minute 60
reject --minute invalid
reject --hour 4 --interval-hours 3
reject --interval-hours
reject --output ''
reject --unknown

printf 'research schedule generation, boundaries, and rejected input: passed\n'
