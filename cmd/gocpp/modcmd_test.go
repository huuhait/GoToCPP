package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// modHost is an HTTPS server that has the URLs of GitHub for the module
// github.com/acme/tiny: a module with a gocpp.mod file and a package.
type modHost struct {
	srv  *httptest.Server
	tags map[string][]byte // tag -> archive
}

func tinyArchive(answer int) []byte {
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)
	top := fmt.Sprintf("tiny-%d", answer)
	tw.WriteHeader(&tar.Header{Name: top + "/", Typeflag: tar.TypeDir, Mode: 0o755, ModTime: time.Unix(1700000000, 0)})
	for name, body := range map[string]string{
		"gocpp.mod": "module github.com/acme/tiny\n",
		"tiny.gp":   fmt.Sprintf("package tiny\n\nfunc Answer() int { return %d }\n", answer),
		"tiny.h":    fmt.Sprintf("#pragma once\ninline int answer() { return %d; }\n", answer),
	} {
		tw.WriteHeader(&tar.Header{Name: top + "/" + name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body)), ModTime: time.Unix(1700000000, 0)})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return raw.Bytes()
}

func startModHost(t *testing.T) *modHost {
	t.Setenv("GOCPP_GATE_PHASE", "04") // header imports are a Phase 04 feature
	h := &modHost{tags: map[string][]byte{}}
	h.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const pre = "/acme/tiny/archive/refs/tags/"
		switch {
		case strings.HasPrefix(r.URL.Path, pre) && strings.HasSuffix(r.URL.Path, ".tar.gz"):
			tag := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, pre), ".tar.gz")
			if b, ok := h.tags[tag]; ok {
				w.Write(b)
				return
			}
		case r.URL.Path == "/acme/tiny.git/info/refs":
			var b bytes.Buffer
			pkt := func(s string) { fmt.Fprintf(&b, "%04x%s", len(s)+4, s) }
			pkt("# service=git-upload-pack\n")
			b.WriteString("0000")
			first := true
			for tag := range h.tags {
				line := strings.Repeat("1", 40) + " refs/tags/" + tag
				if first {
					line += "\x00agent=test"
					first = false
				}
				pkt(line + "\n")
			}
			b.WriteString("0000")
			w.Write(b.Bytes())
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(h.srv.Close)
	return h
}

// env points the module downloads of this test at the host, with a module
// cache of its own.
func (h *modHost) env(t *testing.T) (cache string) {
	cert := h.srv.Certificate()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0o644); err != nil {
		t.Fatal(err)
	}
	cache = filepath.Join(t.TempDir(), "modcache")
	t.Cleanup(func() { removeReadOnly(cache) })
	t.Setenv("GOCPP_MODCACHE", cache)
	t.Setenv("GOCPP_MODURL", "github.com="+h.srv.URL)
	t.Setenv("GOCPP_MODCACERT", ca)
	t.Setenv("GOCPP_OFFLINE", "")
	t.Setenv("GOCPP_MODFLAGS", "")
	t.Setenv("GOCPP_MODPROXY", "")
	return cache
}

func removeReadOnly(dir string) {
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			os.Chmod(p, 0o700)
		}
		return nil
	})
	os.RemoveAll(dir)
}

// captureStderr returns what f writes to the process's standard error (the
// loader reports a download there, not to the stderr of the command).
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = pw
	done := make(chan string)
	go func() {
		var b bytes.Buffer
		b.ReadFrom(pr)
		done <- b.String()
	}()
	defer func() { os.Stderr = old }()
	f()
	pw.Close()
	return <-done
}

// checkLoads fails the test unless "gocpp check ." worked: the loader found
// the module (in the cache, in vendor/) and its header.
func checkLoads(t *testing.T, r result) {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("the loader did not find the module: %+v", r)
	}
}

const modApp = `package main

import (
	"fmt"

	"github.com/acme/tiny/tiny.h"
)

func main() { fmt.Println(tiny.answer()) }
`

