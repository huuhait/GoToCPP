package load

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
)

// A Rewrite is what RewriteFile changed in one file. The later steps read
// it: the namespace probe (Namespaces and Renames), the diagnostics (to show
// what the user wrote instead of C.X), codegen (the C++ name of each use)
// and the editor (hover and go to definition on raylib, std or fs).
type Rewrite struct {
	// Uses lists the uses of header packages and namespaces that
	// RewriteFile rewrote, in source order.
	Uses []*CppUse

	// Namespaces lists the names that RewriteFile took as C++ namespaces
	// (std in std.vector, where Go declares std nowhere), in the order of
	// their first use. The namespace probe checks that the headers of the
	// file declare them. For a name that they don't declare, such as the
	// typo stdd, the caller reports "undefined: stdd" at each use, as
	// go/types would have done. A namespace rename such as fs is not in
	// this list: it is in Renames.
	Namespaces []*NamespaceCandidate

	// Renames lists the namespace renames of the file
	// (fs "::std::filesystem"), in source order, used or not, so that the
	// namespace probe can check their paths too (and report a bad one at
	// its Pos).
	Renames []*NamespaceRename

	// ImportC is the import "C" that RewriteFile added because the file
	// uses C, or nil.
	ImportC *ast.ImportSpec
}

// A CppUse is a use of a header package or a C++ namespace: the name X of
// a selector X.Name, which RewriteFile replaced by C or by a selector on C,
// so that go/types (with FakeImportC) takes the selector as opaque.
type CppUse struct {
	Kind CppUseKind

	// Ident is the name that the user wrote: raylib in raylib.InitWindow,
	// std in std.vector, fs in fs.path, string in string.strlen. Its
	// position is the position of the use. In a NamespaceUse and in a Bare
	// use it stays in the AST, as the Sel of the new C.std, C.fs or
	// C.raylib; the other kinds take it out of the AST.
	Ident *ast.Ident

	// Sel is the selector whose X was Ident (or, in a Bare use, the new
	// selector that replaced Ident). After the rewrite, Sel.X is C or a
	// selector on C, and each identifier in it has the position of Ident.
	// In std.chrono.seconds, Sel is std.chrono, which becomes C.std.chrono.
	Sel *ast.SelectorExpr

	// Path is Sel.X after the rewrite, as Go source: "C" for a header,
	// "C.std" for std, "C.fs" for fs (whose C++ path is in Namespace).
	Path string

	// Include is the header of a HeaderUse, or the header without a name of
	// an UnnamedHeaderUse. It is nil for a NamespaceUse.
	Include *Include

	// Namespace is the C++ namespace of a NamespaceUse: "::std" for
	// std.vector, "::std::filesystem" for fs.path. It is "" for the other
	// kinds.
	Namespace string

	// Rename is the namespace rename that a NamespaceUse comes through
	// (fs), or nil.
	Rename *NamespaceRename

	// Bare is set when a header package or a namespace rename is used
	// without a selector (x := raylib), which is an error. RewriteFile
	// reports it, as go/types does for a Go package, and replaces Ident by
	// a new selector, Sel, on C: C.raylib, C.fs. So go/types reports
	// nothing more.
	Bare bool
}

// A CppUseKind says what the name of a CppUse is.
type CppUseKind int

const (
	// HeaderUse: the name is the package name of a header, such as raylib
	// for "<raylib.h>". raylib.InitWindow becomes C.InitWindow.
	HeaderUse CppUseKind = iota + 1

	// NamespaceUse: the name is a C++ namespace. std.vector becomes
	// C.std.vector, and with the rename fs "::std::filesystem", fs.path
	// becomes C.fs.path (Namespace says ::std::filesystem).
	NamespaceUse

	// UnnamedHeaderUse: the name is the name that a header would have if
	// it had one, such as string for "<string.h>". RewriteFile reports an
	// error that says how to give the header a name, and string.strlen
	// becomes C.strlen, so that go/types doesn't report a second error.
	UnnamedHeaderUse
)

