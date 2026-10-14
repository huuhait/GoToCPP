package cppgen

import (
	_ "embed"
	"fmt"
	"go/types"
	"strconv"
	"strings"
)

// The C++ names of Go names (Phase 02 README §2.2, ADR-010).
//
// gocpp keeps a Go name in C++ wherever it can. A name gets a _ suffix when it is reserved:
//   - a word of cxx_words.txt: a C++20 keyword or alternative token (new, class, and), or an
//     extension word of g++ or clang++ (_Pragma, _Complex, __int128, __is_same, __FILE__);
//     mkwords.sh finds them by trying the identifiers that the compiler programs hold;
//   - a macro of the headers that the runtime includes (cxx_macros.txt: EOF, errno, stdin,
//     NULL, EINVAL, BUFSIZ, INT64_C, the compilers' predefined macros such as __x86_64__),
//     which would replace the name; TestMacroList keeps the list up to date;
//   - a name that the generated code uses unqualified inside a package namespace
//     (unqualifiedNames: int64_t and the other integer types), which a user name would hide;
//   - a name that starts with gp_: the generated names (temporaries such as gp_t0, gp_init_all)
//     start with gp_ and never end in _, so they can't clash with an escaped user name.
//
// Other names that C++ reserves (_X, a__b) stay as written: GCC and Clang accept them
// (ADR-010), and gocpp vet warns about them.
//
// The suffix keeps the mapping one-to-one, also for names that already end in _: new → new_,
// and the user's own new_ → new__, so the two can't meet in one scope. Precisely: the names
// b, b_, b__, ... (b doesn't end in _) form a family, and Escape maps the n-th name of a
// family to its n-th name that is not reserved. So a name below the first reserved one
// stays as it is, and every other one moves up past the reserved ones: __asm and __asm__
// are both reserved, so __asm → __asm_ and __asm_ → __asm___. The result is never reserved,
// and two names never get the same result.

// cxxWords are the C++ keywords and the compilers' extension words (mkwords.sh).
//
//go:embed cxx_words.txt
var cxxWordsTxt string

// cxxMacros are the macros of the headers that gocpp/rt.hpp includes, with g++ and clang++
// and the driver's flags (TestMacroList, which `go test -run TestMacroList -update` runs to
// add new ones). Phase 04 adds the macros of the user's own headers.
//
//go:embed cxx_macros.txt
var cxxMacrosTxt string

// unqualifiedNames are the names that the generated code writes without a namespace inside a
// package namespace. A user name like them would hide them (int64_t x = 1; inside a namespace
// that declares a variable int64_t doesn't compile). The macros that it writes (INT64_C,
// INT64_MIN) are in cxxMacros, and gp and std are safe: a name before :: finds only
// namespaces and types, and Phase 02 has no user types.
var unqualifiedNames = []string{
	"int8_t", "int16_t", "int32_t", "int64_t",
	"uint8_t", "uint16_t", "uint32_t", "uint64_t", "uintptr_t",
}

// moreMacros are macros of the C and C++ standard headers that the runtime doesn't include
// today, but whose names the plan lists (Phase 02 README §2.2): reserving them now means that
// a runtime header that includes <cassert> later doesn't rename user names.
var moreMacros = []string{"assert"}

// reserved holds every reserved name.
var reserved = func() map[string]bool {
	m := make(map[string]bool)
	for _, txt := range []string{cxxWordsTxt, cxxMacrosTxt} {
		for _, w := range listWords(txt) {
			m[w] = true
		}
	}
	for _, w := range unqualifiedNames {
		m[w] = true
	}
	for _, w := range moreMacros {
		m[w] = true
	}
	return m
}()

// listWords returns the words of a list file: one per line, without the empty lines and the
// comment lines (which start with #).
func listWords(txt string) []string {
	var words []string
	for l := range strings.SplitSeq(txt, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			words = append(words, l)
		}
	}
	return words
}

// Escape returns the C++ name of the Go name name: name itself, or name with _ suffixes when
// it is reserved (see the top of names.go). It is deterministic and one-to-one.
func Escape(name string) string {
	return escapeAvoiding(name, "")
}

// MethodName returns the C++ name of the method method of the type whose C++ name is typ: as
// Escape, and a method whose name is the type's (func (U) U(), which C++ would read as a
// constructor) gets a _ suffix too: U_. The methods of one type keep different names.
func MethodName(typ, method string) string {
	return escapeAvoiding(method, typ)
}

