package lower

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
)

// The compiler probes in the build (Phase 04 README §2.2, steps 4.2 and 4.3).
//
// The C++ spelling of a C/C++ name depends on what the name is, which only the C++ compiler
// knows: std.chrono.steady_clock.now() is ::std::chrono::steady_clock::now() because
// steady_clock is a type, std.cout.flush() is ::std::cout.flush() because cout is a value,
// cstdio.EOF is EOF because EOF is a macro (::EOF would break it). Before lowering a package
// with header imports, the driver calls ProbeInterop, which asks the compiler with the two
// probes of internal/interop:
//   - interop.ProbeKinds, once per include set (the files of the package with the same header
//     imports), with every dotted path that the files use (probePaths: std::chrono,
//     std::chrono::steady_clock, std::chrono::steady_clock::now, printf, and the name of each
//     header package, which may be a namespace too: glm for <glm/glm.hpp>);
//   - interop.ProbeMacros, once, with all the includes of the translation unit, for the macros
//     that collide with the names that gocpp writes (macros.go).
//
// The check (check.Check) runs the namespace mode of the kind probe for the same include sets.
// gocpp build, run and emit give it a probe that asks for these paths too (ProbePaths), so the
// lowerer's ProbeKinds call finds them in interop's cache: one compiler run per include set.
// gocpp check and the language server keep the namespace mode, which costs less while someone
// types.
//
// Flags: the driver passes the check's (check.Package.ProbeFlags) to ProbeInterop. The includes
// that the build wraps in extern "C" { } (check.Package.ExternC, step 4.14) go to both probes
// marked with interop.ExternC, as they went to the check's probe.

// An Interop is what the probes found out about the C/C++ names of one package: the kind of
// each path, per file, and the macros of the translation unit. ProbeInterop makes it, and
// Package needs it for a package with header imports.
type Interop struct {
	kinds  map[*load.File]map[string]interop.Kind
	macros map[string]bool // the macros after all the includes (interop.ProbeMacros)
}

// An includeSet is the files of a package with the same header imports, which one ProbeKinds
// call answers.
type includeSet struct {
	files    []*load.File
	includes []string
	kinds    map[string]interop.Kind
}

// includeText returns the #include text of the header import inc: <vector> for a system
// header, "./physics.h" for a local one, marked with interop.ExternC when the build includes it
// inside extern "C" { } (externC, check.Package.ExternC): as check gives them to the namespace
// probe.
func includeText(inc *load.Include, externC map[string]bool) string {
	t := inc.IncludeText()
	if externC[t] {
		return interop.ExternC(t)
	}
	return t
}

// includeSets groups the files of pkg that import headers by their header imports, with the
// includes that externC says marked with interop.ExternC.
func includeSets(lp *load.Package, externC map[string]bool) []*includeSet {
	var sets []*includeSet
	byKey := make(map[string]*includeSet)
	for _, f := range lp.Files {
		if f.Info == nil || len(f.Info.Includes) == 0 {
			continue
		}
		var incs []string
		for _, inc := range f.Info.Includes {
			incs = append(incs, includeText(inc, externC))
		}
		key := strings.Join(incs, "\x00")
		s := byKey[key]
		if s == nil {
			s = &includeSet{includes: incs}
			byKey[key] = s
			sets = append(sets, s)
		}
		s.files = append(s.files, f)
	}
	return sets
}

// A unitInclude is one #include of the translation unit: the first import of a header in the
// package's files, in file and source order. text is its include text for the probes (with the
// mark of interop.ExternC when the build includes it inside extern "C" { }).
type unitInclude struct {
	text string
	inc  *load.Include
}

// unitIncludes returns the #include lines of the translation unit of lp, after gocpp's own
// headers: the header imports of its files, in file order and source order, each header once.
// externC is check.Package.ExternC.
func unitIncludes(lp *load.Package, externC map[string]bool) []unitInclude {
	var list []unitInclude
	seen := make(map[string]bool)
	for _, f := range lp.Files {
		if f.Info == nil {
			continue
		}
		for _, inc := range f.Info.Includes {
			t := includeText(inc, externC)
			if !seen[t] {
				seen[t] = true
				list = append(list, unitInclude{text: t, inc: inc})
			}
		}
	}
	return list
}

