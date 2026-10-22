package check

import (
	"go/ast"
	"go/token"
	"go/types"
	"iter"

	"github.com/huuhait/gocpp/internal/diag"
)

// An Opaque is the result of the opaque pass over one checked package
// (Phase 01 README §2.8). It says, for each expression of the package,
// whether codegen knows its exact type (typed) or has to leave it to the
// C++ compiler (opaque), and it keeps the type expressions that codegen
// prints C/C++ types from.
//
// An expression is typed when Info.Types has its type and that type is
// complete: it has no part that go/types left invalid, outside a named
// type (a builtin such as len and a call without results are typed too).
// A generic function that go/types couldn't instantiate (no
// Info.Instances entry: id(cv), two[int](1, cv)) is not typed.
//
// An expression that is not typed is opaque when it comes from C/C++:
//   - it is a name of C (the loader made C.X out of header and namespace
//     uses: raylib.InitWindow, std.vector), of an object that comes from
//     C/C++ (see below), or of an instance of a generic function with a
//     type argument that is not complete (slices.Sort with
//     var xs []C.int);
//   - or one of its operands is opaque: the X of a selector, a method call
//     or an index, the function or an argument of a call, the operands of
//     unary and binary expressions, the type of a function literal, the
//     parts of a type expression (an operand that is typed doesn't count,
//     even if it comes from C/C++ in the sense of objects below, with one
//     exception: a value or a type of a named type declared over a C/C++
//     type, h in h + 1 with type Handle C.int, whose underlying type
//     go/types doesn't know; see cOperand, which also says where a pointer
//     or a container of one counts);
//   - or it is a composite literal of an opaque type, also one whose type
//     was left out ([]C.Point{{1, 2}}) inside such a literal. (A literal
//     of a named type over a C/C++ type stays typed: P{X: 1} with
//     type P C.Point has the type P, and only its keys are
//     opaque (go/types doesn't look at them), so codegen has to notice a
//     named type whose underlying type is invalid and print the C++ keyed
//     form from TypeExpr(P).)
//   - or it is in a subtree that go/types skipped because it stopped at
//     an opaque expression next to it (see skipped): the T of x.(T) with x
//     opaque, the sizes of make(T, n) with T opaque, the values of
//     const k C.int = 1, the keys of an opaque literal, the function of a
//     generic call with an opaque argument, the arguments of lib.CB(n) and
//     the indices of lib.CB[1:n] (a C/C++ value that go/types can't call
//     or slice), the values of map[Handle]int{1: n}. checkSkipped gives
//     most of these real types first, so they are typed; what it can't
//     type (a name as a key, a value with iota) stays opaque.
//
// An object of the package comes from C/C++ when its declared type comes
// from C/C++ (var xs []C.int, func f(w *SDL.SDL_Window), a struct field, a
// result, type Handle C.int, and also var h Handle, through the named
// type), or, if it has no declared type, when its value is opaque:
// x := lib.foo(), a, b := std.minmax(x, y), the range variables of a range
// over an opaque value, the variable of a type switch over one. A closure
// sees the same objects, so a variable that a closure captures stays what
// it is. (A range variable, or one of several variables declared from one
// value, also comes from C/C++ when that value has a named type over a
// C/C++ type: range v with type V C.vec. A blank name on the left of an
// assignment comes from C/C++ when a value on the right does: go/types
// doesn't record it when the values are invalid; so does one of a range
// that assigns, for _, v = range c, when c does, and the other targets
// there are skipped expressions, skipRange.) An object of another
// package comes from C/C++ when its type is not complete (that package's
// own opaque pass checked where the type came from).
//
// The sanity rule (findOpaque): when the package has no errors but
// "imported and not used", "declared and not used" and "label declared and
// not used", every expression that is not typed must be opaque, else the
// pass reports an internal error. Other errors, also other soft errors of
// go/types, can leave types missing, so they turn the rule off.
//
// The pass labels every expression that go/types recorded in Info.Types,
// and every other expression except those that go/types leaves out in
// valid Go code: names that declare something (in Info.Defs; the names on
// the left of := are never labeled), the Sel of a selector, package names
// (C in C.X, fmt in fmt.Println), labels, the field names in the keys of a
// struct literal, KeyValueExpr nodes, import paths and field tags, the
// FuncType of a function declaration, the x.(type) of a type switch, and
// the ... of [...]T. Each labeled expression is either typed or opaque.
// (In Go's own standard library, go/types leaves no other expression out:
// TestOpaqueGoStdlib.)
//
// The nil *Opaque (a package that was not checked) has no opaque
// expressions and no type expressions.
type Opaque struct {
	exprs     map[ast.Expr]bool
	typeExprs map[types.Object]ast.Expr
}

// IsOpaque reports whether the expression e is opaque: go/types has no
// complete type for it, and it comes from C/C++. Codegen then uses auto
// and the helpers of Phase 04 (gp::deref, gp::range) instead of gocpp's
// exact C++ types.
func (o *Opaque) IsOpaque(e ast.Expr) bool {
	return o != nil && o.exprs[e]
}

