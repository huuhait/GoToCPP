package server

import (
	"bytes"
	"go/ast"
	goformat "go/format"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The `!` and macro features of the editor (step 11.13, ADR-099).
//
// The checked trees are plain Go: the check turned every F!(x) into the code a programmer writes
// by hand, and the loader turned every macro use into the call of a function. What the user wrote
// is still known: check.Package.Must lists the calls (with the place of the `!`), the calls that a
// must function handles without one, the must functions and, for each source statement that the
// lowering rewrote, the code that stands for it; load.Package.MacroUses lists the callee of each
// macro use, and Macros the declarations. The features read those lists and print the code
// with go/format, on request only (hover, tooltips, the expansion), so a keystroke pays for none of
// it; the indexes below are built once per checked package (cache.Package.Memo).

// A bangCall is a call that a `!` handles: one the user wrote with a `!`, or one a must function
// handles for the user.
type bangCall struct {
	check.MustCall
	implicit bool   // no `!` in the text: the enclosing function is a must function
	fname    string // the name of the function the call is in, as the user wrote it
}

// A bangIndex is what the editor knows of the `!` forms of a package, sorted by place.
type bangIndex struct {
	calls  []*bangCall                   // the explicit and the handled calls, by Pos
	byName map[token.Pos]*bangCall       // by the place of the name of the callee
	names  []*bangCall                   // the same, by that place
	funcs  map[token.Pos]*check.MustFunc // by the place of the name of a must function
	stmts  []check.MustStmt
}

type bangKey struct{}

// bang returns the index of the `!` forms of the package of q, or nil when it has none (also when its
// rewrite stopped at an error: the trees keep their helpers then, and the checker says what is wrong).
func (q *query) bang() *bangIndex {
	if q.pkg == nil {
		return nil
	}
	return bangOf(q.pkg)
}

// bangOf is bang for a checked package.
func bangOf(pkg *cache.Package) *bangIndex {
	cp := pkg.Check
	if cp == nil {
		return nil
	}
	m := cp.Must
	if m == nil {
		m = cp.MustPartialInfo() // the package has errors: the places, and a guess of what each ! does
	}
	if m == nil {
		return nil
	}
	return pkg.Memo(bangKey{}, func() any { return buildBang(m) }).(*bangIndex)
}

// cp returns the checked package of the query.
func (q *query) cp() *check.Package {
	if q.pkg != nil {
		return q.pkg.Check
	}
	return nil
}

func buildBang(m *check.MustInfo) *bangIndex {
	b := &bangIndex{byName: make(map[token.Pos]*bangCall, len(m.Calls)+len(m.Handled)), funcs: make(map[token.Pos]*check.MustFunc, len(m.Funcs)), stmts: m.Stmts}
	names := make(map[ast.Node]string, len(m.Funcs))
	for i := range m.Funcs {
		f := &m.Funcs[i]
		names[f.Decl] = f.Name
		b.funcs[f.Decl.Name.NamePos] = f
	}
	add := func(list []check.MustCall, implicit bool) {
		for _, c := range list {
			bc := &bangCall{MustCall: c, implicit: implicit}
			switch fn := c.Func.(type) {
			case *ast.FuncDecl:
				if n, ok := names[fn]; ok {
					bc.fname = n
				} else {
					bc.fname = fn.Name.Name
				}
			case *ast.FuncLit:
				bc.fname = ""
			}
			b.calls = append(b.calls, bc)
			if c.Name.IsValid() {
				b.byName[c.Name] = bc
			}
		}
	}
	add(m.Calls, false)
	add(m.Handled, true)
	sort.SliceStable(b.calls, func(i, j int) bool { return b.calls[i].Pos < b.calls[j].Pos })
	for _, c := range b.calls {
		if c.Name.IsValid() && c.Callee != nil {
			b.names = append(b.names, c)
		}
	}
	sort.SliceStable(b.names, func(i, j int) bool { return b.names[i].Name < b.names[j].Name })
	return b
}

// callOf returns the call that a `!` handles for which id is the name of the callee, or nil.
func (b *bangIndex) callOf(id *ast.Ident) *bangCall {
	if b == nil {
		return nil
	}
	return b.byName[id.Pos()]
}

// callAt returns the call whose name of the callee, with its !, is at pos (the cursor on the name or on the !, or
// right after it), or nil. Unlike callOf it does not need the tree to have the name at its place: the lowering
// moves some names (a loop's condition, a deferred call, the second operand of &&).
func (b *bangIndex) callAt(pos token.Pos) *bangCall {
	if b == nil {
		return nil
	}
	i := sort.Search(len(b.names), func(i int) bool { return b.names[i].Name > pos }) - 1
	if i < 0 {
		return nil
	}
	c := b.names[i]
	end := c.Bang // an implicit call has no !: its name ends where the callee's name does
	if c.Bang.IsValid() {
		end = c.Bang + 1
	} else {
		end = c.Name + token.Pos(len(c.Callee.Name))
	}
	if pos > end {
		return nil
	}
	return c
}

// funcAt returns the must function whose name, with its !, is at pos, or nil. It is for the place where the tree has no
// name of the user's: main! and init!, whose names the lowering replaces (must_main and a wrapper main).
func (b *bangIndex) funcAt(pos token.Pos) *check.MustFunc {
	if b == nil {
		return nil
	}
	for _, f := range b.funcs {
		if f.Decl.Name.NamePos <= pos && pos <= f.Bang+1 {
			return f
		}
	}
	return nil
}

// exitFuncHover is the hover of main! and init! at their declaration.
func (q *query) exitFuncHover(f *check.MustFunc) (*hoverResult, protocol.Range) {
	fd := f.Decl
	start, end := fd.Type.Func, fd.Body.Lbrace
	var written string
	if start.IsValid() && end.IsValid() && q.tok.Offset(start) < q.tok.Offset(end) {
		written = strings.TrimSpace(string(q.src[q.tok.Offset(start):q.tok.Offset(end)]))
	}
	what := "If the program ends with an error, `main`'s defers run first, then `<program>: <error>` is printed to stderr and the program exits with code 1."
	if f.Name == "init" {
		what = "If it returns an error, `<program>: <error>` is printed to stderr and the program exits with code 1, before `main` starts."
	}
	h := &hoverResult{signature: "func " + f.Name + "() error", text: joinParas(mustFuncNote(written), what)}
	return h, q.rangeOf(fd.Name.NamePos, f.Bang)
}

// calleeIdent returns the name that the function expression fun calls: F in F, pkg.F, x.F and F[int].
func calleeIdent(fun ast.Expr) *ast.Ident {
	for {
		switch e := fun.(type) {
		case *ast.Ident:
			return e
		case *ast.SelectorExpr:
			return e.Sel
		case *ast.IndexExpr:
			fun = e.X
		case *ast.IndexListExpr:
			fun = e.X
		case *ast.ParenExpr:
			fun = e.X
		default:
			return nil
		}
	}
}

// stmtOf returns the smallest statement of the source that has the span of c and the code that stands for it.
func (b *bangIndex) stmtOf(c *bangCall) *check.MustStmt {
	var best *check.MustStmt
	for i := range b.stmts {
		s := &b.stmts[i]
		if s.Pos <= c.Pos && c.End <= s.End && (best == nil || s.End-s.Pos < best.End-best.Pos) {
			best = s
		}
	}
	return best
}

// short is the label of what a failing call does, for the inlay hints.
func (c *bangCall) short() string {
	what := "return err"
	switch c.Kind {
	case check.MustFatal:
		what = "t.Fatal"
	case check.MustPanic:
		what = "panic"
	case check.MustExit:
		what = "exit 1"
	}
	if c.Deferred {
		what = "deferred: " + what
	}
	return "→ " + what
}

// behavior says in a sentence what happens when the error of the call is not nil.
func (c *bangCall) behavior() string {
	fn := "the function"
	if c.fname != "" {
		fn = "`" + c.fname + "`"
	}
	closure := c.fname == ""
	if _, ok := c.Func.(*ast.FuncLit); ok {
		closure = true
		fn = "the closure"
	}
	switch {
	case c.Deferred && c.Kind == check.MustReturn:
		return "If this call fails while " + fn + " is about to return a nil error, " + fn + " returns this error instead (an error that is already on its way is kept; when a panic is running the error is dropped)."
	case c.Deferred && c.Kind == check.MustPanic:
		return "If this call fails, " + fn + " panics with the error (when a panic is already running the error is dropped and the panic goes on)."
	case c.Deferred && c.Kind == check.MustExit:
		return "If this call fails while the program ends, `<program>: <error>` is printed to stderr and the program exits with code 1 (when a panic is running the error is dropped)."
	case c.Deferred && c.Kind == check.MustFatal:
		return "If this call fails, the test fails and stops (`t.Fatal`); when a panic is running the error is dropped."
	case c.Kind == check.MustReturn && closure:
		return "If the error is not nil, " + fn + " returns it through its own results (the outer function is not left)."
	case c.Kind == check.MustReturn:
		return "If the error is not nil, " + fn + " returns it: the other results are zero values, and the defers run."
	case c.Kind == check.MustFatal:
		return "If the error is not nil, the test fails and stops: `t.Fatal(err)`."
	case c.Kind == check.MustPanic && c.Func == nil:
		return "If the error is not nil, the initializer of the package variable panics with it (there is no function to return from)."
	case c.Kind == check.MustPanic:
		return "If the error is not nil, " + fn + " panics with it: it has no error result (like `regexp.MustCompile`)."
	case c.Kind == check.MustExit && c.fname == "init":
		return "If the error is not nil, `<program>: <error>` is printed to stderr and the program exits with code 1: it stops before `main` starts (an `init` has no defers of `main` to run)."
	case c.Kind == check.MustExit:
		return "If the error is not nil, `main`'s defers run, then `<program>: <error>` is printed to stderr and the program exits with code 1."
	}
	return ""
}

// tooltip is the text of the inlay hint's tooltip.
func (c *bangCall) tooltip() string {
	head := "`!`: "
	if c.implicit {
		head = "Handled for you: the call drops its error, and `" + c.fname + "` is a must function. "
		if c.fname == "" {
			head = "Handled for you. "
		}
	}
	return head + c.behavior()
}

// printStmts prints the statements as Go, one after the other, with at most maxLines lines.
func printStmts(cp *check.Package, list []ast.Stmt, maxLines int) string {
	if len(list) == 0 {
		return ""
	}
	var b bytes.Buffer
	func() {
		defer func() {
			if recover() != nil {
				b.Reset()
			}
		}()
		if err := goformat.Node(&b, token.NewFileSet(), list); err != nil {
			b.Reset()
		}
	}()
	text := strings.TrimRight(b.String(), "\n")
	if cp != nil {
		text = cp.UserText(text)
	}
	if maxLines > 0 {
		if lines := strings.Split(text, "\n"); len(lines) > maxLines {
			text = strings.Join(lines[:maxLines], "\n") + "\n// ..."
		}
	}
	return text
}

// printExpr prints the expression as Go. A func literal argument whose body does not fit on a line is
// shortened to func(...) T { ... } so that a macro call stays a call.
func printExpr(cp *check.Package, e ast.Expr) string {
	var b bytes.Buffer
	func() {
		defer func() {
			if recover() != nil {
				b.Reset()
			}
		}()
		if err := goformat.Node(&b, token.NewFileSet(), e); err != nil {
			b.Reset()
		}
	}()
	text := b.String()
	if cp != nil {
		text = cp.UserText(text)
	}
	return text
}

// bangHover adds what the user wants to know about a `!` to the hover of the function it calls: what
// the call becomes, and what happens when the error is not nil.
func (q *query) bangHover(h *hoverResult, obj types.Object, c *bangCall) {
	if h == nil || c == nil {
		return
	}
	b := q.bang()
	var lines []string
	if c.implicit {
		lines = append(lines, "No `!` is written here: `"+c.fname+"` is a must function, so the call that drops its error is handled for you.")
	} else {
		lines = append(lines, "`!` takes the error out of the call.")
	}
	if what := c.behavior(); what != "" {
		lines = append(lines, what)
	}
	if fn, ok := obj.(*types.Func); ok {
		if sig, ok := fn.Type().(*types.Signature); ok && sig.Results().Len() > 0 {
			res := sig.Results()
			var vals []string
			for i := 0; i < res.Len()-1; i++ {
				vals = append(vals, types.TypeString(res.At(i).Type(), q.qualifier()))
			}
			switch len(vals) {
			case 0:
				lines = append(lines, "The call gives no value besides the error.")
			case 1:
				lines = append(lines, "The call gives one value: `"+vals[0]+"`.")
			default:
				lines = append(lines, "The call gives the values `"+strings.Join(vals, "`, `")+"`.")
			}
		}
	}
	h.text = joinParas(h.text, strings.Join(lines, " "))
	if s := b.stmtOf(c); s != nil {
		if code := printStmts(q.cp(), s.Lowered, 24); code != "" {
			h.expansion = "// the code that stands for this statement (names with must_ belong to the compiler)\n" + code
		}
	}
}

func joinParas(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "\n\n" + b
}

// mustFuncNote returns the sentence for a must function declared as written in text, or "".
func mustFuncNote(written string) string {
	return "A must function, declared `" + written + "`: the errors it drops are returned, so callers see the signature above (`!` on a call, `F!(...)`, returns the error of one call)."
}

// mustDecl returns the declaration of the must function fd as the user wrote it, without the body.
func mustDecl(fset *token.FileSet, fd *ast.FuncDecl) string {
	d := *fd
	d.Body, d.Doc = nil, nil
	return strings.TrimSpace(formatNode(fset, &d))
}

// macroUseHover adds the hidden arguments and the expansion of a macro use to the hover of the macro.
func (q *query) macroUseHover(h *hoverResult, id *ast.Ident) {
	if h == nil || q.lp.MacroUses == nil {
		return
	}
	mu := q.lp.MacroUses[id]
	if mu == nil || mu.Call == nil || mu.Sig == nil {
		return
	}
	sig, call := mu.Sig, mu.Call
	nh := 3 + len(sig.Text)
	if sig.Wrap {
		nh++
	}
	var notes []string
	if nh <= len(call.Args) {
		names := []string{"#file()", "#line()", "#func()"}
		for _, t := range sig.Text {
			names = append(names, "#text("+t+")")
		}
		var parts []string
		for i, a := range call.Args[:min(nh, 3+len(sig.Text))] {
			val := printExpr(q.cp(), a)
			if id, ok := a.(*ast.Ident); ok && strings.HasPrefix(id.Name, "gp_") {
				val = "(of the outer use)"
			}
			parts = append(parts, "`"+names[i]+"` = `"+val+"`")
		}
		notes = append(notes, "Site arguments the compiler passes (the macro reads them with the built-ins): "+strings.Join(parts, ", ")+".")
	}
	if sig.Wrap {
		notes = append(notes, "A wrap macro: the code of the function it stands on is passed as `#body()`.")
	}
	for _, p := range sig.Params {
		if p.Lazy {
			notes = append(notes, "`"+p.Name+"` is lazy: the argument is not evaluated here; the macro calls `"+p.Name+"()` when, and if, it needs the value.")
		}
	}
	h.text = joinParas(h.text, strings.Join(notes, "\n\n"))
	h.expansion = "// the call that the use becomes (`gocpp expand` shows the whole function)\n" + q.macroCallText(call)
}

// macroCallText prints the call that stands for a macro use; a func literal argument that is long is
// shortened.
func (q *query) macroCallText(call *ast.CallExpr) string {
	var b strings.Builder
	fun := printExpr(q.cp(), call.Fun)
	b.WriteString(fun + "(")
	for i, a := range call.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		text := printExpr(q.cp(), a)
		if lit, ok := a.(*ast.FuncLit); ok && (strings.Contains(text, "\n") || len(text) > 80) {
			text = printExpr(q.cp(), lit.Type) + " { ... }"
		}
		b.WriteString(text)
	}
	b.WriteString(")")
	return b.String()
}

