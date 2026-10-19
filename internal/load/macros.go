package load

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/huuhait/gocpp/internal/syntax/gpast"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// User macros (Phase 10 step 10.9, README §2.2, ADR-075).
//
// A macro is a real Go function. The loader rewrites it, and every use of it, into plain Go
// before go/types sees anything, so a macro is type-checked once, where it is declared, and the
// lowering sees generic functions (which it instantiates in the package that uses them, so a
// macro is inlined across packages by the C++ compiler, with no help of the header):
//
//	macro Assert(cond bool, msg lazy string) { ... #file() ... #text(cond) ... msg() }
//
// becomes
//
//	func Assert[gp_L_msg ~func() string](gp_file string, gp_line int, gp_func string,
//		gp_text_cond string, cond bool, msg gp_L_msg) { ... gp_file ... gp_text_cond ... msg() }
//
// The place of the use goes in as hidden constant arguments: the file, the line and the function
// of the outermost use (a macro used inside a macro passes its own hidden arguments on, so
// #file(), #line() and #func() describe the user's code), and the source text of each parameter
// that the body gives to #text(param) (the text of the innermost use). A lazy parameter becomes a
// type parameter constrained to func() T, which the use fills with a func literal (the lowering
// makes that a template parameter of the instance: no gp::func is built, so a lazy argument
// costs nothing); a wrap macro (one that uses #body()) gets the function's code as a hidden
// func() parameter in the same way.
//
//	#util.Assert(n > 0, note(n))
//	  -> util.Assert("main.gp", 12, "main.f", "n > 0", n > 0, func() string { return note(n) })
//
// On top of a function, a start-style macro becomes the first statement (a func() it returns is
// deferred, unless nil), and a wrap-style macro moves the code of the function into the func
// literal that it gets; each return in that code sets the results (named by the rewrite, if they
// had no name) and returns from the literal:
//
//	#Trace()
//	#Recover()
//	func ratio(a, b int) int { return a / b }
//
//	func ratio(a, b int) (gp_r0 int) {
//		if gp_defer1 := Trace(...); gp_defer1 != nil { defer gp_defer1() }
//		Recover(..., func() { gp_r0 = a / b; return })
//		return
//	}
//
// The function keeps the name of the macro, so that the language server finds the macro by its
// name (hover, definition, references and rename work on #util.Assert as on a function, with the
// right ranges), and a macro is exported exactly when Go would export a function of that name.
// The price is that a plain call Assert(x) or a value f := util.Assert type-checks as the
// function: internal/check finds those uses (Package.MacroUses lists the callees that the rewrite
// made) and says "Assert is a macro: write #Assert(...)".

// A MacroSig is what a package that uses a macro needs to know about its declaration, kept in
// Package.Macros (and in a summary, Package.Summary): the rewrite of a use is made from it.
type MacroSig struct {
	Name string // Assert: also the name of the function that the macro became

	Params   []MacroParam
	Variadic bool // the last parameter is variadic
	Wrap     bool // the body uses #body(): a wrap macro, only for use on top of a func
	// BodyNoEscape is NoEscape for #body(): the body only calls it where it stands (gp_BN).
	BodyNoEscape bool
	Func         bool     // the only result is exactly func(): a start-style macro defers it
	Text         []string // the parameters whose text the body asks for with #text(param), in order

	// Sig is the declaration as written, up to its body, on one line: macro Assert(cond bool, msg lazy string).
	// The language server shows it where it would show the hidden signature of the function.
	Sig      string
	NTParams int // the number of type parameters that the macro declares

	// Pos is where the macro is declared, for messages: file:line.
	File string
	Line int

	bad  bool          // the declaration has errors: its uses are not expanded (the errors are reported once)
	decl *ast.FuncDecl // the declaration (nil in a summary)
}

// A MacroParam is a parameter of a macro as the user wrote it.
type MacroParam struct {
	Name string
	Lazy bool

	// NoEscape says, for a lazy parameter, that the body of the macro only calls it where it stands
	// (f()): never in a func literal, a go or defer statement, an argument of another macro, a
	// variable or a return. The lambda that the use passes can then capture the variables of the
	// caller by reference, since it cannot outlive the call (the lowering: gp_LN_ type parameters).
	NoEscape bool

	// Result is, for a lazy parameter, the type that the thunk returns, spelled so that every
	// use can write it (spellType): own types of the macro's package and types of packages
	// that the macro's file imports are marked, and the use gives them the names that its own
	// file knows.
	Result string
}

const (
	hiddenFile = "gp_file"
	hiddenLine = "gp_line"
	hiddenFunc = "gp_func"
	hiddenBody = "gp_body"
	hiddenText = "gp_text_"
	hiddenLazy = "gp_L_"
	hiddenWrap = "gp_B"
	// the names of the type parameters of a lazy parameter and of the code of a wrap macro that the
	// macro only calls: a lambda for them can capture by reference
	hiddenLazyNoEscape = "gp_LN_"
	hiddenWrapNoEscape = "gp_BN"
	hiddenRes          = "gp_r"
	hiddenDef          = "gp_defer"
)

// builtinNames are the built-in macros: a user macro can't have their names.
var builtinNames = map[string]bool{"file": true, "line": true, "func": true, "text": true,
	"embed": true, "body": true, "Derive": true}

// A macroCtx is the macro declaration whose body is being rewritten.
type macroCtx struct {
	sig    *MacroSig
	params map[string]bool
}

// A useInfo is a use of a user macro, found by the walk and rewritten when the walk has been through
// its arguments.
type useInfo struct {
	sig   *MacroSig
	dep   *Package // the package that declares the macro
	fun   ast.Expr // the function that the use calls (Name, pkg.Name, with its type arguments)
	alias string   // how the use's file calls the macro's package: "" in the package itself and for a dot import
	texts []string // the text of each argument, as written

	callee *MacroUse // what Package.MacroUses has for the callee
}

