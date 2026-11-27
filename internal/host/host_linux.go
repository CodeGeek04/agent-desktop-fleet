package host

import (
	"os"
	"runtime"
)

// Read returns the current host stats.
func Read() (Stats, error) {
	meminfo, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return Stats{}, err
	}
	loadavg, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return Stats{}, err
	}
	return parseLinux(string(meminfo), string(loadavg), runtime.NumCPU())
}
