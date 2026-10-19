package toolchain

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// The tests download from a local HTTPS server and never touch the network: the archives are tiny
// tar files with a fake bin/x86_64-w64-mingw32-clang++ script.

const fakeTop = "llvm-mingw-test-ubuntu-22.04-x86_64"

type entry struct {
	name, body, link string
	typ              byte
	mode             int64
}

func tarOf(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		typ := e.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		h := &tar.Header{Name: e.name, Typeflag: typ, Mode: mode, Linkname: e.link, Size: int64(len(e.body))}
		if typ != tar.TypeReg {
			h.Size = 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if typ == tar.TypeReg {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	return buf.Bytes()
}

// goodTar is a toolchain: a folder, the compiler (a script), a read-only data file and a relative
// link, like llvm-mingw's.
func goodTar(t *testing.T) []byte {
	return tarOf(t,
		entry{name: fakeTop + "/", typ: tar.TypeDir, mode: 0o755},
		entry{name: fakeTop + "/bin/", typ: tar.TypeDir, mode: 0o755},
		entry{name: fakeTop + "/bin/clang-wrapper", body: "#!/bin/sh\necho fake\n", mode: 0o755},
		entry{name: fakeTop + "/bin/x86_64-w64-mingw32-clang++", typ: tar.TypeSymlink, link: "clang-wrapper"},
		entry{name: fakeTop + "/include/a.h", body: "// header\n"},
	)
}

type server struct {
	*httptest.Server
	hits atomic.Int64
	body []byte
}

func newServer(t *testing.T, name string, body []byte, handler func(w http.ResponseWriter, r *http.Request, body []byte) bool) *server {
	t.Helper()
	s := &server{body: body}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		if r.URL.Path != "/"+name {
			http.NotFound(w, r)
			return
		}
		if handler != nil && handler(w, r, s.body) {
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(s.body)))
		w.Write(s.body)
	}))
	t.Cleanup(s.Close)
	return s
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// config returns a configuration for the archive body served by s: a host with no loader check, a
// cache of its own, a client that trusts the test server.
func config(t *testing.T, s *server, name string, body []byte) Config {
	t.Helper()
	cache := t.TempDir()
	t.Cleanup(func() { MakeWritable(cache) }) // the toolchain is read-only
	return Config{
		Mode:     Pinned,
		Cache:    cache,
		BaseURL:  s.URL,
		Client:   s.Client(),
		Host:     "linux/amd64",
		Archives: []Archive{{Host: "linux/amd64", Name: name, SHA256: sum(body), Size: int64(len(body))}},
		Download: true,
		Progress: io.Discard,
		LookPath: func(string) (string, error) { return "", errors.New("not in PATH") },
	}
}

func writableCache(t *testing.T) string {
	dir := t.TempDir()
	t.Cleanup(func() { MakeWritable(dir) })
	return dir
}

func resolve(t *testing.T, cfg Config) (Found, error) {
	t.Helper()
	return Resolve(context.Background(), cfg, "windows", "amd64")
}

