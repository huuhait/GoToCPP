package check

import (
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/load"
)

// A Conversion is a conversion T(x) of an opaque value x (Phase 01 README
// §2.5 step 5): x comes from C/C++, so go/types has no type for it. Then
// go/types gives the conversion no type either: n := int(cstring.strlen(p))
// would make n invalid, and no mistake with n would be reported. So Check
// rewrites the call to gp_cgoh.As[T](x) and checks the package again: then
// the call has the type T. Codegen emits a C++ cast to T for it. gp_cgoh is
// the import of the hidden package with the reserved path gocpp/cgoh
// (load.CgohPath), which Check makes in Go code:
//
//	package cgoh
//
//	func As[T any](x any) T
//	func Unused[T any](x any) [1]T
//	func Value[T any](x any) T
//
// The rewrite changes the user's call node in place: Call.Fun, which was
// the type expression T, becomes the instantiation gp_cgoh.As[T], whose
// type argument is the user's T. So the call keeps its place in the AST
// and its positions, and Info.Types[Call] has the type T.
//
// Two kinds of conversions are errors in Go, but a call would not be, so
// they get another rewrite, which makes go/types report Go's error, at the
// place and in the order of go build (see Kind):
//   - A conversion that is a statement (int(C.f()) on a line of its own:
//     "int(C.f()) (value of type int) is not used") becomes
//     gp_cgoh.Unused[T](x)[0], an index expression, which replaces the
//     call in its statement (or in its parentheses). Info.Types[Call] is
//     [1]T.
//   - A conversion after go or defer ("defer requires function call, not
//     conversion") stays a conversion of a typed value:
//     T(gp_cgoh.Value[T](x)). The type argument is a copy of T with every
//     position at x, so the new call covers the text of x.
//
// Codegen never sees these two, since the package has an error.
//
// Tools that look up the node at a place (hover, go to definition) must
// skip the new nodes: Call.Fun.(*ast.IndexExpr).X (gp_cgoh.As, at the
// position of T), the import of gocpp/cgoh (at the import keyword), and
// the Value call of a Deferred conversion. They go from the call to the
// Conversion with Package.Conversion, and show Original. The phase gates
// skip them in the same way (Package.rewriteNodes).
type Conversion struct {
	// Call is the user's call T(x) after the rewrite. For an AsCall or an
	// UnusedStmt, Call.Fun is the *ast.IndexExpr gp_cgoh.As[T] (or
	// gp_cgoh.Unused[T]), and Call.Args[0] is x. Each new node of Call.Fun
	// has the position of T (the closing bracket the position of the last
	// character of T), so Call.Fun covers the text of T. For a Deferred
	// conversion, Call.Fun is T, and Call.Args[0] is gp_cgoh.Value[T](x).
	Call *ast.CallExpr

	// Type is T as the user wrote it: int in int(x), the *ast.ParenExpr
	// (*Point) in (*Point)(p), []byte in []byte(x).
	Type ast.Expr

	Kind ConversionKind

	// name is the name of the import of gocpp/cgoh in the file.
	name string
}

// A ConversionKind says how Check rewrote a Conversion.
type ConversionKind int

const (
	// AsCall: T(x) became gp_cgoh.As[T](x).
	AsCall ConversionKind = iota

	// UnusedStmt: a conversion that is a statement became
	// gp_cgoh.Unused[T](x)[0].
	UnusedStmt

	// Deferred: the call of a go or defer statement became
	// T(gp_cgoh.Value[T](x)).
	Deferred

	// Compare: a comparison of opaque values x == y (also !=, <, <=, >,
	// >=) became gp_cgoh.Cmp(x == y), a bool, as cgo's comparisons of C
	// values are (Phase 04 step 4.4): var ok bool = p == nil, b := x < 3.
	// Call.Fun is the selector gp_cgoh.Cmp at the place of the comparison,
	// Call.Args[0] the comparison, and Type is nil. Unlike Go's untyped
	// bool, the result has the type bool: a named bool type needs a
	// conversion, B(x == 3).
	Compare
)

// Pos returns the place of the conversion: the start of T.
func (c *Conversion) Pos() token.Pos {
	return c.Call.Pos()
}

// Arg returns x, the value that is converted.
func (c *Conversion) Arg() ast.Expr {
	if c.Kind == Deferred {
		return c.Call.Args[0].(*ast.CallExpr).Args[0]
	}
	return c.Call.Args[0]
}

