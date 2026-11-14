package phpruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/rasoftdev/multiruntime/internal/config"
	"github.com/rasoftdev/multiruntime/internal/fetch"
	"github.com/rasoftdev/multiruntime/internal/versions"
)

// Extensions are compiled with phpize against an installed PHP, so they only
// work for versions built from source (static binaries cannot load .so files).

// zendExtensions must be loaded with zend_extension= instead of extension=.
var zendExtensions = map[string]bool{"xdebug": true, "opcache": true}

// extDeps lists bundled extensions that need a library the base build lacks.
var extDeps = map[string]struct {
	dep  dep
	flag string // configure flag; "%s" is replaced by the deps prefix
}{
	"gmp": {dep{name: "gmp", url: "https://ftp.gnu.org/gnu/gmp/gmp-6.3.0.tar.xz",
		sha:       "a3c2b80201b89e68616f4ad30bc66aee4927c3ce50e33929ca819d5c43538898",
		configure: func(p string) []string { return autoconf(p, "--disable-static") }}, "--with-gmp=%s"},
	"bz2": {dep{name: "bzip2", url: "https://sourceware.org/pub/bzip2/bzip2-1.0.8.tar.gz",
		sha:       "ab5a03176ee106d3f0fa90e381da478ddae405918153cca248e682cd0c4a2269",
		configure: func(p string) []string { return []string{"true"} },
		build:     [][]string{{"make", "CFLAGS=-fPIC -O2 -D_FILE_OFFSET_BITS=64"}},
		install:   [][]string{{"make", "install", "PREFIX=__DEPS__"}}}, "--with-bz2=%s"},
}

func metaOf(ver string) (meta, error) {
	var m meta
	data, err := os.ReadFile(filepath.Join(config.PHPVersionDir(ver), "meta.json"))
	if err != nil {
		return m, fmt.Errorf("PHP %s is not installed", ver)
	}
	return m, json.Unmarshal(data, &m)
}

func extDirOf(prefix string) (string, error) {
	hits, _ := filepath.Glob(filepath.Join(prefix, "lib", "php", "extensions", "*"))
	if len(hits) == 0 {
		return "", fmt.Errorf("no extension dir under %s", prefix)
	}
	return hits[0], nil
}

// ExtList returns the .ini-enabled extensions (name -> ini file) of a version.
func ExtList(ver string) ([]string, error) {
	confd := filepath.Join(config.PHPVersionDir(ver), "etc", "conf.d")
	ents, err := os.ReadDir(confd)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
}

// ExtRemove deletes the ini and shared object of an extension added with ExtAdd.
func ExtRemove(ver, name string) error {
	prefix := config.PHPVersionDir(ver)
	ini, _ := filepath.Glob(filepath.Join(prefix, "etc", "conf.d", "*-"+name+".ini"))
	if len(ini) == 0 {
		return fmt.Errorf("extension %s has no ini in PHP %s", name, ver)
	}
	for _, f := range ini {
		os.Remove(f)
	}
	if dir, err := extDirOf(prefix); err == nil {
		os.Remove(filepath.Join(dir, name+".so"))
	}
	return nil
}

func needTools(tools ...string) error {
	var miss []string
	for _, t := range tools {
		if _, err := exec.LookPath(t); err != nil {
			miss = append(miss, t)
		}
	}
	if len(miss) > 0 {
		return fmt.Errorf("missing tools: %s\n  Debian/Ubuntu: sudo apt install autoconf m4\n  Fedora/RHEL:   sudo dnf install autoconf m4", strings.Join(miss, ", "))
	}
	return nil
}

// ExtAdd builds and enables extension name ("redis", "xdebug@3.1.6", "gmp", ...) for PHP ver.
func ExtAdd(ver, nameSpec string) error {
	m, err := metaOf(ver)
	if err != nil {
		return err
	}
	if m.Provider == "homebrew" {
		return fmt.Errorf("PHP %s comes from Homebrew; add extensions there instead, e.g.\n  brew install shivammathur/extensions/redis@7.4", ver)
	}
	if m.Provider != "source" {
		return fmt.Errorf("PHP %s is a prebuilt static binary and cannot load extensions.\nReinstall it from source: mrt php uninstall %s && mrt php install %s --profile source", ver, ver, ver)
	}
	if miss := missingTools(); len(miss) > 0 {
		return fmt.Errorf("%s", installHint(miss))
	}
	if err := needTools("autoconf", "m4"); err != nil {
		return err
	}
	name, want, _ := strings.Cut(nameSpec, "@")
	name = strings.ToLower(name)
	prefix := config.PHPVersionDir(ver)
	pv, _ := versions.Parse(ver)
	work := filepath.Join(config.CacheDir(), "ext", ver+"-"+name)
	os.RemoveAll(work)
	b := newBuilder(work)
	b.env = append(b.env, "PATH="+filepath.Join(prefix, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	defer os.RemoveAll(work)

	srcDir, extra, err := fetchExtSource(b, ver, pv, name, want)
	if err != nil {
		return err
	}
	for _, x := range extra {
		if err := b.buildDep(x.dep); err != nil {
			return err
		}
	}
	var cfgFlags []string
	for _, x := range extra {
		cfgFlags = append(cfgFlags, strings.Replace(x.flag, "%s", b.deps, 1))
	}
	if err := b.run(srcDir, "phpize", "phpize"); err != nil {
		return err
	}
	cfg := append([]string{"./configure", "--with-php-config=" + filepath.Join(prefix, "bin", "php-config")}, cfgFlags...)
	if err := b.run(srcDir, "configure "+name, cfg...); err != nil {
		return err
	}
	if err := b.run(srcDir, "compile "+name, "make", "-j"+b.jobs); err != nil {
		return err
	}
	if err := b.run(srcDir, "install "+name, "make", "install"); err != nil {
		return err
	}
	dir, err := extDirOf(prefix)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, name+".so")); err != nil {
		return fmt.Errorf("build finished but %s.so was not installed", name)
	}
	directive := "extension"
	if zendExtensions[name] {
		directive = "zend_extension"
	}
	ini := filepath.Join(prefix, "etc", "conf.d", "30-"+name+".ini")
	return os.WriteFile(ini, []byte(directive+"="+name+"\n"), 0o644)
}

