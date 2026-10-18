package check

import (
	"cmp"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// gates holds the phase gates of one program: the phase they allow, and
// what they learn about the packages checked so far.
type gates struct {
	fset  *token.FileSet
	phase int

	// opaqueFields are the embedded struct fields whose type is a C/C++
	// type (its type expression uses C), in the packages checked so far:
	// a selector that go/types can't find on a struct with such a field
	// may be a promoted C++ member (Phase 12).
	opaqueFields map[*types.Var]bool

	// opaqueTypes are the type names declared with a C/C++ type
	// (type Vec = std.vector[int], type Vec2 std.vector[int]): embedding
	// one embeds a C/C++ type too.
	opaqueTypes map[*types.TypeName]bool
}

// newGates returns the gates of a program, which allow the features of
// phase and earlier ones.
func newGates(fset *token.FileSet, phase int) *gates {
	return &gates{fset: fset, phase: phase, opaqueFields: make(map[*types.Var]bool), opaqueTypes: make(map[*types.TypeName]bool)}
}

// apply runs the gates over the checked or unchecked package p (after the
// packages it imports). It adds the errors to p.Errors, or gives p a new
// list when a gate error takes the place of another error. It does nothing
// for a standard package.
func (gs *gates) apply(p *Package) {
	if p.Load.Std {
		return
	}
	if !p.Checked() {
		gateUnchecked(gs.fset, p, gs.phase)
		return
	}
	g := &gateChecker{
		gates:     gs,
		pkg:       p,
		info:      p.Info,
		funs:      make(map[ast.Expr]bool),
		discarded: make(map[*ast.CallExpr]bool),
		declTypes: make(map[ast.Expr]bool),
		cppIdents: make(map[token.Pos]bool),
	}
	g.hidden, g.skipTypes = p.rewriteNodes()
	for _, f := range p.Load.Files {
		g.checkFile(f)
	}
	for _, sel := range g.unresolved {
		g.checkPromoted(sel)
	}
	g.checkTypes()
	g.apply()
}

// A finding is a gate error: msg at pos (to end), for a feature of the
// phase. types says that the types (checkTypes) found it, not the syntax.
type finding struct {
	pos, end token.Pos
	msg      string
	phase    int
	types    bool
}

// better reports whether a is a better error than b at the same place: the
// earlier phase (the first thing to build), then one that the syntax shows,
// then by the message, then the shorter range, so that the choice never
// depends on the order of a map.
func (a finding) better(b finding) bool {
	if a.phase != b.phase {
		return a.phase < b.phase
	}
	if a.types != b.types {
		return !a.types
	}
	if a.msg != b.msg {
		return a.msg < b.msg
	}
	return a.end < b.end
}

// A gateKey is a message in a file: each file gets the first use of each
// feature only.
type gateKey struct {
	file string
	msg  string
}

// A replacement is a gate error that takes the place of the go/types error
// at the same position whose message match accepts. Without such an error,
// there is no gate error either.
type replacement struct {
	finding
	match func(msg string) bool
}

// A gateChecker finds the uses of gated features in one checked package.
type gateChecker struct {
	*gates
	pkg  *Package
	info *types.Info

	// found are the gate errors, before apply keeps one at each place and
	// the first of each message in each file.
	found []finding

	// inTypes is set while checkTypes runs.
	inTypes bool

	// replacements are the gate errors that take the place of go/types
	// errors, at every use.
	replacements []replacement

	// funs are the expressions called as functions (the Fun of a call,
	// without parentheses and without its type arguments): using them is
	// calling them, not taking a function value.
	funs map[ast.Expr]bool

	// discarded are the calls whose results are dropped: a call statement,
	// and the call of a defer or go statement.
	discarded map[*ast.CallExpr]bool

	// declTypes are the func types of declarations, literals and
	// interface methods, which are no function values, and the type
	// parameter lists, whose constraints (any, comparable, ~int | ~string)
	// are generics, not interface values.
	declTypes  map[ast.Expr]bool
	typeParams []*ast.FieldList

	// cppIdents holds the positions of the names that the header and
	// namespace rewrite replaced by C (load.CppUse.Ident): a C at one of
	// them is not a C that the user wrote.
	cppIdents map[token.Pos]bool

	// unresolved are the selectors that go/types found nothing for, which
	// checkPromoted looks at once every struct of the package is known.
	unresolved []*ast.SelectorExpr

	// hidden and skipTypes are the nodes of the rewrite of conversions
	// (Package.rewriteNodes), which the gates don't look at: the user
	// wrote T(x).
	hidden, skipTypes map[ast.Node]bool
}

// feature reports a use of the feature from pos to end if the gate
// rejects it, and returns whether it does.
func (g *gateChecker) feature(id featureID, pos, end token.Pos) bool {
	f := &features[id]
	if f.allowed(g.phase) {
		return false
	}
	g.report(pos, end, f.Message(), f.Phase)
	return true
}

// node reports a use of the feature by the node n.
func (g *gateChecker) node(id featureID, n ast.Node) bool {
	return g.feature(id, n.Pos(), n.End())
}

// report reports the error msg at pos, for a feature of the phase. apply
// keeps only one error at each place (finding.better), and then the first
// one of each message in each file.
func (g *gateChecker) report(pos, end token.Pos, msg string, phase int) {
	if pos.IsValid() {
		g.found = append(g.found, finding{pos: pos, end: end, msg: msg, phase: phase, types: g.inTypes})
	}
}

// replace reports the gate error msg at pos (to end) in the place of the
// go/types error at pos that match accepts.
func (g *gateChecker) replace(pos, end token.Pos, msg string, match func(string) bool) {
	g.replacements = append(g.replacements, replacement{finding{pos: pos, end: end, msg: msg}, match})
}

// checkFile finds the uses that the syntax and the loader show in f.
func (g *gateChecker) checkFile(f *load.File) {
	file := f.AST
	if f.Test {
		g.feature(featTestFiles, file.Package, file.Name.End())
	}
	g.checkImports(f)
	g.checkComments(file)
	for _, d := range f.Directives {
		id := featDirectives
		if d.Name == "no-win32-defaults" {
			id = featNoWin32Defaults
		}
		g.feature(id, d.Pos, d.Pos+token.Pos(len("//gocpp:")+len(d.Name)))
	}
	if f.Rewrite != nil {
		for _, u := range f.Rewrite.Uses {
			g.cppIdents[u.Ident.Pos()] = true
			if u.Kind == load.NamespaceUse {
				g.node(featNamespaces, u.Ident)
			}
		}
	}
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			if decl.Tok == token.VAR {
				g.node(featPackageVars, decl)
			}
		case *ast.FuncDecl:
			if decl.Recv != nil {
				g.node(featMethods, decl.Name)
			} else if decl.Name.Name == "init" {
				g.node(featInitFuncs, decl.Name)
			}
			if decl.Body == nil {
				g.node(featNativeFuncs, decl.Name)
			}
			if decl.Doc != nil {
				for _, c := range decl.Doc.List {
					if strings.HasPrefix(c.Text, "//export ") {
						g.node(featExport, c)
					}
				}
			}
		}
	}
	ast.Inspect(file, g.visit)
}

