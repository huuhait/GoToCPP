package lower

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Generic types (Phase 07 step 7.3, docs/adr/030-generic-instances.md).
//
// An instance of a generic type (Pair[string, []int]) is one C++ type for the whole program, as an
// unnamed struct type is (shared.go): the struct, its members, small inline methods, descriptor
// and method tables are written under include guards in the header of every package that uses
// it. Larger methods are defined once in a using package's descriptor unit. It belongs to the
// namespace of the package that declares the generic type, so
// that its members and the names in its methods' bodies are that package's, and the text is the
// same wherever it is written:
//
//	namespace gpkg_geom {                        // the package that declares type Pair[K, V any]
//	#ifndef GP_INST_3F2A9C01_PAIR
//	#define GP_INST_3F2A9C01_PAIR
//	struct gp_inst_3f2a9c01_Pair { ... members, declared methods ... };
//	#endif
//	}
//
// The declaring package's lowering makes the text (makeInstance), once, when all the packages are
// lowered: the struct from the instantiated underlying type that go/types has, each method from
// its declaration with the receiver's type parameters replaced by the type arguments (the
// substitution of generics.go: apply), the descriptor as for a declared type (typedesc.go), named
// "main.Pair[string,[]int]" as Go's %T prints it (canon.go). The declaring package's own tables
// (its header, its descriptors) are left as they were: what the instance needs of other packages'
// types is named fully qualified, as the same text is written in headers that include theirs.
//
// A generic function that a method's body instantiates is written in the same block, inline
// (generics.go).

// A genericType is a generic type declaration of a package, with its methods.
type genericType struct {
	obj     *types.TypeName
	spec    *ast.TypeSpec
	methods []*genericFunc
}

// An instType is an instance of a generic type: one per list of type arguments, in the whole
// program.
type instType struct {
	key   string
	named *types.Named // the first instance with this key that was met
	obj   *types.TypeName
	owner *gen // the lowering of the package that declares the generic type
	gt    *genericType
	si    *structInfo // a struct or a gp::newtype, or
	iface *ifaceInfo  // an interface
	fns   []*funcInfo // the methods, prepared (makeInstance)
	unit  string      // the unit of the instances that the methods ask for

	bodies      []cppir.Decl // non-inline method definitions, emitted once
	bodyPackage *gen         // a package whose header already includes the complete instance
}

// isInstance reports whether n is an instance of a generic type that the lowering writes (not
// weak.Pointer[T], a runtime class).
func isInstance(n *types.Named) bool {
	return n.TypeArgs().Len() > 0 && !isWeakPointer(n)
}

// regObj returns the object that the tables of the lowering (the structs, the descriptors, the
// interfaces) key the named type n by: its own, or for an instance of a generic type one that
// stands for the instance (go/types has one object for all the instances of a generic type).
func (g *gen) regObj(n *types.Named) *types.TypeName {
	if !isInstance(n) {
		return n.Obj()
	}
	return g.prog.instObj(g, n)
}

// instObj returns the object that stands for the instance n.
func (p *Program) instObj(g *gen, n *types.Named) *types.TypeName {
	key := g.canonText(n)
	if o := p.instObjs[key]; o != nil {
		return o
	}
	if p.instObjs == nil {
		p.instObjs = make(map[string]*types.TypeName)
	}
	o := types.NewTypeName(n.Obj().Pos(), n.Obj().Pkg(), n.Obj().Name()+"["+key+"]", nil)
	p.instObjs[key] = o
	return o
}

// shortName returns the name of the type n as Go writes it after the package: Pair[string,int].
func (g *gen) shortName(n *types.Named) string {
	if n.Obj().Pkg() == nil {
		return n.Obj().Name()
	}
	s := g.canonName(n)
	return s[len(n.Obj().Pkg().Name())+1:]
}

// recordGenericType notes the declaration of the generic type spec, which has no C++ of its own.
func (g *gen) recordGenericType(spec *ast.TypeSpec) {
	tn, _ := g.info.Defs[spec.Name].(*types.TypeName)
	if tn == nil {
		g.errorf(spec.Pos(), "internal error: no object for type %s", spec.Name.Name)
		return
	}
	if g.genericTypes == nil {
		g.genericTypes = make(map[*types.TypeName]*genericType)
	}
	if g.generics == nil {
		g.generics = make(map[*types.Func]*genericFunc)
	}
	g.privateInfoOnce()
	g.genericTypes[tn] = &genericType{obj: tn, spec: spec}
}

