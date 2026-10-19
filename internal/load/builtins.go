package load

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/huuhait/gocpp/internal/syntax/gpast"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// The built-in macros of Phase 10 (step 10.8, README §2.2 "Built-in macros", ADR-058):
//
//	#file()        the file of the use, as a string: its path from the module root
//	               ("util/util.gp"), or the file name for a program without gocpp.mod
//	#line()        the line of the use, as an int
//	#func()        the function that holds the use, as the runtime prints it
//	               ("main.f", "main.(*T).M", "main.f.func1")
//	#text(x)       the source text of x, as a string; x is not evaluated and not checked
//	#embed("f")    the bytes of the file f, as a string
//	#body()        only inside a macro declaration (wrap style): step 10.9
//
// and //go:embed on a package-level variable of type string or []byte (embed.go).
//
// They are source rewrites that run in the loader, once the package's files
// are parsed and their imports are classified, because they need what only
// the loader has: the source text, the module, the file system. Each use is
// replaced by a constant written as plain Go, in parentheses so that the
// node keeps the span of the use:
//
//	#line()       ->  (12)
//	#file()       ->  ("main.gp")
//
// All of them are constants of Go (untyped), so a const declaration or an
// array length can use them. Every node keeps the position of the use: an
// error that go/types finds in the new code points at the text the user
// wrote, and the generated C++ maps to the use.
//
// The user macros (step 10.9, macros.go) are expanded by the same walk, because it
// knows the runtime name of the function that holds a use. Inside a macro
// declaration #file(), #line(), #func() and #text(x) mean the place where the
// macro is USED, so there they become the hidden parameters of the function
// that the macro turns into (gp_file, gp_line, gp_func, gp_text_x), and
// #body() a call of gp_body.

// builtinMacros expands the built-in macros and //go:embed in the files of
// p. It always runs (the syntax of Phase 10 is always on, ADR-095).
func (l *loader) builtinMacros(p *Package) {
	if p.Std {
		return // the standard library has none, and its files have no Src
	}
	b := &builtinPass{l: l, p: p, prefix: runtimePkgName(p)}
	b.initCtx = &funcName{name: b.prefix + ".init", top: true}
	b.wrapped = make(map[*ast.FuncDecl]bool)
	b.wrapRet = make(map[*ast.FuncDecl]bool)
	b.macroSigs()
	for _, f := range p.Files {
		b.file(f)
	}
}

// A builtinPass is the expansion of the built-in macros in one package.
type builtinPass struct {
	l *loader
	p *Package

	// prefix is the name of the package in the names of functions: main,
	// example.com/app/geom.
	prefix string

	// initCtx names the closures of the package-level initializers, which
	// the runtime calls main.init.func1, main.init.func2, ... across the
	// files of the package, and inits counts the func init() declarations
	// (main.init.0, main.init.1, ...).
	initCtx *funcName
	inits   int

	// embedTotal is the number of bytes embedded in the package so far.
	embedTotal int64

	// f is the file being expanded.
	f *File

	// stack holds the functions around the node being walked, the innermost
	// last.
	stack []*funcName

	// macro is the macro declaration whose body is being walked, or nil.
	macro *macroCtx

	// uses are the uses of user macros that the walk has seen and not yet
	// replaced; typeNames the package's type names (macros.go); defers
	// numbers the names of the func values that on-top macros defer.
	uses      map[*ast.CallExpr]*useInfo
	typeNames map[string]bool
	defers    int
	wrapped   map[*ast.FuncDecl]bool // the functions that a wrap macro has been applied to
	wrapRet   map[*ast.FuncDecl]bool // those whose body ends with the return that follows the wrap call

	// nerr counts the errors that the pass reported; errors are dropped while quiet is not 0.
	nerr  int
	quiet int

	// embedName is the local name of the import of "embed" in the file ("" if
	// there is none).
	embedName string

	// methods holds the methods of the package by "Type.Method" (where they are declared), and
	// pkgNames its package-level names: for #Derive (derive.go).
	methods    map[string]string
	pkgNames   map[string]token.Pos
	deriveSeen map[*ast.TypeSpec]map[string]bool
}

