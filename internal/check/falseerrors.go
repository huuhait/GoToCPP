package check

import (
	"cmp"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"github.com/huuhait/gocpp/internal/diag"
)

// The false errors of go/types on opaque code (Phase 01 README §2.8).
//
// FakeImportC makes every C.X invalid, and go/types is silent about an
// operand whose mode is invalid (C.f(), a local x := C.f()). But some C/C++
// values reach it as an operand with a valid mode and an invalid type, and
// then it reports an error that the C++ compiler would not:
//   - a variable or a function of another package whose type is not
//     complete (lib.CB, with var CB = x.get_cb() in lib): -lib.CB,
//     *lib.CB, lib.CB[0], lib.CB(), range lib.Make(), lib.CB.(T);
//   - a composite literal of an opaque type: range (std.vector[int]{1, 2});
//     and the literals inside it whose type was left out, for which it has
//     no type: std.vector[std.vector[int]]{{1, 2}} ("missing type in
//     composite literal");
//   - a pointer to an invalid type: p.x with p := &C.Point{};
//   - a value or a type of a named type declared over a C/C++ type
//     (type Handle C.int, type V C.vec), or a pointer to one: its
//     underlying type is invalid, so h + 1, h == 0, -h, h++, h[0], range h,
//     make(V, n), h.(fmt.Stringer), Handle(n), pp.x and var h Handle = 3
//     are errors;
//   - such values as indexes and sizes (xs[h], xs[lib.CB:],
//     make([]int, h)) and as arguments of builtins (append(hs, 1) with
//     hs []Handle, delete(m, 1), copy(hs, ns), min(h, 1)).
//
// The collector drops such an error (falseErrors.drop) only when its code
// is one of the codes below, its place is the operand of the expression or
// statement that the code is about (a falseSlot; of the expressions that
// start there, the innermost one that go/types couldn't type, see drop),
// and that operand is a C value (opaquePass.cValue: opaque, or of a
// named type over a C/C++ type; for p.x also a pointer to one), or the
// type that a value is assigned to is a named type over a
// C/C++ type. The same code on Go values stays, and so does every other
// error. The expressions that go/types leaves without types then are
// opaque (opaquePass.cOperand), and the ones that it skips are checked
// like the other skipped expressions (skipCall, skipSlice, skipMapValue).
//
// Check drops them only in a package with C in it, and none whose imports
// have errors: there an invalid value can come from an error.
//
// A known limit: go/types reports no error that mentions an invalid
// operand or type ("invalid type" in the message, see followOnError) after
// its first error, so a dropped false error that came first still hides
// the later ones of that kind. They are about values without types, which
// in a package with no other error are C/C++ values (the cases above), so
// what is hidden is mostly more false errors.

// The go/types error codes of the false errors (Go's internal/types/errors;
// Go doesn't change the values).
const (
	invalidConstVal      = 14  // cannot convert 1 (untyped int constant) to type invalid type
	incompatibleAssign   = 23  // cannot use 3 (untyped int constant) as Handle value in ...
	undefinedOp          = 45  // operator - not defined on h
	mismatchedTypes      = 46  // n + h (mismatched types int and Handle)
	nonNumericIncDec     = 48  // h++ (non-numeric type Handle)
	invalidIndirection   = 50  // cannot indirect lib.CB
	nonIndexableOperand  = 51  // cannot index lib.CB
	invalidIndex         = 52  // index h (variable of invalid type type Handle) must be integer
	nonSliceableOperand  = 54  // cannot slice lib.CB
	invalidShiftCount    = 56  // shift count h must be integer
	invalidShiftOperand  = 57  // shifted operand h must be integer
	untypedLit           = 70  // missing type in composite literal
	missingFieldOrMethod = 76  // p.x undefined (type *invalid type has no field or method x)
	invalidCopy          = 86  // invalid copy: ... different element types Handle and int
	invalidMake          = 92  // cannot make V: type must be slice, map, or channel
	invalidAssert        = 94  // lib.CB (variable with invalid type) is not an interface
	invalidConversion    = 96  // cannot convert 3 (untyped int constant) to type Handle
	invalidRangeExpr     = 109 // cannot range over lib.CB (variable with invalid type)
	invalidTypeSwitch    = 116 // h (variable of invalid type type H) is not an interface
	invalidCall          = 127 // cannot call lib.CB (variable with invalid type)
	invalidMinMaxOperand = 150 // h (variable of invalid type type Handle) cannot be ordered

	invalidUnsafeSlice  = 134 // unsafe.Slice(argv[0], 1): argv[0] (variable with invalid type) is not a pointer
	invalidUnsafeString = 145 // unsafe.String(argv[0], 1): argv[0] (variable with invalid type) is not a pointer
)

