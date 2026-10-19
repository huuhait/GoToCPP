package modgraph

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
	"github.com/huuhait/gocpp/internal/modfetch"
	"github.com/huuhait/gocpp/internal/mvs"
)

// A fake is a Source of module files written as text.
type fake struct {
	files map[string]string   // "path@version" -> module file text ("-" for no file)
	vers  map[string][]string // path -> versions
	reads atomic.Int32
}

func newFake(lines ...string) *fake {
	f := &fake{files: map[string]string{}, vers: map[string][]string{}}
	for _, l := range lines {
		// "a@v1.0.0: b@v1.0.0 c@v1.1.0" or "a@v1.0.0" for no requirements, "a@v1.0.0: -" for no file
		head, reqs, _ := strings.Cut(l, ":")
		path, ver, _ := strings.Cut(strings.TrimSpace(head), "@")
		f.add(path, ver, strings.Fields(reqs))
	}
	return f
}

func (f *fake) add(path, ver string, reqs []string) {
	text := "module " + path + "\n"
	if len(reqs) == 1 && reqs[0] == "-" {
		text = "-"
	} else {
		for _, r := range reqs {
			p, v, _ := strings.Cut(r, "@")
			text += "require " + p + " " + v + "\n"
		}
	}
	f.files[path+"@"+ver] = text
	f.vers[path] = append(f.vers[path], ver)
	slices.SortFunc(f.vers[path], gocppmod.CompareVersions)
}

func (f *fake) ModFile(_ context.Context, path, version string) (*modfetch.ModFileInfo, error) {
	f.reads.Add(1)
	t, ok := f.files[path+"@"+version]
	if !ok {
		return nil, fmt.Errorf("%s@%s: reading proxy: 404 Not Found", path, version)
	}
	if t == "-" {
		return &modfetch.ModFileInfo{}, nil
	}
	return &modfetch.ModFileInfo{Name: "go.mod", Data: []byte(t)}, nil
}

func (f *fake) Versions(_ context.Context, path string) ([]string, error) { return f.vers[path], nil }

func mainFile(t *testing.T, text string) *gocppmod.File {
	t.Helper()
	file, errs := gocppmod.Parse("gocpp.mod", []byte(text), nil)
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
	return file
}

