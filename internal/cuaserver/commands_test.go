package cuaserver

import (
	"encoding/base64"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"
)

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func xdo(args string) string { return "exec adf-a env DISPLAY=:1 xdotool " + args }

func TestInputCommandArgv(t *testing.T) {
	cases := []struct {
		name    string
		command string
		params  map[string]any
		want    []string
	}{
		{"click at", "left_click", map[string]any{"x": 10, "y": 20}, []string{xdo("mousemove 10 20 click 1")}},
		{"click here", "left_click", nil, []string{xdo("click 1")}},
		{"click with nulls", "left_click", map[string]any{"x": nil, "y": nil}, []string{xdo("click 1")}},
		{"fractional coordinates", "left_click", map[string]any{"x": 10.4, "y": 20.6}, []string{xdo("mousemove 10 21 click 1")}},
		{"sandbox button", "left_click", map[string]any{"x": 1, "y": 2, "button": "right"}, []string{xdo("mousemove 1 2 click 3")}},
		{"right click", "right_click", map[string]any{"x": 3, "y": 4}, []string{xdo("mousemove 3 4 click 3")}},
		{"double click here", "double_click", nil, []string{xdo("click --repeat 2 --delay 80 1")}},
		{"move", "move_cursor", map[string]any{"x": 5, "y": 6}, []string{xdo("mousemove 5 6")}},
		{"mouse down", "mouse_down", map[string]any{"button": "right"}, []string{xdo("mousedown 3")}},
		{"mouse down at", "mouse_down", map[string]any{"x": 7, "y": 8}, []string{xdo("mousemove 7 8 mousedown 1")}},
		{"mouse up middle", "mouse_up", map[string]any{"x": 7, "y": 8, "button": "middle"}, []string{xdo("mousemove 7 8 mouseup 2")}},
		{"drag path", "drag", map[string]any{"path": [][]int{{1, 2}, {3, 4}, {5, 6}}, "button": "left", "duration": 0.5},
			[]string{xdo("mousemove 1 2 mousedown 1 mousemove 3 4 mousemove 5 6 mouseup 1")}},
		{"drag path objects", "drag", map[string]any{"path": []map[string]int{{"x": 1, "y": 2}, {"x": 3, "y": 4}}, "button": "right"},
			[]string{xdo("mousemove 1 2 mousedown 3 mousemove 3 4 mouseup 3")}},
		{"one-point drag moves", "drag", map[string]any{"path": [][]int{{9, 9}}}, []string{xdo("mousemove 9 9")}},
		{"drag to", "drag_to", map[string]any{"x": 30, "y": 40}, []string{xdo("mousedown 1 mousemove 30 40 mouseup 1")}},
		{"type", "type_text", map[string]any{"text": "héllo"}, []string{xdo("type --delay 12 -- héllo")}},
		{"type newline", "type_text", map[string]any{"text": "a\n"}, []string{xdo("type --delay 12 -- a"), xdo("key Return")}},
		{"press enter", "press_key", map[string]any{"key": "enter"}, []string{xdo("key -- Return")}},
		{"press pynput name", "press_key", map[string]any{"key": "page_down"}, []string{xdo("key -- Next")}},
		{"press xdotool name", "press_key", map[string]any{"key": "BackSpace"}, []string{xdo("key -- BackSpace")}},
		{"press chord", "press_key", map[string]any{"key": "ctrl+c"}, []string{xdo("key -- ctrl+c")}},
		{"press punctuation", "press_key", map[string]any{"key": "/"}, []string{xdo("key -- slash")}},
		{"hotkey", "hotkey", map[string]any{"keys": []string{"ctrl", "shift", "t"}}, []string{xdo("key -- ctrl+shift+t")}},
		{"hotkey cmd", "hotkey", map[string]any{"keys": []string{"command", "c"}}, []string{xdo("key -- super+c")}},
		{"hotkey string", "hotkey", map[string]any{"keys": "alt+F4"}, []string{xdo("key -- alt+F4")}},
		{"sandbox keypress chord", "hotkey", map[string]any{"keys": []string{"ctrl+c"}}, []string{xdo("key -- ctrl+c")}},
		{"sandbox keypress key", "hotkey", map[string]any{"keys": []string{"enter"}}, []string{xdo("key -- Return")}},
		{"key down", "key_down", map[string]any{"key": "shift"}, []string{xdo("keydown -- shift")}},
		{"key up", "key_up", map[string]any{"key": "cmd"}, []string{xdo("keyup -- super")}},
		// cua-computer: x and y are wheel amounts at the pointer; positive y is up.
		{"scroll down", "scroll", map[string]any{"x": 0, "y": -3}, []string{xdo("click --repeat 3 5")}},
		{"scroll up", "scroll", map[string]any{"x": 0, "y": 2}, []string{xdo("click --repeat 2 4")}},
		{"scroll right", "scroll", map[string]any{"x": 4, "y": 0}, []string{xdo("click --repeat 4 7")}},
		{"scroll both", "scroll", map[string]any{"x": -1, "y": -2}, []string{xdo("click --repeat 2 5"), xdo("click --repeat 1 6")}},
		{"scroll capped", "scroll", map[string]any{"x": 0, "y": -500}, []string{xdo("click --repeat 50 5")}},
		{"scroll nothing", "scroll", map[string]any{"x": 0, "y": 0}, nil},
		// cua-sandbox: x and y are the pointer position and scroll_x, scroll_y the amounts.
		{"sandbox scroll down", "scroll", map[string]any{"x": 100, "y": 200, "scroll_x": 0, "scroll_y": -3},
			[]string{xdo("mousemove 100 200 click --repeat 3 5")}},
		{"sandbox scroll up", "scroll", map[string]any{"x": 100, "y": 200, "scroll_y": 3}, []string{xdo("mousemove 100 200 click --repeat 3 4")}},
		{"sandbox scroll both", "scroll", map[string]any{"x": 1, "y": 2, "scroll_x": -2, "scroll_y": 1},
			[]string{xdo("mousemove 1 2 click --repeat 1 4"), xdo("click --repeat 2 6")}},
		{"scroll_down", "scroll_down", map[string]any{"clicks": 5}, []string{xdo("click --repeat 5 5")}},
		{"scroll_up default", "scroll_up", nil, []string{xdo("click --repeat 1 4")}},
		{"scroll_direction", "scroll_direction", map[string]any{"direction": "Left", "clicks": 2}, []string{xdo("click --repeat 2 6")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt := newRuntime("a")
			ts, _ := newServer(t, rt, Options{})
			mustSucceed(t, on(t, ts, tc.command, tc.params))
			if got := execsAfter(rt, 0); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("execs = %q\nwant    %q", got, tc.want)
			}
		})
	}
}

