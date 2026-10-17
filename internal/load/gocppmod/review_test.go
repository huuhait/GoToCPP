package gocppmod

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Tests of the review of step 6.1: recipes with nested blocks, keywords in a
// wrong shape, versions, speed on large files and the limits.

const recipe = "module m\ncxx example.com/x {\n\tdefine {\n\t\tA=1\n\t}\n\tpkg-config glfw3\n\tcflags -DLEAK\n}\ncxx c++20\n"

// TestRecipeBody checks that the body of a brace block is opaque: a nested
// block does not end it, nothing in it is a module-wide setting, and Format
// writes it back as it was.
func TestRecipeBody(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		raw       int // the lines of the body of the first brace block
	}{
		{"nested", recipe, 5},
		{"three deep", "cxx a {\n\tx {\n\t\ty (\n\t\t\tz {\n\t\t\t}\n\t\t)\n\t}\n\tpkg-config leak\n}\npkg-config real\n", 7},
		{"comments in the body", "cxx a {\n\t/* not a go.mod comment */\n\t// c\n\tpkg-config \x01\x7f\n\tx { // open\n\t} // close\n}\n", 5},
		{"parens and braces in strings", "cxx a {\n\tdefine \"}\" ')' `}` \\}\n\tcflags \"-DX={\" // {\n\tinclude )x\n\tend }x\n}\n", 4},
		{"blank lines and indentation", "cxx a {\n\n    \n\t\t\tsources *.cpp  \n\n}\n", 4},
		{"empty", "cxx a {\n}\n", 0},
		{"crlf", "cxx a {\r\n\tx\r\n}\r\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, errs := Parse("gocpp.mod", []byte(tc.src), nil)
			if len(errs) > 0 {
				t.Fatalf("errors: %v", errs)
			}
			if len(f.Cflags) != 0 || (len(f.PkgConfig) != 1 && strings.Contains(tc.src, "pkg-config real")) ||
				(len(f.PkgConfig) != 0 && !strings.Contains(tc.src, "pkg-config real")) {
				t.Errorf("a line of the recipe is a module-wide setting: cflags %+v, pkg-config %+v", f.Cflags, f.PkgConfig)
			}
			var b *Block
			for _, s := range f.Syntax.Stmt {
				if blk, ok := s.(*Block); ok {
					b = blk
					break
				}
			}
			if b == nil || len(b.Raw) != tc.raw || len(f.Unknown) != 1 || f.Unknown[0] != Stmt(b) {
				t.Fatalf("block %+v, unknown %v", b, f.Unknown)
			}
			if got, want := string(Format(f.Syntax)), strings.ReplaceAll(tc.src, "\r\n", "\n"); got != want {
				t.Errorf("Format:\n%q\nwant\n%q", got, want)
			}
		})
	}

	// The statements after the recipe are read.
	f := mustParse(t, recipe)
	if f.Cxx == nil || f.Cxx.Std != "c++20" || len(f.Unknown) != 1 {
		t.Errorf("cxx %+v, unknown %d", f.Cxx, len(f.Unknown))
	}
}

func TestRecipeErrors(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"unterminated", "module m\ncxx a {\n\tx {\n\t}\n", "gocpp.mod:5: syntax error (unterminated block started at gocpp.mod:2:1)"},
		{"unterminated at the end of a line", "cxx a {\n\tx", "gocpp.mod:2:3: syntax error (unterminated block started at gocpp.mod:1:1)"},
		{"stray brace", "module m\n}\n", "gocpp.mod:2: syntax error (unexpected })"},
		{"stray paren", "module m\n)\nmodule n\n", "gocpp.mod:2: syntax error (unexpected ))\ngocpp.mod:3: repeated module statement"},
		{"stray brace after a recipe", "cxx a {\n}\n}\n", "gocpp.mod:3: syntax error (unexpected })"},
		{"paren in a brace block does not close it", "cxx a {\n\tx\n)\n", "gocpp.mod:4: syntax error (unterminated block started at gocpp.mod:1:1)"},
		{"mismatched closer inside", "cxx a {\n\tx {\n\t)\n\t}\n}\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, errs := Parse("gocpp.mod", []byte(tc.src), nil); errs.Error() != tc.want {
				t.Errorf("errors:\n%s\nwant\n%s", errs, tc.want)
			}
		})
	}
	// An unterminated block has no semantic errors for the lines it swallowed.
	f, errs := Parse("gocpp.mod", []byte("module m\nrequire (\n\tbad\n\talso bad\n"), nil)
	if len(errs) != 1 || len(f.Require) != 0 {
		t.Errorf("errors %v, require %v", errs, f.Require)
	}
}