type extNeed = struct {
	dep  dep
	flag string
}

// fetchExtSource unpacks the extension source: bundled in the PHP tarball, or from PECL.
func fetchExtSource(b *builder, ver string, pv versions.V, name, want string) (string, []extNeed, error) {
	var extra []extNeed
	if x, ok := extDeps[name]; ok {
		d := x.dep
		if d.install != nil { // bzip2 style: substitute the deps prefix
			inst := make([][]string, len(d.install))
			for i, c := range d.install {
				inst[i] = append([]string(nil), c...)
				for j := range inst[i] {
					inst[i][j] = strings.Replace(inst[i][j], "__DEPS__", b.deps, 1)
				}
			}
			d.install = inst
		}
		extra = append(extra, extNeed{d, x.flag})
	}
	// 1) bundled with PHP?
	rels, err := fetchPHPNet(pv.Major)
	if err == nil {
		if rel, ok := rels[ver]; ok {
			for _, s := range rel.Source {
				if s.Filename != "php-"+ver+".tar.gz" {
					continue
				}
				archive, err := download("https://www.php.net/distributions/"+s.Filename, s.SHA256)
				if err != nil {
					return "", nil, err
				}
				dir := filepath.Join(b.work, "php-ext")
				os.MkdirAll(dir, 0o755)
				// Only unpack the one extension directory; absent means "not bundled".
				if exec.Command("tar", "-xf", archive, "--strip-components=1", "-C", dir, "php-"+ver+"/ext/"+name).Run() == nil {
					if _, err := os.Stat(filepath.Join(dir, "ext", name, "config.m4")); err == nil {
						return filepath.Join(dir, "ext", name), extra, nil
					}
				}
			}
		}
	}
	// 2) PECL
	pkgVer := want
	if pkgVer == "" {
		pkgVer, err = peclPick(name, ver)
		if err != nil {
			return "", nil, err
		}
	}
	url := fmt.Sprintf("https://pecl.php.net/get/%s-%s.tgz", name, pkgVer)
	archive := filepath.Join(config.CacheDir(), fmt.Sprintf("pecl-%s-%s.tgz", name, pkgVer))
	if _, err := os.Stat(archive); err != nil {
		fmt.Fprintf(os.Stderr, "  downloading %s %s from PECL\n", name, pkgVer)
		if _, err := fetch.Download(url, archive); err != nil {
			return "", nil, fmt.Errorf("PECL package %s %s: %w", name, pkgVer, err)
		}
	}
	dir := filepath.Join(b.work, "pecl")
	os.MkdirAll(dir, 0o755)
	if err := b.run(dir, "unpack "+name, "tar", "-xf", archive, "-C", dir); err != nil {
		return "", nil, err
	}
	var found string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "config.m4" && found == "" {
			found = filepath.Dir(p)
		}
		return nil
	})
	if found == "" {
		return "", nil, fmt.Errorf("%s is not a compilable PECL extension (no config.m4)", name)
	}
	return found, extra, nil
}

var (
	reVersion = regexp.MustCompile(`<v>([^<]+)</v>\s*<s>([^<]+)</s>`)
	rePHPMin  = regexp.MustCompile(`<php>\s*<min>([\d.]+)</min>(?:\s*<max>([\d.]+)</max>)?`)
)

// peclPick finds the newest stable release of a PECL package that supports PHP ver.
func peclPick(name, ver string) (string, error) {
	pv, _ := versions.Parse(ver)
	body, err := fetch.GetBytes("https://pecl.php.net/rest/r/" + name + "/allreleases.xml")
	if err != nil {
		return "", fmt.Errorf("PECL package %q not found", name)
	}
	for _, m := range reVersion.FindAllStringSubmatch(string(body), -1) {
		if m[2] != "stable" {
			continue
		}
		info, err := fetch.GetBytes(fmt.Sprintf("https://pecl.php.net/rest/r/%s/package.%s.xml", name, m[1]))
		if err != nil {
			continue
		}
		pm := rePHPMin.FindStringSubmatch(string(info))
		if pm == nil {
			return m[1], nil
		}
		if min, ok := parseLoose(pm[1]); ok && pv.Cmp(min) < 0 {
			continue // needs a newer PHP than this one
		}
		return m[1], nil
	}
	return "", fmt.Errorf("no stable release of %s supports PHP %s (try %s@<version>)", name, ver, name)
}

// parseLoose parses "7.4" or "7.4.0" style versions.
func parseLoose(s string) (versions.V, bool) {
	p := strings.Split(s, ".")
	for len(p) < 3 {
		p = append(p, "0")
	}
	return versions.Parse(strings.Join(p[:3], "."))
}
