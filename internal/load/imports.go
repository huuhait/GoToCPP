package load

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/diag"
)

// A FileInfo is what ClassifyImports finds out about the imports of one .gp
// file. The shape of an import path says what it is:
//
//	"fmt", "myapp/engine"            a gocpp package (Packages)
//	"<raylib.h>", "<vector>"         a system header (Includes)
//	"./physics.h", "native/x.hpp"    a local header (Includes)
//	fs "::std::filesystem"           a namespace rename (NamespaceRenames)
//
// Before go/types runs, RewriteFile (step 1.8) drops every import that is not
// in Packages and rewrites the uses of header packages (raylib.X becomes C.X)
// and of namespaces (std.X becomes C.std.X, fs.path becomes C.fs.path). Its
// Rewrite records each use.
type FileInfo struct {
	// Packages are the imports of gocpp packages, in source order. They are
	// the only imports that go/types may see: step 1.8 drops all the
	// others, also the imports that ClassifyImports reported as errors.
	Packages []*ast.ImportSpec

	// PackageNames maps the name that each gocpp import gives to the file
	// to its import: the name in the import (f "fmt"), or else the name of
	// the package. A blank or a dot import gives no name, and when two
	// imports give the same name, the first one has it. For RewriteFile, a
	// name that an import gives is never a namespace.
	PackageNames map[string]*ast.ImportSpec

	// Includes are the header imports, in source order: they become
	// #include lines in this order. A header that is imported twice is
	// listed twice.
	Includes []*Include

	// HeaderNames maps the package name of each header that has one to
	// its include.
	HeaderNames map[string]*Include

	// NamespaceRenames maps the name of each namespace rename (fs) to its
	// import.
	NamespaceRenames map[string]*NamespaceRename
}

// An Include is one header import.
type Include struct {
	Kind HeaderKind

	// Path is the import path without the quotes. A system header keeps
	// its <>: "<raylib.h>", "./physics.h".
	Path string

	// Name is the package name of the header in this file: the name given
	// in the import (rl "<raylib.h>"), or else the default name, which is
	// the file name without its folder and its last extension, with the
	// characters that Go doesn't allow in names turned into _
	// ("./my-lib.h" gives my_lib). Name is "" if the header has no name.
	Name string

	// WouldBe and Why are set when Name is "": WouldBe is the name the
	// header would have (the name given in the import, else the default
	// name), and Why says why it has none. When code uses WouldBe as a
	// name anyway, RewriteFile gives a hint instead of a bare error: the
	// error for string.strlen ends in "give it a name: import cstring
	// "<string.h>"". Not for a header imported with _: there the name can
	// still be a namespace.
	WouldBe string
	Why     NoNameReason

	// Pos is the position of the import path: the place for messages
	// about the header.
	Pos token.Pos

	// Spec is the import that the header comes from, which step 1.8
	// drops from the AST.
	Spec *ast.ImportSpec
}

// A HeaderKind says how a header is included.
type HeaderKind int

const (
	// SystemHeader is a header path in <>: "<raylib.h>", "<SDL2/SDL.h>",
	// "<vector>". It becomes #include <raylib.h>.
	SystemHeader HeaderKind = iota + 1

	// LocalHeader is a path that ends in .h, .hh, .hpp, .hxx, .h++ or
	// .inl: "./physics.h", "native/x.hpp", "../shared/x.h". It is looked
	// up next to the .gp file, then in the include folders (Phase 04), and
	// becomes #include "./physics.h".
	LocalHeader
)

// String returns the name of the constant, such as "SystemHeader".
func (k HeaderKind) String() string {
	switch k {
	case SystemHeader:
		return "SystemHeader"
	case LocalHeader:
		return "LocalHeader"
	}
	return fmt.Sprintf("HeaderKind(%d)", int(k))
}

