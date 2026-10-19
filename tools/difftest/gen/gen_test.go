package gen

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// nothing here uses the module cache or the network
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}

func cfgFor(seed uint64, phase int) Config {
	return Config{Seed: seed, Features: FeaturesForPhase(phase), FatalPercent: 15}
}

func TestDeterministic(t *testing.T) {
	for id := 0; id < 30; id++ {
		a, err := Generate(cfgFor(7, 8), id)
		if err != nil {
			t.Fatal(err)
		}
		b, err := Generate(cfgFor(7, 8), id)
		if err != nil {
			t.Fatal(err)
		}
		if a.Src != b.Src {
			t.Fatalf("program %d differs between two generations", id)
		}
	}
	a, _ := Generate(cfgFor(7, 8), 3)
	b, _ := Generate(cfgFor(8, 8), 3)
	c, _ := Generate(cfgFor(7, 8), 4)
	if a.Src == b.Src || a.Src == c.Src {
		t.Error("seed and id must change the program")
	}
}

func many(t *testing.T, seed uint64, phase, n int) []*Program {
	t.Helper()
	var ps []*Program
	bad := 0
	for id := 0; id < n; id++ {
		p, err := safeGenerate(cfgFor(seed, phase), id)
		if err != nil {
			bad++
			if bad <= 3 {
				t.Errorf("seed %d: %v", seed, err)
			}
			continue
		}
		ps = append(ps, p)
	}
	if bad > 0 {
		t.Fatalf("%d of %d programs failed to generate", bad, n)
	}
	return ps
}

func safeGenerate(cfg Config, id int) (p *Program, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("program %d: panic: %v", id, r)
		}
	}()
	return Generate(cfg, id)
}

// TestTypeCheck type-checks 2000 programs, 200 to a bundle, the way gc would.
func TestTypeCheck(t *testing.T) {
	n := 2000
	if testing.Short() {
		n = 400
	}
	seed := uint64(11)
	if s := os.Getenv("DIFFTEST_SEED"); s != "" {
		seed, _ = strconv.ParseUint(s, 10, 64)
	}
	if s := os.Getenv("DIFFTEST_N"); s != "" {
		n, _ = strconv.Atoi(s)
		n = n / 200 * 200
	}
	ps := many(t, seed, 9, n)
	imp := importer.ForCompiler(token.NewFileSet(), "source", nil)
	for i := 0; i < len(ps); i += 200 {
		nfiles := 1 + (i/200)%3*3 // 1, 4 and 7 files
		src, err := BundleFiles(ps[i:i+200], Recovered, nfiles)
		if err != nil {
			t.Fatal(err)
		}
		checkSrc(t, imp, src, fmt.Sprintf("bundle %d", i/200))
	}
}

func TestBundleFiles(t *testing.T) {
	ps := many(t, 3, 9, 30)
	one, _ := BundleFiles(ps, Recovered, 1)
	if strings.Contains(one, FileMarker) {
		t.Fatal("one file must have no marker")
	}
	many4, err := BundleFiles(ps, Recovered, 4)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(many4, FileMarker); n != 5 { // main + 4
		t.Fatalf("%d files", n)
	}
	if !strings.HasPrefix(many4, FileMarker+"main\n") {
		t.Fatal("main first")
	}
	for _, p := range ps {
		if !strings.Contains(many4, "// ---- program "+fmt.Sprint(p.ID)+" ") {
			t.Fatalf("program %d is missing", p.ID)
		}
	}
	imp := importer.ForCompiler(token.NewFileSet(), "source", nil)
	checkSrc(t, imp, many4, "4 files")
	checkSrc(t, imp, one, "1 file")
}