// macroSigs makes the Package.Macros of the package from its macro declarations, before any
// file is rewritten: a use may come in a file before the declaration, or in another package.
func (b *builtinPass) macroSigs() {
	for _, f := range b.p.Files {
		for _, d := range f.AST.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || !gpast.IsMacroDecl(fd) {
				continue
			}
			b.f = f
			sig := b.macroSig(fd)
			if sig == nil {
				continue
			}
			if old := b.p.Macros[sig.Name]; old != nil {
				b.errorf(fd.Name.Pos(), "macro %s redeclared in this package\n\t%s:%d: other declaration of %s", sig.Name, old.File, old.Line, sig.Name)
				continue
			}
			if b.p.Macros == nil {
				b.p.Macros = make(map[string]*MacroSig)
			}
			b.p.Macros[sig.Name] = sig
		}
	}
}

// macroSig makes the signature of the macro declaration fd.
func (b *builtinPass) macroSig(fd *ast.FuncDecl) *MacroSig {
	name := gpast.MacroBase(fd.Name.Name)
	pos := b.l.fset.Position(fd.Pos())
	sig := &MacroSig{Name: name, File: b.fileNameOf(pos.Filename), Line: pos.Line, decl: fd}
	if builtinNames[name] {
		b.errorf(fd.Name.Pos(), "%s is a built-in macro: choose another name for the macro", name)
		sig.bad = true
	}
	if types.Universe.Lookup(name) != nil {
		// The call that the rewrite writes is Name(...): a predeclared name would be the built-in.
		b.errorf(fd.Name.Pos(), "%s is a predeclared name: a macro with that name would be hidden by the built-in. Choose another name", name)
		sig.bad = true
	}
	if fd.Recv != nil {
		b.errorf(fd.Recv.Pos(), "a macro has no receiver")
		sig.bad = true
	}
	if fd.Body == nil {
		sig.bad = true // the parser reported it: missing macro body
	}
	sig.Sig = b.declText(fd)
	tparams := map[string]bool{}
	if fd.Type.TypeParams != nil {
		for _, fl := range fd.Type.TypeParams.List {
			for _, n := range fl.Names {
				tparams[n.Name] = true
				sig.NTParams++
			}
		}
	}
	params := map[string]bool{}
	if fd.Type.Params != nil {
		list := fd.Type.Params.List
		for i, fl := range list {
			typ := fl.Type
			lazyT, lazy := gpast.Lazy(typ)
			if lazy {
				typ = lazyT
			}
			_, variadic := typ.(*ast.Ellipsis)
			if variadic {
				if i != len(list)-1 || len(fl.Names) > 1 {
					b.errorf(typ.Pos(), "can only use ... with the final parameter")
					sig.bad = true
				}
				sig.Variadic = true // (lazy and ... do not parse together)
			}
			names := fl.Names
			if len(names) == 0 {
				names = []*ast.Ident{{Name: ""}}
			}
			for _, n := range names {
				mp := MacroParam{Name: n.Name, Lazy: lazy}
				if strings.HasPrefix(n.Name, "gp_") {
					b.errorf(n.Pos(), "the name %s is reserved: names that start with gp_ are used by the compiler in macros", n.Name)
					sig.bad = true
				}
				if lazy && !variadic {
					res, err := b.spellType(typ, tparams)
					if err != "" {
						b.errorf(typ.Pos(), "lazy parameter %s: %s", n.Name, err)
						sig.bad = true
					}
					mp.Result = res
				}
				if n.Name != "" && n.Name != "_" {
					params[n.Name] = true
				}
				sig.Params = append(sig.Params, mp)
			}
		}
	}
	// What the body asks for: #body() makes a wrap macro, #text(param) a hidden text parameter.
	if fd.Body != nil {
		asked := map[string]bool{}
		reserved := false
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && strings.HasPrefix(id.Name, "gp_") && !reserved {
				// The rewrite adds gp_file, gp_line, gp_func, gp_text_x and gp_body: a name of the
				// macro that starts with gp_ could hide one of them.
				b.errorf(id.Pos(), "the name %s is reserved: names that start with gp_ are used by the compiler in macros", id.Name)
				sig.bad, reserved = true, true
			}
			if id, ok := n.(*ast.Ident); ok && gpast.IsMacroUse(id.Name) {
				switch id.Name {
				case "#body":
					sig.Wrap = true
				}
			}
			if call, ok := n.(*ast.CallExpr); ok && len(call.Args) == 1 {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "#text" {
					if a, ok := call.Args[0].(*ast.Ident); ok && params[a.Name] {
						asked[a.Name] = true
					}
				}
			}
			return true
		})
		for _, p := range sig.Params {
			if asked[p.Name] {
				sig.Text = append(sig.Text, p.Name)
				delete(asked, p.Name)
			}
		}
	}
	if fd.Body != nil {
		for i := range sig.Params {
			if p := &sig.Params[i]; p.Lazy && p.Name != "" && p.Name != "_" {
				p.NoEscape = onlyCalled(fd.Body, p.Name)
			}
		}
		sig.BodyNoEscape = sig.Wrap && onlyCalled(fd.Body, "")
	}
	if r := fd.Type.Results; r != nil && len(r.List) == 1 && len(r.List[0].Names) == 0 {
		if ft, ok := r.List[0].Type.(*ast.FuncType); ok && ft.Params.NumFields() == 0 && ft.Results.NumFields() == 0 {
			sig.Func = true
		}
	}
	return sig
}

