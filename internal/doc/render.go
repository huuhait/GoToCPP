// This file is pkg.go of cmd/internal/doc of Go 1.25.14, which prints the
// documentation (Copyright 2015 The Go Authors. All rights reserved. Use of
// that source code is governed by a BSD-style license that can be found in
// internal/doc/LICENSE).
//
// gocpp: the changes, on purpose:
//   - the flags are the fields of Options, in the session, not globals, and
//     a session reads a Package (source.go) that it parses itself;
//   - nothing calls log.Fatal or os.Exit: errors are returned (Fatalf
//     panics with a PackageError that render recovers, as go doc does);
//   - declarations are printed by gocpp's printer (format.go), which prints
//     the syntax of Phase 10 as it is written;
//   - macros (named "Name#" while go/doc runs, see source.go), the macros on
//     top of a declaration, #Derive and lazy parameters are known (scanPhase10 in source.go
//     and the cases marked "gocpp:" below);
//   - the matching of a symbol knows names with a ! (the symbol Name also
//     finds Name!) and macros (#Name);
//   - the examples of the _test.gp files can be listed (Options.Examples).

package doc

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	godoc "go/doc"
	"go/token"
	"io"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/syntax/gpast"
	"github.com/huuhait/gocpp/internal/syntax/printer"
)

const (
	punchedCardWidth = 80
	indent           = "    "
)

// A session is the documentation of one package for one run.
type session struct {
	opts        Options
	writer      io.Writer           // Destination for output.
	name        string              // Package name, json for encoding/json.
	userPath    string              // String the user used to find this package.
	src         *Package            // The package as found.
	ps          *parsed             // Parsed package.
	comments    []*ast.CommentGroup // The comments of all the files, for -src.
	doc         *godoc.Package
	typedValue  map[*godoc.Value]bool // Consts and vars related to types.
	constructor map[*godoc.Func]bool  // Constructors.
	fs          *token.FileSet        // Needed for printing.
	buf         pkgBuffer
}

// newSession parses the package and runs go/doc on it.
func newSession(w io.Writer, opts Options, pkg *Package, userPath string) (*session, error) {
	ps, err := pkg.parse()
	if err != nil {
		return nil, err
	}
	// TODO (go doc): go/doc does not include typed constants in the constants
	// list, which is what we want. For instance, time.Sunday is of type
	// time.Weekday, so it is defined in the type but not in the Consts list
	// for the package. This prevents
	//	go doc time.Sunday
	// from finding the symbol. Work around this for now, but we should fix
	// it in go/doc. A similar story applies to factory functions.
	mode := godoc.AllDecls
	if opts.Src {
		mode |= godoc.PreserveAST // See comment for session.emit.
	}
	// go/doc.NewFromFiles wants names that end in .go, and .gp files have
	// another name: go/doc.New takes the files as an *ast.Package, as go doc
	// does (the type is deprecated, the function is not).
	astPkg := &ast.Package{Name: pkg.Name, Files: make(map[string]*ast.File, len(ps.files))} //lint:ignore SA1019 the argument of go/doc.New
	for i, f := range ps.files {
		astPkg.Files[ps.fset.File(f.Pos()).Name()] = f
		_ = i
	}
	docPkg := godoc.New(astPkg, pkg.ImportPath, mode)
	ps.restoreMacros()
	s := &session{
		opts:        opts,
		writer:      w,
		name:        pkg.Name,
		userPath:    userPath,
		src:         pkg,
		ps:          ps,
		doc:         docPkg,
		typedValue:  make(map[*godoc.Value]bool),
		constructor: make(map[*godoc.Func]bool),
		fs:          ps.fset,
	}
	for _, f := range ps.files {
		s.comments = append(s.comments, f.Comments...)
	}
	for _, typ := range docPkg.Types {
		docPkg.Consts = append(docPkg.Consts, typ.Consts...)
		docPkg.Vars = append(docPkg.Vars, typ.Vars...)
		docPkg.Funcs = append(docPkg.Funcs, typ.Funcs...)
		if s.isExported(typ.Name) {
			for _, value := range typ.Consts {
				s.typedValue[value] = true
			}
			for _, value := range typ.Vars {
				s.typedValue[value] = true
			}
			for _, fun := range typ.Funcs {
				// We don't count it as a constructor bound to the type
				// if the type itself is not exported.
				s.constructor[fun] = true
			}
		}
	}
	s.addDerived()
	s.buf.pkg = s
	return s, nil
}

func (s *session) ToText(w io.Writer, text, prefix, codePrefix string) {
	d := s.doc.Parser().Parse(text)
	pr := s.doc.Printer()
	pr.TextPrefix = prefix
	pr.TextCodePrefix = codePrefix
	w.Write(pr.Text(d))
}

