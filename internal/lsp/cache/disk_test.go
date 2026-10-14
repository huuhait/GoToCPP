package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache/filecache"
)

// The tests of the summaries on disk (disk.go) and of the side table of
// opaque types (opaque.go), step 11.3b. Every cache is in t.TempDir().

// openDisk opens the disk cache in dir with the version (the "gocpp
// binary").
func openDisk(t testing.TB, dir, version string) *filecache.Cache {
	t.Helper()
	c, err := filecache.Open(filecache.Options{Dir: dir, Version: version})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}

// serverRun is one run of a server: a new pipeline over the tree root with
// the configuration cfg, as after a restart.
func serverRun(t testing.TB, cfg Config, root string) *world {
	p := NewPipeline(cfg)
	t.Cleanup(p.Close)
	return &world{t: t, root: root, p: p, s: NewSession(p.NewState())}
}

// diskCfg returns testConfig with the disk cache c.
func diskCfg(c *filecache.Cache) Config {
	cfg := testConfig()
	cfg.Disk = c
	return cfg
}

// describe returns what hover would show of the uses of the objects of
// other packages in pkg: each one's place, name, object and opaque type.
func describe(fset *token.FileSet, info *types.Info, self *types.Package, opaque func(types.Object) string) string {
	if info == nil {
		return "(not type-checked)"
	}
	var lines []string
	for id, obj := range info.Uses {
		if obj.Pkg() == nil || obj.Pkg() == self {
			continue
		}
		pos := fset.Position(id.Pos())
		lines = append(lines, fmt.Sprintf("%s:%d:%d %s: %s | %s", filepath.Base(pos.Filename), pos.Line, pos.Column,
			id.Name, types.ObjectString(obj, nil), opaque(obj)))
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// view returns the diagnostics of the package of dir and what hover would
// show of its uses (describe).
func (w *world) view(dir string) string {
	w.t.Helper()
	pkg := w.pkg(dir)
	return w.errs(pkg) + "\n--\n" + describe(pkg.Fset(), pkg.Check.Info, pkg.Check.Types, func(obj types.Object) string {
		t, _ := pkg.OpaqueType(obj)
		return t
	})
}

// warm runs a server once over the files with the disk cache in a new
// folder, and returns the tree, the cache folder, the number of checks of
// that cold start and what it saw of main.
func warm(t *testing.T, files map[string]string) (root, dir string, cold int64, view string) {
	t.Helper()
	root, dir = writeTree(t, files), t.TempDir()
	a := serverRun(t, diskCfg(openDisk(t, dir, "gocpp-1")), root)
	view = a.view(".")
	cold = a.checks()
	a.p.Close() // the writes are done
	if _, _, writes := a.p.DiskStats(); writes == 0 {
		t.Fatal("nothing was written to the disk cache")
	}
	return root, dir, cold, view
}

// TestDiskWarmStart: a second server run over the same files checks only
// the package it shows: its imports (also the standard ones) come from
// disk, and it sees the same diagnostics and types.
func TestDiskWarmStart(t *testing.T) {
	root, dir, cold, want := warm(t, app)
	if cold < 4 {
		t.Fatalf("the cold start made %d checks, want main, geom, shapes, fmt and more", cold)
	}
	b := serverRun(t, diskCfg(openDisk(t, dir, "gocpp-1")), root)
	if got := b.view("."); got != want {
		t.Errorf("warm start:\n%s\nwant:\n%s", got, want)
	}
	hits, _, _ := b.p.DiskStats()
	if b.checks() != 1 || hits < 3 {
		t.Errorf("warm start: %d checks (want 1: main), %d summaries from disk", b.checks(), hits)
	}
	// An import that is opened later is checked from source, with the
	// summaries of its own imports from disk.
	if errs := b.errs(b.pkg("geom")); errs != "" || b.checks() != 2 {
		t.Errorf("geom: %d checks (want 2), errors:\n%s", b.checks(), errs)
	}
}

// TestDiskKeys: what changes the key of a summary on disk: the content of
// its files, the API of its imports, the gocpp binary and the
// configuration (each part of it). The next run sees what a run without
// the disk cache sees.
func TestDiskKeys(t *testing.T) {
	type test struct {
		name   string
		edit   func(root string)      // changes the files
		cfg    func(cfg *Config)      // changes the configuration
		ver    string                 // the version of the disk cache
		checks func(cold int64) int64 // the checks of the next run; nil: none from disk
	}
	writeShapes := func(src string) func(string) {
		return func(root string) {
			if err := os.WriteFile(filepath.Join(root, "shapes", "shapes.gp"), []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	tests := []test{
		{name: "nothing", checks: func(int64) int64 { return 1 }},
		// shapes changes, its API not: shapes and main are checked, geom
		// comes from disk (its key has the same API hash of shapes).
		{name: "body edit", edit: writeShapes(app["shapes/shapes.gp"] + "\nfunc f() int { return 1 }\n"),
			checks: func(int64) int64 { return 2 }},
		// The API of shapes changes: geom is checked again too.
		{name: "API edit", edit: writeShapes(app["shapes/shapes.gp"] + "\nfunc F() int { return 1 }\n"),
			checks: func(int64) int64 { return 3 }},
		{name: "binary", ver: "gocpp-2", checks: func(cold int64) int64 { return cold }},
		{name: "tags", cfg: func(cfg *Config) { cfg.Tags = []string{"debug"} }},
		{name: "GOOS", cfg: func(cfg *Config) { cfg.GOOS = "windows" }},
		{name: "GOARCH", cfg: func(cfg *Config) { cfg.GOARCH = "arm64" }},
		// The gate phase: a program of more than one package is gated
		// before Phase 06, so the diagnostics differ too.
		{name: "phase", cfg: func(cfg *Config) { cfg.Phase = 3 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, dir, cold, want := warm(t, app)
			if tt.edit != nil {
				tt.edit(root)
			}
			ver := "gocpp-1"
			if tt.ver != "" {
				ver = tt.ver
			}
			cfg := diskCfg(openDisk(t, dir, ver))
			if tt.cfg != nil {
				tt.cfg(&cfg)
			}
			b := serverRun(t, cfg, root)
			if tt.edit != nil || tt.cfg != nil {
				noDisk := cfg
				noDisk.Disk = nil
				want = serverRun(t, noDisk, root).view(".")
			}
			if got := b.view("."); got != want {
				t.Errorf("the next run:\n%s\nwant (without the disk cache):\n%s", got, want)
			}
			if tt.checks == nil {
				if hits, _, _ := b.p.DiskStats(); hits != 0 {
					t.Errorf("%d summaries from disk, want none", hits)
				}
			} else if got, want := b.checks(), tt.checks(cold); got != want {
				t.Errorf("%d checks, want %d (the cold start made %d)", got, want, cold)
			}
		})
	}
}

// TestDiskCorrupt: entries that are cut short are misses: the packages are
// checked again (with the same results) and written again.
func TestDiskCorrupt(t *testing.T) {
	root, dir, cold, want := warm(t, app)
	n := 0
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), "-"+diskKind) {
			data, _ := os.ReadFile(path)
			os.WriteFile(path, data[:len(data)/2], 0o600)
			n++
		}
		return nil
	})
	if n == 0 {
		t.Fatal("no entries")
	}
	b := serverRun(t, diskCfg(openDisk(t, dir, "gocpp-1")), root)
	if got := b.view("."); got != want {
		t.Errorf("after the damage:\n%s\nwant:\n%s", got, want)
	}
	if b.checks() != cold {
		t.Errorf("%d checks, want %d (all of them)", b.checks(), cold)
	}
	b.p.Close()
	c := serverRun(t, diskCfg(openDisk(t, dir, "gocpp-1")), root)
	if got := c.view("."); got != want || c.checks() != 1 {
		t.Errorf("after the entries were written again: %d checks, view:\n%s", c.checks(), got)
	}
}

// TestDiskBadExport: an entry that is whole, but whose export data can't be
// read, is removed, and its package is checked from source: the importer
// sees what it would without the disk cache; the next run uses the entry
// that was written again.
func TestDiskBadExport(t *testing.T) {
	root, dir, cold, want := warm(t, app)
	c := openDisk(t, dir, "gocpp-1")
	n := 0
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		name, ok := strings.CutSuffix(d.Name(), "-"+diskKind)
		if err != nil || d.IsDir() || !ok {
			return nil
		}
		var key Hash
		if _, err := hex.Decode(key[:], []byte(name)); err != nil {
			t.Fatal(err)
		}
		data, ok := c.Get(diskKind, key)
		if !ok {
			t.Fatalf("no entry %s", name)
		}
		s, err := decodeSummary(data)
		if err != nil {
			t.Fatal(err)
		}
		if !s.HasExport {
			return nil
		}
		s.Export = []byte("not export data")
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(s); err != nil {
			t.Fatal(err)
		}
		if err := c.Set(diskKind, key, buf.Bytes()); err != nil {
			t.Fatal(err)
		}
		n++
		return nil
	})
	if n == 0 {
		t.Fatal("no entries with export data")
	}
	b := serverRun(t, diskCfg(c), root)
	if got := b.view("."); got != want {
		t.Errorf("with bad export data:\n%s\nwant:\n%s", got, want)
	}
	if hits, _, _ := b.p.DiskStats(); hits != 0 || b.checks() != cold {
		t.Errorf("%d summaries from disk, %d checks; want 0 and %d", hits, b.checks(), cold)
	}
	b.p.Close()
	d := serverRun(t, diskCfg(c), root)
	if got := d.view("."); got != want || d.checks() != 1 {
		t.Errorf("after the entries were written again: %d checks, view:\n%s", d.checks(), got)
	}
}