func TestInvalidInputFails(t *testing.T) {
	cases := []struct {
		command string
		params  map[string]any
		want    string
	}{
		{"drag", map[string]any{"path": []any{}}, "Path is empty"},
		{"drag", map[string]any{"path": [][]int{{1}}}, "[x, y] pair"},
		{"drag", nil, "path is required"},
		{"drag_to", map[string]any{"x": 1}, "y is required"},
		{"move_cursor", map[string]any{"x": "left", "y": 1}, "x must be a number"},
		{"mouse_down", map[string]any{"button": "thumb"}, "unknown mouse button"},
		{"hotkey", map[string]any{"keys": []any{"ctrl", 3}}, "a list of strings"},
		{"hotkey", map[string]any{"keys": []string{}}, "no keys given"},
		{"scroll_direction", map[string]any{"direction": "sideways"}, "Invalid direction: sideways"},
		{"screenshot", map[string]any{"format": "gif"}, "unsupported screenshot format"},
		{"run_command", map[string]any{"command": "true", "timeout": -1}, "timeout must be more than 0"},
		{"run_command", map[string]any{"command": "true", "timeout": "NaN"}, "timeout must be a finite number"},
		{"left_click", map[string]any{"x": "Inf", "y": 1}, "x must be a finite number"},
		{"write_bytes", map[string]any{"path": "/x", "content_b64": "%%%"}, "not valid base64"},
		{"read_bytes", map[string]any{"path": "/x", "offset": -1}, "must not be negative"},
		{"read_text", map[string]any{"path": ""}, "path is required"},
	}
	for _, tc := range cases {
		rt := newRuntime("a")
		ts, _ := newServer(t, rt, Options{})
		mustFail(t, on(t, ts, tc.command, tc.params), tc.want)
		if got := execsAfter(rt, 0); len(got) != 0 {
			t.Errorf("%s %v ran %q", tc.command, tc.params, got)
		}
	}
}

