package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// printTypes prints, for "gocpp check -types", the type of every
// expression and every definition in the files of the checked package p,
// one per line in source order (an outer expression before the ones
// inside it), in four columns:
//
//	./main.gp:7:2   value     fmt.Println(n)      (n int, err error)
//	./main.gp:7:14  var       n                   int
//	./main.gp:8:7   value     cstring.strlen(p)   opaque
//	./main.gp:9:6   def var   w                   opaque (*SDL.SDL_Window)
//
// The columns are the position, the mode (as in the ExampleInfo example of
// go/types: void, type, builtin, nil, const, var, mapindex, commaok,
// value; "def <kind>" for a name that a declaration defines), the source
// text (on one line, cut after 40 characters) and the type, relative to
// the package, with " = <value>" for a constant.
//
// The type is "opaque" for an expression that the opaque pass (step 1.11,
// check.Package.Opaque) labels opaque: it comes from C or C++, and
// go/types has no complete type for it, so the C++ compiler checks it
// later. A definition whose declared type comes from C or C++ prints as
// "opaque (<its type expression>)", and one that gets its type from an
// opaque value (k := cstdio.getchar()) as "opaque". A function with C or
// C++ parts in its signature prints them the same way:
// func(w opaque (*SDL.SDL_Window)) int.
//
// A conversion of an opaque value, which Check rewrote to
// gp_cgoh.As[T](x) (check.Conversion), prints as written: T(x), with T
// and x; the nodes that the rewrite added are left out.
func printTypes(w io.Writer, cwd string, fset *token.FileSet, p *check.Package) error {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	hidden := hiddenNodes(p)
	for _, f := range p.Load.Files {
		src, err := readSource(p.Load, f)
		if err != nil {
			return err
		}
		tp := &typePrinter{
			fset:   fset,
			info:   p.Info,
			opaque: p.Opaque,
			qual:   cQualifier(p.Types),
			src:    src,
			name:   diag.ShortFileName(cwd, f.Path),
			skip:   namespaceRoots(f),
			hidden: hidden,
			w:      tw,
		}
		ast.Inspect(f.AST, tp.node)
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	return nil
}

// stdlibDir is the folder that the names of the files of the standard
// packages start with (load.Config.StdlibDir): the embedded standard
// library has no folder on the disk.
const stdlibDir = "$GOCPP/stdlib"

// readSource reads the file f of the package p: from the disk, or from
// the embedded standard library for a standard package.
func readSource(p *load.Package, f *load.File) ([]byte, error) {
	if !p.Std {
		return os.ReadFile(f.Path)
	}
	rel, ok := strings.CutPrefix(f.Path, stdlibDir+string(filepath.Separator))
	if !ok {
		return nil, fmt.Errorf("%s: not a file of the standard library", f.Path)
	}
	return fs.ReadFile(stdlib.FS(), filepath.ToSlash(rel))
}

// A typePrinter prints the types of one file.
type typePrinter struct {
	fset   *token.FileSet
	info   *types.Info
	opaque *check.Opaque
	qual   types.Qualifier
	src    []byte // the file's text
	name   string // the file's name, as printed
	skip   map[ast.Expr]bool
	hidden *hiddenSet
	w      io.Writer
}

// node prints the line of n, if it has one, for ast.Inspect.
func (tp *typePrinter) node(n ast.Node) bool {
	if id, ok := n.(*ast.Ident); ok {
		if obj := tp.info.Defs[id]; obj != nil {
			if kind := defKind(obj); kind != "" {
				tp.line(id, "def "+kind, tp.objType(obj))
			}
			return false
		}
	}
	e, ok := n.(ast.Expr)
	if !ok || tp.skip[e] || tp.hidden.nodes[n] {
		return n != nil
	}
	tv, recorded := tp.info.Types[e]
	switch {
	case tp.opaque.IsOpaque(e):
		m := "value" // go/types didn't record it
		if _, isVar := tp.info.Uses[asIdent(e)].(*types.Var); recorded {
			m = mode(tv)
		} else if isVar {
			m = "var" // v in v.size(), after v := std.vector[int]{}
		}
		tp.line(e, m, "opaque")
	case recorded:
		if typ, ok := tp.hidden.unused[e]; ok {
			tv.Type = tp.info.Types[typ].Type // gp_cgoh.Unused[T](x) is a [1]T
		}
		t := types.TypeString(tv.Type, tp.qual)
		if sig, ok := tv.Type.(*types.Signature); ok && incomplete(sig) {
			t = tp.signature(sig) // a builtin: append([]C.int, ...C.int)
		}
		switch {
		case tv.IsBuiltin() && tv.Type == types.Typ[types.Invalid]:
			t = "-" // go/types records no type for some builtins (len("ab"))
		case tv.Value != nil:
			t += " = " + tv.Value.String()
		}
		tp.line(e, mode(tv), t)
	}
	return true
}

// objType returns the type of the object obj that a definition defines,
// as printTypes prints it.
func (tp *typePrinter) objType(obj types.Object) string {
	if te := tp.opaque.TypeExpr(obj); te != nil {
		return "opaque (" + tp.text(te) + ")"
	}
	if sig, ok := obj.Type().(*types.Signature); ok && incomplete(sig) {
		return tp.signature(sig)
	}
	if incomplete(obj.Type()) {
		return "opaque"
	}
	return types.TypeString(obj.Type(), tp.qual)
}

// signature returns the type of a function with C or C++ parts, like
// types.TypeString (without the receiver), but each parameter or result
// of a type with C or C++ parts as "opaque", with its type expression.
func (tp *typePrinter) signature(sig *types.Signature) string {
	var b strings.Builder
	b.WriteString("func")
	if tps := sig.TypeParams(); tps.Len() > 0 {
		b.WriteString("[")
		for i := range tps.Len() {
			if i > 0 {
				b.WriteString(", ")
			}
			p := tps.At(i)
			b.WriteString(p.Obj().Name() + " " + types.TypeString(p.Constraint(), tp.qual))
		}
		b.WriteString("]")
	}
	b.WriteString(tp.tuple(sig.Params(), sig.Variadic()))
	switch res := sig.Results(); {
	case res.Len() == 1 && res.At(0).Name() == "":
		b.WriteString(" " + tp.varType(res.At(0), false))
	case res.Len() > 0:
		b.WriteString(" " + tp.tuple(res, false))
	}
	return b.String()
}

// tuple returns the parameters or results t in parentheses (see
// signature). variadic says that the last one is ...T.
func (tp *typePrinter) tuple(t *types.Tuple, variadic bool) string {
	parts := make([]string, t.Len())
	for i := range t.Len() {
		v := t.At(i)
		parts[i] = tp.varType(v, variadic && i == t.Len()-1)
		if v.Name() != "" {
			parts[i] = v.Name() + " " + parts[i]
		}
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// varType returns the type of the parameter or result v (see signature);
// dots says that v is the ...T of a variadic function.
func (tp *typePrinter) varType(v *types.Var, dots bool) string {
	if te := tp.opaque.TypeExpr(v); te != nil {
		return "opaque (" + tp.text(te) + ")" // with the ... of ...T
	}
	t := v.Type()
	prefix := ""
	if s, ok := t.(*types.Slice); ok && dots {
		t, prefix = s.Elem(), "..."
	}
	if incomplete(t) {
		return prefix + "opaque"
	}
	return prefix + types.TypeString(t, tp.qual)
}

// incomplete reports whether the type t has a part that go/types left
// invalid (a C or C++ type), as the opaque pass counts it: a named type is
// complete unless one of its type arguments is not.
func incomplete(t types.Type) bool {
	switch t := t.(type) {
	case *types.Basic:
		return t.Kind() == types.Invalid
	case *types.Alias:
		return incomplete(types.Unalias(t))
	case *types.Named:
		for i := range t.TypeArgs().Len() {
			if incomplete(t.TypeArgs().At(i)) {
				return true
			}
		}
	case *types.Pointer:
		return incomplete(t.Elem())
	case *types.Slice:
		return incomplete(t.Elem())
	case *types.Array:
		return incomplete(t.Elem())
	case *types.Chan:
		return incomplete(t.Elem())
	case *types.Map:
		return incomplete(t.Key()) || incomplete(t.Elem())
	case *types.Signature:
		return incomplete(t.Params()) || incomplete(t.Results())
	case *types.Tuple:
		for i := range t.Len() {
			if incomplete(t.At(i).Type()) {
				return true
			}
		}
	case *types.Struct:
		for i := range t.NumFields() {
			if incomplete(t.Field(i).Type()) {
				return true
			}
		}
	}
	return false
}

// A hiddenSet is what Check added to the files of a package when it
// rewrote the conversions of opaque values (check.Conversion).
type hiddenSet struct {
	// nodes are the new nodes: the gp_cgoh.As[T] of gp_cgoh.As[T](x) (but
	// T), the [0] around gp_cgoh.Unused[T](x), and the gp_cgoh.Value[T](x)
	// around the x of a deferred conversion (but x). printTypes leaves
	// them out, so that the conversion prints as the user wrote it.
	nodes map[ast.Node]bool

	// unused maps each call gp_cgoh.Unused[T](x) to T: the call has the
	// type [1]T, and printTypes prints T, the type of T(x).
	unused map[ast.Expr]ast.Expr
}

// hiddenNodes returns the hiddenSet of p.
func hiddenNodes(p *check.Package) *hiddenSet {
	h := &hiddenSet{nodes: make(map[ast.Node]bool), unused: make(map[ast.Expr]ast.Expr)}
	hide := func(root, keep ast.Node) {
		ast.Inspect(root, func(n ast.Node) bool {
			if n == nil || n == keep {
				return false
			}
			h.nodes[n] = true
			return true
		})
	}
	for _, c := range p.Conversions {
		switch c.Kind {
		case check.AsCall, check.UnusedStmt:
			hide(c.Call.Fun, c.Type)
			if c.Kind == check.UnusedStmt {
				h.unused[c.Call] = c.Type
			}
		case check.Compare:
			hide(c.Call.Fun, nil)          // the call stands for the comparison, a bool:
			h.nodes[c.Call.Args[0]] = true // one row for it, not the opaque one inside
		case check.Deferred:
			if inner, ok := c.Call.Args[0].(*ast.CallExpr); ok {
				h.nodes[inner] = true
				hide(inner.Fun, nil)
			}
		}
	}
	if len(h.unused) == 0 {
		return h
	}
	for _, f := range p.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if ix, ok := n.(*ast.IndexExpr); ok && h.unused[ix.X] != nil {
				h.nodes[ix] = true
				h.nodes[ix.Index] = true
			}
			return true
		})
	}
	return h
}