// Original returns the call as the user wrote it, T(x): a new node, which
// is not in the AST, with the Type and the argument of Call. Messages and
// the editor can show it.
func (c *Conversion) Original() *ast.CallExpr {
	if c.Kind == Compare {
		return c.Call // (String shows the comparison)
	}
	return &ast.CallExpr{
		Fun:    c.Type,
		Lparen: c.Call.Lparen,
		Args:   []ast.Expr{c.Arg()},
		Rparen: c.Call.Rparen,
	}
}

// String returns the conversion as the user wrote it, T(x), as Go source
// (types.ExprString), also with conversions inside x.
func (c *Conversion) String() string {
	cv := &converter{names: map[string]bool{c.name: true}}
	if c.Kind == Compare {
		return cv.fixMessage(types.ExprString(c.Call.Args[0]))
	}
	return cv.fixMessage(types.ExprString(c.Original()))
}

// Conversion returns the conversion whose call is call, or nil if call is
// not one.
func (p *Package) Conversion(call *ast.CallExpr) *Conversion {
	return p.conversions[call]
}

// rewriteNodes returns the nodes that the rewrite of the conversions added
// (see Conversion), for passes that look at what the user wrote, such as
// the phase gates: hidden are the roots of the subtrees that the user
// didn't write (gp_cgoh.As, and the Fun of a Value call, with its copy of
// T), and skipTypes are those nodes and all the nodes inside them, and
// the calls whose type is not the user's (Unused, [1]T, and Value). The
// calls of As and Unused stay conversions T(x) for the passes.
func (p *Package) rewriteNodes() (hidden, skipTypes map[ast.Node]bool) {
	hidden, skipTypes = make(map[ast.Node]bool), make(map[ast.Node]bool)
	hide := func(n ast.Node) {
		hidden[n] = true
		ast.Inspect(n, func(n ast.Node) bool {
			if n != nil {
				skipTypes[n] = true
			}
			return true
		})
	}
	for _, c := range p.Conversions {
		switch c.Kind {
		case AsCall, UnusedStmt:
			hide(c.Call.Fun.(*ast.IndexExpr).X)
			if c.Kind == UnusedStmt {
				skipTypes[c.Call] = true
			}
		case Deferred:
			inner := c.Call.Args[0].(*ast.CallExpr)
			hide(inner.Fun)
			skipTypes[inner] = true
		case Compare:
			hide(c.Call.Fun) // the call stands for the comparison, a bool
		}
	}
	return hidden, skipTypes
}

// exprString returns e as Go source, as the user wrote it: like
// types.ExprString, but with the rewritten conversions inside e shown as
// T(x) (see converter.fixMessage), as in the messages of go/types after
// Check. Code that builds the text of a go/types message from the AST,
// to match it, needs it.
func (p *Package) exprString(e ast.Expr) string {
	s := types.ExprString(e)
	if len(p.Conversions) == 0 {
		return s
	}
	cv := &converter{names: make(map[string]bool)}
	for _, c := range p.Conversions {
		cv.names[c.name] = true
	}
	return cv.fixMessage(s)
}

// setConversions sets Conversions and the map of Conversion.
func (p *Package) setConversions(cs []*Conversion) {
	p.Conversions = cs
	p.conversions = make(map[*ast.CallExpr]*Conversion, len(cs))
	for _, c := range cs {
		p.conversions[c.Call] = c
	}
}

// usesC reports whether a file of lp has import "C".
func usesC(lp *load.Package) bool {
	return slices.ContainsFunc(lp.Files, func(f *load.File) bool { return importsC(f.AST) })
}

// followOnError reports whether msg is the message of an error about an
// operand that is invalid already, such as "cannot convert a.X (variable
// with invalid type) to type int" or "int(C.f()) (invalid operand) is not
// used". It is the same rule as go/types' own (report in go/types'
// errors.go, lines 117 to 121 in Go 1.25), which drops such an error unless
// it is the first error of the package: so the reason for the invalid
// operand is silent, C, which is what the rewrite is for. There is no error
// code for it, so the words decide, and a message that has the words in a
// string of the user ("invalid type" in a constant) matches too, in
// go/types and here.
func followOnError(msg string) bool {
	return strings.Index(msg, "invalid operand") > 0 || strings.Index(msg, "invalid type") > 0
}

// cgohName is the name of the import of gocpp/cgoh that the rewrite adds
// to a file. If the package uses the name already, for anything, the
// import is named gp_cgoh_1 (or gp_cgoh_2, and so on), so that it never
// clashes with a user's name: gocpp allows names that start with gp_
// (codegen escapes them).
const cgohName = "gp_cgoh"

