package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/fspath"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// textDocument/completion and completionItem/resolve (step 11.6), like
// gopls's with its default settings: the candidates (completer.go) sorted by
// score, then by name, and filtered by the name being typed (a prefix, in
// any case; gopls matches fuzzily). With snippets (the client's
// snippetSupport), a function is inserted with its call, Join(|), with
// the cursor in the parentheses and no placeholders for the parameters
// (gopls's completeFunctionCalls without usePlaceholders), and a field of
// a struct literal as Name: |. The list is always incomplete, like gopls's:
// the client asks again as the user types.
//
// The documentation of a candidate comes later, in completionItem/resolve,
// for the candidate that the editor shows: finding a doc comment parses
// the file of the declaration, which is too much for every candidate of a
// big package (gopls has it in the list).
//
// Header/namespace names and supported C/C++ variable receivers use completed background
// facts when typed interop is enabled. No request waits for their compiler work.

// completion holds what completion learned from initialize, and the index
// of the packages to import.
type completion struct {
	snippets      atomic.Bool // the client takes snippets
	insertReplace atomic.Bool // the client takes InsertReplaceEdit
	docMarkdown   atomic.Bool // documentation in Markdown, else plain text
	index         *importIndex
	stop          context.CancelFunc
}

// addCompletion adds the requests of completion, whose index of packages to
// import works with the pipeline p.
func (s *Server) addCompletion(p *cache.Pipeline) {
	ctx, cancel := context.WithCancel(context.Background())
	s.comp.index = newImportIndex(ctx, p)
	s.comp.stop = cancel
	s.requests["textDocument/completion"] = method{run: s.completion}
	s.requests["completionItem/resolve"] = method{run: s.resolveCompletion}
}

// initialize takes the client's choices from its capabilities and offers
// completion in caps. hoverMarkdown is the markup of hover, which the
// documentation uses when the client says nothing (like gopls).
func (c *completion) initialize(client protocol.ClientCapabilities, caps *protocol.ServerCapabilities, hoverMarkdown bool) {
	ci := client.TextDocument.Completion.CompletionItem
	c.snippets.Store(ci.SnippetSupport)
	c.insertReplace.Store(ci.InsertReplaceSupport)
	c.docMarkdown.Store(preferMarkdown(ci.DocumentationFormat, hoverMarkdown))
	caps.CompletionProvider = &protocol.CompletionOptions{TriggerCharacters: []string{".", "#"}, ResolveProvider: true}
}

// completionData is the data of an item, for completionItem/resolve: where
// the completion was, and which candidate it is besides its label and kind:
// the package of its object and the package it imports (two packages
// called util can both have a Foo).
type completionData struct {
	URI       protocol.DocumentURI `json:"uri"`
	Line      uint32               `json:"line"`
	Character uint32               `json:"character"`
	Pkg       string               `json:"pkg,omitempty"`
	Import    string               `json:"import,omitempty"`
}

// dataOf returns the data of the item of cand.
func dataOf(d completionData, cand *candidate) completionData {
	if cand.obj != nil && cand.obj.Pkg() != nil {
		d.Pkg = cand.obj.Pkg().Path()
	}
	if cand.imp != nil {
		d.Import = cand.imp.path
	}
	return d
}

