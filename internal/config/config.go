// Package config resolves the on-disk layout of the mrt home directory.
package config

import (
	"os"
	"path/filepath"
)

// Home returns the mrt home ($MRT_HOME or ~/.mrt).
func Home() string {
	if h := os.Getenv("MRT_HOME"); h != "" {
		return h
	}
	d, err := os.UserHomeDir()
	if err != nil {
		d = "."
	}
	return filepath.Join(d, ".mrt")
}

func ShimsDir() string      { return filepath.Join(Home(), "shims") }
func ToolsDir() string      { return filepath.Join(Home(), "tools") }
func CacheDir() string      { return filepath.Join(Home(), "cache") }
func PHPDir() string        { return filepath.Join(Home(), "php") }
func PHPVersions() string   { return filepath.Join(PHPDir(), "versions") }
func PHPGlobalFile() string { return filepath.Join(PHPDir(), "global") }

// PHPVersionDir is the install prefix of an exact PHP version.
func PHPVersionDir(v string) string { return filepath.Join(PHPVersions(), v) }
