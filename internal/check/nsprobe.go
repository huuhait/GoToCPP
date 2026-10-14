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
// Check asks the C++ compiler (interop.ProbeNamespaces) which of these
// names the file's headers really declare as namespaces. For a name that
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
// These errors come first in a package's found order: the probe runs
// before go/types. The probe runs once per include set (the files of a
// package that have the same header imports, with all their names), the
// probes of all packages run at the same time (at most GOMAXPROCS, like
// the compiles of go build), and interop caches the results.
//
// Not yet (Phase 04 adds the package's flags to the probe): the probe
// can't use the flags of //gocpp:pkg-config and //gocpp:cflags, and a
// header that is found only through them (<SDL.h> in /usr/include/SDL2)
// would be "not found". So a package with such a directive (directives are
// package-wide) is not probed at all: no compiler runs, its names stay
// namespaces (opaque), and its headers are not checked.
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
	// compiler was found.
	cxxErr error
}

// probeNamespaces runs the namespace probe for the packages of prog that
// Check checks (not Incomplete), undoes the rewrite of each use of a name
// that is not a namespace (see above), and returns the errors of each
// package, in the order they were found.
func probeNamespaces(prog *load.Program, cfg Config) map[*load.Package][]diag.Diagnostic {
	var pkgs []*probedPackage
	needCXX := false
	for _, lp := range prog.Packages {
		if lp.Incomplete || hasFlagDirectives(lp) {
			continue
		}
		pp := &probedPackage{lp: lp}
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
				needCXX = needCXX || len(includes) > 0
			}
			g.files = append(g.files, f)
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
	if len(pkgs) == 0 {
		return nil
	}

	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
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
	for _, pp := range pkgs {
		for _, g := range pp.groups {
			if len(g.includes) == 0 {
				continue
			}
			if cxxErr != nil {
				pp.cxxErr = cxxErr
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				g.res, g.err = probe(ctx, cxx, pp.lp.Dir, g.includes, g.cands)
			}()
		}
	}
	wg.Wait()

	errs := make(map[*load.Package][]diag.Diagnostic, len(pkgs))
	for _, pp := range pkgs {
		if ds := pp.apply(prog.Fset); len(ds) > 0 {
			errs[pp.lp] = ds
		}
	}
	return errs
}

// hasFlagDirectives reports whether a file of lp has a //gocpp:pkg-config
// or //gocpp:cflags directive, whose flags can change where the headers are
// found.
//
// TODO(phase 04): probe such a package with its flags (pkg-config --cflags
// and the cflags), instead of not at all.
func hasFlagDirectives(lp *load.Package) bool {
	for _, f := range lp.Files {
		for _, d := range f.Directives {
			if d.Name == "pkg-config" || d.Name == "cflags" {
				return true
			}
		}
	}
	return false
}

// includeTexts returns the #include texts of the includes: <vector> for a
// system header, "./physics.h" for a local one.
func includeTexts(incs []*load.Include) []string {
	texts := make([]string, len(incs))
	for i, inc := range incs {
		if inc.Kind == load.LocalHeader {
			texts[i] = `"` + inc.Path + `"`
		} else {
			texts[i] = inc.Path
		}
	}
	return texts
}

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
	for _, f := range pp.lp.Files {
		g := byFile[f]
		if g == nil {
			continue
		}
		fileDs = fileDs[:0]
		incs := f.Info.Includes
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
				if g.res.Kinds[c.Name] == interop.NotNamespace {
					undoNamespace(f.Rewrite, c)
				}
			}
			for _, rn := range f.Rewrite.Renames {
				if g.res.Kinds[rn.Namespace] == interop.NotNamespace {
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
