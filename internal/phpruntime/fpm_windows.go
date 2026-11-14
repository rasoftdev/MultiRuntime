//go:build windows

package phpruntime

import "fmt"

func processAlive(pid int) bool { return false }

func signalFPM(pid int, reload bool) error {
	return fmt.Errorf("php-fpm management is not supported on Windows")
}
