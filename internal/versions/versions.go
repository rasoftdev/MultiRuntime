// Package versions parses and matches dotted version numbers.
package versions

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// V is a major.minor.patch version.
type V struct{ Major, Minor, Patch int }

func (v V) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

// Cmp returns -1, 0 or 1.
func (v V) Cmp(o V) int {
	switch {
	case v.Major != o.Major:
		return sign(v.Major - o.Major)
	case v.Minor != o.Minor:
		return sign(v.Minor - o.Minor)
	default:
		return sign(v.Patch - o.Patch)
	}
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	if n > 0 {
		return 1
	}
	return 0
}

// Parse parses a full "X.Y.Z" version.
func Parse(s string) (V, bool) {
	p, ok := parts(s)
	if !ok || len(p) != 3 {
		return V{}, false
	}
	return V{p[0], p[1], p[2]}, true
}

func parts(s string) ([]int, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if s == "" {
		return nil, false
	}
	var out []int
	for _, f := range strings.Split(s, ".") {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return nil, false
		}
		out = append(out, n)
	}
	if len(out) > 3 {
		return nil, false
	}
	return out, true
}

// ValidSpec reports whether s is "8", "8.3" or "8.3.12".
func ValidSpec(s string) bool { _, ok := parts(s); return ok }

// Match reports whether v satisfies spec ("7", "7.4" or "7.4.33").
func Match(spec string, v V) bool {
	p, ok := parts(spec)
	if !ok {
		return false
	}
	have := []int{v.Major, v.Minor, v.Patch}
	for i, n := range p {
		if have[i] != n {
			return false
		}
	}
	return true
}

// Latest returns the highest version in list that satisfies spec.
func Latest(spec string, list []V) (V, bool) {
	var best V
	found := false
	for _, v := range list {
		if Match(spec, v) && (!found || v.Cmp(best) > 0) {
			best, found = v, true
		}
	}
	return best, found
}

// Sort sorts ascending in place.
func Sort(list []V) { sort.Slice(list, func(i, j int) bool { return list[i].Cmp(list[j]) < 0 }) }
