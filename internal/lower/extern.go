package lower

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
)

// Extern functions (ADR-128, internal/load/extern.go). A function without a body and with a
// //gocpp:extern directive is defined in C or C++:
//
//	//gocpp:extern C
//	func add3(a, b, c C.int) C.int          extern "C" int add3(int, int, int);
//
//	//gocpp:extern ImGui::Handler
//	func handler(w windows.HWND) windows.LRESULT
//	                                        namespace ImGui { LRESULT Handler(HWND); }
//
// The declaration is written in the package's header, in the global namespace, after the
// package's C/C++ headers (headerNeedsC), so the units of the package and of every package
// that includes the header see it; it has the C++ types of the signature, as spelled by
// externType, so that the function's C++ name (which for C++ linkage has the parameter types in
// it) is the one the definition has. A call is a call of the C++ name, lowered like the call of
// a function of a header package (cCall): the arguments are C values (an untyped constant is a C
// literal, a gocpp pointer is the raw address).
//
// Two declarations of one function (a package declares it, or two packages do) must agree: the
// program's Program checks the spelled declarations.

// An externInfo is an extern function of the package.
type externInfo struct {
	decl   *ast.FuncDecl
	fn     *types.Func
	ext    load.ExternDecl
	result cppir.Type
	params []cppir.Type

	// spelled is the declaration as C++ text (the result, then the parameters), for the
	// comparison of two declarations and for the messages.
	spelled string
	paramsS string
}

// An externRecord is a declaration of the program that later ones are compared with.
type externRecord struct {
	info *externInfo
	g    *gen
}

// externDecl returns the extern declaration that the function obj (of any package of the
// program) was declared with.
func (g *gen) externDecl(obj types.Object) (load.ExternDecl, bool) {
	fn, ok := obj.(*types.Func)
	if !ok || g.prog == nil {
		return load.ExternDecl{}, false
	}
	ext, ok := g.prog.externFuncs[fn]
	return ext, ok
}

// externRef returns the C++ name of the call target e if it is an extern function: f in the
// package that declares it, pkg.F in another one.
func (g *gen) externRef(e ast.Expr) (cRef, bool) {
	var id *ast.Ident
	switch x := e.(type) {
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		id = x.Sel
	default:
		return cRef{}, false
	}
	if g.prog == nil || len(g.prog.externFuncs) == 0 {
		return cRef{}, false
	}
	ext, ok := g.externDecl(g.info.Uses[id])
	if !ok {
		return cRef{}, false
	}
	return cRef{x: g.cName(ext.Qualified(), id.Pos()), kind: interop.Value, src: g.srcText(e)}, true
}

// collectExtern handles a function with a //gocpp:extern directive: it is called by its C++
// name from every package, and the header declares it.
func (g *gen) collectExtern(d *ast.FuncDecl, ext load.ExternDecl) {
	fn, _ := g.info.Defs[d.Name].(*types.Func)
	if fn == nil {
		g.errorf(d.Pos(), "internal error: no object for func %s", d.Name.Name)
		return
	}
	info := &externInfo{decl: d, fn: fn, ext: ext, result: typeName("void")}
	if d.Type.Params != nil {
		for _, field := range d.Type.Params.List {
			t := g.externType(field.Type)
			for range max(len(field.Names), 1) {
				info.params = append(info.params, t)
			}
		}
	}
	if d.Type.Results != nil && len(d.Type.Results.List) == 1 {
		info.result = g.externType(d.Type.Results.List[0].Type)
	}
	var texts []string
	for _, p := range info.params {
		texts = append(texts, cppir.TypeString(p))
	}
	info.paramsS = strings.Join(texts, ", ")
	info.spelled = cppir.TypeString(info.result) + " " + ext.Name + "(" + info.paramsS + ")"
	g.names[fn] = ext.Qualified()
	g.externs = append(g.externs, info)
	g.prog.noteExtern(g, info)
}

