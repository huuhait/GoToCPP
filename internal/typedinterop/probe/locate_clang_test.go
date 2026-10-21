package probe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The tests of Locate against the real compiler: testdata/locate/api.hpp is the header, and
// every answer is checked against the text of the file.

var locateIncludes = []string{`"api.hpp"`}

func locateFixture(t *testing.T) (dir, header string) {
	t.Helper()
	dir = t.TempDir()
	data, err := os.ReadFile("testdata/locate/api.hpp")
	if err != nil {
		t.Fatal(err)
	}
	writeHeader(t, dir, "api.hpp", string(data))
	return dir, string(data)
}

func locateReal(t *testing.T, dir string, includes []string, request LocateRequest,
	flags ...string,
) *Locations {
	t.Helper()
	cfg := clangConfig(t, dir, flags...)
	cfg.CacheDir = "off"
	got, err := new(Runner).Locate(context.Background(), cfg, includes, request)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// headerLine returns the 0-based line of the first line of text that contains needle.
func headerLine(t *testing.T, text, needle string) int {
	t.Helper()
	for n, line := range strings.Split(text, "\n") {
		if strings.Contains(line, needle) {
			return n
		}
	}
	t.Fatalf("no line with %q", needle)
	return -1
}

func locateNames(scope string, list ...string) []LocateName {
	out := make([]LocateName, 0, len(list))
	for _, name := range list {
		out = append(out, LocateName{Scope: scope, Name: name})
	}
	return out
}

func onlyDecl(t *testing.T, decls []Declaration) Declaration {
	t.Helper()
	if len(decls) != 1 {
		t.Fatalf("declarations = %+v, want exactly one", decls)
	}
	return decls[0]
}

func TestLocateOverloadSetIsCompleteAndSorted(t *testing.T) {
	dir, text := locateFixture(t)
	got := locateReal(t, dir, locateIncludes, LocateRequest{Names: locateNames("", "add")})
	decls := got.Named("", "add")
	if len(decls) != 2 {
		t.Fatalf("add = %+v, want both overloads", decls)
	}
	wantFile := filepath.Join(dir, "api.hpp")
	for i, want := range []struct {
		line      int
		signature string
	}{
		{headerLine(t, text, "int add(int a, int b = 2);"), "int add(int a, int b = 2)"},
		{headerLine(t, text, "int add(double a);"), "int add(double a)"},
	} {
		d := decls[i]
		if d.File != wantFile || d.Line != want.line || d.Col != 4 || d.Length != 3 ||
			d.Signature != want.signature || d.Kind != "function" {
			t.Errorf("overload %d = %+v, want %s at line %d", i, d, want.signature, want.line)
		}
	}
}

func TestLocateDocComments(t *testing.T) {
	dir, _ := locateFixture(t)
	got := locateReal(t, dir, locateIncludes, LocateRequest{
		Names:   locateNames("", "add", "InitWindow", "Widget", "Vector2"),
		Members: []LocateMembers{{Receiver: "Widget", Names: []string{"w"}}},
	})
	t.Run("doxygen lines", func(t *testing.T) {
		first := got.Named("", "add")[0]
		if first.Doc != "Adds two numbers.\nSecond line." {
			t.Errorf("doc = %q", first.Doc)
		}
	})
	t.Run("a group heading above does not beat a trailing comment", func(t *testing.T) {
		d := onlyDecl(t, got.Named("", "InitWindow"))
		if d.Doc != "Initialize window" {
			t.Errorf("doc = %q, want the trailing comment, not the group heading", d.Doc)
		}
	})
	t.Run("plain trailing comment of the second overload", func(t *testing.T) {
		if doc := got.Named("", "add")[1].Doc; doc != "the double overload" {
			t.Errorf("doc = %q", doc)
		}
	})
	t.Run("plain block comment above", func(t *testing.T) {
		if doc := onlyDecl(t, got.Named("", "Widget")).Doc; doc != "block doc" {
			t.Errorf("doc = %q", doc)
		}
	})
	t.Run("trailing doxygen of a member", func(t *testing.T) {
		if doc := onlyDecl(t, got.Member("Widget", "w")).Doc; doc != "the width" {
			t.Errorf("doc = %q", doc)
		}
	})
	t.Run("no comment, no doc", func(t *testing.T) {
		if doc := onlyDecl(t, got.Named("", "Vector2")).Doc; doc != "" {
			t.Errorf("doc = %q, want none for a declaration whose comments belong to its fields", doc)
		}
	})
}

func TestLocateColumnsAreUTF16(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "wide.hpp", "/* é\U0001F600 */ int wide(int);\nint éx; int after;\n")
	got := locateReal(t, dir, []string{`"wide.hpp"`}, LocateRequest{Names: locateNames("", "wide", "after")})
	d := onlyDecl(t, got.Named("", "wide"))
	// "/* " is 3 units, e-acute 1, the emoji 2 (a surrogate pair, 4 bytes), " */ int " is 8.
	if d.Col != 14 || d.Line != 0 {
		t.Errorf("wide at %d:%d, want 0:14", d.Line, d.Col)
	}
	after := onlyDecl(t, got.Named("", "after"))
	// "int " 4, e-acute and x 2 (two bytes then one), "; int " 6.
	if after.Col != 12 || after.Line != 1 {
		t.Errorf("after at %d:%d, want 1:12", after.Line, after.Col)
	}
}

