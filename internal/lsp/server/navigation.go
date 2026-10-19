package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/huuhait/gocpp/internal/syntax/gpast"
	"go/ast"
	"go/constant"
	goformat "go/format"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// The navigation features (step 11.5): hover (hover.go), go to definition
// and type definition (definition.go), and signature help (signature.go).
// Ideas and some code come from gopls (see COPIED.md).
//
// A request works on the snapshot it took on the read loop, so it sees the
// unsaved edits before it. It looks at the checked package of its document
// (cache.Snapshot.PackageOf), which comes from the cache when nothing
// changed: then a request costs a walk over the file's syntax tree, lookups
// in types.Info, and the parse (from the parse cache too) of the file that
// declares what it shows, for its doc comment and its declaration as the
// user wrote it. A file that doesn't parse answers from what go/types found
// anyway (check.Package.Partial).
//
// The checked syntax trees are the loader's, after its rewrites
// (raylib.InitWindow became C.InitWindow): their names keep the places of
// the names the user wrote, so they find the name at a place, but text
// that the features show comes from a fresh parse of the file, never from
// a rewritten tree.

// navigation holds what the navigation features learned from initialize:
// the markup the client wants.
type navigation struct {
	// hoverMarkdown: the hover text is Markdown, else plain text.
	hoverMarkdown atomic.Bool
	// sigMarkdown: the documentation of signature help is Markdown.
	sigMarkdown atomic.Bool
}

// initialize takes the client's markup choices from its capabilities and
// offers the navigation features in caps.
func (n *navigation) initialize(client protocol.ClientCapabilities, caps *protocol.ServerCapabilities) {
	hover := true
	if h := client.TextDocument.Hover; h != nil {
		hover = preferMarkdown(h.ContentFormat, true)
	}
	n.hoverMarkdown.Store(hover)
	sig := hover
	if sh := client.TextDocument.SignatureHelp; sh != nil && sh.SignatureInformation != nil {
		sig = preferMarkdown(sh.SignatureInformation.DocumentationFormat, hover)
	}
	n.sigMarkdown.Store(sig)
	caps.HoverProvider = &protocol.Or_ServerCapabilities_hoverProvider{Value: true}
	caps.DefinitionProvider = &protocol.Or_ServerCapabilities_definitionProvider{Value: true}
	caps.TypeDefinitionProvider = &protocol.Or_ServerCapabilities_typeDefinitionProvider{Value: true}
	caps.SignatureHelpProvider = &protocol.SignatureHelpOptions{TriggerCharacters: []string{"(", ","}}
}

// preferMarkdown reports whether the first markup kind of formats that the
// server knows is Markdown (the client lists them in the order it
// prefers), or def when formats has none (like gopls, Markdown by
// default).
func preferMarkdown(formats []protocol.MarkupKind, def bool) bool {
	for _, f := range formats {
		switch f {
		case protocol.Markdown:
			return true
		case protocol.PlainText:
			return false
		}
	}
	return def
}

// addNavigation adds the requests of the navigation features.
func (s *Server) addNavigation() {
	s.requests["textDocument/hover"] = method{run: s.hover}
	s.requests["textDocument/definition"] = method{run: s.definition}
	s.requests["textDocument/typeDefinition"] = method{run: s.typeDefinition}
	s.requests["textDocument/signatureHelp"] = method{run: s.signatureHelp}
}

// A query is the place of a request in its checked package.
type query struct {
	snap   *cache.Snapshot
	pkg    *cache.Package
	fset   *token.FileSet
	types  *types.Package
	info   *types.Info
	opaque *check.Opaque
	lp     *load.Package // the package of the document
	file   *load.File    // the document, as checked
	tok    *token.File
	src    []byte
	mapper *protocol.Mapper
	pos    token.Pos // the place of the request
	// exact holds the files that the program loaded from source, whose
	// positions have exact offsets (the others are from export data:
	// file and line only).
	exact map[string]bool
	// tests is the check with the test files that the query is in, for the
	// document of a _test.gp file (newRefQuery); pkg is nil then.
	tests *cache.TestCheck
}