// A funcName is the name that the runtime gives a function, with the counter
// for the closures in it.
type funcName struct {
	name string
	top  bool // a declared function (or the initializers): its closures are name.func1, ...
	n    int
}

// closure returns the name of the next function literal in f: main.f.func1
// inside a declared function, main.f.func2.1 inside a literal.
func (f *funcName) closure() *funcName {
	f.n++
	if f.top {
		return &funcName{name: f.name + ".func" + strconv.Itoa(f.n)}
	}
	return &funcName{name: f.name + "." + strconv.Itoa(f.n)}
}

// runtimePkgName returns the name of package p in the names of functions
// that the runtime prints: main for a program, the import path otherwise,
// with the characters that the linker escapes (a dot in the last element
// is %2e).
func runtimePkgName(p *Package) string {
	if p.Name == "main" {
		return "main"
	}
	s := p.ImportPath
	slash := strings.LastIndex(s, "/")
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= ' ' || i >= slash && c == '.' || c == '%' || c == '"' || c >= 0x7f {
			fmt.Fprintf(&b, "%%%02x", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// funcDeclName returns the runtime name of the declared function d, and
// counts it if it is an init function.
func (b *builtinPass) funcDeclName(d *ast.FuncDecl) string {
	name := d.Name.Name
	if gpast.IsMust(name) {
		name = gpast.MustBase(name) // a must function is the function f in the program
	}
	if d.Recv == nil || len(d.Recv.List) == 0 {
		if name == "init" {
			name = "init." + strconv.Itoa(b.inits)
			b.inits++
		} else if d.Type.TypeParams != nil && len(d.Type.TypeParams.List) > 0 {
			name += "[...]"
		}
		return b.prefix + "." + name
	}
	// A method: main.T.M, main.(*T).M, and [...] for the type parameters of
	// the receiver's type.
	t := ast.Unparen(d.Recv.List[0].Type)
	star := false
	if s, ok := t.(*ast.StarExpr); ok {
		star = true
		t = ast.Unparen(s.X)
	}
	generic := false
	switch x := t.(type) {
	case *ast.IndexExpr:
		t, generic = x.X, true
	case *ast.IndexListExpr:
		t, generic = x.X, true
	}
	recv := "?"
	if id, ok := t.(*ast.Ident); ok {
		recv = id.Name
	}
	if generic {
		recv += "[...]"
	}
	if star {
		recv = "(*" + recv + ")"
	}
	return b.prefix + "." + recv + "." + name
}

// file expands the built-in macros and the user macros of one file.
func (b *builtinPass) file(f *File) {
	b.f = f
	b.embedName = ""
	for _, spec := range f.AST.Imports {
		if path, _ := importPath(spec); path == "embed" {
			b.embedName = "embed"
			if spec.Name != nil {
				b.embedName = spec.Name.Name
			}
		}
	}
	b.goEmbed()
	var decls []ast.Decl
	for i, d := range f.AST.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if gpast.IsMacroDecl(d) {
				b.macroDecl(d) // (lines of macros on top of it are a syntax error)
				decls = append(decls, d)
				continue
			}
			ctx := &funcName{name: b.funcDeclName(d), top: true}
			b.apply(d.Type, ctx)
			if d.Body != nil {
				b.apply(d.Body, ctx)
			}
			if calls := gpast.OnTopOf(f.AST.Decls, i); calls != nil {
				b.onTop(d, calls, ctx)
			}
		case *ast.GenDecl:
			if _, ok := gpast.OnTop(d); ok {
				continue // a line of macros: handled with the declaration below it (and dropped)
			}
			var stubs []ast.Decl
			if d.Tok == token.TYPE { // (on top of a var, const or import: a syntax error, reported once)
				if calls := gpast.OnTopOf(f.AST.Decls, i); calls != nil {
					stubs = b.deriveType(d, calls) // derive.go
				}
			}
			b.apply(d, b.initCtx)
			decls = append(decls, d)
			decls = append(decls, stubs...)
			continue
		}
		decls = append(decls, f.AST.Decls[i])
	}
	f.AST.Decls = decls
	b.leftovers(f)
}

// apply expands the built-in macros in the tree n, in place, where ctx is
// the function that holds n (nil: only check them, in a macro declaration).
// The root n is never a macro use itself.
func (b *builtinPass) apply(n ast.Node, ctx *funcName) {
	if n == nil || reflectNil(n) {
		return
	}
	b.stack = []*funcName{ctx}
	astutil.Apply(n, func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.FuncLit:
			b.stack = append(b.stack, b.stack[len(b.stack)-1].closure())
		case *ast.CallExpr:
			if root := useRoot(n.Fun); root != nil {
				if id, ok := n.Fun.(*ast.Ident); ok {
					if b.builtin(c, n, id) {
						return false
					}
					if base := gpast.MacroBase(id.Name); builtinNames[base] {
						if base == "Derive" {
							b.errorf(n.Pos(), "#Derive is for types: put it on top of a type declaration")
						}
						return true
					}
				}
				b.userMacro(c, n)
			}
		}
		return true
	}, func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.FuncLit:
			b.stack = b.stack[:len(b.stack)-1]
		case *ast.CallExpr:
			if b.uses != nil {
				b.finishUse(c, n)
			}
		}
		return true
	})
}