// A NamespaceRename is an import that gives a C++ namespace another name:
// fs "::std::filesystem". A namespace needs no import otherwise: every
// namespace that the headers declare can be used directly (std.vector).
type NamespaceRename struct {
	Name      string // "fs"
	Namespace string // the import path: "::std::filesystem"

	// Pos is the position of the import path: the place for messages
	// about the namespace.
	Pos token.Pos

	// Spec is the import, which step 1.8 drops from the AST.
	Spec *ast.ImportSpec
}

// A NoNameReason says why a header has no package name (Include.Why).
type NoNameReason int

const (
	// HasName means that the header has a name. It is the zero value.
	HasName NoNameReason = iota

	// Blank: the header is imported with _ (_ "<GL/gl.h>"), or its file
	// name gives the default name _ ("<_.h>").
	Blank

	// DotImport: the header is dot-imported (. "<raylib.h>"), which is an
	// error.
	DotImport

	// Keyword: the default name is a Go keyword: <map>, <sys/select.h>.
	Keyword

	// Predeclared: the name is a predeclared Go name: the default name of
	// <string>, <string.h> or <new>, or a name given in an import, which
	// is an error (string "<string.h>").
	Predeclared

	// Reserved: the name is C, which holds C's built-in types, or init,
	// which can only be a func: the default name of <C.h>, or a name given
	// in an import, which is an error (C "<x.h>").
	Reserved

	// NotAName: the default name is not a valid Go name, because it is
	// empty or starts with a digit: <3dmath.h>, <.h>.
	NotAName

	// UsedByImport: another import of the file uses the name. It is the
	// name of a gocpp package (<math.h> next to "math"), a name given in
	// another import, or the default name of a different header (<time.h>
	// and <sys/time.h>: neither gets the name). It is also the reason when
	// a later import gives a name a second time, which is an error ("rl
	// redeclared in this block"), also when it is the same header.
	UsedByImport

	// UsedByPackage: a package-level declaration of the package, in any
	// of its files, uses the name.
	UsedByPackage
)

// String returns the name of the constant, such as "Predeclared".
func (r NoNameReason) String() string {
	switch r {
	case HasName:
		return "HasName"
	case Blank:
		return "Blank"
	case DotImport:
		return "DotImport"
	case Keyword:
		return "Keyword"
	case Predeclared:
		return "Predeclared"
	case Reserved:
		return "Reserved"
	case NotAName:
		return "NotAName"
	case UsedByImport:
		return "UsedByImport"
	case UsedByPackage:
		return "UsedByPackage"
	}
	return fmt.Sprintf("NoNameReason(%d)", int(r))
}

// ClassifyImports sorts the imports of the file f into gocpp packages,
// headers and namespace renames, and gives each header its package name
// (see FileInfo). It adds an error to errs for each import that is wrong.
// Such an import is in none of the lists, except a header with a bad name,
// which is still included, with no name.
//
// pkgLevel holds the package-level names of all files of the package, with
// the position of the identifier that declares each one (see
// PackageLevelNames). A header doesn't get a default name that is a
// package-level name, and a name given to a header or a namespace must not
// be one: "rl already declared through import of <raylib.h>", reported at
// the declaration, as go/types does for packages (or at the import, if the
// position is not valid).
//
// packageName returns the name of the gocpp package with the given import
// path, from the package clause of its files, or "" if it is not known.
// The loader (step 1.9) finds the paths with PackagePaths and reads their
// package clauses first. If packageName is nil or returns "",
// ClassifyImports guesses the name from the path like the go tools do: the
// last element ("myapp/engine" gives engine), or the one before it when
// the last one is a major version ("math/rand/v2" gives rand), without a
// "go-" prefix and up to the first character that can't be in a name
// ("gopkg.in/yaml.v3" gives yaml). A guess can be wrong: the package in
// "myapp/api/v1" may be named v1.
//
// A header gets no default name when it would be a Go keyword, a
// predeclared Go name, C, init or _, not a valid name, or a name that
// another import of the file or a package-level name uses. When two
// different headers would get the same default name, neither gets it, so
// the result doesn't depend on the order of the imports. A gocpp package
// always keeps its name.
//
// A name given in an import follows Go: when two imports of the file give
// the same name, the second one is an error ("rl redeclared in this
// block"). That is also true for two imports of the same header with the
// same name, when one or both of them is the default name, but not for a
// default name and a different header: then the default name gives way.
// ClassifyImports reports it only when a header or a namespace is one of
// the two, because go/types never sees those imports; go/types reports the
// rest. The name of a header or a namespace can't be C, init or a
// predeclared Go name, and no import can give the name C.
//
// Header imports never cause "imported and not used" errors, because a
// header also brings its namespaces and macros.
func ClassifyImports(fset *token.FileSet, f *ast.File, pkgLevel map[string]token.Pos,
	packageName func(path string) string, errs *diag.List) *FileInfo {
	c := &classifier{
		fset:        fset,
		errs:        errs,
		pkgLevel:    pkgLevel,
		packageName: packageName,
		info: &FileInfo{
			PackageNames:     make(map[string]*ast.ImportSpec),
			HeaderNames:      make(map[string]*Include),
			NamespaceRenames: make(map[string]*NamespaceRename),
		},
	}
	for _, spec := range fileImports(f) {
		c.importSpec(spec)
	}
	c.defaultNames()
	c.declareNames()
	return c.info
}

