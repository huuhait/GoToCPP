package check

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
)

// Gap E1 of typed interop (Phase 12, step 12.8, ADR-097): C/C++ constants in constant
// contexts.
//
// Stock go/types cannot evaluate cstdio.BUFSIZ: the loader rewrote it to C.BUFSIZ, which
// FakeImportC makes opaque, so `var buf [cstdio.BUFSIZ]byte` has no length, `const n =
// math.M_PI * 2` has no value and `case cstdio.EOF:` is compared with nothing. Check cannot
// ask the compiler itself (internal/typedinterop imports this package), so the driver gives
// it Config.Constants. Before go/types runs, substituteConstants finds the C/C++ names that sit
// where a constant is needed, asks the function for their compile-time values (one batch per
// package) and replaces each name in the syntax tree with the value as a real untyped constant at
// the same position:
//
//	[cstdio.BUFSIZ]byte        ->  [8192]byte
//	const n = cmath.M_PI * 2   ->  const n = 0x1.921fb54442d18p+01 * 2
//	case cstdio.EOF:           ->  case -1:
//
// Then stock go/types sees ordinary constants. The places (constCtx) are: an array length, the
// value of a const declaration (both need a constant: a name without one is an error, with the
// compiler's reason), a case of an expression switch and the count of a shift (Go allows a
// variable there, so a value that does not fit the context is not an error: the substitution is
// taken back, and the name stays opaque as in opaque mode). A name that is not a constant stays
// as it was, so does every name outside these places, and a package without the function is
// checked as before.
//
// Only what is a constant expression in the operand of the place is replaced: names, the
// parentheses, unary and binary operators around them and the arguments of a conversion to a Go
// type or of len, cap, min and max. The argument of unsafe.Sizeof is the C++ value itself and
// the arguments of a call of a C/C++ function are C++'s, so neither is looked into.

// A ConstKind says what kind of value a C/C++ constant has.
type ConstKind int

const (
	// ConstNone: the name has no constant value that go/types could use (ConstAnswer.Why says why).
	ConstNone   ConstKind = iota
	ConstInt              // an integer, also an enumerator (Int)
	ConstFloat            // a finite floating-point number (Float, the exact double)
	ConstString           // a string (Str)
	ConstBool             // true or false (Int is 0 or 1)
	ConstChar             // a character: a char literal of C++ (Int is its code)
)

// A CppPath is a C/C++ name as the loader rewrote it: the header package or the namespace it came
// through and the names after it.
type CppPath struct {
	// Ns is the namespace ("::std") of a name through a namespace, "" for a name through a header
	// package.
	Ns string
	// Hdr is the name of the header package (cstdio in cstdio.BUFSIZ), "" for a namespace.
	Hdr string
	// Parts are the names after the header package or the namespace.
	Parts []string
}

// Key returns the text that identifies the path in a ConstQuery's answers.
func (p CppPath) Key() string {
	return p.Ns + "\x00" + p.Hdr + "\x00" + strings.Join(p.Parts, "\x00")
}

// A ConstAnswer is the value of one C/C++ name.
type ConstAnswer struct {
	Kind  ConstKind
	Int   *big.Int
	Float float64
	Str   string

	// NotConst says (when Kind is ConstNone) that the compiler evaluated the name and it is no
	// constant, or has no value in Go: used in an array length or a const declaration it is an
	// error with the reason Why. Without NotConst the answer is "not known" (the name is
	// undefined, the headers do not compile, the probe has no answer) and the name is left as it
	// is: the checks that already exist report what is wrong.
	NotConst bool
	Why      string

	// Spelling is the C++ expression of the name in the program's translation unit (`EOF`,
	// `::kMax`, `::geo::kDim`, `(::kPt).x`), from the probe: the guard of ADR-097 §7 compiles it.
	Spelling string
}

// A ConstQuery is what Config.Constants is asked: the values of Paths in the translation unit of
// one package.
type ConstQuery struct {
	// Dir is the package folder, Flags the probe flags of the package, Includes the #include texts
	// of its header imports (with the extern "C" mark of the build), as the namespace probe has them.
	Dir      string
	Flags    []string
	Includes []string

	// GOOS and GOARCH are the target of the build (the host's when empty): values of headers are
	// those of the machine that runs the compiler, so a function that probes only this machine
	// answers nothing for another target.
	GOOS, GOARCH string

	Paths []CppPath
}

