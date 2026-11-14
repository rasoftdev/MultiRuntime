// Command mrt manages multiple language runtimes side by side (PHP first).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rasoftdev/multiruntime/internal/config"
	"github.com/rasoftdev/multiruntime/internal/phpruntime"
	"github.com/rasoftdev/multiruntime/internal/shim"
	"github.com/rasoftdev/multiruntime/internal/versions"
)

const version = "0.1.0"

func main() {
	// Busybox style: when invoked through a shim symlink, behave as that tool.
	if name := filepath.Base(os.Args[0]); shim.IsShim(name) {
		shim.Run(name, os.Args[1:])
		return
	}
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	var err error
	switch args[0] {
	case "php":
		err = phpCmd(args[1:])
	case "setup":
		err = setup()
	case "env":
		fmt.Printf("export PATH=\"%s:%s:$PATH\"\n", filepath.Join(config.Home(), "bin"), config.ShimsDir())
	case "doctor":
		err = doctor()
	case "version", "--version", "-v":
		fmt.Println("mrt", version)
	case "help", "--help", "-h":
		usage()
	default:
		err = fmt.Errorf("unknown command %q", args[0])
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "mrt:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `mrt - multi runtime manager

  mrt setup                    install shims (php, composer) under ~/.mrt
  mrt env                      print the PATH export line for your shell rc
  mrt doctor                   check the installation

  mrt php ls-remote [spec]     list installable versions
  mrt php install <spec>       install (e.g. 8.3 or 8.3.12) [--profile bulk|common|minimal]
  mrt php list                 list installed versions
  mrt php local <spec>         write .php-version in the current directory
  mrt php global <spec|system> set the default version
  mrt php current              show the version in effect here and why
  mrt php which                path of the php binary in effect
  mrt php uninstall <x.y.z>    remove an installed version
  mrt php outdated             installed lines that have a newer patch release
  mrt php update [spec]        install the newest patch of each installed line [--prune]
  mrt php ext add <name>[@ver] build + enable a PECL or bundled extension (e.g. redis, xdebug, gmp)
  mrt php ext list|remove      manage extensions of the active version [--php x.y.z]

  install flags: --profile source (force compile) | --prebuilt (accept an older prebuilt patch)
`)
}

// extCmd handles `mrt php ext ...` for the PHP in effect (or --php).
func extCmd(args []string, pin, cwd string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mrt php ext add|list|remove <name>")
	}
	ver := pin
	if ver == "" {
		r, err := phpruntime.Resolve(cwd)
		if err != nil {
			return err
		}
		if r.Version == "" {
			return fmt.Errorf("no managed PHP in effect here: pin one with `mrt php local <version>` or pass --php")
		}
		ver = r.Version
	} else if v, ok := versions.Latest(pin, phpruntime.Installed()); ok {
		ver = v.String()
	} else {
		return fmt.Errorf("PHP %s is not installed", pin)
	}
	switch args[0] {
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("usage: mrt php ext add <name>[@version]")
		}
		if err := phpruntime.ExtAdd(ver, args[1]); err != nil {
			return err
		}
		fmt.Printf("%s enabled for PHP %s\n", args[1], ver)
	case "list":
		inis, err := phpruntime.ExtList(ver)
		if err != nil {
			return err
		}
		for _, f := range inis {
			fmt.Println(f)
		}
	case "remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: mrt php ext remove <name>")
		}
		return phpruntime.ExtRemove(ver, args[1])
	default:
		return fmt.Errorf("unknown ext subcommand %q", args[0])
	}
	return nil
}

func setup() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	binDir := filepath.Join(config.Home(), "bin")
	for _, d := range []string{binDir, config.ShimsDir(), config.CacheDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	dst := filepath.Join(binDir, "mrt")
	if a, _ := filepath.EvalSymlinks(self); a != dst {
		if err := copyFile(self, dst); err != nil {
			return err
		}
	}
	for _, n := range shim.Names {
		link := filepath.Join(config.ShimsDir(), n)
		os.Remove(link)
		if err := os.Symlink(dst, link); err != nil {
			return err
		}
	}
	fmt.Println("Installed to", config.Home())
	fmt.Println("Add this to ~/.bashrc or ~/.zshrc, then restart your shell:")
	fmt.Println(`  eval "$(mrt env)"`)
	fmt.Println("Shims go before your system PHP; with no .php-version your system PHP keeps being used.")
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func doctor() error {
	ok := true
	check := func(label string, good bool, detail string) {
		mark := "ok  "
		if !good {
			mark, ok = "FAIL", false
		}
		fmt.Printf("[%s] %s %s\n", mark, label, detail)
	}
	for _, n := range shim.Names {
		_, err := os.Stat(filepath.Join(config.ShimsDir(), n))
		check("shim "+n, err == nil, "")
	}
	check("shims in PATH", strings.Contains(os.Getenv("PATH"), config.ShimsDir()), config.ShimsDir())
	if sys, err := phpruntime.FindOnPath("php"); err == nil {
		check("system php", true, sys)
	} else {
		fmt.Println("[info] no system php on PATH (fine if you only use managed versions)")
	}
	cwd, _ := os.Getwd()
	if r, err := phpruntime.Resolve(cwd); err != nil {
		check("resolution here", false, err.Error())
	} else {
		check("resolution here", true, describe(r))
	}
	if !ok {
		return fmt.Errorf("problems found")
	}
	return nil
}

func describe(r phpruntime.Resolution) string {
	if r.Spec == phpruntime.System {
		return fmt.Sprintf("system PHP (%s)", r.Source)
	}
	return fmt.Sprintf("%s -> %s (from %s)", r.Spec, r.Version, r.Source)
}

// extractFlag removes "--name value" or "--name=value" from args.
func extractFlag(args []string, name string) ([]string, string) {
	var rest []string
	val := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == name && i+1 < len(args):
			val = args[i+1]
			i++
		case strings.HasPrefix(args[i], name+"="):
			val = strings.TrimPrefix(args[i], name+"=")
		default:
			rest = append(rest, args[i])
		}
	}
	return rest, val
}

// extractBool removes a boolean --name flag from args.
func extractBool(args []string, name string) ([]string, bool) {
	var rest []string
	found := false
	for _, a := range args {
		if a == name {
			found = true
		} else {
			rest = append(rest, a)
		}
	}
	return rest, found
}

func phpCmd(args []string) error {
	args, profile := extractFlag(args, "--profile")
	args, pinPHP := extractFlag(args, "--php")
	args, prebuilt := extractBool(args, "--prebuilt")
	args, prune := extractBool(args, "--prune")
	if prebuilt {
		profile = "prebuilt"
	}
	if profile != "" {
		switch profile {
		case "bulk", "common", "minimal", "source", "prebuilt":
			phpruntime.SetProfile(profile)
		default:
			return fmt.Errorf("unknown profile %q (bulk, common, minimal, source)", profile)
		}
	}
	if len(args) == 0 {
		usage()
		return fmt.Errorf("missing php subcommand")
	}
	sub, rest := args[0], args[1:]
	need := func() (string, error) {
		if len(rest) != 1 {
			return "", fmt.Errorf("usage: mrt php %s <version>", sub)
		}
		return rest[0], nil
	}
	cwd, _ := os.Getwd()

	switch sub {
	case "ls-remote":
		rels, err := phpruntime.Remote()
		if err != nil {
			return err
		}
		best := map[string]versions.V{}
		for _, r := range rels {
			if len(rest) > 0 && !versions.Match(rest[0], r.V) {
				continue
			}
			k := fmt.Sprintf("%d.%d", r.V.Major, r.V.Minor)
			if len(rest) > 0 {
				k = r.V.String()
			}
			if cur, ok := best[k]; !ok || r.V.Cmp(cur) > 0 {
				best[k] = r.V
			}
		}
		var list []versions.V
		for _, v := range best {
			list = append(list, v)
		}
		versions.Sort(list)
		for _, v := range list {
			fmt.Println(v)
		}
	case "install":
		spec, err := need()
		if err != nil {
			return err
		}
		v, err := phpruntime.Install(spec)
		if err != nil {
			return err
		}
		fmt.Printf("PHP %s ready\n", v)
	case "list":
		in := phpruntime.Installed()
		if len(in) == 0 {
			fmt.Println("(none installed; try: mrt php install 8.3)")
		}
		r, _ := phpruntime.Resolve(cwd)
		for _, v := range in {
			mark := "  "
			if r.Version == v.String() {
				mark = "* "
			}
			fmt.Println(mark + v.String())
		}
	case "local":
		spec, err := need()
		if err != nil {
			return err
		}
		if !versions.ValidSpec(spec) {
			return fmt.Errorf("invalid version %q", spec)
		}
		if err := phpruntime.WriteVersionFile(cwd, spec); err != nil {
			return err
		}
		if _, ok := versions.Latest(spec, phpruntime.Installed()); !ok {
			fmt.Printf("wrote .php-version = %s (not installed yet: mrt php install %s)\n", spec, spec)
		} else {
			fmt.Printf("wrote .php-version = %s\n", spec)
		}
	case "global":
		spec, err := need()
		if err != nil {
			return err
		}
		if spec != phpruntime.System && !versions.ValidSpec(spec) {
			return fmt.Errorf("invalid version %q", spec)
		}
		return phpruntime.SetGlobal(spec)
	case "current":
		r, err := phpruntime.Resolve(cwd)
		if err != nil {
			return err
		}
		fmt.Println(describe(r))
	case "which":
		r, err := phpruntime.Resolve(cwd)
		if err != nil {
			return err
		}
		if r.Bin == "" {
			p, err := phpruntime.FindOnPath("php")
			if err != nil {
				return err
			}
			r.Bin = p
		}
		fmt.Println(r.Bin)
	case "uninstall":
		spec, err := need()
		if err != nil {
			return err
		}
		return phpruntime.Uninstall(spec)
	case "outdated":
		ups, err := phpruntime.Outdated()
		if err != nil {
			return err
		}
		if len(ups) == 0 {
			fmt.Println("Everything is up to date.")
		}
		for _, u := range ups {
			fmt.Printf("%s -> %s\n", u.Have, u.Latest)
		}
	case "update":
		spec := ""
		if len(rest) > 0 {
			spec = rest[0]
		}
		return phpruntime.Update(spec, prune)
	case "ext":
		return extCmd(rest, pinPHP, cwd)
	default:
		return fmt.Errorf("unknown php subcommand %q", sub)
	}
	return nil
}
