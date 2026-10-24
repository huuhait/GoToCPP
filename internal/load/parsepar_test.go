package load

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// parseSources makes n sources of different sizes; some have syntax errors
// and one is empty.
func parseSources(n int) (names []string, srcs [][]byte) {
	for i := range n {
		var b strings.Builder
		fmt.Fprintf(&b, "package p\n\nimport \"fmt\"\n")
		for j := range 5 + i*7%23 {
			fmt.Fprintf(&b, "\nfunc F%d_%d(a int) int {\n\tx := a * %d\n\tfmt.Println(x)\n\treturn x\n}\n", i, j, j)
		}
		switch {
		case i%5 == 3:
			b.WriteString("\nfunc broken( {\n")
		case i%7 == 6:
			b.Reset()
		}
		names = append(names, fmt.Sprintf("/x/f%02d.gp", i))
		srcs = append(srcs, []byte(b.String()))
	}
	return names, srcs
}

func normalParseFixture() map[string]string {
	padding := strings.Repeat("// normal parser padding\n", 100)
	return map[string]string{
		"gocpp.mod": "module example.com/parallel\n",
		"a.gp":      "package parallel\n//line logical_a.gp:40\nconst A = 1\n" + padding,
		"b.gp":      "package parallel\nfunc F(v int) int { return v + A }\n" + padding,
		"c.gp": "//gocpp:link m\npackage parallel\nimport cstd \"<stddef.h>\"\n" +
			"var Native cstd.size_t\n" + padding,
		"d.gp": "package parallel\nmacro Twice(v int) int { return v + v }\n" +
			"func Macro() int { return #Twice(A) }\n" + padding,
		"native.c":        "//go:build linux\n\nint native_c = 1;\n",
		"native.cc":       "int native_cpp = 2;\n",
		"skip_windows.gp": "package excluded\nthis is deliberately not valid syntax\n",
	}
}

// A missing normal-path wiring fails after a bounded wait instead of hanging.
func releaseParallelParsers(
	entered <-chan struct{}, release chan struct{}, ready func(),
) <-chan bool {
	observed := make(chan bool, 1)
	go func() {
		defer close(release)
		timer := time.NewTimer(time.Minute) // (only ends a hang)
		defer timer.Stop()
		for range 2 {
			select {
			case <-entered:
			case <-timer.C:
				observed <- false
				return
			}
		}
		if ready != nil {
			ready()
		}
		observed <- true
	}()
	return observed
}

type capturedParseFS struct {
	base      *mapFS
	mu        sync.Mutex
	parsing   atomic.Bool
	lateReads atomic.Int32
	canceled  string
	paths     []string
}

func (f *capturedParseFS) ReadFile(name string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.parsing.Load() {
		f.lateReads.Add(1)
	}
	f.paths = append(f.paths, name)
	if name == f.canceled {
		return nil, context.Canceled
	}
	return f.base.ReadFile(name)
}

func (f *capturedParseFS) ReadDir(name string) ([]fs.DirEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.parsing.Load() {
		f.lateReads.Add(1)
	}
	return f.base.ReadDir(name)
}

func (f *capturedParseFS) Stat(name string) (fs.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.parsing.Load() {
		f.lateReads.Add(1)
	}
	return f.base.Stat(name)
}

