package cuaserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/activity"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet/fleettest"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

const token = "s3cret"

var testPNG = append([]byte("\x89PNG\r\n\x1a\n"), []byte("fake png")...)

// newRuntime returns a fake with a warm, provisioned template and a running desktop for
// each name. Screenshots return testPNG.
func newRuntime(names ...string) *fleettest.Fake {
	rt := fleettest.New()
	rt.Machines[fleet.TemplateName] = &smolvm.Machine{Name: fleet.TemplateName, State: "running", Branchable: true,
		Labels: map[string]string{fleet.LabelRole: fleet.RoleTemplate}}
	for _, n := range names {
		addDesktop(rt, n, "running")
	}
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		switch {
		case argv[0] == "cat":
			return []byte(image.Version), nil
		case len(argv) > 4 && argv[2] == "sh" && strings.Contains(argv[4], "scrot"):
			return testPNG, nil
		}
		return nil, nil
	}
	return rt
}

func addDesktop(rt *fleettest.Fake, name, state string) {
	rt.Machines[fleet.DesktopPrefix+name] = &smolvm.Machine{Name: fleet.DesktopPrefix + name, State: state,
		Labels: map[string]string{fleet.LabelRole: fleet.RoleDesktop}}
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

func newServer(t *testing.T, rt *fleettest.Fake, opts Options) (*httptest.Server, *memRecorder) {
	t.Helper()
	m := fleet.NewManager(rt, nil)
	m.Poll, m.Settle = time.Millisecond, 0
	rec := &memRecorder{}
	opts.Token, opts.Activity, opts.Version = token, rec, "1.2.3"
	ts := httptest.NewServer(New(m, opts).Handler())
	t.Cleanup(ts.Close)
	return ts, rec
}

func newServerFor(t *testing.T, m *fleet.Manager) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(New(m, Options{Token: token}).Handler())
	t.Cleanup(ts.Close)
	return ts
}

func request(t *testing.T, ts *httptest.Server, method, path string, hdr map[string]string, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res, string(b)
}

func keyed(extra map[string]string) map[string]string {
	h := map[string]string{"X-API-Key": token}
	for k, v := range extra {
		h[k] = v
	}
	return h
}

// send posts one command the way cua-computer does and decodes its single data frame.
func send(t *testing.T, ts *httptest.Server, path string, hdr map[string]string, command string, params map[string]any) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"command": command, "params": params})
	res, text := request(t, ts, "POST", path, hdr, string(body))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s: status %d: %s", command, res.StatusCode, text)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("%s: content type %q", command, ct)
	}
	if !strings.HasPrefix(text, "data: ") || !strings.HasSuffix(text, "\n\n") || strings.Count(text, "data: ") != 1 {
		t.Fatalf("%s: not a single data frame: %q", command, text)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(text, "data: "))), &out); err != nil {
		t.Fatalf("%s: frame is not JSON: %v: %q", command, err, text)
	}
	return out
}

// on runs a command on desktop a and returns its answer.
func on(t *testing.T, ts *httptest.Server, command string, params map[string]any) map[string]any {
	t.Helper()
	return send(t, ts, "/cmd", keyed(map[string]string{"X-Container-Name": "a"}), command, params)
}

func mustSucceed(t *testing.T, out map[string]any) {
	t.Helper()
	if out["success"] != true {
		t.Fatalf("command failed: %v", out)
	}
}

func mustFail(t *testing.T, out map[string]any, want string) {
	t.Helper()
	if out["success"] != false || !strings.Contains(out["error"].(string), want) {
		t.Fatalf("want a failure mentioning %q, got %v", want, out)
	}
}

// execsAfter returns the commands run in desktops after the first n calls of the log.
func execsAfter(rt *fleettest.Fake, n int) []string {
	var out []string
	for _, c := range rt.Calls()[n:] {
		if strings.HasPrefix(c, "exec adf-") {
			out = append(out, c)
		}
	}
	return out
}

func TestRootAndStatusAreOpen(t *testing.T) {
	ts, _ := newServer(t, newRuntime("a"), Options{})
	for _, path := range []string{"/", "/status", "/d/a/status"} {
		res, body := request(t, ts, "GET", path, nil, "")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d: %s", path, res.StatusCode, body)
		}
	}
	_, body := request(t, ts, "GET", "/status", nil, "")
	var status map[string]any
	if err := json.Unmarshal([]byte(body), &status); err != nil || status["status"] != "ok" || status["os_type"] != "linux" {
		t.Fatalf("status = %s", body)
	}
}

