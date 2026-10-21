package modfetch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

var smallLimits = Limits{MaxFile: 1 << 16, MaxTree: 1 << 20, MaxFiles: 200}

// repeatedHeaders makes a tar.gz of one folder header, the same n times.
func repeatedHeaders(name string, n int) []byte {
	var one bytes.Buffer
	tw := tar.NewWriter(&one)
	tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0o755})
	block := one.Bytes()[:512]
	var out bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&out, gzip.BestSpeed)
	for i := 0; i < n; i++ {
		gz.Write(block)
	}
	gz.Close()
	return out.Bytes()
}

func TestExtractRepeatedHeadersAreLimited(t *testing.T) {
	for _, name := range []string{"t/", "./"} {
		entries := 0
		testEntryHook = func() { entries++ }
		_, _, err := extractTo(t, repeatedHeaders(name, 400_000), smallLimits)
		testEntryHook = nil
		if err == nil || !strings.Contains(err.Error(), "tar headers") {
			t.Errorf("%q: error = %v", name, err)
		}
		// Every header counts: the extraction stopped at the limit, not after 400k.
		if max := smallLimits.MaxFiles + 1000 + 1; entries > max {
			t.Errorf("%q: %d headers were processed, at most %d", name, entries, max)
		}
	}
}

func TestExtractPaxBombIsLimited(t *testing.T) {
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	big := strings.Repeat("a", 900_000)
	for i := 0; i < 200; i++ {
		if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": big}}); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	arc := gzipBytes(raw.Bytes())
	if len(arc) > 1<<20 {
		t.Fatalf("the test archive is %d bytes", len(arc))
	}
	_, _, err := extractTo(t, arc, smallLimits)
	if err == nil || !strings.Contains(err.Error(), "bytes of data in them") {
		t.Errorf("error = %v", err)
	}
}

func TestExtractDecompressedStreamIsCapped(t *testing.T) {
	// Entries that are allowed one by one, with far more data than their
	// headers are allowed to have: here, padding that the tar format ignores.
	var raw bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&raw, gzip.BestSpeed)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "t/", Typeflag: tar.TypeDir, Mode: 0o755})
	tw.WriteHeader(&tar.Header{Name: "t/f", Typeflag: tar.TypeReg, Mode: 0o644, Size: 3})
	tw.Write([]byte("abc"))
	tw.Flush()
	// A tar reader skips what is between entries only as blocks; a huge
	// entry that is a folder is skipped by reading it.
	tw.WriteHeader(&tar.Header{Name: "t/d/", Typeflag: tar.TypeDir, Mode: 0o755, Size: 40 << 20})
	zeros := make([]byte, 1<<20)
	for i := 0; i < 40; i++ {
		gz.Write(zeros)
	}
	tw.Close()
	gz.Close()
	_, _, err := extractTo(t, raw.Bytes(), smallLimits)
	if err == nil || !strings.Contains(err.Error(), "bytes of data in them") && !strings.Contains(err.Error(), "decompressed") {
		t.Errorf("error = %v", err)
	}
}

func TestExtractContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	arc := filepath.Join(dir, "a")
	os.WriteFile(arc, tarGz("", library("t")...), 0o600)
	os.Mkdir(filepath.Join(dir, "out"), 0o700)
	if _, err := Extract(ctx, arc, filepath.Join(dir, "out"), DefaultLimits); !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v", err)
	}
	// Zip too.
	os.WriteFile(arc, zipOf(library("t")...), 0o600)
	if _, err := Extract(ctx, arc, filepath.Join(dir, "out"), DefaultLimits); !errors.Is(err, context.Canceled) {
		t.Errorf("zip: error = %v", err)
	}
}

