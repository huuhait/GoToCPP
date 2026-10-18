package modfetch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

func parseMod(t *testing.T, text string) *gocppmod.File {
	t.Helper()
	f, errs := gocppmod.Parse("gocpp.mod", []byte(text), nil)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	return f
}

// project makes a main module that requires tiny (from the fake host) and a
// module that is replaced by a local folder.
func project(t *testing.T, h *fakeHost) (dir string, f *Fetcher, file *gocppmod.File) {
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3")...))
	dir = t.TempDir()
	mod := "module example.com/app\n\nrequire (\n\tgithub.com/acme/tiny v1.2.3\n\texample.com/loc v0.0.0\n)\n\nreplace example.com/loc => ./loc\n"
	mustWrite(t, filepath.Join(dir, "gocpp.mod"), mod)
	mustWrite(t, filepath.Join(dir, "loc", "gocpp.mod"), "module example.com/loc\n")
	mustWrite(t, filepath.Join(dir, "loc", "a.gp"), "package loc\n")
	mustWrite(t, filepath.Join(dir, "loc", "sub", "b.gp"), "package sub\n")
	mustWrite(t, filepath.Join(dir, "loc", "sub", "b_test.gp"), "package sub\n")
	mustWrite(t, filepath.Join(dir, "loc", "testdata", "c.gp"), "package x\n")
	mustWrite(t, filepath.Join(dir, "loc", "vendor", "junk", "j.gp"), "package j\n")
	f = h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	return dir, f, parseMod(t, mod)
}

func TestVendor(t *testing.T) {
	h := newFakeHost(t)
	dir, f, file := project(t, h)
	if err := f.Vendor(context.Background(), dir, file); err != nil {
		t.Fatal(err)
	}
	v := filepath.Join(dir, "vendor")
	text, err := os.ReadFile(filepath.Join(v, "modules.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# example.com/loc v0.0.0 => ./loc\n## explicit\nexample.com/loc\nexample.com/loc/sub\n" +
		"# github.com/acme/tiny v1.2.3\n## explicit\n"
	if string(text) != want {
		t.Errorf("modules.txt =\n%s\nwant\n%s", text, want)
	}
	vm, errs := gocppmod.ParseVendor("modules.txt", text)
	if len(errs) > 0 || len(gocppmod.CheckVendor(file, vm)) > 0 {
		t.Errorf("not consistent: %v %v", errs, gocppmod.CheckVendor(file, vm))
	}
	// The files of the downloaded module are all there (a C++ library needs
	// its headers), writable and with the exec bit.
	tiny := filepath.Join(v, "github.com", "acme", "tiny")
	if got := readTree(t, tiny); len(got) != 3 || got["include/tiny.h"] != libFiles()["include/tiny.h"] {
		t.Errorf("vendored tiny = %v", got)
	}
	if st, _ := os.Stat(filepath.Join(tiny, "README.md")); st.Mode().Perm() != 0o644 {
		t.Errorf("mode %v", st.Mode())
	}
	if st, _ := os.Stat(filepath.Join(tiny, "build.sh")); st.Mode().Perm() != 0o755 {
		t.Errorf("mode %v", st.Mode())
	}
	if got, err := HashDir(context.Background(), tiny, tmod, tver); err != nil || got != goHash(tmod+"@"+tver, libFiles()) {
		t.Errorf("hash of the copy: %s %v", got, err)
	}
	// The replaced module is copied without its own vendor folder.
	loc := readTree(t, filepath.Join(v, "example.com", "loc"))
	if len(loc) != 5 || loc["a.gp"] == "" || loc["sub/b.gp"] == "" || loc["testdata/c.gp"] == "" {
		t.Errorf("vendored loc = %v", loc)
	}
	// gocpp.sum got the hash of the download, not of the local module.
	s, _ := LoadSums(dir)
	if len(s.Lines()) != 1 || s.Lines()[0].Path != tmod {
		t.Errorf("sums = %v", s.Lines())
	}
	// Nothing temporary is left in the project.
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("left over: %s", e.Name())
		}
	}
	// A second run replaces the folder (a file added by hand is gone), offline.
	mustWrite(t, filepath.Join(v, "stale.txt"), "x")
	f.cfg.Offline = true
	if err := f.Vendor(context.Background(), dir, file); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(v, "stale.txt")); err == nil {
		t.Error("the old vendor folder was kept")
	}
	// Verify: the vendored copy matches gocpp.sum.
	res, err := f.Verify(context.Background(), dir, file)
	if err != nil || len(res) != 1 || res[0].Where != "vendor" || res[0].Problem != "" {
		t.Errorf("Verify = %+v, %v", res, err)
	}
	mustWrite(t, filepath.Join(tiny, "include", "tiny.h"), "#error evil\n")
	res, _ = f.Verify(context.Background(), dir, file)
	if len(res) != 1 || !strings.Contains(res[0].Problem, "dir has been modified") {
		t.Errorf("Verify after a change = %+v", res)
	}
}

