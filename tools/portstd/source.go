package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// A Pin names the Go source that the tool ports from (the PINS file).
type Pin struct {
	Version string // go1.25.14
	URL     string // where the tarball is
	SHA256  string // its SHA-256, lower-case hex
}

var (
	versionRx = regexp.MustCompile(`^go1\.[0-9]+(\.[0-9]+)?$`)
	sha256Rx  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// parsePin reads the text of a PINS file.
func parsePin(text string) (Pin, error) {
	var p Pin
	for n, line := range configLines(text) {
		f := strings.Fields(line)
		if len(f) != 2 {
			return p, fmt.Errorf("line %d: want: <version|url|sha256> <value>", n)
		}
		switch f[0] {
		case "version":
			p.Version = f[1]
		case "url":
			p.URL = f[1]
		case "sha256":
			p.SHA256 = strings.ToLower(f[1])
		default:
			return p, fmt.Errorf("line %d: unknown key %q", n, f[0])
		}
	}
	switch {
	case !versionRx.MatchString(p.Version):
		return p, fmt.Errorf("version %q is not like go1.25.14", p.Version)
	case checkURL(p.URL) != nil:
		return p, fmt.Errorf("url: %v", checkURL(p.URL))
	case !sha256Rx.MatchString(p.SHA256):
		return p, fmt.Errorf("sha256 %q is not 64 hex digits", p.SHA256)
	}
	return p, nil
}

// checkURL accepts an https URL, and an http URL only for this machine (the
// tests serve a fixture on a loopback port): the tarball's hash is pinned, but
// the listing that -pin trusts is not, and nothing here should go in clear text.
func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("%q is not a URL", raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		switch u.Hostname() {
		case "localhost", "127.0.0.1", "::1":
			return nil
		}
	}
	return fmt.Errorf("%q must be an https URL (http is accepted for localhost only)", raw)
}

// Limits against a hostile or broken download: the Go source tarball is about 30 MB
// and 150 MB extracted. Variables so that the tests can lower them.
var (
	maxTarball int64 = 256 << 20
	maxExtract int64 = 1 << 30
)

// staleAge is how old an extract-* folder or download-* file of the cache must be
// before the next run removes it: it was left by a process that was killed
// (a younger one may belong to a process that is still working).
const staleAge = time.Hour

// minor returns the minor version of the language: 25 for go1.25.14.
func (p Pin) minor() int {
	var maj, min int
	fmt.Sscanf(p.Version, "go%d.%d", &maj, &min)
	return min
}

// text returns the PINS file for p, with the explanation at its top.
func (p Pin) text(header string) string {
	return fmt.Sprintf("%sversion %s\nurl     %s\nsha256  %s\n", header, p.Version, p.URL, p.SHA256)
}

// stem is the name of the tarball and of its extracted tree in the cache:
// the version and the start of the hash, so a changed pin never reuses an
// old tree.
func (p Pin) stem() string {
	return p.Version + "-" + p.SHA256[:12]
}

// NoCacheError is the error when the cache has no tarball and downloading is
// not allowed (offline).
type NoCacheError struct {
	Pin   Pin
	Cache string
}

func (e *NoCacheError) Error() string {
	return fmt.Sprintf("the cache %s has no %s.src.tar.gz and downloading is off (offline): run once online, "+
		"or put the official %s there (SHA-256 %s)", e.Cache, e.Pin.stem(), e.Pin.URL, e.Pin.SHA256)
}

// Source is the pinned Go tree in the cache.
type Source struct {
	Pin    Pin
	Cache  string // the folder: $XDG_CACHE_HOME/gocpp/portstd
	Fetch  bool   // may download the tarball
	Client *http.Client
	Log    io.Writer // progress messages, or nil
}

func (s *Source) logf(format string, args ...any) {
	if s.Log != nil {
		fmt.Fprintf(s.Log, format+"\n", args...)
	}
}

func (s *Source) tarball() string { return filepath.Join(s.Cache, s.Pin.stem()+".src.tar.gz") }
func (s *Source) tree() string    { return filepath.Join(s.Cache, s.Pin.stem()) }

