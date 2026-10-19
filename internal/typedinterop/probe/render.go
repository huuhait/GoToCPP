package probe

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// The renderer prints the collector's expression trees (collect/ir.go) as
// C++ for a probe. It needs the kind of each C/C++ path (what the kind probe
// found): a "::" or a "." after a prefix, a template instance or an index,
// a type or a value.

// A rend renders one reference.
type rend struct {
	kinds map[string]interop.Kind
	g     *gen

	// flip chooses the other reading of an ambiguous node (a path whose kind
	// is unknown): a type instead of a value, an instance instead of an
	// index.
	flip bool

	// ambiguous is set when such a node was met.
	ambiguous bool

	// pre are the alias lines (RoleAlias) that the rendered text needs and
	// that no earlier reference emitted; used are the names of all the aliases
	// the text uses, emitted before or not (a unit holds all of them).
	pre  []Line
	used []string

	// reason is why the reference can't be rendered.
	reason string
}

func (r *rend) fail(format string, a ...any) string {
	if r.reason == "" {
		r.reason = fmt.Sprintf(format, a...)
	}
	return "0"
}

// kind returns the kind of the path and whether the kind probe answered.
func (r *rend) kind(p *collect.Path) (interop.Kind, bool) {
	if p.Ns == "C" {
		return interop.Type, true
	}
	k, ok := r.kinds[r.eff(p).KindPath()]
	if !ok || k == interop.Unknown {
		return interop.Unknown, false
	}
	return k, true
}

func (r *rend) prefixKind(p *collect.Path, n int) (interop.Kind, bool) {
	return r.kind(&collect.Path{Ns: p.Ns, Hdr: p.Hdr, Parts: p.Parts[:n]})
}

// eff returns the path as the C++ lookup has it. A header package whose name
// is also a namespace that holds the first name (oop.scale with namespace oop
// { scale }) is looked up in the namespace first, as the opaque build does
// (lower.cBase): the path is ::oop::scale. A macro of that name, or a name
// that the namespace doesn't have, stays at the global level.
func (r *rend) eff(p *collect.Path) *collect.Path {
	if p.Ns != "" || p.Hdr == "" || len(p.Parts) == 0 {
		return p
	}
	first := p.Parts[0]
	if r.kinds[p.Hdr] != interop.Namespace || r.kinds[first] == interop.Macro {
		return p
	}
	if k := r.kinds[p.Hdr+"::"+first]; k == interop.Macro || !kindFound(k) {
		return p
	}
	return &collect.Path{Ns: "::" + p.Hdr, Parts: p.Parts}
}

func kindFound(k interop.Kind) bool {
	return k != interop.NotFound && k != interop.Unknown && k != interop.NotNamespace
}

// isMacro reports whether e is a path whose first part is a macro.
func (r *rend) isMacro(e collect.Expr) bool {
	p, ok := e.(*collect.Path)
	if !ok || p.Ns == "C" {
		return false
	}
	k, _ := r.prefixKind(p, 1)
	return k == interop.Macro
}

// form returns the text of the reference e, whether it is a type, and
// whether it could be rendered.
func (r *rend) form(e collect.Expr) (text string, isType, ok bool) {
	r.reason = ""
	isType = r.isType(e)
	if isType {
		text = r.typ(e)
	} else {
		text = r.value(e)
	}
	return text, isType, r.reason == ""
}