func TestLocateMacros(t *testing.T) {
	dir, text := locateFixture(t)
	got := locateReal(t, dir, locateIncludes, LocateRequest{Names: []LocateName{
		{Name: "PI", Macro: true}, {Name: "MAX", Macro: true}, {Name: "NOSUCH", Macro: true},
	}})
	pi := onlyDecl(t, got.Named("", "PI"))
	if pi.Kind != "macro" || pi.Line != headerLine(t, text, "#define PI") || pi.Col != 8 ||
		pi.Doc != "Pi" || !strings.HasPrefix(pi.Signature, "#define PI 3.14159") {
		t.Errorf("PI = %+v", pi)
	}
	max := onlyDecl(t, got.Named("", "MAX"))
	if max.Doc != "Maximum of two." || !strings.HasPrefix(max.Signature, "#define MAX(a") {
		t.Errorf("MAX = %+v", max)
	}
	if found := got.Named("", "NOSUCH"); len(found) != 0 {
		t.Errorf("an undefined macro = %+v, want nothing", found)
	}
}

func TestLocateKinds(t *testing.T) {
	dir, text := locateFixture(t)
	got := locateReal(t, dir, locateIncludes, LocateRequest{Names: []LocateName{
		{Name: "Widget"}, {Name: "Derived"}, {Name: "Color"}, {Name: "RED"}, {Name: "counter"},
		{Scope: "geo", Name: "kMax"}, {Scope: "geo", Name: "Kind"}, {Scope: "geo", Name: "pick"},
		{Scope: "geo", Name: "W"}, {Name: "Name"}, {Name: "Vector2"},
	}})
	cases := []struct {
		scope, name, kind, signature string
		line                         int
	}{
		{"", "Derived", "type",
			"struct Derived : public Widget {\n    int own;\n    void tick(const char *label = \"x\", ...);\n}",
			headerLine(t, text, "struct Derived")},
		{"", "Color", "type", "enum Color {\n    RED,\n    GREEN = 5,\n}", headerLine(t, text, "enum Color")},
		{"", "RED", "constant", "Color RED", headerLine(t, text, "enum Color")},
		{"", "counter", "variable", "extern int counter", headerLine(t, text, "extern int counter")},
		{"geo", "kMax", "variable", "const int geo::kMax = 7", headerLine(t, text, "const int kMax")},
		{"geo", "Kind", "type", "enum class geo::Kind : unsigned char {\n    A,\n    B = 5,\n}",
			headerLine(t, text, "enum class Kind")},
		{"geo", "pick", "template", "template <class T, int N>\nT geo::pick(T first, int index = ...)",
			headerLine(t, text, "T pick(")},
		{"geo", "W", "type", "typedef Widget geo::W;", headerLine(t, text, "typedef Widget W")},
		{"", "Name", "type", "using Name = std::basic_string<char>;", headerLine(t, text, "using Name")},
		{"", "Vector2", "type", "typedef struct Vector2 Vector2;", headerLine(t, text, "} Vector2;")},
	}
	for _, want := range cases {
		t.Run(want.name, func(t *testing.T) {
			d := onlyDecl(t, got.Named(want.scope, want.name))
			if d.Kind != want.kind || d.Signature != want.signature || d.Line != want.line {
				t.Errorf("%s = %+v, want %s %q at line %d", want.name, d, want.kind,
					want.signature, want.line)
			}
		})
	}
	t.Run("only public members of a record", func(t *testing.T) {
		sig := onlyDecl(t, got.Named("", "Widget")).Signature
		if strings.Contains(sig, "hidden") || !strings.Contains(sig, "int size(int x) const;") {
			t.Errorf("Widget = %s", sig)
		}
	})
}