func TestScreenshot(t *testing.T) {
	rt := newRuntime("a")
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("fake jpeg")...)
	base := rt.ExecFunc
	rt.ExecFunc = func(name, stdin string, argv []string) ([]byte, error) {
		if len(argv) > 4 && strings.Contains(argv[4], ".jpg") {
			return jpeg, nil
		}
		return base(name, stdin, argv)
	}
	ts, _ := newServer(t, rt, Options{})
	out := on(t, ts, "screenshot", nil)
	if out["image_data"] != b64(testPNG) || out["format"] != "png" {
		t.Fatalf("png screenshot = %v", out)
	}
	out = on(t, ts, "screenshot", map[string]any{"format": "jpg", "quality": 40})
	if out["image_data"] != b64(jpeg) || out["format"] != "jpeg" {
		t.Fatalf("jpeg screenshot = %v", out)
	}
	if last := execsAfter(rt, 0); !strings.Contains(last[len(last)-1], "scrot -o -z -q 40 ") {
		t.Fatalf("jpeg quality not passed: %q", last)
	}
}

func TestScreenSizeAndCursor(t *testing.T) {
	rt := newRuntime("a")
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		if strings.Contains(strings.Join(argv, " "), "getmouselocation") {
			return []byte("X=12\nY=34\nSCREEN=0\nWINDOW=1\n"), nil
		}
		return nil, nil
	}
	ts, _ := newServer(t, rt, Options{})
	size := on(t, ts, "get_screen_size", nil)["size"].(map[string]any)
	if size["width"] != float64(1280) || size["height"] != float64(800) {
		t.Fatalf("size = %v", size)
	}
	pos := on(t, ts, "get_cursor_position", nil)["position"].(map[string]any)
	if pos["x"] != float64(12) || pos["y"] != float64(34) {
		t.Fatalf("position = %v", pos)
	}
}

func TestRunCommand(t *testing.T) {
	rt := newRuntime("a")
	var argv []string
	var reply string
	var delay time.Duration
	rt.ExecFunc = func(_, _ string, a []string) ([]byte, error) {
		argv = a
		time.Sleep(delay)
		return []byte(reply), nil
	}
	ts, _ := newServer(t, rt, Options{})

	reply = "0 6 4\nhello\noops"
	out := on(t, ts, "run_command", map[string]any{"command": "echo hello; echo oops >&2"})
	if out["success"] != true || out["stdout"] != "hello\n" || out["stderr"] != "oops" || out["return_code"] != float64(0) {
		t.Fatalf("out = %v", out)
	}
	want := []string{"env", "DISPLAY=:1", "sh", "-c", shellScript, "sh", "echo hello; echo oops >&2", ""}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %q", argv)
	}

	// A failing command is still a successful run_command, as in Cua.
	reply = "3 0 4\nfail"
	out = on(t, ts, "run_command", map[string]any{"command": "exit 3", "timeout": 30})
	if out["success"] != true || out["return_code"] != float64(3) || out["stderr"] != "fail" || argv[len(argv)-1] != "30" {
		t.Fatalf("out = %v argv = %q", out, argv)
	}

	// The shell alias, and a command that ran out of time.
	reply, delay = "124 0 0\n", 30*time.Millisecond
	out = send(t, ts, "/d/a/cmd", keyed(nil), "shell", map[string]any{"command": "sleep 9", "timeout": 0.02})
	if out["success"] != false || out["return_code"] != float64(-1) || out["stderr"] != "Command timed out after 0.02s" {
		t.Fatalf("timeout out = %v", out)
	}

	// Exit 124 before the time is up is the command's own exit code.
	reply, delay = "124 0 0\n", 0
	if out = on(t, ts, "run_command", map[string]any{"command": "exit 124", "timeout": 60}); out["return_code"] != float64(124) {
		t.Fatalf("out = %v", out)
	}
}

