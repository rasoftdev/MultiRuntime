//go:build !windows

package phpruntime

import (
	"os"
	"syscall"
)

func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	return err == nil && p.Signal(syscall.Signal(0)) == nil
}

// signalFPM sends SIGUSR2 (reload) or SIGQUIT (graceful stop) to the master.
func signalFPM(pid int, reload bool) error {
	sig := syscall.SIGQUIT
	if reload {
		sig = syscall.SIGUSR2
	}
	return syscall.Kill(pid, sig)
}
