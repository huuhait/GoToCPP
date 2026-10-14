package cache

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/stdlib"
)

// The benchmarks of step 11.3: a one-line edit in a function body, then the
// checked package, against the budget "keystroke → diagnostics < 100 ms"
// (README §2.1). Run them with
//
//	go test -run '^$' -bench . ./internal/lsp/cache
//
// Each iteration edits the line differently, so every check is a miss.

// bigPackage returns a module with a package big of 50 generated files of
// about 200 lines each (10k lines), which imports fmt and strings, and a
// main package that imports it.
func bigPackage() map[string]string {
	files := map[string]string{
		"gocpp.mod": "module example.com/bench\n",
		"main.gp":   "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/bench/big\"\n)\n\nfunc main() { fmt.Println(big.F0_0(1), big.T0{}) }\n",
	}
	for i := range 50 {
		var b strings.Builder
		fmt.Fprintf(&b, "package big\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\n")
		fmt.Fprintf(&b, "// T%d is a generated type.\ntype T%d struct {\n\tName  string\n\tCount int\n\tTags  []string\n}\n\n", i, i)
		fmt.Fprintf(&b, "// String returns the name.\nfunc (t T%d) String() string { return fmt.Sprintf(\"%%s:%%d\", t.Name, t.Count) }\n\n", i)
		for j := range 8 {
			fmt.Fprintf(&b, "// F%d_%d is a generated function.\nfunc F%d_%d(n int) int {\n", i, j, i, j)
			b.WriteString("\ttotal := 0\n\tparts := []string{}\n")
			b.WriteString("\tfor k := 0; k < n; k++ {\n\t\tif k%2 == 0 {\n\t\t\ttotal += k\n\t\t} else {\n\t\t\ttotal -= k / 2\n\t\t}\n")
			b.WriteString("\t\tparts = append(parts, fmt.Sprint(k))\n\t}\n")
			fmt.Fprintf(&b, "\tt := T%d{Name: strings.Join(parts, \",\"), Count: total}\n", i)
			b.WriteString("\tif strings.HasPrefix(t.String(), \"x\") {\n\t\treturn -1\n\t}\n")
			b.WriteString("\tm := map[string]int{}\n\tfor _, p := range parts {\n\t\tm[p]++\n\t}\n")
			b.WriteString("\treturn total + len(m) // EDIT\n}\n\n")
		}
		files[fmt.Sprintf("big/f%02d.gp", i)] = b.String()
	}
	return files
}

// benchEdit edits the line with "// EDIT" of the file name in the module
// files, b.N times, each time to a new text, and gets the checked package
// of the folder dir after each edit.
func benchEdit(b *testing.B, files map[string]string, name, dir string) {
	w := newWorld(b, testConfig(), files)
	if errs := w.errs(w.pkg(".")); errs != "" {
		b.Fatalf("errors:\n%s", errs)
	}
	w.pkg(dir)
	src := files[name]
	w.open(name, src)
	start := w.checks()
	b.ResetTimer()
	for i := range b.N {
		w.edit(name, strings.Replace(src, "// EDIT", fmt.Sprintf("+ %d // EDIT", i), 1))
		w.pkg(dir)
	}
	b.StopTimer()
	b.ReportMetric(float64(w.checks()-start)/float64(b.N), "checks/op")
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	n, _ := w.p.checks.Stats()
	b.ReportMetric(float64(ms.HeapAlloc)/(1<<20), "heapMB")
	b.ReportMetric(float64(n), "kept")
	if errs := w.errs(w.pkg(dir)); errs != "" {
		b.Fatalf("errors:\n%s", errs)
	}
}

// BenchmarkEditBig edits a function body in the 10k-line package big.
func BenchmarkEditBig(b *testing.B) {
	benchEdit(b, bigPackage(), "big/f25.gp", "big")
}

// BenchmarkEditBigImporter edits a function body in big and gets main,
// which imports big: big is checked again, main is not (its API is the
// same), so the cost is about that of BenchmarkEditBig.
func BenchmarkEditBigImporter(b *testing.B) {
	benchEdit(b, bigPackage(), "big/f25.gp", ".")
}

// BenchmarkEditStdStub edits a function body in a copy of the largest
// standard library stub, os (as a package of a module).
func BenchmarkEditStdStub(b *testing.B) {
	files := map[string]string{
		"gocpp.mod": "module example.com/bench\n",
		"main.gp":   "package main\n\nimport \"example.com/bench/os\"\n\nfunc main() { _ = os.Args }\n",
	}
	src, err := fs.ReadFile(stdlib.FS(), "os/os.gp")
	if err != nil {
		b.Fatal(err)
	}
	files["os/os.gp"] = string(src)
	files["os/edit.gp"] = "package os\n\nfunc edited(n int) int {\n\treturn n // EDIT\n}\n"
	benchEdit(b, files, "os/edit.gp", "os")
}

// BenchmarkColdBig gets the checked package big from new pipelines: the
// first diagnostics of a 10k-line package and of the standard packages it
// imports, with nothing cached.
func BenchmarkColdBig(b *testing.B) {
	files := bigPackage()
	root := writeTree(b, files)
	b.ResetTimer()
	for range b.N {
		p := NewPipeline(testConfig())
		s := NewSession(p.NewState())
		snap, release := s.Snapshot()
		if _, err := snap.Package(context.Background(), filepath.Join(root, "big")); err != nil {
			b.Fatal(err)
		}
		release()
	}
}
