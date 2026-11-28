package cuaserver

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
)

// vm is a desktop as Cua's platform API described a VM. host and endpoints point back at
// this server, which serves every desktop: cua-computer reads host, and cua-sandbox builds
// http://HOST:PORT from the "api" endpoint and names the desktop in X-Container-Name.
type vm struct {
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	OS        string     `json:"os"`
	OSType    string     `json:"os_type"`
	Host      string     `json:"host"`
	Endpoints []endpoint `json:"endpoints"`
	Password  string     `json:"password"` // empty: there is no VNC; clients then build no VNC link
	AdfState  string     `json:"adf_state"`
}

type endpoint struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

// cuaStatus maps an adf state to a Cua VM status. A paused desktop counts as running:
// adf pauses idle desktops on its own and the next command resumes them, and Cua clients
// wait for "running" before they send anything.
func cuaStatus(state string) string {
	switch state {
	case "running", "paused":
		return "running"
	case "stopped":
		return "stopped"
	}
	return state
}

// self is the address the client used to reach this server, so the links in a VM object
// work from wherever the client runs, including when the server listens on 0.0.0.0.
func self(r *http.Request) (string, int) {
	host, portStr, err := net.SplitHostPort(r.Host)
	if err != nil {
		return r.Host, 80
	}
	port, _ := strconv.Atoi(portStr)
	if strings.Contains(host, ":") {
		host = "[" + host + "]" // IPv6, so clients can append ":PORT"
	}
	return host, port
}

func toVM(r *http.Request, d fleet.Desktop) vm {
	host, port := self(r)
	return vm{Name: d.Name, Status: cuaStatus(d.State), OS: "linux", OSType: "linux", Host: host,
		Endpoints: []endpoint{{Name: "api", Host: host, Port: port}}, AdfState: d.State}
}

// writeFleetErr maps fleet errors to the status codes Cua clients act on: 404 for a missing
// VM, 503 for no capacity (cua-sandbox retries it), 409 for a taken name.
func writeFleetErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, fleet.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, fleet.ErrInvalidName):
		status = http.StatusBadRequest
	case errors.Is(err, fleet.ErrExists):
		status = http.StatusConflict
	case errors.Is(err, fleet.ErrNoCapacity):
		status = http.StatusServiceUnavailable
	}
	writeDetail(w, status, err.Error())
}

func (s *Server) handleListVMs(w http.ResponseWriter, r *http.Request) {
	ds, err := s.fleet.List(r.Context())
	if err != nil {
		writeFleetErr(w, err)
		return
	}
	out := make([]vm, 0, len(ds))
	for _, d := range ds {
		out = append(out, toVM(r, d))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetVM(w http.ResponseWriter, r *http.Request) {
	d, err := s.target(r.Context(), r.PathValue("name"))
	if errors.Is(err, fleet.ErrInvalidName) {
		err = fmt.Errorf("%w: %v", fleet.ErrNotFound, err) // a name adf cannot have does not exist
	}
	if err != nil {
		writeFleetErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toVM(r, d))
}

// handleCreateVM creates a desktop. cua-sandbox sends {"os", "region", "configuration"} or
// explicit sizes; adf desktops all have one shape, so only os and an optional name matter.
func (s *Server) handleCreateVM(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		OS   string `json:"os"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
			writeDetail(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
			return
		}
	}
	if body.OS != "" && !strings.EqualFold(body.OS, "linux") {
		writeDetail(w, http.StatusBadRequest, fmt.Sprintf("adf desktops run Linux; os %q is not available", body.OS))
		return
	}
	start := time.Now()
	name, err := s.createDesktop(r, body.Name)
	s.record(name, "create_desktop", "", start, err == nil, errText(err))
	if err != nil {
		writeFleetErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toVM(r, fleet.Desktop{Name: name, State: "running"}))
}

func (s *Server) createDesktop(r *http.Request, name string) (string, error) {
	ctx, cancel := detached(r.Context())
	defer cancel()
	if err := s.ensureTemplate(ctx); err != nil {
		return name, err
	}
	if name != "" {
		_, err := s.fleet.CreateDesktop(ctx, name)
		return name, err
	}
	for range 3 { // a fresh random name is very unlikely to be taken, but retry if it is
		name = s.opts.NamePrefix + randomSuffix()
		_, err := s.fleet.CreateDesktop(ctx, name)
		if !errors.Is(err, fleet.ErrExists) {
			return name, err
		}
	}
	return name, fmt.Errorf("could not find a free desktop name starting with %q", s.opts.NamePrefix)
}

// randomSuffix returns six random lowercase letters and digits.
func randomSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b) // never fails on supported platforms
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

func (s *Server) handleDeleteVM(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	start := time.Now()
	err := s.fleet.Delete(r.Context(), name)
	s.record(name, "delete_desktop", "", start, err == nil, errText(err))
	if err != nil {
		writeFleetErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "status": "deleted"})
}

// handleVMAction starts, stops, suspends or restarts a desktop. Each finishes before the
// answer, so a client that polls afterwards sees the new state at once.
func (s *Server) handleVMAction(w http.ResponseWriter, r *http.Request) {
	ctx, name, action := r.Context(), r.PathValue("name"), r.PathValue("action")
	verbs := map[string]string{"start": "start_desktop", "run": "start_desktop", "stop": "stop_desktop",
		"suspend": "pause_desktop", "restart": "restart_desktop"}
	verb, ok := verbs[action]
	if !ok {
		writeDetail(w, http.StatusNotFound, fmt.Sprintf("unknown action %q (want start, run, stop, suspend or restart)", action))
		return
	}
	start := time.Now()
	d, err := s.target(ctx, name)
	ctx, cancel := detached(ctx)
	defer cancel()
	if err == nil {
		running := d.State == "running"
		switch action {
		case "start", "run":
			if !running {
				err = s.fleet.Start(ctx, name)
			}
		case "stop":
			if d.State != "stopped" {
				err = s.fleet.Stop(ctx, name)
			}
		case "suspend":
			if running {
				_, err = s.fleet.Sleep(ctx, name)
			}
		case "restart":
			if d.State != "stopped" {
				err = s.fleet.Stop(ctx, name)
			}
			if err == nil {
				err = s.fleet.Start(ctx, name)
			}
		}
	}
	s.record(name, verb, "", start, err == nil, errText(err))
	if err != nil {
		writeFleetErr(w, err)
		return
	}
	switch action {
	case "start", "run":
		w.WriteHeader(http.StatusNoContent)
	case "stop":
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "stopping"})
	case "suspend":
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "suspending"})
	case "restart":
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "restarting"})
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
