package cuaserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/computer"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
)

func screenCommands() map[string]command {
	return map[string]command{
		"screenshot": {params: []param{opt("format", "png"), opt("quality", 95)}, need: needAwake, run: screenshot,
			detail: func(p params) string {
				if f, _ := p.strOr("format", "png"); f != "png" {
					return f
				}
				return ""
			}},
		// The screen size is fixed by the image, so this needs no command in the desktop. It
		// still checks the desktop exists: clients poll it to decide the server is ready.
		"get_screen_size": {need: needDesktop, run: func(context.Context, desk, params) (result, error) {
			return result{"size": map[string]int{"width": image.Width, "height": image.Height}}, nil
		}},
		"get_cursor_position": {need: needAwake, run: func(ctx context.Context, d desk, _ params) (result, error) {
			x, y, err := d.c.CursorPosition(ctx)
			return result{"position": map[string]int{"x": x, "y": y}}, err
		}},
		// Parity stubs: Cua's Linux server had no desktop-environment detection worth the name,
		// a fixed accessibility tree and no element search, and code written for it expects that.
		"get_desktop_environment": {need: needDesktop, run: func(context.Context, desk, params) (result, error) {
			return result{"environment": "openbox"}, nil
		}},
		"get_accessibility_tree": {need: needDesktop, run: func(context.Context, desk, params) (result, error) {
			return result{"tree": map[string]any{
				"role": "Window", "title": "Linux Window", "position": map[string]int{"x": 0, "y": 0},
				"size": map[string]int{"width": image.Width, "height": image.Height}, "children": []any{},
			}}, nil
		}},
		"find_element": {params: []param{opt("role", nil), opt("title", nil), opt("value", nil)}, need: needDesktop,
			run: func(context.Context, desk, params) (result, error) {
				return result{"success": false, "message": "Element search not supported on Linux"}, nil
			}},
	}
}

func screenshot(ctx context.Context, d desk, p params) (result, error) {
	format, err := p.strOr("format", "png")
	if err != nil {
		return nil, err
	}
	quality, err := p.intOr("quality", 95)
	if err != nil {
		return nil, err
	}
	var img []byte
	switch strings.ToLower(format) {
	case "png":
		format = "png"
		img, err = d.c.Screenshot(ctx)
	case "jpeg", "jpg":
		format = "jpeg"
		img, err = d.c.ScreenshotJPEG(ctx, quality)
	default:
		return nil, fmt.Errorf("unsupported screenshot format %q (want png or jpeg)", format)
	}
	if err != nil {
		return nil, err
	}
	return result{"image_data": base64.StdEncoding.EncodeToString(img), "format": format}, nil
}

