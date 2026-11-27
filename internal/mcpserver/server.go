// Package mcpserver exposes the desktop fleet as MCP tools.
package mcpserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/activity"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/computer"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

// Options configures the server.
type Options struct {
	Version  string            // reported to clients
	Settle   time.Duration     // wait between an action and its follow-up screenshot
	Activity activity.Recorder // receives one event per tool call that touches a desktop; nil disables
}

// DefaultSettle gives pages a moment to react before the follow-up screenshot.
const DefaultSettle = 500 * time.Millisecond

type tools struct {
	fleet *fleet.Manager
	opts  Options

	mu      sync.Mutex
	ensured bool // template provisioned during this process's lifetime
}

// New returns an MCP server whose tools manage and drive desktops through f.
func New(f *fleet.Manager, opts Options) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "adf",
		Title:   "Agent desktop fleet",
		Version: opts.Version,
	}, &mcp.ServerOptions{
		Instructions: fmt.Sprintf("Sandboxed Linux desktops (%dx%d, Firefox) for computer use. "+
			"Call create_desktop first, then use the action tools with the same desktop name. "+
			"Coordinates are pixels from the top-left corner. Delete desktops you no longer need.",
			image.Width, image.Height),
	})
	t := &tools{fleet: f, opts: opts}
	t.addLifecycle(server)
	t.addActions(server)
	t.addCommand(server)
	return server
}

func ptr[T any](v T) *T { return &v }

// ensureTemplate provisions the template once per process; later calls are free.
func (t *tools) ensureTemplate(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ensured {
		return nil
	}
	if err := t.fleet.EnsureTemplate(ctx); err != nil {
		return err
	}
	t.ensured = true
	return nil
}

// computer returns the actions for a validated desktop name.
func (t *tools) computer(name string) (*computer.Computer, error) {
	if err := fleet.ValidateName(name); err != nil {
		return nil, err
	}
	return t.fleet.Computer(name), nil
}

// explain turns runtime errors into messages an agent can act on.
func explain(name string, err error) error {
	if smolvm.IsNotFound(err) {
		return fmt.Errorf("no running desktop named %q; call list_desktops or create_desktop", name)
	}
	return err
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// withScreenshot returns note plus a screenshot of the desktop, unless want is false.
func (t *tools) withScreenshot(ctx context.Context, desktop string, want *bool, settle time.Duration, note string) (*mcp.CallToolResult, any, error) {
	res := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: note}}}
	if want != nil && !*want {
		return res, nil, nil
	}
	if err := sleepCtx(ctx, settle); err != nil {
		return nil, nil, err
	}
	c, err := t.computer(desktop)
	if err != nil {
		return nil, nil, err
	}
	png, err := c.Screenshot(ctx)
	if err != nil {
		return nil, nil, explain(desktop, err)
	}
	res.Content = append(res.Content, &mcp.ImageContent{Data: png, MIMEType: "image/png"})
	return res, nil, nil
}

// heartbeat is how often a long tool call reports progress when nothing new happened.
var heartbeat = 10 * time.Second

// withProgress forwards fleet steps to the client as progress notifications, plus a
// heartbeat, so clients that reset their timeout on progress keep waiting through the
// first template build. It does nothing unless the client asked for progress.
func withProgress(ctx context.Context, req *mcp.CallToolRequest) (context.Context, func()) {
	if req == nil || req.Session == nil || req.Params == nil || req.Params.GetProgressToken() == nil {
		return ctx, func() {}
	}
	token := req.Params.GetProgressToken()
	var mu sync.Mutex
	count, last := 0.0, "working"
	started := time.Now()
	send := func(msg string) {
		mu.Lock()
		count++
		if msg != "" {
			last = msg
		} else {
			msg = fmt.Sprintf("%s (%s)", last, time.Since(started).Round(time.Second))
		}
		p := count
		mu.Unlock()
		_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{ProgressToken: token, Message: msg, Progress: p})
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				send("")
			}
		}
	}()
	return fleet.WithProgress(ctx, send), func() { close(done) }
}
