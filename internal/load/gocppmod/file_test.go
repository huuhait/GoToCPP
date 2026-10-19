package gocppmod

import (
	"reflect"
	"strings"
	"testing"
)

// adr is the example of ADR-020 and Phase 06 README §2.1.
const adr = `module github.com/huuhait/game

gocpp 0.1                    // minimum gocpp version
cxx c++20                    // C++ standard for generated + user C++ files

require github.com/huuhait/mathx v0.3.0
replace github.com/huuhait/mathx => ../mathx

cflags -DGAME_DEBUG
pkg-config sdl2
`

func mustParse(t *testing.T, src string) *File {
	t.Helper()
	f, errs := Parse("gocpp.mod", []byte(src), nil)
	if len(errs) > 0 {
		t.Fatalf("errors:\n%v", errs)
	}
	return f
}

func TestParseExample(t *testing.T) {
	f := mustParse(t, adr)
	if f.Module == nil || f.Module.Path != "github.com/huuhait/game" {
		t.Errorf("module = %+v", f.Module)
	}
	if f.Gocpp == nil || f.Gocpp.Version != "0.1" {
		t.Errorf("gocpp = %+v", f.Gocpp)
	}
	if f.Cxx == nil || f.Cxx.Std != "c++20" {
		t.Errorf("cxx = %+v", f.Cxx)
	}
	if len(f.Require) != 1 || f.Require[0].Path != "github.com/huuhait/mathx" || f.Require[0].Version != "v0.3.0" || f.Require[0].Indirect {
		t.Errorf("require = %+v", f.Require)
	}
	if len(f.Replace) != 1 {
		t.Fatalf("replace = %+v", f.Replace)
	}
	if r := f.Replace[0]; r.OldPath != "github.com/huuhait/mathx" || r.OldVersion != "" || r.NewPath != "../mathx" || r.NewVersion != "" || !r.Local() {
		t.Errorf("replace = %+v", r)
	}
	if len(f.Cflags) != 1 || !reflect.DeepEqual(f.Cflags[0].Args, []string{"-DGAME_DEBUG"}) {
		t.Errorf("cflags = %+v", f.Cflags)
	}
	if len(f.PkgConfig) != 1 || !reflect.DeepEqual(f.PkgConfig[0].Args, []string{"sdl2"}) {
		t.Errorf("pkg-config = %+v", f.PkgConfig)
	}
	if len(f.Unknown) != 0 {
		t.Errorf("unknown = %v", f.Unknown)
	}
	if got := string(Format(f.Syntax)); got != strings.Replace(adr, "gocpp 0.1                    //", "gocpp 0.1 //", 1) && got != adr {
		// Format writes one space before a suffix comment.
		want := strings.NewReplacer("0.1                    //", "0.1 //", "c++20                    //", "c++20 //").Replace(adr)
		if got != want {
			t.Errorf("Format:\n%s\nwant\n%s", got, want)
		}
	}
}

func TestParseBlocks(t *testing.T) {
	f := mustParse(t, `module example.com/app

require (
	example.com/a v1.2.3
	example.com/b v0.1.0-pre.1+meta // indirect
	example.com/c v0.0.0-20240101000000-abcdef123456 // indirect; for tests
)

replace (
	example.com/a => ../a
	example.com/b v0.1.0-pre.1+meta => /abs/b
	example.com/c => example.org/fork v1.0.0
)

cflags (
	-I include
	"-DNAME=a b"
)
`)
	var got []string
	for _, r := range f.Require {
		got = append(got, r.Path+" "+r.Version+" "+map[bool]string{true: "indirect", false: "direct"}[r.Indirect])
	}
	want := []string{
		"example.com/a v1.2.3 direct",
		"example.com/b v0.1.0-pre.1 indirect",
		"example.com/c v0.0.0-20240101000000-abcdef123456 indirect",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("require = %q, want %q", got, want)
	}
	if len(f.Replace) != 3 {
		t.Fatalf("replace = %+v", f.Replace)
	}
	if r := f.Replace[1]; r.OldVersion != "v0.1.0-pre.1" || r.NewPath != "/abs/b" || !r.Local() {
		t.Errorf("replace[1] = %+v", r)
	}
	if r := f.Replace[2]; r.Local() || r.NewPath != "example.org/fork" || r.NewVersion != "v1.0.0" {
		t.Errorf("replace[2] = %+v", r)
	}
	if len(f.Cflags) != 2 || !reflect.DeepEqual(f.Cflags[0].Args, []string{"-I", "include"}) ||
		!reflect.DeepEqual(f.Cflags[1].Args, []string{"-DNAME=a b"}) {
		t.Errorf("cflags = %+v %+v", f.Cflags[0], f.Cflags[1])
	}
}

