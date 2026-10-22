// Package ftype is step 12.4 of typed interop (Phase 12): it maps the C++
// types that clang reports to the types of gocpp's "foreign" view, which are
// go/types types.
//
// Stock go/types knows nothing about C and C++: every expression that touches
// a C value is invalid for it (internal/check/opaque.go). The interop checker
// of step 12.5 types those expressions with the probe results; this package
// says what type each result is. The mapping is the table of the Phase 12
// README (§2.3):
//
//	C++                              gocpp view (go/types)
//	int, long, size_t, double, ...   named types C.int, C.long, ... in package C (underlying: the Go
//	                                 basic type of the same size); size_t is C.ulong, as in cgo
//	T*                               *T; void* is unsafe.Pointer; a function pointer is a func type
//	T&, const T&                     T (Mapped.Ref says there was a reference; Value says addressable)
//	T[N]                             [N]T
//	struct/class/union               a named struct type; public fields are fields; public
//	                                 bases are embedded fields; methods are in Members
//	enum                             a named integer type, its enumerators are constants
//	std::vector<long>                a named type std.vector[C.long] (members from the declaration)
//	gp_pr::string/slice/array/map    Go's string, []T, [N]T, map[K]V (the stand-ins of the probe)
//
// Anything else is Opaque, with a reason: the interop checker leaves such a
// value to the C++ compiler, as opaque mode does. The mapping never fails and
// never guesses: a name that cannot be resolved, a pointer to member, a type
// with no name, a 128-bit integer are opaque.
//
// A Mapper is not safe for concurrent use.
package ftype

import (
	"fmt"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// A Kind says what a mapped type is.
type Kind uint8

const (
	Opaque  Kind = iota // no gocpp type: the C++ compiler decides
	Void                // void: no value
	Scalar              // a C arithmetic type (C.int, C.double, C.bool)
	Go                  // a Go type (the stand-ins of gocpp's string, slice, array and map)
	Pointer             // T*
	FuncPtr             // R (*)(A...): a func type
	Func                // R (A...): a function (a name, not a pointer)
	Array               // T[N]
	Record              // a class, struct or union
	Enum                // an enumeration
	Nullptr             // std::nullptr_t
)

var kindNames = [...]string{"opaque", "void", "scalar", "go", "pointer", "funcptr", "func", "array", "record", "enum", "nullptr"}

func (k Kind) String() string { return kindNames[k] }

// A Mapped is a C++ type in the foreign view.
type Mapped struct {
	// Type is the go/types type: nil for Void, types.Typ[types.Invalid] for Opaque.
	Type types.Type
	Kind Kind

	// C is the C++ type this was made from.
	C *ctype.Type

	// Alias is the typedef name that the written type used for the type
	// ("size_t" for C.ulong), when the written type had one.
	Alias string

	// Elem is the mapped pointee of a Pointer, the element of an Array: it has the
	// Alias of the element ("FILE" for *_IO_FILE).
	Elem *Mapped

	// Reason says why the type is Opaque.
	Reason string

	// Ref is "&" or "&&" when the type was a reference: Type is the referent's.
	Ref string

	// Const is the top-level const of the C++ type (const int, char *const).
	Const bool

	// Incomplete says that the members are not known: a record that is only
	// declared, or one that the declaration extraction did not report.
	Incomplete bool

	// Scoped says an enum is an "enum class": it does not convert to integers
	// implicitly.
	Scoped bool

	// Flexible says an array had no length (char name[]).
	Flexible bool

	// Partial says that a function type has a parameter or result that is Opaque.
	Partial bool
}

// Display writes the type the way the program names it: with the typedef names
// it used (size_t, *FILE), and the foreign names otherwise.
func (m Mapped) Display() string {
	switch {
	case m.Kind == Opaque:
		return "opaque"
	case m.Alias != "":
		return m.Alias
	case m.Type == nil:
		return "void"
	case m.Elem != nil && m.Kind == Pointer && m.Type != types.Type(types.Typ[types.UnsafePointer]):
		return "*" + m.Elem.Display()
	case m.Elem != nil && m.Kind == Array:
		if a, ok := m.Type.(*types.Array); ok {
			return "[" + strconv.FormatInt(a.Len(), 10) + "]" + m.Elem.Display()
		}
	}
	return typeString(m.Type)
}

func typeString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p.Path() == "unsafe" {
			return "unsafe"
		}
		return ""
	})
}

