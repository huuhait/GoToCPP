package server

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/token"
	"go/types"
	"sort"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// textDocument/semanticTokens/full and /range (step 11.9).
//
// The server marks the names whose meaning a grammar can't know: the kind of
// every name from the type information (a package, a type, a struct or
// interface type, a type parameter, a parameter, a variable, a field, a
// function, a method), with the modifiers declaration (the place that declares
// the name), readonly (constants), defaultLibrary (a predeclared name or one
// of the standard library) and modification (a variable or field that is
// assigned to). It marks nothing else (keywords, strings, comments, numbers:
// the TextMate grammar does that, and a client overlays the tokens on it).
//
// C/C++ names get types of their own (the legend's last three), which a client
// that doesn't know them ignores: cppHeader is the name of a header package
// (raylib in raylib.InitWindow), cppNamespace a C++ namespace (std, fs) and
// cppName a name after one of them or a member of a C/C++ value
// (InitWindow, vector, push_back). The editor extension maps them to colors
// (editors/vscode/package.json). The `!` calls and functions and the #macros
// of Phase 10 come with step 11.13 (the legend has no entries for them yet).
//
// The tokens come from the checked file: a walk over its syntax tree with
// the types.Info. They need the package to be checked, so a file that is not
// in its package (excluded by build constraints, or a test file, which has no
// check of its own in the pipeline) has no tokens. They are not cached: the
// walk is about as fast as a hover (see the benchmarks), and a change makes
// the old tokens stale anyway (a request for semantic tokens is cancelled with
// ContentModified by the next change).

// The token types of the legend, in the order of their numbers.
const (
	tokNamespace = iota
	tokType
	tokStruct
	tokInterface
	tokTypeParameter
	tokParameter
	tokVariable
	tokProperty
	tokFunction
	tokMethod
	tokCppHeader
	tokCppNamespace
	tokCppName
)

var semanticTypes = []string{
	tokNamespace:     "namespace",
	tokType:          "type",
	tokStruct:        "struct",
	tokInterface:     "interface",
	tokTypeParameter: "typeParameter",
	tokParameter:     "parameter",
	tokVariable:      "variable",
	tokProperty:      "property",
	tokFunction:      "function",
	tokMethod:        "method",
	tokCppHeader:     "cppHeader",
	tokCppNamespace:  "cppNamespace",
	tokCppName:       "cppName",
}

// The token modifiers of the legend, as bits.
const (
	modDeclaration = 1 << iota
	modReadonly
	modDefaultLibrary
	modModification
)

var semanticModifiers = []string{"declaration", "readonly", "defaultLibrary", "modification"}

// semanticCapabilities adds the semantic tokens to caps.
func semanticCapabilities(caps *protocol.ServerCapabilities) {
	caps.SemanticTokensProvider = &protocol.SemanticTokensOptions{
		Legend: protocol.SemanticTokensLegend{TokenTypes: semanticTypes, TokenModifiers: semanticModifiers},
		Full:   &protocol.Or_SemanticTokensOptions_full{Value: true},
		Range:  &protocol.Or_SemanticTokensOptions_range{Value: true},
	}
}

func (s *Server) addSemantic() {
	s.requests["textDocument/semanticTokens/full"] = method{run: s.semanticTokensFull, contentModified: true}
	s.requests["textDocument/semanticTokens/range"] = method{run: s.semanticTokensRange, contentModified: true}
}

func (s *Server) semanticTokensFull(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.SemanticTokensParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	return semanticTokens(ctx, e, p.TextDocument.URI, nil)
}

func (s *Server) semanticTokensRange(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.SemanticTokensRangeParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	return semanticTokens(ctx, e, p.TextDocument.URI, &p.Range)
}

// A semToken is a token in the file: where it starts and how long it is, in
// bytes, its type and its modifiers.
type semToken struct {
	off, end int
	typ      int
	mods     uint32
}

