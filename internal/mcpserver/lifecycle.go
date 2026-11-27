package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type desktopInfo struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type listOut struct {
	Desktops []desktopInfo `json:"desktops"`
}

type nameIn struct {
	Name string `json:"name" jsonschema:"desktop name: 1 to 40 lowercase letters, digits or dashes"`
}

func (t *tools) addLifecycle(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_desktops",
		Description: "List all desktops with their state (running or stopped).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: ptr(false)},
	}, t.listDesktops)

	mcp.AddTool(s, &mcp.Tool{
		Name: "create_desktop",
		Description: "Create a new running Linux desktop with Firefox open, and return a screenshot of it. " +
			"Each desktop is an isolated microVM. The very first call on a machine builds the desktop " +
			"template, which takes one to two minutes; later calls take about a second.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(false), OpenWorldHint: ptr(false)},
	}, t.createDesktop)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_desktop",
		Description: "Stop and permanently delete a desktop and everything on it.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(true), IdempotentHint: true, OpenWorldHint: ptr(false)},
	}, t.deleteDesktop)
}

func (t *tools) listDesktops(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listOut, error) {
	ds, err := t.fleet.List(ctx)
	if err != nil {
		return nil, listOut{}, err
	}
	out := listOut{Desktops: []desktopInfo{}}
	for _, d := range ds {
		out.Desktops = append(out.Desktops, desktopInfo{Name: d.Name, State: d.State})
	}
	return nil, out, nil
}

func (t *tools) createDesktop(ctx context.Context, _ *mcp.CallToolRequest, in nameIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if err := t.ensureTemplate(ctx); err != nil {
		return nil, nil, err
	}
	if _, err := t.fleet.CreateDesktop(ctx, in.Name); err != nil {
		return nil, nil, err
	}
	note := fmt.Sprintf("created desktop %q in %s", in.Name, time.Since(start).Round(10*time.Millisecond))
	return t.withScreenshot(ctx, in.Name, nil, 0, note)
}

func (t *tools) deleteDesktop(ctx context.Context, _ *mcp.CallToolRequest, in nameIn) (*mcp.CallToolResult, any, error) {
	if err := t.fleet.Delete(ctx, in.Name); err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("deleted desktop %q", in.Name)}}}, nil, nil
}