// falseCodes holds the codes above.
var falseCodes = map[int]bool{
	invalidConstVal: true, incompatibleAssign: true, undefinedOp: true, mismatchedTypes: true,
	nonNumericIncDec: true, invalidIndirection: true, nonIndexableOperand: true,
	invalidIndex: true, nonSliceableOperand: true, invalidShiftCount: true,
	invalidShiftOperand: true, untypedLit: true, missingFieldOrMethod: true, invalidCopy: true,
	invalidMake: true, invalidAssert: true, invalidConversion: true, invalidRangeExpr: true,
	invalidTypeSwitch: true, invalidCall: true, invalidMinMaxOperand: true,
	invalidUnsafeSlice: true, invalidUnsafeString: true,
}

// A falseSlot is a place where go/types can report a false error: an error
// with the code at the place of an operand of node. The error is false
// when one of values is a C/C++ value (opaquePass.cValue), or one of
// targets is a named type over a C/C++ type. A container of such values
// ([]Handle) is no C/C++ value: it counts only through a target, where its
// element type matters (append, copy, delete, a map index, a send).
//
// node is the expression whose operation the error is about (h + 1 for the
// 1 of h + 1). It is nil for a statement or for a value assigned to a
// typed place (var h Handle = 3, f(3), []Handle{3}): there the operand is
// the value, and no other place of the same kind can start at it.
type falseSlot struct {
	code    int
	node    ast.Expr
	values  []ast.Expr
	targets []types.Type

	// ptr says that a pointer to a named type over a C/C++ type counts as
	// a C/C++ value too (only for p.x: see cPointer).
	ptr bool
}

// A falseErrors finds the false errors of one check of a package.
type falseErrors struct {
	pkg   *types.Package
	info  *types.Info
	files []*ast.File

	// op is an opaque pass over the check's Info, made with slots.
	op *opaquePass

	// slots maps the place of an operand to its slots; it is made on the
	// first error with one of the codes above.
	slots map[token.Pos][]falseSlot
}

func newFalseErrors(pkg *types.Package, info *types.Info, files []*ast.File) *falseErrors {
	return &falseErrors{pkg: pkg, info: info, files: files}
}

