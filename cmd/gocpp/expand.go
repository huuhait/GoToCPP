package main

import (
	"bytes"
	"cmp"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/syntax/gpast"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/printer"
)

// expandUsage is the help text of "gocpp expand". "gocpp expand -h" prints it, followed by the flags.
const expandUsage = `usage: gocpp expand [-full] [-tags list] [file.gp... | package]

gocpp expand prints the plain Go that the compiler works with: the .gp
files of the package with the macros, #Derive lines and ! forms written
out, so that you can see what they do. It checks the package like gocpp
check (the same errors, with exit status 1) and prints to standard
output.

What it shows:

	#Name(args) uses      the call of the function that stands for the macro
	                      (its hidden arguments are the file, line and
	                      function of the use and the text of the arguments),
	                      a lazy argument as a func literal, an on-top line as
	                      the first statement, or the code of the function
	                      moved into a func literal for the wrap style
	macro declarations    the generic function that the compiler makes of them
	#Derive(...) lines    the methods, with their bodies
	f! functions, F!(x)   the explicit "if err != nil" code: must_e1, must_t1
	                      are the names of the temporaries; main! becomes
	                      must_main with a main that calls gp_must.Exit, which
	                      prints "<program>: <error>" and exits with 1

Every declaration that has none of these is printed as you wrote it,
byte for byte, comments included; a declaration that has one is printed
by gofmt's printer with its comments, and has the names above. Names that
start with gp_ and must_ belong to the compiler. A declaration that uses
C or C++ values, and has a macro or a ! in it, shows the compiler's own
forms for them (C.std.vector, gp_cgoh.As[int](x)).

The bytes of a file that #embed puts into a declaration that is printed
from the tree are shortened to their first 40 and a comment with the
length; -full prints them whole. A //go:embed variable is printed as you
wrote it.

The output is your file with the expanded declarations replaced: the rest
(comments, blank lines, spacing, line ends) is yours, byte for byte. With
more than one file, each file starts with a line "//// name" (slashes in
the path, also on Windows). The _test.gp files are not loaded, so they are
not printed. The output is Go-like text for reading, not a program that can
be compiled.
` + interopHelp + `The flags are:
`

// runExpand is "gocpp expand" (Phase 10 step 10.11).
func runExpand(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	c := &checkCommand{stdout: stdout, stderr: stderr}
	flags := flag.NewFlagSet("gocpp expand", flag.ContinueOnError)
	c.diagOpts.register(flags)
	flags.Var(tagsFlag{&c.tags}, "tags", "a comma-separated `list` of build tags that are satisfied, like go build -tags")
	var cross crossFlags
	cross.register(flags)
	c.interop.register(flags)
	full := false
	flags.BoolVar(&full, "full", false, "print the bytes of embedded files in full, not their first 40")
	if code, done := parseFlags("expand", expandUsage, flags, args, stderr); done {
		return code
	}
	var err error
	if c.goos, c.goarch, err = cross.resolve(); err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err)
		return 2
	}
	if c.dir, err = os.Getwd(); err != nil {
		fmt.Fprintf(stderr, "gocpp expand: %v\n", err)
		return exitErrors
	}
	var stop func()
	c.ctx, c.stopped, stop = signalContext()
	defer stop()
	// The check rewrites the files, so the nodes that the loader has made by now are the nodes of
	// the user's text for printExpanded (together with the text of the file, which tells what is a use).
	known := make(map[ast.Node]bool)
	c.afterLoad = func(prog *load.Program) {
		for _, root := range prog.Roots {
			for _, f := range root.Files {
				ast.Inspect(f.AST, func(n ast.Node) bool {
					if n != nil {
						known[n] = true
					}
					return true
				})
			}
		}
	}
	prog, r, code := c.check(flags.Args())
	if r == nil || code != 0 {
		return code
	}
	type unit struct {
		pkg  *check.Package
		file *load.File
	}
	var files []unit
	for _, root := range prog.Roots {
		if p := r.Package(root); p != nil && !c.dropped[root] {
			for _, f := range p.Load.Files {
				files = append(files, unit{p, f})
			}
		}
	}
	var b bytes.Buffer
	for _, u := range files {
		if len(files) > 1 {
			fmt.Fprintf(&b, "//// %s\n", relPath(c.dir, u.file.Path))
		}
		if expandTestHook != nil {
			expandTestHook(u.file)
		}
		if err := printExpanded(&b, prog.Fset, u.pkg, u.file, known, full); err != nil {
			fmt.Fprintf(stderr, "gocpp expand: %s: %v\n", relPath(c.dir, u.file.Path), err)
			return exitInternal
		}
	}
	if _, err := stdout.Write(b.Bytes()); err != nil {
		fmt.Fprintf(stderr, "gocpp expand: %v\n", err)
		return exitErrors
	}
	return 0
}