// bangHints adds the hints for the calls that a `!` handles between the offsets lo and hi: where the
// error goes. A call that the user wrote with a `!` gets "→ return err" after its closing parenthesis;
// a call that a must function handles without one gets the same with a `!` before the arrow, the `!`
// that is not in the text.
func (q *query) bangHints(hints []protocol.InlayHint, lo, hi int) []protocol.InlayHint {
	b := q.bang()
	if b == nil {
		return hints
	}
	first := q.tok.Pos(0)
	last := q.tok.Pos(q.tok.Size())
	// The calls are sorted by Pos, and the ones of this file are together.
	i := sort.Search(len(b.calls), func(i int) bool { return b.calls[i].Pos >= first })
	for ; i < len(b.calls) && b.calls[i].Pos <= last; i++ {
		c := b.calls[i]
		if c.End < first || c.End > last {
			continue
		}
		end := q.tok.Offset(c.End)
		if end < lo || end > hi {
			continue
		}
		label := c.short()
		if c.implicit {
			label = "!" + label
		}
		hints = append(hints, protocol.InlayHint{
			Position:    q.mapper.Position(end),
			Label:       []protocol.InlayHintLabelPart{{Value: label}},
			Tooltip:     &protocol.OrPTooltip_textDocument_inlayHint{Value: protocol.MarkupContent{Kind: protocol.Markdown, Value: c.tooltip()}},
			PaddingLeft: true,
		})
	}
	return hints
}

