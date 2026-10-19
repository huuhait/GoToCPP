package check

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/version"
	"github.com/huuhait/gocpp/stdlib"
)

var updateGates = flag.Bool("update-gates", false, "rewrite docs/spec/feature-matrix.md and testdata/gates/examples.txt")

// checkAt loads the patterns in dir with the standard library stubs (for
// linux/amd64 unless cfg says otherwise) and checks them with the gates of
// the phase. It returns what gocpp check would print, with paths relative
// to dir.
func checkAt(t *testing.T, cfg load.Config, dir string, phase int, patterns ...string) string {
	t.Helper()
	cfg.Dir, cfg.Stdlib, cfg.GoStd = dir, stdlib.FS(), stdlib.IsStd
	if cfg.GOOS == "" {
		cfg.GOOS, cfg.GOARCH = "linux", "amd64"
	}
	prog := load.Load(cfg, patterns...)
	return errorText(t, prog, Check(prog, fakeProbeConfig(phase)), dir, true)
}

// A gateCase is a small program that uses a gated feature.
type gateCase struct {
	name  string
	phase int               // the phase of the feature: the program is rejected before it
	files map[string]string // the files of the program; "main.gp" alone if src is set
	src   string            // main.gp
	tests bool              // load with the _test.gp files

	// before is the phase for want, if it is not phase-1 (or LastPhase
	// for a feature that never comes).
	before int

	// want are the errors before the phase (at phase-1, or at
	// LastPhase for a feature that never comes), each "line:col: msg" in
	// main.gp, or "file:line:col: msg" with a slash in the file.
	want []string

	// wantAt are the errors at the phase and at LastPhase (none if nil).
	wantAt []string
}

func TestGates(t *testing.T) {
	for _, tc := range gateCases {
		t.Run(tc.name, func(t *testing.T) {
			files := tc.files
			if files == nil {
				files = map[string]string{"main.gp": tc.src}
			}
			dir := writeTree(t, files)
			before := tc.phase - 1
			switch {
			case tc.before != 0:
				before = tc.before
			case tc.phase == Never:
				before = LastPhase
			}
			cfg := load.Config{Tests: tc.tests}
			if got, want := checkAt(t, cfg, dir, before, "."), errorLines(tc.want); got != want {
				t.Errorf("at Phase %02d:\n got:\n%s\nwant:\n%s", before, got, want)
			}
			if tc.phase == Never {
				return
			}
			for _, phase := range []int{tc.phase, LastPhase} {
				if got, want := checkAt(t, cfg, dir, phase, "."), errorLines(tc.wantAt); got != want {
					t.Errorf("at Phase %02d:\n got:\n%s\nwant:\n%s", phase, got, want)
				}
			}
		})
	}
}

// errorLines returns what errorText prints for the errors of a gateCase:
// a "# package" line (command-line-arguments if the first error is not
// one), then the errors. An error that starts with "./" is printed as it
// is, the others are in ./main.gp.
func errorLines(errs []string) string {
	if len(errs) == 0 {
		return ""
	}
	var b strings.Builder
	if !strings.HasPrefix(errs[0], "# ") {
		b.WriteString("# command-line-arguments\n")
	}
	for _, e := range errs {
		if !strings.HasPrefix(e, "# ") && !strings.HasPrefix(e, "./") {
			e = "./main.gp:" + e
		}
		b.WriteString(e + "\n")
	}
	return b.String()
}

// main wraps the body in a program.
func main_(body string) string {
	return "package main\n\nfunc main() {\n" + body + "}\n"
}

