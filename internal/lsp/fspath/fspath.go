// Package fspath is how the language server writes, compares and canonicalizes file names (Phase 14 step 14.5,
// ADR-096). The server runs on systems whose paths look different (`/home/me/p` or `C:\Users\Me\p`, a UNC
// name `\\host\share\p`) and compare names differently (a Windows file system is case-insensitive, so
// `c:\users\me\P` and `C:\Users\Me\p` are one file). The cache, the open documents and the answers are all
// keyed by names, so one spelling of a file per server is the rule: a name from the editor goes through
// Canon before anything uses it as a key.
//
// The system a name is written for is a Flavor, and it can be replaced (Use), so that the logic of a Windows
// server is tested on Linux, with a fake file system that has Windows rules (SetLister). Three flavors exist:
//
//   - Posix: slashes, case-sensitive. Canon changes nothing.
//   - Windows: drive letters and UNC names, both kinds of slash, case-insensitive. Canon makes the name clean
//     (`C:\a\b`), puts the drive letter in upper case, and gives every part that exists on disk the case it
//     has there (a name from an editor often has the case that the user typed when the folder was opened).
//   - CaseFold: slashes but case-insensitive (a Linux folder on a case-insensitive file system, the tests).
//
// Not done: short 8.3 names (PROGRA~1), symbolic links and junctions (two names of one file through a link are
// two files here, as in gopls), and the Win32 rule that trailing dots and spaces of a name are dropped.
package fspath

