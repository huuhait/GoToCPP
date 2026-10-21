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
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// A gitHost is an HTTPS server with the URLs of GitHub for several repositories
// ("acme/lib"), each with tagged archives: modules whose gocpp.mod or go.mod
// files are in the archives, as for a real library on GitHub.
type gitHost struct {
	srv   *httptest.Server
	repos map[string]map[string][]byte // "owner/repo" -> tag -> archive

	mu       sync.Mutex
	requests []string // the URL path of every request, in order
}

// requested returns the URL paths that the host was asked for, and forgets them.
func (h *gitHost) requested() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	got := h.requests
	h.requests = nil
	return got
}

func archiveOf(top string, files map[string]string) []byte {
	return archiveAt(top, files, time.Unix(1700000000, 0))
}

func archiveAt(top string, files map[string]string, mtime time.Time) []byte {
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: top + "/", Typeflag: tar.TypeDir, Mode: 0o755, ModTime: mtime})
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		body := files[name]
		tw.WriteHeader(&tar.Header{Name: top + "/" + name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body)), ModTime: mtime})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return raw.Bytes()
}

func startGitHost(t *testing.T) *gitHost {
	h := &gitHost{repos: map[string]map[string][]byte{}}
	h.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.requests = append(h.requests, r.URL.Path)
		h.mu.Unlock()
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if len(parts) >= 3 && parts[len(parts)-1] != "" {
			repo := parts[0] + "/" + parts[1]
			if strings.HasSuffix(r.URL.Path, ".tar.gz") && len(parts) >= 6 && parts[2] == "archive" {
				tag := strings.TrimSuffix(strings.Join(parts[5:], "/"), ".tar.gz")
				if b, ok := h.repos[repo][tag]; ok {
					w.Write(b)
					return
				}
			}
			if strings.HasSuffix(r.URL.Path, ".tar.gz") && len(parts) == 4 && parts[2] == "archive" {
				// A commit: /<repo>/archive/<commit>.tar.gz
				if b, ok := h.repos[repo][strings.TrimSuffix(parts[3], ".tar.gz")]; ok {
					w.Write(b)
					return
				}
			}
			if strings.HasSuffix(r.URL.Path, ".git/info/refs") {
				repo = strings.TrimSuffix(repo, ".git")
				tags, ok := h.repos[repo]
				if !ok {
					http.NotFound(w, r)
					return
				}
				var b bytes.Buffer
				pkt := func(s string) { fmt.Fprintf(&b, "%04x%s", len(s)+4, s) }
				pkt("# service=git-upload-pack\n")
				b.WriteString("0000")
				first := true
				var names []string
				for tag := range tags {
					names = append(names, tag)
				}
				sort.Strings(names)
				for _, tag := range names {
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
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(h.srv.Close)
	return h
}

func (h *gitHost) tag(repo, tag string, files map[string]string) {
	if h.repos[repo] == nil {
		h.repos[repo] = map[string][]byte{}
	}
	h.repos[repo][tag] = archiveOf(strings.ReplaceAll(repo, "/", "-")+"-"+tag, files)
}

// tagAt is tag with the time of the commit (the time of the files of its archive).
func (h *gitHost) tagAt(repo, tag string, when time.Time, files map[string]string) {
	if h.repos[repo] == nil {
		h.repos[repo] = map[string][]byte{}
	}
	h.repos[repo][tag] = archiveAt(strings.ReplaceAll(repo, "/", "-")+"-"+tag, files, when)
}

func (h *gitHost) env(t *testing.T) string {
	cert := h.srv.Certificate()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "modcache")
	t.Cleanup(func() { removeReadOnly(cache) })
	t.Setenv("GOCPP_MODCACHE", cache)
	t.Setenv("GOCPP_MODURL", "github.com="+h.srv.URL)
	t.Setenv("GOCPP_MODCACERT", ca)
	t.Setenv("GOCPP_OFFLINE", "")
	t.Setenv("GOCPP_MODFLAGS", "")
	t.Setenv("GOCPP_MODPROXY", "")
	return cache
}

// The requirements of a library are in its gocpp.mod (or go.mod) in the archive
// of the tag: version selection reads them from the git host.
func TestGetFromGitHostReadsRequirementsOfArchives(t *testing.T) {
	h := startGitHost(t)
	h.tag("acme/lib", "v1.0.0", map[string]string{"gocpp.mod": "module github.com/acme/lib\n\nrequire github.com/acme/dep v1.0.0\n", "lib.h": "int lib;\n"})
	h.tag("acme/lib", "v1.1.0", map[string]string{"gocpp.mod": "module github.com/acme/lib\n\nrequire github.com/acme/dep v1.2.0\n", "lib.h": "int lib;\n"})
	h.tag("acme/dep", "v1.0.0", map[string]string{"gocpp.mod": "module github.com/acme/dep\n", "dep.h": "int dep;\n"})
	h.tag("acme/dep", "v1.2.0", map[string]string{"go.mod": "module github.com/acme/dep\n\nrequire github.com/acme/leaf v1.0.0\n", "dep.h": "int dep;\n"})
	h.tag("acme/leaf", "v1.0.0", map[string]string{"gocpp.mod": "module github.com/acme/leaf\n"})
	h.env(t)
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n"})

	r := gocppRun("", false, "get", "github.com/acme/lib@v1.0.0")
	if r.code != 0 {
		t.Fatalf("get: %+v", r)
	}
	for _, want := range []string{"gocpp: added github.com/acme/dep v1.0.0", "gocpp: added github.com/acme/lib v1.0.0"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, r.stderr)
		}
	}
	// An upgrade of lib raises dep (found through lib's gocpp.mod) and adds leaf (found through dep's go.mod).
	r = gocppRun("", false, "get", "github.com/acme/lib@v1.1.0")
	if r.code != 0 {
		t.Fatalf("get v1.1.0: %+v", r)
	}
	for _, want := range []string{"upgraded github.com/acme/dep v1.0.0 => v1.2.0", "upgraded github.com/acme/lib v1.0.0 => v1.1.0", "added github.com/acme/leaf v1.0.0"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, r.stderr)
		}
	}
	mod := readFile(t, filepath.Join(dir, "gocpp.mod"))
	for _, want := range []string{"require github.com/acme/lib v1.1.0\n", "github.com/acme/dep v1.2.0 // indirect", "github.com/acme/leaf v1.0.0 // indirect"} {
		if !strings.Contains(mod, want) {
			t.Errorf("gocpp.mod lacks %q:\n%s", want, mod)
		}
	}
	// The graph, as a gocpp.mod and a go.mod say it.
	r = gocppRun("", false, "mod", "graph")
	wantGraph := "example.com/app github.com/acme/dep@v1.2.0\nexample.com/app github.com/acme/lib@v1.1.0\nexample.com/app github.com/acme/leaf@v1.0.0\n"
	if r.code != 0 || !strings.HasPrefix(r.stdout, "example.com/app ") || !strings.Contains(r.stdout, "github.com/acme/lib@v1.1.0 github.com/acme/dep@v1.2.0\n") || !strings.Contains(r.stdout, "github.com/acme/dep@v1.2.0 github.com/acme/leaf@v1.0.0\n") {
		t.Errorf("graph = %q (like %q)", r.stdout, wantGraph)
	}
	// And a downgrade of dep takes lib down with it.
	r = gocppRun("", false, "get", "github.com/acme/dep@v1.0.0")
	// (leaf stays: it was in the build list, and nothing that is lowered needs more.)
	if r.code != 0 || !strings.Contains(r.stderr, "downgraded github.com/acme/lib v1.1.0 => v1.0.0") || strings.Contains(r.stderr, "leaf") {
		t.Fatalf("downgrade dep: %+v", r)
	}
}