// onlyCalled reports whether the body of a macro only calls the lazy parameter param (or, for the
// empty name, #body()) where it stands: not inside a func literal, a go or defer statement or an
// argument of another macro, and not as a value. #text(param) is fine.
func onlyCalled(body ast.Node, param string) bool {
	ok := true
	var stack []ast.Node
	hostile := func() bool {
		for _, a := range stack {
			switch a := a.(type) {
			case *ast.FuncLit, *ast.GoStmt, *ast.DeferStmt:
				return true
			case *ast.CallExpr:
				if root := useRoot(a.Fun); root != nil && !builtinNames[gpast.MacroBase(root.Name)] {
					return true // an argument of a macro use: a lazy thunk, or a closure of its own
				}
			}
		}
		return false
	}
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		switch n := n.(type) {
		case *ast.Ident:
			if param != "" && n.Name == param {
				parent, isCall := ast.Node(nil), false
				if len(stack) > 0 {
					parent = stack[len(stack)-1]
					c, isC := parent.(*ast.CallExpr)
					isCall = isC && c.Fun == ast.Expr(n)
					if isC && !isCall {
						if f, ok2 := c.Fun.(*ast.Ident); ok2 && f.Name == "#text" {
							break // #text(param)
						}
					}
				}
				if !isCall || hostile() {
					ok = false
				}
			}
		case *ast.CallExpr:
			if id, isID := n.Fun.(*ast.Ident); isID && param == "" && id.Name == "#body" && hostile() {
				ok = false
			}
		}
		stack = append(stack, n)
		return true
	})
	return ok
}

// declText returns the macro declaration fd as written up to its body, with each run of white space
// as one space.
func (b *builtinPass) declText(fd *ast.FuncDecl) string {
	src := b.f.Src
	file := b.l.fset.File(fd.Pos())
	if fd.Body == nil || file == nil || len(src) != file.Size() {
		return "macro " + gpast.MacroBase(fd.Name.Name) + "(...)"
	}
	start, end := file.Offset(fd.Pos()), file.Offset(fd.Body.Lbrace)
	if start > end || end > len(src) {
		return "macro " + gpast.MacroBase(fd.Name.Name) + "(...)"
	}
	return strings.Join(strings.Fields(string(src[start:end])), " ")
}

// spellType returns the type t of a lazy parameter as text that a use can write: the markers
// \x01Name (a type of the macro's package) and \x02path\x03Name (a type of an imported package)
// stand for the names that the use's file knows. The error says why the type can't be spelled.
func (b *builtinPass) spellType(t ast.Expr, tparams map[string]bool) (string, string) {
	var sb strings.Builder
	var fail string
	var walk func(e ast.Expr)
	field := func(fl *ast.FieldList, sep string) {
		first := true
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			n := max(len(f.Names), 1)
			for range n {
				if !first {
					sb.WriteString(sep)
				}
				first = false
				walk(f.Type)
			}
		}
	}
	walk = func(e ast.Expr) {
		if fail != "" {
			return
		}
		switch e := e.(type) {
		case *ast.Ident:
			switch {
			case tparams[e.Name]:
				fail = "the type " + e.Name + " is a type parameter of the macro: a lazy parameter needs a concrete type, written the same way at every use"
			case b.pkgType(e.Name):
				sb.WriteString("\x01" + e.Name)
			default:
				if _, ok := types.Universe.Lookup(e.Name).(*types.TypeName); ok {
					sb.WriteString(e.Name)
				} else {
					fail = "unknown type " + e.Name
				}
			}
		case *ast.ParenExpr:
			sb.WriteString("(")
			walk(e.X)
			sb.WriteString(")")
		case *ast.StarExpr:
			sb.WriteString("*")
			walk(e.X)
		case *ast.ArrayType:
			switch l := e.Len.(type) {
			case nil:
				sb.WriteString("[]")
			case *ast.BasicLit:
				if l.Kind != token.INT {
					fail = "an array length that is not an integer literal can't be written at every use"
				}
				sb.WriteString("[" + l.Value + "]")
			default:
				fail = "an array length that is not an integer literal can't be written at every use"
			}
			walk(e.Elt)
		case *ast.MapType:
			sb.WriteString("map[")
			walk(e.Key)
			sb.WriteString("]")
			walk(e.Value)
		case *ast.ChanType:
			switch e.Dir {
			case ast.SEND:
				sb.WriteString("chan<- ")
			case ast.RECV:
				sb.WriteString("<-chan ")
			default:
				sb.WriteString("chan ")
			}
			walk(e.Value)
		case *ast.FuncType:
			sb.WriteString("func(")
			field(e.Params, ", ")
			sb.WriteString(")")
			if e.Results.NumFields() > 0 {
				sb.WriteString(" (")
				field(e.Results, ", ")
				sb.WriteString(")")
			}
		case *ast.Ellipsis:
			sb.WriteString("...")
			walk(e.Elt)
		case *ast.InterfaceType:
			if e.Methods.NumFields() != 0 {
				fail = "an interface type with methods can't be written at every use: declare a named type"
			}
			sb.WriteString("interface{}")
		case *ast.StructType:
			if e.Fields.NumFields() != 0 {
				fail = "a struct type with fields can't be written at every use: declare a named type"
			}
			sb.WriteString("struct{}")
		case *ast.SelectorExpr:
			var root ast.Expr = e
			for {
				if sel, ok := root.(*ast.SelectorExpr); ok {
					root = sel.X
					continue
				}
				break
			}
			if id, ok := root.(*ast.Ident); ok && id.Name == "C" {
				fail = "a C or C++ type (C.int, std.string, a type of a header) can't be the type of a lazy parameter, because the use would have to write the type with its own header imports: use a plain parameter"
				return
			}
			x, ok := e.X.(*ast.Ident)
			var spec *ast.ImportSpec
			if ok && b.f.Info != nil {
				spec = b.f.Info.PackageNames[x.Name]
			}
			if spec == nil {
				fail = "the type " + types.ExprString(e) + " is not a type of an imported package"
				return
			}
			path, _ := importPath(spec)
			sb.WriteString("\x02" + path + "\x03" + e.Sel.Name)
		default:
			fail = "the type " + types.ExprString(e) + " can't be written at every use"
		}
	}
	walk(t)
	if fail != "" {
		return "", fail
	}
	return sb.String(), ""
}