func TestAuthAcceptsAPIKeyOrBearer(t *testing.T) {
	ts, _ := newServer(t, newRuntime("a"), Options{})
	cmd := `{"command":"version"}`
	cases := []struct {
		name string
		hdr  map[string]string
		want int
	}{
		{"none", nil, http.StatusUnauthorized},
		{"wrong key", map[string]string{"X-API-Key": "nope"}, http.StatusUnauthorized},
		{"wrong bearer", map[string]string{"Authorization": "Bearer nope"}, http.StatusUnauthorized},
		{"basic scheme", map[string]string{"Authorization": "Basic " + token}, http.StatusUnauthorized},
		{"api key", map[string]string{"X-API-Key": token}, http.StatusOK},
		{"bearer", map[string]string{"Authorization": "Bearer " + token}, http.StatusOK},
		{"proxy bearer and key", map[string]string{"Authorization": "Bearer proxy", "X-API-Key": token}, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, r := range []struct{ method, path, body string }{
				{"POST", "/cmd", cmd}, {"POST", "/d/a/cmd", cmd}, {"GET", "/commands", ""}, {"GET", "/v1/vms", ""},
			} {
				res, body := request(t, ts, r.method, r.path, tc.hdr, r.body)
				if res.StatusCode != tc.want {
					t.Fatalf("%s %s: status %d, want %d: %s", r.method, r.path, res.StatusCode, tc.want, body)
				}
				if tc.want == http.StatusUnauthorized && !strings.Contains(body, `"detail"`) {
					t.Fatalf("401 without a detail: %s", body)
				}
			}
		})
	}
}

func TestEnvelope(t *testing.T) {
	ts, _ := newServer(t, newRuntime("a"), Options{})
	out := on(t, ts, "version", nil)
	if out["success"] != true || out["protocol"] != float64(1) || out["package"] != "adf-1.2.3" {
		t.Fatalf("version = %v", out)
	}
	// Unknown parameters are ignored, as Cua's server filters them to the handler's signature.
	mustSucceed(t, on(t, ts, "get_screen_size", map[string]any{"unexpected": true}))
	// A command with no params object at all is fine.
	res, text := request(t, ts, "POST", "/cmd", keyed(map[string]string{"X-Container-Name": "a"}), `{"command":"get_screen_size"}`)
	if res.StatusCode != http.StatusOK || !strings.Contains(text, `"width":1280`) {
		t.Fatalf("status %d: %s", res.StatusCode, text)
	}
}

func TestBadRequestsAreHTTP400(t *testing.T) {
	ts, _ := newServer(t, newRuntime("a"), Options{})
	cases := map[string]struct{ body, want string }{
		"unknown command": {`{"command":"scrol"}`, "Unknown command: scrol. Did you mean: screenshot, scroll, scroll_direction"},
		"no command":      {`{"params":{}}`, "Command is required"},
		"not json":        {`not json`, "Invalid JSON body"},
		"params not map":  {`{"command":"version","params":[1]}`, "Invalid JSON body"},
	}
	for name, tc := range cases {
		res, body := request(t, ts, "POST", "/cmd", keyed(nil), tc.body)
		if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, tc.want) {
			t.Errorf("%s: status %d body %s, want 400 with %q", name, res.StatusCode, body, tc.want)
		}
	}
}

func TestFailedActionIsHTTP200(t *testing.T) {
	rt := newRuntime("a")
	ts, rec := newServer(t, rt, Options{})
	mustFail(t, on(t, ts, "left_click", map[string]any{"x": 5000, "y": 10}), "outside the 1280x800 screen")
	mustFail(t, on(t, ts, "left_click", map[string]any{"x": 5}), "give both x and y")
	mustFail(t, on(t, ts, "press_key", map[string]any{"key": "not a key!"}), "Unknown key: not a key!")
	mustFail(t, on(t, ts, "type_text", nil), "text is required")
	if got := execsAfter(rt, 0); len(got) != 0 {
		t.Fatalf("invalid input ran commands: %q", got)
	}
	ev := rec.all()
	if len(ev) != 4 || ev[0].OK || ev[0].Error == "" || ev[0].Source != activity.SourceCua {
		t.Fatalf("events = %+v", ev)
	}
}