// PackagePaths returns the import paths of the gocpp packages that the file
// f imports, in source order and without duplicates: the paths that
// ClassifyImports takes as gocpp packages by their shape, except "C" and
// the paths that go/types rejects ("invalid import path"). The loader
// (step 1.9) reads the package clauses of these packages before it calls
// ClassifyImports, so that it can pass their real names.
func PackagePaths(f *ast.File) []string {
	var paths []string
	seen := make(map[string]bool)
	for _, spec := range fileImports(f) {
		p, ok := importPath(spec)
		if !ok || seen[p] || p == "C" || !goImportPathOK(p) || kindOf(p) != packageImport {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}

// IsHeaderOrNamespace reports whether the import path p, without its
// quotes, has the shape of a header or a namespace import, by the rules of
// ClassifyImports: "<raylib.h>", "./physics.h", "native/x.hpp" or
// "::std::filesystem". It looks only at the shape, so a broken path such
// as "<raylib.h" or " <vector>" counts too, while "fmt" and "./physics"
// (a ./ path that is not a header) don't. gocpp fmt keeps the order of an
// import group with such an import, because headers become #include lines.
func IsHeaderOrNamespace(p string) bool {
	switch kindOf(strings.TrimSpace(p)) {
	case systemImport, localImport, namespaceImport:
		return true
	}
	return false
}

// PackageLevelNames returns the package-level names that the files of one
// package declare, with the position of the identifier that declares each
// name first: the names of funcs (not methods and not init), vars, consts
// and types, but not _. ClassifyImports needs them, and so does step 1.8
// (a name that is declared nowhere in Go is taken as a namespace).
func PackageLevelNames(files []*ast.File) map[string]token.Pos {
	names := make(map[string]token.Pos)
	add := func(id *ast.Ident) {
		if id == nil || id.Name == "_" {
			return
		}
		if _, ok := names[id.Name]; !ok {
			names[id.Name] = id.Pos()
		}
	}
	for _, f := range files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name != nil && d.Name.Name != "init" {
					add(d.Name)
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.ValueSpec:
						for _, id := range s.Names {
							add(id)
						}
					case *ast.TypeSpec:
						add(s.Name)
					}
				}
			}
		}
	}
	return names
}

// fileImports returns the import specs of f, in source order. It walks the
// declarations, not f.Imports, so that it also works on an AST that was
// not made by the parser.
func fileImports(f *ast.File) []*ast.ImportSpec {
	var specs []*ast.ImportSpec
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		for _, s := range gd.Specs {
			if spec, ok := s.(*ast.ImportSpec); ok {
				specs = append(specs, spec)
			}
		}
	}
	return specs
}

