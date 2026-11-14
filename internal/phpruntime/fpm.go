package phpruntime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rasoftdev/multiruntime/internal/config"
)

// FPM manages the php-fpm master of one installed PHP version. Each version
// gets its own config, pid file, log and unix socket under its prefix, so
// several versions can run side by side and nginx/apache can point each site
// at the socket of the version it needs.

// FPMBin is the php-fpm executable of an exact installed version.
func FPMBin(v string) string { return filepath.Join(config.PHPVersionDir(v), "sbin", "php-fpm") }

func fpmConf(v string) string { return filepath.Join(config.PHPVersionDir(v), "etc", "php-fpm.conf") }
func fpmPool(v string) string {
	return filepath.Join(config.PHPVersionDir(v), "etc", "php-fpm.d", "www.conf")
}
func fpmRun(v string) string { return filepath.Join(config.PHPVersionDir(v), "var", "run") }
func fpmPID(v string) string { return filepath.Join(fpmRun(v), "php-fpm.pid") }

// FPMLog is the master log of a version.
func FPMLog(v string) string {
	return filepath.Join(config.PHPVersionDir(v), "var", "log", "php-fpm.log")
}

// FPMSocket is the default unix socket of a version.
func FPMSocket(v string) string { return filepath.Join(fpmRun(v), "php-fpm.sock") }

// FPMListen returns the listen address configured in the pool (socket path or
// host:port), falling back to the default socket.
func FPMListen(v string) string {
	b, err := os.ReadFile(fpmPool(v))
	if err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "listen") && !strings.HasPrefix(l, "listen.") {
				if _, val, ok := strings.Cut(l, "="); ok {
					return strings.TrimSpace(val)
				}
			}
		}
	}
	return FPMSocket(v)
}

func checkFPM(v string) error {
	if _, err := os.Stat(FPMBin(v)); err != nil {
		return fmt.Errorf("PHP %s has no php-fpm (%s). Only versions built from source include it: mrt php install %s --profile source", v, FPMBin(v), v)
	}
	return nil
}

// WriteFPMConfig writes the master and pool configuration. An empty listen
// means the default unix socket; otherwise it is a socket path or host:port.
func WriteFPMConfig(v, listen string) error {
	if err := checkFPM(v); err != nil {
		return err
	}
	if listen == "" {
		listen = FPMSocket(v)
	}
	for _, d := range []string{filepath.Dir(fpmPool(v)), fpmRun(v), filepath.Dir(FPMLog(v))} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	master := "; Managed by mrt. Edit freely.\n[global]\n" +
		"pid = " + fpmPID(v) + "\n" +
		"error_log = " + FPMLog(v) + "\n" +
		"daemonize = yes\n" +
		"include = " + filepath.Join(filepath.Dir(fpmPool(v)), "*.conf") + "\n"
	if err := os.WriteFile(fpmConf(v), []byte(master), 0o644); err != nil {
		return err
	}
	pool := "; Managed by mrt. Edit freely.\n[www]\n"
	if os.Geteuid() == 0 {
		pool += "user = nobody\ngroup = nobody\n"
	}
	pool += "listen = " + listen + "\n"
	if !strings.Contains(listen, ":") {
		// Unix socket: the web server user must be able to open it. Run the
		// web server in the same group or widen this mode.
		pool += "listen.mode = 0660\n"
	}
	pool += "pm = dynamic\npm.max_children = 10\npm.start_servers = 2\n" +
		"pm.min_spare_servers = 1\npm.max_spare_servers = 3\n" +
		"catch_workers_output = yes\nclear_env = no\n"
	return os.WriteFile(fpmPool(v), []byte(pool), 0o644)
}

// FPMPid returns the master pid when it is running, else 0.
func FPMPid(v string) int {
	b, err := os.ReadFile(fpmPID(v))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 || !processAlive(pid) {
		return 0
	}
	return pid
}

// FPMStart starts the master (daemonized). A non-empty listen rewrites the pool.
func FPMStart(v, listen string) error {
	if err := checkFPM(v); err != nil {
		return err
	}
	if pid := FPMPid(v); pid != 0 {
		return fmt.Errorf("php-fpm %s already running (pid %d)", v, pid)
	}
	if _, err := os.Stat(fpmPool(v)); err != nil || listen != "" {
		if err := WriteFPMConfig(v, listen); err != nil {
			return err
		}
	}
	os.Remove(fpmPID(v)) // stale
	etc := filepath.Join(config.PHPVersionDir(v), "etc")
	cmd := exec.Command(FPMBin(v), "--fpm-config", fpmConf(v), "--prefix", config.PHPVersionDir(v),
		"-c", filepath.Join(etc, "php.ini"))
	cmd.Env = append(os.Environ(), "PHP_INI_SCAN_DIR="+filepath.Join(etc, "conf.d"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("php-fpm failed to start: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	for i := 0; i < 20; i++ {
		if FPMPid(v) != 0 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("php-fpm did not report a pid; see %s", FPMLog(v))
}

// FPMStop stops the master gracefully.
func FPMStop(v string) error {
	pid := FPMPid(v)
	if pid == 0 {
		return fmt.Errorf("php-fpm %s is not running", v)
	}
	if err := signalFPM(pid, false); err != nil {
		return err
	}
	for i := 0; i < 100; i++ {
		if !processAlive(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("php-fpm %s (pid %d) did not stop in 10s", v, pid)
}

// FPMReload re-reads config and php.ini without dropping connections.
func FPMReload(v string) error {
	pid := FPMPid(v)
	if pid == 0 {
		return fmt.Errorf("php-fpm %s is not running", v)
	}
	return signalFPM(pid, true)
}

// FPMTest validates the generated configuration.
func FPMTest(v string) error {
	if err := checkFPM(v); err != nil {
		return err
	}
	if _, err := os.Stat(fpmConf(v)); err != nil {
		if err := WriteFPMConfig(v, ""); err != nil {
			return err
		}
	}
	etc := filepath.Join(config.PHPVersionDir(v), "etc")
	cmd := exec.Command(FPMBin(v), "-t", "--fpm-config", fpmConf(v), "--prefix", config.PHPVersionDir(v),
		"-c", filepath.Join(etc, "php.ini"))
	cmd.Env = append(os.Environ(), "PHP_INI_SCAN_DIR="+filepath.Join(etc, "conf.d"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