// newQuery returns the query of the place at in the document uri, or nil
// when the document is not a file of its package (a file that the build
// constraints leave out, a _test.gp file) or has no types at all.
func newQuery(ctx context.Context, snap *cache.Snapshot, uri protocol.DocumentURI, at protocol.Position) (*query, error) {
	if err := ctx.Err(); err != nil {
		return nil, err // a cached package would answer anyway
	}
	path := uri.Path()
	if path == "" {
		return nil, nil
	}
	pkg, err := snap.PackageOf(ctx, uri)
	if errors.Is(err, cache.ErrNoPackage) {
		return nil, nil // a folder that has no package: nothing to answer from
	}
	if err != nil {
		return nil, err
	}
	src, err := snap.ReadFile(path)
	if err != nil {
		return nil, nil // no such file: then it is no file of the package either
	}
	return queryIn(snap, pkg, path, src, at), nil
}

// queryIn returns the query of the place at in the file path of the checked
// package pkg, whose text (the document's) is src, or nil (see newQuery).
// The check may have read a fixed text of the same length (completion's
// fixDangling): positions are byte offsets, which are the same.
func queryIn(snap *cache.Snapshot, pkg *cache.Package, path string, src []byte, at protocol.Position) *query {
	if len(pkg.Program.Roots) == 0 {
		return nil
	}
	q := &query{snap: snap, pkg: pkg, fset: pkg.Fset(), lp: pkg.Program.Roots[0], exact: make(map[string]bool)}
	cp := pkg.Check
	switch {
	case cp == nil:
		return nil
	case cp.Checked():
		q.types, q.info, q.opaque = cp.Types, cp.Info, cp.Opaque
	case cp.Partial != nil:
		q.types, q.info, q.opaque = cp.Partial.Types, cp.Partial.Info, cp.Partial.Opaque
	default:
		return nil
	}
	for _, lp := range pkg.Program.Packages {
		for _, f := range lp.Files {
			q.exact[f.Path] = true
		}
	}
	for _, f := range q.lp.Files {
		if f.Path == path {
			q.file = f
		}
	}
	if q.file == nil || q.file.AST == nil {
		return nil
	}
	q.tok = q.fset.File(q.file.AST.FileStart)
	q.src = src
	if q.tok == nil || q.tok.Size() != len(q.src) {
		return nil // can't be: the check read the same text
	}
	q.mapper = protocol.NewMapper(string(q.src))
	q.pos = q.tok.Pos(q.mapper.Offset(at))
	return q
}

// decodePosition decodes the params of a request at a place in a document.
func decodePosition(params json.RawMessage) (protocol.TextDocumentPositionParams, error) {
	var p protocol.TextDocumentPositionParams
	err := decode(params, &p)
	return p, err
}

// rangeOf returns the range of the nodes from pos to end in the document.
func (q *query) rangeOf(pos, end token.Pos) protocol.Range {
	return protocol.Range{Start: q.mapper.Position(q.tok.Offset(pos)), End: q.mapper.Position(q.tok.Offset(end))}
}

// contains reports whether the place of q is in the node n, or right
// after it.
func (q *query) contains(n ast.Node) bool {
	return n != nil && n.Pos().IsValid() && n.Pos() <= q.pos && q.pos <= n.End()
}

// identAt returns the name at pos in f and the nodes around it, from the
// file down (without the name), or nil. A name that ends at pos counts too
// (the cursor right after it) when no name starts or goes on there.
//
// The names that check wrote for the ! forms (File.Synthetic) are skipped.
//
// The names of the hidden import that check added (check.HiddenImport) are
// longer than the text they stand for, so they count only at their first
// character, and only as the C of a cgo helper (C.GoString): elsewhere the
// user's own name at that place is found (T in check's gp_cgoh.As[T](x)).
func identAt(lf *load.File, pos token.Pos) (*ast.Ident, []ast.Node) {
	f := lf.AST
	var id, after *ast.Ident
	var path, afterPath []ast.Node
	hidden := check.HiddenImport(f)
	ast.PreorderStack(f, nil, func(n ast.Node, stack []ast.Node) bool {
		if id != nil || n.Pos() > pos || pos > n.End() {
			return false
		}
		if x, ok := n.(*ast.Ident); ok && hidden != "" && x.Name == hidden {
			if sel, ok := stack[len(stack)-1].(*ast.SelectorExpr); ok && sel.X == x && check.IsHelperSelector(sel) && pos == x.Pos() {
				id, path = x, slices.Clone(stack)
			}
			return false
		}
		if x, ok := n.(*ast.Ident); ok && hidden != "" && len(stack) > 0 {
			// The Cmp and As of check's rewrites of C/C++ comparisons and
			// conversions (gp_cgoh.Cmp(x == y)) take the place of the
			// operand: the name there is the operand's.
			if sel, ok := stack[len(stack)-1].(*ast.SelectorExpr); ok && sel.Sel == x && !check.IsHelperSelector(sel) {
				if root, ok := sel.X.(*ast.Ident); ok && root.Name == hidden {
					return false
				}
			}
		}
		if x, ok := n.(*ast.Ident); ok && lf.Synthetic[x] {
			return false // a name that check wrote for a ! form: the user's text at the place is another node
		}
		if x, ok := n.(*ast.Ident); ok {
			if pos < x.End() {
				id, path = x, slices.Clone(stack)
			} else if after == nil {
				after, afterPath = x, slices.Clone(stack)
			}
		}
		return true
	})
	if id == nil {
		return after, afterPath
	}
	return id, path
}

