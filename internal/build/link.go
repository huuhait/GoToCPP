package build

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Building a program of several packages (Phase 06 steps 6.2, 6.5 to 6.7): Link compiles the translation
// units of all the packages, the packages' own C and C++ files and the C and C++ libraries of the
// modules, and links their objects with the runtime. Every compile is an action of the executor
// (executor.go: at most Jobs at a time, all the errors in a fixed order) that goes through the object
// cache (objcache.go), which keys it by its text, the flags, the compiler and every header the compile
// read, so a program in which nothing changed compiles nothing, and a changed .gp file recompiles its
// own unit and the units of the packages that include a header that changed. A Builder with a Rec
// also records what the build did, for the up-to-date check (stamp.go).

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

// Link compiles the packages and links the program out. The objects of the packages come in the
// order of pkgs; a compile or link that fails is a *CompileError (PkgPath says the package of a
// unit or a source that failed). When several compiles fail, the error is an *Errors that holds
// them in a fixed order (the package's own C and C++ files, the libraries of the modules, then the
// generated units, each in the order of pkgs), and nothing is linked. The generated units may
// include each other's headers through GenRoot, the folder that holds them. At most Jobs compiles
// run at the same time (executor.go).
func (b *Builder) Link(ctx context.Context, pkgs []PackageBuild, out string) error {
	b.applyMode() // a library's -fPIC (modes.go), before anything is compiled or keyed
	// With flags of the environment, the runtime's compiles get gocpp's language flags after
	// them too (as the generated C++ does), then -Wno-error (valid code must not fail for a
	// warning that a user's -Werror -Wshadow turns into an error) and -x c++, which comes right
	// before each runtime source. All of them are part of the key of the runtime's object
	// files.
	flags := CompileFlags(b.CXX, b.ASan, b.NoChecks)
	if env := b.Env.CXXCompile(); len(env) > 0 {
		flags = slices.Concat(flags, env, LanguageFlags(), []string{"-Wno-error", "-x", "c++"})
	}
	b.sc = statMemo()
	defer b.waitPCH() // a header that is still being built is finished, not left half done
	defer b.stores.Wait()
	// Only a program that imports testing is linked with the natives of testing (rtfiles.TestingSources).
	usesTesting := slices.ContainsFunc(pkgs, func(pb PackageBuild) bool { return pb.Path == "testing" })
	rtDir, rtObjs, rtErr := b.runtime(ctx, flags, usesTesting)
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

	// The actions, in the order of the errors: the package's own C and C++ files (an error in
	// them is the user's), the libraries, the generated units.
	var acts []action
	srcObjs := make([][]string, len(pkgs))
	for i := range pkgs {
		if len(pkgs[i].CFiles)+len(pkgs[i].CXXFiles) == 0 {
			continue
		}
		objs, as, err := b.planPackageSources(ctx, rtDir, &pkgs[i], i, len(pkgs) > 1)
		if err != nil {
			return err
		}
		srcObjs[i] = objs
		acts = append(acts, as...)
	}
	libObjs, libActs, libErr := b.planLibs(ctx, rtDir, libs)
	acts = append(acts, libActs...)

	var unitObjs []string
	for i, pb := range pkgs {
		for _, src := range pb.Units {
			obj := filepath.Join(b.Work, strings.TrimSuffix(filepath.Base(src), ".cpp")+".o")
			if len(pkgs) > 1 || len(pb.Units) > 1 {
				obj = filepath.Join(b.Work, fmt.Sprintf("%02d-%s.o", i, strings.TrimSuffix(filepath.Base(src), ".cpp")))
			}
			unitObjs = append(unitObjs, obj)
			pb := &pkgs[i]
			cmd := b.unitCommand(rtDir, pb, src, obj)
			b.record(cmd)
			acts = append(acts, action{name: src, run: func() (func(), error) {
				out, err := b.compileUnit(ctx, rtDir, pb, src, obj)
				if err == nil {
					err = b.emitIR(ctx, b.CXX, "the generated C++", cmd)
				}
				var ce *CompileError
				if errors.As(err, &ce) {
					ce.Include = filepath.Join(rtDir, "include")
					ce.PkgPath = pb.Path
					ce.Unit = src
				}
				return func() { b.warn(out) }, err
			}})
		}
	}
	errs, notStarted := b.runActions(ctx, acts)
	if err := ctx.Err(); err != nil {
		return err
	}
	// A failure to list the sources of a library comes after the errors of the C and C++ files
	// of the packages, which compiled.
	if libErr != nil {
		k := 0
		for i := range pkgs {
			k += len(srcObjs[i])
		}
		errs = slices.Insert(errs, k, libErr)
	}
	if err := joinErrors(notStarted, errs...); err != nil {
		return err
	}
	// The libraries after the object files that use them, as a linker that reads its inputs once
	// (GNU ld with static libraries) needs. The link flags of the packages go last, the packages
	// that import the others first.
	var objs, link []string
	objs = append(objs, unitObjs...)
	for _, o := range srcObjs {
		objs = append(objs, o...)
	}
	if b.Mode.Archive() {
		// A static library: the objects of the program, of the libraries and of the runtime in one
		// archive (a C program links it with the C++ library: -lstdc++, and what the packages' link
		// flags name, which LibraryLinkFlags gives).
		return b.archive(ctx, slices.Concat(objs, libObjs, rtObjs), out)
	}
	for i := len(pkgs) - 1; i >= 0; i-- {
		link = appendLinkFlags(link, pkgs[i].Flags.Link)
	}
	// --gc-sections drops the functions and data that nothing reaches (the compiles put each in
	// a section of its own); macOS's linker does that by itself.
	var gc []string
	if b.GOOS != "darwin" {
		gc = []string{"-Wl,--gc-sections"}
	}
	ldflags := b.autoLTO(b.Env.LDFlags)
	args := slices.Concat(Flags(b.CXX, b.ASan), LinkFlags(b.CXX), gc, b.Mode.LinkFlags(b.Soname, b.VersionScript), objs, libObjs, rtObjs, link, ldflags, []string{"-o", out})
	b.recordLinkPlan(Command{Kind: KindLink, Compiler: compiler(b.CXX), Args: args, Output: out, Pre: slices.Concat(Flags(b.CXX, b.ASan), LinkFlags(b.CXX), gc, b.Mode.LinkFlags(b.Soname, b.VersionScript)),
		Objs: slices.Concat(objs, libObjs, rtObjs), Post: slices.Concat(link, ldflags)})
	b.recordLink(ctx, slices.Concat(link, ldflags))
	b.Stats.linked()
	if b.Rec == nil || b.Rec.Bad() != "" {
		return b.run(ctx, b.CXX, "the generated C++", true, args)
	}
	// A build that is recorded asks the linker for the names of the files it reads (-Wl,-t).
	traced := slices.Insert(slices.Clone(args), len(args)-2, "-Wl,-t")
	printed, err := b.runCommand(ctx, b.CXX, "the generated C++", true, traced, args)
	rest, files := linkTrace(printed, b.Work)
	var ce *CompileError
	if errors.As(err, &ce) {
		ce.Output, _ = linkTrace(ce.Output, b.Work)
	}
	b.warn(rest)
	if errors.Is(err, errOutputOpen) {
		b.Rec.Fail("the linker's output may be incomplete")
		return nil
	}
	if err == nil {
		b.setLinkFiles(files)
	}
	return err
}