func TestLocateStandardLibraryNames(t *testing.T) {
	dir := t.TempDir()
	got := locateReal(t, dir, []string{"<string>", "<cstdio>"}, LocateRequest{Names: []LocateName{
		{Scope: "std", Name: "string"}, {Scope: "std", Name: "fopen"}, {Name: "fopen"},
	}})
	for _, query := range []LocateName{{Scope: "std", Name: "string"}, {Scope: "std", Name: "fopen"}} {
		d := onlyDecl(t, got.Named(query.Scope, query.Name))
		if !filepath.IsAbs(d.File) || filepath.Clean(d.File) != d.File {
			t.Errorf("%v file = %q, want an absolute normalized path", query, d.File)
		}
		text, err := os.ReadFile(d.File)
		if err != nil {
			t.Fatalf("%v: %v", query, err)
		}
		line := strings.Split(string(text), "\n")[d.Line]
		if !strings.HasPrefix(line[d.Col:], query.Name) {
			t.Errorf("%v at %d:%d does not start the name: %q", query, d.Line, d.Col, line)
		}
	}
	sig := onlyDecl(t, got.Named("std", "string")).Signature
	if !strings.Contains(sig, "std::basic_string<char>") {
		t.Errorf("std::string = %s", sig)
	}
	if onlyDecl(t, got.Named("std", "fopen")).Signature != onlyDecl(t, got.Named("", "fopen")).Signature {
		t.Error("std::fopen is the declaration of ::fopen that the using declaration names")
	}
}

func TestLocateMembers(t *testing.T) {
	dir, text := locateFixture(t)
	got := locateReal(t, dir, locateIncludes, LocateRequest{Members: []LocateMembers{
		{Receiver: "Widget", Names: []string{"size", "make", "hidden", "nosuch"}},
		{Receiver: "Derived", Names: []string{"size", "w", "own"}},
		{Receiver: "const Widget *", Names: []string{"w"}},
		{Receiver: "std::basic_string<char>", Names: []string{"push_back"}},
		{Receiver: "Sealed", Names: []string{"x"}},
	}})
	t.Run("overloads", func(t *testing.T) {
		decls := got.Member("Widget", "size")
		if len(decls) != 2 || decls[0].Signature != "int Widget::size() const" ||
			decls[1].Signature != "int Widget::size(int x) const" {
			t.Fatalf("size = %+v", decls)
		}
		if decls[0].Line != headerLine(t, text, "int size() const;") || decls[0].Col != 6 {
			t.Errorf("size at %d:%d", decls[0].Line, decls[0].Col)
		}
	})
	t.Run("static method", func(t *testing.T) {
		got := onlyDecl(t, got.Member("Widget", "make"))
		if got.Kind != "method" || got.Signature != "static Widget Widget::make()" {
			t.Errorf("make = %+v", got)
		}
	})
	t.Run("inherited member points into the base", func(t *testing.T) {
		d := onlyDecl(t, got.Member("Derived", "w"))
		if d.Line != headerLine(t, text, "int w;") || d.Doc != "the width" {
			t.Errorf("Derived.w = %+v", d)
		}
	})
	t.Run("own member of the derived class", func(t *testing.T) {
		if d := onlyDecl(t, got.Member("Derived", "own")); d.Line != headerLine(t, text, "int own;") {
			t.Errorf("Derived.own = %+v", d)
		}
	})
	t.Run("a pointer to const receiver is looked up as its class", func(t *testing.T) {
		if d := onlyDecl(t, got.Member("const Widget *", "w")); d.Line != headerLine(t, text, "int w;") {
			t.Errorf("w = %+v", d)
		}
	})
	t.Run("template receiver finds the instantiated member in the library header", func(t *testing.T) {
		d := onlyDecl(t, got.Member("std::basic_string<char>", "push_back"))
		if !strings.Contains(d.Signature, "push_back(char") || !filepath.IsAbs(d.File) {
			t.Errorf("push_back = %+v", d)
		}
	})
	t.Run("a name that is not a member has no answer", func(t *testing.T) {
		if found := got.Member("Widget", "nosuch"); len(found) != 0 {
			t.Errorf("nosuch = %+v", found)
		}
	})
	t.Run("a final class has no answer and does not fail the others", func(t *testing.T) {
		if found := got.Member("Sealed", "x"); len(found) != 0 {
			t.Errorf("Sealed.x = %+v, want none: a derived class cannot be written", found)
		}
		if len(got.Member("Widget", "size")) != 2 {
			t.Error("the other groups lost their answers")
		}
	})
}

