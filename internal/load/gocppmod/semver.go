package gocppmod

import (
	"strings"
)

// A semver is a version split for comparison. The numbers stay strings: the
// elements of a version are not bounded, and a longer run of digits is the
// bigger number (there are no leading zeros).
type semver struct {
	major, minor, patch string
	pre                 []string // dot separated prerelease; nil for a release
}

func parseSemver(v string) (semver, bool) {
	if !ValidVersion(v) {
		return semver{}, false
	}
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	var pre string
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v, pre = v[:i], v[i+1:]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	s := semver{major: parts[0], minor: parts[1], patch: parts[2]}
	if pre != "" {
		s.pre = strings.Split(pre, ".")
	}
	return s, true
}

func isNum(s string) bool { return s != "" && strings.Trim(s, "0123456789") == "" }

func cmpNum(a, b string) int {
	switch {
	case len(a) != len(b):
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

// CompareVersions compares two module versions by semantic versioning
// precedence: it returns -1, 0 or 1. Build metadata (+incompatible) is
// ignored, as in Go. A string that is not a valid version is lower than every
// valid one, and two of those are compared as strings; the empty string
// ("no version") is lower than all.
func CompareVersions(a, b string) int {
	pa, oka := parseSemver(a)
	pb, okb := parseSemver(b)
	switch {
	case !oka && !okb:
		return strings.Compare(a, b)
	case !oka:
		return -1
	case !okb:
		return 1
	}
	for _, d := range [3]int{cmpNum(pa.major, pb.major), cmpNum(pa.minor, pb.minor), cmpNum(pa.patch, pb.patch)} {
		if d != 0 {
			return d
		}
	}
	switch {
	case pa.pre == nil && pb.pre == nil:
		return 0
	case pa.pre == nil:
		return 1
	case pb.pre == nil:
		return -1
	}
	for i := 0; i < len(pa.pre) && i < len(pb.pre); i++ {
		x, y := pa.pre[i], pb.pre[i]
		xn, yn := isNum(x), isNum(y)
		switch {
		case xn && yn:
			if c := cmpNum(x, y); c != 0 {
				return c
			}
		case xn:
			return -1
		case yn:
			return 1
		case x != y:
			return strings.Compare(x, y)
		}
	}
	switch {
	case len(pa.pre) < len(pb.pre):
		return -1
	case len(pa.pre) > len(pb.pre):
		return 1
	}
	return 0
}

// IsPrerelease reports whether v has a prerelease part (v1.2.3-rc.1, and
// every pseudo-version).
func IsPrerelease(v string) bool {
	p, ok := parseSemver(v)
	return ok && p.pre != nil
}

// Major returns the major version number of v as text ("2" for v2.1.0), or
// "" if v is not a version.
func Major(v string) string {
	p, ok := parseSemver(v)
	if !ok {
		return ""
	}
	return p.major
}