// A ConstFunc returns the values of the paths of a query, by CppPath.Key. A path that is not in the
// result is not known. An error leaves every path as it is (Check adds a warning), except that a
// canceled context ends the work quietly. The function has to be safe for concurrent use.
type ConstFunc func(ctx context.Context, q ConstQuery) (map[string]ConstAnswer, error)

// A ConstUse is a substitution that was made: the name the user wrote, where, and the constant.
type ConstUse struct {
	Pos     token.Position
	At      token.Pos // Pos as a position of the program's file set
	Written string    // cstdio.BUFSIZ
	Path    CppPath
	Value   string // the constant as Go source: 8192, -1, 0x1.8p+00, "text", 'a', true
	Where   string // "array length", "constant declaration", "case", "shift count"

	// Answer is what the probe said (the kind and the exact value), and Spelling the C++ expression
	// of the name: the lowering writes a static_assert for each, so that a compiler that sees another
	// value than the probe's (a header that branches on __clang__) is an error, not a wrong program.
	Answer   ConstAnswer
	Spelling string
}

// constCtx is the place of a name.
type constCtx int

const (
	ctxArrayLen  constCtx = iota + 1 // [N]T
	ctxConstDecl                     // const n = N
	ctxCase                          // case N: of an expression switch
	ctxShift                         // x << N, x >>= N
)

func (c constCtx) String() string {
	switch c {
	case ctxArrayLen:
		return "array length"
	case ctxConstDecl:
		return "constant declaration"
	case ctxCase:
		return "case"
	case ctxShift:
		return "shift count"
	}
	return "?"
}

// hard reports whether a name here must be a constant (else it is an error).
func (c constCtx) hard() bool { return c == ctxArrayLen || c == ctxConstDecl }

// duplicateCase is the go/types error code of a duplicate case: a real error of the program, not a
// consequence of a substitution.
const duplicateCase = 113

// A constRoot is the expression of one place; all substitutions inside it stand or fall together.
type constRoot struct {
	span       ast.Expr // the expression whose errors belong to the place (the whole shift for a count)
	nodes      []nodeText
	fixes      []nameFix
	sole       bool // the span is one name, perhaps in parentheses
	file       string
	start, end int // byte offsets of the expression before any substitution
	pos, endP  token.Pos
	ctx        constCtx
	items      []*substItem
	reverted   bool
}

// A nodeText is a composite node of a place with its text before the substitution.
type nodeText struct {
	node ast.Expr
	orig string
}

// A nameFix puts the user's text back in a message: from is the text of a node after the
// substitution, to the text before.
type nameFix struct{ from, to string }

// A substItem is one name in a place.
type substItem struct {
	slot    *ast.Expr // where the name hangs in the tree
	orig    ast.Expr
	repl    ast.Expr // nil when not replaced
	path    CppPath
	written string
	lit     string // repl as Go source
	ans     ConstAnswer
	root    *constRoot
}

// A constSubst is the substitutions of one package.
type constSubst struct {
	fset  *token.FileSet
	items []*substItem
	roots []*constRoot
}

