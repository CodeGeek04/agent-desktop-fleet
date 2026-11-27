package smolvm

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"
)

var ctx = context.Background()

func TestListParsesMachines(t *testing.T) {
	f := &runner.Fake{Results: []runner.Result{{Stdout: []byte(`[
	 {"name":"adf-a","state":"running","pid":42,"image":"debian:trixie-slim","cpus":2,"memory_mib":2048,
	  "labels":{"adf.role":"desktop"},"branchable":false,"parent_machine":"adf-template","created_at":1795781577},
	 {"name":"adf-template","state":"stopped","pid":null,"labels":{},"branchable":true,"parent_machine":null}
	]`)}}}
	ms, err := New("smolvm", f).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 {
		t.Fatalf("got %d machines, want 2", len(ms))
	}
	a := ms[0]
	if a.Name != "adf-a" || a.State != "running" || a.PID != 42 || a.Labels["adf.role"] != "desktop" ||
		a.ParentMachine != "adf-template" || a.MemoryMiB != 2048 {
		t.Fatalf("unexpected first machine: %+v", a)
	}
	if ms[1].PID != 0 || !ms[1].Branchable || ms[1].ParentMachine != "" {
		t.Fatalf("unexpected second machine: %+v", ms[1])
	}
	wantArgs := []string{"machine", "ls", "--error-format", "json", "--json"}
	if !reflect.DeepEqual(f.Calls[0].Args, wantArgs) {
		t.Fatalf("args = %q, want %q", f.Calls[0].Args, wantArgs)
	}
}

func TestSmolvmJSONErrorBecomesError(t *testing.T) {
	f := &runner.Fake{Results: []runner.Result{{Err: &runner.ExitError{
		Name: "smolvm", Code: 1,
		Stderr: []byte("{\"error\":{\"code\":\"NOT_FOUND\",\"message\":\"vm not found: x\"}}\n"),
	}}}}
	_, err := New("smolvm", f).List(ctx)
	if !IsNotFound(err) {
		t.Fatalf("want not-found error, got %v", err)
	}
	var e *Error
	if !errors.As(err, &e) || e.Message != "vm not found: x" {
		t.Fatalf("unexpected error %#v", err)
	}
}

func TestPlainStderrKeepsExitError(t *testing.T) {
	f := &runner.Fake{Results: []runner.Result{{Err: &runner.ExitError{Name: "smolvm", Code: 7, Stderr: []byte("oops\n")}}}}
	_, err := New("smolvm", f).List(ctx)
	var ee *runner.ExitError
	if !errors.As(err, &ee) || ee.Code != 7 {
		t.Fatalf("want *runner.ExitError with code 7, got %v", err)
	}
	if IsNotFound(err) {
		t.Fatal("plain stderr must not be treated as not found")
	}
}

func TestLifecycleArgs(t *testing.T) {
	cases := []struct {
		name string
		call func(c *Client) error
		want []string
	}{
		{"create", func(c *Client) error {
			return c.Create(ctx, CreateOpts{Name: "adf-template", Image: "debian:trixie-slim", CPUs: 2, MemoryMiB: 2048,
				OverlayGiB: 8, Net: true, Labels: map[string]string{"b": "2", "a": "1"}})
		}, []string{"machine", "create", "--error-format", "json", "--name", "adf-template", "--image", "debian:trixie-slim",
			"--cpus", "2", "--mem", "2048", "--overlay", "8", "--net", "--label", "a=1", "--label", "b=2"}},
		{"start", func(c *Client) error { return c.Start(ctx, "n", false) },
			[]string{"machine", "start", "--error-format", "json", "--name", "n"}},
		{"start branchable", func(c *Client) error { return c.Start(ctx, "n", true) },
			[]string{"machine", "start", "--error-format", "json", "--name", "n", "--branchable"}},
		{"branch", func(c *Client) error {
			return c.Branch(ctx, BranchOpts{From: "adf-template", Name: "adf-a", Labels: map[string]string{"adf.role": "desktop"}})
		}, []string{"machine", "branch", "--error-format", "json", "--from", "adf-template", "--name", "adf-a", "--label", "adf.role=desktop"}},
		{"stop", func(c *Client) error { return c.Stop(ctx, "n") },
			[]string{"machine", "stop", "--error-format", "json", "--name", "n"}},
		{"delete", func(c *Client) error { return c.Delete(ctx, "n", false) },
			[]string{"machine", "delete", "--error-format", "json", "--name", "n", "--force"}},
		{"delete cascade", func(c *Client) error { return c.Delete(ctx, "n", true) },
			[]string{"machine", "delete", "--error-format", "json", "--name", "n", "--force", "--cascade"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &runner.Fake{}
			if err := tc.call(New("smolvm", f)); err != nil {
				t.Fatal(err)
			}
			if f.Calls[0].Name != "smolvm" || !reflect.DeepEqual(f.Calls[0].Args, tc.want) {
				t.Fatalf("got %s %q\nwant smolvm %q", f.Calls[0].Name, f.Calls[0].Args, tc.want)
			}
		})
	}
}

