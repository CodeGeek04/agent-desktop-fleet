// Package smolvm drives the smolvm CLI.
package smolvm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"
)

// Machine is one entry of `smolvm machine ls --json`.
type Machine struct {
	Name          string            `json:"name"`
	State         string            `json:"state"`
	PID           int               `json:"pid"`
	Image         string            `json:"image"`
	CPUs          int               `json:"cpus"`
	MemoryMiB     int               `json:"memory_mib"`
	Labels        map[string]string `json:"labels"`
	Branchable    bool              `json:"branchable"`
	ParentMachine string            `json:"parent_machine"`
	CreatedAt     int64             `json:"created_at"`
}

// Client runs smolvm commands.
type Client struct {
	Bin    string
	Runner runner.Runner
}

// New returns a client for the smolvm binary at bin.
func New(bin string, r runner.Runner) *Client { return &Client{Bin: bin, Runner: r} }

// machine runs `smolvm machine SUB --error-format json ARGS...`.
func (c *Client) machine(ctx context.Context, stdin io.Reader, sub string, args ...string) ([]byte, error) {
	full := append([]string{"machine", sub, "--error-format", "json"}, args...)
	out, err := c.Runner.Run(ctx, stdin, c.Bin, full...)
	if err != nil {
		return out, parseError(err)
	}
	return out, nil
}

// List returns all machines.
func (c *Client) List(ctx context.Context) ([]Machine, error) {
	out, err := c.machine(ctx, nil, "ls", "--json")
	if err != nil {
		return nil, err
	}
	var ms []Machine
	if err := json.Unmarshal(out, &ms); err != nil {
		return nil, fmt.Errorf("parse smolvm machine ls: %w", err)
	}
	return ms, nil
}
