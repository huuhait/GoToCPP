package lower

import (
	"go/types"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
)

// The decisions of typed code generation that need no compiler (ADR-087): which parameter kinds take a
// gocpp string, when a set of overloads agrees, which functions keep a pointer, and when two
// numbers are the same C++ type. cmd/gocpp's TestTypedRun and TestTypedEmit run the whole.

func TestParamConv(t *testing.T) {
	for _, c := range []struct {
		typ, scope string
		want       argConv
	}{
		{"const char *", "", convCStr},
		{"char *", "", convNoStr},
		{"const unsigned char *", "", convNoStr},
		{"int", "", convNoStr},
		{"const int *", "", convNoStr},
		{"std::string", "", convStdString},
		{"const std::string &", "", convStdString},
		{"std::string &&", "", convStdString},
		{"std::string &", "", convNone},                // an lvalue string: no temporary can bind
		{"std::basic_string<char>", "", convStdString}, // as clang prints it
		{"const std::wstring &", "", convNoStr},        // no conversion from a gocpp string
		{"std::basic_string<wchar_t>", "", convNoStr},  //
		{"string", "std", convStdString},               // relative, inside std
		{"string", "", convNone},                       // a user's own string
		{"const string &", "myns", convNone},           //
		{"std::string_view", "", convNone},             // converts by itself, in opaque mode too
		{"Widget", "", convNone},                       // could have a constructor: unknown
		{"const Widget &", "", convNone},               //
		{"int (*)(int)", "", convNoStr},                // a function pointer
		{"int [4]", "", convNoStr},                     // an array
		{"const char *const *", "", convNoStr},         // a pointer to pointers
		{"const char *const", "", convCStr},            // the top-level const of the pointer
	} {
		ty, err := ctype.Parse(c.typ)
		if err != nil {
			t.Fatalf("%s: %v", c.typ, err)
		}
		if got := paramConv(ty, c.scope); got != c.want {
			t.Errorf("paramConv(%q in %q) = %v, want %v", c.typ, c.scope, got, c.want)
		}
	}
}

func sigs(t *testing.T, scope string, texts ...string) []calleeSig {
	t.Helper()
	var out []calleeSig
	for _, s := range texts {
		ty, err := ctype.Parse(s)
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		out = append(out, calleeSig{ty, scope})
	}
	return out
}

