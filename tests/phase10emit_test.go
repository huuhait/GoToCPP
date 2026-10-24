package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// helperMarkers are the signs of a helper call of the hidden packages of Phase 10 in generated C++: the
// namespace of gocpp/must and of gocpp/macro (the C++ name of an import path is gpkg_ plus the path with _), the
// alias that the rewrite gives them, and the helpers' names with their arguments.
var helperMarkers = regexp.MustCompile(`gpkg_gocpp_must|gpkg_gocpp_macro|gp_must\b|gp_macro\b|gp_site_|gp::macro::|gp::string gp_(?:file|func|text_\w*)_\b|gocpp/must|gocpp/macro|\bgp::must::M[123]\b|\bM[123]<`)

// The pattern gp::string gp_file_ (and gp_func_, gp_text_x_) is a marker too: the hidden place arguments of a macro are gp::static_view,
// because a gp::string built in every iteration of a hot loop cost three times the loop (docs/benchmarks/must-macros.md).

// ordinaryPlaceNames are the parameters of the ordinary functions of umacro_gp_names.gp that are named like a macro's hidden arguments.
var ordinaryPlaceNames = regexp.MustCompile(`gp::string gp_(?:file|func|text_\w*)_\b`)

// phase10Prefixes are the names of the tests that use `!`, macros or #Derive.
var phase10Prefixes = []string{"must_", "umacro_", "macro_", "derive_"}

// TestPhase10NoHelpers checks the exit criterion "no helper calls left in generated C++": `!` is lowered to the
// control flow a programmer writes (step 10.5), macros and #Derive are plain Go functions and methods before the
// lowering (steps 10.9 and 10.10), so the C++ of every test program of Phase 10 (tests/run, the module folders
// of tests/pkgs and the golden tests) must name none of the hidden packages' helpers. The only helper that stays is
// the runtime's gp::must_exit at the end of `func main!()` (a runtime function, not a call of gocpp/must).
func TestPhase10NoHelpers(t *testing.T) {
	c := setup(t)
	var progs []*program
	for _, f := range []format{runFormat, pkgsFormat, goldenFormat} {
		for _, p := range loadTests(t, f) {
			if f.kind == "golden" && !strings.HasPrefix(p.name, "must") && !strings.HasPrefix(p.name, "macros") && p.name != "derive" {
				continue
			}
			for _, pre := range phase10Prefixes {
				if strings.HasPrefix(p.name, pre) || f.kind == "golden" {
					progs = append(progs, p)
					break
				}
			}
		}
	}
	// The benchmark programs of Phase 10 and the examples with `!` and macros: modules in folders.
	var dirs []string
	if cases, err := filepath.Glob(filepath.Join(repoRoot, "bench", "phase10", "*", "*")); err == nil {
		dirs = append(dirs, cases...)
	}
	dirs = append(dirs, filepath.Join(repoRoot, "examples", "must-errors"))
	dirs = append(dirs, filepath.Join(repoRoot, "examples", "macros"))
	// The evidence programs of tests/phase10evidence_test.go (macros and `!` across packages).
	for _, rel := range []string{"inline", "cross_wrap/macro", "bang_forms/bang"} {
		dirs = append(dirs, filepath.Join(repoRoot, "tests", "testdata", "phase10", filepath.FromSlash(rel)))
	}
	for _, d := range dirs {
		rel, _ := filepath.Rel(repoRoot, d)
		progs = append(progs, &program{name: strings.ReplaceAll(filepath.ToSlash(rel), "/", "_"), path: filepath.ToSlash(rel), dir: d, arg: ".", header: &header{phase: 9}})
	}
	if len(progs) < 100 {
		t.Fatalf("found only %d Phase 10 programs", len(progs))
	}
	c.runPrograms(t, "Phase 10 programs", progs, func(t *testing.T, p *program) {
		r := c.run(t, gocppJob(p.dir, nil, "", p.header.phase, "emit", p.arg))
		if r.problem != "" || r.exit != 0 {
			t.Fatalf("%s: %s\nstderr:\n%s", r.cmd, r.why(), indent(string(r.stderr)))
		}
		for n, line := range strings.Split(string(r.stdout), "\n") {
			if strings.HasPrefix(line, "#line") {
				continue
			}
			if p.name == "umacro_gp_names" {
				// This test calls ordinary functions whose parameters are named gp_file_ and so on, as gp::string on purpose.
				line = ordinaryPlaceNames.ReplaceAllString(line, "")
			}
			if m := helperMarkers.FindString(line); m != "" {
				t.Errorf("%s: line %d of the C++ names the hidden helper %q: %s", r.cmd, n+1, m, strings.TrimSpace(line))
				return
			}
		}
	})
}

