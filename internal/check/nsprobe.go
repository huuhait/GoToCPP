package check

import (
	"context"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
)

// The namespace probe in the check pipeline (Phase 01 README §2.5 step 4).
//
// The loader took every name that a file with header imports uses as
// std in std.vector, and that Go declares nowhere, as a C++ namespace, and
// rewrote the use to C.std.vector (load.Rewrite.Namespaces), which
// go/types accepts without a word (FakeImportC). Before go/types runs,
// Check asks the C++ compiler (interop.ProbeNamespaces, the namespace mode
// of Phase 04's kind probe, which grew out of Phase 01's namespace probe)
// which of these names the file's headers really declare as namespaces. For a name that
// is not a namespace, such as the typo stdd, Check undoes the rewrite:
// stdd.vector is what the user wrote again, so go/types reports
// "undefined: stdd" at each use, as for any other undefined name: with
// its own message, in the order it finds them, and the value is invalid
// for a reason that go/types knows (an error), not opaque. So the opaque
// pass and the conversions of step 1.10b treat it as the error it is.
//
// The probe also checks the headers themselves: a header that isn't found
// or doesn't compile is an error at the import that brought it in, and
// then the probe can't tell anything about the names, so the uses stay
// namespaces (opaque), and nothing more is reported about them. The same
// goes for a probe that can't run (no C++ compiler): one error per
// package, at its first header import. The paths of the namespace renames
// (fs "::std::filesystem") are checked too.
//
// Before the probe, Check decides which header imports the build includes
// inside extern "C" { } (externc.go, step 4.14), and the probe gets those
// includes marked with interop.ExternC, so that it sees the headers as the
// build does. The errors of the extern-c and no-extern-c directives come
// with the errors of the flags.
//
// These errors come first in a package's found order: the probe runs
// before go/types. The probe runs once per include set (the files of a
// package that have the same header imports, with all their names), the
// probes of all packages run at the same time (at most GOMAXPROCS, like
// the compiles of go build), and interop caches the results.
//
// The probe gets the package's flags (build.ReadPackageFlags): the trusted
// flags of the environment (GOCPP_CFLAGS, GOCPP_CXXFLAGS), then those of
// //gocpp:cflags and //gocpp:pkg-config, after the flag allow-list, so a
// header that is found only through them (<SDL.h> in /usr/include/SDL2) is
// found. The flags are read first, for every package that Check checks,
// also one without header imports: their errors (a flag that the
// allow-list refuses, a pkg-config package that isn't installed) are the
// package's first errors, at the directives. A package with such an error
// is not probed, as the flags it would need are missing: its names stay
// namespaces (opaque), and its headers are not checked. The same goes
// while the language server runs pkg-config in the background
// (build.ErrPending).
//
// Check changes the program it checks: it undoes the rewrites in the ASTs
// and in load.Rewrite (and step 1.10b rewrites conversions). A Program is
// checked once; after a header changes, load it again (the language
// server makes a new snapshot anyway).

// A probeGroup is the files of one package that have the same header
// imports, which the probe checks with one call.
type probeGroup struct {
	files    []*load.File
	includes []string // the #include texts: <vector>, "./physics.h"
	cands    []string // the candidates of all the files, each once

	res interop.Result
	err error
}

// A probedPackage is what the probe found in one package.
type probedPackage struct {
	lp     *load.Package
	groups []*probeGroup

	// cxxErr is set when the package has header imports and no C++
	// compiler was found, or the flags of the environment can't be read.
	cxxErr error

	// flags are the probe's flags for the package (build.ProbeFlags), and
	// cflags the C flags of the C header check (GOCPP_CFLAGS, then the
	// package's compile flags).
	flags, cflags []string

	// wrap holds the include texts of the package's header imports that
	// the build includes inside extern "C" { } (externc.go), and cChecks
	// the local headers that the C header check decides about, with their
	// answers or errors.
	wrap    map[string]bool
	cChecks []*cCheck
}

// A cCheck is the C header check of one local header (Config.CHeader).
type cCheck struct {
	include string   // the #include text: "./legacy.h"
	dir     string   // the package's folder
	flags   []string // probedPackage.cflags
	res     interop.CHeader
	err     error
}

// A packageFlags is what probeNamespaces found of the flags of a package.
type packageFlags struct {
	flags   build.PackageFlags
	probe   []string        // build.ProbeFlags
	externC map[string]bool // probedPackage.wrap
}