// importPath returns the path of spec without its quotes. It reports false
// if the path is missing or is not a valid string literal: the parser
// reported that, and go/types skips such an import too.
func importPath(spec *ast.ImportSpec) (string, bool) {
	if spec.Path == nil {
		return "", false
	}
	p, err := strconv.Unquote(spec.Path.Value)
	return p, err == nil
}

// goImportPathOK reports whether go/types accepts p as an import path (its
// validatedImportPath).
func goImportPathOK(p string) bool {
	const illegalChars = `!"#$%&'()*,:;<=>?[\]^{|}` + "`�"
	for _, r := range p {
		if !unicode.IsGraphic(r) || unicode.IsSpace(r) || strings.ContainsRune(illegalChars, r) {
			return false
		}
	}
	return p != ""
}

// An importKind is what an import path is, by its shape.
type importKind int

const (
	packageImport   importKind = iota // a gocpp package: "fmt", "myapp/engine"
	systemImport                      // a system header: "<raylib.h>" (also the error "raylib.h>")
	localImport                       // a local header: "./physics.h", "native/x.hpp"
	namespaceImport                   // a namespace: "::std::filesystem"
	notHeaderImport                   // an error: a ./ or ../ path that is not a header ("./physics")
)

// kindOf returns the kind of the import path p, which has no space at its
// ends.
func kindOf(p string) importKind {
	switch {
	case strings.HasPrefix(p, "<"):
		return systemImport
	case strings.Contains(p, "::"): // before the > check: "::std::vector<int>"
		return namespaceImport
	case strings.HasSuffix(p, ">"):
		return systemImport
	case isLocalHeader(p):
		return localImport
	case strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../"):
		return notHeaderImport
	}
	return packageImport
}

// classifier holds the state of one ClassifyImports call.
type classifier struct {
	fset        *token.FileSet
	errs        *diag.List
	pkgLevel    map[string]token.Pos
	packageName func(path string) string
	info        *FileInfo

	// names holds the imports that give a name to the file scope, in
	// source order. ClassifyImports declares the names at the end, once
	// the default names of the headers are known.
	names []scopeName
}

// A scopeName is an import that gives a name to the file scope: a gocpp
// package, a header (with a name given in the import, or its default
// name) or a namespace rename.
type scopeName struct {
	name      string // "" for a default name that is not known yet
	spec      *ast.ImportSpec
	inc       *Include         // the header, or nil
	rename    *NamespaceRename // the namespace rename, or nil
	isDefault bool             // the default name of a header
}

// cpp reports whether the import is a header or a namespace, which
// go/types never sees.
func (n *scopeName) cpp() bool { return n.inc != nil || n.rename != nil }

// importSpec sorts one import by the shape of its path.
func (c *classifier) importSpec(spec *ast.ImportSpec) {
	p, ok := importPath(spec)
	if !ok {
		return
	}
	if p == "" {
		c.errorAt(spec.Path, "invalid import path (empty string)") // go/types' message
		return
	}
	name := ""
	if spec.Name != nil {
		name = spec.Name.Name
	}
	kind := kindOf(strings.TrimSpace(p))
	if kind == packageImport {
		c.gocppPackage(spec, p, name) // a space in the path is go/types' error
		return
	}
	what := "header"
	switch kind {
	case namespaceImport:
		what = "namespace"
	case notHeaderImport:
		what = "import"
	}
	switch {
	case strings.TrimLeftFunc(p, unicode.IsSpace) != p:
		c.errorAt(spec.Path, "invalid %s path %s: space at the start", what, strconv.Quote(p))
	case strings.TrimRightFunc(p, unicode.IsSpace) != p:
		c.errorAt(spec.Path, "invalid %s path %s: space at the end", what, strconv.Quote(p))
	case kind == systemImport:
		c.header(spec, p, name, SystemHeader)
	case kind == localImport:
		c.header(spec, p, name, LocalHeader)
	case kind == namespaceImport:
		c.namespace(spec, p, name)
	// The cases below are for a ./ or ../ path that is not a header.
	case isSourceFile(p):
		c.errorAt(spec.Path, "cannot import %s: a C or C++ file can't be imported (import its header; "+
			"gocpp compiles the C and C++ files in the package folder)", strconv.Quote(p))
	default:
		c.errorAt(spec.Path, "invalid import path %s: a local header ends in %s", strconv.Quote(p), headerExtList)
	}
}