// objectAt returns the object of the name id: what it declares or uses, or
// the implicit object of an unnamed import.
func (q *query) objectAt(id *ast.Ident) types.Object {
	if obj := q.info.Defs[id]; obj != nil {
		return obj
	}
	return q.info.Uses[id]
}

// qualifier returns how the types of the document name their packages: by
// the name that the file imports them with, and the package itself without
// a name (like gopls's FileQualifier).
func (q *query) qualifier() types.Qualifier {
	names := make(map[*types.Package]string)
	for _, spec := range q.file.AST.Imports {
		var obj types.Object
		if spec.Name != nil {
			obj = q.info.Defs[spec.Name]
		} else {
			obj = q.info.Implicits[spec]
		}
		if pn, ok := obj.(*types.PkgName); ok {
			if _, ok := names[pn.Imported()]; !ok {
				names[pn.Imported()] = pn.Name()
			}
		}
	}
	return func(p *types.Package) string {
		if p == q.types {
			return ""
		}
		if name, ok := names[p]; ok {
			if name == "." {
				return ""
			}
			return name
		}
		return p.Name()
	}
}

// isBuiltin reports whether obj is predeclared: in the universe (len,
// error, true, nil), or the Error method of error.
func isBuiltin(obj types.Object) bool {
	return obj.Pkg() == nil && !obj.Pos().IsValid()
}

// A declInfo is where an object is declared, in a fresh parse of its file
// (the user's text, which the loader didn't rewrite).
type declInfo struct {
	pf    *cache.ParsedFile
	ident *ast.Ident // the declaring name; nil when it wasn't found (see off)
	off   int        // the offset of the declaring name in pf.Src
	end   int        // the offset of its end, or 0: the end of the name
	// The declaration around it (gopls's findDeclInfo): any of them can
	// be nil, also all three (a local variable).
	decl  ast.Decl
	spec  ast.Spec
	field *ast.Field
}

// declOf returns where obj is declared, or nil when it has no place (a
// predeclared object, or one whose file can't be read). For an object of a
// package that the program loaded from source (the document's own), its
// position is exact. An object of an import comes from export data, whose
// positions have the file and the line only (the column is 1), and which
// can be old (an edit in the import that kept its API, see apiHash): then
// the declaring name of the object's name nearest to that line is taken,
// among the names that declare the same kind of object (sameKind). An
// import name has a place only at its import: not the import "C" that the
// loader adds (load.Rewrite.ImportC).
func (q *query) declOf(ctx context.Context, obj types.Object) (*declInfo, error) {
	return q.declIn(ctx, obj, q.fset)
}

