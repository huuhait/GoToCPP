package lower

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/load"
)

// externFilesCPP lowers the program (several packages) and returns the header and the units of
// each package, by import path.
func externFilesCPP(t *testing.T, sources map[string]string) (map[string]string, error) {
	t.Helper()
	files, err := borrowForeignProgram(t, sources).Lower()
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, file := range files {
		var b strings.Builder
		b.Write(cppir.Print(file.Header, ""))
		for _, unit := range file.Units {
			b.Write(cppir.Print(unit.File, ""))
		}
		out[file.ImportPath] = b.String()
	}
	return out, nil
}

const externMain = `package main

import (
	"unsafe"

	"<vector>"

	"example.com/m/lib"
)

//gocpp:extern C
func add3(a, b, c C.int) C.int

//gocpp:extern C
func fill(buf *C.char, n C.size_t, raw unsafe.Pointer, out **C.char)

//gocpp:extern shapes::geom::area
func area(w, h float64) float64

//gocpp:extern
func total(xs *std.vector[C.int], flag bool, small uint8) int32

//gocpp:extern ::sum
func sumAll(xs *int64)

func main() {
	var buf *C.char
	var raw unsafe.Pointer
	_ = add3(1, 2, 3)
	fill(buf, 8, raw, &buf)
	_ = area(2, 3)
	_ = total(nil, true, 1)
	_ = lib.Twice(4)
}
`

const externLib = `package lib

//gocpp:extern C
func Twice(x C.int) C.int
`

// TestExternLowering: the header declares each extern function in the global namespace with the
// C++ types of the signature (raw pointers, void*, fixed-width numbers), a namespace as nested
// namespaces, and every call, also from another package, is a call of the C++ name.
func TestExternLowering(t *testing.T) {
	cpp, err := externFilesCPP(t, map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"main.gp":    externMain,
		"lib/lib.gp": externLib,
	})
	if err != nil {
		t.Fatal(err)
	}
	mainCPP := cpp["example.com/m"]
	for name, want := range map[string]string{
		"C function":           "extern \"C\" int add3(int, int, int);",
		"raw pointers":         "extern \"C\" void fill(char*, size_t, void*, char**);",
		"namespace":            "namespace shapes {\nnamespace geom {\ndouble area(double, double);\n} // namespace geom\n} // namespace shapes",
		"C++ template pointer": "int32_t total(::std::vector<int>*, bool, uint8_t);",
		"global name":          "void sum(int64_t*);",
		"call C":               "::add3(1, 2, 3)",
		"call with raw":        "::fill(gp::cgo::raw(",
		"out pointer":          "gp::cgo::out_ptr(buf)",
		"call namespace":       "::shapes::geom::area(2.0, 3.0)",
		"call C++":             "::total(nullptr, true, uint8_t(1))",
		"call other package":   "::Twice(4)",
	} {
		if !strings.Contains(mainCPP, want) {
			t.Errorf("%s: missing %q in\n%s", name, want, mainCPP)
		}
	}
	// The header includes the header of the signature's type before the declarations.
	if include, decl := strings.Index(mainCPP, "#include <vector>"), strings.Index(mainCPP, "int32_t total("); include < 0 || include > decl {
		t.Errorf("the header must include <vector> before the declaration of total:\n%s", mainCPP)
	}
	// The declaration comes before the package's namespace, and a function with no use in
	// another package is declared in its own package only.
	if strings.Index(mainCPP, "extern \"C\" int add3") > strings.Index(mainCPP, "namespace gpkg_main") {
		t.Errorf("the declarations must come before the package's namespace:\n%s", mainCPP)
	}
	if want := "extern \"C\" int Twice(int);"; !strings.Contains(cpp["example.com/m/lib"], want) {
		t.Errorf("lib: missing %q in\n%s", want, cpp["example.com/m/lib"])
	}
	if strings.Contains(cpp["example.com/m/lib"], "add3") {
		t.Errorf("lib declares a function of main:\n%s", cpp["example.com/m/lib"])
	}
}

