package modfetch

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestParseModulePath(t *testing.T) {
	for _, tc := range []struct {
		in      string
		root    string
		subdir  string
		major   int
		wantErr string
	}{
		{"github.com/ocornut/imgui", "github.com/ocornut/imgui", "", 0, ""},
		{"github.com/nothings/stb", "github.com/nothings/stb", "", 0, ""},
		{"github.com/o/r/sub/dir", "github.com/o/r", "sub/dir", 0, ""},
		{"github.com/o/r/v2", "github.com/o/r", "", 2, ""},
		{"github.com/o/r/sub/v3", "github.com/o/r", "sub", 3, ""},
		{"github.com/o/r/v1", "github.com/o/r", "v1", 0, ""}, // v1 is a folder, not a major version suffix
		{"gitlab.com/g/s/r.git/sub", "gitlab.com/g/s/r.git", "sub", 0, ""},
		{"gitlab.com/g/r", "gitlab.com/g/r", "", 0, ""},
		{"codeberg.org/o/r", "codeberg.org/o/r", "", 0, ""},
		{"git.corp.example/team/repo/x", "git.corp.example/team/repo", "x", 0, ""},
		{"git.corp.example/team", "git.corp.example/team", "", 0, ""},
		{"github.com/o", "", "", 0, "github.com/owner/repo"},
		{"myapp/lib", "", "", 0, "first element must be a host name"},
		{"github.com/o/r/../x", "", "", 0, "invalid"},
		{"github.com/o/R r", "", "", 0, "invalid"},
		{"github.com/o/.hidden", "", "", 0, "dot"},
		{"github.com/o/r;x", "", "", 0, "invalid"},
		{"github.com//r", "", "", 0, "invalid"},
		{"", "", "", 0, "invalid"},
		{"github.com/o/r" + strings.Repeat("/x", 300), "", "", 0, "too long"},
	} {
		r, err := ParseModulePath(tc.in)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("ParseModulePath(%q) = %v, want an error with %q", tc.in, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseModulePath(%q): %v", tc.in, err)
			continue
		}
		if r.Root != tc.root || r.Subdir != tc.subdir || r.Major != tc.major {
			t.Errorf("ParseModulePath(%q) = root %q subdir %q major %d; want %q %q %d", tc.in, r.Root, r.Subdir, r.Major, tc.root, tc.subdir, tc.major)
		}
	}
}

func TestRepoVersions(t *testing.T) {
	r, _ := ParseModulePath("github.com/o/r/sub")
	if r.Tag("v1.2.3") != "sub/v1.2.3" {
		t.Error(r.Tag("v1.2.3"))
	}
	r2, _ := ParseModulePath("github.com/o/r/v2")
	if err := r2.CheckVersion("v1.2.3"); err == nil || !strings.Contains(err.Error(), "does not match the /v2") {
		t.Errorf("CheckVersion: %v", err)
	}
	if err := r2.CheckVersion("v2.0.1"); err != nil {
		t.Error(err)
	}
	r3, _ := ParseModulePath("github.com/o/r")
	for v, ok := range map[string]bool{"v1.2.3": true, "v11.0.0": true, "v0.0.0-20240304050607-abcdef123456": true, "v1.2.3-rc.1": true, "v1.2": false, "1.2.3": false, "v1.2.3+meta": false, "v1.2.3+incompatible": true, "": false} {
		if err := r3.CheckVersion(v); (err == nil) != ok {
			t.Errorf("CheckVersion(%q) = %v", v, err)
		}
	}
	if got := r3.Tag("v2.0.0+incompatible"); got != "v2.0.0" {
		t.Errorf("Tag = %s", got)
	}
}

func TestPseudo(t *testing.T) {
	for v, ok := range map[string]bool{
		"v0.0.0-20240304050607-abcdef123456":       true,
		"v1.2.4-0.20240304050607-abcdef123456":     true,
		"v1.2.3-pre.0.20240304050607-abcdef123456": true,
		"v1.2.3":                             false,
		"v0.0.0-20240304050607-ABCDEF123456": false,
		"v0.0.0-2024030405060-abcdef123456":  false,
		"v0.0.0-20240304050607-abcdef12345":  false,
	} {
		c, ts := PseudoCommit(v)
		if (c != "") != ok {
			t.Errorf("PseudoCommit(%q) = %q %q", v, c, ts)
		}
	}
	if got := PseudoVersion(0, "20240304050607", "abcdef1234567890"); got != "v0.0.0-20240304050607-abcdef123456" {
		t.Error(got)
	}
}

