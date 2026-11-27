#!/bin/sh
# Start the agent desktop and return once it is ready for screenshots.
# Usage: desktop-start [WIDTHxHEIGHT]   (default 1280x800, Anthropic's recommended size)
set -eu

GEOMETRY="${1:-1280x800}"
export DISPLAY=:1

if ! pgrep -x Xvfb >/dev/null; then
  Xvfb :1 -screen 0 "${GEOMETRY}x24" -nolisten tcp >/tmp/xvfb.log 2>&1 &
fi

# Wait for the X server socket.
i=0
until [ -S /tmp/.X11-unix/X1 ] || [ "$i" -ge 100 ]; do i=$((i + 1)); sleep 0.05; done

pgrep -x openbox >/dev/null || (openbox >/tmp/openbox.log 2>&1 &)
pgrep -x tint2 >/dev/null || (tint2 >/tmp/tint2.log 2>&1 &)
pgrep -x x11vnc >/dev/null || (x11vnc -display :1 -forever -shared -nopw -rfbport 5900 -listen 0.0.0.0 -quiet >/tmp/x11vnc.log 2>&1 &)

if [ "${NO_BROWSER:-0}" != "1" ] && ! pgrep -f firefox-esr >/dev/null; then
  (firefox-esr --new-instance about:blank >/tmp/firefox.log 2>&1 &)
fi

echo "desktop ready on DISPLAY=:1 (${GEOMETRY}), VNC on 5900"
