package cache

import (
	"bytes"
	"context"
	"fmt"
	"go/token"
	"go/types"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/go/gcexportdata"
)

// heapAfterGC returns the live heap.
func heapAfterGC() int64 {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return int64(ms.HeapAlloc)
}

// TestCostFactors measures the factors of the cost of a checked package
// (Package.cost): exportFactor, the memory that the types decoded from
// export data take per byte of the data, for some standard packages and a
// generated 10k-line package; and sourceFactor, the memory of a checked
// package per byte of its source, for that package.
func TestCostFactors(t *testing.T) {
	if testing.Short() {
		t.Skip("measures memory")
	}
	w := newWorld(t, testConfig(), bigPackage())
	var exports [][]byte
	var paths []string
	pkg := w.pkg("big")
	exports, paths = append(exports, pkg.summary.export), append(paths, pkg.summary.importPath)
	snap, release := w.s.Snapshot()
	defer release()
	st := snap.State().(*state)
	for _, path := range []string{"fmt", "os", "strings", "time", "testing", "slices"} {
		h, err := st.handle(context.Background(), pkgID{dir: filepath.Join(w.p.cfg.StdlibDir, path), std: true}, path, nil)
		if err != nil {
			t.Fatal(err)
		}
		s, err := w.p.summaryOf(context.Background(), h)
		if err != nil {
			t.Fatal(err)
		}
		exports, paths = append(exports, s.export), append(paths, path)
	}
	const n = 20
	var size int
	keep := make([]map[string]*types.Package, 0, n*len(exports))
	before := heapAfterGC()
	for range n {
		for i, data := range exports {
			m := make(map[string]*types.Package)
			if _, err := gcexportdata.Read(bytes.NewReader(data), token.NewFileSet(), m, paths[i]); err != nil {
				t.Fatal(err)
			}
			keep = append(keep, m)
			size += len(data)
		}
	}
	grown := heapAfterGC() - before
	factor := float64(grown) / float64(size)
	t.Logf("decoded %d bytes of export data %d times: %.1f bytes of memory per byte", size/n, n, factor)
	if factor > 2*exportFactor || factor < exportFactor/2.0 {
		t.Errorf("the decoded types take %.1f bytes per byte of export data; exportFactor is %d", factor, exportFactor)
	}
	runtime.KeepAlive(keep)
	keep = nil

	// Checks of big with 5 different contents, all kept.
	src := bigPackage()["big/f25.gp"]
	w.open("big/f25.gp", src)
	var pkgs []*Package
	before = heapAfterGC()
	for i := range 5 {
		w.edit("big/f25.gp", strings.Replace(src, "// EDIT", fmt.Sprintf("+ %d // EDIT", i), 1))
		pkgs = append(pkgs, w.pkg("big"))
	}
	grown = heapAfterGC() - before
	var srcSize int64
	for _, f := range pkgs[0].Program.Roots[0].Files {
		srcSize += int64(pkgs[0].Program.Fset.File(f.AST.FileStart).Size())
	}
	factor = float64(grown) / float64(5*srcSize)
	t.Logf("checked %d bytes of source 5 times: %.1f bytes of memory per byte", srcSize, factor)
	if factor > 2*sourceFactor || factor < sourceFactor/2.0 {
		t.Errorf("a checked package takes %.1f bytes per byte of source; sourceFactor is %d", factor, sourceFactor)
	}
	runtime.KeepAlive(pkgs)
}
