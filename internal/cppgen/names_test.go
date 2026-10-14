package cppgen

import (
	"go/token"
	"go/types"
	"slices"
	"strings"
	"testing"
)

func TestEscape(t *testing.T) {
	tests := []struct{ in, want string }{
		{"x", "x"},
		{"Vec2", "Vec2"},
		{"new", "new_"},
		{"class", "class_"},
		{"delete", "delete_"},
		{"new_", "new__"}, // the user's new_ can't meet new
		{"new__", "new___"},
		{"_Pragma", "_Pragma_"},
		{"_Complex", "_Complex_"},
		{"__int128", "__int128_"},
		{"__is_same", "__is_same_"},
		{"__FILE__", "__FILE___"},
		{"EOF", "EOF_"},
		{"errno", "errno_"},
		{"stdin", "stdin_"},
		{"assert", "assert_"},
		{"INT64_C", "INT64_C_"},
		{"int64_t", "int64_t_"},
		{"uintptr_t", "uintptr_t_"},
		{"gp_count", "gp_count_"},
		{"gp_t0", "gp_t0_"},
		{"gp_t0_", "gp_t0__"},
		{"gp", "gp"},
		{"std", "std"},
		{"_X", "_X"},     // reserved by C++, accepted by the compilers (ADR-010)
		{"a__b", "a__b"}, // the same
		{"linux", "linux"},
		{"unix", "unix"},
		{"main", "main"},
		// __asm and __asm__ are both reserved: the family __asm, __asm_, __asm__, ... maps
		// to its free names in order.
		{"__asm", "__asm_"},
		{"__asm_", "__asm___"},
		{"__asm__", "__asm____"},
		{"__asm___", "__asm_____"},
	}
	for _, tt := range tests {
		if got := Escape(tt.in); got != tt.want {
			t.Errorf("Escape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestEscapeOneToOne checks for many names, with each reserved name and its family, that
// Escape is one-to-one, that its result is never reserved, and that a result that starts
// with gp_ ends in _, so it can't be a generated name.
func TestEscapeOneToOne(t *testing.T) {
	var names []string
	for r := range reserved {
		base := strings.TrimRight(r, "_")
		for k := range 6 {
			names = append(names, base+strings.Repeat("_", k))
		}
	}
	for _, s := range []string{"x", "v", "gp", "gp_", "gp_x", "gp_t0", "_", "__", "a_b", "U"} {
		for k := range 4 {
			names = append(names, s+strings.Repeat("_", k))
		}
	}
	slices.Sort(names)
	names = slices.Compact(names)
	seen := make(map[string]string)
	for _, n := range names {
		e := Escape(n)
		if other, ok := seen[e]; ok {
			t.Errorf("Escape(%q) = Escape(%q) = %q", n, other, e)
		}
		seen[e] = n
		if reserved[e] {
			t.Errorf("Escape(%q) = %q, which is reserved", n, e)
		}
		if strings.HasPrefix(e, "gp_") && !strings.HasSuffix(e, "_") {
			t.Errorf("Escape(%q) = %q, which could be a generated name", n, e)
		}
	}
}

func TestMethodName(t *testing.T) {
	tests := []struct{ typ, method, want string }{
		{"U", "U", "U_"}, // func (U) U(): C++ would read U() as a constructor
		{"U", "U_", "U__"},
		{"U", "V", "V"},
		{"U", "new", "new_"},
		{"new_", "new", "new__"}, // type new: its method new can't be new_
		{"Vec2", "Len", "Len"},
	}
	for _, tt := range tests {
		if got := MethodName(tt.typ, tt.method); got != tt.want {
			t.Errorf("MethodName(%q, %q) = %q, want %q", tt.typ, tt.method, got, tt.want)
		}
	}
}

func TestNamespaceName(t *testing.T) {
	tests := []struct{ path, want string }{
		{"main", "gpkg_main"},
		{"fmt", "gpkg_fmt"},
		{"github.com/huuhait/game/engine", "gpkg_github_dcom__huuhait__game__engine"},
		{"github.com/mattn/go-sqlite3", "gpkg_github_dcom__mattn__go_hsqlite3"},
		{"a_b~c+d", "gpkg_a_ub_tc_pd"},
		{"x.u", "gpkg_x_du"},
		{"x_", "gpkg_x_u"},
		{"é", "gpkg__xc3_xa9"},
	}
	for _, tt := range tests {
		if got := NamespaceName(tt.path); got != tt.want {
			t.Errorf("NamespaceName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// TestReservedLists checks the lists that names.go reads.
func TestReservedLists(t *testing.T) {
	// The C++20 keywords and alternative tokens ([lex.key], [lex.digraph]).
	keywords := strings.Fields(`alignas alignof asm auto bool break case catch char char8_t
		char16_t char32_t class concept const consteval constexpr constinit const_cast continue
		co_await co_return co_yield decltype default delete do double dynamic_cast else enum
		explicit export extern false float for friend goto if inline int long mutable namespace
		new noexcept nullptr operator private protected public register reinterpret_cast
		requires return short signed sizeof static static_assert static_cast struct switch
		template this thread_local throw true try typedef typeid typename union unsigned using
		virtual void volatile wchar_t while and and_eq bitand bitor compl not not_eq or or_eq
		xor xor_eq`)
	words := listWords(cxxWordsTxt)
	for _, k := range append(keywords, "_Pragma", "_Complex", "__int128", "__attribute__", "__FILE__", "__VA_ARGS__") {
		if !slices.Contains(words, k) {
			t.Errorf("cxx_words.txt lacks %s", k)
		}
	}
	macros := listWords(cxxMacrosTxt)
	for _, m := range []string{"EOF", "errno", "stdin", "stdout", "stderr", "NULL", "EINVAL", "ENOENT",
		"EAGAIN", "SEEK_SET", "BUFSIZ", "INT64_C", "INT64_MIN", "UINT64_C", "__x86_64__", "__GNUC__"} {
		if !slices.Contains(macros, m) {
			t.Errorf("cxx_macros.txt lacks %s", m)
		}
	}
	for _, list := range [][]string{words, macros} {
		if !slices.IsSorted(list) {
			t.Error("a list is not sorted")
		}
	}
	for r := range reserved {
		if strings.HasPrefix(r, "gp_") {
			t.Errorf("%s is reserved, but gp_ names are the generated code's", r)
		}
		if !token.IsIdentifier(r) && !token.Lookup(r).IsKeyword() {
			t.Errorf("%s is not an identifier", r)
		}
	}
}

// TestFuncNames checks the choices of funcNames: new names for a declaration that uses its
// own name and for a second declaration in one C++ scope, never a taken name, and the
// counters of the temporaries.
func TestFuncNames(t *testing.T) {
	v := func(name string) types.Object { return types.NewVar(token.NoPos, nil, name, types.Typ[types.Int]) }
	f := newFuncNames()
	for _, n := range []string{"v", "v_1", "new_1", "x"} {
		f.reserve(n) // names that the function declares or uses
	}
	f.openScope()
	v0 := v("v")
	check := func(got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	check(f.declare(v0, nil), "v")
	f.openScope()
	check(f.declare(v("v"), map[string]bool{"v": true}), "v_2") // v := v; v_1 is taken
	check(f.declare(v("x"), map[string]bool{"v_2": true}), "x")
	check(f.declare(v("x"), nil), "x_1") // a second x in the same C++ scope
	check(f.declare(v("new"), map[string]bool{"new_": true}), "new_2")
	check(f.declare(v("gp_x"), map[string]bool{"gp_x_": true}), "gp_x_1_") // never like gp_t0
	check(f.declare(v("EOF"), map[string]bool{"EOF_": true}), "EOF_1")
	f.openScope()
	check(f.declare(v("v"), map[string]bool{"v_2": true}), "v") // v := v_2 * 2: no clash
	f.closeScope()
	f.closeScope()
	check(f.names[v0], "v")
	check(f.temp("t"), "gp_t0")
	check(f.temp("t"), "gp_t1")
	check(f.temp("cont_"), "gp_cont_0")
	check(f.temp("t"), "gp_t2")
}