// The names of the functions of gocpp/cgoh.
const (
	asName     = "As"
	unusedName = "Unused"
	valueName  = "Value"
	cmpName    = "Cmp"
)

// unusedExpr is the go/types error code of an expression that is not used
// as a statement: "x (value of type int) is not used" (UnusedExpr in Go's
// internal/types/errors).
const unusedExpr = 100

// newCgohPackage makes the hidden package gocpp/cgoh (see Conversion).
// Check makes it once, for all its packages.
func newCgohPackage() *types.Package {
	pkg := types.NewPackage(load.CgohPath, "cgoh")
	anyType := types.Universe.Lookup("any").Type()
	fn := func(name string, result func(T types.Type) types.Type) {
		tparam := types.NewTypeParam(types.NewTypeName(token.NoPos, pkg, "T", nil), anyType)
		params := types.NewTuple(types.NewParam(token.NoPos, pkg, "x", anyType))
		results := types.NewTuple(types.NewParam(token.NoPos, pkg, "", result(tparam)))
		sig := types.NewSignatureType(nil, nil, []*types.TypeParam{tparam}, params, results, false)
		pkg.Scope().Insert(types.NewFunc(token.NoPos, pkg, name, sig))
	}
	fn(asName, func(T types.Type) types.Type { return T })
	fn(unusedName, func(T types.Type) types.Type { return types.NewArray(T, 1) })
	fn(valueName, func(T types.Type) types.Type { return T })
	// func Cmp(x any) bool
	cmp := types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewParam(token.NoPos, pkg, "x", anyType)),
		types.NewTuple(types.NewParam(token.NoPos, pkg, "", types.Typ[types.Bool])), false)
	pkg.Scope().Insert(types.NewFunc(token.NoPos, pkg, cmpName, cmp))
	addHelpers(pkg) // the cgo helpers (cgohelpers.go)
	pkg.MarkComplete()
	return pkg
}

// cgohCall returns the name of the function of gocpp/cgoh that call calls
// (As, Unused or Value) and its type argument, or "" and nil.
func cgohCall(call *ast.CallExpr, info *types.Info) (string, ast.Expr) {
	ix, ok := call.Fun.(*ast.IndexExpr)
	if !ok || len(call.Args) != 1 {
		return "", nil
	}
	sel, ok := ix.X.(*ast.SelectorExpr)
	if !ok {
		return "", nil
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", nil
	}
	if pn, ok := info.Uses[id].(*types.PkgName); ok && pn.Imported().Path() == load.CgohPath {
		return sel.Sel.Name, ix.Index
	}
	return "", nil
}

// cmpCall reports whether call is gp_cgoh.Cmp(x) (a rewritten comparison).
func cmpCall(call *ast.CallExpr, info *types.Info) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != cmpName || len(call.Args) != 1 {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pn, ok := info.Uses[id].(*types.PkgName)
	return ok && pn.Imported().Path() == load.CgohPath
}

