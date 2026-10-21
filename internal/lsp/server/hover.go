package server

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/doc/comment"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/types/typeutil"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/gpast"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// textDocument/hover (step 11.5): like gopls, the declaration of the name
// under the cursor as Go code, its doc comment, and for a type its
// methods; for a package its doc and import path. A C/C++ name (raylib.X,
// std.vector) and a header import show what the checker knows: where the
// name comes from, and that its type is opaque. Full C/C++ hover, from a
// header index, is Phase 12. C itself (C.int) shows a line on C's
// built-in types: it is no package the user imports (load.Rewrite.ImportC).
//
// The name of a type switch (x in switch x := v.(type)) has no object in
// go/types where it is declared (one per clause instead): there, like gopls,
// hover shows it with the type of v; in a clause, the clause's variable.

// A hoverResult is a hover before it is formatted (gopls's hoverResult).
type hoverResult struct {
	signature         string // declaration, or a proven native type when signatureLanguage is cpp
	signatureLanguage string // empty means Go; applies only to the signature
	doc               string // the doc comment, as text (go/doc/comment syntax)
	methods           string // Go code: the methods of a type, or ""
	footer            string // plain text lines, or ""
	// text is plain text after the signature, for the hovers that have
	// no doc comment of their own (C/C++ names, header imports).
	text string
	// expansion is Go code after the text and the doc: what a `!` or a macro use becomes (step 11.13).
	expansion string
}

func (s *Server) hover(ctx context.Context, e env, params json.RawMessage) (any, error) {
	p, err := decodePosition(params)
	if err != nil {
		return nil, err
	}
	h, rng, err := s.hoverAt(ctx, e, p, true)
	if h == nil || err != nil {
		return nil, err
	}
	markdown := s.nav.hoverMarkdown.Load()
	kind := protocol.PlainText
	if markdown {
		kind = protocol.Markdown
	}
	return &protocol.Hover{Contents: protocol.MarkupContent{Kind: kind, Value: h.format(markdown)}, Range: rng}, nil
}

// hoverAt returns the hover at the place of p, or nil. For a file with a syntax error it may come from a view of an older
// text (stale.go) when allowStale is true, moved to the text of the request; when the range of the hover cannot be moved
// (a word next to the change), the hover is made again from a view of the text itself.
func (s *Server) hoverAt(ctx context.Context, e env, p protocol.TextDocumentPositionParams, allowStale bool) (*hoverResult, protocol.Range, error) {
	q, err := newHoverQuery(ctx, e, p.TextDocument.URI, p.Position, allowStale)
	if q == nil || err != nil {
		return nil, protocol.Range{}, err
	}
	defer q.close()
	h, rng, err := q.hover(ctx)
	if q.isFixed && err == nil && (h == nil || strings.Contains(h.signature, "invalid type")) && q.origFn != nil {
		// The repaired check has no answer (or an invalid one) where the text as it is has: a name that a blanked
		// declaration declares (a type), say.
		if o := q.origFn(); o != nil {
			if h2, rng2, err2 := o.hover(ctx); err2 == nil && h2 != nil {
				h, rng = h2, rng2
			}
		}
	}
	if h != nil && err == nil && q.shift != nil {
		nr, ok := q.newRange(rng)
		if !ok {
			q.close()
			return s.hoverAt(ctx, e, p, false)
		}
		rng = nr
	}
	return h, rng, err
}

