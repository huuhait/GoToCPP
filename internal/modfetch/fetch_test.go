package modfetch

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	tmod = "github.com/acme/tiny"
	tver = "v1.2.3"
)

func TestDownloadGood(t *testing.T) {
	h := newFakeHost(t)
	p := h.tag("acme/tiny", tver, tarGz(testCommit, library("tiny-1.2.3")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	var progress bytes.Buffer
	f.cfg.Stderr = &progress

	m, err := f.Download(context.Background(), tmod, tver, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Downloaded || m.Dir != filepath.Join(f.cfg.CacheDir, "github.com", "acme", "tiny@v1.2.3") {
		t.Errorf("module = %+v", m)
	}
	if got := readTree(t, m.Dir); len(got) != 3 || got["include/tiny.h"] != libFiles()["include/tiny.h"] {
		t.Errorf("tree = %v", got)
	}
	if want := goHash(tmod+"@"+tver, libFiles()); m.Hash != want {
		t.Errorf("hash = %s, want %s (computed by hand)", m.Hash, want)
	}
	if m.ModHash != "" {
		t.Errorf("ModHash = %q for a module with no gocpp.mod", m.ModHash)
	}
	if !strings.Contains(progress.String(), "gocpp: downloading github.com/acme/tiny v1.2.3") {
		t.Errorf("progress = %q", progress.String())
	}
	// Read-only files and folders; the executable bit stays.
	wantModes := map[string]fs.FileMode{
		"README.md":      0o444,
		"build.sh":       0o555,
		"include/tiny.h": 0o444,
		"include":        fs.ModeDir | 0o555,
		".":              fs.ModeDir | 0o555,
	}
	if runtime.GOOS == "windows" {
		// Windows has a read-only flag and no executable bit: build.sh is read-only like the others.
		wantModes["build.sh"] = 0o444
	}
	for rel, want := range wantModes {
		st, err := os.Stat(filepath.Join(m.Dir, rel))
		if err != nil || st.Mode() != want {
			t.Errorf("%s: mode %v (%v), want %v", rel, st.Mode(), err, want)
		}
	}
	if err := os.WriteFile(filepath.Join(m.Dir, "README.md"), []byte("x"), 0o644); err == nil {
		t.Error("a cached file is writable")
	}
	// The second call is served from the cache.
	m2, err := f.Download(context.Background(), tmod, tver, nil)
	if err != nil || m2.Downloaded || m2.Hash != m.Hash {
		t.Errorf("second Download = %+v, %v", m2, err)
	}
	if n := h.count(p); n != 1 {
		t.Errorf("the archive was requested %d times, want 1", n)
	}
	// No temporary files are left.
	ents, _ := os.ReadDir(filepath.Dir(m.Dir))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("left over: %s", e.Name())
		}
	}
	// The user agent.
	if ua := userAgent(); !strings.HasPrefix(ua, "gocpp/") {
		t.Errorf("User-Agent %q", ua)
	}
}

func TestDownloadZip(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, zipOf(library("tiny-1.2.3")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	m, err := f.Download(context.Background(), tmod, tver, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := goHash(tmod+"@"+tver, libFiles()); m.Hash != want {
		t.Errorf("hash = %s, want %s", m.Hash, want)
	}
}

func TestUserAgentSent(t *testing.T) {
	h := newFakeHost(t)
	var ua string
	h.handle("/acme/tiny/archive/refs/tags/v1.2.3.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.Write(tarGz("", library("tiny-1.2.3")...))
	})
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	if _, err := f.Download(context.Background(), tmod, tver, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ua, "gocpp/") {
		t.Errorf("User-Agent = %q", ua)
	}
}

func TestSumsRecordThenMismatch(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz(testCommit, library("tiny-1.2.3")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	main := t.TempDir()
	m, err := f.Fetch(context.Background(), main, tmod, tver, FetchOptions{Record: true})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(main, "gocpp.sum"))
	if err != nil || string(data) != tmod+" "+tver+" "+m.Hash+"\n" {
		t.Fatalf("gocpp.sum = %q, %v", data, err)
	}
	// A cache that matches is fine, and nothing is requested again.
	n := h.total()
	if _, err := f.Fetch(context.Background(), main, tmod, tver, FetchOptions{}); err != nil || h.total() != n {
		t.Fatalf("Fetch from the cache: %v (requests %d -> %d)", err, n, h.total())
	}

	// Another cache, and the host now serves other files for the tag: a
	// security error, and the module does not get into the cache.
	h.tag("acme/tiny", tver, tarGz(testCommit, library("tiny-1.2.3", tfile{Name: "tiny-1.2.3/evil.h", Body: "#error x\n"})...))
	f2 := h.fetcher()
	cleanupCache(t, f2.cfg.CacheDir)
	_, err = f2.Fetch(context.Background(), main, tmod, tver, FetchOptions{Record: true})
	var se *SecurityError
	if !errors.As(err, &se) {
		t.Fatalf("error = %v, want a SecurityError", err)
	}
	for _, want := range []string{"SECURITY ERROR", tmod + " " + tver + ": checksum mismatch", "downloaded: h1:", "gocpp.sum: " + m.Hash, "Nothing was added to the module cache"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message lacks %q:\n%s", want, err)
		}
	}
	if _, err := os.Stat(f2.ModDir(tmod, tver)); err == nil {
		t.Error("the mismatching module is in the cache")
	}
	assertNoTemp(t, f2)
	// gocpp.sum is unchanged.
	if after, _ := os.ReadFile(filepath.Join(main, "gocpp.sum")); string(after) != string(data) {
		t.Errorf("gocpp.sum changed to %q", after)
	}

	// A cache that has other files than gocpp.sum says (an edited cache).
	tamper := f.ModDir(tmod, tver)
	os.Chmod(tamper, 0o755)
	os.Chmod(filepath.Join(tamper, "include"), 0o755)
	os.WriteFile(filepath.Join(tamper, "include", "extra.h"), []byte("x"), 0o644)
	// The hash file still says the old hash: the cache is used as recorded
	// (verify finds it, see TestVerify), but a hash file that is gone is
	// recomputed and then does not match.
	os.Chmod(f.sidecar(tmod, tver), 0o644)
	os.Remove(f.sidecar(tmod, tver))
	_, err = f.Fetch(context.Background(), main, tmod, tver, FetchOptions{})
	if !errors.As(err, &se) || se.Cached == "" || !strings.Contains(err.Error(), "module cache (") {
		t.Errorf("error for an edited cache = %v", err)
	}
}

func assertNoTemp(t *testing.T, f *Fetcher) {
	t.Helper()
	filepath.WalkDir(f.cfg.CacheDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasPrefix(d.Name(), ".") && p != f.cfg.CacheDir {
			t.Errorf("left over in the cache: %s", p)
		}
		return nil
	})
}

