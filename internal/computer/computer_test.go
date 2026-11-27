package computer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

type recorder struct {
	argv [][]string
	out  []byte
	err  error
}

func (r *recorder) exec(_ context.Context, _ io.Reader, argv ...string) ([]byte, error) {
	r.argv = append(r.argv, append([]string(nil), argv...))
	return r.out, r.err
}

var ctx = context.Background()
var screen = Screen{Display: ":1", Width: 1280, Height: 800}

func xdo(args ...string) []string { return append([]string{"env", "DISPLAY=:1", "xdotool"}, args...) }

func TestActionsBuildXdotoolCommands(t *testing.T) {
	cases := []struct {
		name string
		do   func(c *Computer) error
		want []string
	}{
		{"move", func(c *Computer) error { return c.Move(ctx, 10, 20) }, xdo("mousemove", "10", "20")},
		{"left click", func(c *Computer) error { return c.Click(ctx, 10, 20, Left) }, xdo("mousemove", "10", "20", "click", "1")},
		{"default button", func(c *Computer) error { return c.Click(ctx, 1, 2, "") }, xdo("mousemove", "1", "2", "click", "1")},
		{"right click", func(c *Computer) error { return c.Click(ctx, 10, 20, Right) }, xdo("mousemove", "10", "20", "click", "3")},
		{"double click", func(c *Computer) error { return c.DoubleClick(ctx, 5, 6, Left) },
			xdo("mousemove", "5", "6", "click", "--repeat", "2", "--delay", "80", "1")},
		{"drag", func(c *Computer) error { return c.Drag(ctx, 1, 2, 30, 40) },
			xdo("mousemove", "1", "2", "mousedown", "1", "mousemove", "30", "40", "mouseup", "1")},
		{"scroll down", func(c *Computer) error { return c.Scroll(ctx, 100, 200, ScrollDown, 3) },
			xdo("mousemove", "100", "200", "click", "--repeat", "3", "5")},
		{"scroll up", func(c *Computer) error { return c.Scroll(ctx, 100, 200, ScrollUp, 1) },
			xdo("mousemove", "100", "200", "click", "--repeat", "1", "4")},
		{"type", func(c *Computer) error { return c.Type(ctx, "-n hello") }, xdo("type", "--delay", "12", "--", "-n hello")},
		{"key", func(c *Computer) error { return c.Key(ctx, "ctrl+l") }, xdo("key", "--", "ctrl+l")},
		{"key sequence", func(c *Computer) error { return c.Key(ctx, "ctrl+a  BackSpace") }, xdo("key", "--", "ctrl+a", "BackSpace")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &recorder{}
			if err := tc.do(New(r.exec, screen)); err != nil {
				t.Fatal(err)
			}
			if len(r.argv) != 1 || !reflect.DeepEqual(r.argv[0], tc.want) {
				t.Fatalf("argv = %q\nwant   %q", r.argv, tc.want)
			}
		})
	}
}

