package check

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/load"
)

// Slice expressions of C arrays (Phase 04 README §2.7, step 4.10): in cgo, a C array is sliced
// as a Go array is: rec.name[:] is a []C.char over C's memory. go/types gives a slice
// expression of an operand that has no type no type either, so
//
//	name := rec.name[:]    // rec: *C.rec; name has no type
//	for i := range name    // can't be a Go range over a slice
//
// would leave name an unknown C/C++ value, and len(name), name[i] and append(name, c) couldn't be
// checked or lowered as the gocpp slice that it is. So, like a conversion of an opaque value
// (convert.go), the slice expression a[lo:hi:max] is rewritten to a call
// gp_cgoh.SliceCLoHiMax(a, lo, hi, max) of a function whose result is the slice of the invalid
// type, []C.T (like var xs []C.int), and the package is checked again. The call has the operand
// and the bounds that the expression had (the omitted ones are left out), and the name of the
// function says which ones are there (SliceC, SliceCLo, SliceCHi, SliceCLoHi, SliceCHiMax,
// SliceCLoHiMax); Package.OpaqueSlice gives the expression back, for codegen, which writes
// gp::sub(gp::cgo::slice_of(a), lo, hi, max).
// Only an operand that is opaque (comes from C/C++) is rewritten; a slice of a pointer to an
// array, (*[16]C.char)(p)[:n], has a type already.

// sliceForms are the names of the helpers of gocpp/cgoh for the forms of a slice expression,
// by the bounds that it has (low, high, max).
var sliceForms = map[[3]bool]string{
	{false, false, false}: "SliceC",
	{true, false, false}:  "SliceCLo",
	{false, true, false}:  "SliceCHi",
	{true, true, false}:   "SliceCLoHi",
	{false, true, true}:   "SliceCHiMax",
	{true, true, true}:    "SliceCLoHiMax",
}

// A sliceCandidate is a slice expression whose operand has no type, and the field that holds it.
type sliceCandidate struct {
	slice *ast.SliceExpr
	file  *ast.File
	slot  *ast.Expr
}

// opaqueSlices returns the slice expressions in f, in source order, that have no type.
func opaqueSlices(f *ast.File, info *types.Info) []sliceCandidate {
	var cands []sliceCandidate
	ast.Inspect(f, func(n ast.Node) bool {
		for _, slot := range exprSlots(n) {
			s, ok := (*slot).(*ast.SliceExpr)
			if !ok {
				continue
			}
			if tv, ok := info.Types[s]; ok && isValidType(tv.Type) {
				continue
			}
			cands = append(cands, sliceCandidate{slice: s, file: f, slot: slot})
		}
		return true
	})
	return cands
}

// rewriteSlice rewrites the slice expression of c, if its operand is opaque and its bounds are
// not waiting for a rewrite of this round, and reports whether it did.
func (cv *converter) rewriteSlice(a *analysis, c sliceCandidate) bool {
	s := c.slice
	if a.errorIn(s) || a.state(s.X) != opaque {
		return false
	}
	args := []ast.Expr{s.X}
	for _, b := range []ast.Expr{s.Low, s.High, s.Max} {
		if b == nil {
			continue
		}
		if st := a.state(b); st != typed && st != opaque {
			return false // waits for the next round, or isn't C's doing
		}
		args = append(args, b)
	}
	pos := s.X.Pos()
	name := cv.importName(c.file)
	form := sliceForms[[3]bool{s.Low != nil, s.High != nil, s.Max != nil}]
	if form == "" {
		return false // (a[lo::max] is not Go)
	}
	*c.slot = &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{NamePos: pos, Name: name},
			Sel: &ast.Ident{NamePos: pos, Name: form},
		},
		Lparen: s.Lbrack,
		Args:   args,
		Rparen: s.End() - 1,
	}
	return true
}

// OpaqueSlice returns the slice expression a[lo:hi:max] that check rewrote to the call
// gp_cgoh.SliceCLoHiMax(a, lo, hi, max) (or the call of another form), or nil if call is not
// such a call. It is the same expression every time: findSlices made them at the end of the
// check.
func (p *Package) OpaqueSlice(call *ast.CallExpr) *ast.SliceExpr {
	if p == nil {
		return nil
	}
	return p.slices[call]
}

// findSlices makes the slice expressions of the calls of the helpers SliceC... in the files of
// lp, again after each check (and in the ASTs that an earlier Check rewrote).
func findSlices(lp *load.Package, info *types.Info) map[*ast.CallExpr]*ast.SliceExpr {
	var m map[*ast.CallExpr]*ast.SliceExpr
	for _, f := range lp.Files {
		if cgohImport(f.AST) == nil {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			if s := sliceOfCall(call, info); s != nil {
				if m == nil {
					m = make(map[*ast.CallExpr]*ast.SliceExpr)
				}
				m[call] = s
			}
			return true
		})
	}
	return m
}

// sliceOfCall returns the slice expression that the call of a helper SliceC... stands for.
func sliceOfCall(call *ast.CallExpr, info *types.Info) *ast.SliceExpr {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}
	if pn, ok := info.Uses[id].(*types.PkgName); !ok || pn.Imported().Path() != load.CgohPath {
		return nil
	}
	for bounds, name := range sliceForms {
		if name != sel.Sel.Name {
			continue
		}
		s := &ast.SliceExpr{X: call.Args[0], Lbrack: call.Lparen, Rbrack: call.Rparen, Slice3: bounds[2]}
		args := call.Args[1:]
		take := func(has bool) ast.Expr {
			if !has || len(args) == 0 {
				return nil
			}
			e := args[0]
			args = args[1:]
			return e
		}
		s.Low, s.High, s.Max = take(bounds[0]), take(bounds[1]), take(bounds[2])
		return s
	}
	return nil
}

// addSliceHelper declares the helpers func SliceC(a any, bounds ...any) []C.T (and the other
// forms) in the hidden package pkg.
func addSliceHelper(pkg *types.Package) {
	anyType := types.Universe.Lookup("any").Type()
	params := types.NewTuple(
		types.NewParam(token.NoPos, pkg, "a", anyType),
		types.NewParam(token.NoPos, pkg, "bounds", types.NewSlice(anyType)))
	results := types.NewTuple(types.NewParam(token.NoPos, pkg, "", types.NewSlice(types.Typ[types.Invalid])))
	for _, name := range sliceForms {
		pkg.Scope().Insert(types.NewFunc(token.NoPos, pkg, name, types.NewSignatureType(nil, nil, nil, params, results, true)))
	}
}