func TestModFileHashLine(t *testing.T) {
	h := newFakeHost(t)
	modText := "module github.com/acme/tiny\n"
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3", tfile{Name: "tiny-1.2.3/gocpp.mod", Body: modText})...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	main := t.TempDir()
	m, err := f.Fetch(context.Background(), main, tmod, tver, FetchOptions{Record: true})
	if err != nil {
		t.Fatal(err)
	}
	if m.ModHash == "" {
		t.Fatal("no ModHash")
	}
	want, _ := os.ReadFile(filepath.Join(main, "gocpp.sum"))
	if !strings.Contains(string(want), tmod+" "+tver+"/gocpp.mod "+m.ModHash+"\n") || strings.Count(string(want), "\n") != 2 {
		t.Errorf("gocpp.sum = %q", want)
	}
}

func TestModFileWithOtherPathRejected(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3", tfile{Name: "tiny-1.2.3/gocpp.mod", Body: "module github.com/other/fork\n"})...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	_, err := f.Download(context.Background(), tmod, tver, nil)
	if err == nil || !strings.Contains(err.Error(), "declares the module path github.com/other/fork") {
		t.Fatalf("error = %v", err)
	}
	assertNoTemp(t, f)
}

func TestReadOnlyMissingSum(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	main := t.TempDir()
	_, err := f.Fetch(context.Background(), main, tmod, tver, FetchOptions{})
	var ms *MissingSumError
	if !errors.As(err, &ms) {
		t.Fatalf("error = %v", err)
	}
	if h.total() != 0 {
		t.Errorf("%d requests were made for a module that gocpp.sum does not list", h.total())
	}
	if _, err := os.Stat(filepath.Join(main, "gocpp.sum")); err == nil {
		t.Error("gocpp.sum was written")
	}
	if !strings.Contains(err.Error(), "gocpp mod download "+tmod) {
		t.Errorf("message = %v", err)
	}
}

