package gotest

import (
	"bytes"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	gpparser "github.com/huuhait/gocpp/internal/syntax/parser"
)

// scan parses the sources (name -> text; names ending in _test.gp are test files) and scans them
// as the package under test.
func scan(t *testing.T, srcs map[string]string, xsrcs map[string]string) (*Found, []string) {
	t.Helper()
	fset := token.NewFileSet()
	mk := func(srcs map[string]string) *load.Package {
		p := &load.Package{ImportPath: "example.com/m/geom"}
		for _, name := range sortedKeys(srcs) {
			f, err := gpparser.ParseFile(fset, name, srcs[name], gpparser.ParseComments|gpparser.Phase10Syntax)
			if err != nil {
				t.Fatal(err)
			}
			p.Files = append(p.Files, &load.File{Path: name, AST: f, Test: strings.HasSuffix(name, "_test.gp")})
		}
		return p
	}
	p := mk(srcs)
	var xt *load.Package
	if xsrcs != nil {
		xt = mk(xsrcs)
	}
	found, errs := Scan(fset, p, xt)
	var msgs []string
	for _, e := range errs {
		msgs = append(msgs, e.Pos.String()+": "+e.Msg)
	}
	return found, msgs
}

func sortedKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func names(fs []Func) string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Pkg+"."+f.Name)
	}
	return strings.Join(out, " ")
}

func TestIsTest(t *testing.T) {
	for _, tt := range []struct {
		name, prefix string
		want         bool
	}{
		{"Test", "Test", true},
		{"TestX", "Test", true},
		{"Test1", "Test", true},
		{"Test_x", "Test", true},
		{"Testx", "Test", false},
		{"Testé", "Test", false},
		{"TestÉ", "Test", true},
		{"BenchmarkX", "Test", false},
		{"BenchmarkX", "Benchmark", true},
		{"Example_suffix", "Example", true},
		{"Examplef", "Example", false},
		{"", "Test", false},
	} {
		if got := IsTest(tt.name, tt.prefix); got != tt.want {
			t.Errorf("IsTest(%q, %q) = %v, want %v", tt.name, tt.prefix, got, tt.want)
		}
	}
}

func TestScanFunctions(t *testing.T) {
	found, errs := scan(t, map[string]string{
		"geom.gp": `package geom
func TestNotInTestFile(t *testing.T) {}
`,
		"a_test.gp": `package geom
import "testing"
func TestB(t *testing.T) {}
func TestA(t *testing.T) {}
func Testlower(t *testing.T) {}
func helper(t *testing.T) {}
func (x X) TestMethod(t *testing.T) {}
func BenchmarkSpeed(b *testing.B) {}
func FuzzParse(f *testing.F) {}
func ExampleHello() {
	// Output: hi
}
`,
		"b_test.gp": `package geom
import . "testing"
func TestDot(t *T) {}
func TestNamed(tt *testing.T) {}
func TestMain(m *testing.M) {}
`,
	}, nil)
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	if got, want := names(found.Tests), "_test.TestB _test.TestA _test.TestDot _test.TestNamed"; got != want {
		t.Errorf("tests = %q, want %q", got, want)
	}
	if got, want := names(found.Benchmarks), "_test.BenchmarkSpeed"; got != want {
		t.Errorf("benchmarks = %q, want %q", got, want)
	}
	if got, want := names(found.Fuzz), "_test.FuzzParse"; got != want {
		t.Errorf("fuzz = %q, want %q", got, want)
	}
	if got, want := names(found.Examples), "_test.ExampleHello"; got != want {
		t.Errorf("examples = %q, want %q", got, want)
	}
	if found.TestMain == nil || found.TestMain.Name != "TestMain" || found.TestMain.Pkg != "_test" {
		t.Errorf("TestMain = %+v", found.TestMain)
	}
	if !found.NeedTest || found.NeedXTest || !found.ImportTest || found.ImportXTest {
		t.Errorf("imports: %+v", found)
	}
}

