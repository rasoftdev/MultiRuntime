package phpruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rasoftdev/multiruntime/internal/config"
	"github.com/rasoftdev/multiruntime/internal/fetch"
	"github.com/rasoftdev/multiruntime/internal/versions"
)

// ComposerChannel picks the Composer line that supports the given PHP.
// Composer 2.3+ needs PHP 7.2.5+; the 2.2 LTS line covers 5.3.2+.
func ComposerChannel(v string) string {
	pv, ok := versions.Parse(v)
	if ok && pv.Cmp(versions.V{Major: 7, Minor: 2, Patch: 5}) < 0 {
		return "latest-2.2.x"
	}
	return "latest-stable"
}

// ComposerPath is where the phar for a channel lives.
func ComposerPath(channel string) string {
	return filepath.Join(config.ToolsDir(), "composer", channel, "composer.phar")
}

// EnsureComposer downloads and verifies the phar for channel if missing.
func EnsureComposer(channel string) (string, error) {
	dest := ComposerPath(channel)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	base := "https://getcomposer.org/download/" + channel + "/composer.phar"
	sumRaw, err := fetch.GetBytes(base + ".sha256sum")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(sumRaw))
	if len(fields) == 0 {
		return "", fmt.Errorf("empty checksum for composer %s", channel)
	}
	fmt.Fprintf(os.Stderr, "Installing Composer (%s)\n", channel)
	got, err := fetch.Download(base, dest)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(got, fields[0]) {
		os.Remove(dest)
		return "", fmt.Errorf("composer checksum mismatch: got %s want %s", got, fields[0])
	}
	if err := os.Chmod(dest, 0o755); err != nil {
		return "", err
	}
	return dest, nil
}
