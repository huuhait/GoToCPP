package modfetch

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// A Repo is the repository that a module path names, and where in it the
// module is.
type Repo struct {
	// Module is the module path as written.
	Module string

	// Host is the first element of the module path: github.com.
	Host string

	// Root is the module path of the repository itself: github.com/o/r.
	Root string

	// Subdir is the folder of the module in the repository ("" for the
	// top). Its tags are named Subdir + "/" + version, as in Go.
	Subdir string

	// Major is N for a module path that ends with /vN (N >= 2): the major
	// version of the module must be N. 0 for any other path.
	Major int
}

// gitHosts are the hosts whose module paths are host/owner/repo[/subdir].
var gitHosts = map[string]bool{
	"github.com":   true,
	"gitlab.com":   true,
	"codeberg.org": true,
}

var versionLike = regexp.MustCompile(`^v[0-9]+$`)

var majorSuffix = regexp.MustCompile(`^v([2-9]|[1-9][0-9]+)$`)

// CheckModulePath reports why a module path can't be downloaded, or nil. The
// rules are those of Go for the path of a module you can fetch: ASCII
// letters, digits and "-._~" in elements that do not start or end with a
// dot, and a dot in the first element.
func CheckModulePath(path string) error {
	if msg := gocppmod.CheckPath(path); msg != "" {
		return fmt.Errorf("invalid module path %q: %s", path, msg)
	}
	if len(path) > 512 {
		return fmt.Errorf("invalid module path %q: too long", path)
	}
	elems := strings.Split(path, "/")
	if !strings.Contains(elems[0], ".") {
		return fmt.Errorf("invalid module path %q: the first element must be a host name with a dot (only such modules can be downloaded)", path)
	}
	// Like Go's module.CheckPath: no Windows device names (the path becomes a folder name
	// of the module cache), and a last element /v0, /v1 or /v02 is not a major version suffix (on a git host it is a folder).
	if k := len(elems); k > 1 && !gitHosts[elems[0]] && versionLike.MatchString(elems[k-1]) && !majorSuffix.MatchString(elems[k-1]) {
		return fmt.Errorf("invalid module path %q: %s is not a major version suffix (v1 is the default; suffixes start at /v2)", path, elems[k-1])
	}
	for _, e := range elems {
		base := strings.ToLower(e)
		if i := strings.IndexByte(base, '.'); i >= 0 {
			base = base[:i]
		}
		if reserved[strings.TrimRight(base, " ")] {
			return fmt.Errorf("invalid module path %q: element %q is a reserved device name on Windows", path, e)
		}
		if e == "" || e[0] == '.' || e[len(e)-1] == '.' && e != "" {
			return fmt.Errorf("invalid module path %q: element %q must not start or end with a dot", path, e)
		}
		for i := 0; i < len(e); i++ {
			c := e[i]
			switch {
			case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c == '-', c == '.', c == '_', c == '~':
			default:
				return fmt.Errorf("invalid module path %q: character %q is not allowed (use letters, digits and -._~)", path, rune(c))
			}
		}
	}
	return nil
}

// ParseModulePath finds the repository of a module path. The repository is
// the first element after the host plus one more for the known git hosts
// (github.com/owner/repo), or everything up to an element that ends in .git
// (gitlab.com/group/subgroup/repo.git/sub), and the rest is the folder of the
// module in it.
func ParseModulePath(path string) (*Repo, error) {
	if err := CheckModulePath(path); err != nil {
		return nil, err
	}
	elems := strings.Split(path, "/")
	r := &Repo{Module: path, Host: elems[0]}
	n := 0
	for i := 1; i < len(elems); i++ {
		if strings.HasSuffix(elems[i], ".git") && len(elems[i]) > len(".git") {
			n = i + 1
			break
		}
	}
	if n == 0 {
		n = 3
		if len(elems) < n {
			if gitHosts[r.Host] {
				return nil, fmt.Errorf("module path %q: %s paths are %s/owner/repo", path, r.Host, r.Host)
			}
			n = len(elems)
		}
	}
	rootN := n
	rest := elems[n:]
	// A host that is not a known git host has no fixed repository length: the
	// whole path is the repository, and a last /vN is the major version suffix
	// (Go's module.SplitPathVersion).
	if n == len(elems) && n > 1 && majorSuffix.MatchString(elems[n-1]) && !(gitHosts[r.Host] && n == 3) {
		r.Major, _ = strconv.Atoi(elems[n-1][1:])
		rootN = n - 1
		rest = nil
	}
	r.Root = strings.Join(elems[:rootN], "/")
	if k := len(rest); k > 0 && majorSuffix.MatchString(rest[k-1]) {
		r.Major, _ = strconv.Atoi(rest[k-1][1:])
		rest = rest[:k-1]
	}
	r.Subdir = strings.Join(rest, "/")
	return r, nil
}