func TestReadNormalFilesInParallel(t *testing.T) {
	previous := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(previous)
	root := filepath.Join(t.TempDir(), "virtual")
	fsys := &capturedParseFS{base: &mapFS{files: map[string]string{}}}
	for name, text := range normalParseFixture() {
		fsys.base.files[filepath.Join(root, name)] = text
	}
	overlay := filepath.Join(root, "b.gp")
	canceled := filepath.Join(root, "canceled.gp")
	fsys.base.files[canceled] = "package parallel\n"
	fsys.canceled = canceled
	loader := newLoader(Config{
		Dir:     root,
		FS:      fsys,
		GOOS:    "linux",
		GOARCH:  "amd64",
		Overlay: map[string][]byte{overlay: []byte(fsys.base.files[overlay] + "\nvar Overlay = A\n")},
	})
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	observed := releaseParallelParsers(entered, release, nil)
	var calls atomic.Int32
	var sharedFileSet atomic.Bool
	loader.parseFile = func(
		set *token.FileSet, name string, src any, mode parser.Mode,
	) (*ast.File, error) {
		calls.Add(1)
		fsys.parsing.Store(true)
		if set == loader.fset {
			sharedFileSet.Store(true)
		}
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return parser.ParseFile(set, name, src, mode)
	}

	pkg := loader.newPackage("example.com/parallel", root, false, nil)
	loader.read(pkg, nil, false)
	parallel := <-observed
	t.Run("normal path admits private parser workers", func(t *testing.T) {
		if !parallel || calls.Load() != 4 || sharedFileSet.Load() {
			t.Fatalf("parallel=%v, %d parses, shared allocator=%v",
				parallel, calls.Load(), sharedFileSet.Load())
		}
	})
	t.Run("read hooks finish before parsing", func(t *testing.T) {
		if fsys.lateReads.Load() != 0 {
			t.Fatal("parsing reread the configured file system after sources were captured")
		}
	})
	t.Run("overlay bytes are preserved", func(t *testing.T) {
		if slices.Contains(fsys.paths, overlay) {
			t.Fatal("the overlay was read through the file-system hook")
		}
		if !strings.Contains(string(pkg.Files[1].Src), "var Overlay = A") {
			t.Fatal("parsing ignored captured overlay bytes")
		}
	})
	t.Run("canceled read error is preserved", func(t *testing.T) {
		got := pkg.Errors.Diagnostics()
		if len(got) != 1 || got[0].Msg != context.Canceled.Error() {
			t.Fatalf("read-hook error was changed: %v", got)
		}
	})
}

func TestParseParallelSharedFileSet(t *testing.T) {
	names, srcs := parseSources(4)
	set := token.NewFileSet()
	pre := set.AddFile("preexisting", -1, 100)
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var concurrent *token.File
	observed := releaseParallelParsers(entered, release, func() {
		concurrent = set.AddFile("concurrent", -1, 50)
	})
	parseFile := func(
		private *token.FileSet, name string, src any, mode parser.Mode,
	) (*ast.File, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return parser.ParseFile(private, name, src, mode)
	}
	files, _ := parseFilesUsing(set, names, srcs, parser.ParseComments, 2, 0, parseFile)
	if !<-observed {
		t.Fatal("parallel parser did not start two workers")
	}
	if set.File(pre.Pos(0)) != pre || set.File(concurrent.Pos(0)) != concurrent {
		t.Fatal("parsing replaced another caller's file")
	}
	lastEnd := pre.Base() + pre.Size()
	for i, file := range files {
		tokenFile := set.File(file.FileStart)
		if tokenFile == nil || tokenFile.Name() != names[i] || tokenFile.Base() <= lastEnd {
			t.Fatalf("file %d is missing, overlapping or out of order", i)
		}
		lastEnd = tokenFile.Base() + tokenFile.Size()
	}
	if concurrent.Base() <= lastEnd {
		t.Fatal("the shared file-set allocator overlapped the parser's reservation")
	}
	var count int
	set.Iterate(func(file *token.File) bool {
		count++
		if file.Name() == "" {
			t.Error("position reservation escaped into the completed file set")
		}
		return true
	})
	if count != len(files)+2 {
		t.Fatalf("%d token files, want %d", count, len(files)+2)
	}
}

func TestReadParallelParserPanic(t *testing.T) {
	previous := runtime.GOMAXPROCS(2)
	defer runtime.GOMAXPROCS(previous)
	root := writeTree(t, normalParseFixture())
	loader := newLoader(Config{Dir: root, GOOS: "linux", GOARCH: "amd64"})
	var finished atomic.Int32
	loader.parseFile = func(
		set *token.FileSet, name string, src any, mode parser.Mode,
	) (*ast.File, error) {
		defer finished.Add(1)
		switch filepath.Base(name) {
		case "a.gp":
			panic("first source panic")
		case "b.gp":
			panic("second source panic")
		}
		return parser.ParseFile(set, name, src, mode)
	}
	var caught any
	func() {
		defer func() { caught = recover() }()
		pkg := loader.newPackage("example.com/parallel", root, false, nil)
		loader.read(pkg, nil, false)
	}()
	if caught != "first source panic" {
		t.Fatalf("caller recovered %v, want first source panic", caught)
	}
	if finished.Load() != 4 {
		t.Fatal("a parser worker outlived the caller's recovery boundary")
	}
	loader.fset.Iterate(func(file *token.File) bool {
		if file.Name() == "" {
			t.Error("panic left the position reservation in the shared file set")
		}
		return true
	})
}

