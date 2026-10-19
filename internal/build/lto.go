package build

import (
	"context"
	"slices"
	"strings"
)

// Link-time optimization (Phase 06 step 6.9), opt in (Builder.LTO, gocpp build -lto). g++ gets
// -flto=auto (parallel link-time compiles, no "serial compilation" warning) and clang++
// -flto=thin (ThinLTO: parallel and the cheaper of the two, with the same cross-unit inlining
// for what matters here). The flags go to the compiles of the runtime and of the generated
// C++, and to the link; the C and C++ files of the packages and the libraries of the modules
// compile as before (the linker takes ordinary objects next to LTO ones). They don't change
// -O: LTO helps with the optimization level that GOCPP_CXXFLAGS (or the later -release flag) gives.

// LTOFlags returns the flag that turns on LTO for the compiler c, or nil for a compiler of an
// unknown family.
func LTOFlags(c CXX) []string {
	switch c.Family {
	case GCC:
		return []string{"-flto=auto"}
	case Clang:
		return []string{"-flto=thin"}
	}
	return nil
}

// ltoOn reports whether the objects of this build are LTO objects: -lto, or an -flto flag that the
// user gave (GOCPP_CXXFLAGS, the compiler's own arguments, a mode's flags: `-O2 -flto` of a release
// build). A library of such objects needs the matching archiver, whoever asked for LTO.
func (b *Builder) ltoOn() bool {
	if b.LTO {
		return true
	}
	isLTO := func(a string) bool { return strings.HasPrefix(a, "-flto") || a == "-fuse-linker-plugin" }
	return slices.ContainsFunc(b.CXX.Args, isLTO) || slices.ContainsFunc(b.Env.CXXCompile(), isLTO)
}

// compileFlags are the flags of every compile of the runtime and of the generated C++: CompileFlags,
// and the flag of LTO.
func (b *Builder) compileFlags() []string {
	flags := CompileFlags(b.CXX, b.ASan, b.NoChecks)
	if b.LTO {
		flags = append(flags, LTOFlags(b.CXX)...)
	}
	return flags
}

// linkFlags are the flags of the link that come before the object files: Flags, the flags of the
// target (LinkFlags: a Windows program links statically with an 8 MiB stack), and the flag of LTO
// (the link runs the optimizer).
func (b *Builder) linkFlags() []string {
	flags := slices.Concat(Flags(b.CXX, b.ASan), LinkFlags(b.CXX))
	if b.LTO {
		flags = append(flags, LTOFlags(b.CXX)...)
	}
	return flags
}

// rtArchiver returns the archiver for the runtime library of this build: the one that belongs to
// the compiler (FindArchiver), asked once for each build.
func (b *Builder) rtArchiver(ctx context.Context) (Archiver, error) {
	if b.ar != nil {
		return b.ar.a, b.ar.err
	}
	a, err := FindArchiver(ctx, b.CXX, b.ltoOn())
	if ctx.Err() == nil {
		b.ar = &archiverResult{a, err}
	}
	return a, err
}

type archiverResult struct {
	a   Archiver
	err error
}

// noRuntimeLib reports whether GOCPP_NORTLIB turns the runtime library off: the runtime's
// object files are linked one by one (for comparing link times, and for a tool of the platform
// that can't make a library).
func noRuntimeLib() bool { return EnvOff("GOCPP_NORTLIB") }