// substituteConstants replaces the C/C++ names in the constant places of lp with their values
// (see above). It returns nil when there is nothing to do, and the diagnostics that belong to the
// package: an error for a name that must be a constant and is not, a warning when the values could
// not be asked.
func substituteConstants(ctx context.Context, fset *token.FileSet, lp *load.Package, fl packageFlags, cfg Config) (*constSubst, []diag.Diagnostic) {
	if cfg.Constants == nil || lp.Std || lp.Incomplete {
		return nil, nil
	}
	f := &constFinder{fset: fset, uses: make(map[*ast.SelectorExpr]*load.CppUse), seen: make(map[*ast.Expr]bool)}
	includes := unitIncludes(lp, fl.externC)
	if len(includes) == 0 {
		return nil, nil
	}
	for _, file := range lp.Files {
		if file.Rewrite == nil {
			continue
		}
		clear(f.uses)
		for _, u := range file.Rewrite.Uses {
			if !u.Bare && u.Sel != nil && (u.Kind == load.HeaderUse || u.Kind == load.NamespaceUse) {
				f.uses[u.Sel] = u
			}
		}
		if len(f.uses) == 0 {
			continue
		}
		f.file = file.AST
		f.walk(file.AST)
	}
	if len(f.items) == 0 {
		return nil, nil
	}
	q := ConstQuery{Dir: lp.Dir, Flags: fl.probe, Includes: includes, GOOS: cfg.GOOS, GOARCH: cfg.GOARCH}
	seen := make(map[string]bool)
	for _, it := range f.items {
		if k := it.path.Key(); !seen[k] {
			seen[k] = true
			q.Paths = append(q.Paths, it.path)
		}
	}
	slices.SortFunc(q.Paths, func(a, b CppPath) int { return strings.Compare(a.Key(), b.Key()) })
	answers, err := cfg.Constants(ctx, q)
	var out []diag.Diagnostic
	if err != nil {
		if ctx.Err() == nil {
			first := f.items[0]
			out = append(out, diag.Diagnostic{Pos: fset.Position(first.orig.Pos()), Severity: diag.Warning,
				Msg: "cannot evaluate the C/C++ constants of this package: " + err.Error() + " (they stay opaque)"})
		}
		return nil, out
	}
	cs := &constSubst{fset: fset, items: f.items, roots: f.roots}
	f.snapshot()
	for _, it := range f.items {
		a, ok := answers[it.path.Key()]
		if !ok {
			continue
		}
		pos := it.orig.Pos()
		if a.Kind != ConstNone {
			if repl, lit, ok := constExpr(a, pos); ok {
				it.repl, it.lit, it.ans = repl, lit, a
				*it.slot = repl
				continue
			}
		}
		if a.NotConst && it.root.ctx.hard() {
			why := a.Why
			if why == "" {
				why = "it has no compile-time value"
			}
			var msg string
			if it.root.ctx == ctxArrayLen {
				msg = fmt.Sprintf("array length %s must be constant: %s", it.written, why)
			} else {
				msg = fmt.Sprintf("%s is not constant: %s", it.written, why)
			}
			out = append(out, diag.Diagnostic{Pos: fset.Position(pos), End: fset.Position(it.orig.End()), Severity: diag.Error, Msg: msg})
			// The error is the message of the name: go/types and the phase gate say nothing more
			// about a node they cannot look into.
			*it.slot = &ast.BadExpr{From: it.orig.Pos(), To: it.orig.End()}
		}
	}
	for _, r := range f.roots {
		r.buildFixes()
	}
	replaced := slices.ContainsFunc(f.items, func(it *substItem) bool { return it.repl != nil })
	if !replaced && len(out) == 0 {
		return nil, nil
	}
	return cs, out
}

// uses returns the substitutions that stand, for Package.Consts.
func (cs *constSubst) uses() []ConstUse {
	if cs == nil {
		return nil
	}
	var out []ConstUse
	for _, it := range cs.items {
		if it.repl == nil || it.root.reverted {
			continue
		}
		out = append(out, ConstUse{Pos: cs.fset.Position(it.orig.Pos()), At: it.orig.Pos(), Written: it.written, Path: it.path, Value: it.lit,
			Where: it.root.ctx.String(), Answer: it.ans, Spelling: it.ans.Spelling})
	}
	return out
}

// revert takes back the substitutions in the places of the soft kind (a case, a shift count) where
// the check found an error: the name there may be fine for the C++ compiler and not for Go's
// constant rules (a case of -1 in a switch on a byte, a negative shift count), and typed mode must
// not reject what opaque mode accepts. It reports whether it changed the tree. A duplicate case is
// a mistake of the program, kept.
func (cs *constSubst) revert(errs *diag.List) bool {
	if cs == nil {
		return false
	}
	changed := false
	for _, d := range errs.Diagnostics() {
		if d.Severity != diag.Error || d.Code == duplicateCase {
			continue
		}
		for _, r := range cs.roots {
			if r.reverted || r.ctx.hard() || r.file != d.Pos.Filename || d.Pos.Offset < r.start || d.Pos.Offset >= max(r.end, r.start+1) {
				continue
			}
			if !slices.ContainsFunc(r.items, func(it *substItem) bool { return it.repl != nil }) {
				continue
			}
			for _, it := range r.items {
				if it.repl != nil {
					*it.slot = it.orig
				}
			}
			r.reverted, changed = true, true
		}
	}
	return changed
}

