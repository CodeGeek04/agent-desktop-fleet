// Package mcpserver exposes the desktop fleet as MCP tools.
package mcpserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/computer"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

// Options configures the server.
type Options struct {
	Version string        // reported to clients
	Settle  time.Duration // wait between an action and its follow-up screenshot
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
