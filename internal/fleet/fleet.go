// Package fleet manages the desktop template and the desktops branched from it.
package fleet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

// Names and labels used on the runtime.
const (
	TemplateName  = "adf-template"
	DesktopPrefix = "adf-"
	LabelRole     = "adf.role"
	RoleTemplate  = "template"
	RoleDesktop   = "desktop"
	DefaultImage  = "debian:trixie-slim"
)

// Runtime is what the fleet needs from a VM runtime. *smolvm.Client implements it.
type Runtime interface {
	List(ctx context.Context) ([]smolvm.Machine, error)
	Create(ctx context.Context, opts smolvm.CreateOpts) error
	Start(ctx context.Context, name string, branchable bool) error
	Branch(ctx context.Context, opts smolvm.BranchOpts) error
	Stop(ctx context.Context, name string) error
	Delete(ctx context.Context, name string, cascade bool) error
	Exec(ctx context.Context, name string, stdin io.Reader, argv ...string) ([]byte, error)
}

// Manager creates, lists, drives and removes desktops.
type Manager struct {
	RT           Runtime
	Image        string        // OCI image for the template
	Log          io.Writer     // progress messages; nil for silence
	ReadyTimeout time.Duration // how long to wait for the template desktop
	Poll         time.Duration // readiness poll interval
	Settle       time.Duration // extra wait after the desktop first appears, so windows finish drawing
}

// NewManager returns a Manager with default settings.
func NewManager(rt Runtime, log io.Writer) *Manager {
	return &Manager{
		RT: rt, Image: DefaultImage, Log: log,
		ReadyTimeout: 60 * time.Second, Poll: 200 * time.Millisecond, Settle: 2 * time.Second,
	}
}

var nameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)

// ValidateName checks a user-facing desktop name.
func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid desktop name %q: use 1 to 40 lowercase letters, digits or dashes, not starting or ending with a dash", name)
	}
	if name == "template" {
		return errors.New(`"template" is a reserved name`)
	}
	return nil
}

func (m *Manager) logf(format string, args ...any) {
	if m.Log != nil {
		fmt.Fprintf(m.Log, format+"\n", args...)
	}
}

func (m *Manager) find(ctx context.Context, machine string) (*smolvm.Machine, error) {
	ms, err := m.RT.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range ms {
		if ms[i].Name == machine {
			return &ms[i], nil
		}
	}
	return nil, nil
}

// ensureBranchSource leaves the template running with --branchable.
func (m *Manager) ensureBranchSource(ctx context.Context, t *smolvm.Machine) error {
	if t.State == "running" && t.Branchable {
		return nil
	}
	if t.State == "running" {
		if err := m.RT.Stop(ctx, TemplateName); err != nil {
			return fmt.Errorf("stop template: %w", err)
		}
	}
	m.logf("starting template (the first start downloads the base image)")
	if err := m.RT.Start(ctx, TemplateName, true); err != nil {
		return fmt.Errorf("start template: %w", err)
	}
	return nil
}

// EnsureTemplate creates and provisions the template if needed and leaves it running as a branch source.
func (m *Manager) EnsureTemplate(ctx context.Context) error {
	t, err := m.find(ctx, TemplateName)
	if err != nil {
		return err
	}
	if t == nil {
		m.logf("creating template %s from %s", TemplateName, m.Image)
		err := m.RT.Create(ctx, smolvm.CreateOpts{
			Name: TemplateName, Image: m.Image, CPUs: 2, MemoryMiB: 2048, OverlayGiB: 8, Net: true,
			Labels: map[string]string{LabelRole: RoleTemplate},
		})
		if err != nil {
			return fmt.Errorf("create template: %w", err)
		}
		t = &smolvm.Machine{Name: TemplateName, State: "stopped"}
	}
	if err := m.ensureBranchSource(ctx, t); err != nil {
		return err
	}
	out, err := m.RT.Exec(ctx, TemplateName, nil, "cat", "/etc/adf-image-version")
	if err == nil && strings.TrimSpace(string(out)) == image.Version {
		return nil
	}
	m.logf("installing desktop packages (one time, about 1 to 2 minutes)")
	steps := []struct {
		stdin string
		argv  []string
	}{
		{image.SetupScript, []string{"sh", "-c", "cat > /tmp/adf-setup.sh"}},
		{"", []string{"sh", "/tmp/adf-setup.sh"}},
		{image.StartScript, []string{"sh", "-c", "cat > /usr/local/bin/desktop-start && chmod 755 /usr/local/bin/desktop-start"}},
		{"", []string{"sh", "-c", "echo " + image.Version + " > /etc/adf-image-version"}},
	}
	for _, s := range steps {
		var in io.Reader
		if s.stdin != "" {
			in = strings.NewReader(s.stdin)
		}
		if _, err := m.RT.Exec(ctx, TemplateName, in, s.argv...); err != nil {
			return fmt.Errorf("provision template (%s): %w", strings.Join(s.argv, " "), err)
		}
	}
	return nil
}
