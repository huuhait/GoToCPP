package lower

import (
	"github.com/huuhait/gocpp/internal/cppir"
)

// The C++ of the rows for numbers by value (the decision is adapt.Decide's: the range of an enum
// constant and the representability of a number constant are errors there).

func lowerEnumConst(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	return &cppir.Cast{Kind: "static_cast", Type: typeName("::" + st.key.Name), X: st.x}, true
}

func lowerFloatConv(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	return templateCall("gp::cgo::as", []cppir.Node{typeName(st.key.Name)}, st.x), true
}
