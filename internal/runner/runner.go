// Package runner runs host commands. It exists so tests can replace real processes.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Runner runs a program with optional stdin and returns its stdout.
type Runner interface {
	Run(ctx context.Context, stdin io.Reader, name string, args ...string) ([]byte, error)
}

// ExitError is returned when the program ran but exited non-zero.
type ExitError struct {
	Name   string
	Args   []string
	Code   int
	Stderr []byte
}

func (e *ExitError) Error() string {
	msg := strings.TrimSpace(string(e.Stderr))
	if msg == "" {
		msg = "no output on stderr"
	}
	return fmt.Sprintf("%s exited with code %d: %s", e.Name, e.Code, msg)
}

// Exec runs real processes.
type Exec struct{}

// Run implements Runner.
func (Exec) Run(ctx context.Context, stdin io.Reader, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return stdout.Bytes(), &ExitError{Name: name, Args: args, Code: exitErr.ExitCode(), Stderr: stderr.Bytes()}
	}
	return nil, fmt.Errorf("run %s: %w", name, err)
}