// pkgBuffer is a wrapper for bytes.Buffer that prints a package clause the
// first time Write is called.
type pkgBuffer struct {
	pkg     *session
	printed bool // Prevent repeated package clauses.
	bytes.Buffer
}

func (pb *pkgBuffer) Write(p []byte) (int, error) {
	pb.packageClause()
	return pb.Buffer.Write(p)
}

func (pb *pkgBuffer) WriteString(p string) (int, error) { return pb.Write([]byte(p)) }

func (pb *pkgBuffer) WriteRune(r rune) (int, error) { return pb.Write([]byte(string(r))) }

func (pb *pkgBuffer) WriteByte(c byte) error {
	_, err := pb.Write([]byte{c})
	return err
}

func (pb *pkgBuffer) packageClause() {
	if !pb.printed {
		pb.printed = true
		// Only show package clause for commands if requested explicitly.
		if pb.pkg.name != "main" || pb.pkg.opts.Cmd {
			pb.pkg.packageClause()
		}
	}
}

type PackageError string // type returned by session.Fatalf.

func (p PackageError) Error() string {
	return string(p)
}

// prettyPath returns a version of the package path that is suitable for an
// error message. Since the import path is sometimes the unhelpful "" or ".",
// it falls back to the folder.
func (s *session) prettyPath() string {
	path := s.src.ImportPath
	if path != "." && path != "" {
		return path
	}
	return filepath.Clean(filepath.ToSlash(s.src.Dir))
}

// Fatalf is like log.Fatalf, but panics so it can be recovered in render,
// so it doesn't cause an exit. The log prefix is added by the command.
func (s *session) Fatalf(format string, args ...any) {
	panic(PackageError(fmt.Sprintf(format, args...)))
}

func (s *session) Printf(format string, args ...any) {
	fmt.Fprintf(&s.buf, format, args...)
}

func (s *session) flush() error {
	_, err := s.writer.Write(s.buf.Bytes())
	s.buf.Reset() // Not needed, but it's a flush.
	return err
}

var newlineBytes = []byte("\n\n") // We never ask for more than 2.

// newlines guarantees there are n newlines at the end of the buffer.
func (s *session) newlines(n int) {
	for !bytes.HasSuffix(s.buf.Bytes(), newlineBytes[:n]) {
		s.buf.WriteRune('\n')
	}
}

// emit prints the node. If Src is true, it ignores the provided comment,
// assuming the comment is in the node itself. Otherwise, the go/doc package
// clears the stuff we don't want to print anyway. It's a bit of a magic trick.
func (s *session) emit(comment string, node ast.Node) {
	if node != nil {
		var arg any = node
		if s.opts.Src {
			// Need an extra little dance to get internal comments to appear.
			arg = &printer.CommentedNode{Node: node, Comments: s.comments}
		}
		// gocpp: the lines of macros on top of the declaration.
		if lines := s.ps.onTop[node]; len(lines) > 0 {
			s.emitOnTop(node, lines)
		}
		err := formatNode(&s.buf, s.fs, arg)
		if err != nil {
			s.Fatalf("%v", err)
		}
		if comment != "" && !s.opts.Src {
			s.newlines(1)
			s.ToText(&s.buf, comment, indent, indent+indent)
			s.newlines(2) // Blank line after comment to separate from next item.
		} else {
			s.newlines(1)
		}
	}
}

// emitOnTop prints the lines of macros that are on top of node. In the
// source they are between the doc comment of the declaration and the
// declaration, and the printer prints the doc comment first: so with -src
// the doc comment is printed here and taken off the node, and *arg is the
// node without it. Without -src the doc comment is printed by emit, after
// the declaration.
func (s *session) emitOnTop(node ast.Node, lines []string) {
	if s.opts.Src {
		var doc **ast.CommentGroup
		switch n := node.(type) {
		case *ast.FuncDecl:
			doc = &n.Doc
		case *ast.GenDecl:
			doc = &n.Doc
		}
		if doc != nil && *doc != nil {
			for _, c := range (*doc).List {
				s.buf.WriteString(c.Text + "\n")
			}
			*doc = nil
		}
	}
	for _, line := range lines {
		s.buf.WriteString(line + "\n")
	}
}

// oneLineNode returns a one-line summary of the given input node.
func (s *session) oneLineNode(node ast.Node) string {
	const maxDepth = 10
	return s.oneLineNodeDepth(node, maxDepth)
}

