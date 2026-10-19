package bindgen

import (
	"strings"
	"testing"
)

func newTestGen() *gen {
	return newGen(&Config{}, newTable(), &ppResult{})
}

func TestGoType(t *testing.T) {
	cases := []struct {
		in, want string
		unsafe   bool
	}{
		{"int", "C.int", false},
		{"unsigned char", "C.uchar", false},
		{"signed char", "C.schar", false},
		{"long long", "C.longlong", false},
		{"unsigned long long", "C.ulonglong", false},
		{"unsigned long", "C.ulong", false},
		{"long double", "C.longdouble", false},
		{"bool", "C.bool", false},
		{"wchar_t", "C.wchar_t", false},
		{"char16_t", "C.char16_t", false},
		{"__int128", "[16]byte", false},
		{"const char *", "*C.char", false},
		{"const char *const *", "**C.char", false},
		{"void *", "unsafe.Pointer", true},
		{"void **", "*unsafe.Pointer", true},
		{"int[4]", "[4]C.int", false},
		{"int[2][3]", "[2][3]C.int", false},
		{"int[]", "[0]C.int", false},
		{"int (*)[3]", "*[3]C.int", false},
		{"int &", "*C.int", false},
		{"int &&", "*C.int", false},
		{"int (*)(int, const char *)", "func(C.int, *C.char) C.int", false},
		{"void (*)(void *)", "func(unsafe.Pointer)", true},
		{"void (*)()", "func()", false},
		{"int (*)(int, ...)", "func(C.int, ...any) C.int", false},
		{"int (int, int)", "func(C.int, C.int) C.int", false},
		{"int (*(*)(int))(char)", "func(C.int) func(C.char) C.int", false},
		{"size_t", "C.size_t", false},
		{"std::size_t", "C.size_t", false},
		{"uint8_t", "uint8", false},
		{"std::uint64_t", "uint64", false},
		{"uintptr_t", "uintptr", false},
		{"va_list", "C.va_list", false},
		{"__gnuc_va_list", "C.va_list", false},
		{"struct __va_list_tag *", "C.va_list", false},
		{"std::nullptr_t", "unsafe.Pointer", true},
		{"std::vector<int>", "std_vector_int", false},
		{"const std::string &", "*std_string", false},
		{"std::map<std::string, int>", "std_map_std_string_int", false},
		{"Unknown", "Unknown", false},
		{"ns::Thing *", "*ns_Thing", false},
		{"int Foo::*", "unsafe.Pointer", true},
		{"(unnamed struct)", "struct{}", false},
		{"void", "struct{}", false},
	}
	for _, c := range cases {
		g := newTestGen()
		got, err := g.goType(c.in, nil)
		if err != nil {
			t.Errorf("goType(%q): %v", c.in, err)
			continue
		}
		if got != c.want || g.useUnsafe != c.unsafe {
			t.Errorf("goType(%q) = %q (unsafe %v), want %q (%v)", c.in, got, g.useUnsafe, c.want, c.unsafe)
		}
	}
	g := newTestGen()
	if _, err := g.goType("int (", nil); err == nil || !strings.Contains(err.Error(), "int (") {
		t.Errorf("a bad type: %v", err)
	}
}

func TestParamsAndResults(t *testing.T) {
	g := newTestGen()
	ft := func(spelling string) string {
		s, err := g.goType(spelling, nil)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	if got := ft("int (int, char *)"); got != "func(C.int, *C.char) C.int" {
		t.Errorf("%s", got)
	}
	// parameter names: all or none; underscores dropped; duplicates and empty names are _
	var cfg = newTestGen()
	ty := mustParse(t, "void (int, int, int, ...)")
	for _, c := range []struct {
		names []string
		want  string
	}{
		{nil, "C.int, C.int, C.int, ...any"},
		{[]string{"a", "b", "c"}, "a C.int, b C.int, c C.int, args ...any"},
		{[]string{"a", "", "c"}, "a C.int, _ C.int, c C.int, args ...any"},
		{[]string{"", "", ""}, "C.int, C.int, C.int, ...any"},
		{[]string{"__a", "a", "type"}, "a C.int, _ C.int, type_ C.int, args ...any"},
		{[]string{"a", "b"}, "C.int, C.int, C.int, ...any"}, // not one name for each parameter
	} {
		if got := cfg.params(ty, nil, c.names); got != c.want {
			t.Errorf("params(%v) = %q, want %q", c.names, got, c.want)
		}
	}
}

func TestFreshNames(t *testing.T) {
	g := newTestGen()
	for _, c := range [][2]string{
		{"x", "x"}, {"x", "x_2"}, {"x", "x_3"}, {"type", "type_"}, {"type", "type__2"}, {"", "_"}, {"a-b", "a_b"},
	} {
		if got := g.fresh(c[0]); got != c[1] {
			t.Errorf("fresh(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}