// drop reports whether err is a false error of go/types on opaque code.
//
// Several expressions can start at the place of the error: in n + s == h,
// both n + s and the comparison start at n, and an error about n + s
// (mismatched types int and string) is at n. So drop looks at the slots of
// the error's place and code from the innermost expression outward, skips
// the expressions that go/types typed (their operation went well: in
// n + m == h with int n and m, n + m is an int, and the error is about the
// comparison), and decides on the first one left. The slots without an
// expression come last.
func (f *falseErrors) drop(err types.Error) bool {
	code := diag.ErrorCode(err)
	if !falseCodes[code] {
		return false
	}
	if f.slots == nil {
		// Only now: a package without such errors (all Go code) needs no
		// opaque pass here.
		f.op = newOpaquePass(f.pkg, f.info, f.files)
		f.slots = make(map[token.Pos][]falseSlot)
		for _, file := range f.files {
			f.collect(file)
		}
	}
	var cands []falseSlot
	for _, s := range f.slots[err.Pos] {
		if s.code == code {
			cands = append(cands, s)
		}
	}
	slices.SortStableFunc(cands, func(a, b falseSlot) int {
		if a.node == nil || b.node == nil {
			return cmp.Compare(boolInt(a.node == nil), boolInt(b.node == nil))
		}
		return cmp.Compare(a.node.End(), b.node.End()) // all start at err.Pos
	})
	for _, s := range cands {
		if s.node != nil && f.typed(s.node) {
			continue
		}
		return f.isFalse(s)
	}
	return false
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// typed reports whether go/types recorded a valid type for e.
func (f *falseErrors) typed(e ast.Expr) bool {
	tv, ok := f.info.Types[e]
	return ok && tv.Type != nil && tv.Type != types.Typ[types.Invalid]
}

// isFalse reports whether an error of the slot s is false.
func (f *falseErrors) isFalse(s falseSlot) bool {
	for _, t := range s.targets {
		if t != nil && f.op.namedOverC(t) {
			return true
		}
	}
	for _, v := range s.values {
		if f.op.cValue(v) || s.ptr && f.op.cPointer(v) {
			return true
		}
	}
	return false
}

// add records a slot of the operation node for the errors with the code at
// the operand at.
func (f *falseErrors) add(at ast.Expr, code int, node ast.Expr, values ...ast.Expr) {
	if at != nil {
		s := falseSlot{code: code, node: node, values: values}
		f.slots[at.Pos()] = append(f.slots[at.Pos()], s)
	}
}

// addAt records a slot without an expression (see falseSlot) for the
// errors with the code at the operand at.
func (f *falseErrors) addAt(at ast.Expr, code int, targets []types.Type, values ...ast.Expr) {
	if at != nil {
		s := falseSlot{code: code, values: values, targets: targets}
		f.slots[at.Pos()] = append(f.slots[at.Pos()], s)
	}
}

// assign records the slot of the value v assigned to the type target
// (incompatibleAssign).
func (f *falseErrors) assign(v ast.Expr, target types.Type) {
	f.addAt(v, incompatibleAssign, []types.Type{target}, v)
}

// typeOf returns the type that go/types recorded for e, or nil.
func (f *falseErrors) typeOf(e ast.Expr) types.Type {
	if e == nil {
		return nil
	}
	return f.info.Types[e].Type
}

// underOf returns the underlying type of the type that go/types recorded
// for e, or nil.
func (f *falseErrors) underOf(e ast.Expr) types.Type {
	if t := f.typeOf(e); t != nil {
		return t.Underlying()
	}
	return nil
}

// elemOf returns the element type of the slice that e is, or nil.
func (f *falseErrors) elemOf(e ast.Expr) types.Type {
	if s, ok := f.underOf(e).(*types.Slice); ok {
		return s.Elem()
	}
	return nil
}

// collect records the slots of a file.
func (f *falseErrors) collect(file *ast.File) {
	// sigs are the signatures of the functions around a node, for the
	// values of return statements.
	var sigs []*types.Signature
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			switch top.(type) {
			case *ast.FuncDecl, *ast.FuncLit:
				sigs = sigs[:len(sigs)-1]
			}
			return true
		}
		stack = append(stack, n)
		switch n := n.(type) {
		case *ast.FuncDecl:
			var sig *types.Signature
			if obj := f.info.Defs[n.Name]; obj != nil {
				sig, _ = obj.Type().(*types.Signature)
			}
			sigs = append(sigs, sig)
		case *ast.FuncLit:
			sig, _ := f.typeOf(n).(*types.Signature)
			sigs = append(sigs, sig)
		case *ast.ReturnStmt:
			if len(sigs) > 0 && sigs[len(sigs)-1] != nil {
				res := sigs[len(sigs)-1].Results()
				if res.Len() == len(n.Results) {
					for i, r := range n.Results {
						f.assign(r, res.At(i).Type())
					}
				}
			}
		default:
			f.collectNode(n)
		}
		return true
	})
}

// collectNode records the slots of the node n (not a function or a return
// statement).
func (f *falseErrors) collectNode(n ast.Node) {
	switch n := n.(type) {
	case *ast.UnaryExpr:
		f.add(n.X, undefinedOp, n, n.X)
	case *ast.BinaryExpr:
		for _, code := range []int{undefinedOp, mismatchedTypes, invalidShiftOperand} {
			f.add(n.X, code, n, n.X, n.Y)
		}
		f.add(n.Y, mismatchedTypes, n, n.X, n.Y) // a comparison: h == nil
		f.add(n.Y, invalidShiftCount, n, n.Y)
		f.add(n.X, invalidConstVal, n, n.Y)
		f.add(n.Y, invalidConstVal, n, n.X)
	case *ast.AssignStmt:
		f.collectAssign(n)
	case *ast.IncDecStmt:
		f.addAt(n.X, nonNumericIncDec, nil, n.X)
	case *ast.StarExpr:
		f.add(n.X, invalidIndirection, n, n.X)
	case *ast.SelectorExpr:
		// p.x with p := &C.Point{} (a pointer to an invalid type), or
		// pp.x with pp *P and type P C.Point.
		s := falseSlot{code: missingFieldOrMethod, node: n, values: []ast.Expr{n.X}, ptr: true}
		f.slots[n.Sel.Pos()] = append(f.slots[n.Sel.Pos()], s)
	case *ast.IndexExpr:
		f.add(n.X, nonIndexableOperand, n, n.X)
		f.addAt(n.Index, invalidIndex, nil, n.Index) // xs[h]
		if m, ok := f.underOf(n.X).(*types.Map); ok {
			f.assign(n.Index, m.Key()) // m[3]
		}
	case *ast.IndexListExpr:
		f.add(n.X, nonIndexableOperand, n, n.X)
	case *ast.SliceExpr:
		f.add(n.X, nonSliceableOperand, n, n.X)
		for _, e := range []ast.Expr{n.Low, n.High, n.Max} {
			f.addAt(e, invalidIndex, nil, e) // xs[h:]
		}
	case *ast.TypeAssertExpr:
		f.add(n.X, invalidAssert, n, n.X)
		f.addAt(n.X, invalidTypeSwitch, nil, n.X) // the x.(type) of a type switch
	case *ast.RangeStmt:
		f.addAt(n.X, invalidRangeExpr, nil, n.X)
	case *ast.CallExpr:
		f.collectCall(n)
	case *ast.CompositeLit:
		f.collectLit(n)
	case *ast.SwitchStmt:
		if n.Tag != nil {
			for _, stmt := range n.Body.List {
				if cc, ok := stmt.(*ast.CaseClause); ok {
					for _, e := range cc.List {
						f.addAt(e, invalidConstVal, nil, n.Tag)
					}
				}
			}
		}
	case *ast.SendStmt:
		if ch, ok := f.underOf(n.Chan).(*types.Chan); ok {
			f.assign(n.Value, ch.Elem())
		}
	case *ast.ValueSpec:
		if typ := f.typeOf(n.Type); typ != nil {
			for _, v := range n.Values {
				f.assign(v, typ)
			}
		}
	}
}