// gocppPackage handles the import of a gocpp package, such as "fmt".
func (c *classifier) gocppPackage(spec *ast.ImportSpec, p, name string) {
	if p == "C" {
		c.errorAt(spec.Path, `import "C" is not allowed: C's built-in types need no import in gocpp, `+
			"and there is no inline C preamble (put C code in a header next to the .gp file)")
		return
	}
	switch {
	case name == "C":
		c.errorAt(spec.Name, "cannot import package as C - C is reserved for C's built-in types")
		return
	case name == "":
		name = c.pkgName(p)
		if name == "C" {
			c.errorAt(spec.Path, "cannot import package as C - C is reserved for C's built-in types "+
				"(give it another name, like import c %s)", strconv.Quote(p))
			return
		}
	}
	c.info.Packages = append(c.info.Packages, spec)
	if name != "" && name != "_" && name != "." {
		c.names = append(c.names, scopeName{name: name, spec: spec})
	}
}

// pkgName returns the name of the gocpp package with the import path p:
// the name that packageName gives, or else the guess from the path.
func (c *classifier) pkgName(p string) string {
	if c.packageName != nil {
		if name := c.packageName(p); name != "" {
			return name
		}
	}
	return assumedName(p)
}

// header handles a header import.
func (c *classifier) header(spec *ast.ImportSpec, p, name string, kind HeaderKind) {
	if msg := checkHeaderPath(p, kind); msg != "" {
		c.errorAt(spec.Path, "invalid header path %s: %s", strconv.Quote(p), msg)
		return
	}
	inc := &Include{Kind: kind, Path: p, Pos: spec.Path.Pos(), Spec: spec}
	c.info.Includes = append(c.info.Includes, inc)
	switch name {
	case "":
		c.names = append(c.names, scopeName{spec: spec, inc: inc, isDefault: true})
		return
	case "_":
		inc.WouldBe, inc.Why = defaultName(p), Blank
		return
	case ".":
		c.errorAt(spec.Name, "cannot dot-import a header")
		inc.WouldBe, inc.Why = defaultName(p), DotImport
		return
	}
	if why, msg := chosenNameProblem("header", name); msg != "" {
		c.errorAt(spec.Name, "%s", msg)
		inc.WouldBe, inc.Why = name, why
		return
	}
	c.names = append(c.names, scopeName{name: name, spec: spec, inc: inc})
}

// namespace handles a namespace import, such as fs "::std::filesystem".
func (c *classifier) namespace(spec *ast.ImportSpec, p, name string) {
	if msg := checkNamespacePath(p); msg != "" {
		c.errorAt(spec.Path, "invalid namespace path %s: %s", strconv.Quote(p), msg)
		return
	}
	switch name {
	case "":
		// The example name is the last part of the path, or c and the
		// part for a one-part path, which already has that name.
		parts := strings.Split(p[2:], "::")
		short := parts[len(parts)-1]
		if len(parts) == 1 || nameProblem(short) != HasName {
			short = "c" + short
		}
		rename := fmt.Sprintf("import %s %s", short, strconv.Quote(p))
		if why := nameProblem(parts[0]); why != HasName {
			// The namespace can't be used directly: go.X and error.X are
			// not namespace uses in Go code.
			c.errorAt(spec.Path, "namespace %s needs a new name, because %s: %s", p, notANameText(parts[0], why), rename)
			return
		}
		c.errorAt(spec.Path, "namespace %s needs no import: use %s.X directly, or rename it: %s",
			p, strings.Join(parts, "."), rename)
		return
	case "_":
		c.errorAt(spec.Name, "cannot import namespace as _ - a namespace import only gives it a new name")
		return
	case ".":
		c.errorAt(spec.Name, "cannot dot-import a namespace")
		return
	}
	if _, msg := chosenNameProblem("namespace", name); msg != "" {
		c.errorAt(spec.Name, "%s", msg)
		return
	}
	rename := &NamespaceRename{Name: name, Namespace: p, Pos: spec.Path.Pos(), Spec: spec}
	c.names = append(c.names, scopeName{name: name, spec: spec, rename: rename})
}