// TypeExpr returns the type expression that the object obj of the package
// was declared with, if obj's type is not complete because it contains a
// C/C++ type: *SDL.SDL_Window for the parameter w of
// func f(w *SDL.SDL_Window), and likewise for variables, struct fields,
// results, constants and type declarations (type Handle C.int, and a
// struct type with such a field). go/types has no type for such a part, so
// codegen prints it from this expression (after the loader's rewrite,
// rooted in C: *C.SDL_Window). TypeExpr returns nil for any other object:
// a function (its parameters and results have their own), an object of
// another package (ask that package's Opaque), and an object declared
// without a type (x := lib.foo(), whose C++ type is auto). A variable
// declared from a conversion, xs := []C.int(cv) (gp_cgoh.As[[]C.int](cv)
// after the rewrite, see Conversion), counts as declared with its type:
// TypeExpr gives []C.int (Conversion.Type).
//
// A parameter or result without a name has no object: codegen reads its
// type from the fields of the function's FuncType. TypeExpr is keyed by
// the declared object, so for a field of an instance of a generic type
// (G[int].f), pass its Origin().
func (o *Opaque) TypeExpr(obj types.Object) ast.Expr {
	if o == nil {
		return nil
	}
	return o.typeExprs[obj]
}

// TypeExprs returns every object that has a type expression (TypeExpr),
// with that expression, in no fixed order. The language server keeps the
// expressions of the objects that importers can see beside the export data,
// which has no types for them (Phase 11, step 11.3b).
func (o *Opaque) TypeExprs() iter.Seq2[types.Object, ast.Expr] {
	return func(yield func(types.Object, ast.Expr) bool) {
		if o == nil {
			return
		}
		for obj, e := range o.typeExprs {
			if !yield(obj, e) {
				return
			}
		}
	}
}

// findOpaque runs the opaque pass over the checked package p and returns
// its result. Check runs it on the Info of the last check of p. When p
// has no error that can leave a type incomplete (see typesMayBeIncomplete:
// only the errors about something unused can't), every expression that is
// not typed must be opaque (the sanity rule): findOpaque adds an internal
// error to p.Errors for each one that isn't, so codegen never guesses. It
// reports only the innermost of such expressions, not each expression
// around it.
func findOpaque(fset *token.FileSet, p *Package) *Opaque {
	files := make([]*ast.File, len(p.Load.Files))
	for i, f := range p.Load.Files {
		files[i] = f.AST
	}
	op := newOpaquePass(p.Types, p.Info, files)
	op.conversion = p.Conversion
	op.cpp = p.cppSels
	op.label(files)
	if !typesMayBeIncomplete(p.Errors) {
		op.unusedRangeVars(fset, p)
		op.notConstants(fset, p)
		for _, e := range op.bad {
			p.Errors.Add(diag.Diagnostic{
				Pos: fset.Position(e.Pos()),
				Msg: "internal error: expression has no type: " + p.exprString(e) +
					" (this is a bug in gocpp: please report it, with this code)",
			})
		}
	}
	return op.result
}

// provisionalOpaque is the opaque pass over the first pass's check without its sanity errors: the
// second pass for promoted C++ members (cppmembers.go) reads the type expressions from it, and
// findOpaque runs again with the answers.
func provisionalOpaque(fset *token.FileSet, p *Package) *Opaque {
	op := newOpaquePass(p.Types, p.Info, p.astFiles())
	op.conversion = p.Conversion
	op.label(p.astFiles())
	return op.result
}

// promoted reports whether the selector e, whose X is c, is a member of an embedded C/C++ class:
// the second pass's answer when it looked at e, else the first pass's rule (promoted.go).
func (op *opaquePass) promoted(e *ast.SelectorExpr, c ast.Expr) bool {
	if r, ok := op.cpp[e]; ok {
		return r.opaque
	}
	return promotedPath(op.pkg, op.info.Types[c].Type, e.Sel.Name) != nil
}

// unusedRangeVars adds the "declared and not used" errors that go/types leaves out for the
// variables of a range that declares them over a value or element it has no type for (range
// over a C/C++ value, or the element of a gocpp container of C/C++ values): it gives such a
// variable an invalid type and counts it as used, to avoid follow-on errors. With no other error
// that could leave a type incomplete, an invalid type is the opaque value's, so Go's rule
// applies: for i, x := range v {} must use i and x, or write _. The use is any name that
// go/types recorded for the variable (an assignment that only writes it counts, which Go
// doesn't).
func (op *opaquePass) unusedRangeVars(fset *token.FileSet, p *Package) {
	var used map[types.Object]bool
	for _, f := range p.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			r, ok := n.(*ast.RangeStmt)
			if !ok || r.Tok != token.DEFINE {
				return true
			}
			for _, e := range []ast.Expr{r.Key, r.Value} {
				id, ok := e.(*ast.Ident)
				if !ok || id.Name == "_" {
					continue
				}
				obj := op.info.Defs[id]
				if obj == nil || obj.Type() != types.Typ[types.Invalid] {
					continue
				}
				if used == nil {
					used = make(map[types.Object]bool, len(op.info.Uses))
					for _, o := range op.info.Uses {
						used[o] = true
					}
				}
				if !used[obj] && !namedAsKey(r.Body, id.Name) {
					p.Errors.Add(diag.Diagnostic{
						Pos:  fset.Position(id.Pos()),
						Msg:  "declared and not used: " + id.Name,
						Soft: true,
						Code: diag.UnusedVar,
					})
				}
			}
			return true
		})
	}
}

