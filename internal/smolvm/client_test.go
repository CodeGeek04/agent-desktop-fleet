package smolvm

import (
	"context"
	"errors"
	"reflect"
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
