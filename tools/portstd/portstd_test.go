package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"
)

// The tests never use the network or the real cache: a fixture tarball with a
// tiny fake Go tree is built here, and served from a local test server where a
// download is the point.

func TestMain(m *testing.M) {
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}

const fixtureVersion = "go1.25.0"

// fixtureFiles is the fake Go tree: src/<pkg>/<file>.
var fixtureFiles = map[string]string{
	"VERSION": fixtureVersion + "\ntime 2026-01-01T00:00:00Z\n",
	"LICENSE": "BSD\n",

	"src/fx/basic/a.go":         "// Copyright X.\n\n// Package basic is a fixture.\npackage basic\n\nfunc A() int { return 1 }\n",
	"src/fx/basic/a_test.go":    "package basic\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n",
	"src/fx/basic/b_linux.go":   "package basic\n\nfunc OS() string { return \"linux\" }\n",
	"src/fx/basic/b_windows.go": "package basic\n\nfunc OS() string { return \"windows\" }\n",
	"src/fx/basic/c_amd64.go":   "package basic\n\nfunc Fast(x int) int // in asm_amd64.s\n",
	"src/fx/basic/c_generic.go": "// Copyright X.\n\n//go:build !amd64 && !arm64\n\npackage basic\n\nfunc Fast(x int) int { return x }\n",
	"src/fx/basic/asm_amd64.s":  "TEXT ·Fast(SB),$0\n\tRET\n",
	"src/fx/basic/d.go":         "//go:build purego\n\npackage basic\n\nconst D = 1\n",
	"src/fx/basic/e.go":         "//go:build !purego\n\npackage basic\n\nconst E = 1\n",
	"src/fx/basic/f.go":         "//go:build ignore\n\npackage main\n\nfunc main() {}\n",
	"src/fx/basic/g.go":         "// Copyright X.\n\n//go:build (linux || windows) && !race && purego\n\npackage basic\n\nconst G = 1\n",
	"src/fx/basic/h.go":         "//go:build unix || windows\n// +build unix windows\n\npackage basic\n\nconst H = 1\n",
	"src/fx/basic/i.go":         "//go:build cgo\n\npackage basic\n\nconst I = 1\n",
	"src/fx/basic/j.go":         "package basic\n\n// #include <stdio.h>\nimport \"C\"\n\nconst J = 1\n",
	"src/fx/basic/k.go":         "//go:build go1.99\n\npackage basic\n\nconst K = 1\n",
	"src/fx/basic/l.go":         "//go:build go1.20 && gc\n\npackage basic\n\nconst L = 1\n",
	"src/fx/basic/README":       "not Go\n",

	"src/fx/link/link.go": `package link

import _ "unsafe"

// Nanotime is the clock.
//
//go:linkname runtime_nanotime runtime.nanotime
func runtime_nanotime() int64

//go:linkname other other.Thing
func other() int

//go:linkname pushed runtime.pushed
func pushed() int { return 1 }

//go:linkname Exported
func Exported() {}

// Todo has no native yet.
func todo(x int)

//go:nosplit
func ns() {}

func asmfunc(x int) int

//go:linkname badvar runtime.badvar
var badvar int

func Use() int { return int(runtime_nanotime()) + other() + pushed() }
`,
	"src/fx/link/emb.go": "package link\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar data string\n",

	"src/fx/dep/dep.go":           "package dep\n\nimport (\n\t\"fx/basic\"\n\t\"fx/leaf\"\n\t\"runtime\"\n\t\"unsafe\"\n)\n\nvar _ = basic.A\nvar _ = leaf.X\nvar _ = runtime.GOOS\nvar _ unsafe.Pointer\n",
	"src/fx/leaf/leaf.go":         "package leaf\n\nconst X = 1\n",
	"src/fx/onlywin/w_windows.go": "package onlywin\n\nconst W = 1\n",
}

// fixtureTarball returns the tarball of files (under go/) and its SHA-256.
func fixtureTarball(t *testing.T, files map[string]string, extra ...tar.Header) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		h := &tar.Header{Name: "go/" + n, Mode: 0o644, Size: int64(len(files[n])), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		io.WriteString(tw, files[n])
	}
	for _, h := range extra {
		h := h
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		io.WriteString(tw, strings.Repeat("x", int(h.Size)))
	}
	tw.Close()
	zw.Close()
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:])
}

func fixturePin(sha string) Pin {
	return Pin{Version: fixtureVersion, URL: "https://example.invalid/go1.25.0.src.tar.gz", SHA256: sha}
}

