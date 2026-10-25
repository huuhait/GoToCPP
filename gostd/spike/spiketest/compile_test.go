// Copyright 2026 The gocpp Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package spiketest holds the compile check of job 16.1f-stubs: the four spike
// packages plus their import closure, built by the real Go toolchain against
// the pure-Go stubs in gostd/spike/stub/.
//
// The stubs live at gostd/spike/stub/runtime, gostd/spike/stub/internal/bytealg
// and gostd/spike/stub/internal/reflectlite. Their import paths in the GC
// engine are runtime, internal/bytealg and internal/reflectlite: the loader
// maps an import of one of those to gostd/spike/stub/<path>. The test builds
// that world explicitly: a temporary module named "std" gets every package of
// the closure at std/<path> (its own import path) and every .go import that
// names a copied package rewritten to std/<path>; imports of the three stubbed
// paths become the stubs. Then `go build` of the four roots compiles the whole
// closure against the stubs. It is a compile and type check, not a link: the
// runtime stub's natives (bodyless functions) and the //go:linkname hooks are
// resolved by the engine, not by the real Go runtime.
package spiketest

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// A recreation of the fields of gostd/spike/closure.json this test reads.
type closureJSON struct {
	Roots    []string `json:"roots"`
	Stubs    []string `json:"stubs"`
	Packages []struct {
		Path   string   `json:"path"`
		Files  []string `json:"files"`
		SFiles []string `json:"s_files"`
	} `json:"packages"`
}

// stubDir maps a stubbed import path to its directory in this repository.
var stubDirs = map[string]string{
	"runtime":              "gostd/spike/stub/runtime",
	"internal/bytealg":     "gostd/spike/stub/internal/bytealg",
	"internal/reflectlite": "gostd/spike/stub/internal/reflectlite",
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// .../gostd/spike/spiketest/compile_test.go -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func goCommand(t *testing.T) string {
	t.Helper()
	if p, err := exec.LookPath("go"); err == nil {
		return p
	}
	p := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(p); err != nil {
		t.Skipf("no go command (PATH and %s)", p)
	}
	return p
}

func TestCompileFourPackagesAgainstStubs(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "gostd", "spike", "closure.json"))
	if err != nil {
		t.Fatalf("read the closure analysis: %v", err)
	}
	var cj closureJSON
	if err := json.Unmarshal(data, &cj); err != nil {
		t.Fatalf("parse the closure analysis: %v", err)
	}
	if len(cj.Roots) != 4 {
		t.Fatalf("closure.json has %d roots, want 4", len(cj.Roots))
	}
	goroot := runtime.GOROOT()
	version, err := os.ReadFile(filepath.Join(goroot, "VERSION"))
	if err != nil {
		t.Fatalf("read GOROOT/VERSION: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(version)), "go1.25") {
		t.Fatalf("GOROOT/VERSION = %q: the analysis is for Go 1.25", strings.TrimSpace(string(version)))
	}

	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module std\n\ngo 1.25\n"), 0o666); err != nil {
		t.Fatal(err)
	}

	// Copy the closure. Mark every copied package first, so the import rewrite
	// sees them all whatever the JSON order is.
	copied := map[string]bool{}
	for _, p := range cj.Packages {
		if _, ok := stubDirs[p.Path]; !ok {
			copied[p.Path] = true
		}
	}
	for _, p := range cj.Packages {
		if _, ok := stubDirs[p.Path]; ok {
			continue // the stubs are copied below
		}
		src := filepath.Join(goroot, "src", filepath.FromSlash(p.Path))
		dst := filepath.Join(tmp, filepath.FromSlash(p.Path))
		if err := os.MkdirAll(dst, 0o777); err != nil {
			t.Fatal(err)
		}
		for _, name := range p.Files {
			if err := copyRewritten(filepath.Join(src, name), filepath.Join(dst, name), copied, cj.Stubs); err != nil {
				t.Fatalf("%s/%s: %v", p.Path, name, err)
			}
		}
		for _, name := range p.SFiles {
			if err := copyFile(filepath.Join(src, name), filepath.Join(dst, name)); err != nil {
				t.Fatalf("%s/%s: %v", p.Path, name, err)
			}
		}
	}

	// Copy the stubs to their import-path directory (runtime, internal/bytealg,
	// internal/reflectlite).
	for path, dir := range stubDirs {
		src := filepath.Join(root, filepath.FromSlash(dir))
		dst := filepath.Join(tmp, filepath.FromSlash(path))
		if err := os.MkdirAll(dst, 0o777); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			t.Fatalf("stub %s: %v", path, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || strings.HasSuffix(name, "_test.go") {
				continue
			}
			if err := copyRewritten(filepath.Join(src, name), filepath.Join(dst, name), copied, cj.Stubs); err != nil {
				t.Fatalf("stub %s/%s: %v", path, name, err)
			}
		}
	}

	// Build. GO111MODULE=on, no network, no workspace, no toolchain download.
	// Two runs: the plain gc constraints, and with the gocpp tag so the runtime
	// stub's //go:linkname push wrappers (hooks_link.go, //go:build gocpp) are
	// compiled too.
	for _, tags := range []string{"", "gocpp"} {
		// -complete=false: the copied sync (and the closure's other packages
		// with forward declarations the runtime supplies, such as internal/abi)
		// is not the standard library any more, so cmd/go no longer applies
		// its special case for "sync", "time" and friends (work/gc.go) and
		// would pass -complete. Go's build allows those bodyless declarations
		// on purpose; this puts the compiler in the same mode.
		args := []string{"build", "-gcflags=all=-complete=false"}
		if tags != "" {
			args = append(args, "-tags", tags)
		}
		for _, r := range cj.Roots {
			args = append(args, "./"+filepath.ToSlash(r))
		}
		cmd := exec.Command(goCommand(t), args...)
		cmd.Dir = tmp
		cmd.Env = append(os.Environ(),
			"GO111MODULE=on",
			"GOWORK=off",
			"GOPROXY=off",
			"GOTOOLCHAIN=local",
			"GOFLAGS=-mod=mod",
			"CGO_ENABLED=0",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("go build (tags %q) of the four packages against the stubs failed: %v\n%s", tags, err, out)
		}
		if len(out) > 0 {
			t.Logf("go build (tags %q):\n%s", tags, out)
		}
	}
}