func (s *Server) completion(ctx context.Context, e env, params json.RawMessage) (any, error) {
	var p protocol.CompletionParams
	if err := decode(params, &p); err != nil {
		return nil, err
	}
	if mlist, ok, err := s.macroCompletion(ctx, e, p); ok || err != nil {
		return mlist, err // after a # (step 11.13)
	}
	list := &protocol.CompletionList{IsIncomplete: true, Items: []protocol.CompletionItem{}}
	c, err := s.completer(ctx, e.snapshot, p.TextDocument.URI, p.Position, true)
	if c == nil || err != nil {
		return list, err
	}
	q := c.q
	c.collect()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sortCandidates(c.cands)
	snippets := s.comp.snippets.Load()
	format := protocol.PlainTextTextFormat
	if snippets {
		format = protocol.SnippetTextFormat
	}
	replace := q.rangeOf(q.tok.Pos(c.start), q.tok.Pos(c.end))
	insert := q.rangeOf(q.tok.Pos(c.start), q.tok.Pos(c.off))
	suffix := string(q.src[c.off:c.end])
	data := completionData{URI: p.TextDocument.URI, Line: p.Position.Line, Character: p.Position.Character}
	for _, cand := range c.cands {
		var imports []protocol.TextEdit
		if cand.imp != nil {
			// No item when the import can't be added (gopls drops
			// an item whose import edit fails).
			if imports = c.importEdit(cand.imp); len(imports) == 0 {
				continue
			}
		}
		i := len(list.Items)
		f := c.format(cand)
		text := f.plain
		if snippets {
			text = f.snippet
		}
		var edit protocol.Or_CompletionItem_textEdit
		if s.comp.insertReplace.Load() {
			in := insert
			if suffix == "" || strings.Contains(text, suffix) {
				in = replace
			}
			edit.Value = protocol.InsertReplaceEdit{NewText: text, Insert: in, Replace: replace}
		} else {
			edit.Value = protocol.TextEdit{NewText: text, Range: replace}
		}
		item := protocol.CompletionItem{
			Label:            cand.name,
			Kind:             f.kind,
			Detail:           f.detail,
			TextEdit:         &edit,
			InsertTextFormat: &format,
			// The client sorts by sortText: the order of the scores.
			SortText:            fmt.Sprintf("%05d", i),
			FilterText:          cand.name,
			Preselect:           i == 0,
			Data:                dataOf(data, cand),
			AdditionalTextEdits: imports,
		}
		list.Items = append(list.Items, item)
	}
	if c.memberStatus != nil {
		return memberCompletionList{CompletionList: list, Members: c.memberStatus}, nil
	}
	return list, nil
}

// completer returns the completer at the place at of the document uri, or
// nil where nothing is offered. A file whose x. at the end of a line makes
// the parser take the next line into the selector (x. then var y = 1) is
// checked again with a fixed text first (fixDangling), like gopls's
// fixDanglingSelector; that check is for completion only (keyed by the
// fixed text, cache.Snapshot.PackageWithFile).
func (s *Server) completer(ctx context.Context, snap *cache.Snapshot, uri protocol.DocumentURI, at protocol.Position, filter bool) (*completer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pf, err := snap.ParseFile(ctx, uri, cache.ParseFull)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, nil
	}
	var q *query
	if fixed := fixDangling(pf); fixed != nil {
		// Only the check of the fixed text: the one of the user's text,
		// for the diagnostics, is not waited for.
		pkg, err := snap.PackageWithFile(ctx, pf.Path, fixed)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err == nil {
			q = queryIn(snap, pkg, pf.Path, pf.Src, at)
		}
	}
	if q == nil {
		q = s.focusedQuery(ctx, snap, uri, at)
	}
	if q == nil {
		if q, err = newQuery(ctx, snap, uri, at); q == nil || err != nil {
			return nil, err
		}
	}
	return newCompleter(ctx, q, pf, s.comp.index, filter)
}

// focusedQuery returns the query of the place at in the document uri on the focused check of its
// package (cache.Snapshot.Focused), or nil when there is none: the completions at a place of the
// file that was edited need only the declarations of the package and the bodies of that file, not
// the bodies of the others, so a keystroke in a body does not wait for the whole package to be
// checked.
func (s *Server) focusedQuery(
	ctx context.Context, snap *cache.Snapshot, uri protocol.DocumentURI, at protocol.Position,
) *query {
	path := uri.Path()
	if path == "" {
		return nil
	}
	_, pkg := s.diags.focused(ctx, snap, filepath.Dir(path))
	if pkg == nil || !fspath.Equal(pkg.Focus(), path) {
		return nil
	}
	q, err := queryOfPackage(snap, pkg, path, at)
	if err != nil {
		return nil
	}
	return q
}

