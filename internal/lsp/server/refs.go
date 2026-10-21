package server

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/types/objectpath"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/lsp/safe"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// textDocument/references, textDocument/documentHighlight and (rename.go)
// textDocument/rename (step 11.8).
//
// What a name stands for is a go/types object. A package's objects live in
// that package's check, and its importers see them again as objects of their
// own, read from export data, so the same declaration is a different object
// in every check. The search therefore takes the object at the cursor in the
// document's check and turns it into an objKey: the import path of its
// package and its objectpath (golang.org/x/tools/go/types/objectpath, the
// way gopls finds an object in another package's types too). In each package
// that can mention the name, the key is turned back into that check's
// object, and the uses and definitions of that object are the references.
//
// Where it looks. A local name, or an unexported one, is seen only in its own
// package. An exported package-level name is seen in the packages that
// import its package. An exported method or field is seen wherever a value of
// its type can come to: in every package of the module. The module is the one
// of the document's package (load.FindModule); a document without a module
// has its own package only (nothing else can import it). Other modules of the
// workspace are not searched (the plan has one module until Phase 06).
//
// What it also looks for (closure):
//   - a method that implements an interface method, or that an interface
//     method of the workspace is implemented by, stands for all of them (a
//     rename of one of them alone would no longer compile): the search follows
//     the implements relation to a fixed point, like gopls. The interfaces
//     of the standard library count for references and block a rename.
//   - an embedded field is its type's name, and the type's name stands for the
//     embedded fields of that type (x.T): renaming the type renames them too.
//   - a name of a type switch (switch x := v.(type)) is the one declaration
//     of go/types' many objects, one per clause.
//
// Test files (_test.gp) are not part of the checks of the pipeline: a package
// whose test files mention the name is loaded again with them
// (cache.Snapshot.CheckTests, from source, slow: only then), and the names of
// the test files are found there by the place of their declaration.
//
// C/C++ names. A name that comes from a header (raylib.InitWindow, std.vector
// and the members of C++ values) is not a name gocpp declares, so there is no
// declaration to find the references of or to rename: references and
// highlights answer nothing for it, and prepareRename and rename refuse it
// with a message. The Go names around it (a variable that holds a C value, a
// function that returns one, a Go struct with a C field) are ordinary.

func (s *Server) references(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.ReferenceParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	q, err := newRefQuery(ctx, e.snapshot, p.TextDocument.URI, p.Position)
	if err != nil {
		return nil, err
	}
	locs := []protocol.Location{}
	if q == nil {
		return locs, nil
	}
	sym, err := q.symbolAt(ctx, false)
	if err != nil || sym == nil {
		var refused *refusal
		if errors.As(err, &refused) {
			if refused.warn {
				warn(e, "references", &result{incomplete: []string{refused.msg}})
			}
			return locs, nil // a C/C++ name: no references
		}
		return locs, err
	}
	res, err := s.search(ctx, sym)
	if err != nil {
		return nil, err
	}
	warn(e, "references", res)
	s.warnModules(e, "references", q, sym, false)
	docs := newDocsFor(e.snapshot, res.hits)
	locs = make([]protocol.Location, 0, len(res.hits))
	for _, h := range res.hits {
		if h.def && !p.Context.IncludeDeclaration {
			continue
		}
		loc, err := docs.location(h)
		if err != nil {
			return nil, err
		}
		locs = append(locs, loc)
	}
	return protocol.LocationList(locs), nil
}

// warn tells the user, with a window/showMessage warning, that the results of
// the search are not complete.
func warn(e env, what string, res *result) {
	if len(res.incomplete) == 0 || e.notify == nil {
		return
	}
	msg := fmt.Sprintf("gocpp %s: the results are incomplete. %s", what, strings.Join(res.incomplete, "; "))
	if err := e.notify("window/showMessage", &protocol.ShowMessageParams{Type: protocol.Warning, Message: msg}); err != nil {
		e.log.Printf("window/showMessage: %v", err)
	}
}

// documentHighlight answers textDocument/documentHighlight: the places of the
// name in the document, writes (the declaration and assignments) and reads.
func (s *Server) documentHighlight(ctx context.Context, e env, params json.RawMessage) (any, error) {
	p, err := decodePosition(params)
	if err != nil {
		return nil, err
	}
	q, err := newRefQuery(ctx, e.snapshot, p.TextDocument.URI, p.Position)
	if err != nil {
		return nil, err
	}
	hl := []protocol.DocumentHighlight{}
	if q == nil {
		return hl, nil
	}
	sym, err := q.symbolAt(ctx, false)
	if sym == nil || err != nil {
		var refused *refusal
		if errors.As(err, &refused) {
			err = nil
		}
		return hl, err
	}
	// The package of the document is enough: only its own hits are shown, so
	// the test files of other packages are not looked at either (the cursor
	// moves all the time: a highlight must stay cheap).
	res, err := s.searchIn(ctx, sym, []string{q.lp.Dir})
	if err != nil {
		return nil, err
	}
	writes := writePositions(q.file, q.tok)
	for _, h := range res.hits {
		if h.path != q.file.Path {
			continue
		}
		kind := protocol.Read
		if h.def || writes[h.off] {
			kind = protocol.Write
		}
		hl = append(hl, protocol.DocumentHighlight{
			Range: protocol.Range{Start: q.mapper.Position(h.off), End: q.mapper.Position(h.end)},
			Kind:  kind,
		})
	}
	return hl, nil
}

// writePositions returns the offsets of the names that an assignment, an
// inc/dec statement or a range clause stores into: a name, or the selected
// name of a selector (x.f = 1 writes f).
func writePositions(lf *load.File, tok *token.File) map[int]bool {
	f := lf.AST
	w := make(map[int]bool)
	mark := func(e ast.Expr) {
		switch e := ast.Unparen(e).(type) {
		case *ast.Ident:
			if !lf.Synthetic[e] {
				w[tok.Offset(e.Pos())] = true
			}
		case *ast.SelectorExpr:
			if !lf.Synthetic[e.Sel] {
				w[tok.Offset(e.Sel.Pos())] = true
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				mark(l)
			}
		case *ast.IncDecStmt:
			mark(n.X)
		case *ast.RangeStmt:
			if n.Key != nil {
				mark(n.Key)
			}
			if n.Value != nil {
				mark(n.Value)
			}
		}
		return true
	})
	return w
}

// A refusal is why a name cannot be searched or renamed. The message is for
// the user.
type refusal struct {
	msg string
	// warn: the references request that is refused tells the user why (the
	// answer is empty because the code could not be searched, not because the
	// name has no references).
	warn bool
}

func (r *refusal) Error() string { return r.msg }

func refuse(format string, args ...any) error { return &refusal{msg: fmt.Sprintf(format, args...)} }

// A symbol is the name at the place of a request and what it stands for.
type symbol struct {
	q    *query
	id   *ast.Ident // the name at the cursor; nil for an import without a name
	rng  protocol.Range
	name string
	// objs are the objects (origins) of the name in the document's check: one,
	// but the objects of a type switch name, and a type name with the embedded
	// fields it implies come later (closure).
	objs []types.Object
	// extra are declaring names that have no object of their own: the x of
	// switch x := v.(type).
	extra []*ast.Ident
	// scope says where the name can be seen.
	scope scope
	// rename: the symbol is looked for to be renamed, so a type name stands
	// for the embedded fields of that type too (closure).
	rename bool
	// why is the reason the name cannot be renamed, or "".
	why string
	// declPath is the import path of the package that declares the name ("" for
	// a name of the universe or of a file: an import name).
	declPath string
	// declDir is that package's folder, or "" when it is not known or
	// not in the workspace.
	declDir string
	// std: the package is of the standard library.
	std bool
}

// A scope is how far a name can be seen from the package that declares it.
type scope int

const (
	scopeLocal   scope = iota // its own package (a local name, an unexported one)
	scopeImports              // the packages that import its package (an exported package-level name)
	scopeModule               // every package of the module (an exported method or field)
	// scopeFamily is an unexported method: no other package can name it, but a
	// use in an importer can tie its type to an interface that has it (a value
	// of base.Dog passed to a func(base.Animal) in main makes base.Dog.name and
	// base.Animal.name one method), so the importers are searched for ties.
	scopeFamily
)

