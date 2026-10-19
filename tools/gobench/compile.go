package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/huuhait/gocpp/tools/gobench/benchdata"
)

// The compile-time programs are generated in Go syntax once and written as
// .gp for gocpp and as .go for Go, so both compile the same text:
//
//	hello        fmt.Println("hello")
//	pkgs20       a module of 20 packages (each with a struct, methods and functions) and a main that uses them all
//	generics100  one generic function and one generic type, instantiated with 100 different types
//	lines1000    one file of about 1000 lines of plain functions
type genProject struct {
	name   string
	module bool                                // needs go.mod / gocpp.mod
	files  func(variant int) map[string]string // path -> Go-syntax source; variant changes main only
}

func compileProjects() []genProject {
	return []genProject{
		{"hello", false, genHello},
		{"pkgs20", true, genPkgs20},
		{"generics100", false, genGenerics100},
		{"lines1000", false, genLines1000},
	}
}

func genHello(v int) map[string]string {
	return map[string]string{"main.GO": fmt.Sprintf("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\", %d) }\n", v)}
}

func genPkgs20(v int) map[string]string {
	files := map[string]string{}
	var imports, calls strings.Builder
	for i := 0; i < 20; i++ {
		files[fmt.Sprintf("p%d/p%d.GO", i, i)] = fmt.Sprintf(`package p%[1]d

type T struct {
	a, b int
	name string
}

func New(n int) *T { return &T{a: n, b: n * %[2]d, name: "p%[1]d"} }

func (t *T) Sum() int { return t.a + t.b + len(t.name) }

func (t *T) Scale(k int) { t.a *= k; t.b *= k }

func Fold(xs []int) int {
	s := 0
	for _, x := range xs {
		s += x * %[2]d
	}
	return s
}
`, i, i+1)
		fmt.Fprintf(&imports, "\t\"example.com/bench/p%d\"\n", i)
		fmt.Fprintf(&calls, "\tt%[1]d := p%[1]d.New(%[1]d)\n\tt%[1]d.Scale(2)\n\ttotal += t%[1]d.Sum() + p%[1]d.Fold([]int{1, 2, 3})\n", i)
	}
	files["main.GO"] = fmt.Sprintf("package main\n\nimport (\n\t\"fmt\"\n\n%s)\n\nfunc main() {\n\ttotal := %d\n%s\tfmt.Println(total)\n}\n", imports.String(), v, calls.String())
	return files
}

func genGenerics100(v int) map[string]string {
	var b strings.Builder
	b.WriteString("package main\n\nimport \"fmt\"\n\n")
	b.WriteString("func Map[T any, U any](xs []T, f func(T) U) []U {\n\tout := make([]U, 0, len(xs))\n\tfor _, x := range xs {\n\t\tout = append(out, f(x))\n\t}\n\treturn out\n}\n\n")
	b.WriteString("func First[T any](xs []T) T { return xs[0] }\n\n")
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&b, "type S%[1]d struct{ v int }\n\nfunc mk%[1]d(i int) S%[1]d { return S%[1]d{v: i + %[1]d} }\n\n", i)
	}
	fmt.Fprintf(&b, "func main() {\n\ttotal := %d\n\txs := []int{1, 2, 3}\n", v)
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&b, "\ttotal += First(Map(xs, mk%d)).v\n", i)
	}
	b.WriteString("\tfmt.Println(total)\n}\n")
	return map[string]string{"main.GO": b.String()}
}

func genLines1000(v int) map[string]string {
	var b strings.Builder
	b.WriteString("package main\n\nimport \"fmt\"\n\n")
	for i := 0; i < 76; i++ {
		fmt.Fprintf(&b, `func f%[1]d(n int) int {
	s := 0
	for i := 0; i < n; i++ {
		if i%%%[2]d == 0 {
			s += i * %[1]d
		} else {
			s -= i
		}
	}
	return s
}

`, i, i%7+2)
	}
	fmt.Fprintf(&b, "func main() {\n\ttotal := %d\n", v)
	for i := 0; i < 76; i++ {
		fmt.Fprintf(&b, "\ttotal += f%d(10)\n", i)
	}
	b.WriteString("\tfmt.Println(total)\n}\n")
	return map[string]string{"main.GO": b.String()}
}

// writeProject writes the project for the language ("gocpp" or "go") into dir.
func writeProject(dir string, p genProject, variant int, lang string) error {
	ext := ".gp"
	if lang == "go" {
		ext = ".go"
	}
	for path, src := range p.files(variant) {
		path = strings.TrimSuffix(path, ".GO") + ext
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(src), 0o644); err != nil {
			return err
		}
	}
	if p.module {
		mod, content := "gocpp.mod", "module example.com/bench\n"
		if lang == "go" {
			mod, content = "go.mod", "module example.com/bench\n\ngo 1.25\n"
		}
		return os.WriteFile(filepath.Join(dir, mod), []byte(content), 0o644)
	}
	if lang == "go" {
		return os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module bench\n\ngo 1.25\n"), 0o644)
	}
	return nil
}

