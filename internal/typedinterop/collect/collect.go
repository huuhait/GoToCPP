// Package collect is step 12.1 of typed interop (Phase 12): the reference
// collector.
//
// After go/types and the opaque pass (internal/check), the collector lists
// every C/C++ expression of a package as a reference shape: what the program
// asks of C/C++ and how. The probe package (step 12.2) turns the shapes into
// a probe file for clang and reads the types back.
//
// What becomes a reference. Every expression that the opaque pass marks
// opaque (check.Opaque.IsOpaque) gets one reference, whose Expr has the
// whole expression inside it, so one probe answers it:
//
//	cstring.strlen(s) + len(s)   binary(+ | call(::strlen|arg(gp string)) | ...)
//	v := std.vector[int]{1, 2}   a literal, its type std::vector<int64_t>, std::vector as a path
//	n := v.size()                a member call on v, whose type is the type of `v := ...`
//
// The parts of an expression are references of their own too (the call
// strlen(s) in the sum), because the checker needs the type of each
// subexpression. A name of a header package or a namespace is a KindPath
// reference; it is only a name (Ref.Standalone is false) when every use is the
// function of a call or the template of an instance, which have their own
// references.
//
// Operands that are not C/C++ are written the way the generated code gives
// them to C++:
//   - an untyped constant is a C literal with C's default type (5 is an int,
//     2.0 a double, "x" a string literal), as in Phase 04 (§2.6, §2.9);
//   - another Go value is std::declval of its C++ type: int64_t for int, double
//     for float64, a stand-in of the string type for string. A variable, a
//     field, an index or a dereference is an lvalue (std::declval<T&>()), as in
//     the generated code; a call or an operator result is a temporary;
//   - a local variable that holds a C/C++ value (x := f(), a, b := g(),
//     for k, v := range c) is an lvalue of the type that auto gives it: the
//     Decay of its value. The reference of such a use has the declaration
//     inlined (y := x.g() becomes one decltype over f()), up to a size limit,
//     and the declaration's own reference (KindLocal) is used beyond that;
//   - a Go type that C++ can't name without gocpp's runtime (a struct, a
//     function value, an interface) makes the reference Incomplete: it is
//     listed, not probed.
//
// The set is canonical: references with the same C++ meaning are merged (the
// Key is the canonical text), refs are sorted by key and sites by position, so
// the set doesn't depend on the order of declarations, comments, variable names
// or spacing.
package collect

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
)

// inlineLimit is the length of the key of a variable's declaration up to
// which a use inlines it into the expression (see the package comment).
const inlineLimit = 300

// maxKey is the length of a key beyond which a reference is Incomplete
// ("expression too large"), so that a pathological program can't make a
// probe file of megabytes.
const maxKey = 4000

// Collect returns the references of the checked package p. fset is the file
// set of the program (load.Program.Fset). A package that was not checked,
// or has no C/C++ uses, gives an empty set.
func Collect(p *check.Package, fset *token.FileSet) *Set {
	s := &Set{byNode: make(map[any]*Ref), byKey: make(map[string]*Ref)}
	if p == nil || !p.Checked() || p.Opaque == nil || p.Info == nil {
		return s
	}
	s.Dir = p.Load.Dir
	c := &collector{
		p: p, fset: fset, info: p.Info, op: p.Opaque, set: s,
		uses:    make(map[*ast.SelectorExpr]*load.CppUse),
		decls:   make(map[types.Object]*varDecl),
		defRefs: make(map[types.Object]*Ref),
		skip:    make(map[ast.Node]bool),
		noRef:   make(map[ast.Node]bool),
	}
	for _, cv := range p.Conversions {
		// The nodes that check added (gp_cgoh.As[T], gp_cgoh.Cmp) make no
		// reference: the conversion's call does (conv), and T is walked.
		if cv.Kind == check.Deferred {
			continue
		}
		c.noRef[cv.Call.Fun] = true
		if ix, ok := cv.Call.Fun.(*ast.IndexExpr); ok {
			c.noRef[ix.X] = true
		}
	}
	for _, f := range p.Load.Files {
		if f.Rewrite == nil {
			continue
		}
		for _, u := range f.Rewrite.Uses {
			if !u.Bare && u.Sel != nil {
				c.uses[u.Sel] = u
			}
		}
	}
	for _, f := range p.Load.Files {
		c.collectDecls(f.AST)
	}
	for _, f := range p.Load.Files {
		c.walk(f.AST)
	}
	s.finish()
	return s
}

// A varDecl says how a local variable was declared without a type.
type varDecl struct {
	init     ast.Expr // the value: x := init, a, b := init, range init
	idx, n   int      // the variable is the idx-th of n (n > 1 and one value: a tuple)
	rangeVar int      // 1: the key (index) of range init; 2: its value; 0: not a range
}

