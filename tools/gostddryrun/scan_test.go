package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClassifyLinkname(t *testing.T) {
	for _, tc := range []struct {
		target string
		symbol string
		want   string
	}{
		{"", "func-bodyless", "one-argument"},
		{"example/dep.Var", "var", "variable"},
		{"go:fipsinfo", "var", "linker"},
		{"_cgo_init", "func-bodyless", "linker"},
		{"example/dep.Pulled", "func-bodyless", "pull"},
		{"example/dep.Pushed", "func-body", "push"},
		{"example/dep.Pulled", "unknown", "unknown"},
	} {
		if got := classifyLinkname(tc.target, tc.symbol); got != tc.want {
			t.Errorf("classifyLinkname(%q, %q) = %q, want %q", tc.target, tc.symbol, got, tc.want)
		}
	}
}

func TestScanExample(t *testing.T) {
	tags, dir := fakeContext(t, "linux", "amd64")
	files, err := selectDir(newContext(fakeGOROOT, tags, "gocpp"), dir)
	if err != nil {
		t.Fatal(err)
	}
	scan, err := scanPackage("example", dir, files.GoFiles)
	if err != nil {
		t.Fatal(err)
	}
	if scan.Name != "example" {
		t.Errorf("name = %q", scan.Name)
	}
	if len(scan.Bodyless) != 2 {
		t.Fatalf("bodyless = %v, want pullLocal and bodyless", scan.Bodyless)
	}
	sigs := map[string]string{}
	for _, b := range scan.Bodyless {
		sigs[b.Name] = b.Signature
	}
	if got := sigs["pullLocal"]; got != "func pullLocal()" {
		t.Errorf("pullLocal signature = %q", got)
	}
	if got := sigs["bodyless"]; got != "func bodyless() int" {
		t.Errorf("bodyless signature = %q", got)
	}

	kinds := map[string]Linkname{}
	for _, ln := range scan.Linknames {
		kinds[ln.Local] = ln
	}
	for _, tc := range []struct {
		local, target, kind, decl string
	}{
		{"pullLocal", "example/dep.Pulled", "pull", "func-bodyless"},
		{"pushLocal", "example/dep.Pushed", "push", "func-body"},
		{"oneArg", "", "one-argument", "func-body"},
		{"varLink", "example/dep.Var", "variable", "var"},
		{"linkerSym", "go:fipsinfo", "linker", "var"},
	} {
		ln, ok := kinds[tc.local]
		if !ok {
			t.Errorf("no linkname for %s", tc.local)
			continue
		}
		if ln.Target != tc.target || ln.Kind != tc.kind || ln.Decl != tc.decl {
			t.Errorf("%s = {target:%q kind:%q decl:%q}, want {%q %q %q}",
				tc.local, ln.Target, ln.Kind, ln.Decl, tc.target, tc.kind, tc.decl)
		}
		if ln.Line == 0 || ln.File != "example.go" {
			t.Errorf("%s position = %s:%d", tc.local, ln.File, ln.Line)
		}
	}
	if len(scan.Intrinsics) != 0 {
		t.Errorf("example intrinsics = %v", scan.Intrinsics)
	}
}

func TestScanIntrinsics(t *testing.T) {
	for _, tc := range []struct {
		pkg   string
		files []string
		want  []string
	}{
		{"internal/abi", []string{"funcpc.go"}, []string{"FuncPCABI0", "FuncPCABIInternal"}},
		{"internal/runtime/sys", []string{"intrinsics.go"}, []string{"GetCallerPC", "GetCallerSP", "GetClosurePtr"}},
	} {
		dir := filepath.Join(fakeGOROOT, "src", filepath.FromSlash(tc.pkg))
		scan, err := scanPackage(tc.pkg, dir, tc.files)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, in := range scan.Intrinsics {
			got = append(got, in.Name)
			if in.Package != tc.pkg {
				t.Errorf("%s: package = %q", in.Name, in.Package)
			}
		}
		if len(got) != len(tc.want) {
			t.Errorf("%s intrinsics = %v, want %v", tc.pkg, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s intrinsics = %v, want %v", tc.pkg, got, tc.want)
				break
			}
		}
	}
}

func TestScanLinknameInAFloatingComment(t *testing.T) {
	// runtime/linkname.go keeps its one-argument directives in comment groups
	// of their own; the compiler collects every pragma of the file.
	dir := t.TempDir()
	writeFile(t, dir, "a.go", `package p

import _ "unsafe"

// used in another package
//go:linkname exported
`)
	scan, err := scanPackage("p", dir, []string{"a.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Linknames) != 1 {
		t.Fatalf("linknames = %v", scan.Linknames)
	}
	if ln := scan.Linknames[0]; ln.Local != "exported" || ln.Kind != "one-argument" || ln.Decl != "unknown" {
		t.Errorf("linkname = %+v", ln)
	}
}