func TestLocateRefusesWhatItCannotPlace(t *testing.T) {
	dir, _ := locateFixture(t)
	got := locateReal(t, dir, locateIncludes, LocateRequest{Names: []LocateName{
		{Name: "make_one"}, {Name: "geo"}, {Name: "nosuch"}, {Name: "gocpp_loc_end"}, {Name: "bad name"},
	}})
	if got.Len() != 0 {
		t.Errorf("answers = %+v, want none: a name made by a macro, a namespace, an unknown name and a "+
			"reserved name", got.named)
	}
}

func TestLocateHeaderPaths(t *testing.T) {
	t.Run("spaces and commas in the folder name", func(t *testing.T) {
		dir := t.TempDir()
		folder := filepath.Join(dir, "my headers, v2")
		if err := os.Mkdir(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		writeHeader(t, folder, "api.hpp", "int spaced(int);\n")
		got := locateReal(t, dir, []string{`"my headers, v2/api.hpp"`},
			LocateRequest{Names: locateNames("", "spaced")})
		if d := onlyDecl(t, got.Named("", "spaced")); d.File != filepath.Join(folder, "api.hpp") {
			t.Errorf("file = %q", d.File)
		}
	})
	t.Run("an include folder relative to the working folder", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "inc"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeHeader(t, filepath.Join(dir, "inc"), "api.hpp", "int relative(int);\n")
		got := locateReal(t, dir, []string{"<api.hpp>"},
			LocateRequest{Names: locateNames("", "relative")}, "-Iinc")
		if d := onlyDecl(t, got.Named("", "relative")); d.File != filepath.Join(dir, "inc", "api.hpp") {
			t.Errorf("file = %q, want the absolute path", d.File)
		}
	})
	t.Run("a header that includes another with a dot-dot path", func(t *testing.T) {
		dir := t.TempDir()
		for _, sub := range []string{"a", "b"} {
			if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		writeHeader(t, filepath.Join(dir, "b"), "inner.hpp", "int inner(int);\n")
		writeHeader(t, filepath.Join(dir, "a"), "outer.hpp", "#include \"../b/inner.hpp\"\n")
		got := locateReal(t, dir, []string{`"a/outer.hpp"`}, LocateRequest{Names: locateNames("", "inner")})
		if d := onlyDecl(t, got.Named("", "inner")); d.File != filepath.Join(dir, "b", "inner.hpp") {
			t.Errorf("file = %q, want a cleaned path", d.File)
		}
	})
	t.Run("a header reached through a link is the path of the include", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "real"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeHeader(t, filepath.Join(dir, "real"), "api.hpp", "int linked(int);\n")
		if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link")); err != nil {
			t.Skip("no symbolic links:", err)
		}
		got := locateReal(t, dir, []string{"<api.hpp>"},
			LocateRequest{Names: locateNames("", "linked")}, "-Ilink")
		d := onlyDecl(t, got.Named("", "linked"))
		if d.File != filepath.Join(dir, "link", "api.hpp") {
			t.Errorf("file = %q", d.File)
		}
	})
}

func TestLocateNamesAreValidatedAndAskedOnce(t *testing.T) {
	asked := validLocateNames([]LocateName{
		{Name: "add"}, {Name: "add"}, {Scope: "geo", Name: "kMax"}, {Name: "bad name"},
		{Name: "gocpp_loc_end"}, {Scope: "a b", Name: "x"}, {Scope: "geo", Name: "kMax"},
		{Name: "PI", Macro: true},
	})
	want := []LocateName{{Name: "add"}, {Scope: "geo", Name: "kMax"}, {Name: "PI", Macro: true}}
	if !slices.Equal(asked, want) {
		t.Errorf("asked = %v, want %v", asked, want)
	}
	var many []LocateName
	for i := range maxLocateNames + 20 {
		many = append(many, LocateName{Name: fmt.Sprintf("n%d", i)})
	}
	if got := len(validLocateNames(many)); got != maxLocateNames {
		t.Errorf("names kept = %d, want the limit %d", got, maxLocateNames)
	}
}