type collector struct {
	p    *check.Package
	fset *token.FileSet
	info *types.Info
	op   *check.Opaque
	set  *Set

	uses    map[*ast.SelectorExpr]*load.CppUse
	decls   map[types.Object]*varDecl
	defRefs map[types.Object]*Ref
	inDef   map[types.Object]bool
	skip    map[ast.Node]bool // nodes that make no reference and are not entered (keys of literals, ...)
	noRef   map[ast.Node]bool // nodes that make no reference (check's gp_cgoh.As[T]) but whose parts do

	// The context of the walk.
	stack []ast.Node
	sigs  []*types.Signature // the enclosing functions
}

// collectDecls records the variables that are declared without a type from
// a value (x := v, var x = v, for k, v := range c).
func (c *collector) collectDecls(f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok != token.DEFINE {
				return true
			}
			c.declare(n.Lhs, n.Rhs)
		case *ast.ValueSpec:
			if n.Type == nil {
				var lhs []ast.Expr
				for _, id := range n.Names {
					lhs = append(lhs, id)
				}
				c.declare(lhs, n.Values)
			}
		case *ast.RangeStmt:
			if n.Tok != token.DEFINE {
				return true
			}
			for i, e := range []ast.Expr{n.Key, n.Value} {
				if id, ok := e.(*ast.Ident); ok && id.Name != "_" {
					if obj := c.info.Defs[id]; obj != nil {
						c.decls[obj] = &varDecl{init: n.X, rangeVar: i + 1}
					}
				}
			}
		}
		return true
	})
}

func (c *collector) declare(lhs, rhs []ast.Expr) {
	for i, e := range lhs {
		id, ok := e.(*ast.Ident)
		if !ok || id.Name == "_" {
			continue
		}
		obj := c.info.Defs[id]
		if obj == nil {
			continue
		}
		switch {
		case len(rhs) == len(lhs):
			c.decls[obj] = &varDecl{init: rhs[i], idx: 0, n: 1}
		case len(rhs) == 1:
			c.decls[obj] = &varDecl{init: rhs[0], idx: i, n: len(lhs)}
		}
	}
}

// walk visits the expressions of a file in source order and makes the
// references.
func (c *collector) walk(f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			c.pop()
			return true
		}
		switch n := n.(type) {
		case *ast.ImportSpec:
			return false
		case *ast.FuncDecl:
			c.stack = append(c.stack, n)
			c.sigs = append(c.sigs, c.sigOf(n.Name))
			return true
		case *ast.FuncLit:
			c.stack = append(c.stack, n)
			c.sigs = append(c.sigs, c.sigOfExpr(n))
			return true
		case *ast.CompositeLit:
			for _, el := range n.Elts {
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					if id, ok := kv.Key.(*ast.Ident); ok && c.isFieldKey(id) {
						c.skip[id] = true
					}
				}
			}
		}
		c.stack = append(c.stack, n)
		e, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		if c.skip[n] || !c.visit(e) {
			c.pop() // ast.Inspect doesn't call us again for a node it doesn't enter
			return false
		}
		return true
	})
}

// pop ends the node on top of the stack.
func (c *collector) pop() {
	n := c.stack[len(c.stack)-1]
	c.stack = c.stack[:len(c.stack)-1]
	switch n.(type) {
	case *ast.FuncDecl, *ast.FuncLit:
		c.sigs = c.sigs[:len(c.sigs)-1]
	}
}

func (c *collector) sigOf(name *ast.Ident) *types.Signature {
	if obj := c.info.Defs[name]; obj != nil {
		sig, _ := obj.Type().(*types.Signature)
		return sig
	}
	return nil
}

func (c *collector) sigOfExpr(e ast.Expr) *types.Signature {
	if tv, ok := c.info.Types[e]; ok {
		sig, _ := tv.Type.(*types.Signature)
		return sig
	}
	return nil
}

// isFieldKey reports whether the identifier id, the key of a composite
// literal, is a field name (not a variable that is a map key): go/types
// records no use of it in a literal of an opaque type.
func (c *collector) isFieldKey(id *ast.Ident) bool {
	if obj, ok := c.info.Uses[id]; ok {
		if v, ok := obj.(*types.Var); ok {
			return v.IsField()
		}
		return false
	}
	return true
}

// visit makes the reference of the expression e if it has one. It returns
// whether the walk goes into e's parts.
func (c *collector) visit(e ast.Expr) bool {
	if c.noRef[e] {
		return true
	}
	switch x := e.(type) {
	case *ast.SelectorExpr:
		if c.p.IsCgoHelper(x) {
			return false // gp_cgoh.Cstr: check's own name
		}
		if p, written, ok := c.pathOf(x); ok {
			c.register(e, p, written)
			return false // the parts of a path are not expressions of their own
		}
	case *ast.Ident:
		if obj, ok := c.info.Uses[x].(*types.Var); ok && !obj.IsField() && c.op.IsOpaque(x) {
			if r := c.varRef(obj); r != nil {
				c.set.byNode[e] = r
			}
		}
		return false
	}
	if call, ok := e.(*ast.CallExpr); ok {
		// T(x) of a C/C++ value: its result has a Go type, but whether C++ allows
		// the conversion is a question for clang.
		if cv := c.p.Conversion(call); cv != nil && cv.Kind == check.AsCall {
			c.register(e, c.checkRewrite(call), "")
			return true
		}
	}
	if !c.op.IsOpaque(e) {
		return true
	}
	switch e.(type) {
	case *ast.BasicLit, *ast.KeyValueExpr, *ast.Ellipsis:
		return true
	}
	x := c.conv(e)
	switch x.(type) {
	case *Arg, *Lit, *GoType:
		return true // a value of a Go type that C++ gets: nothing to ask clang
	}
	c.register(e, x, "")
	return true
}