// relPath is path relative to dir, with slashes, when it is inside dir.
func relPath(dir, path string) string {
	if rel, err := filepath.Rel(dir, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

var printConfig = printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}

// expandTestHook, if not nil, is called by gocpp expand with each file before it is printed (tests).
var expandTestHook func(f *load.File)

// A srcDecl is a declaration of the source file as it is written: the byte offsets of its start (with its
// documentation) and of its end, whether it has macros, derive lines or ! in it, and whether it is a line of
// macros on top of a declaration.
type srcDecl struct {
	start, end int // with the documentation
	code       int // the start of the code: the func or type keyword, the "#" of a line of macros
	marked     bool
	onTop      bool
}

// rawEnd returns the offset in src where a token of n characters that starts at start ends. The parser
// takes the carriage returns out of comments and raw strings, so the length of their text is shorter than
// their place in a file with CRLF line ends; the carriage returns are not counted.
func rawEnd(src []byte, start, n int) int {
	i := start
	for ; n > 0 && i < len(src); i++ {
		if src[i] != '\r' {
			n--
		}
	}
	return i
}

// sourceDecls parses src again with the syntax of Phase 10 (the check has changed the trees, and a pass that
// is done leaves no mark in them) and returns its declarations.
func sourceDecls(src []byte) []srcDecl {
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, "expand.gp", src, parser.ParseComments|parser.Phase10Syntax|parser.SkipObjectResolution)
	if f == nil {
		return nil
	}
	tf := fset.File(f.Package)
	var out []srcDecl
	for _, d := range f.Decls {
		start := d.Pos()
		if doc := docOf(d); doc != nil {
			start = doc.Pos()
		}
		end := tf.Offset(d.Pos())
		ast.Inspect(d, func(n ast.Node) bool {
			if n == nil {
				return false
			}
			switch n := n.(type) {
			case *ast.Comment, *ast.CommentGroup:
				return false
			case *ast.BasicLit:
				end = max(end, rawEnd(src, tf.Offset(n.ValuePos), len(n.Value)))
			default:
				if e := n.End(); inFile(tf, e) {
					end = max(end, tf.Offset(e))
				}
			}
			return true
		})
		_, onTop := gpast.OnTop(d)
		out = append(out, srcDecl{tf.Offset(start), min(end, len(src)), tf.Offset(d.Pos()), len(gpast.Leftovers(d)) > 0, onTop})
	}
	return out
}

// lineStart, lineEnd and nextLine return the start of the line of offset off, the end of its text (before the
// line end) and the start of the next line.
func lineStart(src []byte, off int) int { return bytes.LastIndexByte(src[:off], '\n') + 1 }

func lineEnd(src []byte, off int) int {
	i := bytes.IndexByte(src[off:], '\n')
	if i < 0 {
		return len(src)
	}
	end := off + i
	if end > off && src[end-1] == '\r' {
		end--
	}
	return end
}

func nextLine(src []byte, off int) int {
	if i := bytes.IndexByte(src[off:], '\n'); i >= 0 {
		return off + i + 1
	}
	return len(src)
}