// pkgType reports whether name is a type declared at package level in the package.
func (b *builtinPass) pkgType(name string) bool {
	if b.typeNames == nil {
		b.typeNames = map[string]bool{}
		for _, f := range b.p.Files {
			for _, d := range f.AST.Decls {
				if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
					for _, s := range gd.Specs {
						b.typeNames[s.(*ast.TypeSpec).Name.Name] = true
					}
				}
			}
		}
	}
	return b.typeNames[name]
}

// macroFun notes the identifier id as the callee of the use u (Package.MacroUses) and returns it.
func (b *builtinPass) macroFun(id *ast.Ident, u *useInfo) *ast.Ident {
	if b.quiet > 0 {
		return id // a look at the signature only: peekSig
	}
	if b.p.MacroUses == nil {
		b.p.MacroUses = make(map[*ast.Ident]*MacroUse)
	}
	mu := &MacroUse{Sig: u.sig, Path: u.dep.ImportPath}
	b.p.MacroUses[id] = mu
	u.callee = mu
	return id
}

// A synth marks the identifier id as written by the rewrite (File.Synthetic) and returns it.
func (b *builtinPass) synth(id *ast.Ident) *ast.Ident {
	if b.f.Synthetic == nil {
		b.f.Synthetic = make(map[*ast.Ident]bool)
	}
	b.f.Synthetic[id] = true
	return id
}

func (b *builtinPass) hidden(name string, pos token.Pos) *ast.Ident {
	return b.synth(&ast.Ident{NamePos: pos, Name: name})
}

// macroDecl rewrites the macro declaration d into the function that it stands for: the body is
// expanded in macro mode (#file() and the others become hidden parameters, #body() a call of one),
// then the signature gets the hidden parameters.
func (b *builtinPass) macroDecl(d *ast.FuncDecl) {
	name := gpast.MacroBase(d.Name.Name)
	sig := b.p.Macros[name]
	if sig == nil || sig.decl != d || d.Body == nil {
		// A second declaration of the same name (reported), or one with no body: nothing to do.
		d.Name = &ast.Ident{NamePos: d.Name.NamePos, Name: name}
		return
	}
	b.macro = &macroCtx{sig: sig, params: map[string]bool{}}
	for _, p := range sig.Params {
		b.macro.params[p.Name] = true
	}
	b.apply(d.Body, &funcName{name: b.prefix + "." + name, top: true})
	b.macro = nil
	if sig.bad {
		return
	}

	pos := d.Name.NamePos
	field := func(name string, typ ast.Expr) *ast.Field {
		return &ast.Field{Names: []*ast.Ident{b.hidden(name, pos)}, Type: typ}
	}
	ident := func(name string) *ast.Ident { return b.hidden(name, pos) }
	var params []*ast.Field
	params = append(params, field(hiddenFile, ident("string")), field(hiddenLine, ident("int")), field(hiddenFunc, ident("string")))
	for _, t := range sig.Text {
		params = append(params, field(hiddenText+t, ident("string")))
	}
	var tparams []*ast.Field
	if d.Type.TypeParams != nil {
		tparams = append(tparams, d.Type.TypeParams.List...)
	}
	funcConstraint := func(results ast.Expr) ast.Expr {
		ft := &ast.FuncType{Func: pos, Params: &ast.FieldList{Opening: pos, Closing: pos}}
		if results != nil {
			ft.Results = &ast.FieldList{List: []*ast.Field{{Type: results}}}
		}
		return &ast.UnaryExpr{OpPos: pos, Op: token.TILDE, X: ft}
	}
	if sig.Wrap {
		tp := hiddenWrap
		if sig.BodyNoEscape {
			tp = hiddenWrapNoEscape
		}
		params = append(params, field(hiddenBody, ident(tp)))
		tparams = append(tparams, &ast.Field{Names: []*ast.Ident{ident(tp)}, Type: funcConstraint(nil)})
	}
	if d.Type.Params != nil {
		for _, fl := range d.Type.Params.List {
			typ, lazy := gpast.Lazy(fl.Type)
			if !lazy {
				params = append(params, fl)
				continue
			}
			for _, n := range fl.Names {
				tp := hiddenLazy + n.Name
				for _, p := range sig.Params {
					if p.Name == n.Name && p.NoEscape {
						tp = hiddenLazyNoEscape + n.Name
					}
				}
				params = append(params, &ast.Field{Names: []*ast.Ident{n}, Type: ident(tp)})
				tparams = append(tparams, &ast.Field{Names: []*ast.Ident{ident(tp)}, Type: funcConstraint(typ)})
			}
		}
	}
	d.Type.Params = &ast.FieldList{Opening: d.Type.Params.Opening, List: params, Closing: d.Type.Params.Closing}
	if len(tparams) > 0 {
		open, closing := pos, pos
		if d.Type.TypeParams != nil {
			open, closing = d.Type.TypeParams.Opening, d.Type.TypeParams.Closing
		}
		d.Type.TypeParams = &ast.FieldList{Opening: open, List: tparams, Closing: closing}
	}
	d.Name = &ast.Ident{NamePos: d.Name.NamePos, Name: sig.Name}
}

// useRoot returns the ident that starts the macro use whose callee is fun: #Name, #alias in
// #alias.Name, and so on through type arguments.
func useRoot(fun ast.Expr) *ast.Ident {
	for {
		switch x := fun.(type) {
		case *ast.Ident:
			if gpast.IsMacroUse(x.Name) {
				return x
			}
			return nil
		case *ast.SelectorExpr:
			fun = x.X
		case *ast.IndexExpr:
			fun = x.X
		case *ast.IndexListExpr:
			fun = x.X
		case *ast.ParenExpr:
			return nil
		default:
			return nil
		}
	}
}