func TestModeOf(t *testing.T) {
	for in, want := range map[string]Mode{"": DefaultMode, "auto": Auto, "local": Local, "pinned": Pinned} {
		if got, err := ModeOf(in); err != nil || got != want {
			t.Errorf("ModeOf(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ModeOf("always"); err == nil || !strings.Contains(err.Error(), "want auto, local or pinned") {
		t.Errorf("ModeOf(always) error = %v", err)
	}
}

func TestPinnedTable(t *testing.T) {
	// The table is what ships: every entry has a name of the release, a full hash and a size, and
	// the hosts are the Linux ones (Windows hosts come with Phase 14).
	if len(Archives) == 0 {
		t.Fatal("no pinned archive")
	}
	seen := map[string]bool{}
	for _, a := range Archives {
		if seen[a.Host] {
			t.Errorf("host %s is pinned twice", a.Host)
		}
		seen[a.Host] = true
		if len(a.SHA256) != 64 || strings.Trim(a.SHA256, "0123456789abcdef") != "" {
			t.Errorf("%s: SHA-256 %q is not 64 lower-case hex digits", a.Host, a.SHA256)
		}
		if !strings.Contains(a.Name, Release) || !strings.Contains(a.Name, Variant) {
			t.Errorf("%s: archive %q is not of release %s (%s)", a.Host, a.Name, Release, Variant)
		}
		if a.Size < 1<<20 {
			t.Errorf("%s: size %d", a.Host, a.Size)
		}
	}
	if !seen["linux/amd64"] {
		t.Error("no archive for linux/amd64, the host of the cross builds")
	}
}

func TestDownload(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	f, err := resolve(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.Source, "downloaded") || !strings.HasSuffix(f.Path, "bin/x86_64-w64-mingw32-clang++") {
		t.Errorf("found %+v", f)
	}
	if !strings.HasPrefix(f.Path, cfg.Dir()) {
		t.Errorf("the compiler %s is not in %s", f.Path, cfg.Dir())
	}
	out, err := exec.Command(f.Path).Output()
	if err != nil || string(out) != "fake\n" {
		t.Errorf("running the unpacked compiler: %q, %v", out, err)
	}
	if fi, err := os.Stat(filepath.Join(cfg.Dir(), "include", "a.h")); err != nil || fi.Mode().Perm()&0o222 != 0 {
		t.Errorf("a.h is not read-only: %v %v", fi, err)
	}
	// The temporary folder and the archive are gone; the lock file may stay.
	entries, _ := os.ReadDir(filepath.Dir(cfg.Dir()))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".download-") {
			t.Errorf("%s was left behind", e.Name())
		}
	}
	// The second run is a cache hit: no request.
	hits := s.hits.Load()
	f2, err := resolve(t, cfg)
	if err != nil || f2.Path != f.Path || !strings.Contains(f2.Source, "cached") {
		t.Errorf("second Resolve = %+v, %v", f2, err)
	}
	if s.hits.Load() != hits {
		t.Error("the cached toolchain was downloaded again")
	}
	if dir, ok := cfg.Installed(); !ok || dir != cfg.Dir() {
		t.Errorf("Installed = %q, %v", dir, ok)
	}
}

func TestHashMismatch(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	cfg.Archives[0].SHA256 = sum([]byte("something else"))
	_, err := resolve(t, cfg)
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") || !strings.Contains(err.Error(), "nothing was unpacked") {
		t.Fatalf("error = %v", err)
	}
	if _, ok := cfg.Installed(); ok {
		t.Error("a toolchain is installed after a hash mismatch")
	}
	if _, err := os.Stat(cfg.Dir()); err == nil {
		t.Error("the toolchain's folder exists after a hash mismatch")
	}
	entries, _ := os.ReadDir(filepath.Dir(cfg.Dir()))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".download-") {
			t.Errorf("%s was left behind", e.Name())
		}
	}
}

func TestHashMismatchSameSize(t *testing.T) {
	// A tampered archive of the pinned size is caught by the hash alone.
	const name = "t.tar"
	body := goodTar(t)
	bad := bytes.Clone(body)
	bad[len(bad)/2] ^= 0xff
	s := newServer(t, name, bad, nil)
	cfg := config(t, s, name, body) // pinned: the good one
	if _, err := resolve(t, cfg); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("error = %v", err)
	}
}

