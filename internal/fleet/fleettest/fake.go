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
	// StatsFunc decides each Stats result. Nil returns zero stats.
	StatsFunc func(name string) (smolvm.Stats, error)
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
	m, known := f.Machines[name]
	stopped := known && m.State != "running"
	f.mu.Unlock()
	if stopped {
		// Like smolvm: exec into a stopped machine fails with a generic error.
		return nil, &smolvm.Error{Code: "SMOLVM_ERROR", Message: "machine '" + name + "' is not running"}
	}
	if fn != nil {
		return fn(name, in, argv)
	}
	return nil, nil
}

// Stats implements fleet.Runtime. Calls are not added to the call log, because metrics
// collectors poll in the background and would make logs nondeterministic.
func (f *Fake) Stats(_ context.Context, name string) (smolvm.Stats, error) {
	f.mu.Lock()
	fn := f.StatsFunc
	f.mu.Unlock()
	if fn != nil {
		return fn(name)
	}
	return smolvm.Stats{}, nil
}

// Pause implements fleet.Runtime.
func (f *Fake) Pause(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("pause " + name)
	m, ok := f.Machines[name]
	if !ok || m.State != "running" {
		return fmt.Errorf("machine %s is not running", name)
	}
	m.State = "paused"
	return nil
}

// Resume implements fleet.Runtime.
func (f *Fake) Resume(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("resume " + name)
	m, ok := f.Machines[name]
	if !ok || m.State != "paused" {
		return fmt.Errorf("machine %s is not paused", name)
	}
	m.State = "running"
	return nil
}