// chosenNameProblem returns the error for a name given in the import of a
// header or a namespace (what), and the reason that the header has no
// name, or "" if the name is fine. A predeclared name is an error, unlike
// for a Go package: go/types never sees the import, so it couldn't hide
// the predeclared name in the file, as Go would. The init message is the
// one that go/types gives for a package.
func chosenNameProblem(what, name string) (NoNameReason, string) {
	switch {
	case name == "C":
		return Reserved, fmt.Sprintf("cannot import %s as C - C is reserved for C's built-in types", what)
	case name == "init":
		return Reserved, fmt.Sprintf("cannot import %s as init - init must be a func", what)
	case types.Universe.Lookup(name) != nil:
		return Predeclared, fmt.Sprintf("cannot import %s as %s - %s is a predeclared Go name "+
			"(use another name, like c%s)", what, name, name, name)
	}
	return HasName, ""
}

// redeclared reports that the import spec gives the name that the earlier
// import first gives too, with go/types' messages.
func (c *classifier) redeclared(name string, spec, first *ast.ImportSpec) {
	end := spec.Path.End()
	if spec.Name != nil {
		end = spec.Name.End()
	}
	c.errs.Add(diag.Diagnostic{
		Pos:     c.fset.Position(spec.Pos()),
		End:     c.fset.Position(end),
		Msg:     name + " redeclared in this block",
		Related: []diag.Related{{Pos: c.fset.Position(first.Pos()), Msg: "other declaration of " + name}},
	})
}

// checkPackageLevel reports an error if the name that spec gives to a
// header or a namespace (what: "<raylib.h>", "\"./x.h\"", "::std") is a
// package-level name. Like go/types, it reports the error at the
// package-level declaration, with the import as the other declaration.
func (c *classifier) checkPackageLevel(name string, spec *ast.ImportSpec, what string) {
	pos, ok := c.pkgLevel[name]
	if !ok {
		return
	}
	msg := name + " already declared through import of " + what
	if !pos.IsValid() {
		c.errs.Add(diag.Diagnostic{Pos: c.fset.Position(spec.Pos()), End: c.fset.Position(spec.Name.End()), Msg: msg})
		return
	}
	c.errs.Add(diag.Diagnostic{
		Pos:     c.fset.Position(pos),
		End:     c.fset.Position(pos + token.Pos(len(name))), // the end of the identifier
		Msg:     msg,
		Related: []diag.Related{{Pos: c.fset.Position(spec.Pos()), Msg: "other declaration of " + name}},
	})
}

// defaultNames gives the headers that are imported without a name their
// default names, where the rules allow it. A default name gives way to a
// name that another import gives (but not to a name given to the same
// header: that is a second declaration), to the default name of a
// different header (then neither header gets the name), and to a
// package-level name.
func (c *classifier) defaultNames() {
	given := make(map[string][]*scopeName)    // the names other than default names
	paths := make(map[string]map[string]bool) // the headers that would get each default name
	for i := range c.names {
		n := &c.names[i]
		if !n.isDefault {
			given[n.name] = append(given[n.name], n)
			continue
		}
		n.name = defaultName(n.inc.Path)
		n.inc.WouldBe, n.inc.Why = n.name, nameProblem(n.name)
		if n.inc.Why == HasName {
			if paths[n.name] == nil {
				paths[n.name] = make(map[string]bool)
			}
			paths[n.name][n.inc.Path] = true
		}
	}
	for i := range c.names {
		n := &c.names[i]
		if !n.isDefault || n.inc.Why != HasName {
			continue
		}
		otherImport := slices.ContainsFunc(given[n.name], func(o *scopeName) bool {
			return o.inc == nil || o.inc.Path != n.inc.Path // not the same header
		})
		_, declared := c.pkgLevel[n.name]
		switch {
		case otherImport || len(paths[n.name]) > 1:
			n.inc.Why = UsedByImport
		case declared:
			n.inc.Why = UsedByPackage
		}
	}
}