// fixDangling returns the text of the parsed file pf with each x. at the end
// of a line made x._ when the parser took what follows into the selector,
// or nil when there is nothing to fix. After x. the parser invents a name
// "_" (a phantom selector) and goes on with the next line: var y = 1 or
// if c { … } is then lost for the rest of the statement, with the names it
// declares (gopls's fixDanglingSelector inserts a real _ and parses again).
// The fixed text is parsed again and fixed again while it has errors, up to
// maxDanglingPasses times, like gopls's loop: a second x. in the statements
// that the first one swallowed is seen only then.
func fixDangling(pf *cache.ParsedFile) []byte {
	if pf.Err == nil {
		return nil
	}
	file, tok, src := pf.File, pf.Tok, pf.Src
	var fixed []byte
	for range maxDanglingPasses {
		next := fixDanglingOnce(file, tok, src)
		if next == nil {
			break
		}
		fixed = next
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, pf.Path, fixed, pf.Mode)
		if err == nil || f == nil {
			break
		}
		file, tok, src = f, fset.File(f.FileStart), fixed
	}
	return fixed
}

// maxDanglingPasses is the most times fixDangling fixes and parses again
// (gopls's 10).
const maxDanglingPasses = 10

// fixDanglingOnce returns a copy of src, the text of file, with the x. at
// the end of a line that have a phantom selector in file made x._, or nil
// when there is none. To keep the byte offsets of the text (the positions
// of the check are those of the document), the _ takes the place of the
// first space or tab between the dot and the next token, out of the
// comments (gopls inserts it): a dot followed by only line ends and
// comments, with no space or tab, is not fixed. Nor is one before ), ] or
// }, which ends the expression anyway, or one with the next token on its
// line.
func fixDanglingOnce(file *ast.File, tok *token.File, src []byte) []byte {
	var fixed []byte
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "_" || !sel.X.End().IsValid() {
			return true
		}
		dot := tok.Offset(sel.X.End())
		for dot < len(src) && (src[dot] == ' ' || src[dot] == '\t') {
			dot++
		}
		if dot >= len(src) || src[dot] != '.' {
			return true
		}
		// The white space and comments after the dot: a newline in
		// them, and the first space or tab out of a comment.
		end, room, newline := dot+1, -1, false
		for end < len(src) {
			switch b := src[end]; {
			case b == ' ' || b == '\t':
				if room < 0 {
					room = end
				}
				end++
				continue
			case b == '\r' || b == '\n':
				newline = newline || b == '\n'
				end++
				continue
			case bytes.HasPrefix(src[end:], []byte("//")):
				k := bytes.IndexByte(src[end:], '\n')
				if k < 0 {
					end = len(src)
				} else {
					end += k // the \n is white space
				}
				continue
			case bytes.HasPrefix(src[end:], []byte("/*")):
				k := bytes.Index(src[end+2:], []byte("*/"))
				if k < 0 {
					end = len(src)
				} else {
					newline = newline || bytes.IndexByte(src[end:end+2+k], '\n') >= 0
					end += 2 + k + 2
				}
				continue
			}
			break
		}
		if end >= len(src) || strings.IndexByte(")]}_", src[end]) >= 0 || !newline {
			return true // the end of the text, of the expression, or a real _
		}
		if room < 0 {
			return true // no room for the _
		}
		if fixed == nil {
			fixed = bytes.Clone(src)
		}
		// "._", then what came before the space or tab at room.
		copy(fixed[dot+2:room+1], src[dot+1:room])
		fixed[dot+1] = '_'
		return true
	})
	return fixed
}

// sortCandidates sorts the candidates by score, then by name (gopls's
// sortItems).
func sortCandidates(cands []*candidate) {
	slices.SortStableFunc(cands, func(a, b *candidate) int {
		switch {
		case a.score > b.score:
			return -1
		case a.score < b.score:
			return 1
		}
		return strings.Compare(a.name, b.name)
	})
}

// importEdit returns the edit that adds the import of p (importEdits),
// made once per package, or nil when the import can't be added.
func (c *completer) importEdit(p *importable) []protocol.TextEdit {
	if c.imports == nil {
		c.imports = make(map[string][]protocol.TextEdit)
	}
	edits, ok := c.imports[p.path]
	if !ok {
		edits = importEdits(c.ctx, c.q, c.pf, *p, c.isStd)
		c.imports[p.path] = edits
	}
	return edits
}