func inputCommands() map[string]command {
	xy := []param{opt("x", nil), opt("y", nil)}
	xyButton := append(append([]param{}, xy...), opt("button", "left"))
	click := func(fixed computer.Button, times int) func(context.Context, desk, params) (result, error) {
		return func(ctx context.Context, d desk, p params) (result, error) {
			at, err := p.point()
			if err != nil {
				return nil, err
			}
			b := fixed
			if b == "" {
				// left_click takes cua-sandbox's button argument, which Cua's server ignored.
				if b, err = p.button(); err != nil {
					return nil, err
				}
			}
			return nil, d.c.ClickAt(ctx, at, b, times)
		}
	}
	button := func(down bool) func(context.Context, desk, params) (result, error) {
		return func(ctx context.Context, d desk, p params) (result, error) {
			at, err := p.point()
			if err != nil {
				return nil, err
			}
			b, err := p.button()
			if err != nil {
				return nil, err
			}
			if down {
				return nil, d.c.MouseDown(ctx, at, b)
			}
			return nil, d.c.MouseUp(ctx, at, b)
		}
	}
	keyEvent := func(down bool) func(context.Context, desk, params) (result, error) {
		return func(ctx context.Context, d desk, p params) (result, error) {
			k, err := p.mustStr("key")
			if err != nil {
				return nil, err
			}
			chord, err := xChord(splitCombo(k))
			if err != nil {
				return nil, err
			}
			if down {
				return nil, d.c.KeyDown(ctx, chord)
			}
			return nil, d.c.KeyUp(ctx, chord)
		}
	}
	keyDetail := func(p params) string { k, _ := p.strOr("key", ""); return k }
	scrollClicks := func(dir computer.Direction) func(context.Context, desk, params) (result, error) {
		return func(ctx context.Context, d desk, p params) (result, error) {
			n, err := p.intOr("clicks", 1)
			if err != nil {
				return nil, err
			}
			return nil, scrollSteps(ctx, d.c, nil, dir, n)
		}
	}
	clicksDetail := func(p params) string { n, _ := p.intOr("clicks", 1); return fmt.Sprintf("%d", n) }

	return map[string]command{
		"left_click":   {params: xyButton, need: needAwake, run: click("", 1), detail: atDetail},
		"right_click":  {params: xy, need: needAwake, run: click(computer.Right, 1), detail: atDetail},
		"double_click": {params: xy, need: needAwake, run: click(computer.Left, 2), detail: atDetail},
		"move_cursor": {params: []param{req("x"), req("y")}, need: needAwake, detail: atDetail,
			run: func(ctx context.Context, d desk, p params) (result, error) {
				x, err := p.mustInt("x")
				if err != nil {
					return nil, err
				}
				y, err := p.mustInt("y")
				if err != nil {
					return nil, err
				}
				return nil, d.c.Move(ctx, x, y)
			}},
		"mouse_down": {params: xyButton, need: needAwake, run: button(true), detail: buttonDetail},
		"mouse_up":   {params: xyButton, need: needAwake, run: button(false), detail: buttonDetail},
		// Cua's drags take a duration; xdotool moves at once, as Cua's Linux server did.
		"drag_to": {params: []param{req("x"), req("y"), opt("button", "left"), opt("duration", 0.5)}, need: needAwake,
			detail: func(p params) string { return "to " + atDetail(p) },
			run: func(ctx context.Context, d desk, p params) (result, error) {
				x, err := p.mustInt("x")
				if err != nil {
					return nil, err
				}
				y, err := p.mustInt("y")
				if err != nil {
					return nil, err
				}
				b, err := p.button()
				if err != nil {
					return nil, err
				}
				return nil, d.c.DragTo(ctx, x, y, b)
			}},
		"drag": {params: []param{req("path"), opt("button", "left"), opt("duration", 0.5)}, need: needAwake, run: drag,
			detail: func(p params) string {
				path, err := p.path("path")
				if err != nil || len(path) == 0 {
					return ""
				}
				first, last := path[0], path[len(path)-1]
				return point(first.X, first.Y) + " to " + point(last.X, last.Y)
			}},
		"type_text": {params: []param{req("text")}, need: needAwake,
			// Only the length is recorded: typed text can be a password.
			detail: func(p params) string {
				t, _ := p.strOr("text", "")
				return fmt.Sprintf("%d characters", len([]rune(t)))
			},
			run: func(ctx context.Context, d desk, p params) (result, error) {
				text, err := p.mustStr("text")
				if err != nil {
					return nil, err
				}
				return nil, d.c.Type(ctx, text)
			}},
		"press_key": {params: []param{req("key")}, need: needAwake, detail: keyDetail,
			run: func(ctx context.Context, d desk, p params) (result, error) {
				k, err := p.mustStr("key")
				if err != nil {
					return nil, err
				}
				// Models often send a whole chord, such as "ctrl+c", to press_key.
				chord, err := xChord(splitCombo(k))
				if err != nil {
					return nil, err
				}
				return nil, d.c.Key(ctx, chord)
			}},
		"hotkey": {params: []param{req("keys")}, need: needAwake, run: hotkey,
			detail: func(p params) string { k, _ := hotkeyKeys(p); return strings.Join(k, "+") }},
		"key_down": {params: []param{req("key")}, need: needAwake, run: keyEvent(true), detail: keyDetail},
		"key_up":   {params: []param{req("key")}, need: needAwake, run: keyEvent(false), detail: keyDetail},
		"scroll": {params: []param{req("x"), req("y")}, need: needAwake, detail: scrollDetail,
			run: func(ctx context.Context, d desk, p params) (result, error) {
				at, dx, dy, err := scrollPlan(p)
				if err != nil {
					return nil, err
				}
				return nil, scrollBy(ctx, d.c, at, dx, dy)
			}},
		"scroll_down": {params: []param{opt("clicks", 1)}, need: needAwake, run: scrollClicks(computer.ScrollDown), detail: clicksDetail},
		"scroll_up":   {params: []param{opt("clicks", 1)}, need: needAwake, run: scrollClicks(computer.ScrollUp), detail: clicksDetail},
		"scroll_direction": {params: []param{req("direction"), opt("clicks", 1)}, need: needAwake,
			detail: func(p params) string {
				dir, _ := p.strOr("direction", "")
				return dir + " " + clicksDetail(p)
			},
			run: func(ctx context.Context, d desk, p params) (result, error) {
				dir, err := p.mustStr("direction")
				if err != nil {
					return nil, err
				}
				n, err := p.intOr("clicks", 1)
				if err != nil {
					return nil, err
				}
				switch dir = strings.ToLower(dir); computer.Direction(dir) {
				case computer.ScrollUp, computer.ScrollDown, computer.ScrollLeft, computer.ScrollRight:
				default:
					return nil, fmt.Errorf("Invalid direction: %s. Use 'up', 'down', 'left', or 'right'.", dir)
				}
				return nil, scrollSteps(ctx, d.c, nil, computer.Direction(dir), n)
			}},
	}
}