import (
	"path"
	"runtime"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

// A Flavor is a way of writing and comparing file names.
type Flavor struct {
	// Windows: drive letters (`C:`), UNC names (`\\host\share`), `\` as the separator (`/` is accepted too).
	Windows bool
	// FoldCase: names that differ in letter case only are the same file.
	FoldCase bool
}

// The flavors.
var (
	Posix    = Flavor{}
	Windows  = Flavor{Windows: true, FoldCase: true}
	CaseFold = Flavor{FoldCase: true}
)

var current atomic.Pointer[Flavor]

func init() {
	f := Posix
	if runtime.GOOS == "windows" {
		f = Windows
	}
	current.Store(&f)
}

// Current returns the flavor of the system the server runs on, or the one a test has put in with Use.
func Current() Flavor { return *current.Load() }

// Use makes f the current flavor and returns the function that puts the old one back. It is for tests; the
// tests that use it must not run in parallel with others of the process.
func Use(f Flavor) (restore func()) {
	old := current.Swap(&f)
	Invalidate()
	return func() {
		current.Store(old)
		Invalidate()
	}
}

// Sep returns the separator of the flavor.
func (f Flavor) Sep() byte {
	if f.Windows {
		return '\\'
	}
	return '/'
}

func (f Flavor) isSep(c byte) bool { return c == '/' || f.Windows && c == '\\' }

// ToSlash returns p with slashes. FromSlash is the opposite.
func (f Flavor) ToSlash(p string) string {
	if f.Windows {
		return strings.ReplaceAll(p, `\`, "/")
	}
	return p
}

// FromSlash returns p with the separators of the flavor.
func (f Flavor) FromSlash(p string) string {
	if f.Windows {
		return strings.ReplaceAll(p, "/", `\`)
	}
	return p
}

// volumeLen returns the length of the volume at the start of p: `C:` or `\\host\share` on Windows, 0 elsewhere
// and for a name without one. p has no `\\?\` prefix (Clean takes it off first).
func (f Flavor) volumeLen(p string) int {
	if !f.Windows {
		return 0
	}
	if len(p) >= 2 && p[1] == ':' && isLetter(p[0]) {
		return 2
	}
	// \\host\share: two separators, a host, a separator, a share.
	if len(p) >= 5 && f.isSep(p[0]) && f.isSep(p[1]) && !f.isSep(p[2]) && p[2] != '.' {
		n := 2
		for n < len(p) && !f.isSep(p[n]) {
			n++
		}
		if n+1 >= len(p) || !f.isSep(p[n]) || f.isSep(p[n+1]) {
			return 0
		}
		n++
		for n < len(p) && !f.isSep(p[n]) {
			n++
		}
		return n
	}
	return 0
}

func isLetter(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

// Volume returns the volume of p: `C:` or `\\host\share`, or "".
func (f Flavor) Volume(p string) string {
	p = f.stripPrefix(p)
	return strings.ReplaceAll(p[:f.volumeLen(p)], "/", `\`)
}

// stripPrefix takes the Win32 file namespace prefix off a Windows name: `\\?\C:\x` is `C:\x` and
// `\\?\UNC\host\share\x` is `\\host\share\x`.
func (f Flavor) stripPrefix(p string) string {
	if !f.Windows || len(p) < 4 || !f.isSep(p[0]) || !f.isSep(p[1]) || p[2] != '?' || !f.isSep(p[3]) {
		return p
	}
	p = p[4:]
	if len(p) >= 4 && strings.EqualFold(p[:3], "UNC") && f.isSep(p[3]) {
		return `\\` + p[4:]
	}
	return p
}

// Clean returns the shortest equivalent name: no `.` or `..` elements, no doubled or trailing separators, and (on
// Windows) the backslash as separator and no `\\?\` prefix. An empty name stays ".".
func (f Flavor) Clean(p string) string {
	if !f.Windows {
		return path.Clean(p)
	}
	p = f.stripPrefix(p)
	vl := f.volumeLen(p)
	vol, rest := strings.ReplaceAll(p[:vl], "/", `\`), p[vl:]
	c := path.Clean(f.ToSlash(rest)) // a rooted name stays rooted, and ".." above the root is dropped
	if c == "." && !(rest != "" && f.isSep(rest[0])) {
		switch {
		case vol == "":
			return "."
		case vol[0] == '\\': // \\host\share is the root of the share, written without a separator as path/filepath does
			return vol
		}
		return vol + "." // `C:.` is the current folder of the drive, as filepath.Clean says
	}
	if vol == "" && len(c) >= 2 && c[1] == ':' && isLetter(c[0]) {
		c = "./" + c // "./c:" must not become the drive c:, as path/filepath has it
	}
	return vol + f.FromSlash(c)
}

// IsAbs reports whether p names a place by itself: `C:\x` or `\\host\share\x` on Windows, `/x` elsewhere.
func (f Flavor) IsAbs(p string) bool {
	if !f.Windows {
		return strings.HasPrefix(p, "/")
	}
	p = f.stripPrefix(p)
	vl := f.volumeLen(p)
	if vl == 0 {
		return false
	}
	if p[0] != '\\' && p[0] != '/' { // a drive: needs a separator after it
		return len(p) > vl && f.isSep(p[vl])
	}
	return true // a UNC name
}

// Dir returns the folder of p, as path/filepath.Dir does.
func (f Flavor) Dir(p string) string {
	if !f.Windows {
		return path.Dir(p)
	}
	p = f.stripPrefix(p)
	vl := f.volumeLen(p)
	i := len(p) - 1
	for i >= vl && !f.isSep(p[i]) {
		i--
	}
	dir := f.Clean(p[vl : i+1])
	if dir == "." && vl > 2 { // a UNC name: Dir of the share is the share, as path/filepath has it
		return strings.ReplaceAll(p[:vl], "/", `\`)
	}
	return strings.ReplaceAll(p[:vl], "/", `\`) + dir
}

// Base returns the last element of p, as path/filepath.Base does.
func (f Flavor) Base(p string) string {
	if !f.Windows {
		return path.Base(p)
	}
	orig := p
	p = f.stripPrefix(p)
	p = p[f.volumeLen(p):]
	for len(p) > 0 && f.isSep(p[len(p)-1]) {
		p = p[:len(p)-1]
	}
	if p == "" {
		if orig == "" {
			return "."
		}
		return string(f.Sep())
	}
	i := len(p) - 1
	for i >= 0 && !f.isSep(p[i]) {
		i--
	}
	return p[i+1:]
}

// Join joins the elements with the separator and cleans the result; empty elements are dropped.
func (f Flavor) Join(elem ...string) string {
	if f.Windows && len(elem) > 0 && len(elem[0]) == 2 && elem[0][1] == ':' && isLetter(elem[0][0]) {
		// A drive with no separator after it stays relative to the folder of the drive: Join("C:", "a") is "C:a".
		i := 1
		for i < len(elem) && elem[i] == "" {
			i++
		}
		return f.Clean(elem[0] + strings.Join(elem[i:], `\`))
	}
	var parts []string
	for _, e := range elem {
		if e != "" {
			parts = append(parts, e)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return f.Clean(strings.Join(parts, string(f.Sep())))
}

// fold returns s in the form that names are compared in, when the flavor folds case: the characters of the name
// in upper case, then in lower case (NTFS compares by its table of upper-case letters; the difference to
// this rule is a few letters of no file name).
func fold(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return strings.ToLower(s)
	}
	return strings.Map(func(r rune) rune { return unicode.ToLower(unicode.ToUpper(r)) }, s)
}

// Key returns what two names have in common when they name the same file: the cleaned name, folded to
// one case on a flavor that folds. It is for map keys and comparisons, never for showing or opening a file.
func (f Flavor) Key(p string) string {
	if !f.FoldCase {
		return p
	}
	if f.Windows || f.Clean(p) != p {
		p = f.Clean(p)
	}
	return fold(p)
}

// Equal reports whether a and b name the same file by their spelling.
func (f Flavor) Equal(a, b string) bool {
	if a == b {
		return true
	}
	return f.FoldCase && f.Key(a) == f.Key(b)
}

// Within reports whether file is dir or below it, by whole names (`/a/bc` is not in `/a/b`).
func (f Flavor) Within(dir, file string) bool {
	d, p := f.Key(f.Clean(dir)), f.Key(f.Clean(file))
	if d == p {
		return true
	}
	sep := string(f.Sep())
	if !strings.HasSuffix(d, sep) {
		d += sep
	}
	return strings.HasPrefix(p, d)
}

// HasPrefixDir is Within for names that are known to be clean and that end the folder with a separator: the
// test of the changed folders of a snapshot.
func (f Flavor) HasPrefixDir(file, dirWithSep string) bool {
	if !f.FoldCase {
		return strings.HasPrefix(file, dirWithSep)
	}
	return len(file) >= len(dirWithSep) && strings.HasPrefix(fold(file), fold(dirWithSep))
}

// Package-level forms, in the current flavor.

func Sep() byte                        { return Current().Sep() }
func Clean(p string) string            { return Current().Clean(p) }
func Dir(p string) string              { return Current().Dir(p) }
func Base(p string) string             { return Current().Base(p) }
func Join(elem ...string) string       { return Current().Join(elem...) }
func Key(p string) string              { return Current().Key(p) }
func Equal(a, b string) bool           { return Current().Equal(a, b) }
func Within(dir, file string) bool     { return Current().Within(dir, file) }
func HasPrefixDir(file, d string) bool { return Current().HasPrefixDir(file, d) }
func IsAbs(p string) bool              { return Current().IsAbs(p) }

// FoldCase reports whether the current flavor compares names without case. A caller that would do work
// for the other flavors only (a second map) asks first.
func FoldCase() bool { return Current().FoldCase }
