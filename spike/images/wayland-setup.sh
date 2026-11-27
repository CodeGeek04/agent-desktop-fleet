#!/bin/sh
# Provision a host-display agent desktop inside a Debian guest (smolvm SMOLVM_DISPLAY mode).
# The host captures the screen and injects input over virtio-gpu and virtio-input,
# so the guest runs no VNC server and no screenshot or input tools.
# Stack: sway (pixman renderer) + Firefox ESR (native Wayland) + udev for input devices.
set -eu

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq --no-install-recommends \
  sway swaybg foot udev firefox-esr fonts-dejavu-core ca-certificates procps \
  >/dev/null
apt-get clean
rm -rf /var/lib/apt/lists/*

# Firefox: skip first-run pages, telemetry prompts and default-browser checks.
mkdir -p /etc/firefox-esr/policies
cat > /etc/firefox-esr/policies/policies.json <<'EOF'
{
  "policies": {
    "DisableTelemetry": true,
    "DisableFirefoxStudies": true,
    "DontCheckDefaultBrowser": true,
    "OverrideFirstRunPage": "",
    "OverridePostUpdatePage": "",
    "UserMessaging": { "SkipOnboarding": true, "WhatsNew": false, "FeatureRecommendations": false },
    "Homepage": { "URL": "about:blank", "StartPage": "homepage" },
    "Preferences": {
      "browser.sessionstore.resume_from_crash": { "Value": false, "Status": "locked" }
    }
  }
}
EOF

mkdir -p /etc/agent-desktop
cat > /etc/agent-desktop/sway.cfg <<'EOF'
output * bg #1d2733 solid_color
default_border normal
focus_follows_mouse no
exec firefox-esr --new-instance about:blank
EOF

cat > /usr/local/bin/desktop-up <<'EOF'
#!/bin/sh
# Bring up input devices and the compositor. Idempotent.
set -eu
mount -o remount,rw /sys 2>/dev/null || true
mkdir -p /dev/input
for d in /sys/class/input/event*; do
  [ -e "$d/dev" ] || continue
  node="/dev/input/$(basename "$d")"
  [ -e "$node" ] || mknod "$node" c "$(cut -d: -f1 "$d/dev")" "$(cut -d: -f2 "$d/dev")"
done
pgrep -x systemd-udevd >/dev/null || /usr/lib/systemd/systemd-udevd --daemon
udevadm trigger --subsystem-match=input --action=add 2>/dev/null || true
udevadm settle 2>/dev/null || true

export XDG_RUNTIME_DIR=/run/agent-desktop WLR_RENDERER=pixman LIBSEAT_BACKEND=noop
export WLR_NO_HARDWARE_CURSORS=1 MOZ_ENABLE_WAYLAND=1
mkdir -p "$XDG_RUNTIME_DIR" && chmod 700 "$XDG_RUNTIME_DIR"
if ! pgrep -x sway >/dev/null; then
  nohup sway -c /etc/agent-desktop/sway.cfg >/var/log/sway.log 2>&1 &
fi
i=0
until [ -S "$XDG_RUNTIME_DIR/wayland-1" ] || [ "$i" -ge 100 ]; do i=$((i + 1)); sleep 0.05; done
echo "desktop up"
EOF
chmod 755 /usr/local/bin/desktop-up
echo "wayland desktop provisioned"