// hover returns the hover at the place of q and the range it is for, or
// nil.
func (q *query) hover(ctx context.Context) (*hoverResult, protocol.Range, error) {
	f := q.file.AST
	// The package clause.
	if q.contains(f.Name) {
		h, err := q.packageHover(q.lp, q.files(), true)
		return h, q.rangeOf(f.Name.Pos(), f.Name.End()), err
	}
	// An import path: of a gocpp package, a header or a namespace rename.
	if info := q.file.Info; info != nil {
		for _, spec := range info.Packages {
			if q.contains(spec.Path) {
				dep := q.lp.Imports[importPath(spec)]
				if dep == nil {
					return nil, protocol.Range{}, nil
				}
				files, err := q.snap.PackageFiles(dep)
				if err != nil {
					return nil, protocol.Range{}, err
				}
				h, err := q.packageHover(dep, files, false)
				return h, q.rangeOf(spec.Path.Pos(), spec.Path.End()), err
			}
		}
		// These imports are not in the checked tree any more (the
		// loader dropped them): the path or the name given in the import.
		for _, inc := range info.Includes {
			if n := q.specPart(inc.Spec); n != nil {
				h, err := q.headerHover(ctx, inc)
				return h, q.rangeOf(n.Pos(), n.End()), err
			}
		}
		for _, rn := range info.NamespaceRenames {
			if n := q.specPart(rn.Spec); n != nil {
				h := &hoverResult{
					signature: fmt.Sprintf("import %s %q", rn.Name, rn.Namespace),
					text:      fmt.Sprintf("The C++ namespace `%s`, named `%s` in this file.", rn.Namespace, rn.Name),
				}
				return h, q.rangeOf(n.Pos(), n.End()), nil
			}
		}
	}
	// The # and the name of a #Derive line (step 11.13).
	if h, rng := q.deriveHover(); h != nil {
		return h, rng, nil
	}
	// A C/C++ name.
	if h, n := q.cppHover(); h != nil {
		return h, q.rangeOf(n.Pos(), n.End()), nil
	}
	id, path := identAt(q.file, q.pos)
	// The name of a `!` call that the lowering moved (code of its own: a loop's condition, a deferred call, the
	// second operand of &&, a chain): the call has its place, and its name the checker's object.
	var moved *bangCall
	if c := q.bang().callAt(q.pos); c != nil && id == nil && q.objectAt(c.Callee) != nil {
		moved = c
		id, path = c.Callee, nil
	}
	if id == nil {
		if f := q.bang().funcAt(q.pos); f != nil {
			h, rng := q.exitFuncHover(f)
			return h, rng, nil
		}
		return nil, protocol.Range{}, nil
	}
	obj := q.objectAt(id)
	rng := q.rangeOf(id.Pos(), id.End())
	if moved != nil {
		rng = q.rangeOf(moved.Name, moved.Name+token.Pos(len(id.Name)))
	}
	if len(path) > 0 {
		// A cgo helper, C.GoString, which check rewrote to a function of its hidden package.
		if sel, ok := path[len(path)-1].(*ast.SelectorExpr); ok {
			if name := check.CgoHelperOf(sel, q.info); name != "" {
				if id == sel.X {
					return cHover(), q.rangeOf(id.Pos(), id.Pos()+1), nil // (the user's C)
				}
				d := check.CgoHelperDocs[name]
				return &hoverResult{signature: d[0], text: d[1]}, rng, nil
			}
		}
	}
	if obj == nil {
		if h := q.typeSwitchHover(id, path); h != nil {
			return h, rng, nil
		}
		// A member of a C/C++ value: nums.push_back.
		if len(path) > 0 {
			if sel, ok := path[len(path)-1].(*ast.SelectorExpr); ok && sel.Sel == id && q.opaque.IsOpaque(sel.X) {
				h := &hoverResult{
					signature: q.spelling(sel) + " // C/C++",
					text:      "A member of a C/C++ value. Its type is opaque to gocpp: the C++ compiler checks it.",
				}
				return h, rng, nil
			}
		}
		return nil, protocol.Range{}, nil
	}
	if isBuiltin(obj) {
		h := builtinHover(obj)
		return h, rng, nil
	}
	if pn, ok := obj.(*types.PkgName); ok {
		if pn.Imported().Path() == "C" {
			return cHover(), rng, nil
		}
		dep := q.lp.Imports[pn.Imported().Path()]
		if dep == nil {
			return &hoverResult{signature: "package " + pn.Imported().Name()}, rng, nil
		}
		files, err := q.snap.PackageFiles(dep)
		if err != nil {
			return nil, protocol.Range{}, err
		}
		h, err := q.packageHover(dep, files, false)
		return h, rng, err
	}
	h, err := q.objectHover(ctx, obj, id, path)
	if h != nil && err == nil {
		c := moved
		if c == nil {
			c = q.bang().callOf(id)
		}
		q.bangHover(h, obj, c)
		q.macroUseHover(h, id)
	}
	return h, rng, err
}

// cHover returns the hover of C (C.int, C.GoString).
func cHover() *hoverResult {
	return &hoverResult{
		signature: "package C",
		text:      "C's built-in types and helpers (`C.int`, `C.size_t`, `C.CString`), which need no import in gocpp. Their types are opaque to gocpp: the C++ compiler checks them.",
	}
}

