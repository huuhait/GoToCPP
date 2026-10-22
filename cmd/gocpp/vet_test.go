package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"

	"github.com/huuhait/gocpp/internal/vet"
)

// vetIn writes the files into a new folder (with a gocpp.mod) and runs
// gocpp vet with args there.
func vetIn(t *testing.T, files map[string]string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	all := map[string]string{"gocpp.mod": "module example.com/m\n"}
	for k, v := range files {
		all[k] = v
	}
	writeFiles(t, dir, all)
	t.Chdir(dir)
	code, stdout, stderr = gocpp(append([]string{"vet"}, args...)...)
	return code, stdout, strings.ReplaceAll(stderr, "\\", "/")
}

const vetBad = `package main

import "fmt"

func main() {
	fmt.Printf("%d\n", "x")
	var f func()
	f = func() { f() }
	f()
}
`

func TestVetClean(t *testing.T) {
	code, out, errOut := vetIn(t, map[string]string{"main.gp": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(1) }\n"}, ".")
	if code != 0 || out != "" || errOut != "" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}

func TestVetFindings(t *testing.T) {
	code, out, errOut := vetIn(t, map[string]string{"main.gp": vetBad}, ".")
	if code != 1 || out != "" {
		t.Errorf("exit %d, stdout %q; want exit 1 and nothing on stdout", code, out)
	}
	lines := strings.Split(strings.TrimSpace(errOut), "\n")
	if len(lines) < 3 || lines[0] != "# example.com/m" && lines[0] != "# command-line-arguments" {
		t.Fatalf("stderr:\n%s", errOut)
	}
	if want := `./main.gp:6:14: fmt.Printf format %d has arg "x" of wrong type string`; lines[1] != want {
		t.Errorf("line 1 = %q, want %q", lines[1], want)
	}
	if !strings.HasPrefix(lines[2], "./main.gp:8:2: reference cycle: f holds a closure that captures f") {
		t.Errorf("line 2 = %q", lines[2])
	}
}

func TestVetSelectAnalyzers(t *testing.T) {
	files := map[string]string{"main.gp": vetBad}
	_, _, errOut := vetIn(t, files, "-printf=false", ".")
	if strings.Contains(errOut, "Printf") || !strings.Contains(errOut, "reference cycle") {
		t.Errorf("-printf=false:\n%s", errOut)
	}
	_, _, errOut = vetIn(t, files, "-printf", ".")
	if !strings.Contains(errOut, "Printf") || strings.Contains(errOut, "reference cycle") {
		t.Errorf("-printf:\n%s", errOut)
	}
	code, _, errOut := vetIn(t, files, "-printf=false", "-cycle=false", ".")
	if code != 0 || errOut != "" {
		t.Errorf("both off: exit %d, %q", code, errOut)
	}
}

func TestVetJSON(t *testing.T) {
	code, out, errOut := vetIn(t, map[string]string{"main.gp": vetBad}, "-json", ".")
	if code != 0 || errOut != "" {
		t.Errorf("exit %d, stderr %q; -json exits with 0", code, errOut)
	}
	var got map[string]map[string][]struct {
		Posn    string `json:"posn"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(got) != 1 {
		t.Fatalf("packages: %v", got)
	}
	for _, byAnalyzer := range got {
		if len(byAnalyzer["printf"]) != 1 || byAnalyzer["printf"][0].Posn != "./main.gp:6:14" {
			t.Errorf("printf: %+v", byAnalyzer["printf"])
		}
		if len(byAnalyzer["cycle"]) != 1 {
			t.Errorf("cycle: %+v", byAnalyzer["cycle"])
		}
	}
}

func TestVetList(t *testing.T) {
	code, out, errOut := gocpp("vet", "-list")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	for _, name := range []string{"printf", "copylocks", "unusedresult", "nilness", "cycle", "reserved", "panicbang", "gobang", "unwaitedbody", "unusedlazy", "ignorederr", "cpplocal", "cstr", "narrowing", "tests", "unsafeptr"} {
		if !regexp.MustCompile(`(?m)^` + name + ` `).MatchString(out) {
			t.Errorf("-list has no %s:\n%s", name, out)
		}
	}
	if strings.Contains(out, "placeholder") {
		t.Errorf("an analyzer is still a placeholder:\n%s", out)
	}
}

func TestVetLoadError(t *testing.T) {
	code, out, errOut := vetIn(t, map[string]string{"main.gp": "package main\n\nfunc main() { undefined() }\n"}, ".")
	if code != 2 || out != "" || !strings.Contains(errOut, "undefined: undefined") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, _, errOut = vetIn(t, map[string]string{"main.gp": "package main\n\nfunc main( {\n"}, ".")
	if code != 2 || !strings.Contains(errOut, "main.gp") {
		t.Errorf("syntax error: exit %d, stderr %q", code, errOut)
	}
	code, _, errOut = vetIn(t, nil, "./nonesuch")
	if code != 2 || errOut == "" {
		t.Errorf("no package: exit %d, stderr %q", code, errOut)
	}
}

func TestVetBadFlag(t *testing.T) {
	code, out, errOut := gocpp("vet", "-nonesuch")
	if code != 2 || out != "" || !strings.Contains(errOut, "flag provided but not defined: -nonesuch") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, _, errOut = gocpp("vet", "-printf=maybe")
	if code != 2 || errOut == "" {
		t.Errorf("-printf=maybe: exit %d, stderr %q", code, errOut)
	}
}

func TestVetHelp(t *testing.T) {
	code, _, errOut := gocpp("vet", "-h")
	if code != 0 || !strings.Contains(errOut, "usage: gocpp vet") || !strings.Contains(errOut, "-printf") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestVetCrossPackageWrapper(t *testing.T) {
	code, _, errOut := vetIn(t, map[string]string{
		"lib/lib.gp": "package lib\n\nimport \"fmt\"\n\nfunc Logf(format string, args ...any) { fmt.Println(fmt.Sprintf(format, args...)) }\n",
		"main.gp":    "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() { lib.Logf(\"%d\", \"x\") }\n",
	}, ".")
	if code != 1 || !strings.Contains(errOut, `./main.gp:5:25: example.com/m/lib.Logf format %d has arg "x" of wrong type string`) {
		t.Errorf("exit %d, stderr:\n%s", code, errOut)
	}
}

// Only the packages named are reported, not the ones they import.
func TestVetReportsRootsOnly(t *testing.T) {
	files := map[string]string{
		"lib/lib.gp": "package lib\n\nimport \"fmt\"\n\nfunc F() { fmt.Printf(\"%d\", \"x\") }\n",
		"main.gp":    "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() { lib.F() }\n",
	}
	code, out, errOut := vetIn(t, files, ".")
	if code != 0 || out != "" || errOut != "" {
		t.Errorf("main: exit %d, %q, %q", code, out, errOut)
	}
	code, _, errOut = vetIn(t, files, "./lib")
	if code != 1 || !strings.Contains(errOut, "# example.com/m/lib") || !strings.Contains(errOut, "lib/lib.gp:5:") {
		t.Errorf("lib: exit %d, %q", code, errOut)
	}
}

func TestVetTestFiles(t *testing.T) {
	code, _, errOut := vetIn(t, map[string]string{
		"main.gp":      "package main\n\nfunc main() {}\n",
		"main_test.gp": "package main\n\nimport \"testing\"\n\nfunc Testlower(t *testing.T) {}\n",
	}, ".")
	if code != 1 || !strings.Contains(errOut, "main_test.gp:5:6: Testlower has malformed name") {
		t.Errorf("exit %d, stderr:\n%s", code, errOut)
	}
}

func TestVetCurrentFolderByDefault(t *testing.T) {
	code, _, errOut := vetIn(t, map[string]string{"main.gp": vetBad})
	if code != 1 || !strings.Contains(errOut, "Printf") {
		t.Errorf("exit %d, stderr:\n%s", code, errOut)
	}
}

func TestVetOutputIsSorted(t *testing.T) {
	files := map[string]string{
		"b/b.gp": "package b\n\nimport \"fmt\"\n\nfunc F() { fmt.Printf(\"%d\", \"b\") }\n",
		"a/a.gp": "package a\n\nimport \"fmt\"\n\nfunc F() { fmt.Printf(\"%d\", \"a\") }\n",
	}
	_, _, first := vetIn(t, files, "./b", "./a")
	for range 5 {
		_, _, again := vetIn(t, files, "./b", "./a")
		if again != first {
			t.Fatalf("the output changed:\n%s\n%s", first, again)
		}
	}
	// The packages come in the order they were named.
	if strings.Index(first, "# example.com/m/b") > strings.Index(first, "# example.com/m/a") {
		t.Errorf("order:\n%s", first)
	}
}

func TestVetOpaqueCode(t *testing.T) {
	// A package with C++ names: no crash, the code beside them is vetted.
	src := "package main\n\nimport (\n\t\"fmt\"\n\n\t\"<vector>\"\n)\n\nfunc main() {\n\tv := std.vector[int]{1}\n\tfmt.Printf(\"%d %v\\n\", v.size(), v)\n\tfmt.Printf(\"%d\\n\", \"x\")\n}\n"
	code, _, errOut := vetIn(t, map[string]string{"main.gp": src}, ".")
	if _, err := exec.LookPath("g++"); err != nil {
		if _, err := exec.LookPath("clang++"); err != nil {
			t.Skip("no C++ compiler to look at <vector>")
		}
	}
	if code != 1 || strings.Count(errOut, "Printf") != 1 || !strings.Contains(errOut, `main.gp:12:`) {
		t.Errorf("exit %d, stderr:\n%s", code, errOut)
	}
	if strings.Contains(errOut, "gocpp bug") {
		t.Errorf("an analyzer failed:\n%s", errOut)
	}
}

func TestVetVerboseSkips(t *testing.T) {
	src := "package main\n\nimport (\n\t\"fmt\"\n\n\t\"<vector>\"\n)\n\nfunc main() {\n\tv := std.vector[int]{}\n\tfmt.Println(v.size())\n\tfmt.Printf(\"%d\\n\", v.size())\n}\n"
	if _, err := exec.LookPath("g++"); err != nil {
		if _, err := exec.LookPath("clang++"); err != nil {
			t.Skip("no C++ compiler")
		}
	}
	files := map[string]string{"main.gp": src}
	code, _, errOut := vetIn(t, files, "-v", ".")
	if code != 0 {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
	// nilness leaves out main (it declares a C++ value); printf leaves out
	// the declaration of v and checks the calls with their C++ values
	// replaced.
	for _, want := range []string{
		"analyzer nilness skipped: 1 function left out, because they use C/C++ values (at ./main.gp:9:1)",
		"analyzer printf skipped: 1 statement left out, because they use C/C++ values; 2 statements checked with the C/C++ values in calls replaced by unknown ones (at ./main.gp:10:2)",
	} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errOut)
		}
	}
	// Without -v the notes are quiet.
	_, _, errOut = vetIn(t, files, ".")
	if strings.Contains(errOut, "skipped") {
		t.Errorf("without -v: %s", errOut)
	}
}

func TestVetCrashedAnalyzerExits2(t *testing.T) {
	boom := &vet.Analyzer{Analyzer: &analysis.Analyzer{
		Name: "boom",
		Doc:  "panics",
		Run:  func(*analysis.Pass) (any, error) { panic("boom") },
	}}
	saved := vet.Analyzers
	vet.Analyzers = append(slices.Clone(saved), boom)
	t.Cleanup(func() { vet.Analyzers = saved })
	// With a finding too: the crash still decides.
	code, _, errOut := vetIn(t, map[string]string{"main.gp": vetBad}, ".")
	if code != 2 || !strings.Contains(errOut, "analyzer boom failed on package example.com/m") || !strings.Contains(errOut, "Printf") {
		t.Errorf("exit %d, stderr:\n%s", code, errOut)
	}
	code, _, errOut = vetIn(t, map[string]string{"main.gp": "package main\n\nfunc main() {}\n"}, "-boom", ".")
	if code != 2 {
		t.Errorf("clean program: exit %d, stderr:\n%s", code, errOut)
	}
	code, _, _ = vetIn(t, map[string]string{"main.gp": "package main\n\nfunc main() {}\n"}, "-boom=false", ".")
	if code != 0 {
		t.Errorf("without the analyzer: exit %d", code)
	}
}

// TestVetExamples runs gocpp vet over the examples of the repository that
// this phase can build: no crash, no finding.
func TestVetExamples(t *testing.T) {
	root := filepath.Join("..", "..", "examples")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir, _ := filepath.Abs(filepath.Join(root, e.Name()))
		src, err := os.ReadFile(filepath.Join(dir, "main.gp"))
		if err != nil {
			continue
		}
		header := string(src)
		if i := strings.Index(header, "\npackage "); i > 0 {
			header = header[:i]
		}
		phase := regexp.MustCompile(`(?m)^// phase: (\d+)`).FindStringSubmatch(header)
		if phase == nil || phase[1] > "05" {
			continue // a feature of a later phase
		}
		lib := regexp.MustCompile(`(?m)^// pkg-config: (.*)$`).FindStringSubmatch(header)
		t.Run(e.Name(), func(t *testing.T) {
			if lib != nil {
				for _, l := range strings.Fields(lib[1]) {
					if exec.Command("pkg-config", "--exists", l).Run() != nil {
						t.Skipf("needs %s", l)
					}
				}
			}
			t.Chdir(dir)
			code, out, errOut := gocpp("vet", ".")
			if code != 0 || out != "" || errOut != "" {
				t.Errorf("exit %d, stdout %q, stderr:\n%s", code, out, errOut)
			}
		})
	}
}

const vetBang = `package lib

import (
	"os"
	"strconv"
)

func Port(s string) int {
	return strconv.Atoi!(s)
}

macro Skip(a lazy int, b lazy int) int {
	return a()
}

func Clean(paths []string) error {
	for _, p := range paths {
		os.Remove(p)
	}
	return nil
}

func Spawn!(p string) {
	go os.Remove(p)
}
`

// The analyzers about ! and macros: a panic in a library, a lazy parameter nobody calls, a
// dropped error, a go statement that drops the error of a must function. The syntax of Phase 10
// needs no switch (ADR-095).
func TestVetBangAndMacros(t *testing.T) {
	code, out, errOut := vetIn(t, map[string]string{"lib/lib.gp": vetBang}, "./lib")
	if code != 1 || out != "" {
		t.Fatalf("exit %d, stdout %q, stderr:\n%s", code, out, errOut)
	}
	for _, want := range []string{
		"# example.com/m/lib",
		"lib/lib.gp:9:9: strconv.Atoi!(s) panics here: Port has no error result",
		"lib/lib.gp:12:24: the lazy parameter b of macro Skip is never used",
		"lib/lib.gp:18:3: the error result of os.Remove is dropped, in a function that returns an error: handle it, write os.Remove!(...) to return it",
		"lib/lib.gp:24:2: go os.Remove(...) in the must function Spawn: the error that it returns is dropped",
	} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errOut)
		}
	}
	// -<analyzer> picks, -<analyzer>=false leaves out.
	_, _, errOut = vetIn(t, map[string]string{"lib/lib.gp": vetBang}, "-panicbang=false", "-gobang=false", "./lib")
	if strings.Contains(errOut, "panics here") || strings.Contains(errOut, "go os.Remove") || !strings.Contains(errOut, "never used") {
		t.Errorf("-panicbang=false -gobang=false:\n%s", errOut)
	}
	_, out, errOut = vetIn(t, map[string]string{"lib/lib.gp": vetBang}, "-json", "-unusedlazy", "./lib")
	if errOut != "" || !strings.Contains(out, `"unusedlazy"`) || strings.Contains(out, "panicbang") {
		t.Errorf("-json -unusedlazy: stdout %q, stderr %q", out, errOut)
	}
}