// declIn is declOf for an object whose positions are in fset: the FileSet
// of the document's package, or of the API of a package that it doesn't
// import (cache.Exports: from export data, so never exact).
func (q *query) declIn(ctx context.Context, obj types.Object, fset *token.FileSet) (*declInfo, error) {
	if !obj.Pos().IsValid() {
		return nil, nil
	}
	posn := fset.Position(obj.Pos())
	if posn.Filename == "" {
		return nil, nil
	}
	pf, err := q.snap.ParsePath(ctx, posn.Filename, cache.ParseFull)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, nil // the file is gone
	}
	exact := fset == q.fset && q.exact[posn.Filename]
	recv := ""
	if fn, ok := obj.(*types.Func); ok {
		if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
			recv = typeName(sig.Recv().Type())
		}
	}
	d := &declInfo{pf: pf, off: -1}
	if _, ok := obj.(*types.PkgName); ok {
		if !exact {
			return nil, nil
		}
		// The import: all of it, like gopls.
		for _, spec := range pf.File.Imports {
			if off := pf.Tok.Offset(spec.Pos()); off == posn.Offset || pf.Tok.Offset(spec.Path.Pos()) == posn.Offset {
				d.off, d.end = off, pf.Tok.Offset(spec.End())
				return d, nil
			}
		}
		return nil, nil // an import that the user didn't write
	}
	best := -1
	ast.PreorderStack(pf.File, nil, func(n ast.Node, stack []ast.Node) bool {
		for _, c := range declaringNames(n, obj.Name()) {
			if fd, ok := n.(*ast.FuncDecl); ok && recvName(fd) != recv {
				continue // another type's method, or a function
			}
			off := pf.Tok.Offset(c.ident.Pos())
			dist := 0
			if exact {
				if off != posn.Offset {
					continue
				}
			} else {
				if !sameKind(obj, n, stack) {
					continue
				}
				dist = abs(pf.Tok.Line(c.ident.Pos()) - posn.Line)
			}
			if best < 0 || dist < best {
				best = dist
				d.ident, d.off, d.field = c.ident, off, c.field
				d.decl, d.spec = c.decl, c.spec
				for i := len(stack) - 1; i >= 0 && d.decl == nil; i-- {
					switch x := stack[i].(type) {
					case ast.Spec:
						if d.spec == nil {
							d.spec = x
						}
					case ast.Decl:
						d.decl = x
					}
				}
			}
		}
		return true
	})
	if d.ident == nil {
		if id := derivedMethodName(pf.File, obj, recv); id != nil {
			// A method that #Derive writes: the document of its package has the line, not the method.
			d.ident, d.off = id, pf.Tok.Offset(id.Pos())
			return d, nil
		}
		if !exact || posn.Offset > len(pf.Src) {
			return nil, nil
		}
		d.off = posn.Offset // a local variable, a label, ...
	}
	return d, nil
}

// derivedMethodName returns the name of the generator in the #Derive line of f that writes the
// method obj of the type recv, or nil (step 10.10: the parsed file has the line and no method).
func derivedMethodName(f *ast.File, obj types.Object, recv string) *ast.Ident {
	if _, ok := obj.(*types.Func); !ok || recv == "" {
		return nil
	}
	for i, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE || len(gd.Specs) != 1 || gd.Specs[0].(*ast.TypeSpec).Name.Name != recv {
			continue
		}
		for _, call := range gpast.OnTopOf(f.Decls, i) {
			if id, ok := call.Fun.(*ast.Ident); !ok || gpast.MacroBase(id.Name) != "Derive" {
				continue
			}
			for _, a := range call.Args {
				if id, ok := a.(*ast.Ident); ok && id.Name == obj.Name() {
					return id
				}
			}
		}
	}
	return nil
}

// sameKind reports whether the node n, under the nodes stack (from the
// file down), declares a name of the same kind as obj, an object of an
// import (from export data, so declared at the top level, or a field or
// method): a function a top-level function, a method a method of the same
// receiver type or a method of an interface type, a field a field of a
// struct, a package variable or constant a top-level var or const, a type
// a type. Never a parameter or a local variable.
func sameKind(obj types.Object, n ast.Node, stack []ast.Node) bool {
	parent := func(i int) ast.Node {
		if i <= len(stack) {
			return stack[len(stack)-i]
		}
		return nil
	}
	switch obj := obj.(type) {
	case *types.Func:
		sig, ok := obj.Type().(*types.Signature)
		if !ok {
			return false
		}
		if sig.Recv() == nil {
			fd, ok := n.(*ast.FuncDecl)
			return ok && fd.Recv == nil
		}
		if types.IsInterface(sig.Recv().Type()) {
			// A method in an interface type, of the interface named
			// like the receiver when it has a name.
			f, ok := n.(*ast.Field)
			if _, in := parent(2).(*ast.InterfaceType); !ok || !in || len(f.Names) == 0 {
				return false
			}
			name := typeName(sig.Recv().Type())
			ts, ok := parent(3).(*ast.TypeSpec)
			return name == "" || ok && ts.Name.Name == name
		}
		fd, ok := n.(*ast.FuncDecl)
		return ok && fd.Recv != nil // of the receiver type: see declOf
	case *types.Var:
		if obj.IsField() {
			_, ok := n.(*ast.Field)
			_, in := parent(2).(*ast.StructType)
			return ok && in
		}
		return topLevel(n, stack, token.VAR)
	case *types.Const:
		return topLevel(n, stack, token.CONST)
	case *types.TypeName:
		_, ok := n.(*ast.TypeSpec)
		return ok
	}
	return false
}

