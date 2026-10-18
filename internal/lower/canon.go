package lower

import (
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
}

// canonName returns the name of t as Go's %T prints it.
func (g *gen) canonName(t types.Type) string {
	return (&canon{}).str(t)
}

// canonText returns the identity text of t (a method's signature, for the itabs).
func (g *gen) canonText(t types.Type) string {
	return (&canon{ident: true, local: g.localTag}).str(t)
}

// localTag returns the unique tag of the type tn if it is declared in a function, else "".
func (g *gen) localTag(tn *types.TypeName) string {
	if tn.Pkg() == nil || tn.Parent() == nil || tn.Parent() == tn.Pkg().Scope() || tn.Parent() == types.Universe {
		return ""
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
	if c.ident {
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
			for i := range args {
				args[i] = c.str(targs.At(i))
			}
			s += "[" + strings.Join(args, ",") + "]"
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
				if c.ident && !exportedName(name) {
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