func TestReadParallelMatchesSequential(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	for _, name := range []string{
		"normal package", "named file order", "overlay", "syntax and external test errors",
		"standard library",
	} {
		t.Run(name, func(t *testing.T) {
			files := normalParseFixture()
			if name == "syntax and external test errors" {
				padding := strings.Repeat("// syntax padding\n", 150)
				files["b.gp"] = "package other\nfunc Broken( {\n" + padding
				files["c.gp"] = ""
				files["x_test.gp"] = "package parallel_test\nfunc Broken( {\n" + padding
				files["y_test.gp"] = "package another_test\nvar N = 1\n" + padding
			}
			root := writeTree(t, files)
			cfg := Config{Dir: root, GOOS: "linux", GOARCH: "amd64"}
			patterns := []string{"."}
			switch name {
			case "named file order":
				patterns = []string{"d.gp", "b.gp", "a.gp", "c.gp"}
			case "overlay":
				cfg.Overlay = map[string][]byte{
					filepath.Join(root, "a.gp"): []byte(strings.ReplaceAll(files["a.gp"], "A = 1", "A = 9")),
					filepath.Join(root, "z.gp"): []byte("package parallel\nvar Overlay = F(1)\n"),
				}
			case "syntax and external test errors":
				cfg.Tests = true
				cfg.AllErrors = true
			case "standard library":
				standard := fstest.MapFS{}
				for file, text := range files {
					if file != "gocpp.mod" {
						standard["parallelstd/"+file] = &fstest.MapFile{Data: []byte(text)}
					}
				}
				cfg.Stdlib = standard
				cfg.StdlibDir = filepath.Join(root, "stdlib")
				patterns = []string{"parallelstd"}
			}

			// Both public Load calls start with identical imported-position space.
			cfg.Fset = token.NewFileSet()
			cfg.Fset.AddFile("preexisting", -1, 300)
			runtime.GOMAXPROCS(1)
			want := Load(cfg, patterns...)
			cfg.Fset = token.NewFileSet()
			cfg.Fset.AddFile("preexisting", -1, 300)
			runtime.GOMAXPROCS(4)
			got := Load(cfg, patterns...)
			assertParallelLoadEqual(t, want, got)
			if name == "syntax and external test errors" && !got.HasErrors() {
				t.Fatal("the error fixture unexpectedly lost its diagnostics")
			}
			if name != "syntax and external test errors" && got.HasErrors() {
				t.Fatalf("valid fixture: %s", errorText(t, got, root))
			}
		})
	}
}

func assertParallelLoadEqual(t *testing.T, want, got *Program) {
	t.Helper()
	if !reflect.DeepEqual(summary(want), summary(got)) {
		t.Fatal("parallel parsing changed package/file/import order")
	}
	if !reflect.DeepEqual(want.Errors.Diagnostics(), got.Errors.Diagnostics()) {
		t.Fatal("parallel parsing changed program diagnostics")
	}
	if want.Fset.Base() != got.Fset.Base() {
		t.Fatal("parallel parsing changed the next file-set base")
	}
	if len(want.Packages) != len(got.Packages) {
		t.Fatal("parallel parsing changed package count")
	}
	for i, expected := range want.Packages {
		actual := got.Packages[i]
		if !reflect.DeepEqual(expected.Errors.Diagnostics(), actual.Errors.Diagnostics()) {
			t.Fatalf("package %s: diagnostics or their ordering changed", expected.ImportPath)
		}
		if !slices.Equal(expected.CFiles, actual.CFiles) ||
			!slices.Equal(expected.CXXFiles, actual.CXXFiles) {
			t.Fatal("parallel parsing changed foreign-source metadata")
		}
		for j, file := range expected.Files {
			other := actual.Files[j]
			if file.Path != other.Path || file.Test != other.Test ||
				!reflect.DeepEqual(file.Src, other.Src) {
				t.Fatal("parallel parsing changed file identity or retained source")
			}
			if !reflect.DeepEqual(file.AST, other.AST) {
				t.Fatalf("%s: rewritten AST or numeric positions changed", file.Path)
			}
			if !reflect.DeepEqual(file.Info, other.Info) ||
				!reflect.DeepEqual(file.Directives, other.Directives) {
				t.Fatalf("%s: import/directive metadata changed", file.Path)
			}
			if !reflect.DeepEqual(file.Rewrite, other.Rewrite) {
				t.Fatalf("%s: the record of the header and namespace rewrite changed", file.Path)
			}
			ast.Inspect(file.AST, func(node ast.Node) bool {
				if node != nil && want.Fset.Position(node.Pos()) != got.Fset.Position(node.Pos()) {
					t.Errorf("%s: resolved source position changed", file.Path)
				}
				return true
			})
		}
	}
	type fileState struct {
		Name       string
		Base, Size int
		Lines      []int
	}
	state := func(set *token.FileSet) []fileState {
		var files []fileState
		set.Iterate(func(file *token.File) bool {
			files = append(files, fileState{file.Name(), file.Base(), file.Size(), file.Lines()})
			return true
		})
		return files
	}
	if !reflect.DeepEqual(state(want.Fset), state(got.Fset)) {
		t.Fatal("parallel parsing changed registered token files or line tables")
	}
}

