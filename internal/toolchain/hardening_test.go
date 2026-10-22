package toolchain

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The tests of the review round of step 6.13: GOCPP_OFFLINE, the redirect policy, the symbolic-link
// rule, the end of the xz stream, an archive without the compiler, stalled and abandoned downloads.

func TestOfflineStopsTheDownload(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	for _, mode := range []Mode{Auto, Pinned} {
		cfg := config(t, s, name, body)
		cfg.Mode = mode
		cfg.Offline = true
		_, err := resolve(t, cfg)
		if err == nil || !strings.Contains(err.Error(), "GOCPP_OFFLINE is set") {
			t.Errorf("mode %s: error = %v, want one that names GOCPP_OFFLINE", mode, err)
		}
		if s.hits.Load() != 0 {
			t.Fatalf("mode %s: the network was used with Offline", mode)
		}
		if _, ok := cfg.Exists(); ok {
			t.Errorf("mode %s: a folder was made", mode)
		}
	}
	// A toolchain that is already there is used offline.
	cfg := config(t, s, name, body)
	if _, err := resolve(t, cfg); err != nil {
		t.Fatal(err)
	}
	hits := s.hits.Load()
	cfg.Offline = true
	if f, err := resolve(t, cfg); err != nil || !strings.Contains(f.Source, "cached") || s.hits.Load() != hits {
		t.Errorf("offline with the toolchain cached = %+v, %v", f, err)
	}
}

func TestFromEnvOffline(t *testing.T) {
	for v, want := range map[string]bool{
		"": false, "0": false, "false": false, "no": false, "off": false, "OFF": false, " Off ": false,
		"1": true, "true": true, "yes": true, "on": true, "On": true, "2": true, "ture": true,
	} {
		t.Setenv("GOCPP_OFFLINE", v)
		cfg, err := FromEnv(true)
		if err != nil || cfg.Offline != want {
			t.Errorf("GOCPP_OFFLINE=%q: Offline = %v, %v; want %v", v, cfg.Offline, err, want)
		}
	}
}

func TestRedirectDowngradeRefused(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the plain http server got %s: the redirect was followed", r.URL)
		w.Write(body)
	}))
	defer plain.Close()
	tls := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+"/"+name, http.StatusFound)
	}))
	defer tls.Close()
	cfg := config(t, &server{Server: tls, body: body}, name, body)
	_, err := resolve(t, cfg)
	if err == nil || !strings.Contains(err.Error(), "only https is accepted") {
		t.Errorf("error = %v, want the refusal of the redirect", err)
	}
	if _, ok := cfg.Exists(); ok {
		t.Error("a folder was made")
	}
}

func TestRedirectToAnotherHTTPSHostIsFollowed(t *testing.T) {
	// GitHub serves release assets from a storage host: another host over https is fine, the hash
	// vouches for the contents.
	const name = "t.tar"
	body := goodTar(t)
	storage := newServer(t, name, body, nil)
	front := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, storage.URL+r.URL.Path, http.StatusFound)
	}))
	defer front.Close()
	cfg := config(t, storage, name, body)
	cfg.BaseURL = front.URL
	// The client must trust both test servers.
	cfg.Client = &http.Client{Transport: &http.Transport{TLSClientConfig: storage.Client().Transport.(*http.Transport).TLSClientConfig}}
	pool := cfg.Client.Transport.(*http.Transport).TLSClientConfig.RootCAs
	pool.AddCert(front.Certificate())
	if f, err := resolve(t, cfg); err != nil || !strings.Contains(f.Source, "downloaded") {
		t.Errorf("through an https redirect: %+v, %v", f, err)
	}
	if storage.hits.Load() != 1 {
		t.Errorf("the storage host got %d requests", storage.hits.Load())
	}
}

func TestRedirectLoopStops(t *testing.T) {
	var self *httptest.Server
	self = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, self.URL+r.URL.Path, http.StatusFound)
	}))
	defer self.Close()
	body := goodTar(t)
	cfg := config(t, &server{Server: self, body: body}, "t.tar", body)
	if _, err := resolve(t, cfg); err == nil || !strings.Contains(err.Error(), "10 redirects") {
		t.Errorf("error = %v", err)
	}
}

