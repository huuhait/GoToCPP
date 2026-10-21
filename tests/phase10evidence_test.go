package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The tests of this file are the evidence for three exit criteria of Phase 10 (plans/2026-10-13-gocpp/
// phase-10-must-and-macros/STATE.md):
//
//   - "`must-errors` and `macros` examples pass on GCC and Clang": TestPhase10ExamplesBothCompilers.
//   - "No helper calls left in generated C++; no measurable overhead (incl. macros across packages)":
//     TestPhase10NoHelpers (phase10emit_test.go) says that no program names a hidden helper;
//     TestPhase10ExpansionIsInline says what the expansion of `!` and of a macro looks like in the C++
//     (control flow and code of the program's own units, no wrapper object);
//     TestPhase10BangEmitsHandWrittenCode compares the C++ of `!` with the C++ of the code a programmer
//     writes; TestPhase10MachineCodeEquivalence compares the machine code that g++ and clang++ make at
//     -release for a macro, `!` and a cross-package use with the machine code of the hand-written
//     program (no wall-clock time: docs/benchmarks/must-macros.md has the timings).
//   - "On-top macros: exit part runs on panic; Locked-style test race-clean; wrap macros behave as in
//     the spike, race-clean": tests/run/umacro_exit_panic_*.gp, umacro_wrap_exit_panic.gp and
//     umacro_wrap_spike.gp (each with a Go twin that real Go runs), tests/race/ok_macro_locked_panic.gp
//     and ok_macro_wrap_spike.gp (twins in tests/race/testdata/twin, judged by real Go's detector).
//
// What counts as a helper call (ADR-037, ADR-075): a call of the hidden packages gocpp/must and
// gocpp/macro (M1, M2, M3, Any, Site, Outer) or of a runtime wrapper that stands for `!` or a macro.
// `!` is lowered to the statements a programmer writes (a call, a test of the error, a return), and a
// macro is a generic function of its package that the unit of the caller instantiates: the C++
// compiler inlines it, its lazy arguments and its #body() are lambdas (no gp::func, no allocation).
// The one runtime function that stays is gp::must_exit at the end of `func main!()`.
//
// Scope: this holds for `!` and for the macros that are generic instances (lazy, wrap, generic). Not
// covered: a plain macro with no lazy parameter, no type parameter and no #body() is an ordinary
// function of its package and a real cross-package call (ADR-075), and the cold path of a lazy
// #Assert builds its panic message differently from hand-written code (only the hot loop is equal).
// The machine-code comparison sees one function per binary, because everything inlines into main:
// that also proves that no macro symbol remains. Its normalization hides the offset behind a
// rip-relative load (for example <_IO_stdin_used+0x4>); immediates, callees and jumps are kept.

// The names of the folders of the evidence programs, relative to the repository root.
const (
	evidenceDir = "tests/testdata/phase10"
	benchDir    = "bench/phase10"
)

// phase10EvidenceGate skips the test when the phase of the run is before the programs it builds.
func phase10EvidenceGate(t *testing.T, c *config) {
	t.Helper()
	if why := phaseGate(9, c.phase, c.phaseFrom); why != "" {
		t.Skip(why)
	}
}

// requireBothCompilers fails the test when the matrix is not g++ and clang++ (GOCPP_CXX narrows it to
// one compiler: the criterion names both, so such a run says nothing about it).
func requireBothCompilers(t *testing.T, c *config) {
	t.Helper()
	have := map[string]bool{}
	for _, cc := range c.compilers {
		have[cc.name] = true
	}
	if !have["g++"] || !have["clang++"] {
		t.Skipf("the matrix has %d compilers: this evidence needs g++ and clang++ (GOCPP_CXX is set?)", len(c.compilers))
	}
}

// TestPhase10ExamplesBothCompilers is the exit criterion "`must-errors` and `macros` examples pass on
// GCC and Clang": each example runs with g++ and with clang++ and prints output.txt (TestExamples runs
// them too, with whatever compilers the matrix has), and the example's twin, run by real Go in the
// example's folder, prints the same bytes, so output.txt is what Go does and not what gocpp prints.
func TestPhase10ExamplesBothCompilers(t *testing.T) {
	c := setup(t)
	requireBothCompilers(t, c)
	var progs []*program
	for _, name := range []string{"must-errors", "macros"} {
		rel := "examples/" + name
		dir := filepath.Join(repoRoot, rel)
		p := &program{name: name, path: rel + "/main.gp", dir: dir, arg: "."}
		if p.ex, p.err = loadExample(dir, rel); p.err == nil {
			p.header = p.ex.header
		}
		trackFiles(dir)
		progs = append(progs, p)
	}
	c.runPrograms(t, "Phase 10 examples", progs, func(t *testing.T, p *program) {
		t.Run("twin under real Go", func(t *testing.T) {
			c.exampleTwinMatches(t, p)
		})
		c.exampleTest(t, p)
	})
}

// exampleTwinMatches builds the twin of an example with real Go and runs it in the example's folder
// (where its config files are), like tools/gen-example-output: the output must be output.txt.
func (c *config) exampleTwinMatches(t *testing.T, p *program) {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go tool: %v", err)
	}
	prog := filepath.Join(t.TempDir(), "twin")
	if runtime.GOOS == "windows" {
		prog += ".exe"
	}
	build := job{
		dir:  filepath.Join(p.dir, "twin"),
		name: goTool,
		args: []string{"build", "-o", prog, "."},
		show: "cd " + filepath.ToSlash(p.path) + "/../twin && go build",
	}
	if r := c.run(t, build); r.problem != "" || r.exit != 0 {
		t.Fatalf("%s: %s\nstderr:\n%s", r.cmd, r.why(), indent(string(r.stderr)))
	}
	run := job{dir: p.dir, env: p.ex.env, name: prog, args: p.ex.args, show: "the twin of " + p.name}
	checkRun(t, c.run(t, run), expect{
		stdout:    string(p.ex.output),
		exact:     true,
		from:      strings.TrimSuffix(p.path, "main.gp") + "output.txt",
		exitCode:  p.header.exitCode,
		anyStderr: true,
	})
}

