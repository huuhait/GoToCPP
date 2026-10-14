package cache

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/cache/filecache"
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

// bigWorkspace returns a module of n copies of the package big of
// bigPackage (10k lines each: p0, p1, ...) and a main package that imports
// them all: a workspace of n×10k lines.
func bigWorkspace(n int) map[string]string {
	files := map[string]string{"gocpp.mod": "module example.com/bench\n"}
	var imports, uses strings.Builder
	for i := range n {
		name := fmt.Sprintf("p%d", i)
		for path, src := range bigPackage() {
			if dir, file, ok := strings.Cut(path, "/"); ok && dir == "big" {
				files[name+"/"+file] = strings.Replace(src, "package big", "package "+name, 1)
			}
		}
		fmt.Fprintf(&imports, "\t\"example.com/bench/%s\"\n", name)
		fmt.Fprintf(&uses, "\tfmt.Println(%s.F0_0(1), %s.T0{})\n", name, name)
	}
	files["main.gp"] = "package main\n\nimport (\n\t\"fmt\"\n\n" + imports.String() + ")\n\nfunc main() {\n" + uses.String() + "}\n"
	return files
}

// benchOpen opens the main package of a 50k-line workspace (bigWorkspace(5))
// in a new pipeline per iteration, as a server that starts: with an empty
// disk cache (warm false: every package is checked, and the summaries are
// written in the background, which is not timed), or with the disk cache
// of an earlier run (warm: only main is checked). It reports the checks and
// the summaries from disk per open, and the live heap after the last open
// (with that server's caches only: the loop keeps no earlier one alive, so
// it registers no Cleanup per open).
func benchOpen(b *testing.B, warm bool) {
	root := writeTree(b, bigWorkspace(5))
	dir := b.TempDir()
	open := func(cacheDir string) *world {
		c, err := filecache.Open(filecache.Options{Dir: cacheDir, Version: "bench"})
		if err != nil {
			b.Fatal(err)
		}
		p := NewPipeline(diskCfg(c))
		w := &world{t: b, root: root, p: p, s: NewSession(p.NewState())}
		if errs := w.errs(w.pkg(".")); errs != "" {
			b.Fatalf("errors:\n%s", errs)
		}
		return w
	}
	closeAll := func(w *world) {
		w.p.Close()
		w.p.cfg.Disk.Close()
	}
	if warm {
		closeAll(open(dir))
	}
	var checks, hits int64
	var last *world
	b.ResetTimer()
	for range b.N {
		cacheDir := dir
		if !warm {
			b.StopTimer()
			cacheDir = b.TempDir()
			b.StartTimer()
		}
		w := open(cacheDir)
		b.StopTimer()
		h, _, _ := w.p.DiskStats()
		checks, hits = checks+w.checks(), hits+h
		closeAll(w)
		last = w
		b.StartTimer()
	}
	b.StopTimer()
	b.ReportMetric(float64(checks)/float64(b.N), "checks/op")
	b.ReportMetric(float64(hits)/float64(b.N), "fromdisk/op")
	b.ReportMetric(float64(heapAfterGC())/(1<<20), "heapMB")
	runtime.KeepAlive(last)
}

// BenchmarkOpenCold50k: the first diagnostics of a 50k-line workspace with
// an empty disk cache (budget < 2 s, README §2.1).
func BenchmarkOpenCold50k(b *testing.B) { benchOpen(b, false) }

// BenchmarkOpenWarm50k: the same with a warm disk cache (budget < 300 ms).
func BenchmarkOpenWarm50k(b *testing.B) { benchOpen(b, true) }