// topLevel reports whether n, under the nodes stack, is a spec of a
// top-level declaration with the token tok (var or const).
func topLevel(n ast.Node, stack []ast.Node, tok token.Token) bool {
	if _, ok := n.(*ast.ValueSpec); !ok || len(stack) != 2 {
		return false
	}
	gd, ok := stack[1].(*ast.GenDecl)
	return ok && gd.Tok == tok
}

// recvName returns the name of the receiver type of the method fd ("T" for
// (t *T) and (l List[E])), or "" for a function.
func recvName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	if id := embeddedName(fd.Recv.List[0].Type); id != nil {
		return id.Name
	}
	return ""
}

// typeName returns the name of the named type t or *t, or "".
func typeName(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		return t.Obj().Name()
	case *types.Interface:
		return "" // a method of a named interface from export data (see objectString)
	}
	return ""
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// A declName is a name that n declares, with the declaration it is in.
type declName struct {
	ident *ast.Ident
	decl  ast.Decl
	spec  ast.Spec
	field *ast.Field
}

// declaringNames returns the names called name that the node n declares
// itself (not in its children): of a function, a type, a constant or
// variable, a field, parameter or result, or the type name of an embedded
// field.
func declaringNames(n ast.Node, name string) []declName {
	var names []declName
	switch n := n.(type) {
	case *ast.FuncDecl:
		if base := gpast.MacroBase(gpast.MustBase(n.Name.Name)); base == name {
			id := n.Name
			if base != id.Name {
				// load!(...) or macro Name(...): the tree has the mark in the name, the source has
				// the name alone at its place (a macro's name has no #).
				id = &ast.Ident{NamePos: id.NamePos, Name: base}
			}
			names = append(names, declName{ident: id, decl: n})
		}
	case *ast.TypeSpec:
		if n.Name.Name == name {
			names = append(names, declName{ident: n.Name, spec: n})
		}
	case *ast.ValueSpec:
		for _, id := range n.Names {
			if id.Name == name {
				names = append(names, declName{ident: id, spec: n})
			}
		}
	case *ast.Field:
		for _, id := range n.Names {
			if id.Name == name {
				names = append(names, declName{ident: id, field: n})
			}
		}
		if len(n.Names) == 0 {
			if id := embeddedName(n.Type); id != nil && id.Name == name {
				names = append(names, declName{ident: id, field: n})
			}
		}
	case *ast.AssignStmt:
		if n.Tok == token.DEFINE {
			for _, x := range n.Lhs {
				if id, ok := x.(*ast.Ident); ok && id.Name == name {
					names = append(names, declName{ident: id})
				}
			}
		}
	}
	return names
}

// embeddedName returns the type name of an embedded field: T in T, *T,
// pkg.T and T[int].
func embeddedName(x ast.Expr) *ast.Ident {
	for {
		switch t := x.(type) {
		case *ast.StarExpr:
			x = t.X
		case *ast.IndexExpr:
			x = t.X
		case *ast.IndexListExpr:
			x = t.X
		case *ast.SelectorExpr:
			return t.Sel
		case *ast.Ident:
			return t
		default:
			return nil
		}
	}
}

