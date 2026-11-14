package phpruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rasoftdev/multiruntime/internal/config"
	"github.com/rasoftdev/multiruntime/internal/fetch"
	"github.com/rasoftdev/multiruntime/internal/versions"
)

// sourceProvider compiles PHP from php.net sources. All the libraries PHP needs
// (OpenSSL 1.1, libxml2, ICU, ...) are built once into a shared private prefix,
// so the result does not depend on the host distro's library versions and the
// newest patch release is available the day php.net publishes it.
type sourceProvider struct{}

func (sourceProvider) Name() string { return "source" }

// depsID names the shared library set. Bump it when legacyDeps changes so
// stale sets are rebuilt instead of reused.
const depsID = "v1"

// A dep is a library built into the shared deps prefix before PHP itself.
type dep struct {
	name, url, sha string
	subdir         string                  // build from this subdirectory of the archive
	configure      func(p string) []string // autotools configure line
	cmake          []string                // extra cmake flags (used instead of configure)
	build, install [][]string              // override make / make install
}

func autoconf(p string, extra ...string) []string {
	return append([]string{"./configure", "--prefix=" + p, "--libdir=" + p + "/lib"}, extra...)
}

var legacyDeps = []dep{
	{name: "zlib", url: "https://zlib.net/fossils/zlib-1.3.1.tar.gz",
		sha:       "9a93b2b7dfdac77ceba5a558a580e74667dd6fede4585b91eefb60f03b72df23",
		configure: func(p string) []string { return []string{"./configure", "--prefix=" + p, "--libdir=" + p + "/lib"} }},
	{name: "openssl", url: "https://www.openssl.org/source/old/1.1.1/openssl-1.1.1w.tar.gz",
		sha:     "cf3098950cb4d853ad95c0841f1f9c6d3dc102dccfcacd521d93925208b76ac8",
		install: [][]string{{"make", "install_sw"}},
		configure: func(p string) []string {
			return []string{"./config", "--prefix=" + p, "--openssldir=" + p + "/ssl", "--libdir=lib",
				"shared", "no-tests", "-Wl,-rpath," + p + "/lib"}
		}},
	{name: "libxml2", url: "https://download.gnome.org/sources/libxml2/2.9/libxml2-2.9.14.tar.xz",
		sha: "60d74a257d1ccec0475e749cba2f21559e48139efba6ff28224357c7c798dfee",
		configure: func(p string) []string {
			return autoconf(p, "--without-python", "--without-lzma", "--without-icu", "--with-zlib="+p, "--disable-static")
		}},
	{name: "libxslt", url: "https://download.gnome.org/sources/libxslt/1.1/libxslt-1.1.37.tar.xz",
		sha: "3a4b27dc8027ccd6146725950336f1ec520928f320f144eb5fa7990ae6123ab4",
		configure: func(p string) []string {
			return autoconf(p, "--with-libxml-prefix="+p, "--without-python", "--without-crypto", "--disable-static")
		}},
	{name: "oniguruma", url: "https://github.com/kkos/oniguruma/releases/download/v6.9.9/onig-6.9.9.tar.gz",
		sha:       "60162bd3b9fc6f4886d4c7a07925ffd374167732f55dce8c491bfd9cd818a6cf",
		configure: func(p string) []string { return autoconf(p, "--disable-static") }},
	{name: "sqlite", url: "https://www.sqlite.org/2024/sqlite-autoconf-3450100.tar.gz",
		sha:       "cd9c27841b7a5932c9897651e20b86c701dd740556989b01ca596fcfa3d49a0a",
		configure: func(p string) []string { return autoconf(p, "--disable-static") }},
	{name: "curl", url: "https://curl.se/download/curl-8.5.0.tar.gz",
		sha: "05fc17ff25b793a437a0906e0484b82172a9f4de02be5ed447e0cab8c3475add",
		configure: func(p string) []string {
			return autoconf(p, "--with-openssl="+p, "--with-zlib="+p, "--disable-static", "--disable-ldap",
				"--without-libpsl", "--without-brotli", "--without-zstd", "--without-nghttp2",
				"--without-libidn2", "--without-librtmp", "--without-libssh2")
		}},
	{name: "libpng", url: "https://downloads.sourceforge.net/project/libpng/libpng16/1.6.43/libpng-1.6.43.tar.gz",
		sha:       "e804e465d4b109b5ad285a8fb71f0dd3f74f0068f91ce3cdfde618180c174925",
		configure: func(p string) []string { return autoconf(p, "--disable-static") }},
	{name: "libjpeg-turbo", url: "https://github.com/libjpeg-turbo/libjpeg-turbo/releases/download/3.0.3/libjpeg-turbo-3.0.3.tar.gz",
		sha:   "343e789069fc7afbcdfe44dbba7dbbf45afa98a15150e079a38e60e44578865d",
		cmake: []string{"-DENABLE_STATIC=OFF", "-DWITH_SIMD=0", "-DWITH_TURBOJPEG=OFF", "-DCMAKE_INSTALL_DEFAULT_LIBDIR=lib"}},
	{name: "freetype", url: "https://downloads.sourceforge.net/project/freetype/freetype2/2.13.2/freetype-2.13.2.tar.xz",
		sha: "12991c4e55c506dd7f9b765933e62fd2be2e06d421505d7950a132e4f1bb484d",
		configure: func(p string) []string {
			return autoconf(p, "--disable-static", "--without-harfbuzz", "--without-brotli", "--without-bzip2",
				"--with-png=yes", "--with-zlib=yes")
		}},
	{name: "libzip", url: "https://github.com/nih-at/libzip/releases/download/v1.10.1/libzip-1.10.1.tar.xz",
		sha: "dc3c8d5b4c8bbd09626864f6bcf93de701540f761d76b85d7c7d710f4bd90318",
		cmake: []string{"-DBUILD_SHARED_LIBS=ON", "-DENABLE_COMMONCRYPTO=OFF", "-DENABLE_GNUTLS=OFF", "-DENABLE_MBEDTLS=OFF",
			"-DENABLE_OPENSSL=OFF", "-DENABLE_BZIP2=OFF", "-DENABLE_LZMA=OFF", "-DENABLE_ZSTD=OFF",
			"-DBUILD_TOOLS=OFF", "-DBUILD_REGRESS=OFF", "-DBUILD_EXAMPLES=OFF", "-DBUILD_DOC=OFF",
			"-DCMAKE_INSTALL_LIBDIR=lib"}},
	{name: "icu", url: "https://github.com/unicode-org/icu/releases/download/release-72-1/icu4c-72_1-src.tgz",
		sha: "a2d2d38217092a7ed56635e34467f92f976b370e20182ad325edea6681a71d68", subdir: "source",
		configure: func(p string) []string {
			return autoconf(p, "--disable-static", "--disable-tests", "--disable-samples", "--disable-extras")
		}},
	{name: "libsodium", url: "https://github.com/jedisct1/libsodium/releases/download/1.0.18-RELEASE/libsodium-1.0.18.tar.gz",
		sha:       "6f504490b342a4f8a4c4a02fc9b866cbef8622d5df4e5452b46be121e46636c1",
		configure: func(p string) []string { return autoconf(p, "--disable-static") }},
	{name: "ncurses", url: "https://ftp.gnu.org/gnu/ncurses/ncurses-6.4.tar.gz",
		sha: "6931283d9ac87c5073f30b6290c4c75f21632bb4fc3603ac8100812bed248159",
		configure: func(p string) []string {
			return autoconf(p, "--with-shared", "--without-debug", "--without-ada", "--without-cxx-binding",
				"--without-manpages", "--without-tests", "--enable-pc-files", "--with-pkg-config-libdir="+p+"/lib/pkgconfig",
				"--with-terminfo-dirs=/etc/terminfo:/lib/terminfo:/usr/share/terminfo:/usr/lib/terminfo")
		}},
	{name: "readline", url: "https://ftp.gnu.org/gnu/readline/readline-8.2.tar.gz",
		sha:       "3feb7171f16a84ee82ca18a36d7b9be109a52c04f492a053331d7d1095007c35",
		configure: func(p string) []string { return autoconf(p, "--with-curses", "--disable-static") }},
	{name: "libpq", url: "https://ftp.postgresql.org/pub/source/v16.4/postgresql-16.4.tar.gz",
		sha: "2e17a90062403e15d6540480fdec50c8b005eb48729a91cb4989ffeb04df193c",
		configure: func(p string) []string {
			return autoconf(p, "--without-readline", "--without-icu", "--without-zlib", "--with-openssl")
		},
		build:   [][]string{{"make", "-C", "src/include"}, {"make", "-C", "src/interfaces/libpq"}, {"make", "-C", "src/bin/pg_config"}},
		install: [][]string{{"make", "-C", "src/include", "install"}, {"make", "-C", "src/interfaces/libpq", "install"}, {"make", "-C", "src/bin/pg_config", "install"}}},
}

