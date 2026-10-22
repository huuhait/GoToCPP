package adapt

import (
	"go/types"
	"strings"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// What each row makes of a parameter type (Row.param). A verdict of Adapt carries the detail the
// lowering needs in its Key; Ignore says that the parameter can never take the argument.

// ---------------------------------------------------------------- strings

// pointerToChar reports whether m is a pointer to the character type name (const or not).
func pointerToChar(m ftype.Mapped, names ...string) (name string, isConst, ok bool) {
	if m.Kind != ftype.Pointer || m.Elem == nil || m.Elem.Kind != ftype.Scalar {
		return "", false, false
	}
	if m.Elem.C == nil || m.Elem.C.Kind != ctype.Builtin {
		return "", false, false
	}
	for _, n := range names {
		if m.Elem.C.Name == n {
			return n, m.Elem.Const || m.Elem.C.Const, true
		}
	}
	return "", false, false
}

// stringNeverTakes reports whether a Go string can never convert to a parameter of the mapped
// type m: a number, a pointer of any kind, an array (a class might have a constructor, so those are
// the C++ compiler's).
func stringNeverTakes(m ftype.Mapped) bool {
	switch m.Kind {
	case ftype.Scalar, ftype.Pointer, ftype.FuncPtr, ftype.Array:
		return true
	}
	return false
}

func cstrParam(m *ftype.Mapper, c Class, p *ctype.Type, scope string) (Key, Verdict) {
	mapped := mapParam(m, p, scope)
	if _, isConst, ok := pointerToChar(mapped, "char"); ok {
		if isConst {
			return Key{Row: RowCStr}, Adapt
		}
		return Key{}, Ignore // a char*: the callee may write; the user passes C.CString
	}
	if stringNeverTakes(mapped) {
		return Key{}, Ignore
	}
	return Key{}, Other
}

func stdStringParam(m *ftype.Mapper, c Class, p *ctype.Type, scope string) (Key, Verdict) {
	switch ParamConv(p, scope) {
	case ConvStdString:
		return Key{Row: RowStdString}, Adapt
	case ConvNoStr:
		return Key{}, Ignore
	}
	return Key{}, Other
}

// wideParam: a const wchar_t*, char16_t* or char32_t* parameter takes the wide copy of a string. A
// string constant is a C literal already, so for it only the wide parameters are adapted, and a
// const char* is left alone (Other).
func wideParam(m *ftype.Mapper, c Class, p *ctype.Type, scope string) (Key, Verdict) {
	mapped := mapParam(m, p, scope)
	if name, isConst, ok := pointerToChar(mapped, "wchar_t", "char16_t", "char32_t"); ok {
		if isConst {
			return Key{Row: RowWide, Name: name}, Adapt
		}
		return Key{}, Ignore
	}
	if c == ClassStringConst && stringNeverTakes(mapped) {
		if _, _, ok := pointerToChar(mapped, "char"); !ok {
			return Key{}, Ignore // a literal reaches a const char* by itself: C++'s
		}
	}
	return Key{}, Other
}

// A Conv is the conversion of a Go string to a parameter.
type Conv int

const (
	ConvNone      Conv = iota
	ConvCStr           // gp::cstr(s).gp_ptr(): a const char* parameter, or a C variadic argument
	ConvStdString      // ::std::string(s)

	// ConvNoStr is a parameter that a gocpp string can never go to (a number, a pointer to
	// anything but const char, a wide string): an overload with it is no candidate for the string.
	ConvNoStr
)

// SigConvs returns the conversion of a Go string for each of the n arguments of a call of one of
// the overloads sigs.
func SigConvs(sigs []Sig, n int) []Conv {
	out := make([]Conv, n)
	for i := 0; i < n; i++ {
		var class Conv
		seen := false
		agree := true
		for _, s := range sigs {
			var pc Conv
			switch {
			case i < len(s.T.Params):
				pc = ParamConv(s.T.Params[i], s.Scope)
			case s.T.Variadic:
				pc = ConvCStr // a char* for %s
			default:
				continue // fewer parameters: not viable for this call
			}
			if pc == ConvNoStr {
				continue
			}
			if !seen {
				class, seen = pc, true
			} else if pc != class {
				agree = false
			}
		}
		if seen && agree {
			out[i] = class
		}
	}
	return out
}

// ParamConv returns the conversion for a parameter of type t: const char* is ConvCStr,
// std::string (by value, const& or &&) ConvStdString.
func ParamConv(t *ctype.Type, scope string) Conv {
	if t == nil {
		return ConvNone
	}
	switch t.Kind {
	case ctype.Pointer:
		if t.Elem != nil && t.Elem.Kind == ctype.Builtin && t.Elem.Name == "char" && t.Elem.Const {
			return ConvCStr
		}
		return ConvNoStr // a char*, an int*, a pointer to a class: no conversion from a gocpp string
	case ctype.Builtin, ctype.Array, ctype.MemberPtr:
		return ConvNoStr
	}
	switch t.Kind {
	case ctype.LRef:
		if t.Elem == nil || !t.Elem.Const {
			return ConvNone // std::string& takes an lvalue string: not a temporary
		}
		t = t.Elem
	case ctype.RRef:
		t = t.Elem
	}
	if t != nil && t.Kind == ctype.Named {
		inStd := scope == "std" || strings.HasPrefix(scope, "std::")
		switch {
		case isStdString(t, inStd):
			return ConvStdString
		case isWideString(t, inStd):
			return ConvNoStr
		}
	}
	return ConvNone
}

// isStdString reports whether the named type t is std::string (std::basic_string<char>, with or
// without the std:: written).
func isStdString(t *ctype.Type, inStd bool) bool {
	p := t.Path
	if len(p) > 0 && p[0].Name == "std" {
		p = p[1:]
	} else if !inStd {
		return false // a bare string is std's only inside std
	}
	if len(p) != 1 {
		return false
	}
	switch p[0].Name {
	case "string":
		return !p[0].HasArgs
	case "basic_string":
		return len(p[0].Args) >= 1 && p[0].Args[0].Type != nil && p[0].Args[0].Type.Kind == ctype.Builtin && p[0].Args[0].Type.Name == "char"
	}
	return false
}

// isWideString reports whether t is std::wstring, u16string, u32string or basic_string of a type
// other than char: a gocpp string converts to none of them.
func isWideString(t *ctype.Type, inStd bool) bool {
	p := t.Path
	if len(p) > 0 && p[0].Name == "std" {
		p = p[1:]
	} else if !inStd {
		return false
	}
	if len(p) != 1 {
		return false
	}
	switch p[0].Name {
	case "wstring", "u16string", "u32string", "u8string":
		return true
	case "basic_string":
		return len(p[0].Args) >= 1 && p[0].Args[0].Type != nil && !(p[0].Args[0].Type.Kind == ctype.Builtin && p[0].Args[0].Type.Name == "char")
	}
	return false
}

// ---------------------------------------------------------------- &x of a number

// arithmeticCNames are the C number types that the scalarptr row takes as a pointee. The character
// types are left out on purpose: a char* is nearly always a buffer or a string, and a pointer to
// one variable would let the callee write past it.
var arithmeticCNames = map[string]bool{
	"short": true, "unsigned short": true, "int": true, "unsigned int": true,
	"long": true, "unsigned long": true, "long long": true, "unsigned long long": true,
	"float": true, "double": true,
}

func scalarPtrParam(m *ftype.Mapper, c Class, p *ctype.Type, scope string) (Key, Verdict) {
	mapped := mapParam(m, p, scope)
	switch {
	case pointsToCNumber(mapped):
		key := Key{Row: RowScalarPtr, Name: mapped.Elem.C.Name}
		key.IsConst = mapped.Elem.Const || mapped.Elem.C.Const
		return key, Adapt
	case isNumberOrEnumValue(mapped):
		return Key{}, Ignore // a pointer does not convert to a number or an enum
	}
	return Key{}, Other
}

// pointsToCNumber reports whether m is a pointer to one of the C number types of the scalarptr row.
func pointsToCNumber(m ftype.Mapped) bool {
	if m.Kind != ftype.Pointer || m.Elem == nil || m.Elem.Kind != ftype.Scalar {
		return false
	}
	c := m.Elem.C
	return c != nil && c.Kind == ctype.Builtin && arithmeticCNames[c.Name]
}

// pointsToVoidPointer reports whether m is a void** that the callee may write (no const).
func pointsToVoidPointer(m ftype.Mapped) bool {
	if m.Kind != ftype.Pointer || m.Elem == nil || m.Elem.Kind != ftype.Pointer || m.Elem.Const {
		return false
	}
	inner := m.Elem.Elem
	return inner != nil && inner.Kind == ftype.Void && !inner.Const
}

// isNumberOrEnumValue reports whether m is a number (not a bool) or an enum taken by value: a
// pointer never converts to it.
func isNumberOrEnumValue(m ftype.Mapped) bool {
	if m.Kind == ftype.Scalar {
		return m.C == nil || m.C.Name != "bool"
	}
	return m.Kind == ftype.Enum && m.Ref == ""
}

func voidOutParam(m *ftype.Mapper, c Class, p *ctype.Type, scope string) (Key, Verdict) {
	mapped := mapParam(m, p, scope)
	switch {
	case pointsToVoidPointer(mapped):
		return Key{Row: RowVoidOut}, Adapt
	case isNumberOrEnumValue(mapped):
		return Key{}, Ignore
	}
	return Key{}, Other
}

// ---------------------------------------------------------------- numbers by value

func enumParam(m *ftype.Mapper, c Class, p *ctype.Type, scope string) (Key, Verdict) {
	mapped := mapParam(m, p, scope)
	if mapped.Kind == ftype.Enum && (mapped.Ref == "" || mapped.Const) {
		if canon := m.Canonical(mapped.Type); canon != "" {
			return Key{Row: RowEnum, Name: canon}, Adapt
		}
	}
	return Key{}, Other
}

// scalarKind returns the Go basic kind of the mapped by-value C number type m (not a bool, not a
// reference).
func scalarKind(m ftype.Mapped) (types.BasicKind, bool) {
	if m.Kind != ftype.Scalar || m.Ref != "" || m.Type == nil {
		return 0, false
	}
	b, ok := m.Type.Underlying().(*types.Basic)
	if !ok || b.Info()&types.IsNumeric == 0 {
		return 0, false
	}
	return b.Kind(), true
}

// numConstParam: an untyped constant goes to a number parameter if Go would allow it there: the
// value must be representable in the C type (300 for a char is an error, as cgo says, where C++
// would wrap it). The argument is not changed. A parameter of another kind (a pointer takes 0, a
// bool, a class) is not this row's.
func numConstParam(m *ftype.Mapper, c Class, p *ctype.Type, scope string) (Key, Verdict) {
	mapped := mapParam(m, p, scope)
	if kind, ok := scalarKind(mapped); ok {
		spelled := GoCName(kind, m.Target())
		if cb := mapped.C; cb != nil && cb.Kind == ctype.Builtin {
			spelled = cb.Name
		}
		return Key{Row: RowNumConst, Kind: kind, Name: spelled}, Adapt
	}
	return Key{}, Other
}

// floatConvParam: a Go float goes to an integer parameter (or a float64 to a float one) with Go's
// conversion, gp::cgo::as<T>, which is defined for every value (C++ makes a float out of the range
// of an integer undefined behavior). A float32 or float64 to a parameter of a float type that holds
// it is C++'s.
func floatConvParam(m *ftype.Mapper, c Class, p *ctype.Type, scope string) (Key, Verdict) {
	mapped := mapParam(m, p, scope)
	kind, ok := scalarKind(mapped)
	if !ok {
		return Key{}, Other
	}
	info := types.Typ[kind].Info()
	switch {
	case info&types.IsInteger != 0:
		return Key{Row: RowFloatConv, Name: GoCName(kind, m.Target())}, Adapt
	case kind == types.Float32 && c == ClassFloat64:
		return Key{Row: RowFloatConv, Name: "float"}, Adapt
	}
	return Key{}, Other
}