// checkImports finds the gated imports of f: headers, namespace renames,
// standard packages of later phases and packages of the module.
func (g *gateChecker) checkImports(f *load.File) {
	if info := f.Info; info != nil {
		for _, inc := range info.Includes {
			end := inc.Pos + token.Pos(len(inc.Path)+2)
			g.feature(featHeaderImports, inc.Pos, end)
			if inc.Kind == load.LocalHeader && load.IsModuleHeaderPath(inc.Path) {
				g.feature(featModuleHeaders, inc.Pos, end)
			}
		}
		for _, r := range info.NamespaceRenames {
			g.feature(featNamespaces, r.Pos, r.Spec.End())
		}
	}
	for _, spec := range f.AST.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path == "C" || path == load.CgohPath {
			continue
		}
		if dep := g.pkg.Load.Imports[path]; dep != nil && !dep.Std {
			g.node(featMultiPackage, spec.Path)
			continue
		}
		phase := otherStdPhase
		if row := lookupStdPackage(path); row != nil {
			phase = row.Phase
		}
		if phase > g.phase {
			g.report(spec.Path.Pos(), spec.Path.End(), pkgMessage(path, phase), phase)
		}
	}
}

// checkComments finds the //go: lines that gocpp code can't use.
func (g *gateChecker) checkComments(file *ast.File) {
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			switch {
			case strings.HasPrefix(c.Text, "//go:linkname"):
				g.node(featLinkname, c)
			case strings.HasPrefix(c.Text, "//go:embed"):
				g.node(featGoEmbed, c)
			}
		}
	}
}