// recordGenericMethod notes the method d of the generic type named (the type of its receiver).
func (g *gen) recordGenericMethod(d *ast.FuncDecl, fn *types.Func, named *types.Named) {
	gt := g.genericTypes[named.Origin().Obj()]
	if gt == nil {
		g.errorf(d.Pos(), "internal error: the method %s of the generic type %s has no declaration of it", d.Name.Name, named.Obj().Name())
		return
	}
	if d.Body == nil {
		g.missingBody(d)
		return
	}
	gt.methods = append(gt.methods, &genericFunc{fn: fn, decl: d, owner: g})
}

// instOf returns the instance of the generic type that n is, made the first time (for the whole
// program), or nil with an error.
func (g *gen) instOf(n *types.Named, pos token.Pos) *instType {
	key := g.canonText(n)
	if it := g.prog.instTypes[key]; it != nil {
		return it
	}
	origin := n.Origin()
	owner := g.other(origin.Obj().Pkg())
	if owner == nil {
		g.notYet(pos, fmt.Sprintf("%s.%s is not available yet", origin.Obj().Pkg().Name(), origin.Obj().Name()))
		return nil
	}
	gt := owner.genericTypes[origin.Obj()]
	if gt == nil {
		g.errorf(pos, "internal error: no declaration of the generic type %s", origin.Obj().Name())
		return nil
	}
	for a := range n.TypeArgs().Types() {
		if g.hasCxxNamed(a) {
			g.notYet(pos, "an instance of the generic type "+origin.Obj().Name()+" with a C/C++ type argument of a generic function is not supported yet: the type arguments of a generic type are Go types")
			return nil
		}
		if hasC(a) {
			g.notYet(pos, "an instance of a generic type with a C/C++ type argument ("+g.canonName(n)+") is not supported yet: the type arguments of a generic type are Go types")
			return nil
		}
	}
	it := &instType{key: key, named: n, obj: g.prog.instObj(g, n), owner: owner, gt: gt}
	sum := sha256.Sum256([]byte(key))
	nn := 4
	name := "gp_inst_" + hex.EncodeToString(sum[:nn]) + "_" + owner.escape(origin.Obj().Name())
	for g.prog.instTypeNames[name] != "" && g.prog.instTypeNames[name] != key && nn < len(sum) {
		nn++
		name = "gp_inst_" + hex.EncodeToString(sum[:nn]) + "_" + owner.escape(origin.Obj().Name())
	}
	if g.prog.instTypeNames == nil {
		g.prog.instTypeNames = make(map[string]string)
		g.prog.instTypes = make(map[string]*instType)
	}
	g.prog.instTypeNames[name] = key
	g.prog.instTypes[key] = it
	it.unit = "\x00inst:" + key
	switch {
	case isInterface(n):
		if it, ok := n.Underlying().(*types.Interface); ok && !it.IsMethodSet() {
			g.unsupported(pos, "an instance of a generic constraint")
			return nil
		}
		info := owner.newIfaceQuiet(n, n.Underlying().(*types.Interface), name)
		info.named = n
		info.goName = g.canonName(n)
		info.owner = owner
		info.shared = &sharedDef{}
		info.inst = it
		it.iface = info
		g.prog.sharedOrder = append(g.prog.sharedOrder, sharedItem{iface: info})
	default:
		si := &structInfo{t: n, name: name, goName: g.canonName(n), owner: owner, shared: &sharedDef{}, inst: it}
		if isStruct(n) {
			st := n.Underlying().(*types.Struct)
			si.st = st
			si.fields = make([]string, st.NumFields())
			blanks := 0
			for i := range st.NumFields() {
				if f := st.Field(i); f.Name() == "_" {
					si.fields[i] = "gp_blank" + strconv.Itoa(blanks)
					blanks++
				} else {
					si.fields[i] = owner.methodName(name, f.Name())
				}
			}
		} else {
			if !isNewtype(n) {
				g.unsupported(pos, "the generic type "+g.canonName(n))
				return nil
			}
			si.under = n.Underlying()
		}
		it.si = si
		g.prog.sharedOrder = append(g.prog.sharedOrder, sharedItem{st: si})
		if owner.fwdNeed == nil {
			owner.fwdNeed = make(map[*structInfo]bool)
		}
		owner.fwdNeed[si] = true
	}
	return it
}

// instStruct returns the struct of the instance n of a generic struct or non-struct type, and
// notes that the package of g uses it.
func (g *gen) instStruct(n *types.Named, pos token.Pos) *structInfo {
	td := g.tdecls()
	reg := g.regObj(n)
	if si := td.named[reg]; si != nil {
		return si
	}
	if !g.split {
		g.notYet(pos, "generic types need the split layout")
		return nil
	}
	it := g.instOf(n, pos)
	if it == nil || it.si == nil {
		return nil
	}
	td.named[reg] = it.si
	td.list = append(td.list, it.si)
	g.noteShared(it.si)
	return it.si
}

