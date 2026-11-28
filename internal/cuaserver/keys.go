package cuaserver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// keyNames maps key names Cua clients send to X keysym names that xdotool understands. Cua's
// own names (enter, esc, pagedown, cmd), pynput attribute names (page_down, ctrl_l), pyautogui
// names and browser KeyboardEvent names all arrive, often from model output, so lookups
// ignore case, spaces, dashes and underscores.
var keyNames = map[string]string{
	"enter": "Return", "return": "Return", "esc": "Escape", "escape": "Escape", "tab": "Tab",
	"space": "space", "spacebar": "space", "backspace": "BackSpace", "del": "Delete", "delete": "Delete",
	"insert": "Insert", "ins": "Insert", "home": "Home", "end": "End",
	"pageup": "Prior", "pgup": "Prior", "prior": "Prior", "pagedown": "Next", "pgdn": "Next", "next": "Next",
	"left": "Left", "right": "Right", "up": "Up", "down": "Down",
	"arrowleft": "Left", "arrowright": "Right", "arrowup": "Up", "arrowdown": "Down",
	"ctrl": "ctrl", "control": "ctrl", "ctrll": "ctrl", "controll": "ctrl", "ctrlr": "Control_R", "controlr": "Control_R",
	"alt": "alt", "option": "alt", "altl": "alt", "altr": "Alt_R", "altgr": "ISO_Level3_Shift",
	"shift": "shift", "shiftl": "shift", "shiftr": "Shift_R",
	"win": "super", "windows": "super", "super": "super", "superl": "super", "superr": "Super_R",
	"cmd": "super", "cmdl": "super", "cmdr": "Super_R", "command": "super", "meta": "super",
	"capslock": "Caps_Lock", "numlock": "Num_Lock", "scrolllock": "Scroll_Lock",
	"printscreen": "Print", "print": "Print", "prtsc": "Print", "prtscr": "Print",
	"pause": "Pause", "menu": "Menu", "apps": "Menu", "contextmenu": "Menu",
}

// charNames names the ASCII punctuation keys, which xdotool's key command takes by keysym name.
var charNames = map[rune]string{
	' ': "space", '!': "exclam", '"': "quotedbl", '#': "numbersign", '$': "dollar", '%': "percent",
	'&': "ampersand", '\'': "apostrophe", '(': "parenleft", ')': "parenright", '*': "asterisk",
	'+': "plus", ',': "comma", '-': "minus", '.': "period", '/': "slash", ':': "colon",
	';': "semicolon", '<': "less", '=': "equal", '>': "greater", '?': "question", '@': "at",
	'[': "bracketleft", '\\': "backslash", ']': "bracketright", '^': "asciicircum", '_': "underscore",
	'`': "grave", '{': "braceleft", '|': "bar", '}': "braceright", '~': "asciitilde",
	'\n': "Return", '\r': "Return", '\t': "Tab", '\b': "BackSpace", 0x1b: "Escape",
}

func init() {
	for i := 1; i <= 24; i++ {
		keyNames["f"+strconv.Itoa(i)] = "F" + strconv.Itoa(i)
	}
	// Models trained on xdotool send punctuation by keysym name, as in "ctrl+minus".
	for _, x := range charNames {
		keyNames[strings.ToLower(x)] = x
	}
}

// keysymRE matches the keypad and media keysym families, such as KP_Enter or XF86AudioMute,
// which pass to xdotool as given. Other names must be in the table: xdotool silently ignores
// a key name it does not know, so passing typos through would report success for nothing.
var keysymRE = regexp.MustCompile(`^(KP|XF86)_?[A-Za-z0-9_]+$`)

// xKey turns one key name from a Cua client into an xdotool key name.
func xKey(name string) (string, error) {
	if r := []rune(name); len(r) == 1 {
		switch c := r[0]; {
		case c < unicode.MaxASCII && (unicode.IsLetter(c) || unicode.IsDigit(c)):
			return name, nil
		case charNames[c] != "":
			return charNames[c], nil
		case unicode.IsPrint(c):
			return fmt.Sprintf("U%04X", c), nil // X's keysym for any Unicode character
		}
		return "", fmt.Errorf("Unknown key: %s", name) // Cua's wording
	}
	norm := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(name)))
	if x, ok := keyNames[norm]; ok {
		return x, nil
	}
	if keysymRE.MatchString(name) {
		return name, nil
	}
	return "", fmt.Errorf("Unknown key: %s", name) // Cua's wording
}

// splitCombo splits a chord written as one string, such as "ctrl+c" or "ctrl++", into keys.
// A lone "+" is the plus key.
func splitCombo(s string) []string {
	if len([]rune(s)) <= 1 || !strings.Contains(s, "+") {
		return []string{s}
	}
	plus := strings.HasSuffix(s, "++")
	if plus {
		s = strings.TrimSuffix(s, "++")
	}
	keys := strings.Split(s, "+")
	if plus {
		keys = append(keys, "+")
	}
	return keys
}

// xChord turns keys pressed together into an xdotool chord such as "ctrl+shift+t".
func xChord(keys []string) (string, error) {
	if len(keys) == 0 {
		return "", fmt.Errorf("no keys given")
	}
	out := make([]string, len(keys))
	for i, k := range keys {
		x, err := xKey(k)
		if err != nil {
			return "", err
		}
		out[i] = x
	}
	return strings.Join(out, "+"), nil
}