var gateCases = []gateCase{
	// Phase 02: the exemptions.
	{name: "phase02", phase: 2, src: `package main

import "fmt"

const big = 1 << 40

func pair(a, b int) (int, bool) { return a + b, a > b }

func main() {
	n, ok := pair(1, 2)
	s := "a" + "b"
	fmt.Println("x", n, ok, 2.5, big, s[0])
	fmt.Print(n)
	fmt.Printf("%d\n", n)
	println(len(s), s < "c", float64(n)/2, uint8(n), s[1])
	for i := range 3 {
		_ = i
	}
	if n > 10 {
		panic("too big")
	}
	panic(n)
}
`},

	{name: "fmt/nil", phase: 5, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(nil)\n}\n",
		want: []string{"6:14: any values are not supported yet (Phase 05)"}},
	{name: "fmt/result as argument", phase: 5, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(fmt.Println(\"x\"))\n}\n",
		want: []string{"6:14: error values are not supported yet (Phase 05)"}},
	{name: "fmt/blank results", phase: 5, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\t_, _ = fmt.Println()\n}\n",
		want: []string{"6:9: error values are not supported yet (Phase 05)"}},
	{name: "fmt/one result", phase: 5, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tn, _ := fmt.Println()\n\tprintln(n)\n}\n",
		want: []string{"6:5: error values are not supported yet (Phase 05)"}},
	{name: "fmt/function value", phase: 5,
		before: 2, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tf := fmt.Println\n\tf()\n}\n",
		want: []string{"6:2: function values are not supported yet (Phase 03)"}},
	{name: "fmt/function value at 03", phase: 5, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tf := fmt.Println\n\tf()\n}\n",
		before: 3,
		want:   []string{"6:2: any values are not supported yet (Phase 05)"}},
	{name: "format/dot at the end", phase: 3, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printf(\"%5.\")\n\tfmt.Printf(\"%.\")\n}\n",
		before: 2,
		want:   []string{"6:13: width, precision and flags in format strings are not supported yet (Phase 03)"}},
	{name: "format/verbs of Phase 02", phase: 2, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printf(\"%v %d %s %t %f %% %!\\n\", 1, 2, \"s\", true, 1.5)\n}\n"},
	{name: "format/width", phase: 3, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printf(\"%5d|%-3s|%.2f\\n\", 1, \"s\", 1.5)\n}\n",
		want: []string{"6:13: width, precision and flags in format strings are not supported yet (Phase 03)"}},
	{name: "format/%q", phase: 3, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printf(\"%q\\n\", \"s\")\n}\n",
		want: []string{"6:13: the format verb %q is not supported yet (Phase 03)"}},
	{name: "format/%T", phase: 5, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printf(\"%T\\n\", 1)\n}\n",
		want: []string{"6:13: the format verb %T is not supported yet (Phase 05)"}},
	{name: "format/%x", phase: 5, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printf(\"%x\\n\", 255)\n}\n",
		want: []string{"6:13: the format verb %x is not supported yet (Phase 05)"}},
	{name: "format/%#v", phase: 5, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printf(\"%#v\\n\", 1)\n}\n",
		before: 3,
		want:   []string{"6:13: the # flag, * and argument indexes in format strings are not supported yet (Phase 05)"}},
	{name: "format/index", phase: 5, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printf(\"%[1]d\\n\", 1)\n}\n",
		want: []string{"6:13: the # flag, * and argument indexes in format strings are not supported yet (Phase 05)"}},
	{name: "format/not a constant", phase: 5, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tf := \"%d\\n\"\n\tfmt.Printf(f, 1)\n}\n",
		want: []string{"7:13: format strings that are not constants are not supported yet (Phase 05)"}},

	// Phase 03.
	{name: "composite/array", phase: 3, src: main_("\tvar a [3]int\n\t_ = a\n"),
		want: []string{"4:6: composite types are not supported yet (Phase 03)"}},
	{name: "composite/literal", phase: 3, src: main_("\tprintln(len([]string{\"a\"}))\n"),
		want: []string{"4:14: composite types are not supported yet (Phase 03)"}},
	{name: "composite/map", phase: 3, src: "package main\n\nfunc f(m map[string]int) {}\n\nfunc main() {}\n",
		want: []string{"3:8: composite types are not supported yet (Phase 03)"}},
	{name: "pointers", phase: 3, src: main_("\tx := 1\n\tp := &x\n\tprintln(*p)\n"),
		want: []string{"5:2: pointers are not supported yet (Phase 03)"}},
	{name: "methods", phase: 3, src: "package main\n\ntype T int\n\nfunc (T) M() {}\n\nfunc main() {}\n",
		want: []string{
			"3:6: type declarations are not supported yet (Phase 03)",
			"5:10: methods are not supported yet (Phase 03)",
		}},
	{name: "function values", phase: 3, src: "package main\n\nfunc g() {}\n\nfunc main() {\n\tf := g\n\tf()\n}\n",
		want: []string{"6:2: function values are not supported yet (Phase 03)"}},
	{name: "closures", phase: 3, src: main_("\tfunc() {}()\n"),
		want: []string{"4:2: closures are not supported yet (Phase 03)"}},
	{name: "defer", phase: 3, src: main_("\tdefer println(\"x\")\n"),
		want: []string{"4:2: defer is not supported yet (Phase 03)"}},
	{name: "type declarations", phase: 3, src: "package main\n\ntype Celsius float64\n\nfunc main() {\n\tvar c Celsius\n\tprintln(c)\n}\n",
		want: []string{"3:6: type declarations are not supported yet (Phase 03)"}},
	{name: "type aliases", phase: 3, src: "package main\n\ntype A = int\n\nfunc main() {\n\tvar a A\n\tprintln(a)\n}\n",
		want: []string{"3:6: type aliases are not supported yet (Phase 03)"}},
	{name: "string slicing", phase: 3, src: main_("\ts := \"ab\"\n\tprintln(s[0], s[1:])\n"),
		want: []string{"5:16: string slicing is not supported yet (Phase 03)"}},
	{name: "range over strings", phase: 3, src: main_("\tfor range \"ab\" {\n\t}\n"),
		want: []string{"4:2: range over strings is not supported yet (Phase 03)"}},
	{name: "string conversions", phase: 3, src: main_("\tprintln(string(rune(65)))\n"),
		want: []string{"4:10: conversions between strings and bytes, runes or integers are not supported yet (Phase 03)"}},
	{name: "complex numbers", phase: 3, src: main_("\tc := 1i\n\tprintln(c)\n"),
		want: []string{"4:2: complex numbers are not supported yet (Phase 03)"}},
	{name: "builtin min", phase: 3, src: main_("\tprintln(min(1, 2), max(3, 4))\n"),
		want: []string{
			"4:10: built-in function min is not supported yet (Phase 03)",
			"4:21: built-in function max is not supported yet (Phase 03)",
		}},
	{name: "variadic functions", phase: 3, src: "package main\n\nfunc f(xs ...int) {}\n\nfunc main() {\n\tf(1, 2)\n}\n",
		want: []string{
			"3:8: composite types are not supported yet (Phase 03)",
			"3:11: variadic functions are not supported yet (Phase 03)",
		}},
	{name: "package-level variables", phase: 3, src: "package main\n\nvar n = 1\n\nfunc main() {\n\tprintln(n)\n}\n",
		want: []string{"3:1: package-level variables are not supported yet (Phase 03)"}},
	{name: "init functions", phase: 3, src: "package main\n\nfunc init() {}\n\nfunc main() {}\n",
		want: []string{"3:6: init functions are not supported yet (Phase 03)"}},
	{name: "fmt.Sprintf", phase: 3, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tprintln(fmt.Sprintf(\"%d\", 1))\n}\n",
		want: []string{"6:14: fmt.Sprintf is not available yet (Phase 03)"}},
	{name: "fmt.Sprintf/dot import", phase: 3, src: "package main\n\nimport . \"fmt\"\n\nfunc main() {\n\tprintln(Sprint(1))\n\tPrintln()\n}\n",
		want: []string{"6:10: fmt.Sprint is not available yet (Phase 03)"}},
	{name: "weak", phase: 3, src: "package main\n\nimport \"weak\"\n\nfunc main() {\n\tvar w weak.Pointer[int]\n\tprintln(w.Value() == nil)\n}\n",
		want: []string{"3:8: package weak is not available yet (Phase 03)", "7:10: pointers are not supported yet (Phase 03)"}},
	{name: "weak/defined type", phase: 7, before: 3,
		src:  "package main\n\nimport \"weak\"\n\ntype WP weak.Pointer[int]\n\ntype A = weak.Pointer[int]\n\nfunc main() {\n\tvar a A\n\tprintln(a.Value() == nil, WP(a) == WP{})\n}\n",
		want: []string{"5:6: a type defined as weak.Pointer[T] is not supported yet (Phase 07)"}},

	// Phase 04.
	{name: "header imports", phase: 4, src: "package main\n\nimport \"<stdio.h>\"\n\nfunc main() {\n\tstdio.puts(\"x\")\n}\n",
		want: []string{"3:8: header imports are not supported yet (Phase 04)"}},
	// A module header is a header import of Phase 04 too: at the same place, the first gate wins.
	{name: "module headers", phase: 4, files: map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"main.gp":    "package main\n\nimport \"example.com/m/native/x.h\"\n\nfunc main() {\n\tx.f()\n}\n",
		"native/x.h": "void f();\n",
	},
		want: []string{"# example.com/m", "3:8: header imports are not supported yet (Phase 04)"}},
	{name: "namespaces", phase: 4, src: "package main\n\nimport \"<vector>\"\n\nfunc main() {\n\tv := std.vector[int]{}\n\tv.push_back(1)\n}\n",
		want: []string{
			"3:8: header imports are not supported yet (Phase 04)",
			"6:7: C++ namespaces are not supported yet (Phase 04)",
		}},
	{name: "namespaces/rename", phase: 4, src: "package main\n\nimport (\n\t\"<filesystem>\"\n\tfs \"::std::filesystem\"\n)\n\nfunc main() {\n\tfs.remove(\"x\")\n}\n",
		want: []string{
			"4:2: header imports are not supported yet (Phase 04)",
			"5:5: C++ namespaces are not supported yet (Phase 04)",
		}},
	{name: "C types", phase: 4, src: main_("\tvar n C.int\n\t_ = n\n"),
		want: []string{"4:8: C types and helpers are not supported yet (Phase 04)"}},
	{name: "directives", phase: 4, src: "// Command x.\n//\n//gocpp:link m\npackage main\n\nfunc main() {}\n",
		want: []string{"3:1: //gocpp: directives are not supported yet (Phase 04)"}},
	{name: "functions without a body", phase: 4, src: "package main\n\nfunc add(a, b int) int\n\nfunc main() {\n\tprintln(add(1, 2))\n}\n",
		want: []string{"3:6: functions without a body are not supported yet (Phase 04)"}},
	{name: "unsafe.Sizeof", phase: 3, src: "package main\n\nimport \"unsafe\"\n\nfunc main() {\n\tprintln(unsafe.Sizeof(1))\n}\n",
		want: []string{"3:8: package unsafe is not available yet (Phase 03)"}},
	{name: "unsafe.Sizeof/dot import", phase: 3, src: "package main\n\nimport . \"unsafe\"\n\nfunc main() {\n\tprintln(Sizeof(1))\n}\n",
		want: []string{"3:10: package unsafe is not available yet (Phase 03)"}},
	{name: "unsafe.Add", phase: 4, src: "package main\n\nimport . \"unsafe\"\n\nfunc main() {\n\t_ = Add(nil, 1)\n}\n",
		before: 3,
		want:   []string{"6:6: unsafe.Add is not available yet (Phase 04)"}},
	{name: "unsafe.Pointer", phase: 4, src: "package main\n\nimport \"unsafe\"\n\nfunc main() {\n\tvar p unsafe.Pointer\n\t_ = p\n}\n",
		before: 3,
		want:   []string{"6:15: unsafe.Pointer is not available yet (Phase 04)"}},

	// Phase 05.
	{name: "interfaces", phase: 5, src: "package main\n\ntype S interface{ M() }\n\nfunc main() {}\n",
		want: []string{"3:8: interfaces are not supported yet (Phase 05)"}},
	{name: "any values", phase: 5, src: main_("\tvar x any = 1\n\t_ = x\n"),
		want: []string{"4:6: any values are not supported yet (Phase 05)"}},
	{name: "error values", phase: 5, src: "package main\n\nfunc f() error { return nil }\n\nfunc main() {\n\tf()\n}\n",
		want: []string{"3:10: error values are not supported yet (Phase 05)"}},
	{name: "error results of fmt", phase: 5, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\t_, err := fmt.Println()\n\t_ = err\n}\n",
		want: []string{"6:5: error values are not supported yet (Phase 05)"}},
	{name: "type assertions", phase: 5, src: main_("\tvar x interface{} = 1\n\t_ = x.(int)\n"),
		want: []string{
			"4:6: any values are not supported yet (Phase 05)",
			"5:6: type assertions are not supported yet (Phase 05)",
		}},
	{name: "type switches", phase: 5, src: main_("\tvar x any\n\tswitch x.(type) {\n\t}\n"),
		want: []string{
			"4:6: any values are not supported yet (Phase 05)",
			"5:2: type switches are not supported yet (Phase 05)",
		}},
	{name: "struct embedding", phase: 5, src: "package main\n\ntype A struct{ n int }\n\ntype B struct{ A }\n\nfunc main() {\n\tvar b B\n\tprintln(b.n)\n}\n",
		want: []string{"5:16: struct embedding is not supported yet (Phase 05)"}},
	{name: "recover", phase: 5, src: main_("\tdefer func() { recover() }()\n"),
		want: []string{"4:17: built-in function recover is not supported yet (Phase 05)"}},
	{name: "panic with a pointer", phase: 5, src: main_("\tvar p *int\n\tpanic(p)\n"),
		want: []string{"5:8: panic with a value of a non-basic type is not supported yet (Phase 05)"}},
	{name: "errors", phase: 5, src: "package main\n\nimport \"errors\"\n\nfunc main() {\n\t_ = errors.New(\"x\")\n}\n",
		want: []string{
			"3:8: package errors is not available yet (Phase 05)",
			"6:6: error values are not supported yet (Phase 05)",
		}},
	{name: "fmt.Errorf", phase: 5, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tprintln(fmt.Errorf(\"x\") == nil)\n}\n",
		want: []string{
			"6:10: error values are not supported yet (Phase 05)",
			"6:14: fmt.Errorf is not available yet (Phase 05)",
		}},
	// A real package since step 9.2: the import is the only error before Phase 09.
	{name: "runtime/cgo", phase: 9, src: "package main\n\nimport \"runtime/cgo\"\n\nfunc main() {\n\tcgo.NewHandle(1).Delete()\n}\n",
		want: []string{"3:8: package runtime/cgo is not available yet (Phase 09)"}},

	{name: "runtime", phase: 8, src: "package main\n\nimport \"runtime\"\n\nfunc main() {\n\truntime.Gosched()\n}\n",
		want: []string{"3:8: package runtime is not available yet (Phase 08)"}},
	{name: "runtime/debug", phase: 8, src: "package main\n\nimport \"runtime/debug\"\n\nfunc main() {\n\tdebug.FreeOSMemory()\n}\n",
		want: []string{"3:8: package runtime/debug is not available yet (Phase 08)"}},
	{name: "embedded C++ type/typo", phase: 5, src: "package main\n\ntype V struct {\n\tFooo\n\tn int\n}\n\nfunc main() {\n\tvar v V\n\tv.x()\n\tv.y = 1\n}\n",
		want: []string{
			"4:2: undefined: Fooo",
			"4:2: struct embedding is not supported yet (Phase 05)",
			"10:4: v.x undefined (type V has no field or method x)",
			"11:4: v.y undefined (type V has no field or method y)",
		},
		wantAt: []string{
			"4:2: undefined: Fooo",
			"10:4: v.x undefined (type V has no field or method x)",
			"11:4: v.y undefined (type V has no field or method y)",
		}},

	// Phase 06.
	{name: "more than one package", phase: 6, before: 4, files: map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"main.gp":    "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() {\n\tprintln(lib.N)\n}\n",
		"lib/lib.gp": "package lib\n\nconst N = 1\n",
	},
		want: []string{"# example.com/m", "3:8: programs of more than one package are not supported yet (Phase 06)"}},
	{name: "export", phase: 6, src: "package main\n\n//export F\nfunc F() {}\n\nfunc main() {}\n",
		want: []string{"3:1: //export functions are not supported yet (Phase 06)"}},

	// Phase 07.
	{name: "generics", phase: 7, src: "package main\n\nfunc F[T any](x T) T { return x }\n\nfunc main() {\n\tprintln(F(1))\n}\n",
		want: []string{"3:7: generic functions are not supported yet (Phase 07)"}},
	{name: "generics/type sets", phase: 7, src: "package main\n\ntype N interface{ ~int | ~int64 }\n\nfunc main() {}\n",
		want: []string{"3:8: generic functions are not supported yet (Phase 07)"}},
	{name: "generics/types", phase: 7, src: "package main\n\ntype S[T any] struct{ v T }\n\nfunc main() {\n\tvar s S[int]\n\t_ = s\n}\n",
		want: []string{"3:7: generic types are not supported yet (Phase 07)"}},
	{name: "range over functions", phase: 7, src: "package main\n\nfunc seq(yield func(int) bool) {}\n\nfunc main() {\n\tfor x := range seq {\n\t\tprintln(x)\n\t}\n}\n",
		want: []string{"6:2: range over functions is not supported yet (Phase 07)"}},
	{name: "slices", phase: 7, src: "package main\n\nimport \"slices\"\n\nfunc main() {\n\tprintln(slices.Contains([]int{1}, 1))\n}\n",
		want: []string{
			"3:8: package slices is not available yet (Phase 07)",
			"6:17: generic functions are not supported yet (Phase 07)",
		}},

	// Phase 08.
	{name: "go", phase: 8, src: main_("\tgo println()\n"),
		want: []string{"4:2: go statements are not supported yet (Phase 08)"}},
	{name: "channels", phase: 8, src: main_("\tc := make(chan int, 1)\n\tc <- 1\n\tclose(c)\n"),
		want: []string{
			"4:2: channels are not supported yet (Phase 08)",
			"6:2: built-in function close is not supported yet (Phase 08)",
		}},
	{name: "select", phase: 8, src: main_("\tselect {}\n"),
		want: []string{"4:2: select statements are not supported yet (Phase 08)"}},
	{name: "sync", phase: 8, src: "package main\n\nimport \"sync\"\n\nfunc main() {\n\tvar mu sync.Mutex\n\tmu.Lock()\n}\n",
		want: []string{"3:8: package sync is not available yet (Phase 08)"}},

	{name: "iter.Pull", phase: 8, src: "package main\n\nimport \"iter\"\n\nfunc main() {\n\t_ = iter.Pull[int]\n}\n",
		before: 7,
		want:   []string{"6:11: iter.Pull is not available yet (Phase 08)"}},

	// Phase 09.
	{name: "strings", phase: 9, src: "package main\n\nimport \"strings\"\n\nfunc main() {\n\tprintln(strings.ToUpper(\"x\"))\n}\n",
		want: []string{"3:8: package strings is not available yet (Phase 09)"}},
	{name: "fmt.Sscan", phase: 9, src: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tvar n int\n\tfmt.Sscan(\"1\", &n)\n}\n",
		want: []string{"7:6: fmt.Sscan is not available yet (Phase 09)"}},
	{name: "test files", phase: 9, tests: true, files: map[string]string{
		"main.gp":      "package main\n\nfunc main() {}\n",
		"main_test.gp": "package main\n\nfunc helper() {}\n",
	},
		want: []string{"# command-line-arguments [command-line-arguments.test]", "./main_test.gp:1:1: test files are not supported yet (Phase 09)"}},

	{name: "embed", phase: 10, src: "package main\n\nimport _ \"embed\"\n\nfunc main() {}\n",
		want:   []string{"3:8: package embed is not available yet (Phase 10)"},
		wantAt: []string{"3:8: package embed is not available yet (Phase 09)"}},

	// Phase 10.
	{name: "must calls", phase: 10, src: "package main\n\nfunc f() (int, error) { return 0, nil }\n\nfunc main() {\n\tx := f!()\n\tprintln(x)\n\ty := f!()\n}\n",
		want:   []string{"6:8: must calls are not supported yet (Phase 10)"},
		wantAt: []string{"6:8: expected ';', found '!'", "9:3: expected '}', found 'EOF'"}},
	{name: "must functions", phase: 10, src: "package main\n\nfunc run!() {}\n\ntype T int\n\nfunc (t *T) Start!() {}\n\nfunc main() {}\n",
		want: []string{"3:9: must functions are not supported yet (Phase 10)"},
		wantAt: []string{
			"3:9: expected '(', found '!'",
			"5:1: expected ')', found 'type'",
			"7:18: missing ',' in parameter list",
			"9:6: expected '(', found main",
		}},
	{name: "must calls/generic", phase: 10, src: main_("\tx := parse![int](\"1\")\n\t_ = x\n"),
		want:   []string{"4:12: must calls are not supported yet (Phase 10)"},
		wantAt: []string{"4:12: expected ';', found '!'", "6:3: expected '}', found 'EOF'"}},
	{name: "macros", phase: 10, src: "package main\n\nmacro Twice(x int) int {\n\treturn x * 2\n}\n\nfunc main() {\n\tprintln(#Twice(2))\n}\n",
		want:   []string{"3:1: macros are not supported yet (Phase 10)"},
		wantAt: []string{"3:1: expected declaration, found macro", "8:10: illegal character U+0023 '#'"}},
	{name: "macros/use", phase: 10, src: main_("\t#Dbg(1)\n"),
		want:   []string{"4:2: macros are not supported yet (Phase 10)"},
		wantAt: []string{"4:2: illegal character U+0023 '#'", "5:3: expected '}', found 'EOF'"}},
	{name: "go:embed", phase: 10, src: "package main\n\n//go:embed hello.txt\nvar s string\n\nfunc main() {\n\tprintln(s)\n}\n",
		want: []string{"3:1: //go:embed is not supported yet (Phase 10)"}},
	{name: "errors.Trace", phase: 10, src: "package main\n\nimport \"errors\"\n\nfunc f() (err error) {\n\terrors.Trace(err)\n\treturn\n}\n\nfunc main() {\n\tf()\n}\n",
		want:   []string{"6:9: errors.Trace is not available yet (Phase 10)"},
		wantAt: []string{"6:9: undefined: errors.Trace"}},

	// Phase 12.
	{name: "C macros as constants", phase: 12, src: "package main\n\nimport \"<stdio.h>\"\n\nconst n = stdio.BUFSIZ\n\nfunc main() {\n\tvar buf [stdio.BUFSIZ]byte\n\t_ = buf\n}\n",
		want: []string{"5:11: C macros in constant expressions are not supported yet (Phase 12)"}},
	{name: "promoted C++ members", phase: 12, src: "package main\n\nimport \"<string>\"\nimport \"<vector>\"\n\ntype V struct {\n\tstd.vector[int]\n\tstd.string\n}\n\nfunc main() {\n\tvar v V\n\tv.push_back(1)\n\tv.vector.push_back(2)\n}\n",
		want:   []string{"13:4: promoted fields and methods of several embedded C/C++ types are not supported yet (Phase 12)"},
		wantAt: []string{"13:4: v.push_back undefined (type V has no field or method push_back)"}},

	// Phase 14.
	{name: "C.wstr", phase: 14, src: main_("\tp := C.wstr(\"x\")\n\t_ = p\n"),
		want: []string{"4:7: C.wstr and C.GoStringW are not supported yet (Phase 14)"}},
	{name: "no-win32-defaults", phase: 14, src: "// Command x.\n//\n//gocpp:no-win32-defaults\npackage main\n\nfunc main() {}\n",
		want: []string{"3:1: //gocpp:no-win32-defaults is not supported yet (Phase 14)"}},

	// Never.
	{name: "go:linkname", phase: Never, src: "package main\n\nimport _ \"unsafe\"\n\n//go:linkname now time.now\nfunc now() int\n\nfunc main() {}\n",
		want: []string{"5:1: //go:linkname is not supported in gocpp"}},
	{name: "net/http", phase: Never, src: "package main\n\nimport \"net/http\"\n\nfunc main() {}\n",
		want: []string{"3:8: package net/http is not supported in gocpp"}},
	{name: "go/ast", phase: Never, src: "package main\n\nimport \"go/ast\"\n\nfunc main() {}\n",
		want: []string{"3:8: package go/ast is not supported in gocpp"}},
	{name: "plugin", phase: Never, src: "package main\n\nimport \"plugin\"\n\nfunc main() {}\n",
		want: []string{"3:8: package plugin is not supported in gocpp"}},
}

// TestGateTables checks the tables: the rows of features are in the
// order of featureID, the names are unique, and the phases are phases.
func TestGateTables(t *testing.T) {
	names := make(map[string]bool)
	validPhase := func(p int) bool { return p == Never || floorPhase <= p && p <= LastPhase }
	for i, f := range features {
		if f.id != featureID(i) {
			t.Errorf("features[%d] has the id %d", i, f.id)
		}
		if f.Name == "" || names[f.Name] || f.Detect == "" {
			t.Errorf("features[%d]: empty or repeated name %q, or no Detect", i, f.Name)
		}
		names[f.Name] = true
		if !validPhase(f.Phase) {
			t.Errorf("%s: phase %d", f.Name, f.Phase)
		}
	}
	paths := make(map[string]bool)
	for _, p := range stdPackages {
		if paths[p.Path] || !validPhase(p.Phase) || p.Path != "unsafe" && !stdlib.IsStd(strings.TrimSuffix(p.Path, "/...")) && !stdlib.IsStd(strings.Replace(p.Path, "...", "ast", 1)) {
			t.Errorf("stdPackages: %s (Phase %d) repeated, not a standard package or a bad phase", p.Path, p.Phase)
		}
		paths[p.Path] = true
	}
	for _, m := range slices.Concat(stdMembers, builtins) {
		if !validPhase(m.Phase) {
			t.Errorf("%s.%s: phase %d", m.Pkg, m.Name, m.Phase)
		}
		if m.Pkg != "" && lookupStdPackage(m.Pkg) == nil {
			t.Errorf("%s.%s: package not in stdPackages", m.Pkg, m.Name)
		}
	}
}

// TestStdPackagesMatchStubs checks that every package with a stub is in
// stdPackages, with the phase that the stub says replaces it.
func TestStdPackagesMatchStubs(t *testing.T) {
	// The phase that replaces the stub, or that maps it to a compiler intrinsic (weak).
	re := regexp.MustCompile(`// (?:type-check\. Phase (\d\d) replaces it|\S+ \(a Phase (\d\d) intrinsic\))`)
	n := 0
	err := fs.WalkDir(stdlib.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".gp") {
			return err
		}
		src, err := fs.ReadFile(stdlib.FS(), path)
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(filepath.Dir(path))
		if !bytes.Contains(src, []byte("// Code generated by tools/stubgen")) {
			// A real package (errors, since step 5.7): not a stub, but still a row of stdPackages.
			// The internal packages of the standard library (step 9.2) are not importable by a
			// program, so they have no row.
			if !isInternalPath(pkg) && lookupStdPackage(pkg) == nil {
				t.Errorf("%s is not in stdPackages", pkg)
			}
			return nil
		}
		m := re.FindSubmatch(src)
		if m == nil {
			t.Errorf("%s: no \"Phase NN replaces it\" line", path)
			return nil
		}
		n++
		stub, _ := strconv.Atoi(string(m[1]) + string(m[2])) // one of them is empty
		row := lookupStdPackage(pkg)
		switch {
		case row == nil:
			t.Errorf("%s is not in stdPackages", pkg)
		case pkg == "fmt":
			// fmt.Print and friends come in Phase 02 (the stand-in fmt
			// of the runtime); the stub's phase is the basic fmt's.
			if m := lookupMember("fmt", "Errorf"); m.Phase != stub {
				t.Errorf("fmt.Errorf has Phase %02d, the stub says %02d", m.Phase, stub)
			}
		case pkg == "os":
			// os.Stdout and os.Stderr, the writers of fmt.Fprint, come in Phase 05.
			if m := lookupMember("os", "Stdout"); m.Phase != 5 {
				t.Errorf("os.Stdout has Phase %02d, want 05", m.Phase)
			}
		case row.Phase != stub:
			t.Errorf("%s has Phase %02d in stdPackages, the stub says %02d", pkg, row.Phase, stub)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 22 { // 24 until unicode/utf8 (step 9.1) and time (step 8.8) became real packages
		t.Errorf("found %d stubs, want at least 22", n)
	}
}

// TestStubsNotGated checks a program that imports every package with a
// stub: at Phase 02, the errors are only at the imports of main.gp, and no
// standard package has errors.
func TestStubsNotGated(t *testing.T) {
	var imports []string
	for _, p := range stdPackages {
		// A folder with a package: runtime/cgo makes the folder runtime, which has no files.
		if files, _ := fs.Glob(stdlib.FS(), p.Path+"/*.gp"); len(files) > 0 {
			imports = append(imports, strconv.Quote(p.Path))
		}
	}
	src := "package main\n\nimport (\n"
	for _, imp := range imports {
		src += "\t_ " + imp + "\n"
	}
	src += ")\n\nfunc main() {}\n"
	dir := writeTree(t, map[string]string{"main.gp": src})
	cfg := load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}
	prog := load.Load(cfg, ".")
	r := Check(prog, fakeProbeConfig(floorPhase))
	std := 0
	for _, p := range r.Packages {
		if !p.Load.Std {
			continue
		}
		std++
		if !p.Checked() || len(p.Errors.Diagnostics()) > 0 {
			t.Errorf("%s: checked %v, errors:\n%s", p.Load.ImportPath, p.Checked(), errorText(t, prog, r, dir, true))
		}
	}
	if std < len(imports) {
		t.Errorf("%d standard packages, want at least %d", std, len(imports))
	}
	got := mainPkg(t, prog, r).Errors.Diagnostics()
	if len(got) != len(imports)-1 { // all but fmt
		t.Errorf("got %d errors, want %d:\n%s", len(got), len(imports)-1, errorText(t, prog, r, dir, true))
	}
	for _, d := range got {
		if !strings.HasSuffix(d.Pos.Filename, "main.gp") || !strings.Contains(d.Msg, "is not available yet") {
			t.Errorf("unexpected error %s: %s", d.Pos, d.Msg)
		}
	}
}

// isInternalPath reports whether the import path has an element "internal".
func isInternalPath(path string) bool {
	return slices.Contains(strings.Split(path, "/"), "internal")
}

func TestConfigPhase(t *testing.T) {
	if got, want := (Config{}).phase(), max(version.Phase, floorPhase); got != want {
		t.Errorf("Config{}.phase() = %d, want %d (version.Phase)", got, want)
	}
	for _, p := range []int{1, 2, 3, 15} {
		if got := (Config{Phase: p}).phase(); got != max(p, floorPhase) {
			t.Errorf("Config{Phase: %d}.phase() = %d", p, got)
		}
	}
}

func TestPhase10Syntax(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want string // feature@offset, separated by spaces
	}{
		{"x := f!(1)", "must calls@6"},
		{"x := os.ReadFile!(p)", "must calls@16"},
		{"x := parse![int](s)", "must calls@10"},
		{"func f!() {}", "must functions@6"},
		{"func (s *S) Start!() {}", "must functions@17"},
		{"func f() {}\nfunc g() { f!() }", "must calls@24"},
		{"#Trace()\nfunc f() {}", "macros@0"},
		{"func f() { x := #Dbg(1) }", "macros@16"},
		{"macro Assert(c bool) {}", "macros@0"},
		{"package p\n\nmacro M() {}\nfunc f!() { g!() }", "must calls@37 must functions@30 macros@11"},
		// Go code.
		{"if !ok { x = !y }", ""},
		{"x := a != b", ""},
		{"x := f(!b)", ""},
		{"x := a[!b]", ""},
		{"if! (a) {}", ""},
		{"x := y ! (z)", ""},
		{"s := \"#x\" // #y\nr := '#'", ""},
		{"type T struct {\n\tmacro int\n}", ""},
		{"func f() {\n\tmacro := 1\n\tmacro = 2\n}", ""},
		{"func f(macro int) {}", ""},
		{"func f() { println(#func(), #file()) }", "macros@19"},
		{"func f() { #util.Assert(true) }", "macros@11"},
		{"func f() { defer x.Close!() }", "must calls@24"},
		{"func parse![T any](s string) T {}", "must functions@10"},
		{"x := F[int]!(y)", ""},
	} {
		var got []string
		for _, u := range phase10Syntax([]byte(tc.src)) {
			got = append(got, fmt.Sprintf("%s@%d", features[u.id].Name, u.offset))
		}
		if strings.Join(got, " ") != tc.want {
			t.Errorf("%q: got %q, want %q", tc.src, strings.Join(got, " "), tc.want)
		}
	}
}

// TestPhase10SyntaxGoCode checks that phase10Syntax finds nothing in Go
// code: the Go source of the dev shell's Go, the stubs and the programs of
// tests/diff.
func TestPhase10SyntaxGoCode(t *testing.T) {
	var roots []string
	if out, err := exec.Command("go", "env", "GOROOT").Output(); err == nil {
		roots = append(roots, filepath.Join(strings.TrimSpace(string(out)), "src"))
	} else {
		t.Logf("no Go source: go env GOROOT: %v", err)
	}
	roots = append(roots, "../../stdlib", "../../tests/diff")
	n := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == "testdata" {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".gp") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			n++
			if uses := phase10Syntax(src); len(uses) > 0 {
				t.Errorf("%s: %s at offset %d", path, features[uses[0].id].Name, uses[0].offset)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d files", n)
}

// The examples: testdata/gates/examples.txt has the gate errors of each
// example at Phase 02 (with -update-gates, the test writes it).
const examplesFile = "testdata/gates/examples.txt"

// gateLine matches the message of a gate error, with its phase.
var gateLine = regexp.MustCompile(`(?:not supported yet|not available yet) \(Phase (\d\d)\)$|not supported in gocpp$`)

// gatePhase returns the phase in the gate error line, Never for one that
// never comes, or 0 if the line is no gate error.
func gatePhase(line string) int {
	m := gateLine.FindStringSubmatch(line)
	if m == nil {
		return 0
	}
	if m[1] == "" {
		return Never
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// TestGateSince checks the Since field: programs of more than one package are
// rejected before Phase 05 with a message that names Phase 06, and pass from
// Phase 05 on, the phase of the project that has them (version.Phase).
func TestGateSince(t *testing.T) {
	f := features[featMultiPackage]
	if f.Since != 5 || f.Phase != 6 {
		t.Fatalf("the multi-package row has Phase %d and Since %d, want 6 and 5", f.Phase, f.Since)
	}
	dir := writeTree(t, map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"main.gp":    "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() {\n\tprintln(lib.N)\n}\n",
		"lib/lib.gp": "package lib\n\nconst N = 1\n",
	})
	cfg := load.Config{}
	for phase := floorPhase; phase <= LastPhase; phase++ {
		got := checkAt(t, cfg, dir, phase, ".")
		want := errorLines(nil)
		if phase < 5 {
			want = errorLines([]string{"# example.com/m", "3:8: programs of more than one package are not supported yet (Phase 06)"})
		}
		if got != want {
			t.Errorf("at Phase %02d:\n got:\n%s\nwant:\n%s", phase, got, want)
		}
	}
	if !f.allowed(5) || f.allowed(4) {
		t.Errorf("allowed(5) = %v, allowed(4) = %v, want true and false", f.allowed(5), f.allowed(4))
	}
	if g := features[featGenerics]; g.allowed(6) || !g.allowed(7) {
		t.Errorf("the generics row: allowed(6) = %v, allowed(7) = %v, want false and true", g.allowed(6), g.allowed(7))
	}
}

// gateLifted returns the phase from which the gates stop reporting the gate
// error line: the phase in it, or Since of its feature when that is earlier.
func gateLifted(line string) int {
	p := gatePhase(line)
	for _, f := range features {
		if f.Since != 0 && strings.HasSuffix(line, f.Message()) {
			return min(p, f.Since)
		}
	}
	return p
}

// exampleErrors returns the error lines of the example at the phase
// (without the "# package" lines).
func exampleErrors(t *testing.T, dir string, phase int, tests bool) []string {
	t.Helper()
	cfg := load.Config{Tests: tests}
	if filepath.Base(dir) == "win32" {
		cfg.GOOS, cfg.GOARCH = "windows", "amd64"
	}
	var lines []string
	for line := range strings.Lines(checkAt(t, cfg, dir, phase, ".")) {
		if line = strings.TrimSuffix(line, "\n"); !strings.HasPrefix(line, "# ") {
			lines = append(lines, line)
		}
	}
	return lines
}

// TestGateExamples checks the 19 examples: at each phase from 02 to 15,
// they give exactly the gate errors of examples.txt whose phase is later
// (and the macro examples, from Phase 10 on, only their syntax errors);
// the latest gate of each example is the phase it is tagged with (its
// phase: line); with their tests, wc and grep-lite need Phase 09.
func TestGateExamples(t *testing.T) {
	mains, err := filepath.Glob("../../examples/*/main.gp")
	if err != nil {
		t.Fatal(err)
	}
	if len(mains) != 19 {
		t.Fatalf("found %d examples, want 19", len(mains))
	}
	var golden strings.Builder
	want := make(map[string][]string)
	if !*updateGates {
		data, err := os.ReadFile(examplesFile)
		if err != nil {
			t.Fatal(err)
		}
		name := ""
		for line := range strings.Lines(string(data)) {
			line = strings.TrimSuffix(line, "\n")
			if n, ok := strings.CutPrefix(line, "== "); ok {
				name = n
				want[name] = []string{}
			} else if line != "" {
				want[name] = append(want[name], line)
			}
		}
	}
	for _, mainFile := range mains {
		dir, err := filepath.Abs(filepath.Dir(mainFile))
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(dir)
		src, err := os.ReadFile(mainFile)
		if err != nil {
			t.Fatal(err)
		}
		tag, err := strconv.Atoi(strings.TrimPrefix(strings.SplitN(string(src), "\n", 2)[0], "// phase: "))
		if err != nil {
			t.Fatalf("%s: no phase line", mainFile)
		}
		got := exampleErrors(t, dir, floorPhase, false)
		if *updateGates {
			fmt.Fprintf(&golden, "== %s\n", name)
			for _, l := range got {
				golden.WriteString(l + "\n")
			}
			golden.WriteString("\n")
			want[name] = got
		}
		t.Run(name, func(t *testing.T) {
			all, ok := want[name]
			if !ok {
				t.Fatalf("not in %s (run with -update-gates)", examplesFile)
			}
			latest := floorPhase
			for _, l := range all {
				p := gatePhase(l)
				if p == 0 {
					t.Errorf("%s has a line that is no gate error: %s", examplesFile, l)
				}
				latest = max(latest, p)
			}
			// win32 is tagged with Phase 14 (Windows), its latest gate is
			// C.wstr.
			if latest != tag {
				t.Errorf("the latest gate is Phase %02d, the example's phase line says %02d", latest, tag)
			}
			for phase := floorPhase; phase <= LastPhase; phase++ {
				var wantGates []string
				for _, l := range all {
					if gateLifted(l) > phase {
						wantGates = append(wantGates, l)
					}
				}
				var gates, others []string
				for _, l := range exampleErrors(t, dir, phase, false) {
					if gatePhase(l) != 0 {
						gates = append(gates, l)
					} else {
						others = append(others, l)
					}
				}
				if !slices.Equal(gates, wantGates) {
					t.Errorf("Phase %02d: got gate errors\n%s\nwant\n%s", phase, strings.Join(gates, "\n"), strings.Join(wantGates, "\n"))
				}
				// Only the syntax of Phase 10 can't be parsed yet: from
				// Phase 10 on, the macro examples have syntax errors.
				if wantOthers := laterSyntax[name] && phase >= 10; wantOthers != (len(others) > 0) {
					t.Errorf("Phase %02d: other errors:\n%s", phase, strings.Join(others, "\n"))
				}
			}
			if _, err := os.Stat(filepath.Join(dir, "main_test.gp")); err == nil {
				if errs := exampleErrors(t, dir, tag, true); len(errs) > 0 {
					t.Errorf("with its tests at Phase %02d:\n%s", tag, strings.Join(errs, "\n"))
				}
				errs := exampleErrors(t, dir, 8, true)
				if !slices.ContainsFunc(errs, func(l string) bool { return strings.HasSuffix(l, "test files are not supported yet (Phase 09)") }) {
					t.Errorf("with its tests at Phase 08, no test files error:\n%s", strings.Join(errs, "\n"))
				}
			}
		})
	}
	if *updateGates {
		if err := os.MkdirAll(filepath.Dir(examplesFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(examplesFile, []byte(golden.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestGateExamplesAllErrors checks that the gate errors of the macro
// examples replace all their syntax errors also with load.Config.AllErrors
// (gocpp check -e).
func TestGateExamplesAllErrors(t *testing.T) {
	for name := range laterSyntax {
		dir, err := filepath.Abs(filepath.Join("../../examples", name))
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.Lines(checkAt(t, load.Config{AllErrors: true}, dir, floorPhase, ".")) {
			if !strings.HasPrefix(line, "# ") && gatePhase(strings.TrimSuffix(line, "\n")) != 10 {
				t.Errorf("%s: %s", name, line)
			}
		}
	}
}

// matrixFile is the generated feature matrix.
const matrixFile = "../../docs/spec/feature-matrix.md"

func TestFeatureMatrix(t *testing.T) {
	want := featureMatrix()
	if *updateGates {
		if err := os.WriteFile(matrixFile, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(matrixFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is not up to date: run go test ./internal/check -run TestFeatureMatrix -update-gates", matrixFile)
	}
	// Every feature and package is in it.
	for _, f := range features {
		if !strings.Contains(want, "| "+mdName(f.Name)+" |") {
			t.Errorf("the matrix has no row for %q", f.Name)
		}
	}
	for _, p := range stdPackages {
		if !strings.Contains(want, "| `"+p.Path+"` |") {
			t.Errorf("the matrix has no row for %s", p.Path)
		}
	}
}

// TestGatesInterfaceMethods checks that the methods of an interface type
// are no function values.
func TestGatesInterfaceMethods(t *testing.T) {
	dir := writeTree(t, map[string]string{"main.gp": "package main\n\ntype S interface {\n\tM(n int) bool\n}\n\nfunc main() {}\n"})
	want := errorLines([]string{
		"3:6: type declarations are not supported yet (Phase 03)",
		"3:8: interfaces are not supported yet (Phase 05)",
	})
	if got := checkAt(t, load.Config{}, dir, floorPhase, "."); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestGatesKeepFoundOrder checks that a gate error that takes the place of
// a go/types error keeps that error's place in the order the errors were
// found, which the limit of 10 errors uses: go/types finds the errors of
// the package-level declarations (lines 16 to 23) before those of the
// function bodies (lines 6 to 12). With and without the gate, the same 10
// errors are shown, only the message of errors.Trace differs.
func TestGatesKeepFoundOrder(t *testing.T) {
	for _, traceFirst := range []bool{false, true} {
		var b strings.Builder
		b.WriteString("package main\n\nimport \"errors\"\n\nfunc f() (err error) {\n")
		if traceFirst {
			b.WriteString("\terrors.Trace(err)\n")
		}
		for i := 1; i <= 6; i++ {
			fmt.Fprintf(&b, "\tu%d()\n", i)
		}
		if !traceFirst {
			b.WriteString("\terrors.Trace(err)\n")
		}
		b.WriteString("\treturn\n}\n\n")
		for i := 1; i <= 8; i++ {
			fmt.Fprintf(&b, "var a%d T%d\n", i, i)
		}
		b.WriteString("\nfunc main() {\n\tf()\n}\n")
		dir := writeTree(t, map[string]string{"main.gp": b.String()})
		text := func(phase int) string {
			cfg := load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}
			prog := load.Load(cfg, ".")
			return errorText(t, prog, Check(prog, fakeProbeConfig(phase)), dir, false)
		}
		got, want := text(9), text(LastPhase)
		want = strings.Replace(want, "undefined: errors.Trace", "errors.Trace is not available yet (Phase 10)", 1)
		if got != want {
			t.Errorf("Trace first %v: at Phase 09:\n%s\nwant (Phase 15's):\n%s", traceFirst, got, want)
		}
		if strings.Count(got, "\n") != 12 || !strings.Contains(got, "too many errors") {
			t.Errorf("Trace first %v: want 10 errors and \"too many errors\":\n%s", traceFirst, got)
		}
		if traceFirst != strings.Contains(got, "errors.Trace is not available yet (Phase 10)") {
			t.Errorf("Trace first %v: the Trace error is shown: %v\n%s", traceFirst, !traceFirst, got)
		}
	}
}

// TestGatesPromotedEveryUse checks that every promoted member of several
// embedded C++ types gets the gate error in the place of go/types' error,
// not only the first one, also through a pointer and in another package,
// and leaves other errors of such a selector alone. (With one embedded
// C++ type the members are promoted: promoted.go.)
func TestGatesPromotedEveryUse(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"vec/vec.gp": "package vec\n\nimport \"<string>\"\nimport \"<vector>\"\n\ntype V struct {\n\tstd.vector[int]\n\tstd.string\n}\n",
		"main.gp":    "package main\n\nimport \"example.com/m/vec\"\n\nfunc main() {\n\tvar v vec.V\n\tv.push_back(1)\n\tp := &v\n\tp.push_back(2)\n\tv.vector.size()\n}\n",
	})
	got := checkAt(t, load.Config{}, dir, 11, ".")
	want := errorLines([]string{
		"# example.com/m",
		"./main.gp:7:4: promoted fields and methods of several embedded C/C++ types are not supported yet (Phase 12)",
		"./main.gp:9:4: promoted fields and methods of several embedded C/C++ types are not supported yet (Phase 12)",
		// A real error: the gate leaves it.
		"./main.gp:10:4: v.vector undefined (cannot refer to unexported field vector)",
	})
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestGatesSyntaxErrorBeforeMacro checks that a syntax error before the
// first Phase 10 syntax of a file stays.
func TestGatesSyntaxErrorBeforeMacro(t *testing.T) {
	dir := writeTree(t, map[string]string{"main.gp": "package main\n\nfunc helper() { x := }\n\nmacro M() {}\n\nfunc main() {\n\t#M()\n}\n"})
	want := errorLines([]string{
		"3:22: expected operand, found '}'",
		"5:1: macros are not supported yet (Phase 10)",
	})
	for _, all := range []bool{false, true} {
		if got := checkAt(t, load.Config{AllErrors: all}, dir, floorPhase, "."); got != want {
			t.Errorf("AllErrors %v: got:\n%s\nwant:\n%s", all, got, want)
		}
	}
}

// TestGatesPromotedTypeNames checks promoted members of a type name
// declared with a C++ type, embedded directly, through a pointer and
// through another alias: with one embedded C++ type they are promoted (no
// error), with a second one the gate says so.
func TestGatesPromotedTypeNames(t *testing.T) {
	const head = "package main\n\nimport \"<string>\"\nimport \"<vector>\"\n\ntype Vec = std.vector[int]\n\ntype Vec2 std.vector[int]\n\ntype Vec3 = Vec\n\n"
	const body = "\nfunc main() {\n\tvar v V\n\tv.push_back(1)\n\tvar w W\n\tw.push_back(2)\n\tvar x X\n\tx.push_back(3)\n}\n"
	one := head + "type V struct{ Vec }\n\ntype W struct{ *Vec2 }\n\ntype X struct{ Vec3 }\n" + body
	dir := writeTree(t, map[string]string{"main.gp": one})
	if got, w := checkAt(t, load.Config{}, dir, 11, "."), ""; got != w {
		t.Errorf("one embedded C++ type:\ngot:\n%s\nwant:\n%s", got, w)
	}
	two := head + "type V struct {\n\tVec\n\tstd.string\n}\n\ntype W struct {\n\t*Vec2\n\tstd.string\n}\n\ntype X struct {\n\tVec3\n\tstd.string\n}\n" + body
	dir = writeTree(t, map[string]string{"main.gp": two})
	got := checkAt(t, load.Config{}, dir, 11, ".")
	var want []string
	for _, line := range []int{29, 31, 33} {
		want = append(want, fmt.Sprintf("%d:4: promoted fields and methods of several embedded C/C++ types are not supported yet (Phase 12)", line))
	}
	if w := errorLines(want); got != w {
		t.Errorf("two embedded C++ types:\ngot:\n%s\nwant:\n%s", got, w)
	}
}