// CMake is only needed to build a few libraries; a pinned official binary is
// used so the host does not need it installed.
var cmakeTools = map[string]struct{ url, sha string }{
	"amd64": {"https://github.com/Kitware/CMake/releases/download/v3.28.6/cmake-3.28.6-linux-x86_64.tar.gz",
		"931e3c0d546ee03ca72bb147ccd9b49e3b6252f765f66bf21b9d165519940458"},
	"arm64": {"https://github.com/Kitware/CMake/releases/download/v3.28.6/cmake-3.28.6-linux-aarch64.tar.gz",
		"7909cc2128ce9442c63ce674a0bfb0e4f4ce04cef667d887e15ad5670d594ba7"},
}

// Release metadata from php.net.
type phpNetRelease struct {
	Source []struct {
		Filename string `json:"filename"`
		SHA256   string `json:"sha256"`
	} `json:"source"`
}

func fetchPHPNet(major int) (map[string]phpNetRelease, error) {
	b, err := fetch.GetBytes(fmt.Sprintf("https://www.php.net/releases/index.php?json&version=%d&max=1000", major))
	if err != nil {
		return nil, err
	}
	m := map[string]phpNetRelease{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("bad php.net response: %w", err)
	}
	return m, nil
}

// supported: the configure flag set below targets PHP 7.4 through 8.5.
func supported(v versions.V) bool {
	return (v.Major == 7 && v.Minor == 4) || (v.Major == 8 && v.Minor <= 5)
}