// IsOpaque reports whether the type has no foreign view.
func (m Mapped) IsOpaque() bool { return m.Kind == Opaque }

// A Target describes the data model of the target the types are mapped for.
type Target struct {
	// LongBits, WcharBits and PtrBits are the sizes of long, wchar_t and a pointer (and so
	// of size_t, ptrdiff_t, intptr_t): 64, 32 and 64 on Linux (LP64); 32, 16 and 64 on
	// Windows (LLP64, where size_t is unsigned long long and int64_t is long long); 32,
	// 32 and 32 on a 32-bit target. A zero PtrBits is 64, a zero WcharBits is 32.
	LongBits  int
	WcharBits int
	PtrBits   int

	// CharUnsigned says that plain char is unsigned (aarch64 Linux, -funsigned-char).
	CharUnsigned bool
}

// The data models of the targets of gocpp, and ILP32 for 32-bit ones.
var (
	LP64  = Target{LongBits: 64, WcharBits: 32, PtrBits: 64}
	LLP64 = Target{LongBits: 32, WcharBits: 16, PtrBits: 64}
	ILP32 = Target{LongBits: 32, WcharBits: 32, PtrBits: 32}
)

// A Mapper maps C++ types, using the declarations of a probe.DeclTable.
type Mapper struct {
	// Pkg is the package of every foreign type (path and name "C").
	Pkg *types.Package

	target Target
	decls  *probe.DeclTable

	basic  map[string]*types.Named // C.int ...
	named  map[string]*record      // by canonical spelling
	sigs   map[*types.Signature]*SigInfo
	nameOf map[*types.Named]string // the canonical spelling of a named type
}

// New returns a mapper for the declarations d (which can be nil, and can grow:
// see Add).
func New(d *probe.DeclTable, t Target) *Mapper {
	if t.LongBits == 0 {
		t = LP64
	}
	if t.PtrBits == 0 {
		t.PtrBits = 64
	}
	if t.WcharBits == 0 {
		t.WcharBits = 32
	}
	m := &Mapper{
		Pkg:    types.NewPackage("C", "C"),
		target: t,
		decls:  &probe.DeclTable{},
		basic:  make(map[string]*types.Named),
		named:  make(map[string]*record),
		sigs:   make(map[*types.Signature]*SigInfo),
		nameOf: make(map[*types.Named]string),
	}
	if d != nil {
		m.Add(d)
	}
	return m
}

// Add merges declarations into the mapper's table. Types made before an Add
// keep what they were (a record that was Incomplete stays so): add all the
// declarations before mapping, or use a new Mapper.
func (m *Mapper) Add(d *probe.DeclTable) {
	t := m.decls
	if t.Records == nil {
		t.Records, t.Enums, t.Funcs, t.Canon, t.Errors = map[string]*probe.Record{}, map[string]*probe.Enum{}, map[string]*probe.FuncSet{}, map[string]string{}, map[string]string{}
	}
	for k, v := range d.Records {
		t.Records[k] = v
	}
	for k, v := range d.Enums {
		t.Enums[k] = v
	}
	for k, v := range d.Funcs {
		t.Funcs[k] = v
	}
	for k, v := range d.Canon {
		t.Canon[k] = v
	}
	for k, v := range d.Errors {
		t.Errors[k] = v
	}
}

// Target returns the data model the mapper maps for.
func (m *Mapper) Target() Target { return m.target }

// Decls returns the declarations the mapper has.
func (m *Mapper) Decls() *probe.DeclTable { return m.decls }

// ---------------------------------------------------------------- builtins

type basicInfo struct {
	goName string
	kind   func(t Target) types.BasicKind
}

func fixed(k types.BasicKind) func(Target) types.BasicKind {
	return func(Target) types.BasicKind { return k }
}

func longKind(signed bool) func(Target) types.BasicKind {
	return func(t Target) types.BasicKind {
		switch {
		case t.LongBits == 32 && signed:
			return types.Int32
		case t.LongBits == 32:
			return types.Uint32
		case signed:
			return types.Int64
		}
		return types.Uint64
	}
}

