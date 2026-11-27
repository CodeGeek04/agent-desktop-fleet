package main

import (
	"testing"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
)

func TestLimitsFromEnv(t *testing.T) {
	t.Setenv("ADF_RESERVE_MB", "1024")
	t.Setenv("ADF_DESKTOP_MB", "768")
	t.Setenv("ADF_MAX_DESKTOPS", "12")
	l, err := limitsFromEnv(fleet.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if l.ReserveBytes != 1024<<20 || l.DesktopBytes != 768<<20 || l.MaxRunning != 12 {
		t.Fatalf("limits = %+v", l)
	}
	t.Setenv("ADF_MAX_DESKTOPS", "lots")
	if _, err := limitsFromEnv(fleet.DefaultLimits()); err == nil {
		t.Fatal("expected an error for a non-number")
	}
	t.Setenv("ADF_MAX_DESKTOPS", "")
	t.Setenv("ADF_DESKTOP_MB", "0")
	if _, err := limitsFromEnv(fleet.DefaultLimits()); err == nil {
		t.Fatal("expected an error for a zero desktop budget")
	}
}

func TestIdleFromEnv(t *testing.T) {
	if d, err := idleFromEnv(); err != nil || d != 10*time.Minute {
		t.Fatalf("default = %v, %v", d, err)
	}
	t.Setenv("ADF_IDLE_PAUSE", "0")
	if d, _ := idleFromEnv(); d != 0 {
		t.Fatalf("0 = %v", d)
	}
	t.Setenv("ADF_IDLE_PAUSE", "45s")
	if d, _ := idleFromEnv(); d != 45*time.Second {
		t.Fatalf("45s = %v", d)
	}
	t.Setenv("ADF_IDLE_PAUSE", "soon")
	if _, err := idleFromEnv(); err == nil {
		t.Fatal("expected an error")
	}
}
