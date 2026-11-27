//go:build integration

package integration

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPOverStdio builds adf, runs `adf mcp` as a child process and drives a desktop through it.
func TestMCPOverStdio(t *testing.T) {
	smolvmBin(t) // skips when smolvm is missing
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	bin := filepath.Join(t.TempDir(), "adf")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/adf")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "mcp")
	cmd.Stderr = os.Stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "it", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	callTool := func(name string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.IsError {
			var msg []string
			for _, c := range res.Content {
				if tc, ok := c.(*mcp.TextContent); ok {
					msg = append(msg, tc.Text)
				}
			}
			t.Fatalf("%s: tool error: %s", name, strings.Join(msg, " "))
		}
		return res
	}
	firstImage := func(res *mcp.CallToolResult) []byte {
		for _, c := range res.Content {
			if ic, ok := c.(*mcp.ImageContent); ok {
				return ic.Data
			}
		}
		t.Fatal("no image in result")
		return nil
	}

	_, _ = cs.CallTool(ctx, &mcp.CallToolParams{Name: "delete_desktop", Arguments: map[string]any{"name": "mcp-it"}})
	start := time.Now()
	created := callTool("create_desktop", map[string]any{"name": "mcp-it"})
	t.Logf("create_desktop took %s", time.Since(start))
	t.Cleanup(func() {
		_, _ = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "delete_desktop", Arguments: map[string]any{"name": "mcp-it"}})
	})
	before := firstImage(created)

	callTool("press_key", map[string]any{"desktop": "mcp-it", "keys": "ctrl+l", "screenshot": false})
	callTool("type_text", map[string]any{"desktop": "mcp-it", "text": "about:robots", "screenshot": false})
	after := firstImage(callTool("press_key", map[string]any{"desktop": "mcp-it", "keys": "Return"}))
	if !bytes.HasPrefix(after, []byte("\x89PNG")) || bytes.Equal(before, after) {
		t.Fatal("screen did not change after navigating")
	}
	if err := os.WriteFile(filepath.Join(os.TempDir(), "adf-mcp-it.png"), after, 0o644); err != nil {
		t.Fatal(err)
	}

	cmdRes := callTool("run_command", map[string]any{"desktop": "mcp-it", "command": "echo hello-from-mcp"})
	var text string
	for _, c := range cmdRes.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "hello-from-mcp") || !strings.Contains(text, `"exit_code":0`) {
		t.Fatalf("run_command result: %s", text)
	}

	listed := callTool("list_desktops", map[string]any{})
	if tc, ok := listed.Content[0].(*mcp.TextContent); !ok || !strings.Contains(tc.Text, `"mcp-it"`) {
		t.Fatalf("list_desktops did not include mcp-it: %+v", listed.Content)
	}
}
