package build

import (
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/load"
)

// The Win32 defaults (Phase 14 step 14.4, ADR-082). <windows.h> is a header package like any
// other, but with its defaults it breaks C++ code: it defines the macros min and max (they
// break std::min and std::max), it pulls in winsock.h, which clashes with winsock2.h, and it
// declares the A (ANSI) functions behind names like MessageBox unless UNICODE is defined. A
// package for windows that imports a header or has C or C++ files of its own is therefore
// compiled, and its headers are probed, with
//
//	-DWIN32_LEAN_AND_MEAN -DNOMINMAX -DUNICODE -D_UNICODE
//
// (Win32Defaults), as Go's own Windows code uses the W functions. Only that package's compiles
// get them (the generated C++, the probes and the package's own C and C++ files): not the
// runtime, nor the sources of a library with a cxx recipe, which have their own flags.
//
// A user changes them, one macro at a time, in any of these ways:
//   - //gocpp:no-win32-defaults in the package: none of the four;
//   - -DNAME, -DNAME=v or -UNAME in //gocpp:cflags, in pkg-config's flags, in the flags of a cxx
//     recipe, or in GOCPP_CFLAGS / GOCPP_CXXFLAGS: gocpp then leaves that macro to the user.

// win32Macros are the macros of the Win32 defaults, in the order they are given.
var win32Macros = []string{"WIN32_LEAN_AND_MEAN", "NOMINMAX", "UNICODE", "_UNICODE"}

// Win32Defaults returns the flags of the Win32 defaults for the package lp, or nil: the package is
// not for windows, imports no header and has no C or C++ file, has //gocpp:no-win32-defaults, or
// the user's flags (the package's compile flags, and GOCPP_CFLAGS and GOCPP_CXXFLAGS) already
// define or undefine the macro. user are the package's compile flags so far.
func Win32Defaults(lp *load.Package, user []string) []string {
	if lp.GOOS != "windows" || len(lp.CFiles)+len(lp.CXXFiles) == 0 && !importsHeader(lp) {
		return nil
	}
	for _, f := range lp.Files {
		for _, d := range f.Directives {
			if d.Name == "no-win32-defaults" {
				return nil
			}
		}
	}
	env, _ := ReadEnvFlags() // a bad quote is reported where the flags are used
	mentioned := macrosOf(slices.Concat(env.CFlags, env.CXXFlags, user))
	var out []string
	for _, m := range win32Macros {
		if !mentioned[m] {
			out = append(out, "-D"+m)
		}
	}
	return out
}

// importsHeader reports whether a file of lp imports a header (import "<windows.h>").
func importsHeader(lp *load.Package) bool {
	for _, f := range lp.Files {
		if f.Info != nil && len(f.Info.Includes) > 0 {
			return true
		}
	}
	return false
}

// macrosOf returns the names that flags define or undefine: -DNAME, -DNAME=v, -UNAME, and the
// forms with the name in the next word ("-D", "NAME").
func macrosOf(flags []string) map[string]bool {
	m := make(map[string]bool)
	for i, f := range flags {
		var name string
		switch {
		case (f == "-D" || f == "-U") && i+1 < len(flags):
			name = flags[i+1]
		case strings.HasPrefix(f, "-D") || strings.HasPrefix(f, "-U"):
			name = f[2:]
		default:
			continue
		}
		name, _, _ = strings.Cut(name, "=")
		m[name] = true
	}
	return m
}