func TestCutOff(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	// A server that says it will send everything, and sends half.
	s := newServer(t, name, body, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		w.Header().Set("Content-Length", fmt.Sprint(len(b)))
		w.Write(b[:len(b)/2])
		if h, ok := w.(http.Hijacker); ok {
			c, _, _ := h.Hijack()
			c.Close()
		}
		return true
	})
	cfg := config(t, s, name, body)
	if _, err := resolve(t, cfg); err == nil || !strings.Contains(err.Error(), "downloading") {
		t.Fatalf("error = %v", err)
	}
	if _, ok := cfg.Installed(); ok {
		t.Error("installed after a cut-off download")
	}
	// Without a Content-Length: the size check catches it.
	s2 := newServer(t, name, body[:len(body)/2], func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Write(b)
		return true
	})
	cfg2 := config(t, s2, name, body)
	if _, err := resolve(t, cfg2); err == nil || !strings.Contains(err.Error(), "cut off") {
		t.Fatalf("chunked: error = %v", err)
	}
}

func TestWrongSize(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	longer := append(bytes.Clone(body), make([]byte, 1024)...)
	s := newServer(t, name, longer, nil)
	cfg := config(t, s, name, body)
	if _, err := resolve(t, cfg); err == nil || !strings.Contains(err.Error(), "pinned archive has") {
		t.Fatalf("error = %v", err)
	}
	s2 := newServer(t, name, longer, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Write(b)
		return true
	})
	cfg2 := config(t, s2, name, body)
	if _, err := resolve(t, cfg2); err == nil || !strings.Contains(err.Error(), "got ") {
		t.Fatalf("chunked: error = %v", err)
	}
}

func TestHTTPErrors(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, "other.tar", body, nil)
	cfg := config(t, s, name, body)
	if _, err := resolve(t, cfg); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("404: error = %v", err)
	}
	s2 := newServer(t, name, body, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		http.Error(w, "no", http.StatusInternalServerError)
		return true
	})
	if _, err := resolve(t, config(t, s2, name, body)); err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("500: error = %v", err)
	}
}

func TestOnlyHTTPS(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) }))
	defer plain.Close()
	for _, base := range []string{plain.URL, "ftp://example.com/x", "/just/a/path", "https://"} {
		s := newServer(t, name, body, nil)
		cfg := config(t, s, name, body)
		cfg.BaseURL = base
		if _, err := resolve(t, cfg); err == nil || !strings.Contains(err.Error(), "want an https:// folder URL") {
			t.Errorf("BaseURL %q: error = %v", base, err)
		}
	}
}

func TestUnsetHashFailsClosed(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	cfg.Archives[0].SHA256 = ""
	_, err := resolve(t, cfg)
	if err == nil || !strings.Contains(err.Error(), "pinned toolchain hash not set") {
		t.Fatalf("error = %v", err)
	}
	if s.hits.Load() != 0 {
		t.Error("the server was asked although the hash is not set")
	}
}

func TestNoArchiveForHost(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	cfg.Host = "plan9/arm"
	if _, err := resolve(t, cfg); err == nil || !strings.Contains(err.Error(), "no toolchain archive is pinned for a plan9/arm host") {
		t.Fatalf("error = %v", err)
	}
	if _, err := Resolve(context.Background(), cfg, "windows", "arm64"); err == nil || !strings.Contains(err.Error(), "no toolchain is pinned for windows/arm64") {
		t.Fatalf("windows/arm64: error = %v", err)
	}
}

func TestLoaderMissing(t *testing.T) {
	// A system without the dynamic loader of the archive's programs (NixOS) is told so before the
	// download starts.
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	cfg.Archives[0].Loader = filepath.Join(t.TempDir(), "ld.so")
	_, err := resolve(t, cfg)
	if err == nil || !strings.Contains(err.Error(), "need ") || !strings.Contains(err.Error(), "GOCPP_TOOLCHAIN=local") {
		t.Fatalf("error = %v", err)
	}
	if s.hits.Load() != 0 {
		t.Error("downloaded although the programs can't start here")
	}
}

