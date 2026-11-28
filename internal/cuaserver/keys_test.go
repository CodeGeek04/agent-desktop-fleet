package cuaserver

import (
	"reflect"
	"strings"
	"testing"
)

func TestXKey(t *testing.T) {
	cases := map[string]string{
		// cua-computer's Key values
		"enter": "Return", "esc": "Escape", "tab": "Tab", "space": "space", "backspace": "BackSpace", "del": "Delete",
		"pagedown": "Next", "pageup": "Prior", "home": "Home", "end": "End",
		"left": "Left", "right": "Right", "up": "Up", "down": "Down",
		"ctrl": "ctrl", "alt": "alt", "shift": "shift", "win": "super", "cmd": "super", "command": "super", "option": "alt",
		"f1": "F1", "f12": "F12", "f24": "F24",
		// pynput, pyautogui, xdotool and browser names, in any case
		"Return": "Return", "ESCAPE": "Escape", "page_down": "Next", "Page_Up": "Prior", "PageDown": "Next",
		"BackSpace": "BackSpace", "delete": "Delete", "ctrl_l": "ctrl", "Control": "ctrl", "ctrl_r": "Control_R",
		"alt_l": "alt", "shift_r": "Shift_R", "super": "super", "meta": "super", "caps_lock": "Caps_Lock",
		"print_screen": "Print", "ArrowLeft": "Left", "page down": "Next", "F5": "F5",
		// single characters
		"a": "a", "Z": "Z", "7": "7", "/": "slash", "+": "plus", "-": "minus", " ": "space", ".": "period",
		"\n": "Return", "é": "U00E9",
		// X names for punctuation, which xdotool-trained models send
		"minus": "minus", "plus": "plus", "equal": "equal", "comma": "comma", "bracketleft": "bracketleft",
		// keypad and media keysyms pass through unchanged
		"KP_Enter": "KP_Enter", "XF86AudioMute": "XF86AudioMute",
	}
	for in, want := range cases {
		got, err := xKey(in)
		if err != nil || got != want {
			t.Errorf("xKey(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	// xdotool ignores names it does not know, so these must fail here instead.
	for _, bad := range []string{"", "not a key", "Retrun", "ctrl;reboot", "--window", "\x00", "Hyper_Typo"} {
		if got, err := xKey(bad); err == nil || !strings.HasPrefix(err.Error(), "Unknown key: ") {
			t.Errorf("xKey(%q) = %q, %v; want an Unknown key error", bad, got, err)
		}
	}
}

func TestSplitCombo(t *testing.T) {
	cases := map[string][]string{
		"enter":        {"enter"},
		"+":            {"+"},
		"ctrl+c":       {"ctrl", "c"},
		"ctrl+shift+t": {"ctrl", "shift", "t"},
		"ctrl++":       {"ctrl", "+"},
	}
	for in, want := range cases {
		if got := splitCombo(in); !reflect.DeepEqual(got, want) {
			t.Errorf("splitCombo(%q) = %q, want %q", in, got, want)
		}
	}
	if chord, err := xChord(splitCombo("Ctrl+Page_Down")); err != nil || chord != "ctrl+Next" {
		t.Errorf("chord = %q, %v", chord, err)
	}
}