// resolveUse finds the macro that a use calls: its signature, its package, and the function
// expression that calls it in plain Go. The second result is false when it reported an error (or
// when the macro is a built-in one: not a user macro).
func (b *builtinPass) resolveUse(call *ast.CallExpr) (*useInfo, bool) {
	var targs []ast.Expr
	fun := call.Fun
	switch x := fun.(type) {
	case *ast.IndexExpr:
		targs, fun = []ast.Expr{x.Index}, x.X
	case *ast.IndexListExpr:
		targs, fun = x.Indices, x.X
	}
	u := &useInfo{}
	var sel *ast.Ident
	var root *ast.Ident
	switch x := fun.(type) {
	case *ast.Ident:
		root = x
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok && gpast.IsMacroUse(id.Name) {
			root, sel = id, x.Sel
		}
	}
	if root == nil {
		b.errorf(call.Fun.Pos(), "a macro is used as #Name(...) or #pkg.Name(...), not %s", types.ExprString(call.Fun))
		return nil, false
	}
	name := gpast.MacroBase(root.Name)
	if sel == nil {
		// #Name: a macro of this package, or of a dot import.
		if s := b.p.Macros[name]; s != nil {
			u.sig, u.dep = s, b.p
		} else {
			for _, spec := range b.f.AST.Imports {
				if spec.Name == nil || spec.Name.Name != "." {
					continue
				}
				path, _ := importPath(spec)
				if dep := b.p.Imports[path]; dep != nil && dep.Macros[name] != nil {
					u.sig, u.dep = dep.Macros[name], dep
					break
				}
			}
		}
		if u.sig == nil {
			b.errorf(root.Pos(), "undefined macro #%s", name)
			return nil, false
		}
		u.fun = b.macroFun(&ast.Ident{NamePos: root.NamePos + 1, Name: u.sig.Name}, u) // the # is not part of the name
	} else {
		spec := b.f.Info.PackageNames[name]
		if spec == nil {
			b.errorf(root.Pos(), "#%s.%s: %s is not an imported package", name, sel.Name, name)
			return nil, false
		}
		path, _ := importPath(spec)
		dep := b.p.Imports[path]
		if dep == nil {
			return nil, false // the import has an error of its own
		}
		s := dep.Macros[sel.Name]
		if s == nil {
			b.errorf(sel.Pos(), "undefined macro #%s.%s: package %s declares no macro %s", name, sel.Name, dep.Name, sel.Name)
			return nil, false
		}
		if !ast.IsExported(sel.Name) && dep != b.p {
			b.errorf(sel.Pos(), "macro %s is not exported by package %s: it can only be used there", sel.Name, dep.Name)
			return nil, false
		}
		u.sig, u.dep, u.alias = s, dep, name
		u.fun = &ast.SelectorExpr{
			X:   &ast.Ident{NamePos: root.NamePos + 1, Name: name},
			Sel: b.macroFun(&ast.Ident{NamePos: sel.NamePos, Name: s.Name}, u),
		}
	}
	if len(targs) > 0 {
		if u.sig.NTParams == 0 {
			b.errorf(targs[0].Pos(), "macro %s is not generic: it takes no type arguments", u.sig.Name)
			return nil, false
		}
		if len(targs) == 1 {
			u.fun = &ast.IndexExpr{X: u.fun, Lbrack: call.Fun.Pos(), Index: targs[0], Rbrack: call.Fun.End()}
		} else {
			u.fun = &ast.IndexListExpr{X: u.fun, Lbrack: call.Fun.Pos(), Indices: targs, Rbrack: call.Fun.End()}
		}
	}
	return u, true
}

// thunkType writes the type of the thunk's result for the lazy parameter p of the macro in
// use u: the markers of spellType become the names that the file of the use knows.
func (b *builtinPass) thunkType(u *useInfo, p MacroParam, at token.Pos) (ast.Expr, bool) {
	var sb strings.Builder
	s := p.Result
	for len(s) > 0 {
		i := strings.IndexAny(s, "\x01\x02")
		if i < 0 {
			sb.WriteString(s)
			break
		}
		sb.WriteString(s[:i])
		s = s[i:]
		if s[0] == '\x01' {
			// A type of the macro's package.
			j := 1
			for j < len(s) && (s[j] == '_' || s[j] >= '0' && s[j] <= '9' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= 0x80) {
				j++
			}
			name := s[1:j]
			s = s[j:]
			if u.dep != b.p {
				if !ast.IsExported(name) {
					b.errorf(at, "#%s: the type %s of the lazy parameter %s is not exported by package %s, so the use can't write it", u.sig.Name, name, p.Name, u.dep.Name)
					return nil, false
				}
				if u.alias != "" {
					sb.WriteString(u.alias + ".")
				}
			}
			sb.WriteString(name)
			continue
		}
		// A type of a package that the macro's file imports: the file of the use must import it.
		end := strings.IndexByte(s, '\x03')
		path := s[1:end]
		s = s[end+1:]
		j := 0
		for j < len(s) && (s[j] == '_' || s[j] >= '0' && s[j] <= '9' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= 0x80) {
			j++
		}
		name := s[:j]
		s = s[j:]
		alias, ok := b.importName(path)
		if !ok {
			b.errorf(at, "#%s: the lazy parameter %s has the type %s.%s, and this file does not import %q: add the import", u.sig.Name, p.Name, pathBase(path), name, path)
			return nil, false
		}
		if alias != "" {
			sb.WriteString(alias + ".")
		}
		sb.WriteString(name)
	}
	e, err := parser.ParseExprFrom(b.l.fset, "<macro type>", sb.String(), 0)
	if err != nil {
		b.errorf(at, "#%s: internal error: the type %s of the lazy parameter %s does not parse: %v", u.sig.Name, sb.String(), p.Name, err)
		return nil, false
	}
	return e, true
}

func pathBase(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}

// importName returns the name that the file being rewritten gives to the package with the
// import path path ("" for a dot import).
func (b *builtinPass) importName(path string) (string, bool) {
	for _, spec := range b.f.AST.Imports {
		if p, _ := importPath(spec); p != path {
			continue
		}
		if spec.Name != nil {
			switch spec.Name.Name {
			case "_":
				continue
			case ".":
				return "", true
			}
			return spec.Name.Name, true
		}
		if dep := b.p.Imports[path]; dep != nil {
			return dep.Name, true
		}
	}
	return "", false
}

