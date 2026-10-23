package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"
)

// The canonical spellings of types for the type descriptors and the itabs (Phase 05 step 5.4;
// docs/spec/cpp-mapping.md, "Interfaces"). The runtime compares method signatures as TEXT, and
// names every descriptor as Go's %T does, so the emitter must write
//
//   - the name of a type, as reflect's String (and %T) prints it: package NAMES (main.Rect,
//     net/url.Values is url.Values), byte as uint8, rune as int32, any as interface {}, no
//     parameter names, the methods of an interface type flattened and sorted ("interface {
//     Close() error; Read([]uint8) (int, error) }"), aliases unaliased, Go's spacing;
//   - the identity text of a signature: the same, but with the full import path of every
//     package (func(net/url.Values, ...string) (int, error)) and a function-local type made
//     unique (two types named L in two functions are two types, which %T prints alike).
//
// types.TypeString does neither (it keeps parameter names, byte, any and alias names and prints
// two local types alike). canon_test.go compares the names with %T of real Go for a corpus of
// types, and the texts with types.Identical.

// A canon writes types in the canonical spellings.
type canon struct {
	// ident: the identity text (import paths, local types made unique) instead of the name.
	ident bool
	// local returns the unique tag of a type declared in a function (its C++ name), "" for
	// a package-level type.
	local func(*types.TypeName) string
	// inArgs: the type is a type argument of a generic type's instance, which Go's %T writes with
	// the import paths of the packages: main.Stack[net/url.URL].
	inArgs bool
	// ordinal returns Go's number of a type declared in a function (0 for any other): gc writes
	// main.Box[main.point·1] for a type argument that is local to a function (localOrdinal).
	ordinal func(*types.TypeName) int
	// implicit returns, for the origin of a type declared in a generic function, how many of its
	// first type arguments are the function's: Go writes a semicolon after them.
	implicit func(*types.TypeName) int
}

// canonName returns the name of t as Go's %T prints it.
func (g *gen) canonName(t types.Type) string {
	return (&canon{ordinal: g.localOrdinal, implicit: g.localImplicit}).str(t)
}

// localOrdinal returns the number that Go's %T appends to a type declared in a function when it is a
// type argument (main.Box[main.point·1]): the 1-based place of its declaration among the defined
// types declared in function bodies of its package, in source order (files in the order of the
// package, closures and generic functions included, aliases not counted). 0 for a package-level
// type and for what the lowering does not know.
func (g *gen) localOrdinal(tn *types.TypeName) int {
	if tn.Pkg() == nil || tn.Pos() == token.NoPos {
		return 0
	}
	o := g.other(tn.Pkg())
	if o == nil {
		return 0
	}
	if o.localOrdinals == nil {
		o.localOrdinals = make(map[token.Pos]int)
		n := 0
		count := func(root ast.Node) {
			ast.Inspect(root, func(x ast.Node) bool {
				if ts, ok := x.(*ast.TypeSpec); ok && ts.Assign == token.NoPos {
					n++
					o.localOrdinals[ts.Name.Pos()] = n
				}
				return true
			})
		}
		for _, f := range o.pkg.Load.Files {
			for _, d := range f.AST.Decls {
				switch d := d.(type) {
				case *ast.FuncDecl:
					if d.Body != nil {
						count(d.Body)
					}
				case *ast.GenDecl:
					if d.Tok == token.VAR { // a function literal in an initializer
						count(d)
					}
				}
			}
		}
	}
	return o.localOrdinals[tn.Pos()]
}

// canonText returns the identity text of t (a method's signature, for the itabs).
func (g *gen) canonText(t types.Type) string {
	return (&canon{ident: true, local: g.localTag, implicit: g.localImplicit}).str(t)
}

// localImplicit returns how many leading type arguments of the type tn are those of the generic
// function that declares it (a type declared in a generic function: localtypes.go), else 0.
func (g *gen) localImplicit(tn *types.TypeName) int {
	if g.prog == nil {
		return 0
	}
	return g.prog.localImplicit[tn]
}

// localTag returns the unique tag of the type tn if it is declared in a function, else "".
func (g *gen) localTag(tn *types.TypeName) string {
	if g.prog != nil {
		if tag, ok := g.prog.localTagOf[tn]; ok {
			return tag // the synthetic origin of a type declared in a generic function (localtypes.go)
		}
	}
	if tn.Pkg() == nil || tn.Parent() == nil || tn.Parent() == tn.Pkg().Scope() || tn.Parent() == types.Universe {
		return ""
	}
	// The tag belongs to the package that declares the type: an instance of another package's generic
	// type (a.Box[a.Box[L]]) is made by that package, which must give L the tag that the declaring
	// package gives it, or one type gets two identities and two structs.
	if o := g.other(tn.Pkg()); o != nil && o != g {
		return o.localTag(tn)
	}
	if n, ok := g.tdecls().local[tn]; ok {
		return n
	}
	// A type that has no C++ struct of its own (a local interface): a number, in the order
	// that the types are first written (not its position: a line added above would rename it).
	if n, ok := g.localTags[tn]; ok {
		return n
	}
	if g.localTags == nil {
		g.localTags = make(map[*types.TypeName]string)
	}
	n := "L" + strconv.Itoa(len(g.localTags)+1)
	g.localTags[tn] = n
	return n
}

// pkgName returns how a package is written in a qualified name.
func (c *canon) pkgName(p *types.Package) string {
	if c.ident || c.inArgs {
		return importPath(p)
	}
	return p.Name()
}

// importPath returns the import path of p as Go's reflect has it: "main" for the main package
// (whatever the build calls it: go run main.go says command-line-arguments).
func importPath(p *types.Package) string {
	if p.Name() == "main" {
		return "main"
	}
	return p.Path()
}

