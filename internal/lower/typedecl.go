package lower

import (
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Type declarations: the C++ structs of the package's struct types (Phase 03 README §2.4, step
// 3.8; structs.go has the expressions):
//
//	type Point struct{ X, Y int }       struct Point {
//	                                        int64_t X = 0;
//	                                        int64_t Y = 0;
//	                                        bool operator==(const Point&) const = default;
//	                                        std::size_t gp_hash() const {
//	                                            return gp::hash_fields(X, Y);
//	                                        }
//	                                        static constexpr const char* gp_type_name = "main.Point";
//	                                        static constexpr auto gp_fields = gp::fields(
//	                                            gp::field(&Point::X, "X", true, false), ...);
//	                                    };
//	                                    static_assert(sizeof(Point) == 16 && alignof(Point) == 8);
//
//   - The fields are data members in source order, at one access level, with their zero values
//     as default member initializers (= 0, = false; a class type's default constructor is its
//     zero value), so T{} and T x{}; are the zero value. An embedded field is a data member
//     named after its type (never a base class: Phase 05 §2.6), and a blank field _ is
//     gp_blank0, gp_blank1, ...: C++ needs a name. The layout is then what gocppSizes computes
//     (internal/check/sizes.go), and a static_assert after each struct checks it.
//   - operator== only for a comparable struct: defaulted, or, with blank fields (which Go's ==
//     skips), the non-blank fields compared in order with &&. gp_hash() goes with it, for map
//     keys (gocpp/hash.hpp's protocol), over the same fields.
//   - gp_type_name and gp_fields are the fmt shim's protocol (gocpp/fmtshim.hpp): the Go name
//     of the type as %T prints it, and every field, also blank and embedded ones, with its Go
//     name, whether it is exported and whether it is embedded, and the type tag of a field whose
//     type has int64, uint64 or uintptr in it (C++ can't tell them from int and uint).
//   - A field's C++ name is its Go name, escaped like a method's (MethodName: a field named like
//     its struct gets a _). A type that a field's name hides inside the struct (Point Point, Go's
//     embedded Point) is written qualified, gpkg_main::Point, as anywhere a variable of the same
//     name is in scope (hidden): GCC rejects a member that changes the meaning of a type name
//     used before it in the class, and both compilers read the name as the member after it.
//
// Which types become structs:
//   - the named struct types of the package, in source order. A type declared in a function is
//     moved to the namespace (C++'s local classes can't have static data members): it keeps its
//     Go name if no package-level name has it, or gets the first free name of T_1, T_2, ...;
//     gp_type_name still says main.T, as Go's %T does;
//   - the anonymous struct types: Go's type identity is structural, so identical struct types
//     (the same fields, types, tags and embedding) are one C++ type, named gp_anon_ and 16 hex
//     digits of the SHA-256 of the type's Go spelling (with package paths). Phase 06 moves them
//     into a header that all the packages share; until then each translation unit has its own.
//
// The structs have no place in the .gp source (the zero Pos), on purpose: no #line maps their
// lines to the .gp file, so in the debug info the code of the default member initializers,
// operator== and gp_hash() points at the generated .cpp file, like the other generated code.
//
// Order: every struct is declared first (struct T;), then defined in dependency order: a field
// of a struct type by value, or of an array of one, needs that struct defined before; a pointer,
// slice or map of it doesn't (the runtime's templates accept an incomplete type), so recursive
// types (type Node struct{ next *Node; kids []Node }) work. A cycle by value is a go/types error.
// The structs come before the prototypes and the package variables.
//
// The named types that aren't structs (type Celsius float64, type IDs []int, step 3.9) are
// structs too, over gp::newtype (named.go: newtypeDef), named, ordered and declared with the
// others. Their methods (step 3.10) are declared in them (methods.go). Not yet: generic types
// (Phase 07).

// A structInfo is one C++ struct: of a named struct type, of an anonymous struct type, or of a
// named type that isn't a struct (a gp::newtype: named.go).
type structInfo struct {
	t      types.Type    // the *types.Named, or the *types.Struct of an anonymous type
	st     *types.Struct // its fields (nil for a gp::newtype)
	under  types.Type    // the underlying type of a gp::newtype, or nil
	name   string        // the C++ name
	goName string        // Go's name of the type, as %T prints it: main.Point, struct { X int }
	fields []string      // the C++ names of the fields

	// owner is the lowering of the package that declares the struct. shared says that the
	// split layout writes it in the global namespace, in the header of every package that
	// uses it (an anonymous struct type: shared.go).
	owner  *gen
	shared *sharedDef
}

// typeDecls holds the structs of the package.
type typeDecls struct {
	list  []*structInfo                   // in source order, then the anonymous ones as they are found
	named map[*types.TypeName]*structInfo // the named struct types
	anon  map[string]*structInfo          // the anonymous struct types, by their Go spelling
	used  map[string]bool                 // the C++ names at namespace scope: functions, variables, types
	local map[*types.TypeName]string      // the C++ names of the types declared in functions (anonKey)

	// hidden are the C++ names that hide a type of the same name where C++ is written now: the
	// variables and functions of the function that is being lowered (its funcNames.taken), or
	// the fields of the struct that is being defined (hide).
	hidden map[string]bool
	// members are the C++ names of the members of the struct whose method is being lowered
	// (methods.go: hideMembers): in a member function they hide the package's names too.
	members map[string]bool
}

// tdecls returns the package's structs.
func (g *gen) tdecls() *typeDecls {
	if g.typeDecls == nil {
		g.typeDecls = &typeDecls{
			named: make(map[*types.TypeName]*structInfo),
			anon:  make(map[string]*structInfo),
			used:  make(map[string]bool),
			local: make(map[*types.TypeName]string),
		}
	}
	return g.typeDecls
}

// hide makes the names hidden (hidden above) until the function it returns is called.
func (g *gen) hide(names map[string]bool) func() {
	td := g.tdecls()
	old := td.hidden
	td.hidden = names
	return func() { td.hidden = old }
}

// collectTypes names the types of a package-level type declaration.
func (g *gen) collectTypes(d *ast.GenDecl) {
	for _, spec := range d.Specs {
		g.typeSpec(spec.(*ast.TypeSpec), false)
	}
}

// collectLocalTypes names the types that the functions declare, after every package-level name
// is known (collect): a local type keeps its Go name unless a package-level name has it.
func (g *gen) collectLocalTypes() {
	td := g.tdecls()
	for _, n := range g.names {
		td.used[n] = true
	}
	for _, si := range td.list {
		td.used[si.name] = true
	}
	for _, f := range g.funcs {
		ast.Inspect(f.decl.Body, func(n ast.Node) bool {
			if d, ok := n.(*ast.GenDecl); ok && d.Tok == token.TYPE {
				for _, spec := range d.Specs {
					g.typeSpec(spec.(*ast.TypeSpec), true)
				}
			}
			return true
		})
	}
}

// typeSpec names the type that spec declares (local: in a function) and adds its struct.
func (g *gen) typeSpec(spec *ast.TypeSpec, local bool) {
	switch {
	case spec.Name.Name == "_":
		return // nothing can use it
	case spec.TypeParams != nil:
		g.unsupported(spec.Pos(), "generic types (Phase 07)")
		return
	case spec.Assign.IsValid():
		return // an alias: its uses are the type it names (cxxType unaliases them)
	}
	tn, _ := g.info.Defs[spec.Name].(*types.TypeName)
	if tn == nil {
		g.errorf(spec.Pos(), "internal error: no object for type %s", spec.Name.Name)
		return
	}
	named, _ := tn.Type().(*types.Named)
	if named == nil {
		return
	}
	if !complete(named) {
		g.unsupported(spec.Pos(), "named types declared over C/C++ types ("+tn.Name()+", a later step of Phase 04)")
		return
	}
	if !isInterface(named) && !isStruct(named) && !g.newtypeOK(spec, named) { // named.go
		return
	}
	if weakStruct(named) {
		// type WP weak.Pointer[T]: a struct for Go, but gp::weak_ptr is a runtime class (weak.go).
		g.unsupported(spec.Pos(), "a type defined as weak.Pointer[T] (use weak.Pointer[T] itself, or an alias)")
		return
	}
	td := g.tdecls()
	name := g.typeName(tn.Name())
	if local {
		base := name
		for i := 1; td.used[name] || reserved[name] || g.isMacro(name); i++ {
			name = base + "_" + strconv.Itoa(i)
			if strings.HasPrefix(name, "gp_") {
				name += "_" // not like a generated name: gp_x_1_ (funcNames.newName)
			}
		}
	}
	td.used[name] = true
	if local {
		td.local[tn] = name // before addStruct: a field may refer to the type itself
	}
	if isInterface(named) {
		g.addIface(named, name) // ifaces.go
		return
	}
	var si *structInfo
	if isStruct(named) {
		si = g.addStruct(named, name, tn.Pkg().Name()+"."+tn.Name())
	} else {
		si = g.addNewtype(named, name, tn.Pkg().Name()+"."+tn.Name()) // named.go
	}
	td.named[tn] = si
}

// addStruct adds the struct of the type t, which the C++ calls name, and the anonymous structs
// that its fields use.
func (g *gen) addStruct(t types.Type, name, goName string) *structInfo {
	st := t.Underlying().(*types.Struct)
	si := &structInfo{t: t, st: st, name: name, goName: goName, fields: make([]string, st.NumFields()), owner: g}
	blanks := 0
	for i := range st.NumFields() {
		f := st.Field(i)
		if f.Name() == "_" {
			si.fields[i] = "gp_blank" + strconv.Itoa(blanks)
			blanks++
		} else {
			si.fields[i] = g.methodName(name, f.Name())
		}
	}
	td := g.tdecls()
	td.list = append(td.list, si)
	for i := range st.NumFields() {
		g.anonStructsIn(st.Field(i).Type())
	}
	return si
}

// anonStructsIn adds the structs of the anonymous struct types in t: t itself, or the types
// that it is made of (an element, a pointer's target, a field, ...).
func (g *gen) anonStructsIn(t types.Type) {
	switch t := types.Unalias(t).(type) {
	case *types.Struct:
		g.anonStruct(t)
	case *types.Pointer:
		g.anonStructsIn(t.Elem())
	case *types.Slice:
		g.anonStructsIn(t.Elem())
	case *types.Array:
		g.anonStructsIn(t.Elem())
	case *types.Map:
		g.anonStructsIn(t.Key())
		g.anonStructsIn(t.Elem())
	case *types.Chan:
		g.anonStructsIn(t.Elem())
	case *types.Tuple:
		for i := range t.Len() {
			g.anonStructsIn(t.At(i).Type())
		}
	case *types.Signature:
		g.anonStructsIn(t.Params())
		g.anonStructsIn(t.Results())
	}
}

// anonStruct returns the struct of the anonymous struct type st, which it adds the first time.
// In the split layout the struct is one for the whole program: the first package that uses
// it makes it, the others use the same structInfo (shared.go).
func (g *gen) anonStruct(st *types.Struct) *structInfo {
	td := g.tdecls()
	key := g.anonKey(st)
	if si := td.anon[key]; si != nil {
		return si
	}
	sum := sha256.Sum256([]byte(key))
	name := "gp_anon_" + hex.EncodeToString(sum[:8])
	if g.split {
		if si := g.prog.anonStructs[key]; si != nil {
			// Another package made it: this one writes it too, with the structs it holds.
			td.anon[key] = si
			td.list = append(td.list, si)
			g.noteShared(si)
			for i := range st.NumFields() {
				g.anonStructsIn(st.Field(i).Type())
			}
			return si
		}
	}
	si := g.addStruct(st, name, goTypeName(st))
	td.anon[key] = si
	if g.split {
		g.prog.anonStructs[key] = si
		si.shared = &sharedDef{}
		g.prog.sharedOrder = append(g.prog.sharedOrder, sharedItem{st: si})
		g.noteShared(si)
	}
	return si
}

// anonKey returns the spelling of the anonymous struct type st that identifies it (anonStruct):
// Go's spelling with package paths and tags, where a type declared in a function is its name, its
// package and its C++ name (T@T_1), since two functions may declare different types of the same
// name. The key has no positions, so a line added above a type doesn't rename the struct.
// The package of an unexported field name is in the key too: struct{ x int } of two packages are
// two Go types (canon.go does the same for the identity text).
func (g *gen) anonKey(st *types.Struct) string {
	return typeStringPkg(st, func(n *types.Named) string {
		obj := n.Obj()
		if obj.Pkg() == nil {
			return obj.Name()
		}
		s := obj.Pkg().Path() + "." + obj.Name()
		if obj.Parent() != obj.Pkg().Scope() {
			// A type declared in a function: its C++ name at namespace scope (T, T_1), which
			// depends only on the order of the declarations, not on their lines.
			s += "@" + g.tdecls().local[obj]
		}
		return s
	})
}

// structOf returns the struct of the struct type t (named or anonymous, through aliases), or of
// the named type t that isn't a struct (a gp::newtype), or nil (with an error at pos) if it has
// none.
func (g *gen) structOf(t types.Type, pos token.Pos) *structInfo {
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		if si := g.tdecls().named[t.Obj()]; si != nil {
			return si
		}
		if o := g.foreign(t.Obj().Pkg()); o != nil {
			if si := o.tdecls().named[t.Obj()]; si != nil {
				return si // the struct of a type of another package: it is written qualified (structType)
			}
		}
	case *types.Struct:
		return g.anonStruct(t)
	}
	g.unsupported(pos, "type "+types.TypeString(t, types.RelativeTo(g.pkg.Types)))
	return nil
}