// typeSwitchHover returns the hover of the name id of a type switch where
// it is declared (switch x := v.(type)), at the end of path, or nil:
// gopls's "var x T", with the type of v.
func (q *query) typeSwitchHover(id *ast.Ident, path []ast.Node) *hoverResult {
	if len(path) < 2 {
		return nil
	}
	assign, ok := path[len(path)-1].(*ast.AssignStmt)
	if !ok || len(assign.Lhs) != 1 || assign.Lhs[0] != id || len(assign.Rhs) != 1 {
		return nil
	}
	sw, ok := path[len(path)-2].(*ast.TypeSwitchStmt)
	ta, ok2 := assign.Rhs[0].(*ast.TypeAssertExpr)
	if !ok || !ok2 || sw.Assign != assign {
		return nil
	}
	t := q.info.TypeOf(ta.X)
	if t == nil || t == types.Typ[types.Invalid] {
		return nil
	}
	return &hoverResult{signature: "var " + id.Name + " " + types.TypeString(t, q.qualifier())}
}

// specPart returns the name or the path of the import spec that the place
// of q is in, or nil.
func (q *query) specPart(spec *ast.ImportSpec) ast.Node {
	switch {
	case spec.Name != nil && q.contains(spec.Name):
		return spec.Name
	case q.contains(spec.Path):
		return spec.Path
	}
	return nil
}

// files returns the names of the files of the document's package.
func (q *query) files() []string {
	names := make([]string, len(q.lp.Files))
	for i, f := range q.lp.Files {
		names[i] = f.Path
	}
	return names
}

// objectHover returns the hover of the object obj, which the name id at
// the end of path declares or uses.
func (q *query) objectHover(ctx context.Context, obj types.Object, id *ast.Ident, path []ast.Node) (*hoverResult, error) {
	qual := q.qualifier()
	d, err := q.declOf(ctx, obj)
	if err != nil {
		return nil, err
	}
	h := &hoverResult{}
	if doc := d.docOf(); doc != nil {
		h.doc = doc.Text()
	}
	h.signature = q.objectString(obj, qual, d)
	if sig := q.macroOf(obj); sig != nil {
		h.signature = sig.Sig // macro Assert(cond bool, msg lazy string), not the hidden parameters of its function
	}
	if d != nil {
		if fd, ok := d.decl.(*ast.FuncDecl); ok && fd.Name != nil && gpast.IsMust(fd.Name.Name) {
			// A must function (step 11.13): the signature above is the one callers see; this is the one written.
			h.text = joinParas(h.text, mustFuncNote(mustDecl(d.pf.Fset, fd)))
		}
	}
	if v, ok := obj.(*types.Var); ok && q.cppValue(v) {
		// x := lib.f(): its type comes from C/C++, and has no spelling.
		kind := "var "
		if v.IsField() {
			kind = "field "
		}
		h.signature = kind + v.Name() + " // C/C++"
		h.text = "Its type comes from C/C++ and is opaque to gocpp: the C++ compiler checks it."
	}
	if v, ok := obj.(*types.Var); ok {
		// The ordinary hover stays the base (its doc comment is kept); a completed background
		// fact only replaces the signature and the note about the type.
		q.addNativeType(h, v)
	}

	// A promoted field: through which embedded fields (gopls).
	if v, ok := obj.(*types.Var); ok && v.IsField() && len(path) > 0 {
		if sel, ok := path[len(path)-1].(*ast.SelectorExpr); ok && sel.Sel == id {
			if through := q.through(sel); through != "" {
				h.signature += " // through " + through
			}
		}
	}
	if d != nil && d.field != nil && d.field.Tag != nil {
		h.signature += " " + d.field.Tag.Value
	}

	tn, ok := obj.(*types.TypeName)
	if !ok {
		return h, nil
	}
	if _, ok := types.Unalias(tn.Type()).(*types.TypeParam); ok {
		return h, nil
	}
	// A type: its declaration as written (gopls's typeDeclContent), then
	// its methods.
	var skip map[string]bool
	if spec, ok := declSpec(d).(*ast.TypeSpec); ok {
		spec2 := *spec
		spec2.Doc, spec2.Comment = nil, nil
		if s := formatNode(d.pf.Fset, &spec2); s != "" {
			h.signature = "type " + s
		}
		if iface, ok := spec.Type.(*ast.InterfaceType); ok && iface.Methods != nil {
			skip = make(map[string]bool)
			for _, m := range iface.Methods.List {
				if len(m.Names) == 1 {
					skip[m.Names[0].Name] = true
				}
			}
		}
	}
	var methods []string
	for _, m := range typeutil.IntuitiveMethodSet(tn.Type(), nil) {
		mobj := m.Obj()
		if !mobj.Exported() && mobj.Pkg() != q.types || skip[mobj.Name()] {
			continue
		}
		methods = append(methods, q.objectString(mobj, qual, nil))
	}
	h.methods = strings.Join(methods, "\n")
	return h, nil
}

