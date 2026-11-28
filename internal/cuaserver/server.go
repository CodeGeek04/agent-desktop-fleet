// Package cuaserver serves the Cua computer-server protocol (POST /cmd and friends) and the
// small /v1/vms platform API that Cua Cloud clients call, on top of adf desktops. Existing
// cua-computer and cua-sandbox code can then drive adf desktops after an environment change.
package cuaserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/activity"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
)

// Options configures the server.
type Options struct {
	Token      string            // API key every request but GET / and GET /status must carry
	Desktop    string            // desktop for requests that name none; empty means the only desktop
	NamePrefix string            // prefix for desktops that POST /v1/vms names itself (default "cua-")
	Version    string            // adf version, reported by the version command
	Activity   activity.Recorder // receives one event per command and lifecycle call; nil disables
	Log        io.Writer         // notes for the person running the server; nil for silence
}

// Server is the Cua-compatible HTTP API for a fleet.
type Server struct {
	fleet    *fleet.Manager
	opts     Options
	commands map[string]command

	mu      sync.Mutex
	ensured bool                   // template provisioned during this process's lifetime
	waking  map[string]*sync.Mutex // one lock per desktop, so concurrent commands start it once

	wsNoted atomic.Bool // the WebSocket note was already logged
}

// maxBody caps a request body. write_bytes sends up to 5 MB of file data as base64 in one call.
const maxBody = 64 << 20

// New returns a server for f.
func New(f *fleet.Manager, opts Options) *Server {
	if opts.NamePrefix == "" {
		opts.NamePrefix = "cua-"
	}
	return &Server{fleet: f, opts: opts, commands: commandTable(opts.Version), waking: map[string]*sync.Mutex{}}
}

// Handler serves the computer-server routes at the root and under /d/{name}/, and the
// platform API under /v1/vms.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleRoot)
	for _, prefix := range []string{"", "/d/{name}"} {
		mux.HandleFunc("GET "+prefix+"/status", s.handleStatus)
		mux.Handle("GET "+prefix+"/commands", s.auth(s.handleCommands))
		mux.Handle("POST "+prefix+"/cmd", s.auth(s.handleCmd))
		mux.Handle(prefix+"/ws", s.notSupported("the WebSocket API (/ws); use cua-computer 0.4 or later, which sends commands over REST"))
		mux.Handle(prefix+"/pty", s.notSupported("interactive terminals (/pty)"))
		mux.Handle(prefix+"/pty/", s.notSupported("interactive terminals (/pty)"))
		mux.Handle(prefix+"/playwright_exec", s.notSupported("/playwright_exec"))
	}
	mux.Handle("GET /v1/vms", s.auth(s.handleListVMs))
	mux.Handle("POST /v1/vms", s.auth(s.handleCreateVM))
	mux.Handle("GET /v1/vms/{name}", s.auth(s.handleGetVM))
	mux.Handle("DELETE /v1/vms/{name}", s.auth(s.handleDeleteVM))
	mux.Handle("POST /v1/vms/{name}/{action}", s.auth(s.handleVMAction))
	return mux
}

func (s *Server) logf(format string, args ...any) {
	if s.opts.Log != nil {
		fmt.Fprintf(s.opts.Log, format+"\n", args...)
	}
}

// writeDetail answers like FastAPI does for HTTP errors, which is what Cua clients expect.
func writeDetail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"detail": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// auth accepts the token as "Authorization: Bearer" (cua-sandbox and the platform API) or
// "X-API-Key" (cua-computer). Either header may carry something else, such as a proxy's
// bearer token, as long as the other one matches.
func (s *Server) auth(next http.HandlerFunc) http.Handler {
	want := []byte(s.opts.Token)
	match := func(got string) bool {
		return got != "" && subtle.ConstantTimeCompare([]byte(got), want) == 1
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, bearer, _ := strings.Cut(r.Header.Get("Authorization"), " ")
		if !strings.EqualFold(scheme, "Bearer") {
			bearer = ""
		}
		key := r.Header.Get("X-API-Key")
		if len(want) > 0 && (match(bearer) || match(key)) {
			next(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", "Bearer")
		msg := "Authentication failed"
		if bearer == "" && key == "" {
			msg = "API key required: send the adf cua-server token as X-API-Key or Authorization: Bearer (CUA_API_KEY)"
		}
		writeDetail(w, http.StatusUnauthorized, msg)
	})
}

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "server": "adf cua-server", "version": s.opts.Version})
}

