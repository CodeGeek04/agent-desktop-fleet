package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"
)

// maxOutput caps stdout and stderr returned to the agent, to protect its context window.
const maxOutput = 64 * 1024

type commandIn struct {
	Desktop string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	Command string `json:"command" jsonschema:"shell command, run with sh -c as root inside the desktop"`
}

type commandOut struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

func truncate(b []byte) string {
	if len(b) <= maxOutput {
		return string(b)
	}
	return string(b[:maxOutput]) + fmt.Sprintf("\n[truncated %d bytes]", len(b)-maxOutput)
}

func (t *tools) addCommand(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "run_command",
		Description: "Run a shell command inside the desktop (as root, with sh -c) and return its exit code, " +
			"stdout and stderr. Use it to install software, inspect files or prepare the desktop; " +
			"the command runs without a display, so use the action tools to drive windows.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(true), OpenWorldHint: ptr(true)},
	}, t.runCommand)
}

func (t *tools) runCommand(ctx context.Context, _ *mcp.CallToolRequest, in commandIn) (*mcp.CallToolResult, commandOut, error) {
	if err := fleet.ValidateName(in.Desktop); err != nil {
		return nil, commandOut{}, err
	}
	out, err := t.fleet.Exec(ctx, in.Desktop, "sh", "-c", in.Command)
	var exitErr *runner.ExitError
	switch {
	case err == nil:
		return nil, commandOut{ExitCode: 0, Stdout: truncate(out), Stderr: ""}, nil
	case errors.As(err, &exitErr):
		return nil, commandOut{ExitCode: exitErr.Code, Stdout: truncate(out), Stderr: truncate(exitErr.Stderr)}, nil
	default:
		return nil, commandOut{}, explain(in.Desktop, err)
	}
}