// helperNames maps the names of check's rewritten helpers to what the user wrote.
var helperNames = strings.NewReplacer("gp_cgoh.Cstr", "C.cstr", "gp_cgoh.Wstr", "C.wstr", "gp_cgoh.Free", "C.free", "gp_cgoh.", "C.")

// text returns the expression e as the user wrote it (check's rewrite of
// conversions and cgo helpers undone).
func (c *collector) text(e ast.Expr) string {
	if call, ok := e.(*ast.CallExpr); ok {
		if cv := c.p.Conversion(call); cv != nil && cv.Kind == check.AsCall {
			return cv.String()
		}
	}
	return helperNames.Replace(types.ExprString(e))
}

// register makes (or finds) the reference with the expression x, and adds the
// use at e.
func (c *collector) register(e ast.Expr, x Expr, written string) *Ref {
	key := x.String()
	r := c.set.byKey[key]
	if r == nil {
		r = &Ref{Kind: kindOf(x), Key: key, Expr: x, Written: written}
		r.Incomplete = incompleteOf(x)
		if len(key) > maxKey && r.Incomplete == "" {
			r.Incomplete = "expression too large"
		}
		c.set.byKey[key] = r
		c.set.Refs = append(c.set.Refs, r)
	}
	ctx, target := c.context(e)
	file, line, col := sitePos(c.fset, c.set.Dir, e.Pos())
	r.Sites = append(r.Sites, Site{File: file, Line: line, Col: col, Ctx: ctx, Target: target, Text: c.text(e)})
	// A name is only a name when it is the function of a call or the template
	// of an instance: the call (or the instance) is the expression.
	nameLike := false
	switch x.(type) {
	case *Path, *Select, *Inst, *IndexOrInst:
		nameLike = ctx == CtxCallee || ctx == CtxTemplate
	}
	if !nameLike {
		r.Standalone = true
	}
	if ctx == CtxConstant {
		r.WantValue = true
	}
	c.set.byNode[e] = r
	return r
}

func kindOf(x Expr) Kind {
	switch x := x.(type) {
	case *Path:
		return KindPath
	case *Inst, *IndexOrInst:
		if _, ok := x.(*IndexOrInst); ok {
			return KindIndex
		}
		return KindInst
	case *Call:
		return KindCall
	case *Select:
		return KindMember
	case *SliceX:
		return KindSlice
	case *Unary:
		return KindUnary
	case *Binary:
		return KindBinary
	case *Star:
		return KindDeref
	case *Addr:
		return KindAddr
	case *Paren:
		return KindParen
	case *Composite:
		return KindLiteral
	case *Decay:
		return KindLocal
	case *Stand, *GoType:
		return KindType
	}
	return KindOther
}

// incompleteOf returns why x can't be probed: the first Unknown in it.
func incompleteOf(x Expr) string {
	why := ""
	Walk(x, func(e Expr) bool {
		switch e := e.(type) {
		case *Unknown:
			if why == "" {
				why = e.Why
			}
		case *DeclRef:
			if why == "" {
				why = e.Def.Incomplete
			}
		}
		return why == ""
	})
	if cl, ok := x.(*Composite); ok && cl.Type == nil && why == "" {
		why = "literal without a type"
	}
	return why
}