// emitFiles returns the C++ that `gocpp emit .` writes for the module in dir, by file: the files are
// separated by lines that start with "//// " and the path of the file.
func emitFiles(t *testing.T, c *config, dir string) map[string]string {
	t.Helper()
	r := c.run(t, gocppJob(dir, nil, "", 10, "emit", "."))
	if r.problem != "" || r.exit != 0 {
		t.Fatalf("%s: %s\nstderr:\n%s", r.cmd, r.why(), indent(string(r.stderr)))
	}
	return splitEmit(string(r.stdout))
}

// splitEmit splits the output of gocpp emit into its files.
func splitEmit(out string) map[string]string {
	files := map[string]string{}
	var name string
	var text strings.Builder
	flush := func() {
		if name != "" {
			files[name] = text.String()
		}
		text.Reset()
	}
	for _, line := range strings.Split(out, "\n") {
		if path, ok := strings.CutPrefix(line, "//// "); ok {
			flush()
			name = strings.TrimSpace(path)
			continue
		}
		text.WriteString(line)
		text.WriteByte('\n')
	}
	flush()
	return files
}

// userUnits returns the files of the emitted C++ that belong to the program and not to the standard
// library: the .gp.cpp and pkg.hpp files under the module path.
func userUnits(files map[string]string, modulePath string) map[string]string {
	units := map[string]string{}
	for name, text := range files {
		if strings.HasPrefix(name, "gpkg/"+modulePath+"/") {
			units[name] = text
		}
	}
	return units
}