func TestScanExternalTests(t *testing.T) {
	found, errs := scan(t, map[string]string{"geom.gp": "package geom\n"},
		map[string]string{"x_test.gp": `package geom_test
import "testing"
func TestX(t *testing.T) {}
func TestMain(m *testing.M) {}
`})
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	if got := names(found.Tests); got != "_xtest.TestX" {
		t.Errorf("tests = %q", got)
	}
	if found.TestMain == nil || found.TestMain.Pkg != "_xtest" {
		t.Errorf("TestMain = %+v", found.TestMain)
	}
	if found.NeedTest || !found.NeedXTest || !found.ImportTest || !found.ImportXTest {
		t.Errorf("imports: %+v", found)
	}
}

// TestMainWithTestFunc: a TestMain(t *testing.T) is a test like any other (go test's rule).
func TestMainWithTestFunc(t *testing.T) {
	found, errs := scan(t, map[string]string{"a_test.gp": "package p\nimport \"testing\"\nfunc TestMain(t *testing.T) {}\n"}, nil)
	if len(errs) > 0 || names(found.Tests) != "_test.TestMain" || found.TestMain != nil {
		t.Errorf("found %+v, errors %v", found, errs)
	}
}

func TestScanErrors(t *testing.T) {
	for _, tt := range []struct{ name, src, want string }{
		{"no param", "func TestX() {}", "a_test.gp:3:1: wrong signature for TestX, must be: func TestX(t *testing.T)"},
		{"wrong type", "func TestX(t *testing.B) {}", "wrong signature for TestX, must be: func TestX(t *testing.T)"},
		{"not a pointer", "func TestX(t testing.T) {}", "wrong signature for TestX, must be: func TestX(t *testing.T)"},
		{"result", "func TestX(t *testing.T) int { return 0 }", "wrong signature for TestX, must be: func TestX(t *testing.T)"},
		{"two params", "func TestX(t *testing.T, n int) {}", "wrong signature for TestX, must be: func TestX(t *testing.T)"},
		{"benchmark", "func BenchmarkX(t *testing.T) {}", "wrong signature for BenchmarkX, must be: func BenchmarkX(b *testing.B)"},
		{"fuzz", "func FuzzX(t *testing.T) {}", "wrong signature for FuzzX, must be: func FuzzX(f *testing.F)"},
		{"testmain", "func TestMain() {}", "wrong signature for TestMain, must be: func TestMain(m *testing.M)"},
		{"type params", "func TestX[T any](t *testing.T) {}", "wrong signature for TestX, test functions cannot have type parameters"},
		{"two testmains", "func TestMain(m *testing.M) {}\nfunc TestMain(m *testing.M) {}", "multiple definitions of TestMain"},
		{"must test", "func TestX!(t *testing.T) {}", "a_test.gp:3:1: TestX! is a must function: it gets an error result, so it is not a test function; write func TestX without the !"},
		{"must benchmark", "func BenchmarkX!(b *testing.B) {}", "BenchmarkX! is a must function: it gets an error result, so it is not a benchmark function"},
		{"must example", "func ExampleX!() {}", "ExampleX! is a must function: it gets an error result, so it is not an example function"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := scan(t, map[string]string{"a_test.gp": "package p\nimport \"testing\"\n" + tt.src + "\n"}, nil)
			if len(errs) == 0 {
				t.Fatalf("no error, want %q", tt.want)
			}
			if !strings.Contains(errs[0], tt.want) {
				t.Errorf("error %q, want %q", errs[0], tt.want)
			}
		})
	}
}

func TestExampleOutput(t *testing.T) {
	for _, tt := range []struct {
		name, body, out string
		unordered, ok   bool
	}{
		{"plain", "// Output: hello", "hello\n", false, true},
		{"lines", "fmt.Println(1)\n\t// Output:\n\t// a\n\t// b", "a\nb\n", false, true},
		{"empty", "// Output:", "", false, true},
		{"unordered", "// Unordered output:\n\t// b\n\t// a", "b\na\n", true, true},
		{"case", "// OUTPUT: x", "x\n", false, true},
		{"none", "fmt.Println(1)", "", false, false},
		{"not last", "// Output: x\n\tf()\n\t// something else", "", false, false},
		{"last wins", "// first\n\tf()\n\t// Output: y", "y\n", false, true},
		{"spaces", "// Output:   z", "z\n", false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			found, errs := scan(t, map[string]string{"a_test.gp": "package p\nfunc ExampleE() {\n\t" + tt.body + "\n}\n"}, nil)
			if len(errs) > 0 {
				t.Fatal(errs)
			}
			if !tt.ok {
				if len(found.Examples) != 0 {
					t.Fatalf("examples = %+v, want none", found.Examples)
				}
				return
			}
			if len(found.Examples) != 1 {
				t.Fatalf("examples = %+v", found.Examples)
			}
			if e := found.Examples[0]; e.Output != tt.out || e.Unordered != tt.unordered {
				t.Errorf("example = {%q, %v}, want {%q, %v}", e.Output, e.Unordered, tt.out, tt.unordered)
			}
		})
	}
}