// A repository with a tag v2.0.0 and no go.mod (most C libraries): its version is
// v2.0.0+incompatible, as in Go, and it can be fetched.
func TestGetIncompatibleVersionFromGitHost(t *testing.T) {
	h := startGitHost(t)
	h.tag("acme/legacy", "v1.9.0", map[string]string{"legacy.h": "int old;\n"})
	h.tag("acme/legacy", "v2.0.0", map[string]string{"legacy.h": "int newer;\n"})
	h.env(t)
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n"})
	r := gocppRun("", false, "get", "github.com/acme/legacy@v2.0.0")
	if r.code != 0 {
		t.Fatalf("get v2.0.0: %+v", r)
	}
	mod := readFile(t, filepath.Join(dir, "gocpp.mod"))
	if !strings.Contains(mod, "github.com/acme/legacy v2.0.0") {
		t.Errorf("gocpp.mod = %q", mod)
	}
	// A library whose releases have no module file: "latest" is its newest tag, v2.0.0, as Go does.
	latestDir := inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n"})
	r = gocppRun("", false, "get", "github.com/acme/legacy")
	if r.code != 0 || !strings.Contains(r.stderr, "added github.com/acme/legacy v2.0.0+incompatible") {
		t.Errorf("latest: %+v", r)
	}
	// Finding that v1.9.0 has no module file reads it for that answer only: its hashes are not kept.
	sum := readFile(t, filepath.Join(latestDir, "gocpp.sum"))
	if strings.Contains(sum, "github.com/acme/legacy v1.9.0") {
		t.Errorf("gocpp.sum records v1.9.0, which get did not ask for:\n%s", sum)
	}
	// One whose v1 has a module file stays on v1; the version is named to cross.
	h.tag("acme/modern", "v1.0.0", map[string]string{"gocpp.mod": "module github.com/acme/modern\n"})
	h.tag("acme/modern", "v3.0.0", map[string]string{"modern.h": "int m;\n"})
	inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n"})
	r = gocppRun("", false, "get", "github.com/acme/modern")
	if r.code != 0 || !strings.Contains(r.stderr, "added github.com/acme/modern v1.0.0") {
		t.Errorf("modern latest: %+v", r)
	}
	r = gocppRun("", false, "get", "github.com/acme/modern@v3.0.0")
	if r.code != 0 || !strings.Contains(r.stderr, "upgraded github.com/acme/modern v1.0.0 => v3.0.0+incompatible") && !strings.Contains(r.stderr, "v3.0.0") {
		t.Errorf("modern v3: %+v", r)
	}
	// ...and the version with +incompatible is the same version as v2.0.0.
	inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n"})
	r = gocppRun("", false, "get", "github.com/acme/legacy@v2.0.0+incompatible")
	if r.code != 0 {
		t.Errorf("v2.0.0+incompatible: %+v", r)
	}
}