// structType returns the C++ spelling of the struct type t (or of a named type that isn't a
// struct): its name, or the name qualified by the namespace where a variable, a function or a
// field of that name hides it (hidden).
func (g *gen) structType(t types.Type, pos token.Pos) cppir.Type {
	si := g.structOf(t, pos)
	if si == nil {
		return typeName("gp_unsupported")
	}
	if si.shared != nil && g.split {
		return typeName(si.name) // in the global namespace (shared.go)
	}
	if si.owner != g {
		return typeName(si.owner.qual(si.name)) // a type of another package
	}
	if g.inTemplate > 0 || g.global > 0 {
		return typeName("::" + g.ns + "::" + si.name) // a C++ template's argument (ctypes.go), or a type in the global namespace
	}
	if td := g.tdecls(); td.hidden[si.name] || td.members[si.name] {
		return typeName(g.ns + "::" + si.name)
	}
	return typeName(si.name)
}

// fieldName returns the C++ name of the field i of the struct type t.
func (g *gen) fieldName(t types.Type, i int, pos token.Pos) string {
	si := g.structOf(t, pos)
	if si == nil {
		return placeholder.Name
	}
	return si.fields[i]
}

// isStruct reports whether t is a struct type (through names and aliases), other than
// weak.Pointer[T], whose Go declaration is a struct.
func isStruct(t types.Type) bool {
	_, ok := under(t).(*types.Struct)
	return ok && !isWeakPointer(t) // weak.go: a runtime type of its own
}

