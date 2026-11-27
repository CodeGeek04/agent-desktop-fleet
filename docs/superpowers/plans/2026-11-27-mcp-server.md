# MCP Server Implementation Plan (plan 2 of 4)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `adf mcp` serves MCP tools over stdio (for Claude Code, Cursor and other local clients) and over authenticated streamable HTTP, so any agent can create, see, drive and remove sandboxed desktops.

**Architecture:** A new `internal/mcpserver` package builds an `*mcp.Server` (official Go SDK v1.8.0) on top of `fleet.Manager`. Each tool is a typed handler; actions return a follow-up screenshot by default so the agent sees the result without an extra call. `cmd/adf/mcp.go` runs it over `mcp.StdioTransport` or `mcp.NewStreamableHTTPHandler` (stateless) behind a bearer-token check. The template is created lazily on the first `create_desktop`. Template operations in `fleet.Manager` are serialized because MCP clients call tools concurrently. The desktop image moves to version 2 with Firefox default preferences that remove first-run, onboarding and privacy-notice tabs.

**Tech Stack:** Go 1.25, github.com/modelcontextprotocol/go-sdk v1.8.0, cobra, smolvm 1.37.7.

**Facts this plan relies on (verified 2026-11-27):**
- go-sdk v1.8.0: `mcp.AddTool(server, &mcp.Tool{...}, func(ctx, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error))` infers the input schema from `In`; fields without `omitempty` are required; `jsonschema:"text"` becomes the description; `*bool` becomes `["null","boolean"]`. A handler error or a missing required argument comes back as a result with `isError: true`, not a protocol error. With a typed `Out` and no `Content`, the SDK fills `Content` with the JSON of `Out`.
- `mcp.ImageContent{Data: rawBytes, MIMEType: "image/png"}` takes raw bytes; the SDK base64-encodes on the wire and the Go client decodes back to raw bytes.
- `mcp.NewInMemoryTransports()` connects a server (`server.Connect(ctx, t, nil)`) and a client (`client.Connect(ctx, t, nil)`) in tests. `mcp.CommandTransport{Command: exec.Command(...)}` runs a stdio server as a child process.
- `mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server, &mcp.StreamableHTTPOptions{Stateless: true})` serves HTTP without session IDs, matching the 2026-07-28 spec direction.
- Firefox 153.8.0esr in Debian trixie keeps profiles in `/root/.config/mozilla`. A file in `/usr/lib/firefox-esr/defaults/pref/` with the prefs in Task 3 gives a clean blank window on a fresh profile; the policies file alone did not suppress the onboarding and privacy-notice tabs. The browser process name is `firefox-esr`; `pkill -f firefox-esr` must not be used from a shell whose own command line contains that word.

---

## File structure

```
internal/fleet/fleettest/fake.go       in-memory Runtime for tests (moved out of fleet_test.go)
internal/fleet/fleet.go                + mutex around template operations, + browser reset in provisioning
internal/fleet/fleet_test.go           rewritten on fleettest.Fake, + concurrency test
internal/image/desktop-setup.sh        + Firefox default prefs file
internal/image/desktop-start.sh        pgrep -x firefox-esr
internal/image/image.go                Version "2", + ResetBrowserScript
internal/image/image_test.go           + checks for prefs and process matching
internal/mcpserver/server.go           New(), shared helpers
internal/mcpserver/lifecycle.go        list_desktops, create_desktop, delete_desktop
internal/mcpserver/actions.go          screenshot, click, move_mouse, drag, scroll, type_text, press_key, cursor_position
internal/mcpserver/command.go          run_command
internal/mcpserver/server_test.go      tests over in-memory transports
cmd/adf/mcp.go                         `adf mcp` (stdio, --http, --token) and bearer-token middleware
cmd/adf/mcp_test.go                    middleware tests
tests/integration/mcp_test.go          stdio end-to-end against real smolvm
README.md                              "Use with Claude Code and Cursor" section
```

---

### Task 1: Extract a shared fake runtime

**Files:**
- Create: `internal/fleet/fleettest/fake.go`
- Modify: `internal/fleet/fleet_test.go` (replace the in-file fake)

- [ ] **Step 1: Write `internal/fleet/fleettest/fake.go`**

```go
// Package fleettest provides an in-memory runtime for testing code built on the fleet package.
package fleettest

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

// Fake is a concurrency-safe in-memory fleet.Runtime that records a readable call log.
type Fake struct {
	mu       sync.Mutex
	Machines map[string]*smolvm.Machine
	calls    []string
	// ExecFunc decides each Exec result. Nil means success with empty output.
	// It is called without the lock held, so it must be safe for concurrent use.
	ExecFunc func(name, stdin string, argv []string) ([]byte, error)
	// StartDelay makes Start take time, like a real boot, so tests can expose races.
	StartDelay time.Duration
}

// New returns an empty Fake.
func New() *Fake { return &Fake{Machines: map[string]*smolvm.Machine{}} }

// Calls returns a copy of the call log.
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *Fake) record(s string) { f.calls = append(f.calls, s) }

func labelString(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, ",")
}

// List implements fleet.Runtime.
func (f *Fake) List(context.Context) ([]smolvm.Machine, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]smolvm.Machine, 0, len(f.Machines))
	for _, m := range f.Machines {
		out = append(out, *m)
	}
	return out, nil
}

// Create implements fleet.Runtime.
func (f *Fake) Create(_ context.Context, o smolvm.CreateOpts) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("create " + o.Name)
	f.Machines[o.Name] = &smolvm.Machine{Name: o.Name, State: "stopped", Labels: o.Labels}
	return nil
}

// Start implements fleet.Runtime.
func (f *Fake) Start(_ context.Context, name string, branchable bool) error {
	f.mu.Lock()
	f.record(fmt.Sprintf("start %s branchable=%v", name, branchable))
	delay := f.StartDelay
	f.mu.Unlock()
	time.Sleep(delay)
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.Machines[name]
	if !ok {
		return fmt.Errorf("vm not found: %s", name)
	}
	m.State, m.Branchable = "running", branchable
	return nil
}

// Branch implements fleet.Runtime.
func (f *Fake) Branch(_ context.Context, o smolvm.BranchOpts) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(fmt.Sprintf("branch %s->%s %s", o.From, o.Name, labelString(o.Labels)))
	if _, ok := f.Machines[o.Name]; ok {
		return fmt.Errorf("machine %s already exists", o.Name)
	}
	f.Machines[o.Name] = &smolvm.Machine{Name: o.Name, State: "running", Labels: o.Labels, ParentMachine: o.From}
	return nil
}

// Stop implements fleet.Runtime.
func (f *Fake) Stop(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("stop " + name)
	m, ok := f.Machines[name]
	if !ok {
		return fmt.Errorf("vm not found: %s", name)
	}
	m.State = "stopped"
	return nil
}

// Delete implements fleet.Runtime.
func (f *Fake) Delete(_ context.Context, name string, cascade bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(fmt.Sprintf("delete %s cascade=%v", name, cascade))
	delete(f.Machines, name)
	return nil
}

// Exec implements fleet.Runtime.
func (f *Fake) Exec(_ context.Context, name string, stdin io.Reader, argv ...string) ([]byte, error) {
	in := ""
	if stdin != nil {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return nil, err
		}
		in = string(b)
	}
	f.mu.Lock()
	f.record("exec " + name + " " + strings.Join(argv, " "))
	fn := f.ExecFunc
	f.mu.Unlock()
	if fn != nil {
		return fn(name, in, argv)
	}
	return nil, nil
}
```

