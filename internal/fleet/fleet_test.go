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
