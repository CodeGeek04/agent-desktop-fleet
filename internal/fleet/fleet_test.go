package fleet

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet/fleettest"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

var ctx = context.Background()

func testManager(rt *fleettest.Fake) *Manager {
	m := NewManager(rt, nil)
	m.Poll, m.Settle, m.ReadyTimeout = time.Millisecond, 0, time.Second
	return m
}

func warmTemplate(rt *fleettest.Fake) {
	rt.Machines[TemplateName] = &smolvm.Machine{Name: TemplateName, State: "running", Branchable: true,
		Labels: map[string]string{LabelRole: RoleTemplate}}
}

func TestEnsureTemplateCreatesAndProvisions(t *testing.T) {
	rt := fleettest.New()
	var gotSetup, gotStart string
	rt.ExecFunc = func(name, stdin string, argv []string) ([]byte, error) {
		joined := strings.Join(argv, " ")
		switch {
		case argv[0] == "cat":
			return nil, errors.New("no such file")
		case strings.Contains(joined, "adf-setup.sh") && stdin != "":
			gotSetup = stdin
		case strings.Contains(joined, "desktop-start") && stdin != "":
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
		"exec adf-template sh -c " + image.ResetBrowserScript,
		"exec adf-template sh -c echo " + image.Version + " > /etc/adf-image-version",
	}
	if got := rt.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if gotSetup != image.SetupScript || gotStart != image.StartScript {
		t.Fatal("setup or start script was not sent as stdin")
	}
	if rt.Machines[TemplateName].Labels[LabelRole] != RoleTemplate {
		t.Fatal("template is missing its role label")
	}
}

func TestEnsureTemplateSkipsProvisionedTemplate(t *testing.T) {
	rt := fleettest.New()
	warmTemplate(rt)
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) { return []byte(image.Version + "\n"), nil }
	if err := testManager(rt).EnsureTemplate(ctx); err != nil {
		t.Fatal(err)
	}
	want := []string{"exec adf-template cat /etc/adf-image-version"}
	if got := rt.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestEnsureTemplateRestartsNonBranchableTemplate(t *testing.T) {
	rt := fleettest.New()
	rt.Machines[TemplateName] = &smolvm.Machine{Name: TemplateName, State: "running", Branchable: false}
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) { return []byte(image.Version), nil }
	if err := testManager(rt).EnsureTemplate(ctx); err != nil {
		t.Fatal(err)
	}
	if got := rt.Calls(); got[0] != "stop adf-template" || got[1] != "start adf-template branchable=true" {
		t.Fatalf("calls = %q", got)
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

func TestWarmTemplateMissingTellsUserToInit(t *testing.T) {
	err := testManager(fleettest.New()).WarmTemplate(ctx)
	if err == nil || !strings.Contains(err.Error(), "adf init") {
		t.Fatalf("err = %v, want a hint to run adf init", err)
	}
}

func TestWarmTemplateStartsDesktopAndWaits(t *testing.T) {
	rt := fleettest.New()
	rt.Machines[TemplateName] = &smolvm.Machine{Name: TemplateName, State: "stopped"}
	searches := 0
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
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
	got := rt.Calls()
	if got[0] != "start adf-template branchable=true" || got[1] != "exec adf-template desktop-start" {
		t.Fatalf("calls = %q", got)
	}
	if searches != 3 {
		t.Fatalf("searched %d times, want 3", searches)
	}
}

func TestWarmTemplateTimesOut(t *testing.T) {
	rt := fleettest.New()
	warmTemplate(rt)
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
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
	rt := fleettest.New()
	warmTemplate(rt)
	d, err := testManager(rt).CreateDesktop(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if d != (Desktop{Name: "a", Machine: "adf-a", State: "running"}) {
		t.Fatalf("desktop = %+v", d)
	}
	got := rt.Calls()
	if last := got[len(got)-1]; last != "branch adf-template->adf-a adf.role=desktop" {
		t.Fatalf("last call = %q", last)
	}
}

func TestCreateDesktopRejectsBadAndDuplicateNames(t *testing.T) {
	rt := fleettest.New()
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
	rt := fleettest.New()
	warmTemplate(rt)
	rt.Machines["adf-b"] = &smolvm.Machine{Name: "adf-b", State: "stopped", Labels: map[string]string{LabelRole: RoleDesktop}}
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{LabelRole: RoleDesktop}}
	rt.Machines["other"] = &smolvm.Machine{Name: "other", State: "running", Labels: map[string]string{LabelRole: RoleDesktop}}
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
	rt := fleettest.New()
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "running", Labels: map[string]string{LabelRole: RoleDesktop}}
	if err := testManager(rt).Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	want := []string{"stop adf-a", "delete adf-a cascade=false"}
	if got := rt.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

func TestDeleteUnknownDesktop(t *testing.T) {
	if err := testManager(fleettest.New()).Delete(ctx, "nope"); err == nil {
		t.Fatal("expected not found error")
	}
}

func TestComputerRunsInsideTheDesktop(t *testing.T) {
	rt := fleettest.New()
	if err := testManager(rt).Computer("a").Key(ctx, "Return"); err != nil {
		t.Fatal(err)
	}
	if got := rt.Calls(); got[0] != "exec adf-a env DISPLAY=:1 xdotool key -- Return" {
		t.Fatalf("call = %q", got[0])
	}
}

func TestConcurrentCreatesStartTemplateOnce(t *testing.T) {
	rt := fleettest.New()
	rt.Machines[TemplateName] = &smolvm.Machine{Name: TemplateName, State: "stopped",
		Labels: map[string]string{LabelRole: RoleTemplate}}
	rt.StartDelay = 20 * time.Millisecond
	m := testManager(rt)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := m.CreateDesktop(ctx, fmt.Sprintf("d%d", i)); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	starts := 0
	for _, c := range rt.Calls() {
		if strings.HasPrefix(c, "start adf-template") {
			starts++
		}
	}
	if starts != 1 {
		t.Fatalf("template started %d times, want 1", starts)
	}
}

func TestStatsPassesThroughWithValidation(t *testing.T) {
	rt := fleettest.New()
	rt.StatsFunc = func(name string) (smolvm.Stats, error) {
		if name != "adf-a" {
			t.Errorf("stats asked for %q", name)
		}
		return smolvm.Stats{CPUPercent: 5}, nil
	}
	m := testManager(rt)
	s, err := m.Stats(ctx, "a")
	if err != nil || s.CPUPercent != 5 {
		t.Fatalf("stats=%+v err=%v", s, err)
	}
	if _, err := m.Stats(ctx, "Bad"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("err = %v, want ErrInvalidName", err)
	}
}

func TestTemplateState(t *testing.T) {
	rt := fleettest.New()
	m := testManager(rt)
	if s, err := m.TemplateState(ctx); err != nil || s != "missing" {
		t.Fatalf("state=%q err=%v", s, err)
	}
	warmTemplate(rt)
	if s, _ := m.TemplateState(ctx); s != "running" {
		t.Fatalf("state = %q", s)
	}
}

func TestErrorsAreTyped(t *testing.T) {
	rt := fleettest.New()
	warmTemplate(rt)
	m := testManager(rt)
	if err := ValidateName("Bad"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("ValidateName: %v", err)
	}
	if err := m.Delete(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete: %v", err)
	}
	if _, err := m.CreateDesktop(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.CreateDesktop(ctx, "a"); !errors.Is(err, ErrExists) {
		t.Errorf("duplicate CreateDesktop: %v", err)
	}
}

func TestMemoryExcludesBalloon(t *testing.T) {
	rt := fleettest.New()
	rt.ExecFunc = func(name, _ string, argv []string) ([]byte, error) {
		if name != "adf-a" || strings.Join(argv, " ") != "cat /proc/meminfo" {
			t.Errorf("unexpected exec %s %q", name, argv)
		}
		return []byte("MemTotal:        2037920 kB\nMemFree:           12236 kB\nMemAvailable:      10240 kB\n" +
			"AnonPages:        299368 kB\nBalloon:         1637048 kB\n"), nil
	}
	mem, err := testManager(rt).Memory(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	wantUsed := int64(2037920-10240-1637048) * 1024
	if mem.TotalBytes != 2037920*1024 || mem.UsedBytes != wantUsed {
		t.Fatalf("mem = %+v, want used %d", mem, wantUsed)
	}
}

func TestMemoryRejectsGarbage(t *testing.T) {
	rt := fleettest.New()
	rt.ExecFunc = func(_, _ string, _ []string) ([]byte, error) { return []byte("nonsense"), nil }
	if _, err := testManager(rt).Memory(ctx, "a"); err == nil {
		t.Fatal("expected an error for output without MemTotal and MemAvailable")
	}
}

func TestStartBringsStoppedDesktopBack(t *testing.T) {
	rt := fleettest.New()
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "stopped", Labels: map[string]string{LabelRole: RoleDesktop}}
	if err := testManager(rt).Start(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"start adf-a branchable=false",
		"exec adf-a desktop-start",
		"exec adf-a env DISPLAY=:1 xdotool search --onlyvisible --class firefox",
	}
	if got := rt.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %q\nwant %q", got, want)
	}
}

func TestActionOnStoppedDesktopStartsItAndRetries(t *testing.T) {
	rt := fleettest.New()
	rt.Machines["adf-a"] = &smolvm.Machine{Name: "adf-a", State: "stopped", Labels: map[string]string{LabelRole: RoleDesktop}}
	if err := testManager(rt).Computer("a").Key(ctx, "Return"); err != nil {
		t.Fatalf("action on a stopped desktop should start it and succeed: %v", err)
	}
	got := rt.Calls()
	if got[0] != "exec adf-a env DISPLAY=:1 xdotool key -- Return" || got[1] != "start adf-a branchable=false" ||
		got[len(got)-1] != "exec adf-a env DISPLAY=:1 xdotool key -- Return" {
		t.Fatalf("calls = %q", got)
	}
}

func TestActionOnMissingDesktopIsNotRetried(t *testing.T) {
	rt := fleettest.New()
	rt.ExecFunc = func(_, _ string, _ []string) ([]byte, error) {
		return nil, &smolvm.Error{Code: "NOT_FOUND", Message: "vm not found: adf-zzz"}
	}
	if err := testManager(rt).Computer("zzz").Key(ctx, "Return"); !smolvm.IsNotFound(err) {
		t.Fatalf("err = %v, want not found", err)
	}
	if got := rt.Calls(); len(got) != 1 {
		t.Fatalf("calls = %q, want a single attempt", got)
	}
}

func TestProgressReportsTemplateSteps(t *testing.T) {
	rt := fleettest.New()
	rt.ExecFunc = func(_, _ string, argv []string) ([]byte, error) {
		if argv[0] == "cat" {
			return nil, errors.New("not provisioned")
		}
		return nil, nil
	}
	var msgs []string
	pctx := WithProgress(ctx, func(msg string) { msgs = append(msgs, msg) })
	if err := testManager(rt).EnsureTemplate(pctx); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(msgs, " | ")
	for _, want := range []string{"creating template", "installing desktop packages"} {
		if !strings.Contains(joined, want) {
			t.Errorf("progress %q is missing %q", joined, want)
		}
	}
}