- [ ] **Step 2: Replace `internal/fleet/fleet_test.go` with a version built on `fleettest.Fake`**

```go
package fleet

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet/fleettest"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

var ctx = context.Background()

func testManager(rt *fleettest.Fake) *Manager {
	m := NewManager(rt, nil)
	m.Poll, m.Settle, m.ReadyTimeout = time.Millisecond, 0, time.Second
	return m
}

func warmTemplate(rt *fleettest.Fake) {
	rt.Machines[TemplateName] = &smolvm.Machine{Name: TemplateName, State: "running", Branchable: true,
		Labels: map[string]string{LabelRole: RoleTemplate}}
}

func TestEnsureTemplateCreatesAndProvisions(t *testing.T) {
	rt := fleettest.New()
	var gotSetup, gotStart string
	rt.ExecFunc = func(name, stdin string, argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		switch {
		case argv[0] == "cat":
			return nil, errors.New("no such file")
		case strings.Contains(joined, "adf-setup.sh") && stdin != "":
			gotSetup = stdin
		case strings.Contains(joined, "desktop-start") && stdin != "":
			gotStart = stdin
		}
		return nil, nil
	}
	if err := testManager(rt).EnsureTemplate(ctx); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"create adf-template",
		"start adf-template branchable=true",
		"exec adf-template cat /etc/adf-image-version",
		"exec adf-template sh -c cat > /tmp/adf-setup.sh",
		"exec adf-template sh /tmp/adf-setup.sh",
		"exec adf-template sh -c cat > /usr/local/bin/desktop-start && chmod 755 /usr/local/bin/desktop-start",
		"exec adf-template sh -c echo " + image.Version + " > /etc/adf-image-version",
	}
	if got := rt.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if gotSetup != image.SetupScript || gotStart != image.StartScript {
		t.Fatal("setup or start script was not sent as stdin")
	}
	if rt.Machines[TemplateName].Labels[LabelRole] != RoleTemplate {
		t.Fatal("template is missing its role label")
	}
}

func TestEnsureTemplateSkipsProvisionedTemplate(t *testing.T) {
	rt := fleettest.New()
	warmTemplate(rt)
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) { return []byte(image.Version + "\n"), nil }
	if err := testManager(rt).EnsureTemplate(ctx); err != nil {
		t.Fatal(err)
	}
	want := []string{"exec adf-template cat /etc/adf-image-version"}
	if got := rt.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestEnsureTemplateRestartsNonBranchableTemplate(t *testing.T) {
	rt := fleettest.New()
	rt.Machines[TemplateName] = &smolvm.Machine{Name: TemplateName, State: "running", Branchable: false}
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) { return []byte(image.Version), nil }
	if err := testManager(rt).EnsureTemplate(ctx); err != nil {
		t.Fatal(err)
	}
	if got := rt.Calls(); got[0] != "stop adf-template" || got[1] != "start adf-template branchable=true" {
		t.Fatalf("calls = %q", got)
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"a", "agent-7", "x1", strings.Repeat("a", 40)} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("%q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-a", "a-", "A", "a_b", "a.b", "template", strings.Repeat("a", 41), "a b"} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestWarmTemplateMissingTellsUserToInit(t *testing.T) {
	err := testManager(fleettest.New()).WarmTemplate(ctx)
	if err == nil || !strings.Contains(err.Error(), "adf init") {
		t.Fatalf("err = %v, want a hint to run adf init", err)
	}
}

func TestWarmTemplateStartsDesktopAndWaits(t *testing.T) {
	rt := fleettest.New()
	rt.Machines[TemplateName] = &smolvm.Machine{Name: TemplateName, State: "stopped"}
	searches := 0
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		if strings.Contains(strings.Join(argv, " "), "xdotool search") {
			searches++
			if searches < 3 {
				return nil, errors.New("no window yet")
			}
		}
		return nil, nil
	}
	if err := testManager(rt).WarmTemplate(ctx); err != nil {
		t.Fatal(err)
	}
	got := rt.Calls()
	if got[0] != "start adf-template branchable=true" || got[1] != "exec adf-template desktop-start" {
		t.Fatalf("calls = %q", got)
	}
	if searches != 3 {
		t.Fatalf("searched %d times, want 3", searches)
	}
}

func TestWarmTemplateTimesOut(t *testing.T) {
	rt := fleettest.New()
	warmTemplate(rt)
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		if argv[0] == "env" {
			return nil, errors.New("no window")
		}
		return nil, nil
	}
	m := testManager(rt)
	m.ReadyTimeout = 20 * time.Millisecond
	if err := m.WarmTemplate(ctx); err == nil {
		t.Fatal("expected a timeout error")
	}
}

func TestCreateDesktopBranchesFromTemplate(t *testing.T) {
	rt := fleettest.New()
	warmTemplate(rt)
	d, err := testManager(rt).CreateDesktop(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if d != (Desktop{Name: "a", Machine: "adf-a", State: "running"}) {
		t.Fatalf("desktop = %+v", d)
	}
	got := rt.Calls()
	if last := got[len(got)-1]; last != "branch adf-template->adf-a adf.role=desktop" {
		t.Fatalf("last call = %q", last)
	}
}

func TestCreateDesktopRejectsBadAndDuplicateNames(t *testing.T) {
	rt := fleettest.New()
	warmTemplate(rt)
	m := testManager(rt)
	if _, err := m.CreateDesktop(ctx, "Bad_Name"); err == nil {
		t.Fatal("expected invalid name error")
	}
	if _, err := m.CreateDesktop(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.CreateDesktop(ctx, "a"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want already exists", err)
	}
}

func TestListReturnsOnlyDesktopsSorted(t *testing.T) {
	rt := fleettest.New()
	warmTemplate(rt)
	rt.Machines["adf-b"] = &smolvm.Machine{Name: "adf-b", State: "stopped", Labels: map[string]string{LabelRole: RoleDesktop}}
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{LabelRole: RoleDesktop}}
	rt.Machines["other"] = &smolvm.Machine{Name: "other", State: "running", Labels: map[string]string{LabelRole: RoleDesktop}}
	ds, err := testManager(rt).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []Desktop{{"a", "adf-a", "running"}, {"b", "adf-b", "stopped"}}
	if !reflect.DeepEqual(ds, want) {
		t.Fatalf("got %+v, want %+v", ds, want)
	}
}

func TestDeleteStopsRunningDesktopFirst(t *testing.T) {
	rt := fleettest.New()
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{LabelRole: RoleDesktop}}
	if err := testManager(rt).Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	want := []string{"stop adf-a", "delete adf-a cascade=false"}
	if got := rt.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestDeleteUnknownDesktop(t *testing.T) {
	if err := testManager(fleettest.New()).Delete(ctx, "nope"); err == nil {
		t.Fatal("expected not found error")
	}
}

func TestComputerRunsInsideTheDesktop(t *testing.T) {
	rt := fleettest.New()
	if err := testManager(rt).Computer("a").Key(ctx, "Return"); err != nil {
		t.Fatal(err)
	}
	if got := rt.Calls(); got[0] != "exec adf-a env DISPLAY=:1 xdotool key -- Return" {
		t.Fatalf("call = %q", got[0])
	}
}
```

