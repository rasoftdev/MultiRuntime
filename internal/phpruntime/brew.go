package phpruntime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rasoftdev/multiruntime/internal/versions"
)

// brewProvider gives macOS access to legacy PHP (7.4) through Homebrew's
// shivammathur/php tap, which already builds it for Intel and Apple Silicon.
// mrt does not compile PHP on macOS; it installs the formula and registers the
// keg's php binary as a managed version, so .php-version and the shims work as
// on Linux. The formula is installed keg-only: nothing is linked into your PATH.
//
// NOT TESTED on a real Mac yet.
type brewProvider struct{}

func (brewProvider) Name() string { return "homebrew" }

// brewFormulas maps the exact version offered to its tap formula. 7.4.33 is
// the final 7.4 release, so it will not change.
var brewFormulas = map[string]string{
	"7.4.33": "shivammathur/php/php@7.4",
}

func brewPath() (string, bool) {
	if p, err := exec.LookPath("brew"); err == nil {
		return p, true
	}
	for _, p := range []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

func (brewProvider) List() ([]Release, error) {
	if runtime.GOOS != "darwin" {
		return nil, nil
	}
	// Offered even when Homebrew is missing, so that Install can explain how to get it.
	var out []Release
	for ver, formula := range brewFormulas {
		if v, ok := versions.Parse(ver); ok {
			out = append(out, Release{V: v, Provider: "homebrew", URL: "brew:" + formula})
		}
	}
	return out, nil
}

func brewRun(brew string, args ...string) error {
	fmt.Fprintf(os.Stderr, "  brew %s\n", strings.Join(args, " "))
	cmd := exec.Command(brew, args...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	return cmd.Run()
}

func (brewProvider) Install(r Release, prefix string) error {
	brew, ok := brewPath()
	if !ok {
		return fmt.Errorf("Homebrew is required to install PHP %s on macOS.\nInstall it from https://brew.sh and run this command again", r.V)
	}
	formula := strings.TrimPrefix(r.URL, "brew:")
	if err := brewRun(brew, "tap", "shivammathur/php"); err != nil {
		return fmt.Errorf("brew tap shivammathur/php: %w", err)
	}
	if err := brewRun(brew, "install", formula); err != nil {
		return fmt.Errorf("brew install %s: %w", formula, err)
	}
	out, err := exec.Command(brew, "--prefix", formula).Output()
	if err != nil {
		return fmt.Errorf("brew --prefix %s: %w", formula, err)
	}
	src := filepath.Join(strings.TrimSpace(string(out)), "bin", "php")
	if st, err := os.Stat(src); err != nil || st.IsDir() {
		return fmt.Errorf("Homebrew installed %s but %s does not exist", formula, src)
	}
	os.RemoveAll(prefix)
	if err := os.MkdirAll(filepath.Join(prefix, "bin"), 0o755); err != nil {
		return err
	}
	return os.Symlink(src, filepath.Join(prefix, "bin", "php"))
}
