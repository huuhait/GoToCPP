package modfetch

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeTree writes the files (name -> content) below a new folder.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestModuleFilesRules(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"gocpp.mod":                 "module example.com/m\n",
		"main.gp":                   "package main\n",
		"util.go":                   "package main\n",
		"lib/a.cpp":                 "int a;\n",
		"lib/a.hpp":                 "extern int a;\n",
		"LICENSE":                   "MIT\n",
		"docs/readme.md":            "hello\n",
		"sub/gocpp.mod":             "module example.com/m/sub\n",
		"sub/x.gp":                  "package sub\n",
		"sub/deeper/y.gp":           "package deeper\n",
		"gosub/go.mod":              "module example.com/m/gosub\n",
		"gosub/z.go":                "package gosub\n",
		".git/config":               "[core]\n",
		"third_party/vendor/k.h":    "int k;\n",
		"vendor/modules.txt":        "# example.com/x v1.0.0\n",
		"vendor/example.com/x/x.go": "package x\n",
		"nested/lib/gocpp.mod":      "module example.com/m/nested/lib\n",
		"nested/keep.gp":            "package nested\n",
		".gitignore":                "build/\n",
	})
	res, err := ListModuleFiles(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".gitignore", "LICENSE", "docs/readme.md", "gocpp.mod", "lib/a.cpp", "lib/a.hpp", "main.gp", "nested/keep.gp", "third_party/vendor/k.h", "util.go"}
	if !slices.Equal(res.Files, want) {
		t.Errorf("files:\n got %v\nwant %v", res.Files, want)
	}
	var skipped []string
	for _, s := range res.Skipped {
		skipped = append(skipped, s.Dir+":"+s.Reason)
	}
	wantSkipped := []string{".git:vcs", "gosub:module", "nested/lib:module", "sub:module", "vendor:vendor"}
	if !slices.Equal(skipped, wantSkipped) {
		t.Errorf("skipped:\n got %v\nwant %v", skipped, wantSkipped)
	}
}

func TestVendorFolderWithoutModulesTxtBelongsToTheModule(t *testing.T) {
	// A C or C++ library keeps its own dependencies in vendor/: only a folder that
	// "gocpp mod vendor" made (it has modules.txt) is left out.
	dir := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "vendor/zlib/zlib.h": "int z;\n"})
	res, err := ListModuleFiles(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Files, "vendor/zlib/zlib.h") || len(res.Skipped) != 0 {
		t.Errorf("%+v", res)
	}
}

func TestModuleFilesRejectSymlinks(t *testing.T) {
	dir := writeTree(t, map[string]string{"a.gp": "package a\n"})
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "link")); err != nil {
		t.Skip(err)
	}
	if _, err := ListModuleFiles(context.Background(), dir); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("symlink: %v", err)
	}
	var buf bytes.Buffer
	if err := CreateZip(context.Background(), &buf, dir, "example.com/m", "v1.0.0"); err == nil {
		t.Error("CreateZip accepted a symbolic link")
	}
}

func TestPruneModuleMakesTreeAndHashAgree(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":     "module example.com/m\n",
		"a.gp":          "package a\n",
		"sub/gocpp.mod": "module example.com/m/sub\n",
		"sub/b.gp":      "package sub\n",
		".git/HEAD":     "ref: x\n",
	}
	dir := writeTree(t, files)
	ctx := context.Background()
	var zbuf bytes.Buffer
	if err := CreateZip(ctx, &zbuf, dir, "example.com/m", "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	if err := PruneModule(ctx, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sub")); err == nil {
		t.Error("the nested module is still in the tree")
	}
	treeHash, err := HashDir(ctx, dir, "example.com/m", "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	// The hash of the zip's files, named as in the zip.
	zr, err := zip.NewReader(bytes.NewReader(zbuf.Bytes()), int64(zbuf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	zipHash, err := Hash1(names, func(name string) (readCloser, error) {
		for _, f := range zr.File {
			if f.Name == name {
				return f.Open()
			}
		}
		return nil, os.ErrNotExist
	})
	if err != nil {
		t.Fatal(err)
	}
	if treeHash != zipHash {
		t.Errorf("tree hash %s, zip hash %s", treeHash, zipHash)
	}
	if !slices.Equal(names, []string{"example.com/m@v1.2.3/a.gp", "example.com/m@v1.2.3/gocpp.mod"}) {
		t.Errorf("zip names = %v", names)
	}
}

func TestCreateZipChecks(t *testing.T) {
	ctx := context.Background()
	ok := writeTree(t, map[string]string{"a.gp": "package a\n"})
	var b bytes.Buffer
	if err := CreateZip(ctx, &b, ok, "example.com/m", "notaversion"); err == nil {
		t.Error("bad version accepted")
	}
	if err := CreateZip(ctx, &b, ok, "example.com//m", "v1.0.0"); err == nil {
		t.Error("bad path accepted")
	}
	// A local module (no dot in its path) may be zipped, which a download check would refuse.
	b.Reset()
	if err := CreateZip(ctx, &b, ok, "mylib", "v0.0.0"); err != nil {
		t.Errorf("dotless path: %v", err)
	}
	// Names that clash on a case-insensitive file system.
	clash := writeTree(t, map[string]string{"Readme.md": "x", "README.md": "y"})
	if err := CreateZip(ctx, &b, clash, "example.com/m", "v1.0.0"); err != nil && !strings.Contains(err.Error(), "differ only in case") {
		t.Errorf("clash: %v", err)
	}
	// Reserved names.
	bad := writeTree(t, map[string]string{"aux.h": "x"})
	if err := CreateZip(ctx, &b, bad, "example.com/m", "v1.0.0"); err == nil {
		t.Error("a Windows device name was accepted")
	}
	// Cancelled.
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := CreateZip(cctx, &b, ok, "example.com/m", "v1.0.0"); err == nil {
		t.Error("a cancelled context was ignored")
	}
}

func TestIsModuleRoot(t *testing.T) {
	dir := writeTree(t, map[string]string{"a/gocpp.mod": "", "b/go.mod": "", "c/other": "", "d/go.mod/x": ""})
	for name, want := range map[string]bool{"a": true, "b": true, "c": false, "d": false} {
		if got := isModuleRoot(filepath.Join(dir, name)); got != want {
			t.Errorf("isModuleRoot(%s) = %v", name, got)
		}
	}
}

func TestCreateZipFolderCaseClash(t *testing.T) {
	dir := writeTree(t, map[string]string{"Sub/x.h": "x", "sub/y.h": "y"})
	var b bytes.Buffer
	if err := CreateZip(context.Background(), &b, dir, "example.com/m", "v1.0.0"); err == nil || !strings.Contains(err.Error(), "differ only in case") {
		t.Errorf("err = %v", err)
	}
}
