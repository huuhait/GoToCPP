package gocppmod

import (
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// recipeOf parses a gocpp.mod with one cxx block with the body lines and
// returns the recipe and the errors of its lines.
func recipeOf(t *testing.T, body ...string) (*Recipe, ErrorList) {
	t.Helper()
	src := "module example.com/app\n\ncxx example.com/lib {\n" + strings.Join(body, "\n") + "\n}\n"
	f, errs := Parse("gocpp.mod", []byte(src), nil)
	if len(errs) > 0 {
		t.Fatalf("gocpp.mod itself has errors: %v\n%s", errs, src)
	}
	blocks, rerrs := f.Recipes()
	if len(rerrs) > 0 || blocks["example.com/lib"] == nil {
		t.Fatalf("Recipes() = %v, %v", blocks, rerrs)
	}
	return ParseRecipe("gocpp.mod", "example.com/lib", blocks["example.com/lib"])
}

func TestRecipeStatements(t *testing.T) {
	r, errs := recipeOf(t,
		"\t// a comment",
		"\tsrc imgui.cpp  imgui_draw.cpp ./backends/*.cpp // trailing comment",
		"\texclude backends/*vulkan*",
		"\tinclude . backends",
		"\tsystem-include third_party/include",
		"\tdefine A B=2 C=\"x y\"",
		"\tprivate-define BUILDING=1",
		"\tcflags -O2 \"-DMSG=a b\"",
		"\tcxxflags -fno-rtti",
		"\tldflags -pthread",
		"\tlink m -lz",
		"\tpkg-config glfw3 gl",
		"\tstd c++17",
		"\tcstd gnu11",
		"",
		"\tlinux: src x11.cpp",
		"\tlinux,darwin: link dl",
		"\t!windows: define UNIX",
	)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	type want struct {
		line  int
		verb  string
		args  []string
		cond  string
		cText string
	}
	for i, w := range []want{
		{5, "src", []string{"imgui.cpp", "imgui_draw.cpp", "backends/*.cpp"}, "", ""},
		{6, "exclude", []string{"backends/*vulkan*"}, "", ""},
		{7, "include", []string{".", "backends"}, "", ""},
		{8, "system-include", []string{"third_party/include"}, "", ""},
		{9, "define", []string{"A", "B=2", "C=x y"}, "", ""},
		{10, "private-define", []string{"BUILDING=1"}, "", ""},
		{11, "cflags", []string{"-O2", "-DMSG=a b"}, "", ""},
		{12, "cxxflags", []string{"-fno-rtti"}, "", ""},
		{13, "ldflags", []string{"-pthread"}, "", ""},
		{14, "link", []string{"-lm", "-lz"}, "", ""},
		{15, "pkg-config", []string{"glfw3", "gl"}, "", ""},
		{16, "std", []string{"c++17"}, "", ""},
		{17, "cstd", []string{"gnu11"}, "", ""},
		{19, "src", []string{"x11.cpp"}, "linux:", ""},
		{20, "link", []string{"-ldl"}, "linux,darwin:", ""},
		{21, "define", []string{"UNIX"}, "!windows:", ""},
	} {
		if i >= len(r.Stmts) {
			t.Fatalf("only %d statements: %+v", len(r.Stmts), r.Stmts)
		}
		s := r.Stmts[i]
		if s.Line != w.line || s.Verb != w.verb || !reflect.DeepEqual(s.Args, w.args) || s.Cond.String() != w.cond {
			t.Errorf("statement %d = line %d %s %q %q, want line %d %s %q %q", i, s.Line, s.Verb, s.Args, s.Cond, w.line, w.verb, w.args, w.cond)
		}
	}
	if len(r.Stmts) != 16 {
		t.Errorf("%d statements, want 16", len(r.Stmts))
	}
	if got := r.Stmts[0].Text; got != "src imgui.cpp  imgui_draw.cpp ./backends/*.cpp" {
		t.Errorf("Text = %q (the comment and the indentation are dropped)", got)
	}
}

func TestRecipeConditions(t *testing.T) {
	for _, tc := range []struct {
		cond string
		os   map[string]bool
	}{
		{"linux:", map[string]bool{"linux": true}},
		{"linux,windows:", map[string]bool{"linux": true, "windows": true}},
		{"!windows:", map[string]bool{"linux": true, "darwin": true, "freebsd": true}},
		{"!linux:", map[string]bool{"windows": true, "darwin": true, "freebsd": true}},
	} {
		r, errs := recipeOf(t, "\t"+tc.cond+" define X")
		if len(errs) > 0 {
			t.Errorf("%s: %v", tc.cond, errs)
			continue
		}
		for _, goos := range []string{"linux", "windows", "darwin", "freebsd"} {
			if got := r.Stmts[0].Cond.Match(goos); got != tc.os[goos] {
				t.Errorf("%s matches %s = %v", tc.cond, goos, got)
			}
		}
	}
	if !(Cond{}).Match("anything") {
		t.Error("an empty condition matches every OS")
	}
}

// TestRecipeLineErrors checks each error with its line and message.
func TestRecipeLineErrors(t *testing.T) {
	long := "src " + strings.Repeat("a", MaxRecipeLine)
	manyArgs := "define " + strings.Repeat("A ", MaxRecipeArgs+1)
	for _, tc := range []struct {
		line string
		want string // a part of the message
	}{
		{"frobnicate x", `unknown statement "frobnicate": gocpp does not know it, and ignoring it could build the wrong library`},
		{"cflags", "usage: cflags flag..."},
		{"src", "usage: src file..."},
		{"define", "usage: define NAME[=value]..."},
		{"std", "usage: std c++17"},
		{"std c++9", `invalid C++ standard "c++9"`},
		{"std c++17 c++20", "invalid C++ standard"},
		{"cstd c++17", "invalid C standard"},
		{"cstd gnu99 x", "invalid C standard"},
		{"src ../x.c", `a ".." element`},
		{"src a/../../x.c", `a ".." element`},
		{"src a/..", `a ".." element`},
		{"src /etc/passwd.c", "an absolute path"},
		{"src `C:\\x.c`", "use / in a path"},
		{`src C:/x.c`, `has a ":"`},
		{"src `a\\b.c`", "use / in a path"},
		{"src ~/x.c", "starts with ~"},
		{"src $HOME/x.c", "$ is not expanded"},
		{"src a/**/b.c", `"**" is not supported`},
		{"src a[.c", "bad pattern"},
		{"src a. b.c", "ends in a dot or a space"},
		{"include a/*", "a pattern is not allowed here"},
		{"system-include /usr/include", "an absolute path"},
		{"define 1A", "the name of a macro"},
		{"define A-B=1", "the name of a macro"},
		{"define =1", "the name of a macro"},
		{"private-define a(x)=1", "the name of a macro"},
		{"link -l", "want a library name"},
		{"link -x", "want a library name"},
		{"link ../x", "want a library name"},
		{"link @x", "want a library name"},
		{"pkg-config -foo", "invalid pkg-config package name"},
		{"pkg-config a/b", "invalid character"},
		{"pkg-config foo.pc", "ends in .pc"},
		{"pkg-config FOO.PC", "ends in .pc"},
		{"define A=\"x", "unterminated quote"},
		{"cflags \\", "backslash at the end"},
		{"linux:", "the condition linux: has no statement after it"},
		{"linuxx: src a.c", `unknown operating system "linuxx"`},
		{"linux,linux: src a.c", "is repeated"},
		{"!linux,windows: src a.c", "names one operating system"},
		{"linux:src a.c", `unknown statement "linux:src"`},
		{"a:b: src a.c", `unknown operating system "a:b"`},
		{"define { x", "blocks inside a recipe are not part of the recipe language"},
		{"src a.c )", "blocks inside a recipe are not part of the recipe language"},
		{long, "the line is too long"},
		{manyArgs, "has more than 4096 arguments"},
		{"src a.c\x00b", "control character"},
		{"src a.c\x01", "control character"},
		{"src a.c\x7f", "control character"},
		{"src \xff.c", "not valid UTF-8"},
		{"/* c */ src a.c", "unknown statement"},
	} {
		r, errs := recipeOf(t, "\tsrc ok.c", "\t"+tc.line, "\tdefine OK")
		if len(errs) != 1 {
			t.Errorf("%q: %d errors, want 1: %v", firstN(tc.line), len(errs), errs)
			continue
		}
		e := errs[0]
		if e.Pos.Line != 5 {
			t.Errorf("%q: error at line %d, want 5", firstN(tc.line), e.Pos.Line)
		}
		if !strings.Contains(e.Err, tc.want) {
			t.Errorf("%q: error %q does not have %q", firstN(tc.line), firstN(e.Err), tc.want)
		}
		if !strings.HasPrefix(e.Err, "cxx example.com/lib: ") {
			t.Errorf("%q: the error does not name the module: %q", firstN(tc.line), firstN(e.Err))
		}
		// The lines around it are still read: an error never hides the rest.
		if len(r.Stmts) != 2 || r.Stmts[0].Verb != "src" || r.Stmts[1].Verb != "define" || r.Stmts[1].Line != 6 {
			t.Errorf("%q: the other lines were not read: %+v", firstN(tc.line), r.Stmts)
		}
		if got := e.Error(); !strings.HasPrefix(got, "gocpp.mod:5: ") {
			t.Errorf("%q: Error() = %q", firstN(tc.line), firstN(got))
		}
	}
}

func firstN(s string) string {
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}

func TestRecipeAllErrorsAtOnce(t *testing.T) {
	_, errs := recipeOf(t, "\tbad1", "\tsrc ../a.c", "\tdefine ok", "\tbad2 x")
	if len(errs) != 3 || errs[0].Pos.Line != 4 || errs[1].Pos.Line != 5 || errs[2].Pos.Line != 7 {
		t.Errorf("errors = %v", errs)
	}
}

func TestRecipeTooManyStatements(t *testing.T) {
	var lines []string
	for i := 0; i < MaxRecipeStatements+10; i++ {
		lines = append(lines, "\tdefine A")
	}
	r, errs := recipeOf(t, lines...)
	if len(errs) != 1 || !strings.Contains(errs[0].Err, "more than 4096 statements") || len(r.Stmts) != MaxRecipeStatements {
		t.Errorf("%d statements, errors %v", len(r.Stmts), errs)
	}
}

func TestCheckRecipePath(t *testing.T) {
	for _, tc := range []struct {
		in, out string
		glob    bool
		bad     string
	}{
		{"a.c", "a.c", false, ""},
		{"./a.c", "a.c", false, ""},
		{"a//b/./c.c", "a/b/c.c", false, ""},
		{".", ".", false, ""},
		{"backends/*.cpp", "backends/*.cpp", true, ""},
		{"a?.c", "a?.c", true, ""},
		{"[ab].c", "[ab].c", true, ""},
		{".hidden/x.c", ".hidden/x.c", true, ""},
		{"", "", false, "empty path"},
		{"a\x00.c", "", false, "NUL"},
		{"a.c\n", "", false, "control character"},
		{"..", "", false, `".."`},
		{"a/../b.c", "", false, `".."`},
		{"/a.c", "", false, "absolute"},
		{"//host/share/a.c", "", false, "absolute"},
		{`\\host\share`, "", false, "use /"},
		{"c:", "", false, `":"`},
		{"a:b", "", false, `":"`},
		{"x/a.c ", "", false, "dot or a space"},
		{"./~", "", false, "starts with ~"},
		{"*.c", "", false, "pattern is not allowed"},
		{"a/**", "", true, "**"},
	} {
		got, why := CheckRecipePath(tc.in, tc.glob)
		if tc.bad == "" && (why != "" || got != tc.out) {
			t.Errorf("CheckRecipePath(%q, %v) = %q, %q; want %q", tc.in, tc.glob, got, why, tc.out)
		}
		if tc.bad != "" && !strings.Contains(why, tc.bad) {
			t.Errorf("CheckRecipePath(%q, %v) = %q, %q; want an error with %q", tc.in, tc.glob, got, why, tc.bad)
		}
	}
}

// TestRecipesOfFile checks how recipes are found in a file: the
// project's and a library's, keyed by the module path; and that a block
// that is not a recipe, or twice for the same module, is an error.
func TestRecipesOfFile(t *testing.T) {
	src := `module example.com/app
cxx c++20

cxx example.com/a {
	src a.c
}
cxx "example.com/b" {
	src b.c
}
cxx example.com/a {
	src a2.c
}
cxx example.com/c d {
}
cxx {
}
cxx example.com/x%y {
}
cxx a/../b {
}
`
	f, errs := Parse("gocpp.mod", []byte(src), nil)
	if len(errs) > 0 {
		t.Fatalf("Parse: %v", errs)
	}
	blocks, rerrs := f.Recipes()
	if len(blocks) != 2 || blocks["example.com/a"] == nil || blocks["example.com/b"] == nil || blocks["example.com/a"].Start.Line != 4 {
		t.Errorf("blocks = %v", blocks)
	}
	var got []string
	for _, e := range rerrs {
		got = append(got, fmt.Sprintf("%d: %s", e.Pos.Line, e.Err))
	}
	want := []string{
		"10: multiple cxx recipes for example.com/a (the first is at line 4)",
		"13: usage: cxx module/path { ... }",
		"15: usage: cxx module/path { ... }",
		`17: cxx example.com/x%y { ... }: invalid module path "example.com/x%y": invalid char '%'`,
		`19: cxx a/../b { ... }: invalid module path "a/../b": invalid path element ".."`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("errors:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// The one-line cxx statement is the C++ standard, not a recipe.
	if f.Cxx == nil || f.Cxx.Std != "c++20" {
		t.Errorf("cxx = %+v", f.Cxx)
	}
}

// TestRecipeKeepsFileOpaque checks that reading a recipe does not change
// how the file is read by other tools: Parse reports nothing about the lines
// of the block (even a statement that no gocpp knows), and Format writes the
// file back as it was.
func TestRecipeKeepsFileOpaque(t *testing.T) {
	src := "module example.com/app\n\ncxx example.com/lib {\n\tfuture-statement 1 2 3\n\tdefine {\n\t\tA=1\n\t}\n\t/* odd */ `unterminated\n}\n\ncflags -DX\n"
	f, errs := Parse("gocpp.mod", []byte(src), nil)
	if len(errs) != 0 {
		t.Fatalf("Parse reports %v: the recipe must stay opaque", errs)
	}
	if len(f.Cflags) != 1 || len(f.Unknown) != 1 {
		t.Errorf("cflags %d, unknown %d", len(f.Cflags), len(f.Unknown))
	}
	if got := string(Format(f.Syntax)); got != src {
		t.Errorf("Format changed the file:\n%s", got)
	}
	// ... and the recipe reader reports each of those lines.
	blocks, _ := f.Recipes()
	_, rerrs := ParseRecipe("gocpp.mod", "example.com/lib", blocks["example.com/lib"])
	if len(rerrs) < 3 {
		t.Errorf("recipe errors = %v", rerrs)
	}
	for _, e := range rerrs {
		if e.Pos.Line < 4 || e.Pos.Line > 8 {
			t.Errorf("error %v is outside the block", e)
		}
	}
}

// TestRecipeLineNumbers checks the line of an error against the file, with
// blank lines, comments and CRLF line ends.
func TestRecipeLineNumbers(t *testing.T) {
	src := "module example.com/app\r\n\r\n// c\r\ncxx example.com/lib { // open\r\n\r\n\t// x\r\n\tsrc a.c\r\n\r\n\tbad\r\n}\r\n"
	f, errs := Parse("gocpp.mod", []byte(src), nil)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	blocks, _ := f.Recipes()
	r, rerrs := ParseRecipe("gocpp.mod", "example.com/lib", blocks["example.com/lib"])
	if len(rerrs) != 1 || rerrs[0].Pos.Line != 9 || len(r.Stmts) != 1 || r.Stmts[0].Line != 7 {
		t.Errorf("errors %v, statements %+v", rerrs, r.Stmts)
	}
}

// FuzzParseRecipe feeds any text as the body of a recipe (newlines, braces and
// quotes included) and checks what ParseRecipe lets through: a path of src,
// exclude, include and system-include is a clean relative path whose elements
// are not "..", with none of the characters a recipe refuses; link gets
// library names, pkg-config names that cannot name a file, define macro names.
func FuzzParseRecipe(f *testing.F) {
	for _, s := range []string{"src a.c", "linux: define A=1", "pkg-config x", "define \"x", "src a/*/b.c", "\x00", strings.Repeat("a ", 1000),
		"src 0/..0", "src a/../b.c\ninclude /x", "define {\n}\nsrc a.c", "link -l:../x", "src `a\\b.c`", "exclude ~/x $y c:d"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		src := "module example.com/app\ncxx example.com/lib {\n" + body + "\n}\n"
		file, _ := Parse("gocpp.mod", []byte(src), nil)
		blocks, _ := file.Recipes()
		b := blocks["example.com/lib"]
		if b == nil {
			return // the block did not close or open as a recipe: Parse said so
		}
		r, _ := ParseRecipe("gocpp.mod", "example.com/lib", b)
		badPath := func(a string) string {
			switch {
			case a == "" || strings.HasPrefix(a, "/") || strings.HasPrefix(a, "~"):
				return "empty, absolute or ~"
			case strings.ContainsAny(a, "\\:$\x00"):
				return "a character that a recipe path refuses"
			case slices.Contains(strings.Split(a, "/"), ".."):
				return "a .. element"
			case path.Clean(a) != a:
				return "not clean"
			}
			for _, c := range a {
				if c < ' ' || c == 0x7f {
					return "a control character"
				}
			}
			return ""
		}
		for _, s := range r.Stmts {
			if len(s.Args) == 0 || s.Line < b.Start.Line+1 {
				t.Fatalf("bad statement %+v", s)
			}
			for _, a := range s.Args {
				switch s.Verb {
				case RecipeSrc, RecipeExclude, RecipeInclude, RecipeSystemInclude:
					if why := badPath(a); why != "" {
						t.Fatalf("%s %q accepted: %s", s.Verb, a, why)
					}
					if (s.Verb == RecipeInclude || s.Verb == RecipeSystemInclude) && strings.ContainsAny(a, "*?[") {
						t.Fatalf("%s %q: a pattern in an include", s.Verb, a)
					}
				case RecipeLink:
					if !strings.HasPrefix(a, "-l") || !libNameRE.MatchString(a[2:]) || strings.ContainsAny(a, "/:") {
						t.Fatalf("link %q accepted", a)
					}
				case RecipePkgConfig:
					if checkPkgConfigName(a) != "" || strings.HasSuffix(strings.ToLower(a), ".pc") {
						t.Fatalf("pkg-config %q accepted", a)
					}
				case RecipeDefine, RecipePrivateDefine:
					name, _, _ := strings.Cut(a, "=")
					if !macroNameRE.MatchString(name) {
						t.Fatalf("define %q accepted", a)
					}
				}
			}
		}
	})
}
