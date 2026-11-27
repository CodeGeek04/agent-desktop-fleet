package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/computer"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
)

// maxWaitSeconds caps the screenshot tool's wait_seconds.
const maxWaitSeconds = 30

type screenshotIn struct {
	Desktop     string  `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	WaitSeconds float64 `json:"wait_seconds,omitempty" jsonschema:"seconds to wait before capturing, 0 to 30, for pages that are still loading"`
}

type clickIn struct {
	Desktop    string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	X          int    `json:"x" jsonschema:"pixels from the left edge"`
	Y          int    `json:"y" jsonschema:"pixels from the top edge"`
	Button     string `json:"button,omitempty" jsonschema:"left (default), middle or right"`
	Double     bool   `json:"double,omitempty" jsonschema:"double-click instead of a single click"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

type pointIn struct {
	Desktop    string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	X          int    `json:"x" jsonschema:"pixels from the left edge"`
	Y          int    `json:"y" jsonschema:"pixels from the top edge"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

type dragIn struct {
	Desktop    string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	FromX      int    `json:"from_x" jsonschema:"start, pixels from the left edge"`
	FromY      int    `json:"from_y" jsonschema:"start, pixels from the top edge"`
	ToX        int    `json:"to_x" jsonschema:"end, pixels from the left edge"`
	ToY        int    `json:"to_y" jsonschema:"end, pixels from the top edge"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

type scrollIn struct {
	Desktop    string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	X          int    `json:"x" jsonschema:"pixels from the left edge; the pointer moves here before scrolling"`
	Y          int    `json:"y" jsonschema:"pixels from the top edge"`
	Direction  string `json:"direction,omitempty" jsonschema:"up, down (default), left or right"`
	Amount     int    `json:"amount,omitempty" jsonschema:"wheel clicks, 1 to 50 (default 3)"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

type typeIn struct {
	Desktop    string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	Text       string `json:"text" jsonschema:"text to type into the focused window"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

type keyIn struct {
	Desktop    string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	Keys       string `json:"keys" jsonschema:"xdotool key names or chords, such as Return, ctrl+l, ctrl+shift+t, Page_Down; separate several with spaces"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

type desktopIn struct {
	Desktop string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
}

type cursorOut struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func (t *tools) addActions(s *mcp.Server) {
	screen := fmt.Sprintf("The screen is %dx%d pixels; (0, 0) is the top-left corner. ", image.Width, image.Height)
	act := &mcp.ToolAnnotations{DestructiveHint: ptr(false), OpenWorldHint: ptr(true)}

	mcp.AddTool(s, &mcp.Tool{Name: "screenshot", Description: "Capture the desktop screen as a PNG. " + screen,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: ptr(false)}}, recorded(t, "screenshot", func(in screenshotIn) string { return in.Desktop }, screenshotDetail, t.screenshot))
	mcp.AddTool(s, &mcp.Tool{Name: "click", Description: "Move the pointer to a point and click. " + screen +
		"Returns a screenshot afterwards unless screenshot is false.", Annotations: act}, recorded(t, "click", func(in clickIn) string { return in.Desktop }, clickDetail, t.click))
	mcp.AddTool(s, &mcp.Tool{Name: "move_mouse", Description: "Move the pointer without clicking, for hover menus. " + screen,
		Annotations: act}, recorded(t, "move_mouse", func(in pointIn) string { return in.Desktop }, func(in pointIn) string { return point(in.X, in.Y) }, t.moveMouse))
	mcp.AddTool(s, &mcp.Tool{Name: "drag", Description: "Press the left button at one point, move to another and release. " + screen,
		Annotations: act}, recorded(t, "drag", func(in dragIn) string { return in.Desktop }, dragDetail, t.drag))
	mcp.AddTool(s, &mcp.Tool{Name: "scroll", Description: "Scroll with the mouse wheel at a point. " + screen,
		Annotations: act}, recorded(t, "scroll", func(in scrollIn) string { return in.Desktop }, scrollDetail, t.scroll))
	mcp.AddTool(s, &mcp.Tool{Name: "type_text", Description: "Type text into the focused window, as keyboard input. " +
		"Click a field first to focus it. A newline (\\n) presses Enter. Use press_key for Tab and shortcuts.", Annotations: act}, recorded(t, "type_text", func(in typeIn) string { return in.Desktop }, typeDetail, t.typeText))
	mcp.AddTool(s, &mcp.Tool{Name: "press_key", Description: "Press a key or shortcut, such as Return, Tab, Escape, ctrl+l " +
		"(focus the address bar), ctrl+t (new tab) or ctrl+w (close tab).", Annotations: act}, recorded(t, "press_key", func(in keyIn) string { return in.Desktop }, func(in keyIn) string { return in.Keys }, t.pressKey))
	mcp.AddTool(s, &mcp.Tool{Name: "cursor_position", Description: "Return the pointer position in pixels.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: ptr(false)}}, t.cursorPosition)
}

// act validates the desktop, runs do, and returns a note plus an optional screenshot.
func (t *tools) act(ctx context.Context, desktop string, shot *bool, note string, do func(c *computer.Computer) error) (*mcp.CallToolResult, any, error) {
	c, err := t.computer(desktop)
	if err != nil {
		return nil, nil, err
	}
	if err := do(c); err != nil {
		return nil, nil, explain(desktop, err)
	}
	return t.withScreenshot(ctx, desktop, shot, t.opts.Settle, note)
}

func (t *tools) screenshot(ctx context.Context, _ *mcp.CallToolRequest, in screenshotIn) (*mcp.CallToolResult, any, error) {
	if in.WaitSeconds < 0 || in.WaitSeconds > maxWaitSeconds {
		return nil, nil, fmt.Errorf("wait_seconds must be between 0 and %d", maxWaitSeconds)
	}
	wait := time.Duration(in.WaitSeconds * float64(time.Second))
	note := fmt.Sprintf("screenshot of %q (%dx%d)", in.Desktop, image.Width, image.Height)
	return t.withScreenshot(ctx, in.Desktop, nil, wait, note)
}

func (t *tools) click(ctx context.Context, _ *mcp.CallToolRequest, in clickIn) (*mcp.CallToolResult, any, error) {
	b := computer.Button(in.Button)
	return t.act(ctx, in.Desktop, in.Screenshot, fmt.Sprintf("clicked at (%d, %d)", in.X, in.Y), func(c *computer.Computer) error {
		if in.Double {
			return c.DoubleClick(ctx, in.X, in.Y, b)
		}
		return c.Click(ctx, in.X, in.Y, b)
	})
}

func (t *tools) moveMouse(ctx context.Context, _ *mcp.CallToolRequest, in pointIn) (*mcp.CallToolResult, any, error) {
	return t.act(ctx, in.Desktop, in.Screenshot, fmt.Sprintf("moved pointer to (%d, %d)", in.X, in.Y), func(c *computer.Computer) error {
		return c.Move(ctx, in.X, in.Y)
	})
}

func (t *tools) drag(ctx context.Context, _ *mcp.CallToolRequest, in dragIn) (*mcp.CallToolResult, any, error) {
	note := fmt.Sprintf("dragged from (%d, %d) to (%d, %d)", in.FromX, in.FromY, in.ToX, in.ToY)
	return t.act(ctx, in.Desktop, in.Screenshot, note, func(c *computer.Computer) error {
		return c.Drag(ctx, in.FromX, in.FromY, in.ToX, in.ToY)
	})
}

func (t *tools) scroll(ctx context.Context, _ *mcp.CallToolRequest, in scrollIn) (*mcp.CallToolResult, any, error) {
	dir, amount := scrollDefaults(in)
	note := fmt.Sprintf("scrolled %s %d at (%d, %d)", dir, amount, in.X, in.Y)
	return t.act(ctx, in.Desktop, in.Screenshot, note, func(c *computer.Computer) error {
		return c.Scroll(ctx, in.X, in.Y, dir, amount)
	})
}

func (t *tools) typeText(ctx context.Context, _ *mcp.CallToolRequest, in typeIn) (*mcp.CallToolResult, any, error) {
	return t.act(ctx, in.Desktop, in.Screenshot, fmt.Sprintf("typed %d characters", len([]rune(in.Text))), func(c *computer.Computer) error {
		return c.Type(ctx, in.Text)
	})
}

func (t *tools) pressKey(ctx context.Context, _ *mcp.CallToolRequest, in keyIn) (*mcp.CallToolResult, any, error) {
	return t.act(ctx, in.Desktop, in.Screenshot, "pressed "+in.Keys, func(c *computer.Computer) error {
		return c.Key(ctx, in.Keys)
	})
}

func (t *tools) cursorPosition(ctx context.Context, _ *mcp.CallToolRequest, in desktopIn) (*mcp.CallToolResult, cursorOut, error) {
	c, err := t.computer(in.Desktop)
	if err != nil {
		return nil, cursorOut{}, err
	}
	x, y, err := c.CursorPosition(ctx)
	if err != nil {
		return nil, cursorOut{}, explain(in.Desktop, err)
	}
	return nil, cursorOut{X: x, Y: y}, nil
}

// scrollDefaults applies the documented defaults: down, 3 wheel clicks.
func scrollDefaults(in scrollIn) (computer.Direction, int) {
	dir := computer.Direction(in.Direction)
	if dir == "" {
		dir = computer.ScrollDown
	}
	amount := in.Amount
	if amount == 0 {
		amount = 3
	}
	return dir, amount
}