// doneMarker is the file that says the tree was extracted completely.
const doneMarker = ".portstd-complete"

// treeComplete reports whether the extracted tree is there: its marker holds
// the SHA-256 of the pinned tarball it was extracted from.
func (s *Source) treeComplete() bool {
	b, err := os.ReadFile(filepath.Join(s.tree(), doneMarker))
	return err == nil && strings.TrimSpace(string(b)) == s.Pin.SHA256
}

// Available reports whether Root can work without downloading.
func (s *Source) Available() bool {
	if s.treeComplete() {
		return true
	}
	_, err := os.Stat(s.tarball())
	return err == nil
}

// Root returns the folder of the extracted tree (src/, test/, VERSION,
// LICENSE), extracting the tarball first when needed, and downloading it when
// Fetch is set and the cache lacks it. The tarball's SHA-256 is checked
// before anything is extracted (the bytes that were hashed are the bytes that
// are extracted). An extracted tree is used as it is: its marker has the pin's
// hash, and the tarball is not read again.
func (s *Source) Root() (string, error) {
	tree := s.tree()
	if s.treeComplete() {
		return tree, nil
	}
	if err := os.MkdirAll(s.Cache, 0o755); err != nil {
		return "", err
	}
	s.cleanStale()
	tb := s.tarball()
	if _, err := os.Stat(tb); err != nil {
		if !s.Fetch {
			return "", &NoCacheError{Pin: s.Pin, Cache: s.Cache}
		}
		if err := s.download(tb); err != nil {
			return "", err
		}
	}
	data, err := readTarball(tb)
	if err != nil {
		return "", err
	}
	if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != s.Pin.SHA256 {
		return "", fmt.Errorf("%s has SHA-256 %s, but PINS says %s: delete the file, or fix PINS (a maintainer re-pins with -pin)", tb, hex.EncodeToString(sum[:]), s.Pin.SHA256)
	}
	s.logf("portstd: extracting %s", filepath.Base(tb))
	tmp, err := os.MkdirTemp(s.Cache, "extract-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := extract(bytes.NewReader(data), tmp, s.Pin.Version); err != nil {
		return "", fmt.Errorf("%s: %v", tb, err)
	}
	if err := os.WriteFile(filepath.Join(tmp, doneMarker), []byte(s.Pin.SHA256+"\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, tree); err != nil {
		// Another process may have been first.
		if s.treeComplete() {
			return tree, nil
		}
		return "", err
	}
	return tree, nil
}

// cleanStale removes what a killed process left in the cache: extract-* folders
// and download-* files older than staleAge.
func (s *Source) cleanStale() {
	entries, err := os.ReadDir(s.Cache)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "extract-") && !strings.HasPrefix(e.Name(), "download-") {
			continue
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) > staleAge {
			os.RemoveAll(filepath.Join(s.Cache, e.Name()))
		}
	}
}

// readTarball reads the whole tarball (at most maxTarball bytes).
func readTarball(name string) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxTarball+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxTarball {
		return nil, fmt.Errorf("%s is larger than %d MB: not the Go source tarball", name, maxTarball>>20)
	}
	return data, nil
}