// phase10Tokens gives addAt the tokens of the forms of Phase 10 that are not names of the checked tree: the
// ! of a must call and of a must function, the # of a qualified macro use (#util.Assert), lazy in the
// parameters of a macro, and #Derive.
func (q *query) phase10Tokens(addAt func(off, end, typ int, mods uint32, text string)) {
	first, last := q.tok.Pos(0), q.tok.Pos(q.tok.Size())
	if b := q.bang(); b != nil {
		i := sort.Search(len(b.calls), func(i int) bool { return b.calls[i].Pos >= first })
		for ; i < len(b.calls) && b.calls[i].Pos <= last; i++ {
			if c := b.calls[i]; c.Bang.IsValid() {
				off := q.tok.Offset(c.Bang)
				addAt(off, off+1, tokOperator, 0, "!")
			}
		}
		for _, f := range b.funcs {
			if f.Bang >= first && f.Bang <= last {
				off := q.tok.Offset(f.Bang)
				addAt(off, off+1, tokOperator, modDeclaration, "!")
			}
		}
	}
	for id, mu := range q.lp.MacroUses {
		if mu.Call == nil || id.Pos() < first || id.Pos() > last {
			continue
		}
		fun := mu.Call.Fun
		for {
			if ix, ok := fun.(*ast.IndexExpr); ok {
				fun = ix.X
			} else if ix, ok := fun.(*ast.IndexListExpr); ok {
				fun = ix.X
			} else {
				break
			}
		}
		if sel, ok := fun.(*ast.SelectorExpr); ok && sel.X.Pos() >= first {
			if off := q.tok.Offset(sel.X.Pos()); off > 0 {
				addAt(off-1, off, tokMacro, 0, "#")
			}
		}
	}
	for _, d := range q.file.AST.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name == nil || q.lp.Macros[fd.Name.Name] == nil || fd.Type.Params == nil || !fd.Type.Func.IsValid() {
			continue
		}
		if off := q.tok.Offset(fd.Type.Func); !bytes.HasPrefix(q.src[off:], []byte("macro")) {
			continue
		}
		for _, field := range fd.Type.Params.List {
			if len(field.Names) == 0 {
				continue
			}
			name := field.Names[len(field.Names)-1]
			if !name.Pos().IsValid() || strings.HasPrefix(name.Name, "gp_") {
				continue
			}
			off := q.tok.Offset(name.End())
			for off < len(q.src) && (q.src[off] == ' ' || q.src[off] == '\t') {
				off++
			}
			if rest := q.src[off:]; bytes.HasPrefix(rest, []byte("lazy")) && len(rest) > 4 && (rest[4] == ' ' || rest[4] == '\t') {
				addAt(off, off+4, tokKeyword, 0, "lazy")
			}
		}
	}
	for _, d := range q.lp.Derives {
		if d.File != q.file || !d.Pos.IsValid() {
			continue
		}
		off := q.tok.Offset(d.Pos)
		addAt(off, off+len("#Derive"), tokMacro, modDefaultLibrary, "#Derive")
	}
}

