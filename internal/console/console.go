// Package console serves the web console: a live wall of desktops, a focused view with
// takeover, metrics and the activity feed.
package console

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/activity"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/computer"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/httpauth"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

//go:embed web
var webFS embed.FS

// Options configures the console.
type Options struct {
	Token        string            // required on every /api/ request
	ActivityPath string            // activity log to show; empty disables the feed
	Recorder     activity.Recorder // records console actions; nil disables recording
	FrameMaxAge  time.Duration     // reuse a captured frame for this long (default 100 ms)
	JPEGQuality  int               // default 60
	StatsEvery   time.Duration     // metrics refresh period (default 3 s)
}

// Server is the console backend.
type Server struct {
	fleet  *fleet.Manager
	opts   Options
	frames frameCache

	statsMu sync.Mutex
	stats   map[string]desktopStats
}

type desktopStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemUsedBytes  int64   `json:"mem_used_bytes"`
	MemTotalBytes int64   `json:"mem_total_bytes"`
}

type desktopJSON struct {
	Name  string        `json:"name"`
	State string        `json:"state"`
	Stats *desktopStats `json:"stats,omitempty"`
}

// New returns a console for f.
func New(f *fleet.Manager, opts Options) *Server {
	if opts.FrameMaxAge == 0 {
		opts.FrameMaxAge = 100 * time.Millisecond
	}
	if opts.JPEGQuality == 0 {
		opts.JPEGQuality = 60
	}
	if opts.StatsEvery == 0 {
		opts.StatsEvery = 3 * time.Second
	}
	return &Server{fleet: f, opts: opts, frames: frameCache{entries: map[string]*frameEntry{}}, stats: map[string]desktopStats{}}
}

// Handler serves the static page without a token and the API with one.
func (s *Server) Handler() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/status", s.handleStatus)
	api.HandleFunc("GET /api/desktops", s.handleList)
	api.HandleFunc("POST /api/desktops", s.handleCreate)
	api.HandleFunc("DELETE /api/desktops/{name}", s.handleDelete)
	api.HandleFunc("POST /api/desktops/{name}/start", s.handleStart)
	api.HandleFunc("POST /api/desktops/{name}/stop", s.handleStop)
	api.HandleFunc("POST /api/desktops/{name}/pause", s.handlePause)
	api.HandleFunc("GET /api/desktops/{name}/screen", s.handleScreen)
	api.HandleFunc("POST /api/desktops/{name}/input", s.handleInput)
	api.HandleFunc("GET /api/activity", s.handleActivity)

	web, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err) // the embedded directory always exists
	}
	root := http.NewServeMux()
	root.Handle("/api/", httpauth.RequireToken(s.opts.Token, api))
	root.Handle("/", http.FileServerFS(web))
	return root
}