func (sourceProvider) List() ([]Release, error) {
	var out []Release
	for _, major := range []int{7, 8} {
		m, err := fetchPHPNet(major)
		if err != nil {
			return nil, err
		}
		for ver, rel := range m {
			v, ok := versions.Parse(ver)
			if !ok || !supported(v) {
				continue
			}
			for _, s := range rel.Source {
				if s.Filename == "php-"+ver+".tar.gz" {
					out = append(out, Release{V: v, Provider: "source",
						URL: "https://www.php.net/distributions/" + s.Filename + "#sha256=" + s.SHA256})
				}
			}
		}
	}
	return out, nil
}

func missingTools() []string {
	var miss []string
	cc := "gcc"
	if _, err := exec.LookPath("gcc"); err != nil {
		if _, err := exec.LookPath("cc"); err == nil {
			cc = "cc"
		}
	}
	for _, t := range []string{cc, "make", "perl", "pkg-config", "tar", "xz"} {
		if _, err := exec.LookPath(t); err != nil {
			miss = append(miss, t)
		}
	}
	return miss
}

func installHint(miss []string) string {
	return fmt.Sprintf("missing build tools: %s\n  Debian/Ubuntu: sudo apt install build-essential perl pkg-config xz-utils\n  Fedora/RHEL:   sudo dnf install gcc gcc-c++ make perl pkgconf-pkg-config xz\n  Alpine:        sudo apk add build-base perl pkgconf xz",
		strings.Join(miss, ", "))
}

type builder struct {
	deps, work, logPath string
	env                 []string
	jobs                string
}

