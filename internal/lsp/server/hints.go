package server

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// textDocument/inlayHint (step 11.9): the names of the parameters at the
// arguments of a call (`name:`), and the types that := and var infer, after the
// names of the variables (`x int := 1`), for range variables too, like
// gopls's parameterNames, assignVariableTypes and rangeVariableTypes hints.
// The two kinds have their own InlayHintKind (Parameter, Type), so a client
// can show one and not the other. The hints for the calls whose error a `!`
// function handles come with step 11.13.
//
// A hint is made only where the checker knows the types: a call of a C/C++
// function, a variable that holds a C/C++ value (its type is opaque) and a
// call of a conversion or of a builtin get none. A parameter name is left out
// when the argument says it already (an argument x, or a selector that ends
// in x, for the parameter x), for a parameter named _ or without a name, and
// for a call whose one argument is a call of several results. Of the
// arguments of a variadic parameter only the first has the hint.

func (s *Server) addHints() {
	s.requests["textDocument/inlayHint"] = method{run: s.inlayHint, contentModified: true}
}

func hintCapabilities(caps *protocol.ServerCapabilities) {
	caps.InlayHintProvider = &protocol.InlayHintOptions{}
}

func (s *Server) inlayHint(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.InlayHintParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	hints := []protocol.InlayHint{}
	q, err := newQuery(ctx, e.snapshot, p.TextDocument.URI, protocol.Position{})
	if err != nil || q == nil {
		return hints, err
	}
	lo, hi, err := q.mapper.RangeOffsets(p.Range)
	if err != nil {
		return nil, protocol.Errorf(protocol.InvalidParams, "%v", err)
	}
	return q.inlayHints(ctx, lo, hi)
}

// inlayHints returns the hints of the file of q between the offsets lo and hi,
// sorted by place.
func (q *query) inlayHints(ctx context.Context, lo, hi int) ([]protocol.InlayHint, error) {
	hints := []protocol.InlayHint{}
	qual := q.qualifier()
	at := func(pos int) protocol.Position { return q.mapper.Position(pos) }
	in := func(off int) bool { return off >= lo && off <= hi }
	typeHint := func(id *ast.Ident) {
		if id == nil || id.Name == "_" || !id.Pos().IsValid() {
			return
		}
		v, ok := q.info.Defs[id].(*types.Var)
		if !ok || v.Type() == nil || v.Type() == types.Typ[types.Invalid] || cache.Incomplete(v.Type()) {
			return
		}
		end := q.tok.Offset(id.End())
		if !in(end) {
			return
		}
		hints = append(hints, protocol.InlayHint{
			Position:    at(end),
			Label:       []protocol.InlayHintLabelPart{{Value: types.TypeString(v.Type(), qual)}},
			Kind:        protocol.Type,
			PaddingLeft: true,
		})
	}
	n := 0
	ast.Inspect(q.file.AST, func(node ast.Node) bool {
		if n++; n%4096 == 0 && ctx.Err() != nil {
			return false
		}
		if node == nil || !node.Pos().IsValid() {
			return true
		}
		if q.tok.Offset(node.End()) < lo || q.tok.Offset(node.Pos()) > hi {
			return false // outside the range
		}
		switch node := node.(type) {
		case *ast.CallExpr:
			hints = q.parameterHints(hints, node, in)
		case *ast.AssignStmt:
			if node.Tok.String() == ":=" {
				for _, l := range node.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						typeHint(id)
					}
				}
			}
		case *ast.RangeStmt:
			if node.Tok.String() == ":=" {
				for _, x := range []ast.Expr{node.Key, node.Value} {
					if id, ok := x.(*ast.Ident); ok {
						typeHint(id)
					}
				}
			}
		case *ast.ValueSpec:
			if node.Type == nil {
				for _, id := range node.Names {
					typeHint(id)
				}
			}
		}
		return true
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(hints, func(i, j int) bool {
		a, b := hints[i].Position, hints[j].Position
		return a.Line < b.Line || a.Line == b.Line && a.Character < b.Character
	})
	return hints, nil
}

// parameterHints adds the parameter name hints of the call to hints.
func (q *query) parameterHints(hints []protocol.InlayHint, call *ast.CallExpr, in func(int) bool) []protocol.InlayHint {
	if len(call.Args) == 0 {
		return hints
	}
	if tv, ok := q.info.Types[call.Fun]; !ok || tv.IsType() || tv.IsBuiltin() {
		return hints // a conversion, a builtin, or a name the checker doesn't know
	}
	ft := q.info.TypeOf(call.Fun)
	if ft == nil {
		return hints
	}
	sig, ok := types.Unalias(ft).Underlying().(*types.Signature)
	if !ok || sig.Params().Len() == 0 {
		return hints
	}
	if len(call.Args) == 1 && sig.Params().Len() > 1 {
		if _, ok := q.info.TypeOf(call.Args[0]).(*types.Tuple); ok {
			return hints // f(g()) with g of several results
		}
	}
	params := sig.Params()
	for i, arg := range call.Args {
		idx := i
		if idx >= params.Len() {
			if !sig.Variadic() {
				break
			}
			idx = params.Len() - 1
		}
		name := params.At(idx).Name()
		if name == "" || name == "_" || !arg.Pos().IsValid() || argSaysName(arg, name) {
			continue
		}
		if sig.Variadic() && idx == params.Len()-1 && i > idx {
			continue // the first of the variadic arguments says it
		}
		off := q.tok.Offset(arg.Pos())
		if !in(off) {
			continue
		}
		hints = append(hints, protocol.InlayHint{
			Position:     q.mapper.Position(off),
			Label:        []protocol.InlayHintLabelPart{{Value: name + ":"}},
			Kind:         protocol.Parameter,
			PaddingRight: true,
		})
	}
	return hints
}

// argSaysName reports whether the argument says the name of its parameter
// already: it is the name, or a selector that ends in it, ignoring case.
func argSaysName(arg ast.Expr, name string) bool {
	switch a := ast.Unparen(arg).(type) {
	case *ast.Ident:
		return strings.EqualFold(a.Name, name)
	case *ast.SelectorExpr:
		return strings.EqualFold(a.Sel.Name, name)
	case *ast.StarExpr:
		return argSaysName(a.X, name)
	case *ast.UnaryExpr:
		return argSaysName(a.X, name)
	}
	return false
}