// docOf returns the doc comment of a declaration (gopls's
// chooseDocComment): the field's, else the spec's, else the declaration's,
// else the comment at the end of the line.
func (d *declInfo) docOf() *ast.CommentGroup {
	if d == nil {
		return nil
	}
	if d.field != nil {
		if d.field.Doc != nil {
			return d.field.Doc
		}
		return d.field.Comment
	}
	switch decl := d.decl.(type) {
	case *ast.FuncDecl:
		return decl.Doc
	case *ast.GenDecl:
		switch spec := d.spec.(type) {
		case *ast.ValueSpec:
			if spec.Doc != nil {
				return spec.Doc
			}
			if decl.Doc != nil {
				return decl.Doc
			}
			return spec.Comment
		case *ast.TypeSpec:
			if spec.Doc != nil {
				return spec.Doc
			}
			if decl.Doc != nil {
				return decl.Doc
			}
			return spec.Comment
		}
	}
	return nil
}

// formatNode returns the Go source of n, a node of the file of fset, as
// gofmt prints it.
func formatNode(fset *token.FileSet, n ast.Node) string {
	var b bytes.Buffer
	if err := goformat.Node(&b, fset, n); err != nil {
		return ""
	}
	return b.String()
}

// spelling returns the source text of the node n of the program (in a file
// of the document's package): how the user wrote it, also where the loader
// rewrote it (*SDL.SDL_Window, not *C.SDL_Window).
func (q *query) spelling(n ast.Node) string {
	posn, end := q.fset.Position(n.Pos()), q.fset.Position(n.End())
	if !q.exact[posn.Filename] || posn.Filename != end.Filename {
		return ""
	}
	src, err := q.snap.ReadFile(posn.Filename)
	if err != nil || end.Offset > len(src) || posn.Offset > end.Offset {
		return ""
	}
	return string(src[posn.Offset:end.Offset])
}

// opaqueSpelling returns the type expression that v was declared with, as
// the user wrote it, when its type comes from C/C++, or "": for an object of
// the document's package from its opaque pass (check.Opaque.TypeExpr), and
// for one of an import from the side table of its summary
// (cache.Package.OpaqueType, step 11.3b), where the export data has
// "invalid type".
func (q *query) opaqueSpelling(v *types.Var) string {
	if q.types == nil {
		return ""
	}
	if v.Pkg() != q.types {
		t, _ := q.pkg.OpaqueType(v)
		return t
	}
	if te := q.opaque.TypeExpr(v.Origin()); te != nil {
		return q.spelling(ast.Unparen(te)) // w (*C.X): *C.X
	}
	return ""
}

// typeString returns the type t as Go writes it, like types.TypeString,
// but with the C/C++ parts of a struct, pointer, slice, array, map or
// channel type as the source wrote them, where a field has a spelling
// (opaqueSpelling): a variable declared without a type, var v =
// struct{ W *SDL.SDL_Window }{}, shows struct{W *SDL.SDL_Window}.
func (q *query) typeString(t types.Type, qual types.Qualifier) string {
	if !cache.Incomplete(t) {
		return types.TypeString(t, qual)
	}
	switch t := t.(type) {
	case *types.Pointer:
		return "*" + q.typeString(t.Elem(), qual)
	case *types.Slice:
		return "[]" + q.typeString(t.Elem(), qual)
	case *types.Array:
		return fmt.Sprintf("[%d]%s", t.Len(), q.typeString(t.Elem(), qual))
	case *types.Map:
		return "map[" + q.typeString(t.Key(), qual) + "]" + q.typeString(t.Elem(), qual)
	case *types.Chan:
		dir := "chan "
		switch t.Dir() {
		case types.SendOnly:
			dir = "chan<- "
		case types.RecvOnly:
			dir = "<-chan "
		}
		return dir + q.typeString(t.Elem(), qual)
	case *types.Struct:
		var b strings.Builder
		b.WriteString("struct{")
		for i := range t.NumFields() {
			if i > 0 {
				b.WriteString("; ")
			}
			f := t.Field(i)
			ft := q.opaqueSpelling(f)
			if ft == "" {
				ft = q.typeString(f.Type(), qual)
			}
			if !f.Embedded() {
				b.WriteString(f.Name())
				b.WriteByte(' ')
			}
			b.WriteString(ft)
			if tag := t.Tag(i); tag != "" {
				b.WriteByte(' ')
				b.WriteString(strconv.Quote(tag))
			}
		}
		b.WriteByte('}')
		return b.String()
	}
	return types.TypeString(t, qual)
}

// A funcSpelling is the types of the parameters and the results of a
// function whose signature has C/C++ parts, as its source wrote them: one
// entry per parameter or result ("*SDL.SDL_Window", "...C.int").
type funcSpelling struct {
	params, results []string
}