- [ ] **Step 3: Run the tests (pure refactor, everything must still pass)**

Run: `go test ./internal/fleet/...`
Expected: `ok` for `internal/fleet`; `internal/fleet/fleettest` has no test files.

- [ ] **Step 4: Commit**

```bash
git add internal/fleet
git commit -m "Move the fleet test runtime into a reusable fleettest package"
```

---

### Task 2: Serialize template operations

MCP clients run tool calls concurrently. Two `create_desktop` calls on a stopped template must not both start it.

**Files:**
- Modify: `internal/fleet/fleet.go`
- Test: `internal/fleet/fleet_test.go` (append)

- [ ] **Step 1: Write the failing test (append to `fleet_test.go`)**

Add `"fmt"` and `"sync"` to the test file's imports.

```go
func TestConcurrentCreatesStartTemplateOnce(t *testing.T) {
	rt := fleettest.New()
	rt.Machines[TemplateName] = &smolvm.Machine{Name: TemplateName, State: "stopped",
		Labels: map[string]string{LabelRole: RoleTemplate}}
	rt.StartDelay = 20 * time.Millisecond
	m := testManager(rt)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := m.CreateDesktop(ctx, fmt.Sprintf("d%d", i)); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	starts := 0
	for _, c := range rt.Calls() {
		if strings.HasPrefix(c, "start adf-template") {
			starts++
		}
	}
	if starts != 1 {
		t.Fatalf("template started %d times, want 1", starts)
	}
}
```

- [ ] **Step 2: Run it with the race detector to see it fail**

Run: `go test -race -count=5 ./internal/fleet/ -run TestConcurrentCreatesStartTemplateOnce`
Expected: FAIL in at least one run (`template started N times, want 1`) or a race report.

- [ ] **Step 3: Add the mutex in `internal/fleet/fleet.go`**

Add `"sync"` to the import block. Add a field to `Manager`, after `Settle`:

```go
	mu sync.Mutex // serializes template operations; tool calls may arrive concurrently
```

At the top of `EnsureTemplate`, before `t, err := m.find(ctx, TemplateName)`, add:

```go
	m.mu.Lock()
	defer m.mu.Unlock()
```

At the top of `WarmTemplate`, before `t, err := m.find(ctx, TemplateName)`, add the same two lines.

- [ ] **Step 4: Run the fleet tests with the race detector**