// A major version suffix: github.com/acme/lib/v2 needs v2 tags, and a v1 version of it is an error.
func TestGetMajorVersionSuffixConflict(t *testing.T) {
	h := startGitHost(t)
	h.tag("acme/mod", "v1.0.0", map[string]string{"gocpp.mod": "module github.com/acme/mod\n"})
	h.tag("acme/mod", "v2.0.0", map[string]string{"gocpp.mod": "module github.com/acme/mod/v2\n"})
	h.env(t)
	inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n"})
	r := gocppRun("", false, "get", "github.com/acme/mod/v2@v1.0.0")
	if r.code != 1 || !strings.Contains(r.stderr, "does not match the /v2") {
		t.Errorf("v1 of /v2: %+v", r)
	}
	r = gocppRun("", false, "get", "github.com/acme/mod/v2@v2.0.0", "github.com/acme/mod@v1.0.0")
	if r.code != 0 {
		t.Fatalf("both majors: %+v", r)
	}
	mod := readFile(t, "gocpp.mod")
	if !strings.Contains(mod, "github.com/acme/mod v1.0.0") || !strings.Contains(mod, "github.com/acme/mod/v2 v2.0.0") {
		t.Errorf("gocpp.mod = %q", mod)
	}
}

// "gocpp get lib" from a pseudo-version never moves to a release whose commit is
// older (Go's rule); a newer release is an upgrade.
func TestGetFromAPseudoVersionNeverGoesBackToAnOlderTag(t *testing.T) {
	const pseudo = "v0.0.0-20210101000000-aaaaaaaaaaaa"
	for _, c := range []struct {
		release time.Time
		want    string // "" for no change
	}{
		{time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC), ""},
		{time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC), "upgraded github.com/acme/lib " + pseudo + " => v1.0.0"},
	} {
		h := startGitHost(t)
		files := map[string]string{"gocpp.mod": "module github.com/acme/lib\n", "lib.h": "int lib;\n"}
		h.tagAt("acme/lib", "v1.0.0", c.release, files)
		h.tagAt("acme/lib", "aaaaaaaaaaaa", time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), files)
		h.env(t)
		dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n\nrequire github.com/acme/lib " + pseudo + "\n"})
		r := gocppRun("", false, "get", "github.com/acme/lib")
		if r.code != 0 {
			t.Fatalf("get (release %v): %+v", c.release, r)
		}
		mod := readFile(t, filepath.Join(dir, "gocpp.mod"))
		if c.want == "" {
			if strings.Contains(r.stderr, "upgraded") || !strings.Contains(mod, pseudo) {
				t.Errorf("release %v: moved back: %s\n%s", c.release, r.stderr, mod)
			}
		} else if !strings.Contains(r.stderr, c.want) {
			t.Errorf("release %v: stderr lacks %q:\n%s", c.release, c.want, r.stderr)
		}
	}
}

// sumVersions returns the version column of every line of gocpp.sum, without the
// "/gocpp.mod" and "/go.mod" suffixes, as "module version" pairs without duplicates.
func sumVersions(t *testing.T, dir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "gocpp.sum"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		version, _, _ := strings.Cut(fields[1], "/")
		pair := fields[0] + " " + version
		if !seen[pair] {
			seen[pair] = true
			out = append(out, pair)
		}
	}
	sort.Strings(out)
	return out
}

