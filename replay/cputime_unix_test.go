//go:build unix

package replay

import "syscall"

// processCPUNanos is the process's user+system CPU time, which unlike wall
// time is not inflated by other load on the host.
func processCPUNanos() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return ru.Utime.Nano() + ru.Stime.Nano()
}
