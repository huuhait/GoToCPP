package lower

import (
	"context"
	"errors"
	"fmt"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// checkInterop loads and checks the program src (the file main.gp) without a C++ compiler: the
// namespace probe says that every candidate is a namespace. It returns the program before the
// check too, for ProbePaths.
func checkInterop(t *testing.T, src string) (*token.FileSet, *check.Package, func(string, []string) []string) {
	t.Helper()
	return checkFiles(t, map[string]string{"main.gp": src})
}

// checkFiles is checkInterop for a package of several files (by name).
func checkFiles(t *testing.T, files map[string]string) (*token.FileSet, *check.Package, func(string, []string) []string) {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prog := load.Load(load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}, ".")
	paths := ProbePaths(prog)
	r := check.Check(prog, check.Config{
		Phase: check.LastPhase,
		CXX:   func(context.Context) (build.CXX, error) { return build.CXX{}, nil },
		Probe: func(_ context.Context, _ build.CXX, _ string, _, _, cands []string) (interop.Result, error) {
			kinds := make(map[string]interop.Kind)
			for _, c := range cands {
				kinds[c] = interop.Namespace
			}
			return interop.Result{Kinds: kinds}, nil
		},
	})
	pkg := r.Package(prog.Roots[0])
	if pkg.Errors.HasErrors() || prog.Errors.HasErrors() {
		var b strings.Builder
		for _, d := range append(prog.Errors.Diagnostics(), pkg.Errors.Diagnostics()...) {
			b.WriteString(d.Pos.String() + ": " + d.Msg + "\n")
		}
		t.Fatalf("the program has errors:\n%s", b.String())
	}
	return r.Fset, pkg, func(_ string, includes []string) []string { return paths(dir, includes) }
}

// fakeInterop returns the probes' answers kinds and macros for every file of pkg.
func fakeInterop(pkg *check.Package, kinds map[string]interop.Kind, macros ...string) *Interop {
	in := &Interop{kinds: make(map[*load.File]map[string]interop.Kind), macros: make(map[string]bool)}
	for _, f := range pkg.Load.Files {
		in.kinds[f] = kinds
	}
	for _, m := range macros {
		in.macros[m] = true
	}
	return in
}

const interopSrc = `package main

import (
	"<lib.h>"
	"<vector>"
	fs "::std::filesystem"
)

type Bool bool

type Point struct{ X int }

func f(n C.int, p *lib.FILE) C.int { return n }

func main() {
	lib.f(1, "s", 2.5, 'c', nil, true, -3, 1<<62, uint64(1)<<63, 1<<63)
	m := lib.MAC
	var v std.vector[Point]
	var a std.array[C.int, 4]
	var it std.vector[int].iterator
	var p fs.path
	_ = std.chrono.clock.now()
	std.out.flush()
	var tm lib.struct_tm
	var g lib.vec3
	var b Bool
	var n C.ulonglong = 5
	_, _, _, _, _, _, _, _, _ = m, v, a, it, p, tm, g, b, n
}
`

// TestProbePaths checks the paths that the lowerer asks the kind probe about, which the
// driver passes to the check's probe too.
func TestProbePaths(t *testing.T) {
	_, pkg, paths := checkInterop(t, interopSrc)
	got := paths("", []string{"<lib.h>", "<vector>"})
	want := []string{"lib", "FILE", "f", "MAC", "std::vector", "std::array", "std::filesystem::path",
		"std::chrono", "std::chrono::clock", "std::chrono::clock::now", "std::out", "std::out::flush", "vec3"}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("ProbePaths:\n got %q\nwant %q", got, want)
	}
	// A header name that is a namespace too: its names are looked up in it as well.
	got = probePaths(pkg.Load.Files[0], map[string]bool{"lib": true})
	for _, p := range []string{"lib::FILE", "lib::vec3", "lib::f"} {
		if !slices.Contains(got, p) {
			t.Errorf("probePaths with the namespace lib: no %s in %q", p, got)
		}
	}
}

// TestCNames checks the C++ of the C/C++ names (cnames.go), of the C/C++ types (ctypes.go), of
// the C literals (cvalues.go) and the macro protection (macros.go), with the probes' answers
// given.
func TestCNames(t *testing.T) {
	fset, pkg, _ := checkInterop(t, interopSrc)
	kinds := map[string]interop.Kind{
		"lib": interop.Namespace, "lib::vec3": interop.Type, "lib::f": interop.NotFound, "lib::FILE": interop.NotFound,
		"f": interop.Value, "MAC": interop.Macro, "FILE": interop.Type, "vec3": interop.Type,
		"std::vector": interop.Template, "std::array": interop.Template, "std::filesystem::path": interop.Type,
		"std::chrono": interop.Namespace, "std::chrono::clock": interop.Type, "std::chrono::clock::now": interop.Value,
		"std::out": interop.Value,
	}
	f, err := Package(fset, pkg, fakeInterop(pkg, kinds, "Bool", "run_main", "MAC", "X"))
	if err != nil {
		t.Fatal(err)
	}
	cpp := string(cppir.Print(f, ""))
	for _, want := range []string{
		"#include <lib.h>\n#include <vector>\n#include <gocpp/fmtcxx.hpp>\n#undef run_main\n",
		`::f(1, "s", 2.5, 'c', nullptr, true, -3, 4611686018427387904, UINT64_C(9223372036854775808), 9223372036854775808ULL);`,
		"auto m = MAC;",
		"::std::vector<::gpkg_main::Point> v{};",
		"::std::array<int, 4> a{};",
		"::std::vector<int64_t>::iterator it{};",
		"::std::filesystem::path p{};",
		"::std::chrono::clock::now()",
		"gp::deref(::std::out).flush();",
		"struct ::tm tm{};",
		"::lib::vec3 g{};", // the namespace first
		"Bool_ b{};",       // a macro of the headers: renamed
		"int64_t X_ = 0;",  // a field too
		"unsigned long long n = 5;",
		"int f(int n, gp::cptr<::FILE> p) {", // a gocpp pointer to a C type
	} {
		if !strings.Contains(cpp, want) {
			t.Errorf("no %q in the C++:\n%s", want, cpp)
		}
	}
	if strings.Contains(cpp, "#undef MAC") || strings.Contains(cpp, "#undef Bool") {
		t.Errorf("a macro that the code uses, or a renamed name's, is #undef'd:\n%s", cpp)
	}
}

// TestBorrowForeignEmission checks that implicit foreign operations preserve the call's owners,
// for both a borrowed pointer and a const-reference value. The signatures can still optimize.
func TestBorrowForeignEmission(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"local", `var v std.vector[int]
		_ = v`},
		{"aggregate", `var v Foreign
		_ = v`},
		{"array", `var v [1]Foreign
		_ = v`},
		{"slice", `v := make([]Foreign, 1)
		_ = v`},
		{"copy", `var v Foreign
		copied := v
		_ = copied`},
		{"assignment", `var first Foreign
		var second Foreign
		first = second
		_ = first`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `package main
			import "<vector>"
			type Node struct{ val int }
			type Foreign struct{ v std.vector[int] }
			var global *Node
			var text string
			func read(p *Node, s string) int {
				` + tt.body + `
				return p.val + len(s)
			}
			func main() { _ = read(global, text) }
			`
			fset, pkg, _ := checkInterop(t, src)
			kinds := map[string]interop.Kind{
				"std":         interop.Namespace,
				"std::vector": interop.Template,
			}
			file, err := Package(fset, pkg, fakeInterop(pkg, kinds))
			if err != nil {
				t.Fatal(err)
			}

			cpp := string(cppir.Print(file, ""))
			for _, want := range []string{
				"read(gp::bptr<Node> p, const gp::string& s)",
				"read(gp::ptr<Node>(global), gp::string(text))",
			} {
				if !strings.Contains(cpp, want) {
					t.Errorf("no %q in the C++:\n%s", want, cpp)
				}
			}
		})
	}
}

func TestBorrowNamedMethodValueEmission(t *testing.T) {
	cpp := borrowForeignCPP(t, `package main
	type Scalar int
	func (s *Scalar) Read() int { return int(*s) }
	func bind(p *Scalar) func() int { return (*p).Read }
	func main() { _ = bind(new(Scalar)) }
	`)
	if !strings.Contains(cpp, "bind(const gp::ptr<Scalar>& p)") {
		t.Fatalf("retained scalar method receiver is not an owning pointer:\n%s", cpp)
	}
	if strings.Contains(cpp, "bind(gp::bptr<Scalar> p)") {
		t.Fatalf("returned method value captured a borrowed receiver:\n%s", cpp)
	}
}

func TestBorrowSiblingForeignEmission(t *testing.T) {
	tests := []struct {
		name string
		call string
	}{
		{"slice construction", "read(global, text, len([]Foreign{foreign}))"},
		{"aggregate conversion", "read(global, text, len([]Foreign{Foreign(foreign)}))"},
		{"method receiver", "global.read(text, len([]Foreign{foreign}))"},
		{"func value", "fn(global, text, len([]Foreign{foreign}))"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `package main
			import "<vector>"
			type Node struct{ val int }
			type Foreign struct{ value std.vector[int] }
			var global *Node
			var text string
			var foreign Foreign
			func read(p *Node, s string, n int) int { return p.val + len(s) + n }
			func (p *Node) read(s string, n int) int { return p.val + len(s) + n }
			var fn = read
			func main() { _ = ` + tt.call + ` }
			`
			cpp := borrowForeignCPP(t, src)
			main := cpp[strings.Index(cpp, "void main() {"):]
			pointer := strings.Index(main, " = global;")
			text := strings.Index(main, " = text;")
			foreign := strings.Index(main, "gp::slice_of<Foreign>")
			if pointer < 0 || text <= pointer || foreign <= text {
				t.Fatalf("argument owners were not collected before foreign copies:\n%s", main)
			}
			if strings.Contains(main, "gp::lend(global)") {
				t.Fatalf("foreign sibling retained a raw lend of the source:\n%s", main)
			}
		})
	}
}

func TestBorrowNestedForeignSnapshots(t *testing.T) {
	tests := []struct {
		name     string
		argument string
	}{
		{"map literal", "len(map[int]Foreign{0: foreign})"},
		{"slice expression", "len([]Foreign{foreign}[:])"},
		{"bounded slice", "len([]Foreign{foreign}[0:1])"},
		{"nested slices", "len([]Foreign{foreign}[:][:])"},
		{"append", "len(append([]Foreign{}, foreign))"},
		{"map key call", "len(map[int]Foreign{change(): foreign})"},
		{"slice bound call", "len([]Foreign{foreign}[:change()+1])"},
		{"call after map", "len(map[int]Foreign{0: foreign}) + change()"},
		{"call before slice", "change() + len([]Foreign{foreign}[:])"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `package main
			import "<vector>"
			type Node struct{ value int }
			type Foreign struct{ value std.vector[int] }
			var pointer *Node
			var text string
			var foreign Foreign
			func change() int {
				text = "after"
				return 0
			}
			func read(p *Node, s string, n int) int { return p.value + len(s) + n }
			func main() { _ = read(pointer, text, ` + tt.argument + `) }
			`
			cpp := borrowForeignCPP(t, src)
			main := cpp[strings.Index(cpp, "void main() {"):]
			pointer := strings.Index(main, " = pointer;")
			text := strings.Index(main, " = text;")
			foreign := strings.Index(main, "foreign")
			if pointer < 0 || text <= pointer || foreign <= text {
				t.Fatalf("nested implicit work preceded argument owners:\n%s", main)
			}
			if strings.Contains(tt.argument, "change()") {
				changed := strings.Index(main, "change()")
				if changed < 0 || changed >= pointer || strings.Count(main, "change()") != 1 {
					t.Fatalf("explicit nested/sibling call must run first and exactly once:\n%s", main)
				}
			}
		})
	}
}

func TestBorrowForeignDeferredEmission(t *testing.T) {
	for _, statement := range []string{"defer", "go"} {
		t.Run(statement, func(t *testing.T) {
			src := `package main
			import "<vector>"
			type Foreign struct{ value std.vector[int] }
			var text string
			var foreign Foreign
			func show(s string, v Foreign) { println(s) }
			func main() { ` + statement + ` show(text, foreign) }
			`
			cpp := borrowForeignCPP(t, src)
			main := cpp[strings.Index(cpp, "void main() {"):]
			text := strings.Index(main, " = text;")
			foreign := strings.Index(main, " = foreign;")
			if text < 0 || foreign <= text {
				t.Fatalf("defer/go collection copied a foreign sibling before the string:\n%s", main)
			}
			if strings.Contains(main, "show, text, foreign") {
				t.Fatalf("defer/go collection still binds the source variables:\n%s", main)
			}
		})
	}
}

func TestBorrowForeignSiblingKeepsExplicitCallOrder(t *testing.T) {
	src := `package main
	import "<vector>"
	type Node struct{ val int }
	type Foreign struct{ value std.vector[int] }
	var global *Node
	var text string
	var foreign Foreign
	func change() int {
		text = "after"
		return 1
	}
	func read(p *Node, s string, n int) int { return p.val + len(s) + n }
	func main() { _ = read(global, text, len([]Foreign{foreign}) + change()) }
	`
	cpp := borrowForeignCPP(t, src)
	main := cpp[strings.Index(cpp, "void main() {"):]
	changed := strings.Index(main, "change();")
	text := strings.Index(main, " = text;")
	if changed < 0 || text <= changed {
		t.Fatalf("explicit Go call must still precede the argument's value read:\n%s", main)
	}
}

func TestBorrowSnapshotArgumentChecks(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		first    string
		second   string
		receiver bool
		explicit bool
	}{
		{"map key bounds", "s[i] + m[a[j]]", "a[j]", "s[i]", false, false},
		{"map key nil", "s[i] + m[*index]", "*index", "s[i]", false, false},
		{"map key field nil", "s[i] + m[holder.value]", "holder->value", "s[i]", false, false},
		{"map key division", "s[i] + m[1/divisor]", "gp::div(", "s[i]", false, false},
		{"ordinary bounds", "s[i] + s[j]", "s[i]", "s[j]", false, false},
		{"ordinary nil", "*index + *otherIndex", "*index", "*otherIndex", false, false},
		{"field before bounds", "holder.value + s[i]", "holder->value", "s[i]", false, false},
		{"explicit call once", "s[next()] + m[a[j]]", "a[j]", "s[gp_t", false, true},
		{"receiver checks", "s[i] + m[a[j]]", "a[j]", "s[i]", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call := func(trailing string) string {
				if tt.receiver {
					return "holders[" + tt.value + "].gather(0, " + trailing + ")"
				}
				return "gather(" + tt.value + ", " + trailing + ")"
			}
			src := `package main
			import "<vector>"
			type Foreign struct{ value std.vector[int] }
			type Checked struct{ value int }
			func (p *Checked) gather(value int, count int) int { return p.value + value + count }
			var foreign Foreign
			var s = []int{1}
			var a = [1]int{0}
			var m = map[int]int{0: 1}
			var index *int
			var otherIndex *int
			var holder *Checked
			var holders []*Checked
			func gather(value int, count int) int { return value + count }
			func next() int { return 0 }
			func plain(i, j, divisor int) int { return ` + call("0") + ` }
			func withForeign(i, j, divisor int) int {
				return ` + call("len([]Foreign{foreign})") + `
			}
			func main() {}
			`
			cpp := borrowForeignCPP(t, src)
			for _, name := range []string{"plain", "withForeign"} {
				t.Run(name, func(t *testing.T) {
					signature := "int64_t " + name + "(int64_t i, int64_t j, int64_t divisor) {"
					start := strings.Index(cpp, signature)
					if start < 0 {
						t.Fatalf("missing %s body:\n%s", name, cpp)
					}
					body, _, _ := strings.Cut(cpp[start:], "\n}")
					first := strings.Index(body, tt.first)
					second := strings.Index(body, tt.second)
					if first < 0 || second <= first || !strings.Contains(body[first:second], ";") {
						t.Fatalf("intra-argument checks are not sequenced in Go order:\n%s", body)
					}
					if tt.explicit && strings.Count(body, "next()") != 1 {
						t.Fatalf("explicit call was evaluated more than once:\n%s", body)
					}
				})
			}
		})
	}
}

// TestBorrowHotCrossThreadAndLending has no fmt, mutex, Once or indirect Hook call: only the
// other package's go statement supplies the thread fact. The controls isolate both prerequisites.
func TestBorrowHotCrossThreadAndLending(t *testing.T) {
	for _, mode := range []struct {
		name     string
		threaded bool
		mutates  bool
	}{
		{"cross-package hot replacement", true, true},
		{"no thread source", false, true},
		{"no replacement", true, false},
	} {
		t.Run(mode.name, func(t *testing.T) {
			start := "worker()"
			if mode.threaded {
				start = "go worker()"
			}
			store := ""
			if mode.mutates {
				store = "cfg = &Config{k: 2}"
			}
			program := borrowForeignProgram(t, map[string]string{
				"gocpp.mod": "module example.com/hotguard\n",
				"bg/bg.gp":  "package bg\nfunc worker() {}\nfunc StartWorker() { " + start + " }\n",
				"main.gp": `package main
				import "example.com/hotguard/bg"
				type Config struct{ k int }
				var cfg = &Config{k: 1}
				func sum(c *Config) int { return c.k }
				func replace() { ` + store + ` }
				func main() {
					_ = bg.StartWorker
					_ = sum(cfg)
				}
				`,
			})
			var root *gen
			for _, g := range program.gens {
				if g.pkg.Types.Name() == "main" {
					root = g
				}
			}
			if root == nil {
				t.Fatal("no main package")
			}
			facts := root.threads()
			if facts.threaded != mode.threaded || facts.initThreads {
				t.Fatalf("threaded/initThreads = %v/%v, want %v/false",
					facts.threaded, facts.initThreads, mode.threaded)
			}
			cfg := root.pkg.Types.Scope().Lookup("cfg").(*types.Var)
			hot := mode.threaded && mode.mutates
			if facts.mutGlobals[cfg] != hot {
				t.Fatalf("hot cfg = %v, want %v", facts.mutGlobals[cfg], hot)
			}
			fn := root.pkg.Types.Scope().Lookup("sum").(*types.Func)
			summary := root.borrowOf(fn)
			if summary == nil || !summary.pure || !summary.borrowed(0) {
				t.Fatalf("sum must remain a pure borrowed callee: %+v", summary)
			}

			files, err := program.Lower()
			if err != nil {
				t.Fatal(err)
			}
			var output strings.Builder
			for _, file := range files {
				output.Write(cppir.Print(file.Header, ""))
				for _, unit := range file.Units {
					output.Write(cppir.Print(unit.File, ""))
				}
			}
			cpp := strings.ReplaceAll(output.String(), "::"+root.ns+"::", "")
			cpp = strings.ReplaceAll(cpp, root.ns+"::", "")
			want := "sum(gp::lend(cfg))"
			if hot {
				want = "sum(gp::ptr<Config>(cfg))"
			}
			if !strings.Contains(cpp, want) {
				t.Fatalf("missing %q:\n%s", want, cpp)
			}
		})
	}
}