// isType reports whether the tree e is a type.
func (r *rend) isType(e collect.Expr) bool {
	switch e := e.(type) {
	case *collect.Path:
		k, known := r.kind(e)
		if tagName(e) && (!known || k == interop.NotFound) {
			return true // geo.struct_geo_pair: struct geo_pair, a type
		}
		if !known {
			// A member of a value (RAYWHITE.r, with RAYWHITE a macro or a variable) is
			// a value; else the kind is unknown and a value is the first reading.
			for i := 1; i < len(e.Parts); i++ {
				if pk, ok := r.prefixKind(e, i); ok && (pk == interop.Value || pk == interop.ValueTemplate || pk == interop.Macro) {
					return false
				}
			}
			return r.ambig(false)
		}
		return k == interop.Type
	case *collect.Inst:
		return r.instIsType(e.X)
	case *collect.IndexOrInst:
		_, typ := r.idx(e.X)
		return typ
	case *collect.Star:
		return r.isType(e.X)
	case *collect.Paren:
		return r.isType(e.X)
	case *collect.Stand, *collect.GoType, *collect.Decay, *collect.DeclRef, *collect.TupleElem, *collect.RangeElem:
		return true
	case *collect.Select:
		if r.isType(e.X) {
			// X::name: a nested type or a static member; the value reading first.
			return r.ambig(false)
		}
	}
	return false
}

// ambig is the choice at an ambiguous node: normal is the default reading,
// flipped when r.flip.
func (r *rend) ambig(normal bool) bool {
	r.ambiguous = true
	if r.flip {
		return !normal
	}
	return normal
}

// instIsType reports whether X[args] is a type: X is a class template. A
// function template (std::max[int]) is a value.
func (r *rend) instIsType(x collect.Expr) bool {
	p, ok := x.(*collect.Path)
	if !ok {
		return false
	}
	k, known := r.kind(p)
	if !known {
		return r.ambig(true) // the kind is unknown: a type first
	}
	return k == interop.Template
}

// idx decides what X[i] with one operand is: inst says that it is a
// template instance (else an index), typ that the instance is a type.
func (r *rend) idx(x collect.Expr) (inst, typ bool) {
	p, ok := x.(*collect.Path)
	if !ok {
		return false, false
	}
	k, known := r.kind(p)
	if !known {
		r.ambiguous = true // the kind is unknown: an index first
		return r.flip, r.flip
	}
	return k == interop.Template || k == interop.ValueTemplate, k == interop.Template
}

// cTypes are the C types of cgo's C package that gocpp knows.
var cTypes = map[string]string{
	"char": "char", "schar": "signed char", "uchar": "unsigned char", "short": "short", "ushort": "unsigned short",
	"int": "int", "uint": "unsigned int", "long": "long", "ulong": "unsigned long", "longlong": "long long",
	"ulonglong": "unsigned long long", "float": "float", "double": "double", "size_t": "size_t", "ssize_t": "ssize_t",
	"intptr_t": "intptr_t", "uintptr_t": "uintptr_t", "int8_t": "int8_t", "uint8_t": "uint8_t", "int16_t": "int16_t",
	"uint16_t": "uint16_t", "int32_t": "int32_t", "uint32_t": "uint32_t", "int64_t": "int64_t", "uint64_t": "uint64_t",
	"wchar_t": "wchar_t", "bool": "bool", "_Bool": "bool", "ptrdiff_t": "ptrdiff_t",
}

// cgoHelpers are the calls of cgo's own helpers that give C++ a value, with
// the type that C++ sees: C.cstr(s) is a holder that converts to const char*,
// C.CString(s) a char*, C.GoString(p) a gocpp string.
var cgoHelpers = map[string]string{
	"cstr": "const char*", "wstr": "const wchar_t*", "WString": "wchar_t*", "CString": "char*", "GoString": "gocpp_probe_internal_pr::string", "GoStringN": "gocpp_probe_internal_pr::string",
	"GoStringW": "gocpp_probe_internal_pr::string", "GoStringWN": "gocpp_probe_internal_pr::string",
	"GoBytes": "gocpp_probe_internal_pr::slice<uint8_t>",
}

// tagName reports whether p is a cgo tag name of a header package: struct_x,
// union_x or enum_x alone.
func tagName(p *collect.Path) bool {
	if p.Ns != "" || len(p.Parts) != 1 {
		return false
	}
	for _, tag := range []string{"struct_", "union_", "enum_"} {
		if rest, ok := strings.CutPrefix(p.Parts[0], tag); ok && rest != "" {
			return true
		}
	}
	return false
}