// context returns where the expression e is used (see Context) and the Go
// type it goes to, when that is known.
func (c *collector) context(e ast.Expr) (Context, string) {
	// c.stack ends with e itself.
	if len(c.stack) < 2 {
		return CtxOther, ""
	}
	parent := c.stack[len(c.stack)-2]
	switch p := parent.(type) {
	case *ast.ExprStmt:
		return CtxStmt, ""
	case *ast.CallExpr:
		if p.Fun == e {
			return CtxCallee, ""
		}
		if cv := c.p.Conversion(p); cv != nil {
			return CtxConvert, c.exprText(cv.Type)
		}
		for i, a := range p.Args {
			if a == e {
				return CtxArg, c.paramType(p, i)
			}
		}
		return CtxArg, ""
	case *ast.BinaryExpr, *ast.UnaryExpr:
		return CtxOperand, ""
	case *ast.StarExpr:
		return c.typeOrOperand(e, parent)
	case *ast.ParenExpr:
		return c.typeOrOperand(e, parent)
	case *ast.AssignStmt:
		if p.Tok == token.DEFINE {
			return CtxInit, ""
		}
		for i, r := range p.Rhs {
			if r == e && len(p.Lhs) == len(p.Rhs) {
				return CtxAssign, c.completeType(p.Lhs[i])
			}
		}
		return CtxAssign, ""
	case *ast.ValueSpec:
		if p.Type == e {
			return CtxType, ""
		}
		if len(c.stack) >= 3 {
			if gd, ok := c.stack[len(c.stack)-3].(*ast.GenDecl); ok && gd.Tok == token.CONST {
				return CtxConstant, ""
			}
		}
		if p.Type != nil {
			if t := c.info.TypeOf(p.Type); t != nil && complete(t) {
				return CtxInit, t.String()
			}
		}
		return CtxInit, ""
	case *ast.ReturnStmt:
		if len(c.sigs) > 0 && c.sigs[len(c.sigs)-1] != nil {
			sig := c.sigs[len(c.sigs)-1]
			for i, r := range p.Results {
				if r == e && sig.Results().Len() == len(p.Results) {
					if t := sig.Results().At(i).Type(); complete(t) {
						return CtxReturn, t.String()
					}
				}
			}
		}
		return CtxReturn, ""
	case *ast.IfStmt:
		return CtxCond, ""
	case *ast.ForStmt:
		return CtxCond, ""
	case *ast.SwitchStmt:
		return CtxCond, ""
	case *ast.CaseClause:
		return CtxConstant, ""
	case *ast.RangeStmt:
		return CtxRange, ""
	case *ast.IndexExpr:
		if p.X == e {
			return CtxTemplate, "" // X[i]: a template (std.vector[int]) or an indexed value; the name alone is not an expression
		}
		return CtxIndex, ""
	case *ast.IndexListExpr:
		if p.X == e {
			return CtxTemplate, ""
		}
		return CtxType, ""
	case *ast.SliceExpr:
		return CtxIndex, ""
	case *ast.SelectorExpr:
		return CtxMember, ""
	case *ast.ArrayType:
		if p.Len == e {
			return CtxConstant, ""
		}
		return CtxType, ""
	case *ast.CompositeLit:
		if p.Type == e {
			return CtxType, ""
		}
		return CtxElement, ""
	case *ast.KeyValueExpr:
		return CtxElement, ""
	case *ast.Field, *ast.MapType, *ast.ChanType, *ast.TypeSpec, *ast.TypeAssertExpr, *ast.FuncType:
		return CtxType, ""
	case *ast.TypeSwitchStmt:
		return CtxCond, ""
	case *ast.IncDecStmt, *ast.SendStmt, *ast.GoStmt, *ast.DeferStmt:
		return CtxStmt, ""
	}
	return CtxOther, ""
}

// typeOrOperand is the context of e inside a * or a parenthesis: the
// context of that node (it is a type or a value as the whole is).
func (c *collector) typeOrOperand(e ast.Expr, parent ast.Node) (Context, string) {
	// Look at the node around the parent.
	if len(c.stack) < 3 {
		return CtxOperand, ""
	}
	saved := c.stack
	c.stack = c.stack[:len(c.stack)-1]
	ctx, target := c.context(parent.(ast.Expr))
	c.stack = saved
	if ctx == CtxType || ctx == CtxConstant {
		return ctx, target
	}
	return CtxOperand, ""
}

// completeType returns the text of the type of e if it is a complete Go
// type (Target of an assignment), else "".
func (c *collector) completeType(e ast.Expr) string {
	if t := c.info.TypeOf(e); t != nil && complete(t) {
		return t.String()
	}
	return ""
}

// paramType returns the text of the type of the i-th argument of the call
// when the function is a Go function with a complete parameter type.
func (c *collector) paramType(call *ast.CallExpr, i int) string {
	tv, ok := c.info.Types[call.Fun]
	if !ok {
		return ""
	}
	sig, ok := tv.Type.(*types.Signature)
	if !ok || sig.Params().Len() == 0 {
		return ""
	}
	n := sig.Params().Len()
	var t types.Type
	switch {
	case i < n-1 || (i == n-1 && !sig.Variadic()):
		t = sig.Params().At(i).Type()
	case sig.Variadic():
		if sl, ok := sig.Params().At(n - 1).Type().(*types.Slice); ok {
			t = sl.Elem()
		}
	}
	if t != nil && complete(t) {
		return t.String()
	}
	return ""
}

func (c *collector) exprText(e ast.Expr) string { return types.ExprString(e) }

// complete reports whether t has no part that go/types left invalid (a
// C/C++ type, with FakeImportC): the same rule as check's.
func complete(t types.Type) bool {
	switch t := t.(type) {
	case *types.Basic:
		return t.Kind() != types.Invalid
	case *types.Alias:
		return complete(types.Unalias(t))
	case *types.Named:
		for i := range t.TypeArgs().Len() {
			if !complete(t.TypeArgs().At(i)) {
				return false
			}
		}
		return true
	case *types.Pointer:
		return complete(t.Elem())
	case *types.Slice:
		return complete(t.Elem())
	case *types.Array:
		return complete(t.Elem())
	case *types.Chan:
		return complete(t.Elem())
	case *types.Map:
		return complete(t.Key()) && complete(t.Elem())
	case *types.Signature:
		return complete(t.Params()) && complete(t.Results())
	case *types.Tuple:
		for i := range t.Len() {
			if !complete(t.At(i).Type()) {
				return false
			}
		}
		return true
	case *types.Struct:
		for i := range t.NumFields() {
			if !complete(t.Field(i).Type()) {
				return false
			}
		}
		return true
	}
	return true
}