func TestBorrowQualifiedForeignSnapshots(t *testing.T) {
	for _, form := range []string{"ordinary", "defer", "go"} {
		t.Run(form, func(t *testing.T) {
			statement := "_ = "
			if form != "ordinary" {
				statement = form + " "
			}
			src := `package main
			import (
				"example.com/borrow/lib"
				"<vector>"
			)
			type Foreign struct{ value std.vector[int] }
			var pointer *lib.Node
			var text string
			var foreign Foreign
			func change() int {
				text = "after"
				return 0
			}
			func main() {
				` + statement + `lib.Read(pointer, text, len([]Foreign{foreign}) + change())
			}
			`
			cpp := borrowForeignProgramCPP(t, map[string]string{
				"gocpp.mod": "module example.com/borrow\n",
				"main.gp":   src,
				"lib/lib.gp": `package lib
				type Node struct{ Value int }
				func Read(p *Node, s string, n int) int { return p.Value + len(s) + n }
				`,
			})
			main := cpp[strings.Index(cpp, "void main() {"):]
			changed := strings.Index(main, "change();")
			pointer := strings.Index(main, " = pointer;")
			text := strings.Index(main, " = text;")
			foreign := strings.Index(main, "gp::slice_of<Foreign>")
			if changed < 0 || pointer <= changed || text <= pointer || foreign <= text {
				t.Fatalf("qualified collection lost explicit-call/snapshot order:\n%s", main)
			}
		})
	}
}

func borrowForeignProgram(t *testing.T, sources map[string]string) *Program {
	t.Helper()
	return borrowForeignProgramWith(t, sources, func(*check.Config) {})
}

