package typedinterop

import (
	"context"
	"go/ast"
	"go/token"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/typedinterop/adapt"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The calls that return an HRESULT are errors in the program (ADR-137). The result table says which
// ones (adapt.ResultTable), and this file asks it of the checked package: check.Config.HResults
// is the hook that gives the answer to the check, which wraps each call in error(call) before it
// is done with the package, and the lowering asks ReturnsHResult of the same report for the
// conversion it meets.

// HResultsHook returns the check.HResultCalls hook of typed interop. opts is the configuration of
// the analysis that the build runs after the check (Analyze), so that the probes of the hook are
// the ones the build asks for and its compiler runs are cache hits. Without clang the hook returns
// ErrNoClang and check goes on without it.
func HResultsHook(opts Options) check.HResultCalls {
	return func(ctx context.Context, fset *token.FileSet, p *check.Package) ([]*ast.CallExpr, error) {
		if len(UnitIncludes(p)) == 0 {
			return nil, nil
		}
		// The probe's answers decide most calls. The declarations are asked for only when a call was not
		// probed (see ReturnsHResult): they cost more than the probe when its answers are cached.
		first := opts
		first.Decls = false
		rep, err := Analyze(ctx, p, fset, first)
		if err != nil {
			return nil, err
		}
		if opts.Decls && rep.hasUnprobedCall() {
			if rep, err = Analyze(ctx, p, fset, opts); err != nil {
				return nil, err
			}
		}
		return rep.HResultCalls(p), nil
	}
}

// hasUnprobedCall reports whether a call of a C/C++ function has no probe answer to tell its type
// from: the declarations of its callee have to.
func (r *Report) hasUnprobedCall() bool {
	if r == nil || r.Set == nil || r.Table == nil {
		return false
	}
	for _, ref := range r.Set.Refs {
		if ref.Kind != collect.KindCall {
			continue
		}
		if res := r.Table.Result(ref.ID); res == nil || !res.OK || ref.Incomplete != "" {
			return true
		}
	}
	return false
}

// HResultCalls returns the calls of the package p that return an HRESULT and that the program uses as
// the error: not as a statement and not converted to a number. (A call that the program converts to
// error itself, error(f()), is wrapped once more, which changes nothing: error(error(f())) is the same value.)
func (r *Report) HResultCalls(p *check.Package) []*ast.CallExpr {
	if r == nil || r.Set == nil || r.Table == nil {
		return nil
	}
	parents := collect.ParentsOf(p)
	parentOf := func(n ast.Node) ast.Node { return parents[n] }
	var out []*ast.CallExpr
	for _, f := range p.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || !r.ReturnsHResult(p, c) || collect.KeepsHResult(p, parentOf, c) {
				return true
			}
			out = append(out, c)
			return true
		})
	}
	return out
}

// ReturnsHResult reports whether the call c of a C/C++ function returns the typedef HRESULT: by the
// probe's answer for the call, or, for a call that the probe could not type (an argument that it
// cannot spell makes the whole call untyped: &n of a Go number variable, &p of a pointer variable for
// a void** parameter, which the argument table adapts), by the declarations of the callee.
func (r *Report) ReturnsHResult(p *check.Package, c *ast.CallExpr) bool {
	ref := r.Set.RefOf(c)
	if ref == nil || ref.Kind != collect.KindCall {
		return false
	}
	res := r.Table.Result(ref.ID)
	if res != nil && res.OK && ref.Incomplete == "" {
		row := adapt.ResultRowOf(res)
		return row != nil && row.Name == adapt.RowHResult
	}
	if r.Mapper == nil {
		return false
	}
	sigs, ok := r.calleeSigs(p, c, ref)
	return ok && adapt.SigsReturnHResult(r.Mapper, sigs)
}

// calleeSigs returns the signatures of the overloads that the call c can be: for a free function the
// overload set of its name, for a method of a C++ object the methods of that name of its class and
// bases. false when there is none, or a candidate is a template.
func (r *Report) calleeSigs(p *check.Package, c *ast.CallExpr, ref *collect.Ref) ([]adapt.Sig, bool) {
	if call, ok := ref.Expr.(*collect.Call); ok {
		if path, ok := call.Fun.(*collect.Path); ok {
			if name, template, ok := probe.FuncNameOf(r.Kinds, path); ok && !template {
				return adapt.FuncSigs(r.Decls, name)
			}
		}
	}
	sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	v, _ := r.Typed(r.MemberObject(p, sel))
	return adapt.MethodSigs(r.Mapper, v, sel.Sel.Name)
}
