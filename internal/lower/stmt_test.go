package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/huuhait/gocpp/internal/cppir"
)

// TestReads checks bind's check whether a value reads a target too early: by C++ name for a
// variable, never for _, and always for a target that is not a variable (Phase 03's a[i] and
// *p), so that its value can't be evaluated in the wrong order.
func TestReads(t *testing.T) {
	v := types.NewVar(token.NoPos, nil, "v", types.Typ[types.Int])
	id := ast.NewIdent("v")
	fg := &funcGen{
		g:     &gen{info: &tinfo{Info: &types.Info{Uses: map[*ast.Ident]types.Object{id: v}}}, names: map[types.Object]string{}},
		names: (&gen{}).newFuncNames(),
		temps: map[ast.Expr]string{},
	}
	fg.names.names[v] = "v"
	lit := &ast.BasicLit{Kind: token.INT, Value: "1"}
	elem := target{x: &cppir.Index{X: cxxName("a"), Index: cxxName("i")}}
	tests := []struct {
		e       ast.Expr
		targets []target
		want    bool
	}{
		{id, []target{varTarget("v")}, true},
		{id, []target{varTarget("w")}, false},
		{lit, []target{varTarget("v")}, false},
		{id, []target{{}}, false}, // _
		{lit, []target{elem}, true},
		{id, []target{{}, elem}, true},
	}
	for i, tt := range tests {
		if got := fg.reads(tt.e, tt.targets); got != tt.want {
			t.Errorf("%d: reads = %v, want %v", i, got, tt.want)
		}
	}
}