// TestHelperMarkers checks what helperMarkers finds: the C++ names of the hidden packages' helpers, and not the
// names that the lowering writes (must_t1, must_e1, must_main) or the runtime's gp::must_exit.
func TestHelperMarkers(t *testing.T) {
	for _, line := range []string{
		"auto gp_t0 = gpkg_gocpp_must::M1<int64_t>(f(x));",
		"gpkg_gocpp_must::Any(std::tuple(a, b));",
		"gp_must.M2(f())",
		"gpkg_gocpp_macro::Outer",
		"static Site gp_site_3;",
		"gp::macro::Site s;",
		"void gp_inst_1_Assert(gp::string gp_file_, int64_t gp_line_, gp::string gp_func_) {",
		"int64_t gp_inst_2_Dbg(gp::static_view gp_file_, gp::string gp_text_v_, int64_t v);",
		"#include \"gpkg/gocpp/must/pkg.hpp\"" + " // gocpp/must",
		"x = gp::must::M3(g());",
		"auto v = M1<int64_t>(f());",
	} {
		if helperMarkers.FindString(line) == "" {
			t.Errorf("no helper found in %q", line)
		}
	}
	for _, line := range []string{
		"int64_t must_t1 = std::get<0>(gp_t0);",
		"gp::error must_e1 = std::get<1>(gp_t0);",
		"gp::error must_main();",
		"gp::must_exit(must_main());",
		"return {0, must_e2};",
		"int64_t must(gp::string s) {",
		"auto M = 3; auto M1x = 4; // not a helper call",
		"void gp_inst_1_Assert(gp::static_view gp_file_, int64_t gp_line_, gp::static_view gp_func_) {",
	} {
		if m := helperMarkers.FindString(line); m != "" {
			t.Errorf("%q is found in %q", m, line)
		}
	}
}

