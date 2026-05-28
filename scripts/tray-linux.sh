#!/usr/bin/env bash
# flow_router — Linux launcher (CGO-free).
# A true persistent tray icon on Linux requires CGO (AppIndicator/GTK). This
# script instead offers a "control surface": notify-send + xdg-open. Pair with
# a .desktop entry to launch from the application menu.
#
# Usage:
#   scripts/tray-linux.sh status   — show a desktop notification with /api/health
#   scripts/tray-linux.sh open     — open dashboard in browser
#   scripts/tray-linux.sh restart  — kill + relaunch the router (best effort)
set -euo pipefail

URL="${FLOW_ROUTER_URL:-http://127.0.0.1:2402}"
TITLE="flow_router"

cmd="${1:-status}"
case "$cmd" in
  status)
    if body=$(curl -sf "$URL/api/health" 2>/dev/null); then
      notify-send "$TITLE" "OK: $body" 2>/dev/null || echo "OK: $body"
    else
      notify-send -u critical "$TITLE" "Router unreachable at $URL" 2>/dev/null || echo "Router unreachable at $URL"
      exit 1
    fi
    ;;
  open)
    xdg-open "$URL" >/dev/null 2>&1 &
    ;;
  restart)
    pkill -x flow_router 2>/dev/null || true
    sleep 1
    nohup flow_router --addr 127.0.0.1:2402 >/dev/null 2>&1 &
    notify-send "$TITLE" "Router restarted" 2>/dev/null || echo "Router restarted"
    ;;
  *)
    echo "Usage: $0 {status|open|restart}" >&2
    exit 2
    ;;
esac
