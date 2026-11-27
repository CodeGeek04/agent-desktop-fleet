#!/bin/sh
# Provision a minimal agent desktop inside a Debian guest.
# Stack: Xvfb (virtual X display), openbox (window manager), tint2 (taskbar),
# xdotool (input), x11vnc (live view), scrot (screenshots), Firefox ESR.
set -eu

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq --no-install-recommends \
  xvfb openbox tint2 xdotool x11vnc scrot \
  firefox-esr fonts-dejavu-core dbus-x11 ca-certificates procps \
  >/dev/null
apt-get clean
rm -rf /var/lib/apt/lists/*

install -m 755 /opt/spike/desktop-start.sh /usr/local/bin/desktop-start
echo "desktop provisioned"
