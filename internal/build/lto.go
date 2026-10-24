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
	if bigObj(b.CXX) && !b.ltoOn() {
		flags = append(flags, "-Wa,-mbig-obj")
	}
	flags = append(flags, b.noICF(b.Env.CXXCompile())...)
	if b.CXX.Family == Clang && lineTablesOnly(slices.Concat(b.CXX.Args, b.Env.CXXCompile())) {
		// clang's line tables name an inlined function by its short name only ("boom"); with the
		// linkage names the runtime's trace shows an inlined Go function as Go does (main.boom(...)).
		// g++'s -g1 has them already (ADR-136, the amendment of 2026-10-23: -release -trace).
		flags = append(flags, "-fdebug-info-for-profiling")
	}
	return flags
}

// lineTablesOnly reports whether the last -g flag of flags asks for the line tables only: -g1,
// -gline-tables-only or -gmlt (the -gno-... and -gdwarf-N forms don't set the level).
func lineTablesOnly(flags []string) bool {
	only := false
	for _, f := range flags {
		switch {
		case f == "-g1" || f == "-gline-tables-only" || f == "-gmlt":
			only = true
		case f == "-g" || f == "-g0" || f == "-g2" || f == "-g3" || f == "-ggdb" || f == "-glldb" ||
			strings.HasPrefix(f, "-ggdb") || f == "-gline-directives-only":
			only = false
		}
	}
	return only
}

// bigObj reports whether the compiles for c need -Wa,-mbig-obj (before LTO is considered). A COFF
// object holds 32767 sections, and every inline function and variable of a header is a section of
// its own (-ffunction-sections): the file of a generics-heavy program (a few dozen instances with
// type switches and descriptors) goes over it, and GNU as stops with "too many sections". The
// big-object format has no such limit. Only GCC's MinGW needs the flag (LLVM's assembler switches
// to the format by itself), and it must not be given for LTO: GCC's linker plugin does not claim
// a big-object file ("plugin needed to handle lto object"), and LTO objects hold GIMPLE, so they
// don't have the sections.
func bigObj(c CXX) bool { return c.GOOS == "windows" && c.Family == GCC }

// linkFlags are the flags of the link that come before the object files: Flags, the flags of the
// target (LinkFlags: a Windows program links statically with a 1 GiB stack), and the flag of LTO
// (the link runs the optimizer).
func (b *Builder) linkFlags() []string {
	flags := slices.Concat(Flags(b.CXX, b.ASan), LinkFlags(b.CXX))
	if b.LTO {
		flags = append(flags, LTOFlags(b.CXX)...)
	}
	return append(flags, b.noICF(b.Env.LDFlags)...) // -flto: the link optimizes too
}

// noICF returns -fno-ipa-icf for an optimized g++ build (the last -O flag of the compiler's own
// arguments and of flags is not -O0): g++'s -O2 folds identical functions into one (-fipa-icf),
// so a panic's trace, runtime.Caller and FuncForPC would name the function that was kept, at its
// line, for the other (two Go functions with the same body). clang folds nothing without the
// linker's --icf, which gocpp never passes.
func (b *Builder) noICF(flags []string) []string {
	if b.CXX.Family != GCC || !optimized(slices.Concat(b.CXX.Args, flags)) {
		return nil
	}
	return []string{"-fno-ipa-icf"}
}

// optimized reports whether the last -O flag of flags asks for optimization (-O1, -O2, -O3, -Os,
// -Oz, -Ofast, -Og: not -O0; no -O flag is -O0).
func optimized(flags []string) bool {
	on := false
	for _, f := range flags {
		if strings.HasPrefix(f, "-O") {
			on = f != "-O0"
		}
	}
	return on
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