// A converter rewrites the conversions of opaque values in one package.
//
// It rewrites the comparisons of opaque values too, x == y to
// gp_cgoh.Cmp(x == y) (see Compare), with the same analysis: a
// comparison whose operands are opaque (a.state), not where Go needs a
// constant. A comparison waits for the conversions of its round, and a
// conversion of a comparison (bool(p == nil)) for the comparison's
// rewrite, after which it is a Go conversion.
//
// After a check of the package, rewrite finds each call T(x) that is a
// conversion (Info.Types[Fun] is a type) with one argument x that has no
// type (or a type with an invalid part), and where the call has no type.
// It rewrites the call (see Conversion) only if x is opaque: x is invalid
// because it comes from C/C++, not because of an error. go/types marks the
// results of errors invalid too, and reports no more errors with them;
// with FakeImportC, C is the only silent source of invalid values. So for
// go build's errors (in a package that a Go twin of the program has), the
// rewrite must not make such a value typed: in x := undefinedName;
// n := int(x); n + "a", go build reports only undefinedName. The analysis
// (see analysis.state) follows x back to its sources: C.f() (a selector
// rooted in C) is opaque, a name goes to its declaration, and a source
// that it doesn't know, or an error in a declaration that it depends on,
// wins. An error at x itself ("cannot convert p (variable of struct type
// P2) to type int") is the conversion's own: then nothing is rewritten.
//
// A conversion whose x depends only on other conversions found in the
// same round (int(int(C.f())), or m := float64(C.f()); n := int(m)) waits:
// the next check gives the inner conversion its type, and then x is no
// longer invalid, so the outer one is a normal Go conversion (string(x) of
// an int is a rune conversion, not a C++ cast). One C source of its own
// makes x opaque at once: in y := float64(C.a()) + C.b(), y stays invalid
// whatever float64(C.a()) becomes, so int(y) is rewritten in the same
// round. And a conversion inside the arguments of a C call doesn't count:
// the result of a call of an invalid function is invalid whatever its
// arguments are (int(C.g(float64(C.h()))) is rewritten in one round).
//
// The checks repeat until a round rewrites nothing. One extra check is
// enough for most packages. A conversion that waits for one to a type
// with a C part needs a third: in a := []C.int(C.f()); i := int(a[0]),
// a[0] is still opaque after the second check (a is a []C.int), so int(a[0])
// is rewritten then. It doesn't skip the wait: int(P2(C.f())), with a C
// field in the struct P2, is a Go error once P2(C.f()) has its type.
//
// Not rewritten:
//   - conversions where Go needs a constant (in a const declaration, an
//     array length, or a key of an array or slice literal): As is a
//     function, so the result would not be a constant, and go/types would
//     report an error where it reports none today;
//   - all conversions of a package that imports a package with errors
//     (checkPackage): an invalid value from there can come from an error.
//
// Where the analysis can't tell, the conversion is not rewritten, so the
// package gets fewer errors than go build gives its Go twin, never more.
// Known cases: a call of a generic function whose type argument go/types
// can't infer from an opaque argument (int(G(C.f())) with
// func G[T any](x T) T), and a value whose declaration the analysis
// doesn't follow.
type converter struct {
	fset *token.FileSet
	lp   *load.Package

	// cgoh is the hidden package of Check.
	cgoh *types.Package

	// names are the names of the imports of gocpp/cgoh in the files of lp
	// (gp_cgoh): fixMessage turns name.As[T](x) back into T(x).
	names map[string]bool

	// name is the name for a new import of gocpp/cgoh, or "" until the
	// first rewrite chooses it.
	name string

	// decls maps the place of each declared name to what gives it its
	// type (see declNodes), or is nil until the first round that has a
	// conversion to look at.
	decls map[token.Pos][]declNode

	// consts are the C/C++ constants that were put into the tree before the check (constsubst.go), or nil.
	consts *constSubst
}

// newConverter returns the converter of the package lp, with the hidden
// package cgoh.
func newConverter(fset *token.FileSet, lp *load.Package, cgoh *types.Package) *converter {
	cv := &converter{fset: fset, lp: lp, cgoh: cgoh, names: make(map[string]bool)}
	for _, f := range lp.Files {
		// An earlier Check of the same ASTs rewrote some calls already.
		if spec := cgohImport(f.AST); spec != nil {
			cv.names[spec.Name.Name] = true
		}
	}
	return cv
}

// cgohImport returns the import of gocpp/cgoh in f, or nil.
func cgohImport(f *ast.File) *ast.ImportSpec {
	for _, spec := range f.Imports {
		if path, err := strconv.Unquote(spec.Path.Value); err == nil && path == load.CgohPath && spec.Name != nil {
			return spec
		}
	}
	return nil
}

// A candidate is a conversion whose argument has no type.
type candidate struct {
	call *ast.CallExpr
	file *ast.File

	// stmt, for a conversion that is a statement, is the field that holds
	// call: the X of the *ast.ExprStmt, or of the parentheses around call.
	stmt *ast.Expr

	// deferred is set for the call of a go or defer statement.
	deferred bool
}