// instIface returns the struct of the instance n of a generic interface type.
func (g *gen) instIface(n *types.Named, pos token.Pos) *ifaceInfo {
	tab := g.itable()
	reg := g.regObj(n)
	if info := tab.named[reg]; info != nil {
		return info
	}
	if !g.split {
		g.notYet(pos, "generic types need the split layout")
		return nil
	}
	it := g.instOf(n, pos)
	if it == nil || it.iface == nil {
		return nil
	}
	tab.named[reg] = it.iface
	tab.list = append(tab.list, it.iface)
	g.noteShared(it.iface)
	return it.iface
}

// makeInstance makes the shared text of the instance it, which this package declares: the struct
// with its members, the descriptor and the method tables, and the definitions of the methods.
func (g *gen) makeInstance(it *instType) {
	d := it.def()
	var refs []sharedItem
	self := it.item()
	// What the instance mentions of the shared types goes to its refs, not to this package's
	// header: the header of the package that declares the generic type does not know what its
	// importers pass as type arguments.
	savedUsed, savedList, savedRec := g.usedShared, g.sharedList, g.rec
	td := g.tdecls()
	nlist := len(td.list)
	g.usedShared, g.sharedList, g.rec = nil, nil, &refs
	g.inShared++
	defer func() {
		g.usedShared, g.sharedList, g.rec = savedUsed, savedList, savedRec
		td.list = td.list[:nlist]
		g.inShared--
	}()

	if it.iface != nil {
		d.decls = g.ifaceDef(it.iface)
		p := implParts{}
		g.ifaceImpl(it.iface, &p)
		d.impl = p.all
		for _, r := range refs {
			if r != self {
				d.refs = append(d.refs, r)
			}
		}
		return
	}
	si := it.si
	// The methods: their types replaced by the type arguments (apply), prepared so that the struct
	// declares them, defined after the prototypes.
	targs := make([]types.Type, it.named.TypeArgs().Len())
	for i := range targs {
		targs[i] = it.named.TypeArgs().At(i)
	}
	saved := struct {
		implicit   map[ast.Expr]types.Type
		uintCounts map[ast.Expr]bool
		unit       string
		info       *tinfo
	}{g.implicit, g.uintCounts, g.unit, g.info}
	defer func() {
		g.implicit, g.uintCounts, g.unit, g.info = saved.implicit, saved.uintCounts, saved.unit, saved.info
	}()
	g.implicit, g.uintCounts, g.unit = nil, make(map[ast.Expr]bool), it.unit
	for _, gm := range it.gt.methods {
		obj, err := gm.apply(targs, g.context(), nil)
		if err != nil {
			g.errorf(gm.decl.Pos(), "internal error: instantiating %s: %v", it.named, err)
			return
		}
		recv := obj.Signature().Recv()
		_, ptr := types.Unalias(recv.Type()).(*types.Pointer)
		m := &methodInfo{recv: recv, ptr: ptr, named: it.named}
		f := &funcInfo{decl: gm.decl, obj: obj, method: m}
		f.prepare(g)
		it.fns = append(it.fns, f)
	}
	// The descriptor and the method tables, then the struct (the tables ask for the forwarders).
	p := implParts{}
	nd := &namedDesc{t: it.named, ty: it.named, si: si, name: "gp_td_" + si.name}
	g.namedDecls(nd, &p)
	d.impl = p.all
	d.decls = append(g.structDef(si), &cppir.Blank{})
	// The definitions of the methods and of the forwarders, and the generic functions they use.
	for _, f := range it.fns {
		fn := g.funcBuild(f)
		if g.err != nil {
			return
		}
		if inlineInstanceMethod(f, fn) {
			fn.Specs = append([]string{"inline"}, fn.Specs...)
			d.post = append(d.post, fn, &cppir.Blank{})
		} else {
			it.bodies = append(it.bodies, fn, &cppir.Blank{})
		}
	}
	for _, fd := range g.forwardDefs(si) {
		if fn, ok := fd.(*cppir.Func); ok {
			fn.Specs = append([]string{"inline"}, fn.Specs...)
		}
		d.post = append(d.post, fd)
	}
	g.drainInstances()
	d.post = append(g.out.instDecls(it.unit), d.post...)
	for _, r := range refs {
		if r != self {
			d.refs = append(d.refs, r)
		}
	}
}

