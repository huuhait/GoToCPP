package modfetch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// A tfile is an entry of a test archive.
type tfile struct {
	Name     string
	Body     string
	Mode     int64
	Type     byte // tar type flag; 0 means a regular file
	Linkname string
}

func dirEntry(name string) tfile { return tfile{Name: name, Type: tar.TypeDir, Mode: 0o755} }

var testTime = time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)

const testCommit = "0123456789abcdef0123456789abcdef01234567"

// tarGz makes a tar.gz archive; commit, if not empty, is the comment of a pax
// global header, as git archive writes it.
func tarGz(commit string, files ...tfile) []byte { return gzipBytes(tarRaw(commit, files...)) }

// tarRaw is the tar file of tarGz, not compressed.
func tarRaw(commit string, files ...tfile) []byte {
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	if commit != "" {
		tw.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": commit}})
	}
	for _, f := range files {
		typ := f.Type
		if typ == 0 {
			typ = tar.TypeReg
		}
		mode := f.Mode
		if mode == 0 {
			mode = 0o644
		}
		h := &tar.Header{Name: f.Name, Typeflag: typ, Mode: mode, ModTime: testTime, Linkname: f.Linkname, Format: tar.FormatPAX}
		if typ == tar.TypeReg {
			h.Size = int64(len(f.Body))
		}
		if err := tw.WriteHeader(h); err != nil {
			panic(err)
		}
		if typ == tar.TypeReg {
			tw.Write([]byte(f.Body))
		}
	}
	tw.Close()
	return raw.Bytes()
}

func gzipBytes(b []byte) []byte {
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	gz.Write(b)
	gz.Close()
	return out.Bytes()
}

func zipOf(files ...tfile) []byte {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range files {
		h := &zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: testTime}
		mode := os.FileMode(f.Mode)
		if f.Mode == 0 {
			mode = 0o644
		}
		switch f.Type {
		case tar.TypeDir:
			h.SetMode(os.ModeDir | mode)
		case tar.TypeSymlink:
			h.SetMode(os.ModeSymlink | mode)
		default:
			h.SetMode(mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			panic(err)
		}
		if f.Type == tar.TypeSymlink {
			w.Write([]byte(f.Linkname))
		} else {
			w.Write([]byte(f.Body))
		}
	}
	zw.Close()
	return out.Bytes()
}

// A library is the files of a small header-only C++ library, in the folder
// repo-ref/ as a git host's archive has it.
func library(top string, extra ...tfile) []tfile {
	fs := []tfile{
		dirEntry(top + "/"),
		{Name: top + "/README.md", Body: "a tiny library\n"},
		dirEntry(top + "/include/"),
		{Name: top + "/include/tiny.h", Body: "#pragma once\ninline int tiny() { return 42; }\n"},
		{Name: top + "/build.sh", Body: "#!/bin/sh\necho never run\n", Mode: 0o755},
	}
	return append(fs, extra...)
}

// A fakeHost is an HTTPS server with the URLs of a git host (GitHub's layout).
type fakeHost struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	handlers map[string]http.HandlerFunc
	hits     map[string]int
	auth     map[string][]string // path -> Authorization headers seen
}

func newFakeHost(t *testing.T) *fakeHost {
	t.Helper()
	h := &fakeHost{t: t, handlers: map[string]http.HandlerFunc{}, hits: map[string]int{}, auth: map[string][]string{}}
	h.srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.hits[r.URL.Path]++
		h.auth[r.URL.Path] = append(h.auth[r.URL.Path], r.Header.Get("Authorization"))
		fn := h.handlers[r.URL.Path]
		h.mu.Unlock()
		if fn == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r)
	}))
	h.srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	h.srv.StartTLS()
	t.Cleanup(h.srv.Close)
	return h
}

func (h *fakeHost) handle(path string, fn http.HandlerFunc) {
	h.mu.Lock()
	h.handlers[path] = fn
	h.mu.Unlock()
}