func TestLocateOverloadSetWithAnUnplaceableMemberIsRefused(t *testing.T) {
	dir := t.TempDir()
	// The second overload's name is made by a token paste: it is spelled nowhere in the files.
	writeHeader(t, dir, "mix.hpp",
		"int mix(int);\n#define PASTE(a) a##ix\nint PASTE(m)(double);\nint clean(int);\n")
	got := locateReal(t, dir, []string{`"mix.hpp"`}, LocateRequest{Names: locateNames("", "mix", "clean")})
	if found := got.Named("", "mix"); len(found) != 0 {
		t.Errorf("mix = %+v, want no answer rather than a part of the overload set", found)
	}
	if len(got.Named("", "clean")) != 1 {
		t.Error("a name beside it lost its answer")
	}
}

func TestLocateHeaderWithAnErrorGivesNoAnswers(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "broken.hpp", "int fine(int);\nint broken(;\n")
	cfg := clangConfig(t, dir)
	cfg.CacheDir = "off"
	got, err := new(Runner).Locate(context.Background(), cfg, []string{`"broken.hpp"`},
		LocateRequest{Names: locateNames("", "fine")})
	if err == nil || got != nil {
		t.Fatalf("got %v, error %v, want an error: the compiler did not read the header as written", got, err)
	}
}

func TestLocateFinalBaseDoesNotLeakOuterNames(t *testing.T) {
	dir := t.TempDir()
	// The class derived from a final class loses its base: the lookup would find the global.
	writeHeader(t, dir, "leak.hpp",
		"int shared;\nstruct Sealed final { int shared; };\nstruct Open { int shared; };\n")
	got := locateReal(t, dir, []string{`"leak.hpp"`}, LocateRequest{Members: []LocateMembers{
		{Receiver: "Sealed", Names: []string{"shared"}},
		{Receiver: "Open", Names: []string{"shared"}},
	}})
	if found := got.Member("Sealed", "shared"); len(found) != 0 {
		t.Errorf("Sealed.shared = %+v, want no answer: the global is not the member", found)
	}
	if d := onlyDecl(t, got.Member("Open", "shared")); d.Line != 2 {
		t.Errorf("Open.shared = %+v", d)
	}
}

func TestLocateRedeclarationsAfterTheDefinition(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "fwd.hpp", strings.Join([]string{
		"struct Fwd { int z; };",
		"struct Fwd;",
		"template <typename T> struct Tpl { T v; };",
		"template <typename T> struct Tpl;",
		"struct Only;",
		"template <typename T, int = 3> struct OnlyT;",
		"enum class Opaque : int;",
		"enum class Defined : int { A };",
		"enum class Defined : int;",
		"struct Late;",
		"struct Late { int q; };",
		"template <typename T> struct LateT;",
		"template <typename T> struct LateT { T w; };",
		"int fn(int);",
		"int fn(int);",
		"",
	}, "\n"))
	got := locateReal(t, dir, []string{`"fwd.hpp"`}, LocateRequest{Names: locateNames("",
		"Fwd", "Tpl", "Only", "OnlyT", "Opaque", "Defined", "Late", "LateT", "fn")})
	for _, name := range []string{"Fwd", "Tpl", "Defined"} {
		t.Run("refused "+name, func(t *testing.T) {
			if found := got.Named("", name); len(found) != 0 {
				t.Errorf("%s = %+v, want no answer: a redeclaration is not the definition", name, found)
			}
		})
	}
	for _, c := range []struct {
		name, signature string
		line            int
	}{
		{"Only", "struct Only;", 4},
		{"OnlyT", "template <typename T, int>\nstruct OnlyT;", 5},
		{"Opaque", "enum class Opaque : int;", 6},
		{"Late", "struct Late {\n    int q;\n}", 10},
		{"LateT", "template <typename T>\nstruct LateT;", 12},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := onlyDecl(t, got.Named("", c.name))
			if d.Signature != c.signature || d.Line != c.line {
				t.Errorf("%s = %q at line %d, want %q at line %d", c.name, d.Signature, d.Line,
					c.signature, c.line)
			}
		})
	}
	t.Run("a function goes to the latest declaration", func(t *testing.T) {
		if d := onlyDecl(t, got.Named("", "fn")); d.Line != 14 {
			t.Errorf("fn = %+v", d)
		}
	})
}

