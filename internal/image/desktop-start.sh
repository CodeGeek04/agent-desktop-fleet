#!/bin/sh
# Start the agent desktop. Idempotent: a second run starts only what is missing.
# Usage: desktop-start [WIDTHxHEIGHT]
set -eu

GEOMETRY="${1:-1280x800}"
export DISPLAY=:1

if ! pgrep -x Xvfb >/dev/null; then
  nohup Xvfb :1 -screen 0 "${GEOMETRY}x24" -nolisten tcp >/tmp/xvfb.log 2>&1 &
fi
i=0
until [ -S /tmp/.X11-unix/X1 ] || [ "$i" -ge 100 ]; do i=$((i + 1)); sleep 0.05; done

pgrep -x openbox >/dev/null || nohup openbox >/tmp/openbox.log 2>&1 &
pgrep -x tint2 >/dev/null || nohup tint2 >/tmp/tint2.log 2>&1 &
pgrep -x firefox-esr >/dev/null || nohup firefox-esr --new-instance >/tmp/firefox.log 2>&1 &

echo "desktop started on DISPLAY=:1 (${GEOMETRY})"