// visit finds the uses that the syntax shows, with the help of the types.
func (g *gateChecker) visit(n ast.Node) bool {
	if g.hidden[n] {
		return false
	}
	switch n := n.(type) {
	case *ast.FuncLit:
		g.node(featClosures, n)
		g.declTypes[n.Type] = true
	case *ast.FuncDecl:
		g.declTypes[n.Type] = true
	case *ast.ExprStmt:
		if call, ok := ast.Unparen(n.X).(*ast.CallExpr); ok {
			g.discarded[call] = true
		}
	case *ast.DeferStmt:
		g.node(featDefer, n)
		g.discarded[n.Call] = true
	case *ast.GoStmt:
		g.node(featGo, n)
		g.discarded[n.Call] = true
	case *ast.SelectStmt:
		g.node(featSelect, n)
	case *ast.SendStmt:
		g.node(featChannels, n)
	case *ast.ChanType:
		g.node(featChannels, n)
	case *ast.UnaryExpr:
		switch n.Op {
		case token.AND:
			g.node(featPointers, n)
		case token.ARROW:
			g.node(featChannels, n)
		}
	case *ast.StarExpr:
		g.node(featPointers, n)
	case *ast.CompositeLit:
		g.node(featComposite, n)
	case *ast.TypeSpec:
		if g.hasC(n.Type) {
			if obj, ok := g.info.Defs[n.Name].(*types.TypeName); ok {
				g.opaqueTypes[obj] = true
			}
		}
		if n.Assign.IsValid() {
			g.node(featTypeAliases, n)
		} else {
			g.node(featTypeDecls, n)
		}
		if n.TypeParams != nil {
			g.node(featGenerics, n.TypeParams)
			g.typeParams = append(g.typeParams, n.TypeParams)
		}
	case *ast.FuncType:
		if n.TypeParams != nil {
			g.node(featGenerics, n.TypeParams)
			g.typeParams = append(g.typeParams, n.TypeParams)
		}
		if l := n.Params.List; len(l) > 0 {
			if e, ok := l[len(l)-1].Type.(*ast.Ellipsis); ok {
				g.node(featVariadic, e)
			}
		}
	case *ast.StructType:
		for _, f := range n.Fields.List {
			if len(f.Names) > 0 {
				continue
			}
			g.node(featEmbedding, f)
			if g.hasC(f.Type) {
				if v, ok := g.info.Defs[embeddedIdent(f.Type)].(*types.Var); ok {
					g.opaqueFields[v] = true
				}
			}
		}
	case *ast.InterfaceType:
		for _, f := range n.Methods.List {
			if len(f.Names) > 0 {
				g.declTypes[f.Type] = true // a method, no function value
			}
		}
	case *ast.TypeAssertExpr:
		if n.Type != nil { // x.(type) is part of a type switch
			g.node(featTypeAssert, n)
		}
	case *ast.TypeSwitchStmt:
		g.node(featTypeSwitch, n)
	case *ast.SliceExpr:
		if g.isString(n.X) {
			g.node(featStringSlice, n)
		}
	case *ast.RangeStmt:
		if t := g.typeOf(n.X); t != nil {
			switch u := t.Underlying().(type) {
			case *types.Basic:
				if u.Info()&types.IsString != 0 {
					g.feature(featRangeString, n.For, n.X.End())
				}
			case *types.Signature:
				g.feature(featRangeFunc, n.For, n.X.End())
			case *types.Chan:
				g.feature(featChannels, n.For, n.X.End())
			}
		}
	case *ast.ArrayType:
		if n.Len != nil && g.hasC(n.Len) && !g.hasUnsafeSize(n.Len) {
			g.node(featCConstants, n.Len)
		}
	case *ast.GenDecl:
		if n.Tok == token.CONST {
			for _, spec := range n.Specs {
				for _, v := range spec.(*ast.ValueSpec).Values {
					if g.hasC(v) && !g.hasUnsafeSize(v) {
						g.node(featCConstants, v)
					}
				}
			}
		}
	case *ast.CallExpr:
		g.visitCall(n)
	case *ast.SelectorExpr:
		g.visitSelector(n)
	}
	return true
}

// embeddedIdent returns the name of the embedded field whose type
// expression is e (go/types records the field at it), or nil.
func embeddedIdent(e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x
		case *ast.StarExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.SelectorExpr:
			return x.Sel
		default:
			return nil
		}
	}
}

