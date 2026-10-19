package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type tarEntry struct {
	name string
	body string
	typ  byte
}

// fixtureTarball builds a small Go source tarball: go/VERSION, go/LICENSE
// and test/ files, plus entries that must not be extracted.
func fixtureTarball(t *testing.T, version string, extra ...tarEntry) ([]byte, string) {
	t.Helper()
	entries := []tarEntry{
		{"go/VERSION", version + "\ntime 2025\n", tar.TypeReg},
		{"go/LICENSE", "BSD\n", tar.TypeReg},
		{"go/test/", "", tar.TypeDir},
		{"go/test/a.go", "// run\n\npackage main\n\nfunc main() { println(1) }\n", tar.TypeReg},
		{"go/test/a.out", "1\n", tar.TypeReg},
		{"go/test/ken/", "", tar.TypeDir},
		{"go/test/ken/b.go", "// run\n\npackage main\n\nfunc main() {}\n", tar.TypeReg},
		{"go/src/fmt/print.go", "package fmt\n", tar.TypeReg},
	}
	entries = append(entries, extra...)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Typeflag: e.typ, Mode: 0o644, Size: int64(len(e.body))}
		if e.typ == tar.TypeDir {
			h.Mode = 0o755
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:])
}

func fixtureSource(t *testing.T, data []byte, sum string, fetch bool, url string) *Source {
	t.Helper()
	p := Pin{Version: "go1.25.14", URL: url, SHA256: sum}
	return &Source{Pin: p, Cache: filepath.Join(t.TempDir(), "cache"), Fetch: fetch}
}