Run: `go test -race -count=5 ./internal/fleet/`
Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/fleet
git commit -m "Serialize template operations for concurrent callers"
```

---

### Task 3: Desktop image version 2 (clean Firefox start)

**Files:**
- Modify: `internal/image/desktop-setup.sh`, `internal/image/desktop-start.sh`, `internal/image/image.go`, `internal/image/image_test.go`
- Modify: `internal/fleet/fleet.go` (provisioning steps), `internal/fleet/fleet_test.go` (expected calls)

- [ ] **Step 1: Write the failing tests**

Append to `internal/image/image_test.go`:

```go
func TestFirefoxStartsClean(t *testing.T) {
	for _, pref := range []string{
		`pref("browser.aboutwelcome.enabled", false);`,
		`pref("datareporting.policy.dataSubmissionPolicyBypassNotification", true);`,
		`pref("browser.sessionstore.resume_from_crash", false);`,
		`pref("browser.newtabpage.enabled", false);`,
	} {
		if !strings.Contains(SetupScript, pref) {
			t.Errorf("setup script is missing %s", pref)
		}
	}
	if !strings.Contains(SetupScript, "/usr/lib/firefox-esr/defaults/pref/adf.js") {
		t.Error("prefs must go to Firefox's defaults/pref directory")
	}
	if strings.Contains(StartScript, "pgrep -f") {
		t.Error("start script must match processes by name (pgrep -x), not by command line")
	}
	if !strings.Contains(ResetBrowserScript, "/root/.config/mozilla") {
		t.Error("browser reset must remove the XDG profile directory")
	}
	if strings.Contains(ResetBrowserScript, "pkill -f") {
		t.Error("browser reset must not use pkill -f, which can match its own shell")
	}
}
```

In `internal/fleet/fleet_test.go`, in `TestEnsureTemplateCreatesAndProvisions`, insert this line into `want` directly before the line that writes the image version:

```go
		"exec adf-template sh -c " + image.ResetBrowserScript,
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/image/ ./internal/fleet/`
Expected: FAIL to compile (`undefined: ResetBrowserScript`).

- [ ] **Step 3: Replace the Firefox section of `internal/image/desktop-setup.sh`**

Replace everything from the line `# Firefox: no first-run pages, ...` down to and including the closing `EOF` of the policies heredoc with:

```sh
# Firefox policies: no telemetry, studies or default-browser checks.
mkdir -p /etc/firefox-esr/policies
cat > /etc/firefox-esr/policies/policies.json <<'EOF'
{
  "policies": {
    "DisableTelemetry": true,
    "DisableFirefoxStudies": true,
    "DontCheckDefaultBrowser": true
  }
}
EOF

# Firefox default prefs: a blank window with no first-run, onboarding, privacy-notice
# or session-restore tabs. Policies alone do not suppress these on Firefox 153 ESR.
cat > /usr/lib/firefox-esr/defaults/pref/adf.js <<'EOF'
pref("browser.aboutwelcome.enabled", false);
pref("trailhead.firstrun.didSeeAboutWelcome", true);
pref("browser.startup.homepage_override.mstone", "ignore");
pref("startup.homepage_welcome_url", "");
pref("startup.homepage_welcome_url.additional", "");
pref("startup.homepage_override_url", "");
pref("datareporting.policy.firstRunURL", "");
pref("datareporting.policy.dataSubmissionPolicyBypassNotification", true);
pref("toolkit.telemetry.reportingpolicy.firstRun", false);
pref("browser.shell.checkDefaultBrowser", false);
pref("browser.sessionstore.resume_from_crash", false);
pref("browser.startup.page", 0);
pref("browser.newtabpage.enabled", false);
pref("browser.toolbars.bookmarks.visibility", "never");
EOF
```

- [ ] **Step 4: Fix process matching in `internal/image/desktop-start.sh`**

Replace:

```sh
pgrep -f firefox-esr >/dev/null || nohup firefox-esr --new-instance >/tmp/firefox.log 2>&1 &
```

with:

```sh
pgrep -x firefox-esr >/dev/null || nohup firefox-esr --new-instance >/tmp/firefox.log 2>&1 &
```

- [ ] **Step 5: Bump the version and add the reset script in `internal/image/image.go`**

Change `const Version = "1"` to `const Version = "2"`, then append:

```go
// ResetBrowserScript stops Firefox and deletes its profile so the next desktop-start
// opens a fresh profile with the current default prefs. Used when a template is upgraded.
const ResetBrowserScript = `pkill -x firefox-esr
i=0
while pgrep -x firefox-esr >/dev/null && [ "$i" -lt 20 ]; do i=$((i + 1)); sleep 0.25; done
rm -rf /root/.mozilla /root/.config/mozilla /root/.cache/mozilla
true`
```

- [ ] **Step 6: Add the reset step to provisioning in `internal/fleet/fleet.go`**

In `EnsureTemplate`, in the `steps` slice, insert this element directly before the step that writes `/etc/adf-image-version`:

```go
		{"", []string{"sh", "-c", image.ResetBrowserScript}},
```

- [ ] **Step 7: Run the tests**

Run: `sh -n internal/image/desktop-setup.sh && sh -n internal/image/desktop-start.sh && go test ./internal/...`
Expected: no shell syntax errors; all packages `ok`.

- [ ] **Step 8: Commit**

```bash
git add internal/image internal/fleet
git commit -m "Image v2: clean Firefox start, match processes by name, reset browser on upgrade"
```

---

### Task 4: MCP server with lifecycle tools

**Files:**
- Create: `internal/mcpserver/server.go`, `internal/mcpserver/lifecycle.go`
- Test: `internal/mcpserver/server_test.go`

- [ ] **Step 1: Add the SDK**

```bash
go get github.com/modelcontextprotocol/go-sdk@v1.8.0
```

- [ ] **Step 2: Write the failing tests in `internal/mcpserver/server_test.go`**

```go
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
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/mcpserver/`
Expected: FAIL to compile (`undefined: New`, `undefined: Options`).

- [ ] **Step 4: Write `internal/mcpserver/server.go`**

```go
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
```

- [ ] **Step 5: Write `internal/mcpserver/lifecycle.go`**

```go
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
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go mod tidy && go test ./internal/mcpserver/`
Expected: `ok`

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/mcpserver
git commit -m "Add MCP server with list, create and delete desktop tools"
```

---

### Task 5: Screenshot and action tools

**Files:**
- Create: `internal/mcpserver/actions.go`
- Modify: `internal/mcpserver/server.go` (register), `internal/mcpserver/server_test.go` (append, extend `wantTools`)

- [ ] **Step 1: Write the failing tests**

In `server_test.go`, replace the `wantTools` declaration with:

```go
var wantTools = []string{"list_desktops", "create_desktop", "delete_desktop", "screenshot", "click",
	"move_mouse", "drag", "scroll", "type_text", "press_key", "cursor_position"}
```

Append:

```go
func desktopRuntime() *fleettest.Fake {
	rt := newTestRuntime()
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{fleet.LabelRole: fleet.RoleDesktop}}
	return rt
}