// String returns the name of the constant, such as "HeaderUse".
func (k CppUseKind) String() string {
	switch k {
	case HeaderUse:
		return "HeaderUse"
	case NamespaceUse:
		return "NamespaceUse"
	case UnnamedHeaderUse:
		return "UnnamedHeaderUse"
	}
	return fmt.Sprintf("CppUseKind(%d)", int(k))
}

// A NamespaceCandidate is a name that RewriteFile took as a C++ namespace
// (see Rewrite.Namespaces).
type NamespaceCandidate struct {
	Name string    // std, for the namespace ::std
	Uses []*CppUse // its uses, in source order
}

// RewriteFile prepares the file f for go/types with FakeImportC (Phase 01
// README §2.5, steps 1, 2, 3 and 6). It changes f in place:
//
//   - It drops every import that is not in info.Packages: the headers, the
//     namespace renames and the imports with errors. An import declaration
//     that becomes empty goes away, and f.Imports lists what is left.
//   - raylib.InitWindow, where raylib is the package name of a header,
//     becomes C.InitWindow.
//   - fs.path, where fs is a namespace rename (fs "::std::filesystem"),
//     becomes C.fs.path, like a namespace: the C++ path is in the use's
//     Namespace.
//   - std.vector, where Go declares std nowhere, becomes C.std.vector: std
//     is taken as a C++ namespace (Rewrite.Namespaces). This is done only
//     in a file that imports a header. In other files, go/types reports
//     "undefined: std".
//   - string.strlen, where string is the name that a header without a
//     name would have (string for "<string.h>"), gets an error with a hint
//     ("<string.h> has no package name, because string is a predeclared Go
//     name: give it a name: import cstring "<string.h>"") and becomes
//     C.strlen, so that go/types reports nothing more. A header imported
//     with _ gets no hint: its name can still be a namespace. A name that
//     two headers would have can be a namespace too (<glm/glm.hpp> and
//     "./gl/glm.h" both lose glm, and ::glm exists), but it still gets the
//     hint for now.
//   - raylib or fs without a selector gets go/types' error for a package
//     ("use of package raylib not in selector") and becomes a selector on
//     C (see CppUse.Bare). A namespace without a selector stays as it is.
//   - It adds import "C" if the file uses C, as the user wrote it or as the
//     rewrites made it.
//
// Only the X of a selector X.Name changes, and only when X is a name:
// std.chrono.seconds becomes C.std.chrono.seconds. That is the same in
// every place, also in types (var v std.vector[int], *SDL.SDL_Window) and
// in composite literals.
//
// Go's scoping decides what a name is. A name that Go declares where it is
// used is never rewritten: a local, a parameter, result or receiver, a
// type parameter, a range or type switch variable, a package-level name of
// any file of the package (pkgLevel, see PackageLevelNames), the name of a
// gocpp import of the file, or a name that a dot import brings into the
// file (dotNames: the exported names of each package that f imports with
// ., which the caller knows once it has imported them). A predeclared
// name, such as len, is not a namespace. Labels, struct fields, methods
// and the keys of composite literals are not in that scope, so they hide
// nothing, and x.std is never a use of std. The variable of a type switch
// is not in scope in its case lists, as in Go, so in
// switch std := v.(type) { case std.string: } the case is a namespace use.
//
// Each new identifier has the position of the name that it replaces, so
// go/types reports errors at what the user wrote, and each new node lies
// inside the text of that name (the new C shares its first character). The import "C" is at the
// import keyword of the first import declaration that is left, or in a new
// declaration at the package keyword: at text that holds no other node, so
// the declarations stay in source order.
//
// f must come from the parser fork (internal/syntax/parser), with its object
// resolution: RewriteFile reads Ident.Obj and f.Unresolved to know the names
// that the file declares, and it doesn't update them. It panics if f was
// parsed with SkipObjectResolution. info is what ClassifyImports returned
// for f. RewriteFile runs once for each parsed file, and ClassifyImports
// doesn't run again on the rewritten file: it would take the import "C" for
// one that the user wrote.
func RewriteFile(fset *token.FileSet, f *ast.File, info *FileInfo, pkgLevel map[string]token.Pos,
	dotNames map[string]bool, errs *diag.List) *Rewrite {
	if f.Scope == nil {
		panic("load.RewriteFile: the file was parsed without object resolution")
	}
	r := &rewriter{
		fset:       fset,
		info:       info,
		pkgLevel:   pkgLevel,
		dotNames:   dotNames,
		errs:       errs,
		unresolved: make(map[*ast.Ident]bool, len(f.Unresolved)),
		wouldBe:    make(map[string][]*Include),
		hidden:     make(map[any]bool),
		namespaces: make(map[string]*NamespaceCandidate),
		rw:         new(Rewrite),
	}
	usesC := false
	for _, id := range f.Unresolved {
		r.unresolved[id] = true
		usesC = usesC || id.Name == "C"
	}
	for _, inc := range info.Includes {
		if inc.Name == "" && inc.Why != Blank && inc.WouldBe != "" {
			r.wouldBe[inc.WouldBe] = append(r.wouldBe[inc.WouldBe], inc)
		}
	}

	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			continue
		}
		r.walk(d)
	}

	dropImports(f, info.Packages)
	if usesC || len(r.rw.Uses) > 0 {
		r.rw.ImportC = addImportC(f)
	}
	f.Imports = fileImports(f)

	for _, rn := range info.NamespaceRenames {
		r.rw.Renames = append(r.rw.Renames, rn)
	}
	slices.SortFunc(r.rw.Renames, func(a, b *NamespaceRename) int { return cmp.Compare(a.Pos, b.Pos) })
	return r.rw
}