// visitCall finds the gated built-in functions, conversions and format
// strings, and the nil arguments of interface parameters, and marks the
// called expression.
func (g *gateChecker) visitCall(call *ast.CallExpr) {
	if name, typ := cgohCall(call, g.info); name != "" {
		// A rewritten conversion: the gates of T(x).
		g.funs[call.Fun] = true
		if tv, ok := g.info.Types[typ]; ok && name != valueName {
			g.checkConversion(call, tv.Type, call.Args[0])
		}
		return
	}
	fun := ast.Unparen(call.Fun)
	g.funs[fun] = true
	switch x := fun.(type) {
	case *ast.IndexExpr:
		g.funs[ast.Unparen(x.X)] = true
	case *ast.IndexListExpr:
		g.funs[ast.Unparen(x.X)] = true
	}
	if tv, ok := g.info.Types[fun]; ok && tv.IsType() {
		if len(call.Args) == 1 {
			g.checkConversion(call, tv.Type, call.Args[0])
		}
		return
	}
	var id *ast.Ident
	switch x := fun.(type) {
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		id = x.Sel
	}
	var obj types.Object
	if id != nil {
		obj = g.info.Uses[id]
	}
	if b, ok := obj.(*types.Builtin); ok {
		if row := lookupBuiltin(b.Name()); row != nil && row.Phase > g.phase {
			g.report(fun.Pos(), fun.End(), row.message(), row.Phase)
		}
		if b.Name() == "panic" && len(call.Args) == 1 {
			if t := g.typeOf(call.Args[0]); t != nil {
				if _, basic := t.(*types.Basic); !basic || t == types.Typ[types.UntypedNil] {
					g.node(featPanicValue, call.Args[0])
				}
			}
		}
		return
	}
	if fn, ok := obj.(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == "fmt" && fn.Parent() == fn.Pkg().Scope() {
		switch fn.Name() {
		case "Printf", "Sprintf", "Errorf":
			if len(call.Args) > 0 {
				g.checkFormat(call.Args[0])
			}
		}
	}
	g.checkNilArgs(call, fun)
}

// checkNilArgs finds a nil argument of an interface parameter, such as
// fmt.Println(nil): a nil interface value, which needs Phase 05. (Each
// other argument has a type, and the gates of that type decide.)
func (g *gateChecker) checkNilArgs(call *ast.CallExpr, fun ast.Expr) {
	t := g.typeOf(fun)
	if t == nil {
		return
	}
	sig, ok := t.Underlying().(*types.Signature)
	if !ok {
		return
	}
	params := sig.Params()
	for i, arg := range call.Args {
		var pt types.Type
		switch {
		case sig.Variadic() && i >= params.Len()-1:
			pt = params.At(params.Len() - 1).Type()
			if s, ok := pt.Underlying().(*types.Slice); ok && !call.Ellipsis.IsValid() {
				pt = s.Elem()
			}
		case i < params.Len():
			pt = params.At(i).Type()
		default:
			continue
		}
		if types.IsInterface(pt) && g.info.TypeOf(arg) == types.Typ[types.UntypedNil] {
			g.node(featAny, arg)
		}
	}
}

// checkFormat finds the gated parts of the format string of fmt.Printf,
// Sprintf or Errorf: a format that is not a constant, verbs, flags, width,
// precision, * and argument indexes. It follows the parsing of Go's
// fmt.(*pp).doPrintf.
func (g *gateChecker) checkFormat(arg ast.Expr) {
	tv := g.info.Types[arg]
	if tv.Value == nil || tv.Value.Kind() != constant.String {
		g.node(featFormatDynamic, arg)
		return
	}
	f := constant.StringVal(tv.Value)
	pos, end := arg.Pos(), arg.End()
	for i := 0; i < len(f); {
		if f[i] != '%' {
			i++
			continue
		}
		i++
		for i < len(f) && strings.IndexByte("#0+- ", f[i]) >= 0 {
			if f[i] == '#' {
				g.feature(featFormatMore, pos, end)
			} else {
				g.feature(featFormatFlags, pos, end)
			}
			i++
		}
		// An argument index, the width and the precision, each with *
		// or digits, then maybe another argument index.
		for part := 0; part < 4 && i < len(f); part++ {
			switch {
			case f[i] == '[':
				g.feature(featFormatMore, pos, end)
				for i < len(f) && f[i] != ']' {
					i++
				}
				i++
			case f[i] == '*':
				g.feature(featFormatMore, pos, end)
				i++
			case f[i] == '.' && i+1 < len(f) || '0' <= f[i] && f[i] <= '9':
				// A . at the end is no precision but the verb, like in
				// Go's doPrintf.
				g.feature(featFormatFlags, pos, end)
				if f[i] == '.' {
					i++
				}
				for i < len(f) && '0' <= f[i] && f[i] <= '9' {
					i++
				}
			}
		}
		if i >= len(f) {
			break // %!(NOVERB), which Phase 02 prints like Go
		}
		verb, size := utf8.DecodeRuneInString(f[i:])
		i += size
		if phase := verbPhase(verb); phase > g.phase {
			g.report(pos, end, verbMessage(verb, phase), phase)
		}
	}
}

// checkConversion finds a conversion between strings and bytes, runes or
// integers: T(arg) with T a string type and arg not a string, or T a slice
// type and arg a string.
func (g *gateChecker) checkConversion(call *ast.CallExpr, to types.Type, arg ast.Expr) {
	from := g.typeOf(arg)
	if from == nil {
		return
	}
	toString, fromString := isStringType(to), isStringType(from)
	_, toSlice := to.Underlying().(*types.Slice)
	if toString && !fromString || toSlice && fromString {
		g.node(featStringConv, call)
	}
}

// visitSelector finds the uses of C that the user wrote and the members of
// standard packages that the stubs don't have yet (errors.Wrapf), and
// keeps the selectors that go/types found nothing for (checkPromoted).
func (g *gateChecker) visitSelector(sel *ast.SelectorExpr) {
	if x, ok := sel.X.(*ast.Ident); ok {
		if pn, ok := g.info.Uses[x].(*types.PkgName); ok {
			path := pn.Imported().Path()
			if path == "C" {
				if !g.cppIdents[x.Pos()] {
					id := featCNames
					if sel.Sel.Name == "wstr" || sel.Sel.Name == "GoStringW" {
						id = featWideStrings
					}
					g.node(id, sel)
				}
				return
			}
			if path == load.CgohPath && CgoHelperOf(sel, g.info) != "" {
				g.node(featCNames, sel) // C.GoString(p), which Check rewrote (cgohelpers.go)
				return
			}
			if g.info.Uses[sel.Sel] == nil {
				// Not in the stub, like errors.Wrapf: go/types says
				// "undefined: errors.Wrapf", the gate says when it comes.
				if m := lookupMember(path, sel.Sel.Name); m != nil && m.Phase > g.phase {
					want := "undefined: " + x.Name + "." + sel.Sel.Name
					g.replace(sel.Sel.Pos(), sel.Sel.End(), m.message(), func(msg string) bool { return msg == want })
				}
			}
			return
		}
	}
	if g.info.Selections[sel] == nil {
		g.unresolved = append(g.unresolved, sel)
	} else if cMayShadow(g.pkg.Types, g.typeOf(sel.X), sel.Sel.Name) {
		g.node(featPromotedCppShadow, sel.Sel)
	}
}

// checkPromoted reports a selector that go/types found nothing for, on a
// struct that embeds more than one C/C++ type: it is a promoted member of
// one of them, but check can't tell which one (Phase 12, typed interop), not
// an error. With one embedded C/C++ type the selector is fine (promoted.go).
func (g *gateChecker) checkPromoted(sel *ast.SelectorExpr) {
	if features[featPromotedCpp].allowed(g.phase) {
		return
	}
	t := g.typeOf(sel.X)
	if t == nil {
		return
	}
	if p, ok := t.Underlying().(*types.Pointer); ok {
		t = p.Elem()
	}
	if !g.embedsOpaque(t, make(map[types.Type]bool)) {
		return
	}
	var found [][]int
	opaqueEmbedded(t, nil, make(map[*types.Struct]bool), &found)
	if len(found) == 1 {
		// One C/C++ type: its member (promoted.go) when sel.X is a value. A method expression
		// (MV.size, (*MV).push_back) has no C++ form yet: its own gate. Any other error stays.
		if tv := g.info.Types[sel.X]; tv.IsType() {
			prefix := g.pkg.exprString(sel) + " undefined ("
			g.replace(sel.Sel.Pos(), sel.Sel.End(), features[featPromotedCppExpr].Message(), func(msg string) bool {
				return strings.HasPrefix(msg, prefix) && strings.Contains(msg, " has no ")
			})
		}
		return
	}
	// The messages show rewritten conversions as the user wrote them.
	prefix := g.pkg.exprString(sel) + " undefined ("
	g.replace(sel.Sel.Pos(), sel.Sel.End(), features[featPromotedCpp].Message(), func(msg string) bool {
		// Not "cannot refer to unexported field": that is a real error.
		return strings.HasPrefix(msg, prefix) && strings.Contains(msg, " has no field or method ")
	})
}

// embedsOpaque reports whether t is a struct with an embedded field of a
// C/C++ type, directly or through embedded structs.
func (g *gateChecker) embedsOpaque(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	s, ok := t.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for i := range s.NumFields() {
		f := s.Field(i)
		if !f.Embedded() {
			continue
		}
		if g.opaqueFields[f] {
			return true
		}
		ft := f.Type()
		if p, ok := ft.Underlying().(*types.Pointer); ok {
			ft = p.Elem()
		}
		if g.isOpaqueType(ft) {
			return true
		}
		if g.embedsOpaque(ft, seen) {
			return true
		}
	}
	return false
}

// isOpaqueType reports whether t is a type name declared with a C/C++
// type, or an alias of one (through other aliases too).
func (g *gateChecker) isOpaqueType(t types.Type) bool {
	for range 100 { // a bound, in case of a broken alias cycle
		switch x := t.(type) {
		case *types.Named:
			return g.opaqueTypes[x.Obj()]
		case *types.Alias:
			if g.opaqueTypes[x.Obj()] {
				return true
			}
			t = x.Rhs()
		default:
			return false
		}
	}
	return false
}

// hasC reports whether the expression uses C or C++: a selector on the
// package C (after the header and namespace rewrite, every C/C++ name is
// one).
func (g *gateChecker) hasC(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if found {
			return false
		}
		if id, ok := n.(*ast.Ident); ok {
			if pn, ok := g.info.Uses[id].(*types.PkgName); ok && pn.Imported().Path() == "C" {
				found = true
			}
		}
		return true
	})
	return found
}