// TestExternDeclaredTwice: two declarations of one C++ function (in one package or in two) must
// agree; C++ overloads (other parameters) are other functions.
func TestExternDeclaredTwice(t *testing.T) {
	const lib = `package lib

//gocpp:extern C
func Twice(x C.int) C.int

//gocpp:extern C
func Same(x C.int) C.int
`
	tests := []struct {
		name string
		main string
		want string // part of the error; "" for none
	}{
		{
			name: "the same C declaration in two packages",
			main: "//gocpp:extern C Twice\nfunc twice(x C.int) C.int\n",
		},
		{
			name: "another result",
			main: "//gocpp:extern C Twice\nfunc twice(x C.int) C.long\n",
			want: "extern function Twice is declared with the C signature long Twice(int), and elsewhere with another one, int Twice(int); the other declaration is at ",
		},
		{
			name: "another parameter",
			main: "//gocpp:extern C Twice\nfunc twice(x C.double) C.int\n",
			want: "extern function Twice is declared with the C signature int Twice(double), and elsewhere with another one, int Twice(int)",
		},
		{
			name: "C and C++ with the same parameters",
			main: "//gocpp:extern Twice\nfunc twice(x C.int) C.int\n",
			want: "extern function Twice is declared with C linkage and with C++ linkage, with the same parameters (int Twice(int))",
		},
		{
			name: "C and C++ overloads differ in the parameters",
			main: "//gocpp:extern Twice\nfunc twice(x C.double) C.double\n",
		},
		{
			name: "int32_t and int are the same type, spelled twice",
			main: "//gocpp:extern C Twice\nfunc twice(x int32) C.int\n",
			want: "write both declarations alike",
		},
		{
			name: "C++ overloads",
			main: "//gocpp:extern over\nfunc overInt(x C.int) C.int\n\n//gocpp:extern over\nfunc overDouble(x C.double) C.double\n",
		},
		{
			name: "C++ with the same parameters and another result",
			main: "//gocpp:extern over\nfunc overA(x C.int) C.int\n\n//gocpp:extern over\nfunc overB(x C.int) C.double\n",
			want: "extern function over is declared with the result double, and elsewhere with the result int for the same parameters",
		},
		{
			name: "in the same package, the same declaration under two Go names",
			main: "//gocpp:extern C Same\nfunc same1(x C.int) C.int\n\n//gocpp:extern C Same\nfunc same2(x C.int) C.int\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "package main\n\nimport \"example.com/m/lib\"\n\n" + tt.main + "\nfunc main() { _ = lib.Twice(1); _ = lib.Same(1) }\n"
			_, err := externFilesCPP(t, map[string]string{
				"gocpp.mod":  "module example.com/m\n",
				"main.gp":    src,
				"lib/lib.gp": lib,
			})
			switch {
			case tt.want == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case tt.want != "" && err == nil:
				t.Errorf("no error, want one with %q", tt.want)
			case tt.want != "" && !strings.Contains(err.Error(), tt.want):
				t.Errorf("error %q, want one with %q", err, tt.want)
			}
		})
	}
}

// cxx20Keywords is the table of keywords and alternative tokens of the C++20 standard
// ([lex.key], [lex.digraph]), written out again here so that a word missing from load.CxxKeywords
// is found.
var cxx20Keywords = []string{
	"alignas", "alignof", "asm", "auto", "bool", "break", "case", "catch", "char", "char8_t",
	"char16_t", "char32_t", "class", "concept", "const", "consteval", "constexpr", "constinit",
	"const_cast", "continue", "co_await", "co_return", "co_yield", "decltype", "default", "delete",
	"do", "double", "dynamic_cast", "else", "enum", "explicit", "export", "extern", "false",
	"float", "for", "friend", "goto", "if", "inline", "int", "long", "mutable", "namespace",
	"new", "noexcept", "nullptr", "operator", "private", "protected", "public", "register",
	"reinterpret_cast", "requires", "return", "short", "signed", "sizeof", "static",
	"static_assert", "static_cast", "struct", "switch", "template", "this", "thread_local",
	"throw", "true", "try", "typedef", "typeid", "typename", "union", "unsigned", "using",
	"virtual", "void", "volatile", "wchar_t", "while",
	"and", "and_eq", "bitand", "bitor", "compl", "not", "not_eq", "or", "or_eq", "xor", "xor_eq",
}