// deriveHover returns the hover of the `#Derive` of a #Derive line when the place of q is on it: the
// generators of the line and the methods they write for the type.
func (q *query) deriveHover() (*hoverResult, protocol.Range) {
	for _, d := range q.lp.Derives {
		if d.File != q.file || !d.Pos.IsValid() {
			continue
		}
		end := d.Pos + token.Pos(len("#Derive"))
		if q.pos < d.Pos || q.pos > end {
			continue
		}
		var gens, methods []string
		qual := q.qualifier()
		for _, g := range d.Gens {
			gens = append(gens, g.Name)
			for _, fd := range g.Funcs {
				if fn, ok := q.info.Defs[fd.Name].(*types.Func); ok {
					methods = append(methods, "`"+q.objectString(fn, qual, nil)+"`")
				}
			}
		}
		h := &hoverResult{
			signature: "#Derive(" + strings.Join(gens, ", ") + ")",
			text:      "Writes methods for the type `" + d.Spec.Name.Name + "` (`gocpp expand` shows their bodies). They are written from the fields of the type, so they follow it when it changes.",
		}
		if len(methods) > 0 {
			h.text += "\n\nMethods: " + strings.Join(methods, ", ") + "."
		}
		return h, q.rangeOf(d.Pos, end)
	}
	return nil, protocol.Range{}
}