// An example with parameters or results is not an example; one without an output comment is
// compiled (its file is imported) and not run.
func TestExampleShapes(t *testing.T) {
	found, errs := scan(t, map[string]string{"a_test.gp": `package p
func ExampleParam(x int) {
	// Output: no
}
func ExampleResult() int {
	// Output: no
	return 0
}
func ExampleNoOutput() {}
`}, nil)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	if len(found.Examples) != 0 {
		t.Errorf("examples = %+v", found.Examples)
	}
	if !found.NeedTest {
		t.Error("the package of an example without output is still used")
	}
}

func TestHasTestFiles(t *testing.T) {
	plain := &load.Package{Files: []*load.File{{Path: "a.gp"}}}
	withTests := &load.Package{Files: []*load.File{{Path: "a.gp"}, {Path: "a_test.gp", Test: true}}}
	xt := &load.Package{Files: []*load.File{{Path: "x_test.gp", Test: true}}}
	if HasTestFiles(plain, nil) {
		t.Error("plain package has test files")
	}
	if !HasTestFiles(withTests, nil) || !HasTestFiles(plain, xt) {
		t.Error("test files not found")
	}
}

func TestMainSource(t *testing.T) {
	found := &Found{
		Tests:       []Func{{Pkg: "_test", Name: "TestA"}, {Pkg: "_xtest", Name: "TestB"}},
		Benchmarks:  []Func{{Pkg: "_test", Name: "BenchmarkA"}},
		Examples:    []Func{{Pkg: "_test", Name: "ExampleA", Output: "a\nb\n"}, {Pkg: "_xtest", Name: "ExampleB", Output: "q\"x", Unordered: true}},
		ImportTest:  true,
		ImportXTest: true,
		NeedTest:    true,
		NeedXTest:   true,
	}
	got := string(Main("example.com/m/geom", found))
	want := `// Code generated by gocpp test. DO NOT EDIT.

package testmain

import (
	"testing"
	"testing/internal/testdeps"

	_test "example.com/m/geom"
	_xtest "example.com/m/geom_test"
)

var tests = []testing.InternalTest{
	{"TestA", _test.TestA},
	{"TestB", _xtest.TestB},
}

var benchmarks = []testing.InternalBenchmark{
	{"BenchmarkA", _test.BenchmarkA},
}

var fuzzTargets = []testing.InternalFuzzTarget{
}

var examples = []testing.InternalExample{
	{"ExampleA", _test.ExampleA, "a\nb\n", false},
	{"ExampleB", _xtest.ExampleB, "q\"x", true},
}

func init() {
	testdeps.ImportPath = "example.com/m/geom"
}

func main() {
	m := testing.MainStart(testdeps.TestDeps{}, tests, benchmarks, fuzzTargets, examples)
	m.Run()
	testdeps.Exit()
}
`
	if got != want {
		t.Errorf("Main =\n%s\nwant\n%s", got, want)
	}
}

func TestMainBlankImportsAndTestMain(t *testing.T) {
	found := &Found{ImportTest: true, ImportXTest: true, NeedXTest: true, TestMain: &Func{Pkg: "_xtest", Name: "TestMain"}}
	got := string(Main("p", found))
	for _, want := range []string{"\t_ \"p\"\n", "\t_xtest \"p_test\"\n", "\t_xtest.TestMain(m)\n", "\ttestdeps.Exit()\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("Main lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "m.Run()") {
		t.Errorf("Main calls m.Run itself with a TestMain:\n%s", got)
	}
}