// TestShapes checks that a keyword in a wrong shape is an error, not an
// unknown statement that is kept and ignored.
func TestShapes(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"require(", "require(\n\ta v1.0.0\n)\n", "gocpp.mod:1: syntax error (missing space between require and ()"},
		{"replace(", "replace(\n)\n", "gocpp.mod:1: syntax error (missing space between replace and ()"},
		{"cxx{", "cxx{\n}\n", "gocpp.mod:1: syntax error (missing space between cxx and {)"},
		{"pkg-config(", "pkg-config(a b)\n", "gocpp.mod:1: syntax error (missing space between pkg-config and (a)"},
		{"require block with words", "require a (\n\tb v1.0.0\n)\n", "gocpp.mod:1: unknown block type: require a ("},
		{"require brace block", "require {\n}\n", "gocpp.mod:1: unknown block type: require {"},
		{"cxx paren block with words", "cxx c++20 (\n)\n", "gocpp.mod:1: unknown block type: cxx c++20 ("},
		{"indented module path", "module m\n\texample.com/c v1.0.0\n", "gocpp.mod:2:2: unknown directive: example.com/c"},
		{"lone quote", "\"\n", "gocpp.mod:1: unknown directive: \""},
		{"version as verb", "v1.0.0\n", "gocpp.mod:1: unknown directive: v1.0.0"},
		{"unknown block", "Foo (\n)\n", "gocpp.mod:1: unknown directive: Foo"},
		{"line in a block of an unknown verb is kept", "foo (\n\tExample.com/x v1\n)\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, errs := Parse("gocpp.mod", []byte(tc.src), nil); errs.Error() != tc.want {
				t.Errorf("errors:\n%s\nwant\n%s", errs, tc.want)
			}
		})
	}
}

func TestVersions(t *testing.T) {
	for v, ok := range map[string]bool{
		"v1.2.3-01": false, "v1.2.3-a.01": false, "v1.2.3-0": true, "v1.2.3-0.1": true, "v1.2.3-a.0": true, "v1.2.3-a01": true,
		"v1.2.3-00": false, "v1.2.3-1a": true, "v1.2.3-rc..1": false, "v1.2.3-0x": true, "v1.2.3+01": true,
	} {
		if ValidVersion(v) != ok {
			t.Errorf("ValidVersion(%q) = %v", v, !ok)
		}
	}
	for v, want := range map[string]string{
		"v1.2.3": "v1.2.3", "v1.2.3+meta": "v1.2.3", "v1.2.3-rc.1+meta": "v1.2.3-rc.1", "v2.0.0+incompatible": "v2.0.0+incompatible",
		"": "", "v1": "v1", "v1.2.3+a+b": "v1.2.3+a+b",
	} {
		if got := CanonicalVersion(v); got != want {
			t.Errorf("CanonicalVersion(%q) = %q, want %q", v, got, want)
		}
	}
	// The file keeps the canonical version, so it compares with a
	// vendor/modules.txt that Go wrote.
	f := mustParse(t, "module m\nrequire a v1.0.0+meta\nrequire b v2.0.0+incompatible\nreplace a v1.0.0+meta => ../a\n")
	if f.Require[0].Version != "v1.0.0" || f.Require[1].Version != "v2.0.0+incompatible" || f.Replace[0].OldVersion != "v1.0.0" {
		t.Errorf("%+v %+v %+v", f.Require[0], f.Require[1], f.Replace[0])
	}
	v, errs := ParseVendor("m.txt", []byte("# a v1.0.0+meta\n## explicit\n"))
	if len(errs) > 0 || v.Modules[0].Version != "v1.0.0" {
		t.Errorf("%v %+v", errs, v.Modules)
	}
	if got := CheckVendor(f, &VendorModules{Modules: []*VendorModule{{Path: "a", Version: "v1.0.0", Explicit: true}, {Path: "b", Version: "v2.0.0+incompatible", Explicit: true}}}); len(got) != 1 {
		t.Errorf("problems: %q", got) // only the replace of a is not marked
	}
	// The messages of an invalid version show it as it was written.
	if _, errs := Parse("gocpp.mod", []byte("require a v1.2.3-01\n"), nil); errs.Error() != `gocpp.mod:1: require a: invalid module version "v1.2.3-01": must be of the form v1.2.3` {
		t.Errorf("errors: %v", errs)
	}
}