// cppCodeLines returns the lines of C++ text without the #line directives, the comments that start a
// line, and blank lines.
func cppCodeLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#line") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// wrapperObjects are the C++ spellings of an object that stands between a macro and its code: a
// type-erased function value (what a lazy argument or #body() would cost if it were not a lambda) and
// an allocation. They are looked for in the parameters and arguments of a macro instance, not in its
// result: a start-style macro returns a func() that the Go program defers, a real func value.
var wrapperObjects = regexp.MustCompile(`gp::func<|std::function|std::make_shared`)

// mustHelpers are the runtime functions that stood for `!` in the first design.
var mustHelpers = regexp.MustCompile(`gp::must_[a-z]+`)

// mustExitCall is the only runtime function of `!` that stays in the C++: the end of `func main!()`.
var mustExitCall = regexp.MustCompile(`gp::must_exit\(`)

// instanceName finds the C++ names of the instances of macros (generic functions that the loader
// writes for them, ADR-075): gp_inst_<hash>_<Name>.
var instanceName = regexp.MustCompile(`\bgp_inst_[0-9a-f]+_\w+`)

// instanceDefinition finds the line that opens the body of a function: a line that starts at the
// first column, names the instance followed by its parameters, and ends with an opening brace.
func instanceDefinition(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^\S[^;\n]*\b` + regexp.QuoteMeta(name) + `\(.*\) \{$`)
}

// hiddenStringPlace finds a macro's hidden place argument spelled as a gp::string: a string with an owner
// built at every use (the arguments are gp::static_view, docs/benchmarks/must-macros.md).
var hiddenStringPlace = regexp.MustCompile(`gp::string gp_(?:file|func|text_\w*)_\b`)

// bangTest finds the test of the error that the lowering of `!` writes after the call.
var bangTest = regexp.MustCompile(`if \(must_e\d+ != nullptr\)`)

// inlineProblems lists what is wrong with the expansion of `!` and of macros in the C++ units of the
// program (see the comment at the top of the file). wantInstances says that the program has macro
// uses, and wantBang that it has `!`, so that an expansion that disappeared cannot pass for an inline
// one.
func inlineProblems(units map[string]string, wantInstances, wantBang bool) []string {
	var problems []string
	bodies := map[string]string{}
	for name, text := range units {
		var body []string
		for _, line := range cppCodeLines(text) {
			if !strings.HasPrefix(line, "#include") {
				body = append(body, line)
			}
		}
		bodies[name] = strings.Join(body, "\n")
	}
	instances := map[string]string{}
	bangTests := 0
	for name, joined := range bodies {
		if m := helperMarkers.FindString(joined); m != "" {
			problems = append(problems, fmt.Sprintf("%s: the C++ names the hidden helper %q", name, m))
		}

		// No wrapper object, and no runtime function of `!` except must_exit at the end of main!.
		withoutExit := mustExitCall.ReplaceAllString(joined, "")
		if m := mustHelpers.FindString(withoutExit); m != "" {
			problems = append(problems, fmt.Sprintf("%s: the expansion calls the runtime helper %q", name, m))
		}
		for _, line := range strings.Split(joined, "\n") {
			loc := instanceName.FindStringIndex(line)
			if loc == nil {
				continue
			}
			if m := wrapperObjects.FindString(line[loc[1]:]); m != "" {
				problems = append(problems, fmt.Sprintf("%s: a macro instance gets a wrapper object %q: %s", name, m, strings.TrimSpace(line)))
			}
		}

		for _, inst := range instanceName.FindAllString(joined, -1) {
			instances[inst] = name
		}
		if strings.Contains(joined, "gp_inst_") && hiddenStringPlace.MatchString(joined) {
			problems = append(problems, fmt.Sprintf("%s: a macro's hidden place argument is a gp::string", name))
		}
		bangTests += len(bangTest.FindAllString(joined, -1))
	}

	// The code of every macro instance is in a unit of the program, so that the C++ compiler sees it.
	for inst, user := range instances {
		defined := false
		for _, joined := range bodies {
			if instanceDefinition(inst).MatchString(joined) {
				defined = true
			}
		}
		if !defined {
			problems = append(problems, fmt.Sprintf("%s: the instance %s has no body in any unit of the program", user, inst))
		}
	}
	if wantInstances && len(instances) == 0 {
		problems = append(problems, "the program uses macros, but its C++ has no macro instance")
	}
	if wantBang && bangTests == 0 {
		problems = append(problems, "the program uses `!`, but its C++ has no `if (must_eN != nullptr)` test")
	}
	sort.Strings(problems)
	return problems
}

// TestPhase10ExpansionIsInline is the structural half of "no helper calls left in generated C++": for
// programs with `!` and macros, among them macros declared in one package and used in another, the C++
// of the program's own units has no helper of the hidden packages, no wrapper object, a test of the
// error as a plain `if` after each `!`, and the code of every macro instance in the units of the
// program (so that the C++ compiler inlines it across packages).
func TestPhase10ExpansionIsInline(t *testing.T) {
	c := setup(t)
	phase10EvidenceGate(t, c)
	cases := []struct {
		name, dir, module string
		instances, bangs  bool
	}{
		{"cross_wrap", evidenceDir + "/cross_wrap/macro", "example.com/evidence", true, true},
		{"inline", evidenceDir + "/inline", "example.com/evidence", true, true},
		{"bench cross", benchDir + "/cross/macro", "example.com/bench", true, false},
		{"bench wrap", benchDir + "/wrap/macro", "example.com/bench", true, false},
		{"bench must_func", benchDir + "/must_func/bang", "example.com/bench", false, true},
		{"macro_user_cross", "tests/pkgs/macro_user_cross", "", true, false},
		{"macro_sites", "tests/pkgs/macro_sites", "", true, false},
		{"must_cross", "tests/pkgs/must_cross", "", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(repoRoot, filepath.FromSlash(tc.dir))
			files := emitFiles(t, c, dir)
			module := tc.module
			if module == "" {
				module = moduleOf(t, dir)
			}
			units := userUnits(files, module)
			if len(units) == 0 {
				t.Fatalf("no unit of the module %s in the emitted C++ (files: %d)", module, len(files))
			}
			for _, problem := range inlineProblems(units, tc.instances, tc.bangs) {
				t.Error(problem)
			}
		})
	}
}

// moduleOf reads the module path from the gocpp.mod of dir.
func moduleOf(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "gocpp.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if path, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(path)
		}
	}
	t.Fatalf("%s/gocpp.mod has no module line", dir)
	return ""
}

// TestInlineProblemsFindWrappers tests the checks above on text: each sign of a helper or a
// wrapper object must be found, and the code the lowering writes must pass.
func TestInlineProblemsFindWrappers(t *testing.T) {
	passing := "namespace gpkg_main {\n" +
		"std::tuple<int64_t, gp::error> f(int64_t n) {\n" +
		"    auto gp_t0 = g(n);\n" +
		"    gp::error must_e1 = std::get<1>(gp_t0);\n" +
		"    if (must_e1 != nullptr) {\n" +
		"        return {0, must_e1};\n" +
		"    }\n" +
		"    return {0, gp::error()};\n" +
		"}\n" +
		"template <typename gp_BN>\n" +
		"void gp_inst_17f8234b_Tally(gp::static_view gp_file_, gp_BN gp_body_) {\n" +
		"    gp_body_();\n" +
		"}\n" +
		"void main() {\n" +
		"    gp::must_exit(must_main());\n" +
		"}\n" +
		"}\n"
	check := func(text string) bool {
		return len(inlineProblems(map[string]string{"x.gp.cpp": text}, true, true)) > 0
	}
	if check(passing) {
		t.Fatal("the code of the lowering is reported")
	}
	for name, bad := range map[string]string{
		"helper package": passing + "auto x = gpkg_gocpp_must::M1<int64_t>(g(n));\n",
		"gp::func param": passing + "void gp_inst_2_Y(gp::func<void()> gp_body_) {\n}\n",
		"gp::func arg":   passing + "gp_inst_2_Y(gp::func<void()>([&] {}));\n",
		"std::function":  passing + "void gp_inst_3_Z(std::function<void()> body) {\n}\n",
		"allocation":     passing + "gp_inst_4_W(std::make_shared<int>(1));\n",
		"must wrapper":   passing + "gp::must_value(g(n));\n",
		"string place":   passing + "void gp_inst_1_X(gp::string gp_file_) {\n}\n",
	} {
		if !check(bad) {
			t.Errorf("%s: not reported", name)
		}
	}
}

// bangForms are the renames that turn the names of the lowering of `!` into the names of the code that
// the hand variant writes: the error variable and the value variable of each form.
var bangRenames = []struct {
	pattern *regexp.Regexp
	to      string
}{
	{regexp.MustCompile(`\bmust_e\d+\b`), "err"},
	{regexp.MustCompile(`\bmust_t\d+\b`), "v"},
}

// normalizeBangCode returns the lines of the C++ of a module's main unit as comparable text: no #line
// lines, the names of the lowering renamed, no line that only opens or closes a block (the lowering
// puts the code of a `!` statement in a block of its own, a programmer's code is flat), and no indent.
func normalizeBangCode(text string) []string {
	var lines []string
	for _, line := range cppCodeLines(text) {
		for _, r := range bangRenames {
			line = r.pattern.ReplaceAllString(line, r.to)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "{" || trimmed == "}" {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}

// mainUnit returns the text of the file of the main package of an emitted module.
func mainUnit(t *testing.T, files map[string]string, modulePath string) string {
	t.Helper()
	name := "gpkg/" + modulePath + "/main.gp.cpp"
	text, ok := files[name]
	if !ok {
		t.Fatalf("the emitted C++ has no %s", name)
	}
	return text
}

// firstLineDifference describes where two lists of lines differ, or returns "".
func firstLineDifference(got, want []string) string {
	for i := 0; i < len(got) || i < len(want); i++ {
		var g, w string
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		if g != w {
			return fmt.Sprintf("line %d: %q, want %q", i+1, g, w)
		}
	}
	return ""
}

// TestPhase10BangEmitsHandWrittenCode is the half of "no measurable overhead" for `!` that can be
// said without a clock: the C++ that gocpp writes for a program with `!` is, up to the names of the
// lowering and the block around a `!` statement, the C++ it writes for the program with the code written
// by hand. The programs are bench/phase10/must_call (the benchmark's two variants) and
// tests/testdata/phase10/bang_forms (a define, an assignment, a statement and two values).
func TestPhase10BangEmitsHandWrittenCode(t *testing.T) {
	c := setup(t)
	phase10EvidenceGate(t, c)
	for _, tc := range []struct{ name, bang, hand string }{
		{"must_call", benchDir + "/must_call/bang", benchDir + "/must_call/hand"},
		{"bang_forms", evidenceDir + "/bang_forms/bang", evidenceDir + "/bang_forms/hand"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bangFiles := emitFiles(t, c, filepath.Join(repoRoot, filepath.FromSlash(tc.bang)))
			handFiles := emitFiles(t, c, filepath.Join(repoRoot, filepath.FromSlash(tc.hand)))
			module := "example.com/bench"
			if strings.HasPrefix(tc.bang, evidenceDir) {
				module = "example.com/evidence"
			}
			got := normalizeBangCode(mainUnit(t, bangFiles, module))
			want := normalizeBangCode(mainUnit(t, handFiles, module))
			if len(got) < 10 {
				t.Fatalf("the main unit has only %d lines of code", len(got))
			}
			if diff := firstLineDifference(got, want); diff != "" {
				t.Errorf("the C++ of `!` is not the C++ of the hand-written code: %s", diff)
			}
		})
	}
}

// TestNormalizeBangCode tests the comparison above: the names and the blocks of the lowering do not
// count, a different call does.
func TestNormalizeBangCode(t *testing.T) {
	bang := "    {\n#line 4 \"x.gp\"\n        auto gp_t0 = step(i);\n        int64_t must_t1 = std::get<0>(gp_t0);\n" +
		"        gp::error must_e2 = std::get<1>(gp_t0);\n        if (must_e2 != nullptr) {\n" +
		"            return {0, must_e2};\n        }\n    }\n"
	hand := "auto gp_t0 = step(i);\nint64_t v = std::get<0>(gp_t0);\ngp::error err = std::get<1>(gp_t0);\n" +
		"if (err != nullptr) {\nreturn {0, err};\n}\n"
	if diff := firstLineDifference(normalizeBangCode(bang), normalizeBangCode(hand)); diff != "" {
		t.Errorf("the same code differs: %s", diff)
	}
	other := strings.Replace(hand, "step(i)", "step2(i)", 1)
	if diff := firstLineDifference(normalizeBangCode(bang), normalizeBangCode(other)); diff == "" {
		t.Error("a different callee is not found")
	}
	dropped := strings.Replace(hand, "return {0, err};", "return {1, err};", 1)
	if diff := firstLineDifference(normalizeBangCode(bang), normalizeBangCode(dropped)); diff == "" {
		t.Error("a different return is not found")
	}
}

// userSymbols matches the symbols of the program's own code in nm's demangled names: the namespaces of
// package main and of the packages of the evidence and benchmark modules.
var userSymbols = regexp.MustCompile(`gpkg_main::|gpkg_example_dcom__(?:evidence|bench)`)

// A textSymbol is a function of the program in the binary.
type textSymbol struct {
	name string
	addr uint64
	size uint64
}

// parseTextSymbols reads `nm -C -S --defined-only` output (address, size, type, name) and returns the
// functions of the program's own code.
func parseTextSymbols(out string) []textSymbol {
	var syms []textSymbol
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		kind := fields[2]
		if kind != "t" && kind != "T" && kind != "w" && kind != "W" {
			continue
		}
		name := strings.Join(fields[3:], " ")
		if !userSymbols.MatchString(name) {
			continue
		}
		addr, err1 := strconv.ParseUint(fields[0], 16, 64)
		size, err2 := strconv.ParseUint(fields[1], 16, 64)
		if err1 != nil || err2 != nil || size == 0 {
			continue
		}
		syms = append(syms, textSymbol{name, addr, size})
	}
	return syms
}

var (
	disasmAddressColon = regexp.MustCompile(`^\s*[0-9a-f]+:\s*`)
	disasmComment      = regexp.MustCompile(`\s+#\s+[0-9a-f]+ <([^>+]*)(?:\+0x[0-9a-f]+)?>`)
	disasmTarget       = regexp.MustCompile(`\s[0-9a-f]{3,} <`)
	disasmRipOffset    = regexp.MustCompile(`0x[0-9a-f]+\(%rip\)`)
	disasmSpaces       = regexp.MustCompile(`\s+`)
)

// normalizeDisassembly turns the lines of objdump's disassembly of one function into comparable
// instructions: no addresses, no raw bytes, the target of a jump or a call as <symbol+offset> (the
// offset inside the function is part of the code, the absolute address is not), the displacement of a
// rip-relative operand and the address in its comment gone (the symbol it names stays).
func normalizeDisassembly(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if !disasmAddressColon.MatchString(line) {
			continue
		}
		line = disasmAddressColon.ReplaceAllString(line, "")
		line = disasmComment.ReplaceAllString(line, " # <$1>")
		line = disasmTarget.ReplaceAllString(line, " <")
		line = disasmRipOffset.ReplaceAllString(line, "X(%rip)")
		line = strings.TrimSpace(disasmSpaces.ReplaceAllString(line, " "))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// disassembleUserCode returns, for each function of the program's own code in the binary, its
// normalized instructions.
func disassembleUserCode(t *testing.T, c *config, binary string) map[string][]string {
	t.Helper()
	dir := filepath.Dir(binary)
	nm := c.run(t, job{dir: dir, name: "nm", args: []string{"-C", "-S", "--defined-only", binary}, show: "nm " + binary})
	if nm.problem != "" || nm.exit != 0 {
		t.Fatalf("%s: %s\nstderr:\n%s", nm.cmd, nm.why(), indent(string(nm.stderr)))
	}
	code := map[string][]string{}
	for _, sym := range parseTextSymbols(string(nm.stdout)) {
		args := []string{
			"-d", "-C", "--no-show-raw-insn",
			"--start-address=" + fmt.Sprintf("0x%x", sym.addr),
			"--stop-address=" + fmt.Sprintf("0x%x", sym.addr+sym.size),
			binary,
		}
		dump := c.run(t, job{dir: dir, name: "objdump", args: args, show: "objdump " + sym.name})
		if dump.problem != "" || dump.exit != 0 {
			t.Fatalf("%s: %s\nstderr:\n%s", dump.cmd, dump.why(), indent(string(dump.stderr)))
		}
		code[sym.name] = normalizeDisassembly(string(dump.stdout))
	}
	return code
}

// buildRelease builds the module in dir with gocpp build -release and the compiler, and returns the
// program.
func buildRelease(t *testing.T, c *config, dir string, cc compiler) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "prog")
	args := []string{"build", "-release", "-o", out, "."}
	r := c.run(t, gocppJob(dir, nil, cc.cxx, 10, args...))
	if r.problem != "" || r.exit != 0 {
		t.Fatalf("%s: %s\noutput:\n%s\n%s", r.cmd, r.why(), indent(string(r.stdout)), indent(string(r.stderr)))
	}
	return out
}

