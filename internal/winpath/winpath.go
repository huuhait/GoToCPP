// Package winpath answers one question of the Windows build driver (Phase 14 step 14.2, ADR-081): can the
// compilers be handed this path? MinGW-w64's g++, ld, as and cc1plus are C programs that read their
// command line in the ANSI code page (CP_ACP: 1252 on most Western systems), unless the system has been
// switched to UTF-8. A path with a character the code page does not have (a user called 日本, a folder with an
// emoji) reaches them as question marks, and nothing is found. Go's own file functions use the wide
// API and have no such limit, so gocpp can read and write the path itself; it only has to keep such a
// path out of the compilers' arguments. Short (8.3) names are ASCII and name the same file, when the
// volume makes them (it usually does for the system volume).
package winpath

import (
	"runtime"
	"strings"
)

// The facts behind the functions; the tests of this and other packages replace them to run the logic on any system.
var (
	// Enabled says that the paths must be made safe: the system is Windows. Elsewhere every function is the identity.
	Enabled = runtime.GOOS == "windows"
	// SafeFunc is ACPSafe, ShortFunc is Short.
	SafeFunc  = acpSafe
	ShortFunc = short
)

// ACPSafe reports whether s can be passed to a program that reads its arguments in the ANSI code page: every character
// has a counterpart there. It is always true on systems other than Windows.
func ACPSafe(s string) bool { return !Enabled || SafeFunc(s) }

// Short returns the 8.3 short name of the existing file or folder p, and whether there is one that is ACPSafe.
// It is false on systems other than Windows, when the volume has no short names, and when p does not exist.
func Short(p string) (string, bool) {
	if !Enabled {
		return "", false
	}
	return ShortFunc(p)
}

// Compiler returns the path p in a form the compilers can read: p itself when it is ACPSafe, else p with its longest
// existing prefix replaced by the short name (the part that does not exist yet stays, so a name for a file to be made
// must be safe already). When there is no such form, p comes back, and the compiler says what it cannot open.
func Compiler(p string) string {
	if ACPSafe(p) {
		return p
	}
	for prefix := p; ; {
		if s, ok := Short(prefix); ok {
			if out := s + p[len(prefix):]; ACPSafe(out) {
				return out
			}
			return p
		}
		cut := strings.LastIndexAny(prefix, `\/`)
		if cut <= 2 {
			return p
		}
		prefix = prefix[:cut]
	}
}

// IncludeFile returns the path p of a header for an #include line in a source file that a compiler reads: the
// compiler opens the name in the line as bytes in the ANSI code page, so a path with a character outside it
// is written as its short name (with slashes), as Compiler gives it. A path that is safe, or has no short
// name, is returned as it is.
func IncludeFile(p string) string {
	if ACPSafe(p) {
		return p
	}
	if m := Compiler(p); m != p {
		return strings.ReplaceAll(m, `\`, "/")
	}
	return p
}

// IncludeSpelling is IncludeFile for the spelling of an include as the loader and the probes carry it: a quoted
// path ("C:/dir/x.h") is mapped inside its quotes; <system> headers and anything else stay as they are.
func IncludeSpelling(h string) string {
	if len(h) >= 2 && h[0] == '"' && h[len(h)-1] == '"' {
		if inner := h[1 : len(h)-1]; !ACPSafe(inner) {
			return `"` + IncludeFile(inner) + `"`
		}
	}
	return h
}