func TestLocateForwardDeclarationOfALibraryTemplate(t *testing.T) {
	dir := t.TempDir()
	ask := LocateRequest{Names: []LocateName{{Scope: "std", Name: "vector"}}}
	alone := locateReal(t, dir, []string{"<vector>"}, ask)
	d := onlyDecl(t, alone.Named("std", "vector"))
	if d.Signature != "template <typename _Tp, typename _Alloc>\nclass std::vector;" ||
		!strings.HasSuffix(d.File, "stl_vector.h") {
		t.Errorf("std::vector = %+v", d)
	}
	// <format> declares vector again, after the definition: the lookup answers with that.
	both := locateReal(t, dir, []string{"<vector>", "<format>"}, ask, "-std=c++20")
	if found := both.Named("std", "vector"); len(found) != 0 {
		t.Errorf("std::vector with <format> = %+v, want no answer rather than the redeclaration", found)
	}
}

func TestLocateUnnamedTemplateParameters(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "tp.hpp", strings.Join([]string{
		"template <typename, int> struct Anon;",
		"template <class, class T, int N, int> struct Mixed;",
		"template <typename...> struct Pack;",
		"template <typename... Ts> struct NamedPack;",
		"template <template <typename> class> struct Template;",
		"",
	}, "\n"))
	got := locateReal(t, dir, []string{`"tp.hpp"`}, LocateRequest{Names: locateNames("",
		"Anon", "Mixed", "Pack", "NamedPack", "Template")})
	for name, want := range map[string]string{
		"Anon":      "template <typename, int>\nstruct Anon;",
		"Mixed":     "template <class, class T, int N, int>\nstruct Mixed;",
		"Pack":      "template <typename...>\nstruct Pack;",
		"NamedPack": "template <typename... Ts>\nstruct NamedPack;",
		"Template":  "template <template <...> class>\nstruct Template;",
	} {
		t.Run(name, func(t *testing.T) {
			if d := onlyDecl(t, got.Named("", name)); d.Signature != want {
				t.Errorf("%s = %q, want %q", name, d.Signature, want)
			}
		})
	}
}

func overloadHeader(count, params int) string {
	var b strings.Builder
	for i := range count {
		b.WriteString("void many(")
		for j := range params {
			fmt.Fprintf(&b, "int a%d, ", j)
		}
		fmt.Fprintf(&b, "char (*)[%d]);\n", i+1)
	}
	return b.String()
}

func TestLocateOverloadSetLimits(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "ok.hpp", overloadHeader(maxOverloads, 1))
	writeHeader(t, dir, "big.hpp", overloadHeader(maxOverloads+1, 1))
	writeHeader(t, dir, "huge.hpp", overloadHeader(1500, 12))
	for _, c := range []struct {
		header string
		want   int
	}{
		{"ok.hpp", maxOverloads}, // the largest set that is shown
		{"big.hpp", 0},           // too large to be a hover
		{"huge.hpp", 0},          // the dump itself was cut: the set is not complete
	} {
		t.Run(c.header, func(t *testing.T) {
			got := locateReal(t, dir, []string{`"` + c.header + `"`},
				LocateRequest{Names: locateNames("", "many")})
			if n := len(got.Named("", "many")); n != c.want {
				t.Errorf("overloads shown = %d, want %d", n, c.want)
			}
		})
	}
}

func TestLocateDocsOfLongComments(t *testing.T) {
	dir := t.TempDir()
	var text strings.Builder
	text.WriteString("/*\n")
	for i := range 60 {
		fmt.Fprintf(&text, "** line %d of a long comment\n", i)
	}
	text.WriteString("*/\nint longdoc(int);\n")
	writeHeader(t, dir, "long.hpp", text.String())
	got := locateReal(t, dir, []string{`"long.hpp"`}, LocateRequest{Names: locateNames("", "longdoc")})
	doc := onlyDecl(t, got.Named("", "longdoc")).Doc
	lines := strings.Split(doc, "\n")
	if len(lines) != maxDocLines+1 || lines[len(lines)-1] != "..." ||
		!strings.HasPrefix(lines[0], "line 0 of") {
		t.Errorf("doc = %d lines, first %q, last %q; want %d lines and the marker", len(lines), lines[0],
			lines[len(lines)-1], maxDocLines)
	}
}

