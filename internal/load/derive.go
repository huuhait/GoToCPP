package load

import (
	"go/ast"
	"go/token"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/syntax/gpast"
)

// #Derive (Phase 10 step 10.10, README §2.2, ADR-080).
//
//	#Derive(String, Equal, Clone, JSON)
//	type Event struct { ... }
//
// is built in two steps, because the generators need type information that exists only after
// go/types (the constants of an enum may be in other files, and Equal, Clone and JSON need the
// kind of every field). The loader does the first step, before go/types sees anything:
//
//   - it adds the stubs, the methods with their final signatures and a body that only panics, so
//     that code which calls e.String() or a.Equal(b) type-checks, in this package and (through the
//     types of the package) in the packages that import it. The method names have the place of the
//     name in the #Derive line: hover and go to definition on a call land there.
//
// The generated bodies import nothing: the language server knows the imports of a package from the
// header of its files alone, so a #Derive line cannot add one. (An enum's String writes the number
// of an unknown value by hand, and Equal and Clone need no package.)
//
// internal/check (derive.go) does the second step: when go/types is done, it writes the bodies
// from the types of the fields and checks the package once more.

// A Derive is a line #Derive(...) on top of a type declaration.
type Derive struct {
	File *File
	Decl *ast.GenDecl  // the type declaration
	Spec *ast.TypeSpec // the type
	Pos  token.Pos     // the # of the line
	Gens []*DeriveGen  // the generators, in the order written
	Recv string        // the name of the receiver of the methods
}

// A DeriveGen is one name in the line: String, Equal, Clone or JSON.
type DeriveGen struct {
	Name  string          // the name as written
	Pos   token.Pos       // where it is written
	Funcs []*ast.FuncDecl // the stubs: the check replaces their bodies
}

// DeriveNames are the generators of #Derive, in the order of the documentation.
var DeriveNames = []string{"String", "Equal", "Clone", "JSON"}

// deriveMethods returns the names of the methods that the generator gen adds.
func deriveMethods(gen string) []string {
	return []string{gen}
}

func isDeriveName(s string) bool {
	for _, n := range DeriveNames {
		if n == s {
			return true
		}
	}
	return false
}

// deriveCall returns the generators written in a call of #Derive, with a sign that the call is
// one.
func deriveCall(call *ast.CallExpr) (ok bool) {
	id, isID := call.Fun.(*ast.Ident)
	return isID && gpast.MacroBase(id.Name) == "Derive"
}

// deriveType handles the lines of macros on top of the type declaration gd: it checks them,
// records the Derive, and returns the stubs to put after the declaration.
func (b *builtinPass) deriveType(gd *ast.GenDecl, calls []*ast.CallExpr) []ast.Decl {
	var stubs []ast.Decl
	var spec *ast.TypeSpec
	if len(gd.Specs) == 1 {
		spec, _ = gd.Specs[0].(*ast.TypeSpec)
	}
	for _, call := range calls {
		if call.Lparen == call.Rparen {
			continue // a line without an argument list: the parser reported it
		}
		if !deriveCall(call) {
			b.errorf(call.Pos(), "a macro on top of a type: only #Derive can be used there (user macros on types are not supported): %s", exprName(call.Fun))
			continue
		}
		if spec == nil {
			b.errorf(call.Pos(), "#Derive on a group of types: write it on top of one type declaration")
			continue
		}
		stubs = append(stubs, b.deriveCall(gd, spec, call)...)
	}
	return stubs
}