// checkTypes finds the uses that the types show: the types written in the
// code, the types of the declared variables and of the values, the
// instances of generic functions and types, and the members of standard
// packages that come later than their package.
func (g *gateChecker) checkTypes() {
	g.inTypes = true
	for e, tv := range g.info.Types {
		if g.skipTypes[e] {
			continue
		}
		switch {
		case tv.IsType():
			if g.declTypes[e] || g.inTypeParams(e) {
				// The signature of a func declaration or literal: its
				// parameter types are type expressions of their own. Or
				// a constraint: generics (visit).
				continue
			}
			g.typeUses(tv.Type, e.Pos(), e.End(), &typeWalk{seen: make(map[types.Type]bool)})
		case tv.IsValue():
			if g.funs[e] || tv.Type == nil {
				continue
			}
			if _, ok := tv.Type.(*types.Tuple); ok {
				// A call with several results: the variables that get
				// them have their own types, but a blank (_) has none,
				// and an argument (f(g())) neither. Only a call whose
				// results are dropped needs nothing.
				if call, ok := e.(*ast.CallExpr); ok && g.discarded[call] {
					continue
				}
			}
			if _, ok := e.(*ast.FuncLit); ok {
				continue // a closure (visit)
			}
			g.typeUses(tv.Type, e.Pos(), e.End(), &typeWalk{seen: make(map[types.Type]bool)})
		}
	}
	for id, obj := range g.info.Defs {
		if g.skipTypes[id] {
			continue
		}
		switch obj := obj.(type) {
		case *types.Var, *types.Const:
			g.typeUses(obj.Type(), id.Pos(), id.End(), &typeWalk{seen: make(map[types.Type]bool)})
		case *types.TypeName:
			// type WP weak.Pointer[T]: the intrinsic (weak.Pointer[T] itself and its aliases)
			// is a runtime class; a type defined over it waits for the generics.
			if f := &features[featGenerics]; !obj.IsAlias() && weakDefined(obj.Type()) && !f.allowed(g.phase) {
				g.report(id.Pos(), id.End(), gateMessage("a type defined as weak.Pointer[T]", false, f.Phase), f.Phase)
			}
		}
	}
	for id := range g.info.Instances {
		if g.skipTypes[id] {
			continue
		}
		if obj := g.info.Uses[id]; obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "weak" {
			continue // a compiler intrinsic (Phase 03 README §2.2)
		}
		g.node(featGenerics, id)
	}
	for id, obj := range g.info.Uses {
		if g.skipTypes[id] {
			continue
		}
		if m := laterMember(obj); m != nil && m.Phase > g.phase {
			g.report(id.Pos(), id.End(), m.message(), m.Phase)
		}
	}
}