// pathText returns the C++ name that the path stands for.
func (r *rend) pathText(p *collect.Path) string {
	if p.Ns == "C" {
		if len(p.Parts) != 1 {
			return r.fail("cgo name C.%s has no C++ spelling", strings.Join(p.Parts, "."))
		}
		n := p.Parts[0]
		if t, ok := cTypes[n]; ok {
			return t
		}
		for _, tag := range []string{"struct", "union", "enum"} {
			if rest, ok := strings.CutPrefix(n, tag+"_"); ok && rest != "" {
				return tag + " " + rest
			}
		}
		return r.fail("cgo name C.%s has no C++ spelling", n)
	}
	p = r.eff(p)
	var b strings.Builder
	k0, known0 := r.prefixKind(p, 1)
	if tagName(p) && !(known0 && k0 != interop.NotFound) {
		// A cgo tag name through a header package: geo.struct_geo_pair is struct geo_pair.
		for _, tag := range []string{"struct", "union", "enum"} {
			if rest, ok := strings.CutPrefix(p.Parts[0], tag+"_"); ok {
				return tag + " " + rest
			}
		}
	}
	switch {
	case p.Ns != "":
		b.WriteString(p.Ns + "::" + p.Parts[0])
	case k0 == interop.Macro:
		b.WriteString(p.Parts[0]) // a macro: no :: (::EOF is not C++)
	default:
		b.WriteString("::" + p.Parts[0])
	}
	text := b.String()
	member := false
	for i := 1; i < len(p.Parts); i++ {
		prev, _ := r.prefixKind(p, i)
		if member || prev == interop.Value || prev == interop.ValueTemplate || prev == interop.Macro {
			member = true
			text = "gocpp_probe_internal_pr::deref(" + text + ")." + p.Parts[i]
		} else {
			text += "::" + p.Parts[i]
		}
	}
	return text
}

func (r *rend) args(es []collect.Expr) string {
	parts := make([]string, len(es))
	for i, e := range es {
		parts[i] = r.value(e)
	}
	return strings.Join(parts, ", ")
}

// targs renders template arguments: types and constants.
func (r *rend) targs(es []collect.Expr) string {
	parts := make([]string, len(es))
	for i, e := range es {
		if r.isType(e) {
			parts[i] = r.typ(e)
		} else {
			parts[i] = r.value(e)
		}
	}
	return strings.Join(parts, ", ")
}

// value renders e as an expression.
func (r *rend) value(e collect.Expr) string {
	switch e := e.(type) {
	case *collect.Path:
		if r.isType(e) {
			return r.fail("type %s used as a value", e.String())
		}
		return r.pathText(e)
	case *collect.Inst:
		if r.instIsType(e.X) {
			return r.fail("template %s used as a value", e.X.String())
		}
		return r.value(e.X) + "<" + r.targs(e.Args) + ">"
	case *collect.IndexOrInst:
		if inst, typ := r.idx(e.X); inst {
			if typ {
				return r.fail("template %s used as a value", e.X.String())
			}
			return r.value(e.X) + "<" + r.targs(e.Args) + ">"
		}
		return "(" + r.value(e.X) + ")[" + r.args(e.Args) + "]"
	case *collect.Arg:
		t := r.typ(e.T)
		if e.LValue {
			return "std::declval<" + t + "&>()"
		}
		return "std::declval<" + t + ">()"
	case *collect.Lit:
		return e.Text
	case *collect.Paren:
		return "(" + r.value(e.X) + ")"
	case *collect.Unary:
		op := e.Op
		switch op {
		case "^":
			op = "~"
		case "<-":
			return r.fail("receive from a channel")
		}
		return "(" + op + r.value(e.X) + ")"
	case *collect.Binary:
		if e.Op == "&^" {
			return "(" + r.value(e.X) + " & ~" + r.value(e.Y) + ")"
		}
		return "(" + r.value(e.X) + " " + e.Op + " " + r.value(e.Y) + ")"
	case *collect.Star:
		if r.isType(e.X) {
			return "std::declval<" + r.typ(e.X) + "*>()"
		}
		return "(*" + r.value(e.X) + ")"
	case *collect.Addr:
		return "(&" + r.value(e.X) + ")"
	case *collect.Select:
		if r.isType(e.X) {
			return r.typ(e.X) + "::" + e.Name
		}
		return "gocpp_probe_internal_pr::deref(" + r.value(e.X) + ")." + e.Name
	case *collect.Call:
		if e.Ellipsis {
			return r.fail("call with a spread argument")
		}
		if p, ok := e.Fun.(*collect.Path); ok && p.Ns == "C" && len(p.Parts) == 1 {
			if t, ok := cgoHelpers[p.Parts[0]]; ok {
				return "std::declval<" + t + ">()" // what the helper gives to C++ (Phase 04, step 4.9)
			}
		}
		return r.callee(e.Fun) + "(" + r.args(e.Args) + ")"
	case *collect.Composite:
		return r.composite(e)
	case *collect.Unknown:
		return r.fail("%s", e.Why)
	case *collect.SliceX:
		return r.fail("slice of a C/C++ value")
	}
	return r.fail("%s used as a value", e.String())
}