func TestLocateDocsOfDeclarationsThatAnEmptyMacroStarts(t *testing.T) {
	dir := t.TempDir()
	var text strings.Builder
	text.WriteString("#define API\n/*\n")
	for i := range 60 {
		fmt.Fprintf(&text, "** line %d of a sqlite style comment\n", i)
	}
	text.WriteString("*/\nAPI int macrodoc(int);\n")
	text.WriteString("/* short */\nAPI int macroshort(int);\n")
	text.WriteString("// line one\n// line two\nAPI int macroslash(int);\n")
	text.WriteString("int macrocode(int); /* not mine */\nAPI int macrobelow(int);\n")
	writeHeader(t, dir, "macro.hpp", text.String())
	names := []string{"macrodoc", "macroshort", "macroslash", "macrobelow"}
	got := locateReal(t, dir, []string{`"macro.hpp"`}, LocateRequest{Names: locateNames("", names...)})
	t.Run("a long block comment is capped", func(t *testing.T) {
		lines := strings.Split(onlyDecl(t, got.Named("", "macrodoc")).Doc, "\n")
		if len(lines) != maxDocLines+1 || lines[len(lines)-1] != "..." ||
			!strings.HasPrefix(lines[0], "line 0 of") {
			t.Errorf("doc = %d lines, first %q, last %q", len(lines), lines[0], lines[len(lines)-1])
		}
	})
	t.Run("a short block comment", func(t *testing.T) {
		if doc := onlyDecl(t, got.Named("", "macroshort")).Doc; doc != "short" {
			t.Errorf("doc = %q", doc)
		}
	})
	t.Run("line comments", func(t *testing.T) {
		if doc := onlyDecl(t, got.Named("", "macroslash")).Doc; doc != "line one\nline two" {
			t.Errorf("doc = %q", doc)
		}
	})
	t.Run("the trailing comment of the line above is not a doc", func(t *testing.T) {
		if doc := onlyDecl(t, got.Named("", "macrobelow")).Doc; doc != "" {
			t.Errorf("doc = %q, want none", doc)
		}
	})
}

func TestLocateDocsOfSystemHeaders(t *testing.T) {
	dir := t.TempDir()
	system := filepath.Join(dir, "system")
	if err := os.MkdirAll(system, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "/**\n * Opens the thing.\n */\nint sysopen(int);\n" +
		"/*\n** sqlite style\n** second line\n**/\nint sysplain(int);\n"
	writeHeader(t, system, "sys.hpp", text)
	names := locateNames("", "sysopen", "sysplain")
	got := locateReal(t, dir, []string{"<sys.hpp>"}, LocateRequest{Names: names}, "-isystem", system)
	t.Run("a documentation comment", func(t *testing.T) {
		if doc := onlyDecl(t, got.Named("", "sysopen")).Doc; doc != "Opens the thing." {
			t.Errorf("doc = %q", doc)
		}
	})
	t.Run("a plain comment with the stars of sqlite", func(t *testing.T) {
		if doc := onlyDecl(t, got.Named("", "sysplain")).Doc; doc != "sqlite style\nsecond line" {
			t.Errorf("doc = %q", doc)
		}
	})
}

func TestLocateRedeclaredFunctionIsTheLatestDeclaration(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "rd.hpp",
		"/// Documented on the first.\nint rd(int);\n/// Documented on the second.\nint rd(int);\n")
	got := locateReal(t, dir, []string{`"rd.hpp"`}, LocateRequest{Names: locateNames("", "rd")})
	d := onlyDecl(t, got.Named("", "rd"))
	// The compiler's lookup has the latest declaration only, and the dump has the comment of the
	// declaration itself: the place and the comment are those of the same declaration.
	if d.Line != 3 || d.Doc != "Documented on the second." {
		t.Errorf("rd = %+v, want the latest declaration with its own comment", d)
	}
}

func TestLocateByteOrderMarkIsNotAColumn(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "bom.hpp", "\xef\xbb\xbfint bom1(int);\r\nint bom2(int);\r\n")
	got := locateReal(t, dir, []string{`"bom.hpp"`}, LocateRequest{Names: locateNames("", "bom1", "bom2")})
	if d := onlyDecl(t, got.Named("", "bom1")); d.Line != 0 || d.Col != 4 {
		t.Errorf("bom1 at %d:%d, want 0:4 as the editor's text without the mark has it", d.Line, d.Col)
	}
	if d := onlyDecl(t, got.Named("", "bom2")); d.Line != 1 || d.Col != 4 {
		t.Errorf("bom2 at %d:%d, want 1:4", d.Line, d.Col)
	}
}