// A link whose target goes through an earlier link to a folder is judged by where it goes.
func TestUnpackLinkThroughLink(t *testing.T) {
	for _, c := range []struct {
		name    string
		entries []entry
		wantErr string
	}{
		{"chain out", []entry{
			{name: fakeTop + "/d/e", typ: tar.TypeSymlink, link: ".."},
			{name: fakeTop + "/d/e/f", typ: tar.TypeSymlink, link: "../.."},
		}, "leaves the toolchain"},
		{"two steps", []entry{
			{name: fakeTop + "/a/b", typ: tar.TypeSymlink, link: "../c"},
			{name: fakeTop + "/c/d", typ: tar.TypeSymlink, link: "../a/b/../../.."},
		}, "leaves the toolchain"},
		{"through a link inside", []entry{
			{name: fakeTop + "/real/x", body: "x"},
			{name: fakeTop + "/alias", typ: tar.TypeSymlink, link: "real"},
			{name: fakeTop + "/alias/y", typ: tar.TypeSymlink, link: "x"},
		}, ""},
		{"loop", []entry{
			{name: fakeTop + "/p", typ: tar.TypeSymlink, link: "q"},
			{name: fakeTop + "/q", typ: tar.TypeSymlink, link: "p"},
			{name: fakeTop + "/r", typ: tar.TypeSymlink, link: "p/z"},
		}, "too many links"},
	} {
		t.Run(c.name, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "root")
			os.Mkdir(root, 0o755)
			err := unpack(bytes.NewReader(tarOf(t, c.entries...)), root, false)
			switch {
			case c.wantErr == "" && err != nil:
				t.Errorf("unpack error = %v", err)
			case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
				t.Errorf("unpack error = %v, want %q", err, c.wantErr)
			}
			// Whatever happened, nothing in the root points outside it.
			filepath.WalkDir(root, func(p string, d os.DirEntry, _ error) error {
				if d != nil && d.Type()&os.ModeSymlink != 0 {
					if dest, err := filepath.EvalSymlinks(p); err == nil && !strings.HasPrefix(dest, root) {
						t.Errorf("%s points to %s, outside the root", p, dest)
					}
				}
				return nil
			})
		})
	}
}

func xzOf(t *testing.T, data []byte, args ...string) []byte {
	t.Helper()
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("no xz program to make the archive")
	}
	cmd := exec.Command("xz", append([]string{"-c"}, args...)...)
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The tar ends before the compressed stream does: the checks of the last block, the index and the
// footer must still run.
func TestUnpackDrainsTheXZStream(t *testing.T) {
	good := xzOf(t, goodTar(t), "-C", "sha256")
	root := filepath.Join(t.TempDir(), "good")
	os.Mkdir(root, 0o755)
	if err := unpack(bytes.NewReader(good), root, true); err != nil {
		t.Fatalf("a good archive: %v", err)
	}
	// Damage the stored check, the index, the footer; cut each of them off.
	for i := 1; i <= 12+8+32; i += 3 {
		bad := bytes.Clone(good)
		bad[len(bad)-i] ^= 0xff
		root := filepath.Join(t.TempDir(), "root")
		os.Mkdir(root, 0o755)
		if err := unpack(bytes.NewReader(bad), root, true); err == nil {
			t.Errorf("flipping the byte %d from the end is accepted", i)
		}
	}
	for _, drop := range []int{1, 12, 12 + 8, 12 + 8 + 32, 12 + 8 + 32 + 4} {
		root := filepath.Join(t.TempDir(), "root")
		os.Mkdir(root, 0o755)
		if err := unpack(bytes.NewReader(good[:len(good)-drop]), root, true); err == nil {
			t.Errorf("an archive without its last %d bytes is accepted", drop)
		}
	}
}

func TestArchiveWithoutCompilerLeavesNothing(t *testing.T) {
	const name = "t.tar"
	body := tarOf(t, entry{name: fakeTop + "/bin/other", body: "x"})
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	for i := 0; i < 2; i++ {
		_, err := resolve(t, cfg)
		if err == nil || !strings.Contains(err.Error(), "has no bin/x86_64-w64-mingw32-clang++") {
			t.Fatalf("run %d: error = %v", i+1, err)
		}
		if dir, ok := cfg.Exists(); ok {
			t.Fatalf("run %d: %s was left behind", i+1, dir)
		}
	}
}

func TestUnfinishedFolderIsReplaced(t *testing.T) {
	// A folder without the compiler (an interrupted older gocpp, or made by hand) does not stop a
	// download, and Remove takes it away.
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	dir := cfg.Dir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "bin", "junk"), []byte("x"), 0o444)
	os.Chmod(filepath.Join(dir, "bin"), 0o555)
	if removed, err := cfg.Remove(); err != nil || removed != dir {
		t.Fatalf("Remove of an unfinished folder = %q, %v", removed, err)
	}
	os.MkdirAll(filepath.Join(dir, "bin"), 0o755)
	if f, err := resolve(t, cfg); err != nil || !strings.Contains(f.Source, "downloaded") {
		t.Errorf("download over an unfinished folder: %+v, %v", f, err)
	}
}