func (b *builder) run(dir, step string, args ...string) error {
	fmt.Fprintf(os.Stderr, "  %s\n", step)
	log, err := os.OpenFile(b.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()
	fmt.Fprintf(log, "\n$ (%s) %s\n", dir, strings.Join(args, " "))
	cmd := exec.Command(b.lookPath(args[0]), args[1:]...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, b.env, log, log
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w\n--- last lines of %s ---\n%s", step, err, b.logPath, tail(b.logPath, 30))
	}
	return nil
}

// lookPath resolves a command against the PATH in b.env (the last PATH entry
// wins), not the PATH of the mrt process: tools such as the bundled cmake or
// the freshly built phpize are only reachable through the build environment.
func (b *builder) lookPath(name string) string {
	if strings.ContainsRune(name, '/') {
		return name
	}
	pathVar := os.Getenv("PATH")
	for _, e := range b.env {
		if v, ok := strings.CutPrefix(e, "PATH="); ok {
			pathVar = v
		}
	}
	for _, d := range filepath.SplitList(pathVar) {
		if d == "" {
			continue
		}
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p
		}
	}
	return name
}

func tail(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// download fetches url into the cache and verifies its sha256.
func download(url, sha string) (string, error) {
	url, _, _ = strings.Cut(url, "#")
	archive := filepath.Join(config.CacheDir(), filepath.Base(url))
	verify := func() bool {
		got, err := fetch.SHA256File(archive)
		return err == nil && strings.EqualFold(got, sha)
	}
	if !verify() {
		fmt.Fprintf(os.Stderr, "  downloading %s\n", filepath.Base(url))
		if _, err := fetch.Download(url, archive); err != nil {
			return "", err
		}
		if !verify() {
			os.Remove(archive)
			return "", fmt.Errorf("checksum mismatch for %s", filepath.Base(url))
		}
	}
	return archive, nil
}

// fetchSource downloads (cached, verified) and unpacks into work/<name>.
func (b *builder) fetchSource(name, url, sha string) (string, error) {
	archive, err := download(url, sha)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(b.work, name)
	os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := b.run(dir, "unpack "+name, "tar", "-xf", archive, "--strip-components=1", "-C", dir); err != nil {
		return "", err
	}
	return dir, nil
}

// ensureCMake unpacks the pinned cmake and puts it first on PATH.
func (b *builder) ensureCMake() error {
	t, ok := cmakeTools[runtime.GOARCH]
	if !ok {
		return fmt.Errorf("no bundled cmake for %s", runtime.GOARCH)
	}
	root := filepath.Join(config.ToolsDir(), "cmake-3.28.6")
	if _, err := os.Stat(filepath.Join(root, "bin", "cmake")); err != nil {
		archive, err := download(t.url, t.sha)
		if err != nil {
			return err
		}
		os.RemoveAll(root)
		if err := os.MkdirAll(root, 0o755); err != nil {
			return err
		}
		if err := b.run(root, "unpack cmake", "tar", "-xf", archive, "--strip-components=1", "-C", root); err != nil {
			return err
		}
	}
	b.env = append(b.env, "PATH="+filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	return nil
}

func (b *builder) marker(name string) string { return filepath.Join(b.deps, ".mrt-built-"+name) }

func (b *builder) buildDep(d dep) error {
	if _, err := os.Stat(b.marker(d.name)); err == nil {
		return nil
	}
	dir, err := b.fetchSource(d.name, d.url, d.sha)
	if err != nil {
		return err
	}
	wd := filepath.Join(dir, d.subdir)
	steps := func(label string, cmds [][]string) error {
		for _, c := range cmds {
			if err := b.run(wd, label+" "+d.name, c...); err != nil {
				return err
			}
		}
		return nil
	}
	if d.cmake != nil {
		args := append([]string{"cmake", "-S", ".", "-B", "build", "-DCMAKE_INSTALL_PREFIX=" + b.deps,
			"-DCMAKE_BUILD_TYPE=Release", "-DCMAKE_PREFIX_PATH=" + b.deps,
			"-DCMAKE_INSTALL_RPATH=" + b.deps + "/lib"}, d.cmake...)
		if err := b.run(wd, "configure "+d.name, args...); err != nil {
			return err
		}
		if err := b.run(wd, "compile "+d.name, "cmake", "--build", "build", "-j", b.jobs); err != nil {
			return err
		}
		if err := b.run(wd, "install "+d.name, "cmake", "--install", "build"); err != nil {
			return err
		}
	} else {
		if err := b.run(wd, "configure "+d.name, d.configure(b.deps)...); err != nil {
			return err
		}
		build := d.build
		if build == nil {
			build = [][]string{{"make", "-j" + b.jobs}}
		}
		if err := steps("compile", build); err != nil {
			return err
		}
		inst := d.install
		if inst == nil {
			inst = [][]string{{"make", "install"}}
		}
		if err := steps("install", inst); err != nil {
			return err
		}
	}
	os.RemoveAll(dir)
	return os.WriteFile(b.marker(d.name), nil, 0o644)
}

// newBuilder prepares the shared environment used for dependency and PHP builds.
func newBuilder(work string) *builder {
	deps := filepath.Join(config.Home(), "deps", depsID)
	os.MkdirAll(deps, 0o755)
	os.MkdirAll(work, 0o755)
	b := &builder{deps: deps, work: work, logPath: filepath.Join(work, "build.log"), jobs: fmt.Sprint(runtime.NumCPU())}
	cflags := "-O2 -fPIC -std=gnu11 -Wno-implicit-function-declaration -Wno-int-conversion -Wno-incompatible-pointer-types -Wno-return-mismatch -Wno-deprecated-declarations -Wno-error"
	b.env = append(os.Environ(),
		"CFLAGS="+cflags,
		"CPPFLAGS=-I"+deps+"/include",
		"LDFLAGS=-L"+deps+"/lib -Wl,-rpath,"+deps+"/lib",
		"PKG_CONFIG_PATH="+deps+"/lib/pkgconfig",
		"LC_ALL=C",
	)
	return b
}

func (s sourceProvider) Install(r Release, prefix string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("source builds are only supported on Linux")
	}
	if miss := missingTools(); len(miss) > 0 {
		return fmt.Errorf("%s", installHint(miss))
	}
	_, sha, _ := strings.Cut(r.URL, "#sha256=")
	if sha == "" {
		return fmt.Errorf("no checksum for %s", r.URL)
	}
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return err
	}
	work := filepath.Join(config.CacheDir(), "build", r.V.String())
	b := newBuilder(work)
	deps := b.deps
	fmt.Fprintf(os.Stderr, "Building PHP %s from source (log: %s)\n", r.V, b.logPath)
	if _, err := os.Stat(b.marker("libpq")); err != nil {
		fmt.Fprintln(os.Stderr, "First build also compiles the shared libraries (~15 min extra, done once).")
	}
	if err := b.ensureCMake(); err != nil {
		return err
	}
	for _, d := range legacyDeps {
		if err := b.buildDep(d); err != nil {
			return err
		}
	}

	src, err := b.fetchSource("php", r.URL, sha)
	if err != nil {
		return err
	}
	cfg := []string{"./configure",
		"--prefix=" + prefix,
		"--with-config-file-path=" + prefix + "/etc",
		"--with-config-file-scan-dir=" + prefix + "/etc/conf.d",
		"--disable-cgi", "--disable-phpdbg", "--without-pear", "--enable-fpm",
		"--with-openssl", "--with-zlib", "--with-curl", "--with-libxml",
		"--enable-mbstring", "--enable-mysqlnd",
		"--with-mysqli=mysqlnd", "--with-pdo-mysql=mysqlnd",
		"--with-sqlite3", "--with-pdo-sqlite",
		"--with-pgsql=" + deps, "--with-pdo-pgsql=" + deps,
		"--enable-gd", "--with-jpeg", "--with-freetype",
		"--with-zip", "--enable-intl", "--with-sodium", "--with-readline=" + deps,
		"--with-xsl", "--enable-soap", "--enable-ftp", "--with-gettext",
		"--enable-bcmath", "--enable-calendar", "--enable-exif", "--enable-pcntl",
		"--enable-sockets", "--enable-opcache",
		"--enable-shmop", "--enable-sysvsem", "--enable-sysvshm", "--enable-sysvmsg",
	}
	if err := b.run(src, "configure php", cfg...); err != nil {
		return err
	}
	if err := b.run(src, "compile php", "make", "-j"+b.jobs); err != nil {
		return err
	}
	if err := b.run(src, "install php", "make", "install"); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(prefix, "bin", "php")); err != nil {
		return fmt.Errorf("build finished without bin/php")
	}
	os.RemoveAll(work)
	return nil
}