func TestNoDownloadWithoutPermission(t *testing.T) {
	// gocpp check and the language server don't download: Download is false.
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	cfg.Download = false
	_, err := resolve(t, cfg)
	if err == nil || !strings.Contains(err.Error(), "not downloaded yet") {
		t.Fatalf("error = %v", err)
	}
	if s.hits.Load() != 0 {
		t.Error("the server was asked")
	}
	// After a download, the same configuration without permission finds the toolchain.
	cfg.Download = true
	if _, err := resolve(t, cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Download = false
	if f, err := resolve(t, cfg); err != nil || !strings.Contains(f.Source, "cached") {
		t.Errorf("Resolve = %+v, %v", f, err)
	}
}

func TestModes(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	inPath := filepath.Join(t.TempDir(), "x86_64-w64-mingw32-g++")
	cfg.LookPath = func(p string) (string, error) {
		if p == "x86_64-w64-mingw32-g++" {
			return inPath, nil
		}
		return "", errors.New("not found")
	}

	cfg.Mode = Auto
	if f, err := resolve(t, cfg); err != nil || f.Path != inPath || f.Source != "PATH" {
		t.Errorf("auto with a compiler in PATH = %+v, %v", f, err)
	}
	cfg.Mode = Local
	if f, err := resolve(t, cfg); err != nil || f.Path != inPath {
		t.Errorf("local with a compiler in PATH = %+v, %v", f, err)
	}
	if s.hits.Load() != 0 {
		t.Error("downloaded although a compiler is in PATH")
	}
	// clang++ is preferred to g++.
	clang := filepath.Join(t.TempDir(), "x86_64-w64-mingw32-clang++")
	cfg.LookPath = func(p string) (string, error) {
		switch p {
		case "x86_64-w64-mingw32-g++":
			return inPath, nil
		case "x86_64-w64-mingw32-clang++":
			return clang, nil
		}
		return "", errors.New("not found")
	}
	cfg.Mode = Auto
	if f, _ := resolve(t, cfg); f.Path != clang {
		t.Errorf("auto chose %s, want the clang++ first", f.Path)
	}
	// pinned ignores PATH.
	cfg.Mode = Pinned
	f, err := resolve(t, cfg)
	if err != nil || strings.Contains(f.Source, "PATH") || s.hits.Load() != 1 {
		t.Errorf("pinned = %+v, %v (hits %d)", f, err, s.hits.Load())
	}

	// auto without a compiler in PATH downloads; local does not and says how to get one.
	cfg.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	cfg.Cache = writableCache(t)
	cfg.Mode = Local
	_, err = resolve(t, cfg)
	if err == nil || !strings.Contains(err.Error(), "GOCPP_TOOLCHAIN=local never downloads") || !strings.Contains(err.Error(), "x86_64-w64-mingw32-clang++ nor x86_64-w64-mingw32-g++") {
		t.Errorf("local: error = %v", err)
	}
	hits := s.hits.Load()
	cfg.Mode = Auto
	if f, err := resolve(t, cfg); err != nil || !strings.Contains(f.Source, "downloaded") || s.hits.Load() != hits+1 {
		t.Errorf("auto without a compiler = %+v, %v", f, err)
	}
	// auto, when the pinned toolchain fails, names both causes.
	cfg.Cache = writableCache(t)
	cfg.Archives[0].SHA256 = ""
	_, err = resolve(t, cfg)
	if err == nil || !strings.Contains(err.Error(), "is in PATH, and the pinned toolchain is not usable") || !strings.Contains(err.Error(), "set GOCPP_CXX") {
		t.Errorf("auto failing: error = %v", err)
	}
}

func TestNoCacheFolder(t *testing.T) {
	cfg := Config{Mode: Pinned, Download: true, LookPath: func(string) (string, error) { return "", errors.New("no") }}
	if _, err := resolve(t, cfg); err == nil || !strings.Contains(err.Error(), "no cache folder") {
		t.Errorf("error = %v", err)
	}
	if cfg.Dir() != "" {
		t.Errorf("Dir() = %q", cfg.Dir())
	}
}

// Two gocpp processes that start at once download the toolchain once: the second waits for the
// lock and finds the finished folder.
func TestLockContention(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	release := make(chan struct{})
	s := newServer(t, name, body, func(w http.ResponseWriter, r *http.Request, b []byte) bool {
		<-release // hold the first download until the others are waiting for the lock
		return false
	})
	cfg := config(t, s, name, body)
	const n = 5
	var wg sync.WaitGroup
	results := make([]Found, n)
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = resolve(t, cfg)
		}()
	}
	// Wait until the one download has started (one request), give the others time to block on
	// the lock, then let it finish. The count of requests is what is asserted, not the time.
	for s.hits.Load() == 0 {
		runtime.Gosched()
	}
	for range 200 {
		runtime.Gosched()
	}
	close(release)
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("resolver %d: %v", i, errs[i])
		}
		if results[i].Path != results[0].Path {
			t.Errorf("resolver %d found %s, resolver 0 %s", i, results[i].Path, results[0].Path)
		}
	}
	if got := s.hits.Load(); got != 1 {
		t.Errorf("%d downloads for %d resolvers started at once, want 1", got, n)
	}
}

