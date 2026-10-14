package server

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// textDocument/signatureHelp (step 11.5), like gopls's: inside the
// parentheses of a call, the signature of the function it calls, with the
// parameter that the cursor is at and the function's doc. A call of a
// C/C++ function has no signature yet (its type is opaque; Phase 12).

// A signatureInfo is a signature before it is formatted (gopls's
// signature).
type signatureInfo struct {
	name         string
	doc          string
	params       []string // "x int", "args ...any"
	results      []string
	resultParens bool
	variadic     bool
}

// label returns the signature as signature help shows it:
// "F(x int, args ...any) (string, error)".
func (s *signatureInfo) label() string {
	var b strings.Builder
	b.WriteString(s.name)
	b.WriteByte('(')
	b.WriteString(strings.Join(s.params, ", "))
	b.WriteByte(')')
	if len(s.results) > 0 {
		b.WriteByte(' ')
		if s.resultParens {
			b.WriteByte('(')
		}
		b.WriteString(strings.Join(s.results, ", "))
		if s.resultParens {
			b.WriteByte(')')
		}
	}
	return b.String()
}

func (s *Server) signatureHelp(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.SignatureHelpParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	q, err := newQuery(ctx, e.snapshot, p.TextDocument.URI, p.Position)
	if q == nil || err != nil {
		return nil, err
	}
	sig, call, err := q.signature(ctx, p.Context)
	if sig == nil || err != nil {
		return nil, err
	}
	info := protocol.SignatureInformation{Label: sig.label()}
	for _, p := range sig.params {
		info.Parameters = append(info.Parameters, protocol.ParameterInformation{Label: p})
	}
	if sig.doc != "" {
		if s.nav.sigMarkdown.Load() {
			info.Documentation = &protocol.Or_SignatureInformation_documentation{Value: protocol.MarkupContent{
				Kind: protocol.Markdown, Value: strings.TrimSuffix(docMarkdown(sig.doc), "\n")}}
		} else {
			info.Documentation = &protocol.Or_SignatureInformation_documentation{Value: protocol.MarkupContent{
				Kind: protocol.PlainText, Value: sig.doc}}
		}
	}
	help := &protocol.SignatureHelp{Signatures: []protocol.SignatureInformation{info}}
	help.ActiveParameter = activeParameter(sig, q.pos, call)
	info.ActiveParameter = help.ActiveParameter
	help.Signatures[0] = info
	return help, nil
}

// signature returns the signature of the innermost call around the place
// of q, and the call (nil when the place is on the name of a function
// that is not called, as in once.Do(f), where it is the signature of f),
// or nil (gopls's SignatureHelp). sc is the context that the client
// sent, or nil.
func (q *query) signature(ctx context.Context, sc *protocol.SignatureHelpContext) (*signatureInfo, *ast.CallExpr, error) {
	path, _ := astutil.PathEnclosingInterval(q.file.AST, q.pos, q.pos)
	var call *ast.CallExpr
	var fn ast.Expr
loop:
	for i, n := range path {
		switch n := n.(type) {
		case *ast.Ident:
			// The name of a function, also one that is not called:
			// once.Do(f⁁) shows f.
			if t := q.info.TypeOf(n); t != nil && q.info.Defs[n] == nil {
				if _, ok := t.Underlying().(*types.Signature); ok {
					fn = n
					if i+1 < len(path) {
						if sel, ok := path[i+1].(*ast.SelectorExpr); ok && sel.Sel == n {
							fn = sel
						}
					}
					break loop
				}
			}
		case *ast.CallExpr:
			// The ) may be missing.
			if n.Lparen <= q.pos && (q.pos <= n.Rparen || !n.Rparen.IsValid()) {
				call, fn = n, n.Fun
				break loop
			}
		case *ast.FuncLit, *ast.FuncType, *ast.CompositeLit:
			// In a function literal or a composite literal, which may
			// be an argument of a call: no help (gopls).
			return nil, nil, nil
		case *ast.BasicLit:
			// Typing in a string: only when asked for (gopls).
			if n.Kind == token.STRING && (sc == nil || sc.TriggerKind != protocol.SigInvoked && !sc.IsRetrigger) {
				return nil, nil, nil
			}
		}
	}
	if fn == nil {
		return nil, nil, nil
	}
	tv, ok := q.info.Types[fn]
	if !ok || tv.IsType() || tv.Type == nil {
		return nil, nil, nil // a C/C++ function (opaque), a conversion, or broken code
	}
	sig, ok := tv.Type.Underlying().(*types.Signature)
	if !ok {
		return nil, nil, nil
	}
	var obj types.Object
	switch fn := ast.Unparen(fn).(type) {
	case *ast.Ident:
		obj = q.info.ObjectOf(fn)
	case *ast.SelectorExpr:
		obj = q.info.ObjectOf(fn.Sel)
	}
	if obj != nil && isBuiltin(obj) {
		if obj.Name() == "Error" {
			return &signatureInfo{name: "Error", results: []string{"string"}, doc: "Error returns the error message."}, call, nil
		}
		if s := builtinSignature(obj.Name()); s != nil {
			return s, call, nil
		}
		return nil, nil, nil
	}
	qual := q.qualifier()
	s := &signatureInfo{name: "func", variadic: sig.Variadic()}
	if obj != nil {
		s.name = obj.Name()
		d, err := q.declOf(ctx, obj)
		if err != nil {
			return nil, nil, err
		}
		if doc := d.docOf(); doc != nil {
			s.doc = doc.Text()
		}
	}
	s.params = q.tupleString(sig.Params(), sig.Variadic(), qual)
	s.results = q.tupleString(sig.Results(), false, qual)
	s.resultParens = len(s.results) > 1 || len(s.results) == 1 && sig.Results().At(0).Name() != ""
	return s, call, nil
}

// activeParameter returns the index of the parameter that the place pos is
// at in the call, or nil (gopls's activeParameter): the number of
// arguments that end before pos, but never past the last parameter of a
// variadic function.
func activeParameter(s *signatureInfo, pos token.Pos, call *ast.CallExpr) *uint32 {
	if call == nil || len(s.params) == 0 {
		return nil
	}
	if !(call.Lparen < pos && (pos <= call.Rparen || !call.Rparen.IsValid())) {
		return nil
	}
	n := uint32(len(s.params))
	var active uint32
	for _, arg := range call.Args {
		if pos <= arg.End() {
			break
		}
		if !s.variadic || active < n-1 {
			active++
		}
	}
	return &active
}