// A typeParts is what structDecls made, by what it is. The unit layout puts it in one
// translation unit in this order (all); the split layout puts the parts in different files
// (split.go).
type typeParts struct {
	structs []*structInfo // the structs that have a forward declaration, in order
	ifaces  []cppir.Decl  // the interface structs: forward declarations, then definitions
	defs    []structDefn  // the structs' definitions in dependency order
	post    []postDefn    // the definitions of the forwarders and the fmt hooks of each struct
	// globalFwd are the forwarders of the shared structs of the package (shared.go), which are in
	// the global namespace: they are defined there, in pkg_desc.cpp.
	globalFwd []cppir.Decl
	impl      implParts // the interfaces' and the descriptors' definitions
}

// A postDefn is the out-of-line definitions of one struct's members: the forwarders of the
// promoted methods (promote.go) and the fmt shim's hooks (methods.go).
type postDefn struct {
	fwd, hooks []cppir.Decl
}

// A structDefn is the definition of one struct, with its static_assert.
type structDefn struct {
	si    *structInfo
	decls []cppir.Decl
}

// all returns the declarations of the unit layout: the structs' forward declarations, the
// interface structs, the definitions, the forwarders and hooks, then the descriptors.
func (p *typeParts) all() []cppir.Decl {
	if p == nil {
		return nil
	}
	var decls []cppir.Decl
	for _, si := range p.structs {
		decls = append(decls, &cppir.Struct{Name: si.name, Forward: true})
	}
	decls = append(decls, &cppir.Blank{})
	decls = append(decls, p.ifaces...) // no data members: before the structs that hold them
	for _, d := range p.defs {
		decls = append(decls, d.decls...)
	}
	for _, d := range p.post {
		decls = append(decls, d.fwd...)
		decls = append(decls, d.hooks...)
	}
	return append(decls, p.impl.all...)
}