func charKind(t Target) types.BasicKind {
	if t.CharUnsigned {
		return types.Uint8
	}
	return types.Int8
}

func wcharKind(t Target) types.BasicKind {
	if t.WcharBits == 16 {
		return types.Uint16
	}
	return types.Int32
}

var builtins = map[string]basicInfo{
	"char": {"char", charKind}, "signed char": {"schar", fixed(types.Int8)}, "unsigned char": {"uchar", fixed(types.Uint8)},
	"short": {"short", fixed(types.Int16)}, "unsigned short": {"ushort", fixed(types.Uint16)},
	"int": {"int", fixed(types.Int32)}, "unsigned int": {"uint", fixed(types.Uint32)},
	"long": {"long", longKind(true)}, "unsigned long": {"ulong", longKind(false)},
	"long long": {"longlong", fixed(types.Int64)}, "unsigned long long": {"ulonglong", fixed(types.Uint64)},
	"float": {"float", fixed(types.Float32)}, "double": {"double", fixed(types.Float64)},
	// long double has no Go type: it is a float64 here, and Mapped.C keeps the truth.
	"long double": {"longdouble", fixed(types.Float64)},
	"bool":        {"bool", fixed(types.Bool)},
	"wchar_t":     {"wchar_t", wcharKind},
	"char8_t":     {"char8_t", fixed(types.Uint8)}, "char16_t": {"char16_t", fixed(types.Uint16)}, "char32_t": {"char32_t", fixed(types.Uint32)},
}

// typedefs are the typedef names of the C library that gocpp names like cgo
// (C.size_t): they are the same type as the builtin they stand for, which depends on
// the target (typedefBase). The value is the class of the name.
var typedefs = map[string]string{
	"size_t": "usize", "uintptr_t": "usize", "ssize_t": "ssize", "ptrdiff_t": "ssize", "intptr_t": "ssize",
	"int64_t": "i64", "uint64_t": "u64",
	"int8_t": "signed char", "uint8_t": "unsigned char", "int16_t": "short", "uint16_t": "unsigned short",
	"int32_t": "int", "uint32_t": "unsigned int",
}

// typedefBase returns the builtin type that a typedef name of the C library stands for on
// the mapper's target: size_t is unsigned long on Linux, unsigned long long on Windows,
// unsigned int on a 32-bit target.
func (m *Mapper) typedefBase(name string) (string, bool) {
	class, ok := typedefs[strings.TrimPrefix(name, "std::")]
	if !ok {
		return "", false
	}
	t := m.target
	switch class {
	case "usize":
		switch {
		case t.PtrBits == 32:
			return "unsigned int", true
		case t.LongBits == 64:
			return "unsigned long", true
		}
		return "unsigned long long", true
	case "ssize":
		switch {
		case t.PtrBits == 32:
			return "int", true
		case t.LongBits == 64:
			return "long", true
		}
		return "long long", true
	case "i64":
		if t.LongBits == 64 {
			return "long", true
		}
		return "long long", true
	case "u64":
		if t.LongBits == 64 {
			return "unsigned long", true
		}
		return "unsigned long long", true
	}
	return class, true
}

// isTypedefName reports whether the name is one of the C library typedefs that need no lookup.
func isTypedefName(name string) bool {
	_, ok := typedefs[strings.TrimPrefix(name, "std::")]
	return ok
}

func (m *Mapper) basicType(name string) (*types.Named, bool) {
	if n, ok := m.basic[name]; ok {
		return n, true
	}
	b, ok := builtins[name]
	if !ok {
		return nil, false
	}
	tn := types.NewTypeName(token.NoPos, m.Pkg, "C."+b.goName, nil)
	n := types.NewNamed(tn, types.Typ[b.kind(m.target)], nil)
	m.basic[name] = n
	m.nameOf[n] = name
	return n, true
}

// ---------------------------------------------------------------- mapping

const maxDepth = 40

// Map maps the canonical C++ type c. written is the type with its typedef
// names (Result.Written parsed), or nil; it only supplies the Alias of a leaf.
// The type must not be relative to a scope (use MapIn for the member types
// of a declaration).
func (m *Mapper) Map(c, written *ctype.Type) Mapped { return m.mapT(c, written, "", 0) }

