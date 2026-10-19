// Package ctype is the structured form of a C++ type for typed interop
// (Phase 12, step 12.3).
//
// Clang tells the probe the type of a C/C++ expression as text
// ("std::map<std::string, int (*)(const char *)>", "int (&)[4]"). Parse reads
// that text, in the spelling clang prints it (clang's printing policy), into a
// Type; String writes it back. For the text that clang prints, String(Parse(s))
// is s again (the tests check it over everything the zoo's headers print),
// and for any other text it is a normal form: Parse(String(t)) equals t.
//
// A Type is a tree of declarator parts (pointer, reference, array, function,
// pointer to member) over a leaf (a builtin type, a possibly qualified and
// templated name, a type with no name, or a special word such as
// "<overloaded function type>"). Qualifiers sit on the node they qualify:
// "char *const" is a Pointer with Const, "const char *" a Pointer to a Char
// with Const.
//
// What Parse does not do: it does not know what a name is (a class, an enum,
// a typedef, a template parameter) or look anything up; that is the job of the
// declaration extraction (probe.Decls). A text it cannot read (a type with an
// attribute, a vector type, a dependent expression it does not understand)
// gives an error, and the caller falls back to the opaque path.
package ctype

import (
	"fmt"
	"strings"
)

// A Kind says which part of a declarator or leaf a Type is.
type Kind uint8

const (
	Invalid   Kind = iota
	Builtin        // void, bool, char, unsigned long, double, __int128, ...: Name
	Named          // a name: Path, with Tag when the text had struct/class/union/enum/typename
	Unnamed        // (unnamed struct), (unnamed union), (unnamed enum), (lambda), (anonymous ...): Name is the text inside the parentheses
	Special        // <overloaded function type>, <bound member function type>, <dependent type>: Name is the whole text
	Pointer        // Elem *
	LRef           // Elem &
	RRef           // Elem &&
	Array          // Elem[Len]
	Func           // Result (Params)
	MemberPtr      // Elem Class::*
)

// A Seg is one part of a qualified name: "std", "vector<long>" (Args set),
// "iterator".
type Seg struct {
	Name string
	// HasArgs says the part had template arguments, also when there are none
	// ("S<>").
	HasArgs bool
	Args    []Arg
}

// An Arg is a template argument: a type, or the text of an expression (a
// value: "5", "true", "-1", "(1 << 3)", "&S::member"), or a pack expansion.
type Arg struct {
	Type *Type
	// Expr is the text of a non-type argument, normalized (white space around
	// the operators is the one clang prints). Empty when Type is set.
	Expr string
	// Pack says the argument ends with "..." (a pack expansion).
	Pack bool
}

// A Type is a parsed C++ type. Its fields are meaningful by Kind; the zero
// value is Invalid.
type Type struct {
	Kind Kind

	// Name is the builtin type (Builtin) in its canonical word order
	// ("unsigned long long"), the text of an Unnamed or Special type.
	Name string

	// Path is a Named type: the parts of its qualified name. Global says that
	// it started with "::".
	Path   []Seg
	Global bool

	// Tag is the elaborated keyword of a name: "struct", "class", "union",
	// "enum", "typename" or "".
	Tag string

	// Elem is the pointee, referent or element of a declarator, the result of
	// a Func, or the type of the member for MemberPtr.
	Elem *Type

	// Class is the class of a MemberPtr.
	Class *Type

	// Len is the length of an Array; Len < 0 means no length ("int[]"), in
	// which case LenExpr may hold a length that is not a number ("N",
	// "sizeof(T)").
	Len     int64
	LenExpr string

	// Params and Variadic are a Func's (its result is Elem).
	Params   []*Type
	Variadic bool

	// Qualifiers of this node. Restrict is __restrict.
	Const, Volatile, Restrict bool

	// Pack says the type ends with "..." (a pack expansion: "Args &&...").
	Pack bool

	// Attr is the text of the attributes after the parameter list of a function
	// type ("__attribute__((noreturn))").
	Attr string

	// Trailing says a function type is written "auto (...) -> Result".
	Trailing bool

	// Function qualifiers: ConstQ and VolatileQ are the "const" and
	// "volatile" after the parameter list of a method type, RefQual is "&"
	// or "&&", Noexcept is "" (none), "noexcept", or the text inside
	// "noexcept(...)" prefixed with "noexcept(". The text is kept as
	// clang prints it ("noexcept(true)").
	ConstQ, VolatileQ bool
	RefQual           string
	Noexcept          string
}