// rewriter holds the state of one RewriteFile call.
type rewriter struct {
	fset     *token.FileSet
	info     *FileInfo
	pkgLevel map[string]token.Pos
	dotNames map[string]bool
	errs     *diag.List

	// unresolved holds f.Unresolved: the names that the file uses but
	// doesn't declare.
	unresolved map[*ast.Ident]bool

	// wouldBe holds the headers without a name that get the hint, by the
	// name that they would have.
	wouldBe map[string][]*Include

	// hidden holds the := of each type switch whose case lists the walk is
	// in: the variable that it declares is not in scope there. The parser
	// makes it the Decl of the variable's Ident.Obj.
	hidden map[any]bool

	// stack holds the nodes from the declaration down to the node that the
	// walk is at, for the parent of a name without a selector.
	stack []ast.Node

	namespaces map[string]*NamespaceCandidate // Rewrite.Namespaces by name
	rw         *Rewrite
}

// walk rewrites the uses in n and below it.
func (r *rewriter) walk(n ast.Node) {
	ast.Inspect(n, r.visit)
}

// visit is the function of the walk: it keeps the stack, and it rewrites n
// if it is a use.
func (r *rewriter) visit(n ast.Node) bool {
	if n == nil {
		r.stack = r.stack[:len(r.stack)-1]
		return false
	}
	switch n := n.(type) {
	case *ast.SelectorExpr:
		if id, ok := n.X.(*ast.Ident); ok && r.selector(id, n) {
			return false // n.X is new, and n.Sel is a name
		}
	case *ast.Ident:
		r.bare(n)
	case *ast.TypeSwitchStmt:
		r.typeSwitch(n)
		return false
	}
	r.stack = append(r.stack, n)
	return true
}