// MapIn maps a type written relative to the scope (a namespace or class:
// "geo"), as the members of a declaration are.
func (m *Mapper) MapIn(c *ctype.Type, scope string) Mapped { return m.mapT(c, nil, scope, 0) }

func opaque(c *ctype.Type, format string, a ...any) Mapped {
	return Mapped{Type: types.Typ[types.Invalid], Kind: Opaque, C: c, Reason: fmt.Sprintf(format, a...)}
}

func (m *Mapper) mapT(c, w *ctype.Type, scope string, depth int) Mapped {
	if c == nil {
		return opaque(nil, "no type")
	}
	if depth > maxDepth {
		return opaque(c, "the type is nested too deeply")
	}
	// The written type only helps while it has the same shape.
	sub := func(k ctype.Kind) *ctype.Type {
		if w != nil && w.Kind == k && w.Elem != nil {
			return w.Elem
		}
		return nil
	}
	switch c.Kind {
	case ctype.Builtin:
		return m.mapBuiltin(c, w)
	case ctype.Pointer:
		e := m.mapT(c.Elem, sub(ctype.Pointer), scope, depth+1)
		out := Mapped{C: c, Const: c.Const}
		switch {
		case e.Kind == Opaque:
			return opaque(c, "pointer to %s", e.Reason)
		case e.Kind == Void:
			out.Kind, out.Type = Pointer, types.Typ[types.UnsafePointer]
		case e.Kind == Func:
			out.Kind, out.Type, out.Partial = FuncPtr, e.Type, e.Partial
		default:
			out.Kind, out.Type = Pointer, types.NewPointer(e.Type)
		}
		out.Elem = &e
		return out
	case ctype.LRef, ctype.RRef:
		e := m.mapT(c.Elem, sub(c.Kind), scope, depth+1)
		if e.Kind == Opaque {
			return e
		}
		e.Ref = "&"
		if c.Kind == ctype.RRef {
			e.Ref = "&&"
		}
		return e
	case ctype.Array:
		e := m.mapT(c.Elem, sub(ctype.Array), scope, depth+1)
		if e.Kind == Opaque {
			return opaque(c, "array of %s", e.Reason)
		}
		if e.Kind == Void || e.Kind == Func {
			return opaque(c, "array of void or functions")
		}
		if c.Len < 0 {
			return Mapped{Type: types.NewArray(e.Type, 0), Kind: Array, C: c, Flexible: true, Const: c.Const, Elem: &e}
		}
		return Mapped{Type: types.NewArray(e.Type, c.Len), Kind: Array, C: c, Const: c.Const, Elem: &e}
	case ctype.Func:
		return m.mapFunc(c, scope, depth)
	case ctype.Named:
		return m.mapNamed(c, w, scope, depth)
	case ctype.MemberPtr:
		return opaque(c, "pointer to member")
	case ctype.Unnamed:
		return opaque(c, "a type with no name (%s)", c.Name)
	case ctype.Special:
		return opaque(c, "%s", c.Name)
	}
	return opaque(c, "unknown kind of type")
}

func (m *Mapper) mapBuiltin(c, w *ctype.Type) Mapped {
	if c.Name == "void" {
		return Mapped{Kind: Void, C: c, Const: c.Const}
	}
	n, ok := m.basicType(c.Name)
	if !ok {
		return opaque(c, "the type %s has no gocpp view", c.Name)
	}
	out := Mapped{Type: n, Kind: Scalar, C: c, Const: c.Const}
	if w != nil && w.Kind == ctype.Named && len(w.Path) > 0 && !w.Path[len(w.Path)-1].HasArgs {
		if base, ok := m.typedefBase(w.QualName()); ok && base == c.Name {
			out.Alias = w.QualName()
		}
	}
	return out
}

