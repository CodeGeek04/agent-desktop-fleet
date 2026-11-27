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
	"github.com/CodeGeek04/agent-desktop-fleet/internal/host"
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

func desktopMachine(name, state string, created time.Time) *smolvm.Machine {
	return &smolvm.Machine{Name: DesktopPrefix + name, State: state, CreatedAt: created.Unix(),
		Labels: map[string]string{LabelRole: RoleDesktop}}
}

func TestCreateRefusedWhenMemoryIsLow(t *testing.T) {
	rt := fleettest.New()
	warmTemplate(rt)
	m := testManager(rt)
	m.Host = func() (host.Stats, error) {
		return host.Stats{TotalMemBytes: 8 << 30, AvailableMemBytes: 2200 << 20}, nil
	}
	_, err := m.CreateDesktop(ctx, "a")
	if !errors.Is(err, ErrNoCapacity) || !strings.Contains(err.Error(), "delete") {
		t.Fatalf("err = %v, want ErrNoCapacity with advice", err)
	}
	for _, c := range rt.Calls() {
		if strings.HasPrefix(c, "branch") {
			t.Fatal("a refused desktop must not be branched")
		}
	}
}

func TestCreateRefusedAtMaxRunning(t *testing.T) {
	rt := fleettest.New()
	warmTemplate(rt)
	rt.Machines["adf-a"] = desktopMachine("a", "running", time.Now())
	m := testManager(rt)
	m.Limits.MaxRunning = 1
	if _, err := m.CreateDesktop(ctx, "b"); !errors.Is(err, ErrNoCapacity) {
		t.Fatalf("err = %v, want ErrNoCapacity", err)
	}
}

func TestCapacity(t *testing.T) {
	rt := fleettest.New()
	warmTemplate(rt)
	rt.Machines["adf-a"] = desktopMachine("a", "running", time.Now())
	rt.Machines["adf-b"] = desktopMachine("b", "running", time.Now())
	rt.Machines["adf-c"] = desktopMachine("c", "paused", time.Now())
	rt.Machines["adf-d"] = desktopMachine("d", "stopped", time.Now())
	m := testManager(rt)
	m.Host = func() (host.Stats, error) {
		return host.Stats{TotalMemBytes: 16 << 30, AvailableMemBytes: 5 << 30, CPUs: 8}, nil
	}
	c, err := m.Capacity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// (5 GiB available - 2 GiB reserve) / 512 MiB per desktop = 6
	if c.Running != 2 || c.Paused != 1 || c.Stopped != 1 || c.MoreThatFit != 6 || !c.HostKnown || c.Host.CPUs != 8 {
		t.Fatalf("capacity = %+v", c)
	}
	m.Limits.MaxRunning = 3
	if c, _ := m.Capacity(ctx); c.MoreThatFit != 1 {
		t.Fatalf("with a cap of 3 and 2 running, more that fit = %d, want 1", c.MoreThatFit)
	}
}

func TestPausedDesktopWakesOnAction(t *testing.T) {
	rt := fleettest.New()
	rt.Machines["adf-a"] = desktopMachine("a", "running", time.Now())
	m := testManager(rt)
	if err := m.Pause(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if rt.Machines["adf-a"].State != "paused" {
		t.Fatal("desktop was not paused")
	}
	if err := m.Computer("a").Key(ctx, "Return"); err != nil {
		t.Fatalf("action on a paused desktop: %v", err)
	}
	got := rt.Calls()
	if got[0] != "pause adf-a" || got[2] != "resume adf-a" || got[len(got)-1] != "exec adf-a env DISPLAY=:1 xdotool key -- Return" {
		t.Fatalf("calls = %q", got)
	}
	for _, c := range got {
		if c == "exec adf-a desktop-start" {
			t.Fatal("resuming keeps memory, so the session must not be restarted")
		}
	}
}

func TestPauseIdle(t *testing.T) {
	now := time.Now()
	rt := fleettest.New()
	warmTemplate(rt)
	rt.Machines["adf-old"] = desktopMachine("old", "running", now.Add(-time.Hour))
	rt.Machines["adf-busy"] = desktopMachine("busy", "running", now.Add(-time.Hour))
	rt.Machines["adf-new"] = desktopMachine("new", "running", now.Add(-time.Minute))
	rt.Machines["adf-zzz"] = desktopMachine("zzz", "paused", now.Add(-time.Hour))
	lastActive := map[string]time.Time{"busy": now.Add(-2 * time.Minute), "old": now.Add(-30 * time.Minute)}
	paused, err := testManager(rt).PauseIdle(ctx, lastActive, 10*time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(paused, []string{"old"}) {
		t.Fatalf("paused = %q, want only old", paused)
	}
}

func TestSleepStopsWhenPauseIsUnsupported(t *testing.T) {
	rt := fleettest.New()
	rt.Machines["adf-a"] = desktopMachine("a", "running", time.Now())
	m := testManager(rt)
	m.PauseUnsupported = "not on this host"
	how, err := m.Sleep(ctx, "a")
	if err != nil || how != "stopped" {
		t.Fatalf("how=%q err=%v, want stopped", how, err)
	}
	if got := rt.Calls(); len(got) != 1 || got[0] != "stop adf-a" {
		t.Fatalf("calls = %q", got)
	}
	m.PauseUnsupported = ""
	rt.Machines["adf-b"] = desktopMachine("b", "running", time.Now())
	if how, err := m.Sleep(ctx, "b"); err != nil || how != "paused" {
		t.Fatalf("supported: how=%q err=%v", how, err)
	}
}

func TestPauseIdleDoesNothingWhenPauseIsUnsupported(t *testing.T) {
	now := time.Now()
	rt := fleettest.New()
	rt.Machines["adf-old"] = desktopMachine("old", "running", now.Add(-time.Hour))
	m := testManager(rt)
	m.PauseUnsupported = "not on this host"
	paused, err := m.PauseIdle(ctx, nil, 10*time.Minute, now)
	if err != nil || len(paused) != 0 || len(rt.Calls()) != 0 {
		t.Fatalf("paused=%q err=%v calls=%q", paused, err, rt.Calls())
	}
}
