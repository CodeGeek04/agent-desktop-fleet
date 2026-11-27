package console

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/computer"
)

// inputReq is one input event from the browser.
type inputReq struct {
	Kind   string `json:"kind"` // click, double_click, move, drag, scroll, type, key
	X      int    `json:"x"`
	Y      int    `json:"y"`
	ToX    int    `json:"to_x"`
	ToY    int    `json:"to_y"`
	Button string `json:"button"`
	Dir    string `json:"dir"`
	Amount int    `json:"amount"`
	Text   string `json:"text"`
	Key    string `json:"key"` // KeyboardEvent.key
	Ctrl   bool   `json:"ctrl"`
	Alt    bool   `json:"alt"`
	Shift  bool   `json:"shift"`
	Meta   bool   `json:"meta"`
}

func point(x, y int) string { return fmt.Sprintf("(%d, %d)", x, y) }

// applyInput performs one input event and returns a short description for the activity log.
// Typed text is described by its length only.
func applyInput(ctx context.Context, c *computer.Computer, in inputReq) (string, error) {
	switch in.Kind {
	case "click":
		return point(in.X, in.Y), c.Click(ctx, in.X, in.Y, computer.Button(in.Button))
	case "double_click":
		return point(in.X, in.Y), c.DoubleClick(ctx, in.X, in.Y, computer.Button(in.Button))
	case "move":
		return point(in.X, in.Y), c.Move(ctx, in.X, in.Y)
	case "drag":
		return point(in.X, in.Y) + " to " + point(in.ToX, in.ToY), c.Drag(ctx, in.X, in.Y, in.ToX, in.ToY)
	case "scroll":
		amount := in.Amount
		if amount == 0 {
			amount = 1
		}
		return fmt.Sprintf("%s %d at %s", in.Dir, amount, point(in.X, in.Y)),
			c.Scroll(ctx, in.X, in.Y, computer.Direction(in.Dir), amount)
	case "type":
		return fmt.Sprintf("%d characters", len([]rune(in.Text))), c.Type(ctx, in.Text)
	case "key":
		combo, text, err := browserKey(in.Key, in.Ctrl, in.Alt, in.Shift, in.Meta)
		if err != nil {
			return in.Key, err
		}
		if text != "" {
			return "1 character", c.Type(ctx, text)
		}
		return combo, c.Key(ctx, combo)
	}
	return "", fmt.Errorf("%w: unknown input kind %q", computer.ErrInvalid, in.Kind)
}

// specialKeys maps KeyboardEvent.key names to xdotool key names.
var specialKeys = map[string]string{
	"Enter": "Return", "Backspace": "BackSpace", "Tab": "Tab", "Escape": "Escape", "Delete": "Delete",
	"Insert": "Insert", "Home": "Home", "End": "End", "PageUp": "Page_Up", "PageDown": "Page_Down",
	"ArrowUp": "Up", "ArrowDown": "Down", "ArrowLeft": "Left", "ArrowRight": "Right", " ": "space",
}

func init() {
	for i := 1; i <= 12; i++ {
		k := "F" + strconv.Itoa(i)
		specialKeys[k] = k
	}
}

// browserKey turns a browser key event into an xdotool chord, or into text to type when it
// is a plain character (the browser has already applied Shift to it).
func browserKey(key string, ctrl, alt, shift, meta bool) (combo, text string, err error) {
	name, special := specialKeys[key]
	chord := ctrl || alt || meta
	if !special {
		r := []rune(key)
		if len(r) != 1 {
			return "", "", fmt.Errorf("%w: unsupported key %q", computer.ErrInvalid, key)
		}
		if !chord {
			return "", key, nil
		}
		if r[0] > unicode.MaxASCII || !(unicode.IsLetter(r[0]) || unicode.IsDigit(r[0])) {
			return "", "", fmt.Errorf("%w: unsupported shortcut key %q", computer.ErrInvalid, key)
		}
		name = strings.ToLower(key)
	}
	var parts []string
	if ctrl {
		parts = append(parts, "ctrl")
	}
	if alt {
		parts = append(parts, "alt")
	}
	if shift && (special || chord) {
		parts = append(parts, "shift")
	}
	if meta {
		parts = append(parts, "super")
	}
	return strings.Join(append(parts, name), "+"), "", nil
}
