package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet/fleettest"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

var testPNG = append([]byte("\x89PNG\r\n\x1a\n"), []byte("fake image")...)

// newTestRuntime returns a fake runtime with a warm, provisioned template whose
// screenshots return testPNG.
func newTestRuntime() *fleettest.Fake {
	rt := fleettest.New()
	rt.Machines[fleet.TemplateName] = &smolvm.Machine{Name: fleet.TemplateName, State: "running", Branchable: true,
		Labels: map[string]string{fleet.LabelRole: fleet.RoleTemplate}}
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		switch {
		case argv[0] == "cat":
			return []byte(image.Version), nil
		case len(argv) > 2 && argv[2] == "sh":
			return testPNG, nil
		}
		return nil, nil
	}
	return rt
}

// connect starts the MCP server on rt and returns a connected client session.
func connect(t *testing.T, rt *fleettest.Fake) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	m := fleet.NewManager(rt, nil)
	m.Settle = 0
	server := New(m, Options{Version: "test"})
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: protocol error: %v", name, err)
	}
	return res
}

func text(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func images(res *mcp.CallToolResult) [][]byte {
	var out [][]byte
	for _, c := range res.Content {
		if ic, ok := c.(*mcp.ImageContent); ok {
			out = append(out, ic.Data)
		}
	}
	return out
}

func TestListDesktops(t *testing.T) {
	rt := newTestRuntime()
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{fleet.LabelRole: fleet.RoleDesktop}}
	res := call(t, connect(t, rt), "list_desktops", map[string]any{})
	if res.IsError {
		t.Fatal(text(res))
	}
	var out struct {
		Desktops []struct{ Name, State string } `json:"desktops"`
	}
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil {
		t.Fatalf("result is not JSON: %v: %s", err, text(res))
	}
	if len(out.Desktops) != 1 || out.Desktops[0].Name != "a" || out.Desktops[0].State != "running" {
		t.Fatalf("desktops = %+v", out.Desktops)
	}
}

func TestListDesktopsEmptyIsArray(t *testing.T) {
	res := call(t, connect(t, newTestRuntime()), "list_desktops", map[string]any{})
	if res.IsError || !strings.Contains(text(res), `"desktops":[]`) {
		t.Fatalf("result = %s", text(res))
	}
}

func TestCreateDesktopReturnsScreenshot(t *testing.T) {
	rt := newTestRuntime()
	res := call(t, connect(t, rt), "create_desktop", map[string]any{"name": "a"})
	if res.IsError {
		t.Fatal(text(res))
	}
	if !strings.Contains(text(res), `created desktop "a"`) {
		t.Fatalf("text = %q", text(res))
	}
	if imgs := images(res); len(imgs) != 1 || !bytes.Equal(imgs[0], testPNG) {
		t.Fatalf("want one screenshot, got %d images", len(imgs))
	}
	if _, ok := rt.Machines["adf-a"]; !ok {
		t.Fatal("desktop machine was not created")
	}
}

func TestCreateDesktopBadNameIsToolError(t *testing.T) {
	res := call(t, connect(t, newTestRuntime()), "create_desktop", map[string]any{"name": "Bad Name"})
	if !res.IsError {
		t.Fatal("expected a tool error")
	}
}

func TestDeleteDesktop(t *testing.T) {
	rt := newTestRuntime()
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{fleet.LabelRole: fleet.RoleDesktop}}
	res := call(t, connect(t, rt), "delete_desktop", map[string]any{"name": "a"})
	if res.IsError {
		t.Fatal(text(res))
	}
	if _, ok := rt.Machines["adf-a"]; ok {
		t.Fatal("desktop still exists")
	}
}

func TestToolsAreListedWithDescriptions(t *testing.T) {
	cs := connect(t, newTestRuntime())
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		if tool.Description == "" {
			t.Errorf("tool %s has no description", tool.Name)
		}
		got[tool.Name] = true
	}
	for _, name := range wantTools {
		if !got[name] {
			t.Errorf("tool %s is missing", name)
		}
	}
}

// wantTools grows as tools are added in later tasks.
var wantTools = []string{"list_desktops", "create_desktop", "delete_desktop"}
