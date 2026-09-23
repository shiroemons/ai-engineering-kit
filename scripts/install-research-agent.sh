#!/bin/bash
set -Eeuo pipefail

HOUR=4
MINUTE=0
while (($#)); do
  case "$1" in
    --hour) HOUR="${2:?hour required}"; shift 2 ;;
    --minute) MINUTE="${2:?minute required}"; shift 2 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done
[[ "$HOUR" =~ ^[0-9]{1,2}$ && "$MINUTE" =~ ^[0-9]{1,2}$ ]] || exit 2
((10#$HOUR <= 23 && 10#$MINUTE <= 59)) || exit 2

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LABEL=com.shiroemons.ai-engineering-kit.research
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
mkdir -p "$HOME/Library/LaunchAgents" "$HOME/Library/Logs/ai-engineering-kit"
/usr/bin/python3 - "$PLIST" "$LABEL" "$ROOT/scripts/research-next.sh" "$HOUR" "$MINUTE" "$HOME/Library/Logs/ai-engineering-kit" <<'PY'
import plistlib
import sys

path, label, script, hour, minute, logs = sys.argv[1:]
data = {
    "Label": label,
    "ProgramArguments": [script],
    "WorkingDirectory": script.rsplit("/scripts/", 1)[0],
    "StartCalendarInterval": {"Hour": int(hour), "Minute": int(minute)},
    "StandardOutPath": logs + "/launchd.out.log",
    "StandardErrorPath": logs + "/launchd.err.log",
    "RunAtLoad": False,
    "EnvironmentVariables": {
        "PATH": "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
    },
}
with open(path, "wb") as output:
    plistlib.dump(data, output)
PY
launchctl bootout "gui/$(id -u)/$LABEL" >/dev/null 2>&1 || true
launchctl bootstrap "gui/$(id -u)" "$PLIST"
echo "installed $LABEL at $(printf '%02d:%02d' "$HOUR" "$MINUTE") local time"