// fixtureConfig is a configuration over a MapFS.
func fixtureConfig(t *testing.T, pin Pin, extra map[string]string) *Config {
	t.Helper()
	m := fstest.MapFS{
		"PINS":         {Data: []byte(pin.text(""))},
		"ports.txt":    {Data: []byte("default os=linux arch=generic tags=purego\nfx/basic\nfx/leaf\n")},
		"natives.txt":  {Data: []byte("runtime.nanotime gp::rt::nanotime\nother.Thing local\nfx/link.todo TODO: write it\n")},
		"boundary.txt": {Data: []byte("runtime 9.2 the runtime\n")},
		"tiers.txt":    {Data: []byte("t: fx/dep\n")},
	}
	for n, c := range extra {
		m[n] = &fstest.MapFile{Data: []byte(c)}
	}
	cfg, err := LoadConfig(m)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// fixtureEnv returns a Porter over the extracted fixture tree.
func fixtureEnv(t *testing.T, extra map[string]string) (*Porter, *Source) {
	t.Helper()
	data, sha := fixtureTarball(t, fixtureFiles)
	pin := fixturePin(sha)
	cache := t.TempDir()
	src := &Source{Pin: pin, Cache: cache}
	if err := os.WriteFile(src.tarball(), data, 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := src.Root()
	if err != nil {
		t.Fatal(err)
	}
	return &Porter{Root: root, Cfg: fixtureConfig(t, pin, extra)}, src
}

func linuxOpts() Options {
	return Options{OS: []string{"linux"}, Arch: "generic", Tags: []string{"purego"}}
}

func takenNames(s *Selection) []string {
	var out []string
	for _, f := range s.Taken {
		out = append(out, f.Name)
	}
	return out
}

func TestSelectLinux(t *testing.T) {
	p, _ := fixtureEnv(t, nil)
	sel, err := selectPackage(p.Root, p.Cfg.Pin, "fx/basic", linuxOpts())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.go", "b_linux.go", "c_generic.go", "d.go", "g.go", "h.go", "l.go"}
	if got := takenNames(sel); !slices.Equal(got, want) {
		t.Errorf("taken %v, want %v", got, want)
	}
	reasons := make(map[string]string)
	for _, s := range sel.Skipped {
		reasons[s.File] = s.Reason
	}
	for file, sub := range map[string]string{
		"b_windows.go": "file name excludes GOOS=linux",
		"c_amd64.go":   "file name excludes",
		"asm_amd64.s":  "assembly",
		"e.go":         "//go:build !purego is false",
		"f.go":         "//go:build ignore is false",
		"i.go":         "//go:build cgo is false",
		"j.go":         "cgo",
		"k.go":         "go1.99",
	} {
		if !strings.Contains(reasons[file], sub) {
			t.Errorf("reason for %s = %q, want it to contain %q", file, reasons[file], sub)
		}
	}
	if _, ok := reasons["README"]; ok {
		t.Error("README is not a source file and must not be listed")
	}
	if sel.Tests != 1 || len(sel.Asm) != 0 {
		t.Errorf("tests %d asm %v, want 1 test file and no kept assembly", sel.Tests, sel.Asm)
	}
	if sel.Name != "basic" {
		t.Errorf("name %q", sel.Name)
	}
}

func TestSelectWindowsAndUnion(t *testing.T) {
	p, _ := fixtureEnv(t, nil)
	o := linuxOpts()
	o.OS = []string{"windows"}
	sel, err := selectPackage(p.Root, p.Cfg.Pin, "fx/basic", o)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := takenNames(sel), []string{"a.go", "b_windows.go", "c_generic.go", "d.go", "g.go", "h.go", "l.go"}; !slices.Equal(got, want) {
		t.Errorf("windows: taken %v, want %v", got, want)
	}
	o.OS = []string{"linux", "windows"}
	sel, err = selectPackage(p.Root, p.Cfg.Pin, "fx/basic", o)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range sel.Taken {
		switch f.Name {
		case "b_linux.go":
			if !slices.Equal(f.For, []string{"linux"}) {
				t.Errorf("b_linux.go for %v", f.For)
			}
		case "b_windows.go":
			if !slices.Equal(f.For, []string{"windows"}) {
				t.Errorf("b_windows.go for %v", f.For)
			}
		case "a.go":
			if !slices.Equal(f.For, []string{"linux", "windows"}) {
				t.Errorf("a.go for %v", f.For)
			}
		}
	}
	// A package that only one system has files for: fine in a union, an error alone.
	o.OS = []string{"linux", "windows"}
	if _, err := selectPackage(p.Root, p.Cfg.Pin, "fx/onlywin", o); err != nil {
		t.Errorf("union: %v", err)
	}
	o.OS = []string{"linux"}
	if _, err := selectPackage(p.Root, p.Cfg.Pin, "fx/onlywin", o); err == nil || !strings.Contains(err.Error(), "no Go files") {
		t.Errorf("linux alone: error %v", err)
	}
	if _, err := selectPackage(p.Root, p.Cfg.Pin, "fx/nothere", o); err == nil {
		t.Error("a missing package must fail")
	}
}

func TestSelectArchAndTags(t *testing.T) {
	p, _ := fixtureEnv(t, nil)
	// With the real architecture the assembly variant is chosen: this is why the default is "generic".
	o := Options{OS: []string{"linux"}, Arch: "amd64", Tags: []string{"purego"}}
	sel, err := selectPackage(p.Root, p.Cfg.Pin, "fx/basic", o)
	if err != nil {
		t.Fatal(err)
	}
	got := takenNames(sel)
	if !slices.Contains(got, "c_amd64.go") || slices.Contains(got, "c_generic.go") {
		t.Errorf("amd64: taken %v", got)
	}
	if !slices.Equal(sel.Asm, []string{"asm_amd64.s"}) {
		t.Errorf("amd64: asm %v", sel.Asm)
	}
	// Without purego, e.go is in and d.go is out.
	o = Options{OS: []string{"linux"}, Arch: "generic"}
	sel, _ = selectPackage(p.Root, p.Cfg.Pin, "fx/basic", o)
	got = takenNames(sel)
	if !slices.Contains(got, "e.go") || slices.Contains(got, "d.go") || slices.Contains(got, "g.go") {
		t.Errorf("no purego: taken %v", got)
	}
}

func TestRewriteBuildLine(t *testing.T) {
	env := tagEnv{arch: "generic", tags: []string{"purego"}, release: 25}
	for _, tc := range []struct {
		line, want string
		goos       []string
	}{
		{"//go:build purego", "", []string{"linux"}},
		{"//go:build !amd64 && !arm64", "", []string{"linux"}},
		{"//go:build linux", "//go:build linux", []string{"linux"}},
		{"//go:build (linux || windows) && !race && purego", "//go:build linux || windows", []string{"linux"}},
		{"//go:build unix || windows", "//go:build unix || windows", []string{"linux"}},
		{"//go:build !purego || linux", "//go:build linux", []string{"linux"}},
		{"//go:build go1.20 && gc && !cgo", "", []string{"linux"}},
		{"//go:build !windows && !(amd64 && purego)", "//go:build !windows", []string{"linux"}},
		{"//go:build (unix && !purego) || linux", "//go:build linux", []string{"linux"}},
		{"//go:build windows || (purego && !ios)", "//go:build windows || !ios", []string{"linux"}},
		{"//go:build !race", "", []string{"linux", "windows"}},
	} {
		got, err := env.rewriteBuildLine(tc.line, tc.goos)
		if err != nil {
			t.Errorf("%s: %v", tc.line, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s -> %q, want %q", tc.line, got, tc.want)
		}
	}
	// A line that is false for a system the file was kept for is a bug of the tool, and says so.
	if _, err := env.rewriteBuildLine("//go:build windows", []string{"linux"}); err == nil || !strings.Contains(err.Error(), "internal error") {
		t.Errorf("a false line: %v", err)
	}
	if _, err := env.rewriteBuildLine("//go:build amd64", []string{"linux"}); err == nil {
		t.Error("amd64 is false for the generic architecture")
	}
}

// portFile returns the content of a ported file.
func portFile(t *testing.T, r *Result, name string) string {
	t.Helper()
	for _, f := range r.Files {
		if f.Name == name {
			return string(f.Content)
		}
	}
	t.Fatalf("no file %s in the port (have %v)", name, func() []string {
		var n []string
		for _, f := range r.Files {
			n = append(n, f.Name)
		}
		return n
	}())
	return ""
}

func TestPortBasicFiles(t *testing.T) {
	p, _ := fixtureEnv(t, nil)
	r, err := p.Port("fx/basic", linuxOpts())
	if err != nil {
		t.Fatal(err)
	}
	a := portFile(t, r, "a.gp")
	if !strings.HasPrefix(a, "// Code ported from Go 1.25.0 src/fx/basic/a.go; DO NOT EDIT by hand; see PORT.md.\n\n// Copyright X.\n") {
		t.Errorf("a.gp starts with:\n%s", a)
	}
	// d.go: the line says purego; it is decided, so the line is gone; the file has no blank-line pile-up.
	d := portFile(t, r, "d.gp")
	if strings.Contains(d, "go:build") || strings.Contains(d, "\n\n\n") {
		t.Errorf("d.gp:\n%s", d)
	}
	// g.go keeps the operating system part; c_generic.go loses its line.
	if g := portFile(t, r, "g.gp"); !strings.Contains(g, "\n//go:build linux || windows\n\npackage basic") {
		t.Errorf("g.gp:\n%s", g)
	}
	if c := portFile(t, r, "c_generic.gp"); strings.Contains(c, "go:build") || !strings.Contains(c, "// Copyright X.\n\npackage basic") {
		t.Errorf("c_generic.gp:\n%s", c)
	}
	// h.go: // +build line goes with the //go:build line staying.
	if h := portFile(t, r, "h.gp"); strings.Contains(h, "+build") || !strings.Contains(h, "//go:build unix || windows") {
		t.Errorf("h.gp:\n%s", h)
	}
	if len(r.Blockers()) != 0 {
		t.Errorf("blockers: %v", r.Blockers())
	}
	for _, f := range r.Files {
		if !strings.HasSuffix(f.Name, ".gp") {
			t.Errorf("file %s is not renamed to .gp", f.Name)
		}
	}
}

func TestPortLinkname(t *testing.T) {
	p, _ := fixtureEnv(t, nil)
	r, err := p.Port("fx/link", linuxOpts())
	if err != nil {
		t.Fatal(err)
	}
	link := portFile(t, r, "link.gp")
	if strings.Contains(link, "go:linkname") {
		t.Errorf("a linkname directive is left:\n%s", link)
	}
	for _, want := range []string{
		// a doc comment keeps its text; the marker is the last line, after a blank comment line
		"// Nanotime is the clock.\n//\n//gocpp:native gp::rt::nanotime\nfunc runtime_nanotime() int64\n",
		// no doc comment: the marker line is the doc, "local" has no argument
		"\n//gocpp:native\nfunc other() int\n",
		"func pushed() int { return 1 }",
		"func Exported() {}",
		"//go:nosplit\nfunc ns() {}",
	} {
		if !strings.Contains(link, want) {
			t.Errorf("link.gp lacks %q:\n%s", want, link)
		}
	}
	if len(r.Natives) != 2 {
		t.Errorf("natives %+v", r.Natives)
	}
	byKind := make(map[string][]Finding)
	for _, f := range r.Findings {
		byKind[f.Kind] = append(byKind[f.Kind], f)
	}
	for kind, n := range map[string]int{
		"linkname-push":   1,
		"linkname-export": 1,
		"linkname-var":    1,
		"native-todo":     1,
		"asm":             1,
		"embed":           1,
		"go:nosplit":      1,
	} {
		if len(byKind[kind]) != n {
			t.Errorf("findings of kind %s: %d, want %d (%v)", kind, len(byKind[kind]), n, byKind[kind])
		}
	}
	for _, kind := range []string{"linkname-var", "native-todo", "asm", "embed"} {
		for _, f := range byKind[kind] {
			if !f.Blocker {
				t.Errorf("%s must be a blocker", kind)
			}
		}
	}
	for _, kind := range []string{"linkname-push", "linkname-export", "go:nosplit"} {
		for _, f := range byKind[kind] {
			if f.Blocker {
				t.Errorf("%s must not be a blocker", kind)
			}
		}
	}
	if f := byKind["asm"]; len(f) == 1 && !strings.Contains(f[0].Detail, "asmfunc") {
		t.Errorf("the asm finding is about %q", f[0].Detail)
	}
	// "unsafe" is imported but not used: no finding. The port still parses and is gofmt-clean.
	for _, f := range r.Findings {
		if f.Kind == "format" {
			t.Errorf("not gofmt-clean: %v", f)
		}
	}
}

func TestPatchRules(t *testing.T) {
	rules := `# patch for fx/basic
@note The value of A is 2 here, not 1.
@skip h.go the file is not needed
@file a.go
@why A returns 2 to show a patch.
@replace
func A() int { return 1 }
@with
func A() int { return 2 }
@end
`
	p, _ := fixtureEnv(t, map[string]string{"patches/fx__basic.rules": rules})
	r, err := p.Port("fx/basic", linuxOpts())
	if err != nil {
		t.Fatal(err)
	}
	if a := portFile(t, r, "a.gp"); !strings.Contains(a, "return 2 }") {
		t.Errorf("a.gp:\n%s", a)
	}
	if len(r.Patches) != 1 || r.Patches[0].File != "a.go" || !strings.Contains(r.Patches[0].Why, "show a patch") {
		t.Errorf("patches %+v", r.Patches)
	}
	for _, f := range r.Files {
		if f.Name == "h.gp" {
			t.Error("@skip did not skip h.go")
		}
	}
	if !slices.ContainsFunc(r.Skipped, func(s Skip) bool {
		return s.File == "h.go" && strings.Contains(s.Reason, "patch: the file is not needed")
	}) {
		t.Errorf("skipped: %+v", r.Skipped)
	}
	md := portMD(r)
	for _, want := range []string{"A returns 2 to show a patch.", "The value of A is 2 here", "patches/fx__basic.rules"} {
		if !strings.Contains(md, want) {
			t.Errorf("PORT.md lacks %q", want)
		}
	}
}

func TestPatchRulesFail(t *testing.T) {
	for name, tc := range map[string]struct{ rules, want string }{
		"no match": {"@file a.go\n@why x\n@replace\nnot there\n@with\ny\n@end\n", "occurs 0 times"},
		"same":     {"@file a.go\n@why x\n@replace\nfunc A\n@with\nfunc A\n@end\n", ""}, // matches once: fine
		"count":    {"@file a.go\n@why x\n@replace count=2\nfunc A\n@with\nfunc B\n@end\n", "occurs 1 times, the rule says 2"},
		"no file":  {"@file zzz.go\n@why x\n@replace\nA\n@with\nB\n@end\n", "does not take"},
		"no why":   {"@file a.go\n@replace\nA\n@with\nB\n@end\n", "needs a reason"},
		"no end":   {"@file a.go\n@why x\n@replace\nA\n@with\nB\n", "needs @with and @end"},
		"unknown":  {"@frob x\n", "unknown line"},
		"skip no":  {"@skip zzz.go because\n", "does not take"},
		"no with":  {"@file a.go\n@why x\n@replace\nA\n@end\n", "needs @with and @end"},
	} {
		t.Run(name, func(t *testing.T) {
			p, _ := fixtureEnv(t, map[string]string{"patches/fx__basic.rules": tc.rules})
			_, err := p.Port("fx/basic", linuxOpts())
			if tc.want == "" {
				if err != nil {
					t.Fatalf("unexpected %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestDeterministic(t *testing.T) {
	p, _ := fixtureEnv(t, nil)
	var prev map[string]string
	for i := range 3 {
		out := t.TempDir()
		for _, pkg := range []string{"fx/basic", "fx/link"} {
			r, err := p.Port(pkg, linuxOpts())
			if err != nil {
				t.Fatal(err)
			}
			if err := writePort(out, r); err != nil {
				t.Fatal(err)
			}
			if err := updateIndex(out, []*Result{r}); err != nil {
				t.Fatal(err)
			}
		}
		cur := make(map[string]string)
		err := filepath.WalkDir(out, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, _ := os.ReadFile(path)
			rel, _ := filepath.Rel(out, path)
			cur[rel] = string(b)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 && !mapsEqual(prev, cur) {
			t.Fatalf("run %d wrote different bytes than the run before", i)
		}
		prev = cur
	}
	if len(prev) < 8 {
		t.Errorf("only %d files written", len(prev))
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestCheckMode(t *testing.T) {
	p, _ := fixtureEnv(t, nil)
	out := t.TempDir()
	r, err := p.Port("fx/basic", linuxOpts())
	if err != nil {
		t.Fatal(err)
	}
	if err := writePort(out, r); err != nil {
		t.Fatal(err)
	}
	if err := updateIndex(out, []*Result{r}); err != nil {
		t.Fatal(err)
	}
	// A hand-written file without the header is allowed (a native .gp/.cpp of a shim).
	os.WriteFile(filepath.Join(pkgDir(out, "fx/basic"), "native.gp"), []byte("package basic\n"), 0o644)
	os.WriteFile(filepath.Join(pkgDir(out, "fx/basic"), "native.cpp"), []byte("// hand\n"), 0o644)
	if probs := append(checkPort(out, r), checkIndex(out, []*Result{r}, true)...); len(probs) != 0 {
		t.Fatalf("clean tree: %v", probs)
	}

	a := filepath.Join(pkgDir(out, "fx/basic"), "a.gp")
	orig, _ := os.ReadFile(a)

	// a hand edit
	os.WriteFile(a, bytes.Replace(orig, []byte("return 1"), []byte("return 3"), 1), 0o644)
	probs := checkPort(out, r)
	if len(probs) != 1 || !strings.Contains(probs[0], "a.gp") || !strings.Contains(probs[0], "hand edit") || !strings.Contains(probs[0], "return 3") {
		t.Errorf("hand edit: %v", probs)
	}
	// removing the header by hand does not hide the edit
	os.WriteFile(a, bytes.TrimPrefix(orig, []byte(strings.SplitAfter(string(orig), "\n")[0])), 0o644)
	if probs := checkPort(out, r); len(probs) != 1 {
		t.Errorf("header removed: %v", probs)
	}
	// a missing file
	os.Remove(a)
	if probs := checkPort(out, r); len(probs) != 1 || !strings.Contains(probs[0], "missing") {
		t.Errorf("missing: %v", probs)
	}
	os.WriteFile(a, orig, 0o644)

	// a ported file that the port does not write
	stale := filepath.Join(pkgDir(out, "fx/basic"), "stale.gp")
	os.WriteFile(stale, []byte(headerLine(p.Cfg.Pin, "fx/basic", "stale.go")+"\n\npackage basic\n"), 0o644)
	if probs := checkPort(out, r); len(probs) != 1 || !strings.Contains(probs[0], "stale.gp") || !strings.Contains(probs[0], "does not write it any more") {
		t.Errorf("stale: %v", probs)
	}
	os.Remove(stale)

	// PORT.md edited
	md := filepath.Join(pkgDir(out, "fx/basic"), "PORT.md")
	mdOrig, _ := os.ReadFile(md)
	os.WriteFile(md, append(mdOrig, "extra\n"...), 0o644)
	if probs := checkPort(out, r); len(probs) != 1 || !strings.Contains(probs[0], "PORT.md") {
		t.Errorf("PORT.md: %v", probs)
	}
	os.WriteFile(md, mdOrig, 0o644)

	// the index
	idx := filepath.Join(out, indexName)
	idxOrig, _ := os.ReadFile(idx)
	os.WriteFile(idx, bytes.Replace(idxOrig, []byte("files="), []byte("files=9"), 1), 0o644)
	if probs := checkIndex(out, []*Result{r}, true); len(probs) == 0 {
		t.Error("an edited index line is not found")
	}
	os.WriteFile(idx, append(idxOrig, "fx/other go1.25.0 os=linux arch=generic tags=- files=1 natives=0 patches=0\n"...), 0o644)
	if probs := checkIndex(out, []*Result{r}, true); len(probs) != 1 || !strings.Contains(probs[0], "fx/other") {
		t.Errorf("extra index line: %v", probs)
	}
	if probs := checkIndex(out, []*Result{r}, false); len(probs) != 0 {
		t.Errorf("a subset check must ignore other lines: %v", probs)
	}
	os.WriteFile(idx, idxOrig, 0o644)
	if probs := append(checkPort(out, r), checkIndex(out, []*Result{r}, true)...); len(probs) != 0 {
		t.Errorf("restored tree: %v", probs)
	}
}

func TestWriteRemovesStaleFiles(t *testing.T) {
	p, _ := fixtureEnv(t, nil)
	out := t.TempDir()
	r, _ := p.Port("fx/basic", linuxOpts())
	writePort(out, r)
	stale := filepath.Join(pkgDir(out, "fx/basic"), "old.gp")
	os.WriteFile(stale, []byte(headerLine(p.Cfg.Pin, "fx/basic", "old.go")+"\n\npackage basic\n"), 0o644)
	hand := filepath.Join(pkgDir(out, "fx/basic"), "hand.gp")
	os.WriteFile(hand, []byte("package basic\n"), 0o644)
	writePort(out, r)
	if _, err := os.Stat(stale); err == nil {
		t.Error("a ported file the port no longer has must be removed")
	}
	if _, err := os.Stat(hand); err != nil {
		t.Error("a hand-written file must stay")
	}
}

func TestListClosure(t *testing.T) {
	p, _ := fixtureEnv(t, nil)
	lr, err := p.List([]string{"fx/dep"}, func(string) Options { return linuxOpts() })
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"fx/basic", "fx/leaf", "fx/dep"}; !slices.Equal(lr.Order, want) {
		t.Errorf("order %v, want %v", lr.Order, want)
	}
	dep := lr.Entries["fx/dep"]
	if !slices.Equal(dep.Deps, []string{"fx/basic", "fx/leaf", "runtime"}) {
		t.Errorf("deps %v (unsafe is not a dependency)", dep.Deps)
	}
	if lr.Entries["runtime"].Boundary == nil || lr.Entries["runtime"].Boundary.Owner != "9.2" {
		t.Errorf("runtime: %+v", lr.Entries["runtime"])
	}
	if !slices.Equal(dep.Reach, []string{"runtime"}) || !slices.Contains(dep.Blocks, "needs runtime") {
		t.Errorf("reach %v blocks %v", dep.Reach, dep.Blocks)
	}
	var text, md bytes.Buffer
	lr.WriteText(&text, "")
	lr.WriteMarkdown(&md, p.Cfg.Pin, linuxOpts())
	if !strings.Contains(text.String(), "provided another way") || !strings.Contains(md.String(), "| `runtime` | 9.2 |") {
		t.Errorf("text:\n%s\nmarkdown:\n%s", text.String(), md.String())
	}
	// asm and linkname blockers show in the list
	lr, err = p.List([]string{"fx/link"}, func(string) Options { return linuxOpts() })
	if err != nil {
		t.Fatal(err)
	}
	blocks := lr.Entries["fx/link"].Blocks
	for _, want := range []string{"asm", "linkname", "go:embed"} {
		if !slices.Contains(blocks, want) {
			t.Errorf("blocks %v lack %s", blocks, want)
		}
	}
}

func TestConfigErrors(t *testing.T) {
	good := fstest.MapFS{
		"PINS":         {Data: []byte(fixturePin(strings.Repeat("a", 64)).text(""))},
		"ports.txt":    {Data: []byte("default os=linux\n")},
		"natives.txt":  {},
		"boundary.txt": {},
		"tiers.txt":    {},
	}
	if _, err := LoadConfig(good); err != nil {
		t.Fatal(err)
	}
	for file, tc := range map[string]struct{ text, want string }{
		"PINS":         {"version go1.25.0\nurl https://x/y\nsha256 abc\n", "64 hex"},
		"ports.txt":    {"a/b color=red\n", "unknown option"},
		"natives.txt":  {"foo\n", "want:"},
		"boundary.txt": {"onlyone\n", "want:"},
		"tiers.txt":    {"nocolon\n", "want:"},
	} {
		m := fstest.MapFS{}
		for k, v := range good {
			m[k] = v
		}
		m[file] = &fstest.MapFile{Data: []byte(tc.text)}
		if _, err := LoadConfig(m); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want %q", file, err, tc.want)
		}
	}
	m := fstest.MapFS{}
	for k, v := range good {
		m[k] = v
	}
	m["ports.txt"] = &fstest.MapFile{Data: []byte("a/b\ndefault os=windows\n")}
	if _, err := LoadConfig(m); err == nil || !strings.Contains(err.Error(), "default line must come before") {
		t.Errorf("late default: %v", err)
	}
}

// --- the source tarball ---

func TestSourceSHAMismatch(t *testing.T) {
	data, sha := fixtureTarball(t, fixtureFiles)
	wrong := strings.Repeat("0", 64)
	if wrong == sha {
		t.Fatal("unlucky")
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write(data)
	}))
	defer srv.Close()
	pin := Pin{Version: fixtureVersion, URL: srv.URL + "/go1.25.0.src.tar.gz", SHA256: wrong}
	cache := t.TempDir()
	s := &Source{Pin: pin, Cache: cache, Fetch: true}
	_, err := s.Root()
	if err == nil || !strings.Contains(err.Error(), "SHA-256 is "+sha) || !strings.Contains(err.Error(), wrong) {
		t.Fatalf("a wrong hash: error %v", err)
	}
	entries, _ := os.ReadDir(cache)
	if len(entries) != 0 {
		t.Errorf("nothing may be kept after a mismatch, the cache has %d entries", len(entries))
	}

	// the right hash: downloaded once, then cached (no second request, also for a new Source)
	pin.SHA256 = sha
	s = &Source{Pin: pin, Cache: cache, Fetch: true}
	root, err := s.Root()
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "src", "fx", "leaf", "leaf.go")); err != nil || !strings.Contains(string(b), "package leaf") {
		t.Errorf("extracted tree: %v %q", err, b)
	}
	before := hits.Load()
	s2 := &Source{Pin: pin, Cache: cache, Fetch: false}
	if root2, err := s2.Root(); err != nil || root2 != root {
		t.Errorf("second use: %v %s", err, root2)
	}
	if hits.Load() != before {
		t.Error("the cache was not used")
	}
	// a cached tarball that was changed: the hash is checked again before extraction
	os.RemoveAll(root)
	if err := os.WriteFile(s.tarball(), append(data[:len(data):len(data)], 0), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s2.Root(); err == nil || !strings.Contains(err.Error(), "PINS says") {
		t.Errorf("a changed cached tarball: %v", err)
	}
}

func TestSourceOffline(t *testing.T) {
	_, sha := fixtureTarball(t, fixtureFiles)
	cache := t.TempDir()
	s := &Source{Pin: fixturePin(sha), Cache: cache}
	if s.Available() {
		t.Error("an empty cache is not available")
	}
	_, err := s.Root()
	var nc *NoCacheError
	if err == nil || !strings.Contains(err.Error(), "offline") || !strings.Contains(err.Error(), sha) {
		t.Fatalf("error %v", err)
	}
	if e, ok := err.(*NoCacheError); !ok || e.Cache != cache {
		t.Errorf("want a *NoCacheError, got %T (%v)", err, nc)
	}
}

func TestExtractIsCareful(t *testing.T) {
	files := map[string]string{"VERSION": fixtureVersion + "\n", "src/a/a.go": "package a\n", "src/cmd/go/main.go": "package main\n", "api/x.txt": "x", "test/t.go": "package main\n"}
	evil := tar.Header{Name: "go/src/../../escape.txt", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg}
	data, sha := fixtureTarball(t, files, evil)
	cache := t.TempDir()
	s := &Source{Pin: fixturePin(sha), Cache: cache}
	os.WriteFile(s.tarball(), data, 0o644)
	// The name cleans to escape.txt, outside go/: it is not wanted, and nothing is written outside.
	root0, err := s.Root()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Dir(cache), cache, root0, filepath.Dir(root0)} {
		if _, err := os.Stat(filepath.Join(dir, "escape.txt")); err == nil {
			t.Fatalf("escape.txt was written in %s", dir)
		}
	}

	// without the evil entry: only the wanted parts, and a link is skipped
	link := tar.Header{Name: "go/src/a/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}
	data, sha = fixtureTarball(t, files, link)
	cache = t.TempDir()
	s = &Source{Pin: fixturePin(sha), Cache: cache}
	os.WriteFile(s.tarball(), data, 0o644)
	root, err := s.Root()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"VERSION", "src/a/a.go", "test/t.go"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f))); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	for _, f := range []string{"src/cmd/go/main.go", "api/x.txt", "src/a/link"} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(f))); err == nil {
			t.Errorf("%s must not be extracted", f)
		}
	}
	// VERSION must match the pin
	data, sha = fixtureTarball(t, map[string]string{"VERSION": "go1.24.0\n", "src/a/a.go": "package a\n"})
	cache = t.TempDir()
	s = &Source{Pin: fixturePin(sha), Cache: cache}
	os.WriteFile(s.tarball(), data, 0o644)
	if _, err := s.Root(); err == nil || !strings.Contains(err.Error(), "go1.24.0") {
		t.Errorf("a VERSION that is not the pin: %v", err)
	}
}

func TestRepin(t *testing.T) {
	data, sha := fixtureTarball(t, fixtureFiles)
	mux := http.NewServeMux()
	mux.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `[{"version":"go1.25.0","files":[{"filename":"go1.25.0.linux-amd64.tar.gz","kind":"archive","sha256":"%s"},{"filename":"go1.25.0.src.tar.gz","kind":"source","sha256":"%s"}]}]`, strings.Repeat("1", 64), sha)
	})
	mux.HandleFunc("/dl/go1.25.0.src.tar.gz", func(w http.ResponseWriter, r *http.Request) { w.Write(data) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cache := t.TempDir()
	pin, err := Repin(nil, srv.URL+"/list", srv.URL+"/dl", "go1.25.0", cache, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if pin.SHA256 != sha || pin.Version != "go1.25.0" || !strings.HasSuffix(pin.URL, "/dl/go1.25.0.src.tar.gz") {
		t.Errorf("pin %+v", pin)
	}
	if _, err := Repin(nil, srv.URL+"/list", srv.URL+"/dl", "go1.25.9", cache, io.Discard); err == nil || !strings.Contains(err.Error(), "no source archive") {
		t.Errorf("an unknown version: %v", err)
	}
	if _, err := Repin(nil, srv.URL+"/list", srv.URL+"/dl", "latest", cache, io.Discard); err == nil {
		t.Error("a malformed version must fail")
	}
	// a listing hash that the tarball does not have: caught
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/list") {
			fmt.Fprintf(w, `[{"version":"go1.25.0","files":[{"filename":"go1.25.0.src.tar.gz","kind":"source","sha256":"%s"}]}]`, strings.Repeat("2", 64))
			return
		}
		w.Write(data)
	}))
	defer bad.Close()
	if _, err := Repin(nil, bad.URL+"/list", bad.URL+"/dl", "go1.25.0", t.TempDir(), io.Discard); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Errorf("a tarball that does not match the listing: %v", err)
	}
}