// line prints one line for the node n.
func (tp *typePrinter) line(n ast.Node, mode, typ string) {
	pos := tp.fset.Position(n.Pos())
	fmt.Fprintf(tp.w, "%s:%d:%d\t%s\t%s\t%s\n", tp.name, pos.Line, pos.Column, mode, tp.text(n), typ)
}

// maxText is the number of characters of an expression's source text
// that printTypes shows.
const maxText = 40

// text returns the source text of n on one line: every run of white space
// becomes one space, and a text longer than maxText characters is cut,
// with "..." at the end.
func (tp *typePrinter) text(n ast.Node) string {
	start, end := tp.fset.Position(n.Pos()).Offset, tp.fset.Position(n.End()).Offset
	if start < 0 || end > len(tp.src) || start > end {
		return "?"
	}
	s := strings.Join(strings.Fields(string(tp.src[start:end])), " ")
	if utf8.RuneCountInString(s) > maxText {
		r := []rune(s)
		s = string(r[:maxText-3]) + "..."
	}
	return s
}

// asIdent returns e if it is an identifier, else nil.
func asIdent(e ast.Expr) *ast.Ident {
	id, _ := e.(*ast.Ident)
	return id
}

// mode names the mode of an expression, like the ExampleInfo example of
// go/types.
func mode(tv types.TypeAndValue) string {
	switch {
	case tv.IsVoid():
		return "void"
	case tv.IsType():
		return "type"
	case tv.IsBuiltin():
		return "builtin"
	case tv.IsNil():
		return "nil"
	case tv.Value != nil:
		return "const"
	case tv.Assignable():
		if tv.Addressable() {
			return "var"
		}
		return "mapindex"
	case tv.HasOk():
		return "commaok"
	case tv.IsValue():
		return "value"
	}
	return "unknown"
}