// probeNamespaces reads the flags of the packages of prog that Check
// checks (not Incomplete), runs the namespace probe for them, undoes the
// rewrite of each use of a name that is not a namespace (see above), and
// returns the errors of each package, in the order they were found, and
// the flags of each package.
func probeNamespaces(prog *load.Program, cfg Config) (map[*load.Package][]diag.Diagnostic, map[*load.Package]packageFlags) {
	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}
	env, envErr := build.ReadEnvFlags()
	if cfg.Env != nil {
		env, envErr = *cfg.Env, nil // the driver's: the environment's flags and the build flags (gocpp build -cxxflags)
	}
	errs := make(map[*load.Package][]diag.Diagnostic)
	flagsOf := make(map[*load.Package]packageFlags)
	var pkgs []*probedPackage
	needCXX := false
	for _, lp := range prog.Packages {
		if lp.Incomplete {
			continue
		}
		pf, ferrs, pending := build.ReadPackageFlags(ctx, prog.Fset, lp, cfg.PkgConfig)
		probeFlags := build.ProbeFlags(env, pf)
		wrap := make(map[string]bool)
		flagsOf[lp] = packageFlags{flags: pf, probe: probeFlags, externC: wrap}
		externC, xerrs := readExternC(prog.Fset, lp)
		if all := directiveOrder(lp, ferrs, xerrs); len(all) > 0 {
			errs[lp] = all
		}
		if len(ferrs) > 0 || pending {
			continue // see above
		}
		pp := &probedPackage{lp: lp, flags: probeFlags, cflags: slices.Concat(env.CFlags, pf.Compile), cxxErr: envErr, wrap: wrap}
		byKey := make(map[string]*probeGroup)
		for _, f := range lp.Files {
			if f.Info == nil || f.Rewrite == nil || len(f.Info.Includes) == 0 && len(f.Rewrite.Renames) == 0 {
				continue
			}
			includes := includeTexts(f.Info.Includes)
			key := strings.Join(includes, "\x00")
			g := byKey[key]
			if g == nil {
				g = &probeGroup{includes: includes}
				byKey[key] = g
				pp.groups = append(pp.groups, g)
				needCXX = needCXX || len(includes) > 0 && envErr == nil
			}
			g.files = append(g.files, f)
			for i, inc := range f.Info.Includes {
				t := includes[i]
				if _, done := wrap[t]; done || slices.ContainsFunc(pp.cChecks, func(c *cCheck) bool { return c.include == t }) {
					continue
				}
				if w, ok := externC[headerKey(inc.Kind, inc.Path)]; ok {
					wrap[t] = w
				} else if externCDefault(inc) {
					pp.cChecks = append(pp.cChecks, &cCheck{include: t, dir: lp.Dir, flags: pp.cflags})
				}
			}
			for _, c := range f.Rewrite.Namespaces {
				if !slices.Contains(g.cands, c.Name) {
					g.cands = append(g.cands, c.Name)
				}
			}
			for _, rn := range f.Rewrite.Renames {
				if !slices.Contains(g.cands, rn.Namespace) {
					g.cands = append(g.cands, rn.Namespace)
				}
			}
		}
		if len(pp.groups) > 0 {
			pkgs = append(pkgs, pp)
		}
	}
	if len(pkgs) == 0 || ctx.Err() != nil {
		return errs, flagsOf
	}

	var cxx build.CXX
	var cxxErr error
	if needCXX {
		find := cfg.CXX
		if find == nil {
			goos, goarch := cfg.GOOS, cfg.GOARCH
			if goos == "" {
				goos = runtime.GOOS
			}
			if goarch == "" {
				goarch = runtime.GOARCH
			}
			find = func(ctx context.Context) (build.CXX, error) { return build.FindCXXFor(ctx, goos, goarch) }
		}
		cxx, cxxErr = find(ctx)
	}
	probe := cfg.Probe
	if probe == nil {
		probe = interop.ProbeNamespaces
	}

	// The probes run at the same time, at most GOMAXPROCS of them. A group
	// without includes needs no compiler: every name is Unknown there.
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup

	// First the C header checks, whose answers the probes need: they get
	// the includes as the build has them, in extern "C" { } or not.
	cheader := cfg.CHeader
	if cheader == nil {
		cheader = interop.CheckCHeaderFor
	}
	checkCHeaders(ctx, pkgs, cxx, cxxErr, cheader, sem)
	for _, pp := range pkgs {
		for _, g := range pp.groups {
			for i, t := range g.includes {
				g.includes[i] = wrapExternC(t, pp.wrap)
			}
		}
	}

	for _, pp := range pkgs {
		for _, g := range pp.groups {
			if len(g.includes) == 0 {
				continue
			}
			if cxxErr != nil {
				pp.cxxErr = cxxErr
			}
			if pp.cxxErr != nil {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				g.res, g.err = probe(ctx, cxx, pp.lp.Dir, pp.flags, g.includes, g.cands)
			}()
		}
	}
	wg.Wait()

	for _, pp := range pkgs {
		if ds := pp.apply(prog.Fset); len(ds) > 0 {
			errs[pp.lp] = append(errs[pp.lp], ds...)
		}
	}
	return errs, flagsOf
}

