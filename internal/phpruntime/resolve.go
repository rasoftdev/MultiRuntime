package phpruntime

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rasoftdev/multiruntime/internal/config"
	"github.com/rasoftdev/multiruntime/internal/versions"
)

// System means "use the PHP found on PATH outside the shims".
const System = "system"

const VersionFile = ".php-version"

// Resolution says which PHP to run and why.
type Resolution struct {
	Spec    string // requested spec or "system"
	Source  string // human description of where Spec came from
	Version string // exact installed version; empty when System
	Bin     string // php binary path; empty when System
}

// readSpec reads the first meaningful line of a version file.
func readSpec(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l != "" && !strings.HasPrefix(l, "#") {
			return l, nil
		}
	}
	return "", nil
}

// FindLocal walks up from dir looking for .php-version.
func FindLocal(dir string) (path, spec string) {
	for {
		p := filepath.Join(dir, VersionFile)
		if s, err := readSpec(p); err == nil && s != "" {
			return p, s
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ""
		}
		dir = parent
	}
}

// Resolve picks the PHP for the directory: MRT_PHP_VERSION, .php-version, global, system.
func Resolve(cwd string) (Resolution, error) {
	spec, source := "", ""
	if s := os.Getenv("MRT_PHP_VERSION"); s != "" {
		spec, source = s, "MRT_PHP_VERSION"
	} else if p, s := FindLocal(cwd); s != "" {
		spec, source = s, p
	} else if s, _ := readSpec(config.PHPGlobalFile()); s != "" {
		spec, source = s, config.PHPGlobalFile()
	}
	if spec == "" || spec == System {
		if spec == "" {
			source = "default"
		}
		return Resolution{Spec: System, Source: source}, nil
	}
	if !versions.ValidSpec(spec) {
		return Resolution{}, fmt.Errorf("invalid version %q in %s", spec, source)
	}
	v, ok := versions.Latest(spec, Installed())
	if !ok {
		return Resolution{}, fmt.Errorf("PHP %s is required by %s but not installed.\nRun: mrt php install %s", spec, source, spec)
	}
	return Resolution{Spec: spec, Source: source, Version: v.String(), Bin: BinPath(v.String())}, nil
}

// Env returns environment overrides so the managed PHP uses its own ini.
func (r Resolution) Env() []string {
	if r.Version == "" {
		return nil
	}
	etc := filepath.Join(config.PHPVersionDir(r.Version), "etc")
	return []string{
		"PHPRC=" + etc,
		"PHP_INI_SCAN_DIR=" + filepath.Join(etc, "conf.d"),
		"MRT_PHP_ACTIVE=" + r.Version,
	}
}

// FindOnPath finds name on PATH, skipping the shims directory.
func FindOnPath(name string) (string, error) {
	shims := filepath.Clean(config.ShimsDir())
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if d == "" || filepath.Clean(d) == shims {
			continue
		}
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s not found on PATH (outside mrt shims)", name)
}

// WriteVersionFile writes .php-version into dir.
func WriteVersionFile(dir, spec string) error {
	return os.WriteFile(filepath.Join(dir, VersionFile), []byte(spec+"\n"), 0o644)
}

// SetGlobal stores the global default.
func SetGlobal(spec string) error {
	if err := os.MkdirAll(config.PHPDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(config.PHPGlobalFile(), []byte(spec+"\n"), 0o644)
}