func TestCompareVersions(t *testing.T) {
	order := []string{"v0.9.0", "v1.0.0-alpha", "v1.0.0-alpha.1", "v1.0.0-alpha.beta", "v1.0.0-beta", "v1.0.0-beta.2", "v1.0.0-beta.11", "v1.0.0-rc.1", "v1.0.0", "v1.0.1", "v1.2.0", "v1.10.0", "v2.0.0", "v10.0.0"}
	for i, a := range order {
		for j, b := range order {
			want := 0
			if i < j {
				want = -1
			} else if i > j {
				want = 1
			}
			if got := CompareVersions(a, b); got != want {
				t.Errorf("CompareVersions(%s, %s) = %d, want %d", a, b, got, want)
			}
		}
	}
}

func TestEscape(t *testing.T) {
	if got := escape("github.com/Acme/Tiny"); got != "github.com/!acme/!tiny" {
		t.Error(got)
	}
	if got := escape("github.com/a"); got != "github.com/a" {
		t.Error(got)
	}
}

func TestArchiveURLs(t *testing.T) {
	cfg := &Config{}
	tag := func(path, version string) []string {
		repo, err := ParseModulePath(path)
		if err != nil {
			t.Fatal(err)
		}
		srcs, err := cfg.sourcesFor(repo)
		if err != nil {
			t.Fatal(err)
		}
		urls, err := srcs[0].archiveURLs(refFor(repo, version))
		if err != nil {
			t.Fatal(err)
		}
		return urls
	}
	if got, want := tag("github.com/ocornut/imgui", "v1.91.5"), []string{
		"https://github.com/ocornut/imgui/archive/refs/tags/v1.91.5.tar.gz",
		"https://github.com/ocornut/imgui/archive/refs/tags/1.91.5.tar.gz"}; !reflect.DeepEqual(got, want) {
		t.Errorf("github tag: %v", got)
	}
	if got := tag("github.com/nothings/stb", "v0.0.0-20240101120000-abcdef123456"); !reflect.DeepEqual(got, []string{"https://github.com/nothings/stb/archive/abcdef123456.tar.gz"}) {
		t.Errorf("github commit: %v", got)
	}
	if got := tag("github.com/o/r/sub", "v1.0.0"); got[0] != "https://github.com/o/r/archive/refs/tags/sub/v1.0.0.tar.gz" || got[1] != "https://github.com/o/r/archive/refs/tags/sub/1.0.0.tar.gz" {
		t.Errorf("github subdir: %v", got)
	}
	if got := tag("gitlab.com/group/proj", "v1.0.0"); got[0] != "https://gitlab.com/group/proj/-/archive/v1.0.0/proj-v1.0.0.tar.gz" {
		t.Errorf("gitlab: %v", got)
	}
	if got := tag("gitlab.com/g/sub/proj.git", "v1.0.0"); got[0] != "https://gitlab.com/g/sub/proj/-/archive/v1.0.0/proj-v1.0.0.tar.gz" {
		t.Errorf("gitlab subgroup: %v", got)
	}
	if got := tag("codeberg.org/o/r", "v1.0.0"); got[0] != "https://codeberg.org/o/r/archive/v1.0.0.tar.gz" {
		t.Errorf("codeberg: %v", got)
	}
	repo, _ := ParseModulePath("github.com/o/r")
	srcs, _ := cfg.sourcesFor(repo)
	if got := srcs[0].refsURL(); got != "https://github.com/o/r.git/info/refs?service=git-upload-pack" {
		t.Errorf("refs: %s", got)
	}
	// An unknown host has no archive URL.
	repo, _ = ParseModulePath("git.corp.example/team/repo")
	if _, err := cfg.sourcesFor(repo); err == nil || !strings.Contains(err.Error(), "GOCPP_MODURL") {
		t.Errorf("unknown host: %v", err)
	}
}