// macroOf returns the signature of the macro whose function obj is (step 10.9: the loader turns a
// macro into a function of the same name), or nil.
func (q *query) macroOf(obj types.Object) *load.MacroSig {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Parent() != fn.Pkg().Scope() {
		return nil
	}
	if fn.Pkg() == q.types {
		return q.lp.Macros[fn.Name()]
	}
	if dep := q.lp.Imports[fn.Pkg().Path()]; dep != nil {
		return dep.Macros[fn.Name()]
	}
	return nil
}

// addNativeType replaces the signature and text of the hover h of the variable with its
// compiler-proven native type, when the exact original binding has a completed background fact.
// Only the retained fact is read: nothing is probed and nothing waits on the compiler. A repaired
// or stale view never matches a fact: its package is a variant, which the lookup rejects, and its
// variable objects are not the original bindings.
func (q *query) addNativeType(h *hoverResult, variable *types.Var) {
	native, incomplete, ready := q.snap.ForeignVariableType(q.pkg, variable)
	if !ready {
		return
	}
	text := fmt.Sprintf("C/C++ native type of variable `%s`.", variable.Name())
	if incomplete {
		text += " Member completion is partial;" +
			" this type does not imply exhaustive member information."
	}
	h.signature = native
	h.signatureLanguage = "cpp"
	h.text = text
}

// cppValue reports whether the variable v of the document's package has
// no type in go/types because its value comes from C/C++: it is declared
// without a type, from an opaque value (x := lib.f(), the variables of a
// range over one), as check.Opaque says.
func (q *query) cppValue(v *types.Var) bool {
	if v.Pkg() != q.types || v.Type() != types.Typ[types.Invalid] || q.opaqueSpelling(v) != "" {
		return false
	}
	posn := q.fset.Position(v.Pos())
	for _, f := range q.lp.Files {
		if f.Path != posn.Filename {
			continue
		}
		id, path := identAt(f, v.Pos())
		if id == nil {
			return false
		}
		for i := len(path) - 1; i >= 0; i-- {
			switch n := path[i].(type) {
			case *ast.AssignStmt:
				for j, x := range n.Lhs {
					if x == id && len(n.Rhs) > 0 {
						return q.opaque.IsOpaque(n.Rhs[min(j, len(n.Rhs)-1)])
					}
				}
				return false
			case *ast.ValueSpec:
				for j, x := range n.Names {
					if x == id && len(n.Values) > 0 {
						return q.opaque.IsOpaque(n.Values[min(j, len(n.Values)-1)])
					}
				}
				return false
			case *ast.RangeStmt:
				return q.opaque.IsOpaque(n.X)
			}
		}
	}
	return false
}