func checkSrc(t *testing.T, imp types.Importer, src, name string) {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	secs := []string{src}
	if strings.HasPrefix(src, FileMarker) {
		secs = strings.Split(src, FileMarker)[1:]
	}
	for i, sec := range secs {
		if strings.HasPrefix(src, FileMarker) {
			_, sec, _ = strings.Cut(sec, "\n")
		}
		f, err := parser.ParseFile(fset, fmt.Sprintf("f%d.go", i), sec, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		files = append(files, f)
	}
	var errs []string
	conf := types.Config{Importer: imp, Error: func(err error) {
		if len(errs) < 8 {
			errs = append(errs, err.Error())
		}
	}}
	conf.Check("main", fset, files, nil)
	if len(errs) > 0 {
		var ctx strings.Builder
		for _, e := range errs {
			fmt.Fprintln(&ctx, e)
		}
		t.Fatalf("%s does not type-check:\n%s", name, ctx.String())
	}
}

func TestProgramsAreUsable(t *testing.T) {
	ps := many(t, 5, 9, 300)
	scen := map[string]int{}
	lines := 0
	for _, p := range ps {
		for _, u := range p.Used {
			scen[u]++
		}
		lines += p.Lines
		if !strings.Contains(p.Src, "func "+p.Entry+"()") {
			t.Fatalf("program %d has no entry %s", p.ID, p.Entry)
		}
	}
	if lines/len(ps) < 40 {
		t.Errorf("programs are too small: %d lines on average", lines/len(ps))
	}
	fs := FeaturesForPhase(9)
	for _, s := range scenarios {
		on := true
		for _, n := range s.need {
			on = on && fs[n]
		}
		if on && scen[s.name] == 0 {
			t.Errorf("scenario %s was never generated in 300 programs", s.name)
		}
	}
}

func typed(t *testing.T, src string) (*token.FileSet, []*ast.File, *types.Info) {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	secs := []string{src}
	if strings.HasPrefix(src, FileMarker) {
		secs = strings.Split(src, FileMarker)[1:]
	}
	for i, sec := range secs {
		if strings.HasPrefix(src, FileMarker) {
			_, sec, _ = strings.Cut(sec, "\n")
		}
		f, err := parser.ParseFile(fset, fmt.Sprintf("f%d.go", i), sec, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	conf := types.Config{Importer: importer.ForCompiler(token.NewFileSet(), "source", nil), Error: func(err error) { t.Error(err) }}
	conf.Check("main", fset, files, info)
	return fset, files, info
}

// TestForbiddenConstructs runs the static check over generated programs and over programs that
// must be refused.
func TestForbiddenConstructs(t *testing.T) {
	n := 600
	if testing.Short() {
		n = 200
	}
	src, err := BundleFiles(many(t, 17, 9, n), Recovered, 3)
	if err != nil {
		t.Fatal(err)
	}
	fset, files, info := typed(t, src)
	if bad := Lint(fset, files, info); len(bad) > 0 {
		var ctx strings.Builder
		for _, b := range bad[:min(len(bad), 8)] {
			ctx.WriteString(b + "\n")
			var fi, ln int
			fmt.Sscanf(b, "f%d.go:%d", &fi, &ln)
			_, text, _ := strings.Cut(strings.Split(src, FileMarker)[fi+1], "\n")
			ctx.WriteString("    " + strings.Split(text, "\n")[ln-1] + "\n")
		}
		t.Fatalf("%d forbidden constructs in generated programs:\n%s", len(bad), ctx.String())
	}
	bad := `package main

import (
	"fmt"
	"time"
	"unsafe"
)

type S struct{ P *int }

func main() {
	m := map[int]int{1: 2}
	for k := range m {
		fmt.Println(k)
	}
	x := 1
	fmt.Printf("%p\n", &x)
	fmt.Println(S{&x}, unsafe.Sizeof(x))
	fmt.Println(func() {}, make(chan int), &x, []*int{&x}, &S{}, time.Now())
}
`
	fset, files, info = typed(t, bad)
	got := strings.Join(Lint(fset, files, info), "\n")
	for _, want := range []string{`import "unsafe"`, "time.Now", "range over a map", "%p", "a pointer inside", "a func inside", "a chan inside", "pointer to a basic value inside"} {
		if !strings.Contains(got, want) {
			t.Errorf("lint missed %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "inside") < 5 {
		t.Errorf("lint count:\n%s", got)
	}
}

// TestFeatureGating: a program only uses what the features allow.
func TestFeatureGating(t *testing.T) {
	cases := []struct {
		phase   int
		absent  []string
		present []string
	}{
		{2, []string{"go func", "chan ", "interface", "defer", "recover()", "select {", "map[string]"}, []string{"fmt.Print"}},
		{3, []string{"go func", "chan ", "defer", "recover()", "interface", "select {"}, []string{"map[", "struct", "[]"}},
		{5, []string{"go func", "chan ", "select {"}, []string{"defer", "recover()", "interface"}},
		{8, nil, []string{"go func", "chan ", "select {", "defer"}},
	}
	for _, c := range cases {
		var all strings.Builder
		for id := 0; id < 60; id++ {
			p, err := safeGenerate(Config{Seed: 4, Features: FeaturesForPhase(c.phase), FatalPercent: 20}, id)
			if err != nil {
				t.Fatalf("phase %d: %v", c.phase, err)
			}
			all.WriteString(p.Src)
		}
		s := all.String()
		for _, a := range c.absent {
			if strings.Contains(s, a) {
				t.Errorf("phase %d: %q is generated but belongs to a later phase", c.phase, a)
			}
		}
		for _, a := range c.present {
			if !strings.Contains(s, a) {
				t.Errorf("phase %d: %q is never generated", c.phase, a)
			}
		}
	}
	// explicit switches
	f := FeaturesForPhase(9)
	if err := f.Parse("-select,-goroutines,+osexit"); err != nil {
		t.Fatal(err)
	}
	if f["select"] || f["goroutines"] || !f["osexit"] || !f["chan"] {
		t.Fatalf("%v", f.Names())
	}
	if err := f.Parse("bogus"); err == nil {
		t.Fatal("unknown feature accepted")
	}
	g := FeaturesForPhase(9)
	g.Parse("none")
	if len(g.Names()) != 0 {
		t.Fatalf("none: %v", g.Names())
	}
}

// TestOptInScenarios: the scenarios that are off by default are generated when asked for, and
// are valid Go.
func TestOptInScenarios(t *testing.T) {
	f := FeaturesForPhase(9)
	if f["capgrowth"] {
		t.Fatal("capgrowth must be off by default")
	}
	if err := f.Parse("+capgrowth"); err != nil {
		t.Fatal(err)
	}
	var ps []*Program
	used := 0
	for id := 0; id < 120; id++ {
		p, err := safeGenerate(Config{Seed: 6, Features: f, Scenarios: 3}, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, u := range p.Used {
			if u == "capgrowth" {
				used++
			}
		}
		ps = append(ps, p)
	}
	if used == 0 {
		t.Fatal("capgrowth was never generated")
	}
	src, err := BundleFiles(ps, Recovered, 2)
	if err != nil {
		t.Fatal(err)
	}
	checkSrc(t, importer.ForCompiler(token.NewFileSet(), "source", nil), src, "capgrowth")
}

func TestStderrMarkers(t *testing.T) {
	ps := many(t, 3, 9, 5)
	rec, err := BundleFiles(ps, Recovered, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec, `println("#### `+fmt.Sprint(ps[0].ID)+`")`) || !strings.Contains(rec, `println("== " + name)`) {
		t.Error("a recovered bundle must mark programs and scenarios on stderr too")
	}
	alone, err := Bundle(ps[:1], Standalone)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(alone, "println(\"####") || strings.Contains(alone, `println("== "`) {
		t.Error("a standalone program keeps stderr for its panic")
	}
}

func TestStateVersion(t *testing.T) {
	if Version < 2 {
		t.Fatal("the generator changed since version 1: Version must say so")
	}
}