func TestGetAndModCommands(t *testing.T) {
	h := startModHost(t)
	h.tags["v1.2.3"] = tinyArchive(42)
	h.tags["v1.10.0"] = tinyArchive(43)
	cache := h.env(t)
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n\ngocpp 0.1\n", "main.gp": modApp})

	// Before: the import fails, since nothing requires the module.
	r := gocppRun("", false, "check", ".")
	if r.code == 0 || !strings.Contains(r.stderr, "no required module provides header github.com/acme/tiny/tiny.h") {
		t.Fatalf("check before get: %+v", r)
	}

	// get @latest picks the highest tag.
	r = gocppRun("", false, "get", "github.com/acme/tiny")
	if r.code != 0 || !strings.Contains(r.stderr, "gocpp: downloading github.com/acme/tiny v1.10.0") || !strings.Contains(r.stderr, "gocpp: added github.com/acme/tiny v1.10.0") {
		t.Fatalf("get: %+v", r)
	}
	mod := readFile(t, filepath.Join(dir, "gocpp.mod"))
	if mod != "module example.com/app\n\ngocpp 0.1\n\nrequire github.com/acme/tiny v1.10.0\n" {
		t.Errorf("gocpp.mod = %q", mod)
	}
	sum := readFile(t, filepath.Join(dir, "gocpp.sum"))
	if !strings.HasPrefix(sum, "github.com/acme/tiny v1.10.0 h1:") || !strings.Contains(sum, "github.com/acme/tiny v1.10.0/gocpp.mod h1:") {
		t.Errorf("gocpp.sum = %q", sum)
	}
	// The package of the module is found in the module cache.
	checkLoads(t, gocppRun("", false, "check", "."))

	// An older version, and back to a tag with a prefix.
	r = gocppRun("", false, "get", "github.com/acme/tiny@v1.2.3")
	if r.code != 0 || !strings.Contains(r.stderr, "downgraded github.com/acme/tiny v1.10.0 => v1.2.3") {
		t.Fatalf("get v1.2.3: %+v", r)
	}
	r = gocppRun("", false, "get", "github.com/acme/tiny@v1")
	if r.code != 0 || !strings.Contains(r.stderr, "upgraded github.com/acme/tiny v1.2.3 => v1.10.0") {
		t.Fatalf("get v1: %+v", r)
	}

	// A version that does not exist: nothing changes.
	before := readFile(t, filepath.Join(dir, "gocpp.mod"))
	r = gocppRun("", false, "get", "github.com/acme/tiny@v7.0.0")
	if r.code != 1 || !strings.Contains(r.stderr, "was not found") || !strings.Contains(r.stderr, "gocpp.mod was not changed") {
		t.Fatalf("get v7: %+v", r)
	}
	if readFile(t, filepath.Join(dir, "gocpp.mod")) != before {
		t.Error("gocpp.mod changed")
	}

	// mod verify.
	r = gocppRun("", false, "mod", "verify")
	if r.code != 0 || r.stdout != "all modules verified\n" {
		t.Fatalf("mod verify: %+v", r)
	}
	tdir := filepath.Join(cache, "github.com", "acme", "tiny@v1.10.0")
	os.Chmod(tdir, 0o755)
	os.Chmod(filepath.Join(tdir, "tiny.h"), 0o644)
	os.WriteFile(filepath.Join(tdir, "tiny.h"), []byte("#error evil\n"), 0o644)
	r = gocppRun("", false, "mod", "verify")
	if r.code != 1 || !strings.Contains(r.stdout, "github.com/acme/tiny v1.10.0: dir has been modified") {
		t.Fatalf("mod verify after a change: %+v", r)
	}
	// A vendor folder is never made from a cache that does not match its hash.
	r = gocppRun("", false, "mod", "vendor")
	if r.code != 1 || !strings.Contains(r.stderr, "the copy does not have the hash of the cached module") {
		t.Fatalf("mod vendor from a changed cache: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "vendor")); err == nil {
		t.Error("vendor/ was made from a changed cache")
	}
	removeReadOnly(cache) // download it again

	// mod vendor, then no network and no cache: the build still finds the module.
	r = gocppRun("", false, "mod", "vendor")
	if r.code != 0 {
		t.Fatalf("mod vendor: %+v", r)
	}
	if got := readFile(t, filepath.Join(dir, "vendor", "modules.txt")); !strings.Contains(got, "# github.com/acme/tiny v1.10.0\n## explicit\ngithub.com/acme/tiny\n") {
		t.Errorf("modules.txt = %q", got)
	}
	removeReadOnly(cache)
	t.Setenv("GOCPP_OFFLINE", "1")
	t.Setenv("GOCPP_MODURL", "github.com=https://127.0.0.1:1")
	checkLoads(t, gocppRun("", false, "check", "."))
	// mod verify checks the vendor folder.
	if r = gocppRun("", false, "mod", "verify"); r.code != 0 || r.stdout != "all modules verified\n" {
		t.Fatalf("mod verify vendor: %+v", r)
	}

	// Without vendor/, offline and with an empty cache: a clear error.
	os.RemoveAll(filepath.Join(dir, "vendor"))
	r = gocppRun("", false, "check", ".")
	if r.code == 0 || !strings.Contains(r.stderr, "is not in the module cache and downloads are off (GOCPP_OFFLINE=1)") {
		t.Fatalf("check offline: %+v", r)
	}

	// get @none removes the line.
	t.Setenv("GOCPP_OFFLINE", "")
	t.Setenv("GOCPP_MODURL", "github.com="+h.srv.URL)
	r = gocppRun("", false, "get", "github.com/acme/tiny@none")
	if r.code != 0 || strings.Contains(readFile(t, filepath.Join(dir, "gocpp.mod")), "require") {
		t.Fatalf("get @none: %+v\n%s", r, readFile(t, filepath.Join(dir, "gocpp.mod")))
	}
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The build downloads a missing module by itself, trusts the first download
// and records it; -mod=readonly stops that, and a changed module is a
// security error.
func TestBuildDownloadsAndChecksSums(t *testing.T) {
	h := startModHost(t)
	h.tags["v1.2.3"] = tinyArchive(42)
	cache := h.env(t)
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n\nrequire github.com/acme/tiny v1.2.3\n", "main.gp": modApp})

	// -mod=readonly: no gocpp.sum line, no download.
	t.Setenv("GOCPP_MODFLAGS", "-mod=readonly")
	r := gocppRun("", false, "check", ".")
	if r.code == 0 || !strings.Contains(r.stderr, "missing entry for github.com/acme/tiny v1.2.3") || strings.Contains(r.stderr, "downloading") {
		t.Fatalf("readonly: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "gocpp.sum")); err == nil {
		t.Error("gocpp.sum was written in readonly mode")
	}
	if _, err := os.Stat(cache); err == nil {
		t.Error("the cache was made in readonly mode")
	}

	// The default: download, record, build.
	t.Setenv("GOCPP_MODFLAGS", "")
	var r2 result
	msg := captureStderr(t, func() { r2 = gocppRun("", false, "check", ".") })
	checkLoads(t, r2)
	if !strings.Contains(msg, "gocpp: downloading github.com/acme/tiny v1.2.3") {
		t.Fatalf("check printed %q on stderr", msg)
	}
	good := readFile(t, filepath.Join(dir, "gocpp.sum"))
	if !strings.HasPrefix(good, "github.com/acme/tiny v1.2.3 h1:") {
		t.Fatalf("gocpp.sum = %q", good)
	}
	// Again: from the cache, no message.
	msg = captureStderr(t, func() { r2 = gocppRun("", false, "check", ".") })
	checkLoads(t, r2)
	if strings.Contains(msg, "downloading") {
		t.Fatalf("second check printed %q", msg)
	}
	// readonly now works, since gocpp.sum has the line.
	t.Setenv("GOCPP_MODFLAGS", "-mod=readonly")
	checkLoads(t, gocppRun("", false, "check", "."))

	// The host serves other files for the tag, and the cache is empty: SECURITY ERROR.
	h.tags["v1.2.3"] = tinyArchive(99)
	removeReadOnly(cache)
	r = gocppRun("", false, "check", ".")
	if r.code == 0 || !strings.Contains(r.stderr, "SECURITY ERROR") || !strings.Contains(r.stderr, "checksum mismatch") {
		t.Fatalf("mismatch: %+v", r)
	}
	if readFile(t, filepath.Join(dir, "gocpp.sum")) != good {
		t.Error("gocpp.sum changed after a mismatch")
	}
	if _, err := os.Stat(filepath.Join(cache, "github.com", "acme", "tiny@v1.2.3")); err == nil {
		t.Error("the module that did not match is in the cache")
	}
}

func TestModCommandUsage(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		out  string
	}{
		{[]string{"mod"}, 2, "usage: gocpp mod <command>"},
		{[]string{"mod", "-h"}, 0, "download [module ...]"},
		{[]string{"mod", "bogus"}, 2, "unknown command"},
		{[]string{"mod", "vendor", "-h"}, 0, "usage: gocpp mod vendor"},
		{[]string{"mod", "download", "-h"}, 0, "usage: gocpp mod download"},
		{[]string{"get", "-h"}, 0, "usage: gocpp get"},
		{[]string{"get", "-nosuchflag"}, 2, "flag provided but not defined"},
	} {
		r := gocppRun("", false, tc.args...)
		if r.code != tc.code || !strings.Contains(r.stdout+r.stderr, tc.out) {
			t.Errorf("gocpp %v = %+v, want exit %d and %q", tc.args, r, tc.code, tc.out)
		}
	}
	// No gocpp.mod.
	inDir(t, map[string]string{"x.gp": "package main\n"})
	r := gocppRun("", false, "mod", "download")
	if r.code != 1 || !strings.Contains(r.stderr, "gocpp.mod file not found") {
		t.Errorf("no gocpp.mod: %+v", r)
	}
	// A broken gocpp.mod.
	inDir(t, map[string]string{"gocpp.mod": "module\n"})
	if r := gocppRun("", false, "get", "github.com/acme/tiny"); r.code != 1 || !strings.Contains(r.stderr, "usage: module") {
		t.Errorf("broken gocpp.mod: %+v", r)
	}
	if r := gocppRun("", false, "get", "@v1"); r.code != 1 && r.code != 2 {
		t.Errorf("get @v1: %+v", r)
	}
}
