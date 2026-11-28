package cuaserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/computer"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

// param describes one command argument for GET /commands, in the shape Cua's server uses.
type param struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Default  any    `json:"default"`
}

func req(name string) param          { return param{Name: name, Required: true} }
func opt(name string, def any) param { return param{Name: name, Default: def} }

// need is what a command needs before it runs.
type need int

const (
	needNothing need = iota // answers without looking at a desktop
	needDesktop             // a desktop that exists, running or not
	needAwake               // a running desktop: a paused or stopped one is started first
)

// desk is the desktop a command runs on.
type desk struct {
	name string
	c    *computer.Computer
	exec func(ctx context.Context, stdin io.Reader, argv ...string) ([]byte, error)
}

// result is a command's answer without "success". It may set "success": false itself to
// report a failure that still carries fields, as run_command does on a timeout.
type result map[string]any

type command struct {
	params []param
	need   need
	run    func(ctx context.Context, d desk, p params) (result, error)
	detail func(p params) string // a short description for the activity log; nil for none
}

// aliases are the other names Cua's server accepted for commands.
var aliases = map[string]string{
	"type": "type_text", "key": "press_key", "tap": "left_click", "click": "left_click",
	"shell": "run_command", "exec": "run_command", "read_file": "read_text", "write_file": "write_text",
	"ls": "list_dir", "mkdir": "create_dir", "rm": "delete_file", "rmdir": "delete_dir",
}

// notYet are Cua commands adf does not implement yet. They fail with a message that names
// the command instead of HTTP 400: on a 400, cua-computer spends several seconds retrying
// over the WebSocket API before it reports anything.
var notYet = map[string]bool{
	"copy_to_clipboard": true, "set_clipboard": true, "open": true, "launch": true, "set_wallpaper": true,
	"get_current_window_id": true, "get_application_windows": true, "get_window_name": true,
	"get_window_size": true, "get_window_position": true, "set_window_size": true,
	"set_window_position": true, "maximize_window": true, "minimize_window": true,
	"activate_window": true, "close_window": true, "get_active_window_bounds": true,
	"get_active_window_title": true, "diorama_cmd": true,
}

// commandTable returns every supported command. version is adf's version.
func commandTable(version string) map[string]command {
	t := map[string]command{
		"version": {need: needNothing, run: func(context.Context, desk, params) (result, error) {
			return result{"protocol": 1, "package": "adf-" + version}, nil
		}},
	}
	for name, c := range screenCommands() {
		t[name] = c
	}
	for name, c := range inputCommands() {
		t[name] = c
	}
	for name, c := range shellCommands() {
		t[name] = c
	}
	for name, c := range fileCommands() {
		t[name] = c
	}
	return t
}

// handleCommands lists the commands and aliases, as Cua's GET /commands does.
func (s *Server) handleCommands(w http.ResponseWriter, _ *http.Request) {
	byCommand := map[string][]string{}
	for alias, name := range aliases {
		byCommand[name] = append(byCommand[name], alias)
	}
	out := map[string]any{}
	for name, c := range s.commands {
		info := map[string]any{"params": append([]param{}, c.params...)}
		if a := byCommand[name]; len(a) > 0 {
			sort.Strings(a)
			info["aliases"] = a
		}
		out[name] = info
	}
	writeJSON(w, http.StatusOK, map[string]any{"commands": out, "aliases": aliases})
}

// handleCmd runs one command. Like Cua's server it answers text/plain with a single
// "data: <json>" frame, reports a failed action as HTTP 200 with "success": false, and
// answers HTTP 400 only for a malformed request or an unknown command.
func (s *Server) handleCmd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command string `json:"command"`
		Params  params `json:"params"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		writeDetail(w, http.StatusBadRequest, "Invalid JSON body: "+err.Error())
		return
	}
	name := strings.TrimSpace(body.Command)
	if name == "" {
		writeDetail(w, http.StatusBadRequest, "Command is required")
		return
	}
	if canonical, ok := aliases[name]; ok {
		name = canonical
	}
	cmd, known := s.commands[name]
	if !known && !notYet[name] {
		writeDetail(w, http.StatusBadRequest, s.unknownCommand(name))
		return
	}
	if body.Params == nil {
		body.Params = params{}
	}
	start := time.Now()
	desktop, res := s.named(r), map[string]any(nil)
	if known {
		desktop, res = s.execute(r.Context(), cmd, desktop, body.Params)
	} else {
		res = failure(fmt.Errorf("%s is not supported by adf cua-server yet", name))
	}
	detail := ""
	if known && cmd.detail != nil {
		detail = cmd.detail(body.Params)
	}
	errMsg, _ := res["error"].(string)
	s.record(desktop, name, detail, start, res["success"] == true, errMsg)
	writeFrame(w, res)
}

// execute finds the desktop, wakes it if the command needs it running, and runs the command.
// It returns the desktop's name, for the activity log, and the full answer.
func (s *Server) execute(ctx context.Context, cmd command, name string, p params) (string, map[string]any) {
	d := desk{name: name}
	if cmd.need != needNothing {
		t, err := s.target(ctx, name)
		if err != nil {
			return name, failure(err)
		}
		d.name = t.Name
		if cmd.need == needAwake && t.State != "running" {
			if err := s.wake(ctx, t.Name); err != nil {
				return d.name, failure(err)
			}
		}
		d.c = s.fleet.Computer(d.name)
		d.exec = func(ctx context.Context, stdin io.Reader, argv ...string) ([]byte, error) {
			return s.fleet.ExecStdin(ctx, d.name, stdin, argv...)
		}
	}
	res, err := cmd.run(ctx, d, p)
	if err != nil {
		if smolvm.IsNotFound(err) {
			err = fmt.Errorf("no desktop named %q: create it with 'adf create %s' or POST /v1/vms", d.name, d.name)
		}
		return d.name, failure(err)
	}
	out := map[string]any{"success": true}
	for k, v := range res {
		out[k] = v
	}
	return d.name, out
}

func failure(err error) map[string]any { return map[string]any{"success": false, "error": err.Error()} }

func writeFrame(w http.ResponseWriter, res map[string]any) {
	data, err := json.Marshal(res)
	if err != nil {
		data, _ = json.Marshal(failure(fmt.Errorf("encode result: %w", err)))
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "data: "+string(data)+"\n\n")
}

// unknownCommand words the error like Cua's server, with up to five suggestions.
func (s *Server) unknownCommand(name string) string {
	lower := strings.ToLower(name)
	prefix := lower
	if len(prefix) > 2 {
		prefix = prefix[:2]
	}
	var names []string
	for n := range s.commands {
		names = append(names, n)
	}
	sort.Strings(names)
	var similar []string
	for _, n := range names {
		if strings.HasPrefix(n, prefix) || strings.Contains(n, lower) {
			similar = append(similar, n)
		}
	}
	msg := "Unknown command: " + name
	if len(similar) > 5 {
		similar = similar[:5]
	}
	if len(similar) > 0 {
		msg += ". Did you mean: " + strings.Join(similar, ", ") + "?"
	}
	return msg
}

func point(x, y int) string { return fmt.Sprintf("(%d, %d)", x, y) }

// clip shortens s for the activity log.
func clip(s string) string {
	r := []rune(s)
	if len(r) > 80 {
		return string(r[:80]) + "..."
	}
	return s
}
