#!/bin/bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
HOUR=''
MINUTE=''
INTERVAL_HOURS=''
OUTPUT=''
fail() { printf 'research-install: %s\n' "$*" >&2; exit 2; }
while (($#)); do
  case "$1" in
    --hour|--minute|--interval-hours|--output)
      (($# >= 2)) && [[ -n "$2" && "$2" != --* ]] || fail "value required for $1"
      case "$1" in
        --hour) HOUR="$2" ;;
        --minute) MINUTE="$2" ;;
        --interval-hours) INTERVAL_HOURS="$2" ;;
        --output) OUTPUT="$2" ;;
      esac
      shift 2
      ;;
    *) fail "unknown option: $1" ;;
  esac
done
[[ -z "$HOUR" || -z "$INTERVAL_HOURS" ]] || fail '--hour and --interval-hours are mutually exclusive'

for command in jq plutil; do
  command -v "$command" >/dev/null || fail "missing command: $command"
done
SCHEDULE="$(jq -er '.schedule
  | select((.interval_hours | type) == "number" and (.minute | type) == "number")
  | [.interval_hours, .minute] | @tsv' "$ROOT/config/research.json")" \
  || fail 'config/research.json must define a numeric schedule.interval_hours and schedule.minute'
IFS=$'\t' read -r CONFIG_INTERVAL CONFIG_MINUTE <<< "$SCHEDULE"
MINUTE="${MINUTE:-$CONFIG_MINUTE}"
[[ "$MINUTE" =~ ^[0-9]{1,2}$ ]] || fail 'minute must be an integer from 0 to 59'
MINUTE=$((10#$MINUTE))
((MINUTE <= 59)) || fail 'minute must be an integer from 0 to 59'

if [[ -n "$HOUR" ]]; then
  [[ "$HOUR" =~ ^[0-9]{1,2}$ ]] || fail 'hour must be an integer from 0 to 23'
  HOUR=$((10#$HOUR))
  ((HOUR <= 23)) || fail 'hour must be an integer from 0 to 23'
  CALENDAR="$(jq -n --argjson hour "$HOUR" --argjson minute "$MINUTE" '{Hour: $hour, Minute: $minute}')"
else
  INTERVAL_HOURS="${INTERVAL_HOURS:-$CONFIG_INTERVAL}"
  [[ "$INTERVAL_HOURS" =~ ^[0-9]{1,2}$ ]] || fail 'interval hours must be a positive divisor of 24'
  INTERVAL_HOURS=$((10#$INTERVAL_HOURS))
  ((INTERVAL_HOURS >= 1 && INTERVAL_HOURS <= 24)) || fail 'interval hours must be a positive divisor of 24'
  ((24 % INTERVAL_HOURS == 0)) || fail 'interval hours must be a positive divisor of 24'
  CALENDAR="$(jq -n --argjson interval "$INTERVAL_HOURS" --argjson minute "$MINUTE" \
    '[range(0; 24; $interval) | {Hour: ., Minute: $minute}]')"
fi

LABEL=com.shiroemons.ai-engineering-kit.research
PLIST="${OUTPUT:-$HOME/Library/LaunchAgents/$LABEL.plist}"
LOG_DIR="$HOME/Library/Logs/ai-engineering-kit"
if [[ -z "$OUTPUT" ]]; then
  # Replacing a running agent would terminate research and leave its lock behind.
  [[ ! -d "$LOG_DIR/research.lock" ]] || fail 'research lock exists; wait for the active run or inspect the stale lock'
  mkdir -p "$HOME/Library/LaunchAgents" "$LOG_DIR"
fi
TEMP_PLIST="$(mktemp "$PLIST.XXXXXXXX")"
trap 'rm -f "$TEMP_PLIST"' EXIT
jq -n --arg label "$LABEL" --arg root "$ROOT" --arg logs "$LOG_DIR" \
  --arg task_home "$HOME" --argjson calendar "$CALENDAR" '{
    Label: $label,
    ProgramArguments: [($root + "/scripts/research-next.sh")],
    WorkingDirectory: $root,
    StartCalendarInterval: $calendar,
    StandardOutPath: ($logs + "/launchd.out.log"),
    StandardErrorPath: ($logs + "/launchd.err.log"),
    RunAtLoad: false,
    EnvironmentVariables: {
      PATH: ($task_home + "/.local/share/mise/shims:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin")
    }
  }' | plutil -convert xml1 -o "$TEMP_PLIST" -
plutil -lint "$TEMP_PLIST" >/dev/null
chmod 644 "$TEMP_PLIST"
if [[ -z "$OUTPUT" ]]; then
  SERVICE="gui/$(id -u)/$LABEL"
  # Acquire the runner's lock through reload to avoid interrupting a scheduled run.
  mkdir "$LOG_DIR/research.lock" || fail 'research started during installation; retry after it finishes'
  trap 'rm -f "$TEMP_PLIST"; rmdir "$LOG_DIR/research.lock"' EXIT
  if launchctl print "$SERVICE" >/dev/null 2>&1; then
    launchctl bootout "$SERVICE"
  fi
fi
mv "$TEMP_PLIST" "$PLIST"
if [[ -n "$OUTPUT" ]]; then
  printf 'generated %s; launchd unchanged\n' "$PLIST"
else
  launchctl bootstrap "gui/$(id -u)" "$PLIST"
  printf 'installed %s\n' "$LABEL"
fi
printf 'local schedule: %s\n' "$(jq -r '[. | if type == "array" then .[] else . end
  | (("0" + (.Hour | tostring))[-2:] + ":" + ("0" + (.Minute | tostring))[-2:])] | join(", ")' <<< "$CALENDAR")"