// funcSpellingOf returns the spelling of the signature of fn
// (cache.Package.OpaqueType), or nil when it has no C/C++ parts or no
// spelling (a package that has load errors only has the types of its
// parameters: opaqueSpelling).
func (q *query) funcSpellingOf(fn *types.Func) *funcSpelling {
	text, ok := q.pkg.OpaqueType(fn)
	if !ok {
		return nil
	}
	return parseFuncSpelling(text, fn)
}

// parseFuncSpelling returns the spelling of the signature of fn from its
// text, "func(w *SDL.SDL_Window) *SDL.SDL_Renderer" (OpaqueType), or nil
// when the text doesn't fit the signature.
func parseFuncSpelling(text string, fn *types.Func) *funcSpelling {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return nil
	}
	// "func[T any](x T) *C.X": a function type with type parameters is
	// a syntax error in an expression, but it parses.
	e, _ := parser.ParseExpr(text)
	ft, ok := e.(*ast.FuncType)
	if !ok {
		return nil
	}
	fields := func(list *ast.FieldList, want int) []string {
		var types []string
		if list != nil {
			for _, f := range list.List {
				// The FileSet of ParseExpr has only text, at base 1;
				// w (*C.X) is w *C.X.
				typ := ast.Unparen(f.Type)
				start, end := int(typ.Pos())-1, int(typ.End())-1
				if start < 0 || end > len(text) || start > end {
					return nil
				}
				for range max(1, len(f.Names)) {
					types = append(types, text[start:end])
				}
			}
		}
		if len(types) != want {
			return nil
		}
		return types
	}
	sp := &funcSpelling{params: fields(ft.Params, sig.Params().Len()), results: fields(ft.Results, sig.Results().Len())}
	if sp.params == nil && sig.Params().Len() > 0 || sp.results == nil && sig.Results().Len() > 0 {
		return nil
	}
	return sp
}

// funcSpellingFor returns funcSpellingOf(obj) for a function, else nil.
func (q *query) funcSpellingFor(obj types.Object) *funcSpelling {
	if fn, ok := obj.(*types.Func); ok {
		return q.funcSpellingOf(fn)
	}
	return nil
}

// tupleString returns the parameters or results t, each with its name:
// "x int", "args ...any" (with variadic, the last one is ...T). spelled,
// if not nil, has the types as the source wrote them (funcSpelling): they
// are used for the types that have C/C++ parts.
func (q *query) tupleString(t *types.Tuple, variadic bool, qual types.Qualifier, spelled []string) []string {
	list := make([]string, t.Len())
	for i := range t.Len() {
		v := t.At(i)
		typ := ""
		if spelled != nil && cache.Incomplete(v.Type()) {
			typ = spelled[i] // ...T already, for the last parameter
		}
		if typ == "" {
			typ = q.opaqueSpelling(v) // ...T already too
		}
		if typ == "" {
			typ = types.TypeString(v.Type(), qual)
			if s, ok := v.Type().(*types.Slice); ok && variadic && i == t.Len()-1 {
				typ = "..." + types.TypeString(s.Elem(), qual)
			}
		}
		if v.Name() != "" {
			typ = v.Name() + " " + typ
		}
		list[i] = typ
	}
	return list
}

// signatureString returns the parameters and the results of sig, as Go
// writes them after the name of a function: "(x int) (string, error)". sp
// is the spelling of the function's C/C++ types, or nil.
func (q *query) signatureString(sig *types.Signature, qual types.Qualifier, sp *funcSpelling) string {
	if sp == nil {
		sp = &funcSpelling{}
	}
	var b bytes.Buffer
	b.WriteByte('(')
	for i, p := range q.tupleString(sig.Params(), sig.Variadic(), qual, sp.params) {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p)
	}
	b.WriteByte(')')
	results := q.tupleString(sig.Results(), false, qual, sp.results)
	if len(results) == 0 {
		return b.String()
	}
	b.WriteByte(' ')
	parens := len(results) > 1 || sig.Results().At(0).Name() != ""
	if parens {
		b.WriteByte('(')
	}
	for i, r := range results {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(r)
	}
	if parens {
		b.WriteByte(')')
	}
	return b.String()
}

