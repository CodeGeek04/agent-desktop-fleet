package fleet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

var ctx = context.Background()

// fakeRT is an in-memory Runtime that records a readable call log.
type fakeRT struct {
	machines map[string]*smolvm.Machine
	calls    []string
	// exec decides each Exec result; nil means success with empty output.
	exec func(name, stdin string, argv []string) ([]byte, error)
}

func newFakeRT() *fakeRT { return &fakeRT{machines: map[string]*smolvm.Machine{}} }

func (f *fakeRT) List(context.Context) ([]smolvm.Machine, error) {
	var out []smolvm.Machine
	for _, m := range f.machines {
		out = append(out, *m)
	}
	return out, nil
}

func (f *fakeRT) Create(_ context.Context, o smolvm.CreateOpts) error {
	f.calls = append(f.calls, "create "+o.Name)
	f.machines[o.Name] = &smolvm.Machine{Name: o.Name, State: "stopped", Labels: o.Labels}
	return nil
}

func (f *fakeRT) Start(_ context.Context, name string, branchable bool) error {
	f.calls = append(f.calls, fmt.Sprintf("start %s branchable=%v", name, branchable))
	m := f.machines[name]
	m.State, m.Branchable = "running", branchable
	return nil
}

func (f *fakeRT) Branch(_ context.Context, o smolvm.BranchOpts) error {
	f.calls = append(f.calls, fmt.Sprintf("branch %s->%s role=%s", o.From, o.Name, o.Labels[LabelRole]))
	f.machines[o.Name] = &smolvm.Machine{Name: o.Name, State: "running", Labels: o.Labels, ParentMachine: o.From}
	return nil
}

func (f *fakeRT) Stop(_ context.Context, name string) error {
	f.calls = append(f.calls, "stop "+name)
	f.machines[name].State = "stopped"
	return nil
}

func (f *fakeRT) Delete(_ context.Context, name string, cascade bool) error {
	f.calls = append(f.calls, fmt.Sprintf("delete %s cascade=%v", name, cascade))
	delete(f.machines, name)
	return nil
}

func (f *fakeRT) Exec(_ context.Context, name string, stdin io.Reader, argv ...string) ([]byte, error) {
	in := ""
	if stdin != nil {
		b, _ := io.ReadAll(stdin)
		in = string(b)
	}
	f.calls = append(f.calls, "exec "+name+" "+strings.Join(argv, " "))
	if f.exec != nil {
		return f.exec(name, in, argv)
	}
	return nil, nil
}

func testManager(rt *fakeRT) *Manager {
	m := NewManager(rt, nil)
	m.Poll, m.Settle, m.ReadyTimeout = time.Millisecond, 0, time.Second
	return m
}

