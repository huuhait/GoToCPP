package lower

import (
	"go/ast"
	"go/types"
)

// Stores to the standard streams of package os (ADR-100). fmt.Print, Println and Printf write to
// descriptor 1 themselves until the program changes os.Stdout, and Scan, Scanln and Scanf read
// descriptor 0 until it changes os.Stdin (gocpp/fmt.hpp: std_gate). The lowering tells fmt when a
// program does:
//
//	os.Stdout = w          gp::fmt::store_std(gpkg_os::Stdout, w);       // an atomic store, then the gates
//	*os.Stdout = *w        gp::fmt::std_changed_after(*gpkg_os::Stdout = *w);
//
// The atomic store is what lets another goroutine print while the variable is swapped: fmt reads the
// variable through an atomic snapshot. Not covered: a store through &os.Stdout, a store into the File
// through another pointer to it, and reflection.

// stdFileVar reports whether e names the variable Stdin, Stdout or Stderr of package os.
func (fg *funcGen) stdFileVar(e ast.Expr) bool {
	var id *ast.Ident
	switch x := ast.Unparen(e).(type) {
	case *ast.SelectorExpr:
		if fg.g.info.Selections[x] != nil {
			return false
		}
		id = x.Sel
	case *ast.Ident:
		id = x
	default:
		return false
	}
	v, ok := fg.g.info.Uses[id].(*types.Var)
	if !ok || v.Pkg() == nil || v.Pkg().Path() != "os" || v.Parent() != v.Pkg().Scope() {
		return false
	}
	switch v.Name() {
	case "Stdin", "Stdout", "Stderr":
		return true
	}
	return false
}

// stdStoreKind says how a store to e tells fmt: stdVar for os.Stdin, os.Stdout and os.Stderr
// themselves (an atomic store, gp::fmt::store_std), stdDeref for what they point to (*os.Stdout = *w).
// It is decided where the target of any store is made (lhs), so that every form that assigns to a
// variable (a = b, a, b = x, y, a = <-ch in a select, for a = range, the init and post statements
// of a for) is covered.
type stdStoreKind int

const (
	stdNone stdStoreKind = iota
	stdVar
	stdDeref
)

func (fg *funcGen) stdStoreKind(e ast.Expr) stdStoreKind {
	e = ast.Unparen(e)
	if st, ok := e.(*ast.StarExpr); ok {
		if fg.stdFileVar(st.X) {
			return stdDeref
		}
		return stdNone
	}
	if fg.stdFileVar(e) {
		return stdVar
	}
	return stdNone
}