// mentioning returns the requests whose path contains text.
func mentioning(requests []string, text string) []string {
	var out []string
	for _, r := range requests {
		if strings.Contains(r, text) {
			out = append(out, r)
		}
	}
	return out
}

// "gocpp get lib@v1.91.5" names a version: no content of the newer v1.93.0 is
// downloaded, and gocpp.sum lists only the modules of the build list.
func TestGetExactVersionDownloadsOnlyThatVersion(t *testing.T) {
	for _, c := range []struct {
		name  string
		files func(version string) map[string]string
	}{
		{"no module file", func(version string) map[string]string {
			return map[string]string{"imgui.h": "int imgui;\n"}
		}},
		{"module file", func(version string) map[string]string {
			return map[string]string{"gocpp.mod": "module github.com/acme/imgui\n", "imgui.h": "int imgui;\n"}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := startGitHost(t)
			h.tag("acme/imgui", "v1.91.5", c.files("v1.91.5"))
			h.tag("acme/imgui", "v1.93.0", c.files("v1.93.0"))
			h.env(t)
			dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n"})
			h.requested()

			r := gocppRun("", false, "get", "github.com/acme/imgui@v1.91.5")
			if r.code != 0 {
				t.Fatalf("get: %+v", r)
			}

			requests := h.requested()
			if got := mentioning(requests, "v1.93.0"); len(got) > 0 {
				t.Errorf("the host was asked for v1.93.0: %q", got)
			}
			if got := mentioning(requests, "v1.91.5.tar.gz"); len(got) == 0 {
				t.Errorf("the pinned archive was not downloaded: %q", requests)
			}
			want := []string{"github.com/acme/imgui v1.91.5"}
			if got := sumVersions(t, dir); !slices.Equal(got, want) {
				t.Errorf("gocpp.sum versions = %q, want %q:\n%s", got, want, readFile(t, filepath.Join(dir, "gocpp.sum")))
			}
		})
	}
}

// twoLibraries is a host with a newer tag for each of two libraries, a project
// that has not required anything, and the host's requests cleared.
func twoLibraries(t *testing.T) (*gitHost, string) {
	t.Helper()
	h := startGitHost(t)
	for _, lib := range []string{"imgui", "implot"} {
		h.tag("acme/"+lib, "v1.0.0", map[string]string{lib + ".h": "int old;\n"})
		h.tag("acme/"+lib, "v1.1.0", map[string]string{lib + ".h": "int newer;\n"})
	}
	h.env(t)
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n"})
	h.requested()
	return h, dir
}

// A second "get" of another module, also pinned, leaves gocpp.sum with the
// versions of the two modules and nothing else.
func TestGetSecondPinnedModuleKeepsSumMinimal(t *testing.T) {
	h, dir := twoLibraries(t)
	for _, arg := range []string{"github.com/acme/imgui@v1.0.0", "github.com/acme/implot@v1.0.0"} {
		if r := gocppRun("", false, "get", arg); r.code != 0 {
			t.Fatalf("get %s: %+v", arg, r)
		}
	}

	if got := mentioning(h.requested(), "v1.1.0"); len(got) > 0 {
		t.Errorf("the host was asked for v1.1.0: %q", got)
	}
	want := []string{"github.com/acme/imgui v1.0.0", "github.com/acme/implot v1.0.0"}
	if got := sumVersions(t, dir); !slices.Equal(got, want) {
		t.Errorf("gocpp.sum versions = %q, want %q", got, want)
	}
}

// "gocpp get" with no module downloads the build list as it is: no other version
// of a required module is asked for or recorded.
func TestGetWithoutArgumentsDownloadsOnlyTheBuildList(t *testing.T) {
	h, dir := twoLibraries(t)
	mod := "module example.com/app\n\nrequire github.com/acme/imgui v1.0.0\n"
	if err := os.WriteFile(filepath.Join(dir, "gocpp.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}

	r := gocppRun("", false, "get")
	if r.code != 0 {
		t.Fatalf("get: %+v", r)
	}

	if got := mentioning(h.requested(), "v1.1.0"); len(got) > 0 {
		t.Errorf("the host was asked for v1.1.0: %q", got)
	}
	want := []string{"github.com/acme/imgui v1.0.0"}
	if got := sumVersions(t, dir); !slices.Equal(got, want) {
		t.Errorf("gocpp.sum versions = %q, want %q", got, want)
	}
}
