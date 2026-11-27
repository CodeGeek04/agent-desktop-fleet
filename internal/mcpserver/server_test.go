package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/activity"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet/fleettest"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

var testPNG = append([]byte("\x89PNG\r\n\x1a\n"), []byte("fake image")...)

// newTestRuntime returns a fake runtime with a warm, provisioned template whose
// screenshots return testPNG.
func newTestRuntime() *fleettest.Fake {
	rt := fleettest.New()
	rt.Machines[fleet.TemplateName] = &smolvm.Machine{Name: fleet.TemplateName, State: "running", Branchable: true,
		Labels: map[string]string{fleet.LabelRole: fleet.RoleTemplate}}
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		switch {
		case argv[0] == "cat":
			return []byte(image.Version), nil
		case len(argv) > 2 && argv[2] == "sh":
			return testPNG, nil
		}
		return nil, nil
	}
	return rt
}

// connect starts the MCP server on rt and returns a connected client session.
func connect(t *testing.T, rt *fleettest.Fake) *mcp.ClientSession {
	t.Helper()
	return connectWith(t, rt, Options{Version: "test"})
}

// connectWith starts the MCP server on rt with opts and returns a connected client session.
func connectWith(t *testing.T, rt *fleettest.Fake, opts Options) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	m := fleet.NewManager(rt, nil)
	m.Settle = 0
	server := New(m, opts)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: protocol error: %v", name, err)
	}
	return res
}

func text(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func images(res *mcp.CallToolResult) [][]byte {
	var out [][]byte
	for _, c := range res.Content {
		if ic, ok := c.(*mcp.ImageContent); ok {
			out = append(out, ic.Data)
		}
	}
	return out
}

func TestListDesktops(t *testing.T) {
	rt := newTestRuntime()
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{fleet.LabelRole: fleet.RoleDesktop}}
	res := call(t, connect(t, rt), "list_desktops", map[string]any{})
	if res.IsError {
		t.Fatal(text(res))
	}
	var out struct {
		Desktops []struct{ Name, State string } `json:"desktops"`
	}
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil {
		t.Fatalf("result is not JSON: %v: %s", err, text(res))
	}
	if len(out.Desktops) != 1 || out.Desktops[0].Name != "a" || out.Desktops[0].State != "running" {
		t.Fatalf("desktops = %+v", out.Desktops)
	}
}

func TestListDesktopsEmptyIsArray(t *testing.T) {
	res := call(t, connect(t, newTestRuntime()), "list_desktops", map[string]any{})
	if res.IsError || !strings.Contains(text(res), `"desktops":[]`) {
		t.Fatalf("result = %s", text(res))
	}
}

func TestCreateDesktopReturnsScreenshot(t *testing.T) {
	rt := newTestRuntime()
	res := call(t, connect(t, rt), "create_desktop", map[string]any{"name": "a"})
	if res.IsError {
		t.Fatal(text(res))
	}
	if !strings.Contains(text(res), `created desktop "a"`) {
		t.Fatalf("text = %q", text(res))
	}
	if imgs := images(res); len(imgs) != 1 || !bytes.Equal(imgs[0], testPNG) {
		t.Fatalf("want one screenshot, got %d images", len(imgs))
	}
	if _, ok := rt.Machines["adf-a"]; !ok {
		t.Fatal("desktop machine was not created")
	}
}

func TestCreateDesktopBadNameIsToolError(t *testing.T) {
	res := call(t, connect(t, newTestRuntime()), "create_desktop", map[string]any{"name": "Bad Name"})
	if !res.IsError {
		t.Fatal("expected a tool error")
	}
}

func TestDeleteDesktop(t *testing.T) {
	rt := newTestRuntime()
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{fleet.LabelRole: fleet.RoleDesktop}}
	res := call(t, connect(t, rt), "delete_desktop", map[string]any{"name": "a"})
	if res.IsError {
		t.Fatal(text(res))
	}
	if _, ok := rt.Machines["adf-a"]; ok {
		t.Fatal("desktop still exists")
	}
}