func TestNotFound(t *testing.T) {
	h := newFakeHost(t)
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	_, err := f.Download(context.Background(), tmod, "v9.9.9", nil)
	var nf *ModuleNotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("error = %v", err)
	}
	for _, want := range []string{"github.com/acme/tiny@v9.9.9 was not found", "/acme/tiny/archive/refs/tags/v9.9.9.tar.gz: 404 Not Found", "/acme/tiny/archive/refs/tags/9.9.9.tar.gz: 404", "gocpp get github.com/acme/tiny@latest"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message lacks %q:\n%s", want, err)
		}
	}
	assertNoTemp(t, f)
}

func TestTagWithoutV(t *testing.T) {
	h := newFakeHost(t)
	h.serve("/acme/tiny/archive/refs/tags/1.2.3.tar.gz", tarGz("", library("tiny-1.2.3")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	if _, err := f.Download(context.Background(), tmod, tver, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSubdirModule(t *testing.T) {
	h := newFakeHost(t)
	files := []tfile{
		dirEntry("repo-x/"), {Name: "repo-x/top.txt", Body: "top"},
		dirEntry("repo-x/libs/"), dirEntry("repo-x/libs/small/"), {Name: "repo-x/libs/small/s.h", Body: "// s\n"},
	}
	p := h.tag("acme/repo", "libs/small/v1.0.0", tarGz("", files...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	m, err := f.Download(context.Background(), "github.com/acme/repo/libs/small", "v1.0.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := readTree(t, m.Dir); len(got) != 1 || got["s.h"] == "" {
		t.Errorf("tree = %v", got)
	}
	if h.count(p) != 1 {
		t.Errorf("requests: %v", h.hits)
	}
	// A folder that is not in the archive.
	h.tag("acme/repo", "nothere/v1.0.0", tarGz("", files...))
	if _, err := f.Download(context.Background(), "github.com/acme/repo/nothere", "v1.0.0", nil); err == nil || !strings.Contains(err.Error(), "no folder nothere") {
		t.Errorf("error = %v", err)
	}
}

func TestPseudoVersionDownload(t *testing.T) {
	h := newFakeHost(t)
	p := h.commit("acme/tiny", "abcdef123456", tarGz("abcdef1234567890abcdef1234567890abcdef12", library("tiny-abcdef123456")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	v := "v0.0.0-20240304050607-abcdef123456"
	m, err := f.Download(context.Background(), tmod, v, nil)
	if err != nil {
		t.Fatal(err)
	}
	if h.count(p) != 1 || m.Dir != filepath.Join(f.cfg.CacheDir, "github.com", "acme", "tiny@"+v) {
		t.Errorf("module %+v, hits %v", m, h.hits)
	}
	// An archive for another commit than the version names.
	h.commit("acme/tiny", "111111111111", tarGz("abcdef1234567890abcdef1234567890abcdef12", library("tiny-1")...))
	if _, err := f.Download(context.Background(), tmod, "v0.0.0-20240304050607-111111111111", nil); err == nil || !strings.Contains(err.Error(), "is for the commit abcdef") {
		t.Errorf("error = %v", err)
	}
}

func TestOffline(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3")...))
	cfg := h.config()
	f, _ := New(cfg)
	cleanupCache(t, cfg.CacheDir)
	cfg.Offline = true
	_, err := f.Download(context.Background(), tmod, tver, nil)
	var oe *OfflineError
	if !errors.As(err, &oe) || !strings.Contains(err.Error(), "GOCPP_OFFLINE=1") {
		t.Fatalf("error = %v", err)
	}
	if h.total() != 0 {
		t.Error("the network was used")
	}
	// Fill the cache, then offline works.
	cfg.Offline = false
	if _, err := f.Download(context.Background(), tmod, tver, nil); err != nil {
		t.Fatal(err)
	}
	cfg.Offline = true
	n := h.total()
	if m, err := f.Download(context.Background(), tmod, tver, nil); err != nil || m.Downloaded || h.total() != n {
		t.Errorf("offline from the cache: %v", err)
	}
	// Get with a query that needs the network.
	_, err = f.Get(context.Background(), t.TempDir(), tmod, "latest")
	if err == nil || !strings.Contains(err.Error(), "downloads are off") {
		t.Errorf("Get offline: %v", err)
	}
}

func TestNoCacheDir(t *testing.T) {
	f, _ := New(&Config{})
	if _, err := f.Download(context.Background(), tmod, tver, nil); err == nil || !strings.Contains(err.Error(), "no module cache folder") {
		t.Errorf("error = %v", err)
	}
}

func TestReadOnlyCacheDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	if runtime.GOOS == "windows" {
		t.Skip("a Windows folder has no permission bits: Chmod cannot make it read-only")
	}
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3")...))
	cfg := h.config()
	cleanupCache(t, cfg.CacheDir)
	f, _ := New(cfg)
	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cfg.CacheDir, 0o555); err != nil {
		t.Fatal(err)
	}
	_, err := f.Download(context.Background(), tmod, tver, nil)
	if err == nil || !strings.Contains(err.Error(), "not writable") || !strings.Contains(err.Error(), "GOCPP_MODCACHE") {
		t.Fatalf("error = %v", err)
	}
	// A module that is in a read-only cache is used without writing.
	os.Chmod(cfg.CacheDir, 0o755)
	if _, err := f.Download(context.Background(), tmod, tver, nil); err != nil {
		t.Fatal(err)
	}
	os.Chmod(cfg.CacheDir, 0o555)
	n := h.total()
	if m, err := f.Download(context.Background(), tmod, tver, nil); err != nil || m.Downloaded || h.total() != n {
		t.Errorf("from a read-only cache: %v", err)
	}
	os.Chmod(cfg.CacheDir, 0o755)
}

func TestTimeout(t *testing.T) {
	h := newFakeHost(t)
	h.handle("/acme/tiny/archive/refs/tags/v1.2.3.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	cfg := h.config()
	cfg.Timeout = 300 * time.Millisecond
	cleanupCache(t, cfg.CacheDir)
	f, _ := New(cfg)
	_, err := f.Download(context.Background(), tmod, tver, nil)
	if err == nil {
		t.Fatal("no error")
	}
	assertNoTemp(t, f)
}

func TestCleanModCache(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3")...))
	f := h.fetcher()
	if _, err := f.Download(context.Background(), tmod, tver, nil); err != nil {
		t.Fatal(err)
	}
	if err := CleanModCache(f.cfg.CacheDir); err != nil {
		t.Fatal(err)
	}
	if ents, _ := os.ReadDir(f.cfg.CacheDir); len(ents) != 0 {
		t.Errorf("cache not empty: %v", ents)
	}
	// Not a cache: refused.
	d := t.TempDir()
	mustWrite(t, filepath.Join(d, "precious"), "x")
	if err := CleanModCache(d); err == nil {
		t.Error("CleanModCache removed a folder that is not a module cache")
	}
	if _, err := os.Stat(filepath.Join(d, "precious")); err != nil {
		t.Error("file removed")
	}
	if err := CleanModCache("relative"); err == nil {
		t.Error("relative folder accepted")
	}
	if err := CleanModCache(filepath.Join(d, "nonexistent")); err != nil {
		t.Errorf("missing cache: %v", err)
	}
}

func TestEscapePath(t *testing.T) {
	h := newFakeHost(t)
	h.tag("Acme/Tiny", tver, tarGz("", library("tiny-1.2.3")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	m, err := f.Download(context.Background(), "github.com/Acme/Tiny", tver, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(m.Dir, filepath.FromSlash("github.com/!acme/!tiny@v1.2.3")) {
		t.Errorf("Dir = %s", m.Dir)
	}
}
