package phpruntime

import (
	"fmt"
	"os"

	"github.com/rasoftdev/multiruntime/internal/config"
	"github.com/rasoftdev/multiruntime/internal/versions"
)

// Upgrade is an installed minor line that has a newer patch release available.
type Upgrade struct{ Have, Latest versions.V }

// newestRemote returns the highest installable patch per "major.minor".
func newestRemote() (map[string]versions.V, error) {
	rels, err := Remote()
	if err != nil {
		return nil, err
	}
	best := map[string]versions.V{}
	for _, r := range rels {
		k := fmt.Sprintf("%d.%d", r.V.Major, r.V.Minor)
		if cur, ok := best[k]; !ok || r.V.Cmp(cur) > 0 {
			best[k] = r.V
		}
	}
	return best, nil
}

// newestInstalled returns the highest installed patch per "major.minor".
func newestInstalled() map[string]versions.V {
	have := map[string]versions.V{}
	for _, v := range Installed() {
		k := fmt.Sprintf("%d.%d", v.Major, v.Minor)
		if cur, ok := have[k]; !ok || v.Cmp(cur) > 0 {
			have[k] = v
		}
	}
	return have
}

// Outdated lists installed minor lines whose newest installable patch is newer.
func Outdated() ([]Upgrade, error) {
	remote, err := newestRemote()
	if err != nil {
		return nil, err
	}
	var out []Upgrade
	for k, v := range newestInstalled() {
		if r, ok := remote[k]; ok && r.Cmp(v) > 0 {
			out = append(out, Upgrade{Have: v, Latest: r})
		}
	}
	return out, nil
}

// Update installs the newest patch of every installed minor line (or only the
// ones matching spec). With prune, older patches of the same line are removed.
func Update(spec string, prune bool) error {
	ups, err := Outdated()
	if err != nil {
		return err
	}
	done := 0
	for _, u := range ups {
		if spec != "" && !versions.Match(spec, u.Have) {
			continue
		}
		fmt.Fprintf(os.Stderr, "Updating %d.%d: %s -> %s\n", u.Have.Major, u.Have.Minor, u.Have, u.Latest)
		if _, err := Install(u.Latest.String()); err != nil {
			return err
		}
		done++
		if prune {
			for _, v := range Installed() {
				if v.Major == u.Latest.Major && v.Minor == u.Latest.Minor && v.Cmp(u.Latest) < 0 {
					os.RemoveAll(config.PHPVersionDir(v.String()))
					fmt.Fprintf(os.Stderr, "Removed %s\n", v)
				}
			}
		}
	}
	if done == 0 {
		fmt.Fprintln(os.Stderr, "Everything is up to date.")
	}
	return nil
}