// through returns the embedded fields that the selection sel goes
// through, "Base, *Inner", or "".
func (q *query) through(sel *ast.SelectorExpr) string {
	s := q.info.Selections[sel]
	if s == nil || len(s.Index()) < 2 {
		return ""
	}
	var parts []string
	t := s.Recv()
	for _, index := range s.Index()[:len(s.Index())-1] {
		if p, ok := t.Underlying().(*types.Pointer); ok {
			t = p.Elem()
		}
		st, ok := t.Underlying().(*types.Struct)
		if !ok || index >= st.NumFields() {
			return ""
		}
		field := st.Field(index)
		t = field.Type()
		name := ""
		if p, ok := t.(*types.Pointer); ok {
			name = "*"
			t = p.Elem()
		}
		if n, ok := types.Unalias(t).(*types.Named); ok {
			name += n.Obj().Name()
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, ", ")
}

// packageHover returns the hover of the package lp with the files: its
// name, its package doc (from the first file that has one, like gopls) and
// its import path. own is set for the package of the document.
func (q *query) packageHover(lp *load.Package, files []string, own bool) (*hoverResult, error) {
	h := &hoverResult{signature: "package " + lp.Name}
	// TODO(Phase 11): files has the files that the build constraints leave
	// out too (see packageLocations).
	if lp.Name == "" {
		h.signature = "package " + filepath.Base(lp.ImportPath)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.gp") {
			continue
		}
		src, err := q.snap.ReadFile(path)
		if err != nil {
			continue
		}
		if f, _ := parsePackageClause(path, src); f != nil && f.Doc != nil {
			h.doc = f.Doc.Text()
			break
		}
	}
	if lp.ImportPath != "command-line-arguments" {
		h.footer = " - Package path: " + lp.ImportPath
	}
	if own && lp.Module != nil && lp.Module.Path != "" {
		h.footer += "\n - Module: " + lp.Module.Path
	}
	h.footer = strings.TrimPrefix(h.footer, "\n")
	return h, nil
}

// parsePackageClause parses the file up to its package clause, with its
// comments (cheap: no cache needed), and returns it and its token.File;
// the file is nil when it has no package clause.
func parsePackageClause(path string, src []byte) (*ast.File, *token.File) {
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, path, src, parser.PackageClauseOnly|parser.ParseComments)
	if f == nil {
		return nil, nil
	}
	return f, fset.File(f.FileStart)
}

// headerHover returns the hover of the header import inc: the header, its
// package name in the file, and its file when it is found.
func (q *query) headerHover(ctx context.Context, inc *load.Include) (*hoverResult, error) {
	h := &hoverResult{signature: fmt.Sprintf("import %q // C/C++ header", inc.Path)}
	if inc.Spec.Name != nil {
		h.signature = fmt.Sprintf("import %s %q // C/C++ header", inc.Spec.Name.Name, inc.Path)
	}
	var lines []string
	if inc.Name != "" {
		lines = append(lines, fmt.Sprintf("Its C-level names are `%s.X` in this file. Their types are opaque to gocpp: the C++ compiler checks them.", inc.Name))
	} else if why := noNameText(inc); why != "" {
		lines = append(lines, "It has no package name in this file: "+why+".")
	}
	path, err := q.headerFile(ctx, inc)
	if err != nil {
		return nil, err
	}
	if path != "" {
		lines = append(lines, "File: `"+path+"`")
	}
	h.text = strings.Join(lines, "\n\n")
	return h, nil
}

// noNameText says why the header import inc gives no package name, or ""
// when the user chose that (_).
func noNameText(inc *load.Include) string {
	n := "`" + inc.WouldBe + "`"
	switch inc.Why {
	case load.Keyword:
		return n + " is a Go keyword"
	case load.Predeclared:
		return n + " is a predeclared Go name"
	case load.Reserved:
		return n + " is reserved"
	case load.NotAName:
		return n + " is not a valid Go name"
	case load.UsedByImport:
		return "another import of the file uses " + n
	case load.UsedByPackage:
		return "a package-level name uses " + n
	}
	return ""
}

// headerFile returns the file of the header import inc of the document, or
// "" (cache.Pipeline.HeaderFile).
func (q *query) headerFile(ctx context.Context, inc *load.Include) (string, error) {
	path, err := q.snap.HeaderFile(ctx, filepath.Dir(q.file.Path), inc)
	if err != nil && ctx.Err() != nil {
		return "", ctx.Err()
	}
	return path, nil // a compiler that fails: no file
}