// namedAsKey reports whether body has a composite literal with a key that is the bare name name
// (std.map[int, int]{i: 3}). Such a key can name a variable or a field of a C/C++ struct
// (C.Rect{w: 640}), so go/types and the skipped-expression pass record no use of it
// (markKeyName); the range variable is conservatively used then, as markKeyName counts it.
func namedAsKey(body *ast.BlockStmt, name string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok {
			for _, e := range lit.Elts {
				if kv, ok := e.(*ast.KeyValueExpr); ok {
					if id, ok := ast.Unparen(kv.Key).(*ast.Ident); ok && id.Name == name {
						found = true
					}
				}
			}
		}
		return !found
	})
	return found
}

// unusedLabel is the go/types error code of "label L declared and not
// used" (UnusedLabel in Go's internal/types/errors).
const unusedLabel = 122

// typesMayBeIncomplete reports whether l has an error that can leave types
// incomplete: any error but the soft errors about something unused. Other
// soft errors of go/types can (for "cannot range over 3: requires go1.22
// or later", the range variables have no type). (hasHardErrors, for the
// packages whose dependencies failed, counts only the errors that are not
// soft.)
func typesMayBeIncomplete(l *diag.List) bool {
	for _, d := range l.Diagnostics() {
		if d.Severity != diag.Error {
			continue
		}
		switch {
		case !d.Soft:
			return true
		case d.Code != diag.UnusedImport && d.Code != diag.UnusedVar && d.Code != unusedLabel:
			return true
		}
	}
	return false
}

// An opaquePass holds the state of the opaque pass over one package.
type opaquePass struct {
	pkg  *types.Package
	info *types.Info

	// cpp is what the second pass for promoted C++ members decided (Package.cppSels), or nil.
	cpp map[*ast.SelectorExpr]*cppSel

	// decls says how each object of the package was declared.
	decls map[types.Object]decl

	// skip holds the expressions that the pass doesn't label (see Opaque).
	skip map[ast.Expr]bool

	// litParent maps a composite literal whose type was left out to the
	// literal it is an element (or a key) of.
	litParent map[*ast.CompositeLit]*ast.CompositeLit
	// mapKey maps the value of an element of a map literal to its key (skipMapValue).
	mapKey map[ast.Expr]ast.Expr

	// instIdents maps the name of a generic function in F[T] with a type argument from C/C++ to
	// that instantiation: go/types records nothing for the name then (a call has the same in
	// skipGeneric).
	instIdents map[*ast.Ident]ast.Expr

	// keyOf maps the key of a keyed element to its composite literal.
	keyOf map[ast.Expr]*ast.CompositeLit

	// blankFrom maps each blank name on the left of an assignment
	// (_, _ = x.(T)) to the values on the right. go/types records the
	// name in Defs, but not when the values are invalid (it doesn't look
	// at the left then): when one of the values is opaque, so is the name.
	blankFrom map[*ast.Ident][]ast.Expr

	// skipped maps each expression in a skipped root (see skipped) to
	// that root, the innermost one if roots nest; roots lists the roots
	// in the order they were found.
	skipped map[ast.Expr]*skipped
	roots   []*skipped

	// opq, fromC and objC are the memos of opaqueExpr, comesFromC and
	// objectFromC.
	opq   map[ast.Expr]bool
	fromC map[ast.Expr]bool
	objC  map[types.Object]bool

	// bad are the expressions that are neither typed nor opaque, in the
	// order they were found.
	bad []ast.Expr

	result *Opaque

	// conversion returns the conversion of the package whose call is
	// call, or nil (Package.Conversion; nil in checkSkipped, which runs
	// before the conversions are known).
	conversion func(call *ast.CallExpr) *Conversion
}

// A decl is how an object of the package was declared: with a type
// expression, or (without one) with the value it starts as. For several
// variables declared from one value (a, b := f(), the range variables
// of range x), init is that value.
type decl struct {
	typ  ast.Expr
	init ast.Expr

	// value says that init is the object's own value (x := v, var x = v),
	// not a value it comes from (a, b := f(), range v).
	value bool

	// yield, when not 0, says that the object is a range variable of a range over a func and
	// init is that func: the object is parameter number yield (counted from 1) of the yield
	// function, whose type go/types may have left invalid (func(yield func(std.string) bool)).
	yield int
}

