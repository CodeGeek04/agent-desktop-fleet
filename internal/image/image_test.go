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