func TestStalledDownloadStops(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	release := make(chan struct{})
	s := newServer(t, name, body, func(w http.ResponseWriter, r *http.Request, body []byte) bool {
		w.Header().Set("Content-Length", "1000000")
		w.Write(body[:10])
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
		return true
	})
	defer close(release)
	cfg := config(t, s, name, body)
	cfg.Archives[0].Size = 1000000
	cfg.IdleTimeout = 300 * time.Millisecond
	done := make(chan error, 1)
	go func() { _, err := resolve(t, cfg); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a stalled download succeeded")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("a download that stalls mid-body did not stop")
	}
	if _, ok := cfg.Exists(); ok {
		t.Error("a folder was made")
	}
}

func TestSlowButSteadyDownloadContinues(t *testing.T) {
	// The idle timeout restarts at every chunk: a slow download is not stopped.
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, func(w http.ResponseWriter, r *http.Request, body []byte) bool {
		w.Header().Set("Content-Length", "")
		for i := 0; i < len(body); i += 512 {
			end := min(i+512, len(body))
			w.Write(body[i:end])
			w.(http.Flusher).Flush()
			time.Sleep(100 * time.Millisecond)
		}
		return true
	})
	cfg := config(t, s, name, body)
	cfg.IdleTimeout = 400 * time.Millisecond
	if _, err := resolve(t, cfg); err != nil {
		t.Errorf("a slow, steady download: %v", err)
	}
}

func TestStaleDownloadFoldersAreSwept(t *testing.T) {
	const name = "t.tar"
	body := goodTar(t)
	s := newServer(t, name, body, nil)
	cfg := config(t, s, name, body)
	parent := filepath.Dir(cfg.Dir())
	os.MkdirAll(parent, 0o755)
	old := filepath.Join(parent, ".download-old")
	fresh := filepath.Join(parent, ".download-fresh")
	for _, d := range []string{old, fresh} {
		os.MkdirAll(filepath.Join(d, "root", "bin"), 0o755)
		os.WriteFile(filepath.Join(d, "root", "bin", "f"), []byte("x"), 0o444)
		os.Chmod(filepath.Join(d, "root", "bin"), 0o555)
	}
	t.Cleanup(func() { MakeWritable(parent) })
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(old, past, past)
	if _, err := resolve(t, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(old); err == nil {
		t.Error("the stale .download folder is still there")
	}
	if _, err := os.Lstat(fresh); err != nil {
		t.Error("a recent .download folder (maybe a download that runs) was removed")
	}
}

func TestMessagesAreNotDoubleNegatives(t *testing.T) {
	// A Linux host builds for Windows, whatever system the test runs on (a Windows host looks in
	// the folders of its installs and says so).
	cfg := Config{Mode: Local, Host: "linux/amd64", Cache: t.TempDir(),
		LookPath: func(string) (string, error) { return "", errors.New("no") }}
	_, err := resolve(t, cfg)
	if err == nil || strings.Contains(err.Error(), "is not in PATH") || !strings.Contains(err.Error(), "neither x86_64-w64-mingw32-clang++ nor x86_64-w64-mingw32-g++ is in PATH") {
		t.Errorf("local: %v", err)
	}
	cfg.Mode = Auto
	cfg.Offline = true
	_, err = resolve(t, cfg)
	if err == nil || strings.Contains(err.Error(), "is not in PATH") || !strings.Contains(err.Error(), "neither x86_64-w64-mingw32-clang++ nor") {
		t.Errorf("auto: %v", err)
	}
}

func TestDefaultModeFlip(t *testing.T) {
	// DefaultMode is the one line that makes the download opt-in (Local) or default (Auto).
	if m, err := ModeOf(""); err != nil || m != DefaultMode {
		t.Errorf("ModeOf(\"\") = %q, %v; want %q", m, err, DefaultMode)
	}
	cfg := Config{Cache: t.TempDir(), Download: true, LookPath: func(string) (string, error) { return "", errors.New("no") }}
	cfg.Archives = []Archive{{Host: "linux/amd64", Name: "x", SHA256: ""}}
	cfg.Host = "linux/amd64"
	_, err := Resolve(context.Background(), cfg, "windows", "amd64") // no mode: the default
	wantDownload := DefaultMode == Auto
	if got := err != nil && strings.Contains(err.Error(), "pinned toolchain hash not set"); got != wantDownload {
		t.Errorf("with the default mode %q, the pinned toolchain was tried = %v: %v", DefaultMode, got, err)
	}
}
