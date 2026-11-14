//go:build windows

package shim

import (
	"errors"
	"os"
	"os/exec"
)

func execve(path string, argv, env []string) error {
	c := exec.Command(path, argv[1:]...)
	c.Env, c.Stdin, c.Stdout, c.Stderr = env, os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}