// A program without the syntax of Phase 10 has nothing for these analyzers to find, and the
// analyzers of the C/C++ boundary work in any phase.
func TestVetBoundary(t *testing.T) {
	const src = `package main

import "<cstdio>"

type holder struct{ p *cstdio.FILE }

func f(n int64, h *holder) {
	var list []*cstdio.FILE
	for i := 0; i < 1; i++ {
		var fp cstdio.FILE
		list = append(list, &fp)
	}
	_ = list
	cstdio.puts(C.cstr("a\x00b"))
	cstdio.printf("%d\n", C.int(n))
}

func main() { f(1, &holder{}) }
`
	code, _, errOut := vetIn(t, map[string]string{"main.gp": src}, ".")
	if code != 1 {
		t.Fatalf("exit %d, stderr:\n%s", code, errOut)
	}
	for _, want := range []string{
		"./main.gp:11:23: &fp takes the address of the C/C++ variable fp and stores it in list, which is declared outside the block of fp",
		"./main.gp:14:14: the string given to C.cstr has a NUL byte",
		"./main.gp:15:24: C.int(n) narrows int64 (64 bits) to C.int (32 bits)",
	} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr lacks %q:\n%s", want, errOut)
		}
	}
}

// The project of the Phase 13 exit criterion: a closure cycle in the program, a printf mistake
// through a wrapper of another package (a wrapper of a wrapper), and a panicking ! in a library,
// also through the library that it imports.
var vetCriterionFiles = map[string]string{
	"logx/logx.gp": `package logx

import "fmt"

// Infof forwards to fmt.Printf, so vet checks its callers like Printf's.
func Infof(format string, args ...any) {
	fmt.Printf(format, args...)
}

// Warnf forwards to Infof: a wrapper of a wrapper.
func Warnf(format string, args ...any) {
	Infof(format, args...)
}
`,
	"parse/parse.gp": `package parse

import "strconv"

// Port panics with the error of Atoi: it has no error result.
func Port(text string) int {
	return strconv.Atoi!(text)
}
`,
	"server/server.gp": `package server

import "example.com/m/parse"

// Listen is a library function that calls a panicking function of another package.
func Listen(address string) int {
	return parse.Port(address)
}
`,
	"main.gp": `package main

import "example.com/m/logx"

func main() {
	logx.Warnf("port %d", "eighty")
	var retry func()
	retry = func() { retry() }
	retry()
}
`,
}