// inTypeParams reports whether e is in a type parameter list.
func (g *gateChecker) inTypeParams(e ast.Expr) bool {
	for _, l := range g.typeParams {
		if l.Pos() <= e.Pos() && e.End() <= l.End() {
			return true
		}
	}
	return false
}

// laterMember returns the row of obj if it is a member of a standard
// package that comes in another phase than its package (fmt.Sprintf),
// also when it is used through a dot import, or a method of a type of
// such a package (time.Time.Format). A method without a row of its own
// comes with its type. Otherwise laterMember returns nil.
func laterMember(obj types.Object) *stdMember {
	pkg := obj.Pkg()
	if pkg == nil {
		return nil
	}
	if fn, ok := obj.(*types.Func); ok {
		if recv := fn.Signature().Recv(); recv != nil {
			t := recv.Type()
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}
			named, ok := types.Unalias(t).(*types.Named)
			if !ok || named.Obj().Pkg() != pkg {
				return nil
			}
			typeName := named.Obj().Name()
			for i := range stdMembers {
				if m := &stdMembers[i]; m.Pkg == pkg.Path() && m.Name == typeName+"."+fn.Name() {
					return laterThanPackage(m)
				}
			}
			return laterThanPackage(lookupMember(pkg.Path(), typeName))
		}
	}
	if obj.Parent() != pkg.Scope() {
		return nil
	}
	return laterThanPackage(lookupMember(pkg.Path(), obj.Name()))
}