// typeSwitch walks the type switch s. Go declares its variable (std in
// switch std := v.(type)) in each clause, after the case list. The
// parser's resolver declares it over the case lists too, so there a name
// that the parser resolved to the variable is not taken as declared.
func (r *rewriter) typeSwitch(s *ast.TypeSwitchStmt) {
	define, _ := s.Assign.(*ast.AssignStmt) // switch std := v.(type), or nil
	r.stack = append(r.stack, s)
	if s.Init != nil {
		r.walk(s.Init)
	}
	if s.Assign != nil {
		r.walk(s.Assign)
	}
	if s.Body != nil {
		r.stack = append(r.stack, s.Body)
		for _, stmt := range s.Body.List {
			cc, ok := stmt.(*ast.CaseClause)
			if !ok {
				r.walk(stmt) // not in a file that the parser made
				continue
			}
			r.stack = append(r.stack, cc)
			if define != nil {
				r.hidden[define] = true
			}
			for _, e := range cc.List {
				r.walk(e)
			}
			delete(r.hidden, define)
			for _, b := range cc.Body {
				r.walk(b)
			}
			r.stack = r.stack[:len(r.stack)-1]
		}
		r.stack = r.stack[:len(r.stack)-1]
	}
	r.stack = r.stack[:len(r.stack)-1]
}

// selector rewrites the selector sel, whose X is the name id, if id is a
// header package or a namespace, and reports whether it did. The walk
// visits the outer selector of a.b.c first, then a.b.
func (r *rewriter) selector(id *ast.Ident, sel *ast.SelectorExpr) bool {
	u := r.useOf(id, sel)
	if u == nil {
		return false
	}
	u.Ident, u.Sel = id, sel
	sel.X = newX(u)
	r.rw.Uses = append(r.rw.Uses, u)
	if u.Kind == NamespaceUse && u.Rename == nil {
		cand := r.namespaces[id.Name]
		if cand == nil {
			cand = &NamespaceCandidate{Name: id.Name}
			r.namespaces[id.Name] = cand
			r.rw.Namespaces = append(r.rw.Namespaces, cand)
		}
		cand.Uses = append(cand.Uses, u)
	}
	return true
}

// useOf returns the use that the name id, the X of the selector sel, is,
// without Ident and Sel, or nil if it is no use: the name is C or _, Go
// declares it, it is predeclared, or the file imports no header.
func (r *rewriter) useOf(id *ast.Ident, sel *ast.SelectorExpr) *CppUse {
	name := id.Name
	if name == "C" || name == "_" || r.declared(id) {
		return nil
	}
	if u := r.importUse(name); u != nil {
		return u
	}
	if incs := r.wouldBe[name]; len(incs) > 0 && !goSelector(name, sel.Sel) {
		r.errorAt(id, r.noNameHint(incs))
		return &CppUse{Kind: UnnamedHeaderUse, Include: incs[0], Path: "C"}
	}
	if types.Universe.Lookup(name) != nil || len(r.info.Includes) == 0 {
		return nil
	}
	return &CppUse{Kind: NamespaceUse, Namespace: "::" + name, Path: "C." + name}
}

// importUse returns the use, without Ident and Sel, of the name that a
// header or a namespace rename of the file has, or nil.
func (r *rewriter) importUse(name string) *CppUse {
	if inc := r.info.HeaderNames[name]; inc != nil {
		return &CppUse{Kind: HeaderUse, Include: inc, Path: "C"}
	}
	if rn := r.info.NamespaceRenames[name]; rn != nil {
		return &CppUse{Kind: NamespaceUse, Namespace: rn.Namespace, Rename: rn, Path: "C." + name}
	}
	return nil
}

// newX returns the X that replaces u.Ident, at its position: C for a
// header, C.std for std and C.fs for fs, with u.Ident as their Sel.
func newX(u *CppUse) ast.Expr {
	c := &ast.Ident{NamePos: u.Ident.Pos(), Name: "C"}
	if u.Kind == NamespaceUse {
		return &ast.SelectorExpr{X: c, Sel: u.Ident}
	}
	return c
}