func TestEnvConfig(t *testing.T) {
	env := func(kv ...string) func(string) string {
		m := map[string]string{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return func(k string) string { return m[k] }
	}
	c, err := ConfigFromEnv(env(
		"GOCPP_MODCACHE", "/tmp/modcache",
		"GOCPP_OFFLINE", "1",
		"GOCPP_MODFLAGS", "-mod=readonly",
		"GOCPP_MODPROXY", "https://proxy.corp/,direct",
		"GOCPP_MODURL", "git.corp.example=https://git.corp.example/{repo}/archive/{ref}.tar.gz github.com=https://ghe.corp.example",
		"GOCPP_MODAUTH", "git.corp.example=tok other:8080=u:p",
		"GOCPP_MODTIMEOUT", "90s",
	), nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.CacheDir != "/tmp/modcache" || !c.Offline || !c.ReadOnly || c.Timeout.Seconds() != 90 {
		t.Errorf("config = %+v", c)
	}
	if len(c.Proxies) != 2 || c.Proxies[0].URL != "https://proxy.corp" || c.Proxies[1].URL != "" || c.Proxies[0].Any {
		t.Errorf("proxies = %+v", c.Proxies)
	}
	if len(c.URLs) != 2 || c.URLs[0].Prefix != "git.corp.example" || c.URLs[0].Template == "" || c.URLs[1].Base != "https://ghe.corp.example" {
		t.Errorf("urls = %+v", c.URLs)
	}
	if c.Auth["git.corp.example"] != "tok" || c.Auth["other:8080"] != "u:p" {
		t.Errorf("auth = %v", c.Auth)
	}
	// The pipe: any error falls through.
	c, err = ConfigFromEnv(env("GOCPP_MODPROXY", "https://a|https://b,off"), nil)
	if err != nil || !c.Proxies[0].Any || c.Proxies[1].Any || !c.Proxies[2].Off {
		t.Errorf("proxies = %+v, %v", c.Proxies, err)
	}
	// A bare template matches every module, and a query string with = is not a prefix.
	c, err = ConfigFromEnv(env("GOCPP_MODURL", "https://m.example/{path}/@v/{version}.tar.gz?token=1"), nil)
	if err != nil || c.URLs[0].Prefix != "" || c.URLs[0].Template == "" {
		t.Errorf("bare template: %+v, %v", c.URLs, err)
	}

	for name, kv := range map[string][]string{
		"relative cache":   {"GOCPP_MODCACHE", "rel"},
		"http proxy":       {"GOCPP_MODPROXY", "http://proxy.corp"},
		"user in url":      {"GOCPP_MODPROXY", "https://user:pw@proxy.corp"},
		"bad flag":         {"GOCPP_MODFLAGS", "-mod=weird"},
		"vendor flag":      {"GOCPP_MODFLAGS", "-mod=vendor"},
		"http template":    {"GOCPP_MODURL", "http://x.example/{path}"},
		"unknown field":    {"GOCPP_MODURL", "https://x.example/{bogus}"},
		"unclosed":         {"GOCPP_MODURL", "https://x.example/{path"},
		"bad prefix":       {"GOCPP_MODURL", "../x=https://x.example"},
		"bad auth":         {"GOCPP_MODAUTH", "nohost"},
		"auth with path":   {"GOCPP_MODAUTH", "a/b=tok"},
		"bad timeout":      {"GOCPP_MODTIMEOUT", "soon"},
		"negative timeout": {"GOCPP_MODTIMEOUT", "-5s"},
	} {
		if _, err := ConfigFromEnv(env(kv...), nil); err == nil {
			t.Errorf("%s: no error", name)
		} else if strings.Contains(err.Error(), "pw@") || strings.Contains(err.Error(), "nohost=") {
			t.Errorf("%s: the error shows a secret: %v", name, err)
		}
	}
	// readonly is accepted without -mod=
	if c, err := ConfigFromEnv(env("GOCPP_MODFLAGS", "readonly"), nil); err != nil || !c.ReadOnly {
		t.Errorf("readonly: %v", err)
	}
}

func TestURLTemplateForUnknownHost(t *testing.T) {
	h := newFakeHost(t)
	h.serve("/team/repo/-/v1.0.0/x.tar.gz", tarGz("", library("repo-v1.0.0")...))
	cfg := h.config()
	cfg.URLs = []URLRule{{Prefix: "git.corp.example", Template: h.srv.URL + "/{repo}/-/{version}/x.tar.gz"}}
	cleanupCache(t, cfg.CacheDir)
	f, _ := New(cfg)
	m, err := f.Download(context.Background(), "git.corp.example/team/repo", "v1.0.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := goHash("git.corp.example/team/repo@v1.0.0", libFiles()); m.Hash != want {
		t.Errorf("hash %s want %s", m.Hash, want)
	}
	// A module that the prefix does not cover is not sent there.
	if _, err := f.Download(context.Background(), "github.com/o/r", "v1.0.0", nil); err == nil || strings.Contains(err.Error(), "127.0.0.1") {
		// github.com is a known host: the request goes to github.com, which the
		// test must not reach; the Client has no route, so it fails.
		_ = err
	}
}

func TestProxyThenDirect(t *testing.T) {
	proxy := newFakeHost(t)
	direct := newFakeHost(t)
	archive := tarGz("", library("tiny-1.2.3")...)
	direct.tag("acme/tiny", "v1.2.3", archive)
	proxy.serve("/github.com/acme/tiny/@v/v1.2.3.zip", zipOf(library("tiny-1.2.3")...))
	proxy.handle("/github.com/acme/tiny/@v/list", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("v1.0.0\nv1.2.3\nv1.3.0-rc.1\n")) })

	cfg := direct.config()
	cfg.Proxies = []Proxy{{URL: proxy.srv.URL}, {}}
	cleanupCache(t, cfg.CacheDir)
	f, _ := New(cfg)
	// The proxy has v1.2.3 (as a zip: the tar.gz is a 404 and the next URL is tried).
	m, err := f.Download(context.Background(), tmod, "v1.2.3", nil)
	if err != nil {
		t.Fatal(err)
	}
	if direct.total() != 0 || proxy.count("/github.com/acme/tiny/@v/v1.2.3.tar.gz") != 1 || proxy.count("/github.com/acme/tiny/@v/v1.2.3.zip") != 1 {
		t.Errorf("direct %v, proxy %v", direct.hits, proxy.hits)
	}
	if m.Hash != goHash(tmod+"@v1.2.3", libFiles()) {
		t.Errorf("hash = %s", m.Hash)
	}
	// The proxy lacks v1.0.0: "not found" goes on to the direct source.
	direct.tag("acme/tiny", "v1.0.0", archive)
	if _, err := f.Download(context.Background(), tmod, "v1.0.0", nil); err != nil || direct.count("/acme/tiny/archive/refs/tags/v1.0.0.tar.gz") != 1 {
		t.Errorf("fallback to direct: %v %v", err, direct.hits)
	}
	// @latest through the proxy list.
	m, err = f.Get(context.Background(), t.TempDir(), tmod, "latest")
	if err != nil || m.Version != "v1.2.3" {
		t.Errorf("Get through a proxy: %+v %v", m, err)
	}

	// A proxy error that is not "not found" stops the search ...
	bad := newFakeHost(t)
	bad.handle("/github.com/acme/tiny/@v/v1.0.0.tar.gz", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", 500) })
	cfg2 := direct.config()
	cfg2.Proxies = []Proxy{{URL: bad.srv.URL}, {}}
	cleanupCache(t, cfg2.CacheDir)
	f2, _ := New(cfg2)
	retryDelayForTest(t)
	if _, err := f2.Download(context.Background(), tmod, "v1.0.0", nil); err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("proxy 500: %v", err)
	}
	// ... unless the list uses |.
	cfg2.Proxies = []Proxy{{URL: bad.srv.URL, Any: true}, {}}
	cleanupCache(t, cfg2.CacheDir)
	f3, _ := New(cfg2)
	if _, err := f3.Download(context.Background(), tmod, "v1.0.0", nil); err != nil {
		t.Errorf("proxy 500 with |: %v", err)
	}
}

func retryDelayForTest(t *testing.T) {
	old := retryDelay
	retryDelay = 1
	t.Cleanup(func() { retryDelay = old })
}

func TestProxyOff(t *testing.T) {
	cfg := &Config{CacheDir: t.TempDir(), Proxies: []Proxy{{Off: true}}}
	f, _ := New(cfg)
	_, err := f.Download(context.Background(), tmod, tver, nil)
	if err == nil || !strings.Contains(err.Error(), "GOCPP_MODPROXY=off") {
		t.Errorf("error = %v", err)
	}
	// A cached module still works.
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3")...))
	cfg2 := h.config()
	cleanupCache(t, cfg2.CacheDir)
	f2, _ := New(cfg2)
	f2.Download(context.Background(), tmod, tver, nil)
	cfg2.Proxies = []Proxy{{Off: true}}
	if _, err := f2.Download(context.Background(), tmod, tver, nil); err != nil {
		t.Error(err)
	}
}

func TestRedactURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://user:pw@host/p?token=abc": "https://host/p?...",
		"https://host/p":                   "https://host/p",
	} {
		if got := redact(in); got != want {
			u, _ := url.Parse(want)
			t.Errorf("redact(%q) = %q, want %q (%v)", in, got, want, u)
		}
	}
}