func newOpaquePass(pkg *types.Package, info *types.Info, files []*ast.File) *opaquePass {
	op := &opaquePass{
		pkg:        pkg,
		info:       info,
		decls:      make(map[types.Object]decl, len(info.Defs)),
		skip:       make(map[ast.Expr]bool, len(info.Defs)),
		litParent:  make(map[*ast.CompositeLit]*ast.CompositeLit),
		mapKey:     make(map[ast.Expr]ast.Expr),
		instIdents: make(map[*ast.Ident]ast.Expr),
		keyOf:      make(map[ast.Expr]*ast.CompositeLit),
		blankFrom:  make(map[*ast.Ident][]ast.Expr),
		skipped:    make(map[ast.Expr]*skipped),
		opq:        make(map[ast.Expr]bool),
		fromC:      make(map[ast.Expr]bool),
		objC:       make(map[types.Object]bool),
		result:     &Opaque{exprs: make(map[ast.Expr]bool), typeExprs: make(map[types.Object]ast.Expr)},
	}
	for _, f := range files {
		ast.Inspect(f, op.collect)
	}
	return op
}

// collect is the first walk over a file: it fills decls, skip, litParent,
// keyOf and skipped.
func (op *opaquePass) collect(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.ImportSpec:
		return false // a name (Defs) and a path, nothing to label
	case *ast.GenDecl:
		op.collectGenDecl(n)
	case *ast.FuncDecl:
		op.skip[n.Type] = true
		if obj := op.info.Defs[n.Name]; obj != nil {
			op.decls[obj] = decl{typ: n.Type}
		}
	case *ast.Field:
		op.collectField(n)
	case *ast.ArrayType:
		if _, ok := n.Len.(*ast.Ellipsis); ok {
			op.skip[n.Len] = true // [...]T
		}
	case *ast.SelectorExpr:
		op.skip[n.Sel] = true
	case *ast.TypeAssertExpr:
		if n.Type == nil {
			op.skip[n] = true // x.(type)
		} else {
			op.addSkipped(n.Type, n.X, skipAssert)
		}
	case *ast.IndexExpr, *ast.IndexListExpr:
		if op.opaqueInstance(n.(ast.Expr)) {
			var base ast.Expr
			if x, ok := n.(*ast.IndexExpr); ok {
				base = x.X
			} else {
				base = n.(*ast.IndexListExpr).X
			}
			if id := instanceIdent(base); id != nil {
				op.instIdents[id] = n.(ast.Expr)
			}
		}
	case *ast.CallExpr:
		op.collectCall(n)
	case *ast.SliceExpr:
		for _, e := range []ast.Expr{n.Low, n.High, n.Max} {
			op.addUnrecorded(e, n.X, skipSlice)
		}
	case *ast.KeyValueExpr:
		op.skip[n] = true
	case *ast.CompositeLit:
		op.collectLit(n)
	case *ast.AssignStmt:
		op.collectAssign(n)
	case *ast.RangeStmt:
		for i, e := range []ast.Expr{n.Key, n.Value} {
			id, _ := e.(*ast.Ident)
			switch {
			case e == nil:
			case n.Tok == token.DEFINE:
				if obj := op.info.Defs[id]; id != nil && obj != nil {
					d := decl{init: n.X}
					if _, ok := op.core(n.X).(*types.Signature); ok {
						d.yield = i + 1 // range over a func
					}
					op.decls[obj] = d
				}
			case id != nil && id.Name == "_":
				op.blankFrom[id] = []ast.Expr{n.X} // for _, v = range c
			default:
				op.addUnrecorded(e, n.X, skipRange)
			}
		}
	case *ast.TypeSwitchStmt:
		op.collectTypeSwitch(n)
	}
	return true
}

// collectGenDecl records the objects of a const, var or type declaration.
func (op *opaquePass) collectGenDecl(d *ast.GenDecl) {
	// In a const group, a spec without type and values repeats the last
	// spec that has some (iota).
	var lastType ast.Expr
	var lastValues []ast.Expr
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if obj := op.info.Defs[s.Name]; obj != nil {
				op.decls[obj] = decl{typ: s.Type}
			}
		case *ast.ValueSpec:
			typ, values := s.Type, s.Values
			if d.Tok == token.CONST {
				if typ == nil && values == nil {
					typ, values = lastType, lastValues
				}
				lastType, lastValues = typ, values
			}
			if d.Tok == token.CONST && typ != nil {
				for _, v := range values {
					if op.skipped[v] == nil {
						op.addSkipped(v, typ, skipConst)
					}
				}
			}
			for i, name := range s.Names {
				obj := op.info.Defs[name]
				if obj == nil {
					continue
				}
				dc := decl{typ: typ}
				if typ == nil {
					dc.init = valueOf(values, i, len(s.Names))
					dc.value = len(values) == len(s.Names)
				}
				op.decls[obj] = dc
			}
		}
	}
}