// symbolAt returns the symbol at the place of q: nil when there is no name
// there (the answer is empty), a *refusal when there is one that is not
// searched.
func (q *query) symbolAt(ctx context.Context, forRename bool) (*symbol, error) {
	f := q.file.AST
	if f.Name != nil && q.contains(f.Name) {
		return nil, refuse("renaming a package is not supported")
	}
	if info := q.file.Info; info != nil {
		for _, inc := range info.Includes {
			if q.specPart(inc.Spec) != nil {
				return nil, refuse("%s is a C/C++ header, not a name that gocpp declares", inc.Path)
			}
		}
	}
	if u, _, _ := q.cppUseAt(); u != nil {
		return nil, refuse("this is a C/C++ name: it is declared in a header, not in gocpp code")
	}
	// An import: the name it declares, from its path too.
	if info := q.file.Info; info != nil {
		for _, spec := range info.Packages {
			if q.contains(spec.Path) && (spec.Name == nil || !q.contains(spec.Name)) {
				return nil, refuse("renaming a package is not supported: rename its name in this file by placing the cursor on the name, or on a use of it")
			}
		}
	}
	id, path := identAt(q.file, q.pos)
	if id == nil {
		return nil, nil
	}
	sym := &symbol{q: q, id: id, name: id.Name, rng: q.rangeOf(id.Pos(), id.End()), rename: forRename}
	obj := q.objectAt(id)
	// A cgo helper: C.GoString.
	if len(path) > 0 {
		if sel, ok := path[len(path)-1].(*ast.SelectorExpr); ok && check.CgoHelperOf(sel, q.info) != "" {
			return nil, refuse("C.%s is a C/C++ name", sel.Sel.Name)
		}
	}
	if obj == nil {
		if objs, extra := q.typeSwitchObjects(id, path); objs != nil {
			sym.objs, sym.extra = objs, extra
			sym.setScope()
			return sym, nil
		}
		if len(path) > 0 {
			if sel, ok := path[len(path)-1].(*ast.SelectorExpr); ok && sel.Sel == id && q.opaque.IsOpaque(sel.X) {
				return nil, refuse("%s is a member of a C/C++ value: it is declared in a header, not in gocpp code", id.Name)
			}
			// pkg.Name where the check of pkg has no types: an error in pkg
			// makes its exports unavailable here.
			if sel, ok := path[len(path)-1].(*ast.SelectorExpr); ok && sel.Sel == id {
				if x, ok := sel.X.(*ast.Ident); ok {
					if pn, ok := q.info.Uses[x].(*types.PkgName); ok && pn.Imported().Path() != "C" && len(pn.Imported().Scope().Names()) == 0 {
						return nil, &refusal{msg: fmt.Sprintf("%s.%s is unavailable: the package %s has errors that make its exports unavailable here; fix them first", x.Name, id.Name, pn.Imported().Path()), warn: true}
					}
				}
			}
		}
		return nil, nil
	}
	if pn, ok := obj.(*types.PkgName); ok && pn.Imported().Path() == "C" || obj.Pkg() != nil && obj.Pkg().Path() == "C" {
		return nil, refuse("%s is a C/C++ name", id.Name)
	}
	obj = origin(obj)
	if v, ok := obj.(*types.Var); ok && v.Embedded() && forRename {
		// An embedded field is its type's name (x.T): renaming the field
		// renames the type, and the type's name renames the fields (the
		// closure). The references of the type and those of the field are
		// two sets, as in gopls: its embedded fields' uses are not uses of
		// the type.
		if tn := embeddedTypeName(v); tn != nil {
			obj = tn
		}
	}
	if v, ok := obj.(*types.Var); ok && !v.IsField() {
		if objs, extra := q.typeSwitchObjects(id, path); objs != nil && slices.Contains(objs, types.Object(v)) {
			sym.objs, sym.extra = objs, extra
			sym.setScope()
			return sym, nil
		}
		if objs, extra := q.typeSwitchOf(v); objs != nil {
			sym.objs, sym.extra = objs, extra
			sym.setScope()
			return sym, nil
		}
	}
	sym.objs = []types.Object{obj}
	sym.name = obj.Name()
	sym.setScope()
	if err := sym.declaredWhere(); err != nil {
		return nil, err
	}
	return sym, nil
}

// origin returns the generic declaration that obj is an instance of's
// member of.
func origin(obj types.Object) types.Object {
	switch o := obj.(type) {
	case *types.Var:
		return o.Origin()
	case *types.Func:
		return o.Origin()
	}
	return obj
}

// embeddedTypeName returns the type name of the embedded field v, or nil.
func embeddedTypeName(v *types.Var) *types.TypeName {
	t := v.Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		return t.Obj()
	}
	return nil
}

// typeSwitchObjects returns the objects of the name at the declaration id of
// a type switch (switch x := v.(type), path ends in the assignment): one
// per clause, and the declaring name, which has no object. It returns nil for
// any other name.
func (q *query) typeSwitchObjects(id *ast.Ident, path []ast.Node) ([]types.Object, []*ast.Ident) {
	if len(path) < 2 {
		return nil, nil
	}
	assign, ok := path[len(path)-1].(*ast.AssignStmt)
	if !ok || len(assign.Lhs) != 1 || assign.Lhs[0] != id {
		return nil, nil
	}
	sw, ok := path[len(path)-2].(*ast.TypeSwitchStmt)
	if !ok || sw.Assign != assign {
		return nil, nil
	}
	return q.clauseObjects(sw), []*ast.Ident{id}
}

// clauseObjects returns the objects of the name of the type switch sw, one per
// clause (go/types' Implicits).
func (q *query) clauseObjects(sw *ast.TypeSwitchStmt) []types.Object {
	var objs []types.Object
	for _, c := range sw.Body.List {
		if obj := q.info.Implicits[c]; obj != nil {
			objs = append(objs, obj)
		}
	}
	return objs
}

// typeSwitchOf returns the objects and the declaring name of the type switch
// that declares v (a use of its name in a clause), or nil. go/types gives
// every clause's object the place of the declaring name.
func (q *query) typeSwitchOf(v *types.Var) ([]types.Object, []*ast.Ident) {
	if !v.Pos().IsValid() || q.fset.File(v.Pos()) != q.tok {
		return nil, nil
	}
	id, path := identAt(q.file, v.Pos())
	if id == nil || id.Pos() != v.Pos() || q.info.Defs[id] != nil {
		return nil, nil
	}
	return q.typeSwitchObjects(id, path)
}

// setScope sets the scope of the symbol from its objects.
func (sym *symbol) setScope() {
	sym.scope = scopeLocal
	if len(sym.objs) != 1 {
		return
	}
	obj := sym.objs[0]
	if obj.Pkg() == nil {
		return
	}
	if fn, ok := obj.(*types.Func); ok && !obj.Exported() {
		if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
			sym.scope = scopeFamily
		}
		return
	}
	if !obj.Exported() {
		return
	}
	switch o := obj.(type) {
	case *types.Func:
		if sig, ok := o.Type().(*types.Signature); ok && sig.Recv() != nil {
			sym.scope = scopeModule
		} else if o.Parent() == o.Pkg().Scope() {
			sym.scope = scopeImports
		}
	case *types.Var:
		if o.IsField() {
			sym.scope = scopeModule
		} else if o.Parent() == o.Pkg().Scope() {
			sym.scope = scopeImports
		}
	case *types.Const:
		if o.Parent() == o.Pkg().Scope() {
			sym.scope = scopeImports
		}
	case *types.TypeName:
		if o.Parent() == o.Pkg().Scope() {
			// The embedded fields of the type (closure) are fields, which
			// are seen wherever a value of their struct can come to.
			sym.scope = scopeModule
		}
	}
}

