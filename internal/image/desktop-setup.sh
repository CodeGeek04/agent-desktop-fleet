#!/bin/sh
# Provision the agent desktop inside a Debian guest. Runs once per template.
# Stack: Xvfb (virtual X display), openbox (window manager), tint2 (taskbar),
# xdotool (input), scrot (screenshots), Firefox ESR.
set -eu

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq --no-install-recommends \
  xvfb openbox tint2 xdotool scrot \
  firefox-esr fonts-dejavu-core dbus-x11 ca-certificates procps \
  >/dev/null
apt-get clean
rm -rf /var/lib/apt/lists/*

# Firefox: no first-run pages, telemetry prompts, default-browser checks or session restore.
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
echo "desktop packages installed"