func buildList(t *testing.T, src Source, mod string) string {
	t.Helper()
	got, err := buildListErr(src, mod)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func buildListErr(src Source, mod string) (string, error) {
	file, errs := gocppmod.Parse("gocpp.mod", []byte(mod), nil)
	if len(errs) > 0 {
		return "", errs[0]
	}
	r := New(context.Background(), &Config{MainDir: os.TempDir(), File: file, Source: src})
	l, _, err := r.BuildList()
	if err != nil {
		return "", err
	}
	var parts []string
	for _, m := range l {
		parts = append(parts, m.String())
	}
	return strings.Join(parts, " "), nil
}

const head = "module example.com/app\n"

func TestBuildListCases(t *testing.T) {
	cases := []struct {
		name string
		src  *fake
		mod  string
		want string
	}{
		{"none", newFake(), head, "example.com/app"},
		{"direct", newFake("example.com/a@v1.0.0"), head + "require example.com/a v1.0.0\n", "example.com/app example.com/a@v1.0.0"},
		{"diamond takes the highest", newFake(
			"example.com/b@v1.0.0: example.com/d@v1.0.0", "example.com/c@v1.0.0: example.com/d@v1.2.0",
			"example.com/d@v1.0.0", "example.com/d@v1.2.0"),
			head + "require example.com/b v1.0.0\nrequire example.com/c v1.0.0\n",
			"example.com/app example.com/b@v1.0.0 example.com/c@v1.0.0 example.com/d@v1.2.0"},
		{"upgrade through an indirect requirement", newFake(
			"example.com/a@v1.0.0: example.com/b@v1.1.0", "example.com/b@v1.0.0", "example.com/b@v1.1.0"),
			head + "require example.com/a v1.0.0\nrequire example.com/b v1.0.0\n",
			"example.com/app example.com/a@v1.0.0 example.com/b@v1.1.0"},
		{"a lower version's requirements still count", newFake(
			"example.com/a@v1.0.0: example.com/c@v1.5.0", "example.com/a@v1.1.0",
			"example.com/c@v1.5.0", "example.com/b@v1.0.0: example.com/a@v1.1.0"),
			head + "require example.com/a v1.0.0\nrequire example.com/b v1.0.0\n",
			"example.com/app example.com/a@v1.1.0 example.com/b@v1.0.0 example.com/c@v1.5.0"},
		{"cycle", newFake("example.com/a@v1.0.0: example.com/b@v1.0.0", "example.com/b@v1.0.0: example.com/a@v1.0.0"),
			head + "require example.com/a v1.0.0\n", "example.com/app example.com/a@v1.0.0 example.com/b@v1.0.0"},
		{"the main module is selected whatever others require", newFake("example.com/a@v1.0.0: example.com/app@v1.9.0", "example.com/app@v1.9.0"),
			head + "require example.com/a v1.0.0\n", "example.com/app example.com/a@v1.0.0"},
		{"module with no module file (a C library) requires nothing", newFake("example.com/lib@v3.1.0+incompatible: -"),
			head + "require example.com/lib v3.1.0+incompatible\n", "example.com/app example.com/lib@v3.1.0+incompatible"},
		{"prerelease is below its release", newFake("example.com/a@v1.0.0-rc.1", "example.com/a@v1.0.0", "example.com/b@v1.0.0: example.com/a@v1.0.0-rc.1"),
			head + "require example.com/a v1.0.0\nrequire example.com/b v1.0.0\n", "example.com/app example.com/a@v1.0.0 example.com/b@v1.0.0"},
		{"pseudo-versions order by time", newFake("example.com/a@v0.0.0-20200101000000-aaaaaaaaaaaa", "example.com/a@v0.0.0-20210101000000-bbbbbbbbbbbb",
			"example.com/b@v1.0.0: example.com/a@v0.0.0-20210101000000-bbbbbbbbbbbb"),
			head + "require example.com/a v0.0.0-20200101000000-aaaaaaaaaaaa\nrequire example.com/b v1.0.0\n",
			"example.com/app example.com/a@v0.0.0-20210101000000-bbbbbbbbbbbb example.com/b@v1.0.0"},
		{"pseudo-version after a release is above it", newFake("example.com/a@v1.2.3", "example.com/a@v1.2.4-0.20200101000000-aaaaaaaaaaaa",
			"example.com/b@v1.0.0: example.com/a@v1.2.4-0.20200101000000-aaaaaaaaaaaa"),
			head + "require example.com/a v1.2.3\nrequire example.com/b v1.0.0\n",
			"example.com/app example.com/a@v1.2.4-0.20200101000000-aaaaaaaaaaaa example.com/b@v1.0.0"},
		{"+incompatible is a version of its own path", newFake("example.com/a@v2.0.0+incompatible", "example.com/a@v1.9.0", "example.com/b@v1.0.0: example.com/a@v2.0.0+incompatible"),
			head + "require example.com/a v1.9.0\nrequire example.com/b v1.0.0\n", "example.com/app example.com/a@v2.0.0+incompatible example.com/b@v1.0.0"},
		{"/v2 is a different module from the unversioned path", newFake("example.com/a@v1.0.0", "example.com/a/v2@v2.0.0"),
			head + "require example.com/a v1.0.0\nrequire example.com/a/v2 v2.0.0\n", "example.com/app example.com/a@v1.0.0 example.com/a/v2@v2.0.0"},
		{"exclude makes a requirement count as nothing", newFake("example.com/a@v1.0.0: example.com/b@v1.5.0", "example.com/b@v1.5.0: example.com/c@v1.0.0"),
			head + "require example.com/a v1.0.0\nexclude example.com/b v1.5.0\n", "example.com/app example.com/a@v1.0.0"},
		{"a main require of an excluded version is nothing as well", newFake(),
			head + "require example.com/a v1.0.0\nexclude example.com/a v1.0.0\n", "example.com/app"},
		{"retract lines of dependencies play no part", newFake("example.com/a@v1.0.0: example.com/b@v1.0.0", "example.com/b@v1.0.0"),
			head + "require example.com/a v1.0.0\n", "example.com/app example.com/a@v1.0.0 example.com/b@v1.0.0"},
		{"several require lines for a module: the highest counts", newFake("example.com/a@v1.0.0", "example.com/a@v1.3.0"),
			head + "require example.com/a v1.0.0\nrequire example.com/a v1.3.0\n", "example.com/app example.com/a@v1.3.0"},
		{"deep chain", newFake("example.com/a@v1.0.0: example.com/b@v1.0.0", "example.com/b@v1.0.0: example.com/c@v1.0.0", "example.com/c@v1.0.0: example.com/d@v1.0.0", "example.com/d@v1.0.0"),
			head + "require example.com/a v1.0.0\n", "example.com/app example.com/a@v1.0.0 example.com/b@v1.0.0 example.com/c@v1.0.0 example.com/d@v1.0.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildList(t, tc.src, tc.mod); got != tc.want {
				t.Errorf("build list:\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestMissingModule(t *testing.T) {
	_, err := buildListErr(newFake("example.com/a@v1.0.0: example.com/b@v1.0.0"), head+"require example.com/a v1.0.0\n")
	var mm *MissingModError
	if err == nil || !errors.As(err, &mm) || mm.Module.Path != "example.com/b" || !strings.Contains(err.Error(), "404 Not Found") {
		t.Fatalf("err = %v", err)
	}
}

func TestModulePathMismatch(t *testing.T) {
	f := newFake()
	f.files["example.com/a@v1.0.0"] = "module example.com/other\n"
	_, err := buildListErr(f, head+"require example.com/a v1.0.0\n")
	if err == nil || !strings.Contains(err.Error(), "module declares its path as: example.com/other") || !strings.Contains(err.Error(), "but was required as: example.com/a") {
		t.Fatalf("err = %v", err)
	}
}

func TestBadModuleFile(t *testing.T) {
	f := newFake()
	f.files["example.com/a@v1.0.0"] = "module example.com/a\nrequire example.com/b notaversion\n"
	_, err := buildListErr(f, head+"require example.com/a v1.0.0\n")
	if err == nil || !strings.Contains(err.Error(), "parsing go.mod") {
		t.Fatalf("err = %v", err)
	}
}

func TestDotlessPaths(t *testing.T) {
	// A module whose path has no dot can't be downloaded...
	_, err := buildListErr(newFake(), "module app\nrequire mylib v1.0.0\n")
	if !errors.Is(err, ErrDotless) {
		t.Fatalf("dotless download: %v", err)
	}
	// ...but a replace line with a local folder makes it fine.
	dir := t.TempDir()
	lib := filepath.Join(dir, "mylib")
	os.MkdirAll(lib, 0o755)
	os.WriteFile(filepath.Join(lib, "gocpp.mod"), []byte("module mylib\nrequire example.com/a v1.0.0\n"), 0o644)
	file := mainFile(t, "module app\nrequire mylib v1.0.0\nreplace mylib => ./mylib\n")
	r := New(context.Background(), &Config{MainDir: dir, File: file, Source: newFake("example.com/a@v1.0.0")})
	l, _, err := r.BuildList()
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(l); got != "[app example.com/a@v1.0.0 mylib@v1.0.0]" {
		t.Fatalf("build list = %v", got)
	}
	// A dotless first element of a path below a dotted one is fine.
	if got := buildList(t, newFake("example.com/x/y@v1.0.0"), head+"require example.com/x/y v1.0.0\n"); !strings.Contains(got, "example.com/x/y@v1.0.0") {
		t.Fatal(got)
	}
}

func TestReplaceLocal(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	os.MkdirAll(a, 0o755)
	// The replacement's gocpp.mod says what the module requires; the version in the build list stays v1.0.0.
	os.WriteFile(filepath.Join(a, "gocpp.mod"), []byte("module example.com/a\nrequire example.com/b v1.2.0\n"), 0o644)
	// go.mod is used when there is no gocpp.mod.
	b := filepath.Join(dir, "b")
	os.MkdirAll(b, 0o755)
	os.WriteFile(filepath.Join(b, "go.mod"), []byte("module example.com/b\n"), 0o644)
	src := newFake("example.com/a@v1.0.0: example.com/zzz@v9.9.9", "example.com/b@v1.2.0")
	file := mainFile(t, head+"require example.com/a v1.0.0\nreplace example.com/a => ./a\nreplace example.com/b v1.2.0 => ./b\n")
	r := New(context.Background(), &Config{MainDir: dir, File: file, Source: src})
	l, _, err := r.BuildList()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range l {
		got = append(got, m.String())
	}
	want := "example.com/app example.com/a@v1.0.0 example.com/b@v1.2.0"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %v, want %s", got, want)
	}
	// A missing replacement folder is a clear error.
	file = mainFile(t, head+"require example.com/a v1.0.0\nreplace example.com/a => ./nowhere\n")
	r = New(context.Background(), &Config{MainDir: dir, File: file, Source: src})
	if _, _, err := r.BuildList(); err == nil || !strings.Contains(err.Error(), "replacement directory ./nowhere") {
		t.Fatalf("missing folder: %v", err)
	}
	// A replacement that declares another path is an error.
	os.WriteFile(filepath.Join(a, "gocpp.mod"), []byte("module example.com/other\n"), 0o644)
	file = mainFile(t, head+"require example.com/a v1.0.0\nreplace example.com/a => ./a\n")
	r = New(context.Background(), &Config{MainDir: dir, File: file, Source: src})
	if _, _, err := r.BuildList(); err == nil || !strings.Contains(err.Error(), "module declares its path as: example.com/other") {
		t.Fatalf("wrong path: %v", err)
	}
	// A replacement that is a file, not a folder.
	os.WriteFile(filepath.Join(dir, "file"), []byte("x"), 0o644)
	file = mainFile(t, head+"require example.com/a v1.0.0\nreplace example.com/a => ./file\n")
	r = New(context.Background(), &Config{MainDir: dir, File: file, Source: src})
	if _, _, err := r.BuildList(); err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("a file: %v", err)
	}
}

func TestReplaceRemote(t *testing.T) {
	// replace example.com/a => example.com/fork v1.1.0: the requirements come from the fork.
	src := newFake("example.com/a@v1.0.0: example.com/zzz@v9.9.9", "example.com/fork@v1.1.0: example.com/b@v1.0.0", "example.com/b@v1.0.0")
	f := src.files
	f["example.com/fork@v1.1.0"] = "module example.com/fork\nrequire example.com/b v1.0.0\n"
	got := buildList(t, src, head+"require example.com/a v1.0.0\nreplace example.com/a => example.com/fork v1.1.0\n")
	if got != "example.com/app example.com/a@v1.0.0 example.com/b@v1.0.0" {
		t.Fatalf("got %s", got)
	}
	// The module file of the fork may declare either path.
	f["example.com/fork@v1.1.0"] = "module example.com/a\nrequire example.com/b v1.0.0\n"
	if got := buildList(t, src, head+"require example.com/a v1.0.0\nreplace example.com/a => example.com/fork v1.1.0\n"); !strings.Contains(got, "b@v1.0.0") {
		t.Fatalf("declared original: %s", got)
	}
}

func TestReadsEachModuleFileOnce(t *testing.T) {
	src := newFake("example.com/a@v1.0.0: example.com/c@v1.0.0", "example.com/b@v1.0.0: example.com/c@v1.0.0", "example.com/c@v1.0.0")
	buildList(t, src, head+"require example.com/a v1.0.0\nrequire example.com/b v1.0.0\n")
	if n := src.reads.Load(); n != 3 {
		t.Errorf("%d module file reads, want 3", n)
	}
}

func TestEdgesAndWhy(t *testing.T) {
	src := newFake("example.com/a@v1.0.0: example.com/b@v1.0.0 example.com/c@v1.0.0", "example.com/b@v1.0.0: example.com/c@v1.1.0", "example.com/c@v1.0.0", "example.com/c@v1.1.0")
	file := mainFile(t, head+"require example.com/a v1.0.0\n")
	r := New(context.Background(), &Config{MainDir: ".", File: file, Source: src})
	_, g, err := r.BuildList()
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, e := range Edges(g) {
		lines = append(lines, e.From.String()+" "+e.To.String())
	}
	want := []string{
		"example.com/app example.com/a@v1.0.0",
		"example.com/a@v1.0.0 example.com/b@v1.0.0",
		"example.com/a@v1.0.0 example.com/c@v1.0.0",
		"example.com/b@v1.0.0 example.com/c@v1.1.0",
	}
	if !slices.Equal(lines, want) {
		t.Errorf("edges:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	paths := func(c []mvs.Module) string {
		var s []string
		for _, m := range c {
			s = append(s, m.Path)
		}
		return strings.Join(s, " > ")
	}
	// c is selected at v1.1.0, which only b requires: the chain goes through b.
	if got := paths(Why(g, r.Main(), "example.com/c")); got != "example.com/app > example.com/a > example.com/b > example.com/c" {
		t.Errorf("why c: %s", got)
	}
	if got := paths(Why(g, r.Main(), "example.com/b")); got != "example.com/app > example.com/a > example.com/b" {
		t.Errorf("why b: %s", got)
	}
	if got := Why(g, r.Main(), "example.com/nope"); got != nil {
		t.Errorf("why nope: %v", got)
	}
	if got := paths(Why(g, r.Main(), "example.com/app")); got != "example.com/app" {
		t.Errorf("why main: %s", got)
	}
}

func TestMaxKeepsFirstOfEqualVersions(t *testing.T) {
	if got := maxVersion("v2.0.0", "v2.0.0+incompatible"); got != "v2.0.0" {
		t.Errorf("got %s", got)
	}
	if got := maxVersion("v2.0.0+incompatible", "v2.0.0"); got != "v2.0.0+incompatible" {
		t.Errorf("got %s", got)
	}
	if got := maxVersion("", "v1.0.0"); got != "" {
		t.Errorf("main module %q", got)
	}
}

func TestPreviousSkipsExcluded(t *testing.T) {
	src := newFake("example.com/a@v1.0.0", "example.com/a@v1.1.0", "example.com/a@v1.2.0")
	file := mainFile(t, head+"exclude example.com/a v1.1.0\n")
	r := New(context.Background(), &Config{MainDir: ".", File: file, Source: src})
	p, err := r.Previous(mvs.Module{Path: "example.com/a", Version: "v1.2.0"})
	if err != nil || p.Version != "v1.0.0" {
		t.Fatalf("previous = %v, %v", p, err)
	}
	p, _ = r.Previous(mvs.Module{Path: "example.com/a", Version: "v1.0.0"})
	if p.Version != mvs.None {
		t.Fatalf("previous of the first = %v", p)
	}
}

// A fake that knows retract lines, as FetcherSource does.
type retractFake struct {
	*fake
	rets map[string][]*gocppmod.Retract
}

func (f retractFake) Retractions(_ context.Context, path string) ([]*gocppmod.Retract, error) {
	return f.rets[path], nil
}

func TestPreviousSkipsRetracted(t *testing.T) {
	src := retractFake{newFake("example.com/a@v1.0.0", "example.com/a@v1.1.0", "example.com/a@v1.2.0", "example.com/a@v1.3.0"),
		map[string][]*gocppmod.Retract{"example.com/a": {{Low: "v1.1.0", High: "v1.2.0"}}}}
	prev := func(file, v string) string {
		r := New(context.Background(), &Config{MainDir: ".", File: mainFile(t, file), Source: src})
		p, err := r.Previous(mvs.Module{Path: "example.com/a", Version: v})
		if err != nil {
			t.Fatal(err)
		}
		return p.Version
	}
	if got := prev(head, "v1.3.0"); got != "v1.0.0" {
		t.Errorf("previous of v1.3.0 = %s, want v1.0.0 (v1.1.0 and v1.2.0 are retracted)", got)
	}
	if got := prev(head, "v1.0.0"); got != mvs.None {
		t.Errorf("previous of v1.0.0 = %s", got)
	}
	// A replace line for all versions: Go loads no retractions then.
	if got := prev(head+"replace example.com/a => example.com/a v1.3.0\n", "v1.3.0"); got != "v1.2.0" {
		t.Errorf("previous with a replace line = %s", got)
	}
	// A Source that does not know retractions skips none.
	r := New(context.Background(), &Config{MainDir: ".", File: mainFile(t, head), Source: src.fake})
	if p, _ := r.Previous(mvs.Module{Path: "example.com/a", Version: "v1.3.0"}); p.Version != "v1.2.0" {
		t.Errorf("plain source: %s", p.Version)
	}
}

func TestUpgradeUsesLatest(t *testing.T) {
	src := newFake("example.com/a@v1.0.0", "example.com/a@v1.1.0", "example.com/b@v1.0.0", "example.com/b@v1.4.0")
	file := mainFile(t, head)
	r := New(context.Background(), &Config{MainDir: ".", File: file, Source: src,
		Latest: func(_ context.Context, path, cur string) (string, error) {
			vs := src.vers[path]
			return vs[len(vs)-1], nil
		}})
	for _, tc := range []struct{ in, want string }{
		{"example.com/a@v1.0.0", "example.com/a@v1.1.0"},
		{"example.com/b@v1.4.0", "example.com/b@v1.4.0"},
	} {
		p, v, _ := strings.Cut(tc.in, "@")
		got, err := r.Upgrade(mvs.Module{Path: p, Version: v})
		if err != nil || got.String() != tc.want {
			t.Errorf("Upgrade(%s) = %v, %v; want %s", tc.in, got, err, tc.want)
		}
	}
}

func TestMissingModErrorNamesTheModuleOnce(t *testing.T) {
	m := mvs.Module{Path: "example.com/lib", Version: "v1.0.0"}
	e := &MissingModError{Module: m, Err: errors.New("example.com/lib@v1.0.0: reading proxy: 404 Not Found")}
	if got, want := e.Error(), "example.com/lib@v1.0.0: reading proxy: 404 Not Found"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	e = &MissingModError{Module: m, What: "replaced by ../x", Err: errors.New("replacement directory ../x: no such file")}
	if got, want := e.Error(), "example.com/lib@v1.0.0 (replaced by ../x): replacement directory ../x: no such file"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
