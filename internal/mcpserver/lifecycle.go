package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
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
	}, recorded(t, "create_desktop", nameOf, nil, t.createDesktop))

	mcp.AddTool(s, &mcp.Tool{
		Name: "pause_desktop",
		Description: "Pause a desktop to free its memory and CPU. It keeps everything, including open windows, " +
			"and the next action on it (or start_desktop) resumes it in a few seconds. Idle desktops are paused automatically.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(false), IdempotentHint: true, OpenWorldHint: ptr(false)},
	}, recorded(t, "pause_desktop", nameOf, nil, t.pauseDesktop))

	mcp.AddTool(s, &mcp.Tool{
		Name: "fleet_status",
		Description: "Show this computer's room for desktops: memory and CPU, desktops running, paused and stopped, " +
			"the limits, and how many more desktops fit now. Check it before creating several desktops.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: ptr(false)},
	}, t.fleetStatus)

	mcp.AddTool(s, &mcp.Tool{
		Name: "start_desktop",
		Description: "Resume a paused desktop or start a stopped one, and return a screenshot. Actions on a " +
			"paused or stopped desktop wake it automatically, so this is only needed to warm one up ahead of time.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(false), IdempotentHint: true, OpenWorldHint: ptr(false)},
	}, recorded(t, "start_desktop", nameOf, nil, t.startDesktop))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_desktop",
		Description: "Stop and permanently delete a desktop and everything on it.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(true), IdempotentHint: true, OpenWorldHint: ptr(false)},
	}, recorded(t, "delete_desktop", nameOf, nil, t.deleteDesktop))
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

func (t *tools) createDesktop(ctx context.Context, req *mcp.CallToolRequest, in nameIn) (*mcp.CallToolResult, any, error) {
	ctx, stop := withProgress(ctx, req)
	defer stop()
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

func (t *tools) pauseDesktop(ctx context.Context, _ *mcp.CallToolRequest, in nameIn) (*mcp.CallToolResult, any, error) {
	how, err := t.fleet.Sleep(ctx, in.Name)
	if err != nil {
		return nil, nil, err
	}
	msg := fmt.Sprintf("paused desktop %q", in.Name)
	if how == "stopped" {
		msg = fmt.Sprintf("stopped desktop %q instead of pausing it (%s): its files are kept but open windows were closed; the next action starts it again", in.Name, t.fleet.PauseUnsupported)
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: msg}}}, nil, nil
}

func (t *tools) fleetStatus(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, fleet.Capacity, error) {
	c, err := t.fleet.Capacity(ctx)
	return nil, c, err
}

func (t *tools) startDesktop(ctx context.Context, req *mcp.CallToolRequest, in nameIn) (*mcp.CallToolResult, any, error) {
	ctx, stop := withProgress(ctx, req)
	defer stop()
	start := time.Now()
	if err := t.fleet.Start(ctx, in.Name); err != nil {
		return nil, nil, err
	}
	note := fmt.Sprintf("started desktop %q in %s", in.Name, time.Since(start).Round(10*time.Millisecond))
	return t.withScreenshot(ctx, in.Name, nil, 0, note)
}