// notANameText says why name can't be used as a name in Go code, for a
// message. why is what nameProblem returns for it.
func notANameText(name string, why NoNameReason) string {
	switch {
	case why == Keyword:
		return name + " is a Go keyword"
	case why == Predeclared:
		return name + " is a predeclared Go name"
	case why == Blank:
		return "_ is the blank identifier"
	case name == "C":
		return "C is reserved for C's built-in types"
	case name == "init":
		return "init can only be a func"
	}
	return name + " is not a valid Go name"
}

// declareNames declares the names of the imports in the file scope, in
// source order, like go/types does: the first import of a name gets it,
// and a later one is an error ("redeclared"), unless both are gocpp
// packages, which go/types checks itself. That is also the case when the
// same header is imported twice with the same name, given in the import
// or not. A name given to a header or a namespace must not be a
// package-level name either.
func (c *classifier) declareNames() {
	declared := make(map[string]*scopeName)
	for i := range c.names {
		n := &c.names[i]
		if n.isDefault && n.inc.Why != HasName {
			continue // the header has no name
		}
		if first, ok := declared[n.name]; ok {
			if n.cpp() || first.cpp() {
				c.redeclared(n.name, n.spec, first.spec)
			}
			if n.inc != nil {
				n.inc.WouldBe, n.inc.Why = n.name, UsedByImport
			}
			continue
		}
		declared[n.name] = n
		switch {
		case n.inc != nil:
			n.inc.Name, n.inc.WouldBe = n.name, ""
			c.info.HeaderNames[n.name] = n.inc
			if !n.isDefault {
				what := n.inc.Path
				if n.inc.Kind == LocalHeader {
					what = strconv.Quote(what)
				}
				c.checkPackageLevel(n.name, n.spec, what)
			}
		case n.rename != nil:
			c.info.NamespaceRenames[n.name] = n.rename
			c.checkPackageLevel(n.name, n.spec, n.rename.Namespace)
		default:
			c.info.PackageNames[n.name] = n.spec
		}
	}
}

// errorAt adds an error at the node n.
func (c *classifier) errorAt(n ast.Node, format string, args ...any) {
	c.errs.Add(diag.Diagnostic{
		Pos: c.fset.Position(n.Pos()),
		End: c.fset.Position(n.End()),
		Msg: fmt.Sprintf(format, args...),
	})
}

// headerExts are the extensions of a local header path, and headerExtList
// lists them for messages.
var headerExts = []string{".h", ".hh", ".hpp", ".hxx", ".h++", ".inl"}

const headerExtList = ".h, .hh, .hpp, .hxx, .h++ or .inl"

