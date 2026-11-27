package console

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
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

const token = "t0ken"

var testJPEG = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("fake jpeg")...)

func isCapture(argv []string) bool {
	return len(argv) > 4 && argv[2] == "sh" && strings.Contains(argv[4], ".jpg")
}

// newRuntime returns a fake with a warm template and one running desktop "a".
func newRuntime() *fleettest.Fake {
	rt := fleettest.New()
	rt.Machines[fleet.TemplateName] = &smolvm.Machine{Name: fleet.TemplateName, State: "running", Branchable: true,
		Labels: map[string]string{fleet.LabelRole: fleet.RoleTemplate}}
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{fleet.LabelRole: fleet.RoleDesktop}}
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		switch {
		case argv[0] == "cat":
			return []byte(image.Version), nil
		case isCapture(argv):
			return testJPEG, nil
		}
		return nil, nil
	}
	return rt
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

func newServer(t *testing.T, rt *fleettest.Fake, opts Options) (*Server, *httptest.Server) {
	t.Helper()
	m := fleet.NewManager(rt, nil)
	m.Poll, m.Settle = time.Millisecond, 0
	opts.Token = token
	s := New(m, opts)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts
}

func do(t *testing.T, ts *httptest.Server, method, path, body string, auth bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func decode(t *testing.T, res *http.Response, v any) {
	t.Helper()
	if err := json.NewDecoder(res.Body).Decode(v); err != nil {
		t.Fatal(err)
	}
}

func countCalls(rt *fleettest.Fake, substr string) int {
	n := 0
	for _, c := range rt.Calls() {
		if strings.Contains(c, substr) {
			n++
		}
	}
	return n
}

func TestAPIRequiresToken(t *testing.T) {
	_, ts := newServer(t, newRuntime(), Options{})
	if res := do(t, ts, "GET", "/api/desktops", "", false); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestStaticPageNeedsNoToken(t *testing.T) {
	_, ts := newServer(t, newRuntime(), Options{})
	res := do(t, ts, "GET", "/", "", false)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), "<title>adf console</title>") {
		t.Fatalf("status=%d body=%q", res.StatusCode, body)
	}
}

func TestStatus(t *testing.T) {
	_, ts := newServer(t, newRuntime(), Options{})
	var out struct{ Template string }
	decode(t, do(t, ts, "GET", "/api/status", "", true), &out)
	if out.Template != "running" {
		t.Fatalf("template = %q", out.Template)
	}
}

func TestListIncludesStatsAfterRefresh(t *testing.T) {
	rt := newRuntime()
	rt.StatsFunc = func(string) (smolvm.Stats, error) {
		return smolvm.Stats{CPUPercent: 12.5, Memory: smolvm.MemoryStats{UsedBytes: 100, TotalBytes: 400}}, nil
	}
	s, ts := newServer(t, rt, Options{})
	s.refreshStats(context.Background())
	var out struct {
		Desktops []struct {
			Name  string
			State string
			Stats *struct {
				CPUPercent    float64 `json:"cpu_percent"`
				MemUsedBytes  int64   `json:"mem_used_bytes"`
				MemTotalBytes int64   `json:"mem_total_bytes"`
			}
		}
	}
	decode(t, do(t, ts, "GET", "/api/desktops", "", true), &out)
	if len(out.Desktops) != 1 || out.Desktops[0].Name != "a" || out.Desktops[0].Stats == nil ||
		out.Desktops[0].Stats.CPUPercent != 12.5 || out.Desktops[0].Stats.MemTotalBytes != 400 {
		t.Fatalf("desktops = %+v", out.Desktops)
	}
}

func TestCreateDesktop(t *testing.T) {
	rt := newRuntime()
	rec := &memRecorder{}
	_, ts := newServer(t, rt, Options{Recorder: rec})
	if res := do(t, ts, "POST", "/api/desktops", `{"name":"b"}`, true); res.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if _, ok := rt.Machines["adf-b"]; !ok {
		t.Fatal("desktop b was not created")
	}
	ev := rec.all()
	if len(ev) != 1 || ev[0].Action != "create_desktop" || ev[0].Source != activity.SourceConsole || !ev[0].OK {
		t.Fatalf("events = %+v", ev)
	}
}

func TestCreateErrors(t *testing.T) {
	rt := newRuntime()
	_, ts := newServer(t, rt, Options{})
	if res := do(t, ts, "POST", "/api/desktops", `{"name":"Bad Name"}`, true); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad name: status = %d", res.StatusCode)
	}
	if countCalls(rt, "cat /etc/adf-image-version") != 0 {
		t.Fatal("an invalid name must not touch the template")
	}
	if res := do(t, ts, "POST", "/api/desktops", `{"name":"a"}`, true); res.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate: status = %d", res.StatusCode)
	}
	if res := do(t, ts, "POST", "/api/desktops", `not json`, true); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad body: status = %d", res.StatusCode)
	}
}

func TestDeleteDesktop(t *testing.T) {
	rt := newRuntime()
	_, ts := newServer(t, rt, Options{})
	if res := do(t, ts, "DELETE", "/api/desktops/a", "", true); res.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if _, ok := rt.Machines["adf-a"]; ok {
		t.Fatal("desktop still exists")
	}
	if res := do(t, ts, "DELETE", "/api/desktops/a", "", true); res.StatusCode != http.StatusNotFound {
		t.Fatalf("second delete: status = %d", res.StatusCode)
	}
}