func TestEmptyTypeIsNoOp(t *testing.T) {
	r := &recorder{}
	if err := New(r.exec, screen).Type(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if len(r.argv) != 0 {
		t.Fatalf("expected no command, got %q", r.argv)
	}
}

func TestInvalidInputIsRejectedBeforeExec(t *testing.T) {
	cases := map[string]func(c *Computer) error{
		"x too large":     func(c *Computer) error { return c.Click(ctx, 1280, 10, Left) },
		"negative y":      func(c *Computer) error { return c.Move(ctx, 10, -1) },
		"drag end off":    func(c *Computer) error { return c.Drag(ctx, 1, 1, 10, 800) },
		"unknown button":  func(c *Computer) error { return c.Click(ctx, 1, 1, "thumb") },
		"bad direction":   func(c *Computer) error { return c.Scroll(ctx, 1, 1, "sideways", 1) },
		"zero amount":     func(c *Computer) error { return c.Scroll(ctx, 1, 1, ScrollDown, 0) },
		"huge amount":     func(c *Computer) error { return c.Scroll(ctx, 1, 1, ScrollDown, 51) },
		"empty key":       func(c *Computer) error { return c.Key(ctx, "  ") },
		"shell in key":    func(c *Computer) error { return c.Key(ctx, "ctrl+l;reboot") },
		"option-like key": func(c *Computer) error { return c.Key(ctx, "--window") },
	}
	for name, do := range cases {
		t.Run(name, func(t *testing.T) {
			r := &recorder{}
			if err := do(New(r.exec, screen)); !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
			if len(r.argv) != 0 {
				t.Fatalf("command ran despite invalid input: %q", r.argv)
			}
		})
	}
}

func TestScreenshotReturnsPNG(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), []byte("rest")...)
	r := &recorder{out: png}
	got, err := New(r.exec, screen).Screenshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, png) {
		t.Fatalf("got %q", got)
	}
	argv := r.argv[0]
	if !reflect.DeepEqual(argv[:4], []string{"env", "DISPLAY=:1", "sh", "-c"}) {
		t.Fatalf("argv prefix = %q", argv[:4])
	}
	if argv[4] != screenshotScript {
		t.Fatalf("script = %q", argv[4])
	}
}

func TestScreenshotRejectsNonPNG(t *testing.T) {
	r := &recorder{out: []byte("not an image")}
	if _, err := New(r.exec, screen).Screenshot(ctx); err == nil {
		t.Fatal("expected an error for non-PNG output")
	}
}

func TestScreenshotWrapsExecError(t *testing.T) {
	r := &recorder{err: errors.New("exec failed")}
	if _, err := New(r.exec, screen).Screenshot(ctx); err == nil {
		t.Fatal("expected an error")
	}
}

func TestCursorPosition(t *testing.T) {
	r := &recorder{out: []byte("X=50\nY=60\nSCREEN=0\nWINDOW=4194884\n")}
	x, y, err := New(r.exec, screen).CursorPosition(ctx)
	if err != nil || x != 50 || y != 60 {
		t.Fatalf("x=%d y=%d err=%v", x, y, err)
	}
	if !reflect.DeepEqual(r.argv[0], xdo("getmouselocation", "--shell")) {
		t.Fatalf("argv = %q", r.argv[0])
	}
}

func TestCursorPositionBadOutput(t *testing.T) {
	r := &recorder{out: []byte("garbage")}
	if _, _, err := New(r.exec, screen).CursorPosition(ctx); err == nil {
		t.Fatal("expected an error")
	}
}

func TestScreenshotJPEG(t *testing.T) {
	jpg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("rest")...)
	r := &recorder{out: jpg}
	got, err := New(r.exec, screen).ScreenshotJPEG(ctx, 60)
	if err != nil || !bytes.Equal(got, jpg) {
		t.Fatalf("got %q err=%v", got, err)
	}
	if !strings.Contains(r.argv[0][4], `scrot -o -z -q 60 "$f.jpg"`) {
		t.Fatalf("script = %q", r.argv[0][4])
	}
}

func TestScreenshotJPEGRejectsBadQualityAndOutput(t *testing.T) {
	r := &recorder{out: []byte("not a jpeg")}
	c := New(r.exec, screen)
	if _, err := c.ScreenshotJPEG(ctx, 0); !errors.Is(err, ErrInvalid) || len(r.argv) != 0 {
		t.Fatalf("quality 0: err=%v calls=%d", err, len(r.argv))
	}
	if _, err := c.ScreenshotJPEG(ctx, 60); err == nil {
		t.Fatal("expected an error for non-JPEG output")
	}
}

func TestTypeTurnsNewlinesIntoReturn(t *testing.T) {
	r := &recorder{}
	if err := New(r.exec, screen).Type(ctx, "example.com\nhello\r\n"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		xdo("type", "--delay", "12", "--", "example.com"),
		xdo("key", "Return"),
		xdo("type", "--delay", "12", "--", "hello"),
		xdo("key", "Return"),
	}
	if !reflect.DeepEqual(r.argv, want) {
		t.Fatalf("argv = %q\nwant   %q", r.argv, want)
	}
}