// The parallel parse gives the files the bases that the sequence gives them,
// the same trees and the same errors.
func TestParseFilesParallel(t *testing.T) {
	for _, n := range []int{1, 2, 3, 10, 40} {
		for _, pre := range []int{0, 1, 500} {
			names, srcs := parseSources(n)
			seq := token.NewFileSet()
			par := token.NewFileSet()
			for _, fs := range []*token.FileSet{seq, par} {
				if pre > 0 {
					fs.AddFile("pre", -1, pre)
				}
			}
			var wantFiles []*ast.File
			var wantErrs []error
			for i := range srcs {
				f, err := parser.ParseFile(seq, names[i], srcs[i], parser.ParseComments)
				wantFiles, wantErrs = append(wantFiles, f), append(wantErrs, err)
			}
			gotFiles, gotErrs := parseFilesWith(par, names, srcs, parser.ParseComments, 8, 0)
			if seq.Base() != par.Base() {
				t.Errorf("n=%d pre=%d: base %d, want %d", n, pre, par.Base(), seq.Base())
			}
			for i := range srcs {
				w, g := wantFiles[i], gotFiles[i]
				if w.FileStart != g.FileStart || w.FileEnd != g.FileEnd {
					t.Errorf("n=%d pre=%d file %d: range %d-%d, want %d-%d", n, pre, i, g.FileStart, g.FileEnd, w.FileStart, w.FileEnd)
				}
				if fmt.Sprint(wantErrs[i]) != fmt.Sprint(gotErrs[i]) {
					t.Errorf("n=%d file %d: error %v, want %v", n, i, gotErrs[i], wantErrs[i])
				}
				if len(w.Decls) != len(g.Decls) || len(w.Comments) != len(g.Comments) {
					t.Errorf("n=%d file %d: %d decls, want %d", n, i, len(g.Decls), len(w.Decls))
				}
				for d := range w.Decls {
					if d < len(g.Decls) && (w.Decls[d].Pos() != g.Decls[d].Pos() || w.Decls[d].End() != g.Decls[d].End()) {
						t.Errorf("n=%d file %d decl %d: positions differ", n, i, d)
					}
				}
				// The position of every declaration reads the same in both sets.
				for d := range w.Decls {
					if d < len(g.Decls) && seq.Position(w.Decls[d].Pos()) != par.Position(g.Decls[d].Pos()) {
						t.Errorf("n=%d file %d decl %d: %v, want %v", n, i, d, par.Position(g.Decls[d].Pos()), seq.Position(w.Decls[d].Pos()))
					}
				}
				if seq.File(w.FileStart) == nil || par.File(g.FileStart) == nil || seq.File(w.FileStart).Name() != par.File(g.FileStart).Name() {
					t.Errorf("n=%d file %d: the file is not in the set", n, i)
				}
			}
			if !reflect.DeepEqual(wantErrs == nil, gotErrs == nil) {
				t.Error("errors")
			}
		}
	}
}
