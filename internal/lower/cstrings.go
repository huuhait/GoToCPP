package lower

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
)

// Strings at the C/C++ boundary (Phase 04 README §2.6, step 4.9):
//
//	C.CString(s)                  gp::cgo::CString(s)            (C's malloc; C.free frees it)
//	C.GoString(p), C.GoStringN(p, n), C.GoBytes(p, n)
//	                              gp::cgo::GoString(p), ...      (gocpp results: string, []byte)
//	C.free(p)                     gp::cgo::free(p)
//	cstdio.puts(C.cstr(s))        ::puts(gp::cstr(s).gp_ptr())   (the holder lives to the end of
//	                                                              the statement)
//	name := C.cstr(s)             auto name = gp::cstr(s);       (a holder variable)
//	cstdio.printf("%s", name)     ::printf("%s", name.gp_ptr())
//	string(x) (x a std::string)   gp::cgo::as<gp::string>(x)     (a copy of the bytes)
//
// check rewrote the helpers that the user wrote into calls of the hidden package gocpp/cgoh
// with real declarations (check.Package.CgoHelper), so their arguments and results have Go's
// types, as in cgo.
//
// A holder (gp::cstr_holder) is a C++ object: through a C function's `...` (printf), C++ would
// pass the object itself, which g++ (with -Werror=conditionally-supported) and clang reject. In
// opaque mode gocpp can't know which parameters are variadic, so every argument of a C/C++
// function that is a holder gets .gp_ptr(), its const char*: a C.cstr(s) call or a holder
// variable. The evaluation-order pass moves the call C.cstr(s) into a temporary when it must
// (order.go), never the raw pointer: gp_ptr is called where the argument is used, and the
// temporary lives as long as the statement's block.
//
// C.cstr(s) is allowed only where its holder lives as long as the C string is used: as an
// argument of a C/C++ function (also a constructor, a member or a C++ callable), and as the value
// of a variable that the holder is then (name := C.cstr(s), var name = C.cstr(s), and name =
// C.cstr(t) for such a variable). Anywhere else (a result, a field of a C struct, a
// variable of type *C.char) the C string would dangle after the statement: a user error that
// says to use C.CString. gocpp can't see what a C/C++ function or constructor does with the
// pointer, so one that keeps it after the statement dangles too: a C API that stores it, or
// std.string_view(C.cstr(s)), a view of the holder's bytes (a use after scope at its next use).
// That is C++'s rule for C/C++ values (README §2.7): std.string(s) makes a copy, and C.CString
// gives C a copy that lives until C.free.

// cgoHelper returns the name of the cgo helper that c calls (GoString), or "".
func (g *gen) cgoHelper(c *ast.CallExpr) string {
	return g.pkg.CgoHelper(c)
}

// cgoHelperCall returns the C++ of the call c of the cgo helper name.
func (fg *funcGen) cgoHelperCall(c *ast.CallExpr, name string) cppir.Expr {
	if c.Ellipsis.IsValid() {
		fg.g.unsupported(c.Ellipsis, "... in a call of C."+name)
		return placeholder
	}
	switch name {
	case "CString", "cstr", "wstr", "WString":
		// A gocpp string (or an opaque value, which the C++ compiler checks).
		fun := "gp::cgo::CString"
		switch name {
		case "WString":
			fun = "gp::cgo::WString"
		case "cstr":
			fun = "gp::cstr"
		case "wstr":
			fun = "gp::wstr" // wstr.hpp: UTF-16 for the W functions (step 14.4)
		}
		return call(fun, fg.exactExpr(c.Args[0]))
	}
	// C pointers, sizes: C/C++ values or constants (a C literal, nullptr for nil).
	args := make([]cppir.Expr, len(c.Args))
	for i, a := range c.Args {
		if tv := fg.g.info.Types[a]; tv.Value != nil {
			// A constant of the hidden C.int: a C literal.
			args[i] = fg.g.cConstant(tv.Value, types.Typ[types.UntypedInt], a.Pos())
			continue
		}
		args[i] = fg.cValueOf(a)
	}
	return call("gp::cgo::"+name, args...)
}

// isCstrCall reports whether e is a call of C.cstr or C.wstr (a wide holder, the same rules).
func (g *gen) isCstrCall(e ast.Expr) bool {
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	h := g.cgoHelper(c)
	return h == "cstr" || h == "wstr"
}