// fuzzRewritten scans a test file with fuzz targets and returns the source of the file after the rewrite.
func fuzzRewritten(t *testing.T, src string, extra string) string {
	t.Helper()
	fset := token.NewFileSet()
	var files []*load.File
	for name, text := range map[string]string{"a_test.gp": src, "b.gp": extra} {
		if text == "" {
			continue
		}
		f, err := parser.ParseFile(fset, name, text, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, &load.File{Path: name, AST: f, Test: strings.HasSuffix(name, "_test.gp")})
	}
	if _, errs := Scan(fset, &load.Package{ImportPath: "p", Files: files}, nil); len(errs) > 0 {
		t.Fatal(errs)
	}
	var out bytes.Buffer
	for _, f := range files {
		if f.Test {
			if err := printer.Fprint(&out, fset, f.AST); err != nil {
				t.Fatal(err)
			}
		}
	}
	return out.String()
}

func TestRewriteFuzz(t *testing.T) {
	const head = "package p\n\nimport \"testing\"\n\n"
	flat := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	tests := []struct {
		name, src, extra string
		want             []string // pieces of the rewritten source (whitespace collapsed)
		not              []string
	}{
		{name: "literal", src: head + `func FuzzX(f *testing.F) { f.Add(1, "a"); f.Fuzz(func(t *testing.T, n int, s string) { _ = n }) }`,
			want: []string{"f.GocppFuzz(func(gocppT *testing.T, gocppVals []interface{}) {", "func(t *testing.T, n int, s string) { _ = n }(gocppT, gocppVals[0].(int), gocppVals[1].(string))", "[]interface{}{*new(int), *new(string)})"},
			not:  []string{"f.Fuzz("}},
		{name: "grouped parameters and byte slices", src: head + `func FuzzX(f *testing.F) { f.Fuzz(func(t *testing.T, a, b []byte, c rune) {}) }`,
			want: []string{"gocppVals[0].([]byte), gocppVals[1].([]byte), gocppVals[2].(rune)", "[]interface{}{*new([]byte), *new([]byte), *new(rune)}"}},
		{name: "function of the package", extra: "package p\n\nimport \"testing\"\n\nfunc fuzzOne(t *testing.T, x uint8) {}\n", src: head + `func FuzzX(f *testing.F) { f.Fuzz(fuzzOne) }`,
			want: []string{"fuzzOne(gocppT, gocppVals[0].(uint8))"}},
		{name: "other receiver name and nested", src: head + `func FuzzX(g *testing.F) { for i := 0; i < 2; i++ { g.Fuzz(func(t *testing.T, b bool) {}) } }`,
			want: []string{"g.GocppFuzz("}},
		{name: "not fuzzable type is left alone", src: head + `func FuzzX(f *testing.F) { f.Fuzz(func(t *testing.T, p *int) {}) }`,
			want: []string{"f.Fuzz(func(t *testing.T, p *int) {})"}, not: []string{"GocppFuzz"}},
		{name: "unknown function is left alone", src: head + `func FuzzX(f *testing.F) { f.Fuzz(helper) }`,
			want: []string{"f.Fuzz(helper)"}, not: []string{"GocppFuzz"}},
		{name: "no second parameter is left alone", src: head + `func FuzzX(f *testing.F) { f.Fuzz(func(t *testing.T) {}) }`,
			want: []string{"f.Fuzz(func(t *testing.T) {})"}},
		{name: "other receiver is left alone", src: head + `func FuzzX(f *testing.F) { var o interface{ Fuzz(func(t *testing.T, n int)) }; o.Fuzz(func(t *testing.T, n int) {}) }`,
			want: []string{"o.Fuzz(func(t *testing.T, n int) {})"}, not: []string{"GocppFuzz"}},
		{name: "a test is not touched", src: head + `func TestX(t *testing.T) { var f *testing.F; f.Fuzz(func(t *testing.T, n int) {}) }`,
			want: []string{"f.Fuzz(func(t *testing.T, n int) {})"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flat(fuzzRewritten(t, tt.src, tt.extra))
			for _, w := range tt.want {
				if !strings.Contains(got, flat(w)) {
					t.Errorf("the rewritten source lacks %q:\n%s", w, got)
				}
			}
			for _, n := range tt.not {
				if strings.Contains(got, n) {
					t.Errorf("the rewritten source has %q:\n%s", n, got)
				}
			}
		})
	}
}