// goName writes a C++ name the Go way: std::vector<long> is std.vector[C.long].
func (m *Mapper) goName(c *ctype.Type) string {
	switch c.Kind {
	case ctype.Builtin:
		if b, ok := builtins[c.Name]; ok {
			return "C." + b.goName
		}
		return c.Name
	case ctype.Pointer:
		return "*" + m.goName(c.Elem)
	case ctype.LRef, ctype.RRef:
		return m.goName(c.Elem)
	case ctype.Array:
		if c.Len >= 0 {
			return "[" + strconv.FormatInt(c.Len, 10) + "]" + m.goName(c.Elem)
		}
		return "[]" + m.goName(c.Elem)
	case ctype.Named:
		var b strings.Builder
		for i, s := range c.Path {
			if i > 0 {
				b.WriteByte('.')
			}
			b.WriteString(s.Name)
			if s.HasArgs {
				b.WriteByte('[')
				for j, a := range s.Args {
					if j > 0 {
						b.WriteString(", ")
					}
					if a.Type != nil {
						b.WriteString(m.goName(a.Type))
					} else {
						b.WriteString(a.Expr)
					}
				}
				b.WriteByte(']')
			}
		}
		return b.String()
	}
	return c.String()
}

func (m *Mapper) mapNamed(c, w *ctype.Type, scope string, depth int) Mapped {
	// Stand-ins: Go's own types.
	if len(c.Path) == 2 && c.Path[0].Name == "gocpp_probe_internal_pr" {
		return m.mapStandIn(c, scope, depth)
	}
	if c.Tag == "typename" || len(c.Path) > 0 && strings.HasPrefix(c.Path[0].Name, "type-parameter") {
		return opaque(c, "a dependent type")
	}
	if scope != "" {
		q, ok := m.qualify(c, scope)
		if !ok {
			return opaque(c, "the name %s is not resolved%s", c.String(), inScope(scope))
		}
		c, scope = q, ""
	}
	key := nameKey(c)
	// A typedef name: the canonical type, with the typedef as the alias of a builtin.
	if canon, isTypedef := m.decls.Canon[key]; isTypedef && canon != key {
		ct, err := ctype.Parse(canon)
		if err != nil {
			return opaque(c, "cannot read the type %s", canon)
		}
		ct.Const, ct.Volatile = ct.Const || c.Const, ct.Volatile || c.Volatile
		out := m.mapT(ct, nil, "", depth+1)
		if out.Kind == Scalar && out.Alias == "" {
			out.Alias = key
		}
		return out
	}
	kc, err := ctype.Parse(key)
	if err != nil {
		return opaque(c, "cannot read the type %s", key)
	}
	if base, ok := m.typedefBase(key); ok { // size_t when no declaration was asked for
		bt, _ := m.basicType(base)
		return Mapped{Type: bt, Kind: Scalar, C: c, Alias: key, Const: c.Const}
	}
	if key == "std::nullptr_t" {
		tn := types.NewTypeName(token.NoPos, m.Pkg, "C.nullptr_t", nil)
		return Mapped{Type: types.NewNamed(tn, types.Typ[types.UntypedNil], nil), Kind: Nullptr, C: c}
	}
	r := m.namedRecord(key, kc, depth)
	out := Mapped{Type: r.t, Kind: r.kind, C: c, Const: c.Const, Incomplete: r.incomplete, Scoped: r.enum != nil && r.enum.Scoped}
	if w != nil && w.Kind == ctype.Named {
		// The written name is an alias only when the declarations say it is a typedef
		// of this type (FILE for _IO_FILE), not a member typedef (value_type).
		if wn := w.QualName(); wn != "" && wn != key && len(w.Path) > 0 && !w.Path[len(w.Path)-1].HasArgs && m.decls.Canon[wn] == key {
			out.Alias = wn
		}
	}
	return out
}

func inScope(scope string) string {
	if scope == "" {
		return ""
	}
	return " in " + scope
}

// nameKey is the spelling of a name without qualifiers: the key of the declarations.
func nameKey(c *ctype.Type) string {
	return strings.TrimPrefix((&ctype.Type{Kind: ctype.Named, Path: c.Path, Global: c.Global}).String(), "::")
}

