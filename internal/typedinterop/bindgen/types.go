package bindgen

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
)

// builtinGo maps the builtin types of C++ (in ctype's canonical word order) to
// the types of gocpp's C: C.int, C.uchar and the like (docs/spec/interop.md,
// "What C holds"). The few that C does not name stay as C.* names (the check
// of gocpp does not look inside C), except the 128 bit integers, which are
// 16 bytes.
var builtinGo = map[string]string{
	"char":               "C.char",
	"signed char":        "C.schar",
	"unsigned char":      "C.uchar",
	"short":              "C.short",
	"unsigned short":     "C.ushort",
	"int":                "C.int",
	"unsigned int":       "C.uint",
	"long":               "C.long",
	"unsigned long":      "C.ulong",
	"long long":          "C.longlong",
	"unsigned long long": "C.ulonglong",
	"float":              "C.float",
	"double":             "C.double",
	"long double":        "C.longdouble",
	"bool":               "C.bool",
	"_Bool":              "C.bool",
	"wchar_t":            "C.wchar_t",
	"char8_t":            "C.char8_t",
	"char16_t":           "C.char16_t",
	"char32_t":           "C.char32_t",
	"__int128":           "[16]byte",
	"unsigned __int128":  "[16]byte",
	"_Float16":           "C.float16",
	"__fp16":             "C.float16",
	"__bf16":             "C.bfloat16",
	"__float128":         "[16]byte",
	"_Float128":          "[16]byte",
}

// standardGo maps names that every program knows from the C standard to Go
// types, wherever the header gets them from.
var standardGo = map[string]string{
	"size_t":    "C.size_t",
	"int8_t":    "int8",
	"int16_t":   "int16",
	"int32_t":   "int32",
	"int64_t":   "int64",
	"uint8_t":   "uint8",
	"uint16_t":  "uint16",
	"uint32_t":  "uint32",
	"uint64_t":  "uint64",
	"uintptr_t": "uintptr",
	// the types of <stdarg.h>
	"va_list":           "C.va_list",
	"__gnuc_va_list":    "C.va_list",
	"__builtin_va_list": "C.va_list",
	"__va_list_tag":     "C.va_list",
	"nullptr_t":         "unsafe.Pointer",
}

// goKeywords are the words that C names cannot take in the view, and the
// predeclared names that the generator itself uses.
var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true, "default": true,
	"defer": true, "else": true, "fallthrough": true, "for": true, "func": true, "go": true,
	"goto": true, "if": true, "import": true, "interface": true, "map": true, "package": true,
	"range": true, "return": true, "select": true, "struct": true, "switch": true, "type": true,
	"var": true, "init": true,
	// used by the output
	"any": true, "int8": true, "int16": true, "int32": true, "int64": true, "uint8": true,
	"uint16": true, "uint32": true, "uint64": true, "uintptr": true, "byte": true, "true": true,
	"false": true, "nil": true, "iota": true, "unsafe": true, "C": true,
}

// goIdent returns name as an identifier of the view: a keyword or a name
// that the output uses gets a trailing underscore; characters that Go does
// not allow become underscores.
func goIdent(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" {
		return "_"
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "_" + s
	}
	if goKeywords[s] {
		s += "_"
	}
	return s
}

// A typeErr is a C++ type that the view cannot write.
type typeErr struct{ msg string }

func (e *typeErr) Error() string { return e.msg }

// goType writes the type with the text spelling (as clang prints it) as seen
// from the C++ scope.
func (g *gen) goType(spelling string, scope []string) (string, error) {
	spelling = g.stripAtomic(spelling)
	t, err := ctype.Parse(spelling)
	if err != nil {
		return "", &typeErr{err.Error()}
	}
	return g.typeExpr(t, scope), nil
}

