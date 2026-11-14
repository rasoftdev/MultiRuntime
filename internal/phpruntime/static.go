package phpruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/rasoftdev/multiruntime/internal/config"
	"github.com/rasoftdev/multiruntime/internal/fetch"
	"github.com/rasoftdev/multiruntime/internal/versions"
)

const staticBase = "https://dl.static-php.dev/static-php-cli"

// staticProvider installs prebuilt self-contained binaries from static-php-cli.
// Upstream only publishes PHP >= 8.0 (and actively builds >= 8.1).
type staticProvider struct{ profile string }

func (s *staticProvider) Name() string { return "static-" + s.profile }

func (s *staticProvider) arch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "aarch64", nil
	}
	return "", fmt.Errorf("unsupported architecture %s", runtime.GOARCH)
}

func (s *staticProvider) List() ([]Release, error) {
	arch, err := s.arch()
	if err != nil {
		return nil, err
	}
	osName := runtime.GOOS
	if osName != "linux" && osName != "darwin" {
		return nil, fmt.Errorf("unsupported OS %s", osName)
	}
	if osName == "darwin" {
		osName = "macos"
	}
	body, err := fetch.GetBytes(fmt.Sprintf("%s/%s/?format=json", staticBase, s.profile))
	if err != nil {
		return nil, err
	}
	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("bad index: %w", err)
	}
	re := regexp.MustCompile(`^php-(\d+\.\d+\.\d+)-cli-` + osName + `-` + arch + `\.tar\.gz$`)
	var out []Release
	for _, e := range entries {
		m := re.FindStringSubmatch(e.Name)
		if m == nil {
			continue
		}
		v, ok := versions.Parse(m[1])
		if !ok {
			continue
		}
		out = append(out, Release{V: v, Provider: s.Name(), URL: fmt.Sprintf("%s/%s/%s", staticBase, s.profile, e.Name)})
	}
	return out, nil
}

func (s *staticProvider) Install(r Release, prefix string) error {
	archive := filepath.Join(config.CacheDir(), filepath.Base(r.URL))
	if _, err := os.Stat(archive); err != nil {
		if _, err := fetch.Download(r.URL, archive); err != nil {
			return err
		}
	}
	os.RemoveAll(prefix)
	bin := filepath.Join(prefix, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	fail := func(err error) error { os.RemoveAll(prefix); return err }
	if err := fetch.ExtractTarGz(archive, bin); err != nil {
		os.Remove(archive) // possibly corrupt; don't keep it
		return fail(err)
	}
	if err := os.Chmod(filepath.Join(bin, "php"), 0o755); err != nil {
		return fail(fmt.Errorf("archive did not contain bin/php: %w", err))
	}
	return nil
}