func TestBackquote(t *testing.T) {
	f := mustParse(t, "cflags `-DA=a b` -DB=`c d` `\\x`\n")
	if want := []string{"-DA=a b", "-DB=c d", `\x`}; !reflect.DeepEqual(f.Cflags[0].Args, want) {
		t.Errorf("args = %q, want %q", f.Cflags[0].Args, want)
	}
	for in, want := range map[string]string{"`a": "unterminated quote", "``": "empty argument"} {
		if _, msg := SplitWord(in); msg != want {
			t.Errorf("SplitWord(%q) = %q", in, msg)
		}
	}
}

func TestFileSizeLimit(t *testing.T) {
	old := maxFileSize
	maxFileSize = 1000
	t.Cleanup(func() { maxFileSize = old })
	big := []byte(strings.Repeat("// x\n", 201)) // 1005 bytes
	if _, errs := Parse("gocpp.mod", big, nil); len(errs) != 1 || errs.Error() != "gocpp.mod:1: file is too large (1005 bytes, the limit is 1000)" {
		t.Errorf("Parse: %v", errs)
	}
	if _, errs := ParseVendor("m.txt", big); len(errs) != 1 || errs.Error() != "m.txt:1: file is too large (1005 bytes, the limit is 1000)" {
		t.Errorf("ParseVendor: %v", errs)
	}
	if _, errs := Parse("gocpp.mod", big[:1000], nil); len(errs) != 0 {
		t.Errorf("a file of the limit: %v", errs)
	}
	if MaxFileSize != 16<<20 {
		t.Errorf("MaxFileSize = %d", MaxFileSize)
	}
}

// bestOf returns the shortest of three runs of f.
func bestOf(f func()) time.Duration {
	best := time.Duration(1 << 62)
	for range 3 {
		start := time.Now()
		f()
		best = min(best, time.Since(start))
	}
	return best
}

// TestScales checks that nothing is quadratic in the size of the file, which
// may be the file of a dependency: six times the lines must not take 14
// times as long (it takes about six; a quadratic one takes 36).
func TestScales(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	gen := func(n int) (mod, vendor string) {
		var m, v, rep strings.Builder
		m.WriteString("module m\nrequire (\n")
		for i := range n {
			fmt.Fprintf(&m, "\texample.com/m%d v1.0.%d\n", i, i)
			fmt.Fprintf(&v, "# example.com/m%d v1.0.%d => ../m%d\n## explicit\nexample.com/m%d\n", i, i, i, i)
			fmt.Fprintf(&rep, "replace example.com/m%d => ../m%d\n", i, i)
		}
		m.WriteString(")\n")
		m.WriteString(rep.String())
		return m.String(), v.String()
	}
	type run struct{ parse, vendor, errors, find func() }
	mk := func(n int) run {
		mod, vendor := gen(n)
		f, _ := Parse("gocpp.mod", []byte(mod), nil)
		v, _ := ParseVendor("m.txt", []byte(vendor))
		bad := strings.Repeat("bad bad\n", n)
		return run{
			parse:  func() { Parse("gocpp.mod", []byte(mod), nil) },
			vendor: func() { CheckVendor(f, v) },
			errors: func() { Parse("gocpp.mod", []byte(bad), nil) },
			find: func() {
				for i := range n {
					p := fmt.Sprintf("example.com/m%d/sub/x", i)
					f.FindRequire(p)
					f.FindReplace(fmt.Sprintf("example.com/m%d", i), "")
					f.FindReplacedPath(p)
				}
			},
		}
	}
	small, large := mk(3000), mk(18000)
	for name, pair := range map[string][2]func(){
		"Parse":       {small.parse, large.parse},
		"CheckVendor": {small.vendor, large.vendor},
		"errors":      {small.errors, large.errors},
		"lookups":     {small.find, large.find},
	} {
		// A stall of a loaded machine looks like a slow run: a real quadratic
		// loop is slow every time.
		var a, b time.Duration
		for attempt := 0; attempt < 5; attempt++ {
			a, b = bestOf(pair[0]), bestOf(pair[1])
			t.Logf("%s: %v for 3000, %v for 18000", name, a, b)
			if a <= time.Millisecond || b <= 14*a {
				break
			}
			if attempt == 4 {
				t.Errorf("%s is not linear: %v for 3000 lines, %v for 18000", name, a, b)
			}
		}
	}
}

