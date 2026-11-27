package runner

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestExecReturnsStdout(t *testing.T) {
	out, err := Exec{}.Run(context.Background(), nil, "sh", "-c", "echo hi")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hi\n" {
		t.Fatalf("got %q, want %q", out, "hi\n")
	}
}

func TestExecPassesStdin(t *testing.T) {
	out, err := Exec{}.Run(context.Background(), strings.NewReader("abc"), "cat")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "abc" {
		t.Fatalf("got %q, want %q", out, "abc")
	}
}

func TestExecNonZeroExitReturnsExitError(t *testing.T) {
	_, err := Exec{}.Run(context.Background(), nil, "sh", "-c", "echo bad >&2; exit 3")
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("want *ExitError, got %v", err)
	}
	if ee.Code != 3 {
		t.Fatalf("code = %d, want 3", ee.Code)
	}
	if strings.TrimSpace(string(ee.Stderr)) != "bad" {
		t.Fatalf("stderr = %q, want %q", ee.Stderr, "bad")
	}
}

func TestFakeRecordsCallsAndReplaysResults(t *testing.T) {
	ctx := context.Background()
	f := &Fake{Results: []Result{{Stdout: []byte("one")}, {Err: errors.New("boom")}}}

	out, err := f.Run(ctx, strings.NewReader("in"), "smolvm", "a", "b")
	if err != nil || string(out) != "one" {
		t.Fatalf("first call: out=%q err=%v", out, err)
	}
	want := Call{Name: "smolvm", Args: []string{"a", "b"}, Stdin: []byte("in")}
	if !reflect.DeepEqual(f.Calls[0], want) {
		t.Fatalf("call = %+v, want %+v", f.Calls[0], want)
	}

	if _, err := f.Run(ctx, nil, "x"); err == nil || err.Error() != "boom" {
		t.Fatalf("second call err = %v, want boom", err)
	}

	out, err = f.Run(ctx, nil, "y")
	if err != nil || out != nil {
		t.Fatalf("third call: out=%q err=%v, want nil nil", out, err)
	}
}