// inlineInstanceMethod keeps small methods visible without LTO. Count lowered statements,
// including receiver copies, temporaries and control-flow bodies: one Go statement can expand
// into many C++ statements. A tiny scalar loop stays inline too: outlining it can prevent useful
// optimization of its caller. Closures and calls spelled append, make or new are not small.
func inlineInstanceMethod(f *funcInfo, fn *cppir.Func) bool {
	complex := false
	ast.Inspect(f.body, func(n ast.Node) bool {
		if complex {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			complex = true
		case *ast.CallExpr:
			if id, ok := ast.Unparen(n.Fun).(*ast.Ident); ok {
				complex = id.Name == "append" || id.Name == "make" || id.Name == "new"
			}
		}
		return !complex
	})
	if complex {
		return false
	}

	remaining := 3
	var small func([]cppir.Stmt) bool
	small = func(stmts []cppir.Stmt) bool {
		for _, stmt := range stmts {
			switch stmt := stmt.(type) {
			case *cppir.Line:
				if !small(stmt.Stmts) {
					return false
				}
			case *cppir.Block:
				if !small(stmt.Stmts) {
					return false
				}
			case *cppir.For:
				remaining--
				if stmt.Init != nil && !small([]cppir.Stmt{stmt.Init}) {
					return false
				}
				if !small(stmt.Body.Stmts) {
					return false
				}
			case *cppir.While:
				remaining--
				if !small(stmt.Body.Stmts) {
					return false
				}
			case *cppir.If:
				remaining--
				if !small([]cppir.Stmt{stmt.Then}) {
					return false
				}
				if stmt.Else != nil && !small([]cppir.Stmt{stmt.Else}) {
					return false
				}
			case *cppir.Comment, *cppir.Blank:
			case *cppir.VarDecl:
				remaining -= len(stmt.Vars)
			case *cppir.ExprStmt, *cppir.Return:
				remaining--
			default:
				return false
			}
			if remaining < 0 {
				return false
			}
		}
		return true
	}
	return small(fn.Body.Stmts)
}

// placeInstanceBodies chooses a translation unit only after every shared dependency is known.
// The first using package in import order already has the instance and its dependencies in its
// header. The declaring package need not: Box[importer.Local] must not add an import cycle or
// change the declaring package's header. Names, descriptors and semantic ownership stay there.
func (p *Program) placeInstanceBodies() {
	for _, g := range p.gens {
		for _, item := range g.sharedClosure() {
			it := item.inst()
			if it != nil && len(it.bodies) > 0 && it.bodyPackage == nil {
				it.bodyPackage = g
			}
		}
	}
}

// instanceBodies returns this package's out-of-line definitions in their original namespaces.
func (g *gen) instanceBodies() []cppir.Decl {
	sec := sections{g: g}
	for _, item := range g.sharedClosure() {
		if it := item.inst(); it != nil && it.bodyPackage == g {
			item.put(&sec, it.bodies...)
		}
	}
	// These definitions are appended after unitFile has protected its own declarations.
	// An importer's native includes can also have macros used only in a moved method body.
	file := &cppir.File{Decls: sec.decls()}
	g.protectNames(file)
	return file.Decls
}

// def returns the shared definition of the instance.
func (it *instType) def() *sharedDef {
	if it.si != nil {
		return it.si.shared
	}
	return it.iface.shared
}

// item returns the shared item of the instance.
func (it *instType) item() sharedItem {
	if it.si != nil {
		return sharedItem{st: it.si}
	}
	return sharedItem{iface: it.iface}
}

// instDesc returns the address of the descriptor of the instance n of a generic type: the object
// of its owner's namespace, defined (inline) with the instance.
func (g *gen) instDesc(n *types.Named, pos token.Pos) cppir.Expr {
	if isInterface(n) {
		info := g.instIface(n, pos)
		if info == nil {
			return placeholder
		}
		return addrOf(g.descName(info.owner, "gp_td_"+info.name))
	}
	si := g.instStruct(n, pos)
	if si == nil {
		return placeholder
	}
	return addrOf(g.descName(si.owner, "gp_td_"+si.name))
}

// isGenericTypeExpr reports whether the index expression x instantiates a generic type
// (Box[std.string]), which the type with C/C++ parts of typeExpr can't spell.
func (g *gen) isGenericTypeExpr(x ast.Expr) bool {
	var base ast.Expr
	switch x := x.(type) {
	case *ast.IndexExpr:
		base = x.X
	case *ast.IndexListExpr:
		base = x.X
	default:
		return false
	}
	var id *ast.Ident
	switch b := ast.Unparen(base).(type) {
	case *ast.Ident:
		id = b
	case *ast.SelectorExpr:
		id = b.Sel
	}
	if id == nil {
		return false
	}
	tn, _ := g.info.Uses[id].(*types.TypeName)
	if tn == nil {
		return false
	}
	n, _ := types.Unalias(tn.Type()).(*types.Named)
	return n != nil && n.TypeParams().Len() > 0
}
