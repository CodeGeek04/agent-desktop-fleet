#!/bin/sh
# Provision the agent desktop inside a Debian guest. Runs once per template.
# Stack: Xvfb (virtual X display), openbox (window manager), tint2 (taskbar),
# xdotool (input), scrot (screenshots), Firefox ESR, and fonts for most scripts
# (Noto core, WenQuanYi Micro Hei for Chinese, Japanese and Korean, colour emoji: 56 MB).
set -eu

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq --no-install-recommends \
  xvfb openbox tint2 xdotool scrot \
  firefox-esr fonts-dejavu-core dbus-x11 ca-certificates procps \
  fonts-noto-core fonts-wqy-microhei fonts-noto-color-emoji \
  >/dev/null
apt-get clean
rm -rf /var/lib/apt/lists/*

# Firefox policies: no telemetry, studies or default-browser checks.
mkdir -p /etc/firefox-esr/policies
cat > /etc/firefox-esr/policies/policies.json <<'EOF'
{
  "policies": {
    "DisableTelemetry": true,
    "DisableFirefoxStudies": true,
    "DontCheckDefaultBrowser": true
  }
}
EOF

# Firefox default prefs: a blank window with no first-run, onboarding, privacy-notice
# or session-restore tabs. Policies alone do not suppress these on Firefox 153 ESR.
cat > /usr/lib/firefox-esr/defaults/pref/adf.js <<'EOF'
pref("browser.aboutwelcome.enabled", false);
pref("trailhead.firstrun.didSeeAboutWelcome", true);
pref("browser.startup.homepage_override.mstone", "ignore");
pref("startup.homepage_welcome_url", "");
pref("startup.homepage_welcome_url.additional", "");
pref("startup.homepage_override_url", "");
pref("datareporting.policy.firstRunURL", "");
pref("datareporting.policy.dataSubmissionPolicyBypassNotification", true);
pref("toolkit.telemetry.reportingpolicy.firstRun", false);
pref("browser.shell.checkDefaultBrowser", false);
pref("browser.sessionstore.resume_from_crash", false);
pref("browser.startup.page", 0);
pref("browser.newtabpage.enabled", false);
pref("browser.toolbars.bookmarks.visibility", "never");
EOF
echo "desktop packages installed"
