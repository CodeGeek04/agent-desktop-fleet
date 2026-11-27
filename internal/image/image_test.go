package image

import (
	"fmt"
	"strings"
	"testing"
)

func TestScriptsMatchConstants(t *testing.T) {
	for _, pkg := range []string{"xvfb", "openbox", "xdotool", "scrot", "firefox-esr"} {
		if !strings.Contains(SetupScript, pkg) {
			t.Errorf("setup script does not install %s", pkg)
		}
	}
	if !strings.Contains(StartScript, "Xvfb "+Display+" ") {
		t.Errorf("start script does not start Xvfb on %s", Display)
	}
	geometry := fmt.Sprintf("%dx%d", Width, Height)
	if !strings.Contains(StartScript, `GEOMETRY="${1:-`+geometry+`}"`) {
		t.Errorf("start script default geometry is not %s", geometry)
	}
	if Version == "" {
		t.Error("Version must not be empty")
	}
}

func TestFirefoxStartsClean(t *testing.T) {
	for _, pref := range []string{
		`pref("browser.aboutwelcome.enabled", false);`,
		`pref("datareporting.policy.dataSubmissionPolicyBypassNotification", true);`,
		`pref("browser.sessionstore.resume_from_crash", false);`,
		`pref("browser.newtabpage.enabled", false);`,
	} {
		if !strings.Contains(SetupScript, pref) {
			t.Errorf("setup script is missing %s", pref)
		}
	}
	if !strings.Contains(SetupScript, "/usr/lib/firefox-esr/defaults/pref/adf.js") {
		t.Error("prefs must go to Firefox's defaults/pref directory")
	}
	if strings.Contains(StartScript, "pgrep -f") {
		t.Error("start script must match processes by name (pgrep -x), not by command line")
	}
	if !strings.Contains(ResetBrowserScript, "/root/.config/mozilla") {
		t.Error("browser reset must remove the XDG profile directory")
	}
	if strings.Contains(ResetBrowserScript, "pkill -f") {
		t.Error("browser reset must not use pkill -f, which can match its own shell")
	}
}