// collectAssign records the slots of an assignment: the values assigned
// to typed places (x = 3), and an assignment operation (h += 2) like the
// binary expression it stands for.
func (f *falseErrors) collectAssign(s *ast.AssignStmt) {
	switch s.Tok {
	case token.DEFINE:
	case token.ASSIGN:
		if len(s.Lhs) == len(s.Rhs) {
			for i, r := range s.Rhs {
				f.assign(r, f.typeOf(s.Lhs[i]))
			}
		}
	default:
		if len(s.Lhs) == 1 && len(s.Rhs) == 1 {
			x, y := s.Lhs[0], s.Rhs[0]
			for _, code := range []int{undefinedOp, mismatchedTypes, invalidShiftOperand} {
				f.addAt(x, code, nil, x, y)
			}
			f.addAt(y, invalidShiftCount, nil, y)
			f.addAt(y, invalidConstVal, nil, x)
		}
	}
}

// collectCall records the slots of a call: the function, the arguments of
// builtins, the argument of a conversion, and the arguments assigned to
// the parameters.
func (f *falseErrors) collectCall(call *ast.CallExpr) {
	if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		if b, ok := f.info.Uses[id].(*types.Builtin); ok {
			f.collectBuiltin(call, b.Name())
			return
		}
	}
	if name := unsafeCall(call, f.info); name != "" && len(call.Args) > 0 {
		// unsafe.Slice(argv[0], n) with argv[0] of an invalid type (a C pointer of a slice
		// that unsafe.Slice made): the first argument is the C value.
		code := map[string]int{"Slice": invalidUnsafeSlice, "String": invalidUnsafeString}[name]
		if code != 0 {
			f.addAt(call.Args[0], code, nil, call.Args[0])
		}
		return
	}
	tv := f.info.Types[call.Fun]
	if tv.IsType() {
		// A conversion to or from a C/C++ value: Handle(3), string(h).
		if len(call.Args) == 1 {
			f.add(call.Args[0], invalidConversion, call, call.Fun, call.Args[0])
		}
		return
	}
	f.add(call.Fun, invalidCall, call, call.Fun)
	sig, ok := tv.Type.(*types.Signature)
	if !ok || call.Ellipsis.IsValid() {
		return
	}
	params := sig.Params()
	for i, arg := range call.Args {
		var typ types.Type
		switch {
		case sig.Variadic() && i >= params.Len()-1:
			if s, ok := params.At(params.Len() - 1).Type().(*types.Slice); ok {
				typ = s.Elem()
			}
		case i < params.Len():
			typ = params.At(i).Type()
		}
		f.assign(arg, typ)
	}
}

// collectBuiltin records the slots of a call of the builtin name.
func (f *falseErrors) collectBuiltin(call *ast.CallExpr, name string) {
	args := call.Args
	switch name {
	case "make":
		if len(args) > 0 {
			f.add(args[0], invalidMake, call, args[0])
			for _, n := range args[1:] {
				f.addAt(n, invalidIndex, nil, n) // make([]int, h)
			}
		}
	case "append":
		if len(args) > 1 && !call.Ellipsis.IsValid() {
			elem := f.elemOf(args[0])
			for _, a := range args[1:] {
				f.assign(a, elem) // append(hs, 1) with hs []Handle
			}
		}
	case "delete":
		if len(args) == 2 {
			if m, ok := f.underOf(args[0]).(*types.Map); ok {
				f.assign(args[1], m.Key())
			}
		}
	case "copy":
		if len(args) == 2 {
			f.addAt(args[0], invalidCopy, []types.Type{f.elemOf(args[0]), f.elemOf(args[1])})
		}
	case "min", "max":
		for _, a := range args {
			f.addAt(a, invalidMinMaxOperand, nil, a)
		}
	}
}