func TestScreenReturnsJPEG(t *testing.T) {
	_, ts := newServer(t, newRuntime(), Options{})
	res := do(t, ts, "GET", "/api/desktops/a/screen", "", true)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/jpeg" || !bytes.Equal(body, testJPEG) {
		t.Fatalf("status=%d type=%q body=%q", res.StatusCode, res.Header.Get("Content-Type"), body)
	}
	if res.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("frames must not be cached by the browser")
	}
}

func TestConcurrentScreenRequestsShareOneCapture(t *testing.T) {
	rt := newRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		if isCapture(argv) {
			time.Sleep(50 * time.Millisecond)
			return testJPEG, nil
		}
		return nil, nil
	}
	_, ts := newServer(t, rt, Options{FrameMaxAge: time.Second})
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := do(t, ts, "GET", "/api/desktops/a/screen", "", true)
			if res.StatusCode != http.StatusOK {
				t.Errorf("status = %d", res.StatusCode)
			}
		}()
	}
	wg.Wait()
	if n := countCalls(rt, "scrot"); n != 1 {
		t.Fatalf("captured %d times, want 1", n)
	}
}

func TestInputClickIsAppliedAndRecorded(t *testing.T) {
	rt := newRuntime()
	rec := &memRecorder{}
	_, ts := newServer(t, rt, Options{Recorder: rec})
	if res := do(t, ts, "POST", "/api/desktops/a/input", `{"kind":"click","x":10,"y":20}`, true); res.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if countCalls(rt, "exec adf-a env DISPLAY=:1 xdotool mousemove 10 20 click 1") != 1 {
		t.Fatalf("calls = %q", rt.Calls())
	}
	ev := rec.all()
	if len(ev) != 1 || ev[0].Action != "click" || ev[0].Detail != "(10, 20)" || ev[0].Source != activity.SourceConsole {
		t.Fatalf("events = %+v", ev)
	}
}

func TestInputInvalidPointIs400WithoutExec(t *testing.T) {
	rt := newRuntime()
	_, ts := newServer(t, rt, Options{})
	if res := do(t, ts, "POST", "/api/desktops/a/input", `{"kind":"click","x":5000,"y":1}`, true); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if countCalls(rt, "xdotool") != 0 {
		t.Fatalf("calls = %q", rt.Calls())
	}
}

func TestInputKeys(t *testing.T) {
	rt := newRuntime()
	_, ts := newServer(t, rt, Options{})
	do(t, ts, "POST", "/api/desktops/a/input", `{"kind":"key","key":"l","ctrl":true}`, true)
	do(t, ts, "POST", "/api/desktops/a/input", `{"kind":"key","key":"a"}`, true)
	if countCalls(rt, "xdotool key -- ctrl+l") != 1 || countCalls(rt, "xdotool type --delay 12 -- a") != 1 {
		t.Fatalf("calls = %q", rt.Calls())
	}
}

func TestBrowserKey(t *testing.T) {
	cases := []struct {
		key                    string
		ctrl, alt, shift, meta bool
		combo, text            string
		bad                    bool
	}{
		{key: "a", text: "a"},
		{key: "A", shift: true, text: "A"},
		{key: "Enter", combo: "Return"},
		{key: " ", combo: "space"},
		{key: "l", ctrl: true, combo: "ctrl+l"},
		{key: "T", ctrl: true, shift: true, combo: "ctrl+shift+t"},
		{key: "Tab", shift: true, combo: "shift+Tab"},
		{key: "ArrowLeft", alt: true, combo: "alt+Left"},
		{key: "PageDown", combo: "Page_Down"},
		{key: "F5", combo: "F5"},
		{key: "l", meta: true, combo: "super+l"},
		{key: "Dead", bad: true},
		{key: "é", ctrl: true, bad: true},
		{key: "Shift", bad: true},
	}
	for _, tc := range cases {
		combo, text, err := browserKey(tc.key, tc.ctrl, tc.alt, tc.shift, tc.meta)
		if tc.bad {
			if err == nil {
				t.Errorf("%q: expected an error", tc.key)
			}
			continue
		}
		if err != nil || combo != tc.combo || text != tc.text {
			t.Errorf("%q: combo=%q text=%q err=%v, want combo=%q text=%q", tc.key, combo, text, err, tc.combo, tc.text)
		}
	}
}

func TestActivityFeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "activity.jsonl")
	log := &activity.Log{Path: path}
	log.Record(activity.Event{Desktop: "a", Action: "click", OK: true})
	log.Record(activity.Event{Desktop: "a", Action: "key", OK: true})
	_, ts := newServer(t, newRuntime(), Options{ActivityPath: path})
	var first struct {
		Events []activity.Event
		Cursor int64
	}
	decode(t, do(t, ts, "GET", "/api/activity", "", true), &first)
	if len(first.Events) != 2 || first.Cursor == 0 {
		t.Fatalf("first = %+v", first)
	}
	log.Record(activity.Event{Desktop: "a", Action: "scroll", OK: true})
	var next struct {
		Events []activity.Event
		Cursor int64
	}
	decode(t, do(t, ts, "GET", "/api/activity?cursor="+itoa(first.Cursor), "", true), &next)
	if len(next.Events) != 1 || next.Events[0].Action != "scroll" || next.Cursor <= first.Cursor {
		t.Fatalf("next = %+v", next)
	}
	if res := do(t, ts, "GET", "/api/activity?cursor=-1", "", true); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("negative cursor: status = %d", res.StatusCode)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
