package modget

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
	"github.com/huuhait/gocpp/internal/modfetch"
	"github.com/huuhait/gocpp/internal/modgraph"
)

// The differential tests run the real go command on the same module graphs
// (a file:// GOPROXY, no network, go 1.16 module files so that nothing is
// pruned) and compare what it selects with what gocpp selects.

// errGoTimeout: the go command did not finish (it can loop on some conflicting
// "@none" requests); the case says nothing about gocpp.
var errGoTimeout = errors.New("the go command timed out")

type goRunner struct {
	t    *testing.T
	root string
	env  []string
}

func newGoRunner(t *testing.T, proxy string) *goRunner {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go command is not on the PATH")
	}
	_ = goBin
	root := t.TempDir()
	r := &goRunner{t: t, root: root}
	r.env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + root,
		"GOPROXY=file://" + proxy,
		"GOSUMDB=off",
		"GONOSUMDB=*",
		"GOMODCACHE=" + filepath.Join(root, "modcache"),
		"GOCACHE=" + filepath.Join(root, "gocache"),
		"GOPATH=" + filepath.Join(root, "gopath"),
		"GOTOOLCHAIN=local",
		"GO111MODULE=on",
		"GOWORK=off",
		"GOPRIVATE=",
		"GONOPROXY=",
		"GOVCS=off",
		"GOINSECURE=",
		"GOENV=off",
		"CGO_ENABLED=0",
	}
	t.Cleanup(func() { removeReadOnly(root) })
	return r
}

// prepare writes go.mod and a go.sum that has the go.mod hash of every module
// of the proxy, so that the commands that follow run with -mod=readonly on the
// go.mod as it is (a -mod=mod run would first rewrite go.mod).
func (g *goRunner) prepare(dir, gomod string, mods []tmod) error {
	var sum strings.Builder
	for _, m := range mods {
		text := m.goMod()
		if m.NoModFile {
			text = "module " + m.Path + "\n"
		}
		h, err := modfetch.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(text)), nil
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(&sum, "%s %s/go.mod %s\n", m.Path, m.Version, h)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), []byte(sum.String()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644)
}

func (g *goRunner) run(dir string, args ...string) (string, error) {
	g.t.Helper()
	return g.runFlags(dir, "-mod=mod", args...)
}

func (g *goRunner) runFlags(dir, flags string, args ...string) (string, error) {
	g.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(append([]string{}, g.env...), "GOFLAGS="+flags+" -modcacherw")
	dieWithTest(cmd)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if err != nil && ctx.Err() != nil {
		return out.String(), errGoTimeout
	}
	if err != nil {
		return out.String(), fmt.Errorf("go %s: %v\n%s", strings.Join(args, " "), err, errb.String())
	}
	return out.String(), nil
}

// listAll is "go list -m all" as sorted "path@version" lines, the main module first.
func (g *goRunner) listAll(dir string) ([]string, error) {
	out, err := g.run(dir, "list", "-m", "all")
	if err != nil {
		return nil, err
	}
	var res []string
	for i, l := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Fields(l)
		if i == 0 {
			res = append(res, f[0])
			continue
		}
		res = append(res, f[0]+"@"+f[1])
	}
	sort.Strings(res[1:])
	return res, nil
}

// graph is "go mod graph" as lines.
func (g *goRunner) graph(dir string) ([]string, error) {
	out, err := g.run(dir, "mod", "graph")
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		// The go version is a pseudo-module in the graph of Go.
		if f := strings.Fields(l); len(f) == 2 && strings.HasPrefix(f[1], "go@") {
			continue
		}
		lines = append(lines, l)
	}
	return lines, nil
}

// A graphSpec is a random module graph.
type graphSpec struct {
	mods []tmod
	main []string // "path@version" requirements of the main module
}

func (s graphSpec) paths() []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range s.mods {
		if !seen[m.Path] {
			seen[m.Path] = true
			out = append(out, m.Path)
		}
	}
	sort.Strings(out)
	return out
}

func (s graphSpec) versionsOf(path string) []string {
	var out []string
	for _, m := range s.mods {
		if m.Path == path {
			out = append(out, m.Version)
		}
	}
	return out
}