// lineBreak returns the line end that the line of off has: "\r\n" or "\n", and the first one of the file for
// the last line.
func lineBreak(src []byte, off int) string {
	if i := bytes.IndexByte(src[off:], '\n'); i >= 0 {
		if off+i > 0 && src[off+i-1] == '\r' {
			return "\r\n"
		}
		return "\n"
	}
	if bytes.Contains(src, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

// An edit replaces src[start:end] by text (an insertion has start == end).
type edit struct {
	start, end int
	text       string
	insert     bool // text is declarations to put in, not a replacement
}

// printExpanded writes the file f as Go source. The text of the file is kept as it is, byte for byte, except
// for the declarations that have a macro, a derive line or a ! in them, which are replaced by what they
// stand for (printed from the syntax tree, with their comments), the lines of #Derive, which are taken out
// when their methods are put after the type, and the declarations that the check wrote (the methods of
// #Derive, the main that calls must_main, the import of the compiler's own gp_must), which are put in after
// the declaration before them.
func printExpanded(w *bytes.Buffer, fset *token.FileSet, pkg *check.Package, f *load.File, known map[ast.Node]bool, full bool) error {
	file := f.AST
	tf := fset.File(file.Package)
	if tf == nil {
		return errors.New("no position for the package clause")
	}
	src := f.Src
	if src == nil {
		if src = readGP(f.Path); src == nil {
			return errors.New("cannot read the file")
		}
	}
	if len(src) != tf.Size() {
		return errors.New("the file changed while gocpp read it")
	}
	orig := sourceDecls(src)
	offset := func(pos token.Pos) int { return tf.Offset(pos) }

	// Where each declaration of the tree is in the source.
	type place struct {
		start, end int // the text of the declaration, with its documentation
		code       int // where the code of the original declaration starts: before it are the comments and lines of macros
		fresh      bool
		changed    bool
	}
	places := make([]place, len(file.Decls))
	for i, d := range file.Decls {
		places[i].fresh = true
		if !known[d] {
			continue
		}
		probe := token.NoPos // a position of the declaration that is its own
		switch d := d.(type) {
		case *ast.FuncDecl:
			probe = d.Pos()
			if d.Body != nil && known[d.Body] {
				probe = d.Body.Rbrace
			}
		case *ast.GenDecl:
			probe = d.TokPos
		}
		if !inFile(tf, probe) {
			continue
		}
		o := offset(probe)
		j := slices.IndexFunc(orig, func(s srcDecl) bool { return s.start <= o && o < s.end })
		if j < 0 || orig[j].onTop {
			continue // a method that #Derive made stands where its line is
		}
		start := orig[j].start
		// The text of the declaration is replaced up to its own end: a comment after it, on its line, stays.
		end := orig[j].end
		// The documentation is not the code: a line of #Derive between a comment and its type is not in it.
		code := start
		if pos := d.Pos(); inFile(tf, pos) {
			code = offset(pos)
		}
		places[i] = place{start: start, end: end, code: orig[j].code, changed: slices.ContainsFunc(orig, func(s srcDecl) bool {
			return s.marked && s.start < end && code < s.end
		})}
	}

	var edits []edit
	var texts []string // what is put in, for the hidden imports
	// printDecl returns the text of d, with the comments between a and b of the source, in the line ends nl.
	printDecl := func(d ast.Decl, a, code, b int, nl string) (string, error) {
		// The comments before the code (the documentation, the ones on the lines of macros) are copied
		// above it: the printer would put the ones between a line of macros and the func in its parameters.
		var head string
		mine := []*ast.CommentGroup{} // not nil: the printer would print the comments of the nodes
		for _, g := range file.Comments {
			o := offset(g.Pos())
			switch {
			case o >= a && o < code:
				last := g.List[len(g.List)-1]
				head += strings.ReplaceAll(string(src[o:rawEnd(src, offset(last.Slash), len(last.Text))]), "\r", "") + "\n"
			case o >= code && o < b:
				mine = append(mine, g)
			}
		}
		tidy(f, d, src, tf, full)
		settle(tf, d, known, file.Package)
		// A file of its own holds the declaration, so that the printer prints every comment of the list.
		one := &ast.File{Package: file.Package, Name: &ast.Ident{NamePos: file.Package, Name: "p"}, Decls: []ast.Decl{d}, Comments: mine}
		var out bytes.Buffer
		if err := printConfig.Fprint(&out, fset, one); err != nil {
			return "", err
		}
		text := out.String()
		if _, rest, ok := strings.Cut(text, "\n"); ok { // package p
			text = strings.TrimLeft(rest, "\n")
		}
		text = head + strings.TrimRight(pkg.UserText(text), "\n")
		texts = append(texts, text)
		return strings.ReplaceAll(text, "\n", nl), nil
	}
	// after is where the declarations that the check wrote go: the start of the line after the last
	// declaration that is in the source.
	after := nextLine(src, lineEnd(src, offset(file.Package)))
	importAt := after
	ins := make(map[int][]string) // the declarations to put in at an offset
	for i, d := range file.Decls {
		p := places[i]
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			if !p.fresh {
				importAt = nextLine(src, p.end)
				after = max(after, importAt)
			}
			continue // the hidden imports are put in below
		}
		nl := lineBreak(src, after)
		switch {
		case !p.fresh && p.changed:
			nl = lineBreak(src, p.start)
			text, err := printDecl(d, p.start, p.code, p.end, nl)
			if err != nil {
				return err
			}
			edits = append(edits, edit{start: p.start, end: p.end, text: text})
			after = nextLine(src, p.end)
		case !p.fresh:
			after = nextLine(src, p.end)
		default:
			text, err := printDecl(d, 1, 1, 0, nl)
			if err != nil {
				return err
			}
			ins[after] = append(ins[after], text)
		}
	}
	// The lines of macros that no declaration takes in are the lines of #Derive: they go, and the comments on
	// the line stay.
	for _, o := range orig {
		if !o.onTop {
			continue
		}
		if !slices.ContainsFunc(places, func(p place) bool { return !p.fresh && p.changed && p.start <= o.start && o.end <= p.end }) {
			edits = append(edits, deriveLineEdit(src, o.start, o.end))
		}
	}
	// The hidden imports.
	var imports []string
	for _, s := range file.Imports {
		path := strings.Trim(s.Path.Value, `"`)
		if s.Name == nil || !strings.HasPrefix(path, "gocpp/") || !strings.HasPrefix(s.Name.Name, "gp_") {
			continue
		}
		if slices.ContainsFunc(texts, func(t string) bool { return strings.Contains(t, s.Name.Name+".") }) {
			imports = append(imports, "import "+s.Name.Name+" "+s.Path.Value)
		}
	}
	if len(imports) > 0 {
		ins[importAt] = slices.Insert(ins[importAt], 0, strings.Join(imports, "\n"))
	}
	for at, texts := range ins {
		edits = append(edits, edit{start: at, end: at, text: strings.Join(texts, "\n\n"), insert: true})
	}

	out, err := applyEdits(src, edits)
	if err != nil {
		return err
	}
	w.Write(out)
	return nil
}

// deriveLineEdit returns the edit that takes the line of macros src[start:end] out. The line goes when nothing
// else is on it; otherwise only the macros go, with the white space on one side, and the comments stay.
func deriveLineEdit(src []byte, start, end int) edit {
	ls, le := lineStart(src, start), lineEnd(src, end)
	before, after := src[ls:start], src[end:le]
	if len(bytes.TrimSpace(before)) == 0 && len(bytes.TrimSpace(after)) == 0 {
		return edit{start: ls, end: nextLine(src, le)}
	}
	if len(bytes.TrimSpace(after)) == 0 { // a comment before the macros: no white space is left at the end
		return edit{start: start - (len(before) - len(bytes.TrimRight(before, " \t"))), end: le}
	}
	// the macros, and the white space after them when nothing but white space is before them
	if len(bytes.TrimSpace(before)) == 0 {
		return edit{start: start, end: end + (len(after) - len(bytes.TrimLeft(after, " \t")))}
	}
	return edit{start: start, end: end}
}

// applyEdits returns src with the edits made. An insertion is a blank line, the text and a line end, and a
// blank line after it when the next line is not blank. Two edits that overlap are a bug: it is an error, not
// text that is left out.
func applyEdits(src []byte, edits []edit) ([]byte, error) {
	slices.SortStableFunc(edits, func(a, b edit) int {
		if c := cmp.Compare(a.start, b.start); c != 0 {
			return c
		}
		return cmp.Compare(a.end, b.end)
	})
	var w bytes.Buffer
	cursor := 0
	for _, e := range edits {
		if e.start < cursor || e.end < e.start || e.end > len(src) {
			return nil, fmt.Errorf("internal error: two changes to the bytes %d to %d of the file overlap", e.start, e.end)
		}
		w.Write(src[cursor:e.start])
		text := e.text
		if e.insert {
			nl := lineBreak(src, max(e.start-1, 0))
			text = strings.ReplaceAll(text, "\n", nl)
			if e.start == len(src) && (len(src) == 0 || src[len(src)-1] != '\n') {
				text = nl + nl + text // after a last line that has no line end
			} else {
				text = nl + text + nl
				if rest := src[e.start:]; len(rest) > 0 && rest[0] != '\n' && !bytes.HasPrefix(rest, []byte("\r\n")) {
					text += nl
				}
			}
		}
		w.WriteString(text)
		cursor = e.end
	}
	w.Write(src[cursor:])
	return w.Bytes(), nil
}

// tidy turns the declaration d, which is printed from the tree, back into what the user wrote where the
// compiler's passes have left a trace that is not an expansion: the uses of C++ namespaces and headers
// (C.std.vector for std.vector; the conversions of C++ values are put back in the text, see
// check.Package.UserText), and the parentheses that the macro rewrite puts around the names of its hidden
// arguments.
func tidy(f *load.File, d ast.Decl, src []byte, tf *token.File, full bool) {
	if f.Rewrite != nil {
		for _, u := range f.Rewrite.Uses {
			if u.Sel != nil {
				u.Sel.X = u.Ident
			}
		}
	}
	astutil.Apply(d, nil, func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.ParenExpr:
			switch x := n.X.(type) {
			case *ast.Ident:
				if strings.HasPrefix(x.Name, "gp_") {
					c.Replace(x)
				}
			case *ast.BasicLit:
				c.Replace(x) // the constants that the loader wrote
			}
		case *ast.BasicLit:
			if !full && n.Kind == token.STRING && len(n.Value) > shortLiteral && !writtenLiteral(src, tf, n) {
				n.Value = shorten(n.Value)
			}
		}
		return true
	})
}

