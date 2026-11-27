package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/activity"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
)

// limitsFromEnv applies ADF_RESERVE_MB, ADF_DESKTOP_MB and ADF_MAX_DESKTOPS to l.
func limitsFromEnv(l fleet.Limits) (fleet.Limits, error) {
	for _, v := range []struct {
		env string
		set func(int64)
	}{
		{"ADF_RESERVE_MB", func(n int64) { l.ReserveBytes = n << 20 }},
		{"ADF_DESKTOP_MB", func(n int64) { l.DesktopBytes = n << 20 }},
		{"ADF_MAX_DESKTOPS", func(n int64) { l.MaxRunning = int(n) }},
	} {
		raw := os.Getenv(v.env)
		if raw == "" {
			continue
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			return l, fmt.Errorf("%s must be a non-negative whole number, got %q", v.env, raw)
		}
		v.set(n)
	}
	if l.DesktopBytes == 0 {
		return l, fmt.Errorf("ADF_DESKTOP_MB must be greater than 0")
	}
	return l, nil
}

// idleFromEnv returns how long a desktop may sit idle before it is paused (ADF_IDLE_PAUSE,
// a duration such as 10m; 0 turns pausing off).
func idleFromEnv() (time.Duration, error) {
	raw := os.Getenv("ADF_IDLE_PAUSE")
	if raw == "" {
		return 10 * time.Minute, nil
	}
	if raw == "0" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("ADF_IDLE_PAUSE must be a duration such as 10m or 0 to turn it off, got %q", raw)
	}
	return d, nil
}

// runIdlePauser pauses idle desktops once a minute until ctx is done. It needs the activity
// log to know when desktops were last used, so it does nothing without one.
func runIdlePauser(ctx context.Context, f *fleet.Manager, logPath string, idle time.Duration) {
	if idle <= 0 || logPath == "" {
		return
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			last, _ := activity.LastActive(logPath)
			_, _ = f.PauseIdle(ctx, last, idle, time.Now())
		}
	}
}

// humanDuration prints whole minutes as "10 min" and anything else as Go does ("45s").
func humanDuration(d time.Duration) string {
	if d >= time.Minute && d%time.Minute == 0 {
		return fmt.Sprintf("%d min", int(d/time.Minute))
	}
	return d.String()
}

// pauseUnsupported explains why pausing does not work on this host, or returns "".
// smolvm 1.37 on Linux cannot resume a paused branched desktop ("rewrite qcow2 backing:
// permission denied"), and a paused desktop cannot be stopped or started either, so pausing
// would strand it. ADF_PAUSE=1 forces pausing on once smolvm fixes this.
func pauseUnsupported() string {
	if os.Getenv("ADF_PAUSE") == "1" || runtime.GOOS != "linux" {
		return ""
	}
	return "smolvm cannot resume paused desktops on Linux yet"
}