// typeExpr writes a parsed type. It does not fail: what has no spelling in
// the view becomes an opaque type or unsafe.Pointer.
func (g *gen) typeExpr(t *ctype.Type, scope []string) string {
	switch t.Kind {
	case ctype.Builtin:
		if t.Name == "void" {
			return "struct{}"
		}
		if s, ok := builtinGo[t.Name]; ok {
			return s
		}
		return g.opaqueType(t.Name)
	case ctype.Pointer:
		e := t.Elem
		if f := g.funcTypedef(e, scope); f != nil {
			e = f // a pointer to a typedef of a function type is a function pointer
		}
		if e.Kind == ctype.Func {
			return g.funcType(e, scope, nil)
		}
		if e.Kind == ctype.Builtin && e.Name == "void" {
			g.useUnsafe = true
			return "unsafe.Pointer"
		}
		if e.Kind == ctype.Named && e.QualName() == "__va_list_tag" {
			return "C.va_list" // va_list as a parameter has decayed to a pointer to its element
		}
		return "*" + g.typeExpr(e, scope)
	case ctype.LRef, ctype.RRef:
		return "*" + g.typeExpr(t.Elem, scope)
	case ctype.Array:
		n := t.Len
		if n < 0 {
			n = 0
		}
		return "[" + itoa(n) + "]" + g.typeExpr(t.Elem, scope)
	case ctype.Func:
		return g.funcType(t, scope, nil)
	case ctype.MemberPtr:
		g.note("a pointer to member is written as unsafe.Pointer")
		g.useUnsafe = true
		return "unsafe.Pointer"
	case ctype.Named:
		return g.named(t, scope)
	case ctype.Unnamed:
		if g.inline != "" {
			return g.inline
		}
		return "struct{}"
	}
	g.note(t.Name + " is written as unsafe.Pointer")
	g.useUnsafe = true
	return "unsafe.Pointer"
}

// funcType writes a function type as a Go func type (a C function pointer).
// names are the parameter names, or nil.
func (g *gen) funcType(f *ctype.Type, scope []string, names []string) string {
	var b strings.Builder
	b.WriteString("func(")
	b.WriteString(g.params(f, scope, names))
	b.WriteString(")")
	if r := g.result(f, scope); r != "" {
		b.WriteString(" " + r)
	}
	return b.String()
}

// result writes the result type of a function type, "" for void.
func (g *gen) result(f *ctype.Type, scope []string) string {
	if f.Elem == nil || f.Elem.Kind == ctype.Builtin && f.Elem.Name == "void" {
		return ""
	}
	if g.isHResult(f.Elem, scope) {
		return "error" // a call of a function that returns an HRESULT is an error in the program (ADR-137)
	}
	return g.typeExpr(f.Elem, scope)
}

// isHResult reports whether t is the typedef HRESULT of the view: the name HRESULT, outside any
// namespace, of a typedef that stands for int or long (LONG on Windows). It is the rule of
// adapt.RowHResult, over the declarations of the header.
func (g *gen) isHResult(t *ctype.Type, scope []string) bool {
	if t.Kind != ctype.Named || t.QualName() != "HRESULT" {
		return false
	}
	for hops := 0; hops < 8 && t.Kind == ctype.Named; hops++ {
		d := g.lookup(t.QualName(), t.Global, scope)
		if d == nil || d.kind != "typedef" {
			return false
		}
		u, err := ctype.Parse(d.node.qual())
		if err != nil {
			return false
		}
		if u.Kind == ctype.Builtin {
			return u.Name == "int" || u.Name == "long"
		}
		t = u
	}
	return false
}

// params writes the parameter list of a function type. Names are used when
// every parameter has one (Go does not allow a mix).
func (g *gen) params(f *ctype.Type, scope []string, names []string) string {
	named := len(names) == len(f.Params) && len(names) > 0
	if named {
		any := false
		for _, n := range names {
			any = any || n != ""
		}
		named = any
	}
	var parts []string
	used := map[string]bool{}
	for i, p := range f.Params {
		ty := g.typeExpr(p, scope)
		if named {
			n := goIdent(paramName(names[i]))
			if names[i] == "" || used[n] {
				n = "_"
			}
			used[n] = true
			parts = append(parts, n+" "+ty)
		} else {
			parts = append(parts, ty)
		}
	}
	if f.Variadic {
		if named {
			parts = append(parts, "args ...any")
		} else {
			parts = append(parts, "...any")
		}
	}
	return strings.Join(parts, ", ")
}

