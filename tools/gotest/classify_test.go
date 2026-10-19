package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var testTarget = Target{GOOS: "linux", GOARCH: "amd64", Minor: 25}

func classify(path, src string) Test {
	return Classify(path, src, ClassifyOptions{Target: testTarget})
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name, path, src, want string
	}{
		{"plain run", "a.go", "// run\n\npackage main\n\nfunc main() { println(1) }\n", "applicable"},
		{"plain errorcheck", "a.go", "// errorcheck\n\npackage p\n\nvar x int = \"s\" // ERROR \"cannot\"\n", "applicable"},
		{"compile", "a.go", "// compile\n\npackage p\n", "applicable"},
		{"build tag windows", "a.go", "//go:build windows\n\n// run\n\npackage main\n", "skip:build-tag"},
		{"skip directive", "a.go", "// skip\n\npackage main\n", "skip:upstream-skip"},
		{"no directive", "a.go", "package main\n", "skip:malformed-recipe"},
		{"unknown directive", "a.go", "// frob\n\npackage main\n", "skip:malformed-recipe"},
		{"asmcheck", "a.go", "// asmcheck\n\npackage p\n", "skip:assembly"},
		{"codegen folder", "codegen/a.go", "// compile\n\npackage p\n", "skip:assembly"},
		{"dwarf folder", "dwarf/a.go", "// run\n\npackage main\n", "skip:debug-info"},
		{"gc flag -m", "a.go", "// errorcheck -0 -m\n\npackage p\n", "skip:gc-flags"},
		{"gc flag -live", "a.go", "// errorcheck -live\n\npackage p\n", "skip:gc-flags"},
		{"gc flag -race", "a.go", "// run -race\n\npackage main\n", "skip:gc-flags"},
		{"gc flag -d=wb", "a.go", "// errorcheck -0 -d=wb\n\npackage p\n", "skip:gc-flags"},
		{"harmless -l -N", "a.go", "// run -gcflags=-l\n\npackage main\n", "skip:gc-flags"},
		{"harmless -l", "a.go", "// compile -l\n\npackage p\n", "applicable"},
		{"harmless -d=panic -0", "a.go", "// errorcheck -0 -d=panic\n\npackage p\n", "applicable"},
		{"experiment default", "a.go", "// run -goexperiment rangefunc\n\npackage main\n", "applicable"},
		{"experiment other", "a.go", "// run -goexperiment arenas\n\npackage main\n", "skip:gc-flags"},
		{"godebug", "a.go", "// run -godebug x=1\n\npackage main\n", "skip:gc-flags"},
		{"cgo", "a.go", "// run\n\npackage main\n\nimport \"C\"\n\nfunc main() {}\n", "skip:cgo"},
		{"syscall", "a.go", "// run\n\npackage main\n\nimport \"syscall\"\n\nvar _ = syscall.Getpid\n", "skip:syscall"},
		{"os/exec", "a.go", "// run\n\npackage main\n\nimport \"os/exec\"\n\nvar _ = exec.Command\n", "skip:os-exec"},
		{"runtime/pprof", "a.go", "// run\n\npackage main\n\nimport \"runtime/pprof\"\n\nvar _ = pprof.Lookup\n", "skip:runtime-internals"},
		{"runtime/debug", "a.go", "// run\n\npackage main\n\nimport \"runtime/debug\"\n\nfunc main() { debug.SetGCPercent(1) }\n", "skip:runtime-internals"},
		{"debug.SetMaxStack", "a.go", "// run\n\npackage main\n\nimport \"runtime/debug\"\n\nfunc main() { debug.SetMaxStack(1) }\n", "skip:stack-growth"},
		{"SetFinalizer", "a.go", "// run\n\npackage main\n\nimport \"runtime\"\n\nfunc main() { runtime.SetFinalizer(nil, nil) }\n", "skip:gc-finalizer"},
		{"MemStats", "a.go", "// run\n\npackage main\n\nimport \"runtime\"\n\nvar m runtime.MemStats\n", "skip:runtime-internals"},
		{"runtime.Gosched is fine", "a.go", "// run\n\npackage main\n\nimport \"runtime\"\n\nfunc main() { runtime.Gosched(); runtime.GC() }\n", "applicable"},
		{"runtime.Caller is fine", "a.go", "// run\n\npackage main\n\nimport \"runtime\"\n\nfunc main() { runtime.Caller(0) }\n", "applicable"},
		{"runtime.Callers", "a.go", "// run\n\npackage main\n\nimport \"runtime\"\n\nfunc main() { runtime.Callers(0, nil) }\n", "skip:runtime-internals"},
		{"unsafe.Sizeof run", "a.go", "// run\n\npackage main\n\nimport \"unsafe\"\n\nvar _ = unsafe.Sizeof(0)\n", "skip:unsafe-layout"},
		{"unsafe.Sizeof compile only checks, so it applies", "a.go", "// compile\n\npackage p\n\nimport \"unsafe\"\n\nvar _ = unsafe.Sizeof(0)\n", "applicable"},
		{"unsafe.Pointer is fine", "a.go", "// run\n\npackage main\n\nimport \"unsafe\"\n\nvar _ unsafe.Pointer\n", "applicable"},
		{"reflect.MakeFunc", "a.go", "// run\n\npackage main\n\nimport \"reflect\"\n\nvar _ = reflect.MakeFunc\n", "skip:reflect-beyond-subset"},
		{"reflect.TypeOf is fine", "a.go", "// run\n\npackage main\n\nimport \"reflect\"\n\nvar _ = reflect.TypeOf\n", "applicable"},
		{"linkname", "a.go", "// run\n\npackage main\n\nimport _ \"unsafe\"\n\n//go:linkname f runtime.f\nfunc f()\n", "skip:linkname"},
		{"nosplit in compile", "a.go", "// compile\n\npackage p\n\n//go:nosplit\nfunc f() {}\n", "skip:gc-pragma"},
		{"declares C", "a.go", "// run\n\npackage main\n\ntype C struct{}\n\nfunc main() { var c C; _ = c }\n", "skip:spec-deviation"},
		{"local C", "a.go", "// compile\n\npackage p\n\nfunc f() { C := 1; _ = C }\n", "skip:spec-deviation"},
		{"C as a field is fine", "a.go", "// run\n\npackage main\n\ntype T struct{ C int }\n\nfunc main() { _ = T{C: 1} }\n", "applicable"},
		{"C in a string is fine", "a.go", "// run\n\npackage main\n\nfunc main() { println(\"C\") }\n", "applicable"},
		{"net/http", "a.go", "// run\n\npackage main\n\nimport \"net/http\"\n\nvar _ = http.Get\n", "skip:not-planned-package"},
		{"go/ast", "a.go", "// run\n\npackage main\n\nimport \"go/ast\"\n\nvar _ ast.Node\n", "skip:not-planned-package"},
		{"line directive errorcheck", "a.go", "// errorcheck\n\npackage p\n\n//line x.go:1\nvar a int = \"\" // ERROR \"x\"\n\n//line y.go:1\nvar b int\n", "skip:line-directive"},
		{"names in comments don't count", "a.go", "// run\n\npackage main\n\n// runtime.SetFinalizer is not used here, and unsafe.Sizeof is not either\nfunc main() { println(\"runtime.MemStats\") }\n", "applicable"},
		{"names in block comments", "a.go", "// run\n\npackage main\n\n/* runtime.Stack */\nfunc main() {}\n", "applicable"},
		{"errorcheck with syntax error imports", "a.go", "// errorcheck\n\npackage main\n\nimport \"syscall\"\n\nfunc f( { // ERROR \"syntax\"\n", "skip:syscall"},
	}
	for _, c := range cases {
		got := classify(c.path, c.src)
		if got.Status != c.want {
			t.Errorf("%s: status %s (%s), want %s", c.name, got.Status, got.Detail, c.want)
		}
	}
}