// TestUsesPhase10Syntax tests the detector behind the "no twin" failure of TestRunVsGo: the syntax of Phase 10 in a
// single file or anywhere in a module (not in its twin), and no false positive for plain Go that mentions the
// signs (`!=`, `if!(a)`, strings and comments).
func TestUsesPhase10Syntax(t *testing.T) {
	for _, tt := range []struct {
		name, src string
		want      bool
	}{
		{"must call", "package main\nfunc f() (int, error) { return 0, nil }\nfunc g() int { return f!() }\n", true},
		{"must func", "package main\nfunc f!() int { return 1 }\n", true},
		{"macro", "package main\nmacro M() {}\n", true},
		{"macro use", "package main\nfunc g() { #M() }\n", true},
		{"derive", "package main\n#Derive(Equal)\ntype T struct{}\n", true},
		{"plain", "package main\nfunc g(a, b int) bool { return a != b }\n", false},
		{"keyword not", "package main\nfunc g(a bool) { if!(a) {} }\n", false},
		{"text", "package main\n// F!(x) and #M() in a comment\nvar s = \"F!(x) #M()\"\n", false},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "x.gp"), []byte(tt.src), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := usesPhase10Syntax(&program{dir: dir, arg: "x.gp"}); got != tt.want {
			t.Errorf("%s: usesPhase10Syntax = %v, want %v", tt.name, got, tt.want)
		}
	}
	// A module: the syntax in a package below the folder counts, the syntax in twin/ does not.
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "pkg"), 0o755)
	os.MkdirAll(filepath.Join(dir, "twin"), 0o755)
	os.WriteFile(filepath.Join(dir, "main.gp"), []byte("package main\nfunc main() {}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "twin", "x.gp"), []byte("package main\nmacro M() {}\n"), 0o644)
	if usesPhase10Syntax(&program{dir: dir, arg: "."}) {
		t.Error("the syntax in twin/ counts")
	}
	os.WriteFile(filepath.Join(dir, "pkg", "p.gp"), []byte("package pkg\nfunc f!() int { return 1 }\n"), 0o644)
	if !usesPhase10Syntax(&program{dir: dir, arg: "."}) {
		t.Error("the syntax in a package of the module does not count")
	}
}

// TestRejectedProblem: a program that Go rejects is a failure of TestRunVsGo, not a skip, when it uses the syntax of
// Phase 10 and has no Go twin (nothing would be compared with real Go), or when it has a twin (the twin is plain Go);
// another program that Go rejects (C/C++ interop, a gocpp extension) is skipped.
func TestRejectedProblem(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("umacro_twinless.gp", "package main\nmacro M() {}\nfunc main() { #M() }\n")
	write("interop.gp", "package main\nimport \"<cstdio>\"\nfunc main() { cstdio.puts(\"hi\") }\n")
	write("umacro_twinned.gp", "package main\nmacro M() {}\nfunc main() { #M() }\n")
	write("testdata/macro_twin/umacro_twinned.go", "package main\nfunc main() {}\n")
	for _, tt := range []struct{ name, want string }{
		{"umacro_twinless", "uses the syntax of Phase 10, Go rejects it, and it has no Go twin"},
		{"interop", ""},
		{"umacro_twinned", "the Go twin of tests/run/umacro_twinned.gp does not compile"},
	} {
		p := &program{name: tt.name, path: "tests/run/" + tt.name + ".gp", dir: dir, arg: tt.name + ".gp"}
		got := rejectedProblem(p)
		if tt.want == "" && got != "" || !strings.Contains(got, tt.want) {
			t.Errorf("%s: rejectedProblem = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// verdict records what goBuildFailed decided (verdictT).
type verdict struct{ fatal, skip string }

func (v *verdict) Fatalf(format string, args ...any) { v.fatal = fmt.Sprintf(format, args...) }
func (v *verdict) Skipf(format string, args ...any)  { v.skip = fmt.Sprintf(format, args...) }

// TestGoBuildFailed: what runVsGoTest does with a program whose Go build fails: a Phase 10 program without a twin that
// Go rejects fails (nothing would be compared), a program that is not Go (interop) is skipped and counted, and a build
// that fails for another reason (no error at a line of the program) fails.
func TestGoBuildFailed(t *testing.T) {
	dir := t.TempDir()
	for name, src := range map[string]string{
		"umacro_twinless.gp": "package main\nmacro M() {}\nfunc main() { #M() }\n",
		"interop.gp":         "package main\nimport \"<cstdio>\"\nfunc main() { cstdio.puts(\"hi\") }\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name, out   string
		fatal, skip string
		rejected    bool
	}{
		{name: "umacro_twinless", out: "./umacro_twinless.go:2:1: syntax error: non-declaration statement outside function body\n",
			fatal: "uses the syntax of Phase 10, Go rejects it, and it has no Go twin"},
		{name: "interop", out: "./interop.go:2:8: invalid import path: \"<cstdio>\"\n",
			skip: "go build rejects the code of tests/run/interop.gp, so it is not a Go program", rejected: true},
		{name: "interop", out: "go: cannot find GOROOT directory\n", fatal: "and not because of an error in the program"},
	} {
		p := &program{name: tt.name, path: "tests/run/" + tt.name + ".gp", dir: dir, arg: tt.name + ".gp"}
		var v verdict
		var n vsGoCount
		goBuildFailed(&v, p, result{cmd: "go build", exit: 1, status: "exit status 1", stdout: []byte(tt.out)}, &n)
		if tt.fatal != "" && !strings.Contains(v.fatal, tt.fatal) || tt.fatal == "" && v.fatal != "" {
			t.Errorf("%s, %q: Fatalf %q, want %q", tt.name, tt.out, v.fatal, tt.fatal)
		}
		if tt.skip != "" && !strings.Contains(v.skip, tt.skip) || tt.skip == "" && v.skip != "" {
			t.Errorf("%s, %q: Skipf %q, want %q", tt.name, tt.out, v.skip, tt.skip)
		}
		if got := n.rejected.Load() == 1; got != tt.rejected {
			t.Errorf("%s, %q: counted as rejected: %v, want %v", tt.name, tt.out, got, tt.rejected)
		}
	}
}
