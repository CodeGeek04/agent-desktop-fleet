// Package computer turns computer-use actions into commands that run inside a desktop.
package computer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Execer runs argv inside one desktop and returns its stdout.
type Execer func(ctx context.Context, stdin io.Reader, argv ...string) ([]byte, error)

// Screen describes the desktop's X display.
type Screen struct {
	Display string
	Width   int
	Height  int
}

// Button is a mouse button.
type Button string

// Mouse buttons. The empty Button means Left.
const (
	Left   Button = "left"
	Middle Button = "middle"
	Right  Button = "right"
)

// Direction is a scroll direction.
type Direction string

// Scroll directions.
const (
	ScrollUp    Direction = "up"
	ScrollDown  Direction = "down"
	ScrollLeft  Direction = "left"
	ScrollRight Direction = "right"
)

// MaxScroll is the largest number of wheel clicks in one Scroll call.
const MaxScroll = 50

// ErrInvalid marks errors caused by bad input, raised before anything runs in the desktop.
var ErrInvalid = errors.New("invalid input")

// Computer performs actions on one desktop.
type Computer struct {
	exec   Execer
	screen Screen
}

// New returns a Computer that runs commands through exec.
func New(exec Execer, screen Screen) *Computer { return &Computer{exec: exec, screen: screen} }

func (c *Computer) xdotool(ctx context.Context, args ...string) ([]byte, error) {
	argv := append([]string{"env", "DISPLAY=" + c.screen.Display, "xdotool"}, args...)
	return c.exec(ctx, nil, argv...)
}

func (c *Computer) checkPoint(x, y int) error {
	if x < 0 || y < 0 || x >= c.screen.Width || y >= c.screen.Height {
		return fmt.Errorf("%w: point (%d, %d) is outside the %dx%d screen", ErrInvalid, x, y, c.screen.Width, c.screen.Height)
	}
	return nil
}

func buttonNumber(b Button) (string, error) {
	switch b {
	case Left, "":
		return "1", nil
	case Middle:
		return "2", nil
	case Right:
		return "3", nil
	}
	return "", fmt.Errorf("%w: unknown mouse button %q (want left, middle or right)", ErrInvalid, b)
}

func itoa(i int) string { return strconv.Itoa(i) }

// Move moves the pointer.
func (c *Computer) Move(ctx context.Context, x, y int) error {
	if err := c.checkPoint(x, y); err != nil {
		return err
	}
	_, err := c.xdotool(ctx, "mousemove", itoa(x), itoa(y))
	return err
}

// Click moves the pointer and clicks once.
func (c *Computer) Click(ctx context.Context, x, y int, b Button) error {
	if err := c.checkPoint(x, y); err != nil {
		return err
	}
	n, err := buttonNumber(b)
	if err != nil {
		return err
	}
	_, err = c.xdotool(ctx, "mousemove", itoa(x), itoa(y), "click", n)
	return err
}

// DoubleClick moves the pointer and clicks twice.
func (c *Computer) DoubleClick(ctx context.Context, x, y int, b Button) error {
	if err := c.checkPoint(x, y); err != nil {
		return err
	}
	n, err := buttonNumber(b)
	if err != nil {
		return err
	}
	_, err = c.xdotool(ctx, "mousemove", itoa(x), itoa(y), "click", "--repeat", "2", "--delay", "80", n)
	return err
}

// Drag presses the left button at (x1, y1), moves to (x2, y2) and releases.
func (c *Computer) Drag(ctx context.Context, x1, y1, x2, y2 int) error {
	if err := c.checkPoint(x1, y1); err != nil {
		return err
	}
	if err := c.checkPoint(x2, y2); err != nil {
		return err
	}
	_, err := c.xdotool(ctx, "mousemove", itoa(x1), itoa(y1), "mousedown", "1",
		"mousemove", itoa(x2), itoa(y2), "mouseup", "1")
	return err
}