func TestCommandsNotSupportedYetFailClearly(t *testing.T) {
	ts, _ := newServer(t, newRuntime("a"), Options{})
	for _, name := range []string{"copy_to_clipboard", "set_clipboard", "launch", "open", "get_current_window_id", "maximize_window"} {
		mustFail(t, on(t, ts, name, nil), name+" is not supported by adf cua-server yet")
	}
	for _, path := range []string{"/ws", "/d/a/ws", "/pty", "/pty/12/stdin", "/playwright_exec"} {
		res, body := request(t, ts, "POST", path, keyed(nil), "{}")
		if res.StatusCode != http.StatusNotImplemented || !strings.Contains(body, "does not support") {
			t.Errorf("%s: status %d: %s", path, res.StatusCode, body)
		}
	}
}

func TestAliases(t *testing.T) {
	rt := newRuntime("a")
	ts, rec := newServer(t, rt, Options{})
	mustSucceed(t, on(t, ts, "click", map[string]any{"x": 1, "y": 2}))
	mustSucceed(t, on(t, ts, "key", map[string]any{"key": "enter"}))
	mustSucceed(t, on(t, ts, "type", map[string]any{"text": "x"}))
	want := []string{
		"exec adf-a env DISPLAY=:1 xdotool mousemove 1 2 click 1",
		"exec adf-a env DISPLAY=:1 xdotool key -- Return",
		"exec adf-a env DISPLAY=:1 xdotool type --delay 12 -- x",
	}
	if got := execsAfter(rt, 0); !reflect.DeepEqual(got, want) {
		t.Fatalf("execs = %q\nwant    %q", got, want)
	}
	if ev := rec.all(); ev[0].Action != "left_click" || ev[1].Action != "press_key" || ev[2].Action != "type_text" {
		t.Fatalf("aliases recorded as %+v", ev)
	}
}

func TestDesktopResolution(t *testing.T) {
	rt := newRuntime("a", "b", "c")
	ts, _ := newServer(t, rt, Options{Desktop: "c"})
	cases := []struct {
		name string
		path string
		hdr  map[string]string
		want string
	}{
		{"path wins", "/d/a/cmd", map[string]string{"X-Container-Name": "b", "X-VM-Name": "c"}, "a"},
		{"container header", "/cmd", map[string]string{"X-Container-Name": "b", "X-VM-Name": "a"}, "b"},
		{"vm header", "/cmd", map[string]string{"X-VM-Name": "a"}, "a"},
		{"--desktop", "/cmd", nil, "c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := len(rt.Calls())
			mustSucceed(t, send(t, ts, tc.path, keyed(tc.hdr), "left_click", nil))
			got := execsAfter(rt, n)
			if len(got) != 1 || !strings.HasPrefix(got[0], "exec adf-"+tc.want+" ") {
				t.Fatalf("ran %q, want it in %s", got, tc.want)
			}
		})
	}
}

func TestOnlyDesktopIsTheDefault(t *testing.T) {
	rt := newRuntime()
	ts, _ := newServer(t, rt, Options{})
	mustFail(t, send(t, ts, "/cmd", keyed(nil), "left_click", nil), "no desktops yet")
	addDesktop(rt, "solo", "running")
	mustSucceed(t, send(t, ts, "/cmd", keyed(nil), "left_click", nil))
	if got := execsAfter(rt, 0); len(got) != 1 || !strings.HasPrefix(got[0], "exec adf-solo ") {
		t.Fatalf("execs = %q", got)
	}
	addDesktop(rt, "other", "running")
	mustFail(t, send(t, ts, "/cmd", keyed(nil), "left_click", nil), "which desktop? set X-Container-Name")
	mustFail(t, send(t, ts, "/cmd", keyed(map[string]string{"X-Container-Name": "missing"}), "left_click", nil),
		"create it with 'adf create missing'")
	mustFail(t, send(t, ts, "/cmd", keyed(map[string]string{"X-Container-Name": "Bad_Name"}), "left_click", nil), "invalid desktop name")
	// version needs no desktop at all, so readiness probes work before one exists.
	mustSucceed(t, send(t, ts, "/cmd", keyed(nil), "version", nil))
}

func TestActionsWakeAsleepDesktops(t *testing.T) {
	for _, state := range []string{"stopped", "paused"} {
		t.Run(state, func(t *testing.T) {
			rt := newRuntime()
			addDesktop(rt, "a", state)
			ts, _ := newServer(t, rt, Options{})
			// The screen size is constant: answering it must not start the desktop.
			mustSucceed(t, on(t, ts, "get_screen_size", nil))
			if len(rt.Calls()) != 0 {
				t.Fatalf("get_screen_size touched the desktop: %q", rt.Calls())
			}
			mustSucceed(t, on(t, ts, "write_text", map[string]any{"path": "/tmp/x", "content": "hi"}))
			calls := strings.Join(rt.Calls(), "\n")
			wake := map[string]string{"stopped": "start adf-a branchable=false", "paused": "resume adf-a"}[state]
			if !strings.Contains(calls, wake) || !strings.HasSuffix(calls, "exec adf-a sh -c "+writeScript+" sh /tmp/x") {
				t.Fatalf("calls:\n%s", calls)
			}
		})
	}
}