// Equal reports whether a and b are the same type (same shape, names,
// arguments and qualifiers).
func Equal(a, b *Type) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind || a.Name != b.Name || a.Global != b.Global || a.Tag != b.Tag ||
		a.Len != b.Len || a.LenExpr != b.LenExpr || a.Variadic != b.Variadic ||
		a.Pack != b.Pack || a.Attr != b.Attr || a.Trailing != b.Trailing || a.Const != b.Const || a.Volatile != b.Volatile || a.Restrict != b.Restrict ||
		a.ConstQ != b.ConstQ || a.VolatileQ != b.VolatileQ || a.RefQual != b.RefQual || a.Noexcept != b.Noexcept {
		return false
	}
	if len(a.Path) != len(b.Path) || len(a.Params) != len(b.Params) {
		return false
	}
	for i := range a.Path {
		x, y := a.Path[i], b.Path[i]
		if x.Name != y.Name || x.HasArgs != y.HasArgs || len(x.Args) != len(y.Args) {
			return false
		}
		for j := range x.Args {
			p, q := x.Args[j], y.Args[j]
			if p.Expr != q.Expr || p.Pack != q.Pack || !Equal(p.Type, q.Type) {
				return false
			}
		}
	}
	for i := range a.Params {
		if !Equal(a.Params[i], b.Params[i]) {
			return false
		}
	}
	return Equal(a.Elem, b.Elem) && Equal(a.Class, b.Class)
}

// Clone returns a deep copy of t.
func (t *Type) Clone() *Type {
	if t == nil {
		return nil
	}
	c := *t
	c.Path = nil
	for _, s := range t.Path {
		n := Seg{Name: s.Name, HasArgs: s.HasArgs}
		for _, a := range s.Args {
			n.Args = append(n.Args, Arg{Type: a.Type.Clone(), Expr: a.Expr, Pack: a.Pack})
		}
		c.Path = append(c.Path, n)
	}
	c.Params = nil
	for _, p := range t.Params {
		c.Params = append(c.Params, p.Clone())
	}
	c.Elem = t.Elem.Clone()
	c.Class = t.Class.Clone()
	return &c
}

// QualName returns the qualified name of a Named type without template
// arguments: "std::vector", "std::basic_string". The leading "::" is not
// included.
func (t *Type) QualName() string {
	if t == nil || t.Kind != Named {
		return ""
	}
	var parts []string
	for _, s := range t.Path {
		parts = append(parts, s.Name)
	}
	return strings.Join(parts, "::")
}

// Last returns the last part of a Named type's name: "vector".
func (t *Type) Last() string {
	if t == nil || t.Kind != Named || len(t.Path) == 0 {
		return ""
	}
	return t.Path[len(t.Path)-1].Name
}

// Args returns the template arguments of the last part of a Named type.
func (t *Type) Args() []Arg {
	if t == nil || t.Kind != Named || len(t.Path) == 0 {
		return nil
	}
	return t.Path[len(t.Path)-1].Args
}

// IsDeclarator reports whether t is a pointer, reference, array, function or
// pointer to member.
func (t *Type) IsDeclarator() bool {
	switch t.Kind {
	case Pointer, LRef, RRef, Array, Func, MemberPtr:
		return true
	}
	return false
}

// IsRef reports whether t is a reference type.
func (t *Type) IsRef() bool { return t != nil && (t.Kind == LRef || t.Kind == RRef) }

// StripRef returns the referent of a reference type, else t.
func (t *Type) StripRef() *Type {
	if t.IsRef() {
		return t.Elem
	}
	return t
}

// Leaf follows pointers, references, arrays and pointers to members down to
// the leaf (not into function types).
func (t *Type) Leaf() *Type {
	for t != nil {
		switch t.Kind {
		case Pointer, LRef, RRef, Array, MemberPtr:
			t = t.Elem
		default:
			return t
		}
	}
	return nil
}

// Walk calls fn for t and every type inside it (elements, parameters,
// results, classes, the types among template arguments), parents first.
func (t *Type) Walk(fn func(*Type)) {
	if t == nil {
		return
	}
	fn(t)
	t.Elem.Walk(fn)
	t.Class.Walk(fn)
	for _, p := range t.Params {
		p.Walk(fn)
	}
	for _, s := range t.Path {
		for _, a := range s.Args {
			a.Type.Walk(fn)
		}
	}
}

// ParseError is the error of Parse.
type ParseError struct {
	Text string
	Pos  int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("cannot read the C++ type %q at %d: %s", e.Text, e.Pos, e.Msg)
}