// structDecls returns the declarations of all the structs (see the top): the forward
// declarations, then the definitions in dependency order, each with its static_assert. It is
// nil if the package has no struct, interface or descriptor.
func (g *gen) structDecls() *typeParts {
	td := g.tdecls()
	// The interface structs, the descriptors, the thunks and the wrappers (ifaces.go,
	// typedesc.go) are made until nothing new is found: they find more types (an unnamed
	// interface in a method's signature, a descriptor of a part).
	var ifaceDefs []cppir.Decl
	var impls implParts
	for {
		n := g.typeCount()
		for i := 0; i < len(td.list); i++ {
			g.structDef(td.list[i]) // (its field types may be unnamed interfaces)
		}
		ifaceDefs = g.ifaceDefs()
		impls = g.implDecls()
		if g.typeCount() == n {
			break
		}
	}
	if len(td.list) == 0 && len(ifaceDefs) == 0 && impls.empty() {
		return nil
	}
	parts := &typeParts{structs: slices.Clone(td.list), ifaces: ifaceDefs, impl: impls}
	done := make(map[*structInfo]bool)
	var define func(si *structInfo)
	define = func(si *structInfo) {
		if done[si] {
			return
		}
		done[si] = true // a cycle by value is a go/types error
		if si.under != nil {
			// A gp::newtype is its underlying type: a gp::array<T, N> base needs T.
			for _, dep := range g.byValue(si.under) {
				define(dep)
			}
		}
		for i := range si.fieldCount() {
			for _, dep := range g.byValue(si.st.Field(i).Type()) {
				define(dep)
			}
			for _, dep := range g.cTypeDeps(si.st.Field(i)) {
				define(dep)
			}
		}
		decls := append(g.structDef(si), &cppir.Blank{})
		parts.defs = append(parts.defs, structDefn{si, decls})
	}
	for i := 0; i < len(td.list); i++ { // (structDef may add none: anonStructsIn found them all)
		define(td.list[i])
	}
	// The forwarders of the promoted methods (promote.go) and the fmt shim's hooks of the method
	// sets (methods.go), after all the structs.
	for _, si := range td.list {
		if si.shared != nil && g.split {
			// A shared struct is defined once for the program: its hooks are in its guarded
			// definition (makeShared), its forwarders in the pkg_desc.cpp of its owner.
			if si.owner == g {
				// (Outside the package's namespace, where the struct is: its names are qualified.)
				g.global++
				parts.globalFwd = append(parts.globalFwd, g.forwardDefs(si)...)
				g.global--
			}
			continue
		}
		parts.post = append(parts.post, postDefn{g.forwardDefs(si), g.hookDefs(si)})
	}
	return parts
}