func TestToolsAreListedWithDescriptions(t *testing.T) {
	cs := connect(t, newTestRuntime())
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		if tool.Description == "" {
			t.Errorf("tool %s has no description", tool.Name)
		}
		got[tool.Name] = true
	}
	for _, name := range wantTools {
		if !got[name] {
			t.Errorf("tool %s is missing", name)
		}
	}
}

// wantTools grows as tools are added in later tasks.
var wantTools = []string{"list_desktops", "create_desktop", "delete_desktop", "screenshot", "click",
	"move_mouse", "drag", "scroll", "type_text", "press_key", "cursor_position", "run_command",
	"start_desktop", "pause_desktop", "fleet_status"}

func desktopRuntime() *fleettest.Fake {
	rt := newTestRuntime()
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{fleet.LabelRole: fleet.RoleDesktop}}
	return rt
}

func TestActionsRunXdotoolAndReturnScreenshots(t *testing.T) {
	cases := []struct {
		tool string
		args map[string]any
		want string // expected exec call
	}{
		{"click", map[string]any{"desktop": "a", "x": 10, "y": 20}, "exec adf-a env DISPLAY=:1 xdotool mousemove 10 20 click 1"},
		{"click", map[string]any{"desktop": "a", "x": 10, "y": 20, "button": "right", "double": true},
			"exec adf-a env DISPLAY=:1 xdotool mousemove 10 20 click --repeat 2 --delay 80 3"},
		{"move_mouse", map[string]any{"desktop": "a", "x": 5, "y": 6}, "exec adf-a env DISPLAY=:1 xdotool mousemove 5 6"},
		{"drag", map[string]any{"desktop": "a", "from_x": 1, "from_y": 2, "to_x": 30, "to_y": 40},
			"exec adf-a env DISPLAY=:1 xdotool mousemove 1 2 mousedown 1 mousemove 30 40 mouseup 1"},
		{"scroll", map[string]any{"desktop": "a", "x": 100, "y": 200}, "exec adf-a env DISPLAY=:1 xdotool mousemove 100 200 click --repeat 3 5"},
		{"scroll", map[string]any{"desktop": "a", "x": 100, "y": 200, "direction": "up", "amount": 2},
			"exec adf-a env DISPLAY=:1 xdotool mousemove 100 200 click --repeat 2 4"},
		{"type_text", map[string]any{"desktop": "a", "text": "hello"}, "exec adf-a env DISPLAY=:1 xdotool type --delay 12 -- hello"},
		{"press_key", map[string]any{"desktop": "a", "keys": "ctrl+l"}, "exec adf-a env DISPLAY=:1 xdotool key -- ctrl+l"},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			rt := desktopRuntime()
			res := call(t, connect(t, rt), tc.tool, tc.args)
			if res.IsError {
				t.Fatal(text(res))
			}
			calls := rt.Calls()
			if calls[0] != tc.want {
				t.Fatalf("first call = %q\nwant         %q", calls[0], tc.want)
			}
			if imgs := images(res); len(imgs) != 1 {
				t.Fatalf("want a follow-up screenshot, got %d images", len(imgs))
			}
		})
	}
}

func TestActionWithoutScreenshot(t *testing.T) {
	rt := desktopRuntime()
	res := call(t, connect(t, rt), "click", map[string]any{"desktop": "a", "x": 1, "y": 1, "screenshot": false})
	if res.IsError || len(images(res)) != 0 || len(rt.Calls()) != 1 {
		t.Fatalf("isError=%v images=%d calls=%q", res.IsError, len(images(res)), rt.Calls())
	}
}

func TestScreenshotTool(t *testing.T) {
	res := call(t, connect(t, desktopRuntime()), "screenshot", map[string]any{"desktop": "a"})
	if res.IsError {
		t.Fatal(text(res))
	}
	if imgs := images(res); len(imgs) != 1 || !bytes.Equal(imgs[0], testPNG) {
		t.Fatal("expected the screenshot bytes")
	}
}

func TestScreenshotWaitOutOfRange(t *testing.T) {
	res := call(t, connect(t, desktopRuntime()), "screenshot", map[string]any{"desktop": "a", "wait_seconds": 31})
	if !res.IsError {
		t.Fatal("expected a tool error for wait_seconds above 30")
	}
}