// argTexts returns the text of each argument of the use as written, one for each parameter (the
// texts of a variadic parameter's arguments are joined with ", ").
func (b *builtinPass) argTexts(sig *MacroSig, args []ast.Expr) []string {
	texts := make([]string, len(sig.Params))
	for i := range sig.Params {
		switch {
		case sig.Variadic && i == len(sig.Params)-1:
			var parts []string
			for _, a := range args[min(i, len(args)):] {
				parts = append(parts, b.sourceText(a))
			}
			texts[i] = strings.Join(parts, ", ")
		case i < len(args):
			texts[i] = b.sourceText(args[i])
		}
	}
	return texts
}

// checkArgs reports a use whose arguments don't fit the parameters.
func (b *builtinPass) checkArgs(call *ast.CallExpr, u *useInfo) bool {
	sig := u.sig
	want := len(sig.Params)
	have := len(call.Args)
	ok := true
	switch {
	case call.Ellipsis.IsValid():
		if !sig.Variadic {
			b.errorf(call.Ellipsis, "#%s: cannot use ... in the arguments: the macro is not variadic", sig.Name)
			return false
		}
		if have != want {
			b.errorf(call.Pos(), "#%s: the arguments with ... must be %d, one for each parameter (have %d)", sig.Name, want, have)
			return false
		}
	case sig.Variadic:
		if have < want-1 {
			ok = false
		}
	case have != want:
		ok = false
	}
	if !ok {
		what, wants := "not enough", strconv.Itoa(want)
		pos := call.Rparen
		if sig.Variadic {
			wants = "at least " + strconv.Itoa(want-1)
		}
		if have > want && !sig.Variadic {
			what, pos = "too many", call.Args[want].Pos()
		}
		b.errorf(pos, "#%s: %s arguments (have %d, want %s)\n\tmacro %s is declared at %s:%d", sig.Name, what, have, wants, sig.Name, sig.File, sig.Line)
		return false
	}
	return true
}

// peekSig returns the signature of the macro that the line of macros call uses, or nil, without
// reporting anything.
func (b *builtinPass) peekSig(call *ast.CallExpr) *MacroSig {
	b.quiet++
	defer func() { b.quiet-- }()
	u, ok := b.resolveUse(call)
	if !ok {
		return nil
	}
	return u.sig
}

// useCall makes the call that stands for the use of the macro u, whose call is call: the hidden
// arguments, then the arguments, a lazy one as a func literal. body, if not nil, is the func
// literal with the code of a function, for a macro used on top of it in wrap style.
func (b *builtinPass) useCall(call *ast.CallExpr, u *useInfo, name string, body ast.Expr) *ast.CallExpr {
	sig := u.sig
	pos := call.Pos()
	var args []ast.Expr
	if b.macro != nil {
		// A macro used inside a macro: its place is the place of the outermost use.
		args = append(args, b.hidden(hiddenFile, pos), b.hidden(hiddenLine, pos), b.hidden(hiddenFunc, pos))
	} else {
		args = append(args,
			&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(b.fileName(pos))},
			&ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(b.l.fset.Position(pos).Line)},
			&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(name)})
	}
	for _, t := range sig.Text {
		for i, p := range sig.Params {
			if p.Name == t {
				args = append(args, &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(u.texts[i])})
				break
			}
		}
	}
	if sig.Wrap {
		args = append(args, body)
	}
	for i, a := range call.Args {
		p := sig.Params[min(i, len(sig.Params)-1)]
		if !p.Lazy || sig.Variadic && i >= len(sig.Params)-1 && p.Result == "" {
			args = append(args, a)
			continue
		}
		res, ok := b.thunkType(u, p, a.Pos())
		if !ok {
			return nil
		}
		args = append(args, thunk(a, res))
	}
	nc := &ast.CallExpr{Fun: u.fun, Lparen: call.Lparen, Args: args, Ellipsis: call.Ellipsis, Rparen: call.Rparen}
	u.callee.Call = nc
	return nc
}

// thunk returns func() T { return arg } with the place of arg, for a lazy argument.
func thunk(arg, result ast.Expr) *ast.FuncLit {
	pos, end := arg.Pos(), arg.End()
	return &ast.FuncLit{
		Type: &ast.FuncType{Func: pos, Params: &ast.FieldList{Opening: pos, Closing: pos},
			Results: &ast.FieldList{List: []*ast.Field{{Type: result}}}},
		Body: &ast.BlockStmt{Lbrace: pos, List: []ast.Stmt{&ast.ReturnStmt{Return: pos, Results: []ast.Expr{arg}}}, Rbrace: end - 1},
	}
}

// userMacro handles the pass over the call of a user macro use: it checks where the use is and
// what is in its arguments, and notes the use so that the call is made once the walk has been through the
// arguments (their own macros and built-ins are expanded first).
func (b *builtinPass) userMacro(c *astutil.Cursor, call *ast.CallExpr) {
	u, ok := b.resolveUse(call)
	if !ok || u.sig.bad {
		return
	}
	if !b.checkArgs(call, u) {
		return
	}
	sig := u.sig
	switch c.Parent().(type) {
	case *ast.ExprStmt:
		if sig.Func {
			b.errorf(call.Pos(), "#%s returns a func(): as a statement it would drop the func that ends its work. Use it on top of a func, or call the result: f := #%s(...)", sig.Name, sig.Name)
			return
		}
	case *ast.DeferStmt, *ast.GoStmt:
		b.errorf(call.Pos(), "#%s can't be the call of a defer or go statement: use it before, and defer or start what it returns", sig.Name)
		return
	}
	if sig.Wrap {
		b.errorf(call.Pos(), "#%s uses #body(): it is a wrap macro, which can only be used on top of a func", sig.Name)
		return
	}
	for i, a := range call.Args {
		if p := sig.Params[min(i, len(sig.Params)-1)]; p.Lazy {
			if must := findMust(a); must != nil {
				b.errorf(must.Pos(), "%s: ! can't be used in a lazy macro argument (it would turn into a panic there): evaluate it before the macro", must.Name)
				return
			}
		}
	}
	u.texts = b.argTexts(sig, call.Args)
	if b.uses == nil {
		b.uses = make(map[*ast.CallExpr]*useInfo)
	}
	b.uses[call] = u
}

