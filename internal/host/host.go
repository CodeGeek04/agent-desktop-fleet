// Package host reports this computer's memory and CPU, so the fleet can refuse work that
// would overload it and show people and agents how much room is left.
package host

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Stats is a snapshot of the host.
type Stats struct {
	TotalMemBytes     int64   `json:"total_mem_bytes"`
	AvailableMemBytes int64   `json:"available_mem_bytes"`
	CPUs              int     `json:"cpus"`
	Load1             float64 `json:"load1"`
}

// parseDarwin reads `sysctl -n hw.memsize kern.memorystatus_level vm.loadavg`.
// kern.memorystatus_level is the free-memory percentage macOS itself uses for memory
// pressure; it accounts for compressed and reclaimable memory.
func parseDarwin(out string, cpus int) (Stats, error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 3 {
		return Stats{}, errors.New("sysctl: unexpected output")
	}
	total, err := strconv.ParseInt(strings.TrimSpace(lines[0]), 10, 64)
	if err != nil {
		return Stats{}, fmt.Errorf("sysctl hw.memsize: %w", err)
	}
	level, err := strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64)
	if err != nil || level < 0 || level > 100 {
		return Stats{}, fmt.Errorf("sysctl kern.memorystatus_level: %q", lines[1])
	}
	load := strings.Fields(strings.Trim(strings.TrimSpace(lines[2]), "{}"))
	if len(load) == 0 {
		return Stats{}, errors.New("sysctl vm.loadavg: empty")
	}
	load1, err := strconv.ParseFloat(load[0], 64)
	if err != nil {
		return Stats{}, fmt.Errorf("sysctl vm.loadavg: %w", err)
	}
	return Stats{TotalMemBytes: total, AvailableMemBytes: total * level / 100, CPUs: cpus, Load1: load1}, nil
}

// parseLinux reads /proc/meminfo and /proc/loadavg.
func parseLinux(meminfo, loadavg string, cpus int) (Stats, error) {
	kib := map[string]int64{}
	for _, line := range strings.Split(meminfo, "\n") {
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if f := strings.Fields(rest); len(f) > 0 {
			if n, err := strconv.ParseInt(f[0], 10, 64); err == nil {
				kib[key] = n
			}
		}
	}
	total, okT := kib["MemTotal"]
	avail, okA := kib["MemAvailable"]
	if !okT || !okA {
		return Stats{}, errors.New("/proc/meminfo: missing MemTotal or MemAvailable")
	}
	f := strings.Fields(loadavg)
	if len(f) == 0 {
		return Stats{}, errors.New("/proc/loadavg: empty")
	}
	load1, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return Stats{}, fmt.Errorf("/proc/loadavg: %w", err)
	}
	return Stats{TotalMemBytes: total * 1024, AvailableMemBytes: avail * 1024, CPUs: cpus, Load1: load1}, nil
}