func genGraph(rng *rand.Rand) graphSpec {
	n := 3 + rng.Intn(6)
	var s graphSpec
	type pv struct{ path, ver string }
	for i := 0; i < n; i++ {
		path := fmt.Sprintf("example.com/m%d", i)
		nv := 1 + rng.Intn(4)
		minor := 0
		for j := 0; j < nv; j++ {
			minor += 1 + rng.Intn(2)
			ver := fmt.Sprintf("v1.%d.%d", minor, rng.Intn(3))
			if rng.Intn(8) == 0 {
				ver = fmt.Sprintf("v0.%d.0", minor)
			}
			if rng.Intn(6) == 0 {
				ver = fmt.Sprintf("v1.%d.0-rc.1", minor)
			}
			s.mods = append(s.mods, tmod{Path: path, Version: ver})
		}
	}
	// Dedupe versions of one module (the generator may repeat one).
	seen := map[pv]bool{}
	var mods []tmod
	for _, m := range s.mods {
		k := pv{m.Path, m.Version}
		if !seen[k] {
			seen[k] = true
			mods = append(mods, m)
		}
	}
	s.mods = mods
	// Libraries that were never Go modules (no go.mod): tags v2 and above are
	// +incompatible versions; and modules with a /v2 path of their own.
	for i := 0; i < n; i++ {
		path := fmt.Sprintf("example.com/m%d", i)
		if rng.Intn(3) == 0 {
			for j, nv := 0, 1+rng.Intn(2); j < nv; j++ {
				s.mods = append(s.mods, tmod{Path: path, Version: fmt.Sprintf("v%d.%d.0+incompatible", 2+j, rng.Intn(3)), NoModFile: true})
			}
			if rng.Intn(2) == 0 {
				// The compatible releases have no go.mod either.
				for k := range s.mods {
					if s.mods[k].Path == path && !strings.HasSuffix(s.mods[k].Version, "+incompatible") {
						s.mods[k].NoModFile = true
					}
				}
			}
		}
		if rng.Intn(4) == 0 {
			s.mods = append(s.mods, tmod{Path: path + "/v2", Version: fmt.Sprintf("v2.%d.0", rng.Intn(3))})
		}
	}
	pick := func(exclude string) string {
		for {
			ps := s.paths()
			p := ps[rng.Intn(len(ps))]
			if p == exclude {
				continue
			}
			vs := s.versionsOf(p)
			return p + "@" + vs[rng.Intn(len(vs))]
		}
	}
	for i := range s.mods {
		k := rng.Intn(4)
		seenReq := map[string]bool{}
		for j := 0; j < k; j++ {
			r := pick(s.mods[i].Path)
			p, _, _ := strings.Cut(r, "@")
			if !seenReq[p] {
				seenReq[p] = true
				s.mods[i].Reqs = append(s.mods[i].Reqs, r)
			}
		}
	}
	seenReq := map[string]bool{}
	for j, k := 0, 1+rng.Intn(3); j < k; j++ {
		r := pick("")
		p, _, _ := strings.Cut(r, "@")
		if !seenReq[p] {
			seenReq[p] = true
			s.main = append(s.main, r)
		}
	}
	return s
}

func mainFileText(reqs []string, gomod bool) string {
	var b strings.Builder
	b.WriteString("module example.com/app\n\n")
	if gomod {
		b.WriteString("go 1.16\n\n")
	} else {
		b.WriteString("gocpp 0.1\n\n")
	}
	for _, r := range reqs {
		p, v, _ := strings.Cut(r, "@")
		fmt.Fprintf(&b, "require %s %s\n", p, v)
	}
	return b.String()
}

// gocppList is the build list of the project in e, as sorted "path@version" lines.
func gocppList(e *env) ([]string, *modgraph.Reqs, error) {
	file, errs := gocppmod.Parse("gocpp.mod", []byte(e.mod()), nil)
	if len(errs) > 0 {
		return nil, nil, errs[0]
	}
	src := &modgraph.FetcherSource{F: e.f, MainDir: e.dir, Pending: &modfetch.PendingSums{}}
	r := modgraph.New(context.Background(), &modgraph.Config{MainDir: e.dir, File: file, Source: src})
	l, _, err := r.BuildList()
	if err != nil {
		return nil, nil, err
	}
	var res []string
	for _, m := range l {
		res = append(res, m.String())
	}
	sort.Strings(res[1:])
	return res, r, nil
}

