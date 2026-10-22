package lower

import "github.com/huuhait/gocpp/internal/cppir"

// The C++ of the string rows (the decision is adapt.Decide's, with the errors of the callee that
// keeps the pointer and of the format string).

func lowerCStr(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	return &cppir.Call{Fun: &cppir.Member{X: call("gp::cstr", st.x), Name: "gp_ptr"}}, true
}

func lowerStdString(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	return &cppir.Call{Fun: cxxName("::std::string"), Args: []cppir.Expr{st.x}}, true
}

func lowerWide(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	fun := &cppir.Name{Name: "gp::wide_cstr", Args: []cppir.Node{typeName(st.key.Name)}}
	holder := &cppir.Call{Fun: fun, Args: []cppir.Expr{st.x}}
	return &cppir.Call{Fun: &cppir.Member{X: holder, Name: "gp_ptr"}}, true
}
