package lower

import (
	"go/ast"
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/adapt"
)

// Go strings as arguments of C/C++ functions (Phase 12 step 12.6, README §2.5).
//
//	cstdio.puts(s)          ::puts(gp::cstr(s).gp_ptr())      a const char* parameter
//	cstdio.printf("%s", s)  ::printf("%s", gp::cstr(s).gp_ptr())   a variadic argument
//	takes(s)                takes(::std::string(s))           a std::string parameter
//
// In opaque mode a gocpp string goes to a std::string_view parameter by itself (gp::string
// converts to it), and anywhere else the user writes C.cstr(s) or std.string(s). Typed mode knows
// the parameters. The conversion is made only when **every** overload that has a parameter at
// that place has the same kind of parameter (a const char* or a std::string): then the opaque
// build would not compile the call, and the typed one cannot change what it means. A set that
// mixes kinds (f(const char*) next to f(std::string_view)) is left to C++'s overload
// resolution on the gocpp string, which prefers the string_view as in opaque mode. A function
// template, a function pointer variable and a name without declarations are left alone.
//
// The C string is gp::cstr's: a copy with a NUL at the end when the string is not
// NUL-terminated already, alive until the end of the statement that contains the call. A
// function that keeps the pointer longer (keeps.go) or returns a pointer into it, whose result is
// then stored, is a user error with the hint to use C.CString, as the opaque lowering asks for
// anyway. A NUL byte inside the string ends the C string there, as with C.CString and C.cstr.

// aliasUseOK reports whether the call c, of a function that returns a pointer into its argument,
// is used where the result is gone by the end of the statement: compared, passed on, or the
// statement itself. Assigned to a variable, stored or returned, it would outlive the string.
func (g *gen) aliasUseOK(c *ast.CallExpr) bool { return adapt.AliasUseOK(g.parentOf, c) }

// parentOf returns the node whose direct child is n (the package's syntax tree), or nil.
func (g *gen) parentOf(n ast.Node) ast.Node {
	if g.parents == nil {
		g.parents = make(map[ast.Node]ast.Node)
		for _, f := range g.pkg.Load.Files {
			var stack []ast.Node
			ast.Inspect(f.AST, func(x ast.Node) bool {
				if x == nil {
					stack = stack[:len(stack)-1]
					return true
				}
				if len(stack) > 0 {
					g.parents[x] = stack[len(stack)-1]
				}
				stack = append(stack, x)
				return true
			})
		}
	}
	return g.parents[n]
}

// calleeSigs returns the function types that the callee of c can be (all the overloads) and its
// name: for a free function, the overload set of its name; for a member of a C++ object, the
// methods of that name of its class and bases. false when a candidate has no signature (a
// template), or the callee is something else (a function pointer, a constructor).
func (g *gen) calleeSigs(c *ast.CallExpr) ([]adapt.Sig, string, bool) {
	rep := g.interop.typed.rep
	if r, ok := g.cRefOf(c.Fun); ok {
		n, ok := r.x.(*cppir.Name)
		if !ok || r.kind != interop.Value {
			return nil, "", false
		}
		name := strings.TrimPrefix(n.Name, "::")
		out, ok := adapt.FuncSigs(rep.Decls, name)
		return out, name, ok
	}
	// x.m(args): the methods m of the class of x.
	sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}
	out, ok := g.methodSigs(g.memberObject(sel), sel.Sel.Name)
	return out, sel.Sel.Name, ok
}

// methodSigs returns the function types of the methods called name of the class that xe is an
// object (or a pointer to one) of, with the methods of its bases; false when there is none, or
// one is a template.
func (g *gen) methodSigs(xe ast.Expr, name string) ([]adapt.Sig, bool) {
	v := g.tvalOf(xe)
	if v == nil {
		return nil, false
	}
	return adapt.MethodSigs(g.interop.typed.rep.Mapper, &v.Value, name)
}

// methodKeepsCharPtr reports whether the call c is a method of a class of the program (not the
// library's) that has a data member that is a char pointer: a setter such as set(const char*) is
// then likely to store the pointer, and nothing in its declaration says otherwise.
func (g *gen) methodKeepsCharPtr(c *ast.CallExpr, cs adapt.CharSet) (string, bool) {
	sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	v := g.tvalOf(g.memberObject(sel))
	if v == nil {
		return "", false
	}
	return adapt.MethodKeepsCharPtr(g.interop.typed.rep.Mapper, &v.Value, cs)
}