// defKind names the kind of object that a definition defines, or returns
// "" for one that printTypes leaves out (a package name, a label).
func defKind(obj types.Object) string {
	switch obj := obj.(type) {
	case *types.Var:
		if obj.IsField() {
			return "field"
		}
		return "var"
	case *types.Const:
		return "const"
	case *types.TypeName:
		return "type"
	case *types.Func:
		return "func"
	}
	return ""
}

// namespaceRoots returns the selectors C.std that the loader made out of
// the namespace name std in std.vector (load.CppUse): printTypes leaves
// them out, as a namespace is not a value.
func namespaceRoots(f *load.File) map[ast.Expr]bool {
	skip := make(map[ast.Expr]bool)
	if f.Rewrite == nil {
		return skip
	}
	for _, u := range f.Rewrite.Uses {
		if u.Kind == load.NamespaceUse && u.Sel != nil {
			skip[u.Sel.X] = true
		}
	}
	return skip
}

// cQualifier is types.RelativeTo(pkg), but it names check's hidden package of the cgo helpers C,
// as the user writes the helpers' types: *C.char, C.int (check.CgoHelper).
func cQualifier(pkg *types.Package) types.Qualifier {
	rel := types.RelativeTo(pkg)
	return func(p *types.Package) string {
		if p.Path() == load.CgohPath {
			return "C"
		}
		return rel(p)
	}
}