func TestOutOfScreenClickIsToolErrorWithoutExec(t *testing.T) {
	rt := desktopRuntime()
	res := call(t, connect(t, rt), "click", map[string]any{"desktop": "a", "x": 5000, "y": 1})
	if !res.IsError || len(rt.Calls()) != 0 {
		t.Fatalf("isError=%v calls=%q", res.IsError, rt.Calls())
	}
}

func TestCursorPosition(t *testing.T) {
	rt := desktopRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		return []byte("X=7\nY=9\nSCREEN=0\nWINDOW=1\n"), nil
	}
	res := call(t, connect(t, rt), "cursor_position", map[string]any{"desktop": "a"})
	if res.IsError || !strings.Contains(text(res), `"x":7`) || !strings.Contains(text(res), `"y":9`) {
		t.Fatalf("result = %s", text(res))
	}
}

func TestUnknownDesktopExplainsWhatToDo(t *testing.T) {
	rt := newTestRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		return nil, &smolvm.Error{Code: "NOT_FOUND", Message: "vm not found: adf-zzz"}
	}
	res := call(t, connect(t, rt), "screenshot", map[string]any{"desktop": "zzz"})
	if !res.IsError || !strings.Contains(text(res), "create_desktop") {
		t.Fatalf("result = %s", text(res))
	}
}

func TestRunCommandSuccess(t *testing.T) {
	rt := desktopRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) { return []byte("hi\n"), nil }
	res := call(t, connect(t, rt), "run_command", map[string]any{"desktop": "a", "command": "echo hi"})
	if res.IsError {
		t.Fatal(text(res))
	}
	if got := rt.Calls()[0]; got != "exec adf-a sh -c echo hi" {
		t.Fatalf("call = %q", got)
	}
	var out struct {
		ExitCode int    `json:"exit_code"`
		Stdout   string `json:"stdout"`
	}
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil || out.ExitCode != 0 || out.Stdout != "hi\n" {
		t.Fatalf("out=%+v err=%v text=%s", out, err, text(res))
	}
}

func TestRunCommandNonZeroExitIsAResultNotAnError(t *testing.T) {
	rt := desktopRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		return []byte("partial"), &runner.ExitError{Name: "smolvm", Code: 3, Stderr: []byte("bad thing\n")}
	}
	res := call(t, connect(t, rt), "run_command", map[string]any{"desktop": "a", "command": "false"})
	if res.IsError {
		t.Fatalf("non-zero exit must not be a tool error: %s", text(res))
	}
	for _, want := range []string{`"exit_code":3`, `"stdout":"partial"`, `"stderr":"bad thing\n"`} {
		if !strings.Contains(text(res), want) {
			t.Errorf("result %s is missing %s", text(res), want)
		}
	}
}

func TestRunCommandTruncatesLongOutput(t *testing.T) {
	rt := desktopRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) { return bytes.Repeat([]byte("x"), maxOutput+100), nil }
	res := call(t, connect(t, rt), "run_command", map[string]any{"desktop": "a", "command": "yes"})
	if res.IsError || !strings.Contains(text(res), "[truncated") {
		t.Fatalf("expected truncation marker, got %d bytes", len(text(res)))
	}
}

func TestRunCommandOtherErrorsAreToolErrors(t *testing.T) {
	rt := desktopRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) { return nil, errors.New("smolvm crashed") }
	res := call(t, connect(t, rt), "run_command", map[string]any{"desktop": "a", "command": "true"})
	if !res.IsError {
		t.Fatal("expected a tool error")
	}
}

type memRecorder struct {
	mu     sync.Mutex
	events []activity.Event
}

func (m *memRecorder) Record(e activity.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
}

func (m *memRecorder) all() []activity.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]activity.Event(nil), m.events...)
}

func TestToolCallsAreRecorded(t *testing.T) {
	rec := &memRecorder{}
	cs := connectWith(t, desktopRuntime(), Options{Version: "test", Activity: rec})
	if res := call(t, cs, "click", map[string]any{"desktop": "a", "x": 10, "y": 20, "screenshot": false}); res.IsError {
		t.Fatal(text(res))
	}
	ev := rec.all()
	if len(ev) != 1 {
		t.Fatalf("events = %+v", ev)
	}
	e := ev[0]
	if e.Desktop != "a" || e.Source != activity.SourceAgent || e.Action != "click" || !e.OK || e.Detail != "(10, 20)" || e.Time.IsZero() {
		t.Fatalf("event = %+v", e)
	}
}