func TestClassifyOverrideAndAsmDir(t *testing.T) {
	o := ClassifyOptions{Target: testTarget, Overrides: map[string]string{"a.go": "go-tool"},
		HasAsm: func(dir string) bool { return dir == "b.dir" }}
	if got := Classify("a.go", "// run\n\npackage main\n", o); got.Status != "skip:go-tool" {
		t.Errorf("override: %s", got.Status)
	}
	if got := Classify("b.go", "// rundir\n\npackage ignored\n", o); got.Status != "skip:assembly" {
		t.Errorf("asm dir: %s", got.Status)
	}
	if got := Classify("c.go", "// rundir\n\npackage ignored\n", o); got.Status != "applicable" {
		t.Errorf("no asm: %s", got.Status)
	}
}

func TestEveryReasonCodeIsDocumented(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range reasons {
		if seen[r.Code] || r.Text == "" || (r.By != "go" && r.By != "gocpp") {
			t.Errorf("bad reason entry %+v", r)
		}
		seen[r.Code] = true
	}
	// every code the classifier can return is in the table
	for _, c := range []string{"build-tag", "upstream-skip", "malformed-recipe", "assembly", "gc-flags", "gc-pragma",
		"linkname", "cgo", "runtime-internals", "gc-finalizer", "stack-growth", "unsafe-layout",
		"reflect-beyond-subset", "syscall", "os-exec", "not-planned-package", "go-tool", "line-directive", "debug-info"} {
		if !seen[c] {
			t.Errorf("reason %s is not in the table", c)
		}
	}
	if _, err := ParseOverrides("a.go nonsense\n"); err == nil {
		t.Error("an unknown reason code in overrides.txt is accepted")
	}
	m, err := ParseOverrides("# c\na.go go-tool  # why\n")
	if err != nil || m["a.go"] != "go-tool" {
		t.Errorf("ParseOverrides: %v %v", m, err)
	}
}