// rewrite rewrites the conversions of opaque values in the files, after
// the check that gave p and found the errors at the places errs (see
// collector.blocking). It reports whether it rewrote any, so that the
// caller checks the package again.
func (cv *converter) rewrite(p *Package, errs []token.Pos) bool {
	// The files need no version check: go/types checks a file with an
	// older //go:build version (go1.17) as go1.21, which has generics.
	var cands []candidate
	var cmps []cmpCandidate
	var uns []unsafeCandidate
	var sls []sliceCandidate
	for _, f := range cv.lp.Files {
		cands = append(cands, opaqueConversions(f.AST, p.Info)...)
		cmps = append(cmps, opaqueComparisons(f.AST, p.Info)...)
		uns = append(uns, opaqueUnsafeCalls(f.AST, p.Info)...)
		sls = append(sls, opaqueSlices(f.AST, p.Info)...)
	}
	if len(cands) == 0 && len(cmps) == 0 && len(uns) == 0 && len(sls) == 0 {
		return false
	}
	if cv.decls == nil {
		files := make([]*ast.File, len(cv.lp.Files))
		for i, f := range cv.lp.Files {
			files[i] = f.AST
		}
		cv.decls = declNodes(files)
	}
	a := &analysis{
		info:  p.Info,
		pkg:   p.Types,
		errs:  slices.Sorted(slices.Values(errs)),
		decls: cv.decls,
		cands: make(map[*ast.CallExpr]bool, len(cands)),
		cmps:  make(map[*ast.BinaryExpr]bool, len(cmps)),
		memo:  make(map[types.Object]state),
		types: make(map[*types.TypeName]state),
	}
	for _, c := range cands {
		a.cands[c.call] = true
	}
	a.sls = make(map[*ast.SliceExpr]bool, len(sls))
	for _, c := range sls {
		a.sls[c.slice] = true
	}
	a.uns = make(map[*ast.CallExpr]bool, len(uns))
	for _, u := range uns {
		a.uns[u.call] = true
	}
	for _, c := range cmps {
		a.cmps[c.cmp] = true
	}
	n := 0
	for _, c := range cands {
		// The candidates of this round stay in a.cands also after their
		// rewrite: a conversion that depends on one waits for the next
		// round.
		arg := c.call.Args[0]
		if a.errorAt(c.call.Pos()) || a.errorAt(arg.Pos()) || a.state(arg) != opaque {
			continue
		}
		cv.rewriteCall(c)
		n++
	}
	for _, c := range cmps {
		if a.errorIn(c.cmp) || max(a.state(c.cmp.X), a.state(c.cmp.Y)) != opaque {
			continue
		}
		cv.rewriteCmp(c)
		n++
	}
	for _, u := range uns {
		if cv.rewriteUnsafe(a, u) {
			n++
		}
	}
	for _, c := range sls {
		if cv.rewriteSlice(a, c) {
			n++
		}
	}
	return n > 0
}

// A cmpCandidate is a comparison whose operands have no type, and the
// field that holds it.
type cmpCandidate struct {
	cmp  *ast.BinaryExpr
	file *ast.File
	slot *ast.Expr
}

// opaqueComparisons returns the comparisons in f, in source order, that
// have no type, except those where Go needs a constant (as
// opaqueConversions).
func opaqueComparisons(f *ast.File, info *types.Info) []cmpCandidate {
	var consts []ast.Node
	var cands []cmpCandidate
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.GenDecl:
			if n.Tok == token.CONST {
				consts = append(consts, n)
			}
		case *ast.ArrayType:
			if n.Len != nil {
				consts = append(consts, n.Len)
			}
		}
		if call, ok := n.(*ast.CallExpr); ok && cmpCall(call, info) {
			return true // rewritten already
		}
		for _, slot := range exprSlots(n) {
			b, ok := (*slot).(*ast.BinaryExpr)
			if !ok {
				continue
			}
			switch b.Op {
			case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
			default:
				continue
			}
			if tv, ok := info.Types[b]; ok && isValidType(tv.Type) {
				continue
			}
			if slices.ContainsFunc(consts, func(c ast.Node) bool { return c.Pos() <= b.Pos() && b.End() <= c.End() }) {
				continue
			}
			cands = append(cands, cmpCandidate{cmp: b, file: f, slot: slot})
		}
		return true
	})
	return cands
}

// exprSlots returns the fields (and slice elements) of the node n that
// hold expressions, so that one can be replaced.
func exprSlots(n ast.Node) []*ast.Expr {
	if n == nil {
		return nil
	}
	v := reflect.ValueOf(n)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return nil
	}
	exprType := reflect.TypeFor[ast.Expr]()
	var slots []*ast.Expr
	s := v.Elem()
	for i := range s.NumField() {
		f := s.Field(i)
		switch {
		case f.Type() == exprType && !f.IsNil():
			slots = append(slots, f.Addr().Interface().(*ast.Expr))
		case f.Kind() == reflect.Slice && f.Type().Elem() == exprType:
			for j := range f.Len() {
				slots = append(slots, f.Index(j).Addr().Interface().(*ast.Expr))
			}
		}
	}
	return slots
}

// rewriteCmp rewrites the comparison of c to gp_cgoh.Cmp(x == y) (see
// Compare), at the place of the comparison, and adds the import of
// gocpp/cgoh to the file if it has none.
func (cv *converter) rewriteCmp(c cmpCandidate) {
	name := cv.importName(c.file)
	b := c.cmp
	*c.slot = &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{NamePos: b.Pos(), Name: name},
			Sel: &ast.Ident{NamePos: b.Pos(), Name: cmpName},
		},
		Lparen: b.Pos(),
		Args:   []ast.Expr{b},
		Rparen: b.End() - 1,
	}
}