// --- the command ---

// cliEnv writes the fixture configuration to a folder and returns the flags that use it.
func cliEnv(t *testing.T, withCache bool) (flags []string, out string) {
	t.Helper()
	data, sha := fixtureTarball(t, fixtureFiles)
	pin := fixturePin(sha)
	dir := t.TempDir()
	for name, text := range map[string]string{
		"PINS":         pin.text(""),
		"ports.txt":    "default os=linux arch=generic tags=purego\nfx/basic\nfx/leaf\n",
		"natives.txt":  "",
		"boundary.txt": "runtime 9.2 the runtime\n",
		"tiers.txt":    "t: fx/dep\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cache := t.TempDir()
	if withCache {
		s := &Source{Pin: pin, Cache: cache}
		os.WriteFile(s.tarball(), data, 0o644)
	}
	out = t.TempDir()
	return []string{"-config", dir, "-cache", cache, "-o", out}, out
}

func runCLI(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestCLIPortAndCheck(t *testing.T) {
	flags, out := cliEnv(t, true)
	code, stdout, stderr := runCLI(append(flags, "-offline")...)
	if code != 0 {
		t.Fatalf("port: exit %d\n%s%s", code, stdout, stderr)
	}
	for _, f := range []string{"fx/basic/a.gp", "fx/basic/PORT.md", "fx/leaf/leaf.gp", "PORTED.txt"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(f))); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	code, stdout, stderr = runCLI(append(flags, "-check")...)
	if code != 0 || !strings.Contains(stdout, "2 package(s) are what the port writes") {
		t.Fatalf("check: exit %d\n%s%s", code, stdout, stderr)
	}
	// a hand edit fails the check, and says which file
	a := filepath.Join(out, "fx", "basic", "a.gp")
	b, _ := os.ReadFile(a)
	os.WriteFile(a, append(b, "// hand\n"...), 0o644)
	code, _, stderr = runCLI(append(flags, "-check")...)
	if code != 1 || !strings.Contains(stderr, "a.gp") || !strings.Contains(stderr, "hand edit") {
		t.Fatalf("check after a hand edit: exit %d\n%s", code, stderr)
	}
	// porting again repairs it
	if code, _, stderr = runCLI(append(flags, "-offline")...); code != 0 {
		t.Fatalf("port again: %d %s", code, stderr)
	}
	if code, _, stderr = runCLI(append(flags, "-check")...); code != 0 {
		t.Fatalf("check after the repair: %d %s", code, stderr)
	}
}

func TestCLICheckSkipsWithoutCache(t *testing.T) {
	flags, _ := cliEnv(t, false)
	code, stdout, _ := runCLI(append(flags, "-check")...)
	if code != 0 || !strings.Contains(stdout, "comparison with Go's source was skipped") || !strings.Contains(stdout, "no go1.25.0-") {
		t.Errorf("exit %d, stdout %q", code, stdout)
	}
	code, _, stderr := runCLI(append(flags, "-check", "-require")...)
	if code != 1 || !strings.Contains(stderr, "-check failed") {
		t.Errorf("-require: exit %d, stderr %q", code, stderr)
	}
	// porting offline without a cache fails clearly
	code, _, stderr = runCLI(append(flags, "-offline")...)
	if code != 1 || !strings.Contains(stderr, "offline") {
		t.Errorf("offline port: exit %d, stderr %q", code, stderr)
	}
}

func TestCLIBlockersRefuse(t *testing.T) {
	flags, out := cliEnv(t, true)
	code, _, stderr := runCLI(append(flags, "-offline", "fx/link")...)
	if code != 1 || !strings.Contains(stderr, "not ported") || !strings.Contains(stderr, "asm") {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "fx", "link")); err == nil {
		t.Error("a package with blockers must not be written")
	}
	if code, _, _ = runCLI(append(flags, "-offline", "-force", "fx/link")...); code != 0 {
		t.Errorf("-force: exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(out, "fx", "link", "link.gp")); err != nil {
		t.Error("-force must write the package")
	}
}

func TestCLIList(t *testing.T) {
	flags, _ := cliEnv(t, true)
	code, stdout, stderr := runCLI(append(flags, "-list", "-tier", "t")...)
	if code != 0 || !strings.Contains(stdout, "fx/dep") || !strings.Contains(stdout, "fx/basic") || !strings.Contains(stdout, "runtime") {
		t.Fatalf("exit %d\n%s%s", code, stdout, stderr)
	}
	if code, _, _ = runCLI(append(flags, "-list")...); code != 1 {
		t.Error("-list without packages must fail")
	}
	if code, _, _ = runCLI(append(flags, "-list", "-tier", "nosuch")...); code != 1 {
		t.Error("an unknown -tier must fail")
	}
}

// --- review round 1 ---

// emptyCache returns the flags of cliEnv with the cache replaced by an empty folder.
func emptyCache(t *testing.T, flags []string) []string {
	t.Helper()
	out := slices.Clone(flags)
	i := slices.Index(out, "-cache")
	out[i+1] = t.TempDir()
	return out
}

func TestCLICheckByHashWithoutCache(t *testing.T) {
	flags, out := cliEnv(t, true)
	if code, _, stderr := runCLI(append(flags, "-offline")...); code != 0 {
		t.Fatalf("port: %d %s", code, stderr)
	}
	noCache := emptyCache(t, flags)
	check := func() (int, string, string) { return runCLI(append(slices.Clone(noCache), "-check")...) }
	code, stdout, stderr := check()
	if code != 0 || !strings.Contains(stdout, "recorded SHA-256") || !strings.Contains(stdout, "2 package(s)") {
		t.Fatalf("clean tree without the cache: exit %d\n%s%s", code, stdout, stderr)
	}
	// a hand edit is found with no copy of Go's source
	a := filepath.Join(out, "fx", "basic", "a.gp")
	orig, _ := os.ReadFile(a)
	os.WriteFile(a, append(slices.Clone(orig), "// hand\n"...), 0o644)
	code, _, stderr = check()
	if code != 1 || !strings.Contains(stderr, "fx/basic/a.gp") || !strings.Contains(stderr, "SHA-256") || !strings.Contains(stderr, "hand edit") {
		t.Fatalf("hand edit without the cache: exit %d\n%s", code, stderr)
	}
	// so is an edit that removes the header, a missing file, and a new ported file
	os.WriteFile(a, bytes.TrimPrefix(orig, []byte(strings.SplitAfter(string(orig), "\n")[0])), 0o644)
	if code, _, _ = check(); code != 1 {
		t.Error("an edit that removes the header is not found")
	}
	os.Remove(a)
	if code, _, stderr = check(); code != 1 || !strings.Contains(stderr, "missing") {
		t.Errorf("missing file: %d %s", code, stderr)
	}
	os.WriteFile(a, orig, 0o644)
	extra := filepath.Join(out, "fx", "basic", "extra.gp")
	os.WriteFile(extra, []byte(headerLine(Pin{Version: fixtureVersion}, "fx/basic", "extra.go")+"\n\npackage basic\n"), 0o644)
	if code, _, stderr = check(); code != 1 || !strings.Contains(stderr, "extra.gp") {
		t.Errorf("new ported file: %d %s", code, stderr)
	}
	os.Remove(extra)
	if code, _, stderr = check(); code != 0 {
		t.Fatalf("restored: %d %s", code, stderr)
	}
	// the list of hashes missing is a failure too
	sums := filepath.Join(out, sumsName)
	b, _ := os.ReadFile(sums)
	os.Remove(sums)
	if code, _, stderr = check(); code != 1 || !strings.Contains(stderr, "no hashes") {
		t.Errorf("no PORTED.sha256: %d %s", code, stderr)
	}
	os.WriteFile(sums, b, 0o644)

	// -require (and GOCPP_REQUIRE_PORTSTD) turn the missing source into an error, hashes or not
	if code, _, stderr = runCLI(append(slices.Clone(noCache), "-check", "-require")...); code != 1 || !strings.Contains(stderr, "-check failed") {
		t.Errorf("-require: %d %s", code, stderr)
	}
	t.Setenv("GOCPP_REQUIRE_PORTSTD", "1")
	if code, _, stderr = check(); code != 1 || !strings.Contains(stderr, "GOCPP_REQUIRE_PORTSTD") {
		t.Errorf("GOCPP_REQUIRE_PORTSTD: %d %s", code, stderr)
	}
	// with the source, the recorded hashes must be the tool's: a consistent hand edit
	// (file and hash both changed) passes the hash check and fails the byte comparison
	os.WriteFile(a, append(slices.Clone(orig), "// hand\n"...), 0o644)
	text, _ := os.ReadFile(sums)
	os.WriteFile(sums, bytes.Replace(text, []byte(hashOf(orig)), []byte(hashOf(append(slices.Clone(orig), "// hand\n"...))), 1), 0o644)
	t.Setenv("GOCPP_REQUIRE_PORTSTD", "")
	if code, _, stderr = check(); code != 0 {
		t.Fatalf("a consistent edit passes the hash check: %d %s", code, stderr)
	}
	if code, _, stderr = runCLI(append(slices.Clone(flags), "-check")...); code != 1 || !strings.Contains(stderr, "hand edit") {
		t.Errorf("with the cache the byte comparison finds it: %d %s", code, stderr)
	}
	os.WriteFile(sums, b, 0o644)
	os.WriteFile(a, orig, 0o644)
	// the recorded hashes themselves are compared with the cache
	os.WriteFile(sums, bytes.Replace(b, []byte(hashOf(orig)), []byte(strings.Repeat("a", 64)), 1), 0o644)
	if code, _, stderr = runCLI(append(slices.Clone(flags), "-check")...); code != 1 || !strings.Contains(stderr, sumsName) {
		t.Errorf("a wrong recorded hash with the cache: %d %s", code, stderr)
	}
}

func TestCLIFetchDoesNotOverrideOffline(t *testing.T) {
	flags, _ := cliEnv(t, false)
	// nothing may download: the pin's URL does not exist, so a download would be an error
	code, stdout, stderr := runCLI(append(flags, "-check", "-fetch", "-offline")...)
	if code != 0 || strings.Contains(stderr, "download") || !strings.Contains(stdout, "skipped") {
		t.Errorf("-fetch -offline: exit %d\n%s%s", code, stdout, stderr)
	}
	t.Setenv("GOCPP_OFFLINE", "1")
	code, stdout, stderr = runCLI(append(flags, "-check", "-fetch")...)
	if code != 0 || strings.Contains(stderr, "download") || !strings.Contains(stdout, "skipped") {
		t.Errorf("-fetch with GOCPP_OFFLINE=1: exit %d\n%s%s", code, stdout, stderr)
	}
}

func TestCLIRefusesHandWrittenFiles(t *testing.T) {
	flags, out := cliEnv(t, true)
	dir := filepath.Join(out, "fx", "basic")
	os.MkdirAll(dir, 0o755)
	clash := filepath.Join(dir, "a.gp") // the port writes a.gp
	stale := filepath.Join(dir, "old.gp")
	stub := filepath.Join(dir, "stub.gp")
	os.WriteFile(clash, []byte("package basic\n\n// mine\n"), 0o644)
	os.WriteFile(stale, []byte("package basic\n\n// old stub by hand\n"), 0o644)
	os.WriteFile(stub, []byte("// Copyright X.\n\n// Code generated by tools/stubgen from go1.25.0's src/fx/basic; DO NOT EDIT.\n\npackage basic\n"), 0o644)

	code, _, stderr := runCLI(append(flags, "-offline", "fx/basic")...)
	if code != 1 || !strings.Contains(stderr, "a.gp: the port overwrites it") || !strings.Contains(stderr, "old.gp: the port does not write it") || strings.Contains(stderr, "stub.gp: ") {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if b, _ := os.ReadFile(clash); !strings.Contains(string(b), "// mine") {
		t.Error("a refused port must not overwrite the hand-written file")
	}
	if _, err := os.Stat(stub); err != nil {
		t.Error("a refused port must not remove the stub")
	}
	if _, err := os.Stat(filepath.Join(dir, "PORT.md")); err == nil {
		t.Error("a refused port must write nothing")
	}
	// -force: overwrites the clash, keeps the stale one with a warning, removes the stub
	code, _, stderr = runCLI(append(flags, "-offline", "-force", "fx/basic")...)
	if code != 0 || !strings.Contains(stderr, "old.gp is hand-written and stays") || !strings.Contains(stderr, "removed the stub") {
		t.Fatalf("-force: exit %d\n%s", code, stderr)
	}
	if b, _ := os.ReadFile(clash); strings.Contains(string(b), "// mine") {
		t.Error("-force must overwrite the clashing file")
	}
	if _, err := os.Stat(stale); err != nil {
		t.Error("-force keeps a hand-written file that the port does not write")
	}
	if _, err := os.Stat(stub); err == nil {
		t.Error("the stub of tools/stubgen must be removed")
	}
	// a stub that appears beside a port later is a check failure
	os.WriteFile(stub, []byte("// Code generated by tools/stubgen from x; DO NOT EDIT.\npackage basic\n"), 0o644)
	if code, _, stderr = runCLI(append(flags, "-check")...); code != 1 || !strings.Contains(stderr, "stub of tools/stubgen") {
		t.Errorf("stub beside a port: %d %s", code, stderr)
	}
	if code, _, stderr = runCLI(append(emptyCache(t, flags), "-check")...); code != 1 || !strings.Contains(stderr, "stub of tools/stubgen") {
		t.Errorf("stub beside a port, by hash: %d %s", code, stderr)
	}
}

// TestCLIOwnFiles: a .gp file that ports.txt lists with an "own" line is gocpp's own file beside the
// port (errors.Wrapf): the port neither refuses nor warns, and keeps it; a listed name that the port
// writes itself is an error; a hand-written file that is not listed is still refused.
func TestCLIOwnFiles(t *testing.T) {
	flags, out := cliEnv(t, true)
	cfgDir := flags[1]
	ports := filepath.Join(cfgDir, "ports.txt")
	b, _ := os.ReadFile(ports)
	os.WriteFile(ports, append(b, "own fx/basic mine.gp\n"...), 0o644)
	dir := filepath.Join(out, "fx", "basic")
	os.MkdirAll(dir, 0o755)
	mine := filepath.Join(dir, "mine.gp")
	os.WriteFile(mine, []byte("package basic\n\n// gocpp's own\n"), 0o644)
	code, _, stderr := runCLI(append(slices.Clone(flags), "-offline", "fx/basic")...)
	if code != 0 || strings.Contains(stderr, "mine.gp") {
		t.Fatalf("an own file beside the port: exit %d\n%s", code, stderr)
	}
	if got, _ := os.ReadFile(mine); !strings.Contains(string(got), "gocpp's own") {
		t.Error("the port must keep the own file")
	}
	// a hand-written file that is not listed is refused
	other := filepath.Join(dir, "other.gp")
	os.WriteFile(other, []byte("package basic\n"), 0o644)
	if code, _, stderr = runCLI(append(slices.Clone(flags), "-offline", "fx/basic")...); code != 1 || !strings.Contains(stderr, "other.gp") || strings.Contains(stderr, "mine.gp") {
		t.Errorf("an unlisted file: exit %d\n%s", code, stderr)
	}
	os.Remove(other)
	// the listed name clashes with a file the port writes
	os.WriteFile(ports, append(b, "own fx/basic a.gp\n"...), 0o644)
	os.WriteFile(filepath.Join(dir, "a.gp"), []byte("package basic\n"), 0o644)
	if code, _, stderr = runCLI(append(slices.Clone(flags), "-offline", "fx/basic")...); code != 1 || !strings.Contains(stderr, "listed as gocpp's own file") {
		t.Errorf("an own name that the port writes: exit %d\n%s", code, stderr)
	}
	// the syntax of the line
	for _, line := range []string{"own fx/basic\n", "own fx/basic sub/x.gp\n", "own fx/basic x.txt\n"} {
		os.WriteFile(ports, append(b, line...), 0o644)
		if code, _, stderr = runCLI(append(slices.Clone(flags), "-offline", "fx/basic")...); code != 1 || !strings.Contains(stderr, "ports.txt") {
			t.Errorf("%q: exit %d\n%s", line, code, stderr)
		}
	}
}

func TestOrphanPatchWarning(t *testing.T) {
	cfg := fixtureConfig(t, fixturePin(strings.Repeat("1", 64)), map[string]string{
		"patches/fx__basic.rules": "# fine\n",
		"patches/fx__basci.rules": "# typo\n",
		"patches/README.txt":      "not rules\n",
	})
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "fx__basci.rules") {
		t.Errorf("warnings: %q", cfg.Warnings)
	}
	cfg = fixtureConfig(t, fixturePin(strings.Repeat("1", 64)), nil)
	if len(cfg.Warnings) != 0 {
		t.Errorf("warnings: %q", cfg.Warnings)
	}
}

func TestPinURLMustBeHTTPS(t *testing.T) {
	good := strings.Repeat("a", 64)
	for _, tc := range []struct {
		url string
		ok  bool
	}{
		{"https://go.dev/dl/go1.25.0.src.tar.gz", true},
		{"http://127.0.0.1:5000/x.tar.gz", true},
		{"http://localhost/x.tar.gz", true},
		{"http://dl.google.com/go/go1.25.0.src.tar.gz", false},
		{"ftp://dl.google.com/x", false},
		{"https://", false},
	} {
		_, err := parsePin(fmt.Sprintf("version go1.25.0\nurl %s\nsha256 %s\n", tc.url, good))
		if (err == nil) != tc.ok {
			t.Errorf("%s: error %v, want ok=%v", tc.url, err, tc.ok)
		}
	}
	if _, err := Repin(nil, "http://example.com/list", "https://dl.example.com", "go1.25.0", t.TempDir(), io.Discard); err == nil || !strings.Contains(err.Error(), "https") {
		t.Errorf("a plain http listing: %v", err)
	}
}

func TestSourceLimitsAndStale(t *testing.T) {
	data, sha := fixtureTarball(t, fixtureFiles)
	// a download over the cap is refused and leaves nothing
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()
	old := maxTarball
	maxTarball = int64(len(data)) - 1
	cache := t.TempDir()
	s := &Source{Pin: Pin{Version: fixtureVersion, URL: srv.URL + "/x.tar.gz", SHA256: sha}, Cache: cache, Fetch: true}
	_, err := s.Root()
	maxTarball = old
	if err == nil || !strings.Contains(err.Error(), "MB") {
		t.Errorf("a download over the cap: %v", err)
	}
	if entries, _ := os.ReadDir(cache); len(entries) != 0 {
		t.Errorf("nothing may be kept: %d entries", len(entries))
	}
	// a cached tarball over the cap is refused
	cache = t.TempDir()
	s = &Source{Pin: fixturePin(sha), Cache: cache}
	os.WriteFile(s.tarball(), data, 0o644)
	maxTarball = int64(len(data)) - 1
	_, err = s.Root()
	maxTarball = old
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("a cached tarball over the cap: %v", err)
	}
	// extraction over the cap is refused (a tarball that unpacks to more than it should)
	oldX := maxExtract
	maxExtract = 10
	_, err = s.Root()
	maxExtract = oldX
	if err == nil || !strings.Contains(err.Error(), "extracted") {
		t.Errorf("extraction over the cap: %v", err)
	}
	if entries, _ := os.ReadDir(cache); len(entries) != 1 { // the tarball only: the half tree is removed
		t.Errorf("a refused extraction leaves its folder: %d entries", len(entries))
	}

	// extract-* and download-* left by a killed process: old ones are removed, young ones stay
	oldDir := filepath.Join(cache, "extract-old")
	youngDir := filepath.Join(cache, "extract-young")
	oldFile := filepath.Join(cache, "download-old")
	for _, d := range []string{oldDir, youngDir} {
		os.MkdirAll(filepath.Join(d, "sub"), 0o755)
	}
	os.WriteFile(oldFile, []byte("x"), 0o644)
	longAgo := time.Now().Add(-2 * staleAge)
	os.Chtimes(oldDir, longAgo, longAgo)
	os.Chtimes(oldFile, longAgo, longAgo)
	if _, err := s.Root(); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{oldDir, oldFile} {
		if _, err := os.Stat(gone); err == nil {
			t.Errorf("%s should have been removed", gone)
		}
	}
	if _, err := os.Stat(youngDir); err != nil {
		t.Error("a young extract-* folder may belong to a running process")
	}

	// a tree whose marker has another hash is not used
	cache = t.TempDir()
	s = &Source{Pin: fixturePin(sha), Cache: cache}
	os.WriteFile(s.tarball(), data, 0o644)
	root, err := s.Root()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, doneMarker), []byte(strings.Repeat("0", 64)+"\n"), 0o644)
	if s.treeComplete() {
		t.Error("a marker with another hash must not count")
	}
}

func TestExtractReadsHashedBytes(t *testing.T) {
	// Root hashes the bytes it extracts: the tarball file changes after the read, not before.
	// A tarball whose bytes differ from the pin is refused, whatever the file looks like.
	data, sha := fixtureTarball(t, fixtureFiles)
	other, _ := fixtureTarball(t, map[string]string{"VERSION": fixtureVersion + "\n", "src/evil/e.go": "package evil\n"})
	cache := t.TempDir()
	s := &Source{Pin: fixturePin(sha), Cache: cache}
	os.WriteFile(s.tarball(), other, 0o644)
	if _, err := s.Root(); err == nil || !strings.Contains(err.Error(), "PINS says") {
		t.Errorf("another tarball: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.tree(), "src", "evil")); err == nil {
		t.Error("nothing of an unverified tarball may be extracted")
	}
	_ = data
}
