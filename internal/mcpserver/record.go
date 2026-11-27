package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/activity"
)

// recorded wraps a tool handler so each call is written to the activity log.
func recorded[In, Out any](t *tools, action string, desktop func(In) string, detail func(In) string, h mcp.ToolHandlerFor[In, Out]) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		start := time.Now()
		res, out, err := h(ctx, req, in)
		if t.opts.Activity != nil {
			e := activity.Event{Time: start, Desktop: desktop(in), Source: activity.SourceAgent, Action: action,
				OK: err == nil, DurationMS: time.Since(start).Milliseconds()}
			if detail != nil {
				e.Detail = detail(in)
			}
			if err != nil {
				e.Error = err.Error()
			}
			t.opts.Activity.Record(e)
		}
		return res, out, err
	}
}

func point(x, y int) string { return fmt.Sprintf("(%d, %d)", x, y) }

func nameOf(in nameIn) string { return in.Name }

func screenshotDetail(in screenshotIn) string {
	if in.WaitSeconds > 0 {
		return fmt.Sprintf("after %gs", in.WaitSeconds)
	}
	return ""
}

func clickDetail(in clickIn) string {
	d := point(in.X, in.Y)
	if in.Button != "" && in.Button != "left" {
		d += " " + in.Button
	}
	if in.Double {
		d += " double"
	}
	return d
}

func dragDetail(in dragIn) string { return point(in.FromX, in.FromY) + " to " + point(in.ToX, in.ToY) }

func scrollDetail(in scrollIn) string {
	dir, amount := scrollDefaults(in)
	return fmt.Sprintf("%s %d at %s", dir, amount, point(in.X, in.Y))
}

// typeDetail records only the length: typed text can be a password.
func typeDetail(in typeIn) string { return fmt.Sprintf("%d characters", len([]rune(in.Text))) }

func commandDetail(in commandIn) string {
	r := []rune(in.Command)
	if len(r) > 80 {
		return string(r[:80]) + "..."
	}
	return in.Command
}