// deriveCall checks one line #Derive(...) on top of the type spec and returns its stubs.
func (b *builtinPass) deriveCall(gd *ast.GenDecl, spec *ast.TypeSpec, call *ast.CallExpr) []ast.Decl {
	if call.Ellipsis.IsValid() {
		b.errorf(call.Ellipsis, "#Derive: cannot use ... in the arguments")
	}
	if len(call.Args) == 0 {
		b.errorf(call.Pos(), "#Derive needs the names of the generators: #Derive(String, Equal, Clone, JSON)")
		return nil
	}
	if spec.Name.Name == "_" {
		b.errorf(call.Pos(), "#Derive on the blank type _: the methods need a name to belong to")
		return nil
	}
	d := &Derive{File: b.f, Decl: gd, Spec: spec, Pos: call.Pos()}
	if b.deriveSeen == nil {
		b.deriveSeen = map[*ast.TypeSpec]map[string]bool{}
	}
	seen := b.deriveSeen[spec] // a generator written on an earlier line of the same type counts too
	if seen == nil {
		seen = map[string]bool{}
		b.deriveSeen[spec] = seen
	}
	d.Recv = deriveRecv(spec, b.packageNames())
	bad := false
	for _, a := range call.Args {
		id, ok := a.(*ast.Ident)
		switch {
		case !ok:
			b.errorf(a.Pos(), "#Derive: %s is not the name of a generator (String, Equal, Clone or JSON)", exprName(a))
			bad = true
		case !isDeriveName(id.Name):
			b.errorf(a.Pos(), "#Derive: %s is not the name of a generator (String, Equal, Clone or JSON)", id.Name)
			bad = true
		case seen[id.Name]:
			b.errorf(a.Pos(), "#Derive(%s) is written twice", id.Name)
			bad = true
		default:
			seen[id.Name] = true
			d.Gens = append(d.Gens, &DeriveGen{Name: id.Name, Pos: id.Pos()})
		}
	}
	if bad {
		return nil
	}
	if spec.Assign.IsValid() {
		b.errorf(call.Pos(), "#Derive on %s, which is an alias: write it on top of the type that %s stands for", spec.Name.Name, spec.Name.Name)
		return nil
	}
	var stubs []ast.Decl
	for _, g := range d.Gens {
		if !b.deriveKind(g, spec) {
			continue
		}
		if g.Name == "JSON" {
			b.errorf(g.Pos, "#Derive(JSON) is not available yet: it needs encoding/json, which is a declaration stub until step 9.11")
			continue
		}
		clash := false
		for _, m := range deriveMethods(g.Name) {
			if at, ok := b.methodOf(spec.Name.Name, m); ok {
				b.errorf(g.Pos, "#Derive(%s): the type %s has a method %s already (declared at %s): remove one of them", g.Name, spec.Name.Name, m, at)
				clash = true
			}
		}
		if clash {
			continue
		}
		for _, m := range deriveMethods(g.Name) {
			fd := b.stub(d, g, m)
			g.Funcs = append(g.Funcs, fd)
			stubs = append(stubs, fd)
		}
	}
	if b.nerr == 0 || len(stubs) > 0 {
		b.p.Derives = append(b.p.Derives, d)
	}
	return stubs
}

// deriveKind reports (and returns false) when the generator g cannot be used for a type declared
// as spec, from the syntax of the declaration alone: the check says more once the types are known.
func (b *builtinPass) deriveKind(g *DeriveGen, spec *ast.TypeSpec) bool {
	name := spec.Name.Name
	switch spec.Type.(type) {
	case *ast.StructType:
		if g.Name == "String" {
			b.errorf(g.Pos, "#Derive(String): %s is a struct: fmt prints a struct by itself (with the plus flag it shows the field names), and String is for integer types with constants", name)
			return false
		}
		return true
	case *ast.InterfaceType:
		b.errorf(g.Pos, "#Derive(%s): %s is an interface type: the generators are for structs and for named array, slice, map and integer types", g.Name, name)
	case *ast.FuncType:
		b.errorf(g.Pos, "#Derive(%s): %s is a func type: the generators are for structs and for named array, slice, map and integer types", g.Name, name)
	case *ast.ChanType:
		b.errorf(g.Pos, "#Derive(%s): %s is a channel type: the generators are for structs and for named array, slice, map and integer types", g.Name, name)
	case *ast.StarExpr:
		b.errorf(g.Pos, "#Derive(%s): %s is a pointer type, which cannot have methods", g.Name, name)
	case *ast.ArrayType, *ast.MapType:
		switch g.Name {
		case "String":
			b.errorf(g.Pos, "#Derive(String): %s is not an integer type with constants", name)
		case "JSON":
			b.errorf(g.Pos, "#Derive(JSON): %s is not a struct type (encoding/json handles slices, arrays and maps by itself)", name)
		default:
			return true
		}
	default:
		return true // a name: the check knows its kind
	}
	return false
}

// methodOf returns where the package declares the method m on the type typ (in any of its
// files), as file:line, or ok=false.
func (b *builtinPass) methodOf(typ, m string) (at string, ok bool) {
	if b.methods == nil {
		b.methods = map[string]string{}
		for _, f := range b.p.Files {
			for _, d := range f.AST.Decls {
				fd, isFn := d.(*ast.FuncDecl)
				if !isFn || fd.Recv == nil || len(fd.Recv.List) != 1 || fd.Name == nil {
					continue
				}
				if base := recvBase(fd.Recv.List[0].Type); base != "" {
					if _, dup := b.methods[base+"."+fd.Name.Name]; !dup {
						pos := b.l.fset.Position(fd.Name.Pos())
						b.methods[base+"."+fd.Name.Name] = b.fileNameOf(pos.Filename) + ":" + strconv.Itoa(pos.Line)
					}
				}
			}
		}
	}
	at, ok = b.methods[typ+"."+m]
	return at, ok
}

