//go:build !darwin && !linux

package host

import (
	"fmt"
	"runtime"
)

// Read is not implemented on this OS yet.
func Read() (Stats, error) {
	return Stats{}, fmt.Errorf("host stats are not supported on %s yet", runtime.GOOS)
}
