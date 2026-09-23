#!/bin/bash
set -Eeuo pipefail
LABEL=com.shiroemons.ai-engineering-kit.research
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
launchctl bootout "gui/$(id -u)/$LABEL" >/dev/null 2>&1 || true
rm -f "$PLIST"
echo "uninstalled $LABEL"