// recvBase returns the name of the type of a receiver: T for T, *T, T[K] and *T[K].
func recvBase(e ast.Expr) string {
	e = ast.Unparen(e)
	if s, ok := e.(*ast.StarExpr); ok {
		e = ast.Unparen(s.X)
	}
	switch x := e.(type) {
	case *ast.IndexExpr:
		e = x.X
	case *ast.IndexListExpr:
		e = x.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// packageNames returns the package-level names of the package.
func (b *builtinPass) packageNames() map[string]token.Pos {
	if b.pkgNames == nil {
		asts := make([]*ast.File, len(b.p.Files))
		for i, f := range b.p.Files {
			asts[i] = f.AST
		}
		b.pkgNames = PackageLevelNames(asts)
	}
	return b.pkgNames
}

// deriveRecv returns the name of the receiver of the derived methods of spec: the lower-case
// first letter of the type's name, as gocpp doc shows it, unless that name is used in the
// declaration of the type, by a type parameter, or at the package level (the generated bodies
// write the types of the fields, which the receiver would hide).
func deriveRecv(spec *ast.TypeSpec, pkgNames map[string]token.Pos) string {
	used := map[string]bool{}
	ast.Inspect(spec.Type, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			used[id.Name] = true
		}
		return true
	})
	if spec.TypeParams != nil {
		for _, f := range spec.TypeParams.List {
			for _, n := range f.Names {
				used[n.Name] = true
			}
			ast.Inspect(f.Type, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					used[id.Name] = true
				}
				return true
			})
		}
	}
	r, _ := utf8.DecodeRuneInString(spec.Name.Name)
	cands := []string{string(unicode.ToLower(r)), "recv", "self", "this"}
	if !unicode.IsLetter(r) {
		cands = cands[1:]
	}
	for _, c := range cands {
		if _, pkg := pkgNames[c]; !used[c] && !pkg && c != "other" && c != "data" {
			return c
		}
	}
	return "gp_recv"
}

// stub returns the method m of the generator g of d: its final signature and a body that only
// panics. Everything has the place of the name of the generator in the #Derive line.
func (b *builtinPass) stub(d *Derive, g *DeriveGen, m string) *ast.FuncDecl {
	pos := g.Pos
	id := func(name string) *ast.Ident { return b.synth(&ast.Ident{NamePos: pos, Name: name}) }
	typ := func() ast.Expr { // T, T[K, V]
		name := id(d.Spec.Name.Name)
		if d.Spec.TypeParams == nil || len(d.Spec.TypeParams.List) == 0 {
			return name
		}
		var args []ast.Expr
		for _, f := range d.Spec.TypeParams.List {
			for _, n := range f.Names {
				args = append(args, id(n.Name))
			}
		}
		if len(args) == 1 {
			return &ast.IndexExpr{X: name, Lbrack: pos, Index: args[0], Rbrack: pos}
		}
		return &ast.IndexListExpr{X: name, Lbrack: pos, Indices: args, Rbrack: pos}
	}
	field := func(name string, t ast.Expr) *ast.Field {
		f := &ast.Field{Type: t}
		if name != "" {
			f.Names = []*ast.Ident{id(name)}
		}
		return f
	}
	list := func(fs ...*ast.Field) *ast.FieldList {
		return &ast.FieldList{Opening: pos, List: fs, Closing: pos}
	}
	recvType := typ()
	var params, results *ast.FieldList
	switch m {
	case "String":
		results = list(field("", id("string")))
	case "Equal":
		params = list(field("other", typ()))
		results = list(field("", id("bool")))
	case "Clone":
		results = list(field("", typ()))
	}
	if params == nil {
		params = list()
	}
	panicCall := &ast.CallExpr{Fun: id("panic"), Lparen: pos, Rparen: pos,
		Args: []ast.Expr{&ast.BasicLit{ValuePos: pos, Kind: token.STRING, Value: `"gp_derive"`}}}
	return &ast.FuncDecl{
		Recv: list(field(d.Recv, recvType)),
		Name: &ast.Ident{NamePos: pos, Name: m},
		Type: &ast.FuncType{Func: pos, Params: params, Results: results},
		Body: &ast.BlockStmt{Lbrace: pos, List: []ast.Stmt{&ast.ExprStmt{X: panicCall}}, Rbrace: pos},
	}
}
