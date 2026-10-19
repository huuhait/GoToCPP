package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/modfetch"
)

// A pmod is a module version of a test module proxy (a file:// GOCPP_MODPROXY).
type pmod struct {
	path, version string
	reqs          []string // "path@version"
}

func writeTestProxy(t *testing.T, dir string, mods []pmod) {
	t.Helper()
	versions := map[string][]string{}
	for _, m := range mods {
		d := filepath.Join(dir, filepath.FromSlash(m.path), "@v")
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		var gm strings.Builder
		fmt.Fprintf(&gm, "module %s\n\ngo 1.16\n", m.path)
		for _, r := range m.reqs {
			p, v, _ := strings.Cut(r, "@")
			fmt.Fprintf(&gm, "require %s %s\n", p, v)
		}
		tree := t.TempDir()
		os.WriteFile(filepath.Join(tree, "go.mod"), []byte(gm.String()), 0o644)
		os.WriteFile(filepath.Join(tree, "x.go"), []byte("package x\n"), 0o644)
		var zip bytes.Buffer
		if err := modfetch.CreateZip(context.Background(), &zip, tree, m.path, m.version); err != nil {
			t.Fatal(err)
		}
		info, _ := json.Marshal(map[string]any{"Version": m.version, "Time": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)})
		for name, data := range map[string][]byte{m.version + ".zip": zip.Bytes(), m.version + ".mod": []byte(gm.String()), m.version + ".info": info} {
			if err := os.WriteFile(filepath.Join(d, name), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		versions[m.path] = append(versions[m.path], m.version)
	}
	for p, vs := range versions {
		sort.Strings(vs)
		os.WriteFile(filepath.Join(dir, filepath.FromSlash(p), "@v", "list"), []byte(strings.Join(vs, "\n")+"\n"), 0o644)
	}
}

// proxyEnv points the module downloads at a file:// proxy of the modules, with
// a module cache of the test's own.
func proxyEnv(t *testing.T, mods ...pmod) (cache string) {
	t.Helper()
	root := t.TempDir()
	proxy := filepath.Join(root, "proxy")
	writeTestProxy(t, proxy, mods)
	cache = filepath.Join(root, "modcache")
	t.Cleanup(func() { removeReadOnly(cache) })
	t.Setenv("GOCPP_MODCACHE", cache)
	t.Setenv("GOCPP_MODPROXY", "file://"+proxy)
	t.Setenv("GOCPP_OFFLINE", "")
	t.Setenv("GOCPP_MODFLAGS", "")
	t.Setenv("GOCPP_MODURL", "")
	return cache
}

var getMods = []pmod{
	{"example.com/a", "v1.0.0", []string{"example.com/b@v1.0.0"}},
	{"example.com/a", "v1.1.0", []string{"example.com/b@v1.0.0"}},
	{"example.com/a", "v1.1.1", nil},
	{"example.com/b", "v1.0.0", nil},
	{"example.com/b", "v1.1.0", nil},
	{"example.com/c", "v1.0.0", []string{"example.com/a@v1.1.0"}},
}

func TestGetCLIPrintsChangesAndWritesBothFiles(t *testing.T) {
	proxyEnv(t, getMods...)
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n\ngocpp 0.1\n"})
	r := gocppRun("", false, "get", "example.com/a@v1.0.0")
	if r.code != 0 {
		t.Fatalf("get: %+v", r)
	}
	for _, want := range []string{"gocpp: added example.com/a v1.0.0\n", "gocpp: added example.com/b v1.0.0\n"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, r.stderr)
		}
	}
	if got := readFile(t, filepath.Join(dir, "gocpp.mod")); got != "module example.com/app\n\ngocpp 0.1\n\nrequire example.com/a v1.0.0\n\nrequire example.com/b v1.0.0 // indirect\n" &&
		!strings.Contains(got, "require example.com/b v1.0.0 // indirect") {
		t.Errorf("gocpp.mod = %q", got)
	}
	sum := readFile(t, filepath.Join(dir, "gocpp.sum"))
	for _, want := range []string{"example.com/a v1.0.0 h1:", "example.com/a v1.0.0/go.mod h1:", "example.com/b v1.0.0 h1:", "example.com/b v1.0.0/go.mod h1:"} {
		if !strings.Contains(sum, want) {
			t.Errorf("gocpp.sum lacks %q:\n%s", want, sum)
		}
	}
	// A second run changes nothing and says nothing.
	r = gocppRun("", false, "get", "example.com/a@v1.0.0")
	if r.code != 0 || strings.Contains(r.stderr, "added") || strings.Contains(r.stderr, "upgraded") {
		t.Errorf("second get: %+v", r)
	}
}