// A waiter for the lock gives up when its context ends.
func TestLockContextCancel(t *testing.T) {
	name := filepath.Join(t.TempDir(), "x.lock")
	unlock, err := lock(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := lock(ctx, name); !errors.Is(err, context.Canceled) {
		t.Errorf("lock with a cancelled context: %v", err)
	}
}

func TestUnpackRejects(t *testing.T) {
	for _, c := range []struct {
		name    string
		entries []entry
		want    string
	}{
		{"dotdot", []entry{{name: fakeTop + "/../evil", body: "x"}}, ".. element"},
		{"absolute", []entry{{name: "/etc/evil", body: "x"}}, "not a relative path"},
		{"two tops", []entry{{name: "a/f", body: "x"}, {name: "b/g", body: "x"}}, "more than one top-level folder"},
		{"absolute link", []entry{{name: fakeTop + "/l", typ: tar.TypeSymlink, link: "/etc/passwd"}}, "not a relative path"},
		{"escaping link", []entry{{name: fakeTop + "/d/l", typ: tar.TypeSymlink, link: "../../../etc"}}, "leaves the toolchain"},
		{"hard link", []entry{{name: fakeTop + "/f", body: "x"}, {name: fakeTop + "/h", typ: tar.TypeLink, link: fakeTop + "/f"}}, "not accepted"},
		{"device", []entry{{name: fakeTop + "/dev", typ: tar.TypeChar}}, "not accepted"},
		{"backslash", []entry{{name: fakeTop + `\x`, body: "x"}}, "not a relative path"},
		{"empty", nil, "empty"},
		{"write through a link", []entry{
			{name: fakeTop + "/d", typ: tar.TypeSymlink, link: "../.."},
			{name: fakeTop + "/d/evil", body: "x"},
		}, "leaves the toolchain"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "root")
			os.Mkdir(root, 0o755)
			err := unpack(bytes.NewReader(tarOf(t, c.entries...)), root, false)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("unpack error = %v, want %q", err, c.want)
			}
			if _, err := os.Lstat(filepath.Join(filepath.Dir(root), "evil")); err == nil {
				t.Error("a file was written outside the root")
			}
		})
	}
}

func TestUnpackLinkStaysInside(t *testing.T) {
	// A link that goes up and comes back inside is fine; llvm-mingw's include links are like that.
	root := filepath.Join(t.TempDir(), "root")
	os.Mkdir(root, 0o755)
	data := tarOf(t,
		entry{name: fakeTop + "/generic/include/x.h", body: "x"},
		entry{name: fakeTop + "/aarch64/include", typ: tar.TypeSymlink, link: "../generic/include"},
	)
	if err := unpack(bytes.NewReader(data), root, false); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "aarch64", "include", "x.h")); err != nil || string(b) != "x" {
		t.Errorf("through the link: %q, %v", b, err)
	}
}