// declaredWhere fills in the package of the symbol: its import path and
// folder, and why it cannot be renamed when it is not declared in the
// workspace.
func (sym *symbol) declaredWhere() error {
	q := sym.q
	obj := sym.objs[0]
	switch {
	case isBuiltin(obj):
		sym.why = fmt.Sprintf("%s is predeclared", obj.Name())
		return nil
	case obj.Pkg() == nil:
		return nil
	}
	if _, ok := obj.(*types.PkgName); ok {
		sym.declPath = q.types.Path()
		sym.declDir = q.lp.Dir
		return nil
	}
	sym.declPath = obj.Pkg().Path()
	switch {
	case sym.declPath == q.lp.ImportPath:
		sym.declDir = q.lp.Dir
	case q.lp.Imports[sym.declPath] != nil:
		dep := q.lp.Imports[sym.declPath]
		sym.std = dep.Std
		sym.declDir = dep.Dir
		if dep.Std {
			sym.declDir = ""
		}
	default:
		sym.std = isStdPath(sym.declPath)
	}
	return nil
}

// isStdPath reports whether the import path is of the embedded standard
// library or has no dot in its first element.
func isStdPath(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

// An objKey names an object independently of the check it is in.
type objKey struct {
	pkg  string          // the import path of the object's package
	path objectpath.Path // the object's path in it; "" when there is none: see local
	name string
	// local is the object itself, for one that has no path (a local name, an
	// import name, a predeclared name): found in the document's check only.
	local types.Object
}

// keyOf returns the key of obj.
func keyOf(obj types.Object) objKey {
	obj = origin(obj)
	k := objKey{name: obj.Name()}
	if obj.Pkg() == nil {
		k.local = obj
		return k
	}
	k.pkg = obj.Pkg().Path()
	if _, ok := obj.(*types.PkgName); !ok {
		if p, err := objectpath.For(obj); err == nil {
			k.path = p
			return k
		}
	}
	k.local = obj
	return k
}

// A view is a checked package, or a package with its test files, as the
// search sees it.
type view struct {
	dir   string
	fset  *token.FileSet
	types *types.Package
	info  *types.Info
	pkg   *cache.Package // nil for a test view
	// files are the files that count in the view by name: all of a checked
	// package, the _test.gp files of a test view.
	files map[string]*ast.File
	test  bool

	// synthetic are the names that check wrote for the ! forms (File.Synthetic) in all the files.
	synthetic map[*ast.Ident]bool
}

// addSynthetic adds the synthetic names of f to the view.
func (v *view) addSynthetic(f *load.File) {
	for id := range f.Synthetic {
		if v.synthetic == nil {
			v.synthetic = make(map[*ast.Ident]bool)
		}
		v.synthetic[id] = true
	}
}

// resolve returns the object of the key in the view's types, or nil.
func (v *view) resolve(k objKey) types.Object {
	if k.local != nil {
		return k.local
	}
	var tp *types.Package
	switch {
	case v.pkg != nil:
		tp = v.pkg.Lookup(k.pkg)
	case v.types != nil:
		tp = findPackage(v.types, k.pkg, make(map[*types.Package]bool))
	}
	if tp == nil {
		return nil
	}
	obj, err := objectpath.Object(tp, k.path)
	if err != nil {
		return nil
	}
	return obj
}

// findPackage returns the package with the import path in root's imports,
// directly or not, or root itself, or nil.
func findPackage(root *types.Package, path string, seen map[*types.Package]bool) *types.Package {
	if root.Path() == path {
		return root
	}
	if seen[root] {
		return nil
	}
	seen[root] = true
	for _, imp := range root.Imports() {
		if tp := findPackage(imp, path, seen); tp != nil {
			return tp
		}
	}
	return nil
}

// faultInViews is set by a test: it is called in the goroutine of each package
// of a search, and panics there.
var faultInViews func(dir string)

// skipImporterTies is set by a test: the search of an unexported method leaves
// out the importers, so that the check of the result is what finds a tie there.
var skipImporterTies bool

// skipTestClosure is set by a test: the closure leaves out the checks with test
// files, so that the check of the result is what finds the broken tests.
var skipTestClosure bool

// A hit is a place where the symbol's name is declared or used.
type hit struct {
	path     string // the file
	off, end int    // the name, in bytes
	def      bool   // the declaring name
	v        *view
	id       *ast.Ident
	obj      types.Object
	// spec is the import that has no name, for a hit at the path of a
	// declaration of an import name that was not written.
	spec *ast.ImportSpec
}

// position returns the place of the hit.
func (h hit) position() token.Position {
	if h.id != nil {
		return h.v.fset.Position(h.id.Pos())
	}
	if h.spec != nil {
		return h.v.fset.Position(h.spec.Path.Pos())
	}
	return token.Position{Filename: h.path}
}

// A result is what a search found.
type result struct {
	hits []hit
	// keys are the objects that stand for the symbol, with the symbol's own.
	keys []objKey
	// views are what was searched, by folder; test views are the test checks.
	views []*view
	// outside is the first related name that is declared outside the
	// workspace (a method of an interface of the standard library): the
	// symbol cannot be renamed then.
	outside string
	// testErr is set when a test check failed.
	testErr error
	// tests are the checks with test files, by folder.
	tests map[string]*cache.TestCheck
	// incomplete says, for each package that imports the declaring package
	// and could not be searched, why: the references of the search are not
	// all of them, and a rename would leave that package behind.
	incomplete []string
	// importersOf returns the folders of the packages that import the package
	// in the folder dir, directly or not; nil when the search did not read the
	// imports of the module.
	importersOf func(dir string) []string
	// dynamic are the methods that the dynamic rule added to the family of a
	// rename (dynamic.go).
	dynamic []dynAdd
	// moduleDirs are the folders of the packages of the module (the package of the
	// symbol alone without a module).
	moduleDirs []string
}

// search finds the places of the symbol in the workspace.
func (s *Server) search(ctx context.Context, sym *symbol) (*result, error) {
	return s.searchIn(ctx, sym, nil)
}

// searchIn is search, limited to the folders dirs when they are not nil (the
// symbol's own package is always in them, when it has a folder).
func (s *Server) searchIn(ctx context.Context, sym *symbol, only []string) (*result, error) {
	r := &searcher{s: s, sym: sym, snap: sym.q.snap, local: len(only) > 0}
	return r.run(ctx, only)
}

// A searcher is one search.
type searcher struct {
	s    *Server
	sym  *symbol
	snap *cache.Snapshot

	// local limits the search to the document's package (highlights).
	local bool

	mod       *load.Module
	pkgs      []importable // the module's packages
	imports   map[string][]cache.ImportRef
	reverse   map[string][]string      // the folders that import an import path (importers)
	fileInfo  map[*token.File]fileInfo // hitOf's table: one search, one view of each file
	pathOfDir map[string]string        // the import path of the module's package in a folder

	// wantAll: a dynamic target is part of the family, so the types of every
	// package of the module matter (dynamic.go); allLoaded: they are searched.
	wantAll, allLoaded bool
	// dyn are the dynamic targets (assertions, type switches, errors.As) of the
	// module on an interface with a method of the symbol's name, found by
	// discoverDynamic in packages that the search may not look at; dynAdded the
	// methods that the dynamic rule added (relatedByShapes).
	dyn      []*dynTarget
	dynAdded []dynAdd
	// dynTest are the folders whose test files have an assertion that can reach
	// an interface of the family: their test checks are searched.
	dynTest map[string]bool
}

// scope is the symbol's scope for this search: the importers of an unexported
// method are looked at for ties only in the narrow family (the wide family
// finds the types that could be used as the interface in the packages it
// searches, as before).
func (r *searcher) scope() scope {
	if r.sym.scope == scopeFamily && r.s.wideFamily {
		return scopeLocal
	}
	return r.sym.scope
}

// run does the search.
func (r *searcher) run(ctx context.Context, only []string) (*result, error) {
	sym := r.sym
	q := sym.q
	res := &result{tests: make(map[string]*cache.TestCheck)}
	keys := newKeySet()
	for _, obj := range sym.objs {
		keys.add(keyOf(obj))
	}
	declPos := make(map[posKey]bool)
	if err := r.initialDecls(ctx, declPos); err != nil {
		return nil, err
	}
	dirs, err := r.dirs(ctx, only)
	if err != nil {
		return nil, err
	}
	if err := r.discoverDynamic(ctx); err != nil {
		return nil, err
	}
	// The packages that can see the symbol, and the objects that stand for
	// it: a related name in another package makes that package, and the ones
	// that import it, part of the search (closure). The checks with the test
	// files take part too: a method of a type that a test file declares
	// belongs to the family of the interface method it implements, and the
	// other way round.
	searched := make(map[string]bool)
	var views, tviews []*view
	var missing []string
	todo := dirs
	again := false
	for round := 0; round < 8; round++ {
		for _, d := range todo {
			searched[d] = true
		}
		more, none, err := r.views(ctx, todo)
		if err != nil {
			return nil, err
		}
		views = append(views, more...)
		missing = append(missing, none...)
		searchedDirs := make([]string, 0, len(searched))
		for d := range searched {
			searchedDirs = append(searchedDirs, d)
		}
		slices.Sort(searchedDirs)
		if tviews, err = r.testViews(ctx, searchedDirs, keys, declPos, res); err != nil {
			return nil, err
		}
		before := len(keys.list)
		if err := r.closure(ctx, append(slices.Clone(views), tviews...), keys, res); err != nil {
			return nil, err
		}
		todo = nil
		if len(only) == 0 {
			todo = r.moreDirs(keys, searched)
			if r.wantAll && !r.allLoaded {
				r.allLoaded, again = true, true // the closure goes on with every package
				for _, p := range r.pkgs {
					if !searched[p.dir] {
						searched[p.dir] = true
						todo = append(todo, p.dir)
					}
				}
				sort.Strings(todo)
			}
		}
		if len(todo) == 0 && len(keys.list) == before && !again {
			break
		}
		again = false
	}
	r.checkImporters(views, missing, res)
	if r.imports != nil {
		res.importersOf = func(dir string) []string {
			for _, p := range r.pkgs {
				if p.dir == dir {
					return r.importers(p.path, true)
				}
			}
			return nil
		}
	}
	res.views = views
	res.dynamic = r.dynAdded
	if r.pkgs != nil {
		for _, p := range r.pkgs {
			res.moduleDirs = append(res.moduleDirs, p.dir)
		}
	} else {
		res.moduleDirs = []string{q.lp.Dir}
	}
	res.keys = keys.list
	// The hits of every view go into one list, made once at the size that the names allow: a
	// popular name has thousands of them, and growing a list of them was a good part of the
	// cost of the request (and of the garbage that its collection left for the next one).
	var tfiles int
	for _, v := range views {
		tfiles += len(v.files)
	}
	if r.fileInfo == nil {
		r.fileInfo = make(map[*token.File]fileInfo, tfiles)
	}
	all := make([]hit, 0, r.hitCapacity(views, keys))
	for _, v := range views {
		all = r.scan(v, keys, declPos, all)
	}
	// The names that have no object: the declaring name of a type switch.
	for _, id := range sym.extra {
		if h, ok := r.hitOf(q.view(), id, nil, true); ok {
			all = append(all, h)
		}
	}
	// The names of the test files.
	for _, v := range tviews {
		res.views = append(res.views, v)
		all = r.scan(v, keys, declPos, all)
	}
	// Sorted by place, and a place once: the first hit found there is the one that stays (views
	// overlap: the checks with test files see the files of the package again).
	slices.SortStableFunc(all, func(a, b hit) int {
		return cmp.Or(strings.Compare(a.path, b.path), cmp.Compare(a.off, b.off))
	})
	res.hits = slices.CompactFunc(all, func(a, b hit) bool {
		return a.path == b.path && a.off == b.off
	})
	return res, nil
}

// checkImporters finds the packages that import the declaring package but have
// no check that sees it: a syntax or import error in the declaring package (or
// in the importer) leaves them without its exports, so the search can't find
// their uses. (A package whose check lacks the declaring package's types
// looks like one that doesn't use the name.)
func (r *searcher) checkImporters(views []*view, missing []string, res *result) {
	sym := r.sym
	if sym.declPath == "" || sym.std || r.imports == nil {
		return
	}
	have := make(map[string]*view, len(views))
	for _, v := range views {
		have[v.dir] = v
	}
	for _, d := range r.importers(sym.declPath, false) {
		if d == sym.declDir {
			continue
		}
		v := have[d]
		switch {
		case v == nil && !slices.Contains(missing, d):
			continue // not searched
		case v != nil && (v.pkg == nil || v.pkg.Lookup(sym.declPath) != nil):
			continue
		}
		res.incomplete = append(res.incomplete, fmt.Sprintf("package %s imports %s, but its check has no types for it: an error in %s (or in %s itself) makes its exports unavailable",
			r.pkgName(d), sym.declPath, sym.declPath, r.pkgName(d)))
	}
	slices.Sort(res.incomplete)
}

// pkgName returns the import path of the module's package in the folder dir,
// or the folder's name.
func (r *searcher) pkgName(dir string) string {
	for _, p := range r.pkgs {
		if p.dir == dir {
			return p.path
		}
	}
	return filepath.Base(dir)
}

// moreDirs returns the folders, not searched yet, of the packages that
// declare the objects of keys and of the packages that import them: the
// places where a related name that closure found in another package is
// used.
func (r *searcher) moreDirs(keys *keySet, searched map[string]bool) []string {
	if skipImporterTies && r.sym.scope == scopeFamily {
		return nil
	}
	var out []string
	for _, k := range keys.list {
		if k.pkg == "" || k.local != nil {
			continue
		}
		for _, p := range r.pkgs {
			if p.path == k.pkg {
				for _, d := range append([]string{p.dir}, r.importers(p.path, true)...) {
					if !searched[d] {
						searched[d] = true
						out = append(out, d)
					}
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// hitCapacity is the number of names that can be hits of the keys in the views: every name of a
// key's spelling, whatever it is the name of.
func (r *searcher) hitCapacity(views []*view, keys *keySet) int {
	var spelled map[string]bool
	n := 0
	for _, k := range keys.list {
		if spelled[k.name] {
			continue
		}
		if spelled == nil {
			spelled = make(map[string]bool, len(keys.list))
		}
		spelled[k.name] = true
		for _, v := range views {
			if l := v.identIndex().byName[k.name]; l != nil {
				n += len(l.defs) + len(l.uses)
			}
		}
	}
	return min(n, maxHitCapacity)
}

// maxHitCapacity bounds the list that hitCapacity asks for: a name that is spelled by a hundred
// thousand names (and is one of the symbol's few of them) does not make a list of that size.
const maxHitCapacity = 1 << 16

// A posKey is the place of a declaration.
type posKey struct {
	path string
	off  int
}

// declKeyOf returns the place of pos as a posKey: the file's own name and the
// byte offset (not the line directives' name that Position would give).
func declKeyOf(fset *token.FileSet, pos token.Pos) (posKey, bool) {
	tf := fset.File(pos)
	if tf == nil {
		return posKey{}, false
	}
	return posKey{tf.Name(), tf.Offset(pos)}, tf.Name() != ""
}

// A keySet is a list of keys without duplicates, in the order they came.
type keySet struct {
	list []objKey
	has  map[objKey]bool
}

func newKeySet() *keySet { return &keySet{has: make(map[objKey]bool)} }

func (s *keySet) add(k objKey) bool {
	if s.has[k] {
		return false
	}
	s.has[k] = true
	s.list = append(s.list, k)
	return true
}

// view returns the view of the document's own package: the checked package,
// or for a _test.gp document the check with the test files.
func (q *query) view() *view {
	files := make(map[string]*ast.File, len(q.lp.Files))
	v := &view{dir: q.lp.Dir, fset: q.fset, types: q.types, info: q.info, pkg: q.pkg, test: q.isTest()}
	for _, f := range q.lp.Files {
		if f.AST != nil && (!q.isTest() || f.Test) {
			files[f.Path] = f.AST
			v.addSynthetic(f)
		}
	}
	v.files = files
	return v
}

// isTest reports whether the query is in a _test.gp document, which is a
// file of a check with tests, not of the package's own check.
func (q *query) isTest() bool { return q.tests != nil }

// newRefQuery is newQuery for the features that also work in the test files
// of a package: the document of a _test.gp file gets a query on the check of
// its package with its tests (cache.Snapshot.CheckTests), which is not
// cached and costs a load of the package and its dependencies from source.
func newRefQuery(ctx context.Context, snap *cache.Snapshot, uri protocol.DocumentURI, at protocol.Position) (*query, error) {
	path := uri.Path()
	if !strings.HasSuffix(path, "_test.gp") {
		return newQuery(ctx, snap, uri, at)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	src, err := snap.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	tc, err := snap.CheckTests(ctx, filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	for _, root := range tc.Program.Roots {
		cp := tc.Result.Package(root)
		if cp == nil {
			continue
		}
		for _, f := range root.Files {
			if f.Path != path || f.AST == nil {
				continue
			}
			q := &query{snap: snap, fset: tc.Program.Fset, lp: root, file: f, exact: make(map[string]bool), tests: tc}
			switch {
			case cp.Checked():
				q.types, q.info, q.opaque = cp.Types, cp.Info, cp.Opaque
			case cp.HasPartial():
				q.types, q.info, q.opaque = cp.PartialResult().Types, cp.PartialResult().Info, cp.PartialResult().Opaque
			default:
				return nil, nil
			}
			for _, lp := range tc.Program.Packages {
				for _, f := range lp.Files {
					q.exact[f.Path] = true
				}
			}
			q.tok = q.fset.File(f.AST.FileStart)
			q.src = src
			if q.tok == nil || q.tok.Size() != len(src) {
				return nil, nil
			}
			q.mapper = protocol.NewMapper(string(src))
			q.pos = q.tok.Pos(q.mapper.Offset(at))
			return q, nil
		}
	}
	return nil, nil
}

// dirs returns the folders of the packages to search: the symbol's scope.
func (r *searcher) dirs(ctx context.Context, only []string) ([]string, error) {
	sym := r.sym
	q := sym.q
	own := q.lp.Dir
	set := map[string]bool{own: true}
	if sym.declDir != "" {
		set[sym.declDir] = true
	}
	if len(only) > 0 {
		return only, nil
	}
	if sc := r.scope(); sc != scopeLocal && sym.declPath != "" {
		mod := q.lp.Module
		if mod != nil && mod.Path != "" && !q.lp.Std {
			r.mod = mod
			pkgs, err := r.s.comp.index.moduleWait(ctx, mod) // all of it: a missing package is a missing reference
			if err != nil {
				return nil, err
			}
			r.pkgs = r.withOpen(pkgs)
			byPath := make(map[string]string, len(r.pkgs))
			for _, p := range r.pkgs {
				byPath[p.path] = p.dir
			}
			if d, ok := byPath[sym.declPath]; ok {
				set[d] = true
				if sym.declDir == "" {
					sym.declDir = d
				}
			}
			if err := r.loadImports(ctx); err != nil {
				return nil, err
			}
			if sc != scopeFamily || !skipImporterTies {
				for _, d := range r.importers(sym.declPath, sc == scopeModule || sc == scopeFamily) {
					set[d] = true
				}
			}
		}
	}
	dirs := make([]string, 0, len(set))
	for d := range set {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs, nil
}

// withOpen adds the packages of the open documents that are in the module
// but not on disk yet (a new folder with an unsaved file).
func (r *searcher) withOpen(pkgs []importable) []importable {
	known := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		known[p.dir] = true
	}
	mod := r.mod
	for _, o := range r.snap.Overlays() {
		name := o.URI().Path()
		dir := filepath.Dir(name)
		rel, err := filepath.Rel(mod.Dir, dir)
		if known[dir] || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) ||
			strings.HasSuffix(name, "_test.gp") || !strings.HasSuffix(name, ".gp") {
			continue
		}
		if pn := packageName(name, []byte(o.Text())); pn != "" {
			known[dir] = true
			pkgs = append(slices.Clone(pkgs), importable{name: pn, path: joinPath(mod.Path, filepath.ToSlash(rel)), dir: dir, main: pn == "main"})
		}
	}
	return pkgs
}

// loadImports reads the imports of the module's packages (cheap: the
// metadata of the pipeline), concurrently.
func (r *searcher) loadImports(ctx context.Context) error {
	r.imports = make(map[string][]cache.ImportRef, len(r.pkgs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	var pc safe.Catcher // a panic in a goroutine is the request's (see protocol.Conn)
	sem := make(chan struct{}, 8)
	for _, p := range r.pkgs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			pc.Run(func() {
				refs, err := r.snap.ImportsOf(ctx, p.dir)
				if err != nil {
					return // a folder that is no package
				}
				mu.Lock()
				r.imports[p.dir] = refs
				mu.Unlock()
			})
		}()
	}
	wg.Wait()
	pc.Rethrow()
	return ctx.Err()
}

// importers returns the folders of the module's packages that import the
// package with the import path path, directly or, with transitive,
// through other packages of the module too.
func (r *searcher) importers(path string, transitive bool) []string {
	if r.reverse == nil { // import path -> the folders that import it
		r.reverse = make(map[string][]string)
		for dir, refs := range r.imports {
			for _, ref := range refs {
				r.reverse[ref.ImportPath] = append(r.reverse[ref.ImportPath], dir)
			}
		}
	}
	reverse := r.reverse
	var out []string
	seen := make(map[string]bool)
	var visit func(path string)
	visit = func(path string) {
		for _, dir := range reverse[path] {
			if seen[dir] {
				continue
			}
			seen[dir] = true
			out = append(out, dir)
			if transitive {
				if r.pathOfDir == nil {
					r.pathOfDir = make(map[string]string, len(r.pkgs))
					for _, p := range r.pkgs {
						r.pathOfDir[p.dir] = p.path
					}
				}
				if path, ok := r.pathOfDir[dir]; ok {
					visit(path)
				}
			}
		}
	}
	visit(path)
	return out
}

// views checks the packages in dirs (concurrently) and returns their views;
// a folder that has no package or no types is left out, and named in the
// second result.
func (r *searcher) views(ctx context.Context, dirs []string) ([]*view, []string, error) {
	q := r.sym.q
	out := make([]*view, len(dirs))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	var pc safe.Catcher
	sem := make(chan struct{}, 8)
	for i, dir := range dirs {
		if dir == q.lp.Dir && !q.isTest() {
			out[i] = q.view()
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			pc.Run(func() {
				if faultInViews != nil {
					faultInViews(dir)
				}
				pkg, err := r.snap.Package(ctx, dir)
				if err != nil {
					if ctx.Err() != nil {
						mu.Lock()
						firstErr = ctx.Err()
						mu.Unlock()
					}
					return
				}
				out[i] = viewOf(pkg)
			})
		}()
	}
	wg.Wait()
	pc.Rethrow()
	if firstErr != nil {
		return nil, nil, firstErr
	}
	var views []*view
	var missing []string
	for i, v := range out {
		if v != nil {
			views = append(views, v)
		} else {
			missing = append(missing, dirs[i])
		}
	}
	return views, missing, ctx.Err()
}

// viewOf returns the view of a checked package, or nil when it has no types
// (not even partial ones).
func viewOf(pkg *cache.Package) *view {
	if len(pkg.Program.Roots) == 0 || pkg.Check == nil {
		return nil
	}
	cp := pkg.Check
	v := &view{dir: pkg.Program.Roots[0].Dir, fset: pkg.Fset(), pkg: pkg, files: make(map[string]*ast.File)}
	switch {
	case cp.Checked():
		v.types, v.info = cp.Types, cp.Info
	case cp.HasPartial():
		v.types, v.info = cp.PartialResult().Types, cp.PartialResult().Info
	default:
		return nil
	}
	for _, f := range pkg.Program.Roots[0].Files {
		if f.AST != nil {
			v.files[f.Path] = f.AST
			v.addSynthetic(f)
		}
	}
	return v
}

// An identIndex lists the names of a check by spelling: the identifiers that
// the check gave an object (as a declaration or a use), and the imports that
// have no name of their own. A search looks up the few spellings of the symbol
// instead of walking every name of the package, which is what made a search
// cost the size of the package and not the number of its results.
type identIndex struct {
	byName map[string]*identList
	specs  map[string][]*ast.ImportSpec // by the name of the package the import declares
}

// An identList holds the names that spell one name: the ones that declare an
// object and the ones that use one, with the objects, so that a search does
// not look them up in the check's maps (which is what a search of a popular
// name would spend its time on).
type identList struct {
	defs, uses []identObj
}

type identObj struct {
	id  *ast.Ident
	obj types.Object
}

type identIndexKey struct{}

// identIndex returns the index of the view's check: shared by every request
// and snapshot that has the package (cache.Package.Memo); a test view is not
// cached, so its index is made for the search.
func (v *view) identIndex() *identIndex {
	if v.pkg != nil && !v.test {
		return v.pkg.Memo(identIndexKey{}, func() any { return buildIdentIndex(v.info) }).(*identIndex)
	}
	return buildIdentIndex(v.info)
}

func buildIdentIndex(info *types.Info) *identIndex {
	ix := &identIndex{byName: make(map[string]*identList, len(info.Defs)/4+len(info.Uses)/16)}
	list := func(name string) *identList {
		l := ix.byName[name]
		if l == nil {
			l = new(identList)
			ix.byName[name] = l
		}
		return l
	}
	for id, obj := range info.Defs {
		if obj != nil {
			l := list(id.Name)
			l.defs = append(l.defs, identObj{id, obj})
		}
	}
	for id, obj := range info.Uses {
		if obj != nil {
			// The embedded field's own name is in Defs and Uses: both lists.
			l := list(id.Name)
			l.uses = append(l.uses, identObj{id, obj})
		}
	}
	for node, obj := range info.Implicits {
		if spec, ok := node.(*ast.ImportSpec); ok && spec.Name == nil && obj != nil {
			if ix.specs == nil {
				ix.specs = make(map[string][]*ast.ImportSpec)
			}
			ix.specs[obj.Name()] = append(ix.specs[obj.Name()], spec)
		}
	}
	return ix
}

// scan returns the hits of the keys in the view: the names that declare or
// use one of the objects, in the files of the view, added to hits (which may be nil). declPos gets
// the place of each declaration that it finds.
func (r *searcher) scan(v *view, keys *keySet, declPos map[posKey]bool, hits []hit) []hit {
	// An object is one of the symbol's by its key (it is found in this check's
	// types), or by the place of its declaration, when it is declared in a
	// file that this check loaded from source: the places are exact there,
	// and the same declaration has the same place in every check, which is how
	// the test files' checks and the names without a path (unexported ones)
	// are found.
	want := make(map[types.Object]bool, len(keys.list))
	for _, k := range keys.list {
		if obj := v.resolve(k); obj != nil {
			want[origin(obj)] = true
		}
	}
	// The answer for an object is the same for every name of it (the places
	// that are added to declPos while the view is scanned are the declarations
	// of objects that match already), so it is worked out once: a position
	// costs a search in the file set.
	known := make(map[types.Object]bool)
	matches := func(obj types.Object) bool {
		obj = origin(obj)
		if want[obj] {
			return true
		}
		if len(declPos) == 0 || !obj.Pos().IsValid() || !v.test && obj.Pkg() != v.types {
			return false
		}
		if m, ok := known[obj]; ok {
			return m
		}
		k, ok := declKeyOf(v.fset, obj.Pos())
		m := ok && declPos[k]
		known[obj] = m
		return m
	}
	add := func(id *ast.Ident, obj types.Object, def bool) {
		if obj == nil || !matches(obj) {
			return
		}
		if h, ok := r.hitOf(v, id, obj, def); ok {
			hits = append(hits, h)
			if def && origin(obj).Pkg() == v.types {
				if k, ok := declKeyOf(v.fset, origin(obj).Pos()); ok {
					declPos[k] = true
				}
			}
		}
	}
	// Only the names that the keys spell can be the symbol's: the index of the
	// check's names (identIndex) finds them without a walk over all of them.
	ix := v.identIndex()
	var seenName map[string]bool
	for _, k := range keys.list {
		if seenName[k.name] {
			continue
		}
		if seenName == nil {
			seenName = make(map[string]bool, len(keys.list))
		}
		seenName[k.name] = true
		if l := ix.byName[k.name]; l != nil {
			for _, d := range l.defs {
				add(d.id, d.obj, true)
			}
			for _, u := range l.uses {
				add(u.id, u.obj, false)
			}
		}
		for _, spec := range ix.specs[k.name] {
			// An import without a name declares the package name at its path.
			if obj := v.info.Implicits[spec]; obj != nil && matches(obj) {
				if h, ok := r.specHit(v, spec, obj); ok {
					hits = append(hits, h)
				}
			}
		}
	}
	return hits
}

// hitOf returns the hit of the name id in the view, or false for a name that
// is not in a file of the view or that the user did not write (the loader's
// rewrites make names with places but another text).
func (r *searcher) hitOf(v *view, id *ast.Ident, obj types.Object, def bool) (hit, bool) {
	if id == nil || !id.Pos().IsValid() || v.synthetic[id] {
		return hit{}, false
	}
	tf := v.fset.File(id.Pos())
	if tf == nil {
		return hit{}, false
	}
	// What the view and the snapshot say of a file is looked up once.
	fi, ok := r.fileInfo[tf]
	if !ok {
		fi.name = tf.Name()
		if file, in := v.files[fi.name]; in && file != nil {
			if src, err := r.snap.ReadFile(fi.name); err == nil {
				fi.src, fi.ok = src, true
			}
		}
		if r.fileInfo == nil {
			r.fileInfo = make(map[*token.File]fileInfo)
		}
		r.fileInfo[tf] = fi
	}
	if !fi.ok {
		return hit{}, false
	}
	off := tf.Offset(id.Pos())
	if off+len(id.Name) > len(fi.src) || string(fi.src[off:off+len(id.Name)]) != id.Name {
		return hit{}, false
	}
	return hit{path: fi.name, off: off, end: off + len(id.Name), def: def, v: v, id: id, obj: obj}, true
}

// A fileInfo is what a search needs of a file of a view: its name, and its
// text when the view counts the file.
type fileInfo struct {
	name string
	src  []byte
	ok   bool
}

// specHit returns the hit for an import without a name: the whole path.
func (r *searcher) specHit(v *view, spec *ast.ImportSpec, obj types.Object) (hit, bool) {
	p := v.fset.Position(spec.Path.Pos())
	if _, ok := v.files[p.Filename]; !ok {
		return hit{}, false
	}
	return hit{path: p.Filename, off: p.Offset, end: p.Offset + len(spec.Path.Value), def: true, v: v, obj: obj, spec: spec}, true
}

// A docs reads the documents of a snapshot once, for positions.
type docs struct {
	snap *cache.Snapshot
	mu   sync.Mutex
	m    map[string]*docEntry
}

type docEntry struct {
	doc *document
	uri protocol.DocumentURI
}

func newDocs(snap *cache.Snapshot) *docs { return &docs{snap: snap, m: make(map[string]*docEntry)} }

// newDocsFor is newDocs for the files of the hits: the table is made at its size.
func newDocsFor(snap *cache.Snapshot, hits []hit) *docs {
	files := 0
	last := ""
	for _, h := range hits { // sorted by file
		if h.path != last {
			files++
			last = h.path
		}
	}
	return &docs{snap: snap, m: make(map[string]*docEntry, files)}
}

func (d *docs) entry(path string) (*docEntry, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e := d.m[path]; e != nil {
		return e, nil
	}
	uri := locationURI(d.snap, path) // the editor's own URI for an open document, else the path's (one URIFromPath)
	doc, err := documentOfSnapshot(d.snap, uri)
	if err != nil {
		return nil, err
	}
	e := &docEntry{doc: doc, uri: uri}
	d.m[path] = e
	return e, nil
}

func (d *docs) get(path string) (*document, error) {
	e, err := d.entry(path)
	if err != nil {
		return nil, err
	}
	return e.doc, nil
}

// location returns the place of the hit.
func (d *docs) location(h hit) (protocol.Location, error) {
	e, err := d.entry(h.path)
	if err != nil {
		return protocol.Location{}, err
	}
	return protocol.Location{
		URI:   e.uri,
		Range: protocol.Range{Start: e.doc.mapper.Position(h.off), End: e.doc.mapper.Position(h.end)},
	}, nil
}

// initialDecls records the places of the declarations of the symbol's objects,
// which the test files are searched by (a test check has the objects of its
// own: they are the same declaration when they are declared in the same
// place). An object of the document's own sources has an exact place; one of
// an import comes from export data, and is found by its name (query.declOf).
func (r *searcher) initialDecls(ctx context.Context, declPos map[posKey]bool) error {
	q := r.sym.q
	for _, obj := range r.sym.objs {
		if !obj.Pos().IsValid() {
			continue
		}
		k, ok := declKeyOf(q.fset, obj.Pos())
		if ok && q.exact[k.path] {
			declPos[k] = true
			continue
		}
		d, err := q.declOf(ctx, obj)
		if err != nil {
			return err
		}
		if d != nil && d.pf != nil && d.off >= 0 {
			declPos[posKey{d.pf.Path, d.off}] = true
		}
	}
	return nil
}

// closure adds to keys the objects that stand for the symbol's: the methods
// that implement or are implemented by its methods, and the embedded fields
// of its type name, until nothing is added. It sets res.outside when a
// related method is not declared in the workspace.
func (r *searcher) closure(ctx context.Context, views []*view, keys *keySet, res *result) error {
	for round := 0; round < 8; round++ {
		added := false
		for _, v := range views {
			if err := ctx.Err(); err != nil {
				return err
			}
			if v.test && skipTestClosure {
				continue
			}
			if r.related(v, keys, res) {
				added = true
			}
		}
		if !r.s.wideFamily && r.relatedByShapes(views, keys) {
			added = true
		}
		if r.relatedAnonFields(views, keys) {
			added = true
		}
		if !added {
			return nil
		}
	}
	return nil
}

// A cand is a named type that the related-method search looks at.
type cand struct {
	named *types.Named
	pkg   *types.Package
}

// related adds the keys of the objects in v that are related to the keys'
// (closure) and reports whether it added one.
func (r *searcher) related(v *view, keys *keySet, res *result) bool {
	var methods []*types.Func
	typeNames := make(map[types.Object]bool)
	for _, k := range keys.list {
		switch o := v.resolve(k).(type) {
		case *types.Func:
			if sig, ok := o.Type().(*types.Signature); ok && sig.Recv() != nil {
				methods = append(methods, o)
			}
		case *types.TypeName:
			typeNames[o] = true
		}
	}
	added := false
	add := func(obj types.Object) bool {
		if keys.add(keyOf(obj)) {
			added = true
			if res.outside == "" && !r.inWorkspace(obj) {
				res.outside = objectName(obj)
			}
			return true
		}
		return false
	}
	// The embedded fields of the type names: x.T (for a rename only: the
	// references of a type are the uses of the type).
	if len(typeNames) > 0 && r.sym.rename {
		for _, obj := range v.info.Defs {
			if f, ok := obj.(*types.Var); ok && f.Embedded() {
				if tn := embeddedTypeName(f); tn != nil && typeNames[tn] {
					add(origin(f))
				}
			}
		}
	}
	if len(methods) == 0 {
		return added
	}
	byName := make(map[string][]*types.Func)
	for _, m := range methods {
		byName[m.Name()] = append(byName[m.Name()], m)
	}
	if !r.s.wideFamily {
		added = r.relatedByTies(v, methods, byName, add) || added
		if r.sym.rename {
			// A method that implements an interface of another package (the
			// standard library's: fmt.Stringer, error) is not renamed: the
			// rename would change what the code does (fmt stops calling
			// it) without a use that shows it, so the search finds the
			// interface's method, and the rename is refused (res.outside).
			for _, c := range r.candidates(v, byName, true) {
				for _, m := range byName[c.name] {
					for _, rel := range relatedMethods(m, c.named, c.pkg) {
						add(rel)
					}
				}
			}
		}
		return added
	}
	cands := r.candidates(v, byName, false)
	for _, c := range cands {
		for _, m := range byName[c.name] {
			for _, rel := range relatedMethods(m, c.named, c.pkg) {
				add(rel)
			}
		}
	}
	// A method that a type promotes from an embedded type (Square embeds Base,
	// which has Name) is the method of that type: the interface that the
	// embedding type satisfies through it is in the family, though the
	// declaring type alone does not satisfy the interface.
	for name, ms := range byName {
		var carriers []methodCand
		for _, c := range cands {
			if c.name != name || types.IsInterface(c.named) {
				continue
			}
			if fn := lookupMethod(c.named, c.pkg, name); fn != nil && slices.ContainsFunc(ms, func(m *types.Func) bool { return origin(m) == origin(fn) }) {
				carriers = append(carriers, c)
			}
		}
		for _, i := range cands {
			if i.name != name || !types.IsInterface(i.named) {
				continue
			}
			for _, c := range carriers {
				if implementsIface(c.named, i.named) {
					if fn := lookupMethod(i.named, i.pkg, name); fn != nil {
						add(origin(fn))
					}
					break
				}
			}
		}
	}
	return added
}

// relatedByTies adds the methods that a use of the view ties to the methods
// (the narrow family, gopls's): for each use that needs an interface of a value
// of another type (v.ties), the interface's method of a name that the family
// has is the implementation of the type's method of that name, so one stands
// for the other. It reports whether it added one. A type that merely could be
// used as an interface, with no use in the module that needs it, is not in the
// family (Options.WideFamily goes back to the wide one).
//
// The ties of a use only: the interfaces that a type assertion, a type switch
// or errors.As takes a value out as have no value of a known type to tie, and
// are followed by relatedByShapes (dynamic.go).
func (r *searcher) relatedByTies(v *view, methods []*types.Func, byName map[string][]*types.Func, add func(types.Object) bool) bool {
	ties := v.ties()
	if len(ties) == 0 {
		return false
	}
	in := make(map[types.Object]bool, len(methods))
	for _, m := range methods {
		in[origin(m)] = true
	}
	added := false
	for changed := true; changed; {
		changed = false
		for _, t := range ties {
			if t.asserted {
				continue
			}
			for name, ms := range byName {
				if !t.names[name] {
					continue
				}
				fs, fd := lookupMethod(t.src, ms[0].Pkg(), name), lookupMethod(t.dst, ms[0].Pkg(), name)
				if fs == nil || fd == nil {
					continue
				}
				os, od := origin(fs), origin(fd)
				switch {
				case in[os] && !in[od]:
					if !r.inWorkspace(od) {
						// An interface outside the workspace (error, fmt.Stringer):
						// a rename must not go on (it would change what the code
						// does there), so it is found and refused; the references
						// of the method are not those of every type that ties to
						// the same interface.
						if r.sym.rename {
							added = add(od) || added
						}
						continue
					}
					in[od], changed, added = true, true, true
					add(od)
				case in[od] && !in[os]:
					if !r.inWorkspace(os) {
						continue
					}
					in[os], changed, added = true, true, true
					add(os)
				}
			}
		}
	}
	return added
}

// lookupMethod returns the method called name of the type t (its own,
// promoted, or an interface's), or nil.
func lookupMethod(t types.Type, pkg *types.Package, name string) *types.Func {
	obj, _, _ := types.LookupFieldOrMethod(t, true, pkg, name)
	fn, _ := obj.(*types.Func)
	return fn
}

// implementsIface reports whether t, or a pointer to it, implements the
// interface type iface.
func implementsIface(t types.Type, iface types.Type) bool {
	it, ok := iface.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	return types.Implements(t, it) || !types.IsInterface(t) && types.Implements(types.NewPointer(t), it)
}

// A methodCand is a candidate with the method name it was chosen for.
type methodCand struct {
	cand
	name string
}

// candidates returns the named types of the view's package and of the packages
// its check read, with a method of one of the names; foreign leaves out the
// types of the workspace (the module's packages).
func (r *searcher) candidates(v *view, names map[string][]*types.Func, foreign bool) []methodCand {
	var pkgs []*types.Package
	pkgs = append(pkgs, v.types)
	if v.pkg != nil {
		for _, path := range v.pkg.ImportedPaths() {
			if tp := v.pkg.Lookup(path); tp != nil && tp != v.types {
				pkgs = append(pkgs, tp)
			}
		}
	} else {
		// A check with tests loads its imports from source: they are
		// the types' imports, directly or not.
		seen := map[*types.Package]bool{v.types: true}
		var walk func(tp *types.Package)
		walk = func(tp *types.Package) {
			for _, imp := range tp.Imports() {
				if !seen[imp] {
					seen[imp] = true
					pkgs = append(pkgs, imp)
					walk(imp)
				}
			}
		}
		walk(v.types)
	}
	workspace := func(tp *types.Package) bool {
		return tp == v.types || r.pathInWorkspace(tp.Path())
	}
	var out []methodCand
	for _, tp := range pkgs {
		scope := tp.Scope()
		for _, n := range scope.Names() {
			tn, ok := scope.Lookup(n).(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok || named.TypeParams().Len() > 0 {
				continue
			}
			if !workspace(tp) && !types.IsInterface(named) {
				continue // a concrete type of a package outside the workspace: not renamed with ours
			}
			if foreign && workspace(tp) {
				continue
			}
			for name := range names {
				if hasMethod(named, tp, name) {
					out = append(out, methodCand{cand{named, tp}, name})
				}
			}
		}
	}
	return out
}

// hasMethod reports whether the type t, or a pointer to it, has a method
// called name (its own, promoted, or an interface's).
func hasMethod(t *types.Named, pkg *types.Package, name string) bool {
	obj, _, _ := types.LookupFieldOrMethod(t, true, pkg, name)
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	return ok && sig.Recv() != nil || types.IsInterface(t)
}

// recvBase returns the type that declares the method m: its receiver type
// without a pointer.
func recvBase(m *types.Func) types.Type {
	sig, ok := m.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return nil
	}
	t := sig.Recv().Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	return types.Unalias(t)
}

// relatedMethods returns the methods of c that are related to the method m
// by the implements relation: when one of m's and c's types is an interface
// that the other (or a pointer to it) implements, or both are interfaces and
// one implements the other, c's method of m's name.
func relatedMethods(m *types.Func, c *types.Named, pkg *types.Package) []types.Object {
	rt := recvBase(m)
	if rt == nil {
		return nil
	}
	if named, ok := rt.(*types.Named); ok && named.TypeParams().Len() > 0 {
		return nil
	}
	find := func(t types.Type) *types.Func {
		obj, _, _ := types.LookupFieldOrMethod(t, true, pkg, m.Name())
		fn, _ := obj.(*types.Func)
		return fn
	}
	implements := func(t types.Type, iface types.Type) bool {
		it, ok := iface.Underlying().(*types.Interface)
		if !ok {
			return false
		}
		return types.Implements(t, it) || !types.IsInterface(t) && types.Implements(types.NewPointer(t), it)
	}
	rIface, cIface := types.IsInterface(rt), types.IsInterface(c)
	var rel bool
	switch {
	case rIface && cIface:
		rel = implements(c, rt) || implements(rt, c)
	case rIface:
		rel = implements(c, rt)
	case cIface:
		rel = implements(rt, c)
	}
	if !rel {
		return nil
	}
	if fn := find(c); fn != nil && origin(fn) != origin(m) {
		return []types.Object{origin(fn)}
	}
	return nil
}

// pathInWorkspace reports whether the package with the import path is the
// document's own or one of the module's.
func (r *searcher) pathInWorkspace(path string) bool {
	path = strings.TrimSuffix(path, "_test") // the external test package of a package
	if path == r.sym.q.lp.ImportPath {
		return true
	}
	for _, p := range r.pkgs {
		if p.path == path {
			return true
		}
	}
	return false
}

// inWorkspace reports whether obj is declared in a package of the
// workspace: the document's own, or one of the module's.
func (r *searcher) inWorkspace(obj types.Object) bool {
	if obj.Pkg() == nil {
		return false
	}
	return r.pathInWorkspace(obj.Pkg().Path())
}

// objectName returns how a message names obj: Type.Method, pkg.Name.
func objectName(obj types.Object) string {
	if fn, ok := obj.(*types.Func); ok {
		if rt := recvBase(fn); rt != nil {
			if n, ok := rt.(*types.Named); ok {
				if n.Obj().Pkg() == nil {
					return n.Obj().Name() + "." + fn.Name() // error.Error
				}
				return n.Obj().Pkg().Name() + "." + n.Obj().Name() + "." + fn.Name()
			}
			// A method of an interface literal: no type name to say.
			if _, ok := rt.Underlying().(*types.Interface); ok && fn.Pkg() != nil {
				return fn.Pkg().Name() + ".(interface literal)." + fn.Name()
			}
		}
	}
	if obj.Pkg() != nil {
		return obj.Pkg().Name() + "." + obj.Name()
	}
	return obj.Name()
}

// testViews returns the views of the test files that mention one of the
// names of the symbol, in the packages that can see it: the packages of dirs
// and the packages whose test files import one of them (test-only importers).
func (r *searcher) testViews(ctx context.Context, dirs []string, keys *keySet, declPos map[posKey]bool, res *result) ([]*view, error) {
	q := r.sym.q
	names := make(map[string]bool)
	names[r.sym.name] = true
	for _, k := range keys.list {
		names[k.name] = true
	}
	var needles [][]byte
	for n := range names {
		needles = append(needles, []byte(n))
	}
	mentions := func(src []byte) bool {
		for _, n := range needles {
			if bytes.Contains(src, n) {
				return true
			}
		}
		return false
	}
	inDirs := make(map[string]bool, len(dirs))
	paths := make(map[string]bool)
	for _, d := range dirs {
		inDirs[d] = true
	}
	for _, p := range r.pkgs {
		if inDirs[p.dir] {
			paths[p.path] = true
		}
	}
	if r.sym.declPath != "" {
		paths[r.sym.declPath] = true
	}
	candidates := slices.Clone(dirs)
	if r.scope() != scopeLocal {
		for _, p := range r.pkgs {
			if !inDirs[p.dir] {
				candidates = append(candidates, p.dir)
			}
		}
	}
	if r.local && q.tests == nil {
		return nil, nil // no test files are searched, unless the cursor is in one
	}
	var out []*view
	for _, dir := range candidates {
		if r.local && dir != q.lp.Dir {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		files, err := r.snap.TestFiles(dir)
		if err != nil {
			continue
		}
		use := r.dynTest[dir] // a test file with an assertion that can reach the family
		for _, name := range files {
			if use {
				break
			}
			src, err := r.snap.ReadFile(name)
			if err != nil || !mentions(src) {
				continue
			}
			if inDirs[dir] {
				use = true
				break
			}
			pf, err := r.snap.ParsePath(ctx, name, parser.ImportsOnly)
			if err != nil {
				continue
			}
			for _, spec := range pf.File.Imports {
				if paths[importPath(spec)] {
					use = true
				}
			}
			if use {
				break
			}
		}
		if !use {
			continue
		}
		tc := q.tests
		if tc == nil || dir != q.lp.Dir {
			tc = res.tests[dir]
		}
		if tc == nil {
			tc, err = r.snap.CheckTests(ctx, dir)
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				r.s.log.Printf("references: the test files of %s were not checked: %v", dir, err)
				if res.testErr == nil {
					res.testErr = fmt.Errorf("the test files of %s could not be checked: %v", dir, err)
				}
				continue
			}
		}
		res.tests[dir] = tc
		for _, root := range tc.Program.Roots {
			cp := tc.Result.Package(root)
			if cp == nil {
				continue
			}
			v := &view{dir: dir, fset: tc.Program.Fset, files: make(map[string]*ast.File), test: true}
			switch {
			case cp.Checked():
				v.types, v.info = cp.Types, cp.Info
			case cp.HasPartial():
				v.types, v.info = cp.PartialResult().Types, cp.PartialResult().Info
			default:
				continue
			}
			for _, f := range root.Files {
				if f.Test && f.AST != nil {
					v.files[f.Path] = f.AST
					v.addSynthetic(f)
				}
			}
			if len(v.files) > 0 {
				out = append(out, v)
			}
		}
	}
	return out, nil
}