// collectLit records the slots of a composite literal: a literal whose
// type was left out, inside a literal of a C/C++ type, and the elements of
// a literal assigned to its element or field types.
func (f *falseErrors) collectLit(lit *ast.CompositeLit) {
	if lit.Type == nil {
		// The type of the nearest literal around it that has one.
		outer := lit
		for outer != nil && outer.Type == nil {
			outer = f.op.litParent[outer]
		}
		if outer != nil {
			f.slots[lit.Lbrace] = append(f.slots[lit.Lbrace],
				falseSlot{code: untypedLit, node: lit, values: []ast.Expr{outer.Type}})
		}
	}
	typ := f.typeOf(lit)
	if typ == nil {
		return
	}
	switch u := typ.Underlying().(type) {
	case *types.Slice:
		f.litElems(lit, nil, u.Elem())
	case *types.Array:
		f.litElems(lit, nil, u.Elem())
	case *types.Map:
		f.litElems(lit, u.Key(), u.Elem())
	case *types.Struct:
		for i, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if id, ok := kv.Key.(*ast.Ident); ok {
					if v, ok := f.info.Uses[id].(*types.Var); ok && v.IsField() {
						f.assign(kv.Value, v.Type())
					}
				}
			} else if i < u.NumFields() {
				f.assign(elt, u.Field(i).Type())
			}
		}
	}
}

// litElems records the slots of the elements of a slice, array or map
// literal: each value is assigned to elem, and each key of a map to key.
func (f *falseErrors) litElems(lit *ast.CompositeLit, key, elem types.Type) {
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key != nil {
				f.assign(kv.Key, key)
			}
			elt = kv.Value
		}
		f.assign(elt, elem)
	}
}

// cValue reports whether e is a C/C++ value for the false errors: it is
// opaque, or its own type is a named type over a C/C++ type (h with
// var h Handle and type Handle C.int, or the type V itself with
// type V C.vec). Only then is an operation on e the C++ compiler's to
// check: a []Handle or a *P is a Go slice or pointer (hs++ and pp + 1 are
// errors).
func (op *opaquePass) cValue(e ast.Expr) bool {
	if e == nil {
		return false
	}
	if op.opaqueExpr(e) {
		return true
	}
	tv, ok := op.info.Types[e]
	return ok && tv.Type != nil && op.namedOverC(tv.Type)
}

// cPointer reports whether e is a C/C++ value (cValue) or a pointer to a
// named type over a C/C++ type: go/types can't select pp.x with var pp *P
// and type P C.Point either.
func (op *opaquePass) cPointer(e ast.Expr) bool {
	if op.cValue(e) {
		return true
	}
	tv, ok := op.info.Types[e]
	if !ok || tv.Type == nil {
		return false
	}
	p, ok := types.Unalias(tv.Type).(*types.Pointer)
	return ok && op.namedOverC(p.Elem())
}

// cNamed reports whether go/types recorded for e a named type over a C/C++
// type (namedOverC), or a pointer, slice, array, map or channel of one: h
// with var h Handle and type Handle C.int, m in delete(m, 1) with
// m map[Handle]int. Only the labels of the opaque pass use it, for the
// arguments of delete and copy (cOperand); whether an error is false asks
// the narrower cValue.
func (op *opaquePass) cNamed(e ast.Expr) bool {
	if e == nil {
		return false
	}
	tv, ok := op.info.Types[e]
	if !ok || tv.Type == nil {
		return false
	}
	t := tv.Type
	switch u := types.Unalias(t).(type) {
	case *types.Pointer:
		t = u.Elem()
	case *types.Slice:
		t = u.Elem()
	case *types.Array:
		t = u.Elem()
	case *types.Chan:
		t = u.Elem()
	case *types.Map:
		if op.namedOverC(u.Key()) {
			return true
		}
		t = u.Elem()
	}
	return op.namedOverC(t)
}

// namedOverC reports whether t is a named type declared directly over a
// C/C++ type (type Handle C.int, type P C.Point, also of another package):
// its underlying type is invalid, so go/types can do nothing with its
// values, and its declaration comes from C/C++ (type V Undefined, invalid
// because of an error, doesn't count).
func (op *opaquePass) namedOverC(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok || n.Underlying() != types.Typ[types.Invalid] {
		return false
	}
	return op.objectFromC(n.Obj())
}