// TestPhase10MachineCodeEquivalence is "no measurable overhead" without a clock: g++ and clang++ at
// -release make the same machine code for the program with the feature and for the program with the
// code written by hand, in every function of the program (and the same set of functions: nothing of the
// macro is left as a function). The pairs: a wrap macro on a function (bench/phase10/wrap), a must call
// in a loop (bench/phase10/must_call), and a wrap macro and a must call of another package
// (tests/testdata/phase10/cross_wrap). The timings of docs/benchmarks/must-macros.md are the other
// half (a lazy `#Assert` message is not here: its cold path prints the place and the text of the
// condition, which the hand-written program spells differently, so only the hot loop is equal).
func TestPhase10MachineCodeEquivalence(t *testing.T) {
	c := setup(t)
	phase10EvidenceGate(t, c)
	requireBothCompilers(t, c)
	if runtime.GOOS != "linux" {
		t.Skip("the disassembly is read from ELF binaries (nm, objdump)")
	}
	for _, tool := range []string{"nm", "objdump"} {
		if _, err := exec.LookPath(tool); err != nil {
			if os.Getenv("GOCPP_REQUIRE_CXX") != "" {
				t.Fatalf("%s is not installed (it comes with the dev shell): %v", tool, err)
			}
			t.Skipf("%s is not installed: %v", tool, err)
		}
	}
	pairs := []struct{ name, feature, hand string }{
		{"wrap", benchDir + "/wrap/macro", benchDir + "/wrap/hand"},
		{"must_call", benchDir + "/must_call/bang", benchDir + "/must_call/hand"},
		{"cross_wrap", evidenceDir + "/cross_wrap/macro", evidenceDir + "/cross_wrap/hand"},
	}
	for _, pair := range pairs {
		t.Run(pair.name, func(t *testing.T) {
			c.forEachCompiler(t, func(t *testing.T, cc compiler) {
				t.Parallel()
				feature := disassembleUserCode(t, c, buildRelease(t, c, filepath.Join(repoRoot, filepath.FromSlash(pair.feature)), cc))
				hand := disassembleUserCode(t, c, buildRelease(t, c, filepath.Join(repoRoot, filepath.FromSlash(pair.hand)), cc))
				if len(feature) == 0 {
					t.Fatal("no function of the program in the binary")
				}
				instructions := 0
				for _, code := range feature {
					instructions += len(code)
				}
				t.Logf("%d functions, %d instructions compared", len(feature), instructions)
				for name, code := range feature {
					want, ok := hand[name]
					if !ok {
						t.Errorf("%s is in the program with the feature and not in the hand-written one", name)
						continue
					}
					if diff := firstLineDifference(code, want); diff != "" {
						t.Errorf("%s: the machine code differs from the hand-written program's: %s", name, diff)
					}
				}
				for name := range hand {
					if _, ok := feature[name]; !ok {
						t.Errorf("%s is in the hand-written program and not in the one with the feature", name)
					}
				}
			})
		})
	}
}