// typeParamsString returns the type parameters of a generic function,
// "[T any]", or "".
func typeParamsString(list *types.TypeParamList, qual types.Qualifier) string {
	if list.Len() == 0 {
		return ""
	}
	var b bytes.Buffer
	b.WriteByte('[')
	for i := range list.Len() {
		if i > 0 {
			b.WriteString(", ")
		}
		tp := list.At(i)
		b.WriteString(tp.Obj().Name())
		b.WriteByte(' ')
		types.WriteType(&b, tp.Constraint(), qual)
	}
	b.WriteByte(']')
	return b.String()
}

// objectString returns the declaration of obj as hover shows it, like
// types.ObjectString with gopls's changes: a method shows its receiver
// name, "func (t T) String() string", and a constant its value,
// "const C untyped int = 3" (the expression it was declared with, when
// that says more: "const K = 1 << 10 // 1024"). A type that comes from
// C/C++ shows as it was written (opaqueSpelling, funcSpellingOf), also
// for an object of an import. d is where obj is declared, or nil.
func (q *query) objectString(obj types.Object, qual types.Qualifier, d *declInfo) string {
	switch obj := obj.(type) {
	case *types.Func:
		sig, ok := obj.Type().(*types.Signature)
		if !ok {
			break
		}
		var b bytes.Buffer
		b.WriteString("func ")
		if recv := sig.Recv(); recv != nil {
			b.WriteByte('(')
			if _, ok := recv.Type().(*types.Interface); ok {
				// The export data makes the methods of a named
				// interface with the interface as the receiver.
				b.WriteString("interface")
			} else {
				if name := recv.Name(); name != "" && name != "_" {
					b.WriteString(name)
					b.WriteByte(' ')
				}
				types.WriteType(&b, recv.Type(), qual)
			}
			b.WriteString(") ")
		} else if s := qual(obj.Pkg()); s != "" {
			b.WriteString(s)
			b.WriteByte('.')
		}
		b.WriteString(obj.Name())
		b.WriteString(typeParamsString(sig.TypeParams(), qual))
		b.WriteString(q.signatureString(sig, qual, q.funcSpellingOf(obj)))
		return b.String()
	case *types.Var:
		if typ := q.opaqueSpelling(obj); typ != "" {
			kind, name := "var ", obj.Name()
			if obj.IsField() {
				kind = "field "
			} else if p := obj.Pkg(); p != nil && p.Scope().Lookup(name) == obj && qual(p) != "" {
				name = qual(p) + "." + name // a variable of an import: var lib.Default
			}
			return kind + name + " " + typ
		}
		if obj.Type() != types.Typ[types.Invalid] && cache.Incomplete(obj.Type()) {
			// A C/C++ part without a type expression of its own: var
			// v = struct{ W *SDL.SDL_Window }{}.
			kind, name := "var ", obj.Name()
			if obj.IsField() {
				kind = "field "
			} else if p := obj.Pkg(); p != nil && p.Scope().Lookup(name) == obj && qual(p) != "" {
				name = qual(p) + "." + name
			}
			return kind + name + " " + q.typeString(obj.Type(), qual)
		}
	case *types.Const:
		return constString(obj, qual, d)
	}
	return types.ObjectString(obj, qual)
}

// constString returns the declaration of the constant obj (gopls's
// objectString): with its value, or with the expression it was declared
// with and the value as a comment when they differ. A string constant
// shows only its value.
func constString(obj *types.Const, qual types.Qualifier, d *declInfo) string {
	str := types.ObjectString(obj, qual)
	decl := obj.Val().String()
	comment := ""
	if spec, ok := declSpec(d).(*ast.ValueSpec); ok && obj.Val().Kind() != constant.String {
		for i, name := range spec.Names {
			if name == d.ident && i < len(spec.Values) {
				if orig := formatNode(d.pf.Fset, spec.Values[i]); orig != "" && orig != decl {
					comment, decl = decl, orig
				}
			}
		}
	}
	str += " = " + decl
	if comment != "" {
		str += " // " + comment
	}
	return str
}

// declSpec returns the spec of d, or nil.
func declSpec(d *declInfo) ast.Spec {
	if d == nil {
		return nil
	}
	return d.spec
}

// importPath returns the path of an import, without the quotes.
func importPath(spec *ast.ImportSpec) string {
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return ""
	}
	return path
}
