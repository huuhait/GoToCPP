// Package stdlib embeds gocpp's standard library, the .gp files of this
// folder, in the gocpp binary, and knows the import paths of Go's standard
// packages. It lives in stdlib/ because a //go:embed pattern can't reach a
// parent folder. The loader resolves a standard import path such as "fmt"
// or "encoding/json" in FS.
//
// In Phase 01 the packages are declaration stubs (README.md); the later
// phases replace them with the real packages.
package stdlib

import (
	"embed"
	"io/fs"
	"slices"
	"strings"
)

// The embedded files: each package's .gp files, and Go's LICENSE, because
// the stubs (and later the ported packages) come from Go's source, and its
// license wants its notice to go with every copy. README.md is for people
// who work on gocpp, so it stays out, and so do std.txt and the Go files.
// A package deeper than two folders (math/rand/v2, from Phase 09) needs one
// more pattern, and so does a new kind of file (a native .cpp file);
// TestFSHasEveryFile finds a file that no pattern matches.
//
//go:embed LICENSE */*.gp */*/*.gp */*/*/*.gp
var files embed.FS

// FS returns the standard library: a read-only file system whose root is
// this folder, with each package in the folder of its import path, such as
// fmt/fmt.gp and encoding/json/json.gp. The root also holds LICENSE, Go's
// license. Nothing else is in it.
func FS() fs.FS {
	return files
}

// std.txt lists the import paths of Go's standard packages (see IsStd).
//
//go:embed std.txt
var stdTxt string

// std holds the paths of std.txt, sorted.
var std = parseStd(stdTxt)

// parseStd returns the paths in the text of std.txt: one path on each line,
// without the empty lines and the comment lines, which start with #.
func parseStd(text string) []string {
	var paths []string
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			paths = append(paths, line)
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

// IsStd reports whether path is the import path of one of Go's standard
// packages, as "go list std" of Go 1.25.14 prints them, whether gocpp has
// the package yet or not. So the loader can tell a standard package that
// is not in FS yet ("package regexp is not available yet") from a path
// that is not a standard package at all.
//
// The internal packages are in the list (internal/bytealg,
// crypto/internal/fips140), because they are standard packages: Go's rule
// for internal packages decides who may import them, so the loader applies
// it first ("use of internal package internal/bytealg not allowed") and
// asks IsStd only after that. The vendor/ packages are not in the list:
// the standard packages import them without the vendor/ prefix
// (golang.org/x/net/dns/dnsmessage), and no program can import them.
func IsStd(path string) bool {
	_, found := slices.BinarySearch(std, path)
	return found
}

// Std returns the import paths that IsStd accepts, sorted. The caller may
// change the result.
func Std() []string {
	return slices.Clone(std)
}

// LinuxOnly lists the standard packages that have files for Linux only so far: package os and what
// it stands on (step 9.6). Windows brings its own files in Phase 14 (and macOS in Phase 15), so the
// tests that check every package, or an example, for another system leave these packages out until
// then (their errors for that system are not failures).
var LinuxOnly = []string{"os", "syscall", "internal/poll", "internal/filepathlite"}