// opaqueConversions returns the conversions in f, in source order, whose
// argument has no type (or a type with an invalid part) and that have no
// type themselves, except those where Go needs a constant.
func opaqueConversions(f *ast.File, info *types.Info) []candidate {
	var consts []ast.Node // where Go needs a constant
	var cands []candidate
	stmts := make(map[*ast.CallExpr]*ast.Expr)
	deferred := make(map[*ast.CallExpr]bool) // the calls of go and defer
	inConst := func(n ast.Node) bool {
		return slices.ContainsFunc(consts, func(c ast.Node) bool { return c.Pos() <= n.Pos() && n.End() <= c.End() })
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.GenDecl:
			if n.Tok == token.CONST {
				consts = append(consts, n)
			}
		case *ast.ArrayType:
			if n.Len != nil {
				consts = append(consts, n.Len)
			}
		case *ast.CompositeLit:
			if tv, ok := info.Types[n]; ok && tv.Type != nil {
				switch tv.Type.Underlying().(type) {
				case *types.Array, *types.Slice:
					for _, e := range n.Elts {
						if kv, ok := e.(*ast.KeyValueExpr); ok {
							consts = append(consts, kv.Key)
						}
					}
				}
			}
		case *ast.GoStmt:
			deferred[n.Call] = true
		case *ast.DeferStmt:
			deferred[n.Call] = true
		case *ast.ExprStmt:
			holder := &n.X
			for {
				paren, ok := (*holder).(*ast.ParenExpr)
				if !ok {
					break
				}
				holder = &paren.X
			}
			if call, ok := (*holder).(*ast.CallExpr); ok {
				stmts[call] = holder
			}
		case *ast.CallExpr:
			if isOpaqueConversion(n, info) && !inConst(n) {
				cands = append(cands, candidate{call: n, file: f, stmt: stmts[n], deferred: deferred[n]})
			}
		}
		return true
	})
	return cands
}

// isOpaqueConversion reports whether call is a conversion T(x) whose x has
// no type, or a type with an invalid part, and that has no type itself.
func isOpaqueConversion(call *ast.CallExpr, info *types.Info) bool {
	if len(call.Args) != 1 || call.Ellipsis.IsValid() {
		return false
	}
	if fun, ok := info.Types[call.Fun]; !ok || !fun.IsType() || !isValidType(fun.Type) {
		return false
	}
	if tv, ok := info.Types[call]; ok && isValidType(tv.Type) {
		return false
	}
	tv, ok := info.Types[call.Args[0]]
	return !ok || !isValidType(tv.Type) || hasInvalid(tv.Type)
}

// rewriteCall rewrites the conversion T(x) of c to gp_cgoh.As[T](x), a
// statement to gp_cgoh.Unused[T](x)[0], and the call of go or defer to
// T(gp_cgoh.Value[T](x)) (see Conversion), and adds the import of
// gocpp/cgoh to the file if it has none.
func (cv *converter) rewriteCall(c candidate) {
	name := cv.importName(c.file)
	call, typ := c.call, c.call.Fun
	inst := func(fun string, typ ast.Expr, pos, end token.Pos) *ast.IndexExpr {
		return &ast.IndexExpr{
			X: &ast.SelectorExpr{
				X:   &ast.Ident{NamePos: pos, Name: name},
				Sel: &ast.Ident{NamePos: pos, Name: fun},
			},
			Lbrack: pos,
			Index:  typ,
			Rbrack: end,
		}
	}
	if c.deferred {
		x := call.Args[0]
		pos := x.Pos()
		call.Args[0] = &ast.CallExpr{
			Fun:    inst(valueName, cloneAt(typ, pos), pos, pos),
			Lparen: pos,
			Args:   []ast.Expr{x},
			Rparen: x.End() - 1,
		}
		return
	}
	fun := asName
	if c.stmt != nil {
		fun = unusedName
	}
	call.Fun = inst(fun, typ, typ.Pos(), typ.End()-1)
	if c.stmt != nil {
		// [0] at the closing parenthesis, so that the index expression
		// ends where the call does.
		*c.stmt = &ast.IndexExpr{
			X:      call,
			Lbrack: call.Rparen,
			Index:  &ast.BasicLit{ValuePos: call.Rparen, Kind: token.INT, Value: "0"},
			Rbrack: call.Rparen,
		}
	}
}