// escapeAvoiding escapes name as Escape does, with avoid (if it isn't "") as one more
// reserved name.
func escapeAvoiding(name, avoid string) string {
	base := strings.TrimRight(name, "_")
	n := len(name) - len(base) // name is the n-th name of the family of base
	taken := func(k int) bool {
		s := base + strings.Repeat("_", k)
		return reserved[s] || s == avoid || (k == 0 && strings.HasPrefix(base, "gp_"))
	}
	k := 0
	for i := -1; ; k++ {
		if !taken(k) {
			if i++; i == n {
				break
			}
		}
	}
	if k == n {
		return name
	}
	return base + strings.Repeat("_", k)
}

// NamespaceName returns the C++ namespace of the package with the import path path (ADR-010):
// gpkg_ and the path, in which letters and digits stay and every other byte is a code that
// starts with _ ("/" → "__", "." → "_d", "-" → "_h", "_" → "_u", "~" → "_t", "+" → "_p", any
// other byte → "_x" and two hex digits), so two paths never give the same namespace. Package
// main is always gpkg_main: a program has one.
func NamespaceName(path string) string {
	var b strings.Builder
	b.WriteString("gpkg_")
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
			b.WriteByte(c)
		case c == '/':
			b.WriteString("__")
		case c == '.':
			b.WriteString("_d")
		case c == '-':
			b.WriteString("_h")
		case c == '_':
			b.WriteString("_u")
		case c == '~':
			b.WriteString("_t")
		case c == '+':
			b.WriteString("_p")
		default:
			fmt.Fprintf(&b, "_x%02x", c)
		}
	}
	return b.String()
}

// A funcNames chooses the C++ names inside one function: of its parameters, results and local
// variables, and of the temporaries that the generated code needs.
//
// A local variable keeps its escaped Go name, unless C++ would see it differently than Go:
//   - the declaration uses the name it declares (v := v, x := x + 1, var v = v): in C++ the
//     new name is already in scope in its own initializer, so int64_t v = v; would read the
//     new, uninitialized v (Phase 02 README §2.2);
//   - the name is declared already in the same C++ scope: the C++ of a statement can put two
//     Go scopes into one C++ block.
//
// Then it gets a new name: name_1, name_2, ... (the first one that is free). A new name is
// never reserved, and never the name of anything that the function declares or uses, so it
// can't hide what the code refers to. All choices depend only on the source, in source order.
type funcNames struct {
	// taken are the names that a new name must not be: the C++ names of every variable and
	// function that the function declares or uses, and the new names chosen so far.
	taken map[string]bool

	// names are the C++ names of the function's own objects.
	names map[types.Object]string

	// scopes are the C++ block scopes, innermost last: the names declared in each.
	scopes []map[string]bool

	// temps counts the temporaries of each kind (temp).
	temps map[string]int
}

func newFuncNames() *funcNames {
	return &funcNames{
		taken: make(map[string]bool),
		names: make(map[types.Object]string),
		temps: make(map[string]int),
	}
}

// reserve marks a C++ name as one that a new name must not be: the name of an object that the
// function declares or uses. Call it for all of them before declaring anything.
func (f *funcNames) reserve(name string) {
	f.taken[name] = true
}

// openScope starts a C++ block scope; closeScope ends it.
func (f *funcNames) openScope()  { f.scopes = append(f.scopes, make(map[string]bool)) }
func (f *funcNames) closeScope() { f.scopes = f.scopes[:len(f.scopes)-1] }

// declare chooses the C++ name of the variable obj, declared in the innermost C++ scope. used
// holds the C++ names that its declaration's initializers refer to (the names of the
// variables and functions in them); obj gets a new name if its own name is one of them, or if
// the scope already has it.
func (f *funcNames) declare(obj types.Object, used map[string]bool) string {
	name := Escape(obj.Name())
	scope := f.scopes[len(f.scopes)-1]
	if used[name] || scope[name] {
		name = f.newName(obj.Name())
	}
	scope[name] = true
	f.names[obj] = name
	return name
}

// newName returns the first of name_1, name_2, ... that is free: not reserved, not taken,
// and not like a generated name (a name that starts with gp_ gets a _ at the end: gp_x_1_).
func (f *funcNames) newName(name string) string {
	for i := 1; ; i++ {
		s := name + "_" + strconv.Itoa(i)
		if strings.HasPrefix(s, "gp_") {
			s += "_"
		}
		if !reserved[s] && !f.taken[s] {
			f.taken[s] = true
			return s
		}
	}
}

// temp returns the next temporary of the function of a kind: gp_<kind><n>, n = 0, 1, ... for
// each kind (temp("t") is gp_t0, then gp_t1). Temporaries start with gp_, so they never clash
// with a user name (Escape gives those a _ suffix). kind must not end in a digit, so two kinds
// can't make the same name (temp("cont_") gives gp_cont_0). A temporary is not declared in a
// scope: each name is declared once in the function.
func (f *funcNames) temp(kind string) string {
	n := f.temps[kind]
	f.temps[kind] = n + 1
	return "gp_" + kind + strconv.Itoa(n)
}
