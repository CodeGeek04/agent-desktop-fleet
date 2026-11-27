package image

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestScriptsMatchConstants(t *testing.T) {
	for _, pkg := range []string{"xvfb", "openbox", "xdotool", "scrot", "firefox-esr",
		"fonts-noto-core", "fonts-wqy-microhei", "fonts-noto-color-emoji"} {
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
	// Debian's firefox-esr reads policies only from its install directory; files in
	// /etc/firefox-esr/policies or defaults/pref are silently ignored.
	const marker = "cat > /usr/lib/firefox-esr/distribution/policies.json <<'EOF'\n"
	i := strings.Index(SetupScript, marker)
	if i < 0 {
		t.Fatal("Firefox policies must go to /usr/lib/firefox-esr/distribution/policies.json")
	}
	body := SetupScript[i+len(marker):]
	body = body[:strings.Index(body, "\nEOF\n")]
	var doc struct {
		Policies map[string]json.RawMessage `json:"policies"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("policies.json is not valid JSON: %v", err)
	}
	for _, name := range []string{"NewTabPage", "Homepage", "FirefoxHome", "UserMessaging", "DisableTelemetry", "Preferences"} {
		if _, ok := doc.Policies[name]; !ok {
			t.Errorf("policies.json is missing %s", name)
		}
	}
	for _, pref := range []string{"browser.aboutwelcome.enabled", "datareporting.policy.dataSubmissionPolicyBypassNotification",
		"browser.sessionstore.resume_from_crash", "dom.ipc.processPrelaunch.enabled"} {
		if !strings.Contains(string(doc.Policies["Preferences"]), `"`+pref+`"`) {
			t.Errorf("policies.json Preferences is missing %s", pref)
		}
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
