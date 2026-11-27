// Package image holds the guest desktop definition: provisioning scripts and display constants.
package image

import _ "embed"

// Version is written to /etc/adf-image-version once a template is provisioned.
// Bump it whenever desktop-setup.sh or desktop-start.sh changes.
const Version = "4"

// Display is the X display the desktop runs on.
const Display = ":1"

// Screen size. 1280x800 stays within current computer-use models' screenshot limits.
const (
	Width  = 1280
	Height = 800
)

// SetupScript installs the desktop packages. It runs once per template.
//
//go:embed desktop-setup.sh
var SetupScript string

// StartScript is installed as /usr/local/bin/desktop-start in the guest.
//
//go:embed desktop-start.sh
var StartScript string

// ResetBrowserScript stops Firefox and deletes its profile so the next desktop-start
// opens a fresh profile with the current default prefs. Used when a template is upgraded.
const ResetBrowserScript = `pkill -x firefox-esr
i=0
while pgrep -x firefox-esr >/dev/null && [ "$i" -lt 20 ]; do i=$((i + 1)); sleep 0.25; done
rm -rf /root/.mozilla /root/.config/mozilla /root/.cache/mozilla
true`
