package lower

import (
	"crypto/sha256"
	"encoding/hex"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
)

// A Program is the set of packages that are lowered together into the split layout (Phase 06
// README §2.2, step 6.2): for each package a header, pkg.hpp, one translation unit for each .gp
// file, pkg_desc.cpp and pkg_init.cpp.
//
// Lowering needs the other packages: a call pkg.F needs the C++ name that pkg chose for F (its
// macros decide, names.go), a type of pkg needs its struct, a conversion to an interface needs
// pkg's descriptor of the type (which pkg writes: it is declared extern in pkg.hpp and defined in
// pkg_desc.cpp, so that one object is the descriptor of the type in the whole program), and a
// deferred call of pkg.F needs to know whether F calls recover. So the packages are lowered in
// the order of their imports, in one process, and a package asks the Program for what another
// one chose. The files are written after all of them are lowered (Lower), because a package
// asks its imports for descriptors while it is lowered, and those are part of their files.
type Program struct {
	fset *token.FileSet
	opts Options

	gens  []*gen                  // the packages, each one after the packages it imports
	byPkg map[*types.Package]*gen // by the package that go/types made
	byLoc map[string]*gen         // by import path
	main  *gen                    // the main package

	// The unnamed struct and interface types, one C++ type each in the whole program (shared.go).
	anonStructs map[string]*structInfo
	anonIfaces  map[string]*ifaceInfo
	sharedOrder []sharedItem // in the order they were found
}

// A PackageInput is a type-checked package for NewProgram, with what the C++ compiler's probes
// found out about its C/C++ names (ProbeInterop; nil for a package without header imports).
type PackageInput struct {
	Pkg     *check.Package
	Interop *Interop
}

// NewProgram returns the Program of the packages in, which must be in the order of their imports
// (each one after the packages it imports) and without errors. A package of in that is imported
// but not in in (fmt, whose functions the runtime has, and weak) is not lowered: its uses are
// intrinsics.
func NewProgram(fset *token.FileSet, in []PackageInput, opts Options) *Program {
	p := &Program{
		fset:        fset,
		opts:        opts,
		byPkg:       make(map[*types.Package]*gen),
		byLoc:       make(map[string]*gen),
		anonStructs: make(map[string]*structInfo),
		anonIfaces:  make(map[string]*ifaceInfo),
	}
	for _, pi := range in {
		g := newGen(fset, pi.Pkg)
		g.prog, g.split, g.opts = p, true, opts
		g.interop = pi.Interop
		if pi.Interop != nil {
			g.macros = pi.Interop.macros
		}
		p.gens = append(p.gens, g)
		p.byPkg[pi.Pkg.Types] = g
		p.byLoc[pi.Pkg.Load.ImportPath] = g
		if pi.Pkg.Types.Name() == "main" {
			p.main = g
		}
	}
	return p
}

// A PackageFiles is the C++ of one package in the split layout.
type PackageFiles struct {
	ImportPath string // the import path of the package ("main" does not stand for the path of the main package)
	Namespace  string // gpkg_main, gpkg_example_dcom__app__geom
	Dir        string // the folder of the files, relative to the root of the generated files: gpkg/<import path>

	Header *cppir.File // pkg.hpp
	Units  []*Unit     // one for each .gp file that has functions: <file>.gp.cpp
	Desc   *cppir.File // pkg_desc.cpp: the descriptors of the package's types, or nil
	Init   *cppir.File // pkg_init.cpp: the package variables, their initialization and init functions; main's has main

	// Funcs maps the Go name of each function of the package (no methods, no init functions) to its C++
	// name in Namespace (a name that is reserved in C++ has a _ suffix, ADR-010), for the glue of
	// the exported functions of a library (Phase 06 step 6.10).
	Funcs map[string]string

	// HeaderC says that pkg.hpp includes the package's C/C++ headers, because a declaration in it
	// needs a C/C++ type (headerNeedsC): the flags that make those headers parse as the package
	// sees them are public flags, which every package that includes pkg.hpp compiles with too.
	HeaderC bool
}

// A Unit is the translation unit of one .gp file.
type Unit struct {
	Name string // the file name: render.gp.cpp
	Src  string // the absolute name of the .gp file
	File *cppir.File
}