// isLocalHeader reports whether the import path p ends in the extension of
// a header.
func isLocalHeader(p string) bool {
	for _, ext := range headerExts {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// isSourceFile reports whether the path p ends in the extension of a C or
// C++ source file, which gocpp compiles when it is next to the .gp files
// (Phase 04).
func isSourceFile(p string) bool {
	for _, ext := range []string{".c", ".cc", ".cpp", ".cxx"} {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// checkHeaderPath returns what is wrong with the header path p, or "".
// A system header path is <file>, without a space after < or before >.
// A header path can't hold a character that can't be written in an
// #include line: a control character, a > (or <) in a system header, or a
// " in a local header. A \ gets its own message, because C and C++ want /
// in include paths on every system.
func checkHeaderPath(p string, kind HeaderKind) string {
	file := p
	if kind == SystemHeader {
		switch {
		case !strings.HasPrefix(p, "<"):
			return "missing < at the start"
		case len(p) == 1 || !strings.HasSuffix(p, ">"):
			return "missing > at the end"
		}
		file = p[1 : len(p)-1]
		switch {
		case strings.TrimSpace(file) == "":
			return "missing file name"
		case strings.TrimLeftFunc(file, unicode.IsSpace) != file:
			return "space after <"
		case strings.TrimRightFunc(file, unicode.IsSpace) != file:
			return "space before >"
		}
	}
	for _, r := range file {
		switch {
		case r == '\\':
			return "use / in header paths"
		case r == utf8.RuneError || !unicode.IsGraphic(r),
			kind == SystemHeader && (r == '<' || r == '>'),
			kind == LocalHeader && r == '"':
			return fmt.Sprintf("invalid character %#U", r)
		}
	}
	return ""
}

// checkNamespacePath returns what is wrong with the namespace path p, or
// "". A namespace path is :: and C++ names joined with ::, as in
// "::std::filesystem".
func checkNamespacePath(p string) string {
	rest, ok := strings.CutPrefix(p, "::")
	if !ok {
		if checkNamespacePath("::"+p) == "" {
			return fmt.Sprintf("it must start with ::, like %q", "::"+p)
		}
		return "it must start with ::"
	}
	for part := range strings.SplitSeq(rest, "::") {
		if part == "" {
			return "missing name after ::"
		}
		if !isCppName(part) {
			return fmt.Sprintf("%q is not a C++ name", part)
		}
	}
	return ""
}

// isCppName reports whether s is a C++ identifier: a letter or _, then
// letters, digits and _. Like Go, it allows non-ASCII letters and digits.
// It doesn't check for C++ keywords; the C++ compiler does that.
func isCppName(s string) bool {
	for i, r := range s {
		if !isNameChar(r) || i == 0 && unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// isNameChar reports whether r may be in a Go name: a letter, a digit or _.
func isNameChar(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// defaultName returns the default package name of the header path p: the
// file name without its folder and its last extension, with each
// character that Go doesn't allow in names turned into _. It can be an
// invalid name: nameProblem checks it.
func defaultName(p string) string {
	file := strings.TrimSuffix(strings.TrimPrefix(p, "<"), ">")
	file = file[strings.LastIndexByte(file, '/')+1:]
	if i := strings.LastIndexByte(file, '.'); i >= 0 {
		file = file[:i]
	}
	return strings.Map(func(r rune) rune {
		if isNameChar(r) {
			return r
		}
		return '_'
	}, file)
}

// nameProblem says why name can't be the default name of a header, or
// returns HasName if it can be one. name has only the characters that
// defaultName leaves.
func nameProblem(name string) NoNameReason {
	r, _ := utf8.DecodeRuneInString(name)
	switch {
	case name == "" || unicode.IsDigit(r):
		return NotAName
	case name == "_":
		return Blank
	case token.IsKeyword(name):
		return Keyword
	case types.Universe.Lookup(name) != nil:
		return Predeclared
	case name == "C" || name == "init":
		return Reserved
	}
	return HasName
}

// assumedName returns the name that the go tools assume for the package
// with the import path p (golang.org/x/tools' ImportPathToAssumedName):
// the last element of the path, or the one before it when the last one is
// a major version such as v2, without a "go-" prefix and up to the first
// character that can't be in a name. It can be "".
func assumedName(p string) string {
	base := path.Base(p)
	if v, ok := strings.CutPrefix(base, "v"); ok {
		if _, err := strconv.Atoi(v); err == nil {
			if dir := path.Dir(p); dir != "." {
				base = path.Base(dir)
			}
		}
	}
	base = strings.TrimPrefix(base, "go-")
	if i := strings.IndexFunc(base, func(r rune) bool { return !isNameChar(r) }); i >= 0 {
		base = base[:i]
	}
	return base
}