// handleStatus is the readiness probe. It is open, like Cua's, and says nothing about desktops.
func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "os_type": "linux", "features": []string{}})
}

// notSupported answers routes of the computer-server API that adf does not serve yet.
func (s *Server) notSupported(what string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/ws") && s.wsNoted.CompareAndSwap(false, true) {
			s.logf("a client tried the WebSocket API (%s), which adf cua-server does not serve yet; "+
				"cua-computer 0.4 and later fall back to it only after a REST call fails", r.URL.Path)
		}
		writeDetail(w, http.StatusNotImplemented, "adf cua-server does not support "+what+" yet")
	})
}

// named returns the desktop a request names: the /d/{name} path, then the X-Container-Name
// header (cua-computer, cua-sandbox), then X-VM-Name (the TypeScript SDK), then --desktop.
// It returns "" when the request names none.
func (s *Server) named(r *http.Request) string {
	for _, v := range []string{r.PathValue("name"), r.Header.Get("X-Container-Name"), r.Header.Get("X-VM-Name"), s.opts.Desktop} {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// target finds the desktop a command runs on and its state. With no name it picks the only
// desktop there is, so a single-desktop setup needs no headers at all.
func (s *Server) target(ctx context.Context, name string) (fleet.Desktop, error) {
	if name != "" {
		if err := fleet.ValidateName(name); err != nil {
			return fleet.Desktop{}, err
		}
	}
	ds, err := s.fleet.List(ctx)
	if err != nil {
		return fleet.Desktop{}, err
	}
	if name == "" {
		switch len(ds) {
		case 0:
			return fleet.Desktop{}, fmt.Errorf("no desktops yet: create one with 'adf create NAME' or POST /v1/vms")
		case 1:
			return ds[0], nil
		}
		names := make([]string, len(ds))
		for i, d := range ds {
			names[i] = d.Name
		}
		return fleet.Desktop{}, fmt.Errorf("which desktop? set X-Container-Name (the Computer or Sandbox name) or use /d/NAME/cmd; desktops: %s",
			strings.Join(names, ", "))
	}
	for _, d := range ds {
		if d.Name == name {
			return d, nil
		}
	}
	return fleet.Desktop{}, fmt.Errorf("%w %q: create it with 'adf create %s' or POST /v1/vms", fleet.ErrNotFound, name, name)
}

// wake starts or resumes a desktop that is not running, as adf's own tools do on any action.
func (s *Server) wake(ctx context.Context, name string) error {
	s.mu.Lock()
	l, ok := s.waking[name]
	if !ok {
		l = &sync.Mutex{}
		s.waking[name] = l
	}
	s.mu.Unlock()
	l.Lock()
	defer l.Unlock()
	// Another command may have woken it while this one waited for the lock.
	d, err := s.target(ctx, name)
	if err != nil || d.State == "running" {
		return err
	}
	s.logf("desktop %q is %s; starting it", name, d.State)
	return s.fleet.Start(ctx, name)
}

// ensureTemplate provisions the template once per process; later calls are free.
func (s *Server) ensureTemplate(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ensured {
		return nil
	}
	if err := s.fleet.EnsureTemplate(ctx); err != nil {
		return err
	}
	s.ensured = true
	return nil
}

func (s *Server) record(desktop, action, detail string, start time.Time, ok bool, errMsg string) {
	if s.opts.Activity == nil || desktop == "" {
		return
	}
	s.opts.Activity.Record(activity.Event{Time: start, Desktop: desktop, Source: activity.SourceCua, Action: action,
		Detail: detail, OK: ok, Error: errMsg, DurationMS: time.Since(start).Milliseconds()})
}