// A download whose context ends in the middle of the extraction stops, and
// leaves nothing in the cache folder.
func TestDownloadCancelledMidExtraction(t *testing.T) {
	h := newFakeHost(t)
	var files []tfile
	files = append(files, dirEntry("tiny-1.2.3/"))
	for i := 0; i < 50; i++ {
		files = append(files, tfile{Name: fmt.Sprintf("tiny-1.2.3/f%d.txt", i), Body: "x"})
	}
	h.tag("acme/tiny", tver, tarGz("", files...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	testEntryHook = func() {
		if n++; n == 5 {
			cancel()
		}
	}
	defer func() { testEntryHook = nil }()
	_, err := f.Download(ctx, tmod, tver, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if n > 10 {
		t.Errorf("%d entries were looked at after the cancel", n)
	}
	assertNoTemp(t, f)
	if m, _ := f.cached(context.Background(), tmod, tver, nil); m != nil {
		t.Error("in the cache")
	}
}

func TestHashDirContextCancelled(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a"), "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := HashDir(ctx, dir, "example.com/m", "v1.0.0"); !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v", err)
	}
}

func TestPortableNames(t *testing.T) {
	reject := []string{".git.", ".git ", "GIT~1", "git~12", ".git::$INDEX_ALLOCATION", ".g\u200cit", ".\u200bgit", ".GIT", "con", "CON", "aux.txt", "NUL.c",
		"f::$DATA", "f:stream", "C:", "a.", "a ", "a..", "COM1", "lpt9.x", "com\u00b9", "CONIN$", "prn.", "nul "}
	for _, name := range reject {
		for _, in := range []string{"t/" + name, "t/sub/" + name + "/x"} {
			if _, err := cleanName(in); err == nil {
				t.Errorf("cleanName(%q) was accepted", in)
			}
		}
	}
	accept := []string{"git", "gitignore", ".gitignore", ".gitattributes", ".github", "config.h", "console.c", "auxiliary.h", "com10", "lpt0", "nul_x", "a.b", "a b", "-x", "~tmp", "git~", "git~x", "x.GIT", "..foo"}
	for _, name := range accept {
		if got, err := cleanName("t/" + name); err != nil || got != "t/"+name {
			t.Errorf("cleanName(%q) = %q, %v", name, got, err)
		}
	}
	// A name with a combining mark: e + U+0301 and the composed form would be one file on macOS.
	for _, name := range []string{"e\u0301", "caf\u00e9/e\u0301.h", "\u0300x"} {
		if _, err := cleanName("t/" + name); err == nil || !strings.Contains(err.Error(), "combining mark") {
			t.Errorf("cleanName(%q) = %v", name, err)
		}
	}
	if _, err := cleanName("t/caf\u00e9.h"); err != nil {
		t.Errorf("a composed name: %v", err)
	}
	// Names that a case-insensitive or normalizing file system joins.
	for _, pair := range [][2]string{{"s", "\u017f"}, {"ab", "a\u200cb"}, {"k", "\u212a"}, {"A", "a"}} {
		arc := tarGz("", dirEntry("t/"), tfile{Name: "t/" + pair[0], Body: "1"}, tfile{Name: "t/" + pair[1], Body: "2"})
		if _, _, err := extractTo(t, arc, DefaultLimits); err == nil || !strings.Contains(err.Error(), "differs only in case") {
			t.Errorf("%q and %q: %v", pair[0], pair[1], err)
		}
	}
}

func TestReadOnlyRefusesMissingModFileLine(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3", tfile{Name: "tiny-1.2.3/gocpp.mod", Body: "module github.com/acme/tiny\n"})...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	main := t.TempDir()
	m, err := f.Download(context.Background(), tmod, tver, nil)
	if err != nil {
		t.Fatal(err)
	}
	// gocpp.sum has the line of the files, but not the gocpp.mod line.
	line := tmod + " " + tver + " " + m.Hash + "\n"
	mustWrite(t, filepath.Join(main, "gocpp.sum"), line)
	_, err = f.Fetch(context.Background(), main, tmod, tver, FetchOptions{})
	var ms *MissingSumError
	if !errors.As(err, &ms) || ms.Line != "gocpp.mod" || !strings.Contains(err.Error(), tver+"/gocpp.mod") {
		t.Fatalf("error = %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(main, "gocpp.sum")); string(got) != line {
		t.Errorf("a read-only fetch wrote gocpp.sum: %q", got)
	}
	// With Record the line is added.
	if _, err := f.Fetch(context.Background(), main, tmod, tver, FetchOptions{Record: true}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(main, "gocpp.sum")); !strings.Contains(string(got), "/gocpp.mod h1:") {
		t.Errorf("gocpp.sum = %q", got)
	}
}

func TestRecordedMessage(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	var out bytes.Buffer
	f.cfg.Stderr = &out
	main := t.TempDir()
	for i := 0; i < 2; i++ {
		if _, err := f.Fetch(context.Background(), main, tmod, tver, FetchOptions{Record: true}); err != nil {
			t.Fatal(err)
		}
	}
	if n := strings.Count(out.String(), "gocpp: recorded github.com/acme/tiny@v1.2.3 in gocpp.sum"); n != 1 {
		t.Errorf("the message appeared %d times:\n%s", n, out.String())
	}
}

func TestLockFallbackWithoutFileLocks(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3")...))
	old := flockImpl
	defer func() { flockImpl = old }()

	flockImpl = func(*os.File) (bool, error) { return false, errLockUnsupported }
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	if _, err := f.Fetch(context.Background(), t.TempDir(), tmod, tver, FetchOptions{Record: true}); err != nil {
		t.Fatalf("with no file locks: %v", err)
	}

	// Another error is not hidden.
	flockImpl = func(*os.File) (bool, error) { return false, errors.New("disk on fire") }
	f2 := h.fetcher()
	cleanupCache(t, f2.cfg.CacheDir)
	if _, err := f2.Download(context.Background(), tmod, tver, nil); err == nil || !strings.Contains(err.Error(), "disk on fire") {
		t.Errorf("error = %v", err)
	}
}

func TestInProcessLockHonorsContext(t *testing.T) {
	f, _ := New(&Config{CacheDir: filepath.Join(t.TempDir(), "cache")})
	unlock, err := f.lockFile(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := f.lockFile(ctx, "x"); err == nil {
		t.Fatal("the lock was taken twice")
	}
}

func TestEditorModeNeverDownloadsOrWrites(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3")...))
	cfg := h.config()
	cleanupCache(t, cfg.CacheDir)
	t.Setenv("GOCPP_MODCACHE", cfg.CacheDir)
	t.Setenv("GOCPP_OFFLINE", "")
	t.Setenv("GOCPP_MODFLAGS", "")
	main := t.TempDir()

	SetOffline(true)
	SetOffline(true) // a second language server in the process
	_, err := Dir(main, tmod, tver)
	var oe *OfflineError
	if !errors.As(err, &oe) || oe.Why == "" || strings.Contains(err.Error(), "GOCPP_OFFLINE") || !strings.Contains(err.Error(), "language server does not download") {
		t.Errorf("error = %v", err)
	}
	if h.total() != 0 {
		t.Error("the network was used")
	}
	SetOffline(false) // one of them ends: the other is still an editor
	if _, err := Dir(main, tmod, tver); !errors.As(err, &oe) {
		t.Errorf("after one SetOffline(false): %v", err)
	}
	SetOffline(false)
	SetOffline(false) // too many: no harm
	if forceOffline.Load() != 0 {
		t.Errorf("forceOffline = %d", forceOffline.Load())
	}

	// A module that is in the cache but not in gocpp.sum: an editor does not record it.
	f, _ := New(cfg)
	if _, err := f.Download(context.Background(), tmod, tver, nil); err != nil {
		t.Fatal(err)
	}
	SetOffline(true)
	defer SetOffline(false)
	_, err = Dir(main, tmod, tver)
	var ms *MissingSumError
	if !errors.As(err, &ms) {
		t.Errorf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(main, "gocpp.sum")); err == nil {
		t.Error("the language server wrote gocpp.sum")
	}
}

func TestSaveSumsIsAtomic(t *testing.T) {
	dir := t.TempDir()
	sum := filepath.Join(dir, "gocpp.sum")
	var init strings.Builder
	line := func(i int) SumLine {
		h := sha256.Sum256([]byte(fmt.Sprint(i)))
		return SumLine{Path: fmt.Sprintf("example.com/m%05d", i), Version: "v1.0.0", Hash: "h1:" + base64.StdEncoding.EncodeToString(h[:])}
	}
	for i := 0; i < 4000; i++ {
		l := line(i)
		fmt.Fprintf(&init, "%s %s %s\n", l.Path, l.Version, l.Hash)
	}
	mustWrite(t, sum, init.String())
	f, _ := New(&Config{CacheDir: filepath.Join(dir, "cache")})
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(stop)
		for i := 4000; i < 4150; i++ {
			if _, err := f.saveSums(sum, []SumLine{line(i)}); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	reads := 0
	for done := false; !done; {
		select {
		case <-stop:
			done = true
		default:
		}
		s, err := LoadSums(dir)
		if err != nil {
			t.Fatalf("a reader saw a damaged gocpp.sum after %d reads: %v", reads, err)
		}
		if lines := len(s.Lines()); lines < 4000 {
			t.Fatalf("a reader saw a damaged gocpp.sum after %d reads: %d lines", reads, lines)
		}
		reads++
	}
	wg.Wait()
	if reads < 3 {
		t.Logf("only %d reads", reads)
	}
}

func TestSidecarNeverCollidesWithAModuleFolder(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", "v1.0.0-x", tarGz("", library("tiny-1")...))
	h.tag("acme/tiny", "v1.0.0-x.h1", tarGz("", library("tiny-2", tfile{Name: "tiny-2/more", Body: "m"})...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	a, err := f.Download(context.Background(), tmod, "v1.0.0-x", nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := f.Download(context.Background(), tmod, "v1.0.0-x.h1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.Hash == b.Hash || a.Dir == b.Dir {
		t.Fatalf("%+v %+v", a, b)
	}
	a2, err := f.cached(context.Background(), tmod, "v1.0.0-x", nil)
	if err != nil || a2.Hash != a.Hash {
		t.Errorf("cached a = %+v, %v", a2, err)
	}
	if h, _, err := readSidecar(f.sidecar(tmod, "v1.0.0-x")); err != nil || h != a.Hash {
		t.Errorf("sidecar of a: %v %v", h, err)
	}
}

func TestURLTemplateHasNoRefsAndAsksNobodyElse(t *testing.T) {
	h := newFakeHost(t)
	cfg := h.config()
	cfg.URLs = []URLRule{{Prefix: "github.com", Template: h.srv.URL + "/{repo}/x/{version}.tar.gz"}}
	cleanupCache(t, cfg.CacheDir)
	f, _ := New(cfg)
	_, err := f.Get(context.Background(), t.TempDir(), tmod, "latest")
	if err == nil || !strings.Contains(err.Error(), "URL template has no list of tags") {
		t.Errorf("error = %v", err)
	}
	if h.total() != 0 {
		t.Errorf("requests: %v", h.hits)
	}
}

func TestMajorSuffixModuleFromCommit(t *testing.T) {
	h := newFakeHost(t)
	commit := "abcdef0123456789abcdef0123456789abcdef01"
	h.refs("acme/tiny", commit, "main", map[string]string{"v2.1.0": strings.Repeat("2", 40)}, map[string]string{"main": commit})
	h.commit("acme/tiny", commit, tarGz(commit, library("tiny-x")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	main := t.TempDir()
	m, err := f.Get(context.Background(), main, tmod+"/v2", "main")
	if err != nil {
		t.Fatal(err)
	}
	if want := "v2.0.0-20240304050607-abcdef012345"; m.Version != want {
		t.Errorf("version = %s, want %s", m.Version, want)
	}
	// And a version of that module is a download of its tag.
	h.tag("acme/tiny", "v2.1.0", tarGz("", library("tiny-2.1.0")...))
	m, err = f.Get(context.Background(), main, tmod+"/v2", "latest")
	if err != nil || m.Version != "v2.1.0" {
		t.Errorf("latest: %+v %v", m, err)
	}
}

func TestInvalidModuleLineInDownloadedModFile(t *testing.T) {
	h := newFakeHost(t)
	for name, body := range map[string]string{"1.0.0": "module\n", "1.0.1": "gocpp 0.1\n", "1.0.2": "module a b c\n"} {
		h.tag("acme/tiny", "v"+name, tarGz("", library("tiny-"+name, tfile{Name: "tiny-" + name + "/gocpp.mod", Body: body})...))
		f := h.fetcher()
		cleanupCache(t, f.cfg.CacheDir)
		_, err := f.Download(context.Background(), tmod, "v"+name, nil)
		if err == nil || !strings.Contains(err.Error(), "no valid module line") {
			t.Errorf("%q: error = %v", body, err)
		}
		assertNoTemp(t, f)
	}
}

func TestTagListIsQuoted(t *testing.T) {
	repo, _ := ParseModulePath(tmod)
	rl := &RefList{Tags: map[string]string{"evil\"\nERROR: pwned": strings.Repeat("1", 40)}}
	got := tagList(rl, repo)
	if strings.Contains(got, "\n") || !strings.Contains(got, `\n`) {
		t.Errorf("tagList = %q", got)
	}
}

func TestSymlinkedFilesAndDevices(t *testing.T) {
	dir := t.TempDir()
	// A device that never ends is not read: /dev/zero, or NUL (which is empty) on Windows.
	device := "/dev/zero"
	if runtime.GOOS == "windows" {
		device = os.DevNull
	}
	if _, err := ReadRegular(device, 1<<20); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("%s: %v", device, err)
	}
	if _, err := ReadRegular(dir, 1<<20); err == nil {
		t.Error("a folder was read")
	}
	mustWrite(t, filepath.Join(dir, "big"), strings.Repeat("x", 100))
	if _, err := ReadRegular(filepath.Join(dir, "big"), 50); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("big file: %v", err)
	}
	real := filepath.Join(dir, "real.sum")
	mustWrite(t, real, "")
	link := filepath.Join(dir, "gocpp.sum")
	if err := os.Symlink(real, link); err != nil {
		t.Skip(err)
	}
	f, _ := New(&Config{CacheDir: filepath.Join(dir, "cache")})
	h := sha256.Sum256([]byte("x"))
	_, err := f.saveSums(link, []SumLine{{"example.com/m", "v1.0.0", "h1:" + base64.StdEncoding.EncodeToString(h[:])}})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Errorf("saveSums through a link: %v", err)
	}
	if st, _ := os.Lstat(link); st.Mode()&os.ModeSymlink == 0 {
		t.Error("the link was replaced")
	}
	if err := CheckWritable(filepath.Join(dir, "none")); err != nil {
		t.Errorf("CheckWritable of a missing file: %v", err)
	}
}

func TestDefaultCacheDirWithoutHome(t *testing.T) {
	// os.UserCacheDir looks at XDG_CACHE_HOME and HOME, and at LocalAppData on Windows.
	cacheVar := "XDG_CACHE_HOME"
	if runtime.GOOS == "windows" {
		cacheVar = "LocalAppData"
	}
	t.Setenv("HOME", "")
	t.Setenv(cacheVar, "")
	t.Setenv("GOCPP_MODCACHE", "")
	cfg, err := ConfigFromEnv(os.Getenv, nil)
	if err != nil || cfg.CacheDir != "" {
		t.Errorf("without HOME: %q, %v", cfg.CacheDir, err)
	}
	cacheBase := t.TempDir()
	t.Setenv(cacheVar, cacheBase)
	cfg, _ = ConfigFromEnv(os.Getenv, nil)
	if cfg.CacheDir != filepath.Join(cacheBase, "gocpp", "mod") {
		t.Errorf("default = %q", cfg.CacheDir)
	}
	t.Setenv("GOCPP_MODCACHE", "off")
	cfg, _ = ConfigFromEnv(os.Getenv, nil)
	if cfg.CacheDir != "" {
		t.Errorf("off = %q", cfg.CacheDir)
	}
}

func TestBaseURLsHaveNoQueryOrFragment(t *testing.T) {
	for _, env := range []struct{ name, val string }{
		{"GOCPP_MODPROXY", "https://p.example/x?token=1"},
		{"GOCPP_MODPROXY", "https://p.example/x#frag"},
		{"GOCPP_MODURL", "github.com=https://m.example?a=b"},
		{"GOCPP_MODURL", "https://m.example/{path}#x"},
	} {
		t.Setenv(env.name, env.val)
		if _, err := ConfigFromEnv(os.Getenv, nil); err == nil {
			t.Errorf("%s=%s was accepted", env.name, env.val)
		}
		t.Setenv(env.name, "")
	}
}

func TestVendorSweepsOldTempFolders(t *testing.T) {
	h := newFakeHost(t)
	dir, f, file := project(t, h)
	old := filepath.Join(dir, ".vendor-tmp-dead")
	os.MkdirAll(filepath.Join(old, "x"), 0o755)
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(old, past, past)
	fresh := filepath.Join(dir, ".vendor-tmp-alive")
	os.Mkdir(fresh, 0o755)
	if err := f.Vendor(context.Background(), dir, file); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("the old folder was kept")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("a recent folder (another gocpp may use it) was removed")
	}
}

// Temporary files that a killed run left next to gocpp.mod and gocpp.sum are
// removed by the next write once they are a day old; nothing else is touched.
func TestStaleWriteTempsAreSwept(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-48 * time.Hour)
	keep := map[string]bool{".gocpp-notes": true, "gocpp.mod-123": true, ".gocpp.sum-456": true /* fresh */, ".gocpp-789": true /* fresh: below */}
	for _, n := range []string{".gocpp.sum-111", ".gocpp.mod-222", ".gocpp-333"} {
		mustWrite(t, filepath.Join(dir, n), "x")
		os.Chtimes(filepath.Join(dir, n), old, old)
	}
	for _, n := range []string{".gocpp-notes", "gocpp.mod-123"} {
		mustWrite(t, filepath.Join(dir, n), "x")
		os.Chtimes(filepath.Join(dir, n), old, old)
	}
	mustWrite(t, filepath.Join(dir, ".gocpp.sum-456"), "x") // a run in progress
	mustWrite(t, filepath.Join(dir, ".gocpp-789"), "x")
	f, _ := New(&Config{CacheDir: filepath.Join(dir, "cache")})
	line := SumLine{Path: "example.com/m", Version: "v1.0.0", Hash: "h1:" + base64.StdEncoding.EncodeToString(make([]byte, 32))}
	if _, err := f.saveSums(filepath.Join(dir, "gocpp.sum"), []SumLine{line}); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(filepath.Join(dir, "other"), []byte("x")); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{".gocpp.sum-111", ".gocpp.mod-222", ".gocpp-333"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			t.Errorf("%s was not removed", n)
		}
	}
	for n := range keep {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("%s was removed", n)
		}
	}
}
