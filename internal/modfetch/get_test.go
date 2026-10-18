package modfetch

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRefs(t *testing.T) {
	h := newFakeHost(t)
	sha := func(c string) string { return strings.Repeat(c, 40) }
	h.refs("acme/tiny", sha("1"), "main",
		map[string]string{"v1.0.0": sha("2"), "v1.2.0": sha("3"), "v1.10.0": sha("4"), "v2.0.0-rc.1": sha("5"), "v1.3.0-annotated": sha("6"), "junk": sha("7"), "sub/v1.0.0": sha("8")},
		map[string]string{"main": sha("1"), "dev": sha("9")})
	f := h.fetcher()
	rl, err := f.gitRefs(context.Background(), f.cfg.URLs[0].Base+"/acme/tiny.git/info/refs?service=git-upload-pack")
	if err != nil {
		t.Fatal(err)
	}
	if rl.Head != sha("1") || rl.HeadName != "main" || rl.Branches["dev"] != sha("9") || rl.Tags["v1.2.0"] != sha("3") {
		t.Errorf("refs = %+v", rl)
	}
	if rl.Tags["v1.3.0-annotated"] != sha("a") {
		t.Errorf("the peeled commit of an annotated tag is %q", rl.Tags["v1.3.0-annotated"])
	}
	repo, _ := ParseModulePath(tmod)
	got := rl.versions(repo)
	want := []string{"v1.0.0", "v1.2.0", "v1.3.0-annotated", "v1.10.0", "v2.0.0-rc.1"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("versions = %v", got)
	}
	if l := latest(got); l != "v1.10.0" {
		t.Errorf("latest = %s (a prerelease must not win over a release)", l)
	}
	sub, _ := ParseModulePath(tmod + "/sub")
	if got := rl.versions(sub); len(got) != 1 || got[0] != "v1.0.0" {
		t.Errorf("versions of sub = %v", got)
	}
}