// cppHover returns the hover of the C/C++ name at the place of q, and the
// name it is for, or nil: a header package or namespace name (raylib,
// std, fs) or a name after one (raylib.InitWindow, std.chrono.seconds).
// The checker knows only where such a name comes from: its type is opaque
// (Phase 12 brings the C/C++ declarations).
func (q *query) cppHover() (*hoverResult, ast.Node) {
	u, names, n := q.cppUseAt()
	if u == nil {
		return nil, nil
	}
	h := &hoverResult{}
	user := strings.Join(append([]string{u.Ident.Name}, names...), ".")
	switch {
	case len(names) == 0 && u.Kind == load.HeaderUse:
		h.signature = fmt.Sprintf("import %s %q // C/C++ header", u.Ident.Name, u.Include.Path)
		h.text = fmt.Sprintf("The header package of `%s`: its C-level names are `%s.X`. Their types are opaque to gocpp: the C++ compiler checks them.", u.Include.Path, u.Ident.Name)
	case len(names) == 0:
		h.signature = "namespace " + strings.TrimPrefix(u.Namespace, "::") + " // C++"
		h.text = fmt.Sprintf("The C++ namespace `%s`.", u.Namespace)
	case u.Kind == load.NamespaceUse:
		h.signature = user + " // C++"
		h.text = fmt.Sprintf("The C++ name `%s::%s`, from the headers of this file. Its type is opaque to gocpp: the C++ compiler checks it.", u.Namespace, strings.Join(names, "::"))
	default:
		from := ""
		if u.Include != nil {
			from = " from `" + u.Include.Path + "`"
		}
		h.signature = user + " // C/C++"
		h.text = fmt.Sprintf("A C/C++ name%s. Its type is opaque to gocpp: the C++ compiler checks it.", from)
	}
	return h, n
}

// cppUseAt returns the use of a header package or a namespace (raylib,
// std) at the place of q, the names after it up to the place (none on the
// header or namespace name itself; [chrono seconds] on seconds of
// std.chrono.seconds), and the name at the place; or nil.
func (q *query) cppUseAt() (*load.CppUse, []string, *ast.Ident) {
	rw := q.file.Rewrite
	if rw == nil || len(rw.Uses) == 0 {
		return nil, nil, nil
	}
	bySel := make(map[*ast.SelectorExpr]*load.CppUse, len(rw.Uses))
	for _, u := range rw.Uses {
		if u.Kind == load.UnnamedHeaderUse {
			continue // an error already
		}
		if q.contains(u.Ident) {
			return u, nil, u.Ident
		}
		if !u.Bare {
			bySel[u.Sel] = u
		}
	}
	id, path := identAt(q.file, q.pos)
	if id == nil || len(path) == 0 {
		return nil, nil, nil
	}
	sel, ok := path[len(path)-1].(*ast.SelectorExpr)
	if !ok || sel.Sel != id {
		return nil, nil, nil
	}
	names := []string{id.Name}
	for e := sel; ; {
		if u := bySel[e]; u != nil {
			return u, names, id
		}
		x, ok := e.X.(*ast.SelectorExpr)
		if !ok {
			return nil, nil, nil
		}
		names = append([]string{x.Sel.Name}, names...)
		e = x
	}
}

// format returns the hover as Markdown or plain text, like gopls's
// formatHover with the full documentation: the declaration (fenced as Go
// in Markdown), then the doc and the methods, then the footer; in
// Markdown, a line (---) between these sections.
func (h *hoverResult) format(markdown bool) string {
	fenced := func(s, language string) string {
		if s != "" && markdown {
			s = "```" + language + "\n" + strings.Trim(s, "\n") + "\n```"
		}
		return s
	}
	language := h.signatureLanguage
	if language == "" {
		language = "go"
	}
	doc := h.doc
	if markdown {
		doc = docMarkdown(doc)
	}
	doc = strings.TrimSuffix(doc, "\n")
	text := h.text // with `code`, which reads as well in plain text
	sections := [][]string{
		{fenced(h.signature, language)},
		{text, doc, fenced(h.methods, "go")},
		{fenced(h.expansion, "go")},
		{h.footer},
	}
	var b strings.Builder
	newline := func() {
		if markdown {
			b.WriteString("\n\n")
		} else {
			b.WriteByte('\n')
		}
	}
	for _, section := range sections {
		start := b.Len()
		for _, part := range section {
			if part == "" {
				continue
			}
			if markdown && b.Len() == start && start > 0 {
				newline()
				b.WriteString("---")
			}
			if b.Len() > 0 {
				newline()
			}
			b.WriteString(part)
		}
	}
	return b.String()
}

// docMarkdown converts the text of a doc comment to Markdown (gopls's
// DocCommentToMarkdown, without links to pkg.go.dev: a doc link shows as
// its text).
func docMarkdown(text string) string {
	if text == "" {
		return ""
	}
	var p comment.Parser
	doc := p.Parse(text)
	var pr comment.Printer
	pr.HeadingID = func(*comment.Heading) string { return "" }
	pr.DocLinkURL = func(*comment.DocLink) string { return "" }
	return string(pr.Markdown(doc))
}