// findMust returns the name of a must call in the expression e that is not inside a func literal (a
// literal's ! is its own), or nil.
func findMust(e ast.Expr) *ast.Ident {
	var found *ast.Ident
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			switch f := n.Fun.(type) {
			case *ast.Ident:
				if gpast.IsMust(f.Name) && found == nil {
					found = f
				}
			case *ast.SelectorExpr:
				if gpast.IsMust(f.Sel.Name) && found == nil {
					found = f.Sel
				}
			}
		}
		return found == nil
	})
	return found
}

// finishUse replaces the call of a macro use, once its arguments have been walked.
func (b *builtinPass) finishUse(c *astutil.Cursor, call *ast.CallExpr) {
	u := b.uses[call]
	if u == nil {
		return
	}
	delete(b.uses, call)
	if nc := b.useCall(call, u, b.funcNow(), nil); nc != nil {
		c.Replace(nc)
	}
}

// onTop rewrites the function d, which has the lines of macros calls on top of it (in source
// order): the last line is the innermost. ctx is the runtime name of d (#func()).
func (b *builtinPass) onTop(d *ast.FuncDecl, calls []*ast.CallExpr, ctx *funcName) {
	// The declaration and its body start at the first line of macros, whose calls are now in the
	// body: a tool that looks for the node at a place (the language server) finds them. (The
	// func literal of a wrap macro starts at the real brace: wrapFunc.)
	defer func() {
		d.Type.Func = calls[0].Pos()
		if d.Body != nil {
			d.Body.Lbrace = calls[0].Pos()
		}
	}()
	// The arguments are expanded as code of d (they run in it): built-ins and macros in them.
	for _, call := range calls {
		holder := &ast.CallExpr{Fun: ast.NewIdent("_"), Args: call.Args}
		b.apply(holder, ctx)
	}
	for i := len(calls) - 1; i >= 0; i-- {
		call := calls[i]
		// A start-style statement that a wrap macro above it (written before it) takes into its func
		// literal declares its variable inside the literal's range, for the capture analysis of the
		// lowering; the others declare it at their own place, so that the nodes nest by position.
		insideWrap := false
		for _, c := range calls[:i] {
			if sig := b.peekSig(c); sig != nil && sig.Wrap {
				insideWrap = true
			}
		}
		if id, ok := call.Fun.(*ast.Ident); ok && builtinNames[gpast.MacroBase(id.Name)] {
			if gpast.MacroBase(id.Name) == "Derive" {
				b.errorf(call.Pos(), "#Derive is for types: put it on top of a type declaration (macros on top of a declaration: step 10.10)")
			} else {
				b.errorf(call.Pos(), "%s is a built-in macro for use inside code, not on top of a func", id.Name)
			}
			continue
		}
		u, ok := b.resolveUse(call)
		if !ok || !b.checkArgs(call, u) {
			continue
		}
		sig := u.sig
		if d.Body == nil {
			b.errorf(call.Pos(), "#%s: the func %s has no body: a macro on top of it has nothing to work on", sig.Name, d.Name.Name)
			continue
		}
		if sig.bad {
			continue
		}
		u.texts = b.argTexts(sig, call.Args)
		if sig.Wrap {
			if gpast.IsMust(d.Name.Name) {
				b.errorf(call.Pos(), "#%s is a wrap macro: it can't be used on top of the must function %s (yet): write the function without !", sig.Name, d.Name.Name)
				continue
			}
			b.wrapFunc(d, call, u, ctx.name)
			continue
		}
		nc := b.useCall(call, u, ctx.name, nil)
		if nc == nil {
			continue
		}
		var stmt ast.Stmt = &ast.ExprStmt{X: nc}
		if sig.Func {
			b.defers++
			name := hiddenDef + strconv.Itoa(b.defers)
			end := call.End()
			declAt := call.Pos()
			if insideWrap {
				declAt = d.Body.Lbrace // the brace of the code that the wrap macro takes: wrapFunc
			}
			stmt = &ast.IfStmt{
				If: call.Pos(),
				Init: &ast.AssignStmt{
					Lhs:    []ast.Expr{b.hidden(name, declAt)},
					TokPos: call.Pos(),
					Tok:    token.DEFINE,
					Rhs:    []ast.Expr{nc},
				},
				Cond: &ast.BinaryExpr{X: b.hidden(name, end), OpPos: end, Op: token.NEQ, Y: b.synth(&ast.Ident{NamePos: end, Name: "nil"})},
				Body: &ast.BlockStmt{Lbrace: end, List: []ast.Stmt{
					&ast.DeferStmt{Defer: end, Call: &ast.CallExpr{Fun: b.hidden(name, end), Lparen: end, Rparen: end}},
				}, Rbrace: end},
			}
		}
		d.Body.List = append([]ast.Stmt{stmt}, d.Body.List...)
	}
}