func TestFailedToolCallIsRecordedAsError(t *testing.T) {
	rec := &memRecorder{}
	cs := connectWith(t, desktopRuntime(), Options{Version: "test", Activity: rec})
	call(t, cs, "click", map[string]any{"desktop": "a", "x": 5000, "y": 1})
	if ev := rec.all(); len(ev) != 1 || ev[0].OK || ev[0].Error == "" {
		t.Fatalf("events = %+v", ev)
	}
}

func TestTypedTextIsNotRecorded(t *testing.T) {
	rec := &memRecorder{}
	cs := connectWith(t, desktopRuntime(), Options{Version: "test", Activity: rec})
	call(t, cs, "type_text", map[string]any{"desktop": "a", "text": "hunter2", "screenshot": false})
	if ev := rec.all(); len(ev) != 1 || ev[0].Detail != "7 characters" {
		t.Fatalf("events = %+v", ev)
	}
}

func TestListDesktopsIsNotRecorded(t *testing.T) {
	rec := &memRecorder{}
	cs := connectWith(t, desktopRuntime(), Options{Version: "test", Activity: rec})
	call(t, cs, "list_desktops", map[string]any{})
	if ev := rec.all(); len(ev) != 0 {
		t.Fatalf("events = %+v", ev)
	}
}

func TestPauseAndStartDesktop(t *testing.T) {
	rt := desktopRuntime()
	cs := connect(t, rt)
	if res := call(t, cs, "pause_desktop", map[string]any{"name": "a"}); res.IsError {
		t.Fatal(text(res))
	}
	if rt.Machines["adf-a"].State != "paused" {
		t.Fatal("desktop is still running")
	}
	res := call(t, cs, "start_desktop", map[string]any{"name": "a"})
	if res.IsError || len(images(res)) != 1 || !strings.Contains(text(res), `started desktop "a"`) {
		t.Fatalf("isError=%v images=%d text=%q", res.IsError, len(images(res)), text(res))
	}
	if rt.Machines["adf-a"].State != "running" {
		t.Fatal("desktop did not resume")
	}
}

func TestFleetStatus(t *testing.T) {
	res := call(t, connect(t, desktopRuntime()), "fleet_status", map[string]any{})
	if res.IsError || !strings.Contains(text(res), `"running":1`) || !strings.Contains(text(res), `"more_that_fit"`) {
		t.Fatalf("result = %s", text(res))
	}
}

func TestCreateDesktopReportsProgress(t *testing.T) {
	heartbeat = 5 * time.Millisecond
	t.Cleanup(func() { heartbeat = 10 * time.Second })
	rt := fleettest.New() // no template yet: the first create builds it
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		switch {
		case argv[0] == "cat":
			return nil, errors.New("not provisioned")
		case len(argv) > 1 && argv[1] == "/tmp/adf-setup.sh":
			time.Sleep(30 * time.Millisecond) // a slow package install
		case len(argv) > 2 && argv[2] == "sh":
			return testPNG, nil
		}
		return nil, nil
	}
	ctx := context.Background()
	m := fleet.NewManager(rt, nil)
	m.Poll, m.Settle = time.Millisecond, 0
	server := New(m, Options{Version: "test"})
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	var mu sync.Mutex
	var msgs []string
	client := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
			mu.Lock()
			msgs = append(msgs, req.Params.Message)
			mu.Unlock()
		},
	})
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	params := &mcp.CallToolParams{Name: "create_desktop", Arguments: map[string]any{"name": "a"}}
	params.SetProgressToken("p1")
	res, err := cs.CallTool(ctx, params)
	if err != nil || res.IsError {
		t.Fatalf("err=%v result=%s", err, text(res))
	}
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		joined := strings.Join(msgs, " | ")
		mu.Unlock()
		if strings.Contains(joined, "installing desktop packages") && len(msgs) >= 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("progress messages = %q", joined)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
