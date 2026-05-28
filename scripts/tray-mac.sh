#!/usr/bin/env bash
# flow_router — macOS launcher (CGO-free).
# Real menu-bar items on macOS need a CGO/Cocoa binary; this script provides
# the same control surface via osascript notifications + `open`.
#
# Usage:
#   scripts/tray-mac.sh status   — show a Notification Center toast with /api/health
#   scripts/tray-mac.sh open     — open dashboard in default browser
#   scripts/tray-mac.sh restart  — kill + relaunch the router (best effort)
set -euo pipefail

URL="${FLOW_ROUTER_URL:-http://127.0.0.1:2402}"
TITLE="flow_router"

cmd="${1:-status}"
case "$cmd" in
  status)
    if body=$(curl -sf "$URL/api/health" 2>/dev/null); then
      osascript -e "display notification \"OK: $body\" with title \"$TITLE\""
    else
      osascript -e "display notification \"Router unreachable at $URL\" with title \"$TITLE\" sound name \"Basso\""
      exit 1
    fi
    ;;
  open)
    open "$URL"
    ;;
  restart)
    pkill -x flow_router 2>/dev/null || true
    sleep 1
    nohup flow_router --addr 127.0.0.1:2402 >/dev/null 2>&1 &
    osascript -e "display notification \"Router restarted\" with title \"$TITLE\""
    ;;
  *)
    echo "Usage: $0 {status|open|restart}" >&2
    exit 2
    ;;
esac
