package cuaserver

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/host"
)

var bearer = map[string]string{"Authorization": "Bearer " + token}

func decodeVM(t *testing.T, body string) vm {
	t.Helper()
	var v vm
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("not a VM: %v: %s", err, body)
	}
	return v
}

// TestVMLifecycle follows what cua-sandbox's Sandbox.ephemeral and cua-computer's cloud
// provider do: create, poll, connect through the endpoint, stop, start, delete.
func TestVMLifecycle(t *testing.T) {
	rt := newRuntime()
	ts, rec := newServer(t, rt, Options{NamePrefix: "cua-test-"})
	u, _ := url.Parse(ts.URL)
	wantPort, _ := strconv.Atoi(u.Port())

	res, body := request(t, ts, "POST", "/v1/vms", bearer, `{"os":"linux","region":"us-east-1","configuration":"small"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("create: %d %s", res.StatusCode, body)
	}
	created := decodeVM(t, body)
	if !strings.HasPrefix(created.Name, "cua-test-") || len(created.Name) != len("cua-test-")+6 || created.Status != "running" {
		t.Fatalf("created = %+v", created)
	}
	if created.Host != "127.0.0.1" || len(created.Endpoints) != 1 || created.Endpoints[0] != (endpoint{Name: "api", Host: "127.0.0.1", Port: wantPort}) {
		t.Fatalf("created VM does not point back at this server: %+v", created)
	}
	name := created.Name

	_, body = request(t, ts, "GET", "/v1/vms/"+name, bearer, "")
	if got := decodeVM(t, body); got.Name != name || got.Status != "running" || got.OS != "linux" || got.AdfState != "running" {
		t.Fatalf("get = %+v", got)
	}
	_, body = request(t, ts, "GET", "/v1/vms", bearer, "")
	var list []vm
	if err := json.Unmarshal([]byte(body), &list); err != nil || len(list) != 1 || list[0].Name != name {
		t.Fatalf("list = %s", body)
	}

	// cua-sandbox then sends commands to the endpoint, naming the VM in X-Container-Name.
	mustSucceed(t, send(t, ts, "/cmd", map[string]string{"X-API-Key": token, "Authorization": "Bearer " + token, "X-Container-Name": name}, "get_screen_size", nil))

	steps := []struct {
		action string
		code   int
		status string // status the client sees afterwards
		state  string // adf's own state
	}{
		{"stop", http.StatusAccepted, "stopped", "stopped"},
		{"stop", http.StatusAccepted, "stopped", "stopped"}, // stopping a stopped desktop is fine
		{"start", http.StatusNoContent, "running", "running"},
		{"suspend", http.StatusAccepted, "running", "paused"}, // paused desktops wake on the next command
		{"run", http.StatusNoContent, "running", "running"},
		{"restart", http.StatusAccepted, "running", "running"},
	}
	for _, s := range steps {
		res, body := request(t, ts, "POST", "/v1/vms/"+name+"/"+s.action, bearer, "")
		if res.StatusCode != s.code {
			t.Fatalf("%s: %d %s", s.action, res.StatusCode, body)
		}
		_, body = request(t, ts, "GET", "/v1/vms/"+name, bearer, "")
		if got := decodeVM(t, body); got.Status != s.status || got.AdfState != s.state {
			t.Fatalf("after %s: %+v", s.action, got)
		}
	}

	res, body = request(t, ts, "DELETE", "/v1/vms/"+name, bearer, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("delete: %d %s", res.StatusCode, body)
	}
	if res, _ := request(t, ts, "GET", "/v1/vms/"+name, bearer, ""); res.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted VM: status %d", res.StatusCode)
	}

	var actions []string
	for _, e := range rec.all() {
		if e.Desktop == name && strings.HasSuffix(e.Action, "_desktop") {
			actions = append(actions, e.Action)
		}
	}
	want := "create_desktop stop_desktop stop_desktop start_desktop pause_desktop start_desktop restart_desktop delete_desktop"
	if strings.Join(actions, " ") != want {
		t.Fatalf("recorded %q\nwant     %q", actions, want)
	}
}

func TestCreateVMWithNameAndErrors(t *testing.T) {
	rt := newRuntime("taken")
	ts, _ := newServer(t, rt, Options{})
	res, body := request(t, ts, "POST", "/v1/vms", bearer, `{"name":"cua-test-named"}`)
	if res.StatusCode != http.StatusOK || decodeVM(t, body).Name != "cua-test-named" {
		t.Fatalf("named create: %d %s", res.StatusCode, body)
	}
	res, body = request(t, ts, "POST", "/v1/vms", bearer, "")
	if v := decodeVM(t, body); res.StatusCode != http.StatusOK || !strings.HasPrefix(v.Name, "cua-") {
		t.Fatalf("create with no body: %d %s", res.StatusCode, body)
	}
	cases := []struct {
		method, path, body string
		code               int
		want               string
	}{
		{"POST", "/v1/vms", `{"name":"taken"}`, http.StatusConflict, "already exists"},
		{"POST", "/v1/vms", `{"os":"windows"}`, http.StatusBadRequest, "adf desktops run Linux"},
		{"POST", "/v1/vms", `{"name":"Bad Name"}`, http.StatusBadRequest, "invalid desktop name"},
		{"POST", "/v1/vms", `{`, http.StatusBadRequest, "Invalid JSON body"},
		{"GET", "/v1/vms/missing", "", http.StatusNotFound, "no desktop named"},
		{"GET", "/v1/vms/Not_Valid", "", http.StatusNotFound, "no desktop named"},
		{"DELETE", "/v1/vms/missing", "", http.StatusNotFound, "no desktop named"},
		{"POST", "/v1/vms/missing/start", "", http.StatusNotFound, "no desktop named"},
		{"POST", "/v1/vms/taken/snapshot", "", http.StatusNotFound, "unknown action"},
	}
	for _, tc := range cases {
		res, body := request(t, ts, tc.method, tc.path, bearer, tc.body)
		if res.StatusCode != tc.code || !strings.Contains(body, tc.want) || !strings.Contains(body, `"detail"`) {
			t.Errorf("%s %s %s: %d %s; want %d with %q", tc.method, tc.path, tc.body, res.StatusCode, body, tc.code, tc.want)
		}
	}
}

func TestCreateVMWithoutRoomIs503(t *testing.T) {
	rt := newRuntime()
	m := fleet.NewManager(rt, nil)
	m.Settle = 0
	m.Host = func() (host.Stats, error) { return host.Stats{AvailableMemBytes: 1 << 20}, nil }
	ts := newServerFor(t, m)
	res, body := request(t, ts, "POST", "/v1/vms", bearer, "{}")
	if res.StatusCode != http.StatusServiceUnavailable || !strings.Contains(body, "not enough room") {
		t.Fatalf("%d %s", res.StatusCode, body)
	}
}

func TestVMHostFollowsTheRequest(t *testing.T) {
	ts, _ := newServer(t, newRuntime("a"), Options{})
	// A client that reached the server as desk.example:9000 gets links to that address.
	req, _ := http.NewRequest("GET", ts.URL+"/v1/vms/a", nil)
	req.Host = "desk.example:9000"
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var v vm
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	if v.Host != "desk.example" || v.Endpoints[0] != (endpoint{Name: "api", Host: "desk.example", Port: 9000}) {
		t.Fatalf("vm = %+v", v)
	}
}