// cloneAt returns a deep copy of the expression e in which every valid
// position is pos. The objects of the parser's resolution (ast.Object)
// and the comments are shared.
func cloneAt(e ast.Expr, pos token.Pos) ast.Expr {
	return cloneValue(reflect.ValueOf(e), pos).Interface().(ast.Expr)
}

var objField, _ = reflect.TypeFor[ast.Ident]().FieldByName("Obj")

var (
	posType     = reflect.TypeFor[token.Pos]()
	objectType  = objField.Type // *ast.Object, deprecated but kept by the parser fork
	commentType = reflect.TypeFor[*ast.CommentGroup]()
)

func cloneValue(v reflect.Value, pos token.Pos) reflect.Value {
	switch {
	case v.Type() == posType:
		if token.Pos(v.Int()).IsValid() {
			return reflect.ValueOf(pos)
		}
		return v
	case v.Type() == objectType || v.Type() == commentType:
		return v
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		c := reflect.New(v.Type().Elem())
		c.Elem().Set(cloneValue(v.Elem(), pos))
		return c
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		c := reflect.New(v.Type()).Elem()
		c.Set(cloneValue(v.Elem(), pos))
		return c
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		c := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			c.Index(i).Set(cloneValue(v.Index(i), pos))
		}
		return c
	case reflect.Struct:
		c := reflect.New(v.Type()).Elem()
		for i := range v.NumField() {
			c.Field(i).Set(cloneValue(v.Field(i), pos))
		}
		return c
	}
	return v
}

// importName returns the name of the import of gocpp/cgoh in f. If f has
// none, it adds one, like load.RewriteFile adds import "C": at the import
// keyword of the first import declaration (f has import "C", or another
// file of the package has), or else in a new declaration at the package
// keyword. So the declarations stay in source order.
func (cv *converter) importName(f *ast.File) string {
	if spec := cgohImport(f); spec != nil {
		return spec.Name.Name
	}
	name := cv.packageName()
	newSpec := func(pos token.Pos) *ast.ImportSpec {
		return &ast.ImportSpec{
			Name: &ast.Ident{NamePos: pos, Name: name},
			Path: &ast.BasicLit{ValuePos: pos, Kind: token.STRING, Value: strconv.Quote(load.CgohPath)},
		}
	}
	var spec *ast.ImportSpec
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			spec = newSpec(gd.TokPos)
			gd.Specs = append(gd.Specs, spec)
			break
		}
	}
	if spec == nil {
		spec = newSpec(f.Package)
		gd := &ast.GenDecl{TokPos: f.Package, Tok: token.IMPORT, Specs: []ast.Spec{spec}}
		f.Decls = slices.Insert(f.Decls, 0, ast.Decl(gd))
	}
	f.Imports = append(f.Imports, spec)
	cv.names[name] = true
	return name
}

// packageName returns the name for a new import of gocpp/cgoh: the name of
// the imports that an earlier Check added, or gp_cgoh, or gp_cgoh_N if a
// file of the package has the name gp_cgoh already.
func (cv *converter) packageName() string {
	if cv.name != "" {
		return cv.name
	}
	if len(cv.names) > 0 {
		cv.name = slices.Sorted(maps.Keys(cv.names))[0]
		return cv.name
	}
	used := make(map[string]bool)
	for _, f := range cv.lp.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && strings.HasPrefix(id.Name, cgohName) {
				used[id.Name] = true
			}
			return true
		})
	}
	cv.name = cgohName
	for i := 1; used[cv.name]; i++ {
		cv.name = cgohName + "_" + strconv.Itoa(i)
	}
	return cv.name
}

// fixMessage returns msg with each gp_cgoh.As[T](x) and
// gp_cgoh.Unused[T](x)[0] turned back into T(x), as the user wrote it,
// gp_cgoh.Value[T](x) into x, and gp_cgoh.As[T] without a call
// ("assignment mismatch: 2 variables but gp_cgoh.As[int] returns 1
// value") into T.
func (cv *converter) fixMessage(msg string) string {
	for name := range cv.names {
		msg = unwrapHelpers(fixHelpers(msg, name), name)
		msg = unwrapCmp(msg, name+"."+cmpName+"(")
		msg = unwrap(msg, name+"."+valueName+"[", valueName)
		msg = unwrap(msg, name+"."+unusedName+"[", unusedName)
		msg = unwrap(msg, name+"."+asName+"[", asName)
	}
	if strings.Contains(msg, "cannot take address of ") {
		// The slice of a C array (&c.vals[0:1]) has go/types' invalid element type.
		msg = strings.Replace(msg, " (value of type []invalid type)", " (slice of C/C++ values)", 1)
	}
	return msg
}