// typeCount returns how many structs, interfaces and descriptors there are, to see whether
// writing them found more (structDecls).
func (g *gen) typeCount() int {
	n := len(g.tdecls().list) + len(g.dtable().named) + len(g.dtable().comps)
	return n + len(g.itable().list)
}

// cTypeDeps returns the structs of the package in the C/C++ type of the field f
// (std.array[Point, 4], std.vector[Node]), which a C++ template may hold by value: they are
// defined before f's struct (a cycle is cut where it closes, as for byValue).
func (g *gen) cTypeDeps(f *types.Var) []*structInfo {
	if complete(f.Type()) {
		return nil
	}
	e := g.pkg.Opaque.TypeExpr(f)
	if e == nil {
		return nil
	}
	var deps []*structInfo
	ast.Inspect(e, func(n ast.Node) bool {
		if x, ok := n.(ast.Expr); ok {
			if tv, ok := g.info.Types[x]; ok && tv.IsType() && complete(tv.Type) {
				deps = append(deps, g.byValue(tv.Type)...)
				return false
			}
		}
		return true
	})
	return deps
}

// fieldCount returns the number of fields of the struct si (0 for a gp::newtype).
func (si *structInfo) fieldCount() int {
	if si.st == nil {
		return 0
	}
	return si.st.NumFields()
}