// laterThanPackage returns m if it comes later than its package, else nil:
// the import of the package has the error then.
func laterThanPackage(m *stdMember) *stdMember {
	if m == nil {
		return nil
	}
	if row := lookupStdPackage(m.Pkg); row != nil && m.Phase <= row.Phase {
		return nil
	}
	return m
}

// A typeWalk is the state of one typeUses walk.
type typeWalk struct {
	seen map[types.Type]bool
	done bool // a gate error was reported
}

// feature reports a use of the feature, if the gate rejects it, and then
// ends the walk.
func (w *typeWalk) feature(g *gateChecker, id featureID, pos, end token.Pos) bool {
	if g.feature(id, pos, end) {
		w.done = true
	}
	return w.done
}

// typeUses reports a feature that the type t needs, at pos: only one, the
// outermost gated part of the type (the first one, in the order of the
// type), so that one use gives one error. A []any gives "composite types"
// before Phase 03 and "any values" from then on; the func type of fmt.Println
// gives "function values" before Phase 03, and from then on "any values"
// (its parameters) but not also "error values" (its results). It doesn't
// look inside named types: a named type of the package is a type
// declaration (gated where it is declared), and one of another package is
// gated with its package.
func (g *gateChecker) typeUses(t types.Type, pos, end token.Pos, w *typeWalk) {
	if t == nil || w.seen[t] || w.done {
		return
	}
	w.seen[t] = true
	switch t := types.Unalias(t).(type) {
	case *types.Basic:
		if t.Info()&types.IsComplex != 0 {
			w.feature(g, featComplex, pos, end)
		}
	case *types.Pointer:
		if !w.feature(g, featPointers, pos, end) {
			g.typeUses(t.Elem(), pos, end, w)
		}
	case *types.Slice:
		if !w.feature(g, featComposite, pos, end) {
			g.typeUses(t.Elem(), pos, end, w)
		}
	case *types.Array:
		if !w.feature(g, featComposite, pos, end) {
			g.typeUses(t.Elem(), pos, end, w)
		}
	case *types.Map:
		if !w.feature(g, featComposite, pos, end) {
			g.typeUses(t.Key(), pos, end, w)
			g.typeUses(t.Elem(), pos, end, w)
		}
	case *types.Struct:
		// An embedded field is in the syntax (visit): a struct type
		// that isn't is a named type of another package.
		if !w.feature(g, featComposite, pos, end) {
			for i := range t.NumFields() {
				g.typeUses(t.Field(i).Type(), pos, end, w)
			}
		}
	case *types.Chan:
		if !w.feature(g, featChannels, pos, end) {
			g.typeUses(t.Elem(), pos, end, w)
		}
	case *types.Signature:
		if !w.feature(g, featFuncValues, pos, end) {
			g.typeUses(t.Params(), pos, end, w)
			g.typeUses(t.Results(), pos, end, w)
		}
	case *types.Tuple:
		for v := range t.Variables() {
			g.typeUses(v.Type(), pos, end, w)
		}
	case *types.Interface:
		switch {
		case !t.IsMethodSet() || t.IsComparable():
			w.feature(g, featGenerics, pos, end) // a constraint with a type set
		case t.NumMethods() == 0 && t.NumEmbeddeds() == 0:
			w.feature(g, featAny, pos, end)
		default:
			w.feature(g, featInterfaces, pos, end)
		}
	case *types.TypeParam:
		w.feature(g, featGenerics, pos, end)
	case *types.Named:
		obj := t.Obj()
		if obj.Pkg() == nil { // a predeclared type
			switch obj.Name() {
			case "error":
				w.feature(g, featErrors, pos, end)
			case "comparable":
				w.feature(g, featGenerics, pos, end)
			}
		}
		if args := t.TypeArgs(); args.Len() > 0 {
			if (obj.Pkg() == nil || obj.Pkg().Path() != "weak") && w.feature(g, featGenerics, pos, end) {
				return
			}
			for a := range args.Types() {
				g.typeUses(a, pos, end, w)
			}
		}
	}
}

