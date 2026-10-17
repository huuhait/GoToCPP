package build

import "slices"

// debugFlags are the flags of every compile in Phase 02 (README §2.7): a
// debug build. -std=c++20 and not gnu++20, so that macros such as linux
// and unix are not predefined. -fwrapv makes signed overflow wrap like
// Go. -O0 is always explicit: a Nix shell's compiler wrapper with
// hardening on makes a missing -O into -O2. (The same wrapper adds
// -D_FORTIFY_SOURCE after these flags, which warns at -O0; the runtime's
// gocpp/nofortify.h, included first by gocpp/rt.hpp, undefines it.)
// -gno-column-info: the debug info gets no columns, because the columns
// would be those of the C++ code, not of the .gp code that #line maps the
// lines to; lldb and editors would point at the wrong place in a .gp line.
// -fno-strict-aliasing: gp::ptr_cast and gp::to_array_ptr (the runtime's
// pointer conversions) access objects through another type, and need it.
var debugFlags = []string{"-std=c++20", "-fwrapv", "-fno-strict-aliasing", "-ffp-contract=off", "-g", "-gno-column-info", "-O0"}

// asanFlags are the flags that -asan adds, to the compiles and to the
// link: AddressSanitizer and UndefinedBehaviorSanitizer, like the
// runtime's own sanitizer tests. With -fno-sanitize-recover=all, the first
// report stops the program with exit status 1 instead of letting it go on.
var asanFlags = []string{"-fsanitize=address,undefined", "-fno-omit-frame-pointer", "-fno-sanitize-recover=all"}

// noChecksFlags are the flags that -nochecks adds to every compile, of the
// generated C++ and of the runtime (Phase 03 §2.9): GOCPP_NOCHECKS turns
// off the runtime's bounds and nil checks (runtime/include/gocpp/checks.hpp)
// and keeps the division and shift checks. The runtime's object files must
// be compiled with it too, as its headers are inline code that every file
// shares (one definition each); they are cached apart, since the flags are
// part of their key (runtimeKey).
var noChecksFlags = []string{"-DGOCPP_NOCHECKS"}

// CompileFlags returns the flags of every compile of a program and of the
// runtime: Flags, and with nochecks the flags of -nochecks.
func CompileFlags(cxx CXX, asan, nochecks bool) []string {
	flags := Flags(cxx, asan)
	if nochecks {
		flags = append(flags, noChecksFlags...)
	}
	return flags
}

// Flags returns the flags of every compile and of the link of a program
// and of the runtime, for the compiler cxx, after cxx.Args: the debug
// flags, the sanitizers with asan, and the flags of the compiler's
// family. For clang only, -Qunused-arguments, so that a Nix wrapper's
// extra flags don't warn at every link (g++ rejects that flag). The
// include folder of the runtime comes on top, for the compiles.
func Flags(cxx CXX, asan bool) []string {
	flags := slices.Clone(debugFlags)
	if asan {
		flags = append(flags, asanFlags...)
	}
	if cxx.Family == Clang {
		flags = append(flags, "-Qunused-arguments")
	}
	return flags
}

// GeneratedFlags returns the flags that a compile of generated C++ (the
// emitter's output) adds to Flags, by the family of cxx. Valid Go can make
// the compilers warn about code whose behavior is right (with -fwrapv):
// clang's -Wtautological-compare for x == x on integers, and g++'s
// -Woverflow when it folds a part of an expression such as
// 65535 * uint16(x | -1). Those two are turned off, each only for the
// family that has it; every other warning stays visible. The runtime's
// own files don't get them. The emitter's tests compile with the same
// flags (internal/cppgen, cxx_test.go).
//
// g++ also gets -Werror=conditionally-supported: a class object passed
// through a C function's `...` (a gp::ptr, or the gp::cstr holder given to
// printf or ioctl) compiles silently with g++ otherwise, and the callee
// reads the address of a copy of the object as its pointer. clang rejects
// it on its own (-Wnon-pod-varargs is an error by default).
// runtime/reject_test.go checks both.
//
// And g++ gets -Werror=narrowing: a narrowing conversion in a braced list (a gocpp int64_t into
// a C int field of a literal, lib.Point{1, n}, Phase 04 §2.5) is an error in C++, which clang
// reports, while g++ only warns for a value that is not a constant. Both compilers then reject
// the same programs.
func GeneratedFlags(cxx CXX) []string {
	switch cxx.Family {
	case Clang:
		return []string{"-Wno-tautological-compare"}
	case GCC:
		return []string{"-Wno-overflow", "-Werror=conditionally-supported", "-Werror=narrowing"}
	}
	return nil
}