func TestVendorOffline(t *testing.T) {
	// "gocpp mod vendor, then an offline build": the vendored project needs
	// no cache and no network.
	h := newFakeHost(t)
	dir, f, file := project(t, h)
	if err := f.Vendor(context.Background(), dir, file); err != nil {
		t.Fatal(err)
	}
	n := h.total()
	CleanModCache(f.cfg.CacheDir)
	if got := readTree(t, filepath.Join(dir, "vendor", "github.com", "acme", "tiny")); len(got) != 3 {
		t.Error("vendor folder lost its files")
	}
	if h.total() != n {
		t.Error("network used")
	}
}

func TestVendorRefusesForeignFolder(t *testing.T) {
	h := newFakeHost(t)
	dir, f, file := project(t, h)
	mustWrite(t, filepath.Join(dir, "vendor", "sqlite", "sqlite3.c"), "// my own third party code\n")
	err := f.Vendor(context.Background(), dir, file)
	if err == nil || !strings.Contains(err.Error(), "not made by gocpp mod vendor") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "vendor", "sqlite", "sqlite3.c")); err != nil {
		t.Error("the foreign vendor folder was changed")
	}
	// An empty folder is fine.
	os.RemoveAll(filepath.Join(dir, "vendor"))
	os.Mkdir(filepath.Join(dir, "vendor"), 0o755)
	if err := f.Vendor(context.Background(), dir, file); err != nil {
		t.Fatal(err)
	}
}

func TestVendorFailureKeepsOldFolder(t *testing.T) {
	h := newFakeHost(t)
	dir, f, file := project(t, h)
	if err := f.Vendor(context.Background(), dir, file); err != nil {
		t.Fatal(err)
	}
	// The module for the new requirement does not exist.
	file2 := parseMod(t, "module example.com/app\nrequire github.com/acme/tiny v9.9.9\n")
	if err := f.Vendor(context.Background(), dir, file2); err == nil {
		t.Fatal("no error")
	}
	if got := readTree(t, filepath.Join(dir, "vendor", "github.com", "acme", "tiny")); len(got) != 3 {
		t.Error("the old vendor folder was damaged")
	}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("left over: %s", e.Name())
		}
	}
}

func TestVendorNonLocalReplace(t *testing.T) {
	h := newFakeHost(t)
	dir, f, _ := project(t, h)
	file := parseMod(t, "module example.com/app\nrequire github.com/acme/tiny v1.2.3\nreplace github.com/acme/tiny => github.com/fork/tiny v1.0.0\n")
	if err := f.Vendor(context.Background(), dir, file); err == nil || !strings.Contains(err.Error(), "not a local folder") {
		t.Errorf("error = %v", err)
	}
}

func TestVendorLeavesOutGitOfAReplacement(t *testing.T) {
	h := newFakeHost(t)
	dir, f, file := project(t, h)
	mustWrite(t, filepath.Join(dir, "loc", ".git", "HEAD"), "ref: refs/heads/main\n")
	mustWrite(t, filepath.Join(dir, "loc", "sub", ".git"), "gitdir: ../../.git/modules/sub\n")
	if err := f.Vendor(context.Background(), dir, file); err != nil {
		t.Fatal(err)
	}
	loc := readTree(t, filepath.Join(dir, "vendor", "example.com", "loc"))
	for name := range loc {
		if strings.Contains(name, ".git") {
			t.Errorf("vendored %s", name)
		}
	}
}

func TestVendorRefusesSymlinksInReplacement(t *testing.T) {
	h := newFakeHost(t)
	dir, f, file := project(t, h)
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "loc", "link")); err != nil {
		t.Skip(err)
	}
	if err := f.Vendor(context.Background(), dir, file); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("error = %v", err)
	}
}

func TestVerifyCache(t *testing.T) {
	h := newFakeHost(t)
	dir, f, file := project(t, h)
	// Nothing is downloaded yet: skipped.
	res, err := f.Verify(context.Background(), dir, file)
	if err != nil || len(res) != 1 || !res[0].Skipped {
		t.Fatalf("Verify = %+v %v", res, err)
	}
	if _, err := f.DownloadAll(context.Background(), dir, file, nil); err != nil {
		t.Fatal(err)
	}
	res, _ = f.Verify(context.Background(), dir, file)
	if len(res) != 1 || res[0].Problem != "" || res[0].Skipped || res[0].Where != "module cache" {
		t.Fatalf("Verify = %+v", res)
	}
	// Modify a file in the cache.
	tdir := f.ModDir(tmod, tver)
	os.Chmod(filepath.Join(tdir, "include"), 0o755)
	os.Chmod(filepath.Join(tdir, "include", "tiny.h"), 0o644)
	os.WriteFile(filepath.Join(tdir, "include", "tiny.h"), []byte("#error evil\n"), 0o644)
	res, _ = f.Verify(context.Background(), dir, file)
	if len(res) != 1 || !strings.Contains(res[0].Problem, "dir has been modified") {
		t.Errorf("after a change: %+v", res)
	}
	// An entry missing in gocpp.sum.
	os.Remove(filepath.Join(dir, "gocpp.sum"))
	res, _ = f.Verify(context.Background(), dir, file)
	if len(res) != 1 || !strings.Contains(res[0].Problem, "modified") {
		t.Logf("%+v", res)
	}
}