// bare reports and rewrites the name id if it is a header package or a
// namespace rename that is used without a selector: x := raylib becomes
// x := C.raylib. Such a name before a dot doesn't get here (selector
// rewrote it), and a name after a dot is not in f.Unresolved.
func (r *rewriter) bare(id *ast.Ident) {
	u := r.importUse(id.Name)
	if u == nil || len(r.stack) == 0 || r.declared(id) {
		return
	}
	r.errorAt(id, "use of package "+id.Name+" not in selector")
	u.Ident, u.Bare, u.Path = id, true, "C"
	u.Sel = &ast.SelectorExpr{X: &ast.Ident{NamePos: id.Pos(), Name: "C"}, Sel: id}
	if replaceChild(r.stack[len(r.stack)-1], id, u.Sel) {
		r.rw.Uses = append(r.rw.Uses, u)
	}
}

// declared reports whether Go declares the name id where it is used: as a
// local or a package-level name of this file (the parser resolved it, and
// it is not a hidden type switch variable), as a type parameter of a
// method's receiver (the parser found it but left Ident.Obj nil, see
// go.dev/issue/50956; such a name is not in f.Unresolved either), as a
// package-level name of another file, as the name of a gocpp import, or as
// a name that a dot import brings in. A name that the parser didn't resolve
// at all is taken as declared too, so it stays as it is: that is only the
// X of a non-name on the left of := (raylib.X := 1), which go/types
// reports.
func (r *rewriter) declared(id *ast.Ident) bool {
	switch {
	case id.Obj != nil:
		if !r.hidden[id.Obj.Decl] {
			return true
		}
	case !r.unresolved[id]:
		return true
	}
	_, pkgLevel := r.pkgLevel[id.Name]
	_, imported := r.info.PackageNames[id.Name]
	return pkgLevel || imported || r.dotNames[id.Name]
}

// errorAt adds the error msg at the name id.
func (r *rewriter) errorAt(id *ast.Ident, msg string) {
	r.errs.Add(diag.Diagnostic{Pos: r.fset.Position(id.Pos()), End: r.fset.Position(id.End()), Msg: msg})
}

// goSelector reports whether name.sel is valid Go, where name is a
// predeclared name. Only a method expression on a predeclared type is:
// error.Error.
func goSelector(name string, sel *ast.Ident) bool {
	tn, ok := types.Universe.Lookup(name).(*types.TypeName)
	if !ok || sel == nil {
		return false
	}
	obj, _, _ := types.LookupFieldOrMethod(tn.Type(), false, nil, sel.Name)
	return obj != nil
}

// noNameHint returns the error for a use of the name that the headers incs
// would have if they had one (string for "<string.h>"). It is about the
// first of them, which is the first in source order.
func (r *rewriter) noNameHint(incs []*Include) string {
	inc := incs[0]
	var why string
	switch inc.Why {
	case Predeclared:
		why = ", because " + inc.WouldBe + " is a predeclared Go name"
	case Reserved:
		why = ", because " + notANameText(inc.WouldBe, inc.Why) // init can only be a func
	case DotImport:
		why = ", because it is dot-imported"
	case UsedByImport:
		// No other import gives the name (RewriteFile looks at those
		// first), so other headers would have the same default name.
		var others []string
		for _, o := range incs[1:] {
			if o.Path != inc.Path && !slices.Contains(others, headerText(o)) {
				others = append(others, headerText(o))
			}
		}
		if len(others) > 0 {
			why = ", because " + strings.Join(others, " and ") + " would have the same name"
		}
	}
	give := "a name"
	if (inc.Why == Predeclared || inc.Why == Reserved) && inc.Spec != nil && inc.Spec.Name != nil {
		give = "another name" // the name chosen in the import is an error
	}
	return fmt.Sprintf("%s has no package name%s: give it %s: import %s %s",
		headerText(inc), why, give, r.suggestion(inc), strconv.Quote(inc.Path))
}