func TestParseQuoted(t *testing.T) {
	f := mustParse(t, "module \"example.com/app\" // c\nrequire `example.com/a` \"v1.0.0\"\n"+
		"replace \"example.com/a\" => \"./a\"\ncflags -DMSG=\"hi there\" '-DX=1 2' a\\ b\n")
	if f.Module.Path != "example.com/app" || f.Require[0].Path != "example.com/a" || f.Require[0].Version != "v1.0.0" ||
		f.Replace[0].OldPath != "example.com/a" || f.Replace[0].NewPath != "./a" {
		t.Errorf("file = %+v %+v %+v", f.Module, f.Require[0], f.Replace[0])
	}
	if want := []string{"-DMSG=hi there", "-DX=1 2", "a b"}; !reflect.DeepEqual(f.Cflags[0].Args, want) {
		t.Errorf("cflags = %q, want %q", f.Cflags[0].Args, want)
	}
}

// TestUnknownKept checks that statements gocpp does not read are kept in the
// file, and that Format writes them back.
func TestUnknownKept(t *testing.T) {
	src := `module example.com/app

go 1.25
toolchain go1.25.1
godebug default=go1.21
future-verb (
	a b
	c
)
cxx example.com/lib {
	sources *.cpp backends/x.cpp
	include . backends
	pkg-config glfw3
	cflags -DNOT_MODULE_WIDE
}
cxx c++17
`
	f := mustParse(t, src)
	if f.Cxx == nil || f.Cxx.Std != "c++17" {
		t.Errorf("cxx = %+v", f.Cxx)
	}
	// Nothing in the brace block is a module-wide setting.
	if len(f.PkgConfig) != 0 || len(f.Cflags) != 0 {
		t.Errorf("settings of the recipe leaked: %+v %+v", f.PkgConfig, f.Cflags)
	}
	var verbs []string
	for _, s := range f.Unknown {
		switch s := s.(type) {
		case *Line:
			verbs = append(verbs, s.Token[0])
		case *Block:
			verbs = append(verbs, s.Token[0]+string(s.Open))
		}
	}
	if want := []string{"go", "toolchain", "godebug", "future-verb(", "cxx{"}; !reflect.DeepEqual(verbs, want) {
		t.Errorf("unknown = %q, want %q", verbs, want)
	}
	if got := string(Format(f.Syntax)); got != src {
		t.Errorf("Format changed the file:\n%s\nwant\n%s", got, src)
	}
}