// reflectNil reports whether n is a nil pointer in a non-nil interface.
func reflectNil(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.FuncType:
		return n == nil
	case *ast.BlockStmt:
		return n == nil
	}
	return false
}

// funcNow returns the runtime name of the function that holds the node
// being walked.
func (b *builtinPass) funcNow() string { return b.stack[len(b.stack)-1].name }

// errorf reports an error at pos.
func (b *builtinPass) errorf(pos token.Pos, format string, args ...any) {
	if b.quiet > 0 {
		return
	}
	b.nerr++
	b.p.Errors.Errorf(b.l.fset.Position(pos), format, args...)
}

// builtin expands the use call of the macro id, if it is a built-in macro,
// and reports whether it did (then its arguments are not walked: they are
// not evaluated, or they are no expressions). A macro that is not built in
// is a user macro: its arguments are walked as usual, and reportMacros says
// that it is not supported yet.
func (b *builtinPass) builtin(c *astutil.Cursor, call *ast.CallExpr, id *ast.Ident) bool {
	name := gpast.MacroBase(id.Name)
	if call.Ellipsis.IsValid() && builtinNames[name] {
		b.errorf(call.Ellipsis, "#%s: cannot use ... in the arguments", name)
	}
	switch name {
	case "file", "line", "func":
		if len(call.Args) != 0 {
			b.errorf(call.Args[0].Pos(), "#%s takes no arguments", name)
		}
		if b.macro != nil {
			// The place of the use of the macro: a hidden parameter.
			b.replaceWith(c, call, b.hidden(map[string]string{"file": hiddenFile, "line": hiddenLine, "func": hiddenFunc}[name], call.Pos()))
			return true
		}
		switch name {
		case "file":
			b.replace(c, call, token.STRING, strconv.Quote(b.fileName(call.Pos())))
		case "line":
			b.replace(c, call, token.INT, strconv.Itoa(b.l.fset.Position(call.Pos()).Line))
		default:
			b.replace(c, call, token.STRING, strconv.Quote(b.funcNow()))
		}
		return true
	case "text":
		if len(call.Args) != 1 {
			b.errorf(call.Pos(), "#text takes one argument: #text(x)")
			b.replace(c, call, token.STRING, `""`)
			return true
		}
		if b.macro != nil {
			// #text(param) is the text that the use wrote for it.
			if a, ok := call.Args[0].(*ast.Ident); ok && b.macro.params[a.Name] && a.Name != "_" && a.Name != "" {
				b.replaceWith(c, call, b.hidden(hiddenText+a.Name, call.Pos()))
				return true
			}
		}
		b.replace(c, call, token.STRING, strconv.Quote(b.sourceText(call.Args[0])))
		return true
	case "embed":
		b.embedCall(c, call)
		return true
	case "body":
		if len(call.Args) != 0 {
			b.errorf(call.Args[0].Pos(), "#body takes no arguments")
		}
		if b.macro == nil {
			b.errorf(call.Pos(), "#body() can only be used inside a macro declaration")
			b.replace(c, call, token.INT, "0")
			return true
		}
		call.Fun = b.hidden(hiddenBody, id.NamePos)
		return false
	}
	return false
}