func (h *fakeHost) serve(path string, body []byte) {
	h.handle(path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(body)
	})
}

func (h *fakeHost) count(path string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hits[path]
}

func (h *fakeHost) total() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, c := range h.hits {
		n += c
	}
	return n
}

// tag serves the archive of a tag of github.com/o/r.
func (h *fakeHost) tag(repo, tag string, body []byte) string {
	p := "/" + repo + "/archive/refs/tags/" + tag + ".tar.gz"
	h.serve(p, body)
	return p
}

func (h *fakeHost) commit(repo, commit string, body []byte) string {
	p := "/" + repo + "/archive/" + commit + ".tar.gz"
	h.serve(p, body)
	return p
}

// refs serves git's info/refs for the repo.
func (h *fakeHost) refs(repo string, head string, headBranch string, tags map[string]string, branches map[string]string) {
	var b bytes.Buffer
	pkt := func(s string) { fmt.Fprintf(&b, "%04x%s", len(s)+4, s) }
	pkt("# service=git-upload-pack\n")
	b.WriteString("0000")
	caps := "multi_ack thin-pack side-band symref=HEAD:refs/heads/" + headBranch + " agent=git/test"
	first := true
	line := func(sha, name string) {
		if first {
			pkt(sha + " " + name + "\x00" + caps + "\n")
			first = false
		} else {
			pkt(sha + " " + name + "\n")
		}
	}
	if head != "" {
		line(head, "HEAD")
	}
	var names []string
	for n := range branches {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		line(branches[n], "refs/heads/"+n)
	}
	names = names[:0]
	for n := range tags {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		line(tags[n], "refs/tags/"+n)
		if strings.HasSuffix(n, "-annotated") {
			line(strings.Repeat("a", 40), "refs/tags/"+n+"^{}")
		}
	}
	b.WriteString("0000")
	body := b.Bytes()
	h.handle("/"+repo+".git/info/refs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("service") != "git-upload-pack" {
			http.Error(w, "want service=git-upload-pack", 400)
			return
		}
		w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
		w.Write(body)
	})
}

// config returns a Config with a cache of its own that sends the github.com
// URLs to this host.
func (h *fakeHost) config() *Config {
	return &Config{
		CacheDir: filepath.Join(h.t.TempDir(), "cache"),
		URLs:     []URLRule{{Prefix: "github.com", Base: h.srv.URL}},
		Client:   h.srv.Client(),
		Limits:   DefaultLimits,
	}
}

func (h *fakeHost) fetcher() *Fetcher {
	h.t.Helper()
	f, err := New(h.config())
	if err != nil {
		h.t.Fatal(err)
	}
	return f
}

// caFile writes the certificate of the server as a PEM file.
func (h *fakeHost) caFile() string {
	cert := h.srv.Certificate()
	p := filepath.Join(h.t.TempDir(), "ca.pem")
	if err := os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0o644); err != nil {
		h.t.Fatal(err)
	}
	return p
}

// readTree returns the files of a tree: name -> content.
func readTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(dir, p)
			out[filepath.ToSlash(rel)] = string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// goHash is the h1 hash computed in the way the doc of Hash1 says, for a
// map of files.
func goHash(prefix string, files map[string]string) string {
	var names []string
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		fmt.Fprintf(h, "%x  %s/%s\n", sha256.Sum256([]byte(files[n])), prefix, n)
	}
	return "h1:" + base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func libFiles() map[string]string {
	return map[string]string{
		"README.md":      "a tiny library\n",
		"include/tiny.h": "#pragma once\ninline int tiny() { return 42; }\n",
		"build.sh":       "#!/bin/sh\necho never run\n",
	}
}

func mustWrite(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cleanupCache lets t.TempDir remove a read-only cache.
func cleanupCache(t *testing.T, dir string) {
	t.Cleanup(func() { RemoveAll(dir) })
}

var _ = io.Discard
