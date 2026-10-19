package modget

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

	"github.com/huuhait/gocpp/internal/load/gocppmod"
	"github.com/huuhait/gocpp/internal/modfetch"
)

// The tests never use the network, the user's module cache or a real module
// proxy: every module comes from a file:// proxy made by the test.
func TestMain(m *testing.M) {
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}

// A tmod is a module version of a test proxy.
type tmod struct {
	Path, Version string
	Reqs          []string          // "path@version"
	Extra         string            // extra lines of its go.mod (retract, exclude)
	Files         map[string]string // files besides go.mod and x.go
	NoModFile     bool              // no go.mod: a library, or a Go module of before modules
	Time          time.Time         // the time in its .info (2020-01-01 if zero)
	Unlisted      bool              // not in the proxy's list (a pseudo-version)
}

func (m tmod) goMod() string {
	var b strings.Builder
	fmt.Fprintf(&b, "module %s\n\ngo 1.16\n", m.Path)
	for _, r := range m.Reqs {
		p, v, _ := strings.Cut(r, "@")
		fmt.Fprintf(&b, "require %s %s\n", p, v)
	}
	b.WriteString(m.Extra)
	return b.String()
}

// writeProxy writes a module proxy (the layout of GOPROXY=file://) for the
// modules.
func writeProxy(t testing.TB, dir string, mods []tmod) {
	t.Helper()
	versions := map[string][]string{}
	for _, m := range mods {
		d := filepath.Join(dir, filepath.FromSlash(m.Path), "@v")
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		tree := t.TempDir()
		files := map[string]string{"x.go": "package x\n"}
		if !m.NoModFile {
			files["go.mod"] = m.goMod()
		}
		for n, c := range m.Files {
			files[n] = c
		}
		for n, c := range files {
			p := filepath.Join(tree, filepath.FromSlash(n))
			os.MkdirAll(filepath.Dir(p), 0o755)
			if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		var zip bytes.Buffer
		if err := modfetch.CreateZip(context.Background(), &zip, tree, m.Path, m.Version); err != nil {
			t.Fatal(err)
		}
		write := func(name string, data []byte) {
			if err := os.WriteFile(filepath.Join(d, name), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		write(m.Version+".zip", zip.Bytes())
		modText := m.goMod()
		if m.NoModFile {
			modText = "module " + m.Path + "\n"
		}
		write(m.Version+".mod", []byte(modText))
		when := m.Time
		if when.IsZero() {
			when = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		info, _ := json.Marshal(map[string]any{"Version": m.Version, "Time": when})
		write(m.Version+".info", info)
		if !m.Unlisted {
			versions[m.Path] = append(versions[m.Path], m.Version)
		}
	}
	for p, vs := range versions {
		sort.Slice(vs, func(i, j int) bool { return gocppmod.CompareVersions(vs[i], vs[j]) < 0 })
		list := strings.Join(vs, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(p), "@v", "list"), []byte(list), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// An env is a project, a module proxy and a module cache.
type env struct {
	t     *testing.T
	proxy string
	cache string
	dir   string
	f     *modfetch.Fetcher
	cfg   *modfetch.Config
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

// newEnv makes a project with the given gocpp.mod text and a proxy of mods.
func newEnv(t *testing.T, gocppMod string, mods ...tmod) *env {
	t.Helper()
	root := t.TempDir()
	e := &env{t: t, proxy: filepath.Join(root, "proxy"), cache: filepath.Join(root, "cache"), dir: filepath.Join(root, "app")}
	t.Cleanup(func() { removeReadOnly(e.cache) })
	os.MkdirAll(e.dir, 0o755)
	writeProxy(t, e.proxy, mods)
	e.reopen()
	e.writeMod(gocppMod)
	return e
}

// reopen makes a new Fetcher (as a new process would have).
func (e *env) reopen() {
	e.cfg = &modfetch.Config{
		CacheDir: e.cache,
		Proxies:  []modfetch.Proxy{{URL: "file://" + e.proxy}},
	}
	f, err := modfetch.New(e.cfg)
	if err != nil {
		e.t.Fatal(err)
	}
	e.f = f
}

func (e *env) writeMod(text string) {
	e.t.Helper()
	if err := os.WriteFile(filepath.Join(e.dir, "gocpp.mod"), []byte(text), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) read(name string) string {
	b, err := os.ReadFile(filepath.Join(e.dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		e.t.Fatal(err)
	}
	return string(b)
}

func (e *env) mod() string { return e.read("gocpp.mod") }
func (e *env) sum() string { return e.read("gocpp.sum") }

// get runs gocpp get.
func (e *env) get(args ...string) (*Result, error) {
	return e.getOpts(Options{Args: args})
}

func (e *env) getOpts(o Options) (*Result, error) {
	o.Dir, o.Fetcher = e.dir, e.f
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return Run(ctx, o)
}

func (e *env) mustGet(args ...string) *Result {
	e.t.Helper()
	r, err := e.get(args...)
	if err != nil {
		e.t.Fatalf("get %v: %v", args, err)
	}
	return r
}

// requires returns the require lines of gocpp.mod as "path@version" (with
// " indirect" for the modules that only others need), sorted.
func (e *env) requires() []string {
	e.t.Helper()
	file, errs := gocppmod.Parse("gocpp.mod", []byte(e.mod()), nil)
	if len(errs) > 0 {
		e.t.Fatal(errs[0])
	}
	var out []string
	for _, r := range file.Require {
		s := r.Path + "@" + r.Version
		if r.Indirect {
			s += " indirect"
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func changes(r *Result) string {
	var s []string
	for _, c := range r.Changes {
		s = append(s, c.String())
	}
	return strings.Join(s, "; ")
}

func v(path string, versions ...string) []tmod {
	var out []tmod
	for _, ver := range versions {
		out = append(out, tmod{Path: path, Version: ver})
	}
	return out
}

func cat(parts ...[]tmod) []tmod {
	var out []tmod
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

const appMod = "module example.com/app\n\ngocpp 0.1\n"