// replace replaces the use call by the constant of the kind and value val,
// in parentheses with the span of the call.
func (b *builtinPass) replace(c *astutil.Cursor, call *ast.CallExpr, kind token.Token, val string) {
	c.Replace(&ast.ParenExpr{
		Lparen: call.Pos(),
		X:      &ast.BasicLit{ValuePos: call.Pos(), Kind: kind, Value: val},
		Rparen: call.Rparen,
	})
}

// replaceWith replaces the use call by the expression x, in parentheses with the span of the call.
func (b *builtinPass) replaceWith(c *astutil.Cursor, call *ast.CallExpr, x ast.Expr) {
	c.Replace(&ast.ParenExpr{Lparen: call.Pos(), X: x, Rparen: call.Rparen})
}

// fileName returns what #file() gives at pos: the path of the file from
// the root of its module, with slashes, or the name of the file when it is
// in no module. A //line directive changes the name, as it does for the
// runtime.
func (b *builtinPass) fileName(pos token.Pos) string {
	return b.fileNameOf(b.l.fset.Position(pos).Filename)
}

// fileNameOf is fileName for the name of a file in the file set.
func (b *builtinPass) fileNameOf(name string) string {
	if name == "" {
		return "??" // `//line :500` has no file name; Go's runtime prints ??
	}
	if m := b.p.Module; m != nil && m.Dir != "" && filepath.IsAbs(name) {
		if rel, err := filepath.Rel(m.Dir, name); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	if filepath.IsAbs(name) {
		return filepath.Base(name)
	}
	return filepath.ToSlash(name)
}

// sourceText returns what #text(x) gives: the source text of x from its
// first token to its last, as the programmer wrote it, except that where x
// spans lines, the line break with its indentation, and each comment, become
// one space (so that an assertion message does not hold the layout of the
// code).
func (b *builtinPass) sourceText(x ast.Expr) string {
	src := b.f.Src
	file := b.l.fset.File(x.Pos())
	if file == nil || len(src) != file.Size() {
		return types.ExprString(x)
	}
	start, end := file.Offset(x.Pos()), file.Offset(x.End())
	if end > len(src) || start > end {
		return types.ExprString(x)
	}
	return normalizeText(src[start:end])
}

// normalizeText returns text with each run of white space that has a line
// break, and each comment, replaced by one space. Strings and every other
// token stay as they are.
func normalizeText(text []byte) string {
	fset := token.NewFileSet()
	tf := fset.AddFile("", -1, len(text))
	var s scanner.Scanner
	s.Init(tf, text, func(token.Position, string) {}, scanner.ScanComments|scanner.Phase10Syntax)
	var out strings.Builder
	prev := 0 // the end of the last token written
	gapBreaks := false
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		off := tf.Offset(pos)
		if tok == token.COMMENT {
			gapBreaks = true
			continue
		}
		if tok == token.SEMICOLON && (lit == "\n" || lit == "EOF") {
			continue // the semicolon that the scanner puts at a line end
		}
		if out.Len() > 0 {
			gap := text[prev:off]
			if gapBreaks || strings.ContainsAny(string(gap), "\r\n") {
				out.WriteByte(' ')
			} else {
				out.Write(gap)
			}
		}
		gapBreaks = false
		end := off + len(lit)
		switch {
		case lit == "":
			end = off + len(tok.String())
		case tok == token.STRING && text[off] == '`':
			// The scanner drops the \r of a raw string: find the real end.
			end = off + 1 + strings.IndexByte(string(text[off+1:]), '`') + 1
		}
		out.Write(text[off:min(end, len(text))])
		prev = min(end, len(text))
	}
	return out.String()
}

// exprName returns the name that a macro use calls, #util.Dbg.
func exprName(e ast.Expr) string { return types.ExprString(e) }