// ProbeInterop runs the compiler probes for the C/C++ names of the checked package pkg (see
// the top of interop.go) with the C++ compiler cxx and the package's flags. For a package
// without header imports it runs nothing. An error is for a probe that fails, or for a header
// that doesn't work (the check reports those first, except in a package that it doesn't probe:
// see check's hasFlagDirectives); it is a *Error at the header's import when it is about one.
func ProbeInterop(ctx context.Context, cxx build.CXX, fset *token.FileSet, pkg *check.Package, flags []string) (*Interop, error) {
	in := &Interop{kinds: make(map[*load.File]map[string]interop.Kind)}
	lp := pkg.Load
	sets := includeSets(lp, pkg.ExternC)
	if len(sets) == 0 {
		return in, nil
	}
	for _, s := range sets {
		paths := setPaths(s.files, nil)
		res, err := interop.ProbeKinds(ctx, cxx, lp.Dir, flags, s.includes, paths)
		if err != nil {
			return nil, fmt.Errorf("cannot probe the C/C++ names of the header imports: %w", err)
		}
		if err := includeError(fset, s, res); err != nil {
			return nil, err
		}
		// A header package whose name is also a namespace (glm for <glm/glm.hpp>): its names
		// are looked up in the namespace first (§2.3). Rare, so it costs one more run only
		// when it happens.
		ns := make(map[string]bool)
		for _, f := range s.files {
			for _, inc := range f.Info.Includes {
				if inc.Name != "" && res.Kinds[inc.Name] == interop.Namespace {
					ns[inc.Name] = true
				}
			}
		}
		if len(ns) > 0 {
			paths = setPaths(s.files, ns)
			if res, err = interop.ProbeKinds(ctx, cxx, lp.Dir, flags, s.includes, paths); err != nil {
				return nil, fmt.Errorf("cannot probe the C/C++ names of the header imports: %w", err)
			}
		}
		s.kinds = res.Kinds
		for _, f := range s.files {
			in.kinds[f] = res.Kinds
		}
	}
	var incs []string
	for _, ui := range unitIncludes(lp, pkg.ExternC) {
		incs = append(incs, ui.text)
	}
	ms, err := interop.ProbeMacros(ctx, cxx, lp.Dir, flags, incs)
	if err != nil {
		return nil, fmt.Errorf("cannot list the macros of the header imports: %w", err)
	}
	in.macros = make(map[string]bool, ms.Len())
	for _, name := range ms.Names() {
		in.macros[name] = true
	}
	return in, nil
}

// includeError returns the error for the first include of s that didn't work, or nil.
func includeError(fset *token.FileSet, s *includeSet, res interop.Result) error {
	if len(res.IncludeErrors) == 0 {
		return nil
	}
	ie := res.IncludeErrors[0]
	// The mark of interop.ExternC in plain words: the user wrote only the header.
	if header, marked := interop.SplitExternC(ie.Include); marked {
		ie.Include = header + ` (inside extern "C")`
	}
	if f := s.files[0]; ie.Index < len(f.Info.Includes) {
		inc := f.Info.Includes[ie.Index]
		return &Error{Pos: fset.Position(inc.Pos), Msg: "the header import doesn't work: " + ie.Error(), User: true}
	}
	return fmt.Errorf("header %s: %v", ie.Include, ie)
}

// ProbePaths returns, for the packages of prog, the dotted C/C++ paths that the lowerer asks
// the kind probe about, by include set: paths(dir, includes) for the files of the package in
// the folder dir whose header imports are includes (their #include texts, as check passes them
// to its probe). The driver adds them to the namespace candidates of the check's probe, so
// that the check and the lowerer share one compiler run (see the top of interop.go). It must
// be called before check.Check, which undoes the namespace uses of the names that are no
// namespaces; their paths are then asked for too, which costs little. The includes may have the
// mark of interop.ExternC (check decides which ones after ProbePaths); it doesn't matter here.
func ProbePaths(prog *load.Program) func(dir string, includes []string) []string {
	m := make(map[string][]string)
	for _, lp := range prog.Packages {
		for _, s := range includeSets(lp, nil) {
			key := lp.Dir + "\x00" + strings.Join(s.includes, "\x00")
			m[key] = setPaths(s.files, nil)
		}
	}
	return func(dir string, includes []string) []string {
		plain := make([]string, len(includes))
		for i, inc := range includes {
			plain[i], _ = interop.SplitExternC(inc)
		}
		return m[dir+"\x00"+strings.Join(plain, "\x00")]
	}
}