// isStd reports whether an import of the document is of a standard
// package: one that gocpp has, or else, outside the document's module, a
// path whose first element has no dot (goimports's rule: a package of Go's
// standard library that gocpp doesn't have yet).
func (c *completer) isStd(path string) bool {
	if c.ix.isStd(path) {
		return true
	}
	if mod := c.q.lp.Module; mod != nil && mod.Path != "" && (path == mod.Path || strings.HasPrefix(path, mod.Path+"/")) {
		return false
	}
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

// A formatted is a candidate as an item shows it.
type formatted struct {
	kind    protocol.CompletionItemKind
	detail  string
	plain   string // the text to insert without snippets
	snippet string // the text to insert as a snippet
}

// format returns the kind, detail and text of cand (gopls's item and
// formatBuiltin): the detail is the type, or the signature of a function
// ("func(sep string, parts ...string) string"), with the C/C++ types as
// written; a candidate to import says from which package.
func (c *completer) format(cand *candidate) formatted {
	f := formatted{kind: cand.kind, detail: cand.detail, plain: cand.name, snippet: escapeSnippet(cand.name)}
	if cand.insert != "" {
		f.plain, f.snippet = cand.insert, escapeSnippet(cand.insert)
	}
	obj := cand.obj
	if obj == nil {
		return f
	}
	q, qual := c.q, c.qual
	universe := obj.Parent() == types.Universe
	switch o := obj.(type) {
	case *types.Builtin:
		f.kind = protocol.FunctionCompletion
		n := 0
		if s := builtinSignature(o.Name()); s != nil {
			f.detail = "func" + strings.TrimPrefix(s.label(), s.name)
			n = len(s.params)
		}
		if cand.invoke {
			f.snippet = c.callSnippet(o.Name(), false, n)
		}
	case *types.TypeName:
		switch {
		case universe && types.IsInterface(o.Type()):
			f.kind = protocol.InterfaceCompletion
		case universe:
			f.kind = protocol.ClassCompletion
		default:
			f.detail, f.kind = typeDetail(o.Type(), qual)
			if cache.Incomplete(o.Type().Underlying()) {
				if t, ok := c.opaqueType(o, cand.exp); ok {
					f.detail = t
				}
			}
		}
	case *types.Const:
		f.kind = protocol.ConstantCompletion
		if !universe {
			f.detail = strings.TrimPrefix(types.TypeString(o.Type(), qual), "untyped ")
		}
	case *types.Var:
		f.kind = protocol.VariableCompletion
		if o.IsField() {
			f.kind = protocol.FieldCompletion
		}
		switch t, ok := c.opaqueType(o, cand.exp); {
		case ok:
			f.detail = t
		case o.Type() == types.Typ[types.Invalid]:
			f.detail = "" // a value from C/C++: no type in gocpp
		default:
			if _, ok := o.Type().(*types.Struct); ok {
				f.detail = "struct{...}"
			} else {
				f.detail = types.TypeString(o.Type(), qual)
			}
		}
		if cand.field {
			f.snippet = c.fieldSnippet(o.Name())
		}
	case *types.Func:
		f.kind = protocol.FunctionCompletion
		sig, _ := o.Type().(*types.Signature)
		if sig != nil && sig.Recv() != nil {
			f.kind = protocol.MethodCompletion
		}
		if sig != nil {
			var sp *funcSpelling
			if cand.exp != nil {
				if text, ok := cand.exp.OpaqueType(o); ok {
					sp = parseFuncSpelling(text, o)
				}
			} else {
				sp = q.funcSpellingOf(o)
			}
			f.detail = "func" + q.signatureString(sig, qual, sp)
			nparams := sig.Params().Len()
			if m := q.macroOf(o); m != nil {
				// A macro (step 10.9): its declaration, not the hidden parameters of its function.
				f.detail, nparams = m.Sig, len(m.Params)
			}
			if cand.invoke {
				f.snippet = c.callSnippet(o.Name(), needsTypeArgs(sig) && q.macroOf(o) == nil, nparams)
			}
		}
	case *types.PkgName:
		f.kind = protocol.ModuleCompletion
		f.detail = fmt.Sprintf("%q", o.Imported().Path())
	case *types.Nil:
		f.kind = protocol.VariableCompletion
	}
	if cand.imp != nil && f.kind != protocol.ModuleCompletion {
		f.detail = strings.TrimSpace(f.detail + " " + importDetail(cand.imp))
	}
	return f
}

// opaqueType returns the type of obj as written when it comes from C/C++:
// from the document's package and its imports (opaqueSpelling,
// cache.Package.OpaqueType), or from exp for a package to import.
func (c *completer) opaqueType(obj types.Object, exp *cache.Exports) (string, bool) {
	if exp != nil {
		return exp.OpaqueType(obj)
	}
	if v, ok := obj.(*types.Var); ok {
		t := c.q.opaqueSpelling(v)
		return t, t != ""
	}
	return c.q.pkg.OpaqueType(obj)
}

// typeDetail returns the detail and the kind of a type name of the type t
// (gopls's FormatType).
func typeDetail(t types.Type, qual types.Qualifier) (string, protocol.CompletionItemKind) {
	u := t.Underlying()
	if types.IsInterface(u) {
		return "interface{...}", protocol.InterfaceCompletion
	}
	if _, ok := u.(*types.Struct); ok {
		return "struct{...}", protocol.StructCompletion
	}
	return types.TypeString(u, qual), protocol.ClassCompletion
}

// callSnippet returns the snippet of a call of the function name with n
// parameters (gopls's functionCallSnippet without placeholders): Join(|),
// or Pick[|]() when the type arguments can't be inferred; only the name
// when parentheses follow the name already (Jo|(x)).
func (c *completer) callSnippet(name string, typeArgs bool, n int) string {
	var b snippetBuilder
	b.text(name)
	if c.end < len(c.q.src) && c.q.src[c.end] == '(' {
		return b.String()
	}
	if typeArgs {
		b.text("[")
		b.placeholder()
		b.text("]")
	}
	b.text("(")
	if n > 0 {
		b.placeholder()
	}
	b.text(")")
	return b.String()
}

// fieldSnippet returns the snippet of a field name in a struct literal
// (gopls's structFieldSnippet): Name: |, with a comma after it in a
// literal over several lines.
func (c *completer) fieldSnippet(name string) string {
	var b snippetBuilder
	b.text(name)
	if c.lit == nil || c.lit.kv != nil {
		return b.String()
	}
	b.text(": ")
	b.placeholder()
	if c.q.tok.Line(c.pos) != c.q.tok.Line(c.lit.lit.Lbrace) {
		b.text(",")
	}
	return b.String()
}

// needsTypeArgs reports whether a call of a function with the signature
// sig must give type arguments: some type parameter can't be inferred from
// the arguments (gopls's inferableTypeParams).
func needsTypeArgs(sig *types.Signature) bool {
	tparams := sig.TypeParams()
	if tparams.Len() == 0 {
		return false
	}
	free := make(map[*types.TypeParam]bool)
	seen := make(map[types.Type]bool)
	var visit func(t types.Type)
	visit = func(t types.Type) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		switch t := t.(type) {
		case *types.TypeParam:
			free[t] = true
		case *types.Pointer:
			visit(t.Elem())
		case *types.Slice:
			visit(t.Elem())
		case *types.Array:
			visit(t.Elem())
		case *types.Chan:
			visit(t.Elem())
		case *types.Map:
			visit(t.Key())
			visit(t.Elem())
		case *types.Tuple:
			for v := range t.Variables() {
				visit(v.Type())
			}
		case *types.Signature:
			visit(t.Params())
			visit(t.Results())
		case *types.Struct:
			for f := range t.Fields() {
				visit(f.Type())
			}
		case *types.Interface:
			for m := range t.ExplicitMethods() {
				visit(m.Type())
			}
			for e := range t.EmbeddedTypes() {
				visit(e)
			}
		case *types.Union:
			for term := range t.Terms() {
				visit(term.Type())
			}
		case *types.Alias:
			visit(types.Unalias(t))
		case *types.Named:
			for a := range t.TypeArgs().Types() {
				visit(a)
			}
		}
	}
	visit(sig.Params())
	// A type parameter in the constraint of an inferred one is inferred
	// too.
	for changed := true; changed; {
		changed = false
		for tp := range tparams.TypeParams() {
			if free[tp] && !seen[tp.Constraint()] {
				n := len(free)
				visit(tp.Constraint())
				changed = changed || len(free) > n
			}
		}
	}
	for tp := range tparams.TypeParams() {
		if !free[tp] {
			return true
		}
	}
	return false
}