// semanticTokens answers a request for the tokens of the document, or of
// the range rng of it.
func semanticTokens(ctx context.Context, e env, uri protocol.DocumentURI, rng *protocol.Range) (any, error) {
	q, err := newQuery(ctx, e.snapshot, uri, protocol.Position{})
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, nil // the document is not a file of its package: no tokens
	}
	if rng == nil && q.pkg != nil && q.tests == nil {
		// The tokens of the whole file depend on the check and the text
		// alone: made once for the package (every request of an editor that
		// scrolls or reopens the file asks again).
		data := q.pkg.Memo(semanticKey{q.file.Path}, func() any {
			return q.encodeTokens(q.semanticTokens(), 0, len(q.src))
		}).([]uint32)
		return &protocol.SemanticTokens{Data: data}, nil
	}
	toks := q.semanticTokens()
	lo, hi := 0, len(q.src)
	if rng != nil {
		var err error
		if lo, hi, err = q.mapper.RangeOffsets(*rng); err != nil {
			return nil, protocol.Errorf(protocol.InvalidParams, "%v", err)
		}
	}
	return &protocol.SemanticTokens{Data: q.encodeTokens(toks, lo, hi)}, nil
}

// semanticKey is the key of the encoded tokens of a file in the package's memo.
type semanticKey struct{ path string }

// encodeTokens returns the tokens that lie in [lo, hi) in the protocol's
// form: five numbers each, relative to the one before.
func (q *query) encodeTokens(toks []semToken, lo, hi int) []uint32 {
	data := make([]uint32, 0, 5*len(toks))
	line, col := uint32(0), uint32(0)
	for _, t := range toks {
		if t.off < lo || t.end > hi {
			continue
		}
		pos := q.mapper.Position(t.off)
		length := utf16Length(string(q.src[t.off:t.end]))
		dl := pos.Line - line
		dc := pos.Character
		if dl == 0 {
			dc -= col
		}
		data = append(data, dl, dc, length, uint32(t.typ), t.mods)
		line, col = pos.Line, pos.Character
	}
	return data
}

// utf16Length returns the number of UTF-16 code units of s.
func utf16Length(s string) uint32 {
	n := uint32(0)
	for _, r := range s {
		n++
		if r >= 0x10000 {
			n++
		}
	}
	return n
}

// semanticTokens returns the tokens of the file of q, sorted by place.
func (q *query) semanticTokens() []semToken {
	var toks []semToken
	seen := make(map[int]bool) // the offsets that have a token
	add := func(id *ast.Ident, typ int, mods uint32) {
		if id == nil || !id.Pos().IsValid() || id.Name == "_" {
			return
		}
		off := q.tok.Offset(id.Pos())
		end := off + len(id.Name)
		if seen[off] || end > len(q.src) || string(q.src[off:end]) != id.Name {
			return // not the name the user wrote (the loader's rewrites)
		}
		seen[off] = true
		toks = append(toks, semToken{off, end, typ, mods})
	}

	// The names of headers and namespaces, which the loader took out of the
	// tree or rewrote.
	if rw := q.file.Rewrite; rw != nil {
		for _, u := range rw.Uses {
			switch u.Kind {
			case load.HeaderUse:
				add(u.Ident, tokCppHeader, 0)
			case load.NamespaceUse:
				add(u.Ident, tokCppNamespace, 0)
			}
		}
	}
	// The imports of headers and the renames of namespaces name them in
	// their specs.
	if info := q.file.Info; info != nil {
		for _, rn := range info.NamespaceRenames {
			if rn.Spec != nil && rn.Spec.Name != nil {
				add(rn.Spec.Name, tokCppNamespace, modDeclaration)
			}
		}
		for _, inc := range info.Includes {
			if inc.Spec != nil && inc.Spec.Name != nil {
				add(inc.Spec.Name, tokCppHeader, modDeclaration)
			}
		}
	}

	params := q.parameterPositions()
	writes := writePositions(q.file, q.tok)
	ast.PreorderStack(q.file.AST, nil, func(n ast.Node, stack []ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || q.file.Synthetic[id] {
			return true // not a name, or one that check wrote for a ! form
		}
		var sel *ast.SelectorExpr
		if len(stack) > 0 {
			sel, _ = stack[len(stack)-1].(*ast.SelectorExpr)
			if sel != nil && sel.Sel != id {
				sel = nil
			}
		}
		def := q.info.Defs[id]
		obj := def
		if obj == nil {
			obj = q.info.Uses[id]
		}
		if obj == nil {
			// A member of a C/C++ value or a name of a header or namespace:
			// opaque to the checker.
			if sel != nil && (q.opaque.IsOpaque(sel.X) || q.cRooted(sel.X)) {
				add(id, tokCppName, 0)
			}
			return true
		}
		typ, mods := q.classify(obj, params)
		if typ < 0 {
			return true
		}
		if def != nil {
			mods |= modDeclaration
		}
		if def == nil && writes[q.tok.Offset(id.Pos())] && (typ == tokVariable || typ == tokProperty || typ == tokParameter) {
			mods |= modModification
		}
		add(id, typ, mods)
		return true
	})
	sort.Slice(toks, func(i, j int) bool { return toks[i].off < toks[j].off })
	return toks
}

