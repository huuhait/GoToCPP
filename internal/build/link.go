package build

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

// Building a program of several packages (Phase 06 step 6.2): Link compiles the translation
// units of all the packages, the packages' own C and C++ files and the C and C++ libraries of the
// modules, and links their objects with the runtime. The action graph with content hashing,
// the -p flag and the other refinements are steps 6.5 to 6.9; this is the simple, correct
// version of it: every unit goes through the object cache (objcache.go), which keys it by its text,
// the flags, the compiler and every header the compile read, so a program in which nothing
// changed compiles nothing, and a changed .gp file recompiles its own unit and the units of the
// packages that include a header that changed.

// A PackageBuild is one package of a program for Link.
type PackageBuild struct {
	// Path is the import path of the package, for messages.
	Path string

	// Dir is the package's folder (absolute), where its CFiles and CXXFiles are; Flags are the
	// package's flags (check.Package.Flags): Compile goes to its compiles, after the
	// environment's, and Link to the link.
	Dir   string
	Flags PackageFlags

	// Public are the compile flags of the packages whose headers this package includes and that
	// those headers need (the public flags of the imported packages: a macro or a -I that
	// changes how a header parses must be the same in every translation unit that includes
	// it). They come after Flags.Compile.
	Public []string

	// CFiles and CXXFiles are the names of the package's C and C++ files in Dir.
	CFiles, CXXFiles []string

	// QuoteDir, if not empty, is the folder of the package's local headers ("./physics.h"): the
	// compiles get -iquote QuoteDir.
	QuoteDir string

	// Units are the translation units of the package that gocpp generated (absolute names).
	Units []string

	// SourceWarnings, if not nil, gets what the compiler printed for a C or C++ file of the
	// package that compiled, instead of Builder.Stderr.
	SourceWarnings func(out []byte)
}

// maxUnitJobs is how many translation units compile at the same time.
var maxUnitJobs = max(1, min(runtime.NumCPU(), 16))

// Link compiles the packages and links the program out. The objects of the packages come in the
// order of pkgs; a compile or link that fails is a *CompileError (PkgPath says the package of a
// unit or a source that failed). The generated units may include each other's headers through
// GenRoot, the folder that holds them.
func (b *Builder) Link(ctx context.Context, pkgs []PackageBuild, out string) error {
	// With flags of the environment, the runtime's compiles get gocpp's language flags after
	// them too (as the generated C++ does), then -Wno-error (valid code must not fail for a
	// warning that a user's -Werror -Wshadow turns into an error) and -x c++, which comes right
	// before each runtime source. All of them are part of the key of the runtime's object
	// files.
	flags := CompileFlags(b.CXX, b.ASan, b.NoChecks)
	if env := b.Env.CXXCompile(); len(env) > 0 {
		flags = slices.Concat(flags, env, LanguageFlags(), []string{"-Wno-error", "-x", "c++"})
	}
	defer b.waitPCH() // a header that is still being built is finished, not left half done
	defer b.stores.Wait()
	rtDir, rtObjs, rtErr := b.runtime(ctx, flags)
	if rtErr != nil {
		return rtErr
	}

	// The libraries of the modules: once for the program, whatever packages import their headers.
	var libs []*CxxLib
	for _, pb := range pkgs {
		for _, lib := range pb.Flags.Libs {
			if !slices.ContainsFunc(libs, func(l *CxxLib) bool { return sameLib(l, lib) }) {
				libs = append(libs, lib)
			}
		}
	}

	// The package's own C and C++ files compile at the same time as the units. An error in
	// them is reported first: it is the user's.
	srcObjs := make([][]string, len(pkgs))
	srcErrs := make([]error, len(pkgs))
	var wg sync.WaitGroup
	for i := range pkgs {
		if len(pkgs[i].CFiles)+len(pkgs[i].CXXFiles) == 0 {
			continue
		}
		wg.Go(func() { srcObjs[i], srcErrs[i] = b.compilePackageSources(ctx, rtDir, &pkgs[i], i, len(pkgs) > 1) })
	}
	var libObjs []string
	var libErr error
	wg.Go(func() { libObjs, libErr = b.compileLibsOf(ctx, rtDir, libs) })

	// The generated units.
	type unit struct {
		pkg int
		src string
		obj string
	}
	var units []unit
	for i, pb := range pkgs {
		for _, src := range pb.Units {
			obj := filepath.Join(b.Work, strings.TrimSuffix(filepath.Base(src), ".cpp")+".o")
			if len(pkgs) > 1 || len(pb.Units) > 1 {
				obj = filepath.Join(b.Work, fmt.Sprintf("%02d-%s.o", i, strings.TrimSuffix(filepath.Base(src), ".cpp")))
			}
			units = append(units, unit{i, src, obj})
		}
	}
	unitErr := make([]error, len(units))
	sem := make(chan struct{}, maxUnitJobs)
	for k, u := range units {
		pb := &pkgs[u.pkg]
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			_, unitErr[k] = b.compileUnit(ctx, rtDir, pb, u.src, u.obj)
		})
	}
	wg.Wait()
	for i, err := range srcErrs {
		if err != nil {
			var ce *CompileError
			if errors.As(err, &ce) {
				ce.PkgPath = pkgs[i].Path
			}
			return err
		}
	}
	if libErr != nil {
		return libErr
	}
	for k, err := range unitErr {
		if err != nil {
			var ce *CompileError
			if errors.As(err, &ce) {
				ce.Include = filepath.Join(rtDir, "include")
				ce.PkgPath = pkgs[units[k].pkg].Path
				ce.Unit = units[k].src
			}
			return err
		}
	}
	// The libraries after the object files that use them, as a linker that reads its inputs once
	// (GNU ld with static libraries) needs. The link flags of the packages go last, the packages
	// that import the others first.
	var objs, link []string
	for _, u := range units {
		objs = append(objs, u.obj)
	}
	for _, o := range srcObjs {
		objs = append(objs, o...)
	}
	for i := len(pkgs) - 1; i >= 0; i-- {
		link = appendLinkFlags(link, pkgs[i].Flags.Link)
	}
	args := slices.Concat(Flags(b.CXX, b.ASan), objs, libObjs, rtObjs, link, b.Env.LDFlags, []string{"-o", out})
	return b.run(ctx, b.CXX, "the generated C++", true, args)
}