// externType returns the C++ type of the type expression e of an extern function's signature
// (internal/check/externfuncs.go lets only these through): the C type as the C++ compiler
// names it, a pointer as a raw pointer (not gp::cptr, which has another C++ type and so another
// mangled name), unsafe.Pointer as void*, a number as the fixed-width type of Go's number.
func (g *gen) externType(e ast.Expr) cppir.Type {
	switch x := ast.Unparen(e).(type) {
	case *ast.StarExpr:
		return &cppir.Ptr{Elem: g.externType(x.X)}
	case *ast.SelectorExpr:
		if tv, ok := g.info.Types[x]; ok && tv.IsType() && isUnsafePointer(tv.Type) {
			return &cppir.Ptr{Elem: typeName("void")}
		}
	}
	g.inTemplate++ // (raw pointers inside, and the package's types fully qualified)
	defer func() { g.inTemplate-- }()
	return g.typeExpr(e)
}

// externDecls returns the declarations of the package's extern functions for the header.
func (g *gen) externDecls() []cppir.Decl {
	var out []cppir.Decl
	for _, x := range g.externs {
		name := x.ext.Name
		var scope []string
		if i := strings.LastIndex(name, "::"); i >= 0 {
			scope = strings.Split(name[:i], "::")
			name = name[i+2:]
		}
		f := &cppir.Func{Result: x.result, Name: name}
		if x.ext.CLinkage {
			f.Specs = []string{`extern "C"`}
		}
		for _, p := range x.params {
			f.Params = append(f.Params, cppir.Param{Type: p})
		}
		var d cppir.Decl = f
		for i := len(scope) - 1; i >= 0; i-- {
			d = &cppir.Namespace{Name: scope[i], Decls: []cppir.Decl{d}}
		}
		out = append(out, d)
	}
	if len(out) > 0 {
		out = append(out, &cppir.Blank{})
	}
	return out
}

// noteExtern registers the extern function x of g with the program and reports it when it
// disagrees with an earlier declaration of the same C++ function.
func (p *Program) noteExtern(g *gen, x *externInfo) {
	if p.externFuncs == nil {
		p.externFuncs = make(map[*types.Func]load.ExternDecl)
		p.externSeen = make(map[string][]externRecord)
	}
	p.externFuncs[x.fn] = x.ext
	for _, old := range p.externSeen[x.ext.Name] {
		if msg := externClash(old.info, x); msg != "" {
			other := g.fset.Position(old.info.decl.Name.Pos())
			g.userError(x.decl.Name.Pos(), "%s; the other declaration is at %s", msg, other)
			break
		}
	}
	p.externSeen[x.ext.Name] = append(p.externSeen[x.ext.Name], externRecord{info: x, g: g})
}

// externClash returns why the declarations a and b of one C++ name can not both be right, or "".
// Two C declarations are one function: they must be identical. C++ functions are the same
// function when their parameters are, and then their results must be; and a C function and a
// C++ function of the same parameters clash in C++ (conflicting declaration of C function).
func externClash(a, b *externInfo) string {
	msg := externClashMessage(a, b)
	if msg != "" && a.spelled != b.spelled && sameSpelledTypes(a.spelled, b.spelled) {
		msg += " (int32_t is int here, and uint32_t is unsigned int, but the check compares the " +
			"spelling: write both declarations alike)"
	}
	return msg
}

// sameSpelledTypes reports whether the C++ signatures a and b are the same once int32_t is int
// and uint32_t is unsigned int, which they are on every target of gocpp.
func sameSpelledTypes(a, b string) bool {
	canon := strings.NewReplacer("uint32_t", "unsigned int", "int32_t", "int")
	return canon.Replace(a) == canon.Replace(b)
}

// externClashMessage is the error of externClash without the hint.
func externClashMessage(a, b *externInfo) string {
	name := b.ext.Name
	sameParams := a.paramsS == b.paramsS
	sameResult := cppir.TypeString(a.result) == cppir.TypeString(b.result)
	switch {
	case a.ext.CLinkage && b.ext.CLinkage && !(sameParams && sameResult):
		return "extern function " + name + " is declared with the C signature " + b.spelled +
			", and elsewhere with another one, " + a.spelled
	case a.ext.CLinkage != b.ext.CLinkage && sameParams:
		return "extern function " + name + " is declared with C linkage and with C++ linkage, " +
			"with the same parameters (" + a.spelled + ")"
	case !a.ext.CLinkage && !b.ext.CLinkage && sameParams && !sameResult:
		return "extern function " + name + " is declared with the result " + cppir.TypeString(b.result) +
			", and elsewhere with the result " + cppir.TypeString(a.result) + " for the same parameters"
	}
	return ""
}