// cRooted reports whether the expression x is C or a selector chain that
// starts at C: the opaque names that the loader made out of header names.
func (q *query) cRooted(x ast.Expr) bool {
	for {
		switch e := ast.Unparen(x).(type) {
		case *ast.SelectorExpr:
			x = e.X
		case *ast.Ident:
			pn, ok := q.info.Uses[e].(*types.PkgName)
			return ok && pn.Imported().Path() == "C"
		default:
			return false
		}
	}
}

// parameterPositions returns the places of the names of the parameters,
// results and receivers that the file declares: go/types has no way to tell a
// parameter from another variable.
func (q *query) parameterPositions() map[token.Pos]bool {
	params := make(map[token.Pos]bool)
	add := func(list *ast.FieldList) {
		if list == nil {
			return
		}
		for _, f := range list.List {
			for _, id := range f.Names {
				if !q.file.Synthetic[id] {
					params[id.Pos()] = true
				}
			}
		}
	}
	ast.Inspect(q.file.AST, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			add(n.Recv)
		case *ast.FuncType:
			add(n.Params)
			add(n.Results)
		}
		return true
	})
	return params
}

// classify returns the token type and modifiers of a name that stands for
// obj, or a negative type for a name that gets none (labels, the hidden
// import of the loader).
func (q *query) classify(obj types.Object, params map[token.Pos]bool) (int, uint32) {
	var mods uint32
	if obj.Pkg() == nil {
		mods |= modDefaultLibrary // predeclared
	} else if dep := q.lp.Imports[obj.Pkg().Path()]; dep != nil && dep.Std {
		mods |= modDefaultLibrary
	}
	switch o := obj.(type) {
	case *types.PkgName:
		if o.Imported().Path() == "C" {
			return -1, 0
		}
		return tokNamespace, mods
	case *types.TypeName:
		switch t := types.Unalias(o.Type()).(type) {
		case *types.TypeParam:
			return tokTypeParameter, mods
		case *types.Named:
			switch t.Underlying().(type) {
			case *types.Struct:
				return tokStruct, mods
			case *types.Interface:
				return tokInterface, mods
			}
		case *types.Interface:
			return tokInterface, mods
		}
		return tokType, mods
	case *types.Var:
		switch {
		case o.IsField():
			return tokProperty, mods
		case params[o.Pos()]:
			return tokParameter, mods
		}
		return tokVariable, mods
	case *types.Const:
		return tokVariable, mods | modReadonly
	case *types.Nil:
		return tokVariable, mods | modReadonly
	case *types.Func:
		if sig, ok := o.Type().(*types.Signature); ok && sig.Recv() != nil {
			return tokMethod, mods
		}
		return tokFunction, mods
	case *types.Builtin:
		return tokFunction, mods
	}
	return -1, 0 // a label
}
