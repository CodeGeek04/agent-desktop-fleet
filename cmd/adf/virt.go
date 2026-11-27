package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// checkVirtualization reports whether this host can run hardware-accelerated VMs.
func checkVirtualization() error {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "kern.hv_support").Output()
		if err != nil {
			return fmt.Errorf("sysctl kern.hv_support: %w", err)
		}
		if strings.TrimSpace(string(out)) != "1" {
			return errors.New("this Mac does not support Hypervisor.framework")
		}
		return nil
	case "linux":
		f, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0)
		if err != nil {
			return fmt.Errorf("cannot open /dev/kvm (%v): enable KVM or nested virtualization and add your user to the kvm group", err)
		}
		return f.Close()
	default:
		return fmt.Errorf("%s hosts are not supported yet", runtime.GOOS)
	}
}