// stdAliases are the standard library's names for instances of its templates
// (std::string is std::basic_string<char>): they are written in headers, and
// the compiler's canonical type never has them.
var stdAliases = map[string]string{
	"std::string": "std::basic_string<char>", "std::wstring": "std::basic_string<wchar_t>",
	"std::u8string": "std::basic_string<char8_t>", "std::u16string": "std::basic_string<char16_t>", "std::u32string": "std::basic_string<char32_t>",
	"std::string_view": "std::basic_string_view<char>", "std::wstring_view": "std::basic_string_view<wchar_t>",
	"std::ostream": "std::basic_ostream<char>", "std::istream": "std::basic_istream<char>", "std::iostream": "std::basic_iostream<char>",
	"std::stringstream": "std::basic_stringstream<char>", "std::ostringstream": "std::basic_ostringstream<char>",
	"std::istringstream": "std::basic_istringstream<char>", "std::ifstream": "std::basic_ifstream<char>",
	"std::ofstream": "std::basic_ofstream<char>", "std::fstream": "std::basic_fstream<char>",
}

// qualifyArgs returns n (a name) with the types among its template arguments
// qualified (see qualify); false when one of them cannot be.
func (m *Mapper) qualifyArgs(n *ctype.Type, scope string) (*ctype.Type, bool) {
	c := n.Clone()
	for i := range c.Path {
		for j := range c.Path[i].Args {
			if a := c.Path[i].Args[j]; a.Type != nil {
				q, ok := m.qualify(a.Type, scope)
				if !ok {
					return nil, false
				}
				c.Path[i].Args[j].Type = q
			}
		}
	}
	return c, true
}

// qualify turns a type written relative to a scope (a namespace such as
// "oop", or "a::b") into the canonical spelling: every name in it, also in the
// template arguments, is looked up from the innermost scope outwards among
// the names the declarations know. Names of the standard library (std::) and of
// the C typedef table need no lookup. It returns false when a name is not known,
// which the declaration extraction can fix (NeedsMore asks for the candidates).
func (m *Mapper) qualify(t *ctype.Type, scope string) (*ctype.Type, bool) {
	if t == nil {
		return nil, false
	}
	if scope == "" {
		return t, true
	}
	switch t.Kind {
	case ctype.Named:
		if len(t.Path) > 0 && t.Path[0].Name == "gocpp_probe_internal_pr" {
			return m.qualifyArgs(t, scope)
		}
		c, ok := m.qualifyArgs(t, scope)
		if !ok {
			return nil, false
		}
		own := nameKey(c)
		var resolved string
		switch {
		case c.Global || len(c.Path) > 0 && c.Path[0].Name == "std":
			resolved = own
		default:
			if isTypedefName(own) {
				resolved = own
				break
			}
			parts := strings.Split(scope, "::")
			for i := len(parts); i >= 0; i-- {
				cand := strings.Join(append(append([]string{}, parts[:i]...), own), "::")
				if m.known(cand) {
					resolved = cand
					break
				}
			}
		}
		if resolved == "" {
			return nil, false
		}
		if alias, ok := stdAliases[resolved]; ok {
			resolved = alias
		}
		r, err := ctype.Parse(resolved)
		if err != nil {
			return nil, false
		}
		r.Const, r.Volatile, r.Restrict, r.Pack, r.Tag = t.Const, t.Volatile, t.Restrict, t.Pack, ""
		return r, true
	case ctype.Pointer, ctype.LRef, ctype.RRef, ctype.Array, ctype.MemberPtr:
		e, ok := m.qualify(t.Elem, scope)
		if !ok {
			return nil, false
		}
		c := *t
		c.Elem = e
		if t.Kind == ctype.MemberPtr {
			cl, ok := m.qualify(t.Class, scope)
			if !ok {
				return nil, false
			}
			c.Class = cl
		}
		return &c, true
	case ctype.Func:
		c := *t
		e, ok := m.qualify(t.Elem, scope)
		if !ok {
			return nil, false
		}
		c.Elem = e
		c.Params = nil
		for _, p := range t.Params {
			q, ok := m.qualify(p, scope)
			if !ok {
				return nil, false
			}
			c.Params = append(c.Params, q)
		}
		return &c, true
	}
	return t, true
}

// known reports whether the declarations say anything about the name.
func (m *Mapper) known(name string) bool {
	if _, ok := m.decls.Canon[name]; ok {
		return true
	}
	if _, ok := m.decls.Records[name]; ok {
		return true
	}
	_, ok := m.decls.Enums[name]
	return ok
}