// pathOf returns the path that the selector chain sel names, if it starts at
// a header package, a namespace or C, and the name as the user wrote it.
func (c *collector) pathOf(sel *ast.SelectorExpr) (*Path, string, bool) {
	var names []string
	var cur ast.Expr = sel
	for {
		s, ok := cur.(*ast.SelectorExpr)
		if !ok {
			break
		}
		names = append(names, s.Sel.Name)
		if u := c.uses[s]; u != nil {
			reverse(names)
			p := &Path{Parts: names}
			written := u.Ident.Name + "." + strings.Join(names, ".")
			switch u.Kind {
			case load.NamespaceUse:
				p.Ns = u.Namespace
			default:
				p.Ns = ""
				if u.Kind == load.HeaderUse {
					p.Hdr = u.Ident.Name
				}
			}
			return p, written, true
		}
		cur = s.X
	}
	if id, ok := cur.(*ast.Ident); ok {
		if pn, ok := c.info.Uses[id].(*types.PkgName); ok && pn.Imported().Path() == "C" {
			reverse(names)
			return &Path{Ns: "C", Parts: names}, "C." + strings.Join(names, "."), true
		}
	}
	return nil, "", false
}

func reverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// varRef returns the reference of the type of the local variable obj: the
// declaration's own (KindLocal) for a variable declared from a value, the
// type expression's for one declared with a C/C++ type.
func (c *collector) varRef(obj *types.Var) *Ref {
	if r := c.defRef(obj); r != nil {
		return r
	}
	if te := c.op.TypeExpr(obj); te != nil {
		return c.set.byKey[c.typeIR(te).String()]
	}
	return nil
}

// defRef returns the reference of a variable declared without a type from
// a C/C++ value: its Expr is the Decay of the value (auto x = value).
func (c *collector) defRef(obj types.Object) *Ref {
	if r, ok := c.defRefs[obj]; ok {
		return r
	}
	d := c.decls[obj]
	if d == nil || c.inDef[obj] {
		return nil
	}
	if c.inDef == nil {
		c.inDef = make(map[types.Object]bool)
	}
	c.inDef[obj] = true
	defer delete(c.inDef, obj)
	if !c.op.IsOpaque(d.init) && complete(obj.Type()) {
		c.defRefs[obj] = nil
		return nil
	}
	var x Expr
	switch {
	case d.rangeVar != 0:
		x = &Decay{X: &RangeElem{X: c.conv(d.init), Key: d.rangeVar == 1}}
	case d.n > 1:
		x = &Decay{X: &TupleElem{X: c.conv(d.init), I: d.idx}}
	default:
		x = &Decay{X: c.conv(d.init)}
	}
	r := c.defRefForExpr(obj, x)
	c.defRefs[obj] = r
	return r
}

// defRefForExpr registers the reference of a variable's declaration.
func (c *collector) defRefForExpr(obj types.Object, x Expr) *Ref {
	key := x.String()
	r := c.set.byKey[key]
	if r == nil {
		r = &Ref{Kind: KindLocal, Key: key, Expr: x, Standalone: true}
		r.Incomplete = incompleteOf(x)
		if len(key) > maxKey && r.Incomplete == "" {
			r.Incomplete = "expression too large"
		}
		c.set.byKey[key] = r
		c.set.Refs = append(c.set.Refs, r)
	}
	file, line, col := sitePos(c.fset, c.set.Dir, obj.Pos())
	r.Sites = append(r.Sites, Site{File: file, Line: line, Col: col, Ctx: CtxInit, Text: obj.Name()})
	if id := c.nameAt(obj); id != nil {
		c.set.byNode[id] = r
	}
	return r
}

// nameAt returns the identifier that declares obj, or nil.
func (c *collector) nameAt(obj types.Object) *ast.Ident {
	for id, o := range c.info.Defs {
		if o == obj {
			return id
		}
	}
	return nil
}

// declType returns the type of the local variable whose reference is def: the
// declaration itself when it is small, else its alias.
func (c *collector) declType(def *Ref) Expr {
	if len(def.Key) <= inlineLimit {
		return def.Expr
	}
	return &DeclRef{Def: def}
}

// ---------------------------------------------------------------- IR

