package lower

import (
	"context"
	"errors"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
		"int f(int n, ::FILE* p) {",
	} {
		if !strings.Contains(cpp, want) {
			t.Errorf("no %q in the C++:\n%s", want, cpp)
		}
	}
	if strings.Contains(cpp, "#undef MAC") || strings.Contains(cpp, "#undef Bool") {
		t.Errorf("a macro that the code uses, or a renamed name's, is #undef'd:\n%s", cpp)
	}
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
		{"var v std.vector[int]; f := func() { _ = v }; f()", "C/C++ values in closures (a func literal that uses v: step 4.8)"},
		{"var v, w std.vector[int]; defer func() { std.swap(v, w) }()", "C/C++ values in closures (a func literal that uses v: step 4.8)"},
		{"var h H; f := func() { _ = h.p }; f()", "C/C++ values in closures (a func literal that uses h: step 4.8)"},
		{"_ = func(x C.int) {}", "func literals with C/C++ parameters or results (step 4.11)"},
		{"var x std.vector[func(int) int]; _ = x", "func types as C++ template arguments (step 4.11)"},
		{"var h H; switch h { case h: }", "switch on values with C/C++ parts (Phase 12)"},
		{"var h, k H; _ = h == k", "== on values with C/C++ parts (Phase 12)"},
		{"var m map[H]int; _ = m", "this use of a C/C++ type (a later step of Phase 04)"},
		{"var a A; _ = a", "aliases of C/C++ types (A): write the C/C++ type (a later step of Phase 04)"},
		{"var v std.vector[int]; for range v {}", "range over C/C++ values (gp::range, step 4.7)"},
		{"var v std.vector[int]; defer v.clear()", "deferred calls of members of C/C++ values (the C++ object must outlive the function: step 4.8)"},
		{"_ = std.vector[int]{1}", "composite literals of C/C++ types (step 4.6)"},
		{"var v std.vector[int]; _ = &v[0]", "& of C/C++ values other than variables (step 4.10)"},
		{"for i := C.int(0); i < 3; i++ { lib.f(&i) }", "& of a C/C++ loop variable (i: each iteration has its own in Go; step 4.8)"},
		{"var p *lib.FILE; defer lib.f(&p)", "& of a C/C++ local in a defer or go statement (the local must outlive the function: step 4.8)"},
		{"var h H; _ = &h.q", "& of C/C++ values other than variables (step 4.10)"},
		{"var p *C.char; _ = p[1:]", "slicing C/C++ values (step 4.10)"},
		{"_ = H{p: nil, q: lib.f()}", "C/C++ values in composite literals (step 4.6)"},
		{"var h H; w := func() int { return len(h.s) }; _ = w", "C/C++ values in closures (a func literal that uses h: step 4.8)"},
		{"_ = string(lib.f())", "conversions of C/C++ values to string (step 4.9)"},
		{"_ = (*C.char)(nil)", "conversions to this C/C++ type (steps 4.9 and 4.10)"},
		{"lib.f(1, lib.g()...)", "... in a call of a C/C++ function"},
		{"var v std.vector[int]; v.push_back(lib.g()...)", "... in a call of a C/C++ function"},
		{"var v std.vector[int]; _ = append([]int{}, v...)", "append of C/C++ values"},
		{"var p *C.int; _ = unsafe.Slice(p, 2)", "unsafe.Slice of C/C++ values (step 4.10)"},
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
	kinds := map[string]interop.Kind{"f": interop.Value, "g": interop.Value, "pow": interop.Value, "FILE": interop.Type,
		"EOF": interop.Macro, "MODE_B": interop.Value, "std::string": interop.Type, "std::vector": interop.Template,
		"std::minmax": interop.ValueTemplate, "std::map": interop.Template}
	tests := []struct{ src, want string }{
		{"var iv int = lib.pow(10, 30); _ = iv",
			"cannot use lib.pow(10, 30) (C/C++ value) as int value in variable declaration: convert it, int(lib.pow(10, 30))"},
		{"var iv int; iv = lib.f(); _ = iv",
			"cannot use lib.f() (C/C++ value) as int value in assignment: convert it, int(lib.f())"},
		{"gi(lib.f())", "cannot use lib.f() (C/C++ value) as int value in argument to gi: convert it, int(lib.f())"},
		{"gs(std.string(\"x\"))",
			`cannot use std.string("x") (C/C++ value) as string value in argument to gs: convert it, string(std.string("x")) (step 4.9)`},
		{"_ = ri", ""},
		{"var p *lib.FILE; _ = uintptr(p)", "cannot convert p (C pointer) to type uintptr: use unsafe.Pointer (step 4.10)"},
		{"var p *lib.FILE; _ = int(p)", "cannot convert p (C pointer) to type int: use unsafe.Pointer (step 4.10)"},
		{"var p *lib.FILE; _ = C.int(p)", "cannot convert p (C pointer) to type C.int: use unsafe.Pointer (step 4.10)"},
		{"var p *lib.FILE; _ = bool(p)", "cannot convert p (C pointer) to type bool: compare it: p != nil"},
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
		{"var x C.int; const d = x + 1", "x + 1 (C/C++ value) is not constant"},
		{"var x C.int; const c = x == 2", "x == 2 (C/C++ value) is not constant"},
		{"const k = lib.f()", "lib.f() (C/C++ value) is not constant"},
		{"const k C.int = 1 << 3; const j = C.int(2)", ""},
		{"var xs []C.int; _ = xs[1:]; var arr [3]C.int; for i, v := range arr { _, _ = i, v }; m := make(map[int]C.int); v, ok := m[1]; _, _ = v, ok", ""},
		{"var cs []C.int; n := 1; cs = append(cs, n)", "cannot use n (variable of type int) as C.int value in argument to append: convert it, C.int(n)"},
		{"var cs []C.int; cs = append(cs, 1, C.int(2)); ds := make([]C.int, 2); copy(ds, cs); m := make(map[C.int]C.double); delete(m, 1); clear(ds); _ = len(cs) + cap(ds) + len(m)", ""},
		{"var m std.map[int, int]; v, ok := m[1]; _, _ = v, ok",
			"assignment mismatch: 2 variables but m[1] is 1 value: a C++ map has no comma-ok form, use m.find(k) != m.end() or m.count(k) == 1"},
		{"p := lib.f(); var a, b = p.value; _, _ = a, b", "assignment mismatch: 2 variables but p.value is 1 value"},
		{"_ = std.vector[int].iterator.x",
			"the compiler probe can't tell whether std.vector[int].iterator is a C/C++ type or a value: if it is a value, write (std.vector[int].iterator).x"},
		// These lower.
		{"var q Q; q.Put(5); pq := &q; pq.Put(6); _ = q.Size; _ = Q.Size; _ = int(q.Size())", ""},
		{"var ci C.int; switch ci { case 3, lib.EOF, lib.MODE_B: }", ""},
		{"var ci C.int = lib.f(); var n int = int(ci); gi(int(lib.f())); _ = n", ""},
		{"fv := 1e30; var a C.int = 5; var b C.double = 2.5; var c C.int = C.int(fv); var q Q; q.Put(7); lib.f(fv, 'x'); _, _, _ = a, b, c", ""},
	}
	for _, tt := range tests {
		src := "package main\n\nimport (\n\t\"<lib.h>\"\n\t\"<map>\"\n\t\"<string>\"\n\t\"<vector>\"\n)\n\n" +
			"type Q struct{ n C.int }\n\ntype B bool\n\nfunc (q *Q) Put(n C.int) { q.n = n }\n\nfunc (q Q) Size() C.int { return q.n }\n\n" +
			"func gi(n int) {}\n\nfunc gs(s string) {}\n\n" +
			"func main() {\n\t" + tt.src + "\n}\n"
		if tt.src == "_ = rc" {
			src += "\nfunc rc() C.int { n := 5; return n }\n"
			tt.want = "cannot use n (variable of type int) as C.int value in return statement: convert it, C.int(n)"
		}
		if tt.src == "_ = rp" {
			src += "\nfunc rp() **lib.FILE { var p *lib.FILE; return &p }\n"
			tt.want = "&p of a C/C++ local can't be a result: C/C++ values follow C++'s lifetime rules, and p ends with the function"
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
		"auto p = ::head();",
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
		"auto gp_t0 = gp::values(::std::minmax(1, 2));", // the references of the pair are copied
		"auto lo = std::get<0>(gp_t0);",
		"auto hi = std::get<1>(gp_t0);",
	} {
		if !strings.Contains(cpp, want) {
			t.Errorf("no %q in the C++:\n%s", want, cpp)
		}
	}
	if strings.Contains(cpp, "gp::ptr<unsigned int>") || strings.Contains(cpp, "gp::new_<unsigned int>") {
		t.Errorf("&x of a C variable boxed it:\n%s", cpp)
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
		"gp::fmtshim::Println(gp::cgo::fmt_arg(EOF), int64_t(1), gp::cgo::fmt_arg(h), gp::cgo::fmt_arg(p));",
		`gp::fmtshim::Printf("%d\n"_gs, gp::cgo::fmt_arg(::f()));`,
		"gp::fmtshim::Sprint(gp::cgo::fmt_arg(gp_a)...)",
		"gp::println(gp::cgo::print_arg(::f()), int64_t(2));",
		"gp::println(gp::cgo::print_arg(gp_a)...)",
	} {
		if !strings.Contains(cpp, want) {
			t.Errorf("the C++ has no %q:\n%s", want, cpp)
		}
	}
}