// callee renders the function of a call: a constructor or a conversion for a
// type, else the value.
func (r *rend) callee(f collect.Expr) string {
	if r.isType(f) {
		return simpleType(r.typ(f))
	}
	switch f := f.(type) {
	case *collect.Path, *collect.Inst, *collect.IndexOrInst:
		return r.value(f)
	case *collect.Select:
		return r.value(f)
	}
	return "(" + r.value(f) + ")"
}

func (r *rend) composite(e *collect.Composite) string {
	if e.Type == nil {
		return r.fail("literal without a type")
	}
	t := r.typ(e.Type)
	for _, el := range e.Elts {
		if el.KeyName != "" {
			// A keyed literal of a C++ class is assigned field by field in the generated
			// code (also for a non-aggregate like std::pair): the probe asks for the type only.
			for _, el := range e.Elts {
				r.value(el.Val) // the values are checked by their own references
			}
			return simpleType(t) + "{}"
		}
	}
	var parts []string
	for _, el := range e.Elts {
		switch {
		case el.Key != nil:
			parts = append(parts, "{"+r.value(el.Key)+", "+r.value(el.Val)+"}")
		default:
			parts = append(parts, r.value(el.Val))
		}
	}
	return t + "{" + strings.Join(parts, ", ") + "}"
}

// simpleType returns a spelling of the type t that can start a functional cast
// or a braced initialization: a name or a template-id as it is, any other type
// (long long, struct stat, char*) through std::type_identity_t.
func simpleType(t string) string {
	if simpleTypeRx.MatchString(t) {
		return t
	}
	return "std::type_identity_t<" + t + ">"
}

var simpleTypeRx = regexp.MustCompile(`^(::)?[A-Za-z_]\w*(<.*>)?(::[A-Za-z_]\w*(<.*>)?)*$`)

// typ renders e as a type.
func (r *rend) typ(e collect.Expr) string {
	switch e := e.(type) {
	case *collect.GoType:
		return e.Cxx
	case *collect.Stand:
		switch e.Name {
		case "string":
			return "gocpp_probe_internal_pr::string"
		case "slice":
			return "gocpp_probe_internal_pr::slice<" + r.typ(e.Args[0]) + ">"
		case "array":
			return "gocpp_probe_internal_pr::array<" + r.typ(e.Args[0]) + ", " + r.value(e.Args[1]) + ">"
		case "map":
			return "gocpp_probe_internal_pr::map<" + r.typ(e.Args[0]) + ", " + r.typ(e.Args[1]) + ">"
		}
		return r.fail("stand-in %s", e.Name)
	case *collect.Path:
		return r.pathText(e)
	case *collect.Inst:
		return r.typ(e.X) + "<" + r.targs(e.Args) + ">"
	case *collect.IndexOrInst:
		return r.typ(e.X) + "<" + r.targs(e.Args) + ">"
	case *collect.Star:
		return r.typ(e.X) + "*"
	case *collect.Paren:
		return r.typ(e.X)
	case *collect.Decay:
		switch e.X.(type) {
		case *collect.TupleElem, *collect.RangeElem:
			return "std::decay_t<" + r.typ(e.X) + ">"
		}
		return "std::decay_t<decltype((" + r.value(e.X) + "))>"
	case *collect.DeclRef:
		return r.alias(e.Def)
	case *collect.TupleElem:
		return "std::tuple_element_t<" + strconv.Itoa(e.I) + ", std::remove_cvref_t<decltype((" + r.value(e.X) + "))>>"
	case *collect.RangeElem:
		which := "val"
		if e.Key {
			which = "key"
		}
		return "typename gocpp_probe_internal_pr::range_t<std::remove_cvref_t<decltype((" + r.value(e.X) + "))>>::" + which
	case *collect.Select:
		return "typename " + r.typ(e.X) + "::" + e.Name
	case *collect.Unknown:
		return r.fail("%s", e.Why)
	}
	return r.fail("%s used as a type", e.String())
}