// The table of reason codes in the guide is the table of classify.go.
func TestGuideTableMatchesReasons(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "guides", "conformance.md"))
	if err != nil {
		t.Skip("no guide:", err)
	}
	rows := map[string]string{}
	for _, l := range strings.Split(string(b), "\n") {
		if m := regexp.MustCompile("^\\| `([a-z-]+)` \\| (go|gocpp) \\| (.*) \\|$").FindStringSubmatch(l); m != nil {
			rows[m[1]] = m[2] + "|" + m[3]
		}
	}
	for _, r := range reasons {
		if want := r.By + "|" + r.Text; rows[r.Code] != want {
			t.Errorf("guide row of %s is %q, classify.go says %q", r.Code, rows[r.Code], want)
		}
	}
	if len(rows) != len(reasons) {
		t.Errorf("the guide has %d reason rows, classify.go %d", len(rows), len(reasons))
	}
}

func TestStripComments(t *testing.T) {
	got := stripComments("a // b\nc /* d\ne */ f \"x.y\" `r\nw` 'q'\n")
	if strings.Contains(got, "b") || strings.Contains(got, "d") || strings.Contains(got, "x.y") || strings.Count(got, "\n") != 4 {
		t.Errorf("stripComments = %q", got)
	}
}

func TestClassifyDirSources(t *testing.T) {
	o := ClassifyOptions{Target: testTarget, DirSources: func(dir string) []string {
		switch dir {
		case "a.dir":
			return []string{"package p\n\nimport \"os/exec\"\n\nvar _ = exec.Command\n"}
		case "b.dir":
			return []string{"package p\n\nvar C int\n"}
		}
		return []string{"package p\n"}
	}}
	for path, want := range map[string]string{"a.go": "skip:os-exec", "b.go": "skip:spec-deviation", "c.go": "applicable"} {
		if got := Classify(path, "// rundir\n\npackage ignored\n", o); got.Status != want {
			t.Errorf("%s: %s (%s), want %s", path, got.Status, got.Detail, want)
		}
	}
}

// A test that uses names of two reasons always gets the same one.
func TestClassifyIsDeterministic(t *testing.T) {
	src := "// run\n\npackage main\n\nimport \"runtime\"\n\nfunc main() { runtime.SetFinalizer(nil, nil); var m runtime.MemStats; runtime.ReadMemStats(&m) }\n"
	first := classify("a.go", src).Status
	for i := 0; i < 200; i++ {
		if got := classify("a.go", src).Status; got != first {
			t.Fatalf("run %d: %s, then %s", i, first, got)
		}
	}
}