// holderVars returns the variables that hold a C.cstr holder: those declared from a call of
// C.cstr without a type (name := C.cstr(s), var name = C.cstr(s)), or from another such
// variable (h2 := h, a copy of the holder, which keeps its own bytes or the same static ones).
func (g *gen) holderVars() map[*types.Var]bool {
	if g.holders != nil {
		return g.holders
	}
	g.holders = make(map[*types.Var]bool)
	g.wideVars = make(map[*types.Var]bool)
	type decl struct {
		v     *types.Var
		value ast.Expr
	}
	var decls []decl
	add := func(names []ast.Expr, values []ast.Expr) {
		if len(names) != len(values) {
			return
		}
		for i, v := range values {
			if id, ok := names[i].(*ast.Ident); ok {
				if obj, ok := g.info.Defs[id].(*types.Var); ok {
					decls = append(decls, decl{obj, v})
				}
			}
		}
	}
	for _, f := range g.pkg.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				if s.Tok == token.DEFINE {
					add(s.Lhs, s.Rhs)
				}
			case *ast.ValueSpec:
				if s.Type == nil {
					names := make([]ast.Expr, len(s.Names))
					for i, n := range s.Names {
						names[i] = n
					}
					add(names, s.Values)
				}
			}
			return true
		})
	}
	// A copy of a copy too: until nothing changes.
	for changed := true; changed; {
		changed = false
		for _, d := range decls {
			if !g.holders[d.v] && g.isHolder(d.value) {
				g.holders[d.v] = true
				g.wideVars[d.v] = g.isWideHolder(d.value)
				changed = true
			}
		}
	}
	return g.holders
}

// isHolder reports whether e is a holder of C.cstr: a call of C.cstr, or a variable that holds
// one (holderVars).
func (g *gen) isHolder(e ast.Expr) bool {
	if g.isCstrCall(e) {
		return true
	}
	if id, ok := ast.Unparen(e).(*ast.Ident); ok {
		if v, ok := g.info.Uses[id].(*types.Var); ok {
			return g.holderVars()[v] // (while holderVars fills it, the map so far)
		}
	}
	return false
}

// isWideHolder reports whether the holder e (see isHolder) is one of C.wstr: the wide string of the
// W functions, whose retained form is C.WString.
func (g *gen) isWideHolder(e ast.Expr) bool {
	if c, ok := ast.Unparen(e).(*ast.CallExpr); ok {
		return g.cgoHelper(c) == "wstr"
	}
	if id, ok := ast.Unparen(e).(*ast.Ident); ok {
		if v, ok := g.info.Uses[id].(*types.Var); ok {
			return g.wideVars[v]
		}
	}
	return false
}

// holderNames returns what the messages about the holder e call it: "C.cstr" or "C.wstr", and the
// helper that makes the copy that C keeps: "C.CString" or "C.WString".
func (g *gen) holderNames(e ast.Expr) (holder, retained string) {
	if g.isWideHolder(e) {
		return "C.wstr", "C.WString"
	}
	return "C.cstr", "C.CString"
}

// cstrPlaceOK reports whether the holder e (a C.cstr call or a holder variable) lives as long as its C
// string is used, the node parent (see above), and reports a user error if it isn't.
func (g *gen) cstrPlaceOK(e ast.Expr, parent ast.Node) bool {
	switch p := parent.(type) {
	case *ast.CallExpr:
		if !containsExpr(p.Fun, e) {
			if r, ok := g.cRefOf(p.Fun); ok {
				if n, ok := r.x.(*cppir.Name); !ok || r.kind != interop.Type || !isBuiltinC(n.Name) {
					return true // an argument of a C/C++ function or a constructor
				}
			}
			if g.opaqueCallee(p) {
				return true // of a member of a C/C++ value or of a C++ callable
			}
		}
	case *ast.AssignStmt:
		if inList(p.Lhs, e) {
			return true // assigning a new holder to a holder variable
		}
		if len(p.Lhs) == len(p.Rhs) {
			for i, v := range p.Rhs {
				if v == e || ast.Unparen(v) == e {
					if isBlank(p.Lhs[i]) && !g.isCstrCall(e) ||
						p.Tok == token.DEFINE && g.info.Defs[identOf(p.Lhs[i])] != nil ||
						p.Tok == token.ASSIGN && g.isHolder(p.Lhs[i]) {
						return true // name := C.cstr(s), name = C.cstr(t)
					}
				}
			}
		}
	case *ast.CompositeLit, *ast.KeyValueExpr:
		holder, retained := g.holderNames(e)
		g.userError(e.Pos(), "%s (%s holder) cannot be stored in a composite literal: its C string could outlive the holder; use %s and C.free for a retained C string", g.srcText(e), holder, retained)
		return false
	case *ast.ValueSpec:
		if p.Type == nil && len(p.Names) == len(p.Values) {
			return true // var name = C.cstr(s)
		}
	}
	src := g.srcText(e)
	_, retained := g.holderNames(e)
	g.userError(e.Pos(), "%s can be only an argument of a C/C++ function or the value of a new variable (name := %s): its C string lives only as long as that; for a C string that lives longer, use %s and C.free", src, src, retained)
	return false
}

// identOf returns e as a name, or nil.
func identOf(e ast.Expr) *ast.Ident {
	id, _ := e.(*ast.Ident)
	return id
}