// Scroll moves the pointer and turns the wheel amount clicks in dir.
func (c *Computer) Scroll(ctx context.Context, x, y int, dir Direction, amount int) error {
	if err := c.checkPoint(x, y); err != nil {
		return err
	}
	if amount < 1 || amount > MaxScroll {
		return fmt.Errorf("%w: scroll amount %d out of range 1 to %d", ErrInvalid, amount, MaxScroll)
	}
	buttons := map[Direction]string{ScrollUp: "4", ScrollDown: "5", ScrollLeft: "6", ScrollRight: "7"}
	b, ok := buttons[dir]
	if !ok {
		return fmt.Errorf("%w: unknown scroll direction %q (want up, down, left or right)", ErrInvalid, dir)
	}
	_, err := c.xdotool(ctx, "mousemove", itoa(x), itoa(y), "click", "--repeat", itoa(amount), b)
	return err
}

// Type types text as keyboard input.
func (c *Computer) Type(ctx context.Context, text string) error {
	if text == "" {
		return nil
	}
	_, err := c.xdotool(ctx, "type", "--delay", "12", "--", text)
	return err
}

// keyRE allows xdotool key names and chords such as Return, ctrl+l, ctrl+shift+Tab, F5.
var keyRE = regexp.MustCompile(`^[A-Za-z0-9_]+(\+[A-Za-z0-9_]+)*$`)

// Key presses a key or chord such as "Return" or "ctrl+l". Several keys separated by
// spaces are pressed in order.
func (c *Computer) Key(ctx context.Context, combo string) error {
	keys := strings.Fields(combo)
	if len(keys) == 0 {
		return fmt.Errorf("%w: no key given", ErrInvalid)
	}
	for _, k := range keys {
		if !keyRE.MatchString(k) {
			return fmt.Errorf("%w: key %q", ErrInvalid, k)
		}
	}
	_, err := c.xdotool(ctx, append([]string{"key", "--"}, keys...)...)
	return err
}

var pngMagic = []byte("\x89PNG\r\n\x1a\n")

// captureScript captures the screen to a unique temp file, prints it, and cleans up.
// ext and opts are fixed by this package, never caller input; opts ends with a space when set.
func captureScript(ext, opts string) string {
	return `f=$(mktemp /tmp/adf-shot-XXXXXX) || exit 1
scrot -o -z ` + opts + `"$f.` + ext + `" && cat "$f.` + ext + `"
rc=$?
rm -f "$f" "$f.` + ext + `"
exit $rc`
}

// screenshotScript captures a PNG.
var screenshotScript = captureScript("png", "")

var jpegMagic = []byte{0xFF, 0xD8, 0xFF}

// Screenshot returns the whole screen as PNG bytes.
func (c *Computer) Screenshot(ctx context.Context) ([]byte, error) {
	out, err := c.exec(ctx, nil, "env", "DISPLAY="+c.screen.Display, "sh", "-c", screenshotScript)
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	if !bytes.HasPrefix(out, pngMagic) {
		return nil, fmt.Errorf("screenshot: output is not a PNG (%d bytes)", len(out))
	}
	return out, nil
}

// CursorPosition returns the pointer position.
func (c *Computer) CursorPosition(ctx context.Context) (int, int, error) {
	out, err := c.xdotool(ctx, "getmouselocation", "--shell")
	if err != nil {
		return 0, 0, err
	}
	x, y := -1, -1
	for _, line := range strings.Split(string(out), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		switch k {
		case "X":
			x = n
		case "Y":
			y = n
		}
	}
	if x < 0 || y < 0 {
		return 0, 0, fmt.Errorf("unexpected xdotool output: %q", out)
	}
	return x, y, nil
}

// ScreenshotJPEG returns the whole screen as JPEG bytes at quality 1 to 100.
// It is smaller and faster than Screenshot, which suits live views.
func (c *Computer) ScreenshotJPEG(ctx context.Context, quality int) ([]byte, error) {
	if quality < 1 || quality > 100 {
		return nil, fmt.Errorf("%w: jpeg quality %d out of range 1 to 100", ErrInvalid, quality)
	}
	script := captureScript("jpg", "-q "+itoa(quality)+" ")
	out, err := c.exec(ctx, nil, "env", "DISPLAY="+c.screen.Display, "sh", "-c", script)
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	if !bytes.HasPrefix(out, jpegMagic) {
		return nil, fmt.Errorf("screenshot: output is not a JPEG (%d bytes)", len(out))
	}
	return out, nil
}