func TestActivityRecordsCuaCommands(t *testing.T) {
	ts, rec := newServer(t, newRuntime("a"), Options{})
	mustSucceed(t, on(t, ts, "left_click", map[string]any{"x": 10, "y": 20}))
	mustSucceed(t, on(t, ts, "type_text", map[string]any{"text": "hunter2"}))
	mustSucceed(t, on(t, ts, "scroll", map[string]any{"x": 0, "y": -3}))
	on(t, ts, "run_command", map[string]any{"command": "true"})
	want := []struct{ action, detail string }{
		{"left_click", "(10, 20)"}, {"type_text", "7 characters"}, {"scroll", "down 3"}, {"run_command", "true"},
	}
	ev := rec.all()
	if len(ev) != len(want) {
		t.Fatalf("events = %+v", ev)
	}
	for i, w := range want {
		e := ev[i]
		if e.Desktop != "a" || e.Source != activity.SourceCua || e.Action != w.action || e.Detail != w.detail {
			t.Errorf("event %d = %+v, want %s %q", i, e, w.action, w.detail)
		}
	}
	// run_command fails here because the fake shell prints nothing, which is recorded too.
	if ev[3].OK || !strings.Contains(ev[3].Error, "unexpected output") {
		t.Errorf("run_command event = %+v", ev[3])
	}
}

func TestCommandsListing(t *testing.T) {
	ts, _ := newServer(t, newRuntime("a"), Options{})
	_, body := request(t, ts, "GET", "/commands", keyed(nil), "")
	var out struct {
		Commands map[string]struct {
			Params  []param  `json:"params"`
			Aliases []string `json:"aliases"`
		} `json:"commands"`
		Aliases map[string]string `json:"aliases"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	click := out.Commands["left_click"]
	if !reflect.DeepEqual(click.Aliases, []string{"click", "tap"}) || len(click.Params) != 3 || click.Params[0].Name != "x" || click.Params[0].Required {
		t.Fatalf("left_click = %+v", click)
	}
	if p := out.Commands["move_cursor"].Params; len(p) != 2 || !p[0].Required {
		t.Fatalf("move_cursor params = %+v", p)
	}
	if out.Aliases["rm"] != "delete_file" {
		t.Fatalf("aliases = %v", out.Aliases)
	}
	if _, listed := out.Commands["copy_to_clipboard"]; listed {
		t.Fatal("commands that are not supported yet must not be listed")
	}
	var names []string
	for n := range out.Commands {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, want := range []string{"drag", "get_accessibility_tree", "hotkey", "key_down", "read_bytes", "run_command", "screenshot", "write_bytes"} {
		if i := sort.SearchStrings(names, want); i == len(names) || names[i] != want {
			t.Errorf("%s is not listed", want)
		}
	}
}

func TestParityStubs(t *testing.T) {
	ts, _ := newServer(t, newRuntime("a"), Options{})
	if out := on(t, ts, "get_desktop_environment", nil); out["environment"] != "openbox" {
		t.Fatalf("environment = %v", out)
	}
	tree := on(t, ts, "get_accessibility_tree", nil)
	if root, _ := tree["tree"].(map[string]any); tree["success"] != true || root["role"] != "Window" || root["title"] != "Linux Window" {
		t.Fatalf("tree = %v", tree)
	}
	if out := on(t, ts, "find_element", map[string]any{"role": "button"}); out["success"] != false || out["message"] != "Element search not supported on Linux" {
		t.Fatalf("find_element = %v", out)
	}
}

func TestLargeBodiesAreAccepted(t *testing.T) {
	rt := newRuntime("a")
	var got int
	rt.ExecFunc = func(_, stdin string, _ []string) ([]byte, error) { got = len(stdin); return nil, nil }
	ts, _ := newServer(t, rt, Options{})
	data := bytes.Repeat([]byte{0xAB}, 5<<20) // cua-computer sends up to 5 MB in one write_bytes
	mustSucceed(t, on(t, ts, "write_bytes", map[string]any{"path": "/tmp/big", "content_b64": b64(data)}))
	if got != len(data) {
		t.Fatalf("desktop received %d bytes, want %d", got, len(data))
	}
}