func TestParsePin(t *testing.T) {
	good := "# c\nversion go1.25.14\nurl https://dl.google.com/go/go1.25.14.src.tar.gz\nsha256 " + strings.Repeat("ab", 32) + "\n"
	p, err := parsePin(good)
	if err != nil || p.Version != "go1.25.14" || p.minor() != 25 || !strings.HasPrefix(p.stem(), "go1.25.14-abababababab") {
		t.Errorf("%+v %v", p, err)
	}
	for _, bad := range []string{
		"version go1.25.14\nurl https://x\n",
		"version latest\nurl https://x\nsha256 " + strings.Repeat("ab", 32) + "\n",
		"version go1.25.14\nurl ftp://x\nsha256 " + strings.Repeat("ab", 32) + "\n",
		"version go1.25.14\nurl https://x\nsha256 xyz\n",
		"version go1.25.14\nfoo bar\n",
		"version\n",
	} {
		if _, err := parsePin(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestPinsFile(t *testing.T) {
	b, err := os.ReadFile("PINS")
	if err != nil {
		t.Fatal(err)
	}
	p, err := parsePin(string(b))
	if err != nil {
		t.Fatal(err)
	}
	// the same tarball as tools/portstd, when that tool is in the tree
	if other, err := os.ReadFile(filepath.Join("..", "portstd", "PINS")); err == nil {
		q, err := parsePin(string(other))
		if err != nil {
			t.Fatal(err)
		}
		if p != q {
			t.Errorf("tools/gotest/PINS %+v differs from tools/portstd/PINS %+v: the tools share the tarball in the cache", p, q)
		}
	}
}

func TestSourceExtractsOnlyTest(t *testing.T) {
	data, sum := fixtureTarball(t, "go1.25.14")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()
	s := fixtureSource(t, data, sum, true, srv.URL+"/go.tgz")
	root, err := s.Root()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"VERSION", "LICENSE", "test/a.go", "test/a.out", "test/ken/b.go"} {
		if _, err := os.Stat(filepath.Join(root, want)); err != nil {
			t.Errorf("%s not extracted: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "src")); err == nil {
		t.Error("src/ was extracted: only test/ is wanted")
	}
	// the second call finds the tree without the network
	srv.Close()
	s.Fetch = false
	if r2, err := s.Root(); err != nil || r2 != root {
		t.Errorf("second Root: %q %v", r2, err)
	}
	if !s.Available() {
		t.Error("Available() = false")
	}
}

func TestSourceOfflineWithoutCache(t *testing.T) {
	data, sum := fixtureTarball(t, "go1.25.14")
	s := fixtureSource(t, data, sum, false, "http://127.0.0.1:1/never")
	_, err := s.Root()
	var nc *NoCacheError
	if !errors.As(err, &nc) {
		t.Fatalf("err = %v, want NoCacheError", err)
	}
	if s.Available() {
		t.Error("Available() = true with an empty cache")
	}
}

func TestSourceUsesTarballInCache(t *testing.T) {
	data, sum := fixtureTarball(t, "go1.25.14")
	s := fixtureSource(t, data, sum, false, "http://127.0.0.1:1/never")
	os.MkdirAll(s.Cache, 0o755)
	if err := os.WriteFile(s.tarball(), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Root(); err != nil {
		t.Fatal(err)
	}
}

func TestSourceUsesPortstdTree(t *testing.T) {
	data, sum := fixtureTarball(t, "go1.25.14")
	s := fixtureSource(t, data, sum, false, "http://127.0.0.1:1/never")
	tree := s.portstdTree()
	os.MkdirAll(filepath.Join(tree, "test"), 0o755)
	os.WriteFile(filepath.Join(tree, portstdDoneMarker), nil, 0o644)
	root, err := s.Root()
	if err != nil || root != tree {
		t.Errorf("Root = %q, %v; want portstd's tree %q", root, err, tree)
	}
}

func TestSourceRejectsWrongHash(t *testing.T) {
	data, _ := fixtureTarball(t, "go1.25.14")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()
	s := fixtureSource(t, data, strings.Repeat("0", 64), true, srv.URL)
	_, err := s.Root()
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("err = %v", err)
	}
	if ents, _ := os.ReadDir(s.Cache); len(ents) != 0 {
		t.Errorf("the cache keeps %d entries after a bad download", len(ents))
	}
	// a bad tarball already in the cache is refused too
	os.WriteFile(s.tarball(), data, 0o644)
	s.Fetch = false
	if _, err := s.Root(); err == nil || !strings.Contains(err.Error(), "delete") {
		t.Errorf("cached tarball with a wrong hash: %v", err)
	}
}

func TestSourceHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	s := fixtureSource(t, nil, strings.Repeat("0", 64), true, srv.URL)
	if _, err := s.Root(); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("err = %v", err)
	}
}

func TestExtractRefusesUnsafeNames(t *testing.T) {
	data, _ := fixtureTarball(t, "go1.25.14", tarEntry{"go/test/../../evil", "x", tar.TypeReg})
	// path.Clean folds the name to go/evil: not under test/, so it is skipped
	tb := filepath.Join(t.TempDir(), "x.tgz")
	os.WriteFile(tb, data, 0o644)
	dir := t.TempDir()
	if err := extract(tb, dir, "go1.25.14"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "evil")); err == nil {
		t.Error("evil was extracted")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil")); err == nil {
		t.Error("evil escaped the folder")
	}
}

func TestExtractIgnoresLinks(t *testing.T) {
	data, _ := fixtureTarball(t, "go1.25.14", tarEntry{"go/test/link.go", "", tar.TypeSymlink})
	tb := filepath.Join(t.TempDir(), "x.tgz")
	os.WriteFile(tb, data, 0o644)
	dir := t.TempDir()
	if err := extract(tb, dir, "go1.25.14"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "test", "link.go")); err == nil {
		t.Error("a symlink was extracted")
	}
}

func TestExtractVersionMismatch(t *testing.T) {
	data, _ := fixtureTarball(t, "go1.24.0")
	tb := filepath.Join(t.TempDir(), "x.tgz")
	os.WriteFile(tb, data, 0o644)
	if err := extract(tb, t.TempDir(), "go1.25.14"); err == nil || !strings.Contains(err.Error(), "VERSION") {
		t.Errorf("err = %v", err)
	}
	// not a Go tarball at all
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tar.NewWriter(gz).Close()
	gz.Close()
	os.WriteFile(tb, buf.Bytes(), 0o644)
	if err := extract(tb, t.TempDir(), "go1.25.14"); err == nil {
		t.Error("an archive without VERSION is accepted")
	}
}