// paramName drops the underscores that C libraries put in front of parameter
// names (__stream, __filename) to keep them clear of user macros.
func paramName(n string) string {
	if t := strings.TrimLeft(n, "_"); t != "" && n != "_" {
		return t
	}
	return n
}

// named writes a type that has a name.
func (g *gen) named(t *ctype.Type, scope []string) string {
	qual := t.QualName()
	hasArgs := false
	for _, s := range t.Path {
		if s.HasArgs {
			hasArgs = true
		}
	}
	if hasArgs {
		return g.opaqueType(t.String())
	}
	std := strings.TrimPrefix(qual, "std::")
	if s, ok := standardGo[std]; ok && (!strings.Contains(qual, "::") || strings.HasPrefix(qual, "std::")) {
		if s == "unsafe.Pointer" {
			g.useUnsafe = true
		}
		return s
	}
	if d := g.lookup(qual, t.Global, scope); d != nil {
		return g.ref(d)
	}
	bare := t.Clone()
	bare.Const, bare.Volatile, bare.Tag = false, false, ""
	return g.opaqueType(bare.String())
}

// lookup finds the type "qual" as seen from scope: the innermost scope first.
func (g *gen) lookup(qual string, global bool, scope []string) *gdecl {
	if !global {
		for i := len(scope); i > 0; i-- {
			if d := g.tab.types[strings.Join(scope[:i], "::")+"::"+qual]; d != nil {
				return d
			}
		}
	}
	return g.tab.types[qual]
}

// ref returns the Go name of a declaration that another declaration uses.
// A declaration that the view would not write otherwise (it is in another
// header, or its name is private, or -exclude dropped it) is written as a
// referenced type.
func (g *gen) ref(d *gdecl) string {
	if d.into != nil {
		d = d.into
	}
	if d.kind == "typedef" && g.isStandard(d) {
		return g.standardName(d)
	}
	g.need(d)
	return d.goName
}

// isStandard says that a typedef is one of the names that are mapped to Go
// types (size_t in the C library's own header).
func (g *gen) isStandard(d *gdecl) bool {
	_, ok := standardGo[d.name]
	return ok && len(d.scope) == 0
}

func (g *gen) standardName(d *gdecl) string {
	s := standardGo[d.name]
	if s == "unsafe.Pointer" {
		g.useUnsafe = true
	}
	return s
}

// opaqueType returns the name of a stand-in for a C++ type that the view
// cannot describe: an instance of a template, or a type that no header in
// view declares. It is written as an empty struct.
func (g *gen) opaqueType(spelling string) string {
	if name, ok := g.opaque[spelling]; ok {
		return name
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range spelling {
		if r < 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
			lastUnderscore = false
		} else if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	name := g.fresh(strings.Trim(b.String(), "_"))
	g.opaque[spelling] = name
	g.opaqueList = append(g.opaqueList, spelling)
	return name
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

var (
	unnamedLoc = regexp.MustCompile(`\((unnamed|anonymous) ([a-z]+(?: [a-z]+)*) at [^)]*\)`)
	fileLoc    = regexp.MustCompile(`[^\s"(),]+\.(?:h|hh|hpp|hxx|h\+\+|inl|c|cc|cpp|cxx):\d+(?::\d+)?`)
)

// scrub takes the places out of the text that clang prints for unnamed types
// ("(unnamed struct at /home/me/x.h:13:18)"): the output must not depend on
// where the headers are.
func scrub(s string) string {
	s = unnamedLoc.ReplaceAllString(s, "($1 $2)")
	return fileLoc.ReplaceAllString(s, "<location>")
}

// funcTypedef returns the function type that the typedef named by t stands for, or nil.
func (g *gen) funcTypedef(t *ctype.Type, scope []string) *ctype.Type {
	if t.Kind != ctype.Named {
		return nil
	}
	for hops := 0; hops < 8 && t.Kind == ctype.Named; hops++ {
		d := g.lookup(t.QualName(), t.Global, scope)
		if d == nil || d.kind != "typedef" {
			return nil
		}
		u, err := ctype.Parse(d.node.qual())
		if err != nil {
			return nil
		}
		if u.Kind == ctype.Func {
			return u
		}
		t = u
	}
	return nil
}