// shortLiteral is the length of the longest string constant that gocpp expand prints in full.
const shortLiteral = 200

// shorten returns the Go string literal lit, which the compiler wrote (the bytes of an embedded file), as its
// first bytes and a comment with the length.
func shorten(lit string) string {
	s, err := strconv.Unquote(lit)
	if err != nil {
		return lit
	}
	n := min(40, len(s))
	for n > 0 && n < len(s) && !utf8.RuneStart(s[n]) { // do not cut a character in two
		n--
	}
	return strconv.Quote(s[:n]) + " /* " + strconv.Itoa(len(s)) + " bytes in all */"
}

// writtenLiteral reports whether the literal lit is in the source as it is (and not a constant that the loader
// made: the place of a macro, the bytes of an embedded file).
func writtenLiteral(src []byte, tf *token.File, lit *ast.BasicLit) bool {
	if !inFile(tf, lit.ValuePos) {
		return false
	}
	off := tf.Offset(lit.ValuePos)
	return off+len(lit.Value) <= len(src) && string(src[off:off+len(lit.Value)]) == lit.Value
}

func inFile(tf *token.File, pos token.Pos) bool {
	return pos.IsValid() && int(pos) >= tf.Base() && int(pos) <= tf.Base()+tf.Size()
}

func docOf(d ast.Decl) *ast.CommentGroup {
	switch d := d.(type) {
	case *ast.GenDecl:
		return d.Doc
	case *ast.FuncDecl:
		return d.Doc
	}
	return nil
}

