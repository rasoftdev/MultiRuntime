// Package phpruntime manages PHP installations, version resolution and execution.
package phpruntime

import (
	"fmt"

	"github.com/rasoftdev/multiruntime/internal/versions"
)

// Release is one installable PHP build offered by a provider.
type Release struct {
	V        versions.V
	Provider string
	URL      string
}

// Provider knows how to list and install PHP builds from some source.
// New sources (distro packages, source builds, containers) implement this.
type Provider interface {
	Name() string
	List() ([]Release, error)
	// Install must leave an executable at <prefix>/bin/php.
	Install(r Release, prefix string) error
}

var providers = []Provider{&staticProvider{profile: "bulk"}, sourceProvider{}}

// SetProfile selects the static-php-cli build profile (bulk, common, minimal),
// or "source" to force compiling from php.net sources.
func SetProfile(p string) {
	if p == "prebuilt" { // accept the newest prebuilt binary even if php.net has a newer patch
		providers = []Provider{&staticProvider{profile: "bulk"}}
		return
	}
	if p == "source" {
		providers = []Provider{sourceProvider{}}
		return
	}
	providers = []Provider{&staticProvider{profile: p}, sourceProvider{}}
}

// Remote lists every release from every provider, newest preference first per version.
func Remote() ([]Release, error) {
	var all []Release
	var firstErr error
	for _, p := range providers {
		rs, err := p.List()
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", p.Name(), err)
			}
			continue
		}
		all = append(all, rs...)
	}
	if len(all) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return all, nil
}

func providerByName(n string) Provider {
	for _, p := range providers {
		if p.Name() == n {
			return p
		}
	}
	return nil
}