func (m *Mapper) mapStandIn(c *ctype.Type, scope string, depth int) Mapped {
	args := c.Args()
	arg := func(i int) (Mapped, bool) {
		if i >= len(args) || args[i].Type == nil {
			return Mapped{}, false
		}
		e := m.mapT(args[i].Type, nil, scope, depth+1)
		return e, e.Kind != Opaque && e.Kind != Void && e.Kind != Func
	}
	switch c.Last() {
	case "string":
		return Mapped{Type: types.Typ[types.String], Kind: Go, C: c, Const: c.Const}
	case "slice":
		if e, ok := arg(0); ok {
			return Mapped{Type: types.NewSlice(e.Type), Kind: Go, C: c, Const: c.Const}
		}
	case "array":
		if e, ok := arg(0); ok && len(args) == 2 && args[1].Type == nil {
			if n, err := strconv.ParseInt(strings.TrimRight(args[1].Expr, "uUlL"), 10, 64); err == nil && n >= 0 {
				return Mapped{Type: types.NewArray(e.Type, n), Kind: Go, C: c, Const: c.Const}
			}
		}
	case "map":
		k, ok1 := arg(0)
		v, ok2 := arg(1)
		if ok1 && ok2 && types.Comparable(k.Type) {
			return Mapped{Type: types.NewMap(k.Type, v.Type), Kind: Go, C: c, Const: c.Const}
		}
	}
	return opaque(c, "a stand-in %s whose parts have no gocpp view", c.String())
}

// ---------------------------------------------------------------- functions

// A ParamInfo says what a parameter of a mapped function was in C++.
type ParamInfo struct {
	Mapped Mapped
	// Ref is "&" or "&&" for a reference parameter (the Go type is the referent's);
	// Const is the const of the referent (const T&).
	Ref   string
	Const bool
}

// A SigInfo is what a Go signature cannot say about a C++ function type.
type SigInfo struct {
	Params []ParamInfo
	Result Mapped
	// Variadic says the C++ function ends in "...": the Go signature does not have it.
	Variadic bool
	// ConstQ, RefQual and Noexcept are the qualifiers of a member function.
	ConstQ   bool
	RefQual  string
	Noexcept bool
	Attr     string
}

// Sig returns the C++ details of a signature made by the mapper.
func (m *Mapper) Sig(s *types.Signature) *SigInfo { return m.sigs[s] }

func (m *Mapper) mapFunc(c *ctype.Type, scope string, depth int) Mapped {
	info := &SigInfo{Variadic: c.Variadic, ConstQ: c.ConstQ, RefQual: c.RefQual, Noexcept: strings.HasPrefix(c.Noexcept, "noexcept") && c.Noexcept != "noexcept(false)", Attr: c.Attr}
	partial := false
	var ps []*types.Var
	for _, p := range c.Params {
		pm := m.mapT(p, nil, scope, depth+1)
		pi := ParamInfo{Mapped: pm, Ref: pm.Ref, Const: pm.Const}
		if pm.Kind == Opaque || pm.Kind == Void {
			partial = true
		}
		// A reference parameter's const is the referent's: Mapped.Const already says it.
		info.Params = append(info.Params, pi)
		t := pm.Type
		if t == nil {
			t = types.Typ[types.Invalid]
		}
		ps = append(ps, types.NewParam(token.NoPos, m.Pkg, "", t))
	}
	res := m.mapT(c.Elem, nil, scope, depth+1)
	info.Result = res
	var rs []*types.Var
	switch res.Kind {
	case Void:
	case Opaque:
		partial = true
		rs = append(rs, types.NewParam(token.NoPos, m.Pkg, "", types.Typ[types.Invalid]))
	default:
		rs = append(rs, types.NewParam(token.NoPos, m.Pkg, "", res.Type))
	}
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(ps...), types.NewTuple(rs...), false)
	m.sigs[sig] = info
	return Mapped{Type: sig, Kind: Func, C: c, Partial: partial}
}

// TypeString writes a foreign type for messages: C.int, *C.char, std.vector[C.long].
func (m *Mapper) TypeString(t types.Type) string { return typeString(t) }
