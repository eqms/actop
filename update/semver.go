// Package update checks whether newer releases of Apple Container and of
// actop are available. It only reports; it never installs anything.
//
// It is a leaf package: stdlib only, no knowledge of the CLI or the UI.
package update

import (
	"strconv"
	"strings"
)

// Version is a three-component release version. Prerelease and build
// metadata are deliberately unsupported: the GitHub /releases/latest
// endpoint never points at a prerelease, so anything carrying a suffix is
// not a release actop should advertise.
type Version struct{ Major, Minor, Patch int }

// Parse accepts "1.2.0" and "v1.2.0" and nothing else. It is the primary
// input filter for remote release tags: a tag that does not match this
// grammar never reaches the render path, so terminal escape sequences
// smuggled into a tag name are rejected here rather than sanitized later.
func Parse(s string) (Version, bool) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	var out [3]int
	for i, p := range parts {
		// strconv.Atoi accepts a leading sign; a version component must be
		// plain digits, so reject anything else outright.
		if p == "" || strings.IndexFunc(p, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return Version{}, false
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, false
		}
		out[i] = n
	}
	return Version{Major: out[0], Minor: out[1], Patch: out[2]}, true
}

// Less reports whether v precedes o.
func (v Version) Less(o Version) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor < o.Minor
	}
	return v.Patch < o.Patch
}

// String returns the canonical form without a "v" prefix.
func (v Version) String() string {
	return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
}