// TestCxxKeywordsAreComplete: every keyword of C++20 is refused as an extern name.
func TestCxxKeywordsAreComplete(t *testing.T) {
	for _, word := range cxx20Keywords {
		if !slices.Contains(load.CxxKeywords, word) {
			t.Errorf("%s is a C++20 keyword and is missing from load.CxxKeywords", word)
		}
	}
}

// TestCxxKeywordsAreOnlyKeywords: load.CxxKeywords holds no word that is not a C++20 keyword (an
// extern function named like a C function would be refused).
func TestCxxKeywordsAreOnlyKeywords(t *testing.T) {
	for _, word := range load.CxxKeywords {
		if !slices.Contains(cxx20Keywords, word) {
			t.Errorf("%s is in load.CxxKeywords and is not a C++20 keyword", word)
		}
	}
}

// TestCxxKeywordsAreReserved: the keywords that the loader refuses as extern names are the
// words that gocpp suffixes with _ in the names it writes (ADR-010).
func TestCxxKeywordsAreReserved(t *testing.T) {
	for _, word := range load.CxxKeywords {
		if !reserved[word] {
			t.Errorf("%s is a C++ keyword of load.CxxKeywords, and not a reserved word of names.go", word)
		}
	}
}

// TestMissingBody: a function or method without a body that gocpp check let through (a bindgen
// view, or a check told to accept it) stops the build with a message for the user, not with an
// internal error.
func TestMissingBody(t *testing.T) {
	const marker = "// Code generated by gocpp bindgen from <lib.h>. DO NOT EDIT.\n\n"
	const plain = "missing function body (to declare a function that C or C++ defines, " +
		"put //gocpp:extern in its doc comment)"
	const view = "missing function body (a gocpp bindgen view is for reading; it is not built)"
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name:  "a function of a bindgen view",
			files: map[string]string{"lib.gp": marker + "package main\n\nfunc add(a C.int, b C.int) C.int\n"},
			want:  "lib.gp:5:6: " + view,
		},
		{
			name:  "a method of a bindgen view",
			files: map[string]string{"lib.gp": marker + "package main\n\ntype T struct{}\n\nfunc (T) add()\n"},
			want:  "lib.gp:7:10: " + view,
		},
		{
			name:  "a method of a generic type of a bindgen view",
			files: map[string]string{"lib.gp": marker + "package main\n\ntype T[P any] struct{}\n\nfunc (T[P]) add()\n"},
			want:  "lib.gp:7:13: " + view,
		},
		{
			name:  "a function of a file that is no view",
			files: map[string]string{"lib.gp": "package main\n\nfunc add(a int32, b int32) int32\n"},
			want:  "lib.gp:3:6: " + plain,
		},
		{
			name:  "a method of a file that is no view",
			files: map[string]string{"lib.gp": "package main\n\ntype T struct{}\n\nfunc (*T) add()\n"},
			want:  "lib.gp:5:11: " + plain,
		},
		{
			name: "a function of a file that is no view, next to a view",
			files: map[string]string{
				"aa.gp": "package main\n\nfunc add()\n",
				"zz.gp": marker + "package main\n\nfunc viewOnly()\n",
			},
			want: "aa.gp:3:6: " + plain,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources := map[string]string{
				"gocpp.mod": "module example.com/m\n",
				"main.gp":   "package main\n\nfunc main() {}\n",
			}
			for name, source := range tt.files {
				sources[name] = source
			}
			// The check accepts a bindgen view; for the other files, it is told to accept them.
			program := borrowForeignProgramWith(t, sources, func(config *check.Config) {
				config.AllowBodyless = true
			})
			_, err := program.Lower()
			var lowered *Error
			if !errors.As(err, &lowered) {
				t.Fatalf("error %v, want a lower.Error", err)
			}
			if !strings.HasSuffix(lowered.Error(), tt.want) {
				t.Errorf("error %q, want one ending %q", lowered, tt.want)
			}
			if !lowered.User {
				t.Errorf("error %q is not marked as the user's (it would print as an internal error)", lowered)
			}
		})
	}
}