// wrapFunc moves the code of the function d into a func literal, which the wrap macro of call gets
// as its gp_body. The code stays as it is: a function with results gets new results, gp_r0..., and
// the literal runs the code as a func literal of its own with the function's old results, whose
// values it assigns to the new ones:
//
//	func f(a int) (n int, err error) {                func f(a int) (gp_r0 int, gp_r1 error) {
//		code                                  ->          W(..., func() {
//	}                                                             gp_r0, gp_r1 = func() (n int, err error) { code }()
//	                                                          })
//	                                                          return
//	                                                  }
//
// so Go's own rules apply to the code (a variable that hides a result, a defer that changes a
// result, return f(), a bare return).
func (b *builtinPass) wrapFunc(d *ast.FuncDecl, call *ast.CallExpr, u *useInfo, name string) {
	body := d.Body
	if rec := directRecover(body); rec != nil {
		b.errorf(rec.Pos(), "recover() called directly by %s, which has the wrap macro #%s: its code runs in a func literal, so recover() would not see the panic. Call it in a deferred func literal", d.Name.Name, u.sig.Name)
	}
	// Everything of the literals starts at the brace of the code: the lowering takes the variables
	// that are declared in that range for the literal's own. Wrap macros nest literals with the same
	// range, which is enough: only the first (innermost) one declares variables (the old results
	// of the code), and no outer literal uses them.
	at := body.Lbrace
	first := !b.wrapped[d]
	b.wrapped[d] = true
	code := body.List
	hasResults := d.Type.Results != nil && len(d.Type.Results.List) > 0
	if !first && b.wrapRet[d] {
		code = code[:len(code)-1] // the return that the first wrap macro put after its call
	}
	var inner *ast.FieldList
	if first && hasResults {
		decl := d.Type.Params.Closing
		if !decl.IsValid() {
			decl = d.Pos()
		}
		var lhs []ast.Expr
		r := d.Type.Results
		inner = &ast.FieldList{Opening: at, Closing: at}
		outer := &ast.FieldList{Opening: r.Opening, Closing: r.Closing}
		n := 0
		for _, fl := range r.List {
			// The literal gets the old results, declared at its brace.
			in := &ast.Field{Type: fl.Type}
			for _, id := range fl.Names {
				in.Names = append(in.Names, &ast.Ident{NamePos: at, Name: id.Name})
			}
			inner.List = append(inner.List, in)
			// The function gets new ones, declared before it.
			out := &ast.Field{Type: fl.Type}
			for range max(len(fl.Names), 1) {
				name := hiddenRes + strconv.Itoa(n)
				n++
				out.Names = append(out.Names, b.hidden(name, decl))
				lhs = append(lhs, b.hidden(name, at))
			}
			outer.List = append(outer.List, out)
		}
		d.Type.Results = outer
		lit := &ast.FuncLit{
			Type: &ast.FuncType{Func: at, Params: &ast.FieldList{Opening: at, Closing: at}, Results: inner},
			Body: &ast.BlockStmt{Lbrace: at, List: code, Rbrace: body.Rbrace},
		}
		code = []ast.Stmt{&ast.AssignStmt{Lhs: lhs, TokPos: at, Tok: token.ASSIGN,
			Rhs: []ast.Expr{&ast.CallExpr{Fun: lit, Lparen: body.Rbrace, Rparen: body.Rbrace}}}}
		b.wrapRet[d] = true
	}
	lit := &ast.FuncLit{
		Type: &ast.FuncType{Func: at, Params: &ast.FieldList{Opening: at, Closing: at}},
		Body: &ast.BlockStmt{Lbrace: at, List: code, Rbrace: body.Rbrace},
	}
	nc := b.useCall(call, u, name, lit)
	if nc == nil {
		return
	}
	// The call spans the code that it holds, so that the nodes nest by position (the language server
	// looks a node up by its place).
	nc.Rparen = body.Rbrace
	list := []ast.Stmt{&ast.ExprStmt{X: nc}}
	if b.wrapRet[d] {
		list = append(list, &ast.ReturnStmt{Return: body.Rbrace})
	}
	d.Body = &ast.BlockStmt{Lbrace: body.Lbrace, List: list, Rbrace: body.Rbrace}
}

// directRecover returns a call of recover() in the code of a function that is not inside a func
// literal of it, or nil.
func directRecover(body *ast.BlockStmt) *ast.CallExpr {
	var found *ast.CallExpr
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "recover" && found == nil {
				found = n
			}
		}
		return found == nil
	})
	return found
}

// topType reports a line of macros on top of a type declaration.
func (b *builtinPass) topType(call *ast.CallExpr) {
	if id, ok := call.Fun.(*ast.Ident); ok && gpast.MacroBase(id.Name) == "Derive" {
		b.errorf(call.Pos(), "macros on top of a declaration are not supported yet (Phase 10, step 10.10): %s", exprName(call.Fun))
		return
	}
	b.errorf(call.Pos(), "a macro on top of a type: only #Derive can be used there (user macros on types are not supported): %s", exprName(call.Fun))
}

// leftovers reports a macro name that no rewrite handled.
func (b *builtinPass) leftovers(f *File) {
	if b.nerr > 0 {
		return // the errors above are why
	}
	for _, d := range f.AST.Decls {
		ast.Inspect(d, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && gpast.IsMacroUse(id.Name) {
				b.errorf(id.Pos(), "%s: a macro can only be used as a call, #Name(...)", id.Name)
			}
			return true
		})
	}
}

// addOnTopUnresolved adds the names in the arguments of the lines of macros on top of a func to
// f.Unresolved, as the parser's resolver would have if it had walked them (it does not walk the lines),
// except the names that the func declares itself (its receiver, parameters and results): the rewrite of
// header packages and namespaces (RewriteFile) takes a name that is not declared in the file for one.
func addOnTopUnresolved(f *ast.File) {
	for i, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		calls := gpast.OnTopOf(f.Decls, i)
		if len(calls) == 0 {
			continue
		}
		own := map[string]bool{}
		for _, fl := range []*ast.FieldList{fd.Recv, fd.Type.Params, fd.Type.Results} {
			if fl == nil {
				continue
			}
			for _, field := range fl.List {
				for _, n := range field.Names {
					own[n.Name] = true
				}
			}
		}
		for _, call := range calls {
			sels := map[*ast.Ident]bool{}
			for _, a := range call.Args {
				ast.Inspect(a, func(n ast.Node) bool {
					switch n := n.(type) {
					case *ast.SelectorExpr:
						sels[n.Sel] = true
					case *ast.KeyValueExpr:
						if id, ok := n.Key.(*ast.Ident); ok {
							sels[id] = true
						}
					case *ast.Ident:
						if !sels[n] && !own[n.Name] && n.Obj == nil {
							f.Unresolved = append(f.Unresolved, n)
						}
					}
					return true
				})
			}
		}
	}
}