// valueOf returns the value that the i-th of n names is declared from:
// values[i], or the one value of a multi-value declaration (a, b := f()),
// or nil.
func valueOf(values []ast.Expr, i, n int) ast.Expr {
	switch {
	case len(values) == n:
		return values[i]
	case len(values) == 1:
		return values[0]
	}
	return nil
}

// collectField records the objects that a field declares: parameters,
// results, receivers, struct fields (also embedded ones), interface
// methods and type parameters. All of them are declared with the field's
// type expression. It also skips the field's tag.
func (op *opaquePass) collectField(f *ast.Field) {
	if f.Tag != nil {
		op.skip[f.Tag] = true
	}
	for _, name := range f.Names {
		if obj := op.info.Defs[name]; obj != nil {
			op.decls[obj] = decl{typ: f.Type}
		}
	}
	if len(f.Names) == 0 {
		// An embedded field: go/types records the field in Defs, at the
		// name of its type (T in *T, pkg.T or T[int]; embeddedIdent is in
		// gatewalk.go).
		if id := embeddedIdent(f.Type); id != nil {
			if obj, ok := op.info.Defs[id].(*types.Var); ok {
				op.decls[obj] = decl{typ: f.Type}
			}
		}
	}
}

// collectLit records the elements of a composite literal: the literals
// among them whose type was left out, and the keys.
func (op *opaquePass) collectLit(lit *ast.CompositeLit) {
	elided := func(e ast.Expr) {
		if inner, ok := e.(*ast.CompositeLit); ok && inner.Type == nil {
			op.litParent[inner] = lit
		}
	}
	var mapType *types.Map
	if tv, ok := op.info.Types[lit]; ok && tv.Type != nil {
		mapType, _ = tv.Type.Underlying().(*types.Map)
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			elided(elt)
			continue
		}
		elided(kv.Value)
		elided(kv.Key)
		op.keyOf[kv.Key] = lit
		op.addSkipped(kv.Key, lit, skipKey)
		if mapType != nil {
			op.mapKey[kv.Value] = kv.Key
			op.addUnrecorded(kv.Value, lit, skipMapValue)
		}
	}
}

// collectAssign records the variables that a short variable declaration
// declares, and skips its names. Of an assignment, it records the blank
// names on the left (see blankFrom).
func (op *opaquePass) collectAssign(s *ast.AssignStmt) {
	if s.Tok == token.ASSIGN {
		for _, lhs := range s.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && id.Name == "_" {
				op.blankFrom[id] = s.Rhs
			}
		}
		return
	}
	if s.Tok != token.DEFINE {
		return
	}
	for i, lhs := range s.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		// go/types records a name on the left of := in Defs if it is
		// new, and in Uses if it is not, never in Types.
		op.skip[id] = true
		if obj := op.info.Defs[id]; obj != nil {
			op.decls[obj] = decl{init: valueOf(s.Rhs, i, len(s.Lhs)), value: len(s.Rhs) == len(s.Lhs)}
		}
	}
}

// collectTypeSwitch records the variable of each case of a type switch
// with a variable (switch v := x.(type)): in a case with one type, v has
// that type; else (several types, nil, default) v has the type of x.
func (op *opaquePass) collectTypeSwitch(s *ast.TypeSwitchStmt) {
	assign, ok := s.Assign.(*ast.AssignStmt)
	if !ok || len(assign.Rhs) != 1 {
		return
	}
	guard, ok := assign.Rhs[0].(*ast.TypeAssertExpr)
	if !ok {
		return
	}
	for _, stmt := range s.Body.List {
		cc, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		obj := op.info.Implicits[cc]
		if obj == nil {
			continue
		}
		if len(cc.List) == 1 && !op.info.Types[cc.List[0]].IsNil() {
			op.decls[obj] = decl{typ: cc.List[0]}
		} else {
			op.decls[obj] = decl{init: guard.X}
		}
	}
}

// label is the second walk: it labels each expression of the files, and
// collects the expressions that are neither typed nor opaque in op.bad
// (only the innermost ones). Then it records the type expressions.
func (op *opaquePass) label(files []*ast.File) {
	type frame struct {
		node     ast.Node
		badBelow bool
	}
	var stack []frame
	visit := func(n ast.Node) bool {
		if n != nil {
			if _, ok := n.(*ast.ImportSpec); ok {
				return false
			}
			stack = append(stack, frame{node: n})
			return true
		}
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		bad := top.badBelow
		if e, ok := top.node.(ast.Expr); ok && op.labeled(e) && !op.typed(e) {
			switch {
			case op.opaqueExpr(e):
				op.result.exprs[e] = true
			case !bad:
				op.bad = append(op.bad, e)
				bad = true
			}
		}
		if bad && len(stack) > 0 {
			stack[len(stack)-1].badBelow = true
		}
		return true
	}
	for _, f := range files {
		ast.Inspect(f, visit)
	}
	for obj, d := range op.decls {
		if _, ok := obj.(*types.Func); ok {
			continue // its parameters and results are recorded
		}
		typ := d.typ
		if typ == nil && d.value {
			typ = op.convertedTo(d.init)
		}
		if typ != nil && incompleteObject(obj) {
			op.result.typeExprs[obj] = typ
		}
	}
}