// alias returns the name of the alias of the local variable whose reference
// is def, and queues its line.
func (r *rend) alias(def *collect.Ref) string {
	name := aliasName(def)
	if r.g.aliases[name] {
		r.used = append(r.used, name)
		return name
	}
	for _, l := range r.pre {
		if aliasNameOf(l.Text) == name {
			r.used = append(r.used, name)
			return name
		}
	}
	sub := &rend{kinds: r.kinds, g: r.g, flip: r.flip}
	text := sub.typ(def.Expr)
	if sub.reason != "" {
		return r.fail("local variable: %s", sub.reason)
	}
	r.ambiguous = r.ambiguous || sub.ambiguous
	r.pre = append(r.pre, sub.pre...)
	r.pre = append(r.pre, Line{Ref: def.ID, Role: RoleAlias, Text: "using " + name + " = " + text + ";", Needs: sub.used})
	r.used = append(r.used, sub.used...)
	r.used = append(r.used, name)
	return name
}

// aliasNameOf returns the name that an alias line declares.
func aliasNameOf(line string) string {
	s := strings.TrimPrefix(line, "using ")
	name, _, _ := strings.Cut(s, " =")
	return name
}

// FuncNames returns the qualified C++ names of the functions that the
// references call (the Fun of a call that is a path whose kind is a value:
// "fopen", "std::min", "geo::f"), sorted and without repeats. A function
// template called with explicit arguments (std::max[float64]) is named by its
// template. These are the names for which step 12.3 extracts overload sets.
func FuncNames(set *collect.Set, kinds map[string]interop.Kind) []string {
	r := &rend{kinds: kinds}
	seen := map[string]bool{}
	var out []string
	add := func(p *collect.Path) {
		if p == nil || p.Ns == "C" && len(p.Parts) == 0 {
			return
		}
		e := r.eff(p)
		if e.Ns == "C" {
			return // cgo's own names (C.int, C.CString)
		}
		if k, ok := r.kind(e); !ok || (k != interop.Value && k != interop.ValueTemplate) {
			return
		}
		// A static member function (std::thread::hardware_concurrency) is in its class's
		// declaration, not in a namespace's overload set.
		if n := len(e.Parts); n > 1 {
			if pk, ok := r.prefixKind(e, n-1); !ok || pk != interop.Namespace {
				return
			}
		}
		name := strings.Join(e.Parts, "::")
		if e.Ns != "" {
			name = strings.TrimPrefix(e.Ns, "::") + "::" + name
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, ref := range set.Refs {
		collect.Walk(ref.Expr, func(e collect.Expr) bool {
			if c, ok := e.(*collect.Call); ok {
				switch f := c.Fun.(type) {
				case *collect.Path:
					add(f)
				case *collect.Inst:
					if p, ok := f.X.(*collect.Path); ok {
						add(p)
					}
				case *collect.IndexOrInst:
					if p, ok := f.X.(*collect.Path); ok {
						add(p)
					}
				}
			}
			return true
		})
	}
	sort.Strings(out)
	return out
}
