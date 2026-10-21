package check

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/load"
)

// Extern functions (ADR-128): a function without a body whose doc comment has
// //gocpp:extern is defined in C or C++ and declared by the .gp file (internal/load/extern.go
// reads the directive). go/types checks the declaration like any function, but it knows nothing
// of C++, so this pass checks what the declaration has to satisfy for the C++ declaration that
// gocpp writes from it (lower/extern.go):
//
//   - every parameter and result has a type with one C++ spelling that does not depend on how
//     gocpp represents Go values: a C/C++ type (C.int, *C.char, windows.HWND, std.string),
//     unsafe.Pointer, or a number or bool type of Go with a size (int32, uint8, float64; not int
//     and uint, whose size C does not share: C.int is 32 bits, Go's int 64), or a pointer to
//     those. A Go string, slice, map, interface, func, channel, struct or named type is a gocpp
//     object, not a C value: convert it at the call (C.CString) or declare the C type;
//   - a result is not unsafe.Pointer and not a pointer to a number of Go: gocpp pointers are
//     managed objects that can not point into the memory of C (a parameter may be one: the
//     call passes the address of a Go variable);
//   - at most one result, and no variadic parameter (C varargs are not supported);
//   - the function is only called: a value like f := add3 would need a Go func value over a C
//     function, which has no representation;
//   - a function or method of a package of the project without a body and without the directive
//     is "missing function body" (go/types reports only a generic function, not a method or an
//     ordinary function); a bindgen view (ADR-092) keeps its bodyless functions, and
//     Config.AllowBodyless turns the error off for the runner of Go's own test files.

// checkExternFuncs reports the mistakes of the extern functions of lp, and the bodyless
// functions and methods of lp that are not extern (unless allowBodyless).
func checkExternFuncs(fset *token.FileSet, lp *load.Package, files []*ast.File, info *types.Info,
	allowBodyless bool, report func(pos token.Pos, err error)) {
	fail := func(pos token.Pos, msg string) {
		report(pos, types.Error{Fset: fset, Pos: pos, Msg: msg})
	}
	for _, f := range files {
		view := load.IsBindgenView(f)
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body != nil {
				continue
			}
			if load.HasExternDirective(fd) {
				if _, ok := load.ExternFunc(fd); ok {
					checkExternSignature(fd, info, fail)
				}
				continue
			}
			if !lp.Std && !view && !allowBodyless && fd.Type.TypeParams == nil {
				fail(fd.Name.Pos(), "missing function body (to declare a function that C or C++ defines, "+
					"put //gocpp:extern in its doc comment)")
			}
		}
	}
	externs := externNames(lp)
	if len(externs) == 0 {
		return
	}
	checkExternValues(files, info, externs, fail)
}

// checkExternSignature reports the parameters and results of the extern function fd whose types
// have no C++ spelling.
func checkExternSignature(fd *ast.FuncDecl, info *types.Info, fail func(pos token.Pos, msg string)) {
	name := fd.Name.Name
	if fd.Type.Params != nil {
		for _, field := range fd.Type.Params.List {
			if ell, ok := field.Type.(*ast.Ellipsis); ok {
				fail(ell.Pos(), "extern function "+name+" can not be variadic: C varargs are not supported "+
					"(declare a C wrapper with a fixed list of parameters, written in a C file of the package)")
				continue
			}
			if why := externTypeProblem(field.Type, info); why != "" {
				fail(field.Type.Pos(),
					"extern function "+name+": parameter type "+types.ExprString(field.Type)+" "+why)
			}
		}
	}
	if fd.Type.Results == nil {
		return
	}
	count := 0
	for _, field := range fd.Type.Results.List {
		count += max(len(field.Names), 1)
	}
	if count > 1 {
		fail(fd.Type.Results.Pos(), "extern function "+name+" returns more than one value: "+
			"a C function returns one (a struct, or a pointer parameter, for more)")
		return
	}
	for _, field := range fd.Type.Results.List {
		why := externTypeProblem(field.Type, info)
		if why == "" {
			why = externResultProblem(field.Type, info)
		}
		if why != "" {
			fail(field.Type.Pos(),
				"extern function "+name+": result type "+types.ExprString(field.Type)+" "+why)
		}
	}
}

// externResultProblem returns why the type expression e, which is fine as a parameter, can not
// be the result of an extern function: a gocpp pointer is a managed object, so a pointer that C
// returns (unsafe.Pointer, or a pointer to a number of Go) has no value to become in Go.
func externResultProblem(e ast.Expr, info *types.Info) string {
	base := ast.Unparen(e)
	stars := 0
	for {
		star, ok := base.(*ast.StarExpr)
		if !ok {
			break
		}
		base = ast.Unparen(star.X)
		stars++
	}
	const why = "can not be the result of a C function: a gocpp pointer is a managed object that can not " +
		"point into the memory of C (write a pointer to a C type, such as *C.char or *C.int, or let the " +
		"C function fill a pointer parameter)"
	switch x := base.(type) {
	case *ast.SelectorExpr:
		if id, ok := ast.Unparen(x.X).(*ast.Ident); ok && x.Sel.Name == "Pointer" {
			if pn, ok := info.Uses[id].(*types.PkgName); ok && pn.Imported().Path() == "unsafe" {
				return why
			}
		}
	case *ast.Ident:
		if tn, ok := info.Uses[x].(*types.TypeName); ok && tn.Parent() == types.Universe && stars > 0 {
			return why
		}
	}
	return ""
}