// convertedTo returns T if e is a conversion T(x) of an opaque value that
// Check rewrote to gp_cgoh.As[T](x) (see Conversion), else nil. A variable
// declared as xs := []C.int(cv) has the type of T, so T is its type
// expression.
func (op *opaquePass) convertedTo(e ast.Expr) ast.Expr {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || op.conversion == nil {
		return nil
	}
	if c := op.conversion(call); c != nil && c.Kind == AsCall {
		return c.Type
	}
	return nil
}

// labeled reports whether the pass labels e: every expression that
// go/types recorded, and every other one except those that go/types leaves
// out in valid code (see Opaque).
func (op *opaquePass) labeled(e ast.Expr) bool {
	if _, ok := op.info.Types[e]; ok {
		// Also an embedded field's name, which is in Defs too.
		return true
	}
	if op.skip[e] {
		return false
	}
	if par, ok := e.(*ast.ParenExpr); ok {
		// (_) = x: go/types records neither the parentheses nor the blank inside them
		if id, ok := ast.Unparen(par).(*ast.Ident); ok && id.Name == "_" {
			return false
		}
	}
	id, ok := e.(*ast.Ident)
	if !ok {
		return true
	}
	if _, ok := op.info.Defs[id]; ok {
		return false // a declared name, or _ on the left of =
	}
	if id.Name == "_" && op.info.Uses[id] == nil {
		return false // the blank label of _: (go/types declares no label for it)
	}
	switch obj := op.info.Uses[id].(type) {
	case *types.PkgName, *types.Label:
		return false
	case *types.Var:
		// A field name in the key of a struct literal. (In a literal of
		// an opaque type, go/types doesn't look at the keys: they are
		// opaque.)
		return !obj.IsField() || op.keyOf[id] == nil
	}
	return true
}

// typed reports whether go/types recorded a complete type for e.
func (op *opaquePass) typed(e ast.Expr) bool {
	tv, ok := op.info.Types[e]
	switch {
	case !ok:
		return false
	case tv.IsBuiltin(), tv.IsVoid():
		return true
	case tv.Type == nil || incomplete(tv.Type):
		return false
	}
	// A generic function that go/types couldn't instantiate (id(cv),
	// two[int](1, cv) with cv opaque) has its generic signature and no
	// instance: codegen can't know its type arguments.
	if sig, ok := tv.Type.(*types.Signature); ok && sig.TypeParams().Len() > 0 {
		if id := instanceIdent(e); id != nil {
			_, ok := op.info.Instances[id]
			return ok
		}
	}
	return true
}