// suggestion returns the name that the hint suggests for the header inc:
// c and the name that it would have (cstring for <string.h>), or for a
// dot import that name itself, if it is free. Else it is the first free
// name of c<name>2, c<name>3, and so on.
func (r *rewriter) suggestion(inc *Include) string {
	if inc.Why == DotImport && r.free(inc.WouldBe) {
		return inc.WouldBe
	}
	base := "c" + inc.WouldBe
	for i := 1; ; i++ { // it ends: the file and the package use only so many names
		name := base
		if i > 1 {
			name += strconv.Itoa(i)
		}
		if r.free(name) {
			return name
		}
	}
}

// free reports whether a header can get the name: a valid name that no
// import of the file, no dot import and no package-level name uses.
func (r *rewriter) free(name string) bool {
	_, pkgLevel := r.pkgLevel[name]
	return nameProblem(name) == HasName && !pkgLevel && !r.dotNames[name] && r.info.PackageNames[name] == nil &&
		r.info.HeaderNames[name] == nil && r.info.NamespaceRenames[name] == nil
}

// headerText returns the path of the header inc as messages show it:
// <raylib.h> for a system header, "./physics.h" for a local one.
func headerText(inc *Include) string {
	if inc.Kind == LocalHeader {
		return strconv.Quote(inc.Path)
	}
	return inc.Path
}

var (
	exprType  = reflect.TypeFor[ast.Expr]()
	exprsType = reflect.TypeFor[[]ast.Expr]()
)

// replaceChild replaces old by repl in parent, in a field of type ast.Expr
// or []ast.Expr, and reports whether it found old. It works with
// reflection, like the Replace of golang.org/x/tools' astutil.Apply: a
// name in an expression is always in such a field.
func replaceChild(parent ast.Node, old *ast.Ident, repl ast.Expr) bool {
	v := reflect.ValueOf(parent)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return false
	}
	v = v.Elem()
	for i := range v.NumField() {
		switch f := v.Field(i); f.Type() {
		case exprType:
			if !f.IsNil() && f.Interface() == ast.Expr(old) {
				f.Set(reflect.ValueOf(repl))
				return true
			}
		case exprsType:
			for j := range f.Len() {
				if e := f.Index(j); !e.IsNil() && e.Interface() == ast.Expr(old) {
					e.Set(reflect.ValueOf(repl))
					return true
				}
			}
		}
	}
	return false
}

// dropImports removes the import specs of f that are not in keep, and the
// import declarations that are empty then.
func dropImports(f *ast.File, keep []*ast.ImportSpec) {
	var decls []ast.Decl
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			decls = append(decls, d)
			continue
		}
		var specs []ast.Spec
		for _, s := range gd.Specs {
			if spec, ok := s.(*ast.ImportSpec); ok && slices.Contains(keep, spec) {
				specs = append(specs, s)
			}
		}
		if len(specs) > 0 {
			gd.Specs = specs
			decls = append(decls, gd)
		}
	}
	f.Decls = decls
}

// addImportC adds import "C" to the first import declaration of f, at its
// import keyword, or else in a new declaration at the package keyword,
// before the other declarations. It returns the new import.
func addImportC(f *ast.File) *ast.ImportSpec {
	newSpec := func(pos token.Pos) *ast.ImportSpec {
		return &ast.ImportSpec{Path: &ast.BasicLit{ValuePos: pos, Kind: token.STRING, Value: `"C"`}}
	}
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			// At the end of the list: the end of a declaration without
			// parentheses is the end of its first import.
			spec := newSpec(gd.TokPos)
			gd.Specs = append(gd.Specs, spec)
			return spec
		}
	}
	spec := newSpec(f.Package)
	gd := &ast.GenDecl{TokPos: f.Package, Tok: token.IMPORT, Specs: []ast.Spec{spec}}
	f.Decls = slices.Insert(f.Decls, 0, ast.Decl(gd))
	return spec
}