// TestLookups checks the indexed lookups of a File.
func TestLookups(t *testing.T) {
	f := mustParse(t, `module m
require example.com/a v1.0.0
require example.com/a/b v2.0.0
require example.com/c v1.0.0
require example.com/c v1.0.0
replace example.com/a => ../a
replace example.com/a v1.0.0 => ../a1
replace example.com/z v1.0.0 => ../z
`)
	for path, want := range map[string]string{
		"example.com/a": "example.com/a", "example.com/a/x": "example.com/a", "example.com/a/b": "example.com/a/b",
		"example.com/a/b/c/d": "example.com/a/b", "example.com/c/x/y": "example.com/c", "example.com/ab": "", "example.com": "", "": "",
	} {
		got := ""
		if r := f.FindRequire(path); r != nil {
			got = r.Path
		}
		if got != want {
			t.Errorf("FindRequire(%q) = %q, want %q", path, got, want)
		}
	}
	for _, tc := range []struct{ path, version, want string }{
		{"example.com/a", "v1.0.0", "../a1"}, {"example.com/a", "v1.1.0", "../a"}, {"example.com/a", "", "../a"},
		{"example.com/z", "v1.0.0", "../z"}, {"example.com/z", "v1.1.0", ""}, {"example.com/z", "", ""}, {"example.com/q", "v1.0.0", ""},
	} {
		got := ""
		if r := f.FindReplace(tc.path, tc.version); r != nil {
			got = r.NewPath
		}
		if got != tc.want {
			t.Errorf("FindReplace(%q, %q) = %q, want %q", tc.path, tc.version, got, tc.want)
		}
	}
	if r := f.FindReplacedPath("example.com/z/sub"); r == nil || r.NewPath != "../z" {
		t.Errorf("FindReplacedPath = %+v", r)
	}
	if f.FindReplacedPath("example.com/c") != nil {
		t.Error("FindReplacedPath of an unreplaced module")
	}
}

// summary describes what a file says, without positions, to compare two
// parses.
func summary(f *File) string {
	var b strings.Builder
	if f.Module != nil {
		fmt.Fprintf(&b, "module %q;", f.Module.Path)
	}
	if f.Gocpp != nil {
		fmt.Fprintf(&b, "gocpp %q;", f.Gocpp.Version)
	}
	if f.Cxx != nil {
		fmt.Fprintf(&b, "cxx %q;", f.Cxx.Std)
	}
	for _, r := range f.Require {
		fmt.Fprintf(&b, "require %q %q %v;", r.Path, r.Version, r.Indirect)
	}
	for _, r := range f.Replace {
		fmt.Fprintf(&b, "replace %q %q %q %q;", r.OldPath, r.OldVersion, r.NewPath, r.NewVersion)
	}
	for _, l := range []struct {
		name string
		list []*Args
	}{{"cflags", f.Cflags}, {"ldflags", f.Ldflags}, {"pkg-config", f.PkgConfig}} {
		for _, a := range l.list {
			fmt.Fprintf(&b, "%s %q;", l.name, a.Args)
		}
	}
	fmt.Fprintf(&b, "unknown %d", len(f.Unknown))
	return b.String()
}

func TestSummaryKept(t *testing.T) {
	// Formatting a file that parsed without errors changes nothing it says.
	f := mustParse(t, recipe+"gocpp 0.1\ncflags -Dx \"-Dy z\"\nrequire a v1.0.0 // indirect\nreplace a v1.0.0 => ../a\ngo 1.25\n")
	g, errs := Parse("gocpp.mod", Format(f.Syntax), nil)
	if len(errs) > 0 || summary(f) != summary(g) {
		t.Errorf("%v\n%s\n%s", errs, summary(f), summary(g))
	}
}

func FuzzParseVendor(f *testing.F) {
	for _, s := range []string{vendorText, "", "# a v1.0.0 => ../b\n## explicit\n", "# a\n", "## explicit\n", "x\n", "# a v1.0.0+m => b v1.0.0\n", "\xff"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		v, _ := ParseVendor("m.txt", []byte(in))
		file, _ := Parse("gocpp.mod", []byte("module m\nrequire a v1.0.0\nreplace a => ../a\n"), nil)
		CheckVendor(file, v)
		for _, m := range v.Modules {
			if v.Module(m.Path) == nil {
				t.Fatalf("module %q is not found", m.Path)
			}
		}
	})
}