var posType = reflect.TypeFor[token.Pos]()

// keepEmpty names the fields of positions that mean something when they are empty: no "...", no arrow, no "=" of
// an alias, no parentheses of a declaration group.
var keepEmpty = map[string]bool{"Ellipsis": true, "Arrow": true, "Assign": true}

// settle gives the nodes of the declaration d the positions that the printer needs to lay it out. The
// type check wrote nodes (the code of the ! forms, the bodies of the derived methods) whose positions are
// the places of the user's text that they stand for, or the lines of a generated file, or none. The
// printer lets the position of a token decide where the comments that are before it go and how many lines
// there are between two statements. A node of the user's text keeps its position. The start of a statement
// that the check wrote keeps its own too, when it is not before the node before it and the statement does
// not follow a statement that the check wrote (the lowering of ! gives the check of a statement the end of
// the statement, and the code that goes before a statement its start); every other node takes the position
// of the node before it,. So the comments of the user stay with the statement that
// they were written before, and the new code is laid out as gofmt does. A declaration that is all new takes
// one position.
func settle(tf *token.File, d ast.Decl, known map[ast.Node]bool, whole token.Pos) {
	// A name or a literal of the user's text that is after one that follows it in the tree has the wrong
	// position (a later pass of the check gave it the end of its function): it is not the user's.
	bad := make(map[ast.Node]bool)
	var leaves []ast.Node
	ast.Inspect(d, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.Ident, *ast.BasicLit:
			if known[n] && inFile(tf, n.Pos()) {
				leaves = append(leaves, n)
			}
		}
		return true
	})
	for i, low := len(leaves)-1, token.Pos(0); i >= 0; i-- {
		if p := leaves[i].Pos(); low != 0 && p > low {
			bad[leaves[i]] = true
		} else if low == 0 || p < low {
			low = p
		}
	}
	isUser := func(n ast.Node) bool { return known[n] && inFile(tf, n.Pos()) && !bad[n] }
	var nodes []ast.Node
	inNew := make(map[ast.Node]bool) // the nodes that have a new node above them
	var stack []ast.Node
	ast.Inspect(d, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		switch n.(type) {
		case *ast.Comment, *ast.CommentGroup:
			return false
		}
		if len(stack) > 0 {
			if p := stack[len(stack)-1]; inNew[p] || !isUser(p) {
				inNew[n] = true
			}
		}
		stack = append(stack, n)
		nodes = append(nodes, n)
		return true
	})
	// next[i] is the position of the first node after nodes[i] that is the user's.
	next := make([]token.Pos, len(nodes)+1)
	for i := len(nodes) - 1; i >= 0; i-- {
		next[i] = next[i+1]
		if i+1 < len(nodes) && isUser(nodes[i+1]) {
			next[i] = nodes[i+1].Pos()
		}
	}
	allNew := !slices.ContainsFunc(nodes, isUser)
	// afterNew holds the statements that the check wrote and that go on what the check wrote before them: the
	// first of a block that is new, and a statement whose anchor is the one of the statement before it (the
	// lowering gives the code that stands for one statement of the user the same place). They take the
	// position of the node before them, so that no blank line comes between them.
	afterNew := make(map[ast.Node]bool)
	start := func(n ast.Node) token.Pos { // the position of the start of a statement, before settle changes it
		v := reflect.ValueOf(n).Elem()
		for j := range v.NumField() {
			if v.Field(j).Type() == posType {
				return token.Pos(v.Field(j).Int())
			}
		}
		return token.NoPos
	}
	list := func(l []ast.Stmt, opened bool) { // opened: the list follows the "{" of a block that is new
		for i := range l {
			switch {
			case isUser(l[i]):
			case i == 0 && opened:
				afterNew[l[i]] = true
			case i > 0 && !isUser(l[i-1]) && start(l[i]) == start(l[i-1]):
				afterNew[l[i]] = true
			}
		}
	}
	for _, n := range nodes {
		switch n := n.(type) {
		case *ast.BlockStmt:
			list(n.List, !isUser(n))
		case *ast.CaseClause:
			list(n.Body, false)
		case *ast.CommClause:
			list(n.Body, false)
		}
	}
	last := token.NoPos // the position of the node before
	for i, n := range nodes {
		user := isUser(n)
		_, stmt := n.(ast.Stmt)
		_, isBlock := n.(*ast.BlockStmt)
		wrapper := isBlock && !user && !inNew[n] // a block that the check wrote around the code of a statement
		v := reflect.ValueOf(n).Elem()
		first := token.NoPos // the position that the node's first field has now
		k := -1              // the index of the field among the fields of positions
		for j := range v.NumField() {
			f := v.Field(j)
			name := v.Type().Field(j).Name
			if f.Type() != posType || !f.CanSet() || keepEmpty[name] {
				continue
			}
			if _, group := n.(*ast.GenDecl); group && (name == "Lparen" || name == "Rparen") {
				continue
			}
			k++
			pos := token.Pos(f.Int())
			switch {
			case allNew:
				pos = whole
			case user && pos != token.NoPos:
				// kept
			case stmt && !user && k == 0 && inFile(tf, pos) && pos >= last && !afterNew[n]:
				// kept: the lowering put the statement there
			case wrapper && name == "Rbrace" && inFile(tf, pos) && pos >= last:
				// kept: the end of the statement that the block stands for, so that the user's lines go on from there
			case last != token.NoPos:
				pos = last
			default:
				pos = next[i]
			}
			if first == token.NoPos {
				first = pos
			}
			f.SetInt(int64(pos))
		}
		switch {
		case user && inFile(tf, n.Pos()):
			last = n.Pos()
		case !user && inFile(tf, first):
			last = first
		}
	}
}
