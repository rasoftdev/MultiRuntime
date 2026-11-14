// Package shim implements the transparent `php` and `composer` entry points.
// Shims are symlinks to the mrt binary; behaviour is chosen from argv[0].
package shim

import (
	"fmt"
	"os"

	"github.com/rasoftdev/multiruntime/internal/phpruntime"
)

// Names lists the commands that get a shim.
var Names = []string{"php", "composer"}

// IsShim reports whether name is one of the shim commands.
func IsShim(name string) bool {
	for _, n := range Names {
		if n == name {
			return true
		}
	}
	return false
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "mrt: "+format+"\n", a...)
	os.Exit(127)
}

// Run executes the shimmed command. It only returns by exiting.
func Run(name string, args []string) {
	cwd, err := os.Getwd()
	if err != nil {
		fail("%v", err)
	}
	res, err := phpruntime.Resolve(cwd)
	if err != nil {
		fail("%v", err)
	}
	env := append(os.Environ(), res.Env()...)

	switch name {
	case "php":
		bin := res.Bin
		if res.Spec == phpruntime.System {
			if bin, err = phpruntime.FindOnPath("php"); err != nil {
				fail("%v", err)
			}
		}
		fail("%v", execve(bin, append([]string{bin}, args...), env))
	case "composer":
		if res.Spec == phpruntime.System {
			bin, err := phpruntime.FindOnPath("composer")
			if err != nil {
				fail("%v (install composer or pin a PHP version with `mrt php local`)", err)
			}
			fail("%v", execve(bin, append([]string{bin}, args...), env))
		}
		phar, err := phpruntime.EnsureComposer(phpruntime.ComposerChannel(res.Version))
		if err != nil {
			fail("%v", err)
		}
		argv := append([]string{res.Bin, phar}, args...)
		fail("%v", execve(res.Bin, argv, env))
	}
}