func TestSigConvs(t *testing.T) {
	for _, c := range []struct {
		name string
		sigs []calleeSig
		n    int
		want []argConv
	}{
		{"puts", sigs(t, "", "int (const char *)"), 1, []argConv{convCStr}},
		{"strstr: char* next to const char*", sigs(t, "", "char *(char *, const char *)", "const char *(const char *, const char *)"), 2, []argConv{convCStr, convCStr}},
		{"printf", sigs(t, "", "int (const char *, ...)"), 3, []argConv{convCStr, convCStr, convCStr}},
		{"stoi: string and wstring", sigs(t, "std", "int (const string &, unsigned long *, int)", "int (const wstring &, unsigned long *, int)"), 1, []argConv{convStdString}},
		{"mixed kinds are C++'s", sigs(t, "", "const char *(const char *)", "const char *(std::string_view)"), 1, []argConv{convNone}},
		{"a class could accept it", sigs(t, "", "void (const char *)", "void (Widget)"), 1, []argConv{convNone}},
		{"numbers only", sigs(t, "", "int (int)", "int (double)"), 1, []argConv{convNone}},
		{"a longer overload is not viable", sigs(t, "", "void (const char *)", "void (int, int)"), 1, []argConv{convCStr}},
		{"std::string next to const char*", sigs(t, "", "void (const std::string &)", "void (const char *)"), 1, []argConv{convNone}},
	} {
		got := sigConvs(c.sigs, c.n)
		if len(got) != len(c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: argument %d is %v, want %v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

func TestKeepsPointer(t *testing.T) {
	text := func(args ...string) func(int) string { return func(n int) string { return args[n] } }
	for _, c := range []struct {
		name string
		args []string
		want bool
	}{
		{"puts", []string{"s"}, false},
		{"fopen", []string{"p", "m"}, false},
		{"strtok", []string{"s", "d"}, false}, // a char* parameter: a Go string never converts to it
		{"openlog", []string{"s", "o", "f"}, true},
		{"std::getopt", []string{"n", "a", "s"}, true},
		{"getopt_long", []string{"n", "a", "s", "l", "i"}, true},
		{"getopt_long_only", []string{"n", "a", "s", "l", "i"}, true},
		{"InitWindow", []string{"1", "2", "t"}, true},
		{"SetWindowTitle", []string{"t"}, true},
		{"SDL_SetWindowTitle", []string{"w", "t"}, false},
		{"sqlite3_bind_text", []string{"st", "1", "s", "-1", "sqlite3.SQLITE_TRANSIENT"}, false},
		{"sqlite3_bind_text", []string{"st", "1", "s", "-1", "sqlite3.SQLITE_STATIC"}, true},
		{"sqlite3_bind_text", []string{"st", "1", "s", "-1", "nil"}, true},
		{"sqlite3_bind_text", []string{"st", "1", "s", "-1"}, false},
		{"sqlite3_bind_text64", []string{"st", "1", "s", "-1", "d", "1"}, true},
		{"sqlite3_bind_blob", []string{"st", "1", "s", "-1", "sqlite3.SQLITE_STATIC"}, true},
		{"sqlite3_result_text", []string{"ctx", "s", "-1", "sqlite3.SQLITE_STATIC"}, true},
		{"sqlite3_result_text", []string{"ctx", "s", "-1", "sqlite3.SQLITE_TRANSIENT"}, false},
		{"sqlite3_result_text16", []string{"ctx", "s", "-1", "free"}, true},
		{"sqlite3_result_text64", []string{"ctx", "s", "-1", "free", "1"}, true},
		{"sqlite3_result_blob", []string{"ctx", "s", "-1", "sqlite3.SQLITE_STATIC"}, true},
		{"curl_easy_setopt", []string{"h", "curl.CURLOPT_POSTFIELDS", "s"}, true},
		{"curl_easy_setopt", []string{"h", "curl.CURLOPT_URL", "s"}, false},
		{"lib_set_name_static", []string{"o", "s"}, true}, // the convention
		{"lib_borrow_name", []string{"o", "s"}, true},
		{"lib_nocopy_name", []string{"o", "s"}, true},
		{"lib_noalloc_name", []string{"o", "s"}, true},
		{"lib_unowned_name", []string{"o", "s"}, true},
		{"lib_set_name", []string{"o", "s"}, false},
	} {
		_, got := keepsPointer(c.name, len(c.args), text(c.args...))
		if got != c.want {
			t.Errorf("keepsPointer(%s %v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
	// The functions whose result points into a string argument, each by name.
	for _, f := range []string{"strstr", "strchr", "strrchr", "strpbrk", "strcasestr", "index", "rindex", "basename",
		"dirname", "memchr", "memmem", "strchrnul", "strptime"} {
		if !aliasFuncs[f] {
			t.Errorf("%s returns a pointer into its argument", f)
		}
	}
	if len(aliasFuncs) != 13 {
		t.Errorf("aliasFuncs has %d names, the test knows 13", len(aliasFuncs))
	}
}

func TestSameScalar(t *testing.T) {
	i := types.Typ
	for _, c := range []struct {
		a, b types.Type
		want bool
	}{
		{i[types.Int], i[types.Int64], true},
		{i[types.Int32], i[types.Int], false},
		{i[types.Uint], i[types.Int], false},
		{i[types.Uint64], i[types.Uint], true},
		{i[types.Float64], i[types.Float32], false},
		{i[types.Float64], i[types.Int64], false},
		{i[types.Bool], i[types.Bool], true},
		{i[types.Bool], i[types.Int], false},
		{i[types.Uint8], i[types.Int8], false},
	} {
		if got := sameScalar(c.a, c.b); got != c.want {
			t.Errorf("sameScalar(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