// conv returns the C++ pieces of the expression e.
func (c *collector) conv(e ast.Expr) Expr {
	if e == nil {
		return &Unknown{Why: "missing expression"}
	}
	if !c.op.IsOpaque(e) {
		return c.goValue(e)
	}
	switch x := e.(type) {
	case *ast.ParenExpr:
		return &Paren{X: c.conv(x.X)}
	case *ast.Ident:
		return c.identValue(x)
	case *ast.SelectorExpr:
		if p, _, ok := c.pathOf(x); ok {
			return p
		}
		return &Select{X: c.conv(x.X), Name: x.Sel.Name}
	case *ast.IndexExpr:
		return &IndexOrInst{X: c.conv(x.X), Args: []Expr{c.convArg(x.Index)}}
	case *ast.IndexListExpr:
		args := make([]Expr, len(x.Indices))
		for i, a := range x.Indices {
			args[i] = c.convArg(a)
		}
		return &Inst{X: c.conv(x.X), Args: args}
	case *ast.SliceExpr:
		return &Unknown{Why: "slice of a C/C++ value"}
	case *ast.CallExpr:
		if v := c.checkRewrite(x); v != nil {
			return v
		}
		if tv, ok := c.info.Types[x.Fun]; ok && tv.IsBuiltin() {
			if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok && (id.Name == "len" || id.Name == "cap") {
				return &Arg{T: &GoType{Cxx: "int64_t"}} // a temporary of type int
			}
			return &Unknown{Why: "builtin " + types.ExprString(x.Fun)}
		}
		if !c.op.IsOpaque(x.Fun) {
			// A call of a Go function (or a conversion to a Go type): C++ sees its
			// result, a temporary of the Go result type.
			if tv, ok := c.info.Types[x]; ok && tv.Type != nil && complete(tv.Type) && tv.Value == nil {
				return c.goValue(x)
			}
			return &Unknown{Why: "result of a Go function with a C/C++ type: " + types.ExprString(x.Fun)}
		}
		args := make([]Expr, len(x.Args))
		for i, a := range x.Args {
			args[i] = c.conv(a)
		}
		return &Call{Fun: c.conv(x.Fun), Args: args, Ellipsis: x.Ellipsis.IsValid()}
	case *ast.StarExpr:
		return &Star{X: c.conv(x.X)}
	case *ast.UnaryExpr:
		switch x.Op {
		case token.AND:
			if cl, ok := ast.Unparen(x.X).(*ast.CompositeLit); ok {
				return &Star{X: c.conv(cl).(*Composite).typeOnly()}
			}
			return &Addr{X: c.conv(x.X)}
		case token.ARROW:
			return &Unknown{Why: "receive from a channel"}
		}
		return &Unary{Op: x.Op.String(), X: c.conv(x.X)}
	case *ast.BinaryExpr:
		return &Binary{Op: x.Op.String(), X: c.conv(x.X), Y: c.conv(x.Y)}
	case *ast.CompositeLit:
		return c.convLit(x)
	case *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.StructType, *ast.InterfaceType:
		return c.typeIR(e)
	case *ast.FuncLit:
		return &Unknown{Why: "function literal"}
	case *ast.TypeAssertExpr:
		return &Unknown{Why: "type assertion"}
	}
	return &Unknown{Why: "expression " + types.ExprString(e)}
}

// checkRewrite returns the pieces of a call that check rewrote: a conversion
// T(x) (gp_cgoh.As[T](x)), a comparison of C/C++ values (gp_cgoh.Cmp(x == y)),
// or a call of a cgo helper (C.cstr(s) is gp_cgoh.Cstr(s)). It returns nil for
// any other call.
func (c *collector) checkRewrite(x *ast.CallExpr) Expr {
	if cv := c.p.Conversion(x); cv != nil {
		switch cv.Kind {
		case check.Compare:
			return c.conv(cv.Arg())
		case check.AsCall, check.UnusedStmt, check.Deferred:
			return &Call{Fun: c.typeIR(cv.Type), Args: []Expr{c.conv(cv.Arg())}}
		}
	}
	switch c.p.CgoHelper(x) {
	case "":
		return nil
	case "cstr":
		return &Arg{T: &GoType{Cxx: "const char*"}} // the holder: C++ sees the const char* it converts to
	case "wstr":
		return &Arg{T: &GoType{Cxx: "const wchar_t*"}} // the wide holder
	case "WString":
		return &Arg{T: &GoType{Cxx: "wchar_t*"}}
	case "CString":
		return &Arg{T: &GoType{Cxx: "char*"}}
	case "GoString", "GoStringN", "GoStringW", "GoStringWN":
		return &Arg{T: &Stand{Name: "string"}}
	case "GoBytes":
		return &Arg{T: &Stand{Name: "slice", Args: []Expr{&GoType{Cxx: "uint8_t"}}}}
	}
	return &Unknown{Why: "cgo helper without a value"}
}

// typeOnly returns the type of a composite literal as a pointer target: &T{}
// is *T (a literal's address is a pointer to a temporary in C++, which only
// the type is probed for).
func (e *Composite) typeOnly() Expr {
	if e.Type == nil {
		return &Unknown{Why: "literal without a type"}
	}
	return e.Type
}