// externBasics are the predeclared types that an extern signature may use: they have the C++
// types int8_t ... uint64_t, bool, float and double (and uintptr_t). Not int and uint: Go's
// int is 64 bits where C's int is 32, a silent mismatch (externTypeProblem says so).
var externBasics = map[string]bool{
	"bool": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"float32": true, "float64": true, "byte": true, "rune": true,
}

// intCName is the C type that a programmer who writes the Go int or uint for a C function means.
func intCName(goName string) string {
	if goName == "uint" {
		return "unsigned int"
	}
	return "int"
}

// intAdvice lists the types that say how many bits the C function takes.
func intAdvice(goName string) string {
	if goName == "uint" {
		return "uint32, uint64, C.ulong or C.size_t"
	}
	return "C.int, int32, int64, C.long or C.longlong"
}

// noCxxType is the problem of a predeclared Go type that C++ has no type for.
const noCxxType = "is a Go type that C++ does not have (use a C type such as *C.char, or a number type of Go)"

// externTypeProblem returns why the type expression e can not be a parameter or result type of
// an extern function, or "" if it can.
func externTypeProblem(e ast.Expr, info *types.Info) string {
	switch x := ast.Unparen(e).(type) {
	case *ast.StarExpr:
		return externTypeProblem(x.X, info)
	case *ast.Ident:
		tn, isType := info.Uses[x].(*types.TypeName)
		switch {
		case isType && tn.Parent() == types.Universe && externBasics[tn.Name()]:
			return ""
		case isType && tn.Parent() == types.Universe && (tn.Name() == "int" || tn.Name() == "uint"):
			return "is the Go " + tn.Name() + " (64 bits), which is not the C " + intCName(tn.Name()) +
				" (32 bits): write " + intAdvice(tn.Name()) + ", as the C function declares"
		case isType && tn.Parent() == types.Universe:
			return noCxxType
		}
		return "is a type declared in Go: write the C or C++ type, or the number type it stands for"
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		root := ast.Unparen(e)
		for {
			switch r := root.(type) {
			case *ast.SelectorExpr:
				if id, ok := ast.Unparen(r.X).(*ast.Ident); ok {
					return externPackageProblem(id, r.Sel.Name, info)
				}
				root = ast.Unparen(r.X)
				continue
			case *ast.IndexExpr:
				root = ast.Unparen(r.X)
				continue
			case *ast.IndexListExpr:
				root = ast.Unparen(r.X)
				continue
			}
			break
		}
		return "is not a C or C++ type"
	}
	return noCxxType
}

// externPackageProblem judges the type pkg.name of an extern signature: a name of C (the
// loader's rewrite of header and namespace names, and C.int), or unsafe.Pointer.
func externPackageProblem(pkg *ast.Ident, name string, info *types.Info) string {
	pn, ok := info.Uses[pkg].(*types.PkgName)
	if !ok {
		return "is not a type of C or C++"
	}
	switch path := pn.Imported().Path(); {
	case path == "C":
		return ""
	case path == "unsafe" && name == "Pointer":
		return ""
	case path == "unsafe":
		return "is not a type of C or C++"
	}
	return "is a type of the Go package " + pn.Imported().Path() + ", which C++ does not have"
}

// ExternSet returns the extern functions that lp and its imports declare, as
// "<import path>.<name>" (nil for none), for the callers that look for the calls of one.
func ExternSet(lp *load.Package) map[string]bool { return externNames(lp) }

// IsExternFunc reports whether fn is in the set that ExternSet returned.
func IsExternFunc(set map[string]bool, fn *types.Func) bool {
	if fn == nil || fn.Pkg() == nil || fn.Parent() != fn.Pkg().Scope() {
		return false
	}
	return set[fn.Pkg().Path()+"."+fn.Name()]
}

// externNames returns the extern functions that lp and its imports declare, as
// "<import path>.<name>".
func externNames(lp *load.Package) map[string]bool {
	var out map[string]bool
	add := func(p *load.Package) {
		for _, f := range p.Files {
			for _, name := range f.Externs {
				if out == nil {
					out = make(map[string]bool)
				}
				out[p.ImportPath+"."+name] = true
			}
		}
	}
	add(lp)
	for _, p := range lp.Imports {
		add(p)
	}
	return out
}

// checkExternValues reports each use of an extern function that is not a call.
func checkExternValues(files []*ast.File, info *types.Info, externs map[string]bool,
	fail func(pos token.Pos, msg string)) {
	isExtern := func(id *ast.Ident) bool {
		fn, _ := info.Uses[id].(*types.Func)
		return IsExternFunc(externs, fn)
	}
	for _, f := range files {
		called := make(map[ast.Expr]bool)
		sels := make(map[*ast.Ident]*ast.SelectorExpr)
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				called[ast.Unparen(n.Fun)] = true
			case *ast.SelectorExpr:
				sels[n.Sel] = n
			case *ast.Ident:
				if !isExtern(n) {
					break
				}
				if sel := sels[n]; sel != nil {
					if !called[sel] {
						fail(sel.Pos(),
							"cannot use extern function "+types.ExprString(sel)+" as a value: "+onlyCall)
					}
				} else if !called[n] {
					fail(n.Pos(), "cannot use extern function "+n.Name+" as a value: "+onlyCall)
				}
			}
			return true
		})
	}
}

const onlyCall = "it is a C or C++ function, only a call is possible " +
	"(wrap it in a Go function to pass it on)"