func TestEnsureTemplateCreatesAndProvisions(t *testing.T) {
	rt := newFakeRT()
	var gotSetup, gotStart string
	rt.exec = func(name, stdin string, argv []string) ([]byte, error) {
		switch {
		case argv[0] == "cat":
			return nil, errors.New("no such file")
		case strings.Contains(strings.Join(argv, " "), "adf-setup.sh") && stdin != "":
			gotSetup = stdin
		case strings.Contains(strings.Join(argv, " "), "desktop-start") && stdin != "":
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
	if !reflect.DeepEqual(rt.calls, want) {
		t.Fatalf("calls:\n%s\nwant:\n%s", strings.Join(rt.calls, "\n"), strings.Join(want, "\n"))
	}
	if gotSetup != image.SetupScript || gotStart != image.StartScript {
		t.Fatal("setup or start script was not sent as stdin")
	}
	if rt.machines[TemplateName].Labels[LabelRole] != RoleTemplate {
		t.Fatal("template is missing its role label")
	}
}

func TestEnsureTemplateSkipsProvisionedTemplate(t *testing.T) {
	rt := newFakeRT()
	rt.machines[TemplateName] = &smolvm.Machine{Name: TemplateName, State: "running", Branchable: true}
	rt.exec = func(_, _ string, argv []string) ([]byte, error) { return []byte(image.Version + "\n"), nil }
	if err := testManager(rt).EnsureTemplate(ctx); err != nil {
		t.Fatal(err)
	}
	want := []string{"exec adf-template cat /etc/adf-image-version"}
	if !reflect.DeepEqual(rt.calls, want) {
		t.Fatalf("calls = %q, want %q", rt.calls, want)
	}
}

func TestEnsureTemplateRestartsNonBranchableTemplate(t *testing.T) {
	rt := newFakeRT()
	rt.machines[TemplateName] = &smolvm.Machine{Name: TemplateName, State: "running", Branchable: false}
	rt.exec = func(_, _ string, argv []string) ([]byte, error) { return []byte(image.Version), nil }
	if err := testManager(rt).EnsureTemplate(ctx); err != nil {
		t.Fatal(err)
	}
	if rt.calls[0] != "stop adf-template" || rt.calls[1] != "start adf-template branchable=true" {
		t.Fatalf("calls = %q", rt.calls)
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

func warmTemplate(rt *fakeRT) {
	rt.machines[TemplateName] = &smolvm.Machine{Name: TemplateName, State: "running", Branchable: true,
		Labels: map[string]string{LabelRole: RoleTemplate}}
}

func TestWarmTemplateMissingTellsUserToInit(t *testing.T) {
	err := testManager(newFakeRT()).WarmTemplate(ctx)
	if err == nil || !strings.Contains(err.Error(), "adf init") {
		t.Fatalf("err = %v, want a hint to run adf init", err)
	}
}

func TestWarmTemplateStartsDesktopAndWaits(t *testing.T) {
	rt := newFakeRT()
	rt.machines[TemplateName] = &smolvm.Machine{Name: TemplateName, State: "stopped"}
	searches := 0
	rt.exec = func(_, _ string, argv []string) ([]byte, error) {
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
	if rt.calls[0] != "start adf-template branchable=true" || rt.calls[1] != "exec adf-template desktop-start" {
		t.Fatalf("calls = %q", rt.calls)
	}
	if searches != 3 {
		t.Fatalf("searched %d times, want 3", searches)
	}
}

func TestWarmTemplateTimesOut(t *testing.T) {
	rt := newFakeRT()
	warmTemplate(rt)
	rt.exec = func(_, _ string, argv []string) ([]byte, error) {
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
	rt := newFakeRT()
	warmTemplate(rt)
	d, err := testManager(rt).CreateDesktop(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if d != (Desktop{Name: "a", Machine: "adf-a", State: "running"}) {
		t.Fatalf("desktop = %+v", d)
	}
	last := rt.calls[len(rt.calls)-1]
	if last != "branch adf-template->adf-a role=desktop" {
		t.Fatalf("last call = %q", last)
	}
}

func TestCreateDesktopRejectsBadAndDuplicateNames(t *testing.T) {
	rt := newFakeRT()
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
	rt := newFakeRT()
	warmTemplate(rt)
	rt.machines["adf-b"] = &smolvm.Machine{Name: "adf-b", State: "stopped", Labels: map[string]string{LabelRole: RoleDesktop}}
	rt.machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{LabelRole: RoleDesktop}}
	rt.machines["other"] = &smolvm.Machine{Name: "other", State: "running", Labels: map[string]string{LabelRole: RoleDesktop}}
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
	rt := newFakeRT()
	rt.machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{LabelRole: RoleDesktop}}
	if err := testManager(rt).Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	want := []string{"stop adf-a", "delete adf-a cascade=false"}
	if !reflect.DeepEqual(rt.calls, want) {
		t.Fatalf("calls = %q, want %q", rt.calls, want)
	}
}

func TestDeleteUnknownDesktop(t *testing.T) {
	if err := testManager(newFakeRT()).Delete(ctx, "nope"); err == nil {
		t.Fatal("expected not found error")
	}
}

func TestComputerRunsInsideTheDesktop(t *testing.T) {
	rt := newFakeRT()
	if err := testManager(rt).Computer("a").Key(ctx, "Return"); err != nil {
		t.Fatal(err)
	}
	if rt.calls[0] != "exec adf-a env DISPLAY=:1 xdotool key -- Return" {
		t.Fatalf("call = %q", rt.calls[0])
	}
}