func atDetail(p params) string {
	at, err := p.point()
	if err != nil {
		return ""
	}
	if at == nil {
		return "at pointer"
	}
	return point(at.X, at.Y)
}

func buttonDetail(p params) string {
	d := atDetail(p)
	if b, _ := p.button(); b != computer.Left {
		d += " " + string(b)
	}
	return d
}

func drag(ctx context.Context, d desk, p params) (result, error) {
	path, err := p.path("path")
	if err != nil {
		return nil, err
	}
	b, err := p.button()
	if err != nil {
		return nil, err
	}
	switch len(path) {
	case 0:
		return nil, fmt.Errorf("Path is empty") // Cua's wording
	case 1:
		// Cua's server only moved the pointer for a one-point path.
		return nil, d.c.Move(ctx, path[0].X, path[0].Y)
	}
	return nil, d.c.DragPath(ctx, path, b)
}

// hotkeyKeys reads hotkey's keys: a list, or one string such as "ctrl+c". Items of the list
// may be chords too: cua-sandbox's keypress("ctrl+c") sends ["ctrl+c"].
func hotkeyKeys(p params) ([]string, error) {
	if s, ok := p["keys"].(string); ok {
		return splitCombo(s), nil
	}
	list, err := p.strings("keys")
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, k := range list {
		keys = append(keys, splitCombo(k)...)
	}
	return keys, nil
}

// hotkey presses keys together. xdotool presses them in order and releases them in reverse,
// as Cua's server did.
func hotkey(ctx context.Context, d desk, p params) (result, error) {
	keys, err := hotkeyKeys(p)
	if err != nil {
		return nil, err
	}
	chord, err := xChord(keys)
	if err != nil {
		return nil, err
	}
	return nil, d.c.Key(ctx, chord)
}

// scrollPlan reads scroll's arguments, which mean different things to different clients.
// cua-computer (and Cua's server) send x and y as wheel amounts at the current pointer.
// cua-sandbox sends x and y as the pointer position, with the amounts in scroll_x and scroll_y.
// Either way a positive y scrolls up and a positive x scrolls right, as with pynput.
func scrollPlan(p params) (at *computer.Point, dx, dy int, err error) {
	if p.has("scroll_x") || p.has("scroll_y") {
		if at, err = p.point(); err != nil {
			return nil, 0, 0, err
		}
		if dx, err = p.intOr("scroll_x", 0); err != nil {
			return nil, 0, 0, err
		}
		dy, err = p.intOr("scroll_y", 0)
		return at, dx, dy, err
	}
	if dx, err = p.intOr("x", 0); err != nil {
		return nil, 0, 0, err
	}
	dy, err = p.intOr("y", 0)
	return nil, dx, dy, err
}

func scrollDetail(p params) string {
	at, dx, dy, err := scrollPlan(p)
	if err != nil {
		return ""
	}
	var parts []string
	if dy != 0 {
		parts = append(parts, fmt.Sprintf("%s %d", pick(dy > 0, "up", "down"), abs(dy)))
	}
	if dx != 0 {
		parts = append(parts, fmt.Sprintf("%s %d", pick(dx > 0, "right", "left"), abs(dx)))
	}
	if at != nil {
		parts = append(parts, "at "+point(at.X, at.Y))
	}
	return strings.Join(parts, " ")
}

// scrollBy scrolls vertically by dy and horizontally by dx wheel clicks at at (or at the
// pointer when at is nil). Positive dy is up and positive dx is right.
func scrollBy(ctx context.Context, c *computer.Computer, at *computer.Point, dx, dy int) error {
	if dx == 0 && dy == 0 {
		if at != nil {
			return c.Move(ctx, at.X, at.Y)
		}
		return nil
	}
	if dy != 0 {
		if err := scrollSteps(ctx, c, at, pick(dy > 0, computer.ScrollUp, computer.ScrollDown), abs(dy)); err != nil {
			return err
		}
		at = nil // the pointer is there now
	}
	if dx != 0 {
		return scrollSteps(ctx, c, at, pick(dx > 0, computer.ScrollRight, computer.ScrollLeft), abs(dx))
	}
	return nil
}

// scrollSteps turns the wheel n clicks. Zero does nothing, a negative n counts as its size
// (Cua's scroll_down took abs(clicks)), and more than computer.MaxScroll is capped, since
// larger values are usually pixels mistaken for clicks.
func scrollSteps(ctx context.Context, c *computer.Computer, at *computer.Point, dir computer.Direction, n int) error {
	n = min(abs(n), computer.MaxScroll)
	if n == 0 {
		return nil
	}
	return c.ScrollAt(ctx, at, dir, n)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func pick[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