func TestFormat(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"empty", "", ""},
		{"spaces", "module   a\n\n\n\nrequire\tb   v1.0.0   // x\n", "module a\n\nrequire b v1.0.0 // x\n"},
		{"crlf", "module a\r\ngocpp 0.1\r\n", "module a\ngocpp 0.1\n"},
		{"no final newline", "module a", "module a\n"},
		{"block indent", "require (\nb v1.0.0\n   c v1.1.0 // i\n)\n", "require (\n\tb v1.0.0\n\tc v1.1.0 // i\n)\n"},
		{"comments", "// top\n\n// about module\nmodule a // m\n\n// end\n", "// top\n\n// about module\nmodule a // m\n\n// end\n"},
		{"block comments and blanks", "require (\n\t// first\n\ta v1.0.0\n\n\n\tb v1.0.0\n\t// last\n)\n",
			"require (\n\t// first\n\ta v1.0.0\n\n\tb v1.0.0\n\t// last\n)\n"},
		{"block suffix", "require ( // all\n\ta v1.0.0\n) // done\n", "require ( // all\n\ta v1.0.0\n) // done\n"},
		{"empty block", "require (\n)\n", "require (\n)\n"},
		{"leading blank lines", "\n\nmodule a\n", "module a\n"},
		{"flags keep punctuation", "cflags -DX=(1,2) -DY={a,b} [z]\n", "cflags -DX=(1,2) -DY={a,b} [z]\n"},
		{"quotes kept", "cflags \"-DA=a  b\" '-DB=c'\n", "cflags \"-DA=a  b\" '-DB=c'\n"},
		{"comment in quotes", "cflags \"-DURL=http://x\"\n", "cflags \"-DURL=http://x\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			syn, errs := ParseSyntax("gocpp.mod", []byte(tc.in))
			if len(errs) > 0 {
				t.Fatalf("errors: %v", errs)
			}
			got := string(Format(syn))
			if got != tc.want {
				t.Errorf("Format =\n%q\nwant\n%q", got, tc.want)
			}
			// Formatting again changes nothing.
			syn2, errs := ParseSyntax("gocpp.mod", []byte(got))
			if len(errs) > 0 {
				t.Fatalf("errors in the output: %v", errs)
			}
			if again := string(Format(syn2)); again != got {
				t.Errorf("Format is not idempotent:\n%q\n%q", got, again)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"module usage", "module\n", "gocpp.mod:1: usage: module module/path"},
		{"module two", "module a b\n", "gocpp.mod:1: usage: module module/path"},
		{"module repeated", "module a\n\nmodule b\n", "gocpp.mod:3: repeated module statement"},
		{"module path", "module a//b\nmodule x/../y\n", "gocpp.mod:2: repeated module statement"}, // a//b is "a" and a comment
		{"module bad path", "module x/../y\n", `gocpp.mod:1: invalid module path "x/../y": invalid path element ".."`},
		{"module header", "module <x.h>\n", `gocpp.mod:1: invalid module path "<x.h>": invalid char '<'`},
		{"unterminated quote", "module \"a\n", `gocpp.mod:1: invalid quoted string: "a`},
		{"gocpp version", "gocpp one\n", `gocpp.mod:1: invalid gocpp version "one": must match format 0.1`},
		{"gocpp usage", "gocpp\n", "gocpp.mod:1: usage: gocpp 0.1"},
		{"gocpp repeated", "gocpp 0.1\ngocpp 0.2\n", "gocpp.mod:2: repeated gocpp statement"},
		{"cxx std", "cxx c++2\n", `gocpp.mod:1: invalid C++ standard "c++2": want c++17, c++20, gnu++20, ...`},
		{"cxx usage", "cxx\n", "gocpp.mod:1: usage: cxx c++20"},
		{"cxx two", "cxx c++20 c++17\n", "gocpp.mod:1: usage: cxx c++20"},
		{"cxx one-line recipe", "cxx example.com/ev {src a.c}\n", "gocpp.mod:1: usage: cxx c++20" + recipeHint},
		{"cxx brace glued", "cxx example.com/ev{\n", `gocpp.mod:1: invalid C++ standard "example.com/ev{": want c++17, c++20, gnu++20, ...` + recipeHint},
		{"cxx repeated", "cxx c++20\ncxx gnu++17\n", "gocpp.mod:2: repeated cxx statement"},
		{"require usage", "require a\n", "gocpp.mod:1: usage: require module/path v1.2.3"},
		{"require version", "require a v1\n", `gocpp.mod:1: require a: invalid module version "v1": must be of the form v1.2.3`},
		{"require path", "require a/ v1.0.0\n", `gocpp.mod:1: invalid module path "a/": trailing slash`},
		{"require in block", "require (\n\ta\n)\n", "gocpp.mod:2:2: usage: require module/path v1.2.3"},
		{"replace usage", "replace a\n", "gocpp.mod:1: usage: replace module/path [v1.2.3] => other/module v1.4\n\t or replace module/path [v1.2.3] => ../local/directory"},
		{"replace no target", "replace a =>\n", "gocpp.mod:1: usage: replace module/path [v1.2.3] => other/module v1.4\n\t or replace module/path [v1.2.3] => ../local/directory"},
		{"replace no version", "replace a => b\n", "gocpp.mod:1: replacement module without version must be directory path (rooted or starting with . or ..)"},
		{"replace dir version", "replace a => ../b v1.0.0\n", `gocpp.mod:1: replacement module directory path "../b" cannot have version`},
		{"replace bad version", "replace a v1 => ../b\n", `gocpp.mod:1: replace a: invalid module version "v1": must be of the form v1.2.3`},
		{"replace bad new version", "replace a => b v1\n", `gocpp.mod:1: replace a: invalid module version "v1": must be of the form v1.2.3`},
		{"replace twice", "replace a => ../b\nreplace a => ../c\n", "gocpp.mod:2: multiple replacements for a (the first is at line 1)"},
		{"replace twice version", "replace a v1.0.0 => ../b\nreplace a v1.0.0 => ../c\n", "gocpp.mod:2: multiple replacements for a@v1.0.0 (the first is at line 1)"},
		{"cflags usage", "cflags\n", "gocpp.mod:1: usage: cflags flag..."},
		{"cflags quote", "cflags -DA=\"b\n", "gocpp.mod:1: unterminated quote in cflags"},
		{"cflags empty", "ldflags \"\"\n", "gocpp.mod:1: empty argument in ldflags"},
		{"cflags backslash", "cflags a\\\n", "gocpp.mod:1: backslash at the end of the line in cflags"},
		{"pkg-config usage", "pkg-config\n", "gocpp.mod:1: usage: pkg-config package..."},
		{"pkg-config flag", "pkg-config -lm\n", `gocpp.mod:1: invalid pkg-config package name "-lm": flags go in cflags or ldflags`},
		{"pkg-config char", "pkg-config a/b\n", `gocpp.mod:1: invalid character '/' in pkg-config package name "a/b"`},
		{"comment style", "module a /* x */\n", "gocpp.mod:1:10: mod files must use // comments (not /* */ comments)"},
		{"bad character", "module a\x00b\n", "gocpp.mod:1:9: unexpected input character '\\x00'"},
		{"unterminated block", "module a\nrequire (\n\tb v1.0.0\n", "gocpp.mod:4: syntax error (unterminated block started at gocpp.mod:2:1)"},
		{"text after paren", "require (\n\tb v1.0.0\n) x\n", "gocpp.mod:3:3: syntax error (expected newline after closing paren)"},
		{"block without verb", "(\n", "gocpp.mod:1: syntax error (unexpected (: a block needs a verb)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := Parse("gocpp.mod", []byte(tc.in), nil)
			if got := errs.Error(); got != tc.want {
				t.Errorf("errors:\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

// TestManyErrors checks that Parse reports every broken statement, in line
// order, and keeps the valid ones.
func TestManyErrors(t *testing.T) {
	f, errs := Parse("gocpp.mod", []byte("module a\ngocpp x\nrequire b v1.0.0\ncxx c++1\n/* bad */\nrequire c v2.0.0\nreplace c\n"), nil)
	want := `gocpp.mod:2: invalid gocpp version "x": must match format 0.1
gocpp.mod:4: invalid C++ standard "c++1": want c++17, c++20, gnu++20, ...
gocpp.mod:5: mod files must use // comments (not /* */ comments)
gocpp.mod:7: usage: replace module/path [v1.2.3] => other/module v1.4
	 or replace module/path [v1.2.3] => ../local/directory`
	if got := errs.Error(); got != want {
		t.Errorf("errors:\n%s\nwant\n%s", got, want)
	}
	if f.Module == nil || f.Module.Path != "a" || len(f.Require) != 2 {
		t.Errorf("the valid statements were not kept: %+v %+v", f.Module, f.Require)
	}
}

func TestHasModuleLine(t *testing.T) {
	for src, want := range map[string]bool{
		"module a\n": true, "module\n": true, "module (\n\ta\n)\n": true,
		"gocpp 0.1\n": false, "// module a\n": false, "": false, "go 1.25\n": false,
	} {
		f, _ := Parse("gocpp.mod", []byte(src), nil)
		if got := f.HasModuleLine(); got != want {
			t.Errorf("HasModuleLine(%q) = %v", src, got)
		}
	}
}

func TestCustomPathCheck(t *testing.T) {
	_, errs := Parse("gocpp.mod", []byte("module a\nrequire b v1.0.0\n"), func(p string) (bool, string) {
		return p == "a", "must be a"
	})
	if want := `gocpp.mod:2: invalid module path "b": must be a`; errs.Error() != want {
		t.Errorf("errors = %v, want %s", errs, want)
	}
}

func TestPaths(t *testing.T) {
	for p, bad := range map[string]string{
		"myapp": "", "example.com/app": "", "github.com/a-b/c_d.e": "", "example.com/a~b": "", "ünï/x": "",
		"": "empty string", "/a": "leading slash", "a/": "trailing slash", "a//b": "double slash",
		"a b": `invalid char ' '`, "a/./b": `invalid path element "."`, "a/../b": `invalid path element ".."`, "a:b": `invalid char ':'`,
	} {
		if got := CheckPath(p); got != bad {
			t.Errorf("CheckPath(%q) = %q, want %q", p, got, bad)
		}
	}
	for v, ok := range map[string]bool{
		"v0.0.0": true, "v1.2.3": true, "v1.2.3-rc.1": true, "v1.2.3+incompatible": true, "v0.0.0-20240101000000-abcdef123456": true,
		"v1": false, "v1.2": false, "1.2.3": false, "v01.2.3": false, "v1.2.3-": false, "": false, "v1.2.3 ": false,
	} {
		if ValidVersion(v) != ok {
			t.Errorf("ValidVersion(%q) = %v", v, !ok)
		}
	}
	for p, ok := range map[string]bool{
		".": true, "..": true, "./a": true, "../a": true, "/a": true, `.\a`: true, `..\a`: true, `\a`: true, `C:\a`: true, "c:/a": true,
		"a": false, "a/b": false, ".a": false, "example.com/a": false, "": false,
	} {
		if IsDirectoryPath(p) != ok {
			t.Errorf("IsDirectoryPath(%q) = %v", p, !ok)
		}
	}
}

// TestRoundTrip formats a mix of inputs twice and checks that the second
// output is the first, and that no statement is lost.
func TestRoundTrip(t *testing.T) {
	inputs := []string{
		adr,
		"// c\nmodule a\n\n// d\n\n// e\nrequire (\n\t// f\n\tb v1.0.0 // g\n\n\t// h\n)\n// i",
		"a\nb c\n\nd ( e\nf ) g\n",
		"x {\n\ty\n}\nx {\n}\n",
		"module \"a\"\nrequire `b` v1.0.0\n",
		"cflags \"unterminated\n",
		"module a\n\n\n// only\n\n\n// comments\n",
	}
	for _, in := range inputs {
		syn, _ := ParseSyntax("gocpp.mod", []byte(in))
		once := Format(syn)
		syn2, errs := ParseSyntax("gocpp.mod", once)
		if len(errs) > 0 && !strings.Contains(in, "unterminated") {
			t.Errorf("errors in the output of %q: %v", in, errs)
		}
		if twice := Format(syn2); string(twice) != string(once) {
			t.Errorf("not idempotent for %q:\n%q\n%q", in, once, twice)
		}
		if len(syn2.Stmt) != len(syn.Stmt) {
			t.Errorf("%q: %d statements, then %d", in, len(syn.Stmt), len(syn2.Stmt))
		}
	}
}

// FuzzParse checks that Parse never panics, and that Format is stable.
func FuzzParse(f *testing.F) {
	for _, s := range []string{adr, "", "(\n", "x (\n", "x {\n}\n}", "module \"\n", "a // b\n", "\"a\\", "'", "`", "\xff\n", "require (\n\n\n)"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		file, perrs := Parse("gocpp.mod", []byte(in), nil)
		once := Format(file.Syntax)
		if len(perrs) == 0 {
			if again, errs := Parse("gocpp.mod", once, nil); len(errs) > 0 || summary(again) != summary(file) {
				t.Fatalf("Format changed what %q says:\n%v\n%s\n%s", in, errs, summary(file), summary(again))
			}
		}
		syn2, errs := ParseSyntax("gocpp.mod", once)
		if len(errs) == 0 {
			if twice := Format(syn2); string(twice) != string(once) {
				t.Fatalf("not idempotent for %q:\n%q\n%q", in, once, twice)
			}
		}
	})
}

// A backslash before a CR in the middle of a line is an error: it would keep nothing, and Format
// gave it the space that follows (found by fuzzing, 15.8: Format was not idempotent for "\\\r//").
func TestBackslashBeforeCR(t *testing.T) {
	for _, in := range []string{"\\\r//", "cflags a\\\rb\n"} {
		_, errs := ParseSyntax("gocpp.mod", []byte(in))
		if len(errs) == 0 || !strings.Contains(errs[0].Err, "unexpected input character") {
			t.Errorf("%q: errors %v", in, errs)
		}
	}
	// A backslash before another character still keeps it.
	syn, errs := ParseSyntax("gocpp.mod", []byte("cflags a\\ b\n"))
	if len(errs) != 0 || len(syn.Stmt) != 1 {
		t.Errorf("escaped space: %v", errs)
	}
}