// exportedName reports whether a name is exported (Unicode, as Go).
func exportedName(name string) bool {
	return token.IsExported(name)
}

// methodsOf returns the methods of the interface type it, all of them (embedded interfaces
// flattened), in the order of the runtime's tables: the exported names by name, then the
// unexported ones by name and package path.
func sortedMethods(it *types.Interface) []*types.Func {
	ms := make([]*types.Func, it.NumMethods())
	for i := range ms {
		ms[i] = it.Method(i)
	}
	sort.SliceStable(ms, func(i, j int) bool { return methodLess(ms[i], ms[j]) })
	return ms
}

// methodLess is the runtime's method_order (runtime/src/type.cpp): exported names first, then
// by name, then by package path, in byte order.
func methodLess(a, b *types.Func) bool {
	ea, eb := exportedName(a.Name()), exportedName(b.Name())
	if ea != eb {
		return ea
	}
	if a.Name() != b.Name() {
		return a.Name() < b.Name()
	}
	if ea {
		return false
	}
	return pkgPath(a) < pkgPath(b)
}

// pkgPath returns the import path of the package of a method or a field, "" for none.
func pkgPath(o types.Object) string {
	if o.Pkg() == nil {
		return ""
	}
	return importPath(o.Pkg())
}

func (c *canon) str(t types.Type) string {
	switch t := types.Unalias(t).(type) {
	case *types.Basic:
		if t.Kind() == types.UnsafePointer {
			return "unsafe.Pointer"
		}
		if t.Kind() == types.Invalid {
			return "?"
		}
		if t.Info()&types.IsUntyped != 0 {
			return types.Default(t).(*types.Basic).Name()
		}
		return types.Typ[t.Kind()].Name() // byte is uint8, rune int32
	case *types.Named:
		obj := t.Obj()
		if obj.Pkg() == nil {
			return obj.Name() // error, comparable
		}
		s := c.pkgName(obj.Pkg()) + "." + obj.Name()
		if targs := t.TypeArgs(); targs != nil && targs.Len() > 0 {
			args := make([]string, targs.Len())
			in := *c
			in.inArgs = true
			for i := range args {
				args[i] = in.str(targs.At(i))
			}
			list := strings.Join(args, ",")
			if c.implicit != nil {
				if n := c.implicit(obj); n > 0 && n < len(args) {
					list = strings.Join(args[:n], ",") + ";" + strings.Join(args[n:], ",")
				}
			}
			s += "[" + list + "]"
		}
		if c.inArgs && !c.ident && c.ordinal != nil {
			if n := c.ordinal(obj); n > 0 {
				s += "·" + strconv.Itoa(n)
			}
		}
		if c.ident && c.local != nil {
			if tag := c.local(obj); tag != "" {
				s += "·" + tag
			}
		}
		return s
	case *types.Pointer:
		return "*" + c.str(t.Elem())
	case *types.Slice:
		return "[]" + c.str(t.Elem())
	case *types.Array:
		return "[" + strconv.FormatInt(t.Len(), 10) + "]" + c.str(t.Elem())
	case *types.Map:
		return "map[" + c.str(t.Key()) + "]" + c.str(t.Elem())
	case *types.Chan:
		elem := c.str(t.Elem())
		switch t.Dir() {
		case types.SendOnly:
			return "chan<- " + elem
		case types.RecvOnly:
			return "<-chan " + elem
		}
		if ch, ok := types.Unalias(t.Elem()).(*types.Chan); ok && ch.Dir() == types.RecvOnly {
			elem = "(" + elem + ")" // chan (<-chan int)
		}
		return "chan " + elem
	case *types.Struct:
		if t.NumFields() == 0 {
			return "struct {}"
		}
		parts := make([]string, t.NumFields())
		for i := range parts {
			f := t.Field(i)
			s := c.str(f.Type())
			if !f.Embedded() {
				name := f.Name()
				if (c.ident || c.inArgs) && !exportedName(name) {
					name = pkgPath(f) + "." + name
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
		return "func" + c.sig(t)
	case *types.Interface:
		if t.NumMethods() == 0 {
			return "interface {}"
		}
		ms := sortedMethods(t)
		parts := make([]string, len(ms))
		for i, m := range ms {
			name := m.Name()
			if !exportedName(name) {
				if c.ident {
					name = pkgPath(m) + "." + name
				} else if m.Pkg() != nil {
					name = m.Pkg().Name() + "." + name
				}
			}
			parts[i] = name + c.sig(m.Signature())
		}
		return "interface { " + strings.Join(parts, "; ") + " }"
	case *types.Tuple:
		return c.tuple(t)
	}
	return types.TypeString(t, func(p *types.Package) string { return p.Name() })
}

// sig writes the parameters and results of a signature: (int, ...string) (bool, error).
func (c *canon) sig(t *types.Signature) string {
	params := make([]string, t.Params().Len())
	for i := range params {
		p := t.Params().At(i).Type()
		if t.Variadic() && i == len(params)-1 {
			if sl, ok := p.(*types.Slice); ok {
				params[i] = "..." + c.str(sl.Elem())
				continue
			}
		}
		params[i] = c.str(p)
	}
	s := "(" + strings.Join(params, ", ") + ")"
	switch r := t.Results(); r.Len() {
	case 0:
	case 1:
		s += " " + c.str(r.At(0).Type())
	default:
		s += " " + c.tuple(r)
	}
	return s
}

func (c *canon) tuple(t *types.Tuple) string {
	parts := make([]string, t.Len())
	for i := range parts {
		parts[i] = c.str(t.At(i).Type())
	}
	return "(" + strings.Join(parts, ", ") + ")"
}