// byValue returns the structs that a field of type t holds by value, which must be defined
// before the struct of the field: t itself (also a named type that isn't a struct, a
// gp::newtype, which is its base class), or the element of an array (of arrays).
func (g *gen) byValue(t types.Type) []*structInfo {
	switch u := types.Unalias(t).(type) {
	case *types.Named:
		if si := g.tdecls().named[u.Obj()]; si != nil {
			return []*structInfo{si}
		}
		// (A type of another package is defined in its header, included before this one.)
	case *types.Struct:
		return []*structInfo{g.anonStruct(u)}
	case *types.Array:
		return g.byValue(u.Elem())
	}
	return nil
}

// structDef returns the definition of the struct si and its static_assert.
func (g *gen) structDef(si *structInfo) []cppir.Decl {
	if si.under != nil {
		return g.newtypeDef(si) // named.go
	}
	hidden := make(map[string]bool)
	for _, f := range si.fields {
		hidden[f] = true
	}
	for _, m := range g.methodNames(si) { // methods.go
		hidden[m] = true
	}
	for _, m := range g.forwarderNames(si) { // promote.go
		hidden[m] = true
	}
	defer g.hide(hidden)()
	s := &cppir.Struct{Name: si.name}
	var compared []string // the non-blank fields
	for i := range si.st.NumFields() {
		f := si.st.Field(i)
		init := zeroInit(f.Type())
		if !complete(f.Type()) {
			init = &cppir.InitList{} // a C/C++ type (ctypes.go): value-initialized, T f{};
		}
		d := varDecl(cppir.Pos{}, g.varType(f, f.Pos()), si.fields[i], init)
		if !complete(f.Type()) && cObject(f.Type()) && g.maybeNoDefault(f) {
			// Go's zero value of the struct needs one of the field: a C++ type with no default
			// constructor is a clear error here (the default member initializer, T f{}, would be
			// an error deep in the struct's own code).
			s.Members = append(s.Members, &cppir.StaticAssert{Pos: g.pos(f.Pos()),
				Cond: &cppir.Name{Name: "gp::cgo::field_zero", Args: []cppir.Node{g.varType(f, f.Pos())}}})
		}
		s.Members = append(s.Members, d)
		if f.Name() != "_" {
			compared = append(compared, si.fields[i])
		}
	}
	cpart := hasCFields(si.st) // a field of a C/C++ type: == and the layout are the C++ compiler's
	if types.Comparable(si.t) && !cpart {
		self := &cppir.Ref{Elem: &cppir.Const{Elem: typeName(si.name)}}
		eq := &cppir.Func{Result: typeName("bool"), Name: "operator==", Params: []cppir.Param{{Type: self}}, Quals: []string{"const", "= default"}}
		if len(compared) < si.st.NumFields() {
			// Go's == skips the blank fields.
			var x cppir.Expr = lit("true")
			for i, f := range compared {
				c := &cppir.Binary{Op: "==", X: cxxName(f), Y: &cppir.Member{X: cxxName("gp_o"), Name: f}}
				if i == 0 {
					x = c
				} else {
					x = &cppir.Binary{Op: "&&", X: x, Y: c}
				}
			}
			eq.Params[0].Name = "gp_o"
			eq.Quals = []string{"const"}
			eq.Body = &cppir.Block{Stmts: []cppir.Stmt{&cppir.Return{X: x}}}
		}
		args := make([]cppir.Expr, len(compared))
		for i, f := range compared {
			args[i] = cxxName(f)
		}
		hash := &cppir.Func{Result: typeName("std::size_t"), Name: "gp_hash", Quals: []string{"const"},
			Body: &cppir.Block{Stmts: []cppir.Stmt{&cppir.Return{X: call("gp::hash_fields", args...)}}}}
		s.Members = append(s.Members, eq, hash)
		if hashMayPanic(si.t) {
			// A field is, or holds, an interface: the hash can panic (unhashable dynamic type), and
			// a read or delete in an empty map must check the key without hashing (map.hpp).
			s.Members = append(s.Members, &cppir.VarDecl{Specs: []string{"static", "constexpr"}, Type: typeName("bool"),
				Vars: []cppir.Declarator{{Name: "gp_hash_panics", Init: lit("true")}}})
		}
	}
	cstr := &cppir.Ptr{Elem: &cppir.Const{Elem: typeName("char")}}
	name := &cppir.VarDecl{Specs: []string{"static", "constexpr"}, Type: cstr,
		Vars: []cppir.Declarator{{Name: "gp_type_name", Init: lit(cppir.CString(si.goName))}}}
	fields := make([]cppir.Expr, si.st.NumFields())
	for i := range si.st.NumFields() {
		f := si.st.Field(i)
		args := []cppir.Expr{
			&cppir.Unary{Op: "&", X: cxxName(si.name + "::" + si.fields[i])},
			lit(cppir.CString(f.Name())),
			lit(strconv.FormatBool(f.Exported())),
			lit(strconv.FormatBool(f.Embedded())),
		}
		if tag := fmtTag(f.Type()); tag != "" {
			fields[i] = templateCall("gp::field", []cppir.Node{typeName(tag)}, args...)
		} else {
			fields[i] = call("gp::field", args...)
		}
	}
	info := &cppir.VarDecl{Specs: []string{"static", "constexpr"}, Type: typeName("auto"),
		Vars: []cppir.Declarator{{Name: "gp_fields", Init: call("gp::fields", fields...)}}}
	s.Members = append(s.Members, name, info)
	s.Members = append(s.Members, g.methodDecls(si)...) // methods.go
	if cpart {
		// gocpp doesn't know the size of a C/C++ type (check.Sizes guesses a word), and Go's ==
		// on such a struct compares the fields of a C struct, which have no == in C++: typed
		// interop (Phase 12) writes it.
		return []cppir.Decl{s}
	}
	return []cppir.Decl{s, g.layoutAssert(si)} // named.go
}