func TestDownloadAll(t *testing.T) {
	h := newFakeHost(t)
	dir, f, file := project(t, h)
	ms, err := f.DownloadAll(context.Background(), dir, file, nil)
	if err != nil || len(ms) != 1 || ms[0].Path != tmod {
		t.Fatalf("DownloadAll = %v %v", ms, err)
	}
	// A module that is not required is an error.
	if _, err := f.DownloadAll(context.Background(), dir, file, []string{"github.com/x/y"}); err == nil || !strings.Contains(err.Error(), "is not required") {
		t.Errorf("error = %v", err)
	}
	// A bad sum stops it.
	mustWrite(t, filepath.Join(dir, "gocpp.sum"), tmod+" "+tver+" h1:"+strings.Repeat("A", 43)+"=\n")
	f2 := h.fetcher()
	cleanupCache(t, f2.cfg.CacheDir)
	if _, err := f2.DownloadAll(context.Background(), dir, file, nil); err == nil || !strings.Contains(err.Error(), "SECURITY ERROR") {
		t.Errorf("error = %v", err)
	}
}

func TestSetRequire(t *testing.T) {
	for _, tc := range []struct{ name, in, path, version, want string }{
		{"update line", "module m\n\nrequire a.com/x v1.0.0\n", "a.com/x", "v1.1.0", "module m\n\nrequire a.com/x v1.1.0\n"},
		{"update in block", "module m\n\nrequire (\n\ta.com/x v1.0.0\n\tb.com/y v1.0.0 // indirect\n)\n", "b.com/y", "v1.2.0", "module m\n\nrequire (\n\ta.com/x v1.0.0\n\tb.com/y v1.2.0\n)\n"},
		{"add to block", "module m\n\nrequire (\n\ta.com/x v1.0.0\n)\n", "b.com/y", "v1.2.0", "module m\n\nrequire (\n\ta.com/x v1.0.0\n\tb.com/y v1.2.0\n)\n"},
		{"add after require", "module m\n\nrequire a.com/x v1.0.0\n\ncflags -DX\n", "b.com/y", "v1.2.0", "module m\n\nrequire a.com/x v1.0.0\nrequire b.com/y v1.2.0\n\ncflags -DX\n"},
		{"add after module", "module m\n\ngocpp 0.1\ncxx c++20\n\ncflags -DX // note\n", "b.com/y", "v1.2.0", "module m\n\ngocpp 0.1\ncxx c++20\n\nrequire b.com/y v1.2.0\n\ncflags -DX // note\n"},
		{"empty file", "", "b.com/y", "v1.2.0", "require b.com/y v1.2.0\n"},
		{"comments kept", "// top\nmodule m // here\n\n// the lib\nrequire a.com/x v1.0.0 // why\n", "a.com/x", "v2.0.0", "// top\nmodule m // here\n\n// the lib\nrequire a.com/x v2.0.0 // why\n"},
		{"unknown kept", "module m\n\ncxx a.com/x {\n\tsources *.cpp\n}\n\nrequire a.com/x v1.0.0\n", "a.com/x", "v1.0.1", "module m\n\ncxx a.com/x {\n\tsources *.cpp\n}\n\nrequire a.com/x v1.0.1\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := parseMod(t, tc.in)
			SetRequire(f, tc.path, tc.version)
			got, err := FormatFile("gocpp.mod", f)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
			g := parseMod(t, string(got))
			if r := g.FindRequire(tc.path); r == nil || r.Version != tc.version {
				t.Errorf("the require line is %+v", r)
			}
		})
	}
}

func TestDropRequire(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"line", "module m\n\nrequire a.com/x v1.0.0\nrequire b.com/y v1.0.0\n", "module m\n\nrequire b.com/y v1.0.0\n"},
		{"block", "module m\n\nrequire (\n\ta.com/x v1.0.0\n\tb.com/y v1.0.0\n)\n", "module m\n\nrequire (\n\tb.com/y v1.0.0\n)\n"},
		{"last in block", "module m\n\nrequire (\n\ta.com/x v1.0.0\n)\n", "module m\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := parseMod(t, tc.in)
			if !DropRequire(f, "a.com/x") {
				t.Fatal("not found")
			}
			got, _ := FormatFile("gocpp.mod", f)
			if string(got) != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
	f := parseMod(t, "module m\n")
	if DropRequire(f, "a.com/x") {
		t.Error("dropped a require that is not there")
	}
}
