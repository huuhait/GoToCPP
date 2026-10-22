package lower

import (
	"go/ast"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// The C++ of the rows for the address of a variable (the decision is adapt.Decide's, with the
// errors of the rows).

func lowerScalarPtr(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	u := ast.Unparen(st.arg).(*ast.UnaryExpr)
	name := st.key.Name
	text := fg.g.srcText(u.X)
	if st.key.IsConst {
		quoted := lit(strconv.Quote(text))
		return templateCall("gp::cgo::scalar_in", []cppir.Node{typeName(name)}, st.x, quoted), true
	}
	param := fg.outParam(typeName(name))
	*st.outs = append(*st.outs, outWrap{
		fun:   &cppir.Name{Name: "gp::cgo::call_out", Args: []cppir.Node{typeName(name)}},
		args:  []cppir.Expr{st.x, lit(strconv.Quote(text))},
		param: param,
	})
	return cxxName(param.Name), true
}

func lowerVoidOut(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	t := fg.outPtrTarget(st.arg)
	if t == nil {
		return nil, false
	}
	param := fg.outParam(&cppir.Ptr{Elem: typeName("void")})
	*st.outs = append(*st.outs, outWrap{
		fun:   cxxName("gp::cgo::call_void_out"),
		args:  []cppir.Expr{fg.expr(t)},
		param: param,
	})
	return cxxName(param.Name), true
}