// hasCFields reports whether the struct st has a field of a C/C++ type by value (also in an array
// or a struct field), whose size and == only the C++ compiler knows.
func hasCFields(st *types.Struct) bool {
	for i := range st.NumFields() {
		t := st.Field(i).Type()
		for {
			a, ok := under(t).(*types.Array)
			if !ok {
				break
			}
			t = a.Elem()
		}
		if !complete(t) {
			return true
		}
		if s, ok := under(t).(*types.Struct); ok && hasCFields(s) {
			return true
		}
	}
	return false
}

// zeroInit returns the default member initializer of a field of type t, its zero value: 0 or
// false for the basic types other than strings and complex numbers, or nil for a class type
// (also gp::complex128), whose default constructor makes its zero value.
func zeroInit(t types.Type) cppir.Expr {
	b := basicOf(t)
	switch {
	case b == nil || b.Info()&(types.IsString|types.IsComplex) != 0:
		return nil
	case b.Info()&types.IsBoolean != 0:
		return lit("false")
	}
	return lit("0")
}

// fmtTag returns the fmt shim's type tag of a field of type t (gocpp/fmtshim.hpp), or "" if it
// needs none: t has no int64, uint64 or uintptr in a part that is not a struct of its own (a
// struct's own fields have their tags).
func fmtTag(t types.Type) string {
	const tag = "gp::fmtshim::tag::"
	elem := func(e types.Type) string {
		if s := fmtTag(e); s != "" {
			return s
		}
		return tag + "none"
	}
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		if isWeakPointer(t) && fmtTag(weakElem(t)) != "" {
			return tag + "weak<" + elem(weakElem(t)) + ">" // weak.Pointer[int64] (weak.go)
		}
	case *types.Basic:
		switch t.Kind() {
		case types.Int64:
			return tag + "int64"
		case types.Uint64:
			return tag + "uint64"
		case types.Uintptr:
			return tag + "uintptr"
		}
	case *types.Pointer:
		if fmtTag(t.Elem()) != "" {
			return tag + "ptr<" + elem(t.Elem()) + ">"
		}
	case *types.Slice:
		if fmtTag(t.Elem()) != "" {
			return tag + "slice<" + elem(t.Elem()) + ">"
		}
	case *types.Array:
		if fmtTag(t.Elem()) != "" {
			return tag + "array<" + elem(t.Elem()) + ">"
		}
	case *types.Map:
		if fmtTag(t.Key()) != "" || fmtTag(t.Elem()) != "" {
			return tag + "map<" + elem(t.Key()) + ", " + elem(t.Elem()) + ">"
		}
	}
	return ""
}