func diffCount(t *testing.T) int {
	var n int
	if _, err := fmt.Sscan(os.Getenv("GOCPP_DIFF_COUNT"), &n); err == nil && n > 0 {
		return n
	}
	if testing.Short() {
		return 10
	}
	return 50
}

func TestDifferentialBuildListAndGraph(t *testing.T) {
	for seed := 1; seed <= diffCount(t); seed++ {
		spec := genGraph(rand.New(rand.NewSource(int64(seed))))
		e := newEnv(t, mainFileText(spec.main, false), spec.mods...)
		g := newGoRunner(t, e.proxy)
		gdir := filepath.Join(g.root, "app")
		os.MkdirAll(gdir, 0o755)
		gomod := mainFileText(spec.main, true)

		if err := g.prepare(gdir, gomod, spec.mods); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		want, goErr := g.listAll(gdir)
		if err := g.prepare(gdir, gomod, spec.mods); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		got, _, err := gocppList(e)
		if goErr != nil || err != nil {
			t.Fatalf("seed %d: go: %v; gocpp: %v", seed, goErr, err)
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("seed %d: build list\n gocpp: %v\n go:    %v\nmods: %+v main: %v", seed, got, want, spec.mods, spec.main)
			continue
		}
		// The graph is compared for main requirements that are the selected
		// versions: go rewrites the others before it prints the graph.
		sel := map[string]string{}
		for _, l := range got[1:] {
			p, v, _ := strings.Cut(l, "@")
			sel[p] = v
		}
		var main2 []string
		for _, r := range spec.main {
			p, _, _ := strings.Cut(r, "@")
			main2 = append(main2, p+"@"+sel[p])
		}
		e.writeMod(mainFileText(main2, false))
		gomod = mainFileText(main2, true)
		if err := g.prepare(gdir, gomod, spec.mods); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		wantGraph, err := g.graph(gdir)
		if err != nil {
			t.Fatal(err)
		}
		_, r, err := gocppList(e)
		if err != nil {
			t.Fatal(err)
		}
		_, gr, err := r.BuildList()
		if err != nil {
			t.Fatal(err)
		}
		var gotGraph []string
		for _, ed := range modgraph.Edges(gr) {
			gotGraph = append(gotGraph, ed.From.String()+" "+ed.To.String())
		}
		if strings.Join(gotGraph, "\n") != strings.Join(wantGraph, "\n") {
			t.Errorf("seed %d: graph\n gocpp: %v\n go:    %v", seed, gotGraph, wantGraph)
		}
	}
}

// TestDifferentialGet applies random "get" commands to the same module graph with
// the go command and with gocpp, and compares the build lists after each one
// (and whether it worked).
func TestDifferentialGet(t *testing.T) {
	n := diffCount(t)
	first := 1
	if v := os.Getenv("GOCPP_DIFF_SEED"); v != "" { // one seed, to debug a failure
		fmt.Sscan(v, &first)
		n = first
	}
	for seed := first; seed <= n; seed++ {
		rng := rand.New(rand.NewSource(int64(1000 + seed)))
		spec := genGraph(rng)
		e := newEnv(t, mainFileText(spec.main, false), spec.mods...)
		g := newGoRunner(t, e.proxy)
		gdir := filepath.Join(g.root, "app")
		os.MkdirAll(gdir, 0o755)
		if err := g.prepare(gdir, mainFileText(spec.main, true), spec.mods); err != nil {
			t.Fatal(err)
		}
		paths := spec.paths()
		for step := 0; step < 4; step++ {
			var args []string
			used := map[string]bool{}
			for k, na := 0, 1+rng.Intn(2); k < na; k++ {
				p := paths[rng.Intn(len(paths))]
				if used[p] {
					continue
				}
				used[p] = true
				var q string
				vsAll := spec.versionsOf(p)
				switch rng.Intn(10) {
				case 6:
					q = ">=" + vsAll[rng.Intn(len(vsAll))]
				case 7:
					q = "<" + vsAll[rng.Intn(len(vsAll))]
				case 8:
					q = []string{"v1", "v1.1", "v0", "patch", "<=v1.3.0", ">v1.1.0"}[rng.Intn(6)]
				}
				if q != "" {
					args = append(args, p+"@"+q)
					continue
				}
				switch rng.Intn(6) {
				case 0:
					q = "latest"
				case 1:
					q = "upgrade"
				case 2:
					q = "none"
				case 3:
					q = "v9.9.9"
				default:
					vs := spec.versionsOf(p)
					q = vs[rng.Intn(len(vs))]
				}
				args = append(args, p+"@"+q)
			}
			desc := fmt.Sprintf("seed %d step %d: get %v", seed, step, args)
			if os.Getenv("GOCPP_DIFF_SEED") != "" {
				t.Logf("%s\n%s", e.mod(), desc)
			}
			_, gocppErr := e.get(args...)
			_, goErr := g.run(gdir, append([]string{"get"}, args...)...)
			if errors.Is(goErr, errGoTimeout) {
				t.Logf("%s: the go command timed out; skipped", desc)
				break
			}
			if (gocppErr == nil) != (goErr == nil) {
				// Known difference: the go command refuses some downgrades that
				// need a version that does not exist; both must agree here.
				t.Errorf("%s: gocpp: %v; go: %v", desc, gocppErr, goErr)
				break
			}
			if goErr != nil {
				continue
			}
			want, err := g.listAll(gdir)
			if err != nil {
				t.Fatalf("%s: %v", desc, err)
			}
			got, _, err := gocppList(e)
			if err != nil {
				t.Fatalf("%s: %v", desc, err)
			}
			if strings.Join(got, " ") != strings.Join(want, " ") {
				t.Errorf("%s:\n gocpp: %v\n go:    %v", desc, got, want)
				break
			}
		}
	}
}