// TestDiskConcurrent: two servers with the same cache folder start at the
// same time (each with its own Cache), and write the same entries; a third
// one then loads them.
func TestDiskConcurrent(t *testing.T) {
	root, dir := writeTree(t, app), t.TempDir()
	runs := []*world{
		serverRun(t, diskCfg(openDisk(t, dir, "gocpp-1")), root),
		serverRun(t, diskCfg(openDisk(t, dir, "gocpp-1")), root),
	}
	views := make([]string, len(runs))
	var wg sync.WaitGroup
	for _, w := range runs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snap, release := w.s.Snapshot()
			defer release()
			for _, d := range []string{"shapes", "geom", "."} {
				if _, err := snap.Package(context.Background(), filepath.Join(root, d)); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	for i, w := range runs {
		views[i] = w.view(".")
		w.p.Close()
	}
	if views[0] != views[1] {
		t.Errorf("the two runs differ:\n%s\n\n%s", views[0], views[1])
	}
	c := serverRun(t, diskCfg(openDisk(t, dir, "gocpp-1")), root)
	if got := c.view("."); got != views[0] || c.checks() != 1 {
		t.Errorf("third run: %d checks, view:\n%s\nwant:\n%s", c.checks(), got, views[0])
	}
}

// opaqueApp is a module whose package lib has types from a C header in its
// API, which main uses.
var opaqueApp = map[string]string{
	"gocpp.mod": "module example.com/app\n",
	"main.gp": `package main

import "example.com/app/lib"

func main() {
	w := lib.Open(lib.Default)
	var h lib.Handle
	a := lib.App{N: 1}
	_, _, _ = w, h, a.Win
	lib.Use(lib.Default, 2)
	_ = lib.Pick[int](1, lib.Default)
}
`,
	"lib/lib.gp": `package lib

import "<SDL.h>"

type Handle SDL.Uint32

type App struct {
	Win *SDL.SDL_Window
	N   int
}

var Default *SDL.SDL_Window

func Open(w *SDL.SDL_Window) *SDL.SDL_Renderer { return nil }

func Pick[T any](x T, w *SDL.SDL_Window) *SDL.SDL_Renderer { return nil }

func Use(w *SDL.SDL_Window, n int) {
	var local *SDL.SDL_Renderer
	_ = local
}
`,
}

// A quickProbe is a namespace probe that answers at once (every name is no
// namespace, or an include error), and counts its calls.
type quickProbe struct {
	mu     sync.Mutex
	calls  int
	incErr bool
}

func (q *quickProbe) probe(_ context.Context, _ build.CXX, _ string, includes, cands []string) (interop.Result, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.calls++
	r := interop.Result{Kinds: make(map[string]interop.Kind)}
	if q.incErr {
		r.IncludeErrors = []interop.IncludeError{{Index: 0, Include: includes[0], Kind: interop.HeaderNotFound, Msg: "'SDL.h' file not found"}}
		return r, nil
	}
	for _, c := range cands {
		r.Kinds[c] = interop.NotNamespace
	}
	return r, nil
}

// probeCfg returns diskCfg with a fake C++ compiler and the probe q.
func probeCfg(c *filecache.Cache, q *quickProbe) Config {
	cfg := diskCfg(c)
	cfg.CXX = func(context.Context) (build.CXX, error) {
		return build.CXX{Path: "/fake/c++", Family: build.Clang, Version: "clang version 99"}, nil
	}
	cfg.Probe = q.probe
	return cfg
}

// settled returns the view of main once the background work of the probe
// is done: the stale packages are checked again in a new snapshot, as the
// server does after Config.OnProbe.
func (w *world) settled() string {
	w.t.Helper()
	for range 10 {
		pkg := w.pkg(".")
		if !pkg.Provisional() && !pkg.Stale() {
			return w.view(".")
		}
		w.p.prober.wait()
		if pkg.Stale() {
			if err := w.s.Recheck([]string{w.root, filepath.Join(w.root, "lib")}); err != nil {
				w.t.Fatal(err)
			}
		}
	}
	w.t.Fatal("the probe never settled")
	return ""
}

// TestDiskOpaque: the side table of opaque types. main sees the C++ types
// of lib's API (hover) the same way whether lib came from source (a check
// of the whole program, as gocpp check does), from its summary in memory,
// or from its summary on disk; with the same diagnostics.
func TestDiskOpaque(t *testing.T) {
	root, dir := writeTree(t, opaqueApp), t.TempDir()
	q := &quickProbe{}
	a := serverRun(t, probeCfg(openDisk(t, dir, "gocpp-1"), q), root)
	memory := a.settled()
	for _, want := range []string{
		"Open: func example.com/app/lib.Open(w invalid type) invalid type | func(w *SDL.SDL_Window) *SDL.SDL_Renderer",
		"Use: func example.com/app/lib.Use(w invalid type, n int) | func(w *SDL.SDL_Window, n int)",
		"Pick: func example.com/app/lib.Pick[T any](x T, w invalid type) invalid type | func[T any](x T, w *SDL.SDL_Window) *SDL.SDL_Renderer",
		"Default: var example.com/app/lib.Default invalid type | *SDL.SDL_Window",
		"Handle: type example.com/app/lib.Handle invalid type | SDL.Uint32",
		"Win: field Win invalid type | *SDL.SDL_Window",
	} {
		if !strings.Contains(memory, want) {
			t.Errorf("from memory: no %q in\n%s", want, memory)
		}
	}
	if !strings.HasPrefix(memory, "\n--\n") {
		t.Errorf("diagnostics:\n%s", memory)
	}

	// From source: the whole program, as gocpp check checks it.
	cfg := probeCfg(nil, &quickProbe{})
	prog := load.Load(load.Config{Dir: root, Stdlib: cfg.Stdlib, StdlibDir: "$GOCPP/stdlib", GoStd: cfg.GoStd,
		GOOS: cfg.GOOS, GOARCH: cfg.GOARCH, AllErrors: true}, ".")
	r := check.Check(prog, check.Config{Phase: cfg.Phase, GOOS: cfg.GOOS, GOARCH: cfg.GOARCH, CXX: cfg.CXX, Probe: cfg.Probe})
	byTypes := make(map[*types.Package]*check.Package)
	for _, cp := range r.Packages {
		byTypes[cp.Types] = cp
	}
	main := r.Package(prog.Roots[0])
	readFile := func(name string) []byte { data, _ := os.ReadFile(name); return data }
	source := describe(prog.Fset, main.Info, main.Types, func(obj types.Object) string {
		if v, ok := obj.(*types.Var); ok {
			obj = v.Origin()
		}
		cp := byTypes[obj.Pkg()]
		if _, ok := obj.(*types.Func); ok {
			return funcSpellings(prog.Fset, cp, readFile)[obj]
		}
		te := cp.Opaque.TypeExpr(obj)
		if te == nil {
			return ""
		}
		t, _ := exprText(prog.Fset, te, readFile)
		return t
	})
	if prog.Errors.HasErrors() || main.Errors.HasErrors() {
		t.Errorf("gocpp check has errors: %v %v", prog.Errors.Diagnostics(), main.Errors.Diagnostics())
	}
	if got := strings.TrimPrefix(memory, "\n--\n"); got != source {
		t.Errorf("from memory:\n%s\nfrom source:\n%s", got, source)
	}

	// From disk: the next run checks only main; lib's summary asks the
	// probe again in the background, and stays.
	a.p.Close()
	q2 := &quickProbe{}
	b := serverRun(t, probeCfg(openDisk(t, dir, "gocpp-1"), q2), root)
	if got := b.view("."); got != memory {
		t.Errorf("from disk:\n%s\nfrom memory:\n%s", got, memory)
	}
	if got := b.settled(); got != memory || b.checks() != 1 {
		t.Errorf("from disk, settled: %d checks (want 1), view:\n%s", b.checks(), got)
	}
	q2.mu.Lock()
	calls := q2.calls
	q2.mu.Unlock()
	if calls == 0 {
		t.Error("the summary from disk didn't ask the probe")
	}
}

// TestDiskProbeChanged: a summary on disk whose probe answers differently
// now (the header is gone) is stale once the answer comes, and its package
// is checked again from source, with the new errors.
func TestDiskProbeChanged(t *testing.T) {
	root, dir := writeTree(t, opaqueApp), t.TempDir()
	a := serverRun(t, probeCfg(openDisk(t, dir, "gocpp-1"), &quickProbe{}), root)
	a.settled()
	a.p.Close()
	b := serverRun(t, probeCfg(openDisk(t, dir, "gocpp-1"), &quickProbe{incErr: true}), root)
	b.settled()
	if b.checks() < 2 {
		t.Errorf("%d checks, want main and lib", b.checks())
	}
	if errs := b.errs(b.pkg("lib")); !strings.Contains(errs, "SDL.h") {
		t.Errorf("lib's errors after the probe: %q", errs)
	}
}

// TestOpaqueAPIHash: the side table is part of the API hash: an edit that
// changes only a C++ type of the API checks the importers again; an edit of
// a function body doesn't.
func TestOpaqueAPIHash(t *testing.T) {
	q := &quickProbe{}
	w := serverRun(t, probeCfg(nil, q), writeTree(t, opaqueApp))
	w.settled()
	lib := w.pkg("lib")
	src := opaqueApp["lib/lib.gp"]
	w.open("lib/lib.gp", strings.Replace(src, "_ = local", "_ = local\n\t_ = n", 1))
	w.settled()
	body := w.pkg("lib")
	if body.APIHash != lib.APIHash {
		t.Error("a body edit changed the API hash")
	}
	before := w.checks()
	w.edit("lib/lib.gp", strings.Replace(src, "var Default *SDL.SDL_Window", "var Default *SDL.SDL_Renderer", 1))
	got := w.settled()
	if w.pkg("lib").APIHash == lib.APIHash {
		t.Error("a change of a C++ type of the API left the API hash the same")
	}
	if w.checks()-before < 2 {
		t.Errorf("%d checks after the edit, want lib and main", w.checks()-before)
	}
	if !strings.Contains(got, "Default: var example.com/app/lib.Default invalid type | *SDL.SDL_Renderer") {
		t.Errorf("main doesn't see the new type:\n%s", got)
	}
}

// TestDiskImportErrors: imports with errors (a folder without .gp files, a
// package with a syntax error, one with a type error) give the importer the
// same diagnostics from disk as from source.
func TestDiskImportErrors(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":    "module example.com/app\n",
		"main.gp":      "package main\n\nimport (\n\t\"example.com/app/bad\"\n\t\"example.com/app/empty\"\n\t\"example.com/app/typed\"\n)\n\nfunc main() { bad.F(); empty.F(); typed.F() }\n",
		"empty/x.txt":  "no .gp files\n",
		"bad/bad.gp":   "package bad\n\nfunc F() {\n",
		"typed/typ.gp": "package typed\n\nfunc F() int { return \"s\" }\n",
	}
	root, dir, cold, want := warm(t, files)
	if !strings.Contains(want, "no .gp files") {
		t.Fatalf("the cold start:\n%s", want)
	}
	b := serverRun(t, diskCfg(openDisk(t, dir, "gocpp-1")), root)
	if got := b.view("."); got != want {
		t.Errorf("from disk:\n%s\nfrom source:\n%s", got, want)
	}
	if b.checks() != 1 {
		t.Errorf("%d checks from disk, want 1 (the cold start made %d)", b.checks(), cold)
	}
}