// unitArgs returns the arguments of the compile of the generated translation unit src of the
// package pb into obj (without the precompiled header and the flags of the dependency file, which
// the object cache adds).
func (b *Builder) unitArgs(rtDir string, pb *PackageBuild, src, obj string) []string {
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
	return compile
}

// unitCommand returns the Command of the compile of the generated unit src.
func (b *Builder) unitCommand(rtDir string, pb *PackageBuild, src, obj string) Command {
	return Command{Kind: KindUnit, Pkg: pb.Path, Lang: "c++", Compiler: compiler(b.CXX), Args: b.unitArgs(rtDir, pb, src, obj),
		Dir: b.genDir(pb), File: src, Output: obj, IR: b.irName(src)}
}

// genDir returns the folder that the commands of the generated units name as their directory:
// the folder of the generated files, else the folder of the package.
func (b *Builder) genDir(pb *PackageBuild) string {
	if b.GenRoot != "" {
		return b.GenRoot
	}
	return pb.Dir
}

// recordLinkPlan records the link in the plan, with the folders of the build.
func (b *Builder) recordLinkPlan(c Command) {
	if b.Plan == nil {
		return
	}
	b.Plan.mu.Lock()
	b.Plan.Link = &c
	b.Plan.GenRoot = b.GenRoot
	b.Plan.CXX, b.Plan.CC, b.Plan.GOOS = b.CXX, b.CC, b.GOOS
	b.Plan.mu.Unlock()
}

// compileUnit compiles the generated translation unit src of the package pb into obj, through the
// object cache, and returns what the compiler printed (its warnings are written to Stderr).
func (b *Builder) compileUnit(ctx context.Context, rtDir string, pb *PackageBuild, src, obj string) ([]byte, error) {
	pkgFlags := slices.Concat(pb.Flags.Compile, pb.Public)
	compile := b.unitArgs(rtDir, pb, src, obj)
	// A plain compile uses the precompiled header of the runtime when there is one (pch.go; the
	// first build with new flags has none yet: startPCH builds it while the runtime compiles).
	pc := b.precompiledFor(rtDir, pkgFlags)
	if b.DryRun {
		pc = nil // the commands that -n shows are the plain ones
	}
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
	return b.compileCached(ctx, b.CXX, src, obj, compile, runGen, pchDeps)
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

// autoLTO returns the link flags with a bare -flto turned into -flto=auto for GCC: without a job
// count, lto-wrapper warns "using serial compilation of N LTRANS jobs" at every link (clang's
// -flto takes no count). The compiles keep -flto, which only writes the intermediate form.
func (b *Builder) autoLTO(flags []string) []string {
	if b.CXX.Family != GCC || !slices.Contains(flags, "-flto") {
		return flags
	}
	out := slices.Clone(flags)
	for i, f := range out {
		if f == "-flto" {
			out[i] = "-flto=auto"
		}
	}
	return out
}