// goTypeName returns Go's name of the type t as reflect's String gives it, which %T prints:
// main.Point, []int, struct { X int; Y string "json:\"y\"" }, func(int) string.
func goTypeName(t types.Type) string {
	return typeString(t, func(n *types.Named) string {
		if n.Obj().Pkg() == nil {
			return n.Obj().Name() // error
		}
		return n.Obj().Pkg().Name() + "." + n.Obj().Name()
	})
}

// typeString writes t as reflect does (goTypeName), with named for the named types.
func typeString(t types.Type, named func(*types.Named) string) string {
	return typeStringOpt(t, named, false)
}

// typeStringPkg is typeString with the import path in front of the name of an unexported field,
// which tells the types of two packages apart: struct { a/b.x int }.
func typeStringPkg(t types.Type, named func(*types.Named) string) string {
	return typeStringOpt(t, named, true)
}

func typeStringOpt(t types.Type, named func(*types.Named) string, fieldPkg bool) string {
	str := func(t types.Type) string { return typeStringOpt(t, named, fieldPkg) }
	switch t := types.Unalias(t).(type) {
	case *types.Basic:
		return types.Typ[t.Kind()].Name() // byte is uint8, rune int32
	case *types.Named:
		if isWeakPointer(t) {
			return named(t) + "[" + str(weakElem(t)) + "]" // weak.Pointer[main.T] (weak.go)
		}
		return named(t)
	case *types.Pointer:
		return "*" + str(t.Elem())
	case *types.Slice:
		return "[]" + str(t.Elem())
	case *types.Array:
		return "[" + strconv.FormatInt(t.Len(), 10) + "]" + str(t.Elem())
	case *types.Map:
		return "map[" + str(t.Key()) + "]" + str(t.Elem())
	case *types.Chan:
		switch t.Dir() {
		case types.SendOnly:
			return "chan<- " + str(t.Elem())
		case types.RecvOnly:
			return "<-chan " + str(t.Elem())
		}
		return "chan " + str(t.Elem())
	case *types.Struct:
		if t.NumFields() == 0 {
			return "struct {}"
		}
		parts := make([]string, t.NumFields())
		for i := range t.NumFields() {
			f := t.Field(i)
			s := str(f.Type())
			if !f.Embedded() {
				name := f.Name()
				if fieldPkg && !exportedName(name) && f.Pkg() != nil && f.Pkg().Name() != "main" {
					name = pkgPath(f) + "." + name // (main's names stay as they were: they keep their hashes)
				}
				s = name + " " + s
			}
			if tag := t.Tag(i); tag != "" {
				s += " " + strconv.Quote(tag)
			}
			parts[i] = s
		}
		return "struct { " + strings.Join(parts, "; ") + " }"
	case *types.Signature:
		params := make([]string, t.Params().Len())
		for i := range params {
			p := t.Params().At(i).Type()
			if t.Variadic() && i == len(params)-1 {
				params[i] = "..." + str(p.(*types.Slice).Elem())
			} else {
				params[i] = str(p)
			}
		}
		s := "func(" + strings.Join(params, ", ") + ")"
		switch r := t.Results(); r.Len() {
		case 0:
		case 1:
			s += " " + str(r.At(0).Type())
		default:
			res := make([]string, r.Len())
			for i := range res {
				res[i] = str(r.At(i).Type())
			}
			s += " (" + strings.Join(res, ", ") + ")"
		}
		return s
	case *types.Interface:
		if t.NumMethods() == 0 {
			return "interface {}"
		}
	}
	return types.TypeString(t, func(p *types.Package) string { return p.Name() })
}