// checkCHeaders runs the C header checks of the packages pkgs with
// cheader (Config.CHeader) and the C++ compiler cxx, at most cap(sem) at
// the same time, and puts their answers into each package's wrap. With no
// C++ compiler (cxxErr, which the probe reports), or the language server's
// placeholder (a compiler of no known family, whose probes answer
// Unknown), nothing is wrapped by the default rules.
func checkCHeaders(ctx context.Context, pkgs []*probedPackage, cxx build.CXX, cxxErr error,
	cheader func(context.Context, build.CXX, string, []string, string) (interop.CHeader, error), sem chan struct{}) {
	var checks []*cCheck
	for _, pp := range pkgs {
		if pp.cxxErr == nil {
			checks = append(checks, pp.cChecks...)
		}
	}
	if len(checks) == 0 || cxxErr != nil || cxx.Family != build.GCC && cxx.Family != build.Clang {
		return
	}
	var wg sync.WaitGroup
	for _, c := range checks {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			c.res, c.err = cheader(ctx, cxx, c.dir, c.flags, c.include)
		})
	}
	wg.Wait()
	for _, pp := range pkgs {
		for _, c := range pp.cChecks {
			if c.err == nil {
				pp.wrap[c.include] = c.res.C && !c.res.Guard
			}
		}
	}
}

// directiveOrder returns the errors of the directives of lp, flags (from
// build.ReadPackageFlags) and externC (from readExternC), each list in
// source order, together in source order: by file, in lp's order, then by
// place.
func directiveOrder(lp *load.Package, flags, externC []diag.Diagnostic) []diag.Diagnostic {
	all := slices.Concat(flags, externC)
	fileIndex := make(map[string]int, len(lp.Files))
	for i, f := range lp.Files {
		fileIndex[f.Path] = i
	}
	// A diagnostic in another file (a line of a cxx recipe in gocpp.mod)
	// comes after those of the package's files, by file name and line.
	index := func(name string) int {
		if i, ok := fileIndex[name]; ok {
			return i
		}
		return len(lp.Files)
	}
	slices.SortStableFunc(all, func(a, b diag.Diagnostic) int {
		if a.Pos.Filename != b.Pos.Filename {
			if ia, ib := index(a.Pos.Filename), index(b.Pos.Filename); ia != ib {
				return ia - ib
			}
			return strings.Compare(a.Pos.Filename, b.Pos.Filename)
		}
		if a.Pos.Offset != b.Pos.Offset {
			return a.Pos.Offset - b.Pos.Offset
		}
		return a.Pos.Line - b.Pos.Line
	})
	return all
}

// notNamespace reports whether the probe's answer k says that a name is not
// a namespace: it is something else (NotNamespace, Macro, or another kind
// of ProbeKinds), or nothing (a typo). Unknown is no answer (the language
// server's probe hasn't answered yet), so the uses stay namespaces then.
func notNamespace(k interop.Kind) bool {
	return k != interop.Namespace && k != interop.Unknown
}

// includeTexts returns the #include texts of the includes (includeText).
func includeTexts(incs []*load.Include) []string {
	texts := make([]string, len(incs))
	for i, inc := range incs {
		texts[i] = includeText(inc)
	}
	return texts
}

// includeText returns the #include text of the include inc: <vector> for a
// system header, "./physics.h" for a local one, and the quoted absolute file
// of a module header.
func includeText(inc *load.Include) string { return inc.IncludeText() }

