package phpruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rasoftdev/multiruntime/internal/config"
	"github.com/rasoftdev/multiruntime/internal/versions"
)

func defaultINI() string {
	ini := "; Managed by mrt. Edit freely; extra settings can go in conf.d/*.ini\n" +
		"memory_limit = 1G\ndate.timezone = UTC\ndisplay_errors = On\nerror_reporting = E_ALL\n"
	for _, ca := range []string{
		"/etc/ssl/certs/ca-certificates.crt", "/etc/pki/tls/certs/ca-bundle.crt",
		"/etc/ssl/cert.pem", "/etc/ssl/ca-bundle.pem",
	} {
		if _, err := os.Stat(ca); err == nil {
			ini += "openssl.cafile = " + ca + "\ncurl.cainfo = " + ca + "\n"
			break
		}
	}
	return ini
}

type meta struct {
	Version  string `json:"version"`
	Provider string `json:"provider"`
	URL      string `json:"url"`
}

// Installed returns installed versions, ascending.
func Installed() []versions.V {
	ents, _ := os.ReadDir(config.PHPVersions())
	var out []versions.V
	for _, e := range ents {
		if v, ok := versions.Parse(e.Name()); ok && e.IsDir() {
			if _, err := os.Stat(filepath.Join(config.PHPVersionDir(e.Name()), "meta.json")); err == nil {
				out = append(out, v)
			}
		}
	}
	versions.Sort(out)
	return out
}

// BinPath is the php executable of an exact installed version.
func BinPath(v string) string { return filepath.Join(config.PHPVersionDir(v), "bin", "php") }

// Install resolves spec to the newest matching release and installs it.
func Install(spec string) (versions.V, error) {
	if !versions.ValidSpec(spec) {
		return versions.V{}, fmt.Errorf("invalid version %q (use 8, 8.3 or 8.3.12)", spec)
	}
	rels, err := Remote()
	if err != nil {
		return versions.V{}, fmt.Errorf("cannot list remote releases: %w", err)
	}
	var best *Release
	for i := range rels {
		r := &rels[i]
		if versions.Match(spec, r.V) && (best == nil || r.V.Cmp(best.V) > 0) {
			best = r
		}
	}
	if best == nil {
		return versions.V{}, fmt.Errorf("no installable PHP %s found.\n%s", spec, legacyHint(spec))
	}
	vs := best.V.String()
	final := config.PHPVersionDir(vs)
	if _, err := os.Stat(filepath.Join(final, "meta.json")); err == nil {
		return best.V, nil
	}
	fmt.Fprintf(os.Stderr, "Installing PHP %s (%s)\n", vs, best.Provider)
	// Providers build straight into the final prefix because compiled paths are
	// baked in. A directory without meta.json is an unfinished install (source
	// builds resume from it).
	if err := providerByName(best.Provider).Install(*best, final); err != nil {
		return versions.V{}, err
	}
	if err := os.MkdirAll(filepath.Join(final, "etc", "conf.d"), 0o755); err != nil {
		return versions.V{}, err
	}
	if err := os.WriteFile(filepath.Join(final, "etc", "php.ini"), []byte(defaultINI()), 0o644); err != nil {
		return versions.V{}, err
	}
	if err := enableOpcache(final); err != nil {
		return versions.V{}, err
	}
	mb, _ := json.MarshalIndent(meta{vs, best.Provider, best.URL}, "", "  ")
	if err := os.WriteFile(filepath.Join(final, "meta.json"), mb, 0o644); err != nil {
		return versions.V{}, err
	}
	return best.V, nil
}

// enableOpcache registers opcache.so when the build produced it.
func enableOpcache(prefix string) error {
	hits, _ := filepath.Glob(filepath.Join(prefix, "lib", "php", "extensions", "*", "opcache.so"))
	if len(hits) == 0 {
		return nil
	}
	return os.WriteFile(filepath.Join(prefix, "etc", "conf.d", "10-opcache.ini"),
		[]byte("zend_extension=opcache\nopcache.enable_cli=0\n"), 0o644)
}

func legacyHint(spec string) string {
	if strings.HasPrefix(spec, "5") || strings.HasPrefix(spec, "7") || strings.HasPrefix(spec, "8.0") {
		return "Legacy versions supported: 7.4 and 8.0 (built from source). Older ones are not supported yet."
	}
	return "Run `mrt php ls-remote` to see what is available."
}

// Uninstall removes an exact installed version.
func Uninstall(v string) error {
	if _, ok := versions.Parse(v); !ok {
		return fmt.Errorf("uninstall needs an exact version like 8.3.12")
	}
	dir := config.PHPVersionDir(v)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("PHP %s is not installed", v)
	}
	return os.RemoveAll(dir)
}
