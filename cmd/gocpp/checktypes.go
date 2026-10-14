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
// "opaque (<its type expression>)"; one that gets its type from an opaque
// value (k := cstdio.getchar()) has go/types' invalid type, and its uses
// are opaque; so has a parameter in the type of a function
// (func(w invalid type)).
func printTypes(w io.Writer, cwd string, fset *token.FileSet, p *check.Package) error {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	for _, f := range p.Load.Files {
		src, err := readSource(p.Load, f)
		if err != nil {
			return err
		}
		tp := &typePrinter{
			fset:   fset,
			info:   p.Info,
			opaque: p.Opaque,
			qual:   types.RelativeTo(p.Types),
			src:    src,
			name:   diag.ShortFileName(cwd, f.Path),
			skip:   namespaceRoots(f),
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
	w      io.Writer
}

// node prints the line of n, if it has one, for ast.Inspect.
func (tp *typePrinter) node(n ast.Node) bool {
	if id, ok := n.(*ast.Ident); ok {
		if obj := tp.info.Defs[id]; obj != nil {
			if kind := defKind(obj); kind != "" {
				t := types.TypeString(obj.Type(), tp.qual)
				if te := tp.opaque.TypeExpr(obj); te != nil {
					t = "opaque (" + tp.text(te) + ")"
				}
				tp.line(id, "def "+kind, t)
			}
			return false
		}
	}
	e, ok := n.(ast.Expr)
	if !ok || tp.skip[e] {
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
		t := types.TypeString(tv.Type, tp.qual)
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