// A cycle, a printf mistake through two wrappers of another package, and a panicking ! in a
// library are each found, at the .gp line, by gocpp vet over the whole project: vet runs once
// and each finding is checked on its own.
func TestVetPhase13ExitCriterion(t *testing.T) {
	code, out, errOut := vetIn(t, vetCriterionFiles, "./...")
	lines := strings.Split(strings.TrimSpace(errOut), "\n")
	t.Run("exit status", func(t *testing.T) {
		if code != 1 || out != "" {
			t.Errorf("exit %d, stdout %q; want exit 1 and nothing on stdout\n%s", code, out, errOut)
		}
	})
	findings := []struct {
		name string
		line string
	}{
		{"closure cycle", "./main.gp:8:2: reference cycle: retry holds a closure that captures retry"},
		{"printf through two wrappers", `./main.gp:6:19: example.com/m/logx.Warnf format %d has arg "eighty" of wrong type string`},
		{"panicking bang in a library", "parse/parse.gp:7:9: strconv.Atoi!(text) panics here: Port has no error result"},
		{"panicking bang through the imported library", "server/server.gp:7:9: parse.Port can panic: parse.Port uses ! at parse.gp:7:9"},
	}
	for _, f := range findings {
		t.Run(f.name, func(t *testing.T) {
			if !slices.ContainsFunc(lines, func(l string) bool { return strings.HasPrefix(l, f.line) }) {
				t.Errorf("no line starts with %q:\n%s", f.line, errOut)
			}
		})
	}
	t.Run("nothing else", func(t *testing.T) {
		// The four findings and the three "# package" lines.
		if len(lines) != len(findings)+3 {
			t.Errorf("%d lines, want %d:\n%s", len(lines), len(findings)+3, errOut)
		}
	})
}

// The same project, mended: the call is right, the closure does not hold itself, and the library
// returns its errors (a must function: its callers get the error). Nothing is reported.
func TestVetPhase13ExitCriterionMended(t *testing.T) {
	files := map[string]string{
		"logx/logx.gp": vetCriterionFiles["logx/logx.gp"],
		"parse/parse.gp": `package parse

import "strconv"

func Port!(text string) int {
	return strconv.Atoi!(text)
}
`,
		"server/server.gp": `package server

import "example.com/m/parse"

func Listen!(address string) int {
	return parse.Port!(address)
}
`,
		"main.gp": `package main

import "example.com/m/logx"

// retry is a top-level function: nothing captures it.
func retry(attempts int) {
	if attempts > 0 {
		retry(attempts - 1)
	}
}

func main() {
	logx.Warnf("port %d", 80)
	retry(3)
}
`,
	}
	code, out, errOut := vetIn(t, files, "./...")
	if code != 0 || out != "" || errOut != "" {
		t.Errorf("exit %d, stdout %q, stderr:\n%s", code, out, errOut)
	}
}
