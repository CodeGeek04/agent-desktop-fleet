//go:build integration

// Package integration runs the fleet against a real smolvm install.
// Run: go test -tags integration -timeout 20m ./tests/integration/ -v
package integration

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

func smolvmBin(t *testing.T) string {
	if p, err := exec.LookPath("smolvm"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".smolvm", "smolvm")
	if _, err := os.Stat(p); err != nil {
		t.Skip("smolvm not installed")
	}
	return p
}

func TestFleetEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	m := fleet.NewManager(smolvm.New(smolvmBin(t), runner.Exec{}), os.Stderr)

	if err := m.EnsureTemplate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.WarmTemplate(ctx); err != nil {
		t.Fatal(err)
	}

	names := []string{"it-a", "it-b"}
	for _, n := range names {
		_ = m.Delete(ctx, n) // leftovers from an earlier failed run
		start := time.Now()
		if _, err := m.CreateDesktop(ctx, n); err != nil {
			t.Fatal(err)
		}
		t.Logf("created %s in %s", n, time.Since(start))
		t.Cleanup(func() { _ = m.Delete(context.Background(), n) })
	}

	ds, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, d := range ds {
		found[d.Name] = d.State == "running"
	}
	for _, n := range names {
		if !found[n] {
			t.Fatalf("desktop %s missing or not running in %+v", n, ds)
		}
	}

	a, b := m.Computer("it-a"), m.Computer("it-b")
	if err := a.Key(ctx, "ctrl+l"); err != nil {
		t.Fatal(err)
	}
	if err := a.Type(ctx, "about:robots"); err != nil {
		t.Fatal(err)
	}
	if err := a.Key(ctx, "Return"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	shotA, err := a.Screenshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	shotB, err := b.Screenshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(shotA) < 1000 || len(shotB) < 1000 {
		t.Fatalf("screenshots too small: %d and %d bytes", len(shotA), len(shotB))
	}
	if bytes.Equal(shotA, shotB) {
		t.Fatal("typing into it-a should make its screen differ from it-b")
	}
	out := filepath.Join(t.TempDir(), "it-a.png")
	if err := os.WriteFile(out, shotA, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("screenshot of it-a written to %s", out)
}