// HeaderPath returns the path of pkg.hpp as the #include line of the other packages writes it,
// relative to the root of the generated files: gpkg/<import path>/pkg.hpp.
func HeaderPath(importPath string) string {
	return PkgDir(importPath) + "/pkg.hpp"
}

// PkgDir returns the folder of the files of the package with the import path path, relative to the
// root of the generated files.
func PkgDir(path string) string {
	return "gpkg/" + path
}

// Lower lowers the packages and returns their files in the order of the packages of NewProgram.
// It returns an *Error for a construct that it can't translate.
func (p *Program) Lower() ([]*PackageFiles, error) {
	for _, g := range p.gens {
		if !g.lowerSplit() {
			return nil, p.firstError()
		}
	}
	p.finishShared()
	// A package asks its imports for descriptors while it is lowered; the last package asked
	// of nothing, so the files are written from the last package to the first: what a package
	// asks of its imports is known when they are written.
	files := make([]*PackageFiles, len(p.gens))
	for i := len(p.gens) - 1; i >= 0; i-- {
		files[i] = p.gens[i].files()
		if p.gens[i].err != nil {
			return nil, p.firstError()
		}
	}
	if err := p.firstError(); err != nil {
		return nil, err
	}
	return files, nil
}

// firstError returns the first error of the packages, in their order, or nil.
func (p *Program) firstError() error {
	for _, g := range p.gens {
		if g.err != nil {
			return g.err
		}
	}
	return nil
}

// other returns the lowering of the package pkg of go/types, or nil for a package that the
// Program does not lower (unsafe, fmt).
func (g *gen) other(pkg *types.Package) *gen {
	if pkg == nil || pkg == g.pkg.Types {
		return g
	}
	if g.prog == nil {
		return nil
	}
	return g.prog.byPkg[pkg]
}

// foreign returns the lowering of the package pkg if it is not the package of g.
func (g *gen) foreign(pkg *types.Package) *gen {
	if o := g.other(pkg); o != g {
		return o
	}
	return nil
}

// qual returns the C++ name of the name n of the package of g, written in another package:
// ::gpkg_geom::n.
func (g *gen) qual(n string) string {
	return "::" + g.ns + "::" + n
}

// isPkgLevel reports whether the variable or function obj is declared at package level, in any
// package of the program.
func isPkgLevel(obj types.Object) bool {
	return obj.Pkg() != nil && obj.Parent() == obj.Pkg().Scope()
}

// importOrder returns the paths of the packages that the package imports and that the Program
// lowers, sorted: the order of the #include lines of its header.
func (g *gen) importOrder() []string {
	var paths []string
	for path, dep := range g.pkg.Load.Imports {
		if o := g.prog.byLoc[dep.ImportPath]; o != nil && o != g {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	var out []string
	for _, path := range paths {
		out = append(out, g.prog.byLoc[g.pkg.Load.Imports[path].ImportPath].pkg.Load.ImportPath)
	}
	return out
}

// firstPos is the place of the package clause of the package's first file, where an error about
// the package as a whole is reported.
func (g *gen) firstPos() token.Pos {
	return g.pkg.Load.Files[0].AST.Name.Pos()
}

// unitName returns the name of the translation unit of the .gp file path: render.gp.cpp. A file
// name with a character other than ASCII letters, digits and . _ - + (a space, a quote, a dollar
// that a compiler wrapper expands, a letter that is no ASCII) gets those characters replaced by _
// and a hash of the name, so that no name of a unit is a problem for the compilers or the shell:
// "dollar$.gp" is dollar_-1a2b3c4d.gp.cpp.
func unitName(path string) string {
	base := filepath.Base(path)
	safe := true
	var b strings.Builder
	for _, r := range base {
		switch {
		case r < utf8.RuneSelf && (r == '.' || r == '_' || r == '-' || r == '+' || '0' <= r && r <= '9' || 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z'):
			b.WriteRune(r)
		default:
			safe = false
			b.WriteByte('_')
		}
	}
	if safe {
		return base + ".cpp"
	}
	sum := sha256.Sum256([]byte(base))
	name := strings.TrimSuffix(b.String(), ".gp")
	return name + "-" + hex.EncodeToString(sum[:4]) + ".gp.cpp"
}