func (c *collector) convLit(x *ast.CompositeLit) Expr {
	lit := &Composite{}
	if x.Type != nil {
		lit.Type = c.typeIR(x.Type)
	}
	for _, el := range x.Elts {
		var elt Elt
		v := el
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			v = kv.Value
			if id, ok := kv.Key.(*ast.Ident); ok && c.isFieldKey(id) {
				elt.KeyName = id.Name
			} else {
				elt.Key = c.conv(kv.Key)
			}
		}
		elt.Val = c.conv(v)
		lit.Elts = append(lit.Elts, elt)
	}
	return lit
}

// identValue returns the pieces of a use of an opaque variable: an lvalue of
// the type it was declared with.
func (c *collector) identValue(id *ast.Ident) Expr {
	obj, ok := c.info.Uses[id].(*types.Var)
	if !ok {
		if c.info.Uses[id] == nil {
			return &Unknown{Why: "name " + id.Name}
		}
		return c.goValue(id)
	}
	if def := c.defRef(obj); def != nil {
		return &Arg{T: c.declType(def), LValue: true}
	}
	if te := c.op.TypeExpr(obj); te != nil {
		return &Arg{T: c.typeIR(te), LValue: true}
	}
	return &Unknown{Why: "variable " + id.Name}
}

// convArg returns an argument of X[...]: a type or a constant.
func (c *collector) convArg(e ast.Expr) Expr {
	if tv, ok := c.info.Types[e]; ok && tv.Value != nil {
		return c.constant(tv)
	}
	if tv, ok := c.info.Types[e]; ok && tv.IsType() {
		return c.typeIR(e)
	}
	switch e.(type) {
	case *ast.ArrayType, *ast.MapType, *ast.StarExpr, *ast.FuncType, *ast.ChanType:
		return c.typeIR(e)
	}
	return c.conv(e)
}

// typeIR returns the pieces of the type expression e.
func (c *collector) typeIR(e ast.Expr) Expr {
	if tv, ok := c.info.Types[e]; ok && tv.IsType() && tv.Type != nil && complete(tv.Type) {
		return c.goType(tv.Type)
	}
	switch x := e.(type) {
	case *ast.ParenExpr:
		return &Paren{X: c.typeIR(x.X)}
	case *ast.StarExpr:
		return &Star{X: c.typeIR(x.X)}
	case *ast.ArrayType:
		elem := c.typeIR(x.Elt)
		if x.Len == nil {
			return &Stand{Name: "slice", Args: []Expr{elem}}
		}
		if tv, ok := c.info.Types[x.Len]; ok && tv.Value != nil {
			return &Stand{Name: "array", Args: []Expr{elem, c.constant(tv)}}
		}
		return &Stand{Name: "array", Args: []Expr{elem, c.conv(x.Len)}} // [cstdio.BUFSIZ]byte
	case *ast.MapType:
		return &Stand{Name: "map", Args: []Expr{c.typeIR(x.Key), c.typeIR(x.Value)}}
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		return c.conv(e)
	case *ast.Ident:
		return &Unknown{Why: "type " + x.Name}
	}
	return &Unknown{Why: "type " + types.ExprString(e)}
}

// goBasics are the C++ types of the basic Go types, as the generated code
// spells them (internal/lower/types.go).
var goBasics = map[types.BasicKind]string{
	types.Bool: "bool", types.Int: "int64_t", types.Int8: "int8_t", types.Int16: "int16_t",
	types.Int32: "int32_t", types.Int64: "int64_t", types.Uint: "uint64_t", types.Uint8: "uint8_t",
	types.Uint16: "uint16_t", types.Uint32: "uint32_t", types.Uint64: "uint64_t", types.Uintptr: "uintptr_t",
	types.Float32: "float", types.Float64: "double",
	types.UntypedBool: "bool", types.UntypedInt: "int64_t", types.UntypedRune: "int32_t", types.UntypedFloat: "double",
}

// goType returns the C++ type of the complete Go type t.
func (c *collector) goType(t types.Type) Expr {
	switch t := types.Unalias(t).(type) {
	case *types.Basic:
		switch {
		case t.Kind() == types.String || t.Kind() == types.UntypedString:
			return &Stand{Name: "string"}
		case t.Kind() == types.UnsafePointer:
			return &GoType{Cxx: "void*"}
		}
		if s := goBasics[t.Kind()]; s != "" {
			return &GoType{Cxx: s}
		}
	case *types.Slice:
		if el := c.goType(t.Elem()); !isUnknown(el) {
			return &Stand{Name: "slice", Args: []Expr{el}}
		}
	case *types.Array:
		if el := c.goType(t.Elem()); !isUnknown(el) {
			return &Stand{Name: "array", Args: []Expr{el, &Lit{Text: strconv.FormatInt(t.Len(), 10)}}}
		}
	case *types.Map:
		k, v := c.goType(t.Key()), c.goType(t.Elem())
		if !isUnknown(k) && !isUnknown(v) {
			return &Stand{Name: "map", Args: []Expr{k, v}}
		}
	}
	return &Unknown{Why: "Go type " + t.String()}
}

