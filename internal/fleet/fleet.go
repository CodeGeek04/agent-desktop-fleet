// Package fleet manages the desktop template and the desktops branched from it.
package fleet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/computer"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/host"
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
	Stats(ctx context.Context, name string) (smolvm.Stats, error)
	Pause(ctx context.Context, name string) error
	Resume(ctx context.Context, name string) error
}

// Errors callers can test with errors.Is.
var (
	ErrInvalidName = errors.New("invalid desktop name")
	ErrNotFound    = errors.New("no desktop named")
	ErrExists      = errors.New("desktop already exists")
	ErrNoCapacity  = errors.New("not enough room on this computer")
)

// Limits keep the host responsive. A desktop is refused when the host would drop below
// ReserveBytes of available memory after giving the desktop DesktopBytes, or when
// MaxRunning desktops already run.
type Limits struct {
	ReserveBytes int64 `json:"reserve_bytes"`
	DesktopBytes int64 `json:"desktop_bytes"`
	MaxRunning   int   `json:"max_running"` // 0 means no fixed cap
}

// DefaultLimits keeps 2 GiB for the host and budgets 512 MiB per desktop: measured idle
// desktops use 200 to 450 MB of host memory, and heavy pages can briefly use more.
func DefaultLimits() Limits {
	return Limits{ReserveBytes: 2 << 30, DesktopBytes: 512 << 20}
}

// Manager creates, lists, drives and removes desktops.
type Manager struct {
	RT           Runtime
	Image        string                     // OCI image for the template
	Log          io.Writer                  // progress messages; nil for silence
	ReadyTimeout time.Duration              // how long to wait for the template desktop
	Poll         time.Duration              // readiness poll interval
	Settle       time.Duration              // extra wait after the desktop first appears, so windows finish drawing
	Limits       Limits                     // admission limits for new and waking desktops
	Host         func() (host.Stats, error) // host probe; nil disables memory checks
	// PrebuiltImage, when set, returns a local file of the prebuilt desktop image for a new
	// template, reporting download progress. On error the template is built from Image instead.
	PrebuiltImage func(ctx context.Context, progress func(string)) (string, error)
	// PauseUnsupported, when set, explains why desktops cannot be paused on this host. Sleep
	// then stops desktops instead, and PauseIdle does nothing.
	PauseUnsupported string

	mu sync.Mutex // serializes template operations; tool calls may arrive concurrently
}

// NewManager returns a Manager with default settings.
func NewManager(rt Runtime, log io.Writer) *Manager {
	return &Manager{
		RT: rt, Image: DefaultImage, Log: log,
		ReadyTimeout: 60 * time.Second, Poll: 200 * time.Millisecond, Settle: 2 * time.Second,
		Limits: DefaultLimits(),
	}
}

var nameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`)

// ValidateName checks a user-facing desktop name.
func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("%w %q: use 1 to 40 lowercase letters, digits or dashes, not starting or ending with a dash", ErrInvalidName, name)
	}
	if name == "template" {
		return fmt.Errorf("%w %q: the name is reserved", ErrInvalidName, name)
	}
	return nil
}

type progressKey struct{}

// WithProgress returns a context whose long-running fleet operations (building the template,
// starting a desktop) report each step to fn as well as to the Manager's log.
func WithProgress(ctx context.Context, fn func(msg string)) context.Context {
	return context.WithValue(ctx, progressKey{}, fn)
}

func (m *Manager) logf(ctx context.Context, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if m.Log != nil {
		fmt.Fprintln(m.Log, msg)
	}
	if fn, ok := ctx.Value(progressKey{}).(func(string)); ok && fn != nil {
		fn(msg)
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
	m.logf(ctx, "starting template (the first start downloads the base image)")
	if err := m.RT.Start(ctx, TemplateName, true); err != nil {
		return fmt.Errorf("start template: %w", err)
	}
	return nil
}

// EnsureTemplate creates and provisions the template if needed and leaves it running as a branch source.
func (m *Manager) EnsureTemplate(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, err := m.find(ctx, TemplateName)
	if err != nil {
		return err
	}
	if t == nil {
		img := m.Image
		if m.PrebuiltImage != nil {
			path, err := m.PrebuiltImage(ctx, func(msg string) { m.logf(ctx, "%s", msg) })
			if err == nil {
				img = path
			} else {
				m.logf(ctx, "prebuilt desktop image unavailable (%v); building it from %s instead", err, m.Image)
			}
		}
		m.logf(ctx, "creating template %s from %s", TemplateName, img)
		err := m.RT.Create(ctx, smolvm.CreateOpts{
			Name: TemplateName, Image: img, CPUs: 2, MemoryMiB: 2048, OverlayGiB: 8, Net: true,
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
	m.logf(ctx, "installing desktop packages (one time, about 1 to 2 minutes)")
	steps := []struct {
		stdin string
		argv  []string
	}{
		{image.SetupScript, []string{"sh", "-c", "cat > /tmp/adf-setup.sh"}},
		{"", []string{"sh", "/tmp/adf-setup.sh"}},
		{image.StartScript, []string{"sh", "-c", "cat > /usr/local/bin/desktop-start && chmod 755 /usr/local/bin/desktop-start"}},
		{"", []string{"sh", "-c", image.ResetBrowserScript}},
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

// Desktop is one agent desktop.
type Desktop struct {
	Name    string // user-facing name, such as "a"
	Machine string // runtime machine name, such as "adf-a"
	State   string // runtime state, such as "running" or "stopped"
}

// WarmTemplate makes sure the template runs as a branch source with its desktop up,
// so every branch starts with a ready desktop.
func (m *Manager) WarmTemplate(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, err := m.find(ctx, TemplateName)
	if err != nil {
		return err
	}
	if t == nil {
		return errors.New("no desktop template yet: run 'adf init' first")
	}
	if err := m.ensureBranchSource(ctx, t); err != nil {
		return err
	}
	if _, err := m.RT.Exec(ctx, TemplateName, nil, "desktop-start"); err != nil {
		return fmt.Errorf("start template desktop: %w", err)
	}
	return m.waitDesktopReady(ctx, TemplateName)
}

// waitDesktopReady polls until Firefox has a visible window. If the first poll already
// succeeds the desktop was warm and no settle time is added.
func (m *Manager) waitDesktopReady(ctx context.Context, machine string) error {
	deadline := time.Now().Add(m.ReadyTimeout)
	for attempt := 1; ; attempt++ {
		_, err := m.RT.Exec(ctx, machine, nil,
			"env", "DISPLAY="+image.Display, "xdotool", "search", "--onlyvisible", "--class", "firefox")
		if err == nil {
			if attempt == 1 {
				return nil
			}
			return sleepCtx(ctx, m.Settle)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("desktop in %s not ready after %s: %w", machine, m.ReadyTimeout, err)
		}
		if err := sleepCtx(ctx, m.Poll); err != nil {
			return err
		}
	}
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

// CreateDesktop branches a new running desktop from the warm template.
func (m *Manager) CreateDesktop(ctx context.Context, name string) (Desktop, error) {
	if err := ValidateName(name); err != nil {
		return Desktop{}, err
	}
	machine := DesktopPrefix + name
	existing, err := m.find(ctx, machine)
	if err != nil {
		return Desktop{}, err
	}
	if existing != nil {
		return Desktop{}, fmt.Errorf("%w: %q", ErrExists, name)
	}
	if err := m.checkCapacity(ctx); err != nil {
		return Desktop{}, err
	}
	if err := m.WarmTemplate(ctx); err != nil {
		return Desktop{}, err
	}
	err = m.RT.Branch(ctx, smolvm.BranchOpts{
		From: TemplateName, Name: machine, Labels: map[string]string{LabelRole: RoleDesktop},
	})
	if err != nil {
		return Desktop{}, fmt.Errorf("branch desktop %q: %w", name, err)
	}
	return Desktop{Name: name, Machine: machine, State: "running"}, nil
}

// List returns all desktops sorted by name.
func (m *Manager) List(ctx context.Context) ([]Desktop, error) {
	ms, err := m.RT.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []Desktop
	for _, mc := range ms {
		if mc.Labels[LabelRole] != RoleDesktop || !strings.HasPrefix(mc.Name, DesktopPrefix) {
			continue
		}
		out = append(out, Desktop{Name: strings.TrimPrefix(mc.Name, DesktopPrefix), Machine: mc.Name, State: mc.State})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Manager) mustFind(ctx context.Context, name string) (*smolvm.Machine, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	mc, err := m.find(ctx, DesktopPrefix+name)
	if err != nil {
		return nil, err
	}
	if mc == nil {
		return nil, fmt.Errorf("%w %q", ErrNotFound, name)
	}
	return mc, nil
}

// Stop stops a desktop.
func (m *Manager) Stop(ctx context.Context, name string) error {
	mc, err := m.mustFind(ctx, name)
	if err != nil {
		return err
	}
	return m.RT.Stop(ctx, mc.Name)
}

// Start makes a desktop usable again. A paused desktop resumes exactly where it was. A
// stopped desktop keeps its disk (files, browser profile) but not its memory, so it boots
// and its screen session restarts.
func (m *Manager) Start(ctx context.Context, name string) error {
	mc, err := m.mustFind(ctx, name)
	if err != nil {
		return err
	}
	switch mc.State {
	case "running":
	case "paused":
		if err := m.checkCapacity(ctx); err != nil {
			return err
		}
		m.logf(ctx, "resuming desktop %q", name)
		if err := m.RT.Resume(ctx, mc.Name); err != nil {
			return fmt.Errorf("resume %q: %w", name, err)
		}
		return m.waitDesktopReady(ctx, mc.Name)
	default:
		if err := m.checkCapacity(ctx); err != nil {
			return err
		}
		m.logf(ctx, "starting desktop %q", name)
		if err := m.RT.Start(ctx, mc.Name, false); err != nil {
			return fmt.Errorf("start %q: %w", name, err)
		}
	}
	m.logf(ctx, "starting the screen session in %q", name)
	if _, err := m.RT.Exec(ctx, mc.Name, nil, "desktop-start"); err != nil {
		return fmt.Errorf("start desktop session in %q: %w", name, err)
	}
	return m.waitDesktopReady(ctx, mc.Name)
}

// Sleep frees a running desktop's memory: it pauses the desktop where pausing works, and
// stops it otherwise (files kept, open windows closed). It reports "paused" or "stopped".
func (m *Manager) Sleep(ctx context.Context, name string) (string, error) {
	if m.PauseUnsupported != "" {
		return "stopped", m.Stop(ctx, name)
	}
	return "paused", m.Pause(ctx, name)
}

// Pause saves a running desktop's memory to disk and frees it. Any action resumes it.
func (m *Manager) Pause(ctx context.Context, name string) error {
	mc, err := m.mustFind(ctx, name)
	if err != nil {
		return err
	}
	if mc.State != "running" {
		return nil
	}
	return m.RT.Pause(ctx, mc.Name)
}

// Delete stops a desktop if it runs and removes it.
func (m *Manager) Delete(ctx context.Context, name string) error {
	mc, err := m.mustFind(ctx, name)
	if err != nil {
		return err
	}
	if mc.State == "running" {
		if err := m.RT.Stop(ctx, mc.Name); err != nil {
			return fmt.Errorf("stop %q: %w", name, err)
		}
	}
	return m.RT.Delete(ctx, mc.Name, false)
}

// Exec runs argv inside a desktop and returns its stdout.
func (m *Manager) Exec(ctx context.Context, name string, argv ...string) ([]byte, error) {
	return m.RT.Exec(ctx, DesktopPrefix+name, nil, argv...)
}

// Computer returns the computer-use actions for a desktop. An action on a stopped desktop
// (for example after the host rebooted) starts it and retries once, so agents are not stuck.
func (m *Manager) Computer(name string) *computer.Computer {
	machine := DesktopPrefix + name
	exec := func(ctx context.Context, stdin io.Reader, argv ...string) ([]byte, error) {
		out, err := m.RT.Exec(ctx, machine, stdin, argv...)
		if err == nil || stdin != nil || !m.isAsleep(ctx, machine, err) {
			return out, err
		}
		m.logf(ctx, "desktop %q is not running; waking it", name)
		if serr := m.Start(ctx, name); serr != nil {
			return out, fmt.Errorf("%w (starting the stopped desktop also failed: %v)", err, serr)
		}
		return m.RT.Exec(ctx, machine, stdin, argv...)
	}
	return computer.New(exec, computer.Screen{Display: image.Display, Width: image.Width, Height: image.Height})
}

// Stats returns a running desktop's CPU and memory use, as seen inside the guest.
func (m *Manager) Stats(ctx context.Context, name string) (smolvm.Stats, error) {
	if err := ValidateName(name); err != nil {
		return smolvm.Stats{}, err
	}
	return m.RT.Stats(ctx, DesktopPrefix+name)
}

// TemplateState returns the template's runtime state, or "missing" before the first build.
func (m *Manager) TemplateState(ctx context.Context) (string, error) {
	t, err := m.find(ctx, TemplateName)
	if err != nil {
		return "", err
	}
	if t == nil {
		return "missing", nil
	}
	return t.State, nil
}

// GuestMemory is memory inside a desktop. UsedBytes leaves out pages the host has taken back
// through the balloon: smolvm memory is elastic, and the guest counts ballooned pages as used.
type GuestMemory struct {
	TotalBytes int64
	UsedBytes  int64
}

// Memory reads a running desktop's memory use from /proc/meminfo.
func (m *Manager) Memory(ctx context.Context, name string) (GuestMemory, error) {
	if err := ValidateName(name); err != nil {
		return GuestMemory{}, err
	}
	out, err := m.RT.Exec(ctx, DesktopPrefix+name, nil, "cat", "/proc/meminfo")
	if err != nil {
		return GuestMemory{}, err
	}
	return parseMeminfo(out)
}

func parseMeminfo(data []byte) (GuestMemory, error) {
	kib := map[string]int64{}
	for _, line := range strings.Split(string(data), "\n") {
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		if n, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
			kib[key] = n
		}
	}
	total, okTotal := kib["MemTotal"]
	avail, okAvail := kib["MemAvailable"]
	if !okTotal || !okAvail {
		return GuestMemory{}, errors.New("meminfo: missing MemTotal or MemAvailable")
	}
	used := total - avail - kib["Balloon"]
	if used < 0 {
		used = 0
	}
	return GuestMemory{TotalBytes: total * 1024, UsedBytes: used * 1024}, nil
}

// isAsleep reports whether err came from smolvm and the machine exists but is paused or stopped.
func (m *Manager) isAsleep(ctx context.Context, machine string, err error) bool {
	var se *smolvm.Error
	if !errors.As(err, &se) || smolvm.IsNotFound(err) {
		return false
	}
	mc, ferr := m.find(ctx, machine)
	return ferr == nil && mc != nil && mc.State != "running"
}

func isDesktop(mc smolvm.Machine) bool {
	return mc.Labels[LabelRole] == RoleDesktop && strings.HasPrefix(mc.Name, DesktopPrefix)
}

// checkCapacity refuses a new or waking desktop that would overload the host.
func (m *Manager) checkCapacity(ctx context.Context) error {
	if m.Limits.MaxRunning > 0 {
		ms, err := m.RT.List(ctx)
		if err != nil {
			return err
		}
		running := 0
		for _, mc := range ms {
			if isDesktop(mc) && mc.State == "running" {
				running++
			}
		}
		if running >= m.Limits.MaxRunning {
			return fmt.Errorf("%w: %d desktops are running and the limit is %d; stop, pause or delete one you no longer need, or raise ADF_MAX_DESKTOPS",
				ErrNoCapacity, running, m.Limits.MaxRunning)
		}
	}
	if m.Host == nil {
		return nil
	}
	hs, err := m.Host()
	if err != nil {
		return nil // a failed probe must not block work
	}
	if hs.AvailableMemBytes < m.Limits.ReserveBytes+m.Limits.DesktopBytes {
		return fmt.Errorf("%w: %s of memory is available and %s stays reserved for the computer itself, so another desktop (about %s) would overload it; stop or delete desktops you no longer need (list_desktops shows them)",
			ErrNoCapacity, HumanBytes(hs.AvailableMemBytes), HumanBytes(m.Limits.ReserveBytes), HumanBytes(m.Limits.DesktopBytes))
	}
	return nil
}

// HumanBytes formats a size such as "1.5 GB" or "512 MB".
func HumanBytes(b int64) string {
	if b >= 1<<30 {
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	}
	return fmt.Sprintf("%d MB", b>>20)
}

// Capacity is how full the host is and how many more desktops fit.
type Capacity struct {
	Host        host.Stats `json:"host"`
	HostKnown   bool       `json:"host_known"`
	Limits      Limits     `json:"limits"`
	Running     int        `json:"running"`
	Paused      int        `json:"paused"`
	Stopped     int        `json:"stopped"`
	MoreThatFit int        `json:"more_that_fit"` // -1 when unknown
}

// Capacity reports host memory and CPU, desktops by state, and how many more fit now.
func (m *Manager) Capacity(ctx context.Context) (Capacity, error) {
	ms, err := m.RT.List(ctx)
	if err != nil {
		return Capacity{}, err
	}
	c := Capacity{Limits: m.Limits, MoreThatFit: -1}
	for _, mc := range ms {
		if !isDesktop(mc) {
			continue
		}
		switch mc.State {
		case "running":
			c.Running++
		case "paused":
			c.Paused++
		default:
			c.Stopped++
		}
	}
	if m.Host != nil {
		if hs, err := m.Host(); err == nil {
			c.Host, c.HostKnown = hs, true
			fit := int64(0)
			if m.Limits.DesktopBytes > 0 {
				fit = (hs.AvailableMemBytes - m.Limits.ReserveBytes) / m.Limits.DesktopBytes
			}
			c.MoreThatFit = int(max(fit, 0))
		}
	}
	if m.Limits.MaxRunning > 0 {
		room := max(m.Limits.MaxRunning-c.Running, 0)
		if c.MoreThatFit < 0 || room < c.MoreThatFit {
			c.MoreThatFit = room
		}
	}
	return c, nil
}

// PauseIdle pauses running desktops whose latest action is older than idle. lastActive maps
// desktop names to their latest action; desktops without one count from their creation.
func (m *Manager) PauseIdle(ctx context.Context, lastActive map[string]time.Time, idle time.Duration, now time.Time) ([]string, error) {
	if m.PauseUnsupported != "" {
		return nil, nil
	}
	ms, err := m.RT.List(ctx)
	if err != nil {
		return nil, err
	}
	var paused []string
	for _, mc := range ms {
		if !isDesktop(mc) || mc.State != "running" {
			continue
		}
		name := strings.TrimPrefix(mc.Name, DesktopPrefix)
		var last time.Time
		if mc.CreatedAt > 0 {
			last = time.Unix(mc.CreatedAt, 0)
		}
		if t, ok := lastActive[name]; ok && t.After(last) {
			last = t
		}
		if last.IsZero() || now.Sub(last) < idle {
			continue
		}
		m.logf(ctx, "pausing idle desktop %q (no activity for %s)", name, now.Sub(last).Round(time.Minute))
		if err := m.RT.Pause(ctx, mc.Name); err != nil {
			m.logf(ctx, "could not pause %q: %v", name, err)
			continue
		}
		paused = append(paused, name)
	}
	return paused, nil
}