// compileUnit compiles the generated translation unit src of the package pb into obj, through the
// object cache, and returns what the compiler printed (its warnings are written to Stderr).
func (b *Builder) compileUnit(ctx context.Context, rtDir string, pb *PackageBuild, src, obj string) ([]byte, error) {
	// The runtime's include folder comes before the package's -I folders, so that no package
	// folder can hide a gocpp/ header. After the flags of the environment and of the package
	// come gocpp's language flags and those of generated C++ again, and -x c++ right before the
	// source, so that no flag of a package (-std=c++17, -ffast-math, -x c) turns them off: for
	// each of them the last one wins. -Wno-error comes before the flags of generated C++: a
	// package's -Werror -Wshadow must not make valid Go fail (for i := ...; { i := i }), and
	// the -Werror= of GeneratedFlags (g++'s conditionally-supported and narrowing) stays an
	// error. Without such flags the command stays short (-x shows it).
	var quote []string // -iquote: the folder of the local headers, then the generated headers
	if pb.QuoteDir != "" {
		quote = append(quote, "-iquote", pb.QuoteDir)
	}
	if b.GenRoot != "" {
		quote = append(quote, "-iquote", b.GenRoot)
	}
	pkgFlags := slices.Concat(pb.Flags.Compile, pb.Public)
	compile := slices.Concat(CompileFlags(b.CXX, b.ASan, b.NoChecks), GeneratedFlags(b.CXX), quote, []string{"-I", filepath.Join(rtDir, "include"), "-c", src, "-o", obj})
	if user := slices.Concat(b.Env.CXXCompile(), pkgFlags); len(user) > 0 {
		compile = slices.Concat(CompileFlags(b.CXX, b.ASan, b.NoChecks), quote, []string{"-I", filepath.Join(rtDir, "include")},
			user, LanguageFlags(), []string{"-Wno-error"}, GeneratedFlags(b.CXX), []string{"-c", "-x", "c++", src, "-o", obj})
	}
	// A plain compile uses the precompiled header of the runtime when there is one (pch.go; the
	// first build with new flags has none yet: startPCH builds it while the runtime compiles).
	pc := b.precompiledFor(rtDir, pkgFlags)
	// The cache (objcache.go) keys the plain compile; the precompiled header only makes the
	// compile that a miss runs faster.
	usedPCH := false
	var mu sync.Mutex
	runGen := func(args []string) ([]byte, error) {
		out, used, err := b.runGenerated(ctx, pc, args)
		mu.Lock()
		usedPCH = used
		mu.Unlock()
		return out, err
	}
	pchDeps := func() string { // g++ doesn't list the headers that its precompiled header replaced
		mu.Lock()
		defer mu.Unlock()
		if usedPCH {
			return pc.depfile()
		}
		return ""
	}
	out, err := b.compileCached(ctx, b.CXX, src, obj, compile, runGen, pchDeps)
	b.warn(out)
	return out, err
}

// sameLib reports whether a and b are the library of one module, as two packages that import its
// headers read it.
func sameLib(a, b *CxxLib) bool {
	return a == b || a.Module == b.Module && a.Version == b.Version && a.File == b.File && a.Dir == b.Dir
}

// appendLinkFlags adds the link flags more to link: a flag that names a library or a folder
// (-lm, -L/opt/lib, -pthread) once.
func appendLinkFlags(link, more []string) []string {
	for _, f := range more {
		if (len(f) > 2 && (strings.HasPrefix(f, "-l") || strings.HasPrefix(f, "-L")) || f == "-pthread") && slices.Contains(link, f) {
			continue
		}
		link = append(link, f)
	}
	return link
}