// unwrap turns each prefix + T + "](" + x + ")" in msg into T + "(" + x +
// ")", for the function fun of gocpp/cgoh: for Unused, the text must end
// in "[0]", which goes too; for Value, it becomes x; for As, prefix + T +
// "]" without a call becomes T. The text before prefix must not end in a
// name, so that xgp_cgoh.As[ stays.
func unwrap(msg, prefix, fun string) string {
	from := 0
	for {
		i := strings.Index(msg[from:], prefix)
		if i < 0 {
			return msg
		}
		i += from
		from = i + 1
		if i > 0 && isNameByte(msg[i-1]) {
			continue
		}
		// T ends at the "]" that closes the "[" of prefix, and x at the
		// ")" that closes the "(" after it.
		j := i + len(prefix)
		k := closing(msg, j)
		if k < 0 {
			continue
		}
		typ := msg[j : k-1]
		if k >= len(msg) || msg[k] != '(' {
			if fun == asName {
				msg = msg[:i] + typ + msg[k:]
				from = i
			}
			continue
		}
		end := closing(msg, k+1)
		if end < 0 {
			continue
		}
		rest := msg[end:]
		switch fun {
		case unusedName:
			var ok bool
			if rest, ok = strings.CutPrefix(rest, "[0]"); !ok {
				continue
			}
		case valueName:
			msg = msg[:i] + msg[k+1:end-1] + rest
			from = i
			continue
		}
		msg = msg[:i] + typ + msg[k:end] + rest
		from = i
	}
}

// unwrapCmp turns each prefix + x + ")" in msg, a rewritten comparison
// gp_cgoh.Cmp(x), into x.
func unwrapCmp(msg, prefix string) string {
	from := 0
	for {
		i := strings.Index(msg[from:], prefix)
		if i < 0 {
			return msg
		}
		i += from
		from = i + 1
		if i > 0 && isNameByte(msg[i-1]) {
			continue
		}
		end := closing(msg, i+len(prefix))
		if end < 0 {
			continue
		}
		msg = msg[:i] + msg[i+len(prefix):end-1] + msg[end:]
		from = i
	}
}

// closing returns the index after the bracket that closes the one before
// msg[i:], or -1. Brackets in string and rune literals (struct tags) don't
// count.
func closing(msg string, i int) int {
	for depth := 1; i < len(msg); i++ {
		switch c := msg[i]; c {
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			if depth--; depth == 0 {
				return i + 1
			}
		case '"', '\'', '`':
			// To the closing quote; a backslash escapes in "" and ''.
			for i++; i < len(msg) && msg[i] != c; i++ {
				if msg[i] == '\\' && c != '`' {
					i++
				}
			}
		}
	}
	return -1
}

// isNameByte reports whether b can be part of a Go name (for ASCII).
func isNameByte(b byte) bool {
	return b == '_' || '0' <= b && b <= '9' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' || b >= 0x80
}

// findConversions returns the rewritten conversions in the files of lp,
// in source order: the calls of As and Unused of gocpp/cgoh, and the
// conversions whose argument is a call of Value. It finds them again after
// each check (and in the ASTs that an earlier Check rewrote), since only
// the rewrite imports gocpp/cgoh.
func findConversions(lp *load.Package, info *types.Info) []*Conversion {
	var cs []*Conversion
	for _, f := range lp.Files {
		spec := cgohImport(f.AST)
		if spec == nil {
			continue
		}
		name := spec.Name.Name
		ast.Inspect(f.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if cmpCall(call, info) {
				cs = append(cs, &Conversion{Call: call, Kind: Compare, name: name})
				return true
			}
			switch fun, typ := cgohCall(call, info); fun {
			case asName:
				cs = append(cs, &Conversion{Call: call, Type: typ, Kind: AsCall, name: name})
			case unusedName:
				cs = append(cs, &Conversion{Call: call, Type: typ, Kind: UnusedStmt, name: name})
			case "":
				if len(call.Args) == 1 {
					if inner, ok := call.Args[0].(*ast.CallExpr); ok {
						if fun, _ := cgohCall(inner, info); fun == valueName {
							cs = append(cs, &Conversion{Call: call, Type: call.Fun, Kind: Deferred, name: name})
						}
					}
				}
			}
			return true
		})
	}
	return cs
}