func TestGetCLIFlagsMayFollowModules(t *testing.T) {
	proxyEnv(t, getMods...)
	inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n\nrequire example.com/a v1.0.0\n"})
	r := gocppRun("", false, "get", "example.com/a", "-u=patch", "-t")
	if r.code != 0 || !strings.Contains(r.stderr, "gocpp: upgraded example.com/a v1.0.0 => v1.1.1") {
		t.Fatalf("get: %+v", r)
	}
	// -u=bogus is a usage error.
	r = gocppRun("", false, "get", "-u=bogus", "example.com/a")
	if r.code != 2 || !strings.Contains(r.stderr, "want -u or -u=patch") {
		t.Errorf("-u=bogus: %+v", r)
	}
}

func TestGetCLIUpgradeAll(t *testing.T) {
	proxyEnv(t, getMods...)
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n\nrequire example.com/c v1.0.0\n"})
	r := gocppRun("", false, "get", "-u")
	if r.code != 0 {
		t.Fatalf("get -u: %+v", r)
	}
	mod := readFile(t, dir+"/gocpp.mod")
	for _, want := range []string{"example.com/a v1.1.1 // indirect", "example.com/b v1.1.0 // indirect"} {
		if !strings.Contains(mod, want) {
			t.Errorf("gocpp.mod lacks %q:\n%s", want, mod)
		}
	}
}

func TestGetCLIFailureChangesNothing(t *testing.T) {
	proxyEnv(t, getMods...)
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n"})
	r := gocppRun("", false, "get", "example.com/a@v1.0.0", "example.com/nothing@v1.0.0")
	if r.code != 1 || !strings.Contains(r.stderr, "gocpp get: gocpp.mod was not changed") {
		t.Fatalf("get: %+v", r)
	}
	if readFile(t, filepath.Join(dir, "gocpp.mod")) != "module example.com/app\n" {
		t.Error("gocpp.mod changed")
	}
	if _, err := os.Stat(filepath.Join(dir, "gocpp.sum")); err == nil {
		t.Error("gocpp.sum was written")
	}
}

func TestGetCLIOffline(t *testing.T) {
	proxyEnv(t, getMods...)
	inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n"})
	t.Setenv("GOCPP_OFFLINE", "1")
	r := gocppRun("", false, "get", "example.com/a")
	if r.code != 1 || !strings.Contains(r.stderr, "downloads are off") {
		t.Fatalf("offline latest: %+v", r)
	}
	r = gocppRun("", false, "get", "example.com/a@v1.0.0")
	if r.code != 1 || !strings.Contains(r.stderr, "GOCPP_OFFLINE=1") {
		t.Fatalf("offline version: %+v", r)
	}
}

func TestModGraphCLI(t *testing.T) {
	proxyEnv(t, getMods...)
	inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n\nrequire example.com/c v1.0.0\n"})
	r := gocppRun("", false, "mod", "graph")
	if r.code != 0 {
		t.Fatalf("mod graph: %+v", r)
	}
	want := "example.com/app example.com/c@v1.0.0\nexample.com/c@v1.0.0 example.com/a@v1.1.0\nexample.com/a@v1.1.0 example.com/b@v1.0.0\n"
	if r.stdout != want {
		t.Errorf("graph:\n%s\nwant:\n%s", r.stdout, want)
	}
	// It writes nothing.
	if _, err := os.Stat("gocpp.sum"); err == nil {
		t.Error("mod graph wrote gocpp.sum")
	}
	if r := gocppRun("", false, "mod", "graph", "extra"); r.code != 2 {
		t.Errorf("graph with an argument: %+v", r)
	}
	if r := gocppRun("", false, "mod", "graph", "-h"); r.code != 0 || !strings.Contains(r.stderr, "usage: gocpp mod graph") {
		t.Errorf("graph -h: %+v", r)
	}
}

func TestModWhyCLI(t *testing.T) {
	proxyEnv(t, getMods...)
	inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n\nrequire example.com/c v1.0.0\n"})
	r := gocppRun("", false, "mod", "why", "-m", "example.com/b", "example.com/zzz")
	if r.code != 0 {
		t.Fatalf("mod why: %+v", r)
	}
	want := "# example.com/b\nexample.com/app\nexample.com/c\nexample.com/a\nexample.com/b\n\n# example.com/zzz\n(main module does not need module example.com/zzz)\n"
	if r.stdout != want {
		t.Errorf("why:\n%s\nwant:\n%s", r.stdout, want)
	}
	if r := gocppRun("", false, "mod", "why"); r.code != 2 {
		t.Errorf("why with no module: %+v", r)
	}
}

func TestModGraphFailsClearlyForAMissingModule(t *testing.T) {
	proxyEnv(t, getMods...)
	inDir(t, map[string]string{"gocpp.mod": "module example.com/app\n\nrequire example.com/missing v1.0.0\n"})
	r := gocppRun("", false, "mod", "graph")
	if r.code != 1 || !strings.Contains(r.stderr, "example.com/missing@v1.0.0") {
		t.Errorf("mod graph: %+v", r)
	}
}