// fixAt puts the names the user wrote back into a message of go/types about a substituted place:
// "invalid array length -1 (untyped int constant)" becomes "invalid array length cstdio.EOF
// (untyped int constant -1)", and "cannot use 5 - 5 - 1 (...)" becomes "cannot use 5 - lib.A - 1
// (...)": by the text of the nodes of the place, not by the value alone, which can also be the
// user's own literal.
func (cs *constSubst) fixAt(pos token.Pos, msg string) string {
	if cs == nil {
		return msg
	}
	for _, r := range cs.roots {
		if r.reverted || pos < r.pos || pos >= max(r.endP, r.pos+1) {
			continue
		}
		if r.sole {
			for _, it := range r.items {
				if it.repl != nil {
					msg = putName(msg, it.lit, it.written)
				}
			}
			continue
		}
		msg = applyFixes(msg, r.fixes)
	}
	return msg
}

// buildFixes computes the texts that fixAt puts back, once the names are replaced: every composite
// node of the place whose text changed.
func (r *constRoot) buildFixes() {
	r.fixes = nil
	r.sole = len(r.items) == 1 && ast.Unparen(r.span) == r.items[0].orig
	for _, n := range r.nodes {
		if now := types.ExprString(n.node); now != n.orig {
			r.fixes = append(r.fixes, nameFix{now, n.orig})
		}
	}
	slices.SortStableFunc(r.fixes, func(a, b nameFix) int { return len(b.from) - len(a.from) })
}

// applyFixes replaces, left to right and longest first, the texts of the fixes in msg where they
// are whole words.
func applyFixes(msg string, fixes []nameFix) string {
	if len(fixes) == 0 {
		return msg
	}
	var b strings.Builder
	for i := 0; i < len(msg); {
		hit := false
		for _, f := range fixes {
			if !strings.HasPrefix(msg[i:], f.from) {
				continue
			}
			end := i + len(f.from)
			if wordByte(msg, i-1) && isWord(f.from[0]) || wordByte(msg, end) && isWord(f.from[len(f.from)-1]) || end < len(msg) && msg[end] == '.' && isWord(f.from[len(f.from)-1]) {
				continue
			}
			b.WriteString(f.to)
			i, hit = end, true
			break
		}
		if !hit {
			b.WriteByte(msg[i])
			i++
		}
	}
	return b.String()
}

// putName replaces the first whole occurrence of lit in msg with name, and makes the operand
// "name (untyped K constant)" say its value. It is for a place that is one name: no literal of the
// user is there to be mistaken for the value.
func putName(msg, lit, name string) string {
	from := 0
	for {
		i := strings.Index(msg[from:], lit)
		if i < 0 {
			return msg
		}
		i += from
		end := i + len(lit)
		if wordByte(msg, i-1) && isWord(lit[0]) || wordByte(msg, end) && isWord(lit[len(lit)-1]) || end < len(msg) && msg[end] == '.' && isWord(lit[len(lit)-1]) {
			from = i + 1
			continue
		}
		rest := msg[end:]
		if strings.HasPrefix(rest, " (untyped ") {
			if j := strings.Index(rest, " constant)"); j > 0 {
				rest = rest[:j] + " constant " + lit + ")" + rest[j+len(" constant)"):]
			}
		}
		return msg[:i] + name + rest
	}
}

func isWord(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80
}

// wordByte reports whether msg[i] exists and is a word byte.
func wordByte(msg string, i int) bool { return i >= 0 && i < len(msg) && isWord(msg[i]) }