// GitName returns the repository path without the host, for a URL: owner/repo
// (with a trailing .git when the module path has one).
func (r *Repo) GitName() string {
	return strings.TrimPrefix(strings.TrimPrefix(r.Root, r.Host), "/")
}

// Tag returns the name of the git tag that has the module's version.
func (r *Repo) Tag(version string) string {
	v := strings.TrimSuffix(version, "+incompatible")
	if r.Subdir != "" {
		return r.Subdir + "/" + v
	}
	return v
}

// CheckVersion reports why version is not a version of the module.
func (r *Repo) CheckVersion(version string) error {
	if !gocppmod.ValidVersion(version) {
		return fmt.Errorf("invalid version %q for %s (want vX.Y.Z)", version, r.Module)
	}
	if strings.Contains(version, "+") && !strings.HasSuffix(version, "+incompatible") {
		return fmt.Errorf("invalid version %q for %s: build metadata is not allowed", version, r.Module)
	}
	if r.Major != 0 && semverMajor(version) != r.Major {
		return fmt.Errorf("version %s does not match the /v%d at the end of the module path %s", version, r.Major, r.Module)
	}
	// Without a /vN suffix, v2 and above are accepted with or without
	// +incompatible (C and C++ libraries tag releases v3.1.0 and have no go.mod:
	// Go would insist on +incompatible, and Versions and Query write it).
	if strings.HasSuffix(version, "+incompatible") && r.Major != 0 {
		return fmt.Errorf("invalid version %s for %s: +incompatible is for a module path with no /v%d suffix", version, r.Module, r.Major)
	}
	return nil
}

func semverMajor(v string) int {
	v = strings.TrimPrefix(v, "v")
	n := strings.IndexAny(v, ".-+")
	if n < 0 {
		n = len(v)
	}
	m, _ := strconv.Atoi(v[:n])
	return m
}

// pseudoRE matches a pseudo-version: the three forms of Go's
// vX.0.0-yyyymmddhhmmss-abcdefabcdef, vX.Y.Z-pre.0.yyyymmddhhmmss-abcdefabcdef
// and vX.Y.(Z+1)-0.yyyymmddhhmmss-abcdefabcdef.
var pseudoRE = regexp.MustCompile(`^v[0-9]+\.(?:0\.0-|[0-9]+\.[0-9]+-(?:[0-9A-Za-z.-]*\.)?0\.)([0-9]{14})-([0-9a-f]{12})(?:\+incompatible)?$`)

// PseudoCommit returns the 12 hex digits of the commit of a pseudo-version
// and its time (yyyymmddhhmmss), or "" if version is not a pseudo-version.
func PseudoCommit(version string) (commit, stamp string) {
	m := pseudoRE.FindStringSubmatch(version)
	if m == nil {
		return "", ""
	}
	return m[2], m[1]
}

// PseudoVersion returns the pseudo-version of a commit made at the time stamp
// (UTC, "20060102150405") for a module of the major version major (0 for a
// path without a /vN suffix): vN.0.0-yyyymmddhhmmss-commit12, as Go names them.
func PseudoVersion(major int, stamp, commit string) string {
	return fmt.Sprintf("v%d.0.0-%s-%s", major, stamp, commit[:12])
}

// escape returns the name of a module path or version in the module cache:
// every upper-case letter is written as ! and the lower-case letter, so that
// two paths that differ in case do not collide on a case-insensitive file
// system. The path must have passed CheckModulePath.
func escape(s string) string {
	hasUpper := false
	for i := 0; i < len(s); i++ {
		if 'A' <= s[i] && s[i] <= 'Z' {
			hasUpper = true
			break
		}
	}
	if !hasUpper {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			b.WriteByte('!')
			c += 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

// parsedVersion is a version split for comparison: only what the rest of the
// package needs of it (gocppmod.CompareVersions does the comparing).
type parsedVersion struct {
	pre []string // dot separated prerelease, nil for a release
}

func parseVersion(v string) (parsedVersion, bool) {
	if !gocppmod.ValidVersion(v) {
		return parsedVersion{}, false
	}
	if gocppmod.IsPrerelease(v) {
		return parsedVersion{pre: []string{"-"}}, true
	}
	return parsedVersion{}, true
}

// CompareVersions compares two valid versions by semantic versioning
// precedence: it returns -1, 0 or 1. Build metadata is ignored.
func CompareVersions(a, b string) int { return gocppmod.CompareVersions(a, b) }