// download fetches the pinned URL into dst, and fails (leaving nothing) when
// the SHA-256 of what came is not the pinned one.
func (s *Source) download(dst string) error {
	if err := checkURL(s.Pin.URL); err != nil {
		return fmt.Errorf("download: %v", err)
	}
	s.logf("portstd: downloading %s", s.Pin.URL)
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	resp, err := client.Get(s.Pin.URL)
	if err != nil {
		return fmt.Errorf("download %s: %v", s.Pin.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", s.Pin.URL, resp.Status)
	}
	f, err := os.CreateTemp(filepath.Dir(dst), "download-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxTarball+1))
	if err == nil && n > maxTarball {
		err = fmt.Errorf("more than %d MB", maxTarball>>20)
	}
	if err != nil {
		f.Close()
		return fmt.Errorf("download %s: %v", s.Pin.URL, err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != s.Pin.SHA256 {
		return fmt.Errorf("download %s: SHA-256 is %s, but PINS says %s (nothing was kept)", s.Pin.URL, got, s.Pin.SHA256)
	}
	return os.Rename(f.Name(), dst)
}

// extracted says which entries of the tarball (relative to its go/ folder)
// are written: the library source without the compiler and the tools
// (src/cmd), Go's own test programs (test/), VERSION and LICENSE.
func extracted(rel string) bool {
	switch {
	case rel == "VERSION", rel == "LICENSE":
		return true
	case rel == "src", rel == "test":
		return true
	case strings.HasPrefix(rel, "src/cmd/"), rel == "src/cmd":
		return false
	}
	return strings.HasPrefix(rel, "src/") || strings.HasPrefix(rel, "test/")
}

// extract writes the wanted files of the tarball (the stream r) into dir. The
// tarball is not trusted: a name that is not under go/ (also one that cleans to
// such a name, like go/src/../../x) is skipped, a name under go/ that is not local
// is refused, links and devices are skipped, and at most maxExtract bytes are written.
func extract(r io.Reader, dir, version string) error {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	var sawVersion bool
	budget := maxExtract
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := path.Clean(h.Name)
		rel, ok := strings.CutPrefix(name, "go/")
		if !ok || !extracted(rel) {
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			return fmt.Errorf("unsafe name %q in the archive", h.Name)
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if h.Mode&0o111 != 0 {
				mode = 0o755
			}
			w, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			n, err := io.Copy(w, io.LimitReader(tr, budget+1))
			if err == nil && n > budget {
				err = fmt.Errorf("%s: more than %d MB extracted", h.Name, maxExtract>>20)
			}
			if err != nil {
				w.Close()
				return err
			}
			budget -= n
			if err := w.Close(); err != nil {
				return err
			}
			if rel == "VERSION" {
				sawVersion = true
			}
		default:
			// links and devices: nothing in the wanted folders needs them
		}
	}
	if !sawVersion {
		return errors.New("the archive has no go/VERSION: not a Go source tarball")
	}
	b, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		return err
	}
	if first, _, _ := strings.Cut(string(b), "\n"); strings.TrimSpace(first) != version {
		return fmt.Errorf("go/VERSION says %q, but PINS says %s", strings.TrimSpace(first), version)
	}
	return nil
}

// A release is one entry of the official download listing.
type release struct {
	Version string `json:"version"`
	Files   []struct {
		Filename string `json:"filename"`
		Kind     string `json:"kind"`
		SHA256   string `json:"sha256"`
	} `json:"files"`
}

// Repin returns the pin for version, from the official listing at
// listURL (https://go.dev/dl/?mode=json&include=all). It downloads the
// tarball from dlBase and checks that it has the listed hash, so that a
// wrong pin is caught at once.
func Repin(client *http.Client, listURL, dlBase, version, cache string, log io.Writer) (Pin, error) {
	if !versionRx.MatchString(version) {
		return Pin{}, fmt.Errorf("version %q is not like go1.25.15", version)
	}
	if err := checkURL(listURL); err != nil {
		return Pin{}, fmt.Errorf("listing: %v", err)
	}
	if err := checkURL(dlBase); err != nil {
		return Pin{}, fmt.Errorf("dl: %v", err)
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	resp, err := client.Get(listURL)
	if err != nil {
		return Pin{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Pin{}, fmt.Errorf("%s: %s", listURL, resp.Status)
	}
	var rels []release
	if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
		return Pin{}, fmt.Errorf("%s: %v", listURL, err)
	}
	want := version + ".src.tar.gz"
	for _, r := range rels {
		if r.Version != version {
			continue
		}
		for _, f := range r.Files {
			if f.Filename == want && f.Kind == "source" && sha256Rx.MatchString(f.SHA256) {
				p := Pin{Version: version, URL: strings.TrimSuffix(dlBase, "/") + "/" + want, SHA256: f.SHA256}
				s := &Source{Pin: p, Cache: cache, Fetch: true, Client: client, Log: log}
				if _, err := s.Root(); err != nil {
					return Pin{}, err
				}
				return p, nil
			}
		}
	}
	return Pin{}, fmt.Errorf("%s: the listing has no source archive %s", listURL, want)
}