// TestNormalizeDisassembly tests the normalization above: addresses and rip displacements do not
// count, the instruction, the callee and the offset of a jump do.
func TestNormalizeDisassembly(t *testing.T) {
	at := func(addr, callee, data string) string {
		return "0000000000" + addr + " <gpkg_main::main()>:\n" +
			"   " + addr + ":\tpush   %rbp\n" +
			"   " + addr + ":\tlea    0x2c5e(%rip),%rdi        # 28004 <" + data + "+0x4>\n" +
			"   " + addr + ":\tcall   a1b00 <" + callee + ">\n" +
			"   " + addr + ":\tje     26145 <gpkg_main::main()+0x145>\n"
	}
	first := normalizeDisassembly(at("26000", "gp::fmt::Println()", "_IO_stdin_used"))
	moved := normalizeDisassembly(strings.ReplaceAll(strings.ReplaceAll(
		at("46000", "gp::fmt::Println()", "_IO_stdin_used"), "a1b00", "c9900"), "26145", "46145"))
	if diff := firstLineDifference(first, moved); diff != "" {
		t.Errorf("the same code at other addresses differs: %s", diff)
	}
	if len(first) != 4 {
		t.Fatalf("got %d instructions, want 4: %q", len(first), first)
	}
	if diff := firstLineDifference(first, normalizeDisassembly(at("26000", "gp::fmt::Print()", "_IO_stdin_used"))); diff == "" {
		t.Error("another callee is not found")
	}
	if diff := firstLineDifference(first, normalizeDisassembly(at("26000", "gp::fmt::Println()", "other_data"))); diff == "" {
		t.Error("another data symbol is not found")
	}
	if diff := firstLineDifference(first, normalizeDisassembly(strings.Replace(
		at("26000", "gp::fmt::Println()", "_IO_stdin_used"), "+0x145", "+0x14a", 1))); diff == "" {
		t.Error("another jump offset is not found")
	}
}

// TestParseTextSymbols tests the filter of nm's lines: the functions of the program's namespaces, not
// data, not the standard library's functions.
func TestParseTextSymbols(t *testing.T) {
	out := strings.Join([]string{
		"0000000000026000 0000000000000a2b t gpkg_main::main()",
		"00000000001374a8 0000000000000008 b gpkg_main::tally",
		"0000000000026b00 0000000000000100 t gpkg_io__fs::walk()",
		"0000000000026c00 0000000000000040 t gp::string gpkg_main::run(long)::$_0::operator()<1>()",
		"0000000000026d00 0000000000000000 t gpkg_main::empty()",
		"0000000000026e00 0000000000000010 W gpkg_example_dcom__evidence__util::Step(long, long)",
	}, "\n")
	var names []string
	for _, sym := range parseTextSymbols(out) {
		names = append(names, sym.name)
	}
	want := []string{
		"gpkg_main::main()",
		"gp::string gpkg_main::run(long)::$_0::operator()<1>()",
		"gpkg_example_dcom__evidence__util::Step(long, long)",
	}
	if strings.Join(names, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", names, want)
	}
}