func isUnknown(e Expr) bool {
	bad := false
	Walk(e, func(x Expr) bool {
		if _, ok := x.(*Unknown); ok {
			bad = true
		}
		return !bad
	})
	return bad
}

// goValue returns the pieces of an expression that is not opaque: a
// constant is a C literal, anything else a value of its Go type.
func (c *collector) goValue(e ast.Expr) Expr {
	tv, ok := c.info.Types[e]
	if !ok {
		return &Unknown{Why: "no type for " + types.ExprString(e)}
	}
	switch {
	case tv.Value != nil:
		return c.constant(tv)
	case tv.IsNil():
		return &Lit{Text: "nullptr"}
	case tv.IsType():
		return c.typeIR(e)
	case tv.IsBuiltin() || tv.IsVoid():
		return &Unknown{Why: "builtin " + types.ExprString(e)}
	case tv.Type == nil || !complete(tv.Type):
		return &Unknown{Why: "no complete type for " + types.ExprString(e)}
	}
	t := c.goType(tv.Type)
	if isUnknown(t) {
		return t
	}
	return &Arg{T: t, LValue: c.isLValue(e)}
}

// isLValue reports whether the Go expression e is a place in the generated
// code: a variable, a field, an element or a dereference.
func (c *collector) isLValue(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		_, ok := c.info.Uses[x].(*types.Var)
		return ok
	case *ast.SelectorExpr:
		if sel := c.info.Selections[x]; sel != nil {
			return sel.Kind() == types.FieldVal
		}
		_, ok := c.info.Uses[x.Sel].(*types.Var)
		return ok
	case *ast.IndexExpr:
		tv := c.info.Types[x.X]
		if tv.Type == nil {
			return false
		}
		switch tv.Type.Underlying().(type) {
		case *types.Slice, *types.Array, *types.Map, *types.Pointer:
			return true
		}
		return false
	case *ast.StarExpr:
		return true
	}
	return false
}

// constant returns the C literal of the constant value tv: an untyped
// constant has C's default type; a typed one is a value of its type, except a
// string, which is a raw literal (Phase 04, §2.6).
func (c *collector) constant(tv types.TypeAndValue) Expr {
	b, _ := types.Unalias(tv.Type).Underlying().(*types.Basic)
	untyped := b != nil && b.Info()&types.IsUntyped != 0
	switch tv.Value.Kind() {
	case constant.String:
		return &Lit{Text: cString(constant.StringVal(tv.Value))}
	case constant.Bool:
		if !untyped && b != nil {
			return &Arg{T: &GoType{Cxx: "bool"}}
		}
		return &Lit{Text: strconv.FormatBool(constant.BoolVal(tv.Value))}
	case constant.Int:
		if !untyped && b != nil {
			if s := goBasics[b.Kind()]; s != "" {
				return &Arg{T: &GoType{Cxx: s}}
			}
		}
		if b != nil && b.Kind() == types.UntypedRune {
			if v, ok := constant.Int64Val(tv.Value); ok && v >= 0x20 && v < 0x7f && v != '\'' && v != '\\' {
				return &Lit{Text: "'" + string(rune(v)) + "'"}
			}
		}
		return &Lit{Text: cInt(tv.Value)}
	case constant.Float:
		if !untyped && b != nil {
			if s := goBasics[b.Kind()]; s != "" {
				return &Arg{T: &GoType{Cxx: s}}
			}
		}
		f, _ := constant.Float64Val(tv.Value)
		return &Lit{Text: cFloat(f)}
	}
	return &Unknown{Why: "constant of kind " + tv.Value.Kind().String()}
}

func cInt(v constant.Value) string {
	if u, ok := constant.Uint64Val(v); ok {
		if u > math.MaxInt64 {
			return strconv.FormatUint(u, 10) + "ULL"
		}
		return strconv.FormatUint(u, 10)
	}
	if i, ok := constant.Int64Val(v); ok {
		if i == math.MinInt64 {
			return "(-9223372036854775807LL - 1)"
		}
		return "(" + strconv.FormatInt(i, 10) + ")"
	}
	return v.String()
}

func cFloat(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	switch {
	case math.IsInf(f, 1):
		return "(1.0 / 0.0)"
	case math.IsInf(f, -1):
		return "(-1.0 / 0.0)"
	case math.IsNaN(f):
		return "(0.0 / 0.0)"
	}
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	if f < 0 {
		return "(" + s + ")"
	}
	return s
}

// cString returns the C++ string literal for s: printable ASCII as it is,
// everything else as 3-digit octal escapes (which can't swallow the digits
// that follow, as \x does).
func cString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '"' || ch == '\\':
			b.WriteByte('\\')
			b.WriteByte(ch)
		case ch >= 0x20 && ch < 0x7f && ch != '?':
			b.WriteByte(ch)
		default:
			b.WriteString("\\" + string([]byte{'0' + ch>>6, '0' + (ch>>3)&7, '0' + ch&7}))
		}
	}
	b.WriteByte('"')
	return b.String()
}