func TestActionsRunXdotoolAndReturnScreenshots(t *testing.T) {
	cases := []struct {
		tool string
		args map[string]any
		want string // expected exec call
	}{
		{"click", map[string]any{"desktop": "a", "x": 10, "y": 20}, "exec adf-a env DISPLAY=:1 xdotool mousemove 10 20 click 1"},
		{"click", map[string]any{"desktop": "a", "x": 10, "y": 20, "button": "right", "double": true},
			"exec adf-a env DISPLAY=:1 xdotool mousemove 10 20 click --repeat 2 --delay 80 3"},
		{"move_mouse", map[string]any{"desktop": "a", "x": 5, "y": 6}, "exec adf-a env DISPLAY=:1 xdotool mousemove 5 6"},
		{"drag", map[string]any{"desktop": "a", "from_x": 1, "from_y": 2, "to_x": 30, "to_y": 40},
			"exec adf-a env DISPLAY=:1 xdotool mousemove 1 2 mousedown 1 mousemove 30 40 mouseup 1"},
		{"scroll", map[string]any{"desktop": "a", "x": 100, "y": 200}, "exec adf-a env DISPLAY=:1 xdotool mousemove 100 200 click --repeat 3 5"},
		{"scroll", map[string]any{"desktop": "a", "x": 100, "y": 200, "direction": "up", "amount": 2},
			"exec adf-a env DISPLAY=:1 xdotool mousemove 100 200 click --repeat 2 4"},
		{"type_text", map[string]any{"desktop": "a", "text": "hello"}, "exec adf-a env DISPLAY=:1 xdotool type --delay 12 -- hello"},
		{"press_key", map[string]any{"desktop": "a", "keys": "ctrl+l"}, "exec adf-a env DISPLAY=:1 xdotool key -- ctrl+l"},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			rt := desktopRuntime()
			res := call(t, connect(t, rt), tc.tool, tc.args)
			if res.IsError {
				t.Fatal(text(res))
			}
			calls := rt.Calls()
			if calls[0] != tc.want {
				t.Fatalf("first call = %q\nwant         %q", calls[0], tc.want)
			}
			if imgs := images(res); len(imgs) != 1 {
				t.Fatalf("want a follow-up screenshot, got %d images", len(imgs))
			}
		})
	}
}

func TestActionWithoutScreenshot(t *testing.T) {
	rt := desktopRuntime()
	res := call(t, connect(t, rt), "click", map[string]any{"desktop": "a", "x": 1, "y": 1, "screenshot": false})
	if res.IsError || len(images(res)) != 0 || len(rt.Calls()) != 1 {
		t.Fatalf("isError=%v images=%d calls=%q", res.IsError, len(images(res)), rt.Calls())
	}
}

func TestScreenshotTool(t *testing.T) {
	res := call(t, connect(t, desktopRuntime()), "screenshot", map[string]any{"desktop": "a"})
	if res.IsError {
		t.Fatal(text(res))
	}
	if imgs := images(res); len(imgs) != 1 || !bytes.Equal(imgs[0], testPNG) {
		t.Fatal("expected the screenshot bytes")
	}
}

func TestScreenshotWaitOutOfRange(t *testing.T) {
	res := call(t, connect(t, desktopRuntime()), "screenshot", map[string]any{"desktop": "a", "wait_seconds": 31})
	if !res.IsError {
		t.Fatal("expected a tool error for wait_seconds above 30")
	}
}

func TestOutOfScreenClickIsToolErrorWithoutExec(t *testing.T) {
	rt := desktopRuntime()
	res := call(t, connect(t, rt), "click", map[string]any{"desktop": "a", "x": 5000, "y": 1})
	if !res.IsError || len(rt.Calls()) != 0 {
		t.Fatalf("isError=%v calls=%q", res.IsError, rt.Calls())
	}
}

func TestCursorPosition(t *testing.T) {
	rt := desktopRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		return []byte("X=7\nY=9\nSCREEN=0\nWINDOW=1\n"), nil
	}
	res := call(t, connect(t, rt), "cursor_position", map[string]any{"desktop": "a"})
	if res.IsError || !strings.Contains(text(res), `"x":7`) || !strings.Contains(text(res), `"y":9`) {
		t.Fatalf("result = %s", text(res))
	}
}

func TestUnknownDesktopExplainsWhatToDo(t *testing.T) {
	rt := newTestRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		return nil, &smolvm.Error{Code: "NOT_FOUND", Message: "vm not found: adf-zzz"}
	}
	res := call(t, connect(t, rt), "screenshot", map[string]any{"desktop": "zzz"})
	if !res.IsError || !strings.Contains(text(res), "create_desktop") {
		t.Fatalf("result = %s", text(res))
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/mcpserver/`
Expected: FAIL (`tool screenshot is missing` and similar).

- [ ] **Step 3: Write `internal/mcpserver/actions.go`**

```go
package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/computer"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
)

// maxWaitSeconds caps the screenshot tool's wait_seconds.
const maxWaitSeconds = 30