// typeOf returns the type of the expression, or nil if it is not known
// or invalid.
func (g *gateChecker) typeOf(e ast.Expr) types.Type {
	t := g.info.TypeOf(e)
	if t == nil || t == types.Typ[types.Invalid] {
		return nil
	}
	return t
}

// isString reports whether the expression has a string type.
func (g *gateChecker) isString(e ast.Expr) bool {
	t := g.typeOf(e)
	return t != nil && isStringType(t)
}

// isStringType reports whether t is a string type (also untyped).
func isStringType(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsString != 0
}

// apply adds the gate errors to the package. A replacement takes the place
// of its go/types error, so that the errors keep the order they were found
// in (the limit of 10 errors keeps the first 10 found); the other gate
// errors come after the go/types errors, sorted by position.
func (g *gateChecker) apply() {
	if len(g.found) == 0 && len(g.replacements) == 0 {
		return
	}
	toDiag := func(f finding) diag.Diagnostic {
		d := diag.Diagnostic{Pos: g.fset.Position(f.pos), Msg: f.msg, Gate: true}
		if f.end > f.pos {
			d.End = g.fset.Position(f.end)
		}
		return d
	}
	if len(g.replacements) > 0 {
		at := make(map[token.Position][]int)
		for i, r := range g.replacements {
			p := g.fset.Position(r.pos)
			at[p] = append(at[p], i)
		}
		used := make([]bool, len(g.replacements))
		g.pkg.Errors = g.pkg.Errors.Rewrite(func(d diag.Diagnostic) []diag.Diagnostic {
			for _, i := range at[d.Pos] {
				if r := g.replacements[i]; !used[i] && r.match(d.Msg) {
					used[i] = true
					return []diag.Diagnostic{toDiag(r.finding)}
				}
			}
			return []diag.Diagnostic{d}
		})
	}
	// One error at each place, then the first of each message in each
	// file (by position, then end).
	best := make(map[token.Pos]finding)
	for _, f := range g.found {
		if old, ok := best[f.pos]; !ok || f.better(old) {
			best[f.pos] = f
		}
	}
	first := make(map[gateKey]finding)
	for _, f := range best {
		k := gateKey{g.fset.File(f.pos).Name(), f.msg}
		if old, ok := first[k]; !ok || f.pos < old.pos || f.pos == old.pos && f.end < old.end {
			first[k] = f
		}
	}
	rest := make([]finding, 0, len(first))
	for _, f := range first {
		rest = append(rest, f)
	}
	slices.SortFunc(rest, func(a, b finding) int {
		return cmp.Or(cmp.Compare(a.pos, b.pos), cmp.Compare(a.end, b.end), strings.Compare(a.msg, b.msg))
	})
	for _, f := range rest {
		g.pkg.Errors.Add(toDiag(f))
	}
}

// hasUnsafeSize reports whether e has a call of unsafe.Sizeof, Alignof or Offsetof that check
// wrapped in gp_cgoh.UnsafeSize (unsafecalls.go): in a constant context it is Go's error "is not
// constant" (with the reason), not the gate of C macros.
func (g *gateChecker) hasUnsafeSize(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && len(call.Args) == 1 {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && unsafeHelperOf(sel, g.info) == "Size" {
				found = true
			}
		}
		return !found
	})
	return found
}