// apply undoes the rewrites of the names that are not namespaces in the
// files of pp and returns the errors of the probe, in file order and, in
// a file, in source order.
func (pp *probedPackage) apply(fset *token.FileSet) []diag.Diagnostic {
	var ds, fileDs []diag.Diagnostic
	add := func(pos, end token.Pos, msg string) {
		fileDs = append(fileDs, diag.Diagnostic{Pos: fset.Position(pos), End: fset.Position(end), Msg: msg})
	}
	byFile := make(map[*load.File]*probeGroup)
	for _, g := range pp.groups {
		for _, f := range g.files {
			byFile[f] = g
		}
	}
	reported := make(map[*probeGroup]bool) // a group whose probe failed
	cxxReported := false
	cErrs := make(map[string]error) // the C header checks that failed, until reported
	for _, c := range pp.cChecks {
		if c.err != nil {
			cErrs[c.include] = c.err
		}
	}
	for _, f := range pp.lp.Files {
		g := byFile[f]
		if g == nil {
			continue
		}
		fileDs = fileDs[:0]
		incs := f.Info.Includes
		for _, inc := range incs {
			t := includeText(inc)
			if err := cErrs[t]; err != nil {
				delete(cErrs, t)
				add(inc.Pos, inc.Spec.Path.End(), "cannot check whether header "+strconv.Quote(inc.Path)+" is a C header: "+err.Error())
			}
		}
		switch {
		case len(incs) == 0:
			// Only namespace renames: without a header, nothing declares
			// a namespace (and no compiler runs).
			for _, rn := range f.Rewrite.Renames {
				add(rn.Pos, rn.Spec.Path.End(), "cannot rename "+rn.Namespace+": the file imports no header that could declare it")
			}
		case pp.cxxErr != nil:
			if !cxxReported {
				cxxReported = true
				add(incs[0].Pos, incs[0].Spec.Path.End(), "cannot check the header imports: "+pp.cxxErr.Error())
			}
		case g.err != nil:
			if !reported[g] {
				reported[g] = true
				add(incs[0].Pos, incs[0].Spec.Path.End(), "cannot check the header imports: "+g.err.Error())
			}
		case len(g.res.IncludeErrors) > 0:
			for _, ie := range g.res.IncludeErrors {
				if ie.Index < len(incs) {
					inc := incs[ie.Index]
					add(inc.Pos, inc.Spec.Path.End(), includeErrorText(inc, ie, pp.lp.Dir))
				}
			}
		default:
			for _, c := range slices.Clone(f.Rewrite.Namespaces) {
				if notNamespace(g.res.Kinds[c.Name]) {
					undoNamespace(f.Rewrite, c)
				}
			}
			for _, rn := range f.Rewrite.Renames {
				if notNamespace(g.res.Kinds[rn.Namespace]) {
					add(rn.Pos, rn.Spec.Path.End(), rn.Namespace+" is not a namespace of the imported headers")
				}
			}
		}
		// In source order: all of them are at imports.
		slices.SortStableFunc(fileDs, func(a, b diag.Diagnostic) int { return a.Pos.Offset - b.Pos.Offset })
		ds = append(ds, fileDs...)
	}
	return ds
}

// undoNamespace undoes the rewrite of the uses of the candidate c of a
// file, whose Rewrite is rw: C.stdd.vector becomes stdd.vector again, as
// the user wrote it, and c and its uses leave rw.Namespaces and rw.Uses,
// so that codegen, the gates and the editor see no namespace there. The
// import "C" that RewriteFile may have added for them stays: go/types
// never reports it as unused.
func undoNamespace(rw *load.Rewrite, c *load.NamespaceCandidate) {
	for _, u := range c.Uses {
		u.Sel.X = u.Ident
	}
	rw.Uses = slices.DeleteFunc(rw.Uses, func(u *load.CppUse) bool { return slices.Contains(c.Uses, u) })
	rw.Namespaces = slices.DeleteFunc(rw.Namespaces, func(n *load.NamespaceCandidate) bool { return n == c })
}

// includeErrorText returns the message for the include error ie of the
// header import inc, in a package in the folder dir.
func includeErrorText(inc *load.Include, ie interop.IncludeError, dir string) string {
	header := inc.Path
	if inc.Kind == load.LocalHeader {
		header = strconv.Quote(inc.Path)
	}
	switch {
	case ie.Kind == interop.HeaderNotFound:
		msg := "cannot find header " + header
		// <physics.h> or <sub/s.h> with that file in the package folder:
		// <> doesn't look there.
		if inc.Kind == load.SystemHeader {
			name := strings.TrimSuffix(strings.TrimPrefix(inc.Path, "<"), ">")
			if fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err == nil && !fi.IsDir() {
				msg += " (the package folder has it: import " + strconv.Quote("./"+name) + ")"
			}
		}
		return msg
	case ie.Kind == interop.HeaderUnclosed:
		// The compiler's message is about the probe's own check line.
		return "header " + header + " ends inside a declaration or a block that it doesn't close"
	case ie.File == "":
		// Another error at the #include line. Its message can have the
		// header's full path; the package folder is left out.
		msg := strings.ReplaceAll(ie.Msg, dir+string(filepath.Separator), "")
		if readErrorRx.MatchString(msg) {
			// "x.h: Permission denied" (gcc), "cannot open file 'x.h':
			// Permission denied" (clang).
			return "header " + header + " can't be read: " + msg
		}
		return "header " + header + ": " + msg
	}
	pos := shortPath(ie.File, dir) + ":" + strconv.Itoa(ie.Line)
	if ie.Column > 0 {
		pos += ":" + strconv.Itoa(ie.Column)
	}
	return "header " + header + " doesn't compile: " + pos + ": " + ie.Msg
}

// readErrorRx matches the compilers' messages at an #include line for a
// header that was found but can't be read.
var readErrorRx = regexp.MustCompile(`^cannot open file |: Permission denied$`)

// shortPath returns the path of the file relative to the folder dir when
// the file is inside it, else the path itself.
func shortPath(file, dir string) string {
	if rel, err := filepath.Rel(dir, file); err == nil && filepath.IsLocal(rel) {
		return rel
	}
	return file
}