type screenshotIn struct {
	Desktop     string  `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	WaitSeconds float64 `json:"wait_seconds,omitempty" jsonschema:"seconds to wait before capturing, 0 to 30, for pages that are still loading"`
}

type clickIn struct {
	Desktop    string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	X          int    `json:"x" jsonschema:"pixels from the left edge"`
	Y          int    `json:"y" jsonschema:"pixels from the top edge"`
	Button     string `json:"button,omitempty" jsonschema:"left (default), middle or right"`
	Double     bool   `json:"double,omitempty" jsonschema:"double-click instead of a single click"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

type pointIn struct {
	Desktop    string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	X          int    `json:"x" jsonschema:"pixels from the left edge"`
	Y          int    `json:"y" jsonschema:"pixels from the top edge"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

type dragIn struct {
	Desktop    string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	FromX      int    `json:"from_x" jsonschema:"start, pixels from the left edge"`
	FromY      int    `json:"from_y" jsonschema:"start, pixels from the top edge"`
	ToX        int    `json:"to_x" jsonschema:"end, pixels from the left edge"`
	ToY        int    `json:"to_y" jsonschema:"end, pixels from the top edge"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

type scrollIn struct {
	Desktop    string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	X          int    `json:"x" jsonschema:"pixels from the left edge; the pointer moves here before scrolling"`
	Y          int    `json:"y" jsonschema:"pixels from the top edge"`
	Direction  string `json:"direction,omitempty" jsonschema:"up, down (default), left or right"`
	Amount     int    `json:"amount,omitempty" jsonschema:"wheel clicks, 1 to 50 (default 3)"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

type typeIn struct {
	Desktop    string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	Text       string `json:"text" jsonschema:"text to type into the focused window"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

type keyIn struct {
	Desktop    string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
	Keys       string `json:"keys" jsonschema:"xdotool key names or chords, such as Return, ctrl+l, ctrl+shift+t, Page_Down; separate several with spaces"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

type desktopIn struct {
	Desktop string `json:"desktop" jsonschema:"name of the desktop, as given to create_desktop"`
}

type cursorOut struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func (t *tools) addActions(s *mcp.Server) {
	screen := fmt.Sprintf("The screen is %dx%d pixels; (0, 0) is the top-left corner. ", image.Width, image.Height)
	act := &mcp.ToolAnnotations{DestructiveHint: ptr(false), OpenWorldHint: ptr(true)}

	mcp.AddTool(s, &mcp.Tool{Name: "screenshot", Description: "Capture the desktop screen as a PNG. " + screen,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: ptr(false)}}, t.screenshot)
	mcp.AddTool(s, &mcp.Tool{Name: "click", Description: "Move the pointer to a point and click. " + screen +
		"Returns a screenshot afterwards unless screenshot is false.", Annotations: act}, t.click)
	mcp.AddTool(s, &mcp.Tool{Name: "move_mouse", Description: "Move the pointer without clicking, for hover menus. " + screen,
		Annotations: act}, t.moveMouse)
	mcp.AddTool(s, &mcp.Tool{Name: "drag", Description: "Press the left button at one point, move to another and release. " + screen,
		Annotations: act}, t.drag)
	mcp.AddTool(s, &mcp.Tool{Name: "scroll", Description: "Scroll with the mouse wheel at a point. " + screen,
		Annotations: act}, t.scroll)
	mcp.AddTool(s, &mcp.Tool{Name: "type_text", Description: "Type text into the focused window, as keyboard input. " +
		"Click a field first to focus it. Use press_key for Enter, Tab and shortcuts.", Annotations: act}, t.typeText)
	mcp.AddTool(s, &mcp.Tool{Name: "press_key", Description: "Press a key or shortcut, such as Return, Tab, Escape, ctrl+l " +
		"(focus the address bar), ctrl+t (new tab) or ctrl+w (close tab).", Annotations: act}, t.pressKey)
	mcp.AddTool(s, &mcp.Tool{Name: "cursor_position", Description: "Return the pointer position in pixels.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: ptr(false)}}, t.cursorPosition)
}

// act validates the desktop, runs do, and returns a note plus an optional screenshot.
func (t *tools) act(ctx context.Context, desktop string, shot *bool, note string, do func(c *computer.Computer) error) (*mcp.CallToolResult, any, error) {
	c, err := t.computer(desktop)
	if err != nil {
		return nil, nil, err
	}
	if err := do(c); err != nil {
		return nil, nil, explain(desktop, err)
	}
	return t.withScreenshot(ctx, desktop, shot, t.opts.Settle, note)
}

func (t *tools) screenshot(ctx context.Context, _ *mcp.CallToolRequest, in screenshotIn) (*mcp.CallToolResult, any, error) {
	if in.WaitSeconds < 0 || in.WaitSeconds > maxWaitSeconds {
		return nil, nil, fmt.Errorf("wait_seconds must be between 0 and %d", maxWaitSeconds)
	}
	wait := time.Duration(in.WaitSeconds * float64(time.Second))
	note := fmt.Sprintf("screenshot of %q (%dx%d)", in.Desktop, image.Width, image.Height)
	return t.withScreenshot(ctx, in.Desktop, nil, wait, note)
}

func (t *tools) click(ctx context.Context, _ *mcp.CallToolRequest, in clickIn) (*mcp.CallToolResult, any, error) {
	b := computer.Button(in.Button)
	return t.act(ctx, in.Desktop, in.Screenshot, fmt.Sprintf("clicked at (%d, %d)", in.X, in.Y), func(c *computer.Computer) error {
		if in.Double {
			return c.DoubleClick(ctx, in.X, in.Y, b)
		}
		return c.Click(ctx, in.X, in.Y, b)
	})
}

func (t *tools) moveMouse(ctx context.Context, _ *mcp.CallToolRequest, in pointIn) (*mcp.CallToolResult, any, error) {
	return t.act(ctx, in.Desktop, in.Screenshot, fmt.Sprintf("moved pointer to (%d, %d)", in.X, in.Y), func(c *computer.Computer) error {
		return c.Move(ctx, in.X, in.Y)
	})
}

func (t *tools) drag(ctx context.Context, _ *mcp.CallToolRequest, in dragIn) (*mcp.CallToolResult, any, error) {
	note := fmt.Sprintf("dragged from (%d, %d) to (%d, %d)", in.FromX, in.FromY, in.ToX, in.ToY)
	return t.act(ctx, in.Desktop, in.Screenshot, note, func(c *computer.Computer) error {
		return c.Drag(ctx, in.FromX, in.FromY, in.ToX, in.ToY)
	})
}

func (t *tools) scroll(ctx context.Context, _ *mcp.CallToolRequest, in scrollIn) (*mcp.CallToolResult, any, error) {
	dir := computer.Direction(in.Direction)
	if dir == "" {
		dir = computer.ScrollDown
	}
	amount := in.Amount
	if amount == 0 {
		amount = 3
	}
	note := fmt.Sprintf("scrolled %s %d at (%d, %d)", dir, amount, in.X, in.Y)
	return t.act(ctx, in.Desktop, in.Screenshot, note, func(c *computer.Computer) error {
		return c.Scroll(ctx, in.X, in.Y, dir, amount)
	})
}

func (t *tools) typeText(ctx context.Context, _ *mcp.CallToolRequest, in typeIn) (*mcp.CallToolResult, any, error) {
	return t.act(ctx, in.Desktop, in.Screenshot, fmt.Sprintf("typed %d characters", len([]rune(in.Text))), func(c *computer.Computer) error {
		return c.Type(ctx, in.Text)
	})
}

func (t *tools) pressKey(ctx context.Context, _ *mcp.CallToolRequest, in keyIn) (*mcp.CallToolResult, any, error) {
	return t.act(ctx, in.Desktop, in.Screenshot, "pressed "+in.Keys, func(c *computer.Computer) error {
		return c.Key(ctx, in.Keys)
	})
}

func (t *tools) cursorPosition(ctx context.Context, _ *mcp.CallToolRequest, in desktopIn) (*mcp.CallToolResult, cursorOut, error) {
	c, err := t.computer(in.Desktop)
	if err != nil {
		return nil, cursorOut{}, err
	}
	x, y, err := c.CursorPosition(ctx)
	if err != nil {
		return nil, cursorOut{}, explain(in.Desktop, err)
	}
	return nil, cursorOut{X: x, Y: y}, nil
}
```

- [ ] **Step 4: Register the action tools in `internal/mcpserver/server.go`**

In `New`, after `t.addLifecycle(server)`, add:

```go
	t.addActions(server)
```

- [ ] **Step 5: Run the tests**

Run: `go vet ./internal/mcpserver/ && go test ./internal/mcpserver/`
Expected: vet clean, `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/mcpserver
git commit -m "Add MCP screenshot and computer action tools"
```

---

### Task 6: run_command tool

**Files:**
- Create: `internal/mcpserver/command.go`
- Modify: `internal/mcpserver/server.go` (register), `internal/mcpserver/server_test.go` (append, extend `wantTools`)

- [ ] **Step 1: Write the failing tests**

In `server_test.go`, add `"run_command"` to the end of `wantTools`, add `"errors"` and `"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"` to the imports, and append:

```go
func TestRunCommandSuccess(t *testing.T) {
	rt := desktopRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) { return []byte("hi\n"), nil }
	res := call(t, connect(t, rt), "run_command", map[string]any{"desktop": "a", "command": "echo hi"})
	if res.IsError {
		t.Fatal(text(res))
	}
	if got := rt.Calls()[0]; got != "exec adf-a sh -c echo hi" {
		t.Fatalf("call = %q", got)
	}
	var out struct {
		ExitCode int    `json:"exit_code"`
		Stdout   string `json:"stdout"`
	}
	if err := json.Unmarshal([]byte(text(res)), &out); err != nil || out.ExitCode != 0 || out.Stdout != "hi\n" {
		t.Fatalf("out=%+v err=%v text=%s", out, err, text(res))
	}
}

func TestRunCommandNonZeroExitIsAResultNotAnError(t *testing.T) {
	rt := desktopRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		return []byte("partial"), &runner.ExitError{Name: "smolvm", Code: 3, Stderr: []byte("bad thing\n")}
	}
	res := call(t, connect(t, rt), "run_command", map[string]any{"desktop": "a", "command": "false"})
	if res.IsError {
		t.Fatalf("non-zero exit must not be a tool error: %s", text(res))
	}
	for _, want := range []string{`"exit_code":3`, `"stdout":"partial"`, `"stderr":"bad thing\n"`} {
		if !strings.Contains(text(res), want) {
			t.Errorf("result %s is missing %s", text(res), want)
		}
	}
}

func TestRunCommandTruncatesLongOutput(t *testing.T) {
	rt := desktopRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) { return bytes.Repeat([]byte("x"), maxOutput+100), nil }
	res := call(t, connect(t, rt), "run_command", map[string]any{"desktop": "a", "command": "yes"})
	if res.IsError || !strings.Contains(text(res), "[truncated") {
		t.Fatalf("expected truncation marker, got %d bytes", len(text(res)))
	}
}

func TestRunCommandOtherErrorsAreToolErrors(t *testing.T) {
	rt := desktopRuntime()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) { return nil, errors.New("smolvm crashed") }
	res := call(t, connect(t, rt), "run_command", map[string]any{"desktop": "a", "command": "true"})
	if !res.IsError {
		t.Fatal("expected a tool error")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/mcpserver/`
Expected: FAIL to compile (`undefined: maxOutput`).

- [ ] **Step 3: Write `internal/mcpserver/command.go`**

```go
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
```

- [ ] **Step 4: Register it in `internal/mcpserver/server.go`**

In `New`, after `t.addActions(server)`, add:

```go
	t.addCommand(server)
```

- [ ] **Step 5: Run the tests**

Run: `go vet ./... && go test ./...`
Expected: vet clean, all packages `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/mcpserver
git commit -m "Add MCP run_command tool"
```

---

### Task 7: `adf mcp` command (stdio and HTTP)

**Files:**
- Create: `cmd/adf/mcp.go`
- Test: `cmd/adf/mcp_test.go`
- Modify: `cmd/adf/main.go` (register)

- [ ] **Step 1: Write the failing test in `cmd/adf/mcp_test.go`**

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireToken(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := requireToken("s3cret", ok)
	cases := map[string]int{
		"":                  http.StatusUnauthorized,
		"Bearer wrong":      http.StatusUnauthorized,
		"s3cret":            http.StatusUnauthorized,
		"Bearer s3cret":     http.StatusTeapot,
		"bearer s3cret":     http.StatusTeapot,
		"Bearer s3cret-and": http.StatusUnauthorized,
	}
	for header, want := range cases {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Authorization %q: status %d, want %d", header, rec.Code, want)
		}
	}
}

func TestNewTokenIsRandomHex(t *testing.T) {
	a, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := newToken()
	if len(a) != 64 || a == b {
		t.Fatalf("tokens %q and %q", a, b)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./cmd/adf/`
Expected: FAIL to compile (`undefined: requireToken`).

- [ ] **Step 3: Write `cmd/adf/mcp.go`**

```go
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/mcpserver"
)

func mcpCommand(a *app) *cobra.Command {
	var httpAddr, token string
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve MCP tools for agents over stdio (default) or HTTP",
		Long: "Serve MCP tools that create, drive and delete desktops.\n\n" +
			"Stdio (Claude Code, Cursor and other local clients):\n" +
			"  claude mcp add adf -- /path/to/adf mcp\n\n" +
			"HTTP (remote clients): adf mcp --http 127.0.0.1:7400\n" +
			"Requests need 'Authorization: Bearer TOKEN'. Without --token or ADF_MCP_TOKEN a\n" +
			"random token is generated and printed to stderr.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.setup(); err != nil {
				return err
			}
			server := mcpserver.New(a.fleet, mcpserver.Options{Version: version, Settle: mcpserver.DefaultSettle})
			if httpAddr == "" {
				return server.Run(cmd.Context(), &mcp.StdioTransport{})
			}
			return serveHTTP(cmd.Context(), server, httpAddr, token)
		},
	}
	cmd.Flags().StringVar(&httpAddr, "http", "", "serve streamable HTTP on this address, such as 127.0.0.1:7400")
	cmd.Flags().StringVar(&token, "token", os.Getenv("ADF_MCP_TOKEN"), "bearer token for HTTP (env ADF_MCP_TOKEN)")
	return cmd
}

func serveHTTP(ctx context.Context, server *mcp.Server, addr, token string) error {
	if token == "" {
		t, err := newToken()
		if err != nil {
			return err
		}
		token = t
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true})
	mux := http.NewServeMux()
	mux.Handle("/mcp", requireToken(token, handler))

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "adf MCP endpoint: http://%s/mcp\nAuthorization: Bearer %s\n", ln.Addr(), token)

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// requireToken rejects requests without "Authorization: Bearer <token>".
func requireToken(token string, next http.Handler) http.Handler {
	want := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, got, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// newToken returns 32 random bytes as hex.
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```

- [ ] **Step 4: Register the command in `cmd/adf/main.go`**

In `newRootCmd`, after `root.AddCommand(actionCommands(a)...)`, add:

```go
	root.AddCommand(mcpCommand(a))
```

- [ ] **Step 5: Run the tests and build**

Run: `go vet ./... && go test ./... && go build -o bin/adf ./cmd/adf && ./bin/adf mcp --help | head -5`
Expected: vet clean, all `ok`, help starts with `Serve MCP tools that create, drive and delete desktops.`

- [ ] **Step 6: Smoke test HTTP auth by hand**

```bash
./bin/adf mcp --http 127.0.0.1:7499 --token t1 & sleep 1
curl -s -o /dev/null -w "no token: %{http_code}\n" -X POST http://127.0.0.1:7499/mcp
curl -s -X POST http://127.0.0.1:7499/mcp -H "Authorization: Bearer t1" \
  -H "Content-Type: application/json" -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | head -c 300; echo
kill %1
```

Expected: `no token: 401`, then a response containing `"list_desktops"`. If the stateless server requires an `initialize` request first, record the exact response in the commit message and rely on the Go client test in Task 8 instead.

- [ ] **Step 7: Commit**

```bash
git add cmd/adf
git commit -m "Add adf mcp command with stdio and token-protected HTTP"
```

---

### Task 8: End-to-end MCP test, docs and PR

**Files:**
- Create: `tests/integration/mcp_test.go`
- Modify: `README.md`

- [ ] **Step 1: Write `tests/integration/mcp_test.go`**

```go
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
```

- [ ] **Step 2: Run it against real smolvm**

Run: `go test -tags integration -timeout 20m ./tests/integration/ -run TestMCPOverStdio -v`
Expected: PASS. The first run after this plan reprovisions the template to image version 2 (about 30 to 90 s). Open `$TMPDIR/adf-mcp-it.png` and confirm Firefox shows `about:robots` with no extra tabs.

- [ ] **Step 3: Run the whole suite**

Run: `go vet ./... && go test ./... && go test -tags integration -timeout 20m ./tests/integration/ -v`
Expected: all pass.

- [ ] **Step 4: Add a section to `README.md`, after the "Try it" section**

````markdown
## Use with Claude Code, Cursor and other MCP clients

`adf mcp` serves the desktops as MCP tools: `create_desktop`, `list_desktops`, `delete_desktop`,
`screenshot`, `click`, `move_mouse`, `drag`, `scroll`, `type_text`, `press_key`, `cursor_position`
and `run_command`. Actions return a screenshot of the result by default.

Claude Code:

```sh
claude mcp add adf -- "$(pwd)/bin/adf" mcp
```

Cursor (`~/.cursor/mcp.json`) and other clients that take a JSON config:

```json
{ "mcpServers": { "adf": { "command": "/absolute/path/to/bin/adf", "args": ["mcp"] } } }
```

Remote clients can use streamable HTTP with a bearer token:

```sh
ADF_MCP_TOKEN=change-me ./bin/adf mcp --http 127.0.0.1:7400   # endpoint: http://127.0.0.1:7400/mcp
```

The first `create_desktop` on a machine builds the desktop template (one to two minutes).
````

- [ ] **Step 5: Commit, push and open a stacked PR**

```bash
git add tests README.md
git commit -m "Add MCP end-to-end test and client setup docs"
git push -u origin feat/mcp
gh pr create --base feat/core-cli --head feat/mcp --title "MCP server: create, drive and delete desktops from any agent" --body-file <prepared body>
```

The PR is stacked on #1 (`feat/core-cli`); after #1 merges, retarget it to `main`.

---

## Out of scope (later plans)

- Live view, thumbnails and takeover in a web console (plan 3).
- Per-model adapters (Anthropic `computer_toolset_20260801`, OpenAI computer actions, Gemini normalized coordinates) for people writing their own agent loops.
- Progress notifications during the first template build.
- OAuth for the HTTP transport; it uses a static bearer token for now.