// The hashes in gocpp.sum are the ones in go.sum for the same zips.
func TestGocppSumEqualsGoSum(t *testing.T) {
	mods := []tmod{
		{Path: "example.com/a", Version: "v1.0.0", Reqs: []string{"example.com/b@v1.0.0"}, Files: map[string]string{"LICENSE": "MIT\n", "sub/deep/y.go": "package deep\n", "data.txt": "x\n"}},
		{Path: "example.com/a", Version: "v1.1.0", Reqs: []string{"example.com/b@v1.0.0"}, Files: map[string]string{"sub/other.go": "package sub\n"}},
		{Path: "example.com/b", Version: "v1.0.0", Files: map[string]string{"b.go": "package b\n"}},
		{Path: "example.com/c/v2", Version: "v2.0.0", Files: map[string]string{"c.go": "package c\n"}},
		{Path: "example.com/lib", Version: "v3.1.0+incompatible", NoModFile: true, Files: map[string]string{"lib.h": "int lib;\n"}},
	}
	e := newEnv(t, appMod, mods...)
	g := newGoRunner(t, e.proxy)
	gdir := filepath.Join(g.root, "app")
	os.MkdirAll(gdir, 0o755)
	if err := os.WriteFile(filepath.Join(gdir, "go.mod"), []byte("module example.com/app\n\ngo 1.16\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.mustGet("example.com/a@v1.1.0", "example.com/c/v2@v2.0.0", "example.com/lib@v3.1.0+incompatible")
	e.mustGet("example.com/a@v1.0.0") // a downgrade also records the older version
	sum := map[string]string{}
	for _, l := range strings.Split(strings.TrimSpace(e.sum()), "\n") {
		f := strings.Fields(l)
		if len(f) != 3 {
			t.Fatalf("bad gocpp.sum line %q", l)
		}
		sum[f[0]+" "+f[1]] = f[2]
	}
	checked := 0
	for key, h := range sum {
		path, ver, _ := strings.Cut(key, " ")
		isMod := strings.HasSuffix(ver, "/go.mod")
		ver = strings.TrimSuffix(ver, "/go.mod")
		out, err := g.run(gdir, "mod", "download", "-json", path+"@"+ver)
		if err != nil {
			t.Errorf("go mod download %s@%s: %v", path, ver, err)
			continue
		}
		field := `"Sum": "`
		if isMod {
			field = `"GoModSum": "`
		}
		i := strings.Index(out, field)
		if i < 0 {
			t.Errorf("go mod download %s@%s: no %s in %s", path, ver, field, out)
			continue
		}
		want := out[i+len(field):]
		want = want[:strings.IndexByte(want, '"')]
		if h != want {
			t.Errorf("%s: gocpp.sum has %s, go has %s", key, h, want)
		}
		checked++
	}
	if checked < 8 {
		t.Errorf("only %d lines compared:\n%s", checked, e.sum())
	}
}