func TestExecArgsAndOutput(t *testing.T) {
	f := &runner.Fake{Results: []runner.Result{{Stdout: []byte("hi\n")}}}
	out, err := New("smolvm", f).Exec(ctx, "adf-a", nil, "echo", "hi")
	if err != nil || string(out) != "hi\n" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	want := []string{"machine", "exec", "--error-format", "json", "--name", "adf-a", "--", "echo", "hi"}
	if !reflect.DeepEqual(f.Calls[0].Args, want) {
		t.Fatalf("args = %q, want %q", f.Calls[0].Args, want)
	}
}

func TestExecWithStdinAddsInteractiveFlag(t *testing.T) {
	f := &runner.Fake{}
	if _, err := New("smolvm", f).Exec(ctx, "adf-a", strings.NewReader("data"), "sh", "-c", "cat > /tmp/x"); err != nil {
		t.Fatal(err)
	}
	want := []string{"machine", "exec", "--error-format", "json", "--name", "adf-a", "-i", "--", "sh", "-c", "cat > /tmp/x"}
	if !reflect.DeepEqual(f.Calls[0].Args, want) {
		t.Fatalf("args = %q, want %q", f.Calls[0].Args, want)
	}
	if string(f.Calls[0].Stdin) != "data" {
		t.Fatalf("stdin = %q, want data", f.Calls[0].Stdin)
	}
}

func TestVersion(t *testing.T) {
	f := &runner.Fake{Results: []runner.Result{{Stdout: []byte("smolvm 1.37.7\n")}}}
	v, err := New("smolvm", f).Version(ctx)
	if err != nil || v != "smolvm 1.37.7" {
		t.Fatalf("v=%q err=%v", v, err)
	}
	if !reflect.DeepEqual(f.Calls[0].Args, []string{"--version"}) {
		t.Fatalf("args = %q", f.Calls[0].Args)
	}
}

func TestStats(t *testing.T) {
	f := &runner.Fake{Results: []runner.Result{{Stdout: []byte(`{"cpu_percent":12.5,"cpus":2,
	  "memory":{"total_bytes":2086825984,"used_bytes":398458880,"available_bytes":1600000000},"name":"adf-a"}`)}}}
	s, err := New("smolvm", f).Stats(ctx, "adf-a")
	if err != nil {
		t.Fatal(err)
	}
	if s.CPUPercent != 12.5 || s.Memory.TotalBytes != 2086825984 || s.Memory.UsedBytes != 398458880 {
		t.Fatalf("stats = %+v", s)
	}
	want := []string{"machine", "stats", "--error-format", "json", "--name", "adf-a", "--json"}
	if !reflect.DeepEqual(f.Calls[0].Args, want) {
		t.Fatalf("args = %q, want %q", f.Calls[0].Args, want)
	}
}
