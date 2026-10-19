package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// A Pin names the Go source that the tool reads Go's tests from (the PINS
// file; the same format as tools/portstd/PINS).
type Pin struct {
	Version string // go1.25.14
	URL     string // where the tarball is
	SHA256  string // its SHA-256, lower-case hex
}

var (
	versionRx = regexp.MustCompile(`^go1\.[0-9]+(\.[0-9]+)?$`)
	sha256Rx  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// configLines returns the lines of a configuration file without comments
// and blank lines, keyed by their 1-based line numbers (in order).
type cfgLine struct {
	N    int
	Text string
}

func configLines(text string) []cfgLine {
	var out []cfgLine
	for i, l := range strings.Split(text, "\n") {
		if j := strings.Index(l, "#"); j >= 0 {
			l = l[:j]
		}
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, cfgLine{i + 1, l})
		}
	}
	return out
}

// parsePin reads the text of a PINS file.
func parsePin(text string) (Pin, error) {
	var p Pin
	for _, cl := range configLines(text) {
		f := strings.Fields(cl.Text)
		if len(f) != 2 {
			return p, fmt.Errorf("line %d: want: <version|url|sha256> <value>", cl.N)
		}
		switch f[0] {
		case "version":
			p.Version = f[1]
		case "url":
			p.URL = f[1]
		case "sha256":
			p.SHA256 = strings.ToLower(f[1])
		default:
			return p, fmt.Errorf("line %d: unknown key %q", cl.N, f[0])
		}
	}
	switch {
	case !versionRx.MatchString(p.Version):
		return p, fmt.Errorf("version %q is not like go1.25.14", p.Version)
	case !strings.HasPrefix(p.URL, "https://") && !strings.HasPrefix(p.URL, "http://"):
		return p, fmt.Errorf("url %q is not an http(s) URL", p.URL)
	case !sha256Rx.MatchString(p.SHA256):
		return p, fmt.Errorf("sha256 %q is not 64 hex digits", p.SHA256)
	}
	return p, nil
}

// minor returns the minor version of the language: 25 for go1.25.14.
func (p Pin) minor() int {
	var maj, min int
	fmt.Sscanf(p.Version, "go%d.%d", &maj, &min)
	return min
}

// stem is the name of the tarball and of its extracted tree in the cache:
// the version and the start of the hash. tools/portstd uses the same
// stem, so the two tools share the tarball.
func (p Pin) stem() string {
	return p.Version + "-" + p.SHA256[:12]
}

// NoCacheError is the error when the cache has no tarball and downloading
// is not allowed (offline).
type NoCacheError struct {
	Pin   Pin
	Cache string
}

func (e *NoCacheError) Error() string {
	return fmt.Sprintf("the cache %s has no %s.src.tar.gz and downloading is off (offline): run once online, "+
		"or put the official %s there (SHA-256 %s)", e.Cache, e.Pin.stem(), e.Pin.URL, e.Pin.SHA256)
}

// Source is Go's test/ folder from the pinned tarball, in the cache.
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

// tree is the folder this tool extracts test/ into. It is not the folder
// of tools/portstd (which holds src/ too): that one is used when it is
// complete and has a test/ folder (portstdTree).
func (s *Source) tree() string { return filepath.Join(s.Cache, s.Pin.stem()+"-gotest") }

func (s *Source) portstdTree() string { return filepath.Join(s.Cache, s.Pin.stem()) }

const (
	doneMarker        = ".gotest-complete"
	portstdDoneMarker = ".portstd-complete"
)

// Available reports whether Root can work without downloading.
func (s *Source) Available() bool {
	if _, err := s.existing(); err == nil {
		return true
	}
	_, err := os.Stat(s.tarball())
	return err == nil
}

// existing returns a complete extracted tree that has test/ and VERSION.
func (s *Source) existing() (string, error) {
	for _, c := range []struct{ dir, marker string }{{s.tree(), doneMarker}, {s.portstdTree(), portstdDoneMarker}} {
		if _, err := os.Stat(filepath.Join(c.dir, c.marker)); err != nil {
			continue
		}
		if st, err := os.Stat(filepath.Join(c.dir, "test")); err == nil && st.IsDir() {
			return c.dir, nil
		}
	}
	return "", os.ErrNotExist
}

// Root returns the folder that holds test/ and VERSION, extracting the
// tarball first when needed and downloading it when Fetch is set and the
// cache lacks it. The SHA-256 of the tarball is checked before anything is
// extracted, also for a tarball that is already in the cache.
func (s *Source) Root() (string, error) {
	if dir, err := s.existing(); err == nil {
		return dir, nil
	}
	if err := os.MkdirAll(s.Cache, 0o755); err != nil {
		return "", err
	}
	tb := s.tarball()
	if _, err := os.Stat(tb); err != nil {
		if !s.Fetch {
			return "", &NoCacheError{Pin: s.Pin, Cache: s.Cache}
		}
		if err := s.download(tb); err != nil {
			return "", err
		}
	} else if got, err := fileSHA256(tb); err != nil {
		return "", err
	} else if got != s.Pin.SHA256 {
		return "", fmt.Errorf("%s has SHA-256 %s, but PINS says %s: delete the file", tb, got, s.Pin.SHA256)
	}
	tmp, err := os.MkdirTemp(s.Cache, "extract-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	s.logf("extracting test/ from %s", filepath.Base(tb))
	if err := extract(tb, tmp, s.Pin.Version); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, doneMarker), nil, 0o644); err != nil {
		return "", err
	}
	if err := os.RemoveAll(s.tree()); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, s.tree()); err != nil {
		return "", err
	}
	return s.tree(), nil
}

func (s *Source) download(dst string) error {
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	s.logf("downloading %s", s.Pin.URL)
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
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
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

// fileSHA256 returns the hex SHA-256 of a file.
func fileSHA256(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extracted says which entries of the tarball (relative to its go/ folder)
// are written: Go's own test programs (test/), VERSION and LICENSE. Nothing
// of it is ever copied into the repository (the license of the test files
// stays with Go's source; the repository keeps the selection list only).
func extracted(rel string) bool {
	switch {
	case rel == "VERSION", rel == "LICENSE", rel == "test":
		return true
	}
	return strings.HasPrefix(rel, "test/")
}

// extract writes the wanted files of the tarball into dir. The tarball is
// not trusted: a name that leaves dir is refused, and anything but a file
// or a folder is skipped.
func extract(tarball, dir, version string) error {
	f, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	var sawVersion bool
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
			w, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return err
			}
			if err := w.Close(); err != nil {
				return err
			}
			if rel == "VERSION" {
				sawVersion = true
			}
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
