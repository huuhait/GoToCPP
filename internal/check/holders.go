package check

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/load"
)

// A Holders knows which expressions are holders of C.cstr (Phase 04 step 4.9): a call of C.cstr or
// C.wstr, or a variable declared from one without a type (name := C.cstr(s), var name = C.cstr(s)),
// or from another such variable (h2 := h, a copy of the holder, which keeps its own bytes or the
// same static ones). A holder is passed to a C/C++ function as it is: its const char* is taken
// where the argument is used, so it is not a Go string to convert.
type Holders struct {
	info   *types.Info
	helper func(*ast.CallExpr) string
	vars   map[*types.Var]bool
	wide   map[*types.Var]bool // the holders among them that hold a C.wstr holder
}

// FindHolders returns the holders of the files. helper returns the name of the cgo helper that a
// call calls ("cstr"), or "" (Package.CgoHelper).
func FindHolders(files []*load.File, info *types.Info, helper func(*ast.CallExpr) string) *Holders {
	h := &Holders{
		info: info, helper: helper,
		vars: make(map[*types.Var]bool), wide: make(map[*types.Var]bool),
	}
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
				if obj, ok := info.Defs[id].(*types.Var); ok {
					decls = append(decls, decl{obj, v})
				}
			}
		}
	}
	for _, f := range files {
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
			if !h.vars[d.v] && h.IsHolder(d.value) {
				h.vars[d.v] = true
				h.wide[d.v] = h.IsWide(d.value)
				changed = true
			}
		}
	}
	return h
}

// IsHolder reports whether e is a holder of C.cstr: a call of C.cstr or C.wstr, or a variable that
// holds one.
func (h *Holders) IsHolder(e ast.Expr) bool {
	if c, ok := ast.Unparen(e).(*ast.CallExpr); ok {
		name := h.helper(c)
		return name == "cstr" || name == "wstr"
	}
	if id, ok := ast.Unparen(e).(*ast.Ident); ok {
		if v, ok := h.info.Uses[id].(*types.Var); ok {
			return h.vars[v]
		}
	}
	return false
}

// IsWide reports whether the holder e (see IsHolder) is one of C.wstr: the wide string of the W
// functions, whose retained form is C.WString.
func (h *Holders) IsWide(e ast.Expr) bool {
	if c, ok := ast.Unparen(e).(*ast.CallExpr); ok {
		return h.helper(c) == "wstr"
	}
	if id, ok := ast.Unparen(e).(*ast.Ident); ok {
		if v, ok := h.info.Uses[id].(*types.Var); ok {
			return h.wide[v]
		}
	}
	return false
}

// Holders returns the holders of the package, found on the first call.
func (p *Package) Holders() *Holders {
	if p == nil || p.Info == nil || p.Load == nil {
		return nil
	}
	p.holdersOnce.Do(func() { p.holders = FindHolders(p.Load.Files, p.Info, p.CgoHelper) })
	return p.holders
}