// runCompileBench measures the build times: for every program and every
// builder (gocpp with g++ and with clang++, each with the default -O0 -g
// flags and with -O2; Go), cold (empty caches), warm (the runtime or the
// standard library cached, one literal of main changed) and no-op (nothing
// changed). reps is the number of runs of each measurement.
func runCompileBench(cfg *Config, reps int) []benchdata.Compile {
	var out []benchdata.Compile
	type builder struct {
		impl, flavor string
	}
	var builders []builder
	seen := map[string]bool{}
	for _, impl := range cfg.Impls {
		if impl == benchdata.ImplGo {
			builders = append(builders, builder{impl, ""})
			continue
		}
		cxx, _, ok := benchdata.SplitGocpp(impl)
		if !ok || seen[cxx] {
			continue
		}
		seen[cxx] = true
		for _, fl := range cfg.compileFlavors() {
			builders = append(builders, builder{"gocpp-" + cxx, fl})
		}
	}
	for _, p := range compileProjects() {
		for _, b := range builders {
			rec := func(mode string, walls []int64, status, note string) {
				out = append(out, benchdata.Compile{Name: p.name, Impl: b.impl, Mode: mode, Flavor: b.flavor, Status: status, Note: note, WallNS: walls})
			}
			cfg.logf("compile %-12s %-14s %-8s ...", p.name, b.impl, b.flavor)
			if b.flavor == "release" && !cfg.Release {
				for _, mode := range []string{"cold", "warm", "noop"} {
					rec(mode, nil, benchdata.StatusSkipped, "gocpp has no build -release yet")
				}
				continue
			}
			walls, err := measureCompile(cfg, p, b.impl, b.flavor, reps)
			for _, mode := range []string{"cold", "warm", "noop"} {
				if err != nil {
					rec(mode, nil, benchdata.StatusBuildFailed, err.Error())
				} else {
					rec(mode, walls[mode], benchdata.StatusOK, "")
				}
			}
		}
	}
	return out
}

// compileFlavors returns the flavors the compile-time runs measure: the
// configured ones, and always the default build (what users get today).
func (c *Config) compileFlavors() []string {
	fls := []string{"default"}
	for _, f := range benchdata.Flavors {
		if f != "default" && slices.Contains(c.Flavors, f) {
			fls = append(fls, f)
		}
	}
	return fls
}

// measureCompile returns the wall times (ns) per mode of one program with one builder.
func measureCompile(cfg *Config, p genProject, impl, flavor string, reps int) (map[string][]int64, error) {
	lang := "gocpp"
	if impl == benchdata.ImplGo {
		lang = "go"
	}
	cxxflags, extra := cfg.flavorFlags(flavor)
	root := filepath.Join(cfg.Work, "compile", p.name, impl+"-"+flavor)
	if err := os.RemoveAll(root); err != nil {
		return nil, err
	}
	defer os.RemoveAll(root)
	src := filepath.Join(root, "src")
	bin := filepath.Join(root, "prog")
	envFor := func(cache string) []string {
		if lang == "go" {
			return cfg.goEnv("GOCACHE=" + cache)
		}
		return cfg.gocppEnv(cfg.cxxOf(impl), "XDG_CACHE_HOME="+cache, "GOCPP_CXXFLAGS="+cxxflags)
	}
	argv := func() []string {
		if lang == "go" {
			return []string{cfg.GoBin, "build", "-o", bin, "."}
		}
		return append(append([]string{cfg.Gocpp, "build"}, extra...), "-o", bin, ".")
	}
	timeBuild := func(env []string) (int64, error) {
		start := time.Now()
		out, err := capture(src, env, 30*time.Minute, argv()...)
		d := time.Since(start)
		if err != nil {
			return 0, fmt.Errorf("%v: %s", err, firstLine(lastErrorLine(out)))
		}
		return d.Nanoseconds(), nil
	}
	walls := map[string][]int64{}
	for i := 0; i < reps; i++ { // cold: a new empty cache every time
		if err := os.RemoveAll(src); err != nil {
			return nil, err
		}
		if err := writeProject(src, p, 0, lang); err != nil {
			return nil, err
		}
		cache := filepath.Join(root, fmt.Sprintf("cold%d", i))
		d, err := timeBuild(envFor(cache))
		if err != nil {
			return nil, err
		}
		walls["cold"] = append(walls["cold"], d)
		if i < reps-1 {
			os.RemoveAll(cache)
		}
	}
	// The last cold cache is the warm one: runtime and standard library built.
	env := envFor(filepath.Join(root, fmt.Sprintf("cold%d", reps-1)))
	for i := 0; i < reps; i++ { // warm: main changes
		if err := writeProject(src, p, i+1, lang); err != nil {
			return nil, err
		}
		d, err := timeBuild(env)
		if err != nil {
			return nil, err
		}
		walls["warm"] = append(walls["warm"], d)
	}
	for i := 0; i < reps; i++ { // no-op: nothing changes
		d, err := timeBuild(env)
		if err != nil {
			return nil, err
		}
		walls["noop"] = append(walls["noop"], d)
	}
	return walls, nil
}