// copyRewritten copies a Go source file, rewriting the import paths of the
// copied packages to std/<path> and imports of a stub to std/<stub path>.
// Nothing else is touched.
func copyRewritten(src, dst string, copied map[string]bool, stubs []string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	text := string(data)
	// Every import path that is copied or stubbed becomes std/<path>.
	repl := map[string]string{}
	for p := range copied {
		repl[p] = "std/" + p
	}
	for _, s := range stubs {
		repl[s] = "std/" + s
	}
	// Longest first, so internal/foo is not rewritten as internal/f.
	keys := make([]string, 0, len(repl))
	for k := range repl {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	// Only the quoted import path occurrences change; strings elsewhere are
	// left alone because the quoted form with a leading quote is specific
	// enough for these packages.
	for _, k := range keys {
		text = strings.ReplaceAll(text, strconv.Quote(k), strconv.Quote(repl[k]))
	}
	return os.WriteFile(dst, []byte(text), 0o666)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o666)
}

// TestClosureCoversStubs is a guard: the JSON must list the three stubs and
// every package the four roots import, so the compile test copies a complete
// tree. It also names the packages the plan does not name (README §3 step
// 16.1): internal/stringslite, internal/synctest and the internal/sync split.
func TestClosureCoversStubs(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "gostd", "spike", "closure.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cj closureJSON
	if err := json.Unmarshal(data, &cj); err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, p := range cj.Packages {
		paths[p.Path] = true
	}
	for _, want := range []string{
		"container/list", "strings", "strconv", "sort",
		"errors", "io", "iter", "math", "math/bits", "slices", "cmp", "sync",
		"unicode", "unicode/utf8",
		// The packages the plan's stub sentence does not name:
		"internal/abi", "internal/cpu", "internal/goarch", "internal/race",
		"internal/stringslite", "internal/sync", "internal/synctest", "sync/atomic",
	} {
		if !paths[want] {
			t.Errorf("closure.json does not list %s", want)
		}
	}
	// The three stubs are not nodes of the graph: they are replaced, and
	// stubDirs copies them.
	for _, s := range cj.Stubs {
		if paths[s] {
			t.Errorf("closure.json lists the stub %s as a translated package", s)
		}
	}
	if len(paths) != 22 {
		t.Errorf("closure.json lists %d packages, want 22 (4 roots, 18 imported)", len(paths))
	}
}

// TestStubsDeclareTheAnalysisAPI checks that every identifier
// gostd/spike/closure.json says the closure uses is declared by the stub
// source, with the same kind. It is the drift guard between the analysis, the
// document and the stubs: a renamed or dropped identifier fails here before
// the engine does.
func TestStubsDeclareTheAnalysisAPI(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "gostd", "spike", "closure.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cj struct {
		Stubs     []string `json:"stubs"`
		StubUsage []struct {
			Package     string `json:"package"`
			Identifiers []struct {
				Name string `json:"name"`
				Kind string `json:"kind"`
			} `json:"identifiers"`
		} `json:"stub_usage"`
	}
	if err := json.Unmarshal(data, &cj); err != nil {
		t.Fatal(err)
	}
	if len(cj.StubUsage) != 3 {
		t.Fatalf("closure.json has %d stub_usage entries, want 3", len(cj.StubUsage))
	}
	total := 0
	for _, su := range cj.StubUsage {
		dir := filepath.Join(root, filepath.FromSlash(stubDirs[su.Package]))
		have := declaredNames(t, dir)
		total += len(su.Identifiers)
		for _, id := range su.Identifiers {
			if !have[id.Kind+" "+id.Name] {
				t.Errorf("%s: closure.json uses %s %s, but the stub does not declare it", su.Package, id.Kind, id.Name)
			}
		}
	}
	if total != 30 {
		t.Errorf("closure.json lists %d stub identifiers, want 30 (12 bytealg, 16 reflectlite, 2 runtime)", total)
	}
}

// declaredNames returns "kind name" for every package-level declaration of the
// Go files in dir (methods are keyed as "method name"; funcs without a
// receiver as "func name").
func declaredNames(t *testing.T, dir string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				kind := "func"
				if d.Recv != nil {
					kind = "method"
				}
				out[kind+" "+d.Name.Name] = true
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						out["type "+s.Name.Name] = true
					case *ast.ValueSpec:
						kind := "var"
						if d.Tok == token.CONST {
							kind = "const"
						}
						for _, n := range s.Names {
							out[kind+" "+n.Name] = true
						}
					}
				}
			}
		}
	}
	return out
}
