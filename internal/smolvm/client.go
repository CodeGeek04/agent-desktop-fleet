// Package smolvm drives the smolvm CLI.
package smolvm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"

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

// CreateOpts configures a new machine.
type CreateOpts struct {
	Name       string
	Image      string
	CPUs       int
	MemoryMiB  int
	OverlayGiB int
	Net        bool
	Labels     map[string]string
}

// BranchOpts configures a branch of a running branchable machine.
type BranchOpts struct {
	From   string
	Name   string
	Labels map[string]string
}

// labelArgs returns --label flags in a stable order.
func labelArgs(labels map[string]string) []string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var args []string
	for _, k := range keys {
		args = append(args, "--label", k+"="+labels[k])
	}
	return args
}

// Create defines a machine without starting it.
func (c *Client) Create(ctx context.Context, o CreateOpts) error {
	args := []string{"--name", o.Name, "--image", o.Image,
		"--cpus", strconv.Itoa(o.CPUs), "--mem", strconv.Itoa(o.MemoryMiB), "--overlay", strconv.Itoa(o.OverlayGiB)}
	if o.Net {
		args = append(args, "--net")
	}
	args = append(args, labelArgs(o.Labels)...)
	_, err := c.machine(ctx, nil, "create", args...)
	return err
}

// Start boots a machine. A branchable machine can be branched while it runs.
func (c *Client) Start(ctx context.Context, name string, branchable bool) error {
	args := []string{"--name", name}
	if branchable {
		args = append(args, "--branchable")
	}
	_, err := c.machine(ctx, nil, "start", args...)
	return err
}

// Branch creates a running copy-on-write child of a running branchable machine.
func (c *Client) Branch(ctx context.Context, o BranchOpts) error {
	args := append([]string{"--from", o.From, "--name", o.Name}, labelArgs(o.Labels)...)
	_, err := c.machine(ctx, nil, "branch", args...)
	return err
}

// Stop stops a machine.
func (c *Client) Stop(ctx context.Context, name string) error {
	_, err := c.machine(ctx, nil, "stop", "--name", name)
	return err
}

// Delete removes a machine. cascade also removes children branched from it.
func (c *Client) Delete(ctx context.Context, name string, cascade bool) error {
	args := []string{"--name", name, "--force"}
	if cascade {
		args = append(args, "--cascade")
	}
	_, err := c.machine(ctx, nil, "delete", args...)
	return err
}
