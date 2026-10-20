package lower

import (
	"go/constant"
	"go/types"
	"math"
	"slices"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// The split layout (Phase 06 README §2.2 and §2.3, steps 6.2 and 6.3): the files of one package.
//
//	gpkg/<import path>/
//	├── pkg.hpp          what the other packages and the package's own translation units need
//	├── render.gp.cpp    one translation unit for each .gp file that has functions
//	├── pkg_desc.cpp     the type descriptors of the package's types (and the forwarders)
//	└── pkg_init.cpp     the package variables, gp_var_init, gp_init_funcs; main's also has main
//
// pkg.hpp holds, in this order (a header is a C++ file, so a name is declared before its use):
//
//  1. #pragma once and gocpp's own header, gocpp/rt.hpp, first, so that no macro of a C/C++ header
//     reaches the runtime;
//  2. the headers of the gocpp packages the package imports (their types, which its own types
//     use), each by its path, gpkg/<import path>/pkg.hpp;
//  3. the package's C/C++ headers, but only when a declaration of the header needs a C/C++ type
//     (a field, a signature, a variable), and the #undef of the macros that the declarations'
//     names collide with. The translation units include them again for their bodies;
//  4. forward declarations of every struct and interface, shared (global) types first;
//  5. the interface structs, then the struct definitions in dependency order (a field by value
//     needs its type defined first), with the types of other packages fully qualified
//     (::gpkg_geom::Point), the fmt hooks (inline) and the inline members of the interfaces;
//  6. extern declarations of the descriptors (defined in pkg_desc.cpp), the prototypes of the
//     functions, the extern package variables (defined in pkg_init.cpp), gp_var_init and
//     gp_init_funcs. Exported constants: every use of a constant is folded (expr.go), so a header
//     has none; C++ users of the package find them in the .gp source (docs/spec/cpp-mapping.md).
//
// A package's files that other packages' files include are only this header: the translation
// units of a package are compiled alone, and the object cache keys them by their text and the
// headers they read.

// lowerSplit lowers the package into g.out for the split layout.
func (g *gen) lowerSplit() bool {
	if g.pkg.Load.IsProgram() {
		mainFn, _ := g.pkg.Types.Scope().Lookup("main").(*types.Func)
		if mainFn == nil {
			g.errorf(g.firstPos(), "%v", errNoMain)
			return false
		}
	}
	return g.lower()
}

// files returns the files of the package, after every package that imports it was lowered and
// has asked for its descriptors.
func (g *gen) files() *PackageFiles {
	pf := &PackageFiles{ImportPath: g.pkg.Load.ImportPath, Namespace: g.ns, Dir: PkgDir(g.pkg.Load.ImportPath), HeaderC: g.headerNeedsC()}
	for _, fi := range g.funcs {
		if fi.decl != nil && fi.decl.Recv == nil && fi.decl.Name.Name != "init" {
			if pf.Funcs == nil {
				pf.Funcs = make(map[string]string)
			}
			pf.Funcs[fi.decl.Name.Name] = fi.name
		}
	}
	parts := g.structDecls()
	if g.err != nil {
		return nil
	}
	g.pendingAsserts = g.constAsserts() // typedconsts.go: the first translation unit takes them
	pf.Header = g.headerFile(parts, pf)
	// The translation units, in the order of the files.
	used := 0
	for _, f := range g.pkg.Load.Files {
		decls := g.out.byFile[f.Path]
		if len(decls) == 0 {
			continue
		}
		decls = slices.Concat(g.out.instDecls(f.Path), decls) // generics.go
		used++
		u := &Unit{Name: unitName(f.Path), Src: f.Path}
		u.File = g.unitFile(pf, u.Name, decls)
		pf.Units = append(pf.Units, u)
	}
	if used != len(g.out.byFile) {
		g.errorf(g.firstPos(), "internal error: functions of a file that is not a file of the package %s are left out", g.pkg.Load.ImportPath)
		return nil
	}
	// The descriptors and the forwarders.
	var desc []cppir.Decl
	if parts != nil {
		for _, d := range parts.post {
			desc = append(desc, d.fwd...)
		}
		desc = append(desc, parts.impl.def...)
	}
	bodies := g.instanceBodies()
	if len(desc) > 0 || len(bodies) > 0 || parts != nil && len(parts.globalFwd) > 0 {
		pf.Desc = g.unitFile(pf, "pkg_desc.cpp", desc)
		if parts != nil && len(parts.globalFwd) > 0 {
			pf.Desc.Decls = append(pf.Desc.Decls, parts.globalFwd...)
		}
		pf.Desc.Decls = append(pf.Desc.Decls, bodies...)
	}
	pf.Init = g.initFile(pf)
	if g.err != nil {
		return nil
	}
	return pf
}

// A section is a run of declarations of the header that are in the package's namespace or in
// the global one.
type section struct {
	global bool
	ns     string // the namespace of a section that is not global, "" for the package's own
	decls  []cppir.Decl
}

// sections builds the top level of a header from sections, joining the neighbors that are in the
// same namespace.
type sections struct {
	g    *gen
	list []section
}

func (s *sections) add(global bool, decls ...cppir.Decl) {
	s.addNS(global, "", decls...)
}

// addNS is add for a section of the namespace ns (the namespace of the package that declares an
// instance of a generic type), or the package's own for "".
func (s *sections) addNS(global bool, ns string, decls ...cppir.Decl) {
	if len(decls) == 0 {
		return
	}
	if !global && ns == s.g.ns {
		ns = ""
	}
	// A blank line belongs to the section it follows, and there is none first or twice.
	if blank := isBlankDecl(decls[0]); blank && len(decls) == 1 {
		if n := len(s.list); n > 0 && !isBlankDecl(s.list[n-1].decls[len(s.list[n-1].decls)-1]) {
			s.list[n-1].decls = append(s.list[n-1].decls, decls[0])
		}
		return
	}
	if n := len(s.list); n > 0 && s.list[n-1].global == global && s.list[n-1].ns == ns {
		s.list[n-1].decls = append(s.list[n-1].decls, decls...)
		return
	}
	s.list = append(s.list, section{global, ns, slices.Clone(decls)})
}

func isBlankDecl(d cppir.Decl) bool {
	_, ok := d.(*cppir.Blank)
	return ok
}

// decls returns the declarations: the namespace blocks of the package, and the global ones as
// they are.
func (s *sections) decls() []cppir.Decl {
	var out []cppir.Decl
	for _, sec := range s.list {
		if sec.global {
			out = append(out, sec.decls...)
			continue
		}
		name := sec.ns
		if name == "" {
			name = s.g.ns
		}
		out = append(out, &cppir.Namespace{Name: name, Decls: sec.decls}, &cppir.Blank{})
	}
	return out
}

// headerFile returns pkg.hpp (see the top of the file).
func (g *gen) headerFile(parts *typeParts, pf *PackageFiles) *cppir.File {
	f := &cppir.File{}
	path := HeaderPath(pf.ImportPath)
	f.Decls = append(f.Decls,
		&cppir.Comment{Text: path + ": generated by gocpp, do not edit"},
		&cppir.Directive{Text: "pragma once"},
		&cppir.Include{Path: "gocpp/rt.hpp", System: true})
	concepts := g.conceptDecls() // concepts.go (Options.Concepts)
	if len(concepts) > 0 {
		f.Decls = append(f.Decls, &cppir.Include{Path: "gocpp/concepts.hpp", System: true})
	}
	for _, imp := range g.importOrder() {
		f.Decls = append(f.Decls, &cppir.Include{Path: HeaderPath(imp)})
	}
	if g.headerNeedsC() {
		f.Decls = append(f.Decls, g.includes()...)
	}
	f.Decls = append(f.Decls, &cppir.Blank{})

	var sec sections
	sec.g = g
	closure := g.sharedClosure()
	// 4. forward declarations
	for _, it := range closure {
		it.put(&sec, &cppir.Struct{Name: it.name(), Forward: true})
	}
	if parts != nil {
		for _, si := range parts.structs {
			if si.shared == nil {
				sec.add(false, &cppir.Struct{Name: si.name, Forward: true})
			}
		}
	}
	// The interface structs of the package too: a shared (unnamed) interface struct below may
	// return one (interface{ Self() Mapper } with a named interface Mapper).
	if slices.ContainsFunc(closure, func(it sharedItem) bool { return it.iface != nil }) {
		for _, info := range g.itable().list {
			if info.shared == nil {
				sec.add(false, &cppir.Struct{Name: info.name, Forward: true})
			}
		}
	}
	sec.add(true, &cppir.Blank{})
	// 5. the interface structs and the structs
	for _, it := range closure {
		if it.iface != nil {
			it.put(&sec, guarded(it, "", it.def().decls)...)
		}
	}
	if parts != nil {
		sec.add(false, parts.ifaces...) // the package's own interface structs: forward declarations, then definitions
	}
	defined := make(map[sharedItem]bool)
	if parts != nil {
		for _, d := range parts.defs {
			if d.si.shared != nil {
				it := sharedItem{st: d.si}
				defined[it] = true
				it.put(&sec, guarded(it, "", d.si.shared.decls)...)
				continue
			}
			sec.add(false, d.decls...)
		}
	}
	// The structs of the closure come in pre-order (a struct before the ones its definition
	// mentions), but a struct holding another by value needs it complete first: each is written
	// after the closure structs it holds by value.
	inClosure := make(map[sharedItem]bool, len(closure))
	for _, it := range closure {
		inClosure[it] = true
	}
	emitted := make(map[sharedItem]bool)
	var emitSt func(it sharedItem)
	emitSt = func(it sharedItem) {
		if emitted[it] || defined[it] {
			return
		}
		emitted[it] = true
		for _, dep := range g.valueDepItems(it) {
			if inClosure[dep] {
				emitSt(dep)
			}
		}
		it.put(&sec, guarded(it, "", it.def().decls)...)
	}
	for _, it := range closure {
		if it.st != nil {
			emitSt(it)
		}
	}
	hasInst := slices.ContainsFunc(closure, func(it sharedItem) bool { return it.inst() != nil })
	// The descriptors of the instances of generic types are defined (inline) in their impl blocks,
	// and a shape names the descriptor of its element, which may come later: declared first.
	for _, it := range closure {
		if it.inst() != nil {
			it.put(&sec, &cppir.VarDecl{Specs: []string{"extern", "const"}, Type: typeName("gp::type_desc"),
				Vars: []cppir.Declarator{{Name: "gp_td_" + it.name()}}})
		}
	}
	if parts != nil && hasInst {
		sec.add(false, parts.impl.ext...) // (the instances of generic types name the package's descriptors)
	}
	for _, it := range closure {
		it.put(&sec, guarded(it, "_IMPL", it.def().impl)...)
	}
	sec.add(false, concepts...)
	// 6. what the translation units need
	var tail []cppir.Decl
	if parts != nil {
		if !hasInst {
			tail = append(tail, parts.impl.ext...)
		}
		tail = append(tail, parts.impl.inl...)
	}
	tail = append(tail, g.constDecls()...)
	tail = append(tail, g.out.protos...)
	tail = append(tail, g.out.varExt...)
	if g.varInits {
		tail = append(tail, &cppir.Func{Result: typeName("void"), Name: "gp_var_init"})
	}
	if len(g.initFns) > 0 {
		tail = append(tail, &cppir.Func{Result: typeName("void"), Name: "gp_init_funcs"})
	}
	sec.add(false, tail...)
	// The methods of the instances of generic types, which use what the package's header declares.
	for _, it := range closure {
		if it.inst() != nil {
			it.put(&sec, guarded(it, "_POST", it.def().post)...)
		}
	}
	f.Decls = append(f.Decls, sec.decls()...)
	if g.headerNeedsC() {
		g.protectNames(f) // macros.go
	}
	return f
}

// headerNeedsC reports whether a declaration of the header needs a C/C++ type, so that the header
// includes the package's C/C++ headers (see the top of the file).
func (g *gen) headerNeedsC() bool {
	if g.interop == nil {
		return false
	}
	for _, si := range g.tdecls().list {
		if si.owner != g {
			continue
		}
		if si.under != nil && !complete(si.under) {
			return true
		}
		for i := range si.fieldCount() {
			if !complete(si.st.Field(i).Type()) {
				return true
			}
		}
	}
	for _, f := range g.funcs {
		if !complete(f.obj.Type()) {
			return true
		}
	}
	for _, v := range g.vars {
		if !complete(v.Type()) {
			return true
		}
	}
	return false
}

// unitFile returns a translation unit of the package that holds decls in its namespace: the
// package's header, its C/C++ headers, and the declarations.
func (g *gen) unitFile(pf *PackageFiles, name string, decls []cppir.Decl) *cppir.File {
	f := &cppir.File{}
	f.Decls = append(f.Decls,
		&cppir.Comment{Text: pf.Dir + "/" + name + ": generated by gocpp, do not edit"},
		&cppir.Include{Path: HeaderPath(pf.ImportPath)})
	g.addCIncludes(f)
	// The guard of the constants (typedconsts.go) goes into one translation unit of the package, the first.
	f.Decls = append(f.Decls, g.takeConstAsserts()...)
	f.Decls = append(f.Decls, &cppir.Blank{})
	if len(decls) > 0 {
		f.Decls = append(f.Decls, &cppir.Namespace{Name: g.ns, Decls: slices.Concat([]cppir.Decl{&cppir.Blank{}}, decls)}, &cppir.Blank{})
	}
	g.protectNames(f) // macros.go
	return f
}

// addCIncludes adds the package's C/C++ includes to the translation unit f (after gocpp's own
// headers, then fmt's std::formatter support, which needs to come after them).
func (g *gen) addCIncludes(f *cppir.File) {
	incs := g.includes()
	if len(incs) == 0 {
		return
	}
	if g.interop == nil {
		g.errorf(g.firstPos(), "internal error: a package with header imports without the compiler probes (lower.ProbeInterop)")
		return
	}
	f.Decls = append(f.Decls, incs...)
	f.Decls = append(f.Decls, &cppir.Include{Path: "gocpp/fmtcxx.hpp", System: true})
}

// initFile returns pkg_init.cpp: the package variables, gp_var_init and gp_init_funcs, and for the
// main package the program's entry: gp_init_all, which initializes the packages in the order
// that Go does (the packages of the Program are in it), and main.
func (g *gen) initFile(pf *PackageFiles) *cppir.File {
	decls := slices.Concat(g.out.instDecls(initUnit), g.out.vars, g.out.inits)
	// The program's entry: the main package, and in a library (Options.Library: c-archive, c-shared,
	// cpp-lib) the last package of the program, which has gp_init_all but no main: the library's
	// glue calls gp_init_all when its first function is called (gocpp/lib.hpp).
	lib := g.opts.Library
	main := g.pkg.Load.IsProgram()
	entry := main && !lib || lib && g == g.prog.gens[len(g.prog.gens)-1]
	if len(decls) == 0 && !entry {
		return nil
	}
	f := g.unitFile(pf, "pkg_init.cpp", decls)
	if !entry {
		return f
	}
	var mainFn *types.Func
	if !lib {
		mainFn, _ = g.pkg.Types.Scope().Lookup("main").(*types.Func)
		if mainFn == nil {
			g.errorf(g.firstPos(), "%v", errNoMain)
			return nil
		}
	}
	init := cppir.Expr(&cppir.Lit{Text: "nullptr"})
	body := &cppir.Block{}
	for _, o := range g.prog.gens {
		if o.varInits {
			body.Stmts = append(body.Stmts, &cppir.ExprStmt{X: call(o.ns + "::gp_var_init")})
		}
		if len(o.initFns) > 0 {
			body.Stmts = append(body.Stmts, &cppir.ExprStmt{X: call(o.ns + "::gp_init_funcs")})
		}
	}
	if len(body.Stmts) > 0 || lib {
		init = &cppir.Unary{Op: "&", X: cxxName("gp_init_all")}
		f.Decls = append(f.Decls, &cppir.Func{Result: typeName("void"), Name: "gp_init_all", Body: body}, &cppir.Blank{})
	}
	if lib {
		return f
	}
	run := call("gp::run_main", cxxName("argc"), cxxName("argv"), init, &cppir.Unary{Op: "&", X: cxxName(g.ns + "::" + g.names[mainFn])})
	f.Decls = append(f.Decls, &cppir.Func{
		Result: typeName("int"),
		Name:   "main",
		Params: []cppir.Param{
			{Type: typeName("int"), Name: "argc"},
			{Type: &cppir.Ptr{Elem: &cppir.Ptr{Elem: typeName("char")}}, Name: "argv"},
		},
		Body: &cppir.Block{Stmts: []cppir.Stmt{&cppir.Return{X: run}}},
	})
	return f
}

// constDecls returns the exported constants of the package as C++ constants, for the C++ code that
// uses the package (docs/spec/cpp-mapping.md): a number or a bool is an inline constexpr of its
// type (an untyped constant has its default type), a string a constexpr std::string_view (a
// gp::string is not constexpr), a constant of a named type an inline const of it. The generated
// code never reads them: go/types folds every use of a constant (expr.go), so a changed
// constant changes the code that uses it, and the objects of the packages that use it are
// rebuilt because their text changes.
func (g *gen) constDecls() []cppir.Decl {
	var decls []cppir.Decl
	for _, c := range g.consts {
		t := c.Type()
		if b, ok := t.(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
			t = types.Default(t)
		}
		name := g.escape(c.Name())
		switch {
		case isNewtype(t) && basicOf(under(t)) != nil && basicOf(under(t)).Info()&types.IsComplex == 0:
			decls = append(decls, &cppir.VarDecl{Specs: []string{"inline", "const"}, Type: g.cxxType(t, c.Pos()),
				Vars: []cppir.Declarator{{Name: name, Init: g.constant(c.Val(), t, c.Pos(), false)}}})
		case basicOf(t) == nil, basicOf(t).Info()&types.IsComplex != 0:
		case basicOf(t).Info()&types.IsString != 0:
			if c.Val().Kind() == constant.String {
				decls = append(decls, &cppir.VarDecl{Specs: []string{"inline", "constexpr"}, Type: typeName("std::string_view"),
					// (the length too: a string may hold NUL bytes)
					Vars: []cppir.Declarator{{Name: name, Init: call("std::string_view", lit(cppir.CString(constant.StringVal(c.Val()))), lit(strconv.Itoa(len(constant.StringVal(c.Val())))))}}})
			}
		case fitsBasic(c.Val(), basicOf(t)):
			decls = append(decls, &cppir.VarDecl{Specs: []string{"inline", "constexpr"}, Type: g.cxxType(t, c.Pos()),
				Vars: []cppir.Declarator{{Name: name, Init: g.constant(c.Val(), t, c.Pos(), false)}}})
		}
	}
	if len(decls) > 0 {
		decls = append(decls, &cppir.Blank{})
	}
	return decls
}

// fitsBasic reports whether the constant v fits the basic type b (an untyped constant can be
// larger than any C++ integer).
func fitsBasic(v constant.Value, b *types.Basic) bool {
	switch {
	case b.Info()&types.IsBoolean != 0:
		return v.Kind() == constant.Bool
	case b.Info()&types.IsInteger != 0:
		if b.Info()&types.IsUnsigned != 0 {
			_, ok := constant.Uint64Val(constant.ToInt(v))
			return ok && constant.ToInt(v).Kind() == constant.Int
		}
		_, ok := constant.Int64Val(constant.ToInt(v))
		return ok && constant.ToInt(v).Kind() == constant.Int
	case b.Info()&types.IsFloat != 0:
		f := constant.ToFloat(v)
		if f.Kind() == constant.Unknown {
			return false
		}
		if b.Kind() == types.Float32 {
			x, _ := constant.Float32Val(f)
			return !math.IsInf(float64(x), 0)
		}
		x, _ := constant.Float64Val(f)
		return !math.IsInf(x, 0) // 1e400 is a fine untyped constant, but no float64
	}
	return false
}
