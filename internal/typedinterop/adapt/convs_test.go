package adapt

import (
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
)

// The decisions of the table that need no compiler (ADR-087): which parameter kinds take a gocpp
// string, when a set of overloads agrees, which functions keep a pointer.

func TestParamConv(t *testing.T) {
	for _, c := range []struct {
		typ, scope string
		want       Conv
	}{
		{"const char *", "", ConvCStr},
		{"char *", "", ConvNoStr},
		{"const unsigned char *", "", ConvNoStr},
		{"int", "", ConvNoStr},
		{"const int *", "", ConvNoStr},
		{"std::string", "", ConvStdString},
		{"const std::string &", "", ConvStdString},
		{"std::string &&", "", ConvStdString},
		{"std::string &", "", ConvNone},                // an lvalue string: no temporary can bind
		{"std::basic_string<char>", "", ConvStdString}, // as clang prints it
		{"const std::wstring &", "", ConvNoStr},        // no conversion from a gocpp string
		{"std::basic_string<wchar_t>", "", ConvNoStr},  //
		{"string", "std", ConvStdString},               // relative, inside std
		{"string", "", ConvNone},                       // a user's own string
		{"const string &", "myns", ConvNone},           //
		{"std::string_view", "", ConvNone},             // converts by itself, in opaque mode too
		{"Widget", "", ConvNone},                       // could have a constructor: unknown
		{"const Widget &", "", ConvNone},               //
		{"int (*)(int)", "", ConvNoStr},                // a function pointer
		{"int [4]", "", ConvNoStr},                     // an array
		{"const char *const *", "", ConvNoStr},         // a pointer to pointers
		{"const char *const", "", ConvCStr},            // the top-level const of the pointer
	} {
		ty, err := ctype.Parse(c.typ)
		if err != nil {
			t.Fatalf("%s: %v", c.typ, err)
		}
		if got := ParamConv(ty, c.scope); got != c.want {
			t.Errorf("ParamConv(%q in %q) = %v, want %v", c.typ, c.scope, got, c.want)
		}
	}
}

func sigs(t *testing.T, scope string, texts ...string) []Sig {
	t.Helper()
	var out []Sig
	for _, s := range texts {
		ty, err := ctype.Parse(s)
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		out = append(out, Sig{T: ty, Scope: scope})
	}
	return out
}

func TestSigConvs(t *testing.T) {
	for _, c := range []struct {
		name string
		sigs []Sig
		n    int
		want []Conv
	}{
		{"puts", sigs(t, "", "int (const char *)"), 1, []Conv{ConvCStr}},
		{"strstr: char* next to const char*", sigs(t, "", "char *(char *, const char *)", "const char *(const char *, const char *)"), 2, []Conv{ConvCStr, ConvCStr}},
		{"printf", sigs(t, "", "int (const char *, ...)"), 3, []Conv{ConvCStr, ConvCStr, ConvCStr}},
		{"stoi: string and wstring", sigs(t, "std", "int (const string &, unsigned long *, int)", "int (const wstring &, unsigned long *, int)"), 1, []Conv{ConvStdString}},
		{"mixed kinds are C++'s", sigs(t, "", "const char *(const char *)", "const char *(std::string_view)"), 1, []Conv{ConvNone}},
		{"a class could accept it", sigs(t, "", "void (const char *)", "void (Widget)"), 1, []Conv{ConvNone}},
		{"numbers only", sigs(t, "", "int (int)", "int (double)"), 1, []Conv{ConvNone}},
		{"a longer overload is not viable", sigs(t, "", "void (const char *)", "void (int, int)"), 1, []Conv{ConvCStr}},
		{"std::string next to const char*", sigs(t, "", "void (const std::string &)", "void (const char *)"), 1, []Conv{ConvNone}},
	} {
		got := SigConvs(c.sigs, c.n)
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
		_, got := KeepsPointer(c.name, len(c.args), text(c.args...))
		if got != c.want {
			t.Errorf("KeepsPointer(%s %v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
	// The functions whose result points into a string argument, each by name.
	for _, f := range []string{"strstr", "strchr", "strrchr", "strpbrk", "strcasestr", "index", "rindex", "basename",
		"dirname", "memchr", "memmem", "strchrnul", "strptime", "wcschr", "wcsrchr", "wcsstr", "wcspbrk", "wmemchr"} {
		if !aliasFuncs[f] {
			t.Errorf("%s returns a pointer into its argument", f)
		}
	}
	if len(aliasFuncs) != 18 {
		t.Errorf("aliasFuncs has %d names, the test knows 18", len(aliasFuncs))
	}
}