// borrowForeignProgramWith is borrowForeignProgram for a check that change configures.
func borrowForeignProgramWith(t *testing.T, sources map[string]string, change func(*check.Config)) *Program {
	t.Helper()
	directory := t.TempDir()
	for name, source := range sources {
		path := filepath.Join(directory, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	program := load.Load(load.Config{
		Dir:    directory,
		Stdlib: stdlib.FS(),
		GoStd:  stdlib.IsStd,
		GOOS:   "linux",
		GOARCH: "amd64",
	}, ".")
	config := check.Config{
		Phase: check.LastPhase,
		CXX:   func(context.Context) (build.CXX, error) { return build.CXX{}, nil },
		Probe: func(
			_ context.Context,
			_ build.CXX,
			_ string,
			_, _, candidates []string,
		) (interop.Result, error) {
			kinds := make(map[string]interop.Kind)
			for _, candidate := range candidates {
				kinds[candidate] = interop.Namespace
			}
			return interop.Result{Kinds: kinds}, nil
		},
	}
	change(&config)
	result := check.Check(program, config)
	if program.Errors.HasErrors() {
		t.Fatalf("load errors: %v", program.Errors.Diagnostics())
	}
	var inputs []PackageInput
	for _, loaded := range program.Packages {
		pkg := result.Package(loaded)
		if pkg.Errors.HasErrors() {
			t.Fatalf("check errors: %v", pkg.Errors.Diagnostics())
		}
		if loaded.Std {
			continue
		}
		inputs = append(inputs, PackageInput{
			Pkg: pkg,
			Interop: fakeInterop(pkg, map[string]interop.Kind{
				"std":         interop.Namespace,
				"std::vector": interop.Template,
			}),
		})
	}
	return NewProgram(result.Fset, inputs, Options{})
}

func borrowForeignProgramCPP(t *testing.T, sources map[string]string) string {
	t.Helper()
	files, err := borrowForeignProgram(t, sources).Lower()
	if err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for _, file := range files {
		output.Write(cppir.Print(file.Header, ""))
		for _, unit := range file.Units {
			output.Write(cppir.Print(unit.File, ""))
		}
	}
	return output.String()
}

func borrowForeignCPP(t *testing.T, src string) string {
	t.Helper()
	fset, pkg, _ := checkInterop(t, src)
	kinds := map[string]interop.Kind{
		"std":         interop.Namespace,
		"std::vector": interop.Template,
	}
	file, err := Package(fset, pkg, fakeInterop(pkg, kinds))
	if err != nil {
		t.Fatal(err)
	}
	return string(cppir.Print(file, ""))
}

// TestCNameErrors checks the errors for the names that the probes found nothing for.
func TestCNameErrors(t *testing.T) {
	tests := []struct {
		src   string
		kinds map[string]interop.Kind
		want  string
		user  bool
	}{
		{"var v std.vectr[int]", map[string]interop.Kind{"std::vectr": interop.NotFound},
			"undefined: std.vectr (the imported headers declare no ::std::vectr)", true},
		{"lib.g()", map[string]interop.Kind{"g": interop.NotFound},
			"undefined: lib.g (the imported headers declare no ::g)", true},
		{"_ = std.chrono.clok.now()", map[string]interop.Kind{"std::chrono": interop.Namespace, "std::chrono::clok": interop.NotFound},
			"undefined: std.chrono.clok (the imported headers declare no ::std::chrono::clok)", true},
		{"var x std.out", map[string]interop.Kind{"std::out": interop.Value},
			"std.out is a C/C++ value, not a type", true},
		{"_ = std.vector.iterator(1)", map[string]interop.Kind{"std::vector": interop.Template},
			"std.vector is a C++ template: give it its template arguments before .iterator (std.vector[T].iterator)", true},
		{"lib.g()", map[string]interop.Kind{"g": interop.Unknown},
			"internal error: the compiler probe has no answer for lib.g (::g)", false},
		{"_ = std.vector[int].npos[0]", map[string]interop.Kind{"std::vector": interop.Template},
			"the compiler probe can't tell whether std.vector[int].npos is a C/C++ type or a value: if it is a value, write (std.vector[int].npos)", true},
	}
	for _, tt := range tests {
		src := "package main\n\nimport (\n\t\"<lib.h>\"\n\t\"<vector>\"\n)\n\nfunc main() {\n\t" + tt.src + "\n}\n"
		if strings.HasPrefix(tt.src, "var ") {
			src = strings.Replace(src, "\n}\n", "\n\t_ = x\n}\n", 1)
			src = strings.Replace(src, "var v ", "var x ", 1)
		}
		fset, pkg, _ := checkInterop(t, src)
		_, err := Package(fset, pkg, fakeInterop(pkg, tt.kinds))
		var e *Error
		if !errors.As(err, &e) {
			t.Errorf("%s: err = %v, want an *Error", tt.src, err)
			continue
		}
		if e.Msg != tt.want || e.User != tt.user {
			t.Errorf("%s:\n got %q (user %v)\nwant %q (user %v)", tt.src, e.Msg, e.User, tt.want, tt.user)
		}
	}
}

// TestIncludeSets checks a package whose files import different headers: one ProbeKinds call
// per include set, each file's names with its own set's answers, and the translation unit's
// includes in file order and source order, each header once, mapped to its import.
func TestIncludeSets(t *testing.T) {
	fset, pkg, paths := checkFiles(t, map[string]string{
		"a.gp": "package main\n\nimport (\n\t\"<cstdio>\"\n\t\"./h.h\"\n)\n\nfunc main() { cstdio.puts(\"a\"); h.g(); f() }\n",
		"b.gp": "package main\n\nimport (\n\t\"<vector>\"\n\t\"<cstdio>\"\n)\n\nfunc f() { var v std.vector[int]; _ = v; cstdio.puts(\"b\") }\n",
	})
	sets := includeSets(pkg.Load, pkg.ExternC)
	if len(sets) != 2 || !slices.Equal(sets[0].includes, []string{"<cstdio>", `"./h.h"`}) || !slices.Equal(sets[1].includes, []string{"<vector>", "<cstdio>"}) {
		t.Fatalf("include sets: %+v", sets)
	}
	if got := paths("", sets[1].includes); !slices.Contains(got, "std::vector") || slices.Contains(got, "g") {
		t.Errorf("paths of b.gp's set: %q", got)
	}
	in := &Interop{kinds: map[*load.File]map[string]interop.Kind{
		pkg.Load.Files[0]: {"puts": interop.Value, "g": interop.Value},
		pkg.Load.Files[1]: {"puts": interop.Macro, "std::vector": interop.Template}, // a macro in b's set only
	}}
	f, err := Package(fset, pkg, in)
	if err != nil {
		t.Fatal(err)
	}
	cpp := withDir(string(cppir.Print(f, "")), pkg.Load.Dir)
	for _, want := range []string{
		"#include <gocpp/rt.hpp>\n#line 4 \"$DIR/a.gp\"\n#include <cstdio>\n#include \"./h.h\"\n#line 4 \"$DIR/b.gp\"\n#include <vector>\n#include <gocpp/fmtcxx.hpp>\n\n",
		`::puts("a");`, "::g();", `puts("b");`, "::std::vector<int64_t> v{};",
	} {
		if !strings.Contains(cpp, want) {
			t.Errorf("no %q in the C++:\n%s", want, cpp)
		}
	}
}

// withDir replaces the folder dir in cpp with $DIR.
func withDir(cpp, dir string) string {
	return strings.ReplaceAll(cpp, dir+string(filepath.Separator), "$DIR/")
}

// TestCConstant checks the C literals of untyped constants (§2.5).
func TestCConstant(t *testing.T) {
	g := &gen{fset: token.NewFileSet()}
	tests := []struct {
		v    constant.Value
		t    types.BasicKind
		want string
	}{
		{constant.MakeInt64(5), types.UntypedInt, "5"},
		{constant.MakeInt64(-5), types.UntypedInt, "-5"},
		{constant.MakeInt64(1 << 40), types.UntypedInt, "1099511627776"},
		{constant.MakeInt64(-1 << 63), types.UntypedInt, "(-9223372036854775807LL - 1)"},
		{constant.MakeUint64(1 << 63), types.UntypedInt, "9223372036854775808ULL"},
		{constant.MakeInt64('a'), types.UntypedRune, "'a'"},
		{constant.MakeInt64('\''), types.UntypedRune, `'\''`},
		{constant.MakeInt64('\\'), types.UntypedRune, `'\\'`},
		{constant.MakeInt64('é'), types.UntypedRune, "233"},
		{constant.MakeFloat64(2), types.UntypedFloat, "2.0"},
		{constant.MakeString("a\"b\n"), types.UntypedString, `"a\"b\n"`},
		{constant.MakeBool(true), types.UntypedBool, "true"},
		{constant.MakeInt64(5), types.Int, "int64_t(5)"}, // a typed constant keeps its type
		{constant.MakeString("x"), types.String, `"x"_gs`},
	}
	for _, tt := range tests {
		x := g.cConstant(tt.v, types.Typ[tt.t], token.NoPos)
		if got := cppir.ExprString(x); got != tt.want {
			t.Errorf("cConstant(%s, %s) = %s, want %s", tt.v, types.Typ[tt.t], got, tt.want)
		}
	}
	if g.cConstant(constant.Shift(constant.MakeInt64(1), token.SHL, 64), types.Typ[types.UntypedInt], token.NoPos); g.err == nil || !g.err.User {
		t.Errorf("1<<64 as a C literal: err = %v, want a user error", g.err)
	}
}

// TestCxxNames checks the identifiers that the macro protection looks at.
func TestCxxNames(t *testing.T) {
	src := "#include <x.h>\n#line 3 \"a b.gp\"\n" +
		`auto s = "str ing"_gs; char c = 'q'; int n = 1.5f + 0x1fULL + 1'000; // comment` + "\n" +
		"gp::ptr<Node> p; /* block */ y;\n"
	got := slices.Sorted(func(yield func(string) bool) {
		for n := range cxxNames([]byte(src)) {
			if !yield(n) {
				return
			}
		}
	})
	want := []string{"Node", "auto", "c", "char", "gp", "int", "n", "p", "ptr", "s", "y"}
	if !slices.Equal(got, want) {
		t.Errorf("cxxNames:\n got %q\nwant %q", got, want)
	}
}

// TestCGuard checks the uses of C/C++ values that lowering doesn't translate yet (cguard.go):
// each one is "not supported yet" with its step, never broken C++ or a crash.
func TestCGuard(t *testing.T) {
	kinds := map[string]interop.Kind{
		"f": interop.Value, "g": interop.Value, "EOF": interop.Macro, "FILE": interop.Type, "std::string": interop.Type,
		"std::string::npos": interop.Value, "std::vector": interop.Template, "std::swap": interop.ValueTemplate,
	}
	tests := []struct{ src, want string }{
		{"var h H; switch h { case h: }", "switch on values with C/C++ parts (Phase 12)"},
		{"var h, k H; _ = h == k", "== on values with C/C++ parts (Phase 12)"},
		{"var m map[H]int; _ = m", "this use of a C/C++ type (a later step of Phase 04)"},
		{"var a A; _ = a", "aliases of C/C++ types (A): write the C/C++ type (a later step of Phase 04)"},
		{"m := map[lib.FILE]int{{}: 1}; _ = m", "this type with C/C++ parts: map[C.FILE]int"},
		{"m := []map[lib.FILE]int{{}}; _ = m", "this type with C/C++ parts: map[C.FILE]int"},
		{"p := &struct{ n C.int }{1}; _ = p", "this type with C/C++ parts: struct{n C.int}"},
		{"lib.f(1, lib.g()...)", "... in a call of a C/C++ function"},
		{"var v std.vector[int]; v.push_back(lib.g()...)", "... in a call of a C/C++ function"},
		{"var v std.vector[int]; _ = append([]int{}, v...)", "append of C/C++ values"},
		{"p := lib.f(); p, p.next = lib.f(), nil", "an assignment that changes p and stores into a member or an element of p: write two assignments"},
		{"v := lib.f(); v, v[0] = lib.f(), 1", "an assignment that changes v and stores into a member or an element of v: write two assignments"},
		{"p := lib.f(); p.a, p.a.b = 1, 2", "an assignment that changes p.a and stores into a member or an element of p.a: write two assignments"},
	}
	for _, tt := range tests {
		src := "package main\n\nimport (\n\t\"fmt\"\n\t\"unsafe\"\n\n\t\"<lib.h>\"\n\t\"<vector>\"\n)\n\n" +
			"var _ = fmt.Sprint\n\nvar _ unsafe.Pointer\n\ntype H struct {\n\tp *lib.FILE\n\tq C.int\n\ts string\n}\n\ntype A = C.int\n\n" +
			"func main() {\n\t" + tt.src + "\n}\n"
		fset, pkg, _ := checkInterop(t, src)
		_, err := Package(fset, pkg, fakeInterop(pkg, kinds))
		var e *Error
		if !errors.As(err, &e) || !strings.HasPrefix(e.Msg, "not supported yet: ") || !strings.HasPrefix(e.Msg[len("not supported yet: "):], tt.want) {
			t.Errorf("%s:\n got %v\nwant not supported yet: %s", tt.src, err, tt.want)
		}
	}
}

// TestCUserErrors checks the user errors of C/C++ values at gocpp targets and conversions that Go
// doesn't allow (cguard.go: targetOK, cConvOK), and that the forms next to them lower.
func TestCUserErrors(t *testing.T) {
	kinds := map[string]interop.Kind{"f": interop.Value, "g": interop.Value, "pow": interop.Value, "FILE": interop.Type, "table": interop.Value,
		"EOF": interop.Macro, "MODE_B": interop.Value, "std::string": interop.Type, "std::vector": interop.Template,
		"std::minmax": interop.ValueTemplate, "std::map": interop.Template,
		"std::unordered_map": interop.Template, "std::multimap": interop.Template}
	tests := []struct{ src, want string }{
		{"var iv int = lib.pow(10, 30); _ = iv",
			"cannot use lib.pow(10, 30) (C/C++ value) as int value in variable declaration: convert it, int(lib.pow(10, 30))"},
		{"var iv int; iv = lib.f(); _ = iv",
			"cannot use lib.f() (C/C++ value) as int value in assignment: convert it, int(lib.f())"},
		{"gi(lib.f())", "cannot use lib.f() (C/C++ value) as int value in argument to gi: convert it, int(lib.f())"},
		{"gs(std.string(\"x\"))",
			`cannot use std.string("x") (C/C++ value) as string value in argument to gs: convert it, string(std.string("x"))`},
		{"_ = ri", ""},
		{"var p *lib.FILE; _ = uintptr(p)", "cannot convert p (C pointer) to type uintptr: convert it through unsafe.Pointer, as in uintptr(unsafe.Pointer(p))"},
		{"var p *lib.FILE; _ = int(p)", "cannot convert p (C pointer) to type int: convert it through unsafe.Pointer, as in uintptr(unsafe.Pointer(p))"},
		{"var p *lib.FILE; _ = C.int(p)", "cannot convert p (C pointer) to type C.int: convert it through unsafe.Pointer, as in uintptr(unsafe.Pointer(p))"},
		{"var p *lib.FILE; _ = bool(p)", "cannot convert p (C pointer) to type bool: compare it: p != nil"},
		// unsafe and pointers (step 4.10, cunsafe.go).
		{"var p *lib.FILE; _ = (*byte)(p)", "cannot convert p (C pointer) to type *byte: convert it through unsafe.Pointer, (*byte)(unsafe.Pointer(p))"},
		{"var p *lib.FILE; _ = (*C.char)(p)", "cannot convert p (C pointer) to type *C.char: convert it through unsafe.Pointer, (*C.char)(unsafe.Pointer(p))"},
		{"var p *C.char; _ = p[1:]", "cannot slice p (variable of type *C.char): a C pointer has no length; use unsafe.Slice(p, n)"},
		{"_ = []*lib.FILE{{}}", "an element {...} of a literal of *lib.FILE is &lib.FILE{...} in Go, which gocpp doesn't write for a C/C++ type: write each element as &lib.FILE{...}"},
		// A gocpp pointer stored into a member that no *T variable owns (cunsafe.go: noteStores).
		{"var n lib.FILE; h := &lib.FILE{}; n.next = h; _ = n", "cannot keep the object alive: h is stored into n.next, which no *T variable of the program owns (a C/C++ local value, a pointer from C, or a pointer inside a C struct): declare the struct n := &T{...} and store through n, or write the literal &T{member: ...}"},
		{"n := lib.f(); h := &lib.FILE{}; n.next = h", "cannot keep the object alive: h is stored into n.next, which no *T variable of the program owns (a C/C++ local value, a pointer from C, or a pointer inside a C struct): declare the struct n := &T{...} and store through n, or write the literal &T{member: ...}"},
		{"var pp **lib.FILE; h := &lib.FILE{}; *pp = h", "cannot keep the object alive: h is stored into *pp, which no *T variable of the program owns (a C/C++ local value, a pointer from C, or a pointer inside a C struct): declare the struct n := &T{...} and store through n, or write the literal &T{member: ...}"},
		{"_ = &lib.f()", "invalid operation: cannot take address of lib.f() (C/C++ value): it must be a variable, a field, an element or *p"},
		{"var x int; _ = unsafe.SliceData(lib.f()); _ = x", "invalid argument: lib.f() (C/C++ value) for unsafe.SliceData: it takes a slice, such as a []C.int"},
		{"_ = unsafe.StringData(lib.f())", "invalid argument: lib.f() (C/C++ value) for unsafe.StringData: it takes a string; for a C string use C.GoString(p)"},
		// Messages show what the user wrote, not the helpers of gocpp/cgoh that check added.
		{"var ip *C.int; var s []int = unsafe.Slice(ip, 2); _ = s", "cannot use unsafe.Slice(ip, 2) (C/C++ value) as []int value in variable declaration"},
		{"var y int = lib.table[:]; _ = y", "cannot use lib.table[:] (C/C++ value) as int value in variable declaration: convert it, int(lib.table[:])"},
		{"var y int = lib.table[1:2]; _ = y", "cannot use lib.table[1:2] (C/C++ value) as int value in variable declaration: convert it, int(lib.table[1:2])"},
		{"var y int = lib.table[:2:3]; _ = y", "cannot use lib.table[:2:3] (C/C++ value) as int value in variable declaration: convert it, int(lib.table[:2:3])"},
		// The flows of gocpp pointers to C types lower: a gp::cptr owns or aliases what it points to.
		{"var q *lib.FILE; q = &lib.FILE{}; var p *lib.FILE = &lib.FILE{}; _, _ = p, q", ""},
		{"_ = rn", ""},
		{"var q Q; ps := []*C.int{&q.n}; ps = append(ps, &q.n); _ = ps", ""},
		{"var ps []*lib.FILE; ps = append(ps, &lib.FILE{}); h := hold(&lib.FILE{}); _, _ = ps, h", ""},
		// These lower: the forms next to them.
		{"var v std.vector[int]; p := &v[0]; var q Q; pq := &q.n; _, _ = p, pq", ""},
		{"var p *C.int; xs := unsafe.Slice(p, 2); var cp *C.char; s := unsafe.String((*byte)(unsafe.Pointer(cp)), 3); b := unsafe.Slice((*byte)(unsafe.Pointer(cp)), C.int(2)); _, _, _ = xs, s, b", ""},
		{"var cp *C.char; _ = (*C.char)(nil); u := unsafe.Pointer(cp); _ = (*C.char)(u); _ = uintptr(u); p := lib.f(); n := (*int)(p); _ = n", ""},
		{"xs := []C.int{1, 2}; _ = unsafe.SliceData(xs); lib.f(unsafe.SliceData(xs), 2); lib.f(&xs[0])", ""},
		{"p := &lib.FILE{}; lib.f(p, &lib.FILE{}); u := unsafe.Pointer(&lib.FILE{}); _, _ = p, u", ""},
		{"var a [4]C.int; s := a[1:3]; t := a[:]; _, _ = s, t", ""},
		{"var p *C.char; _ = string(p)", "cannot convert p (C pointer) to type string: use C.GoString(p)"},
		// The cgo helpers and C.cstr (step 4.9, cstrings.go).
		{"f := C.GoString; _ = f", "must call C.GoString"},
		{"_ = C.nosuch", `undefined: C.nosuch (C has only C's built-in types and the helpers C.CString, C.GoString, C.GoStringN, C.GoBytes, C.free, C.cstr, C.wstr, C.WString, C.GoStringW and C.GoStringWN; a C/C++ name comes through its header: import "<header.h>")`},
		{"_ = C.cstr(\"x\")", `C.cstr("x") can be only an argument of a C/C++ function or the value of a new variable (name := C.cstr("x")): its C string lives only as long as that; for a C string that lives longer, use C.CString and C.free`},
		{"var p *C.char = C.cstr(\"x\"); _ = p", `C.cstr("x") can be only an argument of a C/C++ function or the value of a new variable (name := C.cstr("x")): its C string lives only as long as that; for a C string that lives longer, use C.CString and C.free`},
		{"var q Q; q.Put(C.int(lib.f(C.cstr(\"x\")))); _ = lib.g(C.int(C.cstr(\"x\")))", `C.cstr("x") can be only an argument of a C/C++ function or the value of a new variable (name := C.cstr("x")): its C string lives only as long as that; for a C string that lives longer, use C.CString and C.free`},
		{"_ = C.GoString(C.cstr(\"x\"))", `C.cstr("x") can be only an argument of a C/C++ function or the value of a new variable (name := C.cstr("x")): its C string lives only as long as that; for a C string that lives longer, use C.CString and C.free`},
		{"s := \"x\"; n := C.cstr(s); lib.f(n, C.cstr(s+\"!\")); n = C.cstr(\"y\"); var m = C.cstr(s); lib.f(m); var v std.vector[int]; v.push_back(lib.g(C.cstr(s))); _ = std.string(C.cstr(s))", ""},
		{"s := C.GoString(lib.f()); t := C.GoStringN(lib.f(), 3) + s + \"!\"; b := C.GoBytes(lib.f(), 2); p := C.CString(t); C.free(p); defer C.free(lib.g()); _, _ = string(b), C.GoString(nil)", ""},
		{"_ = string(lib.f()); var cs std.string; var gs string = string(cs) + \"!\"; _ = gs", ""},
		{"_ = lib.FILE{a: C.wstr(\"x\")}", "C.wstr(\"x\") (C.wstr holder) cannot be stored in a composite literal: its C string could outlive the holder; use C.WString and C.free for a retained C string"},
		{"_ = C.GoStringW(C.wstr(\"x\"))", `C.wstr("x") can be only an argument of a C/C++ function or the value of a new variable (name := C.wstr("x")): its C string lives only as long as that; for a C string that lives longer, use C.WString and C.free`},
		{"h := C.wstr(\"x\"); h2 := h; _ = lib.FILE{h2}", "h2 (C.wstr holder) cannot be stored in a composite literal: its C string could outlive the holder; use C.WString and C.free for a retained C string"},
		{"var a [lib.NMAX]C.char; _ = a", "a C/C++ macro in an array length (lib.NMAX) needs typed interop (install clang++ 17 or newer, and do not use -interop=opaque); or write the number, such as [260]windows.WCHAR for MAX_PATH"},
		{"_ = lib.FILE{a: C.cstr(\"x\")}", "C.cstr(\"x\") (C.cstr holder) cannot be stored in a composite literal: its C string could outlive the holder; use C.CString and C.free for a retained C string"},
		{"_ = lib.FILE{C.cstr(\"x\")}", "C.cstr(\"x\") (C.cstr holder) cannot be stored in a composite literal: its C string could outlive the holder; use C.CString and C.free for a retained C string"},
		{"h := C.cstr(\"x\"); h2 := h; var h3 = h2; _ = lib.FILE{a: h3}", "h3 (C.cstr holder) cannot be stored in a composite literal: its C string could outlive the holder; use C.CString and C.free for a retained C string"},
		{"h := C.cstr(\"x\"); h2 := h; _ = lib.FILE{h2}", "h2 (C.cstr holder) cannot be stored in a composite literal: its C string could outlive the holder; use C.CString and C.free for a retained C string"},
		// Untyped constants at C targets: Go's rules (cconsts.go).
		// An error does not convert to a C type (ADR-137): no "convert it" hint, C.long(err) is not it.
		{"var e error; var n C.long = e; _ = n", "cannot use e (variable of type error) as C.long value in variable declaration"},
		{"var c C.char = 200; _ = c", "cannot use 200 (untyped int constant) as C.char value in variable declaration (overflows)"},
		{"var c C.uchar; c = -1; _ = c", "cannot use -1 (untyped int constant) as C.uchar value in assignment (overflows)"},
		{"var i C.int = 2.5; _ = i", "cannot use 2.5 (untyped float constant) as C.int value in variable declaration (truncated)"},
		{"var f C.float = 1e300; _ = f", "cannot use 1e300 (untyped float constant 1e+300) as C.float value in variable declaration (overflows)"},
		{"var s *C.char = \"hi\"; _ = s", `cannot use "hi" (untyped string constant) as *C.char value in variable declaration`},
		{"var x C.int = \"x\"; _ = x", `cannot use "x" (untyped string constant) as C.int value in variable declaration`},
		{"var q Q; q.n = 1 << 40", "cannot use 1 << 40 (untyped int constant 1099511627776) as C.int value in assignment (overflows)"},
		{"var q Q; q.Put(1 << 31)", "cannot use 1 << 31 (untyped int constant 2147483648) as C.int value in argument to q.Put (overflows)"},
		{"_ = rk", ""},
		{"_ = C.int(2.5)", "cannot convert 2.5 (untyped float constant) to type C.int"},
		{"_ = C.char(200)", "constant 200 overflows C.char"},
		{"_ = C.ulonglong(-1)", "constant -1 overflows C.ulonglong"},
		{"_ = C.int(true)", "cannot convert true (untyped bool constant) to type C.int"},
		{"_ = C.int(1e10)", "cannot convert 1e10 (untyped float constant 10000000000) to type C.int"},
		{"var i C.int = 1e10; _ = i", "cannot use 1e10 (untyped float constant 1e+10) as C.int value in variable declaration (truncated)"},
		{"var i C.int = 1 + 2i; _ = i", "cannot use 1 + 2i (untyped complex constant (1 + 2i)) as C.int value in variable declaration (truncated)"},
		{"var x C.int; x += 1 << 40", "1 << 40 (untyped int constant 1099511627776) overflows C.int"},
		{"var c C.char; c -= 2.5", "2.5 (untyped float constant) truncated to C.char"},
		{"var c C.char; _ = c + 300", "300 (untyped int constant) overflows C.char"},
		{"var c C.char; _ = c == 300", "300 (untyped int constant) overflows C.char"},
		{"var c C.char; _ = 300 < c", "300 (untyped int constant) overflows C.char"},
		{"var c C.char; _ = c + 1i", "1i (untyped complex constant (0 + 1i)) truncated to C.char"},
		{"var c C.char; _ = c == \"x\"", `invalid operation: c == "x" (mismatched types C.char and untyped string)`},
		{"var c C.char; _ = true == c", "invalid operation: true == c (mismatched types untyped bool and C.char)"},
		{"var q Q; q.n += 1 << 31", "1 << 31 (untyped int constant 2147483648) overflows C.int"},
		{"var p *lib.FILE; _ = p == 0", "invalid operation: p == 0 (mismatched types *lib.FILE and untyped int)"},
		{"var c C.char; var d C.double = 1 + 0i; c += 127 - 1; _ = c + 'a'; _ = c << 300; _ = d * 1e300; _ = c == -128; _ = d", ""},
		{"var c C.char = -128; var u C.uchar = 'x'; var i C.int = 2.0; var d C.double = 1e300; var z C.size_t = 1 << 63; var p *C.char = nil; _, _, _, _, _, _ = c, u, i, d, z, p; _ = C.double(3) + C.double(C.int(-1))", ""},
		// Known C element, dereference, conversion and declared function-result types.
		{"var a [1]C.char; a[0] = 200", "cannot use 200 (untyped int constant) as C.char value in assignment (overflows)"},
		{"var a [1]C.int; a[0] = 2.5", "cannot use 2.5 (untyped float constant) as C.int value in assignment (truncated)"},
		{"var a []C.char; a[0] = 200", "cannot use 200 (untyped int constant) as C.char value in assignment (overflows)"},
		{"a := make([]C.char, 1); a[0] = 200", "cannot use 200 (untyped int constant) as C.char value in assignment (overflows)"},
		{"m := make(map[int]C.char); m[1] = 200", "cannot use 200 (untyped int constant) as C.char value in assignment (overflows)"},
		{"var a [1][1]C.char; a[0][0] = 200", "cannot use 200 (untyped int constant) as C.char value in assignment (overflows)"},
		{"var n C.char; p := &n; *p = 200", "cannot use 200 (untyped int constant) as C.char value in assignment (overflows)"},
		{"var p *C.int; *p = 2.5", "cannot use 2.5 (untyped float constant) as C.int value in assignment (truncated)"},
		{"var a [1]C.char; a[0] += 200", "200 (untyped int constant) overflows C.char"},
		{"var a [1]C.char; _ = a[0] == 300", "300 (untyped int constant) overflows C.char"},
		{"var a [1]C.char; _ = a[0] + 300", "300 (untyped int constant) overflows C.char"},
		{"var n C.char; p := &n; _ = *p + 300", "300 (untyped int constant) overflows C.char"},
		{"m := make(map[int]C.char); m[1] += 300", "300 (untyped int constant) overflows C.char"},
		{"_ = C.char(1) + 300", "300 (untyped int constant) overflows C.char"},
		{"_ = C.char(1) == 300", "300 (untyped int constant) overflows C.char"},
		{"c := C.char(1); _ = c + 300", "300 (untyped int constant) overflows C.char"},
		{"var q Q; _ = q.Size() + (1 << 40)", "1 << 40 (untyped int constant 1099511627776) overflows C.int"},
		{"_ = getchar() == 300", "300 (untyped int constant) overflows C.char"},
		{"c := getchar(); _ = c + 300", "300 (untyped int constant) overflows C.char"},
		// C++ operator results stay unknown: promotions and overloaded operators need not
		// return either operand's type. Declared C operands above still get overflow checks.
		{"var c C.char = 1; var d C.double = 0.5; mixed := c + d; _ = mixed + 0.5", ""},
		{"s := lib.f() + C.char('!'); _ = s == \"word!\"", ""},
		{"var p *C.char = C.CString(\"abcd\"); defer C.free(p); d := p - p; _ = d == 0; _ = (p - p) == 0", ""},
		{"var c C.char = 1; promoted := +c; _ = promoted == 300", ""},
		// Explicit C conversions check ANY constant, not just untyped ones.
		{"const big int64 = 1 << 40; _ = C.int(big)", "constant 1099511627776 overflows C.int"},
		{"const frac float64 = 2.5; _ = C.int(frac)", "cannot convert frac (constant 2.5 of type float64) to type C.int"},
		{"_ = C.char(uint16(200))", "constant 200 overflows C.char"},
		{"const n int64 = 100; const f float64 = 2; const z complex128 = 1 + 0i; _ = C.int(n); _ = C.int(f); _ = C.double(z); _ = lib.f() + 300", ""},
		// gocpp values at C/C++ targets: as cgo, a user error.
		{"fv := 1e30; var i3 C.int = fv; _ = i3",
			"cannot use fv (variable of type float64) as C.int value in variable declaration: convert it, C.int(fv)"},
		{"nv := 1 << 40; var i4 C.int; i4 = nv; _ = i4",
			"cannot use nv (variable of type int) as C.int value in assignment: convert it, C.int(nv)"},
		{"fv := 1e30; var q Q; q.Put(fv)",
			"cannot use fv (variable of type float64) as C.int value in argument to q.Put: convert it, C.int(fv)"},
		{"var q Q; q.n = len(\"ab\")",
			"cannot use len(\"ab\") (constant 2 of type int) as C.int value in assignment: convert it, C.int(len(\"ab\"))"},
		{"_ = rc", ""},
		{"var s string; var p *lib.FILE = s; _ = p",
			"cannot use s (variable of type string) as *lib.FILE value in variable declaration: convert it, (*lib.FILE)(s)"},
		{"var ci C.int; _ = bool(ci)", "cannot convert ci (C/C++ value of type C.int) to type bool"},
		// Operators where gocpp values meet C/C++ values: as cgo, mismatched types.
		{"var x C.int; n := 2; _ = x + n", "invalid operation: x + n (mismatched types C.int and int): convert one of them: int(x) or C.int(n)"},
		{"var x C.int; _ = lib.f() + int(x)", "invalid operation: lib.f() + int(x) (mismatched types C/C++ value and int): convert int(x) to a C type, C.long(int(x)) or C.size_t(int(x)), or a C/C++ number to int, int(lib.f())"},
		{"n := 2; _ = n < lib.f()", "invalid operation: n < lib.f() (mismatched types int and C/C++ value): convert n to a C type, C.long(n) or C.size_t(n), or a C/C++ number to int, int(lib.f())"},
		{"var b B; _ = lib.f() == b", "invalid operation: lib.f() == b (mismatched types C/C++ value and B): convert b to a C type, C.long(b) or C.size_t(b), or a C/C++ number to B, B(lib.f())"},
		{"var v std.vector[int]; n := 2; _ = v.begin() + n", "invalid operation: v.begin() + n (mismatched types C/C++ value and int): convert n to a C type, C.long(n) or C.size_t(n), or a C/C++ number to int, int(v.begin())"},
		{"var p *C.char; n := 2; _ = p + n", "invalid operation: p + n (mismatched types *C.char and int): convert n to a C type, C.long(n) or C.size_t(n)"},
		{"var x C.int; _ = x * int(2)", "invalid operation: x * int(2) (mismatched types C.int and int): convert one of them: int(x) or C.int(int(2))"},
		{"var s std.string; gs := \"x\"; _ = s == gs", "invalid operation: s == gs (mismatched types C/C++ value and string): convert the string: std.string(gs)"},
		{"var x C.int; n := 2; x += n", "invalid operation: x += n (mismatched types C.int and int): convert one of them: int(x) or C.int(n)"},
		{"n := 2; n -= lib.f()", "invalid operation: n -= lib.f() (mismatched types int and C/C++ value): convert n to a C type, C.long(n) or C.size_t(n), or a C/C++ number to int, int(lib.f())"},
		{"n := 2; _ = n << lib.f()", "invalid operation: n << lib.f() (shift count lib.f() is a C/C++ value): convert it, uint(lib.f())"},
		{"n := 2; n <<= lib.f()", "invalid operation: n <<= lib.f() (shift count lib.f() is a C/C++ value): convert it, uint(lib.f())"},
		// Go's other rules that go/types can't check without the C/C++ types.
		{"var v std.vector[int]; v.at(0) = 5", "cannot assign to v.at(0) (neither addressable nor a map index expression)"},
		{"var v std.vector[int]; v.back()++", "cannot assign to v.back() (neither addressable nor a map index expression)"},
		{"var v std.vector[int]; v.back() += 1", "cannot assign to v.back() (neither addressable nor a map index expression)"},
		{"var v std.vector[int]; v[0]", "v[0] (C/C++ value) is not used"},
		{"var v std.vector[int]; _ = len(v)", "invalid argument: v (C/C++ value) for built-in len: call its C++ function, v.size() or std.size(v)"},
		{"var v std.vector[int]; _ = max(v.size(), 1)", "invalid argument: v.size() (C/C++ value) for built-in max: use std.max, or convert it first"},
		{"var v std.vector[int]; _ = v[1, 2]", "invalid operation: v[1, 2] (a C/C++ value takes one index)"},
		{"var a, b int = std.minmax(1, 2); _, _ = a, b",
			"cannot use std.minmax(1, 2) (C/C++ value) as int value in variable declaration: declare new variables for its values (a, b := x), and convert those"},
		{"a := 0; a, b := std.minmax(1, 2); _, _ = a, b",
			"cannot use std.minmax(1, 2) (C/C++ value) as int value in assignment: declare new variables for its values (a, b := x), and convert those"},
		{"_ = rp", ""},
		{"lib.f(&lib.g); p := &lib.g; _ = p", ""},
		// A variable in a constant is the check's error (internal/check constvalues.go); the call
		// of a C/C++ function is known here only.
		{"const k = lib.f()", "lib.f() (C/C++ value) is not constant"},
		{"const k C.int = 1 << 3; const j = C.int(2)", ""},
		{"var xs []C.int; _ = xs[1:]; var arr [3]C.int; for i, v := range arr { _, _ = i, v }; m := make(map[int]C.int); v, ok := m[1]; _, _ = v, ok", ""},
		// The targets of a range over a C/C++ value that assigns.
		{"var v std.vector[int]; i := 0; for i = range v {}; _ = i",
			"cannot assign the index or key of range over v (a C/C++ value) to i of type int: declare a new variable with :=, and convert it, int(x)"},
		{"var v std.vector[int]; var s string; for _, s = range v {}; _ = s",
			"cannot assign the element of range over v (a C/C++ value) to s of type string: declare a new variable with :=, and convert it, string(x)"},
		{"var v std.vector[int]; xs := []int{0}; for _, xs[0] = range v {}",
			"cannot assign the element of range over v (a C/C++ value) to xs[0] of type int: declare a new variable with :=, and convert it, int(x)"},
		{"var v std.vector[C.double]; var n int; for _, (n) = range v {}; _ = n",
			"cannot assign the element of range over v (a C/C++ value) to n of type int: declare a new variable with :=, and convert it, int(x)"},
		{"var v std.vector[int]; var i int; for (i) = range v {}; _ = i",
			"cannot assign the index or key of range over v (a C/C++ value) to i of type int: declare a new variable with :=, and convert it, int(x)"},
		{"var dst, src std.vector[C.int]; for _, dst.at(0) = range src {}",
			"cannot assign to dst.at(0) (neither addressable nor a map index expression)"},
		{"var dst, src std.vector[C.int]; for _, (dst.at(0)) = range src {}",
			"cannot assign to dst.at(0) (neither addressable nor a map index expression)"},
		{"var v std.vector[int]; var i, x C.long; for i, x = range v {}; for _, _ = range v {}; _, _ = i, x", ""},
		{"var cs []C.int; n := 1; cs = append(cs, n)", "cannot use n (variable of type int) as C.int value in argument to append: convert it, C.int(n)"},
		{"var cs []C.int; cs = append(cs, 1, C.int(2)); ds := make([]C.int, 2); copy(ds, cs); m := make(map[C.int]C.double); delete(m, 1); clear(ds); _ = len(cs) + cap(ds) + len(m)", ""},
		{"var m std.map[int, int]; v, ok := m[1]; _, _ = v, ok",
			"assignment mismatch: 2 variables but m[1] is 1 value: a C++ map has no comma-ok form, use m.find(k) != m.end() or m.count(k) == 1"},
		{"p := lib.f(); var a, b = p.value; _, _ = a, b", "assignment mismatch: 2 variables but p.value is 1 value"},
		{"_ = std.vector[int].iterator.x",
			"the compiler probe can't tell whether std.vector[int].iterator is a C/C++ type or a value: if it is a value, write (std.vector[int].iterator).x"},
		// Composite literals (clits.go): gocpp literals with C/C++ parts follow cgo's rules; the
		// literals of C/C++ types follow C++'s, with Go's rules for their form.
		{"n := 1; _ = []C.int{n}", "cannot use n (variable of type int) as C.int value in array or slice literal: convert it, C.int(n)"},
		{"_ = map[string]C.int{\"a\": len(\"ab\")}",
			"cannot use len(\"ab\") (constant 2 of type int) as C.int value in map literal: convert it, C.int(len(\"ab\"))"},
		{"n := 1; _ = Q{n: n}", "cannot use n (variable of type int) as C.int value in struct literal: convert it, C.int(n)"},
		{"_ = [][]C.int{{1}, {int(2)}}", "cannot use int(2) (constant 2 of type int) as C.int value in array or slice literal: convert it, C.int(int(2))"},
		{"_ = []int{lib.f()}", "cannot use lib.f() (C/C++ value) as int value in array or slice literal: convert it, int(lib.f())"},
		{"_ = map[string]B{\"a\": lib.f()}", "cannot use lib.f() (C/C++ value) as B value in map literal: convert it, B(lib.f())"},
		{"_ = C.int{1}", "invalid composite literal type C.int"},
		{"_ = lib.FILE{a: 1, 2}", "mixture of field:value, key:value and value elements in a literal of lib.FILE"},
		{"_ = std.map[int, int]{1: 2, k: 3}", "mixture of field:value, key:value and value elements in a literal of std.map[int, int]"},
		{"_ = lib.FILE{a: 1, b: 2, a: 3}", "duplicate field name a in struct literal"},
		{"_ = std.map[std.string, int]{\"a\": 2, \"b\": 3, \"a\": 4}", "duplicate key \"a\" in map literal"},
		{"_ = std.unordered_map[float64, int]{1: 2, 1.0: 3}", "duplicate key 1.0 in map literal"},
		{"_ = std.vector[std.map[int, int]]{{1: 2}}", "missing type in composite literal: a keyed literal in a literal of a C/C++ type needs its type (T{f: v})"},
		{"_ = std.multimap[int, int]{1: 2, 1: 3}; _ = std.map[int, int]{(1): 2}", ""},
		{"p := &[]C.int{1}; q := &[2]lib.FILE{}; _ = len(*p) + len(q)", ""},
		{"_ = std.vector[lib.FILE]{{}, {a: 1}}", "missing type in composite literal: a keyed literal in a literal of a C/C++ type needs its type (T{f: v})"},
		{"_ = std.vector[int]{1, 2}; _ = std.map[std.string, int]{\"a\": 1, \"b\": lib.f()}; _ = std.map[int, int]{{1, 2}}; n := 3; _ = lib.FILE{n, 2.5, nil}", ""},
		{"_ = lib.FILE{a: lib.f(), b: nil, c: std.vector[lib.FILE]{{1, 2}}}; _ = std.vector[lib.FILE]{lib.FILE{a: 1}}", ""},
		{"_ = []C.int{1, C.int(2)}; _ = [...]C.int{1}; _ = map[string]C.int{\"a\": 1}; _ = Q{n: 5}; _ = Q{lib.f()}; _ = []lib.FILE{{}, {a: 1}}; _ = []*[]C.int{{1}}", ""},
		// The C number types are Go's fixed-size types (cidentity.go, ADR-130).
		{"var n int32; var x C.int = n; var m int32 = x; x += n; _ = x + n; _ = m == x", ""},
		{"var n uint32; var x C.uint = n; var m uint32 = x; _ = x*n; _ = x == m", ""},
		{"var n int64; var x C.longlong = n; var m int64 = x; _ = x - n; _ = m < x; var y C.long = n; _ = y", ""},
		{"var n uint64; var x C.ulonglong = n; var m uint64 = x; _ = x & n; _ = m != x", ""},
		{"var n int16; var u uint16; var i8 int8; var u8 uint8; var a C.short = n; var b C.ushort = u; var c C.char = i8; var d C.schar = i8; var e C.uchar = u8; _, _, _, _, _ = a, b, c, d, e", ""},
		{"var f float32; var d float64; var a C.float = f; var b C.double = d; _ = a * f; _ = b + d", ""},
		{"var b bool; var c C.bool = b; var d bool = c; _ = c == b; _ = d", ""},
		{"var c C.bool = true; var b bool = bool(c); var d C.bool = C.bool(b); _, _ = b, d", ""},
		{"var q Q; var n int32; q.Put(n); var m int32 = q.Size(); _ = m; _ = []C.int{n}", ""},
		{"var c C.int; var n int; _ = c + n", "invalid operation: c + n (mismatched types C.int and int): convert one of them: int(c) or C.int(n)"},
		{"var n uint32; var x C.int = n; _ = x", "cannot use n (variable of type uint32) as C.int value in variable declaration: convert it, C.int(n)"},
		{"var n int32; var x C.uint = n; _ = x", "cannot use n (variable of type int32) as C.uint value in variable declaration: convert it, C.uint(n)"},
		{"var n float64; var x C.float = n; _ = x", "cannot use n (variable of type float64) as C.float value in variable declaration: convert it, C.float(n)"},
		{"var n int; var x C.longlong = n; _ = x", "cannot use n (variable of type int) as C.longlong value in variable declaration: convert it, C.longlong(n)"},
		{"var n B; var x C.bool = n; _ = x", "cannot use n (variable of type B) as C.bool value in variable declaration: convert it, C.bool(n)"},
		{"var x C.int; var m int; m = x; _ = m", "cannot use x (C/C++ value) as int value in assignment: convert it, int(x)"},
		{"var x C.int = 2; var f C.float = 1.5; var b C.bool = true; _, _, _ = x, f, b", ""},
		{"var b C.bool = 1; _ = b", "cannot use 1 (untyped int constant) as C.bool value in variable declaration"},
		// These lower.
		{"var q Q; q.Put(5); pq := &q; pq.Put(6); _ = q.Size; _ = Q.Size; _ = int(q.Size())", ""},
		{"var ci C.int; switch ci { case 3, lib.EOF, lib.MODE_B: }", ""},
		{"var ci C.int = lib.f(); var n int = int(ci); gi(int(lib.f())); _ = n", ""},
		{"fv := 1e30; var a C.int = 5; var b C.double = 2.5; var c C.int = C.int(fv); var q Q; q.Put(7); lib.f(fv, 'x'); _, _, _ = a, b, c", ""},
	}
	for _, tt := range tests {
		src := "package main\n\nimport (\n\t\"unsafe\"\n\n\t\"<lib.h>\"\n\t\"<map>\"\n\t\"<string>\"\n\t\"<vector>\"\n)\n\n" +
			"var _ unsafe.Pointer\n\ntype Q struct{ n C.int }\n\ntype B bool\n\nfunc (q *Q) Put(n C.int) { q.n = n }\n\nfunc (q Q) Size() C.int { return q.n }\n\n" +
			"func hold(p *lib.FILE) *lib.FILE { return p }\n\n" +
			"func gi(n int) {}\n\nfunc gs(s string) {}\n\nfunc getchar() C.char { return 1 }\n\n" +
			"func main() {\n\t" + tt.src + "\n}\n"
		if tt.src == "_ = rc" {
			src += "\nfunc rc() C.int { n := 5; return n }\n"
			tt.want = "cannot use n (variable of type int) as C.int value in return statement: convert it, C.int(n)"
		}
		if tt.src == "_ = rn" {
			src += "\nfunc rn() *lib.FILE { n := &lib.FILE{}; return n }\n"
		}
		if tt.src == "_ = rp" {
			src += "\nfunc rp() **lib.FILE { var p *lib.FILE; return &p }\n"
			tt.want = "&p of a C/C++ local can't be a result: C/C++ values follow C++'s lifetime rules, and p ends with the function"
		}
		if tt.src == "_ = rk" {
			src += "\nfunc rk() C.schar { return 128 }\n"
			tt.want = "cannot use 128 (untyped int constant) as C.schar value in return statement (overflows)"
		}
		if tt.src == "_ = ri" {
			src += "\nfunc ri() int { return lib.f() }\n"
			tt.want = "cannot use lib.f() (C/C++ value) as int value in return statement: convert it, int(lib.f())"
		}
		fset, pkg, _ := checkInterop(t, src)
		_, err := Package(fset, pkg, fakeInterop(pkg, kinds))
		if tt.want == "" {
			if err != nil {
				t.Errorf("%s: %v", tt.src, err)
			}
			continue
		}
		var e *Error
		if !errors.As(err, &e) || e.Msg != tt.want || !e.User {
			t.Errorf("%s:\n got %v\nwant %s (a user error)", tt.src, err, tt.want)
		}
	}
}

// TestIncludeErrorExternC checks the error of a header inside extern "C" { } (step 4.14): the
// user sees the header as imported, not the mark of interop.ExternC, and ProbePaths finds the
// paths of an include set whose includes have the mark.
func TestIncludeErrorExternC(t *testing.T) {
	fset, pkg, paths := checkFiles(t, map[string]string{
		"main.gp": "package main\n\nimport \"./c.h\"\n\nfunc main() { c.f() }\n",
		"c.h":     "int f(void);\n",
	})
	s := includeSets(pkg.Load, map[string]bool{`"./c.h"`: true})[0]
	if !slices.Equal(s.includes, []string{interop.ExternC(`"./c.h"`)}) {
		t.Fatalf("includes %q", s.includes)
	}
	if got := paths("", s.includes); !slices.Contains(got, "f") {
		t.Errorf("paths of the marked include set: %q", got)
	}
	res := interop.Result{IncludeErrors: []interop.IncludeError{{Index: 0, Include: s.includes[0], File: "/p/c.h", Line: 1, Msg: "boom"}}}
	err := includeError(fset, s, res)
	if want := `main.gp:3:8: the header import doesn't work: "./c.h" (inside extern "C"): /p/c.h:1: boom`; err == nil || !strings.HasSuffix(err.(*Error).Pos.String()+": "+err.(*Error).Msg, want) {
		t.Errorf("includeError = %v, want ...%s", err, want)
	}
}

// TestCValues checks the C++ of the operations on C/C++ values (cvalues.go, steps 4.4 and 4.5):
// members, calls, indexes, operators, conditions, op= and ++, &x, several values, and the
// template-vs-index rule.
func TestCValues(t *testing.T) {
	src := `package main

import (
	"<lib.h>"
	"<vector>"
)

func main() {
	var v std.vector[int]
	p := lib.head()
	v.push_back(1)
	_ = p.next.value
	lib.fn()(2)
	_ = v[0] + 1
	_ = lib.table[2]
	_ = (lib.tmpl)[1]
	_ = lib.tmpl[C.int](3)
	_ = std.get[0](v)
	n := 3
	_ = v[n] << n
	var x C.uint = 5
	_ = x &^ 3
	_ = ^x
	_ = -x
	_ = *p
	_ = p == nil
	if v.empty() || p != nil && !lib.ok {
	}
	for it := v.begin(); it != v.end(); it++ {
	}
	x += 2
	x &^= 1
	x--
	lib.out(&x)
	lo, hi := std.minmax(1, 2)
	_, _ = lo, hi
	mm := std.minmax(1, 2)
	_ = mm
	for i, e := range v {
		_, _ = i, e
	}
	for k := range lib.table {
		_ = k
	}
	for range lib.head().items {
	}
	var j, w C.long
	for j, w = range v {
	}
	for _, w = range v {
	}
	_, _ = j, w
}
`
	fset, pkg, _ := checkInterop(t, src)
	kinds := map[string]interop.Kind{
		"head": interop.Value, "fn": interop.Value, "table": interop.Value, "tmpl": interop.ValueTemplate,
		"ok": interop.Value, "out": interop.Value, "std::vector": interop.Template, "std::get": interop.ValueTemplate,
		"std::minmax": interop.ValueTemplate,
	}
	f, err := Package(fset, pkg, fakeInterop(pkg, kinds))
	if err != nil {
		t.Fatal(err)
	}
	cpp := string(cppir.Print(f, ""))
	for _, want := range []string{
		"auto p = [&]<typename gp_R0>(std::type_identity<gp_R0>)",
		"}(std::type_identity<decltype(::head())>());",                      // the type is outside the lambda (clang and captures)
		"return gp::values<gp_R0>(::head()); } else { return (::head()); }", // a pair of references would be copied
		"gp::deref(v).push_back(1);",
		"static_cast<void>(gp::deref(gp::deref(p).next).value);",
		"::fn()(2);",
		"static_cast<void>(v[0] + 1);",
		"static_cast<void>(::table[2]);",
		"static_cast<void>(::tmpl[1]);",      // (x)[i] is an index
		"static_cast<void>(::tmpl<int>(3));", // x[T](...) of a template is its instance
		"static_cast<void>(::std::get<0>(v));",
		"static_cast<void>(v[n] << n);",
		"static_cast<void>(x & ~3);",
		"static_cast<void>(~x);",
		"static_cast<void>(-x);",
		"static_cast<void>(gp::cgo::star(p));",
		"static_cast<void>(gp::cgo::to_bool(p == nullptr));", // a comparison is a gocpp bool
		"if (gp::cgo::cond(gp::deref(v).empty()) || (p != nullptr && !gp::cgo::cond(::ok))) {",
		"for (auto it = gp::deref(v).begin(); it != gp::deref(v).end(); ++it) {",
		"x += 2;",
		"x &= ~1;",
		"--x;",
		"::out(&x);",
		"auto gp_t0 = gp::values(::std::minmax(1, 2));",                 // the references of the pair are copied
		"auto lo = std::get<0>(static_cast<decltype(gp_t0)&&>(gp_t0));", // each member moves out of the tuple
		"auto hi = std::get<1>(static_cast<decltype(gp_t0)&&>(gp_t0));",
		"auto mm = [&]<typename gp_R1>(std::type_identity<gp_R1>)",
		"return gp::values<gp_R1>(::std::minmax(1, 2)); } else { return (::std::minmax(1, 2)); }", // one variable too
		"for (auto&& [i, e] : gp::range(v)) {",
		"for (auto&& k : gp::range_keys(::table)) {",
		"auto&& gp_t1 = ::head();", // the object of items lives as long as the loop
		"for ([[maybe_unused]] auto&& gp_k0 : gp::range_skip(gp::deref(gp_t1).items)) {",
		"for (auto&& [gp_k1, gp_v0] : gp::range(v)) {",
		"j = static_cast<decltype(gp_k1)&&>(gp_k1); w = static_cast<decltype(gp_v0)&&>(gp_v0);",
		"for (auto&& gp_v1 : gp::range_values(v)) {",
		"w = static_cast<decltype(gp_v1)&&>(gp_v1);",
	} {
		if !strings.Contains(cpp, want) {
			t.Errorf("no %q in the C++:\n%s", want, cpp)
		}
	}
	if strings.Contains(cpp, "gp::ptr<unsigned int>") || strings.Contains(cpp, "gp::new_<unsigned int>") {
		t.Errorf("&x of a C variable boxed it:\n%s", cpp)
	}
}

// TestCClosures checks the C++ of the C/C++ objects that closures and deferred calls use (cbox.go,
// step 4.8): the box of each such local, built in place; the copies of the box that the closure
// and the defer frame keep; the moves of a returned local; and what stays unsupported.
func TestCClosures(t *testing.T) {
	kinds := map[string]interop.Kind{
		"f": interop.Value, "g": interop.Value, "FILE": interop.Type, "std::swap": interop.ValueTemplate,
		"std::vector": interop.Template, "std::unique_ptr": interop.Template, "std::make_unique": interop.ValueTemplate, "std::function": interop.Template,
		"Widget": interop.Type, "make": interop.Value,
	}
	prefix := "package main\n\nimport (\n\t\"<lib.h>\"\n\t\"<vector>\"\n\t\"<memory>\"\n\t\"<functional>\"\n)\n\n" +
		"type H struct {\n\tp *lib.FILE\n\tq C.int\n}\n\n"
	lower := func(body string) (string, error) {
		fset, pkg, _ := checkInterop(t, prefix+body)
		f, err := Package(fset, pkg, fakeInterop(pkg, kinds))
		if err != nil {
			return "", err
		}
		return string(cppir.Print(f, "")), nil
	}
	works := []struct {
		src  string
		want []string
	}{
		{"func main() {\n\tv := std.vector[int]{}\n\tadd := func(x int) { v.push_back(C.int(x)) }\n\tadd(1)\n}\n",
			[]string{"auto v = gp::cgo::box([&]() { return ::std::vector<int64_t>{}; });", "[v]<int = 1>(int64_t x) mutable -> void { gp::deref((*v)).push_back("}},
		{"func main() {\n\tvar v std.vector[int]\n\tdefer v.clear()\n}\n",
			[]string{"gp::ptr<::std::vector<int64_t>> v = gp::cgo::new_box<::std::vector<int64_t>>();", "gp_df.gp_push_call([](auto&& gp_a0) { gp::deref((*gp_a0)).clear(); }, v);"}},
		{"func main() {\n\tvar n C.int = lib.f()\n\tfn := func() { _ = n }\n\tfn()\n\tdefer lib.f(&n)\n}\n",
			[]string{"gp::ptr<int> n = gp::cgo::box_as<int>([&]() -> int { return ::f(); });", "[](auto&& gp_a0, auto&&) { ::f(gp_a0); }, gp::cgo::defer_arg(&(*n)), n);"}},
		// every local that a deferred call mentions is boxed and kept by the frame: a reference into it,
		// and the local whose address a pointer holds
		{"func main() {\n\tvar v std.vector[int]\n\tdefer v.back().clear()\n}\n",
			[]string{"gp::ptr<::std::vector<int64_t>> v = gp::cgo::new_box<::std::vector<int64_t>>();", "gp::cgo::keep(", ", v);"}},
		{"func main() {\n\tvar a std.vector[int]\n\tp := &a\n\tdefer lib.f(p)\n}\n",
			[]string{"gp::ptr<::std::vector<int64_t>> a = gp::cgo::new_box<::std::vector<int64_t>>(); gp_df.gp_hold(a);", "{ ::f(gp_a0); }, gp::cgo::defer_arg(p));"}},
		// the address of a part of a local (&a[0], &r.q) holds the local like &a does
		{"func main() {\n\tvar a std.vector[int]\n\tdefer lib.g()\n\tp := &a[0]\n\t_ = p\n}\n",
			[]string{"gp::ptr<::std::vector<int64_t>> a = gp::cgo::new_box<::std::vector<int64_t>>(); gp_df.gp_hold(a);"}},
		// &T{...} of a C type: an owning pointer built in place; a deferred call on it keeps it
		{"func main() {\n\tp := &lib.Widget{}\n\tdefer p.close()\n}\n",
			[]string{"gp::cgo::box_keep<::Widget>([&]() -> ::Widget { return ::Widget{}; })"}},
		// the address of a local of an inner block that only goes to a call, or into a variable of the
		// same block, holds nothing; one stored in a variable of an outer block is held
		{"func main() {\n\tdefer lib.g()\n\tfor i := 0; i < 2; i++ {\n\t\tvar a std.vector[int]\n\t\tlib.f(&a)\n\t\tp := &a\n\t\t_ = p\n\t}\n}\n",
			[]string{"::std::vector<int64_t> a{};", "::f(&a);"}},
		{"func main() {\n\tdefer lib.g()\n\tvar ps []*std.vector[int]\n\tfor i := 0; i < 2; i++ {\n\t\tvar a std.vector[int]\n\t\tps = append(ps, &a)\n\t}\n}\n",
			[]string{"gp::ptr<::std::vector<int64_t>> a = gp::cgo::new_box<::std::vector<int64_t>>(); gp_df.gp_hold(a);"}},
		// a closure that captures a pointer which holds the address of a loop local, stored outside
		{"func main() {\n\tdefer lib.g()\n\tvar fs []func()\n\tfor i := 0; i < 2; i++ {\n\t\tvar a std.vector[int]\n\t\tp := &a\n\t\tfs = append(fs, func() { _ = p })\n\t}\n}\n",
			[]string{"gp::ptr<::std::vector<int64_t>> a = gp::cgo::new_box<::std::vector<int64_t>>(); gp_df.gp_hold(a);"}},
		// a path from a gocpp pointer: the frame holds a copy of the pointer
		{"type S struct{ v std.vector[int] }\n\nfunc main() {\n\ts := &S{}\n\tdefer s.v.clear()\n}\n",
			[]string{"gp::deref(gp_a0->v).clear();", "}, gp::cgo::nonnil(s));"}},
		// a struct with a C++ field, an array of C++ objects, and a raw pointer are boxed too
		{"func main() {\n\tvar h H\n\tp := lib.f()\n\tf := func() { _ = h.p; _ = p }\n\tf()\n}\n",
			[]string{"gp::ptr<H> h = gp::cgo::new_box<H>();", "auto p = gp::cgo::box("}},
		// the loop variable of a 3-clause for gets a copy of its object for each iteration
		{"func main() {\n\tfor i := lib.f(); i != nil; i = lib.g() {\n\t\tfn := func() { _ = i }\n\t\tfn()\n\t}\n}\n",
			[]string{"gp::cgo::renew(i);"}},
		// a func value result that the closure doesn't touch stays as it was
		{"func main() {\n\tv := std.vector[int]{}\n\tw := std.vector[int]{}\n\tfn := func() { _ = v }\n\tfn()\n\t_ = w\n}\n",
			[]string{"auto v = gp::cgo::box(", "auto w = ::std::vector<int64_t>{};"}},
		// results: a slot for a C++ result that has no name in a function with defers; a moved local
		{"func mk() (lib.Widget, int) {\n\tdefer func() {}()\n\tw := lib.make()\n\treturn w, 1\n}\n\nfunc main() { mk() }\n",
			[]string{"gp::result_slot<::Widget> gp_r0;", "gp_r0 = static_cast<decltype(w)&&>(w);", "return {gp_r0.gp_take(), gp_r1};"}},
		{"func two() (std.unique_ptr[C.int], std.unique_ptr[C.int]) {\n\ta, b := std.make_unique[C.int](1), std.make_unique[C.int](2)\n\treturn a, b\n}\n\nfunc main() { two() }\n",
			[]string{"return {static_cast<decltype(a)&&>(a), static_cast<decltype(b)&&>(b)};"}},
		{"func two() (std.unique_ptr[C.int], std.unique_ptr[C.int]) {\n\ta := std.make_unique[C.int](1)\n\tfn := func() { _ = a }\n\tfn()\n\treturn a, nil\n}\n\nfunc main() { two() }\n",
			[]string{"gp::cgo::result_of(*a)"}},
		// callbacks (step 4.11, ccallback.go): a function of the package, a func literal without
		// captures, a closure, a func value; stored into a C/C++ member; a func with C parts as a
		// std::function's argument
		{"func cb(x C.int) C.int { return x }\n\nfunc main() {\n\tlib.f(cb)\n}\n",
			[]string{"::f(gp::cgo::fn<&gpkg_main::cb>());"}},
		{"func main() {\n\tlib.f(func(x C.int) C.int { return x })\n}\n",
			[]string{"::f(gp::cgo::fn_lit<gp::func<int(int)>>([]<int = 1>(int x) mutable -> int { return x; }));"}},
		{"func main() {\n\tn := 1\n\tlib.f(func(x C.int) C.int { return x + C.int(n) })\n}\n",
			[]string{"::f(gp::cgo::fn_val(gp::func<int(int)>([n]<int = 1>(int x) mutable -> int {"}},
		{"func main() {\n\tg := func(x C.int) {}\n\tvar h H\n\th.p = g\n\tlib.f(g)\n}\n",
			[]string{"h.p = gp::cgo::fn_val(g);", "::f(gp::cgo::fn_val(g));"}},
		{"func main() {\n\tvar f std.function[func(C.int) C.int]\n\t_ = f\n}\n",
			[]string{"::std::function<int(int)> f"}},
		{"func main() {\n\tvar f std.vector[func(C.int) C.int]\n\t_ = f\n}\n",
			[]string{"::std::vector<gp::func<int(int)>> f"}},
	}
	for _, tt := range works {
		cpp, err := lower(tt.src)
		if err != nil {
			t.Errorf("%s:\n%v", tt.src, err)
			continue
		}
		for _, want := range tt.want {
			if !strings.Contains(cpp, want) {
				t.Errorf("%s: no %q in the C++:\n%s", tt.src, want, cpp)
			}
		}
	}
	fails := []struct{ src, want string }{
		{"func main() {\n\tv := std.vector[int]{}\n\tdefer v[0].clear()\n}\n",
			"cannot defer a call whose object is v[0]: Go evaluates it when the defer statement runs, and a deferred C++ call can only keep a variable's object or a temporary: use a func literal, defer func() { ... }()"},
		{"func main() {\n\tfor i := C.int(0); i < 3; i++ {\n\t\tlib.f(&i)\n\t}\n}\n",
			"& of the variable of a 3-clause for loop (i): Go gives each iteration its own variable, and a C/C++ address can't keep it alive: copy it into a variable of the body, j := i, and take &j"},
	}
	for _, tt := range fails {
		_, err := lower(tt.src)
		var e *Error
		if !errors.As(err, &e) || !strings.HasSuffix(e.Msg, tt.want) {
			t.Errorf("%s:\n got %v\nwant %s", tt.src, err, tt.want)
		}
	}
}

// TestCUnsafe checks the C++ of unsafe and pointers at the C/C++ boundary (step 4.10, unsafe.go,
// cunsafe.go): the conversions with unsafe.Pointer, unsafe.Slice and String of C pointers (check
// wraps the calls that have an opaque pointer), &T{...} of a C type, &s.f and &v[i], a slice of a
// C array, and a gocpp pointer through a C function's `...`.
func TestCUnsafe(t *testing.T) {
	src := `package main

import (
	"unsafe"

	"<lib.h>"
	"<vector>"
)

type Rec struct {
	n    C.int
	vals [4]C.int
}

type Wrap struct{ c lib.FILE }

func main() {
	var cp *C.char
	var ip *C.int
	var n C.int = 3
	var v std.vector[int]
	var r Rec
	var a [4]C.int
	x := 7
	px := &x
	up := unsafe.Pointer(cp)
	_ = (*C.char)(up)
	_ = (*byte)(up)
	_ = (*[8]C.int)(up)
	_ = uintptr(up)
	_ = unsafe.Pointer(&n)
	_ = unsafe.Pointer(lib.fn)
	xs := unsafe.Slice(ip, 4)
	_ = len(xs)
	s := unsafe.String((*byte)(up), n)
	_ = s + "!"
	bs := unsafe.Slice((*byte)(up), n)
	_ = len(bs)
	_ = unsafe.Add(up, n)
	q := &lib.FILE{}
	lib.f(q, &lib.FILE{})
	_ = &v[1]
	_ = &r.n
	_ = &r.vals[2]
	_ = &a[1]
	var lr lib.rec
	sl := lr.vals[1:3]
	_ = len(sl)
	_ = lr.vals[:]
	lib.f(px, up, unsafe.Pointer(px), unsafe.SliceData(xs))
	_ = unsafe.Sizeof(n)
	_ = unsafe.Alignof(lr)
	_ = unsafe.Offsetof(lr.q)
	var out unsafe.Pointer
	lib.f(&out)
	oq := &lib.FILE{}
	for ; oq != nil; oq = oq.next {
	}
	var raw *lib.FILE = nil
	raw = oq
	_ = raw
	var db *lib.FILE
	lib.f(&db)
	var head *lib.FILE
	for i := 0; i < 2; i++ {
		head = &lib.FILE{next: head}
	}
	var table []*lib.FILE
	table = append(table, head)
	head.next = table[0]
	head.a.b = nil
	nodes := make([]lib.FILE, 2)
	nodes[0].next = head
	wr := &Wrap{}
	wr.c.next = head
	nxt := head.next
	head.next = nxt.next
	oth := nxt
	head.next, oth.next = oth.next, head.next
	cpy := *head
	_ = cpy
	switch head {
	case nil:
	}
}
`
	fset, pkg, _ := checkInterop(t, src)
	kinds := map[string]interop.Kind{"fn": interop.Value, "f": interop.Value, "FILE": interop.Type, "rec": interop.Type, "std::vector": interop.Template}
	f, err := Package(fset, pkg, fakeInterop(pkg, kinds))
	if err != nil {
		t.Fatal(err)
	}
	cpp := string(cppir.Print(f, ""))
	for _, want := range []string{
		"gp::ptr<void> up = gp::cgo::as<gp::ptr<void>>(cp);",
		"gp::cgo::as<gp::cptr<char>>(up)", // (*C.char)(p): a gocpp pointer sharing the owner of p
		"gp::unsafe_cast<uint8_t>(up)",
		"gp::cgo::as<gp::ptr<gp::array<int, 8>>>(up)",
		"gp::unsafe_addr(up)",
		"gp::cgo::as<gp::ptr<void>>(&n)",
		"gp::cgo::as<gp::ptr<void>>(::fn)",
		"auto xs = gp::unsafe_slice(ip, 4);", // not wrapped in gp::values: it is no call of C++
		"gp::string s = gp::unsafe_string(gp::unsafe_cast<uint8_t>(up), gp::cgo::as<int64_t>(n));",
		"auto q = gp::cgo::box_keep<::FILE>([&]() -> ::FILE { return ::FILE{}; });",
		"::f(gp::cgo::raw(q), gp::cgo::raw(gp::cgo::box_keep<::FILE>([&]() -> ::FILE { return ::FILE{}; })));",
		"static_cast<void>(&v[1]);",
		"static_cast<void>(gp::cptr<int>(gp::ptr<int>(r, &(*r).n)));", // r is a gocpp struct, in its box: the pointer shares its owner
		"static_cast<void>(gp::cptr<int>(gp::ptr<int>(r, &(*r).vals[2])));",
		"static_cast<void>(gp::cptr<int>(gp::ptr<int>(a, &(*a)[1])));",
		"auto sl = gp::sub(gp::cgo::slice_of(gp::deref(lr).vals), 1, 3);",
		"gp::cgo::slice_of(gp::deref(lr).vals)",
		"::f(px.gp_get(), up.gp_get(), gp::unsafe_pointer(px).gp_get(), gp::unsafe_slice_data(xs).gp_get());", // through ...: raw addresses
		"static_cast<void>(gp::unsafe_sizeof<decltype(n)>());",
		"static_cast<void>(gp::unsafe_alignof<decltype(lr)>());",
		"static_cast<void>(offsetof(std::remove_cvref_t<decltype(gp::deref(lr))>, q));",
		"::f(gp::cgo::out_ptr(out));",
		"for (; oq != nullptr; oq = gp::deref(oq).next) {", // the walk over raw pointers from C
		"raw = oq;",
		"gp::cptr<::FILE> db{};",     // a gocpp pointer to a C type
		"::f(gp::cgo::out_ptr(db));", // &db for a T** parameter
		"head = gp::cgo::box_keep<::FILE>([&](const auto& gp_k0) -> ::FILE { return [&]() { ::FILE gp_t0{}; static_assert(gp::cgo::field_ok<decltype(gp_t0.next)>); gp_t0.next = {gp_k0}; return gp_t0; }(); }, head);", // the node keeps head's object
		"auto gp_t1 = table[0]; gp::cgo::store_keep(head, gp::deref(head).next, gp_t1);",                                         // a store into a member of what head owns
		"gp::cgo::store_keep(head, gp::deref(gp::deref(head).a).b, nullptr, gp::cgo::keep_path<decltype(gp::deref(head).a)>());", // through a member struct // the node keeps head's object
		"gp::slice<gp::cptr<::FILE>> table{};",
		"gp::cgo::case_eq(head, nullptr)",
	} {
		if !strings.Contains(cpp, want) {
			t.Errorf("no %q in the C++:\n%s", want, cpp)
		}
	}
}

// TestCStringLowering checks holders moved by evaluation ordering: the temporary owns the
// holder, never its pointer, and the call gets .gp_ptr() at its use (also for holder copies).
// CString results compare like C pointers, with comparisons rewritten to a gocpp bool.
func TestCStringLowering(t *testing.T) {
	src := `package main

import "<cstdio>"

func mk(s string) string { return s + "!" }
func f() C.int { return 7 }
func main() {
	s := "gocpp"
	cstdio.printf("%s %d %s", C.cstr(mk(s)), f(), C.cstr(mk("x") + s))
	h := C.cstr(s)
	h2 := h
	var h3 = h2
	cstdio.printf("%s %s", h2, h3)
	x := C.CString(s)
	_ = x == nil
	_ = x == x
	_ = C.CString(s) != nil
	C.free(x)
}
`
	fset, pkg, _ := checkInterop(t, src)
	f, err := Package(fset, pkg, fakeInterop(pkg, map[string]interop.Kind{"printf": interop.Value}))
	if err != nil {
		t.Fatal(err)
	}
	cpp := string(cppir.Print(f, ""))
	holder := regexp.MustCompile(`auto (gp_t[0-9]+) = gp::cstr\([^;]+;`).FindStringSubmatch(cpp)
	if len(holder) != 2 || strings.Contains(holder[0], ".gp_ptr()") {
		t.Fatalf("no owning holder temporary in the C++:\n%s", cpp)
	}
	for _, want := range []string{
		`::printf("%s %d %s", ` + holder[1] + ".gp_ptr(),",
		`::printf("%s %s", h2.gp_ptr(), h3.gp_ptr());`,
		"gp::cgo::to_bool(x == nullptr)",
		"gp::cgo::to_bool(x == x)",
		"gp::cgo::to_bool(gp::cgo::CString(s) != nullptr)",
	} {
		if !strings.Contains(cpp, want) {
			t.Errorf("no %q in the C++:\n%s", want, cpp)
		}
	}
}

// TestCPrint checks how arguments with C/C++ parts reach fmt and print (fmtcall.go, step
// 4.12): through gp::cgo::fmt_arg and gp::cgo::print_arg, and gocpp/fmtcxx.hpp after the
// C/C++ includes.
func TestCPrint(t *testing.T) {
	kinds := map[string]interop.Kind{"f": interop.Value, "EOF": interop.Macro, "FILE": interop.Type}
	src := "package main\n\nimport (\n\t\"fmt\"\n\n\t\"<lib.h>\"\n)\n\n" +
		"type H struct{ q C.int }\n\nfunc two() (C.int, int) { return 1, 2 }\n\n" +
		"func main() {\n\tvar h H\n\tvar p *lib.FILE\n" +
		"\tfmt.Println(lib.EOF, 1, h, p)\n\tfmt.Printf(\"%d\\n\", lib.f())\n\t_ = fmt.Sprint(two())\n" +
		"\tprintln(lib.f(), 2)\n\tprintln(two())\n}\n"
	fset, pkg, _ := checkInterop(t, src)
	f, err := Package(fset, pkg, fakeInterop(pkg, kinds))
	if err != nil {
		t.Fatal(err)
	}
	cpp := string(cppir.Print(f, ""))
	for _, want := range []string{
		"#include <lib.h>\n#include <gocpp/fmtcxx.hpp>\n",
		"gp::fmt::Println(gp::cgo::fmt_arg(EOF), int64_t(1), gp::cgo::fmt_arg(gp::fmt::as(&gp_td_H, h)), gp::cgo::fmt_arg(p));",
		`gp::fmt::Printf("%d\n"_gs, gp::cgo::fmt_arg(::f()));`,
		"gp::fmt::Sprint(gp::cgo::fmt_arg(gp_r0), gp_r1)",
		"gp::println(gp::cgo::print_arg(::f()), int64_t(2));",
		"gp::println(gp::cgo::print_arg(gp_a)...)",
	} {
		if !strings.Contains(cpp, want) {
			t.Errorf("the C++ has no %q:\n%s", want, cpp)
		}
	}
}

// TestCLiterals checks the C++ of composite literals with C/C++ types (clits.go, step 4.6):
// positional and keyed literals of C/C++ types, a C++ map's pairs, inner literals without their
// types, tag names, and gocpp literals with C/C++ parts.
func TestCLiterals(t *testing.T) {
	src := `package main

import (
	"<lib.h>"
	"<map>"
	"<vector>"
)

type P struct {
	d lib.Vec
	n C.int
}

func f() C.int { return 1 }

func main() {
	_ = lib.Vec{1, f()}
	_ = lib.Vec{y: 2, x: f()}
	_ = lib.Box{max: {1, 2}, min: lib.Vec{}}
	_ = std.vector[lib.Vec]{{1, 2}, {}}
	_ = std.map[int, int]{1: 2, 3: f()}
	_ = lib.struct_pt{1, 2}
	_ = lib.struct_pt{x: 1}
	_ = []C.int{1, f()}
	_ = [...]C.double{1.5, 2}
	_ = []lib.Vec{{1, 2}, {y: 3}}
	_ = map[string]C.int{"a": 1}
	_ = P{d: lib.Vec{1, 2}, n: 3}
}
`
	fset, pkg, _ := checkInterop(t, src)
	kinds := map[string]interop.Kind{"Vec": interop.Type, "Box": interop.Type, "std::vector": interop.Template,
		"std::map": interop.Template}
	f, err := Package(fset, pkg, fakeInterop(pkg, kinds))
	if err != nil {
		t.Fatal(err)
	}
	cpp := string(cppir.Print(f, ""))
	for _, want := range []string{
		"static_cast<void>(::Vec{1, f()});",
		"static_cast<void>([&]() { ::Vec gp_t0{}; static_assert(gp::cgo::field_ok<decltype(gp_t0.y)>); gp_t0.y = {2}; " +
			"static_assert(gp::cgo::field_ok<decltype(gp_t0.x)>); gp_t0.x = {f()}; return gp_t0; }());",
		"static_cast<void>([&]() { ::Box gp_t1{}; static_assert(gp::cgo::field_ok<decltype(gp_t1.max)>); gp_t1.max = {1, 2}; " +
			"static_assert(gp::cgo::field_ok<decltype(gp_t1.min)>); gp_t1.min = ::Vec{}; return gp_t1; }());",
		"static_cast<void>(::std::vector<::Vec>{{1, 2}, {}});",
		"static_cast<void>(gp::cgo::map_t<::std::map<int64_t, int64_t>>{{1, 2}, {3, f()}});",
		"static_cast<void>(std::type_identity_t<struct ::pt>{1, 2});",
		"static_cast<void>([&]() { struct ::pt gp_t2{}; static_assert(gp::cgo::field_ok<decltype(gp_t2.x)>); gp_t2.x = {1}; return gp_t2; }());",
		"static_cast<void>(gp::slice_of<int>({1, f()}));",
		"static_cast<void>(gp::array<double, 2>{1.5, 2});",
		"static_cast<void>(gp::slice_of<::Vec>({::Vec{1, 2}, [&]() { ::Vec gp_t3{}; static_assert(gp::cgo::field_ok<decltype(gp_t3.y)>); gp_t3.y = {3}; return gp_t3; }()}));",
		"static_cast<void>(gp::map_of<gp::string, int>({{\"a\"_gs, 1}}));",
		"static_cast<void>(P{.d = ::Vec{1, 2}, .n = 3});",
	} {
		if !strings.Contains(cpp, want) {
			t.Errorf("no %q in the C++:\n%s", want, cpp)
		}
	}
}

// TestRootIdentOf checks the helper that addrHeld reads the operand of & with (cbox.go): the
// variable that &a[1], &h.c, &x[i].f and &(*p).f start at, which step 4.10's wider cAddr gives it.
func TestRootIdentOf(t *testing.T) {
	tests := []struct{ src, want string }{
		{"a", "a"}, {"(a)", "a"}, {"a[1]", "a"}, {"h.c", "h"}, {"x[i].f", "x"}, {"(*p).f", "p"},
		{"a.b.c[2].d", "a"}, {"*p", "p"}, {"f().x", ""}, {"f()[0]", ""}, {"T{}.x", ""},
	}
	for _, tt := range tests {
		e, err := parser.ParseExpr(tt.src)
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		if id := rootIdentOf(e); id != nil {
			got = id.Name
		}
		if got != tt.want {
			t.Errorf("rootIdentOf(%s) = %q, want %q", tt.src, got, tt.want)
		}
	}
}

// TestCConstTypeLLP64 checks that the ranges of C.long, C.ulong and C.wchar_t follow the target (step 14.4):
// Windows is LLP64, where long has 32 bits and wchar_t is an unsigned 16-bit unit.
func TestCConstTypeLLP64(t *testing.T) {
	for _, tt := range []struct {
		goos, name string
		want       types.BasicKind
	}{
		{"linux", "long", types.Int64}, {"linux", "ulong", types.Uint64}, {"linux", "wchar_t", types.Int32},
		{"windows", "long", types.Int32}, {"windows", "ulong", types.Uint32}, {"windows", "wchar_t", types.Uint16},
		{"windows", "int", types.Int32}, {"windows", "longlong", types.Int64}, {"windows", "size_t", types.Uint64},
	} {
		g := &gen{pkg: &check.Package{Load: &load.Package{GOOS: tt.goos}}}
		if got, ok := g.cConstType(tt.name); !ok || got != tt.want {
			t.Errorf("%s: C.%s has range %v (ok %v), want %v", tt.goos, tt.name, got, ok, tt.want)
		}
	}
}

// TestCAddrOwnership checks which C/C++ locals are boxed for the address that is stored in a
// package variable (cbox.go: addrEscapes, ADR-139): the address of the variable, of a member, of a
// member of a member, of an element of a C array, of a member of a gocpp struct, and through a
// local alias of package data; and which stay on the stack with a raw C++ address (an address that
// only goes to a call, to a local pointer, or to a type switch).
// A variable whose type gocpp does not know (a C/C++ type it cannot read) may be a pointer, so it
// counts as a reference variable for the ownership analysis; a known scalar or struct does not.
func TestRefType(t *testing.T) {
	for name, tc := range map[string]struct {
		typ  types.Type
		want bool
	}{
		"pointer":  {types.NewPointer(types.Typ[types.Int]), true},
		"slice":    {types.NewSlice(types.Typ[types.Int]), true},
		"map":      {types.NewMap(types.Typ[types.String], types.Typ[types.Int]), true},
		"unknown":  {types.Typ[types.Invalid], true},
		"no type":  {nil, true},
		"int":      {types.Typ[types.Int], false},
		"string":   {types.Typ[types.String], false},
		"struct":   {types.NewStruct(nil, nil), false},
		"channel":  {types.NewChan(types.SendRecv, types.Typ[types.Int]), false},
		"function": {types.NewSignatureType(nil, nil, nil, nil, nil, false), false},
	} {
		if got := refType(tc.typ); got != tc.want {
			t.Errorf("refType(%s) = %v, want %v", name, got, tc.want)
		}
	}
}

// One big class: a cursor reused over many locals, and a chain of aliases. The analysis runs once per
// class, not once per variable of it (it took minutes at 5,000 variables when it ran once per
// variable), so this takes a fraction of a second; the bound is generous.
func TestCAddrBigClass(t *testing.T) {
	const n = 2000
	var sb strings.Builder
	sb.WriteString("package main\n\nimport \"<lib.h>\"\n\nvar g *lib.rec\n\nfunc use(p *lib.rec) { g = p }\n\n")
	sb.WriteString("func cursor() {\n\tvar cur *lib.rec\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "\tvar x%d lib.rec\n\tcur = &x%d\n\tg = cur\n", i, i)
	}
	sb.WriteString("}\n\nfunc chain() {\n\tvar q *lib.rec\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "\tvar y%d lib.rec\n\tp%d := &y%d\n\tq = p%d\n\tg = q\n", i, i, i, i)
	}
	sb.WriteString("}\n\nfunc main() {}\n")
	fset, pkg, _ := checkInterop(t, sb.String())
	start := time.Now()
	f, err := Package(fset, pkg, fakeInterop(pkg, map[string]interop.Kind{"rec": interop.Type}))
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 8*raceSlowdown*time.Second {
		t.Errorf("lowering %d locals in one class took %v", n, took)
	}
	cpp := string(cppir.Print(f, ""))
	for _, v := range []string{"x0", "x1999", "y0", "y1999"} {
		if !strings.Contains(cpp, "gp::ptr<::rec> "+v+" = ") {
			t.Errorf("%s is not boxed", v)
		}
	}
}

// mayHoldPointers is true for every type that can carry an address: a value of any other type
// that is computed from &x does not keep x (ADR-139). A type that gocpp does not know counts as
// one that can.
func TestMayHoldPointers(t *testing.T) {
	ptr := types.NewPointer(types.Typ[types.Int])
	named := types.NewNamed(types.NewTypeName(token.NoPos, nil, "Named", nil), types.NewStruct(nil, nil), nil)
	namedPtr := types.NewNamed(types.NewTypeName(token.NoPos, nil, "NamedPtr", nil), ptr, nil)
	withPtr := types.NewStruct([]*types.Var{
		types.NewField(token.NoPos, nil, "n", types.Typ[types.Int], false),
		types.NewField(token.NoPos, nil, "p", ptr, false),
	}, nil)
	ints := types.NewStruct([]*types.Var{
		types.NewField(token.NoPos, nil, "n", types.Typ[types.Int], false),
		types.NewField(token.NoPos, nil, "s", types.Typ[types.String], false),
	}, nil)
	tparam := types.NewTypeParam(types.NewTypeName(token.NoPos, nil, "T", nil), types.NewInterfaceType(nil, nil))
	for name, tc := range map[string]struct {
		typ  types.Type
		want bool
	}{
		"int":                 {types.Typ[types.Int], false},
		"int8":                {types.Typ[types.Int8], false},
		"uint64":              {types.Typ[types.Uint64], false},
		"uintptr":             {types.Typ[types.Uintptr], false},
		"float64":             {types.Typ[types.Float64], false},
		"complex128":          {types.Typ[types.Complex128], false},
		"string":              {types.Typ[types.String], false},
		"bool":                {types.Typ[types.Bool], false},
		"unsafe.Pointer":      {types.Typ[types.UnsafePointer], true},
		"unknown":             {types.Typ[types.Invalid], true},
		"no type":             {nil, true},
		"pointer":             {ptr, true},
		"slice of int":        {types.NewSlice(types.Typ[types.Int]), true},
		"map":                 {types.NewMap(types.Typ[types.String], types.Typ[types.Int]), true},
		"channel":             {types.NewChan(types.SendRecv, types.Typ[types.Int]), true},
		"func":                {types.NewSignatureType(nil, nil, nil, nil, nil, false), true},
		"interface":           {types.NewInterfaceType(nil, nil), true},
		"error":               {types.Universe.Lookup("error").Type(), true},
		"array of int":        {types.NewArray(types.Typ[types.Int], 4), false},
		"array of pointer":    {types.NewArray(ptr, 4), true},
		"struct of numbers":   {ints, false},
		"struct with pointer": {withPtr, true},
		"empty named struct":  {named, false},
		"named pointer":       {namedPtr, true},
		"type parameter":      {tparam, true},
		"tuple":               {types.NewTuple(types.NewVar(token.NoPos, nil, "a", types.Typ[types.Int])), true},
	} {
		if got := mayHoldPointers(tc.typ, 0); got != tc.want {
			t.Errorf("mayHoldPointers(%s) = %v, want %v", name, got, tc.want)
		}
	}
}

// A function with hundreds of range-over-func loops and of function literals, each with a C local whose
// address is stored: the analysis of the function is made once and shared by the lambdas inside it
// (it was made again for each of them: quadratic, 8 to 15 times slower than without the rule at 1,000
// loops). The output is the same as without the sharing.
func manyBodiesSource(loops, lits int) string {
	var sb strings.Builder
	sb.WriteString("package main\n\nimport \"<lib.h>\"\n\nvar g *lib.rec\n\nfunc use(p *lib.rec) { g = p }\n\nfunc cleanup() {}\n\n")
	sb.WriteString("func seq(n int) func(yield func(int) bool) {\n\treturn func(yield func(int) bool) {\n\t\tfor i := 0; i < n; i++ {\n\t\t\tif !yield(i) {\n\t\t\t\treturn\n\t\t\t}\n\t\t}\n\t}\n}\n\n")
	sb.WriteString("func loops() int {\n\tdefer cleanup()\n\tvar cur *lib.rec\n")
	for i := 0; i < loops; i++ {
		fmt.Fprintf(&sb, "\tfor i := range seq(2) {\n\t\tvar x%d lib.rec\n\t\tx%d.n = int32(i)\n\t\tcur = &x%d\n\t}\n", i, i, i)
	}
	sb.WriteString("\tuse(cur)\n\treturn int(cur.n)\n}\n\nfunc lits() int {\n\tvar cur *lib.rec\n")
	for i := 0; i < lits; i++ {
		fmt.Fprintf(&sb, "\tf%d := func() {\n\t\tdefer cleanup()\n\t\tvar y%d lib.rec\n\t\tcur = &y%d\n\t}\n\tf%d()\n", i, i, i, i)
	}
	sb.WriteString("\tuse(cur)\n\treturn int(cur.n)\n}\n\nfunc main() {}\n")
	return sb.String()
}

func lowerManyBodies(t *testing.T, loops, lits int) (string, time.Duration) {
	t.Helper()
	fset, pkg, _ := checkInterop(t, manyBodiesSource(loops, lits))
	start := time.Now()
	f, err := Package(fset, pkg, fakeInterop(pkg, map[string]interop.Kind{"rec": interop.Type}))
	if err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	return string(cppir.Print(f, "")), took
}

func TestCAddrManyBodies(t *testing.T) {
	runs := addrStoresRuns.Load()
	cpp, took := lowerManyBodies(t, 1000, 3000)
	// the 6 functions of the source: the analysis of each (and the one that boxing makes of its own) is
	// made once; the 1000 loop bodies and 3000 literals inside them use the one of their function
	if n := addrStoresRuns.Load() - runs; n > 12 {
		t.Errorf("the analysis was made %d times for 6 functions with 1000 loops and 3000 literals", n)
	}
	if took > 3*raceSlowdown*time.Second {
		t.Errorf("lowering 1000 loops and 3000 literals took %v", took)
	}
	for _, v := range []string{"x0", "x999", "y0", "y2999"} {
		if !strings.Contains(cpp, "gp::ptr<::rec> "+v+" = ") {
			t.Errorf("%s is not boxed", v)
		}
	}
	t.Run("the same output as without sharing", func(t *testing.T) {
		shared, _ := lowerManyBodies(t, 40, 40)
		addrShareOff = true
		defer func() { addrShareOff = false }()
		alone, _ := lowerManyBodies(t, 40, 40)
		lines := regexp.MustCompile(`(?m)^\s*#line .*\n`) // (they name the temporary folder of the test)
		shared, alone = lines.ReplaceAllString(shared, ""), lines.ReplaceAllString(alone, "")
		if shared != alone {
			t.Errorf("the output differs when every body analyses the function on its own")
		}
	})
}

// holderPairsSource is n pairs of a local object that holds a variable (q := &x; ho.p = q) in one loop: two
// classes keep the same &x, and the analysis must box x whichever it visits first (map order).
func holderPairsSource(n int) string {
	var sb strings.Builder
	sb.WriteString("\nfunc holderPairs() int {\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "\tvar ho%d Holder\n", i)
	}
	sb.WriteString("\tfor i := 0; i < 2; i++ {\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "\t\tvar px%d lib.rec\n\t\tq%d := &px%d\n\t\tho%d.p = q%d\n\t\t_ = q%d\n", i, i, i, i, i, i)
	}
	sb.WriteString("\t}\n\treturn int(ho0.p.n)\n}\n")
	return sb.String()
}

// Many holders of the same addresses (2,000 variables each hold the 50 loop locals that cur was set to)
// and many literals: an address that many classes hold is processed once for the function, not once
// for each class and each function inside it.
func TestCAddrManyHolders(t *testing.T) {
	const holders, locals, lits = 2000, 50, 8000
	var sb strings.Builder
	sb.WriteString("package main\n\nimport \"<lib.h>\"\n\ntype H struct{ p *lib.rec }\n\nvar g *lib.rec\n\nfunc use(p *lib.rec) { g = p }\n\nfunc main() {\n\tvar cur *lib.rec\n")
	for i := 0; i < holders; i++ {
		fmt.Fprintf(&sb, "\tvar h%d H\n", i)
	}
	sb.WriteString("\tfor i := 0; i < 50; i++ {\n")
	for i := 0; i < locals; i++ {
		fmt.Fprintf(&sb, "\t\tvar x%d lib.rec\n\t\tcur = &x%d\n", i, i)
	}
	for i := 0; i < holders; i++ {
		fmt.Fprintf(&sb, "\t\th%d.p = cur\n", i)
	}
	sb.WriteString("\t}\n")
	for i := 0; i < lits; i++ {
		sb.WriteString("\tfunc() {}()\n")
	}
	for i := 0; i < holders; i++ {
		fmt.Fprintf(&sb, "\tuse(h%d.p)\n", i)
	}
	sb.WriteString("}\n")
	fset, pkg, _ := checkInterop(t, sb.String())
	listed := outlivingListed.Load()
	start := time.Now()
	f, err := Package(fset, pkg, fakeInterop(pkg, map[string]interop.Kind{"rec": interop.Type}))
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 3*raceSlowdown*time.Second {
		t.Errorf("lowering %d holders of %d addresses and %d literals took %v", holders, locals, lits, took)
	}
	// each address is listed once for each analysis of main (a few), not once for each of the holders
	if n := outlivingListed.Load() - listed; n > 4*locals {
		t.Errorf("the outliving pass listed %d addresses for %d locals (%d holders)", n, locals, holders)
	}
	cpp := string(cppir.Print(f, ""))
	for _, v := range []string{"x0", "x49"} {
		if !strings.Contains(cpp, "gp::ptr<::rec> "+v+" = ") {
			t.Errorf("%s is not boxed", v)
		}
	}
}

func TestCAddrOwnership(t *testing.T) {
	src := `package main

import (
	"unsafe"

	"<lib.h>"
)

type Holder struct {
	p    *lib.rec
	next *Holder
}

type Gen struct {
	in  lib.rec
	cnt int
}

var g *lib.rec
var gi *C.int
var gh Holder
var gs []*lib.rec
var gup unsafe.Pointer

func use(p *lib.rec) {}

func stored() {
	var a lib.rec
	g = &a
}

func member() {
	var b lib.rec
	gi = &b.n
}

func nested() {
	var c lib.rec
	gi = &c.in.n
}

func element() {
	var d lib.rec
	gi = &d.vals[1]
}

func goStruct() {
	e := Gen{}
	gi = &e.in.n
}

func aliased() {
	var f lib.rec
	q := &gh
	q.p = &f
}

func appended() {
	var h lib.rec
	gs = append(gs, &h)
}

func viaLocal() {
	var j lib.rec
	p := &j
	g = p
}

func callOnly() {
	var k lib.rec
	use(&k)
}

func localPointer() {
	var l lib.rec
	p := &l
	use(p)
}

func typeSwitch() {
	var m lib.rec
	var x any = &m
	switch v := x.(type) {
	case *lib.rec:
		use(v)
	}
}

type Wrap struct{ in lib.rec }

var gany any = &Holder{}
var ghs = []*Holder{{}}
var gm = map[string]*Holder{}
var gch = make(chan *Holder, 1)
var gholder = &Holder{}

func getHolder() *Holder { return gholder }

func rangeVar() {
	var r1 lib.rec
	for _, h := range ghs {
		h.p = &r1
	}
}

func commaOk() {
	var r2 lib.rec
	h, ok := gm["k"]
	if ok {
		h.p = &r2
	}
}

func assertion() {
	var r3 lib.rec
	h := gany.(*Holder)
	h.p = &r3
}

func sliceExpr() {
	var r4 lib.rec
	s := gs[:]
	s[0] = &r4
}

func structHolds() {
	var r5 lib.rec
	s := struct{ h *Holder }{gholder}
	s.h.p = &r5
}

func closureParam() {
	var r6 lib.rec
	func(h *Holder) { h.p = &r6 }(gholder)
}

func receive() {
	var r7 lib.rec
	h := <-gch
	h.p = &r7
}

func param(h *Holder) {
	var r8 lib.rec
	h.p = &r8
}

func viaCall() {
	var r9 lib.rec
	h := getHolder()
	h.p = &r9
}

func returned() []*lib.rec {
	var res []*lib.rec
	var r10 lib.rec
	res = append(res, &r10)
	return res
}

func (w *Wrap) receiver() {
	gi = &w.in.n
}

func (w *Wrap) receiverCopy() {
	c := w
	gi = &c.in.n
}

func goPointer(pp *Gen) {
	gi = &pp.in.n
}

var gsend chan *lib.rec = make(chan *lib.rec, 1)

func sent() {
	var r12 lib.rec
	gsend <- &r12
}

func namedResult() (h Holder) {
	var r13 lib.rec
	h.p = &r13
	return
}

func viaAlias() {
	var r11 lib.rec
	h := gholder
	k := h
	k.p = &r11
}

func slicePart() {
	sl := make([]lib.rec, 2)
	gi = &sl[1].n
}

func madeLiteral() {
	var s1 lib.rec
	h := &Holder{}
	h.p = &s1
	use(h.p)
}

func madeNew() {
	var s2 lib.rec
	h := new(Holder)
	h.p = &s2
	use(h.p)
}

func madeLocal() {
	var s3 lib.rec
	var hv Holder
	h := &hv
	h.p = &s3
	use(h.p)
}

func madeSlice() {
	var s4 lib.rec
	s := make([]*lib.rec, 1)
	s[0] = &s4
	use(s[0])
}

var gcap []*lib.rec
var gint *int
var gdone = make(chan bool)

func chain2() {
	var c2 lib.rec
	p1, p2 := &Holder{}, &Holder{}
	gholder = p2
	p2.next = p1
	p1.p = &c2
}

func chain3() {
	var c3 lib.rec
	p1, p2, p3 := &Holder{}, &Holder{}, &Holder{}
	gholder = p3
	p3.next = p2
	p2.next = p1
	p1.p = &c3
}

func aliasLeaves() {
	var c4 lib.rec
	p := &Holder{}
	q := p
	q.p = &c4
	gholder = p
}

func aliasReturned() *Holder {
	var c5 lib.rec
	p := &Holder{}
	q := p
	q.p = &c5
	return p
}

func addressOfObject() {
	var c6 lib.rec
	var h Holder
	p := &h
	p.p = &c6
	gholder = &h
}

func sliceAlias() {
	var c7 lib.rec
	s := make([]*lib.rec, 1)
	t := s
	t[0] = &c7
	gs = s
}

func fill(pp **Holder) { *pp = gholder }

func fillAddr() {
	var c8 lib.rec
	var p *Holder
	fill(&p)
	p.p = &c8
}

func copyPackage() {
	var c9 lib.rec
	copy(gs, []*lib.rec{&c9})
}

func copyLocal() {
	var c10 lib.rec
	d := make([]*lib.rec, 1)
	copy(d, []*lib.rec{&c10})
	gs = d
}

func appendCapacity() {
	var c11 lib.rec
	s := append(gcap, nil)
	s[0] = &c11
}

func goArgument() {
	var c12 lib.rec
	p := &Holder{}
	p.p = &c12
	go func(h *Holder) { use(h.p) }(p)
}

func goCapture() {
	var c13 lib.rec
	p := &Holder{}
	p.p = &c13
	go func() { use(p.p) }()
}

func fillSlice(pp *[]*lib.rec) { *pp = gs }

func addrOfSlice() {
	var c14 lib.rec
	s := make([]*lib.rec, 1)
	fillSlice(&s)
	s[0] = &c14
}

func rangeLocal() {
	var c15 lib.rec
	local := []*Holder{gholder}
	for _, h := range local {
		h.p = &c15
	}
}

func funcLitArgument() {
	var c16 lib.rec
	p := &Holder{}
	p.p = &c16
	func(h *Holder) { gholder = h }(p)
}

func appendDiscarded() {
	var c17 lib.rec
	s := append(gcap, &c17)
	_ = s
}

func sliceOfSlice() {
	var c18 lib.rec
	s := make([]*lib.rec, 1)
	t := s[:]
	t[0] = &c18
	gs = s
}

func sliceOfArray() {
	var c19 lib.rec
	var arr [2]*lib.rec
	t := arr[:]
	t[0] = &c19
	gs = arr[:]
}

func rangeThenStore() {
	var l5 lib.rec
	local := make([]*lib.rec, 1)
	for _, e := range local {
		use(e)
	}
	local[0] = &l5
	use(local[0])
}

func sliceOfSliceLocal() {
	var l6 lib.rec
	s := make([]*lib.rec, 1)
	t := s[:]
	t[0] = &l6
	use(t[0])
}

func sliceOfArrayLocal() {
	var l7 lib.rec
	var arr [2]*lib.rec
	t := arr[:]
	t[0] = &l7
	use(t[0])
}

type Holders []Holder

var gholders2 Holders

func (p *Holders) Fill() { *p = gholders2 }

func implicitAddress() {
	var c20 lib.rec
	s := make(Holders, 1)
	s.Fill()
	s[0].p = &c20
}

func chainLocal() {
	var l1 lib.rec
	p1, p2 := &Holder{}, &Holder{}
	p2.next = p1
	p1.p = &l1
	use(p2.next.p)
}

func aliasLocal() {
	var l2 lib.rec
	p := &Holder{}
	q := p
	q.p = &l2
	use(p.p)
}

func copyLocalOnly() {
	var l3 lib.rec
	d := make([]*lib.rec, 1)
	copy(d, []*lib.rec{&l3})
	use(d[0])
}

func goOwnLocal() {
	go func() {
		var l4 lib.rec
		use(&l4)
	}()
}

func wholeField(pp *Gen) {
	g = &pp.in
}

func goField(pp *Gen) {
	gint = &pp.cnt
}

func useHolder(h Holder) {}

func goNamedAddress() {
	var o1 lib.rec
	go use(&o1)
}

func goNamedPointer() {
	var o2 lib.rec
	p := &o2
	go use(p)
}

func goNamedHolder() {
	var o3 lib.rec
	h := Holder{p: &o3}
	go useHolder(h)
}

func goNamedResult() {
	var o11 lib.rec
	go fillRec(&o11)
}

func paramAlias(param *Holder) {
	var o4 lib.rec
	q := param
	q.p = &o4
}

func depthTwo() {
	var o5 lib.rec
	p := &Holder{}
	p.next = &gh
	p.next.p = &o5
}

func mapParam(m map[string]*Holder) {
	var o6 lib.rec
	m["a"] = &Holder{p: &o6}
}

func starParam(h *Holder) {
	var o7 lib.rec
	*h = Holder{p: &o7}
}

func fillRec(p *lib.rec) int32 { return 1 }

func geti(p *C.int) int32 { return 1 }

func resultOfCall() int {
	var v1 lib.rec
	return int(fillRec(&v1))
}

func resultPlusOne() int {
	var v2 C.int
	return int(geti(&v2)) + 1
}

func resultOfLocalPointer() int {
	var v3 lib.rec
	p := &v3
	return int(fillRec(p))
}

func resultOfLoad() int {
	var v4 C.int
	p := &v4
	return int(*p)
}

func resultsOfStatus() (int, int) {
	var v5 lib.rec
	rc := fillRec(&v5)
	use(nil)
	return int(rc), int(v5.n)
}

func outlivesAppend() int {
	var list []*lib.rec
	for i := 0; i < 2; i++ {
		var o8 lib.rec
		list = append(list, &o8)
	}
	return int(list[0].n)
}

func outlivesPointer() int {
	var p *lib.rec
	for i := 0; i < 2; i++ {
		var o9 lib.rec
		p = &o9
	}
	return int(p.n)
}

func outlivesHolder() int {
	var h Holder
	for i := 0; i < 2; i++ {
		var o10 lib.rec
		h.p = &o10
	}
	return int(h.p.n)
}

func insideBlock() int {
	t := 0
	for i := 0; i < 2; i++ {
		var l8 lib.rec
		var list []*lib.rec
		list = append(list, &l8)
		t += int(list[0].n)
	}
	return t
}

// (the frame of a function with a defer statement holds the local to the end: addrHeld)
func outlivesWithDefer() int {
	defer use(nil)
	var p *lib.rec
	for i := 0; i < 2; i++ {
		var l10 lib.rec
		p = &l10
	}
	return int(p.n)
}

func sameBlock() int {
	var l9 lib.rec
	p := &l9
	return int(p.n)
}

func cleanup() {}

func closureDeferVar() int {
	var cur *lib.rec
	f := func() {
		defer cleanup()
		var o12 lib.rec
		cur = &o12
	}
	f()
	return int(cur.n)
}

func closureDeferStruct() int {
	var h Holder
	f := func() {
		defer cleanup()
		var o13 lib.rec
		h.p = &o13
	}
	f()
	return int(h.p.n)
}

func closureDeferAppend() int {
	var list []*lib.rec
	add := func() {
		defer cleanup()
		var o14 lib.rec
		list = append(list, &o14)
	}
	add()
	add()
	return int(list[0].n)
}

func closureDeferBlock() int {
	var cur *lib.rec
	f := func() {
		defer cleanup()
		for i := 0; i < 2; i++ {
			var o15 lib.rec
			cur = &o15
		}
	}
	f()
	return int(cur.n)
}

func closureInClosureDefer() int {
	var cur *lib.rec
	f := func() {
		defer cleanup()
		g := func() {
			defer cleanup()
			var o16 lib.rec
			cur = &o16
		}
		g()
	}
	f()
	return int(cur.n)
}

func goClosureDefer() int {
	var cur *lib.rec
	done := make(chan bool)
	go func() {
		defer cleanup()
		var o17 lib.rec
		cur = &o17
		done <- true
	}()
	<-done
	return int(cur.n)
}

func seq(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := 0; i < n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func rangeFuncDefer() int {
	defer cleanup()
	var cur *lib.rec
	for range seq(2) {
		var o18 lib.rec
		cur = &o18
	}
	return int(cur.n)
}

func rangeFuncInLoopDefer() int {
	defer cleanup()
	var cur *lib.rec
	for j := 0; j < 2; j++ {
		for range seq(2) {
			var o19 lib.rec
			cur = &o19
		}
	}
	return int(cur.n)
}

func rangeFuncNoDefer() int {
	var cur *lib.rec
	for range seq(2) {
		var o20 lib.rec
		cur = &o20
	}
	return int(cur.n)
}

func declDeferRangeSlice() int {
	defer cleanup()
	var cur *lib.rec
	for range []int{1, 2} {
		var o25 lib.rec
		cur = &o25
	}
	return int(cur.n)
}

func twoRangeFuncLoops() int {
	defer cleanup()
	n := 0
	for i := range seq(2) {
		n += i
	}
	var cur *lib.rec
	for range seq(2) {
		var o26 lib.rec
		cur = &o26
	}
	return int(cur.n) + n
}

// (the locals before and after a range-over-func loop are not in its body: the frame holds them)
func declDeferAroundRangeFunc() int {
	defer cleanup()
	n := 0
	var cur *lib.rec
	{
		var o27 lib.rec
		cur = &o27
	}
	for i := range seq(2) {
		n += i
	}
	{
		var o28 lib.rec
		cur = &o28
	}
	return int(cur.n) + n
}

var gfn func()

// the local of a literal that leaves is the literal's own: its address stays inside
func litLocalLeaves() {
	f := func() {
		var l13 lib.rec
		use(&l13)
	}
	gfn = f
}

func ifaceLeak() {
	var o21 lib.rec
	var i any = &o21
	gany = i
}

func ifaceReturn() any {
	var o22 lib.rec
	var i any = &o22
	return i
}

func unsafeLeak() {
	var o23 lib.rec
	up := unsafe.Pointer(&o23)
	gup = up
}

func unsafeReturn() unsafe.Pointer {
	var o24 lib.rec
	up := unsafe.Pointer(&o24)
	return up
}

type Lits struct{ a, b func() }

var glits Lits

func mkLits(a, b func()) Lits { return Lits{a, b} }

// two literals in one statement: the locals of both are their own
func twoLiterals() {
	glits = mkLits(func() {}, func() {
		var l14 lib.rec
		p := &l14
		use(p)
	})
}

// The package-level initializers: gp_var_init lowers all of them with one funcGen, and a literal in the
// second, third ... initializer is analysed with its own initializer.
var pinit1 func() int = func() int { return 1 }

var pinit2 func() *lib.rec = func() *lib.rec {
	var o29 lib.rec
	p := &o29
	return p
}

var pinit3 func() *lib.rec = func() *lib.rec {
	var o30 lib.rec
	p := &o30
	return p
}

var (
	pinit4 func() int = func() int { return 2 }
	pinit5 func() *lib.rec = func() *lib.rec {
		var o31 lib.rec
		p := &o31
		return p
	}
	pinit6 func() int = func() int {
		var o32 lib.rec
		g = &o32
		return 0
	}
	pinit7 Lits = Lits{a: func() {
		var l15 lib.rec
		use(&l15)
	}, b: func() {
		var o34 lib.rec
		gs = append(gs, &o34)
	}}
)

// a literal that stores its own local into a holder of the function that reads it afterwards
func litStoresOuter() *lib.rec {
	var h Holder
	func() {
		var o40 lib.rec
		h.p = &o40
	}()
	return h.p
}

// three levels: xc into c, c into b, b into a
func litNest3() *lib.rec {
	var a *lib.rec
	func() {
		var b *lib.rec
		func() {
			var c *lib.rec
			func() {
				var o41 lib.rec
				c = &o41
			}()
			b = c
		}()
		a = b
	}()
	return a
}

// a holder copied inside a literal into a package variable
func litCopiesHolder() {
	var o42 lib.rec
	var h1, h2 Holder
	h1.p = &o42
	func() { h2 = h1 }()
	g = h2.p
}

// a deferred literal that stores the address of the enclosing function's local
func deferredLitStore() {
	var o43 lib.rec
	defer func() { g = &o43 }()
}

// a literal kept in a variable and called twice
func litCalledTwice() *lib.rec {
	var cur *lib.rec
	set := func() {
		var o44 lib.rec
		cur = &o44
	}
	set()
	set()
	return cur
}

// the holder is only read in the function (nothing leaves it): the literal's local still outlives the literal
func litHolderRead() int {
	var h Holder
	var x lib.rec
	var a Holder
	a.p = &x
	func() {
		var o47 lib.rec
		h.p = &o47
		_ = a
	}()
	return int(h.p.n) + int(a.p.n)
}

func litVarRead() int {
	var cur *lib.rec
	set := func() {
		var o48 lib.rec
		cur = &o48
	}
	set()
	return int(cur.n)
}

func litNest3Read() int {
	var a *lib.rec
	func() {
		var b *lib.rec
		func() {
			var c *lib.rec
			func() {
				var o49 lib.rec
				c = &o49
			}()
			b = c
		}()
		a = b
	}()
	return int(a.n)
}

// the literal takes the address of the enclosing function's local and stores it where the function reads it
func litTakesOuter() *lib.rec {
	var o50 lib.rec
	var cur *lib.rec
	func() {
		cur = &o50
	}()
	return cur
}

func litTakesOuterGo() {
	var o51 lib.rec
	var cur *lib.rec
	f := func() { cur = &o51 }
	f()
	go func() { g = cur }()
}

func genericKeep[T any](v T) *lib.rec {
	var o45 lib.rec
	var cur *lib.rec
	func() {
		cur = &o45
	}()
	return cur
}

func genericClosure[T any](v T) *lib.rec {
	var cur *lib.rec
	func() {
		var o46 lib.rec
		cur = &o46
	}()
	return cur
}

func genericUser() {
	genericKeep[int](1)
	genericKeep[string]("a")
	genericKeep[float64](1)
	genericClosure[int](1)
	genericClosure[string]("a")
	genericClosure[float64](1)
}

func main() {}
`
	src += holderPairsSource(24)
	fset, pkg, _ := checkInterop(t, src)
	kinds := map[string]interop.Kind{"rec": interop.Type}
	f, err := Package(fset, pkg, fakeInterop(pkg, kinds))
	if err != nil {
		t.Fatal(err)
	}
	cpp := string(cppir.Print(f, ""))
	body := func(name string) string {
		loc := regexp.MustCompile(`(?m)^\S.*\b(?:\w+::)?` + name + `\(.*\) \{$`).FindStringIndex(cpp)
		if loc == nil {
			t.Fatalf("no function %s in the C++:\n%s", name, cpp)
		}
		end := strings.Index(cpp[loc[0]:], "\n}\n")
		return cpp[loc[0] : loc[0]+end]
	}
	// The address of the variable is an owning gp::cptr of its box (or, for a part, an aliasing one
	// that shares the box): `gp::cptr<::rec>(v)`, with v declared as a box.
	for name, v := range map[string]string{
		"stored": "a", "aliased": "f", "appended": "h", "viaLocal": "j",
		"rangeVar": "r1", "commaOk": "r2", "assertion": "r3", "sliceExpr": "r4", "structHolds": "r5",
		"closureParam": "r6", "receive": "r7", "param": "r8", "viaCall": "r9", "returned": "r10",
		"viaAlias": "r11", "sent": "r12", "namedResult": "r13",
		"chain2": "c2", "chain3": "c3", "aliasLeaves": "c4", "aliasReturned": "c5", "addressOfObject": "c6",
		"sliceAlias": "c7", "fillAddr": "c8", "copyPackage": "c9", "copyLocal": "c10", "appendCapacity": "c11",
		"goArgument": "c12", "goCapture": "c13", "addrOfSlice": "c14", "rangeLocal": "c15", "funcLitArgument": "c16", "appendDiscarded": "c17", "implicitAddress": "c20", "sliceOfSlice": "c18", "sliceOfArray": "c19",
		"goNamedAddress": "o1", "goNamedPointer": "o2", "goNamedHolder": "o3", "goNamedResult": "o11", "paramAlias": "o4",
		"depthTwo": "o5", "mapParam": "o6", "starParam": "o7",
		"outlivesAppend": "o8", "outlivesPointer": "o9", "outlivesHolder": "o10",
		"closureDeferVar": "o12", "closureDeferStruct": "o13", "closureDeferAppend": "o14", "closureDeferBlock": "o15",
		"closureInClosureDefer": "o16", "goClosureDefer": "o17", "rangeFuncDefer": "o18", "rangeFuncInLoopDefer": "o19",
		"litStoresOuter": "o40", "litNest3": "o41", "litCopiesHolder": "o42", "deferredLitStore": "o43", "litCalledTwice": "o44",
		"litHolderRead": "o47", "litVarRead": "o48", "litNest3Read": "o49",
		"litTakesOuter": "o50", "litTakesOuterGo": "o51",
		"rangeFuncNoDefer": "o20", "twoRangeFuncLoops": "o26", "ifaceLeak": "o21", "ifaceReturn": "o22", "unsafeLeak": "o23", "unsafeReturn": "o24",
	} {
		t.Run("owned "+name, func(t *testing.T) {
			text := body(name)
			if !strings.Contains(text, "gp::ptr<::rec> "+v+" = ") || !strings.Contains(text, "gp::cptr<::rec>("+v+")") {
				t.Errorf("the stored address of %s in %s is not an owning gp::cptr of its box:\n%s", v, name, text)
			}
		})
	}
	for name, v := range map[string]string{"member": "b", "nested": "c", "element": "d", "goStruct": "e"} {
		t.Run("owned part "+name, func(t *testing.T) {
			text := body(name)
			if !strings.Contains(text, "gp::cptr<") || !strings.Contains(text, "("+v+", &") {
				t.Errorf("the address of a part of %s in %s is not an aliasing gp::cptr that shares its box:\n%s", v, name, text)
			}
		})
	}
	for name, want := range map[string]string{"receiver": "(w, &", "receiverCopy": "(c, &", "goPointer": "(pp, &"} {
		t.Run("owned member "+name, func(t *testing.T) {
			text := body(name)
			if !strings.Contains(text, "gp::cptr<") || !strings.Contains(text, want) {
				t.Errorf("the member address in %s is not an aliasing gp::cptr that shares %q:\n%s", name, want, text)
			}
		})
	}
	for name, local := range map[string]string{
		"callOnly": "k", "localPointer": "l", "typeSwitch": "m",
		"madeLiteral": "s1", "madeNew": "s2", "madeLocal": "s3", "madeSlice": "s4",
		"chainLocal": "l1", "aliasLocal": "l2", "copyLocalOnly": "l3", "goOwnLocal": "l4",
		"rangeThenStore": "l5", "sliceOfSliceLocal": "l6", "sliceOfArrayLocal": "l7",
		"resultOfCall": "v1", "resultOfLocalPointer": "v3", "resultsOfStatus": "v5",
		"insideBlock": "l8", "sameBlock": "l9", "twoLiterals": "l14", "litLocalLeaves": "l13",
	} {
		t.Run("raw "+name, func(t *testing.T) {
			text := body(name)
			if !strings.Contains(text, "::rec "+local+"{};") {
				t.Errorf("%s is not a local of the C++ stack in %s:\n%s", local, name, text)
			}
		})
	}
	// the frame of a function declaration with a defer holds the local to the end (gp_hold), so the
	// address stays a plain one; the body of a range over a slice is no function of its own
	for name, vs := range map[string][]string{
		"outlivesWithDefer": {"l10"}, "declDeferRangeSlice": {"o25"}, "declDeferAroundRangeFunc": {"o27", "o28"},
	} {
		t.Run("outlives with defer "+name, func(t *testing.T) {
			text := body(name)
			for _, v := range vs {
				if !strings.Contains(text, "gp_hold("+v+")") || strings.Contains(text, "gp::cptr<::rec>("+v+")") {
					t.Errorf("%s is not held by the frame alone:\n%s", v, text)
				}
			}
		})
	}
	t.Run("package initializers", func(t *testing.T) {
		text := body("gp_var_init")
		for _, v := range []string{"o29", "o30", "o31", "o32", "o34"} {
			if !strings.Contains(text, "gp::ptr<::rec> "+v+" = ") || !strings.Contains(text, "gp::cptr<::rec>("+v+")") {
				t.Errorf("%s of a package-level initializer is not an owning gp::cptr of its box:\n%s", v, text)
			}
		}
		if !strings.Contains(text, "::rec l15{};") {
			t.Errorf("l15 of a package-level literal is not a local of the C++ stack:\n%s", text)
		}
	})
	t.Run("generic instances", func(t *testing.T) {
		for _, v := range []string{"o45", "o46"} {
			if n := strings.Count(cpp, "gp::ptr<::rec> "+v+" = "); n != 3 {
				t.Errorf("%s is boxed in %d of the 3 instances (int, string, float64)", v, n)
			}
		}
	})
	t.Run("two holders of one address", func(t *testing.T) {
		text := body("holderPairs")
		for i := 0; i < 24; i++ {
			v := fmt.Sprintf("px%d", i)
			if !strings.Contains(text, "gp::ptr<::rec> "+v+" = ") {
				t.Errorf("%s is not boxed:\n%s", v, text)
			}
		}
	})
	// the same for a C scalar (an opaque value): the address that only goes to a call, or is read
	// back, keeps the local on the stack
	for name, decl := range map[string]string{"resultPlusOne": "int v2{};", "resultOfLoad": "int v4{};"} {
		t.Run("raw scalar "+name, func(t *testing.T) {
			if text := body(name); !strings.Contains(text, decl) {
				t.Errorf("%s is not a local of the C++ stack in %s:\n%s", decl, name, text)
			}
		})
	}
	// A step through a slice or a Go pointer leaves the variable's own storage: the variable is not
	// boxed for the address of a member of what it points to. (&sl[i].f of a slice's element is
	// documented as not covered: the pointer is raw.)
	t.Run("a C field of a Go struct is no member of a C object", func(t *testing.T) {
		// (the ordinary address of a field shares the owner; the member rule is for what has no
		// selection: a member of a C/C++ type)
		text := body("wholeField")
		if !strings.Contains(text, "gp::cptr<") || strings.Contains(text, "decltype") {
			t.Errorf("the address of a C field of a Go struct is not the ordinary owning pointer:\n%s", text)
		}
	})
	t.Run("a Go field of a pointer is no C member", func(t *testing.T) {
		if text := body("goField"); strings.Contains(text, "gp::cptr<") || strings.Contains(text, "decltype") {
			t.Errorf("the address of a Go field is wrapped as the address of a C/C++ member:\n%s", text)
		}
	})
	for _, name := range []string{"slicePart", "goPointer"} {
		t.Run("variable not boxed "+name, func(t *testing.T) {
			text := body(name)
			if strings.Contains(text, "gp::ptr<gp::slice") || strings.Contains(text, "gp::ptr<gp::ptr<Gen>>") {
				t.Errorf("a variable of %s is boxed for the address of what it points to:\n%s", name, text)
			}
		})
	}
	for name, want := range map[string]string{
		"member":   "(b, &gp::deref((*b)).n)", // shares b's box
		"element":  "(d, &",
		"goStruct": "(e, &",
	} {
		if text := body(name); !strings.Contains(text, want) {
			t.Errorf("%s: no %q in\n%s", name, want, text)
		}
	}
}
