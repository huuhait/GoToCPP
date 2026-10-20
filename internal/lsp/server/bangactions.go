package server

import (
	"context"
	"go/ast"
	"go/types"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The code action "Handle the error with !" (step 11.13): on the name of a call that drops its error, the
// action puts a `!` after it, so that `os.Remove(p)` becomes `os.Remove!(p)` and `n := strconv.Atoi(s)`,
// which the checker rejects, becomes `n := strconv.Atoi!(s)`. It is a refactoring (refactor.rewrite), offered
// only where the `!` would be accepted:
//
//   - the callee is a named function or method whose last result is exactly `error` (not a type that is
//     assignable to it: a nil *MyErr would be a non-nil error), and the call is not a C/C++ one;
//   - the call is a statement (also after defer) of a function with one result, or the only value of an
//     assignment or a var declaration that wants the other results (`a, b := f()` for `(A, B, error)`);
//   - it has no `!` yet and no must function handles it already: such a call is the argument of the helper
//     (a package with errors) or an assignment of the error (the lowered tree), never the call of a
//     statement, so the shapes above do not match it.
//
// The edit is one insertion, so it is right whatever the types: the checker says if the `!` is not allowed.

// bangActions returns the action for the call at the start of the range of the request, or none.
func (s *Server) bangActions(ctx context.Context, e env, p protocol.CodeActionParams) ([]protocol.CodeAction, error) {
	if p.Range.Start.Line != p.Range.End.Line {
		return nil, nil // a selection of lines: the cursor is not on a name
	}
	q, err := newQuery(ctx, e.snapshot, p.TextDocument.URI, p.Range.Start)
	if q == nil || err != nil {
		return nil, err
	}
	id, path := identAt(q.file, q.pos)
	if id == nil || q.file.Synthetic[id] {
		return nil, nil
	}
	call := q.droppedErrorCall(id, path)
	if call == nil {
		return nil, nil
	}
	doc, err := documentOfSnapshot(e.snapshot, p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	off := q.tok.Offset(id.End())
	if off > len(doc.src) || off < len(id.Name) || string(doc.src[off-len(id.Name):off]) != id.Name {
		return nil, nil // not the name the user wrote (a rewrite of the loader)
	}
	at := doc.mapper.Position(off)
	return []protocol.CodeAction{{
		Title: "Handle the error of " + id.Name + " with !",
		Kind:  protocol.RefactorRewrite,
		Edit: &protocol.WorkspaceEdit{DocumentChanges: []protocol.DocumentChange{{
			TextDocumentEdit: &protocol.TextDocumentEdit{
				TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
					Version:                doc.version,
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: p.TextDocument.URI},
				},
				Edits: []protocol.Or_TextDocumentEdit_edits_Elem{{Value: protocol.TextEdit{Range: protocol.Range{Start: at, End: at}, NewText: "!"}}},
			},
		}}},
	}}, nil
}

// bangCallee accepts only the syntactic forms in which inserting ! after the name is valid. A callee
// wrapped in parentheses needs a rewrite, not an insertion: (f!)() is not the syntax of (f)(). Parentheses
// in a selector's receiver, as in (v).Method(), do not enclose the method name and remain supported.
func bangCallee(fun ast.Expr) bool {
	switch f := fun.(type) {
	case *ast.Ident, *ast.SelectorExpr:
		return true
	case *ast.IndexExpr:
		return bangCallee(f.X)
	case *ast.IndexListExpr:
		return bangCallee(f.X)
	default:
		return false
	}
}

// droppedErrorCall returns the call whose callee is the name id (path is the nodes around it) when the
// call drops an error that a `!` could take, or nil.
func (q *query) droppedErrorCall(id *ast.Ident, path []ast.Node) *ast.CallExpr {
	if len(path) < 2 {
		return nil
	}
	// the call: id is F, or the Sel of x.F, or the X of F[T]
	var call *ast.CallExpr
	var at int
	for i := len(path) - 1; i >= 0 && i >= len(path)-3; i-- {
		if c, ok := path[i].(*ast.CallExpr); ok && calleeIdent(c.Fun) == id {
			call, at = c, i
			break
		}
	}
	if call == nil || at == 0 || !bangCallee(call.Fun) {
		return nil
	}
	ft := q.info.TypeOf(call.Fun)
	if ft == nil {
		return nil
	}
	if tv, ok := q.info.Types[call.Fun]; !ok || tv.IsType() || tv.IsBuiltin() {
		return nil
	}
	sig, ok := types.Unalias(ft).Underlying().(*types.Signature)
	if !ok || sig.Results().Len() == 0 || sig.Results().Len() > 4 {
		return nil
	}
	n := sig.Results().Len()
	if !types.Identical(sig.Results().At(n-1).Type(), types.Universe.Lookup("error").Type()) {
		return nil
	}
	if q.opaque.IsOpaque(call.Fun) || q.cRooted(call.Fun) {
		return nil
	}
	switch p := path[at-1].(type) {
	case *ast.ExprStmt:
		if p.X == call && n == 1 {
			return call
		}
	case *ast.DeferStmt:
		if p.Call == call && n == 1 {
			return call
		}
	case *ast.AssignStmt:
		if len(p.Rhs) == 1 && p.Rhs[0] == call && len(p.Lhs) == n-1 {
			return call
		}
	case *ast.ValueSpec:
		if len(p.Values) == 1 && p.Values[0] == call && len(p.Names) == n-1 {
			return call
		}
	}
	return nil
}