func TestFileCommands(t *testing.T) {
	type call struct{ script, stdin string }
	cases := []struct {
		command string
		params  map[string]any
		reply   string
		want    map[string]any
		call    call
		args    []string
	}{
		{"file_exists", map[string]any{"path": "/etc/hosts"}, "yes\n", map[string]any{"exists": true}, call{fileExistsScript, ""}, []string{"/etc/hosts"}},
		{"file_exists", map[string]any{"path": "/nope"}, "no\n", map[string]any{"exists": false}, call{fileExistsScript, ""}, []string{"/nope"}},
		{"directory_exists", map[string]any{"path": "~"}, "yes\n", map[string]any{"exists": true}, call{dirExistsScript, ""}, []string{"/root"}},
		{"list_dir", map[string]any{"path": "~/Downloads"}, "b.txt\x00a dir\x00", map[string]any{"files": []any{"a dir", "b.txt"}},
			call{listDirScript, ""}, []string{"/root/Downloads"}},
		{"ls", map[string]any{"path": "/empty"}, "", map[string]any{"files": []any{}}, call{listDirScript, ""}, []string{"/empty"}},
		{"read_text", map[string]any{"path": "/tmp/a"}, "hi there", map[string]any{"content": "hi there"}, call{readScript, ""}, []string{"/tmp/a"}},
		{"write_text", map[string]any{"path": "/tmp/a", "content": "new"}, "", map[string]any{}, call{writeScript, "new"}, []string{"/tmp/a"}},
		{"read_bytes", map[string]any{"path": "/bin/x"}, "\x00\x01\xff", map[string]any{"content_b64": b64([]byte{0, 1, 0xff})}, call{readScript, ""}, []string{"/bin/x"}},
		{"read_bytes", map[string]any{"path": "/bin/x", "offset": 1048576, "length": 1048576}, "abc", map[string]any{"content_b64": b64([]byte("abc"))},
			call{readRangeScript, ""}, []string{"/bin/x", "1048576", "1048576"}},
		{"read_bytes", map[string]any{"path": "/bin/x", "offset": 5, "length": nil}, "z", map[string]any{"content_b64": b64([]byte("z"))},
			call{readRangeScript, ""}, []string{"/bin/x", "5", ""}},
		{"write_bytes", map[string]any{"path": "/tmp/b", "content_b64": b64([]byte{9, 8})}, "", map[string]any{}, call{writeScript, "\x09\x08"}, []string{"/tmp/b"}},
		{"write_bytes", map[string]any{"path": "/tmp/b", "content_b64": b64([]byte("more")), "append": true}, "", map[string]any{},
			call{appendScript, "more"}, []string{"/tmp/b"}},
		{"get_file_size", map[string]any{"path": "-odd"}, "42\n", map[string]any{"size": float64(42)}, call{sizeScript, ""}, []string{"./-odd"}},
		{"delete_file", map[string]any{"path": "/tmp/a"}, "", map[string]any{}, call{deleteScript, ""}, []string{"/tmp/a"}},
		{"mkdir", map[string]any{"path": "/tmp/d/e"}, "", map[string]any{}, call{mkdirScript, ""}, []string{"/tmp/d/e"}},
		{"delete_dir", map[string]any{"path": "/tmp/d"}, "", map[string]any{}, call{rmdirScript, ""}, []string{"/tmp/d"}},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			rt := newRuntime("a")
			var got call
			var gotArgs []string
			rt.ExecFunc = func(_, stdin string, argv []string) ([]byte, error) {
				if len(argv) < 4 || argv[0] != "sh" || argv[1] != "-c" || argv[3] != "sh" {
					t.Errorf("argv = %q", argv)
					return nil, nil
				}
				got, gotArgs = call{argv[2], stdin}, argv[4:]
				return []byte(tc.reply), nil
			}
			ts, _ := newServer(t, rt, Options{})
			out := on(t, ts, tc.command, tc.params)
			mustSucceed(t, out)
			delete(out, "success")
			if !reflect.DeepEqual(out, tc.want) {
				t.Errorf("answer = %v, want %v", out, tc.want)
			}
			if got != tc.call || !reflect.DeepEqual(gotArgs, tc.args) {
				t.Errorf("ran %q with stdin %q and args %q\nwant %q with stdin %q and args %q", got.script, got.stdin, gotArgs, tc.call.script, tc.call.stdin, tc.args)
			}
		})
	}
}

func TestGuestErrorsBecomeMessages(t *testing.T) {
	rt := newRuntime("a")
	rt.ExecFunc = func(_, _ string, _ []string) ([]byte, error) {
		return nil, &runner.ExitError{Name: "smolvm", Code: 1, Stderr: []byte("cat: /nope: No such file or directory\n")}
	}
	ts, _ := newServer(t, rt, Options{})
	out := on(t, ts, "read_text", map[string]any{"path": "/nope"})
	if out["success"] != false || out["error"] != "cat: /nope: No such file or directory" {
		t.Fatalf("out = %v", out)
	}
}
