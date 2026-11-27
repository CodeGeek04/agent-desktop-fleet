package host

import (
	"os/exec"
	"runtime"
)

// Read returns the current host stats.
func Read() (Stats, error) {
	out, err := exec.Command("sysctl", "-n", "hw.memsize", "kern.memorystatus_level", "vm.loadavg").Output()
	if err != nil {
		return Stats{}, err
	}
	return parseDarwin(string(out), runtime.NumCPU())
}