// instanceIdent returns the name that go/types records the instance of a
// generic function under (Info.Instances) for the expression e: id in id,
// id[int] and pkg.id. It returns nil for any other expression.
func instanceIdent(e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x
		case *ast.SelectorExpr:
			return x.Sel
		case *ast.ParenExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// opaqueExpr reports whether e is opaque (see Opaque): it has no complete
// type, and it comes from C/C++ itself (it names an object that comes from
// C/C++, it is a literal of an opaque type, go/types skipped it next to an
// opaque expression) or one of its operands is opaque.
func (op *opaquePass) opaqueExpr(e ast.Expr) bool {
	if e == nil || op.typed(e) {
		return false
	}
	if v, ok := op.opq[e]; ok {
		return v
	}
	op.opq[e] = false // in progress, as in comesFromC
	v := op.computeOpaque(e)
	op.opq[e] = v
	return v
}

func (op *opaquePass) computeOpaque(e ast.Expr) bool {
	if sk := op.skipped[e]; sk != nil && op.skippedFromC(sk) {
		return true
	}
	switch e := e.(type) {
	case *ast.Ident:
		if _, ok := op.instIdents[e]; ok {
			return true // F in F[std.vector[int]]
		}
		for _, v := range op.blankFrom[e] {
			if op.cValue(v) {
				return true
			}
		}
		return op.identFromC(e)
	case *ast.CompositeLit:
		if op.litTypeOpaque(e) {
			return true
		}
	}
	found := false
	exprChildren(e, func(c ast.Expr) {
		if !found && op.cOperand(e, c) {
			found = true
		}
	})
	return found
}

// cOperand reports whether the operand c makes the expression e around
// it, which has no complete type, opaque: c is a C/C++ value (cValue:
// opaque, or its own type is a named type over a C/C++ type, type
// Handle C.int, whose underlying type is invalid, so that go/types can't
// type h + 1, h.x or len(h); falseErrors drops its errors about them). Two
// operations go/types also can't type for a Go value with such a type in
// it, after a dropped error: the X of a selector may be a pointer to one
// (pp.x with var pp *P and type P C.Point, cPointer), and an argument of
// delete or copy a container of one (delete(m, 1) with
// m map[Handle]int, cNamed). Nowhere else does such a Go value count, so
// that -hs or pp + 1, whose errors stay, are no more opaque than the
// errors say.
//
// A Go value of a named type with C/C++ parts (type Chars [1]C.char, whose
// underlying type is complete but for the element: holdsC) has a type, but
// what is taken out of it has none: a[0], a[1:], *p with type Ptr *C.int
// and f() with type Fn func() C.int are opaque, and so is the range
// variable over one (objectFromC).
func (op *opaquePass) cOperand(e, c ast.Expr) bool {
	switch e := e.(type) {
	case *ast.SelectorExpr:
		if c == e.X {
			return op.cPointer(c) || op.promoted(e, c)
		}
	case *ast.IndexExpr:
		if c == e.X {
			return op.cValue(c) || op.holdsC(c)
		}
	case *ast.SliceExpr:
		if c == e.X {
			return op.cValue(c) || op.holdsC(c)
		}
	case *ast.StarExpr:
		return op.cValue(c) || op.holdsC(c)
	case *ast.CallExpr:
		if c == e.Fun {
			return op.cValue(c) || op.holdsC(c)
		}
		if id, ok := ast.Unparen(e.Fun).(*ast.Ident); ok {
			b, ok := op.info.Uses[id].(*types.Builtin)
			if ok && (b.Name() == "delete" || b.Name() == "copy") {
				return op.cValue(c) || op.cNamed(c)
			}
		}
	}
	return op.cValue(c)
}

// comesFromC reports whether the type expression e comes from C/C++,
// whatever its type: it names C or an object that comes from C/C++, or a
// part of it does. Unlike opaqueExpr, a part with a complete type counts
// too: Handle in var h Handle, with type Handle C.int, has a complete
// type (a named type), but h comes from C/C++.
func (op *opaquePass) comesFromC(e ast.Expr) bool {
	if v, ok := op.fromC[e]; ok {
		return v
	}
	// While e is in progress, it counts as not from C/C++. Only a
	// recursive type can get back to e (type T struct{ next *T }), and
	// that goes through the name of a type, which is typed: whatever it
	// says, it doesn't change a label.
	op.fromC[e] = false
	v := false
	if id, ok := e.(*ast.Ident); ok {
		v = op.identFromC(id)
	} else {
		exprChildren(e, func(c ast.Expr) {
			if !v && op.comesFromC(c) {
				v = true
			}
		})
	}
	op.fromC[e] = v
	return v
}

// identFromC reports whether the name id comes from C/C++: it names C, or
// an object that comes from C/C++, or an instance of a generic function
// with a type argument that is not complete (slices.Sort(xs) with
// var xs []C.int).
func (op *opaquePass) identFromC(id *ast.Ident) bool {
	if inst, ok := op.info.Instances[id]; ok {
		for i := range inst.TypeArgs.Len() {
			if incomplete(inst.TypeArgs.At(i)) {
				return true
			}
		}
	}
	obj := op.info.Uses[id]
	return obj != nil && op.objectFromC(obj)
}

// litTypeOpaque reports whether the type of the composite literal lit is
// opaque: its own type, or, if it was left out, the type of the literal
// that lit is an element of.
func (op *opaquePass) litTypeOpaque(lit *ast.CompositeLit) bool {
	for lit != nil {
		if lit.Type != nil {
			// Also a named type over a C/C++ type: go/types has no type
			// for the literals left out in V{{1}}.
			return op.cValue(lit.Type)
		}
		lit = op.litParent[lit]
	}
	return false
}

// objectFromC reports whether the object obj comes from C/C++ (see
// Opaque). The package name of import "C" does too.
func (op *opaquePass) objectFromC(obj types.Object) bool {
	switch o := obj.(type) {
	case *types.PkgName:
		return o.Imported().Path() == "C"
	case *types.Var:
		obj = o.Origin()
	case *types.Func:
		obj = o.Origin()
	}
	if obj.Pkg() == nil {
		return false // predeclared
	}
	if obj.Pkg() != op.pkg {
		return incompleteObject(obj)
	}
	if v, ok := op.objC[obj]; ok {
		return v
	}
	op.objC[obj] = false // in progress, as in comesFromC
	v := false
	if d, ok := op.decls[obj]; ok {
		switch {
		case d.typ != nil:
			v = op.comesFromC(d.typ)
		case d.value:
			v = op.opaqueExpr(d.init)
		case d.yield != 0:
			v = op.yieldOpaque(d.init, d.yield)
		default:
			// A range variable, or one of several values: from a value
			// of a named type over a C/C++ type too (range v with
			// type V C.vec), which go/types can't take apart.
			v = op.cValue(d.init) || op.holdsC(d.init)
		}
	}
	op.objC[obj] = v
	return v
}

// core returns the underlying type of the expression e, or nil.
func (op *opaquePass) core(e ast.Expr) types.Type {
	tv, ok := op.info.Types[e]
	if !ok || tv.Type == nil {
		return nil
	}
	return types.Unalias(tv.Type).Underlying()
}

// yieldOpaque reports whether the range variable number idx (from 1) of a range over the func
// seq has a C/C++ type: the corresponding parameter of the yield function is a C/C++ type, or
// has a part that is (go/types left it invalid).
func (op *opaquePass) yieldOpaque(seq ast.Expr, idx int) bool {
	sig, _ := op.core(seq).(*types.Signature)
	if sig == nil || sig.Params().Len() != 1 {
		return false
	}
	ysig, _ := types.Unalias(sig.Params().At(0).Type()).Underlying().(*types.Signature)
	if ysig == nil || idx > ysig.Params().Len() {
		return false
	}
	t := ysig.Params().At(idx - 1).Type()
	return incomplete(t) || op.namedOverC(t)
}

// incompleteObject reports whether the type of obj is not complete: for a
// type name, the type it was declared as (the underlying type of a named
// type, the type of an alias).
func incompleteObject(obj types.Object) bool {
	switch o := obj.(type) {
	case *types.Var, *types.Const, *types.Func:
		return incomplete(o.Type())
	case *types.TypeName:
		switch t := o.Type().(type) {
		case *types.Named:
			return incomplete(t.Underlying())
		case *types.Alias:
			return incomplete(types.Unalias(t))
		}
	}
	return false
}

// incomplete reports whether the type t has a part that go/types left
// invalid (a C/C++ type, with FakeImportC): types.Typ[types.Invalid] itself,
// or a pointer, slice, array, map, channel, function, tuple, struct,
// interface or union type with such a part. A named type is complete unless
// one of its type arguments is not: codegen prints it by its name.
func incomplete(t types.Type) bool {
	switch t := t.(type) {
	case *types.Basic:
		return t.Kind() == types.Invalid
	case *types.Alias:
		return incomplete(types.Unalias(t))
	case *types.Named:
		for i := range t.TypeArgs().Len() {
			if incomplete(t.TypeArgs().At(i)) {
				return true
			}
		}
		return false
	case *types.Pointer:
		return incomplete(t.Elem())
	case *types.Slice:
		return incomplete(t.Elem())
	case *types.Array:
		return incomplete(t.Elem())
	case *types.Chan:
		return incomplete(t.Elem())
	case *types.Map:
		return incomplete(t.Key()) || incomplete(t.Elem())
	case *types.Signature:
		return incomplete(t.Params()) || incomplete(t.Results())
	case *types.Tuple:
		for i := range t.Len() {
			if incomplete(t.At(i).Type()) {
				return true
			}
		}
		return false
	case *types.Struct:
		for i := range t.NumFields() {
			if incomplete(t.Field(i).Type()) {
				return true
			}
		}
		return false
	case *types.Interface:
		for i := range t.NumExplicitMethods() {
			if incomplete(t.ExplicitMethod(i).Type()) {
				return true
			}
		}
		for i := range t.NumEmbeddeds() {
			if incomplete(t.EmbeddedType(i)) {
				return true
			}
		}
		return false
	case *types.Union:
		for i := range t.Len() {
			if incomplete(t.Term(i).Type()) {
				return true
			}
		}
		return false
	}
	return false // a type parameter
}

// exprChildren calls f for each operand of e: the expressions that e is
// made of, and for a function, struct or interface type, the types of its
// fields.
func exprChildren(e ast.Expr, f func(ast.Expr)) {
	each := func(es ...ast.Expr) {
		for _, x := range es {
			if x != nil {
				f(x)
			}
		}
	}
	fields := func(l *ast.FieldList) {
		if l == nil {
			return
		}
		for _, fld := range l.List {
			f(fld.Type)
		}
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		each(e.X)
	case *ast.SelectorExpr:
		each(e.X, e.Sel)
	case *ast.IndexExpr:
		each(e.X, e.Index)
	case *ast.IndexListExpr:
		each(e.X)
		each(e.Indices...)
	case *ast.SliceExpr:
		each(e.X, e.Low, e.High, e.Max)
	case *ast.TypeAssertExpr:
		each(e.X, e.Type)
	case *ast.CallExpr:
		each(e.Fun)
		each(e.Args...)
	case *ast.StarExpr:
		each(e.X)
	case *ast.UnaryExpr:
		each(e.X)
	case *ast.BinaryExpr:
		each(e.X, e.Y)
	case *ast.KeyValueExpr:
		each(e.Key, e.Value)
	case *ast.CompositeLit:
		each(e.Type)
		each(e.Elts...)
	case *ast.FuncLit:
		each(e.Type)
	case *ast.Ellipsis:
		each(e.Elt)
	case *ast.ArrayType:
		each(e.Len, e.Elt)
	case *ast.MapType:
		each(e.Key, e.Value)
	case *ast.ChanType:
		each(e.Value)
	case *ast.FuncType:
		fields(e.TypeParams)
		fields(e.Params)
		fields(e.Results)
	case *ast.StructType:
		fields(e.Fields)
	case *ast.InterfaceType:
		fields(e.Methods)
	}
}