// setPaths returns the paths that the files use (probePaths), each once, in the order they
// are found. ns are the header package names that are namespaces too.
func setPaths(files []*load.File, ns map[string]bool) []string {
	var paths []string
	seen := make(map[string]bool)
	for _, f := range files {
		for _, p := range probePaths(f, ns) {
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	return paths
}

// probePaths returns the paths of the C/C++ names that the file f uses, which the kind probe
// answers for the lowerer (resolve): for every selector chain on a header package or a
// namespace, each of its prefixes: std.chrono.steady_clock.now gives std::chrono,
// std::chrono::steady_clock and std::chrono::steady_clock::now (std is a namespace already:
// the check found that out), fs.path with fs "::std::filesystem" gives std::filesystem::path,
// and cstdio.printf gives cstdio (a header package's name, which can be a namespace too) and
// printf. For the header packages whose names are namespaces (ns), it also gives the paths in
// the namespace: glm::vec3. A chain ends at an index or a call: std.vector[int].iterator gives
// std::vector, whose kind (a template) is all the lowerer needs. A cgo tag name
// (stat.struct_stat) is no path: it is always a type.
func probePaths(f *load.File, ns map[string]bool) []string {
	if f.Rewrite == nil {
		return nil
	}
	uses := make(map[*ast.SelectorExpr]*load.CppUse)
	for _, u := range f.Rewrite.Uses {
		if (u.Kind == load.HeaderUse || u.Kind == load.NamespaceUse) && !u.Bare {
			uses[u.Sel] = u
		}
	}
	if len(uses) == 0 {
		return nil
	}
	var paths []string
	ast.Inspect(f.AST, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		u, names := useChain(uses, sel)
		if u == nil {
			return true
		}
		switch u.Kind {
		case load.NamespaceUse:
			paths = append(paths, strings.TrimPrefix(u.Namespace, "::")+"::"+strings.Join(names, "::"))
		case load.HeaderUse:
			if len(names) == 1 {
				paths = append(paths, u.Ident.Name) // is the header's name a namespace too?
			}
			if _, ok := tagName(names[0]); !ok {
				paths = append(paths, strings.Join(names, "::"))
			}
			if ns[u.Ident.Name] {
				paths = append(paths, u.Ident.Name+"::"+strings.Join(names, "::"))
			}
		}
		return true
	})
	return paths
}

// useChain returns the use of a header package or a namespace that the selector chain sel is
// on, and the names after it: std, [chrono steady_clock now] for std.chrono.steady_clock.now
// (C.std.chrono.steady_clock.now after the loader's rewrite); nil if sel is not such a chain.
// Parentheses end a chain: (std.cout).flush is a member of the value std.cout.
func useChain(uses map[*ast.SelectorExpr]*load.CppUse, sel *ast.SelectorExpr) (*load.CppUse, []string) {
	var names []string
	for {
		names = append(names, sel.Sel.Name)
		if u := uses[sel]; u != nil {
			slices.Reverse(names)
			return u, names
		}
		x, ok := sel.X.(*ast.SelectorExpr)
		if !ok {
			return nil, nil
		}
		sel = x
	}
}

// tagName returns the C++ of a cgo tag name (stat.struct_stat, lib.enum_x, lib.union_y): struct
// ::stat, enum ::x, union ::y, qualified, so that a gocpp type stat in the package's namespace
// can't hide the tag.
func tagName(name string) (string, bool) {
	for _, tag := range []string{"struct", "enum", "union"} {
		if rest, ok := strings.CutPrefix(name, tag+"_"); ok && rest != "" {
			return tag + " ::" + rest, true
		}
	}
	return "", false
}