// RunCollector refreshes desktop metrics until ctx is done.
func (s *Server) RunCollector(ctx context.Context) {
	ticker := time.NewTicker(s.opts.StatsEvery)
	defer ticker.Stop()
	for {
		s.refreshStats(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// refreshStats samples every running desktop in parallel (each sample takes about 0.5 s).
func (s *Server) refreshStats(ctx context.Context) {
	ds, err := s.fleet.List(ctx)
	if err != nil {
		return
	}
	fresh := map[string]desktopStats{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, d := range ds {
		if d.State != "running" {
			continue
		}
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			st, err := s.fleet.Stats(ctx, name)
			if err != nil {
				return
			}
			ds := desktopStats{CPUPercent: st.CPUPercent, MemUsedBytes: st.Memory.UsedBytes, MemTotalBytes: st.Memory.TotalBytes}
			// smolvm counts ballooned memory as used; the guest's own figures leave it out.
			if mem, err := s.fleet.Memory(ctx, name); err == nil {
				ds.MemUsedBytes, ds.MemTotalBytes = mem.UsedBytes, mem.TotalBytes
			}
			mu.Lock()
			fresh[name] = ds
			mu.Unlock()
		}(d.Name)
	}
	wg.Wait()
	s.statsMu.Lock()
	s.stats = fresh
	s.statsMu.Unlock()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeMessage(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeErr maps errors to status codes the page can act on.
func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, computer.ErrInvalid), errors.Is(err, fleet.ErrInvalidName):
		status = http.StatusBadRequest
	case errors.Is(err, fleet.ErrNotFound), smolvm.IsNotFound(err):
		status = http.StatusNotFound
	case errors.Is(err, fleet.ErrExists):
		status = http.StatusConflict
	}
	writeMessage(w, status, err.Error())
}

func (s *Server) record(desktop, action, detail string, start time.Time, err error) {
	if s.opts.Recorder == nil {
		return
	}
	e := activity.Event{Time: start, Desktop: desktop, Source: activity.SourceConsole, Action: action, Detail: detail,
		OK: err == nil, DurationMS: time.Since(start).Milliseconds()}
	if err != nil {
		e.Error = err.Error()
	}
	s.opts.Recorder.Record(e)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	state, err := s.fleet.TemplateState(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	capacity, err := s.fleet.Capacity(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"template": state, "capacity": capacity})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	ds, err := s.fleet.List(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	out := []desktopJSON{}
	s.statsMu.Lock()
	for _, d := range ds {
		j := desktopJSON{Name: d.Name, State: d.State}
		if st, ok := s.stats[d.Name]; ok && d.State == "running" {
			j.Stats = &st
		}
		out = append(out, j)
	}
	s.statsMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"desktops": out})
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
		writeMessage(w, http.StatusBadRequest, `send JSON like {"name": "agent-1"}`)
		return
	}
	if err := fleet.ValidateName(body.Name); err != nil {
		writeErr(w, err)
		return
	}
	start := time.Now()
	err := s.fleet.EnsureTemplate(r.Context())
	if err == nil {
		_, err = s.fleet.CreateDesktop(r.Context(), body.Name)
	}
	s.record(body.Name, "create_desktop", "", start, err)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": body.Name})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	start := time.Now()
	err := s.fleet.Delete(r.Context(), name)
	s.frames.forget(name)
	s.record(name, "delete_desktop", "", start, err)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleScreen(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := fleet.ValidateName(name); err != nil {
		writeErr(w, err)
		return
	}
	c := s.fleet.Computer(name)
	frame, err := s.frames.get(r.Context(), name, s.opts.FrameMaxAge, func(ctx context.Context) ([]byte, error) {
		return c.ScreenshotJPEG(ctx, s.opts.JPEGQuality)
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(frame)
}

func (s *Server) handleInput(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := fleet.ValidateName(name); err != nil {
		writeErr(w, err)
		return
	}
	var in inputReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeMessage(w, http.StatusBadRequest, "send one JSON input event")
		return
	}
	start := time.Now()
	detail, err := applyInput(r.Context(), s.fleet.Computer(name), in)
	s.record(name, in.Kind, detail, start, err)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	if s.opts.ActivityPath == "" {
		writeJSON(w, http.StatusOK, map[string]any{"events": []activity.Event{}, "cursor": 0})
		return
	}
	var (
		events []activity.Event
		cursor int64
		err    error
	)
	if c := r.URL.Query().Get("cursor"); c != "" {
		n, perr := strconv.ParseInt(c, 10, 64)
		if perr != nil || n < 0 {
			writeMessage(w, http.StatusBadRequest, "cursor must be a non-negative integer")
			return
		}
		events, cursor, err = activity.ReadFrom(s.opts.ActivityPath, n)
	} else {
		events, cursor, err = activity.Recent(s.opts.ActivityPath, 200)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	if events == nil {
		events = []activity.Event{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "cursor": cursor})
}

// frameCache keeps the latest frame per desktop so several viewers share one capture.
type frameCache struct {
	mu      sync.Mutex
	entries map[string]*frameEntry
}

type frameEntry struct {
	mu   sync.Mutex // held during a capture, so concurrent requests wait for it
	data []byte
	at   time.Time
}

func (c *frameCache) get(ctx context.Context, name string, maxAge time.Duration, capture func(context.Context) ([]byte, error)) ([]byte, error) {
	c.mu.Lock()
	e := c.entries[name]
	if e == nil {
		e = &frameEntry{}
		c.entries[name] = e
	}
	c.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.data != nil && time.Since(e.at) < maxAge {
		return e.data, nil
	}
	data, err := capture(ctx)
	if err != nil {
		return nil, err
	}
	e.data, e.at = data, time.Now()
	return data, nil
}

func (c *frameCache) forget(name string) {
	c.mu.Lock()
	delete(c.entries, name)
	c.mu.Unlock()
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	start := time.Now()
	err := s.fleet.Start(r.Context(), name)
	s.record(name, "start_desktop", "", start, err)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	start := time.Now()
	err := s.fleet.Stop(r.Context(), name)
	s.frames.forget(name)
	s.record(name, "stop_desktop", "", start, err)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	start := time.Now()
	err := s.fleet.Pause(r.Context(), name)
	s.frames.forget(name)
	s.record(name, "pause_desktop", "", start, err)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
