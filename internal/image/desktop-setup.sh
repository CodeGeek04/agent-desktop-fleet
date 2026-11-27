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

# Firefox settings, as an enterprise policy in the install directory: the only place this
# Debian build reads them from (it ignores /etc/firefox-esr/policies and defaults/pref).
# A blank start page with no first-run, onboarding, privacy-notice, sponsored or news
# content, and no bookmarks bar. Firefox also skips its spare pre-launched content process
# and runs extensions in the main process: the desktop VM already isolates everything, and
# this saves about 120 MB per desktop with a page open (663 MB to 542 MB, 12 to 7 processes).
mkdir -p /usr/lib/firefox-esr/distribution
cat > /usr/lib/firefox-esr/distribution/policies.json <<'EOF'
{
  "policies": {
    "DisableTelemetry": true,
    "DisableFirefoxStudies": true,
    "DontCheckDefaultBrowser": true,
    "DisablePocket": true,
    "DisplayBookmarksToolbar": "never",
    "NoDefaultBookmarks": true,
    "NewTabPage": false,
    "Homepage": { "URL": "about:blank", "StartPage": "none", "Locked": true },
    "OverrideFirstRunPage": "",
    "OverridePostUpdatePage": "",
    "FirefoxHome": {
      "Search": false, "TopSites": false, "SponsoredTopSites": false, "Highlights": false,
      "Pocket": false, "Stories": false, "SponsoredPocket": false, "SponsoredStories": false,
      "Snippets": false, "Locked": true
    },
    "FirefoxSuggest": { "WebSuggestions": false, "SponsoredSuggestions": false, "ImproveSuggest": false, "Locked": true },
    "UserMessaging": {
      "ExtensionRecommendations": false, "FeatureRecommendations": false, "UrlbarInterventions": false,
      "SkipOnboarding": true, "MoreFromMozilla": false, "Locked": true
    },
    "Preferences": {
      "browser.aboutwelcome.enabled": { "Value": false, "Status": "default" },
      "browser.startup.homepage_override.mstone": { "Value": "ignore", "Status": "default" },
      "browser.sessionstore.resume_from_crash": { "Value": false, "Status": "default" },
      "browser.shell.checkDefaultBrowser": { "Value": false, "Status": "default" },
      "datareporting.policy.dataSubmissionPolicyBypassNotification": { "Value": true, "Status": "default" },
      "datareporting.policy.firstRunURL": { "Value": "", "Status": "default" },
      "dom.ipc.processPrelaunch.enabled": { "Value": false, "Status": "default" },
      "extensions.webextensions.remote": { "Value": false, "Status": "default" }
    }
  }
}
EOF
echo "desktop packages installed"
