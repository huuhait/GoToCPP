package main

import (
	"fmt"
	"go/build/constraint"
	"slices"
	"strings"
)

// Build constraints of the files that are kept.
//
// The tool picks the files of a package for a target (select.go), so a kept
// file's //go:build line has already been decided. But gocpp's loader reads
// the line again when it builds a program (internal/load/resolve.go), for
// the machine it builds for, and with its own tags (no purego, no gc). A
// line that says "!purego", or "amd64", would then drop a file that the tool
// chose, or keep one that it did not. So the tool rewrites the line to what
// is left of it when everything but the operating system is decided:
//
//	//go:build !purego && (linux || darwin)    ->  //go:build linux || darwin
//	//go:build !amd64 && !arm64                ->  (no line)
//
// The operating system stays, because Phase 14 ports the same package for
// Windows into the same folder, and the loader has to tell the files of the
// two systems apart.

var knownOS = []string{"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "js", "linux",
	"nacl", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows", "zos"}

// unixOS are the systems for which the tag "unix" holds (go/build's list).
var unixOS = []string{"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "linux",
	"netbsd", "openbsd", "solaris"}

// osImplies are the tags that a system satisfies besides its own name.
var osImplies = map[string][]string{
	"android": {"linux"},
	"illumos": {"solaris"},
	"ios":     {"darwin"},
}

// isOSTag reports whether tag is decided by the operating system (kept in
// the rewritten line).
func isOSTag(tag string) bool {
	return tag == "unix" || slices.Contains(knownOS, tag)
}

// osTagHolds reports whether the tag is true on the system goos.
func osTagHolds(tag, goos string) bool {
	switch {
	case tag == goos:
		return true
	case tag == "unix":
		return slices.Contains(unixOS, goos)
	}
	return slices.Contains(osImplies[goos], tag)
}

// tagEnv decides the tags that are not about the operating system.
type tagEnv struct {
	arch    string
	tags    []string // extra tags that are true (purego)
	release int      // go1.1 to go1.<release> are true
}

// holds reports whether the tag is true.
func (e tagEnv) holds(tag string) bool {
	switch {
	case tag == e.arch:
		return true
	case tag == "gc":
		return true
	case slices.Contains(e.tags, tag):
		return true
	case strings.HasPrefix(tag, "go1."):
		var minor int
		if _, err := fmt.Sscanf(tag, "go1.%d", &minor); err == nil && fmt.Sprintf("go1.%d", minor) == tag {
			return minor <= e.release
		}
	}
	return false // cgo, race, msan, asan, goexperiment.*, ignore, other architectures, ...
}

// simplify decides the tags of x that are not about the operating system and
// returns what is left: a constant when the whole expression is decided
// (const is then true or false and rest is nil), else the rest.
func (e tagEnv) simplify(x constraint.Expr) (rest constraint.Expr, val bool) {
	switch x := x.(type) {
	case *constraint.TagExpr:
		if isOSTag(x.Tag) {
			return x, false
		}
		return nil, e.holds(x.Tag)
	case *constraint.NotExpr:
		r, v := e.simplify(x.X)
		if r == nil {
			return nil, !v
		}
		return &constraint.NotExpr{X: r}, false
	case *constraint.AndExpr:
		return e.fold(x.X, x.Y, true)
	case *constraint.OrExpr:
		return e.fold(x.X, x.Y, false)
	}
	panic(fmt.Sprintf("unknown constraint expression %T", x))
}

// fold simplifies a && b (isAnd) or a || b.
func (e tagEnv) fold(a, b constraint.Expr, isAnd bool) (constraint.Expr, bool) {
	ra, va := e.simplify(a)
	rb, vb := e.simplify(b)
	// The value that decides the whole: false for and, true for or.
	absorbing := !isAnd
	switch {
	case ra == nil && va == absorbing, rb == nil && vb == absorbing:
		return nil, absorbing
	case ra == nil && rb == nil:
		return nil, !absorbing // both decided, neither absorbs: true for and, false for or
	case ra == nil:
		return rb, false
	case rb == nil:
		return ra, false
	}
	if isAnd {
		return &constraint.AndExpr{X: ra, Y: rb}, false
	}
	return &constraint.OrExpr{X: ra, Y: rb}, false
}

// evalOS evaluates an expression that has only operating system tags, for
// the system goos. (The tool uses it as a check: a kept file's rewritten line
// must hold for the systems the file was kept for.)
func evalOS(x constraint.Expr, goos string) bool {
	return x.Eval(func(tag string) bool { return osTagHolds(tag, goos) })
}

// evalAll evaluates the original expression with the full rules, for the
// system goos: what go/build decides.
func (e tagEnv) evalAll(x constraint.Expr, goos string) bool {
	return x.Eval(func(tag string) bool {
		if isOSTag(tag) {
			return osTagHolds(tag, goos)
		}
		return e.holds(tag)
	})
}

// rewriteBuildLine returns the new text of a //go:build line, or "" when the
// line goes away because nothing is left of it. goos lists the systems the
// file was kept for; the original line must hold on each of them (a check of
// the tool itself), and so must what is left of it.
func (e tagEnv) rewriteBuildLine(line string, goos []string) (string, error) {
	x, err := constraint.Parse(line)
	if err != nil {
		return "", err
	}
	for _, os := range goos {
		if !e.evalAll(x, os) {
			return "", fmt.Errorf("internal error: %q is false for GOOS=%s, but the file was kept", line, os)
		}
	}
	rest, val := e.simplify(x)
	if rest == nil {
		if !val {
			return "", fmt.Errorf("internal error: %q is false, but the file was kept", line)
		}
		return "", nil
	}
	for _, os := range goos {
		if !evalOS(rest, os) {
			return "", fmt.Errorf("internal error: the rewrite %q of %q is false for GOOS=%s", rest, line, os)
		}
	}
	return "//go:build " + rest.String(), nil
}
