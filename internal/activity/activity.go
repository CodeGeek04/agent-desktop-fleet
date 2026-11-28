// Package activity records what agents and people do to desktops. Every adf process
// appends JSON lines to one shared file, and the console reads it to show a live feed.
package activity

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Sources of events.
const (
	SourceAgent   = "agent"   // an MCP tool call
	SourceConsole = "console" // a person using the web console
	SourceCua     = "cua"     // a Cua SDK client through adf cua-server
)

// Event is one action on one desktop.
type Event struct {
	Time       time.Time `json:"time"`
	Desktop    string    `json:"desktop"`
	Source     string    `json:"source"`
	Action     string    `json:"action"`
	Detail     string    `json:"detail,omitempty"`
	OK         bool      `json:"ok"`
	Error      string    `json:"error,omitempty"`
	DurationMS int64     `json:"duration_ms"`
}

// Recorder receives events. Recording must never make an action fail.
type Recorder interface {
	Record(Event)
}

// DefaultMaxSize is the size at which Log moves the file to Path+".1" and starts a new one.
const DefaultMaxSize = 8 << 20

// maxRead caps how much of the file a single read looks at.
const maxRead = 4 << 20

// DefaultPath returns the shared log location: ~/Library/Application Support/adf/activity.jsonl
// on macOS and ~/.config/adf/activity.jsonl on Linux.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "adf", "activity.jsonl"), nil
}

// Log appends events to a JSON-lines file. It is safe for concurrent use, and separate
// processes may append to the same file.
type Log struct {
	Path    string
	MaxSize int64 // 0 means DefaultMaxSize

	mu sync.Mutex
}

// Record implements Recorder. Errors are dropped on purpose.
func (l *Log) Record(e Event) { _ = l.Append(e) }

// Append writes one event as a line. The file is private to the user because it can
// contain commands that agents ran.
func (l *Log) Append(e Event) error {
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o700); err != nil {
		return err
	}
	max := l.MaxSize
	if max <= 0 {
		max = DefaultMaxSize
	}
	if fi, err := os.Stat(l.Path); err == nil && fi.Size() >= max {
		_ = os.Rename(l.Path, l.Path+".1")
	}
	f, err := os.OpenFile(l.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(line, '\n'))
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

// ReadFrom returns the complete events after byte offset cursor and the cursor for the next
// call. A trailing partial line is left for later. If the file is now shorter than cursor
// (it was rotated), reading restarts at the beginning.
func ReadFrom(path string, cursor int64) ([]Event, int64, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, cursor, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, cursor, err
	}
	if fi.Size() < cursor {
		cursor = 0
	}
	if _, err := f.Seek(cursor, io.SeekStart); err != nil {
		return nil, cursor, err
	}
	data, err := io.ReadAll(io.LimitReader(f, maxRead))
	if err != nil {
		return nil, cursor, err
	}
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		return nil, cursor, nil
	}
	return parse(data[:end]), cursor + int64(end) + 1, nil
}

// Recent returns up to n of the newest events and the cursor just after them.
func Recent(path string, n int) ([]Event, int64, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	start := fi.Size() - maxRead
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, 0, err
	}
	data, err := io.ReadAll(io.LimitReader(f, maxRead))
	if err != nil {
		return nil, 0, err
	}
	end := bytes.LastIndexByte(data, '\n')
	if end < 0 {
		return nil, start, nil
	}
	body := data[:end]
	if start > 0 {
		// The first line may have been cut in half; skip it.
		if i := bytes.IndexByte(body, '\n'); i >= 0 {
			body = body[i+1:]
		} else {
			body = nil
		}
	}
	events := parse(body)
	if len(events) > n {
		events = events[len(events)-n:]
	}
	return events, start + int64(end) + 1, nil
}

func parse(data []byte) []Event {
	var out []Event
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var e Event
		if json.Unmarshal(line, &e) == nil {
			out = append(out, e)
		}
	}
	return out
}

// LastActive returns the time of the newest event for each desktop in the recent part of the log.
func LastActive(path string) (map[string]time.Time, error) {
	events, _, err := Recent(path, int(^uint(0)>>1))
	last := map[string]time.Time{}
	for _, e := range events {
		if e.Time.After(last[e.Desktop]) {
			last[e.Desktop] = e.Time
		}
	}
	return last, err
}