// A snippetBuilder builds an LSP snippet (gopls's snippet.Builder).
type snippetBuilder struct {
	b    strings.Builder
	stop int
}

// snippetEscaper escapes the characters that mean something in a snippet.
var snippetEscaper = strings.NewReplacer(`\`, `\\`, `}`, `\}`, `$`, `\$`)

func escapeSnippet(s string) string { return snippetEscaper.Replace(s) }

func (b *snippetBuilder) text(s string) { snippetEscaper.WriteString(&b.b, s) }

// placeholder writes the next tab stop, empty.
func (b *snippetBuilder) placeholder() {
	b.stop++
	fmt.Fprintf(&b.b, "${%d:}", b.stop)
}

func (b *snippetBuilder) String() string { return b.b.String() }

// resolveCompletion fills in the documentation of a completion item: the
// doc comment of its declaration, in the markup the client wants. It finds
// the candidate again at the place of the completion (the item's data), in
// the snapshot of this request, by its label and kind.
func (s *Server) resolveCompletion(ctx context.Context, e env, params json.RawMessage) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var item protocol.CompletionItem
	if err := decode(params, &item); err != nil {
		return nil, err
	}
	var d completionData
	if raw, err := json.Marshal(item.Data); err != nil || json.Unmarshal(raw, &d) != nil || d.URI == "" {
		return &item, nil // not ours
	}
	c, err := s.completer(ctx, e.snapshot, d.URI, protocol.Position{Line: d.Line, Character: d.Character}, false)
	if c == nil || err != nil {
		return &item, err
	}
	c.collect()
	for _, cand := range c.cands {
		if cand.name != item.Label || dataOf(completionData{}, cand) != (completionData{Pkg: d.Pkg, Import: d.Import}) ||
			c.format(cand).kind != item.Kind {
			continue
		}
		doc, err := c.doc(ctx, cand)
		if err != nil {
			return nil, err
		}
		if doc = strings.TrimSuffix(doc, "\n"); doc != "" {
			if s.comp.docMarkdown.Load() {
				item.Documentation = &protocol.Or_CompletionItem_documentation{Value: protocol.MarkupContent{
					Kind: protocol.Markdown, Value: strings.TrimSuffix(docMarkdown(doc), "\n")}}
			} else {
				item.Documentation = &protocol.Or_CompletionItem_documentation{Value: protocol.MarkupContent{
					Kind: protocol.PlainText, Value: doc}}
			}
		}
		break
	}
	return &item, ctx.Err()
}

// doc returns the doc comment of the candidate, as text, or "".
func (c *completer) doc(ctx context.Context, cand *candidate) (string, error) {
	obj := cand.obj
	q := c.q
	switch {
	case obj == nil:
		return "", nil
	case obj.Parent() == types.Universe:
		if _, doc := builtinDecl(obj.Name()); doc != nil {
			return doc.Text(), nil
		}
		return "", nil
	}
	if pn, ok := obj.(*types.PkgName); ok {
		dep := q.lp.Imports[pn.Imported().Path()]
		if dep == nil {
			return "", nil
		}
		files, err := q.snap.PackageFiles(dep)
		if err != nil {
			return "", nil
		}
		h, err := q.packageHover(dep, files, false)
		if h == nil || err != nil {
			return "", err
		}
		return h.doc, nil
	}
	fset := q.fset
	if cand.exp != nil {
		fset = cand.exp.Fset
	}
	d, err := q.declIn(ctx, obj, fset)
	if err != nil {
		return "", err
	}
	if doc := d.docOf(); doc != nil {
		return doc.Text(), nil
	}
	return "", nil
}