func TestParseRefsErrors(t *testing.T) {
	for name, body := range map[string]string{
		"html":    "<html>no</html>",
		"cut":     "001e# service=git-upload-pa",
		"v2":      "000eversion 2\n0000",
		"bad sha": "0014# service=x\n" + "0000" + "0012zzzz refs/heads/m\n",
		"empty":   "",
		"error":   "0014# service=x\n0000001dERR repository not found\n",
		"not hex": "xxxx",
	} {
		if _, err := parseRefs([]byte(body)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	// An empty repository: only the capabilities line.
	line := strings.Repeat("0", 40) + " capabilities^{}\x00agent=git\n"
	body := "001e# service=git-upload-pack\n0000" + fmt.Sprintf("%04x", len(line)+4) + line + "0000"
	rl, err := parseRefs([]byte(body))
	if err != nil || len(rl.Tags) != 0 {
		t.Errorf("empty repository: %v %+v", err, rl)
	}
}

func TestGetLatest(t *testing.T) {
	h := newFakeHost(t)
	sha := func(c string) string { return strings.Repeat(c, 40) }
	h.refs("acme/tiny", sha("1"), "main",
		map[string]string{"v1.0.0": sha("2"), "v1.2.0": sha("3"), "v1.10.0": sha("4"), "v2.0.0-rc.1": sha("5")},
		map[string]string{"main": sha("1")})
	h.tag("acme/tiny", "v1.10.0", tarGz("", library("tiny-1.10.0")...))
	h.tag("acme/tiny", "v1.2.0", tarGz("", library("tiny-1.2.0")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	main := t.TempDir()
	for _, q := range []string{"", "latest"} {
		m, err := f.Get(context.Background(), main, tmod, q)
		if err != nil || m.Version != "v1.10.0" {
			t.Errorf("Get(%q) = %+v, %v", q, m, err)
		}
	}
	// A prefix picks the highest of the line.
	m, err := f.Get(context.Background(), main, tmod, "v1.2")
	if err != nil || m.Version != "v1.2.0" {
		t.Errorf("Get(v1.2) = %+v, %v", m, err)
	}
	m, err = f.Get(context.Background(), main, tmod, "v1")
	if err != nil || m.Version != "v1.10.0" {
		t.Errorf("Get(v1) = %+v, %v", m, err)
	}
	if _, err := f.Get(context.Background(), main, tmod, "v7"); err == nil || !strings.Contains(err.Error(), "no version of") {
		t.Errorf("Get(v7): %v", err)
	}
	// gocpp.sum has both.
	data, _ := os.ReadFile(filepath.Join(main, "gocpp.sum"))
	if strings.Count(string(data), "\n") != 2 {
		t.Errorf("gocpp.sum = %q", data)
	}
	// An exact version needs no listing.
	n := h.count("/acme/tiny.git/info/refs")
	if m, err := f.Get(context.Background(), main, tmod, "v1.2.0"); err != nil || m.Version != "v1.2.0" || h.count("/acme/tiny.git/info/refs") != n {
		t.Errorf("exact: %+v %v", m, err)
	}
}

func TestGetNoTagsUsesHead(t *testing.T) {
	h := newFakeHost(t)
	head := "abcdef0123456789abcdef0123456789abcdef01"
	h.refs("acme/tiny", head, "master", map[string]string{"junk": strings.Repeat("7", 40)}, map[string]string{"master": head, "dev": strings.Repeat("9", 40)})
	p := h.commit("acme/tiny", head, tarGz(head, library("tiny-head")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	main := t.TempDir()
	m, err := f.Get(context.Background(), main, tmod, "latest")
	if err != nil {
		t.Fatal(err)
	}
	if want := "v0.0.0-20240304050607-abcdef012345"; m.Version != want {
		t.Errorf("version = %s, want %s (the time is the time of the archive's files)", m.Version, want)
	}
	if h.count(p) != 1 {
		t.Errorf("hits: %v", h.hits)
	}
	if got, _ := os.ReadFile(filepath.Join(main, "gocpp.sum")); !strings.HasPrefix(string(got), tmod+" "+m.Version+" h1:") {
		t.Errorf("gocpp.sum = %q", got)
	}
	// The same again: found in the cache under the pseudo-version.
	m2, err := f.Get(context.Background(), main, tmod, "latest")
	if err != nil || m2.Version != m.Version || m2.Downloaded {
		t.Errorf("second Get: %+v %v", m2, err)
	}
}

func TestGetBranchTagAndCommit(t *testing.T) {
	h := newFakeHost(t)
	dev := strings.Repeat("9", 40)
	odd := strings.Repeat("e", 40)
	h.refs("acme/tiny", strings.Repeat("1", 40), "main", map[string]string{"release-3": odd}, map[string]string{"main": strings.Repeat("1", 40), "dev": dev})
	h.commit("acme/tiny", dev, tarGz(dev, library("tiny-dev")...))
	h.commit("acme/tiny", odd, tarGz(odd, library("tiny-odd")...))
	// A commit that is not at a ref tip: the full name comes from the archive.
	loose := "123456789012"
	full := "123456789012" + strings.Repeat("a", 28)
	h.commit("acme/tiny", loose, tarGz(full, library("tiny-loose")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	main := t.TempDir()
	for q, want := range map[string]string{
		"dev":          "v0.0.0-20240304050607-999999999999",
		"release-3":    "v0.0.0-20240304050607-eeeeeeeeeeee",
		"eeeeeee":      "v0.0.0-20240304050607-eeeeeeeeeeee", // an abbreviation of a ref tip
		"123456789012": "v0.0.0-20240304050607-123456789012",
	} {
		m, err := f.Get(context.Background(), main, tmod, q)
		if err != nil || m.Version != want {
			t.Errorf("Get(%q) = %v, %v; want %s", q, m, err, want)
		}
	}
	if _, err := f.Get(context.Background(), main, tmod, "no-such-thing"); err == nil || !strings.Contains(err.Error(), "no such version, tag, branch or commit") {
		t.Errorf("Get(no-such-thing): %v", err)
	}
}

func TestGetRepositoryNotFound(t *testing.T) {
	h := newFakeHost(t)
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	_, err := f.Get(context.Background(), t.TempDir(), "github.com/acme/missing", "latest")
	if err == nil || !strings.Contains(err.Error(), "can't list the versions of github.com/acme/missing") || !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %v", err)
	}
}

func TestGetNotSmartHTTP(t *testing.T) {
	h := newFakeHost(t)
	h.handle("/acme/tiny.git/info/refs", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("0123456789abcdef refs/heads/main\n")) })
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	_, err := f.Get(context.Background(), t.TempDir(), tmod, "latest")
	if err == nil || !strings.Contains(err.Error(), "smart HTTP") {
		t.Errorf("error = %v", err)
	}
}

func TestListRefsRetriesOn503(t *testing.T) {
	retryDelayForTest(t)
	h := newFakeHost(t)
	sha := strings.Repeat("1", 40)
	h.refs("acme/tiny", sha, "main", map[string]string{"v1.0.0": sha}, map[string]string{"main": sha})
	inner := h.handlers["/acme/tiny.git/info/refs"]
	n := 0
	h.handle("/acme/tiny.git/info/refs", func(w http.ResponseWriter, r *http.Request) {
		if n++; n < 3 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		inner(w, r)
	})
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	h.tag("acme/tiny", "v1.0.0", tarGz("", library("tiny-1.0.0")...))
	m, err := f.Get(context.Background(), t.TempDir(), tmod, "latest")
	if err != nil || m.Version != "v1.0.0" || n != 3 {
		t.Errorf("Get = %+v, %v after %d requests", m, err, n)
	}
}