// oneLineNodeDepth returns a one-line summary of the given input node.
// The depth specifies the maximum depth when traversing the AST.
func (s *session) oneLineNodeDepth(node ast.Node, depth int) string {
	const dotDotDot = "..."
	if depth == 0 {
		return dotDotDot
	}
	depth--

	switch n := node.(type) {
	case nil:
		return ""

	case *ast.GenDecl:
		// Formats const and var declarations.
		trailer := ""
		if len(n.Specs) > 1 {
			trailer = " " + dotDotDot
		}

		// Find the first relevant spec.
		typ := ""
		for i, spec := range n.Specs {
			valueSpec := spec.(*ast.ValueSpec) // Must succeed; we can't mix types in one GenDecl.

			// The type name may carry over from a previous specification in the
			// case of constants and iota.
			if valueSpec.Type != nil {
				typ = fmt.Sprintf(" %s", s.oneLineNodeDepth(valueSpec.Type, depth))
			} else if len(valueSpec.Values) > 0 {
				typ = ""
			}

			if !s.isExported(valueSpec.Names[0].Name) {
				continue
			}
			val := ""
			if i < len(valueSpec.Values) && valueSpec.Values[i] != nil {
				val = fmt.Sprintf(" = %s", s.oneLineNodeDepth(valueSpec.Values[i], depth))
			}
			return fmt.Sprintf("%s %s%s%s%s", n.Tok, valueSpec.Names[0], typ, val, trailer)
		}
		return ""

	case *ast.FuncDecl:
		// Formats func declarations.
		name := n.Name.Name
		recv := s.oneLineNodeDepth(n.Recv, depth)
		if len(recv) > 0 {
			recv = "(" + recv + ") "
		}
		fnc := s.oneLineNodeDepth(n.Type, depth)
		fnc = strings.TrimPrefix(fnc, "func")
		if gpast.IsMacroDecl(n) { // gocpp: macro Name(params)
			return fmt.Sprintf("macro %s%s", gpast.MacroBase(name), fnc)
		}
		return fmt.Sprintf("func %s%s%s", recv, name, fnc)

	case *ast.TypeSpec:
		sep := " "
		if n.Assign.IsValid() {
			sep = " = "
		}
		tparams := s.formatTypeParams(n.TypeParams, depth)
		return fmt.Sprintf("type %s%s%s%s", n.Name.Name, tparams, sep, s.oneLineNodeDepth(n.Type, depth))

	case *ast.FuncType:
		var params []string
		if n.Params != nil {
			for _, field := range n.Params.List {
				params = append(params, s.oneLineField(field, depth))
			}
		}
		needParens := false
		var results []string
		if n.Results != nil {
			needParens = needParens || len(n.Results.List) > 1
			for _, field := range n.Results.List {
				needParens = needParens || len(field.Names) > 0
				results = append(results, s.oneLineField(field, depth))
			}
		}

		tparam := s.formatTypeParams(n.TypeParams, depth)
		param := joinStrings(params)
		if len(results) == 0 {
			return fmt.Sprintf("func%s(%s)", tparam, param)
		}
		result := joinStrings(results)
		if !needParens {
			return fmt.Sprintf("func%s(%s) %s", tparam, param, result)
		}
		return fmt.Sprintf("func%s(%s) (%s)", tparam, param, result)

	case *ast.StructType:
		if n.Fields == nil || len(n.Fields.List) == 0 {
			return "struct{}"
		}
		return "struct{ ... }"

	case *ast.InterfaceType:
		if n.Methods == nil || len(n.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{ ... }"

	case *ast.FieldList:
		if n == nil || len(n.List) == 0 {
			return ""
		}
		if len(n.List) == 1 {
			return s.oneLineField(n.List[0], depth)
		}
		return dotDotDot

	case *ast.FuncLit:
		return s.oneLineNodeDepth(n.Type, depth) + " { ... }"

	case *ast.CompositeLit:
		typ := s.oneLineNodeDepth(n.Type, depth)
		if len(n.Elts) == 0 {
			return fmt.Sprintf("%s{}", typ)
		}
		return fmt.Sprintf("%s{ %s }", typ, dotDotDot)

	case *ast.ArrayType:
		length := s.oneLineNodeDepth(n.Len, depth)
		element := s.oneLineNodeDepth(n.Elt, depth)
		return fmt.Sprintf("[%s]%s", length, element)

	case *ast.MapType:
		key := s.oneLineNodeDepth(n.Key, depth)
		value := s.oneLineNodeDepth(n.Value, depth)
		return fmt.Sprintf("map[%s]%s", key, value)

	case *ast.CallExpr:
		fnc := s.oneLineNodeDepth(n.Fun, depth)
		var args []string
		for _, arg := range n.Args {
			args = append(args, s.oneLineNodeDepth(arg, depth))
		}
		return fmt.Sprintf("%s(%s)", fnc, joinStrings(args))

	case *ast.UnaryExpr:
		if typ, ok := gpast.Lazy(n); ok { // gocpp: lazy T
			return "lazy " + s.oneLineNodeDepth(typ, depth)
		}
		return fmt.Sprintf("%s%s", n.Op, s.oneLineNodeDepth(n.X, depth))

	case *ast.Ident:
		return n.Name

	default:
		// As a fallback, use default formatter for all unknown node types.
		buf := new(strings.Builder)
		formatNode(buf, s.fs, node)
		str := buf.String()
		if strings.Contains(str, "\n") {
			return dotDotDot
		}
		return str
	}
}

func (s *session) formatTypeParams(list *ast.FieldList, depth int) string {
	if list.NumFields() == 0 {
		return ""
	}
	var tparams []string
	for _, field := range list.List {
		tparams = append(tparams, s.oneLineField(field, depth))
	}
	return "[" + joinStrings(tparams) + "]"
}

// oneLineField returns a one-line summary of the field.
func (s *session) oneLineField(field *ast.Field, depth int) string {
	var names []string
	for _, name := range field.Names {
		names = append(names, name.Name)
	}
	if len(names) == 0 {
		return s.oneLineNodeDepth(field.Type, depth)
	}
	return joinStrings(names) + " " + s.oneLineNodeDepth(field.Type, depth)
}

// joinStrings formats the input as a comma-separated list, but truncates
// the list at some reasonable length if necessary.
func joinStrings(ss []string) string {
	var n int
	for i, s := range ss {
		n += len(s) + len(", ")
		if n > punchedCardWidth {
			ss = append(ss[:i:i], "...")
			break
		}
	}
	return strings.Join(ss, ", ")
}

// printHeader prints a header for the section named s, adding a blank line
// on each side.
func (s *session) printHeader(h string) {
	s.Printf("\n%s\n\n", h)
}

// constsDoc prints all const documentation, if any, including a header.
// The one argument is the valueDoc registry.
func (s *session) constsDoc(printed map[*ast.GenDecl]bool) {
	var header bool
	for _, value := range s.doc.Consts {
		// Constants and variables come in groups, and valueDoc prints
		// all the items in the group. We only need to find one exported symbol.
		for _, name := range value.Names {
			if s.isExported(name) && !s.typedValue[value] {
				if !header {
					s.printHeader("CONSTANTS")
					header = true
				}
				s.valueDoc(value, printed)
				break
			}
		}
	}
}

// varsDoc prints all var documentation, if any, including a header.
// Printed is the valueDoc registry.
func (s *session) varsDoc(printed map[*ast.GenDecl]bool) {
	var header bool
	for _, value := range s.doc.Vars {
		// Constants and variables come in groups, and valueDoc prints
		// all the items in the group. We only need to find one exported symbol.
		for _, name := range value.Names {
			if s.isExported(name) && !s.typedValue[value] {
				if !header {
					s.printHeader("VARIABLES")
					header = true
				}
				s.valueDoc(value, printed)
				break
			}
		}
	}
}

// funcsDoc prints all func documentation, if any, including a header.
func (s *session) funcsDoc() {
	var header bool
	for _, fun := range s.doc.Funcs {
		if s.isExported(fun.Name) && !s.constructor[fun] {
			if !header {
				s.printHeader("FUNCTIONS")
				header = true
			}
			s.emit(fun.Doc, fun.Decl)
		}
	}
}

// typesDoc prints all type documentation, if any, including a header.
func (s *session) typesDoc() {
	var header bool
	for _, typ := range s.doc.Types {
		if s.isExported(typ.Name) {
			if !header {
				s.printHeader("TYPES")
				header = true
			}
			s.typeDoc(typ)
		}
	}
}

// packageDoc prints the docs for the package.
func (s *session) packageDoc() {
	s.Printf("") // Trigger the package clause; we know the package exists.
	if s.opts.All || !s.opts.Short {
		s.ToText(&s.buf, s.doc.Doc, "", indent)
		s.newlines(1)
	}

	switch {
	case s.opts.All:
		printed := make(map[*ast.GenDecl]bool) // valueDoc registry
		s.constsDoc(printed)
		s.varsDoc(printed)
		s.funcsDoc()
		s.typesDoc()

	case s.name == "main" && !s.opts.Cmd:
		// Show only package docs for commands.
		return

	default:
		if !s.opts.Short {
			s.newlines(2) // Guarantee blank line before the components.
		}
		s.valueSummary(s.doc.Consts, false)
		s.valueSummary(s.doc.Vars, false)
		s.funcSummary(s.doc.Funcs, false)
		s.typeSummary()
	}

	if !s.opts.Short {
		s.bugs()
	}
	s.examples(true, "")
}

// packageClause prints the package clause.
func (s *session) packageClause() {
	if s.opts.Short {
		return
	}
	// The import path is known when the package is found (in go doc it is
	// computed again from the code roots, with the same result).
	importPath := s.src.ImportPath
	s.Printf("package %s // import %q\n\n", s.name, importPath)
}

// valueSummary prints a one-line summary for each set of values and
// constants. If all the types in a constant or variable declaration belong
// to the same type they can be printed by typeSummary, and so can be
// suppressed here.
func (s *session) valueSummary(values []*godoc.Value, showGrouped bool) {
	var isGrouped map[*godoc.Value]bool
	if !showGrouped {
		isGrouped = make(map[*godoc.Value]bool)
		for _, typ := range s.doc.Types {
			if !s.isExported(typ.Name) {
				continue
			}
			for _, c := range typ.Consts {
				isGrouped[c] = true
			}
			for _, v := range typ.Vars {
				isGrouped[v] = true
			}
		}
	}

	for _, value := range values {
		if !isGrouped[value] {
			if decl := s.oneLineNode(value.Decl); decl != "" {
				s.Printf("%s\n", decl)
			}
		}
	}
}

// funcSummary prints a one-line summary for each function. Constructors
// are printed by typeSummary, below, and so can be suppressed here.
func (s *session) funcSummary(funcs []*godoc.Func, showConstructors bool) {
	for _, fun := range funcs {
		// Exported functions only. The go/doc package does not include methods here.
		if s.isExported(fun.Name) {
			if showConstructors || !s.constructor[fun] {
				s.Printf("%s\n", s.oneLineNode(fun.Decl))
			}
		}
	}
}

// typeSummary prints a one-line summary for each type, followed by its
// constructors.
func (s *session) typeSummary() {
	for _, typ := range s.doc.Types {
		for _, spec := range typ.Decl.Specs {
			typeSpec := spec.(*ast.TypeSpec) // Must succeed.
			if s.isExported(typeSpec.Name.Name) {
				s.Printf("%s\n", s.oneLineNode(typeSpec))
				// Now print the consts, vars, and constructors.
				for _, c := range typ.Consts {
					if decl := s.oneLineNode(c.Decl); decl != "" {
						s.Printf(indent+"%s\n", decl)
					}
				}
				for _, v := range typ.Vars {
					if decl := s.oneLineNode(v.Decl); decl != "" {
						s.Printf(indent+"%s\n", decl)
					}
				}
				for _, constructor := range typ.Funcs {
					if s.isExported(constructor.Name) {
						s.Printf(indent+"%s\n", s.oneLineNode(constructor.Decl))
					}
				}
			}
		}
	}
}

// bugs prints the BUGS information for the package.
// TODO (go doc): Provide access to TODOs and NOTEs as well (very noisy so off by default)?
func (s *session) bugs() {
	if s.doc.Notes["BUG"] == nil {
		return
	}
	s.Printf("\n")
	for _, note := range s.doc.Notes["BUG"] {
		s.Printf("%s: %v\n", "BUG", note.Body)
	}
}

// findValues finds the godoc.Values that describe the symbol.
func (s *session) findValues(symbol string, docValues []*godoc.Value) (values []*godoc.Value) {
	for _, value := range docValues {
		for _, name := range value.Names {
			if s.match(symbol, name) {
				values = append(values, value)
			}
		}
	}
	return
}

// findFuncs finds the godoc.Funcs that describes the symbol.
func (s *session) findFuncs(symbol string) (funcs []*godoc.Func) {
	for _, fun := range s.doc.Funcs {
		if s.match(symbol, fun.Name) {
			funcs = append(funcs, fun)
		}
	}
	return
}

// findTypes finds the godoc.Types that describes the symbol.
// If symbol is empty, it finds all exported types.
func (s *session) findTypes(symbol string) (types []*godoc.Type) {
	for _, typ := range s.doc.Types {
		if symbol == "" && s.isExported(typ.Name) || s.match(symbol, typ.Name) {
			types = append(types, typ)
		}
	}
	return
}

// findTypeSpec returns the ast.TypeSpec within the declaration that defines
// the symbol. The name must match exactly.
func (s *session) findTypeSpec(decl *ast.GenDecl, symbol string) *ast.TypeSpec {
	for _, spec := range decl.Specs {
		typeSpec := spec.(*ast.TypeSpec) // Must succeed.
		if symbol == typeSpec.Name.Name {
			return typeSpec
		}
	}
	return nil
}

// symbolDoc prints the docs for symbol. There may be multiple matches.
// If symbol matches a type, output includes its methods factories and
// associated constants. If there is no top-level symbol, symbolDoc looks
// for methods that match.
func (s *session) symbolDoc(symbol string) bool {
	found := false
	// Functions.
	for _, fun := range s.findFuncs(symbol) {
		// Symbol is a function.
		decl := fun.Decl
		s.emit(fun.Doc, decl)
		found = true
	}
	// Constants and variables behave the same.
	values := s.findValues(symbol, s.doc.Consts)
	values = append(values, s.findValues(symbol, s.doc.Vars)...)
	printed := make(map[*ast.GenDecl]bool) // valueDoc registry
	for _, value := range values {
		s.valueDoc(value, printed)
		found = true
	}
	// Types.
	for _, typ := range s.findTypes(symbol) {
		s.typeDoc(typ)
		found = true
	}
	if !found {
		// See if there are methods.
		if !s.printMethodDoc("", symbol) {
			return false
		}
	}
	return true
}

// valueDoc prints the docs for a constant or variable. The printed map
// records which values have been printed already to avoid duplication.
// Otherwise, a declaration like:
//
//	const ( c = 1; C = 2 )
//
// … could be printed twice if the -u flag is set, as it matches twice.
func (s *session) valueDoc(value *godoc.Value, printed map[*ast.GenDecl]bool) {
	if printed[value.Decl] {
		return
	}
	// Print each spec only if there is at least one exported symbol in it.
	// (See issue 11008.)
	// TODO (go doc): Should we elide unexported symbols from a single spec?
	// It's an unlikely scenario, probably not worth the trouble.
	// TODO (go doc): Would be nice if go/doc did this for us.
	specs := make([]ast.Spec, 0, len(value.Decl.Specs))
	var typ ast.Expr
	for _, spec := range value.Decl.Specs {
		vspec := spec.(*ast.ValueSpec)

		// The type name may carry over from a previous specification in the
		// case of constants and iota.
		if vspec.Type != nil {
			typ = vspec.Type
		}

		for _, ident := range vspec.Names {
			if s.opts.Src || s.isExported(ident.Name) {
				if vspec.Type == nil && vspec.Values == nil && typ != nil {
					// This a standalone identifier, as in the case of iota usage.
					// Thus, assume the type comes from the previous type.
					vspec.Type = &ast.Ident{
						Name:    s.oneLineNode(typ),
						NamePos: vspec.End() - 1,
					}
				}

				specs = append(specs, vspec)
				typ = nil // Only inject type on first exported identifier
				break
			}
		}
	}
	if len(specs) == 0 {
		return
	}
	value.Decl.Specs = specs
	s.emit(value.Doc, value.Decl)
	printed[value.Decl] = true
}

// typeDoc prints the docs for a type, including constructors and other items
// related to it.
func (s *session) typeDoc(typ *godoc.Type) {
	decl := typ.Decl
	spec := s.findTypeSpec(decl, typ.Name)
	s.trimUnexportedElems(spec)
	// If there are multiple types defined, reduce to just this one.
	if len(decl.Specs) > 1 {
		decl.Specs = []ast.Spec{spec}
	}
	s.emit(typ.Doc, decl)
	s.newlines(2)
	// Show associated methods, constants, etc.
	if s.opts.All {
		printed := make(map[*ast.GenDecl]bool) // valueDoc registry
		// We can use append here to print consts, then vars. Ditto for funcs and methods.
		values := typ.Consts
		values = append(values, typ.Vars...)
		for _, value := range values {
			for _, name := range value.Names {
				if s.isExported(name) {
					s.valueDoc(value, printed)
					break
				}
			}
		}
		funcs := typ.Funcs
		funcs = append(funcs, typ.Methods...)
		for _, fun := range funcs {
			if s.isExported(fun.Name) {
				s.emit(fun.Doc, fun.Decl)
				if fun.Doc == "" {
					s.newlines(2)
				}
			}
		}
	} else {
		s.valueSummary(typ.Consts, true)
		s.valueSummary(typ.Vars, true)
		s.funcSummary(typ.Funcs, true)
		s.funcSummary(typ.Methods, true)
	}
}

// trimUnexportedElems modifies spec in place to elide unexported fields from
// structs and methods from interfaces (unless the unexported flag is set or
// we are asked to show the original source).
func (s *session) trimUnexportedElems(spec *ast.TypeSpec) {
	if s.opts.Src {
		return
	}
	switch typ := spec.Type.(type) {
	case *ast.StructType:
		typ.Fields = s.trimUnexportedFields(typ.Fields, false)
	case *ast.InterfaceType:
		typ.Methods = s.trimUnexportedFields(typ.Methods, true)
	}
}

// trimUnexportedFields returns the field list trimmed of unexported fields.
func (s *session) trimUnexportedFields(fields *ast.FieldList, isInterface bool) *ast.FieldList {
	what := "methods"
	if !isInterface {
		what = "fields"
	}

	trimmed := false
	list := make([]*ast.Field, 0, len(fields.List))
	for _, field := range fields.List {
		// When printing fields we normally print field.Doc.
		// Here we are going to pass the AST to the printer,
		// which will print the comments from the AST,
		// not field.Doc which is from go/doc.
		// The two are similar but not identical;
		// for example, field.Doc does not include directives.
		// In order to consistently print field.Doc,
		// we replace the comment in the AST with field.Doc.
		// That will cause the printer to print what we want.
		// See issue #56592.
		if field.Doc != nil {
			doc := field.Doc
			text := doc.Text()

			trailingBlankLine := len(doc.List[len(doc.List)-1].Text) == 2
			if !trailingBlankLine {
				// Remove trailing newline.
				lt := len(text)
				if lt > 0 && text[lt-1] == '\n' {
					text = text[:lt-1]
				}
			}

			start := doc.List[0].Slash
			doc.List = doc.List[:0]
			for _, line := range strings.Split(text, "\n") {
				prefix := "// "
				if len(line) > 0 && line[0] == '\t' {
					prefix = "//"
				}
				doc.List = append(doc.List, &ast.Comment{
					Text: prefix + line,
				})
			}
			doc.List[0].Slash = start
		}

		names := field.Names
		if len(names) == 0 {
			// Embedded type. Use the name of the type. It must be of the form ident or
			// pkg.ident (for structs and interfaces), or *ident or *pkg.ident (structs only).
			// Or a type embedded in a constraint.
			// Nothing else is allowed.
			ty := field.Type
			if se, ok := field.Type.(*ast.StarExpr); !isInterface && ok {
				// The form *ident or *pkg.ident is only valid on
				// embedded types in structs.
				ty = se.X
			}
			constraint := false
			switch ident := ty.(type) {
			case *ast.Ident:
				if isInterface && ident.Name == "error" && ident.Obj == nil {
					// For documentation purposes, we consider the builtin error
					// type special when embedded in an interface, such that it
					// always gets shown publicly.
					list = append(list, field)
					continue
				}
				names = []*ast.Ident{ident}
			case *ast.SelectorExpr:
				// An embedded type may refer to a type in another package.
				names = []*ast.Ident{ident.Sel}
			default:
				// An approximation or union or type
				// literal in an interface.
				constraint = true
			}
			if names == nil && !constraint {
				// Can only happen if AST is incorrect. Safe to continue with a nil list.
			}
		}
		// Trims if any is unexported. Good enough in practice.
		ok := true
		if !s.opts.Unexported {
			for _, name := range names {
				if !s.isExported(name.Name) {
					trimmed = true
					ok = false
					break
				}
			}
		}
		if ok {
			list = append(list, field)
		}
	}
	if !trimmed {
		return fields
	}
	unexportedField := &ast.Field{
		Type: &ast.Ident{
			// Hack: printer will treat this as a field with a named type.
			// Setting Name and NamePos to ("", fields.Closing-1) ensures that
			// when Pos and End are called on this field, they return the
			// position right before closing '}' character.
			Name:    "",
			NamePos: fields.Closing - 1,
		},
		Comment: &ast.CommentGroup{
			List: []*ast.Comment{{Text: fmt.Sprintf("// Has unexported %s.\n", what)}},
		},
	}
	return &ast.FieldList{
		Opening: fields.Opening,
		List:    append(list, unexportedField),
		Closing: fields.Closing,
	}
}

// printMethodDoc prints the docs for matches of symbol.method. If symbol is
// empty, it prints all methods for any concrete type that match the name. It
// reports whether it found any methods.
func (s *session) printMethodDoc(symbol, method string) bool {
	types := s.findTypes(symbol)
	if types == nil {
		if symbol == "" {
			return false
		}
		s.Fatalf("symbol %s is not a type in package %s installed in %q", symbol, s.name, s.src.ImportPath)
	}
	found := false
	for _, typ := range types {
		if len(typ.Methods) > 0 {
			for _, meth := range typ.Methods {
				if s.match(method, meth.Name) {
					decl := meth.Decl
					s.emit(meth.Doc, decl)
					found = true
				}
			}
			continue
		}
		if symbol == "" {
			continue
		}
		// Type may be an interface. The go/doc package does not attach
		// an interface's methods to the godoc.Type. We need to dig around.
		spec := s.findTypeSpec(typ.Decl, typ.Name)
		inter, ok := spec.Type.(*ast.InterfaceType)
		if !ok {
			// Not an interface type.
			continue
		}

		// Collect and print only the methods that match.
		var methods []*ast.Field
		for _, iMethod := range inter.Methods.List {
			// This is an interface, so there can be only one name.
			// TODO (go doc): Anonymous methods (embedding)
			if len(iMethod.Names) == 0 {
				continue
			}
			name := iMethod.Names[0].Name
			if s.match(method, name) {
				methods = append(methods, iMethod)
				found = true
			}
		}
		if found {
			s.Printf("type %s ", spec.Name)
			inter.Methods.List, methods = methods, inter.Methods.List
			err := formatNode(&s.buf, s.fs, inter)
			if err != nil {
				s.Fatalf("%v", err)
			}
			s.newlines(1)
			// Restore the original methods.
			inter.Methods.List = methods
		}
	}
	return found
}

// printFieldDoc prints the docs for matches of symbol.fieldName. It reports
// whether it found any field. Both symbol and fieldName must be non-empty or
// it returns false.
func (s *session) printFieldDoc(symbol, fieldName string) bool {
	if symbol == "" || fieldName == "" {
		return false
	}
	types := s.findTypes(symbol)
	if types == nil {
		s.Fatalf("symbol %s is not a type in package %s installed in %q", symbol, s.name, s.src.ImportPath)
	}
	found := false
	numUnmatched := 0
	for _, typ := range types {
		// Type must be a struct.
		spec := s.findTypeSpec(typ.Decl, typ.Name)
		structType, ok := spec.Type.(*ast.StructType)
		if !ok {
			// Not a struct type.
			continue
		}
		for _, field := range structType.Fields.List {
			// TODO (go doc): Anonymous fields.
			for _, name := range field.Names {
				if !s.match(fieldName, name.Name) {
					numUnmatched++
					continue
				}
				if !found {
					s.Printf("type %s struct {\n", typ.Name)
				}
				if field.Doc != nil {
					// To present indented blocks in comments correctly, process the comment as
					// a unit before adding the leading // to each line.
					docBuf := new(bytes.Buffer)
					s.ToText(docBuf, field.Doc.Text(), "", indent)
					scanner := bufio.NewScanner(docBuf)
					for scanner.Scan() {
						fmt.Fprintf(&s.buf, "%s// %s\n", indent, scanner.Bytes())
					}
				}
				str := s.oneLineNode(field.Type)
				lineComment := ""
				if field.Comment != nil {
					lineComment = fmt.Sprintf("  %s", field.Comment.List[0].Text)
				}
				s.Printf("%s%s %s%s\n", indent, name, str, lineComment)
				found = true
			}
		}
	}
	if found {
		if numUnmatched > 0 {
			s.Printf("\n    // ... other fields elided ...\n")
		}
		s.Printf("}\n")
	}
	return found
}

// isExported reports whether the name is an exported identifier. If the
// unexported flag (-u) is true, isExported returns true because it means
// that we treat the name as if it is exported.
func (s *session) isExported(name string) bool {
	return s.opts.Unexported || token.IsExported(name)
}

// match reports whether the user's symbol matches the program's. A
// lower-case character in the user's string matches either case in the
// program's. The program string must be exported.
//
// gocpp: a program name with a ! (Name!) or a macro (#Name, "Name#" here)
// is matched by the user's Name too, and exactly by Name! and #Name.
func (s *session) match(user, program string) bool {
	if !s.isExported(program) {
		return false
	}
	base, kind := splitName(program)
	switch ubase, ukind := splitUser(user); {
	case ukind == plainName && kind == plainName:
		// Go's rule, below.
	case ukind == plainName:
		user, program = ubase, base
	case ukind == kind:
		user, program = ubase, base
	default:
		return false
	}
	return matchName(user, program, s.opts.MatchCase)
}

// splitUser is splitName for what the user typed: "#Name" is a macro, and
// "Name!" a must function.
func splitUser(user string) (string, nameKind) {
	switch {
	case strings.HasPrefix(user, "#"):
		return user[1:], macroName
	case strings.HasSuffix(user, "!"):
		return strings.TrimSuffix(user, "!"), mustName
	}
	return user, plainName
}

// matchName is the comparison of match, for names that were found exported.
func matchName(user, program string, matchCase bool) bool {
	if matchCase {
		return user == program
	}
	for _, u := range user {
		p, w := utf8.DecodeRuneInString(program)
		program = program[w:]
		if u == p {
			continue
		}
		if unicode.IsLower(u) && simpleFold(u) == simpleFold(p) {
			continue
		}
		return false
	}
	return program == ""
}

// simpleFold returns the minimum rune equivalent to r under
// Unicode-defined simple case folding.
func simpleFold(r rune) rune {
	for {
		r1 := unicode.SimpleFold(r)
		if r1 <= r {
			return r1 // wrapped around, found min
		}
		r = r1
	}
}

// render prints the documentation of the symbol and method that the user
// asked for, like the switch in do() of go doc, and reports whether it found
// them. A PackageError (Fatalf) is returned as an error.
func (s *session) render(symbol, method string) (found bool, err error) {
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if pkgError, ok := e.(PackageError); ok {
			err = pkgError
			return
		}
		panic(e)
	}()
	switch {
	case symbol == "":
		s.packageDoc() // The package exists, so we got some output.
		found = true
	case method == "":
		if s.symbolDoc(symbol) {
			found = true
		}
	case s.printMethodDoc(symbol, method):
		found = true
	case s.printFieldDoc(symbol, method):
		found = true
	}
	if found && symbol != "" {
		name := symbol
		if method != "" {
			name += "_" + method
		}
		s.examples(false, name)
	}
	return found, nil
}