// unitIncludes returns the #include texts of the header imports of lp, each once, in file and
// source order, with the extern "C" mark of the build (typedinterop.UnitIncludes).
func unitIncludes(lp *load.Package, externC map[string]bool) []string {
	var out []string
	seen := make(map[string]bool)
	for _, f := range lp.Files {
		if f.Info == nil {
			continue
		}
		for _, inc := range f.Info.Includes {
			t := inc.IncludeText()
			if externC[t] {
				t = interop.ExternC(t)
			}
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// A constFinder finds the C/C++ names in the constant places of one package.
type constFinder struct {
	fset  *token.FileSet
	file  *ast.File
	uses  map[*ast.SelectorExpr]*load.CppUse
	seen  map[*ast.Expr]bool
	items []*substItem
	roots []*constRoot
}

// walk visits the places of a file.
func (f *constFinder) walk(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ArrayType:
			if x.Len != nil {
				f.place(&x.Len, ctxArrayLen)
			}
		case *ast.GenDecl:
			if x.Tok == token.CONST {
				for _, spec := range x.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for i := range vs.Values {
							f.place(&vs.Values[i], ctxConstDecl)
						}
					}
				}
			}
		case *ast.SwitchStmt:
			// (A type switch is a TypeSwitchStmt: its cases are types.)
			if x.Body != nil {
				for _, c := range x.Body.List {
					if cc, ok := c.(*ast.CaseClause); ok {
						for i := range cc.List {
							f.place(&cc.List[i], ctxCase)
						}
					}
				}
			}
		case *ast.BinaryExpr:
			if x.Op == token.SHL || x.Op == token.SHR {
				f.placeIn(&x.Y, ctxShift, x) // an error of the shifted expression belongs to the place
			}
		case *ast.AssignStmt:
			if (x.Tok == token.SHL_ASSIGN || x.Tok == token.SHR_ASSIGN) && len(x.Rhs) == 1 {
				f.placeIn(&x.Rhs[0], ctxShift, x.Rhs[0])
			}
		}
		return true
	})
}

// place looks into the expression in slot, which is a constant place of kind ctx.
func (f *constFinder) place(slot *ast.Expr, ctx constCtx) { f.placeIn(slot, ctx, *slot) }

// placeIn is place for a place whose errors are reported on span (a bigger expression around slot).
func (f *constFinder) placeIn(slot *ast.Expr, ctx constCtx, span ast.Expr) {
	root := &constRoot{ctx: ctx, span: span}
	if span != nil {
		root.pos, root.endP = span.Pos(), span.End()
		p, q := f.fset.Position(root.pos), f.fset.Position(root.endP)
		root.file, root.start, root.end = p.Filename, p.Offset, q.Offset
	}
	f.expr(slot, root)
	if len(root.items) > 0 {
		ast.Inspect(span, func(n ast.Node) bool {
			switch n.(type) {
			case *ast.BinaryExpr, *ast.ParenExpr, *ast.UnaryExpr, *ast.CallExpr:
				root.nodes = append(root.nodes, nodeText{node: n.(ast.Expr)})
			}
			return true
		})
		f.roots = append(f.roots, root)
	}
}

// snapshot records the text of every composite node of the places as the user wrote it: the tree has
// the loader's C.NAME for a name, so each name is shown for a moment as the identifier of its written
// form.
func (f *constFinder) snapshot() {
	for _, it := range f.items {
		*it.slot = &ast.Ident{NamePos: it.orig.Pos(), Name: it.written}
	}
	for _, r := range f.roots {
		for i := range r.nodes {
			r.nodes[i].orig = types.ExprString(r.nodes[i].node)
		}
	}
	for _, it := range f.items {
		*it.slot = it.orig
	}
}

// expr collects the names in the constant expression in slot.
func (f *constFinder) expr(slot *ast.Expr, root *constRoot) {
	switch x := (*slot).(type) {
	case *ast.SelectorExpr:
		if f.seen[slot] {
			return
		}
		if path, written, ok := f.chain(x); ok {
			f.seen[slot] = true
			it := &substItem{slot: slot, orig: x, path: path, written: written, root: root}
			root.items = append(root.items, it)
			f.items = append(f.items, it)
		}
	case *ast.ParenExpr:
		f.expr(&x.X, root)
	case *ast.UnaryExpr:
		switch x.Op {
		case token.ADD, token.SUB, token.XOR, token.NOT:
			f.expr(&x.X, root)
		}
	case *ast.BinaryExpr:
		f.expr(&x.X, root)
		f.expr(&x.Y, root)
	case *ast.CallExpr:
		if f.constantCall(x) {
			for i := range x.Args {
				f.expr(&x.Args[i], root)
			}
		}
	}
}

// goConstFuncs are the predeclared functions and types whose call with constant arguments is
// constant: the conversions to the basic types, len, cap, min, max, real, imag and complex.
var goConstFuncs = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
	"rune": true, "byte": true, "string": true, "bool": true,
	"len": true, "cap": true, "min": true, "max": true, "real": true, "imag": true, "complex": true,
}

// constantCall reports whether the call c is a conversion to a basic Go type or a call of a
// predeclared function that is constant when its arguments are. A name of the file's own scope
// (id.Obj != nil: the parser resolved it) hides the predeclared one.
func (f *constFinder) constantCall(c *ast.CallExpr) bool {
	id, ok := ast.Unparen(c.Fun).(*ast.Ident)
	return ok && id.Obj == nil && goConstFuncs[id.Name]
}

