package lower

import (
	"maps"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Macro protection (Phase 04 README §2.3, step 4.3). A C/C++ header can define macros whose
// names the generated code uses: X11's #define Bool int would turn a gocpp type Bool into int,
// and #define string ... would break gp::string. Two rules keep the generated code safe:
//
//  1. gocpp's own headers (gocpp/rt.hpp) come first, before the C/C++ includes, so no header
//     macro reaches the runtime's code. (gocpp/fmtcxx.hpp comes after them, because it looks
//     for the headers' <format>; its own names start with gp_.)
//  2. The names after the includes:
//     - a name of the package (a variable, a function, a type, a field, a label) that is a macro
//       of the headers (interop.ProbeMacros) gets a _ suffix, like a name that is a macro of the
//       runtime's headers (names.go: g.escape): type Bool bool is the C++ struct Bool_. The .gp
//       name stays what fmt's %T prints (main.Bool);
//     - every other name that the generated code writes and that is a macro of the headers (a
//       name of the runtime or of another gocpp package: gp::string, std::tuple) is #undef'd
//       after the includes (protectNames). If the code also uses that macro through a header
//       package (p.Println next to fmt.Println), it can't be both: that is a user error at the
//       macro's use, which suggests a helper header.
//
// #undef, not push_macro/pop_macro: nothing after the generated code needs the macros again,
// because every C/C++ include of the translation unit comes before it (Phase 06's layout keeps
// gocpp's headers first too). The package's own names are renamed rather than their macros
// #undef'd, so that a header's other macros that expand to such a macro keep working
// (#define MAKE(x) ((Bool)(x)) next to #define Bool int). The macros of the runtime's headers
// (cxx_macros.txt: EOF, INT64_C) are never #undef'd: the generated code uses some of them, and
// the package's names avoid all of them already.

// isMacro reports whether s is a macro of the package's headers.
func (g *gen) isMacro(s string) bool {
	return g.macros[s]
}

// escape is Escape for a name of the package: a macro of its headers is reserved too.
func (g *gen) escape(name string) string {
	return escapeAvoiding(name, g.isMacro)
}

// typeName is TypeName for a type of the package: a macro of its headers is reserved too.
func (g *gen) typeName(name string) string {
	return escapeAvoiding(name, func(s string) bool { return typeAvoid(s) || g.isMacro(s) })
}

// methodName is MethodName for a method or a field of a type of the package: a macro of its
// headers is reserved too.
func (g *gen) methodName(typ, method string) string {
	return escapeAvoiding(method, func(s string) bool { return s == typ || g.isMacro(s) })
}

// runtimeMacros are the macros of the runtime's headers (names.go), which protectNames never
// #undefs.
var runtimeMacros = func() map[string]bool {
	m := make(map[string]bool)
	for _, w := range listWords(cxxMacrosTxt) {
		m[w] = true
	}
	for _, w := range moreMacros {
		m[w] = true
	}
	return m
}()

// protectNames adds the #undef lines after the includes of f (see the top of macros.go). A
// macro that the code uses through a header package and whose name the generated code uses for
// something else too can't be protected: that is the user's error, at the macro's use.
func (g *gen) protectNames(f *cppir.File) {
	if len(g.macros) == 0 {
		return
	}
	var undef []string
	for _, name := range slices.Sorted(maps.Keys(cxxNames(g.printWithoutCNames(f)))) {
		if !g.isMacro(name) || runtimeMacros[name] {
			continue
		}
		if pos, ok := g.cNames[name]; ok {
			g.userError(pos, "the macro %s of the imported headers has a name that gocpp's generated C++ uses for something else too (a name of the runtime or of a gocpp package), and the macro would break it: use the macro through a small helper header instead (a function in a .h file next to the .gp file that calls %s)", name, name)
			return
		}
		undef = append(undef, name)
	}
	if len(undef) == 0 {
		return
	}
	at := 0
	for i, d := range f.Decls {
		if _, ok := d.(*cppir.Include); ok {
			at = i + 1
		}
	}
	ds := make([]cppir.Decl, len(undef))
	for i, name := range undef {
		ds[i] = &cppir.Directive{Text: "undef " + name}
	}
	f.Decls = slices.Insert(f.Decls, at, ds...)
}

// printWithoutCNames returns the C++ of f with the names of the C/C++ names (cNodes) replaced by
// gp_cname: the names that the generated code itself uses.
func (g *gen) printWithoutCNames(f *cppir.File) []byte {
	mask := func(s string) string {
		var b strings.Builder
		for i := 0; i < len(s); {
			j := i
			for j < len(s) && isIdentRune(rune(s[j])) {
				j++
			}
			if j > i {
				if _, ok := g.cNames[s[i:j]]; ok {
					b.WriteString("gp_cname")
				} else {
					b.WriteString(s[i:j])
				}
				i = j
				continue
			}
			b.WriteByte(s[i])
			i++
		}
		return b.String()
	}
	var restore []func()
	for _, n := range g.cNodes {
		switch x := n.(type) {
		case *cppir.Name:
			old := x.Name
			x.Name, restore = mask(old), append(restore, func() { x.Name = old })
		case *cppir.TypeName:
			old := x.Name
			x.Name, restore = mask(old), append(restore, func() { x.Name = old })
		case *cppir.Member:
			old := x.Name
			x.Name, restore = mask(old), append(restore, func() { x.Name = old })
		case *cppir.Designated:
			old := x.Name
			x.Name, restore = mask(old), append(restore, func() { x.Name = old })
		}
	}
	out := cppir.Print(f, "")
	for i := len(restore) - 1; i >= 0; i-- {
		restore[i]() // backwards: a node recorded twice gets its first name back
	}
	return out
}

// cxxNames returns the identifiers of the C++ text src, outside the preprocessor lines, the
// comments, and the string and character literals (with their suffixes: "x"_gs).
func cxxNames(src []byte) map[string]bool {
	names := make(map[string]bool)
	for line := range strings.SplitSeq(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		for i := 0; i < len(line); {
			c := line[i]
			switch {
			case c == '/' && i+1 < len(line) && line[i+1] == '/':
				i = len(line)
			case c == '/' && i+1 < len(line) && line[i+1] == '*':
				end := strings.Index(line[i+2:], "*/")
				if end < 0 {
					i = len(line)
				} else {
					i += end + 4
				}
			case c == '"' || c == '\'':
				i++
				for i < len(line) && line[i] != c {
					if line[i] == '\\' {
						i++
					}
					i++
				}
				i++
				for i < len(line) && isIdentRune(rune(line[i])) {
					i++ // a literal's suffix: "x"_gs
				}
			case isIdentRune(rune(c)) && !('0' <= c && c <= '9'):
				j := i
				for j < len(line) && isIdentRune(rune(line[j])) {
					j++
				}
				names[line[i:j]] = true
				i = j
			case '0' <= c && c <= '9':
				for i < len(line) && (isIdentRune(rune(line[i])) || line[i] == '.' || line[i] == '\'') {
					i++ // a number, with its suffix: 1.5f, INT64_C is a name
				}
			default:
				i++
			}
		}
	}
	return names
}