func TestXZArchive(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("no xz program to make the archive")
	}
	cmd := exec.Command("xz", "-c", "-6")
	cmd.Stdin = bytes.NewReader(goodTar(t))
	body, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	const name = "t.tar.xz"
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	f, err := resolve(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(f.Path).Output(); err != nil || string(out) != "fake\n" {
		t.Errorf("compiler: %q, %v", out, err)
	}
}

func TestBrokenXZ(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("no xz program to make the archive")
	}
	cmd := exec.Command("xz", "-c", "-6")
	cmd.Stdin = bytes.NewReader(goodTar(t))
	body, _ := cmd.Output()
	bad := bytes.Clone(body)
	bad[len(bad)/2] ^= 0x55
	const name = "t.tar.xz"
	s := newServer(t, name, bad, nil)
	cfg := config(t, s, name, bad) // the hash is of the damaged file: it passes, the xz check fails
	_, err := resolve(t, cfg)
	if err == nil || !strings.Contains(err.Error(), "unpacking") {
		t.Fatalf("error = %v", err)
	}
	if _, ok := cfg.Installed(); ok {
		t.Error("installed from a damaged archive")
	}
}

func TestArchiveWithoutCompiler(t *testing.T) {
	const name = "t.tar"
	body := tarOf(t, entry{name: fakeTop + "/bin/other", body: "x"})
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	_, err := resolve(t, cfg)
	if err == nil || !strings.Contains(err.Error(), "has no bin/x86_64-w64-mingw32-clang++") && !strings.Contains(err.Error(), "no bin/") {
		t.Fatalf("error = %v", err)
	}
}

func TestRemove(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	if _, err := resolve(t, cfg); err != nil {
		t.Fatal(err)
	}
	dir, err := cfg.Remove()
	if err != nil || dir != cfg.Dir() {
		t.Fatalf("Remove = %q, %v", dir, err)
	}
	if _, err := os.Lstat(cfg.Dir()); err == nil {
		t.Error("the folder is still there")
	}
	if dir, err := cfg.Remove(); err != nil || dir != "" {
		t.Errorf("second Remove = %q, %v", dir, err)
	}
	// And it can be downloaded again.
	if f, err := resolve(t, cfg); err != nil || !strings.Contains(f.Source, "downloaded") {
		t.Errorf("after Remove: %+v, %v", f, err)
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("GOCPP_TOOLCHAIN", "pinned")
	t.Setenv("GOCPP_TOOLCHAIN_URL", "https://mirror.example/llvm")
	t.Setenv("XDG_CACHE_HOME", "/xdg")
	cfg, err := FromEnv(true)
	if err != nil || cfg.Mode != Pinned || cfg.BaseURL != "https://mirror.example/llvm" || cfg.Cache != "/xdg/gocpp" || !cfg.Download {
		t.Errorf("FromEnv = %+v, %v", cfg, err)
	}
	u, err := cfg.archiveURL(Archive{Name: "a.tar.xz"})
	if err != nil || u != "https://mirror.example/llvm/a.tar.xz" {
		t.Errorf("archiveURL = %q, %v", u, err)
	}
	cfg.BaseURL = ""
	if u, _ := cfg.archiveURL(Archive{Name: "a.tar.xz"}); u != DefaultBaseURL+"/a.tar.xz" {
		t.Errorf("default URL = %q", u)
	}
	if want := "/xdg/gocpp/toolchain/llvm-mingw-" + Release + "-" + runtime.GOOS + "-" + runtime.GOARCH; cfg.Dir() != want {
		t.Errorf("Dir = %q, want %q", cfg.Dir(), want)
	}
	t.Setenv("GOCPP_TOOLCHAIN", "sometimes")
	if _, err := FromEnv(false); err == nil {
		t.Error("a bad GOCPP_TOOLCHAIN is accepted")
	}
}