// chain returns the C/C++ path of a selector chain that starts at a header package or a namespace
// (the same reading as the collector's), and the name as the user wrote it.
func (f *constFinder) chain(sel *ast.SelectorExpr) (CppPath, string, bool) {
	var names []string
	var cur ast.Expr = sel
	for {
		s, ok := cur.(*ast.SelectorExpr)
		if !ok {
			return CppPath{}, "", false
		}
		names = append(names, s.Sel.Name)
		if u := f.uses[s]; u != nil {
			slices.Reverse(names)
			p := CppPath{Parts: names}
			if u.Kind == load.NamespaceUse {
				p.Ns = u.Namespace
			} else {
				p.Hdr = u.Ident.Name
			}
			return p, u.Ident.Name + "." + strings.Join(names, "."), true
		}
		cur = s.X
	}
}

// constExpr returns the constant a as a syntax tree at pos, and as Go source.
func constExpr(a ConstAnswer, pos token.Pos) (ast.Expr, string, bool) {
	lit := func(kind token.Token, v string) ast.Expr { return &ast.BasicLit{ValuePos: pos, Kind: kind, Value: v} }
	neg := func(e ast.Expr) ast.Expr { return &ast.UnaryExpr{OpPos: pos, Op: token.SUB, X: e} }
	var e ast.Expr
	switch a.Kind {
	case ConstInt:
		if a.Int == nil {
			return nil, "", false
		}
		if a.Int.Sign() < 0 {
			e = neg(lit(token.INT, new(big.Int).Neg(a.Int).String()))
		} else {
			e = lit(token.INT, a.Int.String())
		}
	case ConstChar:
		if a.Int == nil {
			return nil, "", false
		}
		if a.Int.IsInt64() && a.Int.Int64() >= 0 && a.Int.Int64() <= utf8.MaxRune && utf8.ValidRune(rune(a.Int.Int64())) {
			e = lit(token.CHAR, strconv.QuoteRune(rune(a.Int.Int64())))
		} else if a.Int.Sign() < 0 {
			e = neg(lit(token.INT, new(big.Int).Neg(a.Int).String()))
		} else {
			e = lit(token.INT, a.Int.String())
		}
	case ConstBool:
		if a.Int == nil {
			return nil, "", false
		}
		e = &ast.Ident{NamePos: pos, Name: strconv.FormatBool(a.Int.Sign() != 0)}
	case ConstFloat:
		if math.IsNaN(a.Float) || math.IsInf(a.Float, 0) {
			return nil, "", false
		}
		s := floatLiteral(math.Abs(a.Float))
		if math.Signbit(a.Float) && a.Float != 0 {
			e = neg(lit(token.FLOAT, s))
		} else {
			e = lit(token.FLOAT, s)
		}
	case ConstString:
		e = lit(token.STRING, strconv.Quote(a.Str))
	default:
		return nil, "", false
	}
	return e, exprSource(e), true
}

// floatLiteral returns the exact value of the non-negative finite double f as a Go floating-point
// literal: decimal when the exact decimal expansion is short (3.0, 0.5, 1000.25), else hexadecimal
// (0x1.999999999999ap-04), which is exact for every double. A decimal such as 0.1 would be another
// number to go/types, which computes constants exactly: the macro is the double nearest to 0.1.
func floatLiteral(f float64) string {
	if f == 0 {
		return "0.0"
	}
	dec := new(big.Float).SetFloat64(f).Text('f', 1100)
	if strings.Contains(dec, ".") {
		dec = strings.TrimRight(dec, "0")
		if strings.HasSuffix(dec, ".") {
			dec += "0"
		}
	}
	if len(dec) <= 20 {
		return dec
	}
	return strconv.FormatFloat(f, 'x', -1, 64)
}

// exprSource writes the substituted expressions (a literal, a negated literal, true or false) as Go.
func exprSource(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.BasicLit:
		return x.Value
	case *ast.Ident:
		return x.Name
	case *ast.UnaryExpr:
		return "-" + exprSource(x.X)
	}
	return ""
}

// context returns the context of the checks, never nil.
func (c Config) context() context.Context {
	if c.Context != nil {
		return c.Context
	}
	return context.Background()
}
