package check

import (
	"go/token"
	"go/types"
	"sync"

	"github.com/huuhait/gocpp/internal/load"
)

// The hidden packages of Phase 10 (README §2.1 and §2.2), made in Go code
// like gocpp/cgoh (convert.go): no .gp file can import them (the loader
// rejects the paths), the rewrites add the imports.
//
// gocpp/must has the helpers that the type checker gives the calls with !.
// Go lets f(g()) pass all results of g to f, so the stock go/types infers
// every type of F!(x) once it is the call gp_must.M1(F(x)):
//
//	func M1[T any](v T, err error) T
//	func M2[A, B any](a A, b B, err error) (A, B)
//	func M3[A, B, C any](a A, b B, c C, err error) (A, B, C)
//	func Any(results ...any)
//	func Exit(err error)
//
// M1, M2 and M3 are for the calls that give their results (x := F!(a),
// a, b := F!(a), g(F!(a))), Any for a statement (F!(a)). The helpers exist
// only for the type checker: after the last check, the rewrite of
// mustlower.go turns each of them into real control flow, so no call of
// them is left in the C++. The one exception is Exit, which the lowering
// of func main!() and func init!() calls on purpose: it prints the error
// as "<program>: <error>" to stderr and exits with the code 1 (the
// lowerer emits gp::must_exit for it). ADR-124 (proposed: error return traces,
// steps 10.7b to 10.7e) adds a second one that stays: Hop(err, file, line,
// function), the first statement of each error branch, which the lowerer emits as
// gp::etrace::hop; then this comment and dropImports (the import stays while a
// Hop is left) change. None of them has a body: a call
// that is left (a check with errors) never gets to the lowerer.
//
// (The plan had a second hidden package, gocpp/macro, with the type of the use-site record of the macros:
// each use #util.Dbg(x) as util.Dbg(&gp_site_N, x). Step 10.9 does not use it: the place of a use goes in
// as constant arguments, docs/adr/075-user-macros-are-generic-functions.md. The path stays reserved.)

// The names of the functions of gocpp/must.
const (
	mustM1   = "M1"
	mustM2   = "M2"
	mustM3   = "M3"
	mustAny  = "Any"
	mustExit = "Exit"
)

// mustHelper returns the name of the helper of gocpp/must that gives n
// results (1 to 3), and mustAny for n == 0.
func mustHelper(n int) string {
	switch n {
	case 0:
		return mustAny
	case 1:
		return mustM1
	case 2:
		return mustM2
	}
	return mustM3
}

// mustArity returns how many results the helper gives: M1 1, M2 2, M3 3,
// Any 0, and -1 for another name.
func mustArity(name string) int {
	switch name {
	case mustAny:
		return 0
	case mustM1:
		return 1
	case mustM2:
		return 2
	case mustM3:
		return 3
	}
	return -1
}

// hiddenMust returns the hidden package, made once: a types.Package that is complete is never
// changed, so every Check, also at the same time, can use it.
var hiddenMust = sync.OnceValue(newMustPackage)

// newMustPackage makes the hidden package gocpp/must. Check makes it once,
// for all its packages.
func newMustPackage() *types.Package {
	pkg := types.NewPackage(load.MustPath, "must")
	anyType := types.Universe.Lookup("any").Type()
	errType := types.Universe.Lookup("error").Type()
	param := func(name string, t types.Type) *types.Var { return types.NewParam(token.NoPos, pkg, name, t) }
	tparams := func(names ...string) []*types.TypeParam {
		var tps []*types.TypeParam
		for _, n := range names {
			tps = append(tps, types.NewTypeParam(types.NewTypeName(token.NoPos, pkg, n, nil), anyType))
		}
		return tps
	}
	// generic makes func name[T0, T1, ...](v0 T0, v1 T1, ..., err error) (T0, T1, ...).
	generic := func(name string, names ...string) {
		tps := tparams(names...)
		var in, out []*types.Var
		for i, tp := range tps {
			in = append(in, param(string(rune('a'+i)), tp))
			out = append(out, param("", tp))
		}
		in = append(in, param("err", errType))
		sig := types.NewSignatureType(nil, nil, tps, types.NewTuple(in...), types.NewTuple(out...), false)
		pkg.Scope().Insert(types.NewFunc(token.NoPos, pkg, name, sig))
	}
	generic(mustM1, "T")
	generic(mustM2, "A", "B")
	generic(mustM3, "A", "B", "C")
	// func Any(results ...any)
	anySig := types.NewSignatureType(nil, nil, nil,
		types.NewTuple(param("results", types.NewSlice(anyType))), nil, true)
	pkg.Scope().Insert(types.NewFunc(token.NoPos, pkg, mustAny, anySig))
	// func Exit(err error)
	exitSig := types.NewSignatureType(nil, nil, nil, types.NewTuple(param("err", errType)), nil, false)
	pkg.Scope().Insert(types.NewFunc(token.NoPos, pkg, mustExit, exitSig))
	pkg.MarkComplete()
	return pkg
}
