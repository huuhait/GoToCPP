package build

import (
	"strings"

	"github.com/huuhait/gocpp/internal/winpath"
)

// Paths in the compilers' arguments on Windows (Phase 14 step 14.2, ADR-081). MinGW-w64's programs read
// their command line in the ANSI code page, so a path with a character outside it (a user folder
// 日本, a project in a folder with an emoji) arrives as question marks. gocpp, which uses the wide API, can
// read the path; the compiler cannot. CompilerArg turns such a path into its 8.3 short name (ASCII, the
// same file) when the volume has one, so the folders of a user's own C and C++ files and headers work.
// The folders gocpp itself makes (the cache, the work folder) are kept ASCII-safe at the start of the
// process (cmd/gocpp, envpaths_windows.go), and the names it chooses (the program's file in the work
// folder) are ASCII. A path that has no safe form is passed as it is: the compiler then says what
// it cannot open, with the path in the message.

// CompilerSafe reports whether the name or path s can be an argument of the compiler as it is.
func CompilerSafe(s string) bool { return winpath.ACPSafe(s) }

// CompilerArg returns the argument a for the compiler: unchanged when it is safe, else with each path in it
// (a drive letter, a colon and a separator start one; "=" and ";" end one, as in -ffile-prefix-map=a=b)
// replaced by its short name. The existing part of a path is shortened and the rest kept.
func CompilerArg(a string) string {
	if winpath.ACPSafe(a) {
		return a
	}
	var out strings.Builder
	for i := 0; i < len(a); {
		start := drivePath(a, i)
		if start < 0 {
			out.WriteString(a[i:])
			break
		}
		out.WriteString(a[i:start])
		end := start
		for end < len(a) && a[end] != '=' && a[end] != ';' {
			end++
		}
		out.WriteString(winpath.Compiler(a[start:end]))
		i = end
	}
	return out.String()
}

// drivePath returns the index of the next "X:\" or "X:/" in a at or after i, or -1.
func drivePath(a string, i int) int {
	for ; i+2 < len(a); i++ {
		c := a[i]
		if (c|0x20) >= 'a' && (c|0x20) <= 'z' && a[i+1] == ':' && (a[i+2] == '\\' || a[i+2] == '/') {
			return i
		}
	}
	return -1
}

// readsUTF8Arguments says whether the compiler c reads its command line as Unicode, so that it needs no mapping: clang for
// MinGW (llvm-mingw, MSYS2's clang64) converts the wide command line to UTF-8, and GCC 14 and later do the same in the driver
// and cc1. An older GCC (and a compiler whose version is not known) reads the ANSI code page. (Measured on the PC with
// WinLibs' GCC 16.1: a source and an -I folder named 日本 work as they are; the #include text that gocpp writes still needs
// the short name, see winpath.IncludeFile, and so does every file that `as` and `ld` create or open in the cache or the work
// folder, which are kept safe.)
func (c CXX) readsUTF8Arguments() bool {
	return c.Family == Clang || c.Family == GCC && c.Major >= 14
}

// compilerArgs maps every argument (CompilerArg) for the compiler c, when c needs it.
func (c CXX) compilerArgs(args []string) []string {
	if !winpath.Enabled || c.readsUTF8Arguments() {
		return args
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = CompilerArg(a)
	}
	return out
}
